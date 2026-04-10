# Implementation Plan: Script Content Scanning for Hooks

**Bead:** syllago-bym9
**Status:** Plan (no code changes)

## Motivation

The current `ScanHookSecurity()` in `cli/internal/converter/hook_security.go` only scans the `command` string field of hook entries. Per the security considerations doc (Section 2.1: "Scripts Are the Attack Surface, Not Manifests"), the real danger is in script files that commands reference (e.g., `"command": "./check.sh"`). A command like `./check.sh` passes all current patterns cleanly, but the script itself could contain `curl`, `rm -rf`, or equivalent operations in any language.

This plan extends the scanner to read and analyze referenced script files and `provider_data` string values.

---

## 1. File Extensions to Scan

Scripts are identified by file extension. These are the languages commonly used in hook scripts:

| Extension | Language | Notes |
|-----------|----------|-------|
| `.sh` | Shell (bash/sh) | Most common hook script language |
| `.ps1` | PowerShell | Windows hooks, Copilot `powershell` field |
| `.py` | Python | General-purpose scripting |
| `.js` | JavaScript (Node) | Common in JS-heavy projects |
| `.ts` | TypeScript | Often run via `tsx`/`ts-node` |
| `.rb` | Ruby | Less common but used in some toolchains |

Defined as a package-level `var scriptExtensions = map[string]bool{...}` for easy extension later.

---

## 2. Detecting Executable Files

A file is considered a scannable script if **either** condition is true:

1. **Extension match:** File extension is in the `scriptExtensions` set (case-insensitive comparison via `strings.ToLower`).
2. **Executable bit:** On Unix systems, `os.Stat` returns mode bits where any execute bit is set (`mode&0111 != 0`). This catches extensionless scripts with shebangs.

Both checks are needed because:
- Extension-only misses `#!/usr/bin/env python3` scripts with no extension.
- Executable-bit-only misses `.py` files that are invoked via `python script.py` (not marked executable).

**Design decision:** We do NOT read shebangs to determine language. The extension determines which language-specific patterns to apply. Extensionless executables get scanned with the shell patterns only (the most conservative default, since shebangs typically indicate shell-like execution).

---

## 3. Architecture: New Function, Not Extension

**Decision:** Add a new `ScanScriptFiles()` function rather than extending `ScanHookSecurity()`.

