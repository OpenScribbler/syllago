# TUI-CLI Sync — Implementation Plan

**Design doc:** `docs/plans/2026-03-09-tui-cli-sync-design.md`
**Date:** 2026-03-09

---

## Phase 1: Terminology Cleanup

### Task 1: Rename "Import" strings in import.go

**Title:** Rename "Import" user-facing strings to "Add" in import.go
**Depends on:** Nothing
**Files:** `cli/internal/tui/import.go`

**Success criteria:**
- All user-visible "Import" / "import" strings updated to "Add" / "add"
- Go identifiers (function names, type names, variable names) remain unchanged
- `make build` succeeds

**Changes:**

Line 698 — breadcrumb heading:
```go
// Before:
s := zone.Mark("crumb-home", helpStyle.Render("Home")) + " " + helpStyle.Render(">") + " " + titleStyle.Render("Import AI Tools") + "\n"
// After:
s := zone.Mark("crumb-home", helpStyle.Render("Home")) + " " + helpStyle.Render(">") + " " + titleStyle.Render("Add Content") + "\n"
```

Line 710 — help text (stepSource description):
```go
// Before:
s += helpStyle.Render("Bring in skills, agents, prompts, rules, hooks, commands, and MCP configs") + "\n"
s += helpStyle.Render("from your filesystem or a git repository. Create New scaffolds a blank template.") + "\n\n"
// After:
s += helpStyle.Render("Add content from an installed provider, local files, or a git repository.") + "\n"
s += helpStyle.Render("Create New scaffolds a blank template.") + "\n\n"
```

Line 870 — hook select inline action text:
```go
// Before:
s += fmt.Sprintf("  Import Selected (%d)  •  space toggle  •  a all  •  n none\n", count)
// After (will be fully replaced in Phase 2 Task 12, but rename for now):
s += fmt.Sprintf("  Add Selected (%d)  •  space toggle  •  a all  •  n none\n", count)
```

Line 899 — help footer (stepConfirm, non-create):
```go
// Before:
return "enter import • esc back"
// After:
return "enter add • esc back"
```

Line 901 — help footer (stepValidate):
```go
// Before:
return "up/down navigate • space toggle • enter import • esc back"
// After:
return "up/down navigate • space toggle • enter add • esc back"
```

Line 916 — help footer (stepHookSelect):
```go
// Before:
return "up/down navigate • space toggle • a all • n none • enter import • esc back"
// After:
return "up/down navigate • space toggle • a all • n none • enter add • esc back"
```

---

### Task 2: Rename "Import" strings in app.go

**Title:** Rename "Import" user-facing strings to "Add" in app.go
**Depends on:** Nothing
**Files:** `cli/internal/tui/app.go`

**Success criteria:**
- Welcome card, first-run text, and breadcrumb updated
- `make build` succeeds

**Changes:**

Line 1469 — first-run getting-started step:
```go
// Before:
{"1.", "Import existing content:", "syllago import --from claude-code"},
// After:
{"1.", "Add existing content:", "syllago add --from claude-code"},
```

Line 1523 — welcome card:
```go
// Before:
{"Import", "Import your own AI tools from local files or git repos", "welcome-import"},
// After:
{"Add", "Add content from providers, local files, or git repos", "welcome-import"},
```

Line 1742 — breadcrumb text:
```go
// Before:
return "Import"
// After:
return "Add"
```

---

### Task 3: Rename "Import" in sidebar.go and settings.go

**Title:** Rename "Import" references in sidebar.go and settings.go
**Depends on:** Nothing
**Files:** `cli/internal/tui/sidebar.go`, `cli/internal/tui/settings.go`

**Success criteria:**
- Comment in sidebar.go updated
- Settings description text updated
- `make build` succeeds

**Changes:**

`sidebar.go` line 125 — comment:
```go
// Before:
// Utility items: Import, Update, Settings, Registries
// After:
// Utility items: Add, Update, Settings, Registries
```

`settings.go` line 202 — provider description:
```go
// Before:
"Providers are AI coding tools (Claude Code, Cursor, Gemini CLI, etc.).\nEnable the ones you use -- syllago imports their existing configs\nand can export your catalog items back to them.",
// After:
"Providers are AI coding tools (Claude Code, Cursor, Gemini CLI, etc.).\nEnable the ones you use -- syllago adds their existing content\nand can export your catalog items back to them.",
```

