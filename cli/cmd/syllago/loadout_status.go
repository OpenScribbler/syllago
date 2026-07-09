package main

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

var loadoutStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active loadout status",
	Example: `  # Check if a loadout is active
  syllago loadout status

  # JSON output
  syllago loadout status --json`,
	RunE: runLoadoutStatus,
}

func init() {
	loadoutCmd.AddCommand(loadoutStatusCmd)
}

type loadoutStatusResult struct {
	Active    bool   `json:"active"`
	Name      string `json:"name,omitempty"`
	Mode      string `json:"mode,omitempty"`
	AppliedAt string `json:"appliedAt,omitempty"`
}

func runLoadoutStatus(cmd *cobra.Command, args []string) error {
	projectRoot, _ := findProjectRoot()
	checkAndWarnStaleSnapshot(projectRoot)

	manifest, found, err := loadSnapshotManifest(projectRoot)
	if err != nil {
		return err
	}
	if !found {
		if output.JSON {
			output.Print(loadoutStatusResult{Active: false})
		} else {
			fmt.Fprintln(output.Writer, "No active loadout.")
		}
		return nil
	}

	if output.JSON {
		output.Print(loadoutStatusResult{
			Active:    true,
			Name:      manifest.LoadoutName,
			Mode:      manifest.Mode,
			AppliedAt: manifest.CreatedAt.Format("2006-01-02 15:04:05"),
		})
		return nil
	}

	fmt.Fprintf(output.Writer, "Active loadout: %s (%s)\n", manifest.LoadoutName, manifest.Mode)
	fmt.Fprintf(output.Writer, "Applied: %s\n", manifest.CreatedAt.Format("2006-01-02 15:04:05"))

	if len(manifest.Symlinks) > 0 {
		fmt.Fprintf(output.Writer, "\nInstalled symlinks:\n")
		for _, s := range manifest.Symlinks {
			fmt.Fprintf(output.Writer, "  %s -> %s\n", s.Path, s.Target)
		}
	}

	if len(manifest.HookScripts) > 0 {
		fmt.Fprintf(output.Writer, "\nInstalled hooks:\n")
		for _, h := range manifest.HookScripts {
			fmt.Fprintf(output.Writer, "  %s\n", h)
		}
	}

	return nil
}
