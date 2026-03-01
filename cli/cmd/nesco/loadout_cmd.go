package main

import (
	"fmt"

	"github.com/OpenScribbler/nesco/cli/internal/loadout"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/spf13/cobra"
)

var loadoutCmd = &cobra.Command{
	Use:   "loadout",
	Short: "Apply, create, and manage loadouts",
	Long: `Loadouts bundle a curated set of nesco content — rules, hooks, skills,
agents, MCP servers — into a single shareable configuration.

Use "nesco loadout apply" to try or apply a loadout.
Use "nesco loadout create" to build a new loadout interactively.
Use "nesco loadout remove" to revert an active loadout.`,
}

func init() {
	rootCmd.AddCommand(loadoutCmd)
}

// checkAndWarnStaleSnapshot checks for a stale --try snapshot and prints
// a warning to stderr if found. Called at the start of every loadout subcommand
// so the user is aware of an orphaned temporary loadout that wasn't cleaned up
// (e.g., the SessionEnd hook failed or the process was killed).
func checkAndWarnStaleSnapshot(projectRoot string) {
	stale, err := loadout.CheckStaleSnapshot(projectRoot)
	if err != nil || stale == nil {
		return
	}
	fmt.Fprintf(output.ErrWriter,
		"Warning: A temporary loadout %q (applied %s) was not cleaned up.\n"+
			"Run 'nesco loadout remove' to restore your original config.\n\n",
		stale.LoadoutName, stale.CreatedAt.Format("2006-01-02 15:04:05"))
}
