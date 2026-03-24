package tui_v1

import (
	"strings"
	"testing"
)

func TestRenderBreadcrumb(t *testing.T) {
	t.Run("single final segment", func(t *testing.T) {
		result := renderBreadcrumb(BreadcrumbSegment{"Home", ""})
		if !strings.Contains(result, "Home") {
			t.Errorf("expected Home in breadcrumb, got %q", result)
		}
	})

	t.Run("clickable then final", func(t *testing.T) {
		result := renderBreadcrumb(
			BreadcrumbSegment{"Home", "crumb-home"},
			BreadcrumbSegment{"Settings", ""},
		)
		if !strings.Contains(result, "Home") || !strings.Contains(result, "Settings") {
			t.Errorf("expected Home and Settings in breadcrumb, got %q", result)
		}
		// Should contain the > separator
		if !strings.Contains(result, ">") {
			t.Errorf("expected > separator, got %q", result)
		}
	})

	t.Run("three segments", func(t *testing.T) {
		result := renderBreadcrumb(
			BreadcrumbSegment{"Home", "crumb-home"},
			BreadcrumbSegment{"Registries", "crumb-registries"},
			BreadcrumbSegment{"my-registry", ""},
		)
		if !strings.Contains(result, "Home") || !strings.Contains(result, "Registries") || !strings.Contains(result, "my-registry") {
			t.Errorf("expected all segments, got %q", result)
		}
	})
}

func TestRenderStatusMsg(t *testing.T) {
	t.Run("empty message", func(t *testing.T) {
		if got := renderStatusMsg("", false); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("success message", func(t *testing.T) {
		result := renderStatusMsg("Settings saved", false)
		if !strings.Contains(result, "Done: Settings saved") {
			t.Errorf("expected 'Done: Settings saved', got %q", result)
		}
	})

	t.Run("error message", func(t *testing.T) {
		result := renderStatusMsg("File not found", true)
		if !strings.Contains(result, "Error: File not found") {
			t.Errorf("expected 'Error: File not found', got %q", result)
		}
	})
}

func TestCursorPrefix(t *testing.T) {
	t.Run("selected", func(t *testing.T) {
		prefix, _ := cursorPrefix(true)
		if prefix != "> " {
			t.Errorf("expected '> ', got %q", prefix)
		}
	})

	t.Run("not selected", func(t *testing.T) {
		prefix, _ := cursorPrefix(false)
		if prefix != "  " {
			t.Errorf("expected '  ', got %q", prefix)
		}
	})
}

func TestRenderScrollIndicators(t *testing.T) {
	t.Run("list scroll up", func(t *testing.T) {
		result := renderScrollUp(5, false)
		if !strings.Contains(result, "5 more above") {
			t.Errorf("expected '5 more above', got %q", result)
		}
	})

	t.Run("content scroll up", func(t *testing.T) {
		result := renderScrollUp(10, true)
		if !strings.Contains(result, "10 lines above") {
			t.Errorf("expected '10 lines above', got %q", result)
		}
	})

	t.Run("list scroll down", func(t *testing.T) {
		result := renderScrollDown(3, false)
		if !strings.Contains(result, "3 more below") {
			t.Errorf("expected '3 more below', got %q", result)
		}
	})

	t.Run("content scroll down", func(t *testing.T) {
		result := renderScrollDown(7, true)
		if !strings.Contains(result, "7 lines below") {
			t.Errorf("expected '7 lines below', got %q", result)
		}
	})

	t.Run("zero count returns empty", func(t *testing.T) {
		if got := renderScrollUp(0, false); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
		if got := renderScrollDown(0, true); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestRenderDescriptionBox(t *testing.T) {
	t.Run("basic description", func(t *testing.T) {
		result := renderDescriptionBox("This is a description", 40, 3)
		if !strings.Contains(result, "This is a description") {
			t.Errorf("expected description text, got %q", result)
		}
		// Should have separator lines (─)
		if strings.Count(result, "\u2500") < 2 {
			t.Error("expected separator lines")
		}
	})

	t.Run("multi-line description", func(t *testing.T) {
		result := renderDescriptionBox("Line 1\nLine 2\nLine 3", 40, 3)
		if !strings.Contains(result, "Line 1") || !strings.Contains(result, "Line 2") || !strings.Contains(result, "Line 3") {
			t.Errorf("expected all lines, got %q", result)
		}
	})

	t.Run("empty description", func(t *testing.T) {
		if got := renderDescriptionBox("", 40, 3); got != "" {
			t.Errorf("expected empty for empty text, got %q", got)
		}
	})

	t.Run("zero max lines", func(t *testing.T) {
		if got := renderDescriptionBox("text", 40, 0); got != "" {
			t.Errorf("expected empty for zero maxLines, got %q", got)
		}
	})

	t.Run("truncates to maxLines", func(t *testing.T) {
		result := renderDescriptionBox("Line 1\nLine 2\nLine 3\nLine 4", 40, 2)
		if !strings.Contains(result, "Line 1") || !strings.Contains(result, "Line 2") {
			t.Error("expected first 2 lines")
		}
		if strings.Contains(result, "Line 3") || strings.Contains(result, "Line 4") {
			t.Error("should not contain lines beyond maxLines")
		}
	})
}

func TestRenderActionButtons(t *testing.T) {
	t.Run("empty buttons returns empty string", func(t *testing.T) {
		got := renderActionButtons()
		if got != "" {
			t.Errorf("empty buttons should return empty string, got %q", got)
		}
	})
	t.Run("single button contains hotkey and label", func(t *testing.T) {
		got := renderActionButtons(ActionButton{"a", "Create Loadout", "action-a", actionBtnAddStyle})
		if !strings.Contains(got, "[a]") {
			t.Error("button should contain hotkey")
		}
		if !strings.Contains(got, "Create Loadout") {
			t.Error("button should contain label")
		}
	})
	t.Run("multiple buttons are separated", func(t *testing.T) {
		got := renderActionButtons(
			ActionButton{"a", "Add", "action-a", actionBtnAddStyle},
			ActionButton{"r", "Remove", "action-r", actionBtnRemoveStyle},
		)
		if !strings.Contains(got, "[a]") || !strings.Contains(got, "[r]") {
			t.Error("both buttons should appear")
		}
	})
}
