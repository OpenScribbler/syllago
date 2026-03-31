# Phase B Implementation Plan: Install Wizard

**Date:** 2026-03-26 (updated: full-screen pivot)
**Spec:** `docs/plans/2026-03-26-phase-b-install-modal-spec.md`
**Design:** `docs/plans/2026-03-26-tui-wizards-design.md` (W2)

---

## Bead Chain

```
Impl B1 (wizard shell + wizardKind routing)
  └── Validate B1
      └── Impl B2 (risk scanner backend: Level + Lines fields)
          └── Validate B2
              └── Impl B3 (install wizard core: struct, open/close, validateStep)
                  └── Validate B3
                      └── Impl B4 (provider picker step)
                          └── Validate B4
                              └── Impl B5 (location + custom path step)
                                  └── Validate B5
                                      └── Impl B6 (method step + JSON merge skip)
                                          └── Validate B6
                                              └── Impl B7 (risk banner component)
                                                  └── Validate B7
                                                      └── Impl B8 (review step + risk drill-in + code highlighting)
                                                          └── Validate B8
                                                              └── Impl B9 (app wiring: keys, routing, handlers)
                                                                  └── Validate B9
                                                                      └── Impl B10 (metadata button + help + golden files)
                                                                          └── Validate B10
                                                                              └── Phase B Validation
```

**Why this order:**
- B1 (wizard shell) is shared infrastructure — must exist before the install wizard
- B2 (risk scanner) is backend-only — no TUI dependency, provides data for B7/B8
- B3-B6 build the install wizard steps incrementally
- B7 (risk banner) is a reusable component used by B8 (review step)
- B8 (review + drill-in + highlighting) ties everything together
- B9 (app wiring) connects the wizard to the main app
- B10 (polish) adds metadata button, help entries, golden files

---

## Impl B1: Wizard Shell + wizardKind Routing

**New files:** `cli/internal/tui/wizard_shell.go`, `cli/internal/tui/buttons.go`
**Modified files:** `cli/internal/tui/app.go`, `cli/internal/tui/remove.go`

**What to build:**

0. Extract `buttonDef` type and `renderButtons()` from `removeModal` in remove.go
   into `buttons.go` as package-level shared utilities. Update remove.go to use
   the shared version. This is needed by the install wizard and keeps button
   rendering DRY.
1. `wizardShell` struct: `title`, `steps []string`, `active int`, `width int`
2. `newWizardShell(title, steps)` constructor
3. `SetActive(step)`, `SetSteps(steps)`, `SetWidth(w)` methods
4. `View()` — renders the topbar frame with step breadcrumbs:
   - Uses the existing topbar border pattern (`╭──syllago─── Title ──╮`)
   - Active step: bold + primary color
   - Completed steps (index < active): underlined + clickable via zone.Mark
   - Future steps (index > active): muted
   - Zone IDs: `"wiz-step-N"` for clickable completed steps
5. `HandleClick(msg tea.MouseMsg) (int, bool)` — returns step index if a
   completed step was clicked
6. `wizardKind` enum in app.go: `wizardNone`, `wizardInstall`
7. `routeToWizard(msg) (tea.Model, tea.Cmd)` skeleton in app.go — dispatches
   to active wizard's Update. Returns early from `App.Update()` when
   `wizardMode != wizardNone` (suppresses global keys).

**Tests (wizard_shell_test.go):**
- `TestWizardShell_Render4Steps`: 4-step bar renders all labels correctly
- `TestWizardShell_Render2Steps`: 2-step bar (hooks/MCP) renders correctly
- `TestWizardShell_ActiveHighlighted`: Active step has bold + primary styling
- `TestWizardShell_CompletedClickable`: Completed steps have underline, zone marks
- `TestWizardShell_FutureMuted`: Future steps are muted, no zone marks
- `TestWizardShell_ClickCompleted`: Click on completed step returns correct index
- `TestWizardShell_ClickFutureNoop`: Click on future step returns false
- `TestWizardShell_SetStepsDynamic`: Changing steps re-renders correctly
- `TestWizardShell_NarrowWidth`: At 60 chars, step labels truncate gracefully
- `TestApp_WizardModeSuppress`: When wizardMode != none, global keys (1/2/3, R, q) don't fire

