package main

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// Override vars for test redirectability.
var (
	capmonDeriveFomatsDirOverride    string
	capmonDeriveOutputDirOverride    string
	capmonDeriveCanonicalKeyOverride string
)

var capmonDeriveCmd = &cobra.Command{
	Use:   "derive",
	Short: "Derive a seeder spec from a provider format doc",
	Long: `Deterministically derive a seeder spec YAML from a provider format doc.

The derived spec is written to --output-dir/<provider>-<content_type>.yaml.
All canonical_mappings keys are validated against --canonical-keys.
Content types with status=unsupported are omitted from output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		formatsDir, _ := cmd.Flags().GetString("formats-dir")
		outputDir, _ := cmd.Flags().GetString("output-dir")
		canonicalKeys, _ := cmd.Flags().GetString("canonical-keys")

		if provider == "" {
			return fmt.Errorf("--provider is required: specify a provider slug")
		}

		telemetry.Enrich("provider", provider)

		if capmonDeriveFomatsDirOverride != "" {
			formatsDir = capmonDeriveFomatsDirOverride
		}
		if capmonDeriveOutputDirOverride != "" {
			outputDir = capmonDeriveOutputDirOverride
		}
		if capmonDeriveCanonicalKeyOverride != "" {
			canonicalKeys = capmonDeriveCanonicalKeyOverride
		}

		doc, err := capmon.LoadFormatDoc(capmon.FormatDocPath(formatsDir, provider))
		if err != nil {
			return fmt.Errorf("load format doc: %w", err)
		}

		specs, err := capmon.DeriveSeederSpecs(doc, canonicalKeys)
		if err != nil {
			return err
		}

		for _, spec := range specs {
			outPath := capmon.SeederSpecPath(outputDir, provider, spec.ContentType)
			if err := capmon.WriteSeederSpec(spec, outPath); err != nil {
				return fmt.Errorf("write seeder spec for %q: %w", spec.ContentType, err)
			}
			fmt.Printf("✓ Derived seeder spec for %q (%s) → %s\n", provider, spec.ContentType, outPath)
		}
		return nil
	},
}

func init() {
	capmonDeriveCmd.Flags().String("provider", "", "Provider slug to derive spec for (required)")
	capmonDeriveCmd.Flags().String("formats-dir", "docs/provider-formats", "Directory containing provider format docs")
	capmonDeriveCmd.Flags().String("output-dir", ".develop/seeder-specs", "Directory to write the derived seeder spec")
	capmonDeriveCmd.Flags().String("canonical-keys", "docs/spec/canonical-keys.yaml", "Path to canonical-keys.yaml")
	capmonCmd.AddCommand(capmonDeriveCmd)
}
