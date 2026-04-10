package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterAdapter(&CursorAdapter{})
}

// CursorAdapter handles hooks for Cursor.
type CursorAdapter struct{}

func (a *CursorAdapter) ProviderSlug() string { return "cursor" }

// --- Cursor provider-native structs ---

// cursorHookEntry is a single hook in Cursor's native format.
// Cursor uses millisecond timeouts and camelCase status_message.
type cursorHookEntry struct {
	Type          string `json:"type,omitempty"`
	Command       string `json:"command,omitempty"`
	TimeoutMs     int    `json:"timeout,omitempty"`
	StatusMessage string `json:"statusMessage,omitempty"`
	// Metadata fields for round-trip fidelity
	Name         string            `json:"name,omitempty"`
	Blocking     bool              `json:"blocking,omitempty"`
	Degradation  map[string]string `json:"degradation,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
}

// cursorMatcherGroup is a matcher group in Cursor's hook format.
// Cursor supports failClosed (alias for blocking) and loop_limit at group level.
type cursorMatcherGroup struct {
	Matcher    string            `json:"matcher,omitempty"`
	Hooks      []cursorHookEntry `json:"hooks"`
	FailClosed *bool             `json:"failClosed,omitempty"`
	LoopLimit  *int              `json:"loop_limit,omitempty"`
}

// cursorHooksFile is the top-level Cursor hooks config.
type cursorHooksFile struct {
	Version int                             `json:"version"`
	Hooks   map[string][]cursorMatcherGroup `json:"hooks"`
}

func (a *CursorAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	var warnings []ConversionWarning

	// Keyed grouping: same event+matcher go in the same group
	type gKey struct{ event, matcher string }
	groups := map[gKey]*cursorMatcherGroup{}
	var order []gKey

	for _, hook := range hooks.Hooks {
		// 1. Translate event
		nativeEvent, err := TranslateEventToProvider(hook.Event, "cursor")
		if err != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: fmt.Sprintf("hook event %q not supported by cursor; skipped", hook.Event),
			})
			continue
		}

		// 2. Check handler type (Cursor only supports command hooks)
		_, hWarnings, keep := TranslateHandlerType(hook.Handler, "cursor", hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		// 3. Translate matcher
		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, "cursor")
			warnings = append(warnings, mWarnings...)
			if translatedMatcher != nil {
				_ = json.Unmarshal(translatedMatcher, &matcherStr)
			}
		}

		// 4. Translate timeout (canonical seconds -> Cursor milliseconds)
		timeoutMs := TranslateTimeoutToProvider(hook.Handler.Timeout, "cursor")

		// 5. Build entry
		entry := cursorHookEntry{
			Type:          hook.Handler.Type,
			Command:       hook.Handler.Command,
			TimeoutMs:     timeoutMs,
			StatusMessage: hook.Handler.StatusMessage,
			Name:          hook.Name,
			Blocking:      hook.Blocking,
			Degradation:   hook.Degradation,
			Capabilities:  hook.Capabilities,
		}
		if entry.Type == "" {
			entry.Type = "command"
		}

		// 6. Group by event+matcher
		k := gKey{event: nativeEvent, matcher: matcherStr}
		if g, exists := groups[k]; exists {
			g.Hooks = append(g.Hooks, entry)
		} else {
			g := &cursorMatcherGroup{Matcher: matcherStr, Hooks: []cursorHookEntry{entry}}
			// Set failClosed from hook.Blocking
			if hook.Blocking {
				b := true
				g.FailClosed = &b
			}
			// Merge provider_data["cursor"] fields back into group
			if hook.ProviderData != nil {
				if cursorData, ok := hook.ProviderData["cursor"].(map[string]any); ok {
					if ll, ok := cursorData["loop_limit"]; ok {
						switch v := ll.(type) {
						case float64:
							i := int(v)
							g.LoopLimit = &i
						case int:
							g.LoopLimit = &v
						}
					}
					if fc, ok := cursorData["failClosed"].(bool); ok {
						g.FailClosed = &fc
					}
				}
			}
			groups[k] = g
			order = append(order, k)
		}
	}

	// Build result from ordered groups
	result := cursorHooksFile{
		Version: 1,
		Hooks:   make(map[string][]cursorMatcherGroup),
	}
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

func (a *CursorAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	var file cursorHooksFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parsing cursor hooks: %w", err)
	}

	ch := &CanonicalHooks{Spec: SpecVersion}

	for nativeEvent, groups := range file.Hooks {
		// Translate event (decode path: lenient)
		canonEvent, _ := TranslateEventFromProvider(nativeEvent, "cursor")

		for _, group := range groups {
			// Translate matcher
			var matcherJSON json.RawMessage
			if group.Matcher != "" {
				rawMatcher, _ := json.Marshal(group.Matcher)
				translatedMatcher, _ := TranslateMatcherFromProvider(rawMatcher, "cursor")
				matcherJSON = translatedMatcher
			}

			// Build provider_data for Cursor-specific fields
			var providerData map[string]any
			if group.FailClosed != nil || group.LoopLimit != nil {
				pd := map[string]any{}
				if group.FailClosed != nil {
					pd["failClosed"] = *group.FailClosed
				}
				if group.LoopLimit != nil {
					pd["loop_limit"] = *group.LoopLimit
				}
				providerData = map[string]any{"cursor": pd}
			}

			for _, entry := range group.Hooks {
				// Translate timeout (Cursor milliseconds -> canonical seconds)
				timeoutSec := TranslateTimeoutFromProvider(entry.TimeoutMs, "cursor")

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
					},
					Blocking:     entry.Blocking,
					Degradation:  entry.Degradation,
					Capabilities: entry.Capabilities,
					ProviderData: providerData,
				}
				// Also set Blocking from group-level failClosed
				if group.FailClosed != nil && *group.FailClosed {
					hook.Blocking = true
				}
				ch.Hooks = append(ch.Hooks, hook)
			}
		}
	}

	return ch, nil
}

func (a *CursorAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start", "session_end", "before_compact",
			"tool_use_failure", "subagent_start", "subagent_stop",
			"file_changed", "before_model", "after_model", "before_tool_selection",
		},
		SupportsMatchers:         true,
		SupportsAsync:            false,
		SupportsStatusMessage:    true,
		SupportsStructuredOutput: true,
		SupportsBlocking:         true,
		TimeoutUnit:              "milliseconds",
		SupportsPlatform:         false,
		SupportsCWD:              false,
		SupportsEnv:              false,
		SupportsLLMHooks:         false,
		SupportsHTTPHooks:        false,
	}
}
