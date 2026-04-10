# Hook Adapter Tier 3: New Provider Adapters — Implementation Plan

**Design doc:** `docs/plans/2026-03-30-hook-adapter-tier3-new-providers-design.md`
**Date:** 2026-03-30

---

## Phase 0: Pre-Work

### Task 0.1 — Add factory-droid and pi entries to toolmap.go

**File:** `cli/internal/converter/toolmap.go`

Add `"factory-droid"` to `ToolNames` entries. Factory Droid uses PascalCase like CC but diverges on two tools (`Create` not `Write`, `Execute` not `Bash`):

- `"file_read"`:  `"factory-droid": "Read"`
- `"file_write"`: `"factory-droid": "Create"`
- `"file_edit"`:  `"factory-droid": "Edit"`
- `"shell"`:      `"factory-droid": "Execute"`
- `"find"`:       `"factory-droid": "Glob"`
- `"search"`:     `"factory-droid": "Grep"`
- `"web_search"`: `"factory-droid": "WebSearch"`
- `"web_fetch"`:  `"factory-droid": "FetchUrl"`
- `"agent"`:      `"factory-droid": "Task"`

Add `"pi"` to `ToolNames`:

- `"file_read"`:  `"pi": "read"`
- `"file_write"`: `"pi": "write"`
- `"file_edit"`:  `"pi": "edit"`
- `"shell"`:      `"pi": "bash"`
- `"search"`:     `"pi": "grep"`
- `"find"`:       `"pi": "find"`
- `"list"`:       `"pi": "ls"` — Pi exposes an `ls` directory-listing tool distinct from `find`

Add `"factory-droid"` to `HookEvents` for its 9 confirmed events (same PascalCase names as CC):

- `"before_tool_execute"`: `"factory-droid": "PreToolUse"`
- `"after_tool_execute"`:  `"factory-droid": "PostToolUse"`
- `"before_prompt"`:       `"factory-droid": "UserPromptSubmit"`
- `"agent_stop"`:          `"factory-droid": "Stop"`
- `"session_start"`:       `"factory-droid": "SessionStart"`
- `"session_end"`:         `"factory-droid": "SessionEnd"`
- `"before_compact"`:      `"factory-droid": "PreCompact"`
- `"subagent_start"`:      `"factory-droid": "SubagentStart"`
- `"subagent_stop"`:       `"factory-droid": "SubagentStop"`

Add `"pi"` to `HookEvents`. Pi's `agent_end` event covers both `agent_stop` and `subagent_stop` — only add it to `agent_stop` in the toolmap. The Pi adapter handles `subagent_stop` specially in Encode.

- `"before_tool_execute"`: `"pi": "tool_call"`
- `"after_tool_execute"`:  `"pi": "tool_result"`
- `"session_start"`:       `"pi": "session_start"`
- `"session_end"`:         `"pi": "session_shutdown"`
- `"before_prompt"`:       `"pi": "input"`
- `"agent_stop"`:          `"pi": "agent_end"`
- `"before_compact"`:      `"pi": "session_before_compact"`
- `"subagent_start"`:      `"pi": "before_agent_start"`

Pi also exposes provider-specific events with no canonical equivalents. These must be added to `HookEvents` as new canonical keys (using snake_case convention) so they survive round-trip through syllago's canonical format. They are Pi-only: if no other provider maps to them, they simply have a single entry:

- `"turn_start"`:           `"pi": "turn_start"`
- `"turn_end"`:             `"pi": "turn_end"`
- `"model_select"`:         `"pi": "model_select"`
- `"user_bash"`:            `"pi": "user_bash"`
- `"context_update"`:       `"pi": "context"`
- `"message_start"`:        `"pi": "message_start"`
- `"message_end"`:          `"pi": "message_end"`

> **Note:** These 7 canonical keys are Pi-specific extensions. Other provider adapters will emit "unsupported event" warnings when asked to encode these events.

Verify existing Windsurf entries in `HookEvents` are present for the 4 direct-mapped events: `before_prompt`, `agent_stop`, `session_start`, `session_end`. These already exist in the current toolmap.

Confirm `before_tool_execute` and `after_tool_execute` do NOT have windsurf entries — the adapter handles those via split-event logic. Remove them if present.

**Verify existing OpenCode entries match SST's `anomalyco/opencode`** (not the archived `opencode-ai/opencode` Go project). The TypeScript rewrite has different event names. Check each entry in `HookEvents` and `ToolNames` for `"opencode"` against `https://opencode.ai/docs/plugins/`. If any entries reference the old Go project's event/tool names, correct them and note discrepancies in a comment.

**Verify existing `vs-code-copilot` toolmap entries are complete.** The current toolmap should have 18 hook events and the full tool list (Read, Write, Edit, Bash, Glob, Grep, WebFetch, WebSearch, Agent, NotebookEdit, MultiEdit, LS, NotebookRead, KillBash, Skill, AskUserQuestion). Add any missing entries.

Add Windsurf-specific events not yet in toolmap:

```go
"worktree_create":   {..., "windsurf": "post_setup_worktree"},
"transcript_export": {"windsurf": "post_cascade_response_with_transcript"},
```

`transcript_export` is a new canonical event name for the Windsurf transcript hook with no other provider mapping.

### Task 0.2 — Toolmap tests for new entries

**File:** `cli/internal/converter/toolmap_test.go`

Add these test functions:

