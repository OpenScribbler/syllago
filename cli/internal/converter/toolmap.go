package converter

import "strings"

// ToolNames maps canonical tool names (Claude Code) to provider-specific equivalents.
// Note: Zed and Cursor use "edit_file" for both Write and Edit; Codex uses "apply_patch"
// for both. Reverse translation is ambiguous and may return either canonical name;
// round-trips through these providers lose the Write/Edit distinction.
var ToolNames = map[string]map[string]string{
	"Read": {
		"gemini-cli":  "read_file",
		"copilot-cli": "view",
		"kiro":        "read",
		"opencode":    "read",
		"zed":         "read_file",
		"cline":       "read_file",
		"roo-code":    "read_file",
		"cursor":      "read_file",
		"windsurf":    "view_line_range",
		"codex":       "read_file",
	},
	"Write": {
		"gemini-cli":  "write_file",
		"copilot-cli": "create",
		"kiro":        "fs_write",
		"opencode":    "write",
		"zed":         "edit_file",
		"cline":       "write_to_file",
		"roo-code":    "write_to_file",
		"cursor":      "edit_file",
		"windsurf":    "write_to_file",
		"codex":       "apply_patch",
	},
	"Edit": {
		"gemini-cli":  "replace",
		"copilot-cli": "edit",
		"kiro":        "fs_write",
		"opencode":    "edit",
		"zed":         "edit_file",
		"cline":       "replace_in_file",
		"roo-code":    "replace_in_file",
		"cursor":      "edit_file",
		"windsurf":    "edit_file",
		"codex":       "apply_patch",
	},
	"Bash": {
		"gemini-cli":  "run_shell_command",
		"copilot-cli": "bash",
		"kiro":        "shell",
		"opencode":    "bash",
		"zed":         "terminal",
		"cline":       "execute_command",
		"roo-code":    "execute_command",
		"cursor":      "run_terminal_cmd",
		"windsurf":    "run_command",
		"codex":       "shell",
	},
	"Glob": {
		"gemini-cli":  "glob",
		"copilot-cli": "glob",
		"kiro":        "glob",
		"opencode":    "glob",
		"zed":         "find_path",
		"cline":       "list_files",
		"roo-code":    "list_files",
		"cursor":      "file_search",
		"windsurf":    "find_by_name",
		"codex":       "list_dir",
	},
	"Grep": {
		"gemini-cli":  "grep_search",
		"copilot-cli": "grep",
		"kiro":        "grep",
		"opencode":    "grep",
		"zed":         "grep",
		"cline":       "search_files",
		"roo-code":    "search_files",
		"cursor":      "grep_search",
		"windsurf":    "grep_search",
		"codex":       "grep_files",
	},
	"WebSearch": {
		"gemini-cli": "google_web_search",
		"opencode":   "websearch",
		"zed":        "web_search",
		"cursor":     "web_search",
		"windsurf":   "search_web",
		"codex":      "web_search",
		"kiro":       "web_search",
	},
	"Agent": {
		"copilot-cli": "task",
		"opencode":    "task",
		"zed":         "spawn_agent",
		"codex":       "spawn_agent",
		"kiro":        "use_subagent",
	},
	"WebFetch": {
		"gemini-cli":  "web_fetch",
		"copilot-cli": "web_fetch",
		"kiro":        "web_fetch",
		"opencode":    "webfetch",
		"zed":         "fetch",
		"windsurf":    "read_url_content",
	},
	// CC-only tools with no cross-provider equivalents
	"NotebookEdit":    {},
	"MultiEdit":       {},
	"LS":              {},
	"NotebookRead":    {},
	"KillBash":        {},
	"Skill":           {},
	"AskUserQuestion": {},
}

