# Changelog

All notable changes to syllago are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

## [0.7.0] - 2026-03-23

### Added

- 7-verb CLI redesign: `add`, `remove`, `install`, `uninstall`, `share`, `publish`, `sync-and-export`
- `syllago convert` command for cross-provider format conversion
- Global content library at `~/.syllago/content/` with `[GLOBAL]` badge in TUI and CLI
- TUI card grid pages for Library, Loadouts, and Registries
- Full mouse support: click-to-select, click-away modals, scroll wheel
- Toast notification system with animated spinners for async operations
- Breadcrumbs, search (`/`), and Tab focus on all pages
- Help overlay (`?`) covering all screen types
- `syllago registry create` command with scaffold, examples, and auto `git init`
- Native registry indexing via `registry.yaml` items list
- Configurable provider path overrides via `syllago config paths`
- Loadout creation wizard (multi-step TUI)
- Interactive provider selection during `syllago init`
- JSON validation and security callouts in import review step
- `syllago-quickstart` built-in skill (getting started guide)
- `syllago-starter` built-in loadout (bundles guide, quickstart, import, author)
- Context-aware welcome card with 3 journey paths based on provider detection
- No-providers warning in TUI and CLI when AI tools not detected
- `StructuredError` type with 17 error code constants
- SECURITY.md, VERSIONING.md, CHANGELOG.md, ARCHITECTURE.md

### Changed

- `syllago import` deprecated in favor of `syllago add`
- `syllago export` removed in favor of `syllago sync-and-export`
- `syllago promote` removed in favor of `syllago share` / `syllago publish`
- Registry names use `owner/repo` format
- All modals standardized to width 56
- Help text consolidated into global footer bar

### Removed

- `Prompts` and `Apps` content types (migrate to `commands/` or `skills/`)

### Fixed

- MCP merge now extracts server entries from nested config.json format
- Install command respects per-type path overrides from config
- Hook install copies script files to stable location
- Detail view width uses full terminal width
- Registry 0-items bug after add
- Modal input cursor movement with arrow keys
- Orphaned clone directory when config save fails after registry add
- Security notice mentioned "prompts" instead of "commands"
- Codex agent TOML and Kiro agent JSON round-trip conversion

### Security

- CI gates: golangci-lint v2, race detector, go mod tidy check, commands.json freshness
- All GitHub Actions pinned to full-length commit SHAs

## [0.6.1] - 2026-03-01

### Added

- `syllago _gendocs` internal command generating commands.json manifest
- `syl` alias for `syllago`
- New ASCII branding

### Changed

- Renamed project from nesco to syllago across entire codebase

## [0.6.0] - 2026-03-01

### Added

- Loadouts: bundle rules, hooks, skills, agents, MCP servers into `loadout.yaml`
- `syllago loadout apply/remove/list/status/create` commands
- Snapshot system for all-or-nothing apply/revert
- `--try` mode (temporary, auto-reverts) and `--keep` mode (permanent)
- `installed.json` tracking replaces embedded JSON markers in provider configs
- 5 new providers: Zed, Cline, Roo Code, OpenCode, Kiro (total: 11)
- Meta-content: `syllago-guide` and `syllago-author` items
- Built-in badge for content bundled with syllago

### Changed

- 9th content type (Loadouts) added to content model, catalog, and TUI
- MCP merge support extended to all providers
- Improved conversion fidelity across all provider pairs

### Fixed

- Hardcoded home directory paths removed
- `TestScanRealRepo` skipped in CI
- CI job renamed to "Go Test + Build"

### Security

- All GitHub Actions pinned to full-length commit SHAs

## [0.5.0] - 2026-02-26

### Added

- Sandbox: bubblewrap-based process isolation for AI CLI tools (Linux)
  - `syllago sandbox run/check/info` commands
  - Domain, port, and env allowlists
  - Config diff-and-approve on session exit
  - TUI sandbox settings screen
- Distribution: pre-built binaries for 6 platforms (linux/darwin/windows x amd64/arm64)
- `curl | sh` install script with SHA-256 verification
- Homebrew tap: `brew install openscribbler/tap/syllago`
- `syllago update` self-update command
- Golden file visual regression tests for TUI components
- Interactive `/release` skill with two-phase flow
- Release guard hook blocking accidental tag creation

### Changed

- Module path migrated to `github.com/OpenScribbler/syllago`
- README overhauled with installation, sandbox, and registry docs

### Fixed

- Sandbox config diff not detecting deleted files
- `isHighRiskDiff` now checks both original and staged content

### Security

- Sandbox high-risk key detection covers `"commands"` alongside `"mcpServers"` and `"hooks"`
- `golang.org/x/net` bumped to 0.38.0 (security fix)

[Unreleased]: https://github.com/OpenScribbler/syllago/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/OpenScribbler/syllago/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/OpenScribbler/syllago/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/OpenScribbler/syllago/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/OpenScribbler/syllago/releases/tag/v0.5.0
