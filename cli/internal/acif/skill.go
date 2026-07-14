package acif

func CanonicalizeSkill(block map[string]any) (*ItemResult, error) {
	block = unwrapKindBlock(block, "skill")
	out := make(map[string]any, len(block)+1)
	for k, v := range block {
		if k == "activation" || k == "requires" {
			continue
		}
		out[k] = cloneJSONValue(v)
	}

	activation, err := canonicalizeSkillActivation(block["activation"])
	if err != nil {
		return nil, err
	}
	out["activation"] = activation

	if requires, ok := block["requires"]; ok {
		out["requires"] = cloneJSONValue(requires)
	}
	verdict := applyRequiresVerdict(out)
	return &ItemResult{Canonical: out, Verdict: verdict}, nil
}

func canonicalizeSkillActivation(raw any) (map[string]any, error) {
	if raw == nil {
		return map[string]any{"type": "auto", "user_invocable": true}, nil
	}
	in, _ := raw.(map[string]any)
	if in == nil {
		return nil, &RejectError{ID: ErrSkillActivationTypeMissing}
	}
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		if k == "type" || k == "user_invocable" || k == "hook_ref" {
			continue
		}
		out[k] = cloneJSONValue(v)
	}

	activationType, ok := in["type"].(string)
	if !ok || activationType == "" {
		return nil, &RejectError{ID: ErrSkillActivationTypeMissing}
	}
	switch activationType {
	case "auto", "hook", "manual":
	default:
		return nil, &RejectError{ID: ErrSkillActivationTypeInvalid, Detail: activationType}
	}
	out["type"] = activationType

	if hookRef, ok := in["hook_ref"]; ok {
		if activationType != "hook" {
			return nil, &RejectError{ID: ErrSkillHookRefForbidden}
		}
		refMap, _ := hookRef.(map[string]any)
		id, _ := refMap["id"].(string)
		if refMap == nil || id == "" {
			return nil, &RejectError{ID: ErrSkillHookRefIDMissing}
		}
		out["hook_ref"] = cloneJSONValue(refMap)
	}

	if userInvocable, ok := in["user_invocable"]; ok {
		out["user_invocable"] = cloneJSONValue(userInvocable)
	} else {
		out["user_invocable"] = true
	}
	return out, nil
}

func skillDerivedCapabilities(item map[string]any) (map[string]bool, error) {
	block, classification := unwrapItemBlock(item, "skill")
	result, err := CanonicalizeSkill(block)
	if err != nil {
		return nil, err
	}
	activation, _ := result.Canonical["activation"].(map[string]any)
	activationType, _ := activation["type"].(string)
	return map[string]bool{
		"auto_invocable":           activationType == "auto",
		"disable_model_invocation": activationType == "manual" || activationType == "hook",
		"user_invocable":           activation["user_invocable"] == false,
		"skill_bundled_resources":  classification == "multi-file",
	}, nil
}
