package converter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func init() {
	Register(&HooksConverter{})
}

// HookEntry represents a single hook action in syllago canonical format.
// Timeout is in seconds (canonical unit). Provider-specific conversions:
//   - Claude Code / Gemini CLI: milliseconds (divide by 1000 on canonicalize, multiply on render)
//   - Copilot CLI: seconds (no conversion needed)
//   - Kiro: milliseconds in timeout_ms field (multiply by 1000 on render)
//
// Claude Code supports 4 hook types, each with different fields:
//   - "command": Command, Timeout, StatusMessage, Async
//   - "http": URL, Headers, AllowedEnvVars, Timeout, StatusMessage
//   - "prompt": Prompt, Model, Timeout, StatusMessage
//   - "agent": Agent, Timeout, StatusMessage
//
// Type defaults to "command" when empty (backwards compatibility).
type HookEntry struct {
	Type          string `json:"type"`
	Command       string `json:"command,omitempty"`
	Timeout       int    `json:"timeout,omitempty"`
	StatusMessage string `json:"statusMessage,omitempty"`
	Async         bool   `json:"async,omitempty"`

	// HTTP hook fields (type: "http")
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	AllowedEnvVars []string          `json:"allowedEnvVars,omitempty"`

	// Prompt hook fields (type: "prompt")
	Prompt string `json:"prompt,omitempty"`
	Model  string `json:"model,omitempty"`

	// Agent hook fields (type: "agent")
	Agent json.RawMessage `json:"agent,omitempty"`
}

// hookMatcher represents an event matcher with its hooks in canonical format.
type hookMatcher struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []HookEntry `json:"hooks"`
}

// hooksConfig is the top-level hooks structure (syllago canonical format).
type hooksConfig struct {
	Hooks          map[string][]hookMatcher `json:"hooks"`
	SourceProvider string                   `json:"sourceProvider,omitempty"`
}

// HookData is the canonical representation of a single hook group (flat format).
// One event + one matcher + one or more hook entries. Used by the compatibility
// engine, TUI rendering, and import splitting.
type HookData struct {
	Event          string      `json:"event"`
	Matcher        string      `json:"matcher,omitempty"`
	Hooks          []HookEntry `json:"hooks"`
	SourceProvider string      `json:"sourceProvider,omitempty"`
}

// DetectHookFormat returns "flat" if content has a top-level "event" field,
// or "nested" if it has a top-level "hooks" object.
func DetectHookFormat(content []byte) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return "nested" // default
	}
	if _, ok := raw["event"]; ok {
		return "flat"
	}
	return "nested"
}

// ParseFlat parses a flat-format hook file into a HookData.
func ParseFlat(content []byte) (HookData, error) {
	var hd HookData
	if err := json.Unmarshal(content, &hd); err != nil {
		return HookData{}, fmt.Errorf("parsing flat hook: %w", err)
	}
	if hd.Event == "" {
		return HookData{}, fmt.Errorf("flat hook missing 'event' field")
	}
	return hd, nil
}

// ParseNested parses the nested {"hooks":{"EventName":[...]}} format and returns
// all hook groups as individual HookData items.
func ParseNested(content []byte) ([]HookData, error) {
	var cfg hooksConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing nested hooks: %w", err)
	}
	var items []HookData
	for event, matchers := range cfg.Hooks {
		for _, m := range matchers {
			items = append(items, HookData{
				Event:   event,
				Matcher: m.Matcher,
				Hooks:   m.Hooks,
			})
		}
	}
	return items, nil
}

// LoadHookData reads and parses the hook.json from a hook content item.
// If item.Path is a directory, resolves hook.json inside it.
// Returns a HookData for flat format, or the first group for nested format.
func LoadHookData(item catalog.ContentItem) (HookData, error) {
	if item.Type != catalog.Hooks {
		return HookData{}, fmt.Errorf("item is not a hook")
	}
	hookPath := item.Path
	fi, err := os.Stat(hookPath)
	if err != nil {
		return HookData{}, err
	}
	if fi.IsDir() {
		hookPath = filepath.Join(hookPath, "hook.json")
	}
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return HookData{}, err
	}
	if DetectHookFormat(data) == "flat" {
		return ParseFlat(data)
	}
	// Nested: return first group
	items, err := ParseNested(data)
	if err != nil {
		return HookData{}, err
	}
	if len(items) == 0 {
		return HookData{}, fmt.Errorf("no hook groups found in nested format")
	}
	return items[0], nil
}

