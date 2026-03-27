package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// wizardKind identifies the active full-screen wizard (if any).
type wizardKind int

const (
	wizardNone    wizardKind = iota
	wizardInstall            // install wizard (B3+)
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
	contentRoot     string // syllago content repo root (for re-scanning)
	projectRoot     string

	// Sub-models
	topBar   topBarModel
	library  libraryModel  // Library tab: table + drill-in
	explorer explorerModel // Content/Loadout tabs: items list + preview
	gallery  galleryModel  // Loadouts/Registries tabs: card grid + contents sidebar
	helpBar  helpBarModel
	modal    editModal    // reusable edit overlay (name + description)
	confirm  confirmModal // reusable confirm overlay (uninstall + simple confirms)
	remove   removeModal  // multi-step remove overlay (library item removal)
	help     helpOverlay  // keyboard shortcut reference (? key)
	toast    toastModel   // bottom-right notification overlay

	// Wizard mode — when active, captures all key/mouse input
	wizardMode    wizardKind
	installWizard *installWizardModel // nil when not active

	// Dimensions
	width, height int

	// State
	ready            bool   // false until first WindowSizeMsg
	galleryDrillIn   bool   // true when viewing card contents as a library
	galleryDrillCard string // name of the card we drilled into (for breadcrumbs)
}

