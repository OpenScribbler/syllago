package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// initWizard is a bubbletea model for interactive init provider selection.
// It shows all known providers with detected ones pre-checked, and lets
// the user toggle selections before confirming.
type initWizard struct {
	providers []provider.Provider
	checks    []bool
	cursor    int
	done      bool
	cancelled bool
}

// newInitWizard creates a wizard with all providers listed and detected ones pre-checked.
// "detected" is used to determine which providers start checked; "all" is the full
// list displayed to the user (with Detected field set for tag rendering).
func newInitWizard(detected, all []provider.Provider) initWizard {
	checks := make([]bool, len(all))
	detectedSlugs := map[string]bool{}
	for _, p := range detected {
		detectedSlugs[p.Slug] = true
	}
	for i, p := range all {
		checks[i] = detectedSlugs[p.Slug]
	}
	return initWizard{
		providers: all,
		checks:    checks,
	}
}

func (w initWizard) isChecked(i int) bool {
	if i < 0 || i >= len(w.checks) {
		return false
	}
	return w.checks[i]
}

func (w initWizard) selectedSlugs() []string {
	var slugs []string
	for i, p := range w.providers {
		if i < len(w.checks) && w.checks[i] {
			slugs = append(slugs, p.Slug)
		}
	}
	return slugs
}

func (w initWizard) Init() tea.Cmd { return nil }

func (w initWizard) Update(msg tea.Msg) (initWizard, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if w.cursor > 0 {
				w.cursor--
			}
		case tea.KeyDown:
			if w.cursor < len(w.providers)-1 {
				w.cursor++
			}
		case tea.KeySpace:
			if w.cursor < len(w.checks) {
				w.checks[w.cursor] = !w.checks[w.cursor]
			}
		case tea.KeyEnter:
			w.done = true
		case tea.KeyEsc, tea.KeyCtrlC:
			w.cancelled = true
			w.done = true
		}
	}
	return w, nil
}

func (w initWizard) View() string {
	primary := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#047857"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#57534E"))
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6D28D9"))

	var sb strings.Builder
	sb.WriteString(primary.Render("Which tools do you want syllago to manage?") + "\n")
	sb.WriteString(muted.Render("Use space to toggle, enter to confirm.") + "\n\n")

	for i, p := range w.providers {
		check := "[ ]"
		if i < len(w.checks) && w.checks[i] {
			check = "[x]"
		}
		prefix := "  "
		nameStyle := lipgloss.NewStyle()
		if i == w.cursor {
			prefix = "> "
			nameStyle = selected
		}

		var tag string
		if p.Detected {
			tag = muted.Render(" (detected)")
		} else {
			tag = muted.Render(" (not found)")
		}

		sb.WriteString(fmt.Sprintf("  %s%s %s%s\n", prefix, check, nameStyle.Render(p.Name), tag))
	}

	sb.WriteString("\n" + muted.Render("[up/down] navigate   [space] toggle   [enter] confirm   [esc] cancel"))
	return sb.String()
}

// initWizardModel wraps initWizard to implement tea.Model.
// The inner initWizard.Update returns (initWizard, tea.Cmd) for testability,
// but tea.NewProgram requires (tea.Model, tea.Cmd). This wrapper bridges that gap.
type initWizardModel struct {
	wizard initWizard
}

func (m initWizardModel) Init() tea.Cmd { return nil }

func (m initWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	w, cmd := m.wizard.Update(msg)
	m.wizard = w
	if w.done {
		return m, tea.Quit
	}
	return m, cmd
}

func (m initWizardModel) View() string {
	return m.wizard.View()
}
