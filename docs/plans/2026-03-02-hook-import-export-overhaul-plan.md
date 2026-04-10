# Hook Import/Export Overhaul — Implementation Plan

**Design doc:** `docs/plans/2026-03-02-hook-import-export-overhaul-design.md`
**Status:** Ready for implementation

---

## Overview

This plan covers all 16 design sections, grouped into 6 phases. Each task is scoped to 1–3 files and produces a testable state.

**Phases:**
1. Foundation — flat format, content migration, installer fixes
2. Compatibility Engine — `compat.go`, capability registry, `AnalyzeHookCompat`
3. Snapshot Safety + Hash-Based Uninstall
4. TUI: Hooks List (compatibility matrix)
5. TUI: Hook Detail (4-tab layout, Compatibility tab, Install tab warnings)
6. Import Flow — splitting, naming, TUI wizard, CLI flags, scope detection

---

## Phase 1: Foundation

Ground the rest of the work in the correct storage format and a working installer.

---

### Task 1.1 — Migrate existing content to flat format

**Design sections:** 9 (canonical storage format), 13 (migration)

**What:** Convert the two existing example hooks from nested `{"hooks":{...}}` format to the flat installer format (`{"event":"...","hooks":[...]}`). The kitchen-sink example has 5 matcher groups across 3 events — split each into a separate directory. Rename the directories to reflect their content.

**Files to modify:**

- `/home/hhewett/.local/src/syllago/content/hooks/claude-code/example-lint-on-save/hook.json`
  - Convert from `{"hooks":{"PostToolUse":[{"matcher":"Write|Edit","command":"..."}]}}` to:
    ```json
    {
      "event": "PostToolUse",
      "matcher": "Write|Edit",
      "hooks": [
        {
          "type": "command",
          "command": "echo 'Lint check placeholder - replace with your linter command'"
        }
      ]
    }
    ```
  - Note: the existing `example-lint-on-save/hook.json` uses a non-standard simplified format (command directly on matcher, no inner `hooks` array). Normalize to the canonical flat format with the `hooks` array.

**Files to create (split kitchen-sink into 5 items):**

The kitchen-sink has 5 matcher groups:
1. `PreToolUse` / `Bash` matcher — validating-shell-command
2. `PreToolUse` / `""` matcher (LLM prompt) — pretooluse-ai-safety-check
3. `PostToolUse` / `Write|Edit` matcher — running-linter
4. `PostToolUse` / `""` matcher — logging-tool-usage
5. `Notification` / `""` matcher — processing-notification

Create directories for items 1, 3, 4, 5 (item 2 is an LLM hook — keep it but note it will show `None` for non-Claude providers):

```
content/hooks/claude-code/
  validating-shell-command/
    hook.json      # {"event":"PreToolUse","matcher":"Bash","hooks":[...]}
    .syllago.yaml  # copy from kitchen-sink, update description
    README.md      # create minimal
  pretooluse-ai-safety-check/
    hook.json      # LLM prompt hook
    .syllago.yaml
  running-linter/
    hook.json      # {"event":"PostToolUse","matcher":"Write|Edit","hooks":[...]}
    .syllago.yaml
  logging-tool-usage/
    hook.json      # {"event":"PostToolUse","hooks":[...]}  (no matcher)
    .syllago.yaml
  processing-notification/
    hook.json      # {"event":"Notification","hooks":[...]}
    .syllago.yaml
```

Delete `content/hooks/claude-code/example-kitchen-sink-hooks/`.

**Tests:** None needed — this is content migration, not code. Verify with `syllago` list after `make build`.

**Dependencies:** None. Do this first so all subsequent tasks have the right input data.

---

### Task 1.2 — Fix `parseHookFile` to resolve content file from directory

**Design sections:** 9 (key gap: `item.Path` is a directory for directory-format items)

**What:** `installHook` calls `parseHookFile(item.Path)` but for directory-format items, `item.Path` is a directory (e.g., `.../hooks/claude-code/validating-shell-command/`). `os.ReadFile` on a directory fails. Fix it to resolve `hook.json` inside the directory.

**Files to modify:**

- `cli/internal/installer/hooks.go`

**Change:** In `parseHookFile`, detect whether `path` is a directory. If so, resolve `hook.json` inside it:

```go
func parseHookFile(path string) (event string, matcherGroup []byte, err error) {
    fi, err := os.Stat(path)
    if err != nil {
        return "", nil, err
    }
    if fi.IsDir() {
        path = filepath.Join(path, "hook.json")
    }

    data, err := os.ReadFile(path)
    // ... rest unchanged
}
```

The same fix is needed in `uninstallHook` and `checkHookStatus` which also call `parseHookFile(item.Path)`. Since they all go through `parseHookFile`, the single fix covers all three.

**Tests:** Add to `cli/internal/installer/hooks_test.go` (create if it doesn't exist):

```go
func TestParseHookFile_DirectoryFormat(t *testing.T) {
    // Create temp dir with hook.json in flat format
    dir := t.TempDir()
    hookJSON := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`
    os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644)

    event, matcherGroup, err := parseHookFile(dir)
    if err != nil { t.Fatal(err) }
    if event != "PreToolUse" { t.Errorf("got event %q", event) }
    // matcherGroup should not contain "event" field
    if gjson.GetBytes(matcherGroup, "event").Exists() {
        t.Error("matcherGroup should not contain event field")
    }
}

func TestParseHookFile_FileFormat(t *testing.T) {
    // Legacy: path points directly to a .json file
    dir := t.TempDir()
    hookPath := filepath.Join(dir, "hook.json")
    hookJSON := `{"event":"PostToolUse","hooks":[{"type":"command","command":"echo done"}]}`
    os.WriteFile(hookPath, []byte(hookJSON), 0644)

    event, _, err := parseHookFile(hookPath)
    if err != nil { t.Fatal(err) }
    if event != "PostToolUse" { t.Errorf("got event %q", event) }
}
```

**Dependencies:** Task 1.1 (so the test content exists in the right format).

---

### Task 1.3 — Update converter to handle flat format as canonical

**Design sections:** 9 (canonical storage format — flat is the new canonical for individual items)

**What:** The converter's `Canonicalize` method currently expects the nested `{"hooks":{...}}` format. After the migration, individual items are stored in flat format. Add a `CanonicalizeFlat` method (or update `Canonicalize` to auto-detect format) that handles `{"event":"...","matcher":"...","hooks":[...]}` as input.

Also update `Render` to accept flat format as input (for export of individual items).

**Files to modify:**

- `cli/internal/converter/hooks.go`

**Changes:**

Add a helper that auto-detects format and normalizes to a single internal representation:

```go
// HookData is the internal canonical representation of a single hook group.
// This is the flat format: one event + one matcher + one or more hook entries.
type HookData struct {
    Event   string      `json:"event"`
    Matcher string      `json:"matcher,omitempty"`
    Hooks   []hookEntry `json:"hooks"`
}

// parseFlat parses the flat {"event":"...","matcher":"...","hooks":[...]} format.
func parseFlat(content []byte) (HookData, error)

