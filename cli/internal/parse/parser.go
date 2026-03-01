package parse

import (
	"encoding/json"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/model"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// Parser reads a provider-specific file and returns canonical sections.
type Parser interface {
	ParseFile(file DiscoveredFile) ([]model.Section, error)
}

// ImportResult holds the complete parsed output from an import operation.
type ImportResult struct {
	Provider string          `json:"provider"`
	Sections []model.Section `json:"sections"`
	Report   DiscoveryReport `json:"report"`
}

// Import runs full discovery + parsing for a provider.
func Import(prov provider.Provider, parser Parser, projectRoot string) (*ImportResult, error) {
	report := Discover(prov, projectRoot)

	result := &ImportResult{
		Provider: prov.Slug,
		Report:   report,
	}

	for _, file := range report.Files {
		sections, err := parser.ParseFile(file)
		if err != nil {
			report.Unclassified = append(report.Unclassified, file.Path)
			continue
		}
		result.Sections = append(result.Sections, sections...)
	}

	return result, nil
}

// readFileContent is a helper to read file content.
func readFileContent(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// parseJSONFile reads a JSON file into the given target.
func parseJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
