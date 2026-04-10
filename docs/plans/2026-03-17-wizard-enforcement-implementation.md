# Wizard Step Machine Enforcement - Implementation Plan

**Goal:** Add deterministic enforcement (production assertions + exhaustive tests + hook) to prevent wizard step machine bugs across all 5 TUI wizards.

**Architecture:** Two-layer enforcement: PostToolUse hook for immediate feedback on any TUI file edit, pre-commit gate via `make test`. Production `validateStep()` panics catch programmer errors. Exhaustive test matrix covers all forward paths, back paths, special cases, and parallel array invariants.

**Tech Stack:** Go, BubbleTea, bash hooks, Claude Code settings.json

**Design Doc:** `docs/prompts/wizard-enforcement-design.md`

---

## Task 1: validateStep() for importModel

**Files:**
- Modify: `cli/internal/tui/import.go` (add method after line ~265, call at line ~270)

**Depends on:** Nothing

**Success Criteria:**
- [ ] `validateStep()` method exists on `importModel`
- [ ] Called at top of `Update()` (line 269)
- [ ] Checks entry-prerequisites only (not cursor state, not step-output state)
- [ ] Panics with descriptive `"wizard invariant: ..."` messages
- [ ] All existing tests still pass

---

### Step 1: Add validateStep() method to importModel

Add immediately before the `Update()` method (before line 269):

```go
// validateStep checks entry-prerequisites for the current step.
// These are programmer-error assertions — panic on violation.
func (m importModel) validateStep() {
	switch m.step {
	case stepSource:
		// Entry point — no prerequisites beyond constructor state.
	case stepType:
		if len(m.types) == 0 {
			panic("wizard invariant: stepType entered with empty types")
		}
	case stepProvider:
		if len(m.providerNames) == 0 {
			panic("wizard invariant: stepProvider entered with empty providerNames")
		}
	case stepBrowseStart:
		if m.contentType == "" {
			panic("wizard invariant: stepBrowseStart entered with empty contentType")
		}
	case stepBrowse:
		// browser initialized by updateBrowseStart — checked at construction site.
	case stepValidate:
		if len(m.selectedPaths) == 0 && len(m.validationItems) == 0 {
			panic("wizard invariant: stepValidate entered with no selectedPaths or validationItems")
		}
	case stepPath:
		// pathInput initialized by constructor — no runtime prerequisite.
	case stepGitURL:
		// urlInput initialized by constructor — no runtime prerequisite.
	case stepGitPick:
		if len(m.clonedItems) == 0 || m.clonedPath == "" {
			panic("wizard invariant: stepGitPick entered with empty clonedItems or clonedPath")
		}
	case stepConfirm:
		if m.contentType == "" {
			panic("wizard invariant: stepConfirm entered with empty contentType")
		}
		if !m.contentType.IsUniversal() && m.providerName == "" {
			panic("wizard invariant: stepConfirm entered with provider-specific type but empty providerName")
		}
	case stepName:
		if m.contentType == "" {
			panic("wizard invariant: stepName entered with empty contentType")
		}
		if !m.contentType.IsUniversal() && len(m.providerNames) == 0 {
			panic("wizard invariant: stepName entered with provider-specific type but empty providerNames")
		}
	case stepConflict:
		if m.conflict == (conflictInfo{}) && len(m.batchConflicts) == 0 {
			panic("wizard invariant: stepConflict entered with no conflict info")
		}
	case stepHookSelect:
		if len(m.hookCandidates) == 0 {
			panic("wizard invariant: stepHookSelect entered with empty hookCandidates")
		}
		if len(m.hookCandidates) != len(m.hookSelected) || len(m.hookCandidates) != len(m.hookNames) {
			panic("wizard invariant: stepHookSelect parallel arrays have mismatched lengths")
		}
	case stepProviderPick:
		if len(m.providers) == 0 {
			panic("wizard invariant: stepProviderPick entered with empty providers")
		}
	case stepDiscoverySelect:
		if len(m.discoveryItems) == 0 {
			panic("wizard invariant: stepDiscoverySelect entered with empty discoveryItems")
		}
		if len(m.discoveryItems) != len(m.discoverySelected) {
			panic("wizard invariant: stepDiscoverySelect parallel arrays have mismatched lengths")
		}
	}
}
```

### Step 2: Call validateStep() at top of Update()

At line 269, immediately inside the `Update()` method body (before the `switch msg := msg.(type)`):

```go
func (m importModel) Update(msg tea.Msg) (importModel, tea.Cmd) {
	m.validateStep()
	switch msg := msg.(type) {
```

### Step 3: Verify existing tests pass

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestImport|TestPreFilter|TestDiscovery" -count=1`
Expected: PASS

---

## Task 2: validateStep() for createLoadoutScreen

**Files:**
- Modify: `cli/internal/tui/loadout_create.go` (add method before line ~414, call at line ~415)

**Depends on:** Nothing

**Success Criteria:**
- [ ] `validateStep()` method exists on `createLoadoutScreen`
- [ ] Called at top of `Update()` (line 414)
- [ ] Checks entry-prerequisites per step
- [ ] All existing tests still pass

---

### Step 1: Add validateStep() method

Add before the `Update()` method (before line 414):

```go
// validateStep checks entry-prerequisites for the current step.
func (m createLoadoutScreen) validateStep() {
	switch m.step {
	case clStepProvider:
		if len(m.providerList) == 0 {
			panic("wizard invariant: clStepProvider entered with empty providerList")
		}
	case clStepTypes:
		if m.prefilledProvider == "" {
			panic("wizard invariant: clStepTypes entered with empty prefilledProvider")
		}
		if len(m.typeEntries) == 0 {
			panic("wizard invariant: clStepTypes entered with empty typeEntries")
		}
	case clStepItems:
		if len(m.selectedTypes) == 0 {
			panic("wizard invariant: clStepItems entered with empty selectedTypes")
		}
	case clStepName:
		// Name/desc inputs initialized by constructor.
	case clStepDest:
		if len(m.destOptions) == 0 {
			panic("wizard invariant: clStepDest entered with empty destOptions")
		}
	case clStepReview:
		// All state accumulated from prior steps.
	}
}
```

### Step 2: Call at top of Update()

```go
func (m createLoadoutScreen) Update(msg tea.Msg) (createLoadoutScreen, tea.Cmd) {
	m.validateStep()
	switch msg := msg.(type) {
```

### Step 3: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestCreateLoadout|TestLoadout" -count=1`
Expected: PASS

---

## Task 3: validateStep() for installModal and envSetupModal

**Files:**
- Modify: `cli/internal/tui/modal.go` (two methods + two call sites)

**Depends on:** Nothing

**Success Criteria:**
- [ ] Both modal wizards have `validateStep()` methods
- [ ] Called AFTER `if !m.active { return }` guard in each `Update()`
- [ ] All existing tests still pass

---

### Step 1: Add validateStep() for envSetupModal

Add before line 71 (the Update method):

