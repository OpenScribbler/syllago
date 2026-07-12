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

	"github.com/OpenScribbler/syllago/cli/internal/capfeed"
)

// DefaultFeedURL is the published Capability Feed index.
const DefaultFeedURL = "https://openscribbler.github.io/capmon/v1/index.json"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point: flag parsing, exit-code mapping, and
// stdout formatting live here; all feed logic lives in internal/capfeed.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capmon-pull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	feedURL := fs.String("feed-url", DefaultFeedURL, "URL of the Capability Feed v1/index.json")
	check := fs.Bool("check", false, "fetch and inspect the feed index only; write nothing")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx := context.Background()

	if *check {
		return runCheck(ctx, *feedURL, stdout, stderr)
	}

	fmt.Fprintln(stderr, "capmon-pull: full pull not implemented yet; use -check")
	return 2
}

// runCheck fetches and parses the feed index, printing its identity fields.
func runCheck(ctx context.Context, feedURL string, stdout, stderr io.Writer) int {
	f := &capfeed.Fetcher{}
	res, err := f.Fetch(ctx, feedURL, "")
	if err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v\n", err)
		return 1
	}
	idx, err := capfeed.ParseIndex(res.Body)
	if err != nil {
		fmt.Fprintf(stderr, "capmon-pull: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "data_revision: %s\n", idx.DataRevision)
	fmt.Fprintf(stdout, "generated_at: %s\n", idx.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(stdout, "files: %d (providers: %d)\n", len(idx.Files), len(idx.Providers))
	return 0
}
