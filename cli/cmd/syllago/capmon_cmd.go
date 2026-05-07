package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
	// Extractor packages self-register via init(). Import for side effects only.
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_go"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_html"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_json"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_json_schema"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_markdown"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_rust"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_toml"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_typescript"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_yaml"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// capmonCapabilitiesDirOverride allows tests to redirect the verify command
// to a temp directory instead of the repo's docs/provider-capabilities/.
var capmonCapabilitiesDirOverride string

// capmonFetchDryRunEntry is the per-provider dry-run summary emitted by
// 'syllago capmon fetch --dry-run'.
// Sources is populated only when --verbose is also set.
type capmonFetchDryRunEntry struct {
	Provider    string               `json:"provider"`
	SourceCount int                  `json:"source_count"`
	Sources     []capmonSourceResult `json:"sources,omitempty"`
}

// capmonSourceResult is a single per-source entry included in JSON+verbose output.
type capmonSourceResult struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Cached bool   `json:"cached"`
}

// capmonFetchLiveEntry is the per-provider live-fetch summary emitted by
// 'syllago capmon fetch' (no --dry-run) in JSON mode.
// Sources is populated only when --verbose is also set.
type capmonFetchLiveEntry struct {
	Provider string               `json:"provider"`
	Fresh    int                  `json:"fresh"`
	Cached   int                  `json:"cached"`
	Errors   int                  `json:"errors"`
	Sources  []capmonSourceResult `json:"sources,omitempty"`
}

var capmonCmd = &cobra.Command{
	Use:   "capmon",
	Short: "Capability monitor pipeline",
	Long:  "Fetch, extract, diff, and report on AI provider capability drift.",
}

var capmonVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Validate provider-capabilities YAML against JSON Schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		stalenessCheck, _ := cmd.Flags().GetBool("staleness-check")
		thresholdHours, _ := cmd.Flags().GetInt("threshold-hours")
		cacheRoot, _ := cmd.Flags().GetString("cache-root")
		migrationWindow, _ := cmd.Flags().GetBool("migration-window")
		if cacheRoot == "" {
			cacheRoot = ".capmon-cache"
		}

		// Staleness check: read last-run.json and open issue if stale or missing.
		if stalenessCheck {
			manifest, err := capmon.ReadLastRunManifest(cacheRoot)
			if err != nil || time.Since(manifest.FinishedAt) > time.Duration(thresholdHours)*time.Hour {
				reason := "last-run.json missing or unreadable"
				if err == nil {
					reason = fmt.Sprintf("last run was %.1f hours ago (threshold: %d)", time.Since(manifest.FinishedAt).Hours(), thresholdHours)
				}
				_, ghErr := capmon.GHRunner("issue", "create",
					"--title", "capmon: pipeline staleness detected",
					"--label", "capmon,staleness",
					"--body", fmt.Sprintf("Capability monitor pipeline appears stale. %s.", reason),
				)
				return ghErr
			}
			return nil
		}

		dir := capmonCapabilitiesDirOverride
		if dir == "" {
			dir = "docs/provider-capabilities"
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // empty dir is valid
			}
			return fmt.Errorf("read capabilities dir: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
				continue
			}
			path := filepath.Join(dir, e.Name())
			// Skip per-content-type seeder specs (e.g. amp-skills.yaml). Those use
			// `provider:` at the top level instead of `slug:` and have no
			// schema_version field. They are an internal capmon artifact, not a
			// canonical capability YAML, so schema validation does not apply.
			// Mirrors the Slug=="" pattern in internal/capmon/generate.go.
			caps, err := capyaml.LoadCapabilityYAML(path)
			if err == nil && caps.Slug == "" {
				continue
			}
			if err := capyaml.ValidateAgainstSchema(path, migrationWindow); err != nil {
				return fmt.Errorf("validate %s: %w", e.Name(), err)
			}
		}
		return nil
	},
}

var capmonFetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch source URLs and update hash cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		sourcesDir, _ := cmd.Flags().GetString("sources-dir")
		cacheRoot, _ := cmd.Flags().GetString("cache-root")

		// Validate slug format first (applies to both dry-run and live paths).
		if provider != "" {
			if _, err := capmon.SanitizeSlug(provider); err != nil {
				return output.NewStructuredError(output.ErrInputInvalid,
					"invalid --provider: "+err.Error(),
					"Provider slugs must be lowercase alphanumeric with hyphens (e.g. claude-code)")
			}
		}
		if sourcesDir == "" {
			sourcesDir = "docs/provider-sources"
		}
		if cacheRoot == "" {
			cacheRoot = ".capmon-cache"
		}

		// Load manifests once — used for provider validation and dry-run output.
		manifests, err := capmon.LoadAllSourceManifests(sourcesDir)
		if err != nil {
			return fmt.Errorf("load source manifests: %w", err)
		}

		// Validate --provider against known slugs (applies to both paths).
		if provider != "" {
			var validSlugs []string
			found := false
			for _, m := range manifests {
				validSlugs = append(validSlugs, m.Slug)
				if m.Slug == provider {
					found = true
				}
			}
			if !found {
				return output.NewStructuredError(output.ErrInputInvalid,
					fmt.Sprintf("unknown provider %q; valid providers: %s", provider, strings.Join(validSlugs, ", ")),
					"Run 'syllago capmon fetch --dry-run' to list available providers")
			}
		}

		if dryRun {
			var entries []capmonFetchDryRunEntry
			for _, m := range manifests {
				if provider != "" && m.Slug != provider {
					continue
				}
				count := 0
				for _, ct := range m.ContentTypes {
					count += len(ct.Sources)
				}
				entry := capmonFetchDryRunEntry{Provider: m.Slug, SourceCount: count}
				if output.Verbose {
					ctNames := make([]string, 0, len(m.ContentTypes))
					for ctName := range m.ContentTypes {
						ctNames = append(ctNames, ctName)
					}
					sort.Strings(ctNames)
					for _, ctName := range ctNames {
						ct := m.ContentTypes[ctName]
						for i, src := range ct.Sources {
							entry.Sources = append(entry.Sources, capmonSourceResult{
								ID:  fmt.Sprintf("%s.%d", ctName, i),
								URL: src.URL,
							})
						}
					}
				}
				entries = append(entries, entry)
			}

			if output.JSON {
				output.Print(entries)
			} else {
				for _, e := range entries {
					fmt.Fprintf(output.Writer, "%s: %d sources (dry run)\n", e.Provider, e.SourceCount)
					if output.Verbose {
						// List source IDs and URLs per content type (sorted for determinism).
						for _, m := range manifests {
							if m.Slug != e.Provider {
								continue
							}
							ctNames := make([]string, 0, len(m.ContentTypes))
							for ctName := range m.ContentTypes {
								ctNames = append(ctNames, ctName)
							}
							sort.Strings(ctNames)
							for _, ctName := range ctNames {
								ct := m.ContentTypes[ctName]
								for i, src := range ct.Sources {
									fmt.Fprintf(output.Writer, "  %s.%d %s\n", ctName, i, src.URL)
								}
							}
						}
					}
				}
			}

			if provider != "" {
				telemetry.Enrich("provider", provider)
			}
			totalDrySources := 0
			for _, e := range entries {
				totalDrySources += e.SourceCount
			}
			telemetry.Enrich("dry_run", true)
			telemetry.Enrich("source_count", totalDrySources)
			telemetry.Enrich("fetch_errors", 0)
			return nil
		}

		// Live fetch: Stage 1 pipeline.
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Progress hint for non-JSON, non-quiet mode.
		if !output.JSON && !output.Quiet {
			if provider != "" {
				fmt.Fprintf(output.Writer, "Fetching %s...\n", provider)
			} else {
				fmt.Fprintf(output.Writer, "Fetching all providers...\n")
			}
		}

		manifest := capmon.RunManifest{
			RunID:     "fetch-cmd",
			Providers: make(map[string]capmon.ProviderStatus),
		}
		opts := capmon.PipelineOptions{
			CacheRoot:          cacheRoot,
			SourceManifestsDir: sourcesDir,
			ProviderFilter:     provider,
		}
		// Per-source progress via callback (verbose non-JSON mode only).
		if output.Verbose && !output.JSON {
			opts.ProgressFn = func(providerSlug, sourceID string, cached bool) {
				indicator := "[changed]"
				if cached {
					indicator = "[cached]"
				}
				fmt.Fprintf(output.Writer, "  %s/%s %s\n", providerSlug, sourceID, indicator)
			}
		}

		if err := capmon.RunFetchStage(ctx, opts, &manifest); err != nil {
			return fmt.Errorf("fetch stage: %w", err)
		}

		var totalErrors int
		var liveEntries []capmonFetchLiveEntry
		for slug, status := range manifest.Providers {
			errCount := len(status.Errors)
			totalErrors += errCount
			// fresh = successfully fetched from network (not from cache).
			fresh := status.SourcesFetched - status.SourcesCacheHit
			entry := capmonFetchLiveEntry{
				Provider: slug,
				Fresh:    fresh,
				Cached:   status.SourcesCacheHit,
				Errors:   errCount,
			}
			// Populate per-source detail for --json --verbose.
			if output.Verbose && output.JSON {
				results := append([]capmon.SourceResult(nil), status.SourceResults...)
				sort.Slice(results, func(i, j int) bool {
					return results[i].SourceID < results[j].SourceID
				})
				for _, sr := range results {
					entry.Sources = append(entry.Sources, capmonSourceResult{
						ID:     sr.SourceID,
						URL:    sr.URL,
						Cached: sr.Cached,
					})
				}
			}
			liveEntries = append(liveEntries, entry)
		}
		sort.Slice(liveEntries, func(i, j int) bool {
			return liveEntries[i].Provider < liveEntries[j].Provider
		})

		if output.JSON {
			output.Print(liveEntries)
		} else if !output.Quiet {
			for _, e := range liveEntries {
				fmt.Fprintf(output.Writer, "%s: %d fresh, %d cached, %d errors\n",
					e.Provider, e.Fresh, e.Cached, e.Errors)
			}
		}

		if provider != "" {
			telemetry.Enrich("provider", provider)
		}
		totalSources := 0
		for _, e := range liveEntries {
			totalSources += e.Fresh + e.Cached
		}
		telemetry.Enrich("dry_run", false)
		telemetry.Enrich("source_count", totalSources)
		telemetry.Enrich("fetch_errors", totalErrors)

		if totalErrors > 0 {
			return fmt.Errorf("fetch completed with %d error(s); see output for details", totalErrors)
		}
		return nil
	},
}

var capmonExtractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Run extraction on cached sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		if provider != "" {
			if _, err := capmon.SanitizeSlug(provider); err != nil {
				return fmt.Errorf("invalid --provider: %w", err)
			}
		}
		return fmt.Errorf("not yet implemented — use 'syllago capmon run --stage fetch-extract'")
	},
}

var capmonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the full capability monitor pipeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		stage, _ := cmd.Flags().GetString("stage")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		provider, _ := cmd.Flags().GetString("provider")

		telemetry.Enrich("dry_run", dryRun)
		if provider != "" {
			telemetry.Enrich("provider", provider)
		}
		mode := "full"
		if stage != "" {
			mode = stage
		}
		telemetry.Enrich("mode", mode)

		opts := capmon.PipelineOptions{
			Stage:          stage,
			DryRun:         dryRun,
			ProviderFilter: provider,
		}
		exitClass, err := capmon.RunPipeline(cmd.Context(), opts)
		if err != nil {
			return err
		}
		os.Exit(exitClass)
		return nil
	},
}

var capmonDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show field-level changes in provider-capabilities since a git ref",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		if provider != "" {
			if _, err := capmon.SanitizeSlug(provider); err != nil {
				return fmt.Errorf("invalid --provider: %w", err)
			}
		}
		// Full implementation wired via pipeline.go in Phase 9
		return fmt.Errorf("diff output: not yet implemented")
	},
}

var capmonGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Regenerate per-content-type views and spec tables from provider-capabilities YAML",
	RunE: func(cmd *cobra.Command, args []string) error {
		capsDir := "docs/provider-capabilities"
		if err := capmon.GenerateContentTypeViews(capsDir, capsDir+"/by-content-type"); err != nil {
			return err
		}
		return capmon.GenerateHooksSpecTables(capsDir, "docs/spec/hooks")
	},
}

var capmonSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Bootstrap or re-seed provider capability YAML from extracted data",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		forceOverwrite, _ := cmd.Flags().GetBool("force-overwrite-exclusive")
		cacheRoot, _ := cmd.Flags().GetString("cache-root")
		if cacheRoot == "" {
			cacheRoot = ".capmon-cache"
		}
		if provider == "" {
			return fmt.Errorf("--provider is required: specify a provider slug to seed")
		}
		if _, err := capmon.SanitizeSlug(provider); err != nil {
			return fmt.Errorf("invalid --provider: %w", err)
		}
		telemetry.Enrich("provider", provider)

		// Load extracted fields from cache and run recognizers.
		var extracted map[string]string
		if provider != "" {
			var err error
			extracted, err = capmon.LoadAndRecognizeCache(cacheRoot, provider)
			if err != nil {
				// Cache may not exist yet — seed with empty extracted (creates bare stub)
				extracted = make(map[string]string)
			}
		}

		opts := capmon.SeedOptions{
			CapsDir:                 "docs/provider-capabilities",
			Provider:                provider,
			Extracted:               extracted,
			ForceOverwriteExclusive: forceOverwrite,
		}
		return capmon.SeedProviderCapabilities(opts)
	},
}

