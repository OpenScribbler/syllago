# TUI Polish + Registry Experience — Design Document

**Goal:** Complete the final feature workstream before v1 docs. Fix all UX issues, add global content model, overhaul init experience, clean up registry story, and make every TUI flow consistent and polished.

**Decision Date:** 2026-03-01

---

## Problem Statement

The TUI has functional screens but inconsistent behavior across them. Modals don't close properly, click targets are missing, help bars vary per screen, and the install flow has ghost state bugs. The first-run experience references a non-existent community registry. The content model is project-scoped only, which means personal content doesn't follow users between projects. All of this needs to be fixed for v1.

---

## Proposed Solution

Four workstreams executed in order:

1. **UX Bug Fixes + Consistency** — Fix the 10 reported issues from `docs/ui-issues.md`
2. **Global + Project Content Model** — Add `~/.syllago/content/` as a global content location merged with project content
3. **Init Overhaul** — Interactive Astro-style wizard for `syllago init`
4. **Registry Experience** — Remove syllago-tools references, add `syllago registry create` wizard, upgrade TUI registry browser

---

## Workstream 1: UX Bug Fixes + Consistency

Source: `docs/ui-issues.md` + session findings

### 1.1 Unified Help Bar

**Current:** Different help text per screen — category screen shows one set of shortcuts, items screen shows another, detail screen shows yet another.

**Target:** Single persistent help bar at the bottom of every screen. Context-sensitive content but same visual bar. Shows relevant shortcuts for the current screen/focus state.

### 1.2 Quit vs Back Behavior

**Current:** `q` shows as "Quit" on inner screens but actually navigates back.

**Target:**
- Landing page (screenCategory): `q` = Quit. Help bar shows "q quit".
- All other screens: `q` and `Esc` = Back. Help bar shows "esc back". No "Quit" label.

### 1.3 Clickable Selections

**Current:** Provider checkboxes on install tab are keyboard-only (space to toggle). Modal options are keyboard-only.

**Target:** Every element selectable via keyboard is also clickable:
- Provider checkboxes on install tab → click to toggle
- Modal options (location picker, method picker) → click to select
- Settings toggles → click to toggle
- Sandbox list items → click to select

Implementation: Add `zone.Mark()` zones around selectable items, handle `tea.MouseMsg` clicks in the update loop.

### 1.4 Install Flow Layout Restructure

**Current:** Install/Uninstall/Copy/Save buttons are inline with metadata, not clearly separated from the provider list.

**Target:**
```
  Providers
  ─────────
  [x] Claude Code
  [ ] Cursor
  [x] Gemini CLI

  Actions
  ─────────
  [ Install ]  [ Uninstall ]  [ Copy ]  [ Save ]
```

Buttons appear BELOW providers list. Section headers "Providers" and "Actions" make the flow clear: select providers first, then choose an action.

### 1.5 Modal Click-to-Select

**Current:** Modal options only respond to keyboard (up/down + enter).

**Target:** Click any option in a modal to select it. Uses `zone.Mark()` per option row.

### 1.6 Modal Action Buttons Pinned

**Current:** "Select" / "Cancel" text floats after content, causing jitter when content height changes.

**Target:** Pin action hints to the bottom of the modal frame. Modal layout:
```
┌─ Title ──────────────┐
│                       │
│  Option 1             │
│  Option 2             │
│  Option 3             │
│                       │  ← flexible space
│  enter select · esc cancel  │  ← pinned to bottom
└───────────────────────┘
```

### 1.7 Install Method Text Wrapping

**Current:** Symlink description wraps with continuation text flush-left instead of aligned with the description start.

**Target:** Wrapped text indents to align with the start of the description, not the start of the line. Example:
```
  ● Symlink  Stays in sync with repo, auto-updates
             on git pull
```

### 1.8 Modal Escape + Click-Away

**Current:** Clicking outside a modal or pressing Escape doesn't close it.

**Target:** Escape always closes the active modal (returns to previous state without committing). Clicking outside the modal bounds also closes it.

### 1.9 Ghost State in Install Flow

**Current:** Navigating away from an install modal without confirming leaves pending state. Pressing Enter elsewhere triggers the install.

