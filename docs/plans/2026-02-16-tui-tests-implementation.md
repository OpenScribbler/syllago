# Implementation Plan: Comprehensive TUI Integration Tests

## Setup

### 1. Add teatest dependency

```bash
cd cli && go get github.com/charmbracelet/x/exp/teatest@latest
```

### 2. Create `testhelpers_test.go`

Shared fixtures and helpers:

- `init()` — `lipgloss.SetColorProfile(termenv.Ascii)` for deterministic output
- `testCatalog(t)` — Catalog with items of every type (skills x2, agent, prompt w/body, MCP w/env vars, app w/install.sh, rule, hook, command, local skill w/LLM-PROMPT.md). Uses `t.TempDir()` with real files on disk so file viewer works.
- `testProviders(t)` — 2 providers: Claude Code (detected, supports all types) and Cursor (not detected, supports Skills+Rules). InstallDir points to temp dirs.
- `testDetectors()` — 2 mock detectors for settings tests
- `testApp(t)` — Full App with catalog + providers + detectors + dimensions (80x30)
- Key press constants: `keyUp`, `keyDown`, `keyEnter`, `keyEsc`, `keySpace`, `keyTab`, `keyCtrlC`, `keyRune(r)`
- `pressN(app, msg, n)` — Send a key N times
- `assertScreen(t, app, expected)`, `assertContains(t, s, substr)`, `assertNotContains(t, s, substr)`

---

## Test Files

### `category_test.go` (~13 tests)

| Test | What it covers |
|------|---------------|
| `TestCategoryNavigation` | Up/down through all rows (8 types + My Tools + Import + Update + Settings), bounds clamping |
| `TestCategorySelectEachType` | Enter on each content type → screenItems with correct contentType |
| `TestCategorySelectMyTools` | Enter → screenItems, contentType=MyTools, only local items |
| `TestCategorySelectImport` | Enter → screenImport, step=stepSource |
| `TestCategorySelectUpdate` | Enter → screenUpdate, step=stepUpdateMenu |
| `TestCategorySelectSettings` | Enter → screenSettings |
| `TestCategoryMessageClear` | Message cleared on any keypress |
| `TestCategoryUpdateBanner` | updateAvailable=true renders banner |
| `TestCategoryCountDisplay` | View() shows count for each type |
| `TestCategoryQuitFromCategory` | q and ctrl+c produce quit commands |
| `TestCategoryQuitOnlyFromCategory` | q does NOT quit from other screens |
| `TestCategoryVersionDisplay` | View() contains version string |
| `TestCategoryViewHelp` | View() contains navigation help text |

### `items_test.go` (~11 tests)

| Test | What it covers |
|------|---------------|
| `TestItemsNavigationUpDown` | Cursor movement and bounds |
| `TestItemsEnterOpensDetail` | Enter → screenDetail with correct item |
| `TestItemsBackGoesToCategory` | Esc → screenCategory, counts refreshed |
| `TestItemsEmptyList` | "No items found", enter doesn't crash |
| `TestItemsScrollIndicators` | Up/down indicators with 50-item list |
| `TestItemsProviderColumn` | Provider names render for provider-specific types |
| `TestItemsLocalPrefix` | "LOCAL" prefix for local items |
| `TestItemsSearchResultsTypeTag` | Type tags for SearchResults contentType |
| `TestItemsMyToolsTypeTag` | Type tags for MyTools contentType |
| `TestItemsCursorPreserved` | Cursor position preserved after Detail→back |
| `TestItemsTruncation` | Long names/descriptions don't exceed width |

### `detail_test.go` (~30 tests)

**Tab switching:**
- `TestDetailTabCycle` — tab cycles Overview→Files→Install→Overview
- `TestDetailTabShortcuts` — 1/2/3 jump to tabs
- `TestDetailTabBlockedDuringAction` — No switching when confirmAction active
- `TestDetailTabBlockedDuringFileView` — No switching when viewingFile=true

**Overview tab:**
- `TestDetailOverviewReadme` — README renders when ReadmeBody set
- `TestDetailOverviewNoReadme` — "No README.md available" fallback
- `TestDetailOverviewPromptBody` — Prompt body renders
- `TestDetailOverviewAppProviders` — SupportedProviders list
- `TestDetailOverviewMetadata` — Type, path, provider shown
- `TestDetailOverviewLLMPrompt` — LLM prompt preview for local items
- `TestDetailOverviewScroll` — Up/down scrolling, indicators

