package acif

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestPackManifestReconciliation(t *testing.T) {
	t.Parallel()

	t.Run("winner follows source precedence", func(t *testing.T) {
		t.Parallel()
		got := ReconcilePackManifests([]PackManifest{
			{Source: ".codex-plugin/plugin.json", Name: "superpowers"},
			{Source: "package.json", Name: "superpowers"},
			{Source: "gemini-extension.json", Name: "superpowers"},
		})
		if got.CanonicalSource != "package.json" || got.CanonicalDisplayName != "superpowers" {
			t.Fatalf("ReconcilePackManifests() = %+v, want package.json/superpowers", got)
		}
		if len(got.Diagnostics) != 0 {
			t.Fatalf("diagnostics = %#v, want none", got.Diagnostics)
		}
	})

	t.Run("empty name falls through", func(t *testing.T) {
		t.Parallel()
		got := ReconcilePackManifests([]PackManifest{
			{Source: "package.json", Name: " \t\n"},
			{Source: ".codex-plugin/plugin.json", Name: " codex-pack "},
			{Source: "gemini-extension.json", Name: "gemini-pack"},
		})
		if got.CanonicalSource != ".codex-plugin/plugin.json" || got.CanonicalDisplayName != "codex-pack" {
			t.Fatalf("ReconcilePackManifests() = %+v, want codex plugin/codex-pack", got)
		}
		if len(got.Diagnostics) != 1 {
			t.Fatalf("diagnostics = %#v, want conflict from non-empty names", got.Diagnostics)
		}
	})

	t.Run("conflict diagnostic preserves input order", func(t *testing.T) {
		t.Parallel()
		got := ReconcilePackManifests([]PackManifest{
			{Source: "package.json", Name: "superpowers"},
			{Source: "gemini-extension.json", Name: "super-powers"},
		})
		if got.CanonicalSource != "package.json" || got.CanonicalDisplayName != "superpowers" {
			t.Fatalf("ReconcilePackManifests() = %+v, want package.json/superpowers", got)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].ID != "acif.publisher.pack_source_conflict" {
			t.Fatalf("diagnostics = %#v, want one pack_source_conflict", got.Diagnostics)
		}
		if !reflect.DeepEqual(got.Diagnostics[0].Params["sources"], []string{"package.json", "gemini-extension.json"}) {
			t.Fatalf("sources = %#v", got.Diagnostics[0].Params["sources"])
		}
		if !reflect.DeepEqual(got.Diagnostics[0].Params["values"], []string{"superpowers", "super-powers"}) {
			t.Fatalf("values = %#v", got.Diagnostics[0].Params["values"])
		}
	})

	t.Run("no conflict for equal trimmed names", func(t *testing.T) {
		t.Parallel()
		got := ReconcilePackManifests([]PackManifest{
			{Source: "package.json", Name: " superpowers "},
			{Source: "gemini-extension.json", Name: "superpowers"},
		})
		if got.CanonicalSource != "package.json" || got.CanonicalDisplayName != "superpowers" {
			t.Fatalf("ReconcilePackManifests() = %+v, want package.json/superpowers", got)
		}
		if len(got.Diagnostics) != 0 {
			t.Fatalf("diagnostics = %#v, want none", got.Diagnostics)
		}
	})
}

