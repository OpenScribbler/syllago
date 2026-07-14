package acif

import (
	"reflect"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
)

func TestAgentToolNamesAppendixARow(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"claude-code":     "Agent",
		"copilot-cli":     "task",
		"opencode":        "task",
		"vs-code-copilot": "Agent",
		"zed":             "spawn_agent",
		"codex":           "spawn_agent",
		"kiro":            "use_subagent",
		"factory-droid":   "Task",
	}
	if !reflect.DeepEqual(converter.ToolNames["agent"], want) {
		t.Fatalf("converter.ToolNames[agent] = %#v, want %#v", converter.ToolNames["agent"], want)
	}
}

func TestAgentTranslationAndDerivedCapabilities(t *testing.T) {
	t.Parallel()

	spellings := []string{"Agent", "task", "spawn_agent", "use_subagent", "Task"}
	for _, spelling := range spellings {
		spelling := spelling
		t.Run(spelling, func(t *testing.T) {
			t.Parallel()
			got, err := CanonicalizeAgent(map[string]any{"tools": []any{spelling}}, "")
			if err != nil {
				t.Fatalf("CanonicalizeAgent(%s): %v", spelling, err)
			}
			if !reflect.DeepEqual(got.Canonical["tools"], []any{"agent"}) {
				t.Fatalf("tools = %#v, want [agent]", got.Canonical["tools"])
			}
		})
	}

	item := map[string]any{"agent": map[string]any{
		"tools":            []any{"spawn_agent"},
		"disallowed_tools": []any{"Bash"},
		"model":            "gpt-5-codex",
		"mcp_servers":      []any{"demo"},
	}}
	caps, err := DerivedCapabilitiesForItem(item)
	if err != nil {
		t.Fatalf("DerivedCapabilitiesForItem(agent): %v", err)
	}
	want := map[string]bool{
		"tool_restrictions": true,
		"model_selection":   true,
		"per_agent_mcp":     true,
		"subagent_spawning": true,
	}
	if !reflect.DeepEqual(caps, want) {
		t.Fatalf("agent caps = %#v, want %#v", caps, want)
	}

	nulls, err := DerivedCapabilitiesForItem(map[string]any{"agent": map[string]any{
		"tools":            nil,
		"disallowed_tools": nil,
	}})
	if err != nil {
		t.Fatalf("DerivedCapabilitiesForItem(agent nulls): %v", err)
	}
	if nulls["tool_restrictions"] {
		t.Fatalf("null tool arrays counted as restrictions: %#v", nulls)
	}
}

func TestAgentRenderReingestAndLossy(t *testing.T) {
	t.Parallel()

	item := map[string]any{"agent": map[string]any{"tools": []any{"agent"}, "body": "Help with the task.\n"}}
	for _, provider := range []string{"claude-code", "copilot-cli", "opencode", "vs-code-copilot", "zed", "codex", "kiro", "factory-droid"} {
		provider := provider
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			rendered, err := RenderAgent(item, provider)
			if err != nil {
				t.Fatalf("RenderAgent(%s): %v", provider, err)
			}
			if !strings.Contains(rendered.Output, converter.TranslateTool("agent", provider)) {
				t.Fatalf("render output for %s missing native name: %q", provider, rendered.Output)
			}
			roundtrip, err := CanonicalizeAgentProviderConfig(map[string]any{
				"provider": provider,
				"path":     "agent.rendered",
				"content":  rendered.Output,
			})
			if err != nil {
				t.Fatalf("CanonicalizeAgentProviderConfig(%s): %v", provider, err)
			}
			if !reflect.DeepEqual(roundtrip.Canonical["tools"], []any{"agent"}) {
				t.Fatalf("roundtrip tools = %#v", roundtrip.Canonical["tools"])
			}
		})
	}

	lossy, err := RenderAgent(map[string]any{"agent": map[string]any{"tools": []any{"file_write", "file_edit"}}}, "collapsing-provider")
	if err != nil {
		t.Fatalf("RenderAgent(collapsing): %v", err)
	}
	if !reflect.DeepEqual(lossy.Lossy, []string{"write-edit-distinction"}) {
		t.Fatalf("lossy = %#v", lossy.Lossy)
	}
	rt, err := CanonicalizeAgentProviderConfig(map[string]any{
		"provider": "collapsing-provider",
		"content":  lossy.Output,
	})
	if err != nil {
		t.Fatalf("CanonicalizeAgentProviderConfig(collapsing): %v", err)
	}
	if !reflect.DeepEqual(rt.Canonical["tools"], []any{"file_edit", "file_edit"}) {
		t.Fatalf("collapsing roundtrip tools = %#v", rt.Canonical["tools"])
	}
}

func TestAgentResolveReference(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveReference(map[string]any{
		"item":           map[string]any{"agent": map[string]any{"mcp_servers": []any{"demo"}}},
		"registry_state": map[string]any{"known_mcp_items_with_server": map[string]any{"demo": "550e8400-e29b-41d4-a716-446655440000"}},
	})
	if err != nil {
		t.Fatalf("ResolveReference(resolved): %v", err)
	}
	wantCross := map[string]any{
		"source_path":   "agent.mcp_servers[0]",
		"declared_name": "demo",
		"target_kind":   "mcp_config",
		"resolution":    "resolved",
		"target_id":     "550e8400-e29b-41d4-a716-446655440000",
	}
	if !reflect.DeepEqual(resolved["cross_reference"], wantCross) || resolved["install"] != "proceed" {
		t.Fatalf("resolved result = %#v", resolved)
	}

	unresolved, err := ResolveReference(map[string]any{
		"item":           map[string]any{"agent": map[string]any{"mcp_servers": []any{"missing"}}},
		"registry_state": map[string]any{"known_mcp_items_with_server": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("ResolveReference(unresolved): %v", err)
	}
	if unresolved["install"] != "refuse-unless-operator-opt-in" {
		t.Fatalf("unresolved install = %#v", unresolved)
	}
	diags := unresolved["diagnostics"].([]Diagnostic)
	if len(diags) != 1 || diags[0].Params["declared_name"] != "missing" {
		t.Fatalf("unresolved diagnostics = %#v", diags)
	}
}
