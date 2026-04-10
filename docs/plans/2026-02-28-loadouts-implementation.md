# Loadouts Implementation Plan

**Date:** 2026-02-28
**Design doc:** `docs/syllago-loadouts-design.md`
**Status:** Ready to execute

---

## Overview

This plan implements the Syllago Loadouts feature end-to-end. Tasks are organized into sequential phases. Each phase depends on the previous one completing. Within a phase, tasks with no stated dependency can be worked in parallel.

**Module path:** `github.com/OpenScribbler/syllago/cli`

**Key file conventions observed in the codebase:**
- Go packages are flat under `cli/internal/<package>/`
- Cobra commands live in `cli/cmd/syllago/` as `<name>.go` with a matching `<name>_test.go`
- Bubbletea models are structs with `Update(tea.Msg) (T, tea.Cmd)` and `View() string` methods
- JSON files use `github.com/tidwall/gjson` for reads and `github.com/tidwall/sjson` for writes
- Atomic JSON writes use `writeJSONFile` from `installer/jsonmerge.go`
- YAML parsing uses `gopkg.in/yaml.v3`

---

## Phase A: Installer Refactoring — Remove `_syllago` Markers

**Goal:** Eliminate all `_syllago` fields from provider config files. Move install tracking to `.syllago/installed.json`. This is a hard prerequisite for every phase that follows.

---

### A1 — Define `installed.json` data structures

**Files modified:**
- `cli/internal/installer/installed.go` (new file)

**What it does:**
Creates the Go structs and read/write helpers for `.syllago/installed.json`. No business logic — just types and I/O.

```go
package installer

// InstalledHook records a hook placed into settings.json by syllago.
type InstalledHook struct {
    Name        string    `json:"name"`
    Event       string    `json:"event"`
    Command     string    `json:"command"`
    Source      string    `json:"source"`      // "export" or "loadout:<name>"
    InstalledAt time.Time `json:"installedAt"`
}

// InstalledMCP records an MCP server placed into .claude.json by syllago.
type InstalledMCP struct {
    Name        string    `json:"name"`
    Source      string    `json:"source"`
    InstalledAt time.Time `json:"installedAt"`
}

// InstalledSymlink records a symlink placed by syllago.
type InstalledSymlink struct {
    Path        string    `json:"path"`   // absolute path of the symlink
    Target      string    `json:"target"` // absolute path it points to
    Source      string    `json:"source"`
    InstalledAt time.Time `json:"installedAt"`
}

// Installed is the root structure for .syllago/installed.json.
type Installed struct {
    Hooks    []InstalledHook    `json:"hooks,omitempty"`
    MCP      []InstalledMCP     `json:"mcp,omitempty"`
    Symlinks []InstalledSymlink `json:"symlinks,omitempty"`
}
```

Helper functions in the same file:
- `LoadInstalled(projectRoot string) (*Installed, error)` — reads `.syllago/installed.json`, returns empty `&Installed{}` if file does not exist (mirrors `config.Load` pattern)
- `SaveInstalled(projectRoot string, inst *Installed) error` — writes `.syllago/installed.json` atomically using the same temp-then-rename pattern from `config.Save`

**Path:** `.syllago/installed.json` (inside the existing `.syllago/` directory managed by `config.go`)

**Dependencies:** none

**Test expectations:**
- `TestLoadInstalled_MissingFile` — returns empty `Installed`, no error
- `TestLoadInstalled_MalformedJSON` — returns error
- `TestSaveInstalled_RoundTrip` — save then load produces identical struct

---

### A2 — Refactor `hooks.go`: remove `_syllago` marker from `installHook`

**Files modified:**
- `cli/internal/installer/hooks.go`

**What it does:**
In `installHook`:
1. Remove the `sjson.SetBytes(matcherGroup, "_syllago", item.Name)` call (line 54).
2. Remove the `entry.Get("_syllago").String() == item.Name` duplicate-check (line 77). Replace with a lookup in `installed.json`: call `LoadInstalled(repoRoot)` and check whether `inst.Hooks` already contains an entry with matching `Name` and `Event`.
3. After successfully writing `settings.json`, append a new `InstalledHook` to the loaded `Installed` and call `SaveInstalled(repoRoot, inst)`.
4. The `installHook` signature gains `repoRoot string` usage (it already receives it via the `_ string` parameter — rename the blank identifier to `repoRoot`).

**Hook command field for tracking:** extract `command` from the first `hooks[0].command` in the matcher group JSON for storage in `InstalledHook.Command`. Use `gjson.GetBytes(matcherGroup, "hooks.0.command").String()`.

**Dependencies:** A1

**Test expectations:**
- Existing `TestInstallHook_*` tests updated to verify no `_syllago` field in output `settings.json`
- New assertion: after install, `.syllago/installed.json` contains the hook entry
- Duplicate-install check still works (hook already in installed.json → error returned)

---

### A3 — Refactor `hooks.go`: remove `_syllago` marker from `uninstallHook`

**Files modified:**
- `cli/internal/installer/hooks.go`

**What it does:**
In `uninstallHook`, replace the `entry.Get("_syllago").String() == item.Name` check (line 122):
1. Load `installed.json` and find the entry by `Name` and `Event`.
2. If no entry exists in `installed.json`, fall back to matching by command string equality (for hooks installed before this refactor — temporary forward compatibility during the transition window).
3. After deleting from `settings.json`, remove the matching entry from `inst.Hooks` and save `installed.json`.

**Dependencies:** A2

---

### A4 — Refactor `hooks.go`: remove `_syllago` marker from `checkHookStatus`

**Files modified:**
- `cli/internal/installer/hooks.go`

**What it does:**
In `checkHookStatus`, replace the `entry.Get("_syllago").String() == item.Name` check (line 172):
1. Load `installed.json`.
2. Return `StatusInstalled` if any entry in `inst.Hooks` has matching `Name` and `Event`.
3. If not in `installed.json` but event array exists in `settings.json`, return `StatusInstalled` still (same behavior as before — installed by other means).

**Dependencies:** A3

---

### A5 — Refactor `mcp.go`: remove `_syllago` marker from `installMCP`

**Files modified:**
- `cli/internal/installer/mcp.go`

**What it does:**
In `installMCP`:
1. Remove `sjson.SetBytes(configData, "_syllago", true)` (line 121).
2. After writing the MCP config file, append a new `InstalledMCP` to `installed.json`.

The `sjson` import may still be needed for other uses — check before removing the import. The `gjson` import from the marker read in `uninstallMCP` will also change.

**Dependencies:** A1

**Test expectations:**
- `TestInstallMCP_WhitelistsFields` (in `mcp_test.go` line 107): change assertion from `serverConfig.Get("_syllago").Bool()` to verifying that `.syllago/installed.json` contains the MCP entry. The test currently passes `tmpDir` as `repoRoot` so the installed.json will land at `tmpDir/.syllago/installed.json`.

---

### A6 — Refactor `mcp.go`: remove `_syllago` marker from `uninstallMCP` and `checkMCPStatus`

