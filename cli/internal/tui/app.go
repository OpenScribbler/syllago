package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/promote"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

type screen int

const (
	screenCategory screen = iota
	screenItems
	screenDetail
	screenImport
	screenUpdate
	screenSettings
	screenRegistries
	screenSandbox
)

type focusTarget int

const (
	focusSidebar focusTarget = iota
	focusContent
	focusModal
)

// App is the root bubbletea model.
type App struct {
	catalog         *catalog.Catalog
	providers       []provider.Provider
	version         string
	autoUpdate      bool
	isReleaseBuild  bool
	registrySources []catalog.RegistrySource
	projectRoot     string

	screen          screen
	focus           focusTarget
	modal           confirmModal
	saveModal       saveModal
	envModal        envSetupModal
	instModal       installModal
	sidebar         sidebarModel
	items           itemsModel
	detail          detailModel
	search          searchModel
	helpOverlay     helpOverlayModel
	importer        importModel
	updater         updateModel
	settings        settingsModel
	registries      registriesModel
	sandboxSettings sandboxSettingsModel
	registryCfg     *config.Config
	statusMessage   string
	statusWarnings  []string

	// Detail model cache (preserves state when re-entering same item)
	cachedDetail     *detailModel
	cachedDetailPath string

	// Update check state (persists across screen changes)
	remoteVersion string
	commitsBehind int

	showHidden bool // when true, hidden items are included in lists

	// Loadout apply state
	loadoutApplyItem catalog.ContentItem // the loadout item being applied
	loadoutApplyMode string              // "preview", "try", or "keep"
	activeLoadout    string              // name of currently active loadout (empty if none)

	width    int
	height   int
	tooSmall bool // true when terminal is below minimum usable size
}

// NewApp creates a new App model. Set autoUpdate to true to pull updates
// automatically when a newer version is detected on origin. Set isReleaseBuild
// to true for release binaries so the updater uses GitHub Releases instead of git.
// projectRoot is the project root where local/ lives (may differ from cat.RepoRoot
// when contentRoot has been moved to a subdirectory like content/).
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string, autoUpdate bool, registrySources []catalog.RegistrySource, cfg *config.Config, isReleaseBuild bool, projectRoot string) App {
	if cfg == nil {
		cfg = &config.Config{}
	}
	app := App{
		catalog:         cat,
		providers:       providers,
		version:         version,
		autoUpdate:      autoUpdate,
		isReleaseBuild:  isReleaseBuild,
		registrySources: registrySources,
		projectRoot:     projectRoot,
		registryCfg:     cfg,
		screen:          screenCategory,
		focus:           focusSidebar,
		sidebar:         newSidebarModel(cat, version, len(cfg.Registries)),
		search:          newSearchModel(),
	}
	// Overwrite raw counts with visible-only counts (excludes hidden items)
	app.refreshSidebarCounts()
	return app
}

func (a App) Init() tea.Cmd {
	if a.isReleaseBuild {
		return checkForUpdateRelease(a.version)
	}
	return checkForUpdate(a.catalog.RepoRoot, a.version)
}

