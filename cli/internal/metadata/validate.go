package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationError describes a single validation issue.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) String() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// Validate checks that a content item directory meets the minimum requirements
// for its content type. Returns a list of issues found (empty = valid).
func Validate(itemDir string, contentType string, repoRoot string) []ValidationError {
	var errs []ValidationError

	// All types: .syllago.yaml must exist with required fields
	meta, err := Load(itemDir)
	if err != nil {
		errs = append(errs, ValidationError{FileName, fmt.Sprintf("Cannot read: %s", err)})
	} else if meta == nil {
		errs = append(errs, ValidationError{FileName, fmt.Sprintf("Missing. Every item needs a %s file.", FileName)})
	} else {
		if meta.ID == "" {
			errs = append(errs, ValidationError{FileName, "Missing required field: id"})
		}
		if meta.Name == "" {
			errs = append(errs, ValidationError{FileName, "Missing required field: name"})
		}
		if meta.Type == "" {
			errs = append(errs, ValidationError{FileName, "Missing required field: type"})
		}
	}

	// Warn (not error) if README.md is missing — allows gradual adoption
	readmePath := filepath.Join(itemDir, "README.md")
	if _, readmeErr := os.Stat(readmePath); readmeErr != nil {
		errs = append(errs, ValidationError{"README.md", "Missing. Consider adding a README.md for content transparency."})
	}

	// Type-specific validation
	switch contentType {
	case "skills":
		errs = append(errs, validateFrontmatterFile(itemDir, "SKILL.md", []string{"name", "description"})...)
	case "agents":
		errs = append(errs, validateFrontmatterFile(itemDir, "AGENT.md", []string{"name", "description"})...)
	case "mcp":
		errs = append(errs, validateJSONFile(itemDir, "config.json")...)
	}

	return errs
}

// validateFrontmatterFile checks that a file exists and has the required frontmatter fields.
func validateFrontmatterFile(itemDir, filename string, requiredFields []string) []ValidationError {
	var errs []ValidationError
	filePath := filepath.Join(itemDir, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		errs = append(errs, ValidationError{filename, fmt.Sprintf("Missing. See templates/%s for the expected format.", filename)})
		return errs
	}

	// Parse frontmatter
	fm := parseFrontmatterFields(data)
	for _, field := range requiredFields {
		if fm[field] == "" {
			errs = append(errs, ValidationError{filename, fmt.Sprintf("Missing `%s` in frontmatter. See templates/ for the expected format.", field)})
		}
	}

	return errs
}

// validateJSONFile checks that a JSON file exists and is valid.
func validateJSONFile(itemDir, filename string) []ValidationError {
	var errs []ValidationError
	filePath := filepath.Join(itemDir, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		errs = append(errs, ValidationError{filename, "Missing. MCP servers need a config.json file."})
		return errs
	}
	if !json.Valid(data) {
		errs = append(errs, ValidationError{filename, "Invalid JSON."})
	}
	return errs
}

// validateFileExists checks that a file exists.
func validateFileExists(itemDir, filename string) []ValidationError {
	filePath := filepath.Join(itemDir, filename)
	if _, err := os.Stat(filePath); err != nil {
		return []ValidationError{{filename, fmt.Sprintf("Missing. Apps need an %s file.", filename)}}
	}
	return nil
}

// parseFrontmatterFields extracts frontmatter as a simple string map.
func parseFrontmatterFields(data []byte) map[string]string {
	content := string(data)
	content = strings.ReplaceAll(content, "\r\n", "\n")

	if !strings.HasPrefix(content, "---\n") {
		return nil
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return nil
	}
	fmBlock := content[4 : 4+end]

	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmBlock), &raw); err != nil {
		return nil
	}

	result := make(map[string]string)
	for k, v := range raw {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}
