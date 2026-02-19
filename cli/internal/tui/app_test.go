package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAppHasSidebarField(t *testing.T) {
	// Verify App struct has a sidebar field by constructing a zero-value App
	// and checking the sidebar field is accessible (compile-time check).
	var a App
	_ = a.sidebar
}

func TestAppViewContainsBreadcrumb(t *testing.T) {
	// App.View() should include a footer with breadcrumb text
	a := App{
		width:  80,
		height: 24,
		screen: screenCategory,
	}
	view := a.View()
	// The default breadcrumb for screenCategory is "nesco"
	if !strings.Contains(view, "nesco") {
		t.Error("App.View() should contain 'nesco' breadcrumb in the footer")
	}
}

func TestTabTogglesFocus(t *testing.T) {
	// Tab key should toggle focus from sidebar to content
	a := App{
		width:  80,
		height: 24,
		screen: screenItems,
		focus:  focusSidebar,
	}
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	result, _ := a.Update(tabMsg)
	updated := result.(App)
	if updated.focus != focusContent {
		t.Errorf("Tab should move focus from sidebar to content, got focus=%d", updated.focus)
	}
}

func TestEscFromItemsGoesToCategory(t *testing.T) {
	// Esc from screenItems should go back to category/welcome screen
	a := App{
		width:  80,
		height: 24,
		screen: screenItems,
		focus:  focusContent,
	}
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	result, _ := a.Update(escMsg)
	updated := result.(App)
	if updated.focus != focusSidebar {
		t.Errorf("Esc from screenItems should set focus=focusSidebar, got %d", updated.focus)
	}
	if updated.screen != screenCategory {
		t.Errorf("Esc from screenItems should go to screenCategory, got %d", updated.screen)
	}
}
