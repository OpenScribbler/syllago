package main

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/promote"
	"github.com/spf13/cobra"
)

// publishResult is the JSON-serializable output for syllago publish.
type publishResult struct {
	Name       string `json:"name"`
	Registry   string `json:"registry"`
	Branch     string `json:"branch"`
	PRUrl      string `json:"pr_url,omitempty"`
	CompareURL string `json:"compare_url,omitempty"`
}

var publishCmd = &cobra.Command{
	Use:   "publish <name>",
	Short: "Contribute library content to a registry",
	Long: `Copies a library item to a registry clone, stages the change, and
optionally creates a branch and PR.

Examples:
  syllago publish my-skill --registry my-registry
  syllago publish my-rule --registry team-rules --type rules`,
	Args: cobra.ExactArgs(1),
	RunE: runPublish,
}

func init() {
	publishCmd.Flags().String("registry", "", "Registry name to publish to (required)")
	publishCmd.MarkFlagRequired("registry")
	publishCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
	publishCmd.Flags().Bool("no-input", false, "Skip interactive git prompts, stage only")
	rootCmd.AddCommand(publishCmd)
}

func runPublish(cmd *cobra.Command, args []string) error {
	name := args[0]
	registryName, _ := cmd.Flags().GetString("registry")
	typeFilter, _ := cmd.Flags().GetString("type")
	noInput, _ := cmd.Flags().GetBool("no-input")

	// Use an empty temp dir as contentRoot to avoid scan shadowing.
	emptyRoot, err := os.MkdirTemp("", "syllago-publish-*")
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

	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find syllago repo: %w", err)
	}

	if !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Publishing to registry...\n")
	}

	result, err := promote.PromoteToRegistry(root, registryName, *item, noInput)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	if output.JSON {
		output.Print(publishResult{
			Name:       name,
			Registry:   registryName,
			Branch:     result.Branch,
			PRUrl:      result.PRUrl,
			CompareURL: result.CompareURL,
		})
		return nil
	}

	if result.PRUrl != "" {
		fmt.Fprintf(output.Writer, "Published! PR: %s\n", result.PRUrl)
	} else if result.CompareURL != "" {
		fmt.Fprintf(output.Writer, "Published! Branch %q pushed.\n  Open a PR: %s\n", result.Branch, result.CompareURL)
	} else {
		fmt.Fprintf(output.Writer, "Published! Branch %q pushed to registry %q.\n", result.Branch, registryName)
	}

	return nil
}
