# Romanesco

A CLI and TUI for managing AI coding tool content and scanning codebases for context. Browse and install skills, agents, prompts, rules, hooks, commands, and MCP configs across Claude Code, Cursor, Windsurf, Codex, and Gemini CLI. Scan any project to generate context files that help AI agents produce correct code.

## Getting Started

### 1. Clone the repo

We recommend cloning to `~/.local/src/` so it stays out of your project directories:

```bash
mkdir -p ~/.local/src
git clone https://github.com/holdenhewett/romanesco.git ~/.local/src/romanesco
```

This location becomes your local content library. The `nesco` CLI reads from it directly — no need to move or copy the repo elsewhere.

### 2. Build the CLI

Requires Go 1.25+.

```bash
cd ~/.local/src/romanesco
make build
```

This builds the `nesco` binary inside the `cli/` directory. You only need to do this once — after the initial build, `nesco` keeps itself up to date automatically (see [Auto-rebuild](#auto-rebuild) below).

### 3. Add it to your PATH

Add this line to your shell profile so `nesco` is available in every session:

**Bash** (`~/.bashrc`):

```bash
echo 'export PATH="$HOME/.local/src/romanesco/cli:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Zsh** (`~/.zshrc`):

```bash
echo 'export PATH="$HOME/.local/src/romanesco/cli:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

Then verify it works:

```bash
nesco version
```

## What It Does

Romanesco has two sides:

**Content management.** A centralized repository of reusable content for AI coding tools — custom instructions, agent definitions, prompt templates, hook scripts, and MCP server configs. The `nesco` TUI lets you browse, preview, and install content into any supported tool. When you install, Romanesco detects which tools you have, converts formats as needed, and places files in the correct locations.

**Codebase scanning.** `nesco scan` runs detectors against your project to discover tech stack, dependencies, build commands, conventions, and "surprises" (gotchas that trip up AI agents). Results are emitted as context files in each provider's native format — Markdown for Claude Code, MDC for Cursor, etc. Drift detection lets you track how your codebase changes over time.

## Commands

Running `nesco` with no arguments launches the TUI. Subcommands handle scanning, drift detection, and configuration:

| Command | Description |
|---------|-------------|
| `nesco` | Launch the TUI for browsing and installing content |
| `nesco init` | Initialize nesco for a project (creates `.nesco/config.json`) |
| `nesco scan` | Run detectors and generate context files for configured providers |
| `nesco drift` | Compare current codebase state against the stored baseline |
| `nesco baseline` | Save current state as the baseline for drift detection |
| `nesco import` | Read existing AI tool configs into the canonical model (read-only) |
| `nesco parity` | Compare AI tool configs across providers and report gaps |
| `nesco config` | Manage provider selection (`list`, `add`, `remove`) |
| `nesco info` | Show capabilities (`providers`, `formats`) |
| `nesco version` | Print version |

### Global Flags

All commands accept these flags:

```
--json        Output in JSON format
--no-color    Disable color output
-q, --quiet   Suppress non-essential output
-v, --verbose Verbose output
```

### Scanning Workflow

```bash
# First time: initialize and scan
nesco init              # Detect tools, create .nesco/config.json
nesco scan              # Run detectors, emit context files, set baseline

# Later: check for drift
nesco drift             # Compare current state against baseline
nesco drift --ci        # CI mode — exit code 3 if drift detected

# After reviewing drift, accept the new state
nesco baseline          # Update baseline to current state

# Re-scan after changes
nesco scan              # Re-run detectors and update context files
```

### Import and Parity

```bash
# See what a provider already has configured
nesco import --from claude-code
nesco import --from cursor --type rules --preview

# Compare content across providers
nesco parity
```

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

## Auto-rebuild

After the initial `make build`, `nesco` detects when its own source files have changed and automatically rebuilds before running. You don't need to re-run `make build` after pulling updates — just `git pull` and the next invocation handles it.

## Development

```bash
make build      # Build the nesco binary (output: cli/nesco)
make test       # Run tests
make fmt        # Format Go source
make vet        # Run go vet
make build-all  # Cross-compile for linux/darwin/windows amd64 + linux/darwin arm64
```

## License

Apache 2.0 — see [LICENSE](LICENSE) for full text.
