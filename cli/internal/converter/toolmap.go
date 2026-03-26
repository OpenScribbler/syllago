package converter

import "strings"

// ToolNames maps canonical (provider-neutral) tool names to provider-specific equivalents.
// Keys are snake_case neutral names; every provider including claude-code has an explicit entry.
// Note: Zed and Cursor use "edit_file" for both file_write and file_edit; Codex uses "apply_patch"
// for both. Reverse translation is ambiguous and may return either canonical name;
// round-trips through these providers lose the file_write/file_edit distinction.
var ToolNames = map[string]map[string]string{
	"file_read": {
		"claude-code": "Read",
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
	"file_write": {
		"claude-code": "Write",
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
	"file_edit": {
		"claude-code": "Edit",
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
	"shell": {
		"claude-code": "Bash",
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
	"find": {
		"claude-code": "Glob",
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
	"search": {
		"claude-code": "Grep",
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
	"web_search": {
		"claude-code": "WebSearch",
		"gemini-cli":  "google_web_search",
		"opencode":    "websearch",
		"zed":         "web_search",
		"cursor":      "web_search",
		"windsurf":    "search_web",
		"codex":       "web_search",
		"kiro":        "web_search",
	},
	"agent": {
		"claude-code": "Agent",
		"copilot-cli": "task",
		"opencode":    "task",
		"zed":         "spawn_agent",
		"codex":       "spawn_agent",
		"kiro":        "use_subagent",
	},
	"web_fetch": {
		"claude-code": "WebFetch",
		"gemini-cli":  "web_fetch",
		"copilot-cli": "web_fetch",
		"kiro":        "web_fetch",
		"opencode":    "webfetch",
		"zed":         "fetch",
		"windsurf":    "read_url_content",
	},
	// CC-only tools with no cross-provider equivalents
	"notebook_edit": {"claude-code": "NotebookEdit"},
	"multi_edit":    {"claude-code": "MultiEdit"},
	"list_dir":      {"claude-code": "LS"},
	"notebook_read": {"claude-code": "NotebookRead"},
	"kill_shell":    {"claude-code": "KillBash"},
	"skill":         {"claude-code": "Skill"},
	"ask_user":      {"claude-code": "AskUserQuestion"},
}

// HookEvents maps canonical (provider-neutral) event names to provider-specific equivalents.
// Keys are snake_case neutral names; every provider including claude-code has an explicit entry.
var HookEvents = map[string]map[string]string{
	"before_tool_execute": {"claude-code": "PreToolUse", "gemini-cli": "BeforeTool", "copilot-cli": "preToolUse", "kiro": "preToolUse", "cline": "PreToolUse", "cursor": "PreToolUse"},
	"after_tool_execute":  {"claude-code": "PostToolUse", "gemini-cli": "AfterTool", "copilot-cli": "postToolUse", "kiro": "postToolUse", "cline": "PostToolUse", "cursor": "PostToolUse"},
	"before_prompt":       {"claude-code": "UserPromptSubmit", "gemini-cli": "BeforeAgent", "copilot-cli": "userPromptSubmitted", "kiro": "userPromptSubmit", "cline": "UserPromptSubmit", "cursor": "UserPromptSubmit"},
	"agent_stop":          {"claude-code": "Stop", "gemini-cli": "AfterAgent", "kiro": "stop", "copilot-cli": "agentStop", "cursor": "Stop"},
	"session_start":       {"claude-code": "SessionStart", "gemini-cli": "SessionStart", "copilot-cli": "sessionStart", "kiro": "agentSpawn", "cline": "TaskStart", "cursor": "SessionStart"},
	"session_end":         {"claude-code": "SessionEnd", "gemini-cli": "SessionEnd", "copilot-cli": "sessionEnd", "cline": "TaskComplete", "cursor": "SessionEnd"},
	"before_compact":      {"claude-code": "PreCompact", "gemini-cli": "PreCompress", "cline": "PreCompact", "cursor": "PreCompact"},
	"notification":        {"claude-code": "Notification", "gemini-cli": "Notification"},
	"subagent_start":      {"claude-code": "SubagentStart"},
	"subagent_stop":       {"claude-code": "SubagentStop", "copilot-cli": "subagentStop"},
	"error_occurred":      {"claude-code": "ErrorOccurred", "copilot-cli": "errorOccurred"},

	// CC-only events (no cross-provider equivalents)
	"after_tool_failure":  {"claude-code": "PostToolUseFailure"},
	"permission_request":  {"claude-code": "PermissionRequest"},
	"after_compact":       {"claude-code": "PostCompact"},
	"instructions_loaded": {"claude-code": "InstructionsLoaded"},
	"config_change":       {"claude-code": "ConfigChange"},
	"worktree_create":     {"claude-code": "WorktreeCreate"},
	"worktree_remove":     {"claude-code": "WorktreeRemove"},
	"elicitation":         {"claude-code": "Elicitation"},
	"elicitation_result":  {"claude-code": "ElicitationResult"},
	"teammate_idle":       {"claude-code": "TeammateIdle"},
	"task_completed":      {"claude-code": "TaskCompleted"},
	"stop_failure":        {"claude-code": "StopFailure"},

	// Gemini-only events
	"before_model":          {"gemini-cli": "BeforeModel"},
	"after_model":           {"gemini-cli": "AfterModel"},
	"before_tool_selection": {"gemini-cli": "BeforeToolSelection"},

	// Cline-only events
	"task_resume": {"cline": "TaskResume"},
	"task_cancel": {"cline": "TaskCancel"},
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

// IsValidHookEvent reports whether the event name is a known canonical or
// provider-native hook event. This prevents sjson key injection via dots or
// other special characters in crafted event names.
func IsValidHookEvent(event string) bool {
	// Check canonical names (map keys)
	if _, ok := HookEvents[event]; ok {
		return true
	}
	// Check all provider-native names (map values)
	for _, provMap := range HookEvents {
		for _, native := range provMap {
			if native == event {
				return true
			}
		}
	}
	return false
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
// "edit_file" for both file_write and file_edit, Codex uses "apply_patch" for both),
// prefers file_edit over file_write since edit is the more specific operation.
// Also handles backwards compatibility: "Task" was renamed to "Agent" in Claude Code v2.1.63.
func ReverseTranslateTool(name, sourceSlug string) string {
	var match string
	for canonical, m := range ToolNames {
		if provName, ok := m[sourceSlug]; ok && provName == name {
			if canonical == "file_edit" {
				return "file_edit" // prefer file_edit over file_write for ambiguous mappings
			}
			match = canonical
		}
	}
	if match != "" {
		return match
	}
	// Backwards compat: CC's legacy "Task" tool name maps to "agent".
	if sourceSlug == "claude-code" && strings.EqualFold(name, "task") {
		return "agent"
	}
	return name
}

// TranslateMatcher translates a hook matcher string to the target provider.
// Matchers can be:
//   - Simple tool names: "shell" → translate directly
//   - Regex alternations: "file_edit|file_write" → split, translate each, rejoin
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
