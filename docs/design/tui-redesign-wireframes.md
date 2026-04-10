# TUI Redesign Wireframes

Five candidate layouts for the syllago TUI redesign. All layouts share:
- **Top navigation bar** (replaces sidebar) — always visible
- **No sidebar** — all navigation in the top bar
- Mint color for content-type tabs, purple for collection tabs
- Prominent Import/Create actions
- Footer help bar

## Common: Top Navigation Bar

The top bar is the primary navigation element across all layouts.

### At 120 columns
```
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL   Skills  Agents  MCP  Rules  Hooks  Commands  ││  Library  Registries  Loadouts  ││  Settings ▾  + Import  * New │
│       ~~~~~~                                  mint  ││                          purple  ││                              │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### At 80 columns (abbreviated)
```
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Skills Agents MCP Rules Hooks Cmds │ Lib Reg Load │ [G] + Import  * New    │
│      ~~~~~~                        mint │       purple │                        │
╰──────────────────────────────────────────────────────────────────────────────────╯
```

**Notes:**
- `SYL` = logo mark (from logos/logo.svg, rendered as text in TUI)
- Active content tab is highlighted (bold + underline in mint)
- Active collection tab is highlighted (bold + underline in purple)
- `[G]` = gear icon for Settings/Sandbox dropdown (expands on click/enter)
- `+ Import` = add existing content (from registry, provider, or file)
- `* New` = create/author new content from scratch within syllago
- `~~` under "Skills" indicates the active tab

---

## Layout 1: "Studio" — Three-Column IDE

**Philosophy:** Everything visible at once. Items, files, and preview flow left-to-right. Maximum information density for power users.

**Panes:** Items (25%) | Files (20%) | Preview (55%)

### 120x40
```
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL   Skills  Agents  MCP  Rules  Hooks  Commands  ││  Library  Registries  Loadouts  ││  Settings ▾  + Import  * New │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌───────────────────────────┬──────────────────────┬─────────────────────────────────────────────────────────────────────┐
│ Items                     │ Files                │ Preview                                                            │
│ ─────────────────────     │ ──────────────────   │ ─────────────────────────────────────────────────────────────────── │
│ > alpha-skill       [CC]  │ > SKILL.md           │  1  ---                                                            │
│   beta-skill     [CC,GC]  │   helpers.md         │  2  title: Alpha Skill                                             │
│   gamma-skill   [Cursor]  │   utils/             │  3  description: A helpful skill                                   │
│   delta-skill       [CC]  │     tool.md          │  4  providers: [claude-code, gemini-cli]                            │
│   epsilon-skill     [GC]  │     parser.md        │  5  ---                                                            │
│                           │                      │  6                                                                  │
│                           │                      │  7  # Alpha Skill                                                  │
│                           │                      │  8                                                                  │
│                           │                      │  9  This skill extends AI tool capabilities by                      │
│ ─ Source ──────────────── │                      │ 10  providing structured workflows for...                          │
│ Registry: team-rules      │                      │ 11                                                                  │
│ Type: Skills              │                      │ 12  ## Usage                                                        │
│ Providers: CC, Gemini CLI │                      │ 13                                                                  │
│ Files: 4                  │                      │ 14  Invoke with `/alpha` in your AI tool.                           │
│                           │                      │ 15                                                                  │
│                           │                      │ 16  ## Parameters                                                   │
│                           │                      │ 17                                                                  │
│                           │                      │ 18  - `--verbose`: Enable detailed output                           │
│                           │                      │ 19  - `--format`: Output format (json, yaml)                        │
│                           │                      │ 20                                                                  │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
│                           │                      │                                                                     │
├───────────────────────────┴──────────────────────┴─────────────────────────────────────────────────────────────────────┤
│ h/l switch pane  * j/k navigate  * / search  * i install  * ? help                                       syllago    │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### 80x30
```
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Skills Agents MCP Rules Hooks Cmds │ Lib Reg Load │ [G] + Import  * New    │
╰──────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────┬───────────────┬────────────────────────────────────────────────┐
│ Items            │ Files         │ Preview                                        │
│ ──────────────── │ ───────────── │ ──────────────────────────────────────────────  │
│ > alpha-skill CC │ > SKILL.md    │  1  ---                                        │
│   beta-skill  GC │   helpers.md  │  2  title: Alpha Skill                         │
│   gamma-skill CC │   utils/      │  3  description: A helpful skill               │
│   delta-skill    │     tool.md   │  4  providers: [claude-code]                    │
│                  │               │  5  ---                                         │
│ ─ Source ─────── │               │  6                                              │
│ Reg: team-rules  │               │  7  # Alpha Skill                              │
│ Type: Skills     │               │  8                                              │
│ Provs: CC, GC   │               │  9  This skill extends AI tool                  │
│ Files: 4         │               │ 10  capabilities by providing...                │
│                  │               │ 11                                              │
│                  │               │ 12  ## Usage                                    │
│                  │               │ 13                                              │
│                  │               │ 14  Invoke with `/alpha` in your                │
│                  │               │ 15  AI tool.                                    │
│                  │               │ 16                                              │
│                  │               │ 17  ## Parameters                               │
│                  │               │ 18                                              │
│                  │               │ 19  - `--verbose`: Enable detail...             │
│                  │               │ 20  - `--format`: Output format                 │
│                  │               │                                                 │
│                  │               │                                                 │
│                  │               │                                                 │
│                  │               │                                                 │
├──────────────────┴───────────────┴─────────────────────────────────────────────────┤
│ h/l pane * j/k nav * / search * i install * ? help                    syllago    │
╰──────────────────────────────────────────────────────────────────────────────────╯
```

