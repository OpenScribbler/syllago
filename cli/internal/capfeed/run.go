package capfeed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options configures a Capmon Pull run. All network endpoints are
// substitutable (FeedURL is explicit; the attestations API sits behind the
// package base-URL var) and the clock is injectable.
type Options struct {
	// FeedURL is the Capability Feed's v1/index.json URL.
	FeedURL string
	// RepoRoot is the syllago repo root containing docs/provider-capabilities.
	RepoRoot string
	// ETagFile persists the index ETag between runs (best-effort politeness;
	// empty disables persistence).
	ETagFile string
	// SummaryFile receives the machine-readable run summary consumed by the
	// cron workflow (empty disables).
	SummaryFile string
	// TrustedRootJSON is the Sigstore trusted root (the caller passes
	// moat.BundledTrustedRoot bytes; capfeed never imports moat).
	TrustedRootJSON []byte
	// CheckOnly stops after verification + freshness: nothing is written.
	CheckOnly bool
	// Now is the clock; nil → time.Now.
	Now func() time.Time
}

// Summary is the run outcome, serialized to Options.SummaryFile for the
// workflow to build the rolling PR from.
type Summary struct {
	Changed          bool      `json:"changed"`
	DataRevision     string    `json:"data_revision"`
	GeneratedAt      time.Time `json:"generated_at"`
	ChangedProviders []string  `json:"changed_providers"`
}

// mirrorExcluded names index-listed files deliberately not mirrored.
// advisories.json is out of scope for Capmon Pull: not mirrored, not acted
// on.
var mirrorExcluded = map[string]bool{"advisories.json": true}

// Run is the complete Capmon Pull pipeline with fail-closed ordering:
//
//	fetch index (conditional GET) → 304 short-circuit
//	→ parse shape → fetch attestation → verify provenance → freshness
//	→ data_revision marker short-circuit (only AFTER verification)
//	→ fetch + sha256-verify every file → write mirror
//	→ persist ETag → write summary
//
// Any failure at any step writes nothing and returns an error; the caller
// exits non-zero and the last-known-good mirror stays untouched.
func Run(ctx context.Context, opts Options) (*Summary, error) {
	if opts.FeedURL == "" {
		return nil, errors.New("capfeed run: FeedURL is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	f := &Fetcher{}

	prevETag := readETag(opts.ETagFile)
	res, err := f.Fetch(ctx, opts.FeedURL, prevETag)
	if err != nil {
		return nil, err
	}

	// 304: the exact bytes we last processed. Nothing to verify, nothing to
	// write — the polite no-op.
	if res.NotModified {
		sum := &Summary{Changed: false}
		if err := writeSummary(opts.SummaryFile, sum); err != nil {
			return nil, err
		}
		return sum, nil
	}

	// Shape check only — parsing untrusted bytes in memory is safe; acting
	// on them before verification is not.
	idx, err := ParseIndex(res.Body)
	if err != nil {
		return nil, err
	}

	digest := sha256.Sum256(res.Body)
	bundles, err := FetchAttestationBundle(ctx, nil, hex.EncodeToString(digest[:]))
	if err != nil {
		return nil, err
	}
	if err := VerifyFeedProvenance(res.Body, bundles, opts.TrustedRootJSON); err != nil {
		return nil, fmt.Errorf("provenance verification failed (writing nothing): %w", err)
	}

	// generated_at is trusted only now, after verification (ADR 0012).
	if err := CheckFreshness(idx.GeneratedAt, now(), idx.MaxStalenessHours); err != nil {
		return nil, err
	}

	if opts.CheckOnly {
		return &Summary{Changed: false, DataRevision: idx.DataRevision, GeneratedAt: idx.GeneratedAt}, nil
	}

	capDir := filepath.Join(opts.RepoRoot, "docs", "provider-capabilities")

	// Marker comparison runs strictly after verification so an attacker
	// cannot use a replayed data_revision to skip the signature check.
	marker, err := ReadMarker(filepath.Join(capDir, MarkerFileName))
	if err != nil {
		return nil, err
	}
	if marker.DataRevision == idx.DataRevision {
		sum := &Summary{Changed: false, DataRevision: idx.DataRevision, GeneratedAt: idx.GeneratedAt}
		if err := persistETag(opts.ETagFile, res.ETag); err != nil {
			return nil, err
		}
		if err := writeSummary(opts.SummaryFile, sum); err != nil {
			return nil, err
		}
		return sum, nil
	}

	var toMirror []IndexFile
	for _, file := range idx.Files {
		if mirrorExcluded[file.Path] {
			continue
		}
		toMirror = append(toMirror, file)
	}
	files, err := FetchFeedFiles(ctx, f, baseURLOf(opts.FeedURL), toMirror)
	if err != nil {
		return nil, fmt.Errorf("%w (writing nothing)", err)
	}

	mres, err := WriteMirror(capDir, idx, files)
	if err != nil {
		return nil, err
	}

	if err := persistETag(opts.ETagFile, res.ETag); err != nil {
		return nil, err
	}
	sum := &Summary{
		Changed:          true,
		DataRevision:     idx.DataRevision,
		GeneratedAt:      idx.GeneratedAt,
		ChangedProviders: mres.ChangedProviders,
	}
	if err := writeSummary(opts.SummaryFile, sum); err != nil {
		return nil, err
	}
	return sum, nil
}

// baseURLOf strips the final path segment: the feed's files resolve
// relative to the index's directory.
func baseURLOf(feedURL string) string {
	if i := strings.LastIndex(feedURL, "/"); i >= 0 {
		return feedURL[:i+1]
	}
	return feedURL
}

func readETag(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func persistETag(path, etag string) error {
	if path == "" || etag == "" {
		return nil
	}
	if err := os.WriteFile(path, []byte(etag+"\n"), 0o644); err != nil {
		return fmt.Errorf("capfeed run: persisting ETag: %w", err)
	}
	return nil
}

func writeSummary(path string, sum *Summary) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(sum, "", "  ")
	if err != nil {
		return fmt.Errorf("capfeed run: encoding summary: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("capfeed run: writing summary: %w", err)
	}
	return nil
}
