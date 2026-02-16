package main

import (
	"fmt"
	"os"

	"github.com/holdenhewett/romanesco/cli/internal/config"
	"github.com/holdenhewett/romanesco/cli/internal/drift"
	"github.com/holdenhewett/romanesco/cli/internal/output"
	"github.com/holdenhewett/romanesco/cli/internal/scan"
	"github.com/holdenhewett/romanesco/cli/internal/scan/detectors"
	"github.com/spf13/cobra"
)

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Compare current codebase state against baseline",
	Long:  "Re-runs detectors and compares results against the stored baseline. Reports new, changed, and removed sections.",
	RunE:  runDrift,
}

func init() {
	driftCmd.Flags().Bool("ci", false, "CI mode — exit code 3 if drift detected")
	rootCmd.AddCommand(driftCmd)
}

func runDrift(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	nescoDir := config.DirPath(root)
	if !drift.BaselineExists(nescoDir) {
		output.PrintError(1, "no baseline found", "Run `nesco scan` first to create a baseline")
		return fmt.Errorf("no baseline")
	}

	baseline, err := drift.LoadBaseline(nescoDir)
	if err != nil {
		return err
	}

	// Run fresh scan
	scanner := scan.NewScanner(detectors.AllDetectors()...)
	result := scanner.Run(root)

	// Create current baseline from scan results (in-memory, not saved)
	currentBaseline := drift.BaselineFromDocument(result.Document)

	report := drift.Diff(baseline, currentBaseline)

	if output.JSON {
		output.Print(report)
	} else {
		if report.Clean {
			fmt.Fprintln(output.Writer, "No drift detected. Baseline is current.")
		} else {
			if len(report.Changed) > 0 {
				fmt.Fprintf(output.Writer, "Changed (%d):\n", len(report.Changed))
				for _, item := range report.Changed {
					fmt.Fprintf(output.Writer, "  ~ %s: %s\n", item.Category, item.Title)
				}
			}
			if len(report.New) > 0 {
				fmt.Fprintf(output.Writer, "New (%d):\n", len(report.New))
				for _, item := range report.New {
					fmt.Fprintf(output.Writer, "  + %s: %s\n", item.Category, item.Title)
				}
			}
			if len(report.Removed) > 0 {
				fmt.Fprintf(output.Writer, "Removed (%d):\n", len(report.Removed))
				for _, item := range report.Removed {
					fmt.Fprintf(output.Writer, "  - %s: %s\n", item.Category, item.Title)
				}
			}
		}
	}

	ciMode, _ := cmd.Flags().GetBool("ci")
	if ciMode && !report.Clean {
		os.Exit(3)
	}

	return nil
}
