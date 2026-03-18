package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestCreateLoadoutScreenSmoke(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("starts at types step when pre-filled", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		if s.step != clStepTypes {
			t.Errorf("step = %v, want clStepTypes", s.step)
		}
	})
	t.Run("starts at provider step when no prefill", func(t *testing.T) {
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		if s.step != clStepProvider {
			t.Errorf("step = %v, want clStepProvider", s.step)
		}
	})
	t.Run("split view initialized with wiz prefix", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		if s.splitView.zonePrefix != "wiz" {
			t.Errorf("splitView.zonePrefix = %q, want \"wiz\"", s.splitView.zonePrefix)
		}
	})
	t.Run("dest options include registry when scoped", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "my-reg", providers, cat, 80, 30)
		if len(s.destOptions) != 3 {
			t.Errorf("destOptions = %d, want 3 with registry scope", len(s.destOptions))
		}
	})
}

func TestCreateLoadoutScreenUpdate(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("enter on provider step advances to types", func(t *testing.T) {
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter)
		if s.step != clStepTypes {
			t.Errorf("step = %v, want clStepTypes after Enter", s.step)
		}
	})

	t.Run("esc on provider step does not crash", func(t *testing.T) {
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEsc)
		if s.confirmed {
			t.Error("confirmed should be false after Esc")
		}
	})

	t.Run("esc on types step cancels when provider pre-filled", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		// prefilledProvider is set by constructor; typeEntries populated too
		s.step = clStepTypes
		s, _ = s.Update(keyEsc)
		if s.confirmed {
			t.Error("Esc on types with prefilled provider should cancel (confirmed=false), not go back")
		}
	})

	t.Run("space on types toggles checkbox", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		if len(s.typeEntries) == 0 {
			t.Skip("no type entries")
		}
		initial := s.typeEntries[0].checked
		s, _ = s.Update(keySpace)
		if s.typeEntries[0].checked == initial {
			t.Error("space should toggle type checkbox")
		}
	})

	t.Run("enter on types with none selected shows error", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		// Uncheck all
		for i := range s.typeEntries {
			s.typeEntries[i].checked = false
		}
		s, _ = s.Update(keyEnter)
		if s.step != clStepTypes {
			t.Error("should stay on types step when none selected")
		}
		if s.message != "Select at least one content type" {
			t.Errorf("message = %q, want validation message", s.message)
		}
	})

	t.Run("enter on types advances to items", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter)
		if s.step != clStepItems {
			t.Errorf("step = %v, want clStepItems", s.step)
		}
	})

	t.Run("space on items step toggles selection", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // advance to items
		if s.step != clStepItems {
			t.Fatalf("expected clStepItems, got %d", s.step)
		}
		filtered := s.filteredTypeItems()
		if len(filtered) == 0 {
			t.Skip("no items")
		}
		idx := filtered[0]
		initial := s.entries[idx].selected
		s, _ = s.Update(keySpace)
		if s.entries[idx].selected == initial {
			t.Error("space should toggle item selection")
		}
	})

	t.Run("t key toggles showAllCompat", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		initial := s.showAllCompat
		s, _ = s.Update(keyRune('t'))
		if s.showAllCompat == initial {
			t.Error("t should toggle showAllCompat")
		}
	})

	t.Run("slash activates search", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		s, _ = s.Update(keyRune('/'))
		if !s.searchActive {
			t.Error("/ should activate search")
		}
	})

	t.Run("esc clears search when active", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		s.searchActive = true
		s.searchInput.Focus()
		s, _ = s.Update(keyEsc)
		if s.searchActive {
			t.Error("Esc should deactivate search")
		}
	})

	t.Run("l focuses preview pane", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		s, _ = s.Update(keyRune('l'))
		if s.splitView.focusedPane != panePreview {
			t.Error("l should focus preview pane")
		}
	})

	t.Run("h focuses list pane", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepItems
		s.buildTypeItemMaps()
		s.splitView.focusedPane = panePreview
		s, _ = s.Update(keyRune('h'))
		if s.splitView.focusedPane != paneList {
			t.Error("h should focus list pane")
		}
	})

	t.Run("a key selects all compatible items on items step", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		if s.step != clStepItems {
			t.Fatalf("expected clStepItems, got %d", s.step)
		}
		// Deselect all first
		ct := s.currentType()
		for _, idx := range s.typeItemMap[ct] {
			s.entries[idx].selected = false
		}
		// Toggle all on
		s, _ = s.Update(keyRune('a'))
		for _, idx := range s.typeItemMap[ct] {
			if !s.entries[idx].selected {
				t.Errorf("item %d should be selected after toggle-all", idx)
			}
		}
		// Toggle all off
		s, _ = s.Update(keyRune('a'))
		for _, idx := range s.typeItemMap[ct] {
			if s.entries[idx].selected {
				t.Errorf("item %d should be deselected after second toggle-all", idx)
			}
		}
	})

	t.Run("incompatible items render with suffix", func(t *testing.T) {
		// Use cursor provider which only supports Skills + Rules
		s := newCreateLoadoutScreen("cursor", "", providers, cat, 80, 30)
		// Manually set up a type entry for Agents (which cursor doesn't support)
		// to exercise the incompatible rendering path
		s.typeEntries = []typeCheckEntry{
			{ct: catalog.Agents, checked: true, count: 1},
		}
		s.step = clStepTypes
		s, _ = s.Update(keyEnter) // advance to items

		// Force showAllCompat to include items from typeItemMapAll
		s.showAllCompat = true
		got := s.View()
		if !strings.Contains(got, "(incompatible)") {
			t.Error("incompatible items should render with (incompatible) suffix")
		}
	})

	t.Run("search reduces visible items on items step", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		ct := s.currentType()
		allCount := len(s.filteredTypeItems())
		if allCount < 2 {
			t.Skip("need at least 2 items to test search filtering")
		}
		// Get the name of the first item to use as search query
		firstIdx := s.filteredTypeItems()[0]
		query := s.entries[firstIdx].item.Name

		s.perTypeSearch[ct] = query
		filtered := s.filteredTypeItems()
		if len(filtered) >= allCount {
			t.Error("search should reduce visible items")
		}
		if len(filtered) == 0 {
			t.Error("search for existing item name should return at least 1 result")
		}
	})

	t.Run("selections persist after search clear", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		ct := s.currentType()
		filtered := s.filteredTypeItems()
		if len(filtered) == 0 {
			t.Skip("no items")
		}
		// Select the first item
		idx := filtered[0]
		s.entries[idx].selected = true

		// Apply search, then clear it
		s.perTypeSearch[ct] = "nonexistent-query-xyz"
		s.perTypeSearch[ct] = ""

		// Selection should persist
		if !s.entries[idx].selected {
			t.Error("selection should persist after search clear")
		}
	})

	t.Run("esc from first type returns to type selection with selections preserved", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		if s.step != clStepItems {
			t.Fatalf("expected clStepItems, got %d", s.step)
		}
		// Select an item
		filtered := s.filteredTypeItems()
		if len(filtered) == 0 {
			t.Skip("no items")
		}
		idx := filtered[0]
		s.entries[idx].selected = true

		// Esc back to type selection
		s, _ = s.Update(keyEsc)
		if s.step != clStepTypes {
			t.Errorf("step = %v, want clStepTypes", s.step)
		}

		// Selection should be preserved
		if !s.entries[idx].selected {
			t.Error("item selection should be preserved after Esc to type selection")
		}
	})

	t.Run("back-nav preserves cursor position", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		ct := s.currentType()
		filtered := s.filteredTypeItems()
		if len(filtered) < 2 {
			t.Skip("need at least 2 items")
		}
		// Move cursor down
		s, _ = s.Update(keyDown)
		savedCursor := s.perTypeCursor[ct]
		if savedCursor != 1 {
			t.Fatalf("cursor should be 1, got %d", savedCursor)
		}

		// Advance to next type (or name if only 1 type)
		s, _ = s.Update(keyEnter)
		// Go back
		s, _ = s.Update(keyEsc)

		// Cursor should be preserved
		if s.perTypeCursor[ct] != savedCursor {
			t.Errorf("cursor = %d, want %d (should be preserved on back-nav)",
				s.perTypeCursor[ct], savedCursor)
		}
	})

	t.Run("per-type scroll offset preserved on back-nav", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		ct := s.currentType()

		// Manually set a scroll offset
		s.perTypeScroll[ct] = 5

		// Advance and go back
		s, _ = s.Update(keyEnter)
		s, _ = s.Update(keyEsc)

		if s.perTypeScroll[ct] != 5 {
			t.Errorf("scroll offset = %d, want 5 (should be preserved on back-nav)",
				s.perTypeScroll[ct])
		}
	})

	t.Run("empty type shows message and Enter advances", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		// Set up a type with no items by manually adding a fake type entry
		s.typeEntries = append(s.typeEntries, typeCheckEntry{
			ct:      catalog.ContentType("prompts"),
			checked: true,
			count:   0,
		})
		s.step = clStepTypes
		s, _ = s.Update(keyEnter) // advance to items

		// Navigate to the Prompts type step (last one)
		for s.currentType() != catalog.ContentType("prompts") && s.step == clStepItems {
			s, _ = s.Update(keyEnter)
		}
		if s.currentType() != catalog.ContentType("prompts") {
			t.Skip("could not reach Prompts type step")
		}

		got := s.View()
		if !strings.Contains(got, "No ") || !strings.Contains(got, "available") {
			t.Error("empty type should show 'No X available for Y' message")
		}

		// Enter should still advance past empty type
		s, _ = s.Update(keyEnter)
		if s.currentType() == catalog.ContentType("prompts") && s.step == clStepItems {
			t.Error("Enter should advance past empty type")
		}
	})

	t.Run("empty type Esc goes back", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.typeEntries = append(s.typeEntries, typeCheckEntry{
			ct:      catalog.ContentType("prompts"),
			checked: true,
			count:   0,
		})
		s.step = clStepTypes
		s, _ = s.Update(keyEnter) // advance to items

		// Navigate to the Prompts type step
		for s.currentType() != catalog.ContentType("prompts") && s.step == clStepItems {
			s, _ = s.Update(keyEnter)
		}
		if s.currentType() != catalog.ContentType("prompts") {
			t.Skip("could not reach Prompts type step")
		}

		prevTypeIndex := s.typeStepIndex
		s, _ = s.Update(keyEsc)
		if s.typeStepIndex != prevTypeIndex-1 {
			t.Errorf("typeStepIndex = %d, want %d after Esc",
				s.typeStepIndex, prevTypeIndex-1)
		}
	})

	t.Run("full flow through multiple types preserves all selections", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		nTypes := len(s.selectedTypes)
		if nTypes < 2 {
			t.Skip("need at least 2 types to test full flow")
		}

		// Select first item in each type step
		selectedIndices := make(map[int]bool)
		for i := 0; i < nTypes; i++ {
			filtered := s.filteredTypeItems()
			if len(filtered) > 0 {
				idx := filtered[0]
				s.entries[idx].selected = true
				selectedIndices[idx] = true
			}
			s, _ = s.Update(keyEnter) // advance to next type or name
		}
		if s.step != clStepName {
			t.Fatalf("expected clStepName, got %d", s.step)
		}

		// Verify all selections survived
		for idx := range selectedIndices {
			if !s.entries[idx].selected {
				t.Errorf("entry %d (%s) selection lost after full flow",
					idx, s.entries[idx].item.Name)
			}
		}

		// Verify selectedItems returns them
		selected := s.selectedItems()
		if len(selected) != len(selectedIndices) {
			t.Errorf("selectedItems() = %d, want %d", len(selected), len(selectedIndices))
		}
	})

	t.Run("enter advances through types to name", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		nTypes := len(s.selectedTypes)
		for i := 0; i < nTypes; i++ {
			s, _ = s.Update(keyEnter)
		}
		if s.step != clStepName {
			t.Errorf("step = %v, want clStepName after all types", s.step)
		}
	})

	t.Run("tab switches focus between name and desc", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		if !s.nameFirst {
			t.Fatal("nameFirst should be true initially")
		}
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		if s.nameFirst {
			t.Error("Tab should switch to desc")
		}
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		if !s.nameFirst {
			t.Error("Tab should switch back to name")
		}
	})

	t.Run("enter on name with empty shows error", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		s.nameInput.SetValue("")
		s, _ = s.Update(keyEnter)
		if s.step != clStepName {
			t.Error("should stay on name step with empty name")
		}
		if s.message != "name is required" {
			t.Errorf("message = %q", s.message)
		}
	})

	t.Run("enter on name with invalid chars shows error", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		s.nameInput.SetValue("../../evil")
		s, _ = s.Update(keyEnter)
		if s.step != clStepName {
			t.Error("should stay on name step with invalid name")
		}
		if !s.messageIsErr {
			t.Error("message should be error")
		}
	})

	t.Run("enter on name with leading dash shows error", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		s.nameInput.SetValue("-bad")
		s, _ = s.Update(keyEnter)
		if s.step != clStepName {
			t.Error("should stay on name step with leading dash")
		}
		if s.message != "name must not start with a dash" {
			t.Errorf("message = %q", s.message)
		}
	})

	t.Run("enter on name advances to dest", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		s.nameInput.SetValue("my-loadout")
		s, _ = s.Update(keyEnter)
		if s.step != clStepDest {
			t.Errorf("step = %v, want clStepDest", s.step)
		}
	})

	t.Run("enter on dest advances to review", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepDest
		s, _ = s.Update(keyEnter)
		if s.step != clStepReview {
			t.Errorf("step = %v, want clStepReview", s.step)
		}
		if s.reviewBtnCursor != 1 {
			t.Error("reviewBtnCursor should default to 1 (Create)")
		}
	})

	t.Run("review enter on Create confirms", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 1
		s, _ = s.Update(keyEnter)
		if !s.confirmed {
			t.Error("confirmed should be true after Create")
		}
	})

	t.Run("review enter on Back returns to dest", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 0
		s, _ = s.Update(keyEnter)
		if s.step != clStepDest {
			t.Errorf("step = %v, want clStepDest", s.step)
		}
	})

	t.Run("left/right switches review buttons", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 1
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyLeft})
		if s.reviewBtnCursor != 0 {
			t.Errorf("left should move to Back (0), got %d", s.reviewBtnCursor)
		}
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
		if s.reviewBtnCursor != 1 {
			t.Errorf("right should move to Create (1), got %d", s.reviewBtnCursor)
		}
	})

	t.Run("previewCmdForCursor returns valid cmd", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // types -> items
		cmd := s.previewCmdForCursor()
		if cmd == nil {
			// Could be nil if no items have files — that's OK
			return
		}
		msg := cmd()
		if _, ok := msg.(splitViewCursorMsg); !ok {
			t.Errorf("expected splitViewCursorMsg, got %T", msg)
		}
	})
}

