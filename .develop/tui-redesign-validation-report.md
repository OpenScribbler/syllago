# TUI Redesign — Design ↔ Plan Parity Validation Report

**Date:** 2026-02-17
**Design doc:** `docs/plans/2026-02-17-tui-redesign-design.md`
**Implementation plan:** `docs/plans/2026-02-17-tui-redesign-implementation.md`
**Attempt:** 1/5
**Result:** ❌ Gaps found and fixed — re-validated after fixes

---

## Validation Report

### ✅ Covered (12 requirements → 18 tasks)

- **Romanesco semantic colors** (mint primary, viola accent, stone muted, success green, danger red, warning amber) → Task 1.1 Step 1 (exact hex values in `styles.go`)
- **Panel/layout colors** (border `#D4D4D8/#3F3F46`, selectedBg `#D1FAE5/#1A3A2A`, modalBg `#F4F4F5/#27272A`, modalBorder = accent) → Task 1.1 Step 2
- **`secondaryColor` → `accentColor` rename** → Task 1.1 Step 3, Task 1.2 Step 1
- **Sidebar + content layout** (sidebar ~16 chars, content swaps items/detail) → Task 2.1 (`sidebarWidth = 18` including border), Task 3.2 (`lipgloss.JoinHorizontal`)
- **Sidebar items**: content types with counts, separator, utility items (My Tools ◆/◇, Import, Update, Settings) → Task 2.1 full `sidebarModel.View()` implementation
- **Detail view: metadata above horizontal separator, content below** → Task 4.1 (`renderContentSplit()`, pinned header, `─────` separator, scrollable body)
- **Tab bar in detail header** (Overview, Files, Install) → Task 4.1 Step 1 (`renderTabBar()` in header line), Task 5.3 Step 3 (zone marks)
- **Modal: Install confirmation** → Task 6.3
- **Modal: Uninstall confirmation** → Task 6.4 Step 1
- **Modal: Promote confirmation** → Task 6.4 Step 2
- **Modal: App script confirmation** → Task 6.4 Step 3
- **Focus management** (Tab/Shift+Tab between sidebar/content, `focusTarget` enum, modal captures all input) → Task 3.1, Task 3.3 Step 2, Task 6.2 Step 2
- **Footer: breadcrumb + context-sensitive help keys** → Task 3.2 (`renderFooter()`, `breadcrumb()`)
- **New dependency: bubblezone** → Task 1.3, Task 5.1 (`zone.NewGlobal()`, `WithMouseCellMotion()`)
- **New dependency: bubbletea-overlay** → Task 1.3, Task 6.1 (`overlay.PlacePosition()`)
- **Mouse: sidebar item clicks** → Task 5.2 (`zone.Mark("sidebar-N", ...)`, MouseMsg handler)
- **Mouse: item list clicks** → Task 5.3 Steps 1–2 (`zone.Mark("item-N", ...)`)
- **Mouse: tab clicks** → Task 5.3 Steps 3–4 (`zone.Mark("tab-N", ...)`)
- **`zone.Scan()` wrapping** → Task 5.3 Step 5
- **Responsive layout** (sidebar collapses/warning below ~60 cols) → Task 3.2 `tooSmall` guard, Task 3.3 Step 1 WindowSizeMsg handler
- **No orphan tasks** — all 18 tasks trace to design requirements
- **No TBD/TODO/mock data/vague descriptions** — all tasks have concrete code, exact file paths, and line references

---

### ❌ Gaps Found (4 issues — all fixed)

**1. Missing modal: Env setup modal wizard**
- **Type:** Missing task
- **Design source:** Section 3 component hierarchy (`EnvSetupModal`), Section 5 Flow 4 (3-step wizard: select env type → configure → confirm), Section 6 action table ("Environment setup | Modal wizard | Multi-step flow")
- **Plan gap:** No task for `EnvSetupModal`. The `modalPurpose` enum defined in Task 6.3 omitted `modalEnvSetup`. Tasks 6.1–6.4 covered the other four modals but skipped env setup entirely.
- **Fix:** Added **Task 6.6** implementing `envSetupModal` struct in `modal.go` with 3-step wizard (step 1: select type with j/k nav, step 2: configure with textinput fields, step 3: confirm/apply). Added routing in `App.Update()` via `openEnvModalMsg`, overlay in `App.View()`, and key binding in `detail.Update()`.

