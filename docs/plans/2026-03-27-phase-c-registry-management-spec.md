# Phase C: Registry Management — Specification

**Date:** 2026-03-27
**Status:** Spec v2 — post expert panel review
**Design ref:** `docs/plans/2026-03-26-tui-wizards-design.md` (W4, Phase C section)

---

## Overview

Phase C adds three registry management actions to the TUI's Registries tab:

1. **`[a]` Add Registry** — Overlay modal for adding a git URL or local directory as a new registry
2. **`[S]` Sync** — Pull latest changes for the selected registry
3. **`[d]` Remove Registry** — Confirm modal to remove a registry (reuses existing `confirmModal`)

No full-screen wizard. All three features are small overlay modals or single-action commands.

---

## Prerequisites (Already Built)

| Component | File | Status |
|-----------|------|--------|
| Gallery card grid + sidebar | `gallery.go`, `cards.go`, `contents.go` | Done |
| `buildRegistryCards()` | `cards.go:295` | Done |
| `confirmModal` | `confirm.go` | Done |
| Toast system (success/error/progress) | `toast.go` (in app.go) | Done |
| Registry backend: `Clone`, `Sync`, `SyncAll`, `Remove` | `registry/registry.go` | Done |
| Config load/save: `LoadGlobal`, `SaveGlobal` | `config/config.go` | Done |
| `NameFromURL()` | `registry/registry.go:60` | Done |
| `rescanCatalog()` | `app.go:167` | Done |
| Action button rendering on gallery metadata bar | `gallery.go:348` | Done (has `[d] Remove`, `[e] Edit`) |
| Key constants: `keyAdd`, `keyRemove` | `keys.go` | Done |

---

## Feature 1: Registry Add Modal (`[a]`)

### Entry Point

- `[a]` keypress when on the **Registries** tab (not drilled in)
- Only when `registryOpInProgress == false`
- The metadata bar already shows action buttons; `[a]` is wired via `keyAdd` → `actionPressedMsg{action: "add"}`, which currently is a no-op

### Modal Design

Overlay modal using the existing modal pattern (bordered, centered via `overlayModal()`). NOT a full-screen wizard — registries are simple URL + optional fields.

**Modal width:** `min(64, termWidth-10)` — wider than `editModal` (56) because URLs can be 50+ chars.

```
╭─ Add Registry ───────────────────────────────────╮
│                                                  │
│  Source                                          │
│  (•) Git URL                                     │
│  ( ) Local directory                             │
│                                                  │
│  URL                                             │
│  [https://github.com/team/registry___________]   │
│                                                  │
│  Name (derived from URL)                         │
│  [team/registry______________________________]   │
│                                                  │
│  Branch (optional, git only)                     │
│  [___________________________________________]   │
│                                                  │
│                          [Cancel]   [Add]        │
╰──────────────────────────────────────────────────╯
```

### Model: `registryAddModal`

```go
type registryAddModal struct {
    active    bool
    width     int
    height    int

    // Source selection
    sourceGit bool  // true=git URL, false=local dir (default: true)

    // Fields — hand-rolled text input (same pattern as editModal)
    urlValue    string
    nameValue   string
    branchValue string
    cursor      int    // cursor position within the focused text field

    // Focus management
    focusIdx    int   // 0=source radio, 1=url, 2=name, 3=branch, 4=cancel, 5=add

    // State
    nameManuallySet bool   // true if user edited name — stops auto-derivation
    err             string // inline validation error

    // Context for validation
    existingNames []string // populated on Open() from config
}
```

**Note:** Text input uses the same hand-rolled pattern as `editModal` (`renderField()`, `renderValueWithCursor()`, cursor management via rune manipulation). Does NOT use `bubbles/textinput` — consistency with existing modals matters.

### Focus Order (Tab cycle)

```
URL input (1) → Name input (2) → Branch input (3, git only) → Cancel (4) → Add (5) → Source radio (0) → URL input (1)
```

**Default focus on open:** `focusIdx = 1` (URL field). The source radio defaults to Git URL, which is the common case — user can Shift+Tab back to change it.

