# Provider Audit Findings

**Total findings: 78**
**Date: 2026-03-21**

| Severity | Count |
|----------|-------|
| Critical | 14 |
| High | 24 |
| Medium | 22 |

---

## Claude Code (17 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| CC-1 | Critical | toolmap | HookEvents uses "SubagentCompleted" instead of "SubagentStop" | Canonical hook event name is wrong. Will cause silent translation failures when mappings are added. | `cli/internal/converter/toolmap.go:86` | Claude Code event is `SubagentStop` (hooks.md line 49, content-types.md line 633) |
| CC-2 | Critical | converter | Rules lose `paths` frontmatter when rendered to Claude Code | Glob-scoped rules get prose embedding instead of native YAML `paths` frontmatter. Rules load unconditionally instead of conditionally. | `cli/internal/converter/rules.go:63-64` | Claude Code `.claude/rules/*.md` supports `paths` field in YAML frontmatter (content-types.md lines 114-134) |
| CC-3 | Critical | discovery | MCP discovery uses `.claude.json` instead of `.mcp.json` | Team-shared project-level MCP configs in `.mcp.json` are invisible to syllago. | `cli/internal/provider/claude.go:50` | Project-scoped MCP lives in `.mcp.json` (content-types.md lines 429, 1031) |
| CC-4 | Critical | converter | Canonical hook timeout units are inconsistent | Claude Code stores seconds, Copilot canonicalizer stores milliseconds. Round-tripping produces wrong values depending on source. | `cli/internal/converter/hooks.go:296-297, 434` | Claude Code timeouts are seconds (hooks.md lines 102, 110-118); canonical format is ambiguous |
| CC-5 | Critical | converter | SkillMeta.AllowedTools parses comma-separated string wrong | YAML unmarshals `"Read, Grep, Glob"` into single-element `[]string` instead of three elements. Breaks tool translation. | `cli/internal/converter/skills.go:22` | Claude Code `allowed-tools` is comma-separated string, not YAML list (skills-agents.md line 77) |
| CC-6 | High | converter | HookEntry struct missing http, prompt, and agent hook type fields | Only command-type hooks survive conversion. HTTP hooks lose `url`, `headers`, `allowedEnvVars`; prompt/agent hooks lose `prompt`, `model`. | `cli/internal/converter/hooks.go:19-25` | Claude Code supports 4 hook types: command, http, prompt, agent (hooks.md lines 150-223) |
| CC-7 | High | converter | AgentMeta missing `effort`, `hooks`, and `color` fields | Fields silently dropped during canonicalization. `effort` controls reasoning level; `hooks` are critical for workflow agents. | `cli/internal/converter/agents.go:19-36` | Claude Code agents support effort, hooks, color (skills-agents.md lines 449-452) |
| CC-8 | High | toolmap | HookEvents map missing 12 of 22 Claude Code events | Missing: PostToolUseFailure, PermissionRequest, PostCompact, InstructionsLoaded, ConfigChange, WorktreeCreate/Remove, Elicitation/Result, TeammateIdle, TaskCompleted, StopFailure. | `cli/internal/converter/toolmap.go:76-87` | Claude Code has 22 events (hooks.md lines 27-54) |
| CC-9 | High | toolmap | ToolNames map missing key tools including Agent (renamed from Task) | Map still uses "Task" but Claude Code renamed it to "Agent" in v2.1.63. Also missing WebFetch, NotebookEdit, Skill, AskUserQuestion. | `cli/internal/converter/toolmap.go:8-73` | Task renamed to Agent in v2.1.63 (tools.md line 275, 796) |
| CC-10 | High | converter | Rules converter doesn't distinguish CLAUDE.md vs .claude/rules/ | Always-apply rules should target CLAUDE.md; glob-scoped rules should target `.claude/rules/` with `paths` frontmatter. All rules render identically. | `cli/internal/converter/rules.go:339-360` | Two distinct mechanisms: CLAUDE.md (no frontmatter) and .claude/rules/*.md (YAML frontmatter with paths) (content-types.md lines 36-134) |
| CC-11 | High | converter | MCP config missing OAuth support | OAuth-configured MCP servers lose auth config. Struct has no `OAuth` field for Claude Code (only OpenCode). | `cli/internal/converter/mcp.go:16-44` | Claude Code MCP supports `oauth` config with clientId, callbackPort, authServerMetadataUrl (content-types.md lines 520-537) |
| CC-12 | Medium | converter | Hook matchers treat regex as literal tool names | Matchers like `Edit\|Write` or `mcp__github__.*` are passed to `TranslateTool()` as single strings, breaking regex patterns. | `cli/internal/converter/hooks.go` (matcher handling) | Matchers are regex patterns (hooks.md line 228); examples: `Edit\|Write`, `mcp__.*` |
| CC-13 | Medium | opportunity | Leverage native `paths` for cross-provider rule scoping | Claude Code's `.claude/rules/` `paths` frontmatter maps 1:1 to syllago's canonical `globs` field but isn't used. | `cli/internal/converter/rules.go` | `paths` frontmatter in .claude/rules/ (content-types.md line 118) |
| CC-14 | Medium | opportunity | Support `@path` import syntax in CLAUDE.md | Could generate master CLAUDE.md with `@` imports referencing individual rule files. | N/A | CLAUDE.md supports `@path/to/file` imports, max depth 5 (content-types.md line 57) |
| CC-15 | Medium | opportunity | Rename canonical "Task" tool to "Agent" | Canonical name should track current official name. Keep reverse translation from "Task" for compat. | `cli/internal/converter/toolmap.go` | Task renamed to Agent in v2.1.63 (tools.md line 275) |
| CC-16 | Medium | opportunity | Support Claude Code plugin system as content source | Loadouts are conceptually similar to Claude Code plugins (plugin.json manifests bundling skills, agents, hooks, MCP). | N/A | Plugin system documented at content-types.md lines 824-901 |
| CC-17 | Medium | opportunity | `disallowed-tools` on SkillMeta is forward-looking | Field exists in SkillMeta but not in Claude Code skill frontmatter. Only exists in agent frontmatter. Document as syllago extension or remove. | `cli/internal/converter/skills.go:23` | Only `allowed-tools` exists in skill frontmatter (skills-agents.md) |

