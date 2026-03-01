package main

import (
	"fmt"
	"path/filepath"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/loadout"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/spf13/cobra"
)

var loadoutListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available loadouts",
	RunE:  runLoadoutList,
}

func init() {
	loadoutCmd.AddCommand(loadoutListCmd)
}

type loadoutListEntry struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	ItemCount   int    `json:"itemCount"`
	Description string `json:"description"`
}

func runLoadoutList(cmd *cobra.Command, args []string) error {
	projectRoot, _ := findProjectRoot()
	checkAndWarnStaleSnapshot(projectRoot)

	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find nesco repo: %w", err)
	}

	if projectRoot == "" {
		projectRoot = root
	}
	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	var entries []loadoutListEntry
	for _, item := range cat.Items {
		if item.Type != catalog.Loadouts {
			continue
		}
		manifest, parseErr := loadout.Parse(filepath.Join(item.Path, "loadout.yaml"))
		count := 0
		if parseErr == nil {
			count = manifest.ItemCount()
		}
		entries = append(entries, loadoutListEntry{
			Name:        item.Name,
			Provider:    item.Provider,
			ItemCount:   count,
			Description: item.Description,
		})
	}

	if output.JSON {
		output.Print(entries)
		return nil
	}

	if len(entries) == 0 {
		fmt.Fprintln(output.Writer, "No loadouts found.")
		return nil
	}

	// Print table header
	fmt.Fprintf(output.Writer, "%-30s %-15s %-6s %s\n", "NAME", "PROVIDER", "ITEMS", "DESCRIPTION")
	for _, e := range entries {
		desc := e.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(output.Writer, "%-30s %-15s %-6d %s\n", e.Name, e.Provider, e.ItemCount, desc)
	}

	return nil
}
