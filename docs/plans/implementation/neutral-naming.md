# Implementation Plan: Neutral Naming for Content Types

**Bead:** syllago-g9kd
**Parent design:** `docs/plans/2026-03-22-hook-canonical-spec-design.md` (section: "Scope of Neutral Naming")
**Status:** Plan (no code changes)

---

## Summary

The canonical format currently uses Claude Code names as the "neutral" keys in `ToolNames` and `HookEvents` maps (e.g., `"Read"`, `"Bash"`, `"PreToolUse"`). The design doc specifies moving to provider-neutral snake_case names (e.g., `"file_read"`, `"shell"`, `"before_tool_execute"`). This plan covers the naming migration for **all content types**: hooks, skills, agents, commands, and MCP.

---

## 1. Audit of Current CC-Biased Names

### 1a. Tool Names (`ToolNames` map in toolmap.go)

Current canonical keys are Claude Code tool names:

| Current Key (CC-biased) | Used In |
|--------------------------|---------|
| `Read` | skills (allowed-tools, disallowed-tools), agents (tools, disallowedTools), commands (allowed-tools), hooks (matchers) |
| `Write` | same |
| `Edit` | same |
| `Bash` | same |
| `Glob` | same |
| `Grep` | same |
| `WebSearch` | same |
| `WebFetch` | same |
| `Agent` | same |
| `NotebookEdit` | CC-only (no cross-provider mapping) |
| `MultiEdit` | CC-only |
| `LS` | CC-only |
| `NotebookRead` | CC-only |
| `KillBash` | CC-only |
| `Skill` | CC-only |
| `AskUserQuestion` | CC-only |

### 1b. Hook Event Names (`HookEvents` map in toolmap.go)

Current canonical keys are Claude Code event names:

| Current Key (CC-biased) | Used In |
|--------------------------|---------|
| `PreToolUse` | hooks.go canonicalize/render, flat hook parsing |
| `PostToolUse` | same |
| `UserPromptSubmit` | same |
| `Stop` | same |
| `SessionStart` | same |
| `SessionEnd` | same |
| `PreCompact` | same |
| `Notification` | same |
| `SubagentStart` | same |
| `SubagentStop` | same |
| `AgentStop` | same |
| `ErrorOccurred` | same |
| `PostToolUseFailure` | CC-only |
| `PermissionRequest` | CC-only |
| `PostCompact` | CC-only |
| `InstructionsLoaded` | CC-only |
| `ConfigChange` | CC-only |
| `WorktreeCreate` | CC-only |
| `WorktreeRemove` | CC-only |
| `Elicitation` | CC-only |
| `ElicitationResult` | CC-only |
| `TeammateIdle` | CC-only |
| `TaskCompleted` | CC-only |
| `StopFailure` | CC-only |
| `BeforeModel` | Gemini-only |
| `AfterModel` | Gemini-only |
| `BeforeToolSelection` | Gemini-only |
| `TaskResume` | Cline-only |
| `TaskCancel` | Cline-only |

### 1c. Skill Frontmatter Fields (skills.go)

`SkillMeta` struct uses Claude Code YAML field names as canonical:

| Field | CC-biased? | Notes |
|-------|------------|-------|
| `allowed-tools` | **Yes** — values are CC tool names | Tool names inside the list are CC-biased |
| `disallowed-tools` | **Yes** — same | Same |
| `context` | Neutral | Values like `"fork"` are CC-specific but the concept is generic |
| `agent` | Neutral-ish | References CC agent names (e.g., "Explore") |
| `model` | Neutral | Model identifiers are universal |
| `effort` | Neutral | CC-originated but semantically generic |
| `disable-model-invocation` | Neutral | Boolean, concept is universal |
| `user-invocable` | Neutral | Boolean, concept is universal |
| `argument-hint` | Neutral | String, concept is universal |
| `hooks` | Neutral | Opaque blob, event names inside are CC-biased |

### 1d. Agent Frontmatter Fields (agents.go)

`AgentMeta` struct uses mostly CC names:

