package acif

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/moathash"
)

type HookOpts struct {
	Provider string
	BodyRoot string
}

type HookVerdict struct {
	Reason string         `json:"reason"`
	Params map[string]any `json:"params,omitempty"`
}

type HookResult struct {
	Canonical   map[string]any
	Diagnostics []Diagnostic
	Provenance  string
	Verdict     *HookVerdict
}

var validHookHandlerTypes = map[string]bool{
	"command": true,
	"http":    true,
	"prompt":  true,
	"agent":   true,
}

var validHookOS = map[string]bool{
	"darwin":  true,
	"linux":   true,
	"windows": true,
}

var hookOSOrder = []string{"darwin", "linux", "windows"}

func CanonicalizeHook(block map[string]any, opts HookOpts) (*HookResult, error) {
	block = unwrapHookBlock(block)
	out := make(map[string]any, len(block)+2)
	for k, v := range block {
		if isHookTopLevelCanonicalKey(k) {
			continue
		}
		out[k] = cloneJSONValue(v)
	}

	event, _ := block["event"].(string)
	canonicalEvent, ok := canonicalHookEvent(event, opts.Provider)
	if !ok {
		return nil, hookReject(ErrHookEventUnrecognized, event)
	}
	out["event"] = canonicalEvent

	if matcher, ok := block["matcher"]; ok {
		if s, ok := matcher.(string); ok {
			if s != "" {
				if opts.Provider != "" {
					s = converter.ReverseTranslateMatcher(s, opts.Provider)
				}
				out["matcher"] = s
			}
		} else {
			out["matcher"] = cloneJSONValue(matcher)
		}
	}

	rawHandlers, ok := block["handlers"].([]any)
	if !ok || len(rawHandlers) == 0 {
		return nil, hookReject(ErrHookHandlersMissing, "")
	}
	handlers := make([]any, 0, len(rawHandlers))
	result := &HookResult{Canonical: out, Provenance: "declared"}
	for _, rawHandler := range rawHandlers {
		handler, ok := rawHandler.(map[string]any)
		if !ok {
			return nil, hookReject(ErrHookHandlersMissing, "")
		}
		canonicalHandler, verdict, err := canonicalizeHookHandler(handler)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, canonicalHandler)
		if verdict != nil && result.Verdict == nil {
			result.Verdict = verdict
		}
	}
	out["handlers"] = handlers

	if aux, ok := block["auxiliary_files"]; ok {
		canonicalAux, err := canonicalizeAuxiliaryFiles(aux)
		if err != nil {
			return nil, err
		}
		if len(canonicalAux) > 0 {
			out["auxiliary_files"] = canonicalAux
		}
	}

	if blocking, ok := block["blocking"]; ok {
		out["blocking"] = cloneJSONValue(blocking)
	} else {
		out["blocking"] = false
	}

	if requires, ok := block["requires"]; ok {
		if reqMap, ok := requires.(map[string]any); ok {
			if len(reqMap) > 0 {
				out["requires"] = cloneJSONValue(reqMap)
				result.Verdict = orphanKeyVerdict(reqMap)
			}
		} else {
			out["requires"] = cloneJSONValue(requires)
			result.Verdict = &HookVerdict{Reason: ReasonRequiresOrphanKey}
		}
	}

	return result, nil
}

func canonicalHookEvent(event, provider string) (string, bool) {
	if !converter.IsValidHookEvent(event) {
		return "", false
	}
	if _, ok := converter.HookEvents[event]; ok {
		return event, true
	}
	if provider != "" {
		translated := converter.ReverseTranslateHookEvent(event, provider)
		if _, ok := converter.HookEvents[translated]; ok {
			return translated, true
		}
	}
	matches := make([]string, 0, 2)
	for canonical, providerMap := range converter.HookEvents {
		for _, native := range providerMap {
			if native == event {
				matches = append(matches, canonical)
				break
			}
		}
	}
	if len(matches) == 0 {
		return "", false
	}
	for _, match := range matches {
		if match == "error_occurred" {
			return match, true
		}
	}
	sort.Strings(matches)
	return matches[0], true
}

