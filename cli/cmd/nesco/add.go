package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/converter"
	"github.com/holdenhewett/nesco/cli/internal/gitutil"
	"github.com/holdenhewett/nesco/cli/internal/installer"
	"github.com/holdenhewett/nesco/cli/internal/metadata"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/readme"
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
		return fmt.Errorf("could not find nesco repo: %w", err)
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

	// Generate .nesco.yaml metadata
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

	// Canonicalize if a converter exists and source provider is known
	if conv := converter.For(ct); conv != nil && providerSlug != "" {
		if info, statErr := os.Stat(dest); statErr == nil && info.IsDir() {
			if warnings := canonicalizeContent(dest, conv, providerSlug, meta); len(warnings) > 0 {
				for _, w := range warnings {
					fmt.Fprintf(os.Stderr, "warning: %s\n", w)
				}
			}
			// Re-save metadata with source tracking fields
			if ct.IsUniversal() {
				_ = metadata.Save(dest, meta)
			} else {
				_ = metadata.SaveProvider(filepath.Dir(dest), name, meta)
			}
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
		// Next steps hints
		fmt.Fprintf(output.Writer, "  -> Browse in TUI: nesco\n")
		fmt.Fprintf(output.Writer, "  -> Export to a provider: nesco export --to claude-code\n")
	}

	return nil
}

// canonicalizeContent finds the content file in an item directory, preserves
// the original in .source/, and overwrites with canonical format. Updates meta
// with source tracking fields. Returns any warnings from canonicalization.
func canonicalizeContent(itemDir string, conv converter.Converter, providerSlug string, meta *metadata.Meta) []string {
	// Find the content file to canonicalize
	contentFile := findContentFile(itemDir, providerSlug)
	if contentFile == "" {
		return nil
	}

	content, err := os.ReadFile(contentFile)
	if err != nil {
		return []string{fmt.Sprintf("reading content file: %s", err)}
	}

	// Canonicalize
	result, err := conv.Canonicalize(content, providerSlug)
	if err != nil {
		return []string{fmt.Sprintf("canonicalization failed: %s", err)}
	}

	// Preserve original in .source/
	sourceDir := filepath.Join(itemDir, converter.SourceDir)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return []string{fmt.Sprintf("creating .source/: %s", err)}
	}
	sourceFile := filepath.Join(sourceDir, filepath.Base(contentFile))
	if err := os.WriteFile(sourceFile, content, 0644); err != nil {
		return []string{fmt.Sprintf("preserving source: %s", err)}
	}

	// Write canonical content
	canonicalPath := filepath.Join(itemDir, result.Filename)
	if err := os.WriteFile(canonicalPath, result.Content, 0644); err != nil {
		return []string{fmt.Sprintf("writing canonical: %s", err)}
	}

	// Remove original if the filename changed (e.g. rule.mdc → rule.md)
	if contentFile != canonicalPath {
		os.Remove(contentFile)
	}

	// Update metadata with source tracking
	ext := filepath.Ext(filepath.Base(contentFile))
	if len(ext) > 0 {
		ext = ext[1:] // strip leading dot
	}
	meta.SourceProvider = providerSlug
	meta.SourceFormat = ext

	return result.Warnings
}

// findContentFile locates the main content file in an item directory.
// For cursor rules, looks for .mdc first. Then .md, .toml, .json files.
func findContentFile(itemDir, providerSlug string) string {
	entries, err := os.ReadDir(itemDir)
	if err != nil {
		return ""
	}

	// For Cursor, prefer .mdc files
	if providerSlug == "cursor" {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".mdc" {
				return filepath.Join(itemDir, e.Name())
			}
		}
	}

	// For Gemini commands, prefer .toml files
	if providerSlug == "gemini-cli" {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".toml" {
				return filepath.Join(itemDir, e.Name())
			}
		}
	}

	// Look for .md files (excluding README.md, LLM-PROMPT.md)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" &&
			e.Name() != "README.md" && e.Name() != "LLM-PROMPT.md" {
			return filepath.Join(itemDir, e.Name())
		}
	}

	// Look for .json files (hooks, MCP configs)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			return filepath.Join(itemDir, e.Name())
		}
	}

	// Look for .toml files (Gemini commands)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".toml" {
			return filepath.Join(itemDir, e.Name())
		}
	}

	return ""
}
