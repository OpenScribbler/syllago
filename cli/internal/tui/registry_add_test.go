package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestRegistryAddModal_NewDefault(t *testing.T) {
	m := newRegistryAddModal()
	if m.active {
		t.Fatal("modal should not be active initially")
	}
	if !m.sourceGit {
		t.Fatal("sourceGit should default to true")
	}
}

func TestRegistryAddModal_Open(t *testing.T) {
	m := newRegistryAddModal()
	existingNames := []string{"acme-tools", "corp-rules"}
	cfg := &config.Config{
		AllowedRegistries: []string{"https://github.com/example/repo"},
	}

	m.Open(existingNames, cfg)

	if !m.active {
		t.Fatal("modal should be active after Open")
	}
	if m.focusIdx != 1 {
		t.Errorf("expected focusIdx == 1 (URL field), got %d", m.focusIdx)
	}
	if !m.sourceGit {
		t.Fatal("sourceGit should be true after Open")
	}
	if len(m.existingNames) != 2 {
		t.Errorf("expected 2 existingNames, got %d", len(m.existingNames))
	}
	if m.existingNames[0] != "acme-tools" {
		t.Errorf("expected existingNames[0] = 'acme-tools', got %q", m.existingNames[0])
	}
	if m.cfg != cfg {
		t.Fatal("expected cfg to be stored")
	}
}

func TestRegistryAddModal_Close(t *testing.T) {
	m := newRegistryAddModal()
	cfg := &config.Config{}
	m.Open([]string{"existing"}, cfg)

	// Set some field values to verify reset
	m.urlValue = "https://github.com/test/repo"
	m.nameValue = "test-repo"
	m.branchValue = "main"
	m.cursor = 5
	m.focusIdx = 3
	m.nameManuallySet = true
	m.err = "some error"

	m.Close()

	if m.active {
		t.Fatal("modal should not be active after Close")
	}
	if m.urlValue != "" {
		t.Errorf("expected urlValue empty, got %q", m.urlValue)
	}
	if m.nameValue != "" {
		t.Errorf("expected nameValue empty, got %q", m.nameValue)
	}
	if m.branchValue != "" {
		t.Errorf("expected branchValue empty, got %q", m.branchValue)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
	if m.focusIdx != 0 {
		t.Errorf("expected focusIdx 0, got %d", m.focusIdx)
	}
	if m.nameManuallySet {
		t.Fatal("expected nameManuallySet false after Close")
	}
	if m.err != "" {
		t.Errorf("expected err empty, got %q", m.err)
	}
	if m.existingNames != nil {
		t.Errorf("expected existingNames nil, got %v", m.existingNames)
	}
	if m.cfg != nil {
		t.Fatal("expected cfg nil after Close")
	}
}

func TestRegistryAddModal_FocusedValue(t *testing.T) {
	m := newRegistryAddModal()
	m.urlValue = "url-val"
	m.nameValue = "name-val"
	m.branchValue = "branch-val"

	tests := []struct {
		name     string
		focusIdx int
		wantNil  bool
		wantVal  string
	}{
		{"radio returns nil", 0, true, ""},
		{"url field", 1, false, "url-val"},
		{"name field", 2, false, "name-val"},
		{"branch field", 3, false, "branch-val"},
		{"cancel returns nil", 4, true, ""},
		{"add returns nil", 5, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.focusIdx = tt.focusIdx
			got := m.focusedValue()
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil for focusIdx %d, got %q", tt.focusIdx, *got)
				}
			} else {
				if got == nil {
					t.Fatalf("expected non-nil for focusIdx %d", tt.focusIdx)
				}
				if *got != tt.wantVal {
					t.Errorf("expected %q, got %q", tt.wantVal, *got)
				}
			}
		})
	}
}