```go
func (m envSetupModal) validateStep() {
	switch m.step {
	case envStepChoose:
		if len(m.varNames) == 0 {
			panic("wizard invariant: envStepChoose entered with empty varNames")
		}
		if m.varIdx >= len(m.varNames) {
			panic("wizard invariant: envStepChoose entered with varIdx out of range")
		}
	case envStepValue:
		if len(m.varNames) == 0 || m.varIdx >= len(m.varNames) {
			panic("wizard invariant: envStepValue entered with invalid var state")
		}
	case envStepLocation:
		if m.value == "" {
			panic("wizard invariant: envStepLocation entered with empty value")
		}
	case envStepSource:
		// Text input for existing file path — no prerequisite beyond constructor.
	}
}
```

### Step 2: Call after active guard in envSetupModal.Update()

At line 71-72, the Update starts with `if !m.active { return m, nil }`. Add validateStep after:

```go
func (m envSetupModal) Update(msg tea.Msg) (envSetupModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	m.validateStep()
```

### Step 3: Add validateStep() for installModal

Add before line 634 (the Update method):

```go
func (m installModal) validateStep() {
	switch m.step {
	case installStepLocation:
		// Entry step — no prerequisites beyond constructor state (item, providers set).
	case installStepCustomPath:
		if m.locationCursor != 2 {
			panic("wizard invariant: installStepCustomPath entered without Custom location selected")
		}
	case installStepMethod:
		// Method selection — location already chosen.
	}
}
```

### Step 4: Call after active guard in installModal.Update()

At line 634-635:

```go
func (m installModal) Update(msg tea.Msg) (installModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	m.validateStep()
```

### Step 5: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestModal|TestInstall|TestEnv" -count=1`
Expected: PASS

---

## Task 4: validateStep() for updateModel

**Files:**
- Modify: `cli/internal/tui/update.go` (add method before line ~123, call at line ~124)

**Depends on:** Nothing

**Success Criteria:**
- [ ] `validateStep()` method exists on `updateModel`
- [ ] Called at top of `Update()`
- [ ] All existing tests still pass

---

### Step 1: Add validateStep() method

Add before line 123:

```go
func (m updateModel) validateStep() {
	switch m.step {
	case stepUpdateMenu:
		// Entry step — no prerequisites.
	case stepUpdatePreview:
		if m.releaseNotes == "" && m.fallbackLog == "" {
			panic("wizard invariant: stepUpdatePreview entered with no release notes or fallback log")
		}
	case stepUpdatePull:
		// Async operation — triggered by menu selection.
	case stepUpdateDone:
		// Terminal step — shows result.
	}
}
```

### Step 2: Call at top of Update()

```go
func (m updateModel) Update(msg tea.Msg) (updateModel, tea.Cmd) {
	m.validateStep()
	switch msg := msg.(type) {
```

### Step 3: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestUpdate" -count=1`
Expected: PASS

---

## Task 5: Wizard invariant test file — Import forward paths

**Files:**
- Create: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Tasks 1-4 (validateStep methods must exist)

**Success Criteria:**
- [ ] 27 forward-path tests for import wizard
- [ ] Tests verify step transitions without panics
- [ ] Table-driven with subtests

---

### Step 1: Create wizard_invariant_test.go with import forward-path tests

Create the file with test helpers and the 27-path matrix. Tests construct an import model via `navigateToImport(t)` (already defined in import_test.go), simulate key presses to navigate through steps, and assert `app.importer.step` at each transition point. The `validateStep()` call inside `Update()` provides the actual invariant enforcement — if prerequisites are missing, the test panics and the test fails.

Key test structure:

