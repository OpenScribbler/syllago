package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/spf13/cobra"
)

// installResult is the JSON-serializable output for syllago install.
type installResult struct {
	Installed []installedItem `json:"installed"`
	Skipped   []skippedItem   `json:"skipped,omitempty"`
}

type installedItem struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Method   string   `json:"method"`
	Path     string   `json:"path"`
	Warnings []string `json:"warnings,omitempty"` // portability warnings
}

type skippedItem struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

var installCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Activate library content in a provider",
	Long: `Install content from your library into a provider's location.

By default uses a symlink so edits in your library are reflected immediately.
Use --method copy to place a standalone copy instead.`,
	Example: `  # Install a skill to Claude Code
  syllago install my-skill --to claude-code

  # Install with a standalone copy instead of symlink
  syllago install my-skill --to cursor --method copy

  # Install all skills to a provider
  syllago install --to claude-code --type skills

  # Preview what would happen
  syllago install my-skill --to claude-code --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInstall,
}

func init() {
	installCmd.Flags().String("to", "", "Provider to install into (required)")
	installCmd.MarkFlagRequired("to")
	installCmd.Flags().String("type", "", "Filter to a specific content type")
	installCmd.Flags().String("method", "symlink", "Install method: symlink (default) or copy")
	installCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
	installCmd.Flags().String("base-dir", "", "Override base directory for content installation")
	installCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	toSlug, _ := cmd.Flags().GetString("to")
	typeFilter, _ := cmd.Flags().GetString("type")
	methodStr, _ := cmd.Flags().GetString("method")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	baseDir, _ := cmd.Flags().GetString("base-dir")

	method := installer.MethodSymlink
	if methodStr == "copy" {
		method = installer.MethodCopy
	}

	prov := findProviderBySlug(toSlug)
	if prov == nil {
		slugs := providerSlugs()
		se := output.NewStructuredError(
			output.ErrProviderNotFound,
			"unknown provider: "+toSlug,
			"Available: "+strings.Join(slugs, ", "),
		)
		output.PrintStructuredError(se)
		return output.SilentError(se)
	}

	// Build resolver from merged config + CLI flag.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}
	projectRoot, _ := findProjectRoot()
	projectCfg, err := config.Load(projectRoot)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return fmt.Errorf("expanding paths: %w", err)
	}

	// Warn if the target provider is not detected on disk.
	if !output.JSON && !output.Quiet {
		detected := provider.DetectProvidersWithResolver(resolver)
		for _, dp := range detected {
			if dp.Slug == prov.Slug && !dp.Detected {
				fmt.Fprintf(output.ErrWriter, "Warning: %s not detected at default locations.\n", prov.Name)
				fmt.Fprintf(output.ErrWriter, "If installed at a custom path, configure it:\n")
				fmt.Fprintf(output.ErrWriter, "  syllago config paths --provider %s --path /your/path\n", prov.Slug)
				break
			}
		}
	}

	// Scan global library only.
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return fmt.Errorf("cannot determine home directory")
	}
	globalCat, err := catalog.Scan(globalDir, globalDir)
	if err != nil {
		return fmt.Errorf("scanning library: %w", err)
	}

	var items []catalog.ContentItem
	for _, item := range globalCat.Items {
		if len(args) == 1 && item.Name != args[0] {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		if len(args) == 1 {
			hint := typeFilter
			if hint == "" {
				hint = "skills"
			}
			return fmt.Errorf("no item named %q found in your library.\n  Hint: syllago list --type %s", args[0], hint)
		}
		fmt.Fprintln(output.ErrWriter, "no items found in library matching filters")
		return nil
	}

	if len(items) > 1 && !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Installing %d items to %s...\n", len(items), prov.Name)
	}

	var result installResult
	for _, item := range items {
		if dryRun {
			if !output.Quiet {
				fmt.Fprintf(output.Writer, "[dry-run] would install %s (%s) to %s\n", item.Name, item.Type.Label(), prov.Name)
			}
			continue
		}
		desc, err := installer.InstallWithResolver(item, *prov, globalDir, method, resolver)
		if err != nil {
			result.Skipped = append(result.Skipped, skippedItem{Name: item.Name, Reason: err.Error()})
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "  skip %s: %s\n", item.Name, err)
			}
			continue
		}

		// Check for portability warnings by running the converter.
		var warnings []string
		if conv := converter.For(item.Type); conv != nil {
			contentFile := converter.ResolveContentFile(item)
			if contentFile != "" {
				if raw, readErr := os.ReadFile(contentFile); readErr == nil {
					srcProv := ""
					if item.Meta != nil {
						srcProv = item.Meta.SourceProvider
					}
					if canonical, cErr := conv.Canonicalize(raw, srcProv); cErr == nil {
						if rendered, rErr := conv.Render(canonical.Content, *prov); rErr == nil {
							warnings = rendered.Warnings
						}
					}
				}
			}
		}

		result.Installed = append(result.Installed, installedItem{
			Name:     item.Name,
			Type:     string(item.Type),
			Method:   string(method),
			Path:     desc,
			Warnings: warnings,
		})
		if !output.JSON && !output.Quiet {
			if method == installer.MethodSymlink {
				fmt.Fprintf(output.Writer, "Symlinked %s to %s\n", item.Name, desc)
			} else {
				fmt.Fprintf(output.Writer, "Copied %s to %s\n", item.Name, desc)
			}
			for _, w := range warnings {
				fmt.Fprintf(output.ErrWriter, "    - %s\n", w)
			}
		}
	}

	if output.JSON {
		output.Print(result)
		return nil
	}

	if len(result.Installed) > 0 && !output.Quiet {
		fmt.Fprintf(output.Writer, "\n  Next: syllago install %s --to <other-provider>    (install to another provider)\n", firstArg(args))
		fmt.Fprintf(output.Writer, "        syllago convert %s --to <provider>           (convert for sharing)\n", firstArg(args))
	}

	return nil
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "<name>"
}
