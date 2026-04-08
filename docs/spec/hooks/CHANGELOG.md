# Changelog

All notable changes to the Hook Interchange Format Specification.

## [core] 0.3.0 — 2026-04-08

### Breaking Changes

- Corrected gemini-cli `input_rewrite` field name from `hookSpecificOutput.updatedInput` to `hookSpecificOutput.tool_input` — hooks using this capability on gemini-cli were not functional

### Fixed

- [blocking-matrix] gemini-cli `after_tool_execute`: `observe` → `prevent` (supports `decision: "deny"` which hides tool result from agent context)
- [blocking-matrix] windsurf `before_prompt`: `observe` → `prevent` (`pre_user_prompt` supports exit code 2 blocking)
- [blocking-matrix] vs-code-copilot `session_end`: `observe` → `--` (VS Code does not support a SessionEnd event)
- [events §4] Removed kiro `agentSpawn` mapping for `session_start` — event does not appear in Kiro's current 10-event list (`--` in its place)
- [events §4] Fixed kiro double-mapping: both `session_end` and `agent_stop` were mapped to kiro `stop`/`Agent Stop`. Kiro has one "Agent Stop" event. Removed the `session_end` → `stop` mapping; `agent_stop` → `Agent Stop` is retained.
- [capabilities §1.2] gemini-cli `input_rewrite` mechanism corrected from `hookSpecificOutput.updatedInput` to `hookSpecificOutput.tool_input`
- [capabilities §1.7] gemini-cli `custom_env`: corrected from "Not supported" to Supported — `CommandHookConfig.env` field (`Record<string, string>`) present in source code
- [capabilities §1.6] claude-code `platform_commands`: corrected from "Not supported" to Supported — `shell: "bash" | "powershell"` interpreter selection field
- [provider-formats/opencode] Corrected "Hooks: Not supported" — OpenCode has a JavaScript plugin system with named hook events

### Added

- [events §2] `permission_denied` promoted note: added as Extended event in §2, mapped to claude-code `PermissionDenied`
- [events §3] 9 claude-code provider-exclusive events: `instructions_loaded`, `permission_denied`, `task_created`, `task_completed`, `teammate_idle`, `cwd_changed`, `after_compact`, `elicitation`, `elicitation_result`
- [events §3] kiro `manual_trigger` event (user-initiated, not tied to agent lifecycle)
- [events §3] 2 windsurf provider-exclusive events: `windsurf_transcript_response` (post_cascade_response_with_transcript), `windsurf_worktree_setup` (post_setup_worktree)
- [events §3] 7 opencode-exclusive events: `opencode_command_before/after`, `opencode_chat_params`, `opencode_chat_headers`, `opencode_shell_env`, `opencode_tool_definition`, `opencode_tui_events`
- [events §4] `session_end` / opencode: added `session.deleted` mapping (previously `--`)
- [events §4] `before_compact` / opencode: added `experimental.session.compacting` mapping (previously `--`)
- [blocking-matrix §2] New `permission_request` row — opencode: `prevent` (plugin can set `output.status = "deny"`); claude-code: `prevent`; all others: `--`
- Provider: **factory-droid** — events (§4), blocking matrix (§2), capabilities support, tools (§1)
- Provider: **codex** — events (§4), blocking matrix (§2), capabilities support (`llm_evaluated`, `async_execution`, `input_rewrite`), tools (§1)
- Provider: **cline** — events (§4), blocking matrix (§2), capabilities support, tools note; includes script-file architecture note and cline-specific footnote in §4

### Changed

- [capabilities §1.1] Documented opencode's exception-based blocking mechanism and added factory-droid, codex, cline structured_output entries
- [capabilities §1.3] Added codex `llm_evaluated` support (`prompt` and `agent` handler types)
- [capabilities §1.5] Added codex `async_execution` support with `scope` field
- [capabilities §1.1] Added kiro `cache_ttl_seconds` note (provider-unique caching capability, no canonical equivalent)
- [tools §1] Added factory-droid and codex columns to canonical tool name table
- [tools §1] Added matcher field support note: VS Code Copilot ignores matcher; Copilot CLI has no matcher system
- [tools §1] Added cline tool vocabulary note
- [tools §2] Added factory-droid, codex, cline MCP format entries
- [events §4] Added footnotes for kiro casing uncertainty and copilot-cli dual-mapping (`error_occurred` + `tool_use_failure` → `errorOccurred`)
- [events §4] Added cline session mapping footnote (TaskStart/TaskResume merge, architectural note)

