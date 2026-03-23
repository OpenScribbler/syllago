// Package errordocs embeds error documentation markdown files into the binary
// and provides lookup by error code. Inspired by Rust's --explain flag.
package errordocs

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed all:docs
var docsFS embed.FS

// Explain returns the documentation markdown for the given error code.
// Code format: "CATALOG_001" -> looks up "docs/catalog-001.md".
func Explain(code string) (string, error) {
	slug := codeToSlug(code)
	data, err := docsFS.ReadFile("docs/" + slug + ".md")
	if err != nil {
		return "", fmt.Errorf("no documentation found for error code %s", code)
	}
	return string(data), nil
}

// ListCodes returns all error codes that have documentation files,
// sorted alphabetically. Reads *.md filenames from the embedded FS
// and converts slugs back to code format.
func ListCodes() []string {
	entries, err := fs.ReadDir(docsFS, "docs")
	if err != nil {
		return nil
	}
	var codes []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || name == ".gitkeep" {
			continue
		}
		slug := strings.TrimSuffix(name, ".md")
		codes = append(codes, slugToCode(slug))
	}
	sort.Strings(codes)
	return codes
}

// codeToSlug converts "CATALOG_001" to "catalog-001".
func codeToSlug(code string) string {
	return strings.ToLower(strings.ReplaceAll(code, "_", "-"))
}

// slugToCode converts "catalog-001" to "CATALOG_001".
func slugToCode(slug string) string {
	return strings.ToUpper(strings.ReplaceAll(slug, "-", "_"))
}
