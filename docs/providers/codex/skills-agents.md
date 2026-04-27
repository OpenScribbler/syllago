# Codex CLI: Skills & Agents

## Overview

Codex has two distinct customization systems for extending agent behavior:

1. **Skills** — Reusable workflow packages (instructions + scripts + references)
2. **Subagents** — Specialized agent configurations with their own model/sandbox/tools

These are separate concepts. Skills add capabilities to any agent; subagents define
entirely different agent personas that can be spawned in parallel.

---

## Skills

### What Is a Skill?

A skill is a directory containing a `SKILL.md` file plus optional supporting files.
Skills package instructions, resources, and scripts so Codex can follow a workflow
reliably. `[Official]`

Source: [Agent Skills](https://developers.openai.com/codex/skills)

### Directory Structure

```
my-skill/
├── SKILL.md              # Required: instructions + metadata
├── scripts/              # Optional: executable code
├── references/           # Optional: documentation
├── assets/               # Optional: templates, resources
└── agents/
    └── openai.yaml       # Optional: appearance, policy, dependencies
```

`[Official]` Source: [Agent Skills](https://developers.openai.com/codex/skills)

### SKILL.md Format

The file uses YAML frontmatter with markdown body:

```yaml
---
name: commit
description: >-
  Stage and commit changes in semantic groups.
  Use when the user wants to commit, organize commits,
  or clean up a branch before pushing.
---

## Instructions

1. Run `git status` to see current state.
2. Group related changes into semantic commits.
3. Write conventional commit messages.
```

**Required frontmatter fields:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill identifier, used for explicit invocation (`$skill-name`). |
| `description` | string | When this skill should trigger. Clear descriptions improve implicit invocation reliability. |

`[Official]` Source: [Agent Skills](https://developers.openai.com/codex/skills)

### agents/openai.yaml

Optional metadata file for appearance, policy, and dependencies:

```yaml
interface:
  display_name: "Optional user-facing name"
  short_description: "Optional user-facing description"
  icon_small: "./assets/small-logo.svg"
  icon_large: "./assets/large-logo.png"
  brand_color: "#3B82F6"
  default_prompt: "Optional surrounding prompt to use the skill with"

policy:
  allow_implicit_invocation: false

dependencies:
  tools:
    - type: "mcp"
      value: "openaiDeveloperDocs"
      description: "OpenAI Docs MCP server"
      transport: "streamable_http"
      url: "https://developers.openai.com/mcp"
```

**Key fields:**

| Section | Field | Default | Description |
|---------|-------|---------|-------------|
| `policy` | `allow_implicit_invocation` | `true` | When `false`, Codex won't auto-select the skill; explicit `$skill` invocation still works. |
| `interface` | `display_name` | — | User-facing name override. |
| `interface` | `default_prompt` | — | Surrounding prompt template for the skill. |
| `dependencies.tools` | `type`, `value`, `url` | — | MCP server dependencies the skill requires. |

`[Official]` Source: [Agent Skills](https://developers.openai.com/codex/skills)

### Discovery Locations

Skills are loaded from multiple scopes (checked in order):

| Scope | Path | Use Case |
|-------|------|----------|
| REPO (cwd) | `$CWD/.agents/skills` | Folder-specific skills |
| REPO (root) | `$REPO_ROOT/.agents/skills` | Organization/project-wide skills |
| USER | `$HOME/.agents/skills` | Personal cross-repo skills |
| ADMIN | `/etc/codex/skills` | System-level defaults |
| SYSTEM | Bundled with Codex | Built-in skills |

`[Official]` Source: [Agent Skills](https://developers.openai.com/codex/skills)

### Progressive Disclosure

Codex uses progressive disclosure to manage context efficiently:

1. **Discovery:** Loads only metadata (name, description, file path, `agents/openai.yaml`).
2. **Activation:** Loads full `SKILL.md` instructions only when the skill is selected.

This prevents bloating the context window with unused skill instructions.

`[Official]` Source: [Agent Skills](https://developers.openai.com/codex/skills)

### Invocation

| Method | How |
|--------|-----|
| **Explicit** | Type `$skill-name` in prompt, or use `/skills` command |
| **Implicit** | Codex auto-selects when task matches the skill's `description` |

Implicit invocation can be disabled per-skill via `allow_implicit_invocation: false`
in `agents/openai.yaml`.

### Disabling Skills in config.toml

```toml
[[skills.config]]
path = "/path/to/skill/SKILL.md"
enabled = false
```

`[Official]` Source: [Config Reference](https://developers.openai.com/codex/config-reference)

### Skills Catalog

OpenAI maintains a community skills catalog at [github.com/openai/skills](https://github.com/openai/skills)
with three tiers:

| Directory | Description |
|-----------|-------------|
| `.system` | Auto-installed with Codex |
| `.curated` | Community-reviewed, installable by name |
| `.experimental` | Development-stage, requires explicit install |

Install via `$skill-installer` within Codex. Restart Codex after installing to pick up
new skills.

`[Official]` Source: [github.com/openai/skills](https://github.com/openai/skills)

---

## Subagents (Multi-Agent)

### Overview

Codex can spawn specialized agents in parallel for complex tasks. Each subagent has its
own model, sandbox, instructions, and tool configuration. The feature is **stable and
enabled by default**.

`[Official]` Source: [Subagents](https://developers.openai.com/codex/subagents)

### Multi-Agent Feature Flag

```toml
[features]
multi_agent = true   # stable, on by default
```

Enables tools: `spawn_agent`, `send_input`, `resume_agent`, `wait_agent`, `close_agent`.

`[Official]` Source: [Config Reference](https://developers.openai.com/codex/config-reference)

### Global Agent Settings (config.toml)

```toml
[agents]
max_threads = 6                  # Max concurrent agent threads (default: 6)
max_depth = 1                    # Max nesting depth (root = 0, default: 1)
job_max_runtime_seconds = 1800   # Default timeout per worker (default: 1800)
```

`[Official]` Source: [Subagents](https://developers.openai.com/codex/subagents),
[Config Reference](https://developers.openai.com/codex/config-reference)

### Custom Agent TOML Files

Store custom agent definitions as standalone TOML files:

| Scope | Path |
|-------|------|
| Personal | `~/.codex/agents/<name>.toml` |
| Project | `.codex/agents/<name>.toml` |

Each file defines one agent. The `name` field is the identifier (filename is
convention, not authoritative).

#### All Supported Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Agent identifier for spawning. |
| `description` | string | Yes | When Codex should use this agent. |
| `developer_instructions` | string | Yes | Core behavioral instructions (system prompt). |
| `nickname_candidates` | string[] | No | Display name pool for UI labeling. ASCII letters, digits, spaces, hyphens, underscores. |
| `model` | string | No | Model override. Inherits from parent if omitted. |
| `model_reasoning_effort` | string | No | Reasoning level: `"low"`, `"medium"`, `"high"`. |
| `sandbox_mode` | string | No | `"read-only"`, `"workspace-write"`, `"danger-full-access"`. |
| `mcp_servers` | table | No | MCP server configurations for this agent. |
| `skills.config` | array | No | Skills configuration overrides. |

`[Official]` Source: [Subagents](https://developers.openai.com/codex/subagents)

#### Example: Read-Only Explorer

```toml
name = "pr_explorer"
description = "Read-only codebase explorer for gathering evidence before changes are proposed."
model = "gpt-5.3-codex-spark"
model_reasoning_effort = "medium"
sandbox_mode = "read-only"
developer_instructions = """
Stay in exploration mode.
Trace the real execution path, cite files and symbols, and avoid proposing fixes
unless the parent agent asks for them.
Prefer fast search and targeted file reads over broad scans.
"""
```

#### Example: Reviewer with High Reasoning

```toml
name = "reviewer"
description = "PR reviewer focused on correctness, security, and missing tests."
model = "gpt-5.4"
model_reasoning_effort = "high"
sandbox_mode = "read-only"
developer_instructions = """
Review code like an owner.
Prioritize correctness, security, behavior regressions, and missing test coverage.
Lead with concrete findings, include reproduction steps when possible, and avoid
style-only comments unless they hide a real bug.
"""
```

#### Example: Docs Researcher with MCP

```toml
name = "docs_researcher"
description = "Documentation specialist that uses the docs MCP server to verify APIs and framework behavior."
model = "gpt-5.4-mini"
model_reasoning_effort = "medium"
sandbox_mode = "read-only"
developer_instructions = """
Use the docs MCP server to confirm APIs, options, and version-specific behavior.
Return concise answers with links or exact references when available.
Do not make code changes.
"""

[mcp_servers.openaiDeveloperDocs]
url = "https://developers.openai.com/mcp"
```

`[Official]` Source: [Subagents](https://developers.openai.com/codex/subagents)

### Built-In Agents

| Name | Purpose |
|------|---------|
| `default` | General-purpose fallback |
| `worker` | Execution-focused implementation |
| `explorer` | Read-heavy exploration |

Custom agents with matching names override built-ins.

`[Official]` Source: [Subagents](https://developers.openai.com/codex/subagents)

### Spawning Behavior

- **Explicit only:** Codex only spawns subagents when you explicitly ask.
- **Sandbox inheritance:** Subagents inherit parent session's sandbox policy. Individual agents can override via their TOML `sandbox_mode`.
- **Runtime overrides:** Parent-turn live overrides (including `/approvals` changes or `--yolo`) reapply when spawning children.

### Batch Processing (Experimental)

The `spawn_agents_on_csv` tool processes structured batches:

| Parameter | Description |
|-----------|-------------|
| `csv_path` | Source CSV file |
| `instruction` | Worker prompt with `{column_name}` placeholders |
| `id_column` | Stable item identifiers |
| `output_schema` | Fixed JSON shape per worker |
| `output_csv_path` | Result export location |
| `max_concurrency` | Concurrent worker limit |
| `max_runtime_seconds` | Per-worker timeout |

Workers must call `report_agent_job_result` exactly once.

`[Official]` Source: [Subagents](https://developers.openai.com/codex/subagents)

### CLI Management

- `/agent` — Switch between active threads, inspect ongoing activity.
- Direct Codex to steer, stop, or close subagents interactively.

---

## AGENTS.md (Project Instructions)

Separate from skills and agents, `AGENTS.md` provides persistent project-level
instructions. This is Codex's equivalent of Claude Code's `CLAUDE.md`.

### Discovery Order

1. `~/.codex/AGENTS.override.md` (global override)
2. `~/.codex/AGENTS.md` (global default)
3. Walk from repo root to `$CWD`, checking each directory for:
   - `AGENTS.override.md`
   - `AGENTS.md`
   - Fallback filenames from `project_doc_fallback_filenames`

Files concatenate root-downward; closer files take precedence.

### Config Keys

```toml
project_doc_fallback_filenames = ["TEAM_GUIDE.md"]  # Alternative filenames
project_doc_max_bytes = 65536                         # Size limit (default: 64 KiB)
```

`[Official]` Source: [AGENTS.md Guide](https://developers.openai.com/codex/guides/agents-md)

---

## Model Selection

### CLI Flag

```bash
codex -m gpt-5.4              # Override model for this session
codex --oss                    # Use local open-source models via Ollama
```

### config.toml

```toml
model = "gpt-5-codex"
model_reasoning_effort = "high"   # "low", "medium", "high"
model_provider = "openai"         # Default provider
```

### Per-Agent Override

Each custom agent TOML can specify its own `model` and `model_reasoning_effort`,
independent of the parent session.

`[Official]` Source: [Config Reference](https://developers.openai.com/codex/config-reference),
[CLI Reference](https://developers.openai.com/codex/cli/reference)

---

## Tool Permissions & Sandbox

Codex enforces safety at the **kernel layer** (Seatbelt on macOS, Landlock/seccomp on
Linux), not at the application layer like Claude Code.

### Sandbox Modes

| Mode | Description |
|------|-------------|
| `read-only` | No file modifications |
| `workspace-write` | Writes allowed in project directories |
| `danger-full-access` | Unrestricted access |

### Permission Profiles

Named profiles in `config.toml` define filesystem and network access:

```toml
[permissions.restricted]
# Filesystem
paths = { "/home/user/project" = "write", "/etc" = "read", "/tmp" = "none" }

# Network
allowed_domains = ["api.openai.com"]
denied_domains = ["*.example.com"]
```

### Approval Policy

```toml
approval_policy = "on-request"   # "untrusted", "on-request", "never"
```

Granular sub-keys: `sandbox_approval`, `rules`, `mcp_elicitations`,
`request_permissions`, `skill_approval`.

`[Official]` Source: [Config Reference](https://developers.openai.com/codex/config-reference),
[CLI Reference](https://developers.openai.com/codex/cli/reference)

---

## Syllago Implications

- **Skills** map loosely to Claude Code's skills — both are directory-based with a
  markdown definition file, but the metadata format differs (YAML frontmatter vs.
  Claude's format).
- **Subagents** have no direct Claude Code equivalent. Claude Code uses `Agent` tool
  delegation, not standalone agent config files.
- **AGENTS.md** is analogous to `CLAUDE.md` — both provide project-level instructions
  with hierarchical discovery.
- Skills use `.agents/skills/` paths; agents use `.codex/agents/` paths.
- The `agents/openai.yaml` file in skills is Codex-specific metadata with no Claude
  Code parallel.
