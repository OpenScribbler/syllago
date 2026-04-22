package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// removeDoneMsg is sent when a library item remove operation completes.
type removeDoneMsg struct {
	itemName        string
	uninstalledFrom []string
	err             error
}

// uninstallDoneMsg is sent when a provider uninstall operation completes.
type uninstallDoneMsg struct {
	itemName        string
	uninstalledFrom []string
	err             error
}

// selectedItem returns the currently focused content item, or nil.
// For gallery cards (Loadouts/Registries) that aren't drilled-in, returns nil —
// callers must check isGalleryTab() separately for card-level actions.
func (a App) selectedItem() *catalog.ContentItem {
	if a.isGalleryTab() && a.galleryDrillIn {
		return a.library.table.Selected()
	}
	if a.isGalleryTab() {
		// Gallery cards are cardData, not ContentItem. Card-level actions
		// (loadout remove, registry remove) are handled separately.
		return nil
	}
	if a.isLibraryTab() {
		return a.library.table.Selected()
	}
	return a.explorer.items.Selected()
}

// handleRemove opens the confirm modal for removing a library item.
func (a App) handleRemove() (tea.Model, tea.Cmd) {
	// Gallery card remove (loadouts): simplified confirm, no provider checkboxes.
	if a.isGalleryTab() && !a.galleryDrillIn {
		return a.handleGalleryCardRemove()
	}

	item := a.selectedItem()
	if item == nil {
		return a, nil
	}

	// Only local items (library or content root) can be removed — not registry-only items.
	if !item.Library && item.Registry != "" {
		return a, nil
	}

	// Find installed providers for the multi-step flow.
	var installed []provider.Provider
	for _, prov := range a.providers {
		if installer.CheckStatus(*item, prov, a.projectRoot) == installer.StatusInstalled {
			installed = append(installed, prov)
		}
	}

	a.remove.Open(*item, installed)
	return a, nil
}

// handleGalleryCardRemove handles [d] on gallery cards (loadouts).
func (a App) handleGalleryCardRemove() (tea.Model, tea.Cmd) {
	card := a.gallery.selectedCard()
	if card == nil {
		return a, nil
	}
	tabLabel := a.topBar.ActiveTabLabel()
	if tabLabel == "Loadouts" {
		a.confirm.Open(
			fmt.Sprintf("Remove loadout %q?", card.name),
			"This will delete the loadout from disk.\nThis cannot be undone.",
			"Remove",
			true,
			nil,
		)
		// Construct minimal ContentItem for the remove command.
		a.confirm.item = catalog.ContentItem{
			Name: card.name,
			Path: card.path,
			Type: catalog.Loadouts,
		}
		a.confirm.itemName = card.name
		return a, nil
	}
	if tabLabel == "Registries" {
		if a.registryOpInProgress {
			cmd := a.toast.Push("Registry operation in progress", toastWarning)
			return a, cmd
		}
		a.confirm.Open(
			fmt.Sprintf("Remove registry %q?", card.name),
			"This will delete the local clone.\nInstalled content is not affected.",
			"Remove",
			true,
			nil,
		)
		a.confirm.itemName = card.name
		return a, nil
	}
	return a, nil
}

// handleUninstall opens the confirm modal for uninstalling from a provider.
func (a App) handleUninstall() (tea.Model, tea.Cmd) {
	// [x] Uninstall not available on gallery cards.
	if a.isGalleryTab() && !a.galleryDrillIn {
		return a, nil
	}

	item := a.selectedItem()
	if item == nil {
		return a, nil
	}

	var installedProviders []provider.Provider
	for _, prov := range a.providers {
		if installer.CheckStatus(*item, prov, a.projectRoot) == installer.StatusInstalled {
			installedProviders = append(installedProviders, prov)
		}
	}

	if len(installedProviders) == 0 {
		cmd := a.toast.Push("Not installed in any provider", toastWarning)
		return a, cmd
	}

	if len(installedProviders) == 1 {
		prov := installedProviders[0]
		a.confirm.OpenForItem(
			fmt.Sprintf("Uninstall %q?", item.DisplayName),
			fmt.Sprintf("Remove from: %s\nContent stays in your library.", prov.Name),
			"Uninstall",
			false,
			nil,
			*item,
		)
		a.confirm.uninstallProviders = installedProviders
		return a, nil
	}

	// Multiple providers — show checkboxes.
	var checks []confirmCheckbox
	for _, prov := range installedProviders {
		checks = append(checks, confirmCheckbox{
			label:   "Uninstall from " + prov.Name,
			checked: true,
		})
	}

	a.confirm.OpenForItem(
		fmt.Sprintf("Uninstall %q?", item.DisplayName),
		"Content stays in your library.",
		"Uninstall",
		false,
		checks,
		*item,
	)
	a.confirm.uninstallProviders = installedProviders
	return a, nil
}