// parseNested parses the nested {"hooks":{"EventName":[...]}} format and returns
// all hook groups as individual HookData items.
func parseNested(content []byte) ([]HookData, error)

// DetectFormat returns "flat" or "nested" based on JSON structure.
func DetectFormat(content []byte) string
```

Export `HookData` so `compat.go` (Task 2.1) can reference it.

Update `Canonicalize` to dispatch based on detected format:
- Flat input → `parseFlat` → translate event/tool names → return as flat JSON
- Nested input → `parseNested` (unchanged existing logic for batch import)

The `Render` method currently takes `hooksConfig` (nested). Add `RenderFlat(hook HookData, target provider.Provider) (*Result, error)` for single-item export.

**Tests:** Add to `cli/internal/converter/hooks_test.go`:

```go
func TestDetectFormat(t *testing.T) {
    flat := `{"event":"PreToolUse","hooks":[...]}`
    nested := `{"hooks":{"PreToolUse":[...]}}`
    assert("flat", DetectFormat([]byte(flat)))
    assert("nested", DetectFormat([]byte(nested)))
}

func TestCanonicalizeFlat_GeminiCLI(t *testing.T) {
    // BeforeTool (Gemini) → PreToolUse (canonical)
    input := `{"event":"BeforeTool","matcher":"run_shell_command","hooks":[...]}`
    result, _ := c.Canonicalize([]byte(input), "gemini-cli")
    // Verify event translated, matcher translated
}

func TestRenderFlat_Copilot(t *testing.T) {
    hook := HookData{Event: "PreToolUse", Matcher: "Bash", Hooks: []hookEntry{...}}
    // Marshal hook, call RenderFlat
    // Verify matcher dropped with warning, correct copilot format
}
```

**Dependencies:** Task 1.1 (correct content format to test against).

---

## Phase 2: Compatibility Engine

Build the single source of truth for provider capability, used by all subsequent phases.

---

### Task 2.1 — Create `compat.go` with capability registry and `CompatLevel` types

**Design sections:** 8 (compatibility computation engine)

**What:** New file `cli/internal/converter/compat.go` defining all types and the `HookCapabilities` registry. No computation yet — just the data structures and the static table.

**Files to create:**

- `cli/internal/converter/compat.go`

```go
package converter

// CompatLevel represents the compatibility level of a hook for a target provider.
type CompatLevel int

const (
    CompatFull     CompatLevel = iota // All features translate, no behavioral change
    CompatDegraded                    // Minor features lost, core behavior unchanged
    CompatBroken                      // Hook runs but behavior is fundamentally wrong
    CompatNone                        // Cannot install — event doesn't exist on target
)

func (l CompatLevel) Symbol() string {
    switch l {
    case CompatFull:     return "✓"
    case CompatDegraded: return "~"
    case CompatBroken:   return "!"
    case CompatNone:     return "✗"
    }
    return "?"
}

func (l CompatLevel) Label() string {
    switch l {
    case CompatFull:     return "Full"
    case CompatDegraded: return "Degraded"
    case CompatBroken:   return "Broken"
    case CompatNone:     return "None"
    }
    return "Unknown"
}

// HookFeature identifies a specific hook capability that may or may not be
// supported by a given provider.
type HookFeature int

const (
    FeatureMatcher       HookFeature = iota
    FeatureAsync
    FeatureStatusMessage
    FeatureLLMHook
    FeatureTimeout       // fine-grained (ms) vs coarse (seconds)
)

// FeatureSupport describes how a provider handles a specific hook feature.
type FeatureSupport struct {
    Supported bool
    Notes     string      // e.g., "mapped to 'comment' field", "per-entry (not group-level)"
    LostLevel CompatLevel // impact level when this feature is not available in source hook
}

// ProviderCapability describes what hook features a provider supports.
type ProviderCapability struct {
    Features map[HookFeature]FeatureSupport
}

// HookCapabilities is the single source of truth for provider hook support.
// Used by AnalyzeHookCompat, TUI rendering, and tests.
var HookCapabilities = map[string]ProviderCapability{
    "claude-code": {
        Features: map[HookFeature]FeatureSupport{
            FeatureMatcher:       {Supported: true},
            FeatureAsync:         {Supported: true},
            FeatureStatusMessage: {Supported: true},
            FeatureLLMHook:       {Supported: true},
            FeatureTimeout:       {Supported: true, Notes: "milliseconds"},
        },
    },
    "gemini-cli": {
        Features: map[HookFeature]FeatureSupport{
            FeatureMatcher:       {Supported: true},
            FeatureAsync:         {Supported: true},
            FeatureStatusMessage: {Supported: true, Notes: "standard field"},
            FeatureLLMHook:       {Supported: false, LostLevel: CompatNone},
            FeatureTimeout:       {Supported: true, Notes: "milliseconds"},
        },
    },
    "copilot-cli": {
        Features: map[HookFeature]FeatureSupport{
            FeatureMatcher:       {Supported: false, LostLevel: CompatBroken, Notes: "hook fires on ALL tool calls"},
            FeatureAsync:         {Supported: false, LostLevel: CompatBroken, Notes: "hook will block execution"},
            FeatureStatusMessage: {Supported: true, Notes: "mapped to 'comment' field"},
            FeatureLLMHook:       {Supported: false, LostLevel: CompatNone},
            FeatureTimeout:       {Supported: true, Notes: "converted ms→seconds, precision lost", LostLevel: CompatDegraded},
        },
    },
    "kiro": {
        Features: map[HookFeature]FeatureSupport{
            FeatureMatcher:       {Supported: true, Notes: "per-entry (not group-level)"},
            FeatureAsync:         {Supported: false, LostLevel: CompatBroken, Notes: "hook will block execution"},
            FeatureStatusMessage: {Supported: false, LostLevel: CompatDegraded, Notes: "no user-visible status"},
            FeatureLLMHook:       {Supported: false, LostLevel: CompatNone},
            FeatureTimeout:       {Supported: true, Notes: "milliseconds"},
        },
    },
}

// HookProviders returns the slugs of providers that support hooks, in display order.
func HookProviders() []string {
    return []string{"claude-code", "gemini-cli", "copilot-cli", "kiro"}
}
```

**Tests:** `cli/internal/converter/compat_test.go`

```go
func TestHookCapabilities_AllProvidersPresent(t *testing.T) {
    for _, slug := range HookProviders() {
        if _, ok := HookCapabilities[slug]; !ok {
            t.Errorf("HookCapabilities missing provider %q", slug)
        }
    }
}

func TestHookCapabilities_AllFeaturesPresent(t *testing.T) {
    allFeatures := []HookFeature{FeatureMatcher, FeatureAsync, FeatureStatusMessage, FeatureLLMHook, FeatureTimeout}
    for slug, cap := range HookCapabilities {
        for _, f := range allFeatures {
            if _, ok := cap.Features[f]; !ok {
                t.Errorf("provider %q missing feature %d", slug, f)
            }
        }
    }
}

