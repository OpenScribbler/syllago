# TUI Wizards Design — Add, Install, and Action Flows

**Date:** 2026-03-26
**Status:** Design complete, ready for implementation planning

## Overview

The TUI can browse and edit content but cannot add, install, remove, or manage it.
This design covers every state-modifying action the TUI needs, organized into
full-screen wizards, overlay modals, and simple actions.

---

## Design Principles

1. **Wizards are full-screen** — same layout/drill-in experience as native TUI.
   Confirmation modals overlay on top of wizard screens.
2. **Step breadcrumbs** — every wizard shows a step indicator at the top.
   Steps are clickable to go back. State is preserved across all steps.
3. **Universal code scanning** — ALL content types are scanned for executable code.
   Warnings surface whenever code is detected, regardless of type.
4. **Review-then-edit** — review steps let you jump back to any previous step.
5. **Progressive disclosure** — start simple, drill in for details.

---

## Go Architecture Rules

These rules apply to all wizard and modal implementations:

### Wizard Models as Pointer Fields

Full-screen wizard models are stored as **pointer fields** on `App`:
```go
type App struct {
    wizardMode wizardKind         // 0=none, 1=add, 2=share
    addWizard  *addWizardModel    // nil when not active
    shareWiz   *shareWizardModel  // nil when not active
}
```
- Avoids expensive value copies of complex wizard state during BubbleTea's
  `Update()` → return cycle
- Nil when inactive (zero cost)
- Overlay modals (confirm, install, registry add, loadout apply) remain value
  types since they are small

### Wizard Mode Early-Return Routing

`App.Update()` checks `wizardMode` early and delegates all input:
```go
if a.wizardMode != wizardNone {
    if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
        return a, tea.Quit
    }
    return a.routeToWizard(msg)
}
```
This suppresses global keys (`R`, `1/2/3`, `Tab`, etc.) cleanly without
scattering wizard-active checks across the switch.

### All Backend Calls Are tea.Cmds

**Rule:** No blocking I/O in `Update()`. Every backend operation (install,
uninstall, remove, add, clone, share) runs in a `tea.Cmd` goroutine:
```go
case doInstallMsg:
    return a, func() tea.Msg {
        err := installer.Install(msg.provider, msg.location, msg.method, msg.item)
        return installDoneMsg{err: err, item: msg.item}
    }
```
This includes operations that seem "fast" (uninstall, remove). The render
loop must never block.

### Typed Struct Fields for Step Data

Each wizard uses flat, typed struct fields for step results — not
`map[step]interface{}` or `[]any`:
```go
type addWizardModel struct {
    shell        wizardShell
    seq          int
    source       addSource
    contentTypes []catalog.ContentType
    discovered   []catalog.ContentItem
    selected     []int           // indices into discovered
    riskResults  []catalog.RiskIndicator
    executing    bool
    results      []addResult
}
```

### Context-Based Cancellation for Long Operations

Git clone and registry sync use `context.WithTimeout`:
```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
```
The `cancel` func is stored on the wizard model. Wizard dismissal calls
`cancel()` to abort in-flight operations.

### Sequential Command Chain for Multi-Item Progress

The Execute step uses sequential commands (not `tea.Batch`) for deterministic
progress display:
```go
case itemAddedMsg:
    w.markDone(msg.index)
    if next := w.nextPending(); next >= 0 {
        return w, w.addItemCmd(next)
    }
    return w, func() tea.Msg { return allItemsAddedMsg{} }
```

---

## Reusable V3 Components

| Component                   | File                               | Reuse For                                  |
|-----------------------------|------------------------------------|--------------------------------------------|
| Library table + sort/search | `table.go`                         | Item selection lists (add, loadout create)  |
| File tree + preview split   | `filetree.go`, `preview.go`        | Reviewing content before add/install        |
| Edit modal (text inputs)    | `modal.go`                         | Name, URL, description entry                |
| Toast notifications         | `toast.go`                         | Success/error feedback for all actions      |
| Overlay compositing         | `overlayModal()`, `overlayToast()` | Confirm modals on wizard screens            |
| Metadata panel              | `metapanel.go`                     | Item details during selection               |
| Gallery cards               | `cards.go`, `gallery.go`           | Loadout/registry browsing                   |
| Drill-in detail             | library detail mode                | Inspecting items before add/install         |

**New components needed:**

| Component                  | Purpose                                                            |
|----------------------------|--------------------------------------------------------------------|
| **Wizard shell**           | Full-screen wrapper: step breadcrumbs (built-in) + content + help  |
| **Confirm modal**          | Generic yes/no overlay with optional danger styling + checkboxes   |
| **Checkbox list**          | Multi-select item list (for add, loadout type/item selection)      |
| **Risk banner**            | Minimal model: list index + items, delegates rendering to view fn  |
| **Provider picker**        | Provider selection with "already installed" detection, reusable    |

**Consolidated (not separate components):**
- ~~Step breadcrumbs~~ → built into wizard shell as `ViewBreadcrumbs()` method
- ~~Input modal~~ → parameterize existing `editModal` (field count configurable)

---

## Wizard Step Breadcrumb Design

Every wizard renders a step bar at the top of the full-screen view:

```
  Source > Discovery > Selection > Review > Confirm
  ~~~~~~
```

