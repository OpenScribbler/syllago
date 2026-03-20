# Syllago v1.0 Release Strategy

*Design date: 2026-02-25*
*Status: In Progress — updated 2026-03-19*

## Positioning

**Tagline:** "Tired of managing rules for every AI tool separately? Yeah, us too."

**Subline:** Browse, install, and share AI tool content across Claude Code, Cursor, Gemini CLI, and more. Automatic format conversion. Team-ready registries. No lock-in.

**Identity:** Syllago is the package manager for AI coding tool content.

### How We Differ From Rulesync

| | Rulesync | Syllago |
|---|---|---|
| **Model** | Compiler (source -> generated outputs) | Package manager (browse -> install -> share) |
| **Content** | Prescriptive (official skills, simulated features) | Platform (bring your own, share your own) |
| **Interface** | CLI only | Interactive TUI + CLI |
| **Sharing** | Git fetch from repos | Registry system with browsing and discovery |
| **Conversion** | Unidirectional (unified -> tools) | Bidirectional with data loss awareness |
| **Audience** | Individual developer workflow | Solo power users + team standardization |

### Philosophy: Platform, Not Prescription

Syllago does not prescribe what content you should use. The only built-in content is meta-tooling (skills/agents that help you use syllago itself and create content for it). Everything else comes from registries that people create and share. Syllago is the platform, not the content source.

---

## Target Users

1. **Solo power user** who uses 2-3 AI coding tools and wants consistency across them
2. **Team lead / dev tooling person** who standardizes AI tool usage across a team via registries

V1.0 must work well for both from day one.

---

## Decided Priority Order

*Decided: 2026-02-26 | Status updated: 2026-03-19*

Based on risk profiles and unblocking dependencies:

1. ~~**Distribution**~~ ✅ v0.5.0 — Binaries (6 platforms), install script, Homebrew tap, cosign signing, `syllago update`.
2. ~~**CI Pipeline**~~ ✅ v0.5.0 — `go test` + `go build` on PR and push to main.
3. ~~**Tool Coverage (11 providers)**~~ ✅ v0.5.0 + d39214c — All 11 providers: Claude Code, Gemini CLI, Cursor, Windsurf, Codex, Copilot CLI, Cline, Kiro, Roo Code, Zed, OpenCode.
4. ~~**Format Conversion Fidelity**~~ ✅ d39214c — Tool name tables for all providers, MCP merge for 9 providers, Codex multi-agent TOML, hookless provider warnings, field preservation tests.
5. ~~**Meta-Content**~~ ✅ — `syllago-guide` skill, `syllago-author` agent, built-in badge in TUI.
6. ~~**Content Model Restructure**~~ ✅ b7a415f — 42 tasks across 6 phases. `content/` directory, scanner refactor, kitchen-sink examples, CLI fixes (import/export/create/list/inspect), registry features, TUI polish.
7. ~~**SEC-001 Fix**~~ ✅ — Symlink checks in `copyFile` and `copyDir` (Lstat guard + source tree skip).
8. ~~**Sandbox Wrapper**~~ ✅ v0.5.0 — Bubblewrap isolation, egress proxy, config diff-and-approve, TUI settings.
9. ~~**Loadouts**~~ ✅ v0.6.0 — Session config packaging. 35 beads across 7 phases. CLI commands, snapshot system, TUI integration, Claude Code emitter. Design: `docs/syllago-loadouts-design.md`.
10. ~~**TUI Polish + Registry Experience**~~ ✅ — Split-view detail, wizard step machines, rebuildItems pattern, mouse support, registry indexing, golden file tests, loadout creation wizard redesign.
11. **Documentation Site (Content)** — CLI Reference infrastructure complete (`gendocs`, `commands.json`, Astro sync pipeline). Content writing remains.

---

## Remaining Work (as of 2026-03-19)

Ten of eleven workstreams are complete. One workstream remains before v1.

| # | Workstream | Status | Key Items |
|---|-----------|--------|-----------|
| ~~9~~ | ~~Loadouts~~ | ✅ Complete (v0.6.0) | 35 beads, 7 phases. CLI commands, snapshot system, TUI integration, Claude Code emitter |
| ~~2+5~~ | ~~TUI Polish + Registry Experience~~ | ✅ Complete | Split-view detail, wizard step machines, rebuildItems pattern, mouse support, native registry indexing, golden file regression tests, loadout creation wizard redesign |
| 8 | **Documentation Site Content** | In progress — infrastructure done | CLI Reference infra complete (`gendocs`, `commands.json`, Astro Starlight sync). Content writing remains: getting started, provider ref, authoring guide, loadouts guide, registry guide, sandbox guide |