**2. Design deviation: Save prompt not implemented as modal**
- **Type:** Requirement deviation
- **Design source:** Section 5 Flow 2 ("Save prompt modal appears with text input"), Section 6 action table ("Save prompt | Modal | Text input for filename")
- **Plan gap:** Task 6.4 explicitly deferred save to inline textinput, citing complexity of embedding textinput in a modal. This contradicts the design specification.
- **Fix:** Added **Task 6.5** implementing `saveModal` struct in `modal.go` with an embedded `bubbles/textinput`. Emits `openSaveModalMsg` from `detail.Update()`, handled in `App.Update()`. Inline save flow is removed.

**3. Missing mouse zones: Detail action buttons**
- **Type:** Missing task
- **Design source:** Section 5 Flow 6 ("Click action button in detail → triggers that action (modal if needed)")
- **Plan gap:** Tasks 5.1–5.3 covered sidebar, item list, and tab zones. No task added `zone.Mark()` to the `[i]nstall  [u]ninstall  [c]opy  [s]ave` action bar in the detail view.
- **Fix:** Added **Task 6.7** wrapping each action button in `renderInstallTab()` / action bar with `zone.Mark("detail-btn-{install,uninstall,copy,save}", ...)`. MouseMsg handler in `App.Update()` synthesizes the corresponding keypress for each clicked button.

**4. Missing verification: WCAG AA contrast ratios**
- **Type:** Missing verification task
- **Design source:** Section 6 ("All colors pass WCAG AA (4.5:1 minimum for normal text)") with documented ratios for all 7 color roles
- **Plan gap:** Task 1.1 set the correct hex values but no task verified the contrast ratios programmatically. Design compliance was assumed, not checked.
- **Fix:** Added **Task 1.4** adding `TestColorsPassWCAGAA` to `styles_test.go`. The test implements a `contrastRatio()` helper using the WCAG relative luminance formula and checks all 14 color/background combinations (each semantic color on both light and dark terminal backgrounds, plus the selected-item mint-on-mint-bg combination).

---

### Changes to Implementation Plan

The following tasks were added to `/home/hhewett/.local/src/romanesco/docs/plans/2026-02-17-tui-redesign-implementation.md`:

| Task | Group | Description |
|------|-------|-------------|
| Task 1.4 | Group 1: Foundation | WCAG AA contrast verification test in `styles_test.go` |
| Task 6.5 | Group 6: Modal System | Save prompt modal with embedded textinput |
| Task 6.6 | Group 6: Modal System | Env setup multi-step modal wizard |
| Task 6.7 | Group 6: Modal System | `zone.Mark()` on detail action buttons + click handling |

The file summary table and execution order diagram were updated to reflect all new tasks.

---

### Updated Execution Order (delta)

```
...existing tasks unchanged...
Task 6.4 (uninstall/promote/app script modals) ←── after 6.3
Task 6.5 (save prompt modal)   ←── after 6.4
Task 6.6 (env setup modal wizard) ←── after 6.5
Task 6.7 (detail action button zones) ←── after 5.3 + 6.4
Task 1.4 (WCAG AA contrast test) ←── after 1.2
```

---

### Action Required

All gaps have been fixed directly in the implementation plan. The plan now has full design↔plan parity.

**Proceed to Beads creation.**

---

## ✅ PASSED

After the 4 fixes above, all 12 design requirements trace to implementing tasks, no orphan tasks exist, no TBDs remain, and all architecture decisions from the design are reflected in the task structure.

**Total tasks:** 22 (18 original + 4 added)
**Total files modified/created by plan:** 13
