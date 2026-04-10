# Implementation Plan: Hook Execution Audit Logging

**Bead:** syllago-6txg
**Date:** 2026-03-22
**Status:** Plan
**Addresses:** Security review finding P0-2 (enterprise-security audit, section 4)

---

## Motivation

The enterprise security review identifies audit logging as a P0 blocker for enterprise adoption. Without execution audit trails, security teams cannot investigate incidents: "which hooks ran during the timeframe of the breach?" The spec's security considerations document (section 4.4) recommends logging hook executions with timestamp, hook identity, event, exit code, and blocking status.

Syllago does not execute hooks itself -- providers do. But syllago manages hook installation, tracks what is installed (`installed.json`), and is the natural place to log install/uninstall lifecycle events. For actual execution logging, the spec recommends that hook scripts themselves write to the audit log, and syllago provides the infrastructure (log location, format, rotation, query CLI).

This plan covers two categories of audit events:

1. **Lifecycle events** (syllago controls): install, uninstall, update, security scan results
2. **Execution events** (hook scripts report): before/after tool execute, session start/end, exit codes, duration

---

## 1. Audit Log Format

JSON Lines (one JSON object per line, newline-delimited). This is the standard format for structured log ingestion -- compatible with `jq`, Splunk, Datadog, ELK, and `grep` without a parser.

### Lifecycle Event Schema

```json
{
  "ts": "2026-03-22T14:30:05.123Z",
  "version": 1,
  "event_type": "hook.install",
  "hook_name": "safety-check",
  "hook_event": "before_tool_execute",
  "provider": "claude-code",
  "source": "export",
  "command": "./safety-check.sh",
  "group_hash": "a1b2c3d4...",
  "scan_result": "pass",
  "scan_findings": 0
}
```

### Execution Event Schema

```json
{
  "ts": "2026-03-22T14:32:11.456Z",
  "version": 1,
  "event_type": "hook.execute",
  "hook_name": "safety-check",
  "hook_event": "before_tool_execute",
  "exit_code": 0,
  "blocked": false,
  "command_truncated": "./safety-check.sh",
  "duration_ms": 342,
  "matcher": "shell",
  "error": ""
}
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ts` | string (RFC 3339) | Yes | UTC timestamp with millisecond precision |
| `version` | int | Yes | Schema version (always `1` initially). Enables forward-compatible parsing. |
| `event_type` | string | Yes | One of: `hook.install`, `hook.uninstall`, `hook.execute`, `hook.timeout`, `hook.scan` |
| `hook_name` | string | Yes | Name of the hook (matches `installed.json` name) |
| `hook_event` | string | Yes | The lifecycle event (`before_tool_execute`, `session_start`, etc.) |
| `provider` | string | Lifecycle only | Provider slug (`claude-code`, `cursor`, etc.) |
| `source` | string | Lifecycle only | Install source (`export`, `loadout:<name>`, `registry:<url>`) |
| `command_truncated` | string | No | Command string, truncated to 200 chars. Full commands may contain sensitive args. |
| `group_hash` | string | Lifecycle only | SHA256 of the matcher group JSON |
| `exit_code` | int | Execution only | Process exit code (0=success, 1=error, 2=block) |
| `blocked` | bool | Execution only | Whether the hook blocked the triggering action |
| `duration_ms` | int | Execution only | Execution time in milliseconds |
| `matcher` | string | No | What the hook matched against (tool name, pattern, etc.) |
| `scan_result` | string | Scan only | `pass`, `warn`, `fail` |
| `scan_findings` | int | Scan only | Number of findings from security scan |
| `error` | string | No | Error message if the hook failed |

### Why JSON Lines Over Alternatives

- **SQLite:** More powerful queries, but adds a CGo dependency (or a pure-Go driver with limitations). The audit log is append-only and query patterns are simple (filter by time range + event type). JSON Lines is sufficient and keeps the dependency footprint zero.
- **CSV:** No nested fields, quoting headaches, no standard for optional fields.
- **Plain text:** Not machine-parseable without a custom parser.

---

## 2. Log Location

**Default:** `~/.syllago/audit.log`

