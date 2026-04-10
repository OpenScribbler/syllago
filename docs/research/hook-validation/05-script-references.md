# Hook Script References: Provider Comparison

How each AI coding tool provider handles script file references in hook configurations — path resolution, script formats, shell selection, and platform-specific handling.

Research date: 2026-03-22

---

## 1. Claude Code

**Source:** https://code.claude.com/docs/en/hooks

### Command Format

The `command` field accepts inline shell expressions, script paths, or executables in PATH:

```json
{
  "type": "command",
  "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/check-style.sh",
  "timeout": 30
}
```

Inline expressions are fully supported:
```json
{"command": "bash -c 'jq -r \".tool_input.command\" | grep -q \"rm -rf\" && echo denied || exit 0'"}
{"command": "echo 'Hook executed at $(date)' >> ~/hook.log"}
```

### Path Resolution

- **No explicit `cwd` or `working_directory` field** in the hook configuration.
- Hooks run in the current directory with Claude Code's environment.
- The hook *input* JSON includes a `cwd` field (read-only, not configurable).
- Relative paths are resolved from the current working directory.

### Environment Variables for Path Resolution

| Variable | Description |
|----------|-------------|
| `$CLAUDE_PROJECT_DIR` | Project root directory |
| `${CLAUDE_PLUGIN_ROOT}` | Plugin installation directory (changes on updates) |
| `${CLAUDE_PLUGIN_DATA}` | Plugin persistent data directory |
| `$CLAUDE_ENV_FILE` | SessionStart only: file for persisting env vars |
| `$CLAUDE_CODE_REMOTE` | Set to `"true"` in web/remote environments |

Home directory expansion (`~`) is supported in command strings.

### Shell Execution

- **Shell not explicitly documented.** All examples use Bash conventions.
- Shebang lines (`#!/bin/bash`) used in script examples.
- For portability, explicit invocation recommended: `bash "$CLAUDE_PROJECT_DIR"/.claude/hooks/my-script.sh`

### Script Permissions

Not explicitly documented. Examples imply scripts should be executable or invoked via an explicit shell prefix (`bash script.sh`).

### Platform-Specific Handling

No platform-specific command fields or overrides. All examples use Unix conventions. Windows behavior is undocumented.

---

## 2. Gemini CLI

**Source:** https://geminicli.com/docs/hooks/, /reference, /best-practices

### Command Format

The `command` field is required when `type` is `"command"`. Supports script paths with environment variable expansion:

```json
{
  "name": "security-check",
  "type": "command",
  "command": "$GEMINI_PROJECT_DIR/.gemini/hooks/security.sh",
  "timeout": 5000
}
```

Both Bash and Node.js scripts are supported via shebangs (`#!/usr/bin/env bash`, `#!/usr/bin/env node`).

### Path Resolution

- No explicit `cwd` or `working_directory` configuration field.
- The hook input includes `cwd` as a read-only field.
- `$GEMINI_PROJECT_DIR` is the primary mechanism for project-relative paths.
- CLI verifies file existence before execution: `test -f "$GEMINI_PROJECT_DIR/.gemini/hooks/my-hook.sh"`

### Environment Variables

| Variable | Description |
|----------|-------------|
| `$GEMINI_PROJECT_DIR` | Project root path |
| `$GEMINI_SESSION_ID` | Current session ID |
| `$GEMINI_CWD` | Current working directory |
| `$CLAUDE_PROJECT_DIR` | Compatibility alias for Claude Code |

**Warning:** Hooks inherit the full CLI process environment, including sensitive variables like `GEMINI_API_KEY`.

### Shell Execution

- Scripts executed as child processes inheriting the parent shell environment.
- Bash and Node.js supported via shebangs.
- No explicit shell specification documented.

### Script Permissions

- **Unix:** Scripts require `chmod +x`. Documentation explicitly states: "Always make hook scripts executable on macOS/Linux."
- **Windows:** PowerShell scripts (`.ps1`) use execution policy: `Set-ExecutionPolicy RemoteSigned -Scope CurrentUser`

### Platform-Specific Handling

Documentation shows examples for both Unix and Windows PowerShell. No per-platform command fields -- scripts must handle platform differences internally or use separate scripts referenced by the same hook.

### Exit Code Semantics

| Code | Meaning |
|------|---------|
| 0 | Success; stdout JSON is parsed |
| 2 | System block; stderr contains reason; aborts turn |

---

## 3. Cursor

**Source:** https://cursor.com/docs/hooks, https://blog.gitbutler.com/cursor-hooks-deep-dive

### Command Format

The `command` field accepts "a shell string, an absolute path, or a relative path":

```json
{
  "hooks": {
    "afterFileEdit": [
      {
        "command": ".cursor/hooks/format.sh"
      }
    ]
  }
}
```

