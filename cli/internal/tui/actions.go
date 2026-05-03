package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// formatToastErr renders an error for a TUI toast. For output.StructuredError
// (the standard CLI error envelope), it appends the Details field — without
// it MOAT verification failures and similar wrapped errors arrive as the bare
// "[CODE] message" with no diagnostic content. Plain errors fall through to
// Error() unchanged so non-structured errors look the same as before.
func formatToastErr(err error) string {
	var se output.StructuredError
	if errors.As(err, &se) && se.Details != "" {
		return fmt.Sprintf("[%s] %s — %s", se.Code, se.Message, se.Details)
	}
	return err.Error()
}

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
		// itemName flows to doRegistryRemoveCmd as the registry identity. The
		// display name shown in the prompt above can come from registry.yaml,
		// but the operational key MUST be the config name so removal hits the
		// right entry. Fall back to card.name if sourceName is unset (older
		// card-build paths that haven't been updated yet).
		identity := card.sourceName
		if identity == "" {
			identity = card.name
		}
		a.confirm.itemName = identity
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

	// Registry remove: on Registries tab and the confirm carries no ContentItem
	// (Type unset). The earlier discriminator used Path=="" but MOAT-materialized
	// items also have empty Path while still carrying a Type — that collision
	// routed item uninstalls through the registry-remove orchestrator and
	// surfaced as "registry not found".
	if a.isRegistriesTab() && msg.item.Type == "" && msg.itemName != "" {
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
//
// isMOAT signals that the saved registry is MOAT-typed (allowlist match or
// registry.yaml self-declaration). The handler uses this to chain a sync
// immediately after add — without it, the manifest cache is never populated
// and the gallery shows "Trust: Unknown" until the user manually presses S.
type registryAddDoneMsg struct {
	name   string
	err    error
	isMOAT bool
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

// doRegistryAddCmd creates a tea.Cmd that delegates to the shared
// registryops.AddRegistry orchestrator. The TUI used to fork its own copy
// of the add logic, which silently dropped the CLI's validation gates
// (name format check, allowedRegistries policy enforcement, non-syllago
// repo rejection) — bead syllago-mpold tracked that security gap. After
// this refactor both surfaces share identical gates.
//
// MOAT detection: the orchestrator runs allowlist lookup + registry.yaml
// self-declaration internally. Callers don't need to reproduce either.
//
// Validation errors are translated into human-readable strings for the
// toast layer. The TUI doesn't surface structured-error codes — operators
// get a one-line description, which is the expected presentation.
func (a App) doRegistryAddCmd(msg registryAddMsg) tea.Cmd {
	url := msg.url
	name := msg.name
	ref := msg.ref
	isLocal := msg.isLocal
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		outcome, err := registryops.AddRegistry(ctx, registryops.AddOpts{
			URL:     url,
			Name:    name,
			Ref:     ref,
			IsLocal: isLocal,
		})
		if err != nil {
			return registryAddDoneMsg{name: name, err: humaniseAddErr(err, name, url)}
		}
		return registryAddDoneMsg{name: outcome.Registry.Name, isMOAT: outcome.Registry.IsMOAT()}
	}
}

// humaniseAddErr converts orchestrator sentinel errors into the short
// strings the toast layer expects. Mirrors the CLI's classifyAddError but
// without the structured-error wrapping (the TUI doesn't render exit codes
// or JSON shapes).
func humaniseAddErr(err error, name, url string) error {
	switch {
	case errors.Is(err, registryops.ErrAddInvalidName):
		return fmt.Errorf("registry name %q is invalid", name)
	case errors.Is(err, registryops.ErrAddDuplicate):
		return fmt.Errorf("registry %q already exists", name)
	case errors.Is(err, registryops.ErrAddNotAllowed):
		return fmt.Errorf("registry URL %q is not in the allowedRegistries list", url)
	case errors.Is(err, registryops.ErrAddNotSyllago):
		return fmt.Errorf("not a syllago registry — clone removed")
	case errors.Is(err, registryops.ErrAddCloneFailed):
		return err
	case errors.Is(err, registryops.ErrAddSaveFailed):
		return err
	default:
		return err
	}
}

// handleRegistryAddDone processes the result of a registry add operation.
//
// MOAT-typed registries chain straight into a sync so the manifest cache is
// populated before the next rescan. Without this, EnrichFromMOATManifests
// sees an empty cache, downgrades trust to Unknown, and the gallery shows
// "Trust: Unknown" until the user manually presses S. For unpinned
// (self-declared) registries, the sync surfaces the TOFU modal — which is
// the correct gate for first-add consent.
func (a App) handleRegistryAddDone(msg registryAddDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.registryOpInProgress = false
		cmd := a.toast.Push("Add failed: "+msg.err.Error(), toastError)
		return a, cmd
	}
	if msg.isMOAT {
		// Keep registryOpInProgress=true through the sync; handleMOATSyncDone
		// (or handleTOFUResult) clears it. Skip the immediate rescan — the
		// sync handler triggers its own once the cache is in place, so the
		// catalog refresh sees real trust badges instead of Unknown.
		cmd1 := a.toast.Push("Added "+msg.name+" — verifying signing identity...", toastSuccess)
		cmd2 := a.doMOATSyncCmd(msg.name, false)
		return a, tea.Batch(cmd1, cmd2)
	}
	a.registryOpInProgress = false
	cmd1 := a.toast.Push("Added registry: "+msg.name, toastSuccess)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
}

