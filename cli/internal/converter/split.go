package converter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// SplitSettingsHooks reads the hooks section of a settings.json-style file
// and returns one HookData per (event, matcher, handler) triple. Each result
// HookData has exactly one entry in its Hooks slice, matching the canonical
// hooks/0.1 spec shape (one hook = one handler).
//
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
				for _, h := range m.Hooks {
					entry := h
					// Convert provider ms timeouts to canonical seconds
					if entry.Timeout > 0 {
						entry.Timeout = entry.Timeout / 1000
					}
					items = append(items, HookData{
						Event:   canonicalEvent,
						Matcher: matcher,
						Hooks:   []HookEntry{entry},
					})
				}
			}
		}
	}

	return items, nil
}

// DeriveHookName generates a filesystem-safe name from a HookData item.
// With per-handler splitting (post-2026-04-23), hook.Hooks typically has
// exactly one entry; older callers with multi-entry slices still work by
// falling back to the first entry that yields a usable signal.
//
// Priority:
//  1. statusMessage — highest-signal human-readable label
//  2. script basename — e.g., "before_prompt-capture-rating" for `bun run capture-rating.ts`
//  3. event + matcher — when matcher is not a wildcard
//  4. event + first command token — last-resort differentiator
//  5. event alone
func DeriveHookName(hook HookData) string {
	// Priority 1: status message from any entry that sets one
	for _, h := range hook.Hooks {
		if h.StatusMessage != "" {
			return slugify(h.StatusMessage)
		}
	}

	// Priority 2: script basename extracted from command
	for _, h := range hook.Hooks {
		if base := scriptBasename(h.Command); base != "" {
			return slugify(hook.Event + "-" + base)
		}
	}

	// Priority 3: event + matcher (skip trivial wildcards)
	if hook.Matcher != "" && hook.Matcher != "*" {
		return slugify(hook.Event + "-" + hook.Matcher)
	}

	// Priority 4: event + first command token
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

// scriptExtensions are file suffixes recognized as script references in
// shell-style command strings. Ordered by rough frequency in AI-tool hooks.
var scriptExtensions = []string{".sh", ".ts", ".js", ".mjs", ".cjs", ".py", ".rb", ".pl", ".ps1", ".bat"}

// scriptBasename scans a command string for a token that looks like a script
// path (e.g., "bash path/to/foo.sh --flag") and returns the file name without
// its extension. Returns empty string when no script reference is found.
func scriptBasename(cmd string) string {
	if cmd == "" {
		return ""
	}
	for _, tok := range strings.Fields(cmd) {
		// Strip surrounding quotes that survive a naive Fields() split.
		tok = strings.Trim(tok, `"'`)
		lower := strings.ToLower(tok)
		for _, ext := range scriptExtensions {
			if strings.HasSuffix(lower, ext) {
				base := filepath.Base(tok)
				return strings.TrimSuffix(base, filepath.Ext(base))
			}
		}
	}
	return ""
}
