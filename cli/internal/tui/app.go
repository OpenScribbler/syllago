package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// viewMode determines the main layout pattern.
type viewMode int

const (
	viewExplorer viewMode = iota // items list + content zone
	viewGallery                  // card grid + contents sidebar
)

// dropdownID identifies which dropdown is active.
type dropdownID int

const (
	dropdownContent    dropdownID = iota // Content types (Skills, Agents, etc.)
	dropdownCollection                   // Collections (Library, Registries, Loadouts)
	dropdownConfig                       // Config (Settings, Sandbox)
)

// focusArea tracks which area of the UI has keyboard focus.
type focusArea int

const (
	focusItemsList   focusArea = iota // items list (Explorer left pane)
	focusContentZone                  // content zone (Explorer right pane)
	focusCardGrid                     // card grid (Gallery left)
	focusSidebar                      // contents sidebar (Gallery right)
	focusDropdown                     // an open dropdown menu
	focusModal                        // a modal overlay
	focusSearch                       // search bar active
)

// App is the root bubbletea model for the TUI.
type App struct {
	// Backend data
	catalog         *catalog.Catalog
	providers       []provider.Provider
	version         string
	autoUpdate      bool
	isReleaseBuild  bool
	registrySources []catalog.RegistrySource
	cfg             *config.Config
	projectRoot     string

	// Layout state
	width  int
	height int
	mode   viewMode
	focus  focusArea

	// Navigation state
	activeDropdown dropdownID
	contentType    catalog.ContentType
	collectionType catalog.ContentType

	// Sub-models
	topBar   topBarModel
	metadata metadataModel
	explorer explorerModel
	gallery  galleryModel
	modal    modalModel
	toast    toastModel
	search   searchModel
	helpBar  helpBarModel

	ready bool
}

// NewApp creates a new TUI app with the same signature as v1.
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string, autoUpdate bool, registrySources []catalog.RegistrySource, cfg *config.Config, isReleaseBuild bool, projectRoot string) App {
	if cfg == nil {
		cfg = &config.Config{}
	}

	ct := catalog.Skills
	items := cat.ByType(ct)

	a := App{
		catalog:         cat,
		providers:       providers,
		version:         version,
		autoUpdate:      autoUpdate,
		isReleaseBuild:  isReleaseBuild,
		registrySources: registrySources,
		cfg:             cfg,
		projectRoot:     projectRoot,
		mode:            viewExplorer,
		focus:           focusItemsList,
		activeDropdown:  dropdownContent,
		contentType:     ct,
		topBar:          newTopBarModel(),
		search:          newSearchModel(),
		helpBar:         helpBarModel{version: version},
	}

	// Initialize metadata with type summary
	a.metadata = metadataModel{
		ct:    ct,
		items: items,
		width: 80, // will be updated on WindowSizeMsg
	}

	// Initialize explorer with initial items
	a.explorer = newExplorerModel(items, ct, 80, 20)

	return a
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.helpBar.width = msg.Width
		a.metadata.width = msg.Width
		a.toast.width = msg.Width
		mainH := a.mainAreaHeight()
		a.explorer.width = msg.Width
		a.explorer.height = mainH
		a.explorer.items.width = a.itemsWidth()
		a.explorer.items.height = mainH
		a.gallery.width = msg.Width
		a.gallery.height = mainH
		a.ready = true
		return a, nil

	case toastDismissMsg:
		a.toast, _ = a.toast.Update(msg)
		return a, nil

	case topBarSelectMsg:
		return a.handleTopBarSelect(msg), nil

	case itemSelectedMsg:
		a.updateMetadataForItem(&msg.item)
		return a, nil

	case modalCloseMsg:
		a.modal.active = false
		a.focus = focusItemsList
		return a, nil

	case modalConfirmMsg:
		a.modal.active = false
		a.focus = focusItemsList
		return a, nil

	case searchQueryMsg:
		filtered := filterItems(a.catalog.ByType(a.contentType), msg.query)
		a.explorer.items = newItemsModel(filtered, a.contentType)
		a.explorer.items.width = a.itemsWidth()
		a.explorer.items.height = a.mainAreaHeight()
		return a, nil

	case searchCancelMsg:
		items := a.catalog.ByType(a.contentType)
		a.explorer.items = newItemsModel(items, a.contentType)
		a.explorer.items.width = a.itemsWidth()
		a.explorer.items.height = a.mainAreaHeight()
		a.focus = focusItemsList
		return a, nil

	case searchConfirmMsg:
		a.focus = focusItemsList
		return a, nil

	case tea.KeyMsg:
		// Modal takes priority
		if a.modal.active {
			var cmd tea.Cmd
			a.modal, cmd = a.modal.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}

		// Toast handling
		if a.toast.active {
			var cmd tea.Cmd
			a.toast, cmd = a.toast.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			// Success/warning toasts dismiss and pass key through
			// Error toasts consume the key
			if a.toast.toastType == toastError {
				return a, tea.Batch(cmds...)
			}
		}

		// Search handling
		if a.search.active {
			cmd := a.search.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}

		// Top bar dropdown handling
		if a.topBar.menuOpen {
			var cmd tea.Cmd
			a.topBar, cmd = a.topBar.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}

		// Global keys
		switch {
		case msg.Type == tea.KeyCtrlC:
			return a, tea.Quit

		case key.Matches(msg, keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, keys.Search):
			a.search.activate()
			a.focus = focusSearch
			return a, nil

		case key.Matches(msg, keys.Dropdown1), key.Matches(msg, keys.Dropdown2), key.Matches(msg, keys.Dropdown3):
			var cmd tea.Cmd
			a.topBar, cmd = a.topBar.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}

		// Route to active layout
		if a.mode == viewExplorer {
			return a.updateExplorer(msg)
		}
		return a.updateGallery(msg)
	}

	return a, tea.Batch(cmds...)
}