This follows the existing convention: `~/.syllago/` already stores `hooks/<name>/` (script copies) and is referenced by `installed.json` paths. The audit log lives alongside the data it describes.

**Not configurable in v1.** Configuration adds complexity (config file field, env var, CLI flag precedence) for a feature nobody has asked to customize yet. The path is defined in a single function (`auditLogPath()`) so adding configurability later is trivial.

### Why Not Per-Project

Hooks are installed globally (into `~/.claude/settings.json` etc.), not per-project. The audit log matches the scope of the data: global hooks get a global audit log. Per-project audit logs would fragment the timeline and complicate incident response queries.

---

## 3. Log Rotation Strategy

**Size-based rotation** with a single backup.

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Max size | 10 MB | ~50,000-100,000 entries depending on field sizes. Months of data for typical usage. |
| Max backups | 1 | `audit.log` (current) + `audit.log.1` (previous). Two files = ~20 MB max disk. |
| Rotation trigger | On write, when current file exceeds max size | Checked before each append, not on a timer. |

### Rotation Mechanics

1. Before appending, check `audit.log` size via `os.Stat()`
2. If size > 10 MB: rename `audit.log` to `audit.log.1` (overwrites any existing `.1`)
3. Create new `audit.log` and append

This is deliberately simple. No compression, no date-stamped archives, no retention policies. Enterprises that need long-term retention will ship logs to their SIEM via the JSON Lines format -- they do not need syllago to manage archival.

### Why Not Time-Based

Time-based rotation (daily, weekly) creates unpredictably many files if the tool is used sporadically. Size-based is self-limiting: disk usage is bounded at ~20 MB regardless of usage pattern.

---

## 4. Integration Points in Existing Code

### 4.1 Lifecycle Events (syllago-controlled)

These are the events syllago directly controls. Each integration point is a single function call added to existing code.

**File: `cli/internal/installer/hooks.go`**

| Function | Event | Where |
|----------|-------|-------|
| `installHook()` | `hook.install` | After successful `SaveInstalled()` (line 138). Log hook name, event, provider, command, group hash. |
| `uninstallHook()` | `hook.uninstall` | After successful `SaveInstalled()` (line 232). Log hook name, event, provider. |

**File: `cli/internal/converter/hook_security.go`**

| Function | Event | Where |
|----------|-------|-------|
| `ScanHookSecurity()` | `hook.scan` | After scan completes, before returning findings. Log hook name, result (pass/warn/fail), finding count. |

### 4.2 Execution Events (hook-script-reported)

Syllago does not execute hooks -- providers do. For execution audit logging, we provide two mechanisms:

**Mechanism A: Environment variable injection**

When installing a hook, syllago can inject an `AUDIT_LOG` environment variable pointing to `~/.syllago/audit.log` into the hook's `env` block (for providers that support it). Hook scripts that want to participate in audit logging write a JSON line to this path.

This is opt-in and non-blocking. Hooks that do not write to the audit log simply do not produce execution entries. This is acceptable for v1 -- lifecycle events (install/uninstall) are the critical audit trail.

**Mechanism B: Wrapper script generation (future)**

For providers that support it, syllago could generate a thin wrapper script that:
1. Records start time
2. Executes the actual hook command
3. Records end time and exit code
4. Appends an execution audit entry

This is more invasive and changes the hook's execution model. Deferred to a future bead -- listed here for completeness.

### 4.3 New Package: `cli/internal/audit`

A new package keeps audit logic isolated from installer internals.

**Exported API (4 functions):**

```
WriteInstall(name, event, provider, command, groupHash string) error
WriteUninstall(name, event, provider string) error
WriteScan(name string, result string, findings int) error
Query(opts QueryOpts) ([]Entry, error)
```

Each `Write*` function opens the file, appends one JSON line, and closes. No persistent file handle -- audit writes are infrequent (install/uninstall) so the open/append/close overhead is negligible.

`Query` reads the log file(s), filters by the provided options, and returns parsed entries. Used by the CLI command.

---

## 5. CLI Command: `syllago audit`

