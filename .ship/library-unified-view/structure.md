# Structure Outline — library-unified-view

**Feature slug:** library-unified-view
**Design signed:** yes
**Research complete:** yes

---

## Acceptance Criteria

Derived from design success criteria and out-of-scope constraints.

**Must pass:**

1. Library tab renders Library items, Project Content items, and Registry Clone items in one list.
2. Registry Clone items not yet Added render muted with a `[not in library]` chip.
3. Filter chips inside `libraryModel` narrow by state: `all`, `in-library`, `not-in-library`, `project`.
4. Metapanel shows `[a] Add` and `[i] Add + Install` iff `item.Registry != "" && !item.Library`.
5. Metapanel hides `[e] Edit` for non-Library items; shows it only when `item.Library == true`.
6. `syllago list --filter <state>` accepts repeatable values; `--source` is unchanged.
7. `syllago add --install --to <provider>` chains `add.AddItems` then `installer.Install` without a new package.
8. `syllago install <registry-only-item>` error suggestion includes a copy-pasteable `syllago add` command.
9. After Registry Add, TUI shows a one-time hint modal suppressible via `Preferences["hints.registry_add_dismissed"]`.
10. After Registry Add, CLI prints the hint paragraph once (suppressed if preference already set).
11. Three telemetry `PropertyDef` entries added: `filter` (string, list), `install` (bool, add), `item_count` commands annotation updated.
12. All new public functions have unit tests; all changed commands have integration tests.

**Out of scope (must NOT appear):**

- Auto-copying Registry content to Library on registry add.
- `[u] Update` button or conflict behavior.
- New orchestrator package.
- Project Content detection changes.
- `filterBySource` default-case bug fix.
- Pagination or virtual scroll.
- `syllago config set` wiring.

---

## Slice Map

| # | Name | Observable outcome |
|---|------|--------------------|
| 1 | Registry Clone items visible in Library tab | User sees Registry Clone items listed alongside Library items; not-in-library items render muted |
| 2 | Filter chips narrow Library list by state | User presses a filter chip key and sees list narrow to matching items |
| 3 | Metapanel Add and Add+Install buttons for Registry Clone items | User sees `[a] Add` and `[i] Add + Install` in metapanel for not-in-library items; `[e] Edit` hidden |
| 4 | `syllago list --filter` state-based filtering | `syllago list --filter not-in-library` returns only registry-clone items not yet added |
| 5 | `syllago add --install --to` chains add and install | One command adds and installs; `--install` without `--to` is an error |
| 6 | Install error suggestion names the add command | `syllago install registry-only-item` error message includes `syllago add` command |
| 7 | Registry-add hint guides user to the add workflow | One-time hint appears after registry add; dismissal is persisted to config |

---

### Slice 1: Registry Clone items visible in Library tab

**Observable outcome:** After a Registry is added and the catalog is rescanned, the Library tab list includes Registry Clone items alongside existing Library and Project Content items. Items where `item.Registry != "" && !item.Library` render with muted row styling and a `[not in library]` chip in the Name column. Library and Project Content items are unaffected.

**Files:**

- `cli/internal/tui/table.go` — row renderer: apply `mutedStyle` + `[not in library]` chip when `item.Registry != "" && !item.Library`; remove implicit `item.Library` gate that hid registry-clone rows
- `cli/internal/tui/styles.go` — `notInLibraryChipStyle` constant
- `cli/internal/tui/library_test.go` — unit + golden tests

**Interfaces introduced or modified:**

- `tableModel` row renderer (`cli/internal/tui/table.go`) — internal render function; no exported signature change. **Deps:** `in-process`. **Hides:** `notInLibraryChip` text and `mutedStyle` application logic. **Exposes:** visual row string consumed by `View()`.
- `catalog.ContentItem` discriminator fields (`cli/internal/catalog/types.go`) — read-only use of existing `Library bool`, `Registry string`. **Deps:** `in-process`. **Hides:** nothing new. **Exposes:** existing fields; no new exported symbols.

**Test cases:**

