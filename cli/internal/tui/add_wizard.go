package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- Step enum ---

type addStep int

const (
	addStepSource addStep = iota
	addStepType
	addStepDiscovery
	addStepHeuristic
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
	// addSourceMonolithic discovers monolithic rule files (CLAUDE.md,
	// AGENTS.md, GEMINI.md, .cursorrules, .clinerules, .windsurfrules)
	// under the project root + home directory and lets the user multi-select
	// which files to split/import as library rules (D2, D18).
	addSourceMonolithic
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
	path        string         // primary file path (relative to source root)
	sourceDir   string         // absolute directory containing the item
	status      add.ItemStatus // original item status (preserves StatusOutdated for conflict detection)
	underlying  *add.DiscoveryItem
	catalogItem *catalog.ContentItem // original catalog item when available

	// Risk indicators pre-computed during discovery. For hooks these come from
	// the hook command/URL analysis in hooksFromSettingsFile, not file content.
	risks []catalog.RiskIndicator

	// Hook-specific: mirrors addDiscoveryItem fields so triage preview and
	// mergeConfirmIntoDiscovery can reconstruct a full hook discovery item.
	hookData      *converter.HookData
	hookSourceDir string

	// Splittable mirrors the addDiscoveryItem field so the Review step (after
	// mergeConfirmIntoDiscovery) can still render the monolithic-rule hint.
	splittable        bool
	splitSectionCount int
	splitChosen       bool
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
	description string // optional, set via review-step [e] rename modal
	itemType    catalog.ContentType
	path        string
	sourceDir   string
	status      add.ItemStatus
	scope       string
	risks       []catalog.RiskIndicator
	overwrite   bool
	underlying  *add.DiscoveryItem
	catalogItem *catalog.ContentItem // original catalog item when available (has correct Files/Path)

	// Hook-specific: populated by discoverHooksFromProvider so addSingleItem
	// can write a proper flat hook.json instead of copying settings.json.
	hookData      *converter.HookData
	hookSourceDir string // directory of the settings.json, used to resolve relative script paths

	confidence      float64
	detectionSource string
	tier            analyzer.ConfidenceTier

	// Splittable marks a rule whose source file is a recognized monolithic
	// format (CLAUDE.md, AGENTS.md, GEMINI.md, .cursorrules, .clinerules,
	// .windsurfrules) and passes the H2 splitter pre-check. splitSectionCount
	// is the number of rules the default H2 heuristic would produce.
	// Populated in handleDiscoveryDone after provider/local discovery.
	splittable        bool
	splitSectionCount int

	// splitChosen records the user's Heuristic-step decision for this rule:
	// true = split into sections via H2, false = import as a single rule.
	// Default true for splittable items, false otherwise. Only meaningful
	// when splittable=true.
	splitChosen bool
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
	installedCount  int  // number of installed items (trailing block of discoveredItems)
	actionableCount int  // number of actionable items (New/Outdated); grows when triage merges

	// Pre-merge baselines. Captured once in handleDiscoveryDone and used by
	// mergeConfirmIntoDiscovery to re-derive the layout idempotently across
	// Back→Triage→Next cycles.
	preMergeActionableCount int
	preMergeInstalledCount  int

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
	// drillButtonCursor tracks keyboard focus for the Back/Rename/Next
	// button row in drill-in. -1 means focus is on the tree/preview panes;
	// 0/1/2 select Back/Rename/Next respectively. Tab cycles panes → Back →
	// Rename → Next → panes.
	drillButtonCursor int

	// Review rename modal — edits display name + description of the selected
	// review item in memory. The on-disk identity (directory name) never changes.
	renameModal        editModal
	renameItemCursor   int // index into selectedItems() the modal is editing
	renameDiscoveryIdx int // index into m.discoveredItems the modal is editing

	// Execute step
	executeResults   []addExecResult
	executeCurrent   int
	executeOffset    int
	executeDone      bool
	executeCancelled bool
	executing        bool

	// Git source
	gitTempDir string

	// Discovery step — triage-style list of all detected items (auto pre-selected, needs-review unchecked)
	confirmItems    []addConfirmItem
	confirmSelected map[int]bool
	confirmCursor   int
	confirmOffset   int
	confirmPreview  previewModel
	confirmFocus    triageFocusZone

	// Monolithic rule discovery path (D2, D18). Populated when the user
	// chooses addSourceMonolithic on the Source step. discoveryCandidates is
	// the full list surfaced on the Discovery step (one row per discovered
	// file); selectedCandidates holds the indices the user multi-selected
	// with space. chosenHeuristic + markerLiteral are set on the Heuristic
	// step; reviewCandidates is produced by splitting each selected source
	// file and is rendered on the Review step; reviewAccepted tracks which
	// review rows the user keeps (default include-all) and reviewRenames is
	// a per-candidate slug override.
	discoveryCandidates     []monolithicCandidate
	selectedCandidates      []int
	discoveryCandidateCurs  int
	chosenHeuristic         int // splitter.Heuristic value; kept untyped here to avoid an import-cycle footprint
	heuristicCursor         int
	markerLiteral           string
	reviewCandidates        []reviewCandidate
	reviewAccepted          []bool
	reviewRenames           []string
	reviewCandidateCursor   int
	executeMonolithicResult []addExecResult

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
		buttonCursor:  2, // default to [Back] (0=Add, 1=Rename, 2=Back, 3=Cancel)
		renameModal:   newEditModal(),
	}
	m.shell = newWizardShell("Add", m.buildShellLabels())

	return m
}

// Init satisfies the tea.Model interface.
func (m *addWizardModel) Init() tea.Cmd { return nil }