func TestCreateLoadoutScreenView(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("types step renders breadcrumb", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		got := s.View()
		if got == "" {
			t.Error("View() returned empty string")
		}
		if !strings.Contains(got, "Create") {
			t.Error("breadcrumb should contain Create")
		}
	})

	t.Run("types step shows danger badges", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.typeEntries = []typeCheckEntry{
			{ct: catalog.Hooks, checked: true, count: 1},
			{ct: catalog.MCP, checked: true, count: 1},
			{ct: catalog.Skills, checked: true, count: 2},
		}
		got := s.View()
		if !strings.Contains(got, "!!") {
			t.Error("types step should show !! badge for Hooks and MCP")
		}
	})

	t.Run("items step shows content type label", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s, _ = s.Update(keyEnter) // advance to items
		got := s.View()
		if got == "" {
			t.Error("View() returned empty string")
		}
	})

	t.Run("name step renders inputs", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		got := s.View()
		if !strings.Contains(got, "Name your loadout") {
			t.Error("name step should show name prompt")
		}
	})

	t.Run("dest step renders options", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepDest
		got := s.View()
		if !strings.Contains(got, "destination") {
			t.Error("dest step should show destination prompt")
		}
	})

	t.Run("review step renders summary", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.nameInput.SetValue("my-loadout")
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "my-loadout") {
			t.Error("review should show loadout name")
		}
		if !strings.Contains(got, "Review") {
			t.Error("review should show Review heading")
		}
	})

	t.Run("review with security content shows notice", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.nameInput.SetValue("test")
		for i := range s.entries {
			if s.entries[i].item.Type == catalog.Hooks {
				s.entries[i].selected = true
			}
		}
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "Security Notice") {
			t.Error("review should show security warning when hooks selected")
		}
	})
}