| Field | CC-biased? | Notes |
|-------|------------|-------|
| `tools` | **Yes** — values are CC tool names | Tool names in the list are CC-biased |
| `disallowedTools` | **Yes** — same | Same |
| `permissionMode` | **Yes** — values like `"plan"`, `"acceptEdits"` are CC terminology | |
| `skills` | **Yes** — references CC skill names | |
| `mcpServers` | Neutral-ish | camelCase is CC convention but widely adopted |
| `memory` | CC-specific | Concept exists only in CC |
| `background` | CC-specific | CC-specific sub-agent feature |
| `isolation` | **Yes** — value `"worktree"` is CC-specific | |
| `color` | CC-specific | UI feature |
| `hooks` | Neutral | Opaque blob |

### 1e. Command Frontmatter Fields (commands.go)

`CommandMeta` struct:

| Field | CC-biased? | Notes |
|-------|------------|-------|
| `allowed-tools` | **Yes** — values are CC tool names | Same as skills |
| `context` | Same as skills | |
| `agent` | Same as skills | |
| `model` | Neutral | |
| `disable-model-invocation` | Neutral | |
| `user-invocable` | Neutral | |
| `argument-hint` | Neutral | |

### 1f. MCP Config Fields (mcp.go)

`mcpServerConfig` struct:

| Field | CC-biased? | Notes |
|-------|------------|-------|
| `autoApprove` | **Yes** — CC-specific name for tool auto-approval | Cline/Roo use `alwaysAllow` |
| `disabledTools` | **Yes** — values are tool name strings | Added in f0d9502. Kiro/Windsurf/Roo capture and emit. Values may be CC-biased tool names. |
| `mcpServers` (top-level key) | Neutral-ish | Used by CC, Cursor, Kiro, Roo, Cline |
| All other fields | Neutral | `command`, `args`, `env`, `url`, etc. are generic |

**Note (concurrent session):** The following fields were added in another session (commits 845cc2f-e6d093d) and must be accounted for in this migration:
- `SkillMeta.License`, `.Compatibility`, `.Metadata` — neutral field names, no change needed
- `mcpServerConfig.DisabledTools` — tool name values need neutral-naming treatment (same pattern as `autoApprove` tool list values)
- `CommandMeta.Effort` — neutral field name, no change needed
- Agent `permissionMode` 5 CC-specific values — already flagged above
- MCP transport type mapping `http` <-> `streamable-http` — not a naming issue, but a mapping that adapters already handle

---

## 2. Proposed Neutral Names

### 2a. Tool Names (canonical map keys)

| Current (CC) | Proposed (Neutral) | Rationale |
|--------------|--------------------|-----------|
| `Read` | `file_read` | Describes the action, not the tool |
| `Write` | `file_write` | Same |
| `Edit` | `file_edit` | Same |
| `Bash` | `shell` | Provider-neutral; most tools call this "shell" or "terminal" |
| `Glob` | `find` | "Find files by pattern" — matches semantic intent |
| `Grep` | `search` | "Search file contents" |
| `WebSearch` | `web_search` | snake_case normalization |
| `WebFetch` | `web_fetch` | snake_case normalization |
| `Agent` | `agent` | Already neutral, just lowercase |
| `NotebookEdit` | `notebook_edit` | CC-only, snake_case normalization |
| `MultiEdit` | `multi_edit` | CC-only, snake_case normalization |
| `LS` | `list_dir` | CC-only, descriptive |
| `NotebookRead` | `notebook_read` | CC-only, snake_case normalization |
| `KillBash` | `kill_shell` | CC-only, follows `shell` rename |
| `Skill` | `skill` | CC-only, lowercase |
| `AskUserQuestion` | `ask_user` | CC-only, simplified |

These match the design doc's tool vocabulary table exactly (section "Tool Vocabulary").

### 2b. Hook Event Names (canonical map keys)

