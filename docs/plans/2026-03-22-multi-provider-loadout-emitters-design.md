# Multi-Provider Loadout Emitters — Design Doc

**Bead:** syllago-ohdb
**Status:** Design
**Date:** 2026-03-22

## Problem

Loadouts currently only emit to Claude Code. All 5 reviewers flagged this as the #1 blocker for team/enterprise adoption. Teams with mixed AI tool usage (Claude Code, Cursor, Gemini CLI, Copilot CLI, etc.) can't share loadouts across providers.

## Core Principle

**Loadouts are provider-specific, but content is provider-agnostic.**

A loadout targets one provider and guarantees that every item works for that provider. The underlying library content is in syllago's canonical format — provider is a deployment target, not an authoring concern.

Cross-provider "universal loadouts" that target multiple providers simultaneously are a future consideration, deferred until provider convergence makes that viable. Today, providers differ too much (hooks, skills, agents) for a good cross-apply experience.

## Design Decisions

### 1. Content Model

Content in the syllago library is canonical format regardless of which provider directory it lives in. The provider directory (`content/rules/claude-code/`, `content/rules/cursor/`) records where content was imported from (source origin), not a restriction on where it can be deployed.

When a user authors content directly in syllago, the provider directory is incidental — the content is canonical and available to any provider through the conversion system.

### 2. Resolution Strategy

**Prefer same-provider, fallback to any provider — for provider-specific types only.**

This strategy applies to **Rules, Hooks, and Commands** — the content types that live under provider subdirectories (`content/rules/<provider>/`). Universal types (Skills, Agents, MCP) already resolve by `type + name` only and have no provider dimension. Their resolution is unchanged.

When resolving provider-specific references:

1. First pass: match by `type + name + provider` (same provider as the loadout target). This gives a 1:1 match with zero conversion loss, and enables lossless `.source/` roundtrips.
2. Fallback: match by `type + name` only (any provider). The converter handles format differences at install time.

This handles name collisions correctly — if `content/rules/claude-code/logging/` and `content/rules/cursor/logging/` both exist, the same-provider match wins for the respective loadout.

**Fallback tiebreaker:** When multiple non-target providers have an item with the same name (e.g., targeting Copilot CLI, and both `cursor/logging` and `gemini-cli/logging` exist), the catalog's existing precedence order applies (local > content > registry; within the same layer, scan order). This is deterministic but arbitrary between providers. If this becomes a real problem, we can add explicit source mapping in the manifest (`{name: logging, from: cursor}`), but we defer that until we see it matter.

**Current code change:** `findItem()` in `loadout/resolve.go` currently hard-requires `item.Provider == manifest.Provider` for provider-specific types. This changes to prefer-then-fallback.

### 3. Compatibility Scoring

**Unified compat scorer for all content types.**

Extend the existing `CompatLevel` system (currently hooks-only) to cover all content types. This provides a pre-flight assessment: "can this content work on provider X, and at what level?"

#### Compat Levels

| Level | Meaning | Loadout behavior | UI |
|-------|---------|-----------------|-----|
| **Full** | All features translate, no behavioral change | Default allowed | Green checkmark |
| **Degraded** | Minor features lost, core behavior unchanged | Requires explicit override to include | Yellow ~ with explanation |
| **Broken** | Content runs but behavior is fundamentally wrong | Blocked by default, requires --force | Red ! with strong warning |
| **None** | Cannot install — feature doesn't exist on target | Cannot be included | Red X, not selectable |

#### Compat by Content Type

| Type | Key compat factors |
|------|-------------------|
| **Rules** | Format conversion (MD/MDC/TOML). `alwaysApply` semantics. Single-file vs directory providers. Generally high fidelity — most rules convert cleanly. |
| **Skills** | Provider support (not all providers have skills). Skills containing hooks inherit hook compat issues. |
| **Commands** | Format (YAML+MD vs TOML). Tool name translations. Argument handling differences. |
| **Agents** | Model references. Tool access patterns. Some providers don't support agents at all. |
| **Hooks** | Most divergent type. Events, features (matcher, async, statusMessage, LLM hooks, timeout), structured output fields. Already fully modeled in `converter/compat.go`. |
| **MCP** | Config key structure varies (`mcpServers` vs `servers`) but MCP protocol is standard. Generally high compat. |

