# Provider Coverage Reconciliation

**Date:** 2026-04-17
**Status:** Complete — shipped in v0.9.0 (tag pushed 2026-04-17). All phases done; syllago-docs synced (commit 593aefe).
**Scope:**
- Primary: Reconcile syllago provider capability data (Go code + format YAMLs + source manifests) against verified upstream runtime support, starting with 5 known-incomplete providers (Cursor, Roo Code, Zed, OpenCode, Crush).
- Expanded (from Q3 regression sweep): Amp hooks drift (Phase 1c), Cline + Windsurf workflows-as-commands (Phase 1d). Cline skills deferred as known gap (Phase 1e).
- Adjacent: Q4 regression-prevention harness. Capmon pipeline reliability tracked separately as bd `syllago-cnoxd`.

**Related:** `2026-04-16-capmon-phase6-recognition-plan.md` — recognition pipeline (complementary; this plan handles format YAML population, that one handles field-to-canonical-path mapping).

---

## Background

### What we found

A live-doc verification of 6 contested capability cells across 3 providers — Cursor, Roo Code, OpenCode — confirmed all 6 as native runtime support. Two of the six required corrections to syllago's data:

| # | Cell | Verified Verdict | Confidence |
|---|------|------------------|------------|
| 1 | Cursor skills | NATIVE | High |
| 2 | Cursor commands | NATIVE | High |
| 3 | Cursor agents (subagents) | NATIVE | High |
| 4 | Roo Code skills | NATIVE | High |
| 5 | Roo Code commands | **NATIVE — syllago Go code missing it** | High |
| 6 | OpenCode agents | **USER-DEFINABLE — earlier "built-in only" finding was based on the archived repo** | High |

Sources verified live (April 2026):
- Cursor 2.4 changelog (`https://cursor.com/changelog/2-4`) — added subagents + skills (Anthropic Agent Skills standard)
- Cursor 1.6 — added custom slash commands at `.cursor/commands/*.md`
- `https://cursor.com/docs/skills`, `https://cursor.com/docs/context/subagents`, `https://cursor.com/docs/context/commands`
- Roo Code repo examples: `.roo/skills/<slug>/SKILL.md`, `.roo/commands/*.md`
- OpenCode (active fork: `sst/opencode`, NOT the archived `opencode-ai/opencode`) — `https://opencode.ai/docs/agents/` confirms `.opencode/agents/*.md` user-definable agents

Full verification report and quoted evidence available in the syllago-docs session log dated 2026-04-17.

### What was going on

Three distinct data integrity problems across syllago's provider system:

1. **Stale Go provider definitions.** `cli/internal/provider/roocode.go` `SupportsType()` excludes `Commands` — written before Roo Code 3.25 added the `run_slash_command` tool reading from `.roo/commands/*.md`. This causes `providers.json` to report `commands.supported: false` for Roo Code, which propagates downstream to `syllago-docs` data files and rendered MDX pages.

2. **Stub format YAMLs.** `docs/provider-formats/*.yaml` for cursor, roo-code, zed, and opencode are largely skills-only stubs (most marked `status: unsupported`). `crush.yaml` doesn't exist at all. These feed `capabilities.json`, so the canonical-keys / conversion matrix is also incomplete for these 5 providers.

3. **Source manifest drift.** `docs/provider-sources/roo-code.yaml` declares `commands.supported: false`, contradicting the verified runtime behavior. Capmon therefore never fetched/extracted commands docs for Roo Code, so even a clean re-run of the pipeline wouldn't surface the gap.

### Why this happened

Plausible archaeology — none of this was negligence; each gap has a coherent origin:

- **Skills got the gold-standard treatment first.** When canonical keys were expanded (`docs/plans/2026-04-13-canonical-keys-expansion-*.md`), skills was the proving ground. Other content types for newer/less-mature providers were left as stubs to ship the schema work.
- **Cursor 2.4 (Jan 2026) and Roo Code 3.46 are recent.** Cursor only added native subagents + skills in late January 2026; the syllago provider definitions predate those releases.
- **OpenCode fork ambiguity.** The original `opencode-ai/opencode` (now archived) hard-coded 4 agent types (`coder`, `summarizer`, `task`, `title`). The active project is `sst/opencode`, which supports user-definable agents via `.opencode/agents/*.md`. An earlier verification pass cited the archived repo and concluded "built-in only" — that was wrong, and only caught when the syllago-docs `opencode.json` data showed rich frontmatter that contradicted the assumption.
- **Capmon's `supported: false` source-manifest semantics are subtle.** The flag means "no upstream URL to monitor," not "runtime doesn't support." Setting it `false` for Roo Code commands silently took the gap out of the capmon validation loop.

