package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show syllago capabilities",
	Long:  "Machine-readable capability manifest. Useful for agents discovering syllago's features.",
	Example: `  # Show capabilities summary
  syllago info

  # JSON output for agent consumption
  syllago info --json`,
	RunE: runInfo,
}

type detectedProvider struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Detected bool   `json:"detected"`
	Path     string `json:"path,omitempty"`
}

type registryInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func runInfo(cmd *cobra.Command, args []string) error {
	v := version
	if v == "" {
		v = "(dev build)"
	}

	// Detect providers
	detected := provider.DetectProviders()
	var provInfos []detectedProvider
	for _, p := range detected {
		pi := detectedProvider{Name: p.Name, Slug: p.Slug, Detected: p.Detected}
		if p.Detected {
			home, _ := os.UserHomeDir()
			pi.Path = filepath.Join(home, p.ConfigDir)
		}
		provInfos = append(provInfos, pi)
	}

	// Library location
	libraryDir := catalog.GlobalContentDir()

	// Config paths
	globalConfigPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		globalConfigPath = filepath.Join(home, ".syllago", "config.json")
	}
	projectRoot, _ := findProjectRoot()
	projectConfigPath := ""
	if projectRoot != "" {
		projectConfigPath = filepath.Join(projectRoot, ".syllago.json")
	}

	// Registries from merged config
	var registries []registryInfo
	globalCfg, _ := config.LoadGlobal()
	projectCfg, _ := config.Load(projectRoot)
	merged := config.Merge(globalCfg, projectCfg)
	for _, r := range merged.Registries {
		registries = append(registries, registryInfo{Name: r.Name, URL: r.URL})
	}

	if output.JSON {
		manifest := map[string]any{
			"version":      v,
			"contentTypes": catalog.AllContentTypes(),
			"providers":    provInfos,
			"library":      libraryDir,
			"config": map[string]string{
				"global":  globalConfigPath,
				"project": projectConfigPath,
			},
			"registries": registries,
			"commands":   []string{"init", "add", "install", "list", "inspect", "convert", "registry", "config", "info"},
		}
		output.Print(manifest)
		return nil
	}

	fmt.Fprintf(output.Writer, "syllago %s\n\n", v)

	fmt.Fprintf(output.Writer, "Library: %s\n\n", libraryDir)

	fmt.Fprintf(output.Writer, "Providers:\n")
	for _, p := range provInfos {
		status := "x"
		if p.Detected {
			status = "+"
		}
		if p.Path != "" {
			fmt.Fprintf(output.Writer, "  [%s] %s (%s) — %s\n", status, p.Name, p.Slug, p.Path)
		} else {
			fmt.Fprintf(output.Writer, "  [%s] %s (%s)\n", status, p.Name, p.Slug)
		}
	}

	fmt.Fprintf(output.Writer, "\nContent types: %s\n", joinContentTypes())

	fmt.Fprintf(output.Writer, "\nConfig:\n")
	fmt.Fprintf(output.Writer, "  global:  %s\n", fileStatus(globalConfigPath))
	if projectConfigPath != "" {
		fmt.Fprintf(output.Writer, "  project: %s\n", fileStatus(projectConfigPath))
	}

	if len(registries) > 0 {
		fmt.Fprintf(output.Writer, "\nRegistries:\n")
		for _, r := range registries {
			fmt.Fprintf(output.Writer, "  - %s (%s)\n", r.Name, r.URL)
		}
	}

	return nil
}

func joinContentTypes() string {
	types := catalog.AllContentTypes()
	labels := make([]string, len(types))
	for i, ct := range types {
		labels[i] = ct.Label()
	}
	return strings.Join(labels, ", ")
}

