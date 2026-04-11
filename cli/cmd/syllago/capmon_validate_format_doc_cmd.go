package main

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// Override vars for test redirectability.
var (
	capmonFormatDocsDirOverride    string
	capmonCanonicalKeysDirOverride string
)

var capmonValidateFormatDocCmd = &cobra.Command{
	Use:   "validate-format-doc",
	Short: "Validate a provider format doc against the canonical keys vocabulary",
	Long: `Validate a provider's docs/provider-formats/<slug>.yaml file.

Checks:
  - Required top-level fields (provider, last_fetched_at, content_types)
  - All canonical_mappings keys exist in canonical-keys.yaml
  - All provider_extensions entries have required fields (id, name, description, source_ref)
  - confidence values are valid (confirmed | inferred | unknown)

Informational fields (generation_method, notes) are not validated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		formatsDir, _ := cmd.Flags().GetString("formats-dir")
		canonicalKeys, _ := cmd.Flags().GetString("canonical-keys")

		if provider == "" {
			return fmt.Errorf("--provider is required: specify a provider slug to validate")
		}

		telemetry.Enrich("provider", provider)

		if capmonFormatDocsDirOverride != "" {
			formatsDir = capmonFormatDocsDirOverride
		}
		if capmonCanonicalKeysDirOverride != "" {
			canonicalKeys = capmonCanonicalKeysDirOverride
		}

		if err := capmon.ValidateFormatDoc(formatsDir, canonicalKeys, provider); err != nil {
			return err
		}
		fmt.Printf("✓ Schema valid\n✓ All checks passed for provider %q\n", provider)
		return nil
	},
}

func init() {
	capmonValidateFormatDocCmd.Flags().String("provider", "", "Provider slug whose format doc to validate (required)")
	capmonValidateFormatDocCmd.Flags().String("formats-dir", "docs/provider-formats", "Directory containing provider format docs")
	capmonValidateFormatDocCmd.Flags().String("canonical-keys", "docs/spec/canonical-keys.yaml", "Path to canonical-keys.yaml")
	capmonCmd.AddCommand(capmonValidateFormatDocCmd)
}
