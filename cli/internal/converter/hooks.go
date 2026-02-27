package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
)

func init() {
	Register(&HooksConverter{})
}

// hookEntry represents a single hook action in canonical (Claude Code) format.
type hookEntry struct {
	Type          string `json:"type"`
	Command       string `json:"command,omitempty"`
	Timeout       int    `json:"timeout,omitempty"`
	StatusMessage string `json:"statusMessage,omitempty"`
	Async         bool   `json:"async,omitempty"`
}

// hookMatcher represents an event matcher with its hooks in canonical format.
type hookMatcher struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []hookEntry `json:"hooks"`
}

// hooksConfig is the top-level hooks structure (canonical = Claude Code format).
type hooksConfig struct {
	Hooks map[string][]hookMatcher `json:"hooks"`
}

// copilotHookEntry represents a single hook in Copilot CLI format.
type copilotHookEntry struct {
	Bash       string `json:"bash,omitempty"`
	PowerShell string `json:"powershell,omitempty"`
	TimeoutSec int    `json:"timeoutSec,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// copilotHooksConfig is the Copilot hooks structure.
type copilotHooksConfig struct {
	Hooks map[string][]copilotHookEntry `json:"hooks"`
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
	switch sourceProvider {
	case "copilot-cli":
		return canonicalizeCopilotHooks(content)
	default:
		// Claude Code and Gemini CLI share the same structure, just different event/tool names
		return canonicalizeStandardHooks(content, sourceProvider)
	}
}

func (c *HooksConverter) Render(content []byte, target provider.Provider) (*Result, error) {
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
	canonical := hooksConfig{Hooks: make(map[string][]hookMatcher)}
	for event, matchers := range cfg.Hooks {
		canonicalEvent := event
		if sourceProvider != "claude-code" {
			canonicalEvent = ReverseTranslateHookEvent(event, sourceProvider)
		}

		var canonicalMatchers []hookMatcher
		for _, m := range matchers {
			cm := hookMatcher{
				Matcher: m.Matcher,
				Hooks:   m.Hooks,
			}
			// Translate matcher tool name to canonical
			if cm.Matcher != "" && sourceProvider != "claude-code" {
				cm.Matcher = ReverseTranslateTool(cm.Matcher, sourceProvider)
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

	canonical := hooksConfig{Hooks: make(map[string][]hookMatcher)}
	for event, entries := range cfg.Hooks {
		canonicalEvent := ReverseTranslateHookEvent(event, "copilot-cli")

		var matchers []hookMatcher
		for _, e := range entries {
			cmd := e.Bash
			if cmd == "" {
				cmd = e.PowerShell
			}
			timeout := e.TimeoutSec * 1000 // Convert seconds to milliseconds

			he := hookEntry{
				Type:          "command",
				Command:       cmd,
				Timeout:       timeout,
				StatusMessage: e.Comment,
			}
			matchers = append(matchers, hookMatcher{
				Hooks: []hookEntry{he},
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
	scriptIdx := 0

	for event, matchers := range cfg.Hooks {
		var targetEvent string
		if targetSlug == "claude-code" {
			// Claude Code is canonical — events pass through untranslated
			targetEvent = event
		} else {
			translated, supported := TranslateHookEvent(event, targetSlug)
			if !supported {
				warnings = append(warnings, fmt.Sprintf("hook event %q is not supported by %s (dropped)", event, targetSlug))
				continue
			}
			targetEvent = translated
		}

		var targetMatchers []hookMatcher
		for _, m := range matchers {
			tm := hookMatcher{
				Matcher: m.Matcher,
				Hooks:   make([]hookEntry, len(m.Hooks)),
			}
			// Translate matcher tool name
			if tm.Matcher != "" {
				tm.Matcher = TranslateTool(tm.Matcher, targetSlug)
			}
			copy(tm.Hooks, m.Hooks)

			var kept []hookEntry
			for _, h := range tm.Hooks {
				if h.Type == "prompt" || h.Type == "agent" {
					if llmMode == LLMHooksModeGenerate {
						scriptName, scriptContent := generateLLMWrapperScript(h, targetSlug, event, scriptIdx)
						scriptIdx++
						extraFiles[scriptName] = scriptContent
						kept = append(kept, hookEntry{
							Type:          "command",
							Command:       "./" + scriptName,
							Timeout:       30000, // LLM calls need more time
							StatusMessage: fmt.Sprintf("nesco-generated: LLM-evaluated hook (from %s)", h.Type),
						})
						warnings = append(warnings, fmt.Sprintf("LLM hook (type: %q) converted to wrapper script %s", h.Type, scriptName))
					} else {
						warnings = append(warnings, fmt.Sprintf("LLM-evaluated hook (type: %q) dropped for %s (use --llm-hooks=generate to create wrapper scripts)", h.Type, targetSlug))
					}
					continue
				}
				kept = append(kept, h)
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
	out := copilotHooksConfig{Hooks: make(map[string][]copilotHookEntry)}
	var warnings []string
	extraFiles := map[string][]byte{}
	scriptIdx := 0

	for event, matchers := range cfg.Hooks {
		targetEvent, supported := TranslateHookEvent(event, "copilot-cli")
		if !supported {
			warnings = append(warnings, fmt.Sprintf("hook event %q is not supported by copilot-cli (dropped)", event))
			continue
		}

		// Copilot doesn't support matchers — flatten all hooks
		var entries []copilotHookEntry
		for _, m := range matchers {
			if m.Matcher != "" {
				warnings = append(warnings, fmt.Sprintf("matcher %q dropped (copilot-cli does not support matchers)", m.Matcher))
			}
			for _, h := range m.Hooks {
				if h.Type == "prompt" || h.Type == "agent" {
					if llmMode == LLMHooksModeGenerate {
						scriptName, scriptContent := generateLLMWrapperScript(h, "copilot-cli", event, scriptIdx)
						scriptIdx++
						extraFiles[scriptName] = scriptContent
						entries = append(entries, copilotHookEntry{
							Bash:       "./" + scriptName,
							TimeoutSec: 30,
							Comment:    fmt.Sprintf("nesco-generated: LLM-evaluated hook (from %s)", h.Type),
						})
						warnings = append(warnings, fmt.Sprintf("LLM hook (type: %q) converted to wrapper script %s", h.Type, scriptName))
					} else {
						warnings = append(warnings, fmt.Sprintf("LLM-evaluated hook (type: %q) dropped for copilot-cli (use --llm-hooks=generate to create wrapper scripts)", h.Type))
					}
					continue
				}
				entry := copilotHookEntry{
					Bash:       h.Command,
					TimeoutSec: h.Timeout / 1000,
					Comment:    h.StatusMessage,
				}
				entries = append(entries, entry)
			}
		}

		if len(entries) > 0 {
			out.Hooks[targetEvent] = entries
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

// kiroHooksAgent is the shape of the nesco-hooks.json file Kiro reads.
type kiroHooksAgent struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Prompt      string                     `json:"prompt"`
	Hooks       map[string][]kiroHookEntry `json:"hooks"`
}

func renderKiroHooks(cfg hooksConfig, llmMode string) (*Result, error) {
	var warnings []string
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
				matcher = TranslateTool(m.Matcher, "kiro")
			}

			for _, h := range m.Hooks {
				if h.Type == "prompt" || h.Type == "agent" {
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
					continue
				}

				entry := kiroHookEntry{
					Command:   h.Command,
					Matcher:   matcher,
					TimeoutMs: h.Timeout,
				}
				kiroHooks[translated] = append(kiroHooks[translated], entry)
			}
		}
	}

	agent := kiroHooksAgent{
		Name:        "nesco-hooks",
		Description: "Hooks installed by nesco",
		Prompt:      "",
		Hooks:       kiroHooks,
	}

	result, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Content: result, Filename: "nesco-hooks.json", Warnings: warnings}, nil
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
func generateLLMWrapperScript(h hookEntry, targetSlug string, event string, idx int) (string, []byte) {
	scriptName := fmt.Sprintf("nesco-llm-hook-%s-%d.sh", sanitizeForFilename(event), idx)

	cli := cliCommands[targetSlug]
	if cli == "" {
		cli = "gemini" // fallback
	}

	prompt := h.Command
	if prompt == "" {
		prompt = "Evaluate this hook input and respond with a JSON decision."
	}

	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	b.WriteString("# nesco-generated: LLM-evaluated hook wrapper\n")
	b.WriteString(fmt.Sprintf("# Original type: %s | Event: %s\n", h.Type, event))
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
		b.WriteString(fmt.Sprintf("RESPONSE=$(%s -p %s --output-format json 2>/dev/null)\n", cli, escapedPrompt))
		b.WriteString("echo \"$RESPONSE\" | jq -r '.response'\n")
	case "claude-code":
		b.WriteString(fmt.Sprintf("RESPONSE=$(%s -p %s --output-format json 2>/dev/null)\n", cli, escapedPrompt))
		b.WriteString("echo \"$RESPONSE\" | jq -r '.result // .'\n")
	default:
		b.WriteString(fmt.Sprintf("RESPONSE=$(echo %s | %s 2>/dev/null)\n", escapedPrompt, cli))
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
