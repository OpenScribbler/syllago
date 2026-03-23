package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
)

// renderContentSplit returns pinned header and scrollable body separately.
// The pinned header contains: back link + item name + tab bar + metadata + separator.
// The scrollable body contains: tab content only.
func (m detailModel) renderContentSplit() (pinned string, body string) {
	name := StripControlChars(displayName(m.item))

	// Breadcrumb: Home > [Parent >] Category > Item Name
	// Build the current item label with badge suffix
	currentLabel := name
	var badgeSuffix string
	if m.item.IsBuiltin() {
		badgeSuffix = " " + builtinStyle.Render("[BUILT-IN]")
	} else if m.item.Library {
		badgeSuffix = " " + warningStyle.Render("[LIBRARY]")
	} else if m.item.Registry != "" {
		badgeSuffix = " " + countStyle.Render("["+m.item.Registry+"]")
	} else if m.item.Source == "global" {
		badgeSuffix = " " + globalStyle.Render("[GLOBAL]")
	}

	segments := []BreadcrumbSegment{{"Home", "crumb-home"}}
	if m.parentLabel != "" {
		segments = append(segments, BreadcrumbSegment{m.parentLabel, "crumb-parent"})
	}
	catLabel := m.item.Type.Label()
	if m.categoryLabel != "" {
		catLabel = m.categoryLabel
	}
	segments = append(segments, BreadcrumbSegment{catLabel, "crumb-category"})
	segments = append(segments, BreadcrumbSegment{currentLabel, ""})
	pinned += renderBreadcrumb(segments...) + badgeSuffix + "\n\n"

	// Tab bar
	pinned += m.renderTabBar() + "\n"

	// Metadata block (always visible, below tabs)
	pinned += labelStyle.Render("Type: ") + valueStyle.Render(m.item.Type.Label())
	if m.item.IsBuiltin() {
		pinned += "  " + builtinStyle.Render("[Built-in]")
	} else if m.item.Library {
		pinned += "  " + warningStyle.Render("[Library]")
	}
	pinned += "\n"
	if m.item.Registry != "" {
		pinned += labelStyle.Render("Registry: ") + valueStyle.Render(m.item.Registry) + "\n"
	}
	if m.item.Path != "" {
		pinned += labelStyle.Render("Path: ") + valueStyle.Render(m.item.Path) + "\n"
	}
	if !m.item.Type.IsUniversal() && m.item.Provider != "" {
		pinned += labelStyle.Render("Provider: ") + valueStyle.Render(providerDisplayName(m.item.Provider)) + "\n"
	} else {
		detected := m.detectedProviders()
		if len(detected) > 0 {
			var names []string
			for _, p := range detected {
				names = append(names, p.Name)
			}
			pinned += labelStyle.Render("Providers: ") + valueStyle.Render(strings.Join(names, ", ")) + "\n"
		}
	}

	// Override info
	if len(m.overrides) > 0 {
		for _, ov := range m.overrides {
			source := "built-in"
			if ov.Registry != "" {
				source = ov.Registry
			} else if ov.Library {
				source = "library"
			} else if !ov.IsBuiltin() {
				source = "shared"
			}
			pinned += warningStyle.Render("Overrides ["+source+"] version") + "\n"
		}
	}

	// Horizontal separator (full content width)
	sepW := m.width
	if sepW < 20 {
		sepW = 60
	}
	pinned += helpStyle.Render(strings.Repeat("─", sepW)) + "\n\n"

	// Scrollable body: tab content only
	switch m.activeTab {
	case tabCompatibility:
		body = m.renderCompatibilityTab()
	case tabFiles:
		// Update split view dimensions before rendering.
		// Use len(Split()) to match View()'s pinnedHeight calculation — Split includes
		// a trailing empty string so this equals Count()+1, keeping heights aligned.
		pinnedLineCount := len(strings.Split(pinned, "\n"))
		bodyHeight := m.height - pinnedLineCount - 2 // -2 for help bar + margin
		if bodyHeight < 5 {
			bodyHeight = 5
		}
		if m.item.Type == catalog.Loadouts {
			m.loadoutContents.splitView.width = m.width
			m.loadoutContents.splitView.height = bodyHeight
			body = m.renderLoadoutContentsTab()
		} else {
			m.fileViewer.splitView.width = m.width
			m.fileViewer.splitView.height = bodyHeight
			body = m.renderFilesTab()
		}
	case tabInstall:
		if m.item.Type == catalog.Loadouts {
			body = m.renderLoadoutApplyTab()
		} else {
			body = m.renderInstallTab()
		}
	}

	return pinned, body
}