**Files modified:**
- `cli/internal/installer/mcp.go`

**What it does:**
In `uninstallMCP` (line 175):
1. Replace `gjson.GetBytes(fileData, key+"._syllago").Bool()` ownership check with a lookup in `installed.json`.
2. Only allow uninstall if entry exists in `installed.json` (same semantics as before — prevents removing manually-added entries).
3. After deleting from the MCP config file, remove the entry from `inst.MCP` and save.

In `checkMCPStatus` (line 212):
1. Replace `gjson.GetBytes(fileData, key+"._syllago").Bool()` check with `installed.json` lookup.
2. Entry exists in config + in `installed.json` → `StatusInstalled` (syllago-owned).
3. Entry exists in config but not in `installed.json` → `StatusInstalled` (manually added, still report installed).

**Dependencies:** A5

---

### A7 — Update `installer_test.go` and `mcp_test.go`

**Files modified:**
- `cli/internal/installer/mcp_test.go`
- `cli/internal/installer/installed_test.go` (new file, created from A1)

**What it does:**
- In `mcp_test.go` line 107: replace `serverConfig.Get("_syllago").Bool()` with code that reads `tmpDir/.syllago/installed.json` and verifies the MCP entry is present.
- Add integration-style test `TestInstallUninstallMCP_RoundTrip` that installs, verifies `installed.json` entry, uninstalls, verifies `installed.json` entry removed.
- Confirm all existing tests pass with `make test`.

**Dependencies:** A5, A6

---

## Phase B: Content Type Infrastructure

**Goal:** Add `loadouts` as the 9th content type. Update the catalog scanner to discover loadout manifests. Define the `loadout.yaml` parsing package.

---

### B1 — Add `Loadouts` content type to catalog

**Files modified:**
- `cli/internal/catalog/types.go`

**What it does:**
1. Add `Loadouts ContentType = "loadouts"` constant after `Commands`.
2. Add `Loadouts` to the `AllContentTypes()` slice (after `Commands`, before `SearchResults`).
3. Add `"Loadouts"` to `Label()` switch.
4. `IsUniversal()` returns `false` for `Loadouts` — loadouts are provider-specific (same pattern as `Rules`, `Hooks`, `Commands`).

**Why not universal:** Loadout manifests are provider-scoped (`content/loadouts/claude-code/`), same as rules and hooks. The scanner uses `scanProviderSpecific` for non-universal types.

**Dependencies:** none

**Test expectations:**
- `TestAllContentTypes` in `types_test.go` — update expected count/slice to include `Loadouts`
- `TestIsUniversal_Loadouts` — verify `Loadouts.IsUniversal()` returns false
- `TestLabel_Loadouts` — verify `Loadouts.Label()` returns `"Loadouts"`

---

### B2 — Add `loadout.yaml` parser package

**Files modified:**
- `cli/internal/loadout/manifest.go` (new package)
- `cli/internal/loadout/manifest_test.go` (new file)

**What it does:**
Creates the `loadout` package with the manifest struct and parser:

```go
package loadout

// Manifest represents a parsed loadout.yaml file.
type Manifest struct {
    Kind        string   `yaml:"kind"`        // must be "loadout"
    Version     int      `yaml:"version"`     // must be 1
    Provider    string   `yaml:"provider"`    // e.g. "claude-code"
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    Rules       []string `yaml:"rules,omitempty"`
    Hooks       []string `yaml:"hooks,omitempty"`
    Skills      []string `yaml:"skills,omitempty"`
    Agents      []string `yaml:"agents,omitempty"`
    MCP         []string `yaml:"mcp,omitempty"`
    Commands    []string `yaml:"commands,omitempty"`
    Prompts     []string `yaml:"prompts,omitempty"`
    Apps        []string `yaml:"apps,omitempty"`
}

// Parse reads and validates a loadout.yaml file.
// Returns an error if kind != "loadout", version != 1, provider is empty, or name is empty.
func Parse(path string) (*Manifest, error)

// ItemCount returns the total number of referenced items across all sections.
func (m *Manifest) ItemCount() int

// RefsByType returns a map of ContentType -> []name for all non-empty sections.
// Uses catalog.ContentType as key for direct use with the resolver.
func (m *Manifest) RefsByType() map[catalog.ContentType][]string
```

**Dependencies:** B1 (needs `catalog.ContentType` constants)

**Test expectations:**
- `TestParse_Valid` — parses full manifest, all fields populated correctly
- `TestParse_MissingKind` — returns error
- `TestParse_WrongKind` — "rules" instead of "loadout" → error
- `TestParse_MissingProvider` — returns error
- `TestParse_EmptySections` — manifest with no items parses without error
- `TestItemCount` — manifest with 3 rules + 2 hooks → ItemCount() == 5
- Golden file: `testdata/valid-loadout.yaml` input, `testdata/valid-loadout-expected.json` output

---

### B3 — Update catalog scanner to discover loadouts

**Files modified:**
- `cli/internal/catalog/scanner.go`

**What it does:**
The `scanRoot` function already calls `scanProviderSpecific` for non-universal types. Since `Loadouts` is in `AllContentTypes()` after B1, the scanner loop will naturally attempt to scan `content/loadouts/`. However, `scanProviderDir` needs a case for `Loadouts` to find the description from `loadout.yaml`.

In `scanProviderDir`'s switch statement, add:
```go
case Loadouts:
    // Load loadout.yaml for description
    manifest, err := loadout.Parse(filepath.Join(itemDir, "loadout.yaml"))
    if err == nil {
        item.Description = manifest.Description
    }
```

Also update `describeHookJSON` is not relevant here — `loadout.yaml` is YAML, not JSON. The description comes from `manifest.Description` after parsing.

Import `github.com/OpenScribbler/syllago/cli/internal/loadout` in scanner.go.

**Dependencies:** B1, B2

**Test expectations:**
- `TestScanRoot_LoadoutsDiscovered` — create a temp directory with `loadouts/claude-code/test-loadout/loadout.yaml` + `.syllago.yaml`, run scanner, verify item appears in catalog with correct type, provider, and description
- Existing scanner tests still pass

---

### B4 — Add provider support for loadouts in `claude.go`

**Files modified:**
- `cli/internal/provider/claude.go`

**What it does:**
Loadouts are not installed to a filesystem directory — they are orchestrated. The provider doesn't need an `InstallDir` for loadouts (the loadout engine handles apply). However, `SupportsType` needs to return true so that:
1. The TUI shows a check on the Install tab
2. Validation logic can confirm Claude Code supports loadouts

Update `SupportsType`:
```go
case catalog.Loadouts:
    return true
```

`InstallDir` for `Loadouts` should return `""` (not supported via direct install) — the loadout engine is separate from the standard `installer.Install` dispatch.

**Dependencies:** B1

---

### B5 — Create kitchen-sink loadout content item

**Files created:**
- `content/loadouts/claude-code/example-kitchen-sink-loadout/loadout.yaml`
- `content/loadouts/claude-code/example-kitchen-sink-loadout/.syllago.yaml`
- `content/loadouts/claude-code/example-kitchen-sink-loadout/README.md`

