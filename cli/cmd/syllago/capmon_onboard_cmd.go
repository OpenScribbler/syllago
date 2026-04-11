package main

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// capmonOnboardSourcesDirOverride allows tests to redirect the onboard command
// to a temporary sources directory.
var capmonOnboardSourcesDirOverride string

var capmonOnboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Bootstrap a new provider into the capmon pipeline",
	Long: "Validates source manifests, fetches all sources, and creates GitHub issues " +
		"for the new provider. Use this when adding a provider for the first time — " +
		"there is no prior content_hash baseline, so all sources are treated as changed.",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		sourcesDir, _ := cmd.Flags().GetString("sources-dir")
		formatsDir, _ := cmd.Flags().GetString("formats-dir")
		cacheRoot, _ := cmd.Flags().GetString("cache-root")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if provider == "" {
			return fmt.Errorf("--provider is required")
		}
		if _, err := capmon.SanitizeSlug(provider); err != nil {
			return fmt.Errorf("invalid --provider: %w", err)
		}

		// Apply test override.
		if capmonOnboardSourcesDirOverride != "" {
			sourcesDir = capmonOnboardSourcesDirOverride
		}

		telemetry.Enrich("provider", provider)
		telemetry.Enrich("dry_run", dryRun)

		// Step 1: Validate source manifest.
		if err := capmon.ValidateSources(sourcesDir, provider); err != nil {
			return fmt.Errorf("capmon onboard: %w", err)
		}

		// Step 2 & 3: Fetch all sources and create issues per content type.
		return capmon.OnboardProvider(cmd.Context(), capmon.OnboardOptions{
			Provider:   provider,
			SourcesDir: sourcesDir,
			FormatsDir: formatsDir,
			CacheRoot:  cacheRoot,
			DryRun:     dryRun,
		})
	},
}

func init() {
	capmonOnboardCmd.Flags().String("provider", "", "Provider slug to onboard (required)")
	capmonOnboardCmd.Flags().String("sources-dir", "docs/provider-sources", "Directory containing provider source manifests")
	capmonOnboardCmd.Flags().String("formats-dir", "docs/provider-formats", "Directory containing provider format docs")
	capmonOnboardCmd.Flags().String("cache-root", ".capmon-cache", "Root directory for capmon cache")
	capmonOnboardCmd.Flags().Bool("dry-run", false, "Log actions without creating GitHub issues")

	capmonCmd.AddCommand(capmonOnboardCmd)
}
