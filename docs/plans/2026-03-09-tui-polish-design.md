# TUI Polish & Bug Fixes - Design Document

**Goal:** Fix bugs, remove dead features, and redesign key UI patterns for consistency and scalability.

**Decision Date:** 2026-03-09

---

## Problem Statement

The TUI has accumulated several issues as the CLI evolved:
1. Sidebar click areas inconsistent between sections
2. Add wizard lacks breadcrumb navigation
3. Library screen is a flat list that won't scale
4. Loadouts and Library are grouped under "AI Tools" but aren't AI tools
5. Library badge is redundant when viewing the Library screen
6. Provider discovery scans the wrong root and misses global content
7. Settings provider toggle is a dead feature (nothing reads it)
8. Prompts and Apps content types add complexity with no provider mapping

## Proposed Solution

Eight changes organized into four categories: bug fixes, content type cleanup, visual polish, and UI redesigns.

---

## 1. Bug Fixes

### 1a. Sidebar Click Area (#1)

**Problem:** Configuration section items only register clicks on the text itself, not the full row width. AI Tools/Content items pad to full width via count formatting, but Configuration items don't.

**Fix:** In `sidebar.go`, pad Configuration item labels with `fmt.Sprintf("%-*s", inner-2, u.label)` to match Content items. Ensures zone marks cover the full row width.

**Files:** `sidebar.go`, `sidebar_test.go` (if exists), golden files

### 1b. Discovery Path & Scope (#6)

**Problem:** TUI passes `a.projectRoot` (syllago content root) to `DiscoverFromProvider`, but should pass the current working directory where `.claude/`, `.gemini/` etc. live. Additionally, neither CLI nor TUI discovers global/user-level content at `$HOME`.

**Fix:**
1. Resolve `os.Getwd()` at import model creation and pass as project root for discovery
2. Call `DiscoverFromProvider` twice — once with cwd (project scope) and once with `os.UserHomeDir()` (global scope)
3. Add `Scope string` field to `add.DiscoveryItem` ("project" or "global") for display
4. Merge results, dedup by `type/name` (project scope wins over global for same name)
5. Show scope tag in the discovery select checklist: `[project]` or `[global]`
6. *(Deferred)* Surface undetected providers in the provider pick list as dimmed with "not detected" label. Selecting one prompts for a custom path via text input. **Deferred to follow-up PR** — depends on Custom Provider Locations feature (`PathResolver` infrastructure). Implementing path prompt UI without the resolver would be throwaway code.

**Files:** `add/add.go` (DiscoveryItem, DiscoverFromProvider), `import.go` (model creation, view, update), `app.go` (pass cwd), `import_test.go`

### 1c. Settings Cleanup (#7)

**Problem:** Provider toggle in Settings is dead — nothing reads the saved provider list. Users toggle providers and expect it to persist/matter, but it doesn't.

**Fix:**
1. Remove provider row (row 1) from settings
2. Reduce `settingsRowCount()` from 3 to 2
3. Registry auto-sync shifts from row 2 to row 1
4. Auto-save on every toggle — call `m.save()` in `activateRow()` after each change
5. Remove `dirty` field, `s` key binding for manual save
6. Remove sub-picker infrastructure: `editMode`, `subItems`, `subCur`, `settingsEditMode`, `settingsPickerItem`, `applySubPicker()`, `CancelAction()`, `HasPendingAction()`
7. Update `settingsDescriptions` array to remove provider entry

**Files:** `settings.go`, `settings_test.go`, `app.go` (remove HasPendingAction/CancelAction calls), golden files

---

## 2. Content Type Removal

### 2a. Remove Prompts and Apps (#8)

**Problem:** Neither Prompts nor Apps maps to any provider. They add complexity (Prompts has special copy/save buttons, body field handling) without clear value.

**Fix:**
1. Remove `Prompts` and `Apps` from `AllContentTypes()` in `catalog/types.go`
2. Remove `IsUniversal()` cases for Prompts and Apps
3. Remove `Label()` cases for Prompts and Apps
4. Remove Prompts special-casing in detail view (copy/save buttons, body display)
5. Delete example content directories: `content/prompts/`, `content/apps/`
6. Remove `Body` field from `ContentItem` if only used by Prompts
7. Update all tests that reference `catalog.Prompts` or `catalog.Apps`
8. Update converter/provider code that references these types
9. Clean up any `SupportsType` functions that mention Prompts/Apps

**Files:** `catalog/types.go`, `detail.go`, `detail_render.go`, `detail_test.go`, `detail_render_test.go`, `testhelpers_test.go`, `app.go`, `parse/classify.go`, `loadout/manifest.go`, `add/add.go`, converter files, provider files, golden files

---

## 3. Visual Polish

### 3a. Redundant Library Badge (#5)

**Problem:** When viewing the Library screen, every item shows `[Library]` badge. This is redundant — you're already in the Library view.

**Fix:** Pass `hideLibraryBadge bool` when populating the items model for Library view. The items renderer checks this flag and skips the badge rendering.

**Files:** `items.go`, `app.go` (where Library items are loaded), golden files

