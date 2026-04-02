package analyzer

// knownHookEventNames contains all known hook event names across all providers.
// These are hardcoded in the binary — never loaded from converter packages.
// Converters contribute display metadata only; scoring vocabulary is immutable.
//
// This list must stay consistent with converter.HookEvents in toolmap.go,
// but is intentionally separate: vocabulary is for the content-signal detector's
// scoring engine, not for format conversion.
var knownHookEventNames = map[string]bool{
	// Claude Code native
	"PreToolUse":         true,
	"PostToolUse":        true,
	"SessionStart":       true,
	"SessionEnd":         true,
	"UserPromptSubmit":   true,
	"Stop":               true,
	"PreCompact":         true,
	"PostCompact":        true,
	"Notification":       true,
	"SubagentStart":      true,
	"SubagentStop":       true,
	"ErrorOccurred":      true,
	"PostToolUseFailure": true,
	"PermissionRequest":  true,
	"InstructionsLoaded": true,
	"ConfigChange":       true,
	"WorktreeCreate":     true,
	"WorktreeRemove":     true,
	"Elicitation":        true,
	"ElicitationResult":  true,
	"TeammateIdle":       true,
	"TaskCompleted":      true,
	"StopFailure":        true,
	"FileChanged":        true,

	// Gemini CLI native
	"BeforeTool":          true,
	"AfterTool":           true,
	"BeforeAgent":         true,
	"AfterAgent":          true,
	"BeforeModel":         true,
	"AfterModel":          true,
	"BeforeToolSelection": true,
	"PreCompress":         true,

	// Copilot CLI native
	"preToolUse":          true,
	"postToolUse":         true,
	"userPromptSubmitted": true,
	"sessionStart":        true,
	"sessionEnd":          true,
	"agentStop":           true,
	"subagentStop":        true,
	"errorOccurred":       true,

	// Cursor native
	"beforeAgentResponse": true,
	"afterAgentResponse":  true,
	"beforeToolSelection": true,
	"postToolUseFailure":  true,
	"afterFileEdit":       true,

	// Windsurf native
	"pre_user_prompt":                       true,
	"post_cascade_response":                 true,
	"post_setup_worktree":                   true,
	"post_cascade_response_with_transcript": true,

	// OpenCode native
	"tool.execute.before": true,
	"tool.execute.after":  true,
	"session.created":     true,
	"session.idle":        true,
	"session.error":       true,
	"permission.asked":    true,
	"file.edited":         true,

	// Kiro native
	"agentSpawn":          true,
	"Pre Task Execution":  true,
	"Post Task Execution": true,
	"File Save":           true,
	"File Create":         true,
	"File Delete":         true,

	// Pi native
	"tool_call":              true,
	"tool_result":            true,
	"input":                  true,
	"agent_end":              true,
	"session_shutdown":       true,
	"session_before_compact": true,
	"before_agent_start":     true,
	"turn_start":             true,
	"turn_end":               true,
	"model_select":           true,
	"user_bash":              true,
	"context":                true,
	"message_start":          true,
	"message_end":            true,

	// Syllago canonical (provider-neutral)
	"before_tool_execute":   true,
	"after_tool_execute":    true,
	"session_start":         true,
	"session_end":           true,
	"before_prompt":         true,
	"agent_stop":            true,
	"before_compact":        true,
	"after_compact":         true,
	"notification":          true,
	"subagent_start":        true,
	"subagent_stop":         true,
	"error_occurred":        true,
	"tool_use_failure":      true,
	"permission_request":    true,
	"instructions_loaded":   true,
	"config_change":         true,
	"worktree_create":       true,
	"worktree_remove":       true,
	"elicitation":           true,
	"elicitation_result":    true,
	"teammate_idle":         true,
	"task_completed":        true,
	"stop_failure":          true,
	"file_changed":          true,
	"before_model":          true,
	"after_model":           true,
	"before_tool_selection": true,
	"file_created":          true,
	"file_deleted":          true,
	"before_task":           true,
	"after_task":            true,
	"transcript_export":     true,
	"context_update":        true,
	"pre_run_command":       true,
	"post_run_command":      true,
	"pre_read_code":         true,
	"pre_write_code":        true,
}

// directoryKeywords are directory name substrings that indicate AI content presence.
// Used by the content-signal detector's pre-filter.
var directoryKeywords = []string{
	"agent", "skill", "rule", "hook", "command",
	"mcp", "prompt", "steering", "pack", "workflow",
}

// contentSignalExtensions are the only file extensions the content-signal detector inspects.
var contentSignalExtensions = map[string]bool{
	".md":   true,
	".yaml": true,
	".yml":  true,
	".json": true,
	".toml": true,
}
