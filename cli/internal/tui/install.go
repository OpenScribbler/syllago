package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- Step enum ---

type installStep int

const (
	installStepProvider installStep = iota
	installStepLocation
	installStepMethod
	installStepReview
)

// stepHints returns helpbar hints for the current install wizard step.
func (m *installWizardModel) stepHints() []string {
	base := []string{"? help"}
	switch m.step {
	case installStepProvider:
		return append([]string{"↑/↓ select", "enter next", "esc close wizard"}, base...)
	case installStepLocation:
		return append([]string{"↑/↓ select", "enter next", "esc back"}, base...)
	case installStepMethod:
		return append([]string{"↑/↓ select", "enter next", "esc back"}, base...)
	case installStepReview:
		return append([]string{"tab cycle zones", "↑/↓ navigate", "enter confirm", "esc back"}, base...)
	}
	return base
}

// reviewFocusZone tracks which zone is focused on the review step.
// Tab cycles through: risks -> tree -> preview -> buttons -> risks.
type reviewFocusZone int

const (
	reviewZoneRisks   reviewFocusZone = iota // risk indicator list
	reviewZoneTree                           // file tree (skipped for single-file items)
	reviewZonePreview                        // file preview (scroll only)
	reviewZoneButtons                        // Cancel / Back / Install
)

// --- Messages ---

// installResultMsg is emitted when the user confirms the install.
type installResultMsg struct {
	item        catalog.ContentItem
	provider    provider.Provider
	location    string // "global", "project", or custom path
	method      installer.InstallMethod
	isJSONMerge bool
	projectRoot string
}

// installDoneMsg is sent when the async install operation completes.
type installDoneMsg struct {
	itemName     string
	providerName string
	targetPath   string
	err          error
}

// installCloseMsg signals the wizard should close.
type installCloseMsg struct{}

// --- Model ---

type installWizardModel struct {
	shell  wizardShell
	step   installStep
	width  int
	height int

	// Item context
	item     catalog.ContentItem
	itemName string

	// Provider step
	providers         []provider.Provider
	providerInstalled []bool
	providerCursor    int

	// Location step
	locationCursor int
	customPath     string
	customCursor   int

	// Method step
	methodCursor int

	// Review step — risk indicators + file browser
	risks      []catalog.RiskIndicator
	riskBanner riskBanner

	// Review step — focus zone system
	reviewZone   reviewFocusZone
	buttonCursor int // 0=Cancel, 1=Back, 2=Install (-1 = no button focused)

	// Review step — file browser
	reviewTree    fileTreeModel
	reviewPreview previewModel

	// Double-confirm prevention
	confirmed bool

	// Computed on open
	isJSONMerge         bool
	autoSkippedProvider bool

	// Context
	projectRoot string
}

// openInstallWizard creates a new install wizard for the given item.
//
// Why pointer return: the wizard is stored as *installWizardModel on App, so nil
// means "no wizard active" and View() handles the nil case gracefully. Pointer
// receivers also avoid copying the model on every Update call.
func openInstallWizard(item catalog.ContentItem, providers []provider.Provider, projectRoot string) *installWizardModel {
	// Compute display name — prefer DisplayName, fall back to Name.
	itemName := item.DisplayName
	if itemName == "" {
		itemName = item.Name
	}

	// Determine if this type uses JSON merge (hooks, MCP) vs filesystem (rules, skills, etc.).
	// All providers agree on merge vs filesystem for a given type, so checking the first is sufficient.
	isJSONMerge := len(providers) > 0 && installer.IsJSONMerge(providers[0], item.Type)

	// Compute per-provider install status up front so the provider step can show
	// "already installed" indicators without re-checking on every render.
	providerInstalled := make([]bool, len(providers))
	for i, prov := range providers {
		providerInstalled[i] = installer.CheckStatus(item, prov, projectRoot) == installer.StatusInstalled
	}

	// Step labels depend on content type: JSON merge skips location+method.
	var stepLabels []string
	if isJSONMerge {
		stepLabels = []string{"Provider", "Review"}
	} else {
		stepLabels = []string{"Provider", "Location", "Method", "Review"}
	}

	shell := newWizardShell("Install", stepLabels)

	m := &installWizardModel{
		shell:             shell,
		step:              installStepProvider,
		item:              item,
		itemName:          itemName,
		providers:         providers,
		providerInstalled: providerInstalled,
		isJSONMerge:       isJSONMerge,
		projectRoot:       projectRoot,
		buttonCursor:      -1, // no button focused initially
	}

	// Single-provider auto-skip: jump past the provider step when there's only
	// one choice and it's not already installed.
	if len(providers) == 1 && !providerInstalled[0] {
		m.providerCursor = 0
		m.autoSkippedProvider = true
		if isJSONMerge {
			// JSON merge: provider -> review (steps 0 -> 1 in the 2-step shell)
			m.enterReview(1)
		} else {
			// Filesystem: provider -> location
			m.step = installStepLocation
			m.shell.SetActive(1)
		}
	}
	// Single provider AND already installed: stay on provider step so the user
	// sees the "already installed" state and can only Esc out.

	return m
}