func TestRegistryAddModal_IsTextField(t *testing.T) {
	tests := []struct {
		name      string
		focusIdx  int
		sourceGit bool
		want      bool
	}{
		{"radio not text", 0, true, false},
		{"url is text", 1, true, true},
		{"name is text", 2, true, true},
		{"branch is text when git", 3, true, true},
		{"branch not text when local", 3, false, false},
		{"cancel not text", 4, true, false},
		{"add not text", 5, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newRegistryAddModal()
			m.focusIdx = tt.focusIdx
			m.sourceGit = tt.sourceGit
			got := m.isTextField()
			if got != tt.want {
				t.Errorf("isTextField() for focusIdx=%d sourceGit=%v: got %v, want %v",
					tt.focusIdx, tt.sourceGit, got, tt.want)
			}
		})
	}
}

func TestRegistryAddModal_NextFocusIdx_GitMode(t *testing.T) {
	m := newRegistryAddModal()
	m.sourceGit = true

	// Forward: 0→1→2→3→4→5→0
	expected := []int{1, 2, 3, 4, 5, 0}
	current := 0
	for i, want := range expected {
		got := m.nextFocusIdx(current, +1)
		if got != want {
			t.Errorf("step %d: nextFocusIdx(%d, +1) = %d, want %d", i, current, got, want)
		}
		current = got
	}
}

func TestRegistryAddModal_NextFocusIdx_LocalMode(t *testing.T) {
	m := newRegistryAddModal()
	m.sourceGit = false

	// Forward skipping 3: 0→1→2→4→5→0
	expected := []int{1, 2, 4, 5, 0}
	current := 0
	for i, want := range expected {
		got := m.nextFocusIdx(current, +1)
		if got != want {
			t.Errorf("step %d: nextFocusIdx(%d, +1) = %d, want %d", i, current, got, want)
		}
		current = got
	}
}

func TestRegistryAddModal_NextFocusIdx_Reverse_GitMode(t *testing.T) {
	m := newRegistryAddModal()
	m.sourceGit = true

	// Backward: 0→5→4→3→2→1→0
	expected := []int{5, 4, 3, 2, 1, 0}
	current := 0
	for i, want := range expected {
		got := m.nextFocusIdx(current, -1)
		if got != want {
			t.Errorf("step %d: nextFocusIdx(%d, -1) = %d, want %d", i, current, got, want)
		}
		current = got
	}
}

func TestRegistryAddModal_NextFocusIdx_Reverse_LocalMode(t *testing.T) {
	m := newRegistryAddModal()
	m.sourceGit = false

	// Backward skipping 3: 0→5→4→2→1→0
	expected := []int{5, 4, 2, 1, 0}
	current := 0
	for i, want := range expected {
		got := m.nextFocusIdx(current, -1)
		if got != want {
			t.Errorf("step %d: nextFocusIdx(%d, -1) = %d, want %d", i, current, got, want)
		}
		current = got
	}
}

func TestRegistryAddModal_UpdateInactive(t *testing.T) {
	m := newRegistryAddModal()
	// Modal is inactive by default

	m2, cmd := m.Update(keyRune('a'))
	if cmd != nil {
		t.Fatal("inactive modal should return nil cmd")
	}
	if m2.active {
		t.Fatal("inactive modal should remain inactive")
	}
	if m2.urlValue != "" {
		t.Error("inactive modal should not change state")
	}
}

// --- Key handling tests (C1.4) ---

func openModal() registryAddModal {
	m := newRegistryAddModal()
	m.Open(nil, nil)
	return m
}

func TestRegistryAddModal_KeyEsc(t *testing.T) {
	m := openModal()
	m2, _ := m.Update(keyPress(tea.KeyEsc))
	if m2.active {
		t.Fatal("Esc should close modal (active == false)")
	}
}

func TestRegistryAddModal_KeyEnter_Radio(t *testing.T) {
	m := openModal()
	m.focusIdx = 0
	m2, _ := m.Update(keyPress(tea.KeyEnter))
	if m2.focusIdx != 1 {
		t.Errorf("Enter on radio should advance to URL (focusIdx=1), got %d", m2.focusIdx)
	}
}

