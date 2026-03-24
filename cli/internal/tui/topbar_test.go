package tui

import "testing"

func TestTopBar_InitialState(t *testing.T) {
	tb := newTopBar()

	if tb.activeGroup != 0 {
		t.Errorf("expected initial group 0 (Content), got %d", tb.activeGroup)
	}
	if tb.activeTab != 0 {
		t.Errorf("expected initial tab 0 (Skills), got %d", tb.activeTab)
	}
	if tb.ActiveGroupLabel() != "Content" {
		t.Errorf("expected Content, got %q", tb.ActiveGroupLabel())
	}
	if tb.ActiveTabLabel() != "Skills" {
		t.Errorf("expected Skills, got %q", tb.ActiveTabLabel())
	}
}

func TestTopBar_SetGroup(t *testing.T) {
	tb := newTopBar()

	tb.SetGroup(1) // Collections
	if tb.activeGroup != 1 {
		t.Errorf("expected group 1, got %d", tb.activeGroup)
	}
	if tb.ActiveGroupLabel() != "Collections" {
		t.Errorf("expected Collections, got %q", tb.ActiveGroupLabel())
	}
	if tb.activeTab != 0 {
		t.Errorf("SetGroup should reset tab to 0, got %d", tb.activeTab)
	}
	if tb.ActiveTabLabel() != "Library" {
		t.Errorf("expected Library, got %q", tb.ActiveTabLabel())
	}

	tb.SetGroup(2) // Config
	if tb.ActiveGroupLabel() != "Config" {
		t.Errorf("expected Config, got %q", tb.ActiveGroupLabel())
	}
	if tb.ActiveTabLabel() != "Settings" {
		t.Errorf("expected Settings, got %q", tb.ActiveTabLabel())
	}
}

func TestTopBar_NextPrevTab(t *testing.T) {
	tb := newTopBar()
	// Content group: Skills, Agents, MCP, Rules, Hooks, Commands

	tb.NextTab()
	if tb.ActiveTabLabel() != "Agents" {
		t.Errorf("expected Agents, got %q", tb.ActiveTabLabel())
	}

	tb.NextTab()
	if tb.ActiveTabLabel() != "MCP" {
		t.Errorf("expected MCP, got %q", tb.ActiveTabLabel())
	}

	tb.PrevTab()
	if tb.ActiveTabLabel() != "Agents" {
		t.Errorf("expected Agents after PrevTab, got %q", tb.ActiveTabLabel())
	}
}

func TestTopBar_TabWraps(t *testing.T) {
	tb := newTopBar()

	// Wrap backward from first
	tb.PrevTab()
	if tb.ActiveTabLabel() != "Commands" {
		t.Errorf("expected Commands (wrap), got %q", tb.ActiveTabLabel())
	}

	// Wrap forward from last
	tb.NextTab()
	if tb.ActiveTabLabel() != "Skills" {
		t.Errorf("expected Skills (wrap), got %q", tb.ActiveTabLabel())
	}
}

func TestTopBar_Height(t *testing.T) {
	tb := newTopBar()
	if h := tb.Height(); h != 5 {
		t.Errorf("expected height 5, got %d", h)
	}
}

func TestTopBar_RenderContainsElements(t *testing.T) {
	tb := newTopBar()
	tb.SetSize(80)

	view := tb.View()
	assertContains(t, view, "syllago")
	assertContains(t, view, "Content")
	assertContains(t, view, "Collections")
	assertContains(t, view, "Config")
	assertContains(t, view, "Skills")
	assertContains(t, view, "Agents")
	assertContains(t, view, "[a] Add")
	assertContains(t, view, "[n] Create")
}

func TestTopBar_GroupSwitchShowsDifferentTabs(t *testing.T) {
	tb := newTopBar()
	tb.SetSize(80)

	tb.SetGroup(1) // Collections
	view := tb.View()
	assertContains(t, view, "Library")
	assertContains(t, view, "Registries")
	assertContains(t, view, "Loadouts")

	tb.SetGroup(2) // Config
	view = tb.View()
	assertContains(t, view, "Settings")
	assertContains(t, view, "Sandbox")
}

func TestTopBar_SetGroupOutOfBounds(t *testing.T) {
	tb := newTopBar()
	tb.SetGroup(99) // should be no-op
	if tb.activeGroup != 0 {
		t.Errorf("out-of-bounds SetGroup should be no-op, got group %d", tb.activeGroup)
	}
}