#### Scoring Logic

Same-source-provider items score **Full** in practice — content authored for a provider doesn't use features that provider can't handle. However, the scorer still runs the analysis rather than short-circuiting, since content could have been converted *into* a provider directory from canonical format.

Cross-provider scoring uses:
- **Hooks:** Existing `AnalyzeHookCompat()` — per-feature matrix against `HookCapabilities`.
- **Other types:** New compat analyzers per content type, following the same pattern: define a feature matrix per provider, check which source features the target supports.

### 4. Compat-Gated Inclusion

Loadouts should usually be 100% compatible. Degraded content is opt-in, not a silent default.

**At loadout create time (TUI wizard / CLI):**
- Show compat level per item as users select content
- Items below Full compat require explicit confirmation
- Display what specifically is lost (not just "degraded" — show "async hooks will block execution on Copilot CLI")

**At loadout apply time:**
- Preview shows compat level per item
- Degraded items show warnings
- Broken items block apply unless `--force` is used
- None items are excluded (cannot be applied)

**CLI flags:**
- `--allow-degraded` — include degraded items without per-item confirmation
- `--force` — include broken items (with prominent warning)

### 5. Conversion in Apply Pipeline

**Current pipeline (full, from code):**
```
Resolve -> Validate -> Preview -> [conflict check] -> Snapshot -> Apply -> [SessionEnd hook for try mode]
```

**New pipeline:**
```
Resolve -> Assess Compat -> Validate -> Preview -> [conflict check] -> Snapshot -> Convert+Apply -> [SessionEnd hook]
```

New steps:
- **Assess Compat:** After resolution, compute compat level for each resolved ref (source provider vs target provider). Gate based on compat policy (block Broken unless --force, warn on Degraded unless --allow-degraded).
- **Convert+Apply:** Merged step. For each action, if the resolved ref is cross-provider, read canonical content, call `converter.Render(content, targetProvider)`, and write the converted output. Same-provider items use symlink/copy as before.

Note: Conversion happens during Apply (not as a separate pre-step) because it needs to happen per-action alongside filesystem writes. This mirrors how `installWithRenderTo()` works in the installer.

**Hook and MCP conversion:** The current `applyHook()` and `applyMCP()` functions read raw JSON and merge directly — they bypass the converter. For cross-provider loadouts, these functions must detect when the resolved ref's source provider differs from the target, and route through `HooksConverter.Render()` / `MCPConverter.Render()` before merging. This is a structural change to both functions.

### 6. Install Method for Cross-Provider Content

Cross-provider items cannot use symlinks (the source file is canonical markdown, but the target might need MDC or TOML). These items use **copy with conversion** — the same approach `installer.installWithRenderTo()` already uses for individual cross-provider installs.

Same-provider items with `.source/` files can still use symlinks for lossless installs.

| Scenario | Install method |
|----------|---------------|
| Same provider, has `.source/` | Copy `.source/` file to install path (lossless, matches existing `installFromSourceTo()` behavior) |
| Same provider, no `.source/` | Symlink to canonical file (current behavior) |
| Cross-provider | Copy with conversion — `Render()` to target format, write to install path |

### 7. Tracking and Rollback

**Tracking changes needed.** Cross-provider items are copy-installed (not symlinked), so they need distinct tracking:

- **`installed.json`:** The existing `InstalledSymlink` struct has a `Target` field that assumes a symlink. Copy-installed files don't have a symlink target. Add a `Method` field (`"symlink"` or `"copy"`) to `InstalledSymlink` so the same struct handles both. The `Target` field remains populated for symlinks (for staleness checks) and is empty for copies.

