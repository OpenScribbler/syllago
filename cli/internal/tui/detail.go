package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/installer"
	"github.com/OpenScribbler/nesco/cli/internal/promote"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
)

type appInstallDoneMsg struct {
	err    error
	action string // "install" or "uninstall"
}

type promoteDoneMsg struct {
	result *promote.Result
	err    error
}

// openSaveModalMsg is sent by detailModel to ask App to open the save modal.
type openSaveModalMsg struct{}

// openEnvModalMsg is sent by detailModel to ask App to open the env setup wizard modal.
type openEnvModalMsg struct {
	envTypes []string
}

// detailTab represents the active tab on the detail screen.
type detailTab int

const (
	tabOverview detailTab = iota
	tabFiles
	tabInstall
)

type detailModel struct {
	item         catalog.ContentItem
	providers    []provider.Provider
	repoRoot     string
	message      string
	messageIsErr bool
	methodCursor int                  // 0=symlink, 1=copy (for save-method picker)
	mcpConfig    *installer.MCPConfig // parsed on creation for MCP items
	scrollOffset int
	saveInput    textinput.Model
	savePath     string // confirmed save destination (after path input)
	// Sub-models for grouped concerns
	provCheck        provCheckModel // provider checkbox state (Install tab)
	appScriptPreview string         // first N lines of install.sh for preview
	renderedBody     string         // cached glamour-rendered README body (apps only)
	renderedReadme   string         // cached glamour-rendered README.md (all types)
	llmPrompt        string         // loaded from LLM-PROMPT.md for local scaffolded items
	// Override info
	overrides []catalog.ContentItem // lower-precedence items this one shadows
	// Tab state
	activeTab    detailTab
	fileViewer   fileViewerModel // file viewer state (Files tab)
	listPosition int             // 0-based position in the items list (for breadcrumb)
	listTotal    int             // total items in the list
	width        int
	height       int
}

