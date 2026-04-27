# Windsurf Content Types

Research date: 2026-03-21

Windsurf (formerly Codeium) organizes AI coding content across six content types:
Rules, Skills, Workflows, Hooks, MCP, and AGENTS.md. Each operates at up to
three scope levels: system (enterprise/IT-managed), user (global), and workspace
(project-level).

---

## 1. Rules

Rules are persistent instructions included in Cascade's context. They control AI
behavior, enforce conventions, and provide project-specific guidance.

### File Format

Markdown (`.md`) files with optional YAML frontmatter declaring the activation
trigger.

### Locations

| Scope | Path | Char Limit |
|-------|------|------------|
| Global | `~/.codeium/windsurf/memories/global_rules.md` | 6,000 |
| Workspace | `.windsurf/rules/*.md` | 12,000 per file |
| System (macOS) | `/Library/Application Support/Windsurf/rules/*.md` | — |
| System (Linux/WSL) | `/etc/windsurf/rules/*.md` | — |
| System (Windows) | `C:\ProgramData\Windsurf\rules\*.md` | — |
| Legacy | `.windsurfrules` (project root) | — |

Global rules and root-level `AGENTS.md` files do not use frontmatter — they are
always on.

Workspace rules are discovered from the current directory, subdirectories, and
parent directories up to the git root.

### Activation Modes (Frontmatter)

**Always On** — applied to every Cascade action:
```yaml
---
trigger: always_on
---
Your rule content here.
```

**Manual** — activated by `@rule-name` mention in Cascade input:
```yaml
---
trigger: manual
---
Your rule content here.
```

**Model Decision** — Cascade decides whether to apply based on the rule's
description/content:
```yaml
---
trigger: model_decision
---
Your rule content here.
```

**Glob** — activates when working with files matching the pattern:
```yaml
---
trigger: glob
globs: **/*.test.ts
---
Your rule content here.
```

### Frontmatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `trigger` | string | Yes | One of: `always_on`, `manual`, `model_decision`, `glob` |
| `globs` | string | Only for `glob` trigger | File pattern (e.g. `**/*.py`, `src/**/*.ts`) |

[Official] https://docs.windsurf.com/windsurf/cascade/memories

---

## 2. Skills

Skills bundle instructions and supporting files into directories that Cascade
can invoke automatically or via `@mention`. They use progressive disclosure —
only the name and description are loaded until the skill is actually invoked.

### Directory Structure

```
<skills-root>/<skill-name>/
  SKILL.md          # Required — frontmatter + instructions
  [supporting files] # Optional — templates, scripts, checklists
```

### Locations

| Scope | Path |
|-------|------|
| Workspace | `.windsurf/skills/<skill-name>/` |
| Global | `~/.codeium/windsurf/skills/<skill-name>/` |
| System (macOS) | `/Library/Application Support/Windsurf/skills/` |
| System (Linux/WSL) | `/etc/windsurf/skills/` |
| System (Windows) | `C:\ProgramData\Windsurf\skills\` |
| Cross-agent | `.agents/skills/` and `~/.agents/skills/` |
| Claude compat | `.claude/skills/` and `~/.claude/skills/` (if enabled) |

### SKILL.md Format

```markdown
---
name: deploy-to-production
description: Guides the deployment process to production with safety checks
---

## Steps

