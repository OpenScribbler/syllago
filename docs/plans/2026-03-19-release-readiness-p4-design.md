# Release Readiness Phase 4: Repo Files + README

*Design date: 2026-03-19*
*Status: Design Complete*
*Phase: 4 of 5*
*Dependencies: Phase 1 (SECURITY.md accuracy), Phase 2 + 3 (README documents new features)*

## Overview

Create all missing community/security repo files and do a full README rewrite. This is the "first impression" phase — everything a developer sees when they land on the GitHub repo.

## Context

The repo is missing standard open-source community files. The README is ~40% accurate with deprecated command names, missing providers, and no mention of loadouts. The new logo will be created by Holden separately — design doc uses a placeholder.

---

## Work Items

### 1. SECURITY.md

Model after phyllotaxis. Syllago-specific threat surface:

**In-scope vulnerabilities:**
- Malicious content packages (rules, skills, agents, hooks executing arbitrary code)
- Path traversal during import/export operations
- MCP config injection via JSON merge
- Registry trust model (community registries are unverified third-party content)
- Hooks and MCP servers execute arbitrary code by design — user must trust the source
- Symlink-based file disclosure or escape

**Trust model section:**
- Registries are git repositories cloned over HTTPS. Syllago does not verify registry content integrity beyond what git provides.
- App install scripts from registries require explicit user confirmation before execution (Phase 1 fix).
- Built-in content is signed by the syllago maintainers. Registry content is not.
- Users should review content before installing, especially hooks and MCP configs.

**Contact and response:**
- Email: openscribbler.dev@pm.me
- 48-hour acknowledgment target
- 7-day fix target
- No bug bounty (pre-revenue project)

### 2. VERSIONING.md

Model after phyllotaxis.

- Semantic versioning (semver.org)
- Pre-1.0 convention: minor versions are additive, patch versions are fixes
- Post-1.0: standard semver (breaking = major, additive = minor, fix = patch)
- Release process: bump `VERSION` file → PR → merge → `/release` skill handles tag + build + publish
- Release checklist: docs sync verification, commands.json freshness, install.sh test

### 3. CHANGELOG.md

Keep a Changelog format (keepachangelog.com). Backfill from v0.5.0 forward.

Versions to backfill:
- v0.5.0 — Distribution, 11 providers, sandbox, meta-content
- v0.5.x — Patch releases (if any)
- v0.6.0 — Loadouts (35 beads, 7 phases)
- v0.6.x — Subsequent fixes

Categories per version: Added, Changed, Fixed, Security, Deprecated, Removed.

Source: GitHub release notes in `releases/` directory and git log.

### 4. `.github/workflows/pr-policy.yml`

Auto-close external PRs with a friendly redirect to issue templates.

**Allowlist:** holdenhewett, OpenScribblerOwner, dependabot[bot]

**Message:**
```
Thank you for your interest in contributing to syllago!

This project doesn't accept external pull requests at this time. Instead, please:
1. Open an issue describing the bug or feature
2. Include reproduction steps for bugs, or a use case description for features

We review all issues and may implement your suggestion in a future release.
```

### 5. ARCHITECTURE.md

Dual-audience: human developers AND LLMs navigating the codebase.

**Structure:**

```markdown
# Syllago Architecture

## Overview
[2-3 sentence description of what syllago is and how it's built]

## Package Map

### cmd/syllago/
CLI entry point. Cobra command definitions. Each file = one command.
Commands call into internal/ packages for business logic.

### internal/add/
Content discovery and addition. Used by both CLI `add` command and TUI import wizard.

### internal/catalog/
Content scanning, indexing, and querying. The source of truth for what's in the library.

### internal/config/
Configuration management. PathResolver for custom provider locations.

### internal/converter/
Hub-and-spoke format conversion. Claude Code is canonical format.
All conversions go: source → canonical → target.

### internal/installer/
Provider-specific installation logic. JSON merge for hooks/MCP, filesystem for others.

### internal/loadout/
Loadout engine: manifest building, validation, apply, snapshot/restore.

### internal/output/
CLI output formatting. StructuredError type. JSON/text/quiet modes.

### internal/provider/
Provider detection and path resolution for 11 supported tools.

### internal/registry/
Git-based registry management. Clone, sync, manifest parsing.

### internal/sandbox/
Bubblewrap-based process isolation for AI CLI tools.

### internal/tui/
Bubble Tea TUI application. Components, wizards, modals.
Calls into internal/ packages for business logic.

## Data Flow

Content In → Scanner → Canonical Format → Converter → Provider Format → Installer → Installed

## Conversion Model

Hub-and-spoke with Claude Code as canonical format:
- Import: Provider format → Canonicalize → Store
- Export: Load → Convert to target → Install

## Key Conventions

- CLI commands contain business logic (or call shared packages)
- TUI calls the same logic and adds interactive chrome
- Hooks and MCP use JSON merge; all other types use filesystem
- Tests: table-driven with t.Run(), t.TempDir() for fixtures, no mocking library
```

