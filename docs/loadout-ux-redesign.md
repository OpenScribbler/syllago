# Loadout UX Redesign

> **Status:** Phase 1 complete, Phase 2 next
> **Created:** 2026-03-12
> **Scope:** Fix loadout creation, redesign creation wizard, remove README, add split-view preview

## Background

The loadout feature is syllago's most important UX surface — it's how users compose and apply curated bundles of AI coding tool content. The current implementation has critical bugs (loadouts don't appear after creation) and the UX is incomplete (no guided content selection, no preview of what's in a loadout, no way to evaluate content before applying).

This plan addresses three phases of work, each independently shippable.

> **Review status:** Plan reviewed by coding, UX/accessibility, and security experts on 2026-03-12. Full reviews in `docs/review-{coding,ux,security}-expert.md`. All critical and important findings incorporated below.

---

## Phase 1: Fix Creation Bug + New Wizard ✓

### Why

The loadout creation wizard exists but is broken — loadouts don't appear after creation. Even when fixed, the wizard uses a single flat list of all content items, which doesn't scale and doesn't help users think about what they're building. Users need a guided flow that walks through each content type individually.

### Current State

- 4-step modal: provider -> flat item list -> name/desc -> destination
- `doCreateLoadout()` writes `loadout.yaml` to disk and rescans the catalog (`app.go:835-846`)
- The flat item list (`clStepItems`) shows all catalog items regardless of type, with `[type]` labels
- No review step before committing
- Bug: after creation, loadout doesn't appear in the sidebar or loadout cards view

### Task 1.1: Diagnose and fix the creation bug ✓

