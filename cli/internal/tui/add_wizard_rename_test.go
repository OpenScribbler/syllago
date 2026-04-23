package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// TestHandleRenameSaved_UpdatesDisplayNameAndDescription verifies that when the
// review-step rename modal emits editSavedMsg with context="wizard_rename",
// the wizard updates only the in-memory displayName and description of the
// corresponding discovery item — without touching the on-disk identity
// (item.name, which drives the target directory under the content root).
func TestHandleRenameSaved_UpdatesDisplayNameAndDescription(t *testing.T) {
	m := &addWizardModel{
		discoveredItems: []addDiscoveryItem{
			{name: "first-item", displayName: "first-item"},
			{name: "second-item", displayName: "second-item"},
		},
		renameDiscoveryIdx: 1,
	}

	m.handleRenameSaved(editSavedMsg{
		name:        "  My Display Name  ",
		description: "  A description  ",
		context:     "wizard_rename",
	})

	got := m.discoveredItems[1]
	if got.displayName != "My Display Name" {
		t.Errorf("displayName: got %q want %q", got.displayName, "My Display Name")
	}
	if got.description != "A description" {
		t.Errorf("description: got %q want %q", got.description, "A description")
	}
	// Identity (directory name) must not change.
	if got.name != "second-item" {
		t.Errorf("name (identity) changed: got %q want %q", got.name, "second-item")
	}
	// Other items must not be touched.
	if m.discoveredItems[0].displayName != "first-item" {
		t.Errorf("unrelated item displayName changed: got %q", m.discoveredItems[0].displayName)
	}
}

// TestHandleRenameSaved_OutOfBoundsIsNoop guards against panics when the
// modal's recorded discovery index is stale (e.g., items list was rebuilt
// between open and save).
func TestHandleRenameSaved_OutOfBoundsIsNoop(t *testing.T) {
	m := &addWizardModel{
		discoveredItems:    []addDiscoveryItem{{name: "only", displayName: "only"}},
		renameDiscoveryIdx: 5,
	}
	// Should not panic.
	m.handleRenameSaved(editSavedMsg{name: "new", description: "d"})

	if m.discoveredItems[0].displayName != "only" {
		t.Errorf("item changed unexpectedly: %q", m.discoveredItems[0].displayName)
	}
}

// TestReviewItems_EKeyOpensRenameModal verifies that pressing [e] while the
// review items zone is focused opens the rename modal pre-filled with the
// current display name and description, and with the wizard_rename context
// so App.Update routes the save back to the wizard.
func TestReviewItems_EKeyOpensRenameModal(t *testing.T) {
	items := []addDiscoveryItem{
		{
			name:        "my-hook",
			displayName: "my-hook",
			description: "original desc",
			itemType:    catalog.Hooks,
		},
	}
	list := newCheckboxList([]checkboxItem{{label: "my-hook"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:             addStepReview,
		reviewZone:       addReviewZoneItems,
		renameModal:      newEditModal(),
		discoveredItems:  items,
		actionableCount:  1,
		discoveryList:    list,
		reviewItemCursor: 0,
	}

	m2, _ := m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	if !m2.renameModal.active {
		t.Fatalf("rename modal should be active after pressing e")
	}
	if m2.renameModal.context != "wizard_rename" {
		t.Errorf("modal context: got %q want %q", m2.renameModal.context, "wizard_rename")
	}
	if m2.renameModal.name != "my-hook" {
		t.Errorf("modal prefilled name: got %q want %q", m2.renameModal.name, "my-hook")
	}
	if m2.renameModal.description != "original desc" {
		t.Errorf("modal prefilled description: got %q want %q", m2.renameModal.description, "original desc")
	}
}

// TestDrillIn_TabCyclesPanesToButtonsAndBack verifies that Tab in the drill-in
// view cycles tree → preview → Back → Rename → Next → panes (tree). The first
// reported bug was that drill-in had no keyboard path to the title-row buttons
// at all — Tab only toggled tree↔preview.
func TestDrillIn_TabCyclesPanesToButtonsAndBack(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "hook-one", displayName: "hook-one", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{{label: "hook-one"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:              addStepReview,
		reviewZone:        addReviewZoneItems,
		reviewDrillIn:     true,
		drillButtonCursor: -1,
		discoveredItems:   items,
		actionableCount:   1,
		discoveryList:     list,
		reviewItemCursor:  0,
	}
	m.reviewDrillTree.focused = true

	// Tab 1: tree → preview
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyTab})
	if m.reviewDrillTree.focused || !m.reviewDrillPreview.focused {
		t.Fatalf("Tab from tree should focus preview; tree=%v preview=%v",
			m.reviewDrillTree.focused, m.reviewDrillPreview.focused)
	}
	if m.drillButtonCursor != -1 {
		t.Errorf("drillButtonCursor should stay -1 while on panes, got %d", m.drillButtonCursor)
	}

	// Tab 2: preview → Back button
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyTab})
	if m.drillButtonCursor != 0 {
		t.Errorf("Tab from preview should land on Back (0), got %d", m.drillButtonCursor)
	}
	if m.reviewDrillTree.focused || m.reviewDrillPreview.focused {
		t.Errorf("panes should lose focus while buttons focused")
	}

	// Tab 3: Back → Rename
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyTab})
	if m.drillButtonCursor != 1 {
		t.Errorf("Tab from Back should land on Rename (1), got %d", m.drillButtonCursor)
	}

	// Tab 4: Rename → Next
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyTab})
	if m.drillButtonCursor != 2 {
		t.Errorf("Tab from Rename should land on Next (2), got %d", m.drillButtonCursor)
	}

	// Tab 5: Next → panes (back to tree)
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyTab})
	if m.drillButtonCursor != -1 {
		t.Errorf("Tab from Next should return focus to panes (cursor=-1), got %d", m.drillButtonCursor)
	}
	if !m.reviewDrillTree.focused {
		t.Errorf("Tab from Next should land on tree, got tree=%v", m.reviewDrillTree.focused)
	}
}