// validateStep checks entry-prerequisites for the current step. These are
// programmer errors (invariant violations), not user-facing conditions.
// Called at the top of Update() to catch state machine bugs early.
func (m *installWizardModel) validateStep() {
	switch m.step {
	case installStepProvider:
		if m.item.Path == "" {
			panic("wizard invariant: installStepProvider entered with empty item")
		}
	case installStepLocation:
		if m.providerCursor < 0 || m.providerCursor >= len(m.providers) {
			panic("wizard invariant: installStepLocation entered without valid provider")
		}
		if m.providerInstalled[m.providerCursor] {
			panic("wizard invariant: installStepLocation entered with already-installed provider")
		}
	case installStepMethod:
		if m.isJSONMerge {
			panic("wizard invariant: installStepMethod entered for JSON merge type")
		}
		if m.locationCursor < 0 || m.locationCursor > 2 {
			panic(fmt.Sprintf("wizard invariant: installStepMethod entered with invalid location cursor %d", m.locationCursor))
		}
	case installStepReview:
		if m.providerCursor < 0 || m.providerCursor >= len(m.providers) {
			panic("wizard invariant: installStepReview entered without provider")
		}
		if !m.isJSONMerge && m.locationCursor < 0 {
			panic("wizard invariant: installStepReview entered without location")
		}
	}
}

// Init satisfies the tea.Model interface.
func (m *installWizardModel) Init() tea.Cmd {
	return nil
}

// nextSelectableProvider finds the next provider index that isn't already installed.
// direction: +1 for forward, -1 for backward. Wraps around.
// Returns -1 if no selectable provider exists.
func (m *installWizardModel) nextSelectableProvider(from, direction int) int {
	n := len(m.providers)
	if n == 0 {
		return -1
	}
	for i := 0; i < n; i++ {
		idx := ((from+direction*(i+1))%n + n) % n
		if !m.providerInstalled[idx] {
			return idx
		}
	}
	return -1
}

// navigateToStep jumps to a previously completed step, preserving wizard state.
// Only safe to call for steps < m.step (going backwards).
func (m *installWizardModel) navigateToStep(target installStep) {
	// Map install step to shell step index. JSON merge wizards have fewer shell steps.
	shellIdx := int(target)
	if m.isJSONMerge && target == installStepReview {
		shellIdx = 1
	}
	m.step = target
	m.shell.SetActive(shellIdx)
	// Reset review state when navigating away from review
	m.confirmed = false
}

// resolvedInstallPath returns the display path for a location option.
func (m *installWizardModel) resolvedInstallPath(loc int) string {
	prov := m.providers[m.providerCursor]
	switch loc {
	case 0: // Global
		home, err := os.UserHomeDir()
		if err != nil {
			return prov.InstallDir(home, m.item.Type)
		}
		dir := prov.InstallDir(home, m.item.Type)
		if strings.HasPrefix(dir, home) {
			return "~" + dir[len(home):]
		}
		return dir
	case 1: // Project
		dir := prov.InstallDir(m.projectRoot, m.item.Type)
		if strings.HasPrefix(dir, m.projectRoot) {
			return "." + dir[len(m.projectRoot):]
		}
		return dir
	case 2: // Custom
		return m.customPath
	}
	return ""
}

// resolveSettingsPath returns the provider's settings file path for JSON merge types.
// For hooks/MCP, content merges into the provider's settings.json (or equivalent).
func (m *installWizardModel) resolveSettingsPath(prov provider.Provider) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/" + prov.ConfigDir + "/settings.json"
	}
	path := filepath.Join(home, prov.ConfigDir, "settings.json")
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// symlinkDisabled returns true if the selected provider does not support symlinks
// for this item's content type. When true, the Symlink option is grayed out and
// the cursor defaults to Copy.
func (m *installWizardModel) symlinkDisabled() bool {
	prov := m.providers[m.providerCursor]
	if supported, ok := prov.SymlinkSupport[m.item.Type]; ok && !supported {
		return true
	}
	return false
}

// defaultMethodCursor returns the initial cursor position for the method step.
// Returns 1 (Copy) when symlinks are disabled, 0 (Symlink) otherwise.
func (m *installWizardModel) defaultMethodCursor() int {
	if m.symlinkDisabled() {
		return 1 // Copy
	}
	return 0 // Symlink
}

