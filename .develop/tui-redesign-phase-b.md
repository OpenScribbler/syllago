# TUI Redesign — Phase B: Hidden Blocker and Missing Context Analysis

**Date:** 2026-02-17
**Plan:** `/home/hhewett/.local/src/nesco/docs/plans/2026-02-17-tui-redesign-implementation.md`
**Codebase:** `/home/hhewett/.local/src/nesco/cli/internal/tui/`

---

## Summary of Issues Found

| Severity | Count | Category |
|----------|-------|----------|
| BLOCKER  | 2     | `erikgeiser/bubbletea-overlay` does not exist; `keys.Right` binding missing |
| HIGH     | 5     | `doSavePrompt`/`runAppScriptCmd` don't exist; `env.AvailableTypes()`/`env.Refresh()` don't exist; `a.category.message` orphan in Task 3.4; search still uses `a.category.selectedType()` |
| MEDIUM   | 6     | `tooSmall` threshold inconsistency; Tab key conflict in detail; `category.message` field lost on import; `App.View()` tooSmall logic error; sidebar `title` placeholder dead code; zone.Mark on newlines |
| LOW      | 3     | WCAG test for `mutedColor dark`; `TestSelectedItemHasBackground` will fail after 1.1; Task 1.2 lists wrong lines to replace |

---

## Group 1: Foundation — Styles and Dependencies

### Task 1.1: Update Semantic Color Variables in styles.go

**Checks:**

- [x] **Implicit dependencies** — No upstream dependencies. `styles.go` is self-contained.
- [x] **Missing context** — The existing file (lines 5-14) uses `secondaryColor`; the plan correctly identifies all referencing styles (`selectedItemStyle` at line 29, which references `secondaryColor` via its foreground). The `selectedItemStyle` background in the existing code uses raw `lipgloss.AdaptiveColor{}` literals, not a named variable, so Step 3 replaces both correctly.
- [ ] **Hidden blocker** — `selectedItemStyle` in the existing `styles.go` (lines 28-34) has a background using `Light: "#1E293B"` / `Dark: "#E2E8F0"` which is a hardcoded `lipgloss.AdaptiveColor{}` literal, not `selectedBgColor`. The Step 3 replacement correctly switches to `selectedBgColor`, but the executor must also remove the inline literal. The plan code for Step 3 shows the style _without_ the old inline literal, so it is correct as written — but the executor needs to understand this is a full replacement, not an addition.
- [ ] **Cross-task conflict** — `TestSelectedItemHasBackground` in `styles_test.go` currently checks that `selectedItemStyle` has a background color. After Task 1.1 Step 3, `selectedItemStyle` still has a background (`selectedBgColor`), so the test should still pass. However, Task 1.2 does NOT update `TestSelectedItemHasBackground` — it only updates `TestColorsAreAdaptive`. If `selectedBgColor` is `lipgloss.AdaptiveColor` (it is), the test passes. **This is fine** — no conflict.
- [ ] **Cross-task conflict** — `TestColorsAreAdaptive` in `styles_test.go` currently references `secondaryColor`. After Task 1.1, `secondaryColor` is deleted. If the executor runs tests before Task 1.2, `go test` will fail to compile. Task 1.2 depends on Task 1.1, but the plan's execution order does not make this explicit enough. The executor must complete Task 1.2 immediately after 1.1 before running tests.
  - **Fix:** Add to success criteria: "Run `go build ./...` (not `go test`) after Task 1.1 to verify compilation before proceeding to Task 1.2."
- [x] **Success criteria completeness** — Criteria are clear and verifiable.

---

### Task 1.2: Update styles_test.go for Renamed Variable

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 1.1.
- [ ] **Missing context** — The task says to replace lines 21-34, but it does NOT mention updating `TestSelectedItemHasBackground` (lines 9-18). That test checks `selectedItemStyle.GetBackground()` is non-nil and is `AdaptiveColor`. After Task 1.1, the background changes from a raw `lipgloss.AdaptiveColor{}` literal to `selectedBgColor` (which is also `AdaptiveColor`). The test will still pass, but the executor should know this was checked — it is not documented.
  - **Fix:** Add a note: "Verify `TestSelectedItemHasBackground` still passes after Task 1.1 changes — `selectedBgColor` is `AdaptiveColor`, so it should."
- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None.
- [x] **Success criteria completeness** — Clear. The test function names are exact.

---

### Task 1.3: Add New Dependencies

**Checks:**

- [x] **Implicit dependencies** — None. Correctly runs in parallel with 1.1.
- [x] **Missing context** — Existing `go.mod` uses Go 1.25.5 (confirmed). `bubblezone v1.0.0` is confirmed to exist and resolves successfully.
- [x] **Hidden blocker — `bubblezone`** — `github.com/lrstanley/bubblezone` exists and resolves to v1.0.0. The API (`zone.NewGlobal()`, `zone.Mark()`, `zone.Get()`, `zone.Scan()`, `ZoneInfo.InBounds()`) all exist exactly as the plan uses them. **No blocker.**
- [ ] **BLOCKER — `erikgeiser/bubbletea-overlay`** — **This package does not exist.** Verified: `go get github.com/erikgeiser/bubbletea-overlay@v0.6.5` fails with "remote: Repository not found." There is no GitHub repository at that path. The plan's tech stack lists it as a dependency, and Tasks 6.1–6.6 all import it as `overlay "github.com/erikgeiser/bubbletea-overlay"` and call `overlay.PlacePosition(lipgloss.Center, lipgloss.Center, m.View(), background)`.

  The closest real packages are:
  - `github.com/rmhubbert/bubbletea-overlay` (v0.6.3, most actively maintained) — but its API uses `overlay.Composite(fg, bg string, xPos, yPos Position, xOff, yOff int) string`, not `PlacePosition`.
  - `github.com/jsdoublel/bubbletea-overlay` and `github.com/quickphosphat/bubbletea-overlay` also exist but use different APIs.

  **Fix:** Choose an actual package. The simplest drop-in for the plan's centering use-case is `github.com/rmhubbert/bubbletea-overlay`. Replace `overlay.PlacePosition(lipgloss.Center, lipgloss.Center, m.View(), background)` with `overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)` throughout all modal `overlayView()` methods. Update all import paths and the go.mod entry.

  Alternatively, implement centering manually with `lipgloss.Place()` — no overlay dependency needed for simple cases. `lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)` can achieve the same result if the terminal dimensions are passed in.

