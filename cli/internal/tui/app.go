package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
)

type screen int

const (
	screenCategory screen = iota
	screenItems
	screenDetail
	screenImport
	screenUpdate
)

// App is the root bubbletea model.
type App struct {
	catalog   *catalog.Catalog
	providers []provider.Provider
	version   string

	screen   screen
	category categoryModel
	items    itemsModel
	detail   detailModel
	search   searchModel
	importer importModel
	updater  updateModel

	// Update check state (persists across screen changes)
	remoteVersion string
	commitsBehind int

	width    int
	height   int
	tooSmall bool // true when terminal is below minimum usable size
}

// NewApp creates a new App model.
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string) App {
	return App{
		catalog:   cat,
		providers: providers,
		version:   version,
		screen:    screenCategory,
		category:  newCategoryModel(cat, version),
		search:    newSearchModel(),
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
		if msg.err == nil && msg.remoteVersion != "" && msg.remoteVersion != msg.localVersion {
			a.remoteVersion = msg.remoteVersion
			a.commitsBehind = msg.commitsBehind
			a.category.remoteVersion = msg.remoteVersion
			a.category.updateAvailable = true
			a.category.commitsBehind = msg.commitsBehind
		}
		return a, nil

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
		// q quits from category screen only (when not searching)
		if key.Matches(msg, keys.Quit) && !a.search.active && a.screen == screenCategory {
			return a, tea.Quit
		}

		// Search toggle (skip on import/update screens and when detail has active textinput)
		if key.Matches(msg, keys.Search) && !a.search.active && a.screen != screenImport && a.screen != screenUpdate && !a.detail.HasTextInput() {
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
		}
	}

	return a, nil
}

func (a App) View() string {
	if a.tooSmall {
		return "\n" + helpStyle.Render("Terminal too small. Resize to at least 40×10.") + "\n"
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
	}

	// Overlay search if active
	if a.search.active {
		content += "\n" + a.search.View()
	}

	return fmt.Sprintf("\n%s\n", content)
}