---

### Task 4: Update tests for terminology changes

**Title:** Update test assertions for "Import" -> "Add" terminology
**Depends on:** Task 1, Task 2
**Files:** `cli/internal/tui/app_test.go`, `cli/internal/tui/import_test.go`, `cli/internal/tui/integration_test.go`

**Success criteria:**
- All test assertions match new strings
- `cd cli && go test ./internal/tui/ -run TestFirstRun` passes
- `cd cli && go test ./internal/tui/ -run TestImportView` passes

**Changes:**

`app_test.go` lines 88-89:
```go
// Before:
if !strings.Contains(view, "syllago import") {
    t.Error("first-run screen should show 'syllago import' step")
// After:
if !strings.Contains(view, "syllago add") {
    t.Error("first-run screen should show 'syllago add' step")
```

`import_test.go` line 629:
```go
// Before:
assertContains(t, view, "Import AI Tools")
// After:
assertContains(t, view, "Add Content")
```

`integration_test.go` line 171:
```go
// Before:
waitFor(t, tm, "Import AI Tools")
// After:
waitFor(t, tm, "Add Content")
```

---

### Task 5: Regenerate golden files and run full test suite

**Title:** Regenerate golden files after terminology changes
**Depends on:** Task 1, Task 2, Task 3, Task 4
**Files:** `cli/internal/tui/testdata/*.golden`

**Success criteria:**
- `cd cli && go test ./internal/tui/ -update-golden` succeeds
- `cd cli && make test` passes (all tests green)
- Git diff of golden files shows only "Import" -> "Add" and related string changes

**Commands:**
```bash
cd /home/hhewett/.local/src/syllago && make build
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -update-golden
cd /home/hhewett/.local/src/syllago/cli && make test
```

---

### Task 6: Commit Phase 1

**Title:** Commit Phase 1 terminology cleanup
**Depends on:** Task 5

**Success criteria:**
- Clean commit with message: `refactor(tui): rename "Import" to "Add" throughout TUI`
- All tests still pass after commit

---

## Phase 2: Provider Discovery Integration

### Task 7: Add new import steps and model fields

**Title:** Add stepProviderPick, stepDiscoverySelect, discoveryDoneMsg, and new model fields
**Depends on:** Task 6
**Files:** `cli/internal/tui/import.go`

**Success criteria:**
- New enum values compile
- New fields on importModel compile
- New message type compiles
- `make build` succeeds

**Changes:**

Add two new step values to the `importStep` enum (after `stepHookSelect`):
```go
stepHookSelect                    // (hook import only) multi-select which hooks to import
stepProviderPick                  // (provider discovery) pick detected provider
stepDiscoverySelect               // (provider discovery) multi-select discovered items
```

Add new import to the import block:
```go
"github.com/OpenScribbler/syllago/cli/internal/add"
"github.com/OpenScribbler/syllago/cli/internal/catalog"
"github.com/OpenScribbler/syllago/cli/internal/config"
```

Add message type after `importDoneMsg`:
```go
type discoveryDoneMsg struct {
	items []add.DiscoveryItem
	err   error
}
```

Add fields to `importModel` struct (after the hook import state block):
```go
// Provider discovery state
discoveryProvCursor int                 // cursor for stepProviderPick
discoveryItems      []add.DiscoveryItem // results from DiscoverFromProvider
discoverySelected   []bool              // checkbox state per item
discoveryCursor     int                 // cursor for stepDiscoverySelect
discoveryProvider   provider.Provider   // selected provider
```

---

### Task 8: Write tests for stepSource "From Provider" option

**Title:** Test "From Provider" as first source option, cursor index shift
**Depends on:** Task 7
**Files:** `cli/internal/tui/import_test.go`

**Success criteria:**
- Tests written and failing (red phase — source step still has 3 options)

**Tests to add:**

