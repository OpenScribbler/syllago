package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

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

	// Review step
	risks        []catalog.RiskIndicator
	riskBanner   riskBanner
	riskCursor   int
	riskDrillIn  bool
	drillPreview previewModel

	// Focus
	focusIdx int

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
		riskCursor:        -1, // no risk item focused initially
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

// Update handles messages for the install wizard.
func (m *installWizardModel) Update(msg tea.Msg) (*installWizardModel, tea.Cmd) {
	m.validateStep()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	}
	return m, nil
}

func (m *installWizardModel) updateKey(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch m.step {
	case installStepProvider:
		return m.updateKeyProvider(msg)
	case installStepLocation:
		return m.updateKeyLocation(msg)
	case installStepMethod:
		return m.updateKeyMethod(msg)
	case installStepReview:
		return m.updateKeyReview(msg)
	}
	// Fallback Esc
	if msg.Type == tea.KeyEsc {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	return m, nil
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

func (m *installWizardModel) updateKeyProvider(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m, func() tea.Msg { return installCloseMsg{} }

	case msg.Type == tea.KeyDown || msg.Type == tea.KeyTab ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if next := m.nextSelectableProvider(m.providerCursor, +1); next >= 0 {
			m.providerCursor = next
		}

	case msg.Type == tea.KeyUp || msg.Type == tea.KeyShiftTab ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if next := m.nextSelectableProvider(m.providerCursor, -1); next >= 0 {
			m.providerCursor = next
		}

	case msg.Type == tea.KeyEnter:
		// Only advance if current provider is not installed and there's at least one selectable
		if m.providerCursor >= 0 && m.providerCursor < len(m.providers) &&
			!m.providerInstalled[m.providerCursor] {
			if m.isJSONMerge {
				// JSON merge skips location+method, goes straight to review
				m.enterReview(1) // Shell step index: Provider=0, Review=1 (2-step shell)
			} else {
				m.step = installStepLocation
				m.shell.SetActive(1)
			}
		}
	}
	return m, nil
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
	m.riskDrillIn = false
}

func (m *installWizardModel) updateMouse(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Check wizard shell breadcrumb clicks (completed steps are clickable)
	if step, ok := m.shell.HandleClick(msg); ok {
		// Map shell step index back to install step.
		target := installStep(step)
		if m.isJSONMerge && step == 1 {
			target = installStepReview
		}
		if target < m.step {
			m.navigateToStep(target)
		}
		return m, nil
	}

	switch m.step {
	case installStepProvider:
		return m.updateMouseProvider(msg)
	case installStepLocation:
		return m.updateMouseLocation(msg)
	case installStepMethod:
		return m.updateMouseMethod(msg)
	case installStepReview:
		return m.updateMouseReview(msg)
	}
	return m, nil
}

func (m *installWizardModel) updateMouseProvider(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	// Provider row clicks
	for i := range m.providers {
		if zone.Get(fmt.Sprintf("inst-prov-%d", i)).InBounds(msg) {
			if !m.providerInstalled[i] {
				m.providerCursor = i
			}
			return m, nil
		}
	}
	// Button clicks
	if zone.Get("inst-cancel").InBounds(msg) {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	if zone.Get("inst-next").InBounds(msg) {
		// Same as Enter
		if m.providerCursor >= 0 && m.providerCursor < len(m.providers) &&
			!m.providerInstalled[m.providerCursor] {
			if m.isJSONMerge {
				m.enterReview(1)
			} else {
				m.step = installStepLocation
				m.shell.SetActive(1)
			}
		}
	}
	return m, nil
}

// --- Location step ---

func (m *installWizardModel) locationGoBack() (*installWizardModel, tea.Cmd) {
	if m.autoSkippedProvider {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	m.step = installStepProvider
	m.shell.SetActive(0)
	return m, nil
}

func (m *installWizardModel) locationAdvance() (*installWizardModel, tea.Cmd) {
	// Custom path must be non-empty to advance.
	if m.locationCursor == 2 && m.customPath == "" {
		return m, nil
	}
	m.methodCursor = m.defaultMethodCursor()
	m.step = installStepMethod
	m.shell.SetActive(2)
	return m, nil
}

func (m *installWizardModel) updateKeyLocation(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	// When on Custom (cursor==2), text input is active.
	if m.locationCursor == 2 {
		switch msg.Type {
		case tea.KeyEsc:
			return m.locationGoBack()
		case tea.KeyUp:
			m.locationCursor = 1
			return m, nil
		case tea.KeyEnter:
			return m.locationAdvance()
		case tea.KeyBackspace:
			if m.customCursor > 0 {
				runes := []rune(m.customPath)
				m.customPath = string(runes[:m.customCursor-1]) + string(runes[m.customCursor:])
				m.customCursor--
			}
			return m, nil
		case tea.KeyLeft:
			if m.customCursor > 0 {
				m.customCursor--
			}
			return m, nil
		case tea.KeyRight:
			if m.customCursor < len([]rune(m.customPath)) {
				m.customCursor++
			}
			return m, nil
		case tea.KeyHome, tea.KeyCtrlA:
			m.customCursor = 0
			return m, nil
		case tea.KeyEnd, tea.KeyCtrlE:
			m.customCursor = len([]rune(m.customPath))
			return m, nil
		case tea.KeySpace:
			runes := []rune(m.customPath)
			newRunes := make([]rune, 0, len(runes)+1)
			newRunes = append(newRunes, runes[:m.customCursor]...)
			newRunes = append(newRunes, ' ')
			newRunes = append(newRunes, runes[m.customCursor:]...)
			m.customPath = string(newRunes)
			m.customCursor++
			return m, nil
		case tea.KeyRunes:
			runes := []rune(m.customPath)
			newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
			newRunes = append(newRunes, runes[:m.customCursor]...)
			newRunes = append(newRunes, msg.Runes...)
			newRunes = append(newRunes, runes[m.customCursor:]...)
			m.customPath = string(newRunes)
			m.customCursor += len(msg.Runes)
			return m, nil
		}
		return m, nil
	}

	// Global (0) or Project (1) — radio-button mode, no text input.
	switch {
	case msg.Type == tea.KeyEsc:
		return m.locationGoBack()
	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.locationCursor < 2 {
			m.locationCursor++
		}
	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.locationCursor > 0 {
			m.locationCursor--
		}
	case msg.Type == tea.KeyEnter:
		return m.locationAdvance()
	}
	return m, nil
}

func (m *installWizardModel) updateMouseLocation(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	for i := 0; i < 3; i++ {
		if zone.Get(fmt.Sprintf("inst-loc-%d", i)).InBounds(msg) {
			m.locationCursor = i
			return m, nil
		}
	}
	if zone.Get("inst-back").InBounds(msg) {
		return m.locationGoBack()
	}
	if zone.Get("inst-next").InBounds(msg) {
		return m.locationAdvance()
	}
	return m, nil
}

// resolvedInstallPath returns the display path for a location option.
func (m *installWizardModel) resolvedInstallPath(loc int) string {
	prov := m.providers[m.providerCursor]
	switch loc {
	case 0: // Global
		home, err := os.UserHomeDir()
		if err != nil {
			return prov.InstallDir("~", m.item.Type)
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

// renderLocationInput renders the custom path text input with cursor.
func (m *installWizardModel) renderLocationInput(usableW int) string {
	fieldW := usableW - 2 // padding inside field
	bg := inputInactiveBG
	if m.locationCursor == 2 {
		bg = inputActiveBG
	}

	var displayVal string
	if m.locationCursor == 2 {
		// Show cursor
		runes := []rune(m.customPath)
		if m.customCursor >= len(runes) {
			displayVal = truncate(m.customPath+"\u2588", fieldW)
		} else {
			before := string(runes[:m.customCursor])
			under := string(runes[m.customCursor : m.customCursor+1])
			after := string(runes[m.customCursor+1:])
			cursorChar := lipgloss.NewStyle().Reverse(true).Render(under)
			displayVal = truncate(before+cursorChar+after, fieldW)
		}
	} else {
		if m.customPath == "" {
			displayVal = ""
		} else {
			displayVal = truncate(m.customPath, fieldW)
		}
	}

	style := lipgloss.NewStyle().
		Background(bg).
		Foreground(primaryText).
		Width(usableW).
		Padding(0, 1)
	return zone.Mark("inst-custom-path", style.Render(displayVal))
}

func (m *installWizardModel) viewLocation() string {
	pad := "  "
	usableW := m.width - 4

	provName := m.providers[m.providerCursor].Name
	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(
		fmt.Sprintf("Install location for %s:", provName))

	var lines []string
	lines = append(lines, title, "")

	labels := []string{"Global", "Project", "Custom"}
	for i, label := range labels {
		var row string
		prefix := "  "
		if i == m.locationCursor {
			prefix = "> "
		}

		if i < 2 {
			// Global or Project: show resolved path
			path := m.resolvedInstallPath(i)
			pathStr := lipgloss.NewStyle().Foreground(mutedColor).Render("(" + path + ")")
			if i == m.locationCursor {
				row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+label) +
					" " + pathStr
			} else {
				row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+label) +
					" " + pathStr
			}
			lines = append(lines, zone.Mark(fmt.Sprintf("inst-loc-%d", i), row))
		} else {
			// Custom: show label + text input on same line
			if i == m.locationCursor {
				row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+label) +
					"   "
			} else {
				row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+label) +
					"   "
			}
			inputField := m.renderLocationInput(usableW - lipgloss.Width(row))
			lines = append(lines, zone.Mark(fmt.Sprintf("inst-loc-%d", i), row+inputField))
		}
	}

	// Buttons: Back and Next
	lines = append(lines, "")
	lines = append(lines, renderModalButtons(1, usableW, pad, nil,
		buttonDef{"Back", "inst-back", 0},
		buttonDef{"Next", "inst-next", 1},
	))

	return strings.Join(lines, "\n")
}

// --- Method step ---

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

func (m *installWizardModel) methodGoBack() (*installWizardModel, tea.Cmd) {
	m.step = installStepLocation
	m.shell.SetActive(1)
	return m, nil
}

func (m *installWizardModel) methodAdvance() (*installWizardModel, tea.Cmd) {
	m.enterReview(3)
	return m, nil
}

func (m *installWizardModel) updateKeyMethod(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m.methodGoBack()

	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.methodCursor == 0 {
			m.methodCursor = 1
		}
		// If at 1 already, stay (no wrap — only 2 options).

	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.methodCursor == 1 && !m.symlinkDisabled() {
			m.methodCursor = 0
		}
		// If symlink disabled or already at 0, stay.

	case msg.Type == tea.KeyEnter:
		return m.methodAdvance()
	}
	return m, nil
}

func (m *installWizardModel) updateMouseMethod(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	if zone.Get("inst-method-0").InBounds(msg) && !m.symlinkDisabled() {
		m.methodCursor = 0
		return m, nil
	}
	if zone.Get("inst-method-1").InBounds(msg) {
		m.methodCursor = 1
		return m, nil
	}
	if zone.Get("inst-back").InBounds(msg) {
		return m.methodGoBack()
	}
	if zone.Get("inst-next").InBounds(msg) {
		return m.methodAdvance()
	}
	return m, nil
}

func (m *installWizardModel) viewMethod() string {
	pad := "  "
	usableW := m.width - 4

	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render("Install method:")

	var lines []string
	lines = append(lines, title, "")

	// Option 0: Symlink
	{
		prefix := "  "
		if m.methodCursor == 0 {
			prefix = "> "
		}
		var row string
		if m.symlinkDisabled() {
			row = pad + lipgloss.NewStyle().Foreground(mutedColor).Render(
				prefix+"Symlink   (not supported for this provider)")
		} else if m.methodCursor == 0 {
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+"Symlink") +
				"   " + lipgloss.NewStyle().Foreground(mutedColor).Render("(recommended — stays in sync with library)")
		} else {
			row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+"Symlink") +
				"   " + lipgloss.NewStyle().Foreground(mutedColor).Render("(recommended — stays in sync with library)")
		}
		lines = append(lines, zone.Mark("inst-method-0", row))
	}

	// Option 1: Copy
	{
		prefix := "  "
		if m.methodCursor == 1 {
			prefix = "> "
		}
		var row string
		if m.methodCursor == 1 {
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+"Copy") +
				"      " + lipgloss.NewStyle().Foreground(mutedColor).Render("(standalone copy, won't auto-update)")
		} else {
			row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+"Copy") +
				"      " + lipgloss.NewStyle().Foreground(mutedColor).Render("(standalone copy, won't auto-update)")
		}
		lines = append(lines, zone.Mark("inst-method-1", row))
	}

	// Buttons: Back and Next
	lines = append(lines, "")
	lines = append(lines, renderModalButtons(1, usableW, pad, nil,
		buttonDef{"Back", "inst-back", 0},
		buttonDef{"Next", "inst-next", 1},
	))

	return strings.Join(lines, "\n")
}