---

## Gemini CLI (11 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| GC-1 | Critical | toolmap | Glob maps to `list_directory` instead of `glob` | Grants directory listing instead of file search. These are distinct Gemini tools. | `cli/internal/converter/toolmap.go:46` | Gemini has separate `glob` (FindFiles) and `list_directory` (ReadFolder) tools (tools.md lines 121-155) |
| GC-2 | Critical | toolmap | WebSearch maps to `google_search` instead of `google_web_search` | Hook matchers and agent tool allowlists targeting web search silently fail to match. | `cli/internal/converter/toolmap.go:64` | Official tool name is `google_web_search` (tools.md lines 213-226, content-types.md line 1061) |
| GC-3 | High | toolmap | MCP tool name format wrong: uses `server__tool` but Gemini uses `mcp_server_tool` | MCP hook matchers produce wrong format. Claude Code `mcp__github__get_issue` becomes `github__get_issue` instead of `mcp_github_get_issue`. | `cli/internal/converter/toolmap.go:147,157,181-187` | Gemini format is `mcp_<server_name>_<tool_name>` with single underscores and `mcp_` prefix (hooks.md lines 308-313) |
| GC-4 | Medium | toolmap | Grep maps to `grep_search` but primary name is `search_file_content` | May work via alias, but primary function name differs. Needs live verification. | `cli/internal/converter/toolmap.go:55` | Primary name is `search_file_content` (tools.md lines 158-162); `grep_search` appears in some config contexts |
| GC-5 | Medium | toolmap | Missing hook events: BeforeModel, AfterModel, BeforeToolSelection | Gemini-unique events have no canonical mapping. Import silently drops them with no warning. | `cli/internal/converter/toolmap.go:76-87` | Gemini has 3 model-level events with no Claude Code equivalent (hooks.md lines 885-891) |
| GC-6 | Medium | installer | Commands InstallDir returns `.gemini/` instead of `.gemini/commands/` | Commands may install to wrong location if installer doesn't append `commands/` subdirectory. | `cli/internal/provider/gemini.go:23-24` | Commands live in `~/.gemini/commands/*.toml` (content-types.md lines 380-386) |
| GC-7 | Medium | toolmap | No `list_directory` canonical mapping exists | After Glob fix, Gemini's `list_directory` has no canonical equivalent. Passes through untranslated. | `cli/internal/converter/toolmap.go` | `list_directory` is distinct from `glob` (tools.md lines 121-135) |
| GC-8 | Medium | toolmap | No reverse alias handling for `search_file_content` / `grep_search` | ReverseTranslateTool does exact matching. If Gemini uses both names in different contexts, only one will work. | `cli/internal/converter/toolmap.go:134-141` | Both names appear in Gemini docs (content-types.md line 1058) |
| GC-9 | Medium | discovery | Skills discovery missing `.agents/skills/` alternate path | Only discovers from `.gemini/skills/`, not `.agents/skills/` which Gemini also reads. | `cli/internal/provider/gemini.go:42` | Gemini discovers skills from both paths (content-types.md line 819, skills-agents.md line 523) |
| GC-10 | Medium | opportunity | No extension content type support | Gemini extensions (`gemini-extension.json` manifests) bundle commands, skills, agents, MCP, hooks, policies, themes. Cannot import/export as a unit. | N/A | Extension system documented in research |
| GC-11 | Medium | opportunity | No policy engine content type | Gemini's TOML-based 5-tier policy engine for tool permissions has no syllago representation. Enterprise policy rules dropped on import. | N/A | Policy engine documented in research |

---