func TestCreateLoadoutReview(t *testing.T) {
	providers := testProviders(t)

	t.Run("hooks with commands show in security callout", func(t *testing.T) {
		tmp := t.TempDir()
		hookDir := filepath.Join(tmp, "hooks", "claude-code", "my-hook")
		os.MkdirAll(hookDir, 0o755)
		os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(`{
			"event": "PostToolUse",
			"matcher": "Bash",
			"hooks": [{"type": "command", "command": "./lint.sh"}]
		}`), 0o644)
		cat := &catalog.Catalog{
			RepoRoot: tmp,
			Items: []catalog.ContentItem{{
				Name:     "my-hook",
				Type:     catalog.Hooks,
				Path:     hookDir,
				Provider: "claude-code",
				Files:    []string{"hook.json"},
			}},
		}
		// Wide terminal so commands aren't truncated by split view
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 120, 30)
		s.nameInput.SetValue("test")
		for i := range s.entries {
			s.entries[i].selected = true
		}
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "PostToolUse") {
			t.Error("security callout should show hook event name")
		}
		if !strings.Contains(got, "lint.sh") {
			t.Error("security callout should show hook command")
		}
	})

	t.Run("MCP with commands show in security callout", func(t *testing.T) {
		tmp := t.TempDir()
		mcpDir := filepath.Join(tmp, "mcp", "srv")
		os.MkdirAll(mcpDir, 0o755)
		os.WriteFile(filepath.Join(mcpDir, "config.json"), []byte(`{
			"type": "stdio",
			"command": "node",
			"args": ["srv.js"]
		}`), 0o644)
		cat := &catalog.Catalog{
			RepoRoot: tmp,
			Items: []catalog.ContentItem{{
				Name:  "srv",
				Type:  catalog.MCP,
				Path:  mcpDir,
				Files: []string{"config.json"},
			}},
		}
		// Wide terminal so commands aren't truncated by split view
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 120, 30)
		s.nameInput.SetValue("test")
		for i := range s.entries {
			s.entries[i].selected = true
		}
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "node") {
			t.Error("security callout should show MCP command")
		}
		if !strings.Contains(got, "srv.js") {
			t.Error("security callout should show MCP args")
		}
	})

	t.Run("zero items shows warning", func(t *testing.T) {
		cat := testCatalog(t)
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.nameInput.SetValue("test")
		// Don't select anything
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "No items selected") {
			t.Error("review with 0 items should show warning")
		}
	})

	t.Run("Back button returns to dest step", func(t *testing.T) {
		cat := testCatalog(t)
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 0 // Back
		s, _ = s.Update(keyEnter)
		if s.step != clStepDest {
			t.Errorf("step = %v, want clStepDest", s.step)
		}
	})

	t.Run("Esc returns to dest without confirming", func(t *testing.T) {
		cat := testCatalog(t)
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s, _ = s.Update(keyEsc)
		if s.step != clStepDest {
			t.Errorf("step = %v, want clStepDest", s.step)
		}
		if s.confirmed {
			t.Error("Esc should not set confirmed")
		}
	})

	t.Run("Create sets confirmed true", func(t *testing.T) {
		cat := testCatalog(t)
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 1 // Create
		s, _ = s.Update(keyEnter)
		if !s.confirmed {
			t.Error("Create should set confirmed=true")
		}
	})

	t.Run("long item list truncates with + N more", func(t *testing.T) {
		tmp := t.TempDir()
		var items []catalog.ContentItem
		for i := 0; i < 6; i++ {
			name := fmt.Sprintf("skill-%d", i)
			dir := filepath.Join(tmp, "skills", name)
			os.MkdirAll(dir, 0o755)
			os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0o644)
			items = append(items, catalog.ContentItem{
				Name:  name,
				Type:  catalog.Skills,
				Path:  dir,
				Files: []string{"SKILL.md"},
			})
		}
		cat := &catalog.Catalog{RepoRoot: tmp, Items: items}
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.nameInput.SetValue("test")
		for i := range s.entries {
			s.entries[i].selected = true
		}
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "+ 3 more") {
			t.Errorf("should truncate to '+ 3 more' for 6 items (show 3), got:\n%s", got)
		}
	})

	t.Run("ambiguous names show source qualifier", func(t *testing.T) {
		tmp := t.TempDir()
		dir1 := filepath.Join(tmp, "skills", "my-skill-1")
		dir2 := filepath.Join(tmp, "skills", "my-skill-2")
		os.MkdirAll(dir1, 0o755)
		os.MkdirAll(dir2, 0o755)
		os.WriteFile(filepath.Join(dir1, "SKILL.md"), []byte("# same-name"), 0o644)
		os.WriteFile(filepath.Join(dir2, "SKILL.md"), []byte("# same-name"), 0o644)
		cat := &catalog.Catalog{
			RepoRoot: tmp,
			Items: []catalog.ContentItem{
				{Name: "same-name", Type: catalog.Skills, Path: dir1, Registry: "acme-registry", Source: "acme-registry", Files: []string{"SKILL.md"}},
				{Name: "same-name", Type: catalog.Skills, Path: dir2, Source: "project", Files: []string{"SKILL.md"}},
			},
		}
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.nameInput.SetValue("test")
		for i := range s.entries {
			s.entries[i].selected = true
		}
		s.step = clStepReview
		got := s.View()
		if !strings.Contains(got, "acme-registry") {
			t.Error("ambiguous name should show source qualifier (acme-registry)")
		}
		if !strings.Contains(got, "project") {
			t.Error("ambiguous name should show source qualifier (project)")
		}
	})

	t.Run("scroll up/down navigates review content", func(t *testing.T) {
		cat := testCatalog(t)
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 15) // small height to force scroll
		s.nameInput.SetValue("test")
		for i := range s.entries {
			s.entries[i].selected = true
		}
		s.step = clStepReview
		s, _ = s.Update(keyDown)
		s, _ = s.Update(keyDown)
		s, _ = s.Update(keyDown)
		if s.reviewScroll != 3 {
			t.Errorf("reviewScroll = %d, want 3", s.reviewScroll)
		}
		s, _ = s.Update(keyUp)
		if s.reviewScroll != 2 {
			t.Errorf("reviewScroll = %d, want 2 after up", s.reviewScroll)
		}
	})
}

