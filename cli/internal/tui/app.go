package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
		a.helpBar.SetSize(msg.Width)
		return a, nil

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlC:
			return a, tea.Quit
		case msg.String() == "q":
			return a, tea.Quit
		}
		// Phase 2+: dropdown keys, focus routing
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

	topBar := a.renderTopBarPlaceholder()
	content := a.renderEmptyContent()
	helpBar := a.helpBar.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		content,
		helpBar,
	)
}

// contentHeight returns the available height for the main content area.
func (a App) contentHeight() int {
	topBarHeight := 1 // Phase 2: will come from topBar.height()
	helpBarHeight := 1
	return a.height - topBarHeight - helpBarHeight
}

// renderTopBarPlaceholder renders a placeholder header until Phase 2.
func (a App) renderTopBarPlaceholder() string {
	logo := logoStyle.Render("syl") + accentLogoStyle.Render("lago")
	right := mutedStyle.Render("Phase 2: navigation dropdowns")
	gap := strings.Repeat(" ", max(0, a.width-lipgloss.Width(logo)-lipgloss.Width(right)))
	return logo + gap + right
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