```go
func TestImportSourceFromProviderOption(t *testing.T) {
	app := navigateToImport(t)
	// "From Provider" should be at cursor 0, enter goes to stepProviderPick
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepProviderPick {
		t.Fatalf("expected stepProviderPick, got %d", app.importer.step)
	}
}

func TestImportSourceLocalShiftedToIndex1(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 1) // Local is now index 1
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after local (index 1), got %d", app.importer.step)
	}
	if app.importer.isCreate {
		t.Fatal("isCreate should be false for local")
	}
}

func TestImportSourceGitShiftedToIndex2(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 2) // Git is now index 2
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepGitURL {
		t.Fatalf("expected stepGitURL after git (index 2), got %d", app.importer.step)
	}
}

func TestImportSourceCreateShiftedToIndex3(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 3) // Create is now index 3
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after create (index 3), got %d", app.importer.step)
	}
	if !app.importer.isCreate {
		t.Fatal("isCreate should be true for create")
	}
}

func TestImportSourceBoundsClampAt3(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 10) // Way past end
	if app.importer.sourceCursor != 3 {
		t.Fatalf("expected sourceCursor clamped at 3, got %d", app.importer.sourceCursor)
	}
}

func TestImportSourceFromProviderNoProviders(t *testing.T) {
	app := navigateToImport(t)
	app.importer.providers = nil // empty provider list

	m, _ := app.Update(keyEnter) // cursor 0 = From Provider
	app = m.(App)
	if app.importer.step != stepSource {
		t.Fatalf("expected to stay on stepSource with no providers, got %d", app.importer.step)
	}
	if app.importer.message == "" || !app.importer.messageIsErr {
		t.Fatal("expected error message about no providers detected")
	}
}
```

---

### Task 9: Implement "From Provider" source option

**Title:** Add "From Provider" as first source option, shift existing indices
**Depends on:** Task 8
**Files:** `cli/internal/tui/import.go`, `cli/internal/tui/import_test.go`

**Success criteria:**
- "From Provider" renders as first item in stepSource
- Cursor indices: 0=From Provider, 1=Local Path, 2=Git URL, 3=Create New
- Bounds clamp updated from 2 to 3
- Task 8 tests pass
- Existing tests updated for shifted indices

**Changes in import.go:**

`updateSource` — update bounds and case mapping:
```go
func (m importModel) updateSource(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.sourceCursor > 0 {
			m.sourceCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.sourceCursor < 3 {
			m.sourceCursor++
		}
	case key.Matches(msg, keys.Enter):
		switch m.sourceCursor {
		case 0: // From Provider
			if len(m.providers) == 0 {
				m.message = "No providers detected. Install a supported AI coding tool first."
				m.messageIsErr = true
				return m, nil
			}
			m.discoveryProvCursor = 0
			m.step = stepProviderPick
		case 1: // Local path
			m.isCreate = false
			m.step = stepType
			m.typeCursor = 0
		case 2: // Git URL
			m.isCreate = false
			m.step = stepGitURL
			m.urlInput.SetValue("")
			m.urlInput.Focus()
		case 3: // Create New
			m.isCreate = true
			m.step = stepType
			m.typeCursor = 0
		}
	}
	return m, nil
}
```

`View()` stepSource rendering — add "From Provider" and update options list:
```go
case stepSource:
	s += "\n" + helpStyle.Render("Add content from an installed provider, local files, or a git repository.") + "\n"
	s += helpStyle.Render("Create New scaffolds a blank template.") + "\n\n"
	options := []string{"From Provider", "Local Path", "Git URL", "Create New"}
	for i, opt := range options {
		prefix := "   "
		style := itemStyle
		if i == m.sourceCursor {
			prefix = " > "
			style = selectedItemStyle
		}
		row := prefix + style.Render(opt)
		s += zone.Mark(fmt.Sprintf("import-opt-%d", i), row) + "\n"
	}
```

`handleMouseClick` — update stepSource maxItems from 3 to 4:
```go
case stepSource:
	maxItems = 4
```

**Test updates in import_test.go:**

`TestImportSourceNavigation` — update expected max cursor from 2 to 3 and clamp assertion.

`TestImportSourceSelectLocal` — now needs `pressN(app, keyDown, 1)` before Enter.

