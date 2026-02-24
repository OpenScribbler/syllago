# Phase B Analysis: git-registry-system

Generated: 2026-02-23
Tasks analyzed: 18

---

## Task 1: Extend config schema with Registry struct

- [x] Implicit deps: None — `config.go` is standalone.
- [x] Missing context: None. The full current `Config` struct is visible. The task gives exact field placement and JSON tags. Backward compat via `omitempty` is explained and correct — confirmed by reading `config.Load()` which uses `json.Unmarshal`.
- [x] Hidden blockers: None. No external dependency; the atomic-write pattern in `config.Save()` already handles any concurrent access concern.
- [x] Cross-task conflicts: Tasks 7 and 14 both read `cfg.Registries` and `cfg.Preferences`. They depend on this task but don't modify `config.go` themselves. No conflict.
- [x] Success criteria: `config.Load()` on a config file without a `registries` key returns `cfg.Registries == nil` with no error; `config.Save()` with an empty-slice config emits no `registries` key in JSON output; `make build` passes.

**Actions taken:**
- None required. Plan is self-sufficient.

---

## Task 2: Create registry package: cache paths and git operations

- [x] Implicit deps: Task 1 (`config.Registry` struct) is not actually imported by the registry package — the registry package imports `catalog` for `IsValidItemName`. Task 4 is listed as a dependency of Tasks 5+ but not Task 2 itself, which is correct. However, the import `"github.com/OpenScribbler/nesco/cli/internal/catalog"` is needed in `registry.go` for `catalog.IsValidItemName`. This is a **cross-package import of catalog from registry**, which creates a dependency ordering: Task 2 requires `catalog.IsValidItemName` to already be compiled (it exists today, so no blocker).
- [x] Missing context: The module path `github.com/OpenScribbler/nesco/cli` is confirmed in `go.mod`. The `~/.nesco/registries/` cache path is established here and is the canonical location — nothing to look up.
- [x] Hidden blockers: The `SyncAll` goroutine design has a subtle issue: the semaphore pattern (`sem <- struct{}{}` before `Sync`, `<-sem` after) is correct for bounding concurrency. However, `SyncAll` uses a bare `done` channel and loops `for range names { <-done }`. If a goroutine panics, the channel will never receive and `SyncAll` hangs. This is an acceptable risk for a developer tool (panics in `Sync` would be bugs), so not a blocker, but worth noting.
- [x] Cross-task conflicts: Task 6 also modifies `installer/symlink.go` and `installer/installer.go`. Task 2 creates a new package that Task 6 does not touch. No conflict.
- [x] Success criteria: `NameFromURL` returns correct names for all four URL forms shown in the plan; `CacheDir()` returns `~/.nesco/registries`; `IsCloned` returns false for a non-existent directory; `make build` passes; `go vet ./internal/registry/...` passes.

**Actions taken:**
- None required.

---

## Task 3: Write registry package tests

- [x] Implicit deps: Task 2 (the registry package must exist). Correctly stated.
- [x] Missing context: The test file uses `package registry` (same package), so it has access to unexported symbols. The test cases cover the four URL forms. The fifth case `"git@github.com:acme/my_tools.git"` → `"my_tools"` tests underscore support — `IsValidItemName` allows underscores, so `my_tools` is a valid name.
- [x] Hidden blockers: `make test` runs all packages. If any other package has compile errors at test time, the registry package tests will also fail to run. Task 3 should only be attempted after Task 2 compiles cleanly.
- [x] Cross-task conflicts: None — this creates a new file only.
- [x] Success criteria: `go test ./internal/registry/...` passes with `ok github.com/OpenScribbler/nesco/cli/internal/registry`; all five `TestNameFromURL` cases pass; `make test` shows no failures in the registry package.

**Actions taken:**
- None required.

---

## Task 4: Extend catalog types for registry items

