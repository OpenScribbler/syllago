# TUI AI Context Architecture — Design Doc

**Date:** 2026-03-27
**Status:** Draft
**Scope:** File splitting, rule file rewrite, skill restructure, hook enforcement, deduplication

---

## Problem Statement

Deep research into AI-assisted Go TUI development (bubbletea/lipgloss) revealed that the #1 strategy for getting good results is encoding bubbletea-specific invariants into persistent context files. Our audit found a critical gap: **all 12 TUI rule files are scoped to `tui_v1/**` and none trigger for the v3 `tui/` directory.** This means AI gets zero path-scoped rules when editing the current TUI.

Additionally:
- The v3 `CLAUDE.md` (199 lines) and `tui-builder` skill (510 lines) overlap significantly
- Large files (`app.go` at 966 lines, `install.go` at 1068 lines) exceed the 500-800 line threshold research identifies as optimal for AI edit quality
- Key bubbletea invariants (Elm architecture enforcement, layout arithmetic, `tea.Cmd` discipline) are missing from all context files
- The v1 TUI was a clean-slate rewrite — v1 rule content doesn't apply to v3

### Research-Backed Principles

From surveying community strategies, blog posts, charmbracelet discussions, and production TUI projects:

1. **Encode bubbletea rules in persistent context** — the single highest-impact strategy
2. **Split files by concern, keep under 500-800 lines** — gives AI tractable edit surfaces
3. **Write message contracts before implementation** — AI needs to know what messages a component sends/receives
4. **Golden file tests as verification backbone** — turns visual correctness into machine-checkable assertions
5. **Break TUI work into single-component tasks** — "implement the entire wizard" fails; "implement step 3" succeeds

---

## Decision Summary

