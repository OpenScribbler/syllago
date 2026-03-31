package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterAdapter(&FactoryDroidAdapter{})
}

// FactoryDroidAdapter handles hooks for Factory Droid.
// Uses the same JSON schema as Claude Code (event → matcher groups → hook entries)
// with 9 supported events and different tool names (Execute, Create).
type FactoryDroidAdapter struct{}

func (a *FactoryDroidAdapter) ProviderSlug() string { return "factory-droid" }

func (a *FactoryDroidAdapter) FieldsToVerify() []string {
	return []string{VerifyFieldEvent, VerifyFieldName, VerifyFieldMatcher}
}

// Factory Droid uses the same JSON structure as CC — reuse struct types.
type fdHookEntry = ccHookEntry
type fdMatcherGroup = ccMatcherGroup
type fdHooksFile = ccHooksFile

func (a *FactoryDroidAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	const slug = "factory-droid"
	var warnings []ConversionWarning

	groups := map[groupKey]*fdMatcherGroup{}
	var order []groupKey

	for _, hook := range hooks.Hooks {
		nativeEvent, err := TranslateEventToProvider(hook.Event, slug)
		if err != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: fmt.Sprintf("hook event %q not supported by %s; skipped", hook.Event, slug),
			})
			continue
		}

		handler, hWarnings, keep := TranslateHandlerType(hook.Handler, slug, hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, slug)
			warnings = append(warnings, mWarnings...)
			if translatedMatcher != nil {
				_ = json.Unmarshal(translatedMatcher, &matcherStr)
			}
		}

		timeoutMs := TranslateTimeoutToProvider(handler.Timeout, slug)

		entry := fdHookEntry{
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

		k := groupKey{event: nativeEvent, matcher: matcherStr}
		if g, exists := groups[k]; exists {
			g.Hooks = append(g.Hooks, entry)
		} else {
			g := &fdMatcherGroup{Matcher: matcherStr, Hooks: []fdHookEntry{entry}}
			groups[k] = g
			order = append(order, k)
		}
	}

	result := fdHooksFile{Hooks: make(map[string][]fdMatcherGroup)}
	for _, k := range order {
		result.Hooks[k.event] = append(result.Hooks[k.event], *groups[k])
	}

	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return &EncodedResult{
		Content:  content,
		Filename: "settings.json",
		Warnings: warnings,
	}, nil
}

func (a *FactoryDroidAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	const slug = "factory-droid"
	var file fdHooksFile
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

func (a *FactoryDroidAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start", "session_end", "before_compact",
			"subagent_start", "subagent_stop",
		},
		SupportsMatchers:      true,
		SupportsStatusMessage: true,
		SupportsBlocking:      true,
		TimeoutUnit:           "milliseconds",
		SupportsCWD:           true,
		SupportsEnv:           true,
	}
}
