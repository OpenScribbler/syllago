# `.agents/` Cross-Provider Directory Research

**Date:** 2026-04-08  
**Context:** Investigating shared `.agents/` directory usage across AI coding providers to design
conflict detection for `syllago install --to-all`. When multiple providers share a common install
path, installing to all of them creates duplicate content that agents like Gemini CLI detect and
warn about as conflicts.

---

## Background

When `syllago install --to-all` was used to install 17 benchmark skills to all 7 detected
providers, Gemini CLI reported conflict warnings at startup for all 17 skills. Root cause:

- Codex's primary install path for skills = `~/.agents/skills/`
- Gemini CLI also reads from `~/.agents/skills/` (as a cross-client alias path)
- `--to-all` wrote to both `~/.agents/skills/` (via Codex) and `~/.gemini/skills/` (via Gemini)
- Gemini found the same skill in two directories it scans → conflict warning

This research establishes the ground truth for which providers use `.agents/` and how, across
all content types, to inform conflict detection logic in syllago.

---

## Terminology

The `~/.agents/` directory has no official proper name. Community and vendor terminology:

| Term | Source |
|------|--------|
| "generic alias" | Gemini CLI official docs |
| "cross-client skill sharing convention" | agentskills.io implementation guide |
| "universal bus" | Termdock community blog |
| "the `.agents/skills/` convention" | Most common neutral phrasing |

No vendor has coined a proper noun for this directory. The agentskills.io spec deliberately does
not mandate directory paths — `~/.agents/` emerged from implementations converging organically.

---

## Governing Specification

