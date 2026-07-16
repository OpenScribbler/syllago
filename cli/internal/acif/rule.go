package acif

type RuleRenderTarget string

func CanonicalizeRule(block map[string]any) (*ItemResult, error) {
	block = unwrapKindBlock(block, "rule")
	out := make(map[string]any, len(block)+1)
	for k, v := range block {
		if k == "activation" || k == "requires" {
			continue
		}
		out[k] = cloneJSONValue(v)
	}

	activation, err := canonicalizeRuleActivation(block["activation"])
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

func canonicalizeRuleActivation(raw any) (map[string]any, error) {
	if raw == nil {
		return map[string]any{"mode": "always"}, nil
	}
	in, _ := raw.(map[string]any)
	if in == nil {
		return nil, &RejectError{ID: ErrRuleActivationModeMissing}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if k == "mode" || k == "globs" {
			continue
		}
		out[k] = cloneJSONValue(v)
	}

	mode, ok := in["mode"].(string)
	if !ok || mode == "" {
		return nil, &RejectError{ID: ErrRuleActivationModeMissing}
	}
	switch mode {
	case "always", "glob", "manual", "model_decision":
	default:
		return nil, &RejectError{ID: ErrRuleActivationModeInvalid, Detail: mode}
	}

	globs, hasGlobs := in["globs"]
	if mode == "glob" {
		items, ok := globs.([]any)
		if !hasGlobs || !ok || len(items) == 0 {
			return nil, &RejectError{ID: ErrRuleGlobModeWithoutGlobs}
		}
		out["globs"] = cloneJSONValue(items)
	} else if hasGlobs && globs != nil {
		return nil, &RejectError{ID: ErrRuleGlobsWithoutGlobMode}
	}

	out["mode"] = mode
	return out, nil
}

func CanonicalizeRuleProviderConfig(config map[string]any) (*ItemResult, error) {
	if provider, _ := config["provider"].(string); provider != "rule-activation-source" {
		return nil, &RejectError{ID: ErrRuleActivationModeUnmappable}
	}
	// [ACIF-RULE] §10 envelope rule: a modeless provider configuration is a
	// present activation declaration without a mode claim — never the
	// totality net.
	content, ok := config["content"].(map[string]any)
	if !ok {
		return nil, &RejectError{ID: ErrRuleActivationModeMissing, Detail: "provider_config content must be an object carrying source_sub_mode"}
	}
	mode, ok := content["source_sub_mode"].(string)
	if !ok || mode == "" {
		return nil, &RejectError{ID: ErrRuleActivationModeMissing, Detail: "content carries no source_sub_mode string; declare the source-mechanism token"}
	}
	activation := map[string]any{}
	switch mode {
	case "always_on":
		activation["mode"] = "always"
	case "glob", "frontmatter_globs":
		activation["mode"] = "glob"
		if globs, ok := content["globs"]; ok {
			activation["globs"] = cloneJSONValue(globs)
		}
	case "model_decision":
		activation["mode"] = "model_decision"
	case "manual", "slash_command":
		activation["mode"] = "manual"
	case "legacy":
		globs, hasGlobs := content["globs"].([]any)
		alwaysApply, _ := content["always_apply"].(bool)
		switch {
		case alwaysApply:
			activation["mode"] = "always"
		case hasGlobs && len(globs) > 0:
			activation["mode"] = "glob"
			activation["globs"] = cloneJSONValue(globs)
		default:
			activation["mode"] = "model_decision"
		}
	default:
		return nil, &RejectError{ID: ErrRuleActivationModeUnmappable, Detail: mode}
	}
	return CanonicalizeRule(map[string]any{"activation": activation})
}

func ruleDerivedCapabilities(item map[string]any) (map[string]bool, error) {
	block, _ := unwrapItemBlock(item, "rule")
	result, err := CanonicalizeRule(block)
	if err != nil {
		return nil, err
	}
	activation, _ := result.Canonical["activation"].(map[string]any)
	mode, _ := activation["mode"].(string)
	return map[string]bool{
		"activation_mode": mode == "glob" || mode == "manual" || mode == "model_decision",
	}, nil
}

func RuleActivationProjection(item map[string]any) (map[string]any, error) {
	block, _ := unwrapItemBlock(item, "rule")
	result, err := CanonicalizeRule(block)
	if err != nil {
		return nil, err
	}
	activation, _ := result.Canonical["activation"].(map[string]any)
	mode, _ := activation["mode"].(string)
	globs := anyStringSlice(activation["globs"])
	sample := globs
	truncated := false
	if len(sample) > 16 {
		sample = sample[:16]
		truncated = true
	}
	return map[string]any{
		"derivable": mode == "glob" || mode == "manual" || mode == "model_decision",
		"mode":      mode,
		"globs": map[string]any{
			"present":   mode == "glob" && len(globs) > 0,
			"count":     len(globs),
			"sample":    stringsToAnySlice(sample),
			"truncated": truncated,
		},
	}, nil
}

func RenderRule(item map[string]any, target string) (*RenderResult, error) {
	block, _ := unwrapItemBlock(item, "rule")
	result, err := CanonicalizeRule(block)
	if err != nil {
		return nil, err
	}
	body, _ := result.Canonical["body"].(string)
	activation, _ := result.Canonical["activation"].(map[string]any)
	mode, _ := activation["mode"].(string)
	switch target {
	case "native-rule-format":
		return &RenderResult{Output: body}, nil
	case "no-declaration-surface-provider":
		output := body
		if output == "" {
			output = "<!-- rule-body -->\n"
		}
		var diagnostics []Diagnostic
		if mode != "always" {
			diagnostics = append(diagnostics, Diagnostic{
				ID: ErrRuleActivationDegraded,
				Params: map[string]any{
					"mode_lost":          mode,
					"effective_behavior": "always-on",
				},
			})
		}
		return &RenderResult{Output: output, Diagnostics: diagnostics}, nil
	default:
		return &RenderResult{Unsupported: true}, nil
	}
}
