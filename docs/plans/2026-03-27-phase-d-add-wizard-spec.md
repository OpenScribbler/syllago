# Phase D Spec: Add Wizard

**Date:** 2026-03-27
**Phase:** D (fourth phase of TUI Wizards)
**Parent design:** `docs/plans/2026-03-26-tui-wizards-design.md` (W1: Add Wizard)
**Status:** Spec v2 — post-expert-panel review (7+8+7+7 MUST-FIX addressed)

---

## Purpose

Import content from a provider, registry, local path, or git URL into the
syllago library (`~/.syllago/content/`). Full-screen wizard with 5 steps:
Source → Type → Discovery → Review → Execute.

Reuses wizard shell, risk banner, and code highlighting from Phase B.
New component: checkbox list for multi-select item selection.

**Entry points:**
- `[a]` from Library tab — all source types available
- `[a]` from Content > [Type] tab — pre-filtered to that content type (skips Step 2)
- _Future: "Add to Library" from registry item drill-in (deferred)_

---

## Architecture

### Wizard Model on App

Follows the install wizard pointer field pattern:

```go
// In app.go, add to wizardKind enum:
const (
    wizardNone    wizardKind = iota
    wizardInstall
    wizardAdd  // new
)

// In App struct, add:
addWizard *addWizardModel // nil when not active
```

### Routing

`App.routeToWizard()` gains a `wizardAdd` case that delegates to `addWizard.Update()`.
`App.renderContent()` gains a `wizardAdd` case that calls `addWizard.View()`.

**Critical:** `addWizardModel.Update()` MUST use a pointer receiver `(m *addWizardModel)`
and return `(*addWizardModel, tea.Cmd)`, following the install wizard pattern. The
`routeToWizard` call discards the return model (`_, cmd := a.addWizard.Update(msg)`)
because mutations happen in-place via the pointer. This is correct and matches install.

**Key routing note:** The `keyAdd` (`[a]`) handler in `app_update.go` is only reached
when `wizardMode == wizardNone`, because the wizard mode guard short-circuits to
`routeToWizard` before global key handling. No extra suppression of `[a]` during
wizard mode is needed — the checkbox list's `'a'` (select all) is consumed by the
wizard's sub-model Update, not the app's global handler.

### File Organization

| File | Contents |
|------|----------|
| `add_wizard.go` | addWizardModel struct, step enum, constructor, validateStep, helpers |
| `add_wizard_update.go` | Update(), per-step key/mouse handlers |
| `add_wizard_view.go` | View(), per-step renderers |
| `add_wizard_test.go` | Wizard flow tests |
| `checkbox_list.go` | Reusable checkbox list component |
| `checkbox_list_test.go` | Component tests |

---

## Component: Checkbox List (checkbox_list.go)

A reusable multi-select list component. Used by the Add Wizard's Type and
Discovery steps, and potentially by future wizards.

### Struct

```go
type checkboxBadgeStyle int

const (
    badgeStyleNone    checkboxBadgeStyle = iota
    badgeStyleDanger  // dangerColor — !! HIGH, executable code
    badgeStyleWarning // warningColor — ! MED
    badgeStyleSuccess // successColor — New
    badgeStyleMuted   // mutedColor — In library
)

type checkboxItem struct {
    label       string
    description string           // second line, muted
    disabled    bool             // cannot be selected (shown muted)
    badge       string           // optional right-aligned badge (e.g., "!! HIGH", "New")
    badgeStyle  checkboxBadgeStyle // maps to Flexoki palette in render function
}

type checkboxList struct {
    items    []checkboxItem
    selected []bool   // parallel array, same length as items
    cursor   int      // currently highlighted row
    offset   int      // scroll offset for tall lists
    width    int
    height   int      // visible rows (determines scrolling)
    focused  bool
}
```

**All methods use value receivers** (matching `riskBanner` pattern). Callers must
reassign: `m.discoveryList, cmd = m.discoveryList.Update(msg)`.

All `label`, `description`, and `badge` strings MUST be sanitized before display.
Use `sanitizeLine()` (from `table.go`) plus ANSI escape code stripping for
untrusted content (item names/descriptions from discovery).

### Behavior

| Key | Action |
|-----|--------|
| `Up`/`k` | Move cursor up |
| `Down`/`j` | Move cursor down |
| `Space` | Toggle selection on cursor item (skip if disabled) |
| `a` | Select all (non-disabled) |
| `n` | Deselect all |
| `Enter` | Emit `checkboxDrillInMsg{index}` for focused item |
| `PgUp`/`PgDn` | Page navigation (by `m.height` rows) |
| `Home`/`End` | Jump to first/last |

### Messages

```go
type checkboxDrillInMsg struct {
    index int
}
```

### Rendering

```
  [x] my-rule              rules    New
  [ ] pre-tool-validate    hooks    !! Runs: bash  New
  [ ] old-hook             hooks    Already in library
> [x] mcp-server           mcp      New
```

- `>` cursor indicator on focused row
- `[x]` / `[ ]` checkbox
- Disabled items shown in muted style, checkbox replaced with `[-]`
- Badge right-aligned within available width, colored by `badgeStyle`
- Scrollbar indicator when list exceeds visible height
- Scroll offset preserved across back-navigation (same as selections)

### Constructor

All value receivers — callers must reassign on mutation:

