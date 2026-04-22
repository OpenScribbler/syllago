package provmon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// checkSourceHash implements drift detection for ChangeDetection.Method ==
// "source-hash". It loads the provider's FormatDoc (capmon-curated) to read
// the baseline hash for each manifest source, refetches the live content, and
// emits one SourceDrift per source. The top-level Drifted flag is set if ANY
// source transitions to StatusDrifted — fetch/content failures and missing
// baselines are NOT drift, they're diagnostic signals.
func checkSourceHash(ctx context.Context, m *Manifest, formatsDir string) (*VersionDrift, error) {
	drift := &VersionDrift{
		Method:   "source-hash",
		Baseline: m.ChangeDetection.Baseline,
	}

	var doc *capmon.FormatDoc
	docPath := capmon.FormatDocPath(formatsDir, m.Slug)
	if _, err := os.Stat(docPath); err == nil {
		loaded, loadErr := capmon.LoadFormatDoc(docPath)
		if loadErr != nil {
			return nil, fmt.Errorf("loading FormatDoc %s: %w", docPath, loadErr)
		}
		doc = loaded
	}

	for _, ts := range manifestSources(m) {
		sd := checkOneSource(ctx, m.Slug, ts.contentType, ts.entry, doc)
		drift.Sources = append(drift.Sources, sd)
		if sd.Status == StatusDrifted {
			drift.Drifted = true
		}
	}

	return drift, nil
}

// taggedSource pairs a manifest SourceEntry with the content-type bucket it
// came from so checkOneSource can populate SourceDrift.ContentType without a
// second lookup.
type taggedSource struct {
	contentType string
	entry       SourceEntry
}

// manifestSources flattens the six fixed ContentTypes fields into a single
// ordered slice. Order is rules → hooks → mcp → skills → agents → commands,
// matching the declaration order in Manifest.ContentTypes and producing a
// deterministic Sources slice for callers/tests.
func manifestSources(m *Manifest) []taggedSource {
	var out []taggedSource
	buckets := []struct {
		name string
		ct   ContentType
	}{
		{"rules", m.ContentTypes.Rules},
		{"hooks", m.ContentTypes.Hooks},
		{"mcp", m.ContentTypes.MCP},
		{"skills", m.ContentTypes.Skills},
		{"agents", m.ContentTypes.Agents},
		{"commands", m.ContentTypes.Commands},
	}
	for _, b := range buckets {
		for _, entry := range b.ct.Sources {
			out = append(out, taggedSource{contentType: b.name, entry: entry})
		}
	}
	return out
}

// checkOneSource computes the SourceDrift for a single manifest source by
// looking up its baseline in the FormatDoc, fetching the current body,
// validating the response, and comparing sha256 hashes. Every exit path sets
// exactly one SourceDriftStatus.
func checkOneSource(ctx context.Context, slug, contentType string, entry SourceEntry, doc *capmon.FormatDoc) SourceDrift {
	sd := SourceDrift{
		ContentType: contentType,
		URI:         entry.URL,
	}

	if doc == nil {
		sd.Status = StatusSkipped
		sd.ErrorMessage = "FormatDoc missing"
		return sd
	}

	ref, ok := findSourceRef(doc, entry.URL)
	if !ok {
		sd.Status = StatusSkipped
		sd.ErrorMessage = "missing from FormatDoc"
		return sd
	}
	sd.Baseline = ref.ContentHash
	if ref.ContentHash == "" {
		sd.Status = StatusSkipped
		sd.ErrorMessage = "baseline empty"
		return sd
	}

	body, respContentType, finalURL, err := fetchSourceBytes(ctx, ref.FetchMethod, slug, entry.URL)
	if err != nil {
		sd.Status = StatusFetchFailed
		sd.ErrorMessage = err.Error()
		return sd
	}

	// ValidateContentResponse is only meaningful for HTML responses — an
	// origin swapping real content for a login wall is the concrete threat.
	// Short markdown/JSON bodies are a legitimate drift signal and must not
	// be misclassified as content_invalid just for being small.
	if isHTMLContentType(respContentType) {
		if vErr := capmon.ValidateContentResponse(body, respContentType, entry.URL, finalURL); vErr != nil {
			var invalid *capmon.ErrContentInvalid
			if errors.As(vErr, &invalid) {
				sd.Status = StatusContentInvalid
				sd.ErrorMessage = invalid.Reason
				return sd
			}
			sd.Status = StatusContentInvalid
			sd.ErrorMessage = vErr.Error()
			return sd
		}
	}

	sd.CurrentHash = capmon.SHA256Hex(body)
	if sd.CurrentHash == sd.Baseline {
		sd.Status = StatusStable
	} else {
		sd.Status = StatusDrifted
	}
	return sd
}

// findSourceRef returns the first SourceRef in any content-type bucket whose
// URI matches url exactly. Exact match (not substring/normalization) keeps
// manifest and FormatDoc baselines in lock-step — if the URL is rewritten in
// one place but not the other, the check correctly surfaces "missing from
// FormatDoc" instead of silently using a stale baseline.
func findSourceRef(doc *capmon.FormatDoc, url string) (capmon.SourceRef, bool) {
	for _, ct := range doc.ContentTypes {
		for _, src := range ct.Sources {
			if src.URI == url {
				return src, true
			}
		}
	}
	return capmon.SourceRef{}, false
}

// fetchSourceBytes retrieves a source body and returns (body, contentType,
// finalURL, err). For fetchMethod == "chromedp" it delegates to
// capmon.FetchChromedp with a scratch cache dir; otherwise it does a direct
// HTTP GET via provmon's overridable httpClient (so tests can redirect to an
// httptest server). This deliberately does NOT reuse capmon.FetchSource —
// that path bakes in a 7-second retry budget and persistent caching that
// provmon's ephemeral drift reports do not need.
func fetchSourceBytes(ctx context.Context, fetchMethod, slug, rawURL string) ([]byte, string, string, error) {
	if fetchMethod == "chromedp" {
		tmpCache, err := os.MkdirTemp("", "syllago-provmon-chromedp-*")
		if err != nil {
			return nil, "", "", fmt.Errorf("chromedp temp cache: %w", err)
		}
		defer func() { _ = os.RemoveAll(tmpCache) }()
		entry, err := capmon.FetchChromedp(ctx, tmpCache, slug, sanitizeSourceID(rawURL), rawURL)
		if err != nil {
			return nil, "", "", err
		}
		return entry.Raw, "text/html", rawURL, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "syllago-provider-monitor/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("read body: %w", err)
	}
	ct := resp.Header.Get("Content-Type")
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return body, ct, finalURL, nil
}

// isHTMLContentType reports whether the Content-Type header indicates HTML.
// Parameters (e.g. "; charset=utf-8") are stripped before comparison.
func isHTMLContentType(contentType string) bool {
	mimeBase := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	return strings.EqualFold(mimeBase, "text/html") || strings.EqualFold(mimeBase, "application/xhtml+xml")
}

// sanitizeSourceID turns a URL into a filesystem-safe directory name for the
// capmon chromedp cache. URLs contain characters (":", "/", "?", "&", "=")
// that aren't portable across filesystems, so we replace each with "_".
func sanitizeSourceID(url string) string {
	replacer := strings.NewReplacer(
		"://", "_",
		"/", "_",
		"?", "_",
		"&", "_",
		"=", "_",
		":", "_",
	)
	return replacer.Replace(url)
}
