# Implementation Plan: Content Updates and Registry Publishing

Two beads covering the content lifecycle after installation: checking for updates and publishing new content to registries.

**Beads:** syllago-sbiz (Hook update mechanism), syllago-tem7 (Registry publishing flow)

**Context:** The hook author review (`docs/reviews/spec-design-audit-hook-author.md`) identified two critical gaps in the distribution story:
1. "No versioning/update mechanism described" -- the `.syllago.yaml` has a `version` field but nothing consumes it
2. "Registry integration is unclear" -- no publish command, no content hashing, no author attribution on publish

These gaps apply to ALL content types, not just hooks. The plans below are content-type-agnostic.

---

## Part 1: Content Update Mechanism (syllago-sbiz)

### Problem

Users install content from registries. Registry maintainers push updates. Users have no way to know their installed content is outdated or to update it. The `installed.json` tracking file records what was installed and when, but not what version it was or where to check for a newer one.

### Design Decisions

**Why not git-based diffing?** Registries are git repos, so we could diff the installed snapshot against HEAD. But installed content is transformed (converted between providers, merged into settings files). Comparing raw git state to transformed installed state is unreliable. Version strings in `.syllago.yaml` are the right abstraction -- explicit, author-controlled, and comparable without understanding the transformation pipeline.

**Why semver?** The `updater` package already has `parseVersion()` and `versionNewer()` for syllago self-updates. Content versions should use the same format for consistency. The existing functions handle "major.minor.patch" with optional pre-release suffixes, which is sufficient.

**Scope: all content types, not just hooks.** The review focused on hooks, but rules, skills, agents, and MCP configs all have the same update gap. The mechanism should be generic.

### 1.1 Version Field in .syllago.yaml

The `Meta` struct in `cli/internal/metadata/metadata.go` already has a `Version string` field. What's missing:

- **On install:** Record the installed version in `installed.json`. Add a `Version` field to `InstalledHook`, `InstalledMCP`, and `InstalledSymlink` in `cli/internal/installer/installed.go`.
- **On install:** Record the source registry name. `InstalledHook` already has `Source` (e.g. "loadout:foo"), but we need a `SourceRegistry` field to know where to check for updates. `InstalledSymlink` and `InstalledMCP` need the same.
- **Validation on publish:** The `metadata.Validate()` function should warn (not error) when `version` is empty. For registry content, version is strongly recommended but not mandatory -- unversioned content simply can't participate in the update flow.

Changes to `installed.json` schema:

```go
// Added to InstalledHook, InstalledMCP, InstalledSymlink:
Version        string `json:"version,omitempty"`        // version at install time
SourceRegistry string `json:"sourceRegistry,omitempty"` // registry to check for updates
SourceItem     string `json:"sourceItem,omitempty"`     // item path within registry (for lookup)
```

**Files modified:** `cli/internal/installer/installed.go`, `cli/internal/installer/installer.go` (to populate new fields during install), `cli/internal/installer/hooks.go` (same for hook installs).

### 1.2 syllago update Command

New CLI command: `syllago update [name] [--all] [--dry-run] [--registry <name>]`

| Flag | Behavior |
|------|----------|
| `syllago update my-skill` | Update a specific installed item by name |
| `syllago update --all` | Check and update all installed items |
| `syllago update --dry-run` | Show what would be updated without changing anything |
| `syllago update --registry team-rules` | Only update items from a specific registry |

**Command file:** `cli/cmd/syllago/update_content_cmd.go` (not `update_cmd.go` which might conflict with the self-update TUI flow).

**Flow:**