// RenderFlat converts a HookData into the target provider's format for a single hook.
func (c *HooksConverter) RenderFlat(hook HookData, target provider.Provider) (*Result, error) {
	// Wrap the single hook in a hooksConfig so we can reuse existing render logic
	cfg := hooksConfig{
		Hooks: map[string][]hookMatcher{
			hook.Event: {{Matcher: hook.Matcher, Hooks: hook.Hooks}},
		},
		SourceProvider: hook.SourceProvider,
	}
	content, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return c.Render(content, target)
}

// copilotHookEntry represents a single hook in Copilot CLI format.
type copilotHookEntry struct {
	Type       string            `json:"type,omitempty"`
	Bash       string            `json:"bash,omitempty"`
	PowerShell string            `json:"powershell,omitempty"`
	TimeoutSec int               `json:"timeoutSec,omitempty"`
	Comment    string            `json:"comment,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Cwd        string            `json:"cwd,omitempty"`
}

// copilotMatcherGroup represents a matcher group in Copilot CLI format.
// Copilot supports matchers — each group has a matcher pattern and a list of hooks.
type copilotMatcherGroup struct {
	Matcher string             `json:"matcher,omitempty"`
	Hooks   []copilotHookEntry `json:"hooks"`
}

// copilotHooksConfig is the Copilot hooks structure.
type copilotHooksConfig struct {
	Version int                              `json:"version"`
	Hooks   map[string][]copilotMatcherGroup `json:"hooks"`
}

// LLMHooksModeSkip drops LLM-evaluated hooks with a warning (default).
const LLMHooksModeSkip = "skip"

// LLMHooksModeGenerate generates wrapper scripts that call the target provider's CLI.
const LLMHooksModeGenerate = "generate"

type HooksConverter struct {
	// LLMHooksMode controls how LLM-evaluated hooks (type: "prompt"/"agent") are handled.
	// "skip" (default): drop with warning. "generate": create wrapper scripts.
	LLMHooksMode string
}

func (c *HooksConverter) ContentType() catalog.ContentType {
	return catalog.Hooks
}

func (c *HooksConverter) Canonicalize(content []byte, sourceProvider string) (*Result, error) {
	// Auto-detect flat vs nested format
	if DetectHookFormat(content) == "flat" {
		return canonicalizeFlatHook(content, sourceProvider)
	}
	switch sourceProvider {
	case "copilot-cli":
		return canonicalizeCopilotHooks(content)
	case "cursor":
		// Cursor hooks use CC-style event names (mapped via HookEvents).
		// Unique fields (failClosed, loop_limit, version) are not yet preserved.
		return canonicalizeStandardHooks(content, sourceProvider)
	case "windsurf":
		// Windsurf uses per-tool-category events (pre_read_code, etc.) — structural
		// mismatch with generic PreToolUse+matcher. Standard canonicalization is a
		// best-effort pass-through. TODO: implement proper Windsurf event mapping.
		return canonicalizeStandardHooks(content, sourceProvider)
	default:
		// Claude Code and Gemini CLI share the same structure, just different event/tool names
		return canonicalizeStandardHooks(content, sourceProvider)
	}
}

// canonicalizeFlatHook translates a flat-format hook's event and tool names to canonical.
func canonicalizeFlatHook(content []byte, sourceProvider string) (*Result, error) {
	hd, err := ParseFlat(content)
	if err != nil {
		return nil, err
	}

	hd.Event = ReverseTranslateHookEvent(hd.Event, sourceProvider)
	if hd.Matcher != "" {
		hd.Matcher = ReverseTranslateMatcher(hd.Matcher, sourceProvider)
	}

	// Convert provider ms timeouts to canonical seconds (all flat-format providers use ms)
	for i := range hd.Hooks {
		if hd.Hooks[i].Timeout > 0 {
			hd.Hooks[i].Timeout = hd.Hooks[i].Timeout / 1000
		}
	}

	hd.SourceProvider = sourceProvider

	out, err := json.MarshalIndent(hd, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: out, Filename: "hook.json"}, nil
}

// hooklessProviders lists providers that have no hook system.
// Converting hooks to these providers emits a warning instead of content.
var hooklessProviders = map[string]bool{
	"zed":      true,
	"roo-code": true,
}

func (c *HooksConverter) Render(content []byte, target provider.Provider) (*Result, error) {
	if hooklessProviders[target.Slug] {
		return &Result{
			Warnings: []string{fmt.Sprintf("target provider %q does not support hooks; hook content was not converted", target.Slug)},
		}, nil
	}

	var cfg hooksConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing canonical hooks: %w", err)
	}

	mode := c.LLMHooksMode
	if mode == "" {
		mode = LLMHooksModeSkip
	}

	switch target.Slug {
	case "copilot-cli":
		return renderCopilotHooks(cfg, mode)
	case "kiro":
		return renderKiroHooks(cfg, mode)
	case "cursor":
		// Cursor uses hooks.json with a unique schema: {"version": 1, "hooks": {"EventName": [...]}}
		// Each entry supports fields not yet handled: failClosed, loop_limit, version.
		// Cursor uses CC-style event names (PreToolUse, etc.) mapped via HookEvents.
		// TODO: support Cursor-specific fields.
		return renderStandardHooks(cfg, target.Slug, mode)
	case "windsurf":
		// Windsurf uses per-tool-category events (pre_read_code, pre_write_code, etc.)
		// instead of generic PreToolUse+matcher. This structural mismatch means event-level
		// translation may not be accurate. TODO: implement Windsurf-specific event mapping.
		return renderStandardHooks(cfg, target.Slug, mode)
	default:
		// Claude Code and Gemini CLI
		return renderStandardHooks(cfg, target.Slug, mode)
	}
}

// --- Canonicalizers ---

func canonicalizeStandardHooks(content []byte, sourceProvider string) (*Result, error) {
	var cfg hooksConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing hooks JSON: %w", err)
	}

	// Translate event names and matcher tool names to canonical
	canonical := hooksConfig{Hooks: make(map[string][]hookMatcher), SourceProvider: sourceProvider}
	for event, matchers := range cfg.Hooks {
		canonicalEvent := ReverseTranslateHookEvent(event, sourceProvider)

		var canonicalMatchers []hookMatcher
		for _, m := range matchers {
			// Convert hook timeouts from provider ms to canonical seconds
			hooks := make([]HookEntry, len(m.Hooks))
			copy(hooks, m.Hooks)
			for i := range hooks {
				if hooks[i].Timeout > 0 {
					hooks[i].Timeout = hooks[i].Timeout / 1000
				}
			}
			cm := hookMatcher{
				Matcher: m.Matcher,
				Hooks:   hooks,
			}
			// Translate matcher tool name to canonical
			if cm.Matcher != "" {
				cm.Matcher = ReverseTranslateMatcher(cm.Matcher, sourceProvider)
			}
			canonicalMatchers = append(canonicalMatchers, cm)
		}
		canonical.Hooks[canonicalEvent] = canonicalMatchers
	}

	out, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: out, Filename: "hooks.json"}, nil
}

func canonicalizeCopilotHooks(content []byte) (*Result, error) {
	var cfg copilotHooksConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing Copilot hooks JSON: %w", err)
	}

	canonical := hooksConfig{Hooks: make(map[string][]hookMatcher), SourceProvider: "copilot-cli"}
	for event, groups := range cfg.Hooks {
		canonicalEvent := ReverseTranslateHookEvent(event, "copilot-cli")

		var matchers []hookMatcher
		for _, g := range groups {
			var hooks []HookEntry
			for _, e := range g.Hooks {
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
				hooks = append(hooks, he)
			}
			matcher := g.Matcher
			if matcher != "" {
				matcher = ReverseTranslateMatcher(matcher, "copilot-cli")
			}
			matchers = append(matchers, hookMatcher{
				Matcher: matcher,
				Hooks:   hooks,
			})
		}
		canonical.Hooks[canonicalEvent] = matchers
	}

	out, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: out, Filename: "hooks.json"}, nil
}

// --- Renderers ---

func renderStandardHooks(cfg hooksConfig, targetSlug string, llmMode string) (*Result, error) {
	out := hooksConfig{Hooks: make(map[string][]hookMatcher)}
	var warnings []string
	extraFiles := map[string][]byte{}

	// Check for structured output capability loss
	if cfg.SourceProvider != "" {
		warnings = append(warnings, structuredOutputWarnings(cfg.SourceProvider, targetSlug)...)
	}
	scriptIdx := 0

	for event, matchers := range cfg.Hooks {
		translated, supported := TranslateHookEvent(event, targetSlug)
		if !supported {
			warnings = append(warnings, fmt.Sprintf("hook event %q is not supported by %s (dropped)", event, targetSlug))
			continue
		}
		targetEvent := translated

		var targetMatchers []hookMatcher
		for _, m := range matchers {
			tm := hookMatcher{
				Matcher: m.Matcher,
				Hooks:   make([]HookEntry, len(m.Hooks)),
			}
			// Translate matcher tool name
			if tm.Matcher != "" {
				tm.Matcher = TranslateMatcher(tm.Matcher, targetSlug)
			}
			copy(tm.Hooks, m.Hooks)

			var kept []HookEntry
			for _, h := range tm.Hooks {
				hType := h.Type
				if hType == "" {
					hType = "command"
				}

				// Claude Code supports all 4 hook types natively — pass them through.
				// Other providers only support command hooks.
				if hType != "command" && targetSlug != "claude-code" {
					if hType == "prompt" || hType == "agent" {
						if llmMode == LLMHooksModeGenerate {
							scriptName, scriptContent := generateLLMWrapperScript(h, targetSlug, event, scriptIdx)
							scriptIdx++
							extraFiles[scriptName] = scriptContent
							kept = append(kept, HookEntry{
								Type:          "command",
								Command:       "./" + scriptName,
								Timeout:       30000, // LLM calls need more time (ms)
								StatusMessage: fmt.Sprintf("syllago-generated: LLM-evaluated hook (from %s)", h.Type),
							})
							warnings = append(warnings, fmt.Sprintf("LLM hook (type: %q) converted to wrapper script %s", h.Type, scriptName))
						} else {
							warnings = append(warnings, fmt.Sprintf("LLM-evaluated hook (type: %q) dropped for %s (use --llm-hooks=generate to create wrapper scripts)", h.Type, targetSlug))
						}
					} else {
						// http and any other non-command types: no conversion possible, just warn
						warnings = append(warnings, fmt.Sprintf("hook type %q is only supported by Claude Code; dropped for %s", hType, targetSlug))
					}
					continue
				}

				// Convert canonical seconds to provider milliseconds
				rendered := h
				if rendered.Timeout > 0 {
					rendered.Timeout = rendered.Timeout * 1000
				}
				kept = append(kept, rendered)
			}
			tm.Hooks = kept

			if len(tm.Hooks) > 0 {
				targetMatchers = append(targetMatchers, tm)
			}
		}

		if len(targetMatchers) > 0 {
			out.Hooks[targetEvent] = targetMatchers
		}
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	r := &Result{Content: result, Filename: "hooks.json", Warnings: warnings}
	if len(extraFiles) > 0 {
		r.ExtraFiles = extraFiles
	}
	return r, nil
}

func renderCopilotHooks(cfg hooksConfig, llmMode string) (*Result, error) {
	out := copilotHooksConfig{
		Version: 1,
		Hooks:   make(map[string][]copilotMatcherGroup),
	}
	var warnings []string
	extraFiles := map[string][]byte{}

	// Check for structured output capability loss
	if cfg.SourceProvider != "" {
		warnings = append(warnings, structuredOutputWarnings(cfg.SourceProvider, "copilot-cli")...)
	}
	scriptIdx := 0

	for event, matchers := range cfg.Hooks {
		targetEvent, supported := TranslateHookEvent(event, "copilot-cli")
		if !supported {
			warnings = append(warnings, fmt.Sprintf("hook event %q is not supported by copilot-cli (dropped)", event))
			continue
		}

		var groups []copilotMatcherGroup
		for _, m := range matchers {
			// Translate matcher tool name to Copilot's vocabulary
			matcher := ""
			if m.Matcher != "" {
				matcher = TranslateMatcher(m.Matcher, "copilot-cli")
			}

			var entries []copilotHookEntry
			for _, h := range m.Hooks {
				hType := h.Type
				if hType == "" {
					hType = "command"
				}
				if hType != "command" {
					if hType == "prompt" || hType == "agent" {
						if llmMode == LLMHooksModeGenerate {
							scriptName, scriptContent := generateLLMWrapperScript(h, "copilot-cli", event, scriptIdx)
							scriptIdx++
							extraFiles[scriptName] = scriptContent
							entries = append(entries, copilotHookEntry{
								Type:       "command",
								Bash:       "./" + scriptName,
								TimeoutSec: 30,
								Comment:    fmt.Sprintf("syllago-generated: LLM-evaluated hook (from %s)", h.Type),
							})
							warnings = append(warnings, fmt.Sprintf("LLM hook (type: %q) converted to wrapper script %s", h.Type, scriptName))
						} else {
							warnings = append(warnings, fmt.Sprintf("LLM-evaluated hook (type: %q) dropped for copilot-cli (use --llm-hooks=generate to create wrapper scripts)", h.Type))
						}
					} else {
						warnings = append(warnings, fmt.Sprintf("hook type %q is only supported by Claude Code; dropped for copilot-cli", hType))
					}
					continue
				}
				// Canonical timeout is already in seconds — matches Copilot's timeoutSec
				entry := copilotHookEntry{
					Type:       "command",
					Bash:       h.Command,
					TimeoutSec: h.Timeout,
					Comment:    h.StatusMessage,
				}
				entries = append(entries, entry)
			}

			if len(entries) > 0 {
				groups = append(groups, copilotMatcherGroup{
					Matcher: matcher,
					Hooks:   entries,
				})
			}
		}

		if len(groups) > 0 {
			out.Hooks[targetEvent] = groups
		}
	}

	result, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	r := &Result{Content: result, Filename: "hooks.json", Warnings: warnings}
	if len(extraFiles) > 0 {
		r.ExtraFiles = extraFiles
	}
	return r, nil
}

// --- Kiro hooks ---

// kiroHookEntry is a single hook in Kiro's format.
type kiroHookEntry struct {
	Command   string `json:"command"`
	Matcher   string `json:"matcher,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

// kiroHooksAgent is the shape of the syllago-hooks.json file Kiro reads.
type kiroHooksAgent struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Prompt      string                     `json:"prompt"`
	Hooks       map[string][]kiroHookEntry `json:"hooks"`
}

