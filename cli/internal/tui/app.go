package tui

import (
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// wizardKind identifies the active full-screen wizard (if any).
type wizardKind int

const (
	wizardNone    wizardKind = iota
	wizardInstall            // install wizard (B3+)
	wizardAdd                // add wizard (Phase D)
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
	topBar         topBarModel
	library        libraryModel  // Library tab: table + drill-in
	explorer       explorerModel // Content/Loadout tabs: items list + preview
	gallery        galleryModel  // Loadouts/Registries tabs: card grid + contents sidebar
	helpBar        helpBarModel
	modal          editModal           // reusable edit overlay (name + description)
	confirm        confirmModal        // reusable confirm overlay (uninstall + simple confirms)
	remove         removeModal         // multi-step remove overlay (library item removal)
	help           helpOverlay         // keyboard shortcut reference (? key)
	toast          toastModel          // bottom-right notification overlay
	registryAdd    registryAddModal    // registry add overlay
	trustInspector trustInspectorModel // reusable trust inspector (library + registries)

	// Wizard mode — when active, captures all key/mouse input
	wizardMode    wizardKind
	installWizard *installWizardModel // nil when not active
	addWizard     *addWizardModel     // nil when not active

	// Publisher-warn install gate: when the wizard emits an install for a
	// publisher-revoked item, we stash the pending result here and open the
	// shared confirmModal. On confirm, handleConfirmResult dispatches the
	// stashed install; on cancel, it is cleared. Exactly one of pendingInstall
	// / pendingInstallAll is non-nil at a time.
	pendingInstall    *installResultMsg
	pendingInstallAll *installAllResultMsg

	// MOAT install-gate state. moatSession persists for the TUI run so a
	// publisher-warn acknowledgement survives rescans (ADR 0007 G-8 warn-
	// once-per-session); rebuilding it on 'R' would re-prompt on every redraw.
	// moatGate and moatLockfile are refreshed in rescanCatalog so they track
	// the freshest manifest + lockfile. moatMinTier is the project's policy
	// floor; defaults to TrustTierUnsigned (accept any tier) to match
	// moatInstallMinTier in the CLI install path.
	moatSession  *moat.Session
	moatGate     *moat.GateInputs
	moatLockfile *moat.Lockfile
	moatMinTier  moat.TrustTier

	// pendingGateKind disambiguates which MarkConfirmed variant to call when
	// the user confirms the stashed install. PublisherWarn and PrivatePrompt
	// both route through the same confirmModal but consume different Session
	// suppression keys (publisher:hash vs private:registry+hash).
	pendingGateKind        pendingGateKind
	pendingGateRegistryURL string
	pendingGateContentHash string

	// Dimensions
	width, height int

	// State
	ready                bool   // false until first WindowSizeMsg
	galleryDrillIn       bool   // true when viewing card contents as a library
	galleryDrillCard     string // name of the card we drilled into (for breadcrumbs)
	registryOpInProgress bool   // true during async registry operation (add/sync/remove)
	telemetryNotice      bool   // true if first-run telemetry notice should be shown as toast
}

// NewApp creates a new TUI app. Signature matches main.go.
func NewApp(cat *catalog.Catalog, providers []provider.Provider, version string, autoUpdate bool, registrySources []catalog.RegistrySource, cfg *config.Config, isReleaseBuild bool, contentRoot, projectRoot string) App {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if cat == nil {
		cat = &catalog.Catalog{}
	}

	// Check if first-run telemetry notice should display as a TUI toast.
	// PersistentPreRun already called telemetry.Init() which sets noticeSeen=true
	// on the CLI path, so this will only be true if noticeSeen was already false
	// before Init() ran (i.e., first-ever launch).
	showTelemetryNotice := false
	telemetryCfg := telemetry.Status()
	if !telemetryCfg.NoticeSeen && telemetryCfg.Enabled {
		showTelemetryNotice = true
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
		registryAdd:     newRegistryAddModal(),
		trustInspector:  newTrustInspectorModel(),

		moatSession: moat.NewSession(),
		moatMinTier: moat.TrustTierUnsigned,

		telemetryNotice: showTelemetryNotice,
	}
	a.updateNavState()
	return a
}

// telemetryNoticeMsg triggers the first-run telemetry toast in Update().
type telemetryNoticeMsg struct{}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	if a.telemetryNotice {
		return func() tea.Msg { return telemetryNoticeMsg{} }
	}
	return nil
}

