# Documentation Updates for Hook Canonical Spec

## Summary

The hook canonical spec (`docs/spec/hooks-v1.md`) introduces a provider-neutral interchange format with new canonical event names (`before_tool_execute`, `after_tool_execute`, `session_start`, etc.), a structured matcher system, capability model, and degradation strategies. This plan covers updating all documentation, example content, and user-facing text to reflect the spec.

## Current State

**The CC-biased canonical names problem:** Today, syllago's "canonical" format for hooks uses Claude Code's native event names (`PreToolUse`, `PostToolUse`, `SessionStart`, etc.) and tool names (`Bash`, `Read`, `Write`, `Edit`, `Grep`, `Glob`) as canonical. The spec defines truly provider-neutral names (`before_tool_execute`, `after_tool_execute`, `shell`, `file_read`, `file_write`, `file_edit`, `search`, `find`). All documentation, example hooks, converter code comments, and metadata need updating to reflect the new canonical names.

---

## 1. README.md Changes

**File:** `/home/hhewett/.local/src/syllago/.claude/worktrees/hook-canonical-spec/README.md`

### 1.1 Supported Providers Table (lines 113-128)

Currently shows Hooks support for only Claude Code, Gemini CLI, and Copilot CLI. The spec adds Cursor, Windsurf, VS Code Copilot, Copilot CLI, Kiro, and OpenCode to the hook ecosystem. Update the table to show hook support checkmarks for all providers the spec covers, with appropriate annotations for partial support.

### 1.2 Conversion Compatibility Table (lines 143-151)

Current text:
```
| Hooks | Claude Code, Gemini CLI, Copilot CLI | Other providers don't have hook systems |
```

Update to list all 8 providers from the spec. Add a note about degradation levels (full/degraded/broken/none) replacing the binary "has hooks / doesn't" distinction.

### 1.3 Content Types Table (line 137)

Current description: "Event-driven automation scripts that run before/after tool actions"

Consider adding a brief mention that hooks use a provider-neutral canonical format for cross-provider portability.

### 1.4 Roadmap Section (lines 263-266)

Current first item: "Cross-provider skill hook conversion (Claude Code skill-scoped hooks to Gemini global hooks)"

Update or replace this since the canonical spec formalizes cross-provider hook conversion. Consider replacing with items about implementing the full spec (capability inference, degradation strategies, structured output mapping).

---

## 2. CLAUDE.md Changes

**File:** `/home/hhewett/.local/src/syllago/.claude/worktrees/hook-canonical-spec/CLAUDE.md`

### 2.1 Key Conventions Section (line 33)

Current: "Hooks and MCP configs merge into provider settings files (JSON merge). All other content types use filesystem (files, dirs, symlinks)."

Add a note that hooks use the canonical interchange format defined in `docs/spec/hooks-v1.md` for cross-provider conversion. The canonical format uses provider-neutral event names (`before_tool_execute`, not `PreToolUse`) and tool names (`shell`, not `Bash`).

### 2.2 New Section: Hook Canonical Format

Add a brief section (3-5 lines) after Key Conventions explaining:
- The canonical hook format is defined in `docs/spec/hooks-v1.md`
- Canonical event names are `snake_case` and provider-neutral (e.g., `before_tool_execute`)
- Canonical tool names are lowercase descriptive (e.g., `shell`, `file_read`, `file_write`)
- The old CC-native names (`PreToolUse`, `Bash`, etc.) remain as provider-specific names in the toolmap, but are no longer the canonical representation

---

## 3. cli/CLAUDE.md Changes

**File:** `/home/hhewett/.local/src/syllago/.claude/worktrees/hook-canonical-spec/cli/CLAUDE.md`

### 3.1 Package Structure Table (lines 22-42)

#### converter package description (line 29)

Current: "Content format conversion between providers"

Update to: "Content format conversion between providers. Hook conversion uses the canonical interchange format (docs/spec/hooks-v1.md) with provider-neutral event names, tool vocabulary, capability model, and degradation strategies."

### 3.2 Content Types Section (lines 53-55)

Current: "Rules, Skills, Agents, Commands, Hooks, MCP configs, Loadouts"

No change needed (this is fine as-is).

---

## 4. CLI Help Text Changes

