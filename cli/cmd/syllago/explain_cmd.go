package main

import (
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/errordocs"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

var explainListAll bool

var explainCmd = &cobra.Command{
	Use:   "explain CODE",
	Short: "Show detailed documentation for an error code",
	Long: `Display documentation for a syllago error code.

Each error code has a detailed explanation including what it means,
common causes, and how to fix it.`,
	Example: `  syllago explain CATALOG_001
  syllago explain --list`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if explainListAll {
			codes := errordocs.ListCodes()
			if len(codes) == 0 {
				fmt.Fprintln(output.Writer, "No error documentation available.")
				return nil
			}
			if output.JSON {
				output.Print(codes)
				return nil
			}
			fmt.Fprintf(output.Writer, "Available error codes (%d):\n\n", len(codes))
			for _, code := range codes {
				fmt.Fprintf(output.Writer, "  %s\n", code)
			}
			fmt.Fprintf(output.Writer, "\nRun 'syllago explain CODE' for details on a specific error.\n")
			return nil
		}

		if len(args) == 0 {
			return output.NewStructuredError(output.ErrInputMissing,
				"no error code provided",
				"Provide an error code, or use --list to see all codes")
		}

		code := strings.ToUpper(args[0])
		doc, err := errordocs.Explain(code)
		if err != nil {
			return output.NewStructuredError(output.ErrInputInvalid,
				fmt.Sprintf("unknown error code: %s", code),
				"Run 'syllago explain --list' to see all available error codes")
		}

		if output.JSON {
			output.Print(map[string]string{
				"code":          code,
				"documentation": doc,
			})
			return nil
		}

		fmt.Fprintf(output.Writer, "Error %s\n", code)
		fmt.Fprintf(output.Writer, "%s\n", strings.Repeat("─", len("Error ")+len(code)))
		fmt.Fprintln(output.Writer)
		fmt.Fprint(output.Writer, doc)
		return nil
	},
}

func init() {
	explainCmd.Flags().BoolVar(&explainListAll, "list", false, "list all available error codes")
	rootCmd.AddCommand(explainCmd)
}
