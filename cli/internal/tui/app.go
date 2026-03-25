package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
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
	library  libraryModel  // Library tab: table + drill-in
	explorer explorerModel // Content/Loadout tabs: items list + preview
	helpBar  helpBarModel
	modal    textInputModal // reusable text input overlay

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
		library:         newLibraryModel(cat.Items, providers, projectRoot),
		explorer:        newExplorerModel(nil, false),
		helpBar:         newHelpBar(version),
		modal:           newTextInputModal(),
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
		ch := a.contentHeight()
		a.library.SetSize(msg.Width, ch)
		a.explorer.SetSize(msg.Width, ch)
		return a, nil

	case tea.MouseMsg:
		// Modal captures all mouse input when active
		if a.modal.active {
			var cmd tea.Cmd
			a.modal, cmd = a.modal.Update(msg)
			return a, cmd
		}
		return a.routeMouse(msg)

	case tea.KeyMsg:
		// Modal captures all key input when active (except ctrl+c)
		if a.modal.active {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.modal, cmd = a.modal.Update(msg)
			return a, cmd
		}

		// When library search is active, only handle ctrl+c — everything
		// else goes to the search input so letters like 'a', 'q', '1' etc.
		// are typed into the query rather than triggering shortcuts.
		if a.isLibraryTab() && a.library.table.searching {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			return a.routeKey(msg)
		}

		// Global keys always handled first
		switch {
		case msg.Type == tea.KeyCtrlC:
			return a, tea.Quit
		case msg.String() == keyQuit:
			return a, tea.Quit

		// 1/2/3 switch groups
		case msg.String() == keyGroup1:
			cmd := a.topBar.SetGroup(0)
			a.refreshContent()
			a.helpBar.SetHints(a.currentHints())
			return a, cmd
		case msg.String() == keyGroup2:
			cmd := a.topBar.SetGroup(1)
			a.refreshContent()
			a.helpBar.SetHints(a.currentHints())
			return a, cmd
		case msg.String() == keyGroup3:
			cmd := a.topBar.SetGroup(2)
			a.refreshContent()
			a.helpBar.SetHints(a.currentHints())
			return a, cmd

		// Tab cycles sub-tabs within active group
		case msg.Type == tea.KeyTab:
			cmd := a.topBar.NextTab()
			a.refreshContent()
			return a, cmd
		case msg.Type == tea.KeyShiftTab:
			cmd := a.topBar.PrevTab()
			a.refreshContent()
			return a, cmd

		// Action button hotkeys
		case msg.String() == keyAdd:
			return a, a.topBar.actionCmd("add")
		case msg.String() == keyCreate:
			return a, a.topBar.actionCmd("create")

		// Rename selected item
		case msg.String() == keyRename:
			return a.handleRename()

		// Route to active content model
		default:
			return a.routeKey(msg)
		}

	case modalSavedMsg:
		return a.handleModalSaved(msg)

	case modalCancelledMsg:
		return a, nil

	case tabChangedMsg:
		a.refreshContent()
		a.helpBar.SetHints(a.currentHints())
		return a, nil

	case libraryDrillMsg:
		a.helpBar.SetHints(a.currentHints())
		return a, nil

	case libraryCloseMsg:
		a.helpBar.SetHints(a.currentHints())
		return a, nil

	case itemSelectedMsg:
		return a, nil

	case actionPressedMsg:
		return a, nil
	}
	return a, nil
}

// handleRename opens the rename modal for the currently selected item.
func (a App) handleRename() (tea.Model, tea.Cmd) {
	var item *catalog.ContentItem
	if a.isLibraryTab() {
		item = a.library.table.Selected()
	} else {
		item = a.explorer.items.Selected()
	}
	if item == nil {
		return a, nil
	}
	currentName := itemDisplayName(*item)
	a.modal.Open("Rename: "+item.Name, currentName, item.Path)
	return a, nil
}

// handleModalSaved persists the rename to .syllago.yaml and updates in-place.
func (a App) handleModalSaved(msg modalSavedMsg) (tea.Model, tea.Cmd) {
	if msg.value == "" || msg.context == "" {
		return a, nil
	}

	// Load or create metadata
	meta, err := metadata.Load(msg.context)
	if err != nil {
		return a, nil
	}
	if meta == nil {
		meta = &metadata.Meta{}
	}
	meta.Name = msg.value
	if err := metadata.Save(msg.context, meta); err != nil {
		return a, nil
	}

	// Update DisplayName in-place in the catalog (avoid full re-scan)
	for i := range a.catalog.Items {
		if a.catalog.Items[i].Path == msg.context {
			a.catalog.Items[i].DisplayName = msg.value
			break
		}
	}
	a.refreshContent()
	return a, nil
}

