package main

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// capmonSourcesDirOverride allows tests to redirect the validate-sources command
// to a temp directory instead of the real docs/provider-sources path.
var capmonSourcesDirOverride string

var capmonValidateSourcesCmd = &cobra.Command{
	Use:   "validate-sources",
	Short: "Validate a provider's source manifest has URIs for all supported content types",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		sourcesDir, _ := cmd.Flags().GetString("sources-dir")

		if provider == "" {
			return fmt.Errorf("--provider is required: specify a provider slug to validate")
		}

		telemetry.Enrich("provider", provider)

		if capmonSourcesDirOverride != "" {
			sourcesDir = capmonSourcesDirOverride
		}

		if err := capmon.ValidateSources(sourcesDir, provider); err != nil {
			return err
		}
		fmt.Printf("✓ Source manifest valid for provider %q\n", provider)
		return nil
	},
}

func init() {
	capmonValidateSourcesCmd.Flags().String("provider", "", "Provider slug whose source manifest to validate (required)")
	capmonValidateSourcesCmd.Flags().String("sources-dir", "docs/provider-sources", "Directory containing provider source manifests")
	capmonCmd.AddCommand(capmonValidateSourcesCmd)
}