// updateExplorer handles key routing for the Explorer layout.
func (a App) updateExplorer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, keys.Left):
		a.focus = focusItemsList
	case key.Matches(msg, keys.Right):
		a.focus = focusContentZone
	}

	// Route to explorer's own Update which handles focus/preview loading
	var cmd tea.Cmd
	a.explorer, cmd = a.explorer.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	// Update metadata when item changes
	item := a.explorer.items.selectedItem()
	if item != nil {
		a.updateMetadataForItem(item)
	}

	return a, tea.Batch(cmds...)
}

// updateGallery handles key routing for the Gallery layout.
func (a App) updateGallery(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.gallery, cmd = a.gallery.Update(msg)
	return a, cmd
}

// handleTopBarSelect handles dropdown selections from the top bar.
func (a App) handleTopBarSelect(msg topBarSelectMsg) App {
	switch msg.category {
	case dropdownContent:
		ct := contentTypeFromLabel(msg.item)
		a.activeDropdown = dropdownContent
		a.contentType = ct
		a.mode = viewExplorer
		a.focus = focusItemsList
		items := a.catalog.ByType(ct)
		a.explorer = newExplorerModel(items, ct, a.width, a.mainAreaHeight())
		a.metadata = metadataModel{ct: ct, items: items, width: a.width}
	case dropdownCollection:
		a.activeDropdown = dropdownCollection
		a.collectionType = collectionTypeFromLabel(msg.item)
		a.mode = viewGallery
		a.focus = focusCardGrid
		a.buildGalleryForCollection()
	}
	return a
}

// buildGalleryForCollection builds the gallery model for the active collection.
func (a *App) buildGalleryForCollection() {
	var cards []cardData
	ct := a.collectionType

	switch ct {
	case catalog.Loadouts:
		for _, item := range a.catalog.ByType(catalog.Loadouts) {
			cards = append(cards, cardData{
				title: item.Name,
				lines: []string{
					itoa(len(item.Files)) + " items",
					"Source: " + item.Source,
				},
			})
		}
	case catalog.Library:
		// Library shows content type cards
		for _, typeName := range []catalog.ContentType{catalog.Skills, catalog.Agents, catalog.MCP, catalog.Rules, catalog.Hooks, catalog.Commands} {
			items := a.catalog.ByType(typeName)
			if len(items) > 0 {
				cards = append(cards, cardData{
					title: displayTypeName(typeName),
					lines: []string{itoa(len(items)) + " items"},
				})
			}
		}
	default:
		// Registries
		for _, rs := range a.registrySources {
			items := a.catalog.ByType(catalog.Skills) // simplified
			cards = append(cards, cardData{
				title:    rs.Name,
				subtitle: rs.Path,
				lines:    []string{itoa(len(items)) + " items"},
			})
		}
	}

	a.gallery = newGalleryModel(cards, ct, a.width, a.mainAreaHeight())
}

