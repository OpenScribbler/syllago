# Plan: library-unified-view

## Execution order

1. **Slice 1: Registry Clone items visible in Library tab** — test bead + impl bead (TDD)
   - Test: `cli/internal/tui/library_test.go` — asserts `TestLibraryTable_RegistryCloneItemRenderedMuted`, `TestLibraryTable_LibraryItemRenderedNormal`, `TestLibraryTable_ProjectContentRenderedNormal`, `TestGoldenLibrary_UnifiedList`
   - Impl: `cli/internal/tui/table.go`, `cli/internal/tui/styles.go` — removes Library gate in row renderer; adds muted styling + `[not in library]` chip; introduces shared `notInLibrary()` predicate
   - Checkpoint: `go test ./internal/tui/ -run "TestLibraryTable|TestGoldenLibrary_UnifiedList"` passes all four cases

2. **Slice 2: Filter chips narrow Library list by state** — test bead + impl bead (TDD)
   - Test: `cli/internal/tui/library_test.go` — asserts `TestLibraryFilter_AllShowsEverything`, `TestLibraryFilter_NotInLibraryNarrows`, `TestLibraryFilter_InLibraryNarrows`, `TestLibraryFilter_ProjectNarrows`, `TestLibraryFilter_ChipKeyActivates`, `TestLibraryFilter_MouseChipClick`, `TestGoldenLibrary_FilterChipsRendered`
   - Impl: `cli/internal/tui/library.go`, `cli/internal/tui/styles.go`, `cli/internal/tui/keys.go` — `libraryFilter` type, `filteredItems()` helper, chip key bindings, chip zone marks + mouse handler
   - Checkpoint: `go test ./internal/tui/ -run "TestLibraryFilter|TestGoldenLibrary_FilterChips"` passes all seven cases
   - Deps: Slice 1 impl (notInLibrary predicate)

3. **Slice 3: Metapanel Add and Add+Install buttons for Registry Clone items** — test bead + impl bead (TDD)
   - Test: `cli/internal/tui/library_test.go`, `cli/internal/tui/app_update_test.go` — asserts `TestMetapanel_AddButtonsForRegistryClone`, `TestMetapanel_EditButtonForLibraryItem`, `TestMetapanel_NoAddButtonsForProjectContent`, `TestLibraryAddMsg_EmittedOnKeyA`, `TestLibraryAddInstallMsg_EmittedOnKeyI`, `TestApp_HandleLibraryAddMsg_ShowsToast`
   - Impl: `cli/internal/tui/metapanel.go`, `cli/internal/tui/library.go`, `cli/internal/tui/app_update.go` — button matrix update, `libraryAddMsg`/`libraryAddInstallMsg` types, message handlers
   - Checkpoint: `go test ./internal/tui/ -run "TestMetapanel|TestLibraryAdd|TestApp_HandleLibraryAdd"` passes all six cases
   - Deps: Slice 1 impl (Library/Registry discriminators)

4. **Slice 4: `syllago list --filter` state-based filtering** — test bead + impl bead (TDD)
   - Test: `cli/cmd/syllago/list_test.go` — asserts `TestFilterByState_InLibrary`, `TestFilterByState_NotInLibrary`, `TestFilterByState_Project`, `TestFilterByState_MultipleStates`, `TestRunList_FilterFlag_NotInLibrary`, `TestRunList_FilterFlag_Combined`, `TestTelemetryCatalog_FilterPropertyPresent`
   - Impl: `cli/cmd/syllago/list.go`, `cli/internal/telemetry/catalog.go`, `docs/telemetry.json` — `--filter` StringArray flag, `filterByState` predicate, PropertyDef, gendocs
   - Checkpoint: `go test ./... -run "TestFilterByState|TestRunList_Filter|TestTelemetryCatalog_Filter"` passes all seven cases

5. **Slice 5: `syllago add --install --to` chains add and install** — test bead + impl bead (TDD)
   - Test: `cli/cmd/syllago/add_cmd_test.go` — asserts `TestRunAdd_InstallFlagRequiresToFlag`, `TestRunAdd_InstallFlagChainsInstall`, `TestRunAdd_InstallFlagAddFailureSkipsInstall`, `TestRunAdd_InstallFlagDryRunSkipsInstall`, `TestTelemetryCatalog_InstallPropertyPresent`
   - Impl: `cli/cmd/syllago/add_cmd.go`, `cli/internal/telemetry/catalog.go`, `docs/telemetry.json` — `--install` + `--to` flags, post-add install branch, PropertyDef, gendocs
   - Checkpoint: `go test ./... -run "TestRunAdd_Install|TestTelemetryCatalog_Install"` passes all five cases

6. **Slice 6: Install error suggestion names the add command** — test bead + impl bead (TDD)
   - Test: `cli/cmd/syllago/install_cmd_test.go` — asserts `TestInstall_RegistryOnlyItemSuggestion`, `TestInstall_NonExistentItemFallbackSuggestion`
   - Impl: `cli/cmd/syllago/install_cmd.go` — catalog scan for Registry Clone match at not-found error sites; enriched suggestion string
   - Checkpoint: `go test ./... -run "TestInstall_RegistryOnly|TestInstall_NonExistent"` passes both cases

7. **Slice 7: Registry-add hint guides user to the add workflow** — test bead + impl bead (TDD)
   - Test: `cli/internal/tui/app_test.go`, `cli/cmd/syllago/registry_cmd_test.go` — asserts `TestHintModal_ShownWhenPreferenceAbsent`, `TestHintModal_NotShownWhenPreferenceDismissed`, `TestHintModal_DismissPersistsPreference`, `TestRegistryCmd_HintPrintedOnFirstAdd`, `TestRegistryCmd_HintSuppressedAfterDismiss`, `TestGoldenHintModal`
   - Impl: `cli/internal/tui/app.go`, `cli/internal/tui/app_update.go`, `cli/internal/tui/app_view.go`, `cli/cmd/syllago/registry_cmd.go` — `hintModalActive` field, `openHintModalMsg`/`hintDismissedMsg`, modal render, CLI hint paragraph, preference persistence
   - Checkpoint: `go test ./... -run "TestHintModal|TestRegistryCmd_Hint|TestGoldenHintModal"` passes all six cases
   - Deps: Slice 3 impl (add workflow message types exist)

## Gate

Before moving from one slice to the next: the slice's checkpoint must pass. If it fails, stop and involve the user — never skip ahead.

## Acceptance

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

## Non-TDD exemptions

None.