### 4.1 `inspect` command

**File:** `cli/cmd/syllago/inspect.go`

- Line 40: `"Show per-provider compatibility matrix (hooks only)"` -- Update to reference canonical event names in the output format. When displaying hook events, show the canonical name with the provider-native name in parentheses.
- Lines 131-135: The compatibility output currently shows CC-native event names. Update `AnalyzeHookCompat` display to show canonical event names.

### 4.2 `convert` command

**Files in:** `cli/cmd/syllago/`

- Any help text referencing hook events should use canonical names (`before_tool_execute`) rather than CC-native names (`PreToolUse`).

### 4.3 `add` command

- When discovering hooks from a provider, the output should show the canonical event name alongside the provider-native name.

---

## 5. TUI Changes

### 5.1 Detail View Hook Tab

**File:** `cli/internal/tui/detail.go`

- The hook detail view displays `hookData.Event` and `hookData.Matcher` values. After the canonical migration, these will use canonical names. The display should show:
  - Event: `before_tool_execute` (rendered with a human-friendly label like "Before Tool Execute")
  - Matcher: `shell` (with provider-native equivalent shown on the Install tab per-provider)
- The compatibility tab (`tabCompatibility`) should show canonical event/tool names with provider-native mappings per row.

### 5.2 Items List View

**File:** `cli/internal/tui/items.go` (or wherever hook items are rendered in lists)

- Hook items in the sidebar/list currently display CC-native event names. Update to use canonical names or human-friendly labels.

### 5.3 Portability Indicators

- The existing `CompatLevel` system (Full/Degraded/Broken/None) maps well to the spec's degradation model. Update the compatibility analysis to account for:
  - New capability categories from the spec (Section 9): `structured_output`, `input_rewrite`, `llm_evaluated`, `http_handler`, `async_execution`, `platform_commands`, `custom_env`, `configurable_cwd`
  - Degradation strategy display: show what happens when a capability is missing (block/warn/exclude)
  - The spec's blocking behavior matrix (Section 8): show when blocking intent cannot be honored

---

## 6. .syllago.yaml Metadata Format Updates

### 6.1 Hook-Specific Fields

**Current format** (e.g., `content/hooks/claude-code/running-linter/.syllago.yaml`):
```yaml
name: running-linter
description: Example hook that runs a linter after file edits
hidden: true
tags:
  - builtin
  - example
```

**Scanner field** (`cli/internal/catalog/scanner.go`, line 144):
```go
HookEvent string `yaml:"hookEvent,omitempty"`
```

The `hookEvent` field currently stores CC-native names like `PostToolUse`. After migration:
- `hookEvent` should store canonical event names (`after_tool_execute`)
- Consider adding new fields:
  - `hookMatcher` -- canonical matcher (e.g., `shell`, `file_write`)
  - `hookCapabilities` -- inferred capabilities list for display (optional, informational)
  - `hookBlocking` -- whether the hook is blocking

### 6.2 Files That Need hookEvent Value Updates

Every `.syllago.yaml` under `content/hooks/` that has a `hookEvent` field needs its value changed from CC-native to canonical. Check all 7 hook content items:

| Directory | Current hookEvent | New hookEvent |
|-----------|------------------|---------------|
| `example-lint-on-save` | `PostToolUse` | `after_tool_execute` |
| `logging-tool-usage` | `PostToolUse` | `after_tool_execute` |
| `pretooluse-ai-safety-check` | `PreToolUse` | `before_tool_execute` |
| `processing-notification` | `Notification` | `notification` |
| `running-linter` | `PostToolUse` | `after_tool_execute` |
| `validating-shell-command` | `PreToolUse` | `before_tool_execute` |

(Note: these values may not be in the `.syllago.yaml` today -- they may be derived from the `hook.json`. Verify each file.)

---

## 7. Example Hooks Conversion to Canonical Format

All 6 example hooks under `content/hooks/claude-code/` currently use CC-native format (flat-format `hook.json`). They need conversion to the canonical interchange format.

### 7.1 Conversion Table