**What it does:**
Creates the test loadout that exercises every content type supported by Claude Code. References existing kitchen-sink examples.

`loadout.yaml`:
```yaml
kind: loadout
version: 1
provider: claude-code
name: example-kitchen-sink-loadout
description: >
  Test loadout that exercises every content type.
  For development validation only.

rules:
  - example-kitchen-sink-rules
hooks:
  - example-kitchen-sink-hooks
skills:
  - syllago-guide
agents:
  - example-kitchen-sink-agent
mcp:
  - example-kitchen-sink-mcp
commands:
  - example-kitchen-sink-commands
prompts:
  - example-kitchen-sink-prompt
```

`.syllago.yaml`:
```yaml
id: <generated-uuid>
name: example-kitchen-sink-loadout
description: Test loadout that exercises every content type. For development validation only.
tags:
  - example
  - builtin
hidden: true
```

`README.md`: brief description noting this is for development testing only.

**Dependencies:** B3 (scanner must discover it)

---

## Phase C: Core Loadout Engine

**Goal:** Implement the resolver, validator, snapshot system, apply orchestration, and remove logic. This phase produces the engine that CLI commands and TUI actions will call.

---

### C1 — Manifest resolver

**Files modified:**
- `cli/internal/loadout/resolve.go` (new file)

**What it does:**
Takes a `*Manifest` and a `*catalog.Catalog` and resolves each name reference to a `catalog.ContentItem`.

```go
package loadout

// ResolvedRef links a manifest entry to its catalog item.
type ResolvedRef struct {
    Type catalog.ContentType
    Name string
    Item catalog.ContentItem
}

// Resolve resolves all manifest references against the catalog.
// Returns an error describing every unresolved ref (not just the first).
func Resolve(manifest *Manifest, cat *catalog.Catalog) ([]ResolvedRef, error)
```

Resolution logic follows the design doc:
- For provider-specific types (`Rules`, `Hooks`, `Commands`): find an item where `item.Type == ct && item.Provider == manifest.Provider && item.Name == name`
- For universal types (`Skills`, `Agents`, `Prompts`, `MCP`, `Apps`): find an item where `item.Type == ct && item.Name == name`
- Catalog precedence is already encoded in `cat.Items` order (local > content > registry) from `applyPrecedence` — take the first match

**Dependencies:** B1, B2

**Test expectations:**
- `TestResolve_AllFound` — all refs resolve to catalog items
- `TestResolve_MissingRule` — error includes "rules: security-conventions not found"
- `TestResolve_MultipleErrors` — all missing refs reported in one error, not just the first
- `TestResolve_ProviderScopedLookup` — rule lookup uses provider from manifest, not item.Provider field directly

---

### C2 — Loadout validator

**Files modified:**
- `cli/internal/loadout/validate.go` (new file)

**What it does:**
Validates a resolved loadout before apply:

```go
// ValidationResult describes one validation issue.
type ValidationResult struct {
    Ref     ResolvedRef
    Problem string
}

// Validate checks that:
//   1. Each resolved item supports the target provider (via provider.SupportsType)
//   2. No two refs have the same (type, name) pair (dedup)
// Returns a slice of validation results (empty = valid).
func Validate(refs []ResolvedRef, prov provider.Provider) []ValidationResult
```

Note: conflict checking (symlink already exists, hook already installed) happens during `Preview` (C3), not here. Validate is purely about the manifest contents being coherent.

**Dependencies:** C1

**Test expectations:**
- `TestValidate_Clean` — no issues for valid resolved refs
- `TestValidate_UnsupportedType` — item type not supported by provider → issue reported
- `TestValidate_DuplicateRef` — same name listed twice in rules → issue reported

---

### C3 — Preview generator

**Files modified:**
- `cli/internal/loadout/preview.go` (new file)

**What it does:**
Computes what `apply` would do without touching the filesystem. Returns a structured result for both CLI display and TUI rendering.

```go
// PlannedAction describes one action the loadout apply would take.
type PlannedAction struct {
    Type    catalog.ContentType
    Name    string
    Action  string // "create-symlink", "merge-hook", "merge-mcp", "skip-exists", "error-conflict"
    Detail  string // human-readable path or description
    Problem string // non-empty if Action == "error-conflict"
}

// Preview computes all actions without modifying any files.
// repoRoot is used to resolve absolute paths for symlink targets.
// homeDir is used as the base for provider install directories (normally os.UserHomeDir()).
func Preview(refs []ResolvedRef, prov provider.Provider, repoRoot string, homeDir string) ([]PlannedAction, error)
```

For symlink types (`Rules`, `Skills`, `Agents`, `Commands`):
- Compute target path via `prov.InstallDir(homeDir, ct)` + item name
- If `os.Lstat(target)` returns `ErrNotExist` → action "create-symlink"
- If target is a symlink to the same source → action "skip-exists"
- If target is a symlink to a different source → action "error-conflict"
- If target is a regular file/dir → action "error-conflict"

For merge types (`Hooks`, `MCP`):
- Check `installed.json` for existing entry by name → "skip-exists" if present
- Otherwise → "merge-hook" or "merge-mcp"

Returns error only if `os.UserHomeDir()` fails or filesystem stat is impossible. Conflicts are encoded in `PlannedAction.Action`, not as errors — callers decide whether to abort.

**Dependencies:** C1

**Test expectations:**
- `TestPreview_AllNew` — all actions are create/merge
- `TestPreview_ExistingSameTarget` — symlink pointing to same source → skip-exists
- `TestPreview_Conflict` — symlink pointing to different source → error-conflict
- `TestPreview_HookAlreadyInstalled` — hook in installed.json → skip-exists

---

### C4 — Snapshot package

**Files modified:**
- `cli/internal/snapshot/snapshot.go` (new package)
- `cli/internal/snapshot/snapshot_test.go`

**What it does:**
Creates and restores snapshots for loadout apply/remove.

```go
package snapshot

// SnapshotManifest is written to .syllago/snapshots/<timestamp>/manifest.json.
type SnapshotManifest struct {
    LoadoutName string            `json:"loadoutName"`
    Mode        string            `json:"mode"`        // "try" or "keep"
    CreatedAt   time.Time         `json:"createdAt"`
    BackedUpFiles []string        `json:"backedUpFiles"` // relative paths inside snapshot/files/
    Symlinks    []SymlinkRecord   `json:"symlinks"`
    HookScripts []string          `json:"hookScripts"` // informational only
}

// SymlinkRecord tracks a symlink created during apply.
type SymlinkRecord struct {
    Path   string `json:"path"`   // absolute path of the symlink
    Target string `json:"target"` // absolute path it points to
}

// Create backs up files and writes the snapshot manifest.
// filesToBackup is a list of absolute paths to copy into snapshot/files/.
// symlinks and hookScripts are recorded in the manifest.
func Create(projectRoot string, loadoutName string, mode string,
    filesToBackup []string, symlinks []SymlinkRecord, hookScripts []string) (string, error)
// Returns the snapshot directory path (e.g., .syllago/snapshots/20260228T140000/).

// Load reads the manifest from the most recent (only) snapshot directory.
// Returns ErrNoSnapshot (sentinel error) if .syllago/snapshots/ is empty or missing.
func Load(projectRoot string) (*SnapshotManifest, string, error)
// Returns (manifest, snapshotDir, error).

// Restore reads backed-up files from snapshotDir and writes them back to their
// original absolute paths. Does not remove symlinks (caller does that).
func Restore(snapshotDir string, manifest *SnapshotManifest) error

// Delete removes the snapshot directory entirely.
func Delete(snapshotDir string) error

// ErrNoSnapshot is returned by Load when no snapshot exists.
var ErrNoSnapshot = errors.New("no active snapshot")
```