**Success criteria:**
- Wizard shell renders step bar correctly at multiple widths
- Click detection works for completed steps only
- App suppresses global keys when wizard mode is active
- All tests pass, `make build` succeeds

---

## Validate B1

**Checks:**
- Step bar renders correctly with 2, 4, and 7 steps (verify actual text content, not just "no error")
- Zone marks only exist on completed steps (grep the View output for zone IDs)
- `routeToWizard` is called when wizardMode != none (verify via test that sends a key and checks it doesn't trigger group switch)
- Ctrl+C still works during wizard mode (not suppressed)
- `make test` passes (full suite)

---

## Impl B2: Risk Scanner Backend — Level + Lines Fields

**Modified files:** `cli/internal/catalog/risk.go`, `cli/internal/catalog/risk_test.go`

**What to build:**

1. `RiskLevel` enum: `RiskMedium`, `RiskHigh`
2. `RiskLine` struct: `File string`, `Line int`
3. Add `Level RiskLevel` and `Lines []RiskLine` fields to `RiskIndicator`
4. Update `hookRisks()`:
   - "Runs commands" → `RiskHigh`, with Lines pointing to the JSON line(s) containing `"command"`
   - "Network access" → `RiskMedium`, with Lines pointing to `"url"` field lines
5. Update `mcpRisks()`:
   - "Network access" → `RiskMedium`
   - "Environment variables" → `RiskMedium`, Lines pointing to `"env"` field lines
6. Update `skillAgentRisks()`:
   - "Bash access" → `RiskHigh`, Lines pointing to lines containing "Bash"
7. Line number detection: use `strings.Split` on file content and scan line by line
   to find the matching patterns. This is simple substring matching on already-read
   content — no new file I/O.

**Tests (risk_test.go — update existing):**
- `TestRiskIndicators_Hook_Command`: Verify Level=RiskHigh, Lines contains correct file+line
- `TestRiskIndicators_Hook_URL`: Verify Level=RiskMedium, Lines correct
- `TestRiskIndicators_MCP_WithEnv`: Verify both risk levels and Lines populated
- `TestRiskIndicators_Skill_WithBash`: Verify Level=RiskHigh, Lines point to correct line
- `TestRiskIndicators_LinesAccurate`: Create a multi-line hook JSON, verify Lines match actual line numbers
- `TestRiskIndicators_NoRisks`: Rules item returns empty risks (no Level/Lines to check)

**Success criteria:**
- All existing risk_test.go tests still pass (backwards compatible — old tests just don't check new fields)
- New tests verify Level and Lines are populated correctly
- Line numbers are 1-based and match actual file content
- `make build` succeeds

---

## Validate B2

**Checks:**
- Level assignments match spec: commands = HIGH, network/env = MEDIUM, bash = HIGH
- Lines are verified against actual file content (test creates a fixture file, scans it, checks line number matches the line containing the pattern)
- No performance regression: risk scanning is still O(files * lines), not loading files multiple times
- Existing tests in other packages that use RiskIndicator still compile (no breaking changes)
- `make test` passes

---

## Impl B3: Install Wizard Core — Struct, Open/Close, validateStep

**New files:** `cli/internal/tui/install.go`, `cli/internal/tui/wizard_invariant_test.go` (new file)

**What to build:**

1. `installStep` enum with iota
2. `installWizardModel` struct with all fields from spec
3. `openInstallWizard()` function — creates wizard, computes isJSONMerge,
   providerInstalled, step labels, handles single-provider auto-skip.
   Uses `a.projectRoot` for `installer.CheckStatus` (consistent with
   existing `handleRemove`/`handleUninstall` in actions.go).
4. `Close()` via `App` setting pointer to nil
5. `validateStep()` — panics per spec table
6. `Update()` skeleton — validateStep, switch on msg type (KeyMsg, MouseMsg,
   WindowSizeMsg), delegate to per-step handlers (stubs for now)
7. `View()` skeleton — wizard shell + per-step view dispatch (stubs)
8. `Init()` — returns nil (no initial command)
9. Message types: `installResultMsg`, `installDoneMsg`, `installCloseMsg`

**Tests (install_test.go):**
- `TestInstallWizard_Open`: Opens with correct providers, isJSONMerge, step
- `TestInstallWizard_OpenSingleProvider`: Auto-skips to Location
- `TestInstallWizard_OpenSingleProviderInstalled`: No auto-skip
- `TestInstallWizard_OpenJSONMerge`: Sets isJSONMerge, 2-step breadcrumb
- `TestInstallWizard_Close`: After close, pointer is nil

**Tests (wizard_invariant_test.go — NEW FILE):**
- `TestInstallWizard_ValidateStep_Forward`: Walk through all steps without panic
- `TestInstallWizard_ValidateStep_Esc`: Back from each step without panic
- `TestInstallWizard_ValidateStep_AutoSkip`: Single provider transitions without panic
- `TestInstallWizard_ValidateStep_JSONMerge`: JSON merge path (2 steps) without panic

**Success criteria:**
- All fields from spec present and correctly initialized
- Auto-skip logic works for single provider
- validateStep panics on invalid state, passes on valid
- `make build` succeeds, all tests pass

---

## Validate B3

**Checks:**
- Struct fields match spec exactly (no extras, no missing)
- `openInstallWizard` computes providerInstalled by calling actual CheckStatus (with fixture providers)
- isJSONMerge correctly detected for hooks AND MCP types
- validateStep tested: deliberately set invalid state and verify panic (use recover in test)
- `make test` passes

---

## Impl B4: Provider Picker Step

**Modified file:** `cli/internal/tui/install.go`

**What to build:**

1. `updateKeyProvider()` — Up/Down/j/k navigation skipping installed, Enter advances,
   Tab/Shift-Tab cycles, Esc closes wizard
2. `updateMouseProvider()` — click on provider rows, cancel/next buttons
3. `viewProvider()` — renders provider list with status badges
4. `nextSelectableProvider(current, direction)` helper
5. Provider step buttons: Cancel, Next
6. Install-specific button rendering (same pattern as remove.go)

**Tests (install_test.go):**
- `TestInstallWizard_ProviderNav`: Up/Down skips installed, wraps at bounds
- `TestInstallWizard_ProviderEnter`: Enter on valid provider advances to Location
- `TestInstallWizard_ProviderEnterInstalled`: Enter on installed is no-op
- `TestInstallWizard_ProviderEsc`: Esc emits close message
- `TestInstallWizard_ProviderAllInstalled`: No selectable provider, only Esc works
- `TestInstallWizard_ProviderMouse`: Click on row selects it

**Success criteria:**
- Navigation skips already-installed in both directions
- Enter only advances on valid provider
- Esc closes, Cancel closes
- All tests pass, `make build` succeeds

---

## Validate B4

**Checks:**
- Provider rendering includes "(detected)" and "(already installed)" labels
- Edge: first provider installed, cursor starts at second
- Edge: all installed, Next button disabled or no-op
- Tests verify step transitions (check step field), not just "no error"
- `make test` passes

---

## Impl B5: Location + Custom Path Step

**Modified file:** `cli/internal/tui/install.go`

**What to build:**

1. `updateKeyLocation()` — navigation, text input for custom path, Enter/Esc
2. `updateMouseLocation()` — click handling
3. `viewLocation()` — renders three options with resolved paths
4. `resolvedInstallPath()` helper — computes display path from provider + type
5. Custom path text input (reuse editModal pattern: background tint, block cursor)

**Tests (install_test.go):**
- `TestInstallWizard_LocationNav`: Cycles Global/Project/Custom
- `TestInstallWizard_LocationGlobal`: Enter on Global advances to Method
- `TestInstallWizard_LocationProject`: Enter on Project advances to Method
- `TestInstallWizard_LocationCustomType`: Typing updates customPath
- `TestInstallWizard_LocationCustomBackspace`: Backspace works
- `TestInstallWizard_LocationCustomEmpty`: Enter on empty custom is no-op
- `TestInstallWizard_LocationCustomAdvance`: Enter on custom with path advances
- `TestInstallWizard_LocationEscBack`: Esc goes to Provider (or closes if auto-skipped)
- `TestInstallWizard_LocationResolvedPaths`: Paths resolve correctly per provider

**Success criteria:**
- Three options render with correct resolved paths
- Custom text input works
- Path resolution correct for multiple providers
- Back navigation correct
- All tests pass, `make build` succeeds

---

## Validate B5

**Checks:**
- Resolved paths verified for 2+ providers (Claude Code, Cursor)
- Custom path: cursor movement (left/right/home/end) works
- Back from Location preserves location selection when returning
- `make test` passes

---

## Impl B6: Method Step + JSON Merge Skip

**Modified file:** `cli/internal/tui/install.go`

**What to build:**

1. `updateKeyMethod()` — Up/Down for Symlink/Copy, Enter advances, Esc back
2. `viewMethod()` — renders options with disabled state
3. `symlinkDisabled()` and `defaultMethodCursor()` helpers
4. JSON merge skip logic in all step transitions:
   - Location Enter: if isJSONMerge, skip to Review
   - Review Esc: if isJSONMerge, back to Provider (not Method)
   - Provider Enter: if isJSONMerge + single-provider, go to Review

**Tests (install_test.go):**
- `TestInstallWizard_MethodNav`: Cycles Symlink/Copy
- `TestInstallWizard_MethodSymlinkDisabled`: Skipped, defaults to Copy
- `TestInstallWizard_MethodEnter`: Advances to Review
- `TestInstallWizard_MethodEsc`: Back to Location
- `TestInstallWizard_JSONMergeSkip`: Hook item: Provider -> Review (skip 2 steps)
- `TestInstallWizard_JSONMergeBack`: Review Esc goes to Provider
- `TestInstallWizard_JSONMergeSingleAuto`: Single provider + hook = direct to Review

**Success criteria:**
- Symlink disabled detection works correctly
- JSON merge skip verified for hooks AND MCP
- Back navigation goes to correct step based on isJSONMerge
- All tests pass, `make build` succeeds

---

## Validate B6

**Checks:**
- JSON merge tested for both hooks AND MCP (not just one)
- Symlink disabled uses `SymlinkSupport` map correctly (nil = supported)
- Full path tested: hook with 2 providers → Provider → Review (2 steps)
- Full path tested: hook with 1 provider → auto-skip → Review (1 step visible)
- `make test` passes

---

## Impl B7: Risk Banner Component

**New file:** `cli/internal/tui/risk_banner.go`

**What to build:**

1. `riskBanner` struct: risks, cursor, width
2. `newRiskBanner()` constructor
3. `Update(msg tea.KeyMsg)` — Up/Down navigation, Enter emits drill-in message
4. `View()` — bordered list with severity icons, selected highlight, border color
5. `IsEmpty()` helper
6. `riskDrillInMsg` message type (carries selected RiskIndicator)
7. Severity rendering: `!!` (RED) for HIGH, `!` (ORANGE) for MEDIUM
8. Border color logic: RED if any HIGH, ORANGE if all MEDIUM

**Tests (risk_banner_test.go):**
- `TestRiskBanner_RenderHigh`: Shows !! icon, RED border
- `TestRiskBanner_RenderMediumOnly`: Shows ! icon, ORANGE border
- `TestRiskBanner_RenderMixed`: RED border when any HIGH present
- `TestRiskBanner_Navigation`: Up/Down moves cursor, wraps
- `TestRiskBanner_Enter`: Emits riskDrillInMsg with correct risk
- `TestRiskBanner_SingleItem`: No navigation needed, Enter still works
- `TestRiskBanner_Empty`: IsEmpty() returns true, View() returns ""
- `TestRiskBanner_Truncate`: Long command description truncated

**Success criteria:**
- Border color logic correct for all severity combinations
- Navigation works with 1, 2, 5+ items
- Empty state returns zero-height output
- All tests pass, `make build` succeeds

---

## Validate B7

**Checks:**
- Border color tested: HIGH+MEDIUM=RED, MEDIUM-only=ORANGE, HIGH-only=RED
- Navigation wraps correctly at bounds
- Truncation tested with a 200+ char description
- Tests verify rendered output contains severity icons and labels (not just "no error")
- `make test` passes

---

## Impl B8: Review Step + Risk Drill-in + Code Highlighting

**Modified files:** `cli/internal/tui/install.go`, `cli/internal/tui/preview.go` (or equivalent)

**What to build:**

1. `updateKeyReview()` — Left/Right between Cancel/Back/Install, Enter confirm,
   Esc back, Up/Down delegates to risk banner, Enter on risk drills in
2. `viewReview()` — summary (provider, location, method, source) + risk banner
3. `viewReviewJSONMerge()` — merge target + "commands will be executable" + risk banner
4. Risk drill-in sub-view: when `riskDrillIn == true`, show file preview with
   highlighted lines instead of the review summary
5. File preview enhancement: add `highlightLines map[int]bool` support to the
   preview rendering. Highlighted lines get tinted background + gutter marker.
6. `confirmed` flag: set true on Install button Enter, blocks repeat
7. `result()` helper: builds installResultMsg from current state

**Tests (install_test.go):**
- `TestInstallWizard_ReviewRender`: Shows provider, location, method
- `TestInstallWizard_ReviewJSONMerge`: Shows merge path + warning
- `TestInstallWizard_ReviewRiskBanner`: Risk indicators visible in review
- `TestInstallWizard_ReviewDrillIn`: Enter on risk shows file preview
- `TestInstallWizard_ReviewDrillInHighlight`: Risky lines have tinted background
- `TestInstallWizard_ReviewDrillInEsc`: Esc from drill-in returns to review
- `TestInstallWizard_ReviewConfirm`: Enter on Install emits correct message
- `TestInstallWizard_ReviewDoubleConfirm`: Second Enter is no-op
- `TestInstallWizard_ReviewCancel`: Cancel closes wizard
- `TestInstallWizard_ReviewEscBack`: Esc goes to Method (or Provider for JSON merge)

**Risk highlight tests (risk_highlight_test.go):**
- `TestPreview_HighlightLines`: Lines in map rendered with tinted background
- `TestPreview_HighlightGutter`: Highlighted lines have `▌` marker
- `TestPreview_NoHighlights`: Without highlight map, normal rendering
- `TestPreview_HighlightJSON`: JSON file with highlighted command line
- `TestPreview_HighlightMarkdown`: Markdown file with highlighted bash reference
- `TestPreview_HighlightYAML`: YAML file with highlighted risky line

**Success criteria:**
- Review renders correct summary for both filesystem and JSON merge
- Risk banner integrated into review, navigation works
- Drill-in shows file with highlighted risky lines
- Confirm emits correct message, double-confirm blocked
- All tests pass, `make build` succeeds

---

## Validate B8

**Checks:**
- installResultMsg carries all correct fields (provider, location, method, isJSONMerge, projectRoot)
- Risk drill-in: verify highlighted line content matches actual risky code (not just "line is highlighted")
- Drill-in → Esc → review: risk banner cursor preserved
- Double-confirm: send Enter twice, verify only one installResultMsg
- JSON merge review shows correct settings.json path for the selected provider
- `make test` passes

---

## Impl B9: App Wiring

**Modified files:** `cli/internal/tui/app.go`, `cli/internal/tui/actions.go`, `cli/internal/tui/keys.go`,
`cli/internal/tui/library.go`

**What to build:**

1. `keys.go`: Add `keyInstall = "i"`
2. `app.go`: Add `wizardMode wizardKind` and `installWizard *installWizardModel` fields
3. `app.go` NewApp: Initialize `wizardMode: wizardNone`
4. `app.go` Update: Add wizard mode early-return at the very top (before modal checks)
5. `app.go` Update global keys: Add `keyInstall` handler
6. `app.go` Update messages: Add `installResultMsg`, `installDoneMsg`, `installCloseMsg` cases
7. `app.go` View: Render wizard view when wizardMode active
8. `app.go` WindowSizeMsg: Propagate to installWizard if active.
      **IMPORTANT:** WindowSizeMsg must be handled BEFORE the wizard-mode
      early-return (it's unconditional — applies to all components including
      the active wizard). The wizard-mode check applies only to KeyMsg/MouseMsg.
9. `actions.go`: `handleInstall()`, `handleInstallResult()`, `handleInstallDone()`
10. `actions.go`: `doInstallCmd()` tea.Cmd
11. `library.go`: Add `libraryInstallMsg` type, wire `[i]` from library key routing
12. `routeToWizard()` implementation

**Tests (app_test.go or install_integration_test.go):**
- `TestApp_InstallKeyOpens`: Press i with library item -> wizard opens
- `TestApp_InstallKeyNonLibrary`: Press i on non-library -> toast
- `TestApp_InstallKeyNoProviders`: Press i with no providers -> toast
- `TestApp_WizardModeCapture`: Modal keys don't leak through during wizard
- `TestApp_InstallFullFlow`: Full flow -> installDoneMsg -> toast + rescan
- `TestApp_InstallDoneError`: Error -> error toast
- `TestApp_InstallDoneSuccess`: Success -> toast with item+provider names + rescan
- `TestApp_InstallEscExits`: Esc from provider exits wizard, returns to normal view
- `TestApp_InstallDoneMsgAfterClose`: installDoneMsg processed (toast + rescan) even if wizard already closed

**Success criteria:**
- [i] opens wizard for library items
- Wizard mode suppresses all global keys
- Full flow end-to-end works
- Error/success toasts correct
- Catalog rescans after success
- `make build` succeeds, `make test` passes

---

## Validate B9

**Checks:**
- Key capture: 1/2/3, R, q, Tab all suppressed during wizard (tested explicitly)
- Ctrl+C still quits during wizard mode
- Full flow: uses installer.Install with temp dir fixtures (not mocks)
- Toast text contains actual item name and provider name
- After install success, catalog status for that item+provider changes
- `make test` passes (full suite — check no regressions)

---

## Impl B10: Metadata Button + Help + Golden Files

**Modified files:** `cli/internal/tui/metapanel.go`, `cli/internal/tui/help.go`,
`cli/internal/tui/helpbar.go`, test files, `testdata/` golden files

**What to build:**

1. `metapanel.go`: Add `[i] Install` button with visibility logic (library + not fully installed)
   Button order: `[i] Install`, `[x] Uninstall`, `[d] Remove`, `[e] Edit`
   Zone ID: `"meta-install"`, uses `activeButtonStyle`
2. `metapanel.go`: Add `canInstall` to `metaPanelData`
3. `help.go`: Add `[i] Install` to Actions section
4. `helpbar.go`: Add `i install` to context-sensitive hints
5. Wire `"meta-install"` mouse click -> `libraryInstallMsg`
6. Generate golden files for all wizard steps and risk states
7. Update existing golden files affected by new metadata button

**Tests:**
- `TestMetaPanel_InstallButton`: Button appears for installable library items
- `TestMetaPanel_InstallButtonHidden`: Hidden for non-library or fully-installed
- `TestHelp_InstallEntry`: [i] Install in help overlay
- Golden file tests for all install wizard states

**Success criteria:**
- [i] Install button appears at correct position
- Visibility logic correct
- Help updated
- All golden files pass
- `make build` succeeds, `make test` passes

---

## Validate B10

**Checks:**
- Button order verified: Install, Uninstall, Remove, Edit
- Visibility: non-library item → hidden; fully installed → hidden; partially installed → shown
- Golden files reviewed: no visual regressions
- Help overlay has Install in correct section
- `make test` passes (full suite)

---

## Phase B Validation (Final)

**Blocked by:** Validate B10

1. `make test` — all tests pass
2. `make build` — binary builds clean
3. Smoke test in real TUI:
   - Browse library, select a rule, press `[i]`
   - Walk through all 4 steps with step breadcrumbs
   - Verify risk banner appears on review for hooks
   - Drill into risk item, see highlighted code, Esc back
   - Confirm install, verify toast
   - Press `R` to refresh, verify installed status
   - Select a hook, press `[i]` — 2-step flow
   - Press `?` — verify Install in help
4. Wizard invariant tests pass
5. No regressions in existing tests
6. Golden files reviewed and committed
