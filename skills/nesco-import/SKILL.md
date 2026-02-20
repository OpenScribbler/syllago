---
name: nesco-import
description: Import and manage AI coding tool content using nesco CLI
---

# Nesco Import Skill

Instructions for importing content into a nesco repository.

## Content Types

| Type | Directory | Universal | Description |
|------|-----------|-----------|-------------|
| skills | `skills/` | Yes | Reusable skill definitions (SKILL.md) |
| agents | `agents/` | Yes | Agent definitions (AGENT.md) |
| prompts | `prompts/` | Yes | Prompt templates (PROMPT.md) |
| mcp | `mcp/` | Yes | MCP server configurations |
| apps | `apps/` | Yes | Application definitions |
| rules | `rules/<provider>/` | No | Provider-specific rules |
| hooks | `hooks/<provider>/` | No | Provider-specific hooks |
| commands | `commands/<provider>/` | No | Provider-specific commands |

**Universal types** work with any AI tool. **Provider-specific types** require a `--provider` flag.

## Adding Content (Non-Interactive)

Use `nesco add` to import content without the TUI:

```bash
# Universal type (skills, agents, prompts, mcp, apps)
nesco add /path/to/my-skill --type skills

# Provider-specific type (rules, hooks, commands)
nesco add /path/to/my-rules --type rules --provider claude-code

# Custom name (defaults to source directory basename)
nesco add /path/to/content --type skills --name my-custom-name
```

Content is copied to `my-tools/<type>/[<provider>/]<name>/`.

## README Generation

When importing content that lacks a README.md, one is generated automatically with:
- A title derived from the item name
- The content type

Existing README.md files are never overwritten.

## Discovering Existing Provider Content

Use `nesco import` to discover what a provider already has configured:

```bash
# See what Claude Code has
nesco import --from claude-code

# Filter to specific type
nesco import --from claude-code --type rules

# Preview mode (discovery only, no parsing)
nesco import --from claude-code --preview
```

## Directory Structure

```
nesco-repo/
  skills/           # Shared skills (git-tracked)
  agents/           # Shared agents
  rules/
    claude-code/    # Provider-specific rules
    cursor/
  my-tools/         # Local items (gitignored)
    skills/
    rules/
      claude-code/
```

Items in `my-tools/` are local and not shared via git. Use the TUI's promote feature to move items to the shared directory.
