---
name: syllago-import
description: Import and manage AI coding tool content using syllago CLI
---

# Syllago Import Skill

Instructions for importing content into a syllago repository.

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

## Importing Content

Use `syllago import` to bring content into syllago from any provider:

```bash
# Import all content from a provider
syllago import --from claude-code

# Filter to specific type
syllago import --from claude-code --type skills

# Import a specific item by name
syllago import --from cursor --name my-rule

# Preview mode (discovery only, no parsing)
syllago import --from claude-code --preview
```

Content is copied to `local/<type>/[<provider>/]<name>/` and automatically canonicalized.

## Exporting Content

Use `syllago export` to install content into any provider:

```bash
# Export all content to a provider
syllago export --to cursor

# Export only skills to Kiro
syllago export --to kiro --type skills
```

Syllago handles format conversion automatically during export.

## Directory Structure

```
syllago-repo/
  skills/           # Shared skills (git-tracked)
  agents/           # Shared agents
  rules/
    claude-code/    # Provider-specific rules
    cursor/
  local/         # Local items (gitignored)
    skills/
    rules/
      claude-code/
```

Items in `local/` are local and not shared via git. Use the TUI's promote feature to move items to the shared directory.
