# Content Model Restructure

*Design date: 2026-02-27*
*Status: Draft*
*Parent: v1-release-strategy.md (workstreams 2, 4, 5)*

## Overview

This design captures the full content model for syllago v1: how content is classified, where it lives on disk, how the scanner finds it, how the TUI displays it, and how CLI commands interact with it. It also addresses several critical bugs found during design review (two-root problem, CLI import not writing, export scope limitation) and adds team-facing features (registry security, content precedence, promote-to-registry).

---

## 1. Content Classification

All content has exactly one **source** and optionally one or more **tags** that modify behavior.

### Sources (mutually exclusive)

| Source | Stored In | Git-tracked | Badge | Description |
|--------|-----------|-------------|-------|-------------|
| Built-in | `content/` (syllago repo) | Yes | `[BUILT-IN]` or `[EXAMPLE]` | Ships with syllago |
| Shared | `<contentRoot>/` (user workspace) | Yes | *(none)* | Project content visible to team |
| Local | `local/` | No (gitignored) | `[LOCAL]` | User's private imports |
| Registry | `.syllago/registries/<name>/` | Read-only clone | `[registry-name]` | Content from git registries |

### Built-in Sub-categories

Built-in content has two flavors, distinguished by tags:

| Sub-category | Tags | Badge | Visibility | Purpose |
|---|---|---|---|---|
| Meta-tools | `builtin` | `[BUILT-IN]` (purple) | Visible by default | Help users use/create syllago content |
| Examples | `builtin`, `example` | `[EXAMPLE]` (purple dim) | Hidden by default | Demonstrate formats, serve as test fixtures |

**Meta-tools:**
- `syllago-guide` (skill) — Complete reference for using syllago
- `syllago-import` (skill) — Import workflow reference
- `syllago-author` (agent) — Expert at creating syllago-canonical content

**Examples:** Renamed with `example-` prefix, tagged `builtin` + `example`, hidden by default.
- `example-code-review` (skill) — was `code-review`
- `example-code-reviewer` (agent) — was `code-reviewer`
- `example-explain-code` (prompt) — was `explain-code`
- `example-write-tests` (prompt) — was `write-tests`
- Plus kitchen-sink examples for each content type (see section 6)

---

## 2. Directory Structure

### The syllago repo

```
syllago/
├── content/                        # ALL built-in content
│   ├── skills/
│   │   ├── syllago-guide/            [BUILT-IN]
│   │   ├── syllago-import/           [BUILT-IN]
│   │   ├── example-code-review/    [EXAMPLE] hidden
│   │   └── example-kitchen-sink-skill/  [EXAMPLE] hidden
│   ├── agents/
│   │   ├── syllago-author/           [BUILT-IN]
│   │   └── example-kitchen-sink-agent/  [EXAMPLE] hidden
│   ├── rules/
│   │   └── example-kitchen-sink-rules/  [EXAMPLE] hidden
│   ├── hooks/
│   │   └── example-kitchen-sink-hooks/  [EXAMPLE] hidden
│   ├── mcp/
│   │   ├── filesystem/             (existing)
│   │   └── example-kitchen-sink-mcp/    [EXAMPLE] hidden
│   ├── prompts/
│   │   ├── example-explain-code/   [EXAMPLE] hidden
│   │   ├── example-write-tests/    [EXAMPLE] hidden
│   │   └── example-kitchen-sink-prompt/ [EXAMPLE] hidden
│   ├── commands/
│   │   └── example-kitchen-sink-commands/ [EXAMPLE] hidden
│   └── apps/
│       └── example-kitchen-sink-app/    [EXAMPLE] hidden
├── local/                          # User's private content (gitignored)
├── cli/                            # Go source code
├── docs/                           # Design docs, plans
├── templates/                      # Scaffolding for `syllago create`
├── releases/                       # Release notes
├── memory/                         # AI assistant context
├── .syllago/
│   └── config.json                 # contentRoot: "content"
└── README.md, Makefile, etc.
```

**Top level: ~9 entries** (down from 15+). Clean separation of product code from content.

### A user's project workspace

```
my-project/
├── skills/                         # Shared content (git-tracked)
│   └── team-review-skill/
├── agents/
├── rules/
│   └── claude-code/
│       └── conventional-commits/
├── local/                          # Private content (gitignored)
│   └── skills/
│       └── my-experiment/
├── .syllago/
│   ├── config.json                 # providers, registries, settings
│   └── registries/                 # Registry clones (gitignored)
│       └── company-tools/
│           ├── skills/
│           └── rules/
├── src/                            # their actual code
└── .gitignore
```

**Content root:** defaults to `.` (repo root). Configurable via `contentRoot` in `.syllago/config.json`.

---

## 3. Content Root Resolution (Fixing the Two-Root Problem)

### The Problem

