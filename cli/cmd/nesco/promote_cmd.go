package main

import (
	"fmt"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/promote"
	"github.com/spf13/cobra"
)

var promoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote content to shared or registry",
	Long: `Promote local content for sharing with others.

Use "to-registry" to contribute content to a git registry via a PR workflow.
The command creates a branch in the registry clone, copies the content, commits,
pushes, and optionally opens a PR if the gh CLI is installed.

Example:
  nesco promote to-registry nesco-tools skills/my-skill`,
}

var promoteToRegistryCmd = &cobra.Command{
	Use:   "to-registry <registry-name> <type/name>",
	Short: "Contribute content to a registry via PR",
	Long: `Copies a local content item into a registry's clone directory, creates a
contribution branch, commits, pushes, and opens a PR (if gh CLI is available).

The item path uses the format "type/name" for universal content (skills, agents,
prompts, mcp, apps) or "type/provider/name" for provider-specific content
(rules, hooks, commands).

Examples:
  nesco promote to-registry nesco-tools skills/my-skill
  nesco promote to-registry team-rules rules/claude-code/no-console-log`,
	Args: cobra.ExactArgs(2),
	RunE: runPromoteToRegistry,
}

func init() {
	promoteCmd.AddCommand(promoteToRegistryCmd)
	rootCmd.AddCommand(promoteCmd)
}

func runPromoteToRegistry(cmd *cobra.Command, args []string) error {
	registryName := args[0]
	itemPath := args[1]

	// Find repo root and scan catalog
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find nesco repo: %w", err)
	}

	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}

	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	// Look up the item by path (type/name or type/provider/name)
	item, err := findItemByPath(cat, itemPath)
	if err != nil {
		return fmt.Errorf("finding item: %w", err)
	}

	// Promote to registry
	result, err := promote.PromoteToRegistry(root, registryName, *item)
	if err != nil {
		return fmt.Errorf("promote to registry failed: %w", err)
	}

	// Print result
	if result.PRUrl != "" {
		fmt.Fprintf(output.Writer, "PR created: %s\n", result.PRUrl)
	} else if result.CompareURL != "" {
		fmt.Fprintf(output.Writer, "Pushed branch %q. Open a PR:\n  %s\n", result.Branch, result.CompareURL)
	} else {
		fmt.Fprintf(output.Writer, "Pushed branch %q to registry %q.\n", result.Branch, registryName)
	}

	return nil
}
