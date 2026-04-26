package moat

// MOAT v0.6.0 content type alignment (spec §Registry Manifest, see plan G-12).
//
// MOAT normative types: "skill", "agent", "rules", "command".
// Deferred in MOAT (not yet normative): "hook", "mcp".
// Syllago-specific (excluded from MOAT): "loadouts".
//
// Syllago's catalog uses plural forms internally. This file translates at the
// MOAT manifest boundary only — no internal renames are required. Types that
// MOAT does not yet cover are passed through the rest of Syllago but must not
// appear in emitted MOAT manifests.

import "github.com/OpenScribbler/syllago/cli/internal/catalog"

// ToMOATType maps a Syllago catalog.ContentType to its MOAT manifest type
// string. Returns the MOAT form and true when the type is emitted to MOAT
// manifests; returns ("", false) for types excluded from MOAT (loadouts,
// hooks, mcp, virtual types like search/library).
//
// Types excluded from MOAT manifest emission:
//   - Loadouts: Syllago-specific, not part of the MOAT spec.
//   - Hooks, MCP: deferred in MOAT v0.6.0; pass through internally but MUST
//     NOT appear in MOAT manifests until the spec defines them.
//   - SearchResults, Library: virtual types for UI, never serialized.
func ToMOATType(ct catalog.ContentType) (string, bool) {
	switch ct {
	case catalog.Skills:
		return "skill", true
	case catalog.Agents:
		return "agent", true
	case catalog.Commands:
		return "command", true
	case catalog.Rules:
		return "rules", true
	}
	return "", false
}

// FromMOATType maps a MOAT manifest type string to its Syllago ContentType.
// Returns the catalog.ContentType and true for MOAT-recognized types;
// returns ("", false) for deferred types ("hook", "mcp") and unknown strings.
//
// Conforming clients MUST ignore manifest entries whose type is not
// recognized — the MOAT spec reserves the namespace for future extension.
func FromMOATType(s string) (catalog.ContentType, bool) {
	switch s {
	case "skill":
		return catalog.Skills, true
	case "agent":
		return catalog.Agents, true
	case "command":
		return catalog.Commands, true
	case "rules":
		return catalog.Rules, true
	}
	return "", false
}

// IsMOATEmittable reports whether ct should appear in MOAT manifest output.
// Equivalent to the second return value of ToMOATType — provided as a named
// predicate for readability at emission call sites.
func IsMOATEmittable(ct catalog.ContentType) bool {
	_, ok := ToMOATType(ct)
	return ok
}

// CategoryDirForMOATType returns the canonical category-directory name a
// content item of the given MOAT type lives under in the publisher source
// repo. Defined in moat-spec.md §"Repository Layout":
//
//	command → commands/
//	rules   → rules/
//	skill   → skills/
//	agent   → agents/
//
// Returns ("", false) for unknown types — install paths MUST treat unknown
// types as ignorable per spec (the type namespace is reserved for future
// extension).
//
// .moat/publisher.yml override (Tier 2 discovery) is NOT covered here — a
// follow-up bead should add that lookup so non-canonical layouts can be
// installed. Until then, install only succeeds for items at the canonical
// path.
func CategoryDirForMOATType(typeStr string) (string, bool) {
	switch typeStr {
	case "command":
		return "commands", true
	case "rules":
		return "rules", true
	case "skill":
		return "skills", true
	case "agent":
		return "agents", true
	}
	return "", false
}