**Files tab:**
- `TestDetailFilesNavigation` — File list cursor movement
- `TestDetailFilesEnterOpens` — Enter reads file, viewingFile=true
- `TestDetailFilesViewerLineNumbers` — Line number format
- `TestDetailFilesViewerScroll` — Scroll up/down in viewer
- `TestDetailFilesViewerEsc` — Esc closes viewer
- `TestDetailFilesEmpty` — No files message

**Install tab (checkboxes):**
- `TestDetailInstallCheckboxNav` — checkCursor movement
- `TestDetailInstallCheckboxToggle` — Space/enter toggles
- `TestDetailInstallPreChecked` — Already-installed providers pre-checked

**Install tab (install flow):**
- `TestDetailInstallStart` — i opens method picker or installs directly
- `TestDetailInstallMethodPicker` — Navigate, confirm
- `TestDetailInstallAlreadyInstalled` — "already installed" message
- `TestDetailInstallNoProviders` — "No providers detected" message

**Install tab (uninstall flow):**
- `TestDetailUninstallFlow` — Double-press u to confirm
- `TestDetailUninstallNotInstalled` — "Not installed" message
- `TestDetailUninstallEscCancels` — Esc cancels confirmation

**Install tab (prompts):**
- `TestDetailPromptCopy` — c triggers copy
- `TestDetailPromptSavePath` — s opens path input, enter → method picker, esc cancels

**Install tab (promote):**
- `TestDetailPromoteLocal` — Double-press p, command returned
- `TestDetailPromoteNonLocal` — p does nothing
- `TestDetailPromoteEscCancels` — Esc cancels confirmation

**Back navigation:**
- `TestDetailBackCancelsPendingAction` — Esc cancels before navigating

### `detail_env_test.go` (~12 tests)

| Test | What it covers |
|------|---------------|
| `TestEnvSetupStart` | e opens wizard, envVarNames populated |
| `TestEnvChooseNewValue` | Select "Set up new" → actionEnvValue |
| `TestEnvChooseAlreadyConfigured` | Select "Already configured" → actionEnvSource |
| `TestEnvChooseNavigation` | Cursor between 2 options |
| `TestEnvChooseSkip` | Esc skips to next var |
| `TestEnvValueInput` | Type value, enter → actionEnvLocation |
| `TestEnvValueEsc` | Back to actionEnvChoose |
| `TestEnvLocationInput` | Enter → advances to next var |
| `TestEnvLocationEsc` | Back to actionEnvValue |
| `TestEnvSourceInput` | Type path, enter → next var |
| `TestEnvSourceEsc` | Back to actionEnvChoose |
| `TestEnvAllComplete` | After all vars → actionNone |

### `settings_test.go` (~12 tests)

| Test | What it covers |
|------|---------------|
| `TestSettingsNavigation` | 3 rows, bounds clamping |
| `TestSettingsAutoUpdateToggle` | Enter/space toggles bool |
| `TestSettingsProviderSubPicker` | Enter opens editProviders |
| `TestSettingsProviderPickerNav` | Navigate sub-picker |
| `TestSettingsProviderPickerToggle` | Space toggles checkbox |
| `TestSettingsProviderPickerEscApplies` | Esc closes + applies selections |
| `TestSettingsDetectorSubPicker` | Enter opens editDetectors |
| `TestSettingsDetectorPickerToggle` | Toggle + apply flow |
| `TestSettingsSave` | s saves to disk |
| `TestSettingsBackNav` | Esc → screenCategory |
| `TestSettingsBackCancelsSubPicker` | Esc from app closes sub-picker first |
| `TestSettingsViewRendering` | View contains setting labels and values |

### `search_test.go` (~12 tests)

