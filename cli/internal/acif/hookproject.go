package acif

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

func ScriptSelection(block map[string]any, targets []string) (map[string]string, []Diagnostic, error) {
	result, err := CanonicalizeHook(block, HookOpts{})
	if err != nil {
		return nil, nil, err
	}
	handler := firstCommandHandler(result.Canonical)
	selection := make(map[string]string, len(targets))
	missed := make([]any, 0)
	for _, target := range targets {
		script, ok := selectScriptForOS(handler, target)
		if !ok {
			selection[target] = "none"
			missed = append(missed, target)
			continue
		}
		if script["type"] == "inline" {
			selection[target] = "inline"
		} else if path, ok := script["path"].(string); ok {
			selection[target] = path
		} else {
			selection[target] = "none"
			missed = append(missed, target)
		}
	}
	var diagnostics []Diagnostic
	if len(missed) > 0 {
		diagnostics = append(diagnostics, Diagnostic{
			ID:     DiagHookScriptNoPlatformMatch,
			Params: map[string]any{"targets": missed},
		})
	}
	return selection, diagnostics, nil
}

func DerivedCapabilities(block map[string]any) (map[string]bool, error) {
	result, err := CanonicalizeHook(block, HookOpts{})
	if err != nil {
		return nil, err
	}
	caps := map[string]bool{
		"handler_types":    false,
		"matcher_patterns": false,
		"async_execution":  false,
	}
	if _, ok := result.Canonical["matcher"]; ok {
		caps["matcher_patterns"] = true
	}
	for _, handler := range hookHandlers(result.Canonical) {
		if typ, ok := handler["type"].(string); ok && typ != "" {
			caps["handler_types"] = true
		}
		if async, ok := handler["async"].(bool); ok && async {
			caps["async_execution"] = true
		}
	}
	return caps, nil
}

func OSCoverage(block map[string]any) (map[string]any, error) {
	result, err := CanonicalizeHook(block, HookOpts{})
	if err != nil {
		return nil, err
	}
	osSet := make(map[string]bool)
	archSet := make(map[string]bool)
	derivable := false
	unconstrained := false
	divergent := false
	for _, handler := range hookHandlers(result.Canonical) {
		if handler["type"] != "command" {
			continue
		}
		for _, script := range hookScripts(handler) {
			osTags := anyStringSlice(script["os"])
			if len(osTags) == 0 {
				unconstrained = true
			} else {
				derivable = true
				for _, osTag := range osTags {
					osSet[osTag] = true
				}
			}
			for _, arch := range anyStringSlice(script["arch"]) {
				archSet[arch] = true
			}
		}
		if commandHandlerDivergent(handler) {
			divergent = true
		}
	}
	return map[string]any{
		"derivable":     derivable,
		"os":            stringsFromSet(osSet),
		"arch":          stringsFromSet(archSet),
		"unconstrained": unconstrained,
		"os_divergent":  divergent,
		"provenance":    "declared",
	}, nil
}

func firstCommandHandler(block map[string]any) map[string]any {
	for _, handler := range hookHandlers(block) {
		if handler["type"] == "command" {
			return handler
		}
	}
	return nil
}

func selectScriptForOS(handler map[string]any, osTag string) (map[string]any, bool) {
	if handler == nil {
		return nil, false
	}
	var defaultScript map[string]any
	for _, script := range hookScripts(handler) {
		osTags := anyStringSlice(script["os"])
		if len(osTags) == 0 {
			defaultScript = script
			continue
		}
		for _, declared := range osTags {
			if declared == osTag {
				return script, true
			}
		}
	}
	if defaultScript != nil {
		return defaultScript, true
	}
	return nil, false
}

func commandHandlerDivergent(handler map[string]any) bool {
	seen := make(map[string]string)
	for _, osTag := range hookOSOrder {
		script, ok := selectScriptForOS(handler, osTag)
		if !ok {
			continue
		}
		seen[osTag] = executableIdentity(script)
	}
	identities := make(map[string]bool)
	for _, identity := range seen {
		if identity != "" {
			identities[identity] = true
		}
	}
	return len(identities) >= 2
}

func executableIdentity(script map[string]any) string {
	typ, _ := script["type"].(string)
	switch typ {
	case "file":
		path, _ := script["path"].(string)
		return typ + "\x00" + path
	case "inline":
		content, _ := script["content"].(string)
		sum := sha256.Sum256(moat.CanonicalText([]byte(content)))
		return typ + "\x00" + hex.EncodeToString(sum[:])
	default:
		return typ
	}
}

func stringsFromSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for item := range set {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
