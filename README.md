# Romanesco

A terminal UI for managing content across AI coding tools. Install skills, agents, prompts, rules, hooks, commands, and MCP server configs for Claude Code, Cursor, Windsurf, Codex, and Gemini CLI — all from one central repository.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/holdenhewett/romanesco.git
cd romanesco

# Build the TUI
make build

# Launch Romanesco
./cli/nesco
```

## What It Does

Romanesco gives you a centralized content repository for AI coding tools. Store reusable pieces like custom instructions, agent definitions, prompt templates, and tool configurations in one place. The `nesco` TUI lets you browse, preview, and install content into any supported AI coding tool.

When you install content, Romanesco detects which tools you have, converts formats as needed, and places files in the correct locations for each tool.

## Supported Tools

| Tool | Rules | Commands | Hooks | MCP |
|------|-------|----------|-------|-----|
| Claude Code | ✓ | ✓ | ✓ | ✓ |
| Cursor | ✓ | — | — | ✓ |
| Windsurf | ✓ | — | — | ✓ |
| Codex | ✓ | ✓ | — | — |
| Gemini CLI | ✓ | ✓ | ✓ | — |

## Repository Structure

```
romanesco/
├── skills/          # Multi-file skill packages
├── agents/          # Agent definitions
├── prompts/         # Prompt templates
├── rules/           # Per-tool rule files
│   ├── claude-code/
│   ├── cursor/
│   ├── windsurf/
│   ├── codex/
│   └── gemini-cli/
├── hooks/           # Event-driven hooks
│   ├── claude-code/
│   └── gemini-cli/
├── commands/        # Custom slash commands
│   ├── claude-code/
│   ├── codex/
│   └── gemini-cli/
├── mcp/             # MCP server configurations
├── apps/            # Full application packages
├── memory/          # Context files for AI assistants
├── templates/       # Scaffolding for new content
├── my-tools/        # Local content (gitignored)
└── cli/             # Go source code for nesco
```

## Adding Your Own Content

Each content item is a directory (or file) with a `.romanesco.yaml` metadata file:

```yaml
name: my-skill
description: What this skill does
version: "1.0"
tags:
  - productivity
  - code-review
```

Place your content in the appropriate category directory and `nesco` will discover it automatically. Use the `my-tools/` directory for local content you don't want to commit to the repository.

## Building from Source

```bash
cd cli
make build      # Build the nesco binary
make test       # Run tests
make install    # Install to $GOPATH/bin
```

## License

Apache 2.0 — see [LICENSE](LICENSE) for full text.
