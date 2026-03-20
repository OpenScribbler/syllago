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

var deprecatedImportCmd = &cobra.Command{
	Use:    "import",
	Hidden: true,
	Short:  "(removed) use 'add'",
	RunE: func(cmd *cobra.Command, args []string) error {
		output.PrintError(1,
			"Unknown command 'import'.",
			"To add content to your library: syllago add <source>")
		return output.SilentError(fmt.Errorf("import removed"))
	},
}

var deprecatedPromoteCmd = &cobra.Command{
	Use:    "promote",
	Hidden: true,
	Short:  "(removed) use 'share' or 'publish'",
	RunE: func(cmd *cobra.Command, args []string) error {
		output.PrintError(1,
			"Unknown command 'promote'.",
			"To share with your team:        syllago share <name>\n  To publish to a registry: syllago publish <name> --registry <name>")
		return output.SilentError(fmt.Errorf("promote removed"))
	},
}

func init() {
	// Register deprecated stubs for export and promote now that the real
	// commands have been deleted (Phase 3.2). The import stub is NOT registered
	// here because import.go still exists — it will be registered after Phase 3.1.
	rootCmd.AddCommand(deprecatedExportCmd)
	rootCmd.AddCommand(deprecatedPromoteCmd)
}
