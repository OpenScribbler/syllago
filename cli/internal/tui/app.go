package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/promote"
	"github.com/holdenhewett/nesco/cli/internal/provider"
)

type screen int

const (
	screenCategory screen = iota
	screenItems
	screenDetail
	screenImport
	screenUpdate
	screenSettings
)

type focusTarget int

const (
	focusSidebar focusTarget = iota
	focusContent
	focusModal
)

// App is the root bubbletea model.
type App struct {
	catalog    *catalog.Catalog
	providers  []provider.Provider
	version    string
	autoUpdate bool

	screen      screen
	focus       focusTarget
	modal       confirmModal
	saveModal   saveModal
	envModal    envSetupModal
	sidebar     sidebarModel
	items       itemsModel
	detail      detailModel
	search      searchModel
	helpOverlay helpOverlayModel
	importer    importModel
	updater     updateModel
	settings      settingsModel
	statusMessage  string
	statusWarnings []string

	// Detail model cache (preserves state when re-entering same item)
	cachedDetail     *detailModel
	cachedDetailPath string

	// Update check state (persists across screen changes)
	remoteVersion string
	commitsBehind int

	width    int
	height   int
	tooSmall bool // true when terminal is below minimum usable size
}

// NewApp creates a new App model. Set autoUpdate to true to pull updates
// automatically when a newer version is detected on origin.
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string, autoUpdate bool) App {
	return App{
		catalog:    cat,
		providers:  providers,
		version:    version,
		autoUpdate: autoUpdate,
		screen:     screenCategory,
		focus:      focusSidebar,
		sidebar:    newSidebarModel(cat, version),
		search:     newSearchModel(),
	}
}

func (a App) Init() tea.Cmd {
	return checkForUpdate(a.catalog.RepoRoot, a.version)
}

