package tui

import (
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- Install Wizard invariants ---
//
// These tests verify the step machine's validateStep() assertions.
// Each test walks through steps manually, setting the required state
// at each transition. A panic means the invariant was violated.

func TestInstallWizard_ValidateStep_Forward(t *testing.T) {
	t.Parallel()
	// Walk through all 4 steps for a filesystem (non-JSON-merge) item.
	// No panics should occur.
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	root := t.TempDir()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(root, "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)

	// Step 0: Provider
	w.step = installStepProvider
	w.validateStep() // should not panic

	// Step 1: Location (requires valid provider cursor, not installed)
	w.providerCursor = 0
	w.step = installStepLocation
	w.shell.SetActive(1)
	w.validateStep() // should not panic

	// Step 2: Method (requires valid location cursor, not JSON merge)
	w.locationCursor = 0 // "global"
	w.step = installStepMethod
	w.shell.SetActive(2)
	w.validateStep() // should not panic

	// Step 3: Review (requires valid provider, valid location for filesystem)
	w.step = installStepReview
	w.shell.SetActive(3)
	w.validateStep() // should not panic
}

func TestInstallWizard_ValidateStep_Esc(t *testing.T) {
	t.Parallel()
	// Start at review, walk backwards. No panics.
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	root := t.TempDir()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(root, "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)

	// Set up to review step
	w.providerCursor = 1
	w.locationCursor = 1
	w.step = installStepReview
	w.shell.SetActive(3)
	w.validateStep() // should not panic

	// Back to method
	w.step = installStepMethod
	w.shell.SetActive(2)
	w.validateStep() // should not panic

	// Back to location
	w.step = installStepLocation
	w.shell.SetActive(1)
	w.validateStep() // should not panic

	// Back to provider
	w.step = installStepProvider
	w.shell.SetActive(0)
	w.validateStep() // should not panic
}

func TestInstallWizard_ValidateStep_AutoSkip(t *testing.T) {
	t.Parallel()
	// Single provider auto-skip: wizard opens at location step.
	prov := testInstallProvider("Claude Code", "claude-code", true)
	root := t.TempDir()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(root, "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, root)

	// openInstallWizard auto-skipped to location
	if w.step != installStepLocation {
		t.Fatalf("expected auto-skip to location, got step %d", w.step)
	}
	w.validateStep() // should not panic at location with auto-skipped provider
}

func TestInstallWizard_ValidateStep_JSONMerge(t *testing.T) {
	t.Parallel()
	// JSON merge path: provider -> review (skip location+method).
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	root := t.TempDir()
	item := testInstallItem("my-hook", catalog.Hooks, filepath.Join(root, "hooks", "my-hook"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)

	// Step 0: Provider
	w.step = installStepProvider
	w.shell.SetActive(0)
	w.validateStep() // should not panic

	// Step 3 (review): JSON merge skips location+method, but the step enum value
	// is still installStepReview. Shell active is 1 (second of 2 steps).
	w.providerCursor = 0
	w.step = installStepReview
	w.shell.SetActive(1)
	w.validateStep() // should not panic — isJSONMerge means locationCursor < 0 is OK
}

func TestInstallWizard_ValidateStep_PanicsOnEmpty(t *testing.T) {
	t.Parallel()
	// Verify that entering provider step with empty item panics.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for empty item at provider step")
		}
		msg, ok := r.(string)
		if !ok || msg != "wizard invariant: installStepProvider entered with empty item" {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()

	root := t.TempDir()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	// Item with empty Path
	item := catalog.ContentItem{Name: "bad", Type: catalog.Rules}
	w := &installWizardModel{
		shell:             newWizardShell("Install", []string{"Provider", "Location", "Method", "Review"}),
		step:              installStepProvider,
		item:              item,
		providers:         []provider.Provider{prov},
		providerInstalled: []bool{false},
		projectRoot:       root,
	}
	w.validateStep() // should panic
}

func TestInstallWizard_ValidateStep_PanicsOnInstalledLocation(t *testing.T) {
	t.Parallel()
	// Verify that entering location step with installed provider panics.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for installed provider at location step")
		}
	}()

	root := t.TempDir()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(root, "rules", "my-rule"))

	w := &installWizardModel{
		shell:             newWizardShell("Install", []string{"Provider", "Location", "Method", "Review"}),
		step:              installStepLocation,
		item:              item,
		providers:         []provider.Provider{prov},
		providerInstalled: []bool{true}, // installed!
		providerCursor:    0,
		projectRoot:       root,
	}
	w.validateStep() // should panic
}

func TestInstallWizard_ValidateStep_PanicsOnJSONMergeMethod(t *testing.T) {
	t.Parallel()
	// Verify that entering method step for JSON merge type panics.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for JSON merge at method step")
		}
	}()

	root := t.TempDir()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-hook", catalog.Hooks, filepath.Join(root, "hooks", "my-hook"))

	w := &installWizardModel{
		shell:             newWizardShell("Install", []string{"Provider", "Review"}),
		step:              installStepMethod,
		item:              item,
		providers:         []provider.Provider{prov},
		providerInstalled: []bool{false},
		providerCursor:    0,
		isJSONMerge:       true,
		projectRoot:       root,
	}
	w.validateStep() // should panic
}