// renderTabBar renders the tab selector (Files | Install).
// For loadouts, tabs are (Contents | Apply).
// For hooks, tabs are (Files | Compat | Install).
func (m detailModel) renderTabBar() string {
	var tabs []struct {
		label string
		tab   detailTab
	}
	switch m.item.Type {
	case catalog.Loadouts:
		tabs = []struct {
			label string
			tab   detailTab
		}{
			{"Contents", tabFiles},
			{"Apply", tabInstall},
		}
	case catalog.Hooks:
		tabs = []struct {
			label string
			tab   detailTab
		}{
			{"Files", tabFiles},
			{"Compat", tabCompatibility},
			{"Install", tabInstall},
		}
	default:
		tabs = []struct {
			label string
			tab   detailTab
		}{
			{"Files", tabFiles},
			{"Install", tabInstall},
		}
	}

	var parts []string
	for _, t := range tabs {
		var rendered string
		if t.tab == m.activeTab {
			rendered = activeTabStyle.Render(t.label)
		} else {
			rendered = inactiveTabStyle.Render(t.label)
		}
		parts = append(parts, zone.Mark(fmt.Sprintf("tab-%d", int(t.tab)), rendered))
	}

	sep := helpStyle.Render(" | ")
	return strings.Join(parts, sep)
}

// renderCompatibilityTab renders the hook compatibility table and feature warnings.
func (m detailModel) renderCompatibilityTab() string {
	if m.item.Type != catalog.Hooks || m.hookData == nil {
		return helpStyle.Render("Compatibility data not available.") + "\n"
	}

	var s string
	s += labelStyle.Render("Provider Compatibility") + "\n\n"

	// Provider summary table
	for _, result := range m.hookCompat {
		sym := compatCellStyle(result.Level).Render(result.Level.Symbol())
		name := providerDisplayName(result.Provider)
		level := result.Level.Label()
		note := result.Notes

		s += fmt.Sprintf("  %s %-12s  %-10s", sym, name, level)
		if note != "" {
			s += "  " + helpStyle.Render(note)
		}
		s += "\n"
	}

	// Feature warnings for broken/none features
	hasWarnings := false
	for _, result := range m.hookCompat {
		for _, fr := range result.Features {
			if !fr.Supported && fr.Present && fr.Impact >= converter.CompatBroken {
				if !hasWarnings {
					s += "\n" + warningStyle.Render("Warnings") + "\n"
					hasWarnings = true
				}
				name := providerDisplayName(result.Provider)
				s += "  " + compatCellStyle(fr.Impact).Render(result.Level.Symbol()+" "+name+": "+fr.Notes) + "\n"
			}
		}
	}

	return s
}

// renderFilesTab renders the split-view file browser (or single-pane fallback).
func (m detailModel) renderFilesTab() string {
	return m.fileViewer.splitView.View()
}

