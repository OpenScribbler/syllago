# Capmon Phase 6: Full Recognition Pipeline

**Date:** 2026-04-16
**Status:** Planning
**Scope:** Complete the capmon recognition pipeline for all content types across all providers

## Background

The capmon pipeline has 4 stages:
1. **Fetch** — download source docs from provider URLs
2. **Extract** — parse fetched content into structured fields
3. **Diff** — compare extracted fields against capability baselines
4. **Review** — create PRs/issues for drift

Stages 1-2 work across all 15 providers and 6 content types. Stages 3-4 depend on **recognizers** — functions that map extracted fields to canonical capability dot-paths. Currently:

- **2 of 16 providers** have real recognizers (crush, roo-code — skills only)
- **14 providers** have stub recognizers returning empty maps
- **Only skills** has recognition infrastructure (helpers, seeder specs). Rules, hooks, MCP, agents, and commands have zero recognition logic.
- **10 of 15 providers** have stale cache data from an older pipeline version (dash-named dirs that were never re-extracted)
- `extract_html` is the only extractor sub-package with no test file

## Epic Chain

Epics are ordered by dependency — each one builds on the previous. Work one epic at a time to avoid context overload.

---

### Epic 0: Foundation — Cache, schema, and infra

**Goal:** Clean cache data, updated capability JSON schema with new fields, recognition infra ready for multi-content-type work. Everything downstream (Epics 1-6) depends on this.

#### 0a: Cache refresh — DONE
- [x] Chromedp stealth changes committed (b229223)
- [x] Full pipeline run for all 15 providers (run `i6h4hamw`, 2026-04-16)
- [x] 13/15 providers fully clean
- [ ] Fix 2 broken source URLs (amp `rules.2`, copilot-cli `skills.1`) — research in progress
- [ ] Re-fetch amp + copilot-cli after URL fixes
- [ ] Clean up orphan dash-named cache dirs

#### 0b: Capability JSON schema update
Align with the provider convention pages redesign (see `../syllago-docs/docs/plans/2026-04-16-provider-convention-pages-redesign.md`). This is the schema contract between syllago CLI (generates capability JSON) and syllago-docs (renders it).

**On `canonicalMappings` entries — add:**
- `provider_field` (string, optional) — actual native field name (e.g., `"name"`, `"description"`)
- `extension_id` (string, optional) — links to the providerExtension that describes the same concept

**On `providerExtensions` entries — add/change:**
- `description` → `summary` — one sentence, ~150 chars max (breaking rename, absorbed by enrichment pass)
- `provider_field` (string, optional) — native field name when extension describes a specific field
- `conversion` (enum, required) — `translated` | `embedded` | `dropped` | `preserved` | `not-portable`

**Also:**
- Shorten `mechanism` strings (no longer need to carry native field name)
- Update JSON schema definition used for validation

#### 0c: Generator update
Update the Go code that generates capability JSON files to emit the new fields. Key files:
- Capability JSON generator (needs to output `provider_field`, `extension_id`, `summary`, `conversion`)
- `frontmatter_registry.go` — already knows field names per provider (source for `provider_field`)
- Converter functions in `skills.go`, `rules.go`, `commands.go`, `embed.go`, `compat.go` — source for `conversion` values

#### 0d: Infra generalization
- Generalize `SeederSpecPath()` to accept content type param (currently hardcoded to `-skills.yaml`)
- Rename recognizer functions from `recognizeXxxSkills` → `recognizeXxx` (each function will handle all content types for its provider)
- Add recognition helper patterns for non-skills content types
- Add `extract_html` test file (data quality foundation — several providers depend on HTML extraction)

**No second cache refresh needed.** Cache holds raw extracted fields (input). Schema/generator changes affect how recognizers *output* capability data. Epics 1-6 will produce enriched data with all new fields from the start.

---

### Epic 1: Skills recognition — Finish the 14 stubs

**Goal:** All 16 providers have working skills recognizers.

Closest to done. Crush + roo-code show the pattern. The `recognizeSkillsGoStruct` helper handles providers that implement the Agent Skills open standard.

#### 1a: Missing seeder specs
Create `-skills.yaml` seeder specs for: cursor, crush, opencode, roo-code, zed

#### 1b: Implement recognizers
Implement the 14 stub functions. Each recognizer populates both the existing dot-path keys AND the new schema fields (`provider_field`, `conversion`, `summary`). Batch by pattern:
- **GoStruct pattern** (providers implementing Agent Skills standard): likely claude-code, cline, codex, copilot-cli, factory-droid, kiro, pi, amp
- **Custom format** (provider-specific): cursor, windsurf, gemini-cli, opencode, zed