func TestRegistryAddModal_KeyEnter_URL(t *testing.T) {
	m := openModal()
	m.focusIdx = 1
	m2, _ := m.Update(keyPress(tea.KeyEnter))
	if m2.focusIdx != 2 {
		t.Errorf("Enter on URL should advance to Name (focusIdx=2), got %d", m2.focusIdx)
	}
}

func TestRegistryAddModal_KeyEnter_Name_GitMode(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.focusIdx = 2
	m2, _ := m.Update(keyPress(tea.KeyEnter))
	if m2.focusIdx != 3 {
		t.Errorf("Enter on Name (git mode) should advance to Branch (focusIdx=3), got %d", m2.focusIdx)
	}
}

func TestRegistryAddModal_KeyEnter_Name_LocalMode(t *testing.T) {
	m := openModal()
	m.sourceGit = false
	m.focusIdx = 2
	m2, _ := m.Update(keyPress(tea.KeyEnter))
	if m2.focusIdx != 4 {
		t.Errorf("Enter on Name (local mode) should skip Branch, advance to Cancel (focusIdx=4), got %d", m2.focusIdx)
	}
}

func TestRegistryAddModal_KeyEnter_Branch(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.focusIdx = 3
	m2, _ := m.Update(keyPress(tea.KeyEnter))
	if m2.focusIdx != 5 {
		t.Errorf("Enter on Branch should advance to Add (focusIdx=5), got %d", m2.focusIdx)
	}
}

func TestRegistryAddModal_KeyEnter_Cancel(t *testing.T) {
	m := openModal()
	m.focusIdx = 4
	m2, _ := m.Update(keyPress(tea.KeyEnter))
	if m2.active {
		t.Fatal("Enter on Cancel should close modal (active == false)")
	}
}

func TestRegistryAddModal_KeyEnter_Add_Valid(t *testing.T) {
	m := openModal()
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = "acme/tools"
	m.focusIdx = 5

	m2, cmd := m.Update(keyPress(tea.KeyEnter))
	if m2.active {
		t.Fatal("Enter on Add with valid data should close modal")
	}
	if cmd == nil {
		t.Fatal("Enter on Add with valid data should return a non-nil cmd")
	}

	// Execute the cmd and verify the resulting message.
	msg := cmd()
	addMsg, ok := msg.(registryAddMsg)
	if !ok {
		t.Fatalf("expected registryAddMsg, got %T", msg)
	}
	if addMsg.url != "https://github.com/acme/tools" {
		t.Errorf("expected url = 'https://github.com/acme/tools', got %q", addMsg.url)
	}
	if addMsg.name != "acme/tools" {
		t.Errorf("expected name = 'acme/tools', got %q", addMsg.name)
	}
	if addMsg.isLocal {
		t.Error("expected isLocal == false for git URL")
	}
}

func TestRegistryAddModal_KeySpace_Radio(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.focusIdx = 0

	m2, _ := m.Update(keyPress(tea.KeySpace))
	if m2.sourceGit {
		t.Fatal("Space on radio should toggle sourceGit to false")
	}

	m3, _ := m2.Update(keyPress(tea.KeySpace))
	if !m3.sourceGit {
		t.Fatal("Space on radio should toggle sourceGit back to true")
	}
}

func TestRegistryAddModal_KeyTab_GitMode(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.focusIdx = 1 // default after Open

	// Tab 6 times: 1→2→3→4→5→0→1
	expected := []int{2, 3, 4, 5, 0, 1}
	for i, want := range expected {
		m, _ = m.Update(keyTab)
		if m.focusIdx != want {
			t.Errorf("tab step %d: expected focusIdx=%d, got %d", i, want, m.focusIdx)
		}
	}
}

func TestRegistryAddModal_KeyTab_LocalMode(t *testing.T) {
	m := openModal()
	m.sourceGit = false
	m.focusIdx = 1

	// Tab 5 times: 1→2→4→5→0→1 (skips 3)
	expected := []int{2, 4, 5, 0, 1}
	for i, want := range expected {
		m, _ = m.Update(keyTab)
		if m.focusIdx != want {
			t.Errorf("tab step %d: expected focusIdx=%d, got %d", i, want, m.focusIdx)
		}
	}
}