**Strengths:**
- Maximum information density — see items, files, and content simultaneously
- Natural left-to-right flow: browse → drill → read
- Metadata visible in the items pane (below the list)
- Familiar to IDE/editor users

**Weaknesses:**
- Items column is narrow — long names truncate
- Files column is narrow — deep trees truncate
- At 80 cols, all three panes feel cramped
- Three panes to navigate between (h/l switching)

**Accessibility:**
- Clear pane boundaries with visible borders
- Active pane indicated by highlighted border color
- Tab order: Items → Files → Preview

---

## Layout 2: "Workbench" — Dual-Pane + Info Strip

**Philosophy:** Two main panes dominate the screen (items + preview). Metadata and actions live in a compact info strip at the bottom. Files are shown inline in the items list as an expandable tree.

**Panes:** Items with tree (35%) | Preview (65%) | Info strip (3 rows, bottom)

### 120x40
```
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL   Skills  Agents  MCP  Rules  Hooks  Commands  ││  Library  Registries  Loadouts  ││  Settings ▾  + Import  * New │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────────────────────────────┬─────────────────────────────────────────────────────────────────────────────┐
│ Items                                    │ Preview: SKILL.md                                                          │
│ ──────────────────────────────────────── │ ───────────────────────────────────────────────────────────────────────── │
│ v alpha-skill                 [CC, GC]   │  1  ---                                                                    │
│     SKILL.md ...................    2 KB  │  2  title: Alpha Skill                                                     │
│   > helpers.md .....................800B  │  3  description: A helpful skill that extends AI tool                      │
│   v utils/ ......................... (2)  │  4  providers: [claude-code, gemini-cli]                                   │
│       tool.md ...................  1 KB   │  5  ---                                                                    │
│       parser.md .................  500B   │  6                                                                         │
│ > beta-skill                      [CC]   │  7  # Alpha Skill                                                         │
│ > gamma-skill               [Cursor]     │  8                                                                         │
│ > delta-skill                     [CC]   │  9  This skill extends AI tool capabilities by providing structured        │
│ > epsilon-skill                   [GC]   │ 10  workflows for common development tasks.                                │
│                                          │ 11                                                                         │
│                                          │ 12  ## Usage                                                               │
│                                          │ 13                                                                         │
│                                          │ 14  Invoke with `/alpha` in your AI coding tool. The skill will guide      │
│                                          │ 15  you through a step-by-step workflow.                                   │
│                                          │ 16                                                                         │
│                                          │ 17  ## Parameters                                                          │
│                                          │ 18                                                                         │
│                                          │ 19  - `--verbose`: Enable detailed output                                  │
│                                          │ 20  - `--format`: Output format (json, yaml, text)                         │
│                                          │ 21                                                                         │
│                                          │ 22  ## Examples                                                             │
│                                          │ 23                                                                         │
│                                          │ 24  ```bash                                                                │
│                                          │ 25  /alpha --verbose --format json                                         │
│                                          │ 26  ```                                                                    │
│                                          │                                                                            │
│                                          │                                                                            │
│                                          │                                                                            │
│                                          │                                                                            │
│                                          │                                                                            │
│                                          │                                                                            │
├──────────────────────────────────────────┴─────────────────────────────────────────────────────────────────────────────┤
│ Type: Skills    Source: team-rules (registry)    Providers: Claude Code, Gemini CLI    4 files                        │
│ [i] Install to Claude Code    [u] Uninstall    [c] Copy path    [p] Share                                            │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ j/k navigate * tab expand/collapse * h/l pane * / search * ? help                                        syllago    │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### 80x30
```
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Skills Agents MCP Rules Hooks Cmds │ Lib Reg Load │ [G] + Import  * New    │
╰──────────────────────────────────────────────────────────────────────────────────╯
┌───────────────────────────────┬──────────────────────────────────────────────────┐
│ Items                         │ Preview: SKILL.md                                │
│ ───────────────────────────── │ ──────────────────────────────────────────────── │
│ v alpha-skill        [CC,GC]  │  1  ---                                          │
│     SKILL.md ..........  2KB  │  2  title: Alpha Skill                           │
│   > helpers.md ........ 800B  │  3  description: A helpful skill                 │
│   v utils/ ............. (2)  │  4  providers: [claude-code]                     │
│       tool.md .........  1KB  │  5  ---                                          │
│       parser.md .......500B   │  6                                               │
│ > beta-skill           [CC]   │  7  # Alpha Skill                                │
│ > gamma-skill       [Cursor]  │  8                                               │
│ > delta-skill          [CC]   │  9  This skill extends AI tool                   │
│ > epsilon-skill        [GC]   │ 10  capabilities by providing...                 │
│                               │ 11                                               │
│                               │ 12  ## Usage                                     │
│                               │ 13                                               │
│                               │ 14  Invoke with `/alpha` in your                 │
│                               │ 15  AI tool.                                     │
│                               │ 16                                               │
│                               │ 17  ## Parameters                                │
│                               │ 18                                               │
│                               │ 19  - `--verbose`: Enable detail...              │
│                               │ 20  - `--format`: Output format                  │
├───────────────────────────────┴──────────────────────────────────────────────────┤
│ Skills * team-rules * CC,GC * 4 files    [i] Install  [c] Copy  [p] Share       │
├──────────────────────────────────────────────────────────────────────────────────┤
│ j/k nav * tab expand * h/l pane * / search * ? help                  syllago    │
╰──────────────────────────────────────────────────────────────────────────────────╯
```

**Strengths:**
- Items and files are merged into one expandable tree — one less pane
- Preview gets more horizontal space (better for code/markdown)
- Metadata always visible in info strip without consuming main content area
- Actions always visible and accessible
- Scales well from 80 to 160+ columns

**Weaknesses:**
- Expanded file trees push other items down (visual instability)
- Info strip consumes 3 rows at the bottom
- Less "at a glance" file overview than a dedicated files pane

**Accessibility:**
- Two-pane navigation is simpler than three
- Info strip is read-only (no navigation needed)
- Expandable tree: space/tab to toggle, familiar pattern
- Actions have keyboard shortcuts visible inline

---

## Layout 3: "Explorer" — Master-Detail

**Philosophy:** Classic master-detail pattern (like email clients, VS Code). Narrow items list on the left, full detail panel on the right with a metadata header and files/preview split below.

**Panes:** Items (22%) | Detail: [Metadata header (4 rows) / Files+Preview split]

### 120x40
```
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL   Skills  Agents  MCP  Rules  Hooks  Commands  ││  Library  Registries  Loadouts  ││  Settings ▾  + Import  * New │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌─────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────┐
│ Skills (5)              │ alpha-skill                                                                                │
│ ─────────────────────── │ ────────────────────────────────────────────────────────────────────────────────────────────│
│                         │ Type: Skills          Source: team-rules (registry)           Installed: CC [checkmark]     │
│ > alpha-skill           │ Providers: Claude Code, Gemini CLI                            Files: 4                     │
│   beta-skill            │ Description: A helpful skill that extends AI tool capabilities                             │
│   gamma-skill           │ ──────────────────────────────────────────────────────────────────────────────────────────── │
│   delta-skill           │                                                                                            │
│   epsilon-skill         │ Files                  │ Preview: SKILL.md                                                 │
│                         │ ────────────────────── │ ─────────────────────────────────────────────────────────────────  │
│                         │ > SKILL.md             │  1  ---                                                           │
│                         │   helpers.md           │  2  title: Alpha Skill                                            │
│                         │   utils/               │  3  description: A helpful skill that extends AI tool             │
│                         │     tool.md            │  4  providers: [claude-code, gemini-cli]                           │
│                         │     parser.md          │  5  ---                                                           │
│                         │                        │  6                                                                │
│                         │                        │  7  # Alpha Skill                                                │
│                         │                        │  8                                                                │
│                         │                        │  9  This skill extends AI tool capabilities by providing          │
│                         │                        │ 10  structured workflows for common development tasks.            │
│                         │                        │ 11                                                                │
│                         │                        │ 12  ## Usage                                                      │
│                         │                        │ 13                                                                │
│                         │                        │ 14  Invoke with `/alpha` in your AI coding tool. The skill        │
│                         │                        │ 15  will guide you through a step-by-step workflow.               │
│                         │                        │ 16                                                                │
│                         │                        │ 17  ## Parameters                                                 │
│                         │                        │ 18                                                                │
│                         │                        │ 19  - `--verbose`: Enable detailed output                         │
│                         │                        │ 20  - `--format`: Output format (json, yaml, text)                │
│                         │                        │ 21                                                                │
│                         │                        │                                                                   │
│                         │                        │                                                                   │
│                         │                        │                                                                   │
│                         │                        │                                                                   │
│                         │                        │                                                                   │
│                         │                        │                                                                   │
│                         │ [i] Install  [u] Uninstall  [c] Copy  [p] Share                                            │
├─────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────┤
│ j/k navigate * h/l pane * / search * enter select * ? help                                               syllago    │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### 80x30
```
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Skills Agents MCP Rules Hooks Cmds │ Lib Reg Load │ [G] + Import  * New    │
╰──────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────┬───────────────────────────────────────────────────────────────┐
│ Skills (5)       │ alpha-skill                                                   │
│ ──────────────── │ ──────────────────────────────────────────────────────────── │
│                  │ Skills * team-rules * CC,GC * 4 files * Installed: CC [ok]    │
│ > alpha-skill    │ A helpful skill that extends AI tool capabilities             │
│   beta-skill     │ ──────────────────────────────────────────────────────────── │
│   gamma-skill    │                                                               │
│   delta-skill    │ Files            │ Preview: SKILL.md                          │
│   epsilon-skill  │ ──────────────── │ ──────────────────────────────────────── │
│                  │ > SKILL.md       │  1  ---                                   │
│                  │   helpers.md     │  2  title: Alpha Skill                    │
│                  │   utils/         │  3  description: A helpful skill          │
│                  │     tool.md      │  4  providers: [claude-code]              │
│                  │     parser.md    │  5  ---                                   │
│                  │                  │  6                                         │
│                  │                  │  7  # Alpha Skill                         │
│                  │                  │  8                                         │
│                  │                  │  9  This skill extends AI tool            │
│                  │                  │ 10  capabilities by providing...           │
│                  │                  │ 11                                         │
│                  │                  │ 12  ## Usage                               │
│                  │                  │ 13                                         │
│                  │                  │ 14  Invoke with `/alpha` in your           │
│                  │                  │                                            │
│                  │ [i] Install  [u] Uninstall  [c] Copy  [p] Share              │
├──────────────────┴───────────────────────────────────────────────────────────────┤
│ j/k nav * h/l pane * / search * enter select * ? help                syllago    │
╰──────────────────────────────────────────────────────────────────────────────────╯
```