**Target:** Cancelling or navigating away from any modal/wizard resets ALL pending state for that flow. No action is taken unless the user explicitly confirms in the final step.

Implementation: On modal close (escape, click-away, or cancel), reset:
- `installStep` back to 0
- `installLocation` back to empty
- `installMethod` back to default
- Any other wizard-specific state

### 1.10 Cross-Screen Consistency Audit

After fixing 1.1–1.9, audit every screen to verify:
- Same help bar pattern
- Same click behavior
- Same escape/back behavior
- Same modal patterns
- Same button styling

Screens to audit: category, items, detail (all 3 tabs), import (all steps), update, settings, registries, sandbox.

---

## Workstream 2: Global + Project Content Model

### Current Model

Content is project-scoped: `findContentRepoRoot()` walks up from cwd to find a project root, then looks for content directories there. No global content location.

### Target Model

Two content sources, merged in the TUI:

| Source | Location | When visible | Managed by |
|--------|----------|-------------|------------|
| **Global** | `~/.syllago/content/` | Always | User (personal toolkit) |
| **Project** | `<project>/.syllago/content/` or `<project>/content/` | When inside a project | Team (shared via repo) |

**Precedence:** Project content takes precedence over global when names collide (same pattern as Claude Code global/project CLAUDE.md, VS Code user/workspace settings).

**TUI display:** Items show a source badge — `[GLOBAL]`, `[PROJECT]`, `[REGISTRY]`, `[BUILT-IN]` — so users know where each item comes from.

### Implementation

- `~/.syllago/content/` created during first `syllago init` (global init)
- `~/.syllago/config.json` stores global config (providers, preferences)
- Project config at `<project>/.syllago/config.json` overrides global where set
- `catalog.ScanWithRegistries()` extended to accept multiple content roots
- TUI's `NewApp()` receives merged catalog with source annotations

### Config Resolution Order

1. Project `.syllago/config.json` (if inside a project)
2. Global `~/.syllago/config.json` (always)
3. Defaults

For providers: project config wins if set. For registries: merged (both global and project registries shown). For preferences: project overrides global per-key.

---

## Workstream 3: Init Overhaul

### Current Init

Bare `syllago init` — prints detected tools, asks Y/n to save config. No colors, no progressive walkthrough.

### Target Init

Interactive wizard with clear visual progression. Works for two scenarios:

**First-time global init** (`syllago init` with no existing `~/.syllago/`):
```
  ┌─────────────────────────────────────┐
  │       Welcome to syllago!           │
  └─────────────────────────────────────┘

  Let's get you set up. First, I'll create your
  global content directory at ~/.syllago/content/

  Detected AI coding tools:
    ✔ Claude Code
    ✔ Cursor
    ○ Gemini CLI (not found)
    ○ Windsurf (not found)

  Which tools do you want syllago to manage?
  Use space to toggle, enter to confirm.
    [x] Claude Code
    [x] Cursor

  Do you want to add any registries?
  Registries are git repos with shared content.
    > Add a registry URL
    > Create a new registry
    > Skip for now

  ✔ Created ~/.syllago/config.json
  ✔ Created ~/.syllago/content/

  Run 'syllago' to open the TUI, or 'syllago --help' for commands.
```

**Project-level init** (`syllago init` inside a project that already has global config):
```
  Setting up syllago for this project.

  Global config: ~/.syllago/config.json (2 providers)

  This project will get its own content directory
  at .syllago/content/ for team-shared content.

  Use the same providers as global? [Y/n]

  ✔ Created .syllago/config.json
  ✔ Created .syllago/content/
  ✔ Added .syllago/content/ to .gitignore

  Team members can add content here that everyone shares.
```

### Registry Add Flow (within init)

When user picks "Add a registry URL":
```
  Registry URL (git): https://github.com/acme/ai-rules.git
  Name (auto-detected): ai-rules

  ✔ Added registry: ai-rules
  ✔ Cloned 12 items
```

When user picks "Create a new registry":
```
  Registry name: my-team-rules
  Description (optional): Our team's AI coding rules

  ✔ Created my-team-rules/ with registry structure:
      my-team-rules/
      ├── registry.yaml
      ├── rules/
      ├── skills/
      └── agents/

  To share it, push to a git host:
    cd my-team-rules
    git init && git add . && git commit -m "init"
    gh repo create my-team-rules --push
```