// contentHeight returns the available height for the main content area.
func (a App) contentHeight() int {
	topBarHeight := a.topBar.Height()
	helpBarHeight := a.helpBar.Height()
	return a.height - topBarHeight - helpBarHeight
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

// isContentTab returns true if the active tab is in the Content group.
func (a App) isContentTab() bool {
	return a.topBar.ActiveGroupLabel() == "Content"
}

// isRegistriesTab returns true if the active tab is Collections > Registries.
func (a App) isRegistriesTab() bool {
	return a.isGalleryTab() && a.topBar.ActiveTabLabel() == "Registries"
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

// catalogReadyMsg carries the result of an async catalog rescan. Produced by
// rescanCatalog's returned command; consumed by handleCatalogReady.
type catalogReadyMsg struct {
	result *moat.ScanResult
	err    error
}

// rescanCatalog kicks off an async re-scan of disk content + MOAT enrichment.
// All I/O (config load, registry enumeration, lockfile read, scan, sigstore
// verification) runs inside the returned tea.Cmd closure so the TUI event
// loop stays responsive — see .claude/rules/tui-elm.md rule #2.
//
// State mutation happens later in handleCatalogReady, which App.Update()
// dispatches when the catalogReadyMsg arrives. Callers just use the returned
// Cmd; they do not observe App changes synchronously.
func (a *App) rescanCatalog() tea.Cmd {
	root := a.contentRoot
	if root == "" {
		root = a.projectRoot
	}
	projectRoot := a.projectRoot
	if projectRoot == "" {
		projectRoot = root
	}
	return func() tea.Msg {
		result, err := moat.LoadAndScan(root, projectRoot, time.Now())
		return catalogReadyMsg{result: result, err: err}
	}
}

// handleCatalogReady applies the async rescan result to App state and emits
// a success/error toast. The galleryDrillIn reset mirrors the previous
// synchronous behavior — a rescan always returns the user to the gallery
// overview.
func (a App) handleCatalogReady(msg catalogReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return a, a.toast.Push("Refresh failed: "+msg.err.Error(), toastError)
	}
	a.catalog = msg.result.Catalog
	a.moatLockfile = msg.result.Lockfile
	a.moatGate = msg.result.GateInputs
	a.registrySources = msg.result.RegistrySources
	a.cfg = msg.result.Config
	a.galleryDrillIn = false
	a.refreshContent()
	a.updateNavState()
	return a, a.toast.Push("Catalog refreshed", toastSuccess)
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

// currentHints returns context-sensitive help hints based on current state.
func (a App) currentHints() []string {
	// Wizard mode: show step-specific hints instead of app hints
	if a.wizardMode == wizardAdd && a.addWizard != nil {
		return a.addWizard.stepHints()
	}
	if a.wizardMode == wizardInstall && a.installWizard != nil {
		return a.installWizard.stepHints()
	}

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
	if a.isRegistriesTab() && !a.galleryDrillIn {
		return append(base, "arrows grid", "enter select", "tab grid/contents", "/ search", "a add", "S sync", "d remove", "e edit", "R refresh", "? help", "q back")
	}
	if a.isGalleryTab() {
		return append(base, "arrows grid", "enter select", "tab grid/contents", "/ search", "e edit", "d remove", "R refresh", "a add", "? help", "q back")
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

	hints := append(base, "↑/↓ navigate", "←/→ switch pane", "enter detail", "/ search")
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
// Also resizes content models when the help bar height changes (e.g. hints
// wrap to two lines or shrink to one) so the total layout fits the terminal.
func (a *App) updateNavState() {
	prevH := a.helpBar.Height()
	a.helpBar.SetHints(a.currentHints())
	a.updateBreadcrumbs()
	if a.helpBar.Height() != prevH {
		a.resizeContent()
	}
}

// resizeContent recalculates sizes for all content models using the current
// content height. Call after anything that changes contentHeight() (window
// resize, help bar height change, etc.).
func (a *App) resizeContent() {
	ch := a.contentHeight()
	a.library.SetSize(a.width, ch)
	a.explorer.SetSize(a.width, ch)
	a.gallery.SetSize(a.width, ch)
	a.help.SetSize(a.width, ch)
	a.toast.SetSize(a.width, ch)
	a.confirm.width = a.width
	a.confirm.height = ch
	a.remove.width = a.width
	a.remove.height = ch
	a.registryAdd.width = a.width
	a.registryAdd.height = ch
	if a.installWizard != nil {
		a.installWizard.width = a.width
		a.installWizard.height = ch
		a.installWizard.shell.SetWidth(a.width)
	}
	if a.addWizard != nil {
		a.addWizard.width = a.width
		a.addWizard.height = ch
		a.addWizard.shell.SetWidth(a.width)
	}
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