// View renders the install wizard.
func (m *installWizardModel) View() string {
	if m == nil {
		return ""
	}

	// Wizard shell (step breadcrumbs)
	header := m.shell.View()

	// Per-step content
	var content string
	switch m.step {
	case installStepProvider:
		content = m.viewProvider()
	case installStepLocation:
		content = m.viewLocation()
	case installStepMethod:
		content = m.viewMethod()
	case installStepReview:
		if m.riskDrillIn {
			content = m.viewReviewDrillIn()
		} else {
			content = m.viewReview()
		}
	}

	return header + "\n" + content
}

// viewProvider renders the provider picker step.
func (m *installWizardModel) viewProvider() string {
	pad := "  "
	usableW := m.width - 4 // rough usable width for button alignment

	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(
		fmt.Sprintf("Install %q to which provider?", m.itemName))

	var lines []string
	lines = append(lines, title, "")

	for i, prov := range m.providers {
		var row string
		switch {
		case m.providerInstalled[i]:
			// Already installed: muted
			row = pad + "  " + lipgloss.NewStyle().Foreground(mutedColor).Render(
				prov.Name+" (already installed)")
		case i == m.providerCursor:
			// Selected (cursor): bold accent with arrow
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(
				"> "+prov.Name) + " " + lipgloss.NewStyle().Foreground(mutedColor).Render("(detected)")
		default:
			// Normal: primary text
			row = pad + "  " + lipgloss.NewStyle().Foreground(primaryText).Render(
				prov.Name) + " " + lipgloss.NewStyle().Foreground(mutedColor).Render("(detected)")
		}
		lines = append(lines, zone.Mark(fmt.Sprintf("inst-prov-%d", i), row))
	}

	// Buttons: Cancel and Next. Next is always visually focused (focusAt=1).
	lines = append(lines, "")
	lines = append(lines, renderModalButtons(1, usableW, pad, nil,
		buttonDef{"Cancel", "inst-cancel", 0},
		buttonDef{"Next", "inst-next", 1},
	))

	return strings.Join(lines, "\n")
}

