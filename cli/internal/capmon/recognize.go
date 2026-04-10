package capmon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// RecognizeContentTypeDotPaths inspects raw extracted fields from a single source
// and returns a dot-path → value map suitable for passing to SeedProviderCapabilities.
//
// Dot-path format: "<content_type>.<section>.<key>.<field>" → value
// Examples:
//   - "skills.supported" → "true"
//   - "skills.capabilities.frontmatter_name.mechanism" → "yaml key: name"
//   - "hooks.events.before_tool_execute.native_name" → "PreToolUse"
//
// Recognition is pattern-based and deterministic — no LLM calls.
func RecognizeContentTypeDotPaths(fields map[string]FieldValue) map[string]string {
	result := make(map[string]string)
	mergeInto(result, recognizeSkillsGoStruct(fields))
	return result
}

// recognizeSkillsGoStruct recognizes the Agent Skills standard struct pattern.
// Keys of the form "Skill.<FieldName>" with yaml key values map to skills frontmatter capabilities.
// This covers both crush and roo-code (both implement the same Agent Skills open standard).
func recognizeSkillsGoStruct(fields map[string]FieldValue) map[string]string {
	result := make(map[string]string)
	for k, fv := range fields {
		if !strings.HasPrefix(k, "Skill.") {
			continue
		}
		yamlKey := fv.Value // e.g., "name", "description", "license"
		if yamlKey == "" || yamlKey == "-" {
			continue
		}
		capKey := "frontmatter_" + yamlKey
		result["skills.capabilities."+capKey+".supported"] = "true"
		result["skills.capabilities."+capKey+".mechanism"] = "yaml key: " + yamlKey
		result["skills.supported"] = "true"
	}
	return result
}

// LoadAndRecognizeCache reads all extracted.json files for the given provider
// from the cache root, runs all recognizers, and returns a merged dot-path → value map.
// Source directories that are missing or corrupt are silently skipped.
func LoadAndRecognizeCache(cacheRoot, provider string) (map[string]string, error) {
	providerDir := filepath.Join(cacheRoot, provider)
	entries, err := os.ReadDir(providerDir)
	if err != nil {
		return nil, err
	}
	allFields := make(map[string]FieldValue)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		extPath := filepath.Join(providerDir, e.Name(), "extracted.json")
		data, err := os.ReadFile(extPath)
		if err != nil {
			continue
		}
		var src ExtractedSource
		if err := json.Unmarshal(data, &src); err != nil {
			continue
		}
		for k, fv := range src.Fields {
			allFields[k] = fv
		}
	}
	return RecognizeContentTypeDotPaths(allFields), nil
}

func mergeInto(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}