- Unit: `TestLibraryTable_RegistryCloneItemRenderedMuted` — item with `Registry="test-reg"` and `Library=false` produces a row containing `[not in library]` and muted styling.
- Unit: `TestLibraryTable_LibraryItemRenderedNormal` — item with `Library=true` does NOT produce `[not in library]` in the row.
- Unit: `TestLibraryTable_ProjectContentRenderedNormal` — item with `Library=false` and `Registry=""` does NOT produce `[not in library]`.
- Integration (golden): `TestGoldenLibrary_UnifiedList` — snapshot at 80x30 with all three item categories shows muted row for registry clone alongside normal rows.

**Checkpoint:** `go test ./internal/tui/ -run "TestLibraryTable|TestGoldenLibrary_UnifiedList"` passes all four cases. Running `-update-golden` and inspecting the diff shows a muted row for the registry clone item.

---

### Slice 2: Filter chips narrow Library list by state

**Observable outcome:** The Library tab displays a chip row: `[all]  [in-library]  [not-in-library]  [project]`. Pressing the corresponding key or clicking a chip narrows the visible list to matching items and highlights the active chip. The full item slice is still passed via `a.library.SetItems(a.catalog.Items)` — filtering is internal to `libraryModel`.

**Files:**

- `cli/internal/tui/library.go` — `libraryFilter` type, `filteredItems()` helper, keyboard handler for chip key activation, `SetSize` chip-row height accounting
- `cli/internal/tui/library.go` — `updateMouse` extension for chip zone clicks
- `cli/internal/tui/styles.go` — `activeChipStyle`, `inactiveChipStyle` if not already present
- `cli/internal/tui/keys.go` — chip key bindings
- `cli/internal/tui/library_test.go` — unit + golden tests

**Interfaces introduced or modified:**

- `libraryModel` filter state (`cli/internal/tui/library.go`) — new unexported field `activeFilter libraryFilter`; new method `filteredItems() []catalog.ContentItem`. **Deps:** `in-process`. **Hides:** filter predicate switch, chip layout arithmetic, cursor reset on filter change. **Exposes:** chip row in `View()` output; external callers (`app.go`) continue passing the full unfiltered slice via `SetItems`.
- `tableModel.SetItems` (`cli/internal/tui/table.go`) — called again after filter change to swap visible rows; existing signature unchanged. **Deps:** `in-process`. **Hides:** cursor reset on `SetItems`. **Exposes:** unchanged `Selected()` behavior.

**Test cases:**

- Unit: `TestLibraryFilter_AllShowsEverything` — with mixed items, `filterAll` returns all three categories.
- Unit: `TestLibraryFilter_NotInLibraryNarrows` — `filterNotInLibrary` returns only items where `item.Registry != "" && !item.Library`.
- Unit: `TestLibraryFilter_InLibraryNarrows` — `filterInLibrary` returns only `item.Library == true` items.
- Unit: `TestLibraryFilter_ProjectNarrows` — `filterProject` returns only `!item.Library && item.Registry == ""` items.
- Unit: `TestLibraryFilter_ChipKeyActivates` — simulating chip key press transitions `activeFilter` and calls `SetItems` on the table with narrowed results.
- Unit: `TestLibraryFilter_MouseChipClick` — clicking chip zone updates `activeFilter`.
- Integration (golden): `TestGoldenLibrary_FilterChipsRendered` — golden at 80x30 shows chip row with active chip highlighted.

**Checkpoint:** `go test ./internal/tui/ -run "TestLibraryFilter|TestGoldenLibrary_FilterChips"` passes all seven cases. Pressing filter key in a manual smoke session narrows visible rows and highlights the active chip.

---

### Slice 3: Metapanel Add and Add+Install buttons for Registry Clone items

**Observable outcome:** Selecting a not-in-library Registry Clone item shows `[a] Add` and `[i] Add + Install` in the metapanel; `[e] Edit` is absent. Selecting a Library item shows `[e] Edit`; `[a]`/`[i] Add + Install` are absent. Pressing `[a]` triggers the add pipeline silently and shows a success toast. Pressing `[i] Add + Install` opens the install wizard pre-seeded with the item. Both keys and mouse clicks work.