- When `sourceGit == false` (local dir): Branch input is **skipped** in the Tab cycle
- Up/Down arrows on the source radio toggle between Git URL and Local directory
- Space on the source radio toggles the selection (stays on radio)
- Enter on the source radio selects AND advances to URL field (focusIdx = 1)
- Ctrl+S from any field submits the form (same as pressing Enter on Add button)

### Radio Button Rendering

No radio buttons exist elsewhere in the TUI. Follow the `confirmModal` checkbox pattern:

- **Focused radio group** (`focusIdx == 0`): label text in `accentColor` + bold
- **Selected option**: bullet `(•)` in `accentColor`
- **Unselected option**: bullet `( )` in `mutedColor`
- **Unfocused radio group**: both options in `primaryText`, selected bullet in `primaryText`

### Field Behavior

**URL / Path field:**
- Hand-rolled text input with cursor, backspace, paste support (same as `editModal`)
- Placeholder: `"https://github.com/org/repo"` for git, `"/path/to/registry"` for local
- On every character change (when `nameManuallySet == false`): auto-derive name via `registry.NameFromURL()` for git URLs, or `filepath.Base()` for local paths
- Long URLs that exceed field width are truncated in display (full value in model) — same `renderValueWithCursor` approach as `editModal`

**Name field:**
- Auto-populated from URL (see above)
- Once the user types in this field, set `nameManuallySet = true` — stop auto-derivation
- If user clears the field completely, reset `nameManuallySet = false` and resume auto-derivation

**Branch field:**
- Only shown and focusable when `sourceGit == true`
- Optional — empty means default branch
- Placeholder: `"main"`

### Validation (on Enter/Add press or Ctrl+S)

| Check | Error message |
|-------|---------------|
| URL is empty | `"URL is required"` |
| Git URL uses unsupported protocol (not `https://`, `http://`, `ssh://`, `git@`) | `"Only https://, ssh://, and git@ URLs are supported"` |
| Git URL contains `ext::` | `"Only https://, ssh://, and git@ URLs are supported"` |
| URL not in allowed registries | `"URL not permitted by registry allowlist"` |
| Local path doesn't exist (`os.Stat`) | `"Directory does not exist"` |
| Name is empty (after derivation) | `"Name is required"` |
| Name fails `catalog.IsValidRegistryName()` | `"Invalid name (use letters, numbers, - and _ with optional owner/repo format)"` |
| Name conflicts with existing registry (case-insensitive) | `"Registry {name} already exists"` |
| Branch contains invalid characters (not `^[a-zA-Z0-9._/-]+$`) | `"Branch name can only contain letters, numbers, ., _, / and -"` |

Validation errors display inline below the last field, styled with `dangerColor`. The modal does NOT close on validation failure.

**Name conflict check:** Compare against `existingNames` using `strings.EqualFold()`. Case-insensitive to prevent filesystem collisions on macOS/Windows.

**Note:** `os.Stat` for local path validation is acceptable inline since it's sub-millisecond — intentional exception to the "no I/O in Update" rule.

### Local Path Resolution

Before storing in config, local paths are resolved:
1. `filepath.Abs(path)` — convert to absolute path
2. `filepath.EvalSymlinks(resolvedPath)` — resolve symlinks

The resolved path is what gets stored in config and displayed in the result message.

### Result Message

```go
type registryAddMsg struct {
    url     string
    name    string
    ref     string // branch/tag, may be empty
    isLocal bool
}
```

The modal's `Update()` returns a `tea.Cmd` producing `registryAddMsg` (same pattern as `editModal` → `editSavedMsg`). The modal closes itself before emitting the message.

### Async Flow (in `actions.go`)

**Critical: Never mutate `a.cfg` in the `tea.Cmd`.** The `tea.Cmd` closure must not reference `a.cfg` — that would be a data race (goroutine writes while the main event loop reads). Instead, the closure loads a fresh config copy from disk via `config.LoadGlobal()`, mutates that fresh copy (append/filter), and saves it. The done handler in `Update()` calls `rescanCatalog()` which reloads `a.cfg` from disk on the main goroutine.