1. Run pre-deployment checks...
2. Build the release artifact...
3. Deploy to staging first...
```

### SKILL.md Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier. Lowercase letters, numbers, hyphens only. Used for `@mention` activation. |
| `description` | string | Yes | Concise explanation. Helps Cascade decide when to auto-invoke the skill. |

### Activation

- **Automatic**: Cascade matches request to skill description and invokes it
- **Manual**: User types `@skill-name` in Cascade input

### Skills vs Rules vs Workflows

| Use case | Content type |
|----------|-------------|
| Needs supporting files, auto-invoked | Skill |
| Short behavioral constraint | Rule |
| Always triggered manually by user | Workflow |

### skills.json

Skills can also be referenced via `.windsurf/skills.json` in a project, which
tells Windsurf to load external skills when the project is opened. Format details
are sparse in official docs.

[Official] https://docs.windsurf.com/windsurf/cascade/skills

---

## 3. Workflows

Workflows are step-by-step procedures invoked via slash commands. They are
manual-only — Cascade never auto-triggers them.

### File Format

Markdown (`.md`) files containing a title, description, and ordered steps.

### Locations

| Scope | Path |
|-------|------|
| Workspace | `.windsurf/workflows/*.md` |
| Global | `~/.codeium/windsurf/global_workflows/*.md` |
| System (macOS) | `/Library/Application Support/Windsurf/workflows/` |
| System (Linux/WSL) | `/etc/windsurf/workflows/` |
| System (Windows) | `C:\ProgramData\Windsurf\workflows\` |

### Character Limit

12,000 characters per workflow file.

### Activation

Slash command: `/workflow-name` (derived from filename).

Workflows can invoke other workflows (nested invocation).

### Example

File: `.windsurf/workflows/run-tests-and-fix.md`
```markdown
# Run Tests and Fix

Run the test suite, analyze failures, and fix them.

## Steps

1. Run the full test suite with `npm test`
2. Parse any failing test output
3. For each failure, identify the root cause
4. Apply the fix and re-run the failing test
5. Confirm all tests pass
```

[Official] https://docs.windsurf.com/windsurf/cascade/workflows

---

## 4. Hooks

Hooks are shell commands that execute automatically at specific points in
Cascade's action pipeline. They enable logging, security controls, compliance
auditing, and custom automation.

### Configuration File

JSON file named `hooks.json`.

### Locations (merged in order)

| Scope | Path |
|-------|------|
| System (macOS) | `/Library/Application Support/Windsurf/hooks.json` |
| System (Linux/WSL) | `/etc/windsurf/hooks.json` |
| System (Windows) | `C:\ProgramData\Windsurf\hooks.json` |
| User | `~/.codeium/windsurf/hooks.json` |
| Workspace | `.windsurf/hooks.json` |

### Complete Schema

```json
{
  "hooks": {
    "<hook_type>": [
      {
        "command": "bash /path/to/script.sh",
        "show_output": true,
        "working_directory": "/optional/path"
      }
    ]
  }
}
```

### Hook Entry Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Yes | Shell command to execute. Any valid executable with arguments. |
| `show_output` | boolean | No | Display stdout/stderr in Cascade UI. Does not apply to `post_cascade_response`. |
| `working_directory` | string | No | Execution directory. Defaults to workspace root. |

### Hook Types (12 total)

**Pre-hooks** (can block action with exit code 2):

| Type | Trigger | Input JSON `tool_info` fields |
|------|---------|-------------------------------|
| `pre_read_code` | Before reading code files | `file_path` |
| `pre_write_code` | Before modifying code | `file_path`, `edits[{old_string, new_string}]` |
| `pre_run_command` | Before terminal commands | `command_line`, `cwd` |
| `pre_mcp_tool_use` | Before MCP tool invocation | `mcp_server_name`, `mcp_tool_name`, `mcp_tool_arguments` |
| `pre_user_prompt` | Before processing user input | `user_prompt` |

**Post-hooks** (informational, cannot block):

| Type | Trigger | Input JSON `tool_info` fields |
|------|---------|-------------------------------|
| `post_read_code` | After file reads | `file_path` |
| `post_write_code` | After code modifications | `file_path`, `edits[{old_string, new_string}]` |
| `post_run_command` | After command execution | `command_line`, `cwd` |
| `post_mcp_tool_use` | After MCP tool invocation | `mcp_server_name`, `mcp_tool_name`, `mcp_tool_arguments`, `mcp_result` |
| `post_cascade_response` | After agent completes response | `response` (markdown) |
| `post_cascade_response_with_transcript` | After response, with full transcript | `transcript_path` (JSONL file path) |
| `post_setup_worktree` | After git worktree creation | `worktree_path`, `root_workspace_path` |

### Common JSON Input Structure

All hooks receive this envelope via stdin:

```json
{
  "agent_action_name": "hook_event_name",
  "trajectory_id": "unique_conversation_id",
  "execution_id": "unique_turn_id",
  "timestamp": "ISO_8601_timestamp",
  "tool_info": { }
}
```

### Exit Code Behavior

| Code | Effect |
|------|--------|
| `0` | Success — action proceeds |
| `2` | Block (pre-hooks only) — halts action, shows message in UI |
| Other | Non-blocking error — action continues |

### Environment Variables

| Variable | Available in | Description |
|----------|-------------|-------------|
| `$ROOT_WORKSPACE_PATH` | `post_setup_worktree` | Absolute path to the original workspace |

### Example

```json
{
  "hooks": {
    "pre_write_code": [
      {
        "command": "python3 /scripts/validate-write.py",
        "show_output": true
      }
    ],
    "post_cascade_response": [
      {
        "command": "bash /scripts/log-response.sh",
        "show_output": false
      }
    ]
  }
}
```

[Official] https://docs.windsurf.com/windsurf/cascade/hooks

---

## 5. MCP (Model Context Protocol)

MCP connects Cascade to external tool servers (local or remote). Configuration
follows the same schema as Claude Desktop's MCP config.

### Configuration File

`mcp_config.json` — a single JSON file.

### Location

| Platform | Path |
|----------|------|
| All (user) | `~/.codeium/windsurf/mcp_config.json` |
| Windows | `%USERPROFILE%\.codeium\windsurf\mcp_config.json` |

No workspace-level or system-level MCP config locations are documented.

### Transport Types

- **stdio** — local process execution
- **Streamable HTTP** — remote HTTP endpoint
- **SSE** — Server-Sent Events

### Complete Schema

```json
{
  "mcpServers": {
    "<server-name>": {
      "command": "string",
      "args": ["string"],
      "env": { "KEY": "value" },
      "serverUrl": "string",
      "url": "string",
      "headers": { "KEY": "value" }
    }
  }
}
```

### Fields

| Field | Type | Transport | Required | Description |
|-------|------|-----------|----------|-------------|
| `command` | string | stdio | Yes | Executable to run (npx, docker, python3, etc.) |
| `args` | string[] | stdio | No | Command arguments |
| `env` | object | stdio | No | Environment variables as key-value pairs |
| `serverUrl` | string | HTTP | Yes | Remote HTTP endpoint URL |
| `url` | string | SSE | Yes | SSE endpoint URL |
| `headers` | object | HTTP/SSE | No | Custom HTTP headers |

### Environment Variable Interpolation

Supported in: `command`, `args`, `env`, `serverUrl`, `url`, `headers`.

Syntax: `${env:VARIABLE_NAME}`

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${env:GITHUB_TOKEN}"
      }
    }
  }
}
```

### Constraints

- **Tool limit**: 100 total tools across all connected MCP servers
- **Admin whitelisting**: When enabled, all non-whitelisted servers are blocked.
  Server ID must match the key name in `mcp_config.json` (case-sensitive).

### Remote HTTP Example

```json
{
  "mcpServers": {
    "remote-api": {
      "serverUrl": "https://api.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${env:AUTH_TOKEN}"
      }
    }
  }
}
```

[Official] https://docs.windsurf.com/windsurf/cascade/mcp

---

## 6. AGENTS.md

AGENTS.md provides directory-scoped instructions using the cross-tool standard
format. Windsurf feeds these into its Rules engine with activation mode inferred
from file location.

### Format

Plain markdown. No frontmatter required.

### Discovery

- Both `AGENTS.md` and `agents.md` recognized (case-insensitive)
- Scans workspace, subdirectories, and parent directories up to git root
- Multiple files at different directory levels are supported

### Scoping Behavior

| Location | Behavior |
|----------|----------|
| Repository root | Always-on rule — included in every Cascade message |
| Subdirectory | Glob rule with auto-generated pattern `<directory>/**` — applies only when Cascade works on files in that directory |

### Cross-Tool Compatibility

AGENTS.md is read natively by: Windsurf, Cursor, Codex CLI, GitHub Copilot, Amp,
and Devin.

[Official] https://docs.windsurf.com/windsurf/cascade/agents-md

---

## Content Types NOT Supported

Based on research, Windsurf does **not** have equivalents for:

- **Agents** (as standalone configurable entities) — Cascade is the single
  built-in agent. There is no user-defined agent configuration format.
  [Inferred]
- **Prompts** (as a saved/sharable content type) — No prompt library or saved
  prompt format beyond rules and workflows. [Inferred]
- **Commands** (as user-defined CLI commands) — Workflows serve this purpose via
  slash commands. [Inferred]

---

## Summary Table

| Content Type | Format | Workspace Path | User/Global Path | Activation |
|-------------|--------|---------------|------------------|------------|
| Rules | `.md` + YAML frontmatter | `.windsurf/rules/*.md` | `~/.codeium/windsurf/memories/global_rules.md` | always_on, manual, model_decision, glob |
| Skills | `SKILL.md` + YAML frontmatter | `.windsurf/skills/<name>/` | `~/.codeium/windsurf/skills/<name>/` | Auto (description match) or `@mention` |
| Workflows | `.md` (plain) | `.windsurf/workflows/*.md` | `~/.codeium/windsurf/global_workflows/*.md` | `/slash-command` |
| Hooks | `hooks.json` | `.windsurf/hooks.json` | `~/.codeium/windsurf/hooks.json` | Automatic (12 event types) |
| MCP | `mcp_config.json` | — | `~/.codeium/windsurf/mcp_config.json` | Always active |
| AGENTS.md | `.md` (plain) | Any directory | — | Auto (location-based) |
| Legacy rules | `.windsurfrules` | Project root | — | Always on |

---

## Sources

- [Cascade Memories & Rules](https://docs.windsurf.com/windsurf/cascade/memories) [Official]
- [Cascade Skills](https://docs.windsurf.com/windsurf/cascade/skills) [Official]
- [Cascade Workflows](https://docs.windsurf.com/windsurf/cascade/workflows) [Official]
- [Cascade Hooks](https://docs.windsurf.com/windsurf/cascade/hooks) [Official]
- [Cascade MCP Integration](https://docs.windsurf.com/windsurf/cascade/mcp) [Official]
- [AGENTS.md](https://docs.windsurf.com/windsurf/cascade/agents-md) [Official]
- [Windsurf MCP Tutorial](https://windsurf.com/university/tutorials/configuring-first-mcp-server) [Official]
- [awesome-windsurfrules](https://github.com/SchneiderSam/awesome-windsurfrules) [Community]
- [Windsurf Rules Directory](https://windsurf.com/editor/directory) [Official]