// NewApp creates a new TUI app. Signature matches main.go.
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string, autoUpdate bool, registrySources []catalog.RegistrySource, cfg *config.Config, isReleaseBuild bool, contentRoot, projectRoot string) App {
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
		contentRoot:     contentRoot,
		projectRoot:     projectRoot,
		topBar:          newTopBar(),
		library:         newLibraryModel(cat.Items, providers, projectRoot),
		explorer:        newExplorerModel(nil, false, providers, projectRoot),
		gallery:         newGalleryModel(),
		helpBar:         newHelpBar(version),
		modal:           newEditModal(),
		confirm:         newConfirmModal(),
		remove:          newRemoveModal(),
		help:            newHelpOverlay(),
		toast:           newToastModel(),
	}
	a.updateNavState()
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
		a.gallery.SetSize(msg.Width, ch)
		a.help.SetSize(msg.Width, ch)
		a.toast.SetSize(msg.Width, ch)
		a.confirm.width = msg.Width
		a.confirm.height = ch
		a.remove.width = msg.Width
		a.remove.height = ch
		if a.installWizard != nil {
			a.installWizard.width = msg.Width
			a.installWizard.height = msg.Height
			a.installWizard.shell.SetWidth(msg.Width)
		}
		return a, nil

	case tea.MouseMsg:
		// Wizard mode captures all mouse input
		if a.wizardMode != wizardNone {
			return a.routeToWizard(msg)
		}
		// Modal and help overlay capture all mouse input when active
		if a.modal.active {
			var cmd tea.Cmd
			a.modal, cmd = a.modal.Update(msg)
			return a, cmd
		}
		if a.confirm.active {
			var cmd tea.Cmd
			a.confirm, cmd = a.confirm.Update(msg)
			return a, cmd
		}
		if a.remove.active {
			var cmd tea.Cmd
			a.remove, cmd = a.remove.Update(msg)
			return a, cmd
		}
		if a.help.active {
			var cmd tea.Cmd
			a.help, cmd = a.help.Update(msg)
			return a, cmd
		}
		return a.routeMouse(msg)

	case tea.KeyMsg:
		// Toast dismissal takes priority over modals — Esc dismisses toast first.
		if a.toast.visible && msg.Type == tea.KeyEsc {
			cmd := a.toast.Dismiss()
			return a, cmd
		}

		// Wizard mode captures all key input (except ctrl+c and toast dismiss)
		if a.wizardMode != wizardNone {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			return a.routeToWizard(msg)
		}

		// Modal captures all key input when active (except ctrl+c)
		if a.modal.active {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.modal, cmd = a.modal.Update(msg)
			return a, cmd
		}

		// Confirm modal captures all key input when active (except ctrl+c)
		if a.confirm.active {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.confirm, cmd = a.confirm.Update(msg)
			return a, cmd
		}

		// Remove modal captures all key input when active (except ctrl+c)
		if a.remove.active {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.remove, cmd = a.remove.Update(msg)
			return a, cmd
		}

		// Help overlay captures all key input when active
		if a.help.active {
			var cmd tea.Cmd
			a.help, cmd = a.help.Update(msg)
			return a, cmd
		}

		// Toast consumes Esc and 'c' when visible (after modal/help)
		if a.toast.visible {
			consumed, cmd := a.toast.HandleKey(msg)
			if consumed {
				return a, cmd
			}
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
			// Only quit from top-level browse. If in a drill-down or
			// non-landing view, back out one level instead.

			// Gallery drill-in: back out through library detail → library browse → gallery
			if a.isGalleryTab() && a.galleryDrillIn {
				if a.library.mode == libraryDetail {
					a.library.mode = libraryBrowse
					a.library.detailItem = nil
					a.library.SetSize(a.width, a.contentHeight())
					a.updateNavState()
					return a, nil
				}
				// Exit drill-in back to gallery
				a.galleryDrillIn = false
				a.galleryDrillCard = ""
				a.library.SetItems(a.catalog.Items) // restore full library
				a.updateNavState()
				return a, nil
			}

			if a.isLibraryTab() && a.library.mode == libraryDetail {
				a.library.mode = libraryBrowse
				a.library.detailItem = nil
				a.library.SetSize(a.width, a.contentHeight())
				a.updateNavState()
				return a, nil
			}
			if !a.isLibraryTab() && a.explorer.mode == explorerDetail {
				a.explorer.mode = explorerBrowse
				a.explorer.detailItem = nil
				a.explorer.sizeBrowsePanes()
				a.explorer.preview.LoadItem(a.explorer.items.Selected())
				a.updateNavState()
				return a, nil
			}
			if !a.isLibraryTab() {
				// Return to landing page (Collections > Library)
				cmd := a.topBar.SetGroup(0)
				a.galleryDrillIn = false
				a.refreshContent()
				a.updateNavState()
				return a, cmd
			}
			return a, tea.Quit

		// 1/2/3 switch groups
		case msg.String() == keyGroup1:
			cmd := a.topBar.SetGroup(0)
			a.refreshContent()
			a.updateNavState()
			return a, cmd
		case msg.String() == keyGroup2:
			cmd := a.topBar.SetGroup(1)
			a.refreshContent()
			a.updateNavState()
			return a, cmd
		case msg.String() == keyGroup3:
			cmd := a.topBar.SetGroup(2)
			a.refreshContent()
			a.updateNavState()
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
		// keyCreate ("n") is deferred — no-op for now

		// Edit selected item (name + description)
		case msg.String() == keyEdit:
			return a.handleEdit()

		// Remove item from library
		case msg.String() == keyRemove:
			return a.handleRemove()

		// Uninstall item from provider
		case msg.String() == keyUninstall:
			return a.handleUninstall()

		// Install item to a provider
		case msg.String() == keyInstall:
			return a.handleInstall()

		// Help overlay
		case msg.String() == keyHelp:
			a.help.Toggle()
			return a, nil

		// Refresh catalog (re-scan content from disk)
		case msg.String() == keyRefresh:
			cmd := a.rescanCatalog()
			return a, cmd

		// Route to active content model
		default:
			return a.routeKey(msg)
		}

	case toastTickMsg:
		var cmd tea.Cmd
		a.toast, cmd = a.toast.Update(msg)
		return a, cmd

	case editSavedMsg:
		return a.handleEditSaved(msg)

	case editCancelledMsg:
		return a, nil

	case confirmResultMsg:
		return a.handleConfirmResult(msg)

	case removeResultMsg:
		return a.handleRemoveResult(msg)

	case removeDoneMsg:
		return a.handleRemoveDone(msg)

	case uninstallDoneMsg:
		return a.handleUninstallDone(msg)

	case installResultMsg:
		return a.handleInstallResult(msg)

	case installDoneMsg:
		return a.handleInstallDone(msg)

	case installCloseMsg:
		a.installWizard = nil
		a.wizardMode = wizardNone
		return a, nil

	case tabChangedMsg:
		a.galleryDrillIn = false
		a.refreshContent()
		a.updateNavState()
		return a, nil

	case libraryEditMsg:
		return a.handleEdit()

	case libraryInstallMsg:
		return a.handleInstall()

	case libraryRemoveMsg:
		return a.handleRemove()

	case libraryUninstallMsg:
		return a.handleUninstall()

	case libraryDrillMsg:
		a.updateNavState()
		return a, nil

	case libraryCloseMsg:
		a.updateNavState()
		return a, nil

	case explorerDrillMsg:
		a.updateNavState()
		return a, nil

	case explorerCloseMsg:
		a.updateNavState()
		return a, nil

	case cardSelectedMsg:
		return a, nil

	case cardDrillMsg:
		// Drill into the card — show a library view filtered to this card's items
		if msg.card != nil && len(msg.card.items) > 0 {
			a.galleryDrillIn = true
			a.galleryDrillCard = msg.card.name
			a.library.SetItems(msg.card.items)
			a.library.SetSize(a.width, a.contentHeight())
			a.updateNavState()
		}
		return a, nil

	case breadcrumbClickMsg:
		return a.handleBreadcrumbClick(msg)

	case itemSelectedMsg:
		return a, nil

	case actionPressedMsg:
		switch msg.action {
		case "add":
			// Add wizard (Phase D) — no-op for now
		case "remove":
			return a.handleRemove()
		case "uninstall":
			return a.handleUninstall()
		}
		return a, nil

	case helpToggleMsg:
		a.help.Toggle()
		return a, nil
	}
	return a, nil
}

