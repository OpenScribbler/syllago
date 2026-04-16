package capmon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// recognizerRegistry maps provider slugs to their recognizer functions.
// Entries are registered via init() in per-provider recognize_<slug>.go files.
var recognizerRegistry = map[string]func(map[string]FieldValue) map[string]string{}

// RegisterRecognizer adds a provider recognizer to the registry.
// Called from init() in per-provider recognize_*.go files.
func RegisterRecognizer(provider string, fn func(map[string]FieldValue) map[string]string) {
	recognizerRegistry[provider] = fn
}

// IsRecognizerRegistered reports whether a recognizer is registered for the given provider slug.
// Used in tests to assert full provider coverage.
func IsRecognizerRegistered(provider string) bool {
	_, ok := recognizerRegistry[provider]
	return ok
}

// RecognizeContentTypeDotPaths dispatches to the registered recognizer for the provider.
// Returns a dot-path → value map suitable for passing to SeedProviderCapabilities.
//
// Dot-path format: "<content_type>.<section>.<key>.<field>" → value
// Examples:
//   - "skills.supported" → "true"
//   - "skills.capabilities.display_name.mechanism" → "yaml frontmatter key: name"
//   - "skills.capabilities.display_name.confidence" → "confirmed"
//
// If no recognizer is registered for the provider, logs a warning and returns an empty map.
// Recognition is pattern-based and deterministic — no LLM calls.
func RecognizeContentTypeDotPaths(provider string, fields map[string]FieldValue) map[string]string {
	fn, ok := recognizerRegistry[provider]
	if !ok {
		fmt.Fprintf(os.Stderr, "capmon: warning: no recognizer registered for provider %q\n", provider)
		return make(map[string]string)
	}
	result := make(map[string]string)
	mergeInto(result, fn(fields))
	return result
}

// recognizeSkillsGoStruct recognizes the Agent Skills standard struct pattern.
// Keys of the form "Skill.<FieldName>" with yaml key values map to skills frontmatter capabilities.
// This utility is called by individual recognizer functions (e.g., recognizeCrush)
// that implement the Agent Skills open standard.
//
// IMPORTANT: This function is NOT called from RecognizeContentTypeDotPaths directly.
// Individual provider recognizers call it when appropriate.
func recognizeSkillsGoStruct(fields map[string]FieldValue) map[string]string {
	result := make(map[string]string)
	for k, fv := range fields {
		if len(k) < 7 || k[:6] != "Skill." {
			continue
		}
		yamlKey := fv.Value // e.g., "name", "description", "license"
		if yamlKey == "" || yamlKey == "-" {
			continue
		}
		capKey := canonicalKeyFromYAMLKey(yamlKey)
		result["skills.capabilities."+capKey+".supported"] = "true"
		result["skills.capabilities."+capKey+".mechanism"] = "yaml frontmatter key: " + yamlKey
		result["skills.capabilities."+capKey+".confidence"] = "confirmed"
		result["skills.supported"] = "true"
	}
	return result
}

// canonicalKeyFromYAMLKey maps a YAML frontmatter key name to the canonical capability key.
// Unknown keys are passed through as-is (they may already be canonical).
func canonicalKeyFromYAMLKey(yamlKey string) string {
	switch yamlKey {
	case "name":
		return "display_name"
	case "description":
		return "description"
	case "license":
		return "license"
	case "compatibility":
		return "compatibility"
	case "metadata":
		return "metadata_map"
	case "disable-model-invocation", "disable_model_invocation":
		return "disable_model_invocation"
	case "user-invocable", "user_invocable":
		return "user_invocable"
	case "version":
		return "version"
	default:
		return yamlKey
	}
}

// capabilityDotPaths returns the three dot-path entries for a single canonical capability.
// The supported field is always set to "true" — a recognizer only emits a key if it is supported.
// Used by individual recognizer functions to avoid boilerplate.
func capabilityDotPaths(canonicalKey, mechanism, confidence string) map[string]string {
	prefix := "skills.capabilities." + canonicalKey
	return map[string]string{
		prefix + ".supported":  "true",
		prefix + ".mechanism":  mechanism,
		prefix + ".confidence": confidence,
	}
}

// LoadAndRecognizeCache reads all extracted.json files for the given provider
// from the cache root, runs the registered recognizer, and returns a merged
// dot-path → value map. Source directories that are missing or corrupt are silently skipped.
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
	return RecognizeContentTypeDotPaths(provider, allFields), nil
}

func mergeInto(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}
