package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/output"
	"github.com/holdenhewett/romanesco/cli/internal/parity"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var parityCmd = &cobra.Command{
	Use:   "parity",
	Short: "Compare AI tool configs across providers",
	Long:  "Analyzes which providers have content configured and reports gaps between them.",
	RunE:  runParity,
}

func init() {
	rootCmd.AddCommand(parityCmd)
}

func runParity(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	var detected []provider.Provider
	for _, prov := range provider.AllProviders {
		if prov.Detect(home) {
			detected = append(detected, prov)
		}
	}

	if len(detected) < 2 {
		if output.JSON {
			output.Print(map[string]string{"message": "fewer than 2 providers detected, parity analysis requires at least 2"})
		} else {
			fmt.Println("Parity analysis requires at least 2 detected providers.")
			fmt.Printf("Detected: %d\n", len(detected))
		}
		return nil
	}

	report := parity.Analyze(detected, root)

	if output.JSON {
		output.Print(report)
		return nil
	}

	fmt.Println("Coverage Matrix:")
	fmt.Println()

	header := fmt.Sprintf("  %-16s", "Content Type")
	for _, c := range report.Coverages {
		header += fmt.Sprintf("  %-14s", c.Provider)
	}
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", len(header)))

	for _, ct := range catalog.AllContentTypes() {
		row := fmt.Sprintf("  %-16s", ct.Label())
		for _, c := range report.Coverages {
			count := c.Types[ct]
			if count > 0 {
				row += fmt.Sprintf("  %-14s", fmt.Sprintf("+ %d", count))
			} else {
				row += fmt.Sprintf("  %-14s", "-")
			}
		}
		fmt.Println(row)
	}

	if len(report.Gaps) > 0 {
		fmt.Printf("\nGaps Found:\n")
		for _, g := range report.Gaps {
			fmt.Printf("  %s: present in %s, missing in %s\n",
				g.ContentType.Label(),
				strings.Join(g.HasIt, ", "),
				strings.Join(g.MissingIt, ", "),
			)
		}
	} else {
		fmt.Println("\nNo gaps found - all providers are in sync.")
	}

	return nil
}