**Description:** Loadouts created via the wizard don't appear in the TUI after creation. The `doCreateLoadoutMsg` handler (app.go:835) rescans the catalog and refreshes sidebar counts, but the loadout still doesn't show. This could be a catalog scanner issue (not finding the new loadout directory), a path mismatch (writing to a location the scanner doesn't search), or a sidebar/card refresh issue.

**Success criteria:**
- Create a loadout via the wizard -> it appears immediately in the Loadouts card view and sidebar count updates
- Works for both Project and Library destinations
- Toast shows success message with loadout name

**Tests:**
- Unit test: create loadout via `doCreateLoadout()` with a temp catalog, verify `doCreateLoadoutMsg` triggers catalog rescan that includes the new loadout
- Integration test: full wizard flow (provider -> items -> name -> dest -> confirm) -> verify loadout appears in rendered output
- Verify golden files for loadout cards view still pass

### Task 1.2: Add content type selection step ✓

**Description:** Before walking through individual content types, the user should select which types they want in their loadout. This is a new step inserted between provider selection and per-type item selection.

The step shows checkboxes for each content type supported by the selected provider (using `provider.SupportsType()`), **defaulting all to checked**. Only types with available items in the catalog are shown. Microcopy reads: *"Uncheck any content types you want to skip."*

A Select All / Deselect All toggle key (`a`) is available on this step. Since this is inside a modal with its own key scope, `a` does not conflict with the app-wide "add" action.

**Success criteria:**
- After selecting a provider, a checkbox screen shows all supported content types with available items, all checked by default
- `Space` toggles individual checkboxes, `a` toggles all, `Enter` proceeds, `Esc` goes back
- Selecting 0 types and pressing Enter shows validation message ("Select at least one content type")
- Mouse: clicking a checkbox toggles it, clicking a type label toggles it

**Tests:**
- Unit test: given a provider that supports Rules/Skills/Agents but catalog only has Rules and Skills items, verify only Rules and Skills appear as options, both checked
- Unit test: `a` key toggles all checkboxes on/off
- Unit test: selecting 0 types and pressing Enter shows validation message
- Golden file: wizard at content type selection step

### Task 1.3: Replace flat item list with per-type item selection ✓

**Description:** Instead of one flat list of all items, the wizard walks through each selected content type as its own step. Each step shows a checkbox list pre-filtered to items compatible with the selected provider.

A hotkey (`t` for "toggle filter") switches between "compatible only" (default) and "show all" mode where incompatible items appear grayed out and unselectable. The `t` key avoids collision with `a` (which means "add" app-wide and "select all" on the type selection step).

**Entry filtering:** When the provider is selected (step 1), build a `map[ContentType][]loadoutItemEntry` that pre-filters all catalog items by type and provider compatibility. This map is reused across all per-type steps. The "show all" toggle adds items where `!provider.SupportsType(item.Type)` as disabled entries. Items are filtered once, not on every render.

**Per-type state:** Each content type step has its own cursor position, scroll offset, and search query preserved in maps (`map[ContentType]int` for cursors/offsets). Navigating back restores the exact position. No state is cleared on Esc/back — only the `step` and `typeStepIndex` change.

**Step tracking:** Use composite approach — the existing `step` enum plus a `typeStepIndex int` field and `selectedTypes []ContentType` slice. When `step == clStepItems`, `typeStepIndex` indicates which content type is active. Step number display: `fixedStepsBefore + typeStepIndex + 1` of `fixedSteps + len(selectedTypes)`.

**Design:**
- Step title shows the content type: "Select Rules (2 selected)" with running count
- Items show name + source (registry name or "library")
- Incompatible items (when "show all" active) shown with muted style + strikethrough + "(incompatible)" suffix — non-color distinction for accessibility
- Search via `/` key filters the current type's items (pre-filter by type, then search query)
- `Space` toggles selection, `a` toggles all, `t` toggles compatibility filter, `Enter` moves to next type, `Esc` goes back to previous type (or content type selection if on first type)
- Progress indicator: "(Step 3 of 7)" reflecting total wizard steps
- Select All / Deselect All via `a` key (same as type selection step)

**Empty type behavior:** If a selected content type has 0 available items, show the step with "No {type} available for {provider}" message and require Enter to proceed. Do NOT auto-advance — silent skipping is disorienting and makes the step counter jump unpredictably.

**Success criteria:**
- Each content type gets its own dedicated step
- Items are pre-filtered by provider compatibility via `SupportsType()`
- `t` key toggles between compatible-only and show-all modes, with current mode visible in help text
- Incompatible items use strikethrough + "(incompatible)" label (not just color)
- Search filters within the current content type
- Navigating back preserves selections, cursor position, and scroll offset from all previous steps
- Empty types show message, require Enter to proceed

**Tests:**
- Unit test: wizard with Rules step showing 3 compatible items, verify Space toggles selection
- Unit test: `t` key reveals incompatible items as grayed out with strikethrough
- Unit test: `a` key selects/deselects all visible items
- Unit test: search filter reduces visible items, selections persist after clearing search
- Unit test: Esc from first type step returns to content type selection, preserving all selections
- Unit test: Esc from Rules step -> advance back to Rules -> cursor position preserved
- Unit test: empty type step shows message, Enter advances, Esc goes back
- Integration test: full flow through 3 content types, verify all selections appear in final manifest
- Golden file: per-type item selection step at default size

### Task 1.4: Add review step before creation ✓

**Description:** After destination selection and before committing, show a full summary of the loadout grouped by content type. The user can confirm to create or go back to edit.

**Security callout for hooks and MCP:** If the loadout includes hooks or MCP configs (especially from registries), the review step must explicitly display the commands that will be executed. Show a warning section:

```
  !! Security Notice !!
  This loadout includes executable content:
    Hook: PostToolUse -> "./scripts/lint.sh"
    MCP:  my-server   -> "node server.js --port 3000"
  Review these commands before installing.
  Trust this code and verify it before putting it in your environment.
```

These commands are also visible in the preview pane when browsing loadout contents (Phase 3), giving users two places to evaluate the code before applying.

**Design:**
```
Review Loadout                              (6 of 6)

  Name:        my-dev-setup
  Description: Standard development rules and skills
  Provider:    Claude Code
  Destination: Library

  Contents:
    Rules (2):
      concise-output, no-emojis
    Skills (3):
      code-review, testing, docs
    Agents (1):
      research-agent

               [ Back ]  [ Create ]
```

Item names include source in parentheses when the same name exists in multiple sources: `concise-output (acme-registry)`. Types with many items (>3 names that would overflow modal width) truncate with "+ N more".

**Completion vs cancellation:** Add a `confirmed bool` field to the modal. When the user presses Create, set `confirmed = true` and `active = false`. When Esc dismisses, set only `active = false`. The App checks `confirmed` to distinguish completion from cancellation, replacing the current implicit check on `nameInput.Value()`.

**Success criteria:**
- All metadata and selections displayed in a readable grouped format
- Empty content types omitted from the summary
- Hook/MCP commands displayed with security warning when present
- Item names show source qualifier when ambiguous across registries
- Long item lists truncated with "+ N more"
- "Back" returns to destination step, "Create" triggers `doCreateLoadout()`
- Contents match exactly what was selected in per-type steps
- Keyboard: Enter on Create, Esc goes back, Left/Right switches buttons
- Mouse: clickable buttons

**Tests:**
- Unit test: review step renders correct summary for various selection combinations
- Unit test: review with hooks shows security warning with command strings
- Unit test: review with MCP shows server commands
- Unit test: review with 0 items selected shows warning
- Unit test: Back button returns to dest step with state preserved
- Unit test: Esc sets confirmed=false, Create sets confirmed=true
- Golden file: review step with mixed content types
- Golden file: review step with hooks/MCP security warning

### Task 1.5: Update existing tests and golden files ✓

**Description:** The wizard redesign changes step count, step order, and rendered output. All existing `loadout_create_test.go` tests need updating, and golden files that show the wizard need regeneration.

**Success criteria:**
- All tests in `loadout_create_test.go` pass with the new wizard flow
- All golden files that include loadout wizard rendering are updated
- New golden files added for new steps (content type selection, per-type items, review)
- Tests cover keyboard AND mouse interaction for every new step
- Boundary tests: wizard with 50+ items in a single type (scroll), wizard with 0 available items, wizard at 60x20 terminal

**Affected golden files:**
- `fullapp-loadout-cards.golden` and size variants
- Any golden that captures wizard modal rendering
- New: `component-loadout-wizard-*.golden` for each step

---

## Phase 2: Remove README + Files Tab Split View

### Why

README.md is an unnecessary layer of indirection for syllago content. Skills have SKILL.md, agents are markdown files, hooks are config files — the content IS the documentation. Requiring a separate README adds authoring friction and code complexity (parsing, rendering, warnings for missing README). Removing it simplifies the codebase and the authoring experience.

The Overview tab (which renders README) is replaced by merging its purpose into the Files tab with a split-view layout: file tree on left, file preview on right, with the "primary file" pre-selected.

### Task 2.1: Remove README.md infrastructure

**Description:** Remove all README.md handling from the codebase:

- **`catalog/scanner.go`:** Remove `loadReadme()`, `readDescription()` calls for README.md, README.md warnings, and the `ReadmeBody` field population
- **`catalog/types.go`:** Remove `ReadmeBody` field from `ContentItem`
- **`cli/internal/readme/`:** Delete entire package
- **`catalog/scanner.go`:** Update description extraction to use the primary content file instead of README.md (e.g., extract description from SKILL.md frontmatter, or first paragraph of agent .md file)
- **`scanner.go:362`:** Remove `README.md` from `isMetaFile()` skip list (README.md should appear as a regular file if it still exists on disk). **Note:** This means any README.md files in user-authored content directories outside the repo will start appearing in the file tree — a minor UX surprise. Call out in PR description.

**Success criteria:**
- `ReadmeBody` field removed from `ContentItem` struct
- `readme` package deleted
- No code references `README.md` as a special file
- Content descriptions still populated from primary content files
- `make test` passes (after updating dependent tests)

**Tests:**
- Unit test: scanner discovers content items without README.md files
- Unit test: description extracted from SKILL.md frontmatter description field
- Unit test: agent description extracted from first paragraph of .md file
- Verify `catalog/scanner_dir_test.go` updated (currently creates README.md in test fixtures)

### Task 2.2: Delete all README.md files from content/

**Description:** Remove all README.md files from the content directory tree. If any contain useful information not present in the primary content file, migrate that information first.

**Files to delete:**
- `content/apps/example-kitchen-sink-app/README.md`
- `content/hooks/claude-code/example-lint-on-save/README.md`
- `content/loadouts/claude-code/example-kitchen-sink-loadout/README.md`
- `content/rules/claude-code/example-concise-output/README.md`
- `content/rules/cursor/example-concise-output/README.md`
- `content/commands/claude-code/example-review/README.md`
- `content/skills/syllago-import/README.md`
- `content/local/agents/d2-diagram-expert.md/README.md`
- `content/local/agents/research-agent.md/README.md`
- `content/local/agents/pr-verify.md/README.md`
- `content/local/skills/building-agentic-systems/README.md`

**Success criteria:**
- No README.md files exist under `content/`
- Any useful content from deleted READMEs is preserved in primary content files
- `make test` passes
- Catalog scanner finds all content items without README.md

### Task 2.3: Remove Overview tab from detail view

**Description:** The Overview tab currently renders README.md content via glamour. With README removed, this tab has no purpose. Remove it from all content type detail views.

**Changes:**
- **`detail.go` / `detail_render.go`:** Remove Overview tab and its rendering logic
- **Tab indices:** Files becomes tab 0, Install becomes tab 1
- **Tab switching:** Update `Tab` key handling for new tab count
- **Mouse zones:** Update `tab-N` zone IDs for new indices
- **Default tab on enter:** Files tab (index 0)
- **Breadcrumb:** No change needed

**Success criteria:**
- Detail view shows only Files | Install tabs (2 tabs instead of 3)
- Entering detail view lands on Files tab
- Tab switching works correctly between Files and Install
- Mouse clicking tabs works with new zone IDs
- Help text updated to reflect 2-tab layout

**Tests:**
- Update all detail view tests that reference Overview tab or tab indices
- Golden file updates: `fullapp-detail-overview.golden` -> removed, `fullapp-detail-files.golden` -> now tab 0
- Size variant goldens updated
- Verify tab click zones work correctly

**Affected golden files:**
- `fullapp-detail-overview.golden` (and all size variants) — delete
- `fullapp-detail-files.golden` — update (now first tab)
- `fullapp-detail-install.golden` — update (now second tab)
- `fullapp-detail-overflow.golden` — update
- `fullapp-detail-longname.golden` — update
- `component-detail-tabs.golden` — update

### Task 2.4: Add split-view to Files tab

**Description:** Transform the Files tab from a standalone file browser into a split-view layout: file tree on left, file content preview on right.

**Design:**
```
+---------------------------+---------------------------+
| File Tree                 | Preview                   |
|                           |                           |
| > SKILL.md                | # My Skill                |
|   helpers/                |                           |
|     utils.ts              | Description: A skill that |
|   config.yaml             | does something useful...  |
|                           |                           |
|                           | ## Usage                  |
|                           | ...                       |
+---------------------------+---------------------------+
```

**Split-view component:** Build as a reusable `splitViewModel` that owns its own `Update` and `View`. The parent (detailModel) provides items and handles item-selection semantics via messages. This component is reused in Phase 3 for the loadout Contents tab.

```go
type splitViewModel struct {
    // Left pane
    items        []splitViewItem
    cursor       int
    scrollOffset int
    // Right pane (preview)
    previewContent string
    previewScroll  int
    // Layout
    focusedPane int     // 0=left, 1=right
    width       int
    height      int
    leftRatio   float64 // 0.35 default
    collapsed   bool    // true at narrow widths
    // Single-pane mode
    showingPreview bool // true when Enter opens full-width preview
}
```

**Layout:**
- Left pane: 35% width (min 25 chars), contains file tree with cursor navigation
- Right pane: 65% width (remaining), renders selected file content
- Separator: single `|` character between panes
- Split-view activation threshold: based on **content width** (not terminal width) >= 70 chars. This is consistent with the existing responsive breakpoint system which uses `contentW` throughout.
- Adaptive ratio: 40/60 at content widths 70-90, 35/65 at 100+. Gives the file tree breathing room at narrower widths.

**Single-pane fallback (content width < 70):**
When there isn't enough room for a split view, collapse to single pane with replace-in-place interaction:
- **Mode 1 (file tree):** Full-width file tree with cursor navigation. Enter opens the selected file.
- **Mode 2 (file content):** Full-width file content replaces the tree. Esc returns to the tree with cursor position preserved. Up/Down scrolls content.
- This mirrors how file managers work in constrained terminals (ranger single-column mode).

**Primary file selection by type:**
| Content Type | Primary File |
|-------------|-------------|
| Skills | `SKILL.md` |
| Agents | The `.md` file (agent definition) |
| Rules | The rule `.md` file |
| Hooks | Hook config file (`.json` or `.yaml`) |
| MCP | Config file (`.json`) |
| Commands | Command file |
| Loadouts | `loadout.yaml` |

**Behavior:**
- On entering Files tab, cursor is on the primary file and preview shows its content
- Arrow keys navigate the file tree, preview updates to show highlighted file
- File content rendered as plain text (markdown files get glamour rendering with width-constrained style)
- Preview pane scrolls independently (when focused via `l`/Right)
- Binary files show "(binary file)" placeholder
- Glamour render cache keyed by file path — revisiting a file doesn't re-render
- Large files: preview shows first 200 lines with "(N more lines)" indicator

**File tree details:**
- Directories always expanded (no collapse/expand toggle — keeps it simple)
- Indentation: 2 spaces per nesting level
- Built from `catalog.ContentItem.Files []string` (relative paths)
- Items with 50+ files: tree scrolls with standard scroll indicators

**Success criteria:**
- Files tab shows split view when content width >= 70
- Single-pane replace-in-place fallback when content width < 70
- Primary file pre-selected and previewed on entry
- Navigating file tree updates preview in real-time
- Preview scrolls independently when focused
- Works for all content types

**Tests:**
- Unit test: split view renders correctly at 120x40
- Unit test: single-pane fallback at 60x20 (file tree mode and file content mode)
- Unit test: Enter opens file in single-pane mode, Esc returns to tree with cursor preserved
- Unit test: primary file selection for each content type
- Unit test: cursor navigation updates preview content
- Unit test: preview scroll (independent from tree scroll)
- Unit test: glamour cache hit on revisiting a file
- Golden files: `fullapp-detail-files.golden` at all size variants
- Boundary test: content item with 50+ files (file tree scroll)
- Boundary test: large file in preview (200-line cap)

### Task 2.5: Pane navigation for split view

**Description:** Add pane switching within the detail view's split layout, keeping Tab behavior consistent with the rest of the app.

**Navigation model:**
- `Tab` / `Shift+Tab`: toggles sidebar and content area (consistent with all other pages)
- `l` or `Right`: from file tree, move focus to preview pane
- `h` or `Left`: from preview pane, move focus back to file tree
- `1` / `2`: switch between tabs (Files | Install) from anywhere in content area
- Arrow keys navigate within the focused pane:
  - File tree: Up/Down moves cursor
  - Preview: Up/Down scrolls content

**Focus state ownership:** Zone focus within the detail view (`detailZone` enum: file tree vs preview) lives on `detailModel`, not on `App`. App decides sidebar vs content vs modal. `detailModel` decides which pane within content is focused. This keeps the focus hierarchy clean.

```go
type detailZone int
const (
    zoneFileTree detailZone = iota
    zonePreview
)
type detailModel struct {
    // ...
    zoneFocus detailZone // which pane is focused within content
}
```

**Focus indicator:** Pane title rendered in accent color when focused, muted color when unfocused. No full border around focused pane (consumes too much width). Example:

```
 Files (focused)              Preview
 ─────────────                ───────
 > SKILL.md                   # My Skill
   helpers/                   ...
```

**Behavior:**
- On entering detail view, focus lands on file tree
- Switching tabs resets focus to file tree
- Tab to sidebar, then Tab back lands on whichever pane was last focused
- Help bar shows: `l/right preview • h/left tree • 1/2 tabs • tab sidebar`

**Success criteria:**
- Tab toggles sidebar/content (same as all other pages)
- `l`/Right moves focus from tree to preview
- `h`/Left moves focus from preview to tree
- `1`/`2` switch tabs from anywhere in content
- Focus indicator visible (accent vs muted pane title)
- Help bar shows pane navigation keys
- Default focus on file tree when entering detail view

**Tests:**
- Unit test: Tab toggles sidebar/content (not pane cycling)
- Unit test: `l` from tree focuses preview, `h` from preview focuses tree
- Unit test: `1`/`2` switch tabs
- Unit test: arrow keys work within focused pane only
- Unit test: entering detail view focuses file tree
- Unit test: switching tabs resets focus to file tree
- Golden file: detail view with file tree focused vs preview focused

---

## Phase 3: Loadout Contents Split View

### Why

The loadout detail view's Contents tab currently shows a static list of items grouped by type. Users can't preview what's in a loadout without navigating away. Adding a split view (contents list + preview of selected item's primary file) lets users evaluate loadout contents in-place.

