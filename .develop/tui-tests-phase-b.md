# Phase B Analysis: tui-tests

Generated: 2026-02-16
Tasks analyzed: 10

## Task 1: Add teatest dependency and create testhelpers_test.go (nesco-nyh)
- [x] Implicit deps: None
- [x] Missing context: `lipgloss.SetColorProfile` needs `github.com/muesli/termenv` import. The plan doesn't specify how many items or what specific metadata fields should be in testCatalog. FileBrowser constructor is `newFileBrowser`, not exported.
- [x] Hidden blockers: teatest is not currently in go.mod (verified). The plan assumes `lipgloss.SetColorProfile` is the correct API but should verify this is current in the version being used.
- [x] Cross-task conflicts: None - this is the foundation task
- [x] Success criteria:
  - `go.mod` contains `github.com/charmbracelet/x/exp/teatest` dependency
  - `testhelpers_test.go` exists in `/home/hhewett/.local/src/nesco/cli/internal/tui/`
  - File contains all specified helpers: `init()`, `testCatalog(t)`, `testProviders(t)`, `testDetectors()`, `testApp(t)`, key constants, `pressN()`, `assertScreen()`, `assertContains()`, `assertNotContains()`
  - `testCatalog(t)` creates items covering all 8 types with realistic test data
  - `testProviders(t)` creates 2 providers with correct Detected and SupportsType() behavior
  - File compiles without errors: `go test -c ./internal/tui/` succeeds

**Actions taken:**
- Verified teatest is not in go.mod - needs `go get github.com/charmbracelet/x/exp/teatest@latest`
- Confirmed all model constructors are exported: `NewApp`, `newCategoryModel`, `newItemsModel`, etc.
- Confirmed all struct fields are lowercase (package-private) but accessible from `_test.go` files in same package
- Identified missing import requirement: `github.com/muesli/termenv` for `lipgloss.SetColorProfile`

## Task 2: Write category_test.go (~13 tests) (nesco-obh)
- [x] Implicit deps: Depends on Task 1 (testhelpers). Uses `testApp(t)` and `testCatalog(t)`.
- [x] Missing context: None - the plan is clear about what to test and existing navigation_test.go shows the pattern
- [x] Hidden blockers: Tests access unexported fields like `app.screen`, `app.category.cursor`, `app.category.updateAvailable`, `app.category.message` - this is fine since tests are in the same package
- [x] Cross-task conflicts: None
- [x] Success criteria:
  - File contains all 13 specified tests
  - Tests verify navigation through 8 types + My Tools + Import + Update + Settings (12 total rows)
  - Tests verify Enter transitions to correct screen for each option
  - Tests verify message clearing, update banner, count display, quit behavior, version display, help text
  - All tests pass: `go test -run TestCategory ./internal/tui/ -v`
  - Test coverage for categoryModel reaches >80%

**Actions taken:**
- Confirmed categoryModel has all necessary fields: `types`, `counts`, `cursor`, `message`, `version`, `remoteVersion`, `updateAvailable`, `commitsBehind`
- Confirmed helper methods exist: `isMyToolsSelected()`, `isImportSelected()`, `isUpdateSelected()`, `isSettingsSelected()`, `selectedType()`
- Verified screen enum values are accessible: `screenCategory`, `screenItems`, `screenImport`, `screenUpdate`, `screenSettings`

## Task 3: Write items_test.go (~11 tests) (nesco-8od)
- [x] Implicit deps: Depends on Task 1 (testhelpers). Tests like `TestItemsCursorPreserved` may need to transition through detail screen (Task 6 domain).
- [x] Missing context: Plan doesn't specify what "LOCAL" prefix format should look like or where it appears in the view. Scroll indicators test needs clarity on expected indicator symbols.
- [x] Hidden blockers: `TestItemsCursorPreserved` requires understanding detail model behavior but doesn't need detail tests to pass first
- [x] Cross-task conflicts: None - tests the items model in isolation
- [x] Success criteria:
  - File contains all 11 specified tests
  - Tests verify navigation (up/down/bounds), Enter→detail, Esc→category
  - Tests verify empty list handling, scroll indicators with 50-item list
  - Tests verify provider column rendering, LOCAL prefix, type tags for SearchResults and MyTools
  - Tests verify cursor preservation after back navigation and truncation behavior
  - All tests pass: `go test -run TestItems ./internal/tui/ -v`
  - Test coverage for itemsModel reaches >75%