func TestReconcileFrontmatter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		side   map[string]any
		source map[string]any
		mode   string
		action string
		diags  int
	}{
		{
			name:   "default absent adds silently",
			side:   map[string]any{"description": "demo"},
			source: map[string]any{},
			mode:   "default",
			action: "add-silently",
		},
		{
			name:   "overwrite absent adds silently",
			side:   map[string]any{"description": "demo"},
			source: map[string]any{},
			mode:   "overwrite",
			action: "add-silently",
		},
		{
			name:   "default equal leaves untouched",
			side:   map[string]any{"tools": []any{"Read", "Task"}},
			source: map[string]any{"tools": []any{"Read", "Task"}},
			mode:   "default",
			action: "leave-untouched",
		},
		{
			name:   "overwrite equal leaves untouched",
			side:   map[string]any{"tools": []any{"Read", "Task"}},
			source: map[string]any{"tools": []any{"Read", "Task"}},
			mode:   "overwrite",
			action: "leave-untouched",
		},
		{
			name:   "default conflict blocks",
			side:   map[string]any{"description": "canonical"},
			source: map[string]any{"description": "declared"},
			mode:   "default",
			action: "block",
			diags:  1,
		},
		{
			name:   "overwrite conflict overwrites",
			side:   map[string]any{"description": "canonical"},
			source: map[string]any{"description": "declared"},
			mode:   "overwrite",
			action: "overwrite",
			diags:  1,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ReconcileFrontmatter(tc.side, tc.source, tc.mode)
			if got.Action != tc.action || len(got.Diagnostics) != tc.diags {
				t.Fatalf("ReconcileFrontmatter() = %+v, want action=%q diagnostics=%d", got, tc.action, tc.diags)
			}
			if tc.diags > 0 {
				diag := got.Diagnostics[0]
				if diag.ID != "acif.publisher.frontmatter_conflict" {
					t.Fatalf("diagnostic ID = %q", diag.ID)
				}
				if diag.Params["field"] != "description" || diag.Params["declared"] != "declared" || diag.Params["canonical"] != "canonical" {
					t.Fatalf("diagnostic params = %#v", diag.Params)
				}
			}
		})
	}

	t.Run("multiple fields choose strongest action", func(t *testing.T) {
		t.Parallel()
		block := ReconcileFrontmatter(
			map[string]any{"a": "new", "b": "canonical", "c": "same"},
			map[string]any{"b": "declared", "c": "same"},
			"default",
		)
		if block.Action != "block" {
			t.Fatalf("default action = %q, want block", block.Action)
		}

		overwrite := ReconcileFrontmatter(
			map[string]any{"a": "new", "b": "canonical", "c": "same"},
			map[string]any{"b": "declared", "c": "same"},
			"overwrite",
		)
		if overwrite.Action != "overwrite" {
			t.Fatalf("overwrite action = %q, want overwrite", overwrite.Action)
		}
	})
}

func TestEnvelopePublisherSectionForHookSidecar(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"kind":         "hook",
		"id":           "550e8400-e29b-41d4-a716-446655440000",
		"display_name": "Run Checks",
		"pack_id":      "11111111-1111-4111-8111-111111111111",
		"hook": map[string]any{
			"event": "before_tool_execute",
			"handlers": []any{
				map[string]any{"scripts": []any{map[string]any{"type": "inline", "content": "#!/bin/sh\nexit 0\n"}}},
			},
		},
	}
	edited := cloneJSONValue(base).(map[string]any)
	edited["display_name"] = "Run Checks Now"

	section := EnvelopePublisherSection(base)
	keys := make([]string, 0, len(section))
	for key := range section {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"display_name", "id", "kind", "pack_id"}) {
		t.Fatalf("publisher_section keys = %#v", keys)
	}
	if _, ok := section["hook"]; ok {
		t.Fatalf("publisher_section includes hook extension: %#v", section)
	}

	hash := metadataHashForMap(t, section)
	editedHash := metadataHashForMap(t, EnvelopePublisherSection(edited))
	if hash == editedHash {
		t.Fatalf("display_name edit did not move metadata_hash: %s", hash)
	}

	baseHook, err := CanonicalizeHook(base, HookOpts{})
	if err != nil {
		t.Fatalf("CanonicalizeHook(base): %v", err)
	}
	editedHook, err := CanonicalizeHook(edited, HookOpts{})
	if err != nil {
		t.Fatalf("CanonicalizeHook(edited): %v", err)
	}
	baseBody := hookHash(t, baseHook.Canonical, "")
	editedBody := hookHash(t, editedHook.Canonical, "")
	if baseBody != editedBody {
		t.Fatalf("display_name edit moved body_hash: %s != %s", baseBody, editedBody)
	}
}