// TestInstallWizard_ValidateStep_ConflictForward walks through the conflict step
// on the "install to all providers" path without triggering any panics.
func TestInstallWizard_ValidateStep_ConflictForward(t *testing.T) {
	t.Parallel()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)

	// Provider step with selectAll: no panics
	w.step = installStepProvider
	w.selectAll = true
	w.validateStep()

	// Conflict step: requires selectAll + non-empty conflicts
	w.conflicts = []installer.Conflict{{
		SharedPath:   sharedPath,
		InstallingTo: provA,
		AlsoReadBy:   []provider.Provider{provB},
	}}
	w.step = installStepConflict
	w.shell.SetSteps([]string{"Provider", "Conflicts"})
	w.shell.SetActive(1)
	w.validateStep() // should not panic
}

// TestInstallWizard_ValidateStep_ConflictEsc verifies Esc from conflict step
// goes back to provider step without panicking.
func TestInstallWizard_ValidateStep_ConflictEsc(t *testing.T) {
	t.Parallel()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)

	// Put wizard in conflict step
	w.selectAll = true
	w.conflicts = []installer.Conflict{{
		SharedPath:   sharedPath,
		InstallingTo: provA,
		AlsoReadBy:   []provider.Provider{provB},
	}}
	w.step = installStepConflict
	w.shell.SetSteps([]string{"Provider", "Conflicts"})
	w.shell.SetActive(1)
	w.validateStep()

	// Esc: back to provider
	w.conflicts = nil
	w.step = installStepProvider
	w.shell.SetSteps([]string{"Provider", "Location", "Method", "Review"})
	w.shell.SetActive(0)
	w.validateStep()
}

// TestInstallWizard_ValidateStep_ConflictPanicsOnNoConflicts verifies that
// entering the conflict step with empty conflicts panics.
func TestInstallWizard_ValidateStep_ConflictPanicsOnNoConflicts(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for conflict step with no conflicts")
		}
	}()
	root := t.TempDir()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))
	w := &installWizardModel{
		shell:             newWizardShell("Install", []string{"Provider", "Conflicts"}),
		step:              installStepConflict,
		item:              item,
		providers:         []provider.Provider{prov},
		providerInstalled: []bool{false},
		projectRoot:       root,
		selectAll:         true,
		conflicts:         nil, // empty — should panic
	}
	w.validateStep()
}

// TestInstallWizard_ValidateStep_ConflictPanicsOnNoSelectAll verifies that
// entering the conflict step without selectAll panics.
func TestInstallWizard_ValidateStep_ConflictPanicsOnNoSelectAll(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for conflict step without selectAll")
		}
	}()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))
	w := &installWizardModel{
		shell:             newWizardShell("Install", []string{"Provider", "Conflicts"}),
		step:              installStepConflict,
		item:              item,
		providers:         []provider.Provider{provA, provB},
		providerInstalled: []bool{false, false},
		projectRoot:       root,
		selectAll:         false, // not set — should panic
		conflicts: []installer.Conflict{{
			SharedPath:   sharedPath,
			InstallingTo: provA,
			AlsoReadBy:   []provider.Provider{provB},
		}},
	}
	w.validateStep()
}

// --- Add Wizard invariants ---