// CapturingTextInput returns true when the wizard has a text field focused
// that should receive every keystroke verbatim (including digits 1/2/3 that
// the App otherwise hijacks for group navigation). Today that's the rename
// modal (review step) and the path/URL input on the Source step.
func (m *addWizardModel) CapturingTextInput() bool {
	if m == nil {
		return false
	}
	return m.renameModal.active || m.inputActive
}

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
		// Monolithic path skips the Type step entirely — selected-types check
		// doesn't apply. For every other source, Type step must have produced
		// at least one selected content type.
		if m.source != addSourceMonolithic && !m.discovering && len(m.selectedTypes()) == 0 {
			panic("wizard invariant: addStepDiscovery entered without selected types")
		}
	case addStepReview:
		// Monolithic path populates reviewCandidates from buildReviewCandidates
		// and does not use discoveredItems/selectedItems; the Heuristic step
		// guarantees at least one reviewCandidate.
		if m.source == addSourceMonolithic {
			if len(m.reviewCandidates) == 0 {
				panic("wizard invariant: addStepReview entered with no review candidates (monolithic)")
			}
			return
		}
		if len(m.discoveredItems) == 0 {
			panic("wizard invariant: addStepReview entered without discovered items")
		}
		if len(m.selectedItems()) == 0 {
			panic("wizard invariant: addStepReview entered without selected items")
		}
	case addStepHeuristic:
		if m.source == addSourceMonolithic {
			if len(m.selectedCandidates) == 0 {
				panic("wizard invariant: addStepHeuristic entered with no selected candidates")
			}
		} else {
			// Provider flow: must have at least one splittable selected item.
			if !m.hasSplittableSelection() {
				panic("wizard invariant: addStepHeuristic (provider flow) entered without splittable items")
			}
		}
	case addStepExecute:
		// Monolithic path: require at least one accepted review candidate.
		if m.source == addSourceMonolithic {
			any := false
			for _, acc := range m.reviewAccepted {
				if acc {
					any = true
					break
				}
			}
			if !any {
				panic("wizard invariant: addStepExecute entered with no accepted review candidates")
			}
			return
		}
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
		if len(m.confirmItems) == 0 {
			return append([]string{"esc back"}, base...)
		}
		return append([]string{
			"↑/↓ navigate", "space toggle", "a all", "n none",
			"tab switch panes", "enter next", "esc back",
		}, base...)
	case addStepReview:
		if m.reviewDrillIn {
			return append([]string{"tab/←/→ switch panes", "↑/↓ navigate", "pgup/pgdn scroll", "e rename", "esc back to review"}, base...)
		}
		return append([]string{"tab cycle zones", "↑/↓ navigate", "←/→ buttons", "enter inspect", "e rename", "esc back"}, base...)
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

// hasSplittableSelection reports whether any currently selected discovery item
// is flagged splittable=true. Drives the decision to insert the Heuristic step
// into the Provider flow after triage-merge.
func (m *addWizardModel) hasSplittableSelection() bool {
	for _, it := range m.selectedItems() {
		if it.splittable {
			return true
		}
	}
	return false
}

// buildShellLabels returns the correct step label slice for the current permutation.
func (m *addWizardModel) buildShellLabels() []string {
	if m.source == addSourceMonolithic {
		return []string{"Source", "Discovery", "Heuristic", "Review", "Execute"}
	}
	heur := m.hasSplittableSelection()
	if m.preFilterType != "" {
		if heur {
			return []string{"Source", "Discovery", "Heuristic", "Review", "Execute"}
		}
		return []string{"Source", "Discovery", "Review", "Execute"}
	}
	if heur {
		return []string{"Source", "Type", "Discovery", "Heuristic", "Review", "Execute"}
	}
	return []string{"Source", "Type", "Discovery", "Review", "Execute"}
}

// clearTriageState resets all discovery/triage state. Call from goBackFromDiscovery()
// and advanceFromSource() to prevent stale data surviving source changes.
func (m *addWizardModel) clearTriageState() {
	m.confirmItems = nil
	m.confirmSelected = nil
	m.confirmCursor = 0
	m.confirmOffset = 0
	m.confirmPreview = previewModel{}
	m.confirmFocus = triageZoneItems
	m.maxStep = addStepDiscovery
}

// shellIndexForStep maps an addStep to the wizard shell breadcrumb index.
func (m *addWizardModel) shellIndexForStep(s addStep) int {
	// Monolithic path: Source(0) Discovery(1) Heuristic(2) Review(3) Execute(4)
	if m.source == addSourceMonolithic {
		switch s {
		case addStepSource:
			return 0
		case addStepDiscovery:
			return 1
		case addStepHeuristic:
			return 2
		case addStepReview:
			return 3
		case addStepExecute:
			return 4
		}
		return 0
	}

	hasType := m.preFilterType == ""
	heur := m.hasSplittableSelection()

	// Provider-flow permutations (4 possible):
	//   hasType=true,  heur=false: Source(0) Type(1) Discovery(2) Review(3) Execute(4)
	//   hasType=true,  heur=true:  Source(0) Type(1) Discovery(2) Heuristic(3) Review(4) Execute(5)
	//   hasType=false, heur=false: Source(0) Discovery(1) Review(2) Execute(3)
	//   hasType=false, heur=true:  Source(0) Discovery(1) Heuristic(2) Review(3) Execute(4)
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
	case addStepHeuristic:
		if hasType {
			return 3
		}
		return 2
	case addStepReview:
		if heur {
			if hasType {
				return 4
			}
			return 3
		}
		if hasType {
			return 3
		}
		return 2
	case addStepExecute:
		if heur {
			if hasType {
				return 5
			}
			return 4
		}
		if hasType {
			return 4
		}
		return 3
	}
	return int(s)
}