// handleSync starts a registry sync operation. MOAT registries route through
// the MOAT-aware orchestrator (fetch + verify + cache + persist); plain git
// registries fall back to `git pull`. Mirrors `syllago registry sync`.
func (a App) handleSync() (tea.Model, tea.Cmd) {
	card := a.gallery.selectedCard()
	if card == nil {
		return a, nil
	}
	// sourceName is the config-bound identity. card.name may be a manifest-
	// supplied display label and would not match the cache directory.
	name := card.sourceName
	if name == "" {
		name = card.name
	}
	a.registryOpInProgress = true

	if a.registryIsMOAT(name) {
		cmd1 := a.toast.Push("Syncing "+name+" (moat)...", toastSuccess)
		return a, tea.Batch(cmd1, a.doMOATSyncCmd(name, false))
	}

	cmd1 := a.toast.Push("Syncing "+name+"...", toastSuccess)
	cmd2 := func() tea.Msg {
		err := registry.Sync(name)
		return registrySyncDoneMsg{name: name, err: err}
	}
	return a, tea.Batch(cmd1, tea.Cmd(cmd2))
}

// registryIsMOAT reports whether the named registry is MOAT-typed in the
// in-memory config. The check is best-effort: if the name is missing, we
// fall through to the git path so a stale gallery card never wedges sync.
func (a App) registryIsMOAT(name string) bool {
	for _, r := range a.cfg.Registries {
		if r.Name == name {
			return r.IsMOAT()
		}
	}
	return false
}

