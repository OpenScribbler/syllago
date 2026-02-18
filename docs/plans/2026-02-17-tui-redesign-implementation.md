# TUI Redesign - Implementation Plan

**Goal:** Refactor the Nesco TUI from full-screen replacement navigation to a persistent sidebar + content layout with modal overlays, mouse support, and the Romanesco color palette.
**Architecture:** The `App` struct gains a `focusTarget` enum and a `sidebarModel` extracted from `categoryModel`; the existing `screen` enum is replaced by a content-area routing field; `App.View()` composes sidebar and content with `lipgloss.JoinHorizontal`; modals are centered overlays rendered by `bubbletea-overlay`.
**Tech Stack:** Go 1.25, Bubble Tea v1.3.10, lipgloss v1.1.1, bubblezone (latest), bubbletea-overlay v0.6.5
**Design Doc:** `/home/hhewett/.local/src/romanesco/docs/plans/2026-02-17-tui-redesign-design.md`

---

## Group 1: Foundation — Styles and Dependencies

### Task 1.1: Update Semantic Color Variables in styles.go

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/styles.go` (lines 5-14)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/styles_test.go`

**Depends on:** nothing

**Success Criteria:**
- [ ] Six semantic colors match Romanesco palette values exactly
- [ ] `secondaryColor` variable is renamed to `accentColor`
- [ ] Comment reflects correct adaptive convention (Light = light terminal bg, Dark = dark terminal bg)
- [ ] `go build ./...` passes

The comment in styles.go currently has Light and Dark reversed ("Light = color on dark backgrounds"). Fix that alongside the color values.

#### Step 0: Write failing test

```go
// cli/internal/tui/styles_test.go
func TestRomanescoPaletteValues(t *testing.T) {
	// Verify the primary color uses the Romanesco mint value (not the old color)
	ac, ok := primaryColor.(lipgloss.AdaptiveColor)
	if !ok {
		t.Fatal("primaryColor should be AdaptiveColor")
	}
	if ac.Dark != "#6EE7B7" {
		t.Errorf("primaryColor.Dark should be #6EE7B7 (Romanesco mint), got %s", ac.Dark)
	}
	if ac.Light != "#047857" {
		t.Errorf("primaryColor.Light should be #047857 (Romanesco mint dark), got %s", ac.Light)
	}
	// Verify accentColor exists (was secondaryColor)
	acc, ok := accentColor.(lipgloss.AdaptiveColor)
	if !ok {
		t.Fatal("accentColor should be AdaptiveColor")
	}
	if acc.Dark != "#C4B5FD" {
		t.Errorf("accentColor.Dark should be #C4B5FD (Romanesco viola), got %s", acc.Dark)
	}
}

func TestPanelStylesExist(t *testing.T) {
	// Verify panel styles are defined (will fail if vars don't exist)
	_ = sidebarBorderStyle
	_ = contentHeaderStyle
	_ = footerStyle
	_ = breadcrumbStyle
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (accentColor undefined, sidebarBorderStyle undefined)

#### Step 1: Replace the color block

Replace lines 5-14 in `styles.go`:

```go
var (
	// Colors — adaptive for light/dark terminal themes.
	// Light = color on light terminal backgrounds, Dark = color on dark terminal backgrounds.
	primaryColor = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7B7"} // Mint
	accentColor  = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"} // Viola
	mutedColor   = lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"} // Stone
	successColor = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"} // Green
	dangerColor  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"} // Red
	warningColor = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"} // Amber
```

#### Step 2: Add panel/layout color variables after the semantic block

Add immediately after `warningColor`:

```go
	// Panel and layout colors
	borderColor   = lipgloss.AdaptiveColor{Light: "#D4D4D8", Dark: "#3F3F46"}
	selectedBgColor = lipgloss.AdaptiveColor{Light: "#D1FAE5", Dark: "#1A3A2A"}
	modalBgColor  = lipgloss.AdaptiveColor{Light: "#F4F4F5", Dark: "#27272A"}
	modalBorderColor = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"} // same as accent
```

#### Step 3: Update style variables that reference secondaryColor → accentColor

In the same file, update `selectedItemStyle` to use the new colors:

```go
	selectedItemStyle = lipgloss.NewStyle().
		Foreground(accentColor).
		Background(selectedBgColor).
		Bold(true)
```

Also update `searchPromptStyle` to use `primaryColor` (already correct) and `titleStyle` (already correct).

Add new panel styles at the end of the `var` block:

```go
	// Sidebar panel
	sidebarBorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		BorderRight(true)

	// Content panel header bar (item name + tab bar line)
	contentHeaderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		BorderBottom(true)

	// Footer bar
	footerStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		BorderTop(true).
		Foreground(mutedColor)

	// Breadcrumb within footer
	breadcrumbStyle = lipgloss.NewStyle().
		Foreground(mutedColor)
```

#### Step 4: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 1.2: Update styles_test.go for Renamed Variable

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/styles_test.go` (lines 21-34)

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] Test references `accentColor` instead of `secondaryColor`
- [ ] `go test ./internal/tui/...` passes

#### Step 1: Replace the colors map in TestColorsAreAdaptive

```go
func TestColorsAreAdaptive(t *testing.T) {
	colors := map[string]lipgloss.TerminalColor{
		"primaryColor": primaryColor,
		"accentColor":  accentColor,
		"mutedColor":   mutedColor,
		"successColor": successColor,
		"dangerColor":  dangerColor,
		"warningColor": warningColor,
	}

	for name, c := range colors {
		if _, ok := c.(lipgloss.AdaptiveColor); !ok {
			t.Errorf("%s should be AdaptiveColor, got %T", name, c)
		}
	}
}
```

Also add tests for the new panel colors:

```go
func TestPanelColorsAreAdaptive(t *testing.T) {
	colors := map[string]lipgloss.TerminalColor{
		"borderColor":      borderColor,
		"selectedBgColor":  selectedBgColor,
		"modalBgColor":     modalBgColor,
		"modalBorderColor": modalBorderColor,
	}
	for name, c := range colors {
		if _, ok := c.(lipgloss.AdaptiveColor); !ok {
			t.Errorf("%s should be AdaptiveColor, got %T", name, c)
		}
	}
}
```

---

### Task 1.3: Add New Dependencies

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/go.mod` (via go get)
- Modify: `/home/hhewett/.local/src/romanesco/cli/go.sum` (via go get)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/styles_test.go`

**Depends on:** nothing (can run in parallel with Task 1.1)

**Success Criteria:**
- [ ] `go.mod` contains `github.com/lrstanley/bubblezone`
- [ ] `go.mod` contains `github.com/rmhubbert/bubbletea-overlay`
- [ ] `go build ./...` passes after adding imports

#### Step 0: Write failing test

```go
// cli/internal/tui/styles_test.go
func TestDependenciesPresent(t *testing.T) {
	// This test verifies the dependencies are importable by referencing their
	// package paths. It will fail to compile if the packages are not in go.mod.
	// Add an import to a file that uses both packages and verify go build passes.
	// The actual test is: `go build ./...` succeeds after imports are added.
	// This placeholder ensures we think about it before implementing.
	t.Log("Dependencies presence verified by go build ./... in Step 3")
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go build ./...`
Expected: Build fails if any file imports the packages before they're in go.mod. (If no imports yet, verify go.mod does NOT contain the packages.)

#### Step 1: Run go get for both packages

```bash
cd /home/hhewett/.local/src/romanesco/cli
go get github.com/lrstanley/bubblezone@latest
go get github.com/rmhubbert/bubbletea-overlay@latest
```

> **Note:** The package `github.com/erikgeiser/bubbletea-overlay` does not exist. Use `github.com/rmhubbert/bubbletea-overlay` instead. Its API differs from what was originally assumed: the correct call is `overlay.Composite(fg, bg string, xPos, yPos Position, xOff, yOff int) string`, not `overlay.PlacePosition()`. All modal `overlayView()` methods in Tasks 6.1–6.6 use `overlay.Composite()` accordingly.

#### Step 2: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go build ./...`
Expected: PASS (go.mod now contains both packages)

---

## Group 2: Sidebar Model

### Task 2.1: Create sidebar.go

**Files:**
- Create: `/home/hhewett/.local/src/romanesco/cli/internal/tui/sidebar.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/sidebar_test.go`

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] `sidebarModel` struct compiles with no errors
- [ ] `sidebarModel.View()` returns a fixed-width (~18 chars including border) column
- [ ] Cursor navigation works through all items (content types + utility items)
- [ ] Selected item uses `selectedItemStyle`; utility items use diamond prefix (◆/◇)
- [ ] `sidebarModel` does not import `app.go` or create circular deps

The sidebar extracts the rendering logic from `categoryModel.View()`, reformatted for a narrow column. The existing `categoryModel` is kept for now and removed in Group 3 once App is wired to the sidebar.

#### Step 0: Write failing test

```go
// cli/internal/tui/sidebar_test.go
package tui

import (
	"strings"
	"testing"
)

func TestSidebarModelZeroValue(t *testing.T) {
	// A zero-value sidebarModel should not panic when View() is called
	var m sidebarModel
	view := m.View()
	if view == "" {
		t.Error("sidebarModel.View() should return non-empty string even for zero value")
	}
}

func TestSidebarCursorNavigation(t *testing.T) {
	m := sidebarModel{
		types:  nil, // zero content types
		cursor: 0,
		focused: true,
	}
	total := m.totalItems()
	if total < 4 {
		t.Errorf("totalItems() should be at least 4 (utility items), got %d", total)
	}
}

func TestSidebarViewContainsDiamonds(t *testing.T) {
	m := sidebarModel{focused: true}
	view := m.View()
	// Utility items use diamond prefix
	if !strings.Contains(view, "◆") && !strings.Contains(view, "◇") {
		t.Error("sidebarModel.View() should contain diamond prefix (◆ or ◇) for utility items")
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (sidebarModel undefined)

#### Step 1: Write sidebar.go

```go
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

const sidebarWidth = 18 // fixed width including border character