| Hook | Current Format | Canonical Format |
|------|---------------|-----------------|
| `example-lint-on-save/hook.json` | `{"event":"PostToolUse","matcher":"Write\|Edit","hooks":[...]}` | `{"spec":"hooks/1.0","hooks":[{"event":"after_tool_execute","matcher":["file_write","file_edit"],...}]}` |
| `logging-tool-usage/hook.json` | `{"event":"PostToolUse","hooks":[{"async":true,...}]}` | `{"spec":"hooks/1.0","hooks":[{"event":"after_tool_execute","handler":{"type":"command","async":true,...}}]}` |
| `pretooluse-ai-safety-check/hook.json` | `{"event":"PreToolUse","hooks":[{"type":"prompt",...}]}` | `{"spec":"hooks/1.0","hooks":[{"event":"before_tool_execute","handler":{"type":"prompt",...}}]}` |
| `processing-notification/hook.json` | `{"event":"Notification","hooks":[...]}` | `{"spec":"hooks/1.0","hooks":[{"event":"notification","handler":{...}}]}` |
| `running-linter/hook.json` | `{"event":"PostToolUse","matcher":"Write\|Edit","hooks":[{"async":true,...}]}` | `{"spec":"hooks/1.0","hooks":[{"event":"after_tool_execute","matcher":["file_write","file_edit"],"handler":{"async":true,...}}]}` |
| `validating-shell-command/hook.json` | `{"event":"PreToolUse","matcher":"Bash","hooks":[...]}` | `{"spec":"hooks/1.0","hooks":[{"event":"before_tool_execute","matcher":"shell","handler":{...}}]}` |

### 7.2 Key Format Changes

1. **Top-level structure:** Add `"spec": "hooks/1.0"` field. Wrap hooks in a top-level `hooks` array.
2. **Event names:** `PreToolUse` -> `before_tool_execute`, `PostToolUse` -> `after_tool_execute`, `Notification` -> `notification`
3. **Matcher format:** `"Bash"` -> `"shell"`, `"Write|Edit"` -> `["file_write", "file_edit"]` (array OR format from spec Section 6.4)
4. **Handler structure:** The current flat `hooks[0]` entries become `handler` objects per spec Section 3.5
5. **Timeout units:** Current values are in milliseconds (CC-native). Convert to seconds (canonical unit). E.g., `15000` -> `15`, `5000` -> `5`, `10000` -> `10`, `3000` -> `3`, `2000` -> `2`
6. **Blocking:** Add explicit `"blocking": true/false` per spec Section 3.4 (currently implicit)

### 7.3 Structural Decision

The current syllago hook format (`{"event":..., "matcher":..., "hooks":[...]}`) is a flat single-event format. The spec uses `{"spec":"hooks/1.0", "hooks":[{...},{...}]}` with each hook having its own event, matcher, and handler. This is a fundamentally different structure. The converter code will need to:

1. Accept both old (syllago flat) and new (spec canonical) formats during a transition period
2. Store library hooks in spec canonical format going forward
3. Update `ParseFlat`, `ParseNested`, `DetectHookFormat` to handle the spec format

---

## 8. Grep Search: CC-Biased Canonical Names

Files containing CC-native names used as "canonical" that need updating. This is the core of the migration -- every reference to CC-native names as canonical must be evaluated.

### 8.1 Converter Package (highest priority)

| File | What Needs Changing |
|------|-------------------|
| `cli/internal/converter/toolmap.go` | Lines 5-8: Comment says "maps canonical tool names (Claude Code)". Lines 116-117: Comment says "maps canonical event names (Claude Code)". The maps themselves use CC names as keys (`"Read"`, `"Write"`, `"Bash"`, `"PreToolUse"`, `"PostToolUse"`). These need to be re-keyed to spec canonical names (`"file_read"`, `"file_write"`, `"shell"`, `"before_tool_execute"`, `"after_tool_execute"`). |
| `cli/internal/converter/hooks.go` | Lines 18-19: `HookEntry` struct uses CC field names. Lines 57-61: `hooksConfig` struct is the current "canonical" format. Lines 63-71: `HookData` flat format uses CC event names. Lines 207-228: `Canonicalize` treats CC format as canonical passthrough (`sourceProvider != "claude-code"` guards). Lines 319: Same CC passthrough in `canonicalizeStandardHooks`. |
| `cli/internal/converter/split.go` | Line 32-33: CC passthrough in `SplitSettingsHooks`. Line 67: Same pattern. |
| `cli/internal/converter/compat.go` | Hook compatibility analysis uses CC-native event/tool names. |
| `cli/internal/converter/hook_security.go` | Hook security analysis may reference CC-native names. |