// stepForShellIndex is the inverse of shellIndexForStep.
// Used by breadcrumb click handler to map shell index → step enum.
func (m *addWizardModel) stepForShellIndex(idx int) addStep {
	heur := m.hasSplittableSelection()
	if m.preFilterType != "" {
		if heur {
			// Source(0) Discovery(1) Heuristic(2) Review(3) Execute(4)
			switch idx {
			case 0:
				return addStepSource
			case 1:
				return addStepDiscovery
			case 2:
				return addStepHeuristic
			case 3:
				return addStepReview
			case 4:
				return addStepExecute
			}
		} else {
			// Source(0) Discovery(1) Review(2) Execute(3)
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
	} else if heur {
		// Source(0) Type(1) Discovery(2) Heuristic(3) Review(4) Execute(5)
		switch idx {
		case 0:
			return addStepSource
		case 1:
			return addStepType
		case 2:
			return addStepDiscovery
		case 3:
			return addStepHeuristic
		case 4:
			return addStepReview
		case 5:
			return addStepExecute
		}
	} else {
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

// adjustTriageOffset ensures the triage cursor stays in the visible window.
// Uses visual row position (which accounts for section headers between type groups)
// rather than raw item index, since renderTriageItems inserts non-item header rows.
func (m *addWizardModel) adjustTriageOffset() {
	vh := max(3, m.height-12) // -12: added legend line reduces visible pane by 1
	vr := triageItemVisualRow(m.confirmItems, m.confirmCursor)
	if vr < m.confirmOffset {
		m.confirmOffset = vr
	}
	if vr >= m.confirmOffset+vh {
		m.confirmOffset = vr - vh + 1
	}
}

// triageTypeOrder assigns a display sort priority to each content type.
// Skills and Agents are most common in community registries so they sort first.
var triageTypeOrder = map[catalog.ContentType]int{
	catalog.Skills:   0,
	catalog.Agents:   1,
	catalog.Rules:    2,
	catalog.Hooks:    3,
	catalog.Commands: 4,
	catalog.MCP:      5,
	catalog.Loadouts: 6,
}

// sortConfirmItemsByType returns a copy of items sorted by content type (using
// triageTypeOrder) and a new selected map with indices remapped to the sorted
// positions. A stable sort preserves relative order within each type group.
func sortConfirmItemsByType(items []addConfirmItem, selected map[int]bool) ([]addConfirmItem, map[int]bool) {
	type indexed struct {
		origIdx int
		item    addConfirmItem
	}
	work := make([]indexed, len(items))
	for i, item := range items {
		work[i] = indexed{origIdx: i, item: item}
	}
	sort.SliceStable(work, func(i, j int) bool {
		oi := triageTypeOrder[work[i].item.itemType]
		oj := triageTypeOrder[work[j].item.itemType]
		return oi < oj
	})
	sorted := make([]addConfirmItem, len(items))
	newSel := make(map[int]bool, len(selected))
	for newIdx, w := range work {
		sorted[newIdx] = w.item
		if selected[w.origIdx] {
			newSel[newIdx] = true
		}
	}
	return sorted, newSel
}

// triageItemVisualRow returns the 0-indexed row position of the item at idx
// within the scrollable section of the triage items pane. Items must already
// be sorted by type (via sortConfirmItemsByType). Type-group section headers
// inserted before each group add extra rows before each item.
func triageItemVisualRow(items []addConfirmItem, idx int) int {
	if len(items) == 0 {
		return 0
	}
	headers := 1 // first group always has a header at row 0
	for k := 1; k <= idx && k < len(items); k++ {
		if items[k].itemType != items[k-1].itemType {
			headers++
		}
	}
	return idx + headers
}

// loadTriagePreview loads the file at the current triage cursor into the preview.
func (m *addWizardModel) loadTriagePreview() {
	if m.confirmCursor >= len(m.confirmItems) {
		return
	}
	item := m.confirmItems[m.confirmCursor]

	// Hooks use virtual hook.json rendered from hookData, not a real file path.
	if item.itemType == catalog.Hooks && item.hookData != nil {
		di := addDiscoveryItem{
			itemType:      catalog.Hooks,
			hookData:      item.hookData,
			hookSourceDir: item.hookSourceDir,
		}
		content, err := readHookPreviewContent(di, "hook.json")
		m.confirmPreview = newPreviewModel()
		if err == nil {
			m.confirmPreview.lines = strings.Split(content, "\n")
			m.confirmPreview.fileName = "hook.json"
		}
		return
	}

	if item.sourceDir == "" || item.path == "" {
		m.confirmPreview = newPreviewModel()
		return
	}

	// MCPs: extract just the relevant server JSON section instead of reading
	// the full settings file (which can be thousands of lines).
	if item.itemType == catalog.MCP {
		fullPath := filepath.Join(item.sourceDir, item.path)
		extracted := extractJSONSection(fullPath, item.displayName, item.itemType)
		m.confirmPreview = newPreviewModel()
		if extracted != "" {
			m.confirmPreview.lines = strings.Split(extracted, "\n")
			m.confirmPreview.fileName = item.displayName + ".json"
		}
		return
	}

	// ReadFileContent has its own path traversal guard (string-prefix check,
	// does not follow symlinks), so no SafeResolve needed here.
	content, readErr := catalog.ReadFileContent(item.sourceDir, item.path, 200)
	if readErr != nil {
		m.confirmPreview = newPreviewModel()
		return
	}
	m.confirmPreview = newPreviewModel()
	m.confirmPreview.lines = strings.Split(content, "\n")
}

// mergeConfirmIntoDiscovery inserts user-selected confirm items into the
// actionable block of discoveredItems before entering Review. Safe to call
// multiple times — rebuilds from the pre-merge baselines each time
// (idempotency guarantee for Back→Triage→Next flows).
//
// Layout invariant: discoveredItems = [actionable... + merged...] + [installed...]
// Merged items carry StatusNew so they belong in the actionable block;
// placing them in the trailing installed block would hide them from
// visibleDiscoveryItems() when showInstalled=false.
func (m *addWizardModel) mergeConfirmIntoDiscovery() {
	baseActionable := m.preMergeActionableCount
	baseInstalled := m.preMergeInstalledCount

	// Split the current slice back to its pre-merge halves. actionableCount may
	// have grown past baseActionable on a prior merge; installedCount is stable.
	origActionable := append([]addDiscoveryItem(nil), m.discoveredItems[:baseActionable]...)
	origInstalled := append([]addDiscoveryItem(nil), m.discoveredItems[m.actionableCount:m.actionableCount+baseInstalled]...)

	var merged []addDiscoveryItem
	for i, item := range m.confirmItems {
		if !m.confirmSelected[i] {
			continue
		}
		status := item.status
		if status == 0 {
			status = add.StatusNew
		}
		di := addDiscoveryItem{
			name:            item.displayName,
			displayName:     item.displayName,
			itemType:        item.itemType,
			path:            filepath.Join(item.sourceDir, item.path),
			sourceDir:       item.sourceDir,
			status:          status,
			detectionSource: "content-signal",
			tier:            item.tier,
		}
		if item.underlying != nil {
			di.underlying = item.underlying
		} else {
			di.underlying = &add.DiscoveryItem{
				Name:      item.displayName,
				Type:      item.itemType,
				Path:      filepath.Join(item.sourceDir, item.path),
				SourceDir: item.sourceDir,
				Status:    status,
			}
		}
		if item.detected != nil {
			di.confidence = item.detected.Confidence
		}

		// Preserve hook data so Review drill-in can render virtual hook.json.
		if item.hookData != nil {
			di.hookData = item.hookData
			di.hookSourceDir = item.hookSourceDir
		}

		// Preserve splittability + Heuristic-step decision.
		di.splittable = item.splittable
		di.splitSectionCount = item.splitSectionCount
		di.splitChosen = item.splitChosen

		// Prefer pre-computed risks from discovery (hooks carry command/URL risks
		// that aren't derivable from file content alone). Fall back to deriving
		// from the catalogItem or reconstructed ContentItem when none are cached.
		if len(item.risks) > 0 {
			di.risks = item.risks
		}
		// Prefer the original catalogItem (has correct Files list) over a
		// reconstructed one; fall back to reconstruction when not available.
		if item.catalogItem != nil {
			di.catalogItem = item.catalogItem
			if len(di.risks) == 0 {
				di.risks = catalog.RiskIndicators(*item.catalogItem)
			}
		} else if di.sourceDir != "" {
			ci := catalog.ContentItem{
				Name:  item.displayName,
				Type:  item.itemType,
				Path:  item.sourceDir,
				Files: []string{item.path},
			}
			di.catalogItem = &ci
			if len(di.risks) == 0 {
				di.risks = catalog.RiskIndicators(ci)
			}
		}
		merged = append(merged, di)
	}

	m.discoveredItems = append(append(origActionable, merged...), origInstalled...)
	m.actionableCount = baseActionable + len(merged)
	m.installedCount = baseInstalled

	// buildDiscoveryList pre-selects StatusNew/StatusOutdated items, which
	// covers both the original actionable set and the newly merged items.
	m.discoveryList = m.buildDiscoveryList()
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
	m.buttonCursor = 2 // [Back] (0=Add, 1=Rename, 2=Back, 3=Cancel)
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
	// Drill-in always opens with the panes focused; the title-row button cursor
	// is disabled (-1) until the user Tabs into it.
	m.drillButtonCursor = -1
	item := selected[m.reviewItemCursor]

	// Use the catalogItem directly when available (has correct Path + Files
	// from the catalog scanner). Fall back to filesystem scan for items
	// discovered from providers (hooks, MCP, file-based without catalog scan).
	var ci catalog.ContentItem
	if item.itemType == catalog.Hooks && item.hookData != nil {
		// Canonical hook drill-in: show the flat hook.json object + any
		// scripts it references, matching the on-disk layout that the
		// library and Content > Hooks surfaces see.
		files := buildHookPreviewFiles(item)
		m.reviewDrillTree = newFileTreeModel(files)
		m.reviewDrillTree.focused = true
		// buildTree sorts alphabetically, so hook.json may land mid-list.
		// Sync the cursor to match the preview we're about to load.
		m.reviewDrillTree.SelectPath("hook.json")
		m.reviewDrillPreview = newPreviewModel()
		content, err := readHookPreviewContent(item, "hook.json")
		if err != nil {
			m.reviewDrillPreview.lines = []string{"Error: " + err.Error()}
		} else {
			m.reviewDrillPreview.lines = strings.Split(content, "\n")
		}
		m.reviewDrillPreview.fileName = "hook.json"

		m.setupDrillInRisks(item)
		return
	}
	// MCPs always use extractJSONSection (checked before catalogItem so that
	// the synthesized catalogItem from mergeConfirmIntoDiscovery doesn't
	// bypass extraction and cause LoadItem to read the entire settings file).
	if item.itemType == catalog.MCP && item.path != "" {
		extracted := extractJSONSection(item.path, item.name, item.itemType)
		if extracted != "" {
			m.reviewDrillTree = newFileTreeModel([]string{item.name + ".json"})
			m.reviewDrillTree.focused = true
			m.reviewDrillPreview = newPreviewModel()
			m.reviewDrillPreview.fileName = item.name + ".json"
			m.reviewDrillPreview.lines = strings.Split(extracted, "\n")

			highlights := make(map[int]bool)
			for lineNum, line := range m.reviewDrillPreview.lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, `"command"`) ||
					strings.Contains(lower, `"url"`) ||
					strings.Contains(lower, `"env"`) {
					highlights[lineNum+1] = true
				}
			}
			if len(highlights) > 0 {
				m.reviewDrillPreview.SetHighlightLines(highlights)
			}

			m.setupDrillInRisks(item)
			return
		}
		// extractJSONSection failed; fall through to generic file view.
	}
	if item.catalogItem != nil {
		ci = *item.catalogItem
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
	if m.reviewDrillPreview.fileName != "" {
		m.applyDrillInHighlights(m.reviewDrillPreview.fileName, item.itemType)
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
	m.drillButtonCursor = -1
}

// loadDrillInFile loads the file selected in the drill-in tree into the preview.
func (m *addWizardModel) loadDrillInFile() {
	selected := m.selectedItems()
	if m.reviewItemCursor >= len(selected) {
		return
	}
	item := selected[m.reviewItemCursor]

	relPath := m.reviewDrillTree.SelectedPath()
	if relPath == "" {
		return
	}

	// Hooks use a virtual tree (hook.json + bundled scripts) since the
	// canonical representation is not a literal file on disk yet.
	if item.itemType == catalog.Hooks && item.hookData != nil {
		content, err := readHookPreviewContent(item, relPath)
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
		m.applyDrillInHighlights(relPath, item.itemType)
		return
	}

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
	m.applyDrillInHighlights(relPath, item.itemType)
}

// applyDrillInHighlights sets highlight lines on the drill-in preview for the
// given file. Priority:
//  1. Risk-line highlights from drillInItemRisks matching relPath (any file)
//  2. For hooks viewing hook.json: command/url/env line highlights
//  3. No highlights
//
// Scrolls to firstLine-3 when risk highlights are present.
func (m *addWizardModel) applyDrillInHighlights(relPath string, itemType catalog.ContentType) {
	// Priority 1: risk-line highlights for this file
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
		if firstLine > 0 {
			m.reviewDrillPreview.offset = max(0, firstLine-3)
		}
		return
	}

	// Priority 2: hook.json command/url/env highlights (shown to surface intent)
	if itemType == catalog.Hooks && relPath == "hook.json" {
		hookHL := make(map[int]bool)
		for i, line := range m.reviewDrillPreview.lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, `"command"`) ||
				strings.Contains(lower, `"url"`) ||
				strings.Contains(lower, `"env"`) {
				hookHL[i+1] = true
			}
		}
		if len(hookHL) > 0 {
			m.reviewDrillPreview.SetHighlightLines(hookHL)
			return
		}
	}

	m.reviewDrillPreview.SetHighlightLines(nil)
}