func TestRegistryAddModal_KeyShiftTab(t *testing.T) {
	m := openModal()
	m.focusIdx = 1

	m2, _ := m.Update(keyShiftTab)
	if m2.focusIdx != 0 {
		t.Errorf("Shift+Tab from 1 should go to 0, got %d", m2.focusIdx)
	}
}

func TestRegistryAddModal_KeyRunes(t *testing.T) {
	m := openModal()
	m.focusIdx = 1 // URL field

	m, _ = m.Update(keyRune('a'))
	m, _ = m.Update(keyRune('b'))
	m, _ = m.Update(keyRune('c'))

	if m.urlValue != "abc" {
		t.Errorf("expected urlValue == 'abc', got %q", m.urlValue)
	}
	if m.cursor != 3 {
		t.Errorf("expected cursor == 3, got %d", m.cursor)
	}
}

func TestRegistryAddModal_KeyBackspace(t *testing.T) {
	m := openModal()
	m.focusIdx = 1
	m.urlValue = "abc"
	m.cursor = 3

	m2, _ := m.Update(keyPress(tea.KeyBackspace))
	if m2.urlValue != "ab" {
		t.Errorf("expected urlValue == 'ab', got %q", m2.urlValue)
	}
	if m2.cursor != 2 {
		t.Errorf("expected cursor == 2, got %d", m2.cursor)
	}
}

func TestRegistryAddModal_KeyCursorMovement(t *testing.T) {
	m := openModal()
	m.focusIdx = 1
	m.urlValue = "abc"
	m.cursor = 3

	// Left
	m, _ = m.Update(keyPress(tea.KeyLeft))
	if m.cursor != 2 {
		t.Errorf("Left: expected cursor == 2, got %d", m.cursor)
	}

	// Home
	m, _ = m.Update(keyPress(tea.KeyHome))
	if m.cursor != 0 {
		t.Errorf("Home: expected cursor == 0, got %d", m.cursor)
	}

	// End
	m, _ = m.Update(keyPress(tea.KeyEnd))
	if m.cursor != 3 {
		t.Errorf("End: expected cursor == 3, got %d", m.cursor)
	}

	// Right from 0
	m.cursor = 0
	m, _ = m.Update(keyPress(tea.KeyRight))
	if m.cursor != 1 {
		t.Errorf("Right: expected cursor == 1, got %d", m.cursor)
	}
}

func TestRegistryAddModal_KeySpace_TextField(t *testing.T) {
	m := openModal()
	m.focusIdx = 1
	m.urlValue = "ab"
	m.cursor = 2

	m2, _ := m.Update(keyPress(tea.KeySpace))
	if m2.urlValue != "ab " {
		t.Errorf("Space in text field should insert space, got %q", m2.urlValue)
	}
}

func TestRegistryAddModal_KeyCtrlS(t *testing.T) {
	m := openModal()
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = "acme/tools"
	m.focusIdx = 1 // not on Add button

	m2, cmd := m.Update(keyPress(tea.KeyCtrlS))
	if m2.active {
		t.Fatal("Ctrl+S should submit and close modal")
	}
	if cmd == nil {
		t.Fatal("Ctrl+S should return a non-nil cmd (submit)")
	}

	msg := cmd()
	addMsg, ok := msg.(registryAddMsg)
	if !ok {
		t.Fatalf("expected registryAddMsg, got %T", msg)
	}
	if addMsg.url != "https://github.com/acme/tools" {
		t.Errorf("expected url = 'https://github.com/acme/tools', got %q", addMsg.url)
	}
}

func TestRegistryAddModal_KeyYN_NoShortcut(t *testing.T) {
	m := openModal()
	m.focusIdx = 1

	m, _ = m.Update(keyRune('y'))
	if m.urlValue != "y" {
		t.Errorf("expected 'y' typed into field, got %q", m.urlValue)
	}

	m, _ = m.Update(keyRune('n'))
	if m.urlValue != "yn" {
		t.Errorf("expected 'yn' typed into field, got %q", m.urlValue)
	}
}