func TestAddWizard_ValidateStep_Forward(t *testing.T) {
	t.Parallel()
	m := openAddWizard(
		[]provider.Provider{testInstallProvider("Claude Code", "claude-code", true)},
		nil, nil, "/tmp", "/tmp", "",
	)

	// Step 0: Source — no prerequisites
	m.step = addStepSource
	m.validateStep()

	// Step 1: Type — requires source set
	m.source = addSourceProvider
	m.step = addStepType
	m.shell.SetActive(1)
	m.validateStep()

	// Step 2: Discovery — requires source + types
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.discovering = true // during scan, types not checked
	m.shell.SetActive(2)
	m.validateStep()

	// Step 3: Review — requires discovered + selected items
	m.discovering = false
	m.discoveredItems = []addDiscoveryItem{
		{name: "test", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "test", Type: catalog.Rules}},
	}
	m.discoveryList = m.buildDiscoveryList()
	m.step = addStepReview
	m.shell.SetActive(m.shellIndexForStep(addStepReview))
	m.validateStep()

	// Step 4: Execute — requires selected + acknowledged
	m.reviewAcknowledged = true
	m.step = addStepExecute
	m.shell.SetActive(m.shellIndexForStep(addStepExecute))
	m.validateStep()
}

func TestAddWizard_ValidateStep_Esc(t *testing.T) {
	t.Parallel()
	m := openAddWizard(
		[]provider.Provider{testInstallProvider("Claude Code", "claude-code", true)},
		nil, nil, "/tmp", "/tmp", "",
	)

	// Set up to Execute step
	m.source = addSourceProvider
	m.typeChecks = m.buildTypeCheckList()
	m.discoveredItems = []addDiscoveryItem{
		{name: "test", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "test", Type: catalog.Rules}},
	}
	m.discoveryList = m.buildDiscoveryList()
	m.reviewAcknowledged = true

	// Walk backwards
	m.step = addStepExecute
	m.shell.SetActive(m.shellIndexForStep(addStepExecute))
	m.validateStep()

	m.step = addStepReview
	m.shell.SetActive(m.shellIndexForStep(addStepReview))
	m.validateStep()

	m.discovering = false
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.validateStep()

	m.step = addStepType
	m.shell.SetActive(m.shellIndexForStep(addStepType))
	m.validateStep()

	m.step = addStepSource
	m.shell.SetActive(m.shellIndexForStep(addStepSource))
	m.validateStep()
}

func TestAddWizard_ValidateStep_PanicsOnTypeWithoutSource(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for Type without source")
		}
	}()

	m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
	m.source = addSourceNone
	m.step = addStepType
	m.validateStep()
}

func TestAddWizard_ValidateStep_PanicsOnReviewWithoutItems(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for Review without items")
		}
	}()

	m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
	m.source = addSourceLocal
	m.discoveredItems = nil
	m.step = addStepReview
	m.validateStep()
}

func TestAddWizard_ValidateStep_PanicsOnExecuteWithoutAck(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for Execute without acknowledgment")
		}
	}()

	m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
	m.source = addSourceLocal
	m.discoveredItems = []addDiscoveryItem{
		{name: "test", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "test", Type: catalog.Rules}},
	}
	m.discoveryList = m.buildDiscoveryList()
	m.reviewAcknowledged = false
	m.step = addStepExecute
	m.validateStep()
}

// --- Triage-path invariant tests (Task 9.1) ---

// makeTriageItems returns a small set of addConfirmItems for use in triage tests.
// Two items: one Medium confidence (unchecked by default), one High (pre-checked).
func makeTriageItems() []addConfirmItem {
	return []addConfirmItem{
		{
			detected:    &analyzer.DetectedItem{Confidence: 0.75, Provider: "content-signal"},
			tier:        analyzer.TierHigh,
			displayName: "high-confidence-rule",
			itemType:    catalog.Rules,
			path:        "rules/high.md",
			sourceDir:   "/tmp",
		},
		{
			detected:    &analyzer.DetectedItem{Confidence: 0.65, Provider: "content-signal"},
			tier:        analyzer.TierMedium,
			displayName: "medium-confidence-skill",
			itemType:    catalog.Skills,
			path:        "skills/medium.md",
			sourceDir:   "/tmp",
		},
	}
}

