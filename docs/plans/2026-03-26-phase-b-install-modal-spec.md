# Phase B Spec: Install Wizard

**Date:** 2026-03-26 (updated: full-screen pivot)
**Phase:** B (second phase of TUI Wizards)
**Parent design:** `docs/plans/2026-03-26-tui-wizards-design.md` (W2: Install Wizard)
**Status:** Spec — ready for implementation planning

---

## Purpose

Install a library item into a provider's configuration. Full-screen wizard
experience with step breadcrumbs, risk review with code highlighting, and
drill-in to file previews. Completes the "browse library -> install to provider"
loop.

Also establishes three shared infrastructure components reused by Phase D
(Add Wizard) and Phase E (Loadout Apply):
1. **Wizard shell** — step breadcrumbs with clickable back-navigation
2. **Risk banner** — navigable risk indicator list with drill-in
3. **Risk code highlighting** — tinted background on risky lines in file preview

**Entry point:** `[i]` hotkey on a selected item in Library, Content tabs, or
gallery drill-in views.

---

## Architecture: Full-Screen Wizard

The install wizard is a **full-screen wizard** (not an overlay modal), following
the design doc's "Wizard Models as Pointer Fields" pattern. This means:

- **Pointer field on App:** `installWizard *installWizardModel` (nil when inactive)
- **wizardMode routing:** `App.Update()` checks `wizardMode` early and delegates
  all input to the active wizard. Global keys (1/2/3, R, q, Tab) are suppressed.
- **Step breadcrumbs** at the top showing progress through the wizard
- **Full terminal width** for provider list, risk review, and code drill-in

```go
type wizardKind int

const (
    wizardNone    wizardKind = iota
    wizardInstall
    // wizardAdd, wizardShare added in later phases
)
```

---

## Component 1: Wizard Shell (wizard_shell.go)

A reusable wrapper that provides step breadcrumbs for any full-screen wizard.
Used by Install (Phase B), Add (Phase D), and Share (Phase E).

### Step Bar Rendering

```
╭──syllago─── Install ──────────────────────────────────╮
│  [1 Provider]  [2 Location]  [3 Method]  [4 Review]  │
╰───────────────────────────────────────────────────────╯
```

- Active step: bold + primary color
- Completed steps: underlined, clickable (zone-marked for mouse)
- Future steps: muted, non-clickable
- Step count is dynamic (hooks/MCP show only Provider + Review)

### Interface

```go
// wizardShell renders step breadcrumbs for any wizard.
type wizardShell struct {
    title    string       // "Install", "Add", "Share"
    steps    []string     // step labels: ["Provider", "Location", "Method", "Review"]
    active   int          // index of current step (0-based)
    width    int          // terminal width for rendering
}

func newWizardShell(title string, steps []string) wizardShell
func (s *wizardShell) SetActive(step int)
func (s *wizardShell) SetSteps(steps []string) // for dynamic step changes (hooks/MCP)
func (s wizardShell) View() string             // renders the topbar with step breadcrumbs
func (s wizardShell) HandleClick(msg tea.MouseMsg) (int, bool) // returns step index if clicked
```

The wizard shell does NOT handle key input for step navigation — the parent
wizard model handles Esc/Back and calls `SetActive()`. The shell is a pure
view component with click detection.

---

## Component 2: Install Wizard (install.go)

### Step Enum

```go
type installStep int

const (
    installStepProvider installStep = iota // Step 1: pick target provider
    installStepLocation                    // Step 2: pick global/project/custom
    installStepMethod                      // Step 3: pick symlink/copy
    installStepReview                      // Step 4: review, risk, confirm
)
```

### Struct

