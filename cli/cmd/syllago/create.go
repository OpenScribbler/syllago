package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <type> <name>",
	Short: "Scaffold a new content item in the global library",
	Long: `Creates a new content item directory under ~/.syllago/content/ with
.syllago.yaml metadata.

Examples:
  syllago create skills my-new-skill
  syllago create rules my-rule --provider claude-code`,
	Args: cobra.ExactArgs(2),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringP("provider", "p", "", "Provider slug (required for rules, hooks, commands)")
	rootCmd.AddCommand(createCmd)
}

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
	if errMsg := catalog.ValidateUserName(name); errMsg != "" {
		return "", fmt.Errorf("invalid name %q: %s", name, errMsg)
	}

	// Provider-specific types require --provider
	if !ct.IsUniversal() && providerSlug == "" {
		return "", fmt.Errorf("%s is provider-specific; use --provider <slug>", ct)
	}

	return ct, nil
}

// destDirForCreate returns the target directory for a new item in the global library.
// Universal types: <globalDir>/<type>/<name>/
// Provider-specific types: <globalDir>/<type>/<provider>/<name>/
func destDirForCreate(globalDir string, ct catalog.ContentType, name, providerSlug string) string {
	if ct.IsUniversal() {
		return filepath.Join(globalDir, string(ct), name)
	}
	return filepath.Join(globalDir, string(ct), providerSlug, name)
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

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	dest := destDirForCreate(globalDir, ct, name, providerSlug)

	// Check if item already exists
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("item already exists at %s", dest)
	}

	// Create the item directory (with optional template scaffold)
	if err := scaffoldFromTemplate(globalDir, dest, name, ct); err != nil {
		return fmt.Errorf("scaffolding template: %w", err)
	}

	// Write .syllago.yaml metadata
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
