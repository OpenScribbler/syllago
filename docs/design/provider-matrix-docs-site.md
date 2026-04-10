# Provider Compatibility Matrix — Docs Site Feature

**Bead:** syllago-pdyxv
**Priority:** P2
**Date:** 2026-03-27

---

## Vision

An interactive, verified provider compatibility matrix on the syllago docs site (Astro Starlight). This would be **the** authoritative reference for "what does each AI coding tool support?" — positioning syllago as the go-to resource even for people who haven't installed it.

Each data point shows:
- **What** the provider supports
- **When** it was last verified
- **How** it was verified (API fetch, scrape, manual)
- **Confidence level** (high, medium, best-effort)
- **Source URL** linking to official docs

## Why This Matters

1. **Top-of-funnel:** People searching "does Cursor support hooks?" find syllago's matrix. They discover syllago.
2. **Trust:** Verification metadata shows rigor. Nobody else does this.
3. **Authority:** syllago becomes the reference for AI coding tool capabilities.
4. **Migration help:** Comparison answers "what changes if I switch from X to Y?" — leading to "syllago can do that conversion."
5. **Spec credibility:** The hook event matrix shows syllago has mapped the entire landscape.

---

## Data Model

Content collection with per-provider YAML files:

```yaml
# src/content/providers/claude-code.yaml
slug: claude-code
display_name: Claude Code
vendor: Anthropic
website: https://code.claude.com
changelog: https://code.claude.com/docs/en/changelog

content_types:
  rules:
    supported: true
    verified: 2026-03-27T00:00:00Z
    method: api          # api | scrape | manual
    confidence: high     # high | medium | best-effort
    source: https://code.claude.com/docs/en/memory
    format: "CLAUDE.md + .claude/rules/*.md"
    notes: "Markdown with optional YAML frontmatter (description, alwaysApply, globs)"
  hooks:
    supported: true
    verified: 2026-03-27T00:00:00Z
    method: api
    confidence: high
    source: https://code.claude.com/docs/en/hooks
    event_count: 26
    hook_types: ["command", "http", "prompt", "agent"]
    format: ".claude/settings.json"
    notes: "26 events, matcher support, structured output, async"
  mcp:
    supported: true
    verified: 2026-03-27T00:00:00Z
    method: api
    confidence: high
    source: https://code.claude.com/docs/en/mcp
    format: ".mcp.json"
    notes: "stdio, SSE, streamable-http transports. OAuth support."
  skills:
    supported: true
    verified: 2026-03-27T00:00:00Z
    method: api
    confidence: high
    source: https://code.claude.com/docs/en/skills
    format: ".claude/skills/{name}/SKILL.md"
  agents:
    supported: true
    verified: 2026-03-27T00:00:00Z
    method: api
    confidence: high
    source: https://code.claude.com/docs/en/sub-agents
    format: ".claude/agents/{name}.md"
  commands:
    supported: true
    verified: 2026-03-27T00:00:00Z
    method: api
    confidence: high
    source: https://code.claude.com/docs/en/commands
    format: ".claude/commands/{name}.md"
```

## Confidence Tiers

| Tier | Label | Criteria | Visual |
|---|---|---|---|
| **high** | Verified | Fetched from official API or raw markdown. Parseable, diffable. | Green checkmark + date |
| **medium** | Confirmed | Scraped from docs site, human-reviewed. | Yellow checkmark + date |
| **best-effort** | Reported | Manually researched, may be outdated. | Gray info icon + date |

## Verification Methods

| Method | Description | Providers |
|---|---|---|
| api | GitHub API releases or raw markdown files | Gemini CLI, Cline, Roo Code, OpenCode, Copilot CLI |
| llms-txt | llms.txt index + raw .md URLs | Windsurf |
| schema | Machine-readable JSON Schema | OpenCode (config.json) |
| scrape | HTML docs site, parsed | Claude Code, Cursor, Codex, Amp, Kiro |
| manual | Human reviewed docs | Fallback for any provider |

---

## Pages and Components