`TestImportSourceSelectGit` — now needs `pressN(app, keyDown, 2)`.

`TestImportSourceSelectCreate` — now needs `pressN(app, keyDown, 3)`.

`TestImportViewSource` — add assertion: `assertContains(t, view, "From Provider")`.

---

### Task 10: Write tests for stepProviderPick

**Title:** Test provider pick step navigation, selection, esc, and empty provider list
**Depends on:** Task 7
**Files:** `cli/internal/tui/import_test.go`

**Success criteria:**
- Tests written and failing (red phase — stepProviderPick not wired yet)

**Tests to add:**

```go
func TestProviderPickNavigation(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepProviderPick
	// providers comes from testApp — should have at least 2
	if len(app.importer.providers) < 2 {
		t.Skip("need at least 2 providers for navigation test")
	}

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.discoveryProvCursor != 1 {
		t.Fatalf("expected discoveryProvCursor 1, got %d", app.importer.discoveryProvCursor)
	}
}

func TestProviderPickEscBack(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepProviderPick

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource after esc, got %d", app.importer.step)
	}
}

func TestProviderPickSelectDispatchesDiscovery(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepProviderPick
	if len(app.importer.providers) == 0 {
		t.Skip("no providers available")
	}

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected async discovery command on provider select")
	}
}
```

---

### Task 11: Implement stepProviderPick (update, view, step label)

**Title:** Implement provider pick step with navigation, rendering, and async discovery dispatch
**Depends on:** Task 9, Task 10
**Files:** `cli/internal/tui/import.go`

**Success criteria:**
- Provider list renders with cursor pattern
- up/down navigate, enter dispatches discovery, esc goes to source
- Step label shows "Step 2 of 3: Provider"
- Task 10 tests pass
- `make build` succeeds

**Changes:**

Add `updateProviderPick` function:
```go
func (m importModel) updateProviderPick(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.step = stepSource
	case key.Matches(msg, keys.Up):
		if m.discoveryProvCursor > 0 {
			m.discoveryProvCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.discoveryProvCursor < len(m.providers)-1 {
			m.discoveryProvCursor++
		}
	case key.Matches(msg, keys.Enter):
		if len(m.providers) == 0 {
			return m, nil
		}
		prov := m.providers[m.discoveryProvCursor]
		m.discoveryProvider = prov
		return m, func() tea.Msg {
			globalDir := catalog.GlobalContentDir()
			items, err := add.DiscoverFromProvider(prov, m.projectRoot, nil, globalDir)
			return discoveryDoneMsg{items: items, err: err}
		}
	}
	return m, nil
}
```

Wire into `Update()` key dispatch (after stepHookSelect case):
```go
case stepProviderPick:
	return m.updateProviderPick(msg)
case stepDiscoverySelect:
	return m.updateDiscoverySelect(msg)
```

Wire `discoveryDoneMsg` handling in `Update()` (after `importCloneDoneMsg` case):
```go
case discoveryDoneMsg:
	if msg.err != nil {
		m.message = msg.err.Error()
		m.messageIsErr = true
		m.step = stepProviderPick
		return m, nil
	}
	if len(msg.items) == 0 {
		m.message = fmt.Sprintf("No content found in %s.", m.discoveryProvider.Name)
		m.messageIsErr = true
		m.step = stepProviderPick
		return m, nil
	}
	m.discoveryItems = msg.items
	m.discoverySelected = make([]bool, len(msg.items))
	for i, item := range msg.items {
		m.discoverySelected[i] = item.Status != add.StatusInLibrary
	}
	m.discoveryCursor = 0
	m.step = stepDiscoverySelect
	return m, nil
```

Add `stepProviderPick` rendering in `View()`:
```go
case stepProviderPick:
	s += helpStyle.Render("Select a provider to discover content from") + "\n\n"
	for i, prov := range m.providers {
		prefix := "   "
		style := itemStyle
		if i == m.discoveryProvCursor {
			prefix = " > "
			style = selectedItemStyle
		}
		row := prefix + style.Render(prov.Name)
		s += zone.Mark(fmt.Sprintf("import-opt-%d", i), row) + "\n"
	}
```