On `registryAddMsg` in `App.Update()`:

1. Set `a.registryOpInProgress = true`
2. Show progress toast: `"Adding registry: {name}..."`
3. Return `tea.Cmd` that:
   a. If git URL: calls `registry.Clone(url, name, ref)` — I/O only
   b. If local dir: no clone needed (path already validated)
   c. Loads global config fresh: `cfg, _ := config.LoadGlobal()`
   d. Appends new registry: `cfg.Registries = append(cfg.Registries, config.Registry{Name: name, URL: url, Ref: ref})`
   e. Saves: `config.SaveGlobal(cfg)`
   f. Returns `registryAddDoneMsg{name, err}`

On `registryAddDoneMsg` in `App.Update()`:

1. Set `a.registryOpInProgress = false` (regardless of success/error)
2. If error: show error toast
3. If success: show success toast `"Added registry: {name}"`, call `rescanCatalog()`

**Note on config loading:** The `tea.Cmd` loads global config fresh via `config.LoadGlobal()` to avoid mutating the shared `a.cfg` pointer. `rescanCatalog()` in the done handler reloads config from disk, so `a.cfg` is automatically updated.

```go
type registryAddDoneMsg struct {
    name string
    err  error
}
```

### Click-Away Dismissal

Clicks outside the modal zone `"registry-add-zone"` dismiss the modal (consistent with existing modals per `tui-modals.md`).

### Local Directory Handling

For local directories, there is NO clone step. The directory is used directly as a registry source:
- Config entry: `Registry{Name: name, URL: resolvedAbsPath}` where URL is the resolved absolute local path
- No `Ref` field needed
- The directory must exist and should ideally contain a `registry.yaml`, but this is not required (content is scanned regardless)

---

## Feature 2: Registry Sync (`[S]`)

### Entry Point

- `[S]` (shift+s) keypress when on the **Registries** tab (not drilled in)
- Only when `registryOpInProgress == false`
- New key constant: `keySync = "S"` in `keys.go`

### Behavior

Syncs the currently selected registry card. If no registries exist (empty gallery), `[S]` is a no-op.

No "sync all" — always syncs the selected registry. If users want to sync all, they can press `[S]` on each card.

### Flow

`handleSync()` is called directly from the key handler (no intermediate message type — matches `handleEdit()`, `handleRemove()` pattern):

1. Get selected card from gallery. If nil, return no-op.
2. Set `a.registryOpInProgress = true`
3. Show progress toast: `"Syncing {name}..."`
4. Return `tea.Cmd` that calls `registry.Sync(name)`, returns `registrySyncDoneMsg{name, err}`

On `registrySyncDoneMsg` in `App.Update()`:

1. Set `a.registryOpInProgress = false` (regardless of success/error)
2. If error: show error toast `"Sync failed: {error}"`
3. If success: show success toast `"Synced {name}"`, call `rescanCatalog()`

```go
type registrySyncDoneMsg struct {
    name string
    err  error
}
```

### Key Registration

Add to `keys.go`:
```go
keySync = "S"
```

Handle in `app_update.go` alongside other global keys. Only active when on the Registries tab:

```go
case msg.String() == keySync:
    if a.isRegistriesTab() && !a.registryOpInProgress {
        return a.handleSync()
    }
    if a.registryOpInProgress {
        return a, a.toast.Push("Registry operation in progress", toastWarning)
    }
```

### `isRegistriesTab()` Helper

A new helper on `App`. Note: the topbar tab label is `"Registries"` (plural).

```go
func (a App) isRegistriesTab() bool {
    return a.isGalleryTab() && a.topBar.ActiveTabLabel() == "Registries"
}
```

---

## Feature 3: Registry Remove (`[d]`)

### Entry Point

- `[d]` keypress on a registry card (existing `keyRemove` → `handleRemove()`)
- Only when `registryOpInProgress == false`
- Currently, `handleGalleryCardRemove()` in `actions.go:99` returns no-op for registries with the comment `"Registry remove is Phase C — no-op for now."`