- **`snapshot.SymlinkRecord`:** Currently tracks symlink path + target for rollback. Rename concept to `FileRecord` or extend to cover both symlinks and copied files. On rollback, symlinks are removed (os.Remove); copied files are also removed (os.Remove) — both are files the loadout created that shouldn't survive removal.

- **`loadout remove`:** Currently iterates `snapshot.Symlinks` and calls `os.Remove()`. This already works for both symlinks and regular files — `os.Remove()` deletes either. The key fix: ensure copy-installed files are added to the snapshot's file list during apply.

- **`--try` mode:** The SessionEnd auto-revert calls `loadout remove --auto`, which restores the snapshot. As long as copy-installed files are tracked in the snapshot, removal works correctly. No special handling needed beyond proper tracking.

- **Staleness detection:** `stale.go` checks whether installed symlinks still point to valid targets. For copy-installed files, staleness means "the file we wrote no longer exists at the expected path." Add a check for copy-installed files: verify the file exists at the tracked path.

## Existing Infrastructure

These systems are already built and will be leveraged:

| Component | Location | What it does |
|-----------|----------|-------------|
| Hub-and-spoke converters | `converter/*.go` | `Canonicalize()` / `Render()` per content type |
| Hook compat analysis | `converter/compat.go` | `CompatLevel`, `AnalyzeHookCompat()`, `HookCapabilities` matrix |
| Source provider tracking | `metadata/metadata.go` | `SourceProvider` field on content metadata |
| Cross-provider install | `installer/installer.go` | `installWithRenderTo()` — read, render, write |
| Converter warnings | `converter/converter.go` | `Result.Warnings` for data loss reporting |
| Tool name translation | `converter/toolmap.go` | Canonical <-> provider tool name mapping |
| Provider definitions | `provider/*.go` | `InstallDir`, `SupportsType`, `FileFormat` per provider |

## What Needs to Be Built

### New code

1. **Resolution fallback** — modify `findItem()` in `loadout/resolve.go` to prefer-then-fallback for provider-specific types (rules, hooks, commands). Universal types unchanged.
2. **Unified compat scorer** — extend `converter/compat.go` with analyzers for rules, skills, commands, agents, MCP. Each returns `CompatResult` with level + per-feature breakdown.
3. **Compat assessment step** — new function in `loadout/` that scores all resolved refs and gates inclusion based on compat policy flags.
4. **Conversion in apply** — modify `applyActions()` to detect cross-provider items and route through `converter.Render()` before writing.
5. **Hook/MCP cross-provider conversion** — modify `applyHook()` and `applyMCP()` to detect cross-provider refs and route through `HooksConverter.Render()` / `MCPConverter.Render()` before merging into settings/config files.

### Modified code

6. **`loadout/apply.go`** — insert compat assessment step after resolve. Modify `applyActions()` to handle cross-provider conversion. Ensure copy-installed files are tracked in snapshot.
7. **`loadout/preview.go`** — add compat level to `PlannedAction`. New action type `"create-copy"` for cross-provider items (distinct from `"create-symlink"`).
8. **`loadout/validate.go`** — integrate with compat scorer (currently only checks `SupportsType`).
9. **`loadout/create.go`** — surface compat info during manifest building.
10. **`loadout/remove.go`** — ensure removal handles both symlinks and copy-installed files (likely already works since `os.Remove()` handles both, but verify tracking).
11. **`loadout/stale.go`** — add staleness check for copy-installed files (file exists at tracked path).
12. **`installer/installed.go`** — add `Method` field to `InstalledSymlink` to distinguish symlink vs copy.
13. **`snapshot/` package** — ensure snapshot tracks copy-installed files alongside symlinks for rollback.
14. **CLI commands** — add `--allow-degraded` and `--force` flags to `loadout apply` and `loadout create`.
15. **TUI loadout wizard** — show compat indicators during item selection.

### Tests

