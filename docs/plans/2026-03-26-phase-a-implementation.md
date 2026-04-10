# Phase A Implementation Plan: Confirm Modal + Per-Item Actions

**Date:** 2026-03-26
**Bead:** syllago-kopxr
**Design ref:** docs/plans/2026-03-26-tui-wizards-design.md (Phase A, W3)

## Overview

Phase A adds destructive action support to the TUI: a reusable `confirmModal` component, `[d]` Remove and `[x]` Uninstall hotkeys, updated help hints, and hiding the deferred `[n]` Create button. This is the foundation for all future wizard work.

---

## Task 1: Build confirmModal Component

**Bead:** syllago-ogh65
**Files:** `cli/internal/tui/confirm.go` (new), `cli/internal/tui/confirm_test.go` (new)

### Data Model

```go
// confirmCheckbox represents a toggleable checkbox in the confirm modal.
type confirmCheckbox struct {
    label    string
    checked  bool
    readOnly bool // true = always checked, visually distinct, not toggleable
}

// confirmModal is a generic yes/no overlay with optional checkboxes.
type confirmModal struct {
    active   bool
    title    string       // e.g., "Remove \"my-hook\"?"
    body     string       // context message (multi-line OK)
    danger   bool         // red border when true
    confirm  string       // confirm button label (e.g., "Remove", "Uninstall")
    checks   []confirmCheckbox
    focusIdx int          // 0..len(checks)-1 = checkboxes, len(checks) = Cancel, len(checks)+1 = Confirm
    width    int
    height   int

    // Caller context — passed through to result messages untouched.
    item               catalog.ContentItem   // the item being acted on
    itemName           string                // display name for toast messages
    uninstallProviders []provider.Provider   // only for uninstall flow
}
```

### Focus Order

Focus indices:
- `0` to `len(checks)-1` = checkboxes (if any)
- `len(checks)` = Cancel button (default focus on open)
- `len(checks)+1` = Confirm button

Tab/Shift+Tab cycles through all focusable elements (wrapping). Space toggles the focused checkbox (no-op on readOnly checkboxes and buttons). Enter fires only on buttons.

### Key Shortcuts

| Key | Behavior |
|-----|----------|
| `y` | Confirm (from any focus position) |
| `n` | Cancel (from any focus position) |
| `Esc` | Cancel |
| `Tab` / `Shift+Tab` | Cycle focus |
| `Space` | Toggle focused checkbox (skip readOnly) |
| `Enter` | Fire focused button (Cancel or Confirm) |
| `j` / `k` / `Up` / `Down` | Move focus up/down |

### Methods

```go
func newConfirmModal() confirmModal

// Open activates the modal. Default focus on Cancel.
func (m *confirmModal) Open(title, body, confirmLabel string, danger bool, checks []confirmCheckbox)

// OpenForItem is a convenience that also stores item context for the result message.
func (m *confirmModal) OpenForItem(title, body, confirmLabel string, danger bool, checks []confirmCheckbox, item catalog.ContentItem)

func (m *confirmModal) Close()
func (m confirmModal) Update(msg tea.Msg) (confirmModal, tea.Cmd)
func (m confirmModal) View() string
```

### Messages

```go
// confirmResultMsg carries the user's decision back to App.
type confirmResultMsg struct {
    confirmed bool                // true = user pressed Confirm/y
    checks    []confirmCheckbox   // final checkbox state
    item      catalog.ContentItem // passed through from open
    itemName  string              // passed through from open
}
```

A single message type for both confirm and cancel simplifies the App handler — just check `confirmed`.

### View Layout

```
+-- Title --------------------------------+
|                                          |
|  Body text line 1                        |
|  Body text line 2                        |
|                                          |
|  [x] Checkbox 1                          |   (only if checks present)
|  [x] Checkbox 2 (read-only, dimmed)      |
|                                          |
|             [Cancel]   [Confirm]         |
+------------------------------------------+
```

- Border: `dangerColor` when `danger=true`, `accentColor` otherwise (matches editModal pattern)
- Cancel button: default style. Confirm button: danger-styled when `danger=true`
- Focused element gets `accentColor` highlight (same as editModal focus pattern)
- ReadOnly checkboxes render with `mutedColor` text and a locked appearance
- Max width: 50 chars or terminal width - 10, whichever is smaller
- Vertically centered via `overlayModal()` (existing function)