| Current (CC) | Proposed (Neutral) | Rationale |
|--------------|--------------------|-----------|
| `PreToolUse` | `before_tool_execute` | Describes lifecycle moment |
| `PostToolUse` | `after_tool_execute` | Same |
| `UserPromptSubmit` | `before_prompt` | Same |
| `Stop` | `agent_stop` | More specific |
| `SessionStart` | `session_start` | snake_case normalization |
| `SessionEnd` | `session_end` | Same |
| `PreCompact` | `before_compact` | Same |
| `Notification` | `notification` | Lowercase |
| `SubagentStart` | `subagent_start` | snake_case |
| `SubagentStop` | `subagent_stop` | snake_case |
| `AgentStop` | `agent_stop_copilot` | Disambiguate from CC's `Stop` (both map to same neutral) |
| `ErrorOccurred` | `error_occurred` | snake_case |
| `PostToolUseFailure` | `after_tool_failure` | CC-only, descriptive |
| `PermissionRequest` | `permission_request` | CC-only, snake_case |
| `PostCompact` | `after_compact` | CC-only |
| `InstructionsLoaded` | `instructions_loaded` | CC-only |
| `ConfigChange` | `config_change` | CC-only |
| `WorktreeCreate` | `worktree_create` | CC-only |
| `WorktreeRemove` | `worktree_remove` | CC-only |
| `Elicitation` | `elicitation` | CC-only |
| `ElicitationResult` | `elicitation_result` | CC-only |
| `TeammateIdle` | `teammate_idle` | CC-only |
| `TaskCompleted` | `task_completed` | CC-only |
| `StopFailure` | `stop_failure` | CC-only |
| `BeforeModel` | `before_model` | Gemini-only |
| `AfterModel` | `after_model` | Gemini-only |
| `BeforeToolSelection` | `before_tool_selection` | Gemini-only |
| `TaskResume` | `task_resume` | Cline-only |
| `TaskCancel` | `task_cancel` | Cline-only |

**Note on `Stop` vs `AgentStop`:** In the current map, `Stop` (CC's agent loop end) and `AgentStop` (Copilot CLI's `agentStop`) are separate entries. In neutral naming, both semantically mean "agent stopped." The design doc maps them both to `agent_stop`. We need to decide whether to merge them (losing the CC/Copilot distinction) or keep them separate. Recommendation: **merge into `agent_stop`** with CC getting `Stop` and Copilot CLI getting `agentStop` in the provider maps.

### 2c. Skill/Command Frontmatter: Tool Name Values

No field name changes needed. The YAML keys (`allowed-tools`, `disallowed-tools`, etc.) are already reasonable. The **values** inside these lists change from CC tool names to neutral names:

```yaml
# Before (CC-biased values)
allowed-tools:
  - Read
  - Grep
  - Glob

# After (neutral values)
allowed-tools:
  - file_read
  - search
  - find
```

### 2d. Agent Frontmatter: Tool Name Values

Same pattern — `tools` and `disallowedTools` list values change.

### 2e. MCP: `autoApprove` Field Name

| Current | Proposed | Rationale |
|---------|----------|-----------|
| `autoApprove` | `auto_approve` | snake_case, neutral (CC uses camelCase `autoApprove`, Cline/Roo use `alwaysAllow`) |

This is a lower-priority change since the field is provider-specific behavior (different providers have different semantics for auto-approval). Could defer to a separate bead.

---

## 3. Migration Strategy

The pattern is the same as the hooks migration described in the design doc. For each map/struct:

### 3a. `ToolNames` map (toolmap.go)

**Before:**
```go
var ToolNames = map[string]map[string]string{
    "Read": {
        "gemini-cli":  "read_file",
        "copilot-cli": "view",
        // ...
    },
```

**After:**
```go
var ToolNames = map[string]map[string]string{
    "file_read": {
        "claude-code":  "Read",      // CC is now a provider entry, not the key
        "gemini-cli":   "read_file",
        "copilot-cli":  "view",
        // ...
    },
```

Key change: **Claude Code moves from being the implicit key to being an explicit provider entry**, just like every other provider. This is the core de-biasing operation.