func canonicalizeHookHandler(handler map[string]any) (map[string]any, *HookVerdict, error) {
	out := make(map[string]any, len(handler)+2)
	for k, v := range handler {
		if k == "type" || k == "scripts" || k == "async" {
			continue
		}
		out[k] = cloneJSONValue(v)
	}

	rawHandlerType, hasHandlerType := handler["type"]
	handlerType := "command"
	if hasHandlerType {
		typedHandlerType, ok := rawHandlerType.(string)
		if !ok {
			return nil, nil, hookReject(ErrHookHandlerTypeUnrecognized, "")
		}
		handlerType = typedHandlerType
	}
	if handlerType == "" {
		handlerType = "command"
	}
	if !validHookHandlerTypes[handlerType] {
		return nil, nil, hookReject(ErrHookHandlerTypeUnrecognized, handlerType)
	}
	out["type"] = handlerType

	if handlerType != "command" {
		if scripts, ok := handler["scripts"]; ok {
			out["scripts"] = cloneJSONValue(scripts)
		}
		if async, ok := handler["async"]; ok {
			out["async"] = cloneJSONValue(async)
		}
		return out, nil, nil
	}

	if async, ok := handler["async"]; ok {
		out["async"] = cloneJSONValue(async)
	} else {
		out["async"] = false
	}

	rawScripts, ok := handler["scripts"].([]any)
	if !ok || len(rawScripts) == 0 {
		return out, &HookVerdict{Reason: ReasonCommandHandlerScriptsMissing}, nil
	}
	scripts, err := canonicalizeHookScripts(rawScripts)
	if err != nil {
		return nil, nil, err
	}
	out["scripts"] = scripts
	return out, nil, nil
}

func canonicalizeHookScripts(rawScripts []any) ([]any, error) {
	processed := make([]map[string]any, 0, len(rawScripts))
	for _, rawScript := range rawScripts {
		script, ok := rawScript.(map[string]any)
		if !ok {
			return nil, hookReject(ErrHookScriptPathInvalid, "")
		}
		canonical, err := canonicalizeHookScript(script)
		if err != nil {
			return nil, err
		}
		processed = append(processed, canonical)
	}

	if err := checkScriptDisjointness(processed); err != nil {
		return nil, err
	}

	sort.SliceStable(processed, func(i, j int) bool {
		return bytes.Compare(scriptSortKey(processed[i]), scriptSortKey(processed[j])) < 0
	})

	out := make([]any, len(processed))
	for i := range processed {
		out[i] = processed[i]
	}
	return out, nil
}

func canonicalizeHookScript(script map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(script))
	for k, v := range script {
		switch k {
		case "type", "path", "content", "os", "arch":
			continue
		default:
			out[k] = cloneJSONValue(v)
		}
	}

	scriptType, _ := script["type"].(string)
	switch scriptType {
	case "file":
		path, ok := script["path"].(string)
		if !ok || !isValidHookPath(path) {
			return nil, hookReject(ErrHookScriptPathInvalid, path)
		}
		out["path"] = path
	case "inline":
		content, ok := script["content"].(string)
		if !ok {
			return nil, hookReject(ErrHookScriptPathInvalid, "")
		}
		out["content"] = string(moathash.CanonicalText([]byte(content)))
	default:
		return nil, hookReject(ErrHookScriptPathInvalid, scriptType)
	}
	out["type"] = scriptType

	if rawOS, ok := script["os"]; ok {
		osTags, err := canonicalStringArray(rawOS, true, validHookOS, ErrHookScriptOSEmpty, ErrHookScriptOSInvalid)
		if err != nil {
			return nil, err
		}
		out["os"] = stringsToAnySlice(osTags)
	}

	if rawArch, ok := script["arch"]; ok {
		archTags, err := canonicalStringArray(rawArch, true, nil, ErrHookScriptArchEmpty, "")
		if err != nil {
			return nil, err
		}
		out["arch"] = stringsToAnySlice(archTags)
	}

	return out, nil
}