// --- Review step ---

// enterReview transitions to the review step, computing risks and initializing
// the risk banner. shellIdx is the wizard shell step index for Review (varies
// by content type: 3 for filesystem, 1 for JSON merge).
func (m *installWizardModel) enterReview(shellIdx int) {
	m.step = installStepReview
	m.shell.SetActive(shellIdx)
	m.risks = catalog.RiskIndicators(m.item)
	m.riskBanner = newRiskBanner(m.risks, m.width-4)
	m.focusIdx = 2 // Default focus on Install button
	m.confirmed = false
	m.riskDrillIn = false
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

// reviewGoBack navigates back from the review step, accounting for JSON merge
// and auto-skipped provider states.
func (m *installWizardModel) reviewGoBack() (*installWizardModel, tea.Cmd) {
	if m.isJSONMerge && m.autoSkippedProvider {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	if m.isJSONMerge {
		m.step = installStepProvider
		m.shell.SetActive(0)
		return m, nil
	}
	// Filesystem: back to method
	m.step = installStepMethod
	m.shell.SetActive(2)
	return m, nil
}

func (m *installWizardModel) updateKeyReview(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	if m.riskDrillIn {
		return m.updateKeyReviewDrillIn(msg)
	}
	return m.updateKeyReviewNormal(msg)
}

func (m *installWizardModel) updateKeyReviewNormal(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m.reviewGoBack()

	case msg.Type == tea.KeyLeft ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'h'):
		if m.focusIdx > 0 {
			m.focusIdx--
		}

	case msg.Type == tea.KeyRight ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'l'):
		if m.focusIdx < 2 {
			m.focusIdx++
		}

	case msg.Type == tea.KeyTab:
		m.focusIdx = (m.focusIdx + 1) % 3

	case msg.Type == tea.KeyShiftTab:
		m.focusIdx = (m.focusIdx + 2) % 3

	case msg.Type == tea.KeyUp || msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 &&
			(msg.Runes[0] == 'j' || msg.Runes[0] == 'k')):
		// Delegate to risk banner for vertical navigation
		var cmd tea.Cmd
		m.riskBanner, cmd = m.riskBanner.Update(msg)
		if cmd != nil {
			// Check if it's a drill-in message
			result := cmd()
			if drillMsg, ok := result.(riskDrillInMsg); ok {
				m.enterDrillIn(drillMsg.risk)
				return m, nil
			}
		}

	case msg.Type == tea.KeyEnter:
		switch m.focusIdx {
		case 0: // Cancel
			return m, func() tea.Msg { return installCloseMsg{} }
		case 1: // Back
			return m.reviewGoBack()
		case 2: // Install
			if !m.confirmed {
				m.confirmed = true
				result := m.installResult()
				return m, func() tea.Msg { return result }
			}
		}
	}
	return m, nil
}

