package capmon

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// canonicalKeysFile is the minimal structure we need from canonical-keys.yaml.
type canonicalKeysFile struct {
	ContentTypes map[string]map[string]interface{} `yaml:"content_types"`
}

// loadCanonicalKeys parses canonical-keys.yaml and returns the set of valid keys
// per content type.
func loadCanonicalKeys(path string) (map[string]map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read canonical keys %s: %w", path, err)
	}
	var ck canonicalKeysFile
	if err := yaml.Unmarshal(data, &ck); err != nil {
		return nil, fmt.Errorf("parse canonical keys %s: %w", path, err)
	}
	result := make(map[string]map[string]bool)
	for contentType, keys := range ck.ContentTypes {
		keySet := make(map[string]bool, len(keys))
		for k := range keys {
			keySet[k] = true
		}
		result[contentType] = keySet
	}
	return result, nil
}

// supportedUnknownRE matches lines where the supported field is set to the
// string "unknown" instead of a boolean. YAML unmarshal rejects this with an
// unhelpful type error; we catch it here before parsing to give a clear fix.
var supportedUnknownRE = regexp.MustCompile(`(?m)^\s+supported:\s+unknown\s*$`)

// preScanFormatDoc checks raw YAML bytes for known-bad patterns that would
// cause confusing parse errors. Returns a descriptive, actionable error.
func preScanFormatDoc(data []byte) error {
	if supportedUnknownRE.Match(data) {
		return fmt.Errorf("✗ supported: unknown — the supported field must be a boolean (true or false)\n" +
			"  For capabilities absent from source material use:\n" +
			"    supported: false\n" +
			"    confidence: unknown")
	}
	return nil
}

// validConfidenceValues is the controlled vocabulary for the confidence field.
var validConfidenceValues = map[string]bool{
	"confirmed": true,
	"inferred":  true,
	"unknown":   true,
}

// ValidateFormatDoc validates a provider's format doc against the canonical keys vocabulary.
// It collects all errors (non-short-circuiting) and returns them as a combined error.
// Output uses ✓/✗ prefixes suitable for human-readable terminal output.
//
// Validation rules:
//  1. Required top-level fields: provider, last_fetched_at, content_types (non-empty)
//  2. Each key in canonical_mappings must exist in canonical-keys.yaml for the content type
//  3. Each provider_extensions entry must have: id, name, description, source_ref
//  4. confidence values must be confirmed | inferred | unknown
//  5. generation_method and notes are NOT validated (informational)
func ValidateFormatDoc(formatsDir, canonicalKeysPath, provider string) error {
	canonicalKeys, err := loadCanonicalKeys(canonicalKeysPath)
	if err != nil {
		return err
	}

	docPath := FormatDocPath(formatsDir, provider)

	// Pre-scan raw bytes before YAML parse. Patterns like "supported: unknown"
	// cause opaque Go unmarshal errors; catching them here gives a clear fix.
	rawBytes, err := os.ReadFile(docPath)
	if err != nil {
		return fmt.Errorf("read format doc %s: %w", docPath, err)
	}
	if err := preScanFormatDoc(rawBytes); err != nil {
		return err
	}

	doc, err := LoadFormatDoc(docPath)
	if err != nil {
		return err
	}

	var errs []string

	// Rule 1: required top-level fields
	if doc.Provider == "" {
		errs = append(errs, "✗ provider: required field is empty")
	}
	if doc.LastFetchedAt == "" {
		errs = append(errs, "✗ last_fetched_at: required field is empty")
	}
	if len(doc.ContentTypes) == 0 {
		errs = append(errs, "✗ content_types: required field is empty")
	}

	// Per-content-type rules
	for ct, ctDoc := range doc.ContentTypes {
		validKeys := canonicalKeys[ct]

		// Rule 2: canonical_mappings keys must be in vocab
		for key, mapping := range ctDoc.CanonicalMappings {
			if validKeys != nil && !validKeys[key] {
				errs = append(errs, fmt.Sprintf("✗ content_types.%s.canonical_mappings.%s: not in canonical-keys.yaml", ct, key))
			}

			// Rule 4: confidence vocabulary
			if mapping.Confidence != "" && !validConfidenceValues[mapping.Confidence] {
				errs = append(errs, fmt.Sprintf("✗ content_types.%s.canonical_mappings.%s.confidence: invalid value %q (must be confirmed|inferred|unknown)", ct, key, mapping.Confidence))
			}
		}

		// Rule 3: provider_extensions required fields
		for _, ext := range ctDoc.ProviderExtensions {
			var extErrs []string
			if ext.ID == "" {
				extErrs = append(extErrs, "id")
			}
			if ext.Name == "" {
				extErrs = append(extErrs, "name")
			}
			if ext.Description == "" {
				extErrs = append(extErrs, "description")
			}
			if ext.SourceRef == "" {
				extErrs = append(extErrs, "source_ref")
			}
			if len(extErrs) > 0 {
				id := ext.ID
				if id == "" {
					id = "(unnamed)"
				}
				errs = append(errs, fmt.Sprintf("✗ content_types.%s.provider_extensions[%s]: missing required fields: %s", ct, id, strings.Join(extErrs, ", ")))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