Add step label:
```go
case stepProviderPick:
	return "Step 2 of 3: Provider"
case stepDiscoverySelect:
	return "Step 3 of 3: Select Items"
```

Add to `helpText()`:
```go
case stepProviderPick:
	return "up/down navigate • enter select • esc back"
```

Add to `handleMouseClick`:
```go
case stepProviderPick:
	maxItems = len(m.providers)
```

Add to `hasTextInput`:
No change needed (stepProviderPick has no text input).

---

### Task 12: Write tests for stepDiscoverySelect

**Title:** Test discovery select step: rendering, toggle, select/deselect all, add, esc
**Depends on:** Task 7
**Files:** `cli/internal/tui/import_test.go`

**Success criteria:**
- Tests written and failing (red phase)

**Tests to add:**

```go
func TestDiscoverySelectPreselection(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.discoveryItems = []add.DiscoveryItem{
		{Name: "new-rule", Type: catalog.Rules, Status: add.StatusNew},
		{Name: "lib-rule", Type: catalog.Rules, Status: add.StatusInLibrary},
		{Name: "old-rule", Type: catalog.Rules, Status: add.StatusOutdated},
	}
	app.importer.discoverySelected = []bool{true, false, true} // new=true, inlib=false, outdated=true
	app.importer.discoveryCursor = 0

	if !app.importer.discoverySelected[0] {
		t.Fatal("new items should be pre-selected")
	}
	if app.importer.discoverySelected[1] {
		t.Fatal("in-library items should be pre-deselected")
	}
	if !app.importer.discoverySelected[2] {
		t.Fatal("outdated items should be pre-selected")
	}
}

func TestDiscoverySelectToggle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.discoveryItems = []add.DiscoveryItem{
		{Name: "item-1", Type: catalog.Rules, Status: add.StatusNew},
	}
	app.importer.discoverySelected = []bool{true}
	app.importer.discoveryCursor = 0

	m, _ := app.Update(keySpace)
	app = m.(App)
	if app.importer.discoverySelected[0] {
		t.Fatal("space should toggle selection to false")
	}

	m, _ = app.Update(keySpace)
	app = m.(App)
	if !app.importer.discoverySelected[0] {
		t.Fatal("space should toggle selection back to true")
	}
}

func TestDiscoverySelectAll(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.discoveryItems = []add.DiscoveryItem{
		{Name: "a", Type: catalog.Rules, Status: add.StatusNew},
		{Name: "b", Type: catalog.Rules, Status: add.StatusInLibrary},
	}
	app.importer.discoverySelected = []bool{true, false}
	app.importer.discoveryCursor = 0

	m, _ := app.Update(keyRune('a'))
	app = m.(App)
	for i, sel := range app.importer.discoverySelected {
		if !sel {
			t.Fatalf("item %d should be selected after 'a'", i)
		}
	}
}

func TestDiscoveryDeselectAll(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.discoveryItems = []add.DiscoveryItem{
		{Name: "a", Type: catalog.Rules, Status: add.StatusNew},
		{Name: "b", Type: catalog.Rules, Status: add.StatusNew},
	}
	app.importer.discoverySelected = []bool{true, true}
	app.importer.discoveryCursor = 0

	m, _ := app.Update(keyRune('n'))
	app = m.(App)
	for i, sel := range app.importer.discoverySelected {
		if sel {
			t.Fatalf("item %d should be deselected after 'n'", i)
		}
	}
}

func TestDiscoverySelectNavigation(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.discoveryItems = []add.DiscoveryItem{
		{Name: "a", Type: catalog.Rules, Status: add.StatusNew},
		{Name: "b", Type: catalog.Rules, Status: add.StatusNew},
	}
	app.importer.discoverySelected = []bool{true, true}
	app.importer.discoveryCursor = 0

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.discoveryCursor != 1 {
		t.Fatalf("expected discoveryCursor 1, got %d", app.importer.discoveryCursor)
	}

	// Clamp at end
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.importer.discoveryCursor != 1 {
		t.Fatal("discoveryCursor should clamp at end")
	}
}

func TestDiscoverySelectEscBack(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepProviderPick {
		t.Fatalf("expected stepProviderPick after esc, got %d", app.importer.step)
	}
}

func TestDiscoverySelectEnterDispatchesAdd(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.discoveryProvider = app.importer.providers[0]
	app.importer.discoveryItems = []add.DiscoveryItem{
		{Name: "add-me", Type: catalog.Rules, Path: "/tmp/fake", Status: add.StatusNew},
	}
	app.importer.discoverySelected = []bool{true}
	app.importer.discoveryCursor = 0

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected async add command on enter")
	}
}

func TestDiscoverySelectNoneSelectedShowsMessage(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.discoveryItems = []add.DiscoveryItem{
		{Name: "a", Type: catalog.Rules, Status: add.StatusNew},
	}
	app.importer.discoverySelected = []bool{false}
	app.importer.discoveryCursor = 0

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.message == "" {
		t.Fatal("expected error message when no items selected")
	}
}
```