// handleConfirmResult handles confirmModal results (uninstall + loadout simple removes).
//
// MOAT install-gate stash has priority: if the operator just answered a
// publisher-warn or private-prompt, dispatch the stashed install (recording
// the MarkConfirmed so subsequent same-session installs of the same
// (registry, hash) skip the modal) or cancel. Only one of pendingInstall /
// pendingInstallAll is ever set (see handleInstallResult /
// handleInstallAllResult).
func (a App) handleConfirmResult(msg confirmResultMsg) (tea.Model, tea.Cmd) {
	if a.pendingInstall != nil || a.pendingInstallAll != nil {
		kind := a.pendingGateKind
		registryURL := a.pendingGateRegistryURL
		contentHash := a.pendingGateContentHash

		pendingSingle := a.pendingInstall
		pendingAll := a.pendingInstallAll
		a.pendingInstall = nil
		a.pendingInstallAll = nil
		a.pendingGateKind = gateKindNone
		a.pendingGateRegistryURL = ""
		a.pendingGateContentHash = ""

		if !msg.confirmed {
			return a, a.toast.Push(gateCancelledToastText(kind), toastWarning)
		}
		switch kind {
		case gateKindPublisherWarn:
			installer.MarkPublisherConfirmed(a.moatSession, registryURL, contentHash)
		case gateKindPrivatePrompt:
			installer.MarkPrivateConfirmed(a.moatSession, registryURL, contentHash)
		}
		if pendingSingle != nil {
			return a, a.doInstallCmd(*pendingSingle)
		}
		return a, a.doInstallAllCmd(*pendingAll)
	}

	if !msg.confirmed {
		return a, nil
	}

	// Registry remove: on Registries tab, no ContentItem path, but name is set.
	if a.isRegistriesTab() && msg.item.Path == "" && msg.itemName != "" {
		a.registryOpInProgress = true
		cmd1 := a.toast.Push("Removing registry: "+msg.itemName+"...", toastSuccess)
		cmd2 := a.doRegistryRemoveCmd(msg.itemName)
		return a, tea.Batch(cmd1, cmd2)
	}

	// Loadout remove (simple confirm, no providers).
	if msg.item.Type == catalog.Loadouts && len(msg.checks) == 0 {
		return a, a.doSimpleRemoveCmd(msg.item)
	}

	// Otherwise it's an uninstall.
	return a, a.doUninstallCmd(msg)
}

// handleRemoveResult handles removeModal results (multi-step library item removal).
func (a App) handleRemoveResult(msg removeResultMsg) (tea.Model, tea.Cmd) {
	if !msg.confirmed {
		return a, nil
	}
	return a, a.doRemoveCmd(msg)
}

// doRemoveCmd creates a tea.Cmd that removes a library item, optionally uninstalling first.
func (a App) doRemoveCmd(msg removeResultMsg) tea.Cmd {
	item := msg.item
	targetProviders := msg.uninstallProviders
	repoRoot := a.projectRoot

	return func() tea.Msg {
		var uninstalledFrom []string

		for _, prov := range targetProviders {
			if _, err := installer.Uninstall(item, prov, repoRoot); err != nil {
				continue // best-effort uninstall
			}
			uninstalledFrom = append(uninstalledFrom, prov.Name)
		}

		if err := catalog.RemoveLibraryItem(item.Path); err != nil {
			return removeDoneMsg{itemName: item.Name, err: fmt.Errorf("removing from library: %w", err)}
		}

		return removeDoneMsg{itemName: item.Name, uninstalledFrom: uninstalledFrom}
	}
}

// doSimpleRemoveCmd creates a tea.Cmd that removes an item from disk (no uninstall).
func (a App) doSimpleRemoveCmd(item catalog.ContentItem) tea.Cmd {
	return func() tea.Msg {
		if err := catalog.RemoveLibraryItem(item.Path); err != nil {
			return removeDoneMsg{itemName: item.Name, err: fmt.Errorf("removing: %w", err)}
		}
		return removeDoneMsg{itemName: item.Name}
	}
}