### Deferred

~~- Cursor event and blocking data — separate re-fetch in progress due to bot protection on live docs; all cursor entries left unchanged pending verification~~ (resolved in 0.3.1 below)

## [core] 0.3.1 — 2026-04-08

### Fixed

- [events §4] Cursor `session_start` / `session_end`: `--` → `sessionStart` / `sessionEnd` (events exist as of Cursor v2.4, were missed in original spec due to bot-protection on Cursor docs at time of 0.3.0 authoring)
- [events §4] Cursor `after_tool_execute`: `afterFileEdit` → `afterShellExecution / afterMCPExecution / afterFileEdit` (split-event model mirrors before_tool_execute; all three after-events collectively cover the canonical concept)
- [blocking-matrix §2] Cursor `before_prompt`: `observe` → `prevent` (beforeSubmitPrompt supports `{"continue": false}`; blocking limited to continue-only — no userMessage/agentMessage/permission fields)
- [blocking-matrix §2] Cursor `session_start`: `--` → `prevent†` (sessionStart is classified as blocking by spec; footnote added documenting that `{"continue": false}` is silently ignored as of at least v2.4.21)
- [blocking-matrix §2] Cursor `session_end`: `--` → `observe` (sessionEnd event exists; blocking status unconfirmed, classified as observe)

### Added

- [events §3] 3 Cursor provider-exclusive events: `cursor_agent_thought` (afterAgentThought — observe only, requires thinking model), `cursor_tab_file_read` (beforeTabFileRead), `cursor_tab_file_edit` (afterTabFileEdit)
- [events §4] Mapping rows for new Cursor events: `cursor_agent_thought`, `cursor_tab_file_read`, `cursor_tab_file_edit`
- [events §4 footnotes] `cursor sessionStart blocking bug` — `{"continue": false}` silently ignored in Cursor v2.4.21; implementations should warn
- [events §4 footnotes] `cursor after_tool_execute split model` — adapter guidance for encoding/decoding three-way split
- [events §4 footnotes] `cursor beforeSubmitPrompt blocking limits` — continue-only response format; known bug with blocked messages persisting in LLM context history
- [blocking-matrix §2 footnotes] `†cursor session_start` — blocking broken in Cursor as of v2.4.21; cross-reference to events.md

## [core] 0.2.0 — 2026-04-08

### Breaking Changes

- Extracted registries and matrices to separate documents:
  - Event Registry → `events.md`
  - Tool Vocabulary → `tools.md`
  - Capability Registry + Degradation Strategies → `capabilities.md`
  - Blocking Behavior Matrix → `blocking-matrix.md`
  - Provider Strengths → `provider-strengths.md` (non-normative)
- Removed §5.4 (Advanced Output Fields) — capability-specific output fields are now defined in `capabilities.md`
- Renumbered core spec sections: §7 Conversion Pipeline, §8 Conformance Levels, §9 Versioning

### Added

- Directory README (`README.md`) as landing page for GitHub browsing
- Test vectors README (`test-vectors/README.md`) with format contract and index
- Reference implementations in Python and TypeScript (Core conformance)
- Cross-file dependency table in `CONTRIBUTING.md`
- Inline normative minimums in §8 Conformance Levels (core events, tool names, capability IDs)

### Fixed

- Consolidated timeout behavior documentation (§3.5 is now the single source, §4 references it)
- Strengthened `capabilities` field language from SHOULD to MUST NOT for conformance decisions

### Changed

- Single `CHANGELOG.md` with section prefixes replaces per-document changelogs
- Extracted documents carry "Last Modified" datestamps only

## [0.1.0] - 2026-03-27

Version reset from `1.0.0-draft` to `0.1.0` to reflect initial development status. Per Semantic Versioning, major version zero means anything may change. Promotion to v1.0 will happen when third-party adapter implementations exist and the spec has stabilized.

### Breaking (from draft)
- Specification version changed from `"hooks/1.0"` to `"hooks/0.1"` in manifests
- Platform key `"osx"` renamed to `"darwin"` (matches Go GOOS, Node process.platform, Rust target_os)

### Added — Fields
- `name` (optional string) on hook definitions for human-readable identification in warnings, logs, and policy references
- `timeout_action` (`"warn"` | `"block"`, default `"warn"`) on handler definitions — promoted from `provider_data` to a canonical field for safety-critical timeout behavior
- `capabilities` added to hook definition field table (was mentioned in prose but missing from table)

