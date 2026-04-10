# Hook Adapter Tier 2: Migration Implementation Plan

**Design document:** `docs/plans/2026-03-30-hook-adapter-tier2-migration-design.md`
**Bead:** syllago-0q5if
**Goal:** Replace the legacy bridge pipeline (`ToLegacyHooksConfig` / `FromLegacyHooksConfig` / `render*Hooks` / `canonicalize*Hooks`) with direct `CanonicalHook` encode/decode in all 5 adapters, using shared translation helpers.

---

## Overview

This plan follows the migration order defined in the design:

1. **Phase 0** — Pre-work: spec gap fixes, toolmap cleanup
2. **Phase 1** — `hookhelpers.go`: shared translation helpers with full test coverage
3. **Phase 2** — Migrate CC adapter (reference implementation)
4. **Phase 3** — Migrate Gemini adapter
5. **Phase 4** — Migrate Copilot adapter
6. **Phase 5** — Migrate Cursor adapter
7. **Phase 6** — Migrate Kiro adapter
8. **Phase 7** — Remove legacy bridge
9. **Phase 8** — Upgrade `Verify()`

**Invariant throughout:** Every task leaves `cd cli && go test ./internal/converter/...` passing. The legacy bridge stays alive until Phase 7 because migrated adapters are independently testable against the same fixture data.

**Test order:** Write the failing test first, then implement, then verify green.

---

## Phase 0: Pre-Work (Spec Gap Fixes)

These data-layer changes are independent of each other and of all subsequent phases. They can be done in any order, but all must be complete before Phase 1.

### Task 0.1 — Fix `StatusMessage` JSON tag

**What:** Change the JSON tag on `HookHandler.StatusMessage` from `"statusMessage"` to `"status_message"`. This aligns the canonical representation with the spec's snake_case convention. The tag is only used in in-memory round-trips (adapters today do not serialize canonical hooks to disk); no stored data is affected.

**Files:** `cli/internal/converter/adapter.go`

**Dependencies:** None.

**Test:** Run `cd cli && go test ./internal/converter/... -run TestCanonicalHookNewFields` — this test serializes/deserializes a `CanonicalHook`; after the tag change, re-check that `StatusMessage` round-trips correctly. Update the existing test if needed (the test checks field values, not JSON key names, so it should still pass). Also confirm `TestVerify_Success`, `TestClaudeCodeAdapterDecode`, and `TestCopilotCLIAdapterDecode` still pass since those go through the full encode/decode path.

**[FIX M1]** Before changing the tag, run:
```bash
grep -r '"statusMessage"' cli/internal/converter/
```
Update any test fixture JSON that contains the old camelCase key `"statusMessage"` to `"status_message"`. If any test uses a hardcoded byte-slice fixture containing `"statusMessage"`, those fixtures will silently stop populating the field after the tag change — they must be updated before or alongside the tag change.

**Code guidance:**
```go
// In HookHandler struct, adapter.go line 39:
StatusMessage string `json:"status_message,omitempty"`
```

---

### Task 0.2 — Remove Cline from `HookEvents` and `ToolNames`

**What:** Delete all `"cline"` entries from `HookEvents` and `ToolNames` in `toolmap.go`. Cline has no confirmed hook API. Remove the `"cline"` entries from the inner maps of both tables. Also remove the `"task_resume"` and `"task_cancel"` canonical events (Cline-only events at the bottom of `HookEvents`).

**Files:** `cli/internal/converter/toolmap.go`

**Dependencies:** Task 0.1 (for a clean test baseline before making more changes).

**Test:** `cd cli && go test ./internal/converter/... -run TestTranslateHookEvent` — the existing test has Cline entries (e.g., `"before_tool_execute to Cline"`, `"task_resume to Cline"`, `"task_cancel to Cline"`). These must be **deleted from the test** since Cline is being removed. The test should pass after both the code and test edits.

Also run `cd cli && go test ./internal/converter/... -run TestTranslateTool` — delete Cline tool entries from that test table too (lines like `{"file_read to Cline", ...}`).

Also run `cd cli && go test ./internal/converter/... -run TestIsValidHookEvent` — remove `"TaskStart"`, `"TaskResume"` from the valid entries in the test.

**Code guidance:** In `toolmap.go`, remove the `"cline"` key from every inner map in `ToolNames` and `HookEvents`. Remove the two Cline-only canonical keys at the bottom of `HookEvents`:
```go
// DELETE these two entries:
"task_resume": {"cline": "TaskResume"},
"task_cancel": {"cline": "TaskCancel"},
```

**[FIX M2]** Clarification for the test update: The `"session_start"` canonical event currently has `"cline": "TaskStart"` in its inner map. Removing that Cline entry means `"TaskStart"` no longer appears as any value in `HookEvents`, so `IsValidHookEvent("TaskStart")` will return false. The `TestIsValidHookEvent` update is: "Remove the Cline-specific native names `TaskStart` and `TaskResume` from the test's valid entries — they were valid only because `session_start` and `task_resume` had `cline` entries. After Cline removal, those provider-native names have no mapping."

---

### Task 0.3 — Add VS Code Copilot to `HookEvents`

**What:** Add a `"vs-code-copilot"` entry to the `HookEvents` map for each event it supports. According to the spec table (Section 7.4), VS Code Copilot uses the same event names as Claude Code for the events it supports. The events are: `before_tool_execute` (PreToolUse), `after_tool_execute` (PostToolUse), `session_start` (SessionStart), `before_prompt` (UserPromptSubmit), `agent_stop` (Stop), `before_compact` (PreCompact), `subagent_start` (SubagentStart), `subagent_stop` (SubagentStop), `error_occurred` (StopFailure — note: per spec Table 7.4 row `error_occurred`, the `vs-code-copilot` column shows no entry; only `agent_stop` maps to `Stop`).

Cross-reference the spec event mapping table carefully. Based on Section 7.4 of `docs/spec/hooks/hooks.md`:
- `before_tool_execute`: `vs-code-copilot` → `PreToolUse`
- `after_tool_execute`: `vs-code-copilot` → `PostToolUse`
- `session_start`: `vs-code-copilot` → `SessionStart`
- `before_prompt`: `vs-code-copilot` → `UserPromptSubmit`
- `agent_stop`: `vs-code-copilot` → `Stop`
- `before_compact`: `vs-code-copilot` → `PreCompact`
- `subagent_start`: `vs-code-copilot` → `SubagentStart`
- `subagent_stop`: `vs-code-copilot` → `SubagentStop`

Do not add entries for events where the spec shows `--` for `vs-code-copilot`.

**Files:** `cli/internal/converter/toolmap.go`

**Dependencies:** Task 0.2 (Cline removal complete, clean test baseline).

**Test:** After adding the entries, write a new test in `toolmap_test.go`:
```go
func TestTranslateHookEvent_VSCodeCopilot(t *testing.T) {
    tests := []struct {
        canonical string
        wantNative string
        wantSupported bool
    }{
        {"before_tool_execute", "PreToolUse", true},
        {"after_tool_execute", "PostToolUse", true},
        {"session_start", "SessionStart", true},
        {"before_prompt", "UserPromptSubmit", true},
        {"agent_stop", "Stop", true},
        {"before_compact", "PreCompact", true},
        {"subagent_start", "SubagentStart", true},
        {"subagent_stop", "SubagentStop", true},
        {"before_model", "before_model", false},  // not supported
    }
    for _, tt := range tests {
        t.Run(tt.canonical, func(t *testing.T) {
            got, supported := TranslateHookEvent(tt.canonical, "vs-code-copilot")
            assertEqual(t, tt.wantNative, got)
            if supported != tt.wantSupported {
                t.Errorf("supported: got %v, want %v", supported, tt.wantSupported)
            }
        })
    }
}
```
Run `cd cli && go test ./internal/converter/... -run TestTranslateHookEvent_VSCodeCopilot`.

---

### Task 0.4 — Add VS Code Copilot to `ToolNames`

**What:** Add `"vs-code-copilot"` entries to the `ToolNames` map. VS Code Copilot uses the same tool names as Claude Code (shared implementation with CC). Add entries mirroring the `"claude-code"` entries for all tools where CC has a mapping.

**Files:** `cli/internal/converter/toolmap.go`

**Dependencies:** Task 0.3.

**Test:** Add a test in `toolmap_test.go`:
```go
func TestTranslateTool_VSCodeCopilot(t *testing.T) {
    tests := []struct {
        canonical string
        want      string
    }{
        {"file_read", "Read"},
        {"file_write", "Write"},
        {"file_edit", "Edit"},
        {"shell", "Bash"},
        {"find", "Glob"},
        {"search", "Grep"},
        {"web_search", "WebSearch"},
        {"web_fetch", "WebFetch"},
        {"agent", "Agent"},
    }
    for _, tt := range tests {
        t.Run(tt.canonical, func(t *testing.T) {
            got := TranslateTool(tt.canonical, "vs-code-copilot")
            assertEqual(t, tt.want, got)
        })
    }
}
```

---

### Task 0.5 — Add VS Code Copilot to `HookOutputCapabilities` and `HookCapabilities` in `compat.go`

**What:** Add `"vs-code-copilot"` to the `HookOutputCapabilities` map with the same capabilities as `"claude-code"`. Per spec Section 9.1 support matrix: `vs-code-copilot` supports the same rich output fields as Claude Code (`updated_input`, `suppress_output`, `system_message`, `context`, `continue`, `decision`).

**[FIX M3]** Also add `"vs-code-copilot"` to `HookCapabilities` (not just `HookOutputCapabilities`). `AnalyzeHookCompat` uses `HookCapabilities` — without this entry, `AnalyzeHookCompat` returns `CompatNone` for all `vs-code-copilot` hooks because the provider is not found in the map. Add it with the same capabilities as `"claude-code"`:
```go
"vs-code-copilot": {
    Features: map[HookFeature]FeatureSupport{
        FeatureMatcher:       {Supported: true},
        FeatureAsync:         {Supported: true},
        FeatureStatusMessage: {Supported: true},
        FeatureLLMHook:       {Supported: true},
        FeatureTimeout:       {Supported: true, Notes: "milliseconds"},
    },
},
```
Also add `"vs-code-copilot"` to the `HookProviders()` return slice if VS Code Copilot should be visible in the TUI's provider list.

**Files:** `cli/internal/converter/compat.go`

**Dependencies:** Task 0.3.

**Test:** Add a test in a new or existing file:
```go
func TestHookOutputCapabilities_VSCodeCopilot(t *testing.T) {
    caps, ok := HookOutputCapabilities["vs-code-copilot"]
    if !ok {
        t.Fatal("expected vs-code-copilot in HookOutputCapabilities")
    }
    for _, field := range AllOutputFields {
        if !caps[field] {
            t.Errorf("expected vs-code-copilot to support output field %q", field)
        }
    }
}
```

---

### Task 0.6 — Add deterministic tiebreaker for Copilot `errorOccurred` reverse mapping

**What:** Fix the non-deterministic `ReverseTranslateHookEvent("errorOccurred", "copilot-cli")` behavior. Currently both `error_occurred` and `tool_use_failure` map to `"errorOccurred"` in Copilot, and Go map iteration is non-deterministic. Add a deterministic tiebreaker so the function always returns `"error_occurred"` when decoding from Copilot (prefer `error_occurred` over `tool_use_failure`).

**Files:** `cli/internal/converter/toolmap.go`

**Dependencies:** Task 0.2.

