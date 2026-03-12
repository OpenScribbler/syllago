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

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/promote"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

type appInstallDoneMsg struct {
	err    error
	action string // "install" or "uninstall"
}

type shareDoneMsg struct {
	result *promote.Result
	err    error
}

// openSaveModalMsg is sent by detailModel to ask App to open the save modal.
type openSaveModalMsg struct{}

// openEnvModalMsg is sent by detailModel to ask App to open the env setup wizard modal.
type openEnvModalMsg struct {
	envTypes []string
}

// openHookBrokenWarningMsg is sent by detailModel when installing a hook with CompatBroken.
type openHookBrokenWarningMsg struct {
	providerName string
	notes        string
}

// openLoadoutApplyMsg is sent by detailModel to ask App to run a loadout apply.
type openLoadoutApplyMsg struct {
	item catalog.ContentItem
	mode string // "preview", "try", or "keep"
}

// loadoutApplyDoneMsg carries the result of a loadout apply operation.
type loadoutApplyDoneMsg struct {
	result *loadout.ApplyResult
	err    error
	mode   string
}

// detailTab represents the active tab on the detail screen.
type detailTab int

const (
	tabFiles         detailTab = iota
	tabCompatibility           // hooks only
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
	appScriptPreview string // first N lines of install.sh for preview
	llmPrompt        string // loaded from LLM-PROMPT.md for local scaffolded items
	// Override info
	overrides []catalog.ContentItem // lower-precedence items this one shadows
	// Tab state
	activeTab    detailTab
	fileViewer   fileViewerModel // file viewer state (Files tab)
	parentLabel  string          // intermediate breadcrumb (e.g. "Library", "Loadouts")
	listPosition int             // 0-based position in the items list (for breadcrumb)
	listTotal    int             // total items in the list
	width        int
	height       int
	// Loadout-specific state
	loadoutManifest    *loadout.Manifest // parsed manifest (for Loadouts type)
	loadoutManifestErr string            // error from parsing manifest
	loadoutModeCursor  int               // 0=preview, 1=try, 2=keep (Apply tab)
	// Hook-specific state
	hookData   *converter.HookData      // loaded for hook items (nil for all others)
	hookCompat []converter.CompatResult // computed for all 4 providers
}

