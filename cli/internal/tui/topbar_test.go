package tui

import "testing"

func TestTopBar_InitialState(t *testing.T) {
	tb := newTopBar()

	if tb.activeGroup != 0 {
		t.Errorf("expected initial group 0 (Collections), got %d", tb.activeGroup)
	}
	if tb.activeTab != 0 {
		t.Errorf("expected initial tab 0 (Library), got %d", tb.activeTab)
	}
	if tb.ActiveGroupLabel() != "Collections" {
		t.Errorf("expected Collections, got %q", tb.ActiveGroupLabel())
	}
	if tb.ActiveTabLabel() != "Library" {
		t.Errorf("expected Library, got %q", tb.ActiveTabLabel())
	}
}

func TestTopBar_SetGroup(t *testing.T) {
	tb := newTopBar()

	tb.SetGroup(1) // Content
	if tb.activeGroup != 1 {
		t.Errorf("expected group 1, got %d", tb.activeGroup)
	}
	if tb.ActiveGroupLabel() != "Content" {
		t.Errorf("expected Content, got %q", tb.ActiveGroupLabel())
	}
	if tb.activeTab != 0 {
		t.Errorf("SetGroup should reset tab to 0, got %d", tb.activeTab)
	}
	if tb.ActiveTabLabel() != "Skills" {
		t.Errorf("expected Skills, got %q", tb.ActiveTabLabel())
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
	// Collections group: Library, Registries, Loadouts

	tb.NextTab()
	if tb.ActiveTabLabel() != "Registries" {
		t.Errorf("expected Registries, got %q", tb.ActiveTabLabel())
	}

	tb.NextTab()
	if tb.ActiveTabLabel() != "Loadouts" {
		t.Errorf("expected Loadouts, got %q", tb.ActiveTabLabel())
	}

	tb.PrevTab()
	if tb.ActiveTabLabel() != "Registries" {
		t.Errorf("expected Registries after PrevTab, got %q", tb.ActiveTabLabel())
	}
}

func TestTopBar_TabWraps(t *testing.T) {
	tb := newTopBar()

	// Wrap backward from first (Library -> Loadouts)
	tb.PrevTab()
	if tb.ActiveTabLabel() != "Loadouts" {
		t.Errorf("expected Loadouts (wrap), got %q", tb.ActiveTabLabel())
	}

	// Wrap forward from last (Loadouts -> Library)
	tb.NextTab()
	if tb.ActiveTabLabel() != "Library" {
		t.Errorf("expected Library (wrap), got %q", tb.ActiveTabLabel())
	}
}

func TestTopBar_Height(t *testing.T) {
	tb := newTopBar()
	if h := tb.Height(); h != 6 {
		t.Errorf("expected height 6, got %d", h)
	}
}

func TestTopBar_RenderContainsElements(t *testing.T) {
	tb := newTopBar()
	tb.SetSize(80)

	view := tb.View()
	assertContains(t, view, "syllago")
	assertContains(t, view, "Collections")
	assertContains(t, view, "Content")
	assertContains(t, view, "Config")
	assertContains(t, view, "Library")
	assertContains(t, view, "Registries")
	assertContains(t, view, "[a] Add")
	assertContains(t, view, "[n] Create")
}

func TestTopBar_GroupSwitchShowsDifferentTabs(t *testing.T) {
	tb := newTopBar()
	tb.SetSize(80)

	tb.SetGroup(1) // Content
	view := tb.View()
	assertContains(t, view, "Skills")
	assertContains(t, view, "Agents")
	assertContains(t, view, "Commands")

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
