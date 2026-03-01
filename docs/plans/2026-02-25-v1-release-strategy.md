# Syllago v1.0 Release Strategy

*Design date: 2026-02-25*
*Status: In Progress — updated 2026-02-27*

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

*Decided: 2026-02-26 | Status updated: 2026-02-27*

Based on risk profiles and unblocking dependencies:

1. ~~**Distribution**~~ ✅ v0.5.0 — Binaries (6 platforms), install script, Homebrew tap, cosign signing, `syllago update`.
2. ~~**CI Pipeline**~~ ✅ v0.5.0 — `go test` + `go build` on PR and push to main.
3. ~~**Tool Coverage (11 providers)**~~ ✅ v0.5.0 + d39214c — All 11 providers: Claude Code, Gemini CLI, Cursor, Windsurf, Codex, Copilot CLI, Cline, Kiro, Roo Code, Zed, OpenCode.
4. ~~**Format Conversion Fidelity**~~ ✅ d39214c — Tool name tables for all providers, MCP merge for 9 providers, Codex multi-agent TOML, hookless provider warnings, field preservation tests.
5. ~~**Meta-Content**~~ ✅ — `syllago-guide` skill, `syllago-author` agent, built-in badge in TUI.
6. ~~**Content Model Restructure**~~ ✅ b7a415f — 42 tasks across 6 phases. `content/` directory, scanner refactor, kitchen-sink examples, CLI fixes (import/export/create/list/inspect), registry features, TUI polish.
7. ~~**SEC-001 Fix**~~ ✅ — Symlink checks in `copyFile` and `copyDir` (Lstat guard + source tree skip).
8. ~~**Sandbox Wrapper**~~ ✅ v0.5.0 — Bubblewrap isolation, egress proxy, config diff-and-approve, TUI settings.
9. **Loadouts** — Session config packaging. Claude Code only for v1. Design: `docs/syllago-loadouts-design.md` (renamed from "starters").
10. **TUI Polish + Registry Experience** — Audit for remaining gaps after content model restructure.
11. **Documentation Site (Content)** — Scaffolding done. Write actual docs content last, once features are stable.

---

## Remaining Work (as of 2026-02-28)

Eight of ten original workstreams are complete. Two workstreams remain plus a new Starters feature added for v1.

| # | Workstream | Status | Key Items |
|---|-----------|--------|-----------|
| ~~NEW~~ | ~~Content Model Restructure~~ | ✅ Complete (b7a415f) | 42 tasks, 6 phases, 120 files changed |
| ~~NEW~~ | ~~Critical CLI Fixes~~ | ✅ Complete (b7a415f) | Import to local/, export --source, syllago create/list/inspect |
| ~~NEW~~ | ~~Registry & Team Features~~ | ✅ Complete (b7a415f) | allowedRegistries, inspect, precedence, manifest, promote --to-registry |
| NEW | **Starters** | Designed, not started | Session config packaging — `syllago start/stop/revert/bundle` |
| 2+5 | TUI Polish + Registry Experience | Partially done (most items done via content model) | Audit for remaining gaps |
| 8 | Documentation Site Content | Not started (blocked by features) | Getting started, provider ref, authoring guide, CLI ref |

**Implementation order:** Starters → TUI Audit → Docs.

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

### 2. TUI Polish (Must-Have)

**Status:** Partially done — needs audit

**Goal:** Every flow feels consistent, discoverable, and professional.

- ✅ **Modal system** - Refactored for progressive disclosure (v0.5.0)
- ✅ **Golden file visual regression** - Component + full-app tests at 4 terminal sizes (v0.5.0)
- **Registry browser as showcase** - Browse registries, item counts per category, preview content, install with one keypress
- **Bug fixes** - Audit and fix broken/inconsistent flows
- **"Built-in" badge** - Visual indicator in TUI for meta-tools (distinct from "local" and "registry" content)
- **First-run experience** - When launched with no config, guide through setup (detect tools -> suggest registries -> show what's available)

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

### 5. Registry Experience (Must-Have)

**Status:** Partially done — needs audit

**Goal:** Adding and browsing a registry should feel like discovering an app store.

- **`syllago init` suggests adding syllago-tools registry** - First-run guidance
- **Registry browser in TUI** - Browse by category, item count badges, preview before install
- **syllago-tools as reference registry** - Move example content to syllago-tools repo; keep meta-tools in syllago itself
- **Registry README rendering** - Show description, maintainer, last updated in TUI
- **Short alias for common registries** - `syllago registry add syllago-tools` (expands to full GitHub URL)

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

### 9. Starters (Must-Have)

**Status:** Designed, not started

**Goal:** Package entire session configurations — rules, hooks, skills, agents, MCP servers — into shareable artifacts that work across providers.

**Full design:** `docs/syllago-starters-design.md`

**V1 Scope Decisions (2026-02-28):**

- **Claude Code only for v1** — other provider emitters are post-v1 roadmap
- **Platform, not prescription** — ship the tools to create/apply/revert starters; no pre-built opinionated packages
- **No `permissions` section** — enforcement varies too much across providers; post-v1
- **No `extends` composition** — deep merge semantics need more design; post-v1
- **Kitchen-sink starter for testing only** — validates the system, not shipped as recommended content

**V1 deliverables:**
- CLI commands: `syllago start`, `syllago stop`, `syllago revert`, `syllago bundle`
- Three-mode model: preview (default), `--try` (temporary, auto-revert), `--keep` (permanent)
- Starter manifest format (YAML with `ref:` and `inline:` content references)
- Snapshot system for backup/restore of modified files
- Symlink placement for standalone content, merge+markers for shared files (CLAUDE.md, settings.json)
- Per-provider `ConfigureSession` capability (Claude Code only for v1)

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

**Remaining: Content writing (last step before v1 release)**

Docs content is written last, after all features are implemented and stable. This ensures we document what actually shipped, not what we planned. Content scope:
- Getting started (install, first run, basic usage)
- Provider reference (supported tools, what content types each supports, format details)
- Content authoring guide (how to create skills, agents, rules, etc. in syllago-canonical format)
- Starters guide (how to create, apply, and share session configurations)
- Registry guide (how to create, publish, and consume registries)
- Sandbox guide (setup, usage, security model)
- CLI reference (all commands and flags)

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