- Active step is highlighted (bold + primary color)
- Completed steps are clickable (underlined, pointer cursor)
- Future steps are muted/disabled
- Clicking a completed step navigates back, preserving all state
- Enter on a step in the review page also jumps back to that step for edits
- Going back does NOT reset progress UNLESS you make changes that alter
  subsequent steps — you can jump freely between steps until final confirmation
- Back navigation also possible via `[B]` or Cancel buttons in each step
- Step bar replaces the normal topbar tab rows during wizard mode
- Help bar still available with context-specific hotkey hints for each step
- Step names and count are flexible per wizard (add=5, install=3, share=4, etc.)
- Step bar is optional for simple modals (confirmations, registry add) but
  required for multi-step wizards (add, install, share)
- Exiting via `[Esc]` or Cancel prompts confirmation if changes would be lost,
  otherwise exits immediately
- Global keys (`R` refresh, `1`/`2`/`3` group switch) are suppressed during
  wizard mode to prevent catalog mutation while wizard is active

### State Invalidation Mechanism

Each wizard step has a **version counter** (`stepVersion uint`). When a step's
data changes, its version increments. Downstream steps track the version of the
step they depend on. When navigating forward after going back, the wizard checks
whether the dependency version has changed:

- **Version unchanged:** Skip re-execution, reuse cached results
- **Version changed:** Mark downstream steps as stale, re-execute on entry

Example for Add Wizard:
```
Step 1 (Source)    v=3   ← user changed source from provider to registry
Step 2 (Type)      v=2   depends on Step 1 v=2 → STALE (1.v=3 != dep 2)
Step 3 (Discovery) v=1   depends on Step 2 → STALE (transitively)
Step 4 (Review)    v=1   depends on Step 3 → STALE
```

This is tested via `TestAddWizard_BackNavigation_TypeChangeInvalidatesDiscovery`
and similar. The version counter approach is simple, deterministic, and avoids
expensive equality checks on discovery results.

### Async Safety: Sequence Numbers

All async operations (git clone, discovery scan, share execution) use a
**sequence number** (`seq int`) in their result messages, matching the pattern
already used by `toastModel`. The wizard increments `seq` on:
- Wizard cancel/exit
- Step re-entry after back-navigation

Result messages with stale `seq` values are silently dropped in `Update()`.
This prevents:
- Stale results arriving after wizard dismissal
- Old discovery results overwriting new ones after a re-scan

Double-submit prevention: the Execute step disables the Confirm button after
first press and ignores additional Enter/click events.

---

## Security: Universal Code Scanning

**Every content type** is scanned for executable code patterns before add or install.
The existing `catalog.RiskIndicators()` and `converter.ScanHookSecurity()` provide
the backend. The TUI surfaces warnings in two places.

### Scanning Strategy (Research Required)

**Status:** The current scanning is pattern-based regex matching. Before Phase D
(Add Wizard), we need a dedicated research spike to determine:

- How to detect executable code across ALL file types (not just hooks)
- Pattern detection for multi-file, multi-directory skills
- How to catch code embedded in markdown (fenced code blocks, inline scripts)
- How to identify dangerous operations vs. benign code
- What obfuscation techniques are practical to detect vs. not worth trying
- Whether static analysis tools (shellcheck, semgrep) are worth integrating

The goal is quick wins with high coverage, not perfection. A dedicated research
task should produce an expanded pattern set for `risk.go` covering all content
types before the Add Wizard ships.

### 1. Selection List Indicators

Items with detected code get colored badges inline:

```
  [x] my-rule              rules    ! Code detected
  [x] pre-tool-validate    hooks    !! Runs: bash -c "eslint..."
  [ ] readme-generator     skills   ! Bash access
```

- `!!` RED = HIGH severity (runs shell commands, network access)
- `!` ORANGE = MEDIUM severity (Bash reference, env vars, scripts in files)
- No badge = no code detected (text-only content)

### 2. Review Step Risk Banner

The risk banner is a **full navigable list**, not a static box. It occupies a
significant portion of the review screen so users can clearly see the actual
code being flagged.

- First item auto-highlighted on entry
- Arrow keys navigate between flagged items
- Enter drills into the selected item (file tree + preview, same as Library detail)
- Each item shows: name, severity badge, and a preview of the flagged code/command
- RED border if any HIGH severity items, ORANGE if MEDIUM only

```
+-- Code Execution Warning ----------------------------------------+
|                                                                   |
|  3 items contain executable code:                                 |
|                                                                   |
|> !! pre-tool-validate   HIGH   bash -c "eslint --fix $FILE"      |
|  !! mcp-readability     HIGH   npx @anthropic/readability-server  |
|  !  readme-gen          MED    References Bash tool               |
|                                                                   |
|  [Enter] Inspect item   [Esc] Back to review                     |
+-------------------------------------------------------------------+
```

### Content Type Risk Surface

| Type     | What to Scan                                | Risk Source                    |
|----------|---------------------------------------------|--------------------------------|
| Hooks    | `command` field in JSON, referenced scripts | Direct shell execution         |
| MCP      | `command` + `args` in config, `env` vars    | Process launch + network       |
| Skills   | All files for Bash/shell/script references  | Indirect via AI tool use       |
| Agents   | All files for Bash/shell/script references  | Indirect via AI tool use       |
| Rules    | All .md files for code blocks, script refs  | Could instruct AI to run code  |
| Commands | All files for script content                | May contain executable scripts |
| Loadouts | Aggregate from all contained items          | Composite risk                 |

