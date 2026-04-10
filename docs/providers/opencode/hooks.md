# OpenCode Hooks

> **Identity note (2026-03-30):** Two projects were called "OpenCode". This documentation covers
> **SST's OpenCode** (anomalyco/opencode, 133K stars, TypeScript, actively maintained).
> The original Go project (opencode-ai/opencode) was archived September 2025 and became
> **Crush** under Charm. Syllago's toolmap targets SST's version exclusively.

## Overview

OpenCode does not have a dedicated "hooks" system in the way Claude Code does (where hooks are JSON-configured shell commands triggered by lifecycle events). Instead, OpenCode implements extensibility through a **plugin system** with event subscriptions. Plugins are TypeScript/JavaScript modules, not declarative config. [Official]

Source: https://opencode.ai/docs/plugins/

## Plugin-Based Event System

Plugins export a function that receives a context object and returns a hooks object mapping event names to handler functions.

### Plugin Locations
- `.opencode/plugins/` (project-level)
- `~/.config/opencode/plugins/` (global)
- NPM packages via `"plugin"` array in config

### Plugin Context Object
| Property    | Purpose                           |
|-------------|-----------------------------------|
| `project`   | Current project information       |
| `directory` | Current working directory         |
| `worktree`  | Git worktree path                 |
| `client`    | OpenCode SDK client for AI        |
| `$`         | Bun's shell API for commands      |

### Load Order
1. Global config plugins
2. Project config plugins
3. Global plugin directory files
4. Project plugin directory files

[Official] Source: https://opencode.ai/docs/plugins/

## Complete Event List

### Command Events
| Event                | Description                          |
|----------------------|--------------------------------------|
| `command.executed`   | Fired when a command is executed     |

### File Events
| Event                    | Description                      |
|--------------------------|----------------------------------|
| `file.edited`            | File was modified by a tool      |
| `file.watcher.updated`   | File watcher detected a change   |

### Installation Events
| Event                    | Description                      |
|--------------------------|----------------------------------|
| `installation.updated`   | OpenCode installation changed    |

### LSP Events
| Event                      | Description                    |
|----------------------------|--------------------------------|
| `lsp.client.diagnostics`   | LSP diagnostics received       |
| `lsp.updated`              | LSP state updated              |

### Message Events
| Event                    | Description                      |
|--------------------------|----------------------------------|
| `message.part.removed`  | Part of a message was removed    |
| `message.part.updated`  | Part of a message was updated    |
| `message.removed`       | A message was removed            |
| `message.updated`       | A message was updated            |

### Permission Events
| Event                  | Description                        |
|------------------------|------------------------------------|
| `permission.asked`     | User was asked for permission      |
| `permission.replied`   | User responded to permission ask   |

### Server Events
| Event                | Description                          |
|----------------------|--------------------------------------|
| `server.connected`   | Server connection established        |

### Session Events
| Event                  | Description                        |
|------------------------|------------------------------------|
| `session.created`      | New session started                |
| `session.compacted`    | Session context was compacted      |
| `session.deleted`      | Session was deleted                |
| `session.diff`         | Session diff generated             |
| `session.error`        | Session encountered an error       |
| `session.idle`         | Session became idle                |
| `session.status`       | Session status changed             |
| `session.updated`      | Session was updated                |

### Todo Events
| Event            | Description                            |
|------------------|----------------------------------------|
| `todo.updated`   | Todo list was modified                 |

### Shell Events
| Event         | Description                               |
|---------------|-------------------------------------------|
| `shell.env`   | Shell environment setup (modify env vars) |

### Tool Events
| Event                  | Description                        |
|------------------------|------------------------------------|
| `tool.execute.before`  | Before a tool executes (can block) |
| `tool.execute.after`   | After a tool executes              |

### TUI Events
| Event                  | Description                        |
|------------------------|------------------------------------|
| `tui.prompt.append`    | Content appended to prompt         |
| `tui.command.execute`  | TUI command executed               |
| `tui.toast.show`       | Toast notification shown           |

### Experimental Events
| Event                              | Description                |
|------------------------------------|----------------------------|
| `experimental.session.compacting`  | Session compaction started |

[Official] Source: https://opencode.ai/docs/plugins/

## Key Capabilities

### Tool Interception
The `tool.execute.before` event can prevent tool execution by throwing an error:

```typescript
// Block specific tool calls
export default (ctx) => ({
  hooks: {
    "tool.execute.before": (event) => {
      if (event.tool === "bash" && event.args.command.includes("rm -rf")) {
        throw new Error("Blocked dangerous command")
      }
    }
  }
})
```

[Official]

### Environment Injection
The `shell.env` event can modify environment variables for bash tool calls:

```typescript
export default (ctx) => ({
  hooks: {
    "shell.env": (event) => {
      event.output.env.MY_VAR = "value"
    }
  }
})
```

[Official]

### Logging
Plugins should use `client.app.log()` for structured logging instead of `console.*` methods. [Official]

## Dependencies

Local plugins can use npm packages. Add a `package.json` to the `.opencode/` directory with dependencies. OpenCode runs `bun install` at startup. [Official]

## Comparison with Claude Code Hooks

| Aspect           | Claude Code Hooks              | OpenCode Plugins                    |
|------------------|--------------------------------|-------------------------------------|
| Format           | JSON config (settings.json)    | TypeScript/JavaScript files         |
| Mechanism        | Shell command execution         | Event handler functions             |
| Events           | ~5 lifecycle events            | 25+ granular events                 |
| Tool interception| Not supported                  | Supported (tool.execute.before)     |
| Language         | Any (via shell)                | TypeScript/JavaScript only          |
| Configuration    | Declarative                    | Programmatic                        |
| Blocking         | Pre-tool hooks can block       | Throw error to block                |

## Syllago Implications

OpenCode's hook system is fundamentally different from Claude Code's:
- Claude Code hooks are declarative JSON merged into settings files
- OpenCode hooks are programmatic TypeScript/JavaScript plugin files
- Conversion between the two requires generating code from config or vice versa
- The event models overlap partially but are not 1:1