func canonicalStringArray(raw any, rejectEmpty bool, allowed map[string]bool, emptyID, invalidID string) ([]string, error) {
	rawItems, ok := raw.([]any)
	if !ok {
		if invalidID != "" {
			return nil, hookReject(invalidID, "")
		}
		return nil, hookReject(emptyID, "")
	}
	if rejectEmpty && len(rawItems) == 0 {
		return nil, hookReject(emptyID, "")
	}
	seen := make(map[string]bool, len(rawItems))
	items := make([]string, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(string)
		if !ok {
			if invalidID != "" {
				return nil, hookReject(invalidID, "")
			}
			return nil, hookReject(emptyID, "")
		}
		if allowed != nil && !allowed[item] {
			return nil, hookReject(invalidID, item)
		}
		if !seen[item] {
			seen[item] = true
			items = append(items, item)
		}
	}
	sort.Strings(items)
	return items, nil
}

func checkScriptDisjointness(scripts []map[string]any) error {
	defaultEntries := 0
	collisions := make([]Diagnostic, 0)
	for _, script := range scripts {
		if _, ok := script["os"]; !ok {
			defaultEntries++
		}
	}
	if defaultEntries > 1 {
		return hookReject(ErrHookScriptDefaultAmbiguous, "")
	}

	for _, osTag := range hookOSOrder {
		indices := make([]any, 0, 2)
		for i, script := range scripts {
			for _, declared := range anyStringSlice(script["os"]) {
				if declared == osTag {
					indices = append(indices, i)
					break
				}
			}
		}
		if len(indices) >= 2 {
			collisions = append(collisions, Diagnostic{
				ID:     ErrHookScriptPlatformAmbiguous,
				Params: map[string]any{"os": osTag, "entries": indices},
			})
		}
	}
	if len(collisions) > 0 {
		return hookRejectWithDiagnostics(ErrHookScriptPlatformAmbiguous, "", collisions)
	}
	return nil
}

func canonicalizeAuxiliaryFiles(raw any) ([]any, error) {
	rawItems, ok := raw.([]any)
	if !ok {
		return nil, nil
	}
	byPath := make(map[string]map[string]any, len(rawItems))
	paths := make([]string, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			return nil, hookReject(ErrHookScriptPathInvalid, "")
		}
		path, ok := item["path"].(string)
		if !ok || !isValidHookPath(path) {
			return nil, hookReject(ErrHookScriptPathInvalid, path)
		}
		if _, seen := byPath[path]; !seen {
			byPath[path] = cloneJSONValue(item).(map[string]any)
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	out := make([]any, 0, len(paths))
	for _, path := range paths {
		out = append(out, byPath[path])
	}
	return out, nil
}

func scriptSortKey(script map[string]any) []byte {
	raw, err := json.Marshal(script)
	if err != nil {
		return nil
	}
	canonical, err := CanonicalJSON(raw)
	if err != nil {
		return raw
	}
	return canonical
}

func isValidHookPath(path string) bool {
	if path == "" {
		return false
	}
	if strings.Contains(path, `\`) || strings.HasPrefix(path, "/") {
		return false
	}
	if len(path) >= 2 && path[1] == ':' && isASCIIAlpha(path[0]) {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func isASCIIAlpha(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func unwrapHookBlock(block map[string]any) map[string]any {
	if block == nil {
		return map[string]any{}
	}
	if hook, ok := block["hook"].(map[string]any); ok {
		return hook
	}
	return block
}

func isHookTopLevelCanonicalKey(key string) bool {
	switch key {
	case "event", "matcher", "handlers", "auxiliary_files", "blocking", "requires":
		return true
	default:
		return false
	}
}

func stringsToAnySlice(items []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

func anyStringSlice(raw any) []string {
	switch items := raw.(type) {
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return append([]string(nil), items...)
	default:
		return nil
	}
}

func cloneJSONValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = cloneJSONValue(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = cloneJSONValue(v)
		}
		return out
	default:
		return x
	}
}

func hookReject(id, detail string) *RejectError {
	return &RejectError{ID: id, Detail: detail}
}

func hookRejectWithDiagnostics(id, detail string, diagnostics []Diagnostic) *RejectError {
	return &RejectError{ID: id, Detail: detail, Diagnostics: diagnostics}
}
