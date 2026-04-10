# Import Redesign & Terminology Overhaul — Design Document

**Goal:** Redesign syllago's content lifecycle with clear vocabulary, a global content library, provider-neutral canonical format, and complete verb coverage for every user workflow.

**Decision Date:** 2026-03-04

---

## Problem Statement

Syllago's current vocabulary and content flow have accumulated confusion:

- **Import** writes to `<project>/local/`, which is confusing and project-scoped when most content should be globally available
- **Export** and **Install** overlap — both push content to providers, but with different interfaces (CLI vs TUI)
- **Promote** is ambiguous — it handles both team repo sharing and registry publishing
- **Canonical format** is called "Claude Code format," making syllago appear biased toward one provider
- Missing operations: no way to uninstall from providers, remove from library, or do quick format conversion
- "My Tools" sidebar naming feels prototype-quality

These issues compound: a user who adds content can't easily remove it, the vocabulary doesn't guide them to the right operation, and the internal format story creates unnecessary confusion.

---

## Proposed Solution

### Seven verbs across three domains

| Domain | Verb | Purpose |
|--------|------|---------|
| **Library** | `add` | Bring content into `~/.syllago/content/` from any source |
| **Library** | `remove` | Delete content from the library (auto-uninstalls from providers) |
| **Provider** | `install` | Activate library content in a target provider |
| **Provider** | `uninstall` | Deactivate content from a provider |
| **Utility** | `convert` | Render content to a target provider format for ad-hoc sharing |
| **Collaboration** | `share` | Contribute content to a team repo |
| **Collaboration** | `publish` | Contribute content to a community registry |

Plus `loadout` as a sub-command system for bundles.

### Content lifecycle

```
Any source (git URL, filesystem, provider, registry)
  ↓ add (copy + canonicalize)
~/.syllago/content/<type>/<name>/
  ├── content-file        ← syllago canonical format (what you see & edit)
  ├── .source/            ← original format preserved (hidden, for fidelity)
  └── .syllago.yaml       ← metadata (source, provenance, timestamps)
  ↓ install (symlink, convert+copy, or JSON merge)
Provider paths (e.g., ~/.claude/commands/, ~/.cursor/rules/)
  ↓ share / publish
Team repo or registry (with optional git automation)
```

---

## Architecture

### Global content library: `~/.syllago/content/`

All user-owned content lives in one location. No more project-local `local/` directory.

**Directory structure:**
```
~/.syllago/content/
  skills/
    <name>/
      SKILL.md           ← canonical content
      .source/SKILL.md   ← original if different format
      .syllago.yaml      ← metadata
  agents/
    <name>/
  rules/
    <provider>/
      <name>/
  hooks/
    <provider>/
      <name>/
  commands/
    <provider>/
      <name>/
  prompts/
    <name>/
  mcp/
    <name>/
```

Universal types (skills, agents, prompts, mcp) are flat: `<type>/<name>/`.
Provider-specific types (rules, hooks, commands) include provider: `<type>/<provider>/<name>/`.

**What this replaces:**
- `<project>/local/` — removed entirely
- "Create New" in the TUI also writes to `~/.syllago/content/`
- The catalog scanner already scans this directory (`ScanWithGlobalAndRegistries`)

### Syllago-native canonical format

The canonical format is rebranded from "Claude Code format" to "syllago-native format." The internal structs (`RuleMeta`, `SkillMeta`, `AgentMeta`, `HookData`) are already provider-neutral supersets — this is a naming and identity change, not a structural one.

**What changes:**
- Documentation and comments reference "syllago canonical format" instead of "Claude Code format"
- Struct names and field names reviewed for provider-neutral naming
- Claude Code becomes a regular spoke in the hub-and-spoke model (with its own render step)
- Content filenames may be updated to reflect syllago conventions

**What doesn't change:**
- The converter interface (`Canonicalize`/`Render`) stays identical
- The actual struct fields are already supersets of all providers
- Conversion fidelity is unchanged

### `.source/` directory (hidden, for fidelity)

When content is added from a non-canonical format, the original file is preserved in `.source/` alongside the canonical version.

**Purpose:** Lossless same-provider roundtrip. If you add a Cursor rule and later install it back to Cursor, the `.source/` version is used — no conversion artifacts.

