# Nesco

```
░████████   ░███████   ░███████   ░███████   ░███████      ░███████  ░██ ░██
░██    ░██ ░██    ░██ ░██        ░██    ░██ ░██    ░██    ░██    ░██ ░██ ░██
░██    ░██ ░█████████  ░███████  ░██        ░██    ░██    ░██        ░██ ░██
░██    ░██ ░██               ░██ ░██    ░██ ░██    ░██    ░██    ░██ ░██ ░██
░██    ░██  ░███████   ░███████   ░███████   ░███████      ░███████  ░██ ░██
```

```
                                 ##+
                                ###+#
                              ###++++++      ######
                             ###++++++++  ##########
                  ++++++-   ###++++++++#############
                 ++++++-...##++++++++#########+++++++
                 +++++--..##+++++++#######++++++++#+++++++++#+
                 +++++--.#+++++++######+++++#++++++++++++++++++
      #######+++#+++++-.###+++++#####++++#+++++++++++++++++++++
      #######+++#++++--.#++++++#####++#++++++++---------------+
      #######+++#++++--.#+++++###+++++++++------.............-
      ########+++++++--##++++####+#+++----##################..
       #######++++++++-##+++###+++++--##++++++++++++++++++++########
       #######+++#++++-##+++####++++##+++++++++++++++++++++++++++#+####
        #######+++++++-##++###+++-#+++###############+++++++++++++#####
         #######++#++++.#++###++-++######+++############++++++++++++####
        ++#######++#+++-#++##+++#+###++++++####++##########++++++++++++
      +++++#######++++++-#++#++++###+-+++++++++###++#########++++++++#
    ####++++#######++#+++-#+##+++#+#++##+##++++++####++########+++++
   ######+++++######++++++-#++#+++#++##++#+#-++++++####++########-+
   ########+++++######+++++++#++########++#+##+++++++###+++#######
    #########+++++#######++#+++++++++###++##+##-++++++###+++######
      +#########++++########+#++++#####+++##+##--++++++###+++###
         ##########+#+++##############-+++##++##-+++++++###++#
          .-############+#+++++######+++++##++##-++++++++###
         ---....##################+-+++++##++###-++++++++####
         --------...-#########---++++#++###++###--++++++++####
        ++++++---+-----------++++++#++###+++###--++++++++####
        +++++++++++++++++++++++++#++++###++++###--++++++++#####
        #++++++++++++++++++++##+++++####++++###---+++++++++####
           ####+++++++++##++++++++#####++++####--++++++++++####
                 ++++++++++++++######+++++####.--+++++++++#+#
                  ++++++++#########++++++####.---+++++
                  ###############++++++#####----++++++
                  ############+++++++######.---++++++
                   ########    ++++++#####    +++++++
                    ##          +#######+
                                  #####
                                   ##
```

A CLI and TUI for managing AI coding tool content and scanning codebases for context. Browse and install skills, agents, prompts, rules, hooks, commands, and MCP configs across Claude Code, Cursor, Windsurf, Codex, and Gemini CLI. Scan any project to generate context files that help AI agents produce correct code.

## Getting Started

### 1. Clone the repo

We recommend cloning to `~/.local/src/` so it stays out of your project directories:

```bash
mkdir -p ~/.local/src
git clone https://github.com/holdenhewett/nesco.git ~/.local/src/nesco
```

This location becomes your local content library. The `nesco` CLI reads from it directly — no need to move or copy the repo elsewhere.

### 2. Build the CLI

Requires Go 1.25+.

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

Nesco has two sides:

**Content management.** A centralized repository of reusable content for AI coding tools — custom instructions, agent definitions, prompt templates, hook scripts, and MCP server configs. The `nesco` TUI lets you browse, preview, and install content into any supported tool. When you install, Nesco detects which tools you have, converts formats as needed, and places files in the correct locations.

**Codebase scanning.** `nesco scan` runs detectors against your project to discover tech stack, dependencies, build commands, conventions, and "surprises" (gotchas that trip up AI agents). Results are emitted as context files in each provider's native format — Markdown for Claude Code, MDC for Cursor, etc. Drift detection lets you track how your codebase changes over time.

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

Subcommands handle scanning, drift detection, and configuration:

| Command | Description |
|---------|-------------|
| `nesco` | Launch the TUI |
| `nesco init` | Initialize nesco for a project (creates `.nesco/config.json`) |
| `nesco scan` | Run detectors and generate context files for configured providers |
| `nesco drift` | Compare current codebase state against the stored baseline |
| `nesco baseline` | Save current state as the baseline for drift detection |
| `nesco import` | Read existing AI tool configs into the canonical model (read-only) |
| `nesco parity` | Compare AI tool configs across providers and report gaps |
| `nesco add` | Add content items to the catalog (non-interactive import) |
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

## Codebase Detectors

`nesco scan` runs 30+ detectors against your project to discover:

- **Tech stack** — Languages, frameworks, runtime versions
- **Dependencies** — Package managers, dependency files, version constraints
- **Build system** — Build commands, test runners, CI configuration
- **Conventions** — Linting rules, test patterns, directory structure, env variables
- **Surprises** — Go replace directives, Rust feature flags, Python async patterns, TypeScript strictness, competing frameworks, namespace packages, nil interface returns, and other gotchas that trip up AI agents

Results are emitted as context files in each provider's native format (Markdown for Claude Code, MDC for Cursor, etc.) so AI tools automatically get project-specific awareness.

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

## License

Apache 2.0 — see [LICENSE](LICENSE) for full text.
