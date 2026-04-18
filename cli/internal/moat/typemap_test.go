package moat

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestToMOATType locks the plural→singular mapping at the MOAT manifest
// emission boundary. Any drift here silently produces non-conforming
// manifests that every conforming client would then reject.
func TestToMOATType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       catalog.ContentType
		want     string
		wantEmit bool
	}{
		// MOAT normative types (v0.6.0).
		{"skills_to_skill", catalog.Skills, "skill", true},
		{"agents_to_agent", catalog.Agents, "agent", true},
		{"commands_to_command", catalog.Commands, "command", true},
		{"rules_passthrough", catalog.Rules, "rules", true},

		// Deferred in MOAT — must not appear in manifest.
		{"hooks_deferred", catalog.Hooks, "", false},
		{"mcp_deferred", catalog.MCP, "", false},

		// Syllago-specific — excluded from MOAT entirely.
		{"loadouts_excluded", catalog.Loadouts, "", false},

		// Virtual types — never serialized anywhere.
		{"search_virtual", catalog.SearchResults, "", false},
		{"library_virtual", catalog.Library, "", false},

		// Unknown raw string — defensively excluded.
		{"unknown_string", catalog.ContentType("foobar"), "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, gotEmit := ToMOATType(tt.in)
			if got != tt.want || gotEmit != tt.wantEmit {
				t.Errorf("ToMOATType(%q) = (%q, %v); want (%q, %v)",
					tt.in, got, gotEmit, tt.want, tt.wantEmit)
			}
		})
	}
}

// TestFromMOATType locks the reverse mapping used when parsing MOAT manifests.
// Deferred types ("hook", "mcp") MUST return false so clients ignore them —
// an emitted manifest containing those would be non-conforming under v0.6.0.
func TestFromMOATType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     string
		want   catalog.ContentType
		wantOK bool
	}{
		{"skill_to_skills", "skill", catalog.Skills, true},
		{"agent_to_agents", "agent", catalog.Agents, true},
		{"command_to_commands", "command", catalog.Commands, true},
		{"rules_passthrough", "rules", catalog.Rules, true},

		// Deferred or unknown: no mapping.
		{"hook_deferred", "hook", "", false},
		{"mcp_deferred", "mcp", "", false},
		{"loadout_excluded", "loadout", "", false},
		{"empty_string", "", "", false},
		{"unknown_type", "widget", "", false},

		// Case sensitivity: MOAT type strings are lowercase. Upper forms
		// must not match.
		{"case_sensitive_Skill", "Skill", "", false},
		{"case_sensitive_AGENT", "AGENT", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, gotOK := FromMOATType(tt.in)
			if got != tt.want || gotOK != tt.wantOK {
				t.Errorf("FromMOATType(%q) = (%q, %v); want (%q, %v)",
					tt.in, got, gotOK, tt.want, tt.wantOK)
			}
		})
	}
}

// TestMOATTypeRoundtrip verifies every MOAT-emittable Syllago type survives
// a roundtrip through both translators — the guard that keeps emit and
// parse tables in sync.
func TestMOATTypeRoundtrip(t *testing.T) {
	t.Parallel()

	emittable := []catalog.ContentType{
		catalog.Skills, catalog.Agents, catalog.Commands, catalog.Rules,
	}
	for _, ct := range emittable {
		ct := ct
		t.Run(string(ct), func(t *testing.T) {
			t.Parallel()
			moatStr, ok := ToMOATType(ct)
			if !ok {
				t.Fatalf("ToMOATType(%q) unexpectedly excluded from MOAT", ct)
			}
			back, ok := FromMOATType(moatStr)
			if !ok {
				t.Fatalf("FromMOATType(%q) = (_, false); expected reverse mapping", moatStr)
			}
			if back != ct {
				t.Errorf("roundtrip drift: %q → %q → %q", ct, moatStr, back)
			}
		})
	}
}

// TestIsMOATEmittable mirrors ToMOATType's bool return but is the more
// readable predicate at call sites doing "if IsMOATEmittable(item.Type)".
func TestIsMOATEmittable(t *testing.T) {
	t.Parallel()

	emittable := []catalog.ContentType{
		catalog.Skills, catalog.Agents, catalog.Commands, catalog.Rules,
	}
	excluded := []catalog.ContentType{
		catalog.Hooks, catalog.MCP, catalog.Loadouts,
		catalog.SearchResults, catalog.Library,
		catalog.ContentType("unknown"),
	}

	for _, ct := range emittable {
		if !IsMOATEmittable(ct) {
			t.Errorf("IsMOATEmittable(%q) = false; want true", ct)
		}
	}
	for _, ct := range excluded {
		if IsMOATEmittable(ct) {
			t.Errorf("IsMOATEmittable(%q) = true; want false", ct)
		}
	}
}
