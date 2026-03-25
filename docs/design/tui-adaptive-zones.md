# TUI Adaptive Zones Design

**Decision:** Explorer-style base layout (items list left, detail panel right) with adaptive detail zones per content type. The outer shell and navigation model are always the same — only the detail panel internals change.

## Global Style Decisions

### Inline Section Titles

All section/area titles use inline horizontal rules. The title text sits inside the rule line:

```
──Skills (5)──────────────────────────    ──Contents (7)────────────────
──Preview: SKILL.md───────────────────    ──Files──────────────────────
──Loadouts (3)────────────────────────    ──Recent Activity──────────────
```

Format: `──{Title}──{fill remaining width with ─}`
- Title text is rendered in the section's semantic color (mint for content, purple for collections, muted for neutral)
- The horizontal rules use `mutedColor`
- Applies everywhere: items list headers, content zone pane headers, card grid headers, contents sidebar headers, metadata bar separators

### Action Buttons

- Top bar uses `+ Add` (not "Import") and `* New` (not "Create")
- `Add` = bring in existing content (from registry, provider, or file)
- `New` = author/scaffold new content from scratch within syllago

### No Install Status Badges

Install status (`[ok]`, `[--]`) is removed from the items list and metadata bar. Install state is visible in the detail/content zone when you inspect an item (the Install tab or provider checkboxes show which providers have it installed). This declutters the items list and metadata bar.

### Help Bar

The help bar at the bottom spans full width with `syllago vX.X.X` pinned to the right:

```
──────────────────────────────────────────────────────────────────────────────────
j/k navigate * h/l pane * / search * i install * ? help                   syllago v1.2.0
```