```go
package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// navigateToImportFiltered creates an import model pre-filtered for a content type.
// The import screen is not navigated to via the app — the importer is patched directly.
func navigateToImportFiltered(t *testing.T, ct catalog.ContentType) App {
	t.Helper()
	app := navigateToImport(t)
	app.importer.preFilterType = ct
	app.importer.contentType = ct
	return app
}

// --- Import Wizard Forward Paths ---

func TestWizardInvariantImportFromProvider(t *testing.T) {
	t.Run("NoPreFilter_SourceToProviderPick", func(t *testing.T) {
		app := navigateToImport(t)
		// sourceCursor 0 = From Provider
		m, _ := app.Update(keyEnter)
		app = m.(App)
		if app.importer.step != stepProviderPick {
			t.Fatalf("expected stepProviderPick, got %d", app.importer.step)
		}
	})

	t.Run("WithPreFilter_SourceToProviderPick", func(t *testing.T) {
		app := navigateToImportFiltered(t, catalog.Rules)
		// From Provider still goes to stepProviderPick regardless of pre-filter
		m, _ := app.Update(keyEnter)
		app = m.(App)
		if app.importer.step != stepProviderPick {
			t.Fatalf("expected stepProviderPick with pre-filter, got %d", app.importer.step)
		}
	})
}

func TestWizardInvariantImportLocalPath(t *testing.T) {
	// Table: type × pre-filter → expected intermediate step after type selection
	// Universal (Skills, Agents, MCP): → stepBrowseStart (skips provider)
	// Provider-specific (Rules, Hooks, Commands): → stepProvider
	tests := []struct {
		name       string
		ct         catalog.ContentType
		preFilter  bool
		wantAfterType importStep // step reached after type selection (or after source if pre-filtered)
	}{
		{"Skills_NoFilter", catalog.Skills, false, stepBrowseStart},
		{"Skills_WithFilter", catalog.Skills, true, stepBrowseStart},
		{"Agents_NoFilter", catalog.Agents, false, stepBrowseStart},
		{"Agents_WithFilter", catalog.Agents, true, stepBrowseStart},
		{"MCP_NoFilter", catalog.MCP, false, stepBrowseStart},
		{"MCP_WithFilter", catalog.MCP, true, stepBrowseStart},
		{"Rules_NoFilter", catalog.Rules, false, stepProvider},
		{"Rules_WithFilter", catalog.Rules, true, stepProvider},
		{"Hooks_NoFilter", catalog.Hooks, false, stepProvider},
		{"Hooks_WithFilter", catalog.Hooks, true, stepProvider},
		{"Commands_NoFilter", catalog.Commands, false, stepProvider},
		{"Commands_WithFilter", catalog.Commands, true, stepProvider},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var app App
			if tc.preFilter {
				// Pre-filter skips stepType; entering Local Path goes directly
				// to the provider step (provider-specific) or browseStart (universal)
				app = navigateToImportFiltered(t, tc.ct)
				app = pressN(app, keyDown, 1) // cursor 1 = Local Path
				m, _ := app.Update(keyEnter)
				app = m.(App)
				if app.importer.step != tc.wantAfterType {
					t.Fatalf("expected %d after local path with pre-filter, got %d",
						tc.wantAfterType, app.importer.step)
				}
			} else {
				// No pre-filter: source → stepType → select type → wantAfterType
				app = navigateToImport(t)
				app = pressN(app, keyDown, 1) // cursor 1 = Local Path
				m, _ := app.Update(keyEnter)
				app = m.(App)
				if app.importer.step != stepType {
					t.Fatalf("expected stepType, got %d", app.importer.step)
				}
				// Navigate to the right type and select
				for i, tp := range app.importer.types {
					if tp == tc.ct {
						app = pressN(app, keyDown, i)
						break
					}
				}
				m, _ = app.Update(keyEnter)
				app = m.(App)
				if app.importer.step != tc.wantAfterType && app.importer.message == "" {
					t.Fatalf("expected step %d or error message, got step %d",
						tc.wantAfterType, app.importer.step)
				}
			}
		})
	}
}

func TestWizardInvariantImportGitURL(t *testing.T) {
	app := navigateToImport(t)
	// cursor 2 = Git URL — bypasses stepType entirely
	app = pressN(app, keyDown, 2)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepGitURL {
		t.Fatalf("expected stepGitURL, got %d", app.importer.step)
	}
}

func TestWizardInvariantImportCreateNew(t *testing.T) {
	// Table: type × pre-filter → expected step after type selection
	// Universal: → stepName (skips provider)
	// Provider-specific: → stepProvider → stepName
	tests := []struct {
		name      string
		ct        catalog.ContentType
		preFilter bool
		wantStep  importStep // step reached after type selection (or after source if pre-filtered)
	}{
		{"Skills_NoFilter", catalog.Skills, false, stepName},
		{"Skills_WithFilter", catalog.Skills, true, stepName},
		{"Agents_NoFilter", catalog.Agents, false, stepName},
		{"Agents_WithFilter", catalog.Agents, true, stepName},
		{"MCP_NoFilter", catalog.MCP, false, stepName},
		{"MCP_WithFilter", catalog.MCP, true, stepName},
		{"Rules_NoFilter", catalog.Rules, false, stepProvider},
		{"Rules_WithFilter", catalog.Rules, true, stepProvider},
		{"Hooks_NoFilter", catalog.Hooks, false, stepProvider},
		{"Hooks_WithFilter", catalog.Hooks, true, stepProvider},
		{"Commands_NoFilter", catalog.Commands, false, stepProvider},
		{"Commands_WithFilter", catalog.Commands, true, stepProvider},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var app App
			if tc.preFilter {
				app = navigateToImportFiltered(t, tc.ct)
				app.importer.isCreate = true
				// cursor 3 = Create New; with pre-filter source Enter goes to
				// stepProvider (prov-specific) or stepName (universal)
				app = pressN(app, keyDown, 3)
				m, _ := app.Update(keyEnter)
				app = m.(App)
				if app.importer.step != tc.wantStep && app.importer.message == "" {
					t.Fatalf("expected step %d or error, got step %d",
						tc.wantStep, app.importer.step)
				}
			} else {
				// No pre-filter: source → stepType → select type → wantStep
				app = navigateToImport(t)
				app = pressN(app, keyDown, 3) // cursor 3 = Create New
				m, _ := app.Update(keyEnter)
				app = m.(App)
				if app.importer.step != stepType {
					t.Fatalf("expected stepType, got %d", app.importer.step)
				}
				for i, tp := range app.importer.types {
					if tp == tc.ct {
						app = pressN(app, keyDown, i)
						break
					}
				}
				m, _ = app.Update(keyEnter)
				app = m.(App)
				if app.importer.step != tc.wantStep && app.importer.message == "" {
					t.Fatalf("expected step %d or error, got step %d",
						tc.wantStep, app.importer.step)
				}
			}
		})
	}
}
```

Each test uses `navigateToImport(t)` or `navigateToImportFiltered(t, ct)` helpers. The `validateStep()` panics inside `Update()` serve as the actual enforcement — no need to duplicate those checks in the test assertions; a panic causes a test failure automatically.

### Step 2: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestWizardInvariant" -count=1 -v`
Expected: PASS with 27+ subtests

---

## Task 6: Wizard invariant tests — Import Esc/back paths

**Files:**
- Modify: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Task 5

**Success Criteria:**
- [ ] One test per Esc transition with condition variants
- [ ] Tests verify asymmetric back-navigation (preFilter vs no preFilter, universal vs provider-specific)
- [ ] ~10-12 test cases

---

### Step 1: Add Esc/back path tests

Key asymmetries to test (from inventory transition map):

```go
func TestWizardInvariantImportEscPaths(t *testing.T) {
	tests := []struct {
		name       string
		fromStep   importStep
		preFilter  catalog.ContentType
		ct         catalog.ContentType
		isCreate   bool
		expectStep importStep
	}{
		{"stepType→stepSource", stepType, "", "", false, stepSource},
		{"stepProvider+filter→stepSource", stepProvider, catalog.Rules, catalog.Rules, false, stepSource},
		{"stepProvider+noFilter→stepType", stepProvider, "", catalog.Rules, false, stepType},
		{"stepBrowseStart+filter→stepSource", stepBrowseStart, catalog.Skills, catalog.Skills, false, stepSource},
		{"stepBrowseStart+universal→stepType", stepBrowseStart, "", catalog.Skills, false, stepType},
		{"stepBrowseStart+provSpecific→stepProvider", stepBrowseStart, "", catalog.Rules, false, stepProvider},
		{"stepName+filter→stepSource", stepName, catalog.Skills, catalog.Skills, true, stepSource},
		{"stepName+create+provSpecific→stepProvider", stepName, "", catalog.Rules, true, stepProvider},
		{"stepName+create+universal→stepType", stepName, "", catalog.Skills, true, stepType},
		{"stepConfirm+create→stepName", stepConfirm, "", catalog.Skills, true, stepName},
		// ... stepConfirm+clonedPath, stepConfirm+browse variants
	}
	// Each test constructs model at fromStep with appropriate state, sends Esc, checks expectStep
}
```

### Step 2: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestWizardInvariantImportEsc" -count=1 -v`
Expected: PASS

---

## Task 7: Wizard invariant tests — Import special cases + parallel arrays

**Files:**
- Modify: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Task 5

**Success Criteria:**
- [ ] Special case tests: hooks .json, loadouts in type picker, fromRegistryRedirect, empty providers, empty discovery, git clone failure, SearchResults/Library entry
- [ ] Parallel array consistency tests: discovery filtered/unfiltered, hook candidates
- [ ] ~10-12 test cases

---

### Step 1: Add special case tests