### 3b. `HookEvents` map (toolmap.go)

Same pattern:

**Before:**
```go
var HookEvents = map[string]map[string]string{
    "PreToolUse": {"gemini-cli": "BeforeTool", ...},
```

**After:**
```go
var HookEvents = map[string]map[string]string{
    "before_tool_execute": {"claude-code": "PreToolUse", "gemini-cli": "BeforeTool", ...},
```

### 3c. Translation functions (toolmap.go)

`TranslateTool`, `ReverseTranslateTool`, `TranslateHookEvent`, `ReverseTranslateHookEvent` all need updates:

- **TranslateTool**: Currently, if `targetSlug` is `claude-code`, it falls through and returns the key (which IS the CC name). After migration, CC gets an explicit map entry, so the generic lookup works for all providers equally.
- **ReverseTranslateTool**: Currently, `sourceSlug == "claude-code"` is a no-op (name is already canonical). After migration, CC names need reverse translation just like any other provider. The `"Task" -> "Agent"` backwards compat becomes `"Task" -> "agent"`.
- Same pattern for hook event functions.

### 3d. Canonicalize/Render in each converter

**Skills (skills.go):**
- `Canonicalize`: When source is CC, reverse-translate tool names in `allowed-tools`/`disallowed-tools` from CC names to neutral names. Currently CC is a no-op pass-through; after this change, CC tool names get translated like any other provider.
- `Render`: When target is CC, translate neutral tool names back to CC names. Currently the "claude-code" render path doesn't translate tools (they're already CC); after this change it must.

**Agents (agents.go):**
- Same pattern for `tools` and `disallowedTools` arrays.
- The `Canonicalize` already translates tools for non-CC providers (line 89-96). After migration, the `sourceProvider != "claude-code"` guard is removed — CC gets translated too.

**Commands (commands.go):**
- Same pattern for `allowed-tools`.

**Hooks (hooks.go):**
- `canonicalizeStandardHooks`: Currently skips translation when `sourceProvider == "claude-code"`. After migration, CC events and matchers get translated like any other provider.
- `renderStandardHooks`: Currently passes through canonical events untranslated for CC. After migration, CC events get translated from neutral to CC names.
- Same for `canonicalizeFlatHook`.

**MCP (mcp.go):**
- MCP doesn't reference tool names or event names in its canonical format. The `autoApprove` rename is a separate, lower-priority change. No immediate action needed for the core neutral naming migration.

### 3e. Hooks canonical JSON format

The stored `hook.json` files change:

**Before:**
```json
{
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [...]}]
  }
}
```

**After:**
```json
{
  "hooks": {
    "before_tool_execute": [{"matcher": "shell", "hooks": [...]}]
  }
}
```

Since we're pre-release with no users depending on the format, this is a clean break with no migration needed for stored files.

---

## 4. Impact on Existing Tests

### 4a. toolmap_test.go

Every test that references CC tool names as canonical keys needs updating. Tests for `TranslateTool`, `ReverseTranslateTool`, `TranslateHookEvent`, `ReverseTranslateHookEvent`, `TranslateMatcher`, `ReverseTranslateMatcher` all have CC names as the "canonical" form.

**Estimated scope:** All test cases in toolmap_test.go. Every canonical name string literal changes.

### 4b. hooks_test.go

Tests that build canonical hook JSON with CC event names (`"PreToolUse"`, `"SessionStart"`) and CC tool names in matchers (`"Bash"`, `"Edit|Write"`). Both input fixtures and expected output assertions change.

**Estimated scope:** Most test cases. The hook_security_test.go also references canonical event/tool names.

### 4c. skills_test.go

Tests that build canonical skill YAML with CC tool names in `allowed-tools`/`disallowed-tools`. Also tests for tool translation in render paths.

**Estimated scope:** Moderate — tests that exercise tool translation paths and round-trip tests.

### 4d. agents_test.go, codex_agents_test.go

Tests referencing CC tool names in `tools`/`disallowedTools` arrays.