**Rules:**
- Hidden from TUI and CLI (implementation detail)
- Never modified after initial add
- Install logic checks: if `.source/` exists and matches the target provider → use it (lossless)
- Otherwise → render from canonical

### Metadata: `.syllago.yaml`

```yaml
name: my-skill
type: skills
source_provider: cursor          # where it came from (provider slug or "filesystem" or "git")
source_url: https://github.com/user/repo  # for future update capability
source_type: git                 # git | filesystem | registry | provider
source_format: .mdc             # original file format
has_source: true                 # whether .source/ exists
added_at: 2026-03-04T10:30:00Z
added_by: syllago v0.1.0
```

Source tracking enables a future `syllago update` command without metadata migration.

---

## Key Decisions

| # | Decision | Choice | Reasoning |
|---|----------|--------|-----------|
| 1 | Scope | All at once, clean break | Pre-release, no users, no backwards compat needed |
| 2 | Content destination | `~/.syllago/content/` (global) | Single predictable location, already scanned by catalog |
| 3 | `local/` directory | Removed entirely | Confusion source, replaced by global library |
| 4 | Sidebar section | "Library" | Accurately describes personal content collection |
| 5 | `import` → `add` | Rename | "Add to library" is more natural than "import to library" |
| 6 | `remove` | New command | Library needs cleanup capability |
| 7 | `export` | Removed | Absorbed into `install` (no redirect, just gone) |
| 8 | `promote` | Removed | Split into `share` + `publish` |
| 9 | `install` | Absorbs export | `syllago install --to <provider>` handles all activation |
| 10 | `uninstall` | New command | Provider deactivation (reverse symlink/merge) |
| 11 | `convert` | New command | Ad-hoc format conversion for quick sharing |
| 12 | `share` | Replaces promote (team) | Contribute to team repo with opt-in git automation |
| 13 | `publish` | Replaces promote to-registry | Contribute to registry with opt-in git automation |
| 14 | Canonical format | Syllago-native | Rebrand existing structs for provider neutrality |
| 15 | `.source/` | Keep, hide | Fidelity for same-provider roundtrip |
| 16 | Install method | Provider-aware symlink default | Provider defs include `supports_symlinks` per content type |
| 17 | User choice | Symlink (default) or copy | Both CLI and TUI offer the choice |
| 18 | Loadouts | Provider-neutral manifest | One loadout installable to any provider |
| 19 | Loadout constraint | One active per provider (v1) | Simplifies conflict handling |
| 20 | Metadata | Store source info | Enables future `syllago update` |
| 21 | Name collisions | Registry-namespaced | Registry name qualifies content identity |
| 22 | Share/Publish workflow | Place + stage + offer git | Directory placement always, git automation optional |

---

## Command Reference

### Library management

```bash
# Add content to library
syllago add <source>                      # from filesystem path
syllago add <git-url>                     # from git repository
syllago add --from <provider>             # from a provider's installed content

# Remove content from library
syllago remove <name>                     # removes from library + uninstalls from all providers
syllago remove <name> --type skills       # disambiguate if name exists in multiple types
```

### Provider activation

```bash
# Install to a provider
syllago install <name> --to <provider>          # install one item
syllago install --to <provider> --type skills   # install all skills
syllago install <name> --to <provider> --method copy  # force copy instead of symlink

# Uninstall from a provider
syllago uninstall <name> --from <provider>      # remove from one provider
syllago uninstall <name>                        # remove from all providers
```

### Format conversion

```bash
# Quick convert for sharing
syllago convert <name> --to <provider>                    # output to stdout
syllago convert <name> --to <provider> --output ./path/   # output to file
```

### Collaboration

```bash
# Share to team repo
syllago share <name>                      # places in repo + stages
# → "Create branch and PR? [Y/n]"

# Publish to registry
syllago publish <name> --registry <name>  # places in registry clone + stages
# → "Create branch and PR? [Y/n]"
```

### Loadouts

```bash
# Existing commands (unchanged behavior)
syllago loadout list
syllago loadout apply <name> --to <provider>           # preview (default)
syllago loadout apply <name> --to <provider> --try     # temporary, auto-reverts
syllago loadout apply <name> --to <provider> --keep    # permanent
syllago loadout remove                                  # revert active loadout

# Loadout manifest is now provider-neutral (no required provider field)
```

---

## Data Flow

