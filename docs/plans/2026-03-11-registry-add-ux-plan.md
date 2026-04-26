# Plan: Registry Add UX Fix

**Date:** 2026-03-11 (v3 — updated after plan review round)
**Scope:** Registry add flow — status message visibility, smart redirect for non-registry URLs, message lifecycle
**Review:** `docs/reviews/registry-add-ux.md` — 5 expert agents reviewed code + plan (2 rounds)

---

## Problems

### 1. Status messages display in the wrong place
`statusMessage` is set by registry add/remove/sync operations (app.go:604, 606, 626, 628, 651, 653), but it only renders inside `renderContentWelcome()` (app.go:2103-2108), which is the category landing screen. When the user is on the registries screen, that message is invisible — or shows up stale when navigating back later.

The registry screen (`registries.View()`) has no status message area. When a registry add fails or succeeds, the user gets no feedback on that screen.

**Files:** `app.go` (status rendering), `registries.go` (missing status area)

### 2. Errors render as success (green "Done:" text)
`statusMessage` has no error/success flag. Line 2104 always renders with `successMsgStyle` and "Done:" prefix — even for errors like "Add failed: not a syllago registry...". Green text saying "Done: Add failed" actively contradicts the content and violates WCAG 1.4.1 (Use of Color). This is worse than no styling at all.

**Files:** `app.go:103-104` (missing `statusIsErr`), `app.go:2103-2104` (always green)

### 3. "Not a syllago registry" gives a dead-end error
When adding a git URL with provider-native content (e.g., a repo with `.cursorrules` or `CLAUDE.md`), the code at app.go:237-249:
- Clones the repo
- Scans with `ScanNativeContent()`
- Finds provider content but no syllago structure
- Deletes the clone and returns an error: "not a syllago registry (found X content) -- use syllago add <path> instead"

No prompt to the user. No offer to switch to the import flow. References a CLI command that doesn't map to a TUI action. Destroys the clone the user just waited for.

**Files:** `app.go` (doRegistryAdd), `catalog/native_scan.go` (detection)

### 4. Data race in registry add AND remove
Both command closures mutate `cfg.Registries` via a shared pointer while the main goroutine reads it in View():
- **Remove** (app.go:191-205): filters the slice
- **Add** (app.go:251): appends to the slice

Both capture `cfg := a.registryCfg` (a pointer) then mutate `cfg.Registries` in a goroutine. Since `cfg` points to `a.registryCfg`, View() can read the slice concurrently.

**Files:** `app.go:191-205` (remove), `app.go:216-258` (add)

### 5. Status messages never clear
`statusMessage` is write-only. Once set, it persists forever. Stale messages mislead users about current state.

**Files:** `app.go` (no clearing logic exists)

### 6. No visual feedback during async clone
User pastes URL, hits Enter, and nothing happens visually until clone finishes (seconds). `registryOpInProgress` exists but isn't rendered. The status message "Adding registry: ..." is set but invisible (Problem 1).

### 7. Fail-fast validation missing
Name derivation from URL and duplicate/invalid name checking happen inside the async command (app.go:218-228), after the modal closes and the clone begins. These could be checked synchronously before starting the expensive network operation.

---

## Resolved Questions (from expert review)

- **Preserve the clone or re-clone?** Preserve. Re-cloning wastes time and bandwidth. Pass the path through to the import flow.
- **Show detected content in the modal?** Yes. Show what providers were found and how many items — helps the user decide whether to proceed.
- **"Add as registry anyway" option?** No. Not worth the complexity. The redirect to import is the right default.

---

## Implementation

### Phase 0: Safety fixes (pre-requisite)

Fix the data race and add fail-fast validation. These are correctness issues independent of UX.

#### 0a. Fix data races in registry add and remove `romanesco-uiqc`
**Files:** `app.go:191-205` (remove), `app.go:216-258` (add)

Both closures capture `cfg := a.registryCfg` (a pointer) then mutate `cfg.Registries` in a goroutine while the main goroutine reads the same slice in View(). Fix both with the same strategy:

**Config save strategy:** Inside each command closure, load a fresh config from disk (`config.Load`), modify the fresh copy (append or filter), then save. This avoids mutating the shared pointer entirely. The done-message handler already rebuilds from disk via `rebuildRegistryState()`.

**Remove** (app.go:191-205): Pass only the registry name into the closure. Load fresh config, filter out the name, save. Do filesystem removal (`registry.Remove`).

**Add** (app.go:216-258): After cloning and scanning, load fresh config inside the closure, append the new entry, save. Don't touch `a.registryCfg` from the goroutine.

**Error handling:** If `config.Load` fails inside the closure, return the done message with the error (e.g., `registryAddDoneMsg{err: fmt.Errorf("loading config: %w", err)}`). The done-message handler already handles error vs success branching.

**Concurrency note:** `registryOpInProgress` prevents concurrent registry operations from the UI, so there's no TOCTOU risk between Load and Save within a single closure.

#### 0b. Fail-fast name validation before clone `romanesco-badn`
**Files:** `app.go:1160-1171`, `app.go:218-228`

