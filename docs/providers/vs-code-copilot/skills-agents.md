# VS Code Copilot: Skills, Agents, and Custom Personas

Research date: 2026-03-31

**Identity note:** "VS Code Copilot" refers to GitHub Copilot's agent mode
within VS Code (`vs-code-copilot`). Distinct from `copilot-cli` (GitHub Copilot
in the terminal, `gh copilot`).

---

## Table of Contents

- [Skills (SKILL.md)](#skills-skillmd)
- [Custom Agents (.agent.md)](#custom-agents-agentmd)
- [Comparison with Claude Code](#comparison-with-claude-code)

---

## Skills (SKILL.md)

VS Code Copilot supports the `SKILL.md` format for portable, reusable agent
capabilities. Skills package domain-specific knowledge and workflows into
self-contained directories.

### SKILL.md Format

**File format:** Markdown with YAML frontmatter. [Official]

#### Frontmatter Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | Yes | -- | Lowercase identifier, hyphens for spaces. Must match parent directory name. Max 64 characters. |
| `description` | string | Yes | -- | What the skill does and when to use it. Be specific about capabilities and use cases. Max 1024 characters. |
| `showInMenu` | boolean | No | `true` | Show as slash command in the chat `/` menu. Set `false` to hide while still allowing auto-invocation. |

[Official: https://code.visualstudio.com/docs/copilot/customization/agent-skills]

#### Example

```markdown
---
name: deploy-staging
description: Deploys the application to the staging environment. Use after tests pass on the develop branch.
---

# Deploy to Staging

## Steps
1. Verify test suite passes: `npm test`
2. Build production bundle: `npm run build`
3. Deploy: `./scripts/deploy.sh staging`

## Prerequisites
- AWS CLI configured with staging credentials
- Node.js 20+
```

### Discovery and Invocation

- **Automatic discovery:** VS Code scans skill directories on startup.
- **Manual invocation:** Type `/skill-name` in agent chat.
- **Auto-invocation:** Agent reads `description` and invokes relevant skills
  autonomously.
- **Progressive loading:** Only `name` and `description` are read initially;
  full content loads on demand.

[Official: https://code.visualstudio.com/docs/copilot/customization/agent-skills]

### Creation Methods

- `/create-skill` in chat -- describe the skill, agent generates `SKILL.md`
- Extract from conversation: "create a skill from how we just debugged that"
- Manual creation in skill directories

[Official]

### Cross-Provider Portability

Skills created in VS Code work across:
- GitHub Copilot in VS Code (chat and agent mode)
- GitHub Copilot CLI (terminal)
- GitHub Copilot coding agent (automated tasks)

[Official: https://code.visualstudio.com/docs/copilot/customization/agent-skills]

---

## Custom Agents (.agent.md)

Custom agents define specialized AI personas with their own tools, models,
instructions, and handoffs. They are **not** the same concept as Claude Code's
`.claude/agents/` directory -- VS Code custom agents are richer, supporting
model selection, tool restrictions, hooks, and guided handoff workflows.

### File Format

`.agent.md` files use Markdown with YAML frontmatter. The body provides the
agent's system instructions.

[Official: https://code.visualstudio.com/docs/copilot/customization/custom-agents]

### File Locations

| Scope | Location |
|---|---|
| Workspace | `.github/agents/*.agent.md` |
| User (global) | User profile agents directory |

[Official]

### Frontmatter Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | No | Derived from filename | Agent identifier |
| `description` | string | No | -- | Brief summary, shown as chat input placeholder |
| `argument-hint` | string | No | -- | Guidance text for user interactions |
| `tools` | array | No | All tools | List of tool or tool set names available to this agent |
| `agents` | array | No | -- | List of agent names available as subagents |
| `model` | string or array | No | Default model | AI model to use; single name or prioritized list |
| `user-invocable` | boolean | No | `true` | Show in agents dropdown |
| `disable-model-invocation` | boolean | No | `false` | Prevent auto-invocation as subagent |
| `target` | string | No | -- | Environment: `"vscode"` or `"github-copilot"` |
| `mcp-servers` | array | No | -- | MCP server configs (for GitHub Copilot coding agent) |
| `hooks` | object | No | -- | Hook commands scoped to this agent (Preview) |
| `handoffs` | array | No | -- | Guided transitions to other agents |

[Official: https://code.visualstudio.com/docs/copilot/customization/custom-agents]

### Handoff Configuration

Each handoff object enables guided transitions between agents:

| Field | Type | Required | Description |
|---|---|---|---|
| `label` | string | Yes | Display text on the handoff button |
| `agent` | string | Yes | Target agent identifier |
| `prompt` | string | No | Text sent to the target agent |
| `send` | boolean | No | Auto-submit prompt if `true` (default: `false`) |
| `model` | string | No | Model override for the target agent |

[Official]

### Agent-Scoped Hooks

Custom agents can include hooks that only run when that agent is active. The
`hooks` field follows the same format as hook configuration files:

```yaml
---
name: secure-reviewer
description: Reviews code for security issues
hooks:
  PostToolUse:
    - type: command
      command: "./scripts/security-scan.sh"
---
```

Agent-scoped hooks fire **in addition to** workspace and user-level hooks.
[Official]

### Example Custom Agent

```markdown
---
name: security-auditor
description: Reviews code changes for security vulnerabilities
tools: ["read", "search", "search/codebase", "read/problems"]
model: claude-sonnet-4-20250514
user-invocable: true
handoffs:
  - label: "Fix issues"
    agent: "coder"
    prompt: "Fix the security issues identified in the review above"
    send: false
---

# Security Auditor

You are a security-focused code reviewer. Analyze code for:

- SQL injection and XSS vulnerabilities
- Authentication and authorization bypasses
- Hardcoded secrets or credentials
- Insecure cryptographic practices

Report findings with severity levels: Critical, High, Medium, Low.

**Do not modify any files.** Only analyze and report.
```

### Creating Custom Agents

- From the agents dropdown in Chat view > **Configure Custom Agents** > **Create
  new custom agent**
- `/create-agent` command in chat
- Manual creation in `.github/agents/`

[Official]

---

## Comparison with Claude Code

| Feature | VS Code Copilot | Claude Code |
|---|---|---|
| **Skills** | `SKILL.md` with `name`, `description`, `showInMenu` | `SKILL.md` with `name`, `description` |
| **Skill invocation** | `/skill-name` or auto-invoked | `/skill-name` or auto-invoked via Skill tool |
| **Skill cross-provider** | Works across VS Code, CLI, coding agent | Claude Code only |
| **Custom agents** | `.agent.md` with model, tools, hooks, handoffs | `.claude/agents/*.md` with `name`, `tools` |
| **Agent model selection** | Yes (`model` field) | No |
| **Agent tool restriction** | Yes (`tools` array) | Yes (`tools`, `disallowedTools`) |
| **Agent-scoped hooks** | Yes (`hooks` field) | No |
| **Agent handoffs** | Yes (guided transitions) | No |
| **Subagent spawning** | Via `agents` field | Via `Agent` tool |
| **Config file agents** | Yes (`.agent.md` files) | Yes (`.claude/agents/*.md`) |

### What VS Code Copilot Does NOT Have (vs Claude Code)

- **No `CLAUDE.md` agent definitions** -- VS Code reads `CLAUDE.md` as
  instructions only, not as agent configs. Agent definitions use `.agent.md`
  format exclusively. [Official]
- **No `disallowedTools`** -- Tool restriction is inclusion-based (`tools`
  array), not exclusion-based. [Inferred]

### What VS Code Copilot Adds Beyond Claude Code

- **Model selection per agent** -- Each agent can specify its own model.
- **Handoff workflows** -- Guided transitions between specialized agents.
- **Agent-scoped hooks** -- Hooks that only fire for a specific agent.
- **Target environment** -- Agents can be scoped to `vscode` or
  `github-copilot` (coding agent).

---

## Sources

- [Agent Skills in VS Code](https://code.visualstudio.com/docs/copilot/customization/agent-skills) [Official]
- [Custom agents in VS Code](https://code.visualstudio.com/docs/copilot/customization/custom-agents) [Official]
- [Using agents in VS Code](https://code.visualstudio.com/docs/copilot/agents/overview) [Official]
- [Tutorial: Work with agents in VS Code](https://code.visualstudio.com/docs/copilot/agents/agents-tutorial) [Official]
- [Customize AI in VS Code](https://code.visualstudio.com/docs/copilot/customization/overview) [Official]