### Behavior

Reuses the existing `confirmModal`:

```
╭─ Remove registry "team/tools"? ──────╮
│                                        │
│  This will delete the local clone.     │
│  Installed content is not affected.    │
│                                        │
│              [Cancel]   [Remove]       │
╰────────────────────────────────────────╯
```

- No checkboxes (unlike item remove)
- Danger-styled (red border)
- Default focus on Cancel (safe default)

### Flow

1. `handleGalleryCardRemove()` checks `tabLabel == "Registries"` (currently a no-op)
2. Opens `confirmModal` with:
   - Title: `Remove registry "{name}"?`
   - Body: `"This will delete the local clone.\nInstalled content is not affected."`
   - confirmLabel: `"Remove"`
   - danger: `true`
   - checks: `nil`
3. Stores registry name using existing `itemName` field: `a.confirm.itemName = card.name`
4. On confirm, the result flows through `handleConfirmResult()`

### Distinguishing Registry Remove from Other Confirms

**No new field on `confirmModal`.** Instead, use the existing `itemName` field to store the registry name, and dispatch based on app context:

```go
func (a App) handleConfirmResult(msg confirmResultMsg) (tea.Model, tea.Cmd) {
    if !msg.confirmed {
        return a, nil
    }

    // Registry remove: on Registries tab with no item (no ContentItem attached)
    if a.isRegistriesTab() && msg.item.Path == "" && msg.itemName != "" {
        a.registryOpInProgress = true
        return a, a.doRegistryRemoveCmd(msg.itemName)
    }

    // ... existing dispatch logic ...
}
```

This keeps `confirmModal` generic — no registry-specific fields.

### Async Remove Flow

`doRegistryRemoveCmd`:
1. Show progress toast: `"Removing registry: {name}..."`
2. `tea.Cmd` (I/O only) that:
   a. Calls `registry.Remove(name)` (deletes local clone)
   b. Loads global config fresh: `cfg, _ := config.LoadGlobal()`
   c. Removes the registry from `cfg.Registries` by name
   d. Calls `config.SaveGlobal(cfg)`
   e. Returns `registryRemoveDoneMsg{name, err}`

On `registryRemoveDoneMsg` in `App.Update()`:
1. Set `a.registryOpInProgress = false` (regardless of success/error)
2. If error: show error toast
3. If success: toast `"Removed registry: {name}"`, call `rescanCatalog()`

```go
type registryRemoveDoneMsg struct {
    name string
    err  error
}
```

---

## Metadata Bar Updates

The gallery metadata bar (`gallery.go:311-370`) needs additional action buttons for the Registries tab:

**Current buttons (all gallery tabs):** `[d] Remove`, `[e] Edit`

**Registries tab should show:** `[a] Add`, `[S] Sync`, `[d] Remove`, `[e] Edit`

Implementation: `renderMetadata()` checks `g.tabLabel` for the value set by `SetCards()`. In `app.go:234`, the call is `a.gallery.SetCards(cards, "Registry")` (singular). So the check is `g.tabLabel == "Registry"`.

**Important distinction:** `isRegistriesTab()` checks topbar label `"Registries"` (plural). Gallery `tabLabel` is `"Registry"` (singular, set by `SetCards`). Use the correct string in each context.

### Button Zone IDs and Mouse Routing

Zone IDs for new buttons: `meta-add`, `meta-sync`. These emit generic `actionPressedMsg` messages (consistent with keyboard routing), not registry-specific messages:

```go
// In gallery.go mouse handler:
if zone.Get("meta-add").InBounds(msg) {
    return g, func() tea.Msg { return actionPressedMsg{action: "add"} }
}
if zone.Get("meta-sync").InBounds(msg) {
    return g, func() tea.Msg { return actionPressedMsg{action: "sync"} }
}
```

Then in `app_update.go`, `actionPressedMsg{action: "sync"}` routes to `handleSync()`.

---

## Helpbar Updates

The help bar should show context-specific hints for the Registries tab:

```
[a] Add  [S] Sync  [d] Remove  [e] Edit  [?] Help  [q] Back
```