// enterReview transitions to the review step, computing risks and initializing
// the file browser and risk banner. shellIdx is the wizard shell step index
// for Review (varies by content type: 3 for filesystem, 1 for JSON merge).
func (m *installWizardModel) enterReview(shellIdx int) {
	m.step = installStepReview
	m.shell.SetActive(shellIdx)

	// Compute risks
	m.risks = catalog.RiskIndicators(m.item)
	m.riskBanner = newRiskBanner(m.risks, m.width-4)

	// Create file tree from the item's files
	m.reviewTree = newFileTreeModel(m.item.Files)
	m.reviewTree.focused = false

	// Load primary file into preview
	m.reviewPreview = newPreviewModel()
	m.reviewPreview.LoadItem(&m.item)

	// Default focus zone: risks if present, otherwise tree (or preview for single file)
	if len(m.risks) > 0 {
		m.reviewZone = reviewZoneRisks
		// Auto-scroll to the first risk's highlighted line
		m.syncPreviewToRisk()
	} else if m.hasMultipleFiles() {
		m.reviewZone = reviewZoneTree
		m.reviewTree.focused = true
	} else {
		m.reviewZone = reviewZonePreview
		m.reviewPreview.focused = true
	}
	m.buttonCursor = -1
	m.confirmed = false
}

// hasMultipleFiles returns true if the item has more than one file.
func (m *installWizardModel) hasMultipleFiles() bool {
	return len(m.item.Files) > 1
}

// syncPreviewToRisk loads the file and scrolls to the highlighted lines
// for the currently selected risk indicator.
func (m *installWizardModel) syncPreviewToRisk() {
	if m.riskBanner.cursor < 0 || m.riskBanner.cursor >= len(m.risks) {
		return
	}
	risk := m.risks[m.riskBanner.cursor]
	if len(risk.Lines) == 0 {
		// No specific lines — keep current preview
		m.reviewPreview.SetHighlightLines(nil)
		return
	}

	rl := risk.Lines[0]

	// Load file if different from current
	if m.reviewPreview.fileName != rl.File {
		content, err := catalog.ReadFileContent(m.item.Path, rl.File, 500)
		if err == nil {
			m.reviewPreview.lines = strings.Split(content, "\n")
			m.reviewPreview.fileName = rl.File
		}
		// Update tree cursor to match the file
		m.reviewTree.SelectPath(rl.File)
	}

	// Set highlight lines (all lines from this risk in the same file)
	highlights := make(map[int]bool)
	for _, l := range risk.Lines {
		if l.File == rl.File {
			highlights[l.Line] = true
		}
	}
	m.reviewPreview.SetHighlightLines(highlights)

	// Scroll to center on the first highlighted line
	if rl.Line > 0 {
		m.reviewPreview.offset = max(0, rl.Line-3)
	}
}

// installResult builds the installResultMsg from the current wizard state.
func (m *installWizardModel) installResult() installResultMsg {
	prov := m.providers[m.providerCursor]

	var location string
	switch m.locationCursor {
	case 0:
		location = "global"
	case 1:
		location = "project"
	case 2:
		location = m.customPath
	}

	var method installer.InstallMethod
	if m.methodCursor == 0 {
		method = installer.MethodSymlink
	} else {
		method = installer.MethodCopy
	}

	return installResultMsg{
		item:        m.item,
		provider:    prov,
		location:    location,
		method:      method,
		isJSONMerge: m.isJSONMerge,
		projectRoot: m.projectRoot,
	}
}

// reviewZoneOrder returns the ordered list of active zones for Tab cycling.
func (m *installWizardModel) reviewZoneOrder() []reviewFocusZone {
	var zones []reviewFocusZone
	if len(m.risks) > 0 {
		zones = append(zones, reviewZoneRisks)
	}
	if m.hasMultipleFiles() {
		zones = append(zones, reviewZoneTree)
	}
	zones = append(zones, reviewZonePreview)
	zones = append(zones, reviewZoneButtons)
	return zones
}

// setReviewZone switches focus to the given zone, updating sub-model focus state.
func (m *installWizardModel) setReviewZone(z reviewFocusZone) {
	m.reviewZone = z
	m.reviewTree.focused = z == reviewZoneTree
	m.reviewPreview.focused = z == reviewZonePreview
	if z == reviewZoneButtons && m.buttonCursor < 0 {
		m.buttonCursor = 1 // Default to Back (safe, not Install)
	}
}

// loadReviewTreeFile loads the file at the tree cursor into the review preview.
func (m *installWizardModel) loadReviewTreeFile() {
	path := m.reviewTree.SelectedPath()
	if path == "" {
		return
	}
	content, err := catalog.ReadFileContent(m.item.Path, path, 500)
	if err == nil {
		m.reviewPreview.lines = strings.Split(content, "\n")
		m.reviewPreview.fileName = path
		m.reviewPreview.offset = 0
		m.reviewPreview.SetHighlightLines(nil) // Clear risk highlights when browsing files manually
	}
}