func newDetailModel(item catalog.ContentItem, providers []provider.Provider, repoRoot string) detailModel {
	ti := textinput.New()
	ti.Prompt = labelStyle.Render("Save to: ")
	ti.CharLimit = 200

	m := detailModel{
		item:      item,
		providers: providers,
		repoRoot:  repoRoot,
		saveInput: ti,
	}
	// Parse MCP config for preview
	if item.Type == catalog.MCP {
		cfg, _ := installer.ParseMCPConfig(item.Path)
		m.mcpConfig = cfg
	}
	// Pre-render app README body with glamour (cached for performance)
	if item.Type == catalog.Apps && item.Body != "" {
		rendered, err := glamour.Render(item.Body, "auto")
		if err == nil {
			m.renderedBody = rendered
		}
	}
	// Pre-render ReadmeBody for all types (used in Overview tab)
	if item.ReadmeBody != "" {
		rendered, err := glamour.Render(item.ReadmeBody, "auto")
		if err == nil {
			m.renderedReadme = rendered
		}
	}
	// Load LLM prompt for local items
	if item.Local {
		llmPath := filepath.Join(item.Path, "LLM-PROMPT.md")
		if data, err := os.ReadFile(llmPath); err == nil {
			m.llmPrompt = string(data)
		}
	}
	// Initialize provider checkboxes for non-prompt items
	// Pre-check any providers where the item is already installed
	if item.Type != catalog.Prompts {
		detected := m.detectedProviders()
		m.provCheck.checks = make([]bool, len(detected))
		for i, p := range detected {
			status := installer.CheckStatus(item, p, repoRoot)
			m.provCheck.checks[i] = status == installer.StatusInstalled
		}
	}
	return m
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case appInstallDoneMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("%s failed: %s", msg.action, msg.err)
			m.messageIsErr = true
		} else {
			m.message = fmt.Sprintf("%s completed successfully", msg.action)
			m.messageIsErr = false
		}
		return m, nil
	case promoteDoneMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Promote failed: %s", msg.err)
			m.messageIsErr = true
		} else {
			url := msg.result.PRUrl
			if url == "" {
				url = msg.result.CompareURL
			}
			if url != "" {
				m.message = fmt.Sprintf("Promoted! Branch: %s\nPR: %s", msg.result.Branch, url)
			} else {
				m.message = fmt.Sprintf("Promoted! Branch: %s (push manually to create PR)", msg.result.Branch)
			}
			m.messageIsErr = false
		}
		return m, nil
	case tea.KeyMsg:
		// Clear transient message on any keypress (consistent with other screens)
		if m.message != "" && msg.Type != tea.KeyEsc && !m.HasTextInput() {
			m.message = ""
			m.messageIsErr = false
		}

		// Tab switching (blocked during file viewing)
		if !m.fileViewer.viewing {
			switch msg.String() {
			case "tab":
				m.activeTab = (m.activeTab + 1) % 3
				m.scrollOffset = 0
				return m, nil
			case "shift+tab":
				m.activeTab = (m.activeTab + 2) % 3
				m.scrollOffset = 0
				return m, nil
			case "1":
				m.activeTab = tabOverview
				m.scrollOffset = 0
				return m, nil
			case "2":
				m.activeTab = tabFiles
				m.scrollOffset = 0
				return m, nil
			case "3":
				m.activeTab = tabInstall
				m.scrollOffset = 0
				return m, nil
			}
		}

		// File viewer: viewing file content
		if m.activeTab == tabFiles && m.fileViewer.viewing {
			switch {
			case key.Matches(msg, keys.Back):
				m.fileViewer.viewing = false
				m.fileViewer.content = ""
				m.fileViewer.scrollOffset = 0
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.fileViewer.scrollOffset > 0 {
					m.fileViewer.scrollOffset--
				}
				return m, nil
			case key.Matches(msg, keys.Down):
				m.fileViewer.scrollOffset++
				return m, nil
			case key.Matches(msg, keys.PageUp):
				pageSize := m.height - 8
				if pageSize < 1 {
					pageSize = 10
				}
				m.fileViewer.scrollOffset -= pageSize
				if m.fileViewer.scrollOffset < 0 {
					m.fileViewer.scrollOffset = 0
				}
				return m, nil
			case key.Matches(msg, keys.PageDown):
				pageSize := m.height - 8
				if pageSize < 1 {
					pageSize = 10
				}
				m.fileViewer.scrollOffset += pageSize
				return m, nil
			}
			return m, nil
		}

		// File viewer: navigating file list
		if m.activeTab == tabFiles && !m.fileViewer.viewing {
			switch {
			case key.Matches(msg, keys.Back):
				// Let the outer handler deal with Esc (navigate back)
			case key.Matches(msg, keys.Up):
				if m.fileViewer.cursor > 0 {
					m.fileViewer.cursor--
				}
				return m, nil
			case key.Matches(msg, keys.Down):
				if m.fileViewer.cursor < len(m.item.Files)-1 {
					m.fileViewer.cursor++
				}
				return m, nil
			case key.Matches(msg, keys.Enter):
				if m.fileViewer.cursor < len(m.item.Files) {
					relPath := m.item.Files[m.fileViewer.cursor]
					absPath := filepath.Join(m.item.Path, relPath)
					data, readErr := os.ReadFile(absPath)
					if readErr != nil {
						m.message = fmt.Sprintf("Cannot read file: %s", readErr)
						m.messageIsErr = true
					} else {
						m.fileViewer.content = string(data)
						m.fileViewer.scrollOffset = 0
						m.fileViewer.viewing = true
					}
				}
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, keys.Back):
			// Back key is handled by App (navigates to items list or cancels file viewer)

		case key.Matches(msg, keys.Up):
			// Overview tab and scrollable content: scroll
			if m.activeTab == tabOverview {
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			} else if m.activeTab == tabInstall {
				if m.item.Type == catalog.Prompts || m.item.Type == catalog.Apps {
					if m.scrollOffset > 0 {
						m.scrollOffset--
					}
				} else if len(m.provCheck.checks) > 0 && m.provCheck.cursor > 0 {
					m.provCheck.cursor--
				}
			}

		case key.Matches(msg, keys.Down):
			if m.activeTab == tabOverview {
				m.scrollOffset++
				m.clampScroll()
			} else if m.activeTab == tabInstall {
				if m.item.Type == catalog.Prompts || m.item.Type == catalog.Apps {
					m.scrollOffset++
					m.clampScroll()
				} else if len(m.provCheck.checks) > 0 && m.provCheck.cursor < len(m.provCheck.checks)-1 {
					m.provCheck.cursor++
				}
			}

		case key.Matches(msg, keys.PageUp):
			if m.activeTab == tabOverview || (m.activeTab == tabInstall && (m.item.Type == catalog.Prompts || m.item.Type == catalog.Apps)) {
				pageSize := m.height - 6
				if pageSize < 1 {
					pageSize = 10
				}
				m.scrollOffset -= pageSize
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			}

		case key.Matches(msg, keys.PageDown):
			if m.activeTab == tabOverview || (m.activeTab == tabInstall && (m.item.Type == catalog.Prompts || m.item.Type == catalog.Apps)) {
				pageSize := m.height - 6
				if pageSize < 1 {
					pageSize = 10
				}
				m.scrollOffset += pageSize
				m.clampScroll()
			}

		case key.Matches(msg, keys.Space):
			if m.activeTab == tabInstall && m.provCheck.cursor < len(m.provCheck.checks) {
				m.provCheck.checks[m.provCheck.cursor] = !m.provCheck.checks[m.provCheck.cursor]
			}

		case key.Matches(msg, keys.Install):
			if m.activeTab != tabInstall {
				break
			}
			if m.item.Type == catalog.Prompts {
				break
			}
			if m.item.Type == catalog.Apps {
				itemPath := m.item.Path
				return m, func() tea.Msg {
					scriptPreview := loadScriptPreview(itemPath)
					return openModalMsg{
						purpose: modalAppScript,
						title:   "Run install.sh?",
						body:    "WARNING: executes a shell script.\n\n" + scriptPreview,
					}
				}
			}
			// Guard: no providers detected
			if len(m.detectedProviders()) == 0 {
				m.message = "No providers detected for this content type"
				m.messageIsErr = true
				return m, nil
			}
			return m, m.startInstall()

		case key.Matches(msg, keys.Enter):
			if m.activeTab != tabInstall {
				break
			}
			// Enter toggles checkbox
			if m.provCheck.cursor < len(m.provCheck.checks) {
				m.provCheck.checks[m.provCheck.cursor] = !m.provCheck.checks[m.provCheck.cursor]
			}

		case key.Matches(msg, keys.Uninstall):
			if m.activeTab != tabInstall {
				break
			}
			if m.item.Type == catalog.Prompts {
				break
			}
			if m.item.Type == catalog.Apps {
				return m, m.runAppScript("--uninstall")
			}
			// Guard: nothing selected
			hasChecked := false
			for _, c := range m.provCheck.checks {
				if c {
					hasChecked = true
					break
				}
			}
			if !hasChecked {
				m.message = "No providers selected — use Space to select providers first"
				m.messageIsErr = true
				break
			}
			installed := m.installedProviders()
			if len(installed) == 0 {
				m.message = "Not installed in any provider"
				m.messageIsErr = true
			} else {
				var names []string
				for _, p := range installed {
					names = append(names, p.Name)
				}
				return m, func() tea.Msg {
					return openModalMsg{
						purpose: modalUninstall,
						title:   fmt.Sprintf("Uninstall %q?", m.item.Name),
						body:    fmt.Sprintf("Remove from: %s", strings.Join(names, ", ")),
					}
				}
			}

		case key.Matches(msg, keys.Copy):
			if m.item.Local && m.llmPrompt != "" {
				m.doCopyLLMPrompt()
			} else if m.item.Type == catalog.Prompts && m.item.Body != "" {
				m.doCopy()
			}

		case key.Matches(msg, keys.Save):
			if m.activeTab != tabInstall {
				break
			}
			if m.item.Type == catalog.Prompts && m.item.Body != "" {
				return m, func() tea.Msg { return openSaveModalMsg{} }
			}

		case key.Matches(msg, keys.EnvSetup):
			if m.activeTab != tabInstall {
				break
			}
			if m.item.Type == catalog.MCP {
				envTypes := m.unsetEnvVarNames()
				if len(envTypes) == 0 {
					m.message = "All environment variables are set"
					m.messageIsErr = false
					return m, nil
				}
				return m, func() tea.Msg { return openEnvModalMsg{envTypes: envTypes} }
			}

		case key.Matches(msg, keys.Promote):
			if m.item.Local {
				return m, func() tea.Msg {
					return openModalMsg{
						purpose: modalPromote,
						title:   fmt.Sprintf("Promote %q to shared?", m.item.Name),
						body:    "Creates a branch, commits, pushes, and opens a PR.",
					}
				}
			}
		}
	}
	return m, nil
}

