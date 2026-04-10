# Hook Adapter Tier 3: New Provider Adapters — Design Document

**Goal:** Add hook adapters for Factory Droid, VS Code Copilot, Windsurf, and Pi, plus comprehensive provider research docs for all 6 new/updated providers (including Crush and OpenCode/SST).

**Decision Date:** 2026-03-30

---

## Problem Statement

Tier 2 migrated the existing 5 adapters (CC, Gemini, Copilot CLI, Cursor, Kiro) off the legacy bridge onto direct canonical encode/decode with shared helpers. The toolkit is proven. Now we need adapters for the remaining providers that have hook systems, plus research documentation for all new providers to feed the audit/update pipeline.

Two providers (Pi, OpenCode/SST) use a fundamentally different paradigm — programmatic TypeScript extensions instead of declarative JSON config — requiring a new code generation capability.

## Provider Landscape

### Research Findings (2026-03-30)

| Provider | Repo | Stars | Hooks? | Config Format | Hook Paradigm |
|----------|------|-------|--------|---------------|---------------|
| Factory Droid | Factory-AI/factory | 680 | Yes (9 events) | `.factory/settings.json` (JSON) | CC-identical JSON |
| VS Code Copilot | (Microsoft, closed) | N/A | Yes (18 events) | VS Code settings.json | CC-identical JSON |
| Windsurf | (Codeium, closed) | N/A | Yes (12 events) | `.windsurf/hooks.json` (JSON) | Split-event JSON |
| Pi | badlogic/pi-mono | 29K | Yes (25+ events) | `.pi/extensions/*.ts` (TypeScript) | Programmatic TS |
| Crush | charmbracelet/crush | 22K | **No** | `crush.json` (JSON) | N/A |
| OpenCode (SST) | anomalyco/opencode | 133K | Yes (30+ events) | `.opencode/plugins/*.ts` (TypeScript) | Programmatic TS |

### Critical Discovery: OpenCode Identity

There are **two separate projects** called "OpenCode":

1. **SST's OpenCode** (`anomalyco/opencode`) — TypeScript, 133K stars, 2.2M monthly npm installs, actively developed (v1.3.6+). Has rich plugin/hook system with 30+ events. **This is what syllago's existing provider docs and toolmap entries target.**

2. **Original OpenCode** (`opencode-ai/opencode`) — Go, archived September 2025, became **Crush** under Charm. No hooks, no plugins. Dead.

The split occurred July 2025 when Charm acquired the original OpenCode from creator Kujtim Hoxha. SST (Dax, Adam) kept the OpenCode brand and rewrote it in TypeScript. The two projects share no code, no config format, and no compatibility.

## Proposed Solution

### Workstream A: Provider Research Docs

Create/update documentation for 6 providers across 3 directories:

| Provider | `docs/providers/<slug>/` | `docs/provider-formats/` | `docs/provider-sources/` |
|----------|:---:|:---:|:---:|
| Factory Droid (`factory-droid`) | New (4 files) | New | New |
| Pi (`pi`) | New (4 files) | New | New |
| Crush (`crush`) | New (4 files) | New | New |
| VS Code Copilot (`vs-code-copilot`) | New (4 files) | New | New |
| OpenCode/SST (`opencode`) | Update existing | Update existing | Update existing |
| Windsurf (`windsurf`) | Verify current | Verify current | Verify current |

Each provider's `docs/providers/<slug>/` directory contains:
- `tools.md` — built-in tools with names, purposes, parameters
- `content-types.md` — all content type configs with schemas
- `hooks.md` — events, config format, execution model
- `skills-agents.md` — skill/agent definition formats

**Validation requirement:** After creating docs for each provider, a Sonnet subagent validates every claim against actual source code/docs. Fix issues, re-validate. Repeat until the validator returns zero issues. No assumptions that "one small issue" means validation is done.

### Workstream B: Hook Adapters (4 providers)

#### Adapter 1: Factory Droid (CC clone)

Factory Droid uses identical JSON schema, event names, tool names, matchers, and exit code semantics as Claude Code.