---

## CLI-to-TUI Complete Action Map

### Actions by TUI Location

The `share` command is context-aware: sharing to a team repo copies files;
sharing to a registry creates a git branch and PR. The CLI already supports
this via `syllago share [--to <registry>]`.

| Location     | `[a]` Add                 | `[i]` Install | `[x]` Uninstall   | `[d]` Remove   | `[s]` Share  | Other        |
|--------------|---------------------------|---------------|-------------------|----------------|--------------|--------------|
| Library      | Add wizard                | Install modal | Uninstall confirm | Remove confirm | Share wizard | —            |
| Loadouts     | —                         | Apply modal   | —                 | Remove confirm | Share wizard | —            |
| Registries   | Add registry              | —             | —                 | Remove confirm | —            | `[S]` Sync   |
| Content tabs | Add wizard (pre-filtered) | Install modal | Uninstall confirm | Remove confirm | Share wizard | —            |
| Config       | Add Provider picker       | —             | —                 | Remove confirm | —            | `[u]` Update |

### Deferred Actions

| Action                       | Reason                                                              |
|------------------------------|---------------------------------------------------------------------|
| `[n]` Create                 | Needs design work — scaffolding is bare. Hide button until release. |
| Remote install locations     | Local dirs only for now, remote later.                              |
| Content signing/verification | Signing infrastructure is interface-only.                           |

---

## Wizard Specifications

### W1: Add Wizard (Full-Screen)

**Purpose:** Import content from a provider, registry, or local path into the library.

**Entry points:**
- `[a]` from Library tab (all types)
- `[a]` from Content > [Type] tab (pre-filtered to that type)
- "Add to Library" from registry item drill-in

**validateStep() prerequisites:**

| Step | Prerequisites (panic if violated) |
|------|-----------------------------------|
| stepSource | *(none — entry step)* |
| stepType | `source != ""` (source selected or pre-filled) |
| stepDiscovery | `source != ""` AND (`contentTypes != 0` OR `preFilterType != ""`) |
| stepReview | `len(discoveredItems) > 0` AND `len(selectedItems) > 0` |
| stepExecute | `len(selectedItems) > 0` AND `reviewAcknowledged == true` |

**Steps:**

```
Step 1: Source
  Where is the content?
  ( ) Detected provider  [Claude Code, Cursor, ...]
  ( ) Registry           [list of configured registries]
  ( ) Local path         [text input for directory]
  ( ) Git URL            [text input for URL]
```

```
Step 2: Type (skip if pre-filtered from Content tab)
  What type of content?
  ( ) Rules    ( ) Skills    ( ) Agents
  ( ) Hooks    ( ) MCP       ( ) Commands
  For provider source: checkboxes, multi-select allowed
  For registry: show what's available
```

```
Step 3: Discovery (async — show spinner)
  Scanning [source] for [type] content...

  Found 12 items:
  [Reuse library table with checkbox column]
  [x] my-rule              rules    New
  [x] pre-tool-validate    hooks    !! Runs: bash  New
  [ ] old-hook             hooks    Already in library

  Risk indicators shown inline.
  Enter drills into item (file tree + preview, same as Library detail).
  Space toggles selection. 'a' selects all. 'n' selects none.
```

```
Step 4: Review
  Adding 5 items to library:

  [Risk banner — navigable list with drill-in]

  Items:
    my-rule              rules     (new)
    pre-tool-validate    hooks     (new) !! HIGH
    mcp-server           mcp       (new) !! HIGH
    helper-skill         skills    (update — hash differs)
    team-agent           agents    (new)

  Step breadcrumbs are clickable to go back and edit selections.
  Drill into any item to review code before confirming.
```

```
Step 5: Execute (async — show progress)
  Adding items...
  [x] my-rule              Added
  [x] pre-tool-validate    Added
  [ ] mcp-server           Adding...

  Done! 5 items added to library.
  [Enter] Go to Library  [Esc] Close
```

**Type-specific behavior:**

| Type     | Discovery Source                             | Special UI                               |
|----------|----------------------------------------------|------------------------------------------|
| Hooks    | Parse provider settings.json, split by event | Show command, event, matcher per hook    |
| MCP      | Parse provider MCP config, split by server   | Show server name, command, env vars      |
| Skills   | Scan SKILL.md files                          | Show name + description from frontmatter |
| Agents   | Scan AGENT.md files                          | Show name + description from frontmatter |
| Rules    | Scan .md files                               | Show first line of content               |
| Commands | Scan command dirs                            | Show command description                 |

**Conflict handling:**
When an item already exists in the library with a different hash:
- Show conflict inline in review: "(update — content differs)"
- Drill-in shows side-by-side diff (if feasible) or "new" vs "existing"
- User can toggle overwrite per item

**Inline error display:**
When discovery fails or git clone times out, the error renders inline in
the step content (not just a toast):
```
Step 3: Discovery
  Scanning https://github.com/team/rules...

  +-- Error ----------------------------------------+
  | Git clone failed: connection timed out           |
  |                                                  |
  | [r] Retry   [b] Back to source selection         |
  +--------------------------------------------------+
```
Each step has an `err error` field for this. Toasts also fire for visibility.

