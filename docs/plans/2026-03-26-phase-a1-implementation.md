# Phase A.1 Implementation Plan

**Date:** 2026-03-26
**Bead:** syllago-y6bx5
**Design ref:** docs/plans/2026-03-26-phase-a1-design.md

## Overview

Four changes: (1) multi-step Remove modal, (2) action buttons on sub-tab row,
(3) left/right button navigation, (4) title wrapping fix.

---

## Task 1: Multi-Step Remove Modal

**Files:** `cli/internal/tui/confirm.go` (rewrite), `cli/internal/tui/confirm_test.go` (rewrite),
`cli/internal/tui/actions.go` (modify handleRemove, handleConfirmResult, doRemoveCmd)

### Data Model Changes

Replace the flat `confirmModal` with a step-based model:

```go
type removeStep int
const (
    removeStepConfirm  removeStep = iota // Step 1: confirm removal
    removeStepProviders                   // Step 2: select providers to uninstall
    removeStepReview                      // Step 3: review and execute
)

type removeModal struct {
    active   bool
    step     removeStep
    width    int
    height   int

    // Item context
    item     catalog.ContentItem
    itemName string

    // Provider data (computed on open)
    installedProviders []provider.Provider  // providers where item is installed
    providerChecks     []bool              // parallel array: selected for uninstall

    // Step 1 focus: 0=Cancel, 1=Remove/RemoveOnly, 2=Yes (only when installed)
    // Step 2 focus: 0..N-1=checkboxes, N=Back, N+1=Done
    // Step 3 focus: 0=Cancel, 1=Back, 2=Remove
    focusIdx int
}
```

The existing `confirmModal` stays for Uninstall (simple yes/no confirm). Remove
gets its own dedicated `removeModal` with step logic.

### Keep confirmModal for Uninstall

The `confirmModal` is still used for:
- `[x]` Uninstall (simple confirm, no steps needed)
- Loadout card remove (simple confirm, no providers)
- Future simple confirmations

Only library item `[d]` Remove gets the multi-step flow.

### removeModal Methods

```go
func newRemoveModal() removeModal
func (m *removeModal) Open(item catalog.ContentItem, installedProviders []provider.Provider)
func (m *removeModal) Close()
func (m removeModal) Update(msg tea.Msg) (removeModal, tea.Cmd)
func (m removeModal) View() string

// Per-step view helpers
func (m removeModal) viewConfirm(usableW int) string   // Step 1
func (m removeModal) viewProviders(usableW int) string  // Step 2
func (m removeModal) viewReview(usableW int) string     // Step 3

// Focus helpers per step
func (m removeModal) buttonCount() int       // varies by step
func (m removeModal) firstButtonIdx() int    // len(checkboxes) for step 2, 0 for steps 1/3
func (m removeModal) isButtonFocus() bool
```

### Step 1 — Confirm

**When not installed:**
- Body: `"This will remove \"NAME\" from your library.\n\nThis action cannot be undone."` (red text for warning line)
- Buttons: Cancel (focus 0), Remove (focus 1)
- Enter on Remove → fire `removeResultMsg{confirmed: true, uninstallProviders: nil}`

**When installed:**
- Body includes: `"This content is installed in N providers.\nUninstall from them too?"`
- Buttons: Cancel (focus 0), Remove Only (focus 1), Yes (focus 2)
- Enter on Remove Only → jump to Step 3 with empty provider selections
- Enter on Yes → jump to Step 2
- Default focus: Cancel (focus 0)

### Step 2 — Provider Selection

- Checkboxes for each installed provider, **all unchecked by default**
- Focus: 0..N-1 = checkboxes, N = Back, N+1 = Done
- Space toggles checkbox
- Enter on Back → Step 1 (preserving checkbox state)
- Enter on Done → Step 3
- Default focus: first checkbox (focus 0)

### Step 3 — Review

- Shows "Will remove \"NAME\" from library."
- If providers selected: "Will uninstall from:\n  Claude Code\n  Cursor"
- If installed but not all selected: "Still installed in:\n  Windsurf"
- Buttons: Cancel (focus 0), Back (focus 1), Remove (focus 2)
- Enter on Cancel → close modal
- Enter on Back → previous step (Step 2 if providers exist, Step 1 if not)
- Enter on Remove → fire `removeResultMsg`
- Default focus: Cancel (focus 0)

### Messages

```go
type removeResultMsg struct {
    confirmed          bool
    item               catalog.ContentItem
    uninstallProviders []provider.Provider  // only the selected providers
}
```

This replaces `confirmResultMsg` for remove operations. `confirmResultMsg` is still
used by `handleUninstall` (unchanged).