## Cursor (14 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| CU-1 | Critical | converter | Cursor marked as hookless provider (WRONG) | Cursor has full hooks system (`hooks.json`) with 20+ event types. Hook conversion blocked entirely. | `cli/internal/converter/hooks.go:209` | Cursor hooks.json with JSON stdin/stdout protocol, blocking/non-blocking semantics (hooks research) |
| CU-2 | Critical | provider-def | Provider only supports Rules (missing 5 content types) | SupportsType returns true only for Rules. Cursor also supports Skills, Commands, Hooks, MCP, Agents. Test asserts wrong behavior. | `cli/internal/provider/cursor.go:44-51` | Skills (.cursor/skills/), Commands (.cursor/commands/), Hooks (hooks.json), MCP (.cursor/mcp.json), Agents (.cursor/agents/) all documented |
| CU-3 | Critical | toolmap | Cursor completely missing from ToolNames and HookEvents | No tool name or hook event translation exists for Cursor at all. 20 tools with clear cross-provider mappings undocumented. | `cli/internal/converter/toolmap.go` | read_file->Read, edit_file->Edit, run_terminal_cmd->Bash, grep_search->Grep, file_search->Glob, web_search->WebSearch (tools research) |
| CU-4 | Critical | installer | InstallDir only handles Rules | Missing install paths for Skills (.cursor/skills/), Commands (.cursor/commands/), Hooks (__json_merge__ hooks.json), MCP (__json_merge__ mcp.json), Agents (.cursor/agents/). | `cli/internal/provider/cursor.go:14-19` | Install paths documented in research |
| CU-5 | Critical | discovery | DiscoveryPaths only handles Rules | Missing discovery for Skills, Commands, Hooks, MCP, Agents. Cannot catalog existing Cursor content. | `cli/internal/provider/cursor.go:25-32` | Discovery paths documented for all content types in research |
| CU-6 | High | converter | No hooks canonicalization FROM Cursor | No `case "cursor":` in hooks Canonicalize switch. Cursor hooks use unique schema (version, hooks map, command, type, timeout, failClosed, loop_limit, matcher). | `cli/internal/converter/hooks.go` | Cursor hooks.json schema documented in hooks research |
| CU-7 | High | converter | No Cursor skills converter | Cursor has SKILL.md files with different frontmatter schema than Claude Code. No `case "cursor":` in skills Canonicalize or Render. | `cli/internal/converter/skills.go` | Cursor skills: name, description, license, compatibility, metadata, disable-model-invocation frontmatter |
| CU-8 | High | converter | No MCP config support for Cursor | Cursor uses `.cursor/mcp.json` with `mcpServers` key, STDIO/SSE/HTTP transports, variable interpolation. No converter exists. | N/A | .cursor/mcp.json with `${env:NAME}`, `${userHome}` interpolation |
| CU-9 | High | converter | No Cursor subagent/agent converter | Cursor supports `.cursor/agents/<name>.md` with frontmatter (name, description, model, readonly, is_background). No converter exists. | N/A | Cursor agents also read from .claude/agents/, .codex/agents/ |
| CU-10 | Medium | converter | Hook event set much richer than mapped | 20+ events including agent lifecycle, tool use, shell execution, MCP, file operations, subagent, tab, reasoning. Zero Cursor entries in HookEvents. | `cli/internal/converter/toolmap.go:76-87` | Full event list in hooks research |
| CU-11 | Medium | discovery | .cursorrules legacy file not discovered | Deprecated `.cursorrules` project root file not in DiscoveryPaths. Users migrating from older setups have this. | `cli/internal/provider/cursor.go` | .cursorrules documented as deprecated but still supported |
| CU-12 | Medium | discovery | AGENTS.md not associated with Cursor | Cursor reads AGENTS.md from project root and subdirectories. Not in Cursor's discovery paths. | `cli/internal/provider/cursor.go` | Cross-provider AGENTS.md support confirmed in research |
| CU-13 | Medium | toolmap | MCP tool name format unknown for Cursor | TranslateMCPToolName has no Cursor case. Research shows `"MCP:toolname"` colon-separated format in matchers. | `cli/internal/converter/toolmap.go` | Cursor hooks reference MCP tools as `"MCP:toolname"` |
| CU-14 | Medium | opportunity | Skills converter is straightforward | Cursor SKILL.md is close enough to canonical format. Key diffs: Cursor has license/compatibility/metadata, lacks allowed-tools/context/agent/model. | N/A | SKILL.md format documented in skills research |

---