func TestCompatLevel_Symbol(t *testing.T) {
    cases := []struct{ level CompatLevel; sym string }{
        {CompatFull, "✓"}, {CompatDegraded, "~"}, {CompatBroken, "!"}, {CompatNone, "✗"},
    }
    for _, tc := range cases {
        if got := tc.level.Symbol(); got != tc.sym {
            t.Errorf("level %d: got %q want %q", tc.level, got, tc.sym)
        }
    }
}
```

**Dependencies:** None. Pure data definition.

---

### Task 2.2 — Implement `AnalyzeHookCompat`

**Design sections:** 8 (compatibility computation engine)

**What:** Add the computation function to `compat.go`. Takes a `HookData` (from Task 1.3) and a target provider slug, returns a `CompatResult` summarizing what's compatible and what's lost.

**Files to modify:**

- `cli/internal/converter/compat.go`

**Add types and function:**

```go
// FeatureResult describes what happens to one feature when targeting a provider.
type FeatureResult struct {
    Feature   HookFeature
    Present   bool   // true if the source hook uses this feature
    Supported bool   // true if the target provider supports this feature
    Impact    CompatLevel
    Notes     string
}

// CompatResult is the output of AnalyzeHookCompat for one hook + one provider.
type CompatResult struct {
    Provider string
    Level    CompatLevel     // worst level across all features + event support
    Notes    string          // short summary note (e.g., "Native format", "Matcher dropped")
    Features []FeatureResult // per-feature breakdown, only features present in source hook
}

// AnalyzeHookCompat computes compatibility for a single hook against a target provider.
// Algorithm:
//   1. Check event support via HookEvents (CompatNone if not supported)
//   2. Check each feature present in the source hook against HookCapabilities
//   3. Aggregate to worst CompatLevel across all checks
func AnalyzeHookCompat(hook HookData, targetProvider string) CompatResult
```

**Algorithm detail:**
- Start with `level = CompatFull`
- If `TranslateHookEvent(hook.Event, targetProvider)` returns `supported=false` → return `CompatNone` immediately (no further checks needed)
- If any `hookEntry` in `hook.Hooks` has `type == "prompt"` or `type == "agent"` → check `FeatureLLMHook`; LLM hooks on Claude Code targeting non-Claude → `CompatNone` for that entry
- If `hook.Matcher != ""` → check `FeatureMatcher` against target
- If any entry has `async: true` → check `FeatureAsync` against target
- If any entry has `statusMessage != ""` → check `FeatureStatusMessage` against target
- If any entry has `timeout > 0` → check `FeatureTimeout` (only relevant for Copilot's ms→seconds rounding)
- `level = max(level, feature.LostLevel)` for each unsupported feature
- Produce `CompatResult` with populated `Features` slice

**Tests:** `cli/internal/converter/compat_test.go` (extend file from Task 2.1):

```go
func TestAnalyzeHookCompat_FullCompat(t *testing.T) {
    // Simple command hook, PreToolUse, Bash matcher → Full for claude-code, gemini-cli
    hook := HookData{
        Event: "PreToolUse", Matcher: "Bash",
        Hooks: []hookEntry{{Type: "command", Command: "go vet ./..."}},
    }
    r := AnalyzeHookCompat(hook, "claude-code")
    if r.Level != CompatFull { t.Errorf("expected Full, got %v", r.Level) }

    r2 := AnalyzeHookCompat(hook, "gemini-cli")
    if r2.Level != CompatFull { t.Errorf("expected Full for gemini, got %v", r2.Level) }
}

func TestAnalyzeHookCompat_BrokenMatcher_Copilot(t *testing.T) {
    hook := HookData{
        Event: "PreToolUse", Matcher: "Bash",
        Hooks: []hookEntry{{Type: "command", Command: "go vet ./..."}},
    }
    r := AnalyzeHookCompat(hook, "copilot-cli")
    if r.Level != CompatBroken { t.Errorf("expected Broken, got %v", r.Level) }
    // Should have FeatureMatcher in Features
}

func TestAnalyzeHookCompat_NoneEvent(t *testing.T) {
    hook := HookData{
        Event: "SubagentStart",
        Hooks: []hookEntry{{Type: "command", Command: "echo hi"}},
    }
    for _, target := range []string{"gemini-cli", "copilot-cli", "kiro"} {
        r := AnalyzeHookCompat(hook, target)
        if r.Level != CompatNone {
            t.Errorf("target %s: expected None, got %v", target, r.Level)
        }
    }
}

func TestAnalyzeHookCompat_LLMHook_NoneForNonClaude(t *testing.T) {
    hook := HookData{
        Event: "PreToolUse",
        Hooks: []hookEntry{{Type: "prompt", Command: "Is this safe?"}},
    }
    for _, target := range []string{"gemini-cli", "copilot-cli", "kiro"} {
        r := AnalyzeHookCompat(hook, target)
        if r.Level != CompatNone {
            t.Errorf("target %s: expected None for LLM hook, got %v", target, r.Level)
        }
    }
}

func TestAnalyzeHookCompat_StatusMessage_Kiro_Degraded(t *testing.T) {
    hook := HookData{
        Event: "PreToolUse",
        Hooks: []hookEntry{{Type: "command", Command: "echo hi", StatusMessage: "Working..."}},
    }
    r := AnalyzeHookCompat(hook, "kiro")
    if r.Level != CompatDegraded { t.Errorf("expected Degraded, got %v", r.Level) }
}

func TestAnalyzeHookCompat_Async_Kiro_Broken(t *testing.T) {
    hook := HookData{
        Event: "PostToolUse",
        Hooks: []hookEntry{{Type: "command", Command: "echo done", Async: true}},
    }
    r := AnalyzeHookCompat(hook, "kiro")
    if r.Level != CompatBroken { t.Errorf("expected Broken, got %v", r.Level) }
}
```

**Dependencies:** Task 2.1, Task 1.3 (`HookData` type).

---

### Task 2.3 — Add `LoadHookData` to converter package

**Design sections:** 9 (scanner changes — event/matcher extraction for Compatibility tab and list view)

**What:** Add a function to read a hook item's `HookData` from disk for use by the TUI. Do not store it on `ContentItem` — read it on demand (lazy load).

**Important: Import cycle avoidance.** `converter` imports `catalog` (for `ContentType`, `ContentItem`, etc.), so `catalog` cannot import `converter`. Therefore `LoadHookData` lives in the **converter** package (where `HookData` is defined), not catalog. It takes a `catalog.ContentItem` parameter, which converter already imports.

**Files to modify:**

- `cli/internal/converter/hooks.go` — add `LoadHookData` function

**Add:**

```go
// LoadHookData reads and parses the hook.json from a directory-format hook item.
// Returns an error if the item is not a hook or the file is missing/invalid.
func LoadHookData(item catalog.ContentItem) (HookData, error) {
    if item.Type != catalog.Hooks {
        return HookData{}, fmt.Errorf("item is not a hook")
    }
    hookPath := item.Path
    fi, _ := os.Stat(hookPath)
    if fi != nil && fi.IsDir() {
        hookPath = filepath.Join(hookPath, "hook.json")
    }
    data, err := os.ReadFile(hookPath)
    if err != nil {
        return HookData{}, err
    }
    var hd HookData
    if err := json.Unmarshal(data, &hd); err != nil {
        return HookData{}, err
    }
    return hd, nil
}
```

**Callers use `converter.LoadHookData(item)` instead of `converter.LoadHookData(item)`.**

**Tests:** Add to `cli/internal/converter/hooks_test.go`:

```go
func TestLoadHookData_DirectoryFormat(t *testing.T) {
    dir := t.TempDir()
    hookJSON := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"go vet ./..."}]}`
    os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644)
    item := catalog.ContentItem{Type: catalog.Hooks, Path: dir}

    hd, err := LoadHookData(item)
    if err != nil { t.Fatal(err) }
    if hd.Event != "PreToolUse" { t.Errorf("event: %q", hd.Event) }
    if hd.Matcher != "Bash" { t.Errorf("matcher: %q", hd.Matcher) }
}
```