// openRenameModal opens the review-step rename modal for the given cursor
// position in selectedItems(). Pre-fills with the item's current display name
// (falling back to its identifier) and description.
func (m *addWizardModel) openRenameModal(cursor int) {
	selected := m.selectedItems()
	if cursor < 0 || cursor >= len(selected) {
		return
	}
	item := selected[cursor]

	// Map back to the source index in discoveredItems. selectedItems() walks
	// discoveryList.SelectedIndices() into visibleDiscoveryItems(), which is a
	// prefix slice of discoveredItems — so the selected index IS the
	// discoveredItems index.
	selIndices := m.discoveryList.SelectedIndices()
	if cursor >= len(selIndices) {
		return
	}
	discIdx := selIndices[cursor]
	if discIdx < 0 || discIdx >= len(m.discoveredItems) {
		return
	}

	currentName := item.displayName
	if currentName == "" {
		currentName = item.name
	}

	m.renameItemCursor = cursor
	m.renameDiscoveryIdx = discIdx
	m.renameModal.OpenWithContext(
		"Rename: "+item.name,
		currentName,
		item.description,
		item.name, // path field carries the identifier for display/reference
		"wizard_rename",
	)
}

// handleRenameSaved applies a rename from the review modal to the in-memory
// discoveredItem. Nothing is written to disk — the new display name and
// description are persisted into .syllago.yaml at execute time by
// writeHookToLibrary / add.AddItems.
//
// For the monolithic-rule path (addSourceMonolithic), the rename targets
// reviewRenames[reviewCandidateCursor] instead of discoveredItems, since the
// monolithic flow works off buildReviewCandidates output rather than
// discoveredItems.
func (m *addWizardModel) handleRenameSaved(msg editSavedMsg) {
	if m.source == addSourceMonolithic {
		idx := m.reviewCandidateCursor
		if idx < 0 || idx >= len(m.reviewRenames) {
			return
		}
		m.reviewRenames[idx] = strings.TrimSpace(msg.name)
		return
	}
	if m.renameDiscoveryIdx < 0 || m.renameDiscoveryIdx >= len(m.discoveredItems) {
		return
	}
	// Empty name falls back to the identifier so we never end up with a blank
	// display name in the UI.
	m.discoveredItems[m.renameDiscoveryIdx].displayName = strings.TrimSpace(msg.name)
	m.discoveredItems[m.renameDiscoveryIdx].description = strings.TrimSpace(msg.description)
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
			items, confirmItems, err := discoverFromProvider(prov, projectRoot, cfg, contentRoot, types)
			return addDiscoveryDoneMsg{seq: seq, items: items, confirmItems: confirmItems, err: err}
		}

	case addSourceRegistry:
		if m.registryCursor >= len(m.registries) {
			return nil
		}
		reg := m.registries[m.registryCursor]
		return func() tea.Msg {
			items, confirmItems, err := discoverFromRegistry(reg, types, contentRoot)
			return addDiscoveryDoneMsg{seq: seq, items: items, confirmItems: confirmItems, err: err, sourceRegistry: reg.Name}
		}

	case addSourceLocal:
		dir := m.pathInput
		return func() tea.Msg {
			items, confirmItems, err := discoverFromLocalPath(dir, types, contentRoot)
			return addDiscoveryDoneMsg{seq: seq, items: items, confirmItems: confirmItems, err: err}
		}

	case addSourceGit:
		url := m.pathInput
		return func() tea.Msg {
			items, confirmItems, tmpDir, err := discoverFromGitURL(url, types, contentRoot)
			return addDiscoveryDoneMsg{seq: seq, items: items, confirmItems: confirmItems, err: err, tmpDir: tmpDir}
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
) ([]addDiscoveryItem, []addConfirmItem, error) {
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

	// File-based discovery (rules, skills, agents, commands).
	// Run discovery from both the project root and the user's home directory so
	// that global provider installs (e.g. ~/.claude/agents/) are visible even
	// when the TUI is launched from a project directory that has no local copy.
	discovered, err := add.DiscoverFromProvider(prov, projectRoot, resolver, contentRoot)
	if err != nil {
		return nil, nil, err
	}
	if homeDir, hdErr := os.UserHomeDir(); hdErr == nil && homeDir != projectRoot {
		globalDiscovered, _ := add.DiscoverFromProvider(prov, homeDir, nil, contentRoot)
		existingKeys := make(map[string]bool, len(discovered))
		for _, d := range discovered {
			existingKeys[string(d.Type)+"/"+d.Name] = true
		}
		for _, d := range globalDiscovered {
			if !existingKeys[string(d.Type)+"/"+d.Name] {
				discovered = append(discovered, d)
			}
		}
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

		// Build a ContentItem for risk scanning and drill-in preview.
		// ci.Path must be a directory so ReadFileContent(ci.Path, filename)
		// resolves correctly. For directory-based items use SourceDir; for
		// single-file items use the file's parent directory.
		ci := catalog.ContentItem{
			Name: d.Name,
			Type: d.Type,
		}
		if d.SourceDir != "" {
			ci.Path = d.SourceDir
			ci.Files = scanDrillInFiles(d.SourceDir)
		} else {
			ci.Path = filepath.Dir(d.Path)
			ci.Files = []string{filepath.Base(d.Path)}
		}
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

	// Provider sources use pattern-only detection (I11: no analyzer for provider dirs)
	return items, nil, nil
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
		items := hooksFromSettingsFile(loc.Path, prov.Slug, loc.Scope.String(), idx, hooks)
		result = append(result, items...)
	}
	return result
}

