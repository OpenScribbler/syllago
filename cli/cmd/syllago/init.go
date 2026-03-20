package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize syllago for this project",
	Long: "Detects AI coding tools in use, creates .syllago/config.json with provider selection.",
	Example: `  # Interactive setup
  syllago init

  # Skip confirmation prompts
  syllago init --yes

  # Overwrite existing config
  syllago init --force`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolP("yes", "y", false, "Skip interactive confirmation")
	initCmd.Flags().Bool("force", false, "Overwrite existing config")
	rootCmd.AddCommand(initCmd)
}

type initResult struct {
	Detected   []string        `json:"detected"`
	ConfigPath string          `json:"configPath"`
	Installed  []initInstalledItem `json:"installed,omitempty"`
}

type initInstalledItem struct {
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
		output.PrintError(1, ".syllago/config.json already exists", "Use --force to overwrite")
		return fmt.Errorf("config already exists")
	}

	home, _ := os.UserHomeDir()
	detected := provider.DetectedOnly(home)
	yes, _ := cmd.Flags().GetBool("yes")

	var slugs []string
	var registryEntry *config.Registry // non-nil if the wizard collected a registry URL
	var scaffoldRegistry string        // non-empty if the wizard wants to create a registry dir

	// Interactive wizard: let the user toggle providers before confirming.
	// Falls through to auto-accept in non-interactive / --yes / --json modes.
	if !yes && !output.JSON && isInteractive() && os.Getenv("SYLLAGO_NO_PROMPT") != "1" {
		allProviders := provider.DetectProvidersWithResolver(nil)
		wizard := newInitWizard(detected, allProviders)
		model := initWizardModel{wizard: wizard}
		p := tea.NewProgram(model)
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("init wizard: %w", err)
		}
		final := finalModel.(initWizardModel)
		if final.wizard.cancelled {
			fmt.Println("Init cancelled.")
			return nil
		}
		slugs = final.wizard.selectedSlugs()

		switch final.wizard.registryAction {
		case "add":
			url := final.wizard.registryURL
			name := registry.NameFromURL(url)
			registryEntry = &config.Registry{Name: name, URL: url}
		case "create":
			scaffoldRegistry = final.wizard.registryName
		}
	} else {
		// Non-interactive: use detected providers as-is
		if !output.JSON {
			fmt.Printf("Detected AI tools:\n")
			for _, p := range detected {
				fmt.Printf("  + %s\n", p.Name)
			}
			if len(detected) == 0 {
				fmt.Println("  (none detected)")
			}
		}
		slugs = make([]string, len(detected))
		for i, p := range detected {
			slugs[i] = p.Slug
		}
	}
	cfg := &config.Config{
		Providers: slugs,
	}
	if registryEntry != nil {
		cfg.Registries = []config.Registry{*registryEntry}
	}
	if err := config.Save(root, cfg); err != nil {
		return err
	}
	if registryEntry != nil && !output.JSON {
		fmt.Printf("  Added registry %q (%s)\n", registryEntry.Name, registryEntry.URL)
	}
	if scaffoldRegistry != "" {
		if err := registry.Scaffold(root, scaffoldRegistry, ""); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not scaffold registry %q: %s\n", scaffoldRegistry, err)
		} else if !output.JSON {
			fmt.Printf("  Created registry directory %q\n", scaffoldRegistry)
		}
	}

	// Create local/ directory for user content
	if err := os.MkdirAll(filepath.Join(root, "local"), 0755); err != nil {
		return fmt.Errorf("creating local/ directory: %w", err)
	}

	// Ensure .gitignore covers local content and registry cache
	if err := ensureGitignoreEntries(root, []string{"local/", ".syllago/registries/"}); err != nil {
		// Non-fatal: warn but don't block init
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %s\n", err)
	}

	// Ensure global content dir exists (first-time setup)
	if home != "" {
		if mkdirErr := ensureGlobalContentDir(home); mkdirErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not create global content dir: %s\n", mkdirErr)
		}
		// Create global config if it doesn't exist yet
		globalCfgPath := filepath.Join(home, ".syllago", config.FileName)
		if _, statErr := os.Stat(globalCfgPath); os.IsNotExist(statErr) {
			globalCfg := &config.Config{Providers: slugs}
			if saveErr := config.SaveGlobal(globalCfg); saveErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not create global config: %s\n", saveErr)
			} else if !output.JSON {
				fmt.Printf("  Created ~/.syllago/config.json\n")
				fmt.Printf("  Created ~/.syllago/content/\n")
			}
		}
	}

	// Install built-in content if we're in a syllago repo
	var installed []initInstalledItem
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

// ensureGitignoreEntries appends gitignore entries that are not already present.
func ensureGitignoreEntries(projectRoot string, entries []string) error {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	existing := ""
	data, err := os.ReadFile(gitignorePath)
	if err == nil {
		existing = string(data)
	}

	var toAdd []string
	for _, entry := range entries {
		if !strings.Contains(existing, entry) {
			toAdd = append(toAdd, entry)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	// Ensure there's a trailing newline before appending
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	content := existing + strings.Join(toAdd, "\n") + "\n"
	return os.WriteFile(gitignorePath, []byte(content), 0644)
}

// ensureGlobalContentDir creates the global content directory at homeDir/.syllago/content/
// if it doesn't already exist.
func ensureGlobalContentDir(homeDir string) error {
	dir := filepath.Join(homeDir, ".syllago", "content")
	return os.MkdirAll(dir, 0755)
}

// installBuiltins discovers items tagged "builtin" and installs them to detected providers.
func installBuiltins(cmd *cobra.Command, repoRoot string, detected []provider.Provider) []initInstalledItem {
	cat, err := catalog.Scan(repoRoot, repoRoot)
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
	if !yes && isInteractive() && os.Getenv("SYLLAGO_NO_PROMPT") != "1" && !output.JSON {
		fmt.Printf("\nInstall built-in content to detected providers? [Y/n] ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "n" {
			return nil
		}
	}

	var installed []initInstalledItem
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

			desc, err := installer.Install(item, prov, repoRoot, installer.MethodSymlink, "")
			if err != nil {
				if !output.JSON {
					fmt.Fprintf(os.Stderr, "  warning: could not install %s to %s: %s\n", item.Name, prov.Name, err)
				}
				continue
			}

			installed = append(installed, initInstalledItem{
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