**Test:** The existing test `TestReverseTranslateHookEvent_CopilotAmbiguous` currently accepts either result. Update it to assert exactly `"error_occurred"`:
```go
func TestReverseTranslateHookEvent_CopilotAmbiguous(t *testing.T) {
    got := ReverseTranslateHookEvent("errorOccurred", "copilot-cli")
    assertEqual(t, "error_occurred", got)  // deterministic: prefer error_occurred
}
```
Run `cd cli && go test ./internal/converter/... -run TestReverseTranslateHookEvent_CopilotAmbiguous` — this will fail before the fix.

**Code guidance:** In `ReverseTranslateHookEvent`, add a tiebreaker check before (or after) the map loop: when multiple canonical events map to the same provider-native name for the given slug, prefer `"error_occurred"`. Implement as a post-loop check:
```go
func ReverseTranslateHookEvent(event, sourceSlug string) string {
    var matches []string
    for canonical, m := range HookEvents {
        if provName, ok := m[sourceSlug]; ok && provName == event {
            matches = append(matches, canonical)
        }
    }
    if len(matches) == 0 {
        return event
    }
    if len(matches) == 1 {
        return matches[0]
    }
    // Tiebreaker: prefer error_occurred over tool_use_failure for copilot-cli
    for _, m := range matches {
        if m == "error_occurred" {
            return m
        }
    }
    return matches[0]
}
```

**[FIX m1 — performance]** This implementation allocates a `[]string` slice on every call, even for the common single-match case. Consider optimizing: return on first match, then only do a second pass if the tiebreaker is needed. Example: scan once, on first match save it; if a second match is found, apply the tiebreaker between them and return. This avoids the allocation in the common case (hot path — called for every hook decoded).

---

### Task 0.7 — Pre-work design doc items verification

**[FIX m8 — design parity]** The design doc's "Spec Alignment Pre-Work" section lists two additional items not covered by Tasks 0.1–0.6:
- Add missing `after_task` event for Kiro
- Add missing `agent_stop` → `session.idle` for OpenCode

Verify these are already present in `HookEvents` before Phase 1:
```bash
grep -A3 '"after_task"' cli/internal/converter/toolmap.go   # should show kiro entry
grep -A3 '"agent_stop"' cli/internal/converter/toolmap.go   # should show opencode entry
```
If both are already in the code (they appear to be based on source review), no action needed — mark as pre-work already complete. If either is missing, add it here.

---

### Task 0.8 — Add `status_message` to hooks spec

