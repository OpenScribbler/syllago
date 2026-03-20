package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	overlay "github.com/rmhubbert/bubbletea-overlay"

	"gopkg.in/yaml.v3"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/promote"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
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
	screenLibraryCards
	screenLoadoutCards
	screenCreateLoadout
)

type focusTarget int

const (
	focusSidebar focusTarget = iota
	focusContent
	focusModal
)

// registryAddDoneMsg is sent when a background registry clone completes.
type registryAddDoneMsg struct {
	name  string
	err   error
	empty bool // true when the repo has no syllago structure and no native content
}

// registryRemoveDoneMsg is sent when a registry remove operation completes.
type registryRemoveDoneMsg struct {
	name string
	err  error
}

// registrySyncDoneMsg is sent when a registry sync operation completes.
type registrySyncDoneMsg struct {
	name string
	err  error
}

// registryAddNonSyllagoMsg is sent when a registry clone contains provider
// content but no syllago structure, offering a redirect to the import flow.
type registryAddNonSyllagoMsg struct {
	name      string
	clonePath string
	scan      catalog.NativeScanResult
}

// itemRemoveDoneMsg is sent when a library item remove operation completes.
type itemRemoveDoneMsg struct {
	name string
	err  error
}

// doCreateLoadoutMsg is sent when the loadout create operation completes.
type doCreateLoadoutMsg struct {
	name     string
	provider string // provider slug used during creation
	err      error
}

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
	modal                confirmModal
	saveModal            saveModal
	envModal             envSetupModal
	instModal            installModal
	registryAddModal     registryAddModal
	createLoadout createLoadoutScreen
	registryOpInProgress bool
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
	toast toastModel

	// Pending non-syllago redirect (between detection and user confirmation)
	pendingNonSyllagoClone string                  // temp clone path
	pendingNonSyllagoScan  catalog.NativeScanResult // what was detected

	// Detail model cache (preserves state when re-entering same item)
	cachedDetail     *detailModel
	cachedDetailPath string

	// Update check state (persists across screen changes)
	remoteVersion string
	commitsBehind int

	showHidden      bool   // when true, hidden items are included in lists
	cardCursor      int    // selected card index on card view screens
	cardScrollOffset int   // first visible card row on card grid pages
	cardParent      screen // which card screen the items list was entered from (0 = none)

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
	case modalLoadoutApply:
		return a.runLoadoutApply(a.loadoutApplyItem, a.loadoutApplyMode)
	case modalHookBrokenWarning:
		return a.detail.startInstall()
	case modalItemRemove:
		item := a.detail.item
		repoRoot := a.catalog.RepoRoot
		providers := a.providers
		return func() tea.Msg {
			// Uninstall from all providers where installed
			for _, p := range providers {
				if p.Detected && p.SupportsType(item.Type) {
					status := installer.CheckStatus(item, p, repoRoot)
					if status == installer.StatusInstalled {
						installer.Uninstall(item, p, repoRoot)
					}
				}
			}
			// Delete the library directory
			err := os.RemoveAll(item.Path)
			return itemRemoveDoneMsg{name: item.Name, err: err}
		}
	case modalRegistryRemove:
		a.registryOpInProgress = true
		a.toast.show(toastMsg{text: fmt.Sprintf("Removing registry: %s...", a.registries.entries[a.cardCursor].name), isProgress: true})
		name := a.registries.entries[a.cardCursor].name
		root := a.catalog.RepoRoot
		removeCmd := func() tea.Msg {
			freshCfg, err := config.Load(root)
			if err != nil {
				return registryRemoveDoneMsg{name: name, err: fmt.Errorf("loading config: %w", err)}
			}
			var filtered []config.Registry
			for _, r := range freshCfg.Registries {
				if r.Name != name {
					filtered = append(filtered, r)
				}
			}
			freshCfg.Registries = filtered
			if err := config.Save(root, freshCfg); err != nil {
				return registryRemoveDoneMsg{name: name, err: fmt.Errorf("saving config: %w", err)}
			}
			if err := registry.Remove(name); err != nil {
				return registryRemoveDoneMsg{name: name, err: fmt.Errorf("removing clone: %w", err)}
			}
			return registryRemoveDoneMsg{name: name}
		}
		return tea.Batch(removeCmd, a.toast.tickSpinner())
	case modalNonSyllagoRedirect:
		a.importer.cleanup() // clean up any previous clone
		a.importer.initFromExternalClone(a.pendingNonSyllagoClone)
		a.pendingNonSyllagoClone = ""
		a.pendingNonSyllagoScan = catalog.NativeScanResult{}
		a.screen = screenImport
	}
	return nil
}

// cleanupDismissedModal handles cleanup when a modal is dismissed without confirming.
// Currently only needed for the non-syllago redirect modal (clone cleanup).
func (a *App) cleanupDismissedModal() {
	if a.pendingNonSyllagoClone != "" {
		os.RemoveAll(a.pendingNonSyllagoClone)
		a.pendingNonSyllagoClone = ""
		a.pendingNonSyllagoScan = catalog.NativeScanResult{}
	}
}

// rebuildRegistryState reloads config from disk, rebuilds the registries model,
// and rescans the catalog. Called by all registry done-message handlers.
func (a *App) rebuildRegistryState() {
	cfg, err := config.Load(a.catalog.RepoRoot)
	if err == nil {
		a.registryCfg = cfg
	}
	// Rebuild registry sources from fresh config so newly added/removed
	// registries are included in the catalog scan.
	var sources []catalog.RegistrySource
	for _, r := range a.registryCfg.Registries {
		if registry.IsCloned(r.Name) {
			dir, _ := registry.CloneDir(r.Name)
			sources = append(sources, catalog.RegistrySource{Name: r.Name, Path: dir})
		}
	}
	a.registrySources = sources
	// Rescan catalog first so the registries model sees correct item counts.
	cat, scanErr := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
	if scanErr == nil {
		a.catalog = cat
		a.refreshSidebarCounts()
	}
	a.registries = newRegistriesModel(a.catalog.RepoRoot, a.registryCfg, a.catalog)
	a.registries.width = a.width - sidebarWidth - 1
	a.registries.height = a.panelHeight()
	a.sidebar.registryCount = len(a.registryCfg.Registries)
}

// doRegistryAdd starts an async registry clone and config save.
func (a *App) doRegistryAdd(gitURL, nameOverride string) tea.Cmd {
	a.toast.show(toastMsg{text: fmt.Sprintf("Cloning registry: %s...", gitURL), isProgress: true})
	a.registryOpInProgress = true
	root := a.catalog.RepoRoot
	cloneCmd := func() tea.Msg {
		var empty bool
		name := nameOverride
		if name == "" {
			name = registry.NameFromURL(gitURL)
		}
		if err := registry.Clone(gitURL, name, ""); err != nil {
			return registryAddDoneMsg{name: name, err: err}
		}

		// Smart detection: reject non-syllago repos.
		dir, _ := registry.CloneDir(name)
		scanResult := catalog.ScanNativeContent(dir)
		if !scanResult.HasSyllagoStructure && len(scanResult.Providers) > 0 {
			// Before rejecting, check if the repo has indexed items via registry.yaml.
			// A repo with a manifest.Items list is a valid indexed native registry.
			manifest, _ := registry.LoadManifestFromDir(dir)
			if manifest == nil || len(manifest.Items) == 0 {
				return registryAddNonSyllagoMsg{
					name:      name,
					clonePath: dir,
					scan:      scanResult,
				}
			}
		} else if !scanResult.HasSyllagoStructure && len(scanResult.Providers) == 0 {
			// No content at all — flag for warning message
			empty = true
		}

		freshCfg, err := config.Load(root)
		if err != nil {
			dir, _ := registry.CloneDir(name)
			os.RemoveAll(dir)
			return registryAddDoneMsg{name: name, err: fmt.Errorf("loading config: %w", err)}
		}
		freshCfg.Registries = append(freshCfg.Registries, config.Registry{Name: name, URL: gitURL})
		if err := config.Save(root, freshCfg); err != nil {
			dir, _ := registry.CloneDir(name)
			os.RemoveAll(dir)
			return registryAddDoneMsg{name: name, err: fmt.Errorf("saving config: %w", err)}
		}
		return registryAddDoneMsg{name: name, empty: empty}
	}
	return tea.Batch(cloneCmd, a.toast.tickSpinner())
}

// doCreateLoadoutFromScreen writes a loadout.yaml from the screen wizard state.
func (a *App) doCreateLoadoutFromScreen(m createLoadoutScreen) tea.Cmd {
	contentRoot := a.catalog.RepoRoot
	scopeRegistry := m.scopeRegistry
	return func() tea.Msg {
		name := strings.TrimSpace(m.nameInput.Value())
		if errMsg := catalog.ValidateUserName(name); errMsg != "" {
			return doCreateLoadoutMsg{err: fmt.Errorf("invalid loadout name: %s", errMsg)}
		}
		desc := strings.TrimSpace(m.descInput.Value())
		provSlug := m.prefilledProvider

		manifest := loadout.Manifest{
			Kind:        "loadout",
			Version:     1,
			Provider:    provSlug,
			Name:        name,
			Description: desc,
		}
		for _, e := range m.selectedItems() {
			switch e.item.Type {
			case catalog.Rules:
				manifest.Rules = append(manifest.Rules, e.item.Name)
			case catalog.Hooks:
				manifest.Hooks = append(manifest.Hooks, e.item.Name)
			case catalog.Skills:
				manifest.Skills = append(manifest.Skills, e.item.Name)
			case catalog.Agents:
				manifest.Agents = append(manifest.Agents, e.item.Name)
			case catalog.MCP:
				manifest.MCP = append(manifest.MCP, e.item.Name)
			case catalog.Commands:
				manifest.Commands = append(manifest.Commands, e.item.Name)
			}
		}

		var destDir string
		switch m.destCursor {
		case 0:
			destDir = filepath.Join(contentRoot, "loadouts", provSlug)
		case 1:
			globalDir := catalog.GlobalContentDir()
			if globalDir == "" {
				return doCreateLoadoutMsg{err: fmt.Errorf("finding home directory")}
			}
			destDir = filepath.Join(globalDir, "loadouts", provSlug)
		case 2:
			dir, err := registry.CloneDir(scopeRegistry)
			if err != nil {
				return doCreateLoadoutMsg{err: err}
			}
			destDir = filepath.Join(dir, "loadouts", provSlug)
		}

		itemDir := filepath.Join(destDir, name)
		if err := os.MkdirAll(itemDir, 0755); err != nil {
			return doCreateLoadoutMsg{err: fmt.Errorf("creating loadout dir: %w", err)}
		}

		data, err := yaml.Marshal(manifest)
		if err != nil {
			return doCreateLoadoutMsg{err: fmt.Errorf("marshaling manifest: %w", err)}
		}

		outPath := filepath.Join(itemDir, "loadout.yaml")
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return doCreateLoadoutMsg{err: fmt.Errorf("writing loadout.yaml: %w", err)}
		}

		return doCreateLoadoutMsg{name: name, provider: provSlug}
	}
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