func (m *installWizardModel) updateKeyReviewDrillIn(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		m.riskDrillIn = false

	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		m.drillPreview.ScrollUp()

	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		m.drillPreview.ScrollDown()

	case msg.Type == tea.KeyPgUp:
		m.drillPreview.PageUp()

	case msg.Type == tea.KeyPgDown:
		m.drillPreview.PageDown()
	}
	return m, nil
}

// enterDrillIn sets up the drill-in preview for a risk indicator.
func (m *installWizardModel) enterDrillIn(risk catalog.RiskIndicator) {
	m.riskDrillIn = true
	if len(risk.Lines) == 0 {
		// No specific lines — show the primary file
		m.drillPreview = newPreviewModel()
		m.drillPreview.SetSize(m.width-4, m.height-5)
		m.drillPreview.LoadItem(&m.item)
		return
	}

	rl := risk.Lines[0]
	m.drillPreview = newPreviewModel()
	m.drillPreview.SetSize(m.width-4, m.height-5)
	content, err := catalog.ReadFileContent(m.item.Path, rl.File, 500)
	if err == nil {
		m.drillPreview.lines = strings.Split(content, "\n")
		m.drillPreview.fileName = rl.File
	} else {
		m.drillPreview.lines = []string{"Error reading file:", err.Error()}
		m.drillPreview.fileName = rl.File
	}

	// Set highlight lines (all lines from this risk in the same file)
	highlights := make(map[int]bool)
	for _, l := range risk.Lines {
		if l.File == rl.File {
			highlights[l.Line] = true
		}
	}
	m.drillPreview.SetHighlightLines(highlights)

	// Scroll to center on the first highlighted line
	if rl.Line > 0 {
		m.drillPreview.offset = max(0, rl.Line-3)
	}
}

