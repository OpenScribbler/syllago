package tui

import "testing"

func TestTopBar_InitialState(t *testing.T) {
	tb := newTopBar()

	if tb.content.ActiveLabel() != "Skills" {
		t.Errorf("expected default Content=Skills, got %q", tb.content.ActiveLabel())
	}
	if tb.collection.ActiveLabel() != "--" {
		t.Errorf("expected default Collection=--, got %q", tb.collection.ActiveLabel())
	}
	if !tb.collection.disabled {
		t.Error("collection should be disabled initially")
	}
	if tb.content.disabled {
		t.Error("content should not be disabled initially")
	}
	if tb.activeCategory != categoryContent {
		t.Error("initial category should be categoryContent")
	}
}

func TestTopBar_OpenDropdown(t *testing.T) {
	tb := newTopBar()

	tb.OpenDropdown(1)
	if !tb.content.isOpen {
		t.Fatal("OpenDropdown(1) should open content")
	}
	if tb.collection.isOpen || tb.config.isOpen {
		t.Fatal("only content should be open")
	}

	tb.OpenDropdown(2)
	if !tb.collection.isOpen {
		t.Fatal("OpenDropdown(2) should open collection")
	}
	if tb.content.isOpen {
		t.Fatal("content should be closed when collection opens")
	}

	tb.OpenDropdown(3)
	if !tb.config.isOpen {
		t.Fatal("OpenDropdown(3) should open config")
	}
}

func TestTopBar_MutualExclusion_ContentDisablesCollection(t *testing.T) {
	tb := newTopBar()

	// Select a collection first
	tb.HandleActiveMsg(dropdownActiveMsg{id: "collection", index: 0, label: "Library"})
	if tb.activeCategory != categoryCollection {
		t.Error("should be categoryCollection after selecting Library")
	}
	if !tb.content.disabled {
		t.Error("content should be disabled after collection selection")
	}

	// Now select content — should disable collection
	tb.HandleActiveMsg(dropdownActiveMsg{id: "content", index: 2, label: "MCP Configs"})
	if tb.activeCategory != categoryContent {
		t.Error("should be categoryContent after content selection")
	}
	if !tb.collection.disabled {
		t.Error("collection should be disabled after content selection")
	}
	if tb.collection.ActiveLabel() != "--" {
		t.Errorf("collection should be reset to --, got %q", tb.collection.ActiveLabel())
	}
}

func TestTopBar_MutualExclusion_CollectionDisablesContent(t *testing.T) {
	tb := newTopBar()

	// Default is content=Skills active
	tb.HandleActiveMsg(dropdownActiveMsg{id: "collection", index: 1, label: "Registries"})

	if !tb.content.disabled {
		t.Error("content should be disabled after collection selection")
	}
	if tb.content.ActiveLabel() != "--" {
		t.Errorf("content should be reset to --, got %q", tb.content.ActiveLabel())
	}
	if tb.collection.ActiveLabel() != "Registries" {
		t.Errorf("collection should show Registries, got %q", tb.collection.ActiveLabel())
	}
}

func TestTopBar_ConfigIsIndependent(t *testing.T) {
	tb := newTopBar()

	// Select config — should not affect content/collection
	tb.HandleActiveMsg(dropdownActiveMsg{id: "config", index: 0, label: "Settings"})

	if tb.content.ActiveLabel() != "Skills" {
		t.Errorf("content should still be Skills, got %q", tb.content.ActiveLabel())
	}
	if tb.activeCategory != categoryContent {
		t.Error("active category should still be content")
	}
}

func TestTopBar_HasOpenDropdown(t *testing.T) {
	tb := newTopBar()

	if tb.HasOpenDropdown() {
		t.Fatal("should not have open dropdown initially")
	}

	tb.OpenDropdown(1)
	if !tb.HasOpenDropdown() {
		t.Fatal("should have open dropdown after OpenDropdown")
	}
}

func TestTopBar_Height(t *testing.T) {
	tb := newTopBar()

	if h := tb.Height(); h != 1 {
		t.Errorf("closed topbar should have height 1, got %d", h)
	}

	tb.OpenDropdown(1) // Content has 6 items
	if h := tb.Height(); h != 9 {
		// 1 bar + 6 items + 2 border
		t.Errorf("open content dropdown should have height 9, got %d", h)
	}
}

func TestTopBar_RenderBar(t *testing.T) {
	tb := newTopBar()
	tb.SetSize(80)

	view := tb.View()
	assertContains(t, view, "syl")
	assertContains(t, view, "lago")
	assertContains(t, view, "Content: Skills")
	assertContains(t, view, "Collection: --")
	assertContains(t, view, "+ Add")
	assertContains(t, view, "* New")
}

func TestTopBar_RenderOpenDropdown(t *testing.T) {
	tb := newTopBar()
	tb.SetSize(80)
	tb.OpenDropdown(1)

	view := tb.View()
	assertContains(t, view, "> Skills")
	assertContains(t, view, "  Agents")
	assertContains(t, view, "  Hooks")
}
