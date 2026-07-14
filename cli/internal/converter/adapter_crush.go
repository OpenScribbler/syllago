package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	RegisterAdapter(&CrushAdapter{})
}

// CrushAdapter handles hooks for Crush (charmbracelet/crush).
//
// Crush stores hooks in its unified crush.json under the hooks key, an object
// keyed by event name. PreToolUse is the only event. Each entry is a flat
// HookConfig — {name?, matcher?, command, timeout?} — with no nested hooks
// array, and timeouts in seconds (default 30). Crush's schema declares
// additionalProperties: false, so no syllago metadata fields may be added.
// Hooks run before permission checks; exit code 2 or a JSON decision of
// "deny" blocks the tool call, so every crush hook is blocking-capable.
type CrushAdapter struct{}

func (a *CrushAdapter) ProviderSlug() string { return "crush" }

// crushHookEntry is a single HookConfig in crush.json (schema.json HookConfig).
type crushHookEntry struct {
	Name    string `json:"name,omitempty"`
	Matcher string `json:"matcher,omitempty"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // seconds
}

// crushHooksFile is the hooks section of crush.json.
type crushHooksFile struct {
	Hooks map[string][]crushHookEntry `json:"hooks"`
}

func (a *CrushAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	var warnings []ConversionWarning
	result := crushHooksFile{Hooks: make(map[string][]crushHookEntry)}

	for _, hook := range hooks.Hooks {
		// 1. Translate event (crush supports before_tool_execute only)
		nativeEvent, err := TranslateEventToProvider(hook.Event, "crush")
		if err != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: fmt.Sprintf("hook event %q not supported by crush; skipped", hook.Event),
			})
			continue
		}

		// 2. Check handler type against the degradation policy (ADR 0001)
		_, hWarnings, keep := TranslateHandlerType(hook.Handler, "crush", hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		// Crush's schema requires command; an empty one is invalid config.
		if hook.Handler.Command == "" {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: "crush hooks require a command; hook with empty command skipped",
			})
			continue
		}

		// 3. Non-blocking intent cannot be preserved: crush PreToolUse hooks
		// always carry veto power (exit code 2 blocks).
		if !hook.Blocking {
			warnings = append(warnings, ConversionWarning{
				Severity:    "info",
				Description: "crush PreToolUse hooks always have veto power (exit code 2 blocks); non-blocking intent is not preserved",
			})
		}
		if hook.Handler.Async {
			warnings = append(warnings, ConversionWarning{
				Severity:    "info",
				Description: "crush hooks are synchronous; async flag dropped",
			})
		}

		// 4. Translate matcher
		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, "crush")
			warnings = append(warnings, mWarnings...)
			s, sWarnings := crushMatcherString(translatedMatcher)
			matcherStr = s
			warnings = append(warnings, sWarnings...)
		}

		entry := crushHookEntry{
			Name:    hook.Name,
			Matcher: matcherStr,
			Command: hook.Handler.Command,
			Timeout: TranslateTimeoutToProvider(hook.Handler.Timeout, "crush"),
		}
		result.Hooks[nativeEvent] = append(result.Hooks[nativeEvent], entry)
	}

	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return &EncodedResult{
		Content:  content,
		Filename: "crush.json",
		Warnings: warnings,
	}, nil
}

// crushMatcherString renders a translated matcher as a crush regex string.
// Crush matchers are a single regex tested against the tool name, so array
// matchers join as an alternation. Shapes that can't be represented (e.g.
// nested objects) drop to match-all with a warning rather than silently.
func crushMatcherString(m json.RawMessage) (string, []ConversionWarning) {
	if len(m) == 0 {
		return "", nil
	}
	var s string
	if json.Unmarshal(m, &s) == nil {
		return s, nil
	}
	var parts []string
	if json.Unmarshal(m, &parts) == nil {
		return strings.Join(parts, "|"), nil
	}
	return "", []ConversionWarning{{
		Severity:    "warning",
		Capability:  "matcher",
		Description: "matcher shape not representable as a crush regex; hook will match all tools",
	}}
}

func (a *CrushAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	var file crushHooksFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parsing crush hooks: %w", err)
	}

	ch := &CanonicalHooks{Spec: SpecVersion}

	for nativeEvent, entries := range file.Hooks {
		canonEvent, _ := TranslateEventFromProvider(nativeEvent, "crush")

		for _, entry := range entries {
			var matcherJSON json.RawMessage
			if entry.Matcher != "" {
				rawMatcher, _ := json.Marshal(entry.Matcher)
				translatedMatcher, _ := TranslateMatcherFromProvider(rawMatcher, "crush")
				matcherJSON = translatedMatcher
			}

			ch.Hooks = append(ch.Hooks, CanonicalHook{
				Name:    entry.Name,
				Event:   canonEvent,
				Matcher: matcherJSON,
				// Crush hooks run pre-permission with veto power.
				Blocking: canonEvent == "before_tool_execute",
				Handler: HookHandler{
					Type:    "command",
					Command: entry.Command,
					Timeout: TranslateTimeoutFromProvider(entry.Timeout, "crush"),
				},
			})
		}
	}

	return ch, nil
}

func (a *CrushAdapter) Capabilities() ProviderCapabilities {
	return providerHookCapabilities[a.ProviderSlug()]
}
