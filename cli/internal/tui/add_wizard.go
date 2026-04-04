package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tidwall/gjson"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- Step enum ---

type addStep int

const (
	addStepSource addStep = iota
	addStepType
	addStepDiscovery
	addStepTriage // conditional, between Discovery and Review
	addStepReview
	addStepExecute
)

// --- Source types ---

type addSource int

const (
	addSourceNone addSource = iota
	addSourceProvider
	addSourceRegistry
	addSourceLocal
	addSourceGit
)

// --- Review zones ---

type addReviewZone int

const (
	addReviewZoneRisks addReviewZone = iota // unused — kept to preserve iota values
	addReviewZoneItems
	addReviewZoneButtons
)

// --- Triage step types ---

type triageFocusZone int

const (
	triageZoneItems triageFocusZone = iota
	triageZonePreview
	triageZoneButtons
)

// addConfirmItem holds a confirm-bucket item awaiting user triage.
type addConfirmItem struct {
	detected    *analyzer.DetectedItem
	tier        analyzer.ConfidenceTier
	displayName string
	itemType    catalog.ContentType
	path        string // primary file path (relative to source root)
	sourceDir   string // absolute directory containing the item
}

// --- Messages ---

type addCloseMsg struct{}
type addRestartMsg struct{} // restart the wizard from Source step

type addDiscoveryDoneMsg struct {
	seq              int
	items            []addDiscoveryItem
	confirmItems     []addConfirmItem // items requiring user triage
	err              error
	tmpDir           string // non-empty for git URL source
	sourceRegistry   string
	sourceVisibility string
}

type addExecItemDoneMsg struct {
	seq    int
	index  int
	result addExecResult
}

type addExecAllDoneMsg struct {
	seq int
}

// --- Discovery item ---

type addDiscoveryItem struct {
	name        string
	displayName string
	itemType    catalog.ContentType
	path        string
	sourceDir   string
	status      add.ItemStatus
	scope       string
	risks       []catalog.RiskIndicator
	overwrite   bool
	underlying  *add.DiscoveryItem
	catalogItem *catalog.ContentItem // original catalog item when available (has correct Files/Path)

	confidence      float64
	detectionSource string
	tier            analyzer.ConfidenceTier
}

// --- Execute result ---

type addExecResult struct {
	name   string
	status string // "added", "updated", "error", "skipped", "cancelled"
	err    error
}

// --- Model ---

type addWizardModel struct {
	shell   wizardShell
	step    addStep
	maxStep addStep // furthest step reached, for forward breadcrumb navigation
	width   int
	height  int
	seq     int // sequence number for stale message detection

	// Source step
	source         addSource
	sourceCursor   int  // 0=Provider, 1=Registry, 2=Local, 3=Git
	sourceExpanded bool // true when sub-list (provider/registry picker) is open
	inputActive    bool // true when text input (local/git) is active
	pathInput      string
	pathCursor     int
	sourceErr      string // inline error for invalid path/URL

	// Provider sub-list
	providers      []provider.Provider // detected providers only
	providerCursor int

	// Registry sub-list
	registries     []catalog.RegistrySource
	registryCursor int

	// Type step
	typeChecks    checkboxList
	preFilterType catalog.ContentType // non-empty = skip Type step (4-step path)

	// Discovery step
	discovering     bool
	discoveredItems []addDiscoveryItem
	discoveryList   checkboxList
	discoveryErr    string
	showInstalled   bool // toggle for showing "in library" items
	installedCount  int  // number of installed items (at end of discoveredItems)
	actionableCount int  // number of actionable items (New/Outdated)

	// Taint
	sourceRegistry   string
	sourceVisibility string

	// Review step
	risks              []catalog.RiskIndicator
	riskItemMap        []int // maps each index in m.risks to the index in selectedItems()
	riskBanner         riskBanner
	conflicts          []int
	reviewZone         addReviewZone
	reviewItemCursor   int
	reviewItemOffset   int
	buttonCursor       int
	reviewAcknowledged bool

	// Review drill-in sub-view
	reviewDrillIn      bool
	reviewDrillTree    fileTreeModel
	reviewDrillPreview previewModel
	drillInRiskyFiles  map[string]catalog.RiskLevel // file path -> highest risk level
	drillInItemRisks   []catalog.RiskIndicator      // risks for the drilled-in item

	// Execute step
	executeResults   []addExecResult
	executeCurrent   int
	executeOffset    int
	executeDone      bool
	executeCancelled bool
	executing        bool

	// Git source
	gitTempDir string

	// Triage step
	hasTriageStep   bool
	confirmItems    []addConfirmItem
	confirmSelected map[int]bool
	confirmCursor   int
	confirmOffset   int
	confirmPreview  previewModel
	confirmFocus    triageFocusZone

	// Context
	projectRoot string
	contentRoot string
	cfg         *config.Config
}

// openAddWizard creates a new add wizard.
func openAddWizard(
	providers []provider.Provider,
	registries []catalog.RegistrySource,
	cfg *config.Config,
	projectRoot string,
	contentRoot string,
	preFilterType catalog.ContentType,
) *addWizardModel {
	// Filter to detected providers
	var detected []provider.Provider
	for _, p := range providers {
		if p.Detected {
			detected = append(detected, p)
		}
	}

	// Default source cursor: Provider if any detected, else Local
	sourceCursor := 2 // Local
	if len(detected) > 0 {
		sourceCursor = 0 // Provider
	}

	m := &addWizardModel{
		step:          addStepSource,
		providers:     detected,
		registries:    registries,
		cfg:           cfg,
		projectRoot:   projectRoot,
		contentRoot:   contentRoot,
		preFilterType: preFilterType,
		sourceCursor:  sourceCursor,
		buttonCursor:  1, // default to [Back]
	}
	// buildShellLabels reads preFilterType and hasTriageStep, so call after struct init
	m.shell = newWizardShell("Add", m.buildShellLabels())

	return m
}

// Init satisfies the tea.Model interface.
func (m *addWizardModel) Init() tea.Cmd { return nil }

// validateStep checks entry-prerequisites for the current step.
func (m *addWizardModel) validateStep() {
	switch m.step {
	case addStepSource:
		// no prerequisites
	case addStepType:
		if m.source == addSourceNone {
			panic("wizard invariant: addStepType entered without source")
		}
	case addStepDiscovery:
		if m.source == addSourceNone {
			panic("wizard invariant: addStepDiscovery entered without source")
		}
		if !m.discovering && len(m.selectedTypes()) == 0 {
			panic("wizard invariant: addStepDiscovery entered without selected types")
		}
	case addStepTriage:
		if !m.hasTriageStep {
			panic("wizard invariant: addStepTriage entered without hasTriageStep")
		}
		if len(m.confirmItems) == 0 {
			panic("wizard invariant: addStepTriage entered with empty confirmItems")
		}
	case addStepReview:
		if len(m.discoveredItems) == 0 {
			panic("wizard invariant: addStepReview entered without discovered items")
		}
		if len(m.selectedItems()) == 0 {
			panic("wizard invariant: addStepReview entered without selected items")
		}
	case addStepExecute:
		if len(m.selectedItems()) == 0 {
			panic("wizard invariant: addStepExecute entered without selected items")
		}
		if !m.reviewAcknowledged {
			panic("wizard invariant: addStepExecute entered without review acknowledgment")
		}
	}
}