Snapshot timestamp format: `20260228T140000` (compact ISO 8601, safe for directory names).

File backup: copy each file in `filesToBackup` to `snapshotDir/files/<relative-to-home>`. For example, `~/.claude/settings.json` → `snapshotDir/files/.claude/settings.json`. Use `os.UserHomeDir()` to compute relative paths. Non-existent files are skipped (nothing to back up).

**Dependencies:** none (standalone package)

**Test expectations:**
- `TestCreate_BacksUpFiles` — creates snapshot dir, copies files, writes manifest
- `TestCreate_SkipsMissingFiles` — file in filesToBackup doesn't exist → no error, just skipped
- `TestLoad_NoSnapshot` — returns ErrNoSnapshot when directory missing
- `TestRestore_WritesFilesBack` — backed-up files are restored to original paths
- `TestDelete_RemovesDir` — snapshot directory deleted

---

### C5 — Hook script path resolver

**Files modified:**
- `cli/internal/loadout/hookpath.go` (new file)

**What it does:**
When a hook item has `command: ./script.sh` (relative to its item directory), the loadout engine must write an absolute path into `settings.json`.

```go
package loadout

// ResolveHookCommand reads a hook item's command field and returns the
// command string to write into settings.json.
// If the command starts with "./" or "../", it is treated as relative to the item directory
// and resolved to an absolute path. Otherwise, it is returned as-is.
func ResolveHookCommand(itemDir string, command string) string
```

The hook.json file format used by the existing installer has `hooks.<Event>[].hooks[].command`. When the loadout installer reads hook items, it calls this function for each `command` value before merging into `settings.json`.

**Dependencies:** B2

**Test expectations:**
- `TestResolveHookCommand_RelativePath` — `./script.sh` + itemDir → absolute path
- `TestResolveHookCommand_AbsolutePath` — absolute command passes through unchanged
- `TestResolveHookCommand_InlineCommand` — `echo hello` passes through unchanged

---

### C6 — Apply engine

**Files modified:**
- `cli/internal/loadout/apply.go` (new file)

**What it does:**
Orchestrates the full apply sequence: validate → preview → snapshot → apply items → record in `installed.json`. This is the core of the loadout engine.

```go
package loadout

// ApplyOptions configures a loadout apply operation.
type ApplyOptions struct {
    Mode        string // "preview", "try", or "keep"
    ProjectRoot string
    HomeDir     string // defaults to os.UserHomeDir() if empty
    RepoRoot    string // catalog repo root for symlink source resolution
}

// ApplyResult describes what happened during apply.
type ApplyResult struct {
    Actions     []PlannedAction // what was done (or planned, for preview)
    SnapshotDir string          // set on success for try/keep modes
    Warnings    []string
}

// Apply resolves, validates, and applies a loadout to the provider.
// For mode=="preview": computes actions without touching files.
// For mode=="try" or "keep": takes snapshot, applies changes, records in installed.json.
// If any apply step fails, rolls back using the snapshot (all-or-nothing).
func Apply(manifest *Manifest, cat *catalog.Catalog, prov provider.Provider, opts ApplyOptions) (*ApplyResult, error)
```

Apply sequence for `try`/`keep`:
1. `Resolve(manifest, cat)` → refs
2. `Validate(refs, prov)` → abort if any issues
3. `Preview(refs, prov, repoRoot, homeDir)` → actions; abort if any `error-conflict` actions
4. Collect files to back up: `settings.json` (if any hooks or this is `--try`), `.claude.json` (if any MCP refs)
5. `snapshot.Create(...)` → snapshotDir
6. For each action (skipping `skip-exists`):
   - `create-symlink`: call `installer.CreateSymlink(source, target)`
   - `merge-hook`: read hook.json, resolve command paths via `ResolveHookCommand`, call `installHookEntry` (internal helper that appends to settings.json without the `_syllago` marker)
   - `merge-mcp`: call existing `installMCP` logic (minus the `_syllago` marker, already removed in Phase A)
7. If `mode == "try"`: inject `SessionEnd` hook into `settings.json` (see C7)
8. Write all actions to `installed.json` via `installer.SaveInstalled`
9. If any step 6–8 fails: call `snapshot.Restore`, then `snapshot.Delete`, return error

**Notes on hook merging:** The existing `installHook` in `hooks.go` reads from a file path in `item.Path`. The loadout engine needs a lower-level function that takes the already-parsed hook JSON bytes and appends them directly. Extract an `appendHookEntry(settingsPath string, event string, matcherGroup []byte) error` internal helper in `hooks.go` that both `installHook` and the loadout apply engine can call.

**Dependencies:** C1, C2, C3, C4, C5, A1–A7

**Test expectations (integration):**
- `TestApply_PreviewMode` — no files modified
- `TestApply_KeepMode_CreatesSymlinks` — symlinks created at expected paths
- `TestApply_KeepMode_MergesHooks` — settings.json contains hook entries
- `TestApply_KeepMode_MergesMCP` — .claude.json contains MCP entries
- `TestApply_RollbackOnFailure` — if mid-apply step fails, snapshot restored
- `TestApply_ConflictAborts` — error-conflict action prevents apply

---

### C7 — SessionEnd hook injection for `--try` mode

**Files modified:**
- `cli/internal/loadout/apply.go` (extends C6)

**What it does:**
After the main apply completes for `mode == "try"`, appends the auto-revert SessionEnd hook to `.claude/settings.json`:

```json
{
  "matcher": "",
  "hooks": [{
    "type": "command",
    "command": "syllago loadout remove --auto"
  }]
}
```

This is appended to `hooks.SessionEnd` in `settings.json` using the same `appendHookEntry` helper from C6. It is NOT recorded in `installed.json` — it will be reverted when the snapshot restores `settings.json`.

**Dependencies:** C6

**Test expectations:**
- `TestApply_TryMode_InjectsSessionEndHook` — settings.json after `--try` apply contains the SessionEnd hook for `syllago loadout remove --auto`

---

### C8 — Remove/revert engine

**Files modified:**
- `cli/internal/loadout/remove.go` (new file)

**What it does:**
Implements `syllago loadout remove` — restores from snapshot.

