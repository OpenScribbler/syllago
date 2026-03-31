# Add Command UX Redesign

**Date:** 2026-03-05
**Status:** Reviewed
**Feature:** `syllago add` interactive flow and conflict detection

## Problem Statement

The current `syllago add --from <provider>` command has three UX problems discovered during manual testing:

1. **No discovery step.** The command vacuums up everything from the provider and writes it to the library with no chance to see what was found or pick what you want.
2. **Bad conflict messaging.** When content already exists, it asks "Overwrite existing 'agents'? [y/N]" — which is confusing and doesn't communicate what exists or why.
3. **Unclear granularity.** Running `add` with no target implies "add everything" but users may want bulk, type-level, or individual item selection. All three intents get the same flow.

## Design Decisions

### Decision 1: No-args = discovery mode

Running `syllago add --from <provider>` with no positional target becomes **informational only**. It shows what content the provider has, annotated with library status (new, already added, source outdated).

The discovery output starts with a header that makes the informational nature explicit, so users don't mistake it for a confirmation that content was added.

This replaces the `--preview` flag, which becomes unnecessary.

### Decision 2: Positional target for adding

To actually add content, users provide a positional argument specifying what:

```bash
syllago add skills --from claude-code              # All skills
syllago add skills/my-skill --from claude-code     # Specific item
syllago add --all --from claude-code               # Everything
```

This reads naturally ("add skills from claude-code") and separates the "what" from the "where."

Specifying both `--all` and a positional target is an error: "Cannot specify both a target and --all."

### Decision 3: Hash-based conflict detection (file-based content only)

On add, store a SHA-256 hash of the raw source content in `.syllago.yaml` metadata. On re-add, compare hashes to detect three states:

| State | Condition | Behavior |
|-------|-----------|----------|
| **New** | Item not in library | Add normally |
| **Up to date** | Hash matches | Skip, show "up to date" |
| **Source outdated** | Hash differs | Skip, show "source changed (use --force to update)" |

No interactive prompts in the default flow. `--force` overrides both skip states.

**Scope:** Hash comparison applies to file-based content types (rules, skills, agents, commands, prompts, MCP). Hooks are excluded because they're split from `settings.json` and JSON key ordering makes naive hashing unreliable. Hooks use existence-based detection only: "in library" (exists) or "new" (doesn't exist).

### Decision 4: Destination is always global library

`~/.syllago/content/` is the one canonical location. No destination picker. The discovery footer includes "See also" hints for users who actually wanted `convert` or `install`.

### Decision 5: Discovery shows library status

The no-args discovery output annotates each item with its library status, so users can see what's new vs what they already have:

```
$ syllago add --from claude-code

Discovered content from Claude Code:
  Skills (2):
    code-review       (in library)
    security          (new)
  Rules (3):
    style             (in library, outdated)
    testing           (new)
    docs              (new)
  Hooks (1):
    pre-push          (new)

Add by type:
  syllago add skills --from claude-code
  syllago add rules --from claude-code
  syllago add hooks --from claude-code

Add a specific item:
  syllago add skills/security --from claude-code

Add everything:
  syllago add --all --from claude-code

See also:
  Convert format:    syllago convert <item> --to <provider>
  Install content:   syllago install <item> --to <provider>
```

The footer is **contextual**:
- Type examples only list types that were actually discovered
- Item example uses a real discovered item name (preferring one with "new" status)
- "Add by type" only lists types with at least one "new" or "outdated" item

**Status labels:**
- `(new)` — not in library
- `(in library)` — exists, hash matches (up to date)
- `(in library, outdated)` — exists, hash differs (source changed since last add)

The word "outdated" is unambiguous — it means the library has a stale version. Avoids "updated" which could be confused with "we already updated it."

For hooks (no hash comparison): only `(new)` and `(in library)`.

### Decision 6: Drop --preview, keep --dry-run

- `--preview` is replaced by no-args discovery mode. Delete the flag outright (pre-release, no users to migrate).
- `--dry-run` stays for use with a target: shows exactly what would be written without writing.

### Decision 7: Delete --type and --name flags

These are replaced by the positional argument:
- `--type skills` → positional `skills`
- `--type skills --name my-skill` → positional `skills/my-skill`

Delete outright — no hidden deprecated aliases needed. Pre-release tool with no external users.

## Command Syntax

```
syllago add [<type>[/<name>]] --from <provider> [flags]

Arguments:
  <type>           Content type to add (skills, rules, hooks, agents, etc.)
  <type>/<name>    Specific item within a type
  (omit)           Discovery mode — show what's available without adding

Flags:
  --from <provider>   Provider to add from (required)
  --all               Add all discovered content (cannot combine with positional target)
  --force, -f         Overwrite existing items (update changed + re-add identical)
  --dry-run           Show what would be written without writing
  --json              JSON output
  --no-input          Disable interactive prompts (for scripting)
  --exclude <name>    Skip items by name (repeatable, hooks only)
  --scope <scope>     Settings scope: global, project, all (hooks only)
  --base-dir <path>   Override base directory for content discovery
```