```go
func TestWizardInvariantImportSpecialCases(t *testing.T) {
	// HooksJsonDetection: selecting a single .json file for Hooks type goes to
	// stepHookSelect (split into individual hooks) rather than stepValidate.
	// Tested via fileBrowserDoneMsg with a real .json file.
	t.Run("HooksJsonDetection", func(t *testing.T) {
		app := navigateToImport(t)
		app.importer.contentType = catalog.Hooks
		app.importer.providerName = "claude-code"
		// Write a minimal hooks JSON to a temp file
		dir := t.TempDir()
		hookJSON := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`
		hookFile := dir + "/settings.json"
		if err := os.WriteFile(hookFile, []byte(hookJSON), 0o644); err != nil {
			t.Fatal(err)
		}
		msg := fileBrowserDoneMsg{paths: []string{hookFile}}
		m, _ := app.Update(msg)
		app = m.(App)
		if app.importer.step != stepHookSelect {
			t.Fatalf("single .json hooks file: expected stepHookSelect, got %d", app.importer.step)
		}
		if len(app.importer.hookCandidates) == 0 {
			t.Fatal("expected hookCandidates to be populated")
		}
		if len(app.importer.hookCandidates) != len(app.importer.hookSelected) ||
			len(app.importer.hookCandidates) != len(app.importer.hookNames) {
			t.Fatalf("parallel arrays mismatched: candidates=%d selected=%d names=%d",
				len(app.importer.hookCandidates), len(app.importer.hookSelected), len(app.importer.hookNames))
		}
	})

	// EmptyProviders: From Provider with no providers stays on stepSource with error.
	t.Run("EmptyProviders", func(t *testing.T) {
		app := navigateToImport(t)
		app.importer.providers = nil
		// cursor 0 = From Provider
		m, _ := app.Update(keyEnter)
		app = m.(App)
		if app.importer.step != stepSource {
			t.Fatalf("expected to stay on stepSource with no providers, got %d", app.importer.step)
		}
		if app.importer.message == "" || !app.importer.messageIsErr {
			t.Fatal("expected error message about no providers")
		}
	})

	// FromRegistryRedirectEsc: Esc at stepGitPick when fromRegistryRedirect=true
	// sends importBackToRegistriesMsg instead of returning to stepGitURL.
	t.Run("FromRegistryRedirectEsc", func(t *testing.T) {
		app := navigateToImport(t)
		app.importer.step = stepGitPick
		app.importer.fromRegistryRedirect = true
		app.importer.clonedItems = []catalog.ContentItem{
			{Name: "test-item", Type: catalog.Skills},
		}
		_, cmd := app.Update(keyEsc)
		if cmd == nil {
			t.Fatal("expected importBackToRegistriesMsg cmd on esc from registry redirect")
		}
		msg := cmd()
		if _, ok := msg.(importBackToRegistriesMsg); !ok {
			t.Fatalf("expected importBackToRegistriesMsg, got %T", msg)
		}
	})

	// SearchResultsEntryClearsFilter and LibraryEntryClearsFilter: opening the
	// import wizard from SearchResults or Library contexts should have preFilterType=""
	// (the contexts don't set a type filter — they want all types).
	// We verify that the app's importer.preFilterType starts empty when navigated
	// normally (since navigateToImport uses the Add sidebar entry, not a type context).
	t.Run("SearchResultsEntryClearsFilter", func(t *testing.T) {
		app := navigateToImport(t)
		if app.importer.preFilterType != "" {
			t.Fatalf("expected empty preFilterType from Add entry, got %q", app.importer.preFilterType)
		}
	})
}

func TestWizardInvariantImportParallelArrays(t *testing.T) {
	// DiscoveryFilteredArrayMatch: after discoveryDoneMsg with pre-filter,
	// discoveryItems and discoverySelected must have the same length.
	t.Run("DiscoveryFilteredArrayMatch", func(t *testing.T) {
		app := navigateToImport(t)
		app.importer.step = stepProviderPick
		app.importer.preFilterType = catalog.Skills
		msg := discoveryDoneMsg{
			items: []add.DiscoveryItem{
				{Name: "rule-one", Type: catalog.Rules, Status: add.StatusNew},
				{Name: "skill-two", Type: catalog.Skills, Status: add.StatusNew},
				{Name: "skill-three", Type: catalog.Skills, Status: add.StatusOutdated},
			},
		}
		m, _ := app.Update(msg)
		app = m.(App)
		if app.importer.step != stepDiscoverySelect {
			t.Fatalf("expected stepDiscoverySelect, got %d", app.importer.step)
		}
		if len(app.importer.discoveryItems) != len(app.importer.discoverySelected) {
			t.Fatalf("array mismatch: discoveryItems=%d discoverySelected=%d",
				len(app.importer.discoveryItems), len(app.importer.discoverySelected))
		}
		// Only Skills items should survive the pre-filter (2 out of 3)
		if len(app.importer.discoveryItems) != 2 {
			t.Fatalf("expected 2 filtered items, got %d", len(app.importer.discoveryItems))
		}
	})

	// DiscoveryUnfilteredArrayMatch: without pre-filter, all items are shown.
	t.Run("DiscoveryUnfilteredArrayMatch", func(t *testing.T) {
		app := navigateToImport(t)
		app.importer.step = stepProviderPick
		msg := discoveryDoneMsg{
			items: []add.DiscoveryItem{
				{Name: "rule-one", Type: catalog.Rules, Status: add.StatusNew},
				{Name: "skill-two", Type: catalog.Skills, Status: add.StatusInLibrary},
				{Name: "agent-three", Type: catalog.Agents, Status: add.StatusOutdated},
			},
		}
		m, _ := app.Update(msg)
		app = m.(App)
		if app.importer.step != stepDiscoverySelect {
			t.Fatalf("expected stepDiscoverySelect, got %d", app.importer.step)
		}
		if len(app.importer.discoveryItems) != len(app.importer.discoverySelected) {
			t.Fatalf("array mismatch: discoveryItems=%d discoverySelected=%d",
				len(app.importer.discoveryItems), len(app.importer.discoverySelected))
		}
		if len(app.importer.discoveryItems) != 3 {
			t.Fatalf("expected 3 items, got %d", len(app.importer.discoveryItems))
		}
	})

	// HookCandidateArrayMatch: after fileBrowserDoneMsg that triggers hook splitting,
	// hookCandidates, hookSelected, and hookNames must all have the same length.
	t.Run("HookCandidateArrayMatch", func(t *testing.T) {
		app := navigateToImport(t)
		app.importer.contentType = catalog.Hooks
		app.importer.providerName = "claude-code"
		dir := t.TempDir()
		// Two hooks in the JSON
		hookJSON := `{"hooks":{"PreToolUse":[` +
			`{"matcher":"Bash","hooks":[{"type":"command","command":"echo a"}]},` +
			`{"matcher":"Edit","hooks":[{"type":"command","command":"echo b"}]}` +
			`]}}`
		hookFile := dir + "/settings.json"
		if err := os.WriteFile(hookFile, []byte(hookJSON), 0o644); err != nil {
			t.Fatal(err)
		}
		msg := fileBrowserDoneMsg{paths: []string{hookFile}}
		m, _ := app.Update(msg)
		app = m.(App)
		if app.importer.step != stepHookSelect {
			t.Fatalf("expected stepHookSelect, got %d", app.importer.step)
		}
		nc := len(app.importer.hookCandidates)
		ns := len(app.importer.hookSelected)
		nn := len(app.importer.hookNames)
		if nc != ns || nc != nn {
			t.Fatalf("hookCandidates=%d hookSelected=%d hookNames=%d — arrays mismatched", nc, ns, nn)
		}
	})
}
```

Note: the `HooksJsonDetection` and `HookCandidateArrayMatch` tests both use `fileBrowserDoneMsg` directly (not a key press), which is how the file browser returns selections to the importer. The `os` package is already imported by the test file via `import_test.go`'s package-level imports — since all `_test.go` files in package `tui` share imports within the test binary, `os` will be available. Add `"os"` to the import block in `wizard_invariant_test.go` if the linter complains.

### Step 2: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestWizardInvariantImportSpecial|TestWizardInvariantImportParallel" -count=1 -v`
Expected: PASS

