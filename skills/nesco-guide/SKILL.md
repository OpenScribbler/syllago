---
name: nesco-guide
description: Complete reference for using nesco to manage AI coding tool content
---

# Nesco Guide

Nesco is the package manager for AI coding tool content. It lets you browse, install, and share skills, rules, agents, hooks, MCP configs, and more across 11 AI coding tools.

## Core Workflow

```
nesco import --from <provider>    # Bring content in from a provider
nesco export --to <provider>      # Send content out to any provider
nesco                             # Browse everything in the TUI
```

Content lives in `local/` after import. Nesco handles format conversion automatically — a Claude Code skill becomes a Kiro steering file, a Cursor rule becomes a Windsurf rule, etc.

## Commands

| Command | Purpose |
|---------|---------|
| `nesco` | Launch the interactive TUI |
| `nesco init` | Initialize nesco for a project (creates `.nesco/config.json`) |
| `nesco import --from <provider>` | Discover and import content from a provider |
| `nesco export --to <provider>` | Convert and install content to a provider |
| `nesco registry add <url>` | Add a git-based content registry |
| `nesco registry sync` | Pull latest from all registries |
| `nesco registry items` | Browse available registry content |
| `nesco sandbox run <provider>` | Run a provider in a bubblewrap sandbox (Linux) |
| `nesco sandbox check <provider>` | Verify sandbox prerequisites |
| `nesco config list` | Show configured providers |
| `nesco update` | Self-update to latest release |
| `nesco info` | Show content types, providers, and capabilities |

### Import Flags

```bash
nesco import --from claude-code                  # All content
nesco import --from claude-code --type skills    # Only skills
nesco import --from cursor --name my-rule        # Specific item by name
nesco import --from claude-code --preview        # Read-only discovery
```

### Export Flags

```bash
nesco export --to cursor                         # All content to Cursor
nesco export --to kiro --type skills             # Only skills to Kiro
nesco export --to gemini-cli --name research     # Specific item
nesco export --to codex --llm-hooks generate     # Generate hook wrapper scripts
```

## Content Types

| Type | Directory | Scope | Main File |
|------|-----------|-------|-----------|
| Skills | `skills/<name>/` | Universal | SKILL.md |
| Agents | `agents/<name>/` | Universal | AGENT.md |
| Prompts | `prompts/<name>/` | Universal | PROMPT.md |
| MCP Configs | `mcp/<name>/` | Universal | config.json |
| Apps | `apps/<name>/` | Universal | README.md + install.sh |
| Rules | `rules/<provider>/<name>/` | Provider-specific | varies |
| Hooks | `hooks/<provider>/<name>/` | Provider-specific | config.json |
| Commands | `commands/<provider>/<name>/` | Provider-specific | varies |

**Universal types** work with any AI tool. **Provider-specific types** require `--provider` when importing.

## Supported Providers

| Provider | Slug | Rules | Skills | Agents | Hooks | MCP | Commands |
|----------|------|-------|--------|--------|-------|-----|----------|
| Claude Code | `claude-code` | Yes | Yes | Yes | Yes | Yes | Yes |
| Gemini CLI | `gemini-cli` | Yes | Yes | Yes | Yes | Yes | No |
| Cursor | `cursor` | Yes | No | No | No | No | No |
| Windsurf | `windsurf` | Yes | No | No | No | No | No |
| Codex | `codex` | Yes | No | Yes | No | No | Yes |
| Copilot CLI | `copilot-cli` | Yes | No | Yes | Yes | Yes | No |
| Zed | `zed` | Yes | No | No | No | Yes | No |
| Cline | `cline` | Yes | No | No | No | Yes | No |
| Roo Code | `roo-code` | Yes | No | No | No | Yes | No |
| OpenCode | `opencode` | Yes | Yes | No | No | Yes | No |
| Kiro | `kiro` | Yes | Yes | Yes | No | Yes | No |

## Format Conversion

Nesco converts between provider formats automatically during export. When metadata can't be represented in the target format, it's either embedded as prose or reported as a warning.

Key behaviors:
- **Tool name translation**: Claude Code's `Read` becomes Gemini's `read_file`, Kiro's `read`, etc.
- **Frontmatter stripping**: Providers that use plain markdown (Kiro, OpenCode) get metadata embedded as prose notes.
- **MCP merge**: Installing MCP content merges into existing provider configs (doesn't overwrite).
- **Hookless providers**: Exporting hooks to providers without hook support (Cursor, Windsurf, Zed, etc.) emits a warning instead of failing.

## Registries

Registries are git repos containing shared content. Add them with `nesco registry add <url>`, then browse their content in the TUI or with `nesco registry items`.

```bash
nesco registry add https://github.com/user/my-registry.git
nesco registry sync
nesco registry items --type skills
```

Registry content appears in the TUI with a `[registry-name]` badge.

## Directory Layout

```
project/
├── skills/           # Shared skills (git-tracked)
├── agents/           # Shared agents
├── rules/
│   └── claude-code/  # Provider-specific rules
├── mcp/              # MCP server configs
├── local/            # Local items (gitignored)
│   ├── skills/
│   └── rules/
└── .nesco/
    └── config.json   # Project config (providers, registries)
```

Items in `local/` are not committed to git. Use the TUI's promote feature to share them.

## Sandbox (Linux)

Wrap AI CLI tools in bubblewrap sandboxes for filesystem, network, and environment isolation:

```bash
nesco sandbox check claude-code    # Verify prerequisites
nesco sandbox run claude-code      # Launch sandboxed session
nesco sandbox allow-domain npm.org # Add to network allowlist
nesco sandbox info                 # Show effective config
```

Config changes made by the AI tool inside the sandbox are diffed and require approval before being applied back.
