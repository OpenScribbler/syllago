package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
	"github.com/spf13/cobra"
)

var loadoutRemoveCmd = &cobra.Command{
	Use:     "remove",
	Short:   "Remove the active loadout and restore original configuration",
	Example: `  syllago loadout remove`,
	RunE:    runLoadoutRemove,
}

func init() {
	loadoutRemoveCmd.Flags().Bool("auto", false, "Skip confirmation (used by SessionEnd hook)")
	loadoutCmd.AddCommand(loadoutRemoveCmd)
}

func runLoadoutRemove(cmd *cobra.Command, args []string) error {
	projectRoot, _ := findProjectRoot()
	autoMode, _ := cmd.Flags().GetBool("auto")

	if !autoMode {
		checkAndWarnStaleSnapshot(projectRoot)
	}

	// Check for active snapshot first to show what will be reverted
	manifest, _, err := snapshot.Load(projectRoot)
	if errors.Is(err, snapshot.ErrNoSnapshot) {
		if !autoMode {
			fmt.Fprintln(output.Writer, "No active loadout to remove.")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading snapshot: %w", err)
	}

	// Without --auto, show what will happen and ask for confirmation
	if !autoMode {
		fmt.Fprintf(output.Writer, "Active loadout: %s (%s)\n\n", manifest.LoadoutName, manifest.Mode)

		if len(manifest.Symlinks) > 0 {
			fmt.Fprintln(output.Writer, "Symlinks to remove:")
			for _, s := range manifest.Symlinks {
				fmt.Fprintf(output.Writer, "  %s\n", s.Path)
			}
		}

		if len(manifest.BackedUpFiles) > 0 {
			fmt.Fprintln(output.Writer, "\nFiles to restore from snapshot:")
			for _, f := range manifest.BackedUpFiles {
				fmt.Fprintf(output.Writer, "  %s\n", f)
			}
		}

		fmt.Fprintln(output.Writer, "\nNote: Any changes you made to settings.json or .claude.json after")
		fmt.Fprintln(output.Writer, "applying the loadout will be lost — the original files are restored")
		fmt.Fprintln(output.Writer, "from a snapshot.")

		if isInteractive() {
			fmt.Fprintf(output.Writer, "\nRemove loadout? [y/N]: ")
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("no input received")
			}
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Fprintln(output.Writer, "Cancelled.")
				return nil
			}
		}
	}

	opts := loadout.RemoveOptions{
		Auto:        autoMode,
		ProjectRoot: projectRoot,
	}

	result, err := loadout.Remove(opts)
	if errors.Is(err, loadout.ErrNoActiveLoadout) {
		if !autoMode {
			fmt.Fprintln(output.Writer, "No active loadout to remove.")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("removing loadout: %w", err)
	}

	if !autoMode {
		if output.JSON {
			output.Print(result)
		} else {
			fmt.Fprintf(output.Writer, "\nLoadout %q removed. Original configuration restored.\n", result.LoadoutName)
		}
	}

	return nil
}