// doUninstallCmd creates a tea.Cmd that uninstalls an item from providers.
func (a App) doUninstallCmd(msg confirmResultMsg) tea.Cmd {
	item := msg.item
	providers := msg.uninstallProviders
	repoRoot := a.projectRoot

	var targetProviders []provider.Provider
	if len(msg.checks) == 0 {
		targetProviders = providers
	} else {
		for i, c := range msg.checks {
			if c.checked && i < len(providers) {
				targetProviders = append(targetProviders, providers[i])
			}
		}
	}

	return func() tea.Msg {
		var uninstalledFrom []string
		var lastErr error
		for _, prov := range targetProviders {
			if _, err := installer.Uninstall(item, prov, repoRoot); err != nil {
				lastErr = err
			} else {
				uninstalledFrom = append(uninstalledFrom, prov.Name)
			}
		}
		if lastErr != nil && len(uninstalledFrom) == 0 {
			return uninstallDoneMsg{itemName: item.Name, err: lastErr}
		}
		return uninstallDoneMsg{itemName: item.Name, uninstalledFrom: uninstalledFrom}
	}
}

// handleRemoveDone processes the result of a remove operation.
func (a App) handleRemoveDone(msg removeDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		cmd := a.toast.Push("Remove failed: "+msg.err.Error(), toastError)
		return a, cmd
	}
	toastText := fmt.Sprintf("Removed %q", msg.itemName)
	if len(msg.uninstalledFrom) > 0 {
		toastText += " (uninstalled from " + strings.Join(msg.uninstalledFrom, ", ") + ")"
	}
	cmd1 := a.toast.Push(toastText, toastSuccess)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
}

// handleUninstallDone processes the result of an uninstall operation.
func (a App) handleUninstallDone(msg uninstallDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		cmd := a.toast.Push("Uninstall failed: "+msg.err.Error(), toastError)
		return a, cmd
	}
	toastText := fmt.Sprintf("Uninstalled %q from %s", msg.itemName, strings.Join(msg.uninstalledFrom, ", "))
	cmd1 := a.toast.Push(toastText, toastSuccess)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
}

// registryAddDoneMsg is sent when a registry add operation completes.
type registryAddDoneMsg struct {
	name string
	err  error
}

// registrySyncDoneMsg is sent when a registry sync operation completes.
type registrySyncDoneMsg struct {
	name string
	err  error
}

// registryRemoveDoneMsg is sent when a registry remove operation completes.
type registryRemoveDoneMsg struct {
	name string
	err  error
}

// handleRegistryAdd opens the registry add modal.
func (a App) handleRegistryAdd() (tea.Model, tea.Cmd) {
	var existingNames []string
	for _, r := range a.cfg.Registries {
		existingNames = append(existingNames, r.Name)
	}
	a.registryAdd.Open(existingNames, a.cfg)
	return a, nil
}

// handleRegistryAddResult receives the registry add modal result.
func (a App) handleRegistryAddResult(msg registryAddMsg) (tea.Model, tea.Cmd) {
	a.registryOpInProgress = true
	cmd1 := a.toast.Push("Adding registry: "+msg.name+"...", toastSuccess)
	cmd2 := a.doRegistryAddCmd(msg)
	return a, tea.Batch(cmd1, cmd2)
}

// doRegistryAddCmd creates a tea.Cmd that clones the registry and updates config.
func (a App) doRegistryAddCmd(msg registryAddMsg) tea.Cmd {
	url := msg.url
	name := msg.name
	ref := msg.ref
	isLocal := msg.isLocal
	return func() tea.Msg {
		if !isLocal {
			if err := registry.Clone(url, name, ref); err != nil {
				return registryAddDoneMsg{name: name, err: err}
			}
		}
		cfg, err := config.LoadGlobal()
		if err != nil {
			return registryAddDoneMsg{name: name, err: fmt.Errorf("loading config: %w", err)}
		}
		cfg.Registries = append(cfg.Registries, config.Registry{Name: name, URL: url, Ref: ref})
		if err := config.SaveGlobal(cfg); err != nil {
			return registryAddDoneMsg{name: name, err: fmt.Errorf("saving config: %w", err)}
		}
		return registryAddDoneMsg{name: name}
	}
}

// handleRegistryAddDone processes the result of a registry add operation.
func (a App) handleRegistryAddDone(msg registryAddDoneMsg) (tea.Model, tea.Cmd) {
	a.registryOpInProgress = false
	if msg.err != nil {
		cmd := a.toast.Push("Add failed: "+msg.err.Error(), toastError)
		return a, cmd
	}
	cmd1 := a.toast.Push("Added registry: "+msg.name, toastSuccess)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
}