```go
func newCheckboxList(items []checkboxItem) checkboxList
func (c checkboxList) SetSize(w, h int) checkboxList
func (c checkboxList) SelectedIndices() []int  // read-only, no mutation
func (c checkboxList) Update(msg tea.KeyMsg) (checkboxList, tea.Cmd)
func (c checkboxList) View() string
```

---

## Step Enum

```go
type addStep int

const (
    addStepSource    addStep = iota // Where is the content?
    addStepType                     // What type? (skipped if pre-filtered)
    addStepDiscovery                // Scan + select items
    addStepReview                   // Risk review + confirmation
    addStepExecute                  // Add items with progress
)
```

---

## Source Types

```go
type addSource int

const (
    addSourceNone     addSource = iota
    addSourceProvider           // detected provider on disk
    addSourceRegistry           // configured registry
    addSourceLocal              // local directory path
    addSourceGit                // git URL (clone first)
)
```

---

## Model Struct

```go
type addWizardModel struct {
    shell  wizardShell
    step   addStep
    width  int
    height int
    seq    int // async sequence number (incremented on cancel/back)

    // Step 1: Source
    source        addSource
    sourceCursor  int
    sourceExpanded bool   // true when Provider or Registry sub-list is shown
    inputActive    bool   // true when Local path or Git URL text input has focus
    // Provider sub-selection
    providers       []provider.Provider
    providerCursor  int
    // Registry sub-selection
    registries      []catalog.RegistrySource
    registryCursor  int
    // Local/Git text input
    pathInput    string
    pathCursor   int

    // Step 2: Type
    preFilterType catalog.ContentType // non-empty when entered from Content > [Type] tab
    typeChecks    checkboxList        // multi-select for content types

    // Step 3: Discovery
    discovering     bool               // true while async scan in progress
    discoveryErr    string             // non-empty if scan failed
    discoveredItems []addDiscoveryItem // results from scan
    discoveryList   checkboxList       // item selection list

    // Step 4: Review
    reviewAcknowledged bool // set to true when user presses [Add N items]
    risks              []catalog.RiskIndicator
    riskBanner         riskBanner
    reviewZone         addReviewZone   // which zone is focused (risks/items/buttons)
    buttonCursor       int             // -1=none, 0=Cancel, 1=Back, 2=Add
    conflicts          []int           // indices into discoveredItems that conflict

    // Step 5: Execute
    executing        bool
    executeResults   []addExecResult
    executeCurrent   int  // index currently being added
    executeDone      bool
    executeCancelled bool

    // Git URL source — temp dir cleanup
    gitTempDir string // non-empty when source=Git and clone succeeded

    // Taint tracking (set during discovery, used during execute)
    sourceRegistry   string // registry name for taint propagation
    sourceVisibility string // "public", "private", "unknown"

    // Context
    projectRoot string
    contentRoot string // ~/.syllago/content/
    cfg         *config.Config
}

// addReviewZone tracks which zone is focused on the review step.
type addReviewZone int

const (
    addReviewZoneRisks   addReviewZone = iota
    addReviewZoneItems
    addReviewZoneButtons
)

// selectedTypes returns the content types selected in the typeChecks list.
// Used by validateStep and discovery backend.
func (m *addWizardModel) selectedTypes() []catalog.ContentType {
    if m.preFilterType != "" {
        return []catalog.ContentType{m.preFilterType}
    }
    allTypes := []catalog.ContentType{
        catalog.Rules, catalog.Skills, catalog.Agents,
        catalog.Hooks, catalog.MCP, catalog.Commands,
    }
    var result []catalog.ContentType
    for _, idx := range m.typeChecks.SelectedIndices() {
        if idx < len(allTypes) {
            result = append(result, allTypes[idx])
        }
    }
    return result
}

// selectedItems returns the discoveredItems that the user has selected.
func (m *addWizardModel) selectedItems() []addDiscoveryItem {
    var result []addDiscoveryItem
    for _, idx := range m.discoveryList.SelectedIndices() {
        if idx < len(m.discoveredItems) {
            result = append(result, m.discoveredItems[idx])
        }
    }
    return result
}
```

### Discovery Item (TUI-specific wrapper)

```go
type addDiscoveryItem struct {
    name        string
    displayName string
    itemType    catalog.ContentType
    path        string       // absolute path to content
    sourceDir   string       // for directory-based items
    status      add.ItemStatus // New, InLibrary, Outdated
    scope       string       // "global" or "project"
    risks       []catalog.RiskIndicator
    overwrite   bool         // user toggled overwrite for conflicts
}
```

### Execute Result

```go
type addExecResult struct {
    name   string
    status string // "added", "updated", "skipped", "error"
    err    error
}
```

---

## Messages

```go
// addCloseMsg signals the wizard should close.
type addCloseMsg struct{}

// addDiscoveryStartMsg triggers the async discovery scan.
type addDiscoveryStartMsg struct {
    seq int
}

// addDiscoveryDoneMsg returns scan results.
type addDiscoveryDoneMsg struct {
    seq    int
    items  []addDiscoveryItem
    err    error
    tmpDir string // non-empty for git URL source — stored on model for cleanup
}

// addExecItemMsg triggers adding one item.
type addExecItemMsg struct {
    seq   int
    index int
}

// addExecItemDoneMsg reports one item's result.
type addExecItemDoneMsg struct {
    seq    int
    index  int
    result addExecResult
}

// addExecAllDoneMsg signals all items are done.
type addExecAllDoneMsg struct {
    seq int
}
```