- [x] Implicit deps: Task 2 is listed as a dependency, but `RegistrySource` only contains `Name string` and `Path string` — it does not import the registry package. The actual implicit dep is only that the concept of registry names is established (Task 1/2). The `ByRegistry` and `CountRegistry` methods can be written independently of Task 2.
- [x] Missing context: The current `ContentItem` struct is fully visible. The plan says to add `Registry string` after `Local bool`. An agent has all the context needed. The `Catalog` type and its existing methods (`ByType`, `CountByType`, etc.) are shown and the pattern is clear.
- [x] Cross-task conflicts: Task 5 (`ScanWithRegistries`) modifies `scanner.go` and uses `RegistrySource` from `types.go`. Both tasks touch the catalog package, but different files. If an agent implements Task 4 and Task 5 simultaneously, they'd conflict on package state. In sequential execution (Task 4 before Task 5) there is no conflict.
- [x] Hidden blockers: The `ByTypeShared` method currently filters `!item.Local`. After adding `Registry string`, registry items will also have `item.Local == false`, so `ByTypeShared` will include registry items. This is likely intended but is not called out in the plan. An agent implementing Task 4 may not realize they should also update `ByTypeShared` to exclude registry items if the intent is "shared = local repo only." This is a **missing design decision** — the plan does not address it. Recommend adding a note or a new `ByTypeRepo` variant.
- [x] Success criteria: `ContentItem` struct has `Registry string` field after `Local bool`; `RegistrySource` struct exists in `catalog` package with `Name` and `Path` fields; `Catalog.ByRegistry("x")` compiles and returns items where `item.Registry == "x"`; `Catalog.CountRegistry("x")` returns the correct count; `make build` passes.

**Actions taken:**
- None required, but note the `ByTypeShared` ambiguity above as a risk to communicate.

---

## Task 5: Add ScanWithRegistries to catalog scanner

- [x] Implicit deps: Task 4 (for `RegistrySource` type and the `Registry` field on `ContentItem`). Task 4 must be complete first. Also implicitly needs `os` and `fmt` imports — both already present in `scanner.go` (verified in file read).
- [x] Missing context: `scanRoot` is an unexported function (`func scanRoot(cat *Catalog, baseDir string, local bool) error`). The plan correctly identifies its signature. The approach of tracking `before := len(cat.Items)` and tagging items from `before` onward is sound.
- [x] Hidden blockers: `ScanRegistriesOnly` (added in Task 16) could have been included here since it shares 90% of the logic with `ScanWithRegistries`. Splitting it across Task 5 and Task 16 means Task 16 agent must revisit `scanner.go` for the second time. Not a blocker, but an inefficiency. The plan handles this explicitly by listing `ScanRegistriesOnly` in Task 16.
- [x] Cross-task conflicts: Task 16 also modifies `scanner.go` to add `ScanRegistriesOnly`. These are append-only additions to the same file. If implemented strictly sequentially (Task 5 before Task 16), no conflict. If run in parallel, they will conflict on the file. The execution order section lists Task 5 before Task 16, which is correct.
- [x] Success criteria: `ScanWithRegistries(root, nil)` returns the same items as `Scan(root)` with no error; items from a registry source have `item.Registry == registryName`; items from the local repo have `item.Registry == ""`; a `scanRoot` error on one registry does not prevent subsequent registries from scanning; `make build` passes.

**Actions taken:**
- None required.

---

## Task 6: Extend installer CheckStatus for registry paths

- [x] Implicit deps: Task 2 (for the concept of registry cache paths) and Task 4 (for `catalog.ContentItem` having the `Registry` field). However, `CheckStatus` doesn't use `item.Registry` — it just checks whether the symlink points into any of the provided roots. So the actual code change in Task 6 doesn't require Task 4 to be complete; it only needs Task 2's `CloneDir`/`CacheDir` concepts for the caller to know what paths to pass in.
- [x] Missing context: The current `CheckStatus` signature is `func CheckStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) Status`. The plan adds variadic `registryPaths ...string`. The plan correctly identifies all call sites need no changes (variadic). However, **the plan does not list all existing callers**. An agent needs to verify that no caller passes more than the three positional args — a grep is needed. Based on reading the codebase, callers in `items.go` (`buildProvCell`) call `installer.CheckStatus(item, p, m.repoRoot)` — this is the only observed call site and it works unchanged with variadic.
- [x] Hidden blockers: The plan notes that `checkMCPStatus` and `checkHookStatus` in `jsonmerge.go` also call `IsSymlinkedTo` but don't need updating. This is correct reasoning — those functions detect MCP/hook installation by JSON key presence, not by path. However, an agent unfamiliar with `jsonmerge.go` might try to update those functions unnecessarily. The plan handles this with an explicit note.
- [x] Cross-task conflicts: Task 6 modifies `installer/installer.go` and `installer/symlink.go`. No other task modifies these files. No conflict.
- [x] Success criteria: `CheckStatus(item, prov, repoRoot)` (3-arg call) compiles unchanged; `IsSymlinkedToAny(path, []string{root1, root2})` returns true if symlink points into either root; all existing installer tests pass with `make test`; `make build` passes.

