# Commands: Cross-Provider Deep Dive

> Research compiled 2026-03-22. Covers command systems across 13 AI coding tools.
> Output feeds into: converter/compat.go unified compat scorer (syllago-ohdb)

---

## Table of Contents

1. [Overview](#overview)
2. [Per-Provider Deep Dive](#per-provider-deep-dive)
   - [Claude Code](#claude-code)
   - [Cursor](#cursor)
   - [Gemini CLI](#gemini-cli)
   - [Copilot CLI (GitHub Copilot)](#copilot-cli-github-copilot)
   - [VS Code Copilot](#vs-code-copilot)
   - [Windsurf](#windsurf)
   - [Kiro](#kiro)
   - [Codex CLI](#codex-cli)
   - [Cline](#cline)
   - [OpenCode](#opencode)
   - [Roo Code](#roo-code)
   - [Zed](#zed)
   - [Amp](#amp)
3. [Cross-Platform Normalization Problem](#cross-platform-normalization-problem)
4. [Canonical Mapping](#canonical-mapping)
5. [Feature/Capability Matrix](#featurecapability-matrix)
6. [Compat Scoring Implications](#compat-scoring-implications)
7. [Converter Coverage Audit](#converter-coverage-audit)

---

## Overview

Commands (also called slash commands, custom commands, workflows, or custom prompts) let users define reusable prompt templates that can be invoked by name. They range from simple prompt snippets to structured definitions with metadata controlling tool access, execution context, and argument handling.

A major industry trend in 2026: commands and skills are merging. Claude Code, Codex CLI, and Amp have all deprecated standalone commands in favor of a unified "skills" system. However, legacy command files continue to work in all three. For syllago's converter, the practical distinction still matters because the file formats differ and many users still have `.claude/commands/` style files.

### Summary Table

| Provider | Has Commands | Format | Invocation | Args Placeholder | Scope |
|----------|-------------|--------|------------|-----------------|-------|
| Claude Code | Yes (merged into Skills) | MD + YAML frontmatter | `/` prefix | `$ARGUMENTS`, `$0`..`$N` | Project + Global |
| Cursor | Yes | Plain MD (no frontmatter) | `/` prefix | `$1`, `$2` | Project + Global |
| Gemini CLI | Yes | TOML | `/` prefix | `{{args}}` | Project + Global |
| Copilot CLI | Yes | MD + YAML frontmatter | `/` prefix | `$ARGUMENTS` | Project + Global |
| VS Code Copilot | Yes | `.prompt.md` + YAML frontmatter | `/` prefix | `${input:varName}` | Workspace + User profile |
| Windsurf | Yes (Workflows) | Plain MD (structured) | `/` prefix | None (embedded) | Project + Global |
| Kiro | Indirect | Hooks/Steering with frontmatter | `/` prefix | N/A | Project |
| Codex CLI | Yes (deprecated) | Plain MD + optional frontmatter | `/prompts:name` | `$ARGUMENTS`, `$1`..`$9` | Global only |
| Cline | Yes (Workflows) | Plain MD | `/filename.md` | N/A | Project |
| OpenCode | Yes | MD + optional YAML frontmatter | `/` prefix | `$ARGUMENTS`, `$1`..`$N` | Project + Global |
| Roo Code | Yes | MD + YAML frontmatter | `/` prefix | `$ARGUMENTS` | Project + Global |
| Zed | Extension-based | WASM / extension.toml | `/` prefix | varies | Extension |
| Amp | Deprecated | Formerly plain MD | `/` prefix | N/A (use Skills) | Project |

---

## Per-Provider Deep Dive

### Claude Code

**Source:** https://code.claude.com/docs/en/slash-commands

Commands have been merged into the Skills system. A file at `.claude/commands/deploy.md` and a skill at `.claude/skills/deploy/SKILL.md` both create `/deploy` and work identically. Existing `.claude/commands/` files continue to work; skills add optional features (supporting files directory, auto-invocation control).

Skills follow the [Agent Skills](https://agentskills.io) open standard.

#### File Format

Markdown with optional YAML frontmatter:

```yaml
---
name: fix-issue
description: Fix a GitHub issue
disable-model-invocation: true
allowed-tools: Read, Grep, Glob
context: fork
agent: Explore
model: claude-sonnet-4
effort: high
argument-hint: "[issue-number]"
user-invocable: true
hooks:
  PreToolUse:
    - matcher: Bash
      hooks:
        - type: command
          command: echo "pre-hook"
---

Fix GitHub issue $ARGUMENTS following our coding standards.
```

#### Directory Locations

| Scope | Path |
|-------|------|
| Enterprise | Managed settings |
| Personal (global) | `~/.claude/skills/<name>/SKILL.md` or `~/.claude/commands/<name>.md` |
| Project | `.claude/skills/<name>/SKILL.md` or `.claude/commands/<name>.md` |
| Plugin | `<plugin>/skills/<name>/SKILL.md` |
| Monorepo nested | `packages/<pkg>/.claude/skills/` (auto-discovered) |

Priority: enterprise > personal > project. If a skill and command share the same name, the skill wins.

#### Frontmatter Fields (Complete)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Display name and `/` command name. Lowercase, numbers, hyphens. Max 64 chars. Defaults to directory name. |
| `description` | string | Recommended | What the skill does. Used by Claude for auto-invocation decisions. |
| `argument-hint` | string | No | Hint shown in autocomplete. E.g., `[issue-number]` |
| `disable-model-invocation` | bool | No | If `true`, only user can invoke (not Claude). Default: `false` |
| `user-invocable` | bool | No | If `false`, hidden from `/` menu. Default: `true` |
| `allowed-tools` | list | No | Tools Claude can use without permission when active |
| `model` | string | No | Override model for this skill |
| `effort` | string | No | `low`, `medium`, `high`, `max`. Overrides session effort. |
| `context` | string | No | `fork` to run in isolated subagent |
| `agent` | string | No | Subagent type when `context: fork`. Options: `Explore`, `Plan`, `general-purpose`, or custom agent name. |
| `hooks` | object | No | Hooks scoped to this skill's lifecycle |

#### Argument Handling

| Placeholder | Description |
|-------------|-------------|
| `$ARGUMENTS` | All arguments as one string |
| `$ARGUMENTS[N]` | 0-based indexed argument |
| `$N` | Shorthand for `$ARGUMENTS[N]` (e.g., `$0`, `$1`, `$2`) |
| `${CLAUDE_SESSION_ID}` | Current session ID |
| `${CLAUDE_SKILL_DIR}` | Directory containing the SKILL.md |

If `$ARGUMENTS` is not present in content, arguments are appended as `ARGUMENTS: <value>`.

#### Dynamic Context

Shell command injection: `` !`command` `` runs before content is sent to Claude.
```
PR diff: !`gh pr diff`
```

#### Context Budget

Skill descriptions loaded at 2% of context window (fallback 16,000 chars). Override with `SLASH_COMMAND_TOOL_CHAR_BUDGET` env var.

---

### Cursor

**Source:** https://cursor.com/docs/context/commands

Cursor commands are intentionally simple: plain Markdown files with **no YAML frontmatter**. This is a deliberate design choice that distinguishes them from Cursor Rules (`.mdc` files which do use frontmatter).

#### File Format

Plain Markdown only. No frontmatter allowed.

```markdown
Review this code for security vulnerabilities:

1. Check for SQL injection
2. Verify input validation
3. Look for hardcoded credentials
```

#### Directory Locations

| Scope | Path |
|-------|------|
| Project | `.cursor/commands/<name>.md` |
| Global | User-level library (managed via `/commands` UI) |

#### Argument Handling

Positional arguments: `$1`, `$2`, etc.

#### Invocation

Type `/` in Agent chat input, select from dropdown. The filename (sans `.md`) becomes the command name.

#### Key Differences from Other Providers

- **No metadata at all** -- no description, no tool restrictions, no execution context
- Commands carry "more weight" than rules with the LLM because they are user-initiated prompts
- Strongly recommends keeping commands under 150 lines for context efficiency
- Version-controlled via git for team sharing

---

### Gemini CLI

**Source:** https://google-gemini.github.io/gemini-cli/docs/cli/custom-commands.html

Gemini CLI uses TOML format, making it the most structurally different from the Markdown-based ecosystem.

#### File Format

```toml
description = "Refactor code into a pure function"
prompt = """Analyze the provided code.
Refactor it into a pure function.

Response should include:
1. Refactored code block
2. Explanation of changes"""
```

#### Directory Locations

| Scope | Path |
|-------|------|
| Global | `~/.gemini/commands/` |
| Project | `<project-root>/.gemini/commands/` |
| Extension | `<extension>/commands/` |

Project overrides global on name conflict.

#### TOML Keys

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `prompt` | string | Yes | Prompt text sent to model |
| `description` | string | No | One-line description for `/help`. Auto-generated from filename if omitted. |

Note: Only two fields. No tool restrictions, no execution context, no model override.

#### Argument Handling

| Method | Behavior |
|--------|----------|
| `{{args}}` in prompt | Replaced with user input. Auto-shell-escaped inside `!{...}` blocks. |
| No `{{args}}` | User input appended after prompt with two newlines. |

#### Template Directives

| Directive | Purpose | Example |
|-----------|---------|---------|
| `!{command}` | Execute shell command, inject output | `!{git diff --staged}` |
| `@{path}` | Inject file/directory content | `@{docs/best-practices.md}` |

Shell commands within `!{...}` require user confirmation. Arguments inside `!{...}` are automatically shell-escaped. `@{...}` supports multimodal content (images, PDFs, audio, video), respects `.gitignore`/`.geminiignore`, and is processed before shell commands.

#### Namespacing

Subdirectories create colon-separated namespaces:
- `git/commit.toml` becomes `/git:commit`
- `refactor/pure.toml` becomes `/refactor:pure`

#### Reloading

`/commands reload` picks up changes without restart.

---

### Copilot CLI (GitHub Copilot)

**Source:** https://github.com/github/copilot-cli/blob/main/changelog.md, https://docs.github.com/en/copilot/how-tos/use-copilot-agents/use-copilot-cli

Copilot CLI added custom command support progressively through early 2026. As of v0.0.412 (2026-02-19), command files no longer require YAML frontmatter (plain markdown works). The CLI reads commands from the Claude Code compatible `.claude/commands/` directory format and also supports skills.

#### File Format

Markdown with optional YAML frontmatter (Claude Code compatible):

```yaml
---
description: Review code for security issues
allowed-tools: Read, Grep
disable-model-invocation: true
---

Review the code for security vulnerabilities...
```

Plain Markdown (no frontmatter) also works as of v0.0.412.

#### Directory Locations

| Scope | Path |
|-------|------|
| Global | `~/.copilot/commands/` or `~/.claude/commands/` |
| Project | `.github/agents/`, `.claude/commands/` |

Note: The CLI reads from both `.github/` and `.claude/` paths for cross-tool compatibility.

#### Frontmatter Fields

Copilot CLI supports the Claude Code frontmatter schema:
- `description` -- command description
- `allowed-tools` -- tool restrictions
- `disable-model-invocation` -- prevent automatic invocation (added v0.0.412)
- `user-invocable` -- hide from picker when `false` (added v0.0.412)
- `model` -- model override (added v0.0.4 for custom agents)

#### Argument Handling

`$ARGUMENTS` placeholder (Claude Code compatible).

#### Status Notes

- Issue #618 (custom commands from `.github/prompts/`) was closed as completed
- Issue #1113 (custom commands via markdown files) remains open
- SDK clients can register custom slash commands when starting/joining sessions (v1.0.10)
- The CLI increasingly converges on Claude Code's format as a compatibility layer

---

### VS Code Copilot

**Source:** https://code.visualstudio.com/docs/copilot/customization/prompt-files

VS Code Copilot uses a distinct `.prompt.md` format with its own frontmatter schema. These are called "prompt files" and function as slash commands.

#### File Format

```yaml
---
name: security-review
description: Review REST API security
agent: ask
model: Claude Sonnet 4
tools:
  - search/codebase
  - vscode/askQuestions
  - myMcpServer/*
argument-hint: "<endpoint-path>"
---

Perform a security audit checking authentication, input validation, and rate limiting.

Reference: [API standards](../docs/api-standards.md)
Use #tool:browser to check external endpoints.
```

#### Directory Locations

| Scope | Path |
|-------|------|
| Workspace | `.github/prompts/*.prompt.md` |
| User profile | `prompts/` folder in VS Code profile |
| Custom | Configurable via `chat.promptFilesLocations` |

#### Frontmatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Command name after `/`. Defaults to filename. |
| `description` | string | No | Brief description |
| `argument-hint` | string | No | Guidance text in chat input |
| `agent` | string | No | `ask`, `agent`, `plan`, or custom agent name |
| `model` | string | No | Override model |
| `tools` | list | No | Available tools. `<server>/*` for all MCP server tools. |

#### Argument Handling

Uses input variables: `${input:variableName}` or `${input:variableName:placeholder}`.

This is a fundamentally different approach from `$ARGUMENTS` -- variables are named, not positional.

#### Special Syntax

- `#tool:<tool-name>` references tools in body text
- Markdown links to workspace files for context injection
- `/create-prompt` AI-assisted prompt generation

#### Key Differences

- `.prompt.md` extension (not `.md`)
- `tools` field instead of `allowed-tools`
- `agent` field maps to execution mode, not subagent type
- Named input variables instead of positional `$ARGUMENTS`
- No `context: fork` equivalent
- No `disable-model-invocation` -- prompts are always user-invoked

---

### Windsurf

**Source:** https://docs.windsurf.com/windsurf/cascade/workflows

Windsurf calls its command system "Workflows." These are sequential step-based instructions saved as Markdown files. Workflows are **manual-only** -- Cascade never auto-invokes them (use Skills for that).

#### File Format

Plain Markdown with structured sections (title, description, steps). No YAML frontmatter.

```markdown
# Deploy to Production

Deploy the application following our standard release process.

## Steps

1. Run the full test suite
2. Build the production bundle
3. Push to deployment target
4. Verify health checks pass
```

#### Directory Locations

| Scope | Path |
|-------|------|
| Project | `.windsurf/workflows/` |
| Global | `~/.codeium/windsurf/global_workflows/` |
| Enterprise (macOS) | `/Library/Application Support/Windsurf/workflows/` |
| Enterprise (Linux/WSL) | `/etc/windsurf/workflows/` |
| Enterprise (Windows) | `C:\ProgramData\Windsurf\workflows\` |

Auto-discovers workflows from workspace subdirectories and up to git root. Deduplicates by shortest path.

#### Characteristics

- **No frontmatter** -- title/description/steps are structural sections in the Markdown
- **No argument handling** -- no placeholder syntax documented
- **No metadata fields** -- no tool restrictions, model override, etc.
- **Nesting** -- workflows can invoke other workflows via `/workflow-name`
- **Character limit** -- workflow files capped at 12,000 characters
- **AI generation** -- ask Cascade to generate workflows for you

#### Invocation

`/[workflow-name]` in Cascade chat. Can also be created/managed via Customizations panel.

---

### Kiro

**Source:** https://kiro.dev/docs/chat/slash-commands/

Kiro does NOT have a dedicated custom commands system. Instead, three existing content types surface as slash commands:

1. **Hooks with manual triggers** -- appear in `/` menu, execute immediately
2. **Steering files with `inclusion: manual`** -- appear in `/` menu, inject content into context
3. **Agent skills** -- appear in `/` menu, load full skill instructions

#### How It Works

Hooks become commands by setting trigger type to "Manual." Steering files become commands by adding `inclusion: manual` to frontmatter. This is an indirect mechanism -- there is no standalone "command" content type.

#### File Formats

Hooks: JSON configuration files
Steering files: Markdown with frontmatter (including `inclusion: manual`)
Skills: Directory-based with skill definition

#### Key Limitation

No dedicated command file format. No `$ARGUMENTS` equivalent. No custom command directories. The slash command menu is populated by configuring other content types for manual invocation.

---

### Codex CLI

**Source:** https://developers.openai.com/codex/custom-prompts

Custom prompts are **deprecated** in favor of skills. They are stored as Markdown files in `~/.codex/prompts/` (global only, no project scope).

#### File Format

```yaml
---
description: Draft a pull request
argument-hint: "[FILES=<paths>] [PR_TITLE=\"<title>\"]"
---

Create a PR for the following changes: $ARGUMENTS

Include a summary and test plan.
```

#### Directory Location

`~/.codex/prompts/` only. No subdirectories. Non-Markdown files ignored.

#### Frontmatter Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Shown in slash command menu |
| `argument-hint` | string | Documents expected parameters |

Only two frontmatter fields -- much simpler than Claude Code.

#### Argument Handling

| Placeholder | Description |
|-------------|-------------|
| `$ARGUMENTS` | All arguments as one string |
| `$1` through `$9` | Positional arguments |
| `$NAME` | Named arguments (uppercase letters, numbers, underscores). Supplied as `KEY=value`. |
| `$$` | Literal dollar sign |

Named placeholders are a unique Codex feature not found in other providers.

#### Invocation

`/prompts:<name>` in CLI or IDE. Requires restart for file changes to take effect.

---

### Cline

**Source:** https://cline.bot/blog/cline-3-13-toggleable-clinerules-slash-commands-previous-message-editing

Cline's command system uses "workflow" files -- Markdown files in `.clinerules/workflows/`.

#### File Format

Plain Markdown. No YAML frontmatter documented.

#### Directory Location

`.clinerules/workflows/*.md` in project root.

#### Invocation

`/your-workflow.md` -- note the `.md` extension is included in the invocation, unlike other providers.

#### Built-in Commands

`/newtask`, `/smol` (compact), `/deep-planning`, `/explain-changes`, `/reportbug`, `/newrule`

#### Cline CLI 2.0

Extends workflow support to the terminal. Drop a markdown file into custom workflows directory and it becomes `/filename` automatically. Also supports `-y` flag for autonomous execution.

#### Key Limitations

- No frontmatter support
- No argument placeholders
- No tool restrictions or metadata
- Very simple system compared to Claude Code or Gemini CLI

---

### OpenCode

**Source:** https://opencode.ai/docs/commands/

#### File Format

Markdown with optional YAML frontmatter:

```yaml
---
description: Create a React component
agent: coder
model: claude-sonnet-4
subtask: true
---

Create a new React component named $ARGUMENTS with TypeScript support.
Include proper typing and basic structure.

Current project structure: !`ls src/components/`
Existing components: @src/components/index.ts
```

#### Directory Locations

| Scope | Path |
|-------|------|
| Global | `~/.config/opencode/commands/` |
| Project | `.opencode/commands/` |

#### Frontmatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | No | Brief summary shown in TUI |
| `template` | string | No | The prompt template (alternative to body) |
| `agent` | string | No | Which agent executes |
| `model` | string | No | Override model |
| `subtask` | bool | No | Force subagent invocation |

Note: `template` in frontmatter vs body content -- OpenCode supports both. `subtask` is roughly analogous to Claude Code's `context: fork`.

#### Argument Handling

| Placeholder | Description |
|-------------|-------------|
| `$ARGUMENTS` | All arguments as one string |
| `$1`, `$2`, `$3` | Positional arguments |

#### Special Syntax

| Syntax | Purpose |
|--------|---------|
| `` !`command` `` | Inject shell command output |
| `@filename` | Include file content |

These are similar to Gemini CLI's `!{...}` and `@{...}` but with different syntax.

#### JSON Configuration Alternative

Commands can also be defined in `opencode.jsonc`:
```json
{
  "command": {
    "test": {
      "template": "Run tests...",
      "description": "Run tests with coverage"
    }
  }
}
```

#### Other Notes

Custom commands can override built-in commands (`/init`, `/undo`, `/redo`, `/share`, `/help`).

---

### Roo Code

**Source:** https://docs.roocode.com/features/slash-commands

#### File Format

Markdown with optional YAML frontmatter:

```yaml
---
description: Generate REST API with best practices
argument-hint: <endpoint-name> <http-method>
mode: code
---

Create a new REST API endpoint with:
- Proper error handling
- Input validation
- Rate limiting
```

#### Directory Locations

| Scope | Path |
|-------|------|
| Project | `.roo/commands/` |
| Global | `~/.roo/commands/` |

Priority: project > global > built-in.

#### Frontmatter Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Shown in command menu |
| `argument-hint` | string | Expected argument guidance |
| `mode` | string | Mode slug to switch to before execution (e.g., `code`, `architect`, `debug`) |

The `mode` field is unique to Roo Code -- it auto-switches the AI's operational mode before running the command.

#### Argument Handling

`$ARGUMENTS` placeholder. Argument hints use `<placeholder>` format in the UI.

#### Command Name Processing

Names are auto-processed: lowercase, spaces to dashes, special chars removed, consecutive dashes collapsed.

#### Programmatic Execution

The `run_slash_command` tool lets the AI execute slash commands programmatically, enabling chained workflows.

---

### Zed

**Source:** https://zed.dev/docs/extensions/slash-commands

Zed's slash command system is fundamentally different: commands are provided by **extensions** (WASM-based), not by user-authored Markdown files.

#### Built-in Commands

| Command | Purpose |
|---------|---------|
| `/diagnostics` | Inject language server errors |
| `/file` | Insert file/directory content (supports globs) |
| `/now` | Insert current date/time |
| `/prompt` | Insert rule from Rules Library |
| `/symbols` | Insert active symbols from current tab |
| `/tab` | Insert active tab content |

#### Extension-Based Custom Commands

Extensions register commands in `extension.toml`:

```toml
[slash_commands.echo]
description = "Echo back the input"
requires_argument = true

[slash_commands.pick-one]
description = "Pick a random option"
requires_argument = true
```

Implementation is in Rust/WASM via the extension API. This is a developer-level feature, not a user configuration.

#### External Agent Compatibility

Zed supports external agents (Claude Code, Gemini CLI, Codex) via ACP. When using these agents inside Zed, their native slash commands work, but with some limitations -- not all commands behave correctly in the agent window yet.

#### Key Differences

- No user-authored Markdown commands
- Extension API (WASM) for custom commands
- Commands evaluated at insertion time, not dynamically
- `/prompt` can nest rules inside rules

---

### Amp

**Source:** https://ampcode.com/news/slashing-custom-commands

Amp has **removed custom commands entirely** in favor of skills. The two systems were redundant: commands could only be user-invoked, skills could be both user- and agent-invoked.

#### Former Format (Deprecated)

```markdown
Review this PR for:
1. Security issues
2. Performance concerns
3. Code style violations
```

Stored in `.agents/commands/<name>.md`. Also supported executable commands (files with `#!` shebang).

#### Migration to Skills

| Old Path | New Path |
|----------|----------|
| `.agents/commands/code-review.md` | `.agents/skills/code-review/SKILL.md` |
| `.agents/commands/deploy.sh` (executable) | `.agents/skills/deploy/scripts/deploy.sh` + `SKILL.md` |

Skills require SKILL.md with frontmatter (`name`, `description`).

#### Converter Implication

Any existing Amp command files should be treated as plain Markdown (no frontmatter). For rendering to Amp, output should be in the skills format.

---

## Cross-Platform Normalization Problem

### Format Fragmentation

The command ecosystem splits into four format families:

1. **YAML frontmatter + Markdown body** (Claude Code, Copilot CLI, VS Code Copilot, OpenCode, Roo Code, Codex CLI)
2. **Plain Markdown** (Cursor, Windsurf, Cline, Amp legacy)
3. **TOML** (Gemini CLI)
4. **Extension API** (Zed)

### Argument Placeholder Divergence

| Style | Providers | Notes |
|-------|-----------|-------|
| `$ARGUMENTS` | Claude Code, Copilot CLI, OpenCode, Roo Code, Codex CLI | Most common |
| `$1`, `$2`, `$N` | Claude Code, Cursor, Codex CLI, OpenCode | Positional |
| `{{args}}` | Gemini CLI | Unique to Gemini |
| `${input:name}` | VS Code Copilot | Named variables |
| `$NAME` (named) | Codex CLI | `KEY=value` syntax |
| None | Windsurf, Cline, Kiro, Zed | No argument support |

### Template Directive Divergence

| Feature | Claude Code | Gemini CLI | OpenCode |
|---------|------------|------------|----------|
| Shell injection | `` !`cmd` `` | `!{cmd}` | `` !`cmd` `` |
| File injection | N/A (use supporting files) | `@{path}` | `@path` |
| Shell escaping | Manual | Auto in `!{...}` | Manual |

### Metadata Field Divergence

| Field | Claude Code | Gemini CLI | Copilot CLI | VS Code | OpenCode | Roo Code | Codex |
|-------|------------|------------|-------------|---------|----------|----------|-------|
| name | Y | (filename) | Y | Y | (filename) | (filename) | (filename) |
| description | Y | Y | Y | Y | Y | Y | Y |
| allowed-tools | Y | - | Y | (tools) | - | - | - |
| model | Y | - | Y | Y | Y | - | - |
| context/fork | Y | - | - | - | (subtask) | - | - |
| agent | Y | - | - | Y (mode) | Y | - | - |
| mode switching | - | - | - | - | - | Y | - |
| effort | Y | - | - | - | - | - | - |
| argument-hint | Y | - | - | Y | - | Y | Y |
| user-invocable | Y | - | Y | - | - | - | - |
| disable-model-invocation | Y | - | Y | - | - | - | - |

---

## Canonical Mapping

The current canonical format (`CommandMeta`) maps as follows:

```
Canonical Field         -> Primary Source     -> Notes
-------------------------------------------------------------------
name                    -> Claude Code        -> filename-derived in most providers
description             -> universal          -> supported everywhere except Cursor/Windsurf/Cline
allowed-tools           -> Claude Code        -> VS Code uses "tools" with different semantics
context ("fork")        -> Claude Code        -> OpenCode has "subtask: true" equivalent
agent                   -> Claude Code        -> VS Code "agent" is execution mode, not subagent
model                   -> Claude Code        -> supported in CC, Copilot, VS Code, OpenCode
disable-model-invocation -> Claude Code       -> Copilot CLI added support in v0.0.412
user-invocable          -> Claude Code        -> Copilot CLI added support in v0.0.412
argument-hint           -> Claude Code        -> also in VS Code, Roo Code, Codex
```

### Fields NOT in Canonical (but should be considered)

| Field | Provider | Purpose | Add to Canonical? |
|-------|----------|---------|-------------------|
| `mode` | Roo Code | Switch AI operational mode before execution | Maybe -- unique to Roo Code |
| `effort` | Claude Code | Override effort level | Yes -- Claude Code specific but useful |
| `subtask` | OpenCode | Force subagent invocation | Covered by `context: fork` |
| `tools` (list) | VS Code Copilot | Tool availability list (different from allowed-tools) | Map to `allowed-tools` |
| `hooks` | Claude Code | Lifecycle hooks scoped to command | No -- too provider-specific |
| `template` | OpenCode | Prompt text in frontmatter instead of body | No -- body content serves this role |

### Recommended Canonical Additions

1. **`effort`** -- already a Claude Code field, useful for controlling response depth
2. Consider mapping Roo Code's `mode` to the existing `agent` field with a behavioral note

---

## Feature/Capability Matrix

Features rated: Full (F), Partial (P), Behavioral embedding only (B), None (-)

| Capability | Claude Code | Cursor | Gemini CLI | Copilot CLI | VS Code | Windsurf | Kiro | Codex | Cline | OpenCode | Roo Code | Zed | Amp |
|-----------|------------|--------|------------|-------------|---------|----------|------|-------|-------|----------|----------|-----|-----|
| Custom commands | F | F | F | F | F | F | P | F* | P | F | F | P | -* |
| YAML frontmatter | F | - | - | F | F | - | - | F | - | F | F | - | - |
| Description | F | - | F | F | F | P | P | F | - | F | F | P | - |
| Tool restrictions | F | - | - | F | F | - | - | - | - | - | - | - | - |
| Model override | F | - | - | F | F | - | - | - | - | F | - | - | - |
| Argument placeholders | F | P | F | F | F | - | - | F | - | F | F | P | - |
| Positional args | F | F | - | P | - | - | - | F | - | F | - | - | - |
| Named args | - | - | - | - | F | - | - | F | - | - | - | - | - |
| Shell injection | F | - | F | - | - | - | - | - | - | F | - | - | - |
| File injection | - | - | F | - | - | - | - | - | - | F | - | - | - |
| Fork/subagent | F | - | - | - | - | - | - | - | - | F | - | - | - |
| Auto-invocation control | F | - | - | F | - | - | P | - | - | - | - | - | - |
| Mode switching | - | - | - | - | P | - | - | - | - | - | F | - | - |
| Namespace/hierarchy | - | - | F | - | - | - | - | - | - | - | - | - | - |
| Nested workflows | - | - | - | - | - | F | - | - | - | - | - | - | - |
| Enterprise scope | F | - | - | - | - | F | - | - | - | - | - | - | - |

`*` Codex: deprecated. Amp: removed (use skills).

---

## Compat Scoring Implications

### High Compatibility Pairs (score: 0.8-1.0)

- **Claude Code <-> Copilot CLI**: Nearly identical format. Copilot CLI reads `.claude/commands/` directly. Same frontmatter fields, same argument syntax.
- **Claude Code <-> OpenCode**: Same body format, `$ARGUMENTS` syntax. OpenCode has subset of frontmatter fields. `subtask` maps to `context: fork`.
- **Claude Code <-> Roo Code**: Same body format, `$ARGUMENTS` syntax. Roo Code has subset of frontmatter. `mode` field has no Claude Code equivalent (behavioral note).

### Medium Compatibility Pairs (score: 0.5-0.7)

- **Claude Code <-> Codex CLI**: Same argument syntax. Codex has minimal frontmatter (only `description`, `argument-hint`). Named arguments (`$NAME`) are unique to Codex and lossy in other providers.
- **Claude Code <-> Gemini CLI**: `$ARGUMENTS` <-> `{{args}}` is a simple string replace. TOML <-> YAML+MD is a structural transform. Template directives (`!{...}`, `@{...}`) have no Claude Code equivalent.
- **Claude Code <-> VS Code Copilot**: Different file extension, different argument system (`${input:name}` vs `$ARGUMENTS`), `tools` vs `allowed-tools`, `agent` semantics differ.

### Low Compatibility Pairs (score: 0.2-0.4)

- **Claude Code <-> Cursor**: Cursor has no frontmatter at all. All metadata is lost. Only body content and `$1`/`$2` positional args survive.
- **Claude Code <-> Windsurf**: No frontmatter, no arguments, structured step format. Essentially a prose-to-prose conversion with massive metadata loss.
- **Claude Code <-> Cline**: Similar to Windsurf -- no frontmatter, no arguments.
- **Any <-> Zed**: Extension API is fundamentally different. No file-based conversion possible.

### Compat Score Factors

| Factor | Weight | Description |
|--------|--------|-------------|
| Format match | 0.3 | Same file format family (MD+YAML, TOML, plain MD) |
| Metadata preservation | 0.25 | How many frontmatter fields survive conversion |
| Argument fidelity | 0.2 | Placeholder syntax compatibility |
| Tool restrictions | 0.15 | Whether tool access control is preserved |
| Execution context | 0.1 | Fork/subagent/mode semantics |

---

## Converter Coverage Audit

### Current Coverage

The existing `commands.go` converter handles:

| Direction | Provider | Status | Notes |
|-----------|----------|--------|-------|
| Canonicalize | claude-code | Covered | YAML frontmatter + MD body |
| Canonicalize | copilot-cli | Covered | Falls through to Claude Code path (same format) |
| Canonicalize | gemini-cli | Covered | TOML parsing, `{{args}}` preserved |
| Canonicalize | codex | Covered | Plain MD, no frontmatter extraction |
| Canonicalize | opencode | Covered | Falls through to Claude Code path |
| Render | claude-code | Covered | Full YAML frontmatter + MD body |
| Render | copilot-cli | Covered | Same as Claude Code renderer |
| Render | gemini-cli | Covered | TOML output, `$ARGUMENTS` -> `{{args}}` |
| Render | codex | Covered | Plain MD with behavioral notes |
| Render | opencode | Covered | MD with description-only frontmatter |

### Providers NOT Covered (that support commands)

| Provider | Priority | Effort | Notes |
|----------|----------|--------|-------|
| **Cursor** | High | Low | Plain MD renderer: strip frontmatter, keep body. Convert `$ARGUMENTS` to `$1`. |
| **VS Code Copilot** | High | Medium | `.prompt.md` format, different frontmatter schema (`tools` not `allowed-tools`, `agent` not same semantics), `${input:}` argument syntax. |
| **Roo Code** | Medium | Low | Similar to Claude Code. Add `mode` field handling. Render `description` + `argument-hint` + `mode` frontmatter. |
| **Windsurf** | Medium | Medium | Structural transform: flatten frontmatter into step-based Markdown. No metadata preservation on canonicalize. |
| **Cline** | Low | Low | Plain MD like Codex. Invocation includes `.md` extension. |
| **Kiro** | Low | High | No direct command format. Would need to render as steering file with `inclusion: manual` frontmatter, or as a hook config. |
| **Zed** | Skip | N/A | Extension API -- cannot generate WASM extensions from Markdown. |
| **Amp** | Skip | N/A | Commands removed. Render to Amp skills format instead (covered by skills converter). |

### Existing Converter Issues

1. **Codex canonicalize loses data**: Codex commands with `$NAME` named arguments are not extracted or preserved. These would be lost on round-trip.

2. **`$ARGUMENTS` <-> `{{args}}` is one-way**: The converter replaces `$ARGUMENTS` -> `{{args}}` when rendering to Gemini, but does NOT replace `{{args}}` -> `$ARGUMENTS` when canonicalizing from Gemini. This means Gemini-native commands lose their argument placeholders.

3. **OpenCode renderer is too minimal**: The renderer only outputs `description` in frontmatter, but OpenCode supports `agent`, `model`, and `subtask` fields that could be populated from canonical data.

4. **Codex renderer doesn't output frontmatter**: Codex supports `description` and `argument-hint` in frontmatter, but the renderer outputs plain MD with behavioral notes instead.

5. **Gemini template directives**: `!{...}` and `@{...}` directives are detected and warned about but not transformed. No attempt to convert them to OpenCode's `` !`...` `` / `@...` syntax or Claude Code's `` !`...` `` syntax.

6. **`effort` field not in canonical**: Claude Code's `effort` field is not captured in `CommandMeta`.

### Recommended Converter Additions (Priority Order)

1. **Add Cursor renderer** -- strip frontmatter, output plain MD, convert `$ARGUMENTS` to `$1`
2. **Add VS Code Copilot canonicalizer + renderer** -- map `tools` <-> `allowed-tools`, handle `${input:}` syntax, `.prompt.md` extension
3. **Add Roo Code renderer** -- output `description`, `argument-hint`, `mode` frontmatter
4. **Fix Gemini canonicalize** -- replace `{{args}}` -> `$ARGUMENTS` during canonicalization
5. **Enhance OpenCode renderer** -- include `agent`, `model`, `subtask` fields
6. **Enhance Codex renderer** -- include `description`, `argument-hint` frontmatter
7. **Add Windsurf renderer** -- structural transform to step-based Markdown
8. **Add `effort` to CommandMeta** -- Claude Code specific but worth preserving

### Commands vs Skills Convergence

The industry is clearly converging on a unified "skills" model. For the converter:

- **Claude Code**: Commands and skills use identical frontmatter. No separate handling needed.
- **Codex CLI**: Custom prompts deprecated in favor of skills. Converter should support both for backwards compatibility.
- **Amp**: Commands removed. Only skills format for rendering.
- **Copilot CLI**: Supports both commands and skills.

The skills converter should handle the SKILL.md directory structure, while the commands converter handles single-file commands. For providers where they've merged (Claude Code), either converter path works.

---

## Sources

- [Claude Code Skills/Commands](https://code.claude.com/docs/en/slash-commands)
- [Cursor Commands](https://cursor.com/docs/context/commands)
- [Gemini CLI Custom Commands](https://google-gemini.github.io/gemini-cli/docs/cli/custom-commands.html)
- [GitHub Copilot CLI](https://github.com/github/copilot-cli/blob/main/changelog.md)
- [VS Code Copilot Prompt Files](https://code.visualstudio.com/docs/copilot/customization/prompt-files)
- [Windsurf Workflows](https://docs.windsurf.com/windsurf/cascade/workflows)
- [Kiro Slash Commands](https://kiro.dev/docs/chat/slash-commands/)
- [Codex Custom Prompts](https://developers.openai.com/codex/custom-prompts)
- [Cline Slash Commands](https://cline.bot/blog/cline-3-13-toggleable-clinerules-slash-commands-previous-message-editing)
- [OpenCode Commands](https://opencode.ai/docs/commands/)
- [Roo Code Slash Commands](https://docs.roocode.com/features/slash-commands)
- [Zed Slash Command Extensions](https://zed.dev/docs/extensions/slash-commands)
- [Amp Custom Commands (deprecated)](https://ampcode.com/news/slashing-custom-commands)