---

## Step Behavior

### Step 1: Source

**Rendering:**
```
╭───Add ────────────────────────────────────────╮
│  [1 Source]  [2 Type]  [3 Discovery]  ...     │
╰───────────────────────────────────────────────╯

  Where is the content?

  > [ ] Provider      Claude Code, Cursor, ...
    [ ] Registry      team-rules, community, ...
    [ ] Local path    /path/to/content
    [ ] Git URL       https://github.com/...
```

- Radio-button style selection (single choice), using `[ ]` / `[*]` glyphs (consistent with checkbox)
- Up/Down navigates source options (when no sub-list is expanded and no text input is active)
- Enter on Provider: sets `sourceExpanded = true`, expands sub-list inline
- Enter on Registry: sets `sourceExpanded = true`, expands sub-list inline
- Enter on Local path/Git URL: sets `inputActive = true`, activates text input field
- Enter on a sub-selection (specific provider or registry) advances to Step 2
- Enter on Local/Git with non-empty, validated text advances to Step 2

**Two-level Esc behavior:**
- If sub-list is expanded (`sourceExpanded`): Esc collapses sub-list, returns to top-level radio. Wizard stays open.
- If text input is active (`inputActive`) and non-empty: Esc clears input, deactivates it.
- If text input is active and empty: Esc deactivates text input, returns to top-level radio.
- If neither sub-list nor text input is active: Esc closes wizard.

**Text input keyboard model** (mirrors install wizard custom path pattern):
- When `inputActive = true`, all standard text editing keys work (typing, backspace, Ctrl+A/E for home/end)
- Up/k: deactivates input, moves cursor to previous radio option
- Down/j: deactivates input, moves cursor to next radio option
- `j/k/a/n` are consumed by text input when `inputActive = true` (typed as literal characters)
- Previously typed text is preserved when navigating away and returning

**Sub-selections:**

When "Provider" is focused and Enter is pressed, expand inline:
```
  > [*] Provider
        > Claude Code    (detected)
          Cursor         (detected)
          Windsurf       (not detected)
```
Only detected providers are selectable. Non-detected are shown muted.

When "Registry" is focused and Enter is pressed:
```
  > [*] Registry
        > team-rules
          community-hooks
```
Shows configured registries from `cfg.Registries`. If none configured, show
"No registries configured. Close this wizard and add one from the Registries
tab ([3] → Registries → [a])." and disable the option.

**Local path validation** before advancing to Step 2:
- Path must be absolute or start with `~` (expand via `os.UserHomeDir()`)
- Reject empty path (Enter is no-op)
- `os.Stat()` the resolved path: show inline error if it doesn't exist or isn't a directory
- Relative paths and `..` components rejected with inline message

**Git URL validation** before cloning:
- URL must match `https://`, `git@`, or `ssh://` protocol
- Reject `ext::`, `git-remote-ext`, and `file://` protocols explicitly (security: `ext::` executes arbitrary binaries)
- Reject local paths passed as URLs (`/path/to/repo`)
- Show inline error for invalid URL form

**Edge cases:**
- No detected providers: Provider option shown but disabled with "(no providers detected)"
- No configured registries: Registry option shown but disabled with "(no registries)"
- Both empty: Local path and Git URL are always available

### Step 2: Type

**Rendering** (single-column checkboxList, NOT a grid):
```
  What type of content?

> [x] Rules
  [x] Skills
  [ ] Agents
  [x] Hooks
  [x] MCP
  [ ] Commands

  [space] toggle  [a] all  [n] none  [enter] next
```

- Uses checkboxList component with 6 items (one per content type, excluding Loadouts)
- Single-column vertical list (standard checkboxList layout, not a grid)
- Multi-select allowed (Space toggles, `a` selects all, `n` deselects all)
- For provider sources: pre-check types the provider supports (via `prov.SupportsType`)
- For registry/local/git: all types checked by default
- Enter advances to Step 3 (at least one type must be selected)
- Esc goes back to Step 1

**Skip condition:** When entering from Content > [Type] tab, `preFilterType` is set.
Step 2 is skipped entirely — the shell shows 4 steps instead of 5, and the wizard
jumps directly from Source to Discovery.

**Step labels for both paths:**
- 5-step: `["Source", "Type", "Discovery", "Review", "Execute"]`
- 4-step (pre-filtered): `["Source", "Discovery", "Review", "Execute"]`
- Numbers restart from 1 in both paths (matching install wizard convention)

### Step 3: Discovery

**Rendering during scan:**
```
  Scanning Claude Code for rules, skills, hooks...

  ◐  Discovering content...
```

Uses a spinner (tick-based animation, rune sequence `◐◓◑◒`). The scan runs as a `tea.Cmd`.

**During scan:** All keys except Esc are ignored. The checkbox list's `Update()` MUST
NOT be called while `discovering == true` (list is unpopulated, could panic on empty slice).

**Rendering after scan:**
```
  Found 8 items from Claude Code:

  [x] my-rule              rules    New
  [x] security-check       rules    New          ! Code detected
  [ ] old-config           rules    In library
  [x] pre-tool-validate    hooks    New          !! Runs: bash
  [x] mcp-readability      mcp      New          !! Network
  [ ] readme-gen           skills   In library
  [x] helper-skill         skills   Outdated
  [x] team-agent           agents   New

  [space] toggle  [a] all  [n] none  [enter] inspect  [→] continue
```

