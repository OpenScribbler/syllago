package main

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var capmonCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for content drift in provider format docs",
	Long: "Fetch each source URI in provider format docs, compare content hashes, " +
		"and create or append GitHub issues when content has changed.",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		provider, _ := cmd.Flags().GetString("provider")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		formatsDir, _ := cmd.Flags().GetString("formats-dir")
		sourcesDir, _ := cmd.Flags().GetString("sources-dir")
		cacheRoot, _ := cmd.Flags().GetString("cache-root")
		providersJSON, _ := cmd.Flags().GetString("providers-json")
		canonicalKeys, _ := cmd.Flags().GetString("canonical-keys")

		// Mutual exclusion: --all and --provider are incompatible.
		if all && provider != "" {
			return fmt.Errorf("--all and --provider are mutually exclusive")
		}
		if !all && provider == "" {
			return fmt.Errorf("one of --all or --provider is required")
		}

		if provider != "" {
			if _, err := capmon.SanitizeSlug(provider); err != nil {
				return fmt.Errorf("invalid --provider: %w", err)
			}
			telemetry.Enrich("provider", provider)
		}
		telemetry.Enrich("dry_run", dryRun)

		opts := capmon.CapmonCheckOptions{
			ProvidersJSON:     providersJSON,
			FormatsDir:        formatsDir,
			SourcesDir:        sourcesDir,
			CacheRoot:         cacheRoot,
			CanonicalKeysPath: canonicalKeys,
			ProviderFilter:    provider,
			DryRun:            dryRun,
		}
		return capmon.RunCapmonCheck(cmd.Context(), opts)
	},
}

func init() {
	capmonCheckCmd.Flags().Bool("all", false, "Check all providers")
	capmonCheckCmd.Flags().String("provider", "", "Check only this provider slug")
	capmonCheckCmd.Flags().Bool("dry-run", false, "Log actions without creating GitHub issues")
	capmonCheckCmd.Flags().String("formats-dir", "docs/provider-formats", "Directory containing provider format docs")
	capmonCheckCmd.Flags().String("sources-dir", "docs/provider-sources", "Directory containing provider source manifests")
	capmonCheckCmd.Flags().String("cache-root", ".capmon-cache", "Root directory for capmon cache")
	capmonCheckCmd.Flags().String("providers-json", "providers.json", "Path to providers.json")
	capmonCheckCmd.Flags().String("canonical-keys", "docs/spec/canonical-keys.yaml", "Path to canonical-keys.yaml")

	capmonCmd.AddCommand(capmonCheckCmd)
}
