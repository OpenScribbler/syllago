package capmon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OnboardOptions configures an OnboardProvider run.
type OnboardOptions struct {
	// Provider is the slug of the provider being onboarded (required).
	Provider string
	// SourcesDir is the directory containing provider source manifests.
	SourcesDir string
	// FormatsDir is the directory containing provider format docs (unused in
	// onboard but accepted for parity with check options).
	FormatsDir string
	// CacheRoot is the root of the capmon cache.
	CacheRoot string
	// DryRun logs actions but makes no GitHub API calls.
	DryRun bool
}

// OnboardProvider bootstraps a new provider into the capmon pipeline.
// It assumes ValidateSources has already been called by the caller (the cobra
// command does this before invoking OnboardProvider).
//
// For each supported content type with at least one source:
//  1. Fetch each source URI via FetchSource to populate the cache baseline.
//  2. Create a GitHub issue recording the initial content hashes.
//
// Fetch errors are non-blocking — the source is noted in the issue body but
// does not abort the run.
func OnboardProvider(ctx context.Context, opts OnboardOptions) error {
	if opts.SourcesDir == "" {
		opts.SourcesDir = "docs/provider-sources"
	}
	if opts.CacheRoot == "" {
		opts.CacheRoot = ".capmon-cache"
	}

	manifestPath := filepath.Join(opts.SourcesDir, opts.Provider+".yaml")
	manifest, err := LoadSourceManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("onboard: load source manifest: %w", err)
	}

	for ct, ctSource := range manifest.ContentTypes {
		// Skip explicitly unsupported content types.
		if ctSource.Supported != nil && !*ctSource.Supported {
			continue
		}
		if len(ctSource.Sources) == 0 {
			continue
		}

		// Fetch each source, collecting results for the issue body.
		var lines []string
		for i, src := range ctSource.Sources {
			sourceID := fmt.Sprintf("%s-%d", ct, i)
			entry, fetchErr := FetchSource(ctx, opts.CacheRoot, opts.Provider, sourceID, src.URL)
			if fetchErr != nil {
				lines = append(lines, fmt.Sprintf("- %s: FETCH ERROR: %v", src.URL, fetchErr))
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s (hash: %s)", src.URL, entry.Meta.ContentHash))
		}

		title := fmt.Sprintf("capmon: initial content captured for %s/%s", opts.Provider, ct)
		body := fmt.Sprintf("Provider `%s` onboarded for content type `%s`.\n\nSources:\n%s",
			opts.Provider, ct, strings.Join(lines, "\n"))

		if opts.DryRun {
			fmt.Fprintf(os.Stderr, "dry-run: would create capmon onboard issue for %s/%s\n",
				opts.Provider, ct)
			continue
		}

		if _, err := CreateCapmonChangeIssue(ctx, opts.Provider, ct, title, body); err != nil {
			return fmt.Errorf("onboard: create issue for %s/%s: %w", opts.Provider, ct, err)
		}
	}

	return nil
}
