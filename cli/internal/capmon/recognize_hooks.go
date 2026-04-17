package capmon

// Canonical hooks recognition schema and helpers.
//
// Hooks data, like rules, is overwhelmingly extracted from documentation
// markdown — provider docs describe which event names exist, what handler
// shapes are accepted, and how decisions are signaled. There is no single
// typed source struct per provider, so hooks recognition is built on landmark
// matching rather than typed field extraction.
//
// Dot-path emission convention (mirrors skills, rules, …):
//
//   hooks.supported                                        = "true"
//   hooks.capabilities.<canonical_key>.supported           = "true"
//   hooks.capabilities.<canonical_key>.mechanism           = "<human-readable>"
//   hooks.capabilities.<canonical_key>.confidence          = "inferred" | "confirmed"
//
// The canonical keys are listed in CanonicalHooksKeys and defined in
// docs/spec/canonical-keys.yaml under content_types.hooks. Recognizers MUST
// only emit dot-paths whose <canonical_key> appears in CanonicalHooksKeys —
// this is enforced at construction time by HooksLandmarkOptions, which panics
// on unknown keys.
//
// Object-typed canonical keys (handler_types, decision_control, hook_scopes)
// MAY emit nested sub-segments (e.g.
// "hooks.capabilities.decision_control.block.supported"). The canonical-key
// check applies to the first segment only — nested segments are
// recognizer-defined and reviewed in seeder specs.

import (
	"fmt"
)

// HooksContentType is the dot-path root for every hooks capability emission.
const HooksContentType = "hooks"

// CanonicalHooksKeys lists every recognized canonical key for the hooks
// content type, in the same order as docs/spec/canonical-keys.yaml.
// Recognizers MUST only emit capability dot-paths whose first canonical-key
// segment appears in this list. Validation runs at HooksLandmarkOptions
// construction time.
//
// To add a key here, first add it to canonical-keys.yaml so the FormatDoc
// validator and seeder-spec audit stay in sync. The two lists drifting
// silently is the failure mode this slice protects against.
var CanonicalHooksKeys = []string{
	"handler_types",
	"matcher_patterns",
	"decision_control",
	"input_modification",
	"async_execution",
	"hook_scopes",
	"json_io_protocol",
	"context_injection",
	"permission_control",
}

// canonicalHooksKeySet is the lookup form of CanonicalHooksKeys, built once at
// package init so IsCanonicalHooksKey is O(1) without allocating per call.
var canonicalHooksKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CanonicalHooksKeys))
	for _, k := range CanonicalHooksKeys {
		set[k] = struct{}{}
	}
	return set
}()

// IsCanonicalHooksKey reports whether key is one of the canonical hooks keys
// declared in CanonicalHooksKeys. Used by HooksLandmarkOptions to fail fast on
// typos and by tests to guard the canonical-keys.yaml ↔ Go drift surface.
func IsCanonicalHooksKey(key string) bool {
	_, ok := canonicalHooksKeySet[key]
	return ok
}

// HooksLandmarkOptions builds a LandmarkOptions targeting the hooks content
// type from the supplied patterns. It validates every pattern's Capability
// against CanonicalHooksKeys (empty Capability is allowed — that pattern only
// contributes to the top-level hooks.supported emission) and panics on any
// unknown key. The panic surfaces typos at recognizer registration time, not
// silently at extraction time.
//
// For object-typed canonical keys (handler_types, decision_control,
// hook_scopes), Capability MAY use a dot-separated form like
// "decision_control.block" or "hook_scopes.project". Validation only checks
// the first segment.
func HooksLandmarkOptions(patterns ...LandmarkPattern) LandmarkOptions {
	for _, p := range patterns {
		if p.Capability == "" {
			continue
		}
		head := firstSegment(p.Capability)
		if !IsCanonicalHooksKey(head) {
			panic(fmt.Sprintf("capmon: HooksLandmarkOptions pattern uses unknown canonical hooks key %q (capability=%q); see CanonicalHooksKeys", head, p.Capability))
		}
	}
	return LandmarkOptions{
		ContentType: HooksContentType,
		Patterns:    patterns,
	}
}

// HooksLandmarkPattern is the convenience constructor for the common
// single-anchor case-insensitive substring match. Every documentation-source
// hooks recognizer wires up several of these — inlining the StringMatcher
// literal at every call site obscures intent.
//
// canonicalKey is the canonical hooks key (validated by HooksLandmarkOptions
// when the returned pattern is registered). anchor is the substring expected
// in some landmark heading. mechanism is the human-readable explanation
// written to the .mechanism dot-path. required is the optional anchor guard
// shared across patterns in the same recognizer (typically the provider's
// "Hooks" section heading) — pass nil when no guard is needed.
func HooksLandmarkPattern(canonicalKey, anchor, mechanism string, required []StringMatcher) LandmarkPattern {
	return LandmarkPattern{
		Capability: canonicalKey,
		Required:   required,
		Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
		Mechanism:  mechanism,
	}
}