### Why this matters

`providers.json` and `capabilities.json` are the wire-format contract between syllago and downstream consumers (syllago-docs, registries, anyone querying capabilities programmatically). Wrong data here means:
- Users see "commands: not supported" in Roo Code provider docs and don't try to install commands that would actually work.
- Cross-provider conversion logic skips paths it shouldn't skip.
- Conversion-aware features tables in `syllago-docs` (rebuilt 2026-04-13) misrepresent what a target provider can accept.

---

## Phases

Phases are sequenced by dependency — each unblocks the next. Phase 0 is complete.

---

### Phase 0 — Ground truth verification (DONE)

**Goal:** Establish, with cited live-doc evidence, exactly what each of the 5 problem providers natively supports per content type.

- [x] Live-fetch Cursor docs (skills, commands, subagents) via stealth-fetch (cursor.com 429-rate-limits Readability)
- [x] Verify Roo Code skills + commands via repo example files
- [x] Resolve OpenCode fork ambiguity (archived `opencode-ai/opencode` vs active `sst/opencode`); verify via `opencode.ai/docs/agents/`
- [x] Compile structured 6-cell report with per-cell evidence + confidence
- [x] Identify all sites of drift (Go code, format YAMLs, source manifests)

**Artifact:** Verification report in syllago-docs session 2026-04-17.

---

### Phase 1 — Fix Go provider code

**Goal:** Make `prov.SupportsType()` agree with verified runtime support so `providers.json` stops lying.

#### 1a — Roo Code: add Commands

File: `cli/internal/provider/roocode.go`

Changes:
- Add `catalog.Commands` to the `SupportsType` switch:
  ```go
  case catalog.Rules, catalog.Skills, catalog.MCP, catalog.Agents, catalog.Commands:
      return true
  ```
- Add `Commands` to `InstallDir` (project-scope: `.roo/commands`)
- Add `Commands` to `DiscoveryPaths` (`.roo/commands`)
- Add `Commands` to `FileFormat` (`md`)
- Wire `frontmatter_registry.go` if Commands recognizers are needed (coordinate with capmon Phase 6 Epic 5)

Acceptance:
- `go build ./...` clean
- Unit test: `prov.SupportsType(catalog.Commands)` returns `true` for the Roo Code provider
- Manual: `syllago providers list --content commands` includes `roo-code`

#### 1b — Audit other providers (regression sweep) — DONE

Completed as Q3 — audit of 11 "no" cells across 10 providers (amp, claude-code, cline, codex, copilot-cli, factory-droid, gemini-cli, kiro, pi, windsurf) against live upstream docs. See Appendix A for the full 10-provider truth matrix. Two genuine drift findings surfaced, promoted to Phase 1c and 1d below.

#### 1c — Amp: flip hooks to supported

File: `cli/internal/provider/amp.go`

Upstream reality (verified at `https://ampcode.com/manual/hooks.md`):
- Amp supports hooks via the `amp.hooks` JSON array in settings.
- Events: `tool:pre-execute`, `tool:post-execute`.
- Config schema: `event`, `action`, `input.contains` (exact-string match).
- Actions: `send-user-message`, `redact-tool-input`.

Internal state today:
- `amp.go` already declares `ConfigLocations[Hooks] = ".amp/hooks.json"` and `HookTypes: []string{"command"}` — metadata is present.
- `SupportsType` returns `false` for `Hooks`; `InstallDir` has no case for `Hooks`.
- Provider-source manifest (`docs/provider-sources/amp.yaml`) already lists hooks as supported — Go code is the only source out of sync.

Changes:
- Add `catalog.Hooks` to the `SupportsType` switch (flip to `true`).
- Add `catalog.Hooks` to `InstallDir` — return `JSONMergeSentinel` (merges into `.amp/settings.json` under `amp.hooks`).
- Add `catalog.Hooks` to `DiscoveryPaths` (project: `.amp/settings.json`; user: `~/.config/amp/settings.json`).
- `SymlinkSupport[catalog.Hooks] = false` (JSON merge path).
- Revisit canonical `ConfigLocations[catalog.Hooks]` — the existing `.amp/hooks.json` doesn't match the upstream schema (hooks live in `amp.hooks` inside settings.json). Update to `.amp/settings.json` or drop to avoid misleading consumers.

