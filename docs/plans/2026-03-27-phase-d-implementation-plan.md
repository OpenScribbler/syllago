# Phase D: Implementation Plan

**Date:** 2026-03-27
**Spec:** `docs/plans/2026-03-27-phase-d-add-wizard-spec.md` (v2)
**Status:** Plan v3 — post-sanity-check (3 BLOCKs + 9 WARNs addressed)

---

## Task 1: Checkbox List Component

**Files:** `cli/internal/tui/checkbox_list.go`, `cli/internal/tui/checkbox_list_test.go`

### checkbox_list.go

Create the reusable multi-select list component. All value receivers (matching
`riskBanner` pattern).

**Types:**

```go
type checkboxBadgeStyle int

const (
    badgeStyleNone    checkboxBadgeStyle = iota
    badgeStyleDanger
    badgeStyleWarning
    badgeStyleSuccess
    badgeStyleMuted
)

type checkboxItem struct {
    label      string
    description string
    disabled   bool
    badge      string
    badgeStyle checkboxBadgeStyle
}

type checkboxList struct {
    items    []checkboxItem
    selected []bool
    cursor   int
    offset   int
    width    int
    height   int
    focused  bool
}
```

**Functions:**

```go
func newCheckboxList(items []checkboxItem) checkboxList
// Initialize selected = make([]bool, len(items)), cursor = 0, offset = 0

func (c checkboxList) SetSize(w, h int) checkboxList
// Return copy with updated width/height

func (c checkboxList) SelectedIndices() []int
// Return indices where selected[i] == true

func (c checkboxList) Update(msg tea.KeyMsg) (checkboxList, tea.Cmd)
// Key handling:
//   Up/k: cursor--, clamp to 0, adjust offset
//   Down/j: cursor++, clamp to len-1, adjust offset
//   Space: toggle selected[cursor] (skip if disabled)
//   'a': set all non-disabled to true
//   'n': set all to false
//   Enter: return checkboxDrillInMsg{cursor}
//   PgUp: cursor -= height, clamp
//   PgDn: cursor += height, clamp
//   Home: cursor = 0
//   End: cursor = len-1

func (c checkboxList) View() string
// Render visible rows (offset to offset+height):
//   cursor indicator: ">" if focused && i == cursor, else " "
//   checkbox: "[x]" if selected, "[ ]" if not, "[-]" if disabled
//   label (sanitized via sanitizeLine + stripAnsi)
//   badge right-aligned, colored by badgeStyle -> palette color
//   disabled rows: all in mutedStyle
```

**Helper:**

```go
func stripAnsi(s string) string
// Strip ANSI escape sequences from untrusted content
// Simple regex: \x1b\[[0-9;]*[a-zA-Z]

func badgeColor(style checkboxBadgeStyle) lipgloss.TerminalColor
// badgeStyleDanger -> dangerColor
// badgeStyleWarning -> warningColor
// badgeStyleSuccess -> successColor
// badgeStyleMuted -> mutedFg (or mutedStyle foreground)
// badgeStyleNone -> nil (no badge)
```

**Message:**

```go
type checkboxDrillInMsg struct {
    index int
}
```

### checkbox_list_test.go

10 tests, all using `t.Parallel()`:

| # | Test | What to Assert |
|---|------|---------------|
| 1 | `TestCheckboxList_Navigation` | cursor starts at 0. Down moves to 1. Up from 0 stays at 0. Down past end stays at end. |
| 2 | `TestCheckboxList_Toggle` | Space on item 0 sets selected[0]=true. Space again sets false. `SelectedIndices()` returns [0] then []. |
| 3 | `TestCheckboxList_ToggleDisabled` | Space on disabled item: selected stays false. |
| 4 | `TestCheckboxList_SelectAll` | 3 items (1 disabled). 'a' key: selected = [true, false, true]. Disabled stays false. |
| 5 | `TestCheckboxList_SelectNone` | Pre-select all. 'n' key: all false. |
| 6 | `TestCheckboxList_DrillIn` | Enter on cursor=2: returns `checkboxDrillInMsg{index: 2}`. |
| 7 | `TestCheckboxList_Scrolling` | height=3, 10 items. Down x5: offset adjusts so cursor stays visible. PgDn: jumps by height. |
| 8 | `TestCheckboxList_View` | 3 items, item 1 selected. View contains `[x]` for item 1, `[ ]` for others. Cursor indicator `>` on focused row. |
| 9 | `TestCheckboxList_ViewDisabled` | Disabled item renders `[-]` and badge in muted style. |
| 10 | `TestCheckboxList_View_SanitizesEscapeCodes` | Item with label `"test\x1b[1mbold"` renders as `"testbold"` (escape stripped). |

**Success criteria:** All 10 tests pass. `go test ./internal/tui/ -run TestCheckboxList` green.

---

## Task 2: Add Wizard Model + Source Step