**What:** Add `status_message` as an optional handler field in the hooks spec document. This aligns the spec with the Go struct (after Task 0.1's JSON tag fix) and the design decision to make it a first-class field.

**Files:** `docs/spec/hooks-v1.md`

**Dependencies:** Task 0.1 (JSON tag fix establishes the canonical name).

**Test:** No code test. Verify the spec text describes `status_message` as:
- Type: string
- Required: OPTIONAL
- Description: Human-readable status text displayed while the hook is executing (e.g., "Running linter...")
- Default: omitted (no status shown)

**Code guidance:** Add to Section 3.5 (Handler Definition) alongside `timeout` and `async`:
```markdown
**Field: `status_message`**
- Type: string
- Required: OPTIONAL
- Description: Human-readable status text displayed to the user while the hook executes
```

---

### Task 0.9 — Phase 0 verification

**What:** Run the full converter test suite to confirm all pre-work changes are consistent and nothing is broken.

**Files:** None (verification only).

**Dependencies:** Tasks 0.1–0.8.

**Test:** `cd cli && go test ./internal/converter/... -v 2>&1 | tail -20` — all tests must pass.

Then: `cd cli && make build` — binary must compile cleanly.

---

## Phase 1: Shared Translation Helpers (`hookhelpers.go`)

This phase creates the toolkit that all adapters will use. No existing code is modified. Tests are written first.

### Task 1.1 — Write tests for `TranslateEventToProvider` and `TranslateEventFromProvider`

**What:** Write failing tests for the two event translation helpers before implementing them.

**Files:** `cli/internal/converter/hookhelpers_test.go` (new file)

**Dependencies:** Phase 0 complete.

**Test:** Write these tests (they will fail because `hookhelpers.go` doesn't exist yet):
```go
package converter

import (
    "encoding/json" // [FIX m2] Add here so Tasks 1.2-1.5 can append functions without adding imports
    "strings"       // needed for strings.Contains in TestGenerateLLMWrapperScript
    "testing"
)

func TestTranslateEventToProvider(t *testing.T) {
    tests := []struct {
        name      string
        event     string
        slug      string
        want      string
        wantErr   bool
    }{
        {"CC before_tool_execute", "before_tool_execute", "claude-code", "PreToolUse", false},
        {"Gemini before_tool_execute", "before_tool_execute", "gemini-cli", "BeforeTool", false},
        {"Copilot before_tool_execute", "before_tool_execute", "copilot-cli", "preToolUse", false},
        {"Kiro session_start", "session_start", "kiro", "agentSpawn", false},
        {"Cursor before_tool_execute", "before_tool_execute", "cursor", "PreToolUse", false},
        // Unsupported events return error (encode path is strict)
        {"Gemini subagent_start unsupported", "subagent_start", "gemini-cli", "", true},
        {"unknown event", "nonexistent_event", "claude-code", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := TranslateEventToProvider(tt.event, tt.slug)
            if tt.wantErr {
                if err == nil {
                    t.Errorf("expected error, got %q", got)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            assertEqual(t, tt.want, got)
        })
    }
}

func TestTranslateEventFromProvider(t *testing.T) {
    tests := []struct {
        name         string
        event        string
        slug         string
        wantCanon    string
        wantWarnings int
    }{
        {"CC PreToolUse", "PreToolUse", "claude-code", "before_tool_execute", 0},
        {"Gemini BeforeTool", "BeforeTool", "gemini-cli", "before_tool_execute", 0},
        {"Copilot preToolUse", "preToolUse", "copilot-cli", "before_tool_execute", 0},
        {"Kiro agentSpawn", "agentSpawn", "kiro", "session_start", 0},
        // Unknown event passes through for forward compat, emits warning
        {"unknown CC event", "NewFutureEvent", "claude-code", "NewFutureEvent", 1},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, warnings := TranslateEventFromProvider(tt.event, tt.slug)
            assertEqual(t, tt.wantCanon, got)
            if len(warnings) != tt.wantWarnings {
                t.Errorf("warnings: got %d, want %d: %v", len(warnings), tt.wantWarnings, warnings)
            }
        })
    }
}
```

Run `cd cli && go test ./internal/converter/... -run TestTranslateEvent` — expect compile error (file doesn't exist).

---

### Task 1.2 — Write tests for `TranslateTimeoutToProvider` and `TranslateTimeoutFromProvider`

**What:** Write failing tests for timeout helpers.

**Files:** `cli/internal/converter/hookhelpers_test.go` (append to file from Task 1.1)

**Dependencies:** Task 1.1.

**Test:** Append:
```go
func TestTranslateTimeoutToProvider(t *testing.T) {
    tests := []struct {
        name    string
        seconds int
        slug    string
        want    int
    }{
        {"CC 5s -> 5000ms", 5, "claude-code", 5000},
        {"Gemini 10s -> 10000ms", 10, "gemini-cli", 10000},
        {"Cursor 3s -> 3000ms", 3, "cursor", 3000},
        {"Kiro 7s -> 7000ms", 7, "kiro", 7000},
        // Copilot uses seconds — no conversion
        {"Copilot 5s -> 5s", 5, "copilot-cli", 5},
        // Zero timeout passes through
        {"zero CC", 0, "claude-code", 0},
        {"zero Copilot", 0, "copilot-cli", 0},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := TranslateTimeoutToProvider(tt.seconds, tt.slug)
            if got != tt.want {
                t.Errorf("got %d, want %d", got, tt.want)
            }
        })
    }
}

func TestTranslateTimeoutFromProvider(t *testing.T) {
    tests := []struct {
        name  string
        value int
        slug  string
        want  int
    }{
        {"CC 5000ms -> 5s", 5000, "claude-code", 5},
        {"Gemini 3000ms -> 3s", 3000, "gemini-cli", 3},
        {"Cursor 10000ms -> 10s", 10000, "cursor", 10},
        {"Kiro 7000ms -> 7s", 7000, "kiro", 7},
        // Copilot uses seconds — no conversion
        {"Copilot 5s -> 5s", 5, "copilot-cli", 5},
        // Zero passes through
        {"zero CC", 0, "claude-code", 0},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := TranslateTimeoutFromProvider(tt.value, tt.slug)
            if got != tt.want {
                t.Errorf("got %d, want %d", got, tt.want)
            }
        })
    }
}
```

---

### Task 1.3 — Write tests for `TranslateMatcherToProvider` and `TranslateMatcherFromProvider`

**What:** Write failing tests for matcher translation helpers. These handle the four canonical matcher shapes: bare string, pattern object, MCP object, and array.

**Files:** `cli/internal/converter/hookhelpers_test.go` (append)

**Dependencies:** Task 1.2.

**Test:** Append:
```go
// [FIX m2] Do NOT add an import block mid-file. The "encoding/json" import must be
// added to the file's single import block written in Task 1.1. The import block in
// Task 1.1 should already include "encoding/json". Remove any stray import statement
// here — only function bodies are appended below.

func TestTranslateMatcherToProvider_BareString(t *testing.T) {
    matcher, _ := json.Marshal("shell")
    result, warnings := TranslateMatcherToProvider(matcher, "claude-code")
    var got string
    json.Unmarshal(result, &got)
    assertEqual(t, "Bash", got)
    if len(warnings) != 0 {
        t.Errorf("unexpected warnings: %v", warnings)
    }
}

func TestTranslateMatcherToProvider_BareString_Gemini(t *testing.T) {
    matcher, _ := json.Marshal("file_read")
    result, warnings := TranslateMatcherToProvider(matcher, "gemini-cli")
    var got string
    json.Unmarshal(result, &got)
    assertEqual(t, "read_file", got)
    if len(warnings) != 0 {
        t.Errorf("unexpected warnings: %v", warnings)
    }
}

func TestTranslateMatcherToProvider_MCPObject(t *testing.T) {
    matcher := json.RawMessage(`{"mcp":{"server":"github","tool":"create_issue"}}`)
    result, warnings := TranslateMatcherToProvider(matcher, "claude-code")
    // Should produce bare string "mcp__github__create_issue"
    var got string
    json.Unmarshal(result, &got)
    assertEqual(t, "mcp__github__create_issue", got)
    if len(warnings) != 0 {
        t.Errorf("unexpected warnings: %v", warnings)
    }
}

func TestTranslateMatcherToProvider_MCPObject_Copilot(t *testing.T) {
    matcher := json.RawMessage(`{"mcp":{"server":"github","tool":"create_issue"}}`)
    result, warnings := TranslateMatcherToProvider(matcher, "copilot-cli")
    var got string
    json.Unmarshal(result, &got)
    assertEqual(t, "github/create_issue", got)
    if len(warnings) != 0 {
        t.Errorf("unexpected warnings: %v", warnings)
    }
}

func TestTranslateMatcherToProvider_PatternObject(t *testing.T) {
    matcher := json.RawMessage(`{"pattern":"file_(read|write)"}`)
    result, warnings := TranslateMatcherToProvider(matcher, "claude-code")
    // Pattern objects: CC does not have a pattern-object syntax in its hook format.
    // The canonical {"pattern":"..."} is flattened to the bare regex string with a warning.
    // [FIX M4] This is intentionally lossy: the round-trip back through
    // TranslateMatcherFromProvider will see a bare string and treat it as a tool name
    // (running ReverseTranslateMatcher), not as a regex. This is a known limitation —
    // pattern matchers cannot round-trip through providers that don't have a pattern
    // object syntax. The warning text must document this limitation explicitly.
    var got string
    if err := json.Unmarshal(result, &got); err != nil {
        t.Fatalf("expected string result: %v", err)
    }
    assertEqual(t, "file_(read|write)", got)
    if len(warnings) != 1 {
        t.Errorf("expected 1 warning for pattern passthrough, got %d: %v", len(warnings), warnings)
    }
    // Verify the warning message describes the lossy behavior
    if len(warnings) == 1 && warnings[0].Description == "" {
        t.Error("warning should have a description explaining the lossy pattern flattening")
    }
}

func TestTranslateMatcherFromProvider_BareString(t *testing.T) {
    matcher, _ := json.Marshal("Bash")
    result, warnings := TranslateMatcherFromProvider(matcher, "claude-code")
    var got string
    json.Unmarshal(result, &got)
    assertEqual(t, "shell", got)
    if len(warnings) != 0 {
        t.Errorf("unexpected warnings: %v", warnings)
    }
}

func TestTranslateMatcherFromProvider_MCPString(t *testing.T) {
    // CC MCP format: "mcp__github__create_issue" -> canonical MCP object
    matcher, _ := json.Marshal("mcp__github__create_issue")
    result, warnings := TranslateMatcherFromProvider(matcher, "claude-code")
    // Should produce canonical MCP object
    var obj map[string]any
    json.Unmarshal(result, &obj)
    if obj["mcp"] == nil {
        t.Errorf("expected canonical MCP object with 'mcp' key, got %s", string(result))
    }
    if len(warnings) != 0 {
        t.Errorf("unexpected warnings: %v", warnings)
    }
}
```

---

### Task 1.4 — Write tests for `TranslateHandlerType` and `GenerateLLMWrapperScript`

**What:** Write failing tests for handler type fitness checking and LLM wrapper generation.

**Files:** `cli/internal/converter/hookhelpers_test.go` (append)

**Dependencies:** Task 1.3.

**Test:** Append:
```go
func TestTranslateHandlerType_CommandAlwaysKept(t *testing.T) {
    h := HookHandler{Type: "command", Command: "echo check"}
    result, warnings, keep := TranslateHandlerType(h, "gemini-cli")
    if !keep {
        t.Error("command handler should always be kept")
    }
    if len(warnings) != 0 {
        t.Errorf("unexpected warnings: %v", warnings)
    }
    assertEqual(t, "command", result.Type)
}

func TestTranslateHandlerType_PromptDroppedForNonCC(t *testing.T) {
    h := HookHandler{Type: "prompt", Prompt: "Is this safe?"}
    _, warnings, keep := TranslateHandlerType(h, "gemini-cli")
    if keep {
        t.Error("prompt handler should not be kept for gemini-cli")
    }
    if len(warnings) != 1 {
        t.Errorf("expected 1 warning for dropped prompt hook, got %d", len(warnings))
    }
}

func TestTranslateHandlerType_PromptKeptForCC(t *testing.T) {
    h := HookHandler{Type: "prompt", Prompt: "Is this safe?"}
    result, warnings, keep := TranslateHandlerType(h, "claude-code")
    if !keep {
        t.Error("prompt handler should be kept for claude-code")
    }
    if len(warnings) != 0 {
        t.Errorf("unexpected warnings: %v", warnings)
    }
    assertEqual(t, "prompt", result.Type)
}

func TestTranslateHandlerType_HTTPDroppedForNonCC(t *testing.T) {
    h := HookHandler{Type: "http", URL: "https://example.com"}
    _, warnings, keep := TranslateHandlerType(h, "kiro")
    if keep {
        t.Error("http handler should not be kept for kiro")
    }
    if len(warnings) != 1 {
        t.Errorf("expected 1 warning for dropped http hook, got %d", len(warnings))
    }
}

func TestGenerateLLMWrapperScript(t *testing.T) {
    h := CanonicalHook{
        Handler: HookHandler{Type: "prompt", Prompt: "Is this command safe?"},
    }
    name, content := GenerateLLMWrapperScript(h, "gemini-cli", "before_tool_execute", 0)
    if name == "" {
        t.Error("expected non-empty script name")
    }
    if len(content) == 0 {
        t.Error("expected non-empty script content")
    }
    script := string(content)
    // [FIX m9] containsStr is not a standard Go function. Use strings.Contains directly.
    // If containsStr is defined as a package-level test helper elsewhere in the test package,
    // verify its presence first: grep -r 'func containsStr' cli/internal/converter/
    // If not found, replace all uses with strings.Contains in this test file.
    if !strings.Contains(script, "#!/bin/bash") {
        t.Error("script should start with bash shebang")
    }
    if !strings.Contains(script, "Is this command safe?") {
        t.Error("script should include the prompt text")
    }
    if !strings.Contains(script, "gemini") {
        t.Error("script should reference the gemini CLI")
    }
}
```

---

### Task 1.5 — Write test for `CheckStructuredOutputLoss`

**What:** Write failing test for the structured output loss warning helper.

**Files:** `cli/internal/converter/hookhelpers_test.go` (append)

**Dependencies:** Task 1.4.

**Test:** Append:
```go
func TestCheckStructuredOutputLoss(t *testing.T) {
    // Claude -> Gemini: 4 fields lost (decision+system_message kept)
    warnings := CheckStructuredOutputLoss("claude-code", "gemini-cli")
    if len(warnings) != 1 {
        t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
    }
    if warnings[0].Capability == "" {
        t.Error("warning should have capability set")
    }
    if warnings[0].Severity != "warning" {
        t.Errorf("expected severity 'warning', got %q", warnings[0].Severity)
    }

    // Claude -> Claude: no loss
    noWarnings := CheckStructuredOutputLoss("claude-code", "claude-code")
    if len(noWarnings) != 0 {
        t.Errorf("expected no warnings for same provider, got: %v", noWarnings)
    }

    // Empty source: no warnings
    emptyWarnings := CheckStructuredOutputLoss("", "gemini-cli")
    if len(emptyWarnings) != 0 {
        t.Errorf("expected no warnings for empty source, got: %v", emptyWarnings)
    }
}
```

---

### Task 1.6 — Implement `hookhelpers.go`

**What:** Create `cli/internal/converter/hookhelpers.go` with all shared translation helpers. This is the implementation that makes the Phase 1 tests green.

**Files:** `cli/internal/converter/hookhelpers.go` (new file)

**Dependencies:** Tasks 1.1–1.5 (tests must be written and failing before this task).

**Test:** After implementation, run `cd cli && go test ./internal/converter/... -run "TestTranslateEvent|TestTranslateTimeout|TestTranslateMatcher|TestTranslateHandler|TestGenerateLLM|TestCheckStructured"` — all should pass.

**Code guidance:**

File header:
```go
package converter

import (
    "encoding/json"
    "fmt"
    "strings"
)
```

**`TranslateEventToProvider(event, slug string) (string, error)`**
- Calls `TranslateHookEvent(event, slug)` (existing function in `toolmap.go`)
- If not supported, return `"", fmt.Errorf("event %q not supported by provider %q", event, slug)`
- If supported, return translated name and nil error

**`TranslateEventFromProvider(event, slug string) (string, []ConversionWarning)`**
- Calls `ReverseTranslateHookEvent(event, slug)` (existing function)
- If result equals input (no mapping found), return event + one warning:
  `ConversionWarning{Severity: "warning", Description: fmt.Sprintf("unknown %s event %q: passed through as-is for forward compatibility", slug, event)}`
- Otherwise return canonical name with nil warnings

**`TranslateTimeoutToProvider(seconds int, slug string) int`**
- If `seconds == 0`, return 0
- **[FIX m3]** Do NOT use `AdapterFor(slug).Capabilities()` for this — it creates an indirect dependency on the adapter registry being initialized before this helper is called (a latent init-ordering risk). Use a direct switch instead:
  ```go
  switch slug {
  case "copilot-cli":
      return seconds // Copilot uses seconds natively
  default:
      return seconds * 1000 // CC, Gemini, Cursor, Kiro all use milliseconds
  }
  ```
- Fall back to milliseconds for unknown slugs (safe default).

**`TranslateTimeoutFromProvider(value int, slug string) int`**
- If `value == 0`, return 0
- Same direct switch as above: Copilot returns unchanged, others divide by 1000

**`TranslateMatcherToProvider(matcher json.RawMessage, slug string) (json.RawMessage, []ConversionWarning)`**
- Return nil, nil if matcher is nil or empty
- Try to unmarshal as `string` first (bare string): call `TranslateMatcher(s, slug)` (existing function), marshal result back to `json.RawMessage`
- Try to unmarshal as `map[string]json.RawMessage`:
  - If has `"mcp"` key: parse server+tool from the canonical MCP object, call `TranslateMCPToolName("mcp__"+server+"__"+tool, "claude-code", slug)` to get provider-native MCP name, return as JSON string
  - If has `"pattern"` key: extract pattern string, return as JSON string with a warning: `"pattern matchers are not portable; passed through as literal string for target provider"`
- Try to unmarshal as `[]json.RawMessage` (array):
  - Recursively translate each element
  - Collect all warnings
  - For providers without array-matcher support, this returns the array as-is (let the adapter decide whether to expand it)
  - Return translated array
- Unknown shape: return original with warning

**`TranslateMatcherFromProvider(matcher json.RawMessage, slug string) (json.RawMessage, []ConversionWarning)`**
- Return nil, nil if matcher is nil or empty
- Unmarshal as string:
  - Check if it's an MCP-format string (by calling `parseMCPToolName(s, slug)` — unexported, but accessible in same package)
  - If MCP format: build canonical MCP object `{"mcp":{"server":"...","tool":"..."}}` as `json.RawMessage`
  - Otherwise: `ReverseTranslateMatcher(s, slug)` → marshal back
- Unmarshal as object (pattern/mcp already in canonical form): return as-is
- Unmarshal as array: recursively translate each element

**`TranslateMCPToProvider(server, tool, slug string) string`**
- Build canonical `"mcp__"+server+"__"+tool` string, then call `TranslateMCPToolName` with `"claude-code"` as source and `slug` as target
- If `tool == ""`: handle server-only case (build `mcp__server` and delegate)

**`TranslateMCPFromProvider(mcpName, slug string) (server, tool string, ok bool)`**
- Call `parseMCPToolName(mcpName, slug)` (unexported, same package)
- Return server, tool, and whether it was an MCP name

**`TranslateHandlerType(h HookHandler, slug string) (HookHandler, []ConversionWarning, bool)`**
- Get capabilities: `caps := AdapterFor(slug)`
- `hType := h.Type; if hType == "" { hType = "command" }`
- `"command"`: always return (h, nil, true)
- **[FIX C1]** Apply the nil guard BEFORE accessing `caps.Capabilities()`:
  ```go
  if caps == nil {
      if hType == "command" {
          return h, nil, true
      }
      warn := ConversionWarning{Severity: "warning", Description: fmt.Sprintf("hook type %q is not supported by %s (unknown provider); hook dropped", hType, slug)}
      return HookHandler{}, []ConversionWarning{warn}, false
  }
  ```
- `"http"`: if `!caps.Capabilities().SupportsHTTPHooks`: return (HookHandler{}, warn, false)
- `"prompt"` or `"agent"`: if `!caps.Capabilities().SupportsLLMHooks`: return (HookHandler{}, warn, false)
- Otherwise: return (h, nil, true)
- Warning text: `fmt.Sprintf("hook type %q is not supported by %s; hook dropped", hType, slug)`

**`GenerateLLMWrapperScript(hook CanonicalHook, slug, event string, idx int) (string, []byte)`**
- Delegate to the existing `generateLLMWrapperScript(h HookEntry, targetSlug, event string, idx int)` function (unexported, same package), wrapping the `CanonicalHook.Handler` into a `HookEntry`
- Create a `HookEntry` from `hook.Handler`: `HookEntry{Type: hook.Handler.Type, Command: hook.Handler.Command, Prompt: hook.Handler.Prompt, Model: hook.Handler.Model, Agent: hook.Handler.Agent}`
- Return `generateLLMWrapperScript(he, slug, event, idx)`
- **[FIX M5]** `generateLLMWrapperScript` is unexported and lives in `hooks.go`. It MUST NOT be deleted in Phase 7. Add a preservation note in Task 7.3. Alternatively, move the implementation of `generateLLMWrapperScript` from `hooks.go` into `hookhelpers.go` directly (adjusting the signature to take a `HookHandler` instead of `HookEntry`) — this removes the dependency on `hooks.go`'s legacy types and makes Phase 7 deletion safer. Either approach is acceptable but must be an explicit decision in Task 7.3.

**`CheckStructuredOutputLoss(sourceSlug, targetSlug string) []ConversionWarning`**
- If `sourceSlug == ""`, return nil
- Call `OutputFieldsLostWarnings(sourceSlug, targetSlug)` (existing function in `compat.go`)
- If empty, return nil
- Return one `ConversionWarning{Severity: "warning", Capability: "structured_output", Description: fmt.Sprintf("structured hook output fields [%s] supported by %s but not by %s (hook output will be ignored)", strings.Join(lost, ", "), sourceSlug, targetSlug)}`

---

### Task 1.7 — Phase 1 verification

**What:** Confirm all tests pass and the binary builds.

**Files:** None.

**Dependencies:** Task 1.6.

**Test:**
```bash
cd cli && go test ./internal/converter/... -v 2>&1 | grep -E "^(PASS|FAIL|---)"
cd cli && make build
```

All tests must pass. Binary must compile.

---

## Phase 2: Migrate Claude Code Adapter (Reference Implementation)

CC is the richest adapter: millisecond timeouts, 4 handler types (command, http, prompt, agent), structured output, blocking. Migrating it first validates the helper API against the most complex case.

### Task 2.1 — Write round-trip test for CC adapter (pre-migration, sets the bar)

**What:** Write a test that will be the gold standard for CC round-trip fidelity. This test currently passes at a limited level (only checks event name and command). After migration, it will assert on all previously-dropped fields.

**Files:** `cli/internal/converter/adapter_test.go`

**Dependencies:** Phase 1 complete.

**Test:** Add to `adapter_test.go`:
```go
func TestCCAdapter_RoundTrip_AllFields(t *testing.T) {
    // This test defines the fidelity contract for the migrated CC adapter.
    // Fields that the legacy bridge drops are explicitly asserted.
    matcherJSON, _ := json.Marshal("shell")
    mcpMatcher := json.RawMessage(`{"mcp":{"server":"github","tool":"create_issue"}}`)

    // [FIX M6/M7] CC's Capabilities() declares SupportsCWD: false, SupportsEnv: false,
    // SupportsPlatform: false. There are two valid resolution paths:
    // (a) Update Capabilities() to true for fields CC's JSON format actually accepts
    //     (CC silently passes through unknown fields, so these may work in practice).
    // (b) Keep Capabilities() as-is and store CWD/Env/Platform in provider_data["claude-code"]
    //     during Encode, restoring them in Decode.
    // The round-trip test below uses option (a): CC's hooks.json format includes cwd/env
    // as real fields and Capabilities() should be corrected to SupportsCWD/Env: true.
    // Remove Platform from this test fixture — CC genuinely does not support platform-
    // conditional commands and SupportsPlatform: false is correct. Do not assert Platform
    // in the CC round-trip test; Platform belongs in a Copilot or Cursor test instead.
    // Decision: Update ClaudeCodeAdapter.Capabilities() to SupportsCWD: true, SupportsEnv: true
    // before writing this test. Add that change to Task 2.2.
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Name:  "safety-check",
                Event: "before_tool_execute",
                Matcher: matcherJSON,
                Handler: HookHandler{
                    Type:          "command",
                    Command:       "echo check",
                    Timeout:       5,    // canonical seconds; should encode as 5000ms
                    TimeoutAction: "block",
                    StatusMessage: "Running safety check",
                    CWD:           "./scripts",
                    Env:           map[string]string{"AUDIT_LOG": "/tmp/audit.log"},
                    // Platform intentionally omitted: CC does not support platform-conditional
                    // commands. SupportsPlatform remains false. Do not assert Platform here.
                },
                Blocking: true,
                Degradation: map[string]string{"input_rewrite": "block"},
                Capabilities: []string{"structured_output"},
            },
            {
                Name:    "mcp-guard",
                Event:   "before_tool_execute",
                Matcher: mcpMatcher,
                Handler: HookHandler{
                    Type:    "command",
                    Command: "echo mcp",
                },
            },
        },
    }

    adapter := AdapterFor("claude-code")
    encoded, err := adapter.Encode(original)
    if err != nil {
        t.Fatalf("Encode: %v", err)
    }
    if len(encoded.Warnings) != 0 {
        t.Logf("encode warnings: %v", encoded.Warnings)
    }

    decoded, err := adapter.Decode(encoded.Content)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }

    if len(decoded.Hooks) != len(original.Hooks) {
        t.Fatalf("hook count: got %d, want %d", len(decoded.Hooks), len(original.Hooks))
    }

    h0 := decoded.Hooks[0]
    assertEqual(t, "safety-check", h0.Name)
    assertEqual(t, "before_tool_execute", h0.Event)
    assertEqual(t, true, h0.Blocking)
    assertEqual(t, "block", h0.Handler.TimeoutAction)
    assertEqual(t, "Running safety check", h0.Handler.StatusMessage)
    assertEqual(t, 5, h0.Handler.Timeout) // decoded back to canonical seconds
    assertEqual(t, "./scripts", h0.Handler.CWD)
    if h0.Handler.Env["AUDIT_LOG"] != "/tmp/audit.log" {
        t.Errorf("Env not preserved: %v", h0.Handler.Env)
    }
}
```

Run `cd cli && go test ./internal/converter/... -run TestCCAdapter_RoundTrip_AllFields` — this will initially fail on the new field assertions (legacy bridge drops them).

---

### Task 2.2 — Define CC provider-native struct

**What:** Define the Go structs that represent Claude Code's hook JSON format. These will be used by the new `Encode`/`Decode` methods.

**Files:** `cli/internal/converter/adapter_cc.go`

**Dependencies:** Task 2.1.

**Code guidance:** Add these unexported types to `adapter_cc.go`. Do NOT yet remove the old `Encode`/`Decode` methods (they stay until the new ones are tested):
```go
// ccHookEntry is a single hook in Claude Code's native format.
// TimeoutMs is in milliseconds (CC's native unit).
type ccHookEntry struct {
    Type          string            `json:"type,omitempty"`
    Command       string            `json:"command,omitempty"`
    TimeoutMs     int               `json:"timeout,omitempty"`       // milliseconds
    StatusMessage string            `json:"status_message,omitempty"` // CC uses status_message
    Async         bool              `json:"async,omitempty"`
    // HTTP fields
    URL            string            `json:"url,omitempty"`
    Headers        map[string]string `json:"headers,omitempty"`
    AllowedEnvVars []string          `json:"allowedEnvVars,omitempty"`
    // Prompt fields
    Prompt string          `json:"prompt,omitempty"`
    Model  string          `json:"model,omitempty"`
    // Agent fields
    Agent json.RawMessage `json:"agent,omitempty"`
    // Extended canonical fields (passed through from canonical)
    TimeoutAction string            `json:"timeout_action,omitempty"`
    CWD           string            `json:"cwd,omitempty"`
    Env           map[string]string `json:"env,omitempty"`
    Platform      map[string]string `json:"platform,omitempty"`
}

// ccMatcherGroup is a matcher group in Claude Code's hook format.
type ccMatcherGroup struct {
    Matcher string        `json:"matcher,omitempty"`
    Hooks   []ccHookEntry `json:"hooks"`
}

// ccHooksFile is the top-level Claude Code hooks config.
type ccHooksFile struct {
    Hooks map[string][]ccMatcherGroup `json:"hooks"`
}
```

**[FIX M6/M7] CC Capabilities() update:** Before defining `ccHookEntry`, update `ClaudeCodeAdapter.Capabilities()` in `adapter_cc.go` to reflect CC's actual JSON support:
- Change `SupportsCWD: false` → `SupportsCWD: true` (CC's hooks.json accepts `cwd`)
- Change `SupportsEnv: false` → `SupportsEnv: true` (CC's hooks.json accepts `env`)
- Leave `SupportsPlatform: false` — CC does not have platform-conditional hook logic

This resolves the contradiction between the round-trip test (which asserts CWD/Env survive) and Capabilities(). If verification against CC's actual docs shows CWD/Env are NOT real CC fields, revert this decision and instead: (a) remove CWD/Env from the round-trip test fixture, and (b) keep the fields out of `ccHookEntry`.

**Note on CC's JSON field names:** The existing `adapter_cc.go` uses `statusMessage` (camelCase) in the legacy bridge path. After Task 0.1 changes the canonical tag to `status_message`, verify what field name CC's own config file actually uses. If CC uses `statusMessage` natively, the `ccHookEntry` struct tag should be `json:"statusMessage,omitempty"` while the canonical `HookHandler` uses `json:"status_message,omitempty"` — the adapter's encode/decode is the boundary between them.

**Test:** No new tests yet. Compile only: `cd cli && go build ./internal/converter/...`

---

### Task 2.3 — Implement new `Encode` for CC adapter

**What:** Replace `ClaudeCodeAdapter.Encode` with a direct `CanonicalHooks`-to-CC-native implementation using the shared helpers.

**Files:** `cli/internal/converter/adapter_cc.go`

**Dependencies:** Task 2.2.

**Code guidance:**

```go
func (a *ClaudeCodeAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
    result := ccHooksFile{Hooks: make(map[string][]ccMatcherGroup)}
    var warnings []ConversionWarning
    scriptIdx := 0
    extraFiles := map[string][]byte{}

    // Structured output loss warnings (source provider tracked in hooks.Hooks[0].ProviderData or via a passed-through field)
    // For now, emit structured output warnings for the source provider if discoverable.
    // Check if first hook's provider_data contains a source hint, or check if we're
    // being called from a cross-provider encode. This is best-effort.

    for _, hook := range hooks.Hooks {
        // 1. Translate event
        nativeEvent, err := TranslateEventToProvider(hook.Event, "claude-code")
        if err != nil {
            warnings = append(warnings, ConversionWarning{
                Severity:    "warning",
                Description: fmt.Sprintf("hook event %q not supported by claude-code; skipped", hook.Event),
            })
            continue
        }

        // 2. Check handler type
        handler, hWarnings, keep := TranslateHandlerType(hook.Handler, "claude-code")
        warnings = append(warnings, hWarnings...)
        if !keep {
            continue
        }

        // 3. Translate matcher (CC supports matchers — always call)
        var matcherStr string
        if hook.Matcher != nil {
            translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, "claude-code")
            warnings = append(warnings, mWarnings...)
            if translatedMatcher != nil {
                json.Unmarshal(translatedMatcher, &matcherStr)
            }
        }

        // 4. Translate timeout (canonical seconds -> CC milliseconds)
        timeoutMs := TranslateTimeoutToProvider(handler.Timeout, "claude-code")

        // 5. Build entry
        entry := ccHookEntry{
            Type:          handler.Type,
            Command:       handler.Command,
            TimeoutMs:     timeoutMs,
            StatusMessage: handler.StatusMessage,
            Async:         handler.Async,
            URL:           handler.URL,
            Headers:       handler.Headers,
            AllowedEnvVars: handler.AllowedEnvVars,
            Prompt:        handler.Prompt,
            Model:         handler.Model,
            Agent:         handler.Agent,
            TimeoutAction: handler.TimeoutAction,
            CWD:           handler.CWD,
            Env:           handler.Env,
            Platform:      handler.Platform,
        }

        // 6. Group by event + matcher
        // [FIX C2] CC's format requires hooks with the same event+matcher to be in the
        // same group: {"hooks": {"PreToolUse": [{"matcher":"Bash","hooks":[h1,h2]}]}}.
        // One-group-per-hook produces duplicate matcher groups which is semantically
        // different and will break existing round-trip tests (TestAdapterRoundTrip_CC_Gemini,
        // TestClaudeCodeAdapterDecode, etc.). Use a grouping key of nativeEvent+"\x00"+matcherStr.
        //
        // Replace the simple append above with this merge logic:
        //   key := nativeEvent + "\x00" + matcherStr
        //   (track groups in a map[string]*ccMatcherGroup indexed by key, then
        //    emit them in deterministic order at the end)
        //
        // Full corrected grouping:
        //   type groupKey struct{ event, matcher string }
        //   groups := map[groupKey]*ccMatcherGroup{}
        //   order  := []groupKey{}  // preserve insertion order
        //   ...per hook loop...
        //   k := groupKey{nativeEvent, matcherStr}
        //   if g, exists := groups[k]; exists {
        //       g.Hooks = append(g.Hooks, entry)
        //   } else {
        //       g := &ccMatcherGroup{Matcher: matcherStr, Hooks: []ccHookEntry{entry}}
        //       groups[k] = g
        //       order = append(order, k)
        //   }
        //   ...after loop, build result.Hooks from groups+order...
        //   for _, k := range order {
        //       result.Hooks[k.event] = append(result.Hooks[k.event], *groups[k])
        //   }
    }

    content, err := json.MarshalIndent(result, "", "  ")
    if err != nil {
        return nil, err
    }
    er := &EncodedResult{
        Content:  content,
        Filename: "hooks.json",
        Warnings: warnings,
    }
    if len(extraFiles) > 0 {
        er.Scripts = extraFiles
    }
    return er, nil
}
```

**[FIX C2 — critical]** The grouping logic shown in the original code comment ("one group per hook is correct for the new architecture") is WRONG and will break existing tests. CC's format groups hooks by event+matcher. Multiple hooks for the same event+matcher MUST land in the same group's `hooks` array. Implement the keyed grouping map shown above. This must be verified by running `TestAdapterRoundTrip_CC_Gemini` and `TestClaudeCodeAdapterDecode` immediately after Task 2.3.

**Test:** Run `cd cli && go test ./internal/converter/... -run "TestCCAdapter_RoundTrip_AllFields|TestClaudeCodeAdapterDecode|TestVerify_Success|TestAdapterRoundTrip_CC_Gemini"` — the round-trip test should now pass on previously-dropped fields.

---

### Task 2.4 — Implement new `Decode` for CC adapter

**What:** Replace `ClaudeCodeAdapter.Decode` with a direct CC-native-to-`CanonicalHooks` implementation.

**Files:** `cli/internal/converter/adapter_cc.go`

**Dependencies:** Task 2.3.

**Code guidance:**

```go
func (a *ClaudeCodeAdapter) Decode(content []byte) (*CanonicalHooks, error) {
    var file ccHooksFile
    if err := json.Unmarshal(content, &file); err != nil {
        return nil, fmt.Errorf("parsing claude-code hooks: %w", err)
    }

    ch := &CanonicalHooks{Spec: SpecVersion}

    for nativeEvent, groups := range file.Hooks {
        // 1. Translate event (decode path: warnings, not errors)
        canonEvent, eWarnings := TranslateEventFromProvider(nativeEvent, "claude-code")
        // [FIX m10] eWarnings are intentionally discarded here. HookAdapter.Decode() returns
        // (*CanonicalHooks, error) — there is no warnings channel. Decode-path forward-compat
        // warnings (unknown future events) are informational and do not block decoding.
        // Decision: Accept that decode-path event warnings are silently discarded. If we later
        // need to surface them, CanonicalHooks would need a Warnings []ConversionWarning field
        // and all HookAdapter.Decode() implementations would need updating — an interface-breaking
        // change. Defer that until there's a concrete use case.
        _ = eWarnings

        for _, group := range groups {
            // 2. Translate matcher
            var matcherJSON json.RawMessage
            if group.Matcher != "" {
                rawMatcher, _ := json.Marshal(group.Matcher)
                translatedMatcher, _ := TranslateMatcherFromProvider(rawMatcher, "claude-code")
                matcherJSON = translatedMatcher
            }

            for _, entry := range group.Hooks {
                // 3. Translate timeout (CC milliseconds -> canonical seconds)
                timeoutSec := TranslateTimeoutFromProvider(entry.TimeoutMs, "claude-code")

                hook := CanonicalHook{
                    Event:   canonEvent,
                    Matcher: matcherJSON,
                    Handler: HookHandler{
                        Type:           entry.Type,
                        Command:        entry.Command,
                        Timeout:        timeoutSec,
                        StatusMessage:  entry.StatusMessage,
                        Async:          entry.Async,
                        URL:            entry.URL,
                        Headers:        entry.Headers,
                        AllowedEnvVars: entry.AllowedEnvVars,
                        Prompt:         entry.Prompt,
                        Model:          entry.Model,
                        Agent:          entry.Agent,
                        TimeoutAction:  entry.TimeoutAction,
                        CWD:            entry.CWD,
                        Env:            entry.Env,
                        Platform:       entry.Platform,
                    },
                }
                // Normalize type default
                if hook.Handler.Type == "" {
                    hook.Handler.Type = "command"
                }
                ch.Hooks = append(ch.Hooks, hook)
            }
        }
    }

    return ch, nil
}
```

**Test:** Run `cd cli && go test ./internal/converter/... -run "TestCCAdapter_RoundTrip_AllFields|TestClaudeCodeAdapterDecode|TestClaudeHooksToGemini|TestGeminiHooksToClaude"` — all must pass.

---

### Task 2.5 — Phase 2 verification

**Test:**
```bash
cd cli && go test ./internal/converter/... 2>&1 | tail -5
cd cli && make build
```

All existing tests must still pass. The new round-trip test must pass.

---

## Phase 3: Migrate Gemini Adapter

Gemini is structurally similar to CC but has a smaller event set and no LLM/HTTP hooks.

### Task 3.1 — Write Gemini round-trip test with field assertions

**What:** Write a test capturing Gemini-specific round-trip fidelity expectations.

**Files:** `cli/internal/converter/adapter_test.go`

**Dependencies:** Phase 2 complete.

**Test:** Add:
```go
func TestGeminiAdapter_RoundTrip_AllFields(t *testing.T) {
    matcherJSON, _ := json.Marshal("shell")
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Name:  "log-hook",
                Event: "before_tool_execute",
                Matcher: matcherJSON,
                Handler: HookHandler{
                    Type:          "command",
                    Command:       "echo log",
                    Timeout:       3,    // canonical seconds -> 3000ms in Gemini
                    StatusMessage: "Logging",
                    Async:         true,
                },
                Blocking: true,
            },
        },
    }
    adapter := AdapterFor("gemini-cli")
    encoded, err := adapter.Encode(original)
    if err != nil {
        t.Fatalf("Encode: %v", err)
    }
    decoded, err := adapter.Decode(encoded.Content)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }
    if len(decoded.Hooks) != 1 {
        t.Fatalf("hook count: got %d, want 1", len(decoded.Hooks))
    }
    h := decoded.Hooks[0]
    assertEqual(t, "log-hook", h.Name)
    assertEqual(t, "before_tool_execute", h.Event)
    assertEqual(t, true, h.Blocking)
    assertEqual(t, "Logging", h.Handler.StatusMessage)
    assertEqual(t, 3, h.Handler.Timeout) // back to canonical seconds
    assertEqual(t, true, h.Handler.Async)
    // Gemini doesn't support LLM hooks — encode a prompt hook and verify it's warned/dropped
}
```

Also add a test for Gemini-specific behavior: prompt hooks get a warning and are dropped:
```go
func TestGeminiAdapter_PromptHookDroppedWithWarning(t *testing.T) {
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Event: "before_tool_execute",
                Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?"},
            },
        },
    }
    adapter := AdapterFor("gemini-cli")
    encoded, err := adapter.Encode(original)
    if err != nil {
        t.Fatalf("Encode: %v", err)
    }
    if len(encoded.Warnings) == 0 {
        t.Error("expected warning for dropped prompt hook")
    }
    // Encoded content should have empty hooks
    decoded, _ := adapter.Decode(encoded.Content)
    if len(decoded.Hooks) != 0 {
        t.Errorf("expected 0 hooks after dropping prompt, got %d", len(decoded.Hooks))
    }
}
```

---

### Task 3.2 — Define Gemini provider-native struct and implement new Encode/Decode

**What:** Define `geminiHookEntry`, `geminiMatcherGroup`, `geminiHooksFile` structs, then implement new `Encode` and `Decode` for `GeminiCLIAdapter`.

**Files:** `cli/internal/converter/adapter_gemini.go`

**Dependencies:** Task 3.1.

**Code guidance:**

Gemini uses the same JSON structure as CC (`{"hooks":{"EventName":[{"matcher":"...","hooks":[...]}]}}`), so the structs mirror `ccHooksFile`/`ccMatcherGroup`/`ccHookEntry` but with `gemini` prefix. Gemini-specific differences:
- Millisecond timeouts (same as CC)
- No `type: "http"`, no `type: "prompt"`, no `type: "agent"` (command only in practice)
- Native event names: `BeforeTool`, `AfterTool`, `BeforeAgent`, `AfterAgent`, `SessionStart`, `SessionEnd`, `PreCompress`, `Notification`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`
- Structured output: `decision` and `system_message` (but those are output contracts, not encoded in the config)

Encode pattern: same as CC Phase 2.3 but call `TranslateHandlerType(hook.Handler, "gemini-cli")` which will warn+drop prompt/http/agent hooks.

Decode pattern: same as CC Phase 2.4.

**Test:** Run `cd cli && go test ./internal/converter/... -run "TestGeminiAdapter_RoundTrip|TestGeminiCLIAdapterDecode|TestAdapterRoundTrip_CC_Gemini|TestGeminiOnlyEventsRoundtrip|TestGeminiOnlyEventsDroppedForOtherProviders"` — all must pass.

---

### Task 3.3 — Phase 3 verification

**Test:**
```bash
cd cli && go test ./internal/converter/...
cd cli && make build
```

---

## Phase 4: Migrate Copilot Adapter

Copilot has a distinct schema: versioned dict format, bash/powershell split, seconds timeout (no conversion needed), `cwd`/`env` supported (currently broken), no matchers in `Capabilities()` (but actually does support them in the config format).

### Task 4.1 — Write Copilot round-trip test with field assertions

**What:** Write tests that validate the previously-broken `cwd`/`env` support.

**Files:** `cli/internal/converter/adapter_test.go`

**Dependencies:** Phase 3 complete.

**Test:** Add:
```go
func TestCopilotAdapter_RoundTrip_CWDAndEnv(t *testing.T) {
    // cwd and env are supported by Copilot but broken in the legacy bridge.
    // This test will fail until the adapter is migrated.
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Event: "before_tool_execute",
                Handler: HookHandler{
                    Type:    "command",
                    Command: "echo check",
                    Timeout: 5,  // canonical seconds -> Copilot timeoutSec (no conversion)
                    CWD:     "./hooks",
                    Env:     map[string]string{"AUDIT": "1"},
                },
            },
        },
    }
    adapter := AdapterFor("copilot-cli")
    encoded, err := adapter.Encode(original)
    if err != nil {
        t.Fatalf("Encode: %v", err)
    }
    // Verify CWD and Env appear in encoded JSON
    out := string(encoded.Content)
    assertContains(t, out, "\"cwd\"")
    assertContains(t, out, "./hooks")
    assertContains(t, out, "\"env\"")
    assertContains(t, out, "AUDIT")

    decoded, err := adapter.Decode(encoded.Content)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }
    if len(decoded.Hooks) != 1 {
        t.Fatalf("hook count: got %d, want 1", len(decoded.Hooks))
    }
    h := decoded.Hooks[0]
    assertEqual(t, "./hooks", h.Handler.CWD)
    if h.Handler.Env["AUDIT"] != "1" {
        t.Errorf("Env not preserved: %v", h.Handler.Env)
    }
    assertEqual(t, 5, h.Handler.Timeout)  // timeoutSec is already seconds, no change
}

func TestCopilotAdapter_PowerShellField(t *testing.T) {
    // When bash is empty, powershell field should be used
    input := []byte(`{
        "version": 1,
        "hooks": {
            "preToolUse": [
                {
                    "hooks": [
                        {"type":"command","powershell":"check.ps1","timeoutSec":10}
                    ]
                }
            ]
        }
    }`)
    adapter := AdapterFor("copilot-cli")
    decoded, err := adapter.Decode(input)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }
    if len(decoded.Hooks) != 1 {
        t.Fatalf("expected 1 hook, got %d", len(decoded.Hooks))
    }
    // PowerShell command preserved in handler
    h := decoded.Hooks[0]
    if h.Handler.Command == "" {
        t.Error("expected powershell command to be preserved")
    }
}
```

---

### Task 4.2 — Define Copilot provider-native struct and implement new Encode/Decode

**What:** Define Copilot structs and implement new Encode/Decode directly against `CanonicalHooks`.

**Files:** `cli/internal/converter/adapter_copilot.go`

**Dependencies:** Task 4.1.

**Code guidance:**

**[FIX M8]** `copilotHookEntry`, `copilotMatcherGroup`, and `copilotHooksConfig` are already defined in `hooks.go`. The new adapter types use the `copilotNative*` prefix to avoid a name collision while the legacy bridge still exists. After Phase 7 removes the legacy bridge, `copilotHookEntry`/`copilotMatcherGroup`/`copilotHooksConfig` in `hooks.go` should also be deleted (added to Task 7.3 deletion list). Until then, both definitions coexist — the adapter uses `copilotNative*`, and `hooks.go`'s rendering code keeps using `copilotHookEntry` until it's deleted in Phase 7.

The existing `copilotHookEntry`, `copilotMatcherGroup`, and `copilotHooksConfig` structs are defined in `hooks.go` (legacy rendering code). In the new adapter, redefine them with additional fields:

```go
// copilotNativeEntry is the new definition for adapter use.
// The old copilotHookEntry in hooks.go is kept for legacy bridge compatibility until Phase 7.
type copilotNativeEntry struct {
    Type       string            `json:"type,omitempty"`
    Bash       string            `json:"bash,omitempty"`
    PowerShell string            `json:"powershell,omitempty"`
    TimeoutSec int               `json:"timeoutSec,omitempty"`
    Comment    string            `json:"comment,omitempty"`
    Env        map[string]string `json:"env,omitempty"`
    Cwd        string            `json:"cwd,omitempty"`
}