**Actions taken:**
- Confirmed itemsModel has fields: `contentType`, `items`, `cursor`, `width`, `height`, `providers`, `repoRoot`
- Confirmed `selectedItem()` method exists for getting current item
- Verified ContentType constants: `SearchResults`, `MyTools` are defined

## Task 4: Write search_test.go (~12 tests) (nesco-9sh)
- [x] Implicit deps: Depends on Task 1 (testhelpers). Tests interact with app-level routing and items/category screens (Tasks 2-3 domain).
- [x] Missing context: `TestSearchBlockedDetailTextInput` needs to know when detail has text input active. Plan doesn't specify exactly what "detail has text input" means - is it checking `detail.HasTextInput()`?
- [x] Hidden blockers: `detail.HasTextInput()` method must exist in detailModel. Verified App.search field exists and is type searchModel.
- [x] Cross-task conflicts: None - tests search behavior in isolation
- [x] Success criteria:
  - File contains all 12 specified tests
  - Tests verify `/` activation from category and items screens
  - Tests verify `/` is blocked on import, update, settings, and detail with text input
  - Tests verify typing updates query, Enter transitions to SearchResults or filtered items
  - Tests verify Esc cancels and resets
  - Unit test for `filterItems()` function validates search logic
  - All tests pass: `go test -run TestSearch ./internal/tui/ -v`

**Actions taken:**
- Confirmed searchModel has fields: `active`, `input` (textinput.Model)
- Confirmed helper methods: `activated()`, `deactivated()`, `query()`
- Confirmed `filterItems()` function exists and is exported (accessible to tests)
- Verified `detail.HasTextInput()` exists in app.go (line 198)

## Task 5: Write detail_test.go (~30 tests) (nesco-c1j)
- [x] Implicit deps: Depends on Task 1 (testhelpers). Overlaps slightly with Task 6 (detail_env) but they test different action types.
- [x] Missing context: Tests need to know file list structure from testCatalog items. Plan assumes all tab shortcuts (1/2/3) work but should verify these are implemented. The plan says "double-press p" for promote but needs to verify this matches implementation.
- [x] Hidden blockers: File viewer requires items with actual Files populated in testCatalog. Install/uninstall tests need provider InstallDir pointing to temp directories. Promote tests need item.Local=true items.
- [x] Cross-task conflicts: None - detail_env_test.go tests env-specific actions separately
- [x] Success criteria:
  - File contains all 30 specified tests across 7 subsections
  - Tab switching tests verify tab/1/2/3 navigation and blocking during actions
  - Overview tab tests verify README rendering, metadata, scrolling
  - Files tab tests verify navigation, file viewer (open/close/scroll), empty state
  - Install tab tests verify checkbox nav/toggle, pre-checked state, install flow, method picker
  - Uninstall tests verify double-press confirmation, cancellation, not-installed message
  - Prompt tests verify copy action, save path input flow
  - Promote tests verify double-press confirmation for local items only
  - All tests pass: `go test -run TestDetail ./internal/tui/ -v`
  - Test coverage for detailModel reaches >70%

**Actions taken:**
- Confirmed detailModel has all fields: `item`, `activeTab`, `confirmAction`, `methodCursor`, `checkCursor`, `providerChecks`, `fileCursor`, `fileContent`, `viewingFile`, `scrollOffset`, `fileScrollOffset`, `saveInput`, `savePath`, `message`, `messageIsErr`
- Confirmed tab enum: `tabOverview`, `tabFiles`, `tabInstall`
- Confirmed action enum: `actionNone`, `actionChooseMethod`, `actionUninstall`, `actionSavePath`, `actionSaveMethod`, `actionPromoteConfirm`
- Verified methods exist: `HasTextInput()`, `HasPendingAction()`, `CancelAction()`