### Task 3.1: Split view for Contents tab

**Description:** Transform the loadout Contents tab into a split view matching the Files tab pattern from Phase 2.

**Design:**
```
+---------------------------+---------------------------+
| Contents                  | Preview                   |
|                           |                           |
| Rules (2)                 | # Concise Output          |
|   > concise-output        |                           |
|     no-emojis             | Keep responses short and  |
| Skills (3)                | focused. No fluff.        |
|   code-review             |                           |
|   testing                 | ## Guidelines             |
|   docs                    | ...                       |
| Agents (1)                |                           |
|   research-agent          |                           |
+---------------------------+---------------------------+
```

**Reuses `splitViewModel` from Phase 2.** The left pane provides grouped items with type headers as disabled entries. The right pane uses the same preview rendering (glamour for markdown, plain for others, cached).

**Layout:**
- Left pane: grouped item list with type headers and item names, showing source in parentheses (registry name or "library")
- Right pane: preview of highlighted item's primary file (same primary file logic as Phase 2)
- Type headers (e.g., "Rules (2)") are non-selectable — `splitViewModel` skips disabled items on cursor movement (same pattern as incompatible items in the wizard)
- On entering Contents tab, first item is selected and previewed
- Collapses to single pane when content width < 70 (same threshold as Phase 2)