---

### Epic 2: Rules recognition

**Goal:** All providers with rules support have working rules recognizers.

Rules are core to syllago's cross-provider portability. Most providers have rich rules data already in the cache.

#### 2a: Schema + helpers
- Define canonical dot-path schema for rules capabilities (file format, file locations, frontmatter fields, glob support, always-apply behavior, etc.)
- Create recognition helper functions for common rules patterns

#### 2b: Seeder specs
- Create `-rules.yaml` seeder specs per provider

#### 2c: Implement recognizers
- Expand each provider's `recognizeXxx()` function to handle rules fields

---

### Epic 3: Hooks recognition

**Goal:** All providers with hooks support have working hooks recognizers.

Hooks are the most complex content type — events, tool names, match conditions, and config format all vary significantly by provider. The canonical hook spec (`docs/spec/hooks.md`) defines the interchange format.

#### 3a: Schema + helpers
- Define canonical dot-paths for hooks capabilities (events, tool names, match conditions, config format, blocking behavior)
- Map to existing canonical event/tool name tables in `toolmap.go`

#### 3b: Seeder specs per provider

#### 3c: Implement recognizers

---

### Epic 4: MCP recognition

**Goal:** All providers with MCP support have working MCP recognizers.

#### 4a: Schema + helpers
- Define canonical dot-paths for MCP capabilities (transport types, config format, server discovery, registry support)

#### 4b: Seeder specs per provider

#### 4c: Implement recognizers

---

### Epic 5: Agents recognition

**Goal:** All providers with agents support have working agents recognizers.

Agents are the least standardized content type. Many providers don't have a native agent format — they rely on cross-provider conventions like AGENTS.md.

#### 5a: Schema + helpers
#### 5b: Seeder specs per provider
#### 5c: Implement recognizers

---

### Epic 6: Commands recognition

**Goal:** All providers with commands support have working commands recognizers.

Several providers mark commands as `supported: false`. Scope may be smaller than other epics.

#### 6a: Schema + helpers
#### 6b: Seeder specs per provider
#### 6c: Implement recognizers

---

## Provider × Content Type Matrix

| Provider | skills | rules | hooks | mcp | agents | commands |
|----------|--------|-------|-------|-----|--------|----------|
| amp | stub | — | — | — | unsupported | unsupported |
| claude-code | stub | — | — | — | — | — |
| cline | stub | — | — | — | unsupported | — |
| codex | stub | — | — | — | — | — |
| copilot-cli | stub | — | — | — | — | — |
| crush | **done** | — | unsupported | — | unsupported | unsupported |
| cursor | stub | — | — | — | unsupported | unsupported |
| factory-droid | stub | — | — | — | — | — |
| gemini-cli | stub | — | — | — | unsupported | — |
| kiro | stub | — | — | — | — | unsupported |
| opencode | stub | — | unsupported | — | — | — |
| pi | stub | — | — | — | unsupported | — |
| roo-code | **done** | — | unsupported | — | — | — |
| windsurf | stub | — | — | — | — | — |
| zed | stub | — | unsupported | — | — | — |

Legend: **done** = real recognizer, **stub** = returns empty map, **—** = not yet started, **unsupported** = `supported: false` in manifest

## Ordering Rationale

1. **Epic 0 first** — everything downstream needs clean cache data and generalized infra
2. **Skills (1)** — 90% done, fastest to complete, proves the pattern
3. **Rules (2)** — highest value for syllago's portability story
4. **Hooks (3)** — second highest value, most complex content type
5. **MCP (4)** — moderately standardized, important for tool ecosystem
6. **Agents (5)** — least standardized, many providers lack native support
7. **Commands (6)** — sparsest coverage, lowest urgency

Epics 2-6 follow an identical pattern (schema → specs → recognizers), so velocity should increase as we go.

## Cross-Repo Coordination

This work spans two repos:

| Repo | Work | When |
|------|------|------|
| **syllago** (this repo) | Epic 0 (cache, schema, generator, infra) + Epics 1-6 (recognizers) | Start here |
| **syllago-docs** | Zod schema update, component rebuild, TOC fix, page routes | After Epic 0b-0c land (schema contract must be stable) |

The schema contract (capability JSON shape) is the interface. Once Epic 0b-0c stabilize the new fields, syllago-docs work can proceed in parallel with Epics 1-6.

See: `../syllago-docs/docs/plans/2026-04-16-provider-convention-pages-redesign.md` for the full downstream plan.