- [x] **Success criteria completeness** — The criteria specify go.mod entries and `go build ./...`. Add: "Verify `overlay.PlacePosition` API matches the chosen package's exported functions."

---

## Group 2: Sidebar Model

### Task 2.1: Create sidebar.go

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 1.1 (needs `sidebarBorderStyle`, `selectedItemStyle`, `itemStyle`, `titleStyle`, `helpStyle` from styles.go).
- [ ] **Missing context — `keys.Right` does not exist** — Task 3.3 (Step 3) references `key.Matches(msg, keys.Right)` in the sidebar Enter/Right routing block. The `keys.go` file defines `Up`, `Down`, `Enter`, `Back`, `Quit`, `Search`, `Install`, `Uninstall`, `Copy`, `Save`, `Space`, `EnvSetup`, `Promote`, `Tab`, `ShiftTab`, `Help`, `Home`, `End` — but **no `Right` binding**. The sidebar code in Task 2.1 does not use `keys.Right` directly, but Task 3.3 Step 3 does. The executor of 2.1 does not need to know this, but the executor of 3.3 will be blocked.
  - **Fix:** Task 3.3 must add a `Right key.Binding` to `keys.go` before using `key.Matches(msg, keys.Right)`, or use the raw `msg.Type == tea.KeyRight` check instead.
- [ ] **Missing context — dead code in View()** — The sidebar `View()` has:
  ```go
  title := primaryColor.Dark // placeholder; actual rendering uses lipgloss
  _ = title
  ```
  This references `primaryColor.Dark` (a string field on `lipgloss.AdaptiveColor`) directly. While it compiles, it is dead code that will confuse an executor who tries to clean it up. The plan should either remove it or explain why it's there.
  - **Fix:** Remove those two lines entirely from the View() implementation.
- [ ] **Missing context — `sidebarBorderStyle.Width()` behavior** — `sidebarBorderStyle` uses `BorderRight(true)`. When `lipgloss.Style.Width(sidebarWidth)` is called, the width applies to the inner content area, not including the border character. So the rendered column will be `sidebarWidth + 1` chars wide (18 inner + 1 border = 19), not 18. The plan comment says "fixed width (~18 chars including border character)" but the lipgloss `Width()` call gives 18 inner width. This is a layout calculation issue that will affect the `contentWidth := a.width - sidebarWidth` calculation in Task 3.2.
  - **Fix:** Either set `sidebarWidth = 17` (so rendered = 18 with border), or set `contentWidth := a.width - sidebarWidth - 1` in Task 3.2 to account for the border character. Clarify in the task which interpretation is intended.
- [x] **Hidden blockers** — `catalog.AllContentTypes()`, `cat.CountByType()`, `cat.CountLocal()` all confirmed to exist in the catalog package.
- [x] **Cross-task conflicts** — The existing `categoryModel` is kept until Task 3.4. No conflict.
- [x] **Success criteria completeness** — Clear and testable.

---

### Task 2.2: Wire sidebarModel into App struct

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 2.1.
- [x] **Missing context** — `newSidebarModel(cat, version)` requires `*catalog.Catalog` and `string`. Both are available in `NewApp()`. The executor needs to know to keep the existing `category: newCategoryModel(cat, version)` line for now (stated in the task).
- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None. Adding a field to App struct is additive only.
- [x] **Success criteria completeness** — Clear.

---

## Group 3: Layout Composition

### Task 3.1: Add focusTarget Type and Focus Field to App

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 2.2.
- [x] **Missing context** — The `screen` type and constants are at lines 16-25 in the actual file (confirmed). The executor can locate them.
- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None. Purely additive.
- [x] **Success criteria completeness** — Clear.

---

### Task 3.2: Refactor App.View() to Sidebar + Content Composition

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 3.1.
- [ ] **Missing context — `tooSmall` threshold inconsistency** — The current `App.Update()` sets `a.tooSmall = msg.Width < 40 || msg.Height < 10`. Task 3.2's new `View()` checks `if a.width < 60 || a.height < 10`. Task 3.3's `WindowSizeMsg` handler changes the threshold to `msg.Width < 60`. The executor must be aware that Task 3.2's `View()` code has `if a.tooSmall` at the top — but `tooSmall` is still set with the old `< 40` threshold until Task 3.3 updates the WindowSizeMsg handler. The View and Update thresholds will be out of sync between Tasks 3.2 and 3.3.
  - **Fix:** Either update the `WindowSizeMsg` threshold to `< 60` in Task 3.2 as well, or add a note that this inconsistency is intentional and harmless until 3.3 runs (since they should execute in sequence, not parallel).