## Windsurf (11 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| WS-1 | Critical | converter | Hooks marked as unsupported (WRONG) | Windsurf listed in `hooklessProviders`. Has full 12-event hook system (`hooks.json`) since v1.12.41. | `cli/internal/converter/hooks.go:210` | Windsurf hooks.json with command/show_output/working_directory fields |
| WS-2 | Critical | provider-def | Provider only supports Rules (missing 3+ content types) | SupportsType returns true only for Rules. Windsurf also supports Skills, Hooks, MCP. | `cli/internal/provider/windsurf.go:39-46` | Skills (.windsurf/skills/), Hooks (.windsurf/hooks.json), MCP (~/.codeium/windsurf/mcp_config.json) |
| WS-3 | Critical | discovery | Discovery only finds legacy `.windsurfrules` | Modern path `.windsurf/rules/*.md` (directory of frontmatter markdown files) not discovered. | `cli/internal/provider/windsurf.go:26-29`, `cli/internal/catalog/native_scan.go:271` | Modern rules in `.windsurf/rules/*.md` with trigger field; legacy `.windsurfrules` deprecated |
| WS-4 | Critical | installer | EmitPath targets legacy format only | Emits to `.windsurfrules` (single file). Modern rules go in `.windsurf/rules/<name>.md` with YAML frontmatter (trigger field). Installed rules cannot use model_decision, manual, or glob activation. | `cli/internal/provider/windsurf.go:36-38` | Modern format: individual .md files with trigger field (always_on, manual, model_decision, glob) |
| WS-5 | Critical | toolmap | Windsurf completely missing from ToolNames and HookEvents | No tool name or hook event translation. 23 tools documented with clear canonical equivalents. | `cli/internal/converter/toolmap.go` | view_line_range->Read, write_to_file->Write, edit_file->Edit, run_command->Bash, find_by_name->Glob, grep_search->Grep, search_web->WebSearch |
| WS-6 | High | installer | InstallDir only handles Rules at global scope | Missing paths for Skills, Hooks (__json_merge__), MCP (__json_merge__). | `cli/internal/provider/windsurf.go:14-19` | Skills at `~/.codeium/windsurf/skills/`, Hooks/MCP use JSON merge |
| WS-7 | High | converter | Hook event mapping is structural mismatch | Windsurf uses per-tool-category events (pre_read_code, pre_write_code, pre_run_command) vs syllago's event+matcher pattern. Converting requires splitting/merging, not just name translation. | `cli/internal/converter/toolmap.go` | Windsurf has distinct events per tool category, not generic PreToolUse+matcher |
| WS-8 | Medium | converter | No Windsurf skills support in converter | Windsurf follows Agent Skills standard (agentskills.io). SKILL.md format. No converter paths. | `cli/internal/converter/skills.go` | Windsurf skills at .windsurf/skills/<name>/ with SKILL.md |
| WS-9 | Medium | converter | No MCP config support | Windsurf uses `mcp_config.json` with `mcpServers` key, serverUrl (HTTP), url (SSE), `${env:VAR}` interpolation. | N/A | MCP at ~/.codeium/windsurf/mcp_config.json |
| WS-10 | Medium | opportunity | Workflows content type has no syllago equivalent | Windsurf Workflows (`.windsurf/workflows/*.md`, slash-command activated) have no analog content type. | N/A | Workflows documented in research |
| WS-11 | Medium | opportunity | AGENTS.md is cross-provider | Windsurf reads AGENTS.md natively. Content installed as AGENTS.md works without conversion. | N/A | Cross-provider AGENTS.md support confirmed |

---

## Codex (10 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| CX-1 | Critical | converter | Agent TOML uses wrong instructions field structure | Uses nested `[agent.instructions] content = "..."` but Codex uses top-level `developer_instructions` string. Agents may not load instructions. | `cli/internal/converter/codex_agents.go:132-138` | Codex format: `developer_instructions = """..."""` as top-level string |
| CX-2 | Critical | toolmap | Tool name translation uses wrong provider slug `"copilot-cli"` | Produces wrong names: Read->`view` (should be `read_file`), Glob->`glob` (should be `list_dir`), Grep->`rg` (should be `grep_files`), Task->`task` (should be `spawn_agent`). | `cli/internal/converter/codex_agents.go:59,93,179` | Codex tools from spec.rs: read_file, apply_patch, shell, list_dir, grep_files, web_search, spawn_agent |
| CX-3 | Critical | converter | Hooks marked as unsupported (WRONG) | Codex listed in `hooklessProviders`. Has experimental hooks system with 3 events: SessionStart, Stop, UserPromptSubmit. | `cli/internal/converter/hooks.go:211` | Codex hooks.json with 3 events documented in hooks research |
| CX-4 | High | converter | Agent TOML struct missing 5 supported fields | Renderer warns `mcpServers` and `skills` are "not supported by Codex" -- but research shows both ARE supported via `[mcp_servers]` and `[[skills.config]]`. Also missing: model_reasoning_effort, sandbox_mode, nickname_candidates. | `cli/internal/converter/codex_agents.go:132-138,158,161` | Codex supports mcp_servers, skills.config, model_reasoning_effort, sandbox_mode, nickname_candidates |
| CX-5 | High | toolmap | No `"codex"` entries in ToolNames map | Zero Codex entries. Research documents 32 tools. Core 8 need mappings at minimum. | `cli/internal/converter/toolmap.go:8-73` | 32 tools documented in tools research |
| CX-6 | High | provider-def | Skills not modeled as supported content type | SupportsType returns false for Skills. Codex has full skills system at `.agents/skills/` with SKILL.md + directory structure. | `cli/internal/provider/codex.go` (SupportsType) | Skills at .agents/skills/ with SKILL.md, more developed than most providers |
| CX-7 | High | provider-def | MCP not modeled as supported content type | Codex supports MCP in `config.toml` under `[mcp_servers.<id>]` tables (TOML format). | `cli/internal/provider/codex.go` (SupportsType) | MCP in config.toml as TOML tables, not JSON |
| CX-8 | Medium | toolmap | HookEvents map has no Codex entries | Event names happen to match canonical (identity mapping works accidentally), but no intentional mapping exists. | `cli/internal/converter/toolmap.go:76-87` | Codex events: SessionStart, Stop, UserPromptSubmit match canonical names |
| CX-9 | Medium | discovery | DiscoveryPaths missing skills and hooks | Only covers Rules, Commands, Agents. Missing `.agents/skills/` and `.codex/hooks.json`. | `cli/internal/provider/codex.go` (DiscoveryPaths) | Skills at `.agents/skills/` (not `.codex/skills/`); hooks at `.codex/hooks.json` |
| CX-10 | Medium | converter | Multi-agent config format incomplete | `codexConfig` only models `features` and basic `agents.<name>`. Missing global agent settings (max_threads, max_depth, job_max_runtime_seconds) and per-agent description, config_file, nickname_candidates. | `cli/internal/converter/codex_agents.go:19` | Codex config.toml [agents] section has more fields |