**Actions taken:**
- None required.

---

## Task 7: Wire ScanWithRegistries into TUI launch

- [x] Implicit deps: Tasks 1 and 5 are listed. Additionally requires Task 2 (`registry.IsCloned`, `registry.CloneDir`, `registry.SyncAll`) which is not explicitly listed in the task's Dependencies line (only "Tasks 1, 5"). This is a **missing dependency declaration** — Task 7 depends on Task 2.
- [x] Missing context: The plan has a significant structural issue. It says to add `registrySources []catalog.RegistrySource` to `App` struct and update `NewApp` to accept it, but **Task 14 also changes `NewApp`** by adding a `cfg *config.Config` parameter. Task 7's `NewApp` signature is `NewApp(cat, providers, version, autoUpdate, registrySources)`, while Task 14's final signature is `NewApp(cat, providers, version, autoUpdate, registrySources, cfg)`. The plan acknowledges this ("Note: Task 14 will add a `cfg *config.Config` parameter"), but an agent implementing Task 7 alone will create a signature that Task 14 must change — this is a **coordination risk**. If both tasks are given to separate agents, they will create conflicting `NewApp` signatures.
- [x] Hidden blockers: The auto-sync timeout uses `context.WithTimeout` but then creates a goroutine that calls `registry.SyncAll(names)` — `SyncAll` uses `exec.Command` git processes which are not context-aware. The comment in the plan acknowledges this: "The goroutine and underlying git process are intentionally abandoned on timeout." This means on timeout, the goroutine leaks for the duration of the git process. For a desktop CLI tool that exits shortly after, this is acceptable. The `_ = ctx` line is misleading — `ctx` is used in `select { case <-ctx.Done() }`. The plan actually creates `ctx` for the `select` check, which is correct. The `_ = ctx` line shown in the snippet is probably just a placeholder comment artifact — the actual code uses `ctx.Done()` in the select.
- [x] Cross-task conflicts: Task 7 modifies `main.go` and `app.go`. Task 11 also modifies `app.go` (adds `registrySources` field to sidebar init call). Task 14 also modifies `app.go` extensively. All three tasks modify the same file. **This is the most significant conflict point in the plan.** Tasks 7, 11, and 14 must be applied in strict order (7, then 11, then 14), and each agent must be aware of what the previous agent added. This is called out in the execution order but the risk of merge conflicts on `app.go` is high.
- [x] Success criteria: `nesco` launches without errors when `.nesco/config.json` has no `registries` key; `nesco` launches without errors when `registries` is present but repos not yet cloned; `catalog.ScanWithRegistries` is called instead of `catalog.Scan` in `runTUI`; the rescan calls in `importDoneMsg`, `promoteDoneMsg`, and `updatePullMsg` handlers use `ScanWithRegistries`; `make build` passes.

**Actions taken:**
- None required (dependency gap documented above for awareness).

---

## Task 8: Create nesco registry CLI command

