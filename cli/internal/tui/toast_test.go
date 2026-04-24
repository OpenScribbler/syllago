package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestToast_PushAndCurrent(t *testing.T) {
	tm := newToastModel()
	if tm.Current() != nil {
		t.Fatal("new toast model should have no current toast")
	}

	tm.Push("hello", toastSuccess)
	cur := tm.Current()
	if cur == nil {
		t.Fatal("expected current toast after Push")
	}
	if cur.message != "hello" {
		t.Errorf("expected message %q, got %q", "hello", cur.message)
	}
	if cur.level != toastSuccess {
		t.Errorf("expected level toastSuccess, got %d", cur.level)
	}
	if !tm.visible {
		t.Fatal("toast should be visible after Push")
	}
}

func TestToast_Queue(t *testing.T) {
	tm := newToastModel()
	tm.Push("first", toastSuccess)
	tm.Push("second", toastWarning)
	tm.Push("third", toastError)

	if len(tm.queue) != 3 {
		t.Fatalf("expected 3 queued toasts, got %d", len(tm.queue))
	}
	if tm.Current().message != "first" {
		t.Errorf("expected first toast to be current, got %q", tm.Current().message)
	}

	// Dismiss first, second becomes current
	tm.Dismiss()
	if tm.Current().message != "second" {
		t.Errorf("expected second toast after dismiss, got %q", tm.Current().message)
	}

	// Dismiss second, third becomes current
	tm.Dismiss()
	if tm.Current().message != "third" {
		t.Errorf("expected third toast after second dismiss, got %q", tm.Current().message)
	}

	// Dismiss third, nothing left
	tm.Dismiss()
	if tm.Current() != nil {
		t.Fatal("expected no current toast after dismissing all")
	}
	if tm.visible {
		t.Fatal("toast should not be visible when queue is empty")
	}
}

func TestToast_TickDismissesSuccessAndWarning(t *testing.T) {
	for _, level := range []toastLevel{toastSuccess, toastWarning} {
		tm := newToastModel()
		tm.Push("msg", level)
		seq := tm.seq

		tm, _ = tm.Update(toastTickMsg{seq: seq})
		if tm.visible {
			t.Errorf("level %d: toast should be dismissed after tick", level)
		}
	}
}

func TestToast_TickDoesNotDismissError(t *testing.T) {
	tm := newToastModel()
	tm.Push("error msg", toastError)
	seq := tm.seq

	tm, _ = tm.Update(toastTickMsg{seq: seq})
	if !tm.visible {
		t.Fatal("error toast should NOT be dismissed by tick")
	}
	if tm.Current().message != "error msg" {
		t.Fatal("error toast should still be current after tick")
	}
}

func TestToast_StaleTick(t *testing.T) {
	tm := newToastModel()
	tm.Push("first", toastSuccess)
	staleSeq := tm.seq

	// Push another toast (increments seq)
	tm.Dismiss()
	tm.Push("second", toastSuccess)

	// Old tick should be ignored
	tm, _ = tm.Update(toastTickMsg{seq: staleSeq})
	if !tm.visible {
		t.Fatal("stale tick should not dismiss current toast")
	}
}

func TestToast_HandleKeyEscDismisses(t *testing.T) {
	tm := newToastModel()
	tm.Push("msg", toastSuccess)

	consumed, _ := tm.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !consumed {
		t.Fatal("Esc should be consumed by visible toast")
	}
	if tm.visible {
		t.Fatal("toast should be dismissed after Esc")
	}
}

func TestToast_HandleKeyCCopiesErrorOnly(t *testing.T) {
	// 'c' on a success toast should not be consumed
	tm := newToastModel()
	tm.Push("success msg", toastSuccess)
	consumed, _ := tm.HandleKey(keyRune('c'))
	if consumed {
		t.Fatal("'c' should not be consumed for success toasts")
	}

	// 'c' on an error toast should be consumed and dismiss
	tm2 := newToastModel()
	tm2.Push("error msg", toastError)
	consumed, _ = tm2.HandleKey(keyRune('c'))
	if !consumed {
		t.Fatal("'c' should be consumed for error toasts")
	}
	if tm2.visible {
		t.Fatal("error toast should be dismissed after 'c'")
	}
}

func TestToast_HandleKeyNoopWhenInvisible(t *testing.T) {
	tm := newToastModel()
	consumed, cmd := tm.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if consumed {
		t.Fatal("invisible toast should not consume keys")
	}
	if cmd != nil {
		t.Fatal("invisible toast should not produce commands")
	}
}

func TestToast_ViewSuccess(t *testing.T) {
	tm := newToastModel()
	tm.Push("Item saved", toastSuccess)

	view := tm.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "Item saved") {
		t.Error("success toast view should contain the message")
	}
	// Should not contain error-specific hints
	if strings.Contains(stripped, "[esc]") {
		t.Error("success toast should not show [esc] dismiss hint")
	}
}

