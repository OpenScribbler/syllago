# Design Discussion — library-unified-view

## Summary

**Current state.** The Library tab renders only Library items (items with `item.Library == true`, sourced from `~/.syllago/content/`). Registry Clone items appear in the catalog but receive no Add affordance in the metapanel; the `[e] Edit` button is always shown regardless of source, and `[d] Remove` is correctly gated but the gate does not account for the not-in-library state. `syllago list --source` accepts `library`, `shared`, `registry`, `builtin`, and `all` as independent silos with no combined state filter. The add pipeline has no install step; chaining Add and Install requires two separate commands.

**Desired state.** The Library tab renders all three item categories — Library, Project Content, and Registry Clone — in a single unified list. Not-yet-Added Registry Clone items appear in muted styling with a `[not in library]` state indicator. Filter chips narrow by item state. The metapanel surfaces `[a] Add` and `[i] Add + Install` for not-in-library Registry Clone items. `syllago list --filter` accepts repeatable state values. `syllago add --install --to <provider>` chains add.AddItems and installer.Install without a new orchestrator package. A one-time hint (suppressible via `Preferences["hints.registry_add_dismissed"]`) explains the add-before-use requirement after a Registry is added.

**End state narrative.** A user adds a Registry and immediately sees its content listed in-place in the Library tab alongside their existing Library items and Project Content, muted to signal the not-in-library state. They select a Registry Clone item, see `[a] Add` and `[i] Add + Install` in the metapanel, and act without switching views. Power users narrow by filter chip. From the CLI, `syllago list --filter not-in-library` and `syllago add --install --to claude-code rules/my-rule` complete the same workflow without opening the TUI. Error messages for `syllago install <registry-only-item>` now include a copy-pasteable `syllago add` command in their suggestion text.

---

## Patterns to Follow

### 1. Item source discrimination — catalog.ContentItem fields

All item category decisions derive from three fields on `catalog.ContentItem` (`cli/internal/catalog/types.go:67–119`):

```go
Library  bool   // true iff item lives in ~/.syllago/content/
Registry string // non-empty = registry name; empty for Library and Project Content
Source   string // "library", "project"/"", or registry name
Path     string // empty only for MOAT-materialized unstaged items
```

The precise categories in use throughout this feature:

| Category | Discriminator |
|---|---|
| Library | `item.Library == true` |
| Project Content | `!item.Library && item.Registry == ""` |
| Registry Clone (on-disk) | `item.Registry != "" && item.Path != ""` |
| Registry Clone (MOAT unstaged) | `item.Path == "" && item.Source != ""` — `isUnstagedRegistryItem` at `cli/internal/tui/library.go:501–503` |

The `not-in-library` state that drives filter chips and muted rendering maps to Registry Clone items only: `item.Registry != "" && !item.Library`. An item may have both `Library == true` and `Registry != ""` when a Registry Clone item has been explicitly Added; in that case it is a Library item (in-library) despite having a registry tag. The filter `in-library` should therefore test `item.Library`, not `item.Registry == ""`.

### 2. SetItems and refreshContent call chain

The full refresh path that must be preserved (`cli/internal/tui/app.go:354–375`):

```go
func (a *App) refreshContent() {
    ch := a.contentHeight()
    if a.isLibraryTab() {
        a.galleryDrillIn = false
        a.library.SetItems(a.catalog.Items)   // app.go:358
        a.library.SetVerification(a.verification)
        a.library.SetInstalled(a.installed)
        a.library.SetSize(a.width, ch)
        return
    }
    // ...
}
```

`a.catalog.Items` already contains all sources. `SetItems` at `cli/internal/tui/library.go:130–134` passes the full slice to `l.table.SetItems(items)` with no filtering. The filter chips must operate inside `libraryModel` (or its table sub-model) — they must NOT be applied before the `SetItems` call so that switching filters does not require a catalog rescan.

### 3. Metapanel button block

Current button logic at `cli/internal/tui/metapanel.go:229–240`:

```go
if data.canInstall {
    btns = append(btns, zone.Mark("meta-install", activeButtonStyle.Render("[i] Install")))
}
if data.installed != "--" {
    btns = append(btns, zone.Mark("meta-uninstall", activeButtonStyle.Render("[x] Uninstall")))
}
if item.Library || item.Registry == "" {
    btns = append(btns, zone.Mark("meta-remove", activeButtonStyle.Render("[d] Remove")))
}
btns = append(btns, zone.Mark("meta-edit", activeButtonStyle.Render("[e] Edit")))
```

`canInstall` is computed at `metapanel.go:55–63`: it is `false` for Registry Clone items that are not `isUnstagedRegistryItem`. The `[e] Edit` button is currently always shown with no condition; the design calls for hiding it when `item.Library == false`.

The new button matrix for Registry Clone items not yet in Library:

| Button | Condition |
|---|---|
| `[a] Add` | `item.Registry != "" && !item.Library` |
| `[i] Add + Install` | `item.Registry != "" && !item.Library` |
| `[e] Edit` | `item.Library == true` (remove unconditional append) |
| `[i] Install` | `data.canInstall` (unchanged) |
| `[x] Uninstall` | `data.installed != "--"` (unchanged) |
| `[d] Remove` | `item.Library \|\| item.Registry == ""` (unchanged) |
| `[u] Update` | `item.Library && item.Registry != "" && updateAvailable` (new; deferred) |

`computeMetaPanelData` at `metapanel.go:40–70` must be extended to populate a `canAdd bool` field for the `[a] Add` and `[i] Add + Install` buttons, parallel to `canInstall`.

### 4. filterBySource in list.go / helpers.go

Current implementation at `cli/cmd/syllago/helpers.go:153–170`:

```go
func filterBySource(item catalog.ContentItem, source string) bool {
    switch source {
    case "library":    return item.Library
    case "shared":     return !item.Library && item.Registry == "" && !item.IsBuiltin()
    case "registry":   return item.Registry != ""
    case "builtin":    return item.IsBuiltin()
    case "all":        return true
    default:           return item.Library   // silent fallback — known bug risk
    }
}
```

The new `--filter` flag requires state-based predicates, not source-based predicates. The mapping:

| Filter value | Predicate |
|---|---|
| `in-library` | `item.Library` |
| `not-in-library` | `item.Registry != "" && !item.Library` |
| `installed` | item is installed to at least one detected provider |
| `not-installed` | item is not installed to any provider |
| `project` | `!item.Library && item.Registry == ""` |

The `installed` and `not-installed` filter states require an install check per item. In the CLI that means loading `installer.Installed` (snapshot from `installed.json`) or calling `installer.CheckStatus` per item per provider. This is acceptable for CLI but must be documented as an O(n×p) scan where n = item count and p = provider count. The TUI already has `a.installed` and `a.verification` in App state; the filter can reuse them.

The existing `--source` flag and `filterBySource` function are retained unchanged. The `--filter` flag is additive (applied after `--source`). Both flags are independently repeatable: `--filter in-library --filter installed` means items matching both predicates.

### 5. add.AddItems entry point

`add.AddItems` at `cli/internal/add/add.go` writes items to `~/.syllago/content/` and returns. `installer.Install` is never called from the add pipeline (`cli/cmd/syllago/add_cmd.go:304`; `cli/internal/add/add.go:206`). The `--install --to <provider>` flag sequence in `add_cmd.go` will chain these two operations in `runAdd`: call `add.AddItems` then, for each successfully added item, call `installer.Install`. No new package is introduced; both calls remain in `add_cmd.go:runAdd`.

The `--install` flag requires `--to` (or `--to-all`). `--install` without `--to` / `--to-all` is a structured error: `ErrInputMissing`, suggestion `"--install requires --to <provider> or --to-all"`.

### 6. Config.Preferences for hint dismissal

`config.Config.Preferences` at `cli/internal/config/config.go:265` is `map[string]string`. No boolean fields exist on Config. The suppression key `hints.registry_add_dismissed` is stored as `Preferences["hints.registry_add_dismissed"] = "true"`. Read via `config.LoadGlobal()` at `config.go:367`; written via `config.SaveGlobal(cfg)` at `config.go:393`. The `syllago config set hints.registry_add_dismissed true` command already covers this path if `config set` is already wired; otherwise the TUI dismiss action calls `config.LoadGlobal()` → sets the Preferences key → `config.SaveGlobal()` inline.