// handleConfirmAction dispatches the action for a confirmed modal based on its
// purpose. Called from both the keyboard and mouse input paths.
func (a *App) handleConfirmAction() tea.Cmd {
	if !a.modal.confirmed {
		return nil
	}
	switch a.modal.purpose {
	case modalUninstall:
		a.detail.doUninstallAll()
	case modalShare:
		repoRoot := a.detail.repoRoot
		item := a.detail.item
		return func() tea.Msg {
			result, err := promote.Promote(repoRoot, item, false)
			return shareDoneMsg{result: result, err: err}
		}
	case modalAppScript:
		return a.detail.runAppScript()
	case modalLoadoutApply:
		return a.runLoadoutApply(a.loadoutApplyItem, a.loadoutApplyMode)
	case modalHookBrokenWarning:
		return a.detail.startInstall()
	}
	return nil
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

// visibleItems filters out hidden items unless showHidden is true.
func (a App) visibleItems(src []catalog.ContentItem) []catalog.ContentItem {
	if a.showHidden {
		return src
	}
	var result []catalog.ContentItem
	for _, item := range src {
		if item.Meta != nil && item.Meta.Hidden {
			continue
		}
		result = append(result, item)
	}
	return result
}

// countHidden returns the number of hidden items in the slice.
func countHidden(items []catalog.ContentItem) int {
	count := 0
	for _, item := range items {
		if item.Meta != nil && item.Meta.Hidden {
			count++
		}
	}
	return count
}

// refreshSidebarCounts updates the sidebar counts to reflect the current
// showHidden state. This ensures counts match what the user actually sees.
func (a *App) refreshSidebarCounts() {
	counts := make(map[catalog.ContentType]int)
	for _, ct := range catalog.AllContentTypes() {
		counts[ct] = len(a.visibleItems(a.catalog.ByType(ct)))
	}
	a.sidebar.counts = counts
	a.sidebar.libraryCount = len(a.visibleItems(a.libraryItems()))
}

// libraryItems returns all library items from the catalog.
func (a App) libraryItems() []catalog.ContentItem {
	var result []catalog.ContentItem
	for _, item := range a.catalog.Items {
		if item.Library {
			result = append(result, item)
		}
	}
	return result
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
		a.registries.width = contentW
		a.registries.height = ph
		a.sandboxSettings.width = contentW
		a.sandboxSettings.height = ph
		return a, nil

	case appInstallDoneMsg:
		a.cachedDetail = nil // invalidate cache (install state changed)
		if a.screen == screenDetail {
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd
		}

	case shareDoneMsg:
		if a.screen == screenDetail {
			a.cachedDetail = nil // invalidate cache
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			// Rescan catalog after share
			if msg.err == nil {
				cat, err := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
				if err == nil {
					a.catalog = cat
					a.refreshSidebarCounts()
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
			a.importer.message = fmt.Sprintf("Add failed: %s", msg.err)
			a.importer.messageIsErr = true
			return a, nil
		}
		// Rescan catalog
		cat, err := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
		if err == nil {
			a.catalog = cat
			a.statusMessage = fmt.Sprintf("Added %q to library", msg.name)
		} else {
			a.statusMessage = fmt.Sprintf("Imported %q but catalog rescan failed: %s", msg.name, err)
		}
		a.statusWarnings = msg.warnings
		a.refreshSidebarCounts()
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
				a.updater = newUpdateModel(a.catalog.RepoRoot, a.version, a.remoteVersion, a.commitsBehind, a.isReleaseBuild)
				a.updater.width = a.width - sidebarWidth - 1
				a.updater.height = a.panelHeight()
				a.screen = screenUpdate
				return a, a.updater.startPull()
			}
		}
		// Forward to updater so it can clear loading state
		if a.screen == screenUpdate {
			a.updater, _ = a.updater.Update(msg)
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
				cat, err := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
				if err == nil {
					a.catalog = cat
					a.refreshSidebarCounts()
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

	case openInstallModalMsg:
		a.instModal = newInstallModal(msg.item, msg.providers, msg.repoRoot)
		a.focus = focusModal
		return a, nil

	case openHookBrokenWarningMsg:
		body := fmt.Sprintf("%s has broken compatibility:\n\n%s\n\nInstall anyway?", msg.providerName, msg.notes)
		a.modal = newConfirmModal("Compatibility Warning", body)
		a.modal.purpose = modalHookBrokenWarning
		a.focus = focusModal
		return a, nil

	case openLoadoutApplyMsg:
		a.loadoutApplyItem = msg.item
		a.loadoutApplyMode = msg.mode
		if msg.mode == "preview" {
			// Preview mode: run immediately without confirmation
			return a, a.runLoadoutApply(msg.item, msg.mode)
		}
		// Try/Keep modes: show confirmation modal first
		var body string
		if msg.mode == "try" {
			body = "This loadout is temporary. It will auto-revert when the session ends.\nIf auto-revert fails, run: syllago loadout remove\n\nApply?"
		} else {
			body = "This loadout will stay until you run: syllago loadout remove\n\nApply?"
		}
		a.modal = newConfirmModal(fmt.Sprintf("Apply %q (%s)?", msg.item.Name, msg.mode), body)
		a.modal.purpose = modalLoadoutApply
		a.focus = focusModal
		return a, nil

	case loadoutApplyDoneMsg:
		if msg.err != nil {
			a.detail.message = fmt.Sprintf("Apply failed: %s", msg.err)
			a.detail.messageIsErr = true
		} else {
			// Build success message from the actions
			var summary []string
			for _, action := range msg.result.Actions {
				summary = append(summary, fmt.Sprintf("%s %s: %s", action.Action, action.Name, action.Detail))
			}
			if msg.mode == "preview" {
				a.detail.message = fmt.Sprintf("Preview (%d actions):\n%s", len(msg.result.Actions), strings.Join(summary, "\n"))
			} else {
				a.detail.message = fmt.Sprintf("Applied (%s mode, %d actions)", msg.mode, len(msg.result.Actions))
				a.activeLoadout = a.loadoutApplyItem.Name
			}
			a.detail.messageIsErr = false
		}
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
		// Click-away: if any modal is active, check whether the click landed
		// inside the modal bounds (zone "modal-zone"). If inside, forward to
		// the modal's own handler. If outside, dismiss the modal.
		if a.modal.active {
			// Use the zone registered by the previous View()/zone.Scan() pass.
			// This gives us the actual on-screen position of the modal after
			// overlay compositing — no centering formula to replicate.
			z := zone.Get("modal-zone")
			if z.InBounds(msg) {
				relX, relY := z.Pos(msg)
				modalH := z.EndY - z.StartY + 1
				modalW := z.EndX - z.StartX + 1

				// Button row: last content line before bottom padding(1)+border(1)
				buttonRelY := modalH - 3
				if relY == buttonRelY {
					contentLeft := 3            // border(1) + padding(2)
					contentW := modalW - 6      // minus borders(2) and padding(4)
					midX := contentLeft + contentW/2
					if relX < midX {
						a.modal.btnCursor = 0 // Confirm
					} else {
						a.modal.btnCursor = 1 // Cancel
					}
					// Activate the clicked button
					a.modal.confirmed = a.modal.btnCursor == 0
					a.modal.active = false
					a.focus = focusContent
					if cmd := a.handleConfirmAction(); cmd != nil {
						return a, cmd
					}
					return a, nil
				}
				// Click inside modal but not on buttons — ignore
				return a, nil
			}
			// Click outside — dismiss
			a.modal.active = false
			a.modal.confirmed = false
			a.focus = focusContent
			return a, nil
		}
		if a.saveModal.active {
			if zone.Get("modal-zone").InBounds(msg) {
				return a, nil // click inside modal — ignore (save uses keys)
			}
			a.saveModal.active = false
			a.saveModal.confirmed = false
			a.focus = focusContent
			return a, nil
		}
		if a.envModal.active {
			if zone.Get("modal-zone").InBounds(msg) {
				return a, nil // click inside modal — ignore
			}
			a.envModal.active = false
			a.focus = focusContent
			return a, nil
		}
		if a.instModal.active {
			// Use the zone registered by the previous View()/zone.Scan() pass
			// for accurate hit-testing (inner zone marks don't survive overlay
			// compositing, but the outer "modal-zone" wrapping the whole
			// foreground does).
			z := zone.Get("modal-zone")
			if z.InBounds(msg) {
				relX, relY := z.Pos(msg)
				modalH := z.EndY - z.StartY + 1
				modalW := z.EndX - z.StartX + 1

				// Options start at relY=4: border(1)+padding(1)+title(1)+blank(1)
				optionRelY := 4

				// Button row: last content line before bottom padding(1)+border(1)
				buttonRelY := modalH - 3

				// Check button row click first
				if relY == buttonRelY {
					contentLeft := 3            // border(1) + padding(2)
					contentW := modalW - 6      // minus borders(2) and padding(4)
					midX := contentLeft + contentW/2
					if relX < midX {
						a.instModal.btnCursor = 0
					} else {
						a.instModal.btnCursor = 1
					}
					// Synthesize Enter to activate the clicked button
					enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
					var cmd tea.Cmd
					a.instModal, cmd = a.instModal.Update(enterMsg)
					if !a.instModal.active {
						a.focus = focusContent
						if a.instModal.confirmed {
							envCmd := a.detail.doInstallFromModal(a.instModal)
							a.resetInstallModal()
							if envCmd != nil {
								return a, envCmd
							}
						} else {
							a.resetInstallModal()
						}
					}
					return a, cmd
				}

				// Check option row clicks — highlight only, don't advance
				numOptions := 0
				switch a.instModal.step {
				case installStepLocation:
					numOptions = 3
				case installStepMethod:
					numOptions = 2
				}

				for i := 0; i < numOptions; i++ {
					rowTop := optionRelY + i*2
					if relY >= rowTop && relY < rowTop+2 {
						switch a.instModal.step {
						case installStepLocation:
							a.instModal.locationCursor = i
						case installStepMethod:
							a.instModal.methodCursor = i
						}
						return a, nil
					}
				}

				// Click inside modal but not on an option or button — ignore
				return a, nil
			}
			// Click outside modal — close it
			a.resetInstallModal()
			a.focus = focusContent
			return a, nil
		}
		// Check sidebar zones
		for i := 0; i < a.sidebar.totalItems(); i++ {
			if zone.Get(fmt.Sprintf("sidebar-%d", i)).InBounds(msg) {
				a.resetInstallModal() // cancel any in-progress install when navigating away
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
			"welcome-add":        len(a.sidebar.types) + 1,
			"welcome-update":     len(a.sidebar.types) + 2,
			"welcome-settings":   len(a.sidebar.types) + 3,
			"welcome-registries": len(a.sidebar.types) + 4,
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
			a.resetInstallModal() // cancel any in-progress install when navigating away
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
		if zone.Get("crumb-registries").InBounds(msg) {
			if a.screen == screenItems && a.items.sourceRegistry != "" {
				// Navigate back to registries screen
				a.screen = screenRegistries
				a.focus = focusContent
				return a, nil
			}
		}
		// Check detail tab zones
		if a.screen == screenDetail {
			tabs := []detailTab{tabOverview, tabFiles, tabInstall}
			for _, tab := range tabs {
				if zone.Get(fmt.Sprintf("tab-%d", int(tab))).InBounds(msg) {
					// Reset file viewer when switching away from Files tab
					if a.detail.activeTab == tabFiles && tab != tabFiles {
						a.detail.CancelAction()
					}
					a.detail.activeTab = tab
					return a, nil
				}
			}
		}
		// Check file list entry zones (Files tab)
		if a.screen == screenDetail && a.detail.activeTab == tabFiles {
			if a.detail.fileViewer.viewing {
				// Back link in file content view
				if zone.Get("file-back").InBounds(msg) {
					a.detail.CancelAction()
					return a, nil
				}
			} else {
				// File list entries — click to open
				for i := range a.detail.item.Files {
					if zone.Get(fmt.Sprintf("file-%d", i)).InBounds(msg) {
						a.detail.fileViewer.cursor = i
						return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
					}
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
		// Check provider checkbox zones (Install tab)
		if a.screen == screenDetail && a.detail.activeTab == tabInstall {
			detected := a.detail.detectedProviders()
			for i := range detected {
				if zone.Get(fmt.Sprintf("prov-check-%d", i)).InBounds(msg) {
					a.detail.provCheck.cursor = i
					a.detail.provCheck.checks[i] = !a.detail.provCheck.checks[i]
					return a, nil
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
		case screenRegistries:
			var cmd tea.Cmd
			a.registries, cmd = a.registries.Update(msg)
			return a, cmd
		case screenSandbox:
			var cmd tea.Cmd
			a.sandboxSettings, cmd = a.sandboxSettings.Update(msg)
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
				if actionCmd := a.handleConfirmAction(); actionCmd != nil {
					return a, actionCmd
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
		if a.instModal.active {
			var cmd tea.Cmd
			a.instModal, cmd = a.instModal.Update(msg)
			if !a.instModal.active {
				a.focus = focusContent
				if a.instModal.confirmed {
					envCmd := a.detail.doInstallFromModal(a.instModal)
					a.resetInstallModal()
					if envCmd != nil {
						return a, envCmd
					}
				} else {
					a.resetInstallModal() // prevent ghost installs from stale state
				}
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
		if key.Matches(msg, keys.Search) && !a.search.active && a.screen != screenImport && a.screen != screenUpdate && a.screen != screenSettings && a.screen != screenRegistries && a.screen != screenSandbox && !a.detail.HasTextInput() {
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
					src := a.visibleItems(a.catalog.ByType(ct))
					items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
					items.hiddenCount = countHidden(a.catalog.ByType(ct))
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
					filtered := filterItems(a.visibleItems(a.catalog.Items), a.search.query())
					items := newItemsModel(catalog.SearchResults, filtered, a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.panelHeight()
					a.items = items
					a.screen = screenItems
				} else if a.screen == screenItems {
					ct := a.sidebar.selectedType()
					filtered := filterItems(a.visibleItems(a.catalog.ByType(ct)), a.search.query())
					items := newItemsModel(ct, filtered, a.providers, a.catalog.RepoRoot)
					items.hiddenCount = countHidden(a.catalog.ByType(ct))
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
					source = a.visibleItems(a.catalog.Items)
				} else if ct == catalog.Library {
					for _, item := range a.visibleItems(a.catalog.Items) {
						if item.Library {
							source = append(source, item)
						}
					}
				} else {
					source = a.visibleItems(a.catalog.ByType(ct))
				}
				filtered := filterItems(source, query)
				items := newItemsModel(ct, filtered, a.providers, a.catalog.RepoRoot)
				items.hiddenCount = countHidden(a.catalog.ByType(ct))
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
			if a.screen != screenImport && a.screen != screenUpdate && a.screen != screenSettings && a.screen != screenRegistries && a.screen != screenSandbox {
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
			if key.Matches(msg, keys.ToggleHidden) {
				a.showHidden = !a.showHidden
				a.refreshSidebarCounts()
				return a, nil
			}
			if a.focus == focusSidebar {
				if key.Matches(msg, keys.Enter) || key.Matches(msg, keys.Right) {
					if a.sidebar.isUpdateSelected() {
						a.updater = newUpdateModel(a.catalog.RepoRoot, a.version, a.remoteVersion, a.commitsBehind, a.isReleaseBuild)
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
					if a.sidebar.isAddSelected() {
						a.importer = newImportModel(a.providers, a.catalog.RepoRoot, a.projectRoot)
						a.importer.width = a.width - sidebarWidth - 1
						a.importer.height = a.panelHeight()
						a.screen = screenImport
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isRegistriesSelected() {
						a.registries = newRegistriesModel(a.catalog.RepoRoot, a.registryCfg, a.catalog)
						a.registries.width = a.width - sidebarWidth - 1
						a.registries.height = a.panelHeight()
						a.screen = screenRegistries
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isSandboxSelected() {
						a.sandboxSettings = newSandboxSettingsModel(a.catalog.RepoRoot)
						a.sandboxSettings.width = a.width - sidebarWidth - 1
						a.sandboxSettings.height = a.panelHeight()
						a.screen = screenSandbox
						a.focus = focusContent
						return a, nil
					}
					if a.sidebar.isLibrarySelected() {
						var localItems []catalog.ContentItem
						for _, item := range a.visibleItems(a.catalog.Items) {
							if item.Library {
								localItems = append(localItems, item)
							}
						}
						items := newItemsModel(catalog.Library, localItems, a.providers, a.catalog.RepoRoot)
						items.hiddenCount = countHidden(a.catalog.Items)
						items.width = a.width - sidebarWidth - 1
						items.height = a.panelHeight()
						a.items = items
						a.screen = screenItems
						a.focus = focusContent
						return a, nil
					}
					ct := a.sidebar.selectedType()
					src := a.visibleItems(a.catalog.ByType(ct))
					items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
					items.hiddenCount = countHidden(a.catalog.ByType(ct))
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
				if a.items.sourceRegistry != "" {
					// Came from registry drill-in — go back to registries
					a.screen = screenRegistries
					a.focus = focusContent
					return a, nil
				}
				a.screen = screenCategory
				a.focus = focusSidebar
				if a.catalog != nil {
					a.refreshSidebarCounts()
				}
				return a, nil
			}
			if key.Matches(msg, keys.ToggleHidden) {
				a.showHidden = !a.showHidden
				a.refreshSidebarCounts()
				ct := a.items.contentType
				if ct == catalog.Library {
					var localItems []catalog.ContentItem
					for _, item := range a.visibleItems(a.catalog.Items) {
						if item.Library {
							localItems = append(localItems, item)
						}
					}
					items := newItemsModel(catalog.Library, localItems, a.providers, a.catalog.RepoRoot)
					items.hiddenCount = countHidden(a.catalog.Items)
					items.width = a.items.width
					items.height = a.items.height
					a.items = items
				} else if ct != catalog.SearchResults {
					src := a.visibleItems(a.catalog.ByType(ct))
					items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
					items.hiddenCount = countHidden(a.catalog.ByType(ct))
					items.width = a.items.width
					items.height = a.items.height
					a.items = items
				}
				return a, nil
			}
			if key.Matches(msg, keys.Enter) && len(a.items.items) > 0 {
				item := a.items.selectedItem()
				if a.cachedDetailPath == item.Path && a.cachedDetail != nil {
					a.detail = *a.cachedDetail
				} else {
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
					a.detail.overrides = a.catalog.OverridesFor(item.Name, item.Type)
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
			if msg.String() == "ctrl+n" && !a.detail.HasTextInput() {
				if a.items.cursor < len(a.items.items)-1 {
					a.items.cursor++
					item := a.items.selectedItem()
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
					a.detail.overrides = a.catalog.OverridesFor(item.Name, item.Type)
					a.detail.width = a.width
					a.detail.height = a.panelHeight()
					a.detail.listPosition = a.items.cursor
					a.detail.listTotal = len(a.items.items)
				}
				return a, nil
			}
			if msg.String() == "ctrl+p" && !a.detail.HasTextInput() {
				if a.items.cursor > 0 {
					a.items.cursor--
					item := a.items.selectedItem()
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
					a.detail.overrides = a.catalog.OverridesFor(item.Name, item.Type)
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
				if a.items.contentType == catalog.Library {
					var localItems []catalog.ContentItem
					for _, item := range a.visibleItems(a.catalog.Items) {
						if item.Library {
							localItems = append(localItems, item)
						}
					}
					items := newItemsModel(catalog.Library, localItems, a.providers, a.catalog.RepoRoot)
					items.hiddenCount = countHidden(a.catalog.Items)
					items.width = a.width
					items.height = a.panelHeight()
					items.cursor = a.items.cursor
					a.items = items
				} else if a.items.contentType != catalog.SearchResults {
					ct := a.items.contentType
					src := a.visibleItems(a.catalog.ByType(ct))
					items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
					items.hiddenCount = countHidden(a.catalog.ByType(ct))
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

		case screenSandbox:
			if key.Matches(msg, keys.Back) {
				if a.sandboxSettings.editMode != 0 {
					a.sandboxSettings.editMode = 0
					a.sandboxSettings.editInput = ""
					return a, nil
				}
				a.screen = screenCategory
				a.focus = focusSidebar
				return a, nil
			}
			var cmd tea.Cmd
			a.sandboxSettings, cmd = a.sandboxSettings.Update(msg)
			return a, cmd

		case screenRegistries:
			if key.Matches(msg, keys.Back) {
				a.screen = screenCategory
				a.focus = focusSidebar
				return a, nil
			}
			// Enter on a registry → show its items
			if key.Matches(msg, keys.Enter) && len(a.registries.entries) > 0 {
				name := a.registries.selectedName()
				if name != "" {
					regItems := a.visibleItems(a.catalog.ByRegistry(name))
					items := newItemsModel(catalog.SearchResults, regItems, a.providers, a.catalog.RepoRoot)
					items.sourceRegistry = name
					items.width = a.width - sidebarWidth - 1
					items.height = a.panelHeight()
					a.items = items
					a.screen = screenItems
					a.focus = focusContent
					return a, nil
				}
			}
			var cmd tea.Cmd
			a.registries, cmd = a.registries.Update(msg)
			return a, cmd
		}
	}

	return a, nil
}

// resetInstallModal clears all install modal state. Called whenever the install
// modal is dismissed (cancelled or confirmed) or the user navigates away from
// the detail screen while the modal might be open, preventing ghost installs.
func (a *App) resetInstallModal() {
	a.instModal = installModal{}
}

// runLoadoutApply runs a loadout apply operation as a background command.
// The loadout engine handles parsing, resolving, validating, and applying.
// Results are sent back as a loadoutApplyDoneMsg.
func (a App) runLoadoutApply(item catalog.ContentItem, mode string) tea.Cmd {
	cat := a.catalog
	providers := a.providers
	projectRoot := a.projectRoot
	return func() tea.Msg {
		manifest, err := loadout.Parse(filepath.Join(item.Path, "loadout.yaml"))
		if err != nil {
			return loadoutApplyDoneMsg{err: err, mode: mode}
		}

		// Find the target provider by matching the manifest's provider field
		var targetProv provider.Provider
		found := false
		for _, p := range providers {
			if p.Slug == manifest.Provider && p.Detected {
				targetProv = p
				found = true
				break
			}
		}
		if !found {
			return loadoutApplyDoneMsg{
				err:  fmt.Errorf("provider %q not detected", manifest.Provider),
				mode: mode,
			}
		}

		result, err := loadout.Apply(manifest, cat, targetProv, loadout.ApplyOptions{
			Mode:        mode,
			ProjectRoot: projectRoot,
			RepoRoot:    cat.RepoRoot,
		})
		return loadoutApplyDoneMsg{result: result, err: err, mode: mode}
	}
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
	case screenRegistries:
		contentView = a.registries.View()
	case screenSandbox:
		contentView = a.sandboxSettings.View()
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

	if a.instModal.active {
		body = a.instModal.overlayView(body)
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
	catalog.Loadouts: "Curated content bundles for quick session setup",
}

// syllagoFontRaw is the "Syllago" text in block letter style.
const syllagoFontRaw = ` █████████             ████  ████
███░░░░░███           ░░███ ░░███
░███    ░░░  █████ ████ ░███  ░███   ██████    ███████  ██████
░░█████████ ░░███ ░███  ░███  ░███  ░░░░░███  ███░░███ ███░░███
 ░░░░░░░░███ ░███ ░███  ░███  ░███   ███████ ░███ ░███░███ ░███
 ███    ░███ ░███ ░███  ░███  ░███  ███░░███ ░███ ░███░███ ░███
░░█████████  ░░███████  █████ █████░░████████░░███████░░██████
 ░░░░░░░░░    ░░░░░███ ░░░░░ ░░░░░  ░░░░░░░░  ░░░░░███ ░░░░░░
              ███ ░███                        ███ ░███
             ░░██████                        ░░██████
              ░░░░░░                          ░░░░░░`

// syllagoPlantRaw is the syllago fractal plant ASCII art.
const syllagoPlantRaw = `
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

// renderFirstRun returns a getting-started guide shown when the catalog is empty
// and no registries are configured. This is the zero-state experience for new users.
func (a App) renderFirstRun(contentW int) string {
	var s string

	s += titleStyle.Render("Welcome to syllago!") + "\n\n"
	s += helpStyle.Render("No content found. Here's how to get started:") + "\n\n"

	steps := []struct {
		num  string
		head string
		cmd  string
	}{
		{"1.", "Add existing content:", "syllago add --from claude-code"},
		{"2.", "Add a registry:", "syllago registry add <git-url>"},
		{"3.", "Create new content:", "syllago create skill my-first-skill"},
		{"4.", "Create a registry:", "syllago registry create my-registry"},
	}

	for _, step := range steps {
		s += labelStyle.Render(step.num) + " " + valueStyle.Render(step.head) + "\n"
		s += "   " + helpStyle.Render(step.cmd) + "\n\n"
	}

	s += helpStyle.Render("Press ? for help, q to quit.") + "\n"
	return s
}

// renderContentWelcome returns the landing page with centered ASCII art and category cards.
// Uses bordered cards when the terminal is tall enough (height >= 35), falling back to
// a compact single-line list when cards would be truncated.
// When the catalog is empty and no registries are configured, shows the first-run screen.
func (a App) renderContentWelcome() string {
	contentW := a.width - sidebarWidth - 1
	if contentW < 30 {
		contentW = 30
	}

	// First-run: no content and no registries — show getting-started guide
	if (a.catalog == nil || len(a.catalog.Items) == 0) &&
		(a.registryCfg == nil || len(a.registryCfg.Registries) == 0) {
		return a.renderFirstRun(contentW)
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
	// Use sidebar counts (already filtered by showHidden state)
	counts := a.sidebar.counts
	if counts == nil {
		counts = make(map[catalog.ContentType]int)
	}

	configItems := []struct {
		label  string
		desc   string
		zoneID string
	}{
		{"Add", "Add content from providers, local files, or git repos", "welcome-add"},
		{"Update", "Check for updates and pull latest changes", "welcome-update"},
		{"Settings", "Configure paths and providers", "welcome-settings"},
		{"Registries", "Manage git-based content sources from your team or organization", "welcome-registries"},
	}

	// Cards need ~33 lines of panel height. With ASCII art (+13), ~46 total.
	// panelHeight = height - 2, so cards fit at height >= 35, art+cards at >= 48.
	useCards := a.height >= 35

	// --- ASCII art title (only when cards are showing and terminal is large) ---
	if useCards && a.height >= 48 && contentW >= 75 {
		font := trimArt(syllagoFontRaw)
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
	configCardW := (contentW - 5) / 2
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
		// Render config cards in rows of 2
		for i := 0; i < len(configItems); i += 2 {
			leftInner := labelStyle.Render(configItems[i].label) + "\n" + helpStyle.Render(configItems[i].desc)
			left := zone.Mark(configItems[i].zoneID, configCardStyle.Render(leftInner))
			if i+1 < len(configItems) {
				rightInner := labelStyle.Render(configItems[i+1].label) + "\n" + helpStyle.Render(configItems[i+1].desc)
				right := zone.Mark(configItems[i+1].zoneID, configCardStyle.Render(rightInner))
				s += lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n"
			} else {
				s += left + "\n"
			}
		}
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

// contextHelpText returns the appropriate help text for the current screen state.
func (a App) contextHelpText() string {
	switch a.screen {
	case screenCategory:
		return "tab switch panel • / search • ? help • q quit"
	case screenDetail:
		return a.detail.renderHelp()
	case screenItems:
		return "/ search • enter detail • esc back • ? help"
	case screenRegistries:
		return a.registries.helpText()
	case screenSettings:
		return a.settings.helpText()
	case screenImport:
		return a.importer.helpText()
	case screenUpdate:
		return a.updater.helpText()
	case screenSandbox:
		return a.sandboxSettings.helpText()
	default:
		return "esc back • ? help"
	}
}

// renderFooter builds the breadcrumb + context-sensitive help bar.
func (a App) renderFooter() string {
	crumb := a.breadcrumb()
	helpText := a.contextHelpText()

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
		if a.items.sourceRegistry != "" {
			return "Registries > " + a.items.sourceRegistry
		}
		return a.sidebar.selectedType().Label()
	case screenImport:
		return "Add"
	case screenUpdate:
		return "Update"
	case screenSettings:
		return "Settings"
	case screenRegistries:
		return "Registries"
	case screenSandbox:
		return "Sandbox"
	default:
		return "syllago"
	}
}
