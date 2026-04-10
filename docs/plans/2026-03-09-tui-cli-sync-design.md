# TUI-CLI Sync - Design Document

**Goal:** Synchronize the TUI with CLI changes: rename "Import" to "Add" everywhere, and integrate provider discovery into the TUI add wizard.

**Decision Date:** 2026-03-09

---

## Problem Statement

The CLI went through two rounds of redesign:
1. **7-verb vocabulary** (commit 3ec5a9c): `import` -> `add`, `export` -> `convert`/`install`, `promote` -> `share`
2. **Add command redesign** (commit 3d82fec): provider discovery mode, positional args, `internal/add` package

The TUI was partially updated (metadata fields, sidebar labels, install modal) but still has stale "Import" terminology throughout and no provider discovery integration. The `internal/add` package was explicitly designed for TUI reuse ("Both the CLI command and the TUI use these functions directly") but the TUI still uses its own import logic.

## Proposed Solution

Two-phase update in a single PR:

**Phase 1 (Terminology):** Mechanical rename of all "Import" references to "Add" across TUI Go files and tests. No behavioral changes.

**Phase 2 (Provider Discovery):** Add a "From Provider" source option to the TUI add wizard. Wire it to `add.DiscoverFromProvider()`. Multi-select discovered items with status badges, action buttons, and keyboard shortcuts.

## Architecture

### Phase 1: Terminology Cleanup

String replacements across these locations:

| File | Line(s) | Current | Replacement |
|------|---------|---------|-------------|
| `app.go` | 1523 | `"Import"` card label | `"Add"` |
| `app.go` | 1523 | `"Import your own AI tools from local files or git repos"` | `"Add content from providers, local files, or git repos"` |
| `app.go` | 1469 | `"syllago import --from claude-code"` | `"syllago add --from claude-code"` |
| `app.go` | 1742 | breadcrumb `"Import"` | `"Add"` |
| `import.go` | 698 | `"Import AI Tools"` heading | `"Add Content"` |
| `import.go` | 710 | help text referencing import | Updated to reference "add" |
| `import.go` | 870 | `"Import Selected (%d)"` | `"Add Selected (%d)"` |
| `import.go` | 899 | help footer `"enter import"` | `"enter add"` |
| `import.go` | 901 | help footer `"enter import"` | `"enter add"` |
| `import.go` | 916 | help footer `"enter import"` | `"enter add"` |
| `sidebar.go` | 125 | comment `"Import"` | `"Add"` |
| `settings.go` | 202 | `"syllago imports their existing configs..."` | `"syllago adds their existing content..."` |
| `app_test.go` | 88-89 | asserts `"syllago import"` | `"syllago add"` |

After changes: regenerate all golden files with `go test ./internal/tui/ -update-golden`.

### Phase 2: Provider Discovery Integration

#### New Import Steps

Two new values added to the `importStep` enum:

```go
stepProviderPick    // (provider discovery) pick detected provider
stepDiscoverySelect // (provider discovery) multi-select discovered items
```

#### New Fields on `importModel`

```go
// Provider discovery state
discoveryProvCursor int                 // cursor for stepProviderPick
discoveryItems      []add.DiscoveryItem // results from DiscoverFromProvider
discoverySelected   []bool              // checkbox state per item
discoveryCursor     int                 // cursor for stepDiscoverySelect
discoveryProvider   provider.Provider   // selected provider
```

#### New Message Type

```go
type discoveryDoneMsg struct {
    items []add.DiscoveryItem
    err   error
}
```

#### Source Step Changes

The source selection step (`stepSource`) gains a fourth option at position 0:

```
Add Content
─────────────────────────
Bring in content from an installed
provider, local files, or git.

 > From Provider
   Local Path
   Git URL
   Create New
```

Cursor indices shift: From Provider=0, Local Path=1, Git URL=2, Create New=3.

In `updateSource()`:
```go
case 0: // From Provider
    m.discoveryProvCursor = 0
    m.step = stepProviderPick
```

#### Provider Pick Step (`stepProviderPick`)

Lists all detected providers from `m.providers`. Reuses the exact same cursor pattern as the existing `stepProvider` step:

- `"> "` prefix for selected, `"  "` for unselected
- `selectedItemStyle` for focused, `itemStyle` for others
- `zone.Mark(fmt.Sprintf("import-opt-%d", i), row)` for mouse clicks
- `up/down` navigate, `enter` select, `esc` back to source

On enter, dispatches async discovery:

```go
case key.Matches(msg, keys.Enter):
    prov := m.providers[m.discoveryProvCursor]
    m.discoveryProvider = prov
    return m, func() tea.Msg {
        globalDir := catalog.GlobalContentDir()
        resolver := config.NewPathResolver(nil) // or load from config
        items, err := add.DiscoverFromProvider(prov, m.projectRoot, resolver, globalDir)
        return discoveryDoneMsg{items: items, err: err}
    }
```

#### Discovery Select Step (`stepDiscoverySelect`)

Multi-select checklist with status badges and action bar.

**Rendering:**

```
Add from Claude Code
4 items found (2 new, 1 outdated, 1 in library)

  > [x] rules/security          (new)
    [x] rules/testing           (new)
    [ ] skills/dev-workflow      (in library)
    [x] agents/code-review      (outdated)

  Actions
  ────────────────────
  [Select All]  [Deselect All]  [Add Selected (3)]
```

**Visual patterns (matching existing conventions):**

| Element | Style | Source Pattern |
|---------|-------|---------------|
| Cursor prefix | `"> "` selected, `"  "` unselected | All list steps |
| Focused item | `selectedItemStyle` (bold + accent) | All list steps |
| Checkbox checked | `installedStyle.Render("[x]")` (green) | `stepHookSelect` |
| Checkbox unchecked | `"[ ]"` plain | `stepHookSelect` |
| Status `(new)` | plain text | New |
| Status `(in library)` | `installedStyle` (green) | Matches installed indicators |
| Status `(outdated)` | `warningStyle` (amber) | Matches warning indicators |
| In-library item name | `helpStyle` (muted) | Matches disabled/secondary items |
| Section header | `labelStyle.Render("Actions")` | `detail_render.go:506` |
| Section divider | `helpStyle.Render(strings.Repeat("-", 20))` | `detail_render.go:507` |
| Action buttons | `buttonStyle.Render("Label")` in `zone.Mark()` | `detail_render.go:510-523` |
| Summary line | `helpStyle` | Existing step descriptions |

**Pre-selection logic:**
- `StatusNew` items: pre-selected (`true`)
- `StatusOutdated` items: pre-selected (`true`)
- `StatusInLibrary` items: pre-deselected (`false`)

**Action bar buttons (zone-marked for mouse):**

| Button | Zone ID | Keyboard | Action |
|--------|---------|----------|--------|
| `[Select All]` | `"discovery-btn-all"` | `a` | Set all `discoverySelected` to `true` |
| `[Deselect All]` | `"discovery-btn-none"` | `n` | Set all `discoverySelected` to `false` |
| `[Add Selected (N)]` | `"discovery-btn-add"` | `enter` | Execute add for selected items |

**Per-item zone marking for mouse toggle:**
```go
zone.Mark(fmt.Sprintf("discovery-item-%d", i), row)
```
Clicking an item row toggles its selection (same as space bar).

**Keyboard handling (`updateDiscoverySelect`):**

| Key | Action |
|-----|--------|
| `up/down` | Move item cursor |
| `space` | Toggle current item's selection |
| `a` | Select all items |
| `n` | Deselect all items |
| `enter` | Add selected items (dispatch async cmd) |
| `esc` | Back to provider pick |

**Help footer:**
```
up/down navigate . space toggle . a select all . n deselect all . enter add selected . esc back
```
(Using ` . ` as separator to match existing help text pattern)

**Step labels:**
```go
case stepProviderPick:
    return "Step 2 of 3: Provider"
case stepDiscoverySelect:
    return "Step 3 of 3: Select Items"
```

#### Add Execution

On enter from `stepDiscoverySelect`, dispatch async write using `add.WriteItems()` or equivalent from the `internal/add` package:

```go
return m, func() tea.Msg {
    var added []string
    for i, item := range m.discoveryItems {
        if !m.discoverySelected[i] {
            continue
        }
        // Use add package to write item to global library
        // Returns importDoneMsg on completion
    }
    return importDoneMsg{name: strings.Join(added, ", ")}
}
```

This reuses the existing `importDoneMsg` type so App.Update() handles catalog refresh and success/error messaging identically to the existing import flows.

#### Mouse Handling

In `handleMouseClick()`, add cases for the new steps:

```go
case stepProviderPick:
    maxItems = len(m.providers)
case stepDiscoverySelect:
    // Check action button zones first
    if zone.Get("discovery-btn-all").InBounds(msg) { ... }
    if zone.Get("discovery-btn-none").InBounds(msg) { ... }
    if zone.Get("discovery-btn-add").InBounds(msg) { ... }
    // Then check per-item zones for toggle
    for i := 0; i < len(m.discoveryItems); i++ {
        if zone.Get(fmt.Sprintf("discovery-item-%d", i)).InBounds(msg) {
            m.discoverySelected[i] = !m.discoverySelected[i]
        }
    }
```

#### Hook Select Retrofit

Update the existing `stepHookSelect` rendering and handler to match the new action bar pattern:

**Before (text hint):**
```
  Import Selected (3)  .  space toggle  .  a all  .  n none
```

**After (action buttons + updated labels):**
```
  Actions
  ────────────────────
  [Select All]  [Deselect All]  [Add Selected (3)]
```

Zone IDs: `"hook-btn-all"`, `"hook-btn-none"`, `"hook-btn-add"`. Keyboard shortcuts remain the same (`a`, `n`, `enter`). Help footer updated to match: `"up/down navigate . space toggle . a select all . n deselect all . enter add selected . esc back"`.

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Scope | Single design, two phases | Both touch overlapping files; Phase 1 first avoids merge conflicts |
| Source option placement | "From Provider" as first option | Provider discovery is the primary add workflow |
| Selection model | Multi-select checklist | Matches existing hook select pattern; efficient for bulk operations |
| In-library items | Shown but pre-deselected, muted style | User sees full picture; can force re-add if needed |
| Action bar style | Detail-view buttons (zone-marked) | Matches existing Install/Uninstall/Copy pattern; no button cursor complexity |
| Hook select retrofit | Yes, update to match | Consistency between two identical interaction patterns |
| Add package integration | Use `add.DiscoverFromProvider()` directly | Package was explicitly designed for TUI reuse |

## Data Flow

1. User selects "From Provider" -> `stepProviderPick`
2. User picks a provider -> async `add.DiscoverFromProvider()` -> `discoveryDoneMsg`
3. Items populated with status annotations -> `stepDiscoverySelect`
4. User toggles selections -> "Add Selected" -> async write loop -> `importDoneMsg`
5. App.Update() handles `importDoneMsg`: catalog refresh, success message, return to home

## Error Handling

| Scenario | Handling |
|----------|----------|
| No providers detected | Show message: "No providers detected. Install a supported AI coding tool first." Back to source. |
| Discovery returns 0 items | Show message: "No content found in <provider>." Back to provider pick. |
| Discovery errors | `m.message = err.Error()`, `m.messageIsErr = true`, back to provider pick. |
| Write failure (partial) | Show partial success message listing what was added and what failed. |
| All items already in library | List shows all items as `(in library)`, none pre-selected. User can still select and re-add. |

## Success Criteria

1. All TUI text references "Add" instead of "Import" (except Go package/variable names which stay for now)
2. "From Provider" option appears first in the source step
3. Provider discovery scans real provider directories and shows correct status badges
4. Multi-select works with keyboard (space/a/n/enter/esc) and mouse (zone clicks on items and buttons)
5. Action bar has clickable [Select All], [Deselect All], [Add Selected (N)] buttons
6. Hook select step is retrofitted with same action bar pattern
7. All golden files regenerated and tests passing
8. Help footer text is clear and matches keyboard shortcuts exactly

## Open Questions

None - all decisions resolved during brainstorm.

---

## Next Steps

Ready for implementation planning with `Plan` skill.