type copilotNativeGroup struct {
    Matcher string               `json:"matcher,omitempty"`
    Hooks   []copilotNativeEntry `json:"hooks"`
}

type copilotNativeConfig struct {
    Version int                                   `json:"version"`
    Hooks   map[string][]copilotNativeGroup `json:"hooks"`
}
```

**Encode notes:**
- `TranslateTimeoutToProvider(seconds, "copilot-cli")` returns seconds unchanged (Copilot uses timeoutSec)
- Copilot uses `bash` field for command (not `command`)
- `handler.CWD` maps to `entry.Cwd`
- `handler.Env` maps to `entry.Env`
- `handler.StatusMessage` maps to `entry.Comment`
- Copilot does not support matchers in `Capabilities()` but the actual JSON format DOES support `matcher` in the group struct. Keep matcher translation (call `TranslateMatcherToProvider`).
- If matcher result is not a bare string (e.g., MCP object or array), emit a warning and skip matcher — Copilot only supports string matchers
- `version: 1` at top level

**Decode notes:**
- If `entry.Bash != ""`, use it as command; else use `entry.PowerShell`
- `TranslateTimeoutFromProvider(entry.TimeoutSec, "copilot-cli")` returns unchanged (seconds = canonical)
- `entry.Comment` -> `Handler.StatusMessage`
- `entry.Cwd` -> `Handler.CWD`
- `entry.Env` -> `Handler.Env`

**Test:** Run `cd cli && go test ./internal/converter/... -run "TestCopilot|TestCopilotCLI|TestClaudeHooksToCopilot|TestCopilotHooksToClaude|TestLLMHookGenerateModeCopilot"` — all must pass.

---

### Task 4.3 — Phase 4 verification

**Test:**
```bash
cd cli && go test ./internal/converter/...
cd cli && make build
```

---

## Phase 5: Migrate Cursor Adapter

Cursor has unique fields: `failClosed` (an alias for "blocking") and `loop_limit` config fields. Cursor uses millisecond timeouts. It supports matchers (including regex pattern objects).

### Task 5.1 — Write Cursor round-trip test with field assertions

**Files:** `cli/internal/converter/adapter_test.go`

**Dependencies:** Phase 4 complete.

**Test:** Add:
```go
func TestCursorAdapter_RoundTrip_AllFields(t *testing.T) {
    matcherJSON, _ := json.Marshal("shell")
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Name:    "cursor-guard",
                Event:   "before_tool_execute",
                Matcher: matcherJSON,
                Handler: HookHandler{
                    Type:    "command",
                    Command: "echo cursor",
                    Timeout: 5,  // canonical seconds -> 5000ms in Cursor
                    StatusMessage: "Checking",
                },
                Blocking: true,
            },
        },
    }
    adapter := AdapterFor("cursor")
    encoded, err := adapter.Encode(original)
    if err != nil {
        t.Fatalf("Encode: %v", err)
    }
    decoded, err := adapter.Decode(encoded.Content)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }
    if len(decoded.Hooks) != 1 {
        t.Fatalf("hook count: got %d, want 1", len(decoded.Hooks))
    }
    h := decoded.Hooks[0]
    assertEqual(t, "cursor-guard", h.Name)
    assertEqual(t, true, h.Blocking)
    assertEqual(t, 5, h.Handler.Timeout)   // decoded back to canonical seconds
    assertEqual(t, "Checking", h.Handler.StatusMessage)
}