**Additionally complete since 2026-03-01:** Custom provider path overrides (`InstallWithResolver`), hook script file copying, MCP merge nested config extraction, E2E provider smoke tests (Claude Code + Gemini CLI), wizard invariant testing framework.

**Release readiness:** See `docs/plans/2026-03-11-release-readiness.md` for pre-release checklist (security fixes, README rewrite, missing repo files, CI hardening).

---

## V1.0 Workstreams

### 1. Distribution ✅

**Status:** Complete (v0.5.0)

**Goal:** `curl -fsSL https://get.syllago.dev | sh` or `brew install syllago`

- ✅ Pre-built binaries for Linux/macOS/Windows (amd64 + arm64) on GitHub Releases
- ✅ One-liner install script (shell script that detects OS/arch, downloads binary)
- ✅ Homebrew tap (`brew install openscribbler/tap/syllago`)
- ✅ SHA-256 checksums + cosign signing
- ✅ `/release` skill with interactive two-phase flow + release guard hook
- ✅ `syllago update` self-update command

### 2. TUI Polish ✅

**Status:** Complete

**Goal:** Every flow feels consistent, discoverable, and professional.

- ✅ **Modal system** - Refactored for progressive disclosure (v0.5.0), old modal deleted and replaced with wizard mouse support + action buttons
- ✅ **Golden file visual regression** - Component + full-app tests at 4 terminal sizes, expanded with wizard and loadout creation golden files
- ✅ **Split-view detail** - Files/Preview tabs with click-to-select, full-width detail view
- ✅ **Wizard step machines** - `validateStep()` enforcement with invariant tests across all 5 wizards (import, loadout create, install modal, env setup modal, update)
- ✅ **rebuildItems pattern** - Context-preserving data refresh (breadcrumbs, provider filters survive install/remove/toggle)
- ✅ **Mouse support** - Click support for all modal and wizard elements
- ✅ **Loadout creation wizard** - Three-phase redesign with JSON validation, review step security callouts, empty-type handling
- ✅ **Bug fixes** - Registry 0-items bug, toast bugs, detail view width, context-specific help text, text truncation standardization
- ✅ **"Built-in" badge** - Purple `[BUILT-IN]` badge in TUI (distinct from LOCAL and registry badges)
- ✅ **Remove action** - Added to Items/Detail views, cleaned up prompts/apps references

**Why:** The TUI is what makes people say "oh, this is different." It needs to be screenshot-worthy.

### 3. Tool Coverage Expansion ✅

**Status:** Complete (v0.5.0 + d39214c)

**Goal:** Support the mainstream 8-10 tools.

All 11 providers implemented:
- ✅ Claude Code, Gemini CLI, Cursor, Windsurf, Codex, Copilot CLI (original 6)
- ✅ Cline, Kiro, Roo Code, Zed, OpenCode (added v0.5.0)

### 4. Meta-Content: The Built-in Toolkit ✅

**Status:** Complete

**Goal:** Ship 2-3 built-in items that make syllago self-bootstrapping.

- ✅ **`syllago-guide` skill** — Complete reference: all commands, content types, 11 providers, format conversion, registries, sandbox
- ✅ **`syllago-author` agent** — Persona with full canonical format reference, converter constraints, tool name table, .syllago.yaml schema, provider-specific gotchas
- ✅ **`syllago-import` skill** — Updated to reflect current CLI (removed `syllago add` references, added export examples)
- ✅ **"Built-in" badge** — Purple `[BUILT-IN]` badge in TUI items list and detail view (distinct from amber LOCAL and grey registry badges). Uses `IsBuiltin()` helper on `ContentItem` checking `.syllago.yaml` `builtin` tag.

Built-in items: `syllago-guide` (skill), `syllago-author` (agent), `syllago-import` (skill).

### 5. Registry Experience ✅

**Status:** Complete

**Goal:** Adding and browsing a registry should feel like discovering an app store.

