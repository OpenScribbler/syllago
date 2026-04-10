# Pluggable Hook Scanner Interface

**Bead:** syllago-5zb8
**Status:** Plan
**Addresses:** Security review section 2.3 ("What Enterprise Actually Needs" -- pluggable scanner interface)

---

## Problem

The current `ScanHookSecurity()` in `cli/internal/converter/hook_security.go` is a single regex-based scanner that only examines the `command` field in hook JSON manifests. It does not scan script file contents, and enterprises cannot integrate their own SAST tools (Semgrep, ShellCheck, etc.) into the hook install pipeline.

The security review calls for a `HookScanner` interface that accepts a hook directory path and returns findings, letting enterprises plug in their own scanning pipeline.

## Design

### 1. HookScanner Interface

New file: `cli/internal/converter/scanner.go`

```go
// ScanFinding represents a single security concern found by a scanner.
type ScanFinding struct {
    Severity    string // "high", "medium", "low", "info"
    File        string // path to the file containing the finding (relative to hook dir)
    Line        int    // line number (0 if not applicable, e.g. manifest-level findings)
    Description string // human-readable explanation of the concern
    Scanner     string // name of the scanner that produced this finding (e.g. "builtin", "semgrep")
}

// ScanResult is the aggregated output from one or more scanners.
type ScanResult struct {
    Findings []ScanFinding
    Errors   []string // non-fatal scanner errors (e.g. "semgrep not found in PATH")
}

// HookScanner analyzes a hook directory and returns security findings.
//
// hookDir is the absolute path to the hook directory containing hook.json
// and any referenced script files. The scanner must not modify the directory.
type HookScanner interface {
    Name() string
    Scan(hookDir string) (ScanResult, error)
}
```

**Why this shape:**

- `ScanFinding` replaces `SecurityWarning` as the unified result type. It adds `File`, `Line`, and `Scanner` fields that the current struct lacks -- these are essential for actionable output ("what file, what line, which scanner found it").
- `ScanResult` wraps findings with a separate `Errors` slice so that scanner failures (missing binary, timeout) don't silently swallow real findings from other scanners.
- The interface takes a directory path, not raw bytes. This is the key change from the current function signature -- it enables scanners to examine script files, not just the manifest JSON. The security review's core complaint is that the current scanner checks the label on the bottle, not what's inside.

### 2. Default Scanner (Wrapping Current Regex Scanner)

New type in `cli/internal/converter/scanner.go`:

```go
type BuiltinScanner struct{}
```

`BuiltinScanner` implements `HookScanner` by:

1. Reading `hook.json` from the hook directory
2. Calling the existing `ScanHookSecurity()` on the manifest bytes (preserving all current regex patterns)
3. Converting each `SecurityWarning` into a `ScanFinding` with `File: "hook.json"`, `Line: 0`, `Scanner: "builtin"`
4. **New:** Also scanning `.sh`, `.ps1`, `.py`, `.bash` files found in the hook directory against `dangerPatterns`. This directly addresses security review finding 2.2 ("the scanner only examines the command field... never opens check.sh to see what it does").

Script file scanning reads each script line by line and runs the same `dangerPatterns` regex set. Findings include the actual file name and line number. This is a straightforward extension of the existing approach -- not a new analysis engine.

`SecurityWarning` stays as-is for now (the existing type is only used internally by the regex scanner). `ScanFinding` is the public output type.

### 3. External Scanner (Subprocess)

New type in `cli/internal/converter/scanner.go`:

```go
type ExternalScanner struct {
    Path string // absolute path to scanner executable
}
```

`ExternalScanner` implements `HookScanner` by executing the binary as a subprocess:

```
/path/to/scanner <hookDir>
```

**Protocol:** The external scanner receives the hook directory as its sole argument and writes JSON to stdout:

```json
{
  "findings": [
    {
      "severity": "high",
      "file": "check.sh",
      "line": 12,
      "description": "curl with unquoted variable expansion"
    }
  ],
  "errors": ["optional non-fatal error messages"]
}
```

Exit codes:
- `0` -- scan completed (findings may or may not be present)
- `1` -- scan completed with findings (alternative to exit 0 + non-empty findings; both are accepted)
- `2+` -- scanner error (stderr captured into `ScanResult.Errors`)

**Timeout:** 30 seconds default. The subprocess is killed (SIGKILL after 5s grace) if it exceeds the timeout. This prevents a broken scanner from blocking the install flow indefinitely.

**Why subprocess, not a plugin system:** Go plugin system (`plugin` package) is fragile and platform-dependent. Subprocess execution is universal, language-agnostic, and the standard pattern for tool integration (same as how git hooks, pre-commit, and linters work). The JSON-over-stdout protocol is trivial to implement in any language.