**Dependencies:** Task 1.3 (`HookData` type defined in converter package).

---

## Phase 3: Snapshot Safety + Hash-Based Uninstall

Fix the two correctness issues in `installer/hooks.go` before building TUI flows on top.

---

### Task 3.1 — Generalize `SnapshotManifest` for hook operations

**Design sections:** 14 (snapshot safety)

**What:** The `SnapshotManifest` has `LoadoutName` and `Mode` fields that are loadout-specific. Generalize to support hook install/uninstall operations without breaking existing loadout usage.

**Files to modify:**

- `cli/internal/snapshot/snapshot.go`

**Change:** Add a `Source` field alongside (not replacing) `LoadoutName` to avoid breaking existing loadout snapshots:

```go
type SnapshotManifest struct {
    Source        string          `json:"source"`        // NEW: e.g., "hook-install:bash-validator", "loadout:my-loadout"
    LoadoutName   string          `json:"loadoutName"`   // kept for backwards compat with loadout snapshots
    Mode          string          `json:"mode"`
    CreatedAt     time.Time       `json:"createdAt"`
    BackedUpFiles []string        `json:"backedUpFiles"`
    Symlinks      []SymlinkRecord `json:"symlinks"`
    HookScripts   []string        `json:"hookScripts,omitempty"`
}
```

The `Create` function signature stays the same — callers pass `loadoutName` which is copied to both `LoadoutName` and `Source`. Add a separate `CreateForHook(projectRoot, source string, filesToBackup []string) (string, error)` helper that sets `Source` and `Mode: "keep"`.

**Backwards-compat migration in `Load()`:** If `Source` is empty but `LoadoutName` is set (old loadout snapshot), populate `Source = "loadout:" + LoadoutName`. This ensures all code paths can check `Source` without special-casing.

```go
// CreateForHook creates a snapshot before a settings.json modification.
// source should be "hook-install:<name>" or "hook-uninstall:<name>".
func CreateForHook(projectRoot, source string, filesToBackup []string) (string, error) {
    return Create(projectRoot, source, "keep", filesToBackup, nil, nil)
}
```

**Tests:** Add to `cli/internal/snapshot/snapshot_test.go`:

```go
func TestCreateForHook(t *testing.T) {
    dir := t.TempDir()
    // Create a fake settings.json to back up
    settingsPath := filepath.Join(dir, "settings.json")
    os.WriteFile(settingsPath, []byte(`{}`), 0644)

    snapshotDir, err := CreateForHook(dir, "hook-install:bash-validator", []string{settingsPath})
    if err != nil { t.Fatal(err) }
    // Verify snapshot dir exists and manifest has Source set
    manifest, _, err := Load(dir)
    if err != nil { t.Fatal(err) }
    if manifest.Source != "hook-install:bash-validator" { t.Errorf("source: %q", manifest.Source) }
    if manifest.Mode != "keep" { t.Errorf("mode: %q", manifest.Mode) }
    _ = snapshotDir
}
```

**Dependencies:** None.

---

### Task 3.2 — Add hash-based matching to `InstalledHook`

**Design sections:** 15 (hash-based uninstall matching)

**What:** Update `InstalledHook` to store a `GroupHash` (SHA256 of the matcher group JSON). Update `installHook` to compute and store it. Update `uninstallHook` to match by hash instead of command string.

**Files to modify:**

- `cli/internal/installer/installed.go` — update `InstalledHook` struct, add `Scope` field
- `cli/internal/installer/hooks.go` — update `installHook` and `uninstallHook`

**Struct changes in `installed.go`:**

```go
type InstalledHook struct {
    Name        string    `json:"name"`
    Event       string    `json:"event"`
    GroupHash   string    `json:"groupHash"`   // NEW: SHA256 of matcher group JSON
    Command     string    `json:"command"`     // kept for display/debugging
    Source      string    `json:"source"`
    Scope       string    `json:"scope"`       // NEW: "global" or "project"
    InstalledAt time.Time `json:"installedAt"`
}
```

**Changes in `hooks.go` — `installHook`:**

Replace `backupFile(settingsPath)` with `snapshot.CreateForHook(...)`.

After appending the matcher group to settings.json, compute the hash:

```go
import (
    "crypto/sha256"
    "encoding/hex"
    "github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// Replace backupFile with snapshot
snapshotDir, err := snapshot.CreateForHook(repoRoot, "hook-install:"+item.Name, []string{settingsPath})
if err != nil {
    return "", fmt.Errorf("creating snapshot: %w", err)
}

// ... (append hook to settings.json) ...

// If write fails, auto-rollback
if err := writeJSONFile(settingsPath, fileData); err != nil {
    manifest, _, loadErr := snapshot.Load(repoRoot)
    if loadErr == nil {
        snapshot.Restore(snapshotDir, manifest)
    }
    return "", fmt.Errorf("writing %s: %w", settingsPath, err)
}

// Compute hash of matcher group for future uninstall matching
hash := sha256.Sum256(matcherGroup)
groupHash := hex.EncodeToString(hash[:])

inst.Hooks = append(inst.Hooks, InstalledHook{
    Name:        item.Name,
    Event:       event,
    GroupHash:   groupHash,
    Command:     command,
    Source:      "export",
    Scope:       "global", // default; scope detection in Task 6.2
    InstalledAt: time.Now(),
})
```

**Changes in `hooks.go` — `uninstallHook`:**

Replace command-string matching with hash-based matching:

```go
// Replace backupFile with snapshot
snapshotDir, err := snapshot.CreateForHook(repoRoot, "hook-uninstall:"+item.Name, []string{settingsPath})

// ... read settings.json ...

found := -1
if instIdx >= 0 && inst.Hooks[instIdx].GroupHash != "" {
    // Hash-based matching
    storedHash := inst.Hooks[instIdx].GroupHash
    for i, entry := range hooksArray.Array() {
        h := sha256.Sum256([]byte(entry.Raw))
        if hex.EncodeToString(h[:]) == storedHash {
            found = i
            break
        }
    }
    if found == -1 {
        return "", fmt.Errorf("hook %s was modified since installation; use 'syllago restore' to revert", item.Name)
    }
} else if instIdx >= 0 {
    // Fallback to command-string matching for pre-hash installed hooks
    cmd := inst.Hooks[instIdx].Command
    for i, entry := range hooksArray.Array() {
        if entry.Get("hooks.0.command").String() == cmd {
            found = i
            break
        }
    }
}

// If delete fails, auto-rollback
if err := writeJSONFile(settingsPath, fileData); err != nil {
    manifest, _, loadErr := snapshot.Load(repoRoot)
    if loadErr == nil {
        snapshot.Restore(snapshotDir, manifest)
    }
    return "", ...
}
```

