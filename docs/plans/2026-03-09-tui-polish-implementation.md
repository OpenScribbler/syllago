# TUI Polish & Bug Fixes - Implementation Plan

**Design document:** `docs/plans/2026-03-09-tui-polish-design.md`
**Feature:** tui-polish
**Date:** 2026-03-09

---

## Phase 1: Content Type Removal (Prompts & Apps)

Removing dead content types first simplifies every subsequent phase. Prompts and Apps have no provider mapping and add significant special-case code throughout.

### Task 1.1: Remove Prompts and Apps from catalog/types.go

**Files to modify:**
- `cli/internal/catalog/types.go`

**Changes:**
1. Remove `Prompts ContentType = "prompts"` and `Apps ContentType = "apps"` constants
2. Remove `Prompts` and `Apps` from `AllContentTypes()` return slice -- new slice: `{Skills, Agents, MCP, Rules, Hooks, Commands, Loadouts}`
3. Remove `Prompts` and `Apps` cases from `IsUniversal()` -- keep `Skills, Agents, MCP`
4. Remove `Prompts` and `Apps` cases from `Label()`
5. Remove `Body` field from `ContentItem` struct -- it is only used by Prompts and Apps. The `ReadmeBody` field remains.
6. Remove `SupportedProviders` field from `ContentItem` struct -- only used by Apps

**Dependencies:** None
**Verification:** `go build ./cli/...` compiles (will have downstream errors, but types.go itself is clean)

### Task 1.2: Remove Prompts/Apps from scanner.go

**Files to modify:**
- `cli/internal/catalog/scanner.go`

**Changes:**
1. Remove the `case Prompts:` block that parses PROMPT.md for body/description
2. Remove the `case Apps:` block that parses README.md for body/SupportedProviders
3. Remove any remaining references to `Body` or `SupportedProviders` assignments

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/catalog/...`

### Task 1.3: Remove Prompts/Apps from parse/classify.go

**Files to modify:**
- `cli/internal/parse/classify.go`

**Changes:**
1. Remove `"prompts": catalog.Prompts` from the `knownDirs` map

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/parse/...`

### Task 1.4: Remove Prompts/Apps from loadout package

**Files to modify:**
- `cli/internal/loadout/manifest.go`
- `cli/internal/loadout/resolve.go`

**Changes in manifest.go:**
1. Remove `Prompts []string` field and `Apps []string` field from `Manifest` struct
2. Remove `len(m.Prompts) + len(m.Apps)` from `ItemCount()`
3. Remove the `Prompts` and `Apps` blocks from `RefsByType()`

**Changes in resolve.go:**
1. Update the comment to remove "Prompts" and "Apps" from universal type examples

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/loadout/...`

### Task 1.5: Remove Prompts/Apps from add/add.go

**Files to modify:**
- `cli/internal/add/add.go`

**Changes:**
1. Remove `case catalog.Prompts: return "PROMPT.md"` from `contentFilename()`
2. The `AllContentTypes()` loop in `BuildLibraryIndex()` and `DiscoverFromProvider()` will automatically exclude Prompts/Apps since they were removed from `AllContentTypes()`

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/add/...`

### Task 1.6: Remove Prompts/Apps special-casing from detail.go

**Files to modify:**
- `cli/internal/tui/detail.go`

**Changes:**
1. Remove the `if item.Type == catalog.Apps && item.Body != ""` glamour render block
2. Remove `if item.Type != catalog.Prompts` guard on provider checkbox initialization -- always initialize checkboxes
3. Remove all `catalog.Prompts` and `catalog.Apps` cases from `Update()`:
   - Scroll handling in `keys.Up` and `keys.Down`
   - Scrollable check in `keys.PageUp` and `keys.PageDown`
   - Guard on `keys.Install` and `keys.Uninstall`
   - `catalog.Prompts` from `keys.Copy` (keep the Library `llmPrompt` copy)
   - `catalog.Prompts` from `keys.Save`

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/tui/...`

### Task 1.6b: Remove Prompts/Apps from risk, validation, and provider files

**Files to modify:**
- `cli/internal/catalog/risk.go`
- `cli/internal/catalog/risk_test.go`
- `cli/internal/metadata/validate.go`
- `cli/internal/provider/kiro.go`

**Changes in risk.go:**
1. Remove the `case Apps:` block from `RiskIndicators()`
2. Remove the `appRisks()` function

**Changes in risk_test.go:**
1. Remove or rewrite the 3 App-specific tests (at lines 130, 151, 167)

**Changes in validate.go:**
1. Remove `case "prompts":` and `case "apps":` from the validation switch (lines 61, 65)

**Changes in kiro.go:**
1. Remove the `"prompts"` directory path reference (line 41)

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/catalog/... ./cli/internal/metadata/... ./cli/internal/provider/...`