// updateMaxStep updates maxStep to the current step if it's further than before,
// and syncs the shell's maxCompleted for breadcrumb clickability.
func (m *addWizardModel) updateMaxStep() {
	if m.step > m.maxStep {
		m.maxStep = m.step
	}
	m.shell.maxCompleted = m.shellIndexForStep(m.maxStep)
}

// stepHints returns helpbar hints for the current wizard step.
func (m *addWizardModel) stepHints() []string {
	base := []string{"? help"}
	switch m.step {
	case addStepSource:
		if m.inputActive {
			return append([]string{"type path/URL", "enter confirm", "esc cancel"}, base...)
		}
		if m.sourceExpanded {
			return append([]string{"↑/↓ select", "enter confirm", "esc collapse"}, base...)
		}
		return append([]string{"↑/↓ select", "enter expand", "esc close wizard"}, base...)
	case addStepType:
		return append([]string{"↑/↓ navigate", "space toggle", "a all", "n none", "enter next", "esc back"}, base...)
	case addStepDiscovery:
		if m.discovering {
			return append([]string{"esc cancel"}, base...)
		}
		if m.discoveryErr != "" {
			return append([]string{"r retry", "esc back"}, base...)
		}
		hints := []string{"↑/↓ navigate", "space toggle", "a all", "n none"}
		if m.installedCount > 0 {
			hints = append(hints, "h show/hide installed")
		}
		return append(append(hints, "enter next", "esc back"), base...)
	case addStepTriage:
		return append([]string{
			"↑/↓ navigate", "space toggle", "a all", "n none",
			"tab switch panes", "enter next", "esc back",
		}, base...)
	case addStepReview:
		if m.reviewDrillIn {
			return append([]string{"tab/←/→ switch panes", "↑/↓ navigate", "pgup/pgdn scroll", "esc back to review"}, base...)
		}
		return append([]string{"tab cycle zones", "↑/↓ navigate", "←/→ buttons", "enter inspect", "esc back"}, base...)
	case addStepExecute:
		if m.executeDone {
			return append([]string{"enter close", "a add more", "↑/↓ scroll"}, base...)
		}
		return append([]string{"esc cancel remaining"}, base...)
	}
	return base
}

// selectedTypes returns the content types checked in the type step.
// If preFilterType is set, returns just that type.
func (m *addWizardModel) selectedTypes() []catalog.ContentType {
	if m.preFilterType != "" {
		return []catalog.ContentType{m.preFilterType}
	}
	allTypes := []catalog.ContentType{
		catalog.Rules, catalog.Skills, catalog.Agents,
		catalog.Hooks, catalog.MCP, catalog.Commands,
	}
	var result []catalog.ContentType
	for _, idx := range m.typeChecks.SelectedIndices() {
		if idx < len(allTypes) {
			result = append(result, allTypes[idx])
		}
	}
	return result
}

// selectedItems returns the discovered items that are checked in the discovery list.
func (m *addWizardModel) selectedItems() []addDiscoveryItem {
	visible := m.visibleDiscoveryItems()
	var result []addDiscoveryItem
	for _, idx := range m.discoveryList.SelectedIndices() {
		if idx < len(visible) {
			result = append(result, visible[idx])
		}
	}
	return result
}

// buildShellLabels returns the correct step label slice for the current permutation.
func (m *addWizardModel) buildShellLabels() []string {
	if m.preFilterType != "" && m.hasTriageStep {
		return []string{"Source", "Discovery", "Triage", "Review", "Execute"}
	}
	if m.preFilterType != "" {
		return []string{"Source", "Discovery", "Review", "Execute"}
	}
	if m.hasTriageStep {
		return []string{"Source", "Type", "Discovery", "Triage", "Review", "Execute"}
	}
	return []string{"Source", "Type", "Discovery", "Review", "Execute"}
}

// clearTriageState resets all triage step state. Call from goBackFromDiscovery()
// and advanceFromSource() to prevent stale triage data surviving source changes.
func (m *addWizardModel) clearTriageState() {
	m.hasTriageStep = false
	m.confirmItems = nil
	m.confirmSelected = nil
	m.confirmCursor = 0
	m.confirmOffset = 0
	m.confirmPreview = previewModel{}
	m.confirmFocus = triageZoneItems
	m.maxStep = addStepDiscovery
	m.shell.SetSteps(m.buildShellLabels())
	m.updateMaxStep()
}

// shellIndexForStep maps an addStep to the wizard shell breadcrumb index,
// accounting for all 4 permutations of Type and Triage step inclusion.
func (m *addWizardModel) shellIndexForStep(s addStep) int {
	hasType := m.preFilterType == ""
	has := m.hasTriageStep

	switch s {
	case addStepSource:
		return 0
	case addStepType:
		if !hasType {
			panic("shellIndexForStep: addStepType in -Type permutation")
		}
		return 1
	case addStepDiscovery:
		if hasType {
			return 2
		}
		return 1
	case addStepTriage:
		if !has {
			panic("shellIndexForStep: addStepTriage in -Triage permutation")
		}
		if hasType {
			return 3
		}
		return 2
	case addStepReview:
		if hasType && has {
			return 4
		}
		if hasType || has {
			return 3
		}
		return 2
	case addStepExecute:
		if hasType && has {
			return 5
		}
		if hasType || has {
			return 4
		}
		return 3
	}
	return int(s)
}

// stepForShellIndex is the inverse of shellIndexForStep.
// Used by breadcrumb click handler to map shell index → step enum.
func (m *addWizardModel) stepForShellIndex(idx int) addStep {
	hasType := m.preFilterType == ""
	hasTriage := m.hasTriageStep

	switch {
	case hasType && hasTriage:
		// Source(0) Type(1) Discovery(2) Triage(3) Review(4) Execute(5)
		return addStep(idx) // direct mapping
	case hasType && !hasTriage:
		// Source(0) Type(1) Discovery(2) Review(3) Execute(4)
		switch idx {
		case 0:
			return addStepSource
		case 1:
			return addStepType
		case 2:
			return addStepDiscovery
		case 3:
			return addStepReview
		case 4:
			return addStepExecute
		}
	case !hasType && hasTriage:
		// Source(0) Discovery(1) Triage(2) Review(3) Execute(4)
		switch idx {
		case 0:
			return addStepSource
		case 1:
			return addStepDiscovery
		case 2:
			return addStepTriage
		case 3:
			return addStepReview
		case 4:
			return addStepExecute
		}
	default:
		// -Type -Triage: Source(0) Discovery(1) Review(2) Execute(3)
		switch idx {
		case 0:
			return addStepSource
		case 1:
			return addStepDiscovery
		case 2:
			return addStepReview
		case 3:
			return addStepExecute
		}
	}
	return addStepSource
}