**Files:**

- `cli/internal/tui/metapanel.go` — button matrix: add `[a] Add` and `[i] Add + Install` conditions; gate `[e] Edit` to `item.Library == true`
- `cli/internal/tui/library.go` — `libraryAddMsg` and `libraryAddInstallMsg` message types; keyboard + mouse handlers emitting these messages
- `cli/internal/tui/app_update.go` — handle `libraryAddMsg` (call `add.AddItems` via `tea.Cmd`, push success toast, rescan); handle `libraryAddInstallMsg` (open install wizard pre-seeded)
- `cli/internal/tui/library_test.go` — unit tests for button conditions and message emission
- `cli/internal/tui/app_update_test.go` — message routing tests

**Interfaces introduced or modified:**

- `metapanel.go` button renderer (`cli/internal/tui/metapanel.go`) — internal condition block changes; no exported signature change. **Deps:** `in-process`. **Hides:** button visibility predicates for all button types. **Exposes:** rendered button row string; callers receive the same string interface.
- `libraryAddMsg` / `libraryAddInstallMsg` (`cli/internal/tui/library.go`) — `type libraryAddMsg struct{ item *catalog.ContentItem }` and `type libraryAddInstallMsg struct{ item *catalog.ContentItem }`. **Deps:** `in-process`. **Hides:** item pointer plumbing. **Exposes:** typed messages routed through `App.Update`.
- `add.AddItems` (`cli/internal/add`) — existing function; new `tea.Cmd` call site in `app_update.go`. **Deps:** `local-substitutable` — tests use filesystem override (`catalog.GlobalContentDirOverride`) and temp dirs. **Hides:** copy and conversion logic. **Exposes:** `([]catalog.ContentItem, error)` return; TUI wraps result in a done-message.

**Test cases:**

- Unit: `TestMetapanel_AddButtonsForRegistryClone` — not-in-library item produces row containing `[a] Add` and `[i] Add + Install`, does not contain `[e] Edit`.
- Unit: `TestMetapanel_EditButtonForLibraryItem` — Library item produces row containing `[e] Edit`, does not contain `[a] Add`.
- Unit: `TestMetapanel_NoAddButtonsForProjectContent` — project-content item (`!item.Library && item.Registry == ""`) produces row without `[a] Add`.
- Unit: `TestLibraryAddMsg_EmittedOnKeyA` — pressing `a` on a not-in-library selected row emits `libraryAddMsg`.
- Unit: `TestLibraryAddInstallMsg_EmittedOnKeyI` — pressing `i` on a not-in-library selected row emits `libraryAddInstallMsg`.
- Unit: `TestApp_HandleLibraryAddMsg_ShowsToast` — `handleLibraryAddDone` with success result pushes success toast and returns rescan command.

**Checkpoint:** `go test ./internal/tui/ -run "TestMetapanel|TestLibraryAdd|TestApp_HandleLibraryAdd"` passes all six cases. Manual smoke: select a registry clone item and confirm `[a] Add` appears; select a Library item and confirm `[e] Edit` appears and `[a] Add` is absent.

---

### Slice 4: `syllago list --filter` state-based filtering

**Observable outcome:** `syllago list --filter not-in-library` returns only items where `item.Registry != "" && !item.Library`. The flag is repeatable; `--filter in-library --filter project` is a valid OR union. `--source` is unchanged. Text and JSON output use existing formats. Telemetry gains a `filter` property.

**Files:**

- `cli/cmd/syllago/list.go` — add `--filter` flag (StringArray); apply `filterByState` after `filterBySource` inside `runList`; enrich `filter` telemetry property
- `cli/cmd/syllago/list.go` — `filterByState(item catalog.ContentItem, states []string) bool` predicate (inline or local helper)
- `cli/internal/telemetry/catalog.go` — add `PropertyDef{Name: "filter", Type: "string", Commands: []string{"list"}}`
- `docs/telemetry.json` — regenerated via `cd cli && make gendocs`
- `cli/cmd/syllago/list_test.go` — table-driven tests

