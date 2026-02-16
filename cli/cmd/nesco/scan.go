package main

import (
	"fmt"

	"github.com/holdenhewett/romanesco/cli/internal/config"
	"github.com/holdenhewett/romanesco/cli/internal/drift"
	"github.com/holdenhewett/romanesco/cli/internal/emit"
	"github.com/holdenhewett/romanesco/cli/internal/output"
	"github.com/holdenhewett/romanesco/cli/internal/parity"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
	"github.com/holdenhewett/romanesco/cli/internal/reconcile"
	"github.com/holdenhewett/romanesco/cli/internal/scan"
	"github.com/holdenhewett/romanesco/cli/internal/scan/detectors"
	"github.com/spf13/cobra"
	"os"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan codebase and generate context files",
	Long:  "Runs all detectors against the codebase, emits context files for configured providers, and updates the baseline.",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().String("format", "", "Target a single provider format")
	scanCmd.Flags().Bool("all", false, "Emit to all known providers")
	scanCmd.Flags().Bool("dry-run", false, "Show what would be written without writing")
	scanCmd.Flags().Bool("full", false, "Include parity analysis")
	scanCmd.Flags().Bool("yes", false, "Skip first-run confirmation")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		output.PrintError(2, "no detectable project", "Run from a project directory with go.mod, package.json, etc.")
		return fmt.Errorf("no detectable project")
	}

	// Load or create config
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	// If no config, require init or --yes
	if !config.Exists(root) {
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes && os.Getenv("NESCO_NO_PROMPT") != "1" && !output.JSON {
			fmt.Fprintln(output.Writer, "No .nesco/config.json found. Run `nesco init` first, or use --yes to auto-detect.")
			return fmt.Errorf("no config found")
		}
		// Auto-detect providers
		home, _ := os.UserHomeDir()
		for _, prov := range provider.AllProviders {
			if prov.Detect != nil && prov.Detect(home) {
				cfg.Providers = append(cfg.Providers, prov.Slug)
			}
		}
		config.Save(root, cfg)
	}

	// Run scanner
	scanner := scan.NewScanner(detectors.AllDetectors()...)
	result := scanner.Run(root)

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	formatFlag, _ := cmd.Flags().GetString("format")
	emitAll, _ := cmd.Flags().GetBool("all")

	if output.JSON {
		output.Print(result)
		return nil
	}

	// Print scan results summary
	fmt.Fprintf(output.Writer, "Scanned %s in %s\n", root, result.Duration.Round(1e6))
	fmt.Fprintf(output.Writer, "  Sections: %d\n", len(result.Document.Sections))
	if len(result.Warnings) > 0 {
		fmt.Fprintf(output.Writer, "  Warnings: %d\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Fprintf(output.Writer, "    ! %s: %s\n", w.Detector, w.Message)
		}
	}

	// Determine target providers
	var targets []provider.Provider
	if formatFlag != "" {
		prov := findProviderBySlug(formatFlag)
		if prov == nil {
			return fmt.Errorf("unknown provider: %s", formatFlag)
		}
		targets = []provider.Provider{*prov}
	} else if emitAll {
		targets = provider.AllProviders
	} else {
		for _, slug := range cfg.Providers {
			if prov := findProviderBySlug(slug); prov != nil {
				targets = append(targets, *prov)
			}
		}
	}

	// Emit to each provider
	for _, prov := range targets {
		emitter := emit.EmitterForProvider(prov.Slug)
		emitted, err := emitter.Emit(result.Document)
		if err != nil {
			fmt.Fprintf(output.Writer, "  x %s: emit error: %v\n", prov.Name, err)
			continue
		}

		if dryRun {
			fmt.Fprintf(output.Writer, "\n--- %s (dry run) ---\n%s\n", prov.Name, emitted)
			continue
		}

		if prov.EmitPath == nil {
			continue
		}
		outputPath := prov.EmitPath(root)
		format := reconcile.FormatHTML
		if emitter.Format() == "mdc" {
			format = reconcile.FormatYAML
		}
		_, err = reconcile.ReconcileAndWrite(outputPath, emitted, format)
		if err != nil {
			fmt.Fprintf(output.Writer, "  x %s: write error: %v\n", prov.Name, err)
			continue
		}
		fmt.Fprintf(output.Writer, "  + %s: %s\n", prov.Name, outputPath)
	}

	// Update baseline
	if !dryRun {
		nescoDir := config.DirPath(root)
		if err := drift.SaveBaseline(nescoDir, result.Document); err != nil {
			fmt.Fprintf(output.Writer, "  ! baseline update failed: %v\n", err)
		}
	}

	// Parity analysis if --full
	full, _ := cmd.Flags().GetBool("full")
	if full {
		home, _ := os.UserHomeDir()
		var detected []provider.Provider
		for _, prov := range provider.AllProviders {
			if prov.Detect != nil && prov.Detect(home) {
				detected = append(detected, prov)
			}
		}
		if len(detected) >= 2 {
			parityReport := parity.Analyze(detected, root)
			fmt.Fprintf(output.Writer, "\nParity: %s\n", parityReport.Summary)
		}
	}

	return nil
}
