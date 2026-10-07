package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// HookEntry represents a single hook action in syllago canonical format.
// Timeout is in seconds (canonical unit).
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

// DetectHookFormat classifies a hook JSON payload by its top-level shape:
//   - "manifest": canonical hooks/0.1 shape ({"spec":"hooks/0.1","hooks":[...]})
//   - "flat":     legacy flat ({"event":"...","matcher":"...","hooks":[...]})
//   - "nested":   provider settings shape ({"hooks":{"EventName":[...]}})
func DetectHookFormat(content []byte) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return "nested" // default
	}
	// encoding/json matches keys case-insensitively, so probing through a
	// struct classifies "Spec" the way ParseManifest reads it. A map lookup
	// would not, and the scanner would then read a manifest as another shape.
	var probe struct {
		Spec string `json:"spec"`
	}
	if json.Unmarshal(content, &probe) == nil && strings.HasPrefix(probe.Spec, "hooks/") {
		return "manifest"
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

// LLMHooksModeSkip drops LLM-evaluated hooks with a warning (default).
const LLMHooksModeSkip = "skip"

// LLMHooksModeGenerate generates wrapper scripts that call the target provider's CLI.
const LLMHooksModeGenerate = "generate"

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
