// Recognizer conformance contract.
//
// Every recognizer registered via RegisterRecognizer MUST satisfy these five
// requirements. They are stated using RFC-2119 keywords (MUST / MUST NOT /
// SHOULD) so callers and tooling can reason about behavior precisely.
//
//  1. INPUT CONTRACT — Recognizers MUST treat the supplied RecognitionContext
//     as read-only. They MUST NOT mutate the Fields map, the Landmarks slice,
//     or any other field on the context. They MUST NOT retain references to
//     ctx.Fields, ctx.Landmarks, or any string within them across calls.
//     A recognizer that needs to keep data around for diagnostics MUST copy it.
//
//  2. OUTPUT CONTRACT — Every key in RecognitionResult.Capabilities MUST match
//     the regular expression  ^[a-z_]+(\.[a-z_]+)*$  . That is: lowercase
//     ASCII letters and underscores, segments joined by literal dots, no
//     leading/trailing dot, no empty segment. Tested by
//     TestRecognitionConformance_KeyRegex.
//
//  3. PURITY — Recognizers MUST NOT perform I/O (no file reads, no network,
//     no os.Stat). They MUST NOT read package-level mutable globals. They
//     MUST NOT call time.Now, math/rand, crypto/rand, or any other source
//     of nondeterminism. The only allowed inputs are the fields of the
//     supplied RecognitionContext.
//
//  4. DETERMINISM — For two RecognitionContext values that are deeply equal
//     (reflect.DeepEqual), recognizers MUST produce two RecognitionResult
//     values that are deeply equal. Map iteration order MUST NOT leak into
//     the output (build sorted keys when ordering matters).
//
//  5. STABILITY — Callers SHOULD construct RecognitionContext and read
//     RecognitionResult by named field. Code outside the capmon package MUST
//     NOT use struct literals to construct or destructure these types — the
//     capmon package reserves the right to add fields without bumping a
//     major version, and named-field access keeps such additions backward
//     compatible.
//
// Violations of (1) and (3) are silent corruption. Violations of (2) and (4)
// are caught by tests. Violation of (5) breaks at compile time the next time
// capmon adds a field, which is the intended trip-wire.
package capmon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// recognizerRegistry maps provider slugs to their recognizer entries
// (function + declared kind). Entries are registered via init() in per-provider
// recognize_<slug>.go files.
var recognizerRegistry = map[string]recognizerEntry{}

// RegisterRecognizer adds a provider recognizer to the registry along with its
// declared RecognizerKind. Called from init() in per-provider recognize_*.go files.
//
// Panics if a recognizer is already registered for the given provider — this is
// a programmer error caught at startup, not a recoverable runtime condition.
func RegisterRecognizer(provider string, kind RecognizerKind, fn func(RecognitionContext) RecognitionResult) {
	if _, exists := recognizerRegistry[provider]; exists {
		panic(fmt.Sprintf("capmon: recognizer for provider %q already registered", provider))
	}
	recognizerRegistry[provider] = recognizerEntry{fn: fn, kind: kind}
}

// IsRecognizerRegistered reports whether a recognizer is registered for the given provider slug.
// Used in tests to assert full provider coverage.
func IsRecognizerRegistered(provider string) bool {
	_, ok := recognizerRegistry[provider]
	return ok
}

// RecognizerKindFor returns the declared RecognizerKind for the given provider,
// or RecognizerKindUnknown when no recognizer is registered.
func RecognizerKindFor(provider string) RecognizerKind {
	entry, ok := recognizerRegistry[provider]
	if !ok {
		return RecognizerKindUnknown
	}
	return entry.kind
}

// RecognizeContentTypeDotPaths dispatches to the registered recognizer for the provider
// and returns just the capability dot-paths. Backwards-compatible facade over the
// structured RecognizeWithContext — callers that need Status / MissingAnchors should
// use RecognizeWithContext directly.
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
	ctx := RecognitionContext{
		Fields:   fields,
		Provider: provider,
	}
	res := RecognizeWithContext(provider, ctx)
	if res.Capabilities == nil {
		return make(map[string]string)
	}
	return res.Capabilities
}

// RecognizeWithContext dispatches to the registered recognizer for the provider
// using the structured RecognitionContext / RecognitionResult shape. This is the
// preferred entry point for new callers that need to observe the Status and
// MissingAnchors fields (added in PR4 for landmark-based recognition).
//
// If no recognizer is registered for the provider, logs a warning and returns
// an empty result with Status == "not_evaluated".
func RecognizeWithContext(provider string, ctx RecognitionContext) RecognitionResult {
	entry, ok := recognizerRegistry[provider]
	if !ok {
		fmt.Fprintf(os.Stderr, "capmon: warning: no recognizer registered for provider %q\n", provider)
		return RecognitionResult{Capabilities: make(map[string]string), Status: StatusNotEvaluated}
	}
	if ctx.Provider == "" {
		ctx.Provider = provider
	}
	res := entry.fn(ctx)
	if res.Capabilities == nil {
		res.Capabilities = make(map[string]string)
	}
	if res.Status == "" {
		if len(res.Capabilities) > 0 {
			res.Status = StatusRecognized
		} else {
			res.Status = StatusNotEvaluated
		}
	}
	return res
}