// handleEdit opens the edit modal for the currently selected item or card.
func (a App) handleEdit() (tea.Model, tea.Cmd) {
	// Gallery card (not drilled in) — edit the card directly
	if a.isGalleryTab() && !a.galleryDrillIn {
		card := a.gallery.grid.Selected()
		if card == nil || card.path == "" {
			return a, nil
		}
		a.modal.Open("Edit: "+card.name, card.name, card.desc, card.path)
		return a, nil
	}

	var item *catalog.ContentItem
	if a.isLibraryTab() || (a.isGalleryTab() && a.galleryDrillIn) {
		item = a.library.table.Selected()
	} else {
		item = a.explorer.items.Selected()
	}
	if item == nil {
		return a, nil
	}
	currentName := itemDisplayName(*item)
	a.modal.Open("Edit: "+item.Name, currentName, item.Description, item.Path)
	return a, nil
}

// handleEditSaved persists name and description to .syllago.yaml and updates in-place.
// Directory items use metadata.Load/Save (writes <dir>/.syllago.yaml).
// Single-file items (legacy hooks/MCP) use metadata.LoadProvider/SaveProvider
// (writes <parentdir>/.syllago.<filename>.yaml).
func (a App) handleEditSaved(msg editSavedMsg) (tea.Model, tea.Cmd) {
	if msg.path == "" {
		return a, nil
	}

	meta, err := loadMetaForPath(msg.path)
	if err != nil {
		cmd := a.toast.Push("Failed to load metadata: "+err.Error(), toastError)
		return a, cmd
	}
	if meta == nil {
		meta = &metadata.Meta{}
	}
	meta.Name = msg.name
	meta.Description = msg.description
	if err := saveMetaForPath(msg.path, meta); err != nil {
		cmd := a.toast.Push("Failed to save: "+err.Error(), toastError)
		return a, cmd
	}

	// Update in-place in the catalog (avoid full re-scan)
	for i := range a.catalog.Items {
		if a.catalog.Items[i].Path == msg.path {
			a.catalog.Items[i].DisplayName = msg.name
			a.catalog.Items[i].Description = msg.description
			break
		}
	}
	a.refreshContent()
	cmd := a.toast.Push("Saved", toastSuccess)
	return a, cmd
}

