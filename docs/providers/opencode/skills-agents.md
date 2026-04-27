# OpenCode Skills & Agents

## Agents

### Overview

Agents are specialized AI assistants with custom prompts, models, and tool access. OpenCode has two categories: **primary agents** (direct user interaction, cycled via Tab key) and **subagents** (invoked by primary agents for delegated tasks). [Official]

Source: https://opencode.ai/docs/agents/

### Built-in Agents

#### Primary Agents
| Agent        | Purpose                                    | Tool Access        | Hidden |
|--------------|--------------------------------------------|--------------------|--------|
| `build`      | Default agent for development work         | All tools enabled  | No     |
| `plan`       | Analysis and code exploration              | Edits/bash = "ask" | No     |

#### Subagents
| Agent        | Purpose                                    | Tool Access         | Hidden |
|--------------|--------------------------------------------|---------------------|--------|
| `general`    | Multi-step research and complex tasks      | All except todo     | No     |
| `explore`    | Read-only codebase exploration             | Read-only tools     | No     |

#### System Agents (Hidden)
| Agent        | Purpose                                    |
|--------------|--------------------------------------------|
| `compaction` | Automatically compacts long context        |
| `title`      | Generates session titles                   |
| `summary`    | Creates session summaries                  |

[Official] Source: https://opencode.ai/docs/agents/

### Agent Configuration

#### Methods
1. **JSON config** (`opencode.json`): Define under the `"agent"` key
2. **Markdown files**: Place in `~/.config/opencode/agents/` (global) or `.opencode/agents/` (project). Filename = agent ID.
3. **CLI**: `opencode agent create` interactive wizard

#### Configuration Options

| Option        | Type          | Purpose                                      |
|---------------|---------------|----------------------------------------------|
| `description` | string        | Brief explanation (required)                 |
| `mode`        | string        | `"primary"`, `"subagent"`, or `"all"`        |
| `model`       | string        | Override model (`provider/model-id`)         |
| `prompt`      | string        | Custom system prompt (file path or inline)   |
| `temperature` | number        | Randomness control (0.0-1.0)                 |
| `top_p`       | number        | Alternative randomness control               |
| `steps`       | number        | Max agentic iterations before text-only      |
| `disable`     | boolean       | Disable the agent                            |
| `hidden`      | boolean       | Hide from `@` autocomplete                   |
| `color`       | string        | Visual appearance (hex or theme color)       |
| `permission`  | object        | Per-tool permission overrides                |
| `tools`       | object        | Enable/disable specific tools                |

[Official] Source: https://opencode.ai/docs/agents/

### Markdown Agent Format

```yaml
---
description: Security-focused code reviewer
mode: primary
model: anthropic/claude-sonnet-4-5
temperature: 0.3
permission:
  edit: deny
  bash:
    "*": ask
    "git log *": allow
---

You are a security-focused code reviewer. Analyze code for
vulnerabilities, suggest fixes, but never modify files directly.
```

The YAML frontmatter contains configuration; the body is the system prompt. [Official]

### Model Selection

- Primary agents: use the globally configured `model` unless overridden
- Subagents: use the model of the invoking primary agent unless overridden
- Format: `provider/model-id` (e.g., `anthropic/claude-sonnet-4-5`, `openai/gpt-4o`)

[Official]

### Agent Invocation

- **Primary agents**: Cycle via Tab key or configured `switch_agent` keybind
- **Subagents**: Mention with `@` syntax (e.g., `@general search for auth logic`) or automatic invocation by primary agents via the `task` tool
- **Navigation**: `session_child_first`, `session_child_cycle`, `session_parent` keybinds

[Official]

### Default Agent

Set via `default_agent` in config. Must be a primary agent. Falls back to `build` if invalid. [Official]

### Permission Configuration Per Agent

```json
{
  "agent": {
    "build": {
      "permission": {
        "bash": {
          "*": "ask",
          "git push *": "deny"
        },
        "edit": "allow"
      }
    }
  }
}
```

Or in markdown frontmatter:

```yaml
---
permission:
  edit: deny
  bash: allow
  task:
    "general": allow
    "explore": allow
---
```