func renderKiroHooks(cfg hooksConfig, llmMode string) (*Result, error) {
	var warnings []string

	// Check for structured output capability loss
	if cfg.SourceProvider != "" {
		warnings = append(warnings, structuredOutputWarnings(cfg.SourceProvider, "kiro")...)
	}

	kiroHooks := make(map[string][]kiroHookEntry)

	for event, matchers := range cfg.Hooks {
		translated, supported := TranslateHookEvent(event, "kiro")
		if !supported {
			warnings = append(warnings, fmt.Sprintf("hook event %q is not supported by Kiro (dropped)", event))
			continue
		}

		for _, m := range matchers {
			matcher := ""
			if m.Matcher != "" {
				matcher = TranslateMatcher(m.Matcher, "kiro")
			}

			for _, h := range m.Hooks {
				hType := h.Type
				if hType == "" {
					hType = "command"
				}
				if hType != "command" {
					if hType == "prompt" || hType == "agent" {
						if llmMode == LLMHooksModeGenerate {
							scriptName, _ := generateLLMWrapperScript(h, "kiro", event, len(kiroHooks))
							kiroHooks[translated] = append(kiroHooks[translated], kiroHookEntry{
								Command:   "./" + scriptName,
								Matcher:   matcher,
								TimeoutMs: 30000,
							})
							warnings = append(warnings, fmt.Sprintf("LLM hook (type: %q) converted to wrapper script %s", h.Type, scriptName))
						} else {
							warnings = append(warnings, fmt.Sprintf("LLM-evaluated hook (type: %q) dropped for kiro", h.Type))
						}
					} else {
						warnings = append(warnings, fmt.Sprintf("hook type %q is only supported by Claude Code; dropped for kiro", hType))
					}
					continue
				}

				// Convert canonical seconds to Kiro's timeout_ms (milliseconds)
				entry := kiroHookEntry{
					Command:   h.Command,
					Matcher:   matcher,
					TimeoutMs: h.Timeout * 1000,
				}
				kiroHooks[translated] = append(kiroHooks[translated], entry)
			}
		}
	}

	agent := kiroHooksAgent{
		Name:        "syllago-hooks",
		Description: "Hooks installed by syllago",
		Prompt:      "",
		Hooks:       kiroHooks,
	}

	result, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "syllago-hooks.json", Warnings: warnings}, nil
}

