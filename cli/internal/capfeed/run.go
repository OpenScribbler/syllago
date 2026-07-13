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
	// ETagFile persists the index ETag plus the last verified index's
	// freshness fields between runs (best-effort politeness; empty disables
	// persistence — and with it the conditional GET).
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
//	fetch index (conditional GET) → 304 short-circuit (freshness-gated)
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

	state := readETagState(opts.ETagFile)
	prevETag := ""
	if state != nil {
		prevETag = state.ETag
	}
	res, err := f.Fetch(ctx, opts.FeedURL, prevETag)
	if err != nil {
		return nil, err
	}

	// 304: the exact bytes we last processed. Nothing to verify, nothing to
	// write — the polite no-op. The freshness gate still runs, against the
	// state persisted with the ETag: an endlessly replayed 304 must not
	// keep the cron green while the feed goes stale (syllago-u9s3l).
	if res.NotModified {
		if state == nil {
			return nil, errors.New("capfeed run: server returned 304 Not Modified to a request without If-None-Match (writing nothing)")
		}
		if err := CheckFreshness(state.GeneratedAt, now(), state.MaxStalenessHours); err != nil {
			return nil, err
		}
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
		if err := persistETagState(opts.ETagFile, res.ETag, idx); err != nil {
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

	if err := persistETagState(opts.ETagFile, res.ETag, idx); err != nil {
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

// etagState is the between-runs conditional-GET state. GeneratedAt and
// MaxStalenessHours are captured from the last verified index so the 304
// path — which has no body to read them from — can still run the
// freshness gate.
type etagState struct {
	ETag              string    `json:"etag"`
	GeneratedAt       time.Time `json:"generated_at"`
	MaxStalenessHours int       `json:"max_staleness_hours"`
}

// readETagState loads the persisted state. Anything unusable — missing
// file, non-JSON (including the retired plain-text ETag format), or a
// record missing any field the 304 path needs — yields nil: the run sends
// an unconditional GET and verifies from scratch.
func readETagState(path string) *etagState {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s etagState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	if s.ETag == "" || s.GeneratedAt.IsZero() || s.MaxStalenessHours <= 0 {
		return nil
	}
	return &s
}

func persistETagState(path, etag string, idx *Index) error {
	if path == "" {
		return nil
	}
	if etag == "" {
		// A verified 200 without an ETag supersedes whatever is on disk;
		// leaving the old record would let a later 304 validate against an
		// index we no longer hold.
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("capfeed run: clearing superseded ETag state: %w", err)
		}
		return nil
	}
	data, err := json.Marshal(etagState{
		ETag:              etag,
		GeneratedAt:       idx.GeneratedAt,
		MaxStalenessHours: idx.MaxStalenessHours,
	})
	if err != nil {
		return fmt.Errorf("capfeed run: encoding ETag state: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("capfeed run: persisting ETag state: %w", err)
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