```go
package loadout

// RemoveOptions configures a loadout remove operation.
type RemoveOptions struct {
    Auto        bool   // if true, skip confirmation; used by --auto flag
    ProjectRoot string
}

// RemoveResult describes what was reverted.
type RemoveResult struct {
    RestoredFiles  []string // absolute paths of files restored from snapshot
    RemovedSymlinks []string // absolute paths of symlinks deleted
    LoadoutName    string
}

// Remove reads the active snapshot, restores backed-up files, deletes symlinks,
// cleans up installed.json entries (those with source == "loadout:<name>"),
// and deletes the snapshot directory.
// Returns ErrNoActiveLoadout if no snapshot exists.
func Remove(opts RemoveOptions) (*RemoveResult, error)

// ErrNoActiveLoadout is returned when no snapshot is found to revert.
var ErrNoActiveLoadout = errors.New("no active loadout to remove")
```

Remove sequence:
1. `snapshot.Load(projectRoot)` → manifest, snapshotDir
2. `snapshot.Restore(snapshotDir, manifest)` — writes files back
3. For each symlink in `manifest.Symlinks`: `os.Remove(symlink.Path)` (ignore ErrNotExist)
4. Load `installed.json`, remove all entries where `Source == "loadout:" + manifest.LoadoutName`
5. `snapshot.Delete(snapshotDir)`
6. Return `RemoveResult`

**Dependencies:** C4

**Test expectations:**
- `TestRemove_RestoresFiles` — settings.json restored to pre-apply content
- `TestRemove_DeletesSymlinks` — symlinks removed
- `TestRemove_CleansInstalledJSON` — installed.json entries with matching loadout source removed
- `TestRemove_NoSnapshot` — returns ErrNoActiveLoadout
- `TestRemove_SymlinkAlreadyGone` — missing symlink → no error (ErrNotExist ignored)

---

### C9 — Stale snapshot detection

**Files modified:**
- `cli/internal/loadout/stale.go` (new file)

**What it does:**
Detects stale snapshots (from `--try` loads where SessionEnd hook did not fire) and returns a message to show the user.

```go
package loadout

// CheckStaleSnapshot looks for a snapshot that was created in --try mode
// and has not been cleaned up. Returns a non-nil StaleInfo if found.
type StaleInfo struct {
    LoadoutName string
    CreatedAt   time.Time
}

// CheckStaleSnapshot returns stale info if a --try snapshot exists without
// a corresponding active session. "Stale" = mode is "try" AND snapshot age
// is greater than the staleThreshold (24 hours).
// Returns nil if no snapshot exists or the snapshot is recent/keep-mode.
func CheckStaleSnapshot(projectRoot string) (*StaleInfo, error)
```

The 24-hour threshold is a simple heuristic — any `--try` snapshot older than 24 hours almost certainly didn't auto-revert.

**Dependencies:** C4

**Test expectations:**
- `TestCheckStaleSnapshot_NoSnapshot` — returns nil
- `TestCheckStaleSnapshot_RecentTry` — recent try snapshot → returns nil (not stale yet)
- `TestCheckStaleSnapshot_OldTry` — old try snapshot → returns StaleInfo
- `TestCheckStaleSnapshot_KeepMode` — keep-mode snapshot → returns nil

---

## Phase D: CLI Commands

**Goal:** Implement all `syllago loadout` subcommands using the engine from Phase C.

---

### D1 — `syllago loadout` command group scaffold

**Files created:**
- `cli/cmd/syllago/loadout_cmd.go`

**What it does:**
Creates the `loadout` cobra command group and registers it with `rootCmd`. No logic — just the group.

```go
package main

var loadoutCmd = &cobra.Command{
    Use:   "loadout",
    Short: "Apply, create, and manage loadouts",
    Long: `Loadouts bundle a curated set of syllago content — rules, hooks, skills,
agents, MCP servers — into a single shareable configuration.

Use "syllago loadout apply" to try or apply a loadout.
Use "syllago loadout create" to build a new loadout interactively.
Use "syllago loadout remove" to revert an active loadout.`,
}

func init() {
    rootCmd.AddCommand(loadoutCmd)
}
```

**Dependencies:** none

---

### D2 — `syllago loadout list`

**Files modified:**
- `cli/cmd/syllago/loadout_cmd.go`

**What it does:**
Lists available loadouts from the catalog.

```
syllago loadout list
```

Output (human): table with Name, Provider, Items, Description columns. Items = `manifest.ItemCount()`.
Output (JSON with `--json`): array of `{name, provider, itemCount, description}`.

Implementation: scan catalog, filter `ByType(catalog.Loadouts)`, parse each item's `loadout.yaml` for `ItemCount()`.

```go
var loadoutListCmd = &cobra.Command{
    Use:   "list",
    Short: "List available loadouts",
    RunE:  runLoadoutList,
}
```

**Dependencies:** D1, B1, B3

---

### D3 — `syllago loadout status`

**Files modified:**
- `cli/cmd/syllago/loadout_cmd.go`

**What it does:**
Shows which loadout is active (if any), when it was applied, and its mode.

```
syllago loadout status
```

Output examples:
```
Active loadout: security-hardened (keep)
Applied: 2026-02-28 14:00:00

No active loadout.
```

With `--json`: `{active: bool, name: string, mode: string, appliedAt: string}`.

Implementation: call `snapshot.Load(projectRoot)`. If `ErrNoSnapshot`, print "No active loadout." Also call `CheckStaleSnapshot` and print a stale warning if applicable.

**Dependencies:** D1, C4, C9

---

### D4 — `syllago loadout apply` (preview and apply modes)

**Files modified:**
- `cli/cmd/syllago/loadout_apply.go` (new file)

**What it does:**
The main apply command. Supports three modes.

```
syllago loadout apply [name]          # preview (default)
syllago loadout apply [name] --try    # apply temporarily
syllago loadout apply [name] --keep   # apply permanently
```

When `name` is omitted: lists available loadouts and prompts for selection (simple numbered list on stdout, user types a number — no TUI dependency).

Behavior:
1. Load catalog (same pattern as other commands: call `findContentRepoRoot()`, `catalog.ScanWithRegistries()`)
2. Find loadout item by name, parse `loadout.yaml`
3. Check for stale snapshot (C9) and print warning if present — but don't block
4. Check for existing active snapshot: if one exists and mode is `--try` or `--keep`, error: "A loadout is already active. Run `syllago loadout remove` first."
5. Call `loadout.Apply(manifest, cat, provider.ClaudeCode, opts)` with mode from flags
6. Print results:
   - Preview: show planned actions table (create/skip/merge per item)
   - Try/keep: show what was applied, print reminder for `--try`

```go
var loadoutApplyCmd = &cobra.Command{
    Use:   "apply [name]",
    Short: "Apply a loadout",
    Long: `Apply a loadout to configure the current project for Claude Code.