**Execute step cancellation:**
The Execute step supports **partial completion**. If Esc is pressed after some
items have been added:
- Current item finishes (in-flight `tea.Cmd` completes)
- Remaining items are skipped
- Progress shows: "3 of 5 added, 2 skipped"
- User can close or retry the skipped items
- Items already added are NOT rolled back (they are in the library)

---

### W2: Install Modal (Overlay on Current Screen)

**Purpose:** Activate a library item in a provider's location.

**Entry point:** `[i]` on a selected item in Library or Content tabs.

**validateStep() prerequisites:**

| Step | Prerequisites (panic if violated) |
|------|-----------------------------------|
| installStepProvider | `item != nil` (item passed from caller) |
| installStepLocation | `provider != ""` (provider selected or auto-skipped) |
| installStepMethod | `location != ""` AND `item.Type != Hooks` AND `item.Type != MCP` |
| installStepReview | `provider != ""` AND `location != ""` |

**Steps:**

```
Step 1: Provider (skip if only one detected)
  Install to which provider?
  ( ) Claude Code    (detected)
  ( ) Cursor         (detected)  [already installed]
  ( ) Windsurf       (not detected)
```

**Provider picker shows install status:** If the selected content is already
installed in a provider, that provider shows "(already installed)" and is
disabled to prevent conflicts. This component is reused across all wizards
with provider selection (add, install, loadout apply).

```
Step 2: Location
  Install location:
  ( ) Global    (~/.claude/rules/)
  ( ) Project   (./.claude/rules/)
  ( ) Custom    [text input for local directory path]
```

Custom location accepts local directories only (remote locations deferred).

```
Step 3: Method (skip for hooks/MCP which always JSON-merge)
  Install method:
  ( ) Symlink   (recommended — stays in sync with library)
  ( ) Copy      (standalone copy, won't auto-update)
```

```
Step 4: Review + Confirm
  Installing "my-rule" to Claude Code:

  Location: ~/.claude/rules/my-rule -> ~/.syllago/content/rules/my-rule
  Method:   Symlink

  [Risk banner if applicable]

  [Enter] Install  [Esc] Cancel
```

**For Hooks/MCP:** Steps 2-3 are different:
- No location choice (always merges into provider settings)
- No method choice (always JSON merge)
- Review shows: "Will merge into ~/.claude/settings.json"
- Risk banner always shown with command details

---

### W3: Confirm Modal (Small Overlay)

**Purpose:** Generic yes/no confirmation for destructive actions.

**Used by:** Uninstall, Registry Remove, Loadout Remove

**Focus order (Tab cycle):**
- Simple variant: Cancel (default) → Confirm → Cancel
- Checkbox variant: Checkbox 1 → Checkbox 2 → ... → Checkbox N → Cancel (default) → Confirm → Checkbox 1
- Space toggles checkboxes, Enter only fires on buttons
- "Delete from library" checkbox is always checked and read-only (visually distinct)
- `y` shortcut = Confirm, `n` shortcut = Cancel (work from any focus position)

**Design:**
```
+-- Uninstall "my-hook"? --------+
|                                 |
|  Remove from: Claude Code       |
|  Location:    ~/.claude/...     |
|                                 |
|  Content stays in your library. |
|                                 |
|      [Cancel]   [Uninstall]     |
+---------------------------------+
```

- Danger-styled border (red) for destructive actions
- Default focus on Cancel (safe default)
- `y`/`n` keyboard shortcuts
- Shows what will happen (context-specific message)

### Remove Confirmation (Enhanced)

Remove is a destructive action with side effects. It gets a more detailed
confirmation than simple uninstall:

```
+-- Remove "my-hook"? ----------------------------+
|                                                   |
|  This item is installed in 2 providers:           |
|                                                   |
|  [x] Uninstall from Claude Code                  |
|  [x] Uninstall from Cursor                       |
|  [x] Delete from library                         |
|                                                   |
|  This cannot be undone.                           |
|                                                   |
|            [Cancel]     [Remove]                  |
+---------------------------------------------------+
```