// TestAddWizard_ValidateStep_TriageForward walks through all 5 steps (+Type)
// without triggering any validateStep panics.
func TestAddWizard_ValidateStep_TriageForward(t *testing.T) {
	t.Parallel()

	m := openAddWizard(
		[]provider.Provider{testInstallProvider("Claude Code", "claude-code", true)},
		nil, nil, "/tmp", "/tmp", "",
	)

	// Step 0: Source
	m.step = addStepSource
	m.shell.SetActive(m.shellIndexForStep(addStepSource))
	m.validateStep()

	// Step 1: Type — requires source set
	m.source = addSourceLocal
	m.step = addStepType
	m.shell.SetActive(m.shellIndexForStep(addStepType))
	m.validateStep()

	// Step 2: Discovery — types checked, scanning in progress
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.discovering = true
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.validateStep()

	// Discovery done: load confirm items (unified triage-in-discovery)
	m.discovering = false
	m.confirmItems = makeTriageItems()
	m.confirmSelected = map[int]bool{0: true}

	// Merge selected confirm items into discovery before Review
	m.discoveredItems = nil
	m.actionableCount = 0
	m.installedCount = 0
	m.preMergeActionableCount = 0
	m.preMergeInstalledCount = 0
	m.mergeConfirmIntoDiscovery()

	// Step 3: Review — discoveredItems + selectedItems non-empty
	m.step = addStepReview
	m.shell.SetActive(m.shellIndexForStep(addStepReview))
	m.validateStep()

	// Step 4: Execute — selected items + acknowledged
	m.reviewAcknowledged = true
	m.step = addStepExecute
	m.shell.SetActive(m.shellIndexForStep(addStepExecute))
	m.validateStep()
}

// TestAddWizard_ValidateStep_TriageEsc verifies that Esc from the Discovery step
// (with confirm items loaded) goes back to Type without panicking.
func TestAddWizard_ValidateStep_TriageEsc(t *testing.T) {
	t.Parallel()

	m := openAddWizard(
		[]provider.Provider{testInstallProvider("Claude Code", "claude-code", true)},
		nil, nil, "/tmp", "/tmp", "",
	)

	// Set up Discovery step with confirm items loaded (triage-in-discovery state)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.confirmItems = makeTriageItems()
	m.confirmSelected = map[int]bool{}

	m.step = addStepDiscovery
	m.discovering = false
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.validateStep() // should not panic at discovery

	// Simulate Esc: go back to Type
	m.step = addStepType
	m.shell.SetActive(m.shellIndexForStep(addStepType))
	m.validateStep() // should not panic at type
}

// TestAddWizard_ValidateStep_SkipTriageWhenEmpty verifies that if discovery returns
// no confirm items, the triage step is not activated and Review is reachable directly.
func TestAddWizard_ValidateStep_SkipTriageWhenEmpty(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")

	// Set up through Discovery with no confirm items
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.discoveredItems = []addDiscoveryItem{
		{name: "rule-a", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "rule-a", Type: catalog.Rules}},
	}
	m.discoveryList = m.buildDiscoveryList()

	// No confirm items — confirmItems stays nil
	if len(m.confirmItems) != 0 {
		t.Fatalf("expected empty confirmItems, got %d", len(m.confirmItems))
	}

	// Review must be reachable without going through Triage
	m.step = addStepReview
	m.shell.SetActive(m.shellIndexForStep(addStepReview))
	m.validateStep() // should not panic
}

// TestAddWizard_ConfirmItemsParallelArray verifies that confirmItems and
// confirmSelected stay in sync: every index in confirmItems has a corresponding
// key in confirmSelected (possibly false), and no extra keys exist.
func TestAddWizard_ConfirmItemsParallelArray(t *testing.T) {
	t.Parallel()

	items := makeTriageItems()
	sel := map[int]bool{}
	// Simulate pre-check logic: High/User → checked, Medium/Low → unchecked
	for i, item := range items {
		switch item.tier {
		case analyzer.TierHigh, analyzer.TierUser:
			sel[i] = true
		default:
			sel[i] = false
		}
	}

	// Verify sync: every item index has an entry
	for i := range items {
		if _, ok := sel[i]; !ok {
			t.Errorf("confirmSelected missing key %d for item %q", i, items[i].displayName)
		}
	}
	// Verify no out-of-range keys
	for k := range sel {
		if k < 0 || k >= len(items) {
			t.Errorf("confirmSelected has out-of-range key %d (len=%d)", k, len(items))
		}
	}

	// Verify pre-check correctness: high is checked, medium is not
	if !sel[0] {
		t.Errorf("expected item 0 (High) to be pre-checked")
	}
	if sel[1] {
		t.Errorf("expected item 1 (Medium) to be unchecked")
	}
}