```go
type installWizardModel struct {
    shell    wizardShell
    step     installStep
    width    int
    height   int

    // Item context (set on open)
    item     catalog.ContentItem
    itemName string // DisplayName fallback to Name

    // Provider step
    providers         []provider.Provider // detected providers only
    providerInstalled []bool              // parallel: already installed?
    providerCursor    int                 // selected provider index

    // Location step (0=global, 1=project, 2=custom)
    locationCursor int
    customPath     string // text input value when locationCursor==2
    customCursor   int    // cursor position within customPath

    // Method step (0=symlink, 1=copy)
    methodCursor int

    // Review step
    risks       []catalog.RiskIndicator // computed on entering review
    riskCursor  int                     // selected risk item (-1 = none)
    riskDrillIn bool                    // true when viewing file preview

    // Focus
    focusIdx int // meaning varies per step (options vs buttons)

    // Double-confirm prevention
    confirmed bool

    // Computed on open
    isJSONMerge       bool   // true for hooks/MCP
    autoSkippedProvider bool  // true when single-provider was auto-skipped

    // Context
    projectRoot string
    projectRoot string // for installer.CheckStatus + Install (symlink verification)
}
```

### validateStep() Prerequisites

Called at the top of `Update()`. Panics with descriptive messages.

| Step | Prerequisite | Panic message |
|------|-------------|---------------|
| `installStepProvider` | `item.Path != ""` | `"wizard invariant: installStepProvider entered with empty item"` |
| `installStepLocation` | `providerCursor` in range AND not already-installed | `"wizard invariant: installStepLocation entered without valid provider"` |
| `installStepMethod` | `locationCursor` valid AND `!isJSONMerge` | `"wizard invariant: installStepMethod entered for JSON merge type"` |
| `installStepReview` | provider selected AND (isJSONMerge OR location selected) | `"wizard invariant: installStepReview entered without provider+location"` |

### Open / Close

```go
// openInstallWizard creates and returns a new install wizard.
// Called from App when [i] is pressed. Returns a pointer (stored on App).
func openInstallWizard(item catalog.ContentItem, providers []provider.Provider,
    projectRoot string) *installWizardModel
```

Logic:
- Computes `isJSONMerge` from provider/type
- Computes `providerInstalled` via `installer.CheckStatus` per provider
- Sets step labels based on isJSONMerge:
  - Filesystem: ["Provider", "Location", "Method", "Review"]
  - JSON merge: ["Provider", "Review"]
- Single-provider auto-skip: if one provider detected AND not installed,
  set `providerCursor = 0`, `autoSkippedProvider = true`, advance to Location
  (or Review for JSON merge)

Close: App sets `installWizard = nil` and `wizardMode = wizardNone`.

### Step Transitions

```
Provider ──[Enter]──> Location ──[Enter]──> Method ──[Enter]──> Review
  │                      │                                        │
  │ (auto-skip)          │ (JSON merge)                           │
  └──────────────────────┴──> Review                              │
                                │                                  │
                                ├──[Enter on risk item]──> Drill-in
                                │                           │
                                │                    [Esc]──┘
                                │
                                └──[Enter on Install]──> installResultMsg
```

**Back navigation (Esc):**
- Review -> Method (or Location if JSON merge, or close if auto-skipped + JSON merge)
- Review drill-in -> Review
- Method -> Location
- Location -> Provider (or close if auto-skipped)
- Provider -> close wizard

### Messages

```go
// installResultMsg is emitted when the user confirms the install.
type installResultMsg struct {
    item        catalog.ContentItem
    provider    provider.Provider
    location    string                 // "global", "project", or custom path
    method      installer.InstallMethod // MethodSymlink or MethodCopy
    isJSONMerge bool
    projectRoot string                 // passed to installer.Install for symlink verification
}

// installDoneMsg is sent when the async install operation completes.
type installDoneMsg struct {
    itemName     string
    providerName string
    targetPath   string
    err          error
}
```

### View Rendering Per Step

**Step 1: Provider**
```
╭──syllago─── Install ──────────────────────────────────╮
│  [1 Provider]  [2 Location]  [3 Method]  [4 Review]  │
╰───────────────────────────────────────────────────────╯

  Install "my-rule" to which provider?

  > Claude Code         (detected)
    Cursor              (already installed)
    Gemini CLI          (detected)

                                        [Cancel]  [Next]
```

