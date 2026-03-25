---
name: syllago-guide
description: Complete reference for using syllago to manage AI coding tool content
---

# Syllago Guide

Syllago is the package manager for AI coding tool content. It lets you browse, install, and share skills, rules, agents, hooks, MCP configs, and more across 11 AI coding tools.

## Core Workflow

```
syllago import --from <provider>    # Bring content in from a provider
syllago export --to <provider>      # Send content out to any provider
syllago                             # Browse everything in the TUI
```

Content lives in `local/` after import. Syllago handles format conversion automatically — a Claude Code skill becomes a Kiro steering file, a Cursor rule becomes a Windsurf rule, etc.

## Commands

| Command | Purpose |
|---------|---------|
| `syllago` | Launch the interactive TUI |
| `syllago init` | Initialize syllago for a project (creates `.syllago/config.json`) |
| `syllago import --from <provider>` | Discover and import content from a provider |
| `syllago export --to <provider>` | Convert and install content to a provider |
| `syllago registry add <url>` | Add a git-based content registry |
| `syllago registry sync` | Pull latest from all registries |
| `syllago registry items` | Browse available registry content |
| `syllago sandbox run <provider>` | Run a provider in a bubblewrap sandbox (Linux) |
| `syllago sandbox check <provider>` | Verify sandbox prerequisites |
| `syllago config list` | Show configured providers |
| `syllago update` | Self-update to latest release |
| `syllago info` | Show content types, providers, and capabilities |

### Import Flags

```bash
syllago import --from claude-code                  # All content
syllago import --from claude-code --type skills    # Only skills
syllago import --from cursor --name my-rule        # Specific item by name
syllago import --from claude-code --preview        # Read-only discovery
```

### Export Flags

```bash
syllago export --to cursor                         # All content to Cursor
syllago export --to kiro --type skills             # Only skills to Kiro
syllago export --to gemini-cli --name research     # Specific item
syllago export --to codex --llm-hooks generate     # Generate hook wrapper scripts
```

## Content Types

| Type | Directory | Scope | Main File |
|------|-----------|-------|-----------|
| Skills | `skills/<name>/` | Universal | SKILL.md |
| Agents | `agents/<name>/` | Universal | AGENT.md |
| Prompts | `prompts/<name>/` | Universal | PROMPT.md |
| MCP Servers | `mcp/<name>/` | Universal | config.json |
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

Syllago converts between provider formats automatically during export. When metadata can't be represented in the target format, it's either embedded as prose or reported as a warning.

Key behaviors:
- **Tool name translation**: Claude Code's `Read` becomes Gemini's `read_file`, Kiro's `read`, etc.
- **Frontmatter stripping**: Providers that use plain markdown (Kiro, OpenCode) get metadata embedded as prose notes.
- **MCP merge**: Installing MCP content merges into existing provider configs (doesn't overwrite).
- **Hookless providers**: Exporting hooks to providers without hook support (Cursor, Windsurf, Zed, etc.) emits a warning instead of failing.

## Registries

Registries are git repos containing shared content. Add them with `syllago registry add <url>`, then browse their content in the TUI or with `syllago registry items`.

```bash
syllago registry add https://github.com/user/my-registry.git
syllago registry sync
syllago registry items --type skills
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
└── .syllago/
    └── config.json   # Project config (providers, registries)
```

Items in `local/` are not committed to git. Use the TUI's promote feature to share them.

## Sandbox (Linux)

Wrap AI CLI tools in bubblewrap sandboxes for filesystem, network, and environment isolation:

```bash
syllago sandbox check claude-code    # Verify prerequisites
syllago sandbox run claude-code      # Launch sandboxed session
syllago sandbox allow-domain npm.org # Add to network allowlist
syllago sandbox info                 # Show effective config
```

Config changes made by the AI tool inside the sandbox are diffed and require approval before being applied back.