- Context-sensitive hints on the left (changes per screen/focus state)
- `syllago vX.X.X` pinned right (always visible)
- When hints are too long for the available width, lower-priority hints are dropped (same behavior as current TUI's `helpText()` pattern)
- The `?` help overlay shows the FULL keyboard reference for the current screen

### Mouse Parity

**Every interactive element supports both keyboard AND mouse — no exceptions.**

| Element | Keyboard | Mouse |
|---|---|---|
| Group tabs | 1/2/3 to switch group | Click group tab |
| Sub-tabs | h/l to cycle within group | Click sub-tab |
| Action buttons | a = Add, n = Create | Click button |
| Items list | j/k navigate, Enter to select | Click item to select, scroll wheel to scroll |
| Card grid | Arrows to navigate, Enter to drill in | Click card to select+drill, scroll wheel to scroll grid |
| Content zone panes | h/l switch panes, j/k scroll | Click pane to focus, scroll wheel in focused pane |
| Contents sidebar | j/k scroll, Enter on item to drill | Click item to drill, scroll wheel to scroll |
| Metadata bar actions | Hotkey (i/u/c/p) | Click button |
| Modal buttons | Left/Right switch, Enter confirm | Click button |
| Modal body | Up/Down/PgUp/PgDn scroll | Scroll wheel |
| Toast | c to copy, Esc to dismiss | Click "copy" label, click-away to dismiss |
| Help bar | (read-only) | (read-only) |
| Search | / to activate, type to filter, Esc to cancel | Click search area to activate |

## Top Navigation Bar — Two-Tier Tabs

A bordered frame with two tiers of navigation. No dropdowns (those are a GUI pattern that doesn't work well in terminals).

### Layout

```
╭──syllago─────────────────────────────────────────────────────────────────────╮
│               [1] Collections      [2] Content      [3] Config               │
├──────────────────────────────────────────────────────────────────────────────┤
│   Library     Registries     Loadouts              [a] Add      [n] Create   │
╰──────────────────────────────────────────────────────────────────────────────╯
```

- **Top border:** `╭──syllago──╮` with colored logo inline (mint `syl` + viola `lago`)
- **Row 1 (groups):** Centered, button-styled with backgrounds. Active = bold text on cyan BG. Inactive = muted text on gray BG.
- **Separator:** `├────────┤` connecting to left/right border edges
- **Row 2 (sub-tabs + actions):** Sub-tabs left-aligned (bold cyan active, faint inactive). Action buttons right-aligned with background styling.
- **Bottom border:** `╰────────╯` rounded corners
- **Height:** Always 5 lines

### Groups

| Group | Hotkey | Sub-Tabs | Actions |
|-------|--------|----------|---------|
| Collections | `[1]` | Library, Registries, Loadouts | [a] Add, [n] Create |
| Content | `[2]` | Skills, Agents, MCP, Rules, Hooks, Commands | [a] Add, [n] Create |
| Config | `[3]` | Settings, Sandbox | (none) |

**Default on launch:** Collections > Library (everything at a glance).

### Switching to Content > Hooks at 80 cols
```
╭──syllago─────────────────────────────────────────────────────────────────────╮
│               [1] Collections      [2] Content      [3] Config               │
├──────────────────────────────────────────────────────────────────────────────┤
│   Skills     Agents     MCP     Rules     Hooks     Commands   [a] Add [n] Create│
╰──────────────────────────────────────────────────────────────────────────────╯
```

### Keyboard Access

- `1`/`2`/`3` switch groups (resets sub-tab to first item)
- `h`/`l` (or left/right arrows) cycle sub-tabs within the active group (wraps)
- `a` triggers Add action, `n` triggers Create action (context-sensitive to current group+tab)
- All interactive elements also support mouse clicks

### Color Scheme — Flexoki

All theme colors use the [Flexoki](https://stephango.com/flexoki) palette. Logo uses separate syllago brand colors (mint + viola). See `styles.go` for the complete mapping.

## Base Layout (Constant Shell)

The shell has **four fixed zones** that never change position:

1. **Top Bar** — two-tier tabs + action buttons (5 rows including border)
2. **Metadata Bar** — selected item info (3 rows, spans full width above the content split)
3. **Main Area** — Items list (left) + Adaptive Content Zone (right)
4. **Help Bar** — context-sensitive hints (1 row)

The metadata bar is part of the shell, not part of the detail zone. It renders the same way regardless of content type — only the field *values* change.

### Shell at 120x40 (item selected)
```
╭──syllago──────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│                         [1] Collections      [2] Content      [3] Config                                              │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│   Skills     Agents     MCP     Rules     Hooks     Commands                                [a] Add      [n] Create   │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ alpha-skill                                                      [i] Install  [u] Uninstall  [c] Copy  [p] Share     │
│ Type: Skills    Source: team-rules (registry)    Providers: Claude Code, Gemini CLI    Files: 4    Installed: CC [ok] │
│ Description: A helpful skill that extends AI tool capabilities                                                       │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
┌─────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────┐
│ [Items List]            │ [Adaptive Content Zone]                                                                     │
│                         │                                                                                             │
│ Always the same:        │ Changes per content type:                                                                   │
│ - Name column           │ - Skills: Files tree + Preview split                                                        │
│ - Provider badges       │ - Hooks: Files tree + Preview split (+ Compat tab)                                          │
│ - Source indicator       │ - Agents/Rules/MCP/Commands: Full-width preview                                             │
│ - Install status        │                                                                                             │
│                         │                                                                                             │
├─────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────┤
│ [Help Bar — always present, hints change per context]                                                       syllago  │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### Shell at 80x30 (item selected)
```
╭──syllago─────────────────────────────────────────────────────────────────────╮
│               [1] Collections      [2] Content      [3] Config               │
├──────────────────────────────────────────────────────────────────────────────┤
│   Skills     Agents     MCP     Rules     Hooks     Commands   [a] Add [n] Create│
╰──────────────────────────────────────────────────────────────────────────────╯
┌──────────────────────────────────────────────────────────────────────────────────┐
│ alpha-skill                                         [i] Install  [c] Copy      │
│ Skills * team-rules * CC,GC * 4 files                     │
│ A helpful skill that extends AI tool capabilities                               │
├──────────────────────────────────────────────────────────────────────────────────┤
┌──────────────────┬───────────────────────────────────────────────────────────────┐
│ [Items List]     │ [Adaptive Content Zone]                                       │
│                  │                                                               │
├──────────────────┴───────────────────────────────────────────────────────────────┤
│ j/k nav * h/l pane * / search * ? help                               syllago v1.2.0│
╰──────────────────────────────────────────────────────────────────────────────────╯
```

### Metadata Bar Fields (Standard Across All Types)

The metadata bar always shows these fields in a consistent layout:

| Row | Content |
|-----|---------|
| **Row 1** | Item name (left, bold) + Action buttons (right) |
| **Row 2** | Type + Source + Providers + type-specific fields (Files count, Installed status) |
| **Row 3** | Description (if available) or type-specific summary line |

**Type-specific fields on Row 2:**

| Content Type | Extra Row 2 Fields | Row 3 Content |
|---|---|---|
| Skills | Files: {n} | Description from SKILL.md frontmatter |
| Agents | Permission: {mode} | Description from agent frontmatter |
| MCP Servers | Transport: {stdio/http} | Command: {command + args summary} |
| Rules | Scope: {globs} | Description from frontmatter (if any) |
| Hooks | Events: {event names} | Matcher: {matcher} + Handler: {type} |
| Commands | Effort: {level} | Description from frontmatter |

**When no item is selected** (browsing list), the metadata bar shows a summary for the content type:
```
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Skills (5 items)                                                                          + Add     * New         │
│ 3 from registries, 2 from library    Providers: CC (5), Gemini (3), Cursor (1)                                       │
│ Reusable skill definitions that extend AI tool capabilities                                                          │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

### Items List (Left Pane — Constant)

Provider badges and install status are NOT shown in the items list — providers live in the metadata bar, install status is in the detail view. The list shows name and source only. Uses inline title style.

```
At 120+ cols:
──Skills (5)──────────────────────
   Name               Source
 > alpha-skill        team-rules
   beta-skill         team-rules
   gamma-skill        library
   delta-skill        my-registry

At 80 cols:
──Skills (5)──────────
   Name          Source
 > alpha-skill   team-r
   beta-skill    team-r
   gamma-skill   librar

At 60 cols:
──Skills (5)──────
 > alpha-skill
   beta-skill
   gamma-skill
```

### Action Buttons (In Metadata Bar — Always Present)

Actions appear in the metadata bar's Row 1, right-aligned. This keeps them visible at all times without consuming content area space.

```
At 120+ cols: [i] Install  [u] Uninstall  [c] Copy  [p] Share
At 80 cols:   [i] Install  [c] Copy  (overflow in ? help)
```

Actions vary slightly per type (hooks add `[v] Verify`, loadouts add `[a] Apply`), but the position and interaction model are constant.

---

## Adaptive Content Zones by Content Type

Since the metadata bar is now part of the shell (always at the top, full-width), the per-type wireframes only show what's **inside the content zone** (below the metadata bar, beside the items list). The metadata bar + actions are rendered by the shell.

### Skills — Files + Preview Split

Multi-file. Content zone shows file tree on left, file preview on right.

```
120x40:
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL    Content: Skills ▾    Collection: -- ▾    Config ▾                                        + Add     * New   │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ alpha-skill                                                      [i] Install  [u] Uninstall  [c] Copy  [p] Share     │
│ Skills * team-rules (registry) * CC, Gemini CLI * 4 files                                       │
│ A helpful skill that extends AI tool capabilities                                                                    │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
┌─────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────┐
│ Skills (5)              │ Files                  │ Preview: SKILL.md                                                  │
│                         │ ────────────────────── │ ────────────────────────────────────────────────────────────────── │
│ > alpha-skill  team-r   │ > SKILL.md             │  1  ---                                                           │
│   beta-skill   team-r   │   helpers.md           │  2  title: Alpha Skill                                            │
│   gamma-skill  librar   │   utils/               │  3  description: A helpful skill that extends AI tool             │
│   delta-skill  my-reg   │     tool.md            │  4  providers: [claude-code, gemini-cli]                           │
│   epsilon-skill team-r  │     parser.md          │  5  ---                                                           │
│                         │                        │  6                                                                │
│                         │                        │  7  # Alpha Skill                                                │
│                         │                        │  8                                                                │
│                         │                        │  9  This skill extends AI tool capabilities by providing          │
│                         │                        │ 10  structured workflows for common development tasks.            │
│                         │                        │ 11                                                                │
│                         │                        │ 12  ## Usage                                                      │
│                         │                        │ 13                                                                │
│                         │                        │ 14  Invoke with `/alpha` in your AI coding tool.                  │
│                         │                        │ 15                                                                │
│                         │                        │ 16  ## Parameters                                                 │
│                         │                        │ 17                                                                │
│                         │                        │ 18  - `--verbose`: Enable detailed output                         │
│                         │                        │ 19  - `--format`: Output format (json, yaml, text)                │
│                         │                        │ 20                                                                │
│                         │                        │                                                                   │
│                         │                        │                                                                   │
│                         │                        │                                                                   │
│                         │                        │                                                                   │
├─────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────┤
│ j/k navigate * h/l switch pane * / search * i install * ? help                                            syllago v1.2.0│
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

80x30:
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Content: Skills ▾  Collection: -- ▾  Config ▾               + Add * New │
╰──────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────────────────────────────────────────────────────────────────────┐
│ alpha-skill                                         [i] Install  [c] Copy      │
│ Skills * team-rules * CC,GC * 4 files                                │
│ A helpful skill that extends AI tool capabilities                               │
├──────────────────────────────────────────────────────────────────────────────────┤
┌──────────────────┬───────────────────────────────────────────────────────────────┐
│ Skills (5)       │ Files            │ Preview: SKILL.md                          │
│                  │ ──────────────── │ ──────────────────────────────────────── │
│ > alpha-skill    │ > SKILL.md       │  1  ---                                   │
│   beta-skill     │   helpers.md     │  2  title: Alpha Skill                    │
│   gamma-skill    │   utils/         │  3  description: A helpful skill          │
│   delta-skill    │     tool.md      │  4  providers: [claude-code]              │
│   epsilon-skill  │     parser.md    │  5  ---                                   │
│                  │                  │  6                                         │
│                  │                  │  7  # Alpha Skill                         │
│                  │                  │  8                                         │
│                  │                  │  9  This skill extends AI tool            │
│                  │                  │ 10  capabilities by providing...           │
│                  │                  │ 11                                         │
│                  │                  │ 12  ## Usage                               │
├──────────────────┴───────────────────────────────────────────────────────────────┤
│ j/k nav * h/l pane * / search * i install * ? help                   syllago v1.2.0│
╰──────────────────────────────────────────────────────────────────────────────────╯
```

**Content zone:** Files|Preview split
**Keyboard:** h/l switches between Files and Preview sub-panes

---

### Hooks — Files + Preview Split (with Compat tab)

Hooks can be multi-file (hook definition + script files). Same split as Skills, with an additional **Compat tab** to switch the left sub-pane between file tree and compatibility matrix.

```
120x40:
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL    Content: Hooks ▾    Collection: -- ▾    Config ▾                                         + Add     * New   │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ pre-commit-lint                                          [i] Install  [u] Uninstall  [v] Verify  [c] Copy  [p] Share │
│ Hooks * team-rules (registry) * CC, Gemini CLI, Copilot * 3 files                              │
│ Events: before_tool_execute * Matcher: shell * Handler: command                                                      │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
┌─────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────┐
│ Hooks (4)               │ Files | Compat             │ Preview: hook.json                                             │
│                         │ ──────────────────────────── │ ──────────────────────────────────────────────────────────── │
│ > pre-commit-lint        │ > hook.json                │  1  {                                                         │
│   security-scan          │   scripts/                 │  2    "hooks": {                                              │
│   test-runner            │     lint-check.sh          │  3      "before_tool_execute": [                              │
│   format-check           │     lint-fix.sh            │  4        {                                                   │
│                         │                            │  5          "matcher": "shell",                               │
│                         │                            │  6          "handler": {                                      │
│                         │                            │  7            "type": "command",                              │
│                         │                            │  8            "command": "./scripts/lint-check.sh",           │
│                         │                            │  9            "timeout": 30000                                │
│                         │                            │ 10          }                                                 │
│                         │                            │ 11        }                                                   │
│                         │                            │ 12      ]                                                     │
│                         │                            │ 13    }                                                       │
│                         │                            │ 14  }                                                         │
│                         │                            │ 15                                                            │
│                         │                            │                                                               │
│                         │                            │                                                               │
│                         │                            │                                                               │
│                         │                            │                                                               │
│                         │                            │                                                               │
│                         │                            │                                                               │
│                         │                            │                                                               │
│                         │                            │                                                               │
├─────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────┤
│ j/k navigate * h/l switch pane * tab Files/Compat * / search * v verify * ? help                          syllago v1.2.0│
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

Same layout, Compat tab active (tab key switches):
┌─────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────┐
│ Hooks (4)               │ Files | Compat             │ Preview: hook.json                                             │
│                         │ ──────────────────────────── │ ──────────────────────────────────────────────────────────── │
│ > pre-commit-lint        │                            │  1  {                                                         │
│   security-scan          │ Provider       Status      │  2    "hooks": {                                              │
│   test-runner            │ ──────────── ──────────    │  3      "before_tool_execute": [                              │
│   format-check           │ Claude Code  [ok] native   │  ...                                                          │
│                         │ Gemini CLI   [~~] partial   │                                                               │
│                         │ Copilot CLI  [ok] native   │                                                               │
│                         │ Kiro         [!!] limited  │                                                               │
│                         │ Cursor       [--] none     │                                                               │
│                         │ Windsurf     [--] none     │                                                               │
│                         │                            │                                                               │
│                         │ [ok] Full  [~~] Partial    │                                                               │
│                         │ [!!] Limited  [--] None    │                                                               │
│                         │                            │                                                               │
│                         │ Notes:                     │                                                               │
│                         │ Gemini: event mapped to    │                                                               │
│                         │ before_tool_call, matcher  │                                                               │
│                         │ syntax differs.            │                                                               │
```

**Content zone:** Files|Preview split (same as Skills) with a **tab toggle** on the left sub-pane header
**Left sub-pane tabs:** `Files` (file tree) | `Compat` (compatibility matrix)
**Keyboard:** h/l switches sub-panes, Tab toggles between Files/Compat views in the left sub-pane
**Why tabs:** The compat matrix is reference info you check occasionally, not something you need alongside the file tree simultaneously. Tabs avoid cramming three zones into one panel.

---

### Agents, Rules, Commands — Full-Width Preview

Single-file types. No file tree needed — the content zone is a full-width scrollable preview.

```
120x40 (Agents example):
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL    Content: Agents ▾    Collection: -- ▾    Config ▾                                        + Add     * New   │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ code-reviewer                                                    [i] Install  [u] Uninstall  [c] Copy  [p] Share     │
│ Agents * team-rules (registry) * CC, Cursor * 1 file * Permission: acceptEdits                  │
│ Reviews code for security, performance, and maintainability                                                          │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
┌─────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────┐
│ Agents (4)              │ Preview                                                                                     │
│                         │ ──────────────────────────────────────────────────────────────────────────────────────────── │
│ > code-reviewer          │  1  ---                                                                                    │
│   docs-writer            │  2  name: Code Reviewer                                                                   │
│   security-auditor       │  3  description: Reviews code for security, performance,                                   │
│   api-expert             │  4  and maintainability                                                                    │
│                         │  5  permissionMode: acceptEdits                                                            │
│                         │  6  ---                                                                                    │
│                         │  7                                                                                         │
│                         │  8  You are an expert code reviewer. When reviewing code, focus on:                        │
│                         │  9                                                                                         │
│                         │ 10  ## Security                                                                            │
│                         │ 11  - Check for injection vulnerabilities (SQL, XSS, command)                              │
│                         │ 12  - Verify input validation at system boundaries                                         │
│                         │ 13  - Look for exposed secrets or credentials                                              │
│                         │ 14                                                                                         │
│                         │ 15  ## Performance                                                                         │
│                         │ 16  - Identify N+1 queries and unnecessary allocations                                     │
│                         │ 17  - Check for unbounded operations (loops, recursion)                                    │
│                         │ 18                                                                                         │
│                         │ 19  ## Maintainability                                                                     │
│                         │ 20  - Verify naming conventions and code organization                                      │
│                         │ 21  - Check test coverage for new code paths                                               │
│                         │ 22  - Ensure error handling is consistent                                                  │
│                         │ 23                                                                                         │
│                         │ 24  Always provide specific, actionable feedback with code examples                        │
│                         │ 25  when suggesting improvements.                                                          │
├─────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────┤
│ j/k navigate * / search * i install * ? help                                                              syllago v1.2.0│
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

**Content zone:** Full-width preview (no sub-panes)
**Applies to:** Agents, Rules, Commands (all single-file)
**Keyboard:** j/k scrolls preview when detail pane is focused (no h/l sub-pane switching)

---

### MCP Servers — Full-Width JSON Preview

Same full-width preview pattern. The metadata bar surfaces MCP-specific fields (transport, command, env vars) so the JSON preview is supplementary, not the only way to see key info.

```
120x40:
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL    Content: MCP ▾    Collection: -- ▾    Config ▾                                           + Add     * New   │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ postgres-server                                          [i] Install  [u] Uninstall  [e] Edit env  [c] Copy  [p] Shr │
│ MCP * team-rules (registry) * CC, Gemini, Cursor * Transport: stdio                             │
│ Command: npx -y @modelcontextprotocol/server-postgres * Env: DATABASE_URL                                            │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
┌─────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────┐
│ MCP Servers (3)         │ Preview                                                                                     │
│                         │ ──────────────────────────────────────────────────────────────────────────────────────────── │
│ > postgres-server        │  1  {                                                                                      │
│   github-mcp             │  2    "mcpServers": {                                                                      │
│   slack-mcp              │  3      "postgres": {                                                                      │
│                         │  4        "command": "npx",                                                                │
│                         │  5        "args": [                                                                        │
│                         │  6          "-y",                                                                          │
│                         │  7          "@modelcontextprotocol/server-postgres"                                        │
│                         │  8        ],                                                                               │
│                         │  9        "env": {                                                                         │
│                         │ 10          "DATABASE_URL": "postgresql://localhost:5432/mydb"                              │
│                         │ 11        }                                                                                │
│                         │ 12      }                                                                                  │
│                         │ 13    }                                                                                    │
│                         │ 14  }                                                                                      │
│                         │ 15                                                                                         │
│                         │                                                                                            │
│                         │                                                                                            │
│                         │                                                                                            │
│                         │                                                                                            │
│                         │                                                                                            │
│                         │                                                                                            │
│                         │                                                                                            │
│                         │                                                                                            │
├─────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────┤
│ j/k navigate * / search * i install * e edit env * ? help                                                 syllago v1.2.0│
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

**Content zone:** Full-width JSON preview
**Metadata bar extras:** Transport type, full command string, env var names
**Action extras:** `[e] Edit env` for managing environment variables

---

## Content Zone Summary

| Content Type | Content Zone Pattern | Left Sub-pane | Right Sub-pane |
|---|---|---|---|
| **Skills** | Split | Files tree | Preview |
| **Hooks** | Split + tab | Files tree / Compat matrix (tabbed) | Preview |
| **Agents** | Full-width | -- | Preview |
| **Rules** | Full-width | -- | Preview |
| **MCP Servers** | Full-width | -- | Preview |
| **Commands** | Full-width | -- | Preview |

Only **2 content zone patterns** to implement:
1. **Split** — used by Skills and Hooks (Hooks add a tab toggle)
2. **Full-width** — used by Agents, Rules, MCP, Commands

## Navigation Model (Constant Across All Types)

| Key | Action | Always available? |
|---|---|---|
| j/k (up/down) | Navigate items list (left pane focused) or scroll preview (right pane focused) | Yes |
| h/l (left/right) | Switch focus between items list and content zone sub-panes | Only when content zone has sub-panes (Skills, Hooks) |
| Enter | Select item / drill into detail | Yes |
| Esc | Back / close dropdown / deselect | Yes |
| 1/2/3 | Open Content / Collection / Config dropdown | Yes |
| Tab | Toggle sub-pane view (Files/Compat for hooks) | Only for Hooks |
| / | Search/filter items | Yes |
| ? | Help overlay | Yes |
| i | Install | Yes (item selected) |
| u | Uninstall | Yes (item selected) |
| c | Copy | Yes (item selected) |
| p | Share | Yes (item selected) |

**Key insight:** h/l pane switching only activates when the content zone has split panes (Skills, Hooks). For single-pane zones (Agents, Rules, Commands, MCP), h/l switches between items list and content zone — no sub-pane navigation needed.

## Collection Views — "Gallery Grid" Layout

When a **Collection** is active (Library, Registries, Loadouts), the entire content area switches from the Explorer layout to a **Gallery Grid** layout. This is a fundamentally different visual pattern because collections are *containers of things*, not individual items.

Inspired by Gemini's "Shell C: Gallery Grid" concept.

### Gallery Grid Structure

The content area splits into:
- **Left (70-75%):** Card grid — visual cards arranged in a responsive grid
- **Right (25-30%):** Contents sidebar — shows what's inside the selected card

The metadata bar still spans full width and shows info about the selected card.

### Loadouts — Gallery Grid

```
120x40:
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL    Content: -- ▾    Collection: Loadouts ▾    Config ▾                                      + Add     * New   │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Python-Web                                                            [a] Apply  [e] Edit  [d] Delete  [c] Copy      │
│ Loadout * local * 7 items * Target: Claude Code * Status: not applied                                                │
│ Full-stack Python development environment with linting and security                                                  │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
┌────────────────────────────────────────────────────────────────────────────────────┬───────────────────────────────────┐
│ Loadouts (3)                                                                      │ Contents (7)                      │
│                                                                                   │                                   │
│ ╭──────────────────────────╮  ╭──────────────────────────╮  ╭─────────────────── │ Skills                            │
│ │ > Python-Web             │  │   React-Frontend         │  │   Security-Audit   │   Refactor-Python                 │
│ │   ────────────────────── │  │   ────────────────────── │  │   ──────────────── │   Py-Doc-Gen                      │
│ │   4 Skills               │  │   6 Skills               │  │   2 Agents         │   Django-Patterns                 │
│ │   2 Rules                │  │   1 Agent                │  │   5 Rules          │   Test-Generator                  │
│ │   1 Agent                │  │   2 MCP Servers          │  │   3 Hooks          │                                   │
│ │   Target: Claude Code    │  │   Target: Cursor         │  │   Target: CC       │ Rules                             │
│ ╰──────────────────────────╯  ╰──────────────────────────╯  ╰─────────────────── │   Strict-Types                    │
│                                                                                   │   PEP8-Lint                       │
│ ╭──────────────────────────╮                                                      │                                   │
│ │   Go-Backend             │                                                      │ Agents                            │
│ │   ────────────────────── │                                                      │   Code-Reviewer                   │
│ │   3 Skills               │                                                      │                                   │
│ │   4 Rules                │                                                      │                                   │
│ │   2 Hooks                │                                                      │                                   │
│ │   Target: Gemini CLI     │                                                      │                                   │
│ ╰──────────────────────────╯                                                      │                                   │
│                                                                                   │                                   │
│                                                                                   │                                   │
│                                                                                   │                                   │
│                                                                                   │                                   │
│                                                                                   │                                   │
│                                                                                   │                                   │
├────────────────────────────────────────────────────────────────────────────────────┴───────────────────────────────────┤
│ arrows navigate grid * enter select * a apply * e edit * tab grid/contents * ? help                       syllago v1.2.0│
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

80x30:
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Content: -- ▾  Collection: Loadouts ▾  Config ▾             + Add * New │
╰──────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────────────────────────────────────────────────────────────────────┐
│ Python-Web                                            [a] Apply  [e] Edit      │
│ Loadout * local * 7 items * Target: CC * Not applied                            │
│ Full-stack Python dev environment                                               │
├──────────────────────────────────────────────────────────────────────────────────┤
┌──────────────────────────────────────────────────────┬───────────────────────────┐
│ Loadouts (3)                                         │ Contents (7)              │
│                                                      │                           │
│ ╭────────────────────╮  ╭────────────────────╮       │ Skills                    │
│ │ > Python-Web       │  │   React-Frontend   │       │   Refactor-Python         │
│ │   4 Skills, 2 Rul  │  │   6 Skills, 1 Agt  │       │   Py-Doc-Gen              │
│ │   Target: CC       │  │   Target: Cursor   │       │   Django-Patterns         │
│ ╰────────────────────╯  ╰────────────────────╯       │   Test-Generator          │
│ ╭────────────────────╮  ╭────────────────────╮       │ Rules                     │
│ │   Security-Audit   │  │   Go-Backend       │       │   Strict-Types            │
│ │   2 Agts, 5 Rules  │  │   3 Skills, 4 Rul  │       │   PEP8-Lint               │
│ │   Target: CC       │  │   Target: GC       │       │ Agents                    │
│ ╰────────────────────╯  ╰────────────────────╯       │   Code-Reviewer           │
│                                                      │                           │
│                                                      │                           │
├──────────────────────────────────────────────────────┴───────────────────────────┤
│ arrows grid * enter select * a apply * tab grid/contents * ? help    syllago v1.2.0│
╰──────────────────────────────────────────────────────────────────────────────────╯
```

**Card contents:** Name (bold), separator, item counts by type, target provider
**Selection:** Arrow keys navigate the grid (up/down/left/right), selected card has accent border
**Contents sidebar:** Updates live as you navigate cards — grouped by content type
**Actions:** `[a] Apply` (try/keep), `[e] Edit`, `[d] Delete`

### Registries — Gallery Grid

Same Gallery Grid layout, but cards show registry info and contents shows available items.

```
120x40:
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL    Content: -- ▾    Collection: Registries ▾    Config ▾                                    + Add     * New   │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ team-rules                                                              [s] Sync  [r] Remove  [b] Browse  [c] Copy   │
│ Registry * github.com/acme/syllago-rules * 12 items * Last sync: 2 hours ago * Status: up to date                    │
│ Team coding standards and shared AI configurations                                                                   │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
┌────────────────────────────────────────────────────────────────────────────────────┬───────────────────────────────────┐
│ Registries (2)                                                                    │ Items (12)                        │
│                                                                                   │                                   │
│ ╭──────────────────────────────────────╮  ╭──────────────────────────────────────╮│ Skills (4)                        │
│ │ > team-rules                         │  │   community-pack                     ││   Refactor-Python                 │
│ │   github.com/acme/syllago-rules      │  │   github.com/syllago/community      ││   Py-Doc-Gen                      │
│ │   ──────────────────────────────     │  │   ──────────────────────────────     ││   Test-Generator                  │
│ │   4 Skills, 3 Rules, 2 Hooks        │  │   15 Skills, 8 Rules, 5 Agents      ││   Django-Patterns                 │
│ │   3 Agents                           │  │   12 Commands, 4 MCP Servers        ││                                   │
│ │   Last sync: 2h ago  [up to date]   │  │   Last sync: 1d ago  [outdated]     ││ Rules (3)                         │
│ ╰──────────────────────────────────────╯  ╰──────────────────────────────────────╯│   Strict-Types                    │
│                                                                                   │   PEP8-Lint                       │
│                                                                                   │   Error-Handling                  │
│                                                                                   │                                   │
│  [+ Add Registry]                                                                 │ Hooks (2)                         │
│                                                                                   │   Pre-Commit-Lint                 │
│                                                                                   │   Security-Scan                   │
│                                                                                   │                                   │
│                                                                                   │ Agents (3)                        │
│                                                                                   │   Code-Reviewer                   │
│                                                                                   │   Docs-Writer                     │
│                                                                                   │   Security-Auditor                │
│                                                                                   │                                   │
│                                                                                   │                                   │
│                                                                                   │                                   │
│                                                                                   │                                   │
├────────────────────────────────────────────────────────────────────────────────────┴───────────────────────────────────┤
│ arrows navigate * s sync * r remove * b browse items * enter drill in * ? help                            syllago v1.2.0│
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

**Card contents:** Name (bold), URL, separator, item counts by type, sync status
**Contents sidebar:** Shows all items in the selected registry, grouped by type
**Actions:** `[s] Sync`, `[r] Remove`, `[b] Browse` (switches to Explorer layout filtered to this registry's items)
**"Browse" action:** Pressing `b` or `Enter` on a contents item switches to the Content dropdown with that type selected, filtered to the registry source — bridging the Gallery Grid back to the Explorer layout

### Library — Gallery Grid

Library uses the same Gallery Grid but cards represent content types (Skills, Agents, etc.) instead of named collections.

```
120x40:
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL    Content: -- ▾    Collection: Library ▾    Config ▾                                       + Add     * New   │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Library Overview                                                                          [+ Add]     [* New]     │
│ 23 items total * 3 registries * 8 locally created * 15 installed                                                     │
│ Your personal collection of AI coding tool content                                                                   │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
┌────────────────────────────────────────────────────────────────────────────────────────────────┬───────────────────────┐
│ Library                                                                                       │ Recent Activity       │
│                                                                                               │                       │
│ ╭──────────────────────────╮  ╭──────────────────────────╮  ╭──────────────────────────╮      │ Installed              │
│ │ > Skills (8)             │  │   Agents (4)             │  │   MCP Servers (3)        │      │   Refactor-Py -> CC   │
│ │   ────────────────────── │  │   ────────────────────── │  │   ────────────────────── │      │   Auditor -> CC       │
│ │   5 from registries      │  │   2 from registries      │  │   1 from registry        │      │   Strict-Types -> CC  │
│ │   3 locally created      │  │   2 locally created      │  │   2 locally created      │      │                       │
│ │   6 installed            │  │   3 installed            │  │   2 installed            │      │ Added                 │
│ ╰──────────────────────────╯  ╰──────────────────────────╯  ╰──────────────────────────╯      │   Test-Gen (team-r)   │
│ ╭──────────────────────────╮  ╭──────────────────────────╮  ╭──────────────────────────╮      │   PEP8 (library)      │
│ │   Rules (5)              │  │   Hooks (2)              │  │   Commands (1)           │      │                       │
│ │   ────────────────────── │  │   ────────────────────── │  │   ────────────────────── │      │ Updated               │
│ │   3 from registries      │  │   1 from registry        │  │   1 locally created      │      │   team-rules synced   │
│ │   2 locally created      │  │   1 locally created      │  │                          │      │                       │
│ │   4 installed            │  │   1 installed            │  │   0 installed            │      │                       │
│ ╰──────────────────────────╯  ╰──────────────────────────╯  ╰──────────────────────────╯      │                       │
│                                                                                               │                       │
│                                                                                               │                       │
│                                                                                               │                       │
│                                                                                               │                       │
│                                                                                               │                       │
│                                                                                               │                       │
│                                                                                               │                       │
├────────────────────────────────────────────────────────────────────────────────────────────────┴───────────────────────┤
│ arrows navigate * enter browse type * / search * ? help                                                   syllago v1.2.0│
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

**Card contents:** Content type name + count, separator, source breakdown, install count
**Right sidebar:** "Recent Activity" — recent installs, additions, and sync updates
**Enter on a card:** Switches to Content dropdown with that type selected (bridges to Explorer layout)
**Why "Recent Activity":** The Library overview doesn't have "contents" of a single item to show. Instead, the sidebar shows actionable recent events — what you just installed, what was added from a registry sync, etc.

### Gallery Grid Responsive Behavior

| Width | Grid | Contents Sidebar |
|---|---|---|
| 120+ | 3 columns of cards | Full sidebar (~25% width) |
| 80-119 | 2 columns of cards | Narrower sidebar (~30% width) |
| 60-79 | 1 column (stacked cards) | Hidden — Enter drills into contents |
| <60 | "Terminal too small" | Hidden |

### Gallery Grid Keyboard

| Key | Action |
|---|---|
| Up/Down | Move between card rows |
| Left/Right | Move between cards in a row |
| Enter | Drill into card (bridge to Explorer layout for that content/registry) |
| Tab | Switch focus between card grid and contents sidebar |
| Home/End | First/last card |
| a | Apply (Loadouts only) |
| s | Sync (Registries only) |
| r | Remove (Registries only) |
| / | Search/filter cards |

## Responsive Behavior

| Width | Items List | Detail Zone |
|---|---|---|
| 120+ | Full columns (name, source) | Full adaptive zone |
| 80-119 | Compact columns (name, source truncated) | Adaptive zone, narrower splits |
| 60-79 | Name only | Detail replaces items (stacked/drill-in) |
| <60 | "Terminal too small" | (hidden) |

At 60-79 cols, the layout switches from side-by-side to **stacked**: items list is full-width, Enter drills into a full-width detail view, Esc goes back. This matches the current single-pane fallback in `splitViewModel`.

---

## Modals

Modals are centered overlays for confirmations, warnings, multi-step wizards, and any interaction that requires focused user input before continuing. They sit on top of the shell and dim the background.

### Design Principles

1. **All modals share the same visual frame** — rounded border, accent color border, semi-transparent background
2. **Fixed width: 56 characters** — one standard size, no exceptions (matches current convention)
3. **Maximum height: terminal height - 4** — never overflows the terminal; content scrolls instead
4. **Buttons always at the bottom** — pinned outside the scroll region, never pushed off-screen
5. **Body content scrolls** — when content exceeds available height, the body area scrolls independently while title and buttons stay fixed
6. **Scroll indicators** — `(N more above)` / `(N more below)` shown inside the modal body area when content overflows
7. **All text in modals is copyable** — `c` copies the FULL modal body text (not just visible portion) to clipboard
8. **Esc always dismisses** — no trapped modals, Esc is always an exit
9. **Click outside dismisses** — clicking the dimmed background closes the modal

### Modal Anatomy

Every modal has three fixed zones. Only the body zone scrolls:

```
╭──────────────────────────────────────────────────────────╮
│                                                          │  <- Title zone (fixed)
│  Modal Title                                             │     Always visible
│                                                          │
│  (3 more above)                                          │  <- Scroll indicator
│  ...scrollable body content line 4...                    │  <- Body zone (scrolls)
│  ...scrollable body content line 5...                    │     Up/Down or mouse wheel
│  ...scrollable body content line 6...                    │     to scroll
│  ...scrollable body content line 7...                    │
│  ...scrollable body content line 8...                    │
│  (2 more below)                                          │  <- Scroll indicator
│                                                          │
│            [ Cancel ]        [ Confirm ]                 │  <- Button zone (fixed)
│                                                          │     Always visible
╰──────────────────────────────────────────────────────────╯
```

### Size Constraints

```
Modal width:  56 characters (fixed, all modals)
Modal height: min(content_height, terminal_height - 4)

Body height = modal_height - title_rows - button_rows - padding
            = modal_height - 2 (title) - 2 (buttons) - 4 (padding top/bottom)

At 80x30 terminal:  max modal height = 26, body = ~18 scrollable lines
At 60x20 terminal:  max modal height = 16, body = ~8 scrollable lines
At 120x40 terminal: max modal height = 36, body = ~28 scrollable lines
```

### Modal Types

#### Confirmation Modal

For destructive or significant actions (uninstall, remove registry, apply loadout).

```
Background dimmed...
                    ╭──────────────────────────────────────────────────────────╮
                    │                                                          │
                    │  Uninstall alpha-skill?                                  │
                    │                                                          │
                    │  This will remove the symlink from Claude Code.          │
                    │  The content stays in your library.                      │
                    │                                                          │
                    │                                                          │
                    │                                                          │
                    │            [ Cancel ]        [ Uninstall ]               │
                    │                                                          │
                    ╰──────────────────────────────────────────────────────────╯

Keyboard: left/right switch buttons * enter confirm * esc cancel * y/n shortcuts * c copy text
```

#### Warning Modal

For non-blocking warnings that need acknowledgment (hook with broken compat, security scan results).

```
                    ╭──────────────────────────────────────────────────────────╮
                    │                                                          │
                    │  Warning: Hook compatibility issue                       │
                    │                                                          │
                    │  pre-commit-lint has "broken" compatibility              │
                    │  with Gemini CLI:                                        │
                    │                                                          │
                    │  The matcher syntax "shell" is not supported.            │
                    │  The hook will be installed but may not trigger           │
                    │  correctly.                                              │
                    │                                                          │
                    │                                                          │
                    │          [ Cancel ]        [ Install Anyway ]            │
                    │                                                          │
                    ╰──────────────────────────────────────────────────────────╯

Keyboard: same as confirmation
```

#### Wizard Modal (Multi-Step)

For multi-step flows (import, install to provider, env setup). Shows step progress and allows back navigation.

```
                    ╭──────────────────────────────────────────────────────────╮
                    │                                                          │
                    │  Install alpha-skill (1 of 3)                            │
                    │                                                          │
                    │  Select providers:                                       │
                    │                                                          │
                    │    [x] Claude Code                                       │
                    │    [ ] Gemini CLI                                        │
                    │    [ ] Cursor   (not detected)                           │
                    │                                                          │
                    │                                                          │
                    │                                                          │
                    │             [ Back ]          [ Next ]                   │
                    │                                                          │
                    ╰──────────────────────────────────────────────────────────╯

Keyboard: up/down navigate options * space toggle * left/right buttons * enter confirm
          esc goes back one step (first step = cancel)
```

#### Text Input Modal

For single-field inputs (registry URL, loadout name, save path).

```
                    ╭──────────────────────────────────────────────────────────╮
                    │                                                          │
                    │  Add Registry                                            │
                    │                                                          │
                    │  URL:                                                    │
                    │  ┌──────────────────────────────────────────────────┐    │
                    │  │ https://github.com/team/syllago-rules.git       │    │
                    │  └──────────────────────────────────────────────────┘    │
                    │                                                          │
                    │  Name (optional):                                        │
                    │  ┌──────────────────────────────────────────────────┐    │
                    │  │ team-rules                                       │    │
                    │  └──────────────────────────────────────────────────┘    │
                    │                                                          │
                    │            [ Cancel ]          [ Add ]                   │
                    │                                                          │
                    ╰──────────────────────────────────────────────────────────╯

Keyboard: tab switches fields * enter confirms * esc cancels * click focuses field
```

#### Scrolling Modal Example

When body content exceeds available height, scroll indicators appear and Up/Down scrolls the body while title and buttons stay pinned. This commonly occurs with:
- Wizard steps with many options (e.g., 15+ providers or items to select)
- Import conflict resolution with many conflicts
- Error details with long paths or stack traces

```
On a 60x20 terminal (max modal height = 16, body = ~8 lines):

                    ╭──────────────────────────────────────────────────────────╮
                    │                                                          │
                    │  Import from team-rules (1 of 3)                         │
                    │                                                          │
                    │  Select items to import:                                 │
                    │  (5 more above)                                          │
                    │    [x] gamma-skill                                       │
                    │    [ ] delta-rule                                         │
                    │    [x] epsilon-hook                                       │
                    │    [ ] zeta-agent                                         │
                    │    [ ] eta-command                                        │
                    │  (3 more below)                                           │
                    │                                                          │
                    │             [ Back ]          [ Next ]                   │
                    │                                                          │
                    ╰──────────────────────────────────────────────────────────╯

Keyboard: up/down scroll + navigate * space toggle * pgup/pgdn page jump
          c copies full list (all items, not just visible)
```

### Modal Keyboard Summary

| Key | Action | Context |
|---|---|---|
| Esc | Cancel / go back one step / dismiss | Always |
| Enter | Confirm focused button | Always |
| Left/Right | Switch between buttons | Always |
| Up/Down | Navigate options / scroll body | Option lists / long content |
| PgUp/PgDn | Page jump in scrollable body | Long content |
| Space | Toggle checkbox | Option lists |
| Tab | Next form field | Text inputs |
| y/Y | Confirm (shortcut) | Confirmation modals |
| n/N | Cancel (shortcut) | Confirmation modals |
| c | Copy modal body text to clipboard | Always |

---

## Toasts

Toasts are transient, non-actionable messages for system feedback (success, warning, error, failure). They appear as an overlay at the top of the screen, below the top bar but above the metadata bar. They auto-dismiss or dismiss on keypress.

### Design Principles

1. **Non-blocking** — toasts never prevent interaction with the TUI
2. **Always copyable** — `c` copies the toast message to clipboard (critical for error messages)
3. **Always dismissible** — any keypress dismisses success toasts; Esc or `c` dismisses error toasts
4. **Color-coded with text prefix** — meaning is never color-only (accessible)
5. **Position: top overlay** — spans full width below the top bar, pushing content down temporarily
6. **Auto-dismiss: success only** — success toasts auto-dismiss after 3 seconds; errors persist until dismissed

### Toast Types

#### Success Toast
```
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL    Content: Skills ▾    Collection: -- ▾    Config ▾                                        + Add     * New   │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Done: alpha-skill installed to Claude Code                                                                  c copy   │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ alpha-skill                                                      [i] Install  [u] Uninstall  [c] Copy  [p] Share     │
│ ...                                                                                                                  │
```

- Green text with `Done:` prefix
- Auto-dismisses after 3 seconds
- Any keypress dismisses immediately (key passes through to the TUI)
- `c` copies message to clipboard before dismissing

#### Warning Toast
```
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Warn: Hook pre-commit-lint has partial compatibility with Gemini CLI — matcher syntax differs              c copy   │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- Amber/yellow text with `Warn:` prefix
- Auto-dismisses after 5 seconds (longer than success — user may need to read)
- Any keypress dismisses

#### Error Toast
```
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Error: Failed to install alpha-skill to Claude Code                                                         c copy   │
│ Permission denied: ~/.claude/rules/alpha-skill — check directory permissions                                         │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- Red text with `Error:` prefix
- Does NOT auto-dismiss — persists until Esc or `c`
- Can be multi-line for detailed error messages
- `c` copies the full error text (sanitized) to clipboard
- Esc dismisses without copying
- Other keypresses pass through (error stays visible while you continue working)

#### Multi-Line Toast (Batch Operations)
```
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Done: Imported 5 items from team-rules                                                                      c copy   │
│   Skills: alpha-skill, beta-skill                                                                                    │
│   Rules: typescript-strict, go-conventions                                                                           │
│   Hooks: pre-commit-lint                                                                                             │
│   (2 more below)                                                                                                     │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- Scrollable with Up/Down when more than 5 lines
- Shows scroll indicator `(N more below)` / `(N more above)`
- `c` copies ALL lines (not just visible)

### Toast at 80 cols
```
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Content: Skills ▾  Collection: -- ▾  Config ▾               + Add * New │
╰──────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────────────────────────────────────────────────────────────────────┐
│ Done: alpha-skill installed to Claude Code                            c copy    │
└──────────────────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────────────────┐
│ alpha-skill                                         [i] Install  [c] Copy      │
│ ...                                                                             │
```

### Toast Keyboard Summary

| Key | Success Toast | Warning Toast | Error Toast |
|---|---|---|---|
| Any key (except c, Esc) | Dismiss + pass through | Dismiss + pass through | Pass through (toast stays) |
| c | Copy + dismiss | Copy + dismiss | Copy + dismiss |
| Esc | Dismiss | Dismiss | Dismiss (no copy) |
| Up/Down | (n/a) | (n/a) | Scroll (if multi-line) |
| Auto-dismiss | 3 seconds | 5 seconds | Never |

### Toast vs Modal Decision Guide

| Situation | Use | Why |
|---|---|---|
| Action succeeded | Toast (success) | Non-blocking, transient feedback |
| Action succeeded with caveats | Toast (warning) | User should know, but doesn't need to act |
| Action failed | Toast (error) | User needs to see the error, may want to copy it |
| Action needs confirmation | Modal (confirmation) | Blocking — user must decide before proceeding |
| Action has consequences to explain | Modal (warning) | User needs to understand before deciding |
| Multi-step flow | Modal (wizard) | Sequential input required |
| User input needed | Modal (text input) | Focused input collection |

---

## Implementation: TUI Rewrite

### Strategy

**Full rewrite of `cli/internal/tui/`.** Back up the current package to `cli/internal/tui_v1/` for reference if needed, then build the new TUI from scratch. The old code is a safety net, not a starting point.

### What stays untouched

All backend packages — the new TUI imports the same interfaces:
- `catalog` — content discovery, loading, types
- `provider` — provider detection, configuration
- `installer` — install/uninstall operations
- `converter` — format conversion, hook adapters
- `config` — user configuration
- `registry` — remote registry client
- `loadout` — loadout apply/remove/preview
- `promote` — local-to-shared promotion
- `sandbox` — sandbox configuration
- `updater` — self-update logic
- `cmd/syllago` — cobra command wiring (minor updates for new TUI entry point)

### New TUI file structure (planned)

```
cli/internal/tui/
  app.go              — root model, shell rendering, message routing
  styles.go           — all colors, styles, inline title helpers
  keys.go             — all key bindings
  topbar.go           — dropdown navigation bar (Content/Collection/Config)
  metadata.go         — metadata bar (item info + actions)
  helpbar.go          — footer help bar with version
  explorer.go         — Explorer layout (items list + content zone)
  items.go            — items list model (left pane in Explorer)
  content_split.go    — split content zone (Skills, Hooks)
  content_preview.go  — full-width preview zone (Agents, Rules, MCP, Commands)
  gallery.go          — Gallery Grid layout (collections)
  cards.go            — card grid model (left pane in Gallery)
  contents.go         — contents sidebar model (right pane in Gallery)
  modal.go            — modal overlay (confirmation, warning, wizard, input)
  toast.go            — toast overlay (success, warning, error)
  search.go           — search/filter bar
  dropdown.go         — dropdown menu component
  scroll.go           — shared scroll helpers
  helpers.go          — shared rendering helpers (inline titles, truncation, etc.)
  testdata/           — golden files (all new)
```

### Build order

1. **Shell:** app.go + styles.go + keys.go + helpbar.go — renders empty frame
2. **Top bar:** topbar.go + dropdown.go — navigation works, dropdowns open/close
3. **Explorer layout:** explorer.go + items.go + content_preview.go — list items, show preview
4. **Metadata bar:** metadata.go — selected item info + actions
5. **Split content zone:** content_split.go — Skills/Hooks file tree + preview
6. **Gallery Grid:** gallery.go + cards.go + contents.go — collection browsing
7. **Modals:** modal.go — all four types with scroll support
8. **Toasts:** toast.go — success/warning/error with copy
9. **Search:** search.go — live filtering
10. **Polish:** golden tests, responsive sizes, mouse support, accessibility
