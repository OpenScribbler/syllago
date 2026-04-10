# TUI Golden File Testing - Implementation Plan

**Goal:** Add golden file visual regression testing for the syllago TUI so `make test` catches layout regressions and `git diff testdata/` provides the review workflow.

**Architecture:** Three new files in `cli/internal/tui/` — a shared infrastructure file with the `requireGolden` helper and `-update` flag, a full-app snapshot test file using `testApp()`, and a component-isolated snapshot test file constructing sub-models directly. Golden files land in `cli/internal/tui/testdata/`. The `.gitattributes` at the repo root gets an entry to mark golden files as text (no binary corruption).

**Tech Stack:** Go test package, `charmbracelet/x/ansi` (already an indirect dep) for ANSI stripping, custom `requireGolden` helper (no external golden library), `testApp()` + direct model construction for deterministic rendering.

**Design Doc:** `/home/hhewett/.local/src/syllago/docs/plans/2026-02-25-tui-golden-testing-design.md`

---

## Task 1: Golden Infrastructure File

**Files:**
- Create: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_test.go`

**Depends on:** Nothing — this is the foundation.

**Success Criteria:**
- [ ] `go build ./cli/internal/tui/...` passes (no compile error)
- [ ] `go test -run TestGoldenSmoke ./cli/internal/tui/...` passes
- [ ] `-update` flag is registered and recognized

---

### Step 1: Write the infrastructure file

```go
// cli/internal/tui/golden_test.go
package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// update is registered once here; all golden tests check it.
var update = flag.Bool("update", false, "update golden files")

// stripANSI removes ANSI escape sequences from s.
// NO_COLOR=1 (set in init()) handles most cases; this is belt-and-suspenders.
func stripANSI(s string) string {
	return ansi.Strip(s)
}

// requireGolden compares actual against a golden file at testdata/<name>.golden.
// Pass -update to regenerate: go test -update ./cli/internal/tui/...
func requireGolden(t *testing.T, name string, actual string) {
	t.Helper()
	goldenPath := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("create testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("write golden file %s: %v", goldenPath, err)
		}
		t.Logf("updated golden: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file %s not found — run with -update to create it: %v", goldenPath, err)
	}

	if string(expected) != actual {
		t.Errorf("golden file mismatch: %s\n\nRun: go test -update ./cli/internal/tui/...\n\nDiff:\n%s",
			goldenPath, diffStrings(string(expected), actual))
	}
}

// diffStrings produces a simple line-by-line diff between want and got.
func diffStrings(want, got string) string {
	wLines := strings.Split(want, "\n")
	gLines := strings.Split(got, "\n")

	var sb strings.Builder
	max := len(wLines)
	if len(gLines) > max {
		max = len(gLines)
	}
	for i := 0; i < max; i++ {
		var w, g string
		if i < len(wLines) {
			w = wLines[i]
		}
		if i < len(gLines) {
			g = gLines[i]
		}
		if w != g {
			fmt.Fprintf(&sb, "line %d:\n  want: %q\n  got:  %q\n", i+1, w, g)
		}
	}
	return sb.String()
}

// TestGoldenSmoke verifies the golden infrastructure compiles and runs.
func TestGoldenSmoke(t *testing.T) {
	// Just verify the flag is registered and the helpers compile.
	_ = *update
	result := stripANSI("\x1b[31mred\x1b[0m")
	if result != "red" {
		t.Fatalf("stripANSI: got %q, want %q", result, "red")
	}
}
```

### Step 2: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenSmoke ./internal/tui/...
```

Expected: `PASS` — the smoke test verifies compilation and ANSI stripping.

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_test.go
git commit -m "test: add golden file infrastructure (flag, requireGolden, diffStrings)"
```

---

## Task 2: .gitattributes Entry for Golden Files

**Files:**
- Modify: `/home/hhewett/.local/src/syllago/.gitattributes`

**Depends on:** Nothing (can run in parallel with Task 1, committing after)

**Success Criteria:**
- [ ] `.gitattributes` contains the golden file pattern
- [ ] `git check-attr diff cli/internal/tui/testdata/test.golden` shows the attribute is set

---

### Step 1: Add the golden file attribute

The existing `.gitattributes` only has the beads merge driver entry. Add the golden file pattern below it. We use `text eol=lf` rather than `binary` so `git diff` still shows inline diffs — the design doc noted that `binary` prevents inline diffs, which defeats the purpose of the review workflow.

Edit `/home/hhewett/.local/src/syllago/.gitattributes` to add:

```
# Use bd merge for beads JSONL files
.beads/issues.jsonl merge=beads