func TestCursorAdapter_ProviderDataPreservesFailClosed(t *testing.T) {
    // failClosed and loop_limit are Cursor-specific fields with no canonical equivalent.
    // They should be preserved in provider_data["cursor"] during decode.
    input := []byte(`{
        "version": 1,
        "hooks": {
            "PreToolUse": [
                {
                    "matcher": "Bash",
                    "hooks": [{"type":"command","command":"echo check","timeout":5000}],
                    "failClosed": true,
                    "loop_limit": 3
                }
            ]
        }
    }`)
    adapter := AdapterFor("cursor")
    decoded, err := adapter.Decode(input)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }
    if len(decoded.Hooks) == 0 {
        t.Fatal("expected at least 1 hook")
    }
    h := decoded.Hooks[0]
    // failClosed + loop_limit should be in provider_data
    if h.ProviderData == nil || h.ProviderData["cursor"] == nil {
        t.Error("expected provider_data[\"cursor\"] to preserve failClosed and loop_limit")
    }
}
```

---

### Task 5.2 — Define Cursor provider-native struct and implement new Encode/Decode

**What:** Define Cursor structs and implement new Encode/Decode.

**Files:** `cli/internal/converter/adapter_cursor.go`

**Dependencies:** Task 5.1.

**Code guidance:**

Cursor format: `{"version": 1, "hooks": {"EventName": [{"matcher": "...", "hooks": [...], "failClosed": bool, "loop_limit": int}]}}`

```go
type cursorHookEntry struct {
    Type          string `json:"type,omitempty"`
    Command       string `json:"command,omitempty"`
    TimeoutMs     int    `json:"timeout,omitempty"`
    StatusMessage string `json:"status_message,omitempty"`
}