- Uses checkboxList component
- Pre-selected: items with `StatusNew` and `StatusOutdated`
- Not selected: items with `StatusInLibrary` (already in library)
- Risk badges shown inline from `catalog.RiskIndicators()`, colored by `badgeStyle`
- `Enter` exclusively drills into the focused item (file preview, reuses library detail pattern)
- `Right arrow` (→) advances to Step 4 (at least one item must be selected)
- `Enter` NEVER advances to Step 4 — only `→` does. This avoids ambiguity between drill-in and advance.
- Esc goes back to Step 2 (or Step 1 if type was pre-filtered)

**Discovery backends by source:**

| Source | Discovery Function | Notes |
|--------|-------------------|-------|
| Provider | `add.DiscoverFromProvider()` + hook/MCP discovery | Uses existing CLI add backend |
| Registry | `catalog.ScanRegistriesOnly()` then filter | Scans registry path for all types |
| Local path | `catalog.ScanNativeContent()` or `catalog.Scan()` | Native scan if not syllago structure, regular scan if syllago |
| Git URL | Hardened clone to temp dir, then scan | See Git URL Discovery section below |

**Discovery tea.Cmd pattern:**
```go
func (m *addWizardModel) startDiscoveryCmd() tea.Cmd {
    seq := m.seq
    source := m.source
    // capture all needed values
    return func() tea.Msg {
        items, err := discoverItems(...)
        return addDiscoveryDoneMsg{seq: seq, items: items, err: err}
    }
}
```

Stale results (where `msg.seq != m.seq`) are silently dropped in Update().

**Error handling:**
If discovery fails, show error inline (rounded corners, `dangerColor` border):
```
  Scanning https://github.com/team/rules...

  ╭── Error ──────────────────────────────────╮
  │ Git clone failed: connection timed out     │
  │                                            │
  │ [r] Retry   [Esc] Back                     │
  ╰────────────────────────────────────────────╯
```
Toast also fires for visibility. `r` retries (increments seq, restarts cmd).
Esc goes back to Step 2 (or Step 1 if pre-filtered).

**Empty discovery:**
```
  No content found matching your selection.

  [Esc] Back to change source or types
```

### Step 4: Review

**Rendering** (uses `riskBanner.ViewInline()` for unified frame, rounded corners):
```
  Adding 5 items to library:

  ╭── Risk Indicators ───────────────────────────╮
  │> !! pre-tool-validate  HIGH  bash -c "esl..." │
  │  !! mcp-readability    HIGH  npx @anthrop...   │
  │  !  security-check     MED   References Bash   │
  ╰────────────────────────────────────────────────╯

  Items:
    my-rule              rules     (new)
    security-check       rules     (new) ! MED
    pre-tool-validate    hooks     (new) !! HIGH
    mcp-readability      mcp       (new) !! HIGH
    helper-skill         skills    (update — content differs)

  [Cancel]  [Back]  [Add 5 items]
```

**Item list styles:**
- Item name: `primaryText` bold
- Type: `mutedStyle`
- `(new)`: `successColor`
- `(update — content differs)`: `warningColor`
- `!! HIGH` badge: `dangerColor`
- `! MED` badge: `warningColor`

- Risk banner uses `riskBanner.ViewInline()` (matching install wizard — single border, no double-nesting)
- Risk banner is navigable: Up/Down moves between risks, Enter drills in
- Tab cycles: risks → item list → buttons (when no risks: items → buttons)

**Initial focus on entry:** `addReviewZoneButtons` with `buttonCursor = 1` ([Back]).
This is the safe default — Enter on [Back] is non-destructive. User must Tab to risks
or items first if they want to review details. This matches the install wizard pattern.

**Button layout:**
- `[Cancel]` at index 0, `[Back]` at index 1, `[Add N items]` at index 2
- `dangerLabels: nil` (adding is not destructive — no red button styling)
- Default `buttonCursor = 1` ([Back]) on entry via Tab

- Items with `StatusOutdated` shown as "(update — content differs)" in `warningColor`
- Conflict items (same name, different content) show overwrite toggle
- [Add N items] button: primary action, advances to Step 5
- [Back]: returns to Step 3 with selections preserved
- [Cancel]: closes wizard
- Esc from review = [Back] (not close)

**reviewAcknowledged transition:** Set to `true` when the user presses Enter on
[Add N items] button. This gates `validateStep()` for Execute. It is set before
transitioning to `addStepExecute`, in the same Update() call.

**Double-confirm prevention:** After pressing Enter on [Add N items], the button
is disabled (set `reviewAcknowledged = true`) and subsequent Enter presses are ignored.

### Step 5: Execute

**Rendering during execution:**
```
  Adding items...

  ✓ my-rule              Added
  ✓ security-check       Added
  ◐ pre-tool-validate    Adding...
    mcp-readability      Pending
    helper-skill         Pending
```

**Rendering after completion:**
```
  Done! 5 items added to library.

  ✓ my-rule              Added
  ✓ security-check       Added
  ✓ pre-tool-validate    Added
  ✓ mcp-readability      Added
  ✓ helper-skill         Updated

  [Enter] Go to Library
```