// TestDrillIn_EnterOnBackButtonExitsDrillIn verifies Enter on the focused Back
// button returns to the review list — same as Esc, but triggered from the
// button row via the keyboard path the user now has access to.
func TestDrillIn_EnterOnBackButtonExitsDrillIn(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "hook-one", displayName: "hook-one", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{{label: "hook-one"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:              addStepReview,
		reviewDrillIn:     true,
		drillButtonCursor: 0, // Back focused
		discoveredItems:   items,
		actionableCount:   1,
		discoveryList:     list,
		reviewItemCursor:  0,
	}

	m2, _ := m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyEnter})

	if m2.reviewDrillIn {
		t.Errorf("Enter on Back should exit drill-in, still in=%v", m2.reviewDrillIn)
	}
	if m2.reviewZone != addReviewZoneItems {
		t.Errorf("exit should return focus to items zone, got %v", m2.reviewZone)
	}
}

// TestDrillIn_EnterOnRenameButtonOpensModal verifies Enter on the focused
// Rename button opens the rename modal with wizard_rename context.
func TestDrillIn_EnterOnRenameButtonOpensModal(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "hook-one", displayName: "hook-one", description: "x", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{{label: "hook-one"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:              addStepReview,
		reviewDrillIn:     true,
		drillButtonCursor: 1, // Rename focused
		renameModal:       newEditModal(),
		discoveredItems:   items,
		actionableCount:   1,
		discoveryList:     list,
		reviewItemCursor:  0,
	}

	m2, _ := m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyEnter})

	if !m2.renameModal.active {
		t.Fatalf("Enter on Rename should open the modal")
	}
	if m2.renameModal.context != "wizard_rename" {
		t.Errorf("modal context: got %q want %q", m2.renameModal.context, "wizard_rename")
	}
}

// TestDrillIn_EnterOnNextButtonAdvancesItem verifies Enter on the focused Next
// button advances to the next selected item's drill-in (still inside the
// drill-in view, not bounced back to the review list).
func TestDrillIn_EnterOnNextButtonAdvancesItem(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "hook-one", displayName: "hook-one", itemType: catalog.Hooks},
		{name: "hook-two", displayName: "hook-two", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{
		{label: "hook-one"},
		{label: "hook-two"},
	})
	list.selected[0] = true
	list.selected[1] = true

	m := &addWizardModel{
		step:              addStepReview,
		reviewDrillIn:     true,
		drillButtonCursor: 2, // Next focused
		discoveredItems:   items,
		actionableCount:   2,
		discoveryList:     list,
		reviewItemCursor:  0,
		width:             120,
		height:            40,
	}

	m2, _ := m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyEnter})

	if m2.reviewItemCursor != 1 {
		t.Errorf("Next should advance cursor to 1, got %d", m2.reviewItemCursor)
	}
	if !m2.reviewDrillIn {
		t.Errorf("Next should stay in drill-in for the new item, got in=%v", m2.reviewDrillIn)
	}
}