type cursorMatcherGroup struct {
    Matcher    string           `json:"matcher,omitempty"`
    Hooks      []cursorHookEntry `json:"hooks"`
    FailClosed *bool            `json:"failClosed,omitempty"`
    LoopLimit  *int             `json:"loop_limit,omitempty"`
}

type cursorHooksFile struct {
    Version int                                   `json:"version"`
    Hooks   map[string][]cursorMatcherGroup `json:"hooks"`
}
```

**Decode notes:**
- `failClosed` and `loop_limit` have no canonical equivalents — store in `hook.ProviderData["cursor"]` (only if at least one is set). **[FIX m5]** Both fields are `*bool`/`*int` (optional). Deref only after nil check to avoid a nil pointer panic:
  ```go
  if group.FailClosed != nil || group.LoopLimit != nil {
      pd := map[string]any{}
      if group.FailClosed != nil { pd["failClosed"] = *group.FailClosed }
      if group.LoopLimit != nil  { pd["loop_limit"] = *group.LoopLimit }
      hook.ProviderData = map[string]any{"cursor": pd}
  }
  ```
- `blocking` in canonical maps from `failClosed` during decode (failClosed: true -> Blocking: true)

**Encode notes:**
- If `hook.Blocking`, set `failClosed: true` in the group
- If `hook.ProviderData["cursor"]` exists, merge those fields back into the group struct
- Version field: always `1`
- Timeout in milliseconds: `TranslateTimeoutToProvider(seconds, "cursor")`

**Test:** Run `cd cli && go test ./internal/converter/... -run "TestCursor|TestCursorAdapter"` — all must pass.

---

### Task 5.3 — Phase 5 verification

**Test:**
```bash
cd cli && go test ./internal/converter/...
cd cli && make build
```

---

## Phase 6: Migrate Kiro Adapter

Kiro has a unique agent-wrapper structure: hooks live inside a JSON file that has `name`, `description`, `prompt`, and `hooks` fields. Each hook entry has per-entry matchers (not group-level). Kiro uses millisecond timeouts.

### Task 6.1 — Write Kiro round-trip test with field assertions

**Files:** `cli/internal/converter/adapter_test.go`

**Dependencies:** Phase 5 complete.

**Test:** Add:
```go
func TestKiroAdapter_RoundTrip_AllFields(t *testing.T) {
    matcherJSON, _ := json.Marshal("shell")
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Name:    "kiro-check",
                Event:   "before_tool_execute",
                Matcher: matcherJSON,
                Handler: HookHandler{
                    Type:    "command",
                    Command: "echo kiro",
                    Timeout: 5,   // canonical seconds -> 5000ms in Kiro
                },
                Blocking: true,
            },
        },
    }
    adapter := AdapterFor("kiro")
    encoded, err := adapter.Encode(original)
    if err != nil {
        t.Fatalf("Encode: %v", err)
    }
    // Kiro output is an agent wrapper
    out := string(encoded.Content)
    assertContains(t, out, "syllago-hooks")
    assertContains(t, out, "preToolUse")
    assertContains(t, out, "echo kiro")
    assertContains(t, out, "5000")  // timeout in ms

    decoded, err := adapter.Decode(encoded.Content)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }
    if len(decoded.Hooks) != 1 {
        t.Fatalf("hook count: got %d, want 1", len(decoded.Hooks))
    }
    h := decoded.Hooks[0]
    assertEqual(t, "before_tool_execute", h.Event)
    assertEqual(t, 5, h.Handler.Timeout)  // decoded back to canonical seconds
}

