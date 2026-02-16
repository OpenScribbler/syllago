package main

import (
	"fmt"

	"github.com/holdenhewett/romanesco/cli/internal/config"
	"github.com/holdenhewett/romanesco/cli/internal/drift"
	"github.com/holdenhewett/romanesco/cli/internal/output"
	"github.com/holdenhewett/romanesco/cli/internal/scan"
	"github.com/holdenhewett/romanesco/cli/internal/scan/detectors"
	"github.com/spf13/cobra"
)

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Accept current state as baseline",
	Long:  "Runs detectors and saves the current state as the baseline for drift detection, without regenerating context files.",
	RunE:  runBaseline,
}

func init() {
	baselineCmd.Flags().Bool("from-import", false, "Create baseline from imported provider content")
	rootCmd.AddCommand(baselineCmd)
}

func runBaseline(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	nescoDir := config.DirPath(root)

	// Run scan (detectors only, no emit)
	scanner := scan.NewScanner(detectors.AllDetectors()...)
	result := scanner.Run(root)

	if err := drift.SaveBaseline(nescoDir, result.Document); err != nil {
		return fmt.Errorf("saving baseline: %w", err)
	}

	if output.JSON {
		output.Print(map[string]any{
			"sections": len(result.Document.Sections),
			"path":     config.DirPath(root) + "/" + drift.BaselineFileName,
		})
	} else {
		fmt.Fprintf(output.Writer, "Baseline saved with %d sections.\n", len(result.Document.Sections))
		fmt.Fprintf(output.Writer, "  Path: %s/%s\n", nescoDir, drift.BaselineFileName)
		fmt.Fprintln(output.Writer, "  Future `nesco drift` will compare against this baseline.")
	}

	return nil
}