**Item resolution:**
- Contents tab resolves manifest references against the catalog to find actual `ContentItem` objects
- If an item can't be resolved (missing from catalog), show it with a warning indicator and "(not found)" in preview

**Success criteria:**
- Contents tab shows split view with grouped items and preview
- Cursor navigation skips type headers
- Preview updates on cursor movement
- Unresolved items shown with warning
- Collapse to single pane at small widths
- Read-only — no actions from preview (drill-in is roadmap)

**Tests:**
- Unit test: contents list renders items grouped by type with counts
- Unit test: cursor skips type headers
- Unit test: preview shows primary file content of selected item
- Unit test: unresolved item shows "(not found)" in preview
- Unit test: single-pane collapse at 60x20
- Golden files: `fullapp-loadout-detail-contents.golden` at all size variants
- Integration test: navigate to kitchen-sink loadout, verify all contents render with previews

### Task 3.2: Pane navigation for loadout detail

**Description:** Apply the same pane navigation model from Phase 2 (Task 2.5) to the loadout detail view. Same keys, same behavior, same `detailZone` focus state.

**Contents tab navigation:**
- `Tab`: sidebar/content toggle
- `l`/Right: contents list -> preview pane
- `h`/Left: preview pane -> contents list
- `1`/`2`: switch tabs (Contents | Apply)
- Default focus: contents list

