package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/installer"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize nesco for this project",
	Long:  "Detects AI coding tools in use, creates .nesco/config.json with provider selection.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolP("yes", "y", false, "Skip interactive confirmation")
	initCmd.Flags().Bool("force", false, "Overwrite existing config")
	rootCmd.AddCommand(initCmd)
}

type initResult struct {
	Detected   []string        `json:"detected"`
	ConfigPath string          `json:"configPath"`
	Installed  []installedItem `json:"installed,omitempty"`
}

type installedItem struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Path     string `json:"path"`
}

func runInit(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")
	if config.Exists(root) && !force {
		output.PrintError(1, ".nesco/config.json already exists", "Use --force to overwrite")
		return fmt.Errorf("config already exists")
	}

	home, _ := os.UserHomeDir()
	detected := provider.DetectedOnly(home)

	if !output.JSON {
		fmt.Printf("Detected AI tools:\n")
		for _, p := range detected {
			fmt.Printf("  + %s\n", p.Name)
		}
		if len(detected) == 0 {
			fmt.Println("  (none detected)")
		}
	}

	yes, _ := cmd.Flags().GetBool("yes")
	if !yes && isInteractive() && os.Getenv("NESCO_NO_PROMPT") != "1" && !output.JSON {
		fmt.Printf("\nSave to .nesco/config.json? [Y/n] ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "n" {
			return nil
		}
	}

	slugs := make([]string, len(detected))
	for i, p := range detected {
		slugs[i] = p.Slug
	}
	cfg := &config.Config{
		Providers: slugs,
	}
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	// Install built-in content if we're in a nesco repo
	var installed []installedItem
	repoRoot, repoErr := findContentRepoRoot()
	if repoErr == nil {
		installed = installBuiltins(cmd, repoRoot, detected)
	}

	// Output results (JSON is deferred to here so it can include installed items)
	if output.JSON {
		output.Print(initResult{
			Detected:   slugs,
			ConfigPath: config.FilePath(root),
			Installed:  installed,
		})
	}

	return nil
}

// installBuiltins discovers items tagged "builtin" and installs them to detected providers.
func installBuiltins(cmd *cobra.Command, repoRoot string, detected []provider.Provider) []installedItem {
	cat, err := catalog.Scan(repoRoot)
	if err != nil {
		return nil
	}

	// Find items with the "builtin" tag
	var builtins []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Meta != nil && slices.Contains(item.Meta.Tags, "builtin") {
			builtins = append(builtins, item)
		}
	}
	if len(builtins) == 0 {
		return nil
	}

	if !output.JSON {
		fmt.Printf("\nBuilt-in content available:\n")
		for _, item := range builtins {
			fmt.Printf("  + %s (%s)\n", item.Name, item.Type.Label())
		}
	}

	yes, _ := cmd.Flags().GetBool("yes")
	if !yes && isInteractive() && os.Getenv("NESCO_NO_PROMPT") != "1" && !output.JSON {
		fmt.Printf("\nInstall built-in content to detected providers? [Y/n] ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "n" {
			return nil
		}
	}

	var installed []installedItem
	for _, item := range builtins {
		for _, prov := range detected {
			// Check if provider supports this content type
			if installer.CheckStatus(item, prov, repoRoot) == installer.StatusNotAvailable {
				continue
			}
			// Skip if already installed
			if installer.CheckStatus(item, prov, repoRoot) == installer.StatusInstalled {
				continue
			}

			desc, err := installer.Install(item, prov, repoRoot, installer.MethodSymlink)
			if err != nil {
				if !output.JSON {
					fmt.Fprintf(os.Stderr, "  warning: could not install %s to %s: %s\n", item.Name, prov.Name, err)
				}
				continue
			}

			installed = append(installed, installedItem{
				Name:     item.Name,
				Provider: prov.Slug,
				Path:     desc,
			})
			if !output.JSON {
				fmt.Printf("  Installed %s -> %s\n", item.Name, prov.Name)
			}
		}
	}

	return installed
}