// startInstall prepares for installation and returns a command.
// For filesystem types, opens the install modal for location/method selection.
// For JSON-merge-only types, installs directly without a modal.
func (m *detailModel) startInstall() tea.Cmd {
	detected := m.detectedProviders()
	if len(detected) == 0 {
		m.message = "No providers detected for this content type"
		m.messageIsErr = true
		return nil
	}

	// Auto-check the cursor provider if it's not already installed
	if m.provCheck.cursor < len(detected) && m.provCheck.cursor < len(m.provCheck.checks) {
		status := installer.CheckStatus(m.item, detected[m.provCheck.cursor], m.repoRoot)
		if status != installer.StatusInstalled {
			m.provCheck.checks[m.provCheck.cursor] = true
		}
	}

	// Guard: nothing selected
	anyChecked := false
	for _, c := range m.provCheck.checks {
		if c {
			anyChecked = true
			break
		}
	}
	if !anyChecked {
		m.message = "No providers selected — use Space to select providers first"
		m.messageIsErr = true
		return nil
	}

	// Find providers that are checked but not yet installed
	hasNewInstalls := false
	needsModal := false
	var checkedProviders []provider.Provider
	for i, checked := range m.provCheck.checks {
		if !checked || i >= len(detected) {
			continue
		}
		status := installer.CheckStatus(m.item, detected[i], m.repoRoot)
		if status != installer.StatusInstalled {
			hasNewInstalls = true
			checkedProviders = append(checkedProviders, detected[i])
			if !installer.IsJSONMerge(detected[i], m.item.Type) {
				needsModal = true
			}
		}
	}

	if !hasNewInstalls {
		m.message = "All checked providers already installed"
		m.messageIsErr = false
		return nil
	}

	m.message = ""
	if needsModal {
		item := m.item
		repoRoot := m.repoRoot
		return func() tea.Msg {
			return openInstallModalMsg{
				item:      item,
				providers: checkedProviders,
				repoRoot:  repoRoot,
			}
		}
	}
	// JSON-merge-only: install directly without modal
	return m.doInstallChecked()
}