// TestAddWizard_MergeIdempotency verifies that mergeConfirmIntoDiscovery can be
// called twice without duplicating items in discoveredItems.
func TestAddWizard_MergeIdempotency(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
	m.source = addSourceLocal
	m.confirmItems = makeTriageItems()
	// Select both confirm items
	m.confirmSelected = map[int]bool{0: true, 1: true}

	// Seed with two actionable items
	m.discoveredItems = []addDiscoveryItem{
		{name: "a", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "a", Type: catalog.Rules}},
		{name: "b", itemType: catalog.Skills, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "b", Type: catalog.Skills}},
	}
	m.actionableCount = 2
	m.installedCount = 0
	m.preMergeActionableCount = 2
	m.preMergeInstalledCount = 0
	m.discoveryList = m.buildDiscoveryList()

	// First merge
	m.mergeConfirmIntoDiscovery()
	countAfterFirst := len(m.discoveredItems)

	// Second merge — should produce the same count (idempotent)
	m.mergeConfirmIntoDiscovery()
	countAfterSecond := len(m.discoveredItems)

	if countAfterFirst != countAfterSecond {
		t.Errorf("mergeConfirmIntoDiscovery not idempotent: first=%d second=%d",
			countAfterFirst, countAfterSecond)
	}

	// Sanity: 2 actionable + 2 selected confirm items = 4 total
	expected := 2 + 2
	if countAfterFirst != expected {
		t.Errorf("expected %d items after merge, got %d", expected, countAfterFirst)
	}
}

// TestAddWizard_MergePreservesLayoutWithInstalled is a regression test for a
// panic where mergeConfirmIntoDiscovery appended merged items AFTER the
// installed block, violating the [actionable][installed] layout invariant.
// The old code's selection loop then indexed discoveryList.selected past its
// length (which was sized to actionableCount when showInstalled=false),
// crashing with "index out of range".
func TestAddWizard_MergePreservesLayoutWithInstalled(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
	m.source = addSourceLocal

	// One confirm item selected
	m.confirmItems = []addConfirmItem{
		{displayName: "new-from-triage", itemType: catalog.Rules,
			path: "rules/new.md", sourceDir: "/tmp/src"},
	}
	m.confirmSelected = map[int]bool{0: true}

	// Seed: 1 actionable + 1 installed. This is the layout that triggered the
	// original panic — installedCount>0 with showInstalled=false.
	m.discoveredItems = []addDiscoveryItem{
		{name: "actionable-rule", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "actionable-rule", Type: catalog.Rules}},
		{name: "installed-rule", itemType: catalog.Rules, status: add.StatusInLibrary,
			underlying: &add.DiscoveryItem{Name: "installed-rule", Type: catalog.Rules, Status: add.StatusInLibrary}},
	}
	m.actionableCount = 1
	m.installedCount = 1
	m.preMergeActionableCount = 1
	m.preMergeInstalledCount = 1
	m.showInstalled = false
	m.discoveryList = m.buildDiscoveryList()

	// Must not panic.
	m.mergeConfirmIntoDiscovery()

	// Layout after merge: [actionable-rule, new-from-triage] + [installed-rule]
	if got, want := len(m.discoveredItems), 3; got != want {
		t.Fatalf("expected %d items after merge, got %d", want, got)
	}
	if m.actionableCount != 2 {
		t.Errorf("expected actionableCount=2 after merging 1 item, got %d", m.actionableCount)
	}
	if m.installedCount != 1 {
		t.Errorf("expected installedCount unchanged at 1, got %d", m.installedCount)
	}
	if m.discoveredItems[1].name != "new-from-triage" {
		t.Errorf("merged item should be at index 1 (actionable block), got %q",
			m.discoveredItems[1].name)
	}
	if m.discoveredItems[2].status != add.StatusInLibrary {
		t.Errorf("installed item should remain in trailing block, got status %v at index 2",
			m.discoveredItems[2].status)
	}

	// Second merge must produce identical layout (idempotency).
	m.mergeConfirmIntoDiscovery()
	if got, want := len(m.discoveredItems), 3; got != want {
		t.Errorf("second merge broke idempotency: expected %d items, got %d", want, got)
	}
	if m.actionableCount != 2 {
		t.Errorf("second merge broke actionableCount: expected 2, got %d", m.actionableCount)
	}
}