// renderInstallTab renders the install/manage section (providers, method picker, env setup).
func (m detailModel) renderInstallTab() string {
	var s string

	// MCP Server Configuration preview
	if m.item.Type == catalog.MCP && m.mcpConfig != nil {
		s += labelStyle.Render("Server Configuration:") + "\n"
		if m.mcpConfig.Type != "" {
			s += "  " + helpStyle.Render("Type:    ") + valueStyle.Render(m.mcpConfig.Type) + "\n"
		}
		if m.mcpConfig.Command != "" {
			cmd := m.mcpConfig.Command
			if len(m.mcpConfig.Args) > 0 {
				cmd += " " + strings.Join(m.mcpConfig.Args, " ")
			}
			s += "  " + helpStyle.Render("Command: ") + valueStyle.Render(truncate(cmd, m.width-14)) + "\n"
		}
		if m.mcpConfig.URL != "" {
			s += "  " + helpStyle.Render("URL:     ") + valueStyle.Render(truncate(m.mcpConfig.URL, m.width-14)) + "\n"
		}
		if len(m.mcpConfig.Env) > 0 {
			envNames := make([]string, 0, len(m.mcpConfig.Env))
			for name := range m.mcpConfig.Env {
				envNames = append(envNames, name)
			}
			sort.Strings(envNames)
			for _, name := range envNames {
				status := notInstalledStyle.Render("not set")
				if _, ok := os.LookupEnv(name); ok {
					status = installedStyle.Render("set")
				}
				s += "  " + helpStyle.Render("Env:     ") + valueStyle.Render(name) + " " + status + "\n"
			}
		}
		s += "\n"
	}

	// Providers section header
	s += labelStyle.Render("Providers") + "\n"
	s += helpStyle.Render(strings.Repeat("─", 20)) + "\n"

	// Provider section — interactive checkboxes
	{
		supportedProviders := m.supportedProviders()
		detected := m.detectedProviders()

		if len(supportedProviders) > 0 {
			for i, p := range detected {
				status := installer.CheckStatus(m.item, p, m.repoRoot)

				// For hooks: check compat level — CompatNone providers are non-selectable
				var provCompat *converter.CompatResult
				if m.item.Type == catalog.Hooks {
					provCompat = m.hookCompatForProvider(p.Slug)
				}
				isCompatNone := provCompat != nil && provCompat.Level == converter.CompatNone

				check := "[ ]"
				if isCompatNone {
					check = compatCellStyle(converter.CompatNone).Render("[✗]")
				} else if i < len(m.provCheck.checks) && m.provCheck.checks[i] {
					check = installedStyle.Render("[✓]")
				}

				prefix := "  "
				nameStyle := itemStyle
				if i == m.provCheck.cursor {
					prefix = "> "
					nameStyle = selectedItemStyle
				}

				var indicator string
				switch status {
				case installer.StatusInstalled:
					indicator = installedStyle.Render("[ok] installed")
					if m.item.Type == catalog.MCP && m.hasUnsetEnvVars() {
						indicator += " " + warningStyle.Render("[!] needs setup")
					}
				case installer.StatusNotInstalled:
					indicator = notInstalledStyle.Render("[--] available")
				}

				row := fmt.Sprintf("  %s%s %s", prefix, check, nameStyle.Render(p.Name))

				// For hooks: append compat symbol and level label inline
				if m.item.Type == catalog.Hooks && provCompat != nil {
					sym := compatCellStyle(provCompat.Level).Render(provCompat.Level.Symbol())
					row += "  " + sym + " " + provCompat.Level.Label()
				}

				row += "  " + indicator
				s += zone.Mark(fmt.Sprintf("prov-check-%d", i), row) + "\n"
			}

			for _, p := range supportedProviders {
				if p.Detected {
					continue
				}
				name := helpStyle.Render(p.Name)
				// For hooks: show compat for non-detected providers too
				compatPart := ""
				if m.item.Type == catalog.Hooks {
					if cr := m.hookCompatForProvider(p.Slug); cr != nil {
						sym := compatCellStyle(cr.Level).Render(cr.Level.Symbol())
						compatPart = "  " + sym + " " + cr.Level.Label()
					}
				}
				tag := helpStyle.Render("(not detected)")
				s += fmt.Sprintf("      %s%s  %s\n", name, compatPart, tag)
			}
		} else {
			s += helpStyle.Render("No providers support installing this content type yet.") + "\n"
		}
	}

	// Actions section header
	s += "\n"
	s += labelStyle.Render("Actions") + "\n"
	s += helpStyle.Render(strings.Repeat("─", 20)) + "\n"

	// Action buttons — built from item context using semantic styles
	var actionBtns []ActionButton
	actionBtns = append(actionBtns, ActionButton{"i", "Install", "detail-btn-install", actionBtnAddStyle})
	actionBtns = append(actionBtns, ActionButton{"u", "Uninstall", "detail-btn-uninstall", actionBtnUninstallStyle})
	if m.item.Library {
		actionBtns = append(actionBtns, ActionButton{"c", "Copy", "detail-btn-copy", actionBtnDefaultStyle})
	}
	if m.item.Library {
		actionBtns = append(actionBtns, ActionButton{"s", "Save", "detail-btn-save", actionBtnDefaultStyle})
	}
	if m.item.Type == catalog.MCP && m.hasUnsetEnvVars() {
		actionBtns = append(actionBtns, ActionButton{"e", "Env Setup", "detail-btn-env", actionBtnDefaultStyle})
	}
	if m.item.Library {
		actionBtns = append(actionBtns, ActionButton{"p", "Share", "detail-btn-share", actionBtnDefaultStyle})
	}
	s += renderActionButtons(actionBtns...) + "\n"

	return s
}

// renderLoadoutContentsTab renders the split-view contents browser for loadouts.
func (m detailModel) renderLoadoutContentsTab() string {
	if m.loadoutManifestErr != "" {
		return errorMsgStyle.Render("Error parsing loadout: "+m.loadoutManifestErr) + "\n"
	}
	if m.loadoutManifest == nil {
		return helpStyle.Render("No loadout manifest found.") + "\n"
	}

	return m.loadoutContents.splitView.View()
}