// panelHeight returns the usable height for sidebar and content panels,
// reserving space for the footer bar (border line + text = 2 rows).
func (a App) panelHeight() int {
	h := a.height - 2
	if h < 5 {
		h = 5
	}
	return h
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.tooSmall = msg.Width < 60 || msg.Height < 20
		// sidebarWidth is inner content width; rendered width is sidebarWidth + 1 (right border).
		contentW := msg.Width - sidebarWidth - 1
		if contentW < 20 {
			contentW = 20
		}
		ph := a.panelHeight()
		a.sidebar.height = ph
		a.items.width = contentW
		a.items.height = ph
		a.detail.width = contentW
		a.detail.height = ph
		a.detail.clampScroll()
		a.importer.width = contentW
		a.importer.height = ph
		a.updater.width = contentW
		a.updater.height = ph
		a.settings.width = contentW
		a.settings.height = ph
		return a, nil

	case appInstallDoneMsg:
		a.cachedDetail = nil // invalidate cache (install state changed)
		if a.screen == screenDetail {
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd
		}

	case promoteDoneMsg:
		if a.screen == screenDetail {
			a.cachedDetail = nil // invalidate cache
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			// Rescan catalog after promote
			if msg.err == nil {
				cat, err := catalog.Scan(a.catalog.RepoRoot)
				if err == nil {
					a.catalog = cat
					a.sidebar.counts = cat.CountByType()
					a.sidebar.localCount = cat.CountLocal()
				}
			}
			return a, cmd
		}

	case fileBrowserDoneMsg:
		if a.screen == screenImport {
			var cmd tea.Cmd
			a.importer, cmd = a.importer.Update(msg)
			return a, cmd
		}

	case importCloneDoneMsg:
		if a.screen == screenImport {
			var cmd tea.Cmd
			a.importer, cmd = a.importer.Update(msg)
			return a, cmd
		}

	case importDoneMsg:
		a.cachedDetail = nil // invalidate cache
		if msg.err != nil {
			a.importer.message = fmt.Sprintf("Import failed: %s", msg.err)
			a.importer.messageIsErr = true
			return a, nil
		}
		// Rescan catalog
		cat, err := catalog.Scan(a.catalog.RepoRoot)
		if err == nil {
			a.catalog = cat
			a.statusMessage = fmt.Sprintf("Imported %q successfully", msg.name)
		} else {
			a.statusMessage = fmt.Sprintf("Imported %q but catalog rescan failed: %s", msg.name, err)
		}
		a.statusWarnings = msg.warnings
		a.sidebar.counts = a.catalog.CountByType()
		a.sidebar.localCount = a.catalog.CountLocal()
		a.screen = screenCategory
		a.importer.cleanup()
		return a, nil

	case updateCheckMsg:
		if msg.err == nil && msg.remoteVersion != "" && versionNewer(msg.remoteVersion, msg.localVersion) {
			a.remoteVersion = msg.remoteVersion
			a.commitsBehind = msg.commitsBehind
			a.sidebar.remoteVersion = msg.remoteVersion
			a.sidebar.updateAvailable = true
			a.sidebar.commitsBehind = msg.commitsBehind

			if a.autoUpdate {
				a.updater = newUpdateModel(a.catalog.RepoRoot, a.version, a.remoteVersion, a.commitsBehind)
				a.updater.width = a.width - sidebarWidth - 1
				a.updater.height = a.panelHeight()
				a.screen = screenUpdate
				return a, a.updater.startPull()
			}
		}
		return a, nil

	case spinner.TickMsg:
		if a.screen == screenUpdate {
			var cmd tea.Cmd
			a.updater, cmd = a.updater.Update(msg)
			return a, cmd
		}

	case updatePreviewMsg:
		if a.screen == screenUpdate {
			var cmd tea.Cmd
			a.updater, cmd = a.updater.Update(msg)
			return a, cmd
		}

	case updatePullMsg:
		if a.screen == screenUpdate {
			var cmd tea.Cmd
			a.updater, cmd = a.updater.Update(msg)
			// On successful pull, rescan catalog
			if msg.err == nil {
				cat, err := catalog.Scan(a.catalog.RepoRoot)
				if err == nil {
					a.catalog = cat
					a.sidebar.counts = cat.CountByType()
					a.sidebar.localCount = cat.CountLocal()
				}
			}
			return a, cmd
		}

	case openModalMsg:
		a.modal = newConfirmModal(msg.title, msg.body)
		a.modal.purpose = msg.purpose
		a.focus = focusModal
		return a, nil

	case openSaveModalMsg:
		a.saveModal = newSaveModal("filename.md")
		a.focus = focusModal
		return a, nil

	case openEnvModalMsg:
		a.envModal = newEnvSetupModal(msg.envTypes)
		a.focus = focusModal
		return a, nil

	case tea.MouseMsg:
		// Forward wheel events to active screen for scroll support
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			switch a.screen {
			case screenImport:
				var cmd tea.Cmd
				a.importer, cmd = a.importer.Update(msg)
				return a, cmd
			case screenDetail:
				if msg.Button == tea.MouseButtonWheelUp {
					a.detail.scrollOffset--
				} else {
					a.detail.scrollOffset++
				}
				a.detail.clampScroll()
				return a, nil
			case screenUpdate:
				if a.updater.step == stepUpdatePreview {
					if msg.Button == tea.MouseButtonWheelUp {
						if a.updater.scrollOffset > 0 {
							a.updater.scrollOffset--
						}
					} else {
						a.updater.scrollOffset++
					}
				}
				return a, nil
			}
			return a, nil
		}
		if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
			return a, nil
		}
		// Check sidebar zones
		for i := 0; i < a.sidebar.totalItems(); i++ {
			if zone.Get(fmt.Sprintf("sidebar-%d", i)).InBounds(msg) {
				a.sidebar.cursor = i
				a.screen = screenCategory
				a.focus = focusSidebar
				// Synthesize Enter to load content
				return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
			}
		}
		// Check item list zones
		if a.screen == screenItems {
			for i := range a.items.items {
				if zone.Get(fmt.Sprintf("item-%d", i)).InBounds(msg) {
					a.items.cursor = i
					a.focus = focusContent
					return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
				}
			}
		}
		// Check welcome page category links
		allTypes := catalog.AllContentTypes()
		for i, ct := range allTypes {
			if zone.Get(fmt.Sprintf("welcome-%d", i)).InBounds(msg) {
				// Find the sidebar index for this type and select it
				for j, st := range a.sidebar.types {
					if st == ct {
						a.sidebar.cursor = j
						break
					}
				}
				a.screen = screenCategory
				a.focus = focusSidebar
				return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
			}
		}
		// Check welcome page configuration links
		welcomeConfigMap := map[string]int{
			"welcome-import":   len(a.sidebar.types) + 1,
			"welcome-update":   len(a.sidebar.types) + 2,
			"welcome-settings": len(a.sidebar.types) + 3,
		}
		for zoneID, sidebarIdx := range welcomeConfigMap {
			if zone.Get(zoneID).InBounds(msg) {
				a.sidebar.cursor = sidebarIdx
				a.focus = focusSidebar
				return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
			}
		}
		// Check breadcrumb zones (detail and items screens)
		if zone.Get("crumb-home").InBounds(msg) {
			a.screen = screenCategory
			a.focus = focusSidebar
			return a, nil
		}
		if zone.Get("crumb-category").InBounds(msg) {
			if a.screen == screenDetail {
				// Navigate back to items list
				return a.Update(tea.KeyMsg{Type: tea.KeyEsc})
			}
		}
		// Check detail tab zones
		if a.screen == screenDetail {
			tabs := []detailTab{tabOverview, tabFiles, tabInstall}
			for _, tab := range tabs {
				if zone.Get(fmt.Sprintf("tab-%d", int(tab))).InBounds(msg) {
					a.detail.activeTab = tab
					return a, nil
				}
			}
		}
		// Check detail action button zones
		if a.screen == screenDetail {
			btnChars := map[string]string{
				"detail-btn-install":   "i",
				"detail-btn-uninstall": "u",
				"detail-btn-copy":      "c",
				"detail-btn-save":      "s",
			}
			for zoneID, char := range btnChars {
				if zone.Get(zoneID).InBounds(msg) {
					return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(char)})
				}
			}
		}
		// Forward left-clicks to content screens that handle their own zones
		switch a.screen {
		case screenImport:
			var cmd tea.Cmd
			a.importer, cmd = a.importer.Update(msg)
			return a, cmd
		case screenUpdate:
			var cmd tea.Cmd
			a.updater, cmd = a.updater.Update(msg)
			return a, cmd
		case screenSettings:
			var cmd tea.Cmd
			a.settings, cmd = a.settings.Update(msg)
			return a, cmd
		}
		return a, nil

	case tea.KeyMsg:
		// ctrl+c always quits from any screen
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		// If a modal is active, route all input to it
		if a.modal.active {
			var cmd tea.Cmd
			a.modal, cmd = a.modal.Update(msg)
			if !a.modal.active {
				a.focus = focusContent // return focus after dismiss
				if a.modal.confirmed {
					switch a.modal.purpose {
					case modalInstall:
						a.detail.doInstallChecked()
					case modalUninstall:
						a.detail.doUninstallAll()
					case modalPromote:
						repoRoot := a.detail.repoRoot
						item := a.detail.item
						return a, func() tea.Msg {
							result, err := promote.Promote(repoRoot, item)
							return promoteDoneMsg{result: result, err: err}
						}
					case modalAppScript:
						return a, a.detail.runAppScript()
					}
				}
			}
			return a, cmd
		}
		if a.saveModal.active {
			var cmd tea.Cmd
			a.saveModal, cmd = a.saveModal.Update(msg)
			if !a.saveModal.active && a.saveModal.confirmed {
				a.detail.savePath = a.saveModal.value
				a.detail.doSave()
				a.focus = focusContent
			} else if !a.saveModal.active {
				a.focus = focusContent
			}
			return a, cmd
		}
		if a.envModal.active {
			var cmd tea.Cmd
			a.envModal, cmd = a.envModal.Update(msg)
			if !a.envModal.active {
				a.focus = focusContent
			}
			return a, cmd
		}
		// q quits from sidebar, navigates back from content screens
		if key.Matches(msg, keys.Quit) && !a.search.active {
			// Quit when focus is on the sidebar (home base) or on the root category screen
			if a.screen == screenCategory || a.focus == focusSidebar {
				return a, tea.Quit
			}
			// Skip if text input is active
			if a.screen == screenDetail && a.detail.HasTextInput() {
				// fall through to normal handling
			} else if a.screen == screenImport && a.importer.hasTextInput() {
				// fall through to normal handling
			} else if a.screen == screenSettings && a.settings.editMode != editNone {
				// fall through to normal handling
			} else {
				// Synthesize esc key to navigate back
				return a.Update(tea.KeyMsg{Type: tea.KeyEsc})
			}
		}

		// Help overlay toggle (skip when search active or text input active)
		if key.Matches(msg, keys.Help) && !a.search.active {
			if a.helpOverlay.active {
				a.helpOverlay.active = false
				return a, nil
			}
			if a.screen == screenDetail && a.detail.HasTextInput() {
				// Don't activate during text input
			} else {
				a.helpOverlay.active = true
				return a, nil
			}
		}

		// If help overlay is active, esc closes it; all other keys are swallowed
		if a.helpOverlay.active {
			if msg.Type == tea.KeyEsc {
				a.helpOverlay.active = false
			}
			return a, nil
		}

		// Search toggle (skip on import/update/settings screens and when detail has active textinput)
		if key.Matches(msg, keys.Search) && !a.search.active && a.screen != screenImport && a.screen != screenUpdate && a.screen != screenSettings && !a.detail.HasTextInput() {
			a.search = a.search.activated()
			return a, nil
		}

		// Search active: handle escape and enter
		if a.search.active {
			if msg.Type == tea.KeyEsc {
				a.search = a.search.deactivated()
				// If on items screen, reset items
				if a.screen == screenItems {
					ct := a.sidebar.selectedType()
					items := newItemsModel(ct, a.catalog.ByType(ct), a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.panelHeight()
					a.items = items
				}
				return a, nil
			}
			if msg.Type == tea.KeyEnter {
				// Apply filter
				if a.screen == screenCategory {
					// Search across all items, go to items view with filtered results
					filtered := filterItems(a.catalog.Items, a.search.query())
					items := newItemsModel(catalog.SearchResults, filtered, a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.panelHeight()
					a.items = items
					a.screen = screenItems
				} else if a.screen == screenItems {
					ct := a.sidebar.selectedType()
					filtered := filterItems(a.catalog.ByType(ct), a.search.query())
					items := newItemsModel(ct, filtered, a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.panelHeight()
					a.items = items
				}
				a.search = a.search.deactivated()
				return a, nil
			}
			var cmd tea.Cmd
			a.search, cmd = a.search.Update(msg)

			// Live-filter items while typing
			if a.screen == screenItems {
				query := a.search.query()
				ct := a.items.contentType
				var source []catalog.ContentItem
				if ct == catalog.SearchResults {
					source = a.catalog.Items
				} else if ct == catalog.MyTools {
					for _, item := range a.catalog.Items {
						if item.Local {
							source = append(source, item)
						}
					}
				} else {
					source = a.catalog.ByType(ct)
				}
				filtered := filterItems(source, query)
				items := newItemsModel(ct, filtered, a.providers, a.catalog.RepoRoot)
				items.width = a.width
				items.height = a.panelHeight()
				a.items = items
			}

			// Show match count preview on category screen
			if a.screen == screenCategory {
				query := a.search.query()
				if query != "" {
					a.search.matchCount = len(filterItems(a.catalog.Items, query))
				} else {
					a.search.matchCount = -1
				}
			}

			return a, cmd
		}

		// Tab/Shift+Tab: switch focus between sidebar and content.
		// Guard: NOT on screenDetail (Tab still switches detail tabs when content is focused).
		// On screenDetail, Tab is handled by detail.Update() to switch Overview/Files/Install tabs.
		// Panel-focus Tab only fires when sidebar is focused OR when on screens other than screenDetail.
		if (key.Matches(msg, keys.Tab) || key.Matches(msg, keys.ShiftTab)) &&
			!a.search.active && !a.helpOverlay.active &&
			a.screen != screenDetail {
			if a.screen != screenImport && a.screen != screenUpdate && a.screen != screenSettings {
				if !a.detail.HasTextInput() {
					if a.focus == focusSidebar {
						a.focus = focusContent
					} else {
						a.focus = focusSidebar
					}
					return a, nil
				}
			}
		}

		// Screen-specific key handling
		switch a.screen {
		// Sidebar-focused: route input to sidebar; Enter/Right drills into content
		case screenCategory:
			if a.focus == focusSidebar {
				if key.Matches(msg, keys.Enter) || key.Matches(msg, keys.Right) {
					if a.sidebar.isUpdateSelected() {
						a.updater = newUpdateModel(a.catalog.RepoRoot, a.version, a.remoteVersion, a.commitsBehind)
						a.updater.width = a.width - sidebarWidth - 1
						a.updater.height = a.panelHeight()
						a.screen = screenUpdate
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isSettingsSelected() {
						a.settings = newSettingsModel(a.catalog.RepoRoot, a.providers)
						a.settings.width = a.width - sidebarWidth - 1
						a.settings.height = a.panelHeight()
						a.screen = screenSettings
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isImportSelected() {
						a.importer = newImportModel(a.providers, a.catalog.RepoRoot)
						a.importer.width = a.width - sidebarWidth - 1
						a.importer.height = a.panelHeight()
						a.screen = screenImport
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isMyToolsSelected() {
						var localItems []catalog.ContentItem
						for _, item := range a.catalog.Items {
							if item.Local {
								localItems = append(localItems, item)
							}
						}
						items := newItemsModel(catalog.MyTools, localItems, a.providers, a.catalog.RepoRoot)
						items.width = a.width - sidebarWidth - 1
						items.height = a.panelHeight()
						a.items = items
						a.screen = screenItems
						a.focus = focusContent
						return a, nil
					}
					ct := a.sidebar.selectedType()
					items := newItemsModel(ct, a.catalog.ByType(ct), a.providers, a.catalog.RepoRoot)
					items.width = a.width - sidebarWidth - 1
					items.height = a.panelHeight()
					a.items = items
					a.screen = screenItems
					a.focus = focusContent
					return a, nil
				}
				a.sidebar.focused = true
				var cmd tea.Cmd
				a.sidebar, cmd = a.sidebar.Update(msg)
				return a, cmd
			}

		case screenItems:
			if key.Matches(msg, keys.Back) {
				a.screen = screenCategory
				a.focus = focusSidebar
				if a.catalog != nil {
					a.sidebar.counts = a.catalog.CountByType()
					a.sidebar.localCount = a.catalog.CountLocal()
				}
				return a, nil
			}
			if key.Matches(msg, keys.Enter) && len(a.items.items) > 0 {
				item := a.items.selectedItem()
				if a.cachedDetailPath == item.Path && a.cachedDetail != nil {
					a.detail = *a.cachedDetail
				} else {
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
				}
				a.detail.width = a.width
				a.detail.height = a.panelHeight()
				a.detail.listPosition = a.items.cursor
				a.detail.listTotal = len(a.items.items)
				a.screen = screenDetail
				return a, nil
			}
			var cmd tea.Cmd
			a.items, cmd = a.items.Update(msg)
			return a, cmd

		case screenDetail:
			// Next/previous item navigation (ctrl+n / ctrl+p)
			if msg.String() == "ctrl+n" && !a.detail.HasTextInput() && a.detail.confirmAction == actionNone {
				if a.items.cursor < len(a.items.items)-1 {
					a.items.cursor++
					item := a.items.selectedItem()
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
					a.detail.width = a.width
					a.detail.height = a.panelHeight()
					a.detail.listPosition = a.items.cursor
					a.detail.listTotal = len(a.items.items)
				}
				return a, nil
			}
			if msg.String() == "ctrl+p" && !a.detail.HasTextInput() && a.detail.confirmAction == actionNone {
				if a.items.cursor > 0 {
					a.items.cursor--
					item := a.items.selectedItem()
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
					a.detail.width = a.width
					a.detail.height = a.panelHeight()
					a.detail.listPosition = a.items.cursor
					a.detail.listTotal = len(a.items.items)
				}
				return a, nil
			}
			if key.Matches(msg, keys.Back) {
				// If detail has a pending action (confirmation, method picker),
				// cancel it instead of navigating back
				if a.detail.HasPendingAction() {
					a.detail.CancelAction()
					return a, nil
				}
				// Refresh items to show updated install status, unless search results
				if a.items.contentType == catalog.MyTools {
					var localItems []catalog.ContentItem
					for _, item := range a.catalog.Items {
						if item.Local {
							localItems = append(localItems, item)
						}
					}
					items := newItemsModel(catalog.MyTools, localItems, a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.panelHeight()
					items.cursor = a.items.cursor
					a.items = items
				} else if a.items.contentType != catalog.SearchResults {
					ct := a.items.contentType
					items := newItemsModel(ct, a.catalog.ByType(ct), a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.panelHeight()
					items.cursor = a.items.cursor // preserve cursor position
					a.items = items
				}
				// Cache detail state for re-entry
				cached := a.detail
				a.cachedDetail = &cached
				a.cachedDetailPath = a.detail.item.Path
				a.screen = screenItems
				return a, nil
			}
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd

		case screenImport:
			if key.Matches(msg, keys.Back) && a.importer.step == stepSource {
				a.screen = screenCategory
				a.focus = focusSidebar
				a.importer.cleanup()
				return a, nil
			}
			var cmd tea.Cmd
			a.importer, cmd = a.importer.Update(msg)
			return a, cmd

		case screenUpdate:
			if key.Matches(msg, keys.Back) && (a.updater.step == stepUpdateMenu || a.updater.step == stepUpdateDone) {
				a.screen = screenCategory
				a.focus = focusSidebar
				return a, nil
			}
			var cmd tea.Cmd
			a.updater, cmd = a.updater.Update(msg)
			return a, cmd

		case screenSettings:
			if key.Matches(msg, keys.Back) {
				if a.settings.HasPendingAction() {
					a.settings.CancelAction()
					return a, nil
				}
				if a.settings.dirty {
					a.settings.save()
				}
				a.screen = screenCategory
				a.focus = focusSidebar
				return a, nil
			}
			var cmd tea.Cmd
			a.settings, cmd = a.settings.Update(msg)
			return a, cmd
		}
	}

	return a, nil
}

func (a App) View() string {
	if a.tooSmall {
		// Below minimum: skip sidebar, show warning full-width
		if a.width < 60 || a.height < 20 {
			return "\n" + warningStyle.Render("Terminal too small. Resize to at least 60x20.") + "\n"
		}
	}

	// sidebarWidth is the lipgloss inner content width. With BorderRight(true),
	// the rendered sidebar is sidebarWidth + 1 characters wide.
	// Subtract the extra border character so content does not overflow the terminal.
	contentWidth := a.width - sidebarWidth - 1
	if contentWidth < 20 {
		contentWidth = 20
	}
	_ = contentWidth // used by sub-models via WindowSizeMsg; kept for clarity

	// Sidebar (always visible on the left)
	a.sidebar.focused = (a.focus == focusSidebar)
	sidebarView := a.sidebar.View()

	// Content area: route to the active sub-view
	var contentView string
	switch a.screen {
	case screenItems:
		contentView = a.items.View()
	case screenDetail:
		contentView = a.detail.View()
	case screenImport:
		contentView = a.importer.View()
	case screenUpdate:
		contentView = a.updater.View()
	case screenSettings:
		contentView = a.settings.View()
	default:
		// screenCategory: sidebar is the primary UI; show welcome guidance in content
		contentView = a.renderContentWelcome()
	}

	// Help overlay replaces the content view entirely
	if a.helpOverlay.active {
		contentView = a.helpOverlay.View(a.screen)
	}

	// Constrain content to the same height as the sidebar so panels align.
	contentView = lipgloss.NewStyle().Height(a.panelHeight()).Render(contentView)

	// Compose sidebar + content side by side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, contentView)

	// Footer: context-sensitive help on the left, breadcrumb on the right
	footer := a.renderFooter()

	// Search overlay replaces the footer
	if a.search.active {
		footer = a.search.View()
	}

	body := lipgloss.JoinVertical(lipgloss.Left, panels, footer)

	if a.modal.active {
		body = a.modal.overlayView(body)
	}

	if a.saveModal.active {
		body = a.saveModal.overlayView(body)
	}

	if a.envModal.active {
		body = a.envModal.overlayView(body)
	}

	return zone.Scan(body)
}

// categoryDesc maps content types to short descriptions for the welcome cards.
var categoryDesc = map[catalog.ContentType]string{
	catalog.Skills:   "Reusable skill definitions that extend AI tool capabilities",
	catalog.Agents:   "Agent configurations with specialized roles and personalities",
	catalog.Prompts:  "Prompt templates for common tasks and workflows",
	catalog.MCP:      "Model Context Protocol server configurations",
	catalog.Apps:     "Application scaffolds and project templates",
	catalog.Rules:    "Rule files that guide AI coding tool behavior",
	catalog.Hooks:    "Event-driven hooks for automation and validation",
	catalog.Commands: "Custom slash commands for AI coding tools",
}

// nescoFontRaw is the "nesco" text in ANSI Shadow style.
const nescoFontRaw = `░████████   ░███████   ░███████   ░███████   ░███████
░██    ░██ ░██    ░██ ░██        ░██    ░██ ░██    ░██
░██    ░██ ░█████████  ░███████  ░██        ░██    ░██
░██    ░██ ░██               ░██ ░██    ░██ ░██    ░██
░██    ░██  ░███████   ░███████   ░███████   ░███████`

// nescoPlantRaw is the nesco fractal plant ASCII art.
const nescoPlantRaw = `
                                     ##+
                                    ###+#
                                  ###++++++      ######
                                 ###++++++++  ##########
                      ++++++-   ###++++++++#############
                     ++++++-...##++++++++#########+++++++
                     +++++--..##+++++++#######++++++++#+++++++++#+
                     +++++--.#+++++++######+++++#++++++++++++++++++
          #######+++#+++++-.###+++++#####++++#+++++++++++++++++++++
          #######+++#++++--.#++++++#####++#++++++++---------------+
          #######+++#++++--.#+++++###+++++++++------.............-
          ########+++++++--##++++####+#+++----##################..
           #######++++++++-##+++###+++++--##+++++++++++++++++########
           #######+++#++++-##+++####++++##++++++++++++++++++++++++#+####
            #######+++++++-##++###+++-#+++###############+++++++++++++#####
             #######++#++++.#++###++-++######+++############++++++++++++####
            ++#######++#+++-#++##+++#+###++++++####++##########++++++++++++
          +++++#######++++++-#++#++++###+-+++++++++###++#########++++++++#
        ####++++#######++#+++-#+##+++#+#++##+##++++++####++########+++++
       ######+++++######++++++-#++#+++#++##++#+#-++++++####++########-+
       ########+++++######+++++++#++########++#+##+++++++###+++#######
        #########+++++#######++#+++++++++###++##+##-++++++###+++######
          +#########++++########+#++++#####+++##+##--++++++###+++###
             ##########+#+++##############-+++##++##-+++++++###++#
              .-############+#+++++######+++++##++##-++++++++###
             ---....##################+-+++++##++###-++++++++####
             --------...-#########---++++#++###++###--++++++++####
            ++++++---+-----------++++++#++###+++###--++++++++####
            +++++++++++++++++++++++++#++++###++++###--++++++++#####
            #++++++++++++++++++++##+++++####++++###---+++++++++####
               ####+++++++++##++++++++#####++++####--++++++++++####
                     ++++++++++++++######+++++####.--+++++++++#+#
                      ++++++++#########++++++####.---+++++
                      ###############++++++#####----++++++
                      ############+++++++######.---++++++
                       ########    ++++++#####    +++++++
                        ##          +#######+
                                      #####
                                       ##`

// trimArt removes common leading whitespace from multi-line ASCII art.
func trimArt(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	minIndent := len(s)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent < minIndent {
			minIndent = indent
		}
	}
	var result []string
	for _, line := range lines {
		if len(line) > minIndent {
			result = append(result, strings.TrimRight(line[minIndent:], " "))
		} else {
			result = append(result, "")
		}
	}
	return strings.Join(result, "\n")
}

// renderContentWelcome returns the landing page with centered ASCII art and category cards.
// Uses bordered cards when the terminal is tall enough (height >= 35), falling back to
// a compact single-line list when cards would be truncated.
func (a App) renderContentWelcome() string {
	contentW := a.width - sidebarWidth - 1
	if contentW < 30 {
		contentW = 30
	}

	var s string

	// Status message (from import, etc.)
	if a.statusMessage != "" {
		s += successMsgStyle.Render("Done: "+a.statusMessage) + "\n"
		for _, w := range a.statusWarnings {
			s += warningStyle.Render("Warning: "+w) + "\n"
		}
		s += "\n"
	}

	allTypes := catalog.AllContentTypes()
	counts := make(map[catalog.ContentType]int)
	if a.catalog != nil {
		counts = a.catalog.CountByType()
	}

	configItems := []struct {
		label  string
		desc   string
		zoneID string
	}{
		{"Import", "Import your own AI tools from local files or git repos", "welcome-import"},
		{"Update", "Check for updates and pull latest changes", "welcome-update"},
		{"Settings", "Configure paths and providers", "welcome-settings"},
	}

	// Cards need ~33 lines of panel height. With ASCII art (+7), ~40 total.
	// panelHeight = height - 2, so cards fit at height >= 35, art+cards at >= 42.
	useCards := a.height >= 35

	// --- ASCII art title (only when cards are showing and terminal is large) ---
	if useCards && a.height >= 42 && contentW >= 55 {
		font := trimArt(nescoFontRaw)
		s += lipgloss.PlaceHorizontal(contentW, lipgloss.Center, titleStyle.Render(font))
		s += "\n\n"
	}

	if useCards {
		s += a.renderWelcomeCards(allTypes, counts, contentW, configItems)
	} else {
		s += a.renderWelcomeList(allTypes, counts, contentW, configItems)
	}

	return s
}

// renderWelcomeCards renders the category and config sections as bordered cards.
func (a App) renderWelcomeCards(allTypes []catalog.ContentType, counts map[catalog.ContentType]int, contentW int, configItems []struct {
	label  string
	desc   string
	zoneID string
}) string {
	var s string

	singleCol := contentW < 42
	cardW := (contentW - 5) / 2 // 5 = 2 borders left + 2 borders right + 1 gap
	if singleCol {
		cardW = contentW - 2
	}
	if cardW < 18 {
		cardW = 18
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(cardW).
		Padding(0, 1)
	if !singleCol {
		cardStyle = cardStyle.Height(3)
	}

	if singleCol {
		for i, ct := range allTypes {
			inner := renderCategoryCardInner(ct, counts[ct])
			s += zone.Mark(fmt.Sprintf("welcome-%d", i), cardStyle.Render(inner)) + "\n"
		}
	} else {
		for i := 0; i < len(allTypes); i += 2 {
			left := renderCategoryCardInner(allTypes[i], counts[allTypes[i]])
			left = zone.Mark(fmt.Sprintf("welcome-%d", i), cardStyle.Render(left))

			var right string
			if i+1 < len(allTypes) {
				right = renderCategoryCardInner(allTypes[i+1], counts[allTypes[i+1]])
				right = zone.Mark(fmt.Sprintf("welcome-%d", i+1), cardStyle.Render(right))
			}

			s += lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n"
		}
	}

	// Configuration section
	s += "\n"
	s += labelStyle.Render("  Configuration") + "\n\n"

	singleColConfig := contentW < 56
	configCardW := (contentW - 7) / 3
	if singleColConfig {
		configCardW = contentW - 2
	}
	if configCardW < 16 {
		configCardW = 16
	}
	configCardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(configCardW).
		Padding(0, 1)
	if !singleColConfig {
		configCardStyle = configCardStyle.Height(3)
	}

	if singleColConfig {
		for _, ci := range configItems {
			inner := labelStyle.Render(ci.label) + "\n" + helpStyle.Render(ci.desc)
			s += zone.Mark(ci.zoneID, configCardStyle.Render(inner)) + "\n"
		}
	} else {
		var configCards []string
		for _, ci := range configItems {
			inner := labelStyle.Render(ci.label) + "\n" + helpStyle.Render(ci.desc)
			configCards = append(configCards, zone.Mark(ci.zoneID, configCardStyle.Render(inner)))
		}
		s += lipgloss.JoinHorizontal(lipgloss.Top, configCards[0], " ", configCards[1], " ", configCards[2]) + "\n"
	}

	return s
}

// renderWelcomeList renders a compact text list when cards won't fit.
func (a App) renderWelcomeList(allTypes []catalog.ContentType, counts map[catalog.ContentType]int, contentW int, configItems []struct {
	label  string
	desc   string
	zoneID string
}) string {
	var s string

	s += labelStyle.Render("  AI Tools") + "\n"
	for i, ct := range allTypes {
		line := renderCategoryLine(ct, counts[ct], contentW)
		s += zone.Mark(fmt.Sprintf("welcome-%d", i), line) + "\n"
	}

	s += "\n"
	s += labelStyle.Render("  Configuration") + "\n"

	for _, ci := range configItems {
		line := fmt.Sprintf("  %-12s - %s", ci.label, helpStyle.Render(ci.desc))
		if len(line) > contentW {
			line = line[:contentW]
		}
		s += zone.Mark(ci.zoneID, line) + "\n"
	}

	return s
}

// renderCategoryCardInner builds the inner content for a single category card.
func renderCategoryCardInner(ct catalog.ContentType, count int) string {
	title := labelStyle.Render(ct.Label()) + " " + countStyle.Render(fmt.Sprintf("(%d)", count))
	desc := categoryDesc[ct]
	if desc == "" {
		desc = "Browse " + ct.Label() + " content"
	}
	return title + "\n" + helpStyle.Render(desc)
}

// renderCategoryLine builds a compact single-line row for a category.
func renderCategoryLine(ct catalog.ContentType, count int, maxW int) string {
	label := ct.Label()
	desc := categoryDesc[ct]
	if desc == "" {
		desc = "Browse " + ct.Label() + " content"
	}
	line := fmt.Sprintf("  %-10s %s %s", label, countStyle.Render(fmt.Sprintf("%2d", count)), "- "+helpStyle.Render(desc))
	if len(line) > maxW {
		line = line[:maxW]
	}
	return line
}

// renderFooter builds the breadcrumb + context-sensitive help bar.
func (a App) renderFooter() string {
	crumb := a.breadcrumb()
	var helpText string
	switch a.screen {
	case screenDetail:
		helpText = "Esc: back   Tab: switch tab   ?: help   q: quit"
	case screenItems:
		helpText = "/: search   Enter: detail   Esc: sidebar   ?: help   q: quit"
	default:
		helpText = "Tab: switch panel   /: search   ?: help   q: quit"
	}

	// Pad the gap between help text and breadcrumb so crumb is right-aligned
	gap := a.width - len(helpText) - len(crumb)
	if gap < 1 {
		gap = 1
	}
	line := helpText + strings.Repeat(" ", gap) + crumb

	// Apply footer style (muted color + top border) to the full-width line
	return footerStyle.Width(a.width).Render(line)
}

// breadcrumb returns a "Category > Item" navigation string for the current screen.
func (a App) breadcrumb() string {
	switch a.screen {
	case screenDetail:
		return a.sidebar.selectedType().Label() + " > " + displayName(a.detail.item)
	case screenItems:
		return a.sidebar.selectedType().Label()
	case screenImport:
		return "Import"
	case screenUpdate:
		return "Update"
	case screenSettings:
		return "Settings"
	default:
		return "nesco"
	}
}