// --- Structured output warnings ---

// structuredOutputWarnings returns warnings if the source provider supports
// structured hook output fields that the target does not. This is called during
// render to alert users that hook output behavior will be silently lost.
func structuredOutputWarnings(sourceProvider, targetSlug string) []string {
	lost := OutputFieldsLostWarnings(sourceProvider, targetSlug)
	if len(lost) == 0 {
		return nil
	}

	// Build a single warning listing all lost fields
	return []string{
		fmt.Sprintf("structured hook output fields [%s] supported by %s but not by %s (hook output will be ignored)",
			strings.Join(lost, ", "), sourceProvider, targetSlug),
	}
}

// --- LLM wrapper script generation ---

// cliCommands maps provider slugs to their CLI command names.
var cliCommands = map[string]string{
	"claude-code": "claude",
	"gemini-cli":  "gemini",
	"kiro":        "kiro",
}

// generateLLMWrapperScript creates a shell script that calls the target provider's
// CLI to evaluate an LLM hook. Returns (filename, content).
func generateLLMWrapperScript(h HookEntry, targetSlug string, event string, idx int) (string, []byte) {
	scriptName := fmt.Sprintf("syllago-llm-hook-%s-%d.sh", sanitizeForFilename(event), idx)

	cli := cliCommands[targetSlug]
	if cli == "" {
		cli = "gemini" // fallback
	}

	prompt := h.Prompt
	if prompt == "" {
		prompt = h.Command // fallback for legacy format
	}
	if prompt == "" {
		prompt = "Evaluate this hook input and respond with a JSON decision."
	}

	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	b.WriteString("# syllago-generated: LLM-evaluated hook wrapper\n")
	fmt.Fprintf(&b, "# Original type: %s | Event: %s\n", h.Type, event)
	b.WriteString("# Calls the target provider's CLI in non-interactive mode.\n")
	b.WriteString("# No API key needed — uses the locally installed CLI's auth.\n")
	b.WriteString("\n")
	b.WriteString("INPUT=$(cat)\n")
	b.WriteString("\n")
	b.WriteString("# Extract tool context from hook input\n")
	b.WriteString("TOOL_NAME=$(echo \"$INPUT\" | jq -r '.tool_name // empty')\n")
	b.WriteString("TOOL_CMD=$(echo \"$INPUT\" | jq -r '.tool_input.command // empty')\n")
	b.WriteString("\n")

	escapedPrompt := shellEscape(prompt)

	switch targetSlug {
	case "gemini-cli":
		fmt.Fprintf(&b, "RESPONSE=$(%s -p %s --output-format json 2>/dev/null)\n", cli, escapedPrompt)
		b.WriteString("echo \"$RESPONSE\" | jq -r '.response'\n")
	case "claude-code":
		fmt.Fprintf(&b, "RESPONSE=$(%s -p %s --output-format json 2>/dev/null)\n", cli, escapedPrompt)
		b.WriteString("echo \"$RESPONSE\" | jq -r '.result // .'\n")
	default:
		fmt.Fprintf(&b, "RESPONSE=$(echo %s | %s 2>/dev/null)\n", escapedPrompt, cli)
		b.WriteString("echo \"$RESPONSE\"\n")
	}

	return scriptName, []byte(b.String())
}

// sanitizeForFilename replaces non-alphanumeric chars with hyphens.
func sanitizeForFilename(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

// shellEscape wraps a string in single quotes for safe shell embedding.
func shellEscape(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}
