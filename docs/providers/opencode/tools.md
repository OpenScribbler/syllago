# OpenCode Tools

## Overview

OpenCode provides 16 built-in tools plus support for custom tools (TypeScript/JavaScript) and MCP server tools. Tools are available to agents based on permission configuration. [Official]

Source: https://opencode.ai/docs/tools/

## Built-in Tools

| Tool Name    | Purpose                                              | Permission Key |
|--------------|------------------------------------------------------|----------------|
| `bash`       | Execute shell commands in the project environment    | `bash`         |
| `edit`       | Modify existing files using exact string replacement | `edit`         |
| `write`      | Create new files or overwrite existing ones          | `edit`         |
| `patch`      | Apply diff/patch files to modify code                | `edit`         |
| `read`       | Read file contents (supports line ranges)            | `read`         |
| `grep`       | Search file contents using regex                     | `grep`         |
| `glob`       | Find files by glob pattern matching                  | `glob`         |
| `list`       | List files and directories (supports glob filters)   | `list`         |
| `webfetch`   | Fetch and read web page content                      | `webfetch`     |
| `websearch`  | Search the web (uses Exa AI)                         | `websearch`    |
| `todowrite`  | Create/manage todo lists during sessions             | `todowrite`    |
| `todoread`   | Read current todo list state                         | `todoread`     |
| `question`   | Ask the user questions with selectable options        | (always on)    |
| `skill`      | Load SKILL.md content into conversation              | `skill`        |
| `lsp`        | LSP operations (definitions, references, hover)      | `lsp`          |
| `task`       | Invoke a subagent for delegated work                 | `task`         |

[Official] Source: https://opencode.ai/docs/tools/

## Tool Details

### bash
Runs terminal commands. Supports glob-based permission patterns (e.g., `"git *": "allow"`, `"rm *": "deny"`). [Official]

### edit
Primary file modification tool. Uses exact string matching to find and replace text. Controlled by the `edit` permission key. [Official]

### write
Creates new files or fully overwrites existing ones. Shares the `edit` permission key with `edit` and `patch`. [Official]

### patch
Applies diff/patch content to files. Also controlled by the `edit` permission. [Official]

### read
Reads file contents. Supports specifying line ranges for large files. [Official]

### grep
Searches file contents using full regex syntax. Supports file pattern filtering. [Official]

### glob
Finds files matching glob patterns (e.g., `**/*.js`). Returns results sorted by modification time. [Official]

### list
Lists files and directories at a given path. Accepts glob patterns to filter results. [Official]

### webfetch
Fetches web content for documentation research. Controlled by the `webfetch` permission. [Official]

### websearch
Web search via Exa AI. Requires `OPENCODE_ENABLE_EXA=true` environment variable. No separate API key needed. [Official]

### todowrite / todoread
Manages multi-step task tracking during sessions. Disabled for subagents by default. [Official]

### question
Asks the user questions during execution. Supports selectable options for choices. Always available. [Official]

### skill
Loads SKILL.md file content into the conversation on demand. Controlled by `skill` permission with glob-based rules. [Official]

### lsp
Interacts with LSP servers for code intelligence. Supports `goToDefinition`, `findReferences`, and hover. Experimental -- requires `OPENCODE_EXPERIMENTAL_LSP_TOOL=true`. [Official]

### task
Invokes a subagent for delegated work. Permission supports glob patterns to control which subagents can be invoked (e.g., `"general": "allow"`, `"explore": "deny"`). [Official]

## Custom Tools

Custom tools are TypeScript/JavaScript files placed in:
- `.opencode/tools/` (project-level)
- `~/.config/opencode/tools/` (global)

The filename becomes the tool name. Uses the `tool()` helper from `@opencode-ai/plugin`:

```typescript
import { tool } from "@opencode-ai/plugin"
export default tool({
  description: "Query the project database",
  args: {
    query: tool.schema.string().describe("SQL query to execute"),
  },
  async execute(args) {
    return `Executed query: ${args.query}`
  },
})
```

Multiple tools per file: use named exports to create `<filename>_<exportname>` tools.

Custom tools receive a context object with: `agent`, `sessionID`, `messageID`, `directory`, `worktree`.

Custom tools override built-in tools with matching names.

[Official] Source: https://opencode.ai/docs/custom-tools/

## Tool Permission Configuration

Tools are controlled via the `permission` config key in `opencode.json`:

```json
{
  "permission": {
    "*": "ask",
    "bash": {
      "*": "ask",
      "git *": "allow",
      "rm *": "deny"
    },
    "edit": "allow",
    "skill": {
      "*": "allow",
      "internal-*": "deny"
    }
  }
}
```

Permission values: `"allow"` (no approval), `"ask"` (user approval), `"deny"` (blocked).

Additional special permission keys:
- `external_directory` -- access outside the project directory (default: `"ask"`)
- `doom_loop` -- repeated identical tool calls 3+ times (default: `"ask"`)
- `codesearch` -- code search operations

Tools can be enabled/disabled per-agent using the `tools` config:

```json
{
  "tools": {
    "my-mcp-foo": false,
    "my-mcp*": false
  }
}
```

[Official] Source: https://opencode.ai/docs/permissions/

## Syllago Mapping

| Syllago Tool Name | OpenCode Tool Name | Notes                                    |
|-------------------|--------------------|------------------------------------------|
| view (Read)       | `read`             | Direct equivalent                        |
| write             | `write`            | Direct equivalent                        |
| edit              | `edit`             | String replacement vs diff-based         |
| bash              | `bash`             | Direct equivalent                        |
| glob              | `glob`             | Direct equivalent                        |
| grep              | `grep`             | Direct equivalent                        |
| fetch (WebSearch) | `webfetch`         | OpenCode also has separate `websearch`   |
| agent (Task)      | `task`             | Subagent delegation                      |
| --                | `patch`            | No direct syllago equivalent             |
| --                | `list`             | No direct syllago equivalent             |
| --                | `lsp`              | No direct syllago equivalent             |
| --                | `skill`            | No direct syllago equivalent             |
| --                | `todowrite`        | No direct syllago equivalent             |
| --                | `todoread`         | No direct syllago equivalent             |
| --                | `question`         | No direct syllago equivalent             |
| --                | `websearch`        | Separate from webfetch                   |