- [ ] **Missing context — `renderFooter()` uses `strings.Repeat`** — The new `App.View()` calls `renderFooter()` which uses `strings.Repeat(" ", gap)`. The `strings` package is already imported in `app.go` (line 5: `"strings"`). No blocker, but executor should verify the import is present.
- [ ] **Missing context — `helpOverlay.View(a.screen)` unchanged** — The new `View()` keeps `if a.helpOverlay.active { contentView = a.helpOverlay.View(a.screen) }`. The executor should know that `helpOverlayModel.View(screen)` accepts a `screen` parameter — this is in the existing code and unchanged.
- [ ] **Missing context — search overlay logic removed** — The current `App.View()` has a complex search overlay that strips lines and replaces the help bar (lines 539-550 of current app.go). The new `View()` replaces this with `if a.search.active { footer = a.search.View() }`. The executor needs to understand that `searchModel.View()` returns a single-line string suitable as a footer replacement. Looking at the existing code, `a.search.View()` is currently concatenated at the bottom of the content — the new footer approach is a behavior change. The executor should verify that `searchModel.View()` output format is appropriate as a footer line.
- [ ] **Missing context — `screenCategory` in the new View()** — The new `View()` has a `default:` case that calls `a.renderContentWelcome()`, which covers `screenCategory`. But the executor should know that when the sidebar is focused and no category has been entered yet, `a.screen` is still `screenCategory` — so the welcome message is correct for the initial state.
- [ ] **Missing context — `strings` import in breadcrumb/renderFooter** — `renderFooter` calls `strings.Repeat` and `breadcrumb` references `a.sidebar.selectedType().Label()`. The `sidebar` field won't exist until Task 2.2 is complete (which it will be), but `selectedType()` returns `catalog.ContentType` which has a `.Label()` method — confirmed in the codebase.
- [ ] **Missing context — `displayName(a.detail.item)` in breadcrumb** — `displayName` is defined in `items.go`, not `app.go`. It is package-level (same `tui` package), so it's accessible. No blocker.
- [x] **Hidden blockers** — None beyond what's noted.
- [x] **Cross-task conflicts** — None.
- [x] **Success criteria completeness** — Criteria are functional. Add: "Verify `a.search.View()` output is a single-line string compatible with footer rendering."

---

### Task 3.3: Refactor App.Update() for Focus-Based Input Routing

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 3.2.
- [ ] **BLOCKER — `keys.Right` does not exist** — Step 3 has:
  ```go
  if key.Matches(msg, keys.Enter) || key.Matches(msg, keys.Right) {
  ```
  `keys.Right` is not defined in `keys.go`. This will cause a compile error.
  - **Fix:** Add to `keys.go` before Task 3.3:
    ```go
    Right: key.NewBinding(
        key.WithKeys("right", "l"),
        key.WithHelp("right/l", "enter"),
    ),
    ```
    And add `Right key.Binding` to the `keyMap` struct. This is an implicit dependency not captured in the plan.

- [ ] **Missing context — search still uses `a.category.selectedType()`** — In the existing `App.Update()`, the search Esc handler at line 262 does:
  ```go
  ct := a.category.selectedType()
  items := newItemsModel(ct, a.catalog.ByType(ct), a.providers, a.catalog.RepoRoot)
  ```
  And the search Enter handler at line 280 also references `a.category.selectedType()`. These are in the search active block, which Task 3.3 does NOT explicitly say to update. After Task 3.4 removes `category`, these references will become compile errors.
  - **Fix:** Task 3.3 Step 3 must also update the search active block to replace `a.category.selectedType()` with `a.sidebar.selectedType()`. This is an implicit dependency that needs to be explicitly stated.

- [ ] **Missing context — `promote` import needed in app.go** — Task 3.3 Step 3 introduces `promote.Promote()` directly in `App.Update()`:
  ```go
  result, err := promote.Promote(repoRoot, item)
  return promoteDoneMsg{result: result, err: err}
  ```
  Currently `promote` is only imported in `detail.go`. `app.go` does not import the `promote` package. After Task 3.3 Step 3, `app.go` will need:
  ```go
  "github.com/OpenScribbler/nesco/cli/internal/promote"
  ```
  This is not mentioned in the task.
  - **Fix:** Add to the task: "Add `\"github.com/OpenScribbler/nesco/cli/internal/promote\"` to app.go imports."

- [ ] **Missing context — `fmt` import needed in app.go sidebar zone loop** — Task 5.2 (mouse handling) uses `fmt.Sprintf("sidebar-%d", i)` in `App.Update()`. Currently `app.go` already imports `fmt` (line 4), so this is fine. But Task 3.3 does not add fmt calls, so no issue here.

- [ ] **Missing context — `updateCheckMsg` handler still sets `a.category.*`** — The existing `updateCheckMsg` handler (lines 160-162) sets:
  ```go
  a.category.remoteVersion = msg.remoteVersion
  a.category.updateAvailable = true
  a.category.commitsBehind = msg.commitsBehind
  ```
  Task 3.3 does not mention updating this handler. Task 3.4 references replacing `a.category.counts` references, but does not explicitly list `remoteVersion`, `updateAvailable`, or `commitsBehind` on `category`. After Task 3.4 removes the `category` field, these three lines will be compile errors.
  - **Fix:** Task 3.3 or 3.4 must also update the `updateCheckMsg` handler to set `a.sidebar.remoteVersion`, `a.sidebar.updateAvailable`, `a.sidebar.commitsBehind`. The `sidebarModel` struct in Task 2.1 already defines these fields, so the assignments are possible — but neither task explicitly includes this step.

- [ ] **Missing context — `autoUpdate` path in `updateCheckMsg` still uses old width** — The autoUpdate branch (lines 165-170) sets `a.updater.width = a.width` without subtracting `sidebarWidth`. Task 3.3 Step 1 updates the `WindowSizeMsg` handler to set widths correctly, but this auto-update path is not mentioned.
  - **Fix:** Add to Task 3.3: "In the `updateCheckMsg` autoUpdate branch, change `a.updater.width = a.width` to `a.updater.width = a.width - sidebarWidth`."

- [x] **Hidden blockers** — None beyond what's listed.
- [x] **Cross-task conflicts** — See search and category reference issues above.
- [ ] **Success criteria completeness** — Missing: "All `a.category.selectedType()` references in search handlers are replaced with `a.sidebar.selectedType()`." and "No compile errors referencing `a.category`."

