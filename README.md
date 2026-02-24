<div align="center">

# Nesco

<pre>
░████████   ░███████   ░███████   ░███████   ░███████
░██    ░██ ░██    ░██ ░██        ░██    ░██ ░██    ░██
░██    ░██ ░█████████  ░███████  ░██        ░██    ░██
░██    ░██ ░██               ░██ ░██    ░██ ░██    ░██
░██    ░██  ░███████   ░███████   ░███████   ░███████
</pre>

</div>

A CLI and TUI for managing AI coding tool content. Browse, install, and export skills, agents, prompts, rules, hooks, commands, and MCP configs across Claude Code, Cursor, Windsurf, Codex, and Gemini CLI. Content is automatically converted between provider formats when you install or export.

## Getting Started

### 1. Clone the repo

We recommend cloning to `~/.local/src/` so it stays out of your project directories:

```bash
mkdir -p ~/.local/src
git clone https://github.com/OpenScribbler/nesco.git ~/.local/src/nesco
```

This location becomes your local content library. The `nesco` CLI reads from it directly — no need to move or copy the repo elsewhere.

### 2. Build the CLI

Requires Go 1.25.5 or later.

```bash
cd ~/.local/src/nesco
make build
```

This builds the `nesco` binary inside the `cli/` directory. You only need to do this once — after the initial build, `nesco` keeps itself up to date automatically (see [Auto-rebuild](#auto-rebuild) below).

### 3. Add it to your PATH

Add this line to your shell profile so `nesco` is available in every session:

**Bash** (`~/.bashrc`):

```bash
echo 'export PATH="$HOME/.local/src/nesco/cli:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Zsh** (`~/.zshrc`):

```bash
echo 'export PATH="$HOME/.local/src/nesco/cli:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

Then verify it works:

```bash
nesco version
```

## What It Does

A centralized repository of reusable content for AI coding tools — custom instructions, agent definitions, prompt templates, hook scripts, and MCP server configs. The `nesco` TUI lets you browse, preview, and install content into any supported tool. When you install or export, Nesco detects which tools you have, converts formats between providers as needed (e.g., Cursor MDC to Claude Code Markdown), and places files in the correct locations.

The `nesco export` command copies content from your local `my-tools/` directory into a provider's install location, handling cross-provider format conversion automatically. The `nesco add` command imports content from external sources (local filesystem or git repos) into the catalog, canonicalizing provider-specific formats on ingest.

## The TUI

Running `nesco` with no arguments launches a full terminal UI for browsing and managing content.

### Layout

The interface uses a persistent sidebar + content panel layout. The sidebar lists content categories (Skills, Agents, Prompts, MCP, Apps, Rules, Hooks, Commands), and the content panel shows item lists, detail views, or workflow screens.

### Navigation

| Key | Action |
|-----|--------|
| `Up`/`Down` or `j`/`k` | Navigate lists and scroll |
| `PgUp`/`PgDn` | Jump a full viewport |
| `Enter` | Open item / confirm action |
| `Esc` | Go back one level |
| `Tab`/`Shift+Tab` | Switch focus between sidebar and content |
| `/` | Search (live filtering with match count) |
| `?` | Toggle keyboard shortcut help |
| `Home`/`End` | Jump to first/last item |
| `Ctrl+N`/`Ctrl+P` | Next/previous item in detail view |

### Mouse Support

Click to select sidebar items, content list items, detail tabs, action buttons, import/update/settings options, and breadcrumb links. Scroll wheel works in all scrollable areas.

### Detail View

Each content item has a tabbed detail view:

- **Overview** — Description, metadata, tags, and provider compatibility status.
- **Files** — Browse the item's file tree, open files in a scrollable viewer, copy content, or save to disk.
- **Install** — Check/uncheck target providers, then press `i` to install. Environment variable setup for MCP servers with inline `.env` editing.

### Import

Import content from the local filesystem or a git repository:

1. Choose source (Local filesystem or Git URL)
2. Pick content type (or auto-detect for universal types)
3. Browse and select files with inline content detection and file preview
4. Name the item and confirm

If the destination already exists, a **conflict resolution screen** shows a `git diff`-style unified diff with colored additions/removals and hunk headers. Overwrite with `y` or cancel with `Esc`. Batch imports step through each conflict individually.

### Settings

Configure the repository root path, content library location, and default providers. Changes auto-save on exit.

### Update

Check for updates, preview what's new (commit log + diffstat), and pull — all from within the TUI.

## Commands

| Command | Description |
|---------|-------------|
| `nesco` | Launch the TUI |
| `nesco init` | Initialize nesco for a project (creates `.nesco/config.json`) |
| `nesco add` | Add content to the catalog from local filesystem or git repos |
| `nesco export` | Export items from `my-tools/` to a provider's install location |
| `nesco import` | Read existing AI tool configs into the canonical model (read-only) |
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

### Example Workflows

```bash
# Add content from a git repo
nesco add --from https://github.com/user/repo.git --type skills --name my-skill

# Export your local content to Claude Code
nesco export --to claude-code

# Export only rules to Cursor
nesco export --to cursor --type rules

# See what a provider already has configured
nesco import --from claude-code
nesco import --from cursor --type rules --preview
```

## Supported Tools

| Tool        | Rules | Skills | Agents | Commands | Hooks | MCP |
|-------------|:-----:|:------:|:------:|:--------:|:-----:|:---:|
| Claude Code |   ✓   |   ✓    |   ✓    |    ✓     |   ✓   |  ✓  |
| Gemini CLI  |   ✓   |   ✓    |   ✓    |    ✓     |   ✓   |  ✓  |
| Codex       |   ✓   |   —    |   —    |    ✓     |   —   |  —  |
| Cursor      |   ✓   |   —    |   —    |    —     |   —   |  —  |
| Windsurf    |   ✓   |   —    |   —    |    —     |   —   |  —  |

## Repository Structure

```
nesco/
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

Each content item is a directory (or file) with a `.nesco.yaml` metadata file:

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

## Security

Nesco does not operate any registry or marketplace. The built-in content comes from
the [nesco-tools](https://github.com/OpenScribbler/nesco-tools) repository, which you
can audit directly.

**Third-party registries are unverified.** When you run `nesco registry add <url>`,
you are trusting the owner of that repository. Review the content before installing
anything from it.

**Hooks and MCP configs can execute arbitrary code.** A hook is a shell script that
runs automatically in your AI coding session. An MCP server is a process that your AI
tool connects to. Before installing either, read the source.

The nesco maintainers are not affiliated with and accept no liability for any
third-party registry or its content.

## License

Apache 2.0 — see [LICENSE](LICENSE) for full text.