// renderLoadoutApplyTab shows the apply mode selector (Preview / Try / Keep).
// The user picks a mode and presses Enter to trigger the apply flow.
//
// Why three modes:
//   - Preview: dry run, shows what would happen without touching files
//   - Try: applies temporarily, auto-reverts when the session ends
//   - Keep: applies permanently until explicitly removed
func (m detailModel) renderLoadoutApplyTab() string {
	if m.loadoutManifestErr != "" {
		return errorMsgStyle.Render("Error parsing loadout: "+m.loadoutManifestErr) + "\n"
	}
	if m.loadoutManifest == nil {
		return helpStyle.Render("No loadout manifest found.") + "\n"
	}

	var s string
	s += labelStyle.Render("Apply Loadout") + "\n"
	s += helpStyle.Render("  Choose a mode:") + "\n\n"

	type modeOpt struct {
		name string
		desc string
	}
	modes := []modeOpt{
		{"Preview", "Dry run — show what would be applied without changing anything"},
		{"Try", "Apply temporarily — auto-reverts when the session ends"},
		{"Keep", "Apply permanently — stays until you run: syllago loadout remove"},
	}

	for i, mode := range modes {
		prefix := "  "
		nameStyle := itemStyle
		if i == m.loadoutModeCursor {
			prefix = "> "
			nameStyle = selectedItemStyle
		}
		nameLine := fmt.Sprintf("  %s%s", prefix, nameStyle.Render(mode.name))
		s += zone.Mark(fmt.Sprintf("detail-mode-%d", i), nameLine) + "\n"
		s += fmt.Sprintf("      %s\n", helpStyle.Render(mode.desc))
	}

	s += "\n"
	applyBtns := []ActionButton{
		{"enter", "Apply", "detail-btn-apply", actionBtnAddStyle},
	}
	s += renderActionButtons(applyBtns...)

	return s
}

func (m detailModel) View() string {
	pinned, body := m.renderContentSplit()

	pinnedLines := strings.Split(pinned, "\n")
	pinnedHeight := len(pinnedLines)

	bodyLines := strings.Split(body, "\n")

	messageLines := 0
	if m.message != "" {
		messageLines = 1
	}

	// Scrollable area = total height minus pinned header, message, help bar, margins
	visibleHeight := m.height - pinnedHeight - messageLines - 2
	if visibleHeight < 1 {
		visibleHeight = len(bodyLines)
	}

	maxOffset := len(bodyLines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	// Use a local clamped offset (View has a value receiver so mutations are discarded;
	// persistent clamping happens in Update via clampScroll).
	offset := m.scrollOffset
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + visibleHeight
	if end > len(bodyLines) {
		end = len(bodyLines)
	}

	s := pinned // always show pinned header

	if offset > 0 {
		s += renderScrollUp(offset, true) + "\n"
		s += strings.Join(bodyLines[offset:end], "\n")
	} else {
		s += strings.Join(bodyLines[offset:end], "\n")
	}

	if end < len(bodyLines) {
		s += "\n" + renderScrollDown(len(bodyLines)-end, true)
	}

	return s
}

func (m detailModel) helpText() string {
	var helpParts []string
	helpParts = append(helpParts, "esc back", "tab switch tab")

	switch m.activeTab {
	case tabCompatibility:
		// no additional help for placeholder
	case tabFiles:
		if m.item.Type == catalog.Loadouts {
			if m.loadoutContents.splitView.showingPreview {
				helpParts = append(helpParts, "up/down scroll", "esc back to contents")
			} else if m.loadoutContents.splitView.IsSplit() {
				if m.loadoutContents.splitView.FocusedPane() == panePreview {
					helpParts = append(helpParts, "up/down scroll", "h/left contents")
				} else {
					helpParts = append(helpParts, "up/down navigate", "l/right preview")
				}
			} else if len(m.loadoutContents.splitView.items) > 0 {
				helpParts = append(helpParts, "up/down navigate", "enter view")
			}
		} else if m.fileViewer.splitView.showingPreview {
			helpParts = append(helpParts, "up/down scroll", "esc back to files")
		} else if m.fileViewer.splitView.IsSplit() {
			if m.fileViewer.splitView.FocusedPane() == panePreview {
				helpParts = append(helpParts, "up/down scroll", "h/left tree")
			} else {
				helpParts = append(helpParts, "up/down navigate", "l/right preview")
			}
		} else if len(m.item.Files) > 0 {
			helpParts = append(helpParts, "up/down navigate", "enter view")
		}
	case tabInstall:
		if m.item.Type == catalog.Loadouts {
			helpParts = append(helpParts, "up/down navigate", "enter apply")
		} else {
			if len(m.provCheck.checks) > 0 {
				helpParts = append(helpParts, "up/down navigate", "enter/space toggle", "i install", "u uninstall")
				if m.item.Type == catalog.MCP && m.hasUnsetEnvVars() {
					helpParts = append(helpParts, "e env setup")
				}
			}
		}
	}

	if isRemovable(m.item) {
		helpParts = append(helpParts, "r remove "+contentTypeSingular(m.item.Type))
	}

	if m.item.Library {
		if m.llmPrompt != "" {
			helpParts = append(helpParts, "c copy prompt")
		}
		helpParts = append(helpParts, "p share")
	}

	if m.listTotal > 1 {
		helpParts = append(helpParts, "ctrl+n/p next/prev")
	}

	return strings.Join(helpParts, " • ")
}