// doInstallFromModal is called when the install modal is confirmed.
// It reads the location and method choices from the modal and performs the install.
// Returns a tea.Cmd to open the env setup modal if needed.
func (m *detailModel) doInstallFromModal(modal installModal) tea.Cmd {
	method := installer.MethodSymlink
	if modal.MethodCursor() == 1 {
		method = installer.MethodCopy
	}

	var baseDir string
	switch modal.LocationCursor() {
	case 0: // Global (home dir)
		baseDir = ""
	case 1: // Project (current working directory)
		if cwd, err := os.Getwd(); err == nil {
			baseDir = cwd
		}
	case 2: // Custom path
		baseDir = modal.CustomPath()
	}

	return m.doInstallWithOptions(method, baseDir)
}

// doInstallChecked installs the item to all checked providers using defaults.
// Used by startInstall() for JSON-merge-only providers that skip the modal.
func (m *detailModel) doInstallChecked() tea.Cmd {
	return m.doInstallWithOptions(installer.MethodSymlink, "")
}

// doInstallWithOptions installs the item to all checked providers with the given method and base dir.
// Returns a tea.Cmd to open the env setup modal if the item is an MCP server with unset env vars.
func (m *detailModel) doInstallWithOptions(method installer.InstallMethod, baseDir string) tea.Cmd {
	detected := m.detectedProviders()

	var successes, errs []string
	successIndices := map[int]bool{}
	for i, checked := range m.provCheck.checks {
		if !checked || i >= len(detected) {
			continue
		}
		p := detected[i]

		actualMethod := method
		if installer.IsJSONMerge(p, m.item.Type) {
			actualMethod = ""
		}

		path, err := installer.Install(m.item, p, m.repoRoot, actualMethod, baseDir)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", p.Name, err))
		} else {
			successes = append(successes, fmt.Sprintf("%s → %s", p.Name, path))
			successIndices[i] = true
		}
	}

	if len(errs) > 0 {
		m.message = ""
		if len(successes) > 0 {
			m.message = "Installed: " + strings.Join(successes, ", ") + "\n"
		}
		m.message += "Errors: " + strings.Join(errs, "; ")
		m.messageIsErr = true
	} else {
		m.message = "Installed to " + strings.Join(successes, ", ")
		m.messageIsErr = false
	}

	// Refresh checkbox state to reflect actual install status.
	// For providers that just succeeded, force the checkbox to true — CheckStatus
	// only resolves against the home directory, so project-local installs would
	// be incorrectly reported as not installed.
	detected = m.detectedProviders()
	for i, p := range detected {
		if i < len(m.provCheck.checks) {
			if successIndices[i] {
				m.provCheck.checks[i] = true
			} else {
				status := installer.CheckStatus(m.item, p, m.repoRoot)
				m.provCheck.checks[i] = status == installer.StatusInstalled
			}
		}
	}

	// After successful MCP install, offer env setup if there are unset vars
	if m.item.Type == catalog.MCP && m.mcpConfig != nil && len(errs) == 0 {
		unsetNames := m.unsetEnvVarNames()
		if len(unsetNames) > 0 {
			return func() tea.Msg {
				return openEnvModalMsg{envTypes: unsetNames}
			}
		}
	}

	return nil
}