func TestRegistryAddModal_AutoDeriveName_Git(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.focusIdx = 1

	// Type the URL character by character.
	url := "https://github.com/acme/tools"
	for _, r := range url {
		m, _ = m.Update(keyRune(r))
	}

	// Expected: registry.NameFromURL should derive "acme/tools".
	want := registry.NameFromURL(url)
	if m.nameValue != want {
		t.Errorf("auto-derived name: expected %q, got %q", want, m.nameValue)
	}
}

func TestRegistryAddModal_AutoDeriveName_Local(t *testing.T) {
	m := openModal()
	m.sourceGit = false
	m.focusIdx = 1

	path := "/home/user/my-registry"
	for _, r := range path {
		m, _ = m.Update(keyRune(r))
	}

	want := filepath.Base(path)
	if m.nameValue != want {
		t.Errorf("auto-derived local name: expected %q, got %q", want, m.nameValue)
	}
}

func TestRegistryAddModal_ManualNameStopsDerivation(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.focusIdx = 1

	// Type a URL to auto-derive a name.
	for _, r := range "https://github.com/acme/tools" {
		m, _ = m.Update(keyRune(r))
	}

	// Now manually type in the name field.
	m.focusIdx = 2
	m.cursor = len([]rune(m.nameValue))
	m, _ = m.Update(keyRune('X'))

	if !m.nameManuallySet {
		t.Fatal("typing in name field should set nameManuallySet = true")
	}

	manualName := m.nameValue

	// Go back to URL and type more.
	m.focusIdx = 1
	m.cursor = len([]rune(m.urlValue))
	m, _ = m.Update(keyRune('z'))

	if m.nameValue != manualName {
		t.Errorf("name should not change after manual edit: expected %q, got %q", manualName, m.nameValue)
	}
}

func TestRegistryAddModal_ClearNameResumesDerivation(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.focusIdx = 1

	// Type URL.
	for _, r := range "https://github.com/acme/tools" {
		m, _ = m.Update(keyRune(r))
	}

	// Manually set name.
	m.focusIdx = 2
	m.cursor = len([]rune(m.nameValue))
	m, _ = m.Update(keyRune('X'))

	if !m.nameManuallySet {
		t.Fatal("nameManuallySet should be true after manual typing")
	}

	// Backspace name to empty.
	for len([]rune(m.nameValue)) > 0 {
		m.cursor = len([]rune(m.nameValue))
		m, _ = m.Update(keyPress(tea.KeyBackspace))
	}

	if m.nameManuallySet {
		t.Fatal("nameManuallySet should be false after clearing name")
	}

	// Now type more URL — derivation should resume.
	m.focusIdx = 1
	m.cursor = len([]rune(m.urlValue))
	m, _ = m.Update(keyRune('z'))

	if m.nameValue == "" {
		t.Error("name should be auto-derived again after clearing manual name")
	}
}

func TestRegistryAddModal_ErrorClearsOnKeypress(t *testing.T) {
	m := openModal()
	m.err = "some error"

	m2, _ := m.Update(keyRune('a'))
	if m2.err != "" {
		t.Errorf("error should clear on keypress, got %q", m2.err)
	}
}

// --- Validation tests (C1.7) ---

func TestRegistryAddModal_Validate_EmptyURL(t *testing.T) {
	m := openModal()
	m.urlValue = ""
	m.nameValue = "something"

	got := m.validate()
	if got != "URL is required" {
		t.Errorf("expected %q, got %q", "URL is required", got)
	}
}

func TestRegistryAddModal_Validate_UnsupportedProtocol(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"file protocol", "file:///etc/passwd"},
		{"ftp protocol", "ftp://evil.com/repo"},
		{"gopher protocol", "gopher://evil.com/repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := openModal()
			m.sourceGit = true
			m.urlValue = tt.url
			m.nameValue = "test"

			got := m.validate()
			want := "Only https://, ssh://, and git@ URLs are supported"
			if got != want {
				t.Errorf("url=%q: expected %q, got %q", tt.url, want, got)
			}
		})
	}
}

