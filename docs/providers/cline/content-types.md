# Cline Content Types

## Rules (`.clinerules`)

### Overview

Project-specific or global instructions that get appended directly to Cline's system
prompt. Written in plain Markdown. Can exist as a single file or a directory of files.

### Locations

| Scope | Location |
|-------|----------|
| Workspace | `.clinerules` file or `.clinerules/` directory at project root |
| Global (macOS) | `~/Documents/Cline/Rules/` |
| Global (Linux/WSL) | `~/Documents/Cline/Rules/` (fallback: `~/Cline/Rules/`) |
| Global (Windows) | `Documents\Cline\Rules\` |

Workspace rules take precedence over global rules when they conflict. Both are
combined when present.

### File Format

Plain Markdown (`.md` or `.txt`). Optional YAML frontmatter for conditional activation.

**Simple rule (always active):**

```markdown
# Coding Standards

- Use TypeScript strict mode
- Prefer functional patterns
- All functions must have JSDoc comments
```

**Conditional rule (activates only for matching files):**

```yaml
---
paths:
  - "src/components/**"
  - "src/hooks/**"
---

# React Component Rules

- Use functional components only
- Extract custom hooks for shared logic
```

### Directory Structure

```
project-root/
├── .clinerules/           # Directory approach (multiple files)
│   ├── 01-coding.md       # Numeric prefixes optional, for ordering
│   ├── 02-testing.md
│   └── architecture.md
```

Cline processes all `.md` and `.txt` files inside `.clinerules/`, combining them into
a unified set of rules.

### Conditional Rules — Glob Patterns

The `paths` field in YAML frontmatter supports standard glob patterns:

| Pattern | Matches |
|---------|---------|
| `*` | Any characters except `/` |
| `**` | Any characters including `/` (recursive) |
| `?` | Single character |
| `[abc]` | Bracketed characters |
| `{a,b}` | Either pattern |

Rules activate when ANY pattern matches ANY file in the current context (open editors,
mentioned files, modified files, pending operations).

Rules without frontmatter are always active. `paths: []` (empty array) prevents
activation entirely.

### Toggle UI

Since v3.13, rules can be toggled on/off via a popover below the chat input. Rules
are also AI-editable — Cline can modify `.clinerules` files using its file tools.

### Loading Behavior

Rules are NOT watched for changes during an active task. Updates take effect:
- Between tasks
- On task resume
- After extension reload

[Official] https://docs.cline.bot/customization/cline-rules
[Official] https://cline.bot/blog/clinerules-version-controlled-shareable-and-ai-editable-instructions

---

## Custom Instructions (Legacy/Global)

### Overview

Global text field in Cline's VS Code extension settings. Applies to all projects and
all conversations. Not version-controlled.

As of v3.17+, Cline is transitioning from the settings text field to the file-based
global rules system (stored in `~/Documents/Cline/Rules/`). The custom instructions
field may be removed in future versions.

**Format:** Plain text or Markdown, entered in the Cline extension settings UI.

**Relationship to rules:** Custom instructions were the original mechanism. Global
rules files are the current replacement, providing the same functionality with
file-based management, toggleability, and AI-editability.

[Official] https://docs.cline.bot/customization/cline-rules
[Community] https://github.com/cline/cline/issues/4294

---

## MCP Server Configuration

### Overview

MCP server definitions stored in a dedicated JSON file. Configures which MCP servers
Cline can connect to for additional tools and resources.

### File Location

`cline_mcp_settings.json` — accessed via Cline's MCP Servers panel in VS Code.
Stored in VS Code's extension storage directory (not in the workspace).

### Format

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

### Fields — STDIO Transport (Local)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Yes | Executable to run (`node`, `python`, `npx`, `uv`, etc.) |
| `args` | string[] | No | Arguments passed to the command |
| `env` | object | No | Environment variables for the server process |
| `alwaysAllow` | string[] | No | Tool names that auto-approve without user confirmation |
| `disabled` | boolean | No | Disable the server without removing config |

### Fields — SSE Transport (Remote)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | HTTPS endpoint for the remote server |
| `headers` | object | No | Authentication tokens and custom headers |
| `alwaysAllow` | string[] | No | Tool names that auto-approve |
| `disabled` | boolean | No | Disable the server |

### Notes

- Network timeout is configurable (30s to 1h, default 1 minute)
- Cline auto-detects available tools after server configuration
- Currently NOT workspace-scoped — it's per-user extension storage
  (community request exists to move to VS Code `settings.json` or `.vscode/mcp.json`)

[Official] https://docs.cline.bot/mcp/configuring-mcp-servers

---

## `.clineignore`

### Overview

Controls which files Cline can access. Uses `.gitignore` syntax. Placed at workspace
root.

### Format

Standard `.gitignore` pattern syntax (uses the `ignore` npm library):

```gitignore
# Dependencies
node_modules/