### Key Handling

All steps share common keys:
- `Esc` → close modal (cancel)
- `Tab` / `Shift+Tab` → cycle focus (wrapping)
- `Up` / `Down` / `j` / `k` → move focus vertically
- `Left` / `Right` → move between buttons when on button focus (NEW — see Task 3)
- `Space` → toggle checkbox (Step 2 only)
- `Enter` → activate focused button

Step 1 also supports `y`/`n` shortcuts only when not installed (simple 2-button case).
When installed (3 buttons), `y`/`n` are disabled to avoid ambiguity.

### handleRemove Changes (actions.go)

```go
func (a App) handleRemove() (tea.Model, tea.Cmd) {
    // Gallery card remove: still uses simple confirmModal
    if a.isGalleryTab() && !a.galleryDrillIn {
        return a.handleGalleryCardRemove()
    }

    item := a.selectedItem()
    if item == nil || !item.Library {
        return a, nil
    }

    // Find installed providers
    var installed []provider.Provider
    for _, prov := range a.providers {
        if installer.CheckStatus(*item, prov, a.catalog.RepoRoot) == installer.StatusInstalled {
            installed = append(installed, prov)
        }
    }

    a.remove.Open(*item, installed)
    return a, nil
}
```

### handleConfirmResult Changes

Remove the "isRemove" detection logic from `handleConfirmResult`. Remove operations
now go through `removeResultMsg`, not `confirmResultMsg`. The `handleConfirmResult`
only handles uninstall confirmations.

Add new handler:
```go
case removeResultMsg:
    return a.handleRemoveResult(msg)
```

### App Struct Changes

```go
type App struct {
    // ...existing...
    confirm  confirmModal  // for uninstall and simple confirms
    remove   removeModal   // multi-step remove flow
    // ...
}
```

Both modals need capture blocks in Update() and overlay rendering in View().

### Success Criteria

1. Not installed: Step 1 shows Cancel/Remove, pressing Remove executes immediately
2. Installed: Step 1 shows Cancel/Remove Only/Yes
3. Remove Only → Step 3 review shows "Still installed in: [providers]"
4. Yes → Step 2 with unchecked provider checkboxes
5. Step 2 Back → Step 1 (state preserved)
6. Step 2 Done → Step 3 review with selections
7. Step 3 shows "Will uninstall from" and/or "Still installed in" correctly
8. Step 3 Cancel → close, Step 3 Back → previous step, Step 3 Remove → execute
9. Checkbox state preserved across Back navigation
10. Warning text "This action cannot be undone." rendered in red
11. All steps properly handle Esc (close modal)
12. y/n shortcuts only work in simple (not-installed) case

---

## Task 2: Action Buttons on Sub-Tab Row

**Files:** `cli/internal/tui/topbar.go` (modify), `cli/internal/tui/app.go` (modify mouse handler)

### topbar.go Changes

1. Make actions tab-sensitive (not group-level):

```go
// tabActions returns the action buttons for the current tab.
func (t topBarModel) tabActions() []tabAction {
    tab := t.ActiveTabLabel()
    switch tab {
    case "Library":
        return []tabAction{
            {label: "[a] Add", zone: "btn-add", action: "add"},
            {label: "[d] Remove", zone: "btn-remove", action: "remove"},
            {label: "[x] Uninstall", zone: "btn-uninstall", action: "uninstall"},
        }
    case "Registries":
        return []tabAction{
            {label: "[a] Add", zone: "btn-add", action: "add"},
            {label: "[d] Remove", zone: "btn-remove", action: "remove"},
        }
    case "Loadouts":
        return []tabAction{
            {label: "[d] Remove", zone: "btn-remove", action: "remove"},
        }
    case "Skills", "Agents", "MCP", "Rules", "Hooks", "Commands":
        return []tabAction{
            {label: "[a] Add", zone: "btn-add", action: "add"},
            {label: "[d] Remove", zone: "btn-remove", action: "remove"},
            {label: "[x] Uninstall", zone: "btn-uninstall", action: "uninstall"},
        }
    default: // Config tabs
        return nil
    }
}

type tabAction struct {
    label  string
    zone   string
    action string
}
```

2. Remove actions from `tabGroup` struct. The `actions []string` field on `tabGroup`
   is deleted — actions are now computed per-tab by `tabActions()`.

3. Move button rendering from `renderTabRow` to `renderBreadcrumbRow`:

```go
func (t topBarModel) renderTabRow(innerW int) string {
    // Sub-tabs only — no buttons
    // ...existing tab rendering, remove button code...
}

func (t topBarModel) renderBreadcrumbRow(innerW int) string {
    // Left: breadcrumbs (if any)
    // Right: action buttons (context-sensitive)
    left := t.renderBreadcrumbs()
    right := t.renderActionButtons()
    // Layout: left-aligned crumbs, right-aligned buttons
}
```

