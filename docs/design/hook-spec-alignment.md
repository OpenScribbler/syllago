# Hook Spec Alignment — Design & Status

**Goal:** Align CLI hook conversion data structures, event/tool maps, and constants with the updated hooks spec v0.1.0.

**Branch:** `worktree-hook-spec-alignment` (commit `be71ed1`)
**Status:** Tier 1 complete, ready to rebase onto main and merge.

---

## Problem

The hooks spec (`docs/spec/hooks.md`) was updated to v0.1.0 with: version reset, new fields (name, timeout_action, capabilities), expanded event/tool mappings for 8+ providers, renamed events, snake_case canonical output field names, and new provider support (Windsurf, Opencode). The CLI code in `cli/internal/converter/` hadn't been updated to match.

## Three-Tier Approach

### Tier 1: Data Layer (DONE — `be71ed1`)

All changes are additive struct fields, map entries, and constants. No behavioral changes to the conversion pipeline.

**Changes (12 files, +191 -67):**

| Change | Files |
|--------|-------|
| SpecVersion `hooks/1.0` → `hooks/0.1` | `adapter.go` |
| Add `Name`, `Capabilities` to CanonicalHook | `adapter.go` |
| Add `TimeoutAction` to HookHandler | `adapter.go` |
| Rename event `after_tool_failure` → `tool_use_failure` | `toolmap.go`, `adapter_cc.go` |
| Add Windsurf events (4): session_start/end, before_prompt, agent_stop | `toolmap.go` |
| Add Opencode events (7): tool execute, session, error, permission | `toolmap.go` |
| Add new events: file_changed, file_created/deleted, before/after_task | `toolmap.go` |
| Add Cursor extended events: subagent, model, tool_selection | `toolmap.go` |
| Rename output field constants to spec snake_case | `compat.go` |
| Update HookOutputCapabilities: Gemini (decision+system_message), Cursor (decision) | `compat.go` |
| Remove Opencode from hooklessProviders | `hooks.go` |
| Update all 5 adapter Capabilities() event lists | `adapter_*.go` |
| Tests for all changes | `*_test.go` |

### Tier 2: Migrate Adapters Off Legacy Bridge (bead `syllago-0q5if`)

All 5 adapters delegate to the legacy pipeline via `ToLegacyHooksConfig()` / `FromLegacyHooksConfig()`, silently dropping: `blocking`, `degradation`, `provider_data`, `platform`/`cwd`/`env`, `timeout_action`, and structured matchers (MCP object, pattern object, array).

**Solution:** Each adapter's Encode/Decode works directly with `CanonicalHook` structs. ~2000-2500 lines across 8-10 files, estimated 2-3 sessions.

**Per-adapter field handling:**
- **blocking**: CC/Gemini native, others warn on observe-only events
- **degradation**: read during encode for block/warn/exclude per capability
- **provider_data**: round-trip matching slug, ignore others
- **platform**: Copilot supports (bash/powershell), others warn
- **cwd**: Windsurf/Copilot support, others warn
- **env**: VS Code Copilot only, others warn
- **timeout_action**: CC native, others warn on "block"

**Structured matchers** (`CanonicalHook.Matcher` is `json.RawMessage`):
- bare string → tool vocabulary lookup
- `{"pattern":"regex"}` → provider-specific regex
- `{"mcp":{"server":"...","tool":"..."}}` → provider MCP format (CC=`mcp__s__t`, Gemini=`mcp_s_t`, Cursor/Windsurf=`s__t`, Copilot=`s/t`, Zed=`mcp:s:t`)
- array → expand to multiple entries or regex alternation

### Tier 3: New Provider Adapters (bead `syllago-pg2cy`)

- **Windsurf split-event adapter**: per-tool-category events (pre_run_command, pre_read_code, etc.) instead of unified before_tool_execute + matcher
- **Opencode**: generates JS plugin files (not JSON) — fundamentally different format
- **Additional providers**: VS Code Copilot, Codex (experimental), Factory Droid (easy — identical to CC), deepagents-cli, Cline/Roo/Amp (research needed)

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Scope per session | Tier 1 only | Behavioral changes require migrating off legacy bridge |
| Event rename | `after_tool_failure` → `tool_use_failure` | Spec canonical name |
| Output field names | Spec snake_case constants | Canonical identifiers, not CC's JSON keys |
| Windsurf events | Map entries only, no split-event logic | Split-event encode/decode is Tier 2-3 |
| Copilot `errorOccurred` dual mapping | Accept ambiguity in reverse translation | Go map iteration order; both results valid |

## Merge Notes

The worktree branch has no conflicts with current main. `fa6ef31` on main added new files (`frontmatter_registry.go`, etc.) that don't overlap with our changes. Clean rebase expected.

## Beads

| ID | Description | Status |
|----|-------------|--------|
| syllago-0q5if | Tier 2: migrate adapters off legacy bridge | Open (P2) |
| syllago-pg2cy | Tier 3: Windsurf split-event + new adapters | Open (P2) |
