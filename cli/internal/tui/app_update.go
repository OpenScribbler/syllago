package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

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
		a.registryAdd.width = msg.Width
		a.registryAdd.height = ch
		a.trustInspector.SetSize(msg.Width, ch)
		a.configSettings.SetSize(msg.Width, ch)
		a.configSystem.SetSize(msg.Width, ch)
		a.configSandbox.SetSize(msg.Width, ch)
		if a.installWizard != nil {
			a.installWizard.width = msg.Width
			a.installWizard.height = a.contentHeight()
			a.installWizard.shell.SetWidth(msg.Width)
		}
		if a.addWizard != nil {
			a.addWizard.width = msg.Width
			a.addWizard.height = a.contentHeight()
			a.addWizard.shell.SetWidth(msg.Width)
			// Resize type checkbox list
			a.addWizard.typeChecks = a.addWizard.typeChecks.SetSize(msg.Width-4, a.addWizard.typeListHeight())
			// Rebuild discovery list so column widths and height match the new size
			if a.addWizard.step == addStepDiscovery && len(a.addWizard.discoveredItems) > 0 && !a.addWizard.discovering {
				a.addWizard.rebuildDiscoveryListPreserveSelection()
			} else {
				a.addWizard.discoveryList = a.addWizard.discoveryList.SetSize(msg.Width-4, a.addWizard.discoveryListHeight())
			}
			// Adjust review item offset for new height
			a.addWizard.adjustReviewOffset()
		}
		return a, nil

	case tea.MouseMsg:
		// Wizard mode: topbar [?] button and help overlay still work
		if a.wizardMode != wizardNone {
			if a.help.active {
				var cmd tea.Cmd
				a.help, cmd = a.help.Update(msg)
				return a, cmd
			}
			// Allow [?] help button clicks on the topbar
			if zone.Get("btn-help").InBounds(msg) {
				a.help.Toggle()
				return a, nil
			}
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
		if a.registryAdd.active {
			var cmd tea.Cmd
			a.registryAdd, cmd = a.registryAdd.Update(msg)
			return a, cmd
		}
		if a.trustInspector.active {
			var cmd tea.Cmd
			a.trustInspector, cmd = a.trustInspector.Update(msg)
			return a, cmd
		}
		if a.help.active {
			var cmd tea.Cmd
			a.help, cmd = a.help.Update(msg)
			return a, cmd
		}
		// Toast close button takes priority over content
		if a.toast.visible {
			consumed, cmd := a.toast.HandleMouse(msg)
			if consumed {
				return a, cmd
			}
		}
		return a.routeMouse(msg)

	case tea.KeyMsg:
		// Toast dismissal takes priority over modals — Esc dismisses toast first.
		if a.toast.visible && msg.Type == tea.KeyEsc {
			cmd := a.toast.Dismiss()
			return a, cmd
		}

		// Wizard mode captures all key input (except ctrl+c, help, and group keys)
		if a.wizardMode != wizardNone {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			// When a wizard has a text field focused (e.g., rename modal,
			// source-step path input), forward every key including digits —
			// the app's group-key hijack (1/2/3) must not eat typed characters.
			capturing := a.wizardMode == wizardAdd && a.addWizard != nil && a.addWizard.CapturingTextInput()
			if !capturing {
				// Help overlay always available during wizard
				if msg.String() == keyHelp {
					a.help.Toggle()
					return a, nil
				}
				// Suppress group/tab navigation during wizard — user must Esc out first
				switch msg.String() {
				case keyGroup1, keyGroup2, keyGroup3:
					return a, nil
				}
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

		// Registry add modal captures all key input when active (except ctrl+c)
		if a.registryAdd.active {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.registryAdd, cmd = a.registryAdd.Update(msg)
			return a, cmd
		}

		// Trust inspector modal captures all key input when active (except ctrl+c)
		if a.trustInspector.active {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.trustInspector, cmd = a.trustInspector.Update(msg)
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

		// When search is active, only handle ctrl+c — everything
		// else goes to the search input so letters like 'a', 'q', '1' etc.
		// are typed into the query rather than triggering shortcuts.
		if a.isLibraryTab() && a.library.table.searching {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			return a.routeKey(msg)
		}
		if a.isContentTab() && a.explorer.searching {
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			return a.routeKey(msg)
		}
		if a.isGalleryTab() && !a.galleryDrillIn && a.gallery.searching {
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

		// Tab cycles sub-tabs within active group.
		// In Config group, Tab is owned by the sub-model (panel cycling within
		// Settings/Sandbox/System). Use Left/Right to switch Config topbar sub-tabs.
		case msg.Type == tea.KeyTab:
			if a.topBar.ActiveGroupLabel() == "Config" {
				return a.routeKey(msg)
			}
			cmd := a.topBar.NextTab()
			a.refreshContent()
			return a, cmd
		case msg.Type == tea.KeyShiftTab:
			if a.topBar.ActiveGroupLabel() == "Config" {
				return a.routeKey(msg)
			}
			cmd := a.topBar.PrevTab()
			a.refreshContent()
			return a, cmd

		// Left/Right arrow: cycle Config topbar sub-tabs (Settings/Sandbox/System).
		case msg.Type == tea.KeyRight && a.topBar.ActiveGroupLabel() == "Config":
			cmd := a.topBar.NextTab()
			a.refreshContent()
			return a, cmd
		case msg.Type == tea.KeyLeft && a.topBar.ActiveGroupLabel() == "Config":
			cmd := a.topBar.PrevTab()
			a.refreshContent()
			return a, cmd

		// Action button hotkeys
		case msg.String() == keyAdd:
			if a.topBar.ActiveGroupLabel() == "Config" {
				return a.routeKey(msg)
			}
			if a.isRegistriesTab() && !a.galleryDrillIn {
				if a.registryOpInProgress {
					cmd := a.toast.Push("Registry operation in progress", toastWarning)
					return a, cmd
				}
				return a.handleRegistryAdd()
			}
			if a.isLibraryTab() || a.isContentTab() {
				return a.handleAdd()
			}
			return a, a.topBar.actionCmd("add")
		// keyCreate ("n") is deferred — no-op for now

		// Edit selected item (name + description)
		case msg.String() == keyEdit:
			return a.handleEdit()

		// Remove item from library
		case msg.String() == keyRemove:
			if a.topBar.ActiveGroupLabel() == "Config" {
				return a.routeKey(msg)
			}
			return a.handleRemove()

		// Uninstall item from provider
		case msg.String() == keyUninstall:
			return a.handleUninstall()

		// Install item to a provider
		case msg.String() == keyInstall:
			return a.handleInstall()

		// Sync registry (only intercept S on Registries tab)
		case msg.String() == keySync && a.isRegistriesTab():
			if !a.galleryDrillIn && !a.registryOpInProgress {
				return a.handleSync()
			}
			if a.registryOpInProgress {
				cmd := a.toast.Push("Registry operation in progress", toastWarning)
				return a, cmd
			}

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

	case telemetryNoticeMsg:
		const noticeText = "Syllago collects anonymous usage data to help prioritize\n" +
			"development. No file contents or identifying info is collected.\n" +
			"Run \"syllago telemetry off\" to disable.\n" +
			"syllago.dev/telemetry"
		cmd := a.toast.Push(noticeText, toastWarning)
		return a, cmd

	case toastTickMsg:
		var cmd tea.Cmd
		a.toast, cmd = a.toast.Update(msg)
		return a, cmd

	case editSavedMsg:
		// Wizard-scoped rename keeps state in the wizard; library edit writes to disk.
		if msg.context == "wizard_rename" && a.addWizard != nil {
			a.addWizard.handleRenameSaved(msg)
			return a, nil
		}
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

	case installAllResultMsg:
		return a.handleInstallAllResult(msg)

	case installAllDoneMsg:
		return a.handleInstallAllDone(msg)

	case installDoneMsg:
		return a.handleInstallDone(msg)

	case installCloseMsg:
		a.installWizard = nil
		a.wizardMode = wizardNone
		a.updateNavState()
		return a, nil

	case addCloseMsg:
		if a.addWizard != nil && a.addWizard.gitTempDir != "" {
			_ = os.RemoveAll(a.addWizard.gitTempDir)
		}
		a.addWizard = nil
		a.wizardMode = wizardNone
		a.updateNavState()
		cmd := a.rescanCatalog()
		return a, cmd

	case addRestartMsg:
		// Close current wizard, rescan catalog, then reopen a fresh one.
		// The rescan Cmd must be batched with handleAdd's Cmd — discarding
		// it would swallow the refresh (and its toast) entirely.
		if a.addWizard != nil && a.addWizard.gitTempDir != "" {
			_ = os.RemoveAll(a.addWizard.gitTempDir)
		}
		a.addWizard = nil
		a.wizardMode = wizardNone
		rescanCmd := a.rescanCatalog()
		model, handleCmd := a.handleAdd()
		return model, tea.Batch(rescanCmd, handleCmd)

	case catalogReadyMsg:
		return a.handleCatalogReady(msg)

	case addDiscoveryDoneMsg:
		if a.addWizard != nil {
			_, cmd := a.addWizard.Update(msg)
			if msg.err != nil {
				toastCmd := a.toast.Push("Discovery failed: "+msg.err.Error(), toastError)
				return a, tea.Batch(cmd, toastCmd)
			}
			return a, cmd
		}

	case addExecItemDoneMsg:
		if a.addWizard != nil {
			_, cmd := a.addWizard.Update(msg)
			return a, cmd
		}

	case addExecAllDoneMsg:
		if a.addWizard != nil {
			count := len(a.addWizard.selectedItems())
			cmd := a.toast.Push(fmt.Sprintf("Added %d items to library", count), toastSuccess)
			return a, cmd
		}

	case registryAddMsg:
		return a.handleRegistryAddResult(msg)
	case registryAddDoneMsg:
		return a.handleRegistryAddDone(msg)
	case registrySyncDoneMsg:
		return a.handleSyncDone(msg)
	case registryRemoveDoneMsg:
		return a.handleRegistryRemoveDone(msg)

	case tabChangedMsg:
		a.galleryDrillIn = false
		a.refreshContent()
		a.updateNavState()
		if msg.group == 2 {
			a.configSystem.loading = true
			return a, tea.Batch(
				a.configSystem.loadCmd(),
				a.configSettings.loadTelemetryCmd(),
				a.configSandbox.loadCheckCmd(),
			)
		}
		return a, nil

	case systemLoadedMsg:
		updated, _ := a.configSystem.Update(msg)
		a.configSystem = updated.(systemModel)
		return a, nil

	case settingsTelemetryStatusMsg:
		updated, _ := a.configSettings.Update(msg)
		a.configSettings = updated.(settingsModel)
		return a, nil

	case settingsUpdateCheckedMsg:
		updated, _ := a.configSettings.Update(msg)
		a.configSettings = updated.(settingsModel)
		return a, nil

	case sandboxCheckLoadedMsg:
		updated, _ := a.configSandbox.Update(msg)
		a.configSandbox = updated.(sandboxConfigModel)
		return a, nil

	case sandboxSavedMsg:
		updated, _ := a.configSandbox.Update(msg)
		a.configSandbox = updated.(sandboxConfigModel)
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

	case libraryTrustInspectMsg:
		if msg.item != nil {
			a.trustInspector.OpenForItem(*msg.item)
		}
		return a, nil

	case explorerTrustInspectMsg:
		if msg.item != nil {
			a.trustInspector.OpenForItem(*msg.item)
		}
		return a, nil

	case registryTrustInspectMsg:
		if msg.card != nil && msg.card.trust != nil {
			a.trustInspector.OpenForRegistry(registryTrustSummaryFrom(msg.card.trust))
		}
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
			if a.isRegistriesTab() && !a.galleryDrillIn {
				if a.registryOpInProgress {
					cmd := a.toast.Push("Registry operation in progress", toastWarning)
					return a, cmd
				}
				return a.handleRegistryAdd()
			}
			if a.isLibraryTab() || a.isContentTab() {
				return a.handleAdd()
			}
		case "sync":
			if a.isRegistriesTab() && !a.galleryDrillIn && !a.registryOpInProgress {
				return a.handleSync()
			}
			if a.registryOpInProgress {
				cmd := a.toast.Push("Registry operation in progress", toastWarning)
				return a, cmd
			}
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
// This is the same implementation path as the CLI's 'syllago edit' command —
// both delegate to the metadata package; no separate code path exists.
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
	if a.topBar.ActiveGroupLabel() == "Config" {
		return a.routeKeyConfig(msg)
	}
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

// routeKeyConfig dispatches key messages to the active Config sub-model.
func (a App) routeKeyConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.topBar.ActiveTabLabel() {
	case "Settings":
		updated, cmd := a.configSettings.Update(msg)
		a.configSettings = updated.(settingsModel)
		return a, cmd
	case "Sandbox":
		updated, cmd := a.configSandbox.Update(msg)
		a.configSandbox = updated.(sandboxConfigModel)
		return a, cmd
	case "System":
		updated, cmd := a.configSystem.Update(msg)
		a.configSystem = updated.(systemModel)
		return a, cmd
	}
	return a, nil
}

// routeMouse sends mouse messages to topbar + active content model.
func (a App) routeMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	var topCmd tea.Cmd
	a.topBar, topCmd = a.topBar.Update(msg)

	var contentCmd tea.Cmd
	if a.topBar.ActiveGroupLabel() == "Config" {
		switch a.topBar.ActiveTabLabel() {
		case "Settings":
			updated, cmd := a.configSettings.Update(msg)
			a.configSettings = updated.(settingsModel)
			contentCmd = cmd
		case "Sandbox":
			updated, cmd := a.configSandbox.Update(msg)
			a.configSandbox = updated.(sandboxConfigModel)
			contentCmd = cmd
		case "System":
			updated, cmd := a.configSystem.Update(msg)
			a.configSystem = updated.(systemModel)
			contentCmd = cmd
		}
	} else if a.isLibraryTab() {
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
			a.helpBar.SetHints(a.currentHints())
			return a, cmd
		}
	case wizardAdd:
		if a.addWizard != nil {
			_, cmd := a.addWizard.Update(msg)
			a.helpBar.SetHints(a.currentHints())
			return a, cmd
		}
	}
	return a, nil
}
