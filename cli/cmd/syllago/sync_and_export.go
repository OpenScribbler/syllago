package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/spf13/cobra"
)

var syncAndExportCmd = &cobra.Command{
	Use:   "sync-and-export",
	Short: "Sync registries then export content to a provider",
	Long: `Convenience command that syncs all registries then exports.

Equivalent to running:
  syllago registry sync && syllago export --to <provider>

This is useful in CI/CD or automation where you want a single command
to ensure registries are up-to-date before exporting.

Examples:
  syllago sync-and-export --to cursor
  syllago sync-and-export --to all --type skills
  syllago sync-and-export --to kiro --source registry`,
	RunE: runSyncAndExport,
}

func init() {
	syncAndExportCmd.Flags().String("to", "", "Provider slug to export to, or \"all\" for every provider (required)")
	syncAndExportCmd.MarkFlagRequired("to")
	syncAndExportCmd.Flags().String("type", "", "Filter to a specific content type (e.g., skills, rules)")
	syncAndExportCmd.Flags().String("name", "", "Filter by item name (substring match)")
	syncAndExportCmd.Flags().String("source", "local", "Which items to export: local (default), shared, registry, builtin, all")
	syncAndExportCmd.Flags().String("llm-hooks", "skip", "How to handle LLM-evaluated hooks: skip (drop with warning) or generate (create wrapper scripts)")
	rootCmd.AddCommand(syncAndExportCmd)
}