func TestPackSidecarHashing(t *testing.T) {
	t.Parallel()

	declared, err := IngestPackSidecar(map[string]any{"source_kind": "declared"})
	if err != nil {
		t.Fatalf("IngestPackSidecar(declared): %v", err)
	}
	if !declared.Conformant || !declared.Installable {
		t.Fatalf("declared conformant/installable = %#v/%#v", declared.Conformant, declared.Installable)
	}
	if !reflect.DeepEqual(declared.PublisherSection, map[string]any{"source_kind": "declared"}) {
		t.Fatalf("declared publisher_section = %#v", declared.PublisherSection)
	}
	if declared.MetadataHash == "" {
		t.Fatalf("declared metadata_hash is empty")
	}
	if declared.BodyHash != "" {
		t.Fatalf("declared body_hash = %q, want empty", declared.BodyHash)
	}

	inferred, err := IngestPackSidecar(map[string]any{"source_kind": "inferred"})
	if err != nil {
		t.Fatalf("IngestPackSidecar(inferred): %v", err)
	}
	if !inferred.Conformant || !inferred.Installable {
		t.Fatalf("inferred conformant/installable = %#v/%#v", inferred.Conformant, inferred.Installable)
	}
	if inferred.PublisherSection != nil || inferred.MetadataHash != "" || inferred.BodyHash != "" {
		t.Fatalf("inferred hash fields = publisher_section:%#v metadata_hash:%q body_hash:%q",
			inferred.PublisherSection, inferred.MetadataHash, inferred.BodyHash)
	}
}

func TestProviderNativeFrontmatterAgent(t *testing.T) {
	t.Parallel()

	got, err := IngestProviderNativeFrontmatter("agent", map[string]any{
		"tools": []any{"Read", "Task"},
	})
	if err != nil {
		t.Fatalf("IngestProviderNativeFrontmatter(agent): %v", err)
	}
	if !got.Conformant || !got.Installable {
		t.Fatalf("conformant/installable = %#v/%#v", got.Conformant, got.Installable)
	}
	publisherAgent, ok := got.PublisherSection["agent"].(map[string]any)
	if !ok {
		t.Fatalf("publisher_section.agent = %#v", got.PublisherSection["agent"])
	}
	if !reflect.DeepEqual(publisherAgent["tools"], []any{"Read", "Task"}) {
		t.Fatalf("publisher tools = %#v, want declared spellings", publisherAgent["tools"])
	}
	canonicalAgent, ok := got.Canonical["agent"].(map[string]any)
	if !ok {
		t.Fatalf("canonical.agent = %#v", got.Canonical["agent"])
	}
	if !reflect.DeepEqual(canonicalAgent["tools"], []any{"file_read", "agent"}) {
		t.Fatalf("canonical tools = %#v, want translated names", canonicalAgent["tools"])
	}
	if got.MetadataHash == "" || got.BodyHash != "" {
		t.Fatalf("hash fields = metadata_hash:%q body_hash:%q", got.MetadataHash, got.BodyHash)
	}
}

func metadataHashForMap(t *testing.T, section map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(section)
	if err != nil {
		t.Fatalf("marshal publisher_section: %v", err)
	}
	hash, _, err := MetadataHash(raw)
	if err != nil {
		t.Fatalf("MetadataHash(): %v", err)
	}
	return hash
}

func TestProviderNativeFrontmatterOrphanRequires(t *testing.T) {
	t.Parallel()

	got, err := IngestProviderNativeFrontmatter("agent", map[string]any{
		"requires": map[string]any{"tool_restrictions": true},
	})
	if err != nil {
		t.Fatalf("IngestProviderNativeFrontmatter(orphan requires): %v", err)
	}
	if got.Conformant || got.Installable {
		t.Fatalf("conformant/installable = %v/%v, want false/false", got.Conformant, got.Installable)
	}
	if got.Reason != ReasonRequiresOrphanKey {
		t.Fatalf("reason = %q, want %q", got.Reason, ReasonRequiresOrphanKey)
	}
	if got.Params["key"] != "tool_restrictions" {
		t.Fatalf("params = %#v, want key=tool_restrictions", got.Params)
	}
}