---

## Task 7b: Wizard invariant tests — Import Conflict/Batch sub-matrix

**Files:**
- Modify: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Task 5

**Success Criteria:**
- [ ] 4 paths from design doc: single no-conflict, single conflict, batch no-conflicts, batch some-conflicts
- [ ] Tests use real temp-dir files so conflict detection (os.Stat on dest) works
- [ ] Tests exercise the full path to `importDoneMsg` or `stepConflict` as appropriate

---

### Step 1: Add conflict/batch sub-matrix tests

How conflicts work (from import.go):
- `stepValidate → Enter`: single selection → `stepConfirm`; batch with conflicts → `stepConflict`; batch no conflicts → `importDoneMsg` (async cmd)
- `stepConfirm → Enter`: if dest exists → `stepConflict`; else → `importDoneMsg` (async cmd)
- `stepConflict → y`: overwrite; for batch, calls `advanceConflict()` which either shows next conflict or fires `importDoneMsg` cmd; for single, fires `importDoneMsg` cmd directly
- The `conflict` field and `batchConflicts` field distinguish single vs batch conflict paths

```go
func TestWizardInvariantImportConflictBatch(t *testing.T) {
	// Setup helpers: create a source directory and a destination directory
	// to simulate an existing item (conflict scenario).

	t.Run("Single_NoConflict_ValidateToConfirmToCmd", func(t *testing.T) {
		// stepValidate → Enter (single item, no conflict) → stepConfirm
		// At stepConfirm, the dest does not exist → Enter fires importDoneMsg cmd
		app := navigateToImport(t)
		dir := t.TempDir()
		// Create a source skill directory
		srcDir := filepath.Join(dir, "my-skill")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		app.importer.step = stepValidate
		app.importer.contentType = catalog.Skills
		app.importer.selectedPaths = []string{srcDir}
		app.importer.validationItems = []validationItem{
			{path: srcDir, name: "my-skill", included: true},
		}
		app.importer.validateCursor = 0
		m, _ := app.Update(keyEnter) // → stepConfirm (single item)
		app = m.(App)
		if app.importer.step != stepConfirm {
			t.Fatalf("expected stepConfirm, got %d", app.importer.step)
		}
		// At stepConfirm: set sourcePath/itemName and confirm
		app.importer.sourcePath = srcDir
		app.importer.itemName = "my-skill"
		app.importer.repoRoot = dir
		_, cmd := app.Update(keyEnter) // → importDoneMsg cmd (no conflict since dest doesn't exist)
		if cmd == nil {
			t.Fatal("expected importDoneMsg cmd from stepConfirm (no conflict)")
		}
	})

	t.Run("Single_Conflict_ConfirmToConflict", func(t *testing.T) {
		// stepConfirm → Enter when dest already exists → stepConflict
		app := navigateToImport(t)
		dir := t.TempDir()
		// Create both source and destination directories
		srcDir := filepath.Join(dir, "source", "my-skill")
		destDir := filepath.Join(dir, "dest", "skills", "my-skill")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			t.Fatal(err)
		}
		app.importer.step = stepConfirm
		app.importer.contentType = catalog.Skills
		app.importer.sourcePath = srcDir
		app.importer.itemName = "my-skill"
		app.importer.repoRoot = dir
		app.importer.providerName = ""
		// Patch destinationPath to return destDir by setting repoRoot appropriately.
		// destinationPath() builds: repoRoot/contentType/itemName for universal types.
		// So set repoRoot to dir/dest and the path will be dir/dest/skills/my-skill.
		app.importer.repoRoot = filepath.Join(dir, "dest")
		m, _ := app.Update(keyEnter)
		app = m.(App)
		if app.importer.step != stepConflict {
			t.Fatalf("expected stepConflict when dest exists, got %d", app.importer.step)
		}
		if app.importer.conflict == (conflictInfo{}) {
			t.Fatal("expected conflict to be populated at stepConflict")
		}
	})

	t.Run("Batch_NoConflicts_ValidateToCmd", func(t *testing.T) {
		// stepValidate → Enter (multiple items, no conflicts) → importDoneMsg cmd
		app := navigateToImport(t)
		dir := t.TempDir()
		// Two source dirs, no destinations exist
		src1 := filepath.Join(dir, "skill-one")
		src2 := filepath.Join(dir, "skill-two")
		if err := os.MkdirAll(src1, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(src2, 0o755); err != nil {
			t.Fatal(err)
		}
		app.importer.step = stepValidate
		app.importer.contentType = catalog.Skills
		app.importer.repoRoot = filepath.Join(dir, "dest") // dest doesn't exist
		app.importer.selectedPaths = []string{src1, src2}
		app.importer.validationItems = []validationItem{
			{path: src1, name: "skill-one", included: true},
			{path: src2, name: "skill-two", included: true},
		}
		_, cmd := app.Update(keyEnter) // batch, no conflicts → importDoneMsg cmd
		if cmd == nil {
			t.Fatal("expected importDoneMsg cmd for batch with no conflicts")
		}
	})

	t.Run("Batch_SomeConflicts_ValidateToConflict", func(t *testing.T) {
		// stepValidate → Enter (multiple items, one conflict) → stepConflict
		// After stepConflict → y (overwrite) → advanceConflict fires importDoneMsg cmd
		app := navigateToImport(t)
		dir := t.TempDir()
		src1 := filepath.Join(dir, "source", "skill-one")
		src2 := filepath.Join(dir, "source", "skill-two")
		// Create source dirs
		if err := os.MkdirAll(src1, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(src2, 0o755); err != nil {
			t.Fatal(err)
		}
		destRoot := filepath.Join(dir, "dest")
		// Create dest for skill-one only (conflict)
		dest1 := filepath.Join(destRoot, "skills", "skill-one")
		if err := os.MkdirAll(dest1, 0o755); err != nil {
			t.Fatal(err)
		}
		app.importer.step = stepValidate
		app.importer.contentType = catalog.Skills
		app.importer.repoRoot = destRoot
		app.importer.selectedPaths = []string{src1, src2}
		app.importer.validationItems = []validationItem{
			{path: src1, name: "skill-one", included: true},
			{path: src2, name: "skill-two", included: true},
		}
		m, _ := app.Update(keyEnter) // batch with conflict → stepConflict
		app = m.(App)
		if app.importer.step != stepConflict {
			t.Fatalf("expected stepConflict for batch with conflict, got %d", app.importer.step)
		}
		if len(app.importer.batchConflicts) == 0 {
			t.Fatal("expected batchConflicts to be populated")
		}
		// Press 'y' to overwrite → advanceConflict → all conflicts resolved → importDoneMsg cmd
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
		if cmd == nil {
			t.Fatal("expected importDoneMsg cmd after overwrite resolves last batch conflict")
		}
	})
}
```

