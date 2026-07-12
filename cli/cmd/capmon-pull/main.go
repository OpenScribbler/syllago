// Capmon Pull: the maintainer tool that verifies the Capability Feed
// fail-closed and mirrors it into docs/provider-capabilities/.
//
// Not shipped to end users — invoked from .github/workflows/capmon-pull.yml
// and locally via `go run ./cmd/capmon-pull` (syllago-sign precedent,
// ADR 0017). Never added to build-all, release artifacts, or commands.json.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capfeed"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// DefaultFeedURL is the published Capability Feed index.
const DefaultFeedURL = "https://openscribbler.github.io/capmon/v1/index.json"

// nowFunc is the tool's clock; a var so tests can pin it relative to a
// recorded feed snapshot's generated_at.
var nowFunc = time.Now

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point: flag parsing, exit-code mapping, and
// stdout formatting live here; all feed logic lives in internal/capfeed.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capmon-pull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	feedURL := fs.String("feed-url", DefaultFeedURL, "URL of the Capability Feed v1/index.json")
	check := fs.Bool("check", false, "fetch, verify, and inspect the feed index only; write nothing")
	repoRoot := fs.String("repo-root", ".", "syllago repo root containing docs/provider-capabilities")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx := context.Background()

	if *check {
		return runCheck(ctx, *feedURL, stdout, stderr)
	}
	return runPull(ctx, *feedURL, *repoRoot, stdout, stderr)
}

// mirrorExcluded names index-listed files that are deliberately not
// mirrored. advisories.json is out of scope for Capmon Pull: not mirrored,
// not acted on.
var mirrorExcluded = map[string]bool{"advisories.json": true}

// runPull is the full pipeline: verify everything, then mirror. Ordering is
// fail-closed — fetch index → parse shape → fetch attestation → verify
// provenance → freshness → fetch + sha256-verify every file → only then
// write. Any failure writes nothing and exits non-zero.
func runPull(ctx context.Context, feedURL, repoRoot string, stdout, stderr io.Writer) int {
	f := &capfeed.Fetcher{}
	idx, _, code := fetchVerifiedIndex(ctx, f, feedURL, stderr)
	if code != 0 {
		return code
	}

	var toMirror []capfeed.IndexFile
	for _, file := range idx.Files {
		if mirrorExcluded[file.Path] {
			continue
		}
		toMirror = append(toMirror, file)
	}

	files, err := capfeed.FetchFeedFiles(ctx, f, baseURLOf(feedURL), toMirror)
	if err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v (writing nothing)\n", err)
		return 1
	}

	capDir := filepath.Join(repoRoot, "docs", "provider-capabilities")
	res, err := capfeed.WriteMirror(capDir, idx, files)
	if err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "data_revision: %s\n", idx.DataRevision)
	fmt.Fprintf(stdout, "generated_at: %s\n", idx.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(stdout, "written: %d, removed: %d, changed providers: %s\n",
		len(res.Written), len(res.Removed), strings.Join(res.ChangedProviders, ", "))
	return 0
}

// fetchVerifiedIndex runs the shared fail-closed gate: fetch, shape-parse,
// attest, verify provenance, freshness. Returns the parsed index and its
// exact bytes only when every gate passed; otherwise a non-zero exit code.
func fetchVerifiedIndex(ctx context.Context, f *capfeed.Fetcher, feedURL string, stderr io.Writer) (*capfeed.Index, []byte, int) {
	res, err := f.Fetch(ctx, feedURL, "")
	if err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v\n", err)
		return nil, nil, 1
	}

	// Shape check only — parsing untrusted bytes in memory is safe; acting
	// on them before verification is not. A malformed index fails here,
	// before any attestation round-trip.
	idx, err := capfeed.ParseIndex(res.Body)
	if err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v\n", err)
		return nil, nil, 1
	}

	digest := sha256.Sum256(res.Body)
	bundles, err := capfeed.FetchAttestationBundle(ctx, nil, hex.EncodeToString(digest[:]))
	if err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v\n", err)
		return nil, nil, 1
	}

	rootInfo := moat.BundledTrustedRoot(nowFunc())
	if err := capfeed.VerifyFeedProvenance(res.Body, bundles, rootInfo.Bytes); err != nil {
		fmt.Fprintf(stderr, "capmon-pull: provenance verification failed (writing nothing): %v\n", err)
		return nil, nil, 1
	}

	// generated_at is trusted only now, after verification (ADR 0012).
	if err := capfeed.CheckFreshness(idx.GeneratedAt, nowFunc(), idx.MaxStalenessHours); err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v\n", err)
		return nil, nil, 1
	}
	return idx, res.Body, 0
}

// baseURLOf strips the final path segment: the feed's files resolve
// relative to the index's directory.
func baseURLOf(feedURL string) string {
	if i := strings.LastIndex(feedURL, "/"); i >= 0 {
		return feedURL[:i+1]
	}
	return feedURL
}

// runCheck fetches the feed index and refuses to trust it until its SLSA
// provenance verifies. Nothing from an unverified or stale feed reaches
// stdout.
func runCheck(ctx context.Context, feedURL string, stdout, stderr io.Writer) int {
	idx, _, code := fetchVerifiedIndex(ctx, &capfeed.Fetcher{}, feedURL, stderr)
	if code != 0 {
		return code
	}
	fmt.Fprintf(stdout, "data_revision: %s\n", idx.DataRevision)
	fmt.Fprintf(stdout, "generated_at: %s\n", idx.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(stdout, "files: %d (providers: %d)\n", len(idx.Files), len(idx.Providers))
	return 0
}
