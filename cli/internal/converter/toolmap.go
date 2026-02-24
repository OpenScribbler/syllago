package converter

import "strings"

// ToolNames maps canonical tool names (Claude Code) to provider-specific equivalents.
var ToolNames = map[string]map[string]string{
	"Read":      {"gemini-cli": "read_file", "copilot-cli": "view"},
	"Write":     {"gemini-cli": "write_file", "copilot-cli": "apply_patch"},
	"Edit":      {"gemini-cli": "replace", "copilot-cli": "apply_patch"},
	"Bash":      {"gemini-cli": "run_shell_command", "copilot-cli": "shell"},
	"Glob":      {"gemini-cli": "list_directory", "copilot-cli": "glob"},
	"Grep":      {"gemini-cli": "grep_search", "copilot-cli": "rg"},
	"WebSearch": {"gemini-cli": "google_search"},
	"Task":      {"copilot-cli": "task"},
}

// HookEvents maps canonical event names (Claude Code) to provider-specific equivalents.
var HookEvents = map[string]map[string]string{
	"PreToolUse":        {"gemini-cli": "BeforeTool", "copilot-cli": "preToolUse"},
	"PostToolUse":       {"gemini-cli": "AfterTool", "copilot-cli": "postToolUse"},
	"UserPromptSubmit":  {"gemini-cli": "BeforeAgent", "copilot-cli": "userPromptSubmitted"},
	"Stop":              {"gemini-cli": "AfterAgent"},
	"SessionStart":      {"gemini-cli": "SessionStart", "copilot-cli": "sessionStart"},
	"SessionEnd":        {"gemini-cli": "SessionEnd", "copilot-cli": "sessionEnd"},
	"PreCompact":        {"gemini-cli": "PreCompress"},
	"Notification":      {"gemini-cli": "Notification"},
	"SubagentStart":     {},
	"SubagentCompleted": {},
}

// TranslateTool translates a single canonical tool name to the target provider.
// Returns the original name if no mapping exists.
func TranslateTool(name, targetSlug string) string {
	if m, ok := ToolNames[name]; ok {
		if translated, ok := m[targetSlug]; ok {
			return translated
		}
	}
	return name
}

// TranslateTools batch-translates canonical tool names to the target provider.
func TranslateTools(names []string, targetSlug string) []string {
	result := make([]string, len(names))
	for i, name := range names {
		result[i] = TranslateTool(name, targetSlug)
	}
	return result
}

// TranslateHookEvent translates a canonical hook event to the target provider.
// Returns the translated name and whether the target supports this event.
func TranslateHookEvent(event, targetSlug string) (string, bool) {
	m, ok := HookEvents[event]
	if !ok {
		return event, false
	}
	translated, supported := m[targetSlug]
	if !supported {
		return event, false
	}
	return translated, true
}

// ReverseTranslateHookEvent finds the canonical event name from a provider-specific one.
func ReverseTranslateHookEvent(event, sourceSlug string) string {
	for canonical, m := range HookEvents {
		if provName, ok := m[sourceSlug]; ok && provName == event {
			return canonical
		}
	}
	return event
}

// ReverseTranslateTool finds the canonical tool name from a provider-specific one.
func ReverseTranslateTool(name, sourceSlug string) string {
	for canonical, m := range ToolNames {
		if provName, ok := m[sourceSlug]; ok && provName == name {
			return canonical
		}
	}
	return name
}

// TranslateMCPToolName translates MCP tool name format between providers.
// Claude: mcp__server__tool, Gemini: server__tool, Copilot: server/tool
func TranslateMCPToolName(name, sourceSlug, targetSlug string) string {
	// Normalize to parts: server, tool
	server, tool := parseMCPToolName(name, sourceSlug)
	if server == "" {
		return name // not an MCP tool name
	}

	switch targetSlug {
	case "claude-code":
		return "mcp__" + server + "__" + tool
	case "gemini-cli":
		return server + "__" + tool
	case "copilot-cli":
		return server + "/" + tool
	default:
		return name
	}
}

// parseMCPToolName extracts server and tool from a provider-specific MCP tool name.
func parseMCPToolName(name, sourceSlug string) (server, tool string) {
	switch sourceSlug {
	case "claude-code":
		// mcp__server__tool
		if !strings.HasPrefix(name, "mcp__") {
			return "", ""
		}
		rest := strings.TrimPrefix(name, "mcp__")
		parts := strings.SplitN(rest, "__", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	case "gemini-cli":
		// server__tool
		parts := strings.SplitN(name, "__", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	case "copilot-cli":
		// server/tool
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	default:
		return "", ""
	}
}