# Secrets
.env
.env.*

# Build output
/dist/
/build/

# Large data
*.csv
*.sqlite
```

### Behavior

- File is auto-watched for changes (VS Code file watcher)
- The `.clineignore` file itself is always ignored
- If absent, all files are accessible (fail-open)
- Monorepos: each workspace root can have its own `.clineignore`
- Affects: `read_file`, `list_files`, `search_files`, and terminal command validation
  (e.g., `cat .env` is blocked)

### Purpose

- **Security:** Prevent access to credentials, secrets, sensitive data
- **Performance:** Exclude irrelevant large directories to reduce context token usage
- **Focus:** Keep Cline's attention on relevant project files

[Official] https://docs.cline.bot/customization/clineignore

---

## Memory Bank (Convention, Not Built-in Feature)

### Overview

A community-developed methodology for persistent project context across sessions.
NOT a built-in Cline feature — it's a pattern using regular Markdown files plus
`.clinerules` instructions that tell Cline how to read/write them.

### Typical Structure

```
project-root/
├── memory-bank/
│   ├── projectbrief.md      # Project goals and scope
│   ├── activeContext.md      # Current state and focus
│   ├── progress.md           # What's done, what's remaining
│   ├── systemPatterns.md     # Architecture and technical patterns
│   └── techContext.md        # Technology stack and workflows
```

### How It Works

A `.clinerules` file instructs Cline to:
1. Read memory bank files at session start
2. Update them as work progresses
3. Use `new_task` to hand off context when the context window fills up

### Key Files

| File | Purpose |
|------|---------|
| `projectbrief.md` | Project goals, what you're building (keep to ~1 page) |
| `activeContext.md` | Current state only, not a running log |
| `progress.md` | Summary of done/remaining (not detailed changelog) |
| `systemPatterns.md` | Architecture, structural guidelines |
| `techContext.md` | Tech stack, workflows, tooling |

[Community] https://docs.cline.bot/features/memory-bank
[Community] https://github.com/nickbaumann98/cline_docs/blob/main/prompting/custom%20instructions%20library/cline-memory-bank.md

---

## Community Prompts Library

### Overview

A GitHub repository (`cline/prompts`) of community-contributed `.clinerules` and
workflow templates. Browsable from inside the Cline extension via the Prompts Library.

Not a content type per se, but a distribution channel for rules content.

[Official] https://github.com/cline/prompts

---

## Content Types Summary

| Content Type | File/Location | Format | Scope | Syllago-Relevant |
|-------------|---------------|--------|-------|-----------------|
| Rules | `.clinerules` / `.clinerules/*.md` | Markdown + optional YAML frontmatter | Workspace | Yes |
| Global Rules | `~/Documents/Cline/Rules/*.md` | Markdown + optional YAML frontmatter | Global | Yes |
| Custom Instructions | Extension settings UI | Plain text/Markdown | Global | Possibly (legacy) |
| MCP Config | `cline_mcp_settings.json` | JSON (`mcpServers` object) | Global (extension storage) | Yes |
| `.clineignore` | `.clineignore` at project root | `.gitignore` syntax | Workspace | Maybe |
| Memory Bank | `memory-bank/*.md` (convention) | Markdown | Workspace | No (user content, not config) |