### Task 1.7: Remove Prompts/Apps special-casing from detail_render.go

**Files to modify:**
- `cli/internal/tui/detail_render.go`

**Changes:**
1. Remove "Prompt body" display block
2. Remove "App supported providers" display block
3. Remove `catalog.Apps` fallback rendering in README section
4. Remove `if m.item.Type != catalog.Prompts` guard on provider checkbox section -- always show provider checkboxes
5. Remove `catalog.Prompts` and `catalog.Apps` from action button conditionals (keep Library LLM copy button)
6. Remove `{"Prompts", manifest.Prompts}` and `{"Apps", manifest.Apps}` from loadout contents tab
7. Remove `catalog.Prompts` from help text rendering
8. Remove `catalog.Apps` from help text rendering

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/tui/...`

### Task 1.8: Remove Prompts/Apps from app.go

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
1. Remove `catalog.Prompts` and `catalog.Apps` entries from `categoryDesc` map

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/tui/...`

### Task 1.9: Remove Prompts/Apps from items.go

**Files to modify:**
- `cli/internal/tui/items.go`

**Changes:**
1. Remove `catalog.Apps` check in `buildProvCell()` -- the `SupportedProviders` field no longer exists

**Dependencies:** Task 1.1
**Verification:** `go build ./cli/internal/tui/...`

### Task 1.10: Update test helpers and test files

**Files to modify:**
- `cli/internal/tui/testhelpers_test.go`
- `cli/internal/tui/detail_test.go`
- `cli/internal/tui/detail_render_test.go`
- `cli/internal/tui/modal_test.go`
- `cli/internal/catalog/scanner_test.go`
- `cli/internal/loadout/validate_test.go`
- `cli/internal/parse/discovery_test.go`

**Changes in testhelpers_test.go:**
1. Remove `makePrompt()` function
2. Remove `makeApp()` function
3. Remove calls to `makePrompt` and `makeApp` from `testCatalog()`

**Changes in detail_test.go:**
1. Remove/rewrite these tests that reference `catalog.Prompts` or `catalog.Apps`:
   - `TestDetailOverviewPromptBody` (line 318)
   - `TestDetailOverviewAppProviders` (line 324)
   - `TestDetailPromptCopy` (line 737)
   - `TestDetailPromptSavePath` (line 750)
   - `TestHelpBarNoSaveOnOverviewTab` (line 930) -- check for Prompts references

**Changes in detail_render_test.go:**
1. Update `TestRenderContentSplitHasSeparator` (line 13) -- change test item type from `catalog.Prompts` to another valid type
2. Update `TestRenderContentSplitMetadataInPinned` (line 29) -- change test item type from `catalog.Prompts` to another valid type
3. Remove `TestRenderOverviewTabNoRiskForPrompts` (line 112) entirely

**Changes in modal_test.go:**
1. Update the `TestUninstallKeyEmitsOpenModalMsg` subtest that uses `catalog.Apps` (line 335) -- change to a valid content type or remove the Apps-specific subtest

**Changes in scanner_test.go:**
1. Remove test assertions about `Prompts` body

**Changes in validate_test.go:**
1. Remove test entry referencing `catalog.Prompts`

**Changes in discovery_test.go:**
1. Remove any assertions about Prompts type

**Dependencies:** Tasks 1.1-1.9
**Verification:** `cd cli && make test`

### Task 1.11: Delete example content directories

**Files to delete:**
- `content/prompts/` (entire directory tree)
- `content/apps/` (entire directory tree)

**Dependencies:** Task 1.10
**Verification:** `ls content/` -- no prompts or apps directories

### Task 1.12: Regenerate golden files

**Files to modify:** All golden files in `cli/internal/tui/testdata/`

**Changes:**
Run `cd cli && go test ./internal/tui/ -update-golden` and review the diffs:
- All sidebar golden files should no longer show "Prompts" and "Apps" rows
- Category welcome cards should have fewer cards
- Sidebar item counts and cursor positions will shift

