# Hook Adapter Tier 2: Migrate Off Legacy Bridge - Design Document

**Goal:** Replace the legacy bridge pipeline with direct CanonicalHook encode/decode in all 5 adapters, using shared translation helpers to keep code DRY.

**Decision Date:** 2026-03-30

---

## Problem Statement

All 5 hook adapters (CC, Gemini, Copilot, Cursor, Kiro) funnel through `ToLegacyHooksConfig()` / `FromLegacyHooksConfig()`, bottlenecking on the `HookEntry` struct. This silently drops 13 fields that `CanonicalHook` now carries after Tier 1:

| Lost Field | Type | Impact |
|---|---|---|
| `name` | string | Display/identification lost |
| `blocking` | bool | Safety-critical blocking semantics lost |
| `degradation` | map[string]string | Per-capability fallback strategy lost |
| `provider_data` | map[string]any | Provider-specific extensions lost |
| `capabilities` | []string | Informational capability list lost |
| `timeout_action` | string | Timeout behavior ("warn"/"block") lost |
| `status_message` | string | User-facing status text lost (also: JSON tag is camelCase, needs fix) |
| `platform` | map[string]string | Per-OS command overrides lost |
| `cwd` | string | Working directory lost |
| `env` | map[string]string | Environment variables lost |
| Structured matchers | json.RawMessage | MCP objects, regex patterns, arrays flattened to strings |

Round-trip fidelity is broken: encode then decode loses data with no warnings. The `Verify()` function only checks hook count + command presence, missing all field-level loss.

**Concrete bug example:** Copilot's `Capabilities()` declares `SupportsCWD: true` and `SupportsEnv: true`, but `Encode()` calls `ToLegacyHooksConfig()` which drops both fields before they ever reach `renderCopilotHooks`. The capabilities table and the actual encode path contradict each other.

## Proposed Solution

**Hybrid architecture:** Each adapter owns its Encode/Decode directly against CanonicalHook, but calls shared translation helpers for common operations. The legacy bridge disappears; shared logic stays as utility functions.

### Why Hybrid (Not Full Direct, Not Upgrade Bridge)

- **Full direct** duplicates event/matcher/timeout translation across 4 of 5 adapters that share ~80% of their logic. Grows linearly with new providers.
- **Upgrade bridge** fixes simple fields (blocking, cwd, env) but hits a hard wall with structured matchers: `HookEntry.Matcher` is `string`, can't carry `json.RawMessage`. Would need partial rebuild anyway.
- **Hybrid** gives each adapter its own pipeline while sharing a toolkit. New providers compose from helpers, not modify a framework.

## Architecture

### Layer 1: Shared Translation Helpers (`hookhelpers.go`, new file)

Stateless functions that translate between canonical and provider-specific representations. All helpers use the `Translate` verb convention (matching existing `toolmap.go` names) and return `[]ConversionWarning` for consistency with `EncodedResult`.

| Helper | Signature | Purpose |
|---|---|---|
| `TranslateEventToProvider` | `(event, slug string) (string, error)` | Canonical event -> provider event name |
| `TranslateEventFromProvider` | `(event, slug string) (string, []ConversionWarning)` | Provider event -> canonical (warnings, not errors — forward compat) |
| `TranslateMatcherToProvider` | `(matcher json.RawMessage, slug string) (json.RawMessage, []ConversionWarning)` | Canonical matcher -> provider format |
| `TranslateMatcherFromProvider` | `(matcher json.RawMessage, slug string) (json.RawMessage, []ConversionWarning)` | Provider matcher -> canonical format |
| `TranslateTimeoutToProvider` | `(seconds int, slug string) int` | Canonical seconds -> provider unit (ms or s) |
| `TranslateTimeoutFromProvider` | `(value int, slug string) int` | Provider unit -> canonical seconds |
| `TranslateMCPToProvider` | `(server, tool, slug string) string` | MCP server+tool -> provider MCP name |
| `TranslateMCPFromProvider` | `(mcpName, slug string) (server, tool string, ok bool)` | Provider MCP name -> server + tool |
| `TranslateHandlerType` | `(h HookHandler, slug string) (HookHandler, []ConversionWarning, bool)` | Handler type fitness check; bool = keep hook |
| `GenerateLLMWrapperScript` | `(hook CanonicalHook, slug, event string, idx int) (string, []byte)` | LLM prompt -> wrapper shell script |
| `CheckStructuredOutputLoss` | `(sourceSlug, targetSlug string) []ConversionWarning` | Warn about output field losses between providers |

