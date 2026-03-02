package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// Scaffold creates a new registry directory structure at targetDir/name.
// It creates subdirectories for each content type, a registry.yaml manifest,
// and a README.md with basic usage instructions.
//
// Returns an error if the name is invalid or the directory already exists.
func Scaffold(targetDir, name, description string) error {
	if !catalog.IsValidItemName(name) {
		return fmt.Errorf("registry name %q contains invalid characters (use letters, numbers, - and _)", name)
	}

	dir := filepath.Join(targetDir, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("directory %q already exists", dir)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating registry directory: %w", err)
	}

	// Create content type directories with .gitkeep so git tracks them.
	for _, ct := range catalog.AllContentTypes() {
		ctDir := filepath.Join(dir, string(ct))
		if err := os.MkdirAll(ctDir, 0755); err != nil {
			return fmt.Errorf("creating %s directory: %w", ct, err)
		}
		if err := os.WriteFile(filepath.Join(ctDir, ".gitkeep"), []byte(""), 0644); err != nil {
			return fmt.Errorf("creating .gitkeep in %s: %w", ct, err)
		}
	}

	// Write registry.yaml using the Manifest struct for format consistency.
	desc := description
	if desc == "" {
		desc = fmt.Sprintf("%s registry", name)
	}
	manifest := Manifest{
		Name:        name,
		Description: desc,
		Version:     "0.1.0",
	}
	yamlBytes, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshalling registry.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), yamlBytes, 0644); err != nil {
		return fmt.Errorf("writing registry.yaml: %w", err)
	}

	// Write README.md with basic usage instructions.
	var lines []string
	lines = append(lines, "# "+name, "", desc, "")
	lines = append(lines, "## Using this registry", "")
	lines = append(lines, "```sh", "syllago registry add <git-url>", "syllago registry sync", "```", "")
	lines = append(lines, "## Structure", "")
	for _, ct := range catalog.AllContentTypes() {
		lines = append(lines, fmt.Sprintf("- `%s/` -- %s", ct, ct.Label()))
	}
	lines = append(lines, "")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("writing README.md: %w", err)
	}

	return nil
}
