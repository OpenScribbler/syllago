package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/installer"
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

	// Breadcrumb: Home > Category > Item Name
	home := zone.Mark("crumb-home", helpStyle.Render("Home"))
	cat := zone.Mark("crumb-category", helpStyle.Render(m.item.Type.Label()))
	arrow := helpStyle.Render(" > ")
	current := titleStyle.Render(name)
	if m.item.IsBuiltin() {
		current += " " + builtinStyle.Render("[BUILT-IN]")
	} else if m.item.Local {
		current += " " + warningStyle.Render("[LOCAL]")
	} else if m.item.Registry != "" {
		current += " " + countStyle.Render("["+m.item.Registry+"]")
	}
	pinned += home + arrow + cat + arrow + current + "\n\n"

	// Tab bar
	pinned += m.renderTabBar() + "\n"

	// Metadata block (always visible, below tabs)
	pinned += labelStyle.Render("Type: ") + valueStyle.Render(m.item.Type.Label())
	if m.item.IsBuiltin() {
		pinned += "  " + builtinStyle.Render("[Built-in]")
	} else if m.item.Local {
		pinned += "  " + warningStyle.Render("[Local]")
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
			} else if ov.Local {
				source = "local"
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

	// Prompt body (for prompts type)
	if m.item.Type == catalog.Prompts && m.item.Body != "" {
		s += labelStyle.Render("Prompt:") + "\n"
		s += valueStyle.Render(StripControlChars(m.item.Body)) + "\n\n"
	}

	// App supported providers
	if m.item.Type == catalog.Apps && len(m.item.SupportedProviders) > 0 {
		var names []string
		for _, slug := range m.item.SupportedProviders {
			names = append(names, providerDisplayName(slug))
		}
		s += labelStyle.Render("Supported Providers: ") + valueStyle.Render(strings.Join(names, ", ")) + "\n\n"
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
	} else if m.item.Type == catalog.Apps && m.renderedBody != "" {
		// Apps use Body field (from README.md frontmatter body)
		s += m.renderedBody
	} else if m.item.Type == catalog.Apps && m.item.Body != "" {
		s += valueStyle.Render(m.item.Body) + "\n"
	} else {
		s += helpStyle.Render("No README.md available for this item.") + "\n"
	}

	// LLM Prompt (for scaffolded local items)
	if m.item.Local && m.llmPrompt != "" {
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
		s += fmt.Sprintf("  %s%s\n", prefix, style.Render(f))
	}
	return s
}

// renderFileContent shows the content of the selected file with line numbers.
func (m detailModel) renderFileContent() string {
	if m.fileViewer.cursor >= len(m.item.Files) {
		return ""
	}

	relPath := m.item.Files[m.fileViewer.cursor]
	s := labelStyle.Render(relPath) + "\n\n"

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
		s += helpStyle.Render(fmt.Sprintf("(%d lines above)", offset)) + "\n"
	}

	for i := offset; i < end; i++ {
		lineNum := helpStyle.Render(fmt.Sprintf("%4d ", i+1))
		s += lineNum + valueStyle.Render(StripControlChars(lines[i])) + "\n"
	}

	if end < len(lines) {
		s += helpStyle.Render(fmt.Sprintf("(%d lines below)", len(lines)-end)) + "\n"
	}

	return s
}

// renderInstallTab renders the install/manage section (providers, method picker, env setup).
func (m detailModel) renderInstallTab() string {
	var s string

	// Action button bar — rendered in the content area so mouse clicks are targetable
	installBtn := zone.Mark("detail-btn-install", helpStyle.Render("[i]nstall"))
	uninstallBtn := zone.Mark("detail-btn-uninstall", helpStyle.Render("[u]ninstall"))
	copyBtn := zone.Mark("detail-btn-copy", helpStyle.Render("[c]opy"))
	saveBtn := zone.Mark("detail-btn-save", helpStyle.Render("[s]ave"))
	actionBar := installBtn + "  " + uninstallBtn + "  " + copyBtn + "  " + saveBtn
	s += actionBar + "\n\n"

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
			s += "  " + helpStyle.Render("Command: ") + valueStyle.Render(cmd) + "\n"
		}
		if m.mcpConfig.URL != "" {
			s += "  " + helpStyle.Render("URL:     ") + valueStyle.Render(m.mcpConfig.URL) + "\n"
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

	// Provider section — interactive checkboxes for non-prompt items
	if m.item.Type != catalog.Prompts {
		supportedProviders := m.supportedProviders()
		detected := m.detectedProviders()

		if len(supportedProviders) > 0 {
			s += labelStyle.Render("Providers:") + "\n"

			for i, p := range detected {
				status := installer.CheckStatus(m.item, p, m.repoRoot)

				check := "[ ]"
				if i < len(m.provCheck.checks) && m.provCheck.checks[i] {
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

				s += fmt.Sprintf("  %s%s %s  %s\n", prefix, check, nameStyle.Render(p.Name), indicator)
			}

			for _, p := range supportedProviders {
				if p.Detected {
					continue
				}
				name := helpStyle.Render(p.Name)
				tag := helpStyle.Render("(not detected)")
				s += fmt.Sprintf("      %s  %s\n", name, tag)
			}
		} else {
			s += helpStyle.Render("No providers support installing this content type yet.") + "\n"
		}
	}

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
		{"Prompts", manifest.Prompts},
		{"Apps", manifest.Apps},
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
		{"Keep", "Apply permanently — stays until you run: nesco loadout remove"},
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
	helpBar := m.renderHelp()

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
		s += helpStyle.Render(fmt.Sprintf("(%d lines above)", offset)) + "\n"
		s += strings.Join(bodyLines[offset:end], "\n")
	} else {
		s += strings.Join(bodyLines[offset:end], "\n")
	}

	if end < len(bodyLines) {
		s += "\n" + helpStyle.Render(fmt.Sprintf("(%d lines below)", len(bodyLines)-end))
	}

	// Status message — rendered outside scrollable area so it's always visible
	if m.message != "" {
		if m.messageIsErr {
			s += "\n" + errorMsgStyle.Render("Error: "+m.message)
		} else {
			s += "\n" + successMsgStyle.Render("Done: "+m.message)
		}
	}

	s += "\n" + helpBar
	return s
}

func (m detailModel) renderHelp() string {
	var helpParts []string
	helpParts = append(helpParts, "esc back", "tab switch tab")

	switch m.activeTab {
	case tabOverview:
		helpParts = append(helpParts, "up/down scroll")
		if m.item.Type == catalog.Prompts && m.item.Body != "" {
			helpParts = append(helpParts, "c copy")
		}
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
		} else if m.item.Type == catalog.Prompts {
			helpParts = append(helpParts, "up/down scroll")
			if m.item.Body != "" {
				helpParts = append(helpParts, "c copy", "s save")
			}
		} else if m.item.Type == catalog.Apps {
			helpParts = append(helpParts, "up/down scroll", "i install", "u uninstall")
		} else {
			if len(m.provCheck.checks) > 0 {
				helpParts = append(helpParts, "up/down navigate", "enter/space toggle", "i install", "u uninstall")
				if m.item.Type == catalog.MCP && m.hasUnsetEnvVars() {
					helpParts = append(helpParts, "e env setup")
				}
			}
		}
	}

	if m.item.Local {
		if m.llmPrompt != "" {
			helpParts = append(helpParts, "c copy prompt")
		}
		helpParts = append(helpParts, "p promote")
	}

	if m.listTotal > 1 {
		helpParts = append(helpParts, "ctrl+n/p next/prev")
	}

	return helpStyle.Render(strings.Join(helpParts, " • "))
}