```
syllago audit [flags]

Flags:
  --since <duration>    Show entries from the last duration (e.g., 1h, 24h, 7d)
  --event <name>        Filter by hook event (e.g., before_tool_execute, session_start)
  --type <event_type>   Filter by event type (install, uninstall, execute, scan)
  --hook <name>         Filter by hook name
  --json                Output raw JSON Lines (default: human-readable table)
```

### Examples

```bash
# All audit events from the last hour
syllago audit --since 1h

# Only before_tool_execute events
syllago audit --since 24h --event before_tool_execute

# Only install/uninstall lifecycle events
syllago audit --type install --type uninstall

# Raw JSON for piping to jq or SIEM ingestion
syllago audit --since 7d --json

# Events for a specific hook
syllago audit --hook safety-check --since 24h
```

### Human-Readable Output Format

```
TIMESTAMP              TYPE          HOOK             EVENT                  DETAILS
2026-03-22 14:30:05    install       safety-check     before_tool_execute    provider=claude-code scan=pass
2026-03-22 14:32:11    execute       safety-check     before_tool_execute    exit=0 duration=342ms
2026-03-22 14:35:22    execute       safety-check     before_tool_execute    exit=2 blocked duration=128ms
2026-03-22 15:01:00    uninstall     safety-check     before_tool_execute    provider=claude-code
```

### Implementation

New file: `cli/cmd/syllago/audit_cmd.go`

- Cobra command registered under root
- Parses `--since` as Go `time.Duration` (with `d` suffix support for days, since stdlib does not support it)
- Calls `audit.Query()` with filter options
- Formats output as table (default) or JSON Lines (`--json`)
- Supports `output.JSON` global flag (same as other commands)

---

## 6. Performance Impact Analysis

### Write Path (lifecycle events)

- **Frequency:** Hook installs/uninstalls are rare -- a few per session at most, often zero.
- **Cost per write:** `os.OpenFile` (append) + `json.Marshal` one struct + `file.Write` + `file.Close`. Estimated <1ms.
- **Rotation check:** One `os.Stat` call before each write. Negligible.
- **Impact:** Effectively zero. These operations already take 10-100ms (file I/O for settings.json, installed.json, snapshot creation). Adding <1ms is not measurable.

### Write Path (execution events, if wrapper scripts are used)

- **Frequency:** Every hook execution, which could be every tool call.
- **Cost:** Same as above, but the file open/close overhead matters more at high frequency.
- **Mitigation:** Wrapper scripts would use direct shell `echo >>` append, not Go file I/O. Shell append to a file is fast and atomic for single lines.
- **Impact:** ~1-2ms per hook execution. Acceptable given hooks already add 50-5000ms of execution time.

### Read Path (query command)

- **Frequency:** On-demand, user-initiated only.
- **Cost:** Sequential scan of up to 20 MB of JSON Lines. Parsing ~100K entries takes ~200-500ms on modern hardware.
- **Mitigation:** `--since` filter skips entries by timestamp without full parsing (can check just the `ts` prefix). Most queries will use `--since` and scan only recent entries.
- **Impact:** Sub-second for typical queries. Acceptable for an interactive CLI command.

### Disk Usage

- **Bounded:** 20 MB maximum (10 MB active + 10 MB backup).
- **Growth rate:** ~200 bytes per entry. At 100 entries/day, 10 MB lasts ~500 days before first rotation.

---

## 7. Test Cases

### Unit Tests: `cli/internal/audit/audit_test.go`

