package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterAdapter(&DevinAdapter{})
}

// DevinAdapter handles hooks for Devin Desktop (Devin Local) and Devin CLI,
// which share one hook harness. The format is Claude Code-shaped (event →
// matcher groups → hook entries) with Devin's own tool names (exec, read,
// write, edit) and timeouts in seconds. Entries carry only the three fields
// Devin documents — type, command, timeout — so no unknown keys reach its
// config.
type DevinAdapter struct{}

func (a *DevinAdapter) ProviderSlug() string { return "devin" }

func (a *DevinAdapter) FieldsToVerify() []string {
	return []string{VerifyFieldEvent, VerifyFieldMatcher}
}

type devinHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // seconds
}

type devinMatcherGroup struct {
	Matcher string           `json:"matcher"`
	Hooks   []devinHookEntry `json:"hooks"`
}

type devinHooksFile struct {
	Hooks map[string][]devinMatcherGroup `json:"hooks"`
}

func (a *DevinAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	const slug = "devin"
	var warnings []ConversionWarning

	groups := map[groupKey]*devinMatcherGroup{}
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

		if handler.CWD != "" || len(handler.Env) > 0 || len(handler.Platform) > 0 || handler.Async || handler.StatusMessage != "" {
			warnings = append(warnings, ConversionWarning{
				Severity:    "info",
				Description: "devin hooks support only type, command, and timeout; cwd, env, platform, async, and statusMessage dropped",
			})
		}

		if hook.Event == "before_tool_execute" && !hook.Blocking {
			warnings = append(warnings, ConversionWarning{
				Severity:    "info",
				Description: "devin PreToolUse hooks always have veto power (exit code 2 blocks); non-blocking intent is not preserved",
			})
		}

		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, slug)
			warnings = append(warnings, mWarnings...)
			if translatedMatcher != nil {
				_ = json.Unmarshal(translatedMatcher, &matcherStr)
			}
		}

		entry := devinHookEntry{
			Type:    "command",
			Command: handler.Command,
			Timeout: TranslateTimeoutToProvider(handler.Timeout, slug),
		}

		k := groupKey{event: nativeEvent, matcher: matcherStr}
		if g, exists := groups[k]; exists {
			g.Hooks = append(g.Hooks, entry)
		} else {
			groups[k] = &devinMatcherGroup{Matcher: matcherStr, Hooks: []devinHookEntry{entry}}
			order = append(order, k)
		}
	}

	result := devinHooksFile{Hooks: make(map[string][]devinMatcherGroup)}
	for _, k := range order {
		result.Hooks[k.event] = append(result.Hooks[k.event], *groups[k])
	}

	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return &EncodedResult{
		Content:  content,
		Filename: "config.json",
		Warnings: warnings,
	}, nil
}

func (a *DevinAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	const slug = "devin"
	var file devinHooksFile
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
				matcherJSON, _ = TranslateMatcherFromProvider(rawMatcher, slug)
			}

			for _, entry := range group.Hooks {
				hType := entry.Type
				if hType == "" {
					hType = "command"
				}
				ch.Hooks = append(ch.Hooks, CanonicalHook{
					Event:   canonEvent,
					Matcher: matcherJSON,
					Handler: HookHandler{
						Type:    hType,
						Command: entry.Command,
						Timeout: TranslateTimeoutFromProvider(entry.Timeout, slug),
					},
					// Devin blocks on exit code 2 from any tool hook, so a
					// pre-tool hook is blocking.
					Blocking: canonEvent == "before_tool_execute",
				})
			}
		}
	}

	return ch, nil
}

func (a *DevinAdapter) Capabilities() ProviderCapabilities {
	return providerHookCapabilities[a.ProviderSlug()]
}