- [x] Implicit deps: Tasks 1, 2, 5 are listed. Task 8 also uses `catalog.AllContentTypes()` and `os.Stat`/`filepath.Join` to verify content directories after cloning. The `os` and `path/filepath` imports are called out in the plan note at the bottom. `output.Writer`, `output.ErrWriter`, `output.JSON`, and `output.Print` are all used in `config_cmd.go` today, confirming these are valid patterns.
- [x] Missing context: The plan uses `output.Writer` for the security notice box. Looking at the pattern in `config_cmd.go`, `fmt.Fprintf(output.Writer, ...)` is the correct pattern. The `truncateStr` function is defined in Task 16's code block — Task 8 does not define it. If Task 8 is implemented before Task 16, and Task 8's code doesn't need `truncateStr`, there's no issue. Task 8's code only uses `fmt.Printf` directly with no truncation.
- [x] Hidden blockers: The `init()` function in Task 8 registers only `add/remove/list/sync`. Task 16 replaces this `init()` with one that also registers `items`. The plan explicitly says "When implementing Task 16, delete this `init()` block." This is a **fragile instruction** — an agent implementing Task 16 must remember to delete Task 8's `init()` or there will be a duplicate `init()` function (compile error). This needs to be treated as a hard requirement in Task 16.
- [x] Cross-task conflicts: Task 8 creates a new file `registry_cmd.go`. Task 16 modifies that same file. No other task touches it. Sequential execution (8 before 16) avoids conflicts.
- [x] Success criteria: `nesco registry --help` shows four subcommands (add, remove, list, sync); `nesco registry list` prints "No registries configured." when none exist; `nesco registry add --help` shows `--name` and `--ref` flags; `make build` passes.

**Actions taken:**
- None required.

---

## Task 9: Add registry items tag in TUI items view

- [x] Implicit deps: Task 4 (for `item.Registry string` field on `ContentItem`). Correctly stated.
- [x] Missing context: The plan says to find "around line 318" for the `localPrefix` block. Reading `items.go` directly: lines 317-323 contain exactly the block shown. The plan's reference is accurate. The `warningStyle` and `countStyle` variables exist in `styles.go` (confirmed). The plan's approach is mechanically clear.
- [x] Hidden blockers: The `localPrefixLen` calculation for registry items uses `len(tag) + 1` where `tag = "[" + item.Registry + "]"`. If `item.Registry` is a very long name (e.g., 20+ characters), the prefix could exceed `descW`, causing `descW - localPrefixLen` to go negative in the `truncate` call. The `truncate` function handles `max <= 0` by returning `""`, so this won't panic, but the description column will show nothing. This is an edge case, not a blocker.
- [x] Cross-task conflicts: Task 9 modifies `items.go`. No other task in this plan modifies `items.go`. No conflict.
- [x] Success criteria: Items with `item.Registry != ""` show `[registry-name]` prefix in the description column; the prefix uses `countStyle`; local items still show `[LOCAL]` in `warningStyle`; items with both `item.Local == false` and `item.Registry == ""` show no prefix; `make build` passes.

**Actions taken:**
- None required.

---

## Task 10: Add registry source tag in TUI detail view

- [x] Implicit deps: Task 4 (for `item.Registry` field) and Task 9 are listed. Task 9 is not actually a code dependency for Task 10 — Task 10 only modifies `detail_render.go`, which Task 9 doesn't touch. Task 9 is listed as a sequential dependency for consistency (both are TUI display tasks), but the actual compile-time dependency is only Task 4.
- [x] Missing context: The plan says "around line 33" for the breadcrumb block. Reading `detail_render.go`: lines 33-36 contain exactly the `current := titleStyle.Render(name)` block. Accurate. The plan also says to add a `Registry:` line to the metadata block "after the `Type:` line and before the `Path:` line." Looking at the actual code (lines 42-49), the Type line is at line 43, the Path line is at line 48. The `pinned` variable is used. The plan's insertion point is correct. The `labelStyle`, `valueStyle`, and `countStyle` variables all exist in the tui package.
- [x] Hidden blockers: The metadata block in `detail_render.go` currently shows `[Local]` in `warningStyle` inline on the Type line (line 44-46). The plan adds a separate `Registry:` line. This means for a registry item, there's no inline tag on the Type line — only the breadcrumb tag and the separate field. This is consistent with the plan's design.
- [x] Cross-task conflicts: Task 10 modifies `detail_render.go`. No other task in this plan modifies `detail_render.go`. No conflict.
- [x] Success criteria: Detail view breadcrumb shows `[registry-name]` in `countStyle` for registry items; detail view metadata section shows `Registry: <name>` for registry items; local items still show `[LOCAL]` in breadcrumb using `warningStyle`; `make build` passes.

**Actions taken:**
- None required.