Default mode (no flags): preview what would happen without making changes.
--try: apply temporarily; reverts automatically when the session ends.
--keep: apply permanently; run "syllago loadout remove" to undo.`,
    Args: cobra.MaximumNArgs(1),
    RunE: runLoadoutApply,
}

func init() {
    loadoutApplyCmd.Flags().Bool("try", false, "Apply temporarily; auto-revert on session end")
    loadoutApplyCmd.Flags().Bool("keep", false, "Apply permanently")
    loadoutCmd.AddCommand(loadoutApplyCmd)
}
```

For `--try` mode, after applying, print:
```
This loadout is temporary. It will auto-revert when the session ends.
If auto-revert fails, run: syllago loadout remove
```

**Dependencies:** D1, C6, C7

---

### D5 — `syllago loadout remove`

**Files modified:**
- `cli/cmd/syllago/loadout_cmd.go`

**What it does:**
Reverts the active loadout using the snapshot.

```
syllago loadout remove          # confirms before reverting
syllago loadout remove --auto   # skips confirmation (used by SessionEnd hook)
```

Behavior:
1. Call `loadout.Remove(opts)` where `opts.Auto` = value of `--auto` flag
2. Without `--auto`: print what will be reverted, then print the manual-edits warning (see below), then prompt for confirmation (`[y/N]`)
3. With `--auto`: skip confirmation, proceed immediately
4. Print "Loadout removed. Original configuration restored." on success

**Manual-edits warning (required by design):** The confirmation prompt must include: `"Note: Any changes you made to settings.json or .claude.json after applying the loadout will be lost — the original files are restored from a snapshot."` This is explicitly required by the design doc to communicate the clean-round-trip behavior.

For `--auto`, the command must be silent on success (no output). This is for the SessionEnd hook which runs in the background — any stdout would appear in the Claude Code terminal unexpectedly. Add `if !opts.Auto { fmt.Println("...") }` guards on all output.

```go
var loadoutRemoveCmd = &cobra.Command{
    Use:   "remove",
    Short: "Remove the active loadout and restore original configuration",
    RunE:  runLoadoutRemove,
}

func init() {
    loadoutRemoveCmd.Flags().Bool("auto", false, "Skip confirmation (used by SessionEnd hook)")
    loadoutCmd.AddCommand(loadoutRemoveCmd)
}
```

**Dependencies:** D1, C8

---

### D6 — `syllago loadout create` (CLI wizard)

**Files modified:**
- `cli/cmd/syllago/loadout_create_cmd.go` (new file)

**What it does:**
Interactive CLI wizard that creates a new loadout. Launches the TUI builder (Phase F) when stdout is a terminal, otherwise falls back to a flag-based creation for scripting.

For v1, the `syllago loadout create` CLI command simply launches the full TUI with the builder modal pre-opened. This avoids duplicating wizard logic between CLI and TUI.

```go
var loadoutCreateCmd = &cobra.Command{
    Use:   "create",
    Short: "Interactively create a new loadout",
    RunE:  runLoadoutCreate,
}
```

`runLoadoutCreate` calls `runTUI` with an additional flag/option indicating that the TUI should open directly to the loadout builder modal (Phase F, task F4).

**Dependencies:** D1, F4

---

## Phase E: `--try` Auto-Revert Infrastructure

**Goal:** Ensure the stale snapshot detection and `--auto` flag work correctly in all edge cases.

---

### E1 — Stale snapshot warning in all loadout commands

**Files modified:**
- `cli/cmd/syllago/loadout_cmd.go`
- `cli/cmd/syllago/loadout_apply.go`

**What it does:**
Any `syllago loadout` command that runs should check for stale snapshots and display a warning if found. Extract a helper `checkAndWarnStaleSnapshot(projectRoot string)` that:
1. Calls `loadout.CheckStaleSnapshot(projectRoot)`
2. If stale info returned, prints to stderr: `"Warning: A temporary loadout was not cleaned up. Run 'syllago loadout remove' to restore your original config."`
3. Does nothing if no stale snapshot

Call this helper at the start of `runLoadoutApply`, `runLoadoutList`, `runLoadoutStatus`, `runLoadoutRemove`, and `runLoadoutCreate`. The design specifies "any `syllago loadout` command" — all five subcommands must check. Calling it from `runLoadoutRemove` is especially important: if the user runs `syllago loadout remove` to manually clean up a stale --try session, they should see the warning before proceeding.

**Dependencies:** C9, D3, D4, D5, D6

---

### E2 — Integration test: `--try` round-trip with auto-revert

**Files modified:**
- `cli/internal/loadout/apply_test.go`

**What it does:**
An integration test that simulates the full `--try` lifecycle:
1. Create a temp directory with a mock catalog (kitchen-sink loadout pointing to existing example items)
2. Call `Apply` with mode `"try"`
3. Verify `settings.json` contains the SessionEnd hook for `syllago loadout remove --auto`
4. Verify `snapshot.Load` finds the snapshot with mode `"try"`
5. Call `Remove` with `Auto: true`
6. Verify `settings.json` is restored to pre-apply state
7. Verify symlinks are removed
8. Verify `snapshot.Load` returns `ErrNoSnapshot`

**Dependencies:** C6, C7, C8

---

## Phase F: TUI Integration

**Goal:** Add Loadouts to the TUI: sidebar navigation, items list, detail view with three tabs, apply modal, and builder wizard.

---

### F1 — Sidebar: Loadouts appears automatically

**Files modified:**
- `cli/internal/tui/sidebar.go`
- `cli/internal/tui/items.go`

**What it does:**
Since `catalog.AllContentTypes()` now includes `Loadouts` (from B1), the sidebar already iterates over it and shows the count. No structural change to `sidebar.go` is needed — the loop `for i, ct := range m.types` handles it.

However, the items list view (`itemsModel`) needs to handle the `Loadouts` content type for its display. For loadout items, the "install status" column shown in the items list should be omitted (loadouts don't have a per-item install status — they're applied as a whole). In `items.go`, add a case in the column-rendering logic: if `ct == catalog.Loadouts`, render item count from the manifest description instead of provider check badges.

For the active loadout indicator: add a field `activeLoadout string` to `sidebarModel`. When non-empty, the Loadouts row gets a `[*]` suffix or similar indicator. Populated in Phase F3.

**Dependencies:** B1

---

### F2 — Detail view: Loadouts with three tabs

**Files modified:**
- `cli/internal/tui/detail.go`
- `cli/internal/tui/detail_render.go`

**What it does:**
The detail view for a loadout item shows three tabs (Overview, Contents, Apply) instead of the standard three tabs (Overview, Files, Install).

In `detailModel`, the `activeTab` field uses the same `detailTab` iota. Add new constants:
```go
const (
    tabOverview detailTab = iota
    tabFiles                  // existing
    tabInstall                // existing
    tabLoadoutContents        // new: loadout manifest breakdown
    tabLoadoutApply           // new: apply mode selection
)
```

When `item.Type == catalog.Loadouts`, the detail view renders tabs as "Overview | Contents | Apply" by mapping to `tabOverview`, `tabLoadoutContents`, `tabLoadoutApply`.

**Overview tab:** render `item.ReadmeBody` via glamour (same as other types). Show description, provider, item count.

