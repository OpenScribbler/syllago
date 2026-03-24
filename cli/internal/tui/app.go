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
	topBar   topBarModel
	explorer explorerModel
	helpBar  helpBarModel

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
	if cat == nil {
		cat = &catalog.Catalog{}
	}

	// Default view: Collections > Library = all items
	items := cat.Items
	a := App{
		catalog:         cat,
		providers:       providers,
		version:         version,
		autoUpdate:      autoUpdate,
		isReleaseBuild:  isReleaseBuild,
		registrySources: registrySources,
		cfg:             cfg,
		projectRoot:     projectRoot,
		topBar:          newTopBar(),
		explorer:        newExplorerModel(items, true), // Library = mixed types
		helpBar:         newHelpBar(version),
	}
	a.helpBar.SetHints(a.currentHints())
	return a
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
		a.explorer.SetSize(msg.Width, a.contentHeight())
		return a, nil

	case tea.MouseMsg:
		var cmd tea.Cmd
		a.topBar, cmd = a.topBar.Update(msg)
		return a, cmd

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlC:
			return a, tea.Quit
		case msg.String() == keyQuit:
			return a, tea.Quit

		// 1/2/3 switch groups
		case msg.String() == keyGroup1:
			cmd := a.topBar.SetGroup(0)
			a.refreshExplorer()
			a.helpBar.SetHints(a.currentHints())
			return a, cmd
		case msg.String() == keyGroup2:
			cmd := a.topBar.SetGroup(1)
			a.refreshExplorer()
			a.helpBar.SetHints(a.currentHints())
			return a, cmd
		case msg.String() == keyGroup3:
			cmd := a.topBar.SetGroup(2)
			a.refreshExplorer()
			a.helpBar.SetHints(a.currentHints())
			return a, cmd

		// h/l switch sub-tabs within active group
		case msg.String() == keyRight, msg.String() == "right":
			cmd := a.topBar.NextTab()
			a.refreshExplorer()
			return a, cmd
		case msg.String() == keyLeft, msg.String() == "left":
			cmd := a.topBar.PrevTab()
			a.refreshExplorer()
			return a, cmd

		// Action button hotkeys
		case msg.String() == keyAdd:
			return a, a.topBar.actionCmd("add")
		case msg.String() == keyCreate:
			return a, a.topBar.actionCmd("create")

		// Everything else routes to the explorer
		default:
			var cmd tea.Cmd
			a.explorer, cmd = a.explorer.Update(msg)
			return a, cmd
		}

	case tabChangedMsg:
		a.refreshExplorer()
		a.helpBar.SetHints(a.currentHints())
		return a, nil

	case itemSelectedMsg:
		// Preview updates happen inside explorer; nothing to do at app level yet
		return a, nil

	case actionPressedMsg:
		// Phase 4+: open add/create wizards based on msg.group and msg.tab
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
	content := a.renderContent()
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

// renderContent renders the main content area based on the active tab.
func (a App) renderContent() string {
	group := a.topBar.ActiveGroupLabel()
	tab := a.topBar.ActiveTabLabel()

	// Config tabs don't have explorer content
	if group == "Config" {
		return a.renderPlaceholder("Settings view coming soon")
	}

	// Registries needs gallery grid (Phase 6)
	if group == "Collections" && tab == "Registries" {
		return a.renderPlaceholder("Registries view coming soon")
	}

	return a.explorer.View()
}

// refreshExplorer updates the explorer items based on the current tab.
func (a *App) refreshExplorer() {
	items, mixed := a.itemsForCurrentTab()
	a.explorer.SetItems(items, mixed)
	a.explorer.SetSize(a.width, a.contentHeight())
}

// itemsForCurrentTab returns the catalog items for the active tab.
func (a App) itemsForCurrentTab() ([]catalog.ContentItem, bool) {
	group := a.topBar.ActiveGroupLabel()
	tab := a.topBar.ActiveTabLabel()

	switch group {
	case "Collections":
		switch tab {
		case "Library":
			return a.catalog.Items, true // all items, mixed types
		case "Loadouts":
			return a.catalog.ByType(catalog.Loadouts), false
		default:
			return nil, false
		}
	case "Content":
		ct := tabToContentType(tab)
		if ct != "" {
			return a.catalog.ByType(ct), false
		}
	}
	return nil, false
}

// tabToContentType maps a sub-tab label to its content type.
func tabToContentType(tab string) catalog.ContentType {
	switch tab {
	case "Skills":
		return catalog.Skills
	case "Agents":
		return catalog.Agents
	case "MCP":
		return catalog.MCP
	case "Rules":
		return catalog.Rules
	case "Hooks":
		return catalog.Hooks
	case "Commands":
		return catalog.Commands
	}
	return ""
}

// currentHints returns context-sensitive help hints based on current state.
func (a App) currentHints() []string {
	group := a.topBar.ActiveGroupLabel()
	tab := a.topBar.ActiveTabLabel()

	base := []string{"1/2/3 groups", "h/l tabs"}

	if group == "Config" {
		return append(base, "? help", "q quit")
	}

	if group == "Collections" && tab == "Registries" {
		return append(base, "a add", "? help", "q quit")
	}

	hints := append(base, "j/k navigate", "tab focus")
	if group != "Config" {
		hints = append(hints, "a add", "n create")
	}
	return append(hints, "? help", "q quit")
}

// renderPlaceholder renders a centered message for tabs without explorer content.
func (a App) renderPlaceholder(msg string) string {
	h := a.contentHeight()
	return lipgloss.NewStyle().
		Width(a.width).
		Height(h).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(mutedColor).
		Render(msg)
}

// renderTooSmall renders a warning when the terminal is below minimum size.
func (a App) renderTooSmall() string {
	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		warningStyle.Render("Terminal too small\nMinimum: 60x20"),
	)
}
