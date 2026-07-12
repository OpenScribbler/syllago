// Capmon Pull: the maintainer tool that verifies the Capability Feed
// fail-closed and mirrors it into docs/provider-capabilities/.
//
// Not shipped to end users — invoked from .github/workflows/capmon-pull.yml
// and locally via `go run ./cmd/capmon-pull` (syllago-sign precedent,
// ADR 0017). Never added to build-all, release artifacts, or commands.json.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
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

// run is the thin flag-to-Options shim (coverage-exempt wiring); the entire
// pipeline lives in capfeed.Run.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capmon-pull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	feedURL := fs.String("feed-url", DefaultFeedURL, "URL of the Capability Feed v1/index.json")
	check := fs.Bool("check", false, "fetch, verify, and inspect the feed index only; write nothing")
	repoRoot := fs.String("repo-root", ".", "syllago repo root containing docs/provider-capabilities")
	etagFile := fs.String("etag-file", "", "file persisting the index ETag between runs (optional)")
	summaryFile := fs.String("summary-file", "", "file receiving the machine-readable run summary JSON (optional)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	sum, err := capfeed.Run(context.Background(), capfeed.Options{
		FeedURL:         *feedURL,
		RepoRoot:        *repoRoot,
		ETagFile:        *etagFile,
		SummaryFile:     *summaryFile,
		TrustedRootJSON: moat.BundledTrustedRoot(nowFunc()).Bytes,
		CheckOnly:       *check,
		Now:             nowFunc,
	})
	if err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v\n", err)
		return 1
	}

	if sum.DataRevision != "" {
		fmt.Fprintf(stdout, "data_revision: %s\n", sum.DataRevision)
		fmt.Fprintf(stdout, "generated_at: %s\n", sum.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if !*check {
		fmt.Fprintf(stdout, "changed: %t\n", sum.Changed)
		if len(sum.ChangedProviders) > 0 {
			fmt.Fprintf(stdout, "changed_providers: %s\n", strings.Join(sum.ChangedProviders, ", "))
		}
	}
	return 0
}