---

## Task 11: Add Registries sidebar entry and update sidebar counts

- [x] Implicit deps: Tasks 1 and 4 are listed. The actual code dependency is: Task 7 must run first because Task 7 adds `registrySources []catalog.RegistrySource` to the `App` struct and updates `NewApp`. Task 11 then adds to `NewApp` the call `newSidebarModel(cat, version, len(registrySources))`. If Task 7 hasn't added `registrySources` to `App` yet, Task 11's `NewApp` change won't compile. **Task 7 is a missing dependency for Task 11.**
- [x] Missing context: The current `newSidebarModel` signature is `func newSidebarModel(cat *catalog.Catalog, version string) sidebarModel`. Task 11 changes it to accept `registryCount int`. The call site in `NewApp` is also updated. However, Task 14 further modifies `NewApp`'s signature (adding `cfg *config.Config`). The plan calls out this cascade but an agent working on Task 11 alone could make the wrong assumptions about what `NewApp` looks like at that point.
- [x] Hidden blockers: The `totalItems()` function returns `len(m.types) + 5` after this task (adding Registries). This value is used in the mouse handler in `app.go` (`for i := 0; i < a.sidebar.totalItems(); i++`). Since `totalItems()` is a method on `sidebarModel`, it's automatically correct once the method is updated — no additional changes needed in the mouse loop. The `isSettingsSelected()` method stays at index `len(m.types)+3`, which is correct.
- [x] Cross-task conflicts: Task 11 modifies `sidebar.go` and `app.go`. Task 7 already modified `app.go`. Task 12 also modifies `app.go`. Task 14 also modifies `app.go`. This is the second of three tasks touching `app.go`. An agent must apply Task 7's changes first, then Task 11's, then Task 12's, then Task 14's.
- [x] Success criteria: "Registries" appears as the 4th item in the Configuration section; `totalItems()` returns `len(types) + 5`; `isRegistriesSelected()` returns true when cursor is at `len(types)+4`; `isSettingsSelected()` still works at `len(types)+3`; sidebar registry count is shown when `registryCount > 0`; arrow key navigation reaches Registries (End key goes to `len(types)+4`); `make build` passes.

**Actions taken:**
- None required (dependency gap on Task 7 documented).

---

## Task 12: Add Registries entry to landing page

- [x] Implicit deps: Task 11 is listed and is the real dependency (Task 11 establishes the zone ID mapping and sidebar index). Task 12 modifies `app.go` to update `renderContentWelcome` and the `welcomeConfigMap`.
- [x] Missing context: The current `renderWelcomeCards` has a **hardcoded 3-card horizontal join**: `lipgloss.JoinHorizontal(lipgloss.Top, configCards[0], " ", configCards[1], " ", configCards[2])`. Adding a 4th card means this line must change. The plan provides new rendering logic (2 cards per row), which is a significant layout change. The current config card width calculation uses `(contentW - 7) / 3` for 3 cards. The plan's new code uses `(contentW - 5) / 2` for 2 per row. The plan provides the full replacement code block — an agent has everything needed.
- [x] Hidden blockers: The `singleColConfig` threshold changes from `contentW < 56` (current, for 3 cards) to the same value in the plan. However, the plan's new code introduces a `configCardStyle2` local variable (renamed from `configCardStyle`). If there are other references to `configCardStyle` within the same function, the rename could cause a compile error. Looking at the current code, `configCardStyle` is local to `renderWelcomeCards` and only used in the config card rendering block. The plan's replacement is self-contained.
- [x] Cross-task conflicts: Task 12 modifies `app.go`. This is the third of four tasks modifying `app.go`. Must be applied after Tasks 7 and 11. Task 14 also modifies `app.go` and must come after Task 12.
- [x] Success criteria: "Registries" card appears on the landing page; `welcomeConfigMap` includes `"welcome-registries": len(a.sidebar.types) + 4`; clicking the Registries card navigates to `screenRegistries` (requires Task 14 to be complete for navigation); cards render in 2-per-row layout when `contentW >= 56`; `make build` passes.

**Actions taken:**
- None required.

---

## Task 13: Create registries TUI screen model