**Spec:** [agentskills.io](https://agentskills.io)  
**Repo:** [github.com/agentskills/agentskills](https://github.com/agentskills/agentskills)  
**Governance:** Agentic AI Foundation (AAIF), a Linux Foundation directed fund (announced
2025-12-09). Founding contributors: Anthropic (MCP), OpenAI (AGENTS.md), Block (goose).

The spec defines the `SKILL.md` format (frontmatter + body). It does **not** normatively mandate
directory paths. `~/.agents/` is documented in the implementation guide as a recommended
cross-client convention, not a normative requirement.

---

## Provider-by-Provider Findings

### Codex (OpenAI)

**Status: CONFIRMED from source code**  
**Source:** `openai/codex` GitHub — `codex-rs/core-skills/src/loader.rs`  
**Docs:** https://developers.openai.com/codex/skills

**Skills paths:**

| Path | Scope | Role |
|------|-------|------|
| `~/.agents/skills/` | Global (user) | **Primary install path** |
| `~/.codex/skills/` | Global (user) | Deprecated compat path |
| `/etc/codex/skills/` | System | Enterprise admin |
| `<repo-root>/.agents/skills/` | Project | Walks up from cwd to repo root |

**Source code constants (`loader.rs`):**
```rust
const AGENTS_DIR_NAME: &str = ".agents";
const SKILLS_DIR_NAME: &str = "skills";
```

`skill_roots_from_layer_stack_inner()` — for `ConfigLayerSource::User` layers: scans
`$HOME/.agents/skills/` as primary and `<CODEX_HOME>/skills/` as deprecated compat.

`repo_agents_skill_roots()` — walks directories from cwd up to project root, checks each for
`.agents/skills/`.

**Other content types using `.agents/`:** None. `.agents/` is skills-only for Codex.

**Syllago accuracy:** CORRECT. `InstallDir` = `~/.agents/skills/`, `DiscoveryPaths` includes
`.agents/skills/`.

---

### Gemini CLI (Google)

**Status: CONFIRMED from source code**  
**Source:** `google-gemini/gemini-cli` GitHub — `packages/core/src/config/storage.ts` +
`packages/core/src/skills/skillManager.ts`  
**Docs:** https://geminicli.com/docs/cli/skills/  
**Config reference:** https://geminicli.com/docs/reference/configuration/

**Skills paths (from `skillManager.ts`, `discoverSkills()`):**

| Function | Path | Scope |
|----------|------|-------|
| `getUserSkillsDir()` | `~/.gemini/skills/` | Global (primary install) |
| `getUserAgentSkillsDir()` | `~/.agents/skills/` | Global (cross-client alias) |
| `getProjectSkillsDir()` | `.gemini/skills/` | Project |
| `getProjectAgentSkillsDir()` | `.agents/skills/` | Project (cross-client alias) |

**Code explicitly calls `.agents/` a "generic alias"** — both paths are loaded, with the same
precedence tier. `.agents/` takes precedence over `.gemini/` within the same tier.

**Configuration controls:**

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `skills.enabled` | boolean | `true` | Global kill switch for all skills |
| `skills.disabled` | array | `[]` | Per-skill disable list by name |
| `admin.skills.enabled` | boolean | `true` | Admin-level kill switch |

There is **no per-directory toggle**. You cannot disable `~/.agents/skills/` scanning while
keeping `~/.gemini/skills/` active. Granularity is: all-on, all-off, or per-skill name.

**Other content types using `.agents/`:** None. `.agents/` is skills-only for Gemini CLI.

**Detection:** `~/.gemini/` directory presence.

**Syllago accuracy:** CORRECT. `InstallDir` = `~/.gemini/skills/` (Gemini's own primary path).
`DiscoveryPaths` includes both `.gemini/skills/` and `.agents/skills/`.

---

### Windsurf (Codeium)

**Status: CONFIRMED from official docs**  
**Source:** https://docs.windsurf.com/windsurf/cascade/skills  
**Changelog:** https://windsurf.com/changelog

**Skills paths:**

| Path | Scope | Role |
|------|-------|------|
| `~/.codeium/windsurf/skills/` | Global (primary install) | Primary global |
| `~/.agents/skills/` | Global | Cross-agent compat |
| `.windsurf/skills/` | Project | Primary project |
| `.agents/skills/` | Project | Cross-agent compat |
| `.claude/skills/` | Project | Claude Code compat (opt-in setting) |
| `~/.claude/skills/` | Global | Claude Code compat (opt-in setting) |
| `/etc/windsurf/skills/` | System | Enterprise admin (Linux) |
| `/Library/Application Support/Windsurf/skills/` | System | Enterprise admin (macOS) |

**Configuration controls:** No documented per-directory opt-out for `~/.agents/skills/` scanning.
Windsurf scanning of `.agents/skills/` is always-on. The `.claude/skills/` compat path requires
an explicit setting to enable.

**Other content types using `.agents/`:** None documented. Cross-agent compat is skills-only.

**Detection:** `~/.codeium/windsurf/` directory presence.

**Syllago accuracy:** CORRECT. `InstallDir` = `~/.codeium/windsurf/skills/`. `DiscoveryPaths`
includes `.windsurf/skills/` and `.agents/skills/`.

---

### Roo Code

**Status: CONFIRMED from source code**  
**Source:** `RooVetGit/Roo-Code` GitHub — `src/services/skills/SkillsManager.ts` +
`src/services/roo-config/index.ts`  
**Docs:** https://docs.roocode.com/features/skills  
**Release notes:** https://docs.roocode.com/update-notes/v3.38.0 (skills introduced 2025-12-27)

**Skills paths (from `getSkillsDirectories()`, in priority order, lowest to highest):**

| Path | Scope | Priority |
|------|-------|----------|
| `~/.agents/skills/` | Global | Lowest |
| `~/.agents/skills-{mode}/` | Global, mode-specific | Low |
| `.agents/skills/` | Project | Low |
| `.agents/skills-{mode}/` | Project, mode-specific | Low |
| `~/.roo/skills/` | Global (Roo-native) | High |
| `~/.roo/skills-{mode}/` | Global, mode-specific | High |
| `.roo/skills/` | Project | Highest |
| `.roo/skills-{mode}/` | Project, mode-specific | Highest |

**Global path helpers (`roo-config/index.ts`):**
- `getGlobalRooDirectory()` → `~/.roo`
- `getGlobalAgentsDirectory()` → `~/.agents`
- `getProjectAgentsDirectoryForCwd(cwd)` → `<cwd>/.agents`

Mode-specific variants (e.g., `.agents/skills-code`, `.agents/skills-architect`) are an
additional layer syllago does not currently model.

**Configuration controls:** No documented disable toggle for `~/.agents/` scanning. The only
effective suppression is placing a same-named skill in `.roo/skills/` (higher priority) to
shadow the `.agents/` version.

**Other content types using `.agents/`:** None. `.agents/` is skills-only for Roo Code.

**Detection:** `~/.roo/` directory presence.

**Syllago accuracy:** CORRECT for current model. `InstallDir` = `~/.roo/skills/` (Roo-native
primary). `DiscoveryPaths` includes `.roo/skills/` and `.agents/skills/` (project-level). Gap:
`~/.agents/skills/` is also a global discovery path for RooCode, but syllago's `DiscoveryPaths`
field is project-scoped by design.

---

### Amp

**Status: CONFIRMED from official docs**  
**Source:** https://ampcode.com/manual  
**Additional:** https://ampcode.com/news/agent-skills

**Skills paths (in precedence order, first match wins):**

| Path | Scope | Priority |
|------|-------|----------|
| `~/.config/agents/skills/` | Global | Primary (highest) |
| `~/.config/amp/skills/` | Global | Secondary (Amp-specific) |
| `.agents/skills/` | Project | Project-level |
| `.claude/skills/` | Project | Legacy Claude Code compat |
| `~/.claude/skills/` | Global | Legacy Claude Code compat |

**Important distinction:** Amp uses `~/.config/agents/skills/` (XDG-style), NOT `~/.agents/skills/`.
This is unique among the providers surveyed. All other providers use `~/.agents/skills/` directly.

**New content type: Code Review Checks** — `.agents/checks/` directories. This is NOT modeled
in syllago at all:
- `.agents/checks/` — applies to entire codebase
- `subdirectory/.agents/checks/` — applies to a subtree
- `~/.config/amp/checks/` or `~/.config/agents/checks/` — global checks

Checks are YAML files with frontmatter describing code quality rules. Distinct from skills.

**AGENTS.md paths (rules):**
- Walks from cwd up to `$HOME` looking for `AGENTS.md`, `AGENT.md`, or `CLAUDE.md`
- `$HOME/.config/amp/AGENTS.md` — device-specific personal preferences
- `$HOME/.config/AGENTS.md` — global user rules

**Configuration controls:**

| Setting | Key | Scope |
|---------|-----|-------|
| Skills from Claude dirs | `amp.skills.disableClaudeCodeSkills` | Disable `.claude/` paths |
| Additional skill dirs | `amp.skills.path` | Colon-separated extra paths |

No documented way to disable `~/.config/agents/skills/` or `.agents/skills/` scanning.

**Config file locations:**
- Workspace: `.amp/settings.json` or `.amp/settings.jsonc`
- User: `~/.config/amp/settings.json` or `~/.config/amp/settings.jsonc`
- Enterprise: `/Library/Application Support/ampcode/` (macOS), `/etc/ampcode/` (Linux)

**MCP:** Configured via `amp.mcpServers` in settings or bundled in skill directories via `mcp.json`

**Detection:** `~/.config/amp/` directory presence.

**Syllago accuracy:** CORRECT. `InstallDir` = `~/.config/agents/skills/` (confirmed primary).
`DiscoveryPaths` includes `.agents/skills/`. The XDG-style global path (`~/.config/agents/`) is
confirmed — Amp intentionally uses this rather than `~/.agents/` directly.

---

### OpenCode

**Status: BUG FOUND in syllago — DiscoveryPaths incomplete**  
**Source:** `sst/opencode` GitHub — `packages/opencode/src/skill/index.ts`  
**Docs:** https://opencode.ai/docs/skills  
**Config:** https://opencode.ai/docs/config

**Skills paths (from `loadSkills()` in `skill/index.ts`):**

```typescript
const EXTERNAL_DIRS = [".claude", ".agents"]
const EXTERNAL_SKILL_PATTERN = "skills/**/SKILL.md"
const OPENCODE_SKILL_PATTERN = "{skill,skills}/**/SKILL.md"
```

| Path | Scope |
|------|-------|
| `~/.agents/skills/` | Global |
| `~/.claude/skills/` | Global (compat) |
| `~/.config/opencode/skills/` | Global (primary install) |
| `.agents/skills/` | Project |
| `.claude/skills/` | Project (compat) |
| `.opencode/skills/` | Project |

`loadSkills()` explicitly iterates over `EXTERNAL_DIRS = [".claude", ".agents"]` and scans both
globally (`$HOME/<dir>/skills/**/SKILL.md`) and project-level.

**Syllago bug:** `opencode.go` `DiscoveryPaths` for skills only includes `.opencode/skills/`.
It is missing `.agents/skills/`. Fix needed:

```go
case catalog.Skills:
    return []string{
        filepath.Join(projectRoot, ".opencode", "skills"),
        filepath.Join(projectRoot, ".agents", "skills"),
    }
```

**Other content types using `.agents/`:** None confirmed beyond skills.

**Detection:** `~/.config/opencode/` directory or `opencode` binary in PATH.

**Syllago accuracy:** `InstallDir` CORRECT (`~/.config/opencode/skills/`). `DiscoveryPaths`
**WRONG** — missing `.agents/skills/`.

---

### Providers That Do NOT Use `.agents/`

| Provider | Config dir | Skills path | `.agents/` used? |
|----------|-----------|-------------|-----------------|
| Claude Code | `~/.claude/` | `~/.claude/skills/` | No |
| Cline | `.clinerules/` | No skills support | No |
| Copilot CLI | `~/.copilot/` | `~/.github/skills/` | No |
| Cursor | `~/.cursor/` | `~/.cursor/skills/` | No |
| Kiro | `~/.kiro/` | `.kiro/steering/` (via steering files) | No* |
| Zed | `~/.config/zed/` | No skills support | No |

*Kiro has `.kiro/agents/` but this is Kiro-specific agent definitions, not the cross-client
`.agents/` convention.

---

## Summary: `.agents/` Usage Map

### Which providers use `~/.agents/skills/` globally?

| Provider | Global path | Role |
|----------|-------------|------|
| Codex | `~/.agents/skills/` | **Primary install** |
| Gemini CLI | `~/.agents/skills/` | Cross-client alias (also reads `~/.gemini/skills/`) |
| Windsurf | `~/.agents/skills/` | Cross-client compat (also reads `~/.codeium/windsurf/skills/`) |
| Roo Code | `~/.agents/skills/` | Lower-priority fallback (primary = `~/.roo/skills/`) |
| OpenCode | `~/.agents/skills/` | Cross-client compat (primary = `~/.config/opencode/skills/`) |
| Amp | `~/.config/agents/skills/` | **Primary install** (XDG variant — different path!) |

### Which providers use `.agents/skills/` at project level?

Codex, Gemini CLI, Windsurf, Roo Code, OpenCode, Amp — all six.

### Content types beyond skills that use `.agents/`

Only **Amp** uses `.agents/` for a non-skills content type:
- `.agents/checks/` — Code Review Checks (YAML files defining code quality rules)
- `~/.config/agents/checks/` — global checks

No other provider uses `.agents/` for rules, hooks, MCP, agents, or commands.

---

## Conflict Scenario

When `syllago install --to-all` selects providers that include Codex (or any future provider
with `~/.agents/skills/` as primary install) alongside Gemini/Windsurf/RooCode/OpenCode (which
also scan `~/.agents/skills/`), installing to each provider's canonical path creates duplicates:

- Codex install writes to `~/.agents/skills/<name>/`
- Gemini install writes to `~/.gemini/skills/<name>/`
- Gemini startup scans both `~/.agents/skills/` and `~/.gemini/skills/` → finds duplicates → conflict warning

The same pattern applies to Windsurf, RooCode, and OpenCode.

**Amp does NOT participate in this conflict** — Amp uses `~/.config/agents/skills/` (different
path from `~/.agents/skills/`), so installing to Amp's path doesn't conflict with Codex.

---

## Design Implications for syllago

### SharedReadPaths field (proposed)

To detect conflicts before install, syllago needs to know which providers read from paths they
don't own as primary install targets. The current `DiscoveryPaths` field is project-scoped.
A new `GlobalSharedReadPaths func(homeDir string, ct ContentType) []string` field would capture:

- Gemini: `~/.agents/skills/` for skills
- Windsurf: `~/.agents/skills/` for skills
- Roo Code: `~/.agents/skills/` for skills
- OpenCode: `~/.agents/skills/` for skills

### Conflict detection logic

```
For each content type being installed:
  installPaths = map[path]provider for each provider in install set
  
  For each provider in install set:
    For each sharedReadPath in provider.GlobalSharedReadPaths(home, ct):
      If sharedReadPath exists in installPaths (owned by another provider):
        → CONFLICT: warn user, offer choice
```

### User-facing options when conflict detected

1. **Shared path only** — install once to `~/.agents/<type>/` (serves all providers that read it)
2. **Each provider's canonical path** — no shared path, each provider gets its own dir
3. **All paths** — current behavior, causes conflict warnings

### Scope

- Skills only for now (confirmed by research — no other content type has cross-provider `.agents/` paths)
- Amp's `~/.config/agents/` is a different path family — not in conflict with `~/.agents/`
- Amp's `.agents/checks/` is a new content type syllago doesn't model (future work)

---

## Action Items

| Priority | Item | File |
|----------|------|------|
| **Bug** | Fix OpenCode `DiscoveryPaths` — add `.agents/skills/` | `cli/internal/provider/opencode.go` |
| **Feature** | Add `GlobalSharedReadPaths` field to Provider struct | `cli/internal/provider/provider.go` |
| **Feature** | Populate `GlobalSharedReadPaths` for Gemini, Windsurf, RooCode, OpenCode | `gemini.go`, `windsurf.go`, `roocode.go`, `opencode.go` |
| **Feature** | Implement conflict detection in installer | `cli/internal/installer/` |
| **Feature** | Add pre-install prompt (CLI) and modal (TUI) | `cli/cmd/syllago/install_cmd.go`, `cli/internal/tui/` |
| **Future** | Research and model Amp's `.agents/checks/` content type | New content type |
| **Future** | Verify mode-specific RooCode paths (`.agents/skills-{mode}/`) | `cli/internal/provider/roocode.go` |

---

## Sources

| Source | URL | Type |
|--------|-----|------|
| Codex skills loader (source) | `github.com/openai/codex` — `codex-rs/core-skills/src/loader.rs` | Source code |
| Codex skills docs | https://developers.openai.com/codex/skills | Official docs |
| Gemini CLI skillManager (source) | `github.com/google-gemini/gemini-cli` — `packages/core/src/skills/skillManager.ts` | Source code |
| Gemini CLI storage (source) | `github.com/google-gemini/gemini-cli` — `packages/core/src/config/storage.ts` | Source code |
| Gemini CLI skills docs | https://geminicli.com/docs/cli/skills/ | Official docs |
| Gemini CLI config reference | https://geminicli.com/docs/reference/configuration/ | Official docs |
| Windsurf skills docs | https://docs.windsurf.com/windsurf/cascade/skills | Official docs |
| Windsurf changelog | https://windsurf.com/changelog | Official changelog |
| Roo Code SkillsManager (source) | `github.com/RooVetGit/Roo-Code` — `src/services/skills/SkillsManager.ts` | Source code |
| Roo Code roo-config (source) | `github.com/RooVetGit/Roo-Code` — `src/services/roo-config/index.ts` | Source code |
| Roo Code skills docs | https://docs.roocode.com/features/skills | Official docs |
| Roo Code v3.38.0 release | https://docs.roocode.com/update-notes/v3.38.0 | Release notes |
| Amp manual | https://ampcode.com/manual | Official docs |
| Amp Agent Skills announcement | https://ampcode.com/news/agent-skills | Official announcement |
| OpenCode skill/index.ts (source) | `github.com/sst/opencode` — `packages/opencode/src/skill/index.ts` | Source code |
| OpenCode skills docs | https://opencode.ai/docs/skills | Official docs |
| agentskills.io spec | https://agentskills.io/specification | Spec |
| agentskills.io impl guide | https://agentskills.io/client-implementation/adding-skills-support | Spec |
| Termdock cross-agent blog | https://www.termdock.com/en/blog/cross-agent-skills-new-npm | Community |
| Gemini CLI cross-agent (DEV) | https://dev.to/gde/cross-agent-synergy-how-i-keep-gemini-cli-in-sync-with-other-ai-tools-3017 | Community |