- ✅ **Native registry indexing** - `registry.yaml` items for structured registry metadata
- ✅ **Registry browser in TUI** - Browse by category, preview content, install with one keypress, progress toasts
- ✅ **Registry create command** - Scaffolds registry with gitignore, examples, contributing guide, auto git init
- ✅ **Registry integration tests** - Kitchen-sink content tests, comprehensive registry actions test suite
- ✅ **Bug fixes** - Registry 0-items bug, modal input nav, toast display issues
- ✅ **Registry README rendering** - Description and metadata shown in TUI

**Why:** The registry IS the content story. Good registry UX -> people create and share content.

### 6. Sandbox Wrapper ✅

**Status:** Complete (v0.5.0)

**Goal:** `syllago sandbox run <provider>` wraps AI CLI tools in bubblewrap sandboxes.

- ✅ Bubblewrap-based isolation (filesystem, network, PID, IPC)
- ✅ Egress proxy with domain allowlist (auto-detected from project ecosystem)
- ✅ Copy-diff-approve for provider configs (high-risk vs low-risk change detection)
- ✅ Env var allowlist + port allowlist
- ✅ TUI sandbox settings screen
- ✅ `sandbox check`, `sandbox info`, `sandbox run` commands
- Linux-only for v1 (macOS support post-v1)

Full design: `docs/plans/2026-02-25-sandbox-wrapper-design.md`

### 7. Format Conversion Fidelity ✅

**Status:** Complete (d39214c)

**Goal:** Conversions between providers are accurate, merge-safe, and handle provider-specific quirks.

- ✅ **Tool name translation tables** — Canonical-to-provider mappings for all 11 providers. One canonical name maps to multiple provider names (e.g., Read → read_file/view/read). Reverse translation supported.
- ✅ **MCP config merge strategies** — Per-provider config paths (9 providers), JSONC comment stripping for OpenCode, Zed `context_servers` key, project-scope vs user-scope resolution.
- ✅ **Codex multi-agent format** — TOML role configs with `codex_agents.go`, canonicalize + render round-trip.
- ✅ **Hookless provider handling** — Providers without hook systems (8 of 11) emit warnings instead of errors.
- ✅ **Skill rendering consolidation** — `renderPlainMarkdownSkill` with prose embedding for metadata (used by Kiro, OpenCode).
- ✅ **Field preservation tests** — Round-trip tests for converter fidelity.

### 9. Loadouts ✅

**Status:** Complete (v0.6.0) — renamed from "Starters"

**Goal:** Package entire session configurations — rules, hooks, skills, agents, MCP servers — into shareable artifacts that work across providers.

**Full design:** `docs/syllago-loadouts-design.md`

Implemented across 35 beads in 7 phases (A through G):
- ✅ Installer refactoring and content type infrastructure (phases A+B)
- ✅ Core engine and CLI commands (phases C+D)
- ✅ TUI integration, tests, and formatting (phases E+F+G)
- ✅ Claude Code emitter (v1 scope)
- ✅ Snapshot system for backup/restore
- ✅ Loadout manifest format with `ref:` and `inline:` content references

**Post-v1 roadmap:**
- Additional provider emitters (Gemini CLI, Cursor, etc.)
- `extends` composition with deep merge
- `permissions` section (shell allow/deny, file protection)
- Model preferences
- Process wrapping for `--try` mode with launched CLI tools

### 8. Documentation Site (Must-Have)

**Goal:** Public documentation site for syllago, built with Astro Starlight, hosted in a separate repo (`syllago-docs`), with release-gated sync to the CLI repo.

**Scaffolding: DONE** (2026-02-26)

Site infrastructure is live at `https://openscribbler.github.io/syllago-docs/`. Completed:
- Repo created (`OpenScribbler/syllago-docs`)
- Astro Starlight with Flexoki theme configured
- Full sidebar IA with 25 placeholder pages (hero + 24 content)
- GitHub Actions deploy workflow (SHA-pinned)
- GitHub Pages serving
- Tagged `v0.0.1`

Design doc: `docs/plans/2026-02-26-docs-site-design.md`

**CLI Reference Infrastructure: DONE** (2026-03-19)

- `_gendocs` hidden command generates `cli/commands.json` manifest (all 52 commands with usage, flags, examples)
- Cobra `Example` fields added to all 52 CLI commands
- `syllago-docs` repo has Zod schema, `sync-commands.ts` prebuild script, MDX templates, sidebar autogeneration
- Sync pipeline fetches `commands.json` from latest GitHub release and renders individual command pages
- A new syllago release is needed before docs site shows the updated examples