func (m *installWizardModel) updateMouseReview(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	if m.riskDrillIn {
		return m, nil // drill-in only supports keyboard
	}
	if zone.Get("inst-cancel").InBounds(msg) {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	if zone.Get("inst-back").InBounds(msg) {
		return m.reviewGoBack()
	}
	if zone.Get("inst-install").InBounds(msg) {
		if !m.confirmed {
			m.confirmed = true
			result := m.installResult()
			return m, func() tea.Msg { return result }
		}
	}
	// Risk item clicks
	for i := range m.risks {
		if zone.Get(fmt.Sprintf("risk-%d", i)).InBounds(msg) {
			m.riskBanner.cursor = i
			return m, nil
		}
	}
	return m, nil
}

func (m *installWizardModel) viewReview() string {
	pad := "  "
	usableW := m.width - 4

	provName := m.providers[m.providerCursor].Name
	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(
		fmt.Sprintf("Installing %q to %s", m.itemName, provName))

	var lines []string
	lines = append(lines, title, "")

	if m.isJSONMerge {
		// JSON merge items show merge target
		lines = append(lines, pad+mutedStyle.Render("Will merge into ")+
			lipgloss.NewStyle().Foreground(primaryText).Render(provName+" settings"))
	} else {
		// Filesystem items show location and method
		locLabels := []string{"Global", "Project", "Custom"}
		locLabel := locLabels[m.locationCursor]
		locPath := m.resolvedInstallPath(m.locationCursor)

		methodLabel := "Symlink"
		if m.methodCursor == 1 {
			methodLabel = "Copy"
		}

		lines = append(lines, pad+mutedStyle.Render("Location: ")+
			lipgloss.NewStyle().Foreground(primaryText).Render(locLabel+" ("+locPath+")"))
		lines = append(lines, pad+mutedStyle.Render("Method:   ")+
			lipgloss.NewStyle().Foreground(primaryText).Render(methodLabel))
	}

	// Risk banner (renders empty string when no risks)
	bannerView := m.riskBanner.View()
	if bannerView != "" {
		lines = append(lines, "")
		lines = append(lines, pad+bannerView)
	}

	// Buttons: Cancel, Back, Install
	lines = append(lines, "")
	lines = append(lines, renderModalButtons(m.focusIdx, usableW, pad,
		[]string{"Install"}, // Install uses danger styling when focused
		buttonDef{"Cancel", "inst-cancel", 0},
		buttonDef{"Back", "inst-back", 1},
		buttonDef{"Install", "inst-install", 2},
	))

	return strings.Join(lines, "\n")
}

func (m *installWizardModel) viewReviewDrillIn() string {
	pad := "  "

	// Find the label of the risk being inspected
	riskLabel := "Risk"
	if m.riskBanner.cursor >= 0 && m.riskBanner.cursor < len(m.risks) {
		riskLabel = m.risks[m.riskBanner.cursor].Label
	}

	var lines []string
	lines = append(lines, pad+lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(
		"Inspecting: "+riskLabel)+" "+mutedStyle.Render("(Esc to return)"))
	lines = append(lines, "")
	lines = append(lines, m.drillPreview.View())

	return strings.Join(lines, "\n")
}
