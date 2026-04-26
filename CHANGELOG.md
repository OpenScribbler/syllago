# Changelog

All notable changes to syllago are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

## [0.10.1] - 2026-04-26

### Fixed

- Release workflow's Homebrew tap update now fails loudly when the Aembit credential step returns an empty token. Previously `continue-on-error: true` masked the failure and the release silently shipped without updating `OpenScribbler/homebrew-tap`, leaving `brew upgrade syllago` stuck on an older version.
- `Create GitHub Release` step is now idempotent (`gh release view` → `edit` + `upload --clobber` if the release already exists, otherwise `create`). Re-running the release workflow for an existing tag no longer fails with `a release with the same tag name already exists`.
- Homebrew formula update is now a no-op (early `exit 0`) when the formula is already up to date, so re-runs against an unchanged tag don't fail with `nothing to commit`.

### Added

- `workflow_dispatch` trigger on the release workflow so the pipeline can be re-run for an existing tag without amending the tag or pushing a new one. Use `gh workflow run release.yml --ref vX.Y.Z`.

## [0.10.0] - 2026-04-26

### Added

- MOAT reference implementation: Sigstore + Rekor verification, GitHub OIDC numeric-ID pinning, `--signing-identity` / `--signing-issuer` / `--signing-repository-id` / `--signing-repository-owner-id` flags on `registry add`, three trust tiers (`DUAL-ATTESTED`, `SIGNED`, `UNSIGNED`), bundled allowlist with auto-pinning for `OpenScribbler/syllago-meta-registry`, lockfile, revocation handling (4 reasons), 90/180/365-day trusted-root staleness cliff, `MOAT_001`–`MOAT_009` error codes explainable via `syllago explain`
- `syllago moat sign` for self-publishing registries (registry signing profile + per-item attestations)
- Auto-detect MOAT-capable registries from bundled allowlist or `manifest_uri` in `registry.yaml`
- `--min-trust` flag for per-call trust-policy floor
- `--trusted-root` flag for per-registry trusted-root override
- TUI Trust glyphs on Library rows and Registry cards; reusable Trust Inspector modal (`[t]` hotkey) on both pages
- Rules splitter: detect monolithic rule files and split on H2/H3/H4 with literal-marker heuristic, BOM/CRLF/trailing-whitespace normalization, decorative-HR exclusion, MUST/SHOULD/MAY casing preservation, `@import` preservation
- `syllago add --from <path> --split` and the corresponding TUI Provider source flow with auto-detection, heuristic radio step, multi-select review, per-rule rename overrides
- `syllago install --method=append` for monolithic-file install with empty-file create and trailing-newline repair
- Rulestore with `.source/` capture and `.history/` chain of normalized hashes; orphan-history-file load is a hard error
- Install verification: `installcheck` package classifies targets as `Clean`/`Modified`/`Edited`/`Missing`; `--on-clean` and `--on-modified` flags; `installUpdateModal` and `installModifiedModal` per-state TUI modals; library Installed column shows per-target status
- `provmon` source-hash drift detection with `--fail-on=warn|error` policy
- `syllago capmon backfill` subcommand
- TUI Config tab with Settings / Sandbox / System sub-tabs, full mouse parity
- Multi-provider loadouts (`syllago-starter` ships emitters for Claude Code, Gemini CLI, Codex, Pi from one manifest)
- Hook script scanner chain with external-subprocess adapter interface (Semgrep / ShellCheck)
- MCP install paths for Cursor and Windsurf
- `content-format.json` and `syllago-yaml-schema.json` emitted as release artifacts for docs sync
- Telemetry properties: `verification_state`, `decision_action`, `discovery_candidate_count`, `selected_count`, `split_method`, `scope`

### Changed

- TUI add wizard merges Discovery and Triage into a single split-pane step
- Review step lists items grouped by content type with section headers
- Registry add/sync/remove orchestration unified between CLI and TUI through shared `registryops` package
- `cli/internal/tui_v1` legacy package retired
- Personal-import scrub of in-repo `content/` (kept: `syllago-starter` loadout, cross-provider starter rules `concise-comments` and `no-placeholders`, `code-review` skill, hook benchmark corpus)
- Documentation accuracy pass on README, ROADMAP, ARCHITECTURE, CONTRIBUTING, SECURITY

### Fixed

- `syllago registry remove` now removes the entry from all config sources
- Triage and review preview rendering for symlinked content and MCP entries
- Three discovery-triage TUI bugs (right-arrow navigation, hook risk surfacing, agent discovery)
- Registry-card stale glyph no longer wraps the card title onto a second line
- Uninstalling an item from a drilled-in registry no longer routes to registry-remove
- Empty preview when drilling into a MOAT-materialized item
- `syllago registry add` now refuses to overwrite an existing clone destination
- Trust column reading every fresh registry as Stale (staleness label casing bug)
- Wizard discovery paths for split rules and loose file-based content types
- Phantom CLI command names in user-facing error messages and MOAT suggestion strings
- System tab now uses `DetectProviders()` instead of `AllProviders()`
- Snapshot symlink TOCTOU + backup hash verification on restore

### Security

- `go-tuf/v2` pinned to v2.4.1 (patches three CVEs in the sigstore-go init chain)
- Two test-only data races eliminated under `go test -race`

### Removed

- Cursor `commands` content type support — Cursor 1.6 added slash commands but the runtime contract did not match what syllago was emitting; shipping the broken plumbing was worse than removing it

## [0.9.0] - 2026-04-17

### Added

- Roo Code: `commands` content type support (`.roo/commands/*.md`)
- Amp: `hooks` content type support (JSON merge into `.amp/settings.json` under `amp.hooks`)
- Cline: `skills` content type support (`~/.cline/skills/<slug>/SKILL.md`)
- Cline: `commands` content type support mapping Cline Workflows (`.clinerules/workflows/*.md`)
- Windsurf: `commands` content type support mapping Cascade Workflows (`.windsurf/workflows/*.md`)
- `docs/provider-formats/crush.yaml` (new file): rules, skills, mcp capability data
- Cursor capability data for all 6 supported content types (previously skills-only)
- Zed capability data for rules and mcp (previously skills-only)
- OpenCode capability data for rules, agents, commands, mcp (previously skills-only)
- `CheckCoverage()` library function + `TestCoverageNoDrift` test asserting agreement across `cli/internal/provider/`, `docs/provider-formats/`, and `docs/provider-sources/`. Full assertion set gated by `SYLLAGO_COVERAGE_STRICT=1`.

### Fixed

- Amp hooks file format now reports `json` (was defaulting to `markdown`)
- Amp hooks config location corrected from `.amp/hooks.json` to `.amp/settings.json`
- Source manifests for cursor, gemini-cli, opencode, roo-code, windsurf, zed, cline, and crush reconciled against verified upstream runtime support

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
