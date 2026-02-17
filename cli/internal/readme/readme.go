package readme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Generate produces minimal README.md content for a content item.
func Generate(name, contentType, description string) string {
	// Convert kebab-case/snake_case name to title case
	title := strings.ReplaceAll(name, "-", " ")
	title = strings.ReplaceAll(title, "_", " ")
	title = cases.Title(language.English).String(title)

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n", title)

	if description != "" {
		fmt.Fprintf(&sb, "\n%s\n", description)
	}

	fmt.Fprintf(&sb, "\n**Type:** %s\n", contentType)
	return sb.String()
}

// EnsureReadme checks if README.md exists in itemDir, creates one via Generate() if not.
// Returns true if a README was created, false if one already existed.
func EnsureReadme(itemDir, name, contentType, description string) (bool, error) {
	readmePath := filepath.Join(itemDir, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		return false, nil // already exists
	}

	content := Generate(name, contentType, description)
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		return false, fmt.Errorf("creating directory: %w", err)
	}
	if err := os.WriteFile(readmePath, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("writing README.md: %w", err)
	}
	return true, nil
}
