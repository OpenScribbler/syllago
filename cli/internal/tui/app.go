package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
	"github.com/holdenhewett/romanesco/cli/internal/scan"
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

// App is the root bubbletea model.
type App struct {
	catalog    *catalog.Catalog
	providers  []provider.Provider
	detectors  []scan.Detector
	version    string
	autoUpdate bool

	screen      screen
	category    categoryModel
	items       itemsModel
	detail      detailModel
	search      searchModel
	helpOverlay helpOverlayModel
	importer    importModel
	updater     updateModel
	settings    settingsModel

	// Update check state (persists across screen changes)
	remoteVersion string
	commitsBehind int

	width    int
	height   int
	tooSmall bool // true when terminal is below minimum usable size
}

// NewApp creates a new App model. Set autoUpdate to true to pull updates
// automatically when a newer version is detected on origin.
func NewApp(cat *catalog.Catalog, providers []provider.Provider, detectors []scan.Detector, version string, autoUpdate bool) App {
	return App{
		catalog:    cat,
		providers:  providers,
		detectors:  detectors,
		version:    version,
		autoUpdate: autoUpdate,
		screen:     screenCategory,
		category:   newCategoryModel(cat, version),
		search:     newSearchModel(),
	}
}

func (a App) Init() tea.Cmd {
	return checkForUpdate(a.catalog.RepoRoot, a.version)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.tooSmall = msg.Width < 40 || msg.Height < 10
		a.items.width = msg.Width
		a.items.height = msg.Height
		a.detail.width = msg.Width
		a.detail.height = msg.Height
		a.detail.clampScroll()
		a.importer.width = msg.Width
		a.importer.height = msg.Height
		a.updater.width = msg.Width
		a.updater.height = msg.Height
		a.settings.width = msg.Width
		a.settings.height = msg.Height
		return a, nil

	case appInstallDoneMsg:
		if a.screen == screenDetail {
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd
		}

	case promoteDoneMsg:
		if a.screen == screenDetail {
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			// Rescan catalog after promote
			if msg.err == nil {
				cat, err := catalog.Scan(a.catalog.RepoRoot)
				if err == nil {
					a.catalog = cat
					a.category.counts = cat.CountByType()
					a.category.localCount = cat.CountLocal()
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
		if msg.err != nil {
			a.importer.message = fmt.Sprintf("Import failed: %s", msg.err)
			a.importer.messageIsErr = true
			return a, nil
		}
		// Rescan catalog
		cat, err := catalog.Scan(a.catalog.RepoRoot)
		if err == nil {
			a.catalog = cat
			a.category.message = fmt.Sprintf("Imported %q successfully", msg.name)
		} else {
			a.category.message = fmt.Sprintf("Imported %q but catalog rescan failed: %s", msg.name, err)
		}
		a.category.counts = a.catalog.CountByType()
		a.category.localCount = a.catalog.CountLocal()
		a.screen = screenCategory
		a.importer.cleanup()
		return a, nil

	case updateCheckMsg:
		if msg.err == nil && msg.remoteVersion != "" && versionNewer(msg.remoteVersion, msg.localVersion) {
			a.remoteVersion = msg.remoteVersion
			a.commitsBehind = msg.commitsBehind
			a.category.remoteVersion = msg.remoteVersion
			a.category.updateAvailable = true
			a.category.commitsBehind = msg.commitsBehind

			if a.autoUpdate {
				a.updater = newUpdateModel(a.catalog.RepoRoot, a.version, a.remoteVersion, a.commitsBehind)
				a.updater.width = a.width
				a.updater.height = a.height
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
					a.category.counts = cat.CountByType()
					a.category.localCount = cat.CountLocal()
				}
			}
			return a, cmd
		}

	case tea.KeyMsg:
		// ctrl+c always quits from any screen
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		// q quits from category, navigates back from other screens
		if key.Matches(msg, keys.Quit) && !a.search.active {
			if a.screen == screenCategory {
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
					ct := a.category.selectedType()
					items := newItemsModel(ct, a.catalog.ByType(ct), a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.height
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
					items.height = a.height
					a.items = items
					a.screen = screenItems
				} else if a.screen == screenItems {
					ct := a.category.selectedType()
					filtered := filterItems(a.catalog.ByType(ct), a.search.query())
					items := newItemsModel(ct, filtered, a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.height
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
				items.height = a.height
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

		// Screen-specific key handling
		switch a.screen {
		case screenCategory:
			if key.Matches(msg, keys.Enter) {
				if a.category.isUpdateSelected() {
					a.updater = newUpdateModel(a.catalog.RepoRoot, a.version, a.remoteVersion, a.commitsBehind)
					a.updater.width = a.width
					a.updater.height = a.height
					a.screen = screenUpdate
					return a, nil
				}
				if a.category.isSettingsSelected() {
					a.settings = newSettingsModel(a.catalog.RepoRoot, a.providers, a.detectors)
					a.settings.width = a.width
					a.settings.height = a.height
					a.screen = screenSettings
					return a, nil
				}
				if a.category.isImportSelected() {
					a.importer = newImportModel(a.providers, a.catalog.RepoRoot)
					a.importer.width = a.width
					a.importer.height = a.height
					a.screen = screenImport
					return a, nil
				}
				if a.category.isMyToolsSelected() {
					var localItems []catalog.ContentItem
					for _, item := range a.catalog.Items {
						if item.Local {
							localItems = append(localItems, item)
						}
					}
					items := newItemsModel(catalog.MyTools, localItems, a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.height
					a.items = items
					a.screen = screenItems
					return a, nil
				}
				ct := a.category.selectedType()
				items := newItemsModel(ct, a.catalog.ByType(ct), a.providers, a.catalog.RepoRoot)
				items.width = a.width
				items.height = a.height
				a.items = items
				a.screen = screenItems
				return a, nil
			}
			var cmd tea.Cmd
			a.category, cmd = a.category.Update(msg)
			return a, cmd

		case screenItems:
			if key.Matches(msg, keys.Back) {
				a.screen = screenCategory
				// Refresh counts in case installs changed
				a.category.counts = a.catalog.CountByType()
				a.category.localCount = a.catalog.CountLocal()
				return a, nil
			}
			if key.Matches(msg, keys.Enter) && len(a.items.items) > 0 {
				item := a.items.selectedItem()
				a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
				a.detail.width = a.width
				a.detail.height = a.height
				a.screen = screenDetail
				return a, nil
			}
			var cmd tea.Cmd
			a.items, cmd = a.items.Update(msg)
			return a, cmd

		case screenDetail:
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
					items.height = a.height
					items.cursor = a.items.cursor
					a.items = items
				} else if a.items.contentType != catalog.SearchResults {
					ct := a.items.contentType
					items := newItemsModel(ct, a.catalog.ByType(ct), a.providers, a.catalog.RepoRoot)
					items.width = a.width
					items.height = a.height
					items.cursor = a.items.cursor // preserve cursor position
					a.items = items
				}
				a.screen = screenItems
				return a, nil
			}
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd

		case screenImport:
			if key.Matches(msg, keys.Back) && a.importer.step == stepSource {
				a.screen = screenCategory
				a.importer.cleanup()
				return a, nil
			}
			var cmd tea.Cmd
			a.importer, cmd = a.importer.Update(msg)
			return a, cmd

		case screenUpdate:
			if key.Matches(msg, keys.Back) && (a.updater.step == stepUpdateMenu || a.updater.step == stepUpdateDone) {
				a.screen = screenCategory
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
		return "\n" + warningStyle.Render("Terminal too small. Resize to at least 40x10.") + "\n"
	}

	var content string

	switch a.screen {
	case screenCategory:
		content = a.category.View()
	case screenItems:
		content = a.items.View()
	case screenDetail:
		content = a.detail.View()
	case screenImport:
		content = a.importer.View()
	case screenUpdate:
		content = a.updater.View()
	case screenSettings:
		content = a.settings.View()
	}

	// Help overlay replaces all content
	if a.helpOverlay.active {
		content = a.helpOverlay.View(a.screen)
	}

	// Overlay search if active (replaces the help bar)
	if a.search.active {
		lines := strings.Split(content, "\n")
		// Remove trailing empty lines and the help bar
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > 0 {
			lines = lines[:len(lines)-1] // remove help bar
		}
		content = strings.Join(lines, "\n")
		content += "\n" + a.search.View()
	}

	return fmt.Sprintf("\n%s\n", content)
}