func TestCreateLoadoutScreenSplitView(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("wide terminal shows separator", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 120, 40)
		s.step = clStepItems
		s.buildTypeItemMaps()
		got := s.View()
		if !strings.Contains(got, "│") {
			t.Error("wide terminal should show split-view separator")
		}
	})

	t.Run("wide terminal shows title bar", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 120, 40)
		s.step = clStepItems
		s.buildTypeItemMaps()
		got := s.View()
		if !strings.Contains(got, "Items") || !strings.Contains(got, "Preview") {
			t.Error("should show Items | Preview title bar")
		}
	})

	t.Run("narrow terminal single pane", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 60, 20)
		s.step = clStepItems
		s.buildTypeItemMaps()
		got := s.View()
		if got == "" {
			t.Error("narrow terminal View() should not be empty")
		}
	})
}

// ---------------------------------------------------------------------------
// Golden file tests
// ---------------------------------------------------------------------------

// navigateToCreateLoadout navigates from homepage to the create loadout screen.
func navigateToCreateLoadout(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1) // Loadouts in sidebar
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenLoadoutCards)

	// Press 'a' to open create loadout
	m, _ = app.Update(keyRune('a'))
	app = m.(App)
	assertScreen(t, app, screenCreateLoadout)
	return app
}

func navigateToCreateLoadoutSize(t *testing.T, width, height int) App {
	t.Helper()
	app := testAppSize(t, width, height)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	m, _ = app.Update(keyRune('a'))
	app = m.(App)
	assertScreen(t, app, screenCreateLoadout)
	return app
}

