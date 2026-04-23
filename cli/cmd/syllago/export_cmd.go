package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:    "export <name>",
	Short:  "Export content with optional dual-format (canonical + native) output",
	Hidden: true,
	Long: `Exports a library item to canonical format, optionally generating
provider-native configs alongside it for manual consumption.

With --dual, the output directory contains both the canonical hook.json
(source of truth) and pre-generated native configs in native/<provider>/
for users who don't have syllago installed.`,
	Example: `  # Export a hook with dual-format output
  syllago export my-hook --dual

  # Export for specific providers only
  syllago export my-hook --dual --providers claude-code,cursor,gemini-cli

  # Export to a specific directory
  syllago export my-hook --dual --out-dir ./hooks/safety-check`,
	Args: cobra.ExactArgs(1),
	RunE: runExport,
}

func init() {
	exportCmd.Flags().Bool("dual", false, "Generate native configs alongside canonical")
	exportCmd.Flags().String("providers", "", "Comma-separated list of target providers for --dual (default: all portable)")
	exportCmd.Flags().String("out-dir", "", "Output directory (default: current hook directory)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	// TODO: implement export logic
	// 1. Find item in library
	// 2. If --dual: for each provider, run conversion pipeline
	// 3. Write canonical + native/<provider>/hooks.json + README.md
	return fmt.Errorf("export is not yet implemented")
}