- Shows which providers have this content installed (checkboxes)
- User can choose to uninstall from some/all providers
- "Delete from library" is always checked (that's what remove means)
- If no providers have it installed, simplified to just the delete confirmation
- Loadout remove is simpler: just delete from disk, no provider side effects

---

### W4: Registry Add Modal (Overlay)

**Purpose:** Add a new content registry (git URL or local path/repo).

**Entry point:** `[a]` on Registries tab.

**Design:**
```
+-- Add Registry -------------------------+
|                                          |
|  Source                                  |
|  ( ) Git URL                             |
|  ( ) Local directory / repo              |
|                                          |
|  URL / Path                              |
|  [https://github.com/team/registry___]   |
|                                          |
|  Name (optional — derived from URL)      |
|  [team-registry________________________] |
|                                          |
|  Branch / Tag (optional, git only)       |
|  [main_________________________________] |
|                                          |
|            [Cancel]   [Add]              |
+------------------------------------------+
```

- Source toggle switches between URL and local path validation
- **Local directory:** validates directory exists, checks for registry.yaml
- **Local git repo:** validates it's a git repo, can be bare or working tree
- **Remote git URL:** will clone on confirm (async with spinner)
- Name auto-derived from URL/path basename if not provided
- Branch/Tag field only shown for git sources
- After adding: auto-syncs (clone/pull) and shows toast

---

### W5: Loadout Apply Modal (Overlay)

**Purpose:** Apply a loadout to configure a provider.

**Entry point:** Enter on a loadout card, or `[i]` on Loadouts tab.

**Design:**
```
+-- Apply "team-starter"? ----------------+
|                                          |
|  Provider: Claude Code                   |
|  Items:    3 rules, 2 hooks, 1 MCP      |
|                                          |
|  [Risk banner if items have code]        |
|                                          |
|  Mode:                                   |
|  ( ) Preview  (dry run, no changes)      |
|  ( ) Try      (temporary, reverts on     |
|               session end)               |
|  ( ) Keep     (permanent install)        |
|                                          |
|           [Cancel]   [Apply]             |
+------------------------------------------+
```

- Preview is default (safe)
- Try mode shows: "Will auto-revert when session ends"
- Keep mode shows: "Permanent — use loadout remove to undo"
- Risk banner aggregates warnings from all contained items

---

### W6: Share Wizard (Full-Screen)

**Purpose:** Contribute library content or loadouts to a team repo or registry.
Context-aware: team repo shares copy files; registry shares create git branches
and PRs. Maps to `syllago share [--to <registry>]`.

**Entry point:** `[s]` on a selected item in Library, Content tabs, or Loadout card.

**validateStep() prerequisites:**

| Step | Prerequisites (panic if violated) |
|------|-----------------------------------|
| shareStepDest | `item != nil` (item passed from caller) |
| shareStepPrivacy | `destination != ""` AND `item.Meta.SourceVisibility == "private"` |
| shareStepReview | `destination != ""` |
| shareStepExecute | `reviewAcknowledged == true` |

**Steps:**

```
Step 1: Destination
  Share to:
  ( ) Team repo    (current project's content/ directory)
  ( ) Registry     [list of configured registries]
```

```
Step 2: Privacy Check (conditional — only if item from private source)

  WARNING: This item was imported from a private registry.
  Sharing it may expose proprietary content.

  ( ) Continue anyway
  ( ) Cancel
```

```
Step 3: Review
  Sharing "my-rule" to registry "team-tools":

  Files:
    rules/my-rule/rule.md
    rules/my-rule/.syllago.yaml

  [Enter] Share  [Esc] Cancel
```

```
Step 4: Execute (async)
  [If team repo]  Copying to content/rules/my-rule...
  [If registry]   Creating branch and staging files...

  Done!
  [If registry]  Files staged in branch "syllago/share-my-rule".
  [Enter] Close
```

---

## Simple Actions (No Wizard)

| Action          | Trigger                 | Behavior                                                |
|-----------------|-------------------------|---------------------------------------------------------|
| Registry Sync   | `[S]` on Registries tab | `git pull` on selected or all registries. Toast result. |
| Catalog Refresh | `R` anywhere            | Re-scan all content. Toast result. Already implemented. |
| Self-Update     | `[u]` on Config tab     | Check for updates, download if available. Toast result. |

---

## Testing Strategy

Target: **80% minimum per package, 95%+ aspirational** (consistent with project standard).

### Testing Layers

1. **Component unit tests** — Each new component tested in isolation
2. **Wizard flow tests** — Step transitions, state preservation, back-navigation
3. **Integration tests** — Full wizard → backend operation → result verification
4. **Golden file tests** — Visual snapshots at key sizes (60x20, 80x30, 120x40)
5. **Wizard invariant tests** — Entry-prerequisites per step (existing pattern)

### Phase A: Confirm Modal + Per-Item Actions

**Component tests (`confirm_test.go`):**
- Open/close lifecycle
- Default focus on Cancel (safe default)
- `y`/`n` keyboard shortcuts produce correct messages (work from any focus)
- Enter on Cancel → cancelled message
- Enter on Confirm → confirmed message with checkbox selections
- Esc → cancelled message
- Tab cycles: checkboxes → Cancel → Confirm → checkboxes (wrap)
- Space toggles checkbox without advancing focus
- Enter on checkbox does NOT confirm (only on Confirm button)
- "Delete from library" checkbox is always checked and read-only
- All provider checkboxes unchecked + delete checked = valid (library-only remove)
- Danger border styling applied when `danger: true`
- Inactive modal ignores all input
- Mouse: click checkbox toggles it, click Cancel/Confirm fires action

**App integration tests:**
- `[d]` on Library item → confirm modal opens with correct item name
- `[d]` on installed item → shows provider uninstall checkboxes
- `[d]` on non-installed item → simplified confirm (no checkboxes)
- `[x]` on item → uninstall confirm opens with provider name
- `[x]` on non-installed item → no-op or toast "not installed"
- Confirm remove → item removed from catalog, toast shown
- Confirm uninstall → item uninstalled, toast shown
- Cancel → no changes, modal closes
- `[d]` on loadout card → simplified confirm (just delete, no checkboxes)
- `[n]` button hidden (deferred create)

**Golden files:**
- Confirm modal default state (80x30)
- Confirm modal danger variant (80x30)
- Remove confirm with checkboxes (80x30)
- Uninstall confirm (80x30)
- All at 60x20 minimum size

### Phase B: Install Modal

**Component tests (`install_modal_test.go`):**
- Provider picker: shows detected providers
- Provider picker: "already installed" disables provider, shows badge
- Provider picker: single-provider auto-skips step
- Location selection: global/project/custom
- Custom path text input: typing, editing, space handling
- Custom path: empty rejected, nonexistent warns
- Method selection: symlink/copy, symlink disabled when unsupported
- Hooks/MCP: location and method steps skipped (JSON merge shown)
- Review step: shows correct destination path (global symlink, project copy)
- Review step: shows risk banner for hooks/MCP
- Step transitions: forward, back, skip logic
- Back from location to provider preserves selections
- Back from method to location preserves selections
- Esc exits modal from any step
- Enter on review confirms and produces install message
- Double-confirm prevention: Enter on review disabled after first press
- Stale async result ignored after modal dismissal

**Provider picker reuse tests (`provider_picker_test.go`):**
- List rendering with status badges
- Already-installed detection per content type
- Provider detection status (detected vs not)
- Keyboard navigation between providers

**Risk banner tests (`risk_banner_test.go`):**
- Renders with HIGH/MEDIUM items
- Border color: RED for any HIGH, ORANGE for MEDIUM only, RED for all HIGH
- Navigation: arrows move between items, first auto-highlighted
- Enter produces drill-in message for focused item
- Single item: no arrow navigation needed
- Empty list → no banner rendered (zero height)
- Command preview truncated for long commands

**App integration tests:**
- `[i]` on rule item → install modal opens
- `[i]` on hook item → skips location/method, shows JSON merge info
- Full install flow: select provider → location → method → confirm → toast
- Install to already-installed provider → provider disabled in picker
- Risk banner appears for hooks/MCP items

**Golden files:**
- Install modal at each step (80x30, 60x20)
- Provider picker: mixed status (detected, installed, not detected)
- Risk banner: HIGH items, MEDIUM only, all HIGH, mixed
- Install modal for hooks (skipped steps, JSON merge info)

### Phase C: Registry Management

**Component tests (`registry_add_test.go`):**
- Source toggle: git URL vs local dir
- URL field input and editing
- Name auto-derivation from URL
- Name auto-derivation from local path basename
- Branch field shown only for git source
- Validation: empty URL rejected
- Enter on Add produces registry add message with all fields

**App integration tests:**
- `[a]` on Registries tab → registry add modal opens
- Add git URL → clone async, toast on success/failure
- Add local dir → validates exists, toast result
- `[S]` on Registries tab → sync action, toast result
- `[d]` on registry card → confirm modal, remove on confirm

**Golden files:**
- Registry add modal (git URL mode, local dir mode)

### Phase D: Add Wizard

**Wizard shell tests (`wizard_shell_test.go`):**
- Step bar renders correct count for 3, 5, and 7 steps
- Active step highlighted (bold + primary color)
- Completed steps clickable (underlined)
- Future steps muted and non-clickable
- Click on completed step produces navigation message
- Click on future step is no-op
- Step bar truncation at narrow widths (60 chars)
- State preserved when navigating back without changes
- State invalidated when step version changes (stepDirty mechanism)
- Esc with no changes → exits immediately
- Esc with changes → prompts unsaved-changes confirmation
- Help bar shows step-specific hints per active step
- Global keys (R, 1/2/3) suppressed during wizard mode
- Golden: 5-step wizard at 80x30 and 60x20

**Checkbox list tests (`checkbox_list_test.go`):**
- Space toggles item selection
- `a` selects all, `n` selects none
- Arrow keys navigate
- Enter produces drill-in message for focused item
- Disabled items cannot be selected
- Visual: checked/unchecked rendering

**Add wizard flow tests (`add_wizard_test.go`):**
- **Step 1 Source:** Select each source type, verify step transition
- **Step 2 Type:** Single select, multi-select, pre-filtered skip
- **Step 3 Discovery:** Async loading state, items rendered with risk badges
- **Step 3 Discovery:** Selection toggle, select all, deselect all
- **Step 3 Discovery:** Already-in-library items shown with status
- **Step 3 Discovery:** Enter drills into item preview
- **Step 4 Review:** Risk banner shown when items have code
- **Step 4 Review:** Back-navigation to change selections
- **Step 4 Review:** Conflict items marked, overwrite toggleable
- **Step 5 Execute:** Progress rendering, completion toast
- **Full flow:** Source → Type → Discovery → Review → Execute → Library
- **Pre-filtered:** Entry from Content > Hooks → skips type step
- **Registry source:** Shows registry items, no provider picker
- **Git URL source:** Text input, async clone, discovery
- **Back-nav:** Review → Selection, selections preserved
- **Back-nav:** Type change invalidates discovery (version counter)
- **Back-nav:** Source change invalidates everything downstream
- **Async:** Esc during discovery cancels cleanly, wizard exits
- **Async:** Stale discoveryDoneMsg with old seq ignored
- **Async:** Stale importCloneDoneMsg with old seq ignored
- **Async:** Double-confirm on Execute step prevented
- **Empty discovery:** No items found shows "nothing found" message
- **Error state:** Discovery scan failure shows error toast

**Per-content-type discovery tests:**
- Hooks: parse settings.json, split by event, show commands
- MCP: parse config, split by server, show command/env
- Skills: scan SKILL.md, extract frontmatter name/description
- Agents: scan AGENT.md, extract frontmatter
- Rules: scan .md files, extract first line
- Commands: scan command dirs, extract descriptions

**Wizard invariant tests (add to `wizard_invariant_test.go`):**
- Forward-path: all steps complete without panic
- Back-path: navigate back from each step without panic
- Entry prerequisites: each step validates its preconditions
- Parallel arrays: correlated slices stay in sync

**Golden files:**
- Each wizard step at 80x30, 120x40, and 60x20 (minimum)
- Risk banner states: no risk, medium only, high + medium, all high
- Checkbox list: mixed selections, all selected, none selected
- Error states: discovery failure, git clone failure
- Empty discovery: "no items found" state

### Cross-Cutting: Async Safety Tests

These tests verify the `seq` pattern across all async operations:

- `TestApp_StaleDiscoveryDoneMsgIgnored` — wizard dismissed, late msg arrives
- `TestApp_StaleImportCloneDoneMsgIgnored` — clone completes after cancel
- `TestApp_StaleShareExecuteDoneMsgIgnored` — share completes after cancel
- `TestApp_WizardDismissedBeforeAsyncCompletes` — no panic on late delivery
- `TestApp_RefreshIgnoredDuringWizard` — `R` key no-op when wizard active
- `TestApp_DoubleConfirmOnExecuteStep` — second Enter ignored
- `TestApp_BackNavDuringAsync` — going back increments seq, old result dropped

### Phase E: Loadout Apply + Remove + Share

**Loadout apply tests (`loadout_apply_test.go`):**
- Mode selection: preview/try/keep
- Default mode is preview
- Risk banner with aggregated item warnings
- Confirm produces apply message with correct mode
- Cancel closes modal

**Share wizard tests (`share_wizard_test.go`):**
- Step 1: Destination selection (team repo vs registry)
- Step 2: Privacy check shown only for private-source items
- Step 2: Privacy check skipped for public items
- Step 3: Review shows correct files and destination
- Step 4: Async execution with progress
- Full flow: destination → privacy → review → execute
- Loadout share: same flow, loadout files in review

**App integration tests:**
- Enter on loadout card → apply modal
- `[d]` on loadout card → confirm remove
- `[s]` on Library item → share wizard
- `[s]` on loadout → share wizard with loadout files

### Code Scanning Tests (Research Spike, before Phase D)

**Expanded `risk_test.go` tests:**
- Hooks: detect shell commands, network URLs, env var access
- MCP: detect server launch commands, environment variables
- Skills: detect Bash references across multi-file directories
- Agents: detect Bash/shell references in AGENT.md and sub-files
- Rules: detect code blocks in markdown, script file references
- Commands: detect executable scripts, shebang lines
- All types: detect obfuscated patterns (base64, eval, backticks)
- Loadouts: aggregate risk from contained items correctly
- Edge cases: empty files, binary files, deeply nested directories

**`hook_security_test.go` expansion:**
- All HIGH patterns: curl, wget, nc, ssh, recursive rm
- All MEDIUM patterns: chmod, env reads, wildcard matchers
- All LOW patterns: writes to system directories
- Combined patterns: multiple risks in one hook
- Clean hooks: no false positives on benign commands

### Integration Test Strategy

TUI tests verify the wizard produces the correct **messages**, not filesystem
side effects. Backend operations are tested separately in their own packages:

| TUI Test Verifies | Backend Test Verifies |
|---|---|
| Install modal → `doInstallMsg{provider, location, method}` | `installer` package → symlink/copy/merge |
| Add wizard → `doAddMsg{items, source, options}` | `add` package → file writes + metadata |
| Share wizard → `doShareMsg{item, destination}` | `promote` package → git branch + staging |
| Remove confirm → `doRemoveMsg{item, providers}` | `remove` logic → uninstall + delete |

This keeps TUI tests fast (no filesystem), focused (UI behavior only), and
independent of backend implementation changes.

For provider install status detection, use a helper:
```go
func testProviderWithInstall(name, slug string, installed map[string]bool) provider.Provider
```
Where `installed` maps item names to install status, avoiding filesystem checks.

### Test Infrastructure

**New test helpers needed:**
- `testConfirmModal(t)` — creates confirm modal with test data
- `testInstallModal(t)` — creates install modal with provider stubs
- `testWizardShell(t, steps)` — creates wizard shell with given steps
- `testCheckboxList(t, items)` — creates checkbox list with test items
- `testRiskBanner(t, items)` — creates risk banner with test risk data
- `keySpace` helper (already exists pattern in v1 tests)

**Fixture helpers:**
- Provider stubs with install status detection
- Catalog items with various risk profiles
- Registry fixtures (local dir, git URL)
- Loadout manifests with mixed content types

---

## Build Order (Implementation Phases)

### Phase Execution Process

Every phase follows the same gated workflow. No step begins until its
blockers are complete. This is enforced via bead dependencies.

```
1. Write Implementation Plan
   │  Detailed spec: file changes, function signatures, message types,
   │  test list with success criteria per task.
   │
2. Parity Validation Gate
   │  Sub-agent cross-references the plan against this design doc.
   │  Verifies: every design requirement covered, no contradictions
   │  with Go Architecture Rules, all test cases have success criteria,
   │  validateStep prerequisites reflected, async safety patterns included.
   │  BLOCKING: no implementation begins until parity confirmed.
   │
3. Implementation Tasks (may run in parallel where deps allow)
   │  Each task has explicit success criteria defined in the plan.
   │  Task is not marked complete until criteria are met.
   │
4. Per-Task Validation
   │  Separate bead per implementation task. A DIFFERENT agent validates
   │  the work meets success criteria — the implementing agent does not
   │  self-validate. Runs tests, reviews goldens, checks criteria.
   │
5. Phase Validation
      Final bead blocked by all per-task validations.
      Runs full test suite (make test), builds binary, smoke tests in
      real TUI. Confirms phase is complete and ready for next phase.
```

**Bead dependency structure per phase:**

```
Plan (READY)
  └── Parity Check (blocked by plan)
      ├── Impl Task A ──→ Validate Task A
      ├── Impl Task B ──→ Validate Task B  (B may depend on A)
      ├── Impl Task C ──→ Validate Task C
      └── ...
          └── Phase Validation (blocked by ALL task validations)
```

**Incremental planning:** Only the current phase gets a full implementation
plan and beads. The next phase is planned after the current phase passes
its final validation. This prevents wasted planning if earlier phases
reveal design changes.

---

### Phase A: Foundation

**Confirm modal + per-item actions**

- Build generic `confirmModal` component (with optional checkboxes for remove)
- Wire `[d]` Remove (enhanced confirm with provider uninstall checkboxes)
- Wire `[x]` Uninstall (simple confirm → `installer.Uninstall`)
- Add hotkey hints to help bar and help overlay
- Toast feedback for all actions
- Hide `[n]` Create button (deferred)

**Why first:** Simplest. Establishes modal overlay pattern for all future work.
Enables destructive actions on existing content immediately.

### Phase B: Install

**Install modal with provider picker**

- Build provider picker component with "already installed" detection (reusable)
- Build install modal with provider/location/method steps
- Support custom local directory paths
- JSON merge path for hooks/MCP (skip method step)
- Risk banner component (reused by all later wizards)
- Wire `[i]` Install hotkey

**Why second:** Completes the "browse library → install to provider" loop.
Users with content already in library can immediately use it.

### Phase C: Registry Management

**Registry add modal + sync action**

- Build registry add modal (URL or local dir/repo)
- Wire `[a]` on Registries tab
- Wire `[S]` Sync action + toast
- Wire `[d]` Registry remove (reuse confirm modal)

**Why third:** Prerequisite for adding content from registries.

### Phase D: Add Wizard

**Full-screen add/import wizard** (preceded by code scanning research spike)

- **Research spike:** Expand pattern-based code detection to cover all content
  types. Produce updated `risk.go` with comprehensive scanning before building
  the wizard UI.
- Build wizard shell component (step breadcrumbs + back-navigation + state)
- Build checkbox list component
- Build navigable risk banner component
- Implement all 5 steps: source → type → discovery → review → execute
- Per-content-type discovery (provider scan, registry browse, local path, git)
- Async discovery with spinner
- Conflict detection and per-item overwrite toggle
- Universal code scanning on all items

**Why fourth:** The biggest piece. Depends on wizard shell, risk banner,
and checkbox list — all new components. Benefits from registry add being
available for registry-source imports.

### Phase E: Loadout Apply + Remove + Share

**Loadout actions + share wizard**

- Build apply modal with preview/try/keep modes
- Aggregate risk warnings from loadout contents
- Wire Enter on loadout cards → apply
- Wire `[d]` on loadout cards → remove (reuse confirm modal)
- Build share wizard (destination → privacy check → review → execute)
- Wire `[s]` on Library, Content, and Loadout items

**Why fifth:** Depends on install infrastructure from Phase B.

### Deferred to Release

- `[n]` Create wizard — needs design work, hide button for now
- Remote install locations
- Content signing/verification
- Self-update TUI (keep as CLI for now)

---

## Hotkey Summary (Final State)

| Key | Action        | Context                                             |
|-----|---------------|-----------------------------------------------------|
| `a` | Add           | Library, Content tabs, Registries                   |
| `i` | Install       | Selected item (Library, Content) / Apply (Loadouts) |
| `x` | Uninstall     | Installed item                                      |
| `d` | Remove/Delete | Any item, registry, or loadout                      |
| `s` | Share         | Any item or loadout                                 |
| `S` | Sync          | Registries tab                                      |
| `e` | Edit          | Any item (already implemented)                      |
| `R` | Refresh       | Anywhere (already implemented)                      |
| `?` | Help          | Anywhere (already implemented)                      |
| `u` | Update        | Config tab                                          |
| `n` | Create        | **HIDDEN** (deferred)                               |

### Hotkey Rationale: `[d]` for Remove

`[d]` chosen over `[r]` because:
- Universal convention (vim `dd`, file managers, most TUIs use `d`/`Delete`)
- `[r]` could be confused with `[R]` Refresh (case-sensitive but easy to mix up)
- `[e]` Edit and `[d]` Delete are a natural pair
- Lowercase `r` is available but introduces ambiguity with uppercase `R`