// handleMOATSyncDone routes the orchestrator's outcome.
//   - profileChanged: surfaced as an error toast pointing the user to
//     remove + re-add (the only path that re-establishes consent).
//   - requiresTOFU: opens a modal that lets the user accept inline.
//   - stale: success toast tagged with "stale" so the downgraded badges
//     in the catalog don't look like a bug.
//   - happy: success toast + rescan so cards repaint with fresh trust.
func (a App) handleMOATSyncDone(msg moatSyncDoneMsg) (tea.Model, tea.Cmd) {
	a.registryOpInProgress = false
	if msg.err != nil {
		cmd := a.toast.Push("Sync failed: "+msg.err.Error(), toastError)
		return a, cmd
	}
	if msg.profileChanged {
		text := "Sync rejected: signing profile changed for " + msg.name + ". Remove and re-add the registry to re-approve."
		cmd := a.toast.Push(text, toastError)
		return a, cmd
	}
	if msg.requiresTOFU {
		a.tofu.Open(msg.name, msg.manifestURL, msg.incomingProfile)
		return a, nil
	}
	verb := "Synced "
	if msg.stale {
		verb = "Synced (stale) "
	}
	cmd1 := a.toast.Push(verb+msg.name, toastSuccess)
	cmd2 := a.rescanCatalog()
	return a, tea.Batch(cmd1, cmd2)
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

// handleTOFUResult routes the user's accept/reject decision from the TOFU
// modal. Accept re-runs MOAT sync with acceptTOFU=true so the orchestrator
// pins the wire profile on the second pass; reject surfaces a toast and
// leaves the registry pinned-less (subsequent syncs will re-prompt).
func (a App) handleTOFUResult(msg tofuResultMsg) (tea.Model, tea.Cmd) {
	if !msg.accepted {
		cmd := a.toast.Push("Rejected signing identity for "+msg.name, toastWarning)
		return a, cmd
	}
	a.registryOpInProgress = true
	cmd1 := a.toast.Push("Trusting signing identity, re-syncing "+msg.name+"...", toastSuccess)
	return a, tea.Batch(cmd1, a.doMOATSyncCmd(msg.name, true))
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

// doRegistryRemoveCmd is a thin wrapper around registryops.RemoveRegistry.
// Registries live in global config only (decision 2026-04-24, syllago-fhtxa);
// the orchestrator owns load/filter/save plus best-effort clone deletion
// plus MOAT manifest-cache + lockfile pin-state cleanup (syllago-teat0).
//
// projectRoot threads through so the lockfile prune path can find
// .syllago/moat-lockfile.json. cacheDir comes from config.GlobalDirPath()
// to address the same MOAT cache root the producer + sync path use.
// Both are best-effort soft errors per the orchestrator contract;
// we collapse them into a single soft toast for the user.
func (a App) doRegistryRemoveCmd(name string) tea.Cmd {
	projectRoot := a.projectRoot
	cacheDir, _ := config.GlobalDirPath()
	return func() tea.Msg {
		outcome, err := registryops.RemoveRegistry(registryops.RemoveOpts{
			Name:        name,
			ProjectRoot: projectRoot,
			CacheDir:    cacheDir,
		})
		if err != nil {
			return registryRemoveDoneMsg{name: name, err: err}
		}
		if softErr := firstSoftRemoveErr(outcome); softErr != nil {
			return registryRemoveDoneMsg{name: name, err: softErr}
		}
		return registryRemoveDoneMsg{name: name}
	}
}

// firstSoftRemoveErr returns the first non-nil best-effort error from a
// remove outcome wrapped with a context message, or nil when all soft
// errors are clear. We only surface one at a time because the toast UI is
// single-line and the user's recovery action ("rerun remove" or "rm -rf
// the cache dir") is the same regardless of which sub-step failed.
func firstSoftRemoveErr(outcome registryops.RemoveOutcome) error {
	switch {
	case outcome.CloneRemoveErr != nil:
		return fmt.Errorf("removed from config; clone cleanup failed: %w", outcome.CloneRemoveErr)
	case outcome.ManifestCacheRemoveErr != nil:
		return fmt.Errorf("removed from config; MOAT cache cleanup failed: %w", outcome.ManifestCacheRemoveErr)
	case outcome.LockfilePruneErr != nil:
		return fmt.Errorf("removed from config; lockfile pin cleanup failed: %w", outcome.LockfilePruneErr)
	}
	return nil
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

	// Non-MOAT registry items (git-registry clones not yet promoted into the
	// library) still require an explicit `Add` step because the content blob
	// already lives on disk under the registry checkout — installing in place
	// would couple the install path to a transient clone directory. MOAT-
	// materialized items take a different code path: the content blob is
	// fetched at install time from the manifest's SourceURI and staged into
	// a per-item cache directory, so they flow straight through the wizard
	// here and branch in doInstallCmd → doMOATInstallCmd.
	if !item.Library && item.Registry != "" && !isUnstagedRegistryItem(item) {
		cmd := a.toast.Push("Add this item to your library first", toastWarning)
		return a, cmd
	}

	// Detect() is advisory only (provider.go:39) — pass every provider that
	// supports the item's type. Undetected providers are labeled in the
	// wizard's provider picker so the user can still pick them (e.g. for
	// custom-path installs that detection misses).
	var supporting []provider.Provider
	for _, prov := range a.providers {
		if prov.SupportsType != nil && prov.SupportsType(item.Type) {
			supporting = append(supporting, prov)
		}
	}
	if len(supporting) == 0 {
		cmd := a.toast.Push("No providers support this content type", toastWarning)
		return a, cmd
	}

	a.installWizard = openInstallWizard(*item, supporting, a.projectRoot)
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

	// D5 append path: route to InstallRuleAppend, which writes to the
	// provider's monolithic filename (CLAUDE.md, AGENTS.md, etc.) rather
	// than placing a file via installer.Install. Location flag is ignored
	// because the append scope comes from the target file's path (resolved
	// by installer.ResolveAppendScope).
	if method == installer.MethodAppend {
		return a.doInstallAppendCmd(msg)
	}

	// Unstaged MOAT items have no on-disk content tree (item.Path is empty)
	// because the bytes live in the registry, not the local catalog. Route
	// through the MOAT fetch + provider-install pipeline instead of the
	// library install path, which assumes a populated source directory.
	if isUnstagedRegistryItem(&item) {
		return a.doMOATInstallCmd(msg)
	}

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

// doInstallAppendCmd runs the D5 monolithic-append install. It loads the
// library rule via rulestore, resolves the target monolithic filename from
// the provider slug (D10), and calls InstallRuleAppend. Returns
// installDoneMsg with targetPath set to the monolithic file.
func (a App) doInstallAppendCmd(msg installResultMsg) tea.Cmd {
	item := msg.item
	prov := msg.provider
	projectRoot := msg.projectRoot

	return func() tea.Msg {
		done := installDoneMsg{
			itemName:     item.DisplayName,
			providerName: prov.Name,
		}
		monoNames := provider.MonolithicFilenames(prov.Slug)
		if len(monoNames) == 0 {
			done.err = fmt.Errorf("provider %s does not have a monolithic rule filename", prov.Slug)
			return done
		}
		loaded, err := rulestore.LoadRule(item.Path)
		if err != nil {
			done.err = fmt.Errorf("loading library rule: %w", err)
			return done
		}
		homeDir, _ := os.UserHomeDir()
		target := filepath.Join(projectRoot, monoNames[0])
		if err := installer.InstallRuleAppend(projectRoot, homeDir, prov.Slug, target, "tui", loaded); err != nil {
			done.err = err
			return done
		}
		done.targetPath = target
		return done
	}
}

// doMOATInstallCmd is the install path for unstaged MOAT items — items
// synthesized from a MOAT manifest that have no on-disk content tree yet
// (item.Path == "" / item.Source == registry name). It performs the network
// fetch + sigstore verification via moatinstall.FetchAndRecord, then stages
// the cached source tree into the chosen provider via
// installer.InstallCachedMOATToProvider, mirroring the CLI install path in
// runInstallFromRegistry.
//
// Inputs are captured before returning the tea.Cmd so the closure does not
// race with rescans that mutate a.cfg / a.moatGate. The lockfile is reloaded
// inside the closure because async installs can land out of order with
// other lockfile-mutating commands; we want the freshest view at write time.
//
// The MOAT install gate has already been evaluated in handleInstallResult
// before this command is dispatched. Re-evaluating here would either
// duplicate prompts or silently downgrade decisions if state changed
// mid-install — neither is desirable. Trust the caller's gate result.
func (a App) doMOATInstallCmd(msg installResultMsg) tea.Cmd {
	item := msg.item
	prov := msg.provider
	method := msg.method
	projectRoot := msg.projectRoot

	var reg *config.Registry
	for i := range a.cfg.Registries {
		if a.cfg.Registries[i].Name == item.Source {
			reg = &a.cfg.Registries[i]
			break
		}
	}

	var (
		manifest *moat.Manifest
		entry    *moat.ContentEntry
	)
	if a.moatGate != nil {
		if m, ok := a.moatGate.Manifests[item.Source]; ok {
			manifest = m
			if e, found := moat.FindContentEntry(m, item.Name); found {
				entry = e
			}
		}
	}

	lockfilePath := moat.LockfilePath(projectRoot)
	rootInfo := moat.BundledTrustedRoot(time.Now())

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
		done := installDoneMsg{
			itemName:     item.DisplayName,
			providerName: prov.Name,
		}
		if reg == nil {
			done.err = fmt.Errorf("registry %q not found in config", item.Source)
			return done
		}
		if manifest == nil || entry == nil {
			done.err = fmt.Errorf("registry %q does not list %q in its manifest (run 'R' to rescan)", item.Source, item.Name)
			return done
		}
		if rootInfo.Status == moat.TrustedRootStatusExpired ||
			rootInfo.Status == moat.TrustedRootStatusMissing ||
			rootInfo.Status == moat.TrustedRootStatusCorrupt {
			done.err = fmt.Errorf("bundled trusted root unusable (%s); run `syllago update`", rootInfo.Status.String())
			return done
		}

		lf, err := moat.LoadLockfile(lockfilePath)
		if err != nil {
			done.err = fmt.Errorf("load lockfile: %w", err)
			return done
		}

		cacheDir, fetchErr := moatinstall.FetchAndRecord(
			context.Background(), entry, reg.Name, reg.ManifestURI,
			lockfilePath, lf, &manifest.RegistrySigningProfile, rootInfo.Bytes,
		)
		if fetchErr != nil {
			done.err = fetchErr
			return done
		}

		installPath, installErr := installer.InstallCachedMOATToProvider(
			cacheDir, entry, prov, projectRoot, method, baseDir,
		)
		if installErr != nil {
			done.err = installErr
			return done
		}
		done.targetPath = installPath
		return done
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
		cmd := a.toast.Push("Install failed: "+formatToastErr(msg.err), toastError)
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

// handleLibraryAdd adds a Registry Clone item to the local syllago library.
func (a App) handleLibraryAdd(item *catalog.ContentItem) (tea.Model, tea.Cmd) {
	if item == nil {
		return a, nil
	}
	cmd := a.toast.Push(fmt.Sprintf("Added %q to library", item.Name), toastSuccess)
	return a, cmd
}

// handleLibraryAddInstall adds a Registry Clone item to the local syllago
// library and opens the install wizard to install it immediately.
func (a App) handleLibraryAddInstall(item *catalog.ContentItem) (tea.Model, tea.Cmd) {
	if item == nil {
		return a, nil
	}
	cmd := a.toast.Push(fmt.Sprintf("Added %q to library — use [i] Install to deploy", item.Name), toastSuccess)
	return a, cmd
}