---

## Copilot CLI (13 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| CP-1 | Critical | toolmap | Write/Edit mapped to `apply_patch` instead of `create`/`edit` | `apply_patch` is internal wire name. User-facing tools are `edit` (string replacement) and `create` (new file). | `cli/internal/converter/toolmap.go:20,29` | events.jsonl shows `edit` and `create` as tool names |
| CP-2 | Critical | toolmap | Grep mapped to `rg` instead of `grep` | Tool appears in events.jsonl as `grep`, not `rg`. Underlying implementation uses ripgrep but tool name is `grep`. | `cli/internal/converter/toolmap.go:56` | events.jsonl documents tool name as `grep` |
| CP-3 | High | toolmap | Bash mapped to `shell` instead of `bash` | `shell` is the permission category. The tool itself is named `bash`. | `cli/internal/converter/toolmap.go:38` | Built-in tool is `bash`; `shell(COMMAND:*)` is permission syntax |
| CP-4 | High | discovery | MCP config filename wrong: `mcp.json` should be `mcp-config.json` | Discovers/installs MCP from wrong filename. | `cli/internal/provider/copilot.go:54` | MCP config is `mcp-config.json` at both `~/.copilot/` and `.copilot/` |
| CP-5 | High | discovery | Hook file location wrong: single file vs directory | Expects `.copilot/hooks.json` but Copilot uses `.github/hooks/*.json` (directory of JSON files). | `cli/internal/provider/copilot.go:56`, `cli/internal/catalog/native_scan.go:264` | Hooks in `.github/hooks/*.json`, each can contain multiple event types |
| CP-6 | High | converter | Copilot hook config missing `version` field | Rendered hooks missing required `"version": 1` top-level field. | `cli/internal/converter/hooks.go:147-148` | Copilot hook files require `"version": 1` |
| CP-7 | High | converter | Agent metadata struct missing model, target, mcp-servers fields | Only has name, description, tools. Missing at minimum: model, target, mcp-servers, disable-model-invocation, user-invocable. | `cli/internal/converter/agents.go:50-55` | Copilot agent frontmatter supports model, target, mcp-servers, disable-model-invocation |
| CP-8 | Medium | converter | Agent output filename should be `*.agent.md` not `agent.md` | Copilot agents use `<name>.agent.md` filenames. Syllago renders generic `agent.md`. | `cli/internal/converter/agents.go:260` | Copilot naming convention: `<name>.agent.md` |
| CP-9 | Medium | converter | Matcher support wrongly claimed as unsupported | Code says "Copilot doesn't support matchers" and drops them. Research shows Copilot supports Claude Code nested matcher/hooks structure. | `cli/internal/converter/hooks.go:409,413` | Copilot CLI supports matchers for cross-provider compat |
| CP-10 | Medium | converter | Hook entry missing `env`, `cwd`, and `type: "command"` fields | Struct only has bash, powershell, timeoutSec, comment. Missing env, cwd, and required type field. | `cli/internal/converter/hooks.go:139-144` | Copilot hook entries support env, cwd fields; type: "command" is required |
| CP-11 | Medium | toolmap | Missing hook events: SubagentStop, AgentStop, errorOccurred | HookEvents has 5 copilot-cli entries but research documents 8. SubagentStart/SubagentCompleted canonical entries have empty maps. | `cli/internal/converter/toolmap.go:76-87` | Copilot has subagentStop, agentStop, errorOccurred events |
| CP-12 | Medium | provider-def | Skills not supported in provider definition | Copilot CLI supports SKILL.md files in `.github/skills/` and `~/.copilot/skills/`. SupportsType returns false. | `cli/internal/provider/copilot.go:72-79` | Skills at .github/skills/ and ~/.copilot/skills/ |
| CP-13 | Medium | discovery | Instructions discovery paths incomplete | Only discovers `.github/copilot-instructions.md`. Missing: `.github/instructions/*.instructions.md`, cross-provider files (AGENTS.md, CLAUDE.md, GEMINI.md), personal `~/.copilot/copilot-instructions.md`. | `cli/internal/provider/copilot.go:44` | Multiple instruction sources documented in research |