### Implementation

- Use `bubbletea` for the interactive wizard (same library as TUI)
- Detect terminal capabilities — fall back to simple Y/n prompts if not interactive
- `--yes` flag skips all prompts with defaults (for CI/scripting)
- Wizard state machine: detect → select providers → registries → confirm → create

---

## Workstream 4: Registry Experience

### Remove syllago-tools References

| File | Change |
|------|--------|
| `cli/internal/registry/registry.go` | Remove or empty `KnownAliases` map |
| `cli/internal/tui/app.go` | Remove "Add a community registry: syllago registry add syllago-tools" from first-run screen |
| `cli/cmd/syllago/promote_cmd.go` | Change example to use generic registry name |
| `cli/internal/registry/registry_test.go` | Update tests |
| `cli/cmd/syllago/registry_cmd_test.go` | Update tests |
| `README.md` | Remove syllago-tools references |
| Strategy doc | Remove syllago-tools repo references |

### First-Run Screen Update

Replace the current first-run text (which references syllago-tools) with:

```
Welcome to syllago!

No content found. Here's how to get started:

1. Import existing content:   syllago import --from claude-code
2. Add a registry:            syllago registry add <git-url>
3. Create new content:        syllago create skill my-first-skill
4. Create a registry:         syllago registry create my-registry

Press ? for help, q to quit
```

### Registry Create Command

New command: `syllago registry create <name>`

Interactive wizard:
1. Ask for name (or use arg)
2. Ask for description (optional)
3. Scaffold directory structure:
   ```
   <name>/
   ├── registry.yaml      # name, description, version
   ├── skills/
   ├── agents/
   ├── rules/
   └── README.md           # Getting started instructions
   ```
4. Print next steps (git init, push to host)

### TUI Registry Browser Upgrade

**Current:** Read-only table showing name, status, items, version, URL.

**Target:** Drill into a registry to browse its items by category. Select an item to view details and install.

Flow: Registries screen → select registry → shows items grouped by type → select item → detail view with Install action.

This turns the registry browser from a status dashboard into an actual browsing experience.

---

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Content model | Global + project merged | Matches industry pattern (Claude Code, VS Code). Personal toolkit follows user, project content is team-shared. |
| Community registry | None | Syllago is a platform, not a content source. Users bring their own registries. |
| Init style | Interactive bubbletea wizard | First impression matters. Astro-style experience builds confidence. |
| Registry create | Local scaffold + instructions | GitHub integration is post-v1. Keep it simple — create structure, user pushes. |
| Bug fixes first | Before new features | Polish existing flows before adding new ones. Foundation must be solid. |

---

## Success Criteria

- Every modal closes on Escape and click-away
- Every selectable element is clickable
- Help bar is consistent across all screens
- `syllago init` from a fresh install produces a working setup with clear guidance
- Global content shows up in TUI from any directory
- No references to syllago-tools anywhere in codebase
- `syllago registry create` scaffolds a valid registry
- All golden tests updated and passing

---

## Open Questions

1. **Global content directory name** — `~/.syllago/content/` or `~/.syllago/` with content dirs at the top level? The former is cleaner if we also store config there.
2. **Badge styling in TUI** — Current badges: `[BUILT-IN]` (purple), `[LOCAL]` (amber). Need to add `[GLOBAL]` and `[PROJECT]`. Color choices?
3. **Registry browser depth** — Should drilling into a registry show the same category sidebar as the main view, or a flat list?

---

## Implementation Order

1. UX Bug Fixes + Consistency (Workstream 1)
2. syllago-tools removal (Workstream 4, partial)
3. Global + Project Content Model (Workstream 2)
4. Init Overhaul (Workstream 3)
5. Registry Create + Browser Upgrade (Workstream 4, remainder)

Bug fixes first because they affect every other workstream. syllago-tools removal is quick and unblocks the init redesign. Content model is the architectural change everything else builds on. Init and registry features layer on top.

---

## Next Steps

Ready for implementation planning with `Plan` skill.