// handleSync starts a registry sync operation.
func (a App) handleSync() (tea.Model, tea.Cmd) {
	card := a.gallery.selectedCard()
	if card == nil {
		return a, nil
	}
	name := card.name
	a.registryOpInProgress = true
	cmd1 := a.toast.Push("Syncing "+name+"...", toastSuccess)
	cmd2 := func() tea.Msg {
		err := registry.Sync(name)
		return registrySyncDoneMsg{name: name, err: err}
	}
	return a, tea.Batch(cmd1, tea.Cmd(cmd2))
}

// handleSyncDone processes the result of a registry sync operation.
func (a App) handleSyncDone(msg registrySyncDoneMsg) (tea.Model, tea.Cmd) {
	a.registryOpInProgress = false
	if msg.err != nil {
		cmd := a.toast.Push("Sync failed: "+msg.err.Error(), toastError)
		return a, cmd
	}
	cmd1 := a.toast.Push("Synced "+msg.name, toastSuccess)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
}

// handleRegistryRemoveDone processes the result of a registry remove operation.
func (a App) handleRegistryRemoveDone(msg registryRemoveDoneMsg) (tea.Model, tea.Cmd) {
	a.registryOpInProgress = false
	if msg.err != nil {
		cmd := a.toast.Push("Remove failed: "+msg.err.Error(), toastError)
		return a, cmd
	}
	cmd1 := a.toast.Push("Removed registry: "+msg.name, toastSuccess)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
}

// doRegistryRemoveCmd creates a tea.Cmd that removes a registry clone and updates config.
func (a App) doRegistryRemoveCmd(name string) tea.Cmd {
	return func() tea.Msg {
		if err := registry.Remove(name); err != nil {
			return registryRemoveDoneMsg{name: name, err: fmt.Errorf("removing clone: %w", err)}
		}
		cfg, err := config.LoadGlobal()
		if err != nil {
			return registryRemoveDoneMsg{name: name, err: fmt.Errorf("loading config: %w", err)}
		}
		filtered := make([]config.Registry, 0, len(cfg.Registries))
		for _, r := range cfg.Registries {
			if r.Name != name {
				filtered = append(filtered, r)
			}
		}
		cfg.Registries = filtered
		if err := config.SaveGlobal(cfg); err != nil {
			return registryRemoveDoneMsg{name: name, err: fmt.Errorf("saving config: %w", err)}
		}
		return registryRemoveDoneMsg{name: name}
	}
}

// handleAdd opens the add wizard for importing content into the library.
func (a App) handleAdd() (tea.Model, tea.Cmd) {
	// Determine preFilterType from the current Content tab (if any)
	var preFilterType catalog.ContentType
	if a.isContentTab() {
		preFilterType = tabToContentType(a.topBar.ActiveTabLabel())
	}

	a.addWizard = openAddWizard(
		a.providers,
		a.registrySources,
		a.cfg,
		a.projectRoot,
		a.contentRoot,
		preFilterType,
	)
	a.addWizard.width = a.width
	a.addWizard.height = a.contentHeight()
	a.addWizard.shell.SetWidth(a.width)
	a.wizardMode = wizardAdd
	a.updateNavState()
	return a, nil
}

// handleInstall opens the install wizard for the currently selected library item.
func (a App) handleInstall() (tea.Model, tea.Cmd) {
	// Gallery card install not supported (must drill into a card first).
	if a.isGalleryTab() && !a.galleryDrillIn {
		return a, nil
	}

	item := a.selectedItem()
	if item == nil {
		return a, nil
	}

	// Registry-only items must be added to the library first.
	if !item.Library && item.Registry != "" {
		cmd := a.toast.Push("Add this item to your library first", toastWarning)
		return a, cmd
	}

	// Filter to detected providers only.
	var detected []provider.Provider
	for _, prov := range a.providers {
		if prov.Detected {
			detected = append(detected, prov)
		}
	}
	if len(detected) == 0 {
		cmd := a.toast.Push("No providers detected", toastWarning)
		return a, cmd
	}

	a.installWizard = openInstallWizard(*item, detected, a.projectRoot)
	a.installWizard.width = a.width
	a.installWizard.height = a.contentHeight()
	a.installWizard.shell.SetWidth(a.width)
	a.wizardMode = wizardInstall
	a.updateNavState()
	return a, nil
}