`findContentRepoRoot()` walks up from CWD looking for a `skills/` directory. In a user's project that just ran `syllago init`, no content directories exist. The TUI won't launch. Export fails.

### The Fix

**Step 1: `syllago init` creates the workspace structure.**

When `syllago init` runs in a project:
1. Create `.syllago/config.json` with detected providers (existing behavior)
2. Create `local/` directory
3. Add `local/` to `.gitignore` if not already present
4. Add `.syllago/registries/` to `.gitignore` if not already present
5. Offer to install built-in meta-tools to detected providers

`syllago init` does NOT create shared content directories (`skills/`, `rules/`, etc.) at the root — those are created on demand when the user promotes content or creates shared items.

**Step 2: Replace `findContentRepoRoot()` with config-aware resolution.**

New resolution order:
1. If `.syllago/config.json` exists with `contentRoot` field → use `<projectRoot>/<contentRoot>`
2. Else if any content directory exists at project root (`skills/`, `agents/`, etc.) → use project root
3. Else → use project root (scanner handles empty gracefully)

The scanner already handles missing directories fine — `scanRoot` returns empty when dirs don't exist. The issue is `findContentRepoRoot()` *erroring* instead of gracefully returning the project root.

**Step 3: Scanner scans multiple sources.**

```
ScanAll(projectRoot string, config Config) → Catalog
  1. Scan contentRoot (from config or project root) → shared items
  2. Scan local/ → local items
  3. Scan .syllago/registries/ → registry items
  4. If syllago repo: scan content/ for built-in items
```

No content directories? Empty catalog. TUI shows an empty state with guidance: "No content yet. Try `syllago import --from <provider>` to bring in existing content."

---

## 4. TUI Badges and Visibility

### Badge System

| Source | Badge Text | Style | Priority |
|--------|-----------|-------|----------|
| Built-in meta-tool | `[BUILT-IN]` | Purple (accent) | 1 |
| Built-in example | `[EXAMPLE]` | Purple dim | 2 |
| Local import | `[LOCAL]` | Amber (warning) | 3 |
| Registry | `[registry-name]` | Grey (muted) | 4 |
| Shared (no badge) | — | — | 5 |

### Visibility / Hiding

**New metadata field:** `hidden: true` in `.syllago.yaml`.

**Default visibility:**
- Meta-tools (`builtin` tag, no `example` tag): visible
- Examples (`builtin` + `example` tags): hidden by default
- Everything else: visible

**TUI controls:**
- `H` key on any item: toggle hidden status (writes `hidden` field to `.syllago.yaml`)
- Bottom status bar: "X items hidden" indicator when hidden items exist
- `Shift+H` or settings toggle: show/hide all hidden items temporarily (session-only)

**CLI support:**
- `syllago registry items --show-hidden` to include hidden items
- `syllago export --show-hidden` to include hidden items in export scope

### Export/Promote Warnings

When a user attempts to export or promote built-in content:

```
⚠ "syllago-guide" is built-in content that ships with syllago.
  Built-in content is not intended for production use in registries.
  Syllago is not responsible for any misuse of built-in content.
  Continue anyway? [y/N]
```

For examples, the warning is slightly different:

```
⚠ "example-code-review" is an example that ships with syllago.
  Example content demonstrates syllago formats and is not intended for production.
  Syllago is not responsible for any misuse of example content.
  Continue anyway? [y/N]
```

---

## 5. CLI Fixes

### 5a. CLI Import Must Write

**Current behavior:** `syllago import --from <provider>` discovers files, parses them, prints a report, and exits. Files are NOT written to `local/`.

**Fixed behavior:** After parsing, write canonicalized content to `local/`:
1. Discover files from provider
2. Parse and canonicalize each item
3. Write to `local/<type>/[<provider>/]<name>/`
4. Create `.syllago.yaml` with `source_provider`, `source_format`, `imported_at`
5. Print summary of what was imported

Add `--dry-run` flag to restore the current discovery-only behavior.

### 5b. Export Works on All Sources

**Current behavior:** `syllago export` only exports from `local/` (line 100: `if !item.Local { continue }`).

**Fixed behavior:** Export from any source:
- Local items: export as before
- Shared items: export with conversion
- Registry items: export with conversion
- Built-in items: export with warning (see section 4)

Add `--source` flag to filter: `syllago export --to cursor --source local` (default: all).

### 5c. `syllago create` Command

New scaffolding command for CLI content creation:

```bash
syllago create skill my-review-skill
syllago create agent my-helper
syllago create rule --provider cursor my-cursor-rule
```

Behavior:
1. Create directory in `local/<type>/[<provider>/]<name>/`
2. Copy template files from `templates/<type>/`
3. Generate `.syllago.yaml` with name and timestamp
4. Print path and next steps

### 5d. `syllago inspect` Command

