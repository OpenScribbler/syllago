# Native Registry Indexing - Design Document

**Goal:** Allow any repo with provider-native AI content (.claude/, .cursor/, .gemini/, etc.) to serve as a syllago registry without restructuring files — via a content index in registry.yaml.

**Decision Date:** 2026-03-18

---

## Problem Statement

Syllago registries require content in syllago-native layout (skills/, agents/, rules/<provider>/). Real-world repos store content in provider-native format (.claude/skills/, .cursorrules, .gemini/). This forces teams to either restructure their repos or maintain a separate registry repo — both add friction that kills adoption.

## Proposed Solution

A content index in registry.yaml that maps provider-native paths to registry items. The repo itself becomes the registry. The existing ScanNativeContent() function already knows where providers store content — we enhance it to produce structured item data, generate the index via an interactive wizard, and teach the scanner to follow index mappings.

### Data Flow

```
repo with .claude/ content
  -> `syllago registry create --from-native` (run inside repo)
  -> ScanNativeContent() finds all provider-native items
  -> Interactive wizard: select all / one provider / cherry-pick
  -> Optional: scan user-scoped settings for hooks
  -> Generates registry.yaml with items[] section
  -> Repo is now a valid registry

other project
  -> `syllago registry add <url>`
  -> Clones repo, finds registry.yaml with items[]
  -> Scanner follows path mappings to native locations
  -> Items appear in TUI/CLI like any registry content
```

## Architecture

### registry.yaml Format Extension

The existing Manifest struct gets an Items field:

```yaml
name: aembit-docs-tools
description: AI coding tools from the Aembit docs team
version: 0.1.0

items:
  - name: docs-research
    type: skills
    provider: claude-code
    path: .claude/skills/docs-research

  - name: research-agent
    type: agents
    provider: claude-code
    path: .claude/agents/research-agent.md

  - name: astro
    type: commands
    provider: claude-code
    path: .claude/commands/astro.md

  - name: wizard-invariant-gate
    type: hooks
    provider: claude-code
    hookEvent: PostToolUse
    hookIndex: 0
    path: .syllago/hooks/wizard-invariant-gate
    scripts:
      - scripts/wizard-invariant-gate.sh
```

**Field definitions:**
- `path`: relative to repo root, points to item root (directory for multi-file items, file for single-file)
- `type`: existing ContentType values — skills, agents, mcp, rules, hooks, commands
- `provider`: source provider slug (claude-code, cursor, windsurf, etc.)
- `name`: item name as it appears in the registry (derived from directory/file name during generation)
- `hookEvent` / `hookIndex`: for hooks extracted from settings files, identifies which hook
- `scripts`: for hooks with external script files, lists the scripts included with the item
- Format is extensible — requires, tags, description fields can be added later

### Scanner Priority

```
registry.yaml with items[] found?
  -> YES: follow index mappings (native-content registry)
  -> NO: has syllago content dirs (skills/, agents/, etc.)?
    -> YES: scan syllago-native layout (existing behavior)
    -> NO: reject as invalid registry
```

Existing syllago-native registries never see the new code path. The index is opt-in.

### Components Involved

| Component | Change |
|-----------|--------|
| registry/registry.go | Extend Manifest struct with Items []ManifestItem |
| catalog/native_scan.go | Enhance ScanNativeContent() to return structured items (name, type, description) not just file paths. Add missing project-scoped MCP/hooks patterns. |
| catalog/scanner.go | New scanFromIndex() function — reads registry.yaml items and builds ContentItem entries by following paths |
| cmd/syllago/registry_cmd.go | Split create into --new / --from-native modes, add wizard flow for --from-native |
| registry/scaffold.go | Existing scaffold becomes --new path |
| tui/app.go | Update registry add validation to accept repos with registry.yaml items |

### Scanner Integration Point

In scanRoot(), before walking content type directories:

```go
func scanRoot(cat, baseDir, local) {
    // NEW: Check for registry.yaml with items[]
    manifest := loadManifestFromDir(baseDir)
    if manifest != nil && len(manifest.Items) > 0 {
        return scanFromIndex(cat, baseDir, manifest.Items, local)
    }

    // EXISTING: Walk syllago-native layout
    for each contentType...
}
```

scanFromIndex() iterates the items list, resolves each path relative to baseDir, and builds ContentItem entries using the same metadata extraction logic (frontmatter parsing, collectFiles(), etc.) that the existing scanner uses.

## Hooks Design

Hooks are architecturally different from other content types — they're embedded in settings files, not standalone files/directories.

### Hook Sources

