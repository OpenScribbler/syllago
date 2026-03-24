package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestToastSuccess(t *testing.T) {
	toast := showToast("alpha-skill installed to Claude Code", toastSuccess)
	toast.width = 80
	view := toast.View()

	if !strings.Contains(view, "Done:") {
		t.Error("success toast should contain 'Done:' prefix")
	}
	if !strings.Contains(view, "alpha-skill") {
		t.Error("toast should contain message text")
	}
	if !strings.Contains(view, "c copy") {
		t.Error("toast should show copy hint")
	}
}

func TestToastError(t *testing.T) {
	toast := showToast("Failed to install\nPermission denied", toastError)
	toast.width = 80
	view := toast.View()

	if !strings.Contains(view, "Error:") {
		t.Error("error toast should contain 'Error:' prefix")
	}
}

func TestToastAutoDismiss(t *testing.T) {
	toast := showToast("done", toastSuccess)
	cmd := toast.autoDismissCmd()
	if cmd == nil {
		t.Error("success toast should have auto-dismiss cmd")
	}

	errorToast := showToast("error", toastError)
	cmd = errorToast.autoDismissCmd()
	if cmd != nil {
		t.Error("error toast should NOT have auto-dismiss cmd")
	}
}

func TestToastDismissOnKey(t *testing.T) {
	toast := showToast("done", toastSuccess)
	toast.width = 80

	// Any key dismisses success toast
	updated, _ := toast.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if updated.active {
		t.Error("success toast should dismiss on any key")
	}
}

func TestToastErrorPersists(t *testing.T) {
	toast := showToast("error", toastError)
	toast.width = 80

	// Regular keys don't dismiss error toasts
	updated, _ := toast.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !updated.active {
		t.Error("error toast should persist on regular keys")
	}

	// Esc dismisses
	updated, _ = toast.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.active {
		t.Error("error toast should dismiss on Esc")
	}
}

func TestToastCopy(t *testing.T) {
	toast := showToast("some error text", toastError)
	toast.width = 80

	updated, cmd := toast.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if updated.active {
		t.Error("toast should dismiss after copy")
	}
	if cmd == nil {
		t.Error("copy should return a cmd")
	}
}

func TestToastInactive(t *testing.T) {
	toast := toastModel{active: false, width: 80}
	view := toast.View()
	if view != "" {
		t.Error("inactive toast should render empty string")
	}
}