// updateMetadataForItem updates the metadata bar for a specific item.
func (a *App) updateMetadataForItem(item *catalog.ContentItem) {
	a.metadata = metadataModel{
		item:  item,
		ct:    item.Type,
		items: a.catalog.ByType(item.Type),
		width: a.width,
	}
}

// loadPreviewForItem delegates to the explorer's own preview loading.
func (a *App) loadPreviewForItem(item *catalog.ContentItem) {
	a.explorer.loadPreviewForItem(item)
}

// View implements tea.Model.
func (a App) View() string {
	if !a.ready {
		return "Loading..."
	}
	if a.width < 60 || a.height < 20 {
		return "Terminal too small (need 60x20 minimum)"
	}

	var sections []string

	// Top bar
	sections = append(sections, a.topBar.View(a.width))

	// Toast overlay (if active, replaces metadata position)
	if a.toast.active {
		sections = append(sections, a.toast.View())
	}

	// Metadata bar
	sections = append(sections, a.metadata.View())

	// Search bar (if active)
	if a.search.active {
		sections = append(sections, a.search.View())
	}

	// Main area
	mainHeight := a.mainAreaHeight()
	if a.mode == viewExplorer {
		sections = append(sections, a.renderExplorer(mainHeight))
	} else {
		sections = append(sections, a.renderGallery(mainHeight))
	}

	// Help bar
	var hints []helpHint
	if a.mode == viewExplorer {
		hints = explorerHints()
	} else {
		hints = galleryHints()
	}
	sections = append(sections, a.helpBar.View(hints))

	result := strings.Join(sections, "\n")

	// Modal overlay (on top of everything)
	if a.modal.active {
		result = a.renderModalOverlay(result)
	}

	return result
}

// renderExplorer renders the Explorer layout via the explorer model's View.
func (a App) renderExplorer(height int) string {
	a.explorer.width = a.width
	a.explorer.height = height
	return a.explorer.View()
}

// renderGallery renders the Gallery Grid layout.
func (a App) renderGallery(height int) string {
	a.gallery.width = a.width
	a.gallery.height = height
	return a.gallery.View()
}

// renderModalOverlay renders the modal centered on top of the background.
func (a App) renderModalOverlay(background string) string {
	modalView := a.modal.View()
	bgLines := strings.Split(background, "\n")
	modalLines := strings.Split(modalView, "\n")

	// Center the modal
	startY := (len(bgLines) - len(modalLines)) / 2
	startX := (a.width - modalWidth - 2) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	for i, ml := range modalLines {
		row := startY + i
		if row >= len(bgLines) {
			break
		}
		// Overlay modal line onto background
		bgLine := bgLines[row]
		bgRunes := []rune(bgLine)
		mlRunes := []rune(ml)
		// Pad background line if needed
		for len(bgRunes) < startX+len(mlRunes) {
			bgRunes = append(bgRunes, ' ')
		}
		// Replace portion with modal
		copy(bgRunes[startX:], mlRunes)
		bgLines[row] = string(bgRunes)
	}

	return strings.Join(bgLines, "\n")
}

// itemsWidth returns the width allocated to the items list pane.
func (a App) itemsWidth() int {
	if a.width >= 100 {
		return a.width / 4 // 25%
	}
	return a.width * 3 / 10 // 30%
}

// mainAreaHeight calculates the available height for the main content area.
// Shell: top bar (3) + metadata (5) + help bar (2) = 10
func (a App) mainAreaHeight() int {
	h := a.height - 10
	if a.toast.active {
		h -= 3 // toast takes ~3 rows
	}
	if a.search.active {
		h -= 1 // search bar
	}
	if h < 1 {
		h = 1
	}
	return h
}

// contentTypeFromLabel converts a dropdown label to a content type.
func contentTypeFromLabel(label string) catalog.ContentType {
	switch label {
	case "Skills":
		return catalog.Skills
	case "Agents":
		return catalog.Agents
	case "MCP Configs":
		return catalog.MCP
	case "Rules":
		return catalog.Rules
	case "Hooks":
		return catalog.Hooks
	case "Commands":
		return catalog.Commands
	}
	return catalog.Skills
}

// collectionTypeFromLabel converts a dropdown label to a collection type.
func collectionTypeFromLabel(label string) catalog.ContentType {
	switch label {
	case "Library":
		return catalog.Library
	case "Loadouts":
		return catalog.Loadouts
	}
	return catalog.Library
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