- Items are added sequentially via chained `tea.Cmd`s (not `tea.Batch`)
- Each item completion triggers the next item's add command
- Progress indicators: `✓` in `successColor` for done, `◐` in `primaryColor` for in-progress (spinner rune), blank + `mutedStyle` for pending, `✗` in `dangerColor` for error
- Esc during execution: abandons remaining items (current in-flight tea.Cmd completes, result is recorded)
- Partial completion format: "X of Y added, Z errors, W cancelled" (distinguishes errors from cancellation)
- Already-added items are NOT rolled back
- After completion: Enter returns to Library tab, Esc also works
- Toast fires on completion: "Added N items to library"

**Execute tea.Cmd pattern:**
```go
// Sequential: each addExecItemDoneMsg triggers the next addExecItemMsg
case addExecItemDoneMsg:
    // Seq guard: drop stale messages from cancelled runs
    if msg.seq != m.seq {
        return m, nil
    }
    m.executeResults[msg.index] = msg.result
    if next := m.nextPending(); next >= 0 && !m.executeCancelled {
        return m, m.addItemCmd(next)
    }
    m.executeDone = true
    return m, func() tea.Msg { return addExecAllDoneMsg{seq: m.seq} }
```

**Backend call per item type:**

| Type | How to Add |
|------|-----------|
| Rules, Skills, Agents, Commands | `add.AddItems()` with single-item slice |
| Hooks | `addHooksFromDiscovery()` — reads source, writes hook.json + metadata |
| MCP | `addMcpFromDiscovery()` — reads source, writes config.json + metadata |

For hooks and MCP, the wizard calls dedicated helper functions that replicate
the CLI's `addHooksFromLocation` / `addMcpFromLocation` logic but work from
discovery items rather than settings file locations.

**Taint propagation** (CRITICAL for privacy gate):
When calling `add.AddItems()` or the hook/MCP helpers, populate taint fields:
- `addSourceRegistry`: `SourceRegistry = reg.Name`, `SourceVisibility` from
  `registry.ProbeVisibility(reg.URL)` (probed once during discovery, stored on model as
  `sourceRegistry`/`sourceVisibility`)
- `addSourceGit`: `SourceRegistry = registry.NameFromURL(url)`, `SourceVisibility`
  from `registry.ProbeVisibility(url)` (probed once after successful clone)
- `addSourceProvider` / `addSourceLocal`: leave both empty (auto-detection in
  `add.writeItem()` handles symlink and hash-match taint cases)

Hook/MCP helpers MUST also write `.syllago.yaml` metadata with `SourceRegistry`
and `SourceVisibility` populated identically to the rules/skills path.

---

## validateStep() Prerequisites

| Step | Prerequisites (panic if violated) |
|------|-----------------------------------|
| addStepSource | _(none — entry step)_ |
| addStepType | `m.source != addSourceNone` |
| addStepDiscovery | `m.source != addSourceNone` AND (`len(m.selectedTypes()) > 0`) |
| addStepReview | `len(m.discoveredItems) > 0` AND `len(m.selectedItems()) > 0` |
| addStepExecute | `len(m.selectedItems()) > 0` AND `m.reviewAcknowledged` |

Both `selectedTypes()` and `selectedItems()` are defined as methods on the model (see struct section).

---

## Constructor

```go
func openAddWizard(
    providers []provider.Provider,
    registries []catalog.RegistrySource,
    cfg *config.Config,
    projectRoot string,
    contentRoot string,
    preFilterType catalog.ContentType, // empty string = no filter
) *addWizardModel
```

- If `preFilterType` is set, shell shows 4 steps: Source, Discovery, Review, Execute
- Otherwise shell shows 5 steps: Source, Type, Discovery, Review, Execute
- Filters providers to detected-only
- Initial source cursor defaults to Provider if any detected, else Local

---

## Back-Navigation and State Invalidation

State invalidation uses **explicit field resets** (matching install wizard pattern),
not version counters. The `seq` field handles async staleness.

**Back-navigation resets:**

| Navigation | State Cleared |
|-----------|--------------|
| Review → Discovery | `reviewAcknowledged = false`, `risks = nil`, `riskBanner = riskBanner{}`. Selections and scroll offset preserved, no re-scan. |
| Discovery → Type | `discoveredItems = nil`, `discoveryList = checkboxList{}`, `discoveryErr = ""`. Will re-scan on next advance. |
| Discovery → Source (pre-filtered) | Same as above, plus clear source state if needed. |
| Type → Source | `typeChecks` selections preserved (user may want to refine source). |
| Source change after going back | Clear Type selections, Discovery results, Review state. `seq++` to invalidate in-flight. |
| Type change after going back | Clear Discovery results, Review state. `seq++` to invalidate in-flight. |

**Git temp dir cleanup on back-navigation:** When navigating back from Discovery
to Source/Type with `source == addSourceGit` and `gitTempDir != ""`, call
`os.RemoveAll(gitTempDir)` and set `gitTempDir = ""`. New discovery will re-clone.

---

## App Integration (actions.go + app_update.go + app_view.go)

### Launching the Wizard

```go
// In actions.go:
func (a App) handleAdd() (tea.Model, tea.Cmd) {
    // Determine pre-filter from current tab
    var preFilter catalog.ContentType
    if a.isContentTab() {
        preFilter = a.topBar.ActiveContentType()
    }

    detected := filterDetectedProviders(a.providers)
    a.addWizard = openAddWizard(
        detected,
        a.registrySources,
        a.cfg,
        a.projectRoot,
        a.contentRoot,
        preFilter,
    )
    a.addWizard.width = a.width
    a.addWizard.height = a.contentHeight()
    a.addWizard.shell.SetWidth(a.width)
    a.wizardMode = wizardAdd
    return a, nil
}
```