---

## 4. UI Redesigns

### 4a. Sidebar Reorganization (#4)

**Problem:** Loadouts and Library are grouped under "AI Tools" but aren't AI tools. The section label doesn't fit.

**Fix:** Restructure sidebar into three sections:
```
  Content
    Skills          3
    Agents          2
    MCP             1
    Rules           5
    Hooks           4
    Commands        1
    Loadouts        2
  ─────────────
  Collections
    Library        12
    Loadouts        2
  ─────────────
  Configuration
    Add
    Update
    Settings
    Registries      1
    Sandbox
```

Changes:
1. Rename "AI Tools" → "Content"
2. Add "Collections" section between Content and Configuration
3. Move Library from end of Content to Collections
4. Add Loadouts to Collections (remove from Content type list)
5. Update `totalItems()`, cursor index calculations, selector methods
6. Update all sidebar index arithmetic in `app.go`

**Files:** `sidebar.go`, `app.go` (all cursor-to-screen routing), `sidebar_test.go`, integration tests, golden files

### 4b. Library Card View (#3)

**Problem:** Library is a flat list that gets unwieldy with many items. Needs the same card layout as the homepage.

**Fix:**
1. When Library is selected in sidebar, show card view (like homepage) with one card per content type
2. Each card shows type name and count of library items of that type
3. Only show cards for types that have library items (skip empty types)
4. Clicking a card navigates to items screen filtered to that type + library-only
5. Reuse existing card rendering from `renderContentWelcome()`

**Files:** `app.go` (Library screen rendering, navigation), `items.go` (filter by type+library), golden files

### 4c. Loadouts Card View (part of #4)

**Problem:** Loadouts need their own screen under Collections, grouped by provider.

**Fix:**
1. When Loadouts is selected in sidebar, show card view with one card per provider that has loadouts
2. Each card shows provider name and loadout count
3. Clicking a card navigates to items screen filtered to loadouts for that provider
4. Group catalog loadout items by provider for the card data

**Files:** `app.go` (Loadouts screen, navigation), golden files

### 4d. Add Wizard Breadcrumbs (#2)

**Problem:** Add wizard has inconsistent step indicators and no clickable navigation between steps.

**Fix:**
1. Replace the "Step N of M: Label" indicator with a clickable breadcrumb trail
2. Format: `Home > Add > Source > Select Provider` (zone-marked for clicks)
3. Clicking a breadcrumb segment navigates back to that step
4. Apply to ALL Add flows:
   - From Provider: `Home > Add > Source > Provider > Select Items`
   - Local Path: `Home > Add > Source > Type > Browse > Confirm`
   - Git URL: `Home > Add > Source > URL > Pick Item > Confirm`
   - Create New: `Home > Add > Source > Type > Name > Confirm`
5. Remove `stepLabel()` method (replaced by breadcrumb)
6. Each step's crumb is a zone mark like `"add-crumb-source"`, `"add-crumb-type"`, etc.

**Files:** `import.go` (View, stepLabel → breadcrumb, mouse handling for crumb clicks), `import_test.go`, golden files

---

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Sidebar sections | Three: Content, Collections, Configuration | Clear separation of concerns |
| Section label | "Content" not "AI Tools" | More accurate, less jargon |
| Library view | Cards per content type | Mirrors homepage, scales well |
| Loadouts view | Cards per provider | Natural grouping, matches user mental model |
| Add breadcrumbs | Clickable trail across all flows | Consistent navigation, discoverable |
| Settings provider | Remove entirely | Dead feature, nothing reads it |
| Settings save | Auto-save on toggle | Simplest UX, no "forgot to save" |
| Prompts/Apps | Remove entirely | No provider mapping, adds complexity |
| Discovery scope | Both project (cwd) + global ($HOME) | Captures all user content |
| Undetected providers | Deferred to follow-up PR | Depends on Custom Provider Locations / PathResolver |

## Testing Strategy

- **Every visual change:** Regenerate golden files with `go test ./cli/internal/tui/ -update-golden`, review diffs
- **Sidebar changes:** Update integration tests that navigate by cursor index (`TestTeatestCategoryToItems`, `TestTeatestSettingsToggle`, `TestTeatestImportStart`, etc.)
- **Content type removal:** Update all tests referencing `catalog.Prompts` or `catalog.Apps`
- **Settings:** Update `settings_test.go` for removed rows and auto-save behavior
- **Discovery:** Add tests for dual-scope discovery, scope tags, undetected provider handling
- **Breadcrumbs:** Update `import_test.go` step navigation tests, add breadcrumb click tests

## Success Criteria

1. All sidebar items are clickable across the full row width
2. Provider discovery finds content at both project and global level
3. Settings toggles persist without manual save
4. Library and Loadouts have card-based browse screens
5. Sidebar has three clear sections with accurate labels
6. Add wizard has consistent clickable breadcrumbs across all flows
7. Prompts and Apps are fully removed with no dead code
8. All tests pass, golden files updated

## Open Questions

None — all decisions made during brainstorm.

---

## Next Steps

Ready for implementation planning with `Plan` skill.