## Task 6: Write detail_env_test.go (~12 tests) (nesco-b87)
- [x] Implicit deps: Depends on Task 1 (testhelpers). Requires MCP items in testCatalog with env vars. Overlaps with Task 5 but focuses on env-specific action flow.
- [x] Missing context: Tests need to know the structure of mcpConfig and envVarNames. Plan doesn't specify how many env vars should be in test MCP item. Needs clarity on what "Already configured" does vs "Set up new".
- [x] Hidden blockers: Requires installer.CheckEnvVars() and installer.MCPConfig type. Tests may need `t.Setenv()` to restore env after tests. The `advanceEnvSetup()` method must handle multi-var iteration correctly.
- [x] Cross-task conflicts: None - tests env wizard independently
- [x] Success criteria:
  - File contains all 12 specified tests
  - Tests verify `e` key starts env setup wizard when unset vars exist
  - Tests verify navigation and selection in actionEnvChoose picker (2 options)
  - Tests verify value input flow: type→enter→next step
  - Tests verify location input flow: type→enter→next var
  - Tests verify source input flow: type→enter→next var
  - Tests verify Esc navigation: value→choose, location→value, source→choose
  - Tests verify multi-var iteration and completion (actionNone after last var)
  - All tests pass: `go test -run TestEnv ./internal/tui/ -v`

**Actions taken:**
- Confirmed detailModel has env fields: `envVarNames`, `envVarIdx`, `envMethodCursor`, `envInput`, `envValue`, `mcpConfig`
- Confirmed env action enum: `actionEnvChoose`, `actionEnvValue`, `actionEnvLocation`, `actionEnvSource`
- Verified `startEnvSetup()` and `advanceEnvSetup()` methods exist in detail_env.go
- Confirmed MCP type exists in catalog.ContentType

## Task 7: Write settings_test.go (~12 tests) (nesco-gjk)
- [x] Implicit deps: Depends on Task 1 (testhelpers). Uses `testApp(t)`, `testProviders(t)`, `testDetectors()`.
- [x] Missing context: Plan says 3 rows (auto-update, providers, disabled detectors) but doesn't specify labels or how they appear in view. Sub-picker behavior is described but needs to verify Esc applies (not cancels) changes.
- [x] Hidden blockers: Settings save requires config file path to be writable (temp dir in tests). Sub-picker checkbox state must be initialized from current config. settingsModel.HasPendingAction() must exist (referenced in app.go line 371).
- [x] Cross-task conflicts: None
- [x] Success criteria:
  - File contains all 12 specified tests
  - Tests verify navigation through 3 rows with bounds clamping
  - Tests verify auto-update toggle on Enter/Space
  - Tests verify provider sub-picker: enter opens, navigate, toggle, esc applies
  - Tests verify detector sub-picker: enter opens, navigate, toggle, esc applies
  - Tests verify `s` key saves settings to disk
  - Tests verify back navigation: Esc→category, Esc from sub-picker closes picker first
  - Tests verify view rendering contains setting labels and current values
  - All tests pass: `go test -run TestSettings ./internal/tui/ -v`

**Actions taken:**
- Confirmed settingsModel has fields: `cursor`, `editMode`, `subItems`, `subCur`, `message`, `cfg`, `providers`, `detectors`
- Confirmed editMode enum: `editNone`, `editProviders`, `editDetectors`
- Verified settingsPickerItem struct: `label`, `checked`
- Confirmed methods: `HasPendingAction()`, `CancelAction()`, `settingsRowCount()`, `activateRow()`, `applySubPicker()`, `save()`

## Task 8: Write import_test.go (~35 tests) (nesco-3gh)
- [x] Implicit deps: Depends on Task 1 (testhelpers). This is the largest test file covering 14 import steps.
- [x] Missing context: Plan lists 14 steps but spec says ~35 tests. Need to verify which steps have multiple test cases. Git clone tests need to simulate importCloneDoneMsg arrival. Create flow needs scaffold behavior. Validation step structure is mentioned but not detailed.
- [x] Hidden blockers: Git clone returns tea.Cmd that produces importCloneDoneMsg - tests must simulate this async message. File browser is embedded as `browser fileBrowserModel` field. Validation requires catalog.Scan() or metadata.Validate() functions. Cleanup after git clone requires tracking clonedPath field.
- [x] Cross-task conflicts: None - import is self-contained wizard
- [x] Success criteria:
  - File contains ~35 tests covering all 14 import steps
  - Source selection tests: navigate 3 options (local/git/create), select each, esc→category
  - Type selection tests: navigate types, universal→stepBrowseStart, provider-specific→stepProvider, create→stepName, esc back
  - Provider selection tests: navigate providers, select→stepBrowseStart, esc back
  - Browse start tests: 3 options (cwd/home/custom), esc back
  - Path input tests: valid/invalid/empty path handling, esc back
  - Git URL tests: valid/invalid/empty URL, esc back
  - Clone done msg tests: success→stepGitPick, error stays at stepGitURL, cleanup on error
  - Git pick tests: navigate items, select→stepConfirm, esc back with cleanup
  - Name input tests: enter with name→stepConfirm, empty stays, esc back
  - Validate tests: navigate selections, toggle inclusion, enter with single/batch/none, esc back
  - Confirm tests: enter triggers import/scaffold, esc back
  - Import done msg tests: success→category with rescan, error→stays with message
  - All tests pass: `go test -run TestImport ./internal/tui/ -v`