// advanceFromSource transitions from the Source step to the next step.
// Always clears downstream state to avoid showing stale data.
// Returns a tea.Cmd that callers must propagate (non-nil when preFilterType
// triggers immediate discovery).
func (m *addWizardModel) advanceFromSource() tea.Cmd {
	m.typeChecks = m.buildTypeCheckList()
	m.discoveredItems = nil
	m.discoveryList = checkboxList{}
	m.discoveryErr = ""
	m.risks = nil
	m.reviewAcknowledged = false
	m.clearTriageState()
	m.seq++

	if m.preFilterType != "" {
		// Skip Type step — go straight to Discovery and start scanning
		m.step = addStepDiscovery
		m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
		m.discovering = true
		m.updateMaxStep()
		return m.startDiscoveryCmd()
	}
	m.step = addStepType
	m.shell.SetActive(m.shellIndexForStep(addStepType))
	m.updateMaxStep()
	return nil
}

// buildTypeCheckList builds the checkbox list for the Type step.
func (m *addWizardModel) buildTypeCheckList() checkboxList {
	items := []checkboxItem{
		{label: "Rules"},
		{label: "Skills"},
		{label: "Agents"},
		{label: "Hooks"},
		{label: "MCP"},
		{label: "Commands"},
	}
	cl := newCheckboxList(items)
	cl.focused = true
	cl.zonePrefix = "add-type"

	// Default: all checked for most sources.
	// For provider source: only check types supported by that provider.
	allTypes := []catalog.ContentType{
		catalog.Rules, catalog.Skills, catalog.Agents,
		catalog.Hooks, catalog.MCP, catalog.Commands,
	}

	if m.source == addSourceProvider && m.providerCursor < len(m.providers) {
		prov := m.providers[m.providerCursor]
		for i, ct := range allTypes {
			if prov.SupportsType != nil && prov.SupportsType(ct) {
				cl.selected[i] = true
			}
		}
	} else {
		// All checked by default
		for i := range cl.selected {
			cl.selected[i] = true
		}
	}

	// Apply height constraint so the list scrolls instead of overflowing
	cl = cl.SetSize(m.width-4, m.typeListHeight())

	return cl
}

// goBackFromDiscovery navigates back from the Discovery step.
func (m *addWizardModel) goBackFromDiscovery() {
	m.clearTriageState()
	m.discoveredItems = nil
	m.discoveryList = checkboxList{}
	m.discoveryErr = ""
	m.discovering = false

	if m.gitTempDir != "" {
		_ = os.RemoveAll(m.gitTempDir)
		m.gitTempDir = ""
	}

	if m.preFilterType != "" {
		m.step = addStepSource
		m.shell.SetActive(0)
	} else {
		m.step = addStepType
		m.shell.SetActive(1)
	}
}

// enterReview transitions to the Review step.
func (m *addWizardModel) enterReview() {
	m.step = addStepReview
	m.shell.SetActive(m.shellIndexForStep(addStepReview))

	// Compute aggregate risks with per-item mapping
	m.risks = nil
	m.riskItemMap = nil
	for i, item := range m.selectedItems() {
		for range item.risks {
			m.riskItemMap = append(m.riskItemMap, i)
		}
		m.risks = append(m.risks, item.risks...)
	}
	m.riskBanner = newRiskBanner(m.risks, m.width-4)

	// Detect conflicts (Outdated status)
	m.conflicts = nil
	for _, idx := range m.discoveryList.SelectedIndices() {
		if idx < len(m.discoveredItems) && m.discoveredItems[idx].status == add.StatusOutdated {
			m.conflicts = append(m.conflicts, idx)
		}
	}

	// Default focus: items zone so per-item risk info shows immediately
	m.reviewZone = addReviewZoneItems
	m.buttonCursor = 1 // [Back]
	m.reviewItemCursor = 0
	m.reviewItemOffset = 0
	m.reviewAcknowledged = false
	m.updateMaxStep()
}

// highlightRisksForItem updates the risk banner highlighting to show which risks
// belong to the currently selected review item.
func (m *addWizardModel) highlightRisksForItem(itemIdx int) {
	var indices []int
	for i, mappedItem := range m.riskItemMap {
		if mappedItem == itemIdx {
			indices = append(indices, i)
		}
	}
	m.riskBanner.SetHighlighted(indices)
}

// enterReviewDrillIn opens the drill-in sub-view for the selected review item.
func (m *addWizardModel) enterReviewDrillIn() {
	selected := m.selectedItems()
	if m.reviewItemCursor >= len(selected) {
		return
	}
	item := selected[m.reviewItemCursor]

	// Use the catalogItem directly when available (has correct Path + Files
	// from the catalog scanner). Fall back to filesystem scan for items
	// discovered from providers (hooks, MCP, file-based without catalog scan).
	var ci catalog.ContentItem
	if item.catalogItem != nil {
		ci = *item.catalogItem
	} else if (item.itemType == catalog.Hooks || item.itemType == catalog.MCP) && item.path != "" {
		// Hooks/MCP from providers: extract just the relevant JSON section
		// instead of showing the entire settings file.
		ci = catalog.ContentItem{
			Name:  item.name,
			Type:  item.itemType,
			Path:  filepath.Dir(item.path),
			Files: []string{filepath.Base(item.path)},
		}
		// Override the preview content with just the relevant section
		extracted := extractJSONSection(item.path, item.name, item.itemType)
		if extracted != "" {
			m.reviewDrillTree = newFileTreeModel([]string{item.name + ".json"})
			m.reviewDrillTree.focused = true
			m.reviewDrillPreview = newPreviewModel()
			m.reviewDrillPreview.fileName = item.name + ".json"
			m.reviewDrillPreview.lines = strings.Split(extracted, "\n")

			// Highlight key config lines (command, url, env) so the user
			// can immediately see what this hook/MCP server does
			highlights := make(map[int]bool)
			for lineNum, line := range m.reviewDrillPreview.lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, `"command"`) ||
					strings.Contains(lower, `"url"`) ||
					strings.Contains(lower, `"env"`) {
					highlights[lineNum+1] = true // 1-based
				}
			}
			if len(highlights) > 0 {
				m.reviewDrillPreview.SetHighlightLines(highlights)
			}

			m.setupDrillInRisks(item)
			return
		}
		// Fall through to normal handling if extraction fails
	} else {
		// Construct from discovery item fields
		itemPath := item.path
		if item.sourceDir != "" {
			itemPath = item.sourceDir
		}
		ci = catalog.ContentItem{
			Name: item.name,
			Type: item.itemType,
		}
		info, err := os.Stat(itemPath)
		if err != nil {
			return
		}
		if info.IsDir() {
			ci.Path = itemPath
			ci.Files = scanDrillInFiles(itemPath)
		} else {
			ci.Path = filepath.Dir(itemPath)
			ci.Files = []string{filepath.Base(itemPath)}
		}
	}

	m.reviewDrillTree = newFileTreeModel(ci.Files)
	m.reviewDrillTree.focused = true
	m.reviewDrillPreview = newPreviewModel()
	m.reviewDrillPreview.LoadItem(&ci)
	m.setupDrillInRisks(item)
}