---

### Task 3.4: Remove Redundant screenCategory View Routing and Category Model

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 3.3.
- [ ] **Missing context — `importDoneMsg` orphaned `a.category.message`** — The plan says:
  > "In importDoneMsg handler: ... remove: a.screen = screenCategory (sidebar is always visible now)"

  But `importDoneMsg` in the current `app.go` also sets `a.category.message`:
  ```go
  a.category.message = fmt.Sprintf("Imported %q successfully", msg.name)
  // and
  a.category.message = fmt.Sprintf("Imported %q but catalog rescan failed: %s", msg.name, err)
  ```
  The plan says to remove `a.screen = screenCategory` and set `a.focus = focusSidebar`, but does NOT mention what to do with `a.category.message`. This is a compile error after the category field is removed, and there is no replacement for it in the sidebar model.
  - **Fix:** The `sidebarModel` struct in Task 2.1 has no `message` field. Either add a `message string` field to `sidebarModel` and render it, or store the message elsewhere (e.g., `App.statusMessage string`). Add this to the task explicitly.

- [ ] **Missing context — `updateCheckMsg` handler three fields** — As noted in Task 3.3, the `updateCheckMsg` handler sets `a.category.remoteVersion`, `a.category.updateAvailable`, `a.category.commitsBehind`. These must become `a.sidebar.*` assignments. The plan says to replace `a.category.counts` in `updatePullMsg` and `promoteDoneMsg`, but the `updateCheckMsg` references are not listed.
  - **Fix:** Step 3 should also include: "In `updateCheckMsg` handler, replace `a.category.remoteVersion`, `a.category.updateAvailable`, `a.category.commitsBehind` with the corresponding `a.sidebar.*` fields."

- [ ] **Success criteria** — Add: "No references to `a.category` remain anywhere in `app.go`" and "The `importDoneMsg` message is displayed somewhere in the UI after the category is removed."

---

## Group 4: Detail View Layout

### Task 4.1: Refactor renderContent() to Put Metadata Above Separator

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 3.2.
- [ ] **Missing context — `renderContent()` is called from `clampScroll()`** — The existing `clampScroll()` in `detail.go` (lines 874-890) calls `m.renderContent()`. After Step 1 renames behavior (renderContent now returns combined pinned+body), the old `clampScroll()` will still call the new `renderContent()` which returns `pinned + body`. This means clampScroll computes scroll bounds over the combined content including the pinned header, which is wrong. Step 4 provides a new `clampScroll()` that calls `renderContentSplit()`. The executor needs to replace `clampScroll()` in `detail.go`, not `detail_render.go`. The plan says "detail_render.go" for Steps 1-3 and then Step 4 says `clampScroll()` — this function is in `detail.go`, not `detail_render.go`. This is correctly in `detail.go`, but the task header only lists `detail_render.go` as the file being modified, which is misleading.
  - **Fix:** Add `detail.go` to the Files list for Task 4.1: "Modify: `cli/internal/tui/detail.go` (clampScroll method)".

- [ ] **Missing context — renderContent() is also used in detail.View()** — The existing `detail.View()` (in `detail_render.go`, lines 435-495) calls `content := m.renderContent()` and then slices lines from it. Step 3 replaces `View()` entirely with the new split-based version. This is correct, but the executor needs to understand they are replacing the _entire_ `View()` function body in `detail_render.go`, not just one sub-function.

- [ ] **Missing context — metadata fields to delete from renderOverviewTab()** — Step 2 says to "delete the lines at 109-113 of detail_render.go." Looking at the actual file, the metadata block starts at line 109:
  ```go
  s += "\n"
  s += labelStyle.Render("Type: ") + valueStyle.Render(m.item.Type.Label()) + "\n"
  s += labelStyle.Render("Path: ") + valueStyle.Render(m.item.Path) + "\n"
  if m.item.Provider != "" {
      s += labelStyle.Render("Provider: ") + valueStyle.Render(m.item.Provider) + "\n"
  }
  ```
  This spans lines 109-114 (5 lines), not 109-113 (the plan says 4 lines). The plan's line count is off by one. This is minor but the executor should check the actual line numbers.

- [ ] **Missing context — `renderHelp()` returns help bar** — The new `View()` references `m.renderHelp()` which is in `detail_render.go` at line 497. This function is not changing in Task 4.1 and continues to exist. No blocker, but the executor should know not to touch it.

- [ ] **Missing context — `StripControlChars` used in renderContentSplit()** — `StripControlChars` is in `sanitize.go` in the same package. Already used in `detail_render.go`. No blocker.

- [x] **Hidden blockers** — No API limitations.
- [x] **Cross-task conflicts** — Task 4.1 changes `renderTabBar()` output (removes zone marks — those are added in Task 5.3). This is correct ordering since 5.3 depends on 4.1.
- [ ] **Success criteria completeness** — Add: "Modify line list includes `detail.go` for the `clampScroll()` replacement."

---

## Group 5: Mouse Support

### Task 5.1: Initialize bubblezone in main.go

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 1.3.
- [x] **Missing context** — `runTUI()` is at line 162 of `main.go`. `tea.NewProgram(app, tea.WithAltScreen())` is at line 196. The import block is at lines 3-22. `tea.WithMouseCellMotion()` is a valid option in Bubble Tea v1.3.10 (confirmed in the mouse.go API). All correct.
- [x] **Hidden blockers** — `zone.NewGlobal()` signature confirmed: `func NewGlobal()` with no return value. Correct usage.
- [x] **Cross-task conflicts** — None.
- [x] **Success criteria completeness** — Clear. Add: "Verify `zone.NewGlobal()` is called before `p := tea.NewProgram(...)`, not after."

---