### 6. CONTRIBUTING.md Update

Add "Development" section:

```markdown
## Development

### Requirements
- Go 1.25+
- Make

### Building and Testing
See `cli/Makefile`:
- `make build` — Build local binary to ~/.local/bin/syllago
- `make test` — Run test suite (includes go vet)
- `make fmt` — Format code

### Code Organization
See [ARCHITECTURE.md](ARCHITECTURE.md) for the full package map.

### Testing Patterns
- Table-driven tests with `t.Run()`
- `t.TempDir()` for fixtures
- No mocking library — hand-crafted stubs
- TUI components: golden file visual regression tests
  - Regenerate baselines: `cd cli && go test ./internal/tui/ -update-golden`

### Why No External PRs?
Syllago is maintained by a small team using AI-augmented development.
External PRs create coordination overhead that doesn't fit our workflow.
We welcome issues — bug reports, feature requests, and use case descriptions
help us prioritize what to build next.
```

### 7. Full README Rewrite

**Approach:** Full rewrite from clean outline. Preserve positioning narrative. Placeholder for new logo (Holden creating separately).

**Outline:**

1. **Logo + tagline** — [PLACEHOLDER: New logo] + "The package manager for AI coding tool content"
2. **One-paragraph pitch** — What syllago does in 3 sentences
3. **Quick start** — Install → run TUI → add content from existing tools
4. **Key features** — TUI browsing, cross-provider conversion, registries, loadouts, sandbox
5. **Supported providers** — Table with all 11 providers
6. **Content types** — Table with 7 types (Skills, Agents, MCP, Rules, Hooks, Commands, Loadouts)
7. **Conversion compatibility** — Positive-framing table showing what works where
8. **Commands** — Updated table with all current commands
9. **Keyboard shortcuts** — Complete TUI shortcut reference
10. **VHS demo GIFs** — [PLACEHOLDER: 3 demos from Phase 5]
11. **Installation** — curl, Homebrew, go install, from source
12. **Configuration** — Config file location, registries, custom paths
13. **Accessibility** — Honest + helpful: CLI + --json for screen readers, NO_COLOR support, roadmap for accessible TUI mode
14. **Security** — Brief summary pointing to SECURITY.md
15. **Contributing** — Brief summary pointing to CONTRIBUTING.md
16. **License**

**Conversion compatibility table** (positive framing):

| Content Type | Coverage | Notes |
|---|---|---|
| Rules | All 11 providers | Format differs but content fully preserved |
| Skills | All 11 providers | Metadata rendering varies by provider |
| Agents | All 11 providers | Codex uses TOML format (auto-converted) |
| MCP configs | 9 providers | Zed uses `context_servers` key (handled automatically) |
| Hooks | 3 providers | Other providers don't have hook systems |
| Commands | Claude Code | Provider-specific feature |
| Loadouts | Claude Code (v1) | Additional provider emitters planned |

**Accessibility section:**

> All operations are available via CLI commands with `--json` output for scripting and assistive technology. The TUI uses ANSI rendering; for screen reader users, we recommend CLI commands directly. Colors can be disabled with `NO_COLOR=1` or `--no-color`. We're exploring a screen-reader-compatible TUI mode — [feedback welcome](link).

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| README approach | Full rewrite | Current is ~40% accurate; incremental would miss structural issues |
| Repo file template | Phyllotaxis | Consistent style across Holden's projects |
| CHANGELOG backfill | From v0.5.0 | First "real" release; earlier versions were internal |
| PR policy | Auto-close with redirect | Small team, AI-augmented workflow |
| ARCHITECTURE audience | Human + LLM | Differentiator for open-source project |
| Conversion table framing | Positive (what works) | Not "data loss" — confident conversion with transparency |
| Screen reader TUI | Roadmap item | Mention in README, not a v1 deliverable |
| Logo | Placeholder | Holden creating separately |

---

## Out of Scope

- New logo/favicon (Holden creating separately)
- VHS demo tape creation (Phase 5)
- Error code documentation content (Phase 5)
- Full screen-reader TUI mode (roadmap)
