package main

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/promote"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// shareResult is the JSON-serializable output for syllago share.
type shareResult struct {
	Name       string `json:"name"`
	Registry   string `json:"registry,omitempty"`
	Branch     string `json:"branch"`
	PRUrl      string `json:"pr_url,omitempty"`
	CompareURL string `json:"compare_url,omitempty"`
}

// Function-variable indirection lets tests stub the promote package without
// needing to set up a full git-repo-with-remote fixture for wiring coverage.
var (
	promoteFunc           = promote.Promote
	promoteToRegistryFunc = promote.PromoteToRegistry
)

var shareCmd = &cobra.Command{
	Use:   "share <name>",
	Short: "Contribute library content to a team repo or registry",
	Long: `Copies a library item to a target repo, stages the change, and
optionally creates a branch and PR.

By default, shares to the current team repo (the syllago repo you're in).
Use --to to share to a named registry instead.`,
	Example: `  # Share a skill to the current team repo
  syllago share my-skill

  # Share to a named registry
  syllago share my-skill --to my-registry

  # Disambiguate by type
  syllago share my-rule --type rules

  # Non-interactive mode (stage only, no git prompts)
  syllago share my-skill --no-input`,
	Args: cobra.ExactArgs(1),
	RunE: runShare,
}

func init() {
	shareCmd.Flags().String("to", "", "Target registry name (omit for current team repo)")
	shareCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
	shareCmd.Flags().Bool("no-input", false, "Skip interactive git prompts, stage only")
	rootCmd.AddCommand(shareCmd)
}

func runShare(cmd *cobra.Command, args []string) error {
	name := args[0]
	toRegistry, _ := cmd.Flags().GetString("to")
	typeFilter, _ := cmd.Flags().GetString("type")
	noInput, _ := cmd.Flags().GetBool("no-input")

	// Use an empty temp dir as contentRoot to avoid scan shadowing.
	emptyRoot, err := os.MkdirTemp("", "syllago-share-*")
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "creating temp dir failed", "Check filesystem permissions and disk space", err.Error())
	}
	defer func() { _ = os.RemoveAll(emptyRoot) }()

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyRoot, emptyRoot, nil)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library failed", "Check that ~/.syllago/content/ exists and is readable", err.Error())
	}

	item, err := findLibraryItem(cat, name, typeFilter)
	if err != nil {
		return err
	}
	telemetry.Enrich("content_type", string(item.Type))

	root, err := findContentRepoRoot()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogNotFound, "could not find syllago repo", "Use this command from inside a syllago repo directory", err.Error())
	}

	if toRegistry != "" {
		// Share to named registry
		if !output.Quiet && !output.JSON {
			fmt.Fprintf(output.Writer, "Sharing to registry...\n")
		}

		result, err := promoteToRegistryFunc(root, toRegistry, *item, noInput)
		if err != nil {
			return output.NewStructuredErrorDetail(output.ErrPromoteGitFailed, "share to registry failed", "Check git credentials and network connectivity", err.Error())
		}

		if output.JSON {
			output.Print(shareResult{
				Name:       name,
				Registry:   toRegistry,
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
			fmt.Fprintf(output.Writer, "Shared! Branch %q pushed to registry %q.\n", result.Branch, toRegistry)
		}
		return nil
	}

	// Share to current team repo
	if !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Staging changes...\n")
	}

	result, err := promoteFunc(root, *item, noInput)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrPromoteGitFailed, "sharing failed", "Check git status and permissions in the team repo", err.Error())
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
		return nil, output.NewStructuredError(output.ErrItemNotFound, fmt.Sprintf("no item named %q found in your library", name), "Run 'syllago list' to see all library items")
	}
	if len(matches) > 1 {
		return nil, output.NewStructuredError(output.ErrItemAmbiguous, fmt.Sprintf("%q exists in multiple types", name), "Use --type to disambiguate")
	}
	return &matches[0], nil
}