// routeKey sends key messages to the active content model.
func (a App) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.isLibraryTab() {
		var cmd tea.Cmd
		a.library, cmd = a.library.Update(msg)
		return a, cmd
	}
	if a.isGalleryTab() {
		if a.galleryDrillIn {
			var cmd tea.Cmd
			a.library, cmd = a.library.Update(msg)
			return a, cmd
		}
		var cmd tea.Cmd
		a.gallery, cmd = a.gallery.Update(msg)
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
	} else if a.isGalleryTab() && a.galleryDrillIn {
		a.library, contentCmd = a.library.Update(msg)
	} else if a.isGalleryTab() {
		a.gallery, contentCmd = a.gallery.Update(msg)
	} else {
		a.explorer, contentCmd = a.explorer.Update(msg)
	}
	return a, tea.Batch(topCmd, contentCmd)
}

// routeToWizard dispatches messages to the active wizard.
func (a App) routeToWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.wizardMode {
	case wizardInstall:
		if a.installWizard != nil {
			_, cmd := a.installWizard.Update(msg)
			return a, cmd
		}
	}
	return a, nil
}

// View implements tea.Model.
func (a App) View() string {
	if !a.ready {
		return ""
	}
	if a.width < 80 || a.height < 20 {
		return a.renderTooSmall()
	}

	// Full-screen wizard takes over the entire viewport.
	if a.wizardMode == wizardInstall && a.installWizard != nil {
		view := a.installWizard.View()
		if a.toast.visible {
			view = overlayToast(view, a.toast.View(), a.width, a.height)
		}
		return zone.Scan(view)
	}

	topBar := a.topBar.View()
	content := a.renderContent()
	helpBar := a.helpBar.View()

	// Overlay modals on top of existing content
	if a.modal.active {
		content = overlayModal(content, a.modal.View(), a.width, a.contentHeight())
	}
	if a.confirm.active {
		content = overlayModal(content, a.confirm.View(), a.width, a.contentHeight())
	}
	if a.remove.active {
		content = overlayModal(content, a.remove.View(), a.width, a.contentHeight())
	}
	if a.help.active {
		content = overlayModal(content, a.help.View(), a.width, a.contentHeight())
	}
	if a.toast.visible {
		content = overlayToast(content, a.toast.View(), a.width, a.contentHeight())
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

	if group == "Config" {
		return a.renderPlaceholder("Settings view coming soon")
	}

	if a.isLibraryTab() {
		return a.library.View()
	}

	if a.isGalleryTab() {
		if a.galleryDrillIn {
			return a.library.View()
		}
		return a.gallery.View()
	}

	return a.explorer.View()
}

// isLibraryTab returns true if the active tab is Collections > Library.
func (a App) isLibraryTab() bool {
	return a.topBar.ActiveGroupLabel() == "Collections" && a.topBar.ActiveTabLabel() == "Library"
}

// isGalleryTab returns true if the active tab should show the gallery grid.
func (a App) isGalleryTab() bool {
	group := a.topBar.ActiveGroupLabel()
	tab := a.topBar.ActiveTabLabel()
	return group == "Collections" && (tab == "Loadouts" || tab == "Registries")
}

// rescanCatalog re-reads all content from disk and refreshes the active view.
// Re-reads the config to pick up registries added/removed since startup.
func (a *App) rescanCatalog() tea.Cmd {
	root := a.contentRoot
	if root == "" {
		root = a.projectRoot
	}
	projectRoot := a.projectRoot
	if projectRoot == "" {
		projectRoot = root
	}

	// Reload config from all sources to pick up changes.
	// Registries may be stored in the content root config, project config,
	// or global config — merge all three to cover every case.
	globalCfg, _ := config.LoadGlobal()
	projectCfg, _ := config.Load(projectRoot)
	contentCfg, _ := config.Load(root)
	merged := config.Merge(globalCfg, config.Merge(contentCfg, projectCfg))
	var regSources []catalog.RegistrySource
	for _, r := range merged.Registries {
		if registry.IsCloned(r.Name) {
			dir, _ := registry.CloneDir(r.Name)
			regSources = append(regSources, catalog.RegistrySource{Name: r.Name, Path: dir})
		}
	}
	a.registrySources = regSources

	cat, err := catalog.ScanWithGlobalAndRegistries(root, projectRoot, regSources)
	if err != nil {
		return a.toast.Push("Refresh failed: "+err.Error(), toastError)
	}
	a.catalog = cat
	a.galleryDrillIn = false
	a.refreshContent()
	a.updateNavState()
	return a.toast.Push("Catalog refreshed", toastSuccess)
}

// refreshContent updates the active content model based on the current tab.
func (a *App) refreshContent() {
	ch := a.contentHeight()
	if a.isLibraryTab() {
		a.galleryDrillIn = false
		a.library.SetItems(a.catalog.Items)
		a.library.SetSize(a.width, ch)
		return
	}
	if a.isGalleryTab() {
		if !a.galleryDrillIn {
			a.refreshGallery()
			a.gallery.SetSize(a.width, ch)
		}
		return
	}
	a.galleryDrillIn = false
	items, mixed := a.itemsForCurrentTab()
	a.explorer.SetItems(items, mixed)
	a.explorer.SetSize(a.width, ch)
}

// refreshGallery populates the gallery with cards for the current tab.
func (a *App) refreshGallery() {
	tab := a.topBar.ActiveTabLabel()
	switch tab {
	case "Loadouts":
		cards := buildLoadoutCards(a.catalog.ByType(catalog.Loadouts), a.catalog)
		a.gallery.SetCards(cards, "Loadout")
	case "Registries":
		cards := buildRegistryCards(a.registrySources, a.catalog)
		a.gallery.SetCards(cards, "Registry")
	}
}

// itemsForCurrentTab returns the catalog items for the active tab.
func (a App) itemsForCurrentTab() ([]catalog.ContentItem, bool) {
	group := a.topBar.ActiveGroupLabel()
	tab := a.topBar.ActiveTabLabel()

	switch group {
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

	base := []string{"1/2/3 groups", "tab items"}

	if group == "Config" {
		return append(base, "R refresh", "? help", "q quit")
	}

	// Gallery tabs (Loadouts/Registries)
	if a.isGalleryTab() && a.galleryDrillIn {
		if a.library.mode == libraryDetail {
			return append(base, "↑/↓ navigate", "←/→ switch pane", "esc close", "R refresh", "? help", "q back")
		}
		return append(base, "↑/↓ navigate", "enter preview", "/ search", "s sort", "e edit", "d remove", "x uninstall", "R refresh", "? help", "q back")
	}
	if a.isGalleryTab() {
		return append(base, "arrows grid", "enter select", "tab grid/contents", "e edit", "d remove", "R refresh", "a add", "? help", "q back")
	}

	// Library in detail mode has different hints
	if a.isLibraryTab() && a.library.mode == libraryDetail {
		return append(base, "↑/↓ navigate", "←/→ switch pane", "esc close", "R refresh", "? help", "q quit")
	}

	if a.isLibraryTab() {
		return append(base, "↑/↓ navigate", "enter preview", "/ search", "s sort", "i install", "e edit", "d remove", "x uninstall", "R refresh", "a add", "? help", "q quit")
	}

	// Explorer in detail mode
	if a.explorer.mode == explorerDetail {
		return append(base, "↑/↓ navigate", "←/→ switch pane", "esc close", "e edit", "R refresh", "? help", "q back")
	}

	hints := append(base, "↑/↓ navigate", "←/→ switch pane", "enter detail")
	if group != "Config" {
		hints = append(hints, "i install", "e edit", "d remove", "x uninstall", "R refresh", "a add")
	}
	return append(hints, "? help", "q quit")
}

// handleBreadcrumbClick navigates back to the clicked breadcrumb level.
func (a App) handleBreadcrumbClick(msg breadcrumbClickMsg) (tea.Model, tea.Cmd) {
	// Gallery drill-in breadcrumbs: [0]=card name, [1]=item name
	if a.isGalleryTab() && a.galleryDrillIn {
		if msg.index == 0 {
			// Click on card name → back to library browse (stay in drill-in)
			if a.library.mode == libraryDetail {
				a.library.mode = libraryBrowse
				a.library.detailItem = nil
				a.library.SetSize(a.width, a.contentHeight())
				a.updateNavState()
			}
			return a, nil
		}
		// index >= 1 shouldn't navigate anywhere (already at that depth)
		return a, nil
	}

	// Library drill-in: [0]=item name → close detail
	if a.isLibraryTab() && a.library.mode == libraryDetail {
		// Click on item crumb does nothing (already there)
		return a, nil
	}

	// Explorer drill-in: [0]=item name → close detail
	if a.explorer.mode == explorerDetail {
		// Click on item crumb does nothing (already there)
		return a, nil
	}

	return a, nil
}

// updateNavState refreshes hints and breadcrumbs to match the current navigation state.
// Call this after any state transition that changes drill-in/tab state.
func (a *App) updateNavState() {
	a.helpBar.SetHints(a.currentHints())
	a.updateBreadcrumbs()
}

// updateBreadcrumbs sets the topbar breadcrumbs based on the current navigation state.
func (a *App) updateBreadcrumbs() {
	var crumbs []string

	// Gallery drill-in: card name, then optionally detail item
	if a.isGalleryTab() && a.galleryDrillIn {
		crumbs = append(crumbs, a.galleryDrillCard)
		if a.library.mode == libraryDetail && a.library.detailItem != nil {
			crumbs = append(crumbs, itemDisplayName(*a.library.detailItem))
		}
		a.topBar.SetBreadcrumbs(crumbs)
		return
	}

	// Library detail drill-in
	if a.isLibraryTab() && a.library.mode == libraryDetail && a.library.detailItem != nil {
		crumbs = append(crumbs, itemDisplayName(*a.library.detailItem))
		a.topBar.SetBreadcrumbs(crumbs)
		return
	}

	// Explorer detail drill-in
	if !a.isLibraryTab() && !a.isGalleryTab() && a.explorer.mode == explorerDetail && a.explorer.detailItem != nil {
		crumbs = append(crumbs, itemDisplayName(*a.explorer.detailItem))
		a.topBar.SetBreadcrumbs(crumbs)
		return
	}

	// No drill-in — clear breadcrumbs
	a.topBar.ClearBreadcrumbs()
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

// overlayModal centers the modal within the content area. The background
// content shows through on all sides of the modal.
func overlayModal(bg, modal string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	modalLines := strings.Split(modal, "\n")
	modalH := len(modalLines)

	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	// Center the modal vertically, splice modal into each background row
	startRow := max(0, (height-modalH)/2)
	for i, mLine := range modalLines {
		row := startRow + i
		if row >= len(bgLines) {
			break
		}
		// Center the modal line horizontally
		mLineW := lipgloss.Width(mLine)
		pad := max(0, (width-mLineW)/2)
		rightStart := pad + mLineW

		// Splice: bg_left + modal + bg_right
		left := ansi.Truncate(bgLines[row], pad, "")
		right := ""
		if rightStart < width {
			right = ansi.Cut(bgLines[row], rightStart, width)
		}
		bgLines[row] = left + mLine + right
	}

	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}
	return strings.Join(bgLines, "\n")
}

// isFilePath returns true if path points to a file (not a directory).
func isFilePath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		// If path doesn't exist, check if it has an extension (heuristic for files).
		return filepath.Ext(path) != ""
	}
	return !info.IsDir()
}

// loadMetaForPath loads metadata for either a directory or single-file item.
func loadMetaForPath(path string) (*metadata.Meta, error) {
	if isFilePath(path) {
		return metadata.LoadProvider(filepath.Dir(path), filepath.Base(path))
	}
	return metadata.Load(path)
}

// saveMetaForPath saves metadata for either a directory or single-file item.
func saveMetaForPath(path string, meta *metadata.Meta) error {
	if isFilePath(path) {
		return metadata.SaveProvider(filepath.Dir(path), filepath.Base(path), meta)
	}
	return metadata.Save(path, meta)
}
