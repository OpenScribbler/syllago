package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/installer"
)

// renderContent builds the full detail content (without scrolling or help bar).
func (m detailModel) renderContent() string {
	name := StripControlChars(displayName(m.item))
	position := ""
	if m.listTotal > 0 {
		position = fmt.Sprintf(" (%d of %d)", m.listPosition+1, m.listTotal)
	}
	s := helpStyle.Render("nesco > "+m.item.Type.Label()+" >") + " " + titleStyle.Render(name) + helpStyle.Render(position)
	if m.item.Local {
		s += " " + warningStyle.Render("[LOCAL]")
	}
	s += "\n"

	// Description (always shown above tabs)
	if m.item.Description != "" {
		desc := StripControlChars(m.item.Description)
		if len(desc) > 200 {
			desc = desc[:197] + "..."
		}
		s += valueStyle.Render(desc) + "\n"
	}

	// Tab bar
	s += "\n" + m.renderTabBar() + "\n\n"

	// Tab content
	switch m.activeTab {
	case tabOverview:
		s += m.renderOverviewTab()
	case tabFiles:
		s += m.renderFilesTab()
	case tabInstall:
		s += m.renderInstallTab()
	}

	return s
}

// renderTabBar renders the tab selector: [Overview]  Files  Install
func (m detailModel) renderTabBar() string {
	tabs := []struct {
		label string
		tab   detailTab
	}{
		{"Overview", tabOverview},
		{"Files", tabFiles},
		{"Install", tabInstall},
	}

	var parts []string
	for _, t := range tabs {
		label := t.label
		if t.tab == m.activeTab {
			label = selectedItemStyle.Render("[" + label + "]")
		} else {
			label = helpStyle.Render(" " + label + " ")
		}
		parts = append(parts, label)
	}

	return strings.Join(parts, "  ")
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

	// Metadata
	s += "\n"
	s += labelStyle.Render("Type: ") + valueStyle.Render(m.item.Type.Label()) + "\n"
	s += labelStyle.Render("Path: ") + valueStyle.Render(m.item.Path) + "\n"
	if m.item.Provider != "" {
		s += labelStyle.Render("Provider: ") + valueStyle.Render(m.item.Provider) + "\n"
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
	if m.viewingFile {
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
		if i == m.fileCursor {
			prefix = "> "
			style = selectedItemStyle
		}
		s += fmt.Sprintf("  %s%s\n", prefix, style.Render(f))
	}
	return s
}

// renderFileContent shows the content of the selected file with line numbers.
func (m detailModel) renderFileContent() string {
	if m.fileCursor >= len(m.item.Files) {
		return ""
	}

	relPath := m.item.Files[m.fileCursor]
	s := labelStyle.Render(relPath) + "\n\n"

	lines := strings.Split(m.fileContent, "\n")

	// Apply scroll offset
	visibleHeight := m.height - 8 // header + tab bar + file header + help bar + margins
	if visibleHeight < 5 {
		visibleHeight = len(lines)
	}

	offset := m.fileScrollOffset
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
				if i < len(m.providerChecks) && m.providerChecks[i] {
					check = installedStyle.Render("[x]")
				}

				prefix := "  "
				nameStyle := itemStyle
				if i == m.checkCursor && m.confirmAction == actionNone {
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

	// Method picker
	if m.confirmAction == actionChooseMethod || m.confirmAction == actionSaveMethod {
		label := "Install method:"
		if m.confirmAction == actionSaveMethod {
			label = "Save method:"
		}
		s += "\n" + labelStyle.Render(label) + "\n"

		type methodOption struct {
			name string
			desc string
		}
		methods := []methodOption{
			{"Symlink (recommended)", "Stays in sync with repo. Auto-updates on git pull."},
			{"Copy", "Independent copy. Won't change when repo updates."},
		}

		for i, method := range methods {
			prefix := "  "
			nameStyle := itemStyle
			if i == m.methodCursor {
				prefix = "> "
				nameStyle = selectedItemStyle
			}
			s += fmt.Sprintf("  %s%s\n", prefix, nameStyle.Render(method.name))
			s += fmt.Sprintf("      %s\n", helpStyle.Render(method.desc))
		}

		// Show destination paths for checked providers
		if m.confirmAction == actionChooseMethod {
			detected := m.detectedProviders()
			home, err := os.UserHomeDir()
			if err == nil {
				s += "\n" + helpStyle.Render("Destination paths:") + "\n"
				for i, checked := range m.providerChecks {
					if !checked || i >= len(detected) {
						continue
					}
					p := detected[i]
					if installer.IsJSONMerge(p, m.item.Type) {
						s += "  " + helpStyle.Render(p.Name+": ") + valueStyle.Render("(merged into config)") + "\n"
					} else {
						destDir := p.InstallDir(home, m.item.Type)
						dest := filepath.Join(destDir, m.item.Name)
						s += "  " + helpStyle.Render(p.Name+": ") + valueStyle.Render(dest) + "\n"
					}
				}
			}
		}

		confirmKey := "i/enter"
		if m.confirmAction == actionSaveMethod {
			confirmKey = "enter"
		}
		s += "\n" + helpStyle.Render(fmt.Sprintf("up/down select • %s confirm • esc cancel", confirmKey)) + "\n"
	}

	// Save path input
	if m.confirmAction == actionSavePath {
		s += "\n" + m.saveInput.View() + "\n"
		s += helpStyle.Render("enter confirm • esc cancel") + "\n"
	}

	// Env var interactive setup
	if m.envVarIdx < len(m.envVarNames) {
		envName := m.envVarNames[m.envVarIdx]
		switch m.confirmAction {
		case actionEnvChoose:
			s += "\n" + labelStyle.Render("Environment Variable Setup") + "\n"
			s += helpStyle.Render(fmt.Sprintf("  %s (%d of %d)", envName, m.envVarIdx+1, len(m.envVarNames))) + "\n\n"

			options := []string{"Set up new value", "I already have it configured"}
			for i, opt := range options {
				prefix := "  "
				style := itemStyle
				if i == m.envMethodCursor {
					prefix = "> "
					style = selectedItemStyle
				}
				s += fmt.Sprintf("  %s%s\n", prefix, style.Render(opt))
			}
			s += "\n" + helpStyle.Render("  up/down select • enter choose • esc skip") + "\n"

		case actionEnvValue:
			s += "\n" + labelStyle.Render("Environment Variable Setup") + "\n"
			s += helpStyle.Render(fmt.Sprintf("  %s (%d of %d)", envName, m.envVarIdx+1, len(m.envVarNames))) + "\n\n"
			s += "  " + m.envInput.View() + "\n"
			s += "\n" + helpStyle.Render("  enter next • esc back") + "\n"

		case actionEnvLocation:
			s += "\n" + labelStyle.Render("Environment Variable Setup") + "\n"
			s += helpStyle.Render(fmt.Sprintf("  Save %s to:", envName)) + "\n\n"
			s += "  " + m.envInput.View() + "\n"
			s += "\n" + helpStyle.Render("  enter save • esc back") + "\n"

		case actionEnvSource:
			s += "\n" + labelStyle.Render("Environment Variable Setup") + "\n"
			s += helpStyle.Render(fmt.Sprintf("  Load %s from an existing file:", envName)) + "\n\n"
			s += "  " + m.envInput.View() + "\n"
			s += "\n" + helpStyle.Render("  enter load • esc back") + "\n"
		}
	}

	// App install.sh preview
	if m.confirmAction == actionAppScriptConfirm {
		s += "\n" + warningStyle.Render("WARNING: This will execute a shell script") + "\n\n"
		s += labelStyle.Render("install.sh preview (first 20 lines):") + "\n"
		s += helpStyle.Render("───") + "\n"

		for _, line := range strings.Split(StripControlChars(m.appScriptPreview), "\n") {
			s += helpStyle.Render(line) + "\n"
		}

		s += helpStyle.Render("───") + "\n\n"
		if len(strings.Split(m.appScriptPreview, "\n")) >= 20 {
			s += helpStyle.Render("(script continues below...)") + "\n\n"
		}
		s += helpStyle.Render("Press i again to execute, esc to cancel") + "\n"
		return s
	}

	// Confirmation prompts
	switch m.confirmAction {
	case actionUninstall:
		installed := m.installedProviders()
		var names []string
		for _, p := range installed {
			names = append(names, p.Name)
		}
		s += "\n" + helpStyle.Render(fmt.Sprintf("Uninstall from %s? Press u to confirm, esc to cancel", strings.Join(names, ", "))) + "\n"
	case actionPromoteConfirm:
		s += "\n" + warningStyle.Render("Promote to shared?") + "\n"
		s += helpStyle.Render("This creates a branch, commits, pushes, and opens a PR.") + "\n"
		s += helpStyle.Render("Press p again to confirm, esc to cancel.") + "\n"
	}

	return s
}

func (m detailModel) View() string {
	content := m.renderContent()
	lines := strings.Split(content, "\n")

	helpBar := m.renderHelp()

	// Reserve space for message line if present (outside scrollable area)
	messageLines := 0
	if m.message != "" {
		messageLines = 1
	}

	visibleHeight := m.height - 2 - messageLines
	if visibleHeight < 1 {
		visibleHeight = len(lines)
	}

	maxOffset := len(lines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	// Use a local clamped offset (View has a value receiver so mutations are discarded;
	// persistent clamping happens in Update via clampScroll).
	offset := m.scrollOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	var s string

	if offset > 0 {
		visible := lines[offset:end]
		s = helpStyle.Render(fmt.Sprintf("(%d lines above)", offset)) + "\n"
		s += strings.Join(visible, "\n")
	} else {
		visible := lines[offset:end]
		s = strings.Join(visible, "\n")
	}

	if end < len(lines) {
		s += "\n" + helpStyle.Render(fmt.Sprintf("(%d lines below)", len(lines)-end))
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
		if m.item.Type == catalog.Prompts && m.item.Body != "" && m.confirmAction == actionNone {
			helpParts = append(helpParts, "c copy")
		}
	case tabFiles:
		if m.viewingFile {
			helpParts = append(helpParts, "up/down scroll", "esc back to files")
		} else if len(m.item.Files) > 0 {
			helpParts = append(helpParts, "up/down navigate", "enter view")
		}
	case tabInstall:
		if m.item.Type == catalog.Prompts {
			helpParts = append(helpParts, "up/down scroll")
			if m.item.Body != "" && m.confirmAction == actionNone {
				helpParts = append(helpParts, "c copy", "s save")
			}
		} else if m.item.Type == catalog.Apps {
			helpParts = append(helpParts, "up/down scroll", "i install", "u uninstall")
		} else {
			if len(m.providerChecks) > 0 && m.confirmAction == actionNone {
				helpParts = append(helpParts, "up/down navigate", "enter/space toggle", "i install", "u uninstall")
				if m.item.Type == catalog.MCP && m.hasUnsetEnvVars() {
					helpParts = append(helpParts, "e env setup")
				}
			}
		}
	}

	if m.item.Local && m.confirmAction == actionNone {
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