Move name derivation (`registry.NameFromURL`) and validation checks (invalid name, duplicate name) to happen synchronously before calling `doRegistryAdd`. Show validation errors in the modal's `message` field and keep the modal open.

**How the modal lifecycle works:** The modal's `Update` method sets `confirmed = true` and `active = false` on Enter (modal.go:903-904). The outer handler at app.go:1163 detects `!active`, then checks `confirmed`. Validation must happen in the outer handler at lines 1165-1168, and **re-open the modal** on failure by setting `a.registryAddModal.active = true`, `a.registryAddModal.confirmed = false`, and `a.focus = focusModal` (to keep input routing consistent with modal visibility).

```go
if a.registryAddModal.confirmed {
    url := strings.TrimSpace(a.registryAddModal.urlInput.Value())
    nameOverride := strings.TrimSpace(a.registryAddModal.nameInput.Value())
    name := nameOverride
    if name == "" {
        name = registry.NameFromURL(url)
    }
    if !catalog.IsValidRegistryName(name) {
        a.registryAddModal.message = fmt.Sprintf("Invalid registry name: %s", name)
        a.registryAddModal.messageIsErr = true
        a.registryAddModal.active = true
        a.registryAddModal.confirmed = false
        return a, nil
    }
    // Check duplicates similarly, then:
    return a, a.doRegistryAdd(url, nameOverride)
}
```

This re-activation approach works because the modal's `View()` checks `if !m.active { return "" }` — setting `active = true` makes it render again immediately.

---

### Phase 1: Fix status message system

Fix the rendering, styling, and lifecycle of status messages so they actually work.

**Implementation order:** Start with 1d (extract helper) before 1a-1c. The helper consolidates the three done-message handlers, so extracting it first avoids double-touching that code when adding `statusIsErr` logic.

#### 1a. Add error/success distinction to statusMessage `romanesco-x4p4`
**File:** `app.go`

Add `statusIsErr bool` field to App struct alongside `statusMessage`. Set it appropriately:
- `true` at lines 604, 626, 651, 667 (error paths)
- `false` at lines 606, 628, 653, 669 (success paths)

#### 1b. Render status on registries screen `romanesco-l91s`
**File:** `registries.go`

Change `View(cursor int)` to `View(cursor int, statusMsg string, statusIsErr bool)`. Render status at the top between breadcrumb and card grid:
```go
if statusMsg != "" {
    if statusIsErr {
        s += errorMsgStyle.Render(statusMsg) + "\n\n"
    } else {
        s += successMsgStyle.Render(statusMsg) + "\n\n"
    }
}
```

Update call site at `app.go:1906`.

#### 1c. Fix status rendering on welcome screen `romanesco-d409`
**File:** `app.go:2103-2108`

Use `statusIsErr` to select style. Drop redundant "Done:" prefix (past tense messages already communicate completion). Handle in-progress messages: don't prefix with "Done:" when `registryOpInProgress` is true.

```go
if a.statusMessage != "" {
    if a.statusIsErr {
        s += errorMsgStyle.Render(a.statusMessage) + "\n"
    } else {
        s += successMsgStyle.Render(a.statusMessage) + "\n"
    }
    for _, w := range a.statusWarnings {
        s += warningStyle.Render("Warning: " + w) + "\n"
    }
    s += "\n"
}
```

#### 1d. Extract rebuildRegistryState helper `romanesco-6wo0`
**File:** `app.go:601-663`

Three done-message handlers duplicate ~10 lines of config reload + model rebuild + catalog rescan. Extract:

```go
func (a *App) rebuildRegistryState() {
    cfg, err := config.Load(a.catalog.RepoRoot)
    if err == nil {
        a.registryCfg = cfg
    }
    a.registries = newRegistriesModel(a.catalog.RepoRoot, a.registryCfg, a.catalog)
    a.registries.width = a.width - sidebarWidth - 1
    a.registries.height = a.panelHeight()
    a.sidebar.registryCount = len(a.registryCfg.Registries)
    cat, scanErr := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
    if scanErr == nil {
        a.catalog = cat
        a.refreshSidebarCounts()
    }
}
```

**Cursor adjustment:** The `registryRemoveDoneMsg` handler also clamps `a.cardCursor` after rebuild (app.go:637-639). This stays *outside* the helper — the remove handler calls `rebuildRegistryState()` then does the clamp. Add and sync handlers just call the helper directly.

#### 1e. Warn on empty repos `romanesco-mvns`
**File:** `app.go:247-249`

The code has `// No content at all — warn but allow` but never warns. Change success message to indicate emptiness: `"Added registry: %s (empty — no content found)"`.

---

### Phase 2: Smart redirect for non-registry URLs

When `doRegistryAdd` detects a non-syllago repo with provider content, offer to switch to the import flow instead of erroring.

#### 2a. New message type `romanesco-9vwt`
**File:** `app.go`

```go
type registryAddNonSyllagoMsg struct {
    name      string
    clonePath string
    scan      catalog.NativeScanResult
}
```

Replace the error return at app.go:237-246 with this message. **Do not delete the clone** — pass the path through.