func TestToast_ViewError(t *testing.T) {
	tm := newToastModel()
	tm.Push("Something broke", toastError)

	view := tm.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "Something broke") {
		t.Error("error toast view should contain the message")
	}
	if !strings.Contains(stripped, "[esc] dismiss") {
		t.Error("error toast should show dismiss hint")
	}
	if !strings.Contains(stripped, "[c] copy") {
		t.Error("error toast should show copy hint")
	}
}

func TestToast_CloseButtonInAllLevels(t *testing.T) {
	for _, level := range []toastLevel{toastSuccess, toastWarning, toastError} {
		tm := newToastModel()
		tm.Push("msg", level)

		view := tm.View()
		stripped := ansi.Strip(view)
		if !strings.Contains(stripped, "[×]") {
			t.Errorf("level %d: toast should contain [×] close button", level)
		}
	}
}

func TestToast_HandleMouseClose(t *testing.T) {
	tm := newToastModel()
	tm.Push("msg", toastSuccess)

	// Invisible toast should not consume mouse events
	tm2 := newToastModel()
	consumed, _ := tm2.HandleMouse(tea.MouseMsg{})
	if consumed {
		t.Fatal("invisible toast should not consume mouse events")
	}

	// Visible toast — we can't easily simulate zone bounds in unit tests,
	// but verify the method exists and returns false for out-of-bounds clicks
	consumed, _ = tm.HandleMouse(tea.MouseMsg{})
	if consumed {
		t.Fatal("out-of-bounds mouse click should not be consumed")
	}
}

func TestToast_ViewEmptyWhenInvisible(t *testing.T) {
	tm := newToastModel()
	if tm.View() != "" {
		t.Error("invisible toast should return empty view")
	}
}

func TestToast_MessageTruncation(t *testing.T) {
	tm := newToastModel()
	long := strings.Repeat("a", 100)
	tm.Push(long, toastWarning)

	view := tm.View()
	stripped := ansi.Strip(view)
	// Should be truncated (max 50 chars)
	if strings.Contains(stripped, long) {
		t.Error("long message should be truncated in view")
	}
}

func TestToast_ConsistentWidth(t *testing.T) {
	// All toast levels should render at the same width, regardless of message length
	messages := []string{"OK", "Medium length message here", strings.Repeat("x", 80)}
	levels := []toastLevel{toastSuccess, toastWarning, toastError}

	var widths []int
	for _, level := range levels {
		for _, msg := range messages {
			tm := newToastModel()
			tm.Push(msg, level)
			view := tm.View()
			lines := strings.Split(view, "\n")
			for _, line := range lines {
				w := lipgloss.Width(line)
				if w > 0 {
					widths = append(widths, w)
				}
			}
		}
	}

	// All non-zero widths should be the same (62 = 60 + 2 border chars)
	if len(widths) == 0 {
		t.Fatal("expected rendered toast lines")
	}
	expected := widths[0]
	for i, w := range widths {
		if w != expected {
			t.Errorf("width[%d] = %d, expected %d (all toasts should be same width)", i, w, expected)
		}
	}
}

func TestToast_PushReturnsTickCmd(t *testing.T) {
	tm := newToastModel()
	cmd := tm.Push("hello", toastSuccess)
	if cmd == nil {
		t.Fatal("Push for first toast should return a tick command")
	}
}

func TestToast_PushErrorNoTickCmd(t *testing.T) {
	tm := newToastModel()
	cmd := tm.Push("error", toastError)
	if cmd != nil {
		t.Fatal("Push for error toast should not return a tick command")
	}
}

func TestToast_PushDropsOldestNonError_WhenOverLimit(t *testing.T) {
	tm := newToastModel()
	// Push 3 successes (fills queue)
	tm.Push("a", toastSuccess)
	tm.Push("b", toastSuccess)
	tm.Push("c", toastSuccess)
	// 4th should drop "a" (oldest non-error)
	tm.Push("d", toastSuccess)

	if len(tm.queue) != maxVisibleToasts {
		t.Fatalf("queue len = %d, want %d", len(tm.queue), maxVisibleToasts)
	}
	for _, e := range tm.queue {
		if e.message == "a" {
			t.Error("oldest non-error toast should have been dropped")
		}
	}
}

func TestToast_PushKeepsAllErrorsWhenOverLimit(t *testing.T) {
	tm := newToastModel()
	tm.Push("e1", toastError)
	tm.Push("e2", toastError)
	tm.Push("e3", toastError)
	// 4th error — all errors, none can be dropped, queue grows
	tm.Push("e4", toastError)

	if len(tm.queue) != 4 {
		t.Fatalf("queue len = %d, want 4 (all errors kept)", len(tm.queue))
	}
}