```go
func TestToolmap_FactoryDroidEntries(t *testing.T) {
    t.Parallel()
    cases := []struct{ canonical, native string }{
        {"shell", "Execute"},
        {"file_write", "Create"},
        {"file_edit", "Edit"},
        {"file_read", "Read"},
        {"agent", "Task"},
    }
    for _, c := range cases {
        t.Run(c.canonical, func(t *testing.T) {
            got := TranslateTool(c.canonical, "factory-droid")
            if got != c.native {
                t.Errorf("TranslateTool(%q, factory-droid) = %q, want %q", c.canonical, got, c.native)
            }
            back := ReverseTranslateTool(c.native, "factory-droid")
            if back != c.canonical {
                t.Errorf("ReverseTranslateTool(%q, factory-droid) = %q, want %q", c.native, back, c.canonical)
            }
        })
    }
}

func TestToolmap_PiEntries(t *testing.T) {
    t.Parallel()
    cases := []struct{ canonical, native string }{
        {"shell", "bash"},
        {"file_read", "read"},
        {"file_write", "write"},
        {"file_edit", "edit"},
    }
    for _, c := range cases {
        t.Run(c.canonical, func(t *testing.T) {
            got := TranslateTool(c.canonical, "pi")
            if got != c.native {
                t.Errorf("TranslateTool(%q, pi) = %q, want %q", c.canonical, got, c.native)
            }
            back := ReverseTranslateTool(c.native, "pi")
            if back != c.canonical {
                t.Errorf("ReverseTranslateTool(%q, pi) = %q, want %q", c.native, back, c.canonical)
            }
        })
    }
}

func TestToolmap_FactoryDroidEvents(t *testing.T) {
    t.Parallel()
    cases := []struct{ canonical, native string }{
        {"before_tool_execute", "PreToolUse"},
        {"session_start", "SessionStart"},
        {"subagent_stop", "SubagentStop"},
    }
    for _, c := range cases {
        t.Run(c.canonical, func(t *testing.T) {
            got, ok := TranslateHookEvent(c.canonical, "factory-droid")
            if !ok || got != c.native {
                t.Errorf("TranslateHookEvent(%q, factory-droid) = %q/%v, want %q/true", c.canonical, got, ok, c.native)
            }
        })
    }
}

func TestToolmap_PiEvents(t *testing.T) {
    t.Parallel()
    cases := []struct{ canonical, native string }{
        {"before_tool_execute", "tool_call"},
        {"after_tool_execute", "tool_result"},
        {"session_start", "session_start"},
        {"session_end", "session_shutdown"},
        {"before_prompt", "input"},
        {"agent_stop", "agent_end"},
    }
    for _, c := range cases {
        t.Run(c.canonical, func(t *testing.T) {
            got, ok := TranslateHookEvent(c.canonical, "pi")
            if !ok || got != c.native {
                t.Errorf("TranslateHookEvent(%q, pi) = %q/%v, want %q/true", c.canonical, got, ok, c.native)
            }
        })
    }
}

func TestToolmap_WindsurfSplitEventsAbsent(t *testing.T) {
    // before_tool_execute and after_tool_execute must NOT have windsurf entries
    t.Parallel()
    for _, canonical := range []string{"before_tool_execute", "after_tool_execute"} {
        _, ok := TranslateHookEvent(canonical, "windsurf")
        if ok {
            t.Errorf("windsurf must not have a toolmap entry for %q (uses split-event logic)", canonical)
        }
    }
}
```

**Verify:** `cd cli && go test ./internal/converter/ -run TestToolmap -v`

### Task 0.3 — Extend Verify() with VerifyFields interface

**File:** `cli/internal/converter/adapter.go`

Add the optional interface:

```go
// VerifyField constants for FieldsToVerify() return values.
const (
    VerifyFieldEvent   = "event"
    VerifyFieldName    = "name"
    VerifyFieldMatcher = "matcher"
)

// VerifyFields is an optional interface adapters can implement to declare
// which additional fields Verify() should check beyond command/timeout/blocking.
type VerifyFields interface {
    FieldsToVerify() []string // use VerifyField* constants
}
```

In `Verify()`, after the existing `SupportsBlocking` check, add:

```go
if vf, ok := adapter.(VerifyFields); ok {
    for _, field := range vf.FieldsToVerify() {
        switch field {
        case VerifyFieldEvent:
            if dh.Event != oh.Event {
                return &VerifyError{Provider: slug, Detail: "hook " + itoa(i) + " event mismatch: " + dh.Event + " != " + oh.Event}
            }
        case VerifyFieldName:
            if dh.Name != oh.Name {
                return &VerifyError{Provider: slug, Detail: "hook " + itoa(i) + " name mismatch: " + dh.Name + " != " + oh.Name}
            }
        case VerifyFieldMatcher:
            if string(dh.Matcher) != string(oh.Matcher) {
                return &VerifyError{Provider: slug, Detail: "hook " + itoa(i) + " matcher mismatch: " + string(dh.Matcher) + " != " + string(oh.Matcher)}
            }
        }
    }
}
```

### Task 0.4 — Test for VerifyFields extension

**File:** `cli/internal/converter/adapter_test.go`

Add a compile-time interface check to confirm the type is accessible from adapter implementations:

```go
type stubVerifyFields struct{ ClaudeCodeAdapter }
func (s *stubVerifyFields) FieldsToVerify() []string { return []string{"event", "name", "matcher"} }
var _ VerifyFields = (*stubVerifyFields)(nil)
```

**Verify after Phase 0:** `cd cli && go test ./internal/converter/ && make build`

---

## Phase 1: Windsurf Adapter

### Task 1.1 — Create testdata fixtures for Windsurf

**Dir:** `cli/internal/converter/testdata/windsurf/`

Create `simple.json`:
```json
{"hooks": {"pre_run_command": [{"command": "echo pre-run"}], "post_run_command": [{"command": "echo post-run"}]}}
```

Create `wildcard-expanded.json` — all 4 pre-events with identical command (wildcard merge test):
```json
{
  "hooks": {
    "pre_run_command":  [{"command": "echo guard"}],
    "pre_read_code":    [{"command": "echo guard"}],
    "pre_write_code":   [{"command": "echo guard"}],
    "pre_mcp_tool_use": [{"command": "echo guard"}]
  }
}
```

### Task 1.2 — Implement WindsurfAdapter

**File:** `cli/internal/converter/adapter_windsurf.go`

```go
package converter

import (
    "encoding/json"
    "fmt"
)

func init() { RegisterAdapter(&WindsurfAdapter{}) }

type WindsurfAdapter struct{}
func (a *WindsurfAdapter) ProviderSlug() string { return "windsurf" }

type wsHookEntry struct {
    Command          string `json:"command"`
    ShowOutput       bool   `json:"show_output,omitempty"`
    WorkingDirectory string `json:"working_directory,omitempty"`
}
type wsHooksFile struct {
    Hooks map[string][]wsHookEntry `json:"hooks"`
}
```

**Encode algorithm:**

1. For each hook: `TranslateHandlerType(hook.Handler, "windsurf", hook.Degradation)` — drop non-command with warning
2. If `hook.Handler.Timeout > 0`: warn "timeout not supported by windsurf; dropped"
3. If `hook.Degradation != nil`: warn "degradation not supported by windsurf; dropped"
4. Build `wsHookEntry{Command: hook.Handler.Command, WorkingDirectory: hook.Handler.CWD}`
5. Restore `show_output` from `hook.ProviderData["windsurf"]` if present
6. If `!hook.Blocking` and event is `"before_tool_execute"`: wrap command with `"(" + cmd + ") || true"`, emit info warning about blocking-false wrap
7. Route by event:
   - `"before_tool_execute"` → `wsEventsForMatcher(hook.Matcher, true)` → append entry to each returned event
   - `"after_tool_execute"` → `wsEventsForMatcher(hook.Matcher, false)` → same fan-out
   - other → `TranslateEventToProvider(hook.Event, "windsurf")`, warn+skip on error