---

## Zed (7 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| ZD-1 | Critical | toolmap | Task tool maps to `subagent` instead of `spawn_agent` | Tool permission configs require exact names. `subagent` won't match Zed's `spawn_agent` tool. | `cli/internal/converter/toolmap.go:71` | Canonical Zed tool name is `spawn_agent` (tools.md line 43) |
| ZD-2 | Critical | toolmap | MCP tool name uses slash format but Zed uses colon format | TranslateMCPToolName produces `server/tool` but Zed expects `mcp:server:tool`. | `cli/internal/converter/toolmap.go:147,159,189` | Zed format: `mcp:<server>:<tool_name>` (content-types.md line 100, skills-agents.md lines 185-198) |
| ZD-3 | High | discovery | Native scan doesn't discover `.rules` file | Zed's primary rule file `.rules` at project root is not scanned. Only `.zed` directory is scanned as generic settings. | `cli/internal/catalog/native_scan.go:282-283` | Zed primary rule file is `.rules` at project root; also reads 8 other provider filenames |
| ZD-4 | High | discovery | No MCP extraction from `.zed/settings.json` | Native scan doesn't discover MCP servers in `context_servers` key. `extractEmbeddedMCP` only checks `mcpServers` key. | `cli/internal/catalog/native_scan.go:228-231,282-283` | Zed MCP lives in settings.json under `context_servers` key |
| ZD-5 | Medium | toolmap | Missing WebFetch/fetch tool mapping | Zed's `fetch` tool (URL content retrieval) has no canonical mapping. Equivalent to Claude Code's WebFetch. | `cli/internal/converter/toolmap.go:63-66` | Zed has `fetch` tool for URL content retrieval |
| ZD-6 | Medium | discovery | `extractEmbeddedMCP` hardcoded to `mcpServers` key | Only checks Claude Code's key name. Zed uses `context_servers`, OpenCode uses `mcp`. Multi-provider scan broken. | `cli/internal/catalog/native_scan.go:228-231` | Provider-specific keys: context_servers (Zed), mcp (OpenCode), mcpServers (Claude Code) |
| ZD-7 | Medium | opportunity | Zed Agent Profiles as potential content type | Agent profiles (Write/Ask/Minimal/Custom) group tools into permission sets. Could be future syllago content type. | N/A | Agent profiles documented in research |

---

## Cline (9 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| CL-1 | Critical | toolmap | Edit mapped to `apply_diff` instead of `replace_in_file` | `apply_diff` is legacy name. Current Cline uses `replace_in_file`. | `cli/internal/converter/toolmap.go:33` | Cline renamed apply_diff to replace_in_file (tools.md lines 64-65) |
| CL-2 | Critical | converter | Cline marked as hookless provider (WRONG) | Cline has 8 hook events since v3.36: TaskStart, TaskResume, TaskCancel, TaskComplete, PreToolUse, PostToolUse, UserPromptSubmit, PreCompact. | `cli/internal/converter/hooks.go:207` | Cline hooks as file-based executables in .clinerules/hooks/ |
| CL-3 | Critical | provider-def | Provider missing Hooks support in SupportsType | Returns false for Hooks. Also missing from InstallDir. Cline hooks are file-based executables, not JSON merge. | `cli/internal/provider/cline.go:48-55` | Hooks at `.clinerules/hooks/` as named executables (PreToolUse, PostToolUse.ps1, etc.) |
| CL-4 | High | toolmap | No hook event mappings for Cline in HookEvents | No `"cline"` entries at all. Even after removing from hookless list, event translation falls through. | `cli/internal/converter/toolmap.go:76-87` | Mappings: PreToolUse->PreToolUse, PostToolUse->PostToolUse, SessionStart->TaskStart, Stop->TaskComplete |
| CL-5 | High | converter | Cline hooks use file-based executables, not JSON config | Fundamentally different install pattern: named executables with chmod +x, platform-specific naming. No converter exists. | N/A | Executables in .clinerules/hooks/, enable/disable via file permissions |
| CL-6 | Medium | discovery | Global rules path not in provider definition | DiscoveryPaths only covers workspace `.clinerules/`, not global `~/Documents/Cline/Rules/`. | `cli/internal/provider/cline.go` | Global rules at ~/Documents/Cline/Rules/ (all platforms) |
| CL-7 | Medium | discovery | MCP settings path not platform-aware | Actual target path varies by platform (macOS Library, Linux .config, Windows APPDATA). Installer needs platform detection. | `cli/internal/provider/cline.go` | Platform-specific paths under Code/User/globalStorage/saoudrizwan.claude-dev/settings/ |
| CL-8 | Medium | toolmap | Three Cline-specific tools unmapped: browser_action, use_mcp_tool, access_mcp_resource | browser_action has no canonical equivalent. MCP tools handled by MCP converter separately. | `cli/internal/converter/toolmap.go` | browser_action is Puppeteer automation; MCP tools are dispatch wrappers |
| CL-9 | Medium | opportunity | Cline cross-provider rule detection could inform import suggestions | Cline auto-detects .cursorrules, .windsurfrules, AGENTS.md. Could suggest Cline as target for users with multi-provider content. | N/A | Auto-detection documented in research |

