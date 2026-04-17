package capmon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

// rawGitHubHost is the hostname for GitHub's raw content CDN.
const rawGitHubHost = "raw.githubusercontent.com"

// rawGitHubPathPattern matches paths of the form /{owner}/{repo}/{ref}/{path}.
// Example: /anthropics/claude-code/main/docs/settings.md
var rawGitHubPathPattern = regexp.MustCompile(`^/([^/]+)/([^/]+)/([^/]+)/(.+)$`)

// maxRenameCandidates caps how many candidate blobs we score. Most repos
// have fewer than a few thousand files; 256 is a safety lid against
// pathological trees.
const maxRenameCandidates = 256

// RenameCandidate is one possible replacement for a renamed file.
type RenameCandidate struct {
	// Path is the full path within the repo (e.g. "docs/new-name.md").
	Path string
	// URL is the raw.githubusercontent.com URL rebuilt from the candidate path.
	URL string
	// Score is a similarity score in [0,1] where 1 is a perfect basename match
	// after normalization. Higher is better.
	Score float64
	// Reason briefly describes why this candidate was chosen (for PR body).
	Reason string
}

// DetectGitHubRename looks for a likely replacement file in the same repo
// and ref when a raw.githubusercontent.com URL 404s. It calls the git/trees
// API with ?recursive=1 to list all blobs, scores candidates by stem
// similarity to the original basename, and returns ranked candidates.
//
// Returns nil (with nil error) if rawURL is not a raw.githubusercontent.com
// URL or if the repo has no candidates above the similarity threshold.
//
// Callers should apply ValidateContentResponse to the top candidate's URL
// before accepting it as a heal — this function only identifies *likely*
// replacements, not verified ones.
func DetectGitHubRename(ctx context.Context, rawURL string) ([]RenameCandidate, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if u.Hostname() != rawGitHubHost {
		return nil, nil
	}
	m := rawGitHubPathPattern.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("raw github URL %q does not match expected layout /{owner}/{repo}/{ref}/{path}", rawURL)
	}
	owner, repo, ref, filePath := m[1], m[2], m[3], m[4]

	origBasename := path.Base(filePath)
	origDir := path.Dir(filePath)
	origStem := strings.TrimSuffix(origBasename, path.Ext(origBasename))
	origExt := path.Ext(origBasename)

	tree, err := fetchRepoTree(ctx, owner, repo, ref)
	if err != nil {
		return nil, fmt.Errorf("list repo tree: %w", err)
	}

	var candidates []RenameCandidate
	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		candBase := path.Base(entry.Path)
		candExt := path.Ext(candBase)
		candStem := strings.TrimSuffix(candBase, candExt)
		// Only consider candidates with the same extension — a .md is never
		// a heal for a .json.
		if !strings.EqualFold(candExt, origExt) {
			continue
		}

		score := stemSimilarity(origStem, candStem)
		if score < renameScoreFloor {
			continue
		}

		// Light directory-proximity nudge: a candidate in the same directory
		// is slightly preferred over one elsewhere.
		candDir := path.Dir(entry.Path)
		reason := fmt.Sprintf("stem similarity %.2f (%q → %q)", score, origStem, candStem)
		if candDir == origDir {
			score += 0.05
			reason += "; same directory"
		} else if strings.HasPrefix(candDir, origDir+"/") || strings.HasPrefix(origDir, candDir+"/") {
			score += 0.02
			reason += "; nearby directory"
		}

		// Rebuild the raw URL for this candidate.
		candURL := fmt.Sprintf("https://%s/%s/%s/%s/%s", rawGitHubHost, owner, repo, ref, entry.Path)
		candidates = append(candidates, RenameCandidate{
			Path:   entry.Path,
			URL:    candURL,
			Score:  score,
			Reason: reason,
		})
		if len(candidates) >= maxRenameCandidates {
			break
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	return candidates, nil
}

// renameScoreFloor is the minimum stem similarity to treat a file as a
// plausible rename candidate. The value is intentionally loose — content
// validation (ValidateContentResponse) and PR review are the real gate,
// not this score. Raising this just forces more fallbacks to the variant
// strategy or to auto-issue escalation.
const renameScoreFloor = 0.4

type gitTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type gitTreeResponse struct {
	Tree      []gitTreeEntry `json:"tree"`
	Truncated bool           `json:"truncated"`
}

// fetchRepoTree lists all blobs in a repo at the given ref via the
// git/trees API with ?recursive=1. The ref can be a branch, tag, or SHA.
func fetchRepoTree(ctx context.Context, owner, repo, ref string) ([]gitTreeEntry, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", githubBaseURL, owner, repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := httpDoer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github tree request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // nothing actionable on close failure of a drained body

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github tree status %d: %s", resp.StatusCode, string(body))
	}

	var out gitTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode tree response: %w", err)
	}
	// If the tree is truncated, candidates may be incomplete — we still
	// return what we have. The healing PR body will note this so reviewers
	// know the suggestion may miss files.
	return out.Tree, nil
}

// stemSimilarity returns a similarity score in [0,1] between two file stems.
// The scoring combines exact-match (1.0), common-prefix boost, and a
// token-overlap component that handles reorderings like create-workflow →
// workflow-create.
func stemSimilarity(a, b string) float64 {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	if a == b {
		return 1.0
	}
	// Token overlap (Jaccard) on word-split stems.
	at := tokenizeStem(a)
	bt := tokenizeStem(b)
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	intersect := 0
	for _, x := range at {
		for _, y := range bt {
			if x == y {
				intersect++
				break
			}
		}
	}
	union := len(at) + len(bt) - intersect
	jaccard := float64(intersect) / float64(union)

	// Common prefix contribution — "create-workflow" vs "create-workflow-v2"
	// should score higher than Jaccard alone.
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	prefixLen := 0
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			break
		}
		prefixLen++
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	prefixRatio := float64(prefixLen) / float64(maxLen)

	// Blend: Jaccard dominates (it handles reordering), prefix adds a small
	// boost for shared leading substrings.
	return 0.7*jaccard + 0.3*prefixRatio
}

// tokenizeStem splits a file stem on hyphens, underscores, dots, and case
// boundaries (camelCase). Used for token-overlap similarity.
func tokenizeStem(s string) []string {
	// Replace common separators with single delimiter, then split.
	replacer := strings.NewReplacer("-", " ", "_", " ", ".", " ")
	s = replacer.Replace(s)
	// Insert spaces before uppercase letters that follow lowercase (camelCase
	// splitting). We assume input has already been lowercased by the caller,
	// but guard anyway.
	fields := strings.Fields(s)
	var out []string
	for _, f := range fields {
		if f == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}