// setupDrillInRisks configures risk badges on the tree, highlight lines on the
// preview, and sizes the panes. Called at the end of enterReviewDrillIn.
func (m *addWizardModel) setupDrillInRisks(item addDiscoveryItem) {
	// Build a set of risky file paths for tree annotation
	m.drillInRiskyFiles = make(map[string]catalog.RiskLevel)
	for _, r := range item.risks {
		for _, rl := range r.Lines {
			if rl.File != "" {
				if existing, ok := m.drillInRiskyFiles[rl.File]; !ok || r.Level > existing {
					m.drillInRiskyFiles[rl.File] = r.Level
				}
			}
		}
	}
	m.drillInItemRisks = item.risks

	// Set risk badges on tree nodes
	if len(m.drillInRiskyFiles) > 0 {
		badges := make(map[string]string, len(m.drillInRiskyFiles))
		for f, level := range m.drillInRiskyFiles {
			if level == catalog.RiskHigh {
				badges[f] = "!!"
			} else {
				badges[f] = "!"
			}
		}
		m.reviewDrillTree.nodeBadges = badges
	}

	// Set initial highlight lines for the primary file
	if m.reviewDrillPreview.fileName != "" && len(m.drillInItemRisks) > 0 {
		highlights := make(map[int]bool)
		firstLine := 0
		for _, r := range m.drillInItemRisks {
			for _, rl := range r.Lines {
				if rl.File == m.reviewDrillPreview.fileName {
					highlights[rl.Line] = true
					if firstLine == 0 || rl.Line < firstLine {
						firstLine = rl.Line
					}
				}
			}
		}
		if len(highlights) > 0 {
			m.reviewDrillPreview.SetHighlightLines(highlights)
			if firstLine > 0 {
				m.reviewDrillPreview.offset = max(0, firstLine-3)
			}
		}
	}

	// Pre-size the panes so scroll works before the first View()
	innerW := m.width - borderSize
	treeW := max(18, innerW*30/100)
	previewW := innerW - treeW - 1
	paneH := max(5, m.height-7)
	m.reviewDrillTree.SetSize(treeW, paneH)
	m.reviewDrillPreview.SetSize(previewW, paneH)

	m.reviewDrillIn = true
}

// exitReviewDrillIn closes the drill-in sub-view and returns to the review list.
func (m *addWizardModel) exitReviewDrillIn() {
	m.reviewDrillIn = false
	m.reviewZone = addReviewZoneItems // return focus to the item list, not buttons
}

// loadDrillInFile loads the file selected in the drill-in tree into the preview.
func (m *addWizardModel) loadDrillInFile() {
	selected := m.selectedItems()
	if m.reviewItemCursor >= len(selected) {
		return
	}
	item := selected[m.reviewItemCursor]

	// Use catalogItem.Path when available (authoritative), fall back to discovery fields
	var basePath string
	if item.catalogItem != nil {
		basePath = item.catalogItem.Path
	} else {
		basePath = item.path
		if item.sourceDir != "" {
			basePath = item.sourceDir
		}
		// Ensure basePath is a directory for ReadFileContent
		if info, err := os.Stat(basePath); err == nil && !info.IsDir() {
			basePath = filepath.Dir(basePath)
		}
	}

	relPath := m.reviewDrillTree.SelectedPath()
	if relPath == "" {
		return
	}

	content, err := catalog.ReadFileContent(basePath, relPath, 10000)
	if err != nil {
		m.reviewDrillPreview.lines = []string{"Error: " + err.Error()}
		m.reviewDrillPreview.fileName = relPath
		m.reviewDrillPreview.offset = 0
		m.reviewDrillPreview.SetHighlightLines(nil)
		return
	}
	m.reviewDrillPreview.lines = strings.Split(content, "\n")
	m.reviewDrillPreview.fileName = relPath
	m.reviewDrillPreview.offset = 0

	// Check if this file has any risk lines and set highlights
	highlights := make(map[int]bool)
	firstLine := 0
	for _, r := range m.drillInItemRisks {
		for _, rl := range r.Lines {
			if rl.File == relPath {
				highlights[rl.Line] = true
				if firstLine == 0 || rl.Line < firstLine {
					firstLine = rl.Line
				}
			}
		}
	}
	if len(highlights) > 0 {
		m.reviewDrillPreview.SetHighlightLines(highlights)
		// Scroll to center on the first highlighted line
		if firstLine > 0 {
			m.reviewDrillPreview.offset = max(0, firstLine-3)
		}
	} else {
		m.reviewDrillPreview.SetHighlightLines(nil)
	}
}

// enterExecute transitions to the Execute step.
func (m *addWizardModel) enterExecute() {
	m.step = addStepExecute
	m.shell.SetActive(m.shellIndexForStep(addStepExecute))

	selected := m.selectedItems()
	m.executeResults = make([]addExecResult, len(selected))
	m.executeCurrent = 0
	m.executeDone = false
	m.executeCancelled = false
	m.executing = true
	m.updateMaxStep()
}

// startDiscoveryCmd creates an async tea.Cmd to discover content from the selected source.
func (m *addWizardModel) startDiscoveryCmd() tea.Cmd {
	seq := m.seq
	source := m.source
	types := m.selectedTypes()
	contentRoot := m.contentRoot

	switch source {
	case addSourceProvider:
		if m.providerCursor >= len(m.providers) {
			return nil
		}
		prov := m.providers[m.providerCursor]
		projectRoot := m.projectRoot
		cfg := m.cfg
		return func() tea.Msg {
			items, err := discoverFromProvider(prov, projectRoot, cfg, contentRoot, types)
			return addDiscoveryDoneMsg{seq: seq, items: items, err: err}
		}

	case addSourceRegistry:
		if m.registryCursor >= len(m.registries) {
			return nil
		}
		reg := m.registries[m.registryCursor]
		return func() tea.Msg {
			items, err := discoverFromRegistry(reg, types, contentRoot)
			return addDiscoveryDoneMsg{seq: seq, items: items, err: err, sourceRegistry: reg.Name}
		}

	case addSourceLocal:
		dir := m.pathInput
		return func() tea.Msg {
			items, err := discoverFromLocalPath(dir, types, contentRoot)
			return addDiscoveryDoneMsg{seq: seq, items: items, err: err}
		}

	case addSourceGit:
		url := m.pathInput
		return func() tea.Msg {
			items, tmpDir, err := discoverFromGitURL(url, types, contentRoot)
			return addDiscoveryDoneMsg{seq: seq, items: items, err: err, tmpDir: tmpDir}
		}
	}
	return nil
}

// discoveryColLayout holds column widths for the discovery table.
type discoveryColLayout struct {
	name   int
	ctype  int
	status int
	risk   int
}

