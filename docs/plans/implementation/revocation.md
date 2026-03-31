# Hook Revocation Mechanism

**Bead:** syllago-ck9p
**Status:** Plan
**Date:** 2026-03-22

## Context

The security considerations doc (Section 3) defines content integrity via per-file SHA-256 hashes and optional signatures. The policy interface (Section 5) lists "Revocation of previously-allowed hooks" as future work. This plan covers the mechanism for revoking hooks that were previously trusted -- either because a vulnerability was discovered, a key was compromised, or an author acted maliciously.

The goal is a lightweight, offline-capable revocation system that integrates with syllago's existing registry and catalog infrastructure. It does not require a central authority; each registry maintains its own revocation list and syllago merges them locally.

---

## 1. Revocation List Format

Each registry publishes a `revocations.json` file at its root. The format is a JSON document with a version stamp and an array of revocation entries.

```json
{
  "schema": "revocations/1.0",
  "updated": "2026-03-22T14:30:00Z",
  "entries": [
    {
      "hook_id": "safety-check",
      "content_hash": "sha256:a1b2c3d4e5f6...",
      "revoked_at": "2026-03-22T14:30:00Z",
      "reason": "Command exfiltrates environment variables to external endpoint",
      "severity": "critical",
      "registry": "github.com/acme/hooks"
    }
  ]
}
```

### Field definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `hook_id` | string | yes | Hook directory name (matches catalog item name). |
| `content_hash` | string | no | SHA-256 hash of the specific revoked version. When absent, ALL versions of this hook are revoked. |
| `revoked_at` | string (RFC 3339) | yes | Timestamp of revocation. |
| `reason` | string | yes | Human-readable explanation. Displayed to users during install checks and forced re-verification. |
| `severity` | string | yes | One of `"critical"`, `"high"`, `"medium"`. Controls urgency of notifications. |
| `registry` | string | yes | Source registry identifier. Allows merging lists from multiple registries without collision. |

### Why per-registry, not global

- No single authority owns the hook ecosystem. Registries are community-driven.
- A compromised global list would be a single point of failure for supply chain attacks.
- Per-registry lists let organizations maintain their own revocations alongside public ones.
- Syllago merges all registry revocation lists locally into a combined index for fast lookup.

### Why `content_hash` is optional

Two revocation modes serve different needs:

- **Version-specific** (`content_hash` present): Revokes a specific file version. If the author publishes a fixed version, the new version is not affected. This is the common case for vulnerability patches.
- **Blanket** (`content_hash` absent): Revokes all versions of a hook. Used when the author is compromised or the hook concept is inherently dangerous. Users must explicitly re-allow after review.

---

## 2. Distribution Mechanism

Revocation lists reach developer machines through the same channel hooks do: registry sync.

### Sync flow

1. **`syllago sync`** (existing command) pulls registry content via `git pull` or registry API.
2. After pulling content, syllago checks for `revocations.json` at the registry root.
3. If present, the file is parsed and merged into the local revocation index at `~/.syllago/cache/revocations.json`.
4. The merge is additive: entries are never removed from the local index by a sync. An entry can only be removed by the registry publishing an explicit "unrevoke" (see below) or the user running `syllago revocation clear`.

### Local index structure

The local index is a merged superset of all registry revocation lists, keyed by `(registry, hook_id, content_hash)` for deduplication:

```json
{
  "schema": "revocations-index/1.0",
  "last_synced": "2026-03-22T15:00:00Z",
  "entries": [...]
}
```

### Offline behavior

The local index is the source of truth for revocation checks. If a developer is offline, the last-synced index is used. This means there is a window between a revocation being published and a developer syncing -- this is inherent to any distributed revocation system (same trade-off as CRL vs OCSP in TLS).

### Staleness warning

If the local index is older than 7 days, syllago emits a warning during hook execution: "Revocation list is N days old. Run `syllago sync` to update." This balances offline usability with security hygiene.

---

## 3. Forced Re-Verification on Next Execution

When a revocation entry matches an installed hook, syllago must prevent silent execution.

### Check points

Revocation checks happen at two points:

