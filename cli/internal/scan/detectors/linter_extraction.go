package detectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// LinterExtraction extracts formatting and style rules from config files like
// .prettierrc and .editorconfig. Unlike the other cross-language detectors,
// this returns CatConventions — it's extracting facts about the project's
// style preferences, not flagging surprises.
//
// Why extract these? LLMs generating code need to know indentation style,
// quote preference, semicolon usage, etc. Embedding these rules in the context
// document means generated code matches the project's formatter config without
// a second pass through the formatter.
type LinterExtraction struct{}

func (d LinterExtraction) Name() string { return "linter-extraction" }

func (d LinterExtraction) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	if s := extractPrettierConfig(root); s != nil {
		sections = append(sections, *s)
	}
	if s := extractEditorConfig(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

// extractPrettierConfig reads .prettierrc (JSON format) and extracts key
// formatting rules: indentation, quotes, semicolons, line length.
func extractPrettierConfig(root string) *model.TextSection {
	data, err := os.ReadFile(filepath.Join(root, ".prettierrc"))
	if err != nil {
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil
	}

	var rules []string

	if v, ok := config["tabWidth"]; ok {
		rules = append(rules, fmt.Sprintf("indent: %v spaces", v))
	}
	if v, ok := config["useTabs"]; ok {
		if b, ok := v.(bool); ok && b {
			rules = append(rules, "indent: tabs")
		}
	}
	if v, ok := config["singleQuote"]; ok {
		if b, ok := v.(bool); ok && b {
			rules = append(rules, "quotes: single")
		} else {
			rules = append(rules, "quotes: double")
		}
	}
	if v, ok := config["semi"]; ok {
		if b, ok := v.(bool); ok && !b {
			rules = append(rules, "semicolons: no")
		} else {
			rules = append(rules, "semicolons: yes")
		}
	}
	if v, ok := config["printWidth"]; ok {
		rules = append(rules, fmt.Sprintf("line length: %v", v))
	}
	if v, ok := config["trailingComma"]; ok {
		rules = append(rules, fmt.Sprintf("trailing commas: %v", v))
	}

	if len(rules) == 0 {
		return nil
	}

	return &model.TextSection{
		Category: model.CatConventions,
		Origin:   model.OriginAuto,
		Title:    "Prettier Formatting Rules",
		Body:     strings.Join(rules, "\n"),
		Source:   "linter-extraction",
	}
}

// extractEditorConfig reads .editorconfig and extracts key style rules.
// .editorconfig uses an INI-like format with [section] headers and key=value pairs.
// We extract rules from [*] (applies to all files) and any other sections.
func extractEditorConfig(root string) *model.TextSection {
	data, err := os.ReadFile(filepath.Join(root, ".editorconfig"))
	if err != nil {
		return nil
	}

	var rules []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "indent_style":
			rules = append(rules, fmt.Sprintf("indent style: %s", val))
		case "indent_size":
			rules = append(rules, fmt.Sprintf("indent size: %s", val))
		case "end_of_line":
			rules = append(rules, fmt.Sprintf("line endings: %s", val))
		case "max_line_length":
			rules = append(rules, fmt.Sprintf("line length: %s", val))
		case "trim_trailing_whitespace":
			if val == "true" {
				rules = append(rules, "trim trailing whitespace: yes")
			}
		case "insert_final_newline":
			if val == "true" {
				rules = append(rules, "final newline: yes")
			}
		}
	}

	if len(rules) == 0 {
		return nil
	}

	return &model.TextSection{
		Category: model.CatConventions,
		Origin:   model.OriginAuto,
		Title:    "EditorConfig Rules",
		Body:     strings.Join(rules, "\n"),
		Source:   "linter-extraction",
	}
}