**Tests:** Add to `cli/internal/installer/hooks_test.go`:

```go
func TestInstallHook_HashRecorded(t *testing.T) {
    // Set up fake settings.json, call installHook
    // Verify installed.json has GroupHash set
}

func TestUninstallHook_HashMatching(t *testing.T) {
    // Install a hook (records hash), then uninstall
    // Verify hook removed from settings.json
}

func TestUninstallHook_ModifiedHookFails(t *testing.T) {
    // Install a hook, manually modify settings.json, attempt uninstall
    // Verify error "was modified since installation"
}
```

**Dependencies:** Task 1.2, Task 3.1.

---

## Phase 4: TUI — Hooks List (Compatibility Matrix)

Replace the single Provider column with a 4-column compatibility matrix when viewing hooks.

---

### Task 4.1 — Add compatibility matrix column logic to `items.go`

**Design sections:** 2 (TUI hooks list), 6 (responsive layout)

**What:** When `contentType == catalog.Hooks`, the `itemsModel` replaces the standard provider column with a 4-column matrix. Add `buildHookMatrixCells` to compute compat level for each hook item against the 4 hook-capable providers.

**Files to modify:**

- `cli/internal/tui/items.go`

**Add type and helper:**

```go
// hookMatrixCell holds the symbol and styled version for one cell of the compat matrix.
type hookMatrixCell struct {
    plain  string // single char: ✓ ~ ! ✗
    styled string // with lipgloss color applied
}

// hookCompatMatrix is the precomputed 4-column matrix for one hook item.
type hookCompatMatrix [4]hookMatrixCell // indexed by HookProviders() order

// matrixProviders is the fixed order for the compatibility matrix.
var matrixProviders = []string{"claude-code", "gemini-cli", "copilot-cli", "kiro"}

// matrixHeadersFull are the full column headers (panel width >= 101).
var matrixHeadersFull = []string{"Claude", "Gemini", "Copilot", "Kiro"}

// matrixHeadersAbbr are the abbreviated column headers (panel width < 101).
var matrixHeadersAbbr = []string{"CC", "GC", "Cp", "Ki"}

// buildHookMatrix computes the compatibility matrix for a hook item.
// Returns empty cells if the hook data cannot be loaded.
func buildHookMatrix(item catalog.ContentItem) hookCompatMatrix {
    hd, err := converter.LoadHookData(item)
    if err != nil {
        return emptyMatrix()
    }
    var m hookCompatMatrix
    for i, slug := range matrixProviders {
        result := converter.AnalyzeHookCompat(hd, slug)
        m[i] = hookMatrixCell{
            plain:  result.Level.Symbol(),
            styled: compatLevelStyle(result.Level).Render(result.Level.Symbol()),
        }
    }
    return m
}
```

The `compatLevelStyle` helper returns the correct lipgloss style for each level. Add these styles to `styles.go`:

```go
// Compatibility level colors
compatFullColor     = successColor   // green ✓
compatDegradedColor = lipgloss.AdaptiveColor{Light: "#CA8A04", Dark: "#FDE68A"} // yellow ~
compatBrokenColor   = warningColor   // orange/amber !
compatNoneColor     = dangerColor    // red ✗
```

```go
// In tui package (items.go or compat_styles.go):
func compatLevelStyle(level converter.CompatLevel) lipgloss.Style {
    switch level {
    case converter.CompatFull:     return lipgloss.NewStyle().Foreground(compatFullColor)
    case converter.CompatDegraded: return lipgloss.NewStyle().Foreground(compatDegradedColor)
    case converter.CompatBroken:   return lipgloss.NewStyle().Foreground(compatBrokenColor)
    case converter.CompatNone:     return lipgloss.NewStyle().Foreground(compatNoneColor)
    }
    return lipgloss.NewStyle()
}
```

**Width calculation (in `View()`):**

When `contentType == catalog.Hooks`:
```
panelWidth = m.width (or m.termWidth())
useFullHeaders = panelWidth >= 101
matrixHeaders = matrixHeadersFull or matrixHeadersAbbr
matrixColW = max(len(header), 1) for each column
matrixW = sum(matrixColW) + 3*1  // 4 cols × colW + 3 single-space gaps
nameW = m.maxNameLen()
descW = panelWidth - cursorWidth - nameW - colGap - matrixW - colGap
if descW < 10 { descW = 0 } // hide description on narrow panels
```

**In `View()`:** When `contentType == catalog.Hooks`, precompute matrices for all items, build a hook-specific header and separator, and render hook rows using the matrix cells instead of the provider cell.

**Tests:** Add golden file tests:
- `testdata/items-hooks-120x40.golden` — full headers, with description
- `testdata/items-hooks-80x30.golden` — abbreviated headers, with description
- `testdata/items-hooks-60x20.golden` — abbreviated headers, no description

**Dependencies:** Task 2.2 (`AnalyzeHookCompat`), Task 2.3 (`LoadHookData`).

---

## Phase 5: TUI — Hook Detail View

Add the 4th Compatibility tab and hook-specific UI to the detail view.

---

### Task 5.1 — Add `tabCompatibility` and hook metadata to `detailModel`

**Design sections:** 7 (TUI hook detail — 4-tab layout, overview tab)

**What:** Hooks use 4 tabs. Add `tabCompatibility` as the new second tab (shifting Files to 3rd, Install to 4th). Update `detailModel` to store hook-specific data, and update `renderTabBar()` to show 4 tabs for hooks.

**Files to modify:**

- `cli/internal/tui/detail.go`

**Changes:**

Add tab constant:
```go
const (
    tabOverview       detailTab = iota
    tabCompatibility            // new: hooks only
    tabFiles
    tabInstall
)
```

Add fields to `detailModel`:
```go
type detailModel struct {
    // ... existing fields ...
    hookData       *converter.HookData // loaded for hook items (nil for all others)
    hookCompat     []converter.CompatResult // computed for all 4 providers
}
```

Update `newDetailModel`:
```go
if item.Type == catalog.Hooks {
    hd, err := converter.LoadHookData(item)
    if err == nil {
        m.hookData = &hd
        for _, slug := range converter.HookProviders() {
            m.hookCompat = append(m.hookCompat, converter.AnalyzeHookCompat(hd, slug))
        }
    }
}
```

Update tab switching in `Update()` — when the item is a hook, tab cycles over 4 tabs and `%3` becomes `%4`. The `1/2/3/4` shortcuts map to `tabOverview/tabCompatibility/tabFiles/tabInstall`.