**Interfaces introduced or modified:**

- `filterByState` (`cli/cmd/syllago/list.go`) — `func filterByState(item catalog.ContentItem, states []string) bool`; accepted values: `in-library`, `not-in-library`, `installed`, `not-installed`, `project`. **Deps:** `in-process`. **Hides:** per-state predicate switch. **Exposes:** bool result consumed by `runList`; empty `states` slice returns true (no filter applied).
- `runList` (`cli/cmd/syllago/list.go`) — reads new `--filter` StringArray flag; applies `filterByState` after existing `filterBySource`. **Deps:** `in-process`. **Hides:** filter chaining order. **Exposes:** filtered `listResult`; JSON schema unchanged.
- `telemetry.EventCatalog` (`cli/internal/telemetry/catalog.go`) — new `PropertyDef` entry for `filter`. **Deps:** `in-process`. **Hides:** catalog slice construction. **Exposes:** updated `EventCatalog()` return; drift test gates correctness.

**Test cases:**

- Unit: `TestFilterByState_InLibrary` — `Library=true` item matches `in-library`, does not match `not-in-library`.
- Unit: `TestFilterByState_NotInLibrary` — `Registry="r"` and `Library=false` item matches `not-in-library`.
- Unit: `TestFilterByState_Project` — `Library=false` and `Registry=""` item matches `project`.
- Unit: `TestFilterByState_MultipleStates` — item matching any supplied state is included (OR semantics).
- Integration: `TestRunList_FilterFlag_NotInLibrary` — temp catalog with mixed items, `--filter not-in-library` returns only registry-clone items.
- Integration: `TestRunList_FilterFlag_Combined` — `--filter in-library --filter project` returns both categories.
- Unit: `TestTelemetryCatalog_FilterPropertyPresent` — `EventCatalog()` contains `PropertyDef` with `Name=="filter"` in commands for `"list"`.

**Checkpoint:** `go test ./... -run "TestFilterByState|TestRunList_Filter|TestTelemetryCatalog_Filter"` passes all seven cases. `syllago list --filter not-in-library` on a real catalog with a synced registry returns only unadded Registry Clone items.

---

### Slice 5: `syllago add --install --to` chains add and install

**Observable outcome:** `syllago add rules/my-rule --from my-registry --install --to claude-code` adds the item to the Library and then installs it to the named provider without a second command. `--install` without `--to` returns an error. Items that fail the add step are excluded from the install step. `--dry-run --install` does not call `installer.Install`. Telemetry gains an `install` bool property on the `add` command.

**Files:**

- `cli/cmd/syllago/add_cmd.go` — add `--install` (bool) and `--to` (string) flags; post-`add.AddItems` branch: if `--install` is set, resolve provider by slug, call `installer.Install` for each successfully added item; enrich `install` telemetry property
- `cli/internal/telemetry/catalog.go` — add `PropertyDef{Name: "install", Type: "bool", Commands: []string{"add"}}`
- `docs/telemetry.json` — regenerated
- `cli/cmd/syllago/add_cmd_test.go` — table-driven tests

**Interfaces introduced or modified:**

- `runAdd` (`cli/cmd/syllago/add_cmd.go`) — new flags `--install bool` and `--to string`; post-add install branch. **Deps:** `in-process`. **Hides:** install chaining logic and provider slug resolution. **Exposes:** extended command surface; existing `add.AddItems` call site and behavior preserved for callers not using `--install`.
- `installer.Install` (`cli/internal/installer`) — existing function; new call site in `add_cmd.go` when `--install` is set. **Deps:** `local-substitutable` — tests use `provider.AllProviders` override with provider stubs and temp dirs. **Hides:** provider-specific install mechanics. **Exposes:** `(InstallResult, error)` consumed by `runAdd`.
- `telemetry.EventCatalog` (`cli/internal/telemetry/catalog.go`) — new `PropertyDef` for `install`. **Deps:** `in-process`. **Hides:** catalog slice. **Exposes:** updated `EventCatalog()` return; drift test enforces.

