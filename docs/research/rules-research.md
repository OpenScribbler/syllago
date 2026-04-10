# Rules: Cross-Provider Deep Dive

> Research compiled 2026-03-22. Covers rules systems across 13 AI coding tools.
> Output feeds into: converter/compat.go unified compat scorer (syllago-ohdb)

---

## Table of Contents

1. [Overview](#overview)
2. [Per-Provider Deep Dive](#per-provider-deep-dive)
   - [Claude Code](#claude-code)
   - [Cursor](#cursor)
   - [Gemini CLI](#gemini-cli)
   - [Copilot CLI (GitHub Copilot)](#copilot-cli-github-copilot)
   - [VS Code Copilot](#vs-code-copilot)
   - [Windsurf](#windsurf)
   - [Kiro](#kiro)
   - [Codex CLI](#codex-cli)
   - [Cline](#cline)
   - [OpenCode](#opencode)
   - [Roo Code](#roo-code)
   - [Zed](#zed)
   - [Amp](#amp)
3. [Cross-Platform Normalization Problem](#cross-platform-normalization-problem)
4. [Canonical Mapping](#canonical-mapping)
5. [Feature/Capability Matrix](#featurecapability-matrix)
6. [Compat Scoring Implications](#compat-scoring-implications)
7. [Converter Coverage Audit](#converter-coverage-audit)

---

## Overview

### Summary Table

| Provider | File Format | Scoping Model | Multi-file | Global Rules | Conditional Activation | Glob Support |
|----------|-------------|---------------|------------|--------------|----------------------|--------------|
| Claude Code | MD (YAML frontmatter) | paths frontmatter in `.claude/rules/` | Yes (dir, recursive) | Yes (`~/.claude/CLAUDE.md`, `~/.claude/rules/`) | Yes (paths globs) | Yes |
| Cursor | MDC (YAML frontmatter) | alwaysApply/globs/description | Yes (dir) | Yes (`~/.cursor/rules/`) | Yes (globs, alwaysApply, description) | Yes |
| Gemini CLI | MD (plain) | Hierarchical GEMINI.md files | Yes (hierarchy) | Yes (`~/.gemini/GEMINI.md`) | No | No |
| Copilot CLI | MD (YAML frontmatter) | AGENTS.md + .instructions.md | Yes (dir) | Yes (`$HOME/.copilot/copilot-instructions.md`) | Yes (applyTo globs) | Yes |
| VS Code Copilot | MD (YAML frontmatter) | .instructions.md + AGENTS.md + .claude/rules | Yes (dir) | Yes (user profile instructions) | Yes (applyTo globs) | Yes |
| Windsurf | MD (YAML frontmatter) | trigger-based (4 modes) | Yes (dir) | Yes (`~/.codeium/windsurf/memories/global_rules.md`) | Yes (trigger: glob/model_decision/manual) | Yes |
| Kiro | MD (YAML frontmatter) | inclusion modes (3 types) | Yes (dir) | Yes (`~/.kiro/steering/`) | Yes (fileMatch patterns) | Yes |
| Codex CLI | MD (plain) | AGENTS.md hierarchy | Yes (hierarchy) | Yes (`~/.codex/AGENTS.md`) | No | No |
| Cline | MD (YAML frontmatter) | paths frontmatter | Yes (dir) | Yes (`~/Documents/Cline/Rules/`) | Yes (paths globs) | Yes |
| OpenCode | MD (plain) | AGENTS.md + opencode.json refs | Yes (via config) | Yes (`~/.config/opencode/AGENTS.md`) | No | No (via config globs only) |
| Roo Code | MD/TXT (plain) | Mode-based dirs | Yes (dir, recursive) | Yes (`~/.roo/rules/`, `~/.roo/rules-{mode}/`) | Yes (mode-based scoping) | No |
| Zed | Plain text | Single .rules file + Rules Library | No (single file) | Yes (Rules Library defaults) | No | No |
| Amp | MD (YAML frontmatter) | AGENTS.md + globs frontmatter | Yes (hierarchy) | Yes (`~/.config/amp/AGENTS.md`) | Yes (globs frontmatter) | Yes |

---

## Per-Provider Deep Dive

### Claude Code

**Format:** Markdown with optional YAML frontmatter (paths field)

**File Naming:** `CLAUDE.md` (main instructions), `.claude/rules/*.md` (modular rules)

**Directory Structure:**
```
project-root/
├── CLAUDE.md                    # or .claude/CLAUDE.md
└── .claude/
    ├── CLAUDE.md                # alternative location
    └── rules/
        ├── code-style.md        # always-apply (no frontmatter)
        ├── testing.md
        ├── security.md
        └── frontend/            # subdirs supported (recursive)
            ├── react.md
            └── styles.md
```

**Configuration Locations (by scope):**

| Scope | Path | Purpose |
|-------|------|---------|
| Managed policy | `/Library/Application Support/ClaudeCode/CLAUDE.md` (macOS), `/etc/claude-code/CLAUDE.md` (Linux) | Org-wide, cannot be excluded |
| Project | `./CLAUDE.md` or `./.claude/CLAUDE.md` | Team-shared via source control |
| User | `~/.claude/CLAUDE.md` | Personal, all projects |
| User rules | `~/.claude/rules/*.md` | Personal modular rules |

**Frontmatter Fields:**
```yaml
---
paths:
  - "src/api/**/*.ts"
  - "src/**/*.{ts,tsx}"
---
```
- `paths` ([]string): Glob patterns for conditional activation. Supports brace expansion.
- No other frontmatter fields are recognized. Rules without `paths:` are always-apply.

**Precedence:** Managed policy > project rules > user rules. CLAUDE.md files load by walking up from CWD. Subdirectory CLAUDE.md files load on-demand when Claude reads files there.

**Scoping:**
- **Always-apply:** No frontmatter, or no `paths:` field. Loaded at session start.
- **Path-scoped:** Has `paths:` frontmatter. Loaded when Claude reads matching files.

**Unique Capabilities:**
- `@path/to/file` import syntax in CLAUDE.md (up to 5 levels deep)
- `claudeMdExcludes` setting to skip irrelevant CLAUDE.md files in monorepos
- `/init` command auto-generates CLAUDE.md by analyzing codebase
- Auto memory system (separate from rules -- Claude writes its own notes)
- `InstructionsLoaded` hook for debugging which rules loaded
- Symlink support in `.claude/rules/` with cycle detection
- Target: under 200 lines per CLAUDE.md file

**Example (path-scoped rule):**
```markdown
---
paths:
  - "src/api/**/*.ts"
---

# API Development Rules

- All API endpoints must include input validation
- Use the standard error response format
- Include OpenAPI documentation comments
```

**Sources:**
- [Claude Code Memory docs](https://code.claude.com/docs/en/memory)
- [Claude Code Rules Directory guide](https://claudefa.st/blog/guide/mechanics/rules-directory)

---

### Cursor

**Format:** MDC (Markdown with YAML frontmatter), `.mdc` extension

**File Naming:** `*.mdc` files in `.cursor/rules/`, kebab-case filenames

**Directory Structure:**
```
project-root/
└── .cursor/
    └── rules/
        ├── code-style.mdc
        ├── testing.mdc
        └── react-patterns.mdc
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Project | `.cursor/rules/*.mdc` |
| Global | `~/.cursor/rules/*.mdc` |

**Frontmatter Fields:**
```yaml
---
description: "Brief description of what this rule covers"
alwaysApply: true
globs: "**/*.tsx, **/*.ts"
---
```
- `description` (string): Concise explanation; also serves as activation hint for "Agent" type rules
- `alwaysApply` (bool): Whether the rule is always in context
- `globs` (string): Comma-separated glob patterns for auto-attach

**Four Rule Types (determined by frontmatter):**

| Type | Condition | Behavior |
|------|-----------|----------|
| Always | `alwaysApply: true` | Always in context |
| Auto-Attach | `globs` set, no `alwaysApply` | Loads when matching file is active |
| Agent | `description` set, no globs | AI decides whether to apply based on description |
| Manual | None of the above | Must be referenced manually (e.g., `@rulename`) |

**Precedence:** When both `alwaysApply: true` and `globs` are set, globs are ignored (treated as always-apply).

**Unique Capabilities:**
- The MDC format is Cursor-specific; no other provider uses `.mdc`
- "Agent" rule type where AI decides activation based on description
- Community recommends short rules (target 25 lines, max 50 lines)

**Example:**
```mdc
---
description: "React component conventions"
globs: "**/*.tsx"
alwaysApply: false
---

# React Component Rules

- Use functional components with hooks
- Extract custom hooks for shared logic
- Use TypeScript for all component props
```

**Sources:**
- [Cursor Rules Deep Dive](https://forum.cursor.com/t/a-deep-dive-into-cursor-rules-0-45/60721)
- [MDC Best Practices](https://forum.cursor.com/t/my-best-practices-for-mdc-rules-and-troubleshooting/50526)

---

### Gemini CLI

**Format:** Markdown (plain, no frontmatter support)

**File Naming:** `GEMINI.md` (default, configurable in settings.json)

**Directory Structure:**
```
~/.gemini/
└── GEMINI.md              # Global instructions

project-root/
├── GEMINI.md              # Project-level
└── src/
    └── GEMINI.md          # Subdirectory-level
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Global | `~/.gemini/GEMINI.md` |
| Project | `./GEMINI.md` (walks up to `.git` root) |
| Subdirectory | Any `GEMINI.md` in subdirs below CWD |

**Scoping:** All GEMINI.md files are always-on. No conditional activation. Content is concatenated hierarchically (global + project-root-to-CWD + subdirectory files).

**Unique Capabilities:**
- `@file.md` import syntax (relative and absolute paths)
- `/memory show`, `/memory reload`, `/memory add` commands
- Custom filename support via `settings.json`
- `.geminiignore` support for subdirectory scanning
- `GEMINI_SYSTEM_MD` env var for system prompt override (loads `system.md` from `.gemini/`)
- Hierarchical concatenation: all found files combined and sent with every prompt

**Example:**
```markdown
# Project Guidelines

- Use TypeScript with strict mode
- Follow functional programming patterns
- Write unit tests for all utility functions
```

**Sources:**
- [Gemini CLI GEMINI.md docs](https://geminicli.com/docs/cli/gemini-md/)
- [Google Codelabs: Gemini CLI](https://codelabs.developers.google.com/gemini-cli-hands-on)

---

### Copilot CLI (GitHub Copilot)

**Format:** Markdown with optional YAML frontmatter (for .instructions.md files)

**File Naming:** `copilot-instructions.md`, `*.instructions.md`, `AGENTS.md`

**Directory Structure:**
```
project-root/
├── .github/
│   ├── copilot-instructions.md       # Repository-wide
│   └── instructions/
│       ├── react.instructions.md     # Path-specific
│       └── testing/
│           └── unit.instructions.md
├── AGENTS.md                          # Primary instructions
├── CLAUDE.md                          # Cross-tool compat
└── GEMINI.md                          # Cross-tool compat
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Personal | `$HOME/.copilot/copilot-instructions.md` |
| Repository-wide | `.github/copilot-instructions.md` |
| Path-specific | `.github/instructions/*.instructions.md` |
| Agent instructions | `AGENTS.md` (root or CWD) |
| Custom dirs | `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` env var |

**Frontmatter Fields (for .instructions.md files):**
```yaml
---
applyTo: "**/*.ts,**/*.tsx"
---
```
- `applyTo` (string): Comma-separated glob patterns. Required for conditional activation.

**Precedence:** Personal > Repository > Organization. Both `.github/copilot-instructions.md` and matching `.instructions.md` files combine when applicable.

**Unique Capabilities:**
- Cross-tool file support: recognizes `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`
- `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` env var for additional instruction directories
- `/init` command for auto-generating instructions
- Organization-level instructions (GitHub org settings)
- `applyTo` uses comma-separated globs (not YAML array)

**Example (path-specific .instructions.md):**
```markdown
---
applyTo: "**/*.py"
---

# Python Coding Standards

- Follow PEP 8 style guide
- Use type hints for all function signatures
- Write docstrings for public functions
```

**Sources:**
- [Copilot CLI Custom Instructions](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions)
- [GitHub Copilot CLI GA](https://github.blog/changelog/2026-02-25-github-copilot-cli-is-now-generally-available/)

---

### VS Code Copilot

**Format:** Markdown with YAML frontmatter

**File Naming:** `*.instructions.md`, `copilot-instructions.md`, `AGENTS.md`, `CLAUDE.md`

**Directory Structure:**
```
project-root/
├── .github/
│   ├── copilot-instructions.md          # Always-on, project-wide
│   └── instructions/
│       ├── react.instructions.md        # Path-specific
│       └── api/
│           └── design.instructions.md
├── .claude/
│   └── rules/
│       └── style.md                     # Claude-compat (uses paths: instead of applyTo:)
├── AGENTS.md                            # Cross-tool
└── CLAUDE.md                            # Cross-tool
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| User instructions | `~/.copilot/instructions/`, `~/.claude/rules/`, profile instructions folder |
| Workspace | `.github/instructions/*.instructions.md` |
| Always-on | `.github/copilot-instructions.md`, `AGENTS.md`, `CLAUDE.md` |
| Organization | GitHub org-level (auto-discovered) |

**Frontmatter Fields:**
```yaml
---
name: "React Standards"
description: "React/TSX conventions"
applyTo: "**/*.tsx"
excludeAgent: "code-review"
---
```
- `name` (string): Display name in UI (defaults to filename)
- `description` (string): Tooltip/hover text
- `applyTo` (string): Glob pattern(s), comma-separated. Omit or `"**"` for all files.
- `excludeAgent` (string): `"code-review"` or `"coding-agent"` to restrict scope

**Claude Rules Compatibility:**
VS Code Copilot recognizes `.claude/rules/*.md` files but uses `paths` (array) instead of `applyTo` (string) for globs, following Claude Code's format.

**Precedence:** Personal > Repository > Organization

**Unique Capabilities:**
- `/create-instruction` to generate instructions via AI
- Settings Sync for user instructions across devices
- Diagnostics view (right-click Chat > Diagnostics) shows all loaded files
- `chat.useAgentsMdFile`, `chat.useClaudeMdFile` toggle settings
- `chat.instructionsFilesLocations` for custom instruction directories
- Semantic task matching (not just glob matching) for file-based instructions
- `excludeAgent` field to scope rules to specific agent types

**Example:**
```markdown
---
name: "API Design Standards"
description: "REST API conventions"
applyTo: "src/api/**/*.ts"
---

# API Standards

- Use consistent error response format
- Include OpenAPI documentation
- Validate all input parameters
```

**Sources:**
- [VS Code Copilot Custom Instructions](https://code.visualstudio.com/docs/copilot/customization/custom-instructions)
- [GitHub Copilot Instructions Guide](https://docs.github.com/copilot/customizing-copilot/adding-custom-instructions-for-github-copilot)

---

### Windsurf

**Format:** Markdown with YAML frontmatter (trigger field)

**File Naming:** `*.md` files in `.windsurf/rules/`

**Directory Structure:**
```
project-root/
├── .windsurf/
│   └── rules/
│       ├── code-style.md
│       ├── testing.md
│       └── security.md
└── AGENTS.md                    # Also recognized (always-on if at root)
```

**Configuration Locations:**

| Scope | Path | Limit |
|-------|------|-------|
| Global | `~/.codeium/windsurf/memories/global_rules.md` | 6,000 chars |
| Workspace | `.windsurf/rules/*.md` | 12,000 chars per file |
| System (enterprise) | `/Library/Application Support/Windsurf/rules/*.md` (macOS), `/etc/windsurf/rules/*.md` (Linux) | Read-only |

**Frontmatter Fields:**
```yaml
---
trigger: glob
description: "Testing conventions for test files"
globs: "**/*.test.ts"
---
```
- `trigger` (string): One of `always_on`, `glob`, `model_decision`, `manual`
- `description` (string): Description; used as activation hint for `model_decision` trigger
- `globs` (string): File pattern(s) for `glob` trigger type

**Four Trigger Types:**

| Trigger | Behavior | Context Cost |
|---------|----------|-------------|
| `always_on` | Full content in system prompt every message | Always |
| `glob` | Activates when Cascade accesses matching files | On match |
| `model_decision` | Only description shown initially; full content loads when AI deems relevant | Minimal until triggered |
| `manual` | User activates via `@rule-name` in input | On demand |

**Precedence:** Auto-discovers from workspace, subdirectories, and git parent directories. Root-level `AGENTS.md` is always-on.

**Unique Capabilities:**
- `model_decision` trigger type (AI decides based on description -- unique to Windsurf)
- Character limits per file (12K workspace, 6K global)
- Enterprise system-level rules via OS paths
- `rules_applied` field in `post_cascade_response` hook (tracks which rules fired)
- `AGENTS.md` at root treated as always-on; in subdirectories, auto-scoped by glob

**Example:**
```markdown
---
trigger: model_decision
description: "Database migration conventions and schema design patterns"
---

# Database Migration Rules

- Always create reversible migrations
- Use explicit column types, never inferred
- Include both up and down migration scripts
```

**Sources:**
- [Windsurf Cascade Memories docs](https://docs.windsurf.com/windsurf/cascade/memories)
- [Windsurf Rules Directory](https://windsurf.com/editor/directory)

---

### Kiro

**Format:** Markdown with YAML frontmatter

**File Naming:** `*.md` files in `.kiro/steering/`

**Directory Structure:**
```
project-root/
└── .kiro/
    └── steering/
        ├── product.md           # Foundation: product overview
        ├── tech.md              # Foundation: tech stack
        ├── structure.md         # Foundation: project structure
        ├── security.md          # Custom steering
        └── deployment.md        # Custom steering
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Workspace | `.kiro/steering/*.md` |
| Global | `~/.kiro/steering/*.md` |

**Frontmatter Fields:**
```yaml
---
inclusion: fileMatch
fileMatchPattern: "components/**/*.tsx"
name: "React Component Standards"
description: "Conventions for React components"
---
```
- `inclusion` (string): One of `auto` (always), `fileMatch` (conditional), `manual` (on-demand). Default: `always`
- `fileMatchPattern` (string or []string): Glob patterns for `fileMatch` mode
- `name` (string): Display name (required for `auto` mode)
- `description` (string): Description (required for `auto` mode)

**Three Inclusion Modes:**

| Mode | Behavior | Activation |
|------|----------|------------|
| `auto` / `always` | Loaded every interaction | Automatic |
| `fileMatch` | Loaded when working with matching files | Conditional |
| `manual` | Available via `#steering-name` or `/` slash commands | On-demand |

**Precedence:** Workspace steering overrides global steering on conflict.

**Unique Capabilities:**
- Foundation steering files auto-generated: `product.md`, `tech.md`, `structure.md`
- `#[[file:path]]` syntax for live file references in steering docs
- Manual steering available as slash commands (`/steering-name`)
- `AGENTS.md` also recognized (no inclusion mode support)
- Team steering via MDM/Group Policy deployment to `~/.kiro/steering/`

**Example:**
```markdown
---
inclusion: fileMatch
fileMatchPattern:
  - "**/*.ts"
  - "**/*.tsx"
---

# TypeScript Conventions

- Use strict TypeScript configuration
- Prefer interfaces over type aliases for object shapes
- Use const assertions for literal types
```

**Sources:**
- [Kiro Steering docs](https://kiro.dev/docs/steering/)
- [Kiro CLI Steering docs](https://kiro.dev/docs/cli/steering/)
- [Kiro Blog: Agent Steering and MCP](https://kiro.dev/blog/teaching-kiro-new-tricks-with-agent-steering-and-mcp/)

---

### Codex CLI

**Format:** Markdown (plain, no frontmatter support for content rules)

**File Naming:** `AGENTS.md`, `AGENTS.override.md`

**Directory Structure:**
```
~/.codex/
├── AGENTS.md                    # Global defaults
├── AGENTS.override.md           # Global override (takes precedence)
└── config.toml                  # Configuration

project-root/
├── AGENTS.md                    # Project-level
├── AGENTS.override.md           # Project override
└── services/
    └── payments/
        └── AGENTS.override.md   # Nested override
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Global | `~/.codex/AGENTS.md` or `~/.codex/AGENTS.override.md` |
| Project | `AGENTS.md` or `AGENTS.override.md` at git root, walking down to CWD |

**Discovery & Precedence:**
1. Global: checks `AGENTS.override.md` first, then `AGENTS.md` (first non-empty wins)
2. Project: walks from git root to CWD, checking each directory
3. At each level: `AGENTS.override.md` > `AGENTS.md` > fallback filenames
4. Concatenates root-to-leaf with blank lines; later files override earlier
5. Max combined size: `project_doc_max_bytes` (default 32 KiB)

**Unique Capabilities:**
- `AGENTS.override.md` pattern for local overrides without modifying shared files
- `CODEX_HOME` env var for profile-specific instruction sets
- Configurable fallback filenames via `project_doc_fallback_filenames` in config.toml
- `.rules` files for command execution control (experimental) -- separate from content rules
- Max bytes limit (32 KiB default, configurable) prevents context bloat
- Each discovered file becomes a separate user-role message in the prompt

**Example:**
```markdown
# Project Guidelines

## Code Style
- Use TypeScript with strict mode enabled
- Prefer functional programming patterns
- All public APIs must have JSDoc documentation

## Testing
- Run `npm test` before committing
- Maintain >80% code coverage
```

**Sources:**
- [Codex AGENTS.md Guide](https://developers.openai.com/codex/guides/agents-md)
- [Codex Rules](https://developers.openai.com/codex/rules)
- [Codex Config Basics](https://developers.openai.com/codex/config-basic)

---

### Cline

**Format:** Markdown with optional YAML frontmatter

**File Naming:** `*.md` or `*.txt` files in `.clinerules/`

**Directory Structure:**
```
project-root/
├── .clinerules/
│   ├── 01-coding-style.md       # Numbered for ordering
│   ├── 02-testing.md
│   ├── react-patterns.md
│   └── workflows/
│       └── deployment.md
└── .cursorrules                  # Compat: auto-detected
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Global (Windows) | `Documents\Cline\Rules\` |
| Global (macOS/Linux) | `~/Documents/Cline/Rules/` or `~/Cline/Rules/` |
| Workspace | `.clinerules/` at project root |

**Frontmatter Fields:**
```yaml
---
paths:
  - "src/components/**"
  - "src/hooks/**"
---
```
- `paths` ([]string): Glob patterns for conditional activation. Only supported conditional field.

**Scoping:**
- **Always-on:** No frontmatter. Loaded for every task.
- **Path-conditional:** Has `paths:` frontmatter. Activates based on open files, visible tabs, mentioned paths, edited files, and pending operations.

**Glob Syntax:** `*` (characters except `/`), `**` (recursive), `?` (single char), `[abc]` (bracket), `{a,b}` (alternatives).

**Unique Capabilities:**
- Cross-tool compat: auto-detects `.cursorrules`, `.windsurfrules`, `AGENTS.md`
- Individual toggle (enable/disable) per rule without deletion
- AI-editable: Cline can read/write/edit its own rules
- Invalid YAML fails open (rule activates with raw content visible)
- Pending operations trigger conditional rules (not just current context)
- Community frontmatter extensions: `description`, `author`, `version`, `globs`, `tags`

**Example:**
```markdown
---
paths:
  - "src/api/**"
  - "src/middleware/**"
---

# Backend API Rules

- Use Express.js middleware pattern
- Validate all request bodies with Zod
- Return consistent error response format
```

**Sources:**
- [Cline Rules docs](https://docs.cline.bot/customization/cline-rules)
- [Cline Blog: .clinerules](https://cline.bot/blog/clinerules-version-controlled-shareable-and-ai-editable-instructions)

---

### OpenCode

**Format:** Markdown (plain, no frontmatter in AGENTS.md)

**File Naming:** `AGENTS.md` (primary), `CLAUDE.md` (fallback)

**Directory Structure:**
```
project-root/
├── AGENTS.md                # Project-level instructions
└── opencode.json            # Can reference additional instruction files

~/.config/opencode/
├── AGENTS.md                # Global instructions
└── opencode.json            # Global config with instructions field
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Project | `AGENTS.md` in project root |
| Global | `~/.config/opencode/AGENTS.md` |
| Fallback | `CLAUDE.md` (project), `~/.claude/CLAUDE.md` (global) |

**Precedence:** `AGENTS.md` takes precedence over `CLAUDE.md` in the same directory. Project files override global files.

**Instructions via opencode.json:**
```json
{
  "instructions": [
    "CONTRIBUTING.md",
    "docs/guidelines.md",
    ".cursor/rules/*.md",
    "https://example.com/standards.md"
  ]
}
```
- Supports glob patterns and remote URLs (5-second fetch timeout)
- Recommended approach for monorepos with `packages/*/AGENTS.md`

**Unique Capabilities:**
- `/init` command to auto-generate AGENTS.md by scanning project
- Remote URL instruction sources in opencode.json
- Glob patterns in instruction references
- Claude Code compatibility mode (recognizes `CLAUDE.md`, `.claude/CLAUDE.md`)
- Disableable via `OPENCODE_DISABLE_CLAUDE_CODE` env var

**Example:**
```markdown
# Project Standards

## Architecture
- Use Clean Architecture with dependency inversion
- Database access only through repository interfaces

## Code Quality
- All functions must have error handling
- Use structured logging (not fmt.Println)
```

**Sources:**
- [OpenCode Rules docs](https://opencode.ai/docs/rules/)
- [OpenCode Config docs](https://opencode.ai/docs/config/)

---

### Roo Code

**Format:** Markdown or plain text (`.md`, `.txt`), no frontmatter

**File Naming:** Files in `.roo/rules/` and `.roo/rules-{mode-slug}/`

**Directory Structure:**
```
project-root/
└── .roo/
    ├── rules/                   # Shared across all modes
    │   ├── 01-general.md
    │   └── 02-coding-style.md
    ├── rules-code/              # Code mode only
    │   ├── 01-js-style.md
    │   └── 02-ts-style.md
    ├── rules-architect/         # Architect mode only
    ├── rules-docs-writer/       # Custom mode
    └── modes/                   # Mode system prompts
        └── code/
            └── system_prompt.md
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Global shared | `~/.roo/rules/` |
| Global mode-specific | `~/.roo/rules-{modeSlug}/` |
| Workspace shared | `.roo/rules/` |
| Workspace mode-specific | `.roo/rules-{mode-slug}/` |
| Legacy fallback | `.roorules`, `.clinerules` (root file) |

**Mode-Based Scoping:**
- Rules in `.roo/rules/` apply to ALL modes
- Rules in `.roo/rules-{mode}/` apply only to that mode
- Built-in modes: `code`, `architect`, `ask`, `debug`
- Custom modes with custom slugs supported
- Directory-based rules override root-level `.roorules-{mode}` files

**Loading Order:**
1. Global rules first, then workspace rules
2. Mode-specific rules loaded before general rules
3. Files within directories loaded alphabetically by filename
4. Workspace rules override global on conflict

**Unique Capabilities:**
- Mode-based scoping is unique to Roo Code (no other provider has this)
- Supports both `.md` and `.txt` files
- Recursive subdirectory scanning
- Symlink support with cycle detection
- Legacy `.clinerules` fallback compatibility
- Custom system prompts per mode (`.roo/modes/{slug}/system_prompt.md`)

**Example:**
```markdown
# TypeScript Code Style (rules-code/)

- Use strict TypeScript configuration
- Prefer functional components in React
- All API calls must use the typed client
```

**Sources:**
- [Roo Code Custom Instructions](https://docs.roocode.com/features/custom-instructions)
- [Roo Code Custom Modes](https://docs.roocode.com/features/custom-modes)

---

### Zed

**Format:** Plain text (no frontmatter, no markdown requirement)

**File Naming:** `.rules` (primary), also recognizes `.cursorrules`, `.windsurfrules`, `.clinerules`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.github/copilot-instructions.md`

**Directory Structure:**
```
project-root/
├── .rules                       # Primary (first match wins)
├── .cursorrules                 # Fallback
├── AGENTS.md                    # Fallback
└── CLAUDE.md                    # Fallback
```

**File Priority Order:**
1. `.rules`
2. `.cursorrules`
3. `.windsurfrules`
4. `.clinerules`
5. `.github/copilot-instructions.md`
6. `AGENT.md`
7. `AGENTS.md`
8. `CLAUDE.md`
9. `GEMINI.md`

Only the first matching file is loaded. No multi-file support.

**Rules Library:**
- Built-in editor with syntax highlighting
- Access: Agent Panel menu > Rules, or `cmd-alt-l`/`ctrl-alt-l`
- Rules can be set as "default" (auto-included in every interaction)
- Rules referenced via `@rulename` in agent panel

**Scoping:** Single file, always-on. No conditional activation, no glob support.

**Unique Capabilities:**
- Broadest cross-tool file compatibility (9 file names)
- Built-in Rules Library editor UI
- Default rules (auto-included via paperclip icon)
- Legacy "Prompt Library" backwards compatibility

**Example (.rules file):**
```
Use Rust for all new code.
Follow the project's existing patterns for error handling.
Prefer iterators over manual loops.
Write tests for all public functions.
```

**Sources:**
- [Zed AI Rules docs](https://zed.dev/docs/ai/rules)
- [Zed AI Overview](https://zed.dev/docs/ai/overview)

---

### Amp

**Format:** Markdown with optional YAML frontmatter (globs field)

**File Naming:** `AGENTS.md` (primary), `AGENT.md` / `CLAUDE.md` (legacy fallback)

**Directory Structure:**
```
project-root/
├── AGENTS.md                    # Project root (always included)
├── src/
│   └── AGENTS.md                # Subtree (included when files read here)
└── .agents/
    ├── skills/                  # Project skills
    │   └── my-skill/
    │       └── SKILL.md
    └── checks/                  # Code review checks
        └── performance.md

~/.config/amp/
├── AGENTS.md                    # Global
└── settings.json               # Permissions, MCP config
```

**Configuration Locations:**

| Scope | Path |
|-------|------|
| Global | `~/.config/amp/AGENTS.md`, `~/.config/AGENTS.md` |
| Project root | `AGENTS.md` in CWD and parent dirs up to `$HOME` |
| Subtree | `AGENTS.md` in subdirectories (loaded when agent reads files there) |

**Frontmatter Fields:**
```yaml
---
globs:
  - "**/*.ts"
  - "**/*.tsx"
---
```
- `globs` ([]string): File patterns for conditional activation. Implicitly prefixed with `**/` unless starting with `../` or `./`.

**Precedence:** Parent directory AGENTS.md files always included. Subtree files included when agent reads files in that subtree. `AGENT.md` or `CLAUDE.md` used as fallback if no `AGENTS.md` exists.

**Unique Capabilities:**
- `@filepath` references in AGENTS.md (relative, absolute, or `@~/` paths)
- Glob frontmatter in AGENTS.md files (not separate rule files)
- Permission rules system (`amp.permissions` in settings)
- Skills system (`.agents/skills/`) replaces custom commands
- Code review checks (`.agents/checks/`) with severity and tool scoping
- Delegation in permission rules (external program decides allow/reject)
- Broad migration support: recognizes `CLAUDE.md`, `AGENT.md` as fallbacks

**Example (glob-scoped AGENTS.md):**
```markdown
---
globs:
  - "src/frontend/**"
  - "**/*.tsx"
---

# Frontend Conventions

- Use React functional components
- State management via Zustand
- CSS via Tailwind utility classes

See @docs/component-guidelines.md for detailed patterns.
```

**Sources:**
- [Amp Owner's Manual](https://ampcode.com/manual)
- [Amp AI Review](https://www.secondtalent.com/resources/amp-ai-review/)

---

## Cross-Platform Normalization Problem

### What Differs

**1. Activation Model Terminology**

Every provider has a different vocabulary for the same concepts:

| Concept | Claude Code | Cursor | Windsurf | Kiro | Copilot | Cline | Amp |
|---------|-------------|--------|----------|------|---------|-------|-----|
| Always active | (no frontmatter) | `alwaysApply: true` | `trigger: always_on` | `inclusion: auto` | (no applyTo) | (no frontmatter) | (no globs) |
| File-conditional | `paths: [...]` | `globs: "..."` | `trigger: glob` + `globs:` | `inclusion: fileMatch` + `fileMatchPattern:` | `applyTo: "..."` | `paths: [...]` | `globs: [...]` |
| AI-decided | N/A | `description:` (Agent type) | `trigger: model_decision` | N/A | N/A | N/A | N/A |
| Manual/on-demand | N/A | (no fields = Manual) | `trigger: manual` | `inclusion: manual` | N/A | N/A | N/A |

**2. Glob Field Format**

| Provider | Field Name | Type | Example |
|----------|-----------|------|---------|
| Claude Code | `paths` | YAML array | `paths: ["src/**/*.ts"]` |
| Cursor | `globs` | String (comma-separated) | `globs: "**/*.ts, **/*.tsx"` |
| Windsurf | `globs` | String | `globs: "**/*.test.ts"` |
| Kiro | `fileMatchPattern` | String or array | `fileMatchPattern: ["**/*.ts"]` |
| VS Code Copilot | `applyTo` | String (comma-separated) | `applyTo: "**/*.ts,**/*.tsx"` |
| Copilot CLI | `applyTo` | String (comma-separated) | `applyTo: "**/*.ts"` |
| Cline | `paths` | YAML array | `paths: ["src/**"]` |
| Amp | `globs` | YAML array | `globs: ["**/*.ts"]` |

**3. File Extension & Format**

| Provider | Extension | Frontmatter Style |
|----------|-----------|-------------------|
| Cursor | `.mdc` | YAML (alwaysApply, globs, description) |
| Windsurf | `.md` | YAML (trigger, globs, description) |
| Kiro | `.md` | YAML (inclusion, fileMatchPattern, name, description) |
| Claude Code | `.md` | YAML (paths only) |
| Copilot CLI/VS Code | `.instructions.md` | YAML (applyTo, name, description, excludeAgent) |
| Cline | `.md` | YAML (paths only) |
| Amp | `.md` | YAML (globs only) |
| Zed | `.rules` | None |
| Gemini CLI | `.md` (GEMINI.md) | None |
| Codex CLI | `.md` (AGENTS.md) | None |
| OpenCode | `.md` (AGENTS.md) | None |
| Roo Code | `.md` / `.txt` | None |

### The Asymmetry Problem

Not all providers can express the same concepts, creating lossy conversions:

**Features that only some providers support:**

| Feature | Who Has It | Who Doesn't |
|---------|-----------|-------------|
| AI-decided activation | Cursor (Agent type), Windsurf (model_decision) | Everyone else |
| Mode-based scoping | Roo Code | Everyone else |
| Manual/on-demand rules | Cursor, Windsurf, Kiro | Claude Code, Copilot, Cline, Amp, Codex, Gemini, Zed |
| Description as activation hint | Cursor, Windsurf, Kiro, VS Code Copilot | Claude Code, Cline, Amp |
| File imports (@syntax) | Claude Code, Gemini CLI, Amp | Cursor, Windsurf, Kiro, Cline, Roo Code, Zed, Codex |
| Character/size limits | Windsurf (12K/6K), Codex (32K) | Others (no explicit limits) |
| Override files | Codex (AGENTS.override.md) | Everyone else |
| excludeAgent | VS Code Copilot | Everyone else |

**Conversion loss scenarios:**
- Windsurf `model_decision` rule --> Claude Code: loses the AI-decided semantics, becomes either always-on or description-as-prose
- Roo Code mode-specific rule --> any other provider: mode concept doesn't exist, becomes always-on with warning
- Cursor "Agent" type rule --> providers without description field: description becomes prose comment
- Kiro `manual` inclusion --> providers without manual mode: becomes always-on (over-includes) or dropped

### What's Universal

Despite differences, every provider shares these fundamentals:

1. **Markdown body content** -- all providers use markdown (or plain text) for rule content
2. **Project-level placement** -- every provider has a project-root location for rules
3. **Always-on capability** -- every provider can express "this rule always applies"
4. **Human-readable format** -- all are text files, no binary formats

The universal subset is: **a plain markdown file with no frontmatter, placed in the project root, that always applies**. This is the lowest common denominator for cross-provider rules.

---

## Canonical Mapping

### Syllago Canonical Format

```yaml
---
description: "Brief description of the rule"
alwaysApply: true
globs:
  - "src/**/*.ts"
  - "**/*.tsx"
---

# Rule Content

Markdown body with the actual instructions.
```

### Field Mapping Table

| Canonical Field | Claude Code | Cursor | Windsurf | Kiro | Copilot/VS Code | Cline | Amp | Codex | Gemini | OpenCode | Roo Code | Zed |
|----------------|-------------|--------|----------|------|-----------------|-------|-----|-------|--------|----------|----------|-----|
| `description` | (prose) | `description` | `description` | `name`+`description` | `name`+`description` | (prose) | (prose) | (prose) | (prose) | (prose) | (prose) | (HTML comment) |
| `alwaysApply: true` | no frontmatter | `alwaysApply: true` | `trigger: always_on` | `inclusion: auto` | no `applyTo` | no frontmatter | no `globs` | (default) | (default) | (default) | `.roo/rules/` dir | (default) |
| `alwaysApply: false` | has `paths:` | `alwaysApply: false` | trigger != `always_on` | `inclusion: fileMatch` or `manual` | has `applyTo` | has `paths:` | has `globs:` | N/A | N/A | N/A | `.roo/rules-{mode}/` | N/A |
| `globs: [...]` | `paths: [...]` | `globs: "csv"` | `globs: "str"` | `fileMatchPattern: [...]` | `applyTo: "csv"` | `paths: [...]` | `globs: [...]` | (prose) | N/A | N/A | N/A | N/A |

**Key:** "csv" = comma-separated string, "str" = single string, "[...]" = YAML array, "(prose)" = embedded as natural language text, "(default)" = implicit always-on, "N/A" = not supported

---

## Feature/Capability Matrix

### Feature Definitions

| Feature | Description |
|---------|-------------|
| **Multi-file rules** | Multiple rule files in a directory, each loaded independently |
| **Glob-scoped activation** | Rules activate only when working with files matching glob patterns |
| **AI-decided activation** | AI model decides whether to apply a rule based on description |
| **Manual/on-demand** | Rules explicitly invoked by user reference |
| **Description field** | Structured metadata describing what the rule does |
| **Global rules** | User-level rules that apply across all projects |
| **Hierarchical loading** | Rules discovered by walking directory tree |
| **File imports** | Reference other files from within a rule/instruction file |
| **Cross-tool compat** | Recognizes rule files from other providers |
| **Enterprise/managed** | Organization-deployed rules that users cannot override |
| **Override mechanism** | Ability to override shared rules with local overrides |
| **Size limits** | Enforced maximum size for rule content |

### Provider Support Matrix

| Feature | Claude | Cursor | Gemini | Copilot CLI | VS Code | Windsurf | Kiro | Codex | Cline | OpenCode | Roo | Zed | Amp |
|---------|--------|--------|--------|-------------|---------|----------|------|-------|-------|----------|-----|-----|-----|
| Multi-file rules | Y | Y | Y (hierarchy) | Y | Y | Y | Y | Y (hierarchy) | Y | N | Y | N | Y (hierarchy) |
| Glob-scoped | Y | Y | N | Y | Y | Y | Y | N | Y | N | N | N | Y |
| AI-decided | N | Y | N | N | N | Y | N | N | N | N | N | N | N |
| Manual/on-demand | N | Y | N | N | N | Y | Y | N | N | N | N | Y (@) | N |
| Description field | N | Y | N | N | Y | Y | Y | N | N | N | N | N | N |
| Global rules | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y (library) | Y |
| Hierarchical loading | Y | N | Y | Y | N | Y | N | Y | N | N | N | N | Y |
| File imports | Y (@) | N | Y (@) | N | Y (links) | N | Y (#[[file:]]) | N | N | Y (json) | N | N | Y (@) |
| Cross-tool compat | N | N | N | Y | Y | Y | Y | N | Y | Y | Y | Y | Y |
| Enterprise/managed | Y | N | N | N | Y (org) | Y | Y (MDM) | N | N | N | N | N | N |
| Override mechanism | N | N | N | N | N | N | N | Y | N | N | N | N | N |
| Size limits | Soft (200L) | Soft (50L) | N | N | N | Y (12K/6K) | N | Y (32KB) | N | N | N | N | N |

---

## Compat Scoring Implications

### Conversion Paths -- Expected Compat Levels

**Compat levels:** `full` (100% fidelity), `high` (minor losses, cosmetic), `medium` (semantic losses, functional), `low` (significant feature loss)

#### Source: Claude Code (paths-based rules)

| Target | Compat | Notes |
|--------|--------|-------|
| Cursor | **full** | `paths` --> `globs`, `alwaysApply` maps directly |
| Windsurf | **full** | `paths` --> `globs` + `trigger: glob` |
| VS Code Copilot | **full** | `paths` --> `applyTo` |
| Cline | **full** | `paths` maps directly |
| Amp | **full** | `paths` --> `globs` |
| Kiro | **high** | `paths` --> `fileMatchPattern` + `inclusion: fileMatch` |
| Codex CLI | **medium** | Globs embedded as prose; no native conditional support |
| Gemini CLI | **medium** | Globs embedded as prose |
| OpenCode | **medium** | Globs embedded as prose |
| Roo Code | **medium** | Globs dropped (mode-based scoping only); warning emitted |
| Zed | **low** | Globs dropped, all rules become always-on; warning emitted |

#### Source: Cursor (4-type rule system)

| Target | Compat | Notes |
|--------|--------|-------|
| Claude Code | **high** | `alwaysApply`/`globs` map well; "Agent" type loses AI-decided semantics |
| Windsurf | **full** | All 4 types map to Windsurf's 4 triggers |
| VS Code Copilot | **high** | Agent type --> description-only; Manual type unsupported |
| Cline | **high** | `globs` --> `paths`; Agent/Manual types lose activation semantics |
| Amp | **high** | `globs` maps; Agent/Manual types become always-on or prose |
| Kiro | **high** | Manual --> `inclusion: manual`; Agent type mapped as auto |
| Codex/Gemini/OpenCode | **medium** | All conditional types become prose |
| Roo Code | **medium** | Globs dropped; mode mapping not possible |
| Zed | **low** | Everything becomes single always-on file |

#### Source: Windsurf (trigger-based rules)

| Target | Compat | Notes |
|--------|--------|-------|
| Cursor | **full** | `always_on` --> `alwaysApply`, `glob` --> `globs`, `model_decision` --> Agent, `manual` --> Manual |
| Claude Code | **high** | `model_decision` and `manual` lose native semantics |
| VS Code Copilot | **high** | `model_decision` loses AI semantics |
| Cline | **high** | `glob` --> `paths`; `model_decision`/`manual` become prose |
| Amp | **high** | `glob` --> `globs`; `model_decision`/`manual` become always-on/prose |
| Kiro | **high** | `manual` --> `inclusion: manual`; `model_decision` mapped as auto |
| Codex/Gemini/OpenCode | **medium** | All trigger types become prose |
| Roo Code | **medium** | Trigger types don't map to modes |
| Zed | **low** | Everything flattened to single file |

#### Source: Roo Code (mode-based rules)

| Target | Compat | Notes |
|--------|--------|-------|
| All providers | **medium** | Mode concept has no equivalent; rules become always-on with warning |

---

## Converter Coverage Audit

### Current Converter State

**File:** `cli/internal/converter/rules.go`

#### Canonicalize (Import) Coverage

| Provider | Covered | Handler | Notes |
|----------|---------|---------|-------|
| Cursor | Y | `canonicalizeCursorRule` | Parses MDC frontmatter (description, alwaysApply, globs) |
| Windsurf | Y | `canonicalizeWindsurfRule` | Parses trigger field, maps to alwaysApply/globs |
| Cline | Y | `canonicalizeClineRule` | Parses paths frontmatter |
| OpenCode | Y | `canonicalizeMarkdownRule` | Plain markdown, optional YAML frontmatter |
| Kiro | Y | `canonicalizeMarkdownRule` | Plain markdown, optional YAML frontmatter |
| Default | Y | `canonicalizeMarkdownRule` | Catch-all for unknown providers |
| **Copilot CLI** | **MISSING** | -- | Should parse `applyTo` frontmatter from `.instructions.md` files |
| **VS Code Copilot** | **MISSING** | -- | Should parse `applyTo` frontmatter (and `.claude/rules` `paths` compat) |
| **Amp** | **MISSING** | -- | Should parse `globs` array frontmatter from AGENTS.md |
| **Gemini CLI** | **MISSING** | -- | Falls to default handler (acceptable -- plain markdown) |
| **Codex CLI** | **MISSING** | -- | Falls to default handler (acceptable -- plain markdown) |
| **Roo Code** | **MISSING** | -- | Falls to default handler (acceptable -- plain markdown, no frontmatter) |
| **Zed** | **MISSING** | -- | Falls to default handler (acceptable -- plain text) |

#### Render (Export) Coverage

| Provider | Covered | Handler | Notes |
|----------|---------|---------|-------|
| Cursor | Y | `renderCursorRule` | Outputs MDC with canonical frontmatter |
| Windsurf | Y | `renderWindsurfRule` | Maps alwaysApply/globs to trigger types |
| Claude Code | Y | `renderClaudeCodeRule` | paths frontmatter for globs, plain for alwaysApply |
| Codex CLI | Y | `renderSingleFileRule` | Plain markdown, scope as prose |
| Gemini CLI | Y | `renderSingleFileRule` | Plain markdown, scope as prose |
| Copilot CLI | Y | `renderSingleFileRule` | Plain markdown, scope as prose |
| Zed | Y | `renderZedRule` | `.rules` file, description as HTML comment |
| Cline | Y | `renderClineRule` | paths frontmatter, description as HTML comment |
| Roo Code | Y | `renderRooCodeRule` | Plain markdown, globs dropped with warning |
| OpenCode | Y | `renderOpenCodeRule` | AGENTS.md plain markdown, scope as prose |
| Kiro | Y | `renderKiroRule` | Plain markdown, scope as prose |
| **VS Code Copilot** | **MISSING** | -- | Should render `.instructions.md` with `applyTo` frontmatter |
| **Amp** | **MISSING** | -- | Should render AGENTS.md with `globs` frontmatter |

### Issues Found

#### 1. Missing Canonicalize Handlers

**Copilot CLI / VS Code Copilot import:** When importing `.instructions.md` files, the `applyTo` frontmatter field is not parsed. These files currently fall through to `canonicalizeMarkdownRule`, which would try to parse `applyTo` as canonical YAML and fail silently (no `description`/`alwaysApply`/`globs` fields). The `applyTo` string value would be lost entirely.

**Amp import:** AGENTS.md files with `globs` array frontmatter would fall through to `canonicalizeMarkdownRule`. Since `globs` is a valid canonical field name but uses a different YAML structure than Cursor's comma-separated string, this might partially work but needs verification. The Amp `globs` format (YAML array) happens to match the canonical format, so this may work by accident.

**Kiro import:** Currently uses `canonicalizeMarkdownRule`, which doesn't understand Kiro's `inclusion`, `fileMatchPattern`, `name`, or `description` fields. A Kiro rule with `inclusion: fileMatch` and `fileMatchPattern` would lose its conditional activation semantics entirely.

#### 2. Missing Render Handlers

**VS Code Copilot export:** Rules exported to VS Code Copilot use the `renderSingleFileRule` (via Copilot CLI path), which outputs plain markdown with scope as prose. It should instead render `.instructions.md` format with `applyTo` frontmatter for native glob support.

**Amp export:** Rules exported to Amp fall through to `renderMarkdownRule` (generic). Should render AGENTS.md with `globs` YAML array frontmatter for native conditional support.

#### 3. Provider-Specific Features Being Dropped Silently

| Feature Lost | Source | Target | Current Behavior | Recommended |
|-------------|--------|--------|-----------------|-------------|
| `model_decision` trigger | Windsurf | Claude Code | Becomes non-alwaysApply with description | Acceptable (description preserved) |
| `manual` trigger | Windsurf | Claude Code | Becomes non-alwaysApply, no description | Should emit warning |
| Kiro `inclusion` modes | Kiro | Any | Completely ignored | Should parse `fileMatchPattern` as globs |
| Kiro `fileMatchPattern` | Kiro | Any | Completely ignored | Should parse as globs |
| Cursor `description` as Agent hint | Cursor | Claude Code | Preserved in meta but not rendered | Acceptable |
| Windsurf char limits | Windsurf | Any | Not enforced | Could warn on large rules |
| Roo Code modes | Roo Code | Any | N/A (no import) | Should warn when mode-specific |
| `AGENTS.override.md` | Codex | Any | Not recognized | Low priority (niche feature) |
| VS Code `excludeAgent` | VS Code | Any | Not parsed | Low priority |
| VS Code `name` field | VS Code | Any | Not parsed | Low priority |

#### 4. New Features Since Converter Was Written

| Provider | New Feature | Impact |
|----------|------------|--------|
| Kiro | `inclusion: fileMatch` with `fileMatchPattern` | **HIGH** -- conditional activation not imported |
| Kiro | `inclusion: auto` vs `always` (synonyms with different semantics) | LOW -- both map to alwaysApply |
| VS Code Copilot | `.claude/rules` compatibility (uses `paths` instead of `applyTo`) | MEDIUM -- VS Code now natively reads Claude rules |
| Copilot CLI | `AGENTS.md` + `.instructions.md` dual system | **HIGH** -- need to handle both file types |
| Amp | `globs` frontmatter in AGENTS.md | **HIGH** -- conditional activation not imported |
| Windsurf | `rules_applied` tracking in hooks | LOW -- runtime feature, not format |
| Windsurf | Enterprise system-level rules paths | LOW -- deployment concern, not format |
| Claude Code | `@path` import syntax | LOW -- content feature, not rule format |

### Recommended Actions (Priority Order)

1. **Add Kiro canonicalize handler** -- parse `inclusion`/`fileMatchPattern` frontmatter, map `fileMatchPattern` to canonical `globs`, map `inclusion: fileMatch` to `alwaysApply: false`

2. **Add Copilot CLI / VS Code canonicalize handler** -- parse `applyTo` frontmatter field, split comma-separated globs into array, map to canonical `globs`

3. **Add Amp canonicalize handler** -- parse `globs` array frontmatter (may already work via default handler, but should be explicit)

4. **Add VS Code Copilot render handler** -- output `.instructions.md` format with `applyTo` frontmatter instead of plain markdown

5. **Add Amp render handler** -- output AGENTS.md with `globs` array frontmatter

6. **Update Kiro render handler** -- output proper `inclusion: fileMatch` + `fileMatchPattern` frontmatter instead of plain markdown with prose scope

7. **Fix Cursor render** -- currently outputs canonical frontmatter directly to `.mdc`; should ensure `globs` is comma-separated string (not YAML array) per Cursor's native format

8. **Add warnings** for Windsurf `manual` trigger conversion to providers without manual mode

### Sources

- [Claude Code Memory docs](https://code.claude.com/docs/en/memory)
- [Cursor Rules Deep Dive](https://forum.cursor.com/t/a-deep-dive-into-cursor-rules-0-45/60721)
- [Cursor MDC Best Practices](https://forum.cursor.com/t/my-best-practices-for-mdc-rules-and-troubleshooting/50526)
- [Gemini CLI GEMINI.md docs](https://geminicli.com/docs/cli/gemini-md/)
- [Copilot CLI Custom Instructions](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions)
- [VS Code Copilot Custom Instructions](https://code.visualstudio.com/docs/copilot/customization/custom-instructions)
- [Windsurf Cascade Memories](https://docs.windsurf.com/windsurf/cascade/memories)
- [Kiro Steering docs](https://kiro.dev/docs/steering/)
- [Codex AGENTS.md Guide](https://developers.openai.com/codex/guides/agents-md)
- [Cline Rules docs](https://docs.cline.bot/customization/cline-rules)
- [OpenCode Rules docs](https://opencode.ai/docs/rules/)
- [Roo Code Custom Instructions](https://docs.roocode.com/features/custom-instructions)
- [Zed AI Rules](https://zed.dev/docs/ai/rules)
- [Amp Owner's Manual](https://ampcode.com/manual)