This requires updating `currentHints()` in `app.go` to include sync and add hints when on the Registries tab.

---

## `registryOpInProgress` Flag Lifecycle

A boolean flag on `App` that prevents concurrent registry operations.

**Set to `true`:**
- When `registryAddMsg` is handled (async clone starts)
- When `handleSync()` dispatches the sync command
- When `doRegistryRemoveCmd()` dispatches the remove command

**Set to `false`:**
- When `registryAddDoneMsg` is handled (regardless of success or error)
- When `registrySyncDoneMsg` is handled (regardless of success or error)
- When `registryRemoveDoneMsg` is handled (regardless of success or error)

**Blocks (with warning toast `"Registry operation in progress"`):**
- `[a]` (add) — modal doesn't open
- `[S]` (sync) — sync doesn't start
- `[d]` on registry card — confirm doesn't open

---

## Wiring in `app_update.go`

### WindowSizeMsg

```go
case tea.WindowSizeMsg:
    // ... existing code ...
    a.registryAdd.width = msg.Width
    a.registryAdd.height = ch
```

### Key/Mouse Input Capture

Add `registryAddModal` input capture blocks, following the exact pattern of `modal`, `confirm`, and `remove`:

```go
// Key routing (after toast, before global keys):
if a.registryAdd.active {
    if msg.Type == tea.KeyCtrlC {
        return a, tea.Quit
    }
    var cmd tea.Cmd
    a.registryAdd, cmd = a.registryAdd.Update(msg)
    return a, cmd
}

// Mouse routing (after toast):
if a.registryAdd.active {
    var cmd tea.Cmd
    a.registryAdd, cmd = a.registryAdd.Update(msg)
    return a, cmd
}
```

### Overlay Rendering in `app_view.go`

Render after `remove` and before `help` in the overlay stack:

```go
if a.registryAdd.active {
    content = overlayModal(content, a.registryAdd.View(), a.width, a.contentHeight())
}
```

### Message Dispatch

```go
case registryAddMsg:
    return a.handleRegistryAdd(msg)
case registryAddDoneMsg:
    return a.handleRegistryAddDone(msg)
case registrySyncDoneMsg:
    return a.handleSyncDone(msg)
case registryRemoveDoneMsg:
    return a.handleRegistryRemoveDone(msg)
```

---

## Testing Requirements

### Component Tests (`registry_add_test.go`)

| Test | Success Criteria |
|------|-----------------|
| Source toggle: default is git URL | `sourceGit == true` after open |
| Source toggle: space switches to local | `sourceGit == false` after space on radio |
| Source toggle: Enter selects + advances | `focusIdx == 1` (URL field) after Enter on radio |
| Default focus on open is URL field | `focusIdx == 1` after `Open()` |
| URL typing updates name field (git) | Type `https://github.com/acme/tools`, name becomes `acme/tools` |
| URL typing updates name field (local) | Type `/home/user/my-registry`, name becomes `my-registry` |
| Manual name edit stops auto-derivation | Edit name, type more URL, name unchanged |
| Clear name resumes auto-derivation | Clear name field, type URL, name updates again |
| Branch field hidden for local dir | Local dir mode, Tab cycles 5 elements (radio, url, name, cancel, add) |
| Branch field shown for git URL | Git URL mode, Tab cycles 6 elements (radio, url, name, branch, cancel, add) |
| Empty URL rejected | Enter on Add with empty URL → error shown, modal stays open |
| Invalid git URL protocol rejected | `ext::cmd` URL → error `"Only https://, ssh://..."` |
| AllowedRegistries enforcement | URL not in allowlist → error `"URL not permitted..."` |
| IsValidRegistryName rejection | Name `"../evil"` → error inline |
| Duplicate name rejected (case-insensitive) | Existing `"Team-Tools"`, enter `"team-tools"` → error |
| Branch with invalid chars rejected | Enter `"branch name with spaces"` → error shown |
| Enter on Add produces registryAddMsg | Message contains matching `url`, `name`, `ref`, `isLocal` fields |
| Enter on Cancel closes modal | `active == false` |
| Esc closes modal | `active == false` |
| Ctrl+S submits the form | Same as Enter on Add — validation runs, message emitted |
| y/n don't trigger shortcuts | With focus on URL field, pressing 'y' types 'y' (doesn't confirm) |
| Tab cycles all focusable elements | Full cycle returns to start, correct count per mode |
| Local path resolved to absolute | Input `./local-reg`, stored path is absolute |
| Mouse: click Cancel closes | Zone click → modal closes |
| Mouse: click Add submits | Zone click → registryAddMsg emitted |
| Mouse: click outside modal dismisses | Click outside `registry-add-zone` → modal closes |