**Estimated scope:** Moderate.

### 4e. commands_test.go

Tests referencing CC tool names in `allowed-tools`.

**Estimated scope:** Small.

### 4f. kitchen_sink_roundtrip_test.go, field_preservation_test.go

These comprehensive tests exercise full round-trips across providers. All canonical fixtures change.

**Estimated scope:** Large — these tests are the most fixture-heavy.

### 4g. mcp_test.go

MCP tests don't reference tool/event names in the canonical format. No changes needed for the core migration. Only affected if we also rename `autoApprove`.

### 4h. compat.go, compat_test.go

The compatibility engine references hook output field names (not tool/event names). Likely no changes needed unless field names in the output schema are CC-biased.

---

## 5. Backward Compatibility Approach

**No backward compatibility needed.** Per project rules:

> Syllago is pre-release and unpublished. No users depend on current APIs, CLI commands, or file formats. This means no backwards compatibility, migration paths, or deprecation periods needed.

The migration is a clean break:
1. Change the map keys and all references in one pass.
2. Update all test fixtures.
3. Update any stored hook.json files in the test fixtures or registry content.
4. No "accept both old and new" logic — the old names simply stop being canonical.

The only "compat" consideration is the `"Task" -> "Agent"` backwards compat in `ReverseTranslateTool`. This becomes `"Task" -> "agent"` (CC's legacy `Task` tool name reverse-translates to neutral `agent`).

---

## 6. Test Cases

### 6a. Tool Name Translation

| Test | Description |
|------|-------------|
| `TestTranslateTool_NeutralToCC` | `file_read` + `claude-code` -> `Read` |
| `TestTranslateTool_NeutralToGemini` | `shell` + `gemini-cli` -> `run_shell_command` |
| `TestTranslateTool_NeutralToUnknown` | `file_read` + `unknown-provider` -> `file_read` (passthrough) |
| `TestReverseTranslateTool_CCToNeutral` | `Read` + `claude-code` -> `file_read` |
| `TestReverseTranslateTool_GeminiToNeutral` | `read_file` + `gemini-cli` -> `file_read` |
| `TestReverseTranslateTool_AmbiguousMapping` | `edit_file` + `cursor` -> `file_edit` (prefer edit over write) |
| `TestReverseTranslateTool_LegacyTask` | `Task` + `claude-code` -> `agent` (backwards compat) |
| `TestTranslateTool_CCOnlyTools` | `notebook_edit` + `gemini-cli` -> `notebook_edit` (no mapping, passthrough) |

### 6b. Hook Event Translation

| Test | Description |
|------|-------------|
| `TestTranslateHookEvent_NeutralToCC` | `before_tool_execute` + `claude-code` -> `PreToolUse` |
| `TestTranslateHookEvent_NeutralToGemini` | `before_tool_execute` + `gemini-cli` -> `BeforeTool` |
| `TestTranslateHookEvent_Unsupported` | `notification` + `cursor` -> not supported |
| `TestReverseTranslateHookEvent_CCToNeutral` | `PreToolUse` + `claude-code` -> `before_tool_execute` |
| `TestReverseTranslateHookEvent_GeminiToNeutral` | `BeforeTool` + `gemini-cli` -> `before_tool_execute` |
| `TestAgentStopMerge` | Both CC's `Stop` and Copilot's `agentStop` reverse-translate to `agent_stop` |

### 6c. Matcher Translation

| Test | Description |
|------|-------------|
| `TestTranslateMatcher_SimpleNeutral` | `shell` + `gemini-cli` -> `run_shell_command` |
| `TestTranslateMatcher_Alternation` | `file_edit\|file_write` + `cursor` -> `edit_file` (both map to same) |
| `TestTranslateMatcher_Wildcard` | `shell.*` + `gemini-cli` -> `run_shell_command.*` |
| `TestTranslateMatcher_MCP` | `mcp__github__create_issue` passthrough unchanged |
| `TestReverseTranslateMatcher_CCToNeutral` | `Bash` + `claude-code` -> `shell` |

### 6d. Hook Canonicalize/Render Round-Trips

| Test | Description |
|------|-------------|
| `TestHookCanonicalizeCC_NeutralNames` | CC hook JSON with `PreToolUse`/`Bash` canonicalizes to `before_tool_execute`/`shell` |
| `TestHookRenderCC_FromNeutral` | Canonical `before_tool_execute`/`shell` renders to CC `PreToolUse`/`Bash` |
| `TestHookCanonicalizeGemini_NeutralNames` | Gemini `BeforeTool`/`run_shell_command` canonicalizes to `before_tool_execute`/`shell` |
| `TestHookRoundTrip_CC_CC` | CC -> canonical (neutral) -> CC produces equivalent output |
| `TestHookRoundTrip_CC_Gemini` | CC -> canonical (neutral) -> Gemini produces correct Gemini names |
| `TestHookRoundTrip_Gemini_CC` | Gemini -> canonical (neutral) -> CC produces correct CC names |
| `TestFlatHookCanonicalizeCC` | Flat-format CC hook canonicalizes event + matcher to neutral |

### 6e. Skill Tool Name Translation

| Test | Description |
|------|-------------|
| `TestSkillCanonicalizeCC_ToolNames` | CC skill with `allowed-tools: [Read, Grep]` canonicalizes to `[file_read, search]` |
| `TestSkillRenderCC_ToolNames` | Canonical skill with `[file_read, search]` renders to CC `[Read, Grep]` |
| `TestSkillRenderGemini_ToolNames` | Canonical `[file_read]` renders to Gemini `[read_file]` |
| `TestSkillRoundTrip_CC_CC` | CC -> canonical -> CC preserves tool names correctly |

### 6f. Agent Tool Name Translation

| Test | Description |
|------|-------------|
| `TestAgentCanonicalizeCC_ToolNames` | CC agent with `tools: [Read, Bash]` canonicalizes to `[file_read, shell]` |
| `TestAgentRenderCC_ToolNames` | Canonical `[file_read, shell]` renders to CC `[Read, Bash]` |
| `TestAgentRenderRooCode_ToolGroups` | Canonical `[file_read, shell]` maps to Roo groups `[read, command]` |
| `TestAgentCanonalizeKiro_ToolNames` | Kiro tool names reverse-translate to neutral, not CC |

### 6g. Command Tool Name Translation

| Test | Description |
|------|-------------|
| `TestCommandCanonicalizeCC_ToolNames` | CC command with `allowed-tools: [Read]` canonicalizes to `[file_read]` |
| `TestCommandRenderGemini_ToolNames` | Canonical `[file_read]` translates to `[read_file]` in Gemini prose notes |

---

## Implementation Order

1. **toolmap.go** — Rename all map keys, add `claude-code` entries, update translation functions. This is the foundation everything else depends on.
2. **toolmap_test.go** — Update all test fixtures for new canonical names.
3. **hooks.go** — Remove CC-is-canonical special cases in canonicalize/render. Treat CC as a regular provider.
4. **hooks_test.go, hook_security_test.go** — Update all hook test fixtures.
5. **skills.go** — Add tool name translation in CC canonicalize path; add reverse translation in CC render path.
6. **skills_test.go** — Update skill test fixtures.
7. **agents.go** — Remove `sourceProvider != "claude-code"` guard; add translation in CC render path. Update Roo Code group mapping to use neutral names.
8. **agents_test.go, codex_agents_test.go** — Update agent test fixtures.
9. **commands.go** — Add tool name translation in CC canonicalize/render paths.
10. **commands_test.go** — Update command test fixtures.
11. **kitchen_sink_roundtrip_test.go, field_preservation_test.go** — Update comprehensive test fixtures.
12. **compat.go** — Review for any CC-biased names in hook output field references (likely none).
13. **(Deferred)** MCP `autoApprove` rename — lower priority, separate bead if desired.

Each step should be independently testable: run `make test` after each file pair (source + test) to catch regressions incrementally.