### 1. Provider Matrix Page (`/reference/providers`)

Full grid: providers as rows, content types as columns.

```
                  Rules  Hooks  MCP    Skills  Agents  Commands
Claude Code       [H]    [H]    [H]    [H]     [H]     [H]
Cursor            [H]    [H]    [H]    [M]     --      [M]
Gemini CLI        [H]    [H]    [H]    --      --      --
Copilot CLI       [H]    [H]    [H]    [M]     [M]     --
Windsurf          [H]    [H]    [H]    [H]     --      --
Cline             [M]    --     [M]    --      --      --
Roo Code          [M]    --     [M]    --      --      --
OpenCode          [M]    [M]    [M]    --      [M]     --
Codex             [M]    [B]    [M]    [M]     --      --
Kiro              [B]    [B]    [B]    --      --      --
Amp               [B]    --     [B]    [B]     --      --
Zed               [M]    --     [M]    --      --      --

[H] = High confidence  [M] = Medium  [B] = Best-effort  -- = Not supported
```

Features:
- Click any cell for details (format, notes, verified date, source link)
- Filter by content type ("show me all providers that support hooks")
- Sort by name, content type count, last verified date
- Toggle between "supported?" view and "detail" view

### 2. Provider Detail Pages (`/providers/{slug}`)

Full breakdown per provider:
- Content type support with format details
- Hook event list (if applicable) with native event names
- MCP schema details
- Links to official docs
- Verification metadata prominently displayed
- "Last verified" banner at top

### 3. Hook Event Matrix (`/reference/hook-events`)

All events as rows, providers as columns, showing provider-native names.

```
Canonical Event      Claude Code       Cursor              Gemini CLI
before_tool_execute  PreToolUse        preToolUse          BeforeTool
after_tool_execute   PostToolUse       postToolUse         AfterTool
session_start        SessionStart      sessionStart        SessionStart
before_prompt        UserPromptSubmit  beforeSubmitPrompt  BeforeAgent
...
```

Color-coded:
- Blue = spec-canonical event (2+ providers)
- Gray = provider-specific event (1 provider only)
- Dash = not supported

### 4. Comparison Component (reusable)

Select 2-3 providers to compare side-by-side. Highlights differences. Useful for migration planning.

---

## Automation Pipeline

Ties into the provider monitoring system (bead syllago-jbtjw):

1. **CI job** fetches provider changelogs + docs (weekly or on release)
2. If content changed: creates GH issue + updates provider YAML data
3. Human reviews, confirms, merges PR
4. Docs site rebuilds with updated verification dates

For **API-fetchable** providers (Gemini, Windsurf, Copilot, OpenCode): mostly automated. Fetch, diff, update YAML, create PR. Human reviews and merges.

For **scrape/manual** providers (Cursor, Claude Code, Kiro, Amp): automation flags "changelog changed," human investigates.

---

## Relationship to Existing Work

| Artifact | Role |
|---|---|
| `docs/provider-formats/*.md` (CLI repo) | Source data (detailed format references) |
| `src/content/providers/*.yaml` (docs site) | Presentation layer (structured data for components) |
| Provider monitoring (syllago-jbtjw) | Update pipeline (detects changes, creates issues) |
| Provider formats skill | Manual update tool (currently used, feeds into this) |

---

## Tech Implementation

- **Astro content collection** for provider YAML files with Zod schema validation
- **Custom Astro components:** MatrixGrid, ProviderCard, HookEventTable, ComparisonView
- **Static generation** — no client-side data fetching, all rendered at build time
- Optional: JSON API route (`/api/providers.json`) for other tools to consume the matrix data

---

## Open Questions

1. Auto-generate provider detail pages from YAML, or hand-write MDX with embedded data?
2. Show ALL hook events (including provider-specific) or only spec-canonical?
3. Badge for rapidly-changing providers (Kiro, Amp) — "beta" or "unstable"?
4. Expose matrix data as JSON API for programmatic consumption?
5. How to handle providers syllago doesn't support yet but has data for (e.g., Goose, JetBrains AI)?