// discoveryColumns computes column widths for the discovery list.
// Name column is sized to the longest item name (capped at 50% of available width).
func (m *addWizardModel) discoveryColumns() discoveryColLayout {
	w := m.width - 12 // prefix (6: "> [x] ") + right padding (4) + gaps (2)
	ctype := 8
	status := 12
	risk := 6
	fixed := ctype + status + risk + 3 // 3 gaps

	// Find longest name in visible items
	maxName := 4 // minimum for "Name" header
	for _, d := range m.visibleDiscoveryItems() {
		name := d.displayName
		if name == "" {
			name = d.name
		}
		if len(name) > maxName {
			maxName = len(name)
		}
	}
	// Also check selected items (for review reuse)
	for _, d := range m.selectedItems() {
		name := d.displayName
		if name == "" {
			name = d.name
		}
		if len(name) > maxName {
			maxName = len(name)
		}
	}

	// Cap at 50% of available width, minimum 12
	maxAvail := max(12, (w-fixed)*50/100)
	nameW := max(12, min(maxName+2, maxAvail)) // +2 for breathing room
	return discoveryColLayout{nameW, ctype, status, risk}
}

// discoveryHeader renders the column header for the discovery table.
func (m *addWizardModel) discoveryHeader() string {
	cols := m.discoveryColumns()
	prefix := "      " // matches checkbox row prefix width ("> [x] ")
	row := prefix +
		boldStyle.Render(padRight("Name", cols.name)) + " " +
		boldStyle.Render(padRight("Risk", cols.risk)) + " " +
		boldStyle.Render(padRight("Type", cols.ctype)) + " " +
		boldStyle.Render(padRight("Status", cols.status))
	return truncateLine(row, m.width)
}

// toggleInstalled toggles the showInstalled flag and rebuilds the discovery list,
// preserving selection state for the actionable items.
func (m *addWizardModel) toggleInstalled() {
	// Save current selections (indices into visible items)
	oldVisible := m.visibleDiscoveryItems()
	oldSelected := make(map[int]bool)
	for _, idx := range m.discoveryList.SelectedIndices() {
		if idx < len(oldVisible) {
			// Map to discoveredItems index
			oldSelected[idx] = true
		}
	}

	m.showInstalled = !m.showInstalled
	m.discoveryList = m.buildDiscoveryList()

	// Restore selections — actionable items keep the same indices (they're first)
	for idx := range oldSelected {
		if idx < len(m.discoveryList.selected) {
			m.discoveryList.selected[idx] = true
		}
	}
}

// rebuildDiscoveryListPreserveSelection rebuilds the discovery list (e.g., after resize)
// while preserving the current selection and cursor state.
func (m *addWizardModel) rebuildDiscoveryListPreserveSelection() {
	oldSelected := make(map[int]bool)
	for _, idx := range m.discoveryList.SelectedIndices() {
		oldSelected[idx] = true
	}
	oldCursor := m.discoveryList.cursor
	oldOffset := m.discoveryList.offset

	m.discoveryList = m.buildDiscoveryList()

	// Restore selections
	for idx := range oldSelected {
		if idx < len(m.discoveryList.selected) {
			m.discoveryList.selected[idx] = true
		}
	}
	// Restore cursor/offset
	if oldCursor < len(m.discoveryList.items) {
		m.discoveryList.cursor = oldCursor
	}
	m.discoveryList.offset = oldOffset
}

// visibleDiscoveryItems returns the items visible based on the showInstalled toggle.
func (m *addWizardModel) visibleDiscoveryItems() []addDiscoveryItem {
	if m.showInstalled || m.installedCount == 0 {
		return m.discoveredItems
	}
	return m.discoveredItems[:m.actionableCount]
}

// buildDiscoveryList creates a checkboxList from discovered items.
func (m *addWizardModel) buildDiscoveryList() checkboxList {
	visible := m.visibleDiscoveryItems()
	cols := m.discoveryColumns()
	items := make([]checkboxItem, len(visible))
	for i, d := range visible {
		name := d.displayName
		if name == "" {
			name = d.name
		}

		typeLbl := typeLabel(d.itemType)

		var statusLbl string
		var statusBadge checkboxBadgeStyle
		switch d.status {
		case add.StatusNew:
			statusLbl = "new"
			statusBadge = badgeStyleSuccess
		case add.StatusInLibrary:
			statusLbl = "in library"
			statusBadge = badgeStyleMuted
		case add.StatusOutdated:
			statusLbl = "outdated"
			statusBadge = badgeStyleWarning
		}

		var riskLbl string
		if len(d.risks) > 0 {
			hasHigh := false
			for _, r := range d.risks {
				if r.Level == catalog.RiskHigh {
					hasHigh = true
					break
				}
			}
			if hasHigh {
				riskLbl = "!!"
				statusBadge = badgeStyleDanger
			} else {
				riskLbl = "!"
				if statusBadge != badgeStyleWarning {
					statusBadge = badgeStyleWarning
				}
			}
		}

		// Build fixed-width columnar label
		label := padRight(truncate(sanitizeLine(name), cols.name), cols.name) + " " +
			padRight(riskLbl, cols.risk) + " " +
			padRight(truncate(typeLbl, cols.ctype), cols.ctype) + " " +
			padRight(truncate(statusLbl, cols.status), cols.status)

		items[i] = checkboxItem{
			label:      label,
			badge:      "",
			badgeStyle: statusBadge,
		}
	}

	cl := newCheckboxList(items)
	cl.focused = true
	cl.zonePrefix = "add-disc"

	// Pre-select: New and Outdated checked, InLibrary unchecked
	for i, d := range visible {
		switch d.status {
		case add.StatusNew, add.StatusOutdated:
			cl.selected[i] = true
		}
		// Auto-set overwrite for outdated items
		if d.status == add.StatusOutdated {
			// Find the actual index in discoveredItems
			m.discoveredItems[i].overwrite = true
		}
	}

	// Apply height constraint so the list scrolls instead of overflowing
	cl = cl.SetSize(m.width-4, m.discoveryListHeight())

	return cl
}

// listHeight returns the available height for checkbox lists inside the wizard.
// The wizard shell is 3 lines. The title row is 1 line. The blank line after
// title is 1. The column header (discovery) is 1. The installed toggle section
// is ~2 lines. A small padding buffer rounds it out. For the type step (no
// column header or toggle), overhead is smaller.
func (m *addWizardModel) discoveryListHeight() int {
	// shell(3) + title(1) + blank(1) + colHeader(1) + toggle(2) + padding(1) = 9
	h := m.height - 9
	if h < 3 {
		h = 3
	}
	return h
}

func (m *addWizardModel) typeListHeight() int {
	// shell(3) + title(1) + blank(1) + padding(1) = 6
	h := m.height - 6
	if h < 3 {
		h = 3
	}
	return h
}

// reviewRiskBoxLines is the fixed number of lines reserved for the risk box area.
// Always reserved regardless of whether the current item has risks, to prevent
// the item list from jumping when scrolling between items with/without risks.
const reviewRiskBoxLines = 4 // border(1) + up to 2 risk lines + border(1)