// routeKey sends key messages to the active content model.
func (a App) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.isLibraryTab() {
		var cmd tea.Cmd
		a.library, cmd = a.library.Update(msg)
		return a, cmd
	}
	var cmd tea.Cmd
	a.explorer, cmd = a.explorer.Update(msg)
	return a, cmd
}

// routeMouse sends mouse messages to topbar + active content model.
func (a App) routeMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	var topCmd tea.Cmd
	a.topBar, topCmd = a.topBar.Update(msg)

	var contentCmd tea.Cmd
	if a.isLibraryTab() {
		a.library, contentCmd = a.library.Update(msg)
	} else {
		a.explorer, contentCmd = a.explorer.Update(msg)
	}
	return a, tea.Batch(topCmd, contentCmd)
}

// View implements tea.Model.
func (a App) View() string {
	if !a.ready {
		return ""
	}
	if a.width < 80 || a.height < 20 {
		return a.renderTooSmall()
	}

	topBar := a.topBar.View()
	content := a.renderContent()
	helpBar := a.helpBar.View()

	// When modal is active, replace content area with centered modal
	if a.modal.active {
		ch := a.contentHeight()
		modalView := a.modal.View()
		content = lipgloss.Place(a.width, ch, lipgloss.Center, lipgloss.Center, modalView)
	}

	return zone.Scan(lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		content,
		helpBar,
	))
}

// contentHeight returns the available height for the main content area.
func (a App) contentHeight() int {
	topBarHeight := a.topBar.Height()
	helpBarHeight := a.helpBar.Height()
	return a.height - topBarHeight - helpBarHeight
}

// renderContent renders the main content area based on the active tab.
func (a App) renderContent() string {
	group := a.topBar.ActiveGroupLabel()
	tab := a.topBar.ActiveTabLabel()

	if group == "Config" {
		return a.renderPlaceholder("Settings view coming soon")
	}

	if group == "Collections" && tab == "Registries" {
		return a.renderPlaceholder("Registries view coming soon")
	}

	if a.isLibraryTab() {
		return a.library.View()
	}

	return a.explorer.View()
}

// isLibraryTab returns true if the active tab is Collections > Library.
func (a App) isLibraryTab() bool {
	return a.topBar.ActiveGroupLabel() == "Collections" && a.topBar.ActiveTabLabel() == "Library"
}

// refreshContent updates the active content model based on the current tab.
func (a *App) refreshContent() {
	if a.isLibraryTab() {
		a.library.SetItems(a.catalog.Items)
		a.library.SetSize(a.width, a.contentHeight())
	} else {
		items, mixed := a.itemsForCurrentTab()
		a.explorer.SetItems(items, mixed)
		a.explorer.SetSize(a.width, a.contentHeight())
	}
}

// itemsForCurrentTab returns the catalog items for the active tab.
func (a App) itemsForCurrentTab() ([]catalog.ContentItem, bool) {
	group := a.topBar.ActiveGroupLabel()
	tab := a.topBar.ActiveTabLabel()

	switch group {
	case "Collections":
		switch tab {
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

	base := []string{"1/2/3 groups", "tab items"}

	if group == "Config" {
		return append(base, "? help", "q quit")
	}

	if group == "Collections" && tab == "Registries" {
		return append(base, "a add", "? help", "q quit")
	}

	// Library in detail mode has different hints
	if a.isLibraryTab() && a.library.mode == libraryDetail {
		return append(base, "↑/↓ navigate", "←/→ switch pane", "esc close", "? help", "q quit")
	}

	if a.isLibraryTab() {
		return append(base, "↑/↓ navigate", "enter preview", "/ search", "s sort", "r rename", "a add", "n create", "? help", "q quit")
	}

	hints := append(base, "↑/↓ navigate", "←/→ switch pane")
	if group != "Config" {
		hints = append(hints, "r rename", "a add", "n create")
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
		warningStyle.Render("Terminal too small\nMinimum: 80x20"),
	)
}