4. Graceful degradation: if buttons don't fit, drop rightmost first.

### Mouse Handling Changes

Update topbar's `Update()` mouse handling to check new zone IDs:

```go
if zone.Get("btn-remove").InBounds(mouseMsg) {
    return t, t.actionCmd("remove")
}
if zone.Get("btn-uninstall").InBounds(mouseMsg) {
    return t, t.actionCmd("uninstall")
}
```

Update `app.go` to handle the new `actionPressedMsg` actions:

```go
case actionPressedMsg:
    switch msg.action {
    case "add":
        // existing
    case "remove":
        return a.handleRemove()
    case "uninstall":
        return a.handleUninstall()
    }
```

### Success Criteria

1. Buttons render on Row 3 (breadcrumb row), not Row 2
2. Library tab shows [a] Add, [d] Remove, [x] Uninstall
3. Registries shows [a] Add, [d] Remove
4. Loadouts shows [d] Remove
5. Content tabs show [a] Add, [d] Remove, [x] Uninstall
6. Config tabs show no buttons
7. Mouse clicks on buttons fire correct actions
8. Buttons and breadcrumbs coexist on Row 3
9. Narrow terminal: rightmost buttons dropped first
10. Golden files updated

---

## Task 3: Left/Right Button Navigation in Modals

**Files:** `cli/internal/tui/confirm.go` (modify updateKey), `cli/internal/tui/confirm.go` → new `removeModal` (built in Task 1)

### Changes to confirmModal.updateKey()

Add left/right handling when focus is on a button:

```go
case msg.Type == tea.KeyLeft:
    if m.isButtonFocus() {
        // Move to previous button (wrap)
        if m.focusIdx == m.firstButtonIdx() {
            m.focusIdx = m.confirmIdx() // wrap to last button
        } else {
            m.focusIdx--
        }
    }
case msg.Type == tea.KeyRight:
    if m.isButtonFocus() {
        // Move to next button (wrap)
        if m.focusIdx == m.confirmIdx() {
            m.focusIdx = m.firstButtonIdx() // wrap to first button
        } else {
            m.focusIdx++
        }
    }
```

Add helper:
```go
func (m confirmModal) isButtonFocus() bool {
    return m.focusIdx >= m.cancelIdx()
}
func (m confirmModal) firstButtonIdx() int {
    return m.cancelIdx()
}
```

The `removeModal` gets the same left/right logic built in from the start (Task 1).

### Success Criteria

1. Left/Right moves between buttons when focused on a button
2. Left/Right wraps (first ← last, last → first)
3. Left/Right is no-op when focused on checkboxes
4. Works in both confirmModal and removeModal

---

## Task 4: Title Wrapping Fix

**Files:** `cli/internal/tui/confirm.go` (modify View)

### Change

Replace:
```go
titleText := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(m.title)
```

With:
```go
titleText := lipgloss.NewStyle().Bold(true).Foreground(primaryText).MaxWidth(usableW).Render(m.title)
```

Also increase default modal width from 50 to 54:
```go
modalW := min(54, m.width-10)
if modalW < 34 {
    modalW = 34
}
```

Apply the same fix to the `removeModal.View()` (built in Task 1).

### Success Criteria

1. Long item names truncated, not wrapped past border
2. Modal is slightly wider (54 vs 50) to accommodate typical names
3. Golden file with long name verifies no wrapping

---

## Implementation Order

Strict sequential with validation gates:

```
Impl Task 3 (left/right buttons in confirmModal)
  └── Validate Task 3
      └── Impl Task 4 (title wrapping fix)
          └── Validate Task 4
              └── Impl Task 1 (multi-step Remove modal)
                  └── Validate Task 1
                      └── Impl Task 2 (action buttons on sub-tab row)
                          └── Validate Task 2
                              └── Phase A.1 Validation
```

**Rationale for this order:**
- Tasks 3 and 4 are small fixes to existing confirm.go — quick wins
- Task 1 is the biggest change (new removeModal) — depends on Tasks 3/4 being done first
  since it builds on the same file
- Task 2 (topbar buttons) is independent of the modal work but goes last because
  golden files change — easier to review after modal changes are stable

---

## Test Plan

### confirm_test.go Updates (Tasks 3, 4)