```go
// Tab switching logic update:
tabCount := 3
if m.item.Type == catalog.Hooks {
    tabCount = 4
}
switch msg.String() {
case "tab":
    newTab = detailTab((int(m.activeTab) + 1) % tabCount)
case "shift+tab":
    newTab = detailTab((int(m.activeTab) + tabCount - 1) % tabCount)
case "1":
    newTab = tabOverview
case "2":
    if m.item.Type == catalog.Hooks {
        newTab = tabCompatibility
    } else {
        newTab = tabFiles
    }
case "3":
    if m.item.Type == catalog.Hooks {
        newTab = tabFiles
    } else {
        newTab = tabInstall
    }
case "4":
    if m.item.Type == catalog.Hooks {
        newTab = tabInstall
    }
}
```

**Dependencies:** Task 2.2, Task 2.3.

---

### Task 5.2 — Render hook Overview tab with hook metadata

**Design sections:** 7 (Overview tab)

**What:** Add hook-specific metadata block at the top of the Overview tab content — Event, Matcher, Type, Command, Timeout, Async — before risks and README.

**Files to modify:**

- `cli/internal/tui/detail_render.go`

**In `renderOverviewTab()`:** Prepend hook metadata when `m.item.Type == catalog.Hooks && m.hookData != nil`:

```go
if m.item.Type == catalog.Hooks && m.hookData != nil {
    hd := m.hookData
    body += labelStyle.Render("Event:   ") + valueStyle.Render(hd.Event) + "\n"
    if hd.Matcher != "" {
        body += labelStyle.Render("Matcher: ") + valueStyle.Render(hd.Matcher) + "\n"
    }
    if len(hd.Hooks) > 0 {
        h := hd.Hooks[0]
        body += labelStyle.Render("Type:    ") + valueStyle.Render(h.Type) + "\n"
        if h.Command != "" {
            body += labelStyle.Render("Command: ") + valueStyle.Render(truncate(h.Command, m.width-12)) + "\n"
        }
        if h.Timeout > 0 {
            body += labelStyle.Render("Timeout: ") + valueStyle.Render(fmt.Sprintf("%dms", h.Timeout)) + "\n"
        }
        body += labelStyle.Render("Async:   ") + valueStyle.Render(fmt.Sprintf("%v", h.Async)) + "\n"
    }
    body += "\n"
}
```

**Dependencies:** Task 5.1.

---

### Task 5.3 — Render Compatibility tab

**Design sections:** 7 (Compatibility tab)

**What:** Add `renderCompatibilityTab()` to `detail_render.go`. Shows a provider-vs-feature table with colored symbols.

**Files to modify:**

- `cli/internal/tui/detail_render.go`

**In `renderContentSplit()`:**

```go
case tabCompatibility:
    body = m.renderCompatibilityTab()
```

**Add method:**

```go
func (m detailModel) renderCompatibilityTab() string {
    if m.item.Type != catalog.Hooks || m.hookData == nil {
        return helpStyle.Render("Compatibility not available for this item.") + "\n"
    }

    var s string
    s += labelStyle.Render("Compatibility") + "\n\n"

    // Provider summary table
    providerW := 14
    levelW := 10
    s += tableHeaderStyle.Render(fmt.Sprintf("  %-*s  %-*s  %s", providerW, "Provider", levelW, "Level", "Notes")) + "\n"
    s += helpStyle.Render("  " + strings.Repeat("─", providerW) + "  " + strings.Repeat("─", levelW) + "  " + strings.Repeat("─", 36)) + "\n"

    providerNames := map[string]string{
        "claude-code": "Claude Code",
        "gemini-cli":  "Gemini CLI",
        "copilot-cli": "Copilot CLI",
        "kiro":        "Kiro",
    }
    for i, result := range m.hookCompat {
        slug := converter.HookProviders()[i]
        sym := compatLevelStyle(result.Level).Render(result.Level.Symbol())
        level := sym + " " + result.Level.Label()
        note := result.Notes
        if note == "" && result.Level == converter.CompatFull {
            if slug == "claude-code" {
                note = "Native format"
            }
        }
        s += fmt.Sprintf("  %-*s  %-*s  %s\n", providerW, providerNames[slug], levelW, level, note)
    }

    // Feature impact table (only if there are interesting features)
    if m.hookData != nil && hasInterestingFeatures(m.hookData) {
        s += "\n" + labelStyle.Render("Feature Impact") + "\n"
        s += helpStyle.Render("  " + strings.Repeat("─", 50)) + "\n"
        // ... render per-feature matrix ...
    }

    // Specific warnings
    for _, result := range m.hookCompat {
        for _, fr := range result.Features {
            if !fr.Supported && fr.Present && fr.Impact >= converter.CompatBroken {
                slug := result.Provider
                s += "\n" + compatLevelStyle(fr.Impact).Render("  ! "+providerNames[slug]+": "+fr.Notes)
            }
        }
    }

    return s
}
```

**Dependencies:** Task 5.1.

---

### Task 5.4 — Update Install tab with compatibility level display and Broken warning modal

**Design sections:** 7 (Install tab), 12 (export/install integration)

**What:** In the Install tab provider checkboxes, show the compatibility level inline next to each provider name. Disable checkboxes for `CompatNone` providers. Intercept the install action for `CompatBroken` providers and open a warning modal before proceeding.

**Files to modify:**

- `cli/internal/tui/detail_render.go` — update `renderInstallTab()` for hooks
- `cli/internal/tui/detail.go` — intercept install for Broken hooks
- `cli/internal/tui/modal.go` — add `modalHookBrokenWarning` purpose
- `cli/internal/tui/styles.go` — add `compatDegradedColor` if not already added in Task 4.1

**In `renderInstallTab()`**, when `m.item.Type == catalog.Hooks`:

For each detected provider checkbox line, if `m.hookCompat` is populated, append the compat level after the checkbox:

```
  > [✓] Claude Code   ✓ Full        [ok] installed
    [ ] Gemini CLI     ✓ Full        [--] available
    [ ] Copilot CLI    ! Broken      [--] available
        Kiro           ✓ Full        (not detected)
```

Non-detected hook-capable providers (in `matrixProviders` but not in `detectedProviders()`) are shown without a checkbox, grayed out with `(not detected)`.

`CompatNone` providers: replace checkbox with `[✗]` rendered in dangerColor, cursor skips over them.

**In `startInstall()` / `detailModel` hook handling:**

When installing to a `CompatBroken` provider, instead of proceeding directly, emit `openModalMsg{purpose: modalHookBrokenWarning, ...}` with the broken feature details.

Add a new modal message type in `detail.go` and render it in `modal.go`:

```go
// In detail.go:
type openHookBrokenWarningMsg struct {
    provider string
    features []converter.FeatureResult
}
```

When the user toggles install on a `CompatBroken` provider, emit this message. In `App.Update()`, route it to show a warning modal. The modal content comes from the `FeatureResult` slice (the exact issues). Standard 40-char modal width, Cancel as default button (destructive action pattern).

**Tests:** Update golden files for the Install tab of a hooks item. Create `testdata/detail-hooks-install-*.golden`.

**Dependencies:** Task 5.1, Task 5.2, Task 5.3.

---

