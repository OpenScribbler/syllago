# Git-Based Registry System - Design Document

**Goal:** Support enterprise/org distribution of AI tools without building a marketplace, using git-based content sources that organizations manage independently.

**Decision Date:** 2026-02-23

---

## Problem Statement

Syllago currently assumes a single content repository. Organizations need a way to distribute curated AI tool collections (skills, rules, hooks, prompts, agents, etc.) to their teams. Building a centralized marketplace (ClawHub/OpenClaw model) creates trust and moderation problems. Instead, we need a decentralized approach where orgs control their own content registries and Syllago provides the protocol.

## Proposed Solution

A **registry system** — git repositories that follow a known content structure. Organizations host their own registries on any git host. Syllago clones them locally, scans them for content, and makes items browsable/installable through both CLI and TUI.

Key properties:
- **Per-project registries** stored in `.syllago/config.json` — when teams share the config via git, everyone gets the same registries automatically
- **Global cache** at `~/.syllago/registries/<name>/` — clones are shared across projects, not duplicated per-project
- **Shell out to git** for clone/pull — no new Go dependencies needed
- **Registry items behave like shared items** — browsable and installable from TUI, but NOT included in `syllago export` (which only operates on `my-tools/`)

## Architecture

### Components

| Component | Responsibility |
|-----------|---------------|
| `config.go` | Config schema with `Registry` struct and `Registries` field |
| `registry/registry.go` | Git operations: clone, sync, remove, cache management |
| `registry_cmd.go` | CLI commands: `syllago registry add/remove/list/sync/items` |
| `catalog/scanner.go` | `ScanWithRegistries()` for multi-source content scanning |
| `catalog/types.go` | `Registry` field on `ContentItem`, `RegistrySource` type |
| `tui/registries.go` | New TUI screen for browsing registries |
| `tui/sidebar.go` | "Registries" entry in Configuration section |
| `tui/items.go` | `[registry-name]` prefix for registry items |
| `tui/detail_render.go` | Registry name in breadcrumb |

### Data Flow

```
.syllago/config.json → registries[] → git clone/pull → ~/.syllago/registries/<name>/
                                                              ↓
                                                     catalog.ScanWithRegistries()
                                                              ↓
                                                     ContentItem{Registry: "name"}
                                                              ↓
                                              TUI (browse) / CLI (list/install)
```

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Storage location for registry config | `.syllago/config.json` (per-project) | Teams share config via git — everyone gets same registries automatically |
| Clone location | `~/.syllago/registries/<name>/` (global) | Shared across projects, avoids duplicating large repos |
| Git implementation | Shell out to `git` CLI | No new Go dependencies; git is universally available |
| Scanning approach | New `ScanWithRegistries()` function | Avoids changing existing `Scan()` call sites; opt-in for callers that need registries |
| Registry items in export | Excluded | `syllago export` operates on `my-tools/` only — registry items are shared, not owned |
| Name derivation | Last path segment of git URL minus `.git` | Simple, predictable, overridable with `--name` flag |
| Security model | Warnings on add, no verification | Decentralized trust — org controls their registry, user decides to trust it |
| Install status detection | Extend CheckStatus with registry paths | Symlinks to registry cache dirs need the same detection as symlinks to repoRoot |
| Name collisions | Allow duplicates, distinguish by `[registry-name]` tag | Same as Homebrew taps — same-named items from different registries coexist |
| Auto-sync | Explicit by default, configurable auto-sync with timeout | Config option `registryAutoSync` enables auto-sync on TUI launch (5s timeout) |
| Item display in TUI | Mixed into normal content type views (first-class) | Registry items appear alongside shared/local items in content type lists |
| Sidebar counts | Include registry items in totals | Consistent with first-class treatment — counts reflect all content sources |
| Private repo auth | Git handles it — no special handling | SSH keys, credential helpers, tokens all work transparently through git CLI |

## Implementation Steps

### Step 1: Extend config schema

**File:** `cli/internal/config/config.go`

Add `Registry` struct and `Registries []Registry` field (with `omitempty` for backward compat):

```go
type Registry struct {
    Name string `json:"name"`
    URL  string `json:"url"`
    Ref  string `json:"ref,omitempty"`
}
```

Config becomes:
```json
{
  "providers": ["claude-code"],
  "registries": [
    { "name": "team-tools", "url": "git@github.com:acme/syllago-tools.git" }
  ]
}
```

### Step 2: Create registry package

**New file:** `cli/internal/registry/registry.go`