| Test | Description | Criteria |
|------|-------------|----------|
| `TestConfirmModal_LeftRightBetweenButtons` | Left/Right moves between Cancel and Confirm | Focus cycles correctly |
| `TestConfirmModal_LeftRightWraps` | Right from last button → first, Left from first → last | Wrap verified |
| `TestConfirmModal_LeftRightNoOpOnCheckbox` | Left/Right when focused on checkbox → no change | focusIdx unchanged |
| `TestConfirmModal_TitleTruncated` | Long title doesn't wrap past modal border | All lines same width |

### remove_test.go (Task 1 — new file)

| Test | Description | Criteria |
|------|-------------|----------|
| `TestRemoveModal_OpenClose` | Open sets fields, Close clears | active, step, providers correct |
| `TestRemoveModal_NotInstalled_Step1` | No providers → 2 buttons (Cancel, Remove) | No provider section in view |
| `TestRemoveModal_NotInstalled_Remove` | Enter on Remove → removeResultMsg confirmed | No uninstall providers |
| `TestRemoveModal_Installed_Step1` | Has providers → 3 buttons (Cancel, Remove Only, Yes) | Provider count in body |
| `TestRemoveModal_Installed_RemoveOnly` | Enter on Remove Only → Step 3 review | "Still installed in" shown |
| `TestRemoveModal_Installed_Yes` | Enter on Yes → Step 2 | Provider checkboxes shown |
| `TestRemoveModal_Step2_DefaultUnchecked` | All checkboxes unchecked on entry | All false |
| `TestRemoveModal_Step2_SpaceToggles` | Space toggles checkbox | checked flipped |
| `TestRemoveModal_Step2_Back` | Enter on Back → Step 1 | step == removeStepConfirm |
| `TestRemoveModal_Step2_Done` | Enter on Done → Step 3 | Selections carried to review |
| `TestRemoveModal_Step3_ShowsUninstall` | Selected providers shown | "Will uninstall from" in view |
| `TestRemoveModal_Step3_ShowsStillInstalled` | Unselected providers shown | "Still installed in" in view |
| `TestRemoveModal_Step3_Cancel` | Enter on Cancel → close | active=false, confirmed=false |
| `TestRemoveModal_Step3_Back` | Enter on Back → Step 2 (or Step 1) | Correct step |
| `TestRemoveModal_Step3_Remove` | Enter on Remove → removeResultMsg | confirmed=true, providers correct |
| `TestRemoveModal_BackPreservesState` | Back/forward preserves checkbox state | Checkboxes unchanged |
| `TestRemoveModal_EscFromAnyStep` | Esc closes from any step | active=false |
| `TestRemoveModal_YN_NotInstalled` | y/n work when not installed | Shortcuts fire |
| `TestRemoveModal_YN_Disabled_Installed` | y/n ignored when installed (3 buttons) | No result msg |
| `TestRemoveModal_LeftRight_Buttons` | Left/Right cycles buttons per step | Focus correct |
| `TestRemoveModal_WarningRedText` | "This action cannot be undone." in red | ANSI red in view |

### topbar_test.go Updates (Task 2)

| Test | Description | Criteria |
|------|-------------|----------|
| `TestTopBar_ButtonsOnRow3` | Buttons render on breadcrumb row | Row 3 contains button text |
| `TestTopBar_Row2NoButtons` | Tab row has no buttons | Row 2 has only tab names |
| `TestTopBar_LibraryButtons` | Library shows [a][d][x] | All three in view |
| `TestTopBar_RegistriesButtons` | Registries shows [a][d] | Two buttons, no [x] |
| `TestTopBar_LoadoutsButtons` | Loadouts shows [d] | Only Remove |
| `TestTopBar_ContentButtons` | Content tabs show [a][d][x] | All three |
| `TestTopBar_ConfigNoButtons` | Config shows no buttons | No button text |
| `TestTopBar_NarrowDegradation` | 50-char width: some buttons dropped | Fewer buttons, no crash |

### app_test.go Updates (Task 2)

| Test | Description | Criteria |
|------|-------------|----------|
| `TestApp_RemoveButtonClickFiresRemove` | Click btn-remove zone | handleRemove called |
| `TestApp_UninstallButtonClickFiresUninstall` | Click btn-uninstall zone | handleUninstall called |

### Golden Files

| Golden | Description |
|--------|-------------|
| `remove-step1-notinstalled-80x30` | Step 1, no providers |
| `remove-step1-installed-80x30` | Step 1 with provider count |
| `remove-step2-80x30` | Provider selection |
| `remove-step3-80x30` | Review with uninstall + still-installed |
| `remove-step1-60x20` | Minimum size |
| `confirm-simple-80x30` | Updated: wider modal (54), left/right works |
| All existing topbar goldens | Updated: buttons on Row 3 |