1. Load `installed.json` for the project
2. For each installed item (or the named item):
   a. If `SourceRegistry` is empty, skip (can't check -- content was installed from local project, not a registry)
   b. Sync the registry (`registry.Sync()`) to get latest content
   c. Look up the item in the registry by `SourceItem` path
   d. Load its `.syllago.yaml` and compare `Version` against the installed `Version`
   e. If newer, queue for update
3. For each queued update:
   a. Create a snapshot (for rollback)
   b. Uninstall the old version
   c. Install the new version
   d. If install fails, restore from snapshot
4. Report results

**Package:** `cli/internal/installer/update.go` for the core logic (lives alongside install/uninstall). The command file in `cmd/syllago/` is a thin wrapper.

**JSON output format:**

```json
{
  "checked": 12,
  "updated": [
    {"name": "code-review", "type": "skills", "from": "1.0.0", "to": "1.2.0", "registry": "team/rules"}
  ],
  "up_to_date": 10,
  "skipped": [
    {"name": "local-rule", "reason": "no source registry"}
  ],
  "failed": []
}
```

### 1.3 Version Checking Against Registry

The core comparison function:

```go
// CheckForUpdates compares installed items against their source registries.
// Returns items where the registry version is newer than the installed version.
func CheckForUpdates(projectRoot string) ([]UpdateCandidate, error)
```

**Lookup strategy:** Each installed item records `SourceRegistry` (registry name) and `SourceItem` (relative path within registry, e.g. `skills/code-review`). To check:

1. `registry.CloneDir(sourceRegistry)` to get the local clone path
2. `filepath.Join(cloneDir, sourceItem)` to find the item directory
3. `metadata.Load(itemDir)` to read the current `.syllago.yaml`
4. `versionNewer(registryMeta.Version, installed.Version)` to compare

This reuses `registry.CloneDir` and `metadata.Load` -- no new packages needed.

**Edge cases:**
- Item deleted from registry: report as "source removed" (not an error)
- Item has no version in registry: skip with "unversioned" status
- Registry not cloned locally: skip with "registry unavailable" status
- Version string unparseable: treat as "0.0.0" (matches existing `parseVersion` behavior)

### 1.4 Update Notification

Two notification points:

**CLI notification on `syllago list` / `syllago info`:** After listing installed content, append a line like:
```
2 items have updates available. Run `syllago update --dry-run` to see details.
```

This is lightweight -- it reads `installed.json` and checks registry versions, no network calls (registries are already cloned locally). The check only runs if the registry was synced within the last 24 hours (check git log timestamp of the clone). If stale, print "Run `syllago registry sync` to check for updates" instead.

**TUI notification:** In the items list, show a badge next to items with available updates (e.g., a colored arrow or "UPDATE" label). The TUI already has badge rendering for "[LIBRARY]" and "[REGISTRY]" -- follow the same pattern. The update check happens during catalog scan, not on every render.

**File:** `cli/internal/installer/update.go` for `HasUpdatesAvailable(projectRoot string) (int, error)` -- returns count of items with available updates. Called by `list.go` and the TUI's catalog refresh.

### 1.5 Rollback on Failed Update

Updates are a two-phase operation: uninstall old, install new. If the install phase fails, the user is left with content removed but not replaced. The snapshot system (`cli/internal/snapshot/`) already handles this for loadouts.

**Strategy:** Reuse `snapshot.Create` and `snapshot.Restore`.

**Update flow with rollback:**

1. `snapshot.Create(projectRoot, "update:"+itemName, "keep", filesToBackup, symlinks, hookScripts)`
   - Back up the settings files that will be modified (for hooks/MCP: the provider's settings.json)
   - Back up symlink targets (for rules/skills: the symlink path)
2. Uninstall the old version (reuse existing `installer.Uninstall` path)
3. Install the new version (reuse existing `installer.Install` path)
4. If step 3 fails:
   a. `snapshot.Restore(snapshotDir, manifest)` -- puts files back
   b. `snapshot.Delete(snapshotDir)` -- clean up
   c. Return error with "rolled back to previous version" message
5. If step 3 succeeds:
   a. `snapshot.Delete(snapshotDir)` -- clean up, update is permanent
   b. Update `installed.json` with new version

**Gotcha:** For hooks and MCP configs (JSON merge into settings files), rollback means restoring the entire settings file from snapshot -- not surgically removing the new hook and re-adding the old one. This is correct because the snapshot captured the file state before any changes.

### 1.6 Test Cases (Updates)

**Unit tests in `cli/internal/installer/update_test.go`:**

| Test | What it verifies |
|------|------------------|
| `TestCheckForUpdates_NewerVersion` | Detects when registry has newer version than installed |
| `TestCheckForUpdates_SameVersion` | Returns empty when versions match |
| `TestCheckForUpdates_NoSourceRegistry` | Skips items without SourceRegistry field |
| `TestCheckForUpdates_DeletedFromRegistry` | Reports "source removed" for missing items |
| `TestCheckForUpdates_UnversionedRegistry` | Skips items with no version in registry |
| `TestCheckForUpdates_UnversionedInstalled` | Treats missing installed version as "0.0.0" |
| `TestUpdateItem_Success` | Full update cycle: snapshot -> uninstall -> install -> cleanup |
| `TestUpdateItem_RollbackOnFailure` | Verifies snapshot restore when install fails |
| `TestUpdateItem_SymlinkContent` | Update for symlink-based content (rules, skills) |
| `TestUpdateItem_JSONMergeContent` | Update for JSON-merge content (hooks, MCP) |

**Command tests in `cli/cmd/syllago/update_content_cmd_test.go`:**

| Test | What it verifies |
|------|------------------|
| `TestUpdateCmd_DryRun` | --dry-run shows candidates without modifying anything |
| `TestUpdateCmd_NamedItem` | Updating a specific item by name |
| `TestUpdateCmd_AllFlag` | --all updates all eligible items |
| `TestUpdateCmd_JSONOutput` | JSON output format matches spec |
| `TestUpdateCmd_NoUpdatesAvailable` | Clean output when everything is current |

**Integration test pattern:** Create a temp registry (using `createBareRepo` from existing registry test helpers), install an item, bump the version in the registry, sync, then run update. Verify the installed content matches the new version.

---

## Part 2: Registry Publishing Flow (syllago-tem7)

### Problem

The existing `syllago publish` command (`cli/cmd/syllago/publish_cmd.go`) copies library content to a registry clone and creates a git branch + PR. But it has no validation pipeline, no content hashing, no author attribution, and no registry index. The hook author review asks: "How do hooks get into a syllago registry? Is there a publish command?" -- technically yes, but it's incomplete.

### Design Decisions

**Enhance, don't replace.** The existing `publish` command and `promote.PromoteToRegistry()` function handle the git workflow (branch, commit, push, PR). The gaps are pre-publish validation, content integrity, and registry discoverability. We add validation and metadata enrichment to the existing pipeline rather than building a parallel one.

**Registry index is optional.** Registries work today via filesystem scanning (the catalog scanner walks the directory tree). An index file is an optimization for large registries -- it lets users browse content without cloning. Small registries don't need it. The index is generated on publish but not required for registry functionality.

**Content hashing serves trust, not deduplication.** The hash in the registry index lets users verify that installed content matches what was published. It's a security/integrity feature, not a storage optimization.

### 2.1 syllago publish Command CLI Interface

The existing command already has:
- `syllago publish <name> --registry <registry> [--type <type>] [--no-input]`

**Add these flags:**

| Flag | Purpose |
|------|---------|
| `--validate-only` | Run validation without publishing. Exit 0 if valid, exit 1 with errors if not. |
| `--skip-security` | Skip the security scan (for CI environments where scan is done separately) |
| `--message <msg>` | Custom commit message (default: "Add {type}: {name}") |

**Updated flow in `runPublish()`:**

1. **Load and validate** the item (new step -- see 2.2)
2. **Compute content hash** (new step -- see 2.4)
3. **Enrich author attribution** (new step -- see 2.5)
4. Call existing `promote.PromoteToRegistry()` which handles git workflow
5. **Update registry index** if it exists (new step -- see 2.3)

Steps 1-3 happen before the git workflow. Step 5 happens after the commit but before the push.

**File changes:** `cli/cmd/syllago/publish_cmd.go` (add flags, add pre-publish pipeline), `cli/internal/promote/registry_promote.go` (accept enriched metadata, update index).

### 2.2 Validation Before Publish

A `ValidateForPublish()` function that runs a stricter validation pipeline than the general `metadata.Validate()`. Publishing to a registry has higher quality requirements than local use.

**Location:** `cli/internal/promote/validate.go`

**Checks, in order:**

1. **Schema validation** -- existing `metadata.Validate()` checks (`.syllago.yaml` exists, required fields present, primary content file exists per type)
2. **Required metadata for publish:**
   - `id` must be set (already required by Validate)
   - `name` must be set (already required)
   - `description` must be non-empty (warning in general validation, error for publish)
   - `version` must be set and parseable as semver
   - `type` must match the directory the item lives in
3. **Security scan** -- for hooks specifically, run the existing `converter.ScanHookSecurity()` from `cli/internal/converter/hook_security.go`. For all types, scan for:
   - Embedded credentials patterns (API keys, tokens)
   - Suspicious script content (curl-to-bash, encoded payloads)
   - Filesystem paths that reference user home directories or system paths
4. **Content completeness:**
   - README.md exists (warning in general validation, error for publish)
   - For hooks: `hook.json` is valid JSON and has required fields (`event`, `handler`)
   - For MCP: `config.json` is valid JSON
5. **Privacy gate** -- already exists in `promote.CheckPrivacyGate()`, runs before any publish operation

**Return type:**

```go
type PublishValidation struct {
    Errors   []string // blocking -- publish cannot proceed
    Warnings []string // non-blocking -- publish proceeds with warnings shown
}
```

**The `--validate-only` flag** runs this pipeline and prints results without touching git. Useful for CI pre-checks and local authoring feedback.

### 2.3 Registry Index Format

A `registry-index.json` file at the registry root that catalogs all published content with metadata. This is an optimization -- registries work without it (catalog scanner walks the filesystem), but with it:
- Users can browse registry content without cloning (future: web UI, API)
- `syllago registry items` can read the index instead of scanning if available
- The index includes content hashes for integrity verification

**Schema:**

```json
{
  "generated_at": "2026-03-22T14:30:00Z",
  "generator": "syllago v0.5.0",
  "items": [
    {
      "name": "code-review",
      "type": "skills",
      "path": "skills/code-review",
      "version": "1.2.0",
      "description": "AI-assisted code review guidelines",
      "author": "sam@example.com",
      "content_hash": "sha256:a1b2c3d4...",
      "tags": ["review", "quality"],
      "provider": "",
      "updated_at": "2026-03-20T10:00:00Z"
    },
    {
      "name": "safety-hook",
      "type": "hooks",
      "path": "hooks/claude-code/safety-hook",
      "version": "2.0.1",
      "description": "Blocks dangerous shell commands",
      "author": "sam@example.com",
      "content_hash": "sha256:e5f6g7h8...",
      "tags": ["safety", "shell"],
      "provider": "claude-code",
      "updated_at": "2026-03-21T15:00:00Z"
    }
  ]
}
```

**Location:** `cli/internal/registry/index.go`

**Functions:**

- `LoadIndex(registryDir string) (*Index, error)` -- reads `registry-index.json`, returns nil if absent
- `UpdateIndex(registryDir string, item IndexEntry) error` -- adds/updates an entry and writes the file
- `GenerateIndex(registryDir string) (*Index, error)` -- full scan to rebuild index from filesystem (for `syllago registry reindex` command)
- `VerifyHash(registryDir string, entry IndexEntry) (bool, error)` -- checks content hash against actual files

**Integration with publish:** After `promote.PromoteToRegistry()` commits the content, but before push, call `registry.UpdateIndex()` to add the new item to the index. The index update is included in the same commit.

**Optional `syllago registry reindex` subcommand** for registry maintainers to rebuild the full index from scratch. Useful after manual edits or migrations.

### 2.4 Content Hashing on Publish

Every published item gets a SHA-256 hash of its content. The hash covers the content files (not metadata or README), providing a tamper-detection mechanism.

**What gets hashed:**

| Content type | Files included in hash |
|-------------|----------------------|
| Skills | `SKILL.md` |
| Agents | `AGENT.md` |
| MCP | `config.json` |
| Rules | `rule.md` (or primary rule file) |
| Hooks | `hook.json` + all files referenced by `command` field |
| Commands | `command.md` (or primary command file) |

**Algorithm:** Sort the file list alphabetically, concatenate `filepath + \0 + content` for each file, SHA-256 the result. This is deterministic regardless of filesystem ordering.

**Location:** `cli/internal/promote/hash.go`

```go
// ContentHash computes a deterministic SHA-256 hash of the content files
// in an item directory. Only content files are included (not .syllago.yaml,
// README.md, or other metadata).
func ContentHash(itemDir string, contentType catalog.ContentType) (string, error)
```

**Storage:** The hash is written to two places:
1. The `.syllago.yaml` `source_hash` field (already exists in `Meta` struct) -- travels with the content
2. The `registry-index.json` `content_hash` field -- enables verification without reading every file

**Verification on install:** When installing from a registry, if the item has a `source_hash`, compute the hash of the source files and compare. Mismatch means the content was modified after publishing (or the hash is from a different version). This is a warning, not a blocker -- registries are git repos, so tampering is already visible via git history.

### 2.5 Author Attribution

When content is published to a registry, record who published it and when.

**Git-based attribution:** The publish flow already creates a git commit. The commit author IS the attribution. But this information isn't surfaced in the registry index or in the item's metadata.

**On publish, enrich `.syllago.yaml`:**

```yaml
author: "Sam <sam@example.com>"      # from git config user.name + user.email
source_type: registry
source_registry: "team/rules"
```

The `author` field already exists in `Meta`. The publish flow should populate it from git config if it's empty (don't overwrite an explicitly set author).

**Location:** Add to `promote.PromoteToRegistry()` before the commit step:

1. Read git config `user.name` and `user.email` from the registry clone
2. If item's `.syllago.yaml` has no `author`, set it to `"Name <email>"`
3. Write the updated `.syllago.yaml` before staging

**In registry index:** The `author` field in `IndexEntry` comes from the item's `.syllago.yaml`. If still empty after enrichment, fall back to the git commit author.

**Privacy consideration:** The existing `promote.CheckPrivacyGate()` already blocks publishing private content to public registries. Author attribution is separate -- it's about credit, not privacy. Users who publish to public registries accept that their git identity is visible.

### 2.6 Test Cases (Publishing)

**Unit tests in `cli/internal/promote/validate_test.go`:**

| Test | What it verifies |
|------|------------------|
| `TestValidateForPublish_Valid` | Passes for well-formed content with all required fields |
| `TestValidateForPublish_MissingVersion` | Error when version is empty |
| `TestValidateForPublish_MissingDescription` | Error when description is empty |
| `TestValidateForPublish_MissingReadme` | Error when README.md is absent |
| `TestValidateForPublish_HookSecurityScan` | Catches dangerous patterns in hook scripts |
| `TestValidateForPublish_CredentialDetection` | Catches embedded API keys/tokens |
| `TestValidateForPublish_SkipSecurity` | Respects skip-security flag |

**Unit tests in `cli/internal/promote/hash_test.go`:**

| Test | What it verifies |
|------|------------------|
| `TestContentHash_Deterministic` | Same files produce same hash regardless of scan order |
| `TestContentHash_ExcludesMetadata` | `.syllago.yaml` and `README.md` not in hash |
| `TestContentHash_IncludesHookScripts` | Hook hash covers `hook.json` + referenced scripts |
| `TestContentHash_DifferentContent` | Different content produces different hash |

**Unit tests in `cli/internal/registry/index_test.go`:**

| Test | What it verifies |
|------|------------------|
| `TestUpdateIndex_NewItem` | Adds entry to empty index |
| `TestUpdateIndex_ExistingItem` | Updates entry when item already in index |
| `TestGenerateIndex_FullScan` | Rebuilds index from filesystem |
| `TestVerifyHash_Match` | Returns true when content matches hash |
| `TestVerifyHash_Mismatch` | Returns false when content was modified |
| `TestLoadIndex_Missing` | Returns nil when no index file exists |

**Command tests in `cli/cmd/syllago/publish_cmd_test.go`:**

| Test | What it verifies |
|------|------------------|
| `TestPublish_ValidateOnly` | --validate-only runs checks without git operations |
| `TestPublish_ValidationFailure` | Exits with error when validation fails |
| `TestPublish_AuthorEnrichment` | Populates author from git config when empty |
| `TestPublish_ContentHashWritten` | Hash appears in both .syllago.yaml and index |
| `TestPublish_JSONOutput` | Updated JSON output includes hash and author |

**Integration test:** Create a bare registry repo, scaffold an item with full metadata, publish it, verify:
- The `.syllago.yaml` in the registry has `author` and `source_hash`
- The `registry-index.json` has the correct entry
- Cloning the registry and running `syllago registry items` shows the published item

---

## Implementation Order

These two beads are independent but share some infrastructure. Recommended sequence:

1. **installed.json schema changes** (1.1) -- foundation for both beads
2. **Content hashing** (2.4) -- small, self-contained, no dependencies
3. **Publish validation** (2.2) -- uses existing patterns, high value
4. **Author attribution** (2.5) -- small addition to publish flow
5. **Registry index** (2.3) -- depends on hashing and attribution
6. **Version checking** (1.3) -- depends on installed.json changes
7. **Update notification** (1.4) -- depends on version checking
8. **Update command + rollback** (1.2, 1.5) -- depends on everything above

Steps 2-5 can be done as one PR (the publish bead). Steps 1, 6-8 as another (the update bead). Step 1 ships with whichever lands first.

## Files Summary

**New files:**
- `cli/cmd/syllago/update_content_cmd.go` -- update command
- `cli/cmd/syllago/update_content_cmd_test.go`
- `cli/internal/installer/update.go` -- update checking and execution logic
- `cli/internal/installer/update_test.go`
- `cli/internal/promote/validate.go` -- publish validation pipeline
- `cli/internal/promote/validate_test.go`
- `cli/internal/promote/hash.go` -- content hashing
- `cli/internal/promote/hash_test.go`
- `cli/internal/registry/index.go` -- registry index read/write
- `cli/internal/registry/index_test.go`

**Modified files:**
- `cli/internal/installer/installed.go` -- add Version, SourceRegistry, SourceItem fields
- `cli/internal/installer/installer.go` -- populate new fields during install
- `cli/internal/installer/hooks.go` -- populate new fields during hook install
- `cli/cmd/syllago/publish_cmd.go` -- add flags, pre-publish pipeline
- `cli/internal/promote/registry_promote.go` -- author enrichment, index update, hash storage
- `cli/internal/metadata/validate.go` -- description warning for general, error path for publish