**Why:**
- `ScanHookSecurity()` operates on JSON bytes (the hook manifest). Script scanning operates on a filesystem directory. Different inputs, different concerns.
- Callers may want manifest-only scanning (e.g., during import preview when scripts haven't been written to disk yet) vs. full scanning (after install, during audit).
- Keeps each function focused and independently testable.

**Interface:**

```go
// ScriptFinding represents a security concern found in a script file.
type ScriptFinding struct {
    File        string // relative path within hook directory
    Line        int    // 1-indexed line number
    Severity    string // "high", "medium", "low"
    Description string // human-readable explanation
    Snippet     string // the matched line content (trimmed)
}

// ScanScriptFiles scans all script files in a hook directory for
// dangerous patterns. Returns per-file findings with line numbers.
func ScanScriptFiles(hookDir string) ([]ScriptFinding, error)
```

**Integration point:** A higher-level `ScanHookFull()` function combines both:

```go
func ScanHookFull(hookDir string, manifestContent []byte) ([]SecurityWarning, []ScriptFinding, error)
```

This is what the TUI and CLI commands call. It runs `ScanHookSecurity()` on the manifest and `ScanScriptFiles()` on the directory.

---

## 4. Scanning `provider_data` String Values

The `provider_data` field is `map[string]json.RawMessage` -- opaque JSON keyed by provider slug. Malicious content could hide commands in string values nested at any depth.

**Approach:** Recursive extraction of all string values from `provider_data`, then scan each string against the universal danger patterns (not language-specific ones, since we don't know the execution context).

```
extractStrings(value json.RawMessage) []string
```

Walks the JSON tree:
- **String:** return it
- **Object:** recurse into each value
- **Array:** recurse into each element
- **Number/bool/null:** skip

Extracted strings are scanned with the existing `dangerPatterns` list. Findings use a synthetic "file" path like `provider_data.windsurf.working_directory` to indicate the JSON path.

**Why recursive:** `provider_data` schemas are opaque and provider-defined. A flat scan of top-level values would miss nested structures like `{"windsurf": {"scripts": {"pre": "curl ..."}}}`.

**Performance note:** `provider_data` is typically small (a few fields per provider). Recursive extraction adds negligible cost.

---

## 5. Language-Specific Danger Patterns

The existing `dangerPatterns` are shell-oriented. Each non-shell language has its own idioms for the same dangerous operations. New patterns are organized by language.

### 5.1 Pattern Structure

```go
type langPattern struct {
    pattern     *regexp.Regexp
    severity    string
    description string
    languages   []string // which extensions this applies to; empty = all
}
```

A single `langPatterns` slice. During scanning, patterns are filtered by the file's detected language. Shell patterns (existing `dangerPatterns`) apply to `.sh` and extensionless files.

### 5.2 New Patterns by Language

**Python (.py):**

| Pattern | Severity | Description |
|---------|----------|-------------|
| `\burllib\.request\b` | high | network request (urllib) |
| `\brequests\.(get\|post\|put\|delete)\b` | high | network request (requests library) |
| `\bhttpx\.\b` | high | network request (httpx) |
| `\bsocket\.connect\b` | high | raw socket connection |
| `\bsubprocess\.(run\|call\|Popen)\b` | medium | subprocess execution |
| `\bos\.system\b` | medium | shell command execution |
| `\bshutil\.rmtree\b` | high | recursive directory deletion |
| `\bos\.remove\b` | medium | file deletion |

**JavaScript/TypeScript (.js, .ts):**

| Pattern | Severity | Description |
|---------|----------|-------------|
| `\bfetch\s*\(` | high | network request (fetch) |
| `\bhttp\.request\b` | high | network request (Node http) |
| `\bhttps\.request\b` | high | network request (Node https) |
| `\baxios\b` | high | network request (axios) |
| `\bchild_process\b` | medium | subprocess execution |
| `\bexecSync\b\|\bexecFile\b\|\bspawn\b` | medium | subprocess execution |
| `\bfs\.rmSync\b\|\bfs\.rmdirSync\b` | high | recursive file deletion |
| `\bfs\.unlinkSync\b` | medium | file deletion |

**PowerShell (.ps1):**

| Pattern | Severity | Description |
|---------|----------|-------------|
| `\bInvoke-WebRequest\b` | high | network request (Invoke-WebRequest) |
| `\bInvoke-RestMethod\b` | high | network request (Invoke-RestMethod) |
| `\bNew-Object\s+Net\.WebClient\b` | high | network request (WebClient) |
| `\bStart-Process\b` | medium | process execution |
| `\bRemove-Item\b.*-Recurse\b` | high | recursive file deletion |
| `\bSet-ExecutionPolicy\b` | high | execution policy change |

**Ruby (.rb):**

| Pattern | Severity | Description |
|---------|----------|-------------|
| `\bNet::HTTP\b` | high | network request (Net::HTTP) |
| `\bopen-uri\b\|\bURI\.open\b` | high | network request (open-uri) |
| `\bsystem\s*\(` | medium | shell command execution |
| `` \`[^`]+\` `` | medium | backtick shell execution |
| `\bFileUtils\.rm_rf\b` | high | recursive file deletion |

### 5.3 Universal Patterns (All Languages)

These apply regardless of file extension:

| Pattern | Severity | Description |
|---------|----------|-------------|
| `\bcurl\b` | high | network request (curl) |
| `\bwget\b` | high | network request (wget) |
| `\bssh\b` | high | remote access (ssh) |
| `\bnc\b\|\bnetcat\b\|\bncat\b` | high | network tool |
| `\brm\s+(-[a-zA-Z]*r\|-[a-zA-Z]*f)` | high | recursive/forced deletion |
| `(/etc/\|/usr/)` | low | system path reference |

The universal set is the existing `dangerPatterns` (minus shell-specific ones like `env | grep`). These catch cases where a Python script shells out to `curl` or a JS file uses `child_process.execSync('wget ...')`.

---

## 6. Per-File Findings with Line Numbers

Each script file is read line-by-line. For each line:

1. Skip comment-only lines (language-aware: `#` for shell/python/ruby, `//` for JS/TS, `#` for PowerShell). This reduces false positives from documentation comments.
2. Match against universal patterns + language-specific patterns.
3. Record matches with the 1-indexed line number and trimmed line content.

**Snippet truncation:** Lines longer than 120 characters are truncated with `...` suffix to keep findings readable.

**Deduplication:** If the same pattern matches the same line multiple times (e.g., `curl` appears twice on one line), emit only one finding per pattern per line.

---

## 7. Performance Considerations

### 7.1 File Size Limits

Scripts in hook directories are typically small (under 1KB). But to handle pathological cases:

- **Max file size:** 1MB. Files larger than this are skipped with a warning finding ("file too large to scan").
- **Max line count:** 10,000 lines. After this limit, scanning stops for the file and emits a warning.

**Why these limits:** A 1MB script with 10K lines takes roughly 50ms to scan with regexp matching. This is well within acceptable latency for an import or audit operation. The limits exist to prevent accidental inclusion of large binary or generated files, not because normal scripts would hit them.

### 7.2 Directory Walking

`ScanScriptFiles()` walks the hook directory with `filepath.WalkDir`. It skips:
- Hidden directories (`.git`, `.svn`, etc.)
- `node_modules/` (if a JS hook somehow includes dependencies)
- The `hook.json` manifest itself (scanned separately by `ScanHookSecurity()`)

### 7.3 Regex Compilation

All patterns are compiled once at package init time (same approach as existing `dangerPatterns`). No per-call compilation overhead.

### 7.4 Concurrency

No concurrency needed. Hook directories typically contain 1-5 scripts. Sequential scanning is simpler and fast enough. If profiling ever shows this is a bottleneck (it won't), individual file scans could be parallelized trivially since they're independent.

---

## 8. Test Cases

### 8.1 Unit Tests (in `hook_security_test.go`)

**Script scanning:**

| Test | Description |
|------|-------------|
| `TestScanScriptFiles_ShellCurl` | `.sh` file with `curl` on line 5 -- expect HIGH finding at line 5 |
| `TestScanScriptFiles_PythonRequests` | `.py` file with `requests.post()` -- expect HIGH finding |
| `TestScanScriptFiles_NodeFetch` | `.js` file with `fetch(url)` -- expect HIGH finding |
| `TestScanScriptFiles_PowerShellWebRequest` | `.ps1` file with `Invoke-WebRequest` -- expect HIGH finding |
| `TestScanScriptFiles_RubyNetHTTP` | `.rb` file with `Net::HTTP.get` -- expect HIGH finding |
| `TestScanScriptFiles_TypeScriptAxios` | `.ts` file with `axios.post()` -- expect HIGH finding |
| `TestScanScriptFiles_SafeScript` | `.sh` file with `echo` and `jq` only -- expect 0 findings |
| `TestScanScriptFiles_CommentSkipped` | `# curl http://...` in a comment -- expect 0 findings |
| `TestScanScriptFiles_MultipleFindings` | Script with `curl` on line 3 and `rm -rf` on line 7 -- expect 2 findings with correct line numbers |
| `TestScanScriptFiles_ExecutableBitNoExtension` | Extensionless file with `+x` and `curl` -- expect HIGH finding |
| `TestScanScriptFiles_LargeFileSkipped` | File over 1MB -- expect warning finding, no patterns scanned |
| `TestScanScriptFiles_HiddenDirSkipped` | Script in `.git/hooks/` -- expect not scanned |
| `TestScanScriptFiles_EmptyDir` | Empty hook directory -- expect 0 findings, nil error |
| `TestScanScriptFiles_SnippetTruncation` | Very long line (200+ chars) -- finding snippet is truncated to 120 |

**`provider_data` scanning:**

| Test | Description |
|------|-------------|
| `TestScanProviderData_CurlInString` | `{"windsurf": {"cmd": "curl evil.com"}}` -- expect HIGH finding |
| `TestScanProviderData_NestedString` | Deeply nested string value with `wget` -- expect HIGH finding |
| `TestScanProviderData_SafeValues` | `{"windsurf": {"show_output": true}}` -- expect 0 findings |
| `TestScanProviderData_NonStringSkipped` | Numbers and booleans -- expect 0 findings |

**Integration (combined scanning):**

| Test | Description |
|------|-------------|
| `TestScanHookFull_ManifestAndScripts` | Hook dir with manifest containing `echo ./check.sh` + `check.sh` containing `curl` -- both manifest and script findings returned |
| `TestScanHookFull_ManifestOnly` | No script files in directory -- only manifest warnings returned |

### 8.2 Test Fixtures

All tests use `t.TempDir()` with files created programmatically (no checked-in fixture files). Scripts are written with `os.WriteFile` at `0644` or `0755` as needed.

---

## Files Changed

| File | Change |
|------|--------|
| `cli/internal/converter/hook_security.go` | Add `ScriptFinding` type, `ScanScriptFiles()`, `ScanHookFull()`, `extractStrings()`, language pattern tables |
| `cli/internal/converter/hook_security_test.go` | Add all test cases from Section 8 |

No new files needed. The existing `hook_security.go` is the natural home for all security scanning logic.
