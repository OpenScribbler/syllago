# Kiro — Format Reference

Provider slug: `kiro`

Supports: Rules, Agents, Hooks, MCP, Skills (Powers)

Sources: [Kiro docs](https://kiro.dev/docs/), [Kiro MCP config](https://kiro.dev/docs/mcp/configuration/), [Kiro hooks](https://kiro.dev/docs/cli/hooks/), [Kiro agent config](https://kiro.dev/docs/cli/custom-agents/configuration-reference/), [Kiro Powers](https://kiro.dev/docs/powers/create/)

---

## Rules (Steering Files)

**Location:** `<project>/.kiro/steering/*.md`

**Format:** Markdown

**Naming:** Descriptive filenames (e.g., `product.md`, `structure.md`, `tech.md`).

**Frontmatter:** Kiro steering files support optional YAML frontmatter with two fields:

| Field | Type | Description |
|-------|------|-------------|
| `inclusion` | string | Activation mode: `"alwaysInclude"`, `"includeWhenFileMatchesPattern"`, etc. |
| `fileMatchPattern` | string | Glob pattern for file-scoped rules (used with `includeWhenFileMatchesPattern`) |

Rules without frontmatter are treated as always-active.

**Default steering files** (created by `Kiro: Setup Steering for Project`):
- `product.md` — product vision, features, target users
- `structure.md` — project directory organization
- `tech.md` — technology stack and development tools

Custom steering files (e.g., `libraries.md`, `security.md`) can be added freely to the same directory.

**Example:**

```markdown
---
inclusion: includeWhenFileMatchesPattern
fileMatchPattern: "src/**/*.ts"
---

# Technology Stack

- **Runtime:** Node.js 22 with TypeScript 5.7
- **Framework:** Astro with Starlight
- **Testing:** Vitest with @testing-library/react
- **Package manager:** pnpm (not npm or yarn)
```

---

## Agents

**Location:** `<project>/.kiro/agents/*.json` (project) or `~/.kiro/agents/*.json` (global)

**Format:** JSON (filename becomes agent name)

**Agent config fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Agent display name |
| `description` | string | Yes | What the agent does |
| `prompt` | string | Yes | System prompt or `file://` reference to markdown file |
| `model` | string | No | Model ID (e.g., `claude-sonnet-4`) |
| `tools` | string[] | No | Tool allowlist: `["read", "write", "shell", "@git"]` |
| `allowedTools` | string[] | No | Granular tool access: `["read", "@git/git_status"]` |
| `mcpServers` | object | No | Embedded MCP server configs for this agent |
| `resources` | string[] | No | `file://` references injected as context |
| `keyboardShortcut` | string | No | IDE shortcut binding |
| `includeMcpJson` | bool | No | Include project MCP servers |

**Tool names:** `read`, `write`, `shell`, `@git`, `@git/status`, `@git/diff`, `@builtin`, `fs_write`, `*`

**Conversion notes:** When importing Kiro agents, the following Kiro-specific fields are dropped with warnings: `toolAliases`, `toolsSettings`, `resources`, `includeMcpJson`, `includePowers`, `keyboardShortcut`, `welcomeMessage`. These have no equivalent in syllago's canonical agent format.

**Example:**

```json
{
  "name": "aws-rust-agent",
  "description": "Specialized agent for AWS and Rust development",
  "prompt": "file://./prompts/aws-rust-expert.md",
  "model": "claude-sonnet-4",
  "tools": ["read", "write", "shell", "@git"],
  "mcpServers": { "fetch": { "command": "fetch-server", "args": [] } },
  "resources": ["file://README.md"],
  "keyboardShortcut": "ctrl+shift+r",
  "includeMcpJson": true
}
```

---

## Hooks

**Location:** Configured within agent JSON files (`.kiro/agents/*.json`)

**Five hook events:**

| Event | Trigger | Can Block? | Description |
|-------|---------|------------|-------------|
| `agentSpawn` | Agent starts | No | Run setup commands |
| `userPromptSubmit` | User sends prompt | No | Pre-processing |
| `preToolUse` | Before tool runs | Yes (exit code 2) | Validate/block tool calls |
| `postToolUse` | After tool completes | No | Post-processing |
| `stop` | Agent finishes | No | Cleanup |

**Hook config fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Yes | Shell command to execute |
| `matcher` | string | No | Tool filter: `"fs_write"`, `"read"`, `"shell"`, `"@git"`, `"*"`, `"@builtin"` |
| `timeout_ms` | int | No | Max execution time (default: 30000) |
| `cache_ttl_seconds` | int | No | Cache results for this duration |

**Hook I/O:** Receives JSON on stdin with tool name, input, and context. For `preToolUse`: exit code 0 = allow, exit code 2 = block.

---

## MCP Servers

**Location:** `<project>/.kiro/settings/mcp.json` (project) or `~/.kiro/settings/mcp.json` (global)

**Format:** JSON

**Server entry fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Conditional | Executable for stdio transport |
| `args` | string[] | No | Arguments for command |
| `env` | object | No | Environment variables (supports `${VAR_NAME}` expansion) |
| `url` | string | Conditional | Endpoint for remote transport |
| `headers` | object | No | HTTP headers |
| `disabled` | bool | No | Disable without removing |
| `autoApprove` | string[] | No | Tools to auto-approve |
| `disabledTools` | string[] | No | Tools to disable |

Workspace config overrides user-level config.

**Example:**

```json
{
  "mcpServers": {
    "web-search": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-bravesearch"],
      "env": { "BRAVE_API_KEY": "${BRAVE_API_KEY}" },
      "disabled": false,
      "autoApprove": ["brave_search"],
      "disabledTools": ["dangerous_tool"]
    },
    "remote-server": {
      "url": "https://endpoint.example.com",
      "headers": { "Authorization": "Bearer ${TOKEN}" },
      "autoApprove": ["*"]
    }
  }
}
```

---

## Skills (Powers)

**Location:** Self-contained directories with `POWER.md` at root

**Format:** Markdown with standard YAML frontmatter (Agent Skills spec)

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Skill display name |
| `description` | string | Yes | What the skill does |
| `license` | string | No | License identifier |
| `compatibility` | string | No | Compatibility information |
| `metadata` | object | No | Additional key-value metadata |

**Structure:**
```
power-name/
  POWER.md          # Description, activation keywords (YAML frontmatter + markdown)
  mcp.json          # MCP servers the power uses
  steering/         # Power-specific steering files
```

Powers activate automatically based on keyword detection in conversation context.

---

## Specs (Unique Feature)

**Location:** `<project>/.kiro/specs/`

Kiro's Spec-Driven Development (SDD) system:
- `requirements.md` — EARS-format requirements
- `design.md` — Technical design
- `tasks.md` — Implementation task breakdown

Not directly mappable to syllago content types but worth noting for future consideration.

---

## Detection

Detection signals:
- `.kiro/` directory exists in project root
- `~/.kiro/` global config directory exists

---

## Syllago Provider Mapping

| syllago ContentType | Kiro Equivalent | Path |
|-------------------|-----------------|------|
| `Rules` | Steering files | `.kiro/steering/*.md` |
| `MCP` | MCP config | `.kiro/settings/mcp.json` (JSON merge) |
| `Hooks` | Agent hooks | `.kiro/agents/*.json` hooks section |
| `Agents` | Agent configs | `.kiro/agents/*.json` |
| `Skills` | Powers | Powers system (complex mapping) |
| `Commands` | Not supported | — |

---

## Key Differences from Similar Tools

- **JSON agents, not markdown**: Agents are JSON configs that reference markdown prompts via `file://`
- **Hook caching**: Unique `cache_ttl_seconds` feature to avoid re-running expensive checks
- **Blocking hooks**: `preToolUse` hooks can return exit code 2 to prevent tool execution
- **Spec-driven development**: Built-in requirements/design/tasks workflow (unique to Kiro)
- **Powers**: Self-contained capability bundles with auto-activation (similar to skills but bundled with MCP and steering)
- **Env var expansion**: Uses `${VAR}` syntax in MCP configs
