package main

import (
	"fmt"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var capmonBackfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Populate missing content_hash values in a provider format doc",
	Long: "Fetch each source URI in the provider's FormatDoc and write the SHA-256 " +
		"hash back into the file, preserving comments and key order. By default only " +
		"sources whose content_hash is empty are fetched; use --force to re-baseline " +
		"every source. Sources with fetch_method: chromedp are fetched via a headless " +
		"browser.",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		formatsDir, _ := cmd.Flags().GetString("formats-dir")
		force, _ := cmd.Flags().GetBool("force")

		if provider == "" {
			return fmt.Errorf("--provider is required: specify a provider slug")
		}
		if _, err := capmon.SanitizeSlug(provider); err != nil {
			return fmt.Errorf("invalid --provider: %w", err)
		}
		telemetry.Enrich("provider", provider)

		path := filepath.Join(formatsDir, provider+".yaml")
		fetcher := capmon.DefaultSourceFetcher{}
		result, err := capmon.BackfillFormatDoc(cmd.Context(), path, fetcher, capmon.BackfillOptions{Force: force})
		fmt.Fprintf(output.Writer, "capmon backfill %s: %d sources backfilled\n", provider, result.Updated)
		for _, e := range result.Errors {
			fmt.Fprintf(output.Writer, "  error: %v\n", e)
		}
		return err
	},
}

func init() {
	capmonBackfillCmd.Flags().String("provider", "", "Provider slug to backfill (required)")
	capmonBackfillCmd.Flags().String("formats-dir", "docs/provider-formats", "Directory containing provider format docs")
	capmonBackfillCmd.Flags().Bool("force", false, "Re-fetch and overwrite existing content_hash values")
	capmonCmd.AddCommand(capmonBackfillCmd)
}
