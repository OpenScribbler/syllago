# Implementation Plan: `syllago convert --batch`

**Bead:** syllago-qgfb
**Status:** Plan (no code)

## Summary

Add a `--batch` flag to `syllago convert` that takes a directory of provider-native hook files, canonicalizes each one individually, and writes the results to an output directory as canonical hook directories. Each hook gets per-hook warnings, and the command prints a summary at the end.

This is specifically for hook migration: a user points it at their provider's hooks directory and gets a set of canonical hook files ready for import into syllago's library.

## CLI Interface

```
syllago convert --batch ./hooks/ --from claude-code [--output ./canonical-hooks/] [--dry-run] [--json]
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--batch <dir>` | Yes (mutual exclusive with positional `<name>`) | Directory containing hook files to convert |
| `--from <provider>` | Yes (when `--batch`) | Source provider slug. Required because batch operates on raw files, not library items with metadata |
| `--output <dir>` | No | Output directory. Default: `./canonical-hooks/` |
| `--dry-run` | No | Show what would be converted without writing files |
| `--to` | No | Not used with `--batch` (batch canonicalizes only, it does not render to a target). Error if combined with `--batch` |

**Why `--from` is required:** The existing single-item `convert` infers the source provider from catalog metadata (`item.Meta.SourceProvider` or `item.Provider`). Batch mode operates on raw files outside the library, so there's no metadata to read. The user must declare what format they're converting from.

**Why no `--to`:** Batch mode is a canonicalization step, not a render step. The use case is "migrate my hooks into syllago's format." Rendering to a different provider is a separate operation on the already-canonical output. This keeps the batch command focused and avoids combinatorial complexity (batch + render + per-hook warnings from both stages).

## Input Discovery

The batch command walks the input directory looking for hook files:

1. **JSON files** (`*.json`) -- the primary hook config format for all providers
2. **Nested hooks in settings files** -- Claude Code embeds hooks in `settings.json` under the `"hooks"` key. Detect via `DetectHookFormat()` returning "nested".
3. **Directories containing `hook.json`** -- syllago's flat format uses `hook.json` inside a named directory.

**Walk strategy:** Non-recursive by default. Each file/directory at the top level is one conversion unit. Rationale: hook directories are typically flat (one level of nesting at most), and recursive walking risks picking up unrelated JSON files.

**Skipped entries:** Non-JSON files, directories without `hook.json`, files that fail JSON parse. Each skip emits a line to stderr (or a warning in JSON mode).

## Per-Hook Conversion

For each discovered hook file:

1. Read the file content
2. Call `DetectHookFormat()` to determine flat vs nested
3. Call `HooksConverter.Canonicalize(content, sourceProvider)`
4. On success: collect the `Result` (canonical content + warnings)
5. On error: record the error, continue to next file

**Nested format splitting:** A single nested hooks file (e.g., Claude Code's `settings.json` with multiple events) may produce multiple canonical hooks via `ParseNested()`. Each event+matcher group becomes its own output directory. This reuses the existing `ParseNested()` function, then canonicalizes each `HookData` individually using `canonicalizeFlatHook()` (after marshaling to flat format).

**Inline vs script-file hooks:**

| Hook shape | Detection | Handling |
|-----------|-----------|----------|
| Inline command (`"command": "echo hello"`) | `Command` field is a bare command string (no `./` prefix, no path separators) | Preserved as-is in canonical `handler.command` |
| Script reference (`"command": "./check.sh"`) | `Command` starts with `./` or contains path separators | Copy the referenced script file to the output directory alongside `hook.json`. Emit a warning if the script file doesn't exist at the expected path relative to the input directory |
| HTTP hook (`"type": "http"`) | `Type` field | Preserved in canonical format (Claude Code only) |
| LLM hook (`"type": "prompt"/"agent"`) | `Type` field | Preserved in canonical format with capability annotation |

## Output Directory Structure

```
canonical-hooks/
  before-tool-execute--shell/
    hook.json          # canonical flat format
    check.sh           # copied script (if referenced)
  session-start/
    hook.json
  before-tool-execute--mcp-github/
    hook.json
```

**Directory naming:** `<event>` or `<event>--<matcher>` when a matcher is present. Sanitized to lowercase alphanumeric + hyphens (reuse existing `sanitizeForFilename()`). Deduplication via numeric suffix (`-2`, `-3`) if names collide.

**Why flat format output (not nested):** Each output directory is one logical hook, matching syllago's library convention for hook content items. This makes the output directly importable via `syllago add ./canonical-hooks/before-tool-execute--shell/`.

**`hook.json` content:** The canonical flat format (`HookData` struct) with `sourceProvider` field set. This is the same format `canonicalizeFlatHook()` already produces.

## Dry-Run Mode

When `--dry-run` is set:

1. Run all discovery and canonicalization logic
2. Print what would be written (directory names, file counts) without writing
3. Still show per-hook warnings and the summary
4. Exit code 0 (even if some hooks have warnings)

**Text output:**
```
[dry-run] Would create: canonical-hooks/before-tool-execute--shell/hook.json
  Warning: timeout converted from 60000ms to 60s
[dry-run] Would create: canonical-hooks/session-start/hook.json
[dry-run] Would skip: malformed.json (parse error: unexpected EOF)

Summary: 2 would be converted, 0 warnings, 1 error
```

**JSON output (`--json --dry-run`):**
```json
{
  "dryRun": true,
  "hooks": [
    {"name": "before-tool-execute--shell", "status": "would_convert", "warnings": ["timeout converted from 60000ms to 60s"]},
    {"name": "session-start", "status": "would_convert", "warnings": []},
    {"name": "malformed.json", "status": "error", "error": "parse error: unexpected EOF"}
  ],
  "summary": {"converted": 2, "warnings": 0, "errors": 1}
}
```

## Summary Output

After all hooks are processed, print a summary line:

**Text mode:** `Converted 5 hooks (2 warnings, 1 error) to canonical-hooks/`

**JSON mode:** Include `summary` object in the top-level response (shown above).

**Warning count** = total warnings across all hooks (not number of hooks with warnings).

**Exit code:**
- 0: all hooks converted (warnings are OK)
- 1: at least one hook failed to convert (partial success)

## Integration with Existing Code

### Reused functions

| Function | Package | How used |
|----------|---------|----------|
| `DetectHookFormat()` | converter | Determine flat vs nested per input file |
| `ParseNested()` | converter | Split nested config into individual hook groups |
| `ParseFlat()` | converter | Parse flat-format hooks |
| `HooksConverter.Canonicalize()` | converter | Core conversion logic per hook |
| `sanitizeForFilename()` | converter | Directory name generation |

### New code locations

| File | What |
|------|------|
| `cli/cmd/syllago/convert_cmd.go` | Add `--batch`, `--from`, `--dry-run` flags. Add `runConvertBatch()` function. Cobra validation: `--batch` is mutually exclusive with positional arg; `--from` is required when `--batch` is set; `--to` is forbidden with `--batch`. |
| `cli/internal/converter/batch.go` | `BatchCanonicalize(inputDir, sourceProvider string, dryRun bool) (*BatchResult, error)` -- orchestration function. Handles discovery, per-hook conversion, script file copying, output directory creation. |
| `cli/internal/converter/batch_test.go` | Tests for the batch orchestration logic. |

### New types

```go
// BatchHookResult holds the outcome of converting one hook file.
type BatchHookResult struct {
    Name     string   // output directory name
    Source   string   // input file path
    Warnings []string
    Error    error    // nil on success
    Content  []byte   // canonical hook.json content (nil on error or dry-run)
    Scripts  map[string][]byte // script files to copy alongside hook.json
}

// BatchResult holds the aggregate result of a batch conversion.
type BatchResult struct {
    Hooks     []BatchHookResult
    Converted int
    Warnings  int
    Errors    int
}
```

### Validation in convert_cmd.go

The `RunE` function needs a branch at the top:

```
if --batch is set:
    validate --from is set (error if missing)
    validate --to is NOT set (error: "cannot combine --batch with --to")
    validate positional args are empty (error: "cannot combine --batch with item name")
    call runConvertBatch()
else:
    existing single-item logic
```

## Test Cases

### Unit tests (`batch_test.go`)

| Test | Description |
|------|-------------|
| `TestBatchDiscovery_JSONFiles` | Directory with 3 `.json` files -> discovers all 3 |
| `TestBatchDiscovery_SkipsNonJSON` | Directory with `.md`, `.sh`, `.json` -> only processes `.json` |
| `TestBatchDiscovery_HookSubdirs` | Directory with subdirs containing `hook.json` -> discovers each |
| `TestBatchDiscovery_EmptyDir` | Empty directory -> returns empty result, no error |
| `TestBatchCanonicalize_FlatHook` | Single flat-format hook -> correct canonical output |
| `TestBatchCanonicalize_NestedSplits` | One nested file with 3 events -> 3 output directories |
| `TestBatchCanonicalize_InlineCommand` | Hook with bare command string -> preserved as-is |
| `TestBatchCanonicalize_ScriptReference` | Hook referencing `./check.sh` -> script copied to output, warning if missing |
| `TestBatchCanonicalize_MixedSuccess` | 3 files: 2 valid, 1 malformed -> 2 converted, 1 error, exit continues |
| `TestBatchCanonicalize_DirectoryNaming` | Event+matcher -> sanitized dirname (e.g., `before-tool-execute--shell`) |
| `TestBatchCanonicalize_DuplicateNames` | Two hooks with same event+matcher -> second gets `-2` suffix |
| `TestBatchCanonicalize_PerHookWarnings` | Hook with LLM type -> warning attached to that specific hook result |
| `TestBatchCanonicalize_TimeoutConversion` | Claude Code ms timeout -> canonical seconds in output |
| `TestBatchCanonicalize_SourceProviderSet` | Output `hook.json` has `sourceProvider` field set to `--from` value |

### Dry-run tests (`batch_test.go`)

| Test | Description |
|------|-------------|
| `TestBatchDryRun_NoFilesWritten` | Dry-run with valid hooks -> output dir not created |
| `TestBatchDryRun_ReportsWhatWouldHappen` | Result contains correct names, warnings, no content |

### Command-level tests (`convert_cmd_test.go`)

| Test | Description |
|------|-------------|
| `TestConvertBatch_RequiresFrom` | `--batch ./dir` without `--from` -> error |
| `TestConvertBatch_RejectsTo` | `--batch ./dir --from x --to y` -> error |
| `TestConvertBatch_RejectsPositionalArg` | `--batch ./dir <name>` -> error |
| `TestConvertBatch_NonexistentDir` | `--batch ./nope` -> clear error message |
| `TestConvertBatch_JSONOutput` | `--batch --json` -> valid JSON with summary |
| `TestConvertBatch_SummaryOutput` | Text mode -> "Converted N hooks (M warnings, K errors)" |

### Provider-specific tests (`batch_test.go`)

Valid `--from` values (all providers with hook systems): `claude-code`, `gemini-cli`, `cursor`, `windsurf`, `vs-code-copilot`, `copilot-cli`, `kiro`. Invalid values should produce a clear error listing valid options.

| Test | Description |
|------|-------------|
| `TestBatchCanonicalize_ClaudeCodeSettings` | Full Claude Code settings.json with hooks section -> correctly extracts and canonicalizes. PascalCase events (PreToolUse) -> canonical (before_tool_execute). CC tool names (Bash) -> canonical (shell). Timeout stays in seconds. |
| `TestBatchCanonicalize_CopilotFormat` | Copilot CLI hooks.json with `bash`/`powershell` fields -> canonical `command` + `platform.windows`. `timeoutSec` -> canonical `timeout`. camelCase events (preToolUse) -> canonical. `version: 1` field preserved in `provider_data`. |
| `TestBatchCanonicalize_GeminiCLI` | Gemini CLI format with BeforeTool/AfterTool events -> canonical event names. Timeout ms -> canonical seconds. `sequential` field preserved in `provider_data`. MCP tool format `mcp_server_tool` -> structured `{"mcp": {...}}`. |
| `TestBatchCanonicalize_Cursor` | Cursor's split-event model: `beforeShellExecution` -> `before_tool_execute` + `matcher: "shell"`, `beforeMCPExecution` -> `before_tool_execute` + `matcher: {"mcp": {...}}`, `beforeReadFile` -> `before_tool_execute` + `matcher: "file_read"`. `afterFileEdit` -> `after_tool_execute` + `matcher: "file_edit"`. `stop` -> `agent_stop`. Verifies split-to-unified event merge. |
| `TestBatchCanonicalize_Windsurf` | Windsurf snake_case events: `pre_run_command` -> `before_tool_execute` + `matcher: "shell"`, `pre_read_code` -> `before_tool_execute` + `matcher: "file_read"`, `pre_write_code` -> `before_tool_execute` + `matcher: "file_write"`, `pre_mcp_tool_use` -> `before_tool_execute` + `matcher: {"mcp": {...}}`. `working_directory` -> canonical `cwd`. `show_output` preserved in `provider_data`. Verifies per-category-to-unified event merge. |
| `TestBatchCanonicalize_Kiro` | Kiro CLI format: camelCase events (preToolUse, postToolUse) -> canonical. Internal tool names (execute_bash -> shell, fs_read -> file_read, fs_write -> file_write). `timeout_ms` field -> canonical seconds. |
| `TestBatchCanonicalize_VSCodeCopilot` | VS Code Copilot format with same PascalCase events as Claude Code -> canonical. `windows`/`linux`/`osx` command overrides -> canonical `platform` block. `env` field -> canonical `env`. `cwd` field preserved. Verifies the "nearly identical to Claude Code" adapter assumption. |
| `TestBatchCanonicalize_InvalidProvider` | `--from invalid-provider` -> clear error listing valid provider slugs |

## Open Questions

1. **Should `--batch` support `--to` for a combined canonicalize+render pipeline?** Current plan says no -- keep it focused on canonicalization. Rendering can be done per-hook afterward with `syllago convert <name> --to <provider>`. Revisit if users request it.

2. **Recursive walk?** Current plan says non-recursive. If users have deeply nested hook directories, we could add `--recursive` later. Start simple.

3. **Overwrite behavior when output dir exists?** Options: error, merge, or `--force` flag. Recommend: error if the output directory already exists (with `--force` to overwrite). Prevents accidental data loss.
