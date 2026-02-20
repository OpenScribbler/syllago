package main

import (
	"fmt"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show nesco capabilities",
	Long:  "Machine-readable capability manifest. Useful for agents discovering nesco's features.",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := version
		if v == "" {
			v = "(dev build)"
		}
		manifest := map[string]any{
			"version":      v,
			"contentTypes": catalog.AllContentTypes(),
			"providers":    providerSlugs(),
			"commands":     []string{"init", "import", "config", "info"},
		}
		if output.JSON {
			output.Print(manifest)
		} else {
			fmt.Fprintf(output.Writer, "nesco %s\n\n", v)
			fmt.Fprintf(output.Writer, "Content types: %d\n", len(catalog.AllContentTypes()))
			for _, ct := range catalog.AllContentTypes() {
				fmt.Fprintf(output.Writer, "  - %s\n", ct.Label())
			}
			fmt.Fprintln(output.Writer, "\n  Note: Prompts and Apps are standalone types (not installable to providers)")
			fmt.Fprintf(output.Writer, "\nProviders: %d\n", len(provider.AllProviders))
			for _, p := range provider.AllProviders {
				fmt.Fprintf(output.Writer, "  - %s (%s)\n", p.Name, p.Slug)
			}
		}
		return nil
	},
}

var infoProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List all known providers",
	RunE: func(cmd *cobra.Command, args []string) error {
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

var infoFormatsCmd = &cobra.Command{
	Use:   "formats",
	Short: "List supported file formats",
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