- Only detected providers shown (non-detected omitted)
- Already-installed: muted + "(already installed)", skipped by navigation
- Selected: bold + accent color + `>` prefix

**Step 2: Location**
```
  Install location for Claude Code:

  > Global   (~/.claude/rules/)
    Project  (./.claude/rules/)
    Custom   [________________________]

                                        [Back]  [Next]
```

- Resolved paths shown per provider + content type
- Custom text input with background tinting and block cursor

**Step 3: Method**
```
  Install method:

  > Symlink   (recommended — stays in sync with library)
    Copy      (standalone copy, won't auto-update)

                                        [Back]  [Next]
```

- Symlink disabled: muted + "(not supported)" when `SymlinkSupport[type]` is false
- Default to Copy when Symlink disabled

**Step 4: Review**
```
  Installing "my-rule" to Claude Code

  Location: ~/.claude/rules/my-rule
  Method:   Symlink
  Source:   ~/.syllago/content/rules/my-rule

  ╭─ Risk Indicators ──────────────────────────────╮
  │  ! Bash access — content references Bash tool  │
  ╰────────────────────────────────────────────────╯

                              [Cancel]  [Back]  [Install]
```

For hooks/MCP:
```
  Installing "my-hook" to Claude Code

  Will merge into: ~/.claude/settings.json

  ╭─ Risk Indicators ────────────────────────────────────╮
  │  !! Runs commands — bash -c 'curl example.com | sh'  │
  │  !  Network access — Hook makes HTTP requests        │
  ╰──────────────────────────────────────────────────────╯

  Enter to inspect code   Esc back

                              [Cancel]  [Back]  [Install]
```

---

## Component 3: Risk Banner (risk_banner.go)

A reusable navigable list of risk indicators. Used by Install (Phase B),
Add (Phase D), and Loadout Apply (Phase E).

### Struct

```go
type riskBanner struct {
    risks  []catalog.RiskIndicator
    cursor int  // -1 when no item focused, 0+ for selected
    width  int
}

func newRiskBanner(risks []catalog.RiskIndicator, width int) riskBanner
func (b riskBanner) Update(msg tea.KeyMsg) (riskBanner, tea.Cmd)
func (b riskBanner) View() string
func (b riskBanner) IsEmpty() bool
```

### Rendering

- Bordered box with title "Risk Indicators"
- Border color: RED (`dangerColor`) when any risk has Level=HIGH,
  ORANGE (`warningColor`) when all MEDIUM
- Each item: severity icon (`!!` HIGH, `!` MEDIUM) + label + truncated description
- Selected item: bold + accent color
- Empty: returns empty string (zero height)

### Navigation

- Up/Down: move cursor between risk items
- Enter: emits `riskDrillInMsg{risk}` — parent wizard handles drill-in
- Esc: handled by parent (not the banner)

---

## Component 4: Risk Code Highlighting

Not a separate model — an enhancement to the existing file preview component.
When displaying a file in the risk drill-in view, specific lines are highlighted
with a tinted background to show where risky code lives.

### Backend Changes (catalog/risk.go)

Add `Level` and `Lines` fields to `RiskIndicator`:

```go
type RiskLevel int

const (
    RiskMedium RiskLevel = iota
    RiskHigh
)

type RiskLine struct {
    File string // relative path within item
    Line int    // 1-based line number
}

type RiskIndicator struct {
    Label       string
    Description string
    Level       RiskLevel
    Lines       []RiskLine // lines in source files where this risk was detected
}
```

Update `hookRisks()`, `mcpRisks()`, `skillAgentRisks()` to populate Level and
Lines. For example, when scanning hooks JSON and finding a `"command"` field,
record the file and line number where it occurs.