**`wsEventsForMatcher(matcher json.RawMessage, pre bool) ([]string, error)`:**

Three distinct cases based on matcher content:
1. **nil matcher** (`len(matcher) == 0` or `matcher == nil`): return all 4 pre or all 4 post events; no error.
2. **unmarshal error** (matcher is present but not a JSON string, e.g. a number or object): return nil + error; caller warns and skips the hook.
3. **valid string, known value**: map as below.
4. **valid string, unknown value** (any string not in the map): return all 4 pre or all 4 post events; no error (treated as unfiltered wildcard).

String mappings:
- `s == "shell"` → `["pre_run_command"]` / `["post_run_command"]`
- `s == "file_read"` → `["pre_read_code"]` / `["post_read_code"]`
- `s == "file_write"` or `"file_edit"` → `["pre_write_code"]` / `["post_write_code"]`
- `s == "mcp"` → `["pre_mcp_tool_use"]` / `["post_mcp_tool_use"]`

**Decode algorithm:**

1. Unmarshal `wsHooksFile`
2. Call `tryMergeWildcard(file.Hooks, ["pre_run_command","pre_read_code","pre_write_code","pre_mcp_tool_use"], "before_tool_execute", true)` — returns merged hook(s) if all 4 events have exactly 1 entry with identical command
3. Same for post-split → `"after_tool_execute"`
4. Track which events were merged; skip them in the main loop
5. For remaining events: derive canonical matcher via `wsMatcherFromEvent`, set `blocking = wsIsPre(wsEvent)`, determine canonical event (`"before_tool_execute"` / `"after_tool_execute"` for split events, else `TranslateEventFromProvider`)
6. Preserve `show_output` and `working_directory` in `ProviderData["windsurf"]` when non-zero

**`wsMatcherFromEvent(wsEvent string) string`:**
- `pre_run_command` / `post_run_command` → `"shell"`
- `pre_read_code` / `post_read_code` → `"file_read"`
- `pre_write_code` / `post_write_code` → `"file_write"`
- `pre_mcp_tool_use` / `post_mcp_tool_use` → `"mcp"`
- other → `""`

**`wsIsPre(wsEvent string) bool`:** Returns true for all 5 pre-events: `pre_run_command`, `pre_read_code`, `pre_write_code`, `pre_mcp_tool_use`, `pre_user_prompt`. Returns false for all post-events and any unknown event name.

**`tryMergeWildcard`:** All 4 split events present, each with exactly 1 entry, same `.Command` → return 1 `CanonicalHook` with no `Matcher`, `ProviderData["windsurf"]["expanded_from"] = "wildcard"`.

**`Capabilities()`:**
```go
ProviderCapabilities{
    Events:           []string{"before_tool_execute","after_tool_execute","before_prompt","agent_stop","session_start","session_end","worktree_create","transcript_export"},
    SupportsMatchers: true,
    SupportsBlocking: true,
    SupportsCWD:      true,
    TimeoutUnit:      "",
}
```

**`FieldsToVerify()`:** `[]string{VerifyFieldEvent, VerifyFieldMatcher}`

### Task 1.3 — Windsurf adapter tests

**File:** `cli/internal/converter/adapter_windsurf_test.go`

Write one function per scenario:

1. `TestWindsurfAdapterDecode_Simple` — `pre_run_command` → `before_tool_execute` + `shell` matcher + `blocking: true`; `post_run_command` → `after_tool_execute` + `blocking: false`
2. `TestWindsurfAdapterEncode_ShellMatcher` — `shell` matcher → only `pre_run_command` in output; no `pre_read_code`
3. `TestWindsurfAdapterEncode_WildcardExpands` — nil matcher → all 4 pre-events in output
4. `TestWindsurfAdapterDecode_WildcardMerges` — 4 identical pre-events → 1 hook, `Matcher == nil`
5. `TestWindsurfAdapterDecode_PartialWildcardNotMerged` — only 2 of 4 pre-events present with identical command → 2 separate hooks with individual matchers, NOT merged; confirms `tryMergeWildcard` requires all 4 events
6. `TestWindsurfAdapterEncode_TimeoutDroppedWithWarning` — `Timeout: 10` → warning emitted; "timeout" not in output
7. `TestWindsurfAdapterEncode_BlockingFalseWrapped` — `blocking: false` pre-hook → command contains `|| true`
8. `TestWindsurfAdapterEncode_DirectMappedEvents` — `session_start` → `"session_start"`, `before_prompt` → `"pre_user_prompt"`
9. `TestWindsurfAdapterEncode_CWDPreserved` — `Handler.CWD: "./scripts"` → `"working_directory": "./scripts"` in output
10. `TestWindsurfAdapterRoundTrip` — encode+Verify passes for shell hook
11. `TestWindsurfAdapterCapabilities` — `SupportsMatchers: true`, `SupportsLLMHooks: false`, `TimeoutUnit: ""`
12. `TestWindsurfAdapterDecode_ShowOutputPreserved` — input with `"show_output": true` → decoded hook has `ProviderData["windsurf"]["show_output"] == "true"`; re-encode → `"show_output": true` in output JSON

### Task 1.4 — Register windsurf in TestAdapterRegistry

**File:** `cli/internal/converter/adapter_registry_test.go`
**Function:** `TestAdapterRegistry`

```go
expected := []string{"claude-code", "gemini-cli", "copilot-cli", "kiro", "cursor", "windsurf"}
```