// doUninstallAll uninstalls from all providers where the item is currently installed.
func (m *detailModel) doUninstallAll() {
	installed := m.installedProviders()
	var successes, errs []string

	for _, p := range installed {
		path, err := installer.Uninstall(m.item, p, m.repoRoot)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", p.Name, err))
		} else {
			successes = append(successes, fmt.Sprintf("%s → removed %s", p.Name, path))
		}
	}

	if len(errs) > 0 {
		m.message = ""
		if len(successes) > 0 {
			m.message = strings.Join(successes, "; ") + "\n"
		}
		m.message += "Errors: " + strings.Join(errs, "; ")
		m.messageIsErr = true
	} else {
		m.message = "Uninstalled: " + strings.Join(successes, "; ")
		m.messageIsErr = false
	}

	// Uncheck all provider checkboxes after uninstall
	for i := range m.provCheck.checks {
		m.provCheck.checks[i] = false
	}
}

// runAppScript runs the app's install.sh script via tea.ExecProcess.
// Pass no args for install, or "--uninstall" for uninstall.
func (m *detailModel) runAppScript(args ...string) tea.Cmd {
	scriptPath := filepath.Join(m.item.Path, "install.sh")
	if _, err := os.Stat(scriptPath); errors.Is(err, fs.ErrNotExist) {
		m.message = "No install.sh found for this app"
		m.messageIsErr = true
		return nil
	}
	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	action := "Install"
	if len(args) > 0 && args[0] == "--uninstall" {
		action = "Uninstall"
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return appInstallDoneMsg{err: err, action: action}
	})
}

