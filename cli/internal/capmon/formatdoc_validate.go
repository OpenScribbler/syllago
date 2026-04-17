package capmon

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// allowedValueTypes is the controlled vocabulary for provider_extensions[i].value_type.
// Values outside this list produce a warning (not an error).
var allowedValueTypes = map[string]bool{
	"string":            true,
	"string[]":          true,
	"string | string[]": true,
	"bool":              true,
	"int":               true,
	"object":            true,
	"object[]":          true,
	"path":              true,
}

// allowedExampleLangs is the controlled vocabulary for provider_extensions[i].examples[j].lang.
var allowedExampleLangs = map[string]bool{
	"yaml":       true,
	"json":       true,
	"toml":       true,
	"bash":       true,
	"javascript": true,
	"typescript": true,
	"python":     true,
	"markdown":   true,
	"mdx":        true,
	"ini":        true,
	"dotenv":     true,
}

// allowedSourceSections is the controlled vocabulary for sources[i].section.
// Extension-specific values like "Extension: <field-name>" are component-generated,
// not authored in YAML, so they do not appear here.
var allowedSourceSections = map[string]bool{
	"All":                true,
	"Native Format":      true,
	"Canonical Mappings": true,
	"Extensions":         true,
}

// ValidationWarning is a non-fatal allow-list violation found in a format doc.
type ValidationWarning struct {
	File    string // absolute path to the YAML file
	Field   string // dotted field path, e.g., "content_types.skills.provider_extensions[model].value_type"
	Value   string // the offending value
	Message string // human-readable explanation
}

// DeduplicationKey returns the SHA-256-based key used to deduplicate GitHub issues for this warning.
// Key format: sha256(<file> + "\x00" + <field> + "\x00" + <value>), first 16 hex chars.
func (w ValidationWarning) DeduplicationKey() string {
	h := sha256.Sum256([]byte(w.File + "\x00" + w.Field + "\x00" + w.Value))
	return fmt.Sprintf("%x", h[:8])
}

// sortedKeys returns a sorted slice of the keys of a string→bool map.
// Used for deterministic error messages.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

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

// validConversionValues is the controlled vocabulary for the provider extension
// conversion field. Each value answers "what happens to this feature during
// format conversion?" — see docs/plans/2026-04-16-provider-convention-pages-redesign.md
// for full semantics.
var validConversionValues = map[string]bool{
	"translated":   true, // maps to canonical key, actively converted
	"embedded":     true, // appended to body as conversion-notes block
	"dropped":      true, // removed; portability warning emitted
	"preserved":    true, // syntax survives but target may not interpret
	"not-portable": true, // tied to provider runtime; cannot meaningfully exist elsewhere
}

// ValidateFormatDoc validates a provider's format doc against the canonical keys vocabulary.
// It collects all errors (non-short-circuiting) and returns them as a combined error.
// Output uses ✓/✗ prefixes suitable for human-readable terminal output.
//
// Validation rules:
//  1. Required top-level fields: provider, last_fetched_at, content_types (non-empty)
//  2. Each key in canonical_mappings must exist in canonical-keys.yaml for the content type
//  3. Each provider_extensions entry must have: id, name, summary, source_ref, conversion
//  4. confidence values must be confirmed | inferred | unknown
//  5. conversion values must be translated | embedded | dropped | preserved | not-portable
//  6. generation_method and notes are NOT validated (informational)
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
			if ext.Summary == "" {
				extErrs = append(extErrs, "summary")
			}
			if ext.SourceRef == "" {
				extErrs = append(extErrs, "source_ref")
			}
			if ext.Conversion == "" {
				extErrs = append(extErrs, "conversion")
			}
			if len(extErrs) > 0 {
				id := ext.ID
				if id == "" {
					id = "(unnamed)"
				}
				errs = append(errs, fmt.Sprintf("✗ content_types.%s.provider_extensions[%s]: missing required fields: %s", ct, id, strings.Join(extErrs, ", ")))
			}

			// Rule 5: conversion vocabulary
			if ext.Conversion != "" && !validConversionValues[ext.Conversion] {
				id := ext.ID
				if id == "" {
					id = "(unnamed)"
				}
				errs = append(errs, fmt.Sprintf("✗ content_types.%s.provider_extensions[%s].conversion: invalid value %q (must be %s)", ct, id, ext.Conversion, strings.Join(sortedKeys(validConversionValues), "|")))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// ValidateFormatDocWithWarnings validates a format doc and returns both
// blocking errors and non-blocking allow-list warnings.
// The caller decides what to do with warnings (log, open GH issue, etc.).
func ValidateFormatDocWithWarnings(formatsDir, canonicalKeysPath, provider string) ([]ValidationWarning, error) {
	if err := ValidateFormatDoc(formatsDir, canonicalKeysPath, provider); err != nil {
		return nil, err
	}

	docPath := FormatDocPath(formatsDir, provider)
	doc, err := LoadFormatDoc(docPath)
	if err != nil {
		return nil, err
	}

	var warnings []ValidationWarning

	for ct, ctDoc := range doc.ContentTypes {
		for i, src := range ctDoc.Sources {
			if src.Section != "" && !allowedSourceSections[src.Section] {
				field := fmt.Sprintf("content_types.%s.sources[%d].section", ct, i)
				warnings = append(warnings, ValidationWarning{
					File:    docPath,
					Field:   field,
					Value:   src.Section,
					Message: fmt.Sprintf("section %q not in allow-list %v", src.Section, sortedKeys(allowedSourceSections)),
				})
			}
		}

		for _, ext := range ctDoc.ProviderExtensions {
			if ext.ValueType != "" && !allowedValueTypes[ext.ValueType] {
				field := fmt.Sprintf("content_types.%s.provider_extensions[%s].value_type", ct, ext.ID)
				warnings = append(warnings, ValidationWarning{
					File:    docPath,
					Field:   field,
					Value:   ext.ValueType,
					Message: fmt.Sprintf("value_type %q not in allow-list %v", ext.ValueType, sortedKeys(allowedValueTypes)),
				})
			}
			for j, ex := range ext.Examples {
				if ex.Lang == "" {
					field := fmt.Sprintf("content_types.%s.provider_extensions[%s].examples[%d].lang", ct, ext.ID, j)
					warnings = append(warnings, ValidationWarning{
						File:    docPath,
						Field:   field,
						Value:   "",
						Message: "examples[].lang is required and must be non-empty",
					})
					continue
				}
				if !allowedExampleLangs[ex.Lang] {
					field := fmt.Sprintf("content_types.%s.provider_extensions[%s].examples[%d].lang", ct, ext.ID, j)
					warnings = append(warnings, ValidationWarning{
						File:    docPath,
						Field:   field,
						Value:   ex.Lang,
						Message: fmt.Sprintf("lang %q not in allow-list %v", ex.Lang, sortedKeys(allowedExampleLangs)),
					})
				}
				if ex.Code == "" {
					field := fmt.Sprintf("content_types.%s.provider_extensions[%s].examples[%d].code", ct, ext.ID, j)
					warnings = append(warnings, ValidationWarning{
						File:    docPath,
						Field:   field,
						Value:   "",
						Message: "examples[].code is required and must be non-empty",
					})
				}
			}
		}
	}

	return warnings, nil
}