### Mouse Support

Zone IDs: `confirm-check-N`, `confirm-cancel`, `confirm-ok`
- Click checkbox → toggle (unless readOnly)
- Click Cancel → cancel
- Click Confirm → confirm

### Success Criteria

1. Modal opens/closes correctly, captures all input when active
2. Default focus on Cancel (safe default)
3. `y`/`n` shortcuts work from any focus position
4. Tab cycles: checkboxes → Cancel → Confirm → checkboxes (wrap)
5. Space toggles checkboxes, skips readOnly
6. Enter on Cancel → `confirmResultMsg{confirmed: false}`
7. Enter on Confirm → `confirmResultMsg{confirmed: true, checks: [final state]}`
8. Esc → `confirmResultMsg{confirmed: false}`
9. Danger border renders when `danger=true`
10. ReadOnly checkbox always checked, visually distinct, not toggleable
11. Mouse clicks work for checkboxes and buttons
12. Inactive modal ignores all input

---

## Task 2: Wire [d] Remove Action

**Bead:** syllago-d5e3g
**Files:** `cli/internal/tui/app.go` (modify), `cli/internal/tui/keys.go` (modify)

### Key Binding

Add to `keys.go`:
```go
keyRemove    = "d"
keyUninstall = "x"
```

### App Struct Change