### TUI Rendering

The file preview (used in library detail drill-in) gains an optional
`highlightLines map[int]bool` parameter. When set, lines in the map are
rendered with:
- Tinted background (warm/danger color from Flexoki palette)
- Gutter marker (`▌` in danger color)

Normal lines render with no background and a plain `│` gutter.

---

## App Wiring

### keys.go

```go
keyInstall = "i"
```

### App Struct

```go
wizardMode    wizardKind
installWizard *installWizardModel
```

### App.Update() — Wizard Mode Early-Return

**Ordering:** `WindowSizeMsg` is handled first (unconditionally) — it propagates
dimensions to all components INCLUDING the active wizard. The wizard-mode
early-return comes AFTER WindowSizeMsg but BEFORE all modal/key routing:

```go
case tea.WindowSizeMsg:
    // ... existing propagation to topbar, library, etc. ...
    if a.installWizard != nil {
        a.installWizard.width = msg.Width
        a.installWizard.height = msg.Height
    }
    return a, nil
```

Then, for KeyMsg and MouseMsg:

```go
if a.wizardMode != wizardNone {
    if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
        return a, tea.Quit
    }
    return a.routeToWizard(msg)
}
```

`routeToWizard()` dispatches to the active wizard's Update method. If the
wizard returns a close message, App sets `wizardMode = wizardNone` and
`installWizard = nil`.

### Key Routing

Add `[i]` in the global keys section:

```go
case msg.String() == keyInstall:
    return a.handleInstall()
```

### handleInstall() (actions.go)

1. Get selected item via `a.selectedItem()`
2. If nil, no-op
3. If not a library item: toast "Only library items can be installed"
4. Filter providers to detected-only
5. If no detected providers: toast "No providers detected"
6. Create wizard: `a.installWizard = openInstallWizard(*item, detected, a.projectRoot)`
7. Set `a.wizardMode = wizardInstall`

### handleInstallResult() (actions.go)

Returns a `tea.Cmd` that:
1. Resolves baseDir from location
2. Calls `installer.Install()`
3. Returns `installDoneMsg`

### handleInstallDone() (actions.go)

1. Close wizard: `a.installWizard = nil`, `a.wizardMode = wizardNone`
2. If error: toast error
3. If success: toast "Installed {name} to {provider}" + rescan catalog

### App.View()

When `wizardMode == wizardInstall`, render the install wizard's View instead
of the normal topbar + content layout:

```go
if a.wizardMode == wizardInstall && a.installWizard != nil {
    return a.installWizard.View()
}
```

### Metadata Panel Button

Add `[i] Install` to metapanel.go. Shown when item is a library item AND at
least one detected provider where item is NOT already installed.
Button order: `[i] Install`, `[x] Uninstall`, `[d] Remove`, `[e] Edit`.
Zone ID: `"meta-install"`.

### Help

Add `[i] Install` to help overlay Actions section and helpbar hints.

---

## Hooks/MCP JSON Merge Path

When `isJSONMerge` is true:

1. **Step breadcrumbs:** Only show `[1 Provider]  [2 Review]`
2. **Location step:** SKIPPED
3. **Method step:** SKIPPED
4. **Review:** Shows "Will merge into ~/.claude/settings.json" instead of
   location/method. Risk banner always shown (hooks always have risks).

The `installer.Install` function handles JSON merge internally — the wizard
just passes the right arguments and skips the irrelevant UI steps.

---

## Edge Cases

### Single Provider Auto-Skip
One detected provider AND not already installed: skip provider picker, go to
Location (or Review for JSON merge). `autoSkippedProvider = true`. Back from
Location closes the wizard.

### All Providers Already Installed
Provider picker shows all entries disabled. Next button disabled. Only
Esc/Cancel works.

### Custom Path Validation
- Empty: Enter is no-op
- Non-existent: proceed anyway (installer creates it)

### Symlink on Windows Mounts
`installer.Install` handles this transparently (falls back to copy).