// hooksFromSettingsFile converts an already-parsed list of HookData from a
// single settings file into discovery items. Each HookData becomes one item
// with full hookData + hookSourceDir populated so drill-in renders the
// canonical flat hook.json + bundled scripts.
func hooksFromSettingsFile(
	settingsPath, providerSlug, scope string,
	idx add.LibraryIndex,
	hooks []converter.HookData,
) []addDiscoveryItem {
	var result []addDiscoveryItem
	sourceDir := filepath.Dir(settingsPath)
	seen := map[string]int{}
	for _, hook := range hooks {
		hookCopy := hook
		baseName := converter.DeriveHookName(hookCopy)
		// Two different handlers can derive the same name (same event+matcher
		// with no script reference or status message). Suffix collisions with
		// -2, -3, … so each discovery row is uniquely addressable.
		name := baseName
		seen[baseName]++
		if n := seen[baseName]; n > 1 {
			name = fmt.Sprintf("%s-%d", baseName, n)
		}

		key := string(catalog.Hooks) + "/" + providerSlug + "/" + name
		_, inLib := idx[key]
		status := add.StatusNew
		if inLib {
			status = add.StatusInLibrary
		}

		di := add.DiscoveryItem{
			Name:   name,
			Type:   catalog.Hooks,
			Path:   settingsPath,
			Status: status,
			Scope:  scope,
		}

		var hookRisks []catalog.RiskIndicator
		for _, entry := range hookCopy.Hooks {
			if entry.Command != "" {
				hookRisks = append(hookRisks, catalog.RiskIndicator{
					Label:       "Runs commands",
					Description: "Hook executes: " + truncate(entry.Command, 60),
					Level:       catalog.RiskHigh,
				})
				break
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
			name:          name,
			itemType:      catalog.Hooks,
			path:          settingsPath,
			status:        status,
			scope:         scope,
			risks:         hookRisks,
			underlying:    &di,
			hookData:      &hookCopy,
			hookSourceDir: sourceDir,
		})
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
) ([]addDiscoveryItem, []addConfirmItem, error) {
	cats, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{reg})
	if err != nil {
		return nil, nil, err
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

	// Run content-signal analyzer on registry clone dir
	var confirmItems []addConfirmItem
	if len(cats.Items) > 0 {
		regDir := filepath.Dir(cats.Items[0].Path)
		// Walk up to find the registry root (parent of content type dirs)
		for regDir != "/" && filepath.Base(regDir) != reg.Name {
			parent := filepath.Dir(regDir)
			if parent == regDir {
				break
			}
			regDir = parent
		}
		az := analyzer.New(analyzer.DefaultConfig())
		result, azErr := az.Analyze(regDir)
		if azErr == nil {
			patternPaths := make(map[string]bool, len(items))
			for _, it := range items {
				patternPaths[filepath.ToSlash(filepath.Clean(it.path))] = true
			}
			for _, detected := range result.Auto {
				if !typeSet[detected.Type] {
					continue
				}
				canon := filepath.ToSlash(filepath.Clean(filepath.Join(regDir, detected.Path)))
				if patternPaths[canon] {
					continue
				}
				patternPaths[canon] = true
				di := addDiscoveryItem{
					name:            detected.DisplayName,
					itemType:        detected.Type,
					path:            filepath.Join(regDir, detected.Path),
					sourceDir:       filepath.Join(regDir, filepath.Dir(detected.Path)),
					status:          add.StatusNew,
					detectionSource: "content-signal",
					tier:            analyzer.TierForItem(detected),
				}
				items = append(items, di)
			}
			for _, detected := range result.Confirm {
				if !typeSet[detected.Type] {
					continue
				}
				name := detected.DisplayName
				if name == "" {
					name = detected.Name
				}
				ci := addConfirmItem{
					detected:    detected,
					tier:        analyzer.TierForItem(detected),
					displayName: name,
					itemType:    detected.Type,
					path:        filepath.Base(detected.Path),
					sourceDir:   filepath.Join(regDir, filepath.Dir(detected.Path)),
				}
				confirmItems = append(confirmItems, ci)
			}
		}
	}

	return items, confirmItems, nil
}

func discoverFromLocalPath(
	dir string,
	selectedTypes []catalog.ContentType,
	contentRoot string,
) ([]addDiscoveryItem, []addConfirmItem, error) {
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
			return nil, nil, err
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

	// Run content-signal analyzer for fallback detection
	az := analyzer.New(analyzer.DefaultConfig())
	result, azErr := az.Analyze(dir)
	if azErr != nil {
		// Analyzer unavailable — return pattern items only
		return items, nil, nil
	}

	// Build dedup set from pattern-detected paths
	patternPaths := make(map[string]bool, len(items))
	for _, it := range items {
		patternPaths[filepath.ToSlash(filepath.Clean(it.path))] = true
	}

	// Merge Auto items (dedup, type-filtered)
	for _, detected := range result.Auto {
		if !typeSet[detected.Type] {
			continue
		}
		canon := filepath.ToSlash(filepath.Clean(filepath.Join(dir, detected.Path)))
		if patternPaths[canon] {
			continue // pattern-detected wins
		}
		patternPaths[canon] = true
		di := addDiscoveryItem{
			name:            detected.DisplayName,
			itemType:        detected.Type,
			path:            filepath.Join(dir, detected.Path),
			sourceDir:       filepath.Join(dir, filepath.Dir(detected.Path)),
			status:          add.StatusNew,
			detectionSource: "content-signal",
			tier:            analyzer.TierForItem(detected),
		}
		items = append(items, di)
	}

	// Build Confirm items (type-filtered)
	var confirmItems []addConfirmItem
	for _, detected := range result.Confirm {
		if !typeSet[detected.Type] {
			continue
		}
		name := detected.DisplayName
		if name == "" {
			name = detected.Name
		}
		ci := addConfirmItem{
			detected:    detected,
			tier:        analyzer.TierForItem(detected),
			displayName: name,
			itemType:    detected.Type,
			path:        filepath.Base(detected.Path),
			sourceDir:   filepath.Join(dir, filepath.Dir(detected.Path)),
		}
		confirmItems = append(confirmItems, ci)
	}

	return items, confirmItems, nil
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
			// Hooks need special handling: each NativeItem is one hook inside
			// a shared settings.json, but drill-in/library want one flat
			// hook.json per entry. Parse the settings file and split it the
			// same way discoverHooksFromProvider does so the drill-in shows
			// just the single hook (not the whole settings.json).
			if ct == catalog.Hooks {
				seen := make(map[string]bool)
				for _, ni := range nativeItems {
					settingsPath := filepath.Join(baseDir, ni.Path)
					if seen[settingsPath] {
						continue
					}
					seen[settingsPath] = true
					data, err := os.ReadFile(settingsPath)
					if err != nil {
						continue
					}
					hooks, err := converter.SplitSettingsHooks(data, pc.ProviderSlug)
					if err != nil {
						continue
					}
					items = append(items, hooksFromSettingsFile(settingsPath, pc.ProviderSlug, pc.ProviderSlug, idx, hooks)...)
				}
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
) ([]addDiscoveryItem, []addConfirmItem, string, error) {
	tmpDir, err := cloneGitURL(url, 60)
	if err != nil {
		return nil, nil, "", err
	}
	items, confirmItems, err := discoverFromLocalPath(tmpDir, selectedTypes, contentRoot)
	if err != nil {
		_ = os.RemoveAll(filepath.Dir(tmpDir))
		return nil, nil, "", err
	}
	return items, confirmItems, tmpDir, nil
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

	// Hooks discovered from a provider's settings.json need their own write
	// path: item.path is the settings.json (not a hook.json), so the generic
	// add.AddItems copy-from-path flow would write the whole settings file as
	// hook.json. Instead, serialize the parsed HookData directly. Mirrors
	// cli/cmd/syllago/add_cmd.go addHooksFromLocation.
	if item.itemType == catalog.Hooks && item.hookData != nil {
		return writeHookToLibrary(item, contentRoot, srcReg, srcVis, provSlug)
	}

	opts := add.AddOptions{
		Force:            item.overwrite,
		Provider:         provSlug,
		SourceRegistry:   srcReg,
		SourceVisibility: srcVis,
	}

	// Forward review-step rename (if set) into the shared write path. Identity
	// (directory layout) stays keyed on Name; only metadata.Name changes.
	diCopy := *item.underlying
	if item.displayName != "" {
		diCopy.DisplayName = item.displayName
	}
	if item.description != "" {
		diCopy.Description = item.description
	}
	results := add.AddItems([]add.DiscoveryItem{diCopy}, opts, contentRoot, nil, "syllago")
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

// writeHookToLibrary serializes a HookData to ~/.syllago/hooks/<provider>/<name>/hook.json,
// bundles any referenced scripts into the same directory, and writes .syllago.yaml
// metadata. Mirrors addHooksFromLocation in cli/cmd/syllago/add_cmd.go.
func writeHookToLibrary(item addDiscoveryItem, contentRoot, srcReg, srcVis, provSlug string) addExecResult {
	itemDir := filepath.Join(contentRoot, string(catalog.Hooks), provSlug, item.name)

	if !item.overwrite {
		if info, err := os.Stat(itemDir); err == nil && info.IsDir() {
			return addExecResult{name: item.name, status: "skipped"}
		}
	}

	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("creating %s: %w", itemDir, err)}
	}

	// Bundle script files referenced by the hook's commands. Failure here is
	// non-fatal — the hook can still be installed if scripts are absolute paths.
	bundled, _ := converter.BundleHookScripts(item.hookData, item.hookSourceDir, itemDir)

	// Write the canonical hooks/0.1 Manifest shape (single handler per file).
	// SplitSettingsHooks guarantees one entry in hookData.Hooks post-split.
	manifest, err := converter.ManifestFromHookData(*item.hookData)
	if err != nil {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("building manifest: %w", err)}
	}
	hookJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("marshaling hook: %w", err)}
	}
	if err := os.WriteFile(filepath.Join(itemDir, "hook.json"), hookJSON, 0o644); err != nil {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("writing hook.json: %w", err)}
	}

	var bundledMeta []metadata.BundledScriptMeta
	for _, b := range bundled {
		bundledMeta = append(bundledMeta, metadata.BundledScriptMeta{
			OriginalPath: b.OriginalPath,
			Filename:     b.Filename,
		})
	}

	metaName := item.name
	if item.displayName != "" {
		metaName = item.displayName
	}

	now := time.Now().UTC()
	meta := &metadata.Meta{
		ID:               metadata.NewID(),
		Name:             metaName,
		Description:      item.description,
		Type:             string(catalog.Hooks),
		BundledScripts:   bundledMeta,
		AddedAt:          &now,
		SourceProvider:   provSlug,
		SourceFormat:     "json",
		SourceType:       "provider",
		SourceRegistry:   srcReg,
		SourceVisibility: srcVis,
		SourceScope:      item.scope,
	}
	if srcReg != "" {
		meta.SourceType = "registry"
	}
	if err := metadata.Save(itemDir, meta); err != nil {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("writing metadata: %w", err)}
	}

	if item.status == add.StatusInLibrary {
		return addExecResult{name: item.name, status: "updated"}
	}
	return addExecResult{name: item.name, status: "added"}
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