**Files:** `cli/internal/tui/add_wizard.go`, `cli/internal/tui/add_wizard_update.go`, `cli/internal/tui/add_wizard_view.go`, `cli/internal/tui/app.go`, `cli/internal/tui/app_update.go`, `cli/internal/tui/app_view.go`, `cli/internal/tui/actions.go`

### add_wizard.go

**Step enum:**

```go
type addStep int

const (
    addStepSource    addStep = iota
    addStepType
    addStepDiscovery
    addStepReview
    addStepExecute
)
```

**Source types:**

```go
type addSource int

const (
    addSourceNone     addSource = iota
    addSourceProvider
    addSourceRegistry
    addSourceLocal
    addSourceGit
)
```

**Review zones:**

```go
type addReviewZone int

const (
    addReviewZoneRisks   addReviewZone = iota
    addReviewZoneItems
    addReviewZoneButtons
)
```

**Model struct:** Full struct as specified in spec v2 (see spec for complete definition).
Key fields for Task 2: `shell`, `step`, `width`, `height`, `seq`, `source`, `sourceCursor`,
`sourceExpanded`, `inputActive`, `providers`, `providerCursor`, `registries`, `registryCursor`,
`pathInput`, `pathCursor`, `preFilterType`, `projectRoot`, `contentRoot`, `cfg`.

**Constructor:**

```go
func openAddWizard(
    providers []provider.Provider,
    registries []catalog.RegistrySource,
    cfg *config.Config,
    projectRoot string,
    contentRoot string,
    preFilterType catalog.ContentType,
) *addWizardModel
```

Logic:
- Filter providers to `Detected == true`
- Set step labels based on preFilterType:
  - 5-step: `["Source", "Type", "Discovery", "Review", "Execute"]`
  - 4-step: `["Source", "Discovery", "Review", "Execute"]`
- Default `sourceCursor` to 0 (Provider) if any detected, else 2 (Local)
- Initialize shell via `newWizardShell("Add", stepLabels)`

**validateStep():**

```go
func (m *addWizardModel) validateStep() {
    switch m.step {
    case addStepSource:
        // no prerequisites
    case addStepType:
        if m.source == addSourceNone {
            panic("wizard invariant: addStepType entered without source")
        }
    case addStepDiscovery:
        // Added in Task 3
        if m.source == addSourceNone {
            panic("wizard invariant: addStepDiscovery entered without source")
        }
        if len(m.selectedTypes()) == 0 {
            panic("wizard invariant: addStepDiscovery entered without selected types")
        }
    case addStepReview:
        // Added in Task 5
        if len(m.discoveredItems) == 0 {
            panic("wizard invariant: addStepReview entered without discovered items")
        }
        if len(m.selectedItems()) == 0 {
            panic("wizard invariant: addStepReview entered without selected items")
        }
    case addStepExecute:
        // Added in Task 6
        if len(m.selectedItems()) == 0 {
            panic("wizard invariant: addStepExecute entered without selected items")
        }
        if !m.reviewAcknowledged {
            panic("wizard invariant: addStepExecute entered without review acknowledgment")
        }
    }
}
```

**Task ownership:** Task 2 creates the function with Source + Type cases.
Task 3 adds Discovery case. Task 5 adds Review case. Task 6 adds Execute case.
Each task's success criteria includes: "validateStep for this step does not panic on valid entry."

**Helper methods:**

```go
func (m *addWizardModel) selectedTypes() []catalog.ContentType
func (m *addWizardModel) selectedItems() []addDiscoveryItem
```

Both as specified in the spec v2 model section.

**Init:**

```go
func (m *addWizardModel) Init() tea.Cmd { return nil }
```

### add_wizard_update.go

```go
func (m *addWizardModel) Update(msg tea.Msg) (*addWizardModel, tea.Cmd) {
    m.validateStep()
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.updateKey(msg)
    case tea.MouseMsg:
        return m.updateMouse(msg)
    }
    return m, nil
}

func (m *addWizardModel) updateKey(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
    switch m.step {
    case addStepSource:
        return m.updateKeySource(msg)
    // ... (remaining steps in later tasks)
    }
    return m, nil
}
```

**Source step key handler — `updateKeySource`:**

Three modes based on `sourceExpanded` and `inputActive`:

**Mode 1: Top-level radio (neither expanded nor input active)**
- `Up/k`: sourceCursor-- (clamp 0)
- `Down/j`: sourceCursor++ (clamp 3)
- `Enter`:
  - cursor 0 (Provider): if any detected, set `sourceExpanded=true`, `providerCursor` to first detected
  - cursor 1 (Registry): if any configured, set `sourceExpanded=true`, `registryCursor=0`
  - cursor 2 (Local): set `inputActive=true`
  - cursor 3 (Git): set `inputActive=true`
  - Disabled options: no-op
- `Esc`: return `addCloseMsg{}`

