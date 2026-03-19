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

var configPathsCmd = &cobra.Command{
	Use:   "paths",
	Short: "Manage custom provider path overrides",
	Long:  "Configure custom base directories or per-type path overrides for providers.\nOverrides are stored in the global config (~/.syllago/config.json) since paths are machine-specific.",
	Example: `  syllago config paths show
  syllago config paths set claude-code --base-dir ~/custom/claude
  syllago config paths clear claude-code`,
}

var configPathsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show configured path overrides",
	Example: `  # Show all path overrides
  syllago config paths show

  # Show overrides for a specific provider
  syllago config paths show --provider claude-code`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadGlobal()
		if err != nil {
			return fmt.Errorf("loading global config: %w", err)
		}

		filterProvider, _ := cmd.Flags().GetString("provider")

		if len(cfg.ProviderPaths) == 0 {
			if !output.JSON {
				fmt.Fprintln(output.Writer, "No path overrides configured.")
				fmt.Fprintln(output.Writer, "Use 'syllago config paths set <provider>' to add overrides.")
			} else {
				output.Print(map[string]any{})
			}
			return nil
		}

		if output.JSON {
			if filterProvider != "" {
				ppc, ok := cfg.ProviderPaths[filterProvider]
				if !ok {
					output.Print(map[string]any{})
				} else {
					output.Print(map[string]config.ProviderPathConfig{filterProvider: ppc})
				}
			} else {
				output.Print(cfg.ProviderPaths)
			}
			return nil
		}

		// Human-readable output
		for slug, ppc := range cfg.ProviderPaths {
			if filterProvider != "" && slug != filterProvider {
				continue
			}
			fmt.Fprintf(output.Writer, "%s:\n", slug)
			if ppc.BaseDir != "" {
				fmt.Fprintf(output.Writer, "  base-dir: %s\n", ppc.BaseDir)
			}
			for ct, path := range ppc.Paths {
				fmt.Fprintf(output.Writer, "  %s: %s\n", ct, path)
			}
		}
		return nil
	},
}

var configPathsSetCmd = &cobra.Command{
	Use:   "set <provider>",
	Short: "Set path overrides for a provider",
	Long:  "Set a base directory or per-type path override for a provider.\nPaths must be absolute (start with / or ~/).",
	Example: `  # Set a base directory for a provider
  syllago config paths set claude-code --base-dir ~/custom/claude

  # Set a per-type path override
  syllago config paths set cursor --type rules --path ~/my-rules`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		baseDir, _ := cmd.Flags().GetString("base-dir")
		typeName, _ := cmd.Flags().GetString("type")
		pathValue, _ := cmd.Flags().GetString("path")

		if baseDir == "" && typeName == "" {
			return fmt.Errorf("specify --base-dir or both --type and --path")
		}
		if (typeName == "") != (pathValue == "") {
			return fmt.Errorf("--type and --path must be used together")
		}

		// Validate provider slug (warn if unknown, don't block)
		if findProviderBySlug(slug) == nil {
			var slugs []string
			for _, p := range provider.AllProviders {
				slugs = append(slugs, p.Slug)
			}
			fmt.Fprintf(output.ErrWriter, "Warning: '%s' is not a known provider slug.\n", slug)
			fmt.Fprintf(output.ErrWriter, "  Known providers: %s\n", strings.Join(slugs, ", "))
		}

		// Validate path is absolute
		if baseDir != "" && !isAbsoluteOrTilde(baseDir) {
			return fmt.Errorf("--base-dir must be an absolute path (start with / or ~/), got %q", baseDir)
		}
		if pathValue != "" && !isAbsoluteOrTilde(pathValue) {
			return fmt.Errorf("--path must be an absolute path (start with / or ~/), got %q", pathValue)
		}

		// Validate content type
		if typeName != "" && !isValidContentType(typeName) {
			var names []string
			for _, ct := range catalog.AllContentTypes() {
				names = append(names, string(ct))
			}
			return fmt.Errorf("unknown content type %q; valid types: %s", typeName, strings.Join(names, ", "))
		}

		// Clean paths and warn if they don't exist on disk
		if baseDir != "" {
			baseDir = filepath.Clean(baseDir)
			expanded, _ := config.ExpandHome(baseDir)
			if expanded != "" {
				if _, err := os.Stat(expanded); err != nil {
					fmt.Fprintf(output.ErrWriter, "Warning: path does not exist: %s\n", baseDir)
				}
			}
		}
		if pathValue != "" {
			pathValue = filepath.Clean(pathValue)
			expanded, _ := config.ExpandHome(pathValue)
			if expanded != "" {
				if _, err := os.Stat(expanded); err != nil {
					fmt.Fprintf(output.ErrWriter, "Warning: path does not exist: %s\n", pathValue)
				}
			}
		}

		cfg, err := config.LoadGlobal()
		if err != nil {
			return fmt.Errorf("loading global config: %w", err)
		}

		if cfg.ProviderPaths == nil {
			cfg.ProviderPaths = make(map[string]config.ProviderPathConfig)
		}
		ppc := cfg.ProviderPaths[slug]

		if baseDir != "" {
			ppc.BaseDir = baseDir
		}
		if typeName != "" {
			if ppc.Paths == nil {
				ppc.Paths = make(map[string]string)
			}
			ppc.Paths[typeName] = pathValue
		}

		cfg.ProviderPaths[slug] = ppc

		if err := config.SaveGlobal(cfg); err != nil {
			return fmt.Errorf("saving global config: %w", err)
		}

		if baseDir != "" {
			fmt.Fprintf(output.Writer, "Set %s base-dir: %s\n", slug, baseDir)
		}
		if typeName != "" {
			fmt.Fprintf(output.Writer, "Set %s %s path: %s\n", slug, typeName, pathValue)
		}
		return nil
	},
}