### 4. Scanner Chaining

New function in `cli/internal/converter/scanner.go`:

```go
func ChainScanners(scanners ...HookScanner) HookScanner
```

Returns a `chainedScanner` that runs all scanners sequentially and merges their `ScanResult`s:

- All `Findings` are concatenated (each already tagged with `Scanner` name)
- All `Errors` are concatenated
- If any scanner returns a hard error, it is recorded in `Errors` and scanning continues with the remaining scanners (fail-open per scanner, not fail-closed for the whole chain)

**Why fail-open per scanner:** If Semgrep crashes, you still want the builtin scanner results. The install flow decides the policy (block on high findings, warn on medium, etc.) -- the chain just aggregates.

**Ordering:** Builtin scanner always runs first (fast, no external deps). External scanners run in the order specified.

### 5. CLI Flag

New flag on `syllago install`:

```
--hook-scanner path/to/scanner    (repeatable)
```

Each `--hook-scanner` value adds an `ExternalScanner` to the chain. The builtin scanner is always included (it runs first). Multiple flags chain multiple scanners:

```bash
syllago install my-hook --to claude-code \
  --hook-scanner ./scanners/semgrep-wrapper.sh \
  --hook-scanner ./scanners/shellcheck-wrapper.sh
```

The flag is a no-op for non-hook content types (silently ignored).

**Config file support (future):** The flag is the MVP. A `scanners` key in `.syllago/config.yaml` for persistent scanner configuration is a natural follow-up but not part of this bead.

### 6. Integration with Install Flow

The scanner chain runs in `installHook()` in `cli/internal/installer/hooks.go`, after `parseHookFile()` succeeds but **before** writing to settings.json:

```
parseHookFile()
  -> resolveHookScripts()     (copies scripts to stable location)
  -> RunScanChain(hookDir)    (NEW: scan the hook directory)
  -> check for duplicates
  -> snapshot + write to settings.json
```

**Why after resolveHookScripts:** The scripts need to be in their final location for scanning to be meaningful. Scanning the registry cache copy and then installing a different copy would be a TOCTOU gap.

**Behavior on findings:**

| Highest severity | CLI behavior | TUI behavior |
|-----------------|-------------|-------------|
| No findings | Proceed silently | Proceed silently |
| Low/info only | Print findings, proceed | Show findings in detail panel, proceed |
| Medium | Print findings, proceed with warning | Show findings, proceed with warning badge |
| High | Print findings, abort with exit code 1 | Show findings, block install, require confirmation |

High-severity findings block by default. A `--force` flag (already exists on install) overrides this.

**New exported function** for the install flow to call:

```go
// RunScanChain runs all configured scanners against a hook directory
// and returns the aggregated result.
func RunScanChain(hookDir string, extraScanners []string) (ScanResult, error)
```

This constructs the chain (builtin + any external scanner paths), runs it, and returns the merged result. The installer calls this and decides how to handle findings based on severity.

### 7. External Tool Integration Examples

These are thin wrapper scripts that adapt existing tools to the scanner protocol.

#### Semgrep Wrapper

```bash
#!/usr/bin/env bash
# semgrep-scanner.sh -- wraps Semgrep for syllago hook scanning
HOOK_DIR="$1"
# Run semgrep on shell scripts in the hook directory
output=$(semgrep --config=auto --json "$HOOK_DIR" 2>/dev/null)
if [ $? -ge 2 ]; then
  echo '{"findings":[],"errors":["semgrep execution failed"]}'
  exit 0
fi
# Transform semgrep JSON output to scanner protocol
echo "$output" | jq '{
  findings: [.results[] | {
    severity: (if .extra.severity == "ERROR" then "high"
               elif .extra.severity == "WARNING" then "medium"
               else "low" end),
    file: (.path | ltrimstr("'"$HOOK_DIR"'/") ),
    line: .start.line,
    description: .extra.message
  }],
  errors: []
}'
```

#### ShellCheck Wrapper

```bash
#!/usr/bin/env bash
# shellcheck-scanner.sh -- wraps ShellCheck for syllago hook scanning
HOOK_DIR="$1"
findings="[]"
for script in "$HOOK_DIR"/*.sh "$HOOK_DIR"/*.bash; do
  [ -f "$script" ] || continue
  result=$(shellcheck --format=json "$script" 2>/dev/null || true)
  new=$(echo "$result" | jq --arg dir "$HOOK_DIR" '[.[] | {
    severity: (if .level == "error" then "high"
               elif .level == "warning" then "medium"
               else "low" end),
    file: (.file | ltrimstr($dir + "/")),
    line: .line,
    description: .message
  }]')
  findings=$(echo "$findings" "$new" | jq -s 'add')
done
echo "{\"findings\":$findings,\"errors\":[]}"
```

