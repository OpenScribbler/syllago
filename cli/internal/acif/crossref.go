package acif

import "fmt"

func DerivedCapabilitiesForItem(item map[string]any) (map[string]bool, error) {
	switch detectItemKind(item) {
	case "skill":
		return skillDerivedCapabilities(item)
	case "rule":
		return ruleDerivedCapabilities(item)
	case "agent":
		return agentDerivedCapabilities(item)
	case "mcp":
		block, _ := unwrapItemBlock(item, "mcp")
		return mcpDerivedCapabilities(block)
	default:
		return DerivedCapabilities(item)
	}
}

func detectItemKind(item map[string]any) string {
	if item == nil {
		return ""
	}
	for _, key := range []string{"skill", "rule", "command", "agent", "mcp"} {
		if _, ok := item[key]; ok {
			return key
		}
	}
	if kind, _ := item["kind"].(string); kind != "" {
		return kindKey(kind)
	}
	return ""
}

func ResolveReference(input map[string]any) (map[string]any, error) {
	item, _ := input["item"].(map[string]any)
	registry, _ := input["registry_state"].(map[string]any)
	switch detectItemKind(item) {
	case "skill":
		return resolveSkillHookReference(item, registry)
	case "agent":
		return resolveAgentReferences(item, registry)
	default:
		return nil, fmt.Errorf("unsupported reference item")
	}
}

func resolveSkillHookReference(item map[string]any, registry map[string]any) (map[string]any, error) {
	block, _ := unwrapItemBlock(item, "skill")
	result, err := CanonicalizeSkill(block)
	if err != nil {
		return nil, err
	}
	activation, _ := result.Canonical["activation"].(map[string]any)
	hookRef, _ := activation["hook_ref"].(map[string]any)
	id, _ := hookRef["id"].(string)
	knownHooks := anyStringSlice(registry["known_hooks"])
	resolution := "unresolved"
	if stringSliceContains(knownHooks, id) {
		resolution = "resolved"
	}
	return map[string]any{
		"cross_reference": map[string]any{
			"source_path": "skill.activation.hook_ref",
			"target_kind": "hook",
			"resolution":  resolution,
		},
		"reciprocal_entries": []any{map[string]any{
			"source_path": "skill.activation.hook_ref",
			"target_kind": "skill",
		}},
		"install": "proceed",
	}, nil
}

func resolveAgentReferences(item map[string]any, registry map[string]any) (map[string]any, error) {
	block, _ := unwrapItemBlock(item, "agent")
	result, err := CanonicalizeAgent(block, "")
	if err != nil {
		return nil, err
	}
	known, _ := registry["known_mcp_items_with_server"].(map[string]any)
	servers := anyStringSlice(result.Canonical["mcp_servers"])
	entries := make([]map[string]any, 0, len(servers))
	var diagnostics []Diagnostic
	install := "proceed"
	for i, name := range servers {
		sourcePath := fmt.Sprintf("agent.mcp_servers[%d]", i)
		entry := map[string]any{
			"source_path":   sourcePath,
			"declared_name": name,
			"target_kind":   "mcp_config",
		}
		if targetID, ok := known[name].(string); ok && targetID != "" {
			entry["resolution"] = "resolved"
			entry["target_id"] = targetID
		} else {
			entry["resolution"] = "unresolved"
			diagnostics = append(diagnostics, Diagnostic{
				ID:     DiagRegistryReferenceUnresolved,
				Params: map[string]any{"declared_name": name},
			})
			install = "refuse-unless-operator-opt-in"
		}
		entries = append(entries, entry)
	}
	out := map[string]any{"install": install}
	if len(entries) == 1 {
		out["cross_reference"] = entries[0]
	} else {
		out["cross_references"] = entries
	}
	if len(diagnostics) > 0 {
		out["diagnostics"] = diagnostics
	}
	return out, nil
}