// handleInstallResult receives the wizard's confirmation and kicks off the async install.
//
// The MOAT install gate runs here (mirroring cmd/syllago/install_moat.go:
// resolveGateDecision) so all 5 decisions are surfaced at wizard time:
//   - Proceed: straight through to doInstallCmd.
//   - HardBlock: error toast, no modal — registry-source revocations are
//     permanent per ADR 0007 G-15 and cannot be operator-overridden.
//   - PublisherWarn: stash install + open confirm modal (G-8).
//   - PrivatePrompt: stash install + open confirm modal (G-10).
//   - TierBelowPolicy: error toast, no modal — tier cannot be upgraded
//     interactively; only the publisher can.
//
// Items with no MOAT lineage bypass the gate entirely (legacy install path).
func (a App) handleInstallResult(msg installResultMsg) (tea.Model, tea.Cmd) {
	// Close wizard immediately — the install happens async.
	a.installWizard = nil
	a.wizardMode = wizardNone
	a.updateNavState()

	eval, ok := evaluateInstallGate(&a, msg.item)
	if !ok {
		// No live gate (no moatGate configured or item has no MOAT lineage).
		// Fall back to ContentItem fields set by EnrichCatalog at scan time.
		if isPublisherRevoked(msg.item) {
			stashed := msg
			a.pendingInstall = &stashed
			a.pendingInstallAll = nil
			a.pendingGateKind = gateKindPublisherWarn
			a.confirm.OpenForItem(
				publisherWarnTitle(msg.item),
				publisherWarnBody(msg.item, nil),
				"Install anyway",
				true,
				nil,
				msg.item,
			)
			return a, nil
		}
		return a, a.doInstallCmd(msg)
	}

	switch eval.decision.Decision {
	case installer.MOATGateProceed:
		return a, a.doInstallCmd(msg)

	case installer.MOATGateHardBlock:
		return a, a.toast.Push(hardBlockMessage(eval.entryName, eval.decision.Revocation), toastError)

	case installer.MOATGatePublisherWarn:
		stashed := msg
		a.pendingInstall = &stashed
		a.pendingInstallAll = nil
		a.pendingGateKind = gateKindPublisherWarn
		a.pendingGateRegistryURL = eval.registryURL
		a.pendingGateContentHash = eval.contentHash
		a.confirm.OpenForItem(
			publisherWarnTitle(msg.item),
			publisherWarnBody(msg.item, eval.decision.Revocation),
			"Install anyway",
			true,
			nil,
			msg.item,
		)
		return a, nil

	case installer.MOATGatePrivatePrompt:
		stashed := msg
		a.pendingInstall = &stashed
		a.pendingInstallAll = nil
		a.pendingGateKind = gateKindPrivatePrompt
		a.pendingGateRegistryURL = eval.registryURL
		a.pendingGateContentHash = eval.contentHash
		a.confirm.OpenForItem(
			privatePromptTitle(msg.item),
			privatePromptBody(msg.item),
			"Install",
			false, // private-prompt is not a danger action (unlike recalled)
			nil,
			msg.item,
		)
		return a, nil

	case installer.MOATGateTierBelowPolicy:
		return a, a.toast.Push(
			tierBelowPolicyMessage(eval.entryName, eval.decision.ObservedTier, eval.decision.MinTier),
			toastError,
		)
	}

	// Unreachable: MOATGateDecision is a closed enum. If a new variant is
	// added without updating this switch the compiler cannot catch it, so
	// we fall through to the safe (block) default and log via toast.
	return a, a.toast.Push(fmt.Sprintf("Unhandled install gate: %s", eval.decision.Decision), toastError)
}

// doInstallCmd creates a tea.Cmd that performs the install operation in the background.
func (a App) doInstallCmd(msg installResultMsg) tea.Cmd {
	item := msg.item
	prov := msg.provider
	method := msg.method
	projectRoot := msg.projectRoot

	var baseDir string
	switch msg.location {
	case "global":
		baseDir = ""
	case "project":
		baseDir = projectRoot
	default:
		baseDir = msg.location
	}

	return func() tea.Msg {
		desc, err := installer.Install(item, prov, projectRoot, method, baseDir)
		if err != nil {
			return installDoneMsg{
				itemName:     item.DisplayName,
				providerName: prov.Name,
				err:          err,
			}
		}
		return installDoneMsg{
			itemName:     item.DisplayName,
			providerName: prov.Name,
			targetPath:   desc,
		}
	}
}