// TestDrillIn_EnterOnNextButtonExitsIfLast verifies Enter on Next when already
// on the last selected item exits drill-in instead of dead-ending.
func TestDrillIn_EnterOnNextButtonExitsIfLast(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "only", displayName: "only", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{{label: "only"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:              addStepReview,
		reviewDrillIn:     true,
		drillButtonCursor: 2, // Next focused
		discoveredItems:   items,
		actionableCount:   1,
		discoveryList:     list,
		reviewItemCursor:  0,
	}

	m2, _ := m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyEnter})

	if m2.reviewDrillIn {
		t.Errorf("Next on last item should exit drill-in, still in=%v", m2.reviewDrillIn)
	}
}

// TestDrillIn_LeftRightNavigatesButtonsWhenFocused verifies that once focus is
// on the title-row buttons, Left/Right walk between them without falling back
// to the pane-swap behavior that Left/Right does when panes are focused.
func TestDrillIn_LeftRightNavigatesButtonsWhenFocused(t *testing.T) {
	m := &addWizardModel{
		reviewDrillIn:     true,
		drillButtonCursor: 0, // Back
	}

	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyRight})
	if m.drillButtonCursor != 1 {
		t.Errorf("Right on Back should advance to Rename (1), got %d", m.drillButtonCursor)
	}
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyRight})
	if m.drillButtonCursor != 2 {
		t.Errorf("Right on Rename should advance to Next (2), got %d", m.drillButtonCursor)
	}
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyRight})
	if m.drillButtonCursor != 2 {
		t.Errorf("Right on Next should clamp at 2, got %d", m.drillButtonCursor)
	}
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyLeft})
	if m.drillButtonCursor != 0 {
		t.Errorf("Left twice should return to Back (0), got %d", m.drillButtonCursor)
	}
	m, _ = m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyLeft})
	if m.drillButtonCursor != 0 {
		t.Errorf("Left on Back should clamp at 0, got %d", m.drillButtonCursor)
	}
}

// TestDrillIn_EKeyOpensRenameModal verifies that pressing [e] while in the
// drill-in file/preview view opens the rename modal for the current item —
// the same behavior as the review list, so users can pick a display name
// while looking at the item's contents.
func TestDrillIn_EKeyOpensRenameModal(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "hook-one", displayName: "hook-one", description: "", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{{label: "hook-one"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:             addStepReview,
		reviewZone:       addReviewZoneItems,
		reviewDrillIn:    true,
		renameModal:      newEditModal(),
		discoveredItems:  items,
		actionableCount:  1,
		discoveryList:    list,
		reviewItemCursor: 0,
	}

	m2, _ := m.updateKeyReviewDrillIn(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	if !m2.renameModal.active {
		t.Fatalf("rename modal should be active after pressing e in drill-in")
	}
	if m2.renameModal.context != "wizard_rename" {
		t.Errorf("modal context: got %q want %q", m2.renameModal.context, "wizard_rename")
	}
	if !m2.reviewDrillIn {
		t.Errorf("drill-in flag should remain true while modal is open")
	}
}

// TestReviewButtons_RenameButtonOpensModal verifies that selecting the
// Rename button (focus index 1) on the review button row and pressing Enter
// opens the rename modal — the keyboard-equivalent of clicking the button.
func TestReviewButtons_RenameButtonOpensModal(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "my-hook", displayName: "my-hook", description: "orig", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{{label: "my-hook"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:             addStepReview,
		reviewZone:       addReviewZoneButtons,
		buttonCursor:     1, // Rename
		renameModal:      newEditModal(),
		discoveredItems:  items,
		actionableCount:  1,
		discoveryList:    list,
		reviewItemCursor: 0,
	}

	m2, _ := m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyEnter})

	if !m2.renameModal.active {
		t.Fatalf("rename modal should be active after Enter on Rename button")
	}
	if m2.renameModal.context != "wizard_rename" {
		t.Errorf("modal context: got %q want %q", m2.renameModal.context, "wizard_rename")
	}
}

// TestReviewButtons_CursorClampsAtFour verifies the button row has exactly
// four slots (Add/Rename/Back/Cancel) — Right cannot exceed index 3. When no
// items are selected, Left at the leftmost button has nowhere to cross to, so
// the cursor clamps at 0 (the cross-zone branch requires selectedItems > 0).
func TestReviewButtons_CursorClampsAtFour(t *testing.T) {
	m := &addWizardModel{
		step:         addStepReview,
		reviewZone:   addReviewZoneButtons,
		buttonCursor: 0,
	}

	for i := 0; i < 10; i++ {
		m, _ = m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyRight})
	}
	if m.buttonCursor != 3 {
		t.Errorf("Right-spam should clamp cursor at 3, got %d", m.buttonCursor)
	}

	for i := 0; i < 10; i++ {
		m, _ = m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyLeft})
	}
	if m.buttonCursor != 0 {
		t.Errorf("Left-spam (no items) should clamp cursor at 0, got %d", m.buttonCursor)
	}
	if m.reviewZone != addReviewZoneButtons {
		t.Errorf("Left-spam (no items) should stay in buttons zone, got %v", m.reviewZone)
	}
}