**Strengths:**
- Familiar master-detail pattern (Outlook, VS Code, Finder)
- Items list is always visible — easy to browse and compare
- Metadata header provides context without consuming pane space
- Detail area has natural sub-structure (metadata → files → preview)
- Actions visible at the bottom of the detail pane

**Weaknesses:**
- Items list is narrow — only shows names (no inline metadata)
- Detail panel has three sub-sections to navigate
- At 80 cols, the files+preview split inside the detail panel gets cramped
- Deeper nesting (items → detail → files → preview) is more complex

**Accessibility:**
- Tab cycles: Items → Detail metadata → Files → Preview → Actions
- Items list is simple (one column, j/k only)
- Screen reader friendly: natural heading hierarchy in detail panel
- Metadata header is read-only — tab skips to interactive files

---

## Layout 4: "Mosaic" — Adaptive Four-Quadrant

**Philosophy:** Dashboard-style layout with four independently scrollable quadrants. Each quadrant serves a distinct purpose. Responsive: quadrants reflow to two-column or stacked at narrow widths.

**Panes:** Items (top-left) | Metadata (top-right) | Files (bottom-left) | Preview (bottom-right)

### 120x40
```
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL   Skills  Agents  MCP  Rules  Hooks  Commands  ││  Library  Registries  Loadouts  ││  Settings ▾  + Import  * New │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────────────────────────────────────────┬─────────────────────────────────────────────────────────────────┐
│ Items                                                │ Metadata                                                       │
│ ──────────────────────────────────────────────────── │ ───────────────────────────────────────────────────────────── │
│                                                      │                                                                │
│    Name               Provider(s)     Source         │  Name:        alpha-skill                                      │
│    ─────────────────  ──────────────  ────────────── │  Type:        Skills                                           │
│  > alpha-skill        CC, Gemini CLI  team-rules     │  Source:      team-rules (registry)                            │
│    beta-skill         Claude Code     team-rules     │  Providers:   Claude Code, Gemini CLI                          │
│    gamma-skill        Cursor          library        │  Files:       4 (SKILL.md, helpers.md, utils/tool.md, ...)     │
│    delta-skill        Claude Code     team-rules     │  Installed:   Claude Code [checkmark]  Gemini CLI [--]         │
│    epsilon-skill      Gemini CLI      my-registry    │  Description: A helpful skill that extends AI tool             │
│                                                      │               capabilities for structured workflows.           │
│                                                      │                                                                │
│                                                      │  [i] Install  [u] Uninstall  [c] Copy  [p] Share              │
│                                                      │                                                                │
│                                                      │                                                                │
│                                                      │                                                                │
├──────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────┤
│ Files                                                │ Preview: SKILL.md                                              │
│ ──────────────────────────────────────────────────── │ ───────────────────────────────────────────────────────────── │
│                                                      │  1  ---                                                        │
│ > SKILL.md                                  2 KB     │  2  title: Alpha Skill                                         │
│   helpers.md                                800B     │  3  description: A helpful skill that extends AI tool          │
│   utils/                                             │  4  providers: [claude-code, gemini-cli]                        │
│     tool.md                                 1 KB     │  5  ---                                                        │
│     parser.md                               500B     │  6                                                             │
│                                                      │  7  # Alpha Skill                                              │
│                                                      │  8                                                             │
│                                                      │  9  This skill extends AI tool capabilities by providing       │
│                                                      │ 10  structured workflows for common development tasks.         │
│                                                      │ 11                                                             │
│                                                      │ 12  ## Usage                                                   │
│                                                      │ 13                                                             │
│                                                      │ 14  Invoke with `/alpha` in your AI coding tool. The skill     │
│                                                      │ 15  will guide you through a step-by-step workflow.            │
│                                                      │ 16                                                             │
│                                                      │ 17  ## Parameters                                              │
├──────────────────────────────────────────────────────┴─────────────────────────────────────────────────────────────────┤
│ 1-4 switch quadrant * j/k navigate * / search * ? help                                                   syllago    │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### 80x30
```
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Skills Agents MCP Rules Hooks Cmds │ Lib Reg Load │ [G] + Import  * New    │
╰──────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────────────────────────┬───────────────────────────────────────────┐
│ Items                                │ Metadata                                  │
│ ──────────────────────────────────── │ ─────────────────────────────────────── │
│    Name            Prov    Source    │ alpha-skill                               │
│  > alpha-skill     CC,GC  team-ru   │ Skills * team-rules * CC,GC               │
│    beta-skill      CC     team-ru   │ 4 files * Installed: CC [ok]              │
│    gamma-skill     Curs   library   │ A helpful skill that extends              │
│    delta-skill     CC     team-ru   │ AI tool capabilities.                     │
│    epsilon-skill   GC     my-reg    │                                            │
│                                      │ [i] Install [u] Uninstall [c] Copy       │
├──────────────────────────────────────┼───────────────────────────────────────────┤
│ Files                                │ Preview: SKILL.md                         │
│ ──────────────────────────────────── │ ─────────────────────────────────────── │
│ > SKILL.md                    2KB    │  1  ---                                   │
│   helpers.md                  800B   │  2  title: Alpha Skill                    │
│   utils/                             │  3  description: A helpful skill          │
│     tool.md                   1KB    │  4  providers: [claude-code]              │
│     parser.md                 500B   │  5  ---                                   │
│                                      │  6                                        │
│                                      │  7  # Alpha Skill                         │
│                                      │  8                                        │
│                                      │  9  This skill extends AI tool            │
│                                      │ 10  capabilities by providing...           │
│                                      │ 11                                        │
│                                      │ 12  ## Usage                              │
├──────────────────────────────────────┴───────────────────────────────────────────┤
│ 1-4 quadrant * j/k nav * / search * ? help                          syllago    │
╰──────────────────────────────────────────────────────────────────────────────────╯
```

**Strengths:**
- All four information types visible simultaneously
- Items list is wide enough for inline metadata columns (name, provider, source)
- Each quadrant is independent — can scroll/navigate without affecting others
- Metadata pane includes description AND actions — no separate info strip
- Balanced visual weight across all four areas

**Weaknesses:**
- Four panes means more complex keyboard navigation
- Each quadrant gets less vertical space (especially at 80x30)
- Preview area is smaller than in other layouts
- The "quadrant" mental model is less familiar than columns or master-detail

**Accessibility:**
- Number keys (1-4) for direct quadrant access
- Active quadrant has highlighted border
- Each quadrant has its own scroll position
- Tab order follows Z-pattern: top-left → top-right → bottom-left → bottom-right

---

## Layout 5: "Commander" — Full-Width Table + Drawer

**Philosophy:** Maximizes the items list with a rich multi-column table spanning full width. Selecting an item opens a "drawer" panel that slides up from the bottom, showing files and preview. The table remains visible above for context.

**Panes:** Full-width table (top, 40-60%) | Drawer: [Files | Preview] (bottom, 40-60%)

### 120x40 — Table mode (no item selected / drawer closed)
```
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL   Skills  Agents  MCP  Rules  Hooks  Commands  ││  Library  Registries  Loadouts  ││  Settings ▾  + Import  * New │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Skills (5 items, 3 from registries, 2 from library)                                                          / Search│
│                                                                                                                       │
│    Name               Description                            Provider(s)       Source          Installed    Files     │
│    ─────────────────  ─────────────────────────────────────  ────────────────  ──────────────  ───────────  ───────   │
│  > alpha-skill        A helpful skill that extends AI        CC, Gemini CLI    team-rules      CC [ok]       4       │
│    beta-skill         Advanced code review automation        Claude Code       team-rules      CC [ok]       2       │
│    gamma-skill        Multi-provider testing framework       Cursor            library         --            1       │
│    delta-skill        Documentation generator                Claude Code       team-rules      --            3       │
│    epsilon-skill      Performance profiling assistant         Gemini CLI        my-registry     --            2       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
│                                                                                                                       │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ j/k navigate * enter open drawer * / search * i install * ? help                                          syllago    │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### 120x40 — Drawer open (item selected with Enter)
```
╭────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ SYL   Skills  Agents  MCP  Rules  Hooks  Commands  ││  Library  Registries  Loadouts  ││  Settings ▾  + Import  * New │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│    Name               Description                            Provider(s)       Source          Installed    Files     │
│    ─────────────────  ─────────────────────────────────────  ────────────────  ──────────────  ───────────  ───────   │
│  > alpha-skill        A helpful skill that extends AI        CC, Gemini CLI    team-rules      CC [ok]       4       │
│    beta-skill         Advanced code review automation        Claude Code       team-rules      CC [ok]       2       │
│    gamma-skill        Multi-provider testing framework       Cursor            library         --            1       │
│    delta-skill        Documentation generator                Claude Code       team-rules      --            3       │
│    epsilon-skill      Performance profiling assistant         Gemini CLI        my-registry     --            2       │
│                                                                                                                       │
│                                                                                                                       │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ alpha-skill                                                                   [i] Install  [u] Uninstall  [p] Share  │
│ ──────────────────────────────────────────────────────────────────────────────────────────────────────────────────── │
│                                                                                                                       │
│ Files                                    │ Preview: SKILL.md                                                         │
│ ──────────────────────────────────────── │ ──────────────────────────────────────────────────────────────────────── │
│ > SKILL.md                        2 KB   │  1  ---                                                                   │
│   helpers.md                      800B   │  2  title: Alpha Skill                                                    │
│   utils/                                 │  3  description: A helpful skill that extends AI tool                     │
│     tool.md                       1 KB   │  4  providers: [claude-code, gemini-cli]                                  │
│     parser.md                     500B   │  5  ---                                                                   │
│                                          │  6                                                                        │
│                                          │  7  # Alpha Skill                                                         │
│                                          │  8                                                                        │
│                                          │  9  This skill extends AI tool capabilities by providing                  │
│                                          │ 10  structured workflows for common development tasks.                    │
│                                          │ 11                                                                        │
│                                          │ 12  ## Usage                                                              │
│                                          │ 13                                                                        │
│                                          │ 14  Invoke with `/alpha` in your AI coding tool.                          │
│                                          │ 15                                                                        │
│                                          │                                                                           │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ esc close drawer * h/l pane * j/k navigate * tab table/drawer * ? help                                   syllago    │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### 80x30 — Drawer open
```
╭──────────────────────────────────────────────────────────────────────────────────╮
│ SYL  Skills Agents MCP Rules Hooks Cmds │ Lib Reg Load │ [G] + Import  * New    │
╰──────────────────────────────────────────────────────────────────────────────────╯
┌──────────────────────────────────────────────────────────────────────────────────┐
│    Name             Desc                     Prov    Source    Inst   Files      │
│  > alpha-skill      A helpful skill th...    CC,GC   team-r   CC      4         │
│    beta-skill       Advanced code revi...    CC      team-r   CC      2         │
│    gamma-skill      Multi-provider tes...    Curs    librar   --      1         │
│    delta-skill      Documentation gen...     CC      team-r   --      3         │
│    epsilon-skill    Performance profi...     GC      my-reg   --      2         │
├──────────────────────────────────────────────────────────────────────────────────┤
│ alpha-skill                                      [i] Install  [u] Unin  [p] Shr │
│ ──────────────────────────────────────────────────────────────────────────────── │
│ Files                │ Preview: SKILL.md                                         │
│ ──────────────────── │ ─────────────────────────────────────────────────────── │
│ > SKILL.md     2KB   │  1  ---                                                  │
│   helpers.md   800B  │  2  title: Alpha Skill                                   │
│   utils/             │  3  description: A helpful skill                         │
│     tool.md    1KB   │  4  providers: [claude-code]                             │
│     parser.md  500B  │  5  ---                                                  │
│                      │  6                                                       │
│                      │  7  # Alpha Skill                                        │
│                      │  8                                                       │
│                      │  9  This skill extends AI tool                           │
│                      │ 10  capabilities by providing...                          │
├──────────────────────────────────────────────────────────────────────────────────┤
│ esc close * h/l pane * j/k nav * tab table/drawer * ? help           syllago    │
╰──────────────────────────────────────────────────────────────────────────────────╯
```

**Strengths:**
- Full-width table shows ALL metadata at a glance (name, description, provider, source, install status, file count)
- Excellent for comparing items — sortable columns possible in the future
- Drawer pattern is familiar (Slack threads, GitHub PR panels)
- When drawer is closed, maximum browsing density
- Drawer can be resized (drag or keyboard shortcut)
- Table stays visible even with drawer open — easy to switch items

**Weaknesses:**
- Drawer splits screen in half — less vertical space for both table and detail
- At 80 cols, table columns get heavily truncated
- Two interaction modes (table-only vs table+drawer) add conceptual complexity
- Drawer animation/transition could be distracting in a TUI

**Accessibility:**
- Table mode: j/k navigate, columns are screen-reader friendly
- Drawer: tab switches focus between table and drawer
- Esc always closes drawer (predictable escape hatch)
- Status column uses text+symbol (not just color)
- Sort columns with s+letter (future enhancement)

---

## Layout Comparison Matrix

| Aspect              | Studio (3-col) | Workbench (2+strip) | Explorer (master-detail) | Mosaic (4-quad) | Commander (table+drawer) |
|---------------------|----------------|---------------------|--------------------------|-----------------|--------------------------|
| Info density        | High           | Medium-High         | Medium                   | High            | Very High                |
| Pane count          | 3              | 2+strip             | 2 (with sub-split)       | 4               | 1-2 (adaptive)           |
| Nav complexity      | Medium         | Low                 | Medium                   | High            | Low-Medium               |
| 80-col viability    | Cramped        | Good                | Fair                     | Tight           | Good (table truncates)   |
| 120-col ideal       | Great          | Great               | Great                    | Best            | Best                     |
| Accessibility       | Good           | Best                | Good                     | Fair            | Good                     |
| Learning curve      | Low (IDE)      | Low                 | Low (email/Finder)       | Medium          | Low (table)              |
| Preview prominence  | Primary pane   | Primary pane        | Sub-pane                 | Quadrant        | Drawer sub-pane          |
| Browsing vs detail  | Balanced       | Balanced            | Detail-focused           | Balanced        | Browse-focused           |
| Files visibility    | Dedicated pane | Inline in tree      | Sub-pane                 | Dedicated quad  | Drawer sub-pane          |

## Design Notes

### Top Bar Responsive Behavior
- **120+ cols:** Full labels: `Skills  Agents  MCP  Rules  Hooks  Commands`
- **80-100 cols:** Abbreviated: `Skills Agents MCP Rules Hooks Cmds`
- **60-80 cols:** Content types in dropdown menu, only active type shown as label
- **<60 cols:** "Terminal too small" warning (existing behavior)

### Collection Tabs Behavior
When a collection tab is active (Library, Registries, Loadouts), the main content area changes:
- **Library:** Shows items across all content types with source badges
- **Registries:** Shows registry management (add, sync, browse)
- **Loadouts:** Shows loadout cards with apply/create actions

### Color Scheme
- Content type tabs: mint (`#6EE7B7` dark / `#047857` light)
- Collection tabs: purple/viola (`#C4B5FD` dark / `#6D28D9` light)
- Active tab: bold + underline + background highlight
- Inactive tab: muted text
- Import button: green accent
- Create button: purple accent
- Settings gear: muted
