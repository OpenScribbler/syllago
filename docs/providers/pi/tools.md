# Pi Agent Tools

> **Identity note:** "Pi" refers to Mario Zechner's `pi` coding agent
> (`github.com/badlogic/pi-mono`). Not to be confused with Raspberry Pi hardware
> or other "Pi" named projects.

> Research date: 2026-03-30
> Status: Draft -- compiled from official repository source code and docs.

## Overview

Pi ships with 7 built-in tools organized into two groups: **coding tools** (4)
and **read-only tools** (4), with `read` appearing in both. Extensions can
register additional custom tools or override built-in ones entirely.

Tool availability is configurable via the `--tools` CLI flag and the
`setActiveTools()` extension API.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/index.ts]

---

## Built-in Tools

### 1. read

Reads file contents from the filesystem.

**Group:** Coding, Read-Only

**Source:** `src/core/tools/read.ts`

Configurable via `ReadToolOptions` in tool initialization.

Cross-provider equivalent: Claude Code `Read`, Cursor `read_file`

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/read.ts]

---

### 2. write

Writes content to a file, creating it if it does not exist.

**Group:** Coding

**Source:** `src/core/tools/write.ts`

Participates in the per-file mutation queue (`withFileMutationQueue()`) to
ensure write ordering when multiple tools target the same file.

Cross-provider equivalent: Claude Code `Write`, Cursor `create_file`

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/write.ts]

---

### 3. edit

Edits existing file content with diff-based modifications.

**Group:** Coding

**Source:** `src/core/tools/edit.ts` (with `edit-diff.ts` for diff computation)

Also participates in the file mutation queue.

Cross-provider equivalent: Claude Code `Edit`, Cursor `edit_file`

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/edit.ts]

---

### 4. bash

Executes shell commands.

**Group:** Coding

**Source:** `src/core/tools/bash.ts`

Uses a dedicated bash executor (`bash-executor.ts`) for process management.
Configurable via `BashToolOptions`.

Cross-provider equivalent: Claude Code `Bash`, Cursor `run_terminal_command`

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/bash.ts]

---

### 5. grep

Searches file contents by pattern.

**Group:** Read-Only

**Source:** `src/core/tools/grep.ts`

Cross-provider equivalent: Claude Code `Grep`, Cursor `grep_search`

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/grep.ts]

---

### 6. find

Searches for files by name or pattern.

**Group:** Read-Only

**Source:** `src/core/tools/find.ts`

Cross-provider equivalent: Claude Code `Glob`, Cursor `file_search`

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/find.ts]

---

### 7. ls

Lists directory contents.

**Group:** Read-Only

**Source:** `src/core/tools/ls.ts`

Cross-provider equivalent: Claude Code `Bash` (`ls`), Cursor `list_dir`

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/ls.ts]

---

## Tool Groups

Pi organizes tools into named groups used by the SDK and CLI:

| Group | Tools | Use case |
|---|---|---|
| Coding | read, bash, edit, write | Full agent capabilities (default) |
| Read-Only | read, grep, find, ls | Analysis-only tasks |
| All | read, bash, edit, write, grep, find, ls | Complete toolset |

Factory functions create tool sets: `createCodingTools()`,
`createReadOnlyTools()`, `createAllTools()`. All require a `cwd` (working
directory) parameter.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/index.ts]

## Custom Tools via Extensions

Extensions can register entirely new tools or override built-in ones:

```typescript
import { Type } from "@sinclair/typebox";
import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.registerTool({
    name: "my_tool",
    label: "My Tool",
    description: "Does something useful",
    parameters: Type.Object({
      query: Type.String({ description: "Search query" }),
    }),
    async execute(toolCallId, params, signal, onUpdate, ctx) {
      return { content: `Result for: ${params.query}` };
    },
  });
}
```

Custom tools support:
- **TypeBox schemas** for parameter validation
- **Streaming updates** via `onUpdate` callback
- **Custom TUI rendering** via `renderCall()` and `renderResult()` methods
- **Prompt integration** via `promptSnippet` and `promptGuidelines` fields
- **Argument migration** via `prepareArguments()` for session backward compat

To override a built-in tool, register one with the same name. The extension's
tool replaces the built-in.

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md]

## Tool Output Management

Pi provides utilities for managing tool output size:

- `truncateHead()` / `truncateTail()` -- trim output to prevent context overflow
  (~50KB / 2000 lines recommended)
- File mutation queue -- ensures per-file write ordering across concurrent tool
  operations

[Official: https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/truncate.ts]

## Comparison with Other Providers

| Feature | Pi | Claude Code | Cursor |
|---|---|---|---|
| Built-in tools | 7 | 7 (Read, Write, Edit, Bash, Glob, Grep, WebFetch) | 10+ |
| Custom tools | Yes (extension API) | No | No |
| Tool override | Yes (same-name registration) | No | No |
| Output truncation | Built-in utilities | Automatic | Automatic |
| Semantic search | No (use grep) | No | Yes (codebase_search) |

## Sources

- [Tool Index (index.ts)](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/tools/index.ts) [Official]
- [Pi README](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/README.md) [Official]
- [Extensions Documentation](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md) [Official]