### App Integration Tests (`app_test.go` additions)

| Test | Success Criteria |
|------|-----------------|
| `[a]` on Registries tab opens add modal | `a.registryAdd.active == true` |
| `[a]` on Loadouts tab does NOT open add modal | No modal opened |
| `[a]` on Library tab does NOT open add modal | No modal (Phase D) |
| `[a]` while `registryOpInProgress` shows toast | Warning toast, modal does NOT open |
| `[S]` on Registries tab triggers sync | Progress toast shown, sync command returned |
| `[S]` on non-Registries tab is no-op | No message |
| `[S]` while `registryOpInProgress` shows toast | Warning toast, sync does NOT start |
| `[d]` on registry card opens confirm | Confirm modal opens with registry name in `itemName` |
| `[d]` on registry card while `registryOpInProgress` shows toast | Warning toast |
| Confirm registry remove → removes + rescan | `registryRemoveDoneMsg`, catalog refreshed |
| Cancel registry remove → no change | No removal |
| Registry add success → toast + rescan | Progress toast → success toast, catalog refreshed |
| Registry add error → error toast + flag cleared | Error toast, `registryOpInProgress == false` |
| Sync success → toast + rescan | Toast, catalog refreshed |
| Sync error → error toast + flag cleared | Error toast, `registryOpInProgress == false` |
| `registryOpInProgress` cleared on add done (success) | Flag is `false` |
| `registryOpInProgress` cleared on add done (error) | Flag is `false` |
| `registryOpInProgress` cleared on sync done (error) | Flag is `false` |
| `registryOpInProgress` cleared on remove done (error) | Flag is `false` |
| `[e]` on registry card opens edit modal | Edit modal opens with card name + description |

### Golden Files

| File | Description |
|------|-------------|
| `registry-add-modal-git-80x30.golden` | Add modal in git URL mode |
| `registry-add-modal-local-80x30.golden` | Add modal in local dir mode |
| `registry-add-modal-error-80x30.golden` | Add modal with validation error |
| `gallery-registries-buttons-80x30.golden` | Registries metadata bar with [a] Add, [S] Sync buttons |

---

## File Changes

| File | Changes |
|------|---------|
| `registry_add.go` (NEW) | `registryAddModal` struct, `Open()`, `Close()`, `Update()` (value receiver), `View()`, message types, hand-rolled text input, radio rendering, validation |
| `registry_add_test.go` (NEW) | Component tests for the modal |
| `actions.go` | Add `handleRegistryAdd()`, `handleSync()`, `doRegistryAddCmd()`, `doRegistryRemoveCmd()`, `handleRegistryAddDone()`, `handleSyncDone()`, `handleRegistryRemoveDone()`. Update `handleGalleryCardRemove()` for registries. Update `handleConfirmResult()` for registry remove dispatch. |
| `app.go` | Add `registryAdd registryAddModal` field, `registryOpInProgress bool` flag, `isRegistriesTab()` helper |
| `app_update.go` | Wire `registryAddMsg`, `registryAddDoneMsg`, `registrySyncDoneMsg`, `registryRemoveDoneMsg`. Add `registryAdd` to WindowSizeMsg, key capture, mouse capture. Add `keySync` handling. Add `"sync"` case to `actionPressedMsg`. |
| `app_view.go` | Overlay rendering for `registryAdd` (after remove, before help) |
| `keys.go` | Add `keySync = "S"` |
| `gallery.go` | Add `[a] Add` and `[S] Sync` buttons + zone IDs to metadata bar when `g.tabLabel == "Registry"`. Add mouse handlers for `meta-add` and `meta-sync` zones. |
| `app_test.go` | Integration tests for all three features |
| `testdata/*.golden` | New golden files |

