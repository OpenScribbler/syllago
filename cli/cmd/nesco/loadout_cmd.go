package main

import (
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
