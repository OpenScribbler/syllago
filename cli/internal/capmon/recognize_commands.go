package capmon

// Canonical commands recognition schema and helpers.
//
// Commands (also called "slash commands", "custom commands", or "prompts"
// depending on provider) are reusable text templates the user invokes via
// a UI shortcut (typically a "/" prefix). Provider data is sourced from
// documentation pages and config schemas — providers describe argument-
// substitution syntaxes, built-in command catalogs, scope hierarchies,
// and bundled-skill / MCP-prompt re-exports. Recognizers may use either
// landmark matching (doc-source) or typed-source extraction (schema-
// source); this file declares the canonical vocabulary and landmark-
// helper constructors. Typed-source commands recognizers emit dot-paths
// under the same prefix and are subject to the same canonical-key drift
// guard.
//
// Dot-path emission convention (mirrors skills, rules, hooks, mcp, agents):
//
//   commands.supported                                          = "true"
//   commands.capabilities.<canonical_key>.supported             = "true"
//   commands.capabilities.<canonical_key>.mechanism             = "<human-readable>"
//   commands.capabilities.<canonical_key>.confidence            = "inferred" | "confirmed"
//
// The canonical keys are listed in CanonicalCommandsKeys and defined in
// docs/spec/canonical-keys.yaml under content_types.commands. Recognizers
// MUST only emit dot-paths whose <canonical_key> appears in
// CanonicalCommandsKeys — this is enforced at construction time by
// CommandsLandmarkOptions, which panics on unknown keys.
//
// Object-typed canonical keys (argument_substitution) MAY emit nested
// sub-segments (e.g. "commands.capabilities.argument_substitution.dollar_arguments.supported").
// The canonical-key check applies to the first segment only — nested
// segments are recognizer-defined and reviewed in seeder specs.

import (
	"fmt"
)

// CommandsContentType is the dot-path root for every commands capability emission.
const CommandsContentType = "commands"

// CanonicalCommandsKeys lists every recognized canonical key for the
// commands content type, in the same order as docs/spec/canonical-keys.yaml.
// Recognizers MUST only emit capability dot-paths whose first canonical-key
// segment appears in this list. Validation runs at CommandsLandmarkOptions
// construction time.
//
// To add a key here, first add it to canonical-keys.yaml so the FormatDoc
// validator and seeder-spec audit stay in sync. The two lists drifting
// silently is the failure mode this slice protects against.
var CanonicalCommandsKeys = []string{
	"argument_substitution",
	"builtin_commands",
}

// canonicalCommandsKeySet is the lookup form of CanonicalCommandsKeys, built
// once at package init so IsCanonicalCommandsKey is O(1) without allocating
// per call.
var canonicalCommandsKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CanonicalCommandsKeys))
	for _, k := range CanonicalCommandsKeys {
		set[k] = struct{}{}
	}
	return set
}()

// IsCanonicalCommandsKey reports whether key is one of the canonical commands
// keys declared in CanonicalCommandsKeys. Used by CommandsLandmarkOptions to
// fail fast on typos and by tests to guard the canonical-keys.yaml ↔ Go
// drift surface.
func IsCanonicalCommandsKey(key string) bool {
	_, ok := canonicalCommandsKeySet[key]
	return ok
}

// CommandsLandmarkOptions builds a LandmarkOptions targeting the commands
// content type from the supplied patterns. It validates every pattern's
// Capability against CanonicalCommandsKeys (empty Capability is allowed —
// that pattern only contributes to the top-level commands.supported
// emission) and panics on any unknown key. The panic surfaces typos at
// recognizer registration time, not silently at extraction time.
//
// For object-typed canonical keys (argument_substitution), Capability MAY
// use a dot-separated form like "argument_substitution.dollar_arguments".
// Validation only checks the first segment.
func CommandsLandmarkOptions(patterns ...LandmarkPattern) LandmarkOptions {
	for _, p := range patterns {
		if p.Capability == "" {
			continue
		}
		head := firstSegment(p.Capability)
		if !IsCanonicalCommandsKey(head) {
			panic(fmt.Sprintf("capmon: CommandsLandmarkOptions pattern uses unknown canonical commands key %q (capability=%q); see CanonicalCommandsKeys", head, p.Capability))
		}
	}
	return LandmarkOptions{
		ContentType: CommandsContentType,
		Patterns:    patterns,
	}
}

// CommandsLandmarkPattern is the convenience constructor for the common
// single-anchor case-insensitive substring match. Every documentation-source
// commands recognizer wires up several of these — inlining the StringMatcher
// literal at every call site obscures intent.
//
// canonicalKey is the canonical commands key (validated by
// CommandsLandmarkOptions when the returned pattern is registered). anchor
// is the substring expected in some landmark heading. mechanism is the
// human-readable explanation written to the .mechanism dot-path. required
// is the optional anchor guard shared across patterns in the same
// recognizer (typically the provider's "Slash Commands" or "Commands"
// section heading) — pass nil when no guard is needed.
func CommandsLandmarkPattern(canonicalKey, anchor, mechanism string, required []StringMatcher) LandmarkPattern {
	return LandmarkPattern{
		Capability: canonicalKey,
		Required:   required,
		Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
		Mechanism:  mechanism,
	}
}
