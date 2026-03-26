package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
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

	// Only library items use the multi-step remove flow.
	if !item.Library {
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
	// Registry remove is Phase C — no-op for now.
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
func (a App) handleConfirmResult(msg confirmResultMsg) (tea.Model, tea.Cmd) {
	if !msg.confirmed {
		return a, nil
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

		if err := os.RemoveAll(item.Path); err != nil {
			return removeDoneMsg{itemName: item.Name, err: fmt.Errorf("removing from library: %w", err)}
		}

		return removeDoneMsg{itemName: item.Name, uninstalledFrom: uninstalledFrom}
	}
}

// doSimpleRemoveCmd creates a tea.Cmd that removes an item from disk (no uninstall).
func (a App) doSimpleRemoveCmd(item catalog.ContentItem) tea.Cmd {
	return func() tea.Msg {
		if err := os.RemoveAll(item.Path); err != nil {
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
