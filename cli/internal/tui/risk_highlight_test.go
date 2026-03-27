package tui

import (
	"strings"
	"testing"
)

func TestPreview_HighlightLines(t *testing.T) {
	t.Parallel()
	p := newPreviewModel()
	p.SetSize(60, 10)
	p.lines = []string{"line one", "line two", "line three", "line four"}
	p.fileName = "test.txt"
	p.SetHighlightLines(map[int]bool{2: true})

	view := p.View()

	// Line 2 should have the gutter marker
	if !strings.Contains(view, "\u258c") { // ▌
		t.Error("expected highlighted line to contain gutter marker")
	}
}

func TestPreview_HighlightGutter(t *testing.T) {
	t.Parallel()
	p := newPreviewModel()
	p.SetSize(60, 10)
	p.lines = []string{"alpha", "beta", "gamma"}
	p.fileName = "test.txt"
	p.SetHighlightLines(map[int]bool{1: true, 3: true})

	view := p.View()

	// Both highlighted lines should have gutter markers
	count := strings.Count(view, "\u258c")
	if count != 2 {
		t.Errorf("expected 2 gutter markers, got %d", count)
	}
}

func TestPreview_NoHighlights(t *testing.T) {
	t.Parallel()
	p := newPreviewModel()
	p.SetSize(60, 10)
	p.lines = []string{"alpha", "beta", "gamma"}
	p.fileName = "test.txt"
	// No highlights set (nil map)

	view := p.View()

	if strings.Contains(view, "\u258c") {
		t.Error("expected no gutter markers without highlights")
	}
}

func TestPreview_HighlightJSON(t *testing.T) {
	t.Parallel()
	p := newPreviewModel()
	p.SetSize(80, 10)
	p.lines = []string{
		`{`,
		`  "hooks": {`,
		`    "command": "echo hello"`,
		`  }`,
		`}`,
	}
	p.fileName = "hooks.json"
	p.SetHighlightLines(map[int]bool{3: true}) // highlight the "command" line

	view := p.View()

	if !strings.Contains(view, "\u258c") {
		t.Error("expected gutter marker on highlighted JSON line")
	}
	if !strings.Contains(view, "command") {
		t.Error("expected 'command' text in view")
	}
}

func TestPreview_HighlightMarkdown(t *testing.T) {
	t.Parallel()
	p := newPreviewModel()
	p.SetSize(80, 10)
	p.lines = []string{
		"# My Skill",
		"",
		"Use the Bash tool to run commands.",
		"",
		"## Details",
	}
	p.fileName = "SKILL.md"
	p.SetHighlightLines(map[int]bool{3: true}) // highlight the Bash line

	view := p.View()

	if !strings.Contains(view, "\u258c") {
		t.Error("expected gutter marker on highlighted markdown line")
	}
	if !strings.Contains(view, "Bash") {
		t.Error("expected 'Bash' text in view")
	}
}

func TestPreview_HighlightYAML(t *testing.T) {
	t.Parallel()
	p := newPreviewModel()
	p.SetSize(80, 10)
	p.lines = []string{
		"name: my-loadout",
		"provider: claude-code",
		"items:",
		"  - type: rules",
	}
	p.fileName = "loadout.yaml"
	p.SetHighlightLines(map[int]bool{2: true})

	view := p.View()

	if !strings.Contains(view, "\u258c") {
		t.Error("expected gutter marker on highlighted YAML line")
	}
	if !strings.Contains(view, "provider") {
		t.Error("expected 'provider' text in view")
	}
}
