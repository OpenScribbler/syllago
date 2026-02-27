package converter

import "strings"

// ToolNames maps canonical tool names (Claude Code) to provider-specific equivalents.
// Note: Zed uses "edit_file" for both Write and Edit. Reverse translation is ambiguous
// and may return either canonical name; round-trips through Zed lose the distinction.
var ToolNames = map[string]map[string]string{
	"Read": {
		"gemini-cli":  "read_file",
		"copilot-cli": "view",
		"kiro":        "read",
		"opencode":    "view",
		"zed":         "read_file",
		"cline":       "read_file",
		"roo-code":    "ReadFileTool",
	},
	"Write": {
		"gemini-cli":  "write_file",
		"copilot-cli": "apply_patch",
		"kiro":        "fs_write",
		"opencode":    "write",
		"zed":         "edit_file",
		"cline":       "write_to_file",
		"roo-code":    "WriteToFileTool",
	},
	"Edit": {
		"gemini-cli":  "replace",
		"copilot-cli": "apply_patch",
		"kiro":        "fs_write",
		"opencode":    "edit",
		"zed":         "edit_file",
		"cline":       "apply_diff",
		"roo-code":    "EditFileTool",
	},
	"Bash": {
		"gemini-cli":  "run_shell_command",
		"copilot-cli": "shell",
		"kiro":        "shell",
		"opencode":    "bash",
		"zed":         "terminal",
		"cline":       "execute_command",
		"roo-code":    "ExecuteCommandTool",
	},
	"Glob": {
		"gemini-cli":  "list_directory",
		"copilot-cli": "glob",
		"kiro":        "read",
		"opencode":    "glob",
		"zed":         "find_path",
		"cline":       "list_files",
		"roo-code":    "ListFilesTool",
	},
	"Grep": {
		"gemini-cli":  "grep_search",
		"copilot-cli": "rg",
		"kiro":        "read",
		"opencode":    "grep",
		"zed":         "grep",
		"cline":       "search_files",
		"roo-code":    "SearchFilesTool",
	},
	"WebSearch": {
		"gemini-cli": "google_search",
		"opencode":   "fetch",
		"zed":        "web_search",
	},
	"Task": {
		"copilot-cli": "task",
		"opencode":    "agent",
		"zed":         "subagent",
	},
}

// HookEvents maps canonical event names (Claude Code) to provider-specific equivalents.
var HookEvents = map[string]map[string]string{
	"PreToolUse":        {"gemini-cli": "BeforeTool", "copilot-cli": "preToolUse", "kiro": "preToolUse"},
	"PostToolUse":       {"gemini-cli": "AfterTool", "copilot-cli": "postToolUse", "kiro": "postToolUse"},
	"UserPromptSubmit":  {"gemini-cli": "BeforeAgent", "copilot-cli": "userPromptSubmitted", "kiro": "userPromptSubmit"},
	"Stop":              {"gemini-cli": "AfterAgent", "kiro": "stop"},
	"SessionStart":      {"gemini-cli": "SessionStart", "copilot-cli": "sessionStart", "kiro": "agentSpawn"},
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
// Providers group into three patterns:
//   - Prefixed double-underscore: claude-code, kiro → mcp__server__tool
//   - Bare double-underscore: gemini-cli, opencode, cline, roo-code → server__tool
//   - Slash-separated: copilot-cli, zed → server/tool
func TranslateMCPToolName(name, sourceSlug, targetSlug string) string {
	server, tool := parseMCPToolName(name, sourceSlug)
	if server == "" {
		return name // not an MCP tool name
	}

	switch targetSlug {
	case "claude-code", "kiro":
		return "mcp__" + server + "__" + tool
	case "gemini-cli", "opencode", "cline", "roo-code":
		return server + "__" + tool
	case "copilot-cli", "zed":
		return server + "/" + tool
	default:
		return name
	}
}

// parseMCPToolName extracts server and tool from a provider-specific MCP tool name.
// Providers group into three parsing patterns by separator format.
func parseMCPToolName(name, sourceSlug string) (server, tool string) {
	switch sourceSlug {
	case "claude-code", "kiro":
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
	case "gemini-cli", "opencode", "cline", "roo-code":
		// server__tool
		parts := strings.SplitN(name, "__", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	case "copilot-cli", "zed":
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