---

## Implementation Tasks (Preliminary)

**C1: Registry Add Modal component** — `registry_add.go` + `registry_add_test.go`
- Model struct with hand-rolled text input (following `editModal` pattern)
- Radio button rendering (following `confirmModal` checkbox pattern)
- Focus cycling with branch field skip for local dir mode
- Name auto-derivation from URL/path
- Full validation suite (URL protocol, allowlist, name validity, case-insensitive conflict, branch chars)
- Local path resolution (Abs + EvalSymlinks)
- Component-level tests

**C2: Wiring + Integration** — Wire all three features into the app
- `[a]` → opens registry add modal (with `registryOpInProgress` guard)
- `[S]` → sync selected registry (direct call, no intermediate message)
- `[d]` on registry card → confirm modal using `itemName` field (no new confirm fields)
- `registryOpInProgress` flag lifecycle (set/clear in all paths including errors)
- Async commands: I/O only in `tea.Cmd`, config mutation via fresh `LoadGlobal()` in cmd
- Completion handlers: clear flag, show toast, `rescanCatalog()`
- WindowSizeMsg + key/mouse capture + overlay rendering wiring
- Metadata bar buttons for Registries tab (zone IDs, `actionPressedMsg` routing)
- Helpbar hints
- Integration tests + golden files

---

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| No registries configured | Gallery shows "No items found". `[S]` is no-op. `[d]` is no-op. `[a]` still works. |
| Git not installed | Clone/sync fail with "git is required..." error. Toast shows error. |
| Clone timeout | Git clone blocks (handled by OS-level timeout). Future: add `context.WithTimeout`. For Phase C, rely on git's own timeout. |
| Local dir deleted after adding | `rescanCatalog()` will show 0 items for that registry. Gallery card still appears. User can remove. |
| Adding same URL with different name | Allowed — config has no uniqueness constraint on URL, only on name. |
| Registry with no `registry.yaml` | Works fine — content is scanned from directory. Card shows name from config, no description. |
| `[S]` during active operation | Blocked by `registryOpInProgress` flag. Warning toast shown. |
| `[a]` during active operation | Blocked by `registryOpInProgress` flag. Warning toast shown. |
| `[d]` during active operation | Blocked by `registryOpInProgress` flag. Warning toast shown. |
| `[e]` on registry card | Opens `editModal` with card name + description. Saves to `.syllago.yaml` in registry dir. Already works via `handleEdit()`. |
| `ext::` or `file://` URL | Rejected by URL protocol validation with clear error message. |
| URL not in AllowedRegistries | Rejected with `"URL not permitted by registry allowlist"`. Empty allowlist = all URLs permitted. |
| rescanCatalog fails after successful add | The add itself succeeded (config saved). User sees success toast. Can press `R` to manually retry catalog refresh. |

---

## Expert Panel Review Summary

Spec reviewed by 4 domain experts (TUI UX, Go/BubbleTea, Security, Design System). Key findings addressed in this v2:

- **M1:** Config mutation moved out of `tea.Cmd` into `Update()` handlers (race condition fix)
- **M2:** Added `ext::` / unsupported protocol URL validation (command injection prevention)
- **M3:** Added `IsRegistryAllowed()` enforcement (security control)
- **M4:** Added WindowSizeMsg + key/mouse routing + overlay wiring details
- **M5:** Specified `registryOpInProgress` full lifecycle + user feedback toast
- **M6:** Clarified "Registries" (topbar) vs "Registry" (gallery tabLabel) distinction
- **M7:** Switched from `bubbles/textinput` to hand-rolled text input (consistency)
- **M8:** Specified radio button rendering (following checkbox pattern)
- **M9:** Specified modal width, Enter/Space behavior on radio, Ctrl+S shortcut