func TestGoldenFullApp_CreateLoadoutTypes(t *testing.T) {
	app := navigateToCreateLoadout(t)
	// Should start at types step (provider is pre-filled from loadout cards context)
	requireGolden(t, "fullapp-create-loadout-types", snapshotApp(t, app))
}

func TestGoldenFullApp_CreateLoadoutItems(t *testing.T) {
	app := navigateToCreateLoadout(t)
	// Advance to items step
	m, _ := app.Update(keyEnter)
	app = m.(App)
	requireGolden(t, "fullapp-create-loadout-items", snapshotApp(t, app))
}

// TestCreateLoadoutEnterDoesNotTriggerImport verifies that pressing Enter on the
// items step stays on the create loadout screen and does not trigger an import/add.
// Regression test for crossover bug where app-level Enter handlers intercepted keys.
func TestCreateLoadoutEnterDoesNotTriggerImport(t *testing.T) {
	app := navigateToCreateLoadout(t)
	// Step 1: Provider selection — select first provider and advance
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenCreateLoadout)
	if app.createLoadout.step != clStepTypes {
		t.Fatalf("expected clStepTypes after provider, got %d", app.createLoadout.step)
	}

	// Step 2: Select first type checkbox with Space, then advance with Enter
	m, _ = app.Update(keySpace)
	app = m.(App)
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenCreateLoadout)
	if app.createLoadout.step != clStepItems {
		t.Fatalf("expected clStepItems after types, got %d", app.createLoadout.step)
	}

	// Press Enter on items step — should advance to name step, NOT trigger import
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenCreateLoadout)
	if app.toast.active {
		t.Errorf("unexpected toast after Enter on items step: %q", app.toast.text)
	}
	if app.screen == screenCategory {
		t.Error("Enter on items step navigated back to category screen (import crossover bug)")
	}
}