Scripts can be Bash, Python, TypeScript (via Bun), or any executable.

### Path Resolution

Resolution depends on hook source location:

| Source | Resolution Base |
|--------|----------------|
| Project hooks (`.cursor/hooks.json`) | **Project root** |
| User hooks (`~/.cursor/hooks.json`) | `~/.cursor/` |
| Enterprise hooks | Enterprise config directory |
| Team hooks (cloud) | Managed hooks directory |

**Important:** Project hooks use `.cursor/hooks/format.sh` (not `./hooks/format.sh`).

### Working Directory

No configurable `cwd` field in hook configuration. The hook *input* payload includes `cwd` (read-only). Scripts execute from their source directory context.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `$CURSOR_PROJECT_DIR` | Workspace root |
| `$CURSOR_VERSION` | Cursor version |
| `$CURSOR_USER_EMAIL` | Logged-in user email |
| `$CURSOR_TRANSCRIPT_PATH` | Transcript file path (if enabled) |
| `$CURSOR_CODE_REMOTE` | `"true"` in remote workspaces |
| `$CLAUDE_PROJECT_DIR` | Alias for project directory |

Session-scoped variables from `sessionStart` hooks propagate to subsequent hooks.

### Shell Execution

Not explicitly specified. Examples use Bash with `#!/bin/bash` shebangs.

### Script Permissions

Scripts must be executable: `chmod +x .cursor/hooks/format.sh`

### Platform-Specific Handling

Enterprise hooks support OS targeting for platform-specific distribution. System config paths:

| Platform | Path |
|----------|------|
| macOS | `/Library/Application Support/Cursor/hooks.json` |
| Linux/WSL | `/etc/cursor/hooks.json` |
| Windows | `C:\ProgramData\Cursor\hooks.json` |

No per-platform command fields in the hook configuration itself.

---

## 4. Windsurf

**Source:** https://docs.windsurf.com/windsurf/cascade/hooks

### Command Format

Commands can be any valid executable with arguments:

```json
{
  "hooks": {
    "post_write_code": [
      {
        "command": "python3 /Users/yourname/hooks/log_input.py",
        "show_output": true,
        "working_directory": "/absolute/path"
      }
    ]
  }
}
```

### Path Resolution

- **`working_directory` field is supported** -- unique among providers.
- Default working directory: workspace root (or repo root in multi-repo workspaces).
- Relative `working_directory` paths resolve from the default location.
- Absolute `working_directory` paths work directly.
- **Home directory expansion (`~`) is NOT supported** in `working_directory`.

Documentation recommends: "Always use absolute paths in your hook configurations to avoid ambiguity."

### Environment Variables

| Variable | Description |
|----------|-------------|
| `$ROOT_WORKSPACE_PATH` | Original workspace path (only for `post_setup_worktree` hooks) |

No other hook environment variables documented.

### Shell Execution

Not explicitly specified. No documentation on which shell interprets commands.

### Script Permissions

Documentation states: "Ensure your hook scripts have appropriate file system permissions." No specific requirements beyond this.

### Platform-Specific Handling

No per-platform command fields. System config paths differ by OS:

| Platform | Path |
|----------|------|
| macOS | `/Library/Application Support/Windsurf/hooks.json` |
| Linux/WSL | `/etc/windsurf/hooks.json` |
| Windows | `C:\ProgramData\Windsurf\hooks.json` |

---

## 5. GitHub Copilot CLI

**Source:** https://docs.github.com/en/copilot/reference/hooks-configuration

### Command Format

Uses **separate fields per platform** instead of a single `command`:

```json
{
  "type": "command",
  "bash": "./scripts/session-start.sh",
  "powershell": "./scripts/session-start.ps1",
  "cwd": "scripts",
  "timeoutSec": 30
}
```

### Path Resolution

- The `bash` and `powershell` fields specify distinct script paths per platform.
- The **`cwd` field** controls working directory. Relative `cwd` values appear in examples (`"cwd": "scripts"`) but the documentation does not explicitly state the resolution base (likely project root or config file location).
- Script paths like `./scripts/session-start.sh` suggest resolution relative to the project root or `cwd`.

### Shell Execution

- `bash` field: executed on Unix-like systems (likely via `/bin/bash` or `/usr/bin/env bash`).
- `powershell` field: executed on Windows (likely via `pwsh` or `powershell.exe`).
- Explicit platform separation eliminates cross-platform scripting issues.

### Inline Expressions

Only file-based script paths shown in documentation. Inline expressions not explicitly documented.

### Environment Variables

Uses `jq` for JSON parsing from stdin. No documented hook-specific environment variables.

### Script Permissions

Not explicitly documented.

### Platform-Specific Handling

This is the most explicit platform handling of any provider -- the `bash`/`powershell` split forces authors to provide platform-appropriate scripts. No fallback mechanism documented (if only `bash` is provided, Windows behavior is unclear).

---

## 6. VS Code Copilot

**Source:** https://code.visualstudio.com/docs/copilot/customization/hooks

### Command Format

The most feature-rich command specification of any provider, with per-OS overrides, `cwd`, and `env`:

```json
{
  "hooks": {
    "afterFileEdit": [
      {
        "type": "command",
        "command": "./scripts/default.sh",
        "windows": "powershell -File scripts\\script.ps1",
        "linux": "./scripts/format-linux.sh",
        "osx": "./scripts/format-mac.sh",
        "cwd": "relative/path",
        "env": { "AUDIT_LOG": ".github/hooks/audit.log" },
        "timeout": 30
      }
    ]
  }
}
```

### OS-Specific Command Selection

Selection hierarchy with fallback:
1. Check for OS-specific field (`windows`, `linux`, `osx`)
2. Fall back to `command` if no OS-specific field matches

Platform detection occurs on the extension host (relevant for SSH, containers, WSL).

### Path Resolution

- **`cwd` field** resolves relative to repository/workspace root.
- Script paths (`./ prefixed`) resolve relative to workspace root.

### Environment Variables

- **`env` field** passes custom variables to the hook process.
- Standard shell environment is inherited.
- `$TOOL_INPUT_FILE_PATH` is available within command strings for tool-specific hooks.

### Shell Execution

- **Linux/macOS:** Bash (`.sh` files)
- **Windows:** PowerShell (`.ps1` files, or `powershell -File` invocation)

### Script Permissions

Scripts require execute permissions: `chmod +x script.sh`

### Platform-Specific Handling

Best-in-class: three separate OS command fields (`windows`, `linux`, `osx`) with automatic fallback to `command`. This allows a single hook definition to work across all platforms with optimal commands for each.

---

## 7. Kiro

**Source:** https://kiro.dev/docs/hooks/, https://kiro.dev/docs/cli/custom-agents/configuration-reference/

### Command Format

Hooks support two action types: "Ask Kiro" (agent prompt) or "Run Command" (shell command).

```json
{
  "hooks": {
    "preToolUse": [
      {
        "matcher": "execute_bash",
        "command": "{ echo \"$(date) - Bash command:\"; cat; } >> /tmp/bash_audit_log"
      }
    ]
  }
}
```

Inline shell expressions are clearly supported (piping, redirection, command grouping).

### Path Resolution

Not documented. No `cwd` or `working_directory` field mentioned.

### Environment Variables

MCP server configurations support `"env"` fields, but hook-level environment configuration is not documented.

### Shell Execution

Not specified. Examples use Bash syntax (pipes, redirections, `$(date)`).

### Script Permissions

Not documented.

### Platform-Specific Handling

Not documented. No per-platform command fields.

### Documentation Gaps

Kiro's hook documentation is the thinnest of all providers researched. Path resolution, shell selection, permissions, and platform handling are all undocumented.

---

## 8. OpenCode

**Source:** https://opencode.ai/docs/plugins/

### Plugin Model (Not Traditional Hooks)

OpenCode uses a **TypeScript plugin model** rather than shell-command hooks. Plugins are loaded from `.opencode/plugins/` (project) or `~/.config/opencode/plugins/` (global).

```typescript
import type { Plugin } from "@opencode-ai/plugin"

export const MyPlugin: Plugin = async ({ project, client, $, directory, worktree }) => {
  return {
    "shell.env": async (input, output) => {
      output.env.MY_API_KEY = "secret"
      output.env.PROJECT_ROOT = input.cwd
    }
  }
}
```

### External Dependencies

Plugins can use npm packages via a `package.json` in `.opencode/`:

```json
{
  "dependencies": {
    "shescape": "^2.1.0"
  }
}
```

Packages installed via Bun at startup, cached in `~/.cache/opencode/node_modules/`.

### Shell Command Execution

Plugins access Bun's shell API via the `$` parameter:

```typescript
await $`osascript -e 'display notification "Hello"'`
```

### Path Context

The plugin function receives:
- `directory`: current working directory
- `worktree`: git worktree path
- `project`: current project info

### Platform-Specific Handling

No explicit cross-platform abstraction. Platform-specific commands (e.g., `osascript`) must be handled by the plugin author.

### Key Difference

OpenCode's model is fundamentally different from all other providers: it uses a programmatic TypeScript/Bun runtime rather than shell command strings. There is no `command` field, no shell path resolution, and no script permission model -- everything runs through the Bun JavaScript runtime.

---

## Summary Comparison

### Command Specification

| Provider | Command Field | Inline Shell | Script Files | Per-OS Commands |
|----------|--------------|-------------|-------------|-----------------|
| Claude Code | `command` (string) | Yes | Yes | No |
| Gemini CLI | `command` (string) | Unclear | Yes | No |
| Cursor | `command` (string) | "Shell string" supported | Yes | No |
| Windsurf | `command` (string) | Unclear | Yes | No |
| Copilot CLI | `bash` + `powershell` | Not documented | Yes | Yes (2 platforms) |
| VS Code Copilot | `command` + `windows`/`linux`/`osx` | Not documented | Yes | Yes (3 platforms + fallback) |
| Kiro | `command` (string) | Yes | Unclear | No |
| OpenCode | TypeScript plugin API | Via Bun `$` | N/A | No |

### Path Resolution

| Provider | Resolution Base | Configurable CWD | Project Dir Variable |
|----------|----------------|-------------------|---------------------|
| Claude Code | Current directory | No | `$CLAUDE_PROJECT_DIR` |
| Gemini CLI | Undocumented | No | `$GEMINI_PROJECT_DIR` |
| Cursor | Depends on hook source | No | `$CURSOR_PROJECT_DIR` |
| Windsurf | Workspace root (default) | Yes (`working_directory`) | Not documented |
| Copilot CLI | Undocumented | Yes (`cwd`) | Not documented |
| VS Code Copilot | Workspace root | Yes (`cwd`) | Not documented |
| Kiro | Undocumented | No | Not documented |
| OpenCode | Plugin receives `directory` | N/A (programmatic) | N/A |

### Script Permissions

| Provider | Executable Bit Required | Documented |
|----------|------------------------|------------|
| Claude Code | Implied | No |
| Gemini CLI | Yes | Yes, explicitly |
| Cursor | Yes | Yes, explicitly |
| Windsurf | "Appropriate permissions" | Vaguely |
| Copilot CLI | Undocumented | No |
| VS Code Copilot | Yes | Yes |
| Kiro | Undocumented | No |
| OpenCode | N/A (Bun runtime) | N/A |

### Platform-Specific Features

| Provider | Per-OS Command Fields | Per-OS Config Paths | Shell Specification |
|----------|-----------------------|--------------------|---------------------|
| Claude Code | No | No | Bash implied |
| Gemini CLI | No | No | Bash + Node.js via shebang |
| Cursor | No | Yes (enterprise) | Bash implied |
| Windsurf | No | Yes | Unspecified |
| Copilot CLI | `bash`/`powershell` | No | Explicit per-platform |
| VS Code Copilot | `windows`/`linux`/`osx` | No | Bash (Unix), PowerShell (Win) |
| Kiro | No | No | Bash implied |
| OpenCode | No | No | Bun runtime |

---

## Key Findings for Canonical Format Design

### 1. Working Directory Is Inconsistent

Three approaches exist:
- **No CWD field** (Claude Code, Gemini, Cursor, Kiro): hooks run from current directory or project root with no override.
- **`working_directory`** (Windsurf): explicit field, resolves relative to workspace root, no `~` expansion.
- **`cwd`** (Copilot CLI, VS Code Copilot): explicit field, resolves relative to repository root.

A canonical format should support an optional `cwd` field that resolves relative to project root.

### 2. Platform-Specific Commands Need a Unified Model

Three approaches exist:
- **Single `command` string** (Claude, Gemini, Cursor, Windsurf, Kiro): authors handle platform differences in scripts.
- **`bash`/`powershell` split** (Copilot CLI): two fields, one per shell family.
- **`command` + `windows`/`linux`/`osx` overrides** (VS Code Copilot): most flexible, with fallback.

The VS Code Copilot model is the most expressive and backwards-compatible. A canonical format could adopt `command` as the default with optional per-OS overrides.

### 3. Environment Variables Are Provider-Branded

Each provider exposes its own `$PROVIDER_PROJECT_DIR` variable. Gemini CLI offers `$CLAUDE_PROJECT_DIR` as a compatibility alias. A canonical format should define a standard variable name that gets mapped to each provider's equivalent.

### 4. Inline Shell vs Script Files

Most providers accept both, but documentation quality varies. Only Claude Code and Kiro show clear inline expression examples. For portability, a canonical format should recommend script files over inline expressions, since shell syntax varies across platforms.

### 5. OpenCode Is Architecturally Different

OpenCode's TypeScript plugin model cannot be losslessly converted to/from shell-command hooks. Conversion would require either wrapping shell commands in Bun `$` calls (for import) or extracting shell commands from TypeScript (for export). This is a lossy boundary.

### 6. Script Permission Handling Is Mostly Undocumented

Only Gemini CLI and Cursor explicitly require `chmod +x`. A canonical format should document this requirement and potentially handle it during install (setting executable bits on installed scripts).

### 7. Custom Environment Variables

Only VS Code Copilot provides an `env` field for passing custom variables to hooks. This is a useful feature worth including in a canonical format, with conversion dropping it for providers that don't support it.