var configPathsClearCmd = &cobra.Command{
	Use:   "clear <provider>",
	Short: "Clear path overrides for a provider",
	Example: `  # Clear all overrides for a provider
  syllago config paths clear claude-code

  # Clear only a specific type override
  syllago config paths clear cursor --type rules`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		typeName, _ := cmd.Flags().GetString("type")

		cfg, err := config.LoadGlobal()
		if err != nil {
			return fmt.Errorf("loading global config: %w", err)
		}

		if cfg.ProviderPaths == nil {
			return fmt.Errorf("no path overrides configured for %q", slug)
		}

		ppc, ok := cfg.ProviderPaths[slug]
		if !ok {
			return fmt.Errorf("no path overrides configured for %q", slug)
		}

		if typeName != "" {
			// Clear specific type
			delete(ppc.Paths, typeName)
			if ppc.BaseDir == "" && len(ppc.Paths) == 0 {
				delete(cfg.ProviderPaths, slug)
			} else {
				cfg.ProviderPaths[slug] = ppc
			}
			fmt.Fprintf(output.Writer, "Cleared %s %s path override\n", slug, typeName)
		} else {
			// Clear entire provider
			delete(cfg.ProviderPaths, slug)
			fmt.Fprintf(output.Writer, "Cleared all path overrides for %s\n", slug)
		}

		// Clean up empty map
		if len(cfg.ProviderPaths) == 0 {
			cfg.ProviderPaths = nil
		}

		if err := config.SaveGlobal(cfg); err != nil {
			return fmt.Errorf("saving global config: %w", err)
		}
		return nil
	},
}

func init() {
	configPathsShowCmd.Flags().String("provider", "", "filter by provider slug")

	configPathsSetCmd.Flags().String("base-dir", "", "base directory override (replaces home dir)")
	configPathsSetCmd.Flags().String("type", "", "content type (e.g., skills, hooks)")
	configPathsSetCmd.Flags().String("path", "", "absolute path for the content type")

	configPathsClearCmd.Flags().String("type", "", "specific content type to clear (omit to clear all)")

	configPathsCmd.AddCommand(configPathsShowCmd, configPathsSetCmd, configPathsClearCmd)
}

// isAbsoluteOrTilde returns true if a path starts with / or ~/.
func isAbsoluteOrTilde(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, "~/")
}

// isValidContentType checks if a string matches a known content type.
func isValidContentType(name string) bool {
	for _, ct := range catalog.AllContentTypes() {
		if string(ct) == name {
			return true
		}
	}
	return false
}