### Task 5.2: Add zone.Mark() to Sidebar Rendering

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 5.1 and Task 2.1.
- [ ] **Missing context — zone.Mark wrapping newlines** — The sidebar View() builds rows as:
  ```go
  s += zone.Mark(fmt.Sprintf("sidebar-%d", i), rowContent) + "\n"
  ```
  The `\n` is outside the zone.Mark. This is correct — zone marks should not include newlines, as bubblezone uses ANSI sequences that track character positions. Including `\n` inside the mark could misalign hit detection. The plan code has this correct.
- [ ] **Missing context — `zone.Get()` returns `*ZoneInfo`** — The plan uses:
  ```go
  zone.Get(fmt.Sprintf("sidebar-%d", i)).InBounds(msg)
  ```
  `zone.Get()` in bubblezone v1.0.0 returns `*ZoneInfo`, which can be nil if the zone hasn't been scanned yet (before the first `zone.Scan()` in `View()`). Calling `.InBounds()` on a nil pointer panics. The task depends on Task 5.3 adding `zone.Scan()`, but the `MouseMsg` handler in App.Update runs on every mouse event — including ones before the first View render. Bubblezone's `ZoneInfo` has an `IsZero()` method specifically for this.
  - **Fix:** In the MouseMsg handler, add a nil/zero check:
    ```go
    zi := zone.Get(fmt.Sprintf("sidebar-%d", i))
    if zi != nil && !zi.IsZero() && zi.InBounds(msg) {
    ```
    Or verify that bubblezone's global manager handles the nil case gracefully (it does return a zero `ZoneInfo` from `Get()` when not found, but check `IsZero()` before calling `InBounds()`).

- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None.
- [ ] **Success criteria** — Add: "No panic when mouse events arrive before the first View() render (zone not yet scanned)."

---

### Task 5.3: Add zone.Mark() to Items List and Detail Tabs

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 5.2 and Task 4.1.
- [ ] **Missing context — items.go row format** — The plan's Step 1 example:
  ```go
  rowStr := fmt.Sprintf("%s%s%s  %s%s  %s\n", prefix, styledName, typeTag, localPrefix, helpStyle.Render(paddedDesc), provCells[i].styled)
  s += zone.Mark(fmt.Sprintf("item-%d", i), rowStr)
  ```
  The current `items.go` View() at lines 296-315 has two code paths: one for `showProvCol` and one without. The plan shows the `showProvCol` path. The executor must wrap both paths. The plan only shows one.
  - **Fix:** Add to the task: "Wrap item rows in both the `showProvCol` and non-`showProvCol` branches of the rendering loop."

- [ ] **Missing context — zone ID uses loop variable `i` but items view has a scroll offset** — Items view uses `offset`/`end` for viewport rendering. The loop is `for i := offset; i < end; i++`. When clicking item zone `"item-5"`, the handler checks:
  ```go
  for i := range a.items.items {
      if zone.Get(fmt.Sprintf("item-%d", i)).InBounds(msg) {
  ```
  This works correctly because `i` in zone IDs corresponds to the actual item index (not the visual row). Zone marks are registered at render time and track actual screen positions, so clicking row 3 on screen (which is item 5 due to scroll offset) would match `"item-5"`. The implementation is correct.

- [ ] **Missing context — `detail_render.go` does not import zone yet** — Step 3 adds the zone import to `detail_render.go`. The existing file imports are at lines 1-12. Executor needs to add the import.

- [ ] **Missing context — zone.Scan placement** — Step 5 says to wrap `App.View()` output with `zone.Scan()`. But `App.View()` may be called before the modal overlay is applied (Task 6.2 Step 3 adds modal overlay _after_ composing panels). The zone.Scan must happen as the very last operation in App.View(), after all overlays are applied. In the plan's Task 6.2 Step 3, `body = a.modal.overlayView(body)` comes after the footer is joined. The zone.Scan must happen after all modal overlays are applied. The execution order (5.3 before 6.2) means at the time 5.3 is implemented, there are no modals yet, so placement is correct. Just verify in Task 6.2 that the overlay insertion point is before `zone.Scan`.
  - **Fix:** Add to the Task 6.2 View integration: "Ensure modal overlay is applied before the existing `zone.Scan()` call."

- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None beyond the overlay ordering note.
- [x] **Success criteria completeness** — Clear.

---

## Group 6: Modal System

### Task 6.1: Create modal.go with ConfirmModal Component

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 1.3 (for the overlay package).
- [ ] **BLOCKER — `erikgeiser/bubbletea-overlay` still the dependency** — As noted in Task 1.3, this package does not exist. All modal tasks are blocked until the overlay package is resolved. `overlay.PlacePosition(lipgloss.Center, lipgloss.Center, m.View(), background)` does not compile against any real package.
  - **Fix:** Replace with the real package's API, or implement a simple manual centering helper using `lipgloss.Place()`.

- [x] **Hidden blockers (besides overlay)** — None. `lipgloss.RoundedBorder()` exists in lipgloss v1.1.1. `lipgloss.NewStyle().Border()` accepts a `lipgloss.Border` type. Confirmed.
- [x] **Cross-task conflicts** — None.
- [ ] **Success criteria** — Add: "Verify the overlay package API matches the `overlayView()` call signatures used."

---

