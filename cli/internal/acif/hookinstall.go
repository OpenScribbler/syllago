package acif

func EvaluateInstall(item map[string]any, installTargetOS string) (map[string]any, error) {
	result, err := CanonicalizeHook(item, HookOpts{})
	if err != nil {
		return nil, err
	}
	if installTargetOS == "" {
		return map[string]any{"install": "proceed"}, nil
	}

	blocking, _ := result.Canonical["blocking"].(bool)
	var diagnostics []Diagnostic
	for _, handler := range hookHandlers(result.Canonical) {
		if handler["type"] != "command" {
			continue
		}
		if _, ok := selectScriptForOS(handler, installTargetOS); ok {
			continue
		}
		if blocking {
			return map[string]any{"install": "refuse-unless-operator-opt-in"}, nil
		}
		diagnostics = append(diagnostics, Diagnostic{ID: DiagHookScriptNoPlatformMatch})
	}

	out := map[string]any{"install": "proceed"}
	if len(diagnostics) > 0 {
		out["diagnostics"] = diagnostics
	}
	return out, nil
}

func EvaluateRequires(itemRequires map[string]any, consumerRecognizes []string) map[string]string {
	recognized := make(map[string]bool, len(consumerRecognizes))
	for _, key := range consumerRecognizes {
		recognized[key] = true
	}
	for key := range itemRequires {
		if !recognized[key] {
			return map[string]string{
				"evaluation": "unknown",
				"install":    "refuse-unless-operator-opt-in",
			}
		}
	}
	return map[string]string{
		"evaluation": "satisfied",
		"install":    "proceed",
	}
}
