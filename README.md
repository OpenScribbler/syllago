<div align="center">

<img src="logo.svg" alt="Syllago" width="600">

**Convert, bundle, and share AI coding tool content across providers.**

</div>

AI coding tools like Claude Code, Cursor, Gemini CLI, Copilot, and Amp each store rules, skills, agents, and other configurations in their own format. If you've built up a library of custom instructions in one tool, moving them to another means manual copy-pasting and format translation. Syllago automates that.

Syllago maintains a central library of your content -- rules, skills, agents, hooks, MCP server configs, and commands. When you add content from one provider, syllago converts it to its own format. When you install it to another provider, syllago converts it again to the target's native format. The conversion is automatic and bidirectional.

## Prerequisites

- **OS:** Linux, macOS, or Windows (via WSL)
- **Git:** Required for registry operations and content sharing

## Installation

### Homebrew (macOS)

```bash
brew tap OpenScribbler/tap
brew install syllago
```

### Install script (Linux, macOS, Windows)

```bash
curl -fsSL https://raw.githubusercontent.com/OpenScribbler/syllago/main/install.sh | sh
```

Downloads the latest release binary, verifies the SHA-256 checksum, and installs to `~/.local/bin`. Override the install location with `INSTALL_DIR`:

```bash
INSTALL_DIR=/usr/local/bin sh install.sh
```

### go install

```bash
go install github.com/OpenScribbler/syllago/cli/cmd/syllago@latest
```

### From source

Requires Go 1.25+.

```bash
git clone https://github.com/OpenScribbler/syllago.git
cd syllago
make setup    # configure git hooks (gofmt pre-commit check)
make build
```

## Quick Start

**Scenario:** You have Claude Code rules and skills you want to use in Cursor and Gemini CLI.

```bash
# Step 1: See what content Claude Code has
syllago add --from claude-code
# Discovered content from Claude Code:
#   Rules (3): my-coding-rules, typescript-standards, security-policy
#   Skills (2): research-skill, code-review
#   ...

# Step 2: Add all of it to your syllago library
syllago add --all --from claude-code

# Step 3: Install a rule to Cursor (auto-converts to .mdc format)
syllago install my-coding-rules --to cursor

# Step 4: Install a skill to Gemini CLI (auto-converts to Gemini's SKILL.md format)
syllago install research-skill --to gemini-cli
```

Or skip the CLI and browse everything interactively:

```bash
syllago   # launches the TUI
```

## How It Works

Syllago uses three verbs that mirror a simple workflow:

| Step | Command | What it does |
|------|---------|-------------|
| **Add** | `syllago add --from <provider>` | Discovers content in a provider's config directory and copies it into your syllago library |
| **Install** | `syllago install <item> --to <provider>` | Takes a library item and writes it to a provider's config directory, converting the format automatically |
| **Remove** | `syllago remove <item>` | Removes content from your library (and uninstalls from all providers) |

Content in your library is provider-neutral. You add once, install anywhere.

## Features

- **Cross-provider conversion** -- add content from one tool, install to another. Syllago handles format differences (Cursor's `.mdc`, Codex's TOML, Kiro's JSON, Amp's `AGENTS.md`, etc.)
- **Interactive TUI** with card grids, search, mouse support, and keyboard navigation
- **Loadouts** -- bundle multiple content items together and apply them as a unit. Preview with `--try` and revert cleanly
- **Git-based registries** -- browse and install shared content from any compatible git repository
- **Sandbox** -- run AI CLI tools in isolated environments with filesystem, network, and environment filtering (Linux)
- **Registry privacy** -- content from private registries is automatically detected and prevented from being published to public registries
- **`--json` output** on all commands for scripting and CI integration

## Supported Providers

| Tool | Rules | Skills | Agents | MCP | Hooks | Commands |
|------|:-----:|:------:|:------:|:---:|:-----:|:--------:|
| Claude Code | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Gemini CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Copilot CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Codex | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Cursor | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Amp | ✅ | ✅ | - | ✅ | - | - |
| Windsurf | ✅ | ✅ | - | ✅ | ✅ | - |
| Kiro | ✅ | ✅ | ✅ | ✅ | ✅ | - |
| OpenCode | ✅ | ✅ | ✅ | ✅ | - | ✅ |
| Roo Code | ✅ | ✅ | ✅ | ✅ | - | - |
| Cline | ✅ | - | - | ✅ | ✅ | - |
| Zed | ✅ | - | - | ✅ | - | - |

## Content Types

| Type | What it is |
|------|------------|
| Rules | System prompts and custom instructions (e.g., "always use TypeScript strict mode") |
| Skills | Multi-file workflow packages (e.g., a code review workflow with templates and scripts) |
| Agents | AI agent definitions and personas (e.g., a "security reviewer" agent) |
| MCP | Model Context Protocol server configurations |
| Hooks | Event-driven automation scripts that run before/after tool actions |
| Commands | Custom slash commands (e.g., `/deploy`) |
| Loadouts | Bundles of the above, applied as a unit |