### Task 6.2: Add Modal Field to App Struct and Route Modal Input

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 6.1 and 3.3.
- [ ] **Missing context — `focusModal` in `q` quit handler** — The existing `q` quit logic (lines 210-224 of current app.go) checks `a.screen == screenCategory`. After the refactor, the `q` key from `focusModal` state should probably not quit. The existing handler does `if a.screen == screenCategory { return a, tea.Quit }`. If a modal is active, the `q` key will fall through to this check. The plan's modal routing happens at the start of `KeyMsg` handling (Task 6.2 Step 2), _before_ the `q` quit check — so `q` is routed to the modal, not to quit. This is correct behavior assuming the `modal.Update()` does not handle `q`. Looking at `confirmModal.Update()` — it only handles `Enter`, `Esc`, `y`, `n`. So `q` in a modal goes to the modal, finds no match, returns unchanged, and the function returns `a, cmd` from the modal handler — quitting the modal routing early without reaching the quit handler. This is correct.
- [ ] **Missing context — `focus = focusContent` after modal dismiss may be wrong** — Step 2 sets `a.focus = focusContent` when a modal closes. But if the modal was opened from the sidebar (e.g., a future sidebar action), focus should return to sidebar. For now all modals in the plan are opened from the detail view (content area), so `focusContent` is correct. Add a note.
- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None.
- [x] **Success criteria completeness** — Clear.

---

### Task 6.3: Convert Install Flow to Modal

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 6.2.
- [ ] **Missing context — `doInstallChecked()` is a pointer receiver** — Task 6.3 Step 2 calls `a.detail.doInstallChecked()` from App.Update() (after modal confirms). `doInstallChecked()` in `detail.go` is defined as `func (m *detailModel) doInstallChecked()` — a pointer receiver. Since `a.detail` is a value field (not a pointer) in the App struct, the call `a.detail.doInstallChecked()` is valid in Go (Go takes the address automatically for addressable values). This is fine.
- [ ] **Missing context — `doInstallChecked()` uses `m.methodCursor`** — After the modal flow, the method picker (symlink vs copy) is now a separate concern. Task 6.3 converts only the initial install confirmation to a modal — the method picker becomes "the modal step." But looking at the code: `doInstallChecked()` reads `m.methodCursor` to decide symlink vs copy. If the modal system doesn't set `a.detail.methodCursor`, installs will always use symlink (cursor 0). The plan in Step 2 calls `a.detail.doInstallChecked()` directly without setting `methodCursor`. The plan mentions "method picker becomes a modal step" but doesn't implement it in Task 6.3.
  - **Fix:** Either clarify that the method picker is a separate follow-up task, or add a step to Task 6.3 that shows how `methodCursor` is set before calling `doInstallChecked()`. As-is, the modal confirms the install but silently always uses symlink.
- [ ] **Missing context — `doSavePrompt` method does not exist** — Task 6.5 Step 2 references `a.detail.doSavePrompt(a.saveModal.value)`. This method is not defined in the codebase. The existing flow is `doSave()` which uses `m.savePath` and `m.methodCursor`. The executor needs to create a new `doSavePrompt(filename string) (detailModel, tea.Cmd)` method on `detailModel`. This is not described in any task.
  - **Fix:** Add to Task 6.5: "Create a `doSavePrompt(value string) (detailModel, tea.Cmd)` method on `detailModel` that sets `m.savePath = value` and calls the existing save logic."
- [ ] **Missing context — `runAppScriptCmd()` does not exist** — Task 6.4 Step 4 calls `a.detail.runAppScriptCmd()` which is described as "a public method on detailModel that returns the tea.Cmd (extracts from `runAppScript()`)." This method does not exist. The existing `runAppScript()` is a pointer-receiver method returning `tea.Cmd`, and it is already effectively what `runAppScriptCmd()` would be. The executor needs to either rename `runAppScript()` to `runAppScriptCmd()` (breaking existing callers in `detail.go`) or create a wrapper.
  - **Fix:** Clarify that `runAppScriptCmd()` is an alias for the existing `runAppScript()` method. Either rename or add: `func (m *detailModel) runAppScriptCmd() tea.Cmd { return m.runAppScript() }`.

- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None.
- [ ] **Success criteria completeness** — Add: "After confirming the install modal, the install actually executes (not just the modal closes)."

---

### Task 6.4: Convert Uninstall, Save, Promote, and App Script to Modals

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 6.3.
- [ ] **Missing context — `doUninstallAll()` is a pointer receiver** — Same as Task 6.3 analysis. `a.detail.doUninstallAll()` is valid since `a.detail` is addressable. Fine.
- [ ] **Missing context — `loadScriptPreview` is described as a package-level helper** — Task 6.4 Step 3 introduces `loadScriptPreview(itemPath string) string` as a package-level function in `detail.go`. But `openModalMsg` is returned from `detail.Update()` as a `tea.Cmd`, and the body is populated by calling `loadScriptPreview(m.item.Path)` inside the Cmd closure. This reads a file (`os.ReadFile`) inside a Cmd that is expected to be a pure message (it runs synchronously). Since `openModalMsg` is returned as a `func() tea.Msg`, this is actually run on the Bubble Tea event loop — which is fine for file reads.
- [ ] **Missing context — Task 6.4 Step 5 references specific line numbers in detail_render.go** — "Delete lines 418-430" and "lines 400-414." These will have shifted after Task 4.1 rewrites `detail_render.go`. The executor should not use line numbers from the pre-Task 4.1 file as references after Task 4.1 runs.
  - **Fix:** Remove specific line numbers from Step 5 and instead describe the blocks by content: "Delete the `case actionUninstall:` and `case actionPromoteConfirm:` rendering blocks from `renderInstallTab()`" and "Delete the `case actionAppScriptConfirm:` block."

- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None.
- [x] **Success criteria completeness** — Clear.

---

### Task 6.5: Convert Save Prompt to Modal

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 6.4.
- [ ] **Missing context — `saveModal` field name conflicts with type name** — Step 2 adds:
  ```go
  saveModal saveModal
  ```
  In Go, a field name can share the name of its type within the same package. This is valid Go syntax and compiles. However, it can be confusing. No blocker, but a note would help.