**Test cases:**

- Unit: `TestRunAdd_InstallFlagRequiresToFlag` — `--install` without `--to` returns an input-missing error.
- Integration: `TestRunAdd_InstallFlagChainsInstall` — `--install --to claude-code` with a valid item in a temp catalog calls `installer.Install`; verified via provider stub + symlink check.
- Unit: `TestRunAdd_InstallFlagAddFailureSkipsInstall` — when `add.AddItems` returns no successfully added items, `installer.Install` is not called.
- Unit: `TestRunAdd_InstallFlagDryRunSkipsInstall` — `--dry-run --install` does not call `installer.Install`.
- Unit: `TestTelemetryCatalog_InstallPropertyPresent` — `EventCatalog()` contains `PropertyDef` with `Name=="install"` for `"add"`.

**Checkpoint:** `go test ./... -run "TestRunAdd_Install|TestTelemetryCatalog_Install"` passes all five cases. `syllago add rules/test-rule --from test-registry --install --to claude-code` on a fixture catalog completes without requiring a second `syllago install` command.

---

### Slice 6: Install error suggestion names the add command

**Observable outcome:** Running `syllago install rules/registry-only-item --to claude-code` on an item present in the catalog only as a Registry Clone produces an error suggestion reading: `"This item exists in registry 'name'. Add it first: syllago add rules/registry-only-item --from name"`. Items not in the catalog at all retain the existing `syllago list --type` hint. The error code (`INSTALL_002` / `ITEM_001`) is unchanged.

**Files:**

- `cli/cmd/syllago/install_cmd.go` — at the two not-found error sites (lines ~308 and ~552): after the not-in-library check, scan the full catalog for a Registry Clone match; if found, use the enriched suggestion string; otherwise fall through to the existing hint
- `cli/cmd/syllago/install_cmd_test.go` — regression tests

**Interfaces introduced or modified:**

- `runInstall` / `runInstallAll` error paths (`cli/cmd/syllago/install_cmd.go`) — internal suggestion string construction; no exported signature change. **Deps:** `in-process`. **Hides:** catalog lookup for suggestion enrichment, registry-name extraction from `item.Registry`. **Exposes:** improved `output.NewStructuredError` suggestion text via the existing error output channel.
- `catalog.Catalog` lookup (`cli/internal/catalog`) — `ByType` or `Items` scan at the not-found site; read-only new call site on existing catalog instance. **Deps:** `in-process`. **Hides:** item indexing internals. **Exposes:** existing `Items []ContentItem` field; no new methods needed.

**Test cases:**

- Integration: `TestInstall_RegistryOnlyItemSuggestion` — catalog with a Registry Clone item (not in Library), `syllago install rules/item-name --to cc`; error suggestion contains `syllago add rules/item-name --from registry-name`.
- Integration: `TestInstall_NonExistentItemFallbackSuggestion` — item absent from catalog entirely; suggestion falls back to existing `syllago list --type rules` hint without regression.

**Checkpoint:** `go test ./... -run "TestInstall_RegistryOnly|TestInstall_NonExistent"` passes both cases. Manual: `syllago install <registry-clone-name> --to claude-code` prints the `syllago add` command verbatim in the suggestion line.

---

### Slice 7: Registry-add hint guides user to the add workflow

**Observable outcome:** After a Registry is added in the TUI (MOAT or non-MOAT path), if `Preferences["hints.registry_add_dismissed"]` is not `"true"`, a modal explains that Registry Clone items are now visible in the Library tab and can be added with `[a] Add`. Pressing `[dismiss]` writes the preference via `config.SaveGlobal()`. Subsequent Registry adds skip the modal. In the CLI, `syllago registry add <url>` success output includes a one-paragraph add-before-use hint, suppressed if the preference is already set.

**Files:**