Add confirm modal field to App (value type, same as editModal — it's small):
```go
type App struct {
    // ...existing fields...
    modal    editModal      // reusable edit overlay (name + description)
    confirm  confirmModal   // reusable confirm overlay (remove/uninstall)
    // ...
}
```

Initialize in `NewApp()`:
```go
confirm: newConfirmModal(),
```

### Update() Routing

Add confirm modal capture block right after the editModal block (app.go ~line 120):

```go
if a.confirm.active {
    if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyCtrlC {
        return a, tea.Quit
    }
    var cmd tea.Cmd
    a.confirm, cmd = a.confirm.Update(msg)
    return a, cmd
}
```

Add `confirmResultMsg` message handler after editCancelledMsg (~line 267):

```go
case confirmResultMsg:
    return a.handleConfirmResult(msg)
```

### handleRemove()

Called when user presses `[d]`:

```go
func (a App) handleRemove() (tea.Model, tea.Cmd) {
    // Determine selected item based on context (same pattern as handleEdit)
    item := a.selectedItem()
    if item == nil {
        return a, nil
    }

    // Phase A only supports library item remove. Registry remove (Phase C) and
    // other non-library items are no-ops for now — silently ignore rather than
    // showing a toast, since the hotkey hint only appears in relevant contexts.
    if !item.Library {
        return a, nil
    }

    // Check which providers have this item installed
    var checks []confirmCheckbox
    for _, prov := range a.providers {
        if installer.CheckStatus(*item, prov, a.catalog.RepoRoot) == installer.StatusInstalled {
            checks = append(checks, confirmCheckbox{
                label:   "Uninstall from " + prov.Name,
                checked: true,
            })
        }
    }
    // Always-checked "delete from library" checkbox
    checks = append(checks, confirmCheckbox{
        label:    "Delete from library",
        checked:  true,
        readOnly: true,
    })

    body := "This cannot be undone."
    if len(checks) > 1 {
        body = fmt.Sprintf("This item is installed in %d provider(s).\n\n%s", len(checks)-1, body)
    }

    a.confirm.OpenForItem(
        fmt.Sprintf("Remove %q?", item.DisplayName),
        body,
        "Remove",
        true, // danger
        checks,
        *item,
    )
    return a, nil
}
```

### handleConfirmResult()

```go
func (a App) handleConfirmResult(msg confirmResultMsg) (tea.Model, tea.Cmd) {
    if !msg.confirmed {
        return a, nil // user cancelled
    }

    // Determine action based on which confirm was active.
    // Check if this is a remove (has "Delete from library" checkbox) or uninstall.
    isRemove := false
    for _, c := range msg.checks {
        if c.readOnly && c.label == "Delete from library" {
            isRemove = true
            break
        }
    }

    if isRemove {
        return a, a.doRemoveCmd(msg)
    }
    return a, a.doUninstallCmd(msg)
}
```

### doRemoveCmd() — Async Backend

All backend calls are `tea.Cmd` (design rule: no blocking I/O in Update):

```go
type removeDoneMsg struct {
    itemName        string
    uninstalledFrom []string
    err             error
}

func (a App) doRemoveCmd(msg confirmResultMsg) tea.Cmd {
    // Snapshot what we need — closures must not capture App.
    item := msg.item
    providers := a.providers
    repoRoot := a.catalog.RepoRoot

    // Build list of providers to uninstall from based on checked checkboxes.
    var targetProviders []provider.Provider
    for _, c := range msg.checks {
        if !c.readOnly && c.checked {
            provName := strings.TrimPrefix(c.label, "Uninstall from ")
            for _, prov := range providers {
                if prov.Name == provName {
                    targetProviders = append(targetProviders, prov)
                }
            }
        }
    }

    return func() tea.Msg {
        var uninstalledFrom []string

        // Uninstall from selected providers
        for _, prov := range targetProviders {
            if _, err := installer.Uninstall(item, prov, repoRoot); err != nil {
                // Log warning but continue — remove should still proceed
                continue
            }
            uninstalledFrom = append(uninstalledFrom, prov.Name)
        }

        // Delete from library (os.RemoveAll returns nil if path doesn't exist)
        if err := os.RemoveAll(item.Path); err != nil {
            return removeDoneMsg{itemName: item.Name, err: fmt.Errorf("removing from library: %w", err)}
        }

        return removeDoneMsg{itemName: item.Name, uninstalledFrom: uninstalledFrom}
    }
}
```

The item is passed through `confirmResultMsg.item` (set by `OpenForItem`), avoiding
any need to re-scan the catalog from within the `tea.Cmd` goroutine.

### removeDoneMsg Handler

```go
case removeDoneMsg:
    if msg.err != nil {
        cmd := a.toast.Push("Remove failed: "+msg.err.Error(), toastError)
        return a, cmd
    }
    // Re-scan catalog and rebuild views
    cmd := a.rescanCatalog()
    toastMsg := fmt.Sprintf("Removed %q", msg.itemName)
    if len(msg.uninstalledFrom) > 0 {
        toastMsg += " (uninstalled from " + strings.Join(msg.uninstalledFrom, ", ") + ")"
    }
    cmd2 := a.toast.Push(toastMsg, toastSuccess)
    return a, tea.Batch(cmd, cmd2)
```

### selectedItem() Helper

Extract the common "get focused item" logic into a shared method. Note: gallery cards
return `*cardData`, not `*catalog.ContentItem`, so the gallery case needs special handling.
The `handleEdit()` function already has a separate gallery code path for this reason.

```go
// selectedItem returns the currently focused content item, or nil.
// For gallery cards (Loadouts/Registries) that aren't drilled-in, returns nil —
// callers must check isGalleryTab() separately for card-level actions.
func (a App) selectedItem() *catalog.ContentItem {
    if a.isGalleryTab() && a.galleryDrillIn {
        return a.library.selectedItem()
    }
    if a.isGalleryTab() {
        // Gallery cards are cardData, not ContentItem. Card-level actions
        // (loadout remove, registry remove) are handled separately.
        return nil
    }
    if a.isLibraryTab() {
        return a.library.selectedItem()
    }
    return a.explorer.selectedItem()
}
```

Library has `selectedItem()` via `table.Selected()`. Explorer has it via `items.Selected()`.

### Gallery Card Remove (Loadouts)

The `handleRemove()` method needs a separate branch for gallery cards. Loadout remove
is simpler per design doc: just delete from disk, no provider side effects.

```go
// In handleRemove(), before the selectedItem() call:
if a.isGalleryTab() && !a.galleryDrillIn {
    card := a.gallery.selectedCard()
    if card == nil {
        return a, nil
    }
    tabLabel := a.topBar.ActiveTabLabel()
    if tabLabel == "Loadouts" {
        // Loadout remove: simple delete, no provider checkboxes
        a.confirm.Open(
            fmt.Sprintf("Remove loadout %q?", card.name),
            "This will delete the loadout from disk.\nThis cannot be undone.",
            "Remove",
            true, // danger
            nil,  // no checkboxes
        )
        a.confirm.item = loadoutCardToContentItem(card)
        a.confirm.itemName = card.name
        return a, nil
    }
    // Registry remove is Phase C — no-op for now
    return a, nil
}
```

`loadoutCardToContentItem` is a small helper that constructs a minimal `ContentItem`
from the card data (path + name + type=Loadouts), sufficient for the `os.RemoveAll` call.
This avoids needing a full catalog lookup.

### Success Criteria

1. `[d]` on a library item opens confirmModal with correct title, checkboxes
2. `[d]` on an installed item shows provider uninstall checkboxes (all pre-checked)
3. `[d]` on a non-installed item shows simplified confirm (only "Delete from library")
4. `[d]` on a non-library item is a silent no-op (registry/project items deferred to later phases)
5. `[d]` on a loadout card opens simplified confirm (no checkboxes, just delete)
6. Confirm → item removed from disk, catalog re-scanned, toast shown
7. Confirm with providers checked → uninstalls from those providers first, then removes
8. Cancel → no changes, modal closes
9. Toast message includes provider names when uninstalled

---

## Task 3: Wire [x] Uninstall Action

**Bead:** syllago-a3vvt (shared with Task 4 help update)
**Files:** `cli/internal/tui/app.go` (modify)

### handleUninstall()

```go
func (a App) handleUninstall() (tea.Model, tea.Cmd) {
    // [x] Uninstall not available on gallery cards (Loadouts/Registries).
    // Design doc action map: no Uninstall column for Loadouts or Registries.
    if a.isGalleryTab() && !a.galleryDrillIn {
        return a, nil
    }

    item := a.selectedItem()
    if item == nil {
        return a, nil
    }

    // Find which providers have this installed
    var installedProviders []provider.Provider
    for _, prov := range a.providers {
        if installer.CheckStatus(*item, prov, a.catalog.RepoRoot) == installer.StatusInstalled {
            installedProviders = append(installedProviders, prov)
        }
    }

    if len(installedProviders) == 0 {
        cmd := a.toast.Push("Not installed in any provider", toastWarning)
        return a, cmd
    }

    // If installed in exactly one provider, simple confirm
    if len(installedProviders) == 1 {
        prov := installedProviders[0]
        a.confirm.OpenForItem(
            fmt.Sprintf("Uninstall %q?", item.DisplayName),
            fmt.Sprintf("Remove from: %s\nContent stays in your library.", prov.Name),
            "Uninstall",
            false, // not danger — content stays in library
            nil,   // no checkboxes for simple uninstall
            *item,
        )
        // Store provider info for the uninstall command
        a.confirm.uninstallProviders = installedProviders
        return a, nil
    }

    // Multiple providers — show checkboxes
    var checks []confirmCheckbox
    for _, prov := range installedProviders {
        checks = append(checks, confirmCheckbox{
            label:   "Uninstall from " + prov.Name,
            checked: true,
        })
    }

    a.confirm.OpenForItem(
        fmt.Sprintf("Uninstall %q?", item.DisplayName),
        "Content stays in your library.",
        "Uninstall",
        false,
        checks,
        *item,
    )
    a.confirm.uninstallProviders = installedProviders
    return a, nil
}
```

The `uninstallProviders` field is already on the `confirmModal` struct (see Task 1 Data Model).
Set it after `OpenForItem` — it's only used as context for the async command, not for rendering.

### doUninstallCmd()

```go
type uninstallDoneMsg struct {
    itemName        string
    uninstalledFrom []string
    err             error
}

func (a App) doUninstallCmd(msg confirmResultMsg) tea.Cmd {
    item := msg.item
    providers := a.confirm.uninstallProviders
    repoRoot := a.catalog.RepoRoot

    // If no checkboxes, uninstall from all providers in the list
    // If checkboxes, only uninstall from checked ones
    var targetProviders []provider.Provider
    if len(msg.checks) == 0 {
        targetProviders = providers
    } else {
        for i, c := range msg.checks {
            if c.checked && i < len(providers) {
                targetProviders = append(targetProviders, providers[i])
            }
        }
    }

    return func() tea.Msg {
        var uninstalledFrom []string
        var lastErr error
        for _, prov := range targetProviders {
            if _, err := installer.Uninstall(item, prov, repoRoot); err != nil {
                lastErr = err
            } else {
                uninstalledFrom = append(uninstalledFrom, prov.Name)
            }
        }
        if lastErr != nil && len(uninstalledFrom) == 0 {
            return uninstallDoneMsg{itemName: item.Name, err: lastErr}
        }
        return uninstallDoneMsg{itemName: item.Name, uninstalledFrom: uninstalledFrom}
    }
}
```

### uninstallDoneMsg Handler

```go
case uninstallDoneMsg:
    if msg.err != nil {
        cmd := a.toast.Push("Uninstall failed: "+msg.err.Error(), toastError)
        return a, cmd
    }
    cmd := a.rescanCatalog()
    toastMsg := fmt.Sprintf("Uninstalled %q from %s", msg.itemName, strings.Join(msg.uninstalledFrom, ", "))
    cmd2 := a.toast.Push(toastMsg, toastSuccess)
    return a, tea.Batch(cmd, cmd2)
```

### Key Routing in Update()

Add to the global key switch in app.go (~line 233-242):

```go
case msg.String() == keyRemove:
    return a.handleRemove()

case msg.String() == keyUninstall:
    return a.handleUninstall()
```

### Success Criteria

1. `[x]` on an installed item (single provider) opens simple confirm with provider name
2. `[x]` on an installed item (multi-provider) opens confirm with checkboxes
3. `[x]` on a non-installed item shows toast "Not installed in any provider"
4. Confirm → item uninstalled, catalog re-scanned, toast shows provider name(s)
5. Cancel → no changes
6. Content stays in library after uninstall (explicitly stated in body text)
7. Partial uninstall (some providers fail) → toast shows successes, error for failures

---

## Task 4: Update Help Overlay and Helpbar Hints

**Bead:** syllago-a3vvt (shared with Task 3)
**Files:** `cli/internal/tui/help.go` (modify), `cli/internal/tui/app.go` (modify)

### Help Overlay Updates

Add to the "Actions" section in help.go View() (~line 96-103):

```go
section("Actions", [][2]string{
    {"a", "Add content"},
    {"n", "Create new"},          // keep in help even though button hidden
    {"e", "Edit name/description"},
    {"d", "Remove from library"},  // NEW
    {"x", "Uninstall from provider"}, // NEW
    {"/", "Search"},
    {"s / S", "Sort / reverse sort"},
    {"R", "Refresh catalog"},
}),
```

### Helpbar Hint Updates

Update `currentHints()` in app.go to include `d` and `x` in relevant contexts:

**Library browse:** Replace `"a add", "n create"` with `"e edit", "d remove", "x uninstall", "a add"`
**Gallery drill-in browse:** Add `"d remove"`, `"x uninstall"`
**Explorer browse:** Add `"d remove"`, `"x uninstall"`

The exact hint string set per context:

| Context | Hints (appended to base) |
|---------|--------------------------|
| Library browse | navigate, preview, search, sort, edit, **remove**, **uninstall**, refresh, add, help, quit |
| Library detail | navigate, switch pane, close, refresh, help, quit |
| Gallery drill-in browse | navigate, preview, search, sort, edit, **remove**, **uninstall**, refresh, help, back |
| Gallery cards | arrows, select, tab, edit, **remove**, refresh, add, help, back |
| Explorer browse | navigate, switch pane, detail, edit, **remove**, **uninstall**, refresh, add, help, quit |
| Explorer detail | navigate, switch pane, close, edit, refresh, help, back |
| Config | refresh, help, quit |

**Note:** Help hints for `d` and `x` use short labels: `"d remove"`, `"x uninstall"`.

### Success Criteria

1. Help overlay shows `d` and `x` in Actions section
2. Helpbar shows `d remove` and `x uninstall` in browse contexts
3. Helpbar does NOT show `d`/`x` in detail mode or Config tab
4. Hint ordering feels natural (edit, remove, uninstall grouped near each other)

---

## Task 5: Hide [n] Create Button

**Bead:** syllago-d5e3g (shared with Task 2)
**Files:** `cli/internal/tui/topbar.go` (modify), `cli/internal/tui/app.go` (modify)

### Changes

1. **topbar.go:** Remove the `[n] Create` button from the sub-tab row rendering. The `keyCreate` constant stays in keys.go (it's still a defined hotkey, just not shown).

2. **app.go key routing:** Remove or comment out the `keyCreate` case in the global key switch (~line 236-237). The `n` key should be a no-op.

3. **Help overlay:** Keep `n` in help.go but add "(coming soon)" suffix:
   ```go
   {"n", "Create new (coming soon)"},
   ```

4. **Helpbar hints:** Remove `"n create"` from all hint arrays in `currentHints()`.

### Success Criteria

1. `[n] Create` button no longer appears in the topbar sub-tab row
2. Pressing `n` does nothing (no crash, no action)
3. Help overlay still lists `n` with "(coming soon)" note
4. Helpbar no longer shows `n create`

---

## Shared Infrastructure

### Imports

**confirm.go:** `fmt`, `strings`, `bubbletea`, `lipgloss`, `bubblezone`, `catalog`, `provider`
**app.go (new):** `installer` (for `CheckStatus`/`Uninstall` calls in handlers)

### Canonical Types (Single Source of Truth)

The `confirmModal` struct and `confirmResultMsg` type defined in Task 1's Data Model
section are the canonical definitions. The `uninstallProviders` field was added there
to keep all context on a single struct. No separate "final" definition needed.

---

## Test Plan

### confirm_test.go — Component Tests

| Test | Description | Success Criteria |
|------|-------------|------------------|
| `TestConfirmModal_OpenClose` | Open sets fields, Close resets | active=true/false, fields set/cleared |
| `TestConfirmModal_DefaultFocusCancel` | Open → focus is on Cancel | focusIdx == len(checks) |
| `TestConfirmModal_YShortcut` | Press `y` → confirmed | confirmResultMsg{confirmed: true} |
| `TestConfirmModal_NShortcut` | Press `n` → cancelled | confirmResultMsg{confirmed: false} |
| `TestConfirmModal_EscCancels` | Press Esc → cancelled | confirmResultMsg{confirmed: false} |
| `TestConfirmModal_EnterOnCancel` | Focus Cancel, Enter → cancelled | confirmResultMsg{confirmed: false} |
| `TestConfirmModal_EnterOnConfirm` | Tab to Confirm, Enter → confirmed | confirmResultMsg{confirmed: true} |
| `TestConfirmModal_TabCycle` | Tab wraps through all focus positions | Full cycle verified |
| `TestConfirmModal_SpaceTogglesCheckbox` | Tab to checkbox, Space → toggled | checked flipped |
| `TestConfirmModal_SpaceSkipsReadOnly` | Tab to readOnly checkbox, Space → no change | checked remains true |
| `TestConfirmModal_EnterOnCheckboxNoOp` | Tab to checkbox, Enter → no confirm | No message produced |
| `TestConfirmModal_CheckboxStateInResult` | Toggle some, confirm → result has final state | checks reflect toggles |
| `TestConfirmModal_DangerStyling` | Open with danger=true → View has red border | ANSI red in output |
| `TestConfirmModal_InactiveIgnoresInput` | Send keys when !active → no messages | No cmd returned |
| `TestConfirmModal_NoCheckboxes` | Open with nil checks → Cancel+Confirm only | focusIdx 0=Cancel, 1=Confirm |
| `TestConfirmModal_MouseClickCheckbox` | Click checkbox zone → toggled | checked flipped |
| `TestConfirmModal_MouseClickCancel` | Click Cancel zone → cancelled | confirmResultMsg{confirmed: false} |
| `TestConfirmModal_MouseClickConfirm` | Click Confirm zone → confirmed | confirmResultMsg{confirmed: true} |
| `TestConfirmModal_AllProviderUnchecked` | Uncheck all provider checkboxes, confirm → valid (library-only remove) | result has all provider checks=false, readOnly check=true |

### app_confirm_test.go — Integration Tests

| Test | Description | Success Criteria |
|------|-------------|------------------|
| `TestApp_RemoveOpensConfirm` | Press `d` on library item → confirm opens | confirm.active=true, title contains item name |
| `TestApp_RemoveInstalledShowsCheckboxes` | `d` on installed item → checkboxes for providers | len(checks) > 1 |
| `TestApp_RemoveNotInstalledSimple` | `d` on non-installed item → only "Delete" checkbox | len(checks) == 1 |
| `TestApp_RemoveNonLibraryToast` | `d` on non-library item → warning toast | toast contains "not a library item" |
| `TestApp_UninstallOpensConfirm` | `x` on installed item → confirm opens | confirm.active=true |
| `TestApp_UninstallNotInstalledToast` | `x` on non-installed item → warning toast | toast contains "Not installed" |
| `TestApp_UninstallMultiProvider` | `x` on multi-installed → checkboxes | len(checks) > 0 |
| `TestApp_ConfirmCancelNoChanges` | Open confirm, press Esc → no changes | catalog unchanged |
| `TestApp_RemoveConfirmSuccess` | Open remove confirm, press `y` → removeDoneMsg sent | tea.Cmd returns removeDoneMsg with item name |
| `TestApp_UninstallConfirmSuccess` | Open uninstall confirm, press `y` → uninstallDoneMsg sent | tea.Cmd returns uninstallDoneMsg with provider name |
| `TestApp_LoadoutCardRemove` | `d` on loadout card → simplified confirm (no checkboxes) | confirm.active=true, no checks, title contains loadout name |
| `TestApp_UninstallOnGalleryCardNoOp` | `x` on loadout/registry card → no action | No modal opens |
| `TestApp_RemoveNonLibraryNoOp` | `d` on non-library item → silent no-op | No modal, no toast |
| `TestApp_CreateKeyNoOp` | Press `n` → no action, no crash | No modal opens, no command |
| `TestApp_CreateButtonHidden` | Render topbar → no `[n]` visible | View doesn't contain "[n]" |

### Golden File Tests

| Golden | Description | Size |
|--------|-------------|------|
| `confirm-simple-80x30` | Uninstall confirm, no checkboxes | 80x30 |
| `confirm-danger-80x30` | Remove confirm, danger border | 80x30 |
| `confirm-checkboxes-80x30` | Remove with provider checkboxes | 80x30 |
| `confirm-simple-60x20` | Uninstall confirm, minimum size | 60x20 |
| `confirm-danger-60x20` | Remove confirm, minimum size | 60x20 |
| `confirm-checkboxes-60x20` | Remove with checkboxes, minimum size | 60x20 |

---

## Implementation Order

Tasks execute in strict sequential order with validation gates between each:

```
Impl Task 1 (confirmModal)
  └── Validate Task 1 (different sub-agent)
      └── Impl Task 5 (hide [n] Create)
          └── Validate Task 5
              └── Impl Task 2 (wire [d] Remove)
                  └── Validate Task 2
                      └── Impl Task 3 (wire [x] Uninstall)
                          └── Validate Task 3
                              └── Impl Task 4 (help + helpbar)
                                  └── Validate Task 4
                                      └── Phase Validation
```

**Rules:**
- Each validation bead BLOCKS the next implementation bead.
- Validation is performed by a DIFFERENT sub-agent than the implementer.
- If validation fails, fix before moving to the next task.
- No parallel execution — a validation failure on Task 1 may change Task 2's approach.

---

## Design Decisions and Rationale

**Why value type for confirmModal (not pointer)?**
The modal is small (a few strings, a short slice of checkboxes). Value copy overhead is negligible. Pointer fields are reserved for large wizard models per the design doc. Keeping it as a value matches the existing editModal pattern.

**Why pass item through confirmResultMsg instead of re-scanning?**
The `tea.Cmd` closure runs in a goroutine and shouldn't reference `App` fields (BubbleTea semantics). Passing the item through the modal avoids a redundant catalog scan and is deterministic.

**Why a single confirmResultMsg type instead of separate confirmed/cancelled?**
Reduces the number of message types. The `confirmed` bool is checked once in `handleConfirmResult`. The editModal uses separate types (editSavedMsg/editCancelledMsg), but that predates the wizard design. For new code, the single-message pattern is cleaner.

**Why `uninstallProviders` on the modal struct?**
The uninstall flow needs to know which providers to target. We can't derive this from checkbox labels reliably (provider names might contain "Uninstall from " as a prefix but this is fragile). Storing the provider slice directly is explicit and safe.

**Why `d` not `r` for remove?**
Documented in design doc: `d` is universal convention (vim, file managers), avoids confusion with `R` refresh.