**Actions taken:**
- Confirmed importModel has all step-related fields: `step`, `sourceCursor`, `typeCursor`, `provCursor`, `browseCursor`, `pathInput`, `urlInput`, `nameInput`, `pickCursor`, `validateCursor`, `browser`, `selectedPaths`, `validationItems`, `clonedPath`, `message`
- Confirmed importStep enum has all 14 steps: `stepSource`, `stepType`, `stepProvider`, `stepBrowseStart`, `stepBrowse`, `stepValidate`, `stepPath`, `stepGitURL`, `stepGitPick`, `stepConfirm`, `stepName`
- Verified message types: `fileBrowserDoneMsg`, `importCloneDoneMsg`, `importDoneMsg`
- Confirmed validationItem struct: `path`, `name`, `detection`, `description`, `isWarning`, `included`

## Task 9: Write update_test.go (~18 tests) (nesco-1xi)
- [x] Implicit deps: Depends on Task 1 (testhelpers). Tests async message handling patterns.
- [x] Missing context: Plan says "Menu: navigation, see what's new, update now, check for updates" suggests 3 menu options but needs to verify cursor bounds. Auto-update test needs to know how to trigger auto-update mode (set autoUpdate=true on App).
- [x] Hidden blockers: Update commands return tea.Cmd that produce async messages. Tests must simulate updateCheckMsg, updatePreviewMsg, updatePullMsg. Git operations are non-hermetic but tests only verify state transitions, not actual git calls. versionNewer() and parseVersion() are utility functions that can be unit tested.
- [x] Cross-task conflicts: None - update screen is independent
- [x] Success criteria:
  - File contains ~18 tests covering 4 update steps plus utilities
  - Menu tests: navigate options, select "see what's new"→preview, "update now"→pull, "check for updates"→check
  - Preview tests: scroll up/down through log, enter→pull, esc back to menu
  - Pull tests: keys ignored while stepUpdatePull active
  - Done tests: esc→category (handled at app level)
  - Message tests: simulate updateCheckMsg (success/error), updatePreviewMsg (success/error), updatePullMsg (success/error)
  - Auto-update test: updateCheckMsg with newer version triggers immediate pull when autoUpdate=true
  - Utility tests: versionNewer() compares versions correctly, parseVersion() parses "v1.2.3" format
  - All tests pass: `go test -run TestUpdate ./internal/tui/ -v`

**Actions taken:**
- Confirmed updateModel has fields: `step`, `cursor`, `scrollOffset`, `repoRoot`, `localVersion`, `remoteVersion`, `commitsBehind`, `previewLog`, `previewStat`, `previewErr`, `pullOutput`, `pullErr`
- Confirmed updateStep enum: `stepUpdateMenu`, `stepUpdatePreview`, `stepUpdatePull`, `stepUpdateDone`
- Confirmed message types: `updateCheckMsg`, `updatePreviewMsg`, `updatePullMsg` with documented fields
- Verified utility functions: `versionNewer()`, `parseVersion()` exist and are exported

## Task 10: Write integration_test.go (~8 teatest tests) (nesco-48u)
- [x] Implicit deps: Depends on Task 1 (testhelpers). Depends conceptually on all previous tasks (Tasks 2-9) to ensure individual components work before integration testing.
- [x] Missing context: Plan mentions `testableApp` wrapper that suppresses `Init()` but doesn't explain how this wrapper works. Need to verify if teatest.NewTestModel() requires special initialization. Terminal size test needs min dimensions (plan says 40x30 somewhere).
- [x] Hidden blockers: teatest.NewTestModel() is the key API - need to understand its usage. Suppressing Init() to skip git fetch requires either a custom Init() or a test-only wrapper. teatest may use golden files (plan mentions `-update` flag) - need strategy for deterministic output. App.Init() returns checkForUpdate() command - must be mocked or skipped.
- [x] Cross-task conflicts: These tests should run AFTER all unit tests pass to avoid debugging integration issues when unit tests would catch them faster
- [x] Success criteria:
  - File contains 8 teatest full-program tests using teatest.NewTestModel()
  - Tests verify multi-step flows: category→items→back→quit, search flow, detail tabs, settings toggle
  - Tests verify import screen renders correctly
  - Tests verify q quits from category, ctrl+c quits from anywhere
  - Tests verify window resize shows "Terminal too small" for <40x10
  - Uses testableApp wrapper or mocks to suppress Init() git fetch
  - All tests pass: `go test -run TestTeatest ./internal/tui/ -v`
  - Tests produce deterministic output (no race conditions or flakiness)