**Implementation:**
- Clone CC adapter structure, change `ProviderSlug()` to `"factory-droid"`
- Config paths: `.factory/settings.json` instead of `.claude/settings.json`
- Restrict capabilities to 9 confirmed events (vs CC's 25+)
- Emit warnings for CC-exclusive events during encode
- File: `adapter_factory_droid.go`

**Differences from CC:**
- Instructions file: `AGENTS.md` not `CLAUDE.md`
- Config directory: `.factory/` not `.claude/`
- Subagents called "droids" not "agents"
- Source-available, not open source

#### Adapter 2: VS Code Copilot (CC variant)

Same PascalCase event names and tool names as CC. Adds two capabilities:
- `platform_commands` — OS-specific command overrides (macOS/Linux/Windows)
- `custom_env` — environment variable injection into hook commands

**Implementation:**
- Fork from CC adapter, add platform command handling
- Encode: if canonical hook has `platform` map, generate platform-specific command entries
- Decode: merge platform-specific commands back into canonical `platform` field
- Custom env: encode canonical `env` map to provider-native env field; decode reverse
- File: `adapter_vs_code_copilot.go`

**Event mapping (18 events, all CC-aligned):**
- PreToolUse, PostToolUse, UserPromptSubmit, Stop, SessionStart, SubagentStart, SubagentStop, PreCompact (all confirmed in toolmap)

#### Adapter 3: Windsurf (split-event model)

Windsurf uses per-tool-category events instead of a unified `before_tool_execute` + matcher.

**Split-event encoding rules:**

| Canonical | Canonical Matcher | Windsurf Event |
|-----------|-------------------|----------------|
| `before_tool_execute` | `shell` | `pre_run_command` |
| `before_tool_execute` | `file_read` | `pre_read_code` |
| `before_tool_execute` | `file_write` / `file_edit` | `pre_write_code` |
| `before_tool_execute` | MCP | `pre_mcp_tool_use` |
| `before_tool_execute` | wildcard / none | All 4 pre-events |
| `after_tool_execute` | wildcard / none | All 4 post-events |
| `before_prompt` | — | `pre_user_prompt` |
| `agent_stop` | — | `post_cascade_response` |
| `session_start` | — | `session_start` (direct) |
| `session_end` | — | `session_end` (direct) |

**Decode (merge) rules:** Reverse — `pre_run_command` becomes `before_tool_execute` + `shell` matcher.

**Capability restrictions:**
- No timeouts (dropped with warning)
- No blocking field — implicit via exit code 2
- Only `command` handler type (no prompt/agent/http)
- `show_output` and `working_directory` as `provider_data.windsurf` fields
- Anonymous hooks (no names)
- No structured output, no input rewrite, no LLM-evaluated hooks

**Additional events:**
- `post_cascade_response_with_transcript` — transcript export (maps to `agent_stop` variant)
- `post_setup_worktree` — worktree creation (maps to CC's `worktree_create` if present)

**File:** `adapter_windsurf.go`

#### Adapter 4: Pi (programmatic TypeScript)

Pi extensions are TypeScript modules using event subscriptions. This requires a fundamentally new capability: **TypeScript code generation and parsing.**

**Pi extension structure:**
```typescript
export default (pi) => ({
  hooks: {
    "tool_call": (event) => {
      // handler code
    }
  }
})
```

**Event mapping:**

| Canonical Event | Pi Event | Notes |
|----------------|----------|-------|
| `before_tool_execute` | `tool_call` | Can block by throwing |
| `after_tool_execute` | `tool_result` | Can modify result |
| `session_start` | `session_start` | Direct |
| `session_end` | `session_shutdown` | Renamed |
| `before_prompt` | `input` | Can transform/intercept |
| `agent_stop` | `agent_end` | Direct |
| `before_compact` | `session_before_compact` | Cancellable |
| `subagent_start` | `before_agent_start` | Pre-loop |
| `subagent_stop` | `agent_end` | Same as agent_stop |

**Encode approach (canonical → TypeScript):**

1. Use Go `text/template` to generate idiomatic Pi extension code
2. Shell command handlers → `execSync()` wrappers via Node.js `child_process`
3. Matchers → `if (event.tool === "toolname")` conditionals
4. Blocking → `throw new Error(stderr)` on exit code 2
5. Timeout → `execSync({timeout: N})` option
6. Embed **structured marker comments** for lossless round-trip

Example generated output:
```typescript
// Generated by syllago — do not edit markers manually
// @syllago:spec=hooks/0.1

import { execSync } from "child_process"

export default (pi) => ({
  hooks: {
    // @syllago:name=lint-on-save
    // @syllago:event=before_tool_execute
    // @syllago:matcher=shell
    // @syllago:blocking=true
    // @syllago:timeout=10000
    "tool_call": (event) => {
      if (event.tool !== "bash") return

      const input = JSON.stringify({
        tool_name: event.tool,
        tool_input: event.args
      })

      try {
        execSync('/path/to/lint-hook.sh', {
          input: input,
          timeout: 10000,
          stdio: ["pipe", "pipe", "pipe"]
        })
      } catch (err) {
        if (err.status === 2) {
          throw new Error(err.stderr?.toString() || "Blocked by hook: lint-on-save")
        }
      }
    },
  }
})
```

**Why marker comments:**
- Lossless round-trip for syllago-generated code — decode reads markers, not code
- Transparent — human-readable, no magic strings
- Non-invasive — comments don't affect runtime
- Forward-compatible — new fields added as new marker lines

**Decode approach (TypeScript → canonical):**

Two paths, tried in order:

**Path 1: Marker-based decode (syllago-generated code)**
- Scan for `@syllago:` comment lines
- Parse key=value pairs into structured data
- Reconstruct canonical hooks directly from markers
- High fidelity — markers ARE the canonical data

**Path 2: Heuristic decode via pure-Go parsing (hand-written code)**

Two-step parsing pipeline (no CGO required):
1. **Strip TypeScript → JavaScript** using esbuild's public Go API: `api.Transform(tsCode, api.TransformOptions{Loader: api.LoaderTS})`. esbuild is pure Go and removes all type annotations cleanly.
2. **Parse JavaScript AST** using `T14Raptor/go-fAST` (pure Go, MIT, ES6+ support with visitor pattern). Walk the AST to extract hook definitions.

AST walking algorithm:
- Find the default export → arrow function → object literal
- Navigate to the `hooks` property
- For each property in the hooks object:
  - Key = Pi event name → map to canonical event
  - Value = arrow function body → analyze:
    - `IfStatement` with `event.tool === "..."` → extract matcher
    - `CallExpression` matching `execSync(...)` / `spawn(...)` → extract command
    - `ThrowStatement` → mark as blocking
    - Object literal with `timeout: N` → extract timeout
- Tag decoded hooks with `provider_data.pi.decode_confidence: "heuristic"`
- Emit `ConversionWarning` for each inferred field

**What heuristic decode cannot recover:**
- Hook `name` (Pi hooks are anonymous)
- `degradation` strategy (no Pi equivalent)
- Complex handler logic (multi-step, async, closures)
- Exact canonical timeout if not set in execSync options

**Graceful fallback:** If esbuild transform fails (invalid syntax) or AST structure is unrecognizable, emit warning and set handler to `{type: "command", command: "# MANUAL: see original Pi extension at <path>"}`.

**Reusability:** The esbuild+go-fAST parsing infrastructure and marker comment system will be reused for the OpenCode (SST) adapter, since both use the same programmatic TypeScript plugin pattern.

**Files:** `adapter_pi.go`, `adapter_pi_templates.go` (Go templates), `jsparse.go` (esbuild+go-fAST wrapper)

### Toolmap Updates

Add/update entries in `toolmap.go`:

**New `HookEvents` entries:**
- `factory-droid`: All 9 confirmed events (same names as CC)
- `pi`: tool_call, tool_result, session_start, session_shutdown, input, agent_end, session_before_compact, before_agent_start, turn_start, turn_end, model_select, user_bash, context, message_start, message_end

**New `ToolNames` entries:**
- `factory-droid`: Read, Write (Create), Edit, Execute (Bash), Glob, Grep, WebSearch, FetchUrl, Task (Agent)
- `pi`: read, write, edit, bash, grep, find, ls

**Verify existing entries:**
- `opencode`: Confirm all events/tools match SST's `anomalyco/opencode` (not archived project)
- `windsurf`: Verify 12 events match current Windsurf docs
- `vs-code-copilot`: Already mapped — verify completeness

## Architecture

### New Dependencies

| Dependency | Purpose | Impact |
|------------|---------|--------|
| `github.com/evanw/esbuild` | TS→JS stripping via `api.Transform()` | Pure Go, no CGO. Already widely used. |
| `github.com/nicholasgasior/go-fAST` (T14Raptor) | JavaScript AST parsing with visitor pattern | Pure Go, no CGO. MIT license. 164 stars. |

**Build impact:** None — both are pure Go. `CGO_ENABLED=0` builds continue to work. No cross-compilation changes needed.

### File Structure

```
cli/internal/converter/
  adapter_factory_droid.go          # Factory Droid adapter
  adapter_factory_droid_test.go
  adapter_vs_code_copilot.go        # VS Code Copilot adapter
  adapter_vs_code_copilot_test.go
  adapter_windsurf.go               # Windsurf adapter
  adapter_windsurf_test.go
  adapter_pi.go                     # Pi adapter
  adapter_pi_test.go
  adapter_pi_templates.go           # Go text/template for TS generation
  jsparse.go                        # esbuild+go-fAST wrapper for TS→JS→AST parsing
  jsparse_test.go
  testdata/
    factory-droid/                  # Test fixtures
    vs-code-copilot/
    windsurf/
    pi/
      generated-simple.ts          # Marker-based (round-trip test)
      generated-complex.ts         # Multiple hooks with matchers
      handwritten-simple.ts        # Basic hand-written extension
      handwritten-complex.ts       # Complex extension (heuristic test)
```

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| TypeScript parser | esbuild (TS→JS) + go-fAST (JS→AST) | Pure Go, no CGO, no build changes. esbuild strips types, go-fAST provides full AST with visitor. |
| Round-trip strategy for TS | Structured marker comments | Lossless for generated code, transparent, non-invasive |
| Heuristic decode | esbuild + go-fAST AST walking | Handles hand-written Pi extensions with best-effort accuracy |
| Factory Droid approach | CC clone adapter | Identical JSON schema and events — minimal divergence |
| Windsurf approach | Split-event expansion/merge | 1 canonical hook → N windsurf events based on matcher category |
| Provider docs validation | Sonnet subagent loops until clean | Zero tolerance for inaccuracy — docs must match source exactly |
| OpenCode identity | SST's anomalyco/opencode | 133K stars, TypeScript, active. Original Go project is Crush. |
| TS code generation safety | `json.Marshal()` for all interpolated values | Prevents command injection in generated TypeScript (see Security section) |
| Blocking semantics | Fail-closed on timeout/signal kill | Security hooks must block when uncertain, not silently allow |

## Data Flow

### Encode (Canonical → Provider)

```
CanonicalHooks
  |
  +- Factory Droid: hookhelpers -> same JSON as CC (different paths)
  +- VS Code Copilot: hookhelpers -> CC JSON + platform_commands + custom_env
  +- Windsurf: split-event logic -> per-category event JSON (hooks.json)
  +- Pi: Go templates -> TypeScript extension file (.ts)
```

### Decode (Provider → Canonical)

```
Provider Format
  |
  +- Factory Droid: JSON parse -> hookhelpers -> CanonicalHooks
  +- VS Code Copilot: JSON parse -> hookhelpers -> CanonicalHooks
  +- Windsurf: JSON parse -> merge split events -> CanonicalHooks
  +- Pi: marker scan OR esbuild+go-fAST heuristic parse -> CanonicalHooks
```

## Error Handling

### Degradation During Encode

| Canonical Feature | Factory Droid | VS Code Copilot | Windsurf | Pi |
|-------------------|:---:|:---:|:---:|:---:|
| blocking | Yes | Yes | Implicit (exit 2) | Yes (throw) |
| timeout | Yes (ms) | Yes (ms) | **Dropped** (warn) | Yes (execSync) |
| matcher | Yes | Yes | **Category-mapped** | Yes (conditional) |
| degradation strategy | Yes | Yes | **Dropped** (warn) | **Dropped** (warn) |
| structured_output | Likely | Likely | **No** (block) | **No** (block) |
| input_rewrite | Likely | Yes | **No** (block) | **No** (block) |
| llm_evaluated | Unknown | Unknown | **No** (exclude) | **No** (exclude) |
| http_handler | Unknown | Unknown | **No** (exclude) | **No** (exclude) |
| async_execution | Unknown | Unknown | **No** (warn) | **No** (warn) |
| platform_commands | Unknown | Yes | **No** (warn) | **No** (warn) |
| custom_env | Unknown | Yes | **No** (warn) | **Dropped** (warn) |
| name | Yes | Yes | **Dropped** (anonymous) | Via marker only |

### Warnings

All adapters emit `ConversionWarning` structs for:
- Unsupported events (with severity and suggestion)
- Dropped fields (with what was lost)
- Capability mismatches (with degradation strategy applied)
- Heuristic decode confidence levels (Pi only)

## Security Mitigations (from multi-persona review)

### Pi Encode: Command Injection Prevention

**Risk:** `Handler.Command` is a free-form string interpolated into generated TypeScript `execSync()` calls. Malicious canonical hooks from untrusted registries could inject arbitrary code.

**Mitigation:** All user-controlled values interpolated into generated TypeScript MUST go through `json.Marshal()` to produce properly escaped JS string literals. This handles quotes, backslashes, newlines, and unicode escapes correctly. Do NOT rely on `text/template` auto-escaping (which is HTML-oriented).

Fields requiring `json.Marshal()` escaping:
- `Handler.Command` — the shell command
- `Hook.Name` — interpolated into marker comments (strip newlines entirely)
- `Handler.CWD` — working directory in execSync options
- `Handler.Env` values — environment variable values
- `Matcher` values — tool names in conditional checks

Implementation: custom template function `{{jsString .Value}}` that calls `json.Marshal()` internally.

### Pi Encode: Blocking Fail-Closed

**Risk:** When a blocking hook's child process is killed by signal or times out, `err.status` is `null` in Node.js (not 2). The blocking check `if (err.status === 2)` would fail-open, silently allowing dangerous operations.

**Mitigation:** Generated template must handle three cases:
```
exit code 2 → throw (block)
err.killed === true (timeout) → throw (block) unless timeout_action says otherwise
err.status === null (signal) → throw (block) — fail-closed
```

### Pi Decode: Marker Trust

**Risk:** Marker comments can diverge from actual code. Markers say `/safe/script.sh` but code runs `/evil/script.sh`.

**Mitigation:** When markers are present, extract the command from the actual `execSync()` call in the code as well. If marker command differs from code command, emit a `ConversionWarning` with severity "error" and prefer the code (not the marker). This prevents marker poisoning attacks.

### Pi Decode: Marker Injection

**Risk:** Hand-written extensions containing `// @syllago:` comments would be decoded via Path 1 (marker) instead of Path 2 (heuristic), producing incorrect results.

**Mitigation:** Require the header line `// Generated by syllago` to enable marker-based decode. Without this header, always use heuristic decode even if `@syllago:` comments exist.

### Input Size Limits

For heuristic decode: reject files larger than 1MB before parsing. A Pi extension should never be megabytes. Wrap esbuild transform and go-fAST parse in timeout protection.

## Windsurf Round-Trip Semantics (from review feedback)

### Wildcard Expansion Problem

A canonical hook with `before_tool_execute` and no matcher (meaning "all tools") encodes to 4 Windsurf events: `pre_run_command`, `pre_read_code`, `pre_write_code`, `pre_mcp_tool_use`. On decode, these 4 events become 4 canonical hooks with specific matchers (`shell`, `file_read`, `file_write`, `mcp`). This is semantically different from the original "all tools" intent.

**Mitigation:** On encode, when expanding a wildcard canonical hook to N Windsurf events, add `provider_data.windsurf.expanded_from: "wildcard"` to each. On decode, if all hooks in a group share the same command and all have `expanded_from: "wildcard"`, merge them back into a single canonical hook with no matcher.

### Multiple Hooks Per Event

Windsurf supports arrays of hooks per event. If someone has two `pre_run_command` hooks, they must decode as two separate `before_tool_execute` + `shell` canonical hooks. The decode logic must preserve array ordering and not merge hooks that happen to target the same event.

### Blocking Semantics

Windsurf has no `blocking` field — blocking is implicit via exit code 2 on pre-hooks. On encode from canonical:
- `blocking: true` → no special field (exit code 2 behavior is built into the hook script)
- `blocking: false` → wrap command to mask exit code 2 (e.g., `command || true`)

On decode: all Windsurf pre-hooks are treated as `blocking: true` (exit code 2 is always possible). Post-hooks are `blocking: false`.

### Windsurf Event Routing

Windsurf's split-event model does NOT fit the 1:1 `HookEvents` mapping in `toolmap.go`. The `TranslateEventToProvider` helper maps one canonical event to one provider event, but Windsurf needs 1:N.

**Solution:** The Windsurf adapter implements its own event routing, bypassing `TranslateEventToProvider` for `before_tool_execute` and `after_tool_execute`. Direct event mapping for non-split events (`before_prompt`, `agent_stop`, `session_start`, `session_end`) still uses the shared helper.

## Verify() Extension (from review feedback)

The current `Verify()` in `adapter.go` only checks command, timeout, and blocking. New adapters introduce additional fidelity concerns. Extend `Verify()` to also check:

- **Matcher preservation** — verify matchers survive encode→decode
- **Event mapping** — verify canonical event names match after round-trip
- **Name preservation** — verify hook names survive (where provider supports them)
- **Handler type** — verify handler type is preserved or appropriately degraded

Each adapter can opt into field-level checks via a `VerifyFields() []string` method on the `HookAdapter` interface (optional — adapters that don't implement it get the existing basic checks).

## Success Criteria

1. All 4 adapters pass `Verify()` round-trip checks (including extended field-level checks)
2. All 6 providers have complete research docs with zero validation issues
3. Pure-Go dependencies (esbuild, go-fAST) build cleanly with `CGO_ENABLED=0`
4. `make test` passes with all new tests
5. `make build` produces working binary
6. Existing adapter tests still pass (no regressions)
7. Coverage >=80% on new packages
8. Pi marker-based round-trip is lossless
9. Pi heuristic decode correctly handles provided test fixtures
10. Windsurf split-event expansion/merge is reversible (wildcard grouping via provider_data)
11. Pi generated TypeScript passes command injection test cases (json.Marshal escaping verified)
12. Pi blocking hooks fail-closed on timeout and signal kill
13. Verify existing OpenCode toolmap entries match SST's anomalyco/opencode (not archived project)

## Open Questions

1. **Factory Droid extended events:** Only 9 events documented — does it support CC's full 25+ events? May need runtime testing or direct inquiry. Conservative approach: only map the 9 confirmed ones.

2. **Factory Droid tool name divergence:** Research shows "Create" and "Execute" vs CC's "Write" and "Bash". Need to verify exact tool names in hook matchers — if different, Factory Droid is NOT a pure CC clone for matchers.

3. **Pi extension loading:** Pi extensions can import npm packages. Our generated extensions use `child_process` (Node.js stdlib). Verify Pi's runtime (Jiti/Bun) supports this without additional config.

4. **OpenCode (SST) adapter timeline:** Same TS paradigm as Pi. Deferred to Tier 4 or immediately after Pi adapter is proven?

5. **go-fAST maturity:** 164 stars, relatively new. Need to verify ES6+ arrow function parsing works correctly for Pi extension patterns before committing. Fallback: goja (6.8K stars) with slightly less ES6+ coverage.

---

## Next Steps

Ready for implementation planning with `Plan` skill after multi-persona review.

---

## Research Sources

### Factory Droid
- [Factory.ai homepage](https://factory.ai)
- [Factory Hooks Guide](https://docs.factory.ai/cli/configuration/hooks-guide)
- [Factory Settings](https://docs.factory.ai/cli/configuration/settings)
- [Factory Custom Droids](https://docs.factory.ai/cli/configuration/custom-droids)
- [Factory Skills](https://docs.factory.ai/cli/configuration/skills)
- [Factory Plugins](https://docs.factory.ai/cli/configuration/plugins)
- [Factory Release Notes](https://docs.factory.ai/changelog/release-notes)
- [Factory Droid Shield](https://docs.factory.ai/cli/account/droid-shield)
- [Factory Enterprise](https://docs.factory.ai/enterprise)
- [GitHub: Factory-AI/factory](https://github.com/Factory-AI/factory) (680 stars, source-available)

### Pi
- [GitHub: badlogic/pi-mono](https://github.com/badlogic/pi-mono) (29K stars, MIT)
- [Pi Extensions docs](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md)
- [Pi Settings docs](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/settings.md)
- [Pi Skills docs](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md)
- [Pi Packages docs](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/packages.md)
- [Pi Prompt Templates docs](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/prompt-templates.md)
- [Pi Themes docs](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/themes.md)
- [Pi Extension types.ts](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/types.ts)
- [npm: @mariozechner/pi-coding-agent](https://www.npmjs.com/package/@mariozechner/pi-coding-agent) (v0.64.0)
- [Website: shittycodingagent.ai](https://shittycodingagent.ai/)

### Crush (Charm)
- [GitHub: charmbracelet/crush](https://github.com/charmbracelet/crush) (22K stars, FSL-1.1-MIT)
- [Crush schema.json](https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json)
- [Crush config.go](https://github.com/charmbracelet/crush/blob/main/internal/config/config.go)
- [Crush internal/event/](https://github.com/charmbracelet/crush/tree/main/internal/event) (PostHog telemetry only, not lifecycle hooks)
- [Crush plugin proposal (issue #2038)](https://github.com/charmbracelet/crush/issues/2038) (open, not implemented)
- [TheNewStack: Crush review](https://thenewstack.io/terminal-user-interfaces-review-of-crush-ex-opencode-al/)

### OpenCode (SST/Anomaly)
- [GitHub: anomalyco/opencode](https://github.com/anomalyco/opencode) (133K stars, TypeScript)
- [OpenCode Plugins docs](https://opencode.ai/docs/plugins/)
- [npm: opencode-ai](https://www.npmjs.com/package/opencode-ai) (2.2M monthly downloads)
- [OpenCode/Crush split context (issue #1097)](https://github.com/charmbracelet/crush/issues/1097)

### OpenCode (Original, archived)
- [GitHub: opencode-ai/opencode](https://github.com/opencode-ai/opencode) (11.7K stars, archived Sep 2025)
- Internal `pubsub/` package: CRUD events only, no user-facing hooks

### VS Code Copilot
- [GitHub Copilot docs](https://docs.github.com/copilot)
- Existing toolmap entries in `cli/internal/converter/toolmap.go`

### Windsurf
- [Windsurf Hooks docs](https://docs.windsurf.com/windsurf/cascade/hooks)
- Existing provider docs at `docs/providers/windsurf/hooks.md`

---

## Appendix: Multi-Persona Design Review (2026-03-30)

Three review perspectives were applied to the initial design draft:

### Skeptic (Technical Risk)
- **CGO_ENABLED=0 blocker** — Makefile builds all 6 platform targets with CGO_ENABLED=0. Tree-sitter requires CGO. → **Resolved:** Switched to pure-Go esbuild + go-fAST pipeline.
- **Windsurf split-event round-trip** — wildcard expansion is not reversible without metadata. → **Resolved:** Added `provider_data.windsurf.expanded_from` tracking.
- **Windsurf bypasses TranslateEventToProvider** — split-event model needs 1:N mapping. → **Resolved:** Documented adapter-specific event routing.
- **Verify() too weak** — only checks 3 fields, new adapters lose more data. → **Resolved:** Added Verify() extension plan.
- **Factory Droid tool names may diverge** — "Create"/"Execute" vs CC's "Write"/"Bash". → **Added:** Open Question #2.

### Security
- **Command injection in generated TS** — CRITICAL. Handler.Command interpolated into execSync. → **Resolved:** json.Marshal() for all interpolated values (see Security section).
- **Blocking bypass on timeout/signal** — err.status is null for killed processes. → **Resolved:** Fail-closed template (see Security section).
- **Marker/code divergence** — markers could lie about actual command. → **Resolved:** Cross-validate marker vs code command (see Security section).
- **Marker injection in hand-written files** — stray @syllago: comments trick Path 1. → **Resolved:** Require "Generated by syllago" header for marker decode.
- **Newline injection in marker fields** — names with newlines break comment context. → **Resolved:** Strip newlines from all marker values before emitting.

### Pragmatist
- **Provider docs are waste** — zero user-facing value. → **Overruled:** User explicitly requested docs; they feed the audit pipeline.
- **Pi is too complex for Tier 3** — TS codegen should be own tier. → **Overruled:** Pure-Go parsing removes the main complexity concern (CGO). Scope is manageable.
- **Priority ordering** — Windsurf/VS Code Copilot have highest user impact. → **Acknowledged:** Implementation order should be Windsurf > VS Code Copilot > Factory Droid > Pi.
- **Heuristic decode is gold-plating** — marker-only handles the primary use case. → **Kept:** Heuristic decode now uses pure-Go pipeline (no CGO cost). Value for importing hand-written Pi extensions justifies the moderate implementation effort.