**Apply tab navigation:**
- `Tab`: sidebar/content toggle
- No split view on Apply tab — single pane with mode options
- `1`/`2`: switch tabs
- Default focus: apply options (Preview/Try/Keep)

**Success criteria:**
- Pane navigation works identically to Phase 2 detail view
- Focus defaults to content area on tab entry
- Visual focus indicator consistent with Phase 2 implementation

**Tests:**
- Unit test: `l`/`h` switch panes on Contents tab
- Unit test: Apply tab has no preview pane (directional keys don't switch)
- Unit test: switching between Contents and Apply resets focus appropriately

---

## Cross-Cutting Concerns

### Accessibility

All phases must maintain syllago's accessibility standards:
- `NO_COLOR=1` support via Charm stack (automatic)
- Status indicators use text+symbol alongside color, never color-only
- Focused pane indicator uses accent vs muted title styling (no full border — saves width)
- Grayed-out incompatible items in wizard use muted color + strikethrough + "(incompatible)" suffix — three non-overlapping signals
- Keyboard navigation is complete — every interaction reachable without mouse
- Split-view `|` separator uses plain ASCII pipe, works in NO_COLOR and limited Unicode terminals
- Screen reader note (future): when cursor moves in file tree, the selected filename should be on a consistent buffer line so line-by-line readers can track it

### Mouse Parity

Every interactive element added must support both keyboard AND mouse (per `tui-mouse.md`):
- Wizard checkboxes: clickable to toggle
- Wizard type selection: clickable
- Content type toggles: clickable
- Split view panes: clickable to focus
- File tree items: clickable to select
- Review buttons: clickable
- All zone.Mark() IDs follow existing conventions

### Keyboard Standards

All new key bindings defined in `keys.go` as `key.Binding` objects (per `tui-keyboard-bindings.md`):
- No hardcoded key strings in Update methods
- Help text updated for every new screen/step
- Help overlay (`?`) updated with new wizard steps and split view navigation

### Responsive Layout

All new views tested at standard breakpoints (per `tui-responsive.md`):
- 60x20 (minimum) — single-pane fallback, wizard fits
- 80x24 (common real-world default, VT100 standard) — test explicitly
- 80x30 (default) — split view active
- 120x40 (medium) — full layout
- 160x50 (large) — generous spacing

**Wizard modal height:** The current `createLoadoutModalHeight = 24` exceeds minimum terminal height of 20 (max modal = 18 per `tui-modal-patterns.md`). The wizard must cap its height to `min(24, terminalHeight - 2)`. Per-type item lists reduce visible count at small heights (existing `innerH` pattern). The review step scrolls if it exceeds available height, with the button row always pinned to bottom.

### Golden File Strategy

**New golden files needed:**
- `component-loadout-wizard-types.golden` — content type selection step
- `component-loadout-wizard-items.golden` — per-type item selection step
- `component-loadout-wizard-review.golden` — review step
- `fullapp-detail-files-split.golden` — split view (+ size variants)
- `fullapp-loadout-detail-contents.golden` — loadout contents split view (+ size variants)

**Updated golden files:**
- All `fullapp-detail-*.golden` files (tab structure changed)
- All `fullapp-loadout-*.golden` files (wizard and detail changes)
- `component-detail-tabs.golden` (2 tabs instead of 3)

**Deleted golden files:**
- `fullapp-detail-overview.golden` (and all size variants)

### Input Validation and Security

**Name validation (all user-entered names):** Add a shared `isValidName()` function used for loadout names, import names, content names — anywhere a user-provided string becomes a directory name. Restriction: `[a-zA-Z0-9_-]`, no dots, no path separators, no leading dash, max 100 chars. Validate in both the wizard UI (immediate feedback) and in `doCreateLoadout()` / manifest `Parse()` (defense-in-depth).

**JSON merge safety:** Add `json.Valid()` check in `readJSONFileOrEmpty()` before sjson operates on settings files. If the file contains invalid JSON, return a clear error: "settings.json contains invalid JSON; fix or delete the file before applying a loadout." Prevents silent corruption.

**Home directory error:** Check the error from `os.UserHomeDir()` in `doCreateLoadout()` (currently ignored with `home, _ := ...`).

**Smoke test security:**
- Do not log full CLI output in CI — only log pass/fail for assertions
- Pin test content to known-good commits, don't run against PR branch content
- Implement `assert_contains` with `grep -qF` (fixed-string match), not eval or unquoted expansion
- Prefer deterministic CLI introspection commands (`claude mcp list`, config queries) over prompt-based verification where possible, since LLM responses are non-deterministic and inherently flaky for assertions

### End-to-End Testing Strategy

Two layers: automated file-level tests in CI for every PR, plus a manual verification checklist against real provider CLIs before releases.

#### Layer 1: Automated File-Level Tests (CI)

**Test content:** Use the existing kitchen-sink content in `content/` as the test fixture. It already has examples of rules, hooks, skills, agents, commands, MCP configs, and a loadout — enough to exercise the full wizard flow.

These tests use `loadout.Apply()` with mocked home directories (temp dirs). They verify that file operations are correct for each provider's expected paths without needing actual CLIs installed.

**Test matrix:**

| Scenario | Provider | Content Types | Destination | Verification |
|----------|----------|--------------|-------------|--------------|
| Full wizard | Claude Code | All 6 types | Library | Loadout YAML created, all refs present |
| Partial wizard | Gemini CLI | Rules + MCP | Project | Only supported types in manifest |
| Provider filter | Gemini CLI | Rules only | Library | Skills/Agents excluded (not supported) |
| Apply preview | Claude Code | Mixed | N/A | Preview returns correct planned actions |
| Apply try | Claude Code | Rules + Hooks | N/A | Symlinks created, JSON merged, snapshot exists |
| Apply keep | Claude Code | Full | N/A | Permanent install, no SessionEnd hook |
| Round-trip | Claude Code | Full | Library | Create -> appear in catalog -> apply -> verify |

**Per-provider path verification tests:**

For each provider, a dedicated test that applies a loadout to a temp home dir and verifies every file lands exactly where the provider expects it:

| Provider | Rules Path | Hooks Path | Skills Path | Agents Path | MCP Path | Commands Path |
|----------|-----------|-----------|------------|------------|---------|--------------|
| Claude Code | `~/.claude/rules/` | `~/.claude/settings.json` (merge) | `~/.claude/skills/` | `~/.claude/agents/` | `~/.claude.json` (merge) | `~/.claude/commands/` |
| Gemini CLI | `~/.gemini/rules/` | N/A | N/A | N/A | `~/.gemini/settings.json` (merge) | N/A |

These tests assert:
- Symlinks point to correct source files and are valid
- JSON merges insert correct keys without corrupting existing content
- No files created for unsupported content types
- Snapshot created with correct manifest for rollback
- "Try" mode injects SessionEnd hook; "keep" mode does not

#### Layer 2: Real-CLI Smoke Tests (CI — Release Gate)

A separate CI workflow that installs real provider CLIs, applies loadouts, and uses each CLI's programmatic mode to verify content is actually picked up. This catches issues file-level tests can't: provider path changes, config parsing quirks, permission issues.

**Runs:** On release branches and manual trigger only (not every PR). Gates releases.

**Approach:** Each provider CLI has a non-interactive/programmatic mode that lets us send a prompt and capture output without a full TUI session:

| Provider | Programmatic Flag | Example |
|----------|------------------|---------|
| Claude Code | `claude -p "prompt"` | `claude -p "List your active rules"` |
| Gemini CLI | `gemini -p "prompt"` | `gemini -p "What rules are you following?"` |

**Test flow per provider:**

```
1. Install provider CLI (via npm/pip/binary in CI)
2. Set up isolated home dir ($HOME override or XDG dirs)
3. syllago loadout apply (kitchen-sink or provider-specific test loadout)
4. Use CLI programmatic mode to verify content is loaded
5. syllago loadout remove
6. Use CLI programmatic mode to verify content is gone (clean removal)
```

**Claude Code smoke tests:**

```bash
# Apply loadout
syllago loadout apply kitchen-sink --mode keep --home "$TEST_HOME"

# Verify rules are active
OUTPUT=$(claude -p "What rules or instructions are you following? List them." 2>&1)
assert_contains "$OUTPUT" "concise-output"  # rule name from loadout

# Verify MCP servers are configured
claude mcp list | assert_contains "example-mcp-server"

# Verify commands are available
claude -p "/example-review --help" 2>&1 | assert_not_contains "unknown command"

# Verify skills are discoverable
claude -p "What skills do you have available?" 2>&1 | assert_contains "syllago-import"

# Clean removal
syllago loadout remove kitchen-sink --home "$TEST_HOME"
OUTPUT=$(claude -p "What rules are you following?" 2>&1)
assert_not_contains "$OUTPUT" "concise-output"
```

**Gemini CLI smoke tests:**

```bash
syllago loadout apply gemini-test-loadout --mode keep --home "$TEST_HOME"

OUTPUT=$(gemini -p "What custom instructions or rules are you following?" 2>&1)
assert_contains "$OUTPUT" "concise-output"

# Verify MCP config
cat "$TEST_HOME/.gemini/settings.json" | jq '.mcpServers' | assert_not_empty

syllago loadout remove gemini-test-loadout --home "$TEST_HOME"
```

**Edge case tests (automated):**

```bash
# First-time setup: no existing provider config files
rm -rf "$TEST_HOME/.claude"
syllago loadout apply kitchen-sink --mode keep --home "$TEST_HOME"
claude -p "Confirm you're working" 2>&1 | assert_exit_0

# Merge with existing config: pre-populate settings.json, verify no corruption
echo '{"existingKey": true}' > "$TEST_HOME/.claude/settings.json"
syllago loadout apply kitchen-sink --mode keep --home "$TEST_HOME"
cat "$TEST_HOME/.claude/settings.json" | jq '.existingKey' | assert_equals "true"

# Sequential loadouts: apply two, verify both active, remove one
syllago loadout apply loadout-a --mode keep --home "$TEST_HOME"
syllago loadout apply loadout-b --mode keep --home "$TEST_HOME"
claude -p "List rules" 2>&1 | assert_contains "rule-from-a"
claude -p "List rules" 2>&1 | assert_contains "rule-from-b"
syllago loadout remove loadout-a --home "$TEST_HOME"
claude -p "List rules" 2>&1 | assert_not_contains "rule-from-a"
claude -p "List rules" 2>&1 | assert_contains "rule-from-b"
```

**CI workflow design:**

```yaml
# .github/workflows/smoke-test-providers.yml
name: Provider Smoke Tests
on:
  workflow_dispatch:      # manual trigger
  push:
    branches: [release/*] # auto on release branches

jobs:
  smoke-claude:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npm install -g @anthropic-ai/claude-code
      # TODO: authenticate via SSO/service account
      - run: make build
      - run: ./tests/smoke/claude-code.sh

  smoke-gemini:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: pip install google-gemini-cli  # or however it's installed
      # TODO: authenticate via SSO/service account
      - run: make build
      - run: ./tests/smoke/gemini-cli.sh

  # smoke-copilot: Roadmap — Copilot CLI programmatic mode needs research
```

**Authentication:** CLIs authenticate via their normal mechanisms (SSO, OAuth, etc.). No API keys need to be injected — the CI runner just needs to be logged in to each provider. Auth setup is provider-specific and handled in the CI workflow's setup steps.

**Roadmap:**
- Copilot CLI smoke tests — programmatic mode needs research (flags, auth, how rules surface)

**Open questions:**
- CI authentication: determine how to persist SSO/OAuth sessions on CI runners (service accounts, cached tokens, etc.)
- "Try" mode auto-revert: hard to test in CI since it depends on session lifecycle. May need to verify the SessionEnd hook is injected rather than testing actual revert behavior.

---

## Implementation Order

```
Phase 1 (Fix + Wizard)
  1.1 Diagnose/fix creation bug
  1.2 Content type selection step
  1.3 Per-type item selection
  1.4 Review step
  1.5 Update tests and goldens

Phase 2 (Remove README + Split View)
  2.1 Remove README infrastructure
  2.2 Delete README.md files
  2.3 Remove Overview tab
  2.4 Add split-view to Files tab
  2.5 Zone navigation

Phase 3 (Loadout Contents)
  3.1 Split view for Contents tab
  3.2 Zone navigation for loadout detail
```

Each phase is a separate PR. Tasks within a phase are sequential (later tasks depend on earlier ones). Phases are independent — Phase 2 doesn't require Phase 1, and Phase 3 requires Phase 2 (reuses split-view component).