// reviewVisibleHeight returns the number of review items that fit in the
// visible area. Uses a fixed overhead so the list doesn't jump.
func (m *addWizardModel) reviewVisibleHeight() int {
	// shell(3) + header(1) + riskBox(4) + microcopy(1) + colHeader(1) = 10
	overhead := 3 + 1 + reviewRiskBoxLines + 1 + 1
	h := m.height - overhead
	if h < 3 {
		h = 3
	}
	return h
}

// adjustReviewOffset ensures the review item cursor stays within the visible window.
func (m *addWizardModel) adjustReviewOffset() {
	vh := m.reviewVisibleHeight()
	if m.reviewItemCursor < m.reviewItemOffset {
		m.reviewItemOffset = m.reviewItemCursor
	}
	if m.reviewItemCursor >= m.reviewItemOffset+vh {
		m.reviewItemOffset = m.reviewItemCursor - vh + 1
	}
}

// addItemCmd creates an async tea.Cmd for adding a single item.
func (m *addWizardModel) addItemCmd(index int) tea.Cmd {
	seq := m.seq
	items := m.selectedItems()
	if index >= len(items) {
		return nil
	}
	item := items[index]
	contentRoot := m.contentRoot
	sourceReg := m.sourceRegistry
	sourceVis := m.sourceVisibility
	source := m.source
	provSlug := ""
	if source == addSourceProvider && m.providerCursor < len(m.providers) {
		provSlug = m.providers[m.providerCursor].Slug
	}

	return func() tea.Msg {
		result := addSingleItem(item, contentRoot, sourceReg, sourceVis, provSlug)
		return addExecItemDoneMsg{seq: seq, index: index, result: result}
	}
}

// nextPending returns the next pending execute index, or -1 if none.
func (m *addWizardModel) nextPending() int {
	for i := m.executeCurrent; i < len(m.executeResults); i++ {
		if m.executeResults[i].status == "" {
			return i
		}
	}
	return -1
}

// --- Discovery backends ---

func discoverFromProvider(
	prov provider.Provider,
	projectRoot string,
	cfg *config.Config,
	contentRoot string,
	selectedTypes []catalog.ContentType,
) ([]addDiscoveryItem, error) {
	// Build resolver from config
	var resolver *config.PathResolver
	if cfg != nil {
		globalCfg, _ := config.LoadGlobal()
		projectCfg, _ := config.Load(projectRoot)
		merged := config.Merge(globalCfg, projectCfg)
		resolver = config.NewResolver(merged, "")
		_ = resolver.ExpandPaths() // non-fatal
	}

	// Build type filter set
	typeSet := make(map[catalog.ContentType]bool)
	for _, t := range selectedTypes {
		typeSet[t] = true
	}

	// File-based discovery (rules, skills, agents, commands)
	discovered, err := add.DiscoverFromProvider(prov, projectRoot, resolver, contentRoot)
	if err != nil {
		return nil, err
	}

	var items []addDiscoveryItem
	for _, d := range discovered {
		if !typeSet[d.Type] {
			continue
		}

		item := addDiscoveryItem{
			name:       d.Name,
			itemType:   d.Type,
			path:       d.Path,
			sourceDir:  d.SourceDir,
			status:     d.Status,
			scope:      d.Scope,
			underlying: &d,
		}

		// Build a ContentItem for risk scanning and drill-in preview
		ci := catalog.ContentItem{
			Name: d.Name,
			Type: d.Type,
			Path: d.Path,
		}
		if d.SourceDir != "" {
			ci.Path = d.SourceDir
		}
		ci.Files = scanDrillInFiles(ci.Path)
		item.risks = catalog.RiskIndicators(ci)
		item.catalogItem = &ci

		items = append(items, item)
	}

	// Hooks discovery (JSON merge — separate from file-based)
	if typeSet[catalog.Hooks] {
		hookItems := discoverHooksFromProvider(prov, projectRoot, resolver, contentRoot)
		items = append(items, hookItems...)
	}

	// MCP discovery (JSON merge — separate from file-based)
	if typeSet[catalog.MCP] {
		mcpItems := discoverMcpFromProvider(prov, projectRoot, resolver, contentRoot)
		items = append(items, mcpItems...)
	}

	return items, nil
}

// discoverHooksFromProvider reads provider settings files and extracts
// individual hook entries, annotated with library status.
func discoverHooksFromProvider(prov provider.Provider, projectRoot string, resolver *config.PathResolver, contentRoot string) []addDiscoveryItem {
	baseDir := ""
	if resolver != nil {
		baseDir = resolver.BaseDir(prov.Slug)
	}
	locations, err := installer.FindSettingsLocationsWithBase(prov, projectRoot, baseDir)
	if err != nil {
		return nil
	}

	idx, err := add.BuildLibraryIndex(contentRoot)
	if err != nil {
		return nil
	}

	var result []addDiscoveryItem
	for _, loc := range locations {
		data, err := os.ReadFile(loc.Path)
		if err != nil {
			continue
		}
		hooks, err := converter.SplitSettingsHooks(data, prov.Slug)
		if err != nil {
			continue
		}
		for _, hook := range hooks {
			name := converter.DeriveHookName(hook)

			key := string(catalog.Hooks) + "/" + prov.Slug + "/" + name
			_, inLib := idx[key]
			status := add.StatusNew
			if inLib {
				status = add.StatusInLibrary
			}

			di := add.DiscoveryItem{
				Name:   name,
				Type:   catalog.Hooks,
				Path:   loc.Path,
				Status: status,
				Scope:  loc.Scope.String(),
			}

			// Compute risk from the hook's structured data
			var hookRisks []catalog.RiskIndicator
			for _, entry := range hook.Hooks {
				if entry.Command != "" {
					hookRisks = append(hookRisks, catalog.RiskIndicator{
						Label:       "Runs commands",
						Description: "Hook executes: " + truncate(entry.Command, 60),
						Level:       catalog.RiskHigh,
					})
					break // one "Runs commands" indicator is enough
				}
				if entry.URL != "" {
					hookRisks = append(hookRisks, catalog.RiskIndicator{
						Label:       "Network access",
						Description: "Hook calls: " + truncate(entry.URL, 60),
						Level:       catalog.RiskMedium,
					})
					break
				}
			}

			result = append(result, addDiscoveryItem{
				name:       name,
				itemType:   catalog.Hooks,
				path:       loc.Path,
				status:     status,
				scope:      loc.Scope.String(),
				risks:      hookRisks,
				underlying: &di,
			})
		}
	}
	return result
}

