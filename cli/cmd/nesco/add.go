package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/gitutil"
	"github.com/holdenhewett/romanesco/cli/internal/installer"
	"github.com/holdenhewett/romanesco/cli/internal/metadata"
	"github.com/holdenhewett/romanesco/cli/internal/output"
	"github.com/holdenhewett/romanesco/cli/internal/readme"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <source-path>",
	Short: "Add content to the repository (non-interactive)",
	Long:  "Copies content from source path into my-tools/ with metadata and README generation.",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().String("type", "", "Content type (required): skills, agents, prompts, mcp, apps, rules, hooks, commands")
	addCmd.MarkFlagRequired("type")
	addCmd.Flags().String("provider", "", "Provider slug (required for rules, hooks, commands)")
	addCmd.Flags().String("name", "", "Item name (defaults to source directory basename)")
	rootCmd.AddCommand(addCmd)
}

type addResult struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Provider      string `json:"provider,omitempty"`
	Destination   string `json:"destination"`
	ReadmeCreated bool   `json:"readmeCreated"`
}

func runAdd(cmd *cobra.Command, args []string) error {
	sourcePath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolving source path: %w", err)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		output.PrintError(1, "source path not found: "+sourcePath, "")
		return fmt.Errorf("source not found: %s", sourcePath)
	}

	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find romanesco repo: %w", err)
	}

	typeStr, _ := cmd.Flags().GetString("type")
	ct := catalog.ContentType(typeStr)
	if !slices.Contains(catalog.AllContentTypes(), ct) {
		output.PrintError(1, "invalid content type: "+typeStr,
			"Valid types: skills, agents, prompts, mcp, apps, rules, hooks, commands")
		return fmt.Errorf("invalid content type: %s", typeStr)
	}

	providerSlug, _ := cmd.Flags().GetString("provider")
	if !ct.IsUniversal() && providerSlug == "" {
		output.PrintError(1, "--provider is required for "+typeStr,
			"Example: --provider claude-code")
		return fmt.Errorf("--provider required for %s", typeStr)
	}

	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		name = filepath.Base(sourcePath)
	}

	// Build destination path
	var dest string
	if ct.IsUniversal() {
		dest = filepath.Join(root, "my-tools", string(ct), name)
	} else {
		dest = filepath.Join(root, "my-tools", string(ct), providerSlug, name)
	}

	if _, err := os.Stat(dest); err == nil {
		output.PrintError(1, "destination already exists: "+dest,
			"Remove it first or choose a different --name")
		return fmt.Errorf("destination exists: %s", dest)
	}

	if err := installer.CopyContent(sourcePath, dest); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}

	// Generate .romanesco.yaml metadata
	now := time.Now()
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       name,
		Type:       string(ct),
		Source:     sourcePath,
		ImportedAt: &now,
		ImportedBy: gitutil.Username(),
	}
	if ct.IsUniversal() {
		if err := metadata.Save(dest, meta); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save metadata for %s: %s\n", name, err)
		}
	} else {
		if err := metadata.SaveProvider(filepath.Dir(dest), name, meta); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save metadata for %s: %s\n", name, err)
		}
	}

	// Generate README if missing
	readmeCreated, _ := readme.EnsureReadme(dest, name, string(ct), "")

	result := addResult{
		Name:          name,
		Type:          string(ct),
		Provider:      providerSlug,
		Destination:   dest,
		ReadmeCreated: readmeCreated,
	}

	if output.JSON {
		output.Print(result)
	} else {
		fmt.Printf("Added %s to %s\n", name, dest)
		if readmeCreated {
			fmt.Fprintf(os.Stderr, "hint: Generated placeholder README.md for %s. An LLM can improve it.\n", name)
		}
	}

	return nil
}