var capmonTestFixturesCmd = &cobra.Command{
	Use:   "test-fixtures",
	Short: "Report fixture staleness or update fixtures for a provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		update, _ := cmd.Flags().GetBool("update")
		provider, _ := cmd.Flags().GetString("provider")

		if update && provider == "" {
			return fmt.Errorf("--update requires --provider: bulk all-provider updates are refused to preserve per-provider audit trail")
		}
		if update {
			if _, err := capmon.SanitizeSlug(provider); err != nil {
				return fmt.Errorf("invalid --provider: %w", err)
			}
			// Full update implementation via FetchSource/FetchChromedp in Phase 10
			return fmt.Errorf("fixture update for %s: not yet implemented", provider)
		}
		// Report fixture ages from git log
		return reportFixtureAges("cli/internal/capmon/testdata/fixtures")
	},
}

func reportFixtureAges(fixturesDir string) error {
	fmt.Printf("Fixture directory: %s\n", fixturesDir)
	fmt.Printf("Run 'git log --format=%%cr -- <fixture-file>' for per-file ages\n")
	return nil
}

func init() {
	capmonVerifyCmd.Flags().Bool("staleness-check", false, "Check last-run.json age and open issue if stale")
	capmonVerifyCmd.Flags().Int("threshold-hours", 36, "Hours before a run is considered stale (used with --staleness-check)")
	capmonVerifyCmd.Flags().String("cache-root", "", "Path to .capmon-cache/ (default: .capmon-cache)")
	capmonVerifyCmd.Flags().Bool("migration-window", false, "Accept current-minus-one schema_version during schema migrations")

	capmonFetchCmd.Flags().String("provider", "", "Fetch only this provider slug")
	capmonFetchCmd.Flags().Bool("dry-run", false, "Report source counts without fetching or writing cache")
	capmonFetchCmd.Flags().String("sources-dir", "", "Path to provider-sources/ (default: docs/provider-sources)")
	capmonFetchCmd.Flags().String("cache-root", "", "Path to .capmon-cache/ (default: .capmon-cache)")

	capmonExtractCmd.Flags().String("provider", "", "Extract only this provider slug")

	capmonRunCmd.Flags().String("stage", "", "Pipeline stage to run: 'fetch-extract' or 'report' (default: all stages)")
	capmonRunCmd.Flags().Bool("dry-run", false, "Skip Stage 4 PR/issue creation; write report to stdout")
	capmonRunCmd.Flags().String("provider", "", "Limit to this provider slug")

	capmonDiffCmd.Flags().String("provider", "", "Limit diff to this provider slug")
	capmonDiffCmd.Flags().String("since", "", "Git ref to diff against (default: HEAD~1)")

	capmonSeedCmd.Flags().String("provider", "", "Seed only this provider slug")
	capmonSeedCmd.Flags().Bool("force-overwrite-exclusive", false, "Allow overwriting provider_exclusive entries (prints warning)")
	capmonSeedCmd.Flags().String("cache-root", "", "Path to .capmon-cache/ (default: .capmon-cache)")

	capmonTestFixturesCmd.Flags().Bool("update", false, "Re-fetch live source and update fixture files")
	capmonTestFixturesCmd.Flags().String("provider", "", "Provider slug for --update (required with --update)")

	capmonCmd.AddCommand(capmonVerifyCmd)
	capmonCmd.AddCommand(capmonFetchCmd)
	capmonCmd.AddCommand(capmonExtractCmd)
	capmonCmd.AddCommand(capmonRunCmd)
	capmonCmd.AddCommand(capmonDiffCmd)
	capmonCmd.AddCommand(capmonGenerateCmd)
	capmonCmd.AddCommand(capmonSeedCmd)
	capmonCmd.AddCommand(capmonTestFixturesCmd)
}