// TestReviewButtons_LeftAtLeftmostCrossesToItems verifies Left at the Add
// button (cursor 0) crosses back into the items zone when items are selected.
// This makes Left/Right navigation symmetric with the Right-from-items jump,
// so users don't need to discover the Tab shortcut to move between zones.
func TestReviewButtons_LeftAtLeftmostCrossesToItems(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "my-hook", displayName: "my-hook", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{{label: "my-hook"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:            addStepReview,
		reviewZone:      addReviewZoneButtons,
		buttonCursor:    0, // on Add (leftmost)
		discoveredItems: items,
		actionableCount: 1,
		discoveryList:   list,
	}

	m2, _ := m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyLeft})

	if m2.reviewZone != addReviewZoneItems {
		t.Errorf("Left at leftmost button should cross into items zone, got %v", m2.reviewZone)
	}
}

// TestReviewItems_RightCrossesToButtons verifies Right from the items zone
// jumps to the button row, landing on Add (cursor 0). This is the forward
// half of the cross-zone navigation pair.
func TestReviewItems_RightCrossesToButtons(t *testing.T) {
	items := []addDiscoveryItem{
		{name: "my-hook", displayName: "my-hook", itemType: catalog.Hooks},
	}
	list := newCheckboxList([]checkboxItem{{label: "my-hook"}})
	list.selected[0] = true

	m := &addWizardModel{
		step:             addStepReview,
		reviewZone:       addReviewZoneItems,
		buttonCursor:     2, // default Back
		discoveredItems:  items,
		actionableCount:  1,
		discoveryList:    list,
		reviewItemCursor: 0,
	}

	m2, _ := m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyRight})

	if m2.reviewZone != addReviewZoneButtons {
		t.Errorf("Right from items should cross into buttons zone, got %v", m2.reviewZone)
	}
	if m2.buttonCursor != 0 {
		t.Errorf("Right-crossing should land on Add (index 0), got %d", m2.buttonCursor)
	}
}

// TestDrillInTitleRow_ContainsRenameButton verifies the drill-in title row
// renders a Rename button between Back and Next, not just the bare Back/Next
// pair from the shared renderTitleRow helper.
func TestDrillInTitleRow_ContainsRenameButton(t *testing.T) {
	m := &addWizardModel{
		width:  120,
		height: 40,
	}
	row := m.renderDrillInTitleRow("Inspecting: my-hook")

	// All three button labels must be present.
	for _, want := range []string{"Back", "Rename", "Next"} {
		if !strings.Contains(row, want) {
			t.Errorf("drill-in title row missing %q button, got:\n%s", want, row)
		}
	}

	// Rename must sit between Back and Next (left-to-right order).
	backIdx := strings.Index(row, "Back")
	renameIdx := strings.Index(row, "Rename")
	nextIdx := strings.Index(row, "Next")
	if backIdx >= renameIdx || renameIdx >= nextIdx {
		t.Errorf("expected Back < Rename < Next order, got Back=%d Rename=%d Next=%d",
			backIdx, renameIdx, nextIdx)
	}
}

// TestEditModal_SetWidth clamps below a reasonable minimum so the modal
// stays usable even when the tree pane is narrow.
func TestEditModal_SetWidth(t *testing.T) {
	m := newEditModal()
	m.SetWidth(12)
	if m.width != 24 {
		t.Errorf("SetWidth(12) should clamp to 24, got %d", m.width)
	}
	m.SetWidth(40)
	if m.width != 40 {
		t.Errorf("SetWidth(40) should set to 40, got %d", m.width)
	}
}