Functions:
- `CacheDir() string` — returns `~/.syllago/registries`
- `CloneDir(name) string` — returns `~/.syllago/registries/<name>`
- `Clone(url, name, ref) error` — `git clone`, optionally checkout ref
- `Sync(name) error` — `git -C <dir> pull --ff-only`
- `SyncAll(registries) []SyncResult` — pull all
- `Remove(name) error` — `os.RemoveAll`
- `IsCloned(name) bool` — stat check
- `NameFromURL(url) string` — derive name from git URL (last path segment minus `.git`)

Validates names with `catalog.IsValidItemName()`. Checks `git` is on PATH before operations.

### Step 3: Create `syllago registry` command

**New file:** `cli/cmd/syllago/registry_cmd.go`

Subcommands following `config_cmd.go` pattern:
- `syllago registry add <git-url> [--name alias] [--ref branch]`
- `syllago registry remove <name>`
- `syllago registry list`
- `syllago registry sync [name]`

**`registry add` flow:**
1. Derive name from URL (or use `--name`)
2. Validate name, check for duplicates in config
3. Print security warning (prominent box on first-ever registry; brief line on subsequent adds)
4. Clone repo
5. Verify it has content directories (warn if not, don't fail)
6. Save to config

**Security warning (first registry add):**
```
┌──────────────────────────────────────────────────────┐
│                   SECURITY NOTICE                    │
│                                                      │
│  Registries contain AI tool content (skills, rules,  │
│  hooks, prompts) that will be available for install.  │
│  This content can influence how AI tools behave.     │
│                                                      │
│  Syllago does not operate, verify, or audit any        │
│  registry. You are responsible for reviewing what    │
│  you install. Only add registries you trust.         │
│                                                      │
│  The syllago maintainers are not affiliated with and   │
│  accept no liability for any third-party registry.   │
└──────────────────────────────────────────────────────┘
```

**Per-add reminder (every time):**
```
Warning: Registry content is unverified. Only add registries you trust.
```

### Step 4: Extend catalog for multi-source scanning

**File:** `cli/internal/catalog/types.go`
- Add `Registry string` field to `ContentItem` (empty = local repo)
- Add `RegistrySource` struct: `{Name, Path string}`
- Add `ByRegistry(name)` helper to `Catalog`

**File:** `cli/internal/catalog/scanner.go`
- Add `ScanWithRegistries(repoRoot, []RegistrySource) (*Catalog, error)`
- Calls existing `Scan()` first, then `scanRoot()` for each registry
- Tags items by matching `Path` prefix to registry path
- Logs warnings on scan errors but doesn't fail

### Step 5: Wire registries into catalog scanning

**File:** `cli/cmd/syllago/main.go`
- In `runTUI` (line ~87 and ~162): load config, build registry sources, call `ScanWithRegistries`
- Same for rescan points in `app.go` (lines 143, 175, 231)

**NOT wired into** (intentionally):
- `export.go` — operates on `my-tools/` only (unchanged)
- `init.go` / `installBuiltins` — local repo only
- `add.go` — adds from filesystem, not registries

### Step 6: TUI — Registries sidebar section + landing page card

The sidebar currently has two sections: "AI Tools" (8 content types + My Tools) and "Configuration" (Import, Update, Settings). We add "Registries" as a new item in the Configuration section.

**Sidebar (`cli/internal/tui/sidebar.go`):**
- Add "Registries" entry in the Configuration section
- Show count of registered registries next to it
- Update `totalItems()` to include the new entry (+1)
- Add `isRegistriesSelected()` selector method
- Sidebar receives registry count from config — add `registryCount int` field

**Landing page (`cli/internal/tui/app.go` — `renderContentWelcome`):**
- Add a "Registries" card in the Configuration section alongside Import, Update, Settings
- Description: "Manage git-based content sources from your team or organization"
- Clickable via zone mark, navigates to Registries screen

**New screen: `screenRegistries` (`cli/internal/tui/registries.go` — new file):**
- New `registriesModel` sub-model with its own Update/View
- Lists registered registries with: name, URL, item count, clone status
- Full input support: Arrow keys/j/k, Enter to drill in, Esc to go back, mouse clicks, scroll
- Zone marks on each row for click detection
- Enter on a registry → navigate to items view filtered to that registry
- CLI hint at bottom for `syllago registry add/remove/sync`
- Help overlay entry with registry-specific keybindings

**Navigation flow:**
1. Sidebar → "Registries" → `screenRegistries` (list of registries)
2. Enter on a registry → `screenItems` with items filtered by `item.Registry == name`
3. Enter on an item → `screenDetail` (existing, unchanged)
4. Esc navigates back through the stack

**File:** `cli/internal/tui/app.go`
- Add `screenRegistries` to the `screen` enum
- Add `registries registriesModel` field to `App` struct
- Handle Enter on sidebar "Registries" selection
- Handle Esc from screenRegistries
- Filter items by registry when entering from registries screen

### Step 7: TUI display for registry items

**File:** `cli/internal/tui/items.go`
- Add `[registry-name]` prefix for registry items (same pattern as `[LOCAL]` tag)

**File:** `cli/internal/tui/detail_render.go`
- Show registry name in breadcrumb (same pattern as `[LOCAL]` tag)

### Step 8: CLI registry browsing

**Add to `cli/cmd/syllago/registry_cmd.go`:**
- `syllago registry items [name]` — list items from a specific registry (or all registries)
- Supports `--type` filter and `--json` output
- Reuses `catalog.ScanWithRegistries()` + `catalog.ByRegistry()`

### Step 9: Extend installer for registry items

**File:** `cli/internal/installer/installer.go`
- Modify `CheckStatus()` to accept additional valid source roots (registry cache directories)
- When checking symlink targets, check against repoRoot AND each registry clone path
- `Install()` already uses `item.Path` as source — registry items point to `~/.syllago/registries/<name>/...` which works without changes
- Registry items use symlinks (same as shared items), enabling auto-update on `syllago registry sync`

### Step 10: Auto-sync configuration

**File:** `cli/internal/config/config.go`
- Add `registryAutoSync` to preferences map

**File:** `cli/cmd/syllago/main.go`
- In `runTUI()`: if `registryAutoSync` preference is set, sync all registries with a 5-second timeout before scanning
- On timeout or failure, silently fall back to cached content

### Step 11: Security disclaimer in README

Add a "Security" section covering:
- Syllago does not operate any registry or marketplace
- Third-party content is unverified — review before installing
- Hooks and MCP configs can execute arbitrary code

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `git` not on PATH | Error: "git is required for registry operations" |
| Clone fails (bad URL, auth, network) | Error with git's stderr, registry NOT added to config |
| Pull fails (dirty tree, conflicts) | Warning, suggest `syllago registry sync --force <name>` |
| Registry has no content directories | Warning on add, but still added (maybe content comes later) |
| Duplicate registry name | Error: "Registry '<name>' already exists" |
| Config missing `registries` key | Treated as empty list (backward compat) |

## Success Criteria

1. `syllago registry add <url>` clones a git repo and saves it to config
2. `syllago registry list` shows registered registries with sync status
3. `syllago registry sync` pulls latest from all registries
4. `syllago registry items <name>` lists items from a registry
5. `syllago registry remove <name>` removes from config and deletes clone
6. TUI shows "Registries" in sidebar and landing page
7. TUI registries screen lists registries with item counts
8. Entering a registry shows its items in the standard items view
9. Registry items show `[registry-name]` tag in lists and detail views
10. Existing configs without `registries` key load without errors
11. `syllago export` is unaffected — only operates on `my-tools/`

## Open Questions

None — all major decisions resolved during brainstorm review.

---

## Brainstorm Review (2026-02-23)

Reviewed the design against the existing codebase and resolved 6 design gaps:

1. **Install detection gap:** `CheckStatus()` uses `IsSymlinkedTo(target, repoRoot)` — registry items in `~/.syllago/registries/` would never be detected. **Resolution:** Extend CheckStatus to check registry paths too.
2. **Name collision handling:** Two registries could have same-named items. **Resolution:** Allow both, distinguish by `[registry-name]` tag (Homebrew tap pattern).
3. **Sync behavior:** Auto vs manual. **Resolution:** Explicit by default, configurable `registryAutoSync` with 5s timeout.
4. **Registry items in content views:** Should they mix in or stay separate? **Resolution:** Mixed into normal views (first-class). Registries screen manages registries, not items.
5. **Sidebar counts:** Include or exclude registry items? **Resolution:** Include — consistent with first-class treatment.
6. **Private repos:** Authentication. **Resolution:** Git handles it transparently. No syllago-specific auth needed.

Pattern validated against Homebrew taps and Helm chart repos — well-established decentralized registry model.

---

## Next Steps

Ready for implementation planning with `Plan` skill.