These wrappers are documentation/example only -- they ship in `docs/examples/scanners/`, not as part of the binary. Enterprises write their own or adapt these.

### 8. Test Cases

All tests in `cli/internal/converter/scanner_test.go`.

#### BuiltinScanner Tests

| Test | What it verifies |
|------|-----------------|
| `TestBuiltinScanner_ManifestFindings` | Scans a hook.json with `curl` command, gets high-severity finding with `File: "hook.json"` |
| `TestBuiltinScanner_ScriptFindings` | Hook dir with `check.sh` containing `wget`, scanner finds it with correct file name and line number |
| `TestBuiltinScanner_CleanHook` | Hook with no dangerous patterns returns empty findings |
| `TestBuiltinScanner_MultipleScripts` | Dir with `.sh` and `.py` files, both scanned, findings from each tagged with correct file |
| `TestBuiltinScanner_BinaryFilesSkipped` | Non-text files in hook dir are not scanned (no panic on binary content) |

#### ExternalScanner Tests

| Test | What it verifies |
|------|-----------------|
| `TestExternalScanner_ValidOutput` | Subprocess writes valid JSON findings, parsed correctly |
| `TestExternalScanner_EmptyFindings` | Scanner exits 0 with empty findings array -- no error |
| `TestExternalScanner_ExitCode1` | Scanner exits 1 with findings -- treated as success with findings |
| `TestExternalScanner_ExitCode2` | Scanner exits 2 -- error recorded in `ScanResult.Errors`, no findings |
| `TestExternalScanner_Timeout` | Scanner sleeps forever -- killed after timeout, error recorded |
| `TestExternalScanner_InvalidJSON` | Scanner writes garbage -- error recorded, no panic |
| `TestExternalScanner_NotFound` | Scanner path doesn't exist -- error recorded, not a hard failure |

External scanner tests use small bash scripts created in `t.TempDir()` as test fixtures (same pattern as the existing hook test fixtures). No real Semgrep/ShellCheck dependency.

#### ChainedScanner Tests

| Test | What it verifies |
|------|-----------------|
| `TestChainScanners_MergesFindings` | Two scanners with different findings, both appear in result with correct `Scanner` field |
| `TestChainScanners_MergesErrors` | One scanner errors, other succeeds -- findings from good scanner preserved, error from bad one recorded |
| `TestChainScanners_Empty` | No scanners in chain -- returns empty result, no error |
| `TestChainScanners_BuiltinAlwaysFirst` | Verify ordering of findings matches scanner order |

#### Integration Tests

| Test | What it verifies |
|------|-----------------|
| `TestRunScanChain_BuiltinOnly` | No external scanners configured -- builtin runs, returns findings |
| `TestRunScanChain_WithExternal` | Builtin + external scanner path -- both run, results merged |
| `TestRunScanChain_BadExternalPath` | External scanner doesn't exist -- builtin still runs, error recorded for missing scanner |

#### Install Flow Tests (in `cli/internal/installer/hooks_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestInstallHook_HighSeverityBlocks` | Hook with high-severity finding returns error (install blocked) |
| `TestInstallHook_MediumSeverityWarns` | Hook with medium findings installs but scan result is returned for display |
| `TestInstallHook_ForceBypassesScan` | `--force` flag allows install despite high-severity findings |

## File Summary

| File | Action |
|------|--------|
| `cli/internal/converter/scanner.go` | **New** -- `HookScanner` interface, `ScanFinding`, `ScanResult`, `BuiltinScanner`, `ExternalScanner`, `ChainScanners`, `RunScanChain` |
| `cli/internal/converter/scanner_test.go` | **New** -- all scanner tests |
| `cli/internal/converter/hook_security.go` | **No change** -- existing regex scanner stays as-is, wrapped by `BuiltinScanner` |
| `cli/internal/installer/hooks.go` | **Modify** -- call `RunScanChain()` in `installHook()` before writing |
| `cli/internal/installer/hooks_test.go` | **Modify** -- add install-flow scan integration tests |
| `cli/cmd/syllago/install.go` | **Modify** -- add `--hook-scanner` repeatable flag |
| `docs/examples/scanners/` | **New** -- example wrapper scripts (semgrep, shellcheck) |

## Not In Scope

- **Config file for persistent scanner configuration** -- CLI flag is the MVP; config file is a follow-up.
- **TUI scanner result display** -- the TUI already shows security warnings in the detail panel. Adapting it to show `ScanFinding` instead of `SecurityWarning` is a separate bead.
- **Scanner result caching** -- scan on every install. Hooks are small; scanning is fast.
- **Sandboxing the external scanner subprocess** -- the scanner runs with user privileges, same as any CLI tool. Sandboxing scanners is out of scope.
