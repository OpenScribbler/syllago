# Claude Code Provider Audit

Audit of syllago's codebase against Claude Code provider research.
Date: 2026-03-21

Research files:
- `docs/providers/claude-code/tools.md`
- `docs/providers/claude-code/content-types.md`
- `docs/providers/claude-code/hooks.md`
- `docs/providers/claude-code/skills-agents.md`

---

## Inaccuracies

### 1. HookEvents map uses "SubagentCompleted" instead of "SubagentStop"

**File:** `cli/internal/converter/toolmap.go:86`
**Code:** `"SubagentCompleted": {},`
**Research says:** The event is named `SubagentStop` (hooks.md line 49, content-types.md line 633). There is no event named "SubagentCompleted" in Claude Code.
**Impact:** If any provider maps to/from this event, the translation will silently fail. Currently the map is empty `{}` so no provider translates it, but the canonical name is wrong and will cause bugs when mappings are added.
**Fix:** Rename the key from `"SubagentCompleted"` to `"SubagentStop"`.

### 2. Rules rendered to Claude Code lose `paths` frontmatter (glob-scoped rules)

**File:** `cli/internal/converter/rules.go:63-64`
**Code:** Claude Code is routed to `renderSingleFileRule()` which strips frontmatter and embeds scope as prose.
**Research says:** Claude Code `.claude/rules/*.md` files support YAML frontmatter with a `paths` field for glob-scoped activation (content-types.md lines 114-134). This is a native, structured feature -- not something that needs prose embedding.
**Impact:** When converting glob-scoped rules TO Claude Code, the structured `paths` frontmatter is lost. Instead of `paths: ["**/*.ts"]` in YAML, the user gets a prose note "Scope: Apply only when working with files matching: **/*.ts". Claude Code would load this rule unconditionally instead of conditionally.
**Fix:** Add a dedicated `renderClaudeCodeRule()` that emits `paths` frontmatter for glob-scoped rules, similar to how Cline rules use `paths:`.

### 3. MCP discovery path uses `.claude.json` instead of `.mcp.json`

**File:** `cli/internal/provider/claude.go:50`
**Code:** `return []string{filepath.Join(projectRoot, ".claude.json")}`
**Research says:** Project-scoped MCP servers live in `.mcp.json` in the project root (content-types.md lines 429, 1031). The `~/.claude.json` file is the user-level file that stores MCP configs per-project, but `.mcp.json` is the project-level team-shared config.
**Impact:** syllago discovers MCP configs from `.claude.json` (user-local, not committable) instead of `.mcp.json` (project-level, committable). This means team-shared MCP configs are invisible to syllago's discovery.
**Fix:** Change to `filepath.Join(projectRoot, ".mcp.json")` or include both paths.

### 4. Canonical hook timeout units are inconsistent

**File:** `cli/internal/converter/hooks.go:296-297`
**Code:** `timeout := e.TimeoutSec * 1000 // Convert seconds to milliseconds`
**But:** `cli/internal/converter/hooks.go:434` does `TimeoutSec: h.Timeout / 1000`
**Research says:** Claude Code hook timeouts are in **seconds** (hooks.md lines 102, 110-118). The canonical `HookEntry.Timeout` field stores milliseconds (based on the Copilot conversion multiplying by 1000), but Claude Code's own format uses seconds.
**Impact:** When round-tripping Claude Code hooks, the timeout is stored as-is (no conversion in `canonicalizeStandardHooks`), meaning it's stored in seconds in canonical. But the Copilot canonicalizer converts to milliseconds. The canonical format is ambiguous -- some hooks have seconds, others have milliseconds depending on source.
**Fix:** Document and enforce a canonical unit (seconds, matching Claude Code) and only convert for providers that differ.

### 5. SkillMeta.AllowedTools is `[]string` but Claude Code uses comma-separated string

**File:** `cli/internal/converter/skills.go:22`
**Code:** `AllowedTools []string \`yaml:"allowed-tools,omitempty"\``
**Research says:** Claude Code's `allowed-tools` frontmatter field is a comma-separated string value, not a YAML list (skills-agents.md line 77, content-types.md line 214). Example: `allowed-tools: Read, Grep, Glob`.
**Impact:** When parsing Claude Code skills, YAML will try to unmarshal a comma-separated string into `[]string`, which will produce a single-element slice containing the full comma-separated string (e.g., `["Read, Grep, Glob"]` instead of `["Read", "Grep", "Glob"]`). This breaks tool translation during conversion.
**Fix:** Use `string` type for `AllowedTools` in the YAML struct, then split on commas during canonicalization. Or handle both `string` and `[]string` via a custom YAML unmarshaler.