// GoStructOptions configures recognizeGoStruct for a specific content type.
// Each content type (skills, rules, hooks, …) has its own struct prefix in the
// extracted cache (e.g., "Skill." for skills, "Rule." for rules) and its own
// YAML-key-to-canonical-key mapping.
//
// Prefix selection: exactly one of StructPrefixes or PrefixMatcher MUST be set.
// Validation runs at recognizer call time; misconfiguration panics with a
// descriptive message (programmer error, not a recoverable runtime condition).
//   - StructPrefixes is the explicit allow-list — preferred for the 95% case
//     including multi-struct sources like codex (PR3) where a fixed set of
//     struct names is known and a few related names must be excluded.
//   - PrefixMatcher is the escape hatch for arbitrary matching logic
//     (regex, runtime computation, etc.). Use only when the allow-list cannot
//     express the requirement.
type GoStructOptions struct {
	// ContentType is the top-level dot-path prefix: "skills", "rules", etc.
	ContentType string
	// StructPrefixes is the explicit allow-list of field-key prefixes to recognize.
	// Each entry behaves like the old single-prefix StructPrefix — the OR of all
	// entries is matched. Mutually exclusive with PrefixMatcher.
	StructPrefixes []string
	// PrefixMatcher is the escape hatch: a function that returns true for keys
	// the recognizer should process. Mutually exclusive with StructPrefixes.
	PrefixMatcher func(key string) bool
	// KeyMapper converts a YAML key name to a canonical capability key.
	// Unknown keys should be returned as-is.
	KeyMapper func(yamlKey string) string
	// MechanismPrefix is prepended to the YAML key in the mechanism string.
	// Defaults to "yaml frontmatter key: " when empty.
	MechanismPrefix string
}

// validate enforces the StructPrefixes / PrefixMatcher mutual-exclusion invariant.
// Called from recognizeGoStruct on every invocation to catch programmer errors
// at the earliest deterministic moment (test-time when the recognizer first runs).
func (o GoStructOptions) validate() {
	hasList := len(o.StructPrefixes) > 0
	hasMatcher := o.PrefixMatcher != nil
	switch {
	case hasList && hasMatcher:
		panic(fmt.Sprintf("capmon: GoStructOptions for content_type=%q has both StructPrefixes and PrefixMatcher set; exactly one is required", o.ContentType))
	case !hasList && !hasMatcher:
		panic(fmt.Sprintf("capmon: GoStructOptions for content_type=%q has neither StructPrefixes nor PrefixMatcher set; exactly one is required", o.ContentType))
	}
}

// matches reports whether a field key satisfies the prefix-selection criteria.
// Uses StructPrefixes (any-of) when set, otherwise delegates to PrefixMatcher.
// Caller MUST have already invoked validate() to ensure exactly one is configured.
func (o GoStructOptions) matches(key string) bool {
	if len(o.StructPrefixes) > 0 {
		for _, p := range o.StructPrefixes {
			if strings.HasPrefix(key, p) {
				return true
			}
		}
		return false
	}
	return o.PrefixMatcher(key)
}

// recognizeGoStruct recognizes GoStruct-pattern extracted fields for any content type.
// Fields whose keys are accepted by opts (via StructPrefixes or PrefixMatcher) are
// mapped through opts.KeyMapper to produce canonical capability dot-paths rooted at
// opts.ContentType.
//
// This utility is called by individual provider recognizers — it is NOT called from
// RecognizeContentTypeDotPaths directly. Panics on misconfigured opts.
func recognizeGoStruct(fields map[string]FieldValue, opts GoStructOptions) map[string]string {
	opts.validate()
	result := make(map[string]string)
	mechPrefix := opts.MechanismPrefix
	if mechPrefix == "" {
		mechPrefix = "yaml frontmatter key: "
	}
	for k, fv := range fields {
		if !opts.matches(k) {
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
		ContentType:    "skills",
		StructPrefixes: []string{"Skill."},
		KeyMapper:      skillsKeyMapper,
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
	allLandmarks := make([]string, 0)
	format := ""
	partial := false
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
		allLandmarks = append(allLandmarks, src.Landmarks...)
		if format == "" {
			format = src.Format
		}
		if src.Partial {
			partial = true
		}
	}
	ctx := RecognitionContext{
		Fields:    allFields,
		Landmarks: allLandmarks,
		Format:    format,
		Provider:  provider,
		Partial:   partial,
	}
	res := RecognizeWithContext(provider, ctx)
	if res.Capabilities == nil {
		return make(map[string]string), nil
	}
	return res.Capabilities, nil
}

func mergeInto(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}