| Decision | Choice |
|----------|--------|
| V1 rules | Delete all 12 (clean-slate rewrite, content doesn't apply) |
| V1 CLAUDE.md | Delete (`cli/internal/tui_v1/CLAUDE.md`) |
| File splitting | Component boundaries: model/update/view per large file |
| New rules | Write from v3 code + research findings |
| Skill restructure | Slim to golden rules + layout reference; move patterns to rules |
| Deduplication | Single source of truth per concern |
| Hook | Enforce `/tui-builder` skill loading when editing `tui/` files |

---

## Part 1: File Splitting

Split files exceeding ~800 lines into model/update/view concerns. This runs first so that rules and docs can reference the correct file boundaries.

### app.go (966 lines) -> 3 files

| File | Contents | Target Lines |
|------|----------|:--:|
| `app.go` | Model struct, `NewApp()`, `Init()`, constructors, helpers, type defs | ~300 |
| `app_update.go` | `Update()`, all message handlers, `rebuildItems()`, `rescanCatalog()` | ~400 |
| `app_view.go` | `View()`, `overlayModal()`, layout composition, helpbar delegation | ~250 |

### install.go (1068 lines) -> 3 files

| File | Contents | Target Lines |
|------|----------|:--:|
| `install.go` | Model struct, step enum, `newInstallModal()`, `Init()`, `validateStep()` | ~300 |
| `install_update.go` | `Update()`, step transition logic, message handlers | ~400 |
| `install_view.go` | `View()`, per-step rendering, risk banner, progress display | ~350 |

### Splitting Rules

- No behavioral changes — pure mechanical extraction
- All types, constants, and package-level vars stay in the model file (e.g., `app.go`)
- Method receivers stay with the file that defines the method's concern

### Verification Protocol (Per Split)

Each file split (app.go, then install.go) follows this sequence. Do NOT proceed to the next split until all checks pass.

**Step 1: Pre-split baseline**
```bash
cd cli && go build ./...                    # confirm clean build before touching anything
go test ./internal/tui/ -count=1            # confirm all tests pass
go test ./internal/tui/ -count=1 -run Golden # confirm golden baselines match
```
Save the test output — this is your comparison baseline.

**Step 2: Mechanical split**
- Move functions/methods to new files. Every new file must have `package tui` header.
- Do NOT rename, refactor, or modify any function bodies.
- Do NOT change any imports — the new files inherit the package's import namespace.

**Step 3: Compile check**
```bash
cd cli && go build ./...    # catches: duplicate symbols, undefined symbols, missing imports
go vet ./internal/tui/...   # catches: init ordering, unused vars, suspicious constructs
```

**Step 4: Full test suite**
```bash
cd cli && go test ./internal/tui/ -count=1 -v   # all tests, verbose
```
Compare output to Step 1 baseline. Same tests, same pass/fail, same count.

**Step 5: Golden file verification**
```bash
cd cli && go test ./internal/tui/ -run Golden -count=1
git diff cli/internal/tui/testdata/              # MUST be empty — no golden changes
```
If goldens changed, the split accidentally modified behavior. Revert and investigate.

**Step 6: Full CLI build + smoke test**
```bash
make build                  # rebuild the full binary
syllago --help              # basic smoke test — binary starts
syllago tui                 # TUI launches without crash (manual 5-second check)
```

### Why This Is Low-Risk

Go file splits within the same package are the safest refactor possible:
- **Same namespace:** All files in `package tui` share types, functions, and vars. Moving a function between files changes nothing the compiler sees.
- **No import changes:** Unlike Python/JS, there are no per-file imports to update. The package resolves symbols across all its files.
- **No visibility changes:** Go visibility is package-level (exported = capital letter), not file-level. Nothing becomes more or less accessible.
- **Compiler catches mistakes:** Duplicate symbol? Compile error. Missing symbol? Compile error. Wrong type? Compile error. The only thing the compiler *won't* catch is accidentally deleting a function entirely — which the tests cover.

---

## Part 2: Context Architecture

### The Three Layers

```
                    ┌─────────────────────────────────────┐
                    │  tui-builder skill (manual invoke)   │
                    │  Golden rules, layout calculator,    │
                    │  quick-reference checklist           │
                    │  ~150 lines                          │
                    └─────────────────────────────────────┘
                                    │
            ┌───────────────────────┼───────────────────────┐
            │                       │                       │
    ┌───────▼───────┐    ┌─────────▼─────────┐    ┌───────▼───────┐
    │  CLAUDE.md     │    │  Rule files        │    │  Hook          │
    │  (auto-load)   │    │  (path-scoped)     │    │  (enforcement) │
    │  Architecture  │    │  Enforceable       │    │  Warns if      │
    │  + essentials  │    │  patterns +        │    │  skill not     │
    │  ~120 lines    │    │  invariants        │    │  loaded        │
    └────────────────┘    └────────────────────┘    └────────────────┘
```

### What Goes Where

| Concern                                      | Location                       | Why                            |
|----------------------------------------------|--------------------------------|--------------------------------|
| Architecture overview (model tree, file org) | CLAUDE.md                      | Always needed, low churn       |
| Build/test commands                          | CLAUDE.md                      | Always needed                  |
| Color palette + style rules                  | CLAUDE.md                      | Concise, always relevant       |
| Keyboard handling patterns                   | CLAUDE.md                      | Core convention                |
| Message routing priority                     | CLAUDE.md                      | Architecture essential         |
| Testing setup + golden patterns              | CLAUDE.md                      | Always needed for any edit     |
| Layout arithmetic formulas                   | Rule: `tui-layout.md`          | Enforceable, reference-heavy   |
| Elm architecture enforcement                 | Rule: `tui-elm.md`             | Critical invariant AI violates |
| Modal patterns                               | Rule: `tui-modals.md`          | Component-specific patterns    |
| Wizard step machine                          | Rule: `tui-wizard-patterns.md` | Already exists, update path    |
| Items rebuild pattern                        | Rule: `tui-items-rebuild.md`   | Already exists, update path    |
| Golden rules checklist                       | Skill: `tui-builder`           | Deep reference, manual invoke  |
| Layout calculator formulas                   | Skill: `tui-builder`           | Deep reference, manual invoke  |
| Component message contracts                  | Skill: `tui-builder`           | Reference for implementation   |
| Phase log / design history                   | Skill: `tui-builder`           | Historical context, not rules  |
| Gotchas list                                 | Skill: `tui-builder`           | Comprehensive reference        |

### What Gets Deleted

- All 12 `tui_v1`-scoped rule files in `.claude/rules/`
- `cli/internal/tui_v1/CLAUDE.md`
- Duplicate content between CLAUDE.md and skill (after consolidation)

---

## Part 3: Rule Files (New)

All scoped to `cli/internal/tui/**`.

### tui-elm.md — Elm Architecture Enforcement

The single most impactful rule per the research. AI tools trained on standard Go patterns violate these constantly.

```markdown
---
paths:
  - "cli/internal/tui/**"
---

# Elm Architecture Enforcement

## Never Do These

1. **No goroutines** — all async work via `tea.Cmd`. Bubbletea manages goroutines internally.
   ```go
   // WRONG — race condition, silent stale renders
   go func() { m.data = fetchData() }()

   // RIGHT — return a command from Update()
   return m, func() tea.Msg { return dataMsg{fetchData()} }
   ```

2. **No blocking I/O in Update() or View()** — freezes the entire event loop.
   ```go
   // WRONG
   func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
       data, _ := os.ReadFile(path)  // blocks UI

   // RIGHT
   return m, func() tea.Msg {
       data, _ := os.ReadFile(path)
       return fileReadMsg{data}
   }
   ```

3. **No state mutation outside Update()** — model changes must be returned from Update().

4. **No assuming command order** — use `tea.Sequence()` when order matters.

5. **Always propagate Update() to active sub-models** — bubbletea has no auto-routing.
   ```go
   m.child, cmd = m.child.Update(msg)
   cmds = append(cmds, cmd)
   ```

6. **Always handle tea.WindowSizeMsg** — store dimensions, recalculate all dependents.
```

### tui-layout.md — Layout Arithmetic

```markdown
---
paths:
  - "cli/internal/tui/**"
---

# Layout Arithmetic Rules

## Border Subtraction (Always Apply)

Every bordered panel costs 2 chars height (top+bottom) and 2 chars width (left+right).
```go
contentHeight -= 2  // ALWAYS subtract for borders
contentWidth -= 2
```

## Text Truncation Formula

Never rely on terminal auto-wrap. Truncate ALL strings before rendering.
```go
maxTextWidth := panelWidth - 4  // -2 borders, -2 padding
title = truncateString(title, maxTextWidth)
```

## Never Use lipgloss.Width() for Sizing

`Width()` word-wraps (creates multi-line output). `MaxWidth()` truncates.
```go
// WRONG — creates multi-line overflow that breaks height
style.Width(w).Render(content)

// RIGHT — truncates without wrapping
style.MaxWidth(w).Render(content)
```

For bordered panels, use `borderedPanel()` from styles.go which sets both
Width+MaxWidth and Height+MaxHeight.

## Dynamic Dimensions

Never hardcode height/width offsets. Use rendered string dimensions.
```go
// WRONG — breaks when topbar height changes
contentHeight = termHeight - 5

// RIGHT — adapts to actual rendered size
topbarRendered := a.topbar.View()
contentHeight = termHeight - lipgloss.Height(topbarRendered) - helpbarHeight
```

## Parent Owns Child Sizes

Children never calculate their own size. Parent calls SetSize() during WindowSizeMsg.
```

### tui-modals.md — Modal Patterns

```markdown
---
paths:
  - "cli/internal/tui/**"
---

# Modal Patterns

## Standard Modal Structure

- Fixed width: `modalWidth = 56`
- Rounded border in accent color
- Background: `modalBgColor`
- Padding: `Padding(1, 2)`
- Composited via `overlayModal()` — background visible on both sides

## Zone-Safe Borders

Build modal borders manually (`╭─╮│╰─╯`) instead of `lipgloss.Border()`.
Lipgloss dimension styling mangles bubblezone's invisible markers.

## Keyboard Priority

When modal is active, it consumes ALL input except Ctrl+C.
- Tab: cycle focus targets
- Enter: confirm / advance field
- Esc: cancel / go back
- Ctrl+S: save from any field

## Button Rendering

Use `renderButtons(left, right, cursor, contentWidth)` for all two-button footers.
Active: white on accent. Inactive: muted on gray. Buttons pinned to bottom via spacer.

## Click-Away Dismissal

Clicks outside `modal-zone` dismiss. Clicks inside do not propagate to background.
```

### tui-testing.md — Testing Patterns

```markdown
---
paths:
  - "cli/internal/tui/**"
---

# TUI Testing Patterns

## After Any Visual Change

```bash
cd cli && go test ./internal/tui/ -update-golden
git diff internal/tui/testdata/   # review EVERY change
```

Golden files are ground truth. Never update them to match broken code.

## Test at Multiple Sizes

Every visual component must be tested at 60x20, 80x30, and 120x40.

## Golden File Naming

`{component}-{variant}-{width}x{height}.golden`

## Test Helpers (testhelpers_test.go)

- `testApp(t)` — empty catalog, 80x30
- `testAppSize(t, w, h)` — custom dimensions
- `keyRune(r)`, `keyPress(k)`, `pressN(m, key, n)`
- `assertContains(t, view, substr)`, `assertNotContains(t, view, substr)`
- `requireGolden(t, name, snapshot)`, `snapshotApp(t, app)`

## Deterministic Output (testmain_test.go)

Already configured — do not modify without understanding:
- `lipgloss.SetColorProfile(termenv.Ascii)` — no color codes in goldens
- `lipgloss.SetHasDarkBackground(true)` — consistent AdaptiveColor
- Warmup render in init() — prevents AdaptiveColor race condition
```

### Existing Rules to Update

| Rule | Action |
|------|--------|
| `tui-wizard-patterns.md` | Already has no path scope — keep, verify content matches v3 |
| `tui-items-rebuild.md` | Already has no path scope — keep, verify content matches v3 |

---

## Part 4: CLAUDE.md Consolidation

Slim the v3 `cli/internal/tui/CLAUDE.md` to ~120 lines. Remove anything that duplicates rule files or the skill. Focus on architecture essentials that every edit needs.

### Proposed Structure

```
# TUI Component Rules (v3)

## Before You Edit
  - Read styles.go, check keys.go
  - Run golden tests after visual changes
  - Test at multiple sizes (60x20, 80x30, 120x40)

## Architecture
  - Root model is App (app.go)
  - File organization (one model per file, styles in styles.go, keys in keys.go)
  - Message routing priority (global → modal → toast → focused panel)

## File Organization (post-split)
  - app.go / app_update.go / app_view.go
  - install.go / install_update.go / install_view.go
  - One model per file for all other components

## Color Palette (Flexoki)
  - Named color table (primaryColor, accentColor, etc.)
  - Rules: no raw hex, no emojis, check palette before adding

## Keyboard Handling
  - All bindings in keys.go as key.Binding
  - key.Matches(msg, keys.Foo), not msg.String() == "x"

## Navigation
  - Two-tier tabs (groups + sub-tabs)
  - q backs out (only quits from landing page)
  - R refreshes catalog

## Message Passing
  - Sub-models return tea.Cmd producing typed messages
  - App.Update() routes all messages
  - Never send between siblings directly

## Testing
  - Golden files in testdata/
  - After visual changes: go test ./internal/tui/ -update-golden
```

Content removed from CLAUDE.md (now in rules or skill):
- Detailed modal conventions → `tui-modals.md` rule
- Layout arithmetic → `tui-layout.md` rule
- Elm architecture → `tui-elm.md` rule
- Scroll implementation → fold into skill or remove (established pattern)
- Metadata panel details → skill reference
- Edit modal specifics → skill reference
- Help overlay specifics → skill reference

---

## Part 5: Skill Restructure

Slim `tui-builder` from 510 lines to ~150 lines. Focus on three things:

### 1. Golden Rules Checklist (top of file)

Before any layout work, verify:
1. Subtract 2 from height for borders, 2 from width for borders
2. Never rely on auto-wrap — truncate explicitly (`maxTextWidth = panelWidth - 4`)
3. Use `MaxWidth()` not `Width()` — Width word-wraps
4. Use `borderedPanel()` for all bordered panels
5. Parent owns child sizes — call `SetSize()`, never let children calculate
6. All async work via `tea.Cmd` — never goroutines
7. All state changes returned from `Update()` — never mutate elsewhere
8. Always propagate `Update()` to active sub-models
9. Handle `tea.WindowSizeMsg` — store and recalculate

### 2. Component Message Contracts

Quick-reference table of what each component sends/receives:

| Component | Receives | Sends |
|-----------|----------|-------|
| topbar | `tea.WindowSizeMsg`, `tea.KeyMsg`, `tea.MouseMsg` | `tabChangedMsg`, `actionPressedMsg`, `helpToggleMsg` |
| items | `tea.WindowSizeMsg`, `tea.KeyMsg` | `itemSelectedMsg` |
| explorer | `tea.WindowSizeMsg`, `tea.KeyMsg`, content items | (renders only) |
| gallery | `tea.WindowSizeMsg`, `tea.KeyMsg`, `tea.MouseMsg` | `galleryDrillMsg` |
| modal | `tea.KeyMsg`, `tea.MouseMsg` | `editSavedMsg`, `editCancelledMsg` |
| install | `tea.KeyMsg`, step data | `appInstallDoneMsg` |
| toast | `tea.KeyMsg`, `showToastMsg` | `toastDismissedMsg` |

### 3. Gotchas (preserved from current skill)

Keep the full gotchas list — this is high-value, low-duplication content.

### Removed from Skill

- Architecture section (→ CLAUDE.md)
- Navigation details (→ CLAUDE.md)
- Component patterns (→ CLAUDE.md + rules)
- Color palette (→ CLAUDE.md)
- Testing strategy (→ CLAUDE.md + rule)
- Mouse support details (→ CLAUDE.md)
- Phase log (→ archive or delete — historical, not actionable)

---

## Part 6: Hook Design

### Purpose

Enforce that the tui-builder skill is loaded into context before making TUI edits. This addresses the research finding that AI tools need bubbletea invariants front-loaded.

### Implementation

A `PreToolUse` hook on `Edit` and `Write` tools that checks if the target file is in `cli/internal/tui/`:

```jsonc
// In .claude/settings.json or settings.local.json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "command": ".claude/hooks/tui-context-gate.sh \"$TOOL_INPUT\""
      }
    ]
  }
}
```

**Hook behavior:**
- Checks if the file path matches `cli/internal/tui/`
- If yes, checks if the tui-builder skill was invoked in this session (via a temp marker file)
- If skill not loaded, outputs a warning: `"TUI edit detected. Run /tui-builder first to load layout rules and golden checklist."`
- Non-blocking warning (exit 0) — informs but doesn't prevent the edit

**Skill sets marker:**
- When `/tui-builder` runs, it touches a temp file (e.g., `/tmp/syllago-tui-builder-$$`)
- Hook checks for this file's existence
- File is per-session (PID-scoped or session-ID-scoped)

### Alternative: Simpler Approach

If session-scoped markers are fragile, the hook could simply always warn on first TUI edit and then set the marker itself (one warning per session). This is less precise but simpler.

---

## Part 7: Improvement Loop

The skill and rule files must evolve with the codebase. Every TUI bug fix, new component, or pattern discovery is a potential update to the context architecture. This uses a **smart gate** — lightweight nudges during iteration, hard enforcement at commit time.

### Hook 1: Test-Pass Reminder (Non-Blocking)

`PostToolUse` hook on `Bash`. Fires after TUI tests pass when TUI files were edited.

```jsonc
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "command": ".claude/hooks/tui-pattern-nudge.sh \"$TOOL_INPUT\" \"$TOOL_RESULT\""
      }
    ]
  }
}
```

**Hook logic (`tui-pattern-nudge.sh`):**
1. Check: does the command contain `go test ./internal/tui/`?
2. Check: did it exit 0 (tests passed)?
3. Check: were any `cli/internal/tui/*.go` files modified? (via `git diff --name-only`)
4. If all three: emit a non-blocking reminder (exit 0):

```
TUI tests passed after code changes. If you established a new pattern,
fixed a bug, or discovered a gotcha:
  - Update .claude/skills/tui-builder/SKILL.md (gotchas, message contracts)
  - Update .claude/rules/tui-*.md (enforceable invariants)
  - Update cli/internal/tui/CLAUDE.md (architecture changes)
```

5. If no TUI files changed, or tests didn't pass: silent (no output).

**Why non-blocking:** Tests pass many times during iteration. A blocking gate here would interrupt flow. The nudge builds the habit; the commit gate enforces it.

### Hook 2: Commit Gate (Blocking)

`PreToolUse` hook on `Bash`. Fires when committing TUI changes. Blocks until docs are reviewed.

```jsonc
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "command": ".claude/hooks/tui-docs-gate.sh \"$TOOL_INPUT\""
      }
    ]
  }
}
```

**Hook logic (`tui-docs-gate.sh`):**
1. Check: does the command start with `git commit`?
2. Check: does `git diff --cached --name-only` include any `cli/internal/tui/*.go` files?
3. If both: check if the staged changes ALSO include at least one of:
   - `.claude/skills/tui-builder/SKILL.md`
   - `.claude/rules/tui-*.md`
   - `cli/internal/tui/CLAUDE.md`
4. If TUI code is staged but no docs are staged: **block** (exit 2) with message:

```
BLOCKED: TUI code changes detected but no doc updates staged.

If this commit introduces or changes a pattern, update:
  - .claude/skills/tui-builder/SKILL.md (gotchas, contracts)
  - .claude/rules/tui-*.md (enforceable invariants)
  - cli/internal/tui/CLAUDE.md (architecture)

If no doc update is needed (e.g., pure bug fix with no new pattern),
stage a no-op touch to SKILL.md or add --no-doc-update to the commit
message to bypass this gate.
```

5. If docs ARE staged alongside TUI code, or if no TUI code is staged: pass (exit 0).

**Bypass mechanism:** Including `--no-doc-update` in the commit message (detected via `$TOOL_INPUT` grep) skips the gate. This handles pure refactors or mechanical changes that don't establish new patterns. The bypass is explicit and auditable — you have to say why you're skipping.

### What Gets Updated

| Change Type | Update Target | Example |
|-------------|--------------|---------|
| New gotcha discovered | Skill: gotchas section | "AdaptiveColor mutates renderer state" |
| New message type added | Skill: message contracts table | `confirmInstallMsg` added |
| Layout bug fixed | Rule: `tui-layout.md` | "Never set Height() on bordered styles" |
| New modal pattern | Rule: `tui-modals.md` | Confirm modal with y/n shortcuts |
| Architecture change | CLAUDE.md | New component, file split, focus zone added |
| New test helper | Rule: `tui-testing.md` or CLAUDE.md | `testAppWithRegistry(t)` added |

### Why This Works

The improvement loop creates a **ratchet effect**: every TUI session leaves the context architecture slightly better than it found it. Over time:
- Gotchas accumulate from real bugs (not hypotheticals)
- Message contracts stay current with the actual codebase
- Layout rules encode real failure modes, not theoretical ones
- The AI gets measurably better at TUI work session over session

Without the loop, the context files rot — they describe the codebase as it was when they were written, not as it is now. The commit gate ensures the delta is captured at the natural checkpoint.

---

## Execution Order

| Phase | Work | Dependencies |
|-------|------|-------------|
| 1 | **File splitting** — split app.go and install.go | None |
| 2 | **Delete v1 artifacts** — remove 12 rule files + tui_v1/CLAUDE.md | None |
| 3 | **Write new rules** — tui-elm.md, tui-layout.md, tui-modals.md, tui-testing.md | Phase 1 (file references) |
| 4 | **Consolidate CLAUDE.md** — slim to ~120 lines | Phase 2, 3 (no duplication) |
| 5 | **Restructure skill** — slim to ~150 lines | Phase 4 (no duplication) |
| 6 | **Add hooks** — context gate + pattern nudge + commit gate | Phase 5 (skill exists) |
| 7 | **Verify** — run full test suite, build, manual TUI check | All phases |

Phases 1 and 2 can run in parallel. Phases 3-5 are sequential (each builds on the previous). Phase 6 is independent once the skill exists.

Phase 6 now includes three hooks:
- `tui-context-gate.sh` — PreToolUse on Edit/Write: warns if skill not loaded
- `tui-pattern-nudge.sh` — PostToolUse on Bash: non-blocking reminder after tests pass
- `tui-docs-gate.sh` — PreToolUse on Bash: blocks commit if TUI code staged without doc updates

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| File split breaks imports/references | Low | Medium | Run `make test` after each split |
| New rules are too prescriptive for edge cases | Medium | Low | Include "why" for each rule so AI can judge exceptions |
| Skill is too slim, loses useful context | Low | Medium | Gotchas list preserved; phase log archived, not deleted |
| Hook is annoying (warns too often) | Medium | Low | Non-blocking warning, easy to disable |
| Commit gate blocks legitimate no-doc-needed commits | Medium | Low | `--no-doc-update` bypass mechanism |
| Improvement loop creates churn in doc files | Low | Low | Only update when patterns change, not on every commit |
| Deduplication removes something needed | Low | Medium | Git history preserves everything; review each deletion |

---

## Success Criteria

After implementation:
1. Editing any file in `cli/internal/tui/` loads: CLAUDE.md (essentials) + all 4 rule files (invariants)
2. Running `/tui-builder` loads: golden rules checklist + message contracts + gotchas
3. No file in `cli/internal/tui/` exceeds 800 lines
4. `make test` passes with no golden file changes (file splitting is behavior-preserving)
5. Hook fires a warning on first TUI edit if skill wasn't loaded
6. Zero content references `tui_v1` anywhere in `.claude/`
7. After TUI tests pass with edits, a non-blocking nudge reminds to update docs
8. Committing TUI code without staging doc updates is blocked (with explicit bypass)
9. The skill and rules reflect the actual current codebase, not a stale snapshot