### Key Binding Changes

In `app_update.go`, the existing `[a]` handler needs updating:

```go
case msg.String() == keyAdd:
    if a.isRegistriesTab() && !a.galleryDrillIn {
        // Existing registry add behavior
        return a.handleRegistryAdd()
    }
    // NEW: Launch Add Wizard for Library and Content tabs
    if a.isLibraryTab() || a.isContentTab() {
        return a.handleAdd()
    }
    return a, nil
```

### Message Handling

```go
// In app_update.go:
case addCloseMsg:
    // Clean up git temp dir if present
    if a.addWizard != nil && a.addWizard.gitTempDir != "" {
        os.RemoveAll(a.addWizard.gitTempDir)
    }
    a.addWizard = nil
    a.wizardMode = wizardNone
    // Rescan catalog AFTER wizard closes (items may have been added)
    cmd := a.rescanCatalog()
    return a, cmd

// addExecAllDoneMsg is handled entirely inside the wizard via routeToWizard.
// The wizard sets executeDone=true and renders the completion screen.
// No App-level handler needed — the wizard sends addCloseMsg when the user
// presses Enter on "Go to Library", which triggers the rescan above.
```

### Routing

```go
// In routeToWizard:
case wizardAdd:
    if a.addWizard != nil {
        _, cmd := a.addWizard.Update(msg)
        return a, cmd
    }

// In renderContent:
if a.wizardMode == wizardAdd && a.addWizard != nil {
    return a.addWizard.View()
}
```

### WindowSizeMsg Propagation

```go
// In the tea.WindowSizeMsg handler:
if a.addWizard != nil {
    a.addWizard.width = msg.Width
    a.addWizard.height = a.contentHeight()
    a.addWizard.shell.SetWidth(msg.Width)
}
```

---

## Discovery Backend Functions

These are the backend functions called from `tea.Cmd` goroutines. They MUST NOT
access TUI state — they receive all needed data as parameters.

### Provider Discovery

```go
func discoverFromProvider(
    prov provider.Provider,
    projectRoot string,
    cfg *config.Config,
    contentRoot string,
    selectedTypes []catalog.ContentType,
) ([]addDiscoveryItem, error)
```

Internally calls `add.DiscoverFromProvider()` for file-based types, plus
hook/MCP discovery for those types. Filters by `selectedTypes`. Annotates
each item with risk indicators via `catalog.RiskIndicators()`.

### Registry Discovery

```go
func discoverFromRegistry(
    reg catalog.RegistrySource,
    selectedTypes []catalog.ContentType,
    contentRoot string,
) ([]addDiscoveryItem, error)
```

Calls `catalog.ScanRegistriesOnly()` with the single registry, filters by type,
computes library status by comparing against existing library items.

### Local Path Discovery

```go
func discoverFromLocalPath(
    dir string,
    selectedTypes []catalog.ContentType,
    contentRoot string,
) ([]addDiscoveryItem, error)
```