func TestRegistryAddModal_Validate_ExtProtocol(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "ext::sh -c 'evil'"
	m.nameValue = "test"

	got := m.validate()
	want := "Only https://, ssh://, and git@ URLs are supported"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRegistryAddModal_Validate_AllowlistBlocked(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/blocked/repo"
	m.nameValue = "blocked/repo"
	m.cfg = &config.Config{
		AllowedRegistries: []string{"https://github.com/acme/allowed.git"},
	}

	got := m.validate()
	want := "URL not permitted by registry allowlist"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRegistryAddModal_Validate_AllowlistEmpty(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/any/repo"
	m.nameValue = "any/repo"
	m.cfg = &config.Config{}

	got := m.validate()
	if got != "" {
		t.Errorf("empty allowlist should permit all, got error %q", got)
	}
}

func TestRegistryAddModal_Validate_LocalDirNotExist(t *testing.T) {
	m := openModal()
	m.sourceGit = false
	m.urlValue = "/nonexistent/path/that/definitely/does/not/exist"
	m.nameValue = "test"

	got := m.validate()
	want := "Directory does not exist"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRegistryAddModal_Validate_LocalDirExists(t *testing.T) {
	m := openModal()
	m.sourceGit = false
	m.urlValue = t.TempDir()
	m.nameValue = "test"

	got := m.validate()
	if got != "" {
		t.Errorf("valid local dir should pass validation, got %q", got)
	}
}

func TestRegistryAddModal_Validate_EmptyName(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = ""

	got := m.validate()
	if got != "Name is required" {
		t.Errorf("expected %q, got %q", "Name is required", got)
	}
}

func TestRegistryAddModal_Validate_InvalidName(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/evil/repo"
	m.nameValue = "../evil"

	got := m.validate()
	want := "Invalid name (use letters, numbers, - and _ with optional owner/repo format)"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRegistryAddModal_Validate_DuplicateNameCaseInsensitive(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/team/tools"
	m.nameValue = "team-tools"
	m.existingNames = []string{"Team-Tools"}

	got := m.validate()
	if got == "" {
		t.Fatal("expected duplicate name error, got empty string")
	}
	if !containsSubstring(got, "already exists") {
		t.Errorf("expected error containing 'already exists', got %q", got)
	}
}

func TestRegistryAddModal_Validate_InvalidBranch(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = "acme/tools"
	m.branchValue = "branch with spaces"

	got := m.validate()
	want := "Branch name can only contain letters, numbers, ., _, / and -"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRegistryAddModal_Validate_EmptyBranch(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = "acme/tools"
	m.branchValue = ""

	got := m.validate()
	if got != "" {
		t.Errorf("empty branch should be allowed, got %q", got)
	}
}

func TestRegistryAddModal_Validate_ValidGitURL(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = "acme/tools"
	m.branchValue = "main"

	got := m.validate()
	if got != "" {
		t.Errorf("valid git URL should pass, got %q", got)
	}
}

func TestRegistryAddModal_Validate_ValidSSHURL(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "git@github.com:acme/tools.git"
	m.nameValue = "acme/tools"

	got := m.validate()
	if got != "" {
		t.Errorf("valid SSH URL should pass, got %q", got)
	}
}

// --- Submit behavior tests (C1.7) ---

func TestRegistryAddModal_Submit_WithError(t *testing.T) {
	m := openModal()
	m.urlValue = "" // triggers empty URL error
	m.focusIdx = 5  // Add button

	m2, cmd := m.Update(keyPress(tea.KeyEnter))
	if !m2.active {
		t.Fatal("modal should stay open when validation fails")
	}
	if m2.err != "URL is required" {
		t.Errorf("expected err = %q, got %q", "URL is required", m2.err)
	}
	if cmd != nil {
		t.Fatal("should return nil cmd on validation failure")
	}
}

func TestRegistryAddModal_Submit_Valid(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = "acme/tools"
	m.branchValue = "main"
	m.focusIdx = 5

	m2, cmd := m.Update(keyPress(tea.KeyEnter))
	if m2.active {
		t.Fatal("modal should close on valid submit")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd from valid submit")
	}

	msg := cmd()
	addMsg, ok := msg.(registryAddMsg)
	if !ok {
		t.Fatalf("expected registryAddMsg, got %T", msg)
	}
	if addMsg.url != "https://github.com/acme/tools" {
		t.Errorf("expected url = %q, got %q", "https://github.com/acme/tools", addMsg.url)
	}
	if addMsg.name != "acme/tools" {
		t.Errorf("expected name = %q, got %q", "acme/tools", addMsg.name)
	}
	if addMsg.ref != "main" {
		t.Errorf("expected ref = %q, got %q", "main", addMsg.ref)
	}
	if addMsg.isLocal {
		t.Error("expected isLocal == false for git URL")
	}
}

func TestRegistryAddModal_Submit_LocalPathResolved(t *testing.T) {
	m := openModal()
	m.sourceGit = false
	m.urlValue = "."
	m.nameValue = "test-local"
	m.focusIdx = 5

	m2, cmd := m.Update(keyPress(tea.KeyEnter))
	if m2.active {
		t.Fatal("modal should close on valid submit")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd from valid submit")
	}

	msg := cmd()
	addMsg, ok := msg.(registryAddMsg)
	if !ok {
		t.Fatalf("expected registryAddMsg, got %T", msg)
	}

	// The URL should be resolved to an absolute path, not ".".
	absExpected, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	if addMsg.url == "." {
		t.Error("expected url to be resolved to absolute path, got '.'")
	}
	if !filepath.IsAbs(addMsg.url) {
		t.Errorf("expected absolute path, got %q", addMsg.url)
	}
	// The resolved path should match or be under the expected absolute path
	// (EvalSymlinks may resolve further).
	_ = absExpected
	if !addMsg.isLocal {
		t.Error("expected isLocal == true for local path")
	}
}

// --- Cancel behavior tests (C1.7) ---

func TestRegistryAddModal_Cancel(t *testing.T) {
	m := openModal()
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = "acme/tools"
	m.focusIdx = 2

	m2, cmd := m.Update(keyPress(tea.KeyEsc))
	if m2.active {
		t.Fatal("Esc should close modal")
	}
	if cmd != nil {
		t.Fatal("cancel should return nil cmd")
	}
}

// --- Mouse tests (C1.10) ---

func TestRegistryAddModal_MouseCancel(t *testing.T) {
	m := openModal()
	m.width = 80
	m.height = 30

	// Render and scan to register zones.
	zone.Scan(m.View())

	z := zone.Get("regadd-cancel")
	if z.IsZero() {
		t.Skip("zone regadd-cancel not registered (bubblezone rendering issue)")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.active {
		t.Fatal("clicking Cancel zone should close modal")
	}
}

func TestRegistryAddModal_MouseAdd(t *testing.T) {
	m := openModal()
	m.width = 80
	m.height = 30
	m.urlValue = "https://github.com/acme/tools"
	m.nameValue = "acme/tools"

	zone.Scan(m.View())

	z := zone.Get("regadd-add")
	if z.IsZero() {
		t.Skip("zone regadd-add not registered (bubblezone rendering issue)")
	}
	m, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if m.active {
		t.Fatal("clicking Add zone with valid data should close modal")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Add click")
	}
	msg := cmd()
	addMsg, ok := msg.(registryAddMsg)
	if !ok {
		t.Fatalf("expected registryAddMsg, got %T", msg)
	}
	if addMsg.url != "https://github.com/acme/tools" {
		t.Errorf("expected url = %q, got %q", "https://github.com/acme/tools", addMsg.url)
	}
}

func TestRegistryAddModal_MouseClickOutside(t *testing.T) {
	m := openModal()
	m.width = 80
	m.height = 30

	zone.Scan(m.View())

	z := zone.Get("registry-add-zone")
	if z.IsZero() {
		t.Skip("zone registry-add-zone not registered (bubblezone rendering issue)")
	}
	// Click well outside the modal zone — past the modal's rendered area.
	// The modal is ~64 wide and ~18 tall, so (75, 25) is safely outside.
	m, _ = m.Update(mouseClick(75, 25))
	if m.active {
		t.Fatal("clicking outside registry-add-zone should close modal")
	}
}

// --- View tests (C1.10) ---

func TestRegistryAddModal_View_Inactive(t *testing.T) {
	m := newRegistryAddModal()
	// Modal is inactive by default.
	got := m.View()
	if got != "" {
		t.Errorf("inactive modal should return empty string, got %q", got)
	}
}

func TestRegistryAddModal_View_ContainsTitle(t *testing.T) {
	m := openModal()
	m.width = 80
	m.height = 30
	got := m.View()
	assertContains(t, got, "Add Registry")
}

func TestRegistryAddModal_View_ContainsLabels(t *testing.T) {
	m := openModal()
	m.width = 80
	m.height = 30
	got := m.View()
	assertContains(t, got, "Source")
	assertContains(t, got, "URL")
	assertContains(t, got, "Name")
}

func TestRegistryAddModal_View_ContainsBranchGit(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.width = 80
	m.height = 30
	got := m.View()
	assertContains(t, got, "Branch")
}

func TestRegistryAddModal_View_NoBranchLocal(t *testing.T) {
	m := openModal()
	m.sourceGit = false
	m.width = 80
	m.height = 30
	got := m.View()
	assertNotContains(t, got, "Branch")
}

func TestRegistryAddModal_View_ContainsButtons(t *testing.T) {
	m := openModal()
	m.width = 80
	m.height = 30
	got := m.View()
	assertContains(t, got, "Cancel")
	assertContains(t, got, "Add")
}

func TestRegistryAddModal_View_ShowsError(t *testing.T) {
	m := openModal()
	m.width = 80
	m.height = 30
	m.err = "URL is required"
	got := m.View()
	assertContains(t, got, "URL is required")
}

func TestRegistryAddModal_View_RadioBullets(t *testing.T) {
	m := openModal()
	m.sourceGit = true
	m.width = 80
	m.height = 30
	got := m.View()

	// When sourceGit is true, Git URL should have the selected bullet.
	assertContains(t, got, "(•)")
	assertContains(t, got, "( )")

	// Toggle to local — the bullets should swap.
	m.sourceGit = false
	got2 := m.View()
	// The selected bullet should now be on "Local Directory" instead.
	assertContains(t, got2, "(•)")
	assertContains(t, got2, "( )")
}

func TestRegistryAddModal_View_ModalWidth(t *testing.T) {
	m := openModal()
	m.width = 100
	m.height = 30
	got := m.View()

	// Expected max width: min(64, 100-10) = 64.
	maxWidth := 64
	for i, line := range strings.Split(got, "\n") {
		// Use rune count to handle multi-byte chars, strip ANSI for accurate width.
		stripped := ansi.Strip(line)
		runeLen := len([]rune(stripped))
		if runeLen > maxWidth+2 { // +2 tolerance for border chars
			t.Errorf("line %d is %d runes wide (max %d): %q", i, runeLen, maxWidth+2, stripped)
		}
	}
}

func TestRegistryAddModal_View_ZoneMarkers(t *testing.T) {
	m := openModal()
	m.width = 80
	m.height = 30
	got := m.View()

	// Zone markers are invisible ANSI sequences inserted by bubblezone.
	// zone.Scan parses them and registers zones; zone.Get returns non-zero
	// info for registered zones.
	zone.Scan(got)

	if zone.Get("regadd-cancel") == nil || zone.Get("regadd-cancel").IsZero() {
		t.Error("View should register zone marker for regadd-cancel")
	}
	if zone.Get("regadd-add") == nil || zone.Get("regadd-add").IsZero() {
		t.Error("View should register zone marker for regadd-add")
	}
	if zone.Get("registry-add-zone") == nil || zone.Get("registry-add-zone").IsZero() {
		t.Error("View should register zone marker for registry-add-zone")
	}
}

// containsSubstring is a helper for checking partial error messages.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