16. **Compat scorer tests** — per content type, per provider pair.
17. **Resolution fallback tests** — same-provider priority, cross-provider fallback, collision handling, universal types unchanged.
18. **Loadout apply with conversion** — end-to-end: resolve cross-provider content, convert, install, verify output format matches target provider.
19. **Hook/MCP cross-provider apply** — end-to-end: cross-provider hook resolved, converted, merged into correct settings format.
20. **Compat gating tests** — degraded blocked without flag, broken blocked without --force, None excluded entirely.
21. **Remove/rollback tests** — loadout remove cleans up both symlinks and copy-installed files. `--try` mode auto-reverts correctly for cross-provider content.

## Non-Goals

- Cross-provider "universal loadouts" (one manifest, many providers) — deferred
- Automatic loadout generation from detected provider content — separate feature
- Loadout migration (convert an existing loadout from one provider to another) — separate feature
- Changes to the content directory structure — provider directories remain as-is

## Resolved Questions

- **Fallback ambiguity:** When multiple non-target providers match, catalog precedence order applies (deterministic but arbitrary between providers). Explicit source mapping (`{name: x, from: provider}`) deferred until needed.
- **Universal types:** Fallback resolution only applies to provider-specific types (rules, hooks, commands). Skills, agents, and MCP already resolve provider-agnostically.
- **`.source/` install method:** Copy (not symlink), matching existing `installFromSourceTo()` behavior.
- **Copy-installed file cleanup:** Tracked in snapshot alongside symlinks. `os.Remove()` handles both. `--try` mode works correctly as long as tracking is complete.

## Research Prerequisites

Deep-dive research is required for each content type before building compat scoring. The hook research (`docs/research/hook-research.md`) established the bar: per-provider technical comparisons of features, formats, semantics, and divergence points. The same depth is needed for:

1. **Rules research** — format differences (MD vs MDC vs TOML vs other), frontmatter fields and semantics (`alwaysApply`, `globs`, `description`), scoping (global vs project vs always-apply), directory structure expectations per provider. Which providers support rules at all.
2. **Skills research** — spec divergence across providers (the community is flagging this as a major pain point). Which providers support skills, what the skill definition looks like per provider, how hooks-within-skills work, trigger mechanisms, tool restrictions, model access.
3. **Commands research** — format (YAML+MD vs TOML vs other), argument handling, tool access restrictions, model references, execution context differences.
4. **Agents research** — definition format per provider, model references, tool access patterns, system prompt handling, which providers support agents at all.
5. **MCP research** — config key structures (`mcpServers` vs `servers`), transport types supported, environment variable handling, scoping (global vs project). Likely the simplest given MCP is a protocol standard.

Each research doc produces a feature matrix that directly feeds into the compat scorer. Without this, compat scoring would be guesswork.

**Sequencing:** Research all content types first, then build the unified compat scorer, then implement the resolution/conversion/apply pipeline changes.

## Resolved Questions (continued)

- **Compat scoring granularity:** Deep-dive research required for each content type (same depth as hook research). Feature matrices per provider feed directly into compat scorers. Not a shortcut — providers diverge more than initially assumed, especially for skills.
- **Compat UX — progressive disclosure:**
  - **CLI:** Badge inline (e.g., `[~] my-hook -- degraded`), `--verbose` flag shows full reasoning. Output includes hint: `Use --verbose to see compatibility details`.
  - **TUI:** Badge in item list. Click/enter on badge opens a modal showing the full feature breakdown with a close button.
- **`--to` flag cross-provider override:** Blocked unless `--force`. Applying a loadout to a different provider than it was authored for is an advanced operation. When `--force` is used, show the full compat summary before proceeding.

## Open Questions

1. **Converter coverage gaps:** Are there content type + provider combinations where `Render()` isn't implemented yet? These would be CompatNone until the converter is written. Need to audit before implementation.
2. **Research sequencing:** Which content type research to start with? Skills are flagged as the most divergent and painful in the community. Rules and MCP are likely simpler. Commands and agents are in between.