**Contents tab:** parse `loadout.yaml`, render a grouped list:
```
Rules (2):    security-conventions, project-core
Hooks (3):    block-sensitive-files, secret-scanner, audit-logger
Skills (1):   secure-deploy
```

**Apply tab:** three-choice selector (Preview / Try / Keep) using the same cursor pattern as `installStepLocation`. When user presses Enter on a choice, send an `openLoadoutApplyMsg` to App.

**Dependencies:** B2, F1

---

### F3 — Apply modal in TUI

**Files modified:**
- `cli/internal/tui/modal.go`
- `cli/internal/tui/app.go`

**What it does:**
Adds a `loadoutApplyModal` to the App model that handles the apply flow from the TUI.

```go
// openLoadoutApplyMsg is sent by detailModel to ask App to run a loadout apply.
type openLoadoutApplyMsg struct {
    item catalog.ContentItem
    mode string // "preview", "try", "keep"
}
```

In `App`, handle `openLoadoutApplyMsg`:
- For `"preview"`: run `loadout.Apply` synchronously and show results in a `confirmModal` (reuse the existing pattern)
- For `"try"` and `"keep"`: show confirmation modal first, then run apply in a background goroutine (same pattern as `appInstallDoneMsg`)

Add `loadoutApplyDoneMsg` carrying the `*loadout.ApplyResult` and error. App handles it by showing a status message.

For `--try` mode confirmation, the modal body must match the design doc verbatim:
```
This loadout is temporary. It will auto-revert when the session ends.
If auto-revert fails, run: syllago loadout remove
Apply?
```

For `--keep` mode:
```
This loadout will stay until you run: syllago loadout remove
Apply?
```

**Active loadout indicator:** after successful apply, set `a.sidebar.activeLoadout = manifest.Name`. After remove, clear it. Store the active loadout name in the App struct.

**Dependencies:** F2, C6

---

### F4 — Builder wizard modal

**Files modified:**
- `cli/internal/tui/modal.go`
- `cli/internal/tui/app.go`
- `cli/internal/tui/items.go`

**What it does:**
Multi-step wizard modal for creating a loadout. Triggered by `[C]` from the loadouts items list.

Add `loadoutBuilderModal` struct to `modal.go`:

```go
type builderStep int
const (
    builderStepProvider     builderStep = iota // Step 1: choose provider
    builderStepTypes                           // Step 2: choose content types
    builderStepItems                           // Steps 3–N: select items per type
    builderStepReview                          // Review screen
    builderStepName                            // Name + description
    builderStepSave                            // Save confirmation
)

type loadoutBuilderModal struct {
    active      bool
    step        builderStep
    // Step 1
    providerIdx int    // 0 = Claude Code (only option for v1)
    // Step 2
    selectedTypes []catalog.ContentType // which types are checked
    typesCursor   int
    // Step 3..N
    currentTypeIdx  int            // which type we're selecting items for
    itemSelections  map[catalog.ContentType][]string // selected item names per type
    itemsCursor     int
    availableItems  []catalog.ContentItem // items for current type
    // Name/desc
    nameInput textinput.Model
    descInput textinput.Model
    // Error/status
    message     string
    messageIsErr bool
    // Catalog reference (for item lookup per type)
    catalog *catalog.Catalog
}
```

Navigation:
- `[Space]` toggles selection on current item (steps 2 and 3..N)
- `[Enter]` advances to next step; on step 3..N, goes to next selected type or review
- `[Esc]` goes back one step
- `[C]` on loadout items list sends `openBuilderMsg` to App

On save, App calls a new function:
```go
// WriteLoadout creates the loadout directory and files.
// destDir is content/loadouts/<provider>/<name>/ or local/loadouts/<provider>/<name>/
func WriteLoadout(destDir string, manifest *loadout.Manifest, meta *metadata.Meta) error
```

This function lives in `cli/internal/loadout/write.go`.

**Dependencies:** F1, B2, B5

---

### F5 — `syllago loadout create` TUI launch

**Files modified:**
- `cli/cmd/syllago/loadout_create_cmd.go`

**What it does:**
When `syllago loadout create` runs, it launches the TUI with the builder wizard pre-opened. Pass an `openBuilder bool` option to `tui.NewApp` (or use a startup message injected via `tea.Cmd`).

Implementation: add a `startupCmd tea.Cmd` field to `App` that fires on `Init()`. When set, it returns the startup command in addition to the update check. The startup cmd sends `openBuilderMsg{}` to the app model, which opens the builder wizard immediately.

**Dependencies:** D6, F4

---

## Phase G: Testing

**Goal:** Unit tests, integration tests, and golden file tests as specified in the design doc.

---

### G1 — Unit tests for `internal/loadout/`

**Files modified:**
- `cli/internal/loadout/manifest_test.go` (extends B2)
- `cli/internal/loadout/resolve_test.go` (extends C1)
- `cli/internal/loadout/validate_test.go` (extends C2)
- `cli/internal/loadout/preview_test.go` (extends C3)
- `cli/internal/loadout/hookpath_test.go` (extends C5)

**What it does:**
Consolidates all unit tests for the loadout package. Includes the test cases specified in each task above, plus:

- Edge case: manifest with all sections empty (valid, zero items)
- Edge case: manifest name with hyphens and underscores (valid)
- Edge case: manifest name with spaces (invalid — must fail `catalog.IsValidItemName` check in resolver)

Each test file follows the existing pattern: `t.Parallel()` at top level, sub-tests with `t.Run(...)`.

**Dependencies:** C1–C5

---

### G2 — Unit tests for `internal/snapshot/`

**Files modified:**
- `cli/internal/snapshot/snapshot_test.go` (extends C4)

**What it does:**
Complete test coverage for the snapshot package. Adds:
- `TestCreate_ManifestContents` — verify JSON fields in written manifest
- `TestRestore_PreservesPermissions` — restored file has same permissions as original
- `TestLoad_ReturnsLatestSnapshot` — if somehow two snapshot dirs exist, returns the most recent

**Dependencies:** C4

---

### G3 — Integration tests: apply/remove round-trip

**Files modified:**
- `cli/internal/loadout/integration_test.go` (new file)

**What it does:**
End-to-end tests using a real temp filesystem. These tests:
1. Create a minimal temp content library with a test loadout referencing test items
2. Create mock provider config files (`settings.json`, `.claude.json`) with pre-existing content
3. Call `Apply` with `mode="keep"`
4. Assert: symlinks created, hook merged into settings.json, MCP merged into .claude.json, installed.json populated, snapshot exists
5. Call `Remove`
6. Assert: settings.json and .claude.json restored exactly, symlinks gone, installed.json cleaned, snapshot deleted

**Dependencies:** C6, C8

---

### G4 — Golden file tests for settings.json and .claude.json

**Files modified:**
- `cli/internal/loadout/golden_test.go` (new file)
- `cli/internal/loadout/testdata/expected-settings.json` (new golden file)
- `cli/internal/loadout/testdata/expected-claude.json` (new golden file)