// discoverMcpFromProvider reads provider MCP config files and extracts
// individual server entries, annotated with library status.
func discoverMcpFromProvider(prov provider.Provider, projectRoot string, resolver *config.PathResolver, contentRoot string) []addDiscoveryItem {
	baseDir := ""
	if resolver != nil {
		baseDir = resolver.BaseDir(prov.Slug)
	}
	locations := installer.FindMCPLocations(prov, projectRoot, baseDir)

	idx, err := add.BuildLibraryIndex(contentRoot)
	if err != nil {
		return nil
	}

	var result []addDiscoveryItem
	for _, loc := range locations {
		data, err := os.ReadFile(loc.Path)
		if err != nil {
			continue
		}
		if prov.Slug == "opencode" {
			data = converter.StripJSONCComments(data)
		}

		servers := gjson.GetBytes(data, loc.JSONKey)
		if !servers.Exists() || servers.Type != gjson.JSON {
			continue
		}
		servers.ForEach(func(key, val gjson.Result) bool {
			name := key.String()
			libKey := string(catalog.MCP) + "/" + prov.Slug + "/" + name
			_, inLib := idx[libKey]
			status := add.StatusNew
			if inLib {
				status = add.StatusInLibrary
			}
			di := add.DiscoveryItem{
				Name:   name,
				Type:   catalog.MCP,
				Path:   loc.Path,
				Status: status,
				Scope:  loc.Scope.String(),
			}

			// Compute risk from actual server config
			var mcpRisks []catalog.RiskIndicator
			cmd := val.Get("command").String()
			url := val.Get("url").String()
			transport := val.Get("type").String() // "stdio", "sse", "http"
			hasEnv := val.Get("env").Exists() && len(val.Get("env").Map()) > 0

			if cmd != "" {
				mcpRisks = append(mcpRisks, catalog.RiskIndicator{
					Label:       "Runs process",
					Description: "Launches: " + truncate(cmd, 60),
					Level:       catalog.RiskHigh,
				})
			}
			if url != "" || transport == "sse" || transport == "http" {
				mcpRisks = append(mcpRisks, catalog.RiskIndicator{
					Label:       "Network access",
					Description: "Connects to remote endpoint",
					Level:       catalog.RiskMedium,
				})
			}
			if hasEnv {
				mcpRisks = append(mcpRisks, catalog.RiskIndicator{
					Label:       "Environment variables",
					Description: "Server receives environment variables",
					Level:       catalog.RiskMedium,
				})
			}

			result = append(result, addDiscoveryItem{
				name:       name,
				itemType:   catalog.MCP,
				path:       loc.Path,
				status:     status,
				scope:      loc.Scope.String(),
				risks:      mcpRisks,
				underlying: &di,
			})
			return true
		})
	}
	return result
}

func discoverFromRegistry(
	reg catalog.RegistrySource,
	selectedTypes []catalog.ContentType,
	contentRoot string,
) ([]addDiscoveryItem, error) {
	cats, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{reg})
	if err != nil {
		return nil, err
	}

	typeSet := make(map[catalog.ContentType]bool)
	for _, t := range selectedTypes {
		typeSet[t] = true
	}

	// Build library index for status comparison
	idx, _ := add.BuildLibraryIndex(contentRoot)

	var items []addDiscoveryItem
	for _, ci := range cats.Items {
		if !typeSet[ci.Type] {
			continue
		}

		status := add.StatusNew
		key := string(ci.Type) + "/" + ci.Name
		if _, exists := idx[key]; exists {
			status = add.StatusInLibrary
		}

		// Path must point to the primary content file (not the directory)
		// for add.writeItem to read it. SourceDir is the item directory.
		primaryFile := catalog.PrimaryFileName(ci.Files, ci.Type)
		contentPath := ci.Path
		sourceDir := ""
		if primaryFile != "" {
			contentPath = filepath.Join(ci.Path, primaryFile)
			sourceDir = ci.Path
		}

		di := add.DiscoveryItem{
			Name:      ci.Name,
			Type:      ci.Type,
			Path:      contentPath,
			SourceDir: sourceDir,
			Status:    status,
		}
		ciCopy := ci // capture for pointer
		item := addDiscoveryItem{
			name:        ci.Name,
			displayName: ci.DisplayName,
			itemType:    ci.Type,
			path:        contentPath,
			sourceDir:   sourceDir,
			status:      status,
			risks:       catalog.RiskIndicators(ci),
			underlying:  &di,
			catalogItem: &ciCopy,
		}
		items = append(items, item)
	}

	return items, nil
}

func discoverFromLocalPath(
	dir string,
	selectedTypes []catalog.ContentType,
	contentRoot string,
) ([]addDiscoveryItem, error) {
	typeSet := make(map[catalog.ContentType]bool)
	for _, t := range selectedTypes {
		typeSet[t] = true
	}
	idx, _ := add.BuildLibraryIndex(contentRoot)

	// First try syllago structure (registry or content type dirs)
	nativeResult := catalog.ScanNativeContent(dir)
	var items []addDiscoveryItem

	if nativeResult.HasSyllagoStructure {
		cat, err := catalog.Scan(dir, dir)
		if err != nil {
			return nil, err
		}
		items = catalogItemsToDiscovery(cat.Items, typeSet, idx)
	}

	// If syllago scan found nothing, try provider-native patterns
	// (e.g., .claude/rules/, .claude/skills/, .gemini/rules/, etc.)
	if len(items) == 0 && len(nativeResult.Providers) > 0 {
		items = nativeItemsToDiscovery(dir, nativeResult, typeSet, idx)
	}

	// Last resort: try syllago scan anyway (might find something)
	if len(items) == 0 && !nativeResult.HasSyllagoStructure {
		cat, err := catalog.Scan(dir, dir)
		if err == nil && len(cat.Items) > 0 {
			items = catalogItemsToDiscovery(cat.Items, typeSet, idx)
		}
	}

	return items, nil
}

// nativeItemsToDiscovery converts NativeScanResult provider items into addDiscoveryItems.
func nativeItemsToDiscovery(
	baseDir string,
	result catalog.NativeScanResult,
	typeSet map[catalog.ContentType]bool,
	idx add.LibraryIndex,
) []addDiscoveryItem {
	// Map native type labels to catalog content types
	typeLabelMap := map[string]catalog.ContentType{
		"rules":    catalog.Rules,
		"skills":   catalog.Skills,
		"agents":   catalog.Agents,
		"commands": catalog.Commands,
		"hooks":    catalog.Hooks,
		"mcp":      catalog.MCP,
	}

	var items []addDiscoveryItem
	for _, pc := range result.Providers {
		for typeLabel, nativeItems := range pc.Items {
			ct, ok := typeLabelMap[typeLabel]
			if !ok || !typeSet[ct] {
				continue
			}
			for _, ni := range nativeItems {
				fullPath := filepath.Join(baseDir, ni.Path)

				status := add.StatusNew
				key := string(ct) + "/" + ni.Name
				if _, exists := idx[key]; exists {
					status = add.StatusInLibrary
				}

				di := add.DiscoveryItem{
					Name:   ni.Name,
					Type:   ct,
					Path:   fullPath,
					Status: status,
					Scope:  pc.ProviderSlug,
				}

				// Check if path is a directory or file
				info, err := os.Stat(fullPath)
				if err != nil {
					continue
				}
				if info.IsDir() {
					di.SourceDir = fullPath
					// Find primary file
					files := scanDrillInFiles(fullPath)
					primary := catalog.PrimaryFileName(files, ct)
					if primary != "" {
						di.Path = filepath.Join(fullPath, primary)
					}
				}

				item := addDiscoveryItem{
					name:        ni.Name,
					displayName: ni.DisplayName,
					itemType:    ct,
					path:        di.Path,
					sourceDir:   di.SourceDir,
					status:      status,
					scope:       pc.ProviderSlug,
					underlying:  &di,
				}

				// Build catalogItem for drill-in
				ci := catalog.ContentItem{
					Name:        ni.Name,
					DisplayName: ni.DisplayName,
					Type:        ct,
					Path:        fullPath,
				}
				if info.IsDir() {
					ci.Files = scanDrillInFiles(fullPath)
				} else {
					ci.Path = filepath.Dir(fullPath)
					ci.Files = []string{filepath.Base(fullPath)}
				}
				item.catalogItem = &ci
				item.risks = catalog.RiskIndicators(ci)

				items = append(items, item)
			}
		}
	}
	return items
}

