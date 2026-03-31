package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterAdapter(&VSCodeCopilotAdapter{})
}

// VSCodeCopilotAdapter handles hooks for VS Code Copilot.
// Uses the same JSON schema as Claude Code (event → matcher groups → hook entries)
// but with a different set of supported events and no LLM/HTTP hook types.
type VSCodeCopilotAdapter struct{}

func (a *VSCodeCopilotAdapter) ProviderSlug() string { return "vs-code-copilot" }

func (a *VSCodeCopilotAdapter) FieldsToVerify() []string {
	return []string{VerifyFieldEvent, VerifyFieldName, VerifyFieldMatcher}
}

// VS Code Copilot uses the same JSON structure as CC — reuse the CC struct types
// with a type alias for clarity.
type vscHookEntry = ccHookEntry
type vscMatcherGroup = ccMatcherGroup
type vscHooksFile = ccHooksFile

func (a *VSCodeCopilotAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	const slug = "vs-code-copilot"
	var warnings []ConversionWarning

	groups := map[groupKey]*vscMatcherGroup{}
	var order []groupKey

	for _, hook := range hooks.Hooks {
		// 1. Translate event
		nativeEvent, err := TranslateEventToProvider(hook.Event, slug)
		if err != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: fmt.Sprintf("hook event %q not supported by %s; skipped", hook.Event, slug),
			})
			continue
		}

		// 2. Check handler type — VS Code Copilot only supports command hooks
		handler, hWarnings, keep := TranslateHandlerType(hook.Handler, slug, hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		// 3. Translate matcher
		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, slug)
			warnings = append(warnings, mWarnings...)
			if translatedMatcher != nil {
				_ = json.Unmarshal(translatedMatcher, &matcherStr)
			}
		}

		// 4. Translate timeout (canonical seconds -> milliseconds)
		timeoutMs := TranslateTimeoutToProvider(handler.Timeout, slug)

		// 5. Build entry
		entry := vscHookEntry{
			Type:          handler.Type,
			Command:       handler.Command,
			TimeoutMs:     timeoutMs,
			StatusMessage: handler.StatusMessage,
			Async:         handler.Async,
			TimeoutAction: handler.TimeoutAction,
			CWD:           handler.CWD,
			Env:           handler.Env,
			Platform:      handler.Platform,
			Name:          hook.Name,
			Blocking:      hook.Blocking,
			Degradation:   hook.Degradation,
			Capabilities:  hook.Capabilities,
		}

		// 6. Group by event+matcher
		k := groupKey{event: nativeEvent, matcher: matcherStr}
		if g, exists := groups[k]; exists {
			g.Hooks = append(g.Hooks, entry)
		} else {
			g := &vscMatcherGroup{Matcher: matcherStr, Hooks: []vscHookEntry{entry}}
			groups[k] = g
			order = append(order, k)
		}
	}

	result := vscHooksFile{Hooks: make(map[string][]vscMatcherGroup)}
	for _, k := range order {
		result.Hooks[k.event] = append(result.Hooks[k.event], *groups[k])
	}

	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return &EncodedResult{
		Content:  content,
		Filename: "hooks.json",
		Warnings: warnings,
	}, nil
}

func (a *VSCodeCopilotAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	const slug = "vs-code-copilot"
	var file vscHooksFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parsing %s hooks: %w", slug, err)
	}

	ch := &CanonicalHooks{Spec: SpecVersion}

	for nativeEvent, groups := range file.Hooks {
		canonEvent, _ := TranslateEventFromProvider(nativeEvent, slug)

		for _, group := range groups {
			var matcherJSON json.RawMessage
			if group.Matcher != "" {
				rawMatcher, _ := json.Marshal(group.Matcher)
				translatedMatcher, _ := TranslateMatcherFromProvider(rawMatcher, slug)
				matcherJSON = translatedMatcher
			}

			for _, entry := range group.Hooks {
				timeoutSec := TranslateTimeoutFromProvider(entry.TimeoutMs, slug)

				hType := entry.Type
				if hType == "" {
					hType = "command"
				}

				hook := CanonicalHook{
					Name:    entry.Name,
					Event:   canonEvent,
					Matcher: matcherJSON,
					Handler: HookHandler{
						Type:          hType,
						Command:       entry.Command,
						Timeout:       timeoutSec,
						StatusMessage: entry.StatusMessage,
						Async:         entry.Async,
						TimeoutAction: entry.TimeoutAction,
						CWD:           entry.CWD,
						Env:           entry.Env,
						Platform:      entry.Platform,
					},
					Blocking:     entry.Blocking,
					Degradation:  entry.Degradation,
					Capabilities: entry.Capabilities,
				}
				ch.Hooks = append(ch.Hooks, hook)
			}
		}
	}

	return ch, nil
}

func (a *VSCodeCopilotAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start", "session_end", "before_compact",
			"notification", "subagent_start", "subagent_stop",
			"error_occurred", "tool_use_failure",
		},
		SupportsMatchers:         true,
		SupportsAsync:            true,
		SupportsStatusMessage:    true,
		SupportsStructuredOutput: true,
		SupportsBlocking:         true,
		TimeoutUnit:              "milliseconds",
		SupportsPlatform:         true,
		SupportsCWD:              true,
		SupportsEnv:              true,
		SupportsLLMHooks:         false,
		SupportsHTTPHooks:        false,
	}
}
