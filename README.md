<div align="center">

<!-- New logo coming soon -->
<pre>
 █████████             ████  ████
███░░░░░███           ░░███ ░░███
░███    ░░░  █████ ████ ░███  ░███   ██████    ███████  ██████
░░█████████ ░░███ ░███  ░███  ░███  ░░░░░███  ███░░███ ███░░███
 ░░░░░░░░███ ░███ ░███  ░███  ░███   ███████ ░███ ░███░███ ░███
 ███    ░███ ░███ ░███  ░███  ░███  ███░░███ ░███ ░███░███ ░███
░░█████████  ░░███████  █████ █████░░████████░░███████░░██████
 ░░░░░░░░░    ░░░░░███ ░░░░░ ░░░░░  ░░░░░░░░  ░░░░░███ ░░░░░░
              ███ ░███                        ███ ░███
             ░░██████                        ░░██████
              ░░░░░░                          ░░░░░░
</pre>

**Convert, bundle, and share AI coding tool content across providers.**

</div>

Syllago is a content manager for AI coding tools. It maintains a library of reusable content -- rules, skills, agents, hooks, MCP server configs, and commands -- and handles format conversion automatically when you install to a provider or export to another tool. Bundle content into **loadouts** that apply as a unit, preview with `--try`, and revert cleanly. Browse and manage everything through an interactive TUI or automate with CLI commands and `--json` output.

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/OpenScribbler/syllago/main/install.sh | sh

# Launch the TUI
syllago

# Add content from your existing tools
syllago add --from claude-code

# Install to another provider
syllago install my-rule --to cursor
```

## Features

- **Interactive TUI** with card grids, search, mouse support, and keyboard navigation
- **Cross-provider conversion** with hub-and-spoke architecture (Claude Code as canonical format)
- **Git-based registries** -- browse and install from any compatible content repository
- **Loadouts** -- curated content bundles that apply, preview (`--try`), or revert as a unit
- **Sandbox** -- run AI CLI tools in bubblewrap isolation with filesystem, network, and env filtering (Linux)
- **`--json` output** on all commands for scripting and CI integration

## Supported Providers

| Tool | Rules | Skills | Agents | MCP | Hooks | Commands |
|------|:-----:|:------:|:------:|:---:|:-----:|:--------:|
| Claude Code | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Gemini CLI | ✅ | ✅ | ✅ | ✅ | ✅ | - |
| Copilot CLI | ✅ | - | ✅ | ✅ | ✅ | - |
| Codex | ✅ | - | ✅ | - | - | ✅ |
| Cursor | ✅ | - | - | - | - | - |
| Windsurf | ✅ | - | - | - | - | - |
| Zed | ✅ | - | - | - | - | - |
| Cline | ✅ | - | - | ✅ | - | - |
| Roo Code | ✅ | - | - | ✅ | - | - |
| OpenCode | ✅ | ✅ | - | ✅ | - | - |
| Kiro | ✅ | ✅ | ✅ | ✅ | - | - |

## Content Types

| Type | Description |
|------|-------------|
| Rules | System prompts and custom instructions |
| Skills | Multi-file workflow packages |
| Agents | AI agent definitions and personas |
| MCP | MCP server configurations |
| Hooks | Event-driven automation scripts |
| Commands | Custom slash commands |
| Loadouts | Curated content bundles |

## Conversion Compatibility

| Content Type | Coverage | Notes |
|---|---|---|
| Rules | All providers | Format differs but content fully preserved |
| Skills | All providers | Metadata rendering varies by provider |
| Agents | All providers | Codex uses TOML format (auto-converted) |
| MCP configs | Most providers | Zed uses `context_servers` key (handled automatically) |
| Hooks | Claude Code, Gemini CLI, Copilot CLI | Other providers don't have hook systems |
| Commands | Claude Code | Provider-specific feature |
| Loadouts | Claude Code (v1) | Additional provider emitters planned |

## Commands

| Command | Description |
|---------|-------------|
| `syllago` | Launch the interactive TUI |
| `syllago add` | Discover and add content from a provider |
| `syllago import` | Import content from a provider, path, or git URL |
| `syllago install` | Activate library content in a provider |
| `syllago uninstall` | Deactivate content from a provider |
| `syllago remove` | Remove content from your library |
| `syllago convert` | Convert library content to a provider format |
| `syllago share` | Contribute library content to a team repo |
| `syllago publish` | Contribute library content to a registry |
| `syllago loadout` | Apply, create, and manage loadouts |
| `syllago registry` | Manage git-based content registries |
| `syllago sandbox` | Run AI CLI tools in bubblewrap sandboxes (Linux) |
| `syllago init` | Initialize syllago for a project |
| `syllago create` | Scaffold a new content item |
| `syllago inspect` | Show details about a content item |
| `syllago list` | List content items in the catalog |
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
# Add content from your Claude Code setup
syllago add --from claude-code

# Add only rules from Cursor
syllago add --from cursor --type rules

# Install a skill to Gemini CLI (auto-converts format)
syllago install my-skill --to gemini-cli

# Browse registry content
syllago registry add https://github.com/your-team/ai-configs.git
syllago registry sync
syllago registry items --type skills

# Apply a loadout temporarily (auto-reverts when session ends)
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

## Installation

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
make build
```

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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Syllago accepts ideas, not code -- open an issue using one of the structured templates.

## License

Apache 2.0 -- see [LICENSE](LICENSE) for full text.