First runs `catalog.ScanNativeContent(dir)`. If `HasSyllagoStructure` is true,
uses `catalog.Scan(dir, dir)` instead (it's a syllago repo, not native content).
Filters by type, annotates with risk and library status.

### Git URL Discovery

```go
func discoverFromGitURL(
    url string,
    selectedTypes []catalog.ContentType,
    contentRoot string,
) ([]addDiscoveryItem, string, error) // returns tmpDir for cleanup
```

Clones to a temp directory using a **hardened git invocation** equivalent to
`registry.cloneArgs()`:
- Set `GIT_CONFIG_NOSYSTEM=1` in the environment
- Pass `-c core.hooksPath=/dev/null` (prevents repo hooks from executing on clone)
- Pass `--no-recurse-submodules` (prevents submodule-based attacks)
- Use `--depth 1` (shallow clone — faster and limits attack surface)
- Use `context.WithTimeout(60s)` for the clone command

**Do NOT call `gitutil.Clone()`** — it doesn't exist. Either extract the hardened
clone logic to `gitutil.CloneShallow()` as part of this task, or inline the
`exec.CommandContext("git", ...)` call with the flags above.

Returns the temp dir path in `addDiscoveryDoneMsg.tmpDir`. The wizard stores it
in `m.gitTempDir` for cleanup on close, back-navigation, or re-clone.

After successful clone, probe registry visibility via `registry.ProbeVisibility(url)`
and store result in `m.sourceRegistry` / `m.sourceVisibility` for taint propagation.

---

## Testing Strategy

### Checkbox List Tests (checkbox_list_test.go)

| Test | Success Criteria |
|------|-----------------|
| `TestCheckboxList_Navigation` | Up/Down moves cursor, wraps at boundaries |
| `TestCheckboxList_Toggle` | Space toggles selected state |
| `TestCheckboxList_ToggleDisabled` | Space on disabled item is no-op |
| `TestCheckboxList_SelectAll` | `a` selects all non-disabled |
| `TestCheckboxList_SelectNone` | `n` deselects all |
| `TestCheckboxList_DrillIn` | Enter emits checkboxDrillInMsg with correct index |
| `TestCheckboxList_Scrolling` | Cursor past visible area scrolls offset |
| `TestCheckboxList_View` | Checked/unchecked rendering matches expected format |
| `TestCheckboxList_ViewDisabled` | Disabled items show `[-]` and muted style |
| `TestCheckboxList_View_SanitizesEscapeCodes` | Label with ANSI escape `\x1b[1m` renders without the code |

### Add Wizard Flow Tests (add_wizard_test.go)

| Test | Success Criteria |
|------|-----------------|
| `TestAddWizard_SourceStep_ProviderSelect` | Selecting provider + sub-provider advances to Type |
| `TestAddWizard_SourceStep_RegistrySelect` | Selecting registry advances to Type |
| `TestAddWizard_SourceStep_LocalPath` | Entering path + Enter advances to Type |
| `TestAddWizard_SourceStep_NoProviders` | Provider option disabled when none detected |
| `TestAddWizard_SourceStep_NoRegistries` | Registry option disabled when none configured |
| `TestAddWizard_TypeStep_MultiSelect` | Toggle types, advance with Enter |
| `TestAddWizard_TypeStep_PreFiltered` | Step skipped when preFilterType set |
| `TestAddWizard_DiscoveryStep_Loading` | Shows spinner during async scan |
| `TestAddWizard_DiscoveryStep_Results` | Items rendered with status and risk badges |
| `TestAddWizard_DiscoveryStep_PreSelected` | New/Outdated pre-selected, InLibrary not |
| `TestAddWizard_DiscoveryStep_Empty` | "No content found" message shown |
| `TestAddWizard_DiscoveryStep_Error` | Error renders inline with retry option |
| `TestAddWizard_DiscoveryStep_StaleResult` | discoveryDoneMsg with old seq ignored |
| `TestAddWizard_ReviewStep_RiskBanner` | Risk banner shown for items with risks |
| `TestAddWizard_ReviewStep_Conflicts` | Outdated items shown with conflict indicator |
| `TestAddWizard_ReviewStep_DoubleConfirm` | Second Enter on Add button ignored |
| `TestAddWizard_ExecuteStep_Progress` | Items show progress indicators sequentially |
| `TestAddWizard_ExecuteStep_Cancel` | Esc stops remaining items, shows partial |
| `TestAddWizard_ExecuteStep_Done` | Enter on done screen sends addCloseMsg |
| `TestAddWizard_BackNav_ReviewToDiscovery` | Selections preserved, no re-scan |
| `TestAddWizard_BackNav_DiscoveryToType` | Discovery cleared, will re-scan |
| `TestAddWizard_BackNav_TypeChangeInvalidates` | Type change clears stale discovery |
| `TestAddWizard_BackNav_SourceChangeInvalidates` | Source change clears everything |
| `TestAddWizard_EscFromSource` | Closes wizard |
| `TestAddWizard_EscFromType` | Goes back to Source |
| `TestAddWizard_EscFromDiscovery` | Goes back to Type (or Source if pre-filtered) |
| `TestAddWizard_EscFromReview` | Goes back to Discovery |
| `TestAddWizard_EscDuringDiscovery` | Abandons scan (result ignored via seq), goes back |
| `TestAddWizard_DiscoveryStep_KeysIgnoredDuringScan` | Enter/Space/a/n ignored while discovering=true |
| `TestAddWizard_DiscoveryStep_ErrorRetry` | `r` increments seq, sends new discovery cmd, clears error |
| `TestAddWizard_SourceStep_EscCollapsesSubList` | Esc with expanded provider list collapses, doesn't close wizard |
| `TestAddWizard_SourceStep_TextInputNavigation` | Up/Down deactivates text input, moves cursor |
| `TestAddWizard_GitURL_RejectExtProtocol` | `ext::cmd` rejected with inline error, no clone attempted |
| `TestAddWizard_GitURL_RejectFileProtocol` | `file:///etc` rejected with inline error |
| `TestAddWizard_GitURL_ValidFormats` | `https://` and `git@` URLs accepted |
| `TestAddWizard_RegistryTaint_Propagated` | Items from registry have SourceRegistry in metadata |
| `TestAddWizard_GitTaint_Propagated` | Items from git URL have SourceRegistry + SourceVisibility in metadata |
| `TestAddWizard_ReviewStep_DefaultFocusOnBack` | Initial focus is buttons zone, cursor on [Back] |

### Wizard Invariant Tests (wizard_invariant_test.go additions)

| Test | Success Criteria |
|------|-----------------|
| `TestAddWizard_ValidateStep_Forward` | All 5 steps without panic |
| `TestAddWizard_ValidateStep_BackNav` | Navigate back from each step without panic |
| `TestAddWizard_ValidateStep_PreFiltered` | 4-step path without panic |
| `TestAddWizard_ValidateStep_PanicOnEmpty` | Entering Type without source panics |
| `TestAddWizard_ValidateStep_PanicNoItems` | Entering Review without items panics |

### App Integration Tests

| Test | Success Criteria |
|------|-----------------|
| `TestApp_AddKey_Library` | `[a]` on Library opens add wizard |
| `TestApp_AddKey_ContentTab` | `[a]` on Content tab opens add wizard with preFilter |
| `TestApp_AddKey_RegistryTab` | `[a]` on Registries tab opens registry add (existing behavior) |
| `TestApp_AddKey_DuringWizard` | `[a]` suppressed while wizard active |
| `TestApp_AddWizard_Close` | addCloseMsg clears wizard state |
| `TestApp_AddWizard_GlobalKeySuppression` | 1/2/3, R, q suppressed during wizard |

### Golden Files

| Name | Size | Content |
|------|------|---------|
| `add-wizard-source-80x30` | 80×30 | Source step with all options |
| `add-wizard-source-provider-expanded-80x30` | 80×30 | Provider sub-list expanded |
| `add-wizard-type-80x30` | 80×30 | Type selection checkboxes |
| `add-wizard-discovery-loading-80x30` | 80×30 | Spinner during scan |
| `add-wizard-discovery-results-80x30` | 80×30 | Item list with badges |
| `add-wizard-discovery-empty-80x30` | 80×30 | No items found |
| `add-wizard-review-80x30` | 80×30 | Risk banner + item list |
| `add-wizard-execute-progress-80x30` | 80×30 | Mid-execution |
| `add-wizard-execute-done-80x30` | 80×30 | Completion screen |
| All above at 60×20 | 60×20 | Minimum size variants |
| All above at 120×40 | 120×40 | Wide terminal variants |

---

## Implementation Task Breakdown

### Task 1: Checkbox List Component
- Create `checkbox_list.go` with struct, constructor, Update, View
- Create `checkbox_list_test.go` with all component tests
- **Files:** `checkbox_list.go`, `checkbox_list_test.go`
- **Success criteria:** All 9 checkbox list tests pass. Component renders correctly at multiple widths.

### Task 2: Add Wizard Model + Source Step
- Create `add_wizard.go` with model struct, step enum, source types, constructor, validateStep
- Create `add_wizard_update.go` with Update() and source step key handling
- Create `add_wizard_view.go` with View() and source step rendering
- Wire `wizardAdd` into `app.go` (enum value), `app_update.go` (routing + `[a]` key), `app_view.go` (rendering)
- **Files:** `add_wizard.go`, `add_wizard_update.go`, `add_wizard_view.go`, `app.go`, `app_update.go`, `app_view.go`, `actions.go`
- **Success criteria:** `[a]` opens wizard. Source step renders with all 4 options. Provider sub-list expands. Registry sub-list expands. Esc closes wizard. Global keys suppressed.

### Task 3: Type Step
- Add type step key handling and rendering
- Skip logic when preFilterType is set
- **Files:** `add_wizard_update.go`, `add_wizard_view.go`, `add_wizard.go`
- **Success criteria:** Type checkboxes render. Space toggles. Enter advances when ≥1 selected. Step skipped when pre-filtered. Shell shows correct step count.

### Task 4: Discovery Step — Backend + UI
- Implement discovery backend functions (provider, registry, local path)
- Add discovery step UI: spinner, results list, error state
- Wire async discovery cmd with seq-based stale message handling
- **Files:** `add_wizard.go`, `add_wizard_update.go`, `add_wizard_view.go`
- **Success criteria:** Async scan launches on step entry. Results render with checkboxes and badges. Stale results ignored. Error shown inline. Empty state shown. Pre-selection correct (New/Outdated selected, InLibrary not).

### Task 5: Review Step
- Add review step rendering with risk banner and item summary
- Tab cycling: risks → items → buttons
- Conflict detection for outdated items
- Double-confirm prevention
- **Files:** `add_wizard_update.go`, `add_wizard_view.go`
- **Success criteria:** Risk banner appears for risky items. Tab cycles zones correctly. Conflict items marked. Double-Enter prevented. Back preserves selections.

### Task 6: Execute Step
- Sequential item addition via chained tea.Cmds
- Progress rendering with per-item status
- Cancellation support (Esc stops remaining)
- Completion screen with Enter to close
- **Files:** `add_wizard_update.go`, `add_wizard_view.go`, `add_wizard.go`
- **Success criteria:** Items added sequentially with progress. Esc cancels remaining. Partial completion shown. Enter on done closes wizard. Toast fires. Catalog rescanned.

### Task 7: Tests + Golden Files
- Add all flow tests to `add_wizard_test.go`
- Add invariant tests to `wizard_invariant_test.go`
- Add app integration tests to `app_test.go`
- Generate golden files at 3 sizes
- **Files:** `add_wizard_test.go`, `wizard_invariant_test.go`, `app_test.go`, `testdata/*.golden`
- **Success criteria:** All tests pass. Coverage ≥80% for new code. Golden files generated at 60×20, 80×30, 120×40. `make test` passes clean.

### Task 8: Git URL Source (Stretch)
- Clone to temp dir, scan, clean up on wizard close
- Context-based timeout (60s)
- **Files:** `add_wizard.go`, `add_wizard_update.go`
- **Success criteria:** Git URL clone works with timeout. Temp dir cleaned on close. Error shown on timeout/failure.

---

## Scope Boundaries — What NOT to Build

- **No code scanning research spike** — use existing `catalog.RiskIndicators()` as-is
- **No side-by-side diff** for conflicts — just show "(update — content differs)" text
- **No format conversion UI** — conversion happens silently via `add.AddItems()` backend
- **No registry item drill-in entry point** — just `[a]` from Library and Content tabs
- **No Create wizard** — `[n]` remains hidden (deferred)
- **No remote install locations** — local only
- **No mouse support in checkbox list** — keyboard only (mouse can be added later)