// catalogItemsToDiscovery converts catalog.ContentItems into addDiscoveryItems.
func catalogItemsToDiscovery(
	catItems []catalog.ContentItem,
	typeSet map[catalog.ContentType]bool,
	idx add.LibraryIndex,
) []addDiscoveryItem {
	var items []addDiscoveryItem
	for _, ci := range catItems {
		if !typeSet[ci.Type] {
			continue
		}

		status := add.StatusNew
		key := string(ci.Type) + "/" + ci.Name
		if _, exists := idx[key]; exists {
			status = add.StatusInLibrary
		}

		primaryFile := catalog.PrimaryFileName(ci.Files, ci.Type)
		contentPath := ci.Path
		sourceDir := ""
		if primaryFile != "" {
			contentPath = filepath.Join(ci.Path, primaryFile)
			sourceDir = ci.Path
		}

		di := add.DiscoveryItem{
			Name:      ci.Name,
			Type:      ci.Type,
			Path:      contentPath,
			SourceDir: sourceDir,
			Status:    status,
		}
		ciCopy := ci
		item := addDiscoveryItem{
			name:        ci.Name,
			displayName: ci.DisplayName,
			itemType:    ci.Type,
			path:        contentPath,
			sourceDir:   sourceDir,
			status:      status,
			risks:       catalog.RiskIndicators(ci),
			underlying:  &di,
			catalogItem: &ciCopy,
		}
		items = append(items, item)
	}

	return items
}

func discoverFromGitURL(
	url string,
	selectedTypes []catalog.ContentType,
	contentRoot string,
) ([]addDiscoveryItem, string, error) {
	tmpDir, err := cloneGitURL(url, 60)
	if err != nil {
		return nil, "", err
	}
	items, err := discoverFromLocalPath(tmpDir, selectedTypes, contentRoot)
	if err != nil {
		_ = os.RemoveAll(filepath.Dir(tmpDir))
		return nil, "", err
	}
	return items, tmpDir, nil
}

// addSingleItem adds a single item to the library.
func addSingleItem(item addDiscoveryItem, contentRoot, srcReg, srcVis, provSlug string) addExecResult {
	if item.underlying == nil {
		return addExecResult{
			name:   item.name,
			status: "error",
			err:    fmt.Errorf("internal: nil underlying for type %s", item.itemType),
		}
	}

	opts := add.AddOptions{
		Force:            item.overwrite,
		Provider:         provSlug,
		SourceRegistry:   srcReg,
		SourceVisibility: srcVis,
	}

	results := add.AddItems([]add.DiscoveryItem{*item.underlying}, opts, contentRoot, nil, "syllago")
	if len(results) == 0 {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("no result")}
	}

	r := results[0]
	switch r.Status {
	case add.AddStatusAdded:
		return addExecResult{name: item.name, status: "added"}
	case add.AddStatusUpdated:
		return addExecResult{name: item.name, status: "updated"}
	case add.AddStatusUpToDate:
		return addExecResult{name: item.name, status: "skipped"}
	case add.AddStatusSkipped:
		return addExecResult{name: item.name, status: "skipped"}
	case add.AddStatusError:
		return addExecResult{name: item.name, status: "error", err: r.Error}
	default:
		return addExecResult{name: item.name, status: "added"}
	}
}

// --- Git URL validation ---

// validGitURL returns true if the URL looks like a valid git remote.
// Rejects ext::, fd::, file:// protocols for security.
func validGitURL(url string) bool {
	if url == "" {
		return false
	}
	lower := strings.ToLower(url)
	if strings.HasPrefix(lower, "ext::") || strings.HasPrefix(lower, "fd::") || strings.HasPrefix(lower, "file://") {
		return false
	}
	return strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "ssh://") ||
		strings.HasPrefix(lower, "git@")
}

// scanDrillInFiles collects visible files from a path for the drill-in preview.
// For files it returns the basename. For directories it walks and collects relative paths.
func scanDrillInFiles(path string) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{filepath.Base(path)}
	}
	var files []string
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(path, p)
		if rel != "" {
			files = append(files, rel)
		}
		return nil
	})
	return files
}

// cloneGitURL performs a hardened git clone with security restrictions.
// Returns the path to the cloned repo directory. The caller must clean up
// via os.RemoveAll(filepath.Dir(repoDir)) to remove the parent temp dir.
// extractJSONSection reads a JSON settings file and extracts just the relevant
// section for a hook or MCP server by name. Returns pretty-printed JSON of
// just that item, or "" if not found.
func extractJSONSection(filePath, name string, itemType catalog.ContentType) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	switch itemType {
	case catalog.MCP:
		// Try common MCP config paths
		for _, path := range []string{
			"mcpServers." + name,
			"mcp.mcpServers." + name,
		} {
			result := gjson.GetBytes(data, path)
			if result.Exists() {
				// Pretty-print the server config as {"name": {config}}
				return "{\n  " + fmt.Sprintf("%q", name) + ": " + result.Raw + "\n}"
			}
		}

	case catalog.Hooks:
		// Search through hook events to find entries matching this name
		hooksResult := gjson.GetBytes(data, "hooks")
		if !hooksResult.Exists() {
			return ""
		}
		var matched []string
		hooksResult.ForEach(func(event, entries gjson.Result) bool {
			entries.ForEach(func(_, entry gjson.Result) bool {
				derived := converter.DeriveHookName(converter.HookData{
					Event: event.String(),
					Hooks: []converter.HookEntry{{
						Command: entry.Get("command").String(),
						URL:     entry.Get("url").String(),
					}},
				})
				if derived == name {
					matched = append(matched, fmt.Sprintf("  // Event: %s\n  %s", event.String(), entry.Raw))
				}
				return true
			})
			return true
		})
		if len(matched) > 0 {
			return "{\n" + strings.Join(matched, ",\n") + "\n}"
		}
	}

	return ""
}

func cloneGitURL(url string, timeoutSec int) (string, error) {
	parentDir, err := os.MkdirTemp("", "syllago-add-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	repoDir := filepath.Join(parentDir, "repo")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone",
		"-c", "core.hooksPath=/dev/null",
		"--no-recurse-submodules",
		"--depth", "1",
		url, repoDir,
	)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")

	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(parentDir)
		return "", fmt.Errorf("git clone: %w", err)
	}
	return repoDir, nil
}