---

## Roo Code (8 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| RC-1 | Critical | toolmap | Tool names use PascalCase class names instead of snake_case | Uses `ReadFileTool`, `WriteToFileTool`, `ExecuteCommandTool` (TypeScript class names) instead of `read_file`, `write_to_file`, `execute_command` (AI-facing tool-use names). | `cli/internal/converter/toolmap.go:16-62` | AI-facing interface uses snake_case: read_file, write_to_file, execute_command, search_files, list_files |
| RC-2 | Critical | converter | MCP config missing `alwaysAllow` field | Struct lacks alwaysAllow. Renderer warns autoApprove is "dropped (not supported by Roo Code)" -- but Roo Code DOES support it as `alwaysAllow`. Misleading warning + data loss. | `cli/internal/converter/mcp.go:51-60,501,517` | Roo Code supports `alwaysAllow` (array of tool names), identical to Cline's format |
| RC-3 | High | converter | Agent converter omits `mcp` tool group | renderRooCodeAgent has no mapping producing `mcp` group. Agents using MCP tools lose that capability. Stable-order emit list also omits "mcp". | `cli/internal/converter/agents.go:304-315,319` | `mcp` (use_mcp_tool, access_mcp_resource) is standard tool group in most built-in modes |
| RC-4 | High | provider-def | Skills not supported as content type | SupportsType returns false. Roo Code has full skills system: SKILL.md with frontmatter, progressive disclosure, mode-specific paths (.roo/skills-{mode}/), global paths. | `cli/internal/provider/roocode.go:61-68` | Skills at .roo/skills/{name}/SKILL.md, .roo/skills-{mode}/, ~/.roo/skills/, .agents/skills/ |
| RC-5 | Medium | discovery | `.roomodes` not in DiscoveryPaths for Agents | Returns nil for Agents. Custom modes in `.roomodes` not discoverable for catalog listing. | `cli/internal/provider/roocode.go:30-47` | Custom modes stored in `.roomodes` (project) and global settings YAML |
| RC-6 | Medium | discovery | Global rules paths missing | Only covers project-level. Missing `~/.roo/rules/` and `~/.roo/rules-{mode}/`. | `cli/internal/provider/roocode.go:33-41` | Global rule paths documented in research |
| RC-7 | Medium | toolmap | Tool map covers only 6 of 25+ tools | Missing WebSearch (no roo-code entry), Task/new_task, codebase_search, apply_diff, apply_patch, browser_action, switch_mode, skill, etc. | `cli/internal/converter/toolmap.go:8-73` | 25+ tools documented including new_task for subtask delegation |
| RC-8 | Medium | opportunity | Roo Code MCP format nearly identical to Cline | Same `mcpServers` key and `alwaysAllow` field. After fix, renderer could share code with Cline renderer. | N/A | Only additions are `type` (SSE) and `url` fields plus Streamable HTTP |

---