| Test | Description |
|------|-------------|
| `TestWriteInstall_CreatesFile` | First write creates `audit.log` and parent directory. Verify file exists and contains valid JSON. |
| `TestWriteInstall_AppendsToExisting` | Two writes produce two JSON lines. Verify both parse correctly. |
| `TestWriteUninstall_Fields` | Verify all fields are present and correctly populated (timestamp format, event_type, etc.). |
| `TestWriteScan_Fields` | Verify scan-specific fields (scan_result, scan_findings). |
| `TestCommandTruncation` | Command string >200 chars is truncated. Verify exact truncation point and no broken UTF-8. |
| `TestRotation_TriggeredAtMaxSize` | Write entries until file exceeds 10 MB. Verify `audit.log.1` created and `audit.log` is small again. |
| `TestRotation_OverwritesOldBackup` | Trigger rotation twice. Verify only one `.1` file exists (no `.2`). |
| `TestQuery_NoFilters` | Query returns all entries from the log. |
| `TestQuery_SinceFilter` | Write entries with varying timestamps. Query with `--since 1h` returns only recent entries. |
| `TestQuery_EventFilter` | Write entries with different hook events. Filter by `before_tool_execute` returns correct subset. |
| `TestQuery_TypeFilter` | Filter by `event_type` (install vs execute). |
| `TestQuery_HookNameFilter` | Filter by hook name. |
| `TestQuery_CombinedFilters` | Multiple filters applied simultaneously (AND logic). |
| `TestQuery_IncludesRotatedFile` | Query reads from both `audit.log` and `audit.log.1` (rotated file has older entries). |
| `TestQuery_EmptyLog` | Query on nonexistent or empty log returns empty slice, no error. |
| `TestSchemaVersion` | Every written entry has `"version": 1`. |
| `TestTimestampFormat` | Timestamps are valid RFC 3339 with millisecond precision and UTC timezone. |

### Integration Tests: `cli/internal/installer/hooks_audit_test.go`

| Test | Description |
|------|-------------|
| `TestInstallHook_WritesAuditEntry` | Call `installHook()`. Verify audit log contains an `hook.install` entry with correct hook name and provider. |
| `TestUninstallHook_WritesAuditEntry` | Install then uninstall. Verify audit log contains both `hook.install` and `hook.uninstall` entries. |
| `TestInstallHook_AuditOnlyOnSuccess` | Make `installHook()` fail (e.g., duplicate hook). Verify NO audit entry written. |

### CLI Tests: `cli/cmd/syllago/audit_cmd_test.go`

| Test | Description |
|------|-------------|
| `TestAuditCmd_DefaultOutput` | Run `syllago audit --since 24h`. Verify human-readable table format. |
| `TestAuditCmd_JSONOutput` | Run `syllago audit --json`. Verify each line is valid JSON. |
| `TestAuditCmd_SinceFlag` | Verify `--since` parses durations including day suffix (`7d`). |
| `TestAuditCmd_EventFlag` | Verify `--event` filters correctly. |
| `TestAuditCmd_EmptyResults` | Run on empty/nonexistent log. Verify clean "no entries" message, no error. |

---

## 8. File Inventory

New files to create:

| File | Purpose |
|------|---------|
| `cli/internal/audit/audit.go` | Core types (`Entry`, `QueryOpts`), `auditLogPath()`, `appendEntry()`, rotation logic |
| `cli/internal/audit/write.go` | `WriteInstall()`, `WriteUninstall()`, `WriteScan()` convenience functions |
| `cli/internal/audit/query.go` | `Query()` function with filtering |
| `cli/internal/audit/audit_test.go` | Unit tests for write, rotation, query |
| `cli/cmd/syllago/audit_cmd.go` | Cobra command definition and output formatting |
| `cli/cmd/syllago/audit_cmd_test.go` | CLI command tests |

Files to modify:

| File | Change |
|------|--------|
| `cli/internal/installer/hooks.go` | Add `audit.WriteInstall()` call in `installHook()`, `audit.WriteUninstall()` call in `uninstallHook()` |
| `cli/cmd/syllago/main.go` | Register `auditCmd` with root command |

---

## 9. Open Decisions

**Execution logging mechanism.** This plan fully specifies lifecycle audit logging (install/uninstall/scan). Execution logging (every time a provider runs a hook) requires either wrapper script generation or hook-script cooperation via environment variables. Both approaches have trade-offs that warrant a separate design discussion. The lifecycle audit trail alone satisfies the security review's core requirement -- "which hooks were installed and when" -- for incident response.

**Log path configurability.** Hardcoded to `~/.syllago/audit.log` in v1. If enterprises need a different path (e.g., writing to a shared network mount), adding a `config.yaml` field or `SYLLAGO_AUDIT_LOG` env var is a single-function change.