type sidebarModel struct {
	types      []catalog.ContentType
	counts     map[catalog.ContentType]int
	localCount int
	cursor     int
	focused    bool

	// Version/update state (displayed in sidebar header)
	version         string
	remoteVersion   string
	updateAvailable bool
	commitsBehind   int
}

func newSidebarModel(cat *catalog.Catalog, version string) sidebarModel {
	return sidebarModel{
		types:      catalog.AllContentTypes(),
		counts:     cat.CountByType(),
		localCount: cat.CountLocal(),
		version:    version,
	}
}

// totalItems returns the total number of navigable items in the sidebar
// (content types + My Tools + Import + Update + Settings).
func (m sidebarModel) totalItems() int {
	return len(m.types) + 4
}

func (m sidebarModel) Update(msg tea.Msg) (sidebarModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < m.totalItems()-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Home):
			m.cursor = 0
		case key.Matches(msg, keys.End):
			m.cursor = m.totalItems() - 1
		}
	}
	return m, nil
}

func (m sidebarModel) View() string {
	// Inner width: sidebarWidth minus 1 for the right border character
	inner := sidebarWidth - 1

	var s string

	// Header: "nesco" title
	title := primaryColor.Dark // placeholder; actual rendering uses lipgloss
	_ = title
	s += titleStyle.Render("nesco") + "\n\n"

	// Content type rows
	for i, ct := range m.types {
		count := m.counts[ct]
		prefix := "  "
		label := ct.Label()
		countStr := fmt.Sprintf("%2d", count)

		line := fmt.Sprintf("%-*s%s", inner-len(countStr)-2, label, countStr)
		if len(line) > inner {
			line = line[:inner]
		}

		if i == m.cursor {
			s += "▸ " + selectedItemStyle.Render(line) + "\n"
		} else {
			s += prefix + itemStyle.Render(line) + "\n"
		}
	}

	// Separator
	s += helpStyle.Render("  " + "─────────────") + "\n"

	// Utility items: My Tools, Import, Update, Settings
	utilItems := []struct {
		label  string
		index  int
		hasDot bool // true = ◆ (has items), false = ◇
	}{
		{fmt.Sprintf("My Tools %2d", m.localCount), len(m.types), m.localCount > 0},
		{"Import", len(m.types) + 1, false},
		{"Update", len(m.types) + 2, false},
		{"Settings", len(m.types) + 3, false},
	}

	for _, u := range utilItems {
		diamond := "◇"
		if u.hasDot {
			diamond = "◆"
		}
		if u.index == m.cursor {
			s += diamond + " " + selectedItemStyle.Render(u.label) + "\n"
		} else {
			s += diamond + " " + itemStyle.Render(u.label) + "\n"
		}
	}

	return sidebarBorderStyle.Width(sidebarWidth).Render(s)
}

// Selector methods (mirror categoryModel for use in App.Update routing)
func (m sidebarModel) isMyToolsSelected() bool { return m.cursor == len(m.types) }
func (m sidebarModel) isImportSelected() bool   { return m.cursor == len(m.types)+1 }
func (m sidebarModel) isUpdateSelected() bool   { return m.cursor == len(m.types)+2 }
func (m sidebarModel) isSettingsSelected() bool { return m.cursor == len(m.types)+3 }
func (m sidebarModel) selectedType() catalog.ContentType {
	if m.cursor >= len(m.types) {
		return ""
	}
	return m.types[m.cursor]
}
```

#### Step 2: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 2.2: Wire sidebarModel into App struct (compile-only, no behavior change yet)

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go` (lines 27-56, 60-71)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app_test.go`

**Depends on:** Task 2.1

**Success Criteria:**
- [ ] `App` struct has a `sidebar sidebarModel` field
- [ ] `NewApp()` initializes `sidebar` via `newSidebarModel()`
- [ ] `go build ./...` passes (old `category` field kept for now to avoid breaking Update logic)

#### Step 0: Write failing test

```go
// cli/internal/tui/app_test.go
package tui

import (
	"testing"
)