## Phase 6: Import Flow

Split settings.json during import, add the hook selection wizard, extend CLI flags, and add scope detection.

---

### Task 6.1 — Add `SplitSettingsHooks` to converter

**Design sections:** 3 (import splitting), 10 (TUI import wizard), 11 (CLI import)

**What:** Add a function that reads a `settings.json` file (or its hooks section) and splits it into individual `HookData` items, one per event+matcher group. This is the core of the import operation.

**Files to create:**

- `cli/internal/converter/split.go`

```go
package converter

// SplitSettingsHooks reads the hooks section of a settings.json-style file
// and returns one HookData per event+matcher group.
// sourceProvider is used to reverse-translate event and tool names to canonical.
func SplitSettingsHooks(content []byte, sourceProvider string) ([]HookData, error)
```

**Algorithm:**

1. Parse JSON, get `hooks` object (nested format: `{"hooks":{"EventName":[...]}}`)
2. For each event key, reverse-translate to canonical event name
3. For each matcher group under that event:
   - Create a `HookData{Event: canonicalEvent, Matcher: matcher, Hooks: entries}`
   - Reverse-translate matcher tool name if `sourceProvider != "claude-code"`
4. Return all `HookData` items

**Also add name derivation:**

```go
// DeriveHookName generates a filesystem-safe name from a HookData item.
// Priority: statusMessage → slugify, matcher+event, event+command words.
func DeriveHookName(hook HookData) string
```

**Slugify helper:**

```go
// slugify lowercases and replaces non-alphanumeric with hyphens, trims trailing hyphens.
func slugify(s string) string
```

**Tests:** `cli/internal/converter/split_test.go`

```go
func TestSplitSettingsHooks_KitchenSink(t *testing.T) {
    input := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[...]}],"PostToolUse":[...]}}`
    items, err := SplitSettingsHooks([]byte(input), "claude-code")
    if len(items) != expectedCount { ... }
}

func TestSplitSettingsHooks_CopilotFormat(t *testing.T) {
    // Copilot has flat {"hooks":{"preToolUse":[{"bash":"..."}]}}
    // Verify event names reverse-translated, no matcher
}

func TestDeriveHookName_StatusMessage(t *testing.T) {
    hook := HookData{..., Hooks: []hookEntry{{StatusMessage: "Validating shell command..."}}}
    if DeriveHookName(hook) != "validating-shell-command" { ... }
}

func TestDeriveHookName_MatcherEvent(t *testing.T) {
    hook := HookData{Event: "PreToolUse", Matcher: "Bash", Hooks: []hookEntry{{Command: "go vet"}}}
    if DeriveHookName(hook) != "pretooluse-bash" { ... }
}

func TestDeriveHookName_EventCommand(t *testing.T) {
    hook := HookData{Event: "SessionStart", Hooks: []hookEntry{{Command: "echo starting session"}}}
    // "sessionstart-echo" (event + first command word)
}
```

**Dependencies:** Task 1.3 (`HookData`).

---

### Task 6.2 — Add scope detection for settings.json

**Design sections:** 16 (scope detection — global vs project)

**What:** Add a function that finds the settings.json path(s) for a given provider, checking both global (`~/<ConfigDir>/settings.json`) and project (nearest git root `/<ConfigDir>/settings.json`) scopes.

**Files to create:**

- `cli/internal/installer/scope.go`

```go
package installer

// SettingsScope represents where a settings.json lives.
type SettingsScope int

const (
    ScopeGlobal  SettingsScope = iota
    ScopeProject
)

func (s SettingsScope) String() string {
    if s == ScopeGlobal { return "global" }
    return "project"
}

// SettingsLocation describes one discovered settings.json file.
type SettingsLocation struct {
    Scope SettingsScope
    Path  string
}

// FindSettingsLocations returns all settings.json paths for the given provider
// that exist on disk, checking global and project-local scopes.
// projectRoot is the nearest git root (or cwd if not in a git repo).
func FindSettingsLocations(prov provider.Provider, projectRoot string) ([]SettingsLocation, error)
```

**Implementation:**

```go
func FindSettingsLocations(prov provider.Provider, projectRoot string) ([]SettingsLocation, error) {
    var locations []SettingsLocation

    // Global scope: ~/.configDir/settings.json
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, err
    }
    globalPath := filepath.Join(home, prov.ConfigDir, "settings.json")
    if _, err := os.Stat(globalPath); err == nil {
        locations = append(locations, SettingsLocation{Scope: ScopeGlobal, Path: globalPath})
    }

    // Project scope: <projectRoot>/.configDir/settings.json (if projectRoot != home)
    if projectRoot != "" && projectRoot != home {
        projectPath := filepath.Join(projectRoot, prov.ConfigDir, "settings.json")
        if _, err := os.Stat(projectPath); err == nil {
            locations = append(locations, SettingsLocation{Scope: ScopeProject, Path: projectPath})
        }
    }

    return locations, nil
}
```

Also update `hookSettingsPath` in `hooks.go` to use scope from `InstalledHook.Scope`:

```go
// hookSettingsPathForScope returns the settings.json path for a given scope.
func hookSettingsPathForScope(prov provider.Provider, scope SettingsScope, projectRoot string) (string, error) {
    switch scope {
    case ScopeProject:
        if projectRoot == "" {
            return "", fmt.Errorf("no project root for project-scoped install")
        }
        return filepath.Join(projectRoot, prov.ConfigDir, "settings.json"), nil
    default: // ScopeGlobal
        home, err := os.UserHomeDir()
        if err != nil {
            return "", err
        }
        return filepath.Join(home, prov.ConfigDir, "settings.json"), nil
    }
}
```

**Tests:** `cli/internal/installer/scope_test.go`

```go
func TestFindSettingsLocations_GlobalOnly(t *testing.T) {
    // Set up fake home with settings.json, no project settings
    // Verify only global returned
}

