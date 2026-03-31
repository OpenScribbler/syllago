package converter

import (
	"encoding/json"
	"fmt"
	"sort"
)

func init() {
	RegisterAdapter(&WindsurfAdapter{})
}

// WindsurfAdapter handles hooks for Windsurf (Codeium).
// Windsurf uses a split-event model: instead of event+matcher, each tool category
// gets its own event name (pre_run_command, pre_read_code, etc.). Encoding fans out
// canonical before_tool_execute/after_tool_execute hooks into per-tool events;
// decoding merges them back.
type WindsurfAdapter struct{}

func (a *WindsurfAdapter) ProviderSlug() string { return "windsurf" }

func (a *WindsurfAdapter) FieldsToVerify() []string {
	return []string{VerifyFieldEvent, VerifyFieldMatcher}
}

// --- Windsurf provider-native structs ---

type wsHookEntry struct {
	Command          string `json:"command"`
	ShowOutput       bool   `json:"show_output,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
}

type wsHooksFile struct {
	Hooks map[string][]wsHookEntry `json:"hooks"`
}

// --- Split-event mappings ---

// wsSplitEvents maps canonical matcher values to their Windsurf per-tool event names.
// The bool selects pre (true) or post (false) variants.
var wsSplitPre = map[string]string{
	"shell":      "pre_run_command",
	"file_read":  "pre_read_code",
	"file_write": "pre_write_code",
	"file_edit":  "pre_write_code",
	"mcp":        "pre_mcp_tool_use",
}

var wsSplitPost = map[string]string{
	"shell":      "post_run_command",
	"file_read":  "post_read_code",
	"file_write": "post_write_code",
	"file_edit":  "post_write_code",
	"mcp":        "post_mcp_tool_use",
}

// All 4 pre/post events for wildcard expansion (nil matcher).
var wsAllPre = []string{"pre_run_command", "pre_read_code", "pre_write_code", "pre_mcp_tool_use"}
var wsAllPost = []string{"post_run_command", "post_read_code", "post_write_code", "post_mcp_tool_use"}

// wsMatcherFromEvent derives a canonical matcher from a Windsurf split-event name.
func wsMatcherFromEvent(wsEvent string) string {
	switch wsEvent {
	case "pre_run_command", "post_run_command":
		return "shell"
	case "pre_read_code", "post_read_code":
		return "file_read"
	case "pre_write_code", "post_write_code":
		return "file_write"
	case "pre_mcp_tool_use", "post_mcp_tool_use":
		return "mcp"
	default:
		return ""
	}
}

// wsIsPre returns true for pre-events (blocking by default).
func wsIsPre(wsEvent string) bool {
	switch wsEvent {
	case "pre_run_command", "pre_read_code", "pre_write_code", "pre_mcp_tool_use", "pre_user_prompt":
		return true
	default:
		return false
	}
}

// wsIsSplitEvent returns true for events that are part of the split-event model.
func wsIsSplitEvent(wsEvent string) bool {
	return wsMatcherFromEvent(wsEvent) != ""
}

// wsEventsForMatcher returns the Windsurf split-event names for a canonical matcher.
func wsEventsForMatcher(matcher json.RawMessage, pre bool) ([]string, error) {
	splitMap := wsSplitPre
	allEvents := wsAllPre
	if !pre {
		splitMap = wsSplitPost
		allEvents = wsAllPost
	}

	// nil matcher → expand to all 4 events
	if len(matcher) == 0 {
		return allEvents, nil
	}

	// Try to unmarshal as string
	var s string
	if err := json.Unmarshal(matcher, &s); err != nil {
		return nil, fmt.Errorf("windsurf split-event matcher must be a string, got: %s", string(matcher))
	}

	// Known matcher value → specific event
	if ev, ok := splitMap[s]; ok {
		return []string{ev}, nil
	}

	// Unknown string → treat as wildcard (all events)
	return allEvents, nil
}

func (a *WindsurfAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	var warnings []ConversionWarning
	result := wsHooksFile{Hooks: make(map[string][]wsHookEntry)}

	for _, hook := range hooks.Hooks {
		// 1. Only command handlers supported
		_, hWarnings, keep := TranslateHandlerType(hook.Handler, "windsurf", hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		// 2. Timeout not supported — warn and drop
		if hook.Handler.Timeout > 0 {
			warnings = append(warnings, ConversionWarning{
				Severity:    "info",
				Description: "windsurf does not support hook timeouts; timeout dropped",
			})
		}

		// 3. Degradation not supported — warn
		if hook.Degradation != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "info",
				Description: "windsurf does not support degradation policies; degradation dropped",
			})
		}

		// 4. Build entry
		entry := wsHookEntry{
			Command:          hook.Handler.Command,
			WorkingDirectory: hook.Handler.CWD,
		}

		// 5. Restore show_output from provider_data
		if hook.ProviderData != nil {
			if wsData, ok := hook.ProviderData["windsurf"].(map[string]any); ok {
				if so, ok := wsData["show_output"].(bool); ok && so {
					entry.ShowOutput = true
				}
			}
		}

		// 6. Route by event type
		switch hook.Event {
		case "before_tool_execute":
			// Non-blocking pre-hooks: wrap command with || true
			if !hook.Blocking {
				entry.Command = "(" + entry.Command + ") || true"
				warnings = append(warnings, ConversionWarning{
					Severity:    "info",
					Description: "windsurf pre-hooks are blocking by default; non-blocking hook wrapped with || true",
				})
			}
			events, err := wsEventsForMatcher(hook.Matcher, true)
			if err != nil {
				warnings = append(warnings, ConversionWarning{
					Severity:    "warning",
					Description: fmt.Sprintf("windsurf split-event: %v; hook skipped", err),
				})
				continue
			}
			for _, ev := range events {
				result.Hooks[ev] = append(result.Hooks[ev], entry)
			}

		case "after_tool_execute":
			events, err := wsEventsForMatcher(hook.Matcher, false)
			if err != nil {
				warnings = append(warnings, ConversionWarning{
					Severity:    "warning",
					Description: fmt.Sprintf("windsurf split-event: %v; hook skipped", err),
				})
				continue
			}
			for _, ev := range events {
				result.Hooks[ev] = append(result.Hooks[ev], entry)
			}

		default:
			// Direct-mapped events (session_start, before_prompt, etc.)
			nativeEvent, err := TranslateEventToProvider(hook.Event, "windsurf")
			if err != nil {
				warnings = append(warnings, ConversionWarning{
					Severity:    "warning",
					Description: fmt.Sprintf("hook event %q not supported by windsurf; skipped", hook.Event),
				})
				continue
			}
			result.Hooks[nativeEvent] = append(result.Hooks[nativeEvent], entry)
		}
	}

	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return &EncodedResult{
		Content:  content,
		Filename: "windsurf-hooks.json",
		Warnings: warnings,
	}, nil
}

func (a *WindsurfAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	var file wsHooksFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parsing windsurf hooks: %w", err)
	}

	ch := &CanonicalHooks{Spec: SpecVersion}

	// Try wildcard merge for pre-events and post-events
	mergedPre := tryMergeWildcard(file.Hooks, wsAllPre, "before_tool_execute", true)
	mergedPost := tryMergeWildcard(file.Hooks, wsAllPost, "after_tool_execute", false)

	// Track which events were consumed by wildcard merge
	merged := map[string]bool{}
	if mergedPre != nil {
		ch.Hooks = append(ch.Hooks, *mergedPre)
		for _, ev := range wsAllPre {
			merged[ev] = true
		}
	}
	if mergedPost != nil {
		ch.Hooks = append(ch.Hooks, *mergedPost)
		for _, ev := range wsAllPost {
			merged[ev] = true
		}
	}

	// Process remaining events in sorted order for deterministic output
	sortedEvents := make([]string, 0, len(file.Hooks))
	for ev := range file.Hooks {
		if !merged[ev] {
			sortedEvents = append(sortedEvents, ev)
		}
	}
	sort.Strings(sortedEvents)

	for _, wsEvent := range sortedEvents {
		entries := file.Hooks[wsEvent]

		for _, entry := range entries {
			hook := CanonicalHook{
				Handler: HookHandler{
					Type:    "command",
					Command: entry.Command,
					CWD:     entry.WorkingDirectory,
				},
			}

			// Preserve Windsurf-specific fields in provider_data
			if entry.ShowOutput || entry.WorkingDirectory != "" {
				pd := map[string]any{}
				if entry.ShowOutput {
					pd["show_output"] = true
				}
				hook.ProviderData = map[string]any{"windsurf": pd}
			}

			// Determine canonical event and matcher
			if wsIsSplitEvent(wsEvent) {
				if wsIsPre(wsEvent) {
					hook.Event = "before_tool_execute"
					hook.Blocking = true
				} else {
					hook.Event = "after_tool_execute"
					hook.Blocking = false
				}
				matcher := wsMatcherFromEvent(wsEvent)
				if matcher != "" {
					matcherJSON, _ := json.Marshal(matcher)
					hook.Matcher = matcherJSON
				}
			} else {
				// Direct-mapped event
				canonEvent, _ := TranslateEventFromProvider(wsEvent, "windsurf")
				hook.Event = canonEvent
			}

			ch.Hooks = append(ch.Hooks, hook)
		}
	}

	return ch, nil
}

// tryMergeWildcard checks if all split-events have exactly 1 entry with identical
// commands. If so, merges them into a single canonical hook with no matcher (wildcard).
func tryMergeWildcard(hooks map[string][]wsHookEntry, events []string, canonEvent string, isPre bool) *CanonicalHook {
	if len(events) == 0 {
		return nil
	}

	// All events must be present with exactly 1 entry each
	var cmd string
	for i, ev := range events {
		entries, ok := hooks[ev]
		if !ok || len(entries) != 1 {
			return nil
		}
		if i == 0 {
			cmd = entries[0].Command
		} else if entries[0].Command != cmd {
			return nil
		}
	}

	return &CanonicalHook{
		Event:    canonEvent,
		Blocking: isPre,
		Handler: HookHandler{
			Type:    "command",
			Command: cmd,
		},
		ProviderData: map[string]any{
			"windsurf": map[string]any{
				"expanded_from": "wildcard",
			},
		},
	}
}

func (a *WindsurfAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start", "session_end",
			"worktree_create", "transcript_export",
		},
		SupportsMatchers: true,
		SupportsBlocking: true,
		SupportsCWD:      true,
		TimeoutUnit:      "",
	}
}
