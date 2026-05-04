package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

// View implements tea.Model.
func (a App) View() string {
	if !a.ready {
		return ""
	}
	if a.width < 80 || a.height < 20 {
		return a.renderTooSmall()
	}

	topBar := a.topBar.View()
	content := a.renderContent()
	helpBar := a.helpBar.View()

	// Overlay modals on top of existing content
	if a.modal.active {
		content = overlayModal(content, a.modal.View(), a.width, a.contentHeight())
	}
	if a.confirm.active {
		content = overlayModal(content, a.confirm.View(), a.width, a.contentHeight())
	}
	if a.remove.active {
		content = overlayModal(content, a.remove.View(), a.width, a.contentHeight())
	}
	if a.registryAdd.active {
		content = overlayModal(content, a.registryAdd.View(), a.width, a.contentHeight())
	}
	if a.tofu.active {
		content = overlayModal(content, a.tofu.View(), a.width, a.contentHeight())
	}
	if a.trustInspector.active {
		content = overlayModal(content, a.trustInspector.View(), a.width, a.contentHeight())
	}
	if a.hint.active {
		content = overlayModal(content, a.hint.View(), a.width, a.contentHeight())
	}
	if a.help.active {
		content = overlayModal(content, a.help.View(), a.width, a.contentHeight())
	}
	// Consent modal overlays all other modals/wizards on first run. It is
	// placed last (just before the toast layer) so nothing visually leaks
	// through; Update routes input to it before any other component, so
	// what the user sees and what they can interact with stay in sync.
	if a.telemetryConsent.Active() {
		content = overlayModal(content, a.telemetryConsent.View(), a.width, a.contentHeight())
	}
	if a.toast.visible {
		content = overlayToast(content, a.toast.View(), a.width, a.contentHeight())
	}

	return zone.Scan(lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		content,
		helpBar,
	))
}

// renderContent renders the main content area based on the active tab.
func (a App) renderContent() string {
	// Wizard renders inside the content area (topbar + helpbar stay visible).
	if a.wizardMode == wizardInstall && a.installWizard != nil {
		return a.installWizard.View()
	}
	if a.wizardMode == wizardAdd && a.addWizard != nil {
		return a.addWizard.View()
	}

	group := a.topBar.ActiveGroupLabel()

	if group == "Config" {
		return a.renderConfigContent()
	}

	if a.isLibraryTab() {
		return a.library.View()
	}

	if a.isGalleryTab() {
		if a.galleryDrillIn {
			return a.library.View()
		}
		return a.gallery.View()
	}

	return a.explorer.View()
}

// renderConfigContent renders the active Config group sub-tab.
func (a App) renderConfigContent() string {
	switch a.topBar.ActiveTabLabel() {
	case "Settings":
		return a.configSettings.View()
	case "Sandbox":
		return a.configSandbox.View()
	case "System":
		return a.configSystem.View()
	}
	return a.renderPlaceholder("Config")
}

// renderPlaceholder renders a centered message for tabs without explorer content.
func (a App) renderPlaceholder(msg string) string {
	h := a.contentHeight()
	return lipgloss.NewStyle().
		Width(a.width).
		Height(h).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(mutedColor).
		Render(msg)
}

// renderTooSmall renders a warning when the terminal is below minimum size.
func (a App) renderTooSmall() string {
	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		warningStyle.Render("Terminal too small\nMinimum: 80x20"),
	)
}

// overlayModal centers the modal within the content area. The background
// content shows through on all sides of the modal.
func overlayModal(bg, modal string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	modalLines := strings.Split(modal, "\n")
	modalH := len(modalLines)

	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	// Center the modal vertically, splice modal into each background row
	startRow := max(0, (height-modalH)/2)
	for i, mLine := range modalLines {
		row := startRow + i
		if row >= len(bgLines) {
			break
		}
		// Center the modal line horizontally
		mLineW := lipgloss.Width(mLine)
		pad := max(0, (width-mLineW)/2)
		rightStart := pad + mLineW

		// Splice: bg_left + modal + bg_right
		left := ansi.Truncate(bgLines[row], pad, "")
		right := ""
		if rightStart < width {
			right = ansi.Cut(bgLines[row], rightStart, width)
		}
		bgLines[row] = left + mLine + right
	}

	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}
	return strings.Join(bgLines, "\n")
}