## OpenCode (8 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| OC-1 | Critical | toolmap | Read mapped to `view` instead of `read` | Wrong tool name in converted rules/skills. | `cli/internal/converter/toolmap.go:13` | OpenCode file reading tool is `read` (tools.md line 17) |
| OC-2 | Critical | toolmap | WebSearch mapped to `fetch` instead of `webfetch` | Uses non-existent tool name. OpenCode has separate `webfetch` and `websearch` (Exa AI) tools. | `cli/internal/converter/toolmap.go:65` | `webfetch` for URL fetching, `websearch` for Exa AI search (tools.md line 21) |
| OC-3 | Critical | toolmap | Task mapped to `agent` instead of `task` | Wrong tool name. `task` is the subagent delegation tool; agents are configured AI personas. | `cli/internal/converter/toolmap.go:70` | OpenCode subagent tool is `task` (tools.md line 28) |
| OC-4 | High | installer | Skills install directory uses singular "skill" instead of plural "skills" | Primary convention is plural. Discovery only checks singular path, misses content at canonical plural path. | `cli/internal/provider/opencode.go:25,50` | Primary path: `~/.config/opencode/skills/` and `.opencode/skills/` (plural); singular is backwards compat |
| OC-5 | Medium | toolmap | Hook events have no OpenCode mappings | Zero entries. OpenCode has 25+ plugin events with partial canonical mappings (tool.execute.before->PreToolUse, session.created->SessionStart). | `cli/internal/converter/toolmap.go:76-87` | Programmatic (TypeScript) vs declarative (JSON) format difference; but event-level mappings exist |
| OC-6 | Medium | converter | MCP "remote" type mapped to "sse" may lose distinction | OpenCode `remote` covers both SSE and streamable-http. Mapping all to "sse" loses transport specificity. | `cli/internal/converter/mcp.go:185-188` | OpenCode remote supports SSE, streamable-http, OAuth, Dynamic Client Registration |
| OC-7 | Medium | discovery | Discovery misses CLAUDE.md fallback and config-based instructions | Only checks AGENTS.md. OpenCode also reads CLAUDE.md (disabled via env var) and `instructions` array in opencode.json pointing to glob paths/URLs. | `cli/internal/provider/opencode.go:43-44` | CLAUDE.md fallback (content-types.md lines 68-72); instructions array in opencode.json (lines 75-83) |
| OC-8 | Medium | toolmap | Missing tools with no canonical equivalent | 8 unmapped tools: patch, list, lsp, skill, todowrite, todoread, question, websearch. Acceptable since canonical set is Claude Code-based. | `cli/internal/converter/toolmap.go` | Tools documented in research with no cross-provider equivalents |

---

## Kiro (7 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| KI-1 | Critical | toolmap | Glob and Grep both map to `read` instead of `glob`/`grep` | Grants overly broad `read` permission instead of correct granular tools. May not enable glob/grep functionality. | `cli/internal/converter/toolmap.go:48-49,58-59` | Kiro has separate `glob` and `grep` built-in tools (kiro.dev/docs/cli/reference/built-in-tools/) |
| KI-2 | Critical | converter | Agent format is JSON but Kiro uses markdown with YAML frontmatter | kiroAgentConfig uses JSON tags; renderKiroAgent outputs .json. Kiro standard is markdown frontmatter (.md). Exported agents in wrong format. | `cli/internal/converter/agents.go:386-393,449-492` | Kiro agents: YAML frontmatter with name, description, tools, model, allowedTools, etc. + markdown body |
| KI-3 | High | converter | FileFormat returns JSON for agents, should be markdown | Provider definition reports wrong format for agent files. | `cli/internal/provider/kiro.go:52-59` | Standard Kiro agent format is markdown with YAML frontmatter |
| KI-4 | High | toolmap | Missing WebSearch, WebFetch, and Task tool mappings | Kiro has web_search, web_fetch, delegate/use_subagent. Agent with WebSearch keeps literal "WebSearch" which Kiro won't recognize. | `cli/internal/converter/toolmap.go:63-73` | Kiro tools: web_search, web_fetch, use_subagent/delegate |
| KI-5 | High | converter | Agent config struct missing 10+ Kiro-specific fields | Missing: allowedTools (security-relevant), toolAliases, toolsSettings, mcpServers, resources, includeMcpJson, includePowers, keyboardShortcut, welcomeMessage, hooks. | `cli/internal/converter/agents.go:386-393` | 14+ config fields documented; allowedTools (auto-approve) is security-critical |
| KI-6 | Medium | toolmap | IDE-only hook events not mapped | 5 IDE-only events unmapped: FileCreate, FileSave, FileDelete, PreTaskExecution, PostTaskExecution, ManualTrigger. | `cli/internal/converter/toolmap.go` | 10 total events; 5 CLI events mapped, 5 IDE-only not mapped |
| KI-7 | Medium | opportunity | Kiro tool reference syntax richer than syllago models | Supports wildcards (`@server/read_*`), `@builtin` shorthand, `@powers` group. Syllago does 1:1 string mapping. | N/A | Wildcard patterns in tool references documented in research |

---

## Cross-Cutting (2 findings)

| # | Severity | Category | Title | Description | File & Line | Evidence |
|---|----------|----------|-------|-------------|-------------|----------|
| XX-1 | High | toolmap | Canonical "Task" tool should be renamed to "Agent" | Claude Code renamed Task to Agent in v2.1.63. Syllago's canonical name should track the current official name. Multiple providers affected. | `cli/internal/converter/toolmap.go` (Task entry) | Task renamed to Agent in Claude Code v2.1.63 (tools.md line 275). Keep "Task" as reverse-translation alias. |
| XX-2 | High | toolmap | No canonical "WebFetch" tool exists | Multiple providers have URL-fetching tools (Claude Code WebFetch, Zed fetch, OpenCode webfetch, Kiro web_fetch, Windsurf read_url_content) but no canonical entry in ToolNames. | `cli/internal/converter/toolmap.go` | WebFetch exists in Claude Code, Zed, OpenCode, Kiro, Windsurf, Copilot CLI with different names |