**Dependencies:** Tasks 1.10, 1.11
**Verification:** `cd cli && make test` -- all tests pass

---

## Phase 2: Settings Cleanup

### Task 2.1: Remove provider sub-picker infrastructure from settings.go

**Files to modify:**
- `cli/internal/tui/settings.go`

**Changes:**
1. Remove `settingsEditMode` type and its constants `editNone`, `editProviders`
2. Remove `editMode`, `subItems`, `subCur` fields from `settingsModel` struct
3. Remove `dirty` field from `settingsModel` struct
4. Remove `settingsPickerItem` struct
5. Remove `applySubPicker()` method
6. Remove `HasPendingAction()` method
7. Remove `CancelAction()` method
8. Remove the sub-picker key handling block in `Update()`
9. Remove the sub-picker mouse handling in `Update()`
10. Remove `case 1: // providers` from `activateRow()`
11. Change `settingsRowCount()` to return 2
12. Renumber `activateRow()`: row 0 = auto-update, row 1 = registry-auto-sync (was row 2)
13. Add `m.save()` call at the end of `activateRow()` after toggling any value (auto-save)
14. Remove `dirty = true` assignments
15. Remove `keys.Save` key binding handler from `Update()`
16. Remove Row 1 (Providers) rendering from `View()`
17. Update `settingsDescriptions` to only have 2 entries (remove the providers description at index 1)
18. Remove sub-picker overlay rendering from `View()`
19. Update `helpText()` to remove "s save" hint
20. Remove `editMode == editNone` guard from `renderRow()` cursor check

**Dependencies:** Phase 1 complete
**Verification:** `go build ./cli/internal/tui/...`

### Task 2.2: Remove HasPendingAction/CancelAction calls from app.go

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
1. Remove `settings.HasPendingAction()` and `settings.CancelAction()` calls in the `screenSettings` Esc handler
2. Remove the `dirty` check and `save()` call on Esc -- settings now auto-save on every toggle
3. Simplify the `screenSettings` Esc handler to just: navigate back to screenCategory

**Dependencies:** Task 2.1
**Verification:** `go build ./cli/internal/tui/...`

### Task 2.3: Update settings tests

**Files to modify:**
- `cli/internal/tui/settings_test.go`

**Changes:**
1. Update `navigateToSettings` -- cursor offset changes because Prompts and Apps are removed from AllContentTypes()
2. Update `TestSettingsNavigation` -- now 2 rows (0=auto-update, 1=registry-auto-sync), clamp at 1 not 2
3. Remove `TestSettingsProviderSubPicker`
4. Remove `TestSettingsProviderPickerNav`
5. Remove `TestSettingsProviderPickerToggle`
6. Remove `TestSettingsProviderPickerEscApplies`
7. Update `TestSettingsSave` -- remove the 's' key test since manual save is gone. Replace with a test that toggling auto-update triggers auto-save
8. Remove `TestSettingsBackCancelsSubPicker` -- no sub-picker
9. Update `TestSettingsAutoSaveOnEsc` -- remove dirty check, verify that the toggle persisted
10. Update `TestSettingsViewRendering` -- remove `assertContains(t, view, "Providers")`

**Dependencies:** Tasks 2.1, 2.2
**Verification:** `cd cli && go test ./internal/tui/ -run TestSettings`

### Task 2.4: Regenerate golden files for settings

Run `cd cli && go test ./internal/tui/ -update-golden` and review diffs.

**Dependencies:** Task 2.3
**Verification:** `cd cli && make test`

---

## Phase 3: Bug Fixes

### Task 3.1: Fix sidebar click area padding for Configuration items

**Files to modify:**
- `cli/internal/tui/sidebar.go`

**Changes:**
In the `utilItems` loop, the non-selected, non-Registries items render as:
```go
rowContent = "  " + itemStyle.Render(u.label)
```
This doesn't pad to full width. Fix by padding the label:
```go
rowContent = "  " + itemStyle.Render(fmt.Sprintf("%-*s", inner-2, u.label))
```
This matches how content type rows pad via count formatting. Apply to all Configuration items without counts.

**Dependencies:** Phase 1 (content type count changes affect sidebar layout)
**Verification:** Run the app, click on "Add", "Update", "Settings", "Sandbox" text -- clicks should register across the full row width. Also: `cd cli && go test ./internal/tui/ -run TestSidebar`

