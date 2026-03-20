package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

var loadoutCreateCmd = &cobra.Command{
	Use:     "create",
	Short:   "Interactively create a new loadout",
	Example: `  syllago loadout create`,
	RunE:    runLoadoutCreate,
}

func init() {
	loadoutCmd.AddCommand(loadoutCreateCmd)
}

func runLoadoutCreate(cmd *cobra.Command, args []string) error {
	projectRoot, _ := findProjectRoot()
	checkAndWarnStaleSnapshot(projectRoot)

	if !isInteractive() {
		return fmt.Errorf("loadout create requires an interactive terminal")
	}

	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find syllago repo: %w", err)
	}
	if projectRoot == "" {
		projectRoot = root
	}

	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	scanner := bufio.NewScanner(os.Stdin)

	// Step 1: Name
	fmt.Fprint(output.Writer, "Loadout name: ")
	if !scanner.Scan() {
		return fmt.Errorf("no input")
	}
	name := strings.TrimSpace(scanner.Text())
	if errMsg := catalog.ValidateUserName(name); errMsg != "" {
		return fmt.Errorf("invalid loadout name: %s", errMsg)
	}

	// Step 2: Description
	fmt.Fprint(output.Writer, "Description: ")
	if !scanner.Scan() {
		return fmt.Errorf("no input")
	}
	description := strings.TrimSpace(scanner.Text())

	// Step 3: Provider (default claude-code for v1)
	providerSlug := "claude-code"
	fmt.Fprintf(output.Writer, "Provider [%s]: ", providerSlug)
	if scanner.Scan() {
		if input := strings.TrimSpace(scanner.Text()); input != "" {
			providerSlug = input
		}
	}

	itemsByType := map[catalog.ContentType][]string{}

	// Step 4: For each content type, let user select items
	selectableTypes := []catalog.ContentType{
		catalog.Rules, catalog.Hooks, catalog.Skills, catalog.Agents,
		catalog.MCP, catalog.Commands,
	}

	for _, ct := range selectableTypes {
		var available []catalog.ContentItem
		for _, item := range cat.Items {
			if item.Type != ct {
				continue
			}
			// For provider-specific types, only show items for the selected provider
			if !ct.IsUniversal() && ct != catalog.Loadouts && item.Provider != providerSlug {
				continue
			}
			available = append(available, item)
		}
		if len(available) == 0 {
			continue
		}

		fmt.Fprintf(output.Writer, "\n%s (enter numbers separated by commas, or press Enter to skip):\n", ct.Label())
		for i, item := range available {
			desc := item.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(output.Writer, "  %d) %s", i+1, item.Name)
			if desc != "" {
				fmt.Fprintf(output.Writer, " — %s", desc)
			}
			fmt.Fprintln(output.Writer)
		}
		fmt.Fprint(output.Writer, "Select: ")

		if !scanner.Scan() {
			continue
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		var selected []string
		for _, part := range strings.Split(input, ",") {
			part = strings.TrimSpace(part)
			idx := 0
			_, parseErr := fmt.Sscanf(part, "%d", &idx)
			if parseErr != nil || idx < 1 || idx > len(available) {
				fmt.Fprintf(output.ErrWriter, "  skipping invalid selection: %s\n", part)
				continue
			}
			selected = append(selected, available[idx-1].Name)
		}

		itemsByType[ct] = selected
	}

	manifest := loadout.BuildManifest(providerSlug, name, description, itemsByType)

	// Step 5: Review
	fmt.Fprintf(output.Writer, "\n--- Loadout Review ---\n")
	fmt.Fprintf(output.Writer, "Name:        %s\n", manifest.Name)
	fmt.Fprintf(output.Writer, "Provider:    %s\n", manifest.Provider)
	fmt.Fprintf(output.Writer, "Description: %s\n", manifest.Description)

	totalItems := len(manifest.Rules) + len(manifest.Hooks) + len(manifest.Skills) +
		len(manifest.Agents) + len(manifest.MCP) + len(manifest.Commands)
	fmt.Fprintf(output.Writer, "Total items: %d\n", totalItems)

	if totalItems == 0 {
		fmt.Fprintln(output.ErrWriter, "No items selected. Aborting.")
		return nil
	}

	fmt.Fprint(output.Writer, "\nCreate this loadout? [Y/n]: ")
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer == "n" || answer == "no" {
			fmt.Fprintln(output.Writer, "Cancelled.")
			return nil
		}
	}

	// Step 6: Write loadout.yaml
	parentDir := filepath.Join(root, "content", "loadouts", providerSlug)
	outPath, err := loadout.WriteManifest(manifest, parentDir)
	if err != nil {
		return fmt.Errorf("writing loadout: %w", err)
	}

	fmt.Fprintf(output.Writer, "\nCreated loadout at: %s\n", outPath)
	fmt.Fprintln(output.Writer, "Run 'syllago loadout apply "+name+"' to try it out.")

	return nil
}
