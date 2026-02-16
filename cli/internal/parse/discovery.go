package parse

import (
	"os"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
)

// DiscoveredFile represents a single file found during import discovery.
type DiscoveredFile struct {
	Path        string              `json:"path"`
	ContentType catalog.ContentType `json:"contentType"`
	Provider    string              `json:"provider"`
}

// DiscoveryReport summarizes what was found for a provider.
type DiscoveryReport struct {
	Provider     string                      `json:"provider"`
	Files        []DiscoveredFile            `json:"files"`
	Counts       map[catalog.ContentType]int `json:"counts"`
	Unclassified []string                    `json:"unclassified,omitempty"`
}

// Discover finds all content files for a provider in a project directory.
func Discover(prov provider.Provider, projectRoot string) DiscoveryReport {
	report := DiscoveryReport{
		Provider: prov.Slug,
		Counts:   make(map[catalog.ContentType]int),
	}

	for _, ct := range catalog.AllContentTypes() {
		if prov.DiscoveryPaths == nil {
			continue
		}
		paths := prov.DiscoveryPaths(projectRoot, ct)
		for _, p := range paths {
			files := findFiles(p)
			for _, f := range files {
				report.Files = append(report.Files, DiscoveredFile{
					Path:        f,
					ContentType: ct,
					Provider:    prov.Slug,
				})
				report.Counts[ct]++
			}
		}
	}

	return report
}

// findFiles returns file paths at a discovery location.
func findFiles(path string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	if !info.IsDir() {
		return []string{path}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, filepath.Join(path, e.Name()))
	}
	return files
}