| Hook source | How handled |
|-------------|-------------|
| Project-scoped (.claude/settings.json in repo) | Index points to settings file. Scanner extracts individual hooks at read time. No copying needed. |
| User-scoped (~/.claude/settings.json on machine) | Copy hook definition + scripts into .syllago/hooks/<name>/. Index points to copied files. Security warning displayed. |

### Hook Extraction

Each hook from a settings file becomes an individual registry item. This is consistent with how syllago already canonicalizes hooks.

For hooks with external script files:
- If script is in-repo: include in scripts[] list, hook is self-contained
- If script is outside repo: warn during indexing, mark as having external deps

### Missing Native Scan Patterns

providerNativePatterns() needs these additions for MCP and hooks:

**MCP configs (project-scoped):**
- .copilot/mcp.json (Copilot CLI)
- .vscode/mcp.json (Cline)
- .roo/mcp.json (Roo Code)
- .kiro/settings/mcp.json (Kiro)
- opencode.json (OpenCode)

**Hooks (project-scoped):**
- .claude/settings.json — hooks key (Claude Code)
- .copilot/hooks.json (Copilot CLI)
- .kiro/agents/ — hooks embedded in agent JSON (Kiro)

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Command surface | registry create --from-native / --new | One command, two modes. Clear mental model: "create a registry" from scratch or from existing content |
| Index location | Extend registry.yaml with items[] section | Single file for all registry metadata. Extensible format. |
| Item path model | Path-to-root with auto file discovery | Simple, DRY. Scanner walks subdirectories automatically using existing collectFiles() logic. |
| Hook granularity | Individual extraction | Consistent with canonical format. Consumers can install specific hooks. |
| User-scoped hooks | Copy to .syllago/hooks/ with security warning | Makes hooks portable. Warning because scripts execute on consumer's machine. |
| External deps | Out of scope | In-repo content only. External dependencies need a separate dependency resolution system. |
| Wizard UX | Interactive (not flag-based) | Richer experience, matches existing TUI patterns. Shows discovered content for selection. |

## Interactive Wizard Flow

### Step 1: Scan and Display

```
$ syllago registry create --from-native

Scanning for AI coding tool content...

Found content from 1 provider:

  Claude Code
    33 agents     .claude/agents/
    12 skills     .claude/skills/
    10 commands   .claude/commands/
     1 rules      CLAUDE.md
```

### Step 2: Selection Mode

```
How would you like to index this content?

  * All content from all providers
  * Select by provider
  * Select individual items
```

"Select by provider" shows provider checkboxes. "Select individual items" shows items grouped by provider and type with toggles.

### Step 3: User-Scoped Hooks (Optional)

```
Would you also like to include hooks from your user settings?
  * Scan ~/.claude/settings.json
  * No, project content only

SECURITY WARNING
  Hooks contain executable scripts that run on the consumer's machine.
  Only include hooks you trust and intend to share publicly.
  Scripts will be copied to .syllago/hooks/ in this repo.

  Continue? [y/N]
```

If yes, parse settings file, list individual hooks for selection.

### Step 4: Metadata

```
Registry name [aembit-docs-astro]:
Description (optional): AI coding tools for Aembit docs team
```

### Step 5: Generate

```
Generating registry.yaml...

  name: aembit-docs-astro
  56 items indexed (33 agents, 12 skills, 10 commands, 1 rule)

  registry.yaml created
  .syllago/hooks/ created (3 hooks with scripts)

This repo can now be added as a registry:
  syllago registry add <url-to-this-repo>
```

## Error Handling

**During scanning:**
- Empty provider directories: skip silently
- Malformed settings.json: warn, skip hooks, continue with other content
- Skills missing frontmatter: use directory name, empty description

**During user-scoped hook extraction:**
- Script outside repo: warn per-hook with message
- Script doesn't exist: warn per-hook
- User declines security warning: skip hooks entirely, continue

**During registry add (consumer side):**
- items[] paths that don't exist in clone: warn per-item, skip, continue
- Mix of syllago-native dirs AND items[] in same repo: items[] takes precedence

**During install from indexed registry:**
- Hook items with scripts: show security warning before install (existing behavior)
- Skills/agents: standard collectFiles() + copy (existing behavior)

## Success Criteria

1. `syllago registry create --from-native` inside ~/src/aembit_docs_astro generates valid registry.yaml with all items indexed
2. From another project, `syllago registry add <path>` loads items into TUI and CLI
3. Install a skill from the indexed registry — files land correctly
4. Install a hook from the indexed registry — hook merges into settings
5. Existing syllago-native registries continue working unchanged
6. `syllago registry create --new` still scaffolds empty structure

## Open Questions

None — all decisions captured above.

---

## Next Steps

Ready for implementation planning with Plan skill.