**What it does:**
Apply the kitchen-sink loadout in a controlled temp environment and snapshot the resulting `settings.json` and `.claude.json`. Compare against golden files.

Uses the same pattern as `cli/internal/tui/golden_test.go` which uses `charmbracelet/x/exp/golden`.

Update golden files by running: `UPDATE_GOLDEN=1 go test ./internal/loadout/...`

**Dependencies:** G3, B5

---

### G5 — Update existing installer tests

**Files modified:**
- `cli/internal/installer/mcp_test.go`
- `cli/internal/installer/hooks_test.go` (if exists; otherwise via `installer_test.go`)

**What it does:**
Final pass to ensure all installer tests pass after Phase A changes. Specifically:
- `TestInstallMCP_WhitelistsFields`: remove `_syllago` assertion, add `installed.json` assertion
- Any hook tests that check for `_syllago` in settings.json: update same way
- Run `make test` and confirm zero failures

**Dependencies:** A1–A7

---

### G6 — Unit and integration tests for `WriteLoadout`

**Files modified:**
- `cli/internal/loadout/write_test.go` (new file)

**What it does:**
The design's integration test list explicitly requires: *"Builder wizard — Create loadout via wizard → verify loadout.yaml and .syllago.yaml generated correctly."* Task F4 implements `WriteLoadout` in `cli/internal/loadout/write.go` but specifies no tests. This task provides that coverage.

Unit tests for `WriteLoadout`:
- `TestWriteLoadout_CreatesDirectory` — target directory is created if it does not exist
- `TestWriteLoadout_WritesLoadoutYAML` — `loadout.yaml` contains all manifest fields correctly serialized
- `TestWriteLoadout_WritesSyllagoYAML` — `.syllago.yaml` contains correct metadata (id, name, description, tags)
- `TestWriteLoadout_EmptySectionsOmitted` — sections with no items are absent from `loadout.yaml`
- `TestWriteLoadout_OverwriteRefused` — if `loadout.yaml` already exists at dest, return error (prevent silent overwrite)

Integration test (mirrors the design's "Builder wizard" test case):
- `TestBuilderWizardOutput_RoundTrip` — simulate wizard completion: build a `Manifest` and `Meta` struct as the wizard would produce them, call `WriteLoadout`, then call `loadout.Parse` on the resulting file and verify the output matches the input. This validates that the wizard-to-disk-to-parse round-trip is lossless.

Golden file variant:
- `TestWriteLoadout_Golden` — write a known manifest, compare `loadout.yaml` against `testdata/expected-writeloadout.yaml`. Add `testdata/expected-writeloadout.yaml` to the files created list.

**Dependencies:** F4 (WriteLoadout implementation)

---

## Execution Order Summary

```
Phase A (prerequisite): A1 → A2 → A3 → A4 (hooks chain)
                         A1 → A5 → A6 (mcp chain, parallel with hooks)
                         A7 (after both chains)

Phase B (parallel with A): B1 → B2 → B3 → B4 → B5

Phase C (after A and B): C1, C4, C5 can start after B
                          C2 after C1
                          C3 after C1
                          C6 after C1, C2, C3, C4, C5 (and A complete)
                          C7 after C6
                          C8 after C4
                          C9 after C4

Phase D (after C): D1 → D2, D3, D5 (parallel after D1)
                        D4 after C6, C7
                        D6 after F4 (defer until F4 done)

Phase E (after D): E1 after C9, D3, D4
                   E2 after C6, C7, C8

Phase F (after B and C): F1 after B1
                          F2 after B2, F1
                          F3 after F2, C6
                          F4 after F1, B2, B5
                          F5 after D6, F4

Phase G (after all): G1 after C1–C5
                     G2 after C4
                     G3 after C6, C8
                     G4 after G3, B5
                     G5 after A1–A7
                     G6 after F4
```

---

## Files Created (new)

```
cli/internal/installer/installed.go
cli/internal/installer/installed_test.go
cli/internal/loadout/manifest.go
cli/internal/loadout/manifest_test.go
cli/internal/loadout/resolve.go
cli/internal/loadout/resolve_test.go
cli/internal/loadout/validate.go
cli/internal/loadout/validate_test.go
cli/internal/loadout/preview.go
cli/internal/loadout/preview_test.go
cli/internal/loadout/hookpath.go
cli/internal/loadout/hookpath_test.go
cli/internal/loadout/apply.go
cli/internal/loadout/apply_test.go
cli/internal/loadout/remove.go
cli/internal/loadout/remove_test.go
cli/internal/loadout/stale.go
cli/internal/loadout/stale_test.go
cli/internal/loadout/write.go
cli/internal/loadout/integration_test.go
cli/internal/loadout/golden_test.go
cli/internal/loadout/testdata/valid-loadout.yaml
cli/internal/loadout/testdata/expected-settings.json
cli/internal/loadout/testdata/expected-claude.json
cli/internal/loadout/testdata/expected-writeloadout.yaml
cli/internal/loadout/write_test.go
cli/internal/snapshot/snapshot.go
cli/internal/snapshot/snapshot_test.go
cli/cmd/syllago/loadout_cmd.go
cli/cmd/syllago/loadout_apply.go
cli/cmd/syllago/loadout_create_cmd.go
content/loadouts/claude-code/example-kitchen-sink-loadout/loadout.yaml
content/loadouts/claude-code/example-kitchen-sink-loadout/.syllago.yaml
content/loadouts/claude-code/example-kitchen-sink-loadout/README.md
```

---

## Files Modified (existing)

```
cli/internal/catalog/types.go          — add Loadouts const and Label/IsUniversal cases
cli/internal/catalog/scanner.go        — add Loadouts case in scanProviderDir
cli/internal/installer/hooks.go        — remove _syllago marker, use installed.json
cli/internal/installer/mcp.go          — remove _syllago marker, use installed.json
cli/internal/installer/mcp_test.go     — update _syllago assertion
cli/internal/provider/claude.go        — add Loadouts to SupportsType
cli/internal/tui/sidebar.go            — add activeLoadout field
cli/internal/tui/items.go              — loadouts column rendering
cli/internal/tui/detail.go             — three-tab loadout detail view
cli/internal/tui/detail_render.go      — loadout tab rendering
cli/internal/tui/modal.go              — loadoutApplyModal + loadoutBuilderModal
cli/internal/tui/app.go                — handle new modal messages, activeLoadout state
```

---

## Constraints Checklist

- [x] No `_syllago` fields written to any provider config file after Phase A
- [x] Claude Code only for v1 (all engine code targets `provider.ClaudeCode`; SupportsType gate enforces it)
- [x] No `bundle` command (not in this plan)
- [x] No `inline:` manifest support (refs only)
- [x] One loadout at a time (apply errors if snapshot already exists)
- [x] All new code is Go
- [x] New packages follow existing patterns (flat under `cli/internal/`, same test conventions)
- [x] YAML parsing uses `gopkg.in/yaml.v3` (already a dependency)
- [x] Atomic file writes use the temp-then-rename pattern from `jsonmerge.go` and `config.go`