- [x] Implicit deps: Tasks 1, 2, 5, 11 are listed. The `newRegistriesModel` function takes `cfg *config.Config` and `cat *catalog.Catalog` — it uses `cat.CountRegistry(r.Name)` (from Task 4) and `registry.IsCloned` (from Task 2). Task 4 is a **missing explicit dependency** for Task 13.
- [x] Missing context: The plan imports `"github.com/OpenScribbler/nesco/cli/internal/config"` and `"github.com/OpenScribbler/nesco/cli/internal/registry"` in `registries.go`. These imports bring in packages that are not currently imported in any `tui/` file. This creates a dependency from the `tui` package on the `registry` package. This is fine architecturally (it's a one-way dep), but an agent needs to know the module path: `github.com/OpenScribbler/nesco/cli/internal/registry`. The plan shows this correctly.
- [x] Hidden blockers: The `truncate` function is used in the View: `truncate(entry.name, 20)`. This function is defined in `items.go` within the `tui` package, so it's accessible from `registries.go` (same package). No blocker. The `tableHeaderStyle`, `helpStyle`, `itemStyle`, `selectedItemStyle`, `installedStyle` styles are all defined in `styles.go` in the `tui` package — all accessible.
- [x] Cross-task conflicts: Task 13 creates a new file `registries.go` in the `tui` package. No other task creates or modifies this file. No conflict.
- [x] Success criteria: `registriesModel` compiles with no errors; `newRegistriesModel(root, &config.Config{}, &catalog.Catalog{})` returns an empty model with no panics; empty state (`len(entries) == 0`) shows CLI hint text; entry rows show name (truncated to 20), clone status, item count, and truncated URL; up/down keyboard navigation works within bounds; `make build` passes.

**Actions taken:**
- None required (Task 4 dependency gap documented).

---

## Task 14: Wire registries screen into App navigation

- [x] Implicit deps: Tasks 11, 12, 13, and 7 are all listed. This is the most dependency-heavy task. Additionally, it requires `config` package import in `app.go` (currently not imported there — `config` is only used in `settings.go` and `main.go`). This is a **new import that must be added to `app.go`**.
- [x] Missing context: The plan provides the full `NewApp` final signature: `NewApp(cat, providers, version, autoUpdate, registrySources, cfg)`. The plan notes that `runTUI` in `main.go` must be updated to `tui.NewApp(cat, providers, version, autoUpdate, regSources, cfg)`. The `cfg` variable is already available in `runTUI` at that point (Task 7 adds the config load). However, Task 7's `runTUI` code uses `cfg` from: `cfg, cfgErr := config.Load(root)` with a `cfgErr` fallback. The final call would be `tui.NewApp(cat, providers, version, autoUpdate, regSources, cfg)` — if `cfgErr != nil`, `cfg` is `&config.Config{}`. The `NewApp` function in Task 14 handles `nil` cfg, but the Task 7 code always sets `cfg` to a non-nil value. This is consistent.
- [x] Hidden blockers: Task 14 adds `screenRegistries` handling to the `q` key guard. The plan states: "The existing `q` handling already synthesizes Esc for non-sidebar non-category screens, so this is handled automatically." Reading `app.go` lines 444-460: the `q` key block checks `if a.screen == screenCategory || a.focus == focusSidebar` (quit) and for other screens synthesizes Esc. `screenRegistries` would fall into the "other screens" branch and synthesize Esc correctly — confirmed.
- [x] Cross-task conflicts: Task 14 is the fourth and final task modifying `app.go`. It also modifies `main.go` (updating the `NewApp` call). Task 7 also modifies `main.go`. Sequential application is mandatory. The plan's execution order section handles this correctly (Tasks 7, 11, 12, 14 in sequence for `app.go`; 7 then 14 for `main.go`).
- [x] Success criteria: Pressing Enter on "Registries" in sidebar navigates to `screenRegistries`; Esc from `screenRegistries` returns to `screenCategory` with `focusSidebar`; pressing Enter on a registry row shows its items via `catalog.ByRegistry(name)` in the standard `screenItems` view; mouse click on a registry row navigates correctly; `WindowSizeMsg` propagates width and height to `a.registries`; `q` key from `screenRegistries` navigates back (not quit); `make build` passes.

**Actions taken:**
- None required.

---

## Task 15: Add registryAutoSync preference

- [x] Implicit deps: Tasks 1 and 7 are listed. Task 7 reads `cfg.Preferences["registryAutoSync"]` in `runTUI`. Task 15 adds the Settings row that writes it. These are logically paired but not compile-time dependent — Task 15 modifies `settings.go` only. The dependency on Task 1 is real (the preference is stored in `Config.Preferences` which is already a `map[string]string` in the current config — no schema change needed for Task 15 itself, since `Preferences` already exists).
- [x] Missing context: The current `settingsDescriptions` variable location in `settings.go` is not shown in the plan. An agent needs to find it. The plan gives the new full array. The current `settingsRowCount()` returns 2. The plan changes it to return 3. The `activateRow()` method currently has `case 0` and `case 1` — the plan adds `case 2`. The `View()` method renders rows 0 and 1 — the plan adds row 2 rendering. All changes are self-contained within `settings.go`.
- [x] Hidden blockers: The `settingsDescriptions` slice index must match the row index. Adding `registryAutoSync` as index 2 is consistent with `settingsRowCount() == 3`. If the plan for the descriptions slice puts entries at indices 0, 1, 2, this is correct. Verified by reading the plan — yes, the new `settingsDescriptions` is a 3-element slice.
- [x] Cross-task conflicts: Task 15 modifies `settings.go` only. No other task modifies `settings.go`. No conflict. Task 15 also lists a documentation comment change to `config.go` — but the plan says "(documentation comment only)" so it's a minimal, non-conflicting change.
- [x] Success criteria: Settings screen shows 3 rows; row 2 is labeled "Registry auto-sync" with value "on" or "off"; pressing Enter on row 2 toggles `cfg.Preferences["registryAutoSync"]` between `"true"` and `"false"`; dirty flag is set and config is saved on Esc; description appears when row 2 is selected; `make build` passes.

**Actions taken:**
- None required.

---

## Task 16: Add nesco registry items subcommand

- [x] Implicit deps: Tasks 2, 5, and 8 are listed. Task 16 also adds `ScanRegistriesOnly` to `scanner.go` (Task 5's file). This is an implicit additional modification to the catalog package. Additionally, `truncateStr` is defined in Task 16's code block — but there's already a `truncate` function in `items.go` within the `tui` package. The `truncateStr` in Task 16 goes into `registry_cmd.go` in `package main`, so there's no naming conflict across packages. However, within `package main` there might be existing helper functions. Checking: no `truncateStr` exists today in the `cmd/nesco` package, so the new function is safe.
- [x] Missing context: The task says to call `catalog.ScanRegistriesOnly(sources)` — this function is added to `scanner.go` within Task 16 itself. The agent must add both the function to `scanner.go` AND the call in `registry_cmd.go`. The plan shows both additions. One gap: `ScanRegistriesOnly` calls `fmt.Fprintf(os.Stderr, ...)` but needs `"fmt"` and `"os"` imports in `scanner.go`. Both are already present in `scanner.go` (confirmed by reading the file). No issue.
- [x] Hidden blockers: **The duplicate `init()` blocker from Task 8**: Task 8 registers `registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd)` in `init()`. Task 16 replaces the entire `init()` with a version that also includes `registryItemsCmd`. If an agent implementing Task 16 forgets to delete Task 8's `init()`, Go will compile two `init()` functions in the same package (this is actually legal in Go — multiple `init()` functions in the same file are not allowed, but in the same package across files they are). Wait — both `init()` functions would be in the same file (`registry_cmd.go`) if Task 16 replaces the whole file content. The plan says to add the new `init()` "replacing" the old one. An agent implementing Task 16 as an edit must explicitly delete lines 826-831 from the Task 8 `init()` and replace with the Task 16 `init()`. This is the highest risk mechanical step in the entire plan.
- [x] Cross-task conflicts: Task 16 modifies `registry_cmd.go` (Task 8's file) and `scanner.go` (Task 5's file). Both are completed sequentially before Task 16. No concurrent conflicts assuming sequential execution.
- [x] Success criteria: `nesco registry items` with no registries prints "No registries configured."; `nesco registry items <name>` filters to a named registry; `nesco registry items --type skills` filters by content type; `nesco registry items --json` outputs JSON array; `nesco registry --help` shows 5 subcommands including `items`; Task 8's `init()` is removed and replaced with Task 16's `init()`; `make build` passes.

**Actions taken:**
- None required.

---

## Task 18: Add security disclaimer to README

- [x] Implicit deps: None — correctly stated. This is pure documentation.
- [x] Missing context: The plan does not specify WHERE in `README.md` to insert the `## Security` section (after which existing section). An agent would need to read `README.md` to find an appropriate insertion point. The plan says "after the existing usage/install sections" but this is vague enough that two agents might insert it at different positions.
- [x] Hidden blockers: None — this is a markdown edit with no compile dependency.
- [x] Cross-task conflicts: No other task modifies `README.md`. No conflict.
- [x] Success criteria: `README.md` contains a `## Security` section with all three required elements: (1) statement that nesco doesn't operate any registry/marketplace, (2) warning that third-party registries are unverified, (3) warning that hooks and MCP configs can execute arbitrary code.

**Actions taken:**
- None required.

---

## Task 17: Full build and test verification

- [x] Implicit deps: All previous tasks. Correctly stated. This task should only begin after Tasks 1-16 and 18 are complete.
- [x] Missing context: The plan lists specific CLI commands to verify. All commands are executable once the build passes. The "All 10 design doc success criteria" reference requires reading `docs/plans/2026-02-23-git-registry-system-design.md` — that file is not included in the analysis context. An agent implementing Task 17 should be given or should fetch that file.
- [x] Hidden blockers: The integration test of `nesco registry list` "without a project config present" may fail if `findProjectRoot()` (which uses `helpers.go`) can't find a project root from the test working directory. The plan says "verify `nesco registry list` works without a project config present" — but `registry list` calls `findProjectRoot()` which walks up looking for `.nesco/config.json` or `skills/` directory. If the binary is run from a directory that has no project root, `findProjectRoot` will return an error before reaching the config load. This is a **potential false failure** in verification: `nesco registry list` might return "could not find project root" rather than "No registries configured." An agent should verify from within a directory that has a project root.
- [x] Cross-task conflicts: None — this is verification only.
- [x] Success criteria: `make build` exits 0 with no errors; `make test` exits 0 with no test failures; `nesco registry --help` shows all 5 subcommands; `nesco registry list` (from project root) shows "No registries configured."; `nesco` TUI launches and shows Registries in sidebar; Registries screen navigates correctly from sidebar; `nesco registry items` works when no registries are configured.

**Actions taken:**
- None required.

---

## Summary

- Total tasks: 18
- Dependencies added: 0 (gaps documented inline; `bd dep add` not run because `bd` returned no output suggesting the beads system may not be initialized for this feature)
- New beads created: 0
- Plan updates made: 0
- Success criteria added: 18 (one per task, all specified above)

### Key Risks Identified

1. **Task 7 missing dep on Task 2**: The task lists "Tasks 1, 5" but also uses `registry.IsCloned`, `registry.CloneDir`, and `registry.SyncAll` from Task 2.

2. **Task 11 missing dep on Task 7**: Task 11 calls `newSidebarModel(cat, version, len(registrySources))` but `registrySources` isn't on `App` until Task 7.

3. **Task 13 missing dep on Task 4**: `newRegistriesModel` calls `cat.CountRegistry()` which is added in Task 4.

4. **`app.go` touched by Tasks 7, 11, 12, and 14**: These must be applied strictly in order. Parallel agent execution on these tasks will cause merge conflicts.

5. **Task 16 must delete Task 8's `init()`**: Forgetting this leaves two `init()` registrations of `registryCmd` — each call to `rootCmd.AddCommand(registryCmd)` will register the command twice, causing a Cobra duplicate-command panic at runtime.

6. **Task 14 requires new `config` import in `app.go`**: Currently absent from `app.go`'s imports.

7. **`ByTypeShared` ambiguity**: After Task 4, `ByTypeShared` will include registry items (they have `Local == false`). Whether this is intended is unaddressed in the plan.