# Golden test files: normalize line endings, keep text diffs in git
cli/internal/tui/testdata/*.golden text eol=lf
```

### Step 2: Verify the attribute is recognized

```bash
cd /home/hhewett/.local/src/syllago && git check-attr eol cli/internal/tui/testdata/fake.golden
```

Expected: `cli/internal/tui/testdata/fake.golden: eol: lf`

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add .gitattributes
git commit -m "chore: add gitattributes for golden test files (text eol=lf)"
```

---

## Task 3: Full-App Snapshot File (Skeleton + Category Welcome)

**Files:**
- Create: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-category-welcome.golden`

**Depends on:** Task 1 (golden infrastructure)

**Success Criteria:**
- [ ] `go test -run TestGoldenFullApp_CategoryWelcome -update ./cli/internal/tui/...` creates the golden file
- [ ] `go test -run TestGoldenFullApp_CategoryWelcome ./cli/internal/tui/...` passes (no diff)
- [ ] Golden file exists and contains a plain-text rendering with "syllago", "AI Tools", "Skills", "Agents"

---

### Step 1: Write the failing test

```go
// cli/internal/tui/golden_fullapp_test.go
package tui

import "testing"

// snapshotApp renders the full app view, strips ANSI, and returns the result.
func snapshotApp(t *testing.T, app App) string {
	t.Helper()
	return stripANSI(app.View())
}

func TestGoldenFullApp_CategoryWelcome(t *testing.T) {
	app := testApp(t)
	// testApp starts on screenCategory with focusSidebar — no navigation needed.
	requireGolden(t, "fullapp-category-welcome", snapshotApp(t, app))
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenFullApp_CategoryWelcome ./internal/tui/...
```

Expected: FAIL — `golden file testdata/fullapp-category-welcome.golden not found — run with -update to create it`

### Step 3: Generate the golden file

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenFullApp_CategoryWelcome -update ./internal/tui/...
```

Expected: `PASS` with log line `updated golden: testdata/fullapp-category-welcome.golden`

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenFullApp_CategoryWelcome ./internal/tui/...
```

Expected: `PASS`

### Step 5: Verify golden file content

```bash
head -5 /home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-category-welcome.golden
```

Expected: first lines show the sidebar with "syllago" header and content area with "AI Tools", no ANSI escape codes.

### Step 6: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_fullapp_test.go cli/internal/tui/testdata/fullapp-category-welcome.golden
git commit -m "test: add golden snapshot for category welcome screen"
```

---

## Task 4: Full-App Snapshots — Items and Detail Overview

**Files:**
- Modify: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-items-skills.golden`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-detail-overview.golden`

**Depends on:** Task 3 (golden_fullapp_test.go exists)

**Success Criteria:**
- [ ] Both tests pass after `-update` run
- [ ] `fullapp-items-skills.golden` contains "alpha-skill", "beta-skill"
- [ ] `fullapp-detail-overview.golden` contains "Readme body for alpha-skill"

---

### Step 1: Write the failing tests

Append to `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`:

```go
func TestGoldenFullApp_ItemsSkills(t *testing.T) {
	app := testApp(t)
	// Enter on Skills (first sidebar item, cursor=0) → items screen
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	requireGolden(t, "fullapp-items-skills", snapshotApp(t, app))
}

func TestGoldenFullApp_DetailOverview(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// navigateToDetail lands on the first skill's overview tab
	requireGolden(t, "fullapp-detail-overview", snapshotApp(t, app))
}
```

Note: `navigateToDetail` is defined in `detail_test.go` and already available in the package.

### Step 2: Run tests to verify they fail

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_ItemsSkills|TestGoldenFullApp_DetailOverview' ./internal/tui/...
```

Expected: FAIL — both golden files not found.

### Step 3: Generate golden files

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_ItemsSkills|TestGoldenFullApp_DetailOverview' -update ./internal/tui/...
```

Expected: `PASS` with two `updated golden:` log lines.

### Step 4: Run tests to verify they pass

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_ItemsSkills|TestGoldenFullApp_DetailOverview' ./internal/tui/...
```

Expected: `PASS`

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_fullapp_test.go \
        cli/internal/tui/testdata/fullapp-items-skills.golden \
        cli/internal/tui/testdata/fullapp-detail-overview.golden
git commit -m "test: add golden snapshots for items list and detail overview"
```

---

## Task 5: Full-App Snapshots — Detail Files and Detail Install Tabs

**Files:**
- Modify: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-detail-files.golden`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-detail-install.golden`

**Depends on:** Task 4

**Success Criteria:**
- [ ] Both tests pass after `-update` run
- [ ] `fullapp-detail-files.golden` contains "SKILL.md", "README.md"
- [ ] `fullapp-detail-install.golden` contains "Install" and "Claude Code"

---

### Step 1: Write the failing tests

Append to `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`:

```go
func TestGoldenFullApp_DetailFiles(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Switch to Files tab (key "2")
	m, _ := app.Update(keyRune('2'))
	app = m.(App)
	requireGolden(t, "fullapp-detail-files", snapshotApp(t, app))
}

func TestGoldenFullApp_DetailInstall(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Switch to Install tab (key "3")
	m, _ := app.Update(keyRune('3'))
	app = m.(App)
	requireGolden(t, "fullapp-detail-install", snapshotApp(t, app))
}
```

### Step 2: Run tests to verify they fail

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_DetailFiles|TestGoldenFullApp_DetailInstall' ./internal/tui/...
```

Expected: FAIL — both golden files not found.

### Step 3: Generate golden files

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_DetailFiles|TestGoldenFullApp_DetailInstall' -update ./internal/tui/...
```

Expected: `PASS` with two `updated golden:` log lines.

### Step 4: Run tests to verify they pass

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_DetailFiles|TestGoldenFullApp_DetailInstall' ./internal/tui/...
```

Expected: `PASS`

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_fullapp_test.go \
        cli/internal/tui/testdata/fullapp-detail-files.golden \
        cli/internal/tui/testdata/fullapp-detail-install.golden
git commit -m "test: add golden snapshots for detail files and install tabs"
```

---

## Task 6: Full-App Snapshots — Search Results and Modal

**Files:**
- Modify: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-search-results.golden`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-modal.golden`

**Depends on:** Task 5

**Success Criteria:**
- [ ] Both tests pass after `-update` run
- [ ] `fullapp-search-results.golden` contains "alpha-skill" and not "test-agent"
- [ ] `fullapp-modal.golden` contains "Confirm", "Enter/y", "Esc/n"

---

### Step 1: Write the failing tests

Append to `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`:

```go
func TestGoldenFullApp_SearchResults(t *testing.T) {
	app := testApp(t)
	// Activate search
	m, _ := app.Update(keyRune('/'))
	app = m.(App)
	// Type "alpha" one rune at a time
	for _, r := range "alpha" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}
	// Submit search → items screen with filtered results
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	requireGolden(t, "fullapp-search-results", snapshotApp(t, app))
}

func TestGoldenFullApp_Modal(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Force-inject an openModalMsg directly rather than relying on install state.
	// This guarantees the modal is always active for the snapshot — no t.Skip.
	m, _ := app.Update(openModalMsg{
		title:   "Confirm Uninstall",
		message: "Remove alpha-skill from all providers?",
	})
	app = m.(App)
	if !app.modal.active {
		t.Fatalf("modal was not activated after openModalMsg — check openModalMsg handling in App.Update")
	}
	requireGolden(t, "fullapp-modal", snapshotApp(t, app))
}
```

### Step 2: Run tests to verify they fail

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_SearchResults|TestGoldenFullApp_Modal' ./internal/tui/...
```

Expected: FAIL — both golden files not found.

### Step 3: Generate golden files

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_SearchResults|TestGoldenFullApp_Modal' -update ./internal/tui/...
```

Expected: `PASS` — both golden files created (search results and modal).

### Step 4: Run tests to verify they pass

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenFullApp_SearchResults|TestGoldenFullApp_Modal' ./internal/tui/...
```

Expected: `PASS` (both tests pass, no skips)

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_fullapp_test.go \
        cli/internal/tui/testdata/fullapp-search-results.golden \
        cli/internal/tui/testdata/fullapp-modal.golden
git commit -m "test: add golden snapshots for search results and modal"
```

---

## Task 7: Full-App Snapshot — Settings Screen

**Files:**
- Modify: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/fullapp-settings.golden`

**Depends on:** Task 6

**Success Criteria:**
- [ ] Test passes after `-update` run
- [ ] `fullapp-settings.golden` contains "Settings", "Auto-update", "Providers"

---

### Step 1: Write the failing test

Append to `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_fullapp_test.go`:

```go
func TestGoldenFullApp_Settings(t *testing.T) {
	app := testApp(t)
	// Navigate sidebar to Settings: len(AllContentTypes()) + 3 presses down.
	// AllContentTypes() has 8 types, then My Tools (+1), Import (+2), Update (+3), Settings (+4).
	// Index of Settings = 8 + 3 = 11 (0-based), so 11 down presses from cursor=0.
	nTypes := 8 // catalog.AllContentTypes() length
	app = pressN(app, keyDown, nTypes+3)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenSettings)
	requireGolden(t, "fullapp-settings", snapshotApp(t, app))
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenFullApp_Settings ./internal/tui/...
```

Expected: FAIL — golden file not found.

### Step 3: Generate golden file

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenFullApp_Settings -update ./internal/tui/...
```

Expected: `PASS` with `updated golden: testdata/fullapp-settings.golden`

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenFullApp_Settings ./internal/tui/...
```

Expected: `PASS`

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_fullapp_test.go \
        cli/internal/tui/testdata/fullapp-settings.golden
git commit -m "test: add golden snapshot for settings screen"
```

---

## Task 8: Component Snapshot File (Skeleton + Sidebar)

**Files:**
- Create: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_components_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/component-sidebar.golden`

**Depends on:** Task 1 (golden infrastructure)

**Success Criteria:**
- [ ] Test passes after `-update` run
- [ ] `component-sidebar.golden` contains "syllago", "AI Tools", "Skills", "Configuration", "Import"
- [ ] Golden file is shorter than full-app golden (it's sidebar only, no content panel)

---

### Step 1: Write the failing test

```go
// cli/internal/tui/golden_components_test.go
package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestGoldenComponent_Sidebar(t *testing.T) {
	cat := testCatalog(t)
	m := newSidebarModel(cat, "1.0.0", 0)
	m.width = 18  // sidebar column width used in testApp (sidebarWidth constant)
	m.height = 30
	m.focused = true
	requireGolden(t, "component-sidebar", stripANSI(m.View()))
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenComponent_Sidebar ./internal/tui/...
```

Expected: FAIL — `golden file testdata/component-sidebar.golden not found`

### Step 3: Generate golden file

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenComponent_Sidebar -update ./internal/tui/...
```

Expected: `PASS` with `updated golden: testdata/component-sidebar.golden`

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenComponent_Sidebar ./internal/tui/...
```

Expected: `PASS`

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_components_test.go \
        cli/internal/tui/testdata/component-sidebar.golden
git commit -m "test: add golden component snapshot file and sidebar snapshot"
```

---

## Task 9: Component Snapshots — Items List and Detail Tabs

**Files:**
- Modify: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_components_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/component-items.golden`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/component-detail-tabs.golden`

**Depends on:** Task 8

**Success Criteria:**
- [ ] Both tests pass after `-update` run
- [ ] `component-items.golden` contains "alpha-skill", "beta-skill", "A helpful skill"
- [ ] `component-detail-tabs.golden` contains "Overview", "Files", "Install" tab labels

---

### Step 1: Write the failing tests

Append to `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_components_test.go`:

```go
func TestGoldenComponent_Items(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)
	items := cat.ByType(catalog.Skills)
	m := newItemsModel(catalog.Skills, items, providers, cat.RepoRoot)
	m.width = 62 // content panel width = 80 - sidebarWidth(18) = 62
	m.height = 28
	requireGolden(t, "component-items", stripANSI(m.View()))
}

func TestGoldenComponent_DetailTabs(t *testing.T) {
	// Navigate to detail to get a fully-initialized detailModel with tab bar rendered.
	app := navigateToDetail(t, catalog.Skills)
	// Snapshot just the detail view (content panel only)
	requireGolden(t, "component-detail-tabs", stripANSI(app.detail.View()))
}
```

### Step 2: Run tests to verify they fail

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenComponent_Items|TestGoldenComponent_DetailTabs' ./internal/tui/...
```

Expected: FAIL — both golden files not found.

### Step 3: Generate golden files

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenComponent_Items|TestGoldenComponent_DetailTabs' -update ./internal/tui/...
```

Expected: `PASS` with two `updated golden:` log lines.

### Step 4: Run tests to verify they pass

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenComponent_Items|TestGoldenComponent_DetailTabs' ./internal/tui/...
```

Expected: `PASS`

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_components_test.go \
        cli/internal/tui/testdata/component-items.golden \
        cli/internal/tui/testdata/component-detail-tabs.golden
git commit -m "test: add golden snapshots for items list and detail tab bar components"
```

---

## Task 10: Component Snapshots — Modal and Help Overlay

**Files:**
- Modify: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_components_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/component-modal.golden`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/component-help.golden`

**Depends on:** Task 9

**Success Criteria:**
- [ ] Both tests pass after `-update` run
- [ ] `component-modal.golden` contains "Confirm", "[Enter/y] Confirm", "[Esc/n] Cancel"
- [ ] `component-help.golden` contains "Keyboard Shortcuts", "ctrl+c", "esc"

---

### Step 1: Write the failing tests

Append to `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_components_test.go`:

```go
func TestGoldenComponent_Modal(t *testing.T) {
	m := newConfirmModal("Confirm Uninstall", "Remove alpha-skill from all providers?")
	requireGolden(t, "component-modal", stripANSI(m.View()))
}

func TestGoldenComponent_HelpOverlay(t *testing.T) {
	m := helpOverlayModel{active: true}
	// Use screenCategory context for the help overlay
	requireGolden(t, "component-help", stripANSI(m.View(screenCategory)))
}
```

### Step 2: Run tests to verify they fail

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenComponent_Modal|TestGoldenComponent_HelpOverlay' ./internal/tui/...
```

Expected: FAIL — both golden files not found.

### Step 3: Generate golden files

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenComponent_Modal|TestGoldenComponent_HelpOverlay' -update ./internal/tui/...
```

Expected: `PASS` with two `updated golden:` log lines.

### Step 4: Run tests to verify they pass

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run 'TestGoldenComponent_Modal|TestGoldenComponent_HelpOverlay' ./internal/tui/...
```

Expected: `PASS`

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_components_test.go \
        cli/internal/tui/testdata/component-modal.golden \
        cli/internal/tui/testdata/component-help.golden
git commit -m "test: add golden snapshots for confirm modal and help overlay components"
```

---

## Task 11: Component Snapshot — File Browser

**Files:**
- Modify: `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_components_test.go`
- Create (generated): `/home/hhewett/.local/src/syllago/cli/internal/tui/testdata/component-filebrowser.golden`

**Depends on:** Task 10

**Success Criteria:**
- [ ] Test passes after `-update` run
- [ ] `component-filebrowser.golden` contains directory entries (either real files from t.TempDir or an empty/error state message)

---

### Step 1: Write the failing test

Append to `/home/hhewett/.local/src/syllago/cli/internal/tui/golden_components_test.go`:

```go
func TestGoldenComponent_FileBrowser(t *testing.T) {
	// Use the test catalog's temp dir so we have real files to browse
	cat := testCatalog(t)
	fb := newFileBrowser(cat.RepoRoot, catalog.Skills)
	fb.width = 62
	fb.height = 28
	requireGolden(t, "component-filebrowser", stripANSI(fb.View()))
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenComponent_FileBrowser ./internal/tui/...
```

Expected: FAIL — `golden file testdata/component-filebrowser.golden not found`

### Step 3: Generate golden file

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenComponent_FileBrowser -update ./internal/tui/...
```

Expected: `PASS` with `updated golden: testdata/component-filebrowser.golden`

**Note:** The file browser renders the temp directory path which changes each test run. If the golden file contains the absolute temp path it will fail on subsequent runs. Check the generated golden file — if it contains the path, we need to handle this. See Step 3a below.

### Step 3a: If path is in the golden file, adjust the approach

If `component-filebrowser.golden` contains the temp dir path (e.g., `/tmp/TestGoldenComponent...`), the test will be non-deterministic. In that case, replace the test body with a fixed directory:

```go
func TestGoldenComponent_FileBrowser(t *testing.T) {
	// Use os.TempDir() as a stable root — shows consistent directory entries.
	// We snapshot the component structure, not specific file names.
	// If the view contains temp-specific paths, we instead snapshot an empty dir.
	tmp := t.TempDir()
	// Create stable subdirs for deterministic output
	os.MkdirAll(filepath.Join(tmp, "skills", "alpha-skill"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "skills", "beta-skill"), 0o755)

	fb := newFileBrowser(tmp, catalog.Skills)
	fb.width = 62
	fb.height = 28
	requireGolden(t, "component-filebrowser", stripANSI(fb.View()))
}
```

Then re-run `-update` and verify the golden content is stable (same output on repeated runs).

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -run TestGoldenComponent_FileBrowser ./internal/tui/...
```

Expected: `PASS`

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/golden_components_test.go \
        cli/internal/tui/testdata/component-filebrowser.golden
git commit -m "test: add golden snapshot for file browser component"
```

---

## Task 12: Full Test Suite Verification

**Files:**
- None (verification only)

**Depends on:** Tasks 1–11 (all golden files created)

**Success Criteria:**
- [ ] `make test` passes (all 14 golden tests pass alongside existing tests)
- [ ] Total golden file count in testdata/ is exactly 14
- [ ] No duplicate flag registration errors (only one `var update` declaration)
- [ ] `go vet ./cli/internal/tui/...` passes

---

### Step 1: Run the full test suite

```bash
cd /home/hhewett/.local/src/syllago && make test
```

Expected: All tests pass. Look for the golden test names in the output — they should all show `PASS`, no `FAIL`.

### Step 2: Count golden files

```bash
ls /home/hhewett/.local/src/syllago/cli/internal/tui/testdata/*.golden | wc -l
```

Expected: 13 or 14 files (14 if modal golden was created, 13 if the modal test skipped).

### Step 3: Run vet

```bash
cd /home/hhewett/.local/src/syllago/cli && go vet ./internal/tui/...
```

Expected: no output (no issues)

### Step 4: Verify update workflow

Make a trivial visible change to the sidebar (add a space to the "syllago" title render), run update, check the diff, then revert:

```bash
# Make a known change
cd /home/hhewett/.local/src/syllago
sed -i 's/titleStyle.Render("syllago")/titleStyle.Render("syllago ")/' cli/internal/tui/sidebar.go

# Run update
cd cli && go test -update ./internal/tui/... 2>&1 | grep "updated golden"

# Check what changed
git diff cli/internal/tui/testdata/ | head -30

# Revert
git checkout cli/internal/tui/sidebar.go cli/internal/tui/testdata/
```

Expected: `git diff` shows the "syllago " (with trailing space) in the affected golden files. Revert restores both source and golden files cleanly.

### Step 5: Final commit if any fixups were needed

If step 4 revealed any issues with the golden files (unexpected content, non-determinism), fix and recommit:

```bash
cd /home/hhewett/.local/src/syllago/cli && go test -update ./internal/tui/...
cd /home/hhewett/.local/src/syllago
git add cli/internal/tui/testdata/
git commit -m "test: fix golden file determinism after full suite verification"
```

---

## Summary

After all 12 tasks are complete, the syllago TUI will have:

- **3 new test files** in `cli/internal/tui/`: `golden_test.go`, `golden_fullapp_test.go`, `golden_components_test.go`
- **14 golden files** in `cli/internal/tui/testdata/` (all 8 full-app + all 6 component, modal included)
- **One `.gitattributes` entry** ensuring LF line endings on golden files
- **`-update` flag** for regenerating golden files when UI intentionally changes
- **Zero new external dependencies** — `charmbracelet/x/ansi` was already an indirect dep

The developer workflow after any TUI change:
1. `make test` — shows which screens changed (golden diff)
2. `go test -update ./cli/internal/tui/...` — regenerate if the change was intentional
3. `git diff cli/internal/tui/testdata/` — visual review of the change
4. Commit source + golden files together

**Golden File Update Protocol (for AI-assisted development):**
When making a TUI change during a development session, golden files MUST be updated as part of the same change — not deferred. After any TUI code change: run `go test -update ./cli/internal/tui/...`, review `git diff cli/internal/tui/testdata/`, and stage golden file updates in the same commit as the code change. If the golden diff reveals an unintended regression, fix the code before proceeding. See the design doc for full protocol details.