// HookEvents maps canonical event names (Claude Code) to provider-specific equivalents.
var HookEvents = map[string]map[string]string{
	"PreToolUse":       {"gemini-cli": "BeforeTool", "copilot-cli": "preToolUse", "kiro": "preToolUse", "cline": "PreToolUse"},
	"PostToolUse":      {"gemini-cli": "AfterTool", "copilot-cli": "postToolUse", "kiro": "postToolUse", "cline": "PostToolUse"},
	"UserPromptSubmit": {"gemini-cli": "BeforeAgent", "copilot-cli": "userPromptSubmitted", "kiro": "userPromptSubmit", "cline": "UserPromptSubmit"},
	"Stop":             {"gemini-cli": "AfterAgent", "kiro": "stop"},
	"SessionStart":     {"gemini-cli": "SessionStart", "copilot-cli": "sessionStart", "kiro": "agentSpawn", "cline": "TaskStart"},
	"SessionEnd":       {"gemini-cli": "SessionEnd", "copilot-cli": "sessionEnd", "cline": "TaskComplete"},
	"PreCompact":       {"gemini-cli": "PreCompress", "cline": "PreCompact"},
	"Notification":     {"gemini-cli": "Notification"},
	"SubagentStart":    {},
	"SubagentStop":     {"copilot-cli": "subagentStop"},
	"AgentStop":        {"copilot-cli": "agentStop"},
	"ErrorOccurred":    {"copilot-cli": "errorOccurred"},

	// CC-only events (empty maps — no cross-provider equivalents)
	"PostToolUseFailure": {},
	"PermissionRequest":  {},
	"PostCompact":        {},
	"InstructionsLoaded": {},
	"ConfigChange":       {},
	"WorktreeCreate":     {},
	"WorktreeRemove":     {},
	"Elicitation":        {},
	"ElicitationResult":  {},
	"TeammateIdle":       {},
	"TaskCompleted":      {},
	"StopFailure":        {},

	// Gemini-only events
	"BeforeModel":         {"gemini-cli": "BeforeModel"},
	"AfterModel":          {"gemini-cli": "AfterModel"},
	"BeforeToolSelection": {"gemini-cli": "BeforeToolSelection"},

	// Cline-only events
	"TaskResume": {"cline": "TaskResume"},
	"TaskCancel": {"cline": "TaskCancel"},
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
// When multiple canonical names map to the same provider tool (e.g., Cursor/Zed use
// "edit_file" for both Write and Edit, Codex uses "apply_patch" for both), prefers
// Edit over Write since Edit is the more specific operation.
// Also handles backwards compatibility: "Task" was renamed to "Agent" in Claude Code v2.1.63.
func ReverseTranslateTool(name, sourceSlug string) string {
	var match string
	for canonical, m := range ToolNames {
		if provName, ok := m[sourceSlug]; ok && provName == name {
			if canonical == "Edit" {
				return "Edit" // prefer Edit over Write for ambiguous mappings
			}
			match = canonical
		}
	}
	if match != "" {
		return match
	}
	// Backwards compat: "Task" as a canonical name maps to "Agent".
	if strings.EqualFold(name, "task") {
		return "Agent"
	}
	return name
}

// TranslateMatcher translates a hook matcher string to the target provider.
// Matchers can be:
//   - Simple tool names: "Bash" → translate directly
//   - Regex alternations: "Edit|Write" → split, translate each, rejoin
//   - Wildcard patterns: "mcp__github__.*" → strip .*, translate, reattach
//   - MCP prefix patterns: "mcp__.*" → pass through unchanged
func TranslateMatcher(matcher, targetSlug string) string {
	parts := strings.Split(matcher, "|")
	for i, part := range parts {
		parts[i] = translateMatcherComponent(part, targetSlug)
	}
	return strings.Join(parts, "|")
}

// ReverseTranslateMatcher translates a provider-specific hook matcher to canonical.
// Handles the same patterns as TranslateMatcher but in reverse.
func ReverseTranslateMatcher(matcher, sourceSlug string) string {
	parts := strings.Split(matcher, "|")
	for i, part := range parts {
		parts[i] = reverseTranslateMatcherComponent(part, sourceSlug)
	}
	return strings.Join(parts, "|")
}

// translateMatcherComponent translates a single matcher component (no |) to the target.
func translateMatcherComponent(comp, targetSlug string) string {
	// Bare wildcard patterns like ".*" — pass through unchanged
	if comp == ".*" || comp == "*" {
		return comp
	}

	// Strip .* suffix if present, translate the base, reattach
	hasSuffix := strings.HasSuffix(comp, ".*")
	base := comp
	if hasSuffix {
		base = strings.TrimSuffix(comp, ".*")
	}

	// MCP-style prefixes (e.g., "mcp__github__create_issue") — pass through unchanged.
	// These are server-specific tool names that don't map through ToolNames.
	if strings.Contains(base, "__") || strings.Contains(base, "/") || strings.Contains(base, ":") {
		return comp
	}

	translated := TranslateTool(base, targetSlug)
	if hasSuffix {
		return translated + ".*"
	}
	return translated
}

// reverseTranslateMatcherComponent reverse-translates a single matcher component.
func reverseTranslateMatcherComponent(comp, sourceSlug string) string {
	if comp == ".*" || comp == "*" {
		return comp
	}

	hasSuffix := strings.HasSuffix(comp, ".*")
	base := comp
	if hasSuffix {
		base = strings.TrimSuffix(comp, ".*")
	}

	if strings.Contains(base, "__") || strings.Contains(base, "/") || strings.Contains(base, ":") {
		return comp
	}

	translated := ReverseTranslateTool(base, sourceSlug)
	if hasSuffix {
		return translated + ".*"
	}
	return translated
}

// TranslateMCPToolName translates MCP tool name format between providers.
// Providers group into four patterns:
//   - Prefixed double-underscore: claude-code, kiro → mcp__server__tool
//   - Bare double-underscore: opencode, cline, roo-code, cursor, windsurf → server__tool
//   - Slash-separated: copilot-cli, codex → server/tool
//   - Colon-separated: zed → mcp:server:tool
//   - mcp_ prefix single-underscore: gemini-cli → mcp_server_tool
func TranslateMCPToolName(name, sourceSlug, targetSlug string) string {
	server, tool := parseMCPToolName(name, sourceSlug)
	if server == "" {
		return name // not an MCP tool name
	}

	switch targetSlug {
	case "claude-code", "kiro":
		return "mcp__" + server + "__" + tool
	case "gemini-cli":
		return "mcp_" + server + "_" + tool
	case "opencode", "cline", "roo-code", "cursor", "windsurf":
		return server + "__" + tool
	case "copilot-cli", "codex":
		return server + "/" + tool
	case "zed":
		return "mcp:" + server + ":" + tool
	default:
		return name
	}
}

// parseMCPToolName extracts server and tool from a provider-specific MCP tool name.
// Providers group into four parsing patterns by separator format.
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
	case "gemini-cli":
		// mcp_server_tool — prefix "mcp_", then split on first "_"
		if !strings.HasPrefix(name, "mcp_") {
			return "", ""
		}
		rest := strings.TrimPrefix(name, "mcp_")
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	case "opencode", "cline", "roo-code", "cursor", "windsurf":
		// server__tool
		parts := strings.SplitN(name, "__", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	case "copilot-cli", "codex":
		// server/tool
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	case "zed":
		// mcp:server:tool
		if !strings.HasPrefix(name, "mcp:") {
			return "", ""
		}
		rest := strings.TrimPrefix(name, "mcp:")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	default:
		return "", ""
	}
}
