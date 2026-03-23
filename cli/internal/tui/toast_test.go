package tui

import (
	"strings"
	"testing"
)

func TestToastModel_ShowAndDismiss(t *testing.T) {
	var tm toastModel
	tm.width = 60

	tm.show(toastMsg{text: "Settings saved", isErr: false})
	if !tm.active {
		t.Error("expected toast to be active after show")
	}
	if tm.text != "Settings saved" {
		t.Errorf("expected text 'Settings saved', got %q", tm.text)
	}

	tm.dismiss()
	if tm.active {
		t.Error("expected toast to be inactive after dismiss")
	}
}

func TestToastModel_SuccessView(t *testing.T) {
	tm := toastModel{active: true, text: "Installed to Claude Code", width: 60}
	view := tm.view()
	if !strings.Contains(view, "Done: Installed to Claude Code") {
		t.Errorf("expected 'Done: Installed to Claude Code' in view, got %q", view)
	}
}

func TestToastModel_ErrorView(t *testing.T) {
	tm := toastModel{active: true, text: "File not found", isErr: true, width: 60}
	view := tm.view()
	if !strings.Contains(view, "Error: File not found") {
		t.Errorf("expected 'Error: File not found' in view, got %q", view)
	}
	if !strings.Contains(view, "c copy") {
		t.Errorf("expected 'c copy' hint in error toast, got %q", view)
	}
}

func TestToastModel_InactiveViewEmpty(t *testing.T) {
	tm := toastModel{active: false, text: "hello", width: 60}
	if view := tm.view(); view != "" {
		t.Errorf("expected empty view for inactive toast, got %q", view)
	}
}

func TestSanitizeForClipboard(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			name:     "strips absolute paths",
			input:    "error reading /home/user/.config/syllago/config.yaml",
			contains: "<path>",
			excludes: "/home/user",
		},
		{
			name:     "strips git URLs",
			input:    "failed to clone https://github.com/owner/repo.git",
			contains: "<url>",
			excludes: "github.com",
		},
		{
			name:     "redacts secret values",
			input:    "missing API_KEY=sk-abc123 in environment",
			contains: "API_KEY=<redacted>",
			excludes: "sk-abc123",
		},
		{
			name:     "preserves normal text",
			input:    "connection timed out after 5s",
			contains: "connection timed out after 5s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeForClipboard(tt.input)
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, result)
			}
			if tt.excludes != "" && strings.Contains(result, tt.excludes) {
				t.Errorf("expected result to NOT contain %q, got %q", tt.excludes, result)
			}
		})
	}
}