**Remaining: Content writing (last step before v1 release)**

Features are now stable. Content scope:
- Getting started (install, first run, basic usage)
- Provider reference (supported tools, what content types each supports, format details)
- Content authoring guide (how to create skills, agents, rules, etc. in syllago-canonical format)
- Loadouts guide (how to create, apply, and share session configurations)
- Registry guide (how to create, publish, and consume registries)
- Sandbox guide (setup, usage, security model)
- ~~CLI reference (all commands and flags)~~ — infrastructure done, auto-generated from `commands.json`

**Release-gated sync:**
- The `/release` skill checks that `syllago-docs` has a matching version tag before proceeding with a CLI release
- No escape hatch — docs are a release requirement, not a suggestion
- Content-only fixes deploy independently anytime

**Repo landscape:**

| Repo | Purpose |
|---|---|
| `syllago` | CLI source, meta-content, design docs |
| `syllago-docs` | Public documentation site (Astro Starlight) |
| `syllago-tools` | Reference registry content |

**Why:** A tool that manages AI coding tool content needs documentation that looks professional and is easy to navigate. Writing docs last ensures we document reality, not aspirations.

---

## Explicitly NOT in V1.0

| Feature | Reason |
|---|---|
| Simulated features (rulesync-style) | Not our approach - convert what exists, don't simulate what doesn't |
| Global mode | Nice-to-have, not core to team story |
| Lockfile | Team consistency via registries + git, not lockfiles |
| Programmatic API | CLI + TUI is the interface. API later |
| Built-in MCP server | Planned but post-v1.0 |
| Dry run | Useful but not differentiating |
| Codebase scanning | Phase 2+ per existing roadmap |
| Matching rulesync's 24 tools | 10 is enough. Add more post-launch |

---

## Launch Narrative

> Every AI coding tool stores its configuration differently. Cursor uses `.mdc` files. Claude Code uses `CLAUDE.md`. Copilot uses `.instructions.md`. If your team uses more than one tool - or if different team members prefer different tools - you're manually keeping these in sync.
>
> **Syllago is the package manager for AI coding tool content.** It gives you:
>
> 1. **A TUI to browse and install** - Search skills, rules, agents, hooks, and MCP configs. Preview them. Install to any supported tool with one keypress.
> 2. **Automatic format conversion** - Install a Cursor rule into Claude Code. Export a Claude Code skill for Gemini CLI. Syllago handles the translation, and tells you if anything was lost.
> 3. **Registries for sharing** - Teams create git repos of content and register them. Everyone browses and installs from the same source, regardless of which AI tool they prefer.
> 4. **No lock-in, no prescription** - We don't tell you what rules to write. We give you a platform to share and discover them. Stop using syllago anytime - your installed content stays exactly where it is.

---

## Competitive Context

### Rulesync

Rulesync (v7.8.1) is the established tool in this space with 824 GitHub stars, 161K weekly npm downloads, and 24+ supported tools. We don't compete head-on. Our differentiators:

1. **Interactive TUI** (rulesync is CLI-only)
2. **Platform model** (rulesync prescribes; we enable)
3. **Registry system** (rulesync fetches; we browse and discover)
4. **Bidirectional conversion with data loss awareness** (rulesync generates unidirectionally)
5. **Single binary, no runtime** (rulesync requires Node.js)
6. **Team-first design** (registries + per-project config for team standardization)
7. **Sandbox isolation** (bubblewrap-based process isolation for AI CLI tools — no other tool offers this)

### ai-config (azat-io/ai-config)

Lightweight opinionated "dotfiles installer" — ships one developer's curated AI tool configs (4 agents, 9 commands, 7 skills) and deploys them via `npx`. Supports Claude Code, Codex, Gemini CLI, and OpenCode. Not a direct competitor — different layer entirely. ai-config is content with a smart installer; syllago is the platform for managing any content.

**Not a threat because:** No ongoing management, no registry/sharing, no TUI, no format conversion as a standalone feature, no sandbox, requires Node.js 22+, solo maintainer, <1 month old.

**Useful as a reference for:** Provider format details (especially OpenCode adapter, Codex multi-agent TOML format), tool name translation tables across providers, and MCP merge strategies. These informed Workstream 7.