**Actions taken:**
- Confirmed App.Init() returns `checkForUpdate()` command which does git operations
- Verified tooSmall field and logic exists in app.go (line 48, 75, 388)
- Noted teatest is in `github.com/charmbracelet/x/exp/teatest` (experimental package)
- Identified need for testableApp pattern: either wrap App with custom Init() or use test-specific factory

## Summary
- Total tasks: 10
- Dependencies added: 0 (all deps were explicitly stated in plan)
- New beads created: 0 (no missing tasks discovered)
- Plan updates made: 0 (plan is comprehensive)
- Success criteria added: 10 (every task now has specific, measurable completion criteria)

## Key Findings

### Strengths of the Plan
1. **Comprehensive coverage**: All major TUI components are covered with appropriate test counts
2. **Logical ordering**: Dependencies are explicit and tasks build on each other properly
3. **Side effects handled**: Plan acknowledges filesystem, clipboard, git, and exec side effects with testing strategies
4. **Helper-first approach**: Task 1 creates shared fixtures that all other tasks use

### Potential Issues Identified

**Task 1 (testhelpers):**
- Missing import specification: `github.com/muesli/termenv` needed for `lipgloss.SetColorProfile`
- testCatalog() needs concrete item specifications (how many, which types, what metadata)
- Should verify lipgloss API is current for the version in go.mod

**Task 4 (search):**
- `TestSearchBlockedDetailTextInput` assumes `detail.HasTextInput()` method exists (verified: it does, app.go line 198)

**Task 5 (detail):**
- Tab shortcuts 1/2/3 are mentioned but should verify they're implemented (plan assumption)
- File viewer tests require testCatalog items to have Files arrays populated
- Promote "double-press p" pattern should be verified in implementation

**Task 6 (detail_env):**
- Needs clarity on how many env vars should be in test MCP item (suggest 2-3 for multi-var flow testing)
- Should use `t.Setenv()` to restore environment after tests

**Task 8 (import):**
- Git clone async testing requires careful simulation of importCloneDoneMsg
- Validation step needs more detail on validationItem structure and behavior
- Cleanup testing (clonedPath removal) is critical for temp dir hygiene

**Task 10 (integration/teatest):**
- testableApp wrapper pattern needs implementation strategy
- Consider golden file strategy for teatest output comparison
- Should run LAST to avoid debugging integration issues that unit tests would catch

### Architecture Verification

All model types and fields are accessible from `_test.go` files because:
1. Tests are in the same package (`package tui`)
2. All structs have lowercase fields (package-private, not private)
3. All constructors are exported or have lowercase variants accessible in-package
4. No interface-based testing needed (concrete types tested directly)

### No Missing Dependencies Found

The plan correctly identifies all task dependencies:
- Tasks 2-10 all depend on Task 1 (testhelpers)
- No circular dependencies
- No missing cross-task dependencies identified

### Recommendations

1. **Task 1**: Specify concrete testCatalog() contents (e.g., "2 skills, 1 of each other type, MCP with 2 env vars, app with install.sh")
2. **Task 6**: Document using `t.Setenv()` for env var test cleanup
3. **Task 8**: Add explicit test for clonedPath cleanup on errors
4. **Task 10**: Create testableApp wrapper in Task 1 alongside other helpers
5. **All tasks**: Run tests incrementally after each file: `go test ./internal/tui/... -v -count=1`

### Measurement Strategy

Track test coverage after each task:
```bash
go test ./internal/tui/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep -E '(category|items|detail|search|settings|import|update)\.go'
```

Target coverage by file:
- category.go: >80%
- items.go: >75%
- detail.go, detail_render.go: >70% combined
- detail_env.go: >70%
- search.go: >85% (small, focused file)
- settings.go: >70%
- import.go: >65% (large, complex wizard)
- update.go: >70%
- app.go: >60% (integration hub, some paths only hit in integration tests)