| Test | What it covers |
|------|---------------|
| `TestSearchActivateCategory` | / activates from category |
| `TestSearchActivateItems` | / activates from items |
| `TestSearchBlockedImport` | / does NOT activate from import |
| `TestSearchBlockedUpdate` | / does NOT activate from update |
| `TestSearchBlockedSettings` | / does NOT activate from settings |
| `TestSearchBlockedDetailTextInput` | / blocked when detail has text input |
| `TestSearchTypeQuery` | Characters update query |
| `TestSearchEnterFromCategory` | Enter → SearchResults items |
| `TestSearchEnterFromItems` | Enter → filtered same-type items |
| `TestSearchEscCancels` | Esc deactivates, clears query |
| `TestSearchEscResetsItems` | Esc from items resets to full list |
| `TestFilterItemsFunction` | Unit test filterItems() directly |

### `import_test.go` (~35 tests)

Covers all 14 import steps across 3 flows (local, git, create):

- **Source selection**: navigation, select each option, esc → category
- **Type selection**: navigation, universal → stepBrowseStart, provider-specific → stepProvider, create → stepName, esc back
- **Provider selection**: navigation, select → stepBrowseStart, esc back
- **Browse start**: 3 options, esc back
- **Path input**: valid/invalid/empty path, esc back
- **Git URL**: valid/invalid/empty URL, esc back
- **Clone done msg**: success (→ stepGitPick), error (stays), empty (error)
- **Git pick**: navigation, select → stepConfirm, esc back + cleanup
- **Name input**: enter with name → stepConfirm, empty (stays), esc back
- **Validate**: navigation, toggle, enter single/batch/none-selected, esc back
- **Confirm**: enter imports/scaffolds, esc back
- **Import done msg**: success → category + rescan, error → stays

### `update_test.go` (~18 tests)

Covers all 4 update steps + async messages:

- **Menu**: navigation, see what's new, update now, check for updates
- **Preview**: scroll, enter → pull, esc back
- **Pull**: keys ignored while running
- **Done**: esc → category (at app level)
- **Messages**: updateCheckMsg, updatePreviewMsg, updatePullMsg (success + error)
- **Auto-update**: updateCheckMsg triggers immediate pull
- **Utilities**: versionNewer(), parseVersion()

### `integration_test.go` (~8 teatest tests)

Full-program tests using `teatest.NewTestModel`:

| Test | What it covers |
|------|---------------|
| `TestTeatestCategoryToItems` | Start → enter type → items render → esc back → q quit |
| `TestTeatestSearchFlow` | / → type → enter → SearchResults render |
| `TestTeatestDetailTabs` | Navigate to detail → tab cycle → output changes |
| `TestTeatestSettingsToggle` | Navigate to settings → toggle → output changes |
| `TestTeatestImportStart` | Navigate to import → source options render |
| `TestTeatestQuit` | q quits from category |
| `TestTeatestCtrlCAnywhere` | ctrl+c quits from deep in the app |
| `TestTeatestWindowResize` | Small window → "Terminal too small" |

Uses a `testableApp` wrapper that suppresses `Init()` (skips git fetch).

---

## Side Effect Handling

| Side effect | Test approach |
|-------------|--------------|
| Installer (filesystem) | Test state transitions (confirmAction, message text), not actual files. Provider InstallDir points to temp dirs. |
| Clipboard | Test state change (message text). Clipboard fails gracefully in CI. |
| Git clone/fetch/pull | Verify commands are returned (non-nil). Test message handling by sending result msgs directly. |
| tea.ExecProcess | Verify command returned for items with install.sh, nil for items without. |
| os.Setenv (env setup) | Use `t.Setenv()` for auto-restore. |

---

## Implementation Order

1. Add teatest dependency (`go get`)
2. `testhelpers_test.go` — all fixtures
3. `category_test.go` — validates fixtures work
4. `items_test.go` — builds on category
5. `search_test.go` — app-level key routing
6. `detail_test.go` — largest, most complex
7. `detail_env_test.go` — multi-step wizard
8. `settings_test.go` — sub-picker pattern
9. `import_test.go` — 14-step wizard
10. `update_test.go` — async messages
11. `integration_test.go` — teatest full-program tests

Each file is independent and testable after helpers are in place.

---

## Verification

```bash
cd cli && go test ./internal/tui/... -v -count=1
```

Expected: ~165 tests, all passing. Run after each file to catch issues early.

For teatest golden files (if used):
```bash
go test ./internal/tui/... -update  # Generate/update golden files
go test ./internal/tui/... -v       # Verify against golden files
```