---

## Gaps

### 6. HookEntry struct missing `http`, `prompt`, and `agent` hook type fields

**File:** `cli/internal/converter/hooks.go:19-25`
**Code:** `HookEntry` only has: `Type`, `Command`, `Timeout`, `StatusMessage`, `Async`.
**Research says:** Claude Code supports four hook handler types (hooks.md lines 150-223):
- `command`: needs `command`, `async`, `timeout`, `statusMessage`, `once`
- `http`: needs `url`, `headers`, `allowedEnvVars`, `timeout`
- `prompt`: needs `prompt`, `model`, `timeout`
- `agent`: needs `prompt`, `model`, `timeout`
**Impact:** HTTP hooks are silently dropped or corrupted during canonicalization. The `url`, `headers`, `allowedEnvVars`, `prompt`, `model`, and `once` fields are all lost. Only `command`-type hooks survive conversion intact.
**Fix:** Expand `HookEntry` to include all four hook type fields, or use a union/interface pattern.

### 7. AgentMeta missing `effort`, `hooks`, and `color` fields

**File:** `cli/internal/converter/agents.go:19-36`
**Code:** `AgentMeta` has: Name, Description, Tools, DisallowedTools, Model, MaxTurns, PermissionMode, Skills, MCPServers, Memory, Background, Isolation, Temperature, TimeoutMins, Kind.
**Research says:** Claude Code agents also support (skills-agents.md lines 449-452, content-types.md lines 346-347):
- `effort` (string: low/medium/high/max)
- `hooks` (object: lifecycle hooks)
- `color` (string: background color)
**Impact:** These fields are silently dropped when canonicalizing Claude Code agents. The `effort` field is particularly important for agents designed for specific reasoning levels. Hooks are critical for agents that enforce workflows.
**Fix:** Add `Effort string`, `Hooks any`, and `Color string` to `AgentMeta`.

### 8. HookEvents map is missing 12 of 22 Claude Code hook events

**File:** `cli/internal/converter/toolmap.go:76-87`
**Code:** Map contains 10 events: PreToolUse, PostToolUse, UserPromptSubmit, Stop, SessionStart, SessionEnd, PreCompact, Notification, SubagentStart, SubagentCompleted.
**Research says:** Claude Code has 22 events (hooks.md lines 27-54). Missing from the map:
- `PostToolUseFailure`
- `PermissionRequest`
- `PostCompact`
- `InstructionsLoaded`
- `ConfigChange`
- `WorktreeCreate`
- `WorktreeRemove`
- `Elicitation`
- `ElicitationResult`
- `TeammateIdle`
- `TaskCompleted`
- `StopFailure`
**Impact:** These events cannot be translated to/from other providers. Hooks using these events will pass through with untranslated names, which may or may not work depending on the target. For Claude-code-to-Claude-code, this is fine (passthrough). For cross-provider, translations are impossible.
**Fix:** Add entries for all 22 events. Even if other providers don't support them yet (empty maps), the canonical names should be registered so future provider support can be added.

### 9. ToolNames map missing several Claude Code built-in tools

**File:** `cli/internal/converter/toolmap.go:8-73`
**Code:** Map has: Read, Write, Edit, Bash, Glob, Grep, WebSearch, Task.
**Research says:** Claude Code has 28+ tools (tools.md lines 27-58). Missing from the map:
- `WebFetch` (has cross-provider equivalents in some tools)
- `NotebookEdit`
- `Agent` (the renamed Task; map still uses "Task")
- `Skill`
- `AskUserQuestion`
- Various unique tools (LSP, Cron*, EnterWorktree, etc.)
**Impact:** Tool name translation for `WebFetch` won't work. The `Task` entry should be renamed or aliased to `Agent` since that's the current canonical name (tools.md line 275: "Renamed from Task in v2.1.63").
**Fix:** Add `WebFetch` and `Agent` entries. Rename `Task` to `Agent` (or add `Agent` as primary and keep `Task` as alias).

### 10. Claude Code rules `renderSingleFileRule` doesn't handle CLAUDE.md vs .claude/rules/

