package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestRiskBanner_RenderHigh(t *testing.T) {
	t.Parallel()
	b := newRiskBanner([]catalog.RiskIndicator{
		{Label: "Runs commands", Description: "bash -c 'curl ...'", Level: catalog.RiskHigh},
	}, 60)

	view := ansi.Strip(b.View())
	if !strings.Contains(view, "!!") {
		t.Error("expected \"!!\" severity icon for RiskHigh")
	}
	if !strings.Contains(view, "Runs commands") {
		t.Error("expected label in view")
	}
	if b.borderColor() != dangerColor {
		t.Error("expected dangerColor border for RiskHigh")
	}
}

func TestRiskBanner_RenderMediumOnly(t *testing.T) {
	t.Parallel()
	b := newRiskBanner([]catalog.RiskIndicator{
		{Label: "Network access", Description: "Hook makes HTTP requests", Level: catalog.RiskMedium},
		{Label: "Environment variables", Description: "Reads env vars", Level: catalog.RiskMedium},
	}, 60)

	if b.borderColor() != warningColor {
		t.Error("expected warningColor border when all risks are RiskMedium")
	}
	view := ansi.Strip(b.View())
	if !strings.Contains(view, "!") {
		t.Error("expected \"!\" severity icon for RiskMedium")
	}
}

func TestRiskBanner_RenderMixed(t *testing.T) {
	t.Parallel()
	b := newRiskBanner([]catalog.RiskIndicator{
		{Label: "Network access", Description: "HTTP requests", Level: catalog.RiskMedium},
		{Label: "Runs commands", Description: "shell execution", Level: catalog.RiskHigh},
	}, 60)

	if b.borderColor() != dangerColor {
		t.Error("expected dangerColor border when any risk is RiskHigh")
	}
}

func TestRiskBanner_Navigation(t *testing.T) {
	t.Parallel()
	risks := []catalog.RiskIndicator{
		{Label: "Risk A", Level: catalog.RiskMedium},
		{Label: "Risk B", Level: catalog.RiskMedium},
		{Label: "Risk C", Level: catalog.RiskHigh},
	}
	b := newRiskBanner(risks, 60)

	if b.cursor != 0 {
		t.Fatalf("expected initial cursor=0, got %d", b.cursor)
	}

	// Down: 0 -> 1
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyDown})
	if b.cursor != 1 {
		t.Errorf("expected cursor=1 after down, got %d", b.cursor)
	}

	// Down: 1 -> 2
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyDown})
	if b.cursor != 2 {
		t.Errorf("expected cursor=2 after second down, got %d", b.cursor)
	}

	// Down: clamped at 2
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyDown})
	if b.cursor != 2 {
		t.Errorf("expected cursor=2 (clamped), got %d", b.cursor)
	}

	// Up: 2 -> 1
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyUp})
	if b.cursor != 1 {
		t.Errorf("expected cursor=1 after up, got %d", b.cursor)
	}

	// Up: 1 -> 0
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyUp})
	if b.cursor != 0 {
		t.Errorf("expected cursor=0 after second up, got %d", b.cursor)
	}

	// Up: clamped at 0
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyUp})
	if b.cursor != 0 {
		t.Errorf("expected cursor=0 (clamped), got %d", b.cursor)
	}

	// j/k vim keys
	b, _ = b.Update(keyRune('j'))
	if b.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", b.cursor)
	}
	b, _ = b.Update(keyRune('k'))
	if b.cursor != 0 {
		t.Errorf("expected cursor=0 after k, got %d", b.cursor)
	}
}

func TestRiskBanner_Enter(t *testing.T) {
	t.Parallel()
	risks := []catalog.RiskIndicator{
		{Label: "Runs commands", Description: "shell execution", Level: catalog.RiskHigh},
		{Label: "Network access", Description: "HTTP requests", Level: catalog.RiskMedium},
	}
	b := newRiskBanner(risks, 60)

	// cursor starts at 0; press Enter
	b, cmd := b.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Enter")
	}
	msg := cmd()
	drillIn, ok := msg.(riskDrillInMsg)
	if !ok {
		t.Fatalf("expected riskDrillInMsg, got %T", msg)
	}
	if drillIn.risk.Label != "Runs commands" {
		t.Errorf("expected risk label %q, got %q", "Runs commands", drillIn.risk.Label)
	}

	// Navigate to second item and press Enter
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, cmd = b.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Enter on second item")
	}
	msg = cmd()
	drillIn = msg.(riskDrillInMsg)
	if drillIn.risk.Label != "Network access" {
		t.Errorf("expected risk label %q, got %q", "Network access", drillIn.risk.Label)
	}
}

func TestRiskBanner_SingleItem(t *testing.T) {
	t.Parallel()
	b := newRiskBanner([]catalog.RiskIndicator{
		{Label: "Solo risk", Description: "only one", Level: catalog.RiskMedium},
	}, 60)

	if b.cursor != 0 {
		t.Fatalf("expected cursor=0 for single item, got %d", b.cursor)
	}

	// Down stays at 0
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyDown})
	if b.cursor != 0 {
		t.Errorf("expected cursor=0 after down on single item, got %d", b.cursor)
	}

	// Enter still works
	_, cmd := b.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Enter on single item")
	}
	msg := cmd()
	drillIn, ok := msg.(riskDrillInMsg)
	if !ok {
		t.Fatalf("expected riskDrillInMsg, got %T", msg)
	}
	if drillIn.risk.Label != "Solo risk" {
		t.Errorf("expected risk label %q, got %q", "Solo risk", drillIn.risk.Label)
	}
}

func TestRiskBanner_Empty(t *testing.T) {
	t.Parallel()
	b := newRiskBanner(nil, 60)

	if !b.IsEmpty() {
		t.Error("expected IsEmpty() to return true for nil risks")
	}
	if v := b.View(); v != "" {
		t.Errorf("expected empty view for nil risks, got %q", v)
	}

	// Update is a no-op on empty banner
	b, cmd := b.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd from Enter on empty banner")
	}

	// Also test with empty slice (not nil)
	b2 := newRiskBanner([]catalog.RiskIndicator{}, 60)
	if !b2.IsEmpty() {
		t.Error("expected IsEmpty() for empty slice")
	}
	if v := b2.View(); v != "" {
		t.Errorf("expected empty view for empty slice, got %q", v)
	}
}

func TestRiskBanner_Truncate(t *testing.T) {
	t.Parallel()
	longDesc := strings.Repeat("x", 200)
	b := newRiskBanner([]catalog.RiskIndicator{
		{Label: "Long risk", Description: longDesc, Level: catalog.RiskHigh},
	}, 60)

	view := ansi.Strip(b.View())
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		// Use lipgloss.Width for visual width (handles multi-byte unicode box chars).
		w := lipgloss.Width(line)
		if w > 60 {
			t.Errorf("line exceeds visual width 60: width=%d, line=%q", w, line)
		}
	}

	// Verify truncation happened (description should end with "...")
	if !strings.Contains(view, "...") {
		t.Error("expected truncated description to contain \"...\"")
	}
}