The `--help` description explicitly notes that omitting the positional argument triggers discovery mode.

### Usage Patterns

```bash
# Discovery: what does this provider have?
syllago add --from claude-code

# Add by type
syllago add skills --from claude-code
syllago add rules --from cursor
syllago add hooks --from claude-code

# Add specific item
syllago add skills/code-review --from claude-code
syllago add rules/security --from cursor

# Bulk add everything
syllago add --all --from claude-code

# Force-update changed items
syllago add skills --from claude-code --force

# Dry run
syllago add skills --from claude-code --dry-run

# JSON output (for TUI or scripting)
syllago add --from claude-code --json
```

## Output Examples

### Discovery Mode (no target)

```
$ syllago add --from claude-code

Discovered content from Claude Code:
  Skills (2):
    code-review       (in library)
    security          (new)
  Rules (3):
    style             (in library, outdated)
    testing           (new)
    docs              (new)
  Hooks (1):
    pre-push          (new)

Add by type:
  syllago add skills --from claude-code
  syllago add rules --from claude-code
  syllago add hooks --from claude-code

Add a specific item:
  syllago add skills/security --from claude-code

Add everything:
  syllago add --all --from claude-code

See also:
  Convert format:    syllago convert <item> --to <provider>
  Install content:   syllago install <item> --to <provider>
```

### Adding by Type

```
$ syllago add skills --from claude-code

  code-review       up to date
  security          added

Added 1 skill. 1 up to date.
```

### Adding with Updates Available

```
$ syllago add rules --from claude-code

  style             source changed (use --force to update)
  testing           added
  docs              added

Added 2 rules. 1 has updates (use --force).
```

### Force Update

```
$ syllago add rules --from claude-code --force

  style             updated
  testing           added (no changes)
  docs              added (no changes)

Updated 1 rule. 2 already up to date.
```

### Everything Up to Date

```
$ syllago add skills --from claude-code

  code-review       up to date
  security          up to date

2 skills already up to date.
```

### Empty Discovery

```
$ syllago add --from kiro

No content found for Kiro.

  Note: Kiro does not support Skills, Agents
  Searched for Rules in: ~/.kiro/rules/ (not found)
```

### Type Not Supported

```
$ syllago add rules --from kiro

Kiro does not support Rules.

Supported types for Kiro: Commands
```

### Item Not Found

```
$ syllago add skills/nonexistent --from claude-code

No skill named "nonexistent" found in Claude Code.

Available skills: code-review, security
```

### JSON Discovery

```json
{
  "provider": "claude-code",
  "groups": [
    {
      "type": "skills",
      "count": 2,
      "items": [
        {"name": "code-review", "path": "/home/user/.claude/skills/code-review", "status": "in_library"},
        {"name": "security", "path": "/home/user/.claude/skills/security", "status": "new"}
      ]
    }
  ]
}
```

Note: JSON output uses absolute paths (not tilde-collapsed).

## Metadata Changes

Add `SourceHash` field to the `Meta` struct and `.syllago.yaml`:

```yaml
id: abc123
name: code-review
type: skills
source_provider: claude-code
source_format: md
source_type: provider
source_hash: sha256:a1b2c3d4e5f6...   # NEW: hash of raw source content
has_source: false
added_at: "2026-03-05T10:00:00Z"
added_by: syllago-v0.1.0
```

**Schema changes required:**
1. Add `SourceHash string` field to `metadata.Meta` struct
2. Add `source_hash` YAML tag for marshal/unmarshal
3. Compute hash in write path (`writeAddedContent`)
4. Read hash in discovery path (library index scan)

**Cleanup:** Consolidate `ImportedAt`/`ImportedBy` vs `AddedAt`/`AddedBy` if they're redundant (leftover from import→add rename). Resolve before shipping since metadata is already being changed.

Hash is computed from the raw source bytes before canonicalization. This means the hash detects when the provider's content has changed, regardless of whether canonicalization would produce the same output.

## Implementation Architecture

### Core Functions (for TUI reuse)

The add flow is decomposed into shared functions in a new `add` package (or within `parse`) that both the CLI and TUI can call:

```go
package add // or extend parse package

// DiscoveryItem represents a discovered content item annotated with library status.
type DiscoveryItem struct {
    Name        string
    Type        catalog.ContentType
    Path        string              // Absolute path to source file
    Status      ItemStatus          // New, InLibrary, Outdated
    Description string
}

type ItemStatus int
const (
    StatusNew ItemStatus = iota
    StatusInLibrary   // exists, hash matches (or hooks: just exists)
    StatusOutdated    // exists, hash differs
)

// DiscoverFromProvider discovers content from a provider and annotates
// each item with its library status by comparing source hashes.
// For hooks, status is existence-based only (no hash comparison).
func DiscoverFromProvider(prov provider.Provider, resolver *config.PathResolver) ([]DiscoveryItem, error)

// AddResult tracks the outcome for a single item.
type AddResult struct {
    Name    string
    Type    catalog.ContentType
    Status  AddStatus // Added, Updated, UpToDate, Skipped, Error
    Error   error
}

type AddStatus int
const (
    AddStatusAdded AddStatus = iota
    AddStatusUpdated
    AddStatusUpToDate
    AddStatusSkipped   // source changed but --force not set
    AddStatusError
)

// AddItems writes selected items to the library. Returns per-item results.
func AddItems(items []DiscoveryItem, opts AddOptions) []AddResult

type AddOptions struct {
    Force    bool
    DryRun   bool
    Provider string
}
```

**Library index for performance:** `DiscoverFromProvider` builds a library index upfront by scanning `~/.syllago/content/` once into a `map[string]*metadata.Meta` keyed by `type/provider/name` (or `type/name` for universal types). Per-item lookups are then O(1) rather than individual filesystem reads.

### Hash Comparison Flow

```
1. Build library index: scan ~/.syllago/content/, load .syllago.yaml for each item
2. Discover files from provider via DiscoverWithResolver
3. For each discovered file:
   a. Compute SHA-256 of raw content
   b. Look up in library index by type + name (+ provider for non-universal)
   c. If not found → StatusNew
   d. If found and hash matches → StatusInLibrary
   e. If found and hash differs → StatusOutdated
4. For hooks: skip hash comparison, use existence only
   a. If hook dir exists → StatusInLibrary
   b. If not → StatusNew
5. On add: write content + store source_hash in .syllago.yaml
```

## Hooks Special Case

Hooks continue to use the separate `runAddHooks` path because they're stored in `settings.json` and need splitting. The new positional syntax routes `syllago add hooks --from <provider>` to this path (replacing `--type hooks`).

**Status annotations for hooks:** Existence-based only. A hook shows "(in library)" if its directory exists in `~/.syllago/content/hooks/<provider>/<name>/`, or "(new)" if it doesn't. No hash comparison — JSON key ordering makes it unreliable.

The `--exclude` and `--scope` flags remain hooks-specific. They should be hidden from default `--help` output and documented in the hooks section of the Long description. Using them with non-hook targets silently ignores them (not an error — just inapplicable).

## Non-Interactive Behavior

When stdin is not a terminal or `--no-input` is set:

- **Discovery mode** (no target): Works normally, shows discovery output
- **Add with target**: Adds new items, skips existing (same as interactive but no prompts)
- **Add with --all**: Same as above — adds new, skips existing
- **Add with --force**: Adds new + updates existing

No behavior changes needed — the default "inform and skip" approach is already non-interactive friendly.

## Error Cases

| Scenario | Behavior |
|----------|----------|
| Unknown provider | "Unknown provider 'foo'. Available: claude-code, cursor, ..." |
| Unknown type | "Unknown content type 'widgets'. Available: skills, rules, ..." |
| Type not supported | "Kiro does not support Rules. Supported types for Kiro: Commands" |
| Item not found | "No skill named 'missing' found in Claude Code. Available skills: ..." |
| No content found | Show diagnostic (which types unsupported, which paths searched) |
| --all with no content | "No content found for <provider>." + diagnostics |
| --all with positional | "Cannot specify both a target and --all" |
| --from missing | Custom error: "Missing --from flag. Usage: syllago add [type] --from <provider>" |

For the "item not found" case: after filtering discovery results by the requested `<type>/<name>`, if the filtered list is empty, return the named error rather than silently reporting "Added 0 items."

## Test Migration

The following test patterns in `add_cmd_test.go` must be rewritten:

| Old Pattern | New Pattern |
|-------------|-------------|
| `addCmd.Flags().Set("type", "rules")` | `addCmd.RunE(addCmd, []string{"rules"})` |
| `addCmd.Flags().Set("type", "hooks")` | `addCmd.RunE(addCmd, []string{"hooks"})` |
| `addCmd.Flags().Set("name", filter)` | `addCmd.RunE(addCmd, []string{"skills/" + filter})` |
| `addCmd.Flags().Set("preview", "true")` | No target arg = discovery mode |
| `runAddPreviewJSON` helper | Rewrite: set JSON output, run with no target |

This affects 8+ existing tests. Plan the migration as explicit tasks.

## Out of Scope

- Destination selection (always global library)
- Project-local content library
- Interactive multi-select picker (TUI handles this)
- Diffing library vs source content (hash detects change; viewing diff is a future feature)
- Hash-based detection for hooks (deferred due to JSON key ordering complexity)
