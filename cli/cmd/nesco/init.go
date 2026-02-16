package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/config"
	"github.com/holdenhewett/romanesco/cli/internal/output"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize nesco for this project",
	Long:  "Detects AI coding tools in use, creates .nesco/config.json with provider selection.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().Bool("yes", false, "Skip interactive confirmation")
	initCmd.Flags().Bool("force", false, "Overwrite existing config")
	rootCmd.AddCommand(initCmd)
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
	var detected []provider.Provider
	for _, prov := range provider.AllProviders {
		if prov.Detect != nil && prov.Detect(home) {
			detected = append(detected, prov)
		}
	}

	if output.JSON {
		type initResult struct {
			Detected   []string `json:"detected"`
			ConfigPath string   `json:"configPath"`
		}
		slugs := make([]string, len(detected))
		for i, p := range detected {
			slugs[i] = p.Slug
		}
		output.Print(initResult{
			Detected:   slugs,
			ConfigPath: config.FilePath(root),
		})
	} else {
		fmt.Printf("Detected AI tools:\n")
		for _, p := range detected {
			fmt.Printf("  + %s\n", p.Name)
		}
		if len(detected) == 0 {
			fmt.Println("  (none detected)")
		}
	}

	yes, _ := cmd.Flags().GetBool("yes")
	if !yes && os.Getenv("NESCO_NO_PROMPT") != "1" && !output.JSON {
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
	return config.Save(root, cfg)
}