// handleInstallAllResult receives the "install to all" wizard confirmation and
// kicks off async installs to each provider in the filtered list.
//
// The MOAT install gate applies here identically to single-provider install
// (see handleInstallResult) — the same item is installed to N providers,
// so a single confirm covers the whole batch and the gate only fires once.
func (a App) handleInstallAllResult(msg installAllResultMsg) (tea.Model, tea.Cmd) {
	a.installWizard = nil
	a.wizardMode = wizardNone
	a.updateNavState()

	eval, ok := evaluateInstallGate(&a, msg.item)
	if !ok {
		// No live gate — fall back to ContentItem fields set at scan time.
		if isPublisherRevoked(msg.item) {
			stashed := msg
			a.pendingInstallAll = &stashed
			a.pendingInstall = nil
			a.pendingGateKind = gateKindPublisherWarn
			a.confirm.OpenForItem(
				publisherWarnTitle(msg.item),
				publisherWarnBody(msg.item, nil),
				"Install anyway",
				true,
				nil,
				msg.item,
			)
			return a, nil
		}
		return a, a.doInstallAllCmd(msg)
	}

	switch eval.decision.Decision {
	case installer.MOATGateProceed:
		return a, a.doInstallAllCmd(msg)

	case installer.MOATGateHardBlock:
		return a, a.toast.Push(hardBlockMessage(eval.entryName, eval.decision.Revocation), toastError)

	case installer.MOATGatePublisherWarn:
		stashed := msg
		a.pendingInstallAll = &stashed
		a.pendingInstall = nil
		a.pendingGateKind = gateKindPublisherWarn
		a.pendingGateRegistryURL = eval.registryURL
		a.pendingGateContentHash = eval.contentHash
		a.confirm.OpenForItem(
			publisherWarnTitle(msg.item),
			publisherWarnBody(msg.item, eval.decision.Revocation),
			"Install anyway",
			true,
			nil,
			msg.item,
		)
		return a, nil

	case installer.MOATGatePrivatePrompt:
		stashed := msg
		a.pendingInstallAll = &stashed
		a.pendingInstall = nil
		a.pendingGateKind = gateKindPrivatePrompt
		a.pendingGateRegistryURL = eval.registryURL
		a.pendingGateContentHash = eval.contentHash
		a.confirm.OpenForItem(
			privatePromptTitle(msg.item),
			privatePromptBody(msg.item),
			"Install",
			false,
			nil,
			msg.item,
		)
		return a, nil

	case installer.MOATGateTierBelowPolicy:
		return a, a.toast.Push(
			tierBelowPolicyMessage(eval.entryName, eval.decision.ObservedTier, eval.decision.MinTier),
			toastError,
		)
	}

	return a, a.toast.Push(fmt.Sprintf("Unhandled install gate: %s", eval.decision.Decision), toastError)
}

// doInstallAllCmd installs an item to each provider in the list, collecting results.
func (a App) doInstallAllCmd(msg installAllResultMsg) tea.Cmd {
	item := msg.item
	providers := msg.providers
	projectRoot := msg.projectRoot

	return func() tea.Msg {
		var firstErr error
		count := 0
		for _, prov := range providers {
			_, err := installer.Install(item, prov, projectRoot, installer.MethodSymlink, "")
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
			} else {
				count++
			}
		}
		name := item.DisplayName
		if name == "" {
			name = item.Name
		}
		return installAllDoneMsg{
			itemName: name,
			count:    count,
			firstErr: firstErr,
		}
	}
}

// handleInstallAllDone processes the aggregate result of an "install to all" batch.
func (a App) handleInstallAllDone(msg installAllDoneMsg) (tea.Model, tea.Cmd) {
	var toastText string
	toastKind := toastSuccess
	if msg.firstErr != nil {
		toastText = fmt.Sprintf("Installed to %d providers (some errors occurred)", msg.count)
		toastKind = toastWarning
	} else {
		name := msg.itemName
		if name == "" {
			name = "item"
		}
		toastText = fmt.Sprintf("Installed %q to %d providers", name, msg.count)
	}
	cmd1 := a.toast.Push(toastText, toastKind)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
}

// handleInstallDone processes the result of an install operation.
func (a App) handleInstallDone(msg installDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		cmd := a.toast.Push("Install failed: "+msg.err.Error(), toastError)
		return a, cmd
	}
	name := msg.itemName
	if name == "" {
		name = "item"
	}
	toastText := fmt.Sprintf("Installed %q to %s", name, msg.providerName)
	cmd1 := a.toast.Push(toastText, toastSuccess)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
}
