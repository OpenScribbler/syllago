package main

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/promote"
	"github.com/spf13/cobra"
)

// shareResult is the JSON-serializable output for syllago share.
type shareResult struct {
	Name       string `json:"name"`
	Branch     string `json:"branch"`
	PRUrl      string `json:"pr_url,omitempty"`
	CompareURL string `json:"compare_url,omitempty"`
}

var shareCmd = &cobra.Command{
	Use:   "share <name>",
	Short: "Contribute library content to a team repo",
	Long: `Copies a library item to your team repo, stages the change, and
optionally creates a branch and PR.`,
	Example: `  # Share a skill to the team repo
  syllago share my-skill

  # Disambiguate by type
  syllago share my-rule --type rules

  # Non-interactive mode (stage only, no git prompts)
  syllago share my-skill --no-input`,
	Args: cobra.ExactArgs(1),
	RunE: runShare,
}

func init() {
	shareCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
	shareCmd.Flags().Bool("no-input", false, "Skip interactive git prompts, stage only")
	rootCmd.AddCommand(shareCmd)
}

func runShare(cmd *cobra.Command, args []string) error {
	name := args[0]
	typeFilter, _ := cmd.Flags().GetString("type")
	noInput, _ := cmd.Flags().GetBool("no-input")

	// Use an empty temp dir as contentRoot to avoid scan shadowing.
	emptyRoot, err := os.MkdirTemp("", "syllago-share-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(emptyRoot)

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyRoot, emptyRoot, nil)
	if err != nil {
		return fmt.Errorf("scanning library: %w", err)
	}

	item, err := findLibraryItem(cat, name, typeFilter)
	if err != nil {
		return err
	}

	// Find team repo root (the syllago repo the user is working in).
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find syllago team repo: %w\n  Use this command from inside a syllago team repo directory", err)
	}

	if !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Staging changes...\n")
	}

	result, err := promote.Promote(root, *item, noInput)
	if err != nil {
		return fmt.Errorf("sharing failed: %w", err)
	}

	if output.JSON {
		output.Print(shareResult{
			Name:       name,
			Branch:     result.Branch,
			PRUrl:      result.PRUrl,
			CompareURL: result.CompareURL,
		})
		return nil
	}

	if result.PRUrl != "" {
		fmt.Fprintf(output.Writer, "Shared! PR: %s\n", result.PRUrl)
	} else if result.CompareURL != "" {
		fmt.Fprintf(output.Writer, "Shared! Branch %q pushed.\n  Open a PR: %s\n", result.Branch, result.CompareURL)
	} else {
		fmt.Fprintf(output.Writer, "Shared! Branch %q pushed.\n", result.Branch)
	}

	return nil
}

// findLibraryItem looks up an item by name in the global library.
// It is also used by publish_cmd.go (Phase 2.5).
func findLibraryItem(cat *catalog.Catalog, name, typeFilter string) (*catalog.ContentItem, error) {
	var matches []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Source != "global" || item.Name != name {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		matches = append(matches, item)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no item named %q found in your library.\n  Hint: syllago list    (show all library items)", name)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%q exists in multiple types. Use --type to disambiguate.", name)
	}
	return &matches[0], nil
}
