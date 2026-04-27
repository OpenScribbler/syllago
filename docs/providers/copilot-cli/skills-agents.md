# GitHub Copilot CLI: Skills and Agents Reference

Comprehensive reference for GitHub Copilot CLI's skill (`SKILL.md`), custom agent (`.agent.md`), and extension systems.

**Last updated:** 2026-03-21

---

## Table of Contents

- [Skills (Agent Skills)](#skills-agent-skills)
  - [Overview](#skills-overview)
  - [Skill File Format](#skill-file-format)
  - [Frontmatter Fields](#skill-frontmatter-fields)
  - [Directory Structure](#skill-directory-structure)
  - [Storage Locations](#skill-storage-locations)
  - [Progressive Loading](#progressive-loading)
  - [Skills vs Custom Instructions](#skills-vs-custom-instructions)
- [Custom Agents](#custom-agents)
  - [Overview](#agents-overview)
  - [Agent File Format](#agent-file-format)
  - [Frontmatter Fields](#agent-frontmatter-fields)
  - [Tools Configuration](#tools-configuration)
  - [MCP Server Configuration](#mcp-server-configuration)
  - [Storage Locations](#agent-storage-locations)
  - [Subagent Execution](#subagent-execution)
  - [Priority and Deduplication](#priority-and-deduplication)
- [Copilot Extensions (Platform)](#copilot-extensions-platform)
  - [Skillsets vs Agents](#skillsets-vs-agents)
- [Custom Instructions](#custom-instructions)
- [Comparison with Claude Code](#comparison-with-claude-code)
- [Source Index](#source-index)

---

## Source Attribution Key

- `[Official]` -- from docs.github.com official documentation
- `[Community]` -- from community blogs, tutorials, or GitHub examples
- `[Inferred]` -- logically derived from official docs but not explicitly stated
- `[Unverified]` -- mentioned in search results but not confirmed in primary sources

---

## Skills (Agent Skills)

### Skills Overview

Agent Skills are folders of instructions, scripts, and resources that Copilot loads when relevant to a task. They are an **open standard** that works across multiple AI agents -- Copilot in VS Code, Copilot CLI, Copilot Coding Agent, and Claude Code. `[Official]`

Skills were announced December 18, 2025 and reached VS Code in version 1.108. `[Official]`

Unlike custom instructions (which apply broadly), skills are **task-specific** -- Copilot only loads them when the skill's description matches the user's prompt. This keeps context efficient. `[Official]`

### Skill File Format

Every skill is a directory containing a `SKILL.md` file. The file has two parts:

1. **YAML frontmatter** between `---` markers -- metadata
2. **Markdown body** -- instructions Copilot follows when the skill is invoked

```yaml
---
name: deploy-production
description: Guides deployment to production environments including pre-flight checks, rollback procedures, and post-deploy validation.
license: MIT
---

## Instructions

When deploying to production, always run the pre-flight checklist first...
```

`[Official]`

### Skill Frontmatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Lowercase identifier using hyphens for spaces; typically matches directory name |
| `description` | string | Yes | Explains the skill's purpose and when Copilot should use it. This is the primary matching signal. |
| `license` | string | No | License information for the skill |

`[Official]`

**Note:** The Copilot skill frontmatter is simpler than Claude Code's, which supports additional fields like `tools`, `context`, and invocation control (`alwaysApply`, `autoApply`, etc.). `[Inferred]`

### Skill Directory Structure

```
my-skill/
  SKILL.md              # Required: main skill file
  examples/             # Optional: example files
    good-pattern.py
    bad-pattern.py
  scripts/              # Optional: helper scripts
    validate.sh
  reference.md          # Optional: supplementary docs
```

The skill file must be named exactly `SKILL.md`. Supplementary resources (scripts, example files, additional Markdown) are referenced from within SKILL.md instructions. `[Official]`

### Skill Storage Locations

| Level | Path | Scope |
|-------|------|-------|
| Repository | `.github/skills/{skill-name}/SKILL.md` | Project-specific |
| Repository (Claude compat) | `.claude/skills/{skill-name}/SKILL.md` | Project-specific (cross-provider) |
| Personal | `~/.copilot/skills/{skill-name}/SKILL.md` | All projects for this user |
| Personal (Claude compat) | `~/.claude/skills/{skill-name}/SKILL.md` | All projects for this user |

Subdirectory names should be lowercase with hyphens replacing spaces. `[Official]`

### Progressive Loading

Skills load content in three stages to keep context efficient: `[Official]`

1. **Discovery** -- Copilot reads only the skill's `name` and `description` from YAML frontmatter
2. **Instructions loading** -- When a prompt matches a skill's description, Copilot loads the SKILL.md body into context. Users can also trigger this by typing the skill name in chat.
3. **Resource access** -- As Copilot works through instructions, it accesses additional files in the skill directory only when referenced

This means you can install many skills without consuming context. Copilot loads only what is relevant. `[Official]`

### Skills vs Custom Instructions

| Aspect | Skills | Custom Instructions |
|--------|--------|---------------------|
| When loaded | On-demand, when prompt matches description | Always loaded (or path-matched via `applyTo`) |
| Format | `SKILL.md` in a directory | `.instructions.md` files |
| Complexity | Can include scripts, examples, resources | Plain Markdown only |
| Use case | Specialized tasks and workflows | Broad coding standards and guidelines |
| Location | `.github/skills/` or `~/.copilot/skills/` | `.github/instructions/` |

GitHub recommends using custom instructions for simple guidelines relevant to most tasks, and skills for detailed instructions Copilot should only access when relevant. `[Official]`

---

## Custom Agents

### Agents Overview

Custom agents are specialist personas defined by Markdown files with `.agent.md` extension. They specify prompts, allowed tools, and optional MCP servers. When Copilot determines an agent's expertise matches a task, it spins up a **subagent** with its own context window to handle the work. `[Official]`

Agents support **handoffs** -- after one agent finishes, a handoff button can appear to transition to the next agent with pre-filled context (e.g., Plan -> Implement -> Review). `[Official]`

### Agent File Format

Agent profiles are Markdown files with YAML frontmatter:

```yaml
---
name: Security Reviewer
description: Reviews code changes for security vulnerabilities, OWASP top 10 issues, and dependency risks.
tools:
  - read
  - search
  - web
---

## Instructions

When reviewing code for security issues, focus on:
1. Input validation and sanitization
2. Authentication and authorization checks
3. Sensitive data exposure
...
```

The filename (minus extension) becomes the agent ID. E.g., `security-reviewer.agent.md` has ID `security-reviewer`. `[Official]`

### Agent Frontmatter Fields

| Property | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `name` | string | No | filename | Display name for the agent |
| `description` | string | **Yes** | -- | Purpose and capabilities; used for automatic matching |
| `target` | string | No | -- | Target environment: `"vscode"` or `"github-copilot"` |
| `tools` | list or `"*"` | No | all tools | Which tools the agent can access |
| `model` | string | No | inherited | LLM model to use for this agent |
| `disable-model-invocation` | boolean | No | `false` | Prevents Copilot from automatically using this agent |
| `user-invocable` | boolean | No | `true` | Whether users can manually select this agent |
| `mcp-servers` | object | No | -- | Additional MCP servers for this agent |
| `metadata` | object | No | -- | Arbitrary annotation data |
| `infer` | boolean | No | -- | **Deprecated.** Use `disable-model-invocation` and `user-invocable` instead |

`[Official]`

**Maximum prompt content:** 30,000 characters. `[Official]`

### Tools Configuration

Tools control what the agent can do. `[Official]`

| Configuration | Meaning |
|--------------|---------|
| Omit `tools` property | All available tools enabled |
| `tools: ["*"]` | All available tools enabled (explicit) |
| `tools: []` | All tools disabled |
| `tools: [list]` | Only listed tools enabled |

#### Tool Aliases (Built-in)

| Alias | Capability |
|-------|-----------|
| `execute` | Shell command execution |
| `read` | File reading |
| `edit` | File editing |
| `search` | Code/file search |
| `agent` | Subagent spawning |
| `web` | Web browsing |
| `todo` | Task tracking |

MCP server tools can be referenced with namespaced syntax: `some-mcp-server/some-tool`. `[Official]`

#### Tool Processing Order for MCP

1. Out-of-the-box MCP configurations
2. Custom agent's `mcp-servers` configuration
3. Repository-level MCP settings

`[Official]`

### MCP Server Configuration

MCP servers are configured under the `mcp-servers` frontmatter property: `[Official]`

```yaml
---
name: Database Agent
description: Queries and manages database operations
tools:
  - read
  - execute
  - my-db-server/query
  - my-db-server/schema
mcp-servers:
  my-db-server:
    type: stdio
    command: npx
    args: ["-y", "@my-org/db-mcp-server"]
    tools:
      - query
      - schema
    env:
      DB_HOST: "${DB_HOST}"
      DB_PASSWORD: "${{ secrets.DB_PASSWORD }}"
---
```

#### MCP Server Properties

| Property | Type | Description |
|----------|------|-------------|
| `type` | string | Server type; `"stdio"` maps to local execution |
| `command` | string | Command to run the server |
| `args` | list | Command arguments |
| `tools` | list | Which tools from this server to expose |
| `env` | object | Environment variables for the server process |

#### Environment Variable Patterns

| Pattern | Source |
|---------|--------|
| `$VAR` or `${VAR}` | Process environment |
| `${{ secrets.VAR }}` | GitHub repository secrets |
| `${{ vars.VAR }}` | GitHub repository variables |

`[Official]`

### Agent Storage Locations

| Level | Path | Scope |
|-------|------|-------|
| Repository | `.github/agents/{name}.agent.md` | Project-specific |
| Organization | `.github-private/agents/{name}.agent.md` | Org-wide |
| Enterprise | `.github-private/agents/{name}.agent.md` | Enterprise-wide |
| User (CLI) | `~/.copilot/agents/{name}.agent.md` | Personal, all projects |

`[Official]`

### Subagent Execution

When Copilot uses a custom agent, it spawns a **subagent** -- a temporary agent with its own context window. This design: `[Official]`

- Keeps specialized work isolated from the main agent's context
- Allows the subagent to load domain-specific resources without cluttering the main context
- Enables the main agent to focus on high-level planning and coordination

The main agent can see the subagent's summary output but not its full context.

### Priority and Deduplication

When agents with the same filename exist at multiple levels: `[Official]`

1. **System-level** agents override repository-level
2. **Repository-level** agents override organization-level
3. **Organization-level** has lowest priority

The configuration file's name (minus `.md` or `.agent.md`) is used for deduplication. `[Official]`

---

## Copilot Extensions (Platform)

Copilot Extensions are a separate extensibility mechanism from skills and custom agents. They are **server-side integrations** that extend Copilot Chat in github.com and supported IDEs. `[Official]`

### Skillsets vs Agents

Extensions can be built as either **skillsets** or **agents**: `[Official]`

| Aspect | Skillsets | Extension Agents |
|--------|-----------|-----------------|
| Complexity | Lightweight, minimal setup | Full control, complex workflows |
| API endpoints | Up to 5 | Unlimited |
| AI handling | Copilot handles routing, prompts, response formatting | Developer manages all AI interactions |
| LLM control | Uses Copilot's LLM | Can integrate custom LLMs |
| Best for | Simple data retrieval, API calls | Sophisticated multi-step workflows |
| Maintenance | Low | High |

**These are distinct from Agent Skills (SKILL.md) and Custom Agents (.agent.md).** Extensions are server-side integrations requiring deployment infrastructure; skills and custom agents are local file-based configurations. `[Inferred]`

---

## Custom Instructions

Custom instructions are `.instructions.md` files in `.github/instructions/` that provide repository-wide or path-scoped guidance to Copilot. `[Official]`

```markdown
---
applyTo: "app/models/**/*.rb"
excludeAgent: "copilot-review"
---

## Model Conventions

- Use `validates` for all model validations
- Always include database indexes for foreign keys
...
```

| Property | Type | Description |
|----------|------|-------------|
| `applyTo` | glob | Path pattern for when this instruction applies |
| `excludeAgent` | string | Agent that should NOT receive this instruction |

All matching instruction files are **combined** (not priority-based fallback). `[Official]`

**Key difference from skills:** Instructions are always loaded (or path-matched). Skills are on-demand based on prompt relevance. `[Official]`

---

## Comparison with Claude Code

| Feature | Copilot CLI | Claude Code |
|---------|-------------|-------------|
| **Skills** | | |
| File name | `SKILL.md` | `SKILL.md` |
| Frontmatter fields | `name`, `description`, `license` | `name`, `description`, `tools`, `context`, `alwaysApply`, `autoApply`, etc. |
| Progressive loading | Yes (3 stages) | Yes (discovery -> full load) |
| Can restrict tools | No (agent-level only) | Yes (per-skill `tools` field) |
| Shared locations | `~/.copilot/skills/`, `~/.claude/skills/` | `~/.claude/skills/` |
| **Agents** | | |
| File extension | `.agent.md` | `AGENT.md` |
| Tool aliases | `execute`, `read`, `edit`, `search`, `agent`, `web`, `todo` | `Bash`, `Read`, `Write`, `Edit`, `Grep`, `Glob`, etc. |
| MCP in agent | Yes (`mcp-servers` frontmatter) | Yes (`mcp-servers` frontmatter) |
| Handoffs | Yes (chained workflows) | No (subagent delegation only) |
| Model selection | Yes (`model` field) | Yes (`model` field) |
| Max prompt size | 30,000 characters | Not documented limit |
| **Instructions** | | |
| File format | `.instructions.md` in `.github/instructions/` | `CLAUDE.md` / `.claude/rules/*.md` |
| Path scoping | `applyTo` glob | Directory-based hierarchy |
| Agent exclusion | `excludeAgent` property | N/A |

---

## Source Index

1. [About agent skills](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills) `[Official]`
2. [Creating agent skills](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/create-skills) `[Official]`
3. [Creating agent skills for CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-skills) `[Official]`
4. [Creating custom agents for CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-custom-agents-for-cli) `[Official]`
5. [Custom agents configuration reference](https://docs.github.com/en/copilot/reference/custom-agents-configuration) `[Official]`
6. [About custom agents](https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-custom-agents) `[Official]`
7. [About Copilot Extensions skillsets](https://docs.github.com/en/copilot/concepts/build-copilot-extensions/skillsets-for-copilot-extensions) `[Official]`
8. [Copilot Extensions agents](https://docs.github.com/en/copilot/concepts/extensions/agents) `[Official]`
9. [Agent Skills changelog](https://github.blog/changelog/2025-12-18-github-copilot-now-supports-agent-skills/) `[Official]`
10. [Agent-specific instructions changelog](https://github.blog/changelog/2025-11-12-copilot-code-review-and-coding-agent-now-support-agent-specific-instructions/) `[Official]`
11. [awesome-copilot community repo](https://github.com/github/awesome-copilot) `[Community]`
12. [What differentiates Custom Agents, Agent Skills, and Custom Instructions](https://github.com/orgs/community/discussions/183962) `[Community]`
13. [Copilot Agent Skills hands-on](https://visualstudiomagazine.com/articles/2026/01/11/hand-on-with-new-github-copilot-agent-skills-in-vs-code.aspx) `[Community]`