### 8.2 Catalog Package

| File | What Needs Changing |
|------|-------------------|
| `cli/internal/catalog/scanner.go` | Line 144: `HookEvent` field stores CC-native names. |
| `cli/internal/catalog/native_scan.go` | Line 17: `HookEvent` field. Line 214: Stores raw event key from provider format. |
| `cli/internal/catalog/risk.go` | Hook risk analysis may reference CC-native event names. |

### 8.3 Installer Package

| File | What Needs Changing |
|------|-------------------|
| `cli/internal/installer/hooks_*.go` | Hook installation uses CC-native event names when merging into settings files. The installer should convert from canonical to provider-native during install. |

### 8.4 Loadout Package

| File | What Needs Changing |
|------|-------------------|
| `cli/internal/loadout/apply.go` | Lines 136-139, 347-369: `injectSessionEndHook` uses CC-native `"SessionEnd"` event name. This is correct for CC-specific functionality but should be documented as provider-specific, not canonical. |

### 8.5 Registry Package

| File | What Needs Changing |
|------|-------------------|
| `cli/internal/registry/registry_test.go` | Lines 261, 298-299: Test fixtures use `"PostToolUse"` as hookEvent values. |
| `cli/internal/registry/extract_hooks_test.go` | Test data uses CC-native event names. |
| `cli/cmd/syllago/registry_create_native.go` | Lines 159-162, 201-202, 261-262, 398: Propagates `HookEvent` from native scan. |

### 8.6 Test Files (42 files total)

All test files listed in the grep results that use CC-native event/tool names in test fixtures, assertions, and golden files need updating. These should be updated alongside their corresponding source files.

### 8.7 Content Files (6 hook.json files)

All files under `content/hooks/claude-code/` (covered in Section 7 above).

---

## Implementation Order

1. **Spec-canonical format parser** -- Add support for the `{"spec":"hooks/1.0","hooks":[...]}` format in the converter
2. **Canonical name maps** -- Re-key `ToolNames` and `HookEvents` in `toolmap.go` to use spec canonical names, adding CC as a provider entry
3. **Converter pipeline** -- Update `Canonicalize`/`Render` to use spec canonical names internally
4. **Example hooks** -- Convert all 6 `hook.json` files to spec canonical format
5. **Metadata** -- Update `.syllago.yaml` hookEvent values and scanner code
6. **Tests** -- Update all test fixtures and assertions
7. **Documentation** -- README.md, CLAUDE.md, cli/CLAUDE.md, ARCHITECTURE.md
8. **TUI** -- Update detail view, compatibility tab, items display
9. **CLI help text** -- Update inspect, convert, add command output

## Dependencies

- The spec itself (`docs/spec/hooks-v1.md`) must be finalized before implementation
- The converter changes (steps 1-3) must land before the example hooks and tests can be updated
- Documentation updates can happen in parallel with code changes

## Concurrent Session Changes (must be merged first)

A concurrent session (commits 845cc2f through e6d093d) made significant converter changes that affect this plan:

**Modified files that overlap with our grep results:**
- `cli/internal/converter/mcp.go` — Cline SSE, transport type mapping, DisabledTools
- `cli/internal/converter/rules.go` — Cursor globs, Kiro canonicalize/render
- `cli/internal/converter/skills.go` — SkillMeta fields, Kiro/OpenCode overhaul
- `cli/internal/converter/commands.go` — Effort field, OpenCode render
- `cli/internal/converter/agents.go` — permissionMode, Kiro warnings, OpenCode canonicalize/render
- `cli/internal/parse/cursor.go` — MDC parser
- Corresponding *_test.go files

**Impact:** The grep results for CC-biased names (section 8) may have changed — new files added, line numbers shifted, new CC-biased tool name values in `DisabledTools`. Re-run the grep during implementation to get fresh results.

**New provider-formats docs** added by the other session (`docs/provider-formats/{kiro,opencode,cline}.md`) should be reviewed for any hook-related content that needs canonical naming updates.
