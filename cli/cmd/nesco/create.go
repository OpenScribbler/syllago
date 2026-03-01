package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/metadata"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <type> <name>",
	Short: "Scaffold a new content item in local/",
	Long: `Creates a new content item directory under local/ with template files
and .nesco.yaml metadata.

Examples:
  nesco create skills my-new-skill
  nesco create rules my-rule --provider claude-code`,
	Args: cobra.ExactArgs(2),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringP("provider", "p", "", "Provider slug (required for rules, hooks, commands)")
	rootCmd.AddCommand(createCmd)
}

// nameRegex validates item names: letters, numbers, hyphens, underscores.
var nameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateCreateArgs checks that the content type is valid, the name is
// well-formed, and --provider is supplied for provider-specific types.
func validateCreateArgs(typeName, name, providerSlug string) (catalog.ContentType, error) {
	// Validate type
	var ct catalog.ContentType
	for _, t := range catalog.AllContentTypes() {
		if string(t) == typeName {
			ct = t
			break
		}
	}
	if ct == "" {
		valid := make([]string, len(catalog.AllContentTypes()))
		for i, t := range catalog.AllContentTypes() {
			valid[i] = string(t)
		}
		return "", fmt.Errorf("unknown content type %q (valid: %s)", typeName, strings.Join(valid, ", "))
	}

	// Validate name
	if !nameRegex.MatchString(name) {
		return "", fmt.Errorf("invalid name %q (must contain only letters, numbers, hyphens, underscores)", name)
	}

	// Provider-specific types require --provider
	if !ct.IsUniversal() && providerSlug == "" {
		return "", fmt.Errorf("%s is provider-specific; use --provider <slug>", ct)
	}

	return ct, nil
}

// destDirForCreate returns the target directory for a new item.
// Universal types: local/<type>/<name>/
// Provider-specific types: local/<type>/<provider>/<name>/
func destDirForCreate(root string, ct catalog.ContentType, name, providerSlug string) string {
	if ct.IsUniversal() {
		return filepath.Join(root, "local", string(ct), name)
	}
	return filepath.Join(root, "local", string(ct), providerSlug, name)
}

// scaffoldFromTemplate copies template files from templates/<type>/ into dest,
// replacing {{NAME}} placeholders with the item name. If no template exists,
// it just creates the empty directory.
func scaffoldFromTemplate(root, dest, name string, ct catalog.ContentType) error {
	templateDir := filepath.Join(root, "templates", string(ct))
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return os.MkdirAll(dest, 0755)
	}
	return copyDir(templateDir, dest, name)
}

// copyDir recursively copies src to dst, replacing {{NAME}} in file contents.
func copyDir(src, dst, name string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath, name); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a single file, replacing {{NAME}} placeholders in the content.
func copyFile(src, dst, name string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	content := strings.ReplaceAll(string(data), "{{NAME}}", name)

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Preserve the original file's permissions.
	return os.WriteFile(dst, []byte(content), srcInfo.Mode())
}

func runCreate(cmd *cobra.Command, args []string) error {
	typeName := args[0]
	name := args[1]
	providerSlug, _ := cmd.Flags().GetString("provider")

	ct, err := validateCreateArgs(typeName, name, providerSlug)
	if err != nil {
		return err
	}

	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find nesco content repository: %w", err)
	}

	dest := destDirForCreate(root, ct, name, providerSlug)

	// Check if item already exists
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("item already exists at %s", dest)
	}

	// Scaffold from template (or create empty directory)
	if err := scaffoldFromTemplate(root, dest, name, ct); err != nil {
		return fmt.Errorf("scaffolding template: %w", err)
	}

	// Write .nesco.yaml metadata
	now := time.Now()
	meta := &metadata.Meta{
		ID:        metadata.NewID(),
		Name:      name,
		Type:      string(ct),
		CreatedAt: &now,
	}
	if err := metadata.Save(dest, meta); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	cmd.Printf("Created %s/%s at %s\n", ct, name, dest)
	return nil
}