- [ ] **Missing context — `doSavePrompt` doesn't exist (same issue as noted in 6.3)** — Step 2 calls `a.detail.doSavePrompt(a.saveModal.value)`. This method must be created as part of this task. The task description does not include a step to define `doSavePrompt`. The existing save flow in `detail.go` uses `m.savePath`, `m.methodCursor`, and `m.doSave()`. A new method is needed.
  - **Fix:** Add Step 6: "Add `func (m *detailModel) doSavePrompt(filename string) (detailModel, tea.Cmd)` to `detail.go`." The implementation should set `m.savePath = filename`, `m.confirmAction = actionSaveMethod`, and then call `m.doSave()` — or skip the method picker (since the modal is supposed to replace the inline picker entirely).
- [ ] **Missing context — the inline textinput in `detail.go` save flow is complex** — The current save flow uses `actionSavePath` → `actionSaveMethod` → `doSave()`. Task 6.5 Step 5 says to "delete the `actionSave` confirm path and inline textinput rendering." The `actionSave` constant does not exist — the actual constants are `actionSavePath` and `actionSaveMethod`. The executor needs to know to delete the `actionSavePath` handling block in `detail.Update()` (lines 178-194) and the `actionSaveMethod` handling in the `keys.Enter` case (lines 497-504), as well as the rendering blocks in `renderInstallTab()`.
  - **Fix:** Rename "actionSave" in Step 5 to "actionSavePath and actionSaveMethod" and list all blocks to delete.

- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None.
- [x] **Success criteria completeness** — Mostly clear. Add: "The `doSavePrompt` method is defined."

---

### Task 6.6: Implement Env Setup Modal Wizard

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 6.5.
- [ ] **Missing context — `env.AvailableTypes()` does not exist** — Task 6.6 Step 3 has:
  ```go
  envTypes := m.env.AvailableTypes()
  ```
  The `envSetupModel` struct (in `detail_env.go`) has no `AvailableTypes()` method. The struct contains `varNames []string`, `varIdx int`, `methodCursor int`, `value string`, and `input textinput.Model`. There is no concept of "environment types" (like provider names) in the existing implementation — the env setup is driven by `mcpConfig.Env` map keys.
  - **Fix:** Either define `AvailableTypes()` as a new method on `envSetupModel` that returns env type names (e.g., the providers from `mcpConfig`), or rethink what `envTypes` means in this context. The `envSetupModal` in modal.go uses `envTypes []string` as environment type names (claude, cursor, windsurf), but the existing `envSetupModel` is about env _variables_, not env _types_. This is an architectural mismatch between the new modal design and the existing implementation.
  - **Suggested fix:** Add a method to `detailModel` (not `envSetupModel`):
    ```go
    func (m detailModel) envProviderNames() []string {
        // return provider names that have env var requirements
    }
    ```
    Or simplify to pass `m.env.varNames` as the list.

- [ ] **Missing context — `env.Refresh()` does not exist** — Task 6.6 Step 2 calls `a.detail.env.Refresh()`. The `envSetupModel` has no `Refresh()` method. The concept of refreshing env status likely means re-reading which env vars are set from the OS environment. No such method exists.
  - **Fix:** Add `func (m *envSetupModel) Refresh()` to `detail_env.go`, or simply re-call `m.unsetEnvVarNames()` in the App handler after modal closes. The simplest fix: replace `a.detail.env.Refresh()` with direct logic, e.g., updating the env display by re-checking `os.LookupEnv`.

- [ ] **Missing context — `buildEnvInputs` is a placeholder** — The `buildEnvInputs()` function in modal.go is explicitly noted as a placeholder. This means Task 6.6 as written does not produce a working env modal — it produces a skeleton. If this is intentional, the success criteria should say "Step 1 of env modal, full field list TBD." As written, the success criteria say "Step 2: Configure paths/settings (textinput for each required field)" — which implies the fields are real.
  - **Fix:** Either implement `buildEnvInputs` with real fields from the catalog env spec, or change the success criteria to "skeleton compiles; field population is a follow-up task."

- [x] **Hidden blockers** — None beyond the API issues.
- [x] **Cross-task conflicts** — None.
- [ ] **Success criteria completeness** — Missing: "`env.AvailableTypes()` and `env.Refresh()` methods are defined."

---

### Task 6.7: Add zone.Mark() to Detail Action Buttons

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 5.3 and 6.4.
- [ ] **Missing context — action bar location** — The plan says "wherever the action bar `[i]nstall  [u]ninstall  [c]opy  [s]ave` is rendered." Looking at the actual `detail_render.go`, there is no unified action bar string. The help hints appear in `renderHelp()` at lines 497-543, not as zone-markable buttons in the content area. The only place install/uninstall/etc. keys are shown is in `renderHelp()`. Zone marks on help bar text in `renderHelp()` would require zone to be imported in `detail_render.go` and the help text format to change.
  - **Fix:** Clarify where the action buttons are rendered. Either: (a) add a visible action button bar to `renderInstallTab()` as new content (not just help bar text), or (b) wrap the help bar hints in `renderHelp()` with zone marks. Option (a) requires adding a new rendered element; option (b) wraps existing text. The plan assumes an action bar exists — it does not in the current code.

- [ ] **Missing context — `btnKeys` variable is declared but unused** — Step 2 defines `btnKeys` and then immediately does `_ = btnKeys`. This will compile (blank identifier), but it is dead code that the executor might try to clean up, changing the logic.

- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None.
- [ ] **Success criteria completeness** — Add: "Confirm that action buttons are rendered as visible clickable elements in the content area (not just help bar hints)."

---

### Task 1.4: Add WCAG AA Contrast Verification Test

**Checks:**

