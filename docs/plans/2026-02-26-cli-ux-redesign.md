# CLI UX Redesign

**Date:** 2026-02-26
**Status:** Design
**Goal:** Simplify syllago's command structure so TUI and CLI present the same mental model with minimal friction.

## Problem

The current CLI has three commands that overlap confusingly:

| Command | What it does | What users expect |
|---------|-------------|-------------------|
| `syllago import` | Read-only discovery — shows what a provider has, writes nothing | Actually import content |
| `syllago add` | Copies content into local/, canonicalizes, generates metadata | N/A (users don't know this exists) |
| `syllago export` | Copies content out to a provider's install location | This one is fine |

The TUI's Import screen does what users expect `syllago import` to do — it discovers, copies, canonicalizes, and stores. The CLI `import` just reports and stops. Users have to separately run `add` for each item with manual paths.

## Redesigned Command Structure

```
syllago import <source>    ->  Bring content IN (discover + canonicalize + store)
syllago export             ->  Send content OUT (render + write to provider)
syllago promote            ->  Push local content to shared registry (PR workflow)
syllago                    ->  TUI (same operations, interactive)
```

`add` is removed. `import` absorbs its functionality.

### `syllago import`

Three modes based on input:

```bash
# Mode 1: From a provider (discovers + imports)
syllago import --from claude-code
syllago import --from claude-code --type skills
syllago import --from claude-code --name research

# Mode 2: From a path (single item)
syllago import ~/.claude/skills/research --type skills
syllago import ./my-cursor-rule.mdc --type rules --provider cursor

# Mode 3: From a git URL (clone + pick items)
syllago import https://github.com/someone/ai-rules.git
```

#### Interactive selection (Mode 1 default)

When `--from` is used without `--all`, `--type`, or `--name`, the user gets an interactive picker:

```
$ syllago import --from claude-code

Found 12 items from Claude Code:
  Skills (3):
    research              Token-efficient web research
    code-guardian          Security-focused code review
    brainstorm             Collaborative design development
  Rules (5):
    always-bun             Always use bun for TypeScript
    mutual-accountability  Challenge assumptions, verify claims
    ...
  Hooks (2):
    security-validator     Block dangerous shell commands
    readability-wrapper    Wrap fetched content with trust boundaries
  MCP (2):
    duckduckgo             DuckDuckGo search via MCP
    readability            Mozilla Readability content extraction

Import: (a)ll, (s)elect, (n)one? s

Select items to import (space to toggle, enter to confirm):
  [x] research
  [x] code-guardian
  [ ] brainstorm
  [x] always-bun
  ...

Imported 5 items to local/
```

#### Non-interactive flags (scripting/automation)

```bash
syllago import --from claude-code --all            # import everything
syllago import --from claude-code --type skills    # only skills, import all
syllago import --from claude-code --name research  # just that one item
syllago import --from claude-code --preview        # read-only discovery (old behavior)
```

#### What happens on import

1. Discover content from source (provider scan, path read, or git clone)
2. Canonicalize into syllago's internal format
3. Store in `local/{type}/{name}/` (or `local/{type}/{provider}/{name}/` for provider-specific)
4. Preserve original in `.source/` for lossless round-trips
5. Generate `.syllago.yaml` metadata
6. Generate placeholder `README.md`

The canonical format is an implementation detail — users never see or interact with it. They see "I imported a Claude skill" and "I can now export it as a Kiro steering file."

### `syllago export`

Stays mostly the same. Takes content from local/ and writes it to a provider's location.

```bash
syllago export --to cursor
syllago export --to kiro --type skills --name code-guardian
```

Handles three install-dir modes:
- **Filesystem path** (e.g., `~/.claude/skills/`) — copy/symlink
- **JSON merge** (`__json_merge__`) — merge into provider config file
- **Project scope** (`__project_scope__`) — resolve via `DiscoveryPaths(cwd)`

### `syllago promote`

Moves local content to shared/registry scope. Separate from import — you import first, test/tweak, then promote when ready.

```bash
syllago promote research                    # promote a specific item
syllago promote --all --type skills         # promote all local skills
```

Promotion workflow: create branch, move from local/ to shared, open PR.

### `syllago registry sync`

Already exists. Adding `--type` filter for selective sync:

```bash
syllago registry sync                       # sync all registries
syllago registry sync my-rules              # sync specific registry
syllago registry sync --type skills         # sync all, but only re-scan skills
syllago registry sync my-rules --type mcp   # sync one, filter to MCP
```

## Content Tiers

Three tiers with clear origin and trust level:

| Tier | Source | Location | Mutable? |
|------|--------|----------|----------|
| **local** | User imported or created | `local/` (gitignored) | Yes |
| **builtin** | Ships with syllago | Root content dirs (git-tracked) | No (read-only) |
| **registry** | Git-based remote repos | `.syllago/registries/` (cloned) | No (sync to update) |

Each tier answers a different question:
- **local**: "What have I personally brought in?"
- **builtin**: "What does syllago give me out of the box?"
- **registry**: "What am I pulling from shared/community sources?"

Workflows per tier:
- **local** -> import, export, promote
- **builtin** -> export, browse (read-only)
- **registry** -> browse, export, import-to-local (to customize)

### Tier display in TUI and CLI

Items should be tagged with their tier so users always know what they're looking at:

```
$ syllago registry items
NAME                  TYPE        SOURCE          DESCRIPTION
research              skills      builtin         Token-efficient web research
code-guardian         skills      local           Security-focused code review
common-rules          rules       my-rules        Community coding standards
```

## TUI Alignment

The TUI should mirror the CLI's mental model:

| TUI Screen | Maps to CLI | Current state |
|------------|------------|---------------|
| Import | `syllago import` | Already works (discover + store) |
| Detail > Install | `syllago export` | Works for user-scope; needs project-scope fix |
| Detail > Promote | `syllago promote` | Exists but separate from import flow |
| Registries | `syllago registry *` | Works |

### TUI project-scope fix needed

The TUI installer currently fails for project-scope types (Kiro skills/rules, Cline rules, etc.) because `resolveTargetWithBase` returns an error for `ProjectScopeSentinel`. The fix: when the installer encounters `ProjectScopeSentinel`, resolve via `DiscoveryPaths(cwd)` the same way the export command does.

## Migration Path

1. `syllago add` becomes an alias for `syllago import` (deprecation warning)
2. `syllago import --from` gains write behavior (discovery + store)
3. `syllago import --preview` preserves old read-only behavior
4. Remove `add` alias after one release cycle

## Implementation Order

1. **Merge `add` into `import`** — combine discovery + write into one command
2. **Add interactive selection** — prompt when no filters specified
3. **Fix TUI project-scope** — installer handles `ProjectScopeSentinel`
4. **Add `--type` to `registry sync`** — selective sync
5. **Align TUI labels** — match CLI verb names
6. **Deprecate `add`** — alias with warning, then remove