Note: `filepath` is already used in `import_test.go` (same package), so it is available. Add `"path/filepath"` to `wizard_invariant_test.go`'s import block. The `tea` package is needed for the `tea.KeyMsg` literal in the last subtest — add `tea "github.com/charmbracelet/bubbletea"` to the imports.

### Step 2: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestWizardInvariantImportConflict" -count=1 -v`
Expected: PASS with 4 subtests

---

## Task 8: Wizard invariant tests — Loadout Create wizard

**Files:**
- Modify: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Task 2, Task 5

**Success Criteria:**
- [ ] Forward paths: no pre-fill, pre-filled provider, with scope registry
- [ ] Esc paths at each step
- [ ] ~8-10 test cases

---

### Step 1: Add loadout create wizard tests

```go
func TestWizardInvariantCreateLoadoutForward(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("NoPrefill", func(t *testing.T) {
		// clStepProvider → Enter → clStepTypes → Enter → clStepItems → Enter → clStepName
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		if s.step != clStepProvider {
			t.Fatalf("expected clStepProvider at start, got %d", s.step)
		}
		s, _ = s.Update(keyEnter) // provider → types
		if s.step != clStepTypes {
			t.Fatalf("expected clStepTypes after provider Enter, got %d", s.step)
		}
		s, _ = s.Update(keyEnter) // types → items (first type auto-selected)
		if s.step != clStepItems {
			t.Fatalf("expected clStepItems after types Enter, got %d", s.step)
		}
		s, _ = s.Update(keyEnter) // items → name (or next type if multi-type)
		// After items Enter, step is clStepName (single type) or stays clStepItems (next type)
		// Walk through all types until we reach clStepName
		for s.step == clStepItems {
			s, _ = s.Update(keyEnter)
		}
		if s.step != clStepName {
			t.Fatalf("expected clStepName after all items, got %d", s.step)
		}
		s.nameInput.SetValue("my-loadout")
		s, _ = s.Update(keyEnter) // name → dest
		if s.step != clStepDest {
			t.Fatalf("expected clStepDest after name Enter, got %d", s.step)
		}
		s, _ = s.Update(keyEnter) // dest → review
		if s.step != clStepReview {
			t.Fatalf("expected clStepReview after dest Enter, got %d", s.step)
		}
	})

	t.Run("PrefilledProvider", func(t *testing.T) {
		// Pre-filled: starts at clStepTypes, skips provider
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		if s.step != clStepTypes {
			t.Fatalf("expected clStepTypes at start with prefilled provider, got %d", s.step)
		}
		s, _ = s.Update(keyEnter) // types → items
		if s.step != clStepItems {
			t.Fatalf("expected clStepItems, got %d", s.step)
		}
		for s.step == clStepItems {
			s, _ = s.Update(keyEnter)
		}
		if s.step != clStepName {
			t.Fatalf("expected clStepName, got %d", s.step)
		}
	})

	t.Run("WithScopeRegistry", func(t *testing.T) {
		// With registry scope: destOptions has 3 entries (project, global, registry)
		s := newCreateLoadoutScreen("claude-code", "my-registry", providers, cat, 80, 30)
		if len(s.destOptions) != 3 {
			t.Fatalf("expected 3 destOptions with registry scope, got %d", len(s.destOptions))
		}
		// Walk to dest step and verify all 3 options are navigable
		s.step = clStepDest
		s, _ = s.Update(keyDown)
		s, _ = s.Update(keyDown)
		if s.destCursor != 2 {
			t.Fatalf("expected destCursor 2, got %d", s.destCursor)
		}
	})
}

func TestWizardInvariantCreateLoadoutEsc(t *testing.T) {
	cat := testCatalog(t)
	providers := testProviders(t)

	t.Run("TypesEsc_Prefilled_ExitsWizard", func(t *testing.T) {
		// When pre-filled, Esc at clStepTypes exits (no provider step to go back to)
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		if s.step != clStepTypes {
			t.Fatalf("expected clStepTypes, got %d", s.step)
		}
		s, _ = s.Update(keyEsc)
		// confirmed=false is the exit signal; step stays or wizard exits
		// Esc at first step when pre-filled should not crash and should not confirm
		if s.confirmed {
			t.Error("Esc should not confirm the wizard")
		}
	})

	t.Run("TypesEsc_NoPrefill_GoesBackToProvider", func(t *testing.T) {
		s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
		s.step = clStepTypes
		// Set prefilledProvider to empty to ensure "no prefill" path
		s.prefilledProvider = ""
		s, _ = s.Update(keyEsc)
		if s.step != clStepProvider {
			t.Fatalf("expected clStepProvider after Esc on types (no prefill), got %d", s.step)
		}
	})

	t.Run("NameEsc_GoesBackToItems", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepName
		s, _ = s.Update(keyEnter) // items → name already done; now Esc from name
		// Actually set the step directly so we don't need full navigation
		s.step = clStepName
		s, _ = s.Update(keyEsc)
		if s.step != clStepItems {
			t.Fatalf("expected clStepItems after Esc on name, got %d", s.step)
		}
	})

	t.Run("DestEsc_GoesBackToName", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepDest
		s, _ = s.Update(keyEsc)
		if s.step != clStepName {
			t.Fatalf("expected clStepName after Esc on dest, got %d", s.step)
		}
	})

	t.Run("ReviewEsc_GoesBackToDest", func(t *testing.T) {
		s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
		s.step = clStepReview
		s.reviewBtnCursor = 0 // Back button
		s, _ = s.Update(keyEnter) // Review Back → dest
		if s.step != clStepDest {
			t.Fatalf("expected clStepDest after Review Back, got %d", s.step)
		}
	})
}
```

### Step 2: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestWizardInvariantCreateLoadout" -count=1 -v`
Expected: PASS

---

## Task 9: Wizard invariant tests — Install, EnvSetup, Update modals

**Files:**
- Modify: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Tasks 3-4, Task 5

**Success Criteria:**
- [ ] Install modal: normal, custom path, symlink disabled paths
- [ ] Env setup modal: new value path, existing file path, multiple vars
- [ ] Update wizard: full flow, direct update
- [ ] ~10-12 test cases

---

### Step 1: Add install modal tests

