package tui

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// testModel wraps App to suppress Init() — prevents git fetch during tests.
type testModel struct {
	App
}

func (m testModel) Init() tea.Cmd {
	return nil
}

func (m testModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mdl, cmd := m.App.Update(msg)
	return testModel{mdl.(App)}, cmd
}

func (m testModel) View() string {
	return m.App.View()
}

// newTestModel creates a teatest test model from a test app.
func newTestModel(t *testing.T) *teatest.TestModel {
	t.Helper()
	app := testApp(t)
	return teatest.NewTestModel(t, testModel{app},
		teatest.WithInitialTermSize(80, 45),
	)
}

// waitFor is a helper that waits for output to contain a substring.
func waitFor(t *testing.T, tm *teatest.TestModel, substr string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), substr)
	}, teatest.WithDuration(2*time.Second))
}

// ---------------------------------------------------------------------------
// Integration Tests
// ---------------------------------------------------------------------------

func TestTeatestCategoryToItems(t *testing.T) {
	tm := newTestModel(t)

	// Category screen should render on start
	waitFor(t, tm, "syllago")

	// Enter on first item (Skills) → items screen
	tm.Send(keyEnter)
	waitFor(t, tm, "alpha-skill")

	// Single Esc: back to category (renders category welcome content)
	tm.Send(keyEsc)
	waitFor(t, tm, "Reusable skill definitions")

	// Quit
	tm.Send(keyRune('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestTeatestSearchFlow(t *testing.T) {
	tm := newTestModel(t)
	waitFor(t, tm, "syllago")

	// Activate search with /
	tm.Send(keyRune('/'))

	// Type a query
	tm.Type("alpha")

	// Enter submits search → items screen with SearchResults
	tm.Send(keyEnter)

	// Should see the matching item
	waitFor(t, tm, "alpha")

	// Esc back
	tm.Send(keyEsc)
	waitFor(t, tm, "syllago")

	tm.Send(keyRune('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestTeatestDetailTabs(t *testing.T) {
	tm := newTestModel(t)
	waitFor(t, tm, "syllago")

	// Navigate to Skills → enter → items
	tm.Send(keyEnter)
	waitFor(t, tm, "alpha-skill")

	// Enter first item → detail screen
	tm.Send(keyEnter)

	// Should see overview tab content (README)
	waitFor(t, tm, "Readme body")

	// Tab to files
	tm.Send(keyRune('2'))
	waitFor(t, tm, "SKILL.md")

	// Tab to install
	tm.Send(keyRune('3'))
	waitFor(t, tm, "Install")

	// Back to category
	tm.Send(keyEsc) // → items
	tm.Send(keyEsc) // → category
	waitFor(t, tm, "syllago")

	tm.Send(keyRune('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestTeatestSettingsToggle(t *testing.T) {
	tm := newTestModel(t)
	waitFor(t, tm, "syllago")

	// Navigate to Settings (last row)
	nTypes := sidebarContentCount()
	for i := 0; i < nTypes+5; i++ {
		tm.Send(keyDown)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)

	tm.Send(keyEnter)
	waitFor(t, tm, "Settings")

	// Toggle auto-update (cursor at 0)
	tm.Send(keyEnter)

	// Verify settings screen is still rendered
	waitFor(t, tm, "Auto-update")

	tm.Send(keyEsc)
	waitFor(t, tm, "syllago")

	tm.Send(keyRune('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestTeatestImportStart(t *testing.T) {
	tm := newTestModel(t)
	waitFor(t, tm, "syllago")

	// Navigate to Add
	nTypes := sidebarContentCount()
	for i := 0; i < nTypes+3; i++ {
		tm.Send(keyDown)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)

	tm.Send(keyEnter)

	// Should see import breadcrumb
	waitFor(t, tm, "From Provider")

	tm.Send(keyEsc)
	waitFor(t, tm, "syllago")

	tm.Send(keyRune('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestTeatestQuit(t *testing.T) {
	tm := newTestModel(t)
	waitFor(t, tm, "syllago")

	tm.Send(keyRune('q'))

	out := tm.FinalOutput(t, teatest.WithFinalTimeout(2*time.Second))
	data, _ := io.ReadAll(out)
	// Program should have exited — any output is fine, we just verify it exits
	_ = data
}

func TestTeatestCtrlCAnywhere(t *testing.T) {
	tm := newTestModel(t)
	waitFor(t, tm, "syllago")

	// Navigate deep: category → items → detail
	tm.Send(keyEnter)
	waitFor(t, tm, "alpha-skill")

	tm.Send(keyEnter)
	waitFor(t, tm, "Readme body")

	// Ctrl+C from detail should quit
	tm.Send(keyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestTeatestWindowResize(t *testing.T) {
	tm := newTestModel(t)
	waitFor(t, tm, "syllago")

	// Resize to below minimum (40x10)
	tm.Send(tea.WindowSizeMsg{Width: 30, Height: 8})
	waitFor(t, tm, "Terminal too small")

	// Resize back to normal
	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 30})

	// Should recover and show normal UI
	waitFor(t, tm, "syllago")

	tm.Send(keyRune('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