Task permissions control which subagents a primary agent can invoke, using glob patterns. [Official]

### Tool Access Control

Tools can be enabled/disabled per agent:

```json
{
  "agent": {
    "my-agent": {
      "tools": {
        "skill": false,
        "my-mcp*": true
      }
    }
  }
}
```

Glob patterns supported (e.g., `mymcp_*`). [Official]

## Skills

### Overview

Skills are reusable instruction sets that agents discover from the repository or home directory. They function as on-demand tools -- agents see available skills and load full content when needed via the `skill` tool. [Official]

Source: https://opencode.ai/docs/skills/

### File Structure

Skills are organized as directories containing a `SKILL.md` file:

```
.opencode/skills/
  git-release/
    SKILL.md
  deploy-checklist/
    SKILL.md
```

### Skill Locations (searched in order)

| Location                                  | Scope          |
|-------------------------------------------|----------------|
| `.opencode/skills/<name>/SKILL.md`        | Project        |
| `~/.config/opencode/skills/<name>/SKILL.md` | Global       |
| `.claude/skills/<name>/SKILL.md`          | Compatibility  |
| `.agents/skills/<name>/SKILL.md`          | Compatibility  |

OpenCode walks up from the working directory to the git worktree root, loading all matching skills. [Official]

### SKILL.md Format

```yaml
---
name: git-release
description: Create consistent releases and changelogs
license: MIT
compatibility: opencode
metadata:
  audience: maintainers
  workflow: github
---

## What I do

I help create consistent release tags and changelogs following
semantic versioning conventions.

## Steps

1. Review recent commits since last tag
2. Determine version bump (major/minor/patch)
3. Generate changelog entry
4. Create and push git tag
```

### Frontmatter Fields

| Field           | Required | Type              | Constraints                                     |
|-----------------|----------|-------------------|-------------------------------------------------|
| `name`          | Yes      | string            | 1-64 chars, lowercase alphanumeric + hyphens, pattern: `^[a-z0-9]+(-[a-z0-9]+)*$` |
| `description`   | Yes      | string            | 1-1024 chars                                    |
| `license`       | No       | string            | License identifier                              |
| `compatibility` | No       | string            | Tool compatibility hint                         |
| `metadata`      | No       | map[string]string | Key-value pairs                                 |

Unknown fields are ignored. [Official]

### Skill Permissions

Control access via `opencode.json`:

```json
{
  "permission": {
    "skill": {
      "*": "allow",
      "internal-*": "deny",
      "experimental-*": "ask"
    }
  }
}
```

Permission levels: `allow` (immediate), `deny` (hidden), `ask` (user approval). [Official]

### Disabling Skills for an Agent

- Custom agents: `tools: { skill: false }` in frontmatter
- Built-in agents: configure in `opencode.json` agent settings

[Official]

## Syllago Mapping

### Agents
| Syllago Concept    | OpenCode Equivalent           | Notes                              |
|--------------------|-------------------------------|------------------------------------|
| Agent definitions  | Agent markdown/JSON           | Similar concept, different format  |
| Model selection    | `model` field per agent       | Format: `provider/model-id`       |
| Tool permissions   | `permission` + `tools` fields | Glob-pattern-based                 |
| System prompt      | Markdown body or `prompt` ref | Inline or file reference           |

### Skills
| Syllago Concept | OpenCode Equivalent | Notes                                |
|-----------------|---------------------|--------------------------------------|
| Skills          | SKILL.md files      | Very similar concept                 |
| Skill format    | YAML frontmatter + MD body | Similar to Claude Code skills   |
| Skill location  | `.opencode/skills/` | Also reads `.claude/skills/`         |
| Skill naming    | Lowercase + hyphens | Strict pattern enforcement           |

### Key Differences from Claude Code
- OpenCode agents are a first-class concept (Claude Code has no equivalent)
- OpenCode skills are nearly identical to Claude Code skills in format
- OpenCode has explicit primary/subagent distinction
- Agent model override is per-agent, not global-only
- Tool permissions use glob patterns extensively
- `task` permission controls subagent access granularly
