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
	addReviewZoneRisks addReviewZone = iota
	addReviewZoneItems
	addReviewZoneButtons
)

// --- Messages ---

type addCloseMsg struct{}

type addDiscoveryDoneMsg struct {
	seq              int
	items            []addDiscoveryItem
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
}

// --- Execute result ---

type addExecResult struct {
	name   string
	status string // "added", "updated", "error", "skipped", "cancelled"
	err    error
}

// --- Model ---

type addWizardModel struct {
	shell  wizardShell
	step   addStep
	width  int
	height int
	seq    int // sequence number for stale message detection

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

	// Taint
	sourceRegistry   string
	sourceVisibility string

	// Review step
	risks              []catalog.RiskIndicator
	riskBanner         riskBanner
	conflicts          []int
	reviewZone         addReviewZone
	reviewItemCursor   int
	buttonCursor       int
	reviewAcknowledged bool

	// Execute step
	executeResults   []addExecResult
	executeCurrent   int
	executeDone      bool
	executeCancelled bool
	executing        bool

	// Git source
	gitTempDir string

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

	// Step labels depend on preFilterType
	var stepLabels []string
	if preFilterType != "" {
		stepLabels = []string{"Source", "Discovery", "Review", "Execute"}
	} else {
		stepLabels = []string{"Source", "Type", "Discovery", "Review", "Execute"}
	}

	// Default source cursor: Provider if any detected, else Local
	sourceCursor := 2 // Local
	if len(detected) > 0 {
		sourceCursor = 0 // Provider
	}

	m := &addWizardModel{
		shell:         newWizardShell("Add", stepLabels),
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
	var result []addDiscoveryItem
	for _, idx := range m.discoveryList.SelectedIndices() {
		if idx < len(m.discoveredItems) {
			result = append(result, m.discoveredItems[idx])
		}
	}
	return result
}

// shellIndexForStep maps an addStep to the wizard shell breadcrumb index,
// accounting for the 4-step (pre-filtered) vs 5-step (normal) path.
func (m *addWizardModel) shellIndexForStep(s addStep) int {
	if m.preFilterType != "" {
		// 4-step: Source=0, Discovery=1, Review=2, Execute=3
		switch s {
		case addStepSource:
			return 0
		case addStepDiscovery:
			return 1
		case addStepReview:
			return 2
		case addStepExecute:
			return 3
		}
	}
	// 5-step: Source=0, Type=1, Discovery=2, Review=3, Execute=4
	return int(s)
}

// advanceFromSource transitions from the Source step to the next step.
// Always clears downstream state to avoid showing stale data.
func (m *addWizardModel) advanceFromSource() {
	m.typeChecks = m.buildTypeCheckList()
	m.discoveredItems = nil
	m.discoveryList = checkboxList{}
	m.discoveryErr = ""
	m.risks = nil
	m.reviewAcknowledged = false
	m.seq++

	if m.preFilterType != "" {
		m.step = addStepDiscovery
		m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	} else {
		m.step = addStepType
		m.shell.SetActive(m.shellIndexForStep(addStepType))
	}
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

	return cl
}

// goBackFromDiscovery navigates back from the Discovery step.
func (m *addWizardModel) goBackFromDiscovery() {
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

	// Compute aggregate risks
	m.risks = nil
	for _, item := range m.selectedItems() {
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

	// Default focus: buttons zone, cursor on [Back]
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 1 // [Back]
	m.reviewAcknowledged = false
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

// buildDiscoveryList creates a checkboxList from discovered items.
func (m *addWizardModel) buildDiscoveryList() checkboxList {
	items := make([]checkboxItem, len(m.discoveredItems))
	for i, d := range m.discoveredItems {
		var badge string
		var bStyle checkboxBadgeStyle

		switch d.status {
		case add.StatusNew:
			badge = "new"
			bStyle = badgeStyleSuccess
		case add.StatusInLibrary:
			badge = "in library"
			bStyle = badgeStyleMuted
		case add.StatusOutdated:
			badge = "outdated"
			bStyle = badgeStyleWarning
		}

		// Add risk badge if present
		if len(d.risks) > 0 {
			for _, r := range d.risks {
				if r.Level == catalog.RiskHigh {
					badge = "⚠ " + badge
					bStyle = badgeStyleDanger
					break
				}
			}
		}

		label := d.displayName
		if label == "" {
			label = d.name
		}
		label += "  " + mutedStyle.Render("("+string(d.itemType)+")")

		items[i] = checkboxItem{
			label:      label,
			badge:      badge,
			badgeStyle: bStyle,
		}
	}

	cl := newCheckboxList(items)
	cl.focused = true
	cl.zonePrefix = "add-disc"

	// Pre-select: New and Outdated checked, InLibrary unchecked
	for i, d := range m.discoveredItems {
		switch d.status {
		case add.StatusNew, add.StatusOutdated:
			cl.selected[i] = true
		}
		// Auto-set overwrite for outdated items
		if d.status == add.StatusOutdated {
			m.discoveredItems[i].overwrite = true
		}
	}

	return cl
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

		// Annotate with risks
		ci := catalog.ContentItem{
			Name: d.Name,
			Type: d.Type,
			Path: d.Path,
		}
		if d.SourceDir != "" {
			ci.Path = d.SourceDir
		}
		item.risks = catalog.RiskIndicators(ci)

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

			result = append(result, addDiscoveryItem{
				name:     name,
				itemType: catalog.Hooks,
				path:     loc.Path,
				status:   status,
				scope:    loc.Scope.String(),
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
		servers.ForEach(func(key, _ gjson.Result) bool {
			name := key.String()
			libKey := string(catalog.MCP) + "/" + prov.Slug + "/" + name
			_, inLib := idx[libKey]
			status := add.StatusNew
			if inLib {
				status = add.StatusInLibrary
			}
			result = append(result, addDiscoveryItem{
				name:     name,
				itemType: catalog.MCP,
				path:     loc.Path,
				status:   status,
				scope:    loc.Scope.String(),
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

		item := addDiscoveryItem{
			name:     ci.Name,
			itemType: ci.Type,
			path:     ci.Path,
			status:   status,
			risks:    catalog.RiskIndicators(ci),
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
	// Check if it has syllago structure (registry or content type dirs)
	nativeResult := catalog.ScanNativeContent(dir)

	var cat *catalog.Catalog
	var err error
	if nativeResult.HasSyllagoStructure {
		cat, err = catalog.Scan(dir, dir)
		if err != nil {
			return nil, err
		}
	} else {
		// No syllago structure — scan as-is
		cat, err = catalog.Scan(dir, dir)
		if err != nil {
			return nil, err
		}
	}

	typeSet := make(map[catalog.ContentType]bool)
	for _, t := range selectedTypes {
		typeSet[t] = true
	}

	idx, _ := add.BuildLibraryIndex(contentRoot)

	var items []addDiscoveryItem
	for _, ci := range cat.Items {
		if !typeSet[ci.Type] {
			continue
		}

		status := add.StatusNew
		key := string(ci.Type) + "/" + ci.Name
		if _, exists := idx[key]; exists {
			status = add.StatusInLibrary
		}

		item := addDiscoveryItem{
			name:     ci.Name,
			itemType: ci.Type,
			path:     ci.Path,
			status:   status,
			risks:    catalog.RiskIndicators(ci),
		}
		items = append(items, item)
	}

	return items, nil
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

// cloneGitURL performs a hardened git clone with security restrictions.
// Returns the path to the cloned repo directory. The caller must clean up
// via os.RemoveAll(filepath.Dir(repoDir)) to remove the parent temp dir.
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