func TestFindSettingsLocations_BothScopes(t *testing.T) {
    // Set up fake home + project directory both with settings.json
    // Verify both returned in order
}
```

**Dependencies:** Task 3.2 (`InstalledHook.Scope` field).

---

### Task 6.3 — TUI import wizard: hook-splitting step

**Design sections:** 10 (TUI import wizard flow)

**What:** Add `stepHookSelect` to the import wizard. After the user selects a settings.json (or provider directory containing one), the wizard detects hooks, splits them via `SplitSettingsHooks`, shows the multi-select screen, and imports selected items as individual directories.

**Files to modify:**

- `cli/internal/tui/import.go`

**Add step:**

```go
const (
    // ... existing steps ...
    stepHookSelect  // (hook import only) multi-select which hooks to import
)
```

**Add fields to `importModel`:**

```go
// Hook import state
hookCandidates   []converter.HookData // from SplitSettingsHooks
hookNames        []string             // auto-derived names, one per candidate
hookSelected     []bool               // selection state
hookSelectCursor int
```

**Flow:**

When importing hooks (`contentType == catalog.Hooks`):
1. After source is selected (`stepBrowse` or `stepConfirm` completes with a `.json` file):
   - Call `converter.SplitSettingsHooks(content, sourceProvider)`
   - Derive names: `converter.DeriveHookName(hook)` for each
   - Populate `hookCandidates`, `hookNames`, `hookSelected` (all `true` initially)
   - Transition to `stepHookSelect`

2. `stepHookSelect` rendering:
```
Found 4 hooks in settings.json:

  [✓] validating-shell-command   (PreToolUse/Bash)
  [✓] checking-file-permissions  (PreToolUse/Write)
  [✓] post-tool-cleanup          (PostToolUse/*)
  [ ] session-start-echo         (SessionStart/*)

  ▸ Import Selected (3)        Deselect All
```
   - Space: toggle item
   - `a`: select all
   - `n`: deselect all
   - Enter: import selected

3. On confirm: for each selected hook, write it as a directory:
   ```
   <repoRoot>/hooks/<providerName>/<hookName>/
     hook.json         # flat format HookData JSON
     .syllago.yaml     # with description from DeriveHookName
   ```

**Add `importHookItems` function:**

```go
// importHookItems writes selected hook candidates to the content directory.
// Returns a list of imported names and any errors.
func importHookItems(
    candidates []converter.HookData,
    names []string,
    selected []bool,
    providerName string,
    repoRoot string,
) (imported []string, errs []string)
```

Each hook is written as:
```go
itemDir := filepath.Join(repoRoot, "hooks", providerName, names[i])
os.MkdirAll(itemDir, 0755)
hookJSON, _ := json.MarshalIndent(candidates[i], "", "  ")
os.WriteFile(filepath.Join(itemDir, "hook.json"), hookJSON, 0644)
// Write minimal .syllago.yaml
```

**Tests:** Unit test `importHookItems` with temp directory. Golden file tests for `stepHookSelect` rendering.

**Dependencies:** Task 6.1 (`SplitSettingsHooks`, `DeriveHookName`), Task 1.3 (`HookData`).

---

### Task 6.4 — CLI `import` command: hooks flags and batch behavior

**Design sections:** 11 (CLI import flow)

**What:** Update the `syllago import --from <provider> --type hooks` command to use the new splitting logic. Add `--preview`, `--exclude`, and `--force` flags.

**Files to locate and modify:**

Find the CLI import command file:

```
cli/cmd/syllago/          # likely location for cobra commands
```

Look for the import command handler and update it for hooks:

**Flags to add:**

```go
importCmd.Flags().BoolVar(&importPreview, "preview", false, "Show what would be imported without writing")
importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Alias for --preview")
importCmd.Flags().StringArrayVar(&importExclude, "exclude", nil, "Skip hooks by auto-derived name")
importCmd.Flags().BoolVar(&importForce, "force", false, "Overwrite existing items with the same name")
importCmd.Flags().StringVar(&importScope, "scope", "global", "Settings scope: global, project, or all")
```

**Behavior for hooks:**

```go
case catalog.Hooks:
    // Find settings.json for provider
    locations, err := installer.FindSettingsLocations(prov, projectRoot)
    // Filter by --scope
    // For each location, read and split
    candidates, _ := converter.SplitSettingsHooks(settingsData, fromProvider)
    // Apply --exclude filter
    // If --preview: print list and exit
    // Else: import each (--force overwrites)
    for _, hook := range candidates {
        name := converter.DeriveHookName(hook)
        // Write to hooks/<provider>/<name>/
        fmt.Printf("  %s   (%s/%s)\n", name, hook.Event, orStar(hook.Matcher))
    }
    fmt.Printf("\nImported %d hooks to local/hooks/%s/\n", count, fromProvider)
```

**Tests:** CLI integration test with a temp repo containing a fake settings.json. Verify correct directories created, `--preview` doesn't write, `--exclude` skips named items.

**Dependencies:** Task 6.1, Task 6.2.

---

### Task 5.5 — Update `renderTabBar` and help text for 4-tab hooks

**Design sections:** 7 (tab bar)

**What:** `renderTabBar()` currently always renders 3 tabs. Update it to render 4 tabs for hooks. Update `renderHelp()` for the detail model to include the Compatibility tab in its key hints.

**Files to modify:**

- `cli/internal/tui/detail_render.go`

**Change `renderTabBar()`:**

```go
func (m detailModel) renderTabBar() string {
    if m.item.Type == catalog.Hooks {
        tabs := []struct{ label string; tab detailTab }{
            {"Overview", tabOverview},
            {"Compatibility", tabCompatibility},
            {"Files", tabFiles},
            {"Install", tabInstall},
        }
        // render with same existing tab styling
        // active tab uses accentColor, inactive uses mutedColor
        // zone marks: "tab-0", "tab-1", "tab-2", "tab-3"
    } else {
        // existing 3-tab rendering
    }
}
```

Update key hint text when on a hooks item to include `2 compat`.

**After all visual changes:** Regenerate golden files:
```bash
cd cli && go test ./internal/tui/ -update-golden
```

**Dependencies:** Task 5.1, Task 5.2, Task 5.3, Task 5.4.

---

## Phase Summary

| Phase | Tasks | Produces |
|-------|-------|---------|
| 1. Foundation | 1.1–1.3 | Correct storage format, working installer, flat converter |
| 2. Compat Engine | 2.1–2.3 | `HookCapabilities`, `AnalyzeHookCompat`, `LoadHookData` |
| 3. Safety | 3.1–3.2 | Snapshot-backed installs, hash-based uninstall |
| 4. TUI List | 4.1 | Compatibility matrix in hooks item list |
| 5. TUI Detail | 5.1–5.5 | 4-tab detail view with Overview metadata, Compatibility tab, Install warnings, tab bar |
| 6. Import | 6.1–6.4 | Settings splitting, name derivation, TUI wizard step, CLI flags, scope detection |

---

## Dependency Graph

```
1.1 ──► 1.2
1.1 ──► 1.3 ──► 2.1 ──► 2.2 ──► 4.1
                  │               │
                2.3 ◄─────────────┘
                  │
                 5.1 ──► 5.2
                  │      ├──► 5.3
                  │      └──► 5.4 ──► 5.5
3.1 ──► 3.2
1.3 ──► 6.1 ──► 6.3
3.2 ──► 6.2 ──► 6.4
              └──► 6.3
```

---

## Notes on Implementation Order

Start with Phase 1 tasks in order (1.1 → 1.2 → 1.3) before anything else. The migration is mechanical but critical — all subsequent tests depend on the flat format being correct.

Phase 2 can proceed immediately after 1.3 is done. Phases 3 and 2 are independent of each other and can proceed in parallel if desired.

Phase 4 requires all of Phase 2. Phase 5 requires Phase 2 and can start as soon as 2.2 and 2.3 are done (5.1 → 5.2 → 5.3 → 5.4 → 5.5 in order). Task 5.5 (tab bar update) is the final TUI detail polish — do it after all Phase 5 visual work.

Phase 6 requires 1.3 (for `HookData`) and 6.1 (for splitting). Tasks 6.2 and 6.4 are sequential within the phase but independent of Phase 5.

After every phase, run `make test`. After every TUI visual change, update golden files.