func fileStatus(path string) string {
	if path == "" {
		return "(not set)"
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return path + " (not found)"
}

var infoProvidersCmd = &cobra.Command{
	Use:   "providers [slug]",
	Short: "List providers or show data quality for a specific slug",
	Example: `  # List providers and their supported types
  syllago info providers

  # JSON output
  syllago info providers --json

  # Data quality summary for one provider
  syllago info providers claude-code
  syllago info providers claude-code --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return runInfoProvidersSlug(cmd, args)
		}
		type provInfo struct {
			Name  string   `json:"name"`
			Slug  string   `json:"slug"`
			Types []string `json:"supportedTypes"`
		}
		var infos []provInfo
		for _, p := range provider.AllProviders {
			var types []string
			if p.SupportsType != nil {
				for _, ct := range catalog.AllContentTypes() {
					if p.SupportsType(ct) {
						types = append(types, ct.Label())
					}
				}
			}
			infos = append(infos, provInfo{Name: p.Name, Slug: p.Slug, Types: types})
		}
		if output.JSON {
			output.Print(infos)
		} else {
			for _, info := range infos {
				fmt.Printf("%s (%s)\n", info.Name, info.Slug)
				for _, t := range info.Types {
					fmt.Printf("  - %s\n", t)
				}
			}
		}
		return nil
	},
}

// infoProviderFormatsDir is the path to docs/provider-formats/. Overridable in tests.
var infoProviderFormatsDir = filepath.Join("..", "docs", "provider-formats")

func runInfoProvidersSlug(cmd *cobra.Command, args []string) error {
	slug := args[0]

	providers, trackingIssues, err := loadProviderFormatsDir(infoProviderFormatsDir)
	if err != nil {
		return fmt.Errorf("loading provider formats: %w", err)
	}
	dq := computeDataQuality(providers, trackingIssues)

	entry, ok := dq.Providers[slug]
	if !ok {
		return fmt.Errorf("provider %q not found in %s", slug, infoProviderFormatsDir)
	}

	if output.JSON {
		output.Print(map[string]any{
			"slug":                         slug,
			"unspecified_required_count":   entry.UnspecifiedRequiredCount,
			"unspecified_value_type_count": entry.UnspecifiedValueTypeCount,
			"unspecified_examples_count":   entry.UnspecifiedExamplesCount,
			"tracking_issue":               entry.TrackingIssue,
		})
		return nil
	}

	fmt.Fprintf(output.Writer, "Data quality for %s:\n", slug)
	fmt.Fprintf(output.Writer, "  Extensions missing required field:    %d\n", entry.UnspecifiedRequiredCount)
	fmt.Fprintf(output.Writer, "  Extensions missing value_type field:  %d\n", entry.UnspecifiedValueTypeCount)
	fmt.Fprintf(output.Writer, "  Extensions missing examples:          %d\n", entry.UnspecifiedExamplesCount)
	if entry.TrackingIssue != "" {
		fmt.Fprintf(output.Writer, "\n  Tracking issue: %s\n", entry.TrackingIssue)
	} else {
		fmt.Fprintf(output.Writer, "\n  Tracking issue: (not yet filed)\n")
	}
	return nil
}

var infoFormatsCmd = &cobra.Command{
	Use:     "formats",
	Short:   "List supported file formats",
	Example: `  syllago info formats`,
	RunE: func(cmd *cobra.Command, args []string) error {
		type formatInfo struct {
			Format    string   `json:"format"`
			Extension string   `json:"extension"`
			Providers []string `json:"providers"`
		}
		formats := []formatInfo{
			{Format: "Markdown", Extension: ".md", Providers: []string{"claude-code", "windsurf", "codex", "gemini-cli"}},
			{Format: "Cursor MDC", Extension: ".mdc", Providers: []string{"cursor"}},
			{Format: "JSON", Extension: ".json", Providers: []string{"claude-code", "cursor"}},
		}
		if output.JSON {
			output.Print(formats)
		} else {
			fmt.Fprintln(output.Writer, "Supported formats:")
			for _, f := range formats {
				provList := strings.Join(f.Providers, ", ")
				fmt.Fprintf(output.Writer, "  %s (%s) — used by: %s\n", f.Format, f.Extension, provList)
			}
		}
		return nil
	},
}

func init() {
	infoCmd.AddCommand(infoProvidersCmd, infoFormatsCmd)
	rootCmd.AddCommand(infoCmd)
}

func providerSlugs() []string {
	slugs := make([]string, len(provider.AllProviders))
	for i, p := range provider.AllProviders {
		slugs[i] = p.Slug
	}
	return slugs
}