Pre-install auditing for registry content:

```bash
syllago inspect company-tools/mcp/database-server
```

Shows:
- Full config.json / SKILL.md / AGENT.md content
- Metadata from .syllago.yaml
- Risk indicators (hooks that run commands, MCP with network access, etc.)
- File list

---

## 6. Kitchen-Sink Examples

### Purpose

Kitchen-sink examples serve three roles:
1. **Documentation**: Show every configurable field for each content type
2. **Test fixtures**: Comprehensive converter round-trip testing
3. **Format reference**: Show what canonical format looks like when fully populated

### Scope

One kitchen-sink example per content type:

| Content Type | Example Name | Key Fields Exercised |
|---|---|---|
| Skills | `example-kitchen-sink-skill` | `name`, `description`, `allowed-tools`, `disallowed-tools`, `context: fork`, `agent`, `model`, `user-invocable: true`, `argument-hint`, `disable-model-invocation` |
| Agents | `example-kitchen-sink-agent` | `name`, `description`, `tools`, `disallowedTools`, `model`, `maxTurns`, `permissionMode`, `skills`, `mcpServers`, `memory`, `background`, `isolation`, `temperature`, `timeout_mins`, `kind` |
| Rules | `example-kitchen-sink-rules` | One rule per provider (Claude Code markdown, Cursor MDC with globs, Codex TOML, Gemini CLI markdown, Windsurf markdown, etc.) |
| Hooks | `example-kitchen-sink-hooks` | `PreToolUse`, `PostToolUse`, `Notification` events; `command` and `url` hook types; matcher patterns |
| MCP | `example-kitchen-sink-mcp` | `command`, `args`, `env` with variable references, all `MCPConfig` fields |
| Prompts | `example-kitchen-sink-prompt` | Full frontmatter + rich body with sections |
| Commands | `example-kitchen-sink-commands` | Per-provider command formats |
| Apps | `example-kitchen-sink-app` | `README.md` + `install.sh` with all metadata fields |

### Maintenance Guardrail

A CI test validates that kitchen-sink examples cover all defined struct fields:

```go
// TestKitchenSinkFieldCoverage ensures kitchen-sink examples use every field
// defined in the converter structs. If you add a field to SkillMeta, AgentMeta,
// etc., this test fails until you update the corresponding kitchen-sink example.
func TestKitchenSinkFieldCoverage(t *testing.T) {
    // For each content type:
    // 1. Reflect on the canonical struct (SkillMeta, AgentMeta, etc.)
    // 2. Parse the kitchen-sink example
    // 3. Assert every exported field is populated (non-zero value)
}
```

This test runs in CI. Adding a new frontmatter field without updating the kitchen-sink example fails the build.

### Converter Round-Trip Tests

Each kitchen-sink example gets a round-trip test:

```go
func TestKitchenSinkSkillRoundTrip(t *testing.T) {
    // For each target provider:
    // 1. Canonicalize the kitchen-sink skill
    // 2. Render to target provider format
    // 3. Canonicalize back from target format
    // 4. Compare: assert no unexpected data loss
    // 5. Collect and assert expected warnings match
}
```

---

## 7. Registry Security

### 7a. Allowed Registries

New config field in `.syllago/config.json`:

```json
{
  "allowedRegistries": [
    "https://github.com/acme-corp/syllago-tools.git",
    "https://github.com/OpenScribbler/syllago-tools.git"
  ]
}
```

When set, `syllago registry add <url>` rejects URLs not in the list. Team leads commit this to the project repo to enforce registry policy.

When empty or absent, any URL is allowed (default for solo users).

### 7b. Content Risk Indicators

The `syllago inspect` command (5d) and TUI detail view show risk indicators:

| Indicator | Trigger |
|-----------|---------|
| `⚠ Runs commands` | Hooks with `type: command`, MCP configs, apps with `install.sh` |
| `⚠ Network access` | MCP servers, hooks with `type: url` |
| `⚠ Environment variables` | MCP configs with `env` references |
| `⚠ Bash access` | Skills/agents with `Bash` in allowed tools |

These are informational, not blocking. They help team leads make informed decisions.

### 7c. Registry Manifests

Optional `registry.yaml` at registry repo root:

```yaml
name: acme-syllago-tools
description: Approved AI tool content for Acme Corp
maintainers:
  - devtools-team@acme.com
version: "2.1.0"
min_syllago_version: "1.0.0"
```

Displayed in TUI registry browser and `syllago registry list` output. Not required — registries without a manifest still work.

---

## 8. Content Precedence and Deduplication

When the same item appears in multiple sources, syllago applies precedence:

```
1. Local (highest) — user's explicit choice
2. Shared — project team's choice
3. Registry — external source
4. Built-in (lowest) — syllago defaults
```

