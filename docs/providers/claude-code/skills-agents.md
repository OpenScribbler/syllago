# Claude Code: Skills and Agents Reference

Comprehensive reference for Claude Code's skill (`SKILL.md`) and agent/subagent (`AGENT.md`) metadata systems.

**Last updated:** 2026-03-20

---

## Table of Contents

- [Skills](#skills)
  - [File Format](#skill-file-format)
  - [Frontmatter Fields](#skill-frontmatter-fields)
  - [String Substitutions](#string-substitutions)
  - [Tool Permission Syntax](#tool-permission-syntax)
  - [Invocation](#skill-invocation)
  - [Invocation Control Matrix](#invocation-control-matrix)
  - [Context Field and Subagent Execution](#context-field-and-subagent-execution)
  - [Hooks in Skills](#hooks-in-skills)
  - [Directory Structure](#skill-directory-structure)
  - [Progressive Disclosure](#progressive-disclosure)
  - [Dynamic Context Injection](#dynamic-context-injection)
  - [Storage Locations](#skill-storage-locations)
  - [Skill Discovery Budget](#skill-discovery-budget)
  - [Permission Rules for Skills](#permission-rules-for-skills)
- [Agents (Subagents)](#agents-subagents)
  - [File Format](#agent-file-format)
  - [Frontmatter Fields](#agent-frontmatter-fields)
  - [Model Field Values](#model-field-values)
  - [Permission Modes](#permission-modes)
  - [Tool Access Control](#tool-access-control)
  - [MCP Server Configuration](#mcp-server-configuration)
  - [Persistent Memory](#persistent-memory)
  - [Hooks in Agents](#hooks-in-agents)
  - [Preloaded Skills](#preloaded-skills)
  - [Storage Locations](#agent-storage-locations)
  - [Built-in Subagents](#built-in-subagents)
  - [Agent Invocation Methods](#agent-invocation-methods)
  - [Foreground vs Background Execution](#foreground-vs-background-execution)
  - [Agent Capabilities and Limitations](#agent-capabilities-and-limitations)
- [Agent Skills Open Standard](#agent-skills-open-standard)
- [Source Index](#source-index)

---

## Skills

### Skill File Format

Every skill is a directory containing a `SKILL.md` file. The file has two parts:

1. **YAML frontmatter** between `---` markers -- metadata and configuration
2. **Markdown body** -- instructions Claude follows when the skill is invoked

```yaml
---
name: my-skill
description: What this skill does and when to use it
---

Your skill instructions here in markdown...
```

`[Official]` Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Skill Frontmatter Fields

All fields are optional. Only `description` is recommended so Claude knows when to use the skill. `[Official]`

| Field | Required | Type | Description | Default |
|:------|:---------|:-----|:------------|:--------|
| `name` | No | `string` | Display name for the skill. Becomes the `/slash-command`. If omitted, uses the directory name. | Directory name |
| `description` | Recommended | `string` | What the skill does and when to use it. Claude uses this for automatic discovery. If omitted, uses the first paragraph of markdown content. | First paragraph |
| `argument-hint` | No | `string` | Hint shown during autocomplete to indicate expected arguments. | (none) |
| `disable-model-invocation` | No | `boolean` | Set to `true` to prevent Claude from automatically loading this skill. User-only via `/name`. | `false` |
| `user-invocable` | No | `boolean` | Set to `false` to hide from the `/` menu. Claude-only background knowledge. | `true` |
| `allowed-tools` | No | `string` | Comma-separated list of tools Claude can use without asking permission when this skill is active. | (inherits all) |
| `model` | No | `string` | Model to use when this skill is active. | (inherits session) |
| `effort` | No | `string` | Effort level override. Values: `low`, `medium`, `high`, `max` (Opus 4.6 only). | (inherits session) |
| `context` | No | `string` | Set to `fork` to run in a forked subagent context. | (runs inline) |
| `agent` | No | `string` | Which subagent type to use when `context: fork` is set. Built-in options: `Explore`, `Plan`, `general-purpose`. Also accepts any custom subagent name from `.claude/agents/`. | `general-purpose` |
| `hooks` | No | `object` | Hooks scoped to this skill's lifecycle. Same format as settings.json hooks. | (none) |

`[Official]` Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

#### Name Field Constraints

| Constraint | Rule |
|:-----------|:-----|
| Maximum length | 64 characters |
| Allowed characters | Lowercase letters (`a-z`), numbers (`0-9`), hyphens (`-`) |
| Cannot start/end with | Hyphen (`-`) |
| Cannot contain | Consecutive hyphens (`--`), XML tags |
| Reserved words | Cannot contain `anthropic` or `claude` |
| Must match | Parent directory name (per Agent Skills spec) |

`[Official]` Sources: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills), [agentskills.io/specification](https://agentskills.io/specification)

#### Description Field Constraints

| Constraint | Rule |
|:-----------|:-----|
| Maximum length | 1024 characters |
| Minimum length | Non-empty (1+ characters) |
| Cannot contain | XML tags |
| Best practice | Write in third person; include both what it does AND when to use it |

`[Official]` Sources: [platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices), [agentskills.io/specification](https://agentskills.io/specification)

#### Effort Field Values

| Value | Description |
|:------|:------------|
| `low` | Minimal reasoning effort |
| `medium` | Balanced reasoning effort |
| `high` | Thorough reasoning effort |
| `max` | Maximum reasoning effort (Opus 4.6 only) |

`[Official]` Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### String Substitutions

Skills support dynamic string substitution in the markdown body. These are replaced before the content is sent to Claude. `[Official]`

| Variable | Description |
|:---------|:------------|
| `$ARGUMENTS` | All arguments passed when invoking the skill. If not present in content, arguments are appended as `ARGUMENTS: <value>`. |
| `$ARGUMENTS[N]` | Access a specific argument by 0-based index (e.g., `$ARGUMENTS[0]`). |
| `$N` | Shorthand for `$ARGUMENTS[N]` (e.g., `$0`, `$1`, `$2`). |
| `${CLAUDE_SESSION_ID}` | The current session ID. |
| `${CLAUDE_SKILL_DIR}` | The directory containing the skill's `SKILL.md` file. For plugin skills, this is the skill's subdirectory within the plugin, not the plugin root. |

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Tool Permission Syntax

The `allowed-tools` field accepts a comma-separated (in Claude Code skills) or space-delimited (in the Agent Skills spec) list of tool names. `[Official]`

**Claude Code tools available** (non-exhaustive):

| Tool | Description |
|:-----|:------------|
| `Read` | Read files |
| `Write` | Write files |
| `Edit` | Edit files |
| `MultiEdit` | Multi-file edit |
| `Grep` | Search file contents |
| `Glob` | Search file names |
| `Bash` | Execute shell commands |
| `Bash(pattern)` | Restricted Bash (e.g., `Bash(git:*)`, `Bash(npm run lint)`) |
| `NotebookEdit` | Edit Jupyter notebooks |
| `Agent` | Spawn subagents |
| `Agent(name)` | Spawn specific subagent types |
| `Skill` | Invoke skills |
| `Skill(name)` | Invoke specific skills |

Glob-style patterns are supported for Bash restrictions: `Bash(git:*)` allows any git command. `[Official]`

**Agent Skills open standard syntax:** Space-delimited, with glob patterns:
```yaml
allowed-tools: Bash(git:*) Bash(jq:*) Read
```

`[Official]` Source: [agentskills.io/specification](https://agentskills.io/specification)

**Claude Code syntax:** Comma-separated:
```yaml
allowed-tools: Read, Grep, Glob
```

`[Official]` Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Skill Invocation

Skills can be invoked in two ways: `[Official]`

1. **User invocation** -- Type `/skill-name` in Claude Code (optionally with arguments: `/fix-issue 123`)
2. **Automatic invocation** -- Claude loads the skill automatically when the conversation matches the description

How Claude discovers skills:
- At startup, skill metadata (name + description) is loaded into the system prompt
- This is lightweight: many skills can be installed without context penalty
- When triggered, Claude uses Bash to read SKILL.md from the filesystem, bringing full instructions into context
- Full skill content only loads when invoked; descriptions are always in context

**Skill-to-skill invocation:** Skills cannot directly invoke other skills during execution. However, a skill running with `context: fork` in a subagent could potentially trigger skill discovery through its instructions. Subagents themselves cannot spawn other subagents. `[Official]`

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Invocation Control Matrix

| Frontmatter | User can invoke | Claude can invoke | Context loading behavior |
|:------------|:----------------|:------------------|:-------------------------|
| (defaults) | Yes | Yes | Description always in context; full skill loads when invoked |
| `disable-model-invocation: true` | Yes | No | Description NOT in context; full skill loads when user invokes |
| `user-invocable: false` | No | Yes | Description always in context; full skill loads when invoked |

**Key distinction:** `user-invocable` only controls menu visibility, not Skill tool access. Use `disable-model-invocation: true` to block programmatic invocation. `[Official]`

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Context Field and Subagent Execution

Setting `context: fork` runs the skill in an isolated subagent context. The skill content becomes the prompt that drives the subagent. The subagent does NOT have access to your conversation history. `[Official]`

```yaml
---
name: deep-research
description: Research a topic thoroughly
context: fork
agent: Explore
---

Research $ARGUMENTS thoroughly...
```

The `agent` field specifies which subagent configuration to use:

| Value | Description |
|:------|:------------|
| `Explore` | Built-in read-only agent, uses Haiku model, optimized for codebase search |
| `Plan` | Built-in research agent for planning, inherits parent model |
| `general-purpose` | Built-in capable agent with all tools, inherits parent model |
| `<custom-name>` | Any custom subagent from `.claude/agents/` |
| (omitted) | Defaults to `general-purpose` |

**How `context: fork` interacts with subagent `skills` field:**

| Approach | System prompt | Task | Also loads |
|:---------|:-------------|:-----|:-----------|
| Skill with `context: fork` | From agent type (Explore, Plan, etc.) | SKILL.md content | CLAUDE.md |
| Subagent with `skills` field | Subagent's markdown body | Claude's delegation message | Preloaded skills + CLAUDE.md |

`[Official]` Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Hooks in Skills

Skills can define lifecycle hooks in their frontmatter. These hooks only run while the skill is active. `[Official]`

```yaml
---
name: secure-ops
description: Operations with security checks
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/security-check.sh"
          timeout: 30
          statusMessage: "Validating command..."
  PostToolUse:
    - matcher: "Write|Edit"
      hooks:
        - type: command
          command: "./scripts/lint-check.sh"
          once: true
---
```

**Hook event types** (all supported in skills):

| Event | Matcher input | When it fires |
|:------|:-------------|:--------------|
| `PreToolUse` | Tool name | Before a tool is used |
| `PostToolUse` | Tool name | After a tool is used |
| `Stop` | (none) | When the skill execution completes |
| All other events | Various | See hooks documentation |

**Hook handler types:**

| Type | Description |
|:-----|:------------|
| `command` | Run a shell command. Hook input passed as JSON via stdin. |
| `http` | Make an HTTP request. |
| `prompt` | Send a prompt to a model. |
| `agent` | Spawn an agent to evaluate. |

**Common handler fields:**

| Field | Type | Description |
|:------|:-----|:------------|
| `type` | `string` | Required. One of `command`, `http`, `prompt`, `agent`. |
| `timeout` | `number` | Seconds before canceling. Defaults: 600 (command), 30 (prompt), 60 (agent). |
| `statusMessage` | `string` | Custom spinner message while hook runs. |
| `once` | `boolean` | Skills only: if `true`, runs once per session then is removed. |

**Exit code behavior (command hooks):**
- Exit 0: Hook passes, execution continues
- Exit 2: Hook blocks the operation and returns error to Claude
- Other non-zero: Hook error, behavior depends on event type

`[Official]` Source: [code.claude.com/docs/en/hooks](https://code.claude.com/docs/en/hooks)

### Skill Directory Structure

```
my-skill/
├── SKILL.md           # Required: main instructions and frontmatter
├── template.md        # Optional: template for Claude to fill in
├── examples/
│   └── sample.md      # Optional: example output
├── scripts/
│   └── validate.sh    # Optional: executable scripts
├── references/
│   └── api-docs.md    # Optional: reference documentation
└── assets/
    └── schema.json    # Optional: templates, data files, resources
```

**Key points:** `[Official]`
- Only `SKILL.md` is required
- Reference files from `SKILL.md` so Claude knows what they contain and when to load them
- Keep `SKILL.md` under 500 lines; move detailed material to separate files
- Keep file references one level deep from SKILL.md (avoid nested reference chains)

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Progressive Disclosure

Skills use a 3-level progressive disclosure system: `[Official]`

| Level | Content | When loaded | Token cost |
|:------|:--------|:------------|:-----------|
| 1. Metadata | `name` + `description` (~100 tokens) | Always at startup | Minimal |
| 2. Instructions | Full `SKILL.md` body (<500 lines recommended) | When skill triggers | Moderate |
| 3. Resources | Files in `scripts/`, `references/`, `assets/` | On-demand when needed | Only when accessed |

**How it works:**
- At startup, only metadata from all skills is loaded into the system prompt
- When a skill is triggered, Claude reads `SKILL.md` via the filesystem
- Supporting files are read only when the skill instructions reference them
- Scripts can be executed without loading their source into context (only output consumes tokens)

Source: [platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices)

### Dynamic Context Injection

The `` !`<command>` `` syntax runs shell commands before skill content is sent to Claude. The command output replaces the placeholder. `[Official]`

```yaml
---
name: pr-summary
description: Summarize a pull request
context: fork
agent: Explore
---

## Pull request context
- PR diff: !`gh pr diff`
- PR comments: !`gh pr view --comments`
- Changed files: !`gh pr diff --name-only`
```

This is preprocessing -- Claude only sees the final rendered output, not the commands.

**Extended thinking:** Include the word `ultrathink` anywhere in skill content to enable extended thinking. `[Official]`

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Skill Storage Locations

| Location | Path | Scope | Priority |
|:---------|:-----|:------|:---------|
| Enterprise | Via managed settings | All users in organization | Highest |
| Personal | `~/.claude/skills/<skill-name>/SKILL.md` | All your projects | High |
| Project | `.claude/skills/<skill-name>/SKILL.md` | This project only | Medium |
| Plugin | `<plugin>/skills/<skill-name>/SKILL.md` | Where plugin is enabled | Lowest |

When skills share the same name, higher-priority locations win. Plugin skills use a `plugin-name:skill-name` namespace, so they cannot conflict with other levels. `[Official]`

**Legacy commands:** Files in `.claude/commands/` still work with the same frontmatter. If a skill and a command share the same name, the skill takes precedence. `[Official]`

**Automatic discovery:** When working with files in subdirectories, Claude Code discovers skills from nested `.claude/skills/` directories (supports monorepos). Skills from `--add-dir` directories are also loaded and support live change detection. `[Official]`

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Skill Discovery Budget

Skill descriptions are loaded into context so Claude knows what's available. If you have many skills, they may exceed the character budget. `[Official]`

- Budget scales dynamically at **2% of the context window**
- Fallback budget: **16,000 characters**
- Override via `SLASH_COMMAND_TOOL_CHAR_BUDGET` environment variable
- Run `/context` to check for warnings about excluded skills

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

### Permission Rules for Skills

Control which skills Claude can invoke via permission settings: `[Official]`

```
# Allow specific skills
Skill(commit)
Skill(review-pr *)

# Deny specific skills
Skill(deploy *)
```

Syntax: `Skill(name)` for exact match, `Skill(name *)` for prefix match with any arguments.

**Disable all skills:** Add `Skill` to deny rules in `/permissions`.

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

---

## Agents (Subagents)

### Agent File Format

Subagents are Markdown files (`.md`) with YAML frontmatter. The frontmatter defines configuration; the body becomes the system prompt. `[Official]`

```yaml
---
name: code-reviewer
description: Reviews code for quality and best practices
tools: Read, Glob, Grep
model: sonnet
---

You are a code reviewer. When invoked, analyze the code and provide
specific, actionable feedback on quality, security, and best practices.
```

Subagents receive only their own system prompt (plus basic environment details like working directory), NOT the full Claude Code system prompt. CLAUDE.md files and project memory still load through normal message flow. `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Agent Frontmatter Fields

Only `name` and `description` are required. `[Official]`

| Field | Required | Type | Description | Default |
|:------|:---------|:-----|:------------|:--------|
| `name` | Yes | `string` | Unique identifier. Lowercase letters and hyphens. | (required) |
| `description` | Yes | `string` | When Claude should delegate to this subagent. Used for automatic routing. | (required) |
| `tools` | No | `string` | Comma-separated allowlist of tools the subagent can use. | Inherits all tools |
| `disallowedTools` | No | `string` | Comma-separated denylist of tools to remove from inherited set. | (none) |
| `model` | No | `string` | Model to use. See [Model Field Values](#model-field-values). | `inherit` |
| `permissionMode` | No | `string` | Permission behavior. See [Permission Modes](#permission-modes). | `default` |
| `maxTurns` | No | `number` | Maximum agentic turns before the subagent stops. | (unlimited) |
| `skills` | No | `list` | Skills to preload into the subagent's context at startup. Full content is injected, not just made available. | (none) |
| `mcpServers` | No | `list` | MCP servers available to this subagent. See [MCP Server Configuration](#mcp-server-configuration). | (inherits from session) |
| `hooks` | No | `object` | Lifecycle hooks scoped to this subagent. Same format as settings.json hooks. | (none) |
| `memory` | No | `string` | Persistent memory scope: `user`, `project`, or `local`. | (disabled) |
| `background` | No | `boolean` | Set to `true` to always run as a background task. | `false` |
| `effort` | No | `string` | Effort level override: `low`, `medium`, `high`, `max` (Opus 4.6 only). | (inherits session) |
| `isolation` | No | `string` | Set to `worktree` to run in a temporary git worktree. Worktree is cleaned up if no changes. | (none) |
| `color` | No | `string` | Background color for the subagent in the UI. ~8 named colors accepted. | (default palette) |

`[Official]` Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

#### The `color` Field

The `color` field is functional but **not documented in the official frontmatter table**. It appears in the `/agents` interactive creation flow and in generated agent files. `[Community]`

**Known valid values** (named colors only):
`blue`, `purple`, `yellow`, `orange`, `green`, `magenta`, and possibly a few others. `[Community]`

**NOT supported:** Hex values (`#RRGGBB`), 256-color codes, or other custom formats. Invalid values silently fall back to defaults. `[Community]`

Sources: [GitHub issue #8501](https://github.com/anthropics/claude-code/issues/8501), [GitHub issue #23691](https://github.com/anthropics/claude-code/issues/23691)

### Model Field Values

| Value | Description |
|:------|:------------|
| `sonnet` | Claude Sonnet (balanced capability and speed) |
| `opus` | Claude Opus (most capable) |
| `haiku` | Claude Haiku (fastest, most economical) |
| `inherit` | Same model as the main conversation |
| Full model ID | e.g., `claude-opus-4-6`, `claude-sonnet-4-6`. Same values as `--model` CLI flag |
| (omitted) | Defaults to `inherit` |

`[Official]` Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Permission Modes

| Mode | Behavior |
|:-----|:---------|
| `default` | Standard permission checking with prompts |
| `acceptEdits` | Auto-accept file edits |
| `dontAsk` | Auto-deny permission prompts (explicitly allowed tools still work) |
| `bypassPermissions` | Skip all permission prompts. Writes to `.git`, `.claude`, `.vscode`, `.idea` still prompt (except `.claude/commands`, `.claude/agents`, `.claude/skills`). |
| `plan` | Plan mode (read-only exploration) |

If the parent uses `bypassPermissions`, it takes precedence and cannot be overridden. `[Official]`

**Security note:** Plugin subagents do NOT support `hooks`, `mcpServers`, or `permissionMode` fields. These are ignored when loading agents from plugins. `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Tool Access Control

#### Using `tools` (allowlist)

Comma-separated list. Only these tools are available:

```yaml
tools: Read, Grep, Glob, Bash
```

#### Using `disallowedTools` (denylist)

Comma-separated list. Inherits everything EXCEPT these:

```yaml
disallowedTools: Write, Edit
```

#### Interaction between `tools` and `disallowedTools`

If both are set, `disallowedTools` is applied first, then `tools` is resolved against the remaining pool. A tool listed in both is removed. `[Official]`

#### Restricting spawnable subagents

When an agent runs as the main thread with `claude --agent`, use `Agent(type)` syntax:

```yaml
tools: Agent(worker, researcher), Read, Bash
```

This is an allowlist: only `worker` and `researcher` can be spawned. Omitting `Agent` from `tools` entirely prevents spawning any subagents. `[Official]`

**Note:** This restriction only applies to agents running as the main thread. Subagents cannot spawn other subagents regardless. `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### MCP Server Configuration

The `mcpServers` field accepts a list of entries, each being either a string reference or an inline definition: `[Official]`

```yaml
mcpServers:
  # Inline definition: scoped to this subagent only
  - playwright:
      type: stdio
      command: npx
      args: ["-y", "@playwright/mcp@latest"]
  # Reference by name: reuses already-configured server
  - github
```

Inline definitions use the same schema as `.mcp.json` entries (`stdio`, `http`, `sse`, `ws`), keyed by server name.

**Key behavior:** Inline servers are connected when the subagent starts and disconnected when it finishes. To keep an MCP server out of the main conversation entirely, define it inline in the subagent. `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Persistent Memory

The `memory` field gives the subagent a persistent directory that survives across conversations. `[Official]`

| Scope | Location | Use when |
|:------|:---------|:---------|
| `user` | `~/.claude/agent-memory/<name>/` | Learnings should apply across all projects |
| `project` | `.claude/agent-memory/<name>/` | Knowledge is project-specific, shareable via VCS |
| `local` | `.claude/agent-memory-local/<name>/` | Project-specific but should NOT be checked in |

**When memory is enabled:**
- System prompt includes instructions for reading/writing to the memory directory
- First 200 lines of `MEMORY.md` in the memory directory are included in the system prompt
- If `MEMORY.md` exceeds 200 lines, instructions to curate it are included
- `Read`, `Write`, and `Edit` tools are automatically enabled for memory management

`[Official]` Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Hooks in Agents

Agents support the same hook format as skills. All hook events are supported. `[Official]`

**Special behavior:** `Stop` hooks defined in agent frontmatter are automatically converted to `SubagentStop` events at runtime. `[Official]`

```yaml
---
name: code-reviewer
description: Review code changes
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/validate-command.sh"
  PostToolUse:
    - matcher: "Edit|Write"
      hooks:
        - type: command
          command: "./scripts/run-linter.sh"
  Stop:
    - hooks:
        - type: command
          command: "./scripts/post-review.sh"
---
```

**Project-level hooks for subagent events** (in `settings.json`, not frontmatter):

| Event | Matcher input | When it fires |
|:------|:-------------|:--------------|
| `SubagentStart` | Agent type name | When a subagent begins execution |
| `SubagentStop` | Agent type name | When a subagent completes |

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Preloaded Skills

The `skills` field injects full skill content into the subagent's context at startup. This is different from normal skill discovery. `[Official]`

```yaml
---
name: api-developer
description: Implement API endpoints following team conventions
skills:
  - api-conventions
  - error-handling-patterns
---
```

**Key behaviors:**
- Full content of each skill is injected, not just made available for invocation
- Subagents do NOT inherit skills from the parent conversation
- Skills must be listed explicitly
- This is the inverse of `context: fork` in skills

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Agent Storage Locations

| Location | Scope | Priority | How to create |
|:---------|:------|:---------|:--------------|
| `--agents` CLI flag | Current session only | 1 (highest) | Pass JSON at launch |
| `.claude/agents/` | Current project | 2 | Interactive or manual |
| `~/.claude/agents/` | All your projects | 3 | Interactive or manual |
| Plugin's `agents/` directory | Where plugin is enabled | 4 (lowest) | Installed with plugins |

When multiple subagents share the same name, the higher-priority location wins. `[Official]`

#### CLI-defined agents

Passed as JSON, session-only, not saved to disk:

```bash
claude --agents '{
  "code-reviewer": {
    "description": "Expert code reviewer.",
    "prompt": "You are a senior code reviewer...",
    "tools": ["Read", "Grep", "Glob", "Bash"],
    "model": "sonnet"
  }
}'
```

The `--agents` flag accepts all frontmatter fields. Use `prompt` for the system prompt (equivalent to the markdown body in file-based agents). `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Built-in Subagents

| Agent | Model | Tools | Purpose |
|:------|:------|:------|:--------|
| **Explore** | Haiku | Read-only (Write/Edit denied) | File discovery, code search, codebase exploration |
| **Plan** | Inherits | Read-only (Write/Edit denied) | Codebase research for planning |
| **general-purpose** | Inherits | All tools | Complex research, multi-step operations, code modifications |
| **Bash** | Inherits | Terminal commands | Running terminal commands in separate context |
| **statusline-setup** | Sonnet | (specific) | Configuring the status line |
| **Claude Code Guide** | Haiku | (specific) | Answering questions about Claude Code features |

**Explore thoroughness levels:** When invoking Explore, Claude specifies `quick`, `medium`, or `very thorough`. `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Agent Invocation Methods

| Method | Syntax | Behavior |
|:-------|:-------|:---------|
| **Natural language** | "Use the code-reviewer subagent to..." | Claude decides whether to delegate |
| **@-mention** | `@"code-reviewer (agent)"` | Guarantees specific subagent runs for one task |
| **Session-wide** | `claude --agent code-reviewer` | Entire session uses that agent's config |
| **Settings default** | `{"agent": "code-reviewer"}` in `.claude/settings.json` | Default for every session in a project |

**@-mention details:** Type `@` and pick from typeahead. Plugin agents appear as `<plugin-name>:<agent-name>`. You can type manually: `@agent-<name>` or `@agent-<plugin-name>:<agent-name>`. `[Official]`

**Session-wide details:** The agent's system prompt replaces the default Claude Code system prompt entirely (same as `--system-prompt`). CLAUDE.md files still load normally. The agent name appears as `@<name>` in the startup header. CLI flag overrides the settings.json `agent` field. `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Foreground vs Background Execution

| Mode | Behavior |
|:-----|:---------|
| **Foreground** | Blocks main conversation. Permission prompts and clarifying questions passed through. |
| **Background** | Runs concurrently. Pre-approves permissions before launch. Auto-denies anything not pre-approved. Clarifying questions fail but agent continues. |

**Control background execution:**
- Set `background: true` in frontmatter to always run in background
- Ask Claude to "run this in the background"
- Press `Ctrl+B` to background a running task
- Set `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1` to disable all background tasks

If a background agent fails due to missing permissions, start a new foreground agent with the same task to retry. `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

### Agent Capabilities and Limitations

**What agents CAN do:** `[Official]`
- Read, write, and edit files
- Execute shell commands
- Search codebases
- Use MCP tools
- Run with different models and permission modes
- Maintain persistent memory
- Be resumed with full conversation history (via `SendMessage` tool with agent ID)
- Run in isolated git worktrees

**What agents CANNOT do:** `[Official]`
- Spawn other subagents (no nesting)
- Access the parent conversation's full history (only the delegation message)
- Override a parent's `bypassPermissions` mode

**Resuming agents:** Each subagent invocation creates a new instance. To continue previous work, ask Claude to resume it. Claude uses `SendMessage` with the agent's ID. Transcripts persist at `~/.claude/projects/{project}/{sessionId}/subagents/agent-{agentId}.jsonl`. `[Official]`

**Auto-compaction:** Subagents support automatic compaction at ~95% capacity. Override with `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE`. `[Official]`

Source: [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents)

---

## Agent Skills Open Standard

Claude Code skills follow the [Agent Skills](https://agentskills.io) open standard, which is supported by 30+ AI tools including Cursor, VS Code Copilot, Gemini CLI, OpenCode, Goose, Roo Code, and others. `[Official]`

### Open Standard Frontmatter Fields

The open standard defines a smaller set of fields than Claude Code supports: `[Official]`

| Field | Required | Constraints |
|:------|:---------|:------------|
| `name` | Yes | Max 64 chars. Lowercase alphanumeric + hyphens. No leading/trailing/consecutive hyphens. Must match directory name. |
| `description` | Yes | Max 1024 chars. Non-empty. |
| `license` | No | License name or reference to bundled license file. |
| `compatibility` | No | Max 500 chars. Environment requirements. |
| `metadata` | No | Arbitrary string-to-string key-value map. |
| `allowed-tools` | No | Space-delimited list of pre-approved tools. (Experimental) |

Source: [agentskills.io/specification](https://agentskills.io/specification)

### Claude Code Extensions to the Standard

Claude Code adds these fields beyond the open standard: `[Official]`

- `argument-hint`
- `disable-model-invocation`
- `user-invocable`
- `model`
- `effort`
- `context`
- `agent`
- `hooks`

These fields are Claude Code-specific and may not be recognized by other Agent Skills-compatible tools.

### Open Standard vs Claude Code Field Comparison

| Field | Open Standard | Claude Code |
|:------|:-------------|:------------|
| `name` | Required | Optional (defaults to directory name) |
| `description` | Required | Recommended (defaults to first paragraph) |
| `license` | Optional | Not documented in Claude Code docs |
| `compatibility` | Optional | Not documented in Claude Code docs |
| `metadata` | Optional | Not documented in Claude Code docs |
| `allowed-tools` | Optional (space-delimited) | Optional (comma-separated) |
| `argument-hint` | Not in spec | Optional |
| `disable-model-invocation` | Not in spec | Optional |
| `user-invocable` | Not in spec | Optional |
| `model` | Not in spec | Optional |
| `effort` | Not in spec | Optional |
| `context` | Not in spec | Optional |
| `agent` | Not in spec | Optional |
| `hooks` | Not in spec | Optional |

---

## Source Index

All claims in this document are tagged with one of these labels:

| Tag | Meaning |
|:----|:--------|
| `[Official]` | From Anthropic's official documentation (code.claude.com, platform.claude.com, agentskills.io) |
| `[Community]` | From community analysis, blog posts, or GitHub issues/discussions |
| `[Inferred]` | Derived from observed behavior or logical deduction |
| `[Unverified]` | Reported but not confirmed against official sources |

### Primary Sources

| Source | URL |
|:-------|:----|
| Claude Code Skills Documentation | [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills) |
| Claude Code Subagents Documentation | [code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents) |
| Agent Skills Specification (Open Standard) | [agentskills.io/specification](https://agentskills.io/specification) |
| Skill Authoring Best Practices | [platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices) |
| Agent Skills Overview | [platform.claude.com/docs/en/agents-and-tools/agent-skills/overview](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview) |
| Claude Code Hooks Documentation | [code.claude.com/docs/en/hooks](https://code.claude.com/docs/en/hooks) |
| Anthropic Skills Repository | [github.com/anthropics/skills](https://github.com/anthropics/skills) |
| Anthropic Engineering Blog: Agent Skills | [anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills) |

### Community/Issue Sources

| Source | URL |
|:-------|:----|
| Subagent Frontmatter Documentation Bug | [github.com/anthropics/claude-code/issues/8501](https://github.com/anthropics/claude-code/issues/8501) |
| Feature Request: disallowed-tools in Skills | [github.com/anthropics/claude-code/issues/6005](https://github.com/anthropics/claude-code/issues/6005) |
| Agent Team Color Configuration Request | [github.com/anthropics/claude-code/issues/23691](https://github.com/anthropics/claude-code/issues/23691) |
| Claude Code Skills Deep Dive (Lee Han Chung) | [leehanchung.github.io/blogs/2025/10/26/claude-skills-deep-dive](https://leehanchung.github.io/blogs/2025/10/26/claude-skills-deep-dive/) |
| Inside Claude Code Skills (Mikhail Shilkov) | [mikhail.io/2025/10/claude-code-skills](https://mikhail.io/2025/10/claude-code-skills/) |
