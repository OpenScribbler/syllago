# Release Readiness Phase 5: Docs Site + Demos

*Design date: 2026-03-19*
*Status: Design Complete*
*Phase: 5 of 5*
*Dependencies: All previous phases (documents what was built, demos show final product)*

## Overview

Create the docs-site infrastructure for error codes, add the conversion reference page, produce VHS demo tapes, and verify distribution. This is the "polish and ship" phase.

## Context

The syllago-docs site lives at `../syllago-docs/` (OpenScribbler/syllago-docs). It's built with Astro Starlight and already has CLI reference infrastructure (gendocs, commands.json sync). This phase adds error code documentation and demo content.

---

## Work Items

### 1. Error Codes Astro Content Collection (syllago-docs)

**1a. Content collection schema**

Create a Zod schema for error codes in `syllago-docs/`:
```typescript
// src/content/config.ts (extend existing)
const errorsCollection = defineCollection({
  type: 'content',
  schema: z.object({
    code: z.string(),           // "CATALOG_001"
    title: z.string(),          // "No catalog found"
    category: z.string(),       // "catalog"
    severity: z.enum(['error', 'warning', 'info']),
    summary: z.string(),        // One-line description
  }),
});
```

**1b. Error page template**

Each error code gets a page at `/errors/<code>/` with:
- Error code and title
- What it means (explanation)
- Common causes (bullet list)
- How to fix it (step-by-step)
- Example output (code block showing what the user sees)
- Related errors (links)

**1c. Error index page**

Auto-generated index at `/errors/` listing all error codes grouped by category (catalog, registry, provider, install, convert).

**1d. Initial error pages**

Populate with the error codes defined in Phase 2. Each page starts with the code, summary, and basic fix steps. Troubleshooting examples added manually over time as we encounter real user issues.

### 2. Conversion Compatibility Page (syllago-docs)

A reference page at `/reference/conversion/` showing:
- The hub-and-spoke conversion model (Claude Code as canonical)
- Per-content-type compatibility matrix (same data as README table, expanded)
- Provider-specific format notes (Codex TOML, Zed context_servers, etc.)
- What happens during conversion (metadata preservation, format translation)

Positive framing throughout — this is a feature showcase, not a limitations page.

### 3. VHS Demo Tapes

Three VHS `.tape` files that produce GIF recordings for the README and future marketing.

**Demo 1: TUI Registry Browsing**
- Launch TUI
- Navigate to Registries
- Add a syllago example registry (use one from OpenScribbler org)
- Show items populating in the library
- Browse items, preview one, show the split-view detail
- ~20 seconds

**Demo 2: Claude Code Loadout**
- Create a loadout for Claude Code (include skills, agents, hooks)
- Apply the loadout
- Launch Claude Code showing the loadout content is active
- Visually demonstrate: skill loads, agent available, hook honored
- ~30 seconds

**Demo 3: Cross-Provider Conversion**
- Show a Claude Code skill with rich frontmatter
- Convert to Gemini CLI format (`syllago convert`)
- Show the converted output — frontmatter preserved in provider-appropriate format
- Create the same loadout but for Gemini CLI
- Show the loadout applies cleanly
- ~30 seconds

**Tape files** stored in `docs/vhs/` in the syllago repo. GIF outputs referenced by README.

### 4. `go install` Verification

Manual pre-release verification:
```bash
go install github.com/OpenScribbler/syllago/cli/cmd/syllago@latest
syllago version
```

Test both `@latest` and `@v<current-version>`. Verify module path is discoverable and binary runs correctly.

Document this as a step in the release checklist (VERSIONING.md from Phase 4).

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Error docs format | Astro content collection | Structured, auto-indexed, consistent with existing site infrastructure |
| Error page template | Code + explanation + causes + fix | Comprehensive but expandable over time |
| Demo scenarios | Registry browse, Claude Code loadout, cross-provider conversion | Narrative arc: discover → use → port. Almost an advertisement. |
| Demo length | 20-30 seconds each | Short enough for README, detailed enough to show value |
| Tape file location | docs/vhs/ in syllago repo | Co-located with source, referenced by README |

---

## Out of Scope

- Full docs content writing (getting started guide, provider reference, etc.) — separate workstream
- Video production for YouTube (future marketing, uses same demo content)
- Automated `go install` CI check (manual verification sufficient for v1)
- Error page troubleshooting examples (added manually over time)
