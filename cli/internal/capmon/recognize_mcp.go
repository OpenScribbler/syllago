package capmon

// Canonical MCP recognition schema and helpers.
//
// MCP (Model Context Protocol) data is sourced from a mix of typed schemas
// (JSON Schema config files, TypeScript types, Rust/Go structs) and
// documentation markdown — providers describe transport types, server scopes,
// OAuth flows, env-var expansion, allowlists, and resource referencing across
// both shapes. Recognizers may use either landmark matching (doc-source) or
// typed-source extraction (schema-source); this file declares the canonical
// vocabulary and landmark-helper constructors. Typed-source MCP recognizers
// emit dot-paths under the same prefix and are subject to the same canonical-
// key drift guard.
//
// Dot-path emission convention (mirrors skills, rules, hooks):
//
//   mcp.supported                                          = "true"
//   mcp.capabilities.<canonical_key>.supported             = "true"
//   mcp.capabilities.<canonical_key>.mechanism             = "<human-readable>"
//   mcp.capabilities.<canonical_key>.confidence            = "inferred" | "confirmed"
//
// The canonical keys are listed in CanonicalMcpKeys and defined in
// docs/spec/canonical-keys.yaml under content_types.mcp. Recognizers MUST
// only emit dot-paths whose <canonical_key> appears in CanonicalMcpKeys —
// this is enforced at construction time by McpLandmarkOptions, which panics
// on unknown keys.
//
// Object-typed canonical keys (transport_types, tool_filtering) MAY emit
// nested sub-segments (e.g. "mcp.capabilities.transport_types.stdio.supported"
// or "mcp.capabilities.tool_filtering.allowlist.supported"). The canonical-
// key check applies to the first segment only — nested segments are
// recognizer-defined and reviewed in seeder specs.

import (
	"fmt"
)

// McpContentType is the dot-path root for every MCP capability emission.
const McpContentType = "mcp"

// CanonicalMcpKeys lists every recognized canonical key for the MCP content
// type, in the same order as docs/spec/canonical-keys.yaml. Recognizers MUST
// only emit capability dot-paths whose first canonical-key segment appears in
// this list. Validation runs at McpLandmarkOptions construction time.
//
// To add a key here, first add it to canonical-keys.yaml so the FormatDoc
// validator and seeder-spec audit stay in sync. The two lists drifting silently
// is the failure mode this slice protects against.
var CanonicalMcpKeys = []string{
	"transport_types",
	"oauth_support",
	"env_var_expansion",
	"tool_filtering",
	"auto_approve",
	"marketplace",
	"resource_referencing",
	"enterprise_management",
}

// canonicalMcpKeySet is the lookup form of CanonicalMcpKeys, built once at
// package init so IsCanonicalMcpKey is O(1) without allocating per call.
var canonicalMcpKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CanonicalMcpKeys))
	for _, k := range CanonicalMcpKeys {
		set[k] = struct{}{}
	}
	return set
}()

// IsCanonicalMcpKey reports whether key is one of the canonical MCP keys
// declared in CanonicalMcpKeys. Used by McpLandmarkOptions to fail fast on
// typos and by tests to guard the canonical-keys.yaml ↔ Go drift surface.
func IsCanonicalMcpKey(key string) bool {
	_, ok := canonicalMcpKeySet[key]
	return ok
}

// McpLandmarkOptions builds a LandmarkOptions targeting the mcp content type
// from the supplied patterns. It validates every pattern's Capability against
// CanonicalMcpKeys (empty Capability is allowed — that pattern only contributes
// to the top-level mcp.supported emission) and panics on any unknown key. The
// panic surfaces typos at recognizer registration time, not silently at
// extraction time.
//
// For object-typed canonical keys (transport_types, tool_filtering),
// Capability MAY use a dot-separated form like "transport_types.stdio" or
// "tool_filtering.allowlist". Validation only checks the first segment.
func McpLandmarkOptions(patterns ...LandmarkPattern) LandmarkOptions {
	for _, p := range patterns {
		if p.Capability == "" {
			continue
		}
		head := firstSegment(p.Capability)
		if !IsCanonicalMcpKey(head) {
			panic(fmt.Sprintf("capmon: McpLandmarkOptions pattern uses unknown canonical mcp key %q (capability=%q); see CanonicalMcpKeys", head, p.Capability))
		}
	}
	return LandmarkOptions{
		ContentType: McpContentType,
		Patterns:    patterns,
	}
}

// McpLandmarkPattern is the convenience constructor for the common
// single-anchor case-insensitive substring match. Every documentation-source
// MCP recognizer wires up several of these — inlining the StringMatcher
// literal at every call site obscures intent.
//
// canonicalKey is the canonical MCP key (validated by McpLandmarkOptions
// when the returned pattern is registered). anchor is the substring expected
// in some landmark heading. mechanism is the human-readable explanation
// written to the .mechanism dot-path. required is the optional anchor guard
// shared across patterns in the same recognizer (typically the provider's
// "MCP" section heading) — pass nil when no guard is needed.
func McpLandmarkPattern(canonicalKey, anchor, mechanism string, required []StringMatcher) LandmarkPattern {
	return LandmarkPattern{
		Capability: canonicalKey,
		Required:   required,
		Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
		Mechanism:  mechanism,
	}
}