- `cli/internal/tui/app.go` — `hintModalActive bool` field; `openHintModalMsg{}` and `hintDismissedMsg{}` types
- `cli/internal/tui/app_update.go` — in `handleRegistryAddDone` and `handleMOATSyncDone`: check preference via `config.LoadGlobal()`, conditionally dispatch `openHintModalMsg`; handle `hintDismissedMsg` to call `config.SaveGlobal()` and clear `hintModalActive`
- `cli/internal/tui/app_view.go` — render hint modal overlay when `hintModalActive == true` (manual border pattern per modal rules; `overlayModal` helper)
- `cli/cmd/syllago/registry_cmd.go` — after success at line ~157: `config.LoadGlobal()`, print hint paragraph if preference absent
- `cli/internal/tui/app_test.go` — unit tests for hint message routing
- `cli/cmd/syllago/registry_cmd_test.go` — CLI output assertion tests

**Interfaces introduced or modified:**

- `openHintModalMsg` / `hintDismissedMsg` (`cli/internal/tui/app.go`) — `type openHintModalMsg struct{}` and `type hintDismissedMsg struct{}`. **Deps:** `in-process`. **Hides:** preference key constant `"hints.registry_add_dismissed"`. **Exposes:** typed messages routed through `App.Update`.
- `config.LoadGlobal` / `config.SaveGlobal` (`cli/internal/config/config.go`) — existing functions; new call sites in TUI `tea.Cmd` closures and CLI success path. **Deps:** `local-substitutable` — tests use temp home dir or `catalog.GlobalContentDirOverride`. **Hides:** JSON marshal/unmarshal, file path resolution. **Exposes:** `(*Config, error)` and `error`; `Preferences map[string]string` persisted.
- `registry_cmd.go` hint output — inline print guarded by preference check; no new exported symbols. **Deps:** `in-process`. **Hides:** preference read and string construction. **Exposes:** printed paragraph to stdout.

**Test cases:**

- Unit: `TestHintModal_ShownWhenPreferenceAbsent` — `handleRegistryAddDone` with no preference set produces an `openHintModalMsg` command.
- Unit: `TestHintModal_NotShownWhenPreferenceDismissed` — preference `"hints.registry_add_dismissed"="true"` means no `openHintModalMsg` command.
- Unit: `TestHintModal_DismissPersistsPreference` — `hintDismissedMsg` handler calls `config.SaveGlobal` with the preference set.
- Integration: `TestRegistryCmd_HintPrintedOnFirstAdd` — CLI registry add with no preference set includes hint paragraph in stdout.
- Integration: `TestRegistryCmd_HintSuppressedAfterDismiss` — CLI registry add with preference already set does not include hint paragraph.
- Integration (golden): `TestGoldenHintModal` — golden snapshot of hint modal overlay at 80x30.

**Checkpoint:** `go test ./... -run "TestHintModal|TestRegistryCmd_Hint|TestGoldenHintModal"` passes all six cases. Manual TUI: add a Registry, confirm hint modal appears; press dismiss; add another Registry, confirm no modal. Manual CLI: `syllago registry add <url>` prints hint on first run, suppressed on second.

---

## Patterns

### Item source discrimination (shared predicate)

```go
// notInLibrary returns true for Registry Clone items not yet Added to the Library.
func notInLibrary(item catalog.ContentItem) bool {
    return item.Registry != "" && !item.Library
}
```

This predicate drives row muting (Slice 1), filter chips (Slice 2), metapanel button visibility (Slice 3), and `filterByState` (Slice 4). Define it once; do not duplicate.

### Filter chip rendering (TUI)

Each chip must have a `zone.Mark` call in `View()` and a `zone.Get(...).InBounds(msg)` check in `updateMouse`. Chip key bindings must be declared in `keys.go`. Active chip uses `activeChipStyle`; inactive uses `inactiveChipStyle`.

### Preference read-before-write (hint dismissal)

Load the full config via `config.LoadGlobal()`, mutate only the `Preferences` key, then write back with `config.SaveGlobal()`. Never write a partial config struct.

### Telemetry drift gate

After any change to `telemetry/catalog.go`, regenerate `docs/telemetry.json` via `cd cli && make gendocs`. The `TestGentelemetry_CatalogMatchesEnrichCalls` test fails CI on divergence.

STRUCTURE_COMPLETE