---

### Task 13: Implement stepDiscoverySelect (update handler)

**Title:** Implement discovery select keyboard handler
**Depends on:** Task 11, Task 12
**Files:** `cli/internal/tui/import.go`

**Success criteria:**
- space toggles, a selects all, n deselects all
- up/down navigate cursor
- enter dispatches async add, esc goes back
- Task 12 tests pass

**Add `updateDiscoverySelect` function:**

```go
func (m importModel) updateDiscoverySelect(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.step = stepProviderPick
	case key.Matches(msg, keys.Up):
		if m.discoveryCursor > 0 {
			m.discoveryCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.discoveryCursor < len(m.discoveryItems)-1 {
			m.discoveryCursor++
		}
	case msg.String() == " ":
		m.discoverySelected[m.discoveryCursor] = !m.discoverySelected[m.discoveryCursor]
	case msg.String() == "a":
		for i := range m.discoverySelected {
			m.discoverySelected[i] = true
		}
	case msg.String() == "n":
		for i := range m.discoverySelected {
			m.discoverySelected[i] = false
		}
	case key.Matches(msg, keys.Enter):
		var selected []add.DiscoveryItem
		for i, item := range m.discoveryItems {
			if m.discoverySelected[i] {
				selected = append(selected, item)
			}
		}
		if len(selected) == 0 {
			m.message = "No items selected"
			m.messageIsErr = true
			return m, nil
		}
		prov := m.discoveryProvider
		return m, func() tea.Msg {
			globalDir := catalog.GlobalContentDir()
			opts := add.AddOptions{Force: true, Provider: prov.Slug}
			results := add.AddItems(selected, opts, globalDir, nil, "syllago")
			var added []string
			var errs []string
			for _, r := range results {
				switch r.Status {
				case add.AddStatusAdded, add.AddStatusUpdated:
					added = append(added, r.Name)
				case add.AddStatusError:
					errs = append(errs, fmt.Sprintf("%s: %v", r.Name, r.Error))
				}
			}
			if len(errs) > 0 {
				return importDoneMsg{
					name: strings.Join(added, ", "),
					err:  fmt.Errorf("partial: %s", strings.Join(errs, "; ")),
				}
			}
			return importDoneMsg{name: strings.Join(added, ", ")}
		}
	}
	return m, nil
}
```

---

### Task 14: Implement stepDiscoverySelect view rendering

**Title:** Render discovery select step with checkboxes, status badges, and action buttons
**Depends on:** Task 13
**Files:** `cli/internal/tui/import.go`

**Success criteria:**
- Items render with cursor prefix, checkbox, name, and status badge
- Status badges use correct styles: plain for `(new)`, `installedStyle` for `(in library)`, `warningStyle` for `(outdated)`
- In-library item names use `helpStyle` (muted)
- Action bar with zone-marked buttons renders below items
- `make build` succeeds

**Add rendering in `View()` switch:**

