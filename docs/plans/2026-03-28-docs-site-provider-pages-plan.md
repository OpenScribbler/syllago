# Docs Site Provider Pages — Implementation Plan

**Date:** 2026-03-28
**Status:** Ready to execute
**Design ref:** `docs/design/provider-matrix-docs-site.md`
**Capability schema draft:** `docs/design/capability-schema-draft.yaml`

---

## What's Done

The pipeline from CLI → docs site is working:

```
provider/*.go → _genproviders → providers.json → sync-providers.ts → providers/*.mdx
```

- 12 provider pages auto-generated from Go code
- Compatibility matrix on index page
- Per-provider detail pages with support tables, formats, paths
- Pre-push hook validates providers.json freshness
- Docs site builds clean with generated pages

## What's Missing

The generated pages currently only contain **structural data** (what's supported, install methods, paths). The design doc envisions richer content that requires **capability data** (hook events, MCP details, format nuances). Here's what needs to happen.

---

## Phase 1: Enrich providers.json (CLI repo)

**Goal:** Add the data that makes the provider pages actually useful.

### 1a. Add hook event data to _genproviders

The converter already has hook event mappings in `cli/internal/converter/hooks.go`. Export them.

**What to add per provider:**
- `hookEvents`: list of native event names (e.g., `["PreToolUse", "PostToolUse", ...]`)
- `hookTypes`: list of supported hook handler types (e.g., `["command"]` or `["command", "http", "prompt", "agent"]`)
- `hookConfigLocation`: where hooks are configured (e.g., `.claude/settings.json`)

**Source:** `cli/internal/converter/hooks.go` has `HookEvents` map and `HookAdapter` interface. The toolmap already defines per-provider event names.

### 1b. Add MCP transport data

**What to add per provider:**
- `mcpTransports`: list of supported transports (e.g., `["stdio", "sse", "streamable-http"]`)
- `mcpConfigLocation`: where MCP is configured (e.g., `.mcp.json`)

**Source:** This data currently lives in the provider format docs (`docs/provider-formats/*.md`) and the source manifests we just built. May need to codify it in Go.

### 1c. Add rules format details

**What to add per provider:**
- `rulesFileName`: primary rules file name (e.g., `CLAUDE.md`, `GEMINI.md`, `.cursorrules`)
- `rulesFrontmatter`: supported frontmatter fields (e.g., `["description", "alwaysApply", "paths"]`)
- `rulesHierarchy`: how rules are discovered and prioritized

**Source:** Already in provider Go definitions (EmitPath, DiscoveryPaths). Frontmatter fields need to be codified.

### 1d. Update providers.json schema and tests

- Update `ProviderCapEntry` and `ContentCapability` Go types
- Update `genproviders_test.go` to verify new fields
- Regenerate `providers.json`

---

## Phase 2: Enrich sync-providers.ts (docs repo)

**Goal:** Render the new data in the generated MDX pages.

### 2a. Hook events section per provider

Add a "Hook Events" table to provider pages that have hooks:

```
| Event | Native Name | Category |
|-------|------------|----------|
| before_tool_execute | PreToolUse | Tool |
| after_tool_execute | PostToolUse | Tool |
| session_start | SessionStart | Lifecycle |
```

### 2b. MCP details section per provider

Add an "MCP Configuration" section showing:
- Supported transports
- Config file location
- Any provider-specific MCP quirks

### 2c. Rules format section per provider

Show the rules file name, frontmatter fields, and hierarchy/discovery order.

### 2d. Update index page

- Add a "Hooks support" column showing event count per provider
- Add quick-reference icons for MCP transport support

---

## Phase 3: Hook Event Matrix Page (docs repo)

**Goal:** The cross-provider hook event comparison page from the design doc.

### 3a. Add hook event matrix data to providers.json

This needs a cross-provider mapping: canonical event → each provider's native name.

**Source:** `cli/internal/converter/hooks.go` has this in the `HookEvents` map.

### 3b. Create sync-hook-events.ts or extend sync-providers.ts

Generate a dedicated `/reference/hook-events` page with the full matrix:

```
| Canonical Event      | Claude Code    | Cursor          | Gemini CLI   | ...
|---------------------|----------------|-----------------|--------------|
| before_tool_execute | PreToolUse     | preToolUse      | BeforeTool   |
| after_tool_execute  | PostToolUse    | postToolUse     | AfterTool    |
```

### 3c. Add to sidebar

Add under Reference section alongside hooks-v1.mdx.

---

## Phase 4: Provider Comparison Component (docs repo)

**Goal:** Interactive side-by-side comparison of 2-3 providers.

### 4a. Astro component

Build a `<ProviderComparison>` component that:
- Lets users select 2-3 providers
- Shows differences highlighted
- Useful for migration planning ("what changes if I switch from Cursor to Claude Code?")

### 4b. Dedicated comparison page

`/reference/provider-comparison` with the component embedded.

---

## Phase 5: Automation (CI/CD)

### 5a. Add providers.json to release workflow

Already done — release.yml updated to generate and publish providers.json as a release asset.

### 5b. Wire provider-monitor into CI

GitHub Actions cron job (daily or twice daily):
- Runs `go run ./cmd/provider-monitor`
- If any URLs broken or versions drifted: creates a GitHub issue
- If all clean: no-op

### 5c. Auto-sync on docs deploy

Already done — `prebuild` script in docs repo runs sync-providers.ts before Astro build.

---

## Dependency Order

```
Phase 1 (CLI enrichment) → Phase 2 (docs rendering) → Phase 3 (hook matrix)
                                                     → Phase 4 (comparison)
Phase 5 (CI) can run in parallel with any phase
```

Phase 1 is the blocker — until providers.json has richer data, the docs pages can't show it.

---

## Open Design Questions (from capability-schema-draft.yaml)

These need answers before Phase 1 implementation:

1. **Platform-specific paths** — Cline's MCP settings path differs by OS. Keep platform logic in Go, just export the "typical" path? Or add a `platformPaths` field?

2. **Hook detail level** — Just event names in providers.json, or full I/O schemas? Recommendation: event names + handler types. Link to hook spec for full schemas.

3. **Tool name mappings** — The converter maps tool names between providers. Export as part of providers.json (per-provider `toolNames` field), or separate manifest? Recommendation: separate `tool-mappings.json` — it's cross-provider, not per-provider.

4. **Frontmatter fields** — Do we list all supported frontmatter fields per content type per provider? This is valuable for conversion accuracy. Recommendation: yes, include it — it's small and high-signal.