### Double-Confirm Prevention
`confirmed` flag prevents repeat Enter on Install button.

### Stale installDoneMsg
If wizard closed while install in-flight, handler still processes the message
(toast + rescan) since the install already happened on disk.

---

## Test Requirements

### Wizard Shell Tests (wizard_shell_test.go)
- Step bar renders correct count for 2, 4 steps
- Active step highlighted (bold + primary color)
- Completed steps clickable (underlined)
- Future steps muted and non-clickable
- Click on completed step returns step index
- SetSteps dynamically changes step labels
- Step bar truncation at narrow widths (60 chars)

### Install Wizard Tests (install_test.go)
- Provider picker: detected providers with status badges
- Provider picker: already-installed disabled, navigation skips
- Provider picker: single-provider auto-skip
- Provider picker: all installed — only Esc works
- Location: global/project/custom with resolved paths
- Location: custom text input (typing, backspace, space)
- Location: empty custom path blocks advance
- Method: symlink/copy, symlink disabled when unsupported
- Hooks/MCP: location + method skipped, 2-step breadcrumb
- Review: correct destination path, risk indicators shown
- Review: risk drill-in — Enter on risk shows file preview
- Review: risky lines highlighted in drill-in
- Back navigation preserves selections at each level
- Esc exits from provider, backs out from other steps
- Enter on review confirms with correct installResultMsg fields
- Double-confirm prevention
- Stale async result handling

### Risk Banner Tests (risk_banner_test.go)
- Renders with risk indicators
- Border RED for HIGH, ORANGE for MEDIUM only
- Up/Down navigates between items
- Enter emits drill-in message
- Single item: no navigation needed
- Empty list: zero height
- Command preview truncated

### Risk Code Highlighting Tests (risk_highlight_test.go)
- File preview with highlight lines shows tinted background
- Gutter marker on highlighted lines
- No highlights when empty
- Works for JSON, YAML, Markdown files

### App Integration Tests
- `[i]` opens install wizard full-screen
- `[i]` on non-library item -> toast
- `[i]` with no providers -> toast
- Full flow: provider -> location -> method -> confirm -> toast + rescan
- Hook item: 2-step flow with risk banner
- Global keys suppressed during wizard
- Esc from provider exits wizard

### Wizard Invariant Tests (wizard_invariant_test.go)
- Forward-path: all steps without panics (filesystem + JSON merge)
- Esc-path: back from each step without panics
- Auto-skip: single provider transitions without panics

### Golden Files
- Install wizard at each step (80x30, 60x20)
- Provider picker: mixed status
- Risk banner: with indicators, empty
- Risk drill-in: highlighted risky lines
- Hooks/MCP: 2-step breadcrumb, JSON merge review

---

## Dependencies

### Packages Used
- `cli/internal/installer` — `Install()`, `CheckStatus()`, `IsJSONMerge()`, `InstallMethod`
- `cli/internal/provider` — `Provider`, `JSONMergeSentinel`, `SymlinkSupport`
- `cli/internal/catalog` — `ContentItem`, `ContentType`, `RiskIndicators()`

### Reusable Patterns From Phase A
- `buttonDef` + `renderButtons()` — currently defined on `removeModal` in remove.go.
  Must be extracted to a shared location (e.g., `buttons.go`) or duplicated with
  `inst-` zone prefixes. Extraction preferred since confirmModal also uses buttons.
- `overlayModal()` from app.go — confirm modals on wizard screens
- `activeButtonStyle` from styles.go — metadata panel buttons
- Toast push pattern from actions.go
- `rescanCatalog()` from app.go

### New Shared Infrastructure (built in Phase B, reused by D and E)
- `wizardShell` — step breadcrumbs + click detection
- `riskBanner` — navigable risk indicator list
- Risk code highlighting — tinted line rendering in file preview
- `wizardKind` enum + `routeToWizard()` dispatch