// contentWidth returns the usable width for the content panel (right of sidebar).
func (a App) contentWidth() int {
	w := a.width - sidebarWidth - 1
	if w < 20 {
		w = 20
	}
	return w
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
	a.sidebar.loadoutsCount = counts[catalog.Loadouts]
}

// rebuildItems refreshes the items list data while preserving navigation context
// (breadcrumbs, provider filter, display flags). Use this whenever the underlying
// catalog data may have changed (install, remove, toggle-hidden, search cancel).
// See .claude/rules/tui-items-rebuild.md for the full pattern.
func (a *App) rebuildItems() {
	a.rebuildItemsFiltered("")
}

// rebuildItemsFiltered is like rebuildItems but applies a search filter to the results.
// Pass empty string for no filter (full rebuild).
func (a *App) rebuildItemsFiltered(query string) {
	ct := a.items.contentType
	if ct == catalog.SearchResults && query == "" {
		return // nothing to rebuild for search results without a query
	}

	savedCtx := a.items.ctx
	oldCursor := a.items.cursor

	// Build the source item list
	var src []catalog.ContentItem
	if ct == catalog.Library {
		for _, item := range a.visibleItems(a.catalog.Items) {
			if item.Library {
				src = append(src, item)
			}
		}
		savedCtx.hiddenCount = countHidden(a.catalog.Items)
		savedCtx.hideLibraryBadge = true
	} else if ct == catalog.SearchResults {
		src = a.visibleItems(a.catalog.Items)
		// SearchResults: hiddenCount stays as-is
	} else {
		src = a.visibleItems(a.catalog.ByType(ct))
		savedCtx.hiddenCount = countHidden(a.catalog.ByType(ct))

		// Apply provider filter (e.g., loadouts drilled in from cards)
		if savedCtx.sourceProvider != "" {
			var filtered []catalog.ContentItem
			for _, item := range src {
				if item.Provider == savedCtx.sourceProvider {
					filtered = append(filtered, item)
				}
			}
			src = filtered
		}
	}

	// Apply search filter if provided
	if query != "" {
		src = filterItems(src, query)
	}

	items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
	items.ctx = savedCtx
	items.width = a.width - sidebarWidth - 1
	items.height = a.panelHeight()

	// Clamp cursor to valid range
	if oldCursor >= len(items.items) && len(items.items) > 0 {
		items.cursor = len(items.items) - 1
	} else {
		items.cursor = oldCursor
	}

	a.items = items
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
		a.detail.fileViewer.splitView.width = contentW
		a.detail.loadoutContents.splitView.width = contentW
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
		a.createLoadout.width = contentW
		a.createLoadout.height = ph
		a.createLoadout.splitView.width = contentW
		a.createLoadout.splitView.height = ph - 5
		a.toast.width = contentW
		return a, nil

	case toastMsg:
		a.toast.show(msg)
		return a, nil

	case shareDoneMsg:
		if a.screen == screenDetail {
			a.cachedDetail = nil // invalidate cache
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			a.promoteDetailMessage()
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

	case splitViewCursorMsg:
		if a.screen == screenDetail {
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd
		}
		if a.screen == screenCreateLoadout {
			if msg.item.Path != "" {
				content, err := os.ReadFile(msg.item.Path)
				if err != nil {
					a.createLoadout.splitView.SetPreview("")
				} else {
					a.createLoadout.splitView.SetPreview(string(content))
				}
			} else {
				a.createLoadout.splitView.SetPreview("")
			}
			return a, nil
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

	case discoveryDoneMsg:
		if a.screen == screenImport {
			var cmd tea.Cmd
			a.importer, cmd = a.importer.Update(msg)
			return a, cmd
		}

	case importBackToRegistriesMsg:
		a.screen = screenRegistries
		return a, nil

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
			text := fmt.Sprintf("Added %q to library", msg.name)
			if len(msg.warnings) > 0 {
				text += "\n" + strings.Join(msg.warnings, "\n")
			}
			a.toast.show(toastMsg{text: text})
		} else {
			text := fmt.Sprintf("Added %q but catalog rescan failed: %s", msg.name, err)
			if len(msg.warnings) > 0 {
				text += "\n" + strings.Join(msg.warnings, "\n")
			}
			a.toast.show(toastMsg{text: text, isErr: true})
		}
		a.refreshSidebarCounts()
		a.importer.cleanup()

		// Navigate to the imported content:
		// - Single item with known type → detail view
		// - Multiple items with known type → items list
		// - Unknown type (discovery/mixed) → Library cards
		ct := msg.contentType
		isSingleItem := ct != "" && !strings.Contains(msg.name, ", ")

		if ct != "" && err == nil {
			src := a.visibleItems(cat.ByType(ct))
			items := newItemsModel(ct, src, a.providers, cat.RepoRoot)
			items.ctx.hiddenCount = countHidden(cat.ByType(ct))
			items.ctx.hideLibraryBadge = true
			items.ctx.parentLabel = "Library"
			items.width = a.width - sidebarWidth - 1
			items.height = a.panelHeight()
			a.items = items
			a.cardParent = screenLibraryCards

			if isSingleItem {
				// Find the new item and navigate to its detail view
				for i, item := range src {
					if item.Name == msg.name {
						a.items.cursor = i
						a.detail = newDetailModel(item, a.providers, cat.RepoRoot, cat)
						a.detail.overrides = cat.OverridesFor(item.Name, item.Type)
						a.detail.parentLabel = "Library"
						a.detail.categoryLabel = ct.Label()
						a.detail.width = a.contentWidth()
						a.detail.height = a.panelHeight()
						a.detail.fileViewer.splitView.width = a.contentWidth()
						a.detail.loadoutContents.splitView.width = a.contentWidth()
						a.detail.listPosition = i
						a.detail.listTotal = len(src)
						a.screen = screenDetail
						return a, nil
					}
				}
			}
			// Batch import or item not found — show items list
			a.screen = screenItems
		} else {
			// Unknown type or rescan failed — fall back to Library cards
			a.screen = screenLibraryCards
		}
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
		var cmds []tea.Cmd
		if a.toast.active && a.toast.isProgress {
			cmds = append(cmds, a.toast.updateSpinner(msg))
		}
		if a.screen == screenUpdate {
			var cmd tea.Cmd
			a.updater, cmd = a.updater.Update(msg)
			cmds = append(cmds, cmd)
		}
		if len(cmds) > 0 {
			return a, tea.Batch(cmds...)
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

	case itemRemoveDoneMsg:
		if msg.err != nil {
			a.toast.show(toastMsg{text: fmt.Sprintf("Remove failed: %s", msg.err), isErr: true})
		} else {
			a.toast.show(toastMsg{text: fmt.Sprintf("Removed %q from library", msg.name)})
			a.cachedDetail = nil
			cat, err := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
			if err == nil {
				a.catalog = cat
				a.refreshSidebarCounts()
			}
			// If on detail screen, navigate back to items
			if a.screen == screenDetail {
				a.screen = screenItems
				a.rebuildItems()
			} else if a.screen == screenItems {
				a.rebuildItems()
			}
		}
		return a, nil

	case registryAddDoneMsg:
		a.registryOpInProgress = false
		if msg.err != nil {
			a.toast.show(toastMsg{text: fmt.Sprintf("Add failed: %s", msg.err), isErr: true})
		} else {
			a.rebuildRegistryState()
			count := a.catalog.CountRegistry(msg.name)
			if msg.empty {
				a.toast.show(toastMsg{text: fmt.Sprintf("Added registry: %s (empty — no content found)", msg.name)})
			} else if count == 1 {
				a.toast.show(toastMsg{text: fmt.Sprintf("Added registry: %s (1 item)", msg.name)})
			} else {
				a.toast.show(toastMsg{text: fmt.Sprintf("Added registry: %s (%d items)", msg.name, count)})
			}
		}
		return a, nil

	case registryAddNonSyllagoMsg:
		a.registryOpInProgress = false
		a.pendingNonSyllagoClone = msg.clonePath
		a.pendingNonSyllagoScan = msg.scan
		var provNames []string
		for _, pc := range msg.scan.Providers {
			provNames = append(provNames, pc.ProviderName)
		}
		body := fmt.Sprintf("This repository contains %s content\nbut isn't a syllago registry.\n\nBrowse and import individual items?", strings.Join(provNames, ", "))
		a.modal = newConfirmModal("Not a Syllago Registry", body)
		a.modal.purpose = modalNonSyllagoRedirect
		a.focus = focusModal
		return a, nil

	case registryRemoveDoneMsg:
		a.registryOpInProgress = false
		if msg.err != nil {
			a.toast.show(toastMsg{text: fmt.Sprintf("Remove failed: %s", msg.err), isErr: true})
		} else {
			a.toast.show(toastMsg{text: fmt.Sprintf("Removed registry: %s", msg.name)})
			a.rebuildRegistryState()
			if a.cardCursor >= len(a.registries.entries) && a.cardCursor > 0 {
				a.cardCursor--
			}
		}
		return a, nil

	case registrySyncDoneMsg:
		a.registryOpInProgress = false
		if msg.err != nil {
			a.toast.show(toastMsg{text: fmt.Sprintf("Sync failed for %s: %s", msg.name, msg.err), isErr: true})
		} else {
			a.toast.show(toastMsg{text: fmt.Sprintf("Synced: %s", msg.name)})
			a.rebuildRegistryState()
		}
		return a, nil

	case doCreateLoadoutMsg:
		if msg.err != nil {
			a.toast.show(toastMsg{text: fmt.Sprintf("Create loadout failed: %s", msg.err), isErr: true})
		} else {
			a.toast.show(toastMsg{text: fmt.Sprintf("Created loadout: %s", msg.name)})
			cat, err := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
			if err == nil {
				a.catalog = cat
				a.refreshSidebarCounts()

				// Build items list for this provider's loadouts
				prov := msg.provider
				src := a.visibleItems(cat.ByType(catalog.Loadouts))
				var filtered []catalog.ContentItem
				for _, item := range src {
					if item.Provider == prov {
						filtered = append(filtered, item)
					}
				}
				items := newItemsModel(catalog.Loadouts, filtered, a.providers, cat.RepoRoot)
				items.ctx.sourceProvider = prov
				items.ctx.parentLabel = "Loadouts"
				items.width = a.width - sidebarWidth - 1
				items.height = a.panelHeight()
				a.items = items
				a.cardParent = screenLoadoutCards

				// Find the new loadout and navigate to its detail view
				for i, item := range filtered {
					if item.Name == msg.name {
						a.items.cursor = i
						a.detail = newDetailModel(item, a.providers, cat.RepoRoot, cat)
						a.detail.overrides = cat.OverridesFor(item.Name, item.Type)
						a.detail.parentLabel = "Loadouts"
						a.detail.categoryLabel = providerDisplayName(prov)
						a.detail.width = a.contentWidth()
						a.detail.height = a.panelHeight()
						a.detail.fileViewer.splitView.width = a.contentWidth()
						a.detail.loadoutContents.splitView.width = a.contentWidth()
						a.detail.listPosition = i
						a.detail.listTotal = len(filtered)
						a.screen = screenDetail
						return a, nil
					}
				}
				// Fallback: loadout not found in catalog, show provider's list
				a.screen = screenItems
			}
		}
		return a, nil

	case tea.MouseMsg:
		// Forward wheel events to active screen for scroll support
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			up := msg.Button == tea.MouseButtonWheelUp

			// Help overlay intercepts all scroll when active
			if a.helpOverlay.active {
				if up {
					a.helpOverlay.Update(tea.KeyMsg{Type: tea.KeyUp})
				} else {
					a.helpOverlay.Update(tea.KeyMsg{Type: tea.KeyDown})
				}
				return a, nil
			}

			// Sidebar wheel: scroll sidebar when mouse is within sidebar bounds
			if msg.X < sidebarWidth {
				if up && a.sidebar.cursor > 0 {
					a.sidebar.cursor--
				} else if !up && a.sidebar.cursor < a.sidebar.totalItems()-1 {
					a.sidebar.cursor++
				}
				return a, nil
			}

			switch a.screen {
			case screenImport:
				var cmd tea.Cmd
				a.importer, cmd = a.importer.Update(msg)
				return a, cmd
			case screenDetail:
				// Files tab: forward to detail model so the split view
				// can route scroll to the correct pane (list vs preview).
				if a.detail.activeTab == tabFiles {
					var cmd tea.Cmd
					a.detail, cmd = a.detail.Update(msg)
					return a, cmd
				}
				if up {
					a.detail.scrollOffset--
				} else {
					a.detail.scrollOffset++
				}
				a.detail.clampScroll()
				return a, nil
			case screenUpdate:
				if a.updater.step == stepUpdatePreview {
					if up {
						if a.updater.scrollOffset > 0 {
							a.updater.scrollOffset--
						}
					} else {
						a.updater.scrollOffset++
					}
				}
				return a, nil
			case screenItems:
				if up && a.items.cursor > 0 {
					a.items.cursor--
				} else if !up && a.items.cursor < len(a.items.items)-1 {
					a.items.cursor++
				}
				return a, nil
			case screenCategory, screenLibraryCards, screenLoadoutCards:
				contentW := a.width - sidebarWidth - 1
				cols := 2
				if contentW < 42 {
					cols = 1
				}
				var maxCards int
				switch a.screen {
				case screenCategory:
					maxCards = a.welcomeCardCount()
				case screenLibraryCards:
					maxCards = len(a.libraryCardTypes())
				case screenLoadoutCards:
					maxCards = len(a.loadoutCardProviders())
				}
				if up {
					a.cardCursor -= cols
					if a.cardCursor < 0 {
						a.cardCursor = 0
					}
				} else {
					a.cardCursor += cols
					if a.cardCursor >= maxCards {
						a.cardCursor = maxCards - 1
					}
				}
				return a, nil
			case screenRegistries:
				cols := 2
				if a.registries.width < 42 {
					cols = 1
				}
				maxCards := len(a.registries.entries)
				if up {
					a.cardCursor -= cols
					if a.cardCursor < 0 {
						a.cardCursor = 0
					}
				} else {
					a.cardCursor += cols
					if a.cardCursor >= maxCards {
						a.cardCursor = maxCards - 1
					}
				}
				return a, nil
			case screenCreateLoadout:
				var cmd tea.Cmd
				a.createLoadout, cmd = a.createLoadout.Update(msg)
				return a, cmd
			}
			return a, nil
		}
		if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
			return a, nil
		}
		// Modal mouse handling is centralized here in app.go, NOT in each
		// modal's Update() method. Inner zone.Mark() calls in a modal's View()
		// don't survive overlay.Composite() — only the outer "modal-zone"
		// wrapper is usable. So we use coordinate-based hit testing (relX/relY)
		// within the modal-zone bounds for buttons, options, and input fields.
		//
		// Click outside modal-zone → dismiss. Click inside → route by relY.
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
			a.cleanupDismissedModal()
			return a, nil
		}
		if a.saveModal.active {
			z := zone.Get("modal-zone")
			if z.InBounds(msg) {
				relX, relY := z.Pos(msg)
				modalH := z.EndY - z.StartY + 1
				modalW := z.EndX - z.StartX + 1
				buttonRelY := modalH - 3
				if relY == buttonRelY {
					contentLeft := 3
					contentW := modalW - 6
					midX := contentLeft + contentW/2
					if relX < midX {
						// Save button
						if strings.TrimSpace(a.saveModal.input.Value()) != "" {
							a.saveModal.value = strings.TrimSpace(a.saveModal.input.Value())
							a.saveModal.confirmed = true
							a.saveModal.active = false
							a.focus = focusContent
						}
					} else {
						// Cancel button
						a.saveModal.active = false
						a.saveModal.confirmed = false
						a.focus = focusContent
					}
					return a, nil
				}
				// Input field click (relY=4: border+padding+title+blank)
				if relY == 4 {
					a.saveModal.focusedField = 0
					a.saveModal.input.Focus()
					return a, nil
				}
				return a, nil // click inside modal but not on buttons
			}
			a.saveModal.active = false
			a.saveModal.confirmed = false
			a.focus = focusContent
			return a, nil
		}
		if a.envModal.active {
			z := zone.Get("modal-zone")
			if z.InBounds(msg) {
				relX, relY := z.Pos(msg)
				modalH := z.EndY - z.StartY + 1
				modalW := z.EndX - z.StartX + 1
				buttonRelY := modalH - 3

				// Button row clicks — all steps
				if relY == buttonRelY {
					contentLeft := 3
					contentW := modalW - 6
					midX := contentLeft + contentW/2
					if relX < midX {
						a.envModal.btnCursor = 0
					} else {
						a.envModal.btnCursor = 1
					}
					switch a.envModal.step {
					case envStepChoose:
						m, cmd := a.envModal.Update(tea.KeyMsg{Type: tea.KeyEnter})
						a.envModal = m
						if !a.envModal.active {
							a.focus = focusContent
						}
						return a, cmd
					default:
						// Value/Location/Source: left button = Enter, right button = Esc (back)
						if a.envModal.btnCursor == 0 {
							m, cmd := a.envModal.Update(tea.KeyMsg{Type: tea.KeyEnter})
							a.envModal = m
							if !a.envModal.active {
								a.focus = focusContent
							}
							return a, cmd
						}
						m, cmd := a.envModal.Update(tea.KeyMsg{Type: tea.KeyEsc})
						a.envModal = m
						return a, cmd
					}
				}

				// Radio option clicks — envStepChoose only (relY 5-6)
				if a.envModal.step == envStepChoose {
					if relY == 5 {
						a.envModal.methodCursor = 0
						return a, nil
					}
					if relY == 6 {
						a.envModal.methodCursor = 1
						return a, nil
					}
				}

				// Text input click-to-focus (Value/Location/Source steps)
				if a.envModal.step != envStepChoose {
					if zone.Get("modal-field-input").InBounds(msg) {
						a.envModal.input.Focus()
						return a, nil
					}
				}

				return a, nil // click inside modal but not on interactive elements
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

				// Custom path step: click on input field (relY=4) focuses it
				if a.instModal.step == installStepCustomPath && relY == 4 {
					a.instModal.customPathInput.Focus()
					return a, nil
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
							// Don't allow selecting symlink when disabled
							if i == 0 && a.instModal.symlinkDisabled() {
								return a, nil
							}
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
		if a.registryAddModal.active {
			z := zone.Get("modal-zone")
			if z.InBounds(msg) {
				relX, relY := z.Pos(msg)
				modalH := z.EndY - z.StartY + 1
				modalW := z.EndX - z.StartX + 1

				// Button row: last content line before bottom padding(1)+border(1)
				buttonRelY := modalH - 3
				if relY == buttonRelY {
					a.registryAddModal.focusedField = 2
					a.registryAddModal.urlInput.Blur()
					a.registryAddModal.nameInput.Blur()
					contentLeft := 3
					contentW := modalW - 6
					midX := contentLeft + contentW/2
					if relX < midX {
						a.registryAddModal.btnCursor = 0
					} else {
						a.registryAddModal.btnCursor = 1
					}
					enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
					var cmd tea.Cmd
					a.registryAddModal, cmd = a.registryAddModal.Update(enterMsg)
					if !a.registryAddModal.active {
						a.focus = focusContent
						if a.registryAddModal.confirmed {
							url := strings.TrimSpace(a.registryAddModal.urlInput.Value())
							nameOverride := strings.TrimSpace(a.registryAddModal.nameInput.Value())
							return a, a.doRegistryAdd(url, nameOverride)
						}
					}
					return a, cmd
				}

				// URL field click (relY ~= 4: border+padding+title+blank)
				if relY >= 4 && relY <= 4 {
					a.registryAddModal.focusedField = 0
					a.registryAddModal.nameInput.Blur()
					a.registryAddModal.urlInput.Focus()
					return a, nil
				}
				// Name field click (relY ~= 5)
				if relY >= 5 && relY <= 5 {
					a.registryAddModal.focusedField = 1
					a.registryAddModal.urlInput.Blur()
					a.registryAddModal.nameInput.Focus()
					return a, nil
				}

				return a, nil // click inside modal elsewhere — ignore
			}
			// Click outside modal — close it
			a.registryAddModal.active = false
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
			if a.items.contentType == catalog.Loadouts {
				if zone.Get("action-a").InBounds(msg) {
					provider := a.items.ctx.sourceProvider
					contentW := a.width - sidebarWidth - 1
					a.createLoadout = newCreateLoadoutScreen(provider, a.items.ctx.sourceRegistry, a.providers, a.catalog, contentW, a.panelHeight())
					a.screen = screenCreateLoadout
					a.focus = focusContent
					return a, nil
				}
			}
			// Task 1.6: library items list action buttons
			if zone.Get("action-a").InBounds(msg) {
				return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
			}
			if zone.Get("action-r").InBounds(msg) {
				return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
			}
			for i := range a.items.items {
				if zone.Get(fmt.Sprintf("item-%d", i)).InBounds(msg) {
					a.items.cursor = i
					a.focus = focusContent
					return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
				}
			}
		}
		// Check welcome page content type links
		for i := range a.sidebar.types {
			if zone.Get(fmt.Sprintf("welcome-%d", i)).InBounds(msg) {
				a.sidebar.cursor = i
				a.screen = screenCategory
				a.focus = focusSidebar
				return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
			}
		}
		// Check welcome page collection/configuration links (mapped to sidebar indices)
		welcomeZoneMap := map[string]int{
			"welcome-library":    a.sidebar.libraryIdx(),
			"welcome-loadouts":   a.sidebar.loadoutsIdx(),
			"welcome-registries": a.sidebar.registriesIdx(),
			"welcome-add":        a.sidebar.addIdx(),
			"welcome-update":     a.sidebar.updateIdx(),
			"welcome-settings":   a.sidebar.settingsIdx(),
			"welcome-sandbox":    a.sidebar.sandboxIdx(),
		}
		for zoneID, sidebarIdx := range welcomeZoneMap {
			if zone.Get(zoneID).InBounds(msg) {
				a.sidebar.cursor = sidebarIdx
				a.focus = focusSidebar
				return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
			}
		}
		// Check library card clicks
		if a.screen == screenLibraryCards {
			if zone.Get("action-a").InBounds(msg) {
				a.importer = newImportModel(a.providers, a.catalog.RepoRoot, a.projectRoot)
				a.importer.width = a.width - sidebarWidth - 1
				a.importer.height = a.panelHeight()
				a.screen = screenImport
				a.focus = focusContent
				return a, nil
			}
			for i, ct := range a.libraryCardTypes() {
				if zone.Get(fmt.Sprintf("library-card-%s", ct)).InBounds(msg) {
					a.cardCursor = i
					return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
				}
			}
		}
		// Check loadout card clicks
		if a.screen == screenLoadoutCards {
			if zone.Get("action-a").InBounds(msg) {
				contentW := a.width - sidebarWidth - 1
				a.createLoadout = newCreateLoadoutScreen("", "", a.providers, a.catalog, contentW, a.panelHeight())
				a.screen = screenCreateLoadout
				a.focus = focusContent
				return a, nil
			}
			for i, prov := range a.loadoutCardProviders() {
				if zone.Get(fmt.Sprintf("loadout-card-%s", prov)).InBounds(msg) {
					a.cardCursor = i
					return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
				}
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
			if a.screen == screenItems {
				// Navigate back to card page (same as Esc from items)
				return a.Update(tea.KeyMsg{Type: tea.KeyEsc})
			}
		}
		if zone.Get("crumb-registries").InBounds(msg) {
			if a.screen == screenItems && a.items.ctx.sourceRegistry != "" {
				// Navigate back to registries screen
				a.screen = screenRegistries
				a.focus = focusContent
				return a, nil
			}
		}
		if zone.Get("crumb-parent").InBounds(msg) {
			if a.cardParent != 0 {
				if a.screen == screenDetail {
					// Detail → items first (Esc behavior)
					a.screen = screenItems
					return a, nil
				}
				if a.screen == screenItems {
					a.screen = a.cardParent
					a.cardParent = 0
					a.focus = focusContent
					return a, nil
				}
			}
		}
		// Check detail tab zones
		if a.screen == screenDetail {
			tabs := []detailTab{tabFiles, tabCompatibility, tabInstall}
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
		// Split view mouse events (Files tab) — delegated to split view via detail Update
		if a.screen == screenDetail && a.detail.activeTab == tabFiles {
			// Back link in single-pane preview
			if zone.Get("sv-files-back").InBounds(msg) {
				a.detail.fileViewer.splitView.showingPreview = false
				return a, nil
			}
			if zone.Get("sv-contents-back").InBounds(msg) {
				a.detail.loadoutContents.splitView.showingPreview = false
				return a, nil
			}
			// Forward left clicks to split view for item selection
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd
		}
		// Check detail action button zones
		if a.screen == screenDetail {
			btnChars := map[string]string{
				"detail-btn-install":   "i",
				"detail-btn-uninstall": "u",
				"detail-btn-copy":      "c",
				"detail-btn-save":      "s",
				"detail-btn-env":       "e",
				"detail-btn-share":     "p",
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
					// Skip CompatNone providers (keyboard navigation also skips them)
					if a.detail.item.Type == catalog.Hooks {
						if cr := a.detail.hookCompatForProvider(detected[i].Slug); cr != nil && cr.Level == converter.CompatNone {
							return a, nil
						}
					}
					a.detail.provCheck.cursor = i
					a.detail.provCheck.checks[i] = !a.detail.provCheck.checks[i]
					return a, nil
				}
			}
			// Loadout mode selector (Preview/Try/Keep) and Apply button
			if a.detail.item.Type == catalog.Loadouts {
				if zone.Get("detail-btn-apply").InBounds(msg) {
					return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
				}
				for i := 0; i < 3; i++ {
					if zone.Get(fmt.Sprintf("detail-mode-%d", i)).InBounds(msg) {
						a.detail.loadoutModeCursor = i
						return a, nil
					}
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
			a.promoteSettingsMessage()
			return a, cmd
		case screenRegistries:
			// Action buttons
			if zone.Get("action-a").InBounds(msg) {
				return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
			}
			if zone.Get("action-s").InBounds(msg) {
				return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
			}
			if zone.Get("action-r").InBounds(msg) {
				return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
			}
			// Check registry card clicks
			if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
				for i := range a.registries.entries {
					if zone.Get(fmt.Sprintf("registry-card-%d", i)).InBounds(msg) {
						a.cardCursor = i
						return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
					}
				}
			}
			return a, nil
		case screenSandbox:
			var cmd tea.Cmd
			a.sandboxSettings, cmd = a.sandboxSettings.Update(msg)
			a.promoteSandboxMessage()
			return a, cmd
		case screenCreateLoadout:
			var cmd tea.Cmd
			a.createLoadout, cmd = a.createLoadout.Update(msg)
			if a.createLoadout.confirmed {
				return a, a.doCreateLoadoutFromScreen(a.createLoadout)
			}
			return a, cmd
		}
		return a, nil

	case tea.KeyMsg:
		// Toast key routing (before modal routing)
		if a.toast.active {
			if a.toast.isProgress {
				// Progress toast: not dismissable — persists until replaced
			} else if a.toast.isErr {
				// Error toast: Esc dismisses, c copies. All other keys pass through.
				switch {
				case key.Matches(msg, keys.Back):
					a.toast.dismiss()
					return a, nil
				case msg.String() == "c":
					if errMsg := a.toast.copyToClipboard(); errMsg != "" {
						a.toast.text = errMsg
					} else {
						a.toast.dismiss()
					}
					return a, nil
				}
				// Other keys fall through — toast stays visible
			} else {
				// Success toast: if scrollable, handle scroll keys; otherwise dismiss
				if a.toast.isScrollable() {
					switch {
					case key.Matches(msg, keys.Back):
						a.toast.dismiss()
						return a, nil
					case key.Matches(msg, keys.Up):
						a.toast.scrollOffset--
						a.toast.clampScroll()
						return a, nil
					case key.Matches(msg, keys.Down):
						a.toast.scrollOffset++
						a.toast.clampScroll()
						return a, nil
					}
				}
				a.toast.dismiss()
				// Fall through so the key also triggers its normal action
			}
		}
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
				a.cleanupDismissedModal()
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
		if a.registryAddModal.active {
			var cmd tea.Cmd
			a.registryAddModal, cmd = a.registryAddModal.Update(msg)
			if !a.registryAddModal.active {
				a.focus = focusContent
				if a.registryAddModal.confirmed {
					url := strings.TrimSpace(a.registryAddModal.urlInput.Value())
					nameOverride := strings.TrimSpace(a.registryAddModal.nameInput.Value())
					name := nameOverride
					if name == "" {
						name = registry.NameFromURL(url)
					}
					if !catalog.IsValidRegistryName(name) {
						a.registryAddModal.message = fmt.Sprintf("Invalid registry name: %s", name)
						a.registryAddModal.messageIsErr = true
						a.registryAddModal.active = true
						a.registryAddModal.confirmed = false
						a.focus = focusModal
						return a, nil
					}
					for _, r := range a.registryCfg.Registries {
						if r.Name == name {
							a.registryAddModal.message = fmt.Sprintf("Registry %q already exists", name)
							a.registryAddModal.messageIsErr = true
							a.registryAddModal.active = true
							a.registryAddModal.confirmed = false
							a.focus = focusModal
							return a, nil
						}
					}
					return a, a.doRegistryAdd(url, nameOverride)
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
				a.helpOverlay.scrollOffset = 0
				a.helpOverlay.height = a.panelHeight()
				return a, nil
			}
		}

		// If help overlay is active, delegate to its Update (handles scroll + esc)
		if a.helpOverlay.active {
			a.helpOverlay.Update(msg)
			return a, nil
		}

		// Search toggle (skip on import/update/settings/createLoadout screens and when detail has active textinput)
		if key.Matches(msg, keys.Search) && !a.search.active && a.screen != screenImport && a.screen != screenUpdate && a.screen != screenSettings && a.screen != screenSandbox && a.screen != screenCreateLoadout && !a.detail.HasTextInput() {
			a.search = a.search.activated()
			return a, nil
		}

		// Search active: handle escape and enter
		if a.search.active {
			if msg.Type == tea.KeyEsc {
				a.search = a.search.deactivated()
				// If on items screen, reset items
				if a.screen == screenItems {
					a.rebuildItems()
				}
				// If on registries screen, reset entries
				if a.screen == screenRegistries {
					a.registries.entries = a.registries.allEntries
					if a.cardCursor >= len(a.registries.entries) {
						a.cardCursor = max(0, len(a.registries.entries)-1)
					}
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
					a.cardParent = 0
					a.screen = screenItems
					a.focus = focusContent
				} else if a.screen == screenItems {
					a.rebuildItemsFiltered(a.search.query())
				}
				a.search = a.search.deactivated()
				return a, nil
			}
			var cmd tea.Cmd
			a.search, cmd = a.search.Update(msg)

			// Live-filter items while typing
			if a.screen == screenItems {
				a.rebuildItemsFiltered(a.search.query())
			}

			// Live-filter registries while typing
			if a.screen == screenRegistries {
				query := a.search.query()
				a.registries.entries = filterRegistryEntries(a.registries.allEntries, query)
				if a.cardCursor >= len(a.registries.entries) && a.cardCursor > 0 {
					a.cardCursor = len(a.registries.entries) - 1
				}
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
		// Works on all screens with sidebar (excluding single-pane and detail screens).
		// Detail screen uses Tab for cycling its own tabs (Files/Install, Contents/Apply).
		if (key.Matches(msg, keys.Tab) || key.Matches(msg, keys.ShiftTab)) &&
			!a.search.active && !a.helpOverlay.active {
			if a.screen == screenDetail {
				// Cycle detail tabs: tabFiles -> tabInstall (or tabCompatibility for hooks)
				if !a.detail.HasTextInput() {
					var cmd tea.Cmd
					a.detail, cmd = a.detail.Update(msg)
					return a, cmd
				}
			} else if a.screen != screenImport && a.screen != screenUpdate && a.screen != screenSettings && a.screen != screenSandbox && a.screen != screenCreateLoadout {
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

		// Sidebar-focused: route input to sidebar on ANY screen.
		// Enter/Right drills into the selected sidebar section regardless of current screen.
		// Excluded: single-pane screens and the create loadout wizard.
		if a.focus == focusSidebar &&
			a.screen != screenImport &&
			a.screen != screenUpdate && a.screen != screenSettings &&
			a.screen != screenSandbox && a.screen != screenCreateLoadout {
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
					a.settings = newSettingsModel(a.catalog.RepoRoot)
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
					a.cardCursor = 0
					a.cardScrollOffset = 0
					a.screen = screenLibraryCards
					a.focus = focusContent
					return a, nil
				}
				if a.sidebar.isLoadoutsSelected() {
					loadoutItems := a.visibleItems(a.catalog.ByType(catalog.Loadouts))
					if len(loadoutItems) == 0 {
						items := newItemsModel(catalog.Loadouts, loadoutItems, a.providers, a.catalog.RepoRoot)
						items.ctx.hiddenCount = countHidden(a.catalog.ByType(catalog.Loadouts))
						items.width = a.width - sidebarWidth - 1
						items.height = a.panelHeight()
						a.items = items
						a.cardParent = 0
						a.screen = screenItems
						a.focus = focusContent
					} else {
						a.cardCursor = 0
						a.cardScrollOffset = 0
						a.screen = screenLoadoutCards
						a.focus = focusContent
					}
					return a, nil
				}
				ct := a.sidebar.selectedType()
				src := a.visibleItems(a.catalog.ByType(ct))
				items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
				items.ctx.hiddenCount = countHidden(a.catalog.ByType(ct))
				items.width = a.width - sidebarWidth - 1
				items.height = a.panelHeight()
				a.items = items
				a.cardParent = 0
				a.screen = screenItems
				a.focus = focusContent
				return a, nil
			}
			a.sidebar.focused = true
			var cmd tea.Cmd
			a.sidebar, cmd = a.sidebar.Update(msg)
			return a, cmd
		}

		// Screen-specific key handling
		switch a.screen {
		case screenCategory:
			if key.Matches(msg, keys.ToggleHidden) {
				a.showHidden = !a.showHidden
				a.refreshSidebarCounts()
				return a, nil
			}
			if a.focus == focusContent {
				totalCards := a.welcomeCardCount()
				if a.cardCursor >= totalCards {
					a.cardCursor = max(0, totalCards-1)
				}
				contentW := a.width - sidebarWidth - 1
				singleCol := contentW < 42 || a.height < 35
				cols := 2
				if singleCol {
					cols = 1
				}
				if key.Matches(msg, keys.Up) {
					if a.cardCursor >= cols {
						a.cardCursor -= cols
					}
					return a, nil
				}
				if key.Matches(msg, keys.Down) {
					if a.cardCursor+cols < totalCards {
						a.cardCursor += cols
					}
					return a, nil
				}
				if key.Matches(msg, keys.Left) {
					if a.cardCursor > 0 {
						a.cardCursor--
					}
					return a, nil
				}
				if key.Matches(msg, keys.Right) {
					if a.cardCursor+1 < totalCards {
						a.cardCursor++
					}
					return a, nil
				}
				if key.Matches(msg, keys.Enter) && totalCards > 0 {
					a.activateWelcomeCard()
					return a, nil
				}
				return a, nil
			}

		case screenItems:
			if key.Matches(msg, keys.Back) {
				if a.items.ctx.sourceRegistry != "" {
					// Came from registry drill-in — go back to registries
					a.screen = screenRegistries
					a.focus = focusContent
					return a, nil
				}
				if a.cardParent == screenLibraryCards || a.cardParent == screenLoadoutCards {
					// Came from card drill-in — go back to card screen
					a.screen = a.cardParent
					a.cardParent = 0
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
				a.rebuildItems()
				return a, nil
			}
			if key.Matches(msg, keys.CreateLoadout) && a.items.ctx.sourceRegistry != "" {
				contentW := a.width - sidebarWidth - 1
				a.createLoadout = newCreateLoadoutScreen("", a.items.ctx.sourceRegistry, a.providers, a.catalog, contentW, a.panelHeight())
				a.screen = screenCreateLoadout
				a.focus = focusContent
				return a, nil
			}
			if key.Matches(msg, keys.Add) {
				// If we're on a loadout items list, open create loadout wizard
				if a.items.contentType == catalog.Loadouts {
					provider := a.items.ctx.sourceProvider
					contentW := a.width - sidebarWidth - 1
					a.createLoadout = newCreateLoadoutScreen(provider, a.items.ctx.sourceRegistry, a.providers, a.catalog, contentW, a.panelHeight())
					a.screen = screenCreateLoadout
					a.focus = focusContent
					return a, nil
				}
				ct := a.items.contentType
				regFilter := a.items.ctx.sourceRegistry
				if ct == catalog.SearchResults || ct == catalog.Library {
					ct = ""
				}
				a.importer = newImportModelWithFilter(a.providers, a.catalog.RepoRoot, a.projectRoot, ct, regFilter)
				a.importer.width = a.width - sidebarWidth - 1
				a.importer.height = a.panelHeight()
				a.screen = screenImport
				a.focus = focusContent
				return a, nil
			}
			if key.Matches(msg, keys.Delete) && len(a.items.items) > 0 {
				item := a.items.selectedItem()
				if isRemovable(item) {
					// Populate detail so handleConfirmAction can access item info
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot, a.catalog)
					installed := a.detail.installedProviders()
					var body string
					if len(installed) > 0 {
						var names []string
						for _, p := range installed {
							names = append(names, p.Name)
						}
						body = fmt.Sprintf("Remove %q from your library?\n\nThis will uninstall from %s\nand delete the content directory.", item.Name, strings.Join(names, ", "))
					} else {
						body = fmt.Sprintf("Remove %q from your library?\n\nThis will delete the content directory.", item.Name)
					}
					a.modal = newConfirmModal("Remove from Library", body)
					a.modal.purpose = modalItemRemove
					a.focus = focusModal
					return a, nil
				}
			}
			if key.Matches(msg, keys.Enter) && len(a.items.items) > 0 {
				item := a.items.selectedItem()
				if a.cachedDetailPath == item.Path && a.cachedDetail != nil {
					a.detail = *a.cachedDetail
				} else {
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot, a.catalog)
					a.detail.overrides = a.catalog.OverridesFor(item.Name, item.Type)
				}
				a.detail.parentLabel = a.items.ctx.parentLabel
				if a.items.ctx.sourceProvider != "" {
					a.detail.categoryLabel = providerDisplayName(a.items.ctx.sourceProvider)
				}
				a.detail.width = a.contentWidth()
				a.detail.height = a.panelHeight()
				a.detail.fileViewer.splitView.width = a.contentWidth()
				a.detail.loadoutContents.splitView.width = a.contentWidth()
				a.detail.listPosition = a.items.cursor
				a.detail.listTotal = len(a.items.items)
				a.screen = screenDetail
				return a, nil
			}
			var cmd tea.Cmd
			a.items, cmd = a.items.Update(msg)
			return a, cmd

		case screenLibraryCards:
			if key.Matches(msg, keys.Back) {
				a.screen = screenCategory
				a.focus = focusSidebar
				return a, nil
			}
			cards := a.libraryCardTypes()
			if key.Matches(msg, keys.Up) {
				if a.cardCursor >= 2 {
					a.cardCursor -= 2
				}
				return a, nil
			}
			if key.Matches(msg, keys.Down) {
				if a.cardCursor+2 < len(cards) {
					a.cardCursor += 2
				}
				return a, nil
			}
			if key.Matches(msg, keys.Left) {
				if a.cardCursor > 0 {
					a.cardCursor--
				}
				return a, nil
			}
			if key.Matches(msg, keys.Right) {
				if a.cardCursor+1 < len(cards) {
					a.cardCursor++
				}
				return a, nil
			}
			if key.Matches(msg, keys.Enter) && len(cards) > 0 {
				ct := cards[a.cardCursor]
				var filtered []catalog.ContentItem
				for _, item := range a.visibleItems(a.catalog.Items) {
					if item.Library && item.Type == ct {
						filtered = append(filtered, item)
					}
				}
				items := newItemsModel(ct, filtered, a.providers, a.catalog.RepoRoot)
				items.ctx.hiddenCount = countHidden(a.catalog.Items)
				items.ctx.hideLibraryBadge = true
				items.ctx.parentLabel = "Library"
				items.width = a.width - sidebarWidth - 1
				items.height = a.panelHeight()
				a.items = items
				a.cardParent = screenLibraryCards
				a.screen = screenItems
				return a, nil
			}
			if key.Matches(msg, keys.Add) {
				a.importer = newImportModel(a.providers, a.catalog.RepoRoot, a.projectRoot)
				a.importer.width = a.width - sidebarWidth - 1
				a.importer.height = a.panelHeight()
				a.screen = screenImport
				a.focus = focusContent
				return a, nil
			}
			return a, nil

		case screenLoadoutCards:
			if key.Matches(msg, keys.Back) {
				a.screen = screenCategory
				a.focus = focusSidebar
				return a, nil
			}
			providers := a.loadoutCardProviders()
			if key.Matches(msg, keys.Up) {
				if a.cardCursor >= 2 {
					a.cardCursor -= 2
				}
				return a, nil
			}
			if key.Matches(msg, keys.Down) {
				if a.cardCursor+2 < len(providers) {
					a.cardCursor += 2
				}
				return a, nil
			}
			if key.Matches(msg, keys.Left) {
				if a.cardCursor > 0 {
					a.cardCursor--
				}
				return a, nil
			}
			if key.Matches(msg, keys.Right) {
				if a.cardCursor+1 < len(providers) {
					a.cardCursor++
				}
				return a, nil
			}
			if key.Matches(msg, keys.Enter) && len(providers) > 0 {
				prov := providers[a.cardCursor]
				var filtered []catalog.ContentItem
				for _, item := range a.visibleItems(a.catalog.ByType(catalog.Loadouts)) {
					if item.Provider == prov {
						filtered = append(filtered, item)
					}
				}
				items := newItemsModel(catalog.Loadouts, filtered, a.providers, a.catalog.RepoRoot)
				items.ctx.hiddenCount = countHidden(a.catalog.ByType(catalog.Loadouts))
				items.ctx.sourceProvider = prov
				items.ctx.parentLabel = "Loadouts"
				items.width = a.width - sidebarWidth - 1
				items.height = a.panelHeight()
				a.items = items
				a.cardParent = screenLoadoutCards
				a.screen = screenItems
				return a, nil
			}
			if key.Matches(msg, keys.Add) {
				contentW := a.width - sidebarWidth - 1
				a.createLoadout = newCreateLoadoutScreen("", "", a.providers, a.catalog, contentW, a.panelHeight())
				a.screen = screenCreateLoadout
				a.focus = focusContent
				return a, nil
			}
			return a, nil

		case screenCreateLoadout:
			// Esc on first step navigates back
			if msg.Type == tea.KeyEsc {
				if a.createLoadout.step == clStepProvider ||
					(a.createLoadout.step == clStepTypes && a.createLoadout.prefilledProvider != "") {
					a.screen = screenLoadoutCards
					a.focus = focusContent
					return a, nil
				}
			}
			var cmd tea.Cmd
			a.createLoadout, cmd = a.createLoadout.Update(msg)
			if a.createLoadout.confirmed {
				return a, a.doCreateLoadoutFromScreen(a.createLoadout)
			}
			return a, cmd

		case screenDetail:
			// Next/previous item navigation (ctrl+n / ctrl+p)
			if msg.String() == "ctrl+n" && !a.detail.HasTextInput() {
				if a.items.cursor < len(a.items.items)-1 {
					a.items.cursor++
					item := a.items.selectedItem()
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot, a.catalog)
					a.detail.overrides = a.catalog.OverridesFor(item.Name, item.Type)
					a.detail.width = a.contentWidth()
					a.detail.height = a.panelHeight()
					a.detail.fileViewer.splitView.width = a.contentWidth()
					a.detail.loadoutContents.splitView.width = a.contentWidth()
					a.detail.listPosition = a.items.cursor
					a.detail.listTotal = len(a.items.items)
				}
				return a, nil
			}
			if msg.String() == "ctrl+p" && !a.detail.HasTextInput() {
				if a.items.cursor > 0 {
					a.items.cursor--
					item := a.items.selectedItem()
					a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot, a.catalog)
					a.detail.overrides = a.catalog.OverridesFor(item.Name, item.Type)
					a.detail.width = a.contentWidth()
					a.detail.height = a.panelHeight()
					a.detail.fileViewer.splitView.width = a.contentWidth()
					a.detail.loadoutContents.splitView.width = a.contentWidth()
					a.detail.listPosition = a.items.cursor
					a.detail.listTotal = len(a.items.items)
				}
				return a, nil
			}
			if key.Matches(msg, keys.Delete) && isRemovable(a.detail.item) {
				installed := a.detail.installedProviders()
				var body string
				if len(installed) > 0 {
					var names []string
					for _, p := range installed {
						names = append(names, p.Name)
					}
					body = fmt.Sprintf("Remove %q from your library?\n\nThis will uninstall from %s\nand delete the content directory.", a.detail.item.Name, strings.Join(names, ", "))
				} else {
					body = fmt.Sprintf("Remove %q from your library?\n\nThis will delete the content directory.", a.detail.item.Name)
				}
				a.modal = newConfirmModal("Remove from Library", body)
				a.modal.purpose = modalItemRemove
				a.focus = focusModal
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
				a.rebuildItems()
				// Cache detail state for re-entry
				cached := a.detail
				a.cachedDetail = &cached
				a.cachedDetailPath = a.detail.item.Path
				a.screen = screenItems
				return a, nil
			}
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			a.promoteDetailMessage()
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
				a.screen = screenCategory
				a.focus = focusSidebar
				return a, nil
			}
			var cmd tea.Cmd
			a.settings, cmd = a.settings.Update(msg)
			a.promoteSettingsMessage()
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
			a.promoteSandboxMessage()
			return a, cmd

		case screenRegistries:
			if key.Matches(msg, keys.Back) {
				a.screen = screenCategory
				a.focus = focusSidebar
				return a, nil
			}
			cols := 2
			if a.width < 80 {
				cols = 1
			}
			if key.Matches(msg, keys.Up) {
				if a.cardCursor >= cols {
					a.cardCursor -= cols
				}
				return a, nil
			}
			if key.Matches(msg, keys.Down) {
				if a.cardCursor+cols < len(a.registries.entries) {
					a.cardCursor += cols
				}
				return a, nil
			}
			if key.Matches(msg, keys.Left) {
				if a.cardCursor > 0 {
					a.cardCursor--
				}
				return a, nil
			}
			if key.Matches(msg, keys.Right) {
				if a.cardCursor+1 < len(a.registries.entries) {
					a.cardCursor++
				}
				return a, nil
			}
			if key.Matches(msg, keys.Enter) && len(a.registries.entries) > 0 {
				entry := a.registries.entries[a.cardCursor]
				regItems := a.visibleItems(a.catalog.ByRegistry(entry.name))
				items := newItemsModel(catalog.SearchResults, regItems, a.providers, a.catalog.RepoRoot)
				items.ctx.sourceRegistry = entry.name
				items.width = a.width - sidebarWidth - 1
				items.height = a.panelHeight()
				a.items = items
				a.cardParent = 0
				a.screen = screenItems
				a.focus = focusContent
				return a, nil
			}
			if key.Matches(msg, keys.Add) && !a.registryOpInProgress {
				a.registryAddModal = newRegistryAddModal()
				a.focus = focusModal
				return a, nil
			}
			if key.Matches(msg, keys.Delete) && len(a.registries.entries) > 0 && !a.registryOpInProgress {
				entry := a.registries.entries[a.cardCursor]
				body := fmt.Sprintf("Remove registry %q?\n\nThis deletes the local clone.\nInstalled content is not affected.", entry.name)
				a.modal = newConfirmModal("Remove Registry", body)
				a.modal.purpose = modalRegistryRemove
				a.focus = focusModal
				return a, nil
			}
			if key.Matches(msg, keys.Refresh) && len(a.registries.entries) > 0 && !a.registryOpInProgress {
				entry := a.registries.entries[a.cardCursor]
				a.toast.show(toastMsg{text: fmt.Sprintf("Syncing %s...", entry.name), isProgress: true})
				a.registryOpInProgress = true
				syncCmd := func() tea.Msg {
					err := registry.Sync(entry.name)
					return registrySyncDoneMsg{name: entry.name, err: err}
				}
				return a, tea.Batch(syncCmd, a.toast.tickSpinner())
			}
			return a, nil
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
		contentView, a.cardScrollOffset = a.registries.View(a.cardCursor, a.cardScrollOffset)
	case screenSandbox:
		contentView = a.sandboxSettings.View()
	case screenLibraryCards:
		contentView = a.renderLibraryCards()
	case screenLoadoutCards:
		contentView = a.renderLoadoutCards()
	case screenCreateLoadout:
		contentView = a.createLoadout.View()
	default:
		// screenCategory: sidebar is the primary UI; show welcome guidance in content
		contentView = a.renderContentWelcome()
	}

	// Help overlay replaces the content view entirely
	if a.helpOverlay.active {
		contentView = a.helpOverlay.View(a.screen)
	}

	// Constrain content to the same height as the sidebar so panels align.
	// Height pads short content; MaxHeight truncates long content.
	contentView = lipgloss.NewStyle().Height(a.panelHeight()).MaxHeight(a.panelHeight()).Render(contentView)

	// Compose sidebar + content side by side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, contentView)

	// Footer: context-sensitive help on the left, breadcrumb on the right
	footer := a.renderFooter()

	// Search overlay replaces the footer
	if a.search.active {
		footer = a.search.View()
	}

	body := lipgloss.JoinVertical(lipgloss.Left, panels, footer)

	// Toast overlay (after panels, before modals)
	if a.toast.active {
		toastView := a.toast.view()
		if toastView != "" {
			// Position toast at bottom-center of the content pane
			body = a.overlayToast(body, toastView)
		}
	}

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

	if a.registryAddModal.active {
		body = a.registryAddModal.overlayView(body)
	}

	return zone.Scan(body)
}

// promoteDetailMessage checks if the detail model has a pending message
// and promotes it to the centralized toast system.
func (a *App) promoteDetailMessage() {
	if a.detail.message != "" {
		a.toast.show(toastMsg{text: a.detail.message, isErr: a.detail.messageIsErr})
		a.detail.message = ""
		a.detail.messageIsErr = false
	}
}

// promoteSettingsMessage checks if the settings model has a pending message.
func (a *App) promoteSettingsMessage() {
	if a.settings.message != "" {
		a.toast.show(toastMsg{text: a.settings.message, isErr: a.settings.messageIsErr})
		a.settings.message = ""
		a.settings.messageIsErr = false
	}
}

// promoteSandboxMessage checks if the sandbox settings model has a pending message.
func (a *App) promoteSandboxMessage() {
	if a.sandboxSettings.message != "" {
		a.toast.show(toastMsg{text: a.sandboxSettings.message, isErr: a.sandboxSettings.messageIsErr})
		a.sandboxSettings.message = ""
		a.sandboxSettings.messageIsErr = false
	}
}

// overlayToast positions the toast at bottom-center of the body.
func (a App) overlayToast(body, toastView string) string {
	return overlay.Composite(toastView, body, overlay.Center, overlay.Bottom, 0, -2)
}

// categoryDesc maps content types to short descriptions for the welcome cards.
var categoryDesc = map[catalog.ContentType]string{
	catalog.Skills:   "Reusable skill definitions that extend AI tool capabilities",
	catalog.Agents:   "Agent configurations with specialized roles and personalities",
	catalog.MCP:      "Model Context Protocol server configurations",
	catalog.Rules:    "Rule files that guide AI coding tool behavior",
	catalog.Hooks:    "Event-driven hooks for automation and validation",
	catalog.Commands: "Custom slash commands for AI coding tools",
	catalog.Loadouts: "Curated content bundles for quick session setup",
}

// sylSplitCols defines the rune column where "SYL" ends on each line of syllagoFontRaw.
// Left of the split (SYL) gets title/mint color, right (LAGO) gets accent/purple color.
// The split falls in the whitespace gap between the first "l" and the second "l" on every row.
// Note: line 0 loses its leading space via TrimSpace, shifting columns left by 1.
var sylSplitCols = []int{28, 28, 28, 28, 28, 28, 29, 28, 22, 21, 20}

// renderSyllagoArt renders the ASCII art with "SYL" in mint and "LAGO" in purple,
// centered within the given width.
func renderSyllagoArt(width int) string {
	art := strings.Trim(syllagoFontRaw, "\n")
	lines := strings.Split(art, "\n")

	// Find the widest line (in runes, since all chars are single-width display).
	maxW := 0
	for _, line := range lines {
		if w := len([]rune(line)); w > maxW {
			maxW = w
		}
	}
	pad := (width - maxW) / 2
	if pad < 0 {
		pad = 0
	}
	padStr := strings.Repeat(" ", pad)

	var result []string
	for i, line := range lines {
		runes := []rune(line)
		// Pad line to maxW so all lines have uniform width.
		if len(runes) < maxW {
			runes = append(runes, []rune(strings.Repeat(" ", maxW-len(runes)))...)
		}
		var styled string
		if i >= len(sylSplitCols) || sylSplitCols[i] >= len(runes) {
			styled = titleStyle.Render(string(runes))
		} else {
			col := sylSplitCols[i]
			styled = titleStyle.Render(string(runes[:col])) + artAccentStyle.Render(string(runes[col:]))
		}
		result = append(result, padStr+styled)
	}
	return strings.Join(result, "\n")
}

// syllagoFontRaw is the "Syllago" text in block letter style.
// Leading newline ensures TrimSpace doesn't eat alignment spaces on line 0.
const syllagoFontRaw = `
  █████████             ████  ████
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

	// Content types match sidebar (excludes Loadouts — it appears in Collections)
	contentTypes := a.sidebar.types
	// Use sidebar counts (already filtered by showHidden state)
	counts := a.sidebar.counts
	if counts == nil {
		counts = make(map[catalog.ContentType]int)
	}

	collectionItems := []welcomeCollectionItem{
		{"Library", a.sidebar.libraryCount, "All content across categories in one view", "welcome-library"},
		{"Loadouts", a.sidebar.loadoutsCount, "Curated content bundles for quick session setup", "welcome-loadouts"},
		{"Registries", a.sidebar.registryCount, "Manage git-based content sources from your team or organization", "welcome-registries"},
	}

	configItems := []welcomeConfigItem{
		{"Add", "Add content from providers, local files, or git repos", "welcome-add"},
		{"Update", "Check for updates and pull latest changes", "welcome-update"},
		{"Settings", "Configure paths and providers", "welcome-settings"},
		{"Sandbox", "Isolated environment for testing content", "welcome-sandbox"},
	}

	// Cards need ~33 lines of panel height. With ASCII art (+13), ~46 total.
	// panelHeight = height - 2, so cards fit at height >= 35, art+cards at >= 48.
	useCards := a.height >= 35

	// Track how many lines the header (art) consumes so cards get the correct available height.
	headerLines := 0

	// --- ASCII art title (only when cards are showing and terminal is large) ---
	if useCards && a.height >= 48 && contentW >= 75 {
		art := renderSyllagoArt(contentW)
		s += art + "\n\n"
		headerLines = strings.Count(art, "\n") + 2 // newlines within art + 2 for "\n\n" suffix
	}

	if useCards {
		s += a.renderWelcomeCards(contentTypes, counts, contentW, collectionItems, configItems, headerLines)
	} else {
		s += a.renderWelcomeList(contentTypes, counts, contentW, collectionItems, configItems)
	}

	return s
}

// welcomeCollectionItem describes a collection entry on the welcome page.
type welcomeCollectionItem struct {
	label  string
	count  int
	desc   string
	zoneID string
}

// welcomeConfigItem describes a configuration entry on the welcome page.
type welcomeConfigItem struct {
	label  string
	desc   string
	zoneID string
}

// welcomeCardCount returns the total number of welcome cards across all three sections.
func (a App) welcomeCardCount() int {
	return len(a.sidebar.types) + 3 + 4 // content types + collections + config
}

// activateWelcomeCard triggers the action for the welcome card at a.cardCursor.
// Cards are laid out as: [content types...] [library, loadouts, registries] [add, update, settings, sandbox].
func (a *App) activateWelcomeCard() {
	nContent := len(a.sidebar.types)
	idx := a.cardCursor

	if idx < nContent {
		// Content type card — navigate via sidebar
		a.sidebar.cursor = idx
		a.focus = focusSidebar
		// Simulate Enter to drill into the selected content type
		ct := a.sidebar.selectedType()
		src := a.visibleItems(a.catalog.ByType(ct))
		items := newItemsModel(ct, src, a.providers, a.catalog.RepoRoot)
		items.ctx.hiddenCount = countHidden(a.catalog.ByType(ct))
		items.width = a.width - sidebarWidth - 1
		items.height = a.panelHeight()
		a.items = items
		a.cardParent = 0
		a.screen = screenItems
		a.focus = focusContent
		return
	}
	idx -= nContent

	// Collection cards: Library, Loadouts, Registries
	collectionSidebarMap := []int{
		a.sidebar.libraryIdx(),
		a.sidebar.loadoutsIdx(),
		a.sidebar.registriesIdx(),
	}
	if idx < 3 {
		a.sidebar.cursor = collectionSidebarMap[idx]
		a.focus = focusSidebar
		// Use the same dispatch as sidebar Enter
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
		*a = m.(App)
		return
	}
	idx -= 3

	// Config cards: Add, Update, Settings, Sandbox
	configSidebarMap := []int{
		a.sidebar.addIdx(),
		a.sidebar.updateIdx(),
		a.sidebar.settingsIdx(),
		a.sidebar.sandboxIdx(),
	}
	if idx < 4 {
		a.sidebar.cursor = configSidebarMap[idx]
		a.focus = focusSidebar
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
		*a = m.(App)
		return
	}
}

// renderWelcomeCards renders the three-section welcome page as bordered cards.
func (a App) renderWelcomeCards(contentTypes []catalog.ContentType, counts map[catalog.ContentType]int, contentW int, collectionItems []welcomeCollectionItem, configItems []welcomeConfigItem, headerLines int) string {
	var s string

	singleCol := contentW < 42
	cardW := (contentW - 5) / 2 // 5 = 2 borders left + 2 borders right + 1 gap
	if singleCol {
		cardW = contentW - 2
	}
	if cardW < 18 {
		cardW = 18
	}

	normalStyle := cardNormalStyle.Width(cardW)
	selectedStyle := cardSelectedStyle.Width(cardW)
	if !singleCol {
		normalStyle = normalStyle.Height(3)
		selectedStyle = selectedStyle.Height(3)
	}

	// cardStyleFor returns selected or normal card style based on focus and cursor.
	focused := a.focus == focusContent
	cardStyleFor := func(flatIdx int) lipgloss.Style {
		if focused && flatIdx == a.cardCursor {
			return selectedStyle
		}
		return normalStyle
	}

	collOffset := len(contentTypes)
	cfgOffset := collOffset + len(collectionItems)

	renderCardAt := func(flatIdx int) string {
		if flatIdx < collOffset {
			ct := contentTypes[flatIdx]
			inner := renderCategoryCardInner(ct, counts[ct])
			return zone.Mark(fmt.Sprintf("welcome-%d", flatIdx), cardStyleFor(flatIdx).Render(inner))
		} else if flatIdx < cfgOffset {
			ci := collectionItems[flatIdx-collOffset]
			inner := renderCollectionCardInner(ci.label, ci.count, ci.desc)
			return zone.Mark(ci.zoneID, cardStyleFor(flatIdx).Render(inner))
		} else {
			ci := configItems[flatIdx-cfgOffset]
			inner := labelStyle.Render(ci.label) + "\n" + helpStyle.Render(ci.desc)
			return zone.Mark(ci.zoneID, cardStyleFor(flatIdx).Render(inner))
		}
	}

	// Build per-section row ranges. Each section renders its own cards independently,
	// so odd-count sections don't bleed into the next section's row.
	type sectionInfo struct {
		label string
		start int // first flat index
		count int // number of cards
	}
	sections := []sectionInfo{
		{"Content", 0, len(contentTypes)},
		{"Collections", collOffset, len(collectionItems)},
		{"Configuration", cfgOffset, len(configItems)},
	}

	// Compute total visual rows across all sections.
	cols := 2
	if singleCol {
		cols = 1
	}
	totalCards := len(contentTypes) + len(collectionItems) + len(configItems)

	// For scroll, treat all cards as flat and use cardScrollRange to determine viewport.
	// Subtract headerLines (ASCII art + spacing) so cards fit within the remaining panel space.
	cardRowHeight := 6
	availH := a.panelHeight() - headerLines
	if availH < 6 {
		availH = 6
	}
	_, _, newOffset := cardScrollRange(a.cardCursor, totalCards, cols, availH, cardRowHeight, a.cardScrollOffset)
	a.cardScrollOffset = newOffset

	// Render all sections, then do line-based viewport clipping.
	var body string
	for si, sec := range sections {
		if sec.count == 0 {
			continue
		}
		if si > 0 {
			body += "\n"
		}
		body += labelStyle.Render("  "+sec.label) + "\n\n"
		if singleCol {
			for i := 0; i < sec.count; i++ {
				body += renderCardAt(sec.start+i) + "\n"
			}
		} else {
			for i := 0; i < sec.count; i += 2 {
				left := renderCardAt(sec.start + i)
				var right string
				if i+1 < sec.count {
					right = renderCardAt(sec.start + i + 1)
				}
				body += lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n"
			}
		}
	}

	// Line-based scroll clipping
	lines := strings.Split(body, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > availH {
		// Cursor-following: estimate which line the cursor card is on
		cursorLine := 0
		cardsSeen := 0
		for i, line := range lines {
			_ = line
			// Approximate: each card row is ~cardRowHeight lines, section headers are 2-3 lines
			if cardsSeen > a.cardCursor {
				break
			}
			cursorLine = i
			// Count rendered cards by looking for zone marks (rough heuristic)
			if strings.Contains(line, "╭") || strings.Contains(line, "welcome-") || strings.Contains(line, "loadout-card") {
				cardsSeen += cols
			}
		}
		startLine := newOffset * cardRowHeight
		if startLine > cursorLine {
			startLine = cursorLine
		}

		// Reserve lines for scroll indicators within availH so total output fits the panel.
		viewportH := availH
		hasAbove := startLine > 0
		hasBelow := true // we know len(lines) > availH
		if hasAbove {
			viewportH--
		}
		if hasBelow {
			viewportH--
		}
		if viewportH < 1 {
			viewportH = 1
		}

		if startLine+viewportH > len(lines) {
			startLine = len(lines) - viewportH
		}
		if startLine < 0 {
			startLine = 0
		}
		endLine := startLine + viewportH
		if endLine > len(lines) {
			endLine = len(lines)
		}
		if startLine > 0 {
			s += "  " + renderScrollUp(startLine, true) + "\n"
		}
		s += strings.Join(lines[startLine:endLine], "\n") + "\n"
		remaining := len(lines) - endLine
		if remaining > 0 {
			s += "  " + renderScrollDown(remaining, true) + "\n"
		}
	} else {
		s += body
	}

	return s
}

// renderWelcomeList renders a compact text list when cards won't fit.
func (a App) renderWelcomeList(contentTypes []catalog.ContentType, counts map[catalog.ContentType]int, contentW int, collectionItems []welcomeCollectionItem, configItems []welcomeConfigItem) string {
	var s string

	// ── Content ──
	s += labelStyle.Render("  Content") + "\n"
	for i, ct := range contentTypes {
		line := renderCategoryLine(ct, counts[ct], contentW)
		s += zone.Mark(fmt.Sprintf("welcome-%d", i), line) + "\n"
	}

	// ── Collections ──
	s += "\n"
	s += labelStyle.Render("  Collections") + "\n"
	for _, ci := range collectionItems {
		countStr := fmt.Sprintf("(%d)", ci.count)
		line := fmt.Sprintf("  %-12s %s", ci.label, countStr)
		if len(line) > contentW {
			line = line[:contentW]
		}
		s += zone.Mark(ci.zoneID, line) + "\n"
	}

	// ── Configuration ──
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

// renderCollectionCardInner builds the inner content for a collection card.
func renderCollectionCardInner(label string, count int, desc string) string {
	title := labelStyle.Render(label) + " " + countStyle.Render(fmt.Sprintf("(%d)", count))
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

// libraryCardTypes returns the content types that have library items, in display order.
func (a App) libraryCardTypes() []catalog.ContentType {
	typeCounts := make(map[catalog.ContentType]int)
	for _, item := range a.visibleItems(a.catalog.Items) {
		if item.Library {
			typeCounts[item.Type]++
		}
	}
	// Preserve AllContentTypes display order, only include types with items.
	var result []catalog.ContentType
	for _, ct := range catalog.AllContentTypes() {
		if typeCounts[ct] > 0 {
			result = append(result, ct)
		}
	}
	return result
}

// loadoutCardProviders returns providers to show on the loadouts card screen.
// Shows all detected providers — even those with no loadouts — so the grid
// is always populated and the user can create loadouts for any provider.
func (a App) loadoutCardProviders() []string {
	hasLoadouts := make(map[string]bool)
	for _, item := range a.catalog.ByType(catalog.Loadouts) {
		if item.Provider != "" {
			hasLoadouts[item.Provider] = true
		}
	}
	var result []string
	for _, prov := range a.providers {
		if prov.Detected {
			result = append(result, prov.Slug)
		}
	}
	if len(result) == 0 {
		for slug := range hasLoadouts {
			result = append(result, slug)
		}
		sort.Strings(result)
	}
	return result
}

// renderLibraryCards renders a card grid grouping library items by content type.
func (a App) renderLibraryCards() string {
	cards := a.libraryCardTypes()
	if len(cards) == 0 {
		return "\n" + helpStyle.Render("  No library items found.")
	}

	// Count items per type
	typeCounts := make(map[catalog.ContentType]int)
	for _, item := range a.visibleItems(a.catalog.Items) {
		if item.Library {
			typeCounts[item.Type]++
		}
	}

	contentW := a.width - sidebarWidth - 1
	var s string
	s += renderBreadcrumb(
		BreadcrumbSegment{"Home", "crumb-home"},
		BreadcrumbSegment{"Library", ""},
	) + "\n"
	s += renderActionButtons(
		ActionButton{"a", "Add Content", "action-a", actionBtnAddStyle},
	) + "\n"

	singleCol := contentW < 42
	cardW := (contentW - 5) / 2
	if singleCol {
		cardW = contentW - 2
	}
	if cardW < 18 {
		cardW = 18
	}

	cardBase := cardNormalStyle.Width(cardW)
	if !singleCol {
		cardBase = cardBase.Height(3)
	}
	cardSel := cardSelectedStyle.Width(cardW)
	if !singleCol {
		cardSel = cardSel.Height(3)
	}

	renderCard := func(idx int, ct catalog.ContentType) string {
		inner := labelStyle.Render(ct.Label()) + " " + countStyle.Render(fmt.Sprintf("(%d)", typeCounts[ct]))
		desc := categoryDesc[ct]
		if desc == "" {
			desc = "Browse " + ct.Label() + " content"
		}
		inner += "\n" + helpStyle.Render(desc)

		style := cardBase
		if idx == a.cardCursor {
			style = cardSel
		}
		return zone.Mark(fmt.Sprintf("library-card-%s", ct), style.Render(inner))
	}

	cols := 2
	if singleCol {
		cols = 1
	}
	totalRows := (len(cards) + cols - 1) / cols
	cardRowHeight := 6
	headerLines := 3
	availH := a.panelHeight() - headerLines
	firstRow, visibleRows, newOffset := cardScrollRange(a.cardCursor, len(cards), cols, availH, cardRowHeight, a.cardScrollOffset)
	a.cardScrollOffset = newOffset

	if firstRow > 0 {
		s += "  " + renderScrollUp(firstRow*cols, false) + "\n"
	}

	lastRow := firstRow + visibleRows
	if lastRow > totalRows {
		lastRow = totalRows
	}

	if singleCol {
		for row := firstRow; row < lastRow; row++ {
			if row >= len(cards) {
				break
			}
			s += renderCard(row, cards[row]) + "\n"
		}
	} else {
		for row := firstRow; row < lastRow; row++ {
			i := row * 2
			if i >= len(cards) {
				break
			}
			left := renderCard(i, cards[i])
			var right string
			if i+1 < len(cards) {
				right = renderCard(i+1, cards[i+1])
			}
			s += lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n"
		}
	}

	hiddenBelow := len(cards) - lastRow*cols
	if hiddenBelow < 0 {
		hiddenBelow = 0
	}
	if hiddenBelow > 0 {
		s += "  " + renderScrollDown(hiddenBelow, false) + "\n"
	}

	return s
}

// renderLoadoutCards renders a card grid grouping loadout items by provider.
func (a App) renderLoadoutCards() string {
	providers := a.loadoutCardProviders()
	if len(providers) == 0 {
		return "\n" + helpStyle.Render("  No loadouts found.")
	}

	// Count items per provider
	provCounts := make(map[string]int)
	for _, item := range a.visibleItems(a.catalog.ByType(catalog.Loadouts)) {
		provCounts[item.Provider]++
	}

	contentW := a.width - sidebarWidth - 1
	var s string
	s += renderBreadcrumb(
		BreadcrumbSegment{"Home", "crumb-home"},
		BreadcrumbSegment{"Loadouts", ""},
	) + "\n"
	s += renderActionButtons(
		ActionButton{"a", "Create Loadout", "action-a", actionBtnAddStyle},
	) + "\n"

	singleCol := contentW < 42
	cardW := (contentW - 5) / 2
	if singleCol {
		cardW = contentW - 2
	}
	if cardW < 18 {
		cardW = 18
	}

	cardBase := cardNormalStyle.Width(cardW)
	if !singleCol {
		cardBase = cardBase.Height(3)
	}
	cardSel := cardSelectedStyle.Width(cardW)
	if !singleCol {
		cardSel = cardSel.Height(3)
	}

	renderCard := func(idx int, prov string) string {
		count := provCounts[prov]
		name := providerDisplayName(prov)
		var inner string
		if count > 0 {
			inner = labelStyle.Render(name) + " " + countStyle.Render(fmt.Sprintf("(%d)", count))
			inner += "\n" + helpStyle.Render("Loadouts for "+name)
		} else {
			inner = labelStyle.Render(name)
			inner += "\n" + helpStyle.Render("No loadouts")
		}

		style := cardBase
		if idx == a.cardCursor {
			style = cardSel
		}
		return zone.Mark(fmt.Sprintf("loadout-card-%s", prov), style.Render(inner))
	}

	cols := 2
	if singleCol {
		cols = 1
	}
	totalRows := (len(providers) + cols - 1) / cols
	cardRowHeight := 6
	headerLines := 3
	availH := a.panelHeight() - headerLines
	firstRow, visibleRows, newOffset := cardScrollRange(a.cardCursor, len(providers), cols, availH, cardRowHeight, a.cardScrollOffset)
	a.cardScrollOffset = newOffset

	if firstRow > 0 {
		s += "  " + renderScrollUp(firstRow*cols, false) + "\n"
	}

	lastRow := firstRow + visibleRows
	if lastRow > totalRows {
		lastRow = totalRows
	}

	if singleCol {
		for row := firstRow; row < lastRow; row++ {
			if row >= len(providers) {
				break
			}
			s += renderCard(row, providers[row]) + "\n"
		}
	} else {
		for row := firstRow; row < lastRow; row++ {
			i := row * 2
			if i >= len(providers) {
				break
			}
			left := renderCard(i, providers[i])
			var right string
			if i+1 < len(providers) {
				right = renderCard(i+1, providers[i+1])
			}
			s += lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n"
		}
	}

	hiddenBelow := len(providers) - lastRow*cols
	if hiddenBelow < 0 {
		hiddenBelow = 0
	}
	if hiddenBelow > 0 {
		s += "  " + renderScrollDown(hiddenBelow, false) + "\n"
	}

	return s
}

// contextHelpText returns the appropriate help text for the current screen state.
func (a App) contextHelpText() string {
	switch a.screen {
	case screenCategory:
		return "tab switch panel • / search • ? help • q quit"
	case screenDetail:
		return a.detail.helpText()
	case screenItems:
		return a.items.helpText()
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
	case screenLibraryCards:
		return "arrows navigate • enter browse • a add content • esc back"
	case screenLoadoutCards:
		return "arrows navigate • enter browse • a create loadout • esc back"
	case screenCreateLoadout:
		switch a.createLoadout.step {
		case clStepItems:
			return "space toggle • a toggle all • t filter • / search • h/l panes • enter next • esc back"
		case clStepName:
			return "tab switch field • enter next • esc back"
		case clStepReview:
			return "left/right buttons • enter confirm • esc back"
		default:
			return "up/down navigate • enter next • esc back"
		}
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
		parent := a.itemsBreadcrumb()
		return parent + " > " + displayName(a.detail.item)
	case screenItems:
		return a.itemsBreadcrumb()
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
	case screenLibraryCards:
		return "Library"
	case screenLoadoutCards:
		return "Loadouts"
	case screenCreateLoadout:
		return "Loadouts > Create"
	default:
		return "syllago"
	}
}

// itemsBreadcrumb returns the breadcrumb for the items screen, including
// the parent context (Library, Loadouts, Registries) when applicable.
func (a App) itemsBreadcrumb() string {
	if a.items.ctx.sourceRegistry != "" {
		return "Registries > " + a.items.ctx.sourceRegistry
	}
	label := a.items.contentType.Label()
	switch a.cardParent {
	case screenLibraryCards:
		return "Library > " + label
	case screenLoadoutCards:
		if a.items.ctx.sourceProvider != "" {
			return "Loadouts > " + providerDisplayName(a.items.ctx.sourceProvider)
		}
		return "Loadouts > " + label
	default:
		return label
	}
}