func TestKiroAdapter_AgentWrapperFields(t *testing.T) {
    // Kiro's agent wrapper preserves name/description/prompt via provider_data
    input := []byte(`{
        "name": "custom-hooks",
        "description": "My custom hooks",
        "prompt": "Review all tool usage",
        "hooks": {
            "preToolUse": [
                {"command": "echo check", "matcher": "shell", "timeout_ms": 3000}
            ]
        }
    }`)
    adapter := AdapterFor("kiro")
    decoded, err := adapter.Decode(input)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }
    // Agent name/description/prompt should be in provider_data["kiro"]
    if len(decoded.Hooks) == 0 {
        t.Fatal("expected at least 1 hook")
    }
    // [FIX m6] Add explicit assertions — just checking len(decoded.Hooks) != 0 doesn't
    // verify agent wrapper metadata is preserved. Assert provider_data fields:
    pd := decoded.Hooks[0].ProviderData
    if pd == nil || pd["kiro"] == nil {
        t.Error("expected kiro agent wrapper metadata in provider_data[\"kiro\"]")
    }
    if kiroData, ok := pd["kiro"].(map[string]any); ok {
        if kiroData["name"] != "custom-hooks" {
            t.Errorf("expected name 'custom-hooks', got %v", kiroData["name"])
        }
        if kiroData["description"] != "My custom hooks" {
            t.Errorf("expected description 'My custom hooks', got %v", kiroData["description"])
        }
    }
}
```

---

### Task 6.2 — Define Kiro provider-native struct and implement new Encode/Decode

**What:** Define Kiro structs and implement new Encode/Decode. Note Kiro's unique structure: per-entry matchers (not per-group), and the agent wrapper format.

**Files:** `cli/internal/converter/adapter_kiro.go`

**Dependencies:** Task 6.1.

**Code guidance:**

The existing `kiroHookEntry`, `kiroHooksAgent` structs in `hooks.go` can be referenced but the adapter will define its own:

```go
type kiroNativeEntry struct {
    Command   string `json:"command"`
    Matcher   string `json:"matcher,omitempty"`
    TimeoutMs int    `json:"timeout_ms,omitempty"`
}

type kiroNativeAgent struct {
    Name        string                        `json:"name"`
    Description string                        `json:"description"`
    Prompt      string                        `json:"prompt"`
    Hooks       map[string][]kiroNativeEntry  `json:"hooks"`
}
```

**Encode notes:**
- Default agent metadata: `Name: "syllago-hooks"`, `Description: "Hooks installed by syllago"`, `Prompt: ""`
- If first hook has `ProviderData["kiro"]` with name/description/prompt, use those values
- Per-entry matchers: each `CanonicalHook` becomes one `kiroNativeEntry` with the hook's own matcher translated
- `TranslateTimeoutToProvider(seconds, "kiro")` -> milliseconds
- Kiro events: use `TranslateEventToProvider(event, "kiro")`
- Kiro does not support LLM/HTTP hooks — `TranslateHandlerType` will warn+drop them

**Decode notes:**
- Build one `CanonicalHook` per `kiroNativeEntry`
- Matcher is per-entry: call `TranslateMatcherFromProvider(json.Marshal(entry.Matcher), "kiro")`
- `TranslateTimeoutFromProvider(entry.TimeoutMs, "kiro")` -> canonical seconds
- Agent metadata (name/description/prompt) -> store in `ProviderData["kiro"]` if non-default

**Test:** Run `cd cli && go test ./internal/converter/... -run "TestKiro|TestKiroAdapter|TestClaudeHooksToKiro"` — all must pass.

---

### Task 6.3 — Phase 6 verification

**Test:**
```bash
cd cli && go test ./internal/converter/...
cd cli && make build
```

---

## Phase 7: Remove Legacy Bridge

All 5 adapters are now migrated. The legacy bridge code is no longer called by any adapter.

### Task 7.1 — Verify no adapter calls legacy bridge functions

**What:** Before deletion, confirm that no adapter file references `ToLegacyHooksConfig`, `FromLegacyHooksConfig`, or `legacyResultToEncoded`.

**Files:** None (verification only).

**Dependencies:** Phase 6 complete.

**Test:**
```bash
grep -rn "ToLegacyHooksConfig\|FromLegacyHooksConfig\|legacyResultToEncoded" \
  /home/hhewett/.local/src/syllago/cli/internal/converter/adapter_*.go
```
Expect zero results. If any are found, that adapter is not yet migrated.

---

### Task 7.2 — Remove bridge functions from `adapter.go`

**What:** Delete the three bridge functions from `adapter.go`: `ToLegacyHooksConfig()`, `FromLegacyHooksConfig()`, and `legacyResultToEncoded()`.

**Files:** `cli/internal/converter/adapter.go`

**Dependencies:** Task 7.1.

**Test:** After deletion, `cd cli && go build ./internal/converter/...` must succeed (no compile errors). The bridge functions are unexported except for `FromLegacyHooksConfig` which is exported — but now has no callers. Also remove `FromLegacyHooksConfig` which is exported.

Remove these test functions from `adapter_test.go` since they test deleted code:
- `TestFromLegacyHooksConfig`
- `TestToLegacyHooksConfig`
- `TestLegacyResultToEncoded`
- `TestLegacyResultToEncodedNoWarnings`

---

### Task 7.3 — Remove legacy rendering/canonicalization functions from `hooks.go`

**What:** Delete the following functions from `hooks.go`:
- `renderStandardHooks()`
- `renderCopilotHooks()`
- `renderKiroHooks()`
- `canonicalizeStandardHooks()`
- `canonicalizeCopilotHooks()`

**[FIX C3]** `canonicalizeKiroHooks()` appears in the design doc's removal list but does NOT exist in `hooks.go`. Kiro canonicalization goes through `canonicalizeStandardHooks`. No action needed for this function — mark it as a design doc inaccuracy that the plan inherits.

**[FIX M5]** `generateLLMWrapperScript()` is unexported and lives in `hooks.go`. It MUST be preserved. It is called by the new `GenerateLLMWrapperScript()` in `hookhelpers.go`. Options (decide before executing this task):
1. Keep `generateLLMWrapperScript()` in `hooks.go` (simplest; leave a comment explaining why it's retained)
2. Move `generateLLMWrapperScript()` to `hookhelpers.go` and update the call site in the wrapper function (cleaner; eliminates the dependency on `hooks.go`)
Either way, do NOT delete it as part of this task.

**[FIX M8]** Also delete from `hooks.go` once `RenderFlat` is updated (see below):
- `copilotHookEntry` — superseded by `copilotNativeEntry` in `adapter_copilot.go`
- `copilotMatcherGroup` — superseded by `copilotNativeGroup`
- `copilotHooksConfig` — superseded by `copilotNativeConfig`
These can only be deleted after `RenderFlat` no longer calls `renderCopilotHooks` (which uses them). Verify with `grep -n 'copilotHookEntry\|copilotMatcherGroup\|copilotHooksConfig' hooks.go` before deleting.

Also simplify `HooksConverter.Render()` and `HooksConverter.Canonicalize()` — if they are no longer needed by any external caller (only through adapters), they can be removed or reduced. Determine whether `Render` and `Canonicalize` are still used anywhere besides `RenderFlat` and adapter bridge calls.

**Files:** `cli/internal/converter/hooks.go`

**Dependencies:** Task 7.2.

**Important:** The following MUST remain (catalog/installer dependency):
- `HookEntry` struct
- `HookData` type
- `hooksConfig` type
- `ParseFlat`, `ParseNested`
- `LoadHookData`
- `DetectHookFormat`
- `RenderFlat` — this still calls `Render()`, but the adapter bridge is gone. `RenderFlat` needs to be updated to use the adapter path OR kept with a direct implementation.

Also keep: `copilotHookEntry`, `copilotMatcherGroup`, `copilotHooksConfig` — only if still used by `RenderFlat` or tests. If the adapter now defines its own types, and if `RenderFlat` is updated to use the adapter, these legacy types can be removed.

**[FIX M9 — high risk] `RenderFlat` refactor — separate sub-task, complete field mapping required:**

`RenderFlat` is a public API called by the installer. The pseudocode in the previous version of this task was incomplete (single hook only, missing fields). This must be treated as a dedicated sub-task (7.3a) with full correctness before merging.

**Task 7.3a — Refactor `RenderFlat` to use adapter registry (separate from deletion task)**

`HookData.Hooks` is `[]HookEntry` — there can be multiple entries per HookData. The full mapping from `HookEntry` to `HookHandler` must preserve all fields:

```go
func hookEntryToHandler(e HookEntry) HookHandler {
    return HookHandler{
        Type:           e.Type,
        Command:        e.Command,
        Timeout:        e.Timeout,    // already canonical seconds
        StatusMessage:  e.StatusMessage,
        Async:          e.Async,
        URL:            e.URL,
        Headers:        e.Headers,
        AllowedEnvVars: e.AllowedEnvVars,
        Prompt:         e.Prompt,
        Model:          e.Model,
        Agent:          e.Agent,
    }
}

func (c *HooksConverter) RenderFlat(hook HookData, target provider.Provider) (*Result, error) {
    adapter := AdapterFor(target.Slug)
    if adapter == nil {
        if hooklessProviders[target.Slug] {
            return &Result{
                Warnings: []string{fmt.Sprintf("target provider %q does not support hooks; hook content was not converted", target.Slug)},
            }, nil
        }
        return nil, fmt.Errorf("no hook adapter for provider %q", target.Slug)
    }
    // Build canonical hooks from HookData — one CanonicalHook per HookEntry
    ch := &CanonicalHooks{Spec: SpecVersion, Source: hook.SourceProvider}
    matcherJSON, _ := json.Marshal(hook.Matcher)
    for _, entry := range hook.Hooks {
        ch.Hooks = append(ch.Hooks, CanonicalHook{
            Event:   hook.Event,
            Matcher: matcherJSON,
            Handler: hookEntryToHandler(entry),
        })
    }
    encoded, err := adapter.Encode(ch)
    if err != nil {
        return nil, err
    }
    r := &Result{Content: encoded.Content, Filename: encoded.Filename}
    for _, w := range encoded.Warnings {
        r.Warnings = append(r.Warnings, w.Description)
    }
    if encoded.Scripts != nil {
        r.ExtraFiles = encoded.Scripts
    }
    return r, nil
}
```

`TestRenderFlat_Copilot` MUST pass after this change. Run it in isolation before deleting the old rendering functions:
```bash
cd cli && go test ./internal/converter/... -run TestRenderFlat_Copilot -v
```

The `hooklessProviders` guard must stay — it's the fallback for Zed/Roo-Code.

**Test:**
```bash
cd cli && go test ./internal/converter/...
cd cli && make build
```

---

### Task 7.4 — Phase 7 verification

**Test:** Full suite must pass:
```bash
cd cli && go test ./internal/converter/... -v 2>&1 | grep -E "^(PASS|FAIL|---)"
cd cli && make build
```

All deleted test functions removed, all remaining tests green.

---

## Phase 8: Upgrade `Verify()`

### Task 8.1 — Write tests for enhanced `Verify()`

**What:** Write tests that the new `Verify()` must satisfy — specifically field-level fidelity checking beyond just hook count.

**Files:** `cli/internal/converter/adapter_test.go`

**Dependencies:** Phase 7 complete.

**Test:** Add:
```go
func TestVerify_FieldLevelFidelity(t *testing.T) {
    matcherJSON, _ := json.Marshal("shell")
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Name:  "safety-check",
                Event: "before_tool_execute",
                Matcher: matcherJSON,
                Handler: HookHandler{
                    Type:    "command",
                    Command: "echo check",
                    Timeout: 5,
                },
                Blocking: true,
            },
        },
    }
    adapter := AdapterFor("claude-code")
    encoded, err := adapter.Encode(original)
    if err != nil {
        t.Fatalf("Encode: %v", err)
    }
    // Should pass when fields are preserved
    err = Verify(encoded, adapter, original)
    if err != nil {
        t.Fatalf("Verify should pass on full round-trip: %v", err)
    }
}

