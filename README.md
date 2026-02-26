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

A CLI and TUI for managing AI coding tool content. Browse, install, and export skills, agents, prompts, rules, hooks, commands, and MCP configs across Claude Code, Gemini CLI, Codex, Cursor, Windsurf, and Copilot CLI. Content is automatically converted between provider formats when you install or export.

## Installation

### Install script (Linux, macOS, Windows)

```bash
curl -fsSL https://raw.githubusercontent.com/OpenScribbler/nesco/main/install.sh | sh
```

Downloads the latest release binary for your platform, verifies the SHA-256 checksum, and installs to `~/.local/bin`. Override the install location with `INSTALL_DIR`:

```bash
INSTALL_DIR=/usr/local/bin sh install.sh
```

### Homebrew (macOS, Linux)

```bash
brew install openscribbler/tap/nesco
```

### From source

Requires Go 1.25 or later.

```bash
git clone https://github.com/OpenScribbler/nesco.git ~/.local/src/nesco
cd ~/.local/src/nesco
make build
```

See [Development](#development) for build details.

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
| `nesco registry` | Manage git-based content registries (`add`, `remove`, `list`, `sync`, `items`) |
| `nesco sandbox` | Run AI CLI tools in bubblewrap sandboxes (Linux only) |
| `nesco update` | Update nesco to the latest release |
| `nesco info` | Show capabilities (`providers`, `formats`) |
| `nesco completion` | Generate shell autocompletion (bash, zsh, fish, powershell) |
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
| Copilot CLI |   ✓   |   —    |   ✓    |    ✓     |   ✓   |  ✓  |
| Codex       |   ✓   |   —    |   —    |    ✓     |   —   |  —  |
| Cursor      |   ✓   |   —    |   —    |    —     |   —   |  —  |
| Windsurf    |   ✓   |   —    |   —    |    —     |   —   |  —  |

## Sandbox

Nesco can wrap AI CLI tools in [bubblewrap](https://github.com/containers/bubblewrap) sandboxes that restrict filesystem access, network egress, and environment variables. Linux only.

```bash
# Run Claude Code in a sandbox
nesco sandbox run claude-code

# Check prerequisites
nesco sandbox check claude-code

# Manage the domain allowlist
nesco sandbox allow-domain example.com
nesco sandbox deny-domain example.com
nesco sandbox domains
```

The sandbox provides:

- **Filesystem isolation** — The project directory is writable; everything else is read-only or hidden
- **Network egress proxy** — Only allowed domains can be reached (provider API + ecosystem registries)
- **Environment filtering** — Only explicitly allowed env vars are forwarded
- **Config change review** — Provider config changes made inside the sandbox are diffed and require approval before being applied back

Requires bubblewrap >= 0.4.0 and socat >= 1.7.0.

## Registries

Registries are git repositories of nesco content that you can browse and install from.

```bash
# Add a registry
nesco registry add https://github.com/OpenScribbler/nesco-tools.git

# List registered registries
nesco registry list

# Sync (pull latest)
nesco registry sync

# Browse items across all registries
nesco registry items
```

Registries also appear in the TUI — browse by category, preview content, and install with one keypress.

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

## Updating

**Release builds** (installed via install script or Homebrew): run `nesco update` or use the Update screen in the TUI. Both use the same updater — downloads the latest release, verifies the checksum, and replaces the binary.

**Dev builds** (built from source): `nesco` detects when its own source files have changed and automatically rebuilds before running. Just `git pull` — the next invocation handles the rest.

## Development

Requires Go 1.25 or later.

```bash
make build      # Build the nesco binary (output: cli/nesco)
make test       # Run tests
make fmt        # Format Go source
make vet        # Run go vet
make build-all  # Cross-compile for all 6 targets (linux/darwin/windows × amd64/arm64)
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to contribute. Nesco accepts ideas, not code — open an issue using one of the structured templates and describe what you'd like to see.

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