```go
func TestWizardInvariantInstallModal(t *testing.T) {
	providers := testProviders(t)
	cat := testCatalog(t)
	item := cat.Items[0] // any item (e.g., alpha-skill)

	t.Run("NormalFlow_LocationToMethod", func(t *testing.T) {
		// installStepLocation → Enter (cursor 0 = global) → installStepMethod
		m := newInstallModal(item, providers, cat.RepoRoot)
		if m.step != installStepLocation {
			t.Fatalf("expected installStepLocation, got %d", m.step)
		}
		// locationCursor 0 = global — Enter goes directly to installStepMethod
		m, _ = m.Update(keyEnter)
		if m.step != installStepMethod {
			t.Fatalf("expected installStepMethod after Enter on global, got %d", m.step)
		}
	})

	t.Run("CustomPath_LocationToCustomToMethod", func(t *testing.T) {
		// installStepLocation → cursor 2 (Custom) → Enter → installStepCustomPath
		// → type path → Enter → installStepMethod
		m := newInstallModal(item, providers, cat.RepoRoot)
		// Navigate to Custom (cursor 2)
		m, _ = m.Update(keyDown)
		m, _ = m.Update(keyDown)
		if m.locationCursor != 2 {
			t.Fatalf("expected locationCursor 2, got %d", m.locationCursor)
		}
		m, _ = m.Update(keyEnter) // → installStepCustomPath
		if m.step != installStepCustomPath {
			t.Fatalf("expected installStepCustomPath, got %d", m.step)
		}
		// Type a path and confirm
		m.customPathInput.SetValue("/tmp/test-install")
		m, _ = m.Update(keyEnter) // → installStepMethod
		if m.step != installStepMethod {
			t.Fatalf("expected installStepMethod after custom path Enter, got %d", m.step)
		}
	})

	t.Run("SymlinkDisabled_MethodCursorDefaultsToCopy", func(t *testing.T) {
		// When symlink is disabled for the content type, methodCursor defaults to 1 (copy)
		// Use a provider that explicitly disables symlinks for the item's type
		disableSymlinkProviders := []provider.Provider{
			{
				Name:  "Claude Code",
				Slug:  "claude-code",
				SymlinkSupport: map[catalog.ContentType]bool{
					item.Type: false,
				},
			},
		}
		m := newInstallModal(item, disableSymlinkProviders, cat.RepoRoot)
		m, _ = m.Update(keyEnter) // location → method
		if m.step != installStepMethod {
			t.Fatalf("expected installStepMethod, got %d", m.step)
		}
		if m.methodCursor != 1 {
			t.Fatalf("expected methodCursor 1 (copy) when symlink disabled, got %d", m.methodCursor)
		}
	})

	t.Run("MethodEsc_GoesBackToLocation", func(t *testing.T) {
		m := newInstallModal(item, providers, cat.RepoRoot)
		m.step = installStepMethod
		m, _ = m.Update(keyEsc)
		if m.step != installStepLocation {
			t.Fatalf("expected installStepLocation after Esc on method, got %d", m.step)
		}
	})

	t.Run("LocationEsc_ClosesModal", func(t *testing.T) {
		m := newInstallModal(item, providers, cat.RepoRoot)
		m, _ = m.Update(keyEsc)
		if m.active {
			t.Fatal("Esc at location step should deactivate modal")
		}
	})
}
```

### Step 2: Add env setup modal tests

```go
func TestWizardInvariantEnvSetupModal(t *testing.T) {
	t.Run("NewValuePath_ChooseToValueToLocation", func(t *testing.T) {
		// envStepChoose (methodCursor=0 = set up new) → Enter → envStepValue
		// → Enter with value → envStepLocation
		m := newEnvSetupModal([]string{"TEST_API_KEY"})
		if m.step != envStepChoose {
			t.Fatalf("expected envStepChoose, got %d", m.step)
		}
		// methodCursor=0 = "Set up new value" — press Enter
		m, _ = m.Update(keyEnter)
		if m.step != envStepValue {
			t.Fatalf("expected envStepValue after Choose Enter (new value), got %d", m.step)
		}
		// Enter a value and confirm
		m.input.SetValue("my-secret-key")
		m, _ = m.Update(keyEnter)
		if m.step != envStepLocation {
			t.Fatalf("expected envStepLocation after value Enter, got %d", m.step)
		}
		if m.value != "my-secret-key" {
			t.Fatalf("expected value %q, got %q", "my-secret-key", m.value)
		}
	})

	t.Run("ExistingFilePath_ChooseToSource", func(t *testing.T) {
		// envStepChoose (methodCursor=1 = already configured) → Enter → envStepSource
		m := newEnvSetupModal([]string{"TEST_SECRET"})
		// Navigate to "Already configured" (cursor 1)
		m, _ = m.Update(keyDown)
		if m.methodCursor != 1 {
			t.Fatalf("expected methodCursor 1, got %d", m.methodCursor)
		}
		m, _ = m.Update(keyEnter)
		if m.step != envStepSource {
			t.Fatalf("expected envStepSource after Choose Enter (already configured), got %d", m.step)
		}
	})

	t.Run("MultipleVars_IteratesEachVar", func(t *testing.T) {
		// With 2 vars, completing the first should advance varIdx and reset to envStepChoose
		m := newEnvSetupModal([]string{"VAR_ONE", "VAR_TWO"})
		if m.varIdx != 0 {
			t.Fatalf("expected varIdx=0, got %d", m.varIdx)
		}
		if len(m.varNames) != 2 {
			t.Fatalf("expected 2 varNames, got %d", len(m.varNames))
		}
		// Advance through VAR_ONE: choose → value → location → (skip location to advance)
		m, _ = m.Update(keyEnter) // choose → value
		m.input.SetValue("val1")
		m, _ = m.Update(keyEnter) // value → location
		// Esc at location skips the var and advances
		m, _ = m.Update(keyEsc)
		if !m.active {
			t.Fatal("modal should still be active after first var (has second var)")
		}
		if m.varIdx != 1 {
			t.Fatalf("expected varIdx=1 after first var, got %d", m.varIdx)
		}
		if m.step != envStepChoose {
			t.Fatalf("expected envStepChoose for second var, got %d", m.step)
		}
	})
}
```

### Step 3: Add update wizard tests

```go
func TestWizardInvariantUpdateWizard(t *testing.T) {
	t.Run("PreviewFlow_MenuToPreview", func(t *testing.T) {
		// stepUpdateMenu → cursor 0 ("See what's new") → async → stepUpdatePreview
		// The async part returns a tea.Cmd, not a step transition.
		// We test the transition by injecting updatePreviewMsg directly.
		m := newUpdateModel("", "v0.1.0", "v0.2.0", 5, false)
		if m.step != stepUpdateMenu {
			t.Fatalf("expected stepUpdateMenu, got %d", m.step)
		}
		// Simulate the async result arriving
		m, _ = m.Update(updatePreviewMsg{
			releaseNotes: "## v0.2.0\n\n- Feature X\n",
			versionRange: "v0.1.0 → v0.2.0",
		})
		if m.step != stepUpdatePreview {
			t.Fatalf("expected stepUpdatePreview after updatePreviewMsg, got %d", m.step)
		}
		if m.releaseNotes == "" && m.fallbackLog == "" {
			t.Fatal("expected releaseNotes or fallbackLog to be set at stepUpdatePreview")
		}
	})

	t.Run("DirectUpdate_MenuToPull", func(t *testing.T) {
		// stepUpdateMenu → cursor 1 ("Update now") → dispatches async → stepUpdatePull
		// Simulate the update available path: cursor 1 = "Update now"
		m := newUpdateModel("", "v0.1.0", "v0.2.0", 5, false)
		m, _ = m.Update(keyDown) // cursor 1 = Update now
		if m.cursor != 1 {
			t.Fatalf("expected cursor 1, got %d", m.cursor)
		}
		_, cmd := m.Update(keyEnter)
		if cmd == nil {
			t.Fatal("expected async cmd for Update now")
		}
		// Simulate the pull completing
		m, _ = m.Update(updatePullMsg{output: "Already up to date."})
		if m.step != stepUpdateDone {
			t.Fatalf("expected stepUpdateDone after updatePullMsg, got %d", m.step)
		}
	})
}
```

