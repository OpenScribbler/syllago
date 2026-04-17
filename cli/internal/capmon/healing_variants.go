package capmon

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

// maxVariants caps how many URL permutations we probe per source. Beyond
// this the search starts hitting origin rate limits and finding random
// near-matches that aren't actual renames.
const maxVariants = 20

// VariantCandidate is one URL permutation probed via HEAD.
type VariantCandidate struct {
	URL    string
	Reason string // short description of the transformation
}

// GenerateVariants returns a ranked list of plausible URL replacements
// for rawURL. Transformations include case swaps, separator swaps, and
// common doc-prefix substitutions (create-X ↔ add-X ↔ new-X).
//
// The caller is expected to probe candidates in order and accept the
// first one whose HEAD returns 2xx and whose body passes
// ValidateContentResponse. Variants are generated deterministically so
// order is stable across runs.
func GenerateVariants(rawURL string) []VariantCandidate {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	dir := path.Dir(u.Path)
	base := path.Base(u.Path)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	seen := map[string]bool{rawURL: true}
	var out []VariantCandidate
	add := func(newStem, reason string) {
		if len(out) >= maxVariants {
			return
		}
		if newStem == stem {
			return
		}
		newPath := path.Join(dir, newStem+ext)
		newURL := *u
		newURL.Path = newPath
		s := newURL.String()
		if seen[s] {
			return
		}
		seen[s] = true
		out = append(out, VariantCandidate{URL: s, Reason: reason})
	}

	// Case variants — useful when a docs site renames "Foo.md" to "foo.md".
	if lower := strings.ToLower(stem); lower != stem {
		add(lower, "lowercase stem")
	}

	// Separator swaps.
	if strings.Contains(stem, "-") {
		add(strings.ReplaceAll(stem, "-", "_"), "hyphen → underscore")
	}
	if strings.Contains(stem, "_") {
		add(strings.ReplaceAll(stem, "_", "-"), "underscore → hyphen")
	}

	// Common prefix swaps — capmon's providers use verb prefixes that get
	// reorganized as docs mature. "create-X" often becomes "add-X" or just
	// "X" after the authoring-vs-consuming verbs are split.
	prefixSwaps := [][2]string{
		{"create-", "add-"},
		{"create-", "new-"},
		{"add-", "create-"},
		{"add-", "new-"},
		{"new-", "create-"},
		{"new-", "add-"},
	}
	for _, swap := range prefixSwaps {
		if strings.HasPrefix(stem, swap[0]) {
			add(swap[1]+strings.TrimPrefix(stem, swap[0]), fmt.Sprintf("prefix %q → %q", swap[0], swap[1]))
		}
	}

	// Drop-verb variant: "create-workflow" → "workflow".
	for _, prefix := range []string{"create-", "add-", "new-", "using-", "writing-"} {
		if strings.HasPrefix(stem, prefix) {
			add(strings.TrimPrefix(stem, prefix), fmt.Sprintf("dropped prefix %q", prefix))
		}
	}

	// Index fallback — some docs migrate "foo.md" to "foo/index.md".
	if !strings.Contains(stem, "/") {
		add(stem+"/index", "converted to index file")
	}

	return out
}

// ProbeVariants issues HEAD requests against each candidate and returns
// the first one that responds with a 2xx status. The second return value
// is the matched candidate (or zero value). A returned bool of false
// means no variant responded successfully.
//
// Uses a non-following client: HEAD 200 is the acceptance signal. The
// orchestrator will issue a real GET and call ValidateContentResponse
// before trusting the variant.
func ProbeVariants(ctx context.Context, variants []VariantCandidate) (VariantCandidate, bool, error) {
	// Bound to the first maxVariants candidates as a safety lid in case a
	// caller supplies more.
	if len(variants) > maxVariants {
		variants = variants[:maxVariants]
	}

	// Per-request timeout bounds DNS + connect + HEAD. Unreachable variants
	// are common (that's the whole point of probing), so we can't wait on
	// default DNS timeouts for each of 20 candidates.
	noFollow := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, v := range variants {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, v.URL, nil)
		if err != nil {
			return VariantCandidate{}, false, fmt.Errorf("create HEAD for %q: %w", v.URL, err)
		}
		req.Header.Set("User-Agent", "syllago-capmon/1.0")
		resp, err := noFollow.Do(req)
		if err != nil {
			// Network errors — skip this candidate, try the next.
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return v, true, nil
		}
		// A 3xx response is common if variants match a redirect target; we
		// could follow, but the redirect strategy already covers that path
		// — better to let the orchestrator retry with the redirect chain
		// follower applied to this URL.
	}
	return VariantCandidate{}, false, nil
}

// SortedVariantsForTest returns variants in their generation order with
// stable ordering for tests. The underlying generator is already
// deterministic, but this hook lets tests assert on a sorted view when
// comparing sets.
func SortedVariantsForTest(vs []VariantCandidate) []VariantCandidate {
	out := make([]VariantCandidate, len(vs))
	copy(out, vs)
	sort.Slice(out, func(i, j int) bool {
		return out[i].URL < out[j].URL
	})
	return out
}
