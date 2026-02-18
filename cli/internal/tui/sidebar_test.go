package tui

import (
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

func TestSidebarViewHasZoneMarks(t *testing.T) {
	m := sidebarModel{
		types:   catalog.AllContentTypes(),
		counts:  map[catalog.ContentType]int{},
		focused: true,
	}
	view := m.View()

	if view == "" {
		t.Fatal("sidebarModel.View() returned empty string")
	}

	// zone.Mark() embeds ANSI escape sequences of the form \x1b[NNNNz around
	// wrapped content. NO_COLOR only suppresses lipgloss styling, not bubblezone
	// markers. When zone marks are present, the output contains the ESC character.
	// This confirms sidebar rows are wrapped with zone.Mark().
	if !strings.Contains(view, "\x1b[") {
		t.Error("sidebarModel.View() output contains no ANSI escape sequences — zone.Mark() calls appear to be missing")
	}
}