### Task 3.2: Fix discovery path -- use cwd instead of projectRoot

**Files to modify:**
- `cli/internal/tui/import.go`

**Changes:**
In the discovery command function, the discovery call currently passes `m.projectRoot`. Change to resolve the actual working directory:
```go
cwd, _ := os.Getwd()
if cwd == "" {
    cwd = m.projectRoot
}
items, err := add.DiscoverFromProvider(prov, cwd, nil, globalDir)
```

Check all `DiscoverFromProvider` calls in import.go and apply the same fix if they use `m.projectRoot`.

**Dependencies:** None (independent)
**Verification:** From a project directory with `.claude/` rules, run `syllago` TUI, go to Add > From Provider -- discovered items should include the project-local provider content.

### Task 3.3: Add dual-scope discovery (project + global)

**Files to modify:**
- `cli/internal/add/add.go`
- `cli/internal/tui/import.go`

**Note:** Before implementing, verify that `discoveryDoneMsg` struct fields match the code below. The existing code at line 1741 constructs `discoveryDoneMsg{items: items, err: err}`, so the `items` field should exist, but confirm the type is compatible with `[]add.DiscoveryItem`.

**Changes in add.go:**
1. Add `Scope string` field to `DiscoveryItem` struct (after `Status`) -- values: "project" or "global"

**Changes in import.go:**
In the discovery command function, call `DiscoverFromProvider` twice:
```go
return func() tea.Msg {
    globalDir := catalog.GlobalContentDir()
    cwd, _ := os.Getwd()
    if cwd == "" {
        cwd = m.projectRoot
    }
    homeDir, _ := os.UserHomeDir()

    // Project-scope discovery
    projectItems, err := add.DiscoverFromProvider(prov, cwd, nil, globalDir)
    if err != nil {
        return discoveryDoneMsg{err: err}
    }
    for i := range projectItems {
        projectItems[i].Scope = "project"
    }

    // Global-scope discovery (skip if cwd == home)
    var globalItems []add.DiscoveryItem
    if homeDir != "" && homeDir != cwd {
        globalItems, _ = add.DiscoverFromProvider(prov, homeDir, nil, globalDir)
        for i := range globalItems {
            globalItems[i].Scope = "global"
        }
    }

    // Merge: project wins for same type/name
    seen := make(map[string]bool)
    var merged []add.DiscoveryItem
    for _, item := range projectItems {
        key := string(item.Type) + "/" + item.Name
        seen[key] = true
        merged = append(merged, item)
    }
    for _, item := range globalItems {
        key := string(item.Type) + "/" + item.Name
        if !seen[key] {
            merged = append(merged, item)
        }
    }
    return discoveryDoneMsg{items: merged}
}
```

Also update the discovery select rendering to show scope tags:
```go
scopeTag := ""
if item.Scope == "global" {
    scopeTag = " " + helpStyle.Render("[global]")
} else if item.Scope == "project" {
    scopeTag = " " + helpStyle.Render("[project]")
}
```

**Dependencies:** Task 3.2
**Verification:** From a project with provider content at both `~/.claude/` and `./claude/`, confirm both scopes appear in discovery. Add a unit test for merge/dedup logic.

### Task 3.4: Undetected providers in provider pick list

**Deferred.** Showing undetected providers dimmed with "not detected" label and prompting for custom path (design doc section 1b, item 6) is deferred to a follow-up PR. Rationale: it depends on the custom provider locations feature (see `~/.claude/plans/purring-orbiting-minsky.md`) which adds `PathResolver` infrastructure. Implementing the UI without the resolver would require throwaway code. The current behavior (only showing detected providers) is functional.

### Task 3.5: Regenerate golden files for bug fixes

Run `cd cli && go test ./internal/tui/ -update-golden` and review diffs.

**Dependencies:** Tasks 3.1-3.3
**Verification:** `cd cli && make test`

---

## Phase 4: Visual Polish -- Hide Redundant Library Badge

### Task 4.1: Add hideLibraryBadge flag to items rendering

**Files to modify:**
- `cli/internal/tui/items.go`