func TestAppHasSidebarField(t *testing.T) {
	// Verify App struct has a sidebar field by constructing a zero-value App
	// and checking the sidebar field is accessible (compile-time check).
	var a App
	_ = a.sidebar
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (App.sidebar undefined)

#### Step 1: Add sidebar field to App struct

In `app.go`, add after line 36 (`category categoryModel`):

```go
	sidebar     sidebarModel
```

#### Step 2: Initialize in NewApp()

Add to the `return App{...}` literal in `NewApp()`:

```go
		sidebar:    newSidebarModel(cat, version),
```

#### Step 3: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

## Group 3: Layout Composition

### Task 3.1: Add focusTarget Type and Focus Field to App

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go` (lines 16-25, 27-56)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app_test.go`

**Depends on:** Task 2.2

**Success Criteria:**
- [ ] `focusTarget` type and constants compile
- [ ] `App` struct has `focus focusTarget` field
- [ ] `NewApp()` sets `focus: focusSidebar`

#### Step 0: Write failing test

```go
// cli/internal/tui/app_test.go
func TestFocusTargetConstants(t *testing.T) {
	// Verify the three focus constants are defined and distinct
	if focusSidebar == focusContent {
		t.Error("focusSidebar and focusContent should be distinct")
	}
	if focusContent == focusModal {
		t.Error("focusContent and focusModal should be distinct")
	}
	// Verify App has a focus field
	var a App
	_ = a.focus
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (focusSidebar undefined, App.focus undefined)

#### Step 1: Add focusTarget type after the screen enum

Insert after line 25 (after the `screen` const block):

```go
type focusTarget int

const (
	focusSidebar focusTarget = iota
	focusContent
	focusModal
)
```

#### Step 2: Add focus field to App struct

Add after the `screen screen` field:

```go
	focus       focusTarget
```

#### Step 3: Set initial focus in NewApp

Add to `NewApp()` return literal:

```go
		focus:    focusSidebar,
```

#### Step 4: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 3.2: Refactor App.View() to Sidebar + Content Composition

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go` (lines 511-553)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app_test.go`

**Depends on:** Task 3.1

**Success Criteria:**
- [ ] Sidebar is always rendered on the left regardless of current screen
- [ ] Content area width = total width − sidebarWidth
- [ ] `lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)` composes the panels
- [ ] Footer is rendered below the joined panels via `lipgloss.JoinVertical`
- [ ] "Too small" guard still works (width < 60 collapses sidebar, shows content only)
- [ ] Search and help overlays still work (they replace the footer, not the panels)

#### Step 0: Write failing test

```go
// cli/internal/tui/app_test.go
func TestAppViewContainsBreadcrumb(t *testing.T) {
	// App.View() should include a footer with breadcrumb text
	a := App{
		width:  80,
		height: 24,
		screen: screenCategory,
	}
	view := a.View()
	// The default breadcrumb for screenCategory is "nesco"
	if !strings.Contains(view, "nesco") {
		t.Error("App.View() should contain 'nesco' breadcrumb in the footer")
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (breadcrumb/footer not rendered in current App.View())

#### Step 1: Replace App.View() entirely

```go
func (a App) View() string {
	if a.tooSmall {
		// Below minimum: skip sidebar, show warning or content full-width
		if a.width < 60 || a.height < 10 {
			return "\n" + warningStyle.Render("Terminal too small. Resize to at least 60x10.") + "\n"
		}
	}

	// sidebarWidth is the lipgloss inner content width. With BorderRight(true),
	// the rendered sidebar is sidebarWidth + 1 characters wide (19 total, not 18).
	// Subtract the extra border character so content does not overflow the terminal.
	contentWidth := a.width - sidebarWidth - 1
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Sidebar (always visible)
	a.sidebar.focused = (a.focus == focusSidebar)
	sidebarView := a.sidebar.View()

	// Content area: route to active sub-view
	var contentView string
	switch a.screen {
	case screenItems:
		contentView = a.items.View()
	case screenDetail:
		contentView = a.detail.View()
	case screenImport:
		contentView = a.importer.View()
	case screenUpdate:
		contentView = a.updater.View()
	case screenSettings:
		contentView = a.settings.View()
	default:
		// screenCategory: show items if a category is "selected" but not yet drilled in,
		// or show a welcome/empty state
		contentView = a.renderContentWelcome()
	}

	// Help overlay replaces the content view entirely
	if a.helpOverlay.active {
		contentView = a.helpOverlay.View(a.screen)
	}

	// Compose sidebar + content horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, contentView)

	// Footer: breadcrumb on left, help on right
	footer := a.renderFooter()

	// Search overlay replaces footer
	if a.search.active {
		footer = a.search.View()
	}

	body := lipgloss.JoinVertical(lipgloss.Left, panels, footer)

	return fmt.Sprintf("\n%s\n", body)
}

// renderContentWelcome returns a placeholder for when no category is drilled into.
func (a App) renderContentWelcome() string {
	return helpStyle.Render("Select a category from the sidebar to browse content.")
}

// renderFooter builds the breadcrumb + context-sensitive help bar.
func (a App) renderFooter() string {
	crumb := a.breadcrumb()
	var helpText string
	switch a.screen {
	case screenDetail:
		helpText = "Esc: back   Tab: switch tab   ?: help   q: quit"
	case screenItems:
		helpText = "/: search   Enter: detail   Esc: sidebar   ?: help   q: quit"
	default:
		helpText = "Tab: switch panel   /: search   ?: help   q: quit"
	}

	left := footerStyle.Render(helpText)
	right := breadcrumbStyle.Render(crumb)

	// Pad left to fill width, right-align crumb
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// breadcrumb returns a "Category > Item" string for the current navigation state.
func (a App) breadcrumb() string {
	switch a.screen {
	case screenDetail:
		return a.sidebar.selectedType().Label() + " > " + displayName(a.detail.item)
	case screenItems:
		return a.sidebar.selectedType().Label()
	case screenImport:
		return "Import"
	case screenUpdate:
		return "Update"
	case screenSettings:
		return "Settings"
	default:
		return "nesco"
	}
}
```

#### Step 2: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 3.3: Refactor App.Update() for Focus-Based Input Routing

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go` (lines 77-509)
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/keys.go` (add `Right` binding)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app_test.go`

**Depends on:** Task 3.2

**Success Criteria:**
- [ ] `Right key.Binding` added to `keyMap` struct and initialized in `keys.go` with keys `"right", "l"`
- [ ] Tab key toggles focus between sidebar and content (when not on screenDetail; see guard note below)
- [ ] All input routes to modal first (when modal active — prep for Group 6)
- [ ] When `focusSidebar`: j/k/up/down navigate sidebar, Enter/Right loads items into content and shifts focus to content
- [ ] When `focusContent`: existing screen-specific key handlers fire
- [ ] Esc from items view shifts focus back to sidebar (instead of going to screenCategory)
- [ ] `WindowSizeMsg` propagates content width to sub-models (width − sidebarWidth)
- [ ] All existing keyboard shortcuts continue to work within the content area
- [ ] All `a.category.selectedType()` references in the search active block are replaced with `a.sidebar.selectedType()`

#### Step 0: Write failing test

```go
// cli/internal/tui/app_test.go
func TestTabTogglesFocus(t *testing.T) {
	// Tab key should toggle focus from sidebar to content
	a := App{
		width:  80,
		height: 24,
		screen: screenItems,
		focus:  focusSidebar,
	}
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	result, _ := a.Update(tabMsg)
	updated := result.(App)
	if updated.focus != focusContent {
		t.Errorf("Tab should move focus from sidebar to content, got focus=%d", updated.focus)
	}
}

func TestEscFromItemsReturnsFocusToSidebar(t *testing.T) {
	// Esc from screenItems should shift focus to sidebar, not change screen
	a := App{
		width:  80,
		height: 24,
		screen: screenItems,
		focus:  focusContent,
	}
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	result, _ := a.Update(escMsg)
	updated := result.(App)
	if updated.focus != focusSidebar {
		t.Errorf("Esc from screenItems should set focus=focusSidebar, got %d", updated.focus)
	}
	if updated.screen != screenItems {
		t.Errorf("Esc from screenItems should keep screen=screenItems, got %d", updated.screen)
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (Tab does not toggle focus, Esc changes screen instead of focus)

#### Step 0c: Add Right binding to keys.go

Before making any changes to `app.go`, add the `Right` binding to `keys.go`. The `keyMap` struct currently has no `Right` field; `key.Matches(msg, keys.Right)` used in Step 3 will not compile without it.

Add `Right key.Binding` to the `keyMap` struct and initialize it in the `keys` variable:

```go
// In the keyMap struct:
Right key.Binding

// In the keys var:
Right: key.NewBinding(
    key.WithKeys("right", "l"),
    key.WithHelp("right/l", "enter"),
),
```

#### Step 1: Update WindowSizeMsg handler to account for sidebar width

Replace the `case tea.WindowSizeMsg:` block:

```go
case tea.WindowSizeMsg:
	a.width = msg.Width
	a.height = msg.Height
	a.tooSmall = msg.Width < 60 || msg.Height < 10
	// sidebarWidth is inner content width; rendered width is sidebarWidth + 1 (right border).
	contentW := msg.Width - sidebarWidth - 1
	if contentW < 20 {
		contentW = 20
	}
	a.items.width = contentW
	a.items.height = msg.Height
	a.detail.width = contentW
	a.detail.height = msg.Height
	a.detail.clampScroll()
	a.importer.width = contentW
	a.importer.height = msg.Height
	a.updater.width = contentW
	a.updater.height = msg.Height
	a.settings.width = contentW
	a.settings.height = msg.Height
	return a, nil
```

#### Step 2: Add Tab focus-switching before screen-specific handling

In the `case tea.KeyMsg:` block, after the search-active block and before `switch a.screen`, add:

```go
		// Tab/Shift+Tab: switch focus between sidebar and content.
		// Guard: NOT on screenDetail (Tab still switches detail tabs when content is focused).
		// On screenDetail, Tab is handled by detail.Update() to switch Overview/Files/Install tabs.
		// Panel-focus Tab only fires when sidebar is focused OR when on screens other than screenDetail.
		if (key.Matches(msg, keys.Tab) || key.Matches(msg, keys.ShiftTab)) &&
			!a.search.active && !a.helpOverlay.active &&
			a.screen != screenDetail {
			if a.screen != screenImport && a.screen != screenUpdate && a.screen != screenSettings {
				if !a.detail.HasTextInput() {
					if a.focus == focusSidebar {
						a.focus = focusContent
					} else {
						a.focus = focusSidebar
					}
					return a, nil
				}
			}
		}
```

#### Step 2b: Update search active block to replace `a.category` references

Within the existing search-active block (before `switch a.screen`), the Esc and Enter handlers reference `a.category.selectedType()`. Replace every occurrence with `a.sidebar.selectedType()`. The relevant lines are at approximately app.go lines 262 and 280:

```go
// Before (in the search Esc handler):
ct := a.category.selectedType()

// After:
ct := a.sidebar.selectedType()
```

Apply the same replacement in the search Enter handler. Failure to do this will cause a compile error when `a.category` is removed in Task 3.4.

#### Step 3: Replace screenCategory handling with sidebar-focus routing

Replace the entire `case screenCategory:` block in `switch a.screen`:

```go
		// Sidebar-focused: route input to sidebar; Enter drills into content
		case screenCategory:
			if a.focus == focusSidebar {
				if key.Matches(msg, keys.Enter) || key.Matches(msg, keys.Right) {
					if a.sidebar.isUpdateSelected() {
						a.updater = newUpdateModel(a.catalog.RepoRoot, a.version, a.remoteVersion, a.commitsBehind)
						a.updater.width = a.width - sidebarWidth
						a.updater.height = a.height
						a.screen = screenUpdate
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isSettingsSelected() {
						a.settings = newSettingsModel(a.catalog.RepoRoot, a.providers, a.detectors)
						a.settings.width = a.width - sidebarWidth
						a.settings.height = a.height
						a.screen = screenSettings
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isImportSelected() {
						a.importer = newImportModel(a.providers, a.catalog.RepoRoot)
						a.importer.width = a.width - sidebarWidth
						a.importer.height = a.height
						a.screen = screenImport
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isMyToolsSelected() {
						var localItems []catalog.ContentItem
						for _, item := range a.catalog.Items {
							if item.Local {
								localItems = append(localItems, item)
							}
						}
						items := newItemsModel(catalog.MyTools, localItems, a.providers, a.catalog.RepoRoot)
						items.width = a.width - sidebarWidth
						items.height = a.height
						a.items = items
						a.screen = screenItems
						a.focus = focusContent
						return a, nil
					}
					ct := a.sidebar.selectedType()
					items := newItemsModel(ct, a.catalog.ByType(ct), a.providers, a.catalog.RepoRoot)
					items.width = a.width - sidebarWidth
					items.height = a.height
					a.items = items
					a.screen = screenItems
					a.focus = focusContent
					return a, nil
				}
				var cmd tea.Cmd
				a.sidebar, cmd = a.sidebar.Update(msg)
				return a, cmd
			}
```

#### Step 4: Update screenItems Esc to return focus to sidebar instead of changing screen

Replace the `if key.Matches(msg, keys.Back)` inside `case screenItems:`:

```go
		case screenItems:
			if key.Matches(msg, keys.Back) {
				// Esc: shift focus back to sidebar (sidebar stays visible, items remain)
				a.focus = focusSidebar
				a.sidebar.counts = a.catalog.CountByType()
				a.sidebar.localCount = a.catalog.CountLocal()
				return a, nil
			}
```

#### Step 5: Keep screenDetail, screenImport, screenUpdate, screenSettings handlers unchanged

The remaining screen handlers (screenDetail, screenImport, screenUpdate, screenSettings) stay as-is. Their Esc navigation is still screen-based (e.g., screenDetail → screenItems; screenImport → screenCategory which now means sidebar focus).

For `screenImport` back:

```go
		case screenImport:
			if key.Matches(msg, keys.Back) && a.importer.step == stepSource {
				a.screen = screenCategory
				a.focus = focusSidebar
				a.importer.cleanup()
				return a, nil
			}
```

For `screenUpdate` and `screenSettings` back, add `a.focus = focusSidebar` after `a.screen = screenCategory`.

#### Step 6: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 3.4: Remove Redundant screenCategory View Routing and Category Model

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go` (App.View switch)
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go` (App struct, NewApp)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app_test.go`

**Depends on:** Task 3.3

**Success Criteria:**
- [ ] `App.View()` no longer has a `case screenCategory:` branch (sidebar handles that)
- [ ] `category categoryModel` field removed from `App` struct (sidebar replaced it)
- [ ] `go build ./...` passes
- [ ] Navigating categories, drilling into items, and pressing Esc back to sidebar all work correctly
- [ ] No references to `a.category` remain anywhere in `app.go`
- [ ] Import success/failure message is displayed in the UI after category field is removed

#### Step 0: Write failing test

```go
// cli/internal/tui/app_test.go
func TestAppHasNoCategory(t *testing.T) {
	// After removing categoryModel, App should have no category field.
	// This is a compile-time check: if App.category exists, this assignment fails.
	// We verify by checking App.sidebar is the navigation mechanism.
	var a App
	// Accessing a.sidebar should work; a.category should not compile after Task 3.4
	_ = a.sidebar
	// The presence of a.statusMessage field is also verifiable
	_ = a.statusMessage
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (App.statusMessage undefined)

#### Step 1: Remove category field from App struct

Delete `category categoryModel` from App struct.

#### Step 2: Remove category initialization from NewApp

Delete `category: newCategoryModel(cat, version),` from NewApp.

#### Step 3: Update ALL remaining references to a.category

In `app.go` Update(), replace every `a.category.*` reference. The full set of replacements required:

**`updateCheckMsg` handler** (app.go ~lines 156-172): Replace the three fields that `updateCheckMsg` sets on `a.category`:
```go
// Before:
a.category.remoteVersion = msg.remoteVersion
a.category.updateAvailable = true
a.category.commitsBehind = msg.commitsBehind

// After:
a.sidebar.remoteVersion = msg.remoteVersion
a.sidebar.updateAvailable = true
a.sidebar.commitsBehind = msg.commitsBehind
```
The `sidebarModel` struct (defined in Task 2.1) already has `remoteVersion string`, `updateAvailable bool`, and `commitsBehind int` fields.

**`importDoneMsg` handler**: The handler currently sets `a.category.message` to display import success/failure to the user. `sidebarModel` has no `message` field. Store the message in `App` instead — add a `statusMessage string` field to the `App` struct and render it in the footer:
```go
// In App struct (add alongside other fields):
statusMessage string

// In importDoneMsg handler (replace a.category.message = ...):
a.sidebar.counts = a.catalog.CountByType()
a.sidebar.localCount = a.catalog.CountLocal()
a.statusMessage = fmt.Sprintf("Imported %q successfully", msg.name)
// (for the error case: a.statusMessage = fmt.Sprintf("Imported %q but catalog rescan failed: %s", msg.name, err))
a.focus = focusSidebar
```
Update `renderFooter()` to display `a.statusMessage` when set (e.g., append it to the footer line). This replaces the old in-sidebar message display.

**`promoteDoneMsg` handler**: Replace `a.category.counts` with `a.sidebar.counts`.

**`updatePullMsg` handler**: Replace `a.category.counts` with `a.sidebar.counts`.

**Any remaining `a.category.counts` or `a.category.localCount` lines** (e.g., in the Esc handler for screenItems from Task 3.3 Step 4): replace with `a.sidebar.counts` and `a.sidebar.localCount`.

#### Step 4: Delete category.go

Once `categoryModel` is fully replaced by `sidebarModel` and no references remain, delete:
`/home/hhewett/.local/src/romanesco/cli/internal/tui/category.go`

#### Step 5: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

## Group 4: Detail View Layout

### Task 4.1: Refactor renderContent() to Put Metadata Above Separator

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_render.go` (lines 15-50)
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail.go` (`clampScroll()` method — Step 4 rewrites it)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_render_test.go`

**Depends on:** Task 3.2 (sidebar exists and content width is set correctly)

**Success Criteria:**
- [ ] Detail header bar shows item name + tab bar on one line (not separate lines)
- [ ] Metadata (Type, Path, Providers) appears above a horizontal separator — always visible
- [ ] Scrollable content area is everything below the separator only
- [ ] `clampScroll()` accounts for the pinned header+metadata height

The key structural change: move Type/Path/Provider metadata out of `renderOverviewTab()` and into a pinned header section in `renderContent()`. Only the tab content (README, file list, install panel) scrolls.

#### Step 0: Write failing test

```go
// cli/internal/tui/detail_render_test.go
package tui

import (
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

func TestRenderContentSplitHasSeparator(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name: "test-tool",
			Type: catalog.Prompts,
		},
		width:  60,
		height: 24,
	}
	pinned, _ := m.renderContentSplit()
	// The separator (─ repeated) must appear in the pinned section
	if !strings.Contains(pinned, "─") {
		t.Error("renderContentSplit pinned section should contain a horizontal separator (─)")
	}
}

func TestRenderContentSplitMetadataInPinned(t *testing.T) {
	m := detailModel{
		item: catalog.ContentItem{
			Name: "test-tool",
			Type: catalog.Prompts,
		},
		width:  60,
		height: 24,
	}
	pinned, body := m.renderContentSplit()
	// Type metadata must be in pinned, not in body
	if !strings.Contains(pinned, "Type:") {
		t.Error("renderContentSplit pinned section should contain 'Type:' metadata")
	}
	if strings.Contains(body, "Type:") {
		t.Error("renderContentSplit body should not contain 'Type:' (moved to pinned)")
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (renderContentSplit undefined)

#### Step 1: Replace renderContent() with pinned-header version

```go
// renderContent builds the full detail content (without scrolling or help bar).
// Returns two strings: pinnedHeader (always visible) and scrollableBody (scrolls).
func (m detailModel) renderContent() string {
	// This method returns the combined string for compatibility with clampScroll.
	pinned, body := m.renderContentSplit()
	return pinned + body
}

// renderContentSplit returns pinned header and scrollable body separately.
func (m detailModel) renderContentSplit() (pinned string, body string) {
	name := StripControlChars(displayName(m.item))
	position := ""
	if m.listTotal > 0 {
		position = fmt.Sprintf(" (%d/%d)", m.listPosition+1, m.listTotal)
	}

	// Header line: name + position + LOCAL tag + tab bar (right side)
	nameStr := titleStyle.Render(name)
	if m.item.Local {
		nameStr += " " + warningStyle.Render("[LOCAL]")
	}
	nameStr += helpStyle.Render(position)
	tabBar := m.renderTabBar()

	// Combine name and tab bar on one line with separator
	headerLine := nameStr + "  " + tabBar
	pinned += headerLine + "\n"

	// Metadata block: Type, Path, Providers (always visible, above separator)
	pinned += "\n"
	pinned += labelStyle.Render("Type: ") + valueStyle.Render(m.item.Type.Label())
	if m.item.Path != "" {
		pinned += "   " + labelStyle.Render("Path: ") + valueStyle.Render(m.item.Path)
	}
	pinned += "\n"
	if m.item.Provider != "" {
		pinned += labelStyle.Render("Providers: ") + valueStyle.Render(m.item.Provider) + "\n"
	}

	// Horizontal separator
	pinned += "\n" + helpStyle.Render(strings.Repeat("─", 60)) + "\n\n"

	// Scrollable body: tab content
	switch m.activeTab {
	case tabOverview:
		body = m.renderOverviewTab()
	case tabFiles:
		body = m.renderFilesTab()
	case tabInstall:
		body = m.renderInstallTab()
	}

	return pinned, body
}
```

#### Step 2: Remove metadata from renderOverviewTab()

In `renderOverviewTab()`, delete the lines that render Type, Path, Provider (they're now in the pinned header). This is the block at lines 109-113 of `detail_render.go`:

```go
	// DELETE THESE LINES (now in pinned header):
	// s += "\n"
	// s += labelStyle.Render("Type: ") + valueStyle.Render(m.item.Type.Label()) + "\n"
	// s += labelStyle.Render("Path: ") + valueStyle.Render(m.item.Path) + "\n"
	// if m.item.Provider != "" {
	//     s += labelStyle.Render("Provider: ") + valueStyle.Render(m.item.Provider) + "\n"
	// }
```

#### Step 3: Update View() to use split rendering

Replace the `content := m.renderContent()` / scroll logic in `View()`:

```go
func (m detailModel) View() string {
	pinned, body := m.renderContentSplit()

	pinnedLines := strings.Split(pinned, "\n")
	pinnedHeight := len(pinnedLines)

	bodyLines := strings.Split(body, "\n")
	helpBar := m.renderHelp()

	messageLines := 0
	if m.message != "" {
		messageLines = 1
	}

	// Scrollable area = total height minus pinned header, message, help bar, margins
	visibleHeight := m.height - pinnedHeight - messageLines - 2
	if visibleHeight < 1 {
		visibleHeight = len(bodyLines)
	}

	maxOffset := len(bodyLines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	offset := m.scrollOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + visibleHeight
	if end > len(bodyLines) {
		end = len(bodyLines)
	}

	var s string
	s = pinned // always show pinned header

	if offset > 0 {
		s += helpStyle.Render(fmt.Sprintf("(%d lines above)", offset)) + "\n"
		s += strings.Join(bodyLines[offset:end], "\n")
	} else {
		s += strings.Join(bodyLines[offset:end], "\n")
	}

	if end < len(bodyLines) {
		s += "\n" + helpStyle.Render(fmt.Sprintf("(%d lines below)", len(bodyLines)-end))
	}

	if m.message != "" {
		if m.messageIsErr {
			s += "\n" + errorMsgStyle.Render("Error: "+m.message)
		} else {
			s += "\n" + successMsgStyle.Render("Done: "+m.message)
		}
	}

	s += "\n" + helpBar
	return s
}
```

#### Step 4: Update clampScroll() to account for pinned header height

```go
func (m *detailModel) clampScroll() {
	pinned, body := m.renderContentSplit()
	pinnedLines := strings.Split(pinned, "\n")
	bodyLines := strings.Split(body, "\n")

	visibleHeight := m.height - len(pinnedLines) - 2
	if visibleHeight < 1 {
		visibleHeight = len(bodyLines)
	}

	maxOffset := len(bodyLines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}
```

#### Step 5: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

## Group 5: Mouse Support

### Task 5.1: Initialize bubblezone in main.go

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/cmd/nesco/main.go` (around line 196)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/styles_test.go`

**Depends on:** Task 1.3

**Success Criteria:**
- [ ] `zone.NewGlobal()` is called before `tea.NewProgram`
- [ ] `tea.WithMouseCellMotion()` is passed to `tea.NewProgram`
- [ ] `go build ./...` passes

#### Step 0: Write failing test

```go
// cli/internal/tui/styles_test.go
func TestBubblezoneImportable(t *testing.T) {
	// Verify bubblezone is available as a dependency.
	// The real integration test is go build ./... succeeding with the import in main.go.
	// This compile-time check uses the zone package to confirm it's in go.mod.
	_ = zone.NewGlobal
	t.Log("bubblezone package is importable")
}
```

(Add `import zone "github.com/lrstanley/bubblezone"` to styles_test.go imports.)

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (zone import not yet in styles_test.go, or zone.NewGlobal referenced before Task 1.3 completes)

#### Step 1: Add import and initialization in main.go's runTUI function

Locate the `tea.NewProgram(...)` call and update it:

```go
import zone "github.com/lrstanley/bubblezone"

// In runTUI(), before tea.NewProgram:
zone.NewGlobal()

p := tea.NewProgram(app,
    tea.WithAltScreen(),
    tea.WithMouseCellMotion(),
)
```

#### Step 2: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go build ./...`
Expected: PASS

---

### Task 5.2: Add zone.Mark() to Sidebar Rendering

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/sidebar.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/sidebar_test.go`

**Depends on:** Task 5.1, Task 2.1

**Success Criteria:**
- [ ] Each sidebar row is wrapped with `zone.Mark("sidebar-N", ...)` where N is the item index
- [ ] Clicking a sidebar item sets `sidebar.cursor` to that index
- [ ] `go build ./...` passes

#### Step 0: Write failing test

```go
// cli/internal/tui/sidebar_test.go
func TestSidebarViewHasZoneMarks(t *testing.T) {
	m := sidebarModel{focused: true}
	view := m.View()
	// zone.Mark() wraps content with invisible zone identifiers visible in raw strings
	// The zone package uses special escape sequences; the mark ID appears in the raw output
	if !strings.Contains(view, "sidebar-") {
		t.Error("sidebarModel.View() should contain zone marks with 'sidebar-' prefix")
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (no zone.Mark calls in View() yet)

#### Step 1: Add zone import to sidebar.go

```go
import zone "github.com/lrstanley/bubblezone"
```

#### Step 2: Wrap each sidebar row

In `sidebarModel.View()`, change each row render:

```go
rowContent := "  " + itemStyle.Render(line)
if i == m.cursor {
    rowContent = "▸ " + selectedItemStyle.Render(line)
}
s += zone.Mark(fmt.Sprintf("sidebar-%d", i), rowContent) + "\n"
```

Do the same for utility items, using indices `len(m.types)` through `len(m.types)+3`.

#### Step 3: Handle tea.MouseMsg in App.Update()

Add a `case tea.MouseMsg:` handler in App.Update(), before `case tea.KeyMsg:`:

```go
case tea.MouseMsg:
    if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
        return a, nil
    }
    // Check sidebar zones
    for i := 0; i < a.sidebar.totalItems(); i++ {
        if zone.Get(fmt.Sprintf("sidebar-%d", i)).InBounds(msg) {
            a.sidebar.cursor = i
            a.focus = focusSidebar
            // Synthesize Enter to load content
            return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
        }
    }
    return a, nil
```

#### Step 4: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 5.3: Add zone.Mark() to Items List and Detail Tabs

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/items.go` (View method)
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_render.go` (renderTabBar)
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go` (MouseMsg handler + App.View return)
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_render_test.go`

**Depends on:** Task 5.2

**Success Criteria:**
- [ ] Clicking an item row in the items list opens its detail view
- [ ] Clicking a tab in the detail view switches to that tab
- [ ] `go build ./...` passes

#### Step 0: Write failing test

```go
// cli/internal/tui/detail_render_test.go
func TestRenderTabBarHasZoneMarks(t *testing.T) {
	m := detailModel{
		item:      catalog.ContentItem{Name: "test", Type: catalog.Prompts},
		activeTab: tabOverview,
	}
	tabBar := m.renderTabBar()
	// zone.Mark embeds the ID string in the output
	if !strings.Contains(tabBar, "tab-") {
		t.Error("renderTabBar() should contain zone marks with 'tab-' prefix")
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (no zone.Mark in renderTabBar yet)

#### Step 1: Wrap item rows in items.go View()

Add import: `zone "github.com/lrstanley/bubblezone"`

In the row rendering loop:

```go
rowStr := fmt.Sprintf("%s%s%s  %s%s  %s\n", prefix, styledName, typeTag, localPrefix, helpStyle.Render(paddedDesc), provCells[i].styled)
s += zone.Mark(fmt.Sprintf("item-%d", i), rowStr)
```

#### Step 2: Handle item clicks in App.Update() MouseMsg handler

Add to the `case tea.MouseMsg:` handler:

```go
    // Check item list zones
    if a.screen == screenItems {
        for i := range a.items.items {
            if zone.Get(fmt.Sprintf("item-%d", i)).InBounds(msg) {
                a.items.cursor = i
                a.focus = focusContent
                return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
            }
        }
    }
```

#### Step 3: Wrap tab bar entries in detail_render.go renderTabBar()

```go
import zone "github.com/lrstanley/bubblezone"

func (m detailModel) renderTabBar() string {
    tabs := []struct {
        label string
        tab   detailTab
    }{
        {"Overview", tabOverview},
        {"Files", tabFiles},
        {"Install", tabInstall},
    }

    var parts []string
    for _, t := range tabs {
        label := t.label
        var rendered string
        if t.tab == m.activeTab {
            rendered = selectedItemStyle.Render("[" + label + "]")
        } else {
            rendered = helpStyle.Render(" " + label + " ")
        }
        parts = append(parts, zone.Mark(fmt.Sprintf("tab-%d", int(t.tab)), rendered))
    }

    return strings.Join(parts, "  ")
}
```

#### Step 4: Handle tab clicks in App.Update() MouseMsg handler

```go
    // Check detail tab zones
    if a.screen == screenDetail {
        for i := 0; i < 3; i++ {
            if zone.Get(fmt.Sprintf("tab-%d", i)).InBounds(msg) {
                a.detail.activeTab = detailTab(i)
                a.detail.scrollOffset = 0
                return a, nil
            }
        }
    }
```

#### Step 5: Wrap App.View() output with zone.Scan()

At the end of `App.View()`, change:

```go
// Before:
return fmt.Sprintf("\n%s\n", body)

// After:
return zone.Scan(fmt.Sprintf("\n%s\n", body))
```

#### Step 6: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

## Group 6: Modal System

### Task 6.1: Create modal.go with ConfirmModal Component

**Files:**
- Create: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal_test.go`

**Depends on:** Task 1.3

**Success Criteria:**
- [ ] `confirmModal` struct compiles
- [ ] `confirmModal.View()` renders a centered bordered box with title, body text, and key hints
- [ ] `confirmModal.Update()` handles Enter (confirm) and Esc (cancel)
- [ ] `go build ./...` passes

#### Step 0: Write failing test

```go
// cli/internal/tui/modal_test.go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfirmModalViewContainsTitle(t *testing.T) {
	m := newConfirmModal("Delete item?", "This cannot be undone.")
	view := m.View()
	if !strings.Contains(view, "Delete item?") {
		t.Error("confirmModal.View() should contain the title text")
	}
	if !strings.Contains(view, "This cannot be undone.") {
		t.Error("confirmModal.View() should contain the body text")
	}
}

func TestConfirmModalEnterConfirms(t *testing.T) {
	m := newConfirmModal("Confirm?", "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.confirmed {
		t.Error("Enter should set confirmed=true")
	}
	if updated.active {
		t.Error("Enter should set active=false")
	}
}

func TestConfirmModalEscCancels(t *testing.T) {
	m := newConfirmModal("Confirm?", "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.confirmed {
		t.Error("Esc should leave confirmed=false")
	}
	if updated.active {
		t.Error("Esc should set active=false")
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (confirmModal undefined)

#### Step 1: Write modal.go

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

// confirmModal is a centered confirmation dialog.
// It wraps bubbletea-overlay for positioning.
type confirmModal struct {
	title    string
	body     string // multi-line body text
	active   bool
	confirmed bool
}

func newConfirmModal(title, body string) confirmModal {
	return confirmModal{title: title, body: body, active: true}
}

func (m confirmModal) Update(msg tea.Msg) (confirmModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.confirmed = true
			m.active = false
		case tea.KeyEsc:
			m.confirmed = false
			m.active = false
		}
		switch msg.String() {
		case "y", "Y":
			m.confirmed = true
			m.active = false
		case "n", "N":
			m.confirmed = false
			m.active = false
		}
	}
	return m, nil
}

func (m confirmModal) View() string {
	if !m.active {
		return ""
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(40)

	content := labelStyle.Render(m.title) + "\n\n"
	if m.body != "" {
		content += valueStyle.Render(m.body) + "\n\n"
	}
	content += helpStyle.Render("[Enter/y] Confirm   [Esc/n] Cancel")

	return modalStyle.Render(content)
}

// overlayView returns the modal centered over the given background content,
// using bubbletea-overlay for positioning.
func (m confirmModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)
}
```

#### Step 2: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 6.2: Add Modal Field to App Struct and Route Modal Input

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal_test.go`

**Depends on:** Task 6.1, Task 3.3

**Success Criteria:**
- [ ] `App` has a `modal confirmModal` field
- [ ] When `modal.active`, all key input routes to `modal.Update()` first
- [ ] `focusModal` is set when a modal opens; cleared when it closes
- [ ] `go build ./...` passes

#### Step 0: Write failing test

```go
// cli/internal/tui/modal_test.go
func TestAppModalCapturesInput(t *testing.T) {
	// When modal is active, key input should not reach the screen handler
	a := App{
		width:  80,
		height: 24,
		screen: screenCategory,
		focus:  focusModal,
		modal:  newConfirmModal("Test?", ""),
	}
	// Pressing 'q' (quit) should be intercepted by the modal, not quit the app
	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	result, _ := a.Update(qMsg)
	updated := result.(App)
	// Modal should have closed (n = cancel) but app should not have quit
	if updated.modal.active {
		t.Error("modal should be inactive after pressing 'n'")
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (App.modal field undefined)

#### Step 1: Add modal field to App struct

```go
	modal confirmModal
```

#### Step 2: Add modal routing at the top of the KeyMsg handler

At the start of `case tea.KeyMsg:` (after ctrl+c), before the search handling:

```go
		// If a modal is active, route all input to it
		if a.modal.active {
			var cmd tea.Cmd
			a.modal, cmd = a.modal.Update(msg)
			if !a.modal.active {
				a.focus = focusContent // return focus after dismiss
				// Modal result handling happens in Task 6.3+
			}
			return a, cmd
		}
```

#### Step 3: Render modal overlay in App.View()

After composing `panels` and `footer` in View(), add:

```go
	if a.modal.active {
		body = a.modal.overlayView(body)
	}
```

#### Step 4: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 6.3: Convert Install Flow to Modal

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal_test.go`

**Depends on:** Task 6.2

**Success Criteria:**
- [ ] Pressing `i` on the Install tab opens a `confirmModal` ("Install [name]?")
- [ ] Confirming the modal triggers the install
- [ ] Canceling the modal returns to detail view with no change
- [ ] The existing inline method picker (actionChooseMethod) is replaced by modal flow for symlink vs copy selection

This replaces `actionChooseMethod` → `confirmModal`. The method picker becomes a modal step.

#### Step 0: Write failing test

```go
// cli/internal/tui/modal_test.go
func TestOpenModalMsgOpensModal(t *testing.T) {
	// Sending openModalMsg to App should open the confirmModal
	a := App{
		width:  80,
		height: 24,
		screen: screenDetail,
	}
	msg := openModalMsg{
		purpose: modalInstall,
		title:   "Install test-tool?",
		body:    "Install using symlink or copy.",
	}
	result, _ := a.Update(msg)
	updated := result.(App)
	if !updated.modal.active {
		t.Error("openModalMsg should set modal.active=true")
	}
	if updated.modal.purpose != modalInstall {
		t.Errorf("modal purpose should be modalInstall, got %d", updated.modal.purpose)
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (openModalMsg undefined, modalInstall undefined)

#### Step 1: Define a modalPurpose type to track what the modal is confirming

In `modal.go`:

```go
type modalPurpose int

const (
	modalNone        modalPurpose = iota
	modalInstall
	modalUninstall
	modalSave
	modalPromote
	modalAppScript
)
```

Add `purpose modalPurpose` to the `confirmModal` struct.

#### Step 2: In App.Update(), after modal closes with confirmed=true, handle modalInstall

```go
		if !a.modal.active && a.modal.confirmed {
			switch a.modal.purpose {
			case modalInstall:
				a.detail.doInstallChecked()
			case modalUninstall:
				a.detail.doUninstallAll()
			case modalPromote:
				// trigger promote command
				repoRoot := a.catalog.RepoRoot
				item := a.detail.item
				return a, func() tea.Msg {
					result, err := promote.Promote(repoRoot, item)
					return promoteDoneMsg{result: result, err: err}
				}
			}
		}
```

#### Step 3: In detail.go Update(), when `keys.Install` is pressed, send a message to App to open modal

Rather than setting `confirmAction = actionChooseMethod` directly, emit a new message type:

```go
type openModalMsg struct {
	purpose modalPurpose
	title   string
	body    string
}
```

In `detail.Update()`, replace the `startInstall()` call path:

```go
case key.Matches(msg, keys.Install):
	if m.activeTab != tabInstall {
		break
	}
	// ... (existing Apps handling stays) ...
	// For non-prompt, non-app items that need install:
	return m, func() tea.Msg {
		return openModalMsg{
			purpose: modalInstall,
			title:   fmt.Sprintf("Install %q?", m.item.Name),
			body:    "Install to checked providers using symlink or copy.",
		}
	}
```

#### Step 4: Handle openModalMsg in App.Update()

```go
case openModalMsg:
	a.modal = newConfirmModal(msg.title, msg.body)
	a.modal.purpose = msg.purpose
	a.focus = focusModal
	return a, nil
```

#### Step 5: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 6.4: Convert Uninstall, Save, Promote, and App Script to Modals

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal_test.go`

**Depends on:** Task 6.3

**Success Criteria:**
- [ ] Pressing `u` opens an uninstall confirmation modal instead of inline `actionUninstall`
- [ ] Pressing `p` opens a promote confirmation modal instead of inline `actionPromoteConfirm`
- [ ] Pressing `i` on an App opens an app script confirmation modal instead of inline `actionAppScriptConfirm`
- [ ] Save (`s`) continues using textinput inline (not a modal — modal would need embedded textinput which adds complexity)
- [ ] All existing `confirmAction` paths for these actions are replaced

#### Step 0: Write failing test

```go
// cli/internal/tui/modal_test.go
func TestModalPurposesAreDefined(t *testing.T) {
	// All required modal purposes must be defined and distinct
	purposes := []modalPurpose{
		modalNone, modalInstall, modalUninstall, modalSave, modalPromote, modalAppScript,
	}
	seen := map[modalPurpose]bool{}
	for _, p := range purposes {
		if seen[p] {
			t.Errorf("modalPurpose %d is duplicated", p)
		}
		seen[p] = true
	}
	if len(seen) != 6 {
		t.Errorf("expected 6 distinct modal purposes, got %d", len(seen))
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (modalSave, modalPromote, modalAppScript undefined if not yet added in Task 6.3)

#### Step 1: Replace actionUninstall emit

In `detail.Update()`, `case key.Matches(msg, keys.Uninstall)`:

```go
// Replace:
// m.confirmAction = actionUninstall
// With:
installed := m.installedProviders()
var names []string
for _, p := range installed {
    names = append(names, p.Name)
}
return m, func() tea.Msg {
    return openModalMsg{
        purpose: modalUninstall,
        title:   fmt.Sprintf("Uninstall %q?", m.item.Name),
        body:    fmt.Sprintf("Remove from: %s", strings.Join(names, ", ")),
    }
}
```

#### Step 2: Replace actionPromoteConfirm emit

```go
// Replace:
// m.confirmAction = actionPromoteConfirm
// With:
return m, func() tea.Msg {
    return openModalMsg{
        purpose: modalPromote,
        title:   fmt.Sprintf("Promote %q to shared?", m.item.Name),
        body:    "Creates a branch, commits, pushes, and opens a PR.",
    }
}
```

#### Step 3: Replace actionAppScriptConfirm emit

```go
// Replace:
// m.confirmAction = actionAppScriptConfirm
// With:
return m, func() tea.Msg {
    scriptPreview := loadScriptPreview(m.item.Path)
    return openModalMsg{
        purpose: modalAppScript,
        title:   "Run install.sh?",
        body:    "WARNING: executes a shell script.\n\n" + scriptPreview,
    }
}
```

Add `loadScriptPreview` as a package-level helper in `detail.go`:

```go
func loadScriptPreview(itemPath string) string {
    data, err := os.ReadFile(filepath.Join(itemPath, "install.sh"))
    if err != nil {
        return "(script not found)"
    }
    lines := strings.Split(string(data), "\n")
    if len(lines) > 10 {
        lines = lines[:10]
    }
    return strings.Join(lines, "\n")
}
```

#### Step 4: Handle modalAppScript in App.Update() after confirm

`runAppScriptCmd()` does not yet exist as a method. The existing `runAppScript()` at `detail.go` line 733 is a pointer-receiver method returning `tea.Cmd`. Add a thin public wrapper to `detail.go`:

```go
// runAppScriptCmd is a public alias for runAppScript, called from App when the
// app script modal is confirmed.
func (m *detailModel) runAppScriptCmd() (*detailModel, tea.Cmd) {
    cmd := m.runAppScript()
    return m, cmd
}
```

Then in `App.Update()` after modal confirms:

```go
case modalAppScript:
    var cmd tea.Cmd
    cmd = a.detail.runAppScript()
    return a, cmd
```

Alternatively, since `runAppScript()` is already accessible from `App.Update()` (same package), call it directly without a wrapper:

```go
case modalAppScript:
    return a, a.detail.runAppScript()
```

#### Step 5: Remove inline confirmation rendering from detail_render.go

In `renderInstallTab()`, delete the `case actionUninstall:` and `case actionPromoteConfirm:` rendering blocks. Also delete the `case actionAppScriptConfirm:` rendering block. These are now shown as modals. Do not use line numbers as references — the file will have shifted after Task 4.1 rewrites it. Identify the blocks by their `case` labels instead.

#### Step 6: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 6.5: Convert Save Prompt to Modal

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal_test.go`

**Depends on:** Task 6.4

**Success Criteria:**
- [ ] Pressing `s` in the detail view opens a modal with an embedded `textinput` component for the filename
- [ ] `modalSave` purpose is defined in the `modalPurpose` enum (already listed in Task 6.3 Step 1)
- [ ] Confirming the modal triggers the save with the entered filename
- [ ] Canceling closes the modal with no change
- [ ] The existing inline textinput save flow in `detail.go` is removed

The design (section 5 Flow 2, section 6 action table) specifies save prompt as a modal with text input. This task implements it as a modal by embedding a `bubbles/textinput` inside a new `saveModal` struct (separate from `confirmModal` since it needs text input state).

#### Step 0: Write failing test

```go
// cli/internal/tui/modal_test.go
func TestSaveModalViewContainsInput(t *testing.T) {
	m := newSaveModal("filename.md")
	view := m.View()
	if !strings.Contains(view, "Save prompt as:") {
		t.Error("saveModal.View() should contain 'Save prompt as:' label")
	}
}

func TestSaveModalEnterWithValueConfirms(t *testing.T) {
	m := newSaveModal("filename.md")
	// Type a filename into the input
	m.input.SetValue("my-prompt.md")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.confirmed {
		t.Error("Enter with non-empty input should set confirmed=true")
	}
	if updated.value != "my-prompt.md" {
		t.Errorf("value should be 'my-prompt.md', got %q", updated.value)
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (saveModal undefined, newSaveModal undefined)

#### Step 1: Add saveModal struct to modal.go

```go
import "github.com/charmbracelet/bubbles/textinput"

// saveModal is a modal dialog with a text input for entering a filename.
type saveModal struct {
	active    bool
	input     textinput.Model
	confirmed bool
	value     string // set on confirm
}

func newSaveModal(placeholder string) saveModal {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 36
	return saveModal{active: true, input: ti}
}

func (m saveModal) Update(msg tea.Msg) (saveModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if strings.TrimSpace(m.input.Value()) != "" {
				m.value = strings.TrimSpace(m.input.Value())
				m.confirmed = true
				m.active = false
				return m, nil
			}
		case tea.KeyEsc:
			m.active = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m saveModal) View() string {
	if !m.active {
		return ""
	}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(40)
	content := labelStyle.Render("Save prompt as:") + "\n\n"
	content += m.input.View() + "\n\n"
	content += helpStyle.Render("[Enter] Save   [Esc] Cancel")
	return modalStyle.Render(content)
}

func (m saveModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)
}
```

#### Step 2: Add saveModal field to App struct and routing

In `app.go`, add alongside `modal confirmModal`:

```go
	saveModal saveModal
```

Add `doSavePrompt` to `detail.go` — this method does not currently exist. The existing save flow uses `doSave()` at line 772 which reads `m.savePath` and `m.methodCursor`. The modal-driven flow sets the filename externally, so add:

```go
// doSavePrompt sets the save path from the modal input value and triggers save.
// It replaces the inline actionSavePath/actionSaveMethod flow.
func (m *detailModel) doSavePrompt(filename string) (*detailModel, tea.Cmd) {
    m.savePath = filename
    // Default to symlink (methodCursor 0); user chose via modal, not method picker
    m.methodCursor = 0
    return m, m.doSave()
}
```

In the `KeyMsg` handler, after the `confirmModal` routing block:

```go
		if a.saveModal.active {
			var cmd tea.Cmd
			a.saveModal, cmd = a.saveModal.Update(msg)
			if !a.saveModal.active && a.saveModal.confirmed {
				cmd = a.detail.doSave()
				a.detail.savePath = a.saveModal.value
				a.focus = focusContent
			} else if !a.saveModal.active {
				a.focus = focusContent
			}
			return a, cmd
		}
```

In `App.View()`, after the `confirmModal` overlay check:

```go
	if a.saveModal.active {
		body = a.saveModal.overlayView(body)
	}
```

#### Step 3: Emit openSaveModalMsg from detail.go

Add a new message type alongside `openModalMsg`:

```go
type openSaveModalMsg struct{}
```

In `detail.Update()`, replace the inline textinput save flow triggered by `keys.Save`:

```go
case key.Matches(msg, keys.Save):
	return m, func() tea.Msg { return openSaveModalMsg{} }
```

#### Step 4: Handle openSaveModalMsg in App.Update()

```go
case openSaveModalMsg:
	a.saveModal = newSaveModal("filename.md")
	a.focus = focusModal
	return a, nil
```

#### Step 5: Remove inline save textinput from detail.go

Delete the `actionSavePath` and `actionSaveMethod` confirm paths from `detail.Update()` (the block at ~lines 178-194 handling `actionSavePath`, and the `actionSaveMethod` handling inside the `keys.Enter` case). Also delete their rendering blocks from `detail_render.go` (identify by the `case actionSavePath:` and `case actionSaveMethod:` labels — do not rely on line numbers as they shift after Task 4.1 rewrites the file). The save flow is now fully modal-driven.

#### Step 6: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 6.6: Implement Env Setup Modal Wizard

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/modal_test.go`

**Depends on:** Task 6.5

**Success Criteria:**
- [ ] Pressing `e` in the detail view opens a multi-step env setup modal wizard
- [ ] Step 1: Select environment type (j/k to navigate, Enter to select)
- [ ] Step 2: Configure paths/settings (textinput for each required field)
- [ ] Step 3: Confirm and apply (shows summary, Enter to apply, Esc to go back)
- [ ] Modal closes after apply; detail view shows updated env status
- [ ] `go build ./...` passes

The design (section 3 component hierarchy: EnvSetupModal, section 5 Flow 4) specifies a multi-step env setup wizard as a modal. This matches the existing `detail_env.go` sub-model's logic but wraps it in a modal overlay.

#### Step 0: Write failing test

```go
// cli/internal/tui/modal_test.go
func TestEnvSetupModalStep1Navigation(t *testing.T) {
	m := newEnvSetupModal([]string{"claude", "cursor", "windsurf"})
	if m.step != envStepSelectType {
		t.Errorf("initial step should be envStepSelectType, got %d", m.step)
	}
	// Down arrow should move cursor
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.cursor != 1 {
		t.Errorf("Down should move cursor to 1, got %d", updated.cursor)
	}
}

func TestEnvSetupModalStep1EnterAdvances(t *testing.T) {
	m := newEnvSetupModal([]string{"claude"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != envStepConfigure {
		t.Errorf("Enter on step 1 should advance to envStepConfigure, got %d", updated.step)
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (newEnvSetupModal undefined, envStepSelectType undefined)

#### Step 1: Add envSetupModal struct to modal.go

```go
type envSetupStep int

const (
	envStepSelectType envSetupStep = iota
	envStepConfigure
	envStepConfirm
)

// envSetupModal is a multi-step wizard for configuring environment variables.
type envSetupModal struct {
	active   bool
	step     envSetupStep
	envTypes []string       // e.g. ["claude", "cursor", "windsurf"]
	cursor   int            // selected env type in step 1
	inputs   []textinput.Model // config fields in step 2
	inputIdx int            // focused input in step 2
	applied  bool
}

func newEnvSetupModal(envTypes []string) envSetupModal {
	return envSetupModal{
		active:   true,
		step:     envStepSelectType,
		envTypes: envTypes,
	}
}

func (m envSetupModal) Update(msg tea.Msg) (envSetupModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case envStepSelectType:
			switch msg.Type {
			case tea.KeyUp:
				if m.cursor > 0 {
					m.cursor--
				}
			case tea.KeyDown:
				if m.cursor < len(m.envTypes)-1 {
					m.cursor++
				}
			case tea.KeyEnter:
				// Build config inputs for selected env type
				m.inputs = buildEnvInputs(m.envTypes[m.cursor])
				m.inputIdx = 0
				if len(m.inputs) > 0 {
					m.inputs[0].Focus()
				}
				m.step = envStepConfigure
			case tea.KeyEsc:
				m.active = false
			}
		case envStepConfigure:
			switch msg.Type {
			case tea.KeyEnter:
				m.step = envStepConfirm
			case tea.KeyEsc:
				m.step = envStepSelectType
			case tea.KeyTab:
				m.inputs[m.inputIdx].Blur()
				m.inputIdx = (m.inputIdx + 1) % len(m.inputs)
				m.inputs[m.inputIdx].Focus()
			}
			if m.inputIdx < len(m.inputs) {
				var cmd tea.Cmd
				m.inputs[m.inputIdx], cmd = m.inputs[m.inputIdx].Update(msg)
				return m, cmd
			}
		case envStepConfirm:
			switch msg.Type {
			case tea.KeyEnter:
				m.applied = true
				m.active = false
			case tea.KeyEsc:
				m.step = envStepConfigure
			}
		}
	}
	return m, nil
}

func (m envSetupModal) View() string {
	if !m.active {
		return ""
	}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(44)

	var content string
	switch m.step {
	case envStepSelectType:
		content = labelStyle.Render("Step 1: Select environment type") + "\n\n"
		for i, t := range m.envTypes {
			if i == m.cursor {
				content += selectedItemStyle.Render("▸ "+t) + "\n"
			} else {
				content += "  " + t + "\n"
			}
		}
		content += "\n" + helpStyle.Render("[↑↓] Navigate   [Enter] Select   [Esc] Cancel")
	case envStepConfigure:
		content = labelStyle.Render("Step 2: Configure "+m.envTypes[m.cursor]) + "\n\n"
		for _, inp := range m.inputs {
			content += inp.View() + "\n"
		}
		content += "\n" + helpStyle.Render("[Tab] Next field   [Enter] Continue   [Esc] Back")
	case envStepConfirm:
		content = labelStyle.Render("Step 3: Confirm") + "\n\n"
		content += valueStyle.Render("Apply environment settings for "+m.envTypes[m.cursor]+"?") + "\n\n"
		content += helpStyle.Render("[Enter] Apply   [Esc] Back")
	}
	return modalStyle.Render(content)
}

func (m envSetupModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)
}

// buildEnvInputs returns textinput models for the fields required by the given env type.
// The actual field list comes from the catalog env spec for that provider.
func buildEnvInputs(envType string) []textinput.Model {
	// Fields are defined per-provider; this is a placeholder for the real implementation.
	// The detail_env.go model already knows what fields each provider needs.
	ti := textinput.New()
	ti.Placeholder = "Path or value"
	ti.CharLimit = 200
	ti.Width = 36
	return []textinput.Model{ti}
}
```

#### Step 2: Add envSetupModal field to App struct and routing

In `app.go`, add:

```go
	envModal envSetupModal
```

In the `KeyMsg` handler, after the `saveModal` routing block:

```go
		if a.envModal.active {
			var cmd tea.Cmd
			a.envModal, cmd = a.envModal.Update(msg)
			if !a.envModal.active {
				if a.envModal.applied {
					// Re-initialize env sub-model so it re-reads current OS env vars.
					// env.Refresh() does not exist; instead re-create the envSetupModel
					// from the detail item, which re-reads the environment on construction.
					a.detail.env = newEnvSetupModel(a.detail.item)
				}
				a.focus = focusContent
			}
			return a, cmd
		}
```

> **Note:** `a.detail.env.Refresh()` was originally written here but `envSetupModel` has no `Refresh()` method. The replacement above re-constructs the env sub-model, which re-reads current env vars. Verify that `newEnvSetupModel(item)` is the correct constructor call by checking `detail_env.go`; adjust the constructor name/args as needed.

In `App.View()`, after the `saveModal` overlay check:

```go
	if a.envModal.active {
		body = a.envModal.overlayView(body)
	}
```

#### Step 3: Emit openEnvModalMsg from detail.go

```go
type openEnvModalMsg struct {
	envTypes []string
}
```

In `detail.Update()`, when `keys.EnvSetup` is pressed:

```go
case key.Matches(msg, keys.EnvSetup):
	// env.AvailableTypes() does not exist on envSetupModel.
	// The env setup modal uses provider names as env type labels.
	// Derive them from the item's provider field, or use the env var names
	// already known to the detail_env.go sub-model (m.env.varNames).
	envTypes := m.env.varNames // slice of env var names this item needs
	return m, func() tea.Msg {
		return openEnvModalMsg{envTypes: envTypes}
	}
```

> **Note:** `m.env.varNames` is used here as a proxy for "available types." If the intent is to show named environment profiles (e.g., "claude", "cursor") rather than individual var names, derive the list from `m.item.Provider` or add a helper method `func (m envSetupModel) providerNames() []string` to `detail_env.go` that returns the provider names from the env config.

#### Step 4: Handle openEnvModalMsg in App.Update()

```go
case openEnvModalMsg:
	a.envModal = newEnvSetupModal(msg.envTypes)
	a.focus = focusModal
	return a, nil
```

#### Step 5: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 6.7: Add zone.Mark() to Detail Action Buttons

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_render.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/app.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_render_test.go`

**Depends on:** Task 5.3

**Success Criteria:**
- [ ] A visible action button bar (`[i]nstall  [u]ninstall  [c]opy  [s]ave`) is added to the install tab content area in `renderInstallTab()` (not just the help bar)
- [ ] Each button string is wrapped with `zone.Mark()`
- [ ] Clicking each button in the detail view triggers the same action as the corresponding keypress
- [ ] `go build ./...` passes

The design (section 5 Flow 6) specifies: "Click action button in detail → triggers that action (modal if needed)." Task 5.3 handles sidebar, item list, and tab clicks but omits the action buttons.

**Important:** The current codebase has no unified action button bar in the content area — action hints appear only in `renderHelp()` as plain text at the bottom of the screen. This task must first **add** a rendered button bar to the install tab content, then wrap those buttons with zone marks. Wrapping the help bar text in `renderHelp()` is not sufficient — the mouse hit zones would cover the footer rather than clearly-labeled in-content buttons.

#### Step 0: Write failing test

```go
// cli/internal/tui/detail_render_test.go
func TestRenderInstallTabHasActionButtons(t *testing.T) {
	m := detailModel{
		item:      catalog.ContentItem{Name: "test-tool", Type: catalog.Prompts},
		activeTab: tabInstall,
		width:     60,
		height:    24,
	}
	body := m.renderInstallTab()
	// Action bar must be present with zone marks
	if !strings.Contains(body, "detail-btn-install") {
		t.Error("renderInstallTab() should contain zone mark 'detail-btn-install'")
	}
	if !strings.Contains(body, "detail-btn-uninstall") {
		t.Error("renderInstallTab() should contain zone mark 'detail-btn-uninstall'")
	}
}
```

#### Step 0b: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: FAIL (no zone marks in renderInstallTab yet)

#### Step 1: Add an action button bar to renderInstallTab()

At the top of `renderInstallTab()` (after the normal action state guards), add a visible button bar to the rendered content:

```go
import zone "github.com/lrstanley/bubblezone"

// Action button bar — rendered in the content area so mouse clicks are targetable
installBtn := zone.Mark("detail-btn-install", helpStyle.Render("[i]nstall"))
uninstallBtn := zone.Mark("detail-btn-uninstall", helpStyle.Render("[u]ninstall"))
copyBtn := zone.Mark("detail-btn-copy", helpStyle.Render("[c]opy"))
saveBtn := zone.Mark("detail-btn-save", helpStyle.Render("[s]ave"))
actionBar := installBtn + "  " + uninstallBtn + "  " + copyBtn + "  " + saveBtn
s += actionBar + "\n\n"
```

The `actionBar` string is prepended to the install tab body so it appears as visible, clickable text in the content area above the install status lines.

#### Step 2: Handle action button clicks in App.Update() MouseMsg handler

Add to the `case tea.MouseMsg:` handler in `app.go`:

```go
	// Check detail action button zones
	if a.screen == screenDetail {
		btnChars := map[string]string{
			"detail-btn-install":   "i",
			"detail-btn-uninstall": "u",
			"detail-btn-copy":      "c",
			"detail-btn-save":      "s",
		}
		for zoneID, char := range btnChars {
			if zone.Get(zoneID).InBounds(msg) {
				return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(char)})
			}
		}
	}
```

> **Note:** The `btnKeys` map with `_ = btnKeys` pattern from the original draft has been removed — it was dead code. The zone IDs and synthetic key runes are sufficient.

#### Step 3: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/...`
Expected: PASS

---

### Task 1.4: Add WCAG AA Contrast Verification Test

**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/styles_test.go`

**Depends on:** Task 1.2

**Success Criteria:**
- [ ] A test function `TestColorsPassWCAGAA` verifies all semantic and panel colors achieve >= 4.5:1 contrast ratio on both light and dark backgrounds
- [ ] `go test ./internal/tui/...` passes

The design (section 6) documents specific contrast ratios for each color. The plan previously had no verification step.

#### Step 1: Add contrast ratio helper and test

```go
// contrastRatio calculates the WCAG contrast ratio between two hex colors (#RRGGBB).
// Returns the ratio (e.g. 4.5 for 4.5:1).
func contrastRatio(hex1, hex2 string) float64 {
	l1 := relativeLuminance(hex1)
	l2 := relativeLuminance(hex2)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	toLinear := func(c int64) float64 {
		s := float64(c) / 255.0
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*toLinear(r) + 0.7152*toLinear(g) + 0.0722*toLinear(b)
}

// TestColorsPassWCAGAA verifies all Romanesco palette colors achieve >= 4.5:1
// contrast ratio against their respective terminal background assumptions.
// Light colors are tested against white (#FFFFFF); dark colors against near-black (#18181B).
func TestColorsPassWCAGAA(t *testing.T) {
	const minRatio = 4.5
	lightBg := "#FFFFFF"
	darkBg := "#18181B"

	checks := []struct {
		name    string
		fg      string // foreground hex
		bg      string // background hex
		minRatio float64
	}{
		// Semantic colors (Light = on light terminal, Dark = on dark terminal)
		{"primaryColor light",  "#047857", lightBg, minRatio},
		{"primaryColor dark",   "#6EE7B7", darkBg,  minRatio},
		{"accentColor light",   "#6D28D9", lightBg, minRatio},
		{"accentColor dark",    "#C4B5FD", darkBg,  minRatio},
		{"mutedColor light",    "#57534E", lightBg, minRatio},
		{"mutedColor dark",     "#A8A29E", darkBg,  minRatio},
		{"successColor light",  "#15803D", lightBg, minRatio},
		{"successColor dark",   "#4ADE80", darkBg,  minRatio},
		{"dangerColor light",   "#B91C1C", lightBg, minRatio},
		{"dangerColor dark",    "#FCA5A5", darkBg,  minRatio},
		{"warningColor light",  "#B45309", lightBg, minRatio},
		{"warningColor dark",   "#FCD34D", darkBg,  minRatio},
		// Selected item: foreground against selectedBgColor
		{"selected mint on light bg", "#047857", "#D1FAE5", minRatio},
		{"selected mint on dark bg",  "#6EE7B7", "#1A3A2A", minRatio},
	}

	for _, c := range checks {
		ratio := contrastRatio(c.fg, c.bg)
		if ratio < c.minRatio {
			t.Errorf("%s: contrast %.2f:1 < %.1f:1 (fg=%s bg=%s)", c.name, ratio, c.minRatio, c.fg, c.bg)
		}
	}
}
```

---

## Summary of All Files

| File | Action |
|------|--------|
| `cli/internal/tui/styles.go` | Modify: Romanesco palette + panel styles |
| `cli/internal/tui/styles_test.go` | Modify: rename secondaryColor → accentColor, add panel color tests + WCAG AA test |
| `cli/internal/tui/sidebar.go` | Create: sidebarModel struct + View + Update |
| `cli/internal/tui/sidebar_test.go` | Create: sidebarModel unit tests |
| `cli/internal/tui/app_test.go` | Create: App struct, focus, View, Update unit tests |
| `cli/internal/tui/modal.go` | Create: confirmModal, saveModal, envSetupModal + overlay rendering |
| `cli/internal/tui/modal_test.go` | Create: modal unit tests |
| `cli/internal/tui/detail_render_test.go` | Create: renderContentSplit, renderTabBar, renderInstallTab unit tests |
| `cli/internal/tui/app.go` | Modify: focusTarget, App struct, View, Update (all modal routing) |
| `cli/internal/tui/category.go` | Delete: replaced by sidebar.go |
| `cli/internal/tui/detail.go` | Modify: emit openModalMsg, openSaveModalMsg, openEnvModalMsg instead of inline confirm |
| `cli/internal/tui/detail_render.go` | Modify: pinned header + scrollable body split; zone.Mark() on action buttons |
| `cli/internal/tui/items.go` | Modify: add zone.Mark() to rows |
| `cli/cmd/nesco/main.go` | Modify: zone.NewGlobal() + tea.WithMouseCellMotion() |
| `cli/go.mod` | Modify: add bubblezone + bubbletea-overlay |
| `cli/go.sum` | Modify: updated by go get |

---

## Execution Order

```
Task 1.3 (go get deps)          ←── parallel with 1.1
Task 1.1 (colors)
Task 1.2 (test update)          ←── after 1.1
Task 1.4 (WCAG AA contrast test) ←── after 1.2
Task 2.1 (sidebar.go)          ←── after 1.1
Task 2.2 (wire sidebar to App) ←── after 2.1
Task 3.1 (focusTarget)         ←── after 2.2
Task 3.2 (App.View refactor)   ←── after 3.1
Task 3.3 (App.Update refactor) ←── after 3.2
Task 3.4 (remove category.go)  ←── after 3.3
Task 4.1 (detail pinned header) ←── after 3.2 (needs content width set correctly)
Task 5.1 (zone in main.go)     ←── after 1.3
Task 5.2 (zone marks sidebar)  ←── after 5.1 + 2.1
Task 5.3 (zone marks items/tabs, zone.Scan) ←── after 5.2 + 4.1
Task 6.1 (modal.go)            ←── after 1.3
Task 6.2 (App modal field + routing) ←── after 6.1 + 3.3
Task 6.3 (install modal)       ←── after 6.2
Task 6.4 (uninstall/promote/app script modals) ←── after 6.3
Task 6.5 (save prompt modal)   ←── after 6.4
Task 6.6 (env setup modal wizard) ←── after 6.5
Task 6.7 (detail action button zones) ←── after 5.3 + 6.4
```