Acceptance:
- `go build ./...` clean
- Unit test: `prov.SupportsType(catalog.Hooks)` returns `true` for Amp
- Manual: `syllago providers list --content hooks` includes `amp`

#### 1d — Cline + Windsurf: add Commands (workflows)

Both providers have user-definable markdown workflow files invoked via slash command — shape-identical to Claude Code slash commands. Syllago maps these to the `Commands` content type.

**Cline** (`cli/internal/provider/cline.go`) — verified at `https://docs.cline.bot/customization/workflows.md`:
- Files: `.clinerules/workflows/*.md` (project) and `~/Documents/Cline/Workflows/*.md` (global, platform-specific).
- Format: markdown with title + steps; filename becomes the slash command (e.g. `deploy.md` → `/deploy.md`).
- Invocation: `/filename.md` in Cline prompt.

Changes to `cline.go`:
- Add `catalog.Commands` to `SupportsType` switch.
- Add `catalog.Commands` to `InstallDir` — project scope (`ProjectScopeSentinel` with target `.clinerules/workflows`).
- Add `catalog.Commands` to `DiscoveryPaths` — project `.clinerules/workflows` + user `~/Documents/Cline/Workflows` (handle platform differences per existing helper pattern).
- `SymlinkSupport[catalog.Commands] = true`.
- `FileFormat(Commands)` = `FormatMarkdown`.

**Windsurf** (`cli/internal/provider/windsurf.go`) — verified at `https://docs.windsurf.com/windsurf/cascade/workflows.md`:
- Files: `.windsurf/workflows/*.md` (workspace) and `~/.codeium/windsurf/global_workflows/*.md` (global).
- Format: markdown with title, description, steps; 12,000 character limit per file.
- Invocation: `/workflow-name` (manual-only — never auto-invoked).

Changes to `windsurf.go`:
- Add `catalog.Commands` to `SupportsType` switch.
- Add `catalog.Commands` to `InstallDir` — project scope with target `.windsurf/workflows`.
- Add `catalog.Commands` to `DiscoveryPaths` — project `.windsurf/workflows` + user `~/.codeium/windsurf/global_workflows`.
- `SymlinkSupport[catalog.Commands] = true`.
- `FileFormat(Commands)` = `FormatMarkdown`.

Acceptance (both):
- `go build ./...` clean
- Unit test: `prov.SupportsType(catalog.Commands)` returns `true` for cline and windsurf
- Manual: `syllago providers list --content commands` includes both slugs

#### 1e — Cline skills (known gap — deferred)

Cline supports user-definable skills in `.cline/skills/`, `.clinerules/skills/`, and `.claude/skills/` (verified at `https://docs.cline.bot/customization/skills.md`). The provider-source manifest already flags this as "not yet implemented in syllago for cline". Implementation deferred — either pick up in Phase 2 extension of this plan or file a separate issue.

---

### Phase 2 — Author missing format YAMLs

**Goal:** Replace skills-only stubs in `docs/provider-formats/` with full content-type coverage matching verified runtime behavior. This populates `capabilities.json`.

Use `gemini-cli.yaml` as the gold-standard template (already covers all 5 content types with canonical mappings).

#### 2a — `docs/provider-formats/cursor.yaml`

Replace stub. Author entries for: rules, skills, hooks, mcp, commands, agents.

Key facts to encode:
- **rules**: `mdc` format, `.cursor/rules/*.mdc`, frontmatter: `description`, `alwaysApply`, `globs`
- **skills**: `md`, `.agents/skills/`, `.cursor/skills/`, `~/.agents/skills/`; frontmatter per Anthropic Agent Skills standard: `name`, `description`, `license`, `compatibility`, `metadata`, `disable-model-invocation`
- **hooks**: `json`, `.cursor/settings.json`, json-merge install, hookEvents per existing data
- **mcp**: `json`, `.cursor/mcp.json`, transports: `stdio`, `sse`, `streamable-http`
- **commands**: `md`, `.cursor/commands/*.md`, filename-as-command-name (Cursor 1.6+)
- **agents**: `md`, `.cursor/agents/*.md` + `~/.cursor/agents/`, frontmatter: `name`, `description`, `model`, `readonly`, `is_background` (Cursor 2.4+); document the 3 built-in subagents (Explore, Bash, Browser)