**Changes:**
1. Add `hideLibraryBadge bool` field to `itemsModel` struct
2. In the `View()` method, where Library badge is rendered, add a check:
```go
} else if item.Library {
    if !m.hideLibraryBadge {
        localPrefix = warningStyle.Render("[LIBRARY]") + " "
        localPrefixLen = 10
    }
}
```

**Dependencies:** Phase 1 (content types changed)
**Verification:** `go build ./cli/internal/tui/...`

### Task 4.2: Set hideLibraryBadge when entering Library view

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
In the `isLibrarySelected()` handler, after creating the itemsModel, set:
```go
items.hideLibraryBadge = true
```

Also set the same flag in:
- The back-from-detail Library refresh
- The search live-filter Library branch

**Dependencies:** Task 4.1
**Verification:** Navigate to Library in the TUI -- items should NOT show `[LIBRARY]` badge. Navigate to a content type list -- Library items should still show the badge.

### Task 4.3: Regenerate golden files

Run `cd cli && go test ./internal/tui/ -update-golden` and review.

**Dependencies:** Task 4.2
**Verification:** `cd cli && make test`

---

## Phase 5: Sidebar Reorganization

### Task 5.1a: Sidebar data model changes

**Files to modify:**
- `cli/internal/tui/sidebar.go`

**Changes:**
1. Filter out `Loadouts` from content types displayed in sidebar:
```go
var filtered []catalog.ContentType
for _, ct := range catalog.AllContentTypes() {
    if ct != catalog.Loadouts {
        filtered = append(filtered, ct)
    }
}
m.types = filtered
```
2. Add `loadoutsCount int` field to `sidebarModel`
3. **Update `totalItems()`:** Change from `len(m.types) + 6` to `len(m.types) + 7`. This is a critical single-line change -- the +7 accounts for: Library + Loadouts + Add + Update + Settings + Registries + Sandbox. Test failures will cascade if this is wrong.
4. Update index arithmetic:
   - Content types: indices `0..len(types)-1`
   - Library: `len(types)`
   - Loadouts (collections): `len(types) + 1`
   - Add: `len(types) + 2`
   - Update: `len(types) + 3`
   - Settings: `len(types) + 4`
   - Registries: `len(types) + 5`
   - Sandbox: `len(types) + 6`
5. Update selector methods:
```go
func (m sidebarModel) isLibrarySelected() bool    { return m.cursor == len(m.types) }
func (m sidebarModel) isLoadoutsSelected() bool   { return m.cursor == len(m.types)+1 }
func (m sidebarModel) isAddSelected() bool        { return m.cursor == len(m.types)+2 }
func (m sidebarModel) isUpdateSelected() bool     { return m.cursor == len(m.types)+3 }
func (m sidebarModel) isSettingsSelected() bool   { return m.cursor == len(m.types)+4 }
func (m sidebarModel) isRegistriesSelected() bool { return m.cursor == len(m.types)+5 }
func (m sidebarModel) isSandboxSelected() bool    { return m.cursor == len(m.types)+6 }
```

**Dependencies:** Phases 1-4
**Verification:** `go build ./cli/internal/tui/...`

### Task 5.1b: Sidebar view rendering changes

**Files to modify:**
- `cli/internal/tui/sidebar.go`

**Changes:**
Restructure from two sections (AI Tools + Configuration) into three (Content + Collections + Configuration).

New layout:
```
Content         <- renamed from "AI Tools"
  Skills        3
  Agents        2
  MCP           1
  Rules         5
  Hooks         4
  Commands      1
-----------
Collections
  Library      12
  Loadouts      2
-----------
Configuration
  Add
  Update
  Settings
  Registries    1
  Sandbox
```

1. Rename section header from "AI Tools" to "Content"
2. Rewrite `View()` to render three sections with separators between them
3. Pad ALL non-selected items to full row width

**Dependencies:** Task 5.1a
**Verification:** `go build ./cli/internal/tui/...`

### Task 5.2: Update app.go sidebar routing for new indices

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
1. Add handler for `isLoadoutsSelected()` in the Enter/Right key handler. For now, navigate to items screen filtered to Loadouts:
```go
if a.sidebar.isLoadoutsSelected() {
    src := a.visibleItems(a.catalog.ByType(catalog.Loadouts))
    items := newItemsModel(catalog.Loadouts, src, a.providers, a.catalog.RepoRoot)
    items.width = a.width - sidebarWidth - 1
    items.height = a.panelHeight()
    a.items = items
    a.screen = screenItems
    a.focus = focusContent
    return a, nil
}
```
2. All other `isXxxSelected()` calls are unchanged since they use method names, not raw indices.
3. Update `refreshSidebarCounts()` to also update `loadoutsCount`.

