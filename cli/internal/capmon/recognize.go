package capmon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// GoStructOptions configures recognizeGoStruct for a specific content type.
// Each content type (skills, rules, hooks, …) has its own struct prefix in the
// extracted cache (e.g., "Skill." for skills, "Rule." for rules) and its own
// YAML-key-to-canonical-key mapping.
type GoStructOptions struct {
	// ContentType is the top-level dot-path prefix: "skills", "rules", etc.
	ContentType string
	// StructPrefix matches extracted field keys (e.g., "Skill." matches "Skill.Name").
	StructPrefix string
	// KeyMapper converts a YAML key name to a canonical capability key.
	// Unknown keys should be returned as-is.
	KeyMapper func(yamlKey string) string
	// MechanismPrefix is prepended to the YAML key in the mechanism string.
	// Defaults to "yaml frontmatter key: " when empty.
	MechanismPrefix string
}

// recognizeGoStruct recognizes GoStruct-pattern extracted fields for any content type.
// Fields whose keys start with opts.StructPrefix are mapped through opts.KeyMapper
// to produce canonical capability dot-paths rooted at opts.ContentType.
//
// This utility is called by individual provider recognizers — it is NOT called from
// RecognizeContentTypeDotPaths directly.
func recognizeGoStruct(fields map[string]FieldValue, opts GoStructOptions) map[string]string {
	result := make(map[string]string)
	mechPrefix := opts.MechanismPrefix
	if mechPrefix == "" {
		mechPrefix = "yaml frontmatter key: "
	}
	for k, fv := range fields {
		if !strings.HasPrefix(k, opts.StructPrefix) {
			continue
		}
		yamlKey := fv.Value // e.g., "name", "description", "license"
		if yamlKey == "" || yamlKey == "-" {
			continue
		}
		capKey := opts.KeyMapper(yamlKey)
		capPath := opts.ContentType + ".capabilities." + capKey
		result[capPath+".supported"] = "true"
		result[capPath+".mechanism"] = mechPrefix + yamlKey
		result[capPath+".confidence"] = "confirmed"
		result[opts.ContentType+".supported"] = "true"
	}
	return result
}

// SkillsGoStructOptions returns the preset GoStructOptions for providers that
// implement the Agent Skills open standard (crush, roo-code, and any provider
// whose skills format mirrors the standard's Skill struct).
func SkillsGoStructOptions() GoStructOptions {
	return GoStructOptions{
		ContentType:  "skills",
		StructPrefix: "Skill.",
		KeyMapper:    skillsKeyMapper,
	}
}

// skillsKeyMapper maps a skills YAML frontmatter key name to the canonical capability key.
// Unknown keys are passed through as-is (they may already be canonical).
func skillsKeyMapper(yamlKey string) string {
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

// capabilityDotPaths returns the three dot-path entries for a single canonical capability
// under the given content type. The supported field is always "true" — a recognizer only
// emits a key if it is supported. Used by provider recognizers to avoid boilerplate.
func capabilityDotPaths(contentType, canonicalKey, mechanism, confidence string) map[string]string {
	prefix := contentType + ".capabilities." + canonicalKey
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