**Deduplication rules:**
- Match by `name` + `type` (case-insensitive)
- Only the highest-precedence version appears in the TUI by default
- Lower-precedence versions are accessible via "Show all sources" toggle
- The TUI detail view notes when an item overrides a lower-precedence version: "Overrides [BUILT-IN] version"

---

## 9. Additional CLI Commands

### 9a. `syllago sync-and-export`

One-step command for team workflows:

```bash
syllago sync-and-export --to cursor
```

Equivalent to `syllago registry sync && syllago export --to cursor`. Useful in project setup scripts and CI.

### 9b. `syllago export --to all`

Export to all configured providers at once:

```bash
syllago export --to all --json
```

Iterates through configured providers in `.syllago/config.json`, exports to each, reports results. The `--json` flag makes it CI-friendly.

### 9c. `syllago promote --to-registry <name>`

Contribute local content to a registry (similar to Homebrew contributions):

```bash
syllago promote --to-registry company-tools skills/my-skill
```

1. Fork the registry repo (if not already forked)
2. Create branch `syllago/contribute/<type>/<name>`
3. Copy content to the registry's directory structure
4. Commit and push
5. Open PR against the registry repo

### 9d. `syllago list`

Quick CLI inventory of all content:

```bash
syllago list                      # All content with source badges
syllago list --source local       # Only local items
syllago list --source registry    # Only registry items
syllago list --type skills        # Only skills
```

Shows name, type, source, description in a compact table.

---

## 10. First-Run Experience

When `syllago` (TUI) launches with no content and no registries:

```
┌─────────────────────────────────────────────────┐
│  Welcome to syllago!                              │
│                                                 │
│  No content found. Here's how to get started:   │
│                                                 │
│  1. Import existing content:                    │
│     syllago import --from claude-code             │
│                                                 │
│  2. Add a community registry:                   │
│     syllago registry add syllago-tools              │
│                                                 │
│  3. Create new content:                         │
│     syllago create skill my-first-skill           │
│                                                 │
│  Press q to exit, or ? for help.                │
└─────────────────────────────────────────────────┘
```

The `syllago-tools` alias expands to the full OpenScribbler registry URL.

---

## 11. Implementation Order

Based on dependency chains and risk:

### Phase 1: Foundation (must be first)
1. **Fix content root resolution** — Replace `findContentRepoRoot()` with config-aware logic
2. **`syllago init` scaffolds workspace** — Create `local/`, `.gitignore` entries, graceful empty state
3. **Move content to `content/` directory** — Rename dirs, update scanner, update `.syllago/config.json`
4. **Add `hidden` field to metadata** — Simple schema addition

### Phase 2: Content Model
5. **Rename examples with `example-` prefix** — Rename dirs, add `example` tag, set `hidden: true`
6. **Add `[EXAMPLE]` badge to TUI** — Alongside existing `[BUILT-IN]` badge
7. **Implement hide/show toggle** — `H` key, status bar indicator, settings toggle
8. **Add export/promote warnings for built-in content**

### Phase 3: Kitchen-Sink Examples
9. **Create kitchen-sink examples** — One per content type, all fields populated
10. **Add field coverage test** — Reflect-based CI test for struct coverage
11. **Add round-trip tests** — Converter tests using kitchen-sink examples

### Phase 4: CLI Fixes
12. **CLI import writes to `local/`** — Core fix with `--dry-run` flag
13. **Export works on all sources** — Remove local-only restriction, add `--source` flag
14. **`syllago create` command** — Scaffolding from templates
15. **`syllago list` command** — CLI inventory

### Phase 5: Registry and Team Features
16. **`allowedRegistries` config** — Registry URL whitelist
17. **`syllago inspect` command** — Pre-install auditing with risk indicators
18. **Content precedence/dedup** — Priority ordering, override display
19. **Registry manifest** — Optional `registry.yaml`
20. **`syllago promote --to-registry`** — Registry contribution workflow

### Phase 6: Polish
21. **`syllago sync-and-export`** — One-step sync + export
22. **`syllago export --to all`** — Multi-provider export
23. **First-run experience** — Empty state guidance in TUI
24. **Registry short aliases** — `syllago-tools` → full URL

---

## Migration Path

The `content/` restructure is a breaking change for anyone who has the syllago repo cloned. Migration:

1. Move `skills/`, `agents/`, `rules/`, `hooks/`, `mcp/`, `prompts/`, `commands/`, `apps/` → `content/`
2. Update `.syllago/config.json` to set `contentRoot: "content"`
3. Update scanner to handle both old and new paths during transition
4. Update all references in templates, README, CLAUDE.md, LLM-PROMPT.md files
5. Bump to v1.0.0 — clean break

Since syllago is pre-v1 with no external users depending on the directory layout, this is a clean migration. Anyone on `main` gets the new structure on next pull.