func TestGoldenSized_CreateLoadoutTypes(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := navigateToCreateLoadoutSize(t, sz.width, sz.height)
			requireGolden(t, "fullapp-create-loadout-types-"+sz.tag,
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// TestCreateLoadoutRescanFindsNewLoadout verifies that after doCreateLoadoutFromScreen
// writes a loadout to the project destination, the catalog rescan finds it.
// This is the regression test for the creation bug where loadouts didn't appear.
func TestCreateLoadoutRescanFindsNewLoadout(t *testing.T) {
	app := testApp(t)
	contentRoot := app.catalog.RepoRoot

	// Count loadouts before creation
	beforeCount := len(app.catalog.ByType(catalog.Loadouts))

	// Set up the screen as if the user completed the wizard
	scr := newCreateLoadoutScreen("claude-code", "", app.providers, app.catalog, 80, 30)
	scr.nameInput.SetValue("new-test-loadout")
	scr.descInput.SetValue("A test loadout")
	scr.step = clStepDest
	scr.destCursor = 0 // Project destination
	scr.confirmed = true

	// Execute doCreateLoadoutFromScreen (the tea.Cmd) synchronously
	cmd := app.doCreateLoadoutFromScreen(scr)
	msg := cmd()

	// Verify no error
	result := msg.(doCreateLoadoutMsg)
	if result.err != nil {
		t.Fatalf("doCreateLoadout failed: %v", result.err)
	}

	// Verify the loadout.yaml was written to contentRoot (not projectRoot)
	loadoutPath := filepath.Join(contentRoot, "loadouts", "claude-code", "new-test-loadout", "loadout.yaml")
	if _, err := os.Stat(loadoutPath); err != nil {
		t.Fatalf("loadout.yaml not found at %s: %v", loadoutPath, err)
	}

	// Simulate the doCreateLoadoutMsg handler: rescan the catalog
	cat, err := catalog.Scan(contentRoot, contentRoot)
	if err != nil {
		t.Fatalf("rescan failed: %v", err)
	}

	afterCount := len(cat.ByType(catalog.Loadouts))
	if afterCount != beforeCount+1 {
		t.Errorf("loadout count after rescan = %d, want %d (before=%d)", afterCount, beforeCount+1, beforeCount)
	}

	// Verify the new loadout is in the catalog
	found := false
	for _, item := range cat.ByType(catalog.Loadouts) {
		if item.Name == "new-test-loadout" {
			found = true
			if item.Provider != "claude-code" {
				t.Errorf("provider = %q, want claude-code", item.Provider)
			}
			break
		}
	}
	if !found {
		t.Error("new-test-loadout not found in catalog after rescan")
	}
}

// TestCreateLoadoutHandlerNavigatesToDetail verifies the full handler integration:
// doCreateLoadoutMsg → catalog rescan → navigation to detail screen, toast, and sidebar update.
func TestCreateLoadoutHandlerNavigatesToDetail(t *testing.T) {
	app := testApp(t)
	app.screen = screenCreateLoadout

	beforeLoadoutCount := app.sidebar.loadoutsCount

	// Set up wizard state and execute the create command
	scr := newCreateLoadoutScreen("claude-code", "", app.providers, app.catalog, 80, 30)
	scr.nameInput.SetValue("handler-test-loadout")
	scr.descInput.SetValue("Tests handler integration")
	scr.destCursor = 0
	app.createLoadout = scr

	cmd := app.doCreateLoadoutFromScreen(scr)
	msg := cmd()

	// Verify the Cmd produced a success message
	result := msg.(doCreateLoadoutMsg)
	if result.err != nil {
		t.Fatalf("doCreateLoadout failed: %v", result.err)
	}

	// Feed the message into the App handler
	newApp, _ := app.Update(result)
	app = newApp.(App)

	// 1. Should navigate to detail screen showing the new loadout
	if app.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail", app.screen)
	}
	if app.detail.item.Name != "handler-test-loadout" {
		t.Errorf("detail item name = %q, want handler-test-loadout", app.detail.item.Name)
	}
	if app.detail.item.Provider != "claude-code" {
		t.Errorf("detail item provider = %q, want claude-code", app.detail.item.Provider)
	}

	// 2. Toast should show success message
	if !app.toast.active {
		t.Error("toast should be active after creation")
	}
	if !strings.Contains(app.toast.text, "handler-test-loadout") {
		t.Errorf("toast text = %q, should contain loadout name", app.toast.text)
	}
	if app.toast.isErr {
		t.Error("toast should not be an error")
	}

	// 3. Sidebar loadout count should increase
	if app.sidebar.loadoutsCount <= beforeLoadoutCount {
		t.Errorf("sidebar loadoutsCount = %d, should be > %d", app.sidebar.loadoutsCount, beforeLoadoutCount)
	}

	// 4. Items model should be set up for this provider's loadouts
	if app.items.ctx.sourceProvider != "claude-code" {
		t.Errorf("items sourceProvider = %q, want claude-code", app.items.ctx.sourceProvider)
	}
	if app.cardParent != screenLoadoutCards {
		t.Errorf("cardParent = %v, want screenLoadoutCards", app.cardParent)
	}
}

// TestCreateLoadoutLibraryDestination verifies that creating a loadout to the
// Library destination (~/.syllago/content/) is found by the global content scan.
func TestCreateLoadoutLibraryDestination(t *testing.T) {
	app := testApp(t)
	app.screen = screenCreateLoadout

	// Point GlobalContentDirOverride to a temp dir so the library write + rescan works
	globalDir := t.TempDir()
	origOverride := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origOverride })

	// Set up wizard for Library destination (destCursor=1)
	scr := newCreateLoadoutScreen("claude-code", "", app.providers, app.catalog, 80, 30)
	scr.nameInput.SetValue("library-loadout")
	scr.descInput.SetValue("Goes to library")
	scr.destCursor = 1 // Library destination
	app.createLoadout = scr

	cmd := app.doCreateLoadoutFromScreen(scr)
	msg := cmd()

	result := msg.(doCreateLoadoutMsg)
	if result.err != nil {
		t.Fatalf("doCreateLoadout failed: %v", result.err)
	}

	// Verify file was written to the global content dir
	loadoutPath := filepath.Join(globalDir, "loadouts", "claude-code", "library-loadout", "loadout.yaml")
	if _, err := os.Stat(loadoutPath); err != nil {
		t.Fatalf("loadout.yaml not found at library path %s: %v", loadoutPath, err)
	}

	// Feed the message into the App handler (uses ScanWithGlobalAndRegistries)
	newApp, _ := app.Update(result)
	app = newApp.(App)

	// Should navigate to detail screen
	if app.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail", app.screen)
	}
	if app.detail.item.Name != "library-loadout" {
		t.Errorf("detail item name = %q, want library-loadout", app.detail.item.Name)
	}
}
