package main

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

// Deprecated command stubs. These exist solely to intercept old command names
// and provide guidance to users. They print a redirect message and exit 1.

var deprecatedExportCmd = &cobra.Command{
	Use:    "export",
	Hidden: true,
	Short:  "(removed) use 'install' or 'convert'",
	RunE: func(cmd *cobra.Command, args []string) error {
		output.PrintError(1,
			"Unknown command 'export'.",
			"To install content into a provider: syllago install <name> --to <provider>\n  To convert for sharing:            syllago convert <name> --to <provider>")
		return output.SilentError(fmt.Errorf("export removed"))
	},
}

var deprecatedPromoteCmd = &cobra.Command{
	Use:    "promote",
	Hidden: true,
	Short:  "(removed) use 'share'",
	RunE: func(cmd *cobra.Command, args []string) error {
		output.PrintError(1,
			"Unknown command 'promote'.",
			"Use 'syllago share <name>' to contribute content to a shared git repo.")
		return output.SilentError(fmt.Errorf("promote removed"))
	},
}

func init() {
	rootCmd.AddCommand(deprecatedExportCmd)
	rootCmd.AddCommand(deprecatedPromoteCmd)
}