### 7. INSTALL_002 and ITEM_001 suggestion text

Current INSTALL_002 suggestions at `cli/cmd/syllago/install_cmd.go:308,552` and `install_cmd_append.go:88–93`:

```
"Hint: syllago list --type " + hint
```

For the registry-only case (item exists in a Registry Clone but not in Library), the suggestion becomes:

```
"Hint: syllago add <type>/<name> --from <registry-name> && syllago install <name> --to <provider>"
```

This is additive: call sites that already have a `Registry` field on the unresolved item can emit the richer suggestion; call sites without registry context keep the existing hint. No new error code is introduced. No caller branches on the code value so updating suggestion text is safe.

### 8. Telemetry PropertyDefs

Two new properties on the `list` command and one on `add` must be registered in `EventCatalog()` at `cli/internal/telemetry/catalog.go:28`:

- `filter` (string, repeatable values joined by comma) — `list` command; records active `--filter` values.
- `library_tag` (bool) — `list` command; records whether `[library]` column was requested (if the column is surfaced as a flag rather than always-on).
- `install` (bool) — `add` command; records whether `--install` was set.

The drift-detection test `TestGentelemetry_CatalogMatchesEnrichCalls` enforces registration. `make gendocs` must be run after catalog changes to regenerate `telemetry.json`.

---

## Disambiguation

### Disambiguation: filter chip state vs. re-passing a filtered slice to SetItems

Two implementation paths exist for filter chips in the Library tab:

**Option A — pre-filter outside libraryModel.** App computes a filtered `[]catalog.ContentItem` based on the active chip and passes it to `a.library.SetItems(filteredItems)`. This matches the pattern at `app.go:358` where `a.catalog.Items` is passed directly.

**Option B — store full slice inside libraryModel, apply filter in View/renderTable.** `libraryModel` holds `a.catalog.Items` in full and applies the chip predicate at render time. Filter changes do not require a `SetItems` call.

**Decision: Option B.** The `.claude/rules/tui-items-rebuild.md` rule says "Never set items on sub-models directly from message handlers" — the correct pattern is `refreshContent()`. However, filter chip state changes are not catalog state changes; they do not warrant a full `refreshContent()` cycle. Option A would require App to compute filtered slices and call `SetItems` on every chip toggle, coupling filter logic to App. Option B keeps filter state inside `libraryModel` alongside the full item set, consistent with how `libraryBrowse` search filtering already works (the search string is internal to the model, not applied upstream before `SetItems`). The `SetItems` call site at `app.go:358` passes `a.catalog.Items` unchanged; chip filter is a presentation concern owned by `libraryModel`.

### Disambiguation: --filter as a new flag vs. extending --source in list_cmd

Two paths exist for the state-filter CLI feature:

**Option A — extend the existing `--source` flag** with new values (`in-library`, `not-in-library`, `installed`, `not-installed`, `project`). Simple: one flag, no new parse path.

**Option B — add `--filter` as a new repeatable flag** alongside the existing `--source` flag. `--source` retains its current semantic (origin-based), `--filter` is state-based (relationship-based). Both can be combined: `syllago list --source registry --filter not-in-library`.

**Decision: Option B.** `--source` and `--filter` have categorically different semantics. `--source registry` answers "where did this item come from?"; `--filter not-in-library` answers "what is this item's Add/Install relationship state?". Conflating them into one flag makes the default case (`"all"`) ambiguous and breaks the existing silent fallback (`helpers.go:168`) for unknown values. A repeatable `--filter` flag follows the design concept's stated intent and is composable with `--source`. The JSON output `listItem.Source` field is unaffected; a `listItem.LibraryState` string field is added for state annotation.

---

## Design Questions

### A — Filter chip location: inside libraryModel or a new sibling component?

The filter chips for `in-library`, `not-in-library`, `installed`, `not-installed`, `project` can live as:

- **A — inside libraryModel** as a `filterChips []chipState` field rendered at the top of the library table, handled in `libraryModel.Update`.
- **B — as a new sibling filterBarModel** composed into App alongside the library model, with messages flowing through App.
- **C — as part of the topBar sub-tab row** rendered on the right side of the tab bar, using existing `topBar` zone marks.

**Recommended: A.** The chips are scoped entirely to Library tab behavior and affect only `libraryModel`'s render pass. Options B and C spread state across components for a concern that does not leave the Library tab. Option A is self-contained and consistent with how search filtering is handled (search state lives inside `libraryModel`/`tableModel`).

### B — `[i] Add + Install` in metapanel: invoke install wizard or use flag-driven silent path?

When the user presses `[i]` on a not-in-library Registry Clone item in the TUI:

- **A — launch the full install wizard** (installWizardModel) after the add completes, so the user chooses provider and method interactively.
- **B — add silently to Library, then immediately present a provider picker modal** (narrower than the full wizard) to capture just the `--to` target.
- **C — add silently to Library, then launch the full install wizard** pre-seeded with the just-added item.

**Recommended: C.** The install wizard already handles provider selection, method selection, and the MOAT gate. Pre-seeding with the item skips the initial item-selection step. Option A is ambiguous about what "after the add completes" means in async tea.Cmd flows. Option B introduces a new modal that duplicates wizard functionality. Option C reuses the existing installWizardModel without structural changes, consistent with the "no new orchestrator" constraint.

### C — Post-registry-add hint: modal or toast?

After `syllago registry add` (CLI) or the TUI registry add action:

- **A — TUI modal** for the one-time hint, then suppress via `hints.registry_add_dismissed`. CLI emits a paragraph of hint text.
- **B — extended toast** (multi-line, with a `[dismiss forever]` action) for the hint. CLI emits a single hint line.
- **C — no TUI modal; use an extended toast in TUI, CLI paragraph** (same as B for TUI, A for CLI).

**Recommended: A.** The design concept explicitly calls for a TUI modal. A modal is appropriate here because the user needs to understand the add-before-use requirement before interacting with the newly visible items; a toast auto-dismisses before they can read it. The modal has a single `[got it — don't show again]` button that writes `Preferences["hints.registry_add_dismissed"] = "true"`. This matches the existing modal pattern in `cli/internal/tui` (`tui-modals.md`): fixed width 56, manual box-drawing characters, click-away dismissal.

---

## Out of Scope

The following items are excluded from this feature per the design concept, sharpened with research findings:

- **Auto-copying registry content to Library on registry add.** The unified view makes Registry Clone items visible without copying them; auto-copy would bypass the user's explicit Add step and conflict with the taint-propagation model (`--source-registry`, `--source-visibility` hidden flags on `add_cmd`).
- **Replacing the two-step add-then-install model.** `--install` on `add_cmd` is a convenience chaining of two existing operations, not a replacement. The install wizard and `syllago install` remain the primary install paths.
- **New error codes.** Updated suggestion strings on `INSTALL_002` and `ITEM_001` are sufficient; no new error code constants or `docs/errors/*.md` files are required.
- **New orchestrator package.** The `add --install` chain lives in `add_cmd.go:runAdd`. No new package is introduced.
- **Project Content detection changes.** The `!item.Library && item.Registry == ""` discriminator is used as-is. How Project Content is discovered is unchanged.
- **Pagination or virtual scroll for large registries.** Unbounded row counts are a known risk; deferred explicitly.
- **`[u] Update` conflict behavior.** The button is reserved but conflict resolution for local edits to a registry-sourced Library item is deferred.
- **`syllago config set` wiring.** If `config set` is not already wired, the hint dismissal write is done inline in the TUI action handler; adding a `config set` subcommand is out of scope for this feature.
- **`filterBySource` default-case bug fix.** The silent fallback to `item.Library` for unknown `--source` values (`helpers.go:168`) is a pre-existing behavior; this feature does not change `--source` semantics and does not fix the fallback.

DESIGN_COMPLETE