### Step 4: Verify

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -run "TestWizardInvariant(Install|EnvSetup|Update)" -count=1 -v`
Expected: PASS

---

## Task 10: PostToolUse hook script

**Files:**
- Create: `.claude/hooks/wizard-invariant-gate.sh`
- Modify: `.claude/settings.json` (add PostToolUse entry)

**Depends on:** Tasks 1-4 (validateStep methods exist so tests compile)

**Success Criteria:**
- [ ] Hook triggers on Edit/Write to any `cli/internal/tui/*.go` file
- [ ] Runs `go test -run TestWizardInvariant` on trigger
- [ ] Shows test output to Claude on failure
- [ ] Passes silently on success
- [ ] Does NOT block edits (PostToolUse is feedback, not blocking)

---

### Step 1: Create hook script

Create `.claude/hooks/wizard-invariant-gate.sh`:

```bash
#!/usr/bin/env bash
# PostToolUse hook: Run wizard invariant tests after any TUI file edit.
# Provides immediate feedback — NOT a hard gate (PostToolUse cannot undo edits).

set -euo pipefail

INPUT=$(cat)

FILE_PATH=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    fp = data.get('input', {}).get('file_path', '')
    print(fp)
except (json.JSONDecodeError, KeyError, TypeError):
    print('')
" 2>/dev/null || echo "")

# Only trigger on cli/internal/tui/*.go files
if [[ ! "$FILE_PATH" =~ cli/internal/tui/[^/]+\.go$ ]]; then
    exit 0
fi

# Run wizard invariant tests
cd "$(git rev-parse --show-toplevel)/cli" && go test ./internal/tui/ -run "TestWizardInvariant" -count=1 2>&1
```

### Step 2: Edit settings.json to add PostToolUse

First, read `.claude/settings.json` to see the existing content. As of this writing it contains three PreToolUse entries. Edit the file to add a `PostToolUse` key alongside the existing `PreToolUse` key. The full merged result should look like:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "python3 \"$(git rev-parse --show-toplevel)/.claude/hooks/release-guard.py\""
          }
        ]
      },
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "bash \"$(git rev-parse --show-toplevel)/.claude/hooks/tui-convention-check.sh\""
          }
        ]
      },
      {
        "matcher": "Read",
        "hooks": [
          {
            "type": "command",
            "command": "bash \"$(git rev-parse --show-toplevel)/.claude/hooks/tui-rules-reminder.sh\""
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "bash \"$(git rev-parse --show-toplevel)/.claude/hooks/wizard-invariant-gate.sh\""
          }
        ]
      }
    ]
  }
}
```

Do not replace the file wholesale — use Edit to add the `"PostToolUse": [...]` block after the closing `]` of the existing `PreToolUse` array. Read the file first to confirm its current exact content before editing, in case it has changed since this plan was written.

### Step 3: Verify

Edit any TUI file → hook should run and show test results. Verify with a trivial edit (add/remove blank line).

---

## Task 11: Claude rule file

**Files:**
- Create: `.claude/rules/tui-wizard-patterns.md`

**Depends on:** Nothing

**Success Criteria:**
- [ ] Documents the wizard step machine pattern
- [ ] Documents what validateStep() checks
- [ ] Includes checklist for adding a new wizard
- [ ] Auto-loads when any TUI file is touched (via .claude/rules/ convention)

---

### Step 1: Create the rule file

Create `.claude/rules/tui-wizard-patterns.md` documenting:

1. **Pattern overview:** Wizards use typed step enums, sequential transitions, validateStep() at top of Update()
2. **validateStep() scope:** Entry-prerequisites ONLY. Not cursor positions, not step-output state, not constructor state.
3. **Modal vs full-screen:** Modal wizards place validateStep after `if !m.active` guard. Full-screen wizards call unconditionally.
4. **Checklist for adding a new wizard:**
   - Define step enum with iota
   - Add validateStep() method with entry-prerequisites per step
   - Call validateStep() at top of Update()
   - Add forward-path tests to wizard_invariant_test.go
   - Add Esc/back path tests
   - Add special case tests for conditional branches

### Step 2: Verify

Run: `ls -la .claude/rules/tui-wizard-patterns.md` — file exists.

---

## Task 12: Full test suite verification + commit

**Files:**
- No new changes

**Depends on:** All previous tasks

**Success Criteria:**
- [ ] `make test` passes (full suite including wizard invariants)
- [ ] Golden files regenerated if needed
- [ ] All changes committed

---

### Step 1: Run full test suite

Run: `cd /home/hhewett/.local/src/syllago && make test`
Expected: PASS

### Step 2: Regenerate golden files if needed

Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -update-golden`

### Step 3: Commit

```bash
git add cli/internal/tui/import.go cli/internal/tui/loadout_create.go cli/internal/tui/modal.go cli/internal/tui/update.go
git add cli/internal/tui/wizard_invariant_test.go
git add .claude/hooks/wizard-invariant-gate.sh .claude/settings.json .claude/rules/tui-wizard-patterns.md
git commit -m "feat(tui): add wizard step machine enforcement

Add validateStep() production assertions to all 5 wizards, exhaustive
invariant test matrix (~75 tests), PostToolUse hook for immediate
feedback, and Claude rule documenting the pattern.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Dependency Graph

```
Tasks 1-4 (validateStep methods) — independent, can run in parallel
    │
    v
Task 5 (test file + import forward paths) — needs Tasks 1-4
    │
    ├──> Task 6 (import esc paths)
    ├──> Task 7 (import special cases + parallel arrays)
    ├──> Task 7b (import conflict/batch sub-matrix)
    ├──> Task 8 (loadout create tests)
    └──> Task 9 (install/env/update tests)
              │
              v
         Task 10 (hook) — needs tests to exist
         Task 11 (rule) — independent
              │
              v
         Task 12 (verify + commit)
```

**Parallel opportunities:**
- Tasks 1, 2, 3, 4 can all run in parallel
- Tasks 6, 7, 7b, 8, 9 can all run in parallel (after Task 5)
- Tasks 10, 11 can run in parallel
