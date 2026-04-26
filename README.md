<div align="center">

<img src="logos/social-preview.svg" alt="Syllago" width="600">

[![CI](https://github.com/OpenScribbler/syllago/actions/workflows/ci.yml/badge.svg)](https://github.com/OpenScribbler/syllago/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/OpenScribbler/syllago)](https://github.com/OpenScribbler/syllago/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/OpenScribbler/syllago/cli)](https://goreportcard.com/report/github.com/OpenScribbler/syllago/cli)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/OpenScribbler/syllago/badge)](https://scorecard.dev/viewer/?uri=github.com/OpenScribbler/syllago)

**Convert, bundle, and share AI coding tool content across providers.**

</div>

## Why Syllago?

AI coding tools like Claude Code, Cursor, Gemini CLI, Copilot, and Amp each store rules, skills, agents, and configurations in their own format. Switching tools -- or rolling out configurations across a team -- means manual copy-pasting and format translation. Syllago automates that: add content once, install it anywhere, and organize it into shareable collections called loadouts.

- **Portable content** -- Add from one tool, install to another. Syllago canonicalizes everything into its own intermediate format, then converts to the target provider's native format automatically.
- **Shared configurations** -- Distribute AI tool content through git-based registries. Push updates once, and your team syncs automatically.
- **Governed distribution** -- Use privacy gates to keep internal content out of public registries. Evaluate untrusted hooks and MCP configs in sandbox isolation before they run. Pipe `--json` output into your CI pipelines and audit workflows.
- **One library** -- Your content lives in a single, provider-neutral library. No duplication across tools, no drift between copies.

For a deeper introduction, see the [documentation site](https://syllago.dev).

## Prerequisites

- **OS:** Linux, macOS, or Windows (native via MSYS2/Git Bash, or WSL)
- **Git:** Required for registry operations and content sharing
- **Go 1.25+:** Only required for `go install` or building from source

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

```bash
git clone https://github.com/OpenScribbler/syllago.git
cd syllago
make setup    # configure git hooks (gofmt pre-commit check)
make build
```

## Quick Start

**Scenario:** You have Claude Code rules and skills you want to use in Cursor and Gemini CLI.

```bash
# Step 1: Initialize your syllago content repository (first time only)
syllago init

# Step 2: See what content Claude Code has
syllago add --from claude-code
# Discovered content from Claude Code:
#   Rules (3): my-coding-rules, typescript-standards, security-policy
#   Skills (2): research-skill, code-review
#   ...

# Step 3: Add all of it to your syllago library
syllago add --all --from claude-code

# Step 4: Install a rule to Cursor (auto-converts to .mdc format)
syllago install my-coding-rules --to cursor

# Step 5: Install a skill to Gemini CLI (auto-converts to Gemini's SKILL.md format)
syllago install research-skill --to gemini-cli
```

Or skip the CLI and browse everything interactively:

```bash
syllago   # launches the TUI
```

## How It Works

Syllago uses a **hub-and-spoke conversion model**. When you add content from a provider, syllago converts it into its own canonical format -- a provider-neutral intermediate representation. When you install that content to a different provider, syllago converts from canonical to the target's native format. This means adding support for a new provider only requires two conversions (to/from canonical), not N-to-N mappings between every provider pair.

Specification files define how each content type looks in canonical form. See the [canonical keys reference](https://syllago.dev/reference/canonical-keys/) for details.

### The library

Everything you add goes into your **library** (`~/.syllago/content/`). Your library is your single source of truth -- provider-neutral, deduplicated, and ready to install to any supported provider.

### Commands

Syllago's CLI is organized around a content lifecycle:

| Step | Command | What it does |
|------|---------|-------------|
| **Discover** | `syllago add --from <provider>` | Scans a provider's config directory and shows you what content exists |
| **Add** | `syllago add --all --from <provider>` | Copies content into your library, converting to canonical format |
| **Install** | `syllago install <item> --to <provider>` | Writes a library item to a provider's config directory in its native format |
| **Convert** | `syllago convert <item> --to <provider>` | Converts content for a provider without installing it |
| **Refresh** | `syllago refresh` | Pulls updates from registries you subscribe to |
| **Share** | `syllago share <item>` | Contributes library content to a team repo or registry |

For the full command reference, see [syllago.dev/using-syllago/cli-reference/](https://syllago.dev/using-syllago/cli-reference/).

## Supported Providers

| Tool | Rules | Skills | Agents | MCP | Hooks | Commands |
|------|:-----:|:------:|:------:|:---:|:-----:|:--------:|
| Claude Code | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Gemini CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Copilot CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Codex | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Factory Droid | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Cursor | ✅ | ✅ | ✅ | ✅ | ✅ | — |
| Kiro | ✅ | ✅ | ✅ | ✅ | ✅ | — |
| Windsurf | ✅ | ✅ | — | ✅ | ✅ | ✅ |
| Cline | ✅ | ✅ | — | ✅ | ✅ | ✅ |
| OpenCode | ✅ | ✅ | ✅ | ✅ | — | ✅ |
| Roo Code | ✅ | ✅ | ✅ | ✅ | — | ✅ |
| Pi | ✅ | ✅ | — | — | ✅ | ✅ |
| Amp | ✅ | ✅ | — | ✅ | ✅ | — |
| Crush | ✅ | ✅ | — | ✅ | — | — |
| Zed | ✅ | — | — | ✅ | — | — |

Run `syllago info providers` to see this matrix for the version of syllago you have installed.

## Content Types

| Type | What it is |
|------|------------|
| Rules | System prompts and custom instructions (e.g., "always use TypeScript strict mode") |
| Skills | Multi-file workflow packages (e.g., a code review workflow with templates and scripts) |
| Agents | AI agent definitions and personas (e.g., a "security reviewer" agent) |
| MCP Servers | Model Context Protocol server configurations |
| Hooks | Event-driven automation scripts that run before/after tool actions |
| Commands | Custom slash commands (e.g., `/deploy`) |

Use `syllago compat <item>` to see which providers support a specific content item and what changes during conversion.

For the full provider-by-content-type matrix, see [Compare providers](https://syllago.dev/reference/compare-providers/).

## Conversion Example

Syllago handles provider-specific format differences automatically. For example, a Cursor rule (`.mdc`) becomes a Claude Code rule (`.md`):

```
# Input (Cursor)                    # Output (Claude Code)
---                                 ---
description: TS conventions         paths:
alwaysApply: false                      - '*.ts'
globs: "*.ts, *.tsx"                    - '*.tsx'
---                                 ---

Use strict TypeScript.              Use strict TypeScript.
```

Cursor uses `globs` as a comma-separated string with `alwaysApply`. Claude Code uses a `paths` YAML array. The body content passes through unchanged.

Try it yourself: `syllago convert ./my-rule.mdc --from cursor --to claude-code`

For more conversion walkthroughs across providers (Windsurf, Copilot, Codex, Kiro, etc.), see [Format conversion](https://syllago.dev/using-syllago/format-conversion/).

## Collections

Collections let you organize and distribute content beyond individual items.

### Library

Your library (`~/.syllago/content/`) stores everything you've added in syllago's canonical format. Install any item to any supported provider from here. The library lives locally on your machine.

### Loadouts

A **loadout** bundles multiple content items and applies them as a unit. Package a complete AI tool setup (rules + skills + hooks + MCP configs) for a specific workflow or role. Pass `--try` to preview a loadout temporarily -- syllago auto-reverts when you're done.

### Registries

A **registry** is a git repository that distributes syllago content. Push curated configurations to a registry; your team picks them up with `syllago registry sync` (refreshes the registry index) and `syllago refresh` (applies content updates to the local library). Mark registries as public or private — syllago's privacy gates block content from private registries from reaching public ones. Sign your registry and content with [MOAT](#trust-and-supply-chain) so consumers can verify provenance, not just retrieve files.

## Features

- **Cross-provider conversion** — Add content from one tool, install to another. Syllago handles format differences (Cursor's `.mdc`, Codex's TOML, Kiro's JSON, Amp's `AGENTS.md`, etc.)
- **Interactive TUI** — Browse, search, install, and manage content with card grids, mouse support, and keyboard navigation
- **Verifiable trust (MOAT)** — Signed registries and signed publisher content using Sigstore + Rekor. Configurable trust tiers (Unsigned / Signed / Dual-Attested) with install-gate enforcement. See [Trust and Supply Chain](#trust-and-supply-chain).
- **Sandbox** — Run AI CLI tools in isolated environments with filesystem, network, and environment filtering (Linux)
- **Registry privacy** — Syllago detects content from private registries and blocks it from reaching public ones
- **`--json` output** — Pipe any command's output into scripts, CI pipelines, or other automation

## Trust and Supply Chain

AI tool content runs as code: hooks execute on every tool call, MCP servers shell out to external programs, and rules steer the model itself. When that content travels through git registries, "I trust the publisher" needs to be more than a vibe. Syllago implements **MOAT** — *Model for Origin, Attestation, and Trust* — to make registry and content integrity verifiable end-to-end.

The MOAT specification is community-owned and lives at **[github.com/OpenScribbler/moat](https://github.com/OpenScribbler/moat)**. Syllago is the reference implementation.

### Two layers of attestation

**Signed registries.** Each registry has a pinned signing identity bound to its source repository's GitHub OIDC numeric ID, not a mutable repo path. Renaming or transferring a registry repo to a different owner does not transfer its trust — the pinned identity won't match, and `syllago registry sync` blocks the update. New registries pin via TOFU on first add, or via a bundled allowlist for well-known operators.

**Signed publisher content.** Individual content items are signed by their publishers using [Sigstore](https://www.sigstore.dev/) keyless signing — short-lived Fulcio certificates issued from OIDC identities (typically a GitHub Actions workflow), with the signature recorded in the [Rekor](https://docs.sigstore.dev/logging/overview/) transparency log. There are no long-lived keys to rotate or protect; revocation is verifiable through transparency-log lookups.

### Trust tiers

| Tier              | What it means                                                                                                                      |
|-------------------|------------------------------------------------------------------------------------------------------------------------------------|
| **Unsigned**      | Content has no publisher attestation. Install only if you trust the registry operator.                                             |
| **Signed**        | Publisher attestation present and verified against Rekor.                                                                          |
| **Dual-Attested** | Publisher signature *and* registry signature both verify, and the per-item Rekor log index pins to the registry's signing profile. |

The install gate enforces a configurable minimum tier. CI pipelines get specific exit codes for missing signatures, identity mismatches, and Rekor revocations so failures are unambiguous.

### Operator commands

```bash
syllago moat trust status                     # show trusted root + per-registry identities
syllago registry add <url>                    # pin signing identity (TOFU or allowlist)
syllago registry add <url> --signing-identity # explicit identity for custom registries
syllago registry sync                         # verify signatures, refuse mismatched updates
```

Detailed walkthroughs: [MOAT overview](https://syllago.dev/moat/), [Trust tiers](https://syllago.dev/moat/trust-tiers/), [Pinning a signing identity](https://syllago.dev/moat/registry-add-signing-identity/).

### Other supply-chain practices

- **Signed syllago releases** — release binaries signed with cosign keyless. Verify with `cosign verify-blob --bundle checksums.txt.bundle checksums.txt`.
- **SHA-256 checksums** — `checksums.txt` ships with every release.
- **Pinned CI dependencies** — all GitHub Actions pinned to full-length commit SHAs.
- **Automated vulnerability scanning** — `govulncheck` runs in CI; Dependabot patches dependency CVEs.
- **Registry privacy gates** — content tagged from a private registry is blocked from reaching a public one.
- **Sandbox isolation** — `syllago sandbox run` executes AI CLI tools inside [bubblewrap](https://github.com/containers/bubblewrap) with filesystem, network, and environment filtering (Linux).

See [SECURITY.md](SECURITY.md) for the full security policy, threat model, and how to report vulnerabilities.

## CLI Reference

The lifecycle table at [Commands](#commands) covers the everyday verbs (`add`, `install`, `convert`, `share`). Beyond those, syllago ships commands for trust verification (`moat`), registry updates (`refresh`), inspection (`inspect`, `list`, `compat`, `info`, `doctor`), automation (`sync-install`), and more.

For the full command reference with flags, examples, and exit codes, see **[syllago.dev/using-syllago/cli-reference/](https://syllago.dev/using-syllago/cli-reference/)**. Every command has its own page; subcommands (e.g. `loadout apply`, `registry sync`, `sandbox run`) are documented individually.

You can also list every command locally:

```bash
syllago --help                  # top-level commands
syllago <command> --help        # flags and examples for any command
```

### Global flags

```
--json        Output in JSON format
--no-color    Disable color output
-q, --quiet   Suppress non-essential output
-v, --verbose Verbose output
```

## Interactive TUI

Run `syllago` with no arguments to launch the terminal UI. It supports keyboard navigation, mouse interaction (click cards, tabs, breadcrumbs, and modal buttons), live search (`/`), and contextual help (`?`).

For the full keyboard reference, see [The TUI](https://syllago.dev/using-syllago/tui/).

## Configuration

Syllago uses two config files: `~/.syllago/config.json` (global defaults) and `.syllago/config.json` (per-project). Run `syllago init` for first-time setup — the wizard handles provider selection and registry setup. Override default provider paths with `syllago config paths`.

For full configuration reference, see [`syllago config`](https://syllago.dev/using-syllago/cli-reference/config/) and [Core concepts](https://syllago.dev/getting-started/core-concepts/).

## Accessibility

Every operation works through CLI commands with `--json` output for scripting and assistive technology. The TUI uses ANSI rendering; if you use a screen reader, we recommend running CLI commands directly. Disable colors with `NO_COLOR=1` or `--no-color`. We're exploring a screen-reader-compatible TUI mode -- [feedback welcome](https://github.com/OpenScribbler/syllago/issues).

## Telemetry

Syllago collects anonymous usage data (command names, exit codes, provider/content-type counts — no file contents, names, paths, or registry URLs) to help prioritize work. It's opt-out, not opt-in. Disable with `syllago telemetry off`. Full details, the property catalog, and deletion process: [syllago.dev/telemetry](https://syllago.dev/reference/telemetry/) and `syllago telemetry status`.

## Roadmap

Recent and upcoming work:

- **Privacy and integrity** — registry privacy gates, content integrity hashes, audit trail *(done)*
- **MOAT trust model** — signed registries, signed publisher content, trust tiers *(done)*
- **Distribution** — bulk install, `add --from shared`, provider-to-provider conversion, SBOM *(done)*
- **Platform** — `syllago doctor`, enhanced `syllago info`, dependency review CI *(done)*
- **Security** — hook signing and verification, script scanning, policy engine *(next)*
- **Multi-provider loadouts** — define once, install to every detected agent or a specific list *(next)*
- **Providers** — VS Code Copilot, Qwen Code, Kimi CLI, Trae Agent, and more
- **Specs** — formal specs for all canonical formats (hooks spec is drafted)

See [ROADMAP.md](ROADMAP.md) for the full roadmap with status tracking.

## Contributing

We welcome ideas, bug reports, and feature suggestions -- open an issue to get started. We accept pull requests from [vouched contributors](https://github.com/mitchellh/vouch). See [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to get involved.

## License

Apache 2.0 -- see [LICENSE](LICENSE) for full text.