**Design principles:**
- Encode path: `TranslateEventToProvider` returns `error` (unknown event = can't encode)
- Decode path: `TranslateEventFromProvider` returns `[]ConversionWarning` (unknown event = pass through for forward compat)
- All matcher/output helpers return `json.RawMessage`, never `any` — callers embed directly
- Adapters check their own capabilities before calling matcher helpers (don't hide drop logic inside helpers)

### Canonical Matcher Schema

`CanonicalHook.Matcher` (`json.RawMessage`) holds one of four defined shapes:

| Shape | Example | Detection |
|---|---|---|
| Bare string | `"shell"` | `json.Unmarshal` into `string` succeeds |
| Pattern object | `{"pattern": "file_(read\|write)"}` | Object with `"pattern"` key |
| MCP object | `{"mcp": {"server": "github", "tool": "create_issue"}}` | Object with `"mcp"` key |
| Array (OR) | `["shell", {"mcp": {"server": "fs"}}]` | JSON array; elements are any of the above three |

`TranslateMatcherToProvider` dispatches on shape. Unknown shapes pass through with a warning.

### Layer 2: Adapter Encode/Decode (per adapter file)

Each adapter's `Encode()` builds provider-native JSON directly from `CanonicalHook` structs:

```
for each CanonicalHook:
  1. TranslateEventToProvider(hook.Event, slug)
  2. Check adapter capabilities for matcher support
  3. If supported: TranslateMatcherToProvider(hook.Matcher, slug)
  4. TranslateTimeoutToProvider(hook.Handler.Timeout, slug)
  5. TranslateHandlerType(hook.Handler, slug) -> keep/skip
  6. Map fields the adapter supports; emit warnings for unsupported fields inline
  7. Render provider_data[slug] if present
  8. json.Marshal(providerStruct)
```

Each adapter's `Decode()` parses provider-native JSON and builds `CanonicalHook` structs:

```
for each provider hook entry:
  1. TranslateEventFromProvider(entry.Event, slug)
  2. TranslateMatcherFromProvider(entry.Matcher, slug)
  3. TranslateTimeoutFromProvider(entry.Timeout, slug)
  4. Map provider-specific handler fields to CanonicalHook.Handler
  5. Map provider-specific hook fields (blocking, name, etc.)
  6. Preserve unrecognized provider fields in provider_data[slug]
```

### Provider-Specific Schemas (per adapter)

Each adapter defines its own provider-native struct for marshal/unmarshal:

- **CC:** Flat array of hook objects, millisecond timeouts, all handler types
- **Gemini:** Similar to CC but subset of events, no LLM/HTTP hooks
- **Copilot:** Versioned dict (`{ version: "1.0", hooks: { event: [...] } }`), bash/powershell split, seconds timeout
- **Cursor:** Versioned config with failClosed, loop_limit; matchers supported
- **Kiro:** Agent wrapper with name/description/prompt; per-entry matchers; millisecond timeouts

### What Gets Removed

**Adapter bridge code (removed — adapters no longer call these):**

| Code | File | Status |
|---|---|---|
| `ToLegacyHooksConfig()` | adapter.go | Removed |
| `FromLegacyHooksConfig()` | adapter.go | Removed |
| `legacyResultToEncoded()` | adapter.go | Removed |
| `providerBySlug()` | adapter.go | Removed |

**Explicitly OUT OF SCOPE for removal:**

| Code | File | Why |
|---|---|---|
| `HookEntry` struct | hooks.go | Used by `LoadHookData`, installer, TUI via catalog |
| `HookEntry` struct | hooks.go | Used by `LoadHookData`, installer, TUI via catalog |
| `HookData` type | hooks.go | Used by `AnalyzeHookCompat` in compat.go |
| `hooksConfig` type | hooks.go | Used by `LoadHookData` and flat-format parsing |
| `ParseFlat`, `ParseNested` | hooks.go | Content loading path, not adapter code |
| `LoadHookData` | hooks.go | Called by catalog/installer |
| `AnalyzeHookCompat` | compat.go | Takes `HookData`, used by TUI |
| `renderStandardHooks()` | hooks.go | Used by `HooksConverter.Render()` for CLI convert command |
| `renderCopilotHooks()` | hooks.go | Used by `HooksConverter.Render()` for CLI convert command |
| `renderKiroHooks()` | hooks.go | Used by `HooksConverter.Render()` for CLI convert command |
| `canonicalizeStandardHooks()` | hooks.go | Used by `HooksConverter.Canonicalize()` for CLI convert command |
| `canonicalizeCopilotHooks()` | hooks.go | Used by `HooksConverter.Canonicalize()` for CLI convert command |

These are `HooksConverter` pipeline concerns (CLI convert command), not adapter concerns. The adapters bypass `HooksConverter` entirely. Migrating `HooksConverter` to use adapters internally is a separate task.

## Key Decisions

| Decision | Choice | Reasoning |
|---|---|---|
| Architecture | Hybrid (direct + shared helpers) | DRY without framework coupling |
| Helper naming | `Translate*` verb convention | Matches existing `toolmap.go` pattern |
| Warning types | `[]ConversionWarning` everywhere | Consistent with `EncodedResult`, carries severity/capability |
| Matcher return type | `json.RawMessage` (not `any`) | Callers embed directly, no type assertion needed |
| Encode errors vs decode warnings | Encode returns `error`; decode returns `[]ConversionWarning` | Encode = can't generate invalid output; decode = forward compat |
| `ApplyDegradation` | **Deferred** — no hook content uses `degradation` maps today | Speculative infrastructure; add when concrete use case exists |
| `CheckCapabilitySupport` | **Inline per adapter**, not shared helper | 3-4 lines per case, providers differ enough that sharing doesn't simplify |
| `status_message` | Add to spec, fix JSON tag to snake_case | Universal UI concept, pre-release safe to change |
| Cline in HookEvents | Remove | No confirmed hook API; re-add when verified |
| VS Code Copilot in toolmap | Add events/tools now (pre-work) | Spec defines it fully |
| New adapters | Tier 3 | Tier 2 builds toolkit; Tier 3 validates with Factory Droid smoke test |
| `Verify()` | Upgrade to check field-level fidelity | Must know which fields provider supports to distinguish intentional drops from bugs |
| `HookData`/`HookEntry` removal | Out of scope | Used by installer/TUI; separate migration |
| Copilot `errorOccurred` ambiguity | Add deterministic tiebreaker in decode | Go map iteration is non-deterministic; prefer `error_occurred` over `tool_use_failure` |
| Migration order | Incremental: helpers first, one adapter at a time, bridge removal last | Every step compiles and tests pass |

## Migration Order

1. **Write `hookhelpers.go`** — all shared translation helpers, with tests. No existing code removed or modified.
2. **Migrate CC adapter** — Reference implementation, richest feature set, validates helper API.
3. **Migrate Gemini adapter** — Structurally similar to CC, simpler (fewer hook types). Confirms pattern.
4. **Migrate Copilot adapter** — Different schema (versioned dict, bash/powershell). Validates `cwd`/`env` support that's currently broken.
5. **Migrate Cursor adapter** — Unique fields (failClosed, loop_limit). Similar to CC otherwise.
6. **Migrate Kiro adapter** — Agent wrapper structure, per-entry matchers.
7. **Remove legacy bridge** — Delete `ToLegacy*`, `FromLegacy*`, `render*Hooks`, `canonicalize*Hooks`.
8. **Upgrade `Verify()`** — Field-level fidelity checking.

Each step is independently compilable and testable. If we stop at step 3, the system works with 2 migrated adapters and 3 still on the bridge.

## Data Flow

### Encode Path (CanonicalHook -> Provider Native)

```
CanonicalHooks.Hooks[]
  |
  v (per hook)
adapter.Encode()
  |-- TranslateEventToProvider(hook.Event)
  |-- Check caps -> TranslateMatcherToProvider(hook.Matcher) or skip+warn
  |-- TranslateTimeoutToProvider(hook.Handler.Timeout)
  |-- TranslateHandlerType(hook.Handler) -> keep/skip
  |-- Inline capability warnings for unsupported fields
  |-- Render provider_data[slug] if present
  |-- json.Marshal(providerStruct)
  v
EncodedResult { Content []byte, Warnings []ConversionWarning }
```

### Decode Path (Provider Native -> CanonicalHook)

```
Provider JSON bytes
  |
  v
adapter.Decode()
  |-- json.Unmarshal into provider-native struct
  |-- (per entry)
  |   |-- TranslateEventFromProvider(entry.Event) -> event + warnings
  |   |-- TranslateMatcherFromProvider(entry.Matcher)
  |   |-- TranslateTimeoutFromProvider(entry.Timeout)
  |   |-- Map handler fields (command, platform, cwd, env, etc.)
  |   |-- Map hook fields (blocking, name, degradation, etc.)
  |   |-- Preserve unrecognized fields in provider_data[slug]
  |   v
  |   CanonicalHook
  v
CanonicalHooks { Spec: "hooks/0.1", Hooks: []CanonicalHook }
```

## Error Handling

| Scenario | Behavior |
|---|---|
| Unknown event during encode | Warning + skip hook (don't fail entire encode) |
| Unknown event during decode | Warning + pass through as-is (forward compat) |
| Structured matcher on provider without matchers | Adapter skips matcher call, emits warning |
| MCP matcher format unknown for provider | Fall back to bare string, warning emitted |
| Non-command handler type on provider without LLM/HTTP | `TranslateHandlerType` returns keep=false, adapter skips hook + warns |
| Array matcher on provider without array support | Expand to multiple hooks or regex alternation (provider-dependent) |

## Spec Alignment Pre-Work

Before adapter migration, these data-layer additions are needed (same pattern as Tier 1):

| Change | File |
|---|---|
| Add VS Code Copilot events to HookEvents | toolmap.go |
| Add VS Code Copilot tools to ToolNames | toolmap.go |
| Add VS Code Copilot to HookOutputCapabilities | compat.go |
| Add missing `after_task` event for Kiro | toolmap.go |
| Add missing `agent_stop` -> `session.idle` for OpenCode | toolmap.go |
| Remove Cline entries from HookEvents and ToolNames | toolmap.go |
| Fix `StatusMessage` JSON tag: `"statusMessage"` -> `"status_message"` | adapter.go |
| Add `status_message` to spec Section 3.5 | docs/spec/hooks-v1.md |
| Add deterministic tiebreaker for Copilot `errorOccurred` reverse mapping | toolmap.go |

**Note on `status_message` tag fix:** No stored canonical hook data exists in the repo (hooks are stored in provider-native format, not canonical). The tag change only affects round-trip through `CanonicalHook` structs, which is in-memory. Safe to change.

## Success Criteria

1. All 5 adapters encode/decode directly with CanonicalHook — no legacy bridge calls
2. Round-trip test: encode(decode(provider_json)) preserves all previously-dropped fields
3. Structured matchers (MCP, regex, array) survive round-trip through adapters that support matchers
4. Unsupported fields emit `ConversionWarning` (not silent drops)
5. Legacy bridge code removed (ToLegacy*, FromLegacy*, render*Hooks, canonicalize*Hooks)
6. `HookData`/`HookEntry`/flat-format parsing explicitly preserved (installer/TUI dependency)
7. All existing converter tests pass (updated for new behavior)
8. New round-trip tests with explicit field-value assertions (including timeout values, not just command presence)
9. `make build` succeeds
10. Cline removed from HookEvents/ToolNames
11. VS Code Copilot entries added to toolmap/compat
12. `provider_data` round-trip test: encode to provider, decode back, verify `provider_data[slug]` preserved

## Scope Boundaries

**In scope:**
- Migrate 5 existing adapters (incremental, one at a time)
- Shared helper toolkit (`hookhelpers.go`)
- Remove legacy bridge rendering/canonicalization functions
- Spec gap fixes (toolmap, compat, status_message)
- Cline cleanup
- Upgrade `Verify()` for field-level checks
- Comprehensive round-trip tests with value assertions

**Out of scope:**
- New adapters (VS Code Copilot, Factory Droid, Windsurf, OpenCode, Codex, deepagents-cli) — Tier 3
- `HookData`/`HookEntry` removal — used by installer/TUI
- `AnalyzeHookCompat` migration to CanonicalHook — depends on HookData
- `ApplyDegradation` helper — deferred until concrete use case
- Spec document updates beyond status_message
- CLI command changes
- TUI changes

## Beads

| ID | Description | Status |
|---|---|---|
| syllago-0q5if | Tier 2: migrate adapters off legacy bridge | Open (P2) |
| syllago-tto3m | Clean up Cline from HookEvents | Open (P2) |
| syllago-pg2cy | Tier 3: Windsurf split-event + new adapters | Open (P2) |
| syllago-ms77k | Hook adapter: VS Code Copilot | Open (P2), depends on Tier 2 |
| syllago-h5rsk | Hook adapter: Factory Droid | Open (P3), depends on Tier 2 |
| syllago-pr28y | Hook adapter: Codex (experimental) | Open (P4), depends on Tier 2 |
| syllago-0vmz4 | Hook adapter: deepagents-cli | Open (P4), depends on Tier 2 |

---

## Next Steps

Ready for implementation planning with `Plan` skill.