// loadScriptPreview reads the first 10 lines of install.sh from itemPath.
// Returns a placeholder string if the file cannot be read.
func loadScriptPreview(itemPath string) string {
	data, err := os.ReadFile(filepath.Join(itemPath, "install.sh"))
	if err != nil {
		return "(script not found)"
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 10 {
		lines = lines[:10]
	}
	return strings.Join(lines, "\n")
}

func (m *detailModel) doCopy() {
	if err := clipboard.WriteAll(m.item.Body); err != nil {
		m.message = fmt.Sprintf("Copy failed: %s", err)
		m.messageIsErr = true
	} else {
		m.message = "Prompt copied to clipboard"
		m.messageIsErr = false
	}
}

func (m *detailModel) doCopyLLMPrompt() {
	if err := clipboard.WriteAll(m.llmPrompt); err != nil {
		m.message = fmt.Sprintf("Copy failed: %s", err)
		m.messageIsErr = true
	} else {
		m.message = "LLM prompt copied to clipboard"
		m.messageIsErr = false
	}
}

func (m *detailModel) doSave() {
	target, err := expandHome(m.savePath)
	if err != nil {
		m.message = err.Error()
		m.messageIsErr = true
		return
	}

	sourcePath := filepath.Join(m.item.Path, "PROMPT.md")

	switch m.methodCursor {
	case 0: // symlink
		if err := installer.CreateSymlink(sourcePath, target); err != nil {
			m.message = fmt.Sprintf("Save failed: %s", err)
			m.messageIsErr = true
		} else {
			m.message = fmt.Sprintf("Saved (symlink) to %s", target)
			m.messageIsErr = false
		}
	case 1: // copy
		if err := installer.CopyContent(sourcePath, target); err != nil {
			m.message = fmt.Sprintf("Save failed: %s", err)
			m.messageIsErr = true
		} else {
			m.message = fmt.Sprintf("Saved (copy) to %s", target)
			m.messageIsErr = false
		}
	}
}

// doSavePrompt sets the save path from the modal input value and triggers save.
// It replaces the inline actionSavePath/actionSaveMethod flow.
func (m *detailModel) doSavePrompt(filename string) {
	m.savePath = filename
	// Default to symlink (methodCursor 0); user chose via modal, not method picker
	m.methodCursor = 0
	m.doSave()
}

// HasPendingAction returns true if the detail view has an active file viewer
// that should consume the Back key instead of navigating away.
func (m detailModel) HasPendingAction() bool {
	return m.fileViewer.viewing
}

// HasTextInput returns true if the detail view has an active text input
// that should capture keyboard input. All text inputs are now handled
// at the modal level.
func (m detailModel) HasTextInput() bool {
	return false
}

// CancelAction clears the file viewer state.
func (m *detailModel) CancelAction() {
	if m.fileViewer.viewing {
		m.fileViewer.viewing = false
		m.fileViewer.content = ""
		m.fileViewer.scrollOffset = 0
	}
}

// supportedProviders returns all providers that support this item's content type.
func (m detailModel) supportedProviders() []provider.Provider {
	var supported []provider.Provider
	for _, p := range m.providers {
		if p.SupportsType(m.item.Type) {
			supported = append(supported, p)
		}
	}
	return supported
}

// detectedProviders returns detected providers that support this item's content type.
func (m detailModel) detectedProviders() []provider.Provider {
	var detected []provider.Provider
	for _, p := range m.providers {
		if p.Detected && p.SupportsType(m.item.Type) {
			detected = append(detected, p)
		}
	}
	return detected
}

// installedProviders returns detected providers where this item is currently installed.
func (m detailModel) installedProviders() []provider.Provider {
	var installed []provider.Provider
	for _, p := range m.providers {
		if p.Detected && p.SupportsType(m.item.Type) {
			status := installer.CheckStatus(m.item, p, m.repoRoot)
			if status == installer.StatusInstalled {
				installed = append(installed, p)
			}
		}
	}
	return installed
}

// clampScroll ensures scrollOffset stays within valid bounds.
func (m *detailModel) clampScroll() {
	pinned, body := m.renderContentSplit()
	pinnedLines := strings.Split(pinned, "\n")
	bodyLines := strings.Split(body, "\n")

	visibleHeight := m.height - len(pinnedLines) - 2
	if visibleHeight < 1 {
		visibleHeight = len(bodyLines)
	}

	maxOffset := len(bodyLines) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}
