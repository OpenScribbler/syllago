package capmon

// Canonical agents recognition schema and helpers.
//
// Agents (also called "subagents", "personas", "modes", or "@mentions"
// depending on provider) are reusable role-scoped configurations that
// constrain a model's behavior, tool access, or context. Provider data is
// sourced from a mix of typed schemas (TOML/YAML/JSON config types,
// TS/Rust/Go structs) and documentation markdown — providers describe
// definition formats, scope hierarchies, invocation patterns, tool
// restrictions, model overrides, per-agent MCP, and subagent spawning across
// both shapes. Recognizers may use either landmark matching (doc-source) or
// typed-source extraction (schema-source); this file declares the canonical
// vocabulary and landmark-helper constructors. Typed-source agents
// recognizers emit dot-paths under the same prefix and are subject to the
// same canonical-key drift guard.
//
// Dot-path emission convention (mirrors skills, rules, hooks, mcp):
//
//   agents.supported                                          = "true"
//   agents.capabilities.<canonical_key>.supported             = "true"
//   agents.capabilities.<canonical_key>.mechanism             = "<human-readable>"
//   agents.capabilities.<canonical_key>.confidence            = "inferred" | "confirmed"
//
// The canonical keys are listed in CanonicalAgentsKeys and defined in
// docs/spec/canonical-keys.yaml under content_types.agents. Recognizers MUST
// only emit dot-paths whose <canonical_key> appears in CanonicalAgentsKeys —
// this is enforced at construction time by AgentsLandmarkOptions, which
// panics on unknown keys.
//
// Object-typed canonical keys (invocation_patterns, agent_scopes) MAY emit
// nested sub-segments (e.g. "agents.capabilities.invocation_patterns.at_mention.supported"
// or "agents.capabilities.agent_scopes.project.supported"). The canonical-
// key check applies to the first segment only — nested segments are
// recognizer-defined and reviewed in seeder specs.

import (
	"fmt"
)

// AgentsContentType is the dot-path root for every agents capability emission.
const AgentsContentType = "agents"

// CanonicalAgentsKeys lists every recognized canonical key for the agents
// content type, in the same order as docs/spec/canonical-keys.yaml.
// Recognizers MUST only emit capability dot-paths whose first canonical-key
// segment appears in this list. Validation runs at AgentsLandmarkOptions
// construction time.
//
// To add a key here, first add it to canonical-keys.yaml so the FormatDoc
// validator and seeder-spec audit stay in sync. The two lists drifting
// silently is the failure mode this slice protects against.
var CanonicalAgentsKeys = []string{
	"definition_format",
	"tool_restrictions",
	"invocation_patterns",
	"agent_scopes",
	"model_selection",
	"per_agent_mcp",
	"subagent_spawning",
}

// canonicalAgentsKeySet is the lookup form of CanonicalAgentsKeys, built
// once at package init so IsCanonicalAgentsKey is O(1) without allocating
// per call.
var canonicalAgentsKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CanonicalAgentsKeys))
	for _, k := range CanonicalAgentsKeys {
		set[k] = struct{}{}
	}
	return set
}()

// IsCanonicalAgentsKey reports whether key is one of the canonical agents
// keys declared in CanonicalAgentsKeys. Used by AgentsLandmarkOptions to
// fail fast on typos and by tests to guard the canonical-keys.yaml ↔ Go
// drift surface.
func IsCanonicalAgentsKey(key string) bool {
	_, ok := canonicalAgentsKeySet[key]
	return ok
}

// AgentsLandmarkOptions builds a LandmarkOptions targeting the agents
// content type from the supplied patterns. It validates every pattern's
// Capability against CanonicalAgentsKeys (empty Capability is allowed —
// that pattern only contributes to the top-level agents.supported emission)
// and panics on any unknown key. The panic surfaces typos at recognizer
// registration time, not silently at extraction time.
//
// For object-typed canonical keys (invocation_patterns, agent_scopes),
// Capability MAY use a dot-separated form like "invocation_patterns.at_mention"
// or "agent_scopes.project". Validation only checks the first segment.
func AgentsLandmarkOptions(patterns ...LandmarkPattern) LandmarkOptions {
	for _, p := range patterns {
		if p.Capability == "" {
			continue
		}
		head := firstSegment(p.Capability)
		if !IsCanonicalAgentsKey(head) {
			panic(fmt.Sprintf("capmon: AgentsLandmarkOptions pattern uses unknown canonical agents key %q (capability=%q); see CanonicalAgentsKeys", head, p.Capability))
		}
	}
	return LandmarkOptions{
		ContentType: AgentsContentType,
		Patterns:    patterns,
	}
}

// AgentsLandmarkPattern is the convenience constructor for the common
// single-anchor case-insensitive substring match. Every documentation-source
// agents recognizer wires up several of these — inlining the StringMatcher
// literal at every call site obscures intent.
//
// canonicalKey is the canonical agents key (validated by AgentsLandmarkOptions
// when the returned pattern is registered). anchor is the substring expected
// in some landmark heading. mechanism is the human-readable explanation
// written to the .mechanism dot-path. required is the optional anchor guard
// shared across patterns in the same recognizer (typically the provider's
// "Subagents" or "Agents" section heading) — pass nil when no guard is
// needed.
func AgentsLandmarkPattern(canonicalKey, anchor, mechanism string, required []StringMatcher) LandmarkPattern {
	return LandmarkPattern{
		Capability: canonicalKey,
		Required:   required,
		Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
		Mechanism:  mechanism,
	}
}