**File:** `cli/internal/converter/rules.go:339-360`
**Research says:** Claude Code has two distinct rule mechanisms (content-types.md lines 36-134):
- `CLAUDE.md` files: no frontmatter, loaded at session start, additive across directory tree
- `.claude/rules/*.md` files: optional YAML frontmatter with `paths` field, support recursive discovery
**Impact:** The converter renders all rules as a single `rule.md` file with no distinction. It doesn't know whether the target should be a CLAUDE.md file or a `.claude/rules/` file. Always-apply rules would be better as CLAUDE.md additions; glob-scoped rules should be `.claude/rules/` files with `paths` frontmatter.

### 11. MCP config missing OAuth support for Claude Code

**File:** `cli/internal/converter/mcp.go:16-44`
**Research says:** Claude Code MCP configs support OAuth configuration (content-types.md lines 520-537):
```json
{"oauth": {"clientId": "...", "callbackPort": 8080, "authServerMetadataUrl": "..."}}
```
**Impact:** OAuth-configured MCP servers lose their auth config during conversion. The `mcpServerConfig` struct has no `OAuth` field for Claude Code (it does for OpenCode via `json.RawMessage`).
**Fix:** The existing `OAuth json.RawMessage` field is only populated from OpenCode. Ensure Claude Code canonicalization also preserves the `oauth` field.

---

## Opportunities

### 12. Leverage Claude Code's native `paths` for better cross-provider rule scoping

**Evidence:** Claude Code's `.claude/rules/` supports `paths` frontmatter (content-types.md line 118) which maps directly to syllago's canonical `globs` field. This is a 1:1 mapping that currently isn't used -- rules go through `renderSingleFileRule` which embeds scope as prose.
**Opportunity:** Rendering glob-scoped rules to Claude Code with native `paths` frontmatter would preserve structured activation semantics, making Claude Code the best target for Cursor/Windsurf glob-scoped rules.

### 13. Support Claude Code's `@path` import syntax in CLAUDE.md rules

**Evidence:** CLAUDE.md files support `@path/to/file` imports (content-types.md line 57). Max depth: 5 hops.
**Opportunity:** When installing multiple rules to Claude Code, syllago could generate a master CLAUDE.md that uses `@` imports to reference individual rule files in `.claude/rules/`. This keeps the CLAUDE.md clean while still loading all content.

### 14. Rename canonical "Task" tool to "Agent"

**Evidence:** The `Task` tool was renamed to `Agent` in Claude Code v2.1.63 (tools.md line 275, 796). The old name still works as an alias, but the canonical name should track the current official name.
**Opportunity:** Since syllago uses Claude Code as the canonical format, updating the ToolNames map key from "Task" to "Agent" would be more accurate and future-proof. Keep reverse translation from "Task" for backwards compatibility.

### 15. Support `disallowed-tools` in SkillMeta consistently

**Evidence:** The `SkillMeta` struct has `DisallowedTools []string \`yaml:"disallowed-tools"\`` (skills.go:23), but this field doesn't exist in Claude Code's skill frontmatter (skills-agents.md confirms only `allowed-tools`). It exists only in Claude Code's agent frontmatter as `disallowedTools`.
**Opportunity:** This field on SkillMeta is forward-looking (GitHub issue #6005 requests it). If kept, document that it's a syllago extension. If removed, it simplifies the skill struct.

### 16. Support Claude Code's plugin system as a native content source

**Evidence:** Claude Code has a full plugin system (content-types.md lines 824-901) that bundles skills, agents, hooks, MCP, LSP, and output styles. Plugins have a `plugin.json` manifest.
**Opportunity:** syllago loadouts are conceptually similar to Claude Code plugins. A future feature could import/export between syllago loadouts and Claude Code plugins, making syllago content distributable through Claude Code's plugin marketplace.

### 17. Hook matchers support regex, not just literal tool names

**Evidence:** Matchers are regex patterns (hooks.md line 228). Examples: `Edit|Write`, `mcp__github__.*`, `mcp__.*`.
**Opportunity:** The current hook converter treats matchers as literal tool names and translates them with `TranslateTool()`. This breaks regex matchers like `Edit|Write` (would try to translate the string "Edit|Write" as a single tool name). The converter should parse matchers as regex, identify tool names within them, and translate each individually.