// TestAddWizard_ClearTriageState verifies that clearTriageState resets all
// triage-related fields and collapses the shell back to 5 steps.
func TestAddWizard_ClearTriageState(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
	m.source = addSourceLocal

	// Load confirm state
	m.confirmItems = makeTriageItems()
	m.confirmSelected = map[int]bool{0: true}
	m.confirmCursor = 1
	m.confirmOffset = 1
	m.confirmFocus = triageZonePreview
	m.maxStep = addStepDiscovery

	// Clear
	m.clearTriageState()

	// Verify all triage fields are reset
	if m.confirmItems != nil {
		t.Errorf("expected confirmItems=nil after clear, got %v", m.confirmItems)
	}
	if m.confirmSelected != nil {
		t.Errorf("expected confirmSelected=nil after clear, got %v", m.confirmSelected)
	}
	if m.confirmCursor != 0 {
		t.Errorf("expected confirmCursor=0 after clear, got %d", m.confirmCursor)
	}
	if m.confirmOffset != 0 {
		t.Errorf("expected confirmOffset=0 after clear, got %d", m.confirmOffset)
	}
	if m.confirmFocus != triageZoneItems {
		t.Errorf("expected confirmFocus=triageZoneItems after clear, got %d", m.confirmFocus)
	}
	if m.maxStep != addStepDiscovery {
		t.Errorf("expected maxStep=addStepDiscovery after clear, got %d", m.maxStep)
	}

	// Shell should now have 5 labels (no Triage): Source/Type/Discovery/Review/Execute
	wantLabels := []string{"Source", "Type", "Discovery", "Review", "Execute"}
	gotLabels := m.buildShellLabels()
	if len(gotLabels) != len(wantLabels) {
		t.Errorf("expected %d shell labels after clear, got %d: %v", len(wantLabels), len(gotLabels), gotLabels)
	}
}

// --- stepForShellIndex table-driven tests (Task 9.2) ---

// TestAddWizard_StepForShellIndex covers both permutations (±Type)
// across all valid shell indices.
func TestAddWizard_StepForShellIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		preFilterType catalog.ContentType // non-empty = -Type (Type step skipped)
		idx           int
		want          addStep
	}{
		// +Type: Source(0) Type(1) Discovery(2) Review(3) Execute(4)
		{"+Type idx=0", "", 0, addStepSource},
		{"+Type idx=1", "", 1, addStepType},
		{"+Type idx=2", "", 2, addStepDiscovery},
		{"+Type idx=3", "", 3, addStepReview},
		{"+Type idx=4", "", 4, addStepExecute},

		// -Type: Source(0) Discovery(1) Review(2) Execute(3)
		{"-Type idx=0", catalog.Rules, 0, addStepSource},
		{"-Type idx=1", catalog.Rules, 1, addStepDiscovery},
		{"-Type idx=2", catalog.Rules, 2, addStepReview},
		{"-Type idx=3", catalog.Rules, 3, addStepExecute},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", tc.preFilterType)
			got := m.stepForShellIndex(tc.idx)
			if got != tc.want {
				t.Errorf("stepForShellIndex(%d) = %d, want %d", tc.idx, got, tc.want)
			}
		})
	}
}

// --- Heuristic step invariant (Task 4.1) ---

// TestAddWizard_HeuristicStep_InvariantDiscoveryNonEmpty verifies that
// entering addStepHeuristic with an empty selectedCandidates slice panics
// with the expected invariant message. The heuristic step is only reachable
// from the monolithic-rule discovery path after at least one candidate
// source file has been multi-selected.
func TestAddWizard_HeuristicStep_InvariantDiscoveryNonEmpty(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for Heuristic without selected candidates")
		}
		msg, ok := r.(string)
		if !ok || msg != "wizard invariant: addStepHeuristic entered with no selected candidates" {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()

	m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
	m.source = addSourceMonolithic
	m.selectedCandidates = nil
	m.step = addStepHeuristic
	m.validateStep()
}