func runSyncAndExport(cmd *cobra.Command, args []string) error {
	// Find project root and load config to get registry list.
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("could not find project root: %w", err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Sync registries if any are configured.
	if len(cfg.Registries) > 0 {
		names := make([]string, len(cfg.Registries))
		for i, r := range cfg.Registries {
			names[i] = r.Name
		}

		if !output.JSON {
			fmt.Fprintf(output.Writer, "Syncing %d registries...\n", len(names))
		}

		results := registry.SyncAll(names)
		for _, res := range results {
			if res.Err != nil {
				return fmt.Errorf("registry sync failed for %q: %w", res.Name, res.Err)
			}
			if !output.JSON {
				fmt.Fprintf(output.Writer, "Synced: %s\n", res.Name)
			}
		}
	}

	toSlug, _ := cmd.Flags().GetString("to")
	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")
	sourceFilter, _ := cmd.Flags().GetString("source")
	llmHooksMode, _ := cmd.Flags().GetString("llm-hooks")

	return runExportOp(root, toSlug, typeFilter, nameFilter, sourceFilter, llmHooksMode, "")
}

// exportResult is the JSON-serializable output for export operations.
type exportResult struct {
	Exported []exportedItem `json:"exported"`
	Skipped  []exportSkippedItem  `json:"skipped,omitempty"`
}

type exportedItem struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Destination string   `json:"destination"`
	Converted   bool     `json:"converted,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type exportSkippedItem struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// runExportOp contains the core export logic shared by sync-and-export.
// toSlug may be "all" to export to every known provider.
func runExportOp(root, toSlug, typeFilter, nameFilter, sourceFilter, llmHooksMode, baseDir string) error {
	if toSlug == "all" {
		return runExportAll(root, typeFilter, nameFilter, sourceFilter, llmHooksMode, baseDir)
	}

	prov := findProviderBySlug(toSlug)
	if prov == nil {
		slugs := providerSlugs()
		slugs = append(slugs, "all")
		output.PrintError(1, "unknown provider: "+toSlug,
			"Available: "+strings.Join(slugs, ", "))
		return output.SilentError(fmt.Errorf("unknown provider: %s", toSlug))
	}

	// Build resolver from merged config + CLI flag.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}
	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}
	projectCfg, err := config.Load(projectRoot)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return fmt.Errorf("expanding paths: %w", err)
	}

	// Configure the hooks converter with the LLM hooks mode.
	if hooksConv, ok := converter.For(catalog.Hooks).(*converter.HooksConverter); ok {
		hooksConv.LLMHooksMode = llmHooksMode
	}

	// Scan the catalog.
	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	// Collect items matching source, type, and name filters.
	var items []catalog.ContentItem
	for _, item := range cat.Items {
		if !filterBySource(item, sourceFilter) {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		if nameFilter != "" && !strings.Contains(item.Name, nameFilter) {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		msg := "no items found"
		if sourceFilter != "all" {
			msg += " in " + sourceFilter
		}
		if typeFilter != "" || nameFilter != "" {
			msg += " matching filters"
		}
		fmt.Fprintln(output.ErrWriter, msg)
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	result := exportResult{}

	for _, item := range items {
		// Warn about built-in or example content before processing.
		if msg := exportWarnMessage(item); msg != "" {
			fmt.Fprintf(output.ErrWriter, "  warning: %s is %s\n", item.Name, msg)
		}

		// Check if provider supports this type via SupportsType.
		if prov.SupportsType != nil && !prov.SupportsType(item.Type) {
			skip := exportSkippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s does not support %s", prov.Name, item.Type.Label()),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): %s does not support %s\n",
					item.Name, item.Type.Label(), prov.Name, item.Type.Label())
			}
			continue
		}

		// Check for JSON merge sentinel — these types require config-file
		// merging rather than filesystem copy.
		installDir := resolver.InstallDir(*prov, item.Type, homeDir)
		if installDir == provider.JSONMergeSentinel {
			// Allow converter-based cross-provider export for JSON merge types
			srcProv := effectiveProvider(item)
			if conv := converter.For(item.Type); conv != nil && srcProv != "" && srcProv != toSlug {
				contentFile := converter.ResolveContentFile(item)
				if contentFile != "" {
					content, readErr := os.ReadFile(contentFile)
					if readErr == nil {
						canonical, canonErr := conv.Canonicalize(content, srcProv)
						if canonErr != nil {
							canonical = &converter.Result{Content: content}
						}
						rendered, renderErr := conv.Render(canonical.Content, *prov)
						if renderErr == nil && rendered.Content != nil {
							dest := filepath.Join(item.Path, "exported-"+toSlug+"-"+rendered.Filename)
							if writeErr := os.WriteFile(dest, rendered.Content, 0644); writeErr == nil {
								for name, extraContent := range rendered.ExtraFiles {
									extraPath := filepath.Join(item.Path, name)
									if extraErr := os.WriteFile(extraPath, extraContent, 0755); extraErr != nil {
										rendered.Warnings = append(rendered.Warnings, fmt.Sprintf("failed to write %s: %s", name, extraErr))
									}
								}
								result.Exported = append(result.Exported, exportedItem{
									Name:        item.Name,
									Type:        string(item.Type),
									Destination: dest,
									Converted:   true,
									Warnings:    append(rendered.Warnings, fmt.Sprintf("JSON merge type: saved to %s (merge manually into provider config)", dest)),
								})
								if !output.JSON {
									fmt.Fprintf(output.Writer, "Exported %s to %s (converted, merge manually)\n", item.Name, dest)
									for _, w := range rendered.Warnings {
										fmt.Fprintf(output.ErrWriter, "  warning: %s\n", w)
									}
								}
								continue
							}
						}
					}
				}
			}

			skip := exportSkippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s for %s requires JSON merge (not supported by export)", item.Type.Label(), prov.Name),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): requires JSON merge for %s (use the TUI to install)\n",
					item.Name, item.Type.Label(), prov.Name)
			}
			continue
		}

		// Project-scope types: resolve install dir from DiscoveryPaths using CWD.
		if installDir == provider.ProjectScopeSentinel {
			cwd, cwdErr := os.Getwd()
			if cwdErr != nil {
				return fmt.Errorf("getting working directory: %w", cwdErr)
			}
			if prov.DiscoveryPaths != nil {
				paths := resolver.DiscoveryPaths(*prov, item.Type, cwd)
				if len(paths) > 0 {
					installDir = paths[0]
				}
			}
			if installDir == provider.ProjectScopeSentinel {
				skip := exportSkippedItem{
					Name:   item.Name,
					Type:   string(item.Type),
					Reason: fmt.Sprintf("%s %s requires a project directory (no discovery path configured)", prov.Name, item.Type.Label()),
				}
				result.Skipped = append(result.Skipped, skip)
				if !output.JSON {
					fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): %s %s requires a project directory\n",
						item.Name, item.Type.Label(), prov.Name, item.Type.Label())
				}
				continue
			}
		}

		if installDir == "" {
			skip := exportSkippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s does not support %s", prov.Name, item.Type.Label()),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): %s does not support %s\n",
					item.Name, item.Type.Label(), prov.Name, item.Type.Label())
			}
			continue
		}

		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", installDir, err)
		}

		// Try cross-provider rendering via converter
		if conv := converter.For(item.Type); conv != nil {
			exported, handled := exportWithConverter(item, *prov, toSlug, conv, installDir)
			if handled {
				if exported != nil {
					result.Exported = append(result.Exported, *exported)
					if !output.JSON {
						fmt.Fprintf(output.Writer, "Exported %s to %s (converted)\n", item.Name, exported.Destination)
						for _, w := range exported.Warnings {
							fmt.Fprintf(output.ErrWriter, "  warning: %s\n", w)
						}
					}
				} else {
					// Skipped by converter (e.g. non-alwaysApply for single-file provider)
					skip := exportSkippedItem{
						Name:   item.Name,
						Type:   string(item.Type),
						Reason: fmt.Sprintf("not compatible with %s format", prov.Name),
					}
					result.Skipped = append(result.Skipped, skip)
					if !output.JSON {
						fmt.Fprintf(output.ErrWriter, "Skipping %s: not compatible with %s format\n", item.Name, prov.Name)
					}
				}
				continue
			}
		}

		// Fallback: direct copy (no converter or same-provider without .source/)
		dest := filepath.Join(installDir, item.Name)

		if err := installer.CopyContent(item.Path, dest); err != nil {
			return fmt.Errorf("copying %s: %w", item.Name, err)
		}

		result.Exported = append(result.Exported, exportedItem{
			Name:        item.Name,
			Type:        string(item.Type),
			Destination: dest,
		})

		if !output.JSON {
			fmt.Fprintf(output.Writer, "Exported %s to %s\n", item.Name, dest)
		}
	}

	if output.JSON {
		output.Print(result)
	} else if len(result.Exported) == 0 && len(result.Skipped) > 0 {
		fmt.Fprintln(output.ErrWriter, "No items were exported (all skipped).")
	}

	return nil
}

// runExportAll exports to every known provider in sequence.
func runExportAll(root, typeFilter, nameFilter, sourceFilter, llmHooksMode, baseDir string) error {
	type providerSummary struct {
		Slug string
		Err  error
	}

	var summaries []providerSummary

	for _, prov := range provider.AllProviders {
		if !output.JSON {
			fmt.Fprintf(output.Writer, "\n--- %s (%s) ---\n", prov.Name, prov.Slug)
		}

		err := runExportOp(root, prov.Slug, typeFilter, nameFilter, sourceFilter, llmHooksMode, baseDir)
		summaries = append(summaries, providerSummary{Slug: prov.Slug, Err: err})
	}

	// Print summary.
	if !output.JSON {
		fmt.Fprintf(output.Writer, "\n=== Export All Summary ===\n")
		hasErrors := false
		for _, s := range summaries {
			status := "ok"
			if s.Err != nil {
				status = "error: " + s.Err.Error()
				hasErrors = true
			}
			fmt.Fprintf(output.Writer, "  %-20s %s\n", s.Slug, status)
		}

		// Show filter reminder if filters were active.
		if typeFilter != "" || nameFilter != "" {
			filters := []string{}
			if typeFilter != "" {
				filters = append(filters, "type="+typeFilter)
			}
			if nameFilter != "" {
				filters = append(filters, "name="+nameFilter)
			}
			fmt.Fprintf(output.Writer, "  (filtered by %s)\n", strings.Join(filters, ", "))
		}

		if hasErrors {
			return fmt.Errorf("one or more provider exports failed")
		}
	}

	return nil
}

// exportWithConverter handles export with cross-provider conversion.
// Returns (exportedItem, true) if the converter handled the item.
// Returns (nil, true) if the converter skipped it (not compatible).
// Returns (nil, false) if the converter doesn't apply (fall through to default copy).
func exportWithConverter(item catalog.ContentItem, prov provider.Provider, toSlug string, conv converter.Converter, installDir string) (*exportedItem, bool) {
	srcProvider := effectiveProvider(item)

	// Same provider + has .source/ → copy original verbatim (lossless)
	if converter.HasSourceFile(item) && srcProvider == toSlug {
		srcPath := converter.SourceFilePath(item)
		if srcPath == "" {
			return nil, false
		}
		dest := filepath.Join(installDir, item.Name, filepath.Base(srcPath))
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, false
		}
		if err := installer.CopyContent(srcPath, dest); err != nil {
			return nil, false
		}
		return &exportedItem{
			Name:        item.Name,
			Type:        string(item.Type),
			Destination: dest,
		}, true
	}

	// Cross-provider → canonicalize then render
	if srcProvider != "" && srcProvider != toSlug {
		contentFile := converter.ResolveContentFile(item)
		if contentFile == "" {
			return nil, false
		}
		content, err := os.ReadFile(contentFile)
		if err != nil {
			return nil, false
		}

		// Canonicalize from source provider format, then render to target
		canonical, err := conv.Canonicalize(content, srcProvider)
		if err != nil {
			return nil, false
		}

		rendered, err := conv.Render(canonical.Content, prov)
		if err != nil {
			return nil, false
		}

		// nil Content means skip
		if rendered.Content == nil {
			return nil, true
		}

		dest := filepath.Join(installDir, item.Name, rendered.Filename)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, false
		}
		if err := os.WriteFile(dest, rendered.Content, 0644); err != nil {
			return nil, false
		}

		// Write any extra files (e.g. generated LLM hook wrapper scripts)
		for name, content := range rendered.ExtraFiles {
			extraPath := filepath.Join(filepath.Dir(dest), name)
			if err := os.WriteFile(extraPath, content, 0755); err != nil {
				// Non-fatal: warn but continue
				rendered.Warnings = append(rendered.Warnings, fmt.Sprintf("failed to write %s: %s", name, err))
			}
		}

		return &exportedItem{
			Name:        item.Name,
			Type:        string(item.Type),
			Destination: dest,
			Converted:   true,
			Warnings:    rendered.Warnings,
		}, true
	}

	// No conversion needed — fall through to default copy
	return nil, false
}