### Task 1.5 — Verify Phase 1

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt
go test ./internal/converter/ -run TestWindsurf -v
go test ./internal/converter/
make build
```

---

## Phase 2: VS Code Copilot Adapter

### Task 2.1 — Create testdata fixtures for vs-code-copilot

**Dir:** `cli/internal/converter/testdata/vs-code-copilot/`

Create `simple.json` — identical schema to CC:
```json
{"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo check", "timeout": 5000}]}]}}
```

Create `platform-commands.json`:
```json
{"hooks": {"PreToolUse": [{"hooks": [{"type": "command", "command": "echo default", "platform": {"darwin": "echo mac", "linux": "echo linux"}}]}]}}
```

Create `custom-env.json`:
```json
{"hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "echo init", "env": {"AUDIT_LOG": "/tmp/audit.log"}}]}]}}
```

### Task 2.2 — Implement VSCodeCopilotAdapter

**File:** `cli/internal/converter/adapter_vs_code_copilot.go`

VS Code Copilot uses the same JSON structure as Claude Code. Both `platform` and `env` fields already exist in `HookHandler` and CC already passes them through.

Define `vscHookEntry`, `vscMatcherGroup`, `vscHooksFile` structs with identical JSON fields to CC's `ccHookEntry`, `ccMatcherGroup`, `ccHooksFile`. Implement `Encode` and `Decode` identical to `ClaudeCodeAdapter` but with slug `"vs-code-copilot"` throughout.

**Encode algorithm step — LLM hook handling:** Before encoding each hook, check `hook.Handler.Type`. If `hook.Handler.Type != ""` and `hook.Handler.Type != "command"`: emit warning `"vs-code-copilot does not support handler type %q; hook dropped"` and skip. This handles the case where a hook was imported from a provider that supports LLM/HTTP hooks.

`EncodedResult.Filename`: `"hooks.json"`.

**`Capabilities()`:**
```go
ProviderCapabilities{
    Events: []string{
        "before_tool_execute","after_tool_execute","before_prompt",
        "agent_stop","session_start","session_end","before_compact",
        "notification","subagent_start","subagent_stop",
        "error_occurred","tool_use_failure",
    },
    SupportsMatchers: true, SupportsAsync: true, SupportsStatusMessage: true,
    SupportsStructuredOutput: true, SupportsBlocking: true,
    TimeoutUnit: "milliseconds", SupportsPlatform: true,
    SupportsCWD: true, SupportsEnv: true,
    SupportsLLMHooks: false, SupportsHTTPHooks: false,
}
```

**`FieldsToVerify()`:** `[]string{VerifyFieldEvent, VerifyFieldName, VerifyFieldMatcher}`

> **Note on platform/env:** `platform` and `env` fields are lossless through encode+decode (they survive roundtrip via `ProviderData`), but Verify() does not check them because `VerifyFields` only exposes keys that map to top-level `CanonicalHook` fields (`event`, `name`, `matcher`). Platform/env are stored in `HookHandler` sub-fields and survive by virtue of identical struct encoding — no additional FieldsToVerify entry needed.

### Task 2.3 — VS Code Copilot adapter tests

**File:** `cli/internal/converter/adapter_vs_code_copilot_test.go`

1. `TestVSCodeCopilotAdapterDecode` — decode simple hook, verify event/command/timeout
2. `TestVSCodeCopilotAdapterEncode` — output contains "PreToolUse", "Bash", "5000"
3. `TestVSCodeCopilotAdapter_PlatformCommands_RoundTrip` — `platform` map survives encode+decode
4. `TestVSCodeCopilotAdapter_EnvMap_RoundTrip` — `env` map survives encode+decode
5. `TestVSCodeCopilotAdapter_UnsupportedEvent_Dropped` — `worktree_create` → warning, 0 hooks after decode
6. `TestVSCodeCopilotAdapter_LLMHookDropped` — `prompt` type → warning emitted
7. `TestVSCodeCopilotAdapterCapabilities` — `SupportsPlatform: true`, `SupportsEnv: true`, `SupportsLLMHooks: false`
8. `TestVSCodeCopilotAdapterRoundTrip` — encode+Verify passes

### Task 2.4 — Register vs-code-copilot in TestAdapterRegistry

**File:** `cli/internal/converter/adapter_registry_test.go`
**Function:** `TestAdapterRegistry`

```go
expected := []string{"claude-code", "gemini-cli", "copilot-cli", "kiro", "cursor", "windsurf", "vs-code-copilot"}
```

### Task 2.5 — Verify Phase 2

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt
go test ./internal/converter/ -run TestVSCodeCopilot -v
go test ./internal/converter/
make build
```

---

## Phase 3: Factory Droid Adapter

### Task 3.1 — Create testdata fixtures for factory-droid

**Dir:** `cli/internal/converter/testdata/factory-droid/`

Create `simple.json` — uses `Execute` (not `Bash`):
```json
{"hooks": {"PreToolUse": [{"matcher": "Execute", "hooks": [{"type": "command", "command": "echo check", "timeout": 5000}]}]}}
```

Create `all-events.json` — one hook per each of the 9 supported events with their PascalCase names: `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `Stop`, `SessionStart`, `SessionEnd`, `PreCompact`, `SubagentStart`, `SubagentStop`.

### Task 3.2 — Implement FactoryDroidAdapter

**File:** `cli/internal/converter/adapter_factory_droid.go`

Factory Droid uses identical JSON structure to Claude Code. Differences:
1. `ProviderSlug()` returns `"factory-droid"`
2. 9 supported events (toolmap handles rejection via `TranslateEventToProvider` returning error → encode converts to warning)
3. Tool names differ: `Execute` / `Create` — handled by toolmap
4. `EncodedResult.Filename`: `"settings.json"` — **verify against Factory Droid docs before writing** (likely `.factory/settings.json`; confirm the bare filename is correct, not a full path like `factory-hooks.json`)

Define `fdHookEntry`, `fdMatcherGroup`, `fdHooksFile` with identical JSON fields to CC counterparts. Implement `Encode`/`Decode` identical to `ClaudeCodeAdapter` but using slug `"factory-droid"` and filename `"settings.json"`.

**`Capabilities()`:**
```go
ProviderCapabilities{
    Events: []string{
        "before_tool_execute","after_tool_execute","before_prompt",
        "agent_stop","session_start","session_end","before_compact",
        "subagent_start","subagent_stop",
    },
    SupportsMatchers: true, SupportsStatusMessage: true,
    SupportsBlocking: true, TimeoutUnit: "milliseconds",
    SupportsCWD: true, SupportsEnv: true,
    SupportsStructuredOutput: false, // unconfirmed; conservative
}
```

**`FieldsToVerify()`:** `[]string{VerifyFieldEvent, VerifyFieldName, VerifyFieldMatcher}`

### Task 3.3 — Factory Droid adapter tests

**File:** `cli/internal/converter/adapter_factory_droid_test.go`

1. `TestFactoryDroidAdapterDecode` — `Execute` matcher → canonical `shell`
2. `TestFactoryDroidAdapterEncode` — `shell` matcher → `Execute` in output; filename is `settings.json`
3. `TestFactoryDroidAdapterEncode_WriteBecomesCreate` — `file_write` matcher → `Create`
4. `TestFactoryDroidAdapterEncode_CCExclusiveEventDropped` — `worktree_create` → warning emitted
5. `TestFactoryDroidAdapterRoundTrip` — encode+Verify passes
6. `TestFactoryDroidAdapterCapabilities` — `SupportsMatchers: true`, `SupportsLLMHooks: false`, `len(Events) == 9`

### Task 3.4 — Register factory-droid in TestAdapterRegistry

**File:** `cli/internal/converter/adapter_registry_test.go`
**Function:** `TestAdapterRegistry`

```go
expected := []string{"claude-code", "gemini-cli", "copilot-cli", "kiro", "cursor", "windsurf", "vs-code-copilot", "factory-droid"}
```

### Task 3.5 — Verify Phase 3

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt
go test ./internal/converter/ -run TestFactoryDroid -v
go test ./internal/converter/
make build
```

---

## Phase 4: Pi Adapter

Pi generates TypeScript code and parses it back. Four sub-phases.

### Phase 4a: jsparse.go

#### Task 4a.1 — Add esbuild dependency

```bash
cd /home/hhewett/.local/src/syllago/cli && go get github.com/evanw/esbuild/pkg/api
CGO_ENABLED=0 go build ./...
```

**esbuild import path (verified):** `github.com/evanw/esbuild/pkg/api` — this is the correct and stable import path for esbuild's Go API.

Note on go-fAST: `github.com/T14Raptor/go-fAST` is an ES6+ AST parser for Go. Its import path must be confirmed from the repository README before adding it as a dependency (`go get github.com/T14Raptor/go-fAST`). If the module is unavailable, has insufficient ES6 arrow function support, or the import path differs from the repo name, `walkHooksAST` is implemented as a stub returning empty + info warning. The marker-decode path is fully functional regardless of go-fAST availability.

#### Task 4a.2 — Implement jsparse.go

**File:** `cli/internal/converter/jsparse.go`

Imports: `"errors"`, `"fmt"`, `"strings"`, `esbuild "github.com/evanw/esbuild/pkg/api"`

Key types:
```go
const jsparseSizeLimit = 1 * 1024 * 1024

type JSParseResult struct {
    Hooks        []JSHookDef
    DecodeMethod string // "marker" or "heuristic"
    Warnings     []ConversionWarning
}

type JSHookDef struct {
    PiEvent        string
    Event          string            // canonical event name (populated from Markers["event"] during marker decode)
    Command        string
    Timeout        int               // milliseconds
    Blocking       bool
    ToolMatcher    string
    Markers        map[string]string
    ConfidenceHigh bool
}

var ErrFileTooLarge = errors.New("jsparse: file exceeds 1MB size limit")
```

**`ParsePiExtension(tsCode []byte) (*JSParseResult, error)`:**
1. Return `ErrFileTooLarge` if `len(tsCode) > jsparseSizeLimit`
2. If `hasSyllagoHeader(tsCode)` → try `parseByMarkers` → if ok and `len(hooks) > 0`, set `DecodeMethod = "marker"` and return
3. Fall through to `parseByHeuristic`

**`hasSyllagoHeader(code []byte) bool`:** First 22 bytes equal `"// Generated by syllago"`.

**`stripTypeScript(tsCode []byte) ([]byte, error)`:** Call `esbuild.Transform` with `LoaderTS`. Return error if `len(result.Errors) > 0`.

**`parseByMarkers(tsCode []byte) (*JSParseResult, error)`:**
- Split into lines, scan for `// @syllago:` prefix
- Accumulate key=value pairs into `current map[string]string`
- When a non-marker, non-empty line is hit after markers: if `current["pi_event"] != ""`, flush to `JSHookDef{PiEvent, Markers, Blocking: current["blocking"]=="true", ToolMatcher: current["matcher"], Command: current["command"], ConfidenceHigh: true}`
- After scan: call `crossValidateMarkerCommands(tsCode, result.Hooks)`

**`crossValidateMarkerCommands`:** Extract `execSync` first-arg strings via `extractExecSyncCommands`. For each hook where marker command != code command: emit `Severity: "error"` warning about marker poisoning, update `hooks[i].Command` to code value.

**`extractExecSyncCommands(tsCode []byte) []string`:** Line-by-line scan for lines containing `execSync(`. Extract the first string argument using these rules:
- Accept single-quoted (`'...'`) or double-quoted (`"..."`) string literals only
- Do NOT attempt to parse template literals (backticks) — skip the line with an info warning
- Handle simple escaped quotes within the same quote style (`\'` inside single-quoted, `\"` inside double-quoted) — stop at first unescaped closing delimiter
- Do NOT attempt multi-line argument extraction — only consider arguments on the same line as `execSync(`
- Skip lines where no string argument can be cleanly extracted (complex expressions, variables, template literals)

**`parseByHeuristic`:** Call `stripTypeScript`. On error: set `DecodeMethod = "heuristic"`, append a warning with `Severity: "warn"` and message `"TypeScript parse failed — Pi extension requires manual review"`, and append one stub `JSHookDef` with `Command: "# MANUAL: see original Pi extension at <path>"`. Return result without error (caller sees the warning, not a hard failure). On success: call `walkHooksAST`.

**`walkHooksAST(jsCode []byte) ([]JSHookDef, []ConversionWarning)`:** Stub — returns nil hooks and one `Severity: "info"` warning that heuristic AST decode is not yet implemented.

#### Task 4a.3 — jsparse tests

**File:** `cli/internal/converter/jsparse_test.go`

1. `TestParsePiExtension_SizeLimitEnforced` — oversized input → `ErrFileTooLarge`
2. `TestHasSyllagoHeader_True` — code with header → true
3. `TestHasSyllagoHeader_False_HandWritten` — `@syllago:` without header → false
4. `TestHasSyllagoHeader_False_Empty` — empty → false
5. `TestStripTypeScript_RemovesTypes` — TS with `: string` annotation → JS without it, value preserved
6. `TestStripTypeScript_InvalidSyntax` — malformed TS → non-nil error
7. `TestParsePiExtension_MarkerDecode` — syllago-generated fixture → `DecodeMethod == "marker"`, 1+ hooks
8. `TestParsePiExtension_HeuristicFallback` — hand-written fixture → `DecodeMethod == "heuristic"` (0 hooks ok)
9. `TestExtractExecSyncCommands_Quotes` — both single-quote and double-quote forms extracted correctly

### Phase 4b: Pi code generation templates

#### Task 4b.1 — Implement adapter_pi_templates.go

**File:** `cli/internal/converter/adapter_pi_templates.go`

Imports: `"bytes"`, `"encoding/json"`, `"fmt"`, `"strings"`, `"text/template"`

**`jsString(s string) string`:** `json.Marshal(s)` → return as string. Returns `""` on error (never happens for strings).

**`jsStringNoNewlines(s string) string`:** Replace `\n` and `\r` with space, then `jsString`.

Types:
```go
type piHookTemplateData struct {
    Name, PiEvent, Command, ToolMatcher string
    TimeoutMs int
    Blocking  bool
}
type piExtensionTemplateData struct {
    Hooks []piHookTemplateData
}
```

Template constant (`piTemplate`). Full Go template code:

**Marker lines use `jsStringNoNewlines` for all string fields to prevent newline injection that would break the comment context.** The template uses `jsStringNoNewlines` (not `jsString`) for `.Name`, `.PiEvent`, `.ToolMatcher`, and `.Command` in marker comments. The `jsString` function (which adds JSON quotes) is used for runtime string literals only.

```go
const piTemplate = `// Generated by syllago — do not edit marker comments
import { execSync } from "child_process";
import type { ExtensionContext } from "@badlogic/pi";

export function activate(ctx: ExtensionContext): void {
{{- range .Hooks}}
// @syllago:name={{jsStringNoNewlines .Name}}
// @syllago:pi_event={{jsStringNoNewlines .PiEvent}}
{{- if .ToolMatcher}}
// @syllago:matcher={{jsStringNoNewlines .ToolMatcher}}
{{- end}}
// @syllago:blocking={{.Blocking}}
{{- if .TimeoutMs}}
// @syllago:timeout={{.TimeoutMs}}
{{- end}}
  ctx.hooks.on({{jsString .PiEvent}}, (event) => {
{{- if .ToolMatcher}}
    if (event.tool !== {{jsString .ToolMatcher}}) return;
{{- end}}
{{- if .Blocking}}
    try {
      execSync({{jsString .Command}}, { stdio: "pipe"{{if .TimeoutMs}}, timeout: {{.TimeoutMs}}{{end}} });
    } catch (err: any) {
      if (err.status === 2 || err.killed === true || err.status === null) {
        throw new Error(err.stderr?.toString() || "hook failed");
      }
    }
{{- else}}
    execSync({{jsString .Command}}, { stdio: "pipe"{{if .TimeoutMs}}, timeout: {{.TimeoutMs}}{{end}} });
{{- end}}
  });
{{end -}}
}
`
```

**`renderPiExtension(data piExtensionTemplateData) ([]byte, error)`:** Execute `piTemplate` into a buffer using `template.New("pi").Funcs(template.FuncMap{"jsString": jsString}).Parse(piTemplate)`.

Register template funcs including `jsString`.

#### Task 4b.2 — Template tests

**File:** `cli/internal/converter/adapter_pi_templates_test.go`

1. `TestJsString_BasicEscaping` — quotes, backslash, newline all escaped via JSON rules
2. `TestJsString_CommandInjectionPrevention` — `$(rm -rf /)` command → no raw `$(` in output; output is valid JSON string literal
3. `TestJsStringNoNewlines_StripsCRLF` — CR/LF removed
4. `TestRenderPiExtension_BasicOutput` — header present, event name, command, timeout visible
5. `TestRenderPiExtension_BlockingFailClosed` — `Blocking: true` → all three: `err.status === 2`, `err.killed === true`, `err.status === null`
6. `TestRenderPiExtension_NonBlockingNoThrow` — `Blocking: false` → no `throw new Error`
7. `TestRenderPiExtension_CommandInjectionInTemplate` — malicious command → no literal injection in output

### Phase 4c: Pi adapter implementation

#### Task 4c.1 — Create testdata for Pi

**Dir:** `cli/internal/converter/testdata/pi/`

Create `generated-simple.ts` (syllago-generated with full marker block and `execSync` call — see design doc example).

Create `handwritten-simple.ts` (no syllago header, plain TypeScript with `tool_call` and `session_start` handlers using `execSync`).

#### Task 4c.2 — Implement adapter_pi.go

**File:** `cli/internal/converter/adapter_pi.go`

Imports: `"encoding/json"`, `"fmt"`, `"strings"` (for string ops in Decode)

**`Encode`:**
1. `TranslateHandlerType` → drop non-command
2. Warn on `hook.Handler.Env != nil` (env injection not supported)
3. Warn on `hook.Degradation != nil`
4. `subagent_stop` → `piEvent = "agent_end"` (hardcoded; no toolmap entry for this Pi→canonical direction)
5. Other events → `TranslateEventToProvider(hook.Event, "pi")`, warn+skip on error
6. Extract tool matcher: unmarshal `hook.Matcher` as string, then `TranslateTool(s, "pi")`
7. `timeoutMs = hook.Handler.Timeout * 1000`
8. Append to `templateHooks`
9. If empty: return stub file; else call `renderPiExtension`
10. `EncodedResult.Filename = "syllago-hooks.ts"`

**`Decode`:**
1. `ParsePiExtension(content)` — return error on `ErrFileTooLarge`
2. For each `JSHookDef`:
   - Canonical event: prefer `jshook.Markers["event"]` if marker decode and non-empty; fall back to `ReverseTranslateHookEvent(jshook.PiEvent, "pi")`; if both empty, emit warning and skip hook
   - Matcher: `ReverseTranslateTool(jshook.ToolMatcher, "pi")` → `json.Marshal`; if `ToolMatcher == ""`, leave `Matcher` as nil
   - Name: `jshook.Markers["name"]` if marker decode and non-empty; otherwise leave empty string (not an error)
   - Timeout: parse `jshook.Markers["timeout"]` as ms → divide by 1000 if non-empty; else use `jshook.Timeout / 1000`
   - Blocking: `jshook.Markers["blocking"] == "true"` if marker decode; else `jshook.Blocking`
   - If heuristic decode and `!jshook.ConfidenceHigh`: set `ProviderData["pi"]["decode_confidence"] = "heuristic"`

**`Capabilities()`:**
```go
ProviderCapabilities{
    Events: []string{
        "before_tool_execute","after_tool_execute","session_start","session_end",
        "before_prompt","agent_stop","before_compact","subagent_start","subagent_stop",
    },
    SupportsMatchers: true, SupportsBlocking: true,
    TimeoutUnit: "milliseconds",
}
```

**`FieldsToVerify()`:** `[]string{VerifyFieldEvent}` — only event is reliably lossless via markers; matcher may be lost in heuristic decode.

#### Task 4c.3 — Pi adapter tests

**File:** `cli/internal/converter/adapter_pi_test.go`

Imports: `"encoding/json"`, `"strings"`, `"testing"`

1. `TestPiAdapterEncode_Basic` — "Generated by syllago" in output, `"tool_call"` event, quoted command, filename `"syllago-hooks.ts"`
2. `TestPiAdapterEncode_MatcherBecomesTool` — `shell` matcher → `"bash"` in if-guard
3. `TestPiAdapterEncode_SubagentStopMapsToAgentEnd` — `subagent_stop` → `"agent_end"` event
4. `TestPiAdapterEncode_TimeoutInMs` — `Timeout: 10` → `timeout: 10000`
5. `TestPiAdapterEncode_BlockingFailClosed` — `Blocking: true` → all three conditions in catch
6. `TestPiAdapterEncode_CommandInjectionSafe` — malicious command → no literal injection
7. `TestPiAdapterEncode_UnsupportedEventDropped` — `worktree_create` → warning
8. `TestPiAdapterDecode_MarkerBased` — `generated-simple.ts` → name/event/blocking/timeout preserved
9. `TestPiAdapterDecode_Heuristic` — `handwritten-simple.ts` (no syllago header) → at least 0 hooks returned without error, `DecodeMethod == "heuristic"` in parse result; confirms fallback path executes without panic
10. `TestPiAdapterCapabilities` — `SupportsBlocking: true`, `SupportsEnv: false`, `SupportsLLMHooks: false`

### Task 4d — Register pi in TestAdapterRegistry

**File:** `cli/internal/converter/adapter_registry_test.go`
**Function:** `TestAdapterRegistry`

```go
expected := []string{"claude-code", "gemini-cli", "copilot-cli", "kiro", "cursor", "windsurf", "vs-code-copilot", "factory-droid", "pi"}
```

### Task 4e — Verify Phase 4

```bash
cd /home/hhewett/.local/src/syllago/cli && go get github.com/evanw/esbuild/pkg/api
make fmt
CGO_ENABLED=0 go build ./...
go test ./internal/converter/ -run TestPi -v
go test ./internal/converter/ -run TestJsparse -v
go test ./internal/converter/ -run TestRender -v
go test ./internal/converter/
make build
```

---

## Phase 5: Provider Research Docs

**Validation requirement (applies to all tasks in this phase):** After creating or updating each provider's docs, a Sonnet subagent validates every claim against actual source code or official documentation. The validator reads each doc file and checks all stated facts (event names, file paths, config schemas, tool names) against the primary sources listed in the design doc's Research Sources section. Fix every issue flagged. Re-run the validator on the updated file. Repeat until the validator returns zero issues. Do not advance to the next provider task until the current one passes validation.

### Task 5.1 — Factory Droid provider docs

> **Identity note:** "Factory Droid" refers to Factory AI's coding agent CLI (`factory-droid`). No known naming conflicts with other tools as of 2026-03-30.

**Dir:** `docs/providers/factory-droid/` — 4 files

Audit metadata block for all files:
```
provider: factory-droid
provider_version: "latest"
report_format: 1
researched: 2026-03-30
researcher: claude-sonnet-4-6
changelog_checked: https://docs.factory.ai/changelog/release-notes
```

`hooks.md`: Config at `.factory/settings.json`. 9 confirmed events (same JSON schema as CC, PascalCase names). Exit code 2 blocks on pre-hooks. Tool names `Execute`/`Create` differ from CC. Source: `[Unverified] https://docs.factory.ai/cli/configuration/hooks-guide` — verify URL returns 200 before marking [Official].

`tools.md`: Read, Edit, Create, Execute, Glob, Grep, WebSearch, FetchUrl, Task. Note CC divergence. Source: `[Unverified] https://docs.factory.ai/cli/configuration` — verify URL returns 200 before marking [Official].

`content-types.md`: Rules (`AGENTS.md`), Skills (`.factory/skills/`), hooks (`.factory/settings.json`), MCP, Custom Droids.

`skills-agents.md`: Custom Droids (`.factory/droids/`), Skills format, model selection, Droid Shield.

Also create `docs/provider-sources/factory-droid.yaml` (from `_template.yaml`) and `docs/provider-formats/factory-droid.md`.

Mark unverifiable claims `[Unverified]`. Mark all verified claims with `[Official]` and URLs.

**Validation step:** Dispatch Sonnet subagent to validate all 4 files + `factory-droid.yaml` against `https://docs.factory.ai/cli/configuration/hooks-guide`, `https://docs.factory.ai/cli/configuration/settings`, and `https://github.com/Factory-AI/factory`. Fix all issues. Re-validate until zero issues. Provide read-only instructions: "Do NOT write files — report issues as a list only."

### Task 5.2 — Pi provider docs

> **Identity note:** "Pi" refers to Mario Zechner's `pi` coding agent (`github.com/badlogic/pi-mono`). Not to be confused with Raspberry Pi hardware or other "Pi" named projects. No naming conflicts relevant to syllago's scope.

**Dir:** `docs/providers/pi/` — 4 files

```
provider: pi
provider_version: "0.64.0"
report_format: 1
researched: 2026-03-30
researcher: claude-sonnet-4-6
changelog_checked: https://github.com/badlogic/pi-mono/releases
```

`hooks.md`: Extensions at `.pi/extensions/*.ts`. Events from `types.ts` — verify exact path via `github.com/badlogic/pi-mono` repository tree before writing docs (likely `packages/core/src/extensions/types.ts` or `packages/coding-agent/src/extensions/types.ts`). Blocking via `throw`. Runtime: Jiti/Bun. No declarative format.

`tools.md`: `read`, `write`, `edit`, `bash`, `grep`, `find`, `ls` and Pi-specific tools from types.ts.

`content-types.md`: Extensions, Settings (`.pi/settings.json`), Skills (`.pi/skills/`), prompt templates.

`skills-agents.md`: Skills format per `packages/coding-agent/docs/skills.md`.

Also create `docs/provider-sources/pi.yaml`.

**Validation step:** Dispatch Sonnet subagent to validate all 4 files + `pi.yaml` against `https://github.com/badlogic/pi-mono` (check repository tree for correct path to `types.ts`, `skills.md`, `settings.md`, `extensions.md`) and `https://www.npmjs.com/package/@mariozechner/pi-coding-agent`. Fix all issues. Re-validate until zero issues.

### Task 5.3 — Crush provider docs

> **Scope note:** Crush is **research-only** for this tier. Crush has no hook system, so no adapter will be implemented. This task documents findings for future reference and sets the baseline for if/when Crush adds hooks.

**Dir:** `docs/providers/crush/` — 4 files

```
provider: crush
provider_version: "latest"
report_format: 1
researched: 2026-03-30
researcher: claude-sonnet-4-6
changelog_checked: https://github.com/charmbracelet/crush/releases
```

Key finding: Crush has NO hook system. `internal/event/` is PostHog telemetry only. Plugin proposal (issue #2038) is open, not implemented.

`hooks.md`: Explicitly document that hooks are not supported. Reference issue #2038.

`content-types.md`: `crush.json` config schema from `raw.githubusercontent.com/charmbracelet/crush/main/schema.json`. Rules (`AGENT.md`). No MCP, no hooks.

`tools.md`: Tools from source inspection.

`skills-agents.md`: Custom agents not supported as of 2026-03-30.

Also create `docs/provider-sources/crush.yaml`.

**Validation step:** Dispatch Sonnet subagent to validate all 4 files + `crush.yaml` against `https://github.com/charmbracelet/crush`, `https://raw.githubusercontent.com/charmbracelet/crush/main/schema.json`, and issue #2038. Fix all issues. Re-validate until zero issues.

### Task 5.4 — VS Code Copilot provider docs

> **Identity note:** "VS Code Copilot" refers to GitHub Copilot's agent mode within VS Code (`vs-code-copilot`). Distinct from `copilot-cli` (GitHub Copilot in the terminal, `gh copilot`). Both are GitHub Copilot products but with separate hook systems and configs.

**Dir:** `docs/providers/vs-code-copilot/` — 4 files

```
provider: vs-code-copilot
provider_version: "latest"
report_format: 1
researched: 2026-03-30
researcher: claude-sonnet-4-6
changelog_checked: https://github.blog/changelog/
```

`hooks.md`: 18 events, same JSON format as CC (PascalCase, matcher groups), `platform` per-OS commands, `env` injection.

`tools.md`: Same as CC (PascalCase): Read, Write, Edit, Bash, Glob, Grep, WebFetch, WebSearch, Agent, NotebookEdit, MultiEdit, LS, NotebookRead, KillBash, Skill, AskUserQuestion.

`content-types.md`: Rules (`.github/copilot-instructions.md`), MCP config location, hooks config.

`skills-agents.md`: No equivalent to CC skills/agents via config files.

**Validation step:** Dispatch Sonnet subagent to validate all 4 files against `https://docs.github.com/copilot` and the VS Code Copilot extension docs. Fix all issues. Re-validate until zero issues.

### Task 5.5 — Update OpenCode provider docs

**File:** `docs/providers/opencode/hooks.md`

Add identity note at the top:

```
> **Identity note (2026-03-30):** Two projects were called "OpenCode". This documentation covers
> **SST's OpenCode** (anomalyco/opencode, 133K stars, TypeScript, actively maintained).
> The original Go project (opencode-ai/opencode) was archived September 2025 and became
> **Crush** under Charm. Syllago's toolmap targets SST's version exclusively.
```

Update `researched` date in metadata block to `2026-03-30`.

**Validation step:** Dispatch Sonnet subagent to validate updated `hooks.md` against `https://opencode.ai/docs/plugins/` and `https://github.com/anomalyco/opencode`. Confirm all event names, plugin structure, and config paths reflect SST's TypeScript version (not the archived Go project). Fix all issues. Re-validate until zero issues.

### Task 5.6 — Verify Windsurf provider docs metadata

**File:** `docs/providers/windsurf/hooks.md`

If the audit metadata block is missing, prepend it:
```
provider: windsurf
provider_version: "latest"
report_format: 1
researched: 2026-03-30
researcher: claude-sonnet-4-6
changelog_checked: https://docs.windsurf.com/changelog
```

If `docs/provider-sources/windsurf.yaml` does not exist, create it from `_template.yaml`.

**Validation step:** Dispatch Sonnet subagent to validate `docs/providers/windsurf/hooks.md` against `https://docs.windsurf.com/windsurf/cascade/hooks`. Confirm the 12 events listed match current Windsurf docs. Fix all issues. Re-validate until zero issues.

---

## Phase 6: Final Validation

### Task 6.1 — Full test suite

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./... 2>&1 | tail -30
```

All existing tests must pass. No regressions.

### Task 6.2 — Coverage check

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -coverprofile=cov.out
go tool cover -func=cov.out | grep total
go tool cover -func=cov.out | grep "adapter_"
```

Target: >=80% on `cli/internal/converter/`.

### Task 6.3 — CGO_ENABLED=0 build

```bash
cd /home/hhewett/.local/src/syllago/cli && CGO_ENABLED=0 go build ./...
```

Must succeed. Confirms no CGO deps were introduced.

### Task 6.4 — Adapter registry completeness

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestAdapterRegistry -v
```

Expected: PASS with all 9 adapters: `claude-code`, `gemini-cli`, `copilot-cli`, `kiro`, `cursor`, `windsurf`, `vs-code-copilot`, `factory-droid`, `pi`.

> **Note:** The `TestAdapterRegistry` test in `adapter_registry_test.go` should compare against `len(expected)` dynamically (e.g. `if got := len(adapters); got != len(expected)`) rather than hardcoding `9`. This way, if a phase is deferred and a later phase's adapter is registered, the expected slice drives the count — update the expected slice, not a magic number.

### Task 6.5 — Round-trip Verify checks

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run "TestRoundTrip|TestVerify" -v
```

All Verify tests must pass for the 4 new adapters.

### Task 6.6 — Final build and smoke test

```bash
cd /home/hhewett/.local/src/syllago && make build
syllago help
```

### Task 6.7 — gofmt compliance

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && git diff --name-only
```

No files modified by `make fmt`.

---

## Success Checklist

- [ ] `TestAdapterRegistry` passes with 9 adapters
- [ ] `TestWindsurf*` — split-event expansion/merge, wildcard round-trip, timeout warning, blocking-false wrap
- [ ] `TestVSCodeCopilot*` — platform + env round-trip, unsupported event dropped
- [ ] `TestFactoryDroid*` — Execute/Create tool names, settings.json filename, 9-event cap enforced
- [ ] `TestPi*` — blocking fail-closed (3 conditions), command injection safe, marker decode lossless
- [ ] `TestJsparse*` — size limit, header detection, esbuild strip, marker parse, cross-validation
- [ ] `TestRenderPiExtension*` — fail-closed blocking template, no command injection in output; marker lines use `jsStringNoNewlines` for all string fields
- [ ] `TestToolmap_FactoryDroidEntries` — Execute/Create reverse-translate correctly
- [ ] `TestToolmap_PiEntries` — includes `ls` → `list` translation
- [ ] `TestToolmap_WindsurfSplitEventsAbsent` — no windsurf entry for before/after_tool_execute
- [ ] Pi toolmap has 7 Pi-specific canonical event entries (turn_start, turn_end, model_select, user_bash, context_update, message_start, message_end)
- [ ] OpenCode toolmap entries verified against SST's `anomalyco/opencode` (not archived Go project)
- [ ] `vs-code-copilot` toolmap verified complete (18 events, full tool list)
- [ ] `CGO_ENABLED=0 go build ./...` — clean
- [ ] Coverage >=80% on `cli/internal/converter/`
- [ ] Provider docs: factory-droid (new), pi (new), crush (new), vs-code-copilot (new), opencode (identity note added), windsurf (metadata verified)
- [ ] All 6 provider docs validated by Sonnet subagent with zero issues
- [ ] `make build` produces working binary
- [ ] `make fmt` produces no diff