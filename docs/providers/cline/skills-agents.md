# Cline Skills & Agents

Cline does not have a discrete "skills" or "agents" content type. Its equivalent
concepts are **.clinerules** (custom instructions), **Plan & Act modes** (behavioral
modes), and **MCP server integration** (tool extensibility).

## .clinerules (Custom Instructions)

`.clinerules` is Cline's primary mechanism for customizing agent behavior. Rules are
markdown/text files that get injected into the system prompt.

[Official] https://docs.cline.bot/customization/cline-rules

### File Structure

```
<project-root>/
├── .clinerules/
│   ├── 01-coding-style.md
│   ├── 02-testing.md
│   ├── architecture.md
│   └── hooks/           # hook scripts live here too
└── ...
```

Cline processes all `.md` and `.txt` files in `.clinerules/`, combining them into
unified instructions. Numeric prefixes are optional organizational aids.

### Global vs Workspace Rules

| Scope | Location | Use Case |
|---|---|---|
| Workspace | `<project>/.clinerules/` | Team standards, project constraints (version-controlled) |
| Global | `~/Documents/Cline/Rules/` (macOS/Linux) | Personal preferences across all projects |
| Global | `Documents\Cline\Rules` (Windows) | Personal preferences across all projects |

Workspace rules take precedence during conflicts.

### Cross-Provider Rule Detection

Cline auto-detects rules from other providers:

| File | Provider |
|---|---|
| `.clinerules/` | Cline (primary) |
| `.cursorrules` | Cursor |
| `.windsurfrules` | Windsurf |
| `AGENTS.md` | Cross-tool compatible |

[Official] https://docs.cline.bot/customization/cline-rules

### Conditional Rules (Path-Based)

Rules can target specific file paths using YAML frontmatter:

```yaml
---
paths:
  - "src/components/**"
  - "src/hooks/**"
---

# React Component Guidelines
Use functional components with hooks...
```

- `*` matches characters except `/`
- `**` matches recursively including `/`
- `{a,b}` matches either pattern
- Rules without frontmatter are always active
- A rule activates if **any** pattern matches **any** file in context
- Context includes: files mentioned in prompt, open editor tabs, visible panes,
  files Cline creates/modifies, pending operations

[Official] https://docs.cline.bot/customization/cline-rules

### Toggleable Rules

A popover below the chat input shows active rules. Users can enable/disable specific
rule files with a click. This provides two levels of control: manual toggles and
automatic condition-based activation.

[Official] https://cline.ghost.io/clinerules-version-controlled-shareable-and-ai-editable-instructions/

### AI-Editable Rules

Because rules are regular files, Cline can read, write, and edit them. You can tell
Cline "update the api-style-guide.md rule to include pagination standards" and it will
modify the file directly -- creating a feedback loop where the agent refines its own
instructions.

[Official] https://cline.ghost.io/clinerules-version-controlled-shareable-and-ai-editable-instructions/

## Plan & Act Modes

Cline has two behavioral modes (not custom persona modes like Roo Code). These are
built-in and not user-definable.

[Official] https://deepwiki.com/cline/cline/3.4-plan-and-act-modes

### Plan Mode

- Read-only exploration and architecture
- Restricted tool access (no file writes, no command execution)
- AI focuses on analysis, clarifying questions, and solution design
- Special system prompt that constrains behavior to planning

### Act Mode

- Full tool access for implementation
- File creation/editing, command execution, browser use, MCP tools
- Standard execution mode

### Independent Model Configuration

Each mode can use a different AI model. Controlled by `planActSeparateModelsSetting`
in global state.

Typical pattern: reasoning-heavy model (Claude Opus, o3) for Plan mode, faster/cheaper
model (Claude Sonnet, GPT-4o) for Act mode.

[Official] https://deepwiki.com/cline/cline/3.4-plan-and-act-modes

### Switching

- **VS Code**: Toggle button in the Cline panel
- **CLI**: Press `Tab` to switch between modes during interactive sessions

[Official] https://docs.cline.bot/cline-cli/interactive-mode

### No Custom Modes

Cline does **not** support user-defined custom modes or persona-based agent roles
(unlike Roo Code which has Architect, Code, Debug, Ask, and Custom modes). There is a
community proposal for "LLM Profiles" that would allow saving named Plan+Act model
combinations, but this is not yet implemented.

[Community] https://github.com/cline/cline/discussions/3412

## MCP Tool Integration

Cline integrates MCP servers as first-class tools the agent can invoke during task
execution.

### Configuration File

MCP servers are configured in `cline_mcp_settings.json`:

| Platform | Path |
|---|---|
| macOS | `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |
| Linux | `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |
| Windows | `%APPDATA%/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |

[Official] https://docs.cline.bot/mcp/configuring-mcp-servers

### Configuration Format

```json
{
  "mcpServers": {
    "server-name": {
      "command": "node",
      "args": ["/path/to/server.js"],
      "env": {
        "API_KEY": "your_api_key"
      },
      "alwaysAllow": ["tool1", "tool2"],
      "disabled": false
    }
  }
}
```

| Field | Type | Description |
|---|---|---|
| `command` | string | Executable to run (`node`, `python`, `npx`, etc.) |
| `args` | string[] | Arguments passed to the command |
| `env` | object | Environment variables for the server process |
| `alwaysAllow` | string[] | Tool names auto-approved without prompting |
| `disabled` | boolean | Enable/disable the server |

### Transport Types

- **STDIO**: Local process (child process spawned by Cline)
- **SSE**: Remote server via Server-Sent Events URL

### Scope Limitation

MCP configuration is **global only** -- a single config file applies across all
projects/workspaces. There is no project-level MCP configuration support yet.

[Community] https://github.com/cline/cline/discussions/2418

### Natural Language Setup

Cline can install MCP servers conversationally. Tell it "add a tool for X" and it
handles server creation and registration.

[Official] https://cline.bot/blog/the-developers-guide-to-mcp-from-basics-to-advanced-workflows

## Auto-Approve Settings

Cline provides granular control over which actions the agent can take without user
approval.

[Official] https://docs.cline.bot/features/auto-approve

### Permission Categories

| Category | Description |
|---|---|
| Read project files | Access workspace contents |
| Read all files | Extend beyond workspace (requires base toggle) |
| Edit project files | Create/modify workspace files |
| Edit all files | Modify files outside workspace (requires base toggle) |
| Execute safe commands | Run commands the model deems safe |
| Execute all commands | Include potentially dangerous commands |
| Use the browser | Web fetching and search |
| Use MCP servers | Access MCP tools and resources |

### YOLO Mode

Auto-approves everything: file changes, terminal commands, browser actions, MCP tools,
and mode transitions. Disables all safety checks.

### MCP-Specific Auto-Approve

Individual MCP tools can be auto-approved via the `alwaysAllow` array in
`cline_mcp_settings.json`. This is per-server, per-tool granularity.

[Official] https://docs.cline.bot/features/auto-approve

## Diff Strategy

Cline uses a search-and-replace diff-based approach for file edits, presenting changes
in the editor's native diff view. Users can edit or revert changes directly in the diff
view.

v3.12 significantly improved diff edit performance for large files, adding an indicator
showing the number of edits being applied.

[Official] https://cline.bot/blog/cline-v3-12-faster-diff-edits-model-favorites-and-more

## Syllago Mapping Considerations

| Cline Concept | Syllago Content Type | Notes |
|---|---|---|
| `.clinerules/*.md` | Rules | Direct mapping; markdown files with optional YAML frontmatter |
| `.clinerules/hooks/` | Hooks | Executable scripts, 8 event types |
| `cline_mcp_settings.json` | MCP configs | JSON merge into settings file; global-only scope |
| Plan & Act modes | N/A | Built-in behavioral modes, not user-defined content |
| Auto-approve settings | N/A | Runtime configuration, not shareable content |

[Inferred] Mapping analysis based on syllago's content type model.
