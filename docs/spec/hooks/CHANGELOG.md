# Changelog

All notable changes to the Hook Interchange Format Specification.

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