### Add flow

```
Source file (any format)
  → Copy to ~/.syllago/content/<type>/<name>/
  → If source format ≠ canonical: save original to .source/
  → Canonicalize content → save as canonical content file
  → Write .syllago.yaml metadata (including source_url)
```

### Install flow

```
Library item in ~/.syllago/content/
  → Check provider supports_symlinks for this content type
  → If target == source provider AND .source/ exists:
      → Symlink .source/ file to provider path (lossless)
  → Else if target format == canonical format:
      → Symlink canonical file to provider path
  → Else:
      → Render from canonical → target format
      → Copy rendered file to provider path
  → If hooks/MCP:
      → JSON merge into provider settings.json
      → Track in installed.json for clean uninstall
  → User can override: --method copy forces copy instead of symlink
```

### Uninstall flow

```
Installed content in provider path
  → If symlink: remove symlink
  → If copy: remove copied file
  → If JSON merge (hooks/MCP):
      → Read installed.json for tracked entries
      → Remove merged entries from settings.json
      → Update installed.json
```

### Share/Publish flow

```
Library item in ~/.syllago/content/
  → Copy canonical content to target directory (team repo or registry clone)
  → git add (stage changes)
  → Offer: "Create branch and PR? [Y/n]"
    → Yes: create branch, commit, push, open PR
    → No: "Changes staged — take it from here."
```

### Convert flow

```
Library item in ~/.syllago/content/
  → Render canonical → target provider format
  → Output to stdout or --output path
  → No state changes, no side effects
```

---

## TUI Changes

### Sidebar

- "My Tools" → "Library"
- Filter changes from `item.Local == true` to `item.Source == "global"` (content from `~/.syllago/content/`)
- `CountLocal()` → `CountLibrary()` (or equivalent)
- "Import" sidebar action → "Add"

### Install modal

- Default selection: symlink (pre-selected)
- Second option: copy
- Provider dropdown respects `supports_symlinks` — if provider doesn't support symlinks for this content type, symlink option is disabled with explanation

### Detail view

- Install tab: unchanged behavior, updated labels
- "Export" action: removed
- "Promote" action: replaced with "Share" and "Publish" options

---

## Provider Definition Changes

Add `supports_symlinks` map to provider definitions:

```go
type Provider struct {
    // ... existing fields
    SymlinkSupport map[ContentType]bool  // per-content-type symlink compatibility
}
```

This must be verified per-provider. Hooks and MCP are always `false` (JSON merge, not filesystem).

---

## CLI Design Standards