func newDetailModel(item catalog.ContentItem, providers []provider.Provider, repoRoot string) detailModel {
	ti := textinput.New()
	ti.Prompt = labelStyle.Render("Save to: ")
	ti.CharLimit = 200

	m := detailModel{
		item:       item,
		providers:  providers,
		repoRoot:   repoRoot,
		saveInput:  ti,
		fileViewer: newFileViewer(item),
	}
	// Parse MCP config for preview
	if item.Type == catalog.MCP {
		cfg, _ := installer.ParseMCPConfig(item.Path)
		m.mcpConfig = cfg
	}
	// Load LLM prompt for library items
	if item.Library {
		llmPath := filepath.Join(item.Path, "LLM-PROMPT.md")
		if data, err := os.ReadFile(llmPath); err == nil {
			m.llmPrompt = string(data)
		}
	}
	// Parse loadout manifest for loadout items
	if item.Type == catalog.Loadouts {
		manifest, err := loadout.Parse(filepath.Join(item.Path, "loadout.yaml"))
		if err != nil {
			m.loadoutManifestErr = err.Error()
		} else {
			m.loadoutManifest = manifest
		}
	}
	// Load hook data and compute compatibility for hook items
	if item.Type == catalog.Hooks {
		hd, err := converter.LoadHookData(item)
		if err == nil {
			m.hookData = &hd
			for _, slug := range converter.HookProviders() {
				m.hookCompat = append(m.hookCompat, converter.AnalyzeHookCompat(hd, slug))
			}
		}
	}
	// Initialize provider checkboxes
	// Pre-check any providers where the item is already installed
	{
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
	case shareDoneMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Share failed: %s", msg.err)
			m.messageIsErr = true
		} else {
			url := msg.result.PRUrl
			if url == "" {
				url = msg.result.CompareURL
			}
			if url != "" {
				m.message = fmt.Sprintf("Shared! Branch: %s\nPR: %s", msg.result.Branch, url)
			} else {
				m.message = fmt.Sprintf("Shared! Branch: %s (push manually to create PR)", msg.result.Branch)
			}
			m.messageIsErr = false
		}
		return m, nil
	case splitViewCursorMsg:
		// Load preview content when the split view cursor moves
		m.fileViewer.loadPreview(msg.index)
		return m, nil

	case tea.MouseMsg:
		// Delegate mouse events to split view when on Files tab
		if m.activeTab == tabFiles && m.item.Type != catalog.Loadouts {
			sv, cmd := m.fileViewer.splitView.Update(msg)
			m.fileViewer.splitView = sv
			return m, cmd
		}

	case tea.KeyMsg:
		// Clear transient message on any keypress (consistent with other screens)
		if m.message != "" && msg.Type != tea.KeyEsc && !m.HasTextInput() {
			m.message = ""
			m.messageIsErr = false
		}

		// Tab switching (blocked during single-pane file preview)
		if !m.fileViewer.splitView.showingPreview {
			var newTab detailTab = -1
			// Number keys always switch tabs regardless of pane focus.
			switch msg.String() {
			case "1":
				newTab = tabFiles
			case "2":
				if m.item.Type == catalog.Hooks {
					newTab = tabCompatibility
				} else {
					newTab = tabInstall
				}
			case "3":
				if m.item.Type == catalog.Hooks {
					newTab = tabInstall
				}
			}
			if newTab >= 0 {
				if m.activeTab == tabFiles && newTab != tabFiles {
					m.CancelAction()
				}
				m.activeTab = newTab
				m.scrollOffset = 0
				m.fileViewer.splitView.focusedPane = paneList
				return m, nil
			}
		}

		// Delegate navigation keys to split view when on Files tab (for non-loadout types).
		// Non-navigation keys (p, r, i, u, c, e, etc.) fall through to detail handlers below.
		if m.activeTab == tabFiles && m.item.Type != catalog.Loadouts {
			isSplitViewKey := key.Matches(msg, keys.Up, keys.Down, keys.Left, keys.Right,
				keys.Enter, keys.PageUp, keys.PageDown, keys.Home, keys.End) ||
				msg.String() == "l"
			// In single-pane preview, Esc exits the preview
			if m.fileViewer.splitView.showingPreview && key.Matches(msg, keys.Back) {
				isSplitViewKey = true
			}
			if isSplitViewKey {
				sv, cmd := m.fileViewer.splitView.Update(msg)
				m.fileViewer.splitView = sv
				return m, cmd
			}
		}

		switch {
		case key.Matches(msg, keys.Back):
			// Back key is handled by App (navigates to items list or cancels file viewer)

		case key.Matches(msg, keys.Up):
			if m.activeTab == tabFiles && m.item.Type == catalog.Loadouts {
				// Contents tab for loadouts: scroll
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			} else if m.activeTab == tabInstall {
				if m.item.Type == catalog.Loadouts {
					if m.loadoutModeCursor > 0 {
						m.loadoutModeCursor--
					}
				} else if len(m.provCheck.checks) > 0 && m.provCheck.cursor > 0 {
					m.provCheck.cursor--
					// Skip CompatNone providers when navigating up (hooks only)
					if m.item.Type == catalog.Hooks {
						detected := m.detectedProviders()
						for m.provCheck.cursor > 0 {
							slug := detected[m.provCheck.cursor].Slug
							if cr := m.hookCompatForProvider(slug); cr != nil && cr.Level == converter.CompatNone {
								m.provCheck.cursor--
							} else {
								break
							}
						}
						// If stuck on CompatNone at index 0, skip forward to first selectable
						if m.provCheck.cursor < len(detected) {
							slug := detected[m.provCheck.cursor].Slug
							if cr := m.hookCompatForProvider(slug); cr != nil && cr.Level == converter.CompatNone {
								for m.provCheck.cursor < len(detected)-1 {
									m.provCheck.cursor++
									slug = detected[m.provCheck.cursor].Slug
									if cr2 := m.hookCompatForProvider(slug); cr2 == nil || cr2.Level != converter.CompatNone {
										break
									}
								}
							}
						}
					}
				}
			}

		case key.Matches(msg, keys.Down):
			if m.activeTab == tabFiles && m.item.Type == catalog.Loadouts {
				// Contents tab for loadouts: scroll
				m.scrollOffset++
				m.clampScroll()
			} else if m.activeTab == tabInstall {
				if m.item.Type == catalog.Loadouts {
					if m.loadoutModeCursor < 2 {
						m.loadoutModeCursor++
					}
				} else if len(m.provCheck.checks) > 0 && m.provCheck.cursor < len(m.provCheck.checks)-1 {
					m.provCheck.cursor++
					// Skip CompatNone providers when navigating down (hooks only)
					if m.item.Type == catalog.Hooks {
						detected := m.detectedProviders()
						for m.provCheck.cursor < len(detected)-1 {
							slug := detected[m.provCheck.cursor].Slug
							if cr := m.hookCompatForProvider(slug); cr != nil && cr.Level == converter.CompatNone {
								m.provCheck.cursor++
							} else {
								break
							}
						}
						// If stuck on CompatNone at the last index, skip back to last selectable
						if m.provCheck.cursor < len(detected) {
							slug := detected[m.provCheck.cursor].Slug
							if cr := m.hookCompatForProvider(slug); cr != nil && cr.Level == converter.CompatNone {
								for m.provCheck.cursor > 0 {
									m.provCheck.cursor--
									slug = detected[m.provCheck.cursor].Slug
									if cr2 := m.hookCompatForProvider(slug); cr2 == nil || cr2.Level != converter.CompatNone {
										break
									}
								}
							}
						}
					}
				}
			}

		case key.Matches(msg, keys.PageUp):
			scrollable := m.activeTab == tabFiles && m.item.Type == catalog.Loadouts
			if scrollable {
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
			scrollable := m.activeTab == tabFiles && m.item.Type == catalog.Loadouts
			if scrollable {
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
			if m.item.Type == catalog.Loadouts {
				break
			}
			// Guard: no providers detected
			if len(m.detectedProviders()) == 0 {
				m.message = "No providers detected for this content type"
				m.messageIsErr = true
				return m, nil
			}
			// For hooks: warn before installing to a CompatBroken provider
			if m.item.Type == catalog.Hooks {
				detected := m.detectedProviders()
				if m.provCheck.cursor < len(detected) {
					selectedProv := detected[m.provCheck.cursor]
					if cr := m.hookCompatForProvider(selectedProv.Slug); cr != nil && cr.Level == converter.CompatBroken {
						provName := selectedProv.Name
						notes := cr.Notes
						return m, func() tea.Msg {
							return openHookBrokenWarningMsg{providerName: provName, notes: notes}
						}
					}
				}
			}
			return m, m.startInstall()

		case key.Matches(msg, keys.Enter):
			if m.activeTab != tabInstall {
				break
			}
			// Loadout Apply tab: trigger apply with selected mode
			if m.item.Type == catalog.Loadouts {
				if m.loadoutManifest == nil {
					m.message = "Cannot apply: loadout manifest not loaded"
					m.messageIsErr = true
					return m, nil
				}
				modes := []string{"preview", "try", "keep"}
				mode := modes[m.loadoutModeCursor]
				item := m.item
				return m, func() tea.Msg {
					return openLoadoutApplyMsg{item: item, mode: mode}
				}
			}
			// Enter toggles checkbox
			if m.provCheck.cursor < len(m.provCheck.checks) {
				m.provCheck.checks[m.provCheck.cursor] = !m.provCheck.checks[m.provCheck.cursor]
			}

		case key.Matches(msg, keys.Uninstall):
			if m.activeTab != tabInstall {
				break
			}
			if m.item.Type == catalog.Loadouts {
				break
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
			if m.item.Library && m.llmPrompt != "" {
				m.doCopyLLMPrompt()
			}

		case key.Matches(msg, keys.Save):
			break

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

		case key.Matches(msg, keys.Share):
			if m.item.Library {
				return m, func() tea.Msg {
					return openModalMsg{
						purpose: modalShare,
						title:   fmt.Sprintf("Share %q to team repo?", m.item.Name),
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

// HasPendingAction returns true if the detail view has an active single-pane
// preview that should consume the Back key instead of navigating away.
func (m detailModel) HasPendingAction() bool {
	return m.fileViewer.splitView.showingPreview
}

// HasTextInput returns true if the detail view has an active text input
// that should capture keyboard input. All text inputs are now handled
// at the modal level.
func (m detailModel) HasTextInput() bool {
	return false
}

// CancelAction resets the file viewer to its initial state.
func (m *detailModel) CancelAction() {
	m.fileViewer.splitView.showingPreview = false
	m.fileViewer.splitView.focusedPane = paneList
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

// hookCompatForProvider returns the CompatResult for the given provider slug,
// or nil if m.hookCompat is not loaded or the slug is not found.
func (m detailModel) hookCompatForProvider(slug string) *converter.CompatResult {
	if m.hookCompat == nil {
		return nil
	}
	providers := converter.HookProviders()
	for i, p := range providers {
		if p == slug && i < len(m.hookCompat) {
			return &m.hookCompat[i]
		}
	}
	return nil
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
