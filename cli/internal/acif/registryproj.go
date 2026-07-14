package acif

import "fmt"

func TupleEndpointProjection(item map[string]any) map[string]any {
	projection := map[string]any{
		"tuple_fields": []any{"item_id", "body_hash", "metadata_hash_if_present", "version_if_declared"},
	}
	members, _ := item["pack_members"].([]any)
	for i, rawMember := range members {
		member, _ := rawMember.(map[string]any)
		value := any(nil)
		if member != nil && member["publisher_section"] == "present" {
			value = "present"
		}
		projection[fmt.Sprintf("member_%d", i+1)] = map[string]any{"metadata_hash": value}
	}
	return projection
}

func ValidateInstallScopeCapabilities(item map[string]any) map[string]any {
	for _, rawEntry := range item {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := entry["source"]; !ok {
			return map[string]any{
				"conformant": false,
				"reason":     ReasonRegistryProvenanceTagMissing,
			}
		}
	}
	return map[string]any{"conformant": true}
}

func ValidateRegistryAdvisory(item map[string]any) map[string]any {
	for _, rawEntry := range item {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := entry["method"]; !ok {
			return map[string]any{
				"conformant": false,
				"reason":     ReasonRegistryMethodStampMissing,
			}
		}
	}
	return map[string]any{"conformant": true}
}

func EvaluateRegistryInstallCrossReferences(item map[string]any) (map[string]any, bool) {
	refs, ok := item["cross_references"].([]any)
	if !ok {
		return nil, false
	}
	for _, rawRef := range refs {
		ref, ok := rawRef.(map[string]any)
		if !ok {
			continue
		}
		switch ref["resolution"] {
		case "unresolved", "revoked":
			return map[string]any{"install": "refuse-unless-operator-opt-in"}, true
		}
	}
	return nil, false
}

func ValidateRegistryEmitSidecar(sidecar map[string]any) (map[string]any, bool, error) {
	if sidecar == nil {
		return nil, false, nil
	}
	if _, ok := sidecar["kind"]; ok {
		return nil, false, nil
	}
	if _, ok := sidecar["publisher_section"]; ok {
		return nil, false, nil
	}
	rawRegistry, ok := sidecar["registry_section"]
	if !ok {
		return nil, false, nil
	}
	registry, _ := rawRegistry.(map[string]any)
	sourceURI, _ := registry["source_uri"].(string)
	if sourceURI == "" {
		return nil, true, &RejectError{ID: ErrSourceURIMissing}
	}
	return map[string]any{"conformant": true}, true, nil
}