Adopted from [clig.dev](https://clig.dev/) — these patterns apply to all syllago commands.

### Standard flags (consistent across all commands)

| Flag | Meaning | Commands |
|------|---------|----------|
| `-h`, `--help` | Show help | All |
| `-n`, `--dry-run` | Show what would happen, don't do it | `install`, `uninstall`, `remove`, `loadout apply` |
| `-f`, `--force` | Skip confirmation prompts | `add` (overwrite), `remove`, `uninstall` |
| `-q`, `--quiet` | Suppress non-essential output | All |
| `-o`, `--output` | Output file path | `convert` |
| `--json` | Machine-readable JSON output | All |
| `--no-input` | Disable all interactive prompts, use defaults | All (especially `share`, `publish`, `install`) |
| `--method` | Install method: `symlink` (default) or `copy` | `install`, `loadout apply` |
| `--to` | Target provider | `install`, `convert`, `loadout apply` |
| `--from` | Source provider | `add`, `uninstall` |

### Next-command suggestions

Every mutation should suggest the logical next step. This teaches the workflow without docs.

```
$ syllago add ./my-cool-agent
+ Added agent 'my-cool-agent' to library.

  Next: syllago install my-cool-agent --to claude-code
```

```
$ syllago install my-cool-agent --to claude-code
+ Symlinked my-cool-agent to ~/.claude/agents/my-cool-agent.md

  Next: syllago install my-cool-agent --to gemini    (another provider)
        syllago convert my-cool-agent --to cursor    (convert for sharing)
```

```
$ syllago uninstall my-cool-agent --from claude-code
+ Removed symlink: ~/.claude/agents/my-cool-agent.md

  my-cool-agent is still in your library.
  Remove with: syllago remove my-cool-agent
```

```
$ syllago remove my-cool-agent
? This will remove 'my-cool-agent' from your library and uninstall from all providers. Continue? [y/N]
+ Uninstalled from: claude-code, gemini
+ Removed from library.
```

### State change explanations

Every command that modifies the system must explain what it did:

- **add**: "Added <type> '<name>' to library" + source info
- **remove**: "Uninstalled from: <providers>. Removed from library."
- **install**: "Symlinked <name> to <path>" or "Copied <name> to <path>" or "Merged <n> hooks into <settings-file>"
- **uninstall**: "Removed symlink: <path>" or "Reversed hook merge in <file> (<n> hooks removed)"
- **convert**: "Rendered <name> as <provider> format to <output-path>"
- **share**: "Copied <name> to <repo-path>. Changes staged."
- **publish**: "Copied <name> to <registry-path>. Changes staged."

### Confirm before danger

| Severity | Commands | Behavior |
|----------|----------|----------|
| **Mild** | `add` (overwrite existing) | Prompt: "Overwrite existing <name>? [y/N]". Skip with `--force`. |
| **Moderate** | `remove`, `uninstall` | Prompt by default. Show what will be affected. Skip with `--force`. |
| **Moderate** | `loadout apply --keep` | Show preview first (already designed). |
| **None** | `install`, `convert`, `share`, `publish` | Non-destructive or easily reversible — no confirmation needed. |

### Conversational error messages

Rewrite errors for humans. Include what happened, why, and what to do about it.

```
# Bad
Error: ENOENT: ~/.cursor/rules/my-rule.mdc

# Good
Can't install to Cursor — the rules directory doesn't exist.
  Expected: ~/.cursor/rules/
  You might need to open Cursor first to create its config directory.
```

```
# Bad
Error: item not found in catalog

# Good
No skill named 'code-reveiw' found in your library.
  Did you mean 'code-review'?
  Hint: syllago list --type skills    (show all skills in your library)
```

### Deprecated command redirects

```
$ syllago export --to cursor
Unknown command 'export'.
  To install content into a provider: syllago install <name> --to cursor
  To convert for sharing:            syllago convert <name> --to cursor

$ syllago import ./my-skill
Unknown command 'import'.
  To add content to your library:    syllago add ./my-skill

$ syllago promote my-skill
Unknown command 'promote'.
  To share with your team:           syllago share my-skill
  To publish to a registry:          syllago publish my-skill --registry <name>
```

### TTY detection and non-interactive mode

- All prompts (confirmations, share/publish git automation, install method choice) require TTY
- When `stdin` is not a TTY: use defaults silently (no prompts)
- `--no-input` flag explicitly disables prompts even on TTY
- If required input is missing in non-interactive mode: fail with clear instructions

```
$ syllago remove my-skill --no-input
+ Uninstalled from: claude-code
+ Removed from library.
# (no confirmation prompt — --no-input implies --force for confirmations)

$ echo "yes" | syllago remove my-skill
Error: Cannot prompt for confirmation (not a terminal).
  Use --force to skip confirmation: syllago remove my-skill --force
```

### Output routing

- Primary output (results, state changes) → `stdout`
- Progress indicators, warnings → `stderr`
- `--json` output → `stdout` (structured, for piping)
- `--quiet` suppresses all non-essential output (only errors remain)

### Progress indicators

Long operations must show activity within 100ms:

- `add <git-url>` → spinner: "Cloning repository..."
- `install` with conversion → spinner: "Converting to <provider> format..."
- `share` / `publish` → spinner: "Staging changes..."
- `loadout apply` → progress bar for multi-item installs

### No catch-all subcommands

Every operation requires an explicit subcommand. `syllago <name>` without a verb is an error:

```
$ syllago my-cool-agent
Unknown command 'my-cool-agent'.
  To add to library:      syllago add my-cool-agent
  To install to provider: syllago install my-cool-agent --to <provider>
  To show details:        syllago show my-cool-agent
```

### Color usage

- Green: success indicators (checkmarks, "Added", "Installed")
- Red: errors and destructive action warnings
- Yellow: warnings, confirmations, deprecation notices
- Dim/gray: secondary information (paths, hints)
- Disable when: `NO_COLOR` set, `TERM=dumb`, `--no-color` flag, stdout not a TTY

---

## Implementation Approach

Refactor in place — the existing architecture is well-decoupled and survives the redesign.

### New files (5)

| File | Purpose | Logic source |
|------|---------|-------------|
| `cli/cmd/syllago/install_cmd.go` | CLI `install --to <provider>` | Harvest from `export.go` (converter pipeline, cross-provider logic) |
| `cli/cmd/syllago/share_cmd.go` | CLI `share` (team repo) | Harvest from `promote.go` (git workflow) |
| `cli/cmd/syllago/publish_cmd.go` | CLI `publish` (registry) | Wrap `registry_promote.go` |
| `cli/cmd/syllago/remove_cmd.go` | CLI `remove` (library cleanup) | New logic (catalog lookup + uninstall + delete) |
| `cli/cmd/syllago/convert_cmd.go` | CLI `convert` (ad-hoc format conversion) | New logic (canonicalize → render → output) |

### Deleted files (2)

| File | Reason |
|------|--------|
| `cli/cmd/syllago/export.go` | Absorbed into `install_cmd.go` |
| `cli/cmd/syllago/promote_cmd.go` | Replaced by `share_cmd.go` + `publish_cmd.go` |

### Refactored files (~15)

| File | Lines | Change scope | Key changes |
|------|-------|-------------|-------------|
| `cli/cmd/syllago/import.go` | 453 | ~5% | Rename to `add`, swap destination paths |
| `cli/cmd/syllago/loadout_apply.go` | 217 | ~5% | Make `provider` field optional in manifest |
| `cli/internal/tui/import.go` | 1,772 | ~2% | `destinationPath()` and `batchDestForSource()` path swap, title string |
| `cli/internal/tui/app.go` | 1,754 | ~3% | Promote handler → share/publish split, importer path |
| `cli/internal/tui/detail.go` | 1,005 | ~6% | Promote key → share/publish, SymlinkSupport in install tab |
| `cli/internal/tui/sidebar.go` | 193 | ~1% | "My Tools" → "Library" string |
| `cli/internal/promote/promote.go` | 219 | ~10% | Source path from `~/.syllago/content/` instead of `local/` |
| `cli/internal/installer/installer.go` | 321 | ~5% | SymlinkSupport check in method selection |
| `cli/internal/catalog/scanner.go` | 560 | ~7% | Remove `local/` scan, update source tagging |
| `cli/internal/catalog/types.go` | 192 | ~10% | `Local` → `Library` field, `MyTools` → `Library` virtual type |
| `cli/internal/provider/provider.go` | 77 | ~5% | Add `SymlinkSupport` field |
| Provider definition files (×11) | ~50 each | ~2% each | Set `SymlinkSupport` per content type |

### Untouched (~20+ files)

All converter files, most installer internals, registry promote logic, loadout apply/remove logic, snapshot system, metadata package.

### Landmine tracking

These areas require careful trace-through to avoid silent breakage:

- `item.Local` — referenced in 6+ places across scanner, sidebar, items, app
- `CountLocal()` — drives sidebar count, will return 0 if not updated
- Hook import path — hardcoded `local/` path in CLI import.go line 391 (separate from main path logic)
- `--base-dir` override — must be preserved when export is absorbed into install
- `Install()` provider dispatch — `item.Provider` may be unset for global content

---

## Out of Scope (Future Work)

- `syllago update` — re-pull content from source URL (metadata captures source info now)
- Multi-loadout per provider — stacking multiple loadouts
- Content versioning — tracking versions beyond source_url
- Real-time sync — auto-updating installed content when library changes
- Project-scoped content — if needed, team repos serve this purpose via registries

---

## Success Criteria

1. All seven commands work end-to-end (add, remove, install, uninstall, convert, share, publish)
2. Loadouts can be installed to any provider from a single provider-neutral manifest
3. Content lives in `~/.syllago/content/` and is visible in the TUI's "Library" section
4. No references to "export," "promote," "import," or "My Tools" in the codebase
5. Canonical format documentation references "syllago-native format," not "Claude Code format"
6. `.source/` enables lossless same-provider roundtrip (verified by existing roundtrip tests)

---

## Supersedes

This design supersedes the earlier `docs/2026-03-03-import-redesign-and-terminology.md` document, which was the initial exploration that led to this comprehensive redesign.

---

## Next Steps

Ready for implementation planning with the Plan skill.
