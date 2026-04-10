# Factory Droid Skills and Custom Droids

Factory Droid (Factory AI's coding agent CLI, `factory-droid`) provides two
mechanisms for extending agent behavior: Skills (reusable prompt-based
capabilities) and Custom Droids (specialized subagents with their own model,
tools, and context).

**Identity note:** "Factory Droid" refers to Factory AI's coding agent CLI
(`factory-droid`). No known naming conflicts with other tools as of 2026-03-30.

## Custom Droids

Custom Droids are specialized subagents defined as Markdown files. Each Droid
operates with its own system prompt, model preference, and tool access
restrictions. They enable delegation of focused tasks without repeating
instructions.

### Storage

| Scope | Path | Behavior |
|---|---|---|
| Project | `.factory/droids/<name>.md` | Shared with teammates via version control |
| Personal | `~/.factory/droids/<name>.md` | Follows you across workspaces |

Project definitions override personal ones when names match.

[Official: https://docs.factory.ai/cli/configuration/custom-droids]

### File Format

Markdown with YAML frontmatter:

```markdown
---
name: code-reviewer
description: Reviews code changes for correctness and style
model: inherit
tools:
  - read-only
  - Grep
reasoningEffort: high
---

# Code Reviewer

## Instructions

1. Read the changed files
2. Check for correctness issues
3. Verify style compliance
4. Report findings with file paths and line numbers
```

### Frontmatter Fields

| Field | Required | Default | Description |
|---|---|---|---|
| `name` | Yes | -- | Lowercase letters, digits, hyphens, underscores |
| `description` | No | -- | Max 500 characters |
| `model` | No | `"inherit"` | `"inherit"` for parent session's model, or a specific model ID |
| `tools` | No | All tools | Array of tool names or category strings |
| `reasoningEffort` | No | -- | For compatible models (e.g., `"high"`, `"low"`) |

[Official: https://docs.factory.ai/cli/configuration/custom-droids]

### Tool Categories

Tools can be specified individually (e.g., `["Read", "Edit"]`) or by category:

| Category | Tools included |
|---|---|
| `read-only` | Read, LS, Grep, Glob |
| `edit` | Create, Edit, ApplyPatch |
| `execute` | Execute |
| `web` | WebSearch, FetchUrl |
| `mcp` | Dynamically populated from configured MCP servers |

[Official: https://docs.factory.ai/cli/configuration/custom-droids]

### Model Selection

The `model` field controls which LLM the Droid uses:

- `"inherit"` -- uses the parent session's model (default)
- Specific model identifier -- uses that model directly

Factory Droid supports mixed models where different models handle planning vs.
execution phases.

[Official: https://docs.factory.ai/cli/configuration/mixed-models] (referenced
in llms.txt; not fetched directly) [Unverified -- specific model IDs and
selection mechanics not confirmed from docs]

### Invocation

Droids are invoked through the `Task` tool or by asking the agent directly
(e.g., "Use subagent `code-reviewer`"). The CLI scans Droid directories and
exposes them as `subagent_type` targets for the Task tool.

Changes to Droid files are picked up on the next menu open or task invocation.

[Official: https://docs.factory.ai/cli/configuration/custom-droids]

## Skills

Skills are reusable capabilities that extend what the Droid can do. They serve
as focused workflows invoked via slash commands or automatic agent detection.

### Storage

| Scope | Path |
|---|---|
| Workspace | `.factory/skills/<skill-name>/SKILL.md` |
| Personal | `~/.factory/skills/<skill-name>/SKILL.md` |
| Legacy | `.agent/skills/<skill-name>/SKILL.md` |

Monorepos can use centralized `.factory/skills/` or distribute skill folders
alongside subprojects.

[Official: https://docs.factory.ai/cli/configuration/skills]

### File Format

Markdown with YAML frontmatter:

```markdown
---
name: data-analyst
description: Analyzes datasets and generates reports with visualizations
---

# Data Analyst

## Instructions

1. Load the dataset from the specified path
2. Identify data types and missing values
3. Generate summary statistics
4. Create visualizations for key metrics
```

Skills can include supporting files in the same directory: `references.md`,
`schemas/`, `checklists.md`, etc.

### Frontmatter Fields

| Field | Required | Default | Description |
|---|---|---|---|
| `name` | No | Directory name | Lowercase, alphanumeric, hyphens |
| `description` | Recommended | -- | Guides agent invocation decisions |
| `user-invocable` | No | `true` | `false` = agent-only, not shown as slash command |
| `disable-model-invocation` | No | `false` | `true` = user-only via `/skill-name` |

[Official: https://docs.factory.ai/cli/configuration/skills]

### Invocation Control

| Mode | `user-invocable` | `disable-model-invocation` | Behavior |
|---|---|---|---|
| Default | `true` | `false` | Both users and agent can invoke |
| User-only | `true` | `true` | Only via `/skill-name` (e.g., `/deploy`) |
| Agent-only | `false` | `false` | Background knowledge, not a slash command |

[Official: https://docs.factory.ai/cli/configuration/skills]

### Skills vs. Droids vs. Commands

| Feature | Skills | Custom Droids | Commands |
|---|---|---|---|
| Scope | Focused workflow | Full subagent config | One-shot prompt/script |
| Own model | No | Yes | No |
| Own tool restrictions | No | Yes | Via `allowed-tools` frontmatter |
| Context isolation | No | Yes (fresh context window) | No |
| Supporting files | Yes | No [Unverified] | No |
| Invocation | `/skill-name` or auto | Task tool | `/command-name` |

## Droid Shield

Factory AI provides Droid Shield, an automatic secret detection system for git
commits. It prevents accidental credential leaks.

[Official: https://docs.factory.ai/security/droid-shield] (URL returned 404;
referenced in llms.txt index)

**Droid Shield Plus** adds enterprise-grade prompt injection detection and data
protection.

[Unverified -- specific configuration and behavior not confirmed from docs]

## Sources

- [Factory Droid Custom Droids](https://docs.factory.ai/cli/configuration/custom-droids) [Official]
- [Factory Droid Skills](https://docs.factory.ai/cli/configuration/skills) [Official]
- [Factory Droid llms.txt](https://docs.factory.ai/llms.txt) [Official]