1. **Install time** -- Before installing a hook, check the revocation index. If revoked, block installation and display the reason. This is the primary defense.
2. **Execution time** -- Before a hook executes (in the install/loadout-apply pipeline, not in the provider's runtime), verify the hook's content hash against the revocation index. This catches hooks that were installed before the revocation was published.

### Execution-time flow

```
Hook triggered by provider runtime
  -> syllago pre-execution check (if syllago manages the hook)
    -> Compute content hash of installed hook files
    -> Look up (registry, hook_id, content_hash) in local revocation index
    -> If match found:
       1. Block execution (hook does not run)
       2. Display: "Hook '{hook_id}' has been revoked: {reason}"
       3. Log revocation-block event to audit log
       4. Prompt: "Run `syllago remove {hook_id}` to uninstall, or `syllago inspect {hook_id}` to review"
    -> If no match: proceed normally
```

### Important constraint

Syllago can only enforce revocation for hooks it manages. If a user manually copies hook files into a provider's config directory, syllago has no interception point. This is documented as a known limitation -- the revocation mechanism protects the managed installation path, not arbitrary files.

### Re-verification vs. auto-removal

The mechanism blocks execution and notifies rather than auto-removing. Rationale:

- Auto-removal is destructive and may break a user's workflow unexpectedly.
- The user may want to inspect the hook before deciding what to do.
- A false-positive revocation (registry error) would silently break setups if auto-removal were the default.
- Blocking execution achieves the security goal (the revoked code does not run) without the irreversibility cost.

---

## 4. CLI Command: `syllago revoke`

### Usage

```
syllago revoke <hook-id> [flags]

Flags:
  --hash string       Revoke a specific version (SHA-256 content hash)
  --reason string     Reason for revocation (required)
  --severity string   One of: critical, high, medium (default: "high")
  --registry string   Registry to publish revocation to (default: auto-detect from hook source)
```

### Behavior

1. Validate the hook exists in the catalog or registry.
2. If `--hash` is not provided, compute the content hash of the current installed version. If neither installed nor `--hash` given, prompt: revoke all versions or specify a hash.
3. Create a revocation entry and append it to the registry's `revocations.json`.
4. Commit the change to the registry repo (if local checkout) or submit via registry API.
5. Log the revocation event to the audit log.
6. Print confirmation with the revocation details.

### Who can revoke

This command modifies the registry's `revocations.json`, so it is gated by the same write access controls as publishing hooks to the registry. In practice:

- **Git-based registries:** requires push access to the repo.
- **API-based registries (future):** requires authenticated write token.

Syllago does not implement its own authorization layer -- it delegates to the registry's access control.

### Unrevoke

```
syllago revoke --undo <hook-id> [--hash string] [--registry string]
```

Removes a revocation entry. Same access controls apply. The `--undo` flag is intentionally verbose to prevent accidental unrevocation.

---

## 5. Audit Logging Integration

All revocation events are logged to syllago's audit log (extending the audit logging recommended in security-considerations.md Section 4.4).

### Events logged

| Event | Fields | When |
|-------|--------|------|
| `revocation.published` | hook_id, content_hash, reason, severity, registry, actor | `syllago revoke` creates a new entry |
| `revocation.blocked` | hook_id, content_hash, reason, severity, registry, trigger_event | A revoked hook is blocked at execution time |
| `revocation.install_denied` | hook_id, content_hash, reason, severity, registry | Install attempt blocked due to revocation |
| `revocation.synced` | registry, entries_added, entries_total | `syllago sync` updates the local revocation index |
| `revocation.removed` | hook_id, content_hash, registry, actor | `syllago revoke --undo` removes an entry |
| `revocation.stale_warning` | last_synced, days_stale | Staleness warning emitted |

### Log format

Entries follow the existing audit log format (structured JSON, one line per event). The `category` field is `"revocation"` for all events above.

```json
{
  "timestamp": "2026-03-22T15:00:00Z",
  "category": "revocation",
  "event": "revocation.blocked",
  "hook_id": "safety-check",
  "content_hash": "sha256:a1b2c3d4e5f6...",
  "reason": "Command exfiltrates environment variables",
  "severity": "critical",
  "registry": "github.com/acme/hooks",
  "trigger_event": "before_tool_execute"
}
```

---

## 6. Handling Already-Installed Revoked Hooks

When a sync pulls new revocation entries, some may match hooks already installed on the machine.

### Detection

After every `syllago sync`, syllago scans the local catalog for installed hooks and cross-references against the updated revocation index. This scan is fast: it compares `(hook_id, content_hash)` tuples, not file contents (hashes are computed at install time and stored in the catalog metadata).

### Notification

If matches are found, syllago prints a summary:

```
WARNING: 2 installed hooks have been revoked since your last sync:

  [CRITICAL] safety-check (github.com/acme/hooks)
    Reason: Command exfiltrates environment variables to external endpoint
    Action: Hook will be blocked on next execution. Run `syllago remove safety-check` to uninstall.

  [HIGH] auto-format (github.com/team/tools)
    Reason: Vulnerability in path traversal handling (fixed in v2.1)
    Action: Hook will be blocked on next execution. Run `syllago update auto-format` to get the fixed version.
```

### Behavior between sync and removal

- The hook remains on disk (not auto-removed).
- On next execution, the revocation check blocks it (Section 3).
- The user resolves by removing, updating, or explicitly re-allowing.

### Explicit re-allow (override)

For cases where a user has reviewed the hook and wants to run it despite revocation (e.g., false positive, or the user patched it locally):

```
syllago allow <hook-id> [--hash string]
```

This adds a local override entry in `~/.syllago/config.yaml` under `revocation_overrides`. The override is:
- Scoped to the specific content hash (if provided) or the hook ID.
- Local only -- does not affect the registry.
- Logged as `revocation.overridden` in the audit log.
- Displayed as a warning on every execution: "Running revoked hook '{hook_id}' (locally overridden)."

---

## 7. Test Cases

### Unit tests

| Test | Description |
|------|-------------|
| `TestRevocationListParse` | Parse valid `revocations.json`, verify all fields. |
| `TestRevocationListParseEmpty` | Parse empty entries array -- valid, no revocations. |
| `TestRevocationListParseMalformed` | Reject malformed JSON with clear error. |
| `TestRevocationIndexMerge` | Merge two registry lists: no duplicates, additive only. |
| `TestRevocationIndexMergeIdempotent` | Merging the same list twice produces identical index. |
| `TestRevocationLookupByHash` | Lookup matches on `(registry, hook_id, content_hash)`. |
| `TestRevocationLookupBlanket` | Lookup with no `content_hash` matches any version of the hook. |
| `TestRevocationLookupMiss` | Non-revoked hook returns no match. |
| `TestRevocationStalenessWarning` | Index older than 7 days triggers warning. |
| `TestRevocationStalenessNoWarning` | Recent index does not trigger warning. |

### Install-time tests

| Test | Description |
|------|-------------|
| `TestInstallBlockedByRevocation` | Installing a revoked hook fails with revocation reason. |
| `TestInstallAllowedAfterUnrevoke` | Installing a hook succeeds after its revocation is removed. |
| `TestInstallSpecificHashRevoked` | Version-specific revocation blocks that version but allows others. |
| `TestInstallBlanketRevocation` | Blanket revocation blocks all versions. |

### Execution-time tests

| Test | Description |
|------|-------------|
| `TestExecutionBlockedByRevocation` | A hook installed before revocation is blocked on next run. |
| `TestExecutionBlockedAuditLog` | Blocked execution produces `revocation.blocked` audit entry. |
| `TestExecutionAllowedWithOverride` | `syllago allow` override lets a revoked hook execute with warning. |
| `TestExecutionOverrideAuditLog` | Overridden execution produces `revocation.overridden` audit entry. |

### CLI command tests

| Test | Description |
|------|-------------|
| `TestRevokeCommandCreatesEntry` | `syllago revoke` appends entry to `revocations.json`. |
| `TestRevokeCommandRequiresReason` | Missing `--reason` flag produces error. |
| `TestRevokeCommandAutoDetectsRegistry` | Without `--registry`, detects from hook's catalog metadata. |
| `TestRevokeUndoRemovesEntry` | `syllago revoke --undo` removes the entry. |
| `TestRevokeUndoNonexistent` | Undoing a non-existent revocation produces clear error. |
| `TestRevokeCommandAuditLog` | Revoke produces `revocation.published` audit entry. |

### Sync integration tests

| Test | Description |
|------|-------------|
| `TestSyncPullsRevocationList` | `syllago sync` fetches and merges `revocations.json`. |
| `TestSyncDetectsInstalledRevokedHooks` | Post-sync scan identifies installed hooks that are now revoked. |
| `TestSyncNoRevocationFile` | Registry without `revocations.json` is handled gracefully (no error). |
| `TestSyncMergesMultipleRegistries` | Syncing two registries merges both into the local index. |

---

## Implementation Order

1. **Revocation list parsing and index** -- `internal/revocation/` package with types, parse, merge, lookup.
2. **Install-time check** -- Wire revocation lookup into the installer pipeline.
3. **Sync integration** -- Fetch `revocations.json` during `syllago sync`, merge into local index.
4. **Post-sync scan** -- After sync, cross-reference installed hooks against updated index.
5. **`syllago revoke` command** -- CLI command to publish revocation entries.
6. **Audit logging** -- Emit revocation events to the audit log.
7. **`syllago allow` override** -- Local override mechanism for false positives.
8. **Execution-time check** -- Block revoked hooks at execution time (requires hook execution interception).

Steps 1-4 are the core mechanism. Steps 5-7 are the management interface. Step 8 depends on how deeply syllago integrates with provider hook runtimes -- it may be deferred if syllago does not intercept execution today.
