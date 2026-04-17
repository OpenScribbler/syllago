package capmon

// Canonical rules recognition schema and helpers.
//
// Rules data is overwhelmingly extracted from documentation markdown — provider
// docs describe how rule files load, where they live, and which other providers'
// rule files are recognized. There is no single typed source struct (the way
// Skill.* exists in the Agent Skills standard), so rules recognition is built on
// landmark/heading matching rather than typed field extraction.
//
// Dot-path emission convention (mirrors skills, hooks, …):
//
//   rules.supported                                        = "true"
//   rules.capabilities.<canonical_key>.supported           = "true"
//   rules.capabilities.<canonical_key>.mechanism           = "<human-readable>"
//   rules.capabilities.<canonical_key>.confidence          = "inferred" | "confirmed"
//
// The canonical keys are listed in CanonicalRulesKeys and defined in
// docs/spec/canonical-keys.yaml under content_types.rules. Recognizers MUST
// only emit dot-paths whose <canonical_key> appears in CanonicalRulesKeys —
// this is enforced at construction time by RulesLandmarkOptions, which panics
// on unknown keys.
//
// Object-typed canonical keys (activation_mode, cross_provider_recognition)
// MAY emit nested sub-segments (e.g.
// "rules.capabilities.cross_provider_recognition.agents_md.supported"). The
// canonical-key check applies to the first segment only — nested segments are
// recognizer-defined and reviewed in seeder specs.

import (
	"fmt"
)

// RulesContentType is the dot-path root for every rules capability emission.
const RulesContentType = "rules"

// CanonicalRulesKeys lists every recognized canonical key for the rules content
// type, in the same order as docs/spec/canonical-keys.yaml. Recognizers MUST
// only emit capability dot-paths whose first canonical-key segment appears in
// this list. Validation runs at RulesLandmarkOptions construction time.
//
// To add a key here, first add it to canonical-keys.yaml so the FormatDoc
// validator and seeder-spec audit stay in sync. The two lists drifting silently
// is the failure mode this slice protects against.
var CanonicalRulesKeys = []string{
	"activation_mode",
	"file_imports",
	"cross_provider_recognition",
	"auto_memory",
	"hierarchical_loading",
}

// canonicalRulesKeySet is the lookup form of CanonicalRulesKeys, built once at
// package init so IsCanonicalRulesKey is O(1) without allocating per call.
var canonicalRulesKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CanonicalRulesKeys))
	for _, k := range CanonicalRulesKeys {
		set[k] = struct{}{}
	}
	return set
}()

// IsCanonicalRulesKey reports whether key is one of the canonical rules keys
// declared in CanonicalRulesKeys. Used by RulesLandmarkOptions to fail fast on
// typos and by tests to guard the canonical-keys.yaml ↔ Go drift surface.
func IsCanonicalRulesKey(key string) bool {
	_, ok := canonicalRulesKeySet[key]
	return ok
}

// RulesLandmarkOptions builds a LandmarkOptions targeting the rules content
// type from the supplied patterns. It validates every pattern's Capability
// against CanonicalRulesKeys (empty Capability is allowed — that pattern only
// contributes to the top-level rules.supported emission) and panics on any
// unknown key. The panic surfaces typos at recognizer registration time, not
// silently at extraction time.
//
// For object-typed canonical keys (activation_mode,
// cross_provider_recognition), Capability MAY use a dot-separated form like
// "activation_mode.always_on" or "cross_provider_recognition.agents_md".
// Validation only checks the first segment.
func RulesLandmarkOptions(patterns ...LandmarkPattern) LandmarkOptions {
	for _, p := range patterns {
		if p.Capability == "" {
			continue
		}
		head := firstSegment(p.Capability)
		if !IsCanonicalRulesKey(head) {
			panic(fmt.Sprintf("capmon: RulesLandmarkOptions pattern uses unknown canonical rules key %q (capability=%q); see CanonicalRulesKeys", head, p.Capability))
		}
	}
	return LandmarkOptions{
		ContentType: RulesContentType,
		Patterns:    patterns,
	}
}

// RulesLandmarkPattern is the convenience constructor for the common
// single-anchor case-insensitive substring match. Every documentation-source
// rules recognizer wires up several of these — inlining the StringMatcher
// literal at every call site obscures intent.
//
// canonicalKey is the canonical rules key (validated by RulesLandmarkOptions
// when the returned pattern is registered). anchor is the substring expected
// in some landmark heading. mechanism is the human-readable explanation
// written to the .mechanism dot-path. required is the optional anchor guard
// shared across patterns in the same recognizer (typically the provider's
// "Rules" section heading) — pass nil when no guard is needed.
func RulesLandmarkPattern(canonicalKey, anchor, mechanism string, required []StringMatcher) LandmarkPattern {
	return LandmarkPattern{
		Capability: canonicalKey,
		Required:   required,
		Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
		Mechanism:  mechanism,
	}
}

// firstSegment returns the substring before the first '.' in s, or s if no dot
// is present. Used to extract the canonical-key head of a possibly nested
// Capability path so validation handles "activation_mode.always_on" the same
// as "activation_mode".
func firstSegment(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return s[:i]
		}
	}
	return s
}