- [x] **Implicit dependencies** — Correctly depends on Task 1.2.
- [ ] **Missing context — `mutedColor dark` may fail WCAG AA** — The test checks `"mutedColor dark"` with `fg="#A8A29E"` on `bg="#18181B"`. Calculating: `#A8A29E` (168, 162, 158) against `#18181B` (24, 24, 27). The approximate contrast ratio is ~4.9:1, which passes. However, `selectedBgColor dark` `fg="#6EE7B7"` on `bg="#1A3A2A"` is also in the test as `"selected mint on dark bg"`. That ratio is approximately: mint green on dark green — likely below 4.5:1 since both have green luminance. This specific check may fail.
  - **Verify:** Calculate `contrastRatio("#6EE7B7", "#1A3A2A")` before committing the test. If it fails, either adjust `selectedBgColor` dark value or lower `minRatio` for that specific check.

- [ ] **Missing context — `math` package import required** — The test uses `math.Pow`. The test file (`styles_test.go`) does not currently import `math`. The executor must add `"math"` and `"strconv"` imports to the test file.

- [ ] **Missing context — `strings` package import required** — `relativeLuminance` uses `strings.TrimPrefix`. Again, not currently imported in `styles_test.go`.

- [x] **Hidden blockers** — None.
- [x] **Cross-task conflicts** — None.
- [ ] **Success criteria completeness** — Add: "`math`, `strconv`, and `strings` packages are imported in the test file."

---

## Cross-Cutting Issues

### Issue A: Tab Key Conflict in Detail View

Task 3.3 Step 2 adds Tab focus-switching before screen-specific handling. In `screenDetail`, Tab is _currently_ used to switch between Overview/Files/Install tabs (handled in `detail.Update()`). The new code routes Tab to focus-switching when `!a.detail.HasTextInput()`. This means Tab can no longer switch detail tabs via keyboard when `focusContent` — the Tab key will shift focus to the sidebar instead.

The plan says "detail has no text input" as the guard, but tab-switching in detail view (Overview/Files/Install) doesn't involve text input. After this change, the user must use `1`, `2`, `3` keys to switch detail tabs, or click them with mouse. The `keys.Tab` conflict is not acknowledged in the plan.

- **Fix:** Either use Shift+Tab for focus toggle (already in the plan: `keys.ShiftTab`), or only allow Tab to switch focus when `a.screen != screenDetail`. The current plan's guard `if !a.detail.HasTextInput()` is insufficient — it should also check `a.screen != screenDetail` for sidebar/content focus switching, or only use ShiftTab for panel switching.

### Issue B: Search Handler in App.Update() References `a.category.selectedType()`

Confirmed in source (lines 262, 280 of app.go). After Task 3.4 removes `category`, two places in the search active block reference `a.category.selectedType()`. Neither Task 3.3 nor 3.4 includes steps to fix these. This is a compile-time blocker for Task 3.4.

### Issue C: Execution Order — Tasks 1.4 is Placed Last in Group 1

Task 1.4 appears at the end of the document (after Group 6), not in the Group 1 section. The execution order diagram correctly places it after Task 1.2. The section heading placement in the document is confusing — the executor might execute it too late.

- **Fix:** Move Task 1.4 into the Group 1 section body, after Task 1.2.

---

## Consolidated Fix List

| # | Task | Fix Needed |
|---|------|------------|
| 1 | 1.3  | BLOCKER: Replace `erikgeiser/bubbletea-overlay` (does not exist) with `rmhubbert/bubbletea-overlay` and update all `overlay.PlacePosition()` calls to `overlay.Composite()` |
| 2 | 3.3  | BLOCKER: Add `Right key.Binding` to `keys.go` before using `keys.Right` in sidebar routing |
| 3 | 3.3  | HIGH: Update search active block to replace `a.category.selectedType()` with `a.sidebar.selectedType()` |
| 4 | 3.3  | HIGH: Add `promote` import to `app.go` (currently only in `detail.go`) |
| 5 | 3.3/3.4 | HIGH: Update `updateCheckMsg` handler to set `a.sidebar.remoteVersion/updateAvailable/commitsBehind` |
| 6 | 3.4  | HIGH: Decide where `importDoneMsg` success message goes (no `message` field on `sidebarModel`) |
| 7 | 6.3/6.5 | HIGH: `doSavePrompt()` and `runAppScriptCmd()` methods do not exist; must be created |
| 8 | 6.6  | HIGH: `env.AvailableTypes()` and `env.Refresh()` methods do not exist on `envSetupModel` |
| 9 | 3.2  | MEDIUM: `tooSmall` threshold inconsistency between View (60) and Update (still 40 until 3.3) |
| 10 | 3.3  | MEDIUM: Tab key conflict — Tab switches detail tabs today; new code intercepts it for panel focus |
| 11 | 2.1  | MEDIUM: `sidebarWidth` of 18 with `BorderRight(true)` renders as 19 chars; adjust width calculation |
| 12 | 4.1  | MEDIUM: Add `detail.go` to the task's Files list (clampScroll is in detail.go, not detail_render.go) |
| 13 | 5.2  | MEDIUM: Add nil/zero check before calling `zone.Get().InBounds()` to prevent panic |
| 14 | 6.4  | MEDIUM: Remove specific line numbers (418-430, 400-414) — they shift after Task 4.1 rewrites the file |
| 15 | 6.7  | MEDIUM: No unified action button bar exists in current code; must be created, not just wrapped |
| 16 | 1.4  | LOW: Add `math`, `strconv`, `strings` imports to test file |
| 17 | 1.4  | LOW: Verify `contrastRatio("#6EE7B7", "#1A3A2A")` actually passes 4.5:1 before committing |
| 18 | 3.3  | LOW: `a.updater.width = a.width` in autoUpdate path needs `- sidebarWidth` |
| 19 | 5.3  | LOW: Note that zone.Scan in App.View must remain as the last operation, after all modal overlays |

---

*Analysis complete. The two blockers (non-existent overlay package and missing `keys.Right` binding) must be resolved before any implementation begins. The five HIGH issues will cause compile errors during the later group tasks if not addressed.*