// buildHookPreviewFiles returns the virtual file list for a hook drill-in:
// "hook.json" (the flat single-hook object) followed by any referenced
// scripts that exist in item.hookSourceDir. This mirrors the on-disk layout
// that writeHookToLibrary will produce, so the wizard preview shows the same
// files the user will inspect in the library and Content > Hooks surfaces.
func buildHookPreviewFiles(item addDiscoveryItem) []string {
	files := []string{"hook.json"}
	if item.hookData == nil {
		return files
	}
	seen := map[string]bool{"hook.json": true}
	for _, h := range item.hookData.Hooks {
		ref := converter.ExtractScriptRef(h.Command)
		if ref == "" {
			continue
		}
		resolved, ok := resolveHookScriptPath(ref, item.hookSourceDir)
		if !ok {
			continue
		}
		base := filepath.Base(resolved)
		if base == "." || base == "/" || seen[base] {
			continue
		}
		if _, err := os.Stat(resolved); err != nil {
			continue
		}
		files = append(files, base)
		seen[base] = true
	}
	return files
}

// resolveHookScriptPath resolves a hook command's script reference to an
// absolute path, expanding env vars ($PAI_DIR, ${PAI_DIR}), ~/, and relative
// paths. Returns (path, true) on success, ("", false) if the ref cannot be
// resolved (e.g. unset env var yielding a path that still contains '$').
func resolveHookScriptPath(ref, sourceDir string) (string, bool) {
	if ref == "" {
		return "", false
	}
	// os.ExpandEnv handles both $VAR and ${VAR} forms. Unset vars expand to
	// empty, so $UNSET/foo.sh becomes /foo.sh — caught by the Stat check
	// upstream, but we also bail if the result still contains '$' (which
	// means an escaped or malformed var was left behind).
	expanded := os.ExpandEnv(ref)
	if expanded == "" {
		return "", false
	}
	if strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		return filepath.Join(home, expanded[2:]), true
	}
	if filepath.IsAbs(expanded) {
		return expanded, true
	}
	if sourceDir == "" {
		return "", false
	}
	return filepath.Clean(filepath.Join(sourceDir, expanded)), true
}