**Dependencies:** Task 5.1a, 5.1b
**Verification:** Run TUI, verify all sidebar items navigate correctly.

### Task 5.3: Update sidebar and navigation tests

**Files to modify:**
- `cli/internal/tui/sidebar_test.go`
- `cli/internal/tui/settings_test.go`
- `cli/internal/tui/import_test.go`
- `cli/internal/tui/navigation_test.go`
- `cli/internal/tui/category_test.go`
- `cli/internal/tui/esc_back_test.go`
- `cli/internal/tui/integration_test.go`

**Changes:**
All tests that navigate by pressing keyDown N times to reach a sidebar item need cursor offset updates. After Phase 1, `AllContentTypes()` returns 7 types but sidebar filters out Loadouts, so sidebar content items = 6. New index map:

| Item | New index |
|------|-----------|
| Skills | 0 |
| Agents | 1 |
| MCP | 2 |
| Rules | 3 |
| Hooks | 4 |
| Commands | 5 |
| Library | 6 |
| Loadouts | 7 |
| Add | 8 |
| Update | 9 |
| Settings | 10 |
| Registries | 11 |
| Sandbox | 12 |

Update all `pressN(app, keyDown, N)` calls to use the new indices.

**Dependencies:** Tasks 5.1a, 5.1b, 5.2
**Verification:** `cd cli && make test`

### Task 5.4: Regenerate golden files

Run `cd cli && go test ./internal/tui/ -update-golden`.

**Dependencies:** Task 5.3
**Verification:** `cd cli && make test`

---

## Phase 6: Card Views (Library & Loadouts)

### Task 6.1a: Library card view -- screen enum and render method

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
1. Add `screenLibraryCards` to the `screen` enum
2. Create a `renderLibraryCards()` method on App that:
   - Groups library items by content type
   - For each type with library items, renders a card (reuse `cardStyle` from `renderWelcomeCards`)
   - Each card shows: type name, item count
   - Cards are zone-marked (e.g., `"library-card-skills"`) for click navigation

**Dependencies:** Phase 5
**Verification:** `go build ./cli/internal/tui/...`

### Task 6.1b: Library card view -- keyboard navigation

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
1. Add keyboard handling for `screenLibraryCards`: up/down to navigate between cards, Enter to drill into the selected card

**Dependencies:** Task 6.1a
**Verification:** Navigate to Library, use arrow keys between cards, Enter to drill in.

### Task 6.1c: Library card view -- click zones and drill-down

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
1. Wire up click zone handling for library cards
2. Clicking a card navigates to items screen filtered to that type + library-only:
```go
src := a.catalog.ByTypeLibrary(ct)
items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
items.hideLibraryBadge = true
```

**Dependencies:** Task 6.1a
**Verification:** Click a library card, see filtered items.

### Task 6.2a: Loadouts card view -- screen enum and render method

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
1. Add `screenLoadoutCards` to the `screen` enum
2. Create a `renderLoadoutCards()` method that:
   - Gets all loadout items from catalog
   - Groups them by provider field
   - Renders one card per provider with loadout count
   - Zone-marked for click navigation

**Dependencies:** Phase 5
**Verification:** `go build ./cli/internal/tui/...`

### Task 6.2b: Loadouts card view -- click zones and drill-down

**Files to modify:**
- `cli/internal/tui/app.go`

**Changes:**
1. Add keyboard handling for `screenLoadoutCards`: up/down navigation, Enter to drill in
2. Wire up click zone handling
3. When clicking a card, navigate to items filtered to loadouts for that provider

**Dependencies:** Task 6.2a
**Verification:** Navigate to Loadouts in Collections, see cards grouped by provider. Click a card to drill in.

### Task 6.3: Update tests for card views

**Files to modify:**
- `cli/internal/tui/category_test.go` (or new `cli/internal/tui/library_test.go`)
- `cli/internal/tui/navigation_test.go`

**Changes:**
1. Test that navigating to Library shows card screen (not items directly)
2. Test that Enter on a library card navigates to items filtered by type
3. Test keyboard navigation between cards
4. Test that navigating to Loadouts shows card screen

