package capmon

// Canonical skills recognition schema and helpers.
//
// Skills data comes from two source shapes:
//
//  1. Typed Go-struct sources (the Agent Skills open standard's Skill.* fields,
//     used by crush, roo-code, opencode, codex, …) — already handled by
//     SkillsGoStructOptions in recognize.go and the recognizeGoStruct path.
//
//  2. Documentation markdown / human-readable specs (windsurf, cursor, kiro,
//     factory-droid, …) — handled by landmark/heading matching via this module's
//     SkillsLandmarkOptions / SkillsLandmarkPattern helpers.
//
// Many providers compose both shapes (e.g. gemini-cli reads Skill.* via
// recognizeGoStruct AND adds landmark patterns for canonical keys not encoded
// in the typed struct). The two paths share the same canonical key vocabulary
// declared in CanonicalSkillsKeys; this file exists so landmark recognizers can
// participate in the same fail-fast canonical-key validation that
// RulesLandmarkOptions provides for rules.
//
// Dot-path emission convention (mirrors rules, hooks, …):
//
//   skills.supported                                       = "true"
//   skills.capabilities.<canonical_key>.supported          = "true"
//   skills.capabilities.<canonical_key>.mechanism          = "<human-readable>"
//   skills.capabilities.<canonical_key>.confidence         = "inferred" | "confirmed"
//
// The canonical keys are listed in CanonicalSkillsKeys and defined in
// docs/spec/canonical-keys.yaml under content_types.skills. Recognizers MUST
// only emit dot-paths whose <canonical_key> appears in CanonicalSkillsKeys —
// this is enforced at construction time by SkillsLandmarkOptions, which panics
// on unknown keys.
//
// Object-typed canonical keys (compatibility, metadata_map) MAY emit nested
// sub-segments. The canonical-key check applies to the first segment only.

import (
	"fmt"
)

// SkillsContentType is the dot-path root for every skills capability emission.
const SkillsContentType = "skills"

// CanonicalSkillsKeys lists every recognized canonical key for the skills
// content type, in the same order as docs/spec/canonical-keys.yaml. Recognizers
// MUST only emit capability dot-paths whose first canonical-key segment appears
// in this list. Validation runs at SkillsLandmarkOptions construction time.
//
// To add a key here, first add it to canonical-keys.yaml so the FormatDoc
// validator and seeder-spec audit stay in sync. The two lists drifting silently
// is the failure mode this slice protects against.
var CanonicalSkillsKeys = []string{
	"display_name",
	"description",
	"license",
	"compatibility",
	"metadata_map",
	"disable_model_invocation",
	"user_invocable",
	"version",
	"project_scope",
	"global_scope",
	"shared_scope",
	"canonical_filename",
	"custom_filename",
}

// canonicalSkillsKeySet is the lookup form of CanonicalSkillsKeys, built once
// at package init so IsCanonicalSkillsKey is O(1) without allocating per call.
var canonicalSkillsKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CanonicalSkillsKeys))
	for _, k := range CanonicalSkillsKeys {
		set[k] = struct{}{}
	}
	return set
}()

// IsCanonicalSkillsKey reports whether key is one of the canonical skills keys
// declared in CanonicalSkillsKeys. Used by SkillsLandmarkOptions to fail fast
// on typos and by tests to guard the canonical-keys.yaml ↔ Go drift surface.
func IsCanonicalSkillsKey(key string) bool {
	_, ok := canonicalSkillsKeySet[key]
	return ok
}

// SkillsLandmarkOptions builds a LandmarkOptions targeting the skills content
// type from the supplied patterns. It validates every pattern's Capability
// against CanonicalSkillsKeys (empty Capability is allowed — that pattern only
// contributes to the top-level skills.supported emission) and panics on any
// unknown key. The panic surfaces typos at recognizer registration time, not
// silently at extraction time.
//
// For object-typed canonical keys (compatibility, metadata_map), Capability
// MAY use a dot-separated form like "compatibility.platforms" or
// "metadata_map.custom". Validation only checks the first segment.
func SkillsLandmarkOptions(patterns ...LandmarkPattern) LandmarkOptions {
	for _, p := range patterns {
		if p.Capability == "" {
			continue
		}
		head := firstSegment(p.Capability)
		if !IsCanonicalSkillsKey(head) {
			panic(fmt.Sprintf("capmon: SkillsLandmarkOptions pattern uses unknown canonical skills key %q (capability=%q); see CanonicalSkillsKeys", head, p.Capability))
		}
	}
	return LandmarkOptions{
		ContentType: SkillsContentType,
		Patterns:    patterns,
	}
}

// SkillsLandmarkPattern is the convenience constructor for the common
// single-anchor case-insensitive substring match. Every documentation-source
// skills recognizer wires up several of these — inlining the StringMatcher
// literal at every call site obscures intent.
//
// canonicalKey is the canonical skills key (validated by SkillsLandmarkOptions
// when the returned pattern is registered). anchor is the substring expected
// in some landmark heading. mechanism is the human-readable explanation
// written to the .mechanism dot-path. required is the optional anchor guard
// shared across patterns in the same recognizer (typically the provider's
// "Skills" section heading) — pass nil when no guard is needed.
func SkillsLandmarkPattern(canonicalKey, anchor, mechanism string, required []StringMatcher) LandmarkPattern {
	return LandmarkPattern{
		Capability: canonicalKey,
		Required:   required,
		Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
		Mechanism:  mechanism,
	}
}
