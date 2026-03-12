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

// renderContent builds the full detail content (without scrolling or help bar).
// Returns the combined string of pinned header and scrollable body.
func (m detailModel) renderContent() string {
	pinned, body := m.renderContentSplit()
	return pinned + body
}

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
	segments = append(segments, BreadcrumbSegment{m.item.Type.Label(), "crumb-category"})
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
	case tabOverview:
		body = m.renderOverviewTab()
	case tabCompatibility:
		body = m.renderCompatibilityTab()
	case tabFiles:
		if m.item.Type == catalog.Loadouts {
			body = m.renderLoadoutContentsTab()
		} else {
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

// renderTabBar renders the tab selector (Overview | Files | Install).
// For loadouts, tabs are (Overview | Contents | Apply).
// For hooks, tabs are (Overview | Compat | Files | Install).
func (m detailModel) renderTabBar() string {
	var tabs []struct {
		label string
		tab   detailTab
	}
	if m.item.Type == catalog.Loadouts {
		tabs = []struct {
			label string
			tab   detailTab
		}{
			{"Overview", tabOverview},
			{"Contents", tabFiles},
			{"Apply", tabInstall},
		}
	} else if m.item.Type == catalog.Hooks {
		tabs = []struct {
			label string
			tab   detailTab
		}{
			{"Overview", tabOverview},
			{"Compat", tabCompatibility},
			{"Files", tabFiles},
			{"Install", tabInstall},
		}
	} else {
		tabs = []struct {
			label string
			tab   detailTab
		}{
			{"Overview", tabOverview},
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

// renderOverviewTab renders the README.md content (or falls back to description/body).
func (m detailModel) renderOverviewTab() string {
	var s string

	// Hook-specific metadata (shown at top for hook items)
	if m.item.Type == catalog.Hooks && m.hookData != nil {
		hd := m.hookData
		s += labelStyle.Render("Event:   ") + valueStyle.Render(hd.Event) + "\n"
		if hd.Matcher != "" {
			s += labelStyle.Render("Matcher: ") + valueStyle.Render(hd.Matcher) + "\n"
		}
		if len(hd.Hooks) > 0 {
			h := hd.Hooks[0]
			s += labelStyle.Render("Type:    ") + valueStyle.Render(h.Type) + "\n"
			if h.Command != "" {
				cmdDisplay := truncate(h.Command, m.width-12)
				s += labelStyle.Render("Command: ") + valueStyle.Render(cmdDisplay) + "\n"
			}
			if h.Timeout > 0 {
				s += labelStyle.Render("Timeout: ") + valueStyle.Render(fmt.Sprintf("%dms", h.Timeout)) + "\n"
			}
			if h.Async {
				s += labelStyle.Render("Async:   ") + valueStyle.Render("true") + "\n"
			}
		}
		s += "\n"
	}

	// Risk indicators (shown after description, before README)
	risks := catalog.RiskIndicators(m.item)
	if len(risks) > 0 {
		s += "\n"
		for _, r := range risks {
			s += warningStyle.Render("⚠  "+r.Label) + "\n"
			s += helpStyle.Render("   "+r.Description) + "\n"
		}
		s += "\n"
	}

	// Rendered README (if available)
	if m.renderedReadme != "" {
		s += m.renderedReadme
	} else {
		s += helpStyle.Render("No README.md available for this item.") + "\n"
	}

	// LLM Prompt (for scaffolded library items)
	if m.item.Library && m.llmPrompt != "" {
		s += "\n" + labelStyle.Render("LLM Prompt Available") + " " + helpStyle.Render("(press c to copy)") + "\n"
		lines := strings.Split(m.llmPrompt, "\n")
		preview := lines
		if len(preview) > 8 {
			preview = preview[:8]
		}
		for _, line := range preview {
			s += helpStyle.Render("  "+line) + "\n"
		}
		if len(lines) > 8 {
			s += helpStyle.Render(fmt.Sprintf("  ... (%d more lines)", len(lines)-8)) + "\n"
		}
	}

	return s
}

// renderCompatibilityTab renders the hook compatibility table and feature warnings.
func (m detailModel) renderCompatibilityTab() string {
	if m.item.Type != catalog.Hooks || m.hookData == nil {
		return helpStyle.Render("Compatibility data not available.") + "\n"
	}

	var s string
	s += labelStyle.Render("Provider Compatibility") + "\n\n"

	// Provider summary table
	providerNames := map[string]string{
		"claude-code": "Claude Code",
		"gemini-cli":  "Gemini CLI",
		"copilot-cli": "Copilot CLI",
		"kiro":        "Kiro",
	}

	for _, result := range m.hookCompat {
		sym := compatCellStyle(result.Level).Render(result.Level.Symbol())
		name := providerNames[result.Provider]
		if name == "" {
			name = result.Provider
		}
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
				name := providerNames[result.Provider]
				s += "  " + compatCellStyle(fr.Impact).Render(result.Level.Symbol()+" "+name+": "+fr.Notes) + "\n"
			}
		}
	}

	return s
}

// renderFilesTab renders the file list or file content viewer.
func (m detailModel) renderFilesTab() string {
	if m.fileViewer.viewing {
		return m.renderFileContent()
	}
	return m.renderFileList()
}

// renderFileList shows the list of files in the item directory.
func (m detailModel) renderFileList() string {
	if len(m.item.Files) == 0 {
		return helpStyle.Render("No files in this item.") + "\n"
	}

	var s string
	for i, f := range m.item.Files {
		prefix := "  "
		style := itemStyle
		if i == m.fileViewer.cursor {
			prefix = "> "
			style = selectedItemStyle
		}
		entry := fmt.Sprintf("  %s%s", prefix, style.Render(f))
		s += zone.Mark(fmt.Sprintf("file-%d", i), entry) + "\n"
	}
	return s
}

// renderFileContent shows the content of the selected file with line numbers.
func (m detailModel) renderFileContent() string {
	if m.fileViewer.cursor >= len(m.item.Files) {
		return ""
	}

	relPath := m.item.Files[m.fileViewer.cursor]
	backLink := zone.Mark("file-back", backLinkStyle.Render("← Back to files"))
	s := backLink + "  " + labelStyle.Render(relPath) + "\n\n"

	lines := strings.Split(m.fileViewer.content, "\n")

	// Apply scroll offset
	visibleHeight := m.height - 8 // header + tab bar + file header + help bar + margins
	if visibleHeight < 5 {
		visibleHeight = len(lines)
	}

	offset := m.fileViewer.scrollOffset
	maxOffset := len(lines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	if offset > 0 {
		s += renderScrollUp(offset, true) + "\n"
	}

	for i := offset; i < end; i++ {
		lineNum := helpStyle.Render(fmt.Sprintf("%4d ", i+1))
		s += lineNum + valueStyle.Render(StripControlChars(lines[i])) + "\n"
	}

	if end < len(lines) {
		s += renderScrollDown(len(lines)-end, true) + "\n"
	}

	return s
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

	// Action buttons — styled with background color, conditionally shown
	installBtn := zone.Mark("detail-btn-install", buttonStyle.Render("Install"))
	uninstallBtn := zone.Mark("detail-btn-uninstall", buttonStyle.Render("Uninstall"))

	actionBar := installBtn + "  " + uninstallBtn

	// Copy is functional for library items with LLM prompts
	if m.item.Library {
		copyBtn := zone.Mark("detail-btn-copy", buttonStyle.Render("Copy"))
		actionBar += "  " + copyBtn
	}
	s += actionBar + "\n"

	return s
}

// renderLoadoutContentsTab shows the loadout manifest items grouped by type.
// This gives the user a quick overview of what a loadout includes without
// needing to read the raw loadout.yaml file.
func (m detailModel) renderLoadoutContentsTab() string {
	if m.loadoutManifestErr != "" {
		return errorMsgStyle.Render("Error parsing loadout: "+m.loadoutManifestErr) + "\n"
	}
	if m.loadoutManifest == nil {
		return helpStyle.Render("No loadout manifest found.") + "\n"
	}

	manifest := m.loadoutManifest
	var s string

	s += labelStyle.Render("Loadout Contents") + "\n"
	s += helpStyle.Render(fmt.Sprintf("  Provider: %s  |  %d items total", manifest.Provider, manifest.ItemCount())) + "\n\n"

	// Show items grouped by type, in display order. The order matches
	// AllContentTypes() so the user sees a consistent layout.
	typeOrder := []struct {
		label string
		items []string
	}{
		{"Rules", manifest.Rules},
		{"Hooks", manifest.Hooks},
		{"Skills", manifest.Skills},
		{"Agents", manifest.Agents},
		{"MCP Configs", manifest.MCP},
		{"Commands", manifest.Commands},
	}

	for _, group := range typeOrder {
		if len(group.items) == 0 {
			continue
		}
		s += labelStyle.Render(fmt.Sprintf("  %s (%d):", group.label, len(group.items))) + "\n"
		for _, name := range group.items {
			s += "    " + valueStyle.Render(name) + "\n"
		}
		s += "\n"
	}

	return s
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
	s += helpStyle.Render("  Choose a mode and press Enter:") + "\n\n"

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
		s += fmt.Sprintf("  %s%s\n", prefix, nameStyle.Render(mode.name))
		s += fmt.Sprintf("      %s\n", helpStyle.Render(mode.desc))
	}

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
	case tabOverview:
		helpParts = append(helpParts, "up/down scroll")
	case tabCompatibility:
		// no additional help for placeholder
	case tabFiles:
		if m.item.Type == catalog.Loadouts {
			helpParts = append(helpParts, "up/down scroll")
		} else if m.fileViewer.viewing {
			helpParts = append(helpParts, "up/down scroll", "esc back to files")
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