**Dependencies:** Tasks 6.1a-6.1c, 6.2a-6.2b
**Verification:** `cd cli && make test`

### Task 6.4: Regenerate golden files

Run `cd cli && go test ./internal/tui/ -update-golden`.

**Dependencies:** Task 6.3
**Verification:** `cd cli && make test`

---

## Phase 7: Add Wizard Breadcrumbs

### Task 7.1a: Breadcrumb and sourceLabel methods

**Files to modify:**
- `cli/internal/tui/import.go`

**Changes:**
1. Replace `stepLabel()` method with `breadcrumb()` method that returns a clickable breadcrumb string based on the current step and flow:
```go
func (m importModel) breadcrumb() string {
    home := zone.Mark("crumb-home", helpStyle.Render("Home"))
    arrow := helpStyle.Render(" > ")
    add := zone.Mark("add-crumb-source", helpStyle.Render("Add"))

    parts := []string{home, add}

    switch m.step {
    case stepSource:
        parts = append(parts, titleStyle.Render("Source"))
    case stepType:
        source := m.sourceLabel()
        parts = append(parts, zone.Mark("add-crumb-type-back", helpStyle.Render(source)))
        parts = append(parts, titleStyle.Render("Content Type"))
    case stepProviderPick:
        parts = append(parts, titleStyle.Render("Select Provider"))
    case stepDiscoverySelect:
        parts = append(parts, zone.Mark("add-crumb-provider-back", helpStyle.Render(m.discoveryProvider.Name)))
        parts = append(parts, titleStyle.Render("Select Items"))
    // ... similar for each step
    }

    return strings.Join(parts, arrow)
}
```

2. Add helper `sourceLabel()`:
```go
func (m importModel) sourceLabel() string {
    switch m.sourceCursor {
    case 0: return "From Provider"
    case 1: return "Local Path"
    case 2: return "Git URL"
    case 3: return "Create New"
    }
    return "Source"
}
```

3. Update `View()` to use breadcrumb instead of stepLabel

**Dependencies:** Phase 1 (content types changed, Prompts/Apps removed from type picker)
**Verification:** `go build ./cli/internal/tui/...`

### Task 7.1b: Breadcrumb mouse click handling

**Files to modify:**
- `cli/internal/tui/import.go`

**Changes:**
1. Add mouse click handling for breadcrumb zone marks -- each crumb zone ID maps to a step to navigate back to:
   - `"crumb-home"` -> navigate back to screenCategory
   - `"add-crumb-source"` -> navigate to stepSource
   - `"add-crumb-type-back"` -> navigate to stepSource (go back from type selection)
   - `"add-crumb-provider-back"` -> navigate to stepProviderPick (go back from discovery)

**Dependencies:** Task 7.1a
**Verification:** Click breadcrumb segments in the TUI to verify navigation.

### Task 7.2: Update import tests for breadcrumbs

**Files to modify:**
- `cli/internal/tui/import_test.go`

**Changes:**
1. Remove any assertions that check for "Step N of M" text in the view output
2. Add tests for breadcrumb rendering at each step
3. Add tests for breadcrumb click navigation (clicking a crumb navigates back to that step)
4. Test that clicking "Home" crumb navigates all the way back

**Dependencies:** Tasks 7.1a, 7.1b
**Verification:** `cd cli && go test ./internal/tui/ -run TestImport`

### Task 7.3: Regenerate golden files

Run `cd cli && go test ./internal/tui/ -update-golden`.

**Dependencies:** Task 7.2
**Verification:** `cd cli && make test`

---

## Phase Summary

| Phase | Tasks | Key Risk |
|-------|-------|----------|
| 1: Content Type Removal | 13 tasks | Wide blast radius across many files; must not miss references |
| 2: Settings Cleanup | 4 tasks | Low risk, well-scoped |
| 3: Bug Fixes | 5 tasks | Discovery dual-scope adds new behavior; undetected providers deferred |
| 4: Visual Polish | 3 tasks | Low risk |
| 5: Sidebar Reorg | 5 tasks | Index arithmetic affects many tests |
| 6: Card Views | 7 tasks | New screens, most complex UI work |
| 7: Breadcrumbs | 4 tasks | Many step combinations to handle |

**Total:** 41 tasks across 7 phases.

**Commit strategy:** One commit per phase after all tests pass and golden files are regenerated.