// readHookPreviewContent returns the preview content for a virtual hook file.
// "hook.json" returns a pretty-printed canonical Manifest JSON (hooks/0.1
// shape); other names are read from item.hookSourceDir as bundled script
// files would be.
func readHookPreviewContent(item addDiscoveryItem, relPath string) (string, error) {
	if relPath == "hook.json" {
		if item.hookData == nil {
			return "", fmt.Errorf("no hook data")
		}
		manifest, err := converter.ManifestFromHookData(*item.hookData)
		if err != nil {
			return "", fmt.Errorf("building manifest: %w", err)
		}
		out, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshaling hook: %w", err)
		}
		return string(out), nil
	}
	// Find the matching script ref so we resolve the real path (./foo, ~/foo,
	// $VAR/foo, or /abs/foo), not just a blind join of the basename.
	for _, h := range item.hookData.Hooks {
		ref := converter.ExtractScriptRef(h.Command)
		if ref == "" {
			continue
		}
		resolved, ok := resolveHookScriptPath(ref, item.hookSourceDir)
		if !ok || filepath.Base(resolved) != relPath {
			continue
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", relPath, err)
		}
		return string(data), nil
	}
	return "", fmt.Errorf("script %s not referenced by any hook entry", relPath)
}

// scanDrillInFiles collects visible files from a path for the drill-in preview.
// For files it returns the basename. For directories it walks and collects relative paths.
func scanDrillInFiles(path string) []string {
	info, err := os.Stat(path) // follows symlinks
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{filepath.Base(path)}
	}
	// WalkDir uses Lstat internally, so a symlinked root shows up as a
	// single non-directory entry instead of being walked. Resolve first.
	resolved := path
	if r, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		resolved = r
	}
	var files []string
	_ = filepath.WalkDir(resolved, func(p string, d os.DirEntry, err error) error {
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
		rel, _ := filepath.Rel(resolved, p)
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
