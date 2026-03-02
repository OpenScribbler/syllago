package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

const (
	stepProviders = 0
	stepRegistry  = 1
	stepInput     = 2 // text input sub-step for URL or registry name
)

const (
	registryOptAdd    = 0
	registryOptCreate = 1
	registryOptSkip   = 2
)

// initWizard is a bubbletea model for interactive init provider selection.
// It shows all known providers with detected ones pre-checked, and lets
// the user toggle selections before confirming. After provider selection,
// it offers registry setup options.
type initWizard struct {
	providers []provider.Provider
	checks    []bool
	cursor    int
	done      bool
	cancelled bool

	step           int
	registryCursor int

	// Text input for URL or registry name entry
	textInput textinput.Model
	inputMode int // registryOptAdd or registryOptCreate

	// Registry result
	registryAction string // "add", "create", or "skip"
	registryURL    string
	registryName   string
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
		switch w.step {
		case stepProviders:
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
				// Advance to registry step instead of finishing
				w.step = stepRegistry
				w.registryCursor = 0
			case tea.KeyEsc, tea.KeyCtrlC:
				w.cancelled = true
				w.done = true
			}

		case stepRegistry:
			switch msg.Type {
			case tea.KeyUp:
				if w.registryCursor > 0 {
					w.registryCursor--
				}
			case tea.KeyDown:
				if w.registryCursor < 2 {
					w.registryCursor++
				}
			case tea.KeyEnter:
				switch w.registryCursor {
				case registryOptAdd:
					ti := textinput.New()
					ti.Placeholder = "https://github.com/you/your-registry.git"
					ti.CharLimit = 500
					ti.Width = 50
					ti.Focus()
					w.textInput = ti
					w.inputMode = registryOptAdd
					w.step = stepInput
				case registryOptCreate:
					ti := textinput.New()
					ti.Placeholder = "my-registry"
					ti.CharLimit = 100
					ti.Width = 50
					ti.Focus()
					w.textInput = ti
					w.inputMode = registryOptCreate
					w.step = stepInput
				case registryOptSkip:
					w.registryAction = "skip"
					w.done = true
				}
			case tea.KeyEsc, tea.KeyCtrlC:
				w.cancelled = true
				w.done = true
			}

		case stepInput:
			switch msg.Type {
			case tea.KeyEnter:
				val := strings.TrimSpace(w.textInput.Value())
				if val == "" {
					// Ignore empty submissions
					break
				}
				if w.inputMode == registryOptAdd {
					w.registryAction = "add"
					w.registryURL = val
				} else {
					w.registryAction = "create"
					w.registryName = val
				}
				w.done = true
			case tea.KeyEsc, tea.KeyCtrlC:
				w.cancelled = true
				w.done = true
			default:
				var cmd tea.Cmd
				w.textInput, cmd = w.textInput.Update(msg)
				return w, cmd
			}
		}
	}
	return w, nil
}

func (w initWizard) View() string {
	primary := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#047857"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#57534E"))
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6D28D9"))

	var sb strings.Builder

	switch w.step {
	case stepProviders:
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

	case stepRegistry:
		sb.WriteString(primary.Render("Set up a registry?") + "\n")
		sb.WriteString(muted.Render("Registries are git repos with shared AI content.") + "\n\n")

		options := []string{"Add a registry URL", "Create a new registry", "Skip for now"}
		for i, opt := range options {
			prefix := "  "
			nameStyle := lipgloss.NewStyle()
			if i == w.registryCursor {
				prefix = "> "
				nameStyle = selected
			}
			sb.WriteString(fmt.Sprintf("  %s%s\n", prefix, nameStyle.Render(opt)))
		}

		sb.WriteString("\n" + muted.Render("[up/down] navigate   [enter] select   [esc] cancel"))

	case stepInput:
		var prompt string
		if w.inputMode == registryOptAdd {
			prompt = "Enter the registry URL:"
		} else {
			prompt = "Enter a name for the new registry:"
		}
		sb.WriteString(primary.Render(prompt) + "\n\n")
		sb.WriteString("  " + w.textInput.View() + "\n")
		sb.WriteString("\n" + muted.Render("[enter] confirm   [esc] cancel"))
	}

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