**Mode 2: Sub-list expanded (sourceExpanded=true)**
- `Up/k`: move sub-cursor (providerCursor or registryCursor)
- `Down/j`: move sub-cursor
- `Enter`: set new source. If source changed from previous value, clear downstream
  state (`typeChecks = newCheckboxList(...)`, `discoveredItems = nil`,
  `discoveryList = checkboxList{}`, `risks = nil`, `reviewAcknowledged = false`,
  `seq++`). Then call `advanceFromSource()`.
- `Esc`: collapse sub-list (`sourceExpanded=false`)

**Mode 3: Text input active (inputActive=true)**
- Standard text editing: runes append to `pathInput`, backspace, Ctrl+A/E
- `Up/k`: deactivate input, move sourceCursor up
- `Down/j`: deactivate input, move sourceCursor down
- `Enter`:
  - If Local (cursor 2): validate path (absolute, exists, is dir). If valid, set `source=addSourceLocal`, advance. If invalid, set inline error.
  - If Git (cursor 3): validate URL (https://, git@, ssh://). Reject ext::, fd::, file://. If valid, set `source=addSourceGit`, advance. If invalid, set inline error.
  - Empty input: no-op
- `Esc`: if pathInput non-empty, clear it and deactivate. If empty, deactivate.

**Advancing to next step:**

```go
func (m *addWizardModel) advanceFromSource() {
    // Always clear downstream state on advance from Source (simple, always correct).
    // This avoids needing a prevSource field — the cost of re-initializing typeChecks
    // is negligible compared to the bug risk of showing stale discovery for a new source.
    m.typeChecks = m.buildTypeCheckList() // re-init with source-appropriate defaults
    m.discoveredItems = nil
    m.discoveryList = checkboxList{}
    m.discoveryErr = ""
    m.risks = nil
    m.reviewAcknowledged = false
    m.seq++

    if m.preFilterType != "" {
        m.step = addStepDiscovery
        m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
    } else {
        m.step = addStepType
        m.shell.SetActive(m.shellIndexForStep(addStepType))
    }
}

// RULE: Always use m.shellIndexForStep(step) instead of hardcoded SetActive(N).
// This handles both 4-step and 5-step paths correctly.
```

### add_wizard_view.go

```go
func (m *addWizardModel) View() string {
    if m == nil {
        return ""
    }
    header := m.shell.View()
    var content string
    switch m.step {
    case addStepSource:
        content = m.viewSource()
    // ... (remaining steps in later tasks)
    }
    output := header + "\n" + content
    outputLines := strings.Count(output, "\n") + 1
    if outputLines < m.height {
        output += strings.Repeat("\n", m.height-outputLines)
    }
    return output
}
```

**viewSource():**
- Title: "Where is the content?"
- 4 radio options using `[ ]` / `[*]` glyphs
- `>` cursor indicator
- Disabled options (no providers / no registries) shown muted with explanation
- When `sourceExpanded`: inline sub-list below the selected radio option
- When `inputActive`: text input field with block cursor
- Inline error for invalid path/URL (if set)

### App wiring

**app.go** — Add `wizardAdd` to enum, `addWizard *addWizardModel` to struct.

**app_update.go** — Three changes:
1. `routeToWizard`: add `case wizardAdd:` (delegates to `a.addWizard.Update(msg)`)
2. `case addCloseMsg:` handler (cleanup gitTempDir, nil wizard, rescanCatalog)
3. `keyAdd` handler: add `if a.isLibraryTab() || a.isContentTab() { return a.handleAdd() }`

**app_view.go** — `renderContent`: add `wizardAdd` case.

**actions.go** — Add `handleAdd()` function (as specified in spec).

**WindowSizeMsg** — Propagate to addWizard (width, height, shell.SetWidth).

**Success criteria:**
- `[a]` on Library tab opens add wizard (view contains "Where is the content?")
- `[a]` on Content > Rules tab opens add wizard with preFilterType=Rules (4-step shell)
- `[a]` on Registries tab still opens registry add modal (existing behavior preserved)
- Provider sub-list expands on Enter, collapses on Esc
- Text input works for Local/Git options
- Git URL validation rejects `ext::malicious` with inline error
- Local path validation rejects relative paths
- Esc from top-level Source closes wizard
- Esc from expanded sub-list collapses sub-list (does NOT close wizard)
- Global keys (1/2/3, R, q) suppressed during wizard
- `make build` succeeds

---

## Task 3: Type Step

**Files:** `cli/internal/tui/add_wizard_update.go`, `cli/internal/tui/add_wizard_view.go`, `cli/internal/tui/add_wizard.go`

### add_wizard_update.go additions

**updateKeyType():**

```go
func (m *addWizardModel) updateKeyType(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
    switch {
    case msg.Type == tea.KeyEsc:
        // Go back to Source
        m.step = addStepSource
        m.shell.SetActive(0)
        return m, nil

    case msg.Type == tea.KeyEnter:
        // Advance if at least one type selected
        if len(m.selectedTypes()) > 0 {
            m.step = addStepDiscovery
            m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
            m.discovering = true
            return m, m.startDiscoveryCmd()
        }
        return m, nil

    default:
        // Delegate to checkboxList
        var cmd tea.Cmd
        m.typeChecks, cmd = m.typeChecks.Update(msg)
        return m, cmd
    }
}
```

### add_wizard_view.go additions

**viewType():**

```go
func (m *addWizardModel) viewType() string
```

- Title: "What type of content?"
- Render `m.typeChecks.View()` (single-column checkbox list)
- Help line: `[space] toggle  [a] all  [n] none  [enter] next`

### add_wizard.go additions

In the constructor, initialize `typeChecks`:

```go
typeItems := []checkboxItem{
    {label: "Rules"},
    {label: "Skills"},
    {label: "Agents"},
    {label: "Hooks"},
    {label: "MCP"},
    {label: "Commands"},
}
// For provider source: pre-check supported types
// For other sources: all checked by default
m.typeChecks = newCheckboxList(typeItems)
// Set initial selections based on source type
```

Pre-check logic is applied after source is selected in `advanceFromSource()`.

**Success criteria:**
- Type step renders 6 checkboxes in single column
- Space toggles checkbox
- `a` selects all, `n` deselects all
- Enter with ≥1 selected advances to Discovery (starts async scan)
- Enter with 0 selected: no-op
- Esc goes back to Source step
- When preFilterType set: Type step skipped entirely
- Shell shows `[1 Source] [2 Discovery] ...` in 4-step path
- Shell shows `[1 Source] [2 Type] [3 Discovery] ...` in 5-step path
- `make build` succeeds

---

## Task 4: Discovery Step — Backend + UI

**Files:** `cli/internal/tui/add_wizard.go`, `cli/internal/tui/add_wizard_update.go`, `cli/internal/tui/add_wizard_view.go`

### Discovery backend functions (in add_wizard.go)

All functions are package-level (not methods), called from tea.Cmd goroutines.
They receive all data as parameters, NEVER access model state.

```go
func discoverFromProvider(
    prov provider.Provider,
    projectRoot string,
    resolver *config.PathResolver, // NOT *config.Config — real API needs PathResolver
    contentRoot string,
    selectedTypes []catalog.ContentType,
) ([]addDiscoveryItem, error)
```

**Note:** `add.DiscoverFromProvider()` takes `*config.PathResolver`, not `*config.Config`.
The caller (wizard's `startDiscoveryCmd`) must construct the resolver:
```go
resolver := config.NewResolver(config.Merge(globalCfg, projectCfg), "")
resolver.ExpandPaths() // ignore error — non-fatal for discovery
```
This can be done once in `openAddWizard()` and stored on the model, or constructed
fresh in each `startDiscoveryCmd` closure from the stored `cfg`.

- Calls `add.DiscoverFromProvider()` for file-based types
- For hooks: calls hook discovery (adapts `discoverHooksForDisplay` pattern from add_cmd.go)
- For MCP: calls MCP discovery (adapts `discoverMcpForDisplay` pattern)
- Filters by selectedTypes
- Annotates each item with risks: build `catalog.ContentItem` from path, call `catalog.RiskIndicators()`

```go
func discoverFromRegistry(
    reg catalog.RegistrySource,
    selectedTypes []catalog.ContentType,
    contentRoot string,
) ([]addDiscoveryItem, error)
```
- Calls `catalog.ScanRegistriesOnly([]catalog.RegistrySource{reg})`
- Filters by selectedTypes
- Computes library status via `add.BuildLibraryIndex()` + key comparison

```go
func discoverFromLocalPath(
    dir string,
    selectedTypes []catalog.ContentType,
    contentRoot string,
) ([]addDiscoveryItem, error)
```
- `catalog.ScanNativeContent(dir)` first
- If `HasSyllagoStructure`: `catalog.Scan(dir, dir)` instead
- Filter, annotate with risk + library status

### addDiscoveryItem type (in add_wizard.go)

```go
type addDiscoveryItem struct {
    name        string
    displayName string
    itemType    catalog.ContentType
    path        string
    sourceDir   string
    status      add.ItemStatus
    scope       string
    risks       []catalog.RiskIndicator
    overwrite   bool
    // Embed the underlying add.DiscoveryItem for Execute step
    underlying  *add.DiscoveryItem // nil for hooks/MCP (they use separate add path)
}
```

### Async discovery command

```go
func (m *addWizardModel) startDiscoveryCmd() tea.Cmd {
    seq := m.seq
    source := m.source
    // Capture all needed params from model
    switch source {
    case addSourceProvider:
        prov := m.providers[m.providerCursor]
        types := m.selectedTypes()
        projectRoot := m.projectRoot
        cfg := m.cfg
        contentRoot := m.contentRoot
        return func() tea.Msg {
            items, err := discoverFromProvider(prov, projectRoot, cfg, contentRoot, types)
            return addDiscoveryDoneMsg{seq: seq, items: items, err: err}
        }
    case addSourceRegistry:
        // similar capture pattern
    case addSourceLocal:
        // similar
    }
    return nil
}
```

### Discovery step UI (updateKeyDiscovery)

Three sub-modes:
1. **Discovering** (`m.discovering == true`): only Esc accepted (abandons scan, `seq++`, go back)
2. **Error** (`m.discoveryErr != ""`): `r` retries, Esc goes back
3. **Results** (normal): delegate to checkboxList. `→` (Right arrow) advances to Review.

```go
func (m *addWizardModel) updateKeyDiscovery(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
    // Guard: during scan, only Esc works
    if m.discovering {
        if msg.Type == tea.KeyEsc {
            m.seq++
            m.discovering = false
            m.goBackFromDiscovery()
            return m, nil
        }
        return m, nil // ignore all other keys
    }

    // Error state
    if m.discoveryErr != "" {
        if msg.String() == "r" {
            m.discoveryErr = ""
            m.seq++
            m.discovering = true
            return m, m.startDiscoveryCmd()
        }
        if msg.Type == tea.KeyEsc {
            m.goBackFromDiscovery()
            return m, nil
        }
        return m, nil
    }

    // Normal results
    switch {
    case msg.Type == tea.KeyEsc:
        m.goBackFromDiscovery()
        return m, nil
    case msg.Type == tea.KeyRight:
        if len(m.discoveryList.SelectedIndices()) > 0 {
            m.enterReview()
            return m, nil
        }
    default:
        var cmd tea.Cmd
        m.discoveryList, cmd = m.discoveryList.Update(msg)
        return m, cmd
    }
    return m, nil
}
```

**Handle addDiscoveryDoneMsg in Update:**

The `addDiscoveryDoneMsg` carries taint fields for registry/git sources:

```go
type addDiscoveryDoneMsg struct {
    seq              int
    items            []addDiscoveryItem
    err              error
    tmpDir           string // non-empty for git URL source
    sourceRegistry   string // taint: registry name
    sourceVisibility string // taint: "public", "private", "unknown"
}
```

Handler:
```go
case addDiscoveryDoneMsg:
    if msg.seq != m.seq {
        return m, nil // stale
    }
    m.discovering = false
    if msg.err != nil {
        m.discoveryErr = msg.err.Error()
        // Toast is handled at App level — see below
        return m, nil
    }
    m.discoveredItems = msg.items
    if msg.tmpDir != "" {
        m.gitTempDir = msg.tmpDir
    }
    // Store taint for Execute step
    m.sourceRegistry = msg.sourceRegistry
    m.sourceVisibility = msg.sourceVisibility
    // Build checkboxList from results
    m.discoveryList = m.buildDiscoveryList()
    return m, nil
```

Each discovery backend populates taint in its return message:
- `discoverFromProvider`: leaves taint empty (auto-detected by `add.writeItem`)
- `discoverFromRegistry`: sets `sourceRegistry = reg.Name`, calls `registry.ProbeVisibility(reg.URL)`
- `discoverFromGitURL`: sets `sourceRegistry = registry.NameFromURL(url)`, calls `registry.ProbeVisibility(url)`

**Toast mechanism:** Wizards cannot call `a.toast.Push()` directly (they don't have
access to the App). Instead, the App handles `addDiscoveryDoneMsg` at the App level
for the error case, same pattern as `installDoneMsg`:

```go
// In app_update.go (NOT inside routeToWizard):
case addDiscoveryDoneMsg:
    // Route to wizard for state management
    if a.addWizard != nil {
        _, cmd := a.addWizard.Update(msg)
        // Also fire toast if error
        if msg.err != nil {
            toastCmd := a.toast.Push("Discovery failed: "+msg.err.Error(), toastError)
            return a, tea.Batch(cmd, toastCmd)
        }
        return a, cmd
    }
```

This means `addDiscoveryDoneMsg` must be handled at the App level BEFORE the
generic `routeToWizard` dispatch. Add it alongside the existing `installResultMsg`,
`installDoneMsg`, etc. handlers in the message switch.

Similarly, `addExecAllDoneMsg` fires the completion toast at App level:
```go
case addExecAllDoneMsg:
    if a.addWizard != nil {
        count := len(a.addWizard.selectedItems())
        cmd := a.toast.Push(fmt.Sprintf("Added %d items to library", count), toastSuccess)
        return a, cmd
    }
```

**buildDiscoveryList():** Creates `checkboxItem` per discovered item, pre-selects New/Outdated,
leaves InLibrary deselected. Badges from risk indicators.

**IMPORTANT: Auto-set overwrite for Outdated items.** When building the discovery list,
set `item.overwrite = true` for any item with `status == add.StatusOutdated`. This is
required because `add.AddItems()` returns `AddStatusSkipped` for Outdated items unless
`Force=true`. Without this, Outdated items silently fail to update despite being selected.

### viewDiscovery()

- If `discovering`: spinner + "Discovering content..."
- If `discoveryErr`: error box (rounded corners, dangerColor border) with `[r] Retry [Esc] Back`
- If `len(discoveredItems) == 0`: "No content found" message
- Otherwise: count header + `discoveryList.View()` + help line

**Success criteria:**
- Entering Discovery triggers async scan (spinner visible)
- Provider discovery returns correct items with status and risk badges
- Registry discovery filters by selected types
- Local path discovery handles both native and syllago structures
- Stale discoveryDoneMsg (old seq) silently dropped
- Error renders inline with rounded corners, `r` retries, Esc goes back
- Empty discovery shows "No content found"
- Pre-selection: New/Outdated checked, InLibrary unchecked
- Enter drills into item (checkboxDrillInMsg)
- Right arrow advances to Review when ≥1 selected
- Keys ignored during scan (no panic on empty list)
- `make build` succeeds

---

## Task 5: Review Step

**Files:** `cli/internal/tui/add_wizard_update.go`, `cli/internal/tui/add_wizard_view.go`

### enterReview()

```go
func (m *addWizardModel) enterReview() {
    m.step = addStepReview
    shellIdx := m.shellIndexForStep(addStepReview)
    m.shell.SetActive(shellIdx)

    // Compute aggregate risks from selected items
    m.risks = nil
    for _, item := range m.selectedItems() {
        m.risks = append(m.risks, item.risks...)
    }
    m.riskBanner = newRiskBanner(m.risks, m.width-4)

    // Detect conflicts (Outdated status)
    m.conflicts = nil
    for _, idx := range m.discoveryList.SelectedIndices() {
        if m.discoveredItems[idx].status == add.StatusOutdated {
            m.conflicts = append(m.conflicts, idx)
        }
    }

    // Default focus: buttons zone, cursor on [Back]
    m.reviewZone = addReviewZoneButtons
    m.buttonCursor = 1 // [Back]
    m.reviewAcknowledged = false
}
```

### updateKeyReview()

Tab cycles zones. Per-zone key handling:

**Risks zone:** delegate to riskBanner.Update(). Tab -> next zone.
**Items zone:** Up/Down scroll item list. Tab -> next zone.
**Buttons zone:** Left/Right moves buttonCursor (0=Cancel, 1=Back, 2=Add).
  - Enter on Cancel: addCloseMsg
  - Enter on Back: go back to Discovery (selections preserved)
  - Enter on Add: if not reviewAcknowledged, set it, advance to Execute

### viewReview()

- Header: "Adding N items to library:"
- Risk banner via `m.riskBanner.ViewInline()` (if non-empty)
- Item list with styles per spec (name=bold, type=muted, status colored, badges colored)
- Buttons: `[Cancel] [Back] [Add N items]` via install wizard button rendering pattern
- Active button highlighted, others muted

**Success criteria:**
- Risk banner appears when items have risks, absent when none
- Tab cycles: risks → items → buttons (skips risks when none)
- Default focus on [Back] button
- Enter on [Add N items] sets reviewAcknowledged, advances to Execute
- Double-Enter on [Add] ignored (reviewAcknowledged already true)
- Enter on [Back] returns to Discovery with selections preserved
- Enter on [Cancel] closes wizard
- Conflict items shown with "(update — content differs)" in warningColor
- Esc from review = go back to Discovery (not close)
- `make build` succeeds

---

## Task 6: Execute Step

**Files:** `cli/internal/tui/add_wizard_update.go`, `cli/internal/tui/add_wizard_view.go`, `cli/internal/tui/add_wizard.go`

### enterExecute()

```go
func (m *addWizardModel) enterExecute() {
    m.step = addStepExecute
    shellIdx := m.shellIndexForStep(addStepExecute)
    m.shell.SetActive(shellIdx)

    selected := m.selectedItems()
    m.executeResults = make([]addExecResult, len(selected))
    m.executeCurrent = 0
    m.executeDone = false
    m.executeCancelled = false
    m.executing = true
}
```

### addItemCmd(index)

```go
func (m *addWizardModel) addItemCmd(index int) tea.Cmd {
    seq := m.seq
    items := m.selectedItems()
    item := items[index]
    contentRoot := m.contentRoot
    sourceReg := m.sourceRegistry
    sourceVis := m.sourceVisibility

    return func() tea.Msg {
        result := addSingleItem(item, contentRoot, sourceReg, sourceVis)
        return addExecItemDoneMsg{seq: seq, index: index, result: result}
    }
}
```

**addSingleItem()** — package-level, called from goroutine:

```go
func addSingleItem(item addDiscoveryItem, contentRoot, srcReg, srcVis string) addExecResult {
    switch item.itemType {
    case catalog.Hooks:
        return addHookFromDiscovery(item, contentRoot, srcReg, srcVis)
    case catalog.MCP:
        return addMcpFromDiscovery(item, contentRoot, srcReg, srcVis)
    default:
        // File-based: use add.AddItems with single-item slice
        if item.underlying == nil {
            return addExecResult{name: item.name, status: "error",
                err: fmt.Errorf("internal: nil underlying for type %s", item.itemType)}
        }
        opts := add.AddOptions{
            Force:            item.overwrite,
            Provider:         providerSlugFromSource(item),
            SourceRegistry:   srcReg,
            SourceVisibility: srcVis,
        }
        results := add.AddItems([]add.DiscoveryItem{*item.underlying}, opts, contentRoot, nil, "syllago")
        if len(results) > 0 {
            return convertAddResult(results[0])
        }
        return addExecResult{name: item.name, status: "error", err: fmt.Errorf("no result")}
    }
}
```

### Execute step key handler

```go
func (m *addWizardModel) updateKeyExecute(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
    if m.executeDone {
        // Completion screen: Enter or Esc closes wizard
        if msg.Type == tea.KeyEnter || msg.Type == tea.KeyEsc {
            return m, func() tea.Msg { return addCloseMsg{} }
        }
        return m, nil
    }
    // During execution: Esc cancels remaining
    if msg.Type == tea.KeyEsc {
        m.executeCancelled = true
        return m, nil
    }
    return m, nil
}
```

### Handle addExecItemDoneMsg and addExecAllDoneMsg in Update

```go
case addExecItemDoneMsg:
    if msg.seq != m.seq { return m, nil }
    m.executeResults[msg.index] = msg.result
    m.executeCurrent = msg.index + 1
    if next := m.nextPending(); next >= 0 && !m.executeCancelled {
        return m, m.addItemCmd(next)
    }
    m.executeDone = true
    m.executing = false
    // Emit addExecAllDoneMsg to trigger toast at App level
    return m, func() tea.Msg { return addExecAllDoneMsg{seq: m.seq} }

// At App level (app_update.go), addExecAllDoneMsg fires the completion toast:
// case addExecAllDoneMsg:
//     count := countAdded(a.addWizard)
//     cmd := a.toast.Push(fmt.Sprintf("Added %d items to library", count), toastSuccess)
//     return a, cmd
// Note: wizard is still showing the completion screen at this point. The toast
// appears overtop. Catalog rescan happens later on addCloseMsg.
```

### viewExecute()

- Header: "Adding items..." or "Done! N items added to library."
- Per-item progress: `✓` successColor / `◐` primaryColor (spinner) / `✗` dangerColor / blank mutedStyle
- Status text: "Added" / "Updated" / "Adding..." / "Error: ..." / "Pending" / "Cancelled"
- Completion: count summary + `[Enter] Go to Library`
- Partial: "X of Y added, Z errors, W cancelled"

**Success criteria:**
- Items added sequentially (item 1 finishes before item 2 starts)
- Progress indicators update per-item
- Esc cancels remaining items (current completes)
- Partial completion shows correct counts
- Enter on completion screen sends addCloseMsg
- addCloseMsg handler rescans catalog + cleans up gitTempDir
- Taint (sourceRegistry/sourceVisibility) populated in AddOptions
- Seq guard drops stale messages
- `make build` succeeds

---

## Task 7: Tests + Golden Files

**Files:** `cli/internal/tui/add_wizard_test.go`, `cli/internal/tui/wizard_invariant_test.go`, `cli/internal/tui/app_test.go`, `cli/internal/tui/testdata/*.golden`

### add_wizard_test.go

All 37 wizard flow tests from spec v2 test table. Key patterns:

**Test helpers needed:**

```go
func testAddWizardProviders() []provider.Provider // 2 detected, 1 not
func testAddWizardRegistries() []catalog.RegistrySource // 1 registry
func testAddWizardConfig() *config.Config
func testOpenAddWizard(t *testing.T) *addWizardModel // standard 5-step
func testOpenAddWizardPreFiltered(t *testing.T, ct catalog.ContentType) *addWizardModel // 4-step
```

**Discovery test pattern:** For tests that need discovery results, mock via direct
`addDiscoveryDoneMsg` injection rather than real filesystem scanning:

```go
m, _ := m.Update(addDiscoveryDoneMsg{
    seq:   m.seq,
    items: testDiscoveryItems(),
})
```

### wizard_invariant_test.go additions

5 new tests following existing pattern in the file.

### app_test.go additions

6 integration tests. Use `testAppWithItems()` + key sequences to verify
wizard opens/closes correctly.

### Golden files

Generate at 3 sizes (60×20, 80×30, 120×40) for each step visual:
- `add-wizard-source-{size}.golden`
- `add-wizard-source-provider-expanded-{size}.golden`
- `add-wizard-type-{size}.golden`
- `add-wizard-discovery-loading-{size}.golden`
- `add-wizard-discovery-results-{size}.golden`
- `add-wizard-discovery-empty-{size}.golden`
- `add-wizard-review-{size}.golden`
- `add-wizard-execute-progress-{size}.golden`
- `add-wizard-execute-done-{size}.golden`

= 9 × 3 = 27 golden files

**Success criteria:**
- All 37 flow tests pass
- All 5 invariant tests pass
- All 6 app integration tests pass
- 27 golden files generated
- Coverage ≥80% on new code: `go test ./internal/tui/ -coverprofile=cov.out && go tool cover -func=cov.out | grep -E 'add_wizard|checkbox'`
- `make test` passes clean (no regressions)

---

## Task 8: Git URL Source (Stretch)

**Files:** `cli/internal/tui/add_wizard.go`, `cli/internal/tui/add_wizard_update.go`

### Hardened clone function

```go
func cloneGitURL(url string, timeout time.Duration) (repoDir string, err error) {
    // Create parent temp dir, clone into subdirectory to avoid git's
    // "destination path already exists" error on older versions.
    parentDir, err := os.MkdirTemp("", "syllago-add-*")
    if err != nil {
        return "", fmt.Errorf("creating temp dir: %w", err)
    }
    repoDir = filepath.Join(parentDir, "repo")

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", "clone",
        "-c", "core.hooksPath=/dev/null",      // prevent repo hooks from executing
        "--no-recurse-submodules",              // prevent submodule attacks
        "--depth", "1",                         // shallow clone (faster, smaller surface)
        url, repoDir,
    )
    cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")

    if err := cmd.Run(); err != nil {
        os.RemoveAll(parentDir)
        return "", fmt.Errorf("git clone: %w", err)
    }
    return repoDir, nil
    // Note: caller must os.RemoveAll(filepath.Dir(repoDir)) to clean up parentDir
}
```

### discoverFromGitURL

```go
func discoverFromGitURL(
    url string,
    selectedTypes []catalog.ContentType,
    contentRoot string,
) ([]addDiscoveryItem, string, error) {
    tmpDir, err := cloneGitURL(url, 60*time.Second)
    if err != nil {
        return nil, "", err
    }
    items, err := discoverFromLocalPath(tmpDir, selectedTypes, contentRoot)
    if err != nil {
        os.RemoveAll(tmpDir)
        return nil, "", err
    }
    return items, tmpDir, nil
}
```

### Cleanup points

1. `addCloseMsg` handler: `os.RemoveAll(m.gitTempDir)`
2. Back from Discovery when `source == addSourceGit`: `os.RemoveAll(m.gitTempDir); m.gitTempDir = ""`
3. Re-clone (user changes URL and re-discovers): clean old tmpDir first

### Taint probe after clone

After successful clone, before returning discovery results:
```go
m.sourceRegistry = registry.NameFromURL(url)
vis, _ := registry.ProbeVisibility(url)
m.sourceVisibility = vis
```

**Success criteria:**
- Git URL `https://github.com/example/repo` clones with hardened flags
- `ext::malicious` rejected before clone (validation in Source step)
- Clone timeout after 60s produces inline error
- tmpDir cleaned on wizard close
- tmpDir cleaned on back-navigation from Discovery
- Taint probed and stored for Execute step
- `make build` succeeds

---

## Helper function: shellIndexForStep

Used throughout to map addStep to shell breadcrumb index, accounting for
pre-filtered (4-step) vs normal (5-step) paths:

```go
func (m *addWizardModel) shellIndexForStep(s addStep) int {
    if m.preFilterType != "" {
        // 4-step: Source=0, Discovery=1, Review=2, Execute=3
        switch s {
        case addStepSource:    return 0
        case addStepDiscovery: return 1
        case addStepReview:    return 2
        case addStepExecute:   return 3
        }
    }
    // 5-step: Source=0, Type=1, Discovery=2, Review=3, Execute=4
    return int(s)
}
```

---

## Helper function: goBackFromDiscovery

```go
func (m *addWizardModel) goBackFromDiscovery() {
    // Clear discovery state
    m.discoveredItems = nil
    m.discoveryList = checkboxList{}
    m.discoveryErr = ""
    m.discovering = false

    // Clean up git temp dir if present
    if m.gitTempDir != "" {
        os.RemoveAll(m.gitTempDir)
        m.gitTempDir = ""
    }

    if m.preFilterType != "" {
        m.step = addStepSource
        m.shell.SetActive(0)
    } else {
        m.step = addStepType
        m.shell.SetActive(1)
    }
}
```