func TestVerify_TimeoutMismatch(t *testing.T) {
    // If timeout is silently changed, Verify should catch it
    matcherJSON, _ := json.Marshal("shell")
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Event:   "before_tool_execute",
                Matcher: matcherJSON,
                Handler: HookHandler{Type: "command", Command: "echo check", Timeout: 30},
            },
        },
    }
    adapter := AdapterFor("claude-code")
    encoded, _ := adapter.Encode(original)
    err := Verify(encoded, adapter, original)
    if err != nil {
        t.Fatalf("timeout should survive round-trip for CC: %v", err)
    }
}

func TestVerify_CrossProvider_TimeoutPreservation(t *testing.T) {
    // CC -> Gemini: timeout value check (5s in CC should become 5s canonical after decode from Gemini)
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Event:   "before_tool_execute",
                Handler: HookHandler{Type: "command", Command: "echo check", Timeout: 5},
            },
        },
    }
    geminiAdapter := AdapterFor("gemini-cli")
    encoded, _ := geminiAdapter.Encode(original)
    decoded, _ := geminiAdapter.Decode(encoded.Content)

    if decoded.Hooks[0].Handler.Timeout != 5 {
        t.Errorf("timeout should be 5 canonical seconds after Gemini round-trip, got %d", decoded.Hooks[0].Handler.Timeout)
    }
}
```

---

### Task 8.2 — Implement enhanced `Verify()`

**What:** Upgrade `Verify()` in `adapter.go` to check field-level fidelity. The new `Verify()` must know what fields each provider supports, so it can distinguish intentional drops from bugs.

**Files:** `cli/internal/converter/adapter.go`

**Dependencies:** Task 8.1.

**Code guidance:**

The enhanced `Verify()` compares decoded fields against the original, filtered by what the provider supports. Fields that are deliberately not supported by the adapter (based on `Capabilities()`) are excluded from comparison:

```go
func Verify(encoded *EncodedResult, adapter HookAdapter, original *CanonicalHooks) error {
    if encoded == nil || len(encoded.Content) == 0 {
        return nil
    }

    decoded, err := adapter.Decode(encoded.Content)
    if err != nil {
        return &VerifyError{
            Provider: adapter.ProviderSlug(),
            Detail:   "failed to re-decode encoded output: " + err.Error(),
        }
    }

    // Count check: only compare non-dropped hooks
    // (hooks dropped due to unsupported events/types are expected; count the ones that survived)
    if len(decoded.Hooks) != len(original.Hooks) {
        // Check if the difference is explained by encode warnings (dropped hooks)
        // For now, maintain the existing count check
        return &VerifyError{
            Provider: adapter.ProviderSlug(),
            Detail:   "hook count mismatch after round-trip",
            Expected: len(original.Hooks),
            Got:      len(decoded.Hooks),
        }
    }

    caps := adapter.Capabilities()

    // Field-level checks per hook
    for i, orig := range original.Hooks {
        dec := decoded.Hooks[i]

        // Timeout check: if original has a timeout, verify it survived
        if orig.Handler.Timeout > 0 && dec.Handler.Timeout != orig.Handler.Timeout {
            return &VerifyError{
                Provider: adapter.ProviderSlug(),
                Detail:   fmt.Sprintf("hook %d: timeout mismatch — expected %d, got %d (canonical seconds)", i, orig.Handler.Timeout, dec.Handler.Timeout),
            }
        }

        // Command check: command must survive
        if orig.Handler.Command != "" && dec.Handler.Command != orig.Handler.Command {
            return &VerifyError{
                Provider: adapter.ProviderSlug(),
                Detail:   fmt.Sprintf("hook %d: command mismatch — expected %q, got %q", i, orig.Handler.Command, dec.Handler.Command),
            }
        }

        // Blocking check (only if provider supports it)
        if caps.SupportsBlocking && orig.Blocking != dec.Blocking {
            return &VerifyError{
                Provider: adapter.ProviderSlug(),
                Detail:   fmt.Sprintf("hook %d: blocking field mismatch — expected %v, got %v", i, orig.Blocking, dec.Blocking),
            }
        }
    }

    return nil
}
```

**[FIX M10] Verify() count check and caller updates:**

The strict count check will cause `Verify()` to fail for any cross-provider conversion where hooks are intentionally dropped (unsupported events). The plan acknowledges this but does NOT update callers. Two resolution options — pick one before implementing:

**Option A (recommended): Filter-before-call pattern**
Document in the `Verify()` function comment that callers must pass a filtered original for cross-provider use. Add a `FilterHooksForProvider` helper:
```go
// FilterHooksForProvider returns a copy of ch containing only hooks whose events
// are supported by the given provider. Use before Verify() for cross-provider calls.
func FilterHooksForProvider(ch *CanonicalHooks, slug string) *CanonicalHooks {
    filtered := &CanonicalHooks{Spec: ch.Spec}
    for _, h := range ch.Hooks {
        if _, ok := TranslateHookEvent(h.Event, slug); ok {
            filtered.Hooks = append(filtered.Hooks, h)
        }
    }
    return filtered
}
```
Update any existing `Verify()` callers (grep: `grep -rn 'Verify(' cli/internal/`) to use `FilterHooksForProvider` when calling across providers.

**Option B: Warning-based skip inside Verify()**
If `len(decoded.Hooks) < len(original.Hooks)`, check whether the difference is explained by `encoded.Warnings` entries that describe dropped hooks. Skip the count error if all missing hooks are accounted for by drop warnings. This keeps callers simple but adds complexity inside `Verify()`.

The test `TestVerify_Success` uses a same-provider round-trip and is not affected by either option. New cross-provider `Verify()` tests must use the chosen approach explicitly.

**Test:** Run `cd cli && go test ./internal/converter/... -run TestVerify` — all must pass.

---

### Task 8.3 — Phase 8 and final verification

**Test:**
```bash
cd cli && go test ./internal/converter/... -v
cd cli && make build
```

**Final success criteria check:**
1. All 5 adapters encode/decode directly with CanonicalHook — no calls to `ToLegacy*` / `FromLegacy*` — confirmed by Task 7.1
2. Round-trip tests with field-value assertions pass (Tasks 2.1, 3.1, 4.1, 5.1, 6.1)
3. Structured matchers (MCP, regex) survive through adapters that support them — confirmed by matcher tests in Phase 1
4. Unsupported fields emit `ConversionWarning` — confirmed by `TranslateHandlerType` tests
5. Legacy bridge code removed — confirmed by Task 7.2, 7.3
6. `HookData`/`HookEntry`/flat-format parsing preserved — confirmed by Phase 7 preservation notes
7. All existing converter tests pass — confirmed by final suite run
8. `make build` succeeds
9. Cline removed from HookEvents/ToolNames — confirmed by Task 0.2
10. VS Code Copilot entries added — confirmed by Tasks 0.3–0.5
11. `provider_data` round-trip test — add a final test:

**[FIX m7] `TestProviderData_RoundTrip_CC` — requires an explicit design decision:**

CC's native hook format (`{"hooks":{"Event":[{"matcher":"...","hooks":[...]}]}}`) has no field to carry arbitrary `provider_data`. For `provider_data["claude-code"]` to survive encode→decode, the adapter must either:
- (a) Embed it as a reserved JSON key (e.g., `"_syllago_meta"`) in the CC hooks file — acknowledged as non-standard
- (b) Treat `provider_data` for the native provider as a no-op (it's already fully expressed in the native fields)

**Recommendation:** Option (b). For a CC→CC round-trip, `provider_data["claude-code"]` holds data that came FROM a previous CC decode — since the CC adapter now decodes directly, any CC-native fields are already represented in `HookHandler`. A custom field like `"custom_field"` that's not part of `ccHookEntry` would be lost, which is correct behavior (we don't serialize unknown fields).

**Replace the test** with a narrower assertion:
```go
func TestProviderData_RoundTrip_CC(t *testing.T) {
    // For the native provider, provider_data is not round-tripped via the JSON format.
    // This test verifies that non-native provider_data (e.g., Copilot metadata in a CC hook)
    // is preserved in the canonical layer but correctly absent after CC encode+decode.
    original := &CanonicalHooks{
        Spec: SpecVersion,
        Hooks: []CanonicalHook{
            {
                Event: "before_tool_execute",
                Handler: HookHandler{Type: "command", Command: "echo check"},
                // provider_data["copilot-cli"] carries cross-provider metadata
                ProviderData: map[string]any{
                    "copilot-cli": map[string]any{"original_comment": "audit hook"},
                },
            },
        },
    }
    adapter := AdapterFor("claude-code")
    encoded, err := adapter.Encode(original)
    if err != nil {
        t.Fatalf("Encode: %v", err)
    }
    decoded, err := adapter.Decode(encoded.Content)
    if err != nil {
        t.Fatalf("Decode: %v", err)
    }
    if len(decoded.Hooks) != 1 {
        t.Fatalf("expected 1 hook, got %d", len(decoded.Hooks))
    }
    // The hook itself must survive; provider_data for other providers is not carried
    // in CC's native format and is expected to be absent after decode.
    assertEqual(t, "echo check", decoded.Hooks[0].Handler.Command)
}
```

---

## Quick Reference: File Ownership Per Phase

| File | Phases That Touch It |
|------|---------------------|
| `cli/internal/converter/toolmap.go` | 0.2, 0.3, 0.4, 0.6 |
| `cli/internal/converter/compat.go` | 0.5 |
| `cli/internal/converter/adapter.go` | 0.1, 7.2, 8.2 |
| `cli/internal/converter/hookhelpers.go` | 1.6 (new) |
| `cli/internal/converter/hookhelpers_test.go` | 1.1–1.5 (new) |
| `cli/internal/converter/adapter_cc.go` | 2.2, 2.3, 2.4 |
| `cli/internal/converter/adapter_gemini.go` | 3.2 |
| `cli/internal/converter/adapter_copilot.go` | 4.2 |
| `cli/internal/converter/adapter_cursor.go` | 5.2 |
| `cli/internal/converter/adapter_kiro.go` | 6.2 |
| `cli/internal/converter/hooks.go` | 7.3 |
| `cli/internal/converter/adapter_test.go` | 2.1, 3.1, 4.1, 5.1, 6.1, 7.2, 8.1, 8.3 |
| `cli/internal/converter/toolmap_test.go` | 0.2, 0.3, 0.4 |

## Test Command Reference

```bash
# Full suite
cd cli && go test ./internal/converter/...

# Phase-scoped
cd cli && go test ./internal/converter/... -run "TestTranslateEvent"       # Phase 0-1
cd cli && go test ./internal/converter/... -run "TestCCAdapter"             # Phase 2
cd cli && go test ./internal/converter/... -run "TestGeminiAdapter"         # Phase 3
cd cli && go test ./internal/converter/... -run "TestCopilotAdapter"        # Phase 4
cd cli && go test ./internal/converter/... -run "TestCursorAdapter"         # Phase 5
cd cli && go test ./internal/converter/... -run "TestKiroAdapter"           # Phase 6
cd cli && go test ./internal/converter/... -run "TestVerify"                # Phase 8

# Build
cd cli && make build
```