```go
case stepDiscoverySelect:
	count := 0
	for _, sel := range m.discoverySelected {
		if sel {
			count++
		}
	}
	newCount := 0
	outdatedCount := 0
	libCount := 0
	for _, item := range m.discoveryItems {
		switch item.Status {
		case add.StatusNew:
			newCount++
		case add.StatusOutdated:
			outdatedCount++
		case add.StatusInLibrary:
			libCount++
		}
	}
	s += labelStyle.Render(fmt.Sprintf("Add from %s", m.discoveryProvider.Name)) + "\n"
	s += helpStyle.Render(fmt.Sprintf("%d items found (%d new, %d outdated, %d in library)", len(m.discoveryItems), newCount, outdatedCount, libCount)) + "\n\n"

	for i, item := range m.discoveryItems {
		check := "[ ]"
		if m.discoverySelected[i] {
			check = installedStyle.Render("[x]")
		}
		prefix := "  "
		style := itemStyle
		if i == m.discoveryCursor {
			prefix = "> "
			style = selectedItemStyle
		}

		nameStr := fmt.Sprintf("%s/%s", item.Type, item.Name)
		if item.Status == add.StatusInLibrary {
			nameStr = helpStyle.Render(nameStr)
		} else {
			nameStr = style.Render(nameStr)
		}

		var badge string
		switch item.Status {
		case add.StatusNew:
			badge = "(new)"
		case add.StatusInLibrary:
			badge = installedStyle.Render("(in library)")
		case add.StatusOutdated:
			badge = warningStyle.Render("(outdated)")
		}

		row := fmt.Sprintf("  %s%s %s  %s", prefix, check, nameStr, badge)
		s += zone.Mark(fmt.Sprintf("discovery-item-%d", i), row) + "\n"
	}

	s += "\n"
	s += labelStyle.Render("Actions") + "\n"
	s += helpStyle.Render(strings.Repeat("─", 20)) + "\n"
	selectAllBtn := zone.Mark("discovery-btn-all", buttonStyle.Render("Select All"))
	deselectAllBtn := zone.Mark("discovery-btn-none", buttonStyle.Render("Deselect All"))
	addBtn := zone.Mark("discovery-btn-add", buttonStyle.Render(fmt.Sprintf("Add Selected (%d)", count)))
	s += selectAllBtn + "  " + deselectAllBtn + "  " + addBtn + "\n"
```

Add help footer:
```go
case stepDiscoverySelect:
	return "up/down navigate • space toggle • a select all • n deselect all • enter add selected • esc back"
```

---

### Task 15: Implement mouse handling for discovery select

**Title:** Add mouse click handling for discovery item rows and action buttons
**Depends on:** Task 14
**Files:** `cli/internal/tui/import.go`

**Success criteria:**
- Clicking a discovery item row toggles its checkbox
- Clicking action buttons (Select All, Deselect All, Add Selected) triggers correct behavior
- `make build` succeeds

**Changes in `Update()` mouse handling section** (inside the `tea.MouseMsg` case, before the existing zone click handling):

Add discovery select mouse handling for left-click release:
```go
if m.step == stepDiscoverySelect && msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
	// Check action buttons first
	if zone.Get("discovery-btn-all").InBounds(msg) {
		for i := range m.discoverySelected {
			m.discoverySelected[i] = true
		}
		return m, nil
	}
	if zone.Get("discovery-btn-none").InBounds(msg) {
		for i := range m.discoverySelected {
			m.discoverySelected[i] = false
		}
		return m, nil
	}
	if zone.Get("discovery-btn-add").InBounds(msg) {
		return m.updateDiscoverySelect(tea.KeyMsg{Type: tea.KeyEnter})
	}
	// Per-item toggle
	for i := 0; i < len(m.discoveryItems); i++ {
		if zone.Get(fmt.Sprintf("discovery-item-%d", i)).InBounds(msg) {
			m.discoverySelected[i] = !m.discoverySelected[i]
			return m, nil
		}
	}
	return m, nil
}
```

---

### Task 16: Retrofit hook select with action buttons

**Title:** Replace hook select inline hint text with zone-marked action buttons
**Depends on:** Task 14
**Files:** `cli/internal/tui/import.go`

**Success criteria:**
- Hook select step renders Actions header + divider + [Select All] [Deselect All] [Add Selected (N)] buttons
- Buttons are zone-marked with IDs: `hook-btn-all`, `hook-btn-none`, `hook-btn-add`
- Help footer updated to match discovery select pattern
- `make build` succeeds

**Changes in `View()` stepHookSelect rendering** — replace the inline hint line:

```go
case stepHookSelect:
	count := 0
	for _, sel := range m.hookSelected {
		if sel {
			count++
		}
	}
	s += labelStyle.Render(fmt.Sprintf("Found %d hooks in settings.json:", len(m.hookCandidates))) + "\n\n"
	for i, hook := range m.hookCandidates {
		check := "[ ]"
		if m.hookSelected[i] {
			check = installedStyle.Render("[✓]")
		}
		prefix := "  "
		style := itemStyle
		if i == m.hookSelectCursor {
			prefix = "> "
			style = selectedItemStyle
		}
		matcher := hook.Matcher
		if matcher == "" {
			matcher = "*"
		}
		s += fmt.Sprintf("  %s%s %s  %s\n", prefix, check, style.Render(m.hookNames[i]), helpStyle.Render("("+hook.Event+"/"+matcher+")"))
	}
	s += "\n"
	s += labelStyle.Render("Actions") + "\n"
	s += helpStyle.Render(strings.Repeat("─", 20)) + "\n"
	selectAllBtn := zone.Mark("hook-btn-all", buttonStyle.Render("Select All"))
	deselectAllBtn := zone.Mark("hook-btn-none", buttonStyle.Render("Deselect All"))
	addBtn := zone.Mark("hook-btn-add", buttonStyle.Render(fmt.Sprintf("Add Selected (%d)", count)))
	s += selectAllBtn + "  " + deselectAllBtn + "  " + addBtn + "\n"
```

Update help footer for stepHookSelect:
```go
case stepHookSelect:
	return "up/down navigate • space toggle • a select all • n deselect all • enter add selected • esc back"
```

Add mouse handling for hook buttons (in the left-click release section):
```go
if m.step == stepHookSelect && msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
	if zone.Get("hook-btn-all").InBounds(msg) {
		for i := range m.hookSelected {
			m.hookSelected[i] = true
		}
		return m, nil
	}
	if zone.Get("hook-btn-none").InBounds(msg) {
		for i := range m.hookSelected {
			m.hookSelected[i] = false
		}
		return m, nil
	}
	if zone.Get("hook-btn-add").InBounds(msg) {
		return m.updateHookSelect(tea.KeyMsg{Type: tea.KeyEnter})
	}
}
```

---

### Task 17: Regenerate golden files and run full test suite

**Title:** Regenerate golden files after Phase 2 changes and verify all tests pass
**Depends on:** Task 9, Task 11, Task 13, Task 14, Task 15, Task 16
**Files:** `cli/internal/tui/testdata/*.golden`

**Success criteria:**
- `cd cli && go test ./internal/tui/ -update-golden` succeeds
- `cd cli && make test` passes (all tests green)
- Golden file diffs show expected new content

**Commands:**
```bash
cd /home/hhewett/.local/src/syllago && make build
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -update-golden
cd /home/hhewett/.local/src/syllago/cli && make test
```

---

### Task 18: Commit Phase 2

**Title:** Commit Phase 2 provider discovery integration
**Depends on:** Task 17

**Success criteria:**
- Clean commit with message: `feat(tui): add provider discovery to TUI add wizard`
- All tests pass after commit

---

## Task Dependency Graph

```
Phase 1:
  Task 1 (import.go strings) ──┐
  Task 2 (app.go strings) ─────┤
  Task 3 (sidebar/settings) ───┤
                                ├──→ Task 4 (test updates) ──→ Task 5 (golden + test) ──→ Task 6 (commit)
                                │
Phase 2:                        │
  Task 7 (enum + fields) ──────┼──→ Task 8 (source tests) ──→ Task 9 (source impl) ─────┐
                                │                                                          │
                                ├──→ Task 10 (pick tests) ──→ Task 11 (pick impl) ────────┤
                                │                                                          │
                                └──→ Task 12 (select tests) ──→ Task 13 (select handler) ─┤
                                                                                           │
                                     Task 14 (select view) ←──────────────────────────────┘
                                          │
                                          ├──→ Task 15 (mouse handling)
                                          ├──→ Task 16 (hook retrofit)
                                          │
                                          └──→ Task 17 (golden + test) ──→ Task 18 (commit)
```
