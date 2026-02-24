package parse

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
)

// DiscoveredFile represents a single file found during import discovery.
type DiscoveredFile struct {
	Path        string              `json:"path"`
	ContentType catalog.ContentType `json:"contentType"`
	Provider    string              `json:"provider"`
}

// DiscoveryReport summarizes what was found for a provider.
type DiscoveryReport struct {
	Provider      string                           `json:"provider"`
	Files         []DiscoveredFile                 `json:"files"`
	Counts        map[catalog.ContentType]int      `json:"counts"`
	Unclassified  []string                         `json:"unclassified,omitempty"`
	Unsupported   []catalog.ContentType            `json:"unsupported,omitempty"`
	SearchedPaths map[catalog.ContentType][]string `json:"searchedPaths,omitempty"`
}

// Discover finds all content files for a provider in a project directory.
func Discover(prov provider.Provider, projectRoot string) DiscoveryReport {
	report := DiscoveryReport{
		Provider:      prov.Slug,
		Counts:        make(map[catalog.ContentType]int),
		SearchedPaths: make(map[catalog.ContentType][]string),
	}

	for _, ct := range catalog.AllContentTypes() {
		// Track unsupported types so callers can explain why nothing was found.
		if prov.SupportsType != nil && !prov.SupportsType(ct) {
			report.Unsupported = append(report.Unsupported, ct)
			continue
		}

		if prov.DiscoveryPaths == nil {
			continue
		}
		paths := prov.DiscoveryPaths(projectRoot, ct)
		if len(paths) > 0 {
			report.SearchedPaths[ct] = paths
		}
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
		full := filepath.Join(path, e.Name())
		if e.IsDir() {
			// Recurse one level: collect files inside this subdirectory.
			sub, err := os.ReadDir(full)
			if err != nil {
				continue
			}
			for _, se := range sub {
				if !se.IsDir() {
					files = append(files, filepath.Join(full, se.Name()))
				}
			}
			continue
		}
		files = append(files, full)
	}
	return files
}
