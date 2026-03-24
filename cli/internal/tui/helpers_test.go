package tui

import (
	"strings"
	"testing"
)

func TestRenderScrollUp(t *testing.T) {
	result := renderScrollUp(5, false)
	if !strings.Contains(result, "5 more above") {
		t.Error("should show count and 'more above'")
	}

	result = renderScrollUp(3, true)
	if !strings.Contains(result, "3 lines above") {
		t.Error("content view should show 'lines above'")
	}

	result = renderScrollUp(0, false)
	if result != "" {
		t.Error("zero count should return empty")
	}
}

func TestRenderScrollDown(t *testing.T) {
	result := renderScrollDown(10, false)
	if !strings.Contains(result, "10 more below") {
		t.Error("should show count and 'more below'")
	}
}

func TestCursorPrefix(t *testing.T) {
	prefix, _ := cursorPrefix(true)
	if prefix != "> " {
		t.Errorf("selected prefix = %q, want '> '", prefix)
	}

	prefix, _ = cursorPrefix(false)
	if prefix != "  " {
		t.Errorf("unselected prefix = %q, want '  '", prefix)
	}
}

func TestPadToWidth(t *testing.T) {
	result := padToWidth("hi", 10)
	if len(result) < 10 {
		t.Errorf("padToWidth should pad to at least 10 chars, got %d", len(result))
	}
}