func TestToast_PushDropsSuccess_PrefersErrors(t *testing.T) {
	tm := newToastModel()
	tm.Push("err1", toastError)
	tm.Push("succ1", toastSuccess)
	tm.Push("err2", toastError)
	// 4th push overflows, should drop succ1 (the only non-error)
	tm.Push("succ2", toastSuccess)

	found := false
	for _, e := range tm.queue {
		if e.message == "succ1" {
			found = true
		}
	}
	if found {
		t.Error("succ1 should have been dropped when queue overflows")
	}
	if len(tm.queue) != maxVisibleToasts {
		t.Errorf("queue len = %d, want %d", len(tm.queue), maxVisibleToasts)
	}
}

func TestToast_QueuedPushNoCmd(t *testing.T) {
	tm := newToastModel()
	tm.Push("first", toastSuccess)
	cmd := tm.Push("second", toastWarning)
	if cmd != nil {
		t.Fatal("Push when a toast is already showing should return nil")
	}
}

func TestOverlayToast_Placement(t *testing.T) {
	// Create a simple background
	bg := strings.Repeat(strings.Repeat(".", 80)+"\n", 24)
	bg = strings.TrimRight(bg, "\n")

	tm := newToastModel()
	tm.Push("test", toastSuccess)
	toast := tm.View()

	result := overlayToast(bg, toast, 80, 24)
	lines := strings.Split(result, "\n")
	if len(lines) != 24 {
		t.Errorf("expected 24 lines, got %d", len(lines))
	}

	// Toast should appear near the bottom (not at the very top)
	toastFound := false
	for i := len(lines) / 2; i < len(lines); i++ {
		if strings.Contains(ansi.Strip(lines[i]), "test") {
			toastFound = true
			break
		}
	}
	if !toastFound {
		t.Error("toast should appear in the bottom half of the content area")
	}
}

func TestApp_ToastOnEditSave(t *testing.T) {
	app := testAppWithItems(t)

	// Simulate an edit save message — path won't exist so it'll error
	m, cmd := app.Update(editSavedMsg{name: "test", description: "desc", path: "/nonexistent"})
	a := m.(App)
	_ = cmd // may produce toast tick

	if !a.toast.visible {
		t.Fatal("expected toast to be visible after edit save attempt")
	}
}

func TestIsFilePath(t *testing.T) {
	// Existing directory
	dir := t.TempDir()
	if isFilePath(dir) {
		t.Error("expected directory to return false")
	}

	// Existing file
	f := filepath.Join(dir, "hook.json")
	os.WriteFile(f, []byte("{}"), 0644)
	if !isFilePath(f) {
		t.Error("expected file to return true")
	}

	// Non-existent path with extension (heuristic)
	if !isFilePath("/does/not/exist/hook.json") {
		t.Error("expected non-existent path with extension to return true")
	}

	// Non-existent path without extension (directory heuristic)
	if isFilePath("/does/not/exist/hookdir") {
		t.Error("expected non-existent path without extension to return false")
	}
}

func TestEditSave_SingleFileHook(t *testing.T) {
	dir := t.TempDir()
	hookFile := filepath.Join(dir, "my-hook.json")
	os.WriteFile(hookFile, []byte(`{"event":"PreToolUse"}`), 0644)

	app := testAppWithItems(t)
	m, _ := app.Update(editSavedMsg{name: "My Hook", description: "Validates tools", path: hookFile})
	a := m.(App)

	// Should produce a success toast (not error)
	if !a.toast.visible {
		t.Fatal("expected toast after edit save")
	}
	cur := a.toast.Current()
	if cur.level != toastSuccess {
		t.Errorf("expected success toast, got level %d with message: %s", cur.level, cur.message)
	}

	// Should have written provider-specific metadata file
	metaPath := filepath.Join(dir, ".syllago.my-hook.json.yaml")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("expected provider metadata file at %s: %v", metaPath, err)
	}
	if !strings.Contains(string(data), "My Hook") {
		t.Error("metadata file should contain the display name")
	}
	if !strings.Contains(string(data), "Validates tools") {
		t.Error("metadata file should contain the description")
	}
}

func TestEditSave_DirectoryItem(t *testing.T) {
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(itemDir, 0755)

	app := testAppWithItems(t)
	m, _ := app.Update(editSavedMsg{name: "My Skill", description: "Does things", path: itemDir})
	a := m.(App)

	if !a.toast.visible {
		t.Fatal("expected toast after edit save")
	}
	cur := a.toast.Current()
	if cur.level != toastSuccess {
		t.Errorf("expected success toast, got level %d with message: %s", cur.level, cur.message)
	}

	// Should have written directory-level metadata file
	metaPath := filepath.Join(itemDir, ".syllago.yaml")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("expected metadata file at %s: %v", metaPath, err)
	}
	if !strings.Contains(string(data), "My Skill") {
		t.Error("metadata file should contain the display name")
	}
}