## Conversion Compatibility

| Content Type | Coverage | Notes |
|---|---|---|
| Rules | All providers | Format differs but content fully preserved |
| Skills | All providers | Metadata rendering varies by provider |
| Agents | All providers | Codex uses TOML format (auto-converted) |
| MCP configs | Most providers | Zed uses `context_servers` key (handled automatically) |
| Hooks | Claude Code, Gemini CLI, Copilot CLI, Codex, Cursor, Windsurf, Kiro, Cline | Canonical interchange format (`docs/spec/hooks-v1.md`) with degradation strategies. Amp, OpenCode, Zed, Roo Code lack hook systems |
| Commands | Claude Code, Gemini CLI, Copilot CLI, Codex, Cursor, OpenCode | Slash command definitions (e.g., `/deploy`) |
| Loadouts | Claude Code (v1) | Additional provider emitters planned |

## Commands

| Command | Description |
|---------|-------------|
| `syllago` | Launch the interactive TUI |
| `syllago add` | Discover and add content from a provider |
| `syllago install` | Activate library content in a provider |
| `syllago uninstall` | Deactivate content from a provider |
| `syllago remove` | Remove content from your library |
| `syllago convert` | Convert library content to a provider format |
| `syllago share` | Contribute library content to a team repo |
| `syllago publish` | Contribute library content to a registry |
| `syllago loadout` | Apply, create, and manage loadouts |
| `syllago registry` | Manage git-based content registries |
| `syllago sandbox` | Run AI CLI tools in isolated sandboxes (Linux) |
| `syllago sync-and-export` | Sync registries then install content to a provider (CI/automation) |
| `syllago init` | Initialize syllago for a project |
| `syllago create` | Scaffold a new content item |
| `syllago inspect` | Show details about a content item |
| `syllago list` | List content items in the library |
| `syllago config` | View and edit configuration |
| `syllago update` | Update syllago to the latest release |
| `syllago info` | Show capabilities (providers, formats) |
| `syllago completion` | Generate shell autocompletion scripts |
| `syllago version` | Print version |

### Global Flags

```
--json        Output in JSON format
--no-color    Disable color output
-q, --quiet   Suppress non-essential output
-v, --verbose Verbose output
```

### Example Workflows

```bash
# Add all content from Claude Code
syllago add --from claude-code

# Add only rules from Cursor
syllago add rules --from cursor

# Install a skill to Gemini CLI (auto-converts format)
syllago install my-skill --to gemini-cli

# Browse and install from a shared team registry
syllago registry add https://github.com/your-team/ai-configs.git
syllago registry sync
syllago registry items --type skills

# Apply a loadout temporarily (auto-reverts when done)
syllago loadout apply my-loadout --try

# Convert content for a specific provider without installing
syllago convert my-rule --to windsurf
```

## TUI Keyboard Shortcuts

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
| `i` | Install selected item |
| `u` | Uninstall selected item |
| `a` | Add content (context-specific) |
| `r` | Remove item |
| `c` | Copy content to clipboard |
| `H` | Toggle hidden items |

Mouse support: click to select cards, items, tabs, breadcrumbs, and modal buttons. Scroll wheel works in all scrollable areas.

## Configuration

Syllago uses two config files:

- **Project:** `.syllago/config.json` -- providers and registries for this project
- **Global:** `~/.syllago/config.json` -- default providers, global library settings

Run `syllago init` for first-time setup. The init wizard helps you select providers and add registries.

### Custom Provider Paths

If your AI tools are installed at non-default locations:

```bash
syllago config paths set claude-code --base-dir /custom/path
syllago config paths show
```

## Accessibility

All operations are available via CLI commands with `--json` output for scripting and assistive technology. The TUI uses ANSI rendering; for screen reader users, we recommend CLI commands directly. Colors can be disabled with `NO_COLOR=1` or `--no-color`. We're exploring a screen-reader-compatible TUI mode -- [feedback welcome](https://github.com/OpenScribbler/syllago/issues).

## Security

Syllago does not operate any registry or marketplace. Third-party registries are unverified -- review content before installing, especially hooks and MCP configs which execute code by design.

See [SECURITY.md](SECURITY.md) for the full security policy, threat model, and how to report vulnerabilities.

## Roadmap

- Hook signing and verification (Sigstore + GPG)
- Hook execution audit logging
- Registry trust tiers (trusted/verified/community)
- Batch hook migration (`syllago convert --batch`)
- Dual-format hook distribution (`syllago export --dual`)
- Content update mechanism (`syllago update`)
- Additional loadout provider emitters beyond Claude Code
- VHS demo GIFs for README

## Contributing

Contributions are welcome as issues -- see [CONTRIBUTING.md](CONTRIBUTING.md) for how to submit bug reports, feature ideas, and improvement suggestions.

## License

Apache 2.0 -- see [LICENSE](LICENSE) for full text.