### Added — Events
- `tool_use_failure` (extended) — 3 providers: Claude Code, Cursor, Copilot CLI
- `file_changed` (extended) — 4 providers: Claude Code, Kiro, OpenCode, Cursor
- `before_model` promoted from provider-exclusive to extended — now 2 providers: Gemini CLI, Cursor
- `after_model` promoted from provider-exclusive to extended — now 2 providers: Gemini CLI, Cursor
- `before_tool_selection` promoted from provider-exclusive to extended — now 2 providers: Gemini CLI, Cursor
- Cursor native event names added to mapping table for `subagent_start`, `subagent_stop`, `before_model`, `after_model`, `before_tool_selection`

### Added — Spec Text
- Hook execution ordering: hooks MUST execute in array order, implementations MUST NOT reorder (Section 3.3)
- Script references vs. inline commands: documented `./` prefix convention, recommended scripts for blocking hooks (Section 3.5)
- Pattern matchers documented as not portable across providers; bare strings recommended for cross-provider use (Section 6.2)
- Unknown bare string matchers MUST be passed through as literal strings with a warning (Section 6.1)
- Regex flavor upgraded from SHOULD RE2 to MUST RE2 (Section 6.2)
- Truth table for exit code / decision / blocking interaction order (Section 5.3)
- Behavior when `decision` field is absent: defaults to `"allow"` (Section 5.2)
- Behavior when stdout is not valid JSON: treated as exit code 1 (Section 5.2)
- `timeout: 0` means no timeout / implementation default (Section 3.5)
- Empty `hooks` array MUST be rejected as validation error (Section 3.3)
- Blocking on observe-only events: not a validation error, SHOULD warn (Section 7.1)
- Blocking-on-observe warning upgraded from SHOULD to MUST (Section 8.2)
- Timing shift warning (Section 8.3): adapters MUST warn when before-event hooks map to after-events on target providers
- CI/CD headless environments explicitly covered by `ask` → `deny` fallback (Section 5.2)
- `structured_output` inference rule rewritten to remove circular dependency (Section 9.1)
- Patch version sentence clarified in Section 14.1

### Added — Security
- Cross-provider amplification risk documented (Section 1.3)
- Prompt injection via hook `context`/`system_message` output (Section 4.1)
- Transitive prompt injection via LLM-evaluated hooks (Section 4.2)
- Async execution observability risk (Section 4.3)
- Policy file integrity risk (Section 4.4)
- HTTP handler URLs: MUST HTTPS, MUST display URL upgraded from SHOULD (Section 2.4)

### Added — Test Vectors
- Windsurf provider: `simple-blocking.json`, `full-featured.json`, `multi-event.json` (4th provider, enterprise split-event architecture)
- Degradation test: `degradation-input-rewrite.json` canonical + Windsurf, Cursor, Gemini CLI outputs (safety-critical `input_rewrite` → `block` behavior)
- Negative/invalid vectors: 6 files in `test-vectors/invalid/` (missing-spec, missing-hooks, empty-hooks-array, missing-event, missing-handler, invalid-degradation-strategy)
- Round-trip vectors: `claude-code/roundtrip-source.json` + `claude-code/roundtrip-canonical.json`
- Cursor `multi-event.json` warning strengthened with TIMING SHIFT label for before→after semantic inversion
- Windsurf `degradation-input-rewrite.json` shows unconditional block when `input_rewrite` is unsupported

### Added — Ecosystem
- `CHANGELOG.md` (this file)
- CONTRIBUTING.md: `_comment` and `_warnings` test vector conventions documented
- CONTRIBUTING.md: schema validation commands added
- JSON Schema: `examples` arrays for `event` and handler `type` fields
- JSON Schema: `name` field on hook definitions
- JSON Schema: `timeout_action` field on handler definitions
- JSON Schema: `darwin` replaces `osx` in platform properties
- Glossary entries: "hook directory", "project root", "structurally equivalent", "hook name"

### Fixed
- Phantom `signatures` and `author` field references in security-considerations.md marked as "(Future)"
- Section numbering in security-considerations.md
- MCP format table duplication removed (Section 10.2 now cross-references Section 6.3)
- `file_saved` (Kiro) consolidated into broader `file_changed` extended event