#### 2b. Handle the message in App.Update `romanesco-cn1m`
**File:** `app.go`

Add new App fields to hold pending non-syllago data between message receipt and user confirmation:

```go
pendingNonSyllagoClone string                  // temp clone path
pendingNonSyllagoScan  catalog.NativeScanResult // what was detected
```

On receiving `registryAddNonSyllagoMsg`:
1. Store `msg.clonePath` → `a.pendingNonSyllagoClone`, `msg.scan` → `a.pendingNonSyllagoScan`
2. Build a description of what was found (e.g., "Found Claude Code, Cursor content")
3. Show a confirmation modal:
   - Title: "Not a Syllago Registry"
   - Body: "This repository contains [provider] content but isn't a syllago registry.\n\nBrowse and import individual items?"

#### 2c. Bridge to import flow on confirm `romanesco-ceiy`
**Files:** `app.go`, `import.go`

On confirm:
1. Call `a.importer.cleanup()` first (in case a previous clone is still held)
2. Call `a.importer.initFromExternalClone(a.pendingNonSyllagoClone)` — see below
3. Clear `a.pendingNonSyllagoClone` and `a.pendingNonSyllagoScan` (import model now owns the clone)
4. Switch to `screenImport`

**`initFromExternalClone(path string)` method on `*importModel`:**

This method mirrors the logic in the `importCloneDoneMsg` handler (import.go:277-309) but skips the clone step since we already have the directory. Fields to set:

```go
func (m *importModel) initFromExternalClone(path string) {
    m.clonedPath = path  // takes ownership for cleanup
    m.message = ""
    m.messageIsErr = false
    m.pickCursor = 0

    cat, err := catalog.Scan(path, path)
    if err != nil || len(cat.Items) == 0 {
        // Content detected by native scan but not importable via catalog.Scan.
        // Raw provider files (.cursorrules, CLAUDE.md) need CLI conversion.
        m.step = stepGitURL
        m.message = "Detected provider content but no importable items. Use CLI: syllago add <path>"
        m.messageIsErr = true
        m.cleanup()  // nothing to browse, clean up
        return
    }
    m.clonedItems = cat.Items
    m.step = stepGitPick
}
```

All other importModel fields (`sourceCursor`, `typeCursor`, `browser`, `conflict`, etc.) are irrelevant for the git-pick flow and don't need resetting — the step enum controls which fields are read.

**Esc from stepGitPick after redirect:** The import model's `updateGitPick` handler (import.go:709-713) navigates to `stepGitURL` on Esc and calls `cleanup()`. When entered via registry redirect, this leaves the user at a dead-end empty URL input. Add a `fromRegistryRedirect bool` field to `importModel`. In `initFromExternalClone`, set `m.fromRegistryRedirect = true`. In `updateGitPick`'s Esc handler, when `m.fromRegistryRedirect` is true, return a message that tells `App.Update` to switch back to `screenRegistries` (cleanup still runs — import model owns the clone). Clear `fromRegistryRedirect` on exit.

#### 2d. Cleanup on dismiss `romanesco-4f93`
The clone must be cleaned up on **all** dismiss paths, not just the Cancel button:

1. **Cancel button clicked** → `os.RemoveAll(a.pendingNonSyllagoClone)`
2. **Esc pressed** → same cleanup (both handled in the modal's dismiss path)
3. **User confirms → import flow** → `importModel` takes ownership via `initFromExternalClone`; its existing `cleanup()` handles removal on import cancel/failure

Note: sidebar clicks during a modal are not reachable (focus is trapped to `focusModal`), so path 3 from the earlier review is not needed.

After cleanup on paths 1-2, clear `a.pendingNonSyllagoClone` and `a.pendingNonSyllagoScan`.

---

### Phase 3: Clear status messages on keypress `romanesco-3zgj`

**File:** `app.go`

Add at the top of the `tea.KeyMsg` handler in `App.Update()`, before delegating to screen-specific handlers:

```go
case tea.KeyMsg:
    // Clear transient status on any keypress
    if a.statusMessage != "" {
        a.statusMessage = ""
        a.statusIsErr = false
        a.statusWarnings = nil
    }
```

Only clear on actual keypresses, not on async message completions.

---

## Key Files

| File | Role |
|------|------|
| `cli/internal/tui/app.go` | Central hub — registry add logic, status rendering, message routing |
| `cli/internal/tui/registries.go` | Registry card grid — needs status message area |
| `cli/internal/tui/modal.go` | registryAddModal — current single-step URL input |
| `cli/internal/tui/import.go` | Import wizard — has stepGitPick we want to bridge to |
| `cli/internal/catalog/native_scan.go` | Non-syllago content detection |

## Deferred to Accessibility Pass (bead romanesco-p3b9)

These accessibility improvements were identified in the review but are out of scope for this fix:
- Modal announcement markers (`[DIALOG]` prefix)
- Shift+Tab / Up+Down / Ctrl+C in modals
- Text-based selection indicators on registry cards
- Required field markers on URL input
- Validation error symbol prefixes
- strings.Builder optimization in registries.View()