// TestCapturingTextInput reports whether the wizard currently has a text
// field focused. App-level key routing relies on this to forward digits
// (1/2/3) to the text field instead of hijacking them for group navigation.
func TestCapturingTextInput(t *testing.T) {
	// Nil-safe so the App's `a.addWizard.CapturingTextInput()` call doesn't
	// crash when the wizard pointer is nil.
	var nilWizard *addWizardModel
	if nilWizard.CapturingTextInput() {
		t.Errorf("nil wizard should return false")
	}

	// Fresh wizard, no modal, no input — no capture.
	m := &addWizardModel{renameModal: newEditModal()}
	if m.CapturingTextInput() {
		t.Errorf("fresh wizard should not capture")
	}

	// Rename modal open — capture.
	m.renameModal.Open("title", "name", "desc", "path")
	if !m.CapturingTextInput() {
		t.Errorf("wizard with active rename modal should capture")
	}
	m.renameModal.Close()

	// Source-step path input active — capture.
	m.inputActive = true
	if !m.CapturingTextInput() {
		t.Errorf("wizard with active path input should capture")
	}
}

// TestRenameModal_AcceptsDigitRunes verifies that the modal's own Update
// writes digit runes into the focused field. This is the downstream half of
// the 1/2/3 fix — App-level key routing must forward the key, AND the modal
// must append it to m.name.
func TestRenameModal_AcceptsDigitRunes(t *testing.T) {
	m := newEditModal()
	m.OpenWithContext("Rename", "", "", "path", "wizard_rename")

	// Simulate pressing "1", "2", "3" on the name field (focusIdx 0).
	for _, r := range []rune{'1', '2', '3'} {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}

	if m.name != "123" {
		t.Errorf("name after typing 1/2/3: got %q want %q", m.name, "123")
	}
}

// TestEditModal_ViewContainsContextHint verifies the modal shows a microcopy
// line explaining where the description appears. Wizard-rename context gets
// "after you add" phrasing since the item isn't saved yet; library edit
// gets the shorter form.
func TestEditModal_ViewContainsContextHint(t *testing.T) {
	lib := newEditModal()
	lib.Open("Edit", "foo", "", "path")
	if got := lib.View(); !strings.Contains(got, "Shown in your library.") {
		t.Errorf("library edit modal missing hint, got:\n%s", got)
	}

	wiz := newEditModal()
	wiz.OpenWithContext("Rename", "foo", "", "path", "wizard_rename")
	if got := wiz.View(); !strings.Contains(got, "after you add this item") {
		t.Errorf("wizard rename modal missing wizard-specific hint, got:\n%s", got)
	}
}

// TestWriteHookToLibrary_HonorsDisplayNameAndDescription verifies that a
// rename applied to a hook before adding (via the review-step [e] modal) is
// persisted into .syllago.yaml without changing the on-disk directory name
// or hook.json payload.
func TestWriteHookToLibrary_HonorsDisplayNameAndDescription(t *testing.T) {
	contentRoot := t.TempDir()

	hook := converter.HookData{
		Event: "before_tool_execute",
		Hooks: []converter.HookEntry{{Type: "command", Command: "echo hi"}},
	}

	item := addDiscoveryItem{
		name:        "original-dir-name",
		displayName: "Friendly Display Name",
		description: "User-supplied description",
		itemType:    catalog.Hooks,
		scope:       "global",
		hookData:    &hook,
	}

	result := writeHookToLibrary(item, contentRoot, "", "", "claude-code")
	if result.status != "added" {
		t.Fatalf("expected status=added, got %q err=%v", result.status, result.err)
	}

	// Directory must still be keyed on item.name (identity), NOT displayName.
	itemDir := filepath.Join(contentRoot, string(catalog.Hooks), "claude-code", "original-dir-name")
	if _, err := os.Stat(itemDir); err != nil {
		t.Fatalf("item dir should exist at identity-keyed path: %v", err)
	}

	// displayName must NOT be used as a directory.
	strayDir := filepath.Join(contentRoot, string(catalog.Hooks), "claude-code", "Friendly Display Name")
	if _, err := os.Stat(strayDir); err == nil {
		t.Errorf("unexpected directory created from displayName: %s", strayDir)
	}

	// Metadata should carry displayName + description.
	meta, err := metadata.Load(itemDir)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta.Name != "Friendly Display Name" {
		t.Errorf("meta.Name: got %q want %q", meta.Name, "Friendly Display Name")
	}
	if meta.Description != "User-supplied description" {
		t.Errorf("meta.Description: got %q want %q", meta.Description, "User-supplied description")
	}

	// hook.json payload unchanged — still the canonical manifest.
	hookPath := filepath.Join(itemDir, "hook.json")
	hookBytes, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(hookBytes, &parsed); err != nil {
		t.Fatalf("hook.json should be valid JSON: %v", err)
	}
	if parsed["spec"] != converter.SpecVersion {
		t.Errorf("hook.json spec: got %v want %q", parsed["spec"], converter.SpecVersion)
	}
}