#### 2b — `docs/provider-formats/roo-code.yaml`

Replace stub. Author entries for: rules, skills, mcp, agents (Custom Modes), commands.

Key facts:
- **rules**: project-scope, multi-mode rule dirs (`.roo/rules`, `.roo/rules-code`, `.roo/rules-architect`, etc.)
- **skills**: `md`, `.roo/skills/<slug>/SKILL.md` + `.agents/skills/`, frontmatter: `name`, `description` (lighter subset than Anthropic standard)
- **mcp**: `json`, `.roo/mcp.json`, transports: `stdio`, `sse`, `streamable-http`
- **agents**: `yaml`, custom modes via `.roomodes`, frontmatter: `slug`, `name`, `roleDefinition`, `whenToUse`, `customInstructions`, `groups`
- **commands**: `md`, `.roo/commands/*.md`, frontmatter: `description`, `argument-hint`, `mode` (Roo Code 3.25+)

#### 2c — `docs/provider-formats/zed.yaml`

Replace stub. Author entries for: rules, mcp ONLY. (Zed's built-in agents and slash commands are not user-definable per Zed docs.)

Key facts:
- **rules**: `md`, agent rules location
- **mcp**: `json`, transports per Zed docs
- Mark hooks/skills/agents/commands as `status: unsupported` with a one-line reason ("Zed's slash commands and agents are built-in, not user-definable")

#### 2d — `docs/provider-formats/opencode.yaml`

Skills entry already populated (gold-standard convention example). Add: rules, agents, commands, mcp.

Key facts (from `opencode.ai/docs/`):
- **rules**: `AGENTS.md` and `CLAUDE.md` at project root; cross-provider convention
- **agents**: `md`, `.opencode/agents/*.md` + `~/.config/opencode/agents/`, frontmatter: `description` (required), `mode` (`primary`|`subagent`), `model`, `temperature`, `maxIterations`. **Verify against `sst/opencode` source** — the syllago-docs `opencode.json` lists `tools`, `steps`, `color` which aren't in the public docs and may be undocumented or stale.
- **commands**: `md`, `.opencode/commands/*.md`, frontmatter: `description`, `agent`, `model`, `subtask`
- **mcp**: `jsonc`, `opencode.json` or `opencode.jsonc`, stdio transport

#### 2e — `docs/provider-formats/crush.yaml`

Create from scratch. Author: rules, skills, mcp.

Key facts:
- **rules**: project-scope `AGENTS.md`
- **skills**: `md`, `.crush/skills/`, cross-provider convention; frontmatter per Anthropic Agent Skills standard
- **mcp**: `json`, `crush.json`, transports: `stdio`, `http`, `sse`
- Mark hooks/agents/commands as `status: unsupported`

#### 2f — `docs/provider-formats/amp.yaml` — add hooks entry

Follows Phase 1c Go-code fix. Author a hooks entry matching the upstream schema:
- Install target: `.amp/settings.json` under `amp.hooks` (JSON merge)
- Event names (canonical → native): map syllago canonical events to `tool:pre-execute` / `tool:post-execute`
- Hook schema fields: `event`, `action`, `input.contains`

#### 2g — `docs/provider-formats/cline.yaml` — add commands entry (workflows)

Follows Phase 1d Go-code fix. Author commands entry for Cline workflows:
- Install target: `.clinerules/workflows/` (project), `~/Documents/Cline/Workflows/` (global, platform-specific)
- Format: `md`, filename-as-command-name
- Invocation: `/filename.md`
- Source URL: `https://docs.cline.bot/customization/workflows.md`

#### 2h — `docs/provider-formats/windsurf.yaml` — add commands entry (workflows)

Follows Phase 1d Go-code fix. Author commands entry for Cascade workflows:
- Install target: `.windsurf/workflows/` (workspace), `~/.codeium/windsurf/global_workflows/` (global)
- Format: `md`, filename-as-command-name, 12,000 character limit per file
- Invocation: `/workflow-name` (manual-only)
- Source URL: `https://docs.windsurf.com/windsurf/cascade/workflows.md`

#### Cross-cutting validation for Phase 2

For each YAML:
- Run `syllago capmon validate-format-doc <slug>` — must pass
- Run `syllago capmon validate-sources --provider <slug>` — must pass

Acceptance for entire Phase 2:
- All 5 YAMLs validate green
- `gencapabilities.go` regen produces a `capabilities.json` that includes all 5 providers across their supported content types with canonical mappings populated

---

### Phase 3 — Update source manifests

**Goal:** Ensure `docs/provider-sources/*.yaml` reflects the corrected runtime support so capmon can monitor what actually exists.

#### 3a — `docs/provider-sources/roo-code.yaml`

- Flip `commands.supported: false` → `true`
- Add discovery URL for commands (e.g., point at the Roo Code GitHub repo's `.roo/commands/` directory or a docs page covering custom commands)
- Update the human-readable comment block at the top to reflect the corrected support matrix

#### 3b — Audit the other 4 problem providers' source manifests

For cursor, zed, opencode, crush:
- Re-read `docs/provider-sources/<slug>.yaml`
- For every cell now marked supported in Phase 1/2 but `false` in the source manifest, flip to `true` and add a source URL
- Update top-of-file comments

#### 3c — `docs/provider-sources/crush.yaml` cleanup

The current file's comment claims "not yet implemented in syllago Go provider code" — this is stale (Crush IS implemented in `cli/internal/provider/crush.go`). Remove the misleading comment.

Acceptance:
- `syllago capmon validate-sources --provider <each>` passes
- No source manifest claims `supported: false` for a content type the Go code reports as `true`

---

### Phase 4 — Run capmon end-to-end

**Goal:** Validate the corrected manifests fetch + extract cleanly, and that recognition (where implemented) produces sensible canonical mappings.

For each of the 5 problem providers:

1. `syllago capmon fetch --provider <slug>` — should succeed for all newly-flipped sources
2. `syllago capmon extract --provider <slug>` — should produce non-empty `extracted.json` per cache entry
3. `syllago capmon seed --provider <slug>` — bootstrap any new YAML stubs that need it (note: most authoring is in Phase 2, but seed can help cross-check)
4. `syllago capmon run --provider <slug>` — full pipeline end-to-end
5. Spot-check `meta.json` for each cache entry: no fetch errors, no extract errors

Acceptance:
- 5 of 5 problem providers pass full `capmon run` clean
- `bd preflight` passes
- No new orphan dash-named cache dirs (per Capmon Phase 6 Epic 0 hygiene)

---

### Phase 5 — Regenerate, release

**Goal:** Ship corrected `providers.json` and `capabilities.json` so syllago-docs can sync.

#### 5a — Regenerate generated artifacts

- `make generate` (or whichever target runs `gencapabilities.go` + provider manifest generation)
- Diff `providers.json` and `capabilities.json` against previous version
- Verify diff matches expectations (Roo Code commands flipped to supported, Cursor 6 content types now populated, etc.)

#### 5b — Update CHANGELOG / release notes

In `syllago/CHANGELOG.md`, add entry describing the coverage corrections. Be specific:
- "Fixed: Roo Code now correctly reports `commands` support (was `false`)"
- "Added: Full capability coverage for Cursor (skills, commands, agents) per Cursor 1.6/2.4 releases"
- "Added: Full capability coverage for OpenCode (rules, commands, agents, mcp); skills already present"
- "Added: Initial capability coverage for Crush"
- "Added: Capability coverage for Zed (rules, mcp)"

#### 5c — Cut release

Per syllago's release process (see `releases/` directory and `VERSIONING.md`). This is likely a `0.7.x` patch or `0.8.0` minor depending on consumer impact. The schema isn't changing — only data — so a patch may be acceptable.

Acceptance:
- New syllago release tagged on GitHub
- `providers.json` and `capabilities.json` available as release assets

---

### Phase 6 — Sync to syllago-docs

**Goal:** Pull the corrected upstream data into `syllago-docs` so rendered pages match reality.

In `/home/hhewett/.local/src/syllago-docs/`:

1. `bun scripts/sync-providers.ts` — pulls latest `providers.json`, regenerates `src/data/providers/*.json` and `src/content/docs/using-syllago/providers/*.mdx`
2. `bun scripts/sync-capabilities.ts` — pulls latest `capabilities.json`, regenerates capabilities collection
3. `bun run build` — verify Astro build clean (no broken sidebar links, no missing-route errors)
4. Spot-check rendered pages:
   - `/using-syllago/providers/roo-code/commands/` — should now exist
   - `/using-syllago/providers/cursor/agents/` — frontmatter table populated
   - `/using-syllago/providers/opencode/agents/` — frontmatter fields match docs
5. Update `syllago-docs/CHANGELOG.md` per the project rule (entries under today's date with **Added/Changed/Fixed**)

Acceptance:
- Astro build green
- All 5 problem providers' MDX pages render with full content
- `bun astro check` clean (no link errors)
- syllago-docs CHANGELOG updated

---

## Open Questions / Loose Ends

### Q1 — OpenCode frontmatter field discrepancy — RESOLVED
Canonical frontmatter identified from `sst/opencode` source at `packages/opencode/src/config/agent.ts` (the markdown loader's Zod `Info` schema):
- Fields: `model`, `variant`, `temperature`, `top_p`, `prompt`, `tools` (deprecated — use `permission`), `disable`, `description`, `mode` (`subagent`|`primary`|`all`), `hidden`, `options`, `color`, `steps`, `maxSteps` (deprecated — use `steps`), `permission`.
- `name` is filename-derived (`configEntryNameFromPath`), not a frontmatter field.
- Loader glob: `{agent,agents}/**/*.md` (both singular and plural, recursive).
- Errors in existing data: plan's Phase 2d text listed `maxIterations` (doesn't exist — actual field is `steps`). `syllago-docs/src/data/providers/opencode.json` lists `name` (spurious) and misses `mode`, `permission`, `top_p`.

**Action folded into Phase 2d.** No upstream-docs PR — syllago-docs regenerates from syllago's YAMLs on release.

### Q2 — Cursor subagents `Task` tool bug — RESOLVED
No callout in syllago docs, format YAML, or source manifest. Scope boundary: syllago documents provider content shape per the provider's own docs; upstream runtime bugs are out of scope.

**Action:** None. Describe Cursor agents straight per `cursor.com/docs/context/subagents`. If Cursor later marks the feature unsupported in their own docs, syllago follows.

### Q3 — Are there other providers with similar gaps? — RESOLVED
Full 10-provider × `SupportsType`-false audit completed (April 2026). Results in Appendix A. Findings promoted to Phase 1c (Amp hooks drift) and Phase 1d (Cline + Windsurf workflows → Commands). Cline skills deferred to Phase 1e as known gap. All other "no" cells verified correct.

### Q4 — Regression prevention — DESIGNED

**Goal:** catch Go-code × format-YAML × source-manifest drift before it ships, and before stale data lingers after a capmon run.

**Zero drift tolerance.** No escape hatch, no skip tag, no `// nolint:coverage`. If the check fails, the build fails or the capmon run fails.

**Design:**

1. Extract check logic into a reusable library function:
   - `cli/internal/provider/coverage.go` — `CheckCoverage() []Violation`
   - `cli/internal/provider/coverage_test.go` — invokes `CheckCoverage()`, fails test on non-empty violations

2. Invoke the same function at the end of capmon pipeline operations:
   - `syllago capmon run` — after generate steps, before exit
   - `syllago capmon seed` — after direct YAML mutations
   - Non-zero exit + violation list printed (which cell, which axis disagrees)

3. Scope — 6 content types × 13 providers = 78 cells:
   - Covered: Rules, Skills, Agents, Commands, Hooks, MCP
   - Excluded: Loadouts (syllago-specific, single-provider by design, not a capmon concept). Document rationale as a comment at the top of `coverage.go`.

4. Assertions (4 per cell):
   - **Go ↔ source manifest:** `prov.SupportsType(ct) == (provider-sources/<slug>.yaml content_types.<ct>.supported != false AND has ≥1 source)`
   - **Go ↔ format YAML:** `prov.SupportsType(ct) == (provider-formats/<slug>.yaml content_types.<ct> has non-stub entry AND status != "unsupported")`
   - **Go internal — config map implies supported:** `ConfigLocations[ct]` set ⇒ `SupportsType(ct) == true` (catches the Amp hooks drift specifically)
   - **Go internal — install-dir parity:** `InstallDir(home, ct) != ""` ⇔ `SupportsType(ct) == true`

5. Non-stub definition for format YAML entries:
   - Counts as non-stub if `status: supported` OR has ≥1 `sources:` entry
   - Counts as stub if `status: unsupported` OR `sources: []`

6. Repo root discovery: walk up from `os.Getwd()` until finding `go.mod`, fail fast with a clear message if missing. Works from any nested test directory.

**Adjacent follow-up** (not Q4 scope): capmon pipeline reliability — auto-escalation when `capmon run` fails repeatedly. Tracked as **bd issue `syllago-cnoxd`**.

**Would have caught:**
- Roo Code commands drift (Phase 1a driver) — Go false, YAML would-be true
- Amp hooks drift (Q3 finding) — `ConfigLocations[Hooks]` wired but `SupportsType(Hooks) == false`
- Cline commands, Windsurf commands (Q3 workflow mapping) — Go false, upstream supports

**Implementation timing:** add the test alongside Phase 1 Go code changes so the harness exists as the flips happen.

### Q5 — Crush successor status to OpenCode — RESOLVED

Crush and `sst/opencode` are **sibling successors** to the archived `opencode-ai/opencode`, not parent/child. Verified 2026-04-17:

- `opencode-ai/opencode` existed first (single project, now archived).
- The original author was hired by Charm and continued the work there, relaunching it as **Crush** under the proprietary Charm License. `charmbracelet/crush` created 2025-05-21; `fork: false`, `parent: null`, `source: null` per GitHub API — not a GitHub fork.
- In parallel, the SST team forked `opencode-ai/opencode` into `sst/opencode` (MIT-licensed, active).
- Both diverged from the same archived ancestor. Separate codebases, separate teams, separate content formats.

**Comment block for `crush.yaml` (Phase 2e):**

> Crush is an independent Charm project. Its lineage traces to the archived `opencode-ai/opencode`, whose original author continued the work at Charm. `sst/opencode` is a separate fork of the same archived ancestor by a different team. Crush and `sst/opencode` are sibling successors, not parent/child. Licensed under the Charm License (proprietary).

**Sources:**
- `https://github.com/charmbracelet/crush/discussions/360` ("Difference between this and opencode?")
- `https://github.com/charmbracelet/crush/issues/1097` (community confusion around the rename/fork)
- GitHub API: `repos/charmbracelet/crush` metadata

---

## Appendix A — Verified Ground Truth Matrix

For each provider × content type, what the upstream runtime actually supports (verified 2026-04-17):

**Initial 5-provider scope** (drove this plan):

| Provider | Rules | Skills | Hooks | MCP | Commands | Agents |
|----------|-------|--------|-------|-----|----------|--------|
| Cursor | NATIVE | NATIVE (2.4+, Anthropic standard) | NATIVE | NATIVE | NATIVE (1.6+) | NATIVE (2.4+ subagents) |
| Roo Code | NATIVE | NATIVE (3.46+, SkillsManager) | unsupported | NATIVE | **NATIVE (3.25+) — Go missing** | NATIVE (Custom Modes) |
| Zed | NATIVE | unsupported | unsupported | NATIVE | unsupported (built-in) | unsupported (built-in) |
| OpenCode | NATIVE (AGENTS.md) | NATIVE (cross-provider convention) | unsupported | NATIVE | NATIVE | **NATIVE — earlier "built-in only" was wrong fork** |
| Crush | NATIVE (AGENTS.md) | NATIVE (cross-provider convention) | unsupported | NATIVE | unsupported | unsupported |

**Regression-sweep audit** (Q3 — remaining 10 providers, 11 "no" cells verified against live upstream docs):

| Provider | Cell | Go says | Upstream verdict | Status |
|----------|------|---------|------------------|--------|
| amp | agents | false | No native format (AGENTS.md + built-in Task subagents only) | ✅ correct |
| amp | commands | false | No user-definable commands (built-in palette only) | ✅ correct |
| **amp** | **hooks** | **false** | **NATIVE — `amp.hooks` array in settings JSON; events `tool:pre-execute`/`tool:post-execute`** | ❌ **DRIFT → Phase 1c** |
| cline | skills | false | NATIVE — `SKILL.md` in `.cline/skills/`, `.clinerules/skills/`, `.claude/skills/` | ⚠️ known gap → Phase 1e |
| cline | agents | false | Runtime-only (prompted, not file-based) | ✅ correct |
| **cline** | **commands** | **false** | **Workflows: `.clinerules/workflows/*.md` invoked as `/filename.md`** | ❌ **MAP → Phase 1d** |
| kiro | commands | false | No commands concept (nav: Steering, Powers, Hooks, Specs, Custom Agents) | ✅ correct |
| pi | agents | false | No native format; subagents via extension SDK | ✅ correct |
| pi | mcp | false | No MCP support (not in settings, not in repo tree) | ✅ correct |
| windsurf | agents | false | Fast Context built-in; Agent Command Center is runtime UI | ✅ correct |
| **windsurf** | **commands** | **false** | **Cascade workflows: `.windsurf/workflows/*.md` invoked as `/workflow-name`** | ❌ **MAP → Phase 1d** |

Providers with no "no" cells in Q3 audit (nothing to verify): claude-code, codex, copilot-cli, factory-droid, gemini-cli.

**Q3 verification sources (live 2026-04-17):**
- Amp hooks: `https://ampcode.com/manual/hooks.md`
- Amp agents/commands: `https://ampcode.com/manual`
- Cline skills: `https://docs.cline.bot/customization/skills.md`
- Cline workflows: `https://docs.cline.bot/customization/workflows.md`
- Cline subagents: `https://docs.cline.bot/features/subagents.md`
- Cline llms.txt: `https://docs.cline.bot/llms.txt`
- Kiro CLI: `https://kiro.dev/docs/cli/`
- Kiro docs index: `https://kiro.dev/docs/`
- Pi repo tree: `github.com/badlogic/pi-mono` (recursive git tree API)
- Pi settings: `https://raw.githubusercontent.com/badlogic/pi-mono/main/packages/coding-agent/docs/settings.md`
- Windsurf workflows: `https://docs.windsurf.com/windsurf/cascade/workflows.md`
- Windsurf Agent Command Center: `https://docs.windsurf.com/windsurf/agent-command-center.md`
- Windsurf llms.txt: `https://docs.windsurf.com/llms.txt`

## Appendix B — Source URLs (verified)

- Cursor changelog 2.4: `https://cursor.com/changelog/2-4`
- Cursor skills: `https://cursor.com/docs/skills`
- Cursor commands: `https://cursor.com/docs/context/commands`
- Cursor subagents: `https://cursor.com/docs/context/subagents`
- Roo Code skills example: `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/.roo/skills/roo-translation/SKILL.md`
- Roo Code commands example: `https://raw.githubusercontent.com/RooVetGit/Roo-Code/main/.roo/commands/commit.md`
- OpenCode agents (active project): `https://opencode.ai/docs/agents/`
- OpenCode agent system reference: `https://deepwiki.com/sst/opencode/3.2-agent-system`

## Appendix C — Files touched (forward-looking inventory)

**syllago repo:**
- `cli/internal/provider/roocode.go` — Phase 1a
- `cli/internal/provider/amp.go` — Phase 1c (hooks drift)
- `cli/internal/provider/cline.go` — Phase 1d (commands = workflows)
- `cli/internal/provider/windsurf.go` — Phase 1d (commands = workflows)
- `cli/internal/provider/*.go` (regression sweep) — Phase 1b (done as Q3)
- `docs/provider-formats/cursor.yaml` — Phase 2a
- `docs/provider-formats/roo-code.yaml` — Phase 2b
- `docs/provider-formats/zed.yaml` — Phase 2c
- `docs/provider-formats/opencode.yaml` — Phase 2d
- `docs/provider-formats/crush.yaml` — Phase 2e (new file)
- `docs/provider-formats/amp.yaml` — Phase 2f (add hooks entry)
- `docs/provider-formats/cline.yaml` — Phase 2g (add commands entry)
- `docs/provider-formats/windsurf.yaml` — Phase 2h (add commands entry)
- `docs/provider-sources/roo-code.yaml` — Phase 3a
- `docs/provider-sources/{cursor,zed,opencode,crush}.yaml` — Phase 3b
- `docs/provider-sources/{cline,windsurf}.yaml` — Phase 3b (annotate workflows as commands)
- `CHANGELOG.md` — Phase 5b
- `cli/internal/provider/coverage_test.go` — Q4 (new test, post-release follow-up)

**syllago-docs repo:**
- `src/data/providers/*.json` — Phase 6 (auto-regenerated)
- `src/content/docs/using-syllago/providers/**/*.mdx` — Phase 6 (auto-regenerated)
- `CHANGELOG.md` — Phase 6
