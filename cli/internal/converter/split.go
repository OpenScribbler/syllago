package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SplitSettingsHooks reads the hooks section of a settings.json-style file
// and returns one HookData per event+matcher group.
// sourceProvider is used to reverse-translate event and tool names to canonical.
func SplitSettingsHooks(content []byte, sourceProvider string) ([]HookData, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("parsing hooks JSON: %w", err)
	}

	hooksRaw, ok := raw["hooks"]
	if !ok {
		return nil, fmt.Errorf("no 'hooks' key found in content")
	}

	var eventMap map[string]json.RawMessage
	if err := json.Unmarshal(hooksRaw, &eventMap); err != nil {
		return nil, fmt.Errorf("parsing hooks object: %w", err)
	}

	var items []HookData

	for event, matchersRaw := range eventMap {
		canonicalEvent := ReverseTranslateHookEvent(event, sourceProvider)

		if sourceProvider == "copilot-cli" {
			// Copilot format: array of {bash, powershell, timeoutSec, comment}
			var entries []copilotHookEntry
			if err := json.Unmarshal(matchersRaw, &entries); err != nil {
				return nil, fmt.Errorf("parsing copilot hooks for event %q: %w", event, err)
			}
			for _, e := range entries {
				cmd := e.Bash
				if cmd == "" {
					cmd = e.PowerShell
				}
				// Copilot timeoutSec is already in seconds — matches canonical unit
				he := HookEntry{
					Type:          "command",
					Command:       cmd,
					Timeout:       e.TimeoutSec,
					StatusMessage: e.Comment,
				}
				items = append(items, HookData{
					Event: canonicalEvent,
					Hooks: []HookEntry{he},
				})
			}
		} else {
			// Standard format (claude-code, gemini-cli, kiro): array of {matcher, hooks[]}
			var matchers []hookMatcher
			if err := json.Unmarshal(matchersRaw, &matchers); err != nil {
				return nil, fmt.Errorf("parsing matchers for event %q: %w", event, err)
			}
			for _, m := range matchers {
				matcher := m.Matcher
				if matcher != "" {
					matcher = ReverseTranslateMatcher(matcher, sourceProvider)
				}
				// Convert provider ms timeouts to canonical seconds
				hooks := make([]HookEntry, len(m.Hooks))
				copy(hooks, m.Hooks)
				for i := range hooks {
					if hooks[i].Timeout > 0 {
						hooks[i].Timeout = hooks[i].Timeout / 1000
					}
				}
				items = append(items, HookData{
					Event:   canonicalEvent,
					Matcher: matcher,
					Hooks:   hooks,
				})
			}
		}
	}

	return items, nil
}

// DeriveHookName generates a filesystem-safe name from a HookData item.
// Priority:
//  1. statusMessage if present → slugify
//  2. matcher + event → slugify (e.g., "pretooluse-bash")
//  3. event + first meaningful word(s) from command
func DeriveHookName(hook HookData) string {
	// Priority 1: use statusMessage from the first hook that has one
	for _, h := range hook.Hooks {
		if h.StatusMessage != "" {
			return slugify(h.StatusMessage)
		}
	}

	// Priority 2: matcher + event
	if hook.Matcher != "" {
		return slugify(hook.Event + "-" + hook.Matcher)
	}

	// Priority 3: event + first meaningful word from command
	for _, h := range hook.Hooks {
		if h.Command != "" {
			fields := strings.Fields(h.Command)
			if len(fields) > 0 {
				return slugify(hook.Event + "-" + fields[0])
			}
		}
	}

	return slugify(hook.Event)
}
