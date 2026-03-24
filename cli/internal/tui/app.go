package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// App is the root bubbletea model for the TUI.
type App struct {
	// Backend data (passed from main.go, used by future phases)
	catalog         *catalog.Catalog
	providers       []provider.Provider
	version         string
	autoUpdate      bool
	isReleaseBuild  bool
	registrySources []catalog.RegistrySource
	cfg             *config.Config
	projectRoot     string

	// Sub-models
	topBar  topBarModel
	helpBar helpBarModel

	// Dimensions
	width, height int

	// State
	ready bool // false until first WindowSizeMsg
}

// NewApp creates a new TUI app. Signature matches main.go.
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string, autoUpdate bool, registrySources []catalog.RegistrySource, cfg *config.Config, isReleaseBuild bool, projectRoot string) App {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return App{
		catalog:         cat,
		providers:       providers,
		version:         version,
		autoUpdate:      autoUpdate,
		isReleaseBuild:  isReleaseBuild,
		registrySources: registrySources,
		cfg:             cfg,
		projectRoot:     projectRoot,
		topBar:          newTopBar(),
		helpBar:         newHelpBar(version),
	}
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.topBar.SetSize(msg.Width)
		a.helpBar.SetSize(msg.Width)
		return a, nil

	case tea.MouseMsg:
		var cmd tea.Cmd
		a.topBar, cmd = a.topBar.Update(msg)
		return a, cmd

	case tea.KeyMsg:
		// Open dropdown captures ALL keys (modal pattern).
		if a.topBar.HasOpenDropdown() {
			// Global escape hatch
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.topBar, cmd = a.topBar.Update(msg)
			return a, cmd
		}

		switch {
		case msg.Type == tea.KeyCtrlC:
			return a, tea.Quit
		case msg.String() == "q":
			return a, tea.Quit

		// 1/2/3 open dropdowns from anywhere
		case msg.String() == "1":
			a.topBar.OpenDropdown(1)
			return a, nil
		case msg.String() == "2":
			a.topBar.OpenDropdown(2)
			return a, nil
		case msg.String() == "3":
			a.topBar.OpenDropdown(3)
			return a, nil
		}
		// Phase 3+: focus-based routing to explorer/gallery

	case dropdownActiveMsg:
		a.topBar.HandleActiveMsg(msg)
		a.helpBar.SetHints(a.currentHints())
		return a, nil
	}
	return a, nil
}

// View implements tea.Model.
func (a App) View() string {
	if !a.ready {
		return ""
	}
	if a.width < 60 || a.height < 20 {
		return a.renderTooSmall()
	}

	topBar := a.topBar.View()
	content := a.renderEmptyContent()
	helpBar := a.helpBar.View()

	return zone.Scan(lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		content,
		helpBar,
	))
}

// contentHeight returns the available height for the main content area.
func (a App) contentHeight() int {
	topBarHeight := a.topBar.Height()
	helpBarHeight := 1
	return a.height - topBarHeight - helpBarHeight
}

// currentHints returns context-sensitive help hints based on current state.
func (a App) currentHints() []string {
	if a.topBar.HasOpenDropdown() {
		return []string{"j/k navigate", "enter select", "esc close"}
	}
	return []string{"1/2/3 dropdowns", "? help", "q quit"}
}

// renderEmptyContent renders the empty main content area.
func (a App) renderEmptyContent() string {
	h := a.contentHeight()
	return lipgloss.NewStyle().
		Width(a.width).
		Height(h).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(mutedColor).
		Render("No content loaded.\n\nPhase 3 adds the explorer layout.")
}

// renderTooSmall renders a warning when the terminal is below minimum size.
func (a App) renderTooSmall() string {
	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		warningStyle.Render("Terminal too small\nMinimum: 60x20"),
	)
}
