# Release Readiness Phase 3: Content + Onboarding

*Design date: 2026-03-19*
*Status: Design Complete*
*Phase: 3 of 5*
*Dependencies: Independent, but Phase 2's error system and provider detection improvements enhance this*

## Overview

Create built-in content that makes syllago feel alive on first launch, and build the onboarding experience that guides new users through their first interaction. This is the "first 5 minutes" phase.

## Context

The UX agent audit identified that a new user installing syllago encounters an empty library with no guidance — a dead end. Three distinct user journeys need support:

1. **Solo dev with AI tools installed** — Guide them to discover and organize existing content
2. **Team member with a registry URL** — Guide them to add the team registry
3. **Explorer with nothing** — Give them something to see and do

---

## Work Items

### 1. `syllago-quickstart` Skill

A narrative getting-started guide (distinct from `syllago-guide` which is a reference).

**Content structure:**
- "Your first 5 minutes with syllago" narrative
- Step 1: Discover what you have (`syllago add --from claude-code`)
- Step 2: Browse your library (TUI walkthrough)
- Step 3: Install to another provider (`syllago install skills/my-skill --to cursor`)
- Step 4: Add a registry for community content
- Step 5: Create a loadout for your team

**Format:** Standard syllago skill (markdown with frontmatter). Tagged as `builtin` in `.syllago.yaml`.

### 2. `syllago-starter` Loadout

A built-in loadout that bundles the meta-tools for immediate use.

**Contents:**
- `syllago-guide` skill (reference)
- `syllago-quickstart` skill (getting started)
- `syllago-author` agent (content creation helper)
- `syllago-import` skill (import workflow reference)

**Provider target:** Claude Code (as canonical format). Users can convert for other providers.

**Purpose:** Gives the explorer journey something concrete to apply. "Apply this loadout to set up syllago's built-in tools in your provider."

### 3. Welcome Card (TUI First-Run Experience)

When the library is empty on first launch, show a welcome card instead of an empty list.

**Detection logic:**
1. Check library item count (if 0, show welcome)
2. Check `provider.DetectProviders()` (respecting PathResolver custom paths from Phase 2)
3. Check configured registries count

**Three journey paths rendered based on detection:**

**Journey A: "You have AI tools installed"** (providers detected, no registry)
```
Welcome to Syllago

Detected: Claude Code, Cursor

→ Press 'a' to discover content from your existing tools
→ Or press 'R' to add a community registry
```

**Journey B: "You have a registry URL"** (no providers detected OR providers detected, with instruction to add registry)
```
Welcome to Syllago

→ Press 'R' to go to Registries, then 'a' to add your team's registry
→ Browse and install content for any of 11 supported AI coding tools
```

**Journey C: "Explorer mode"** (nothing detected)
```
Welcome to Syllago

→ Press 'a' to add content from a local path or git URL
→ Built-in content available: run `syllago loadout apply syllago-starter`
→ Learn more: press '?' for keyboard shortcuts
```

**Implementation:** New component in TUI that renders conditionally when items list is empty. Disappears once any content exists in library. Not a modal — just the content area.

### 4. No-Providers-Detected Warning

**TUI:** Yellow inline banner at top of main view when no providers are detected:
```
⚠ No AI coding tools detected. Content can be browsed and organized,
  but won't activate until a supported tool is installed.
  Configure custom paths: syllago config paths
```

**CLI:** Warning message when running `syllago install` or `syllago add` with no providers:
```
Warning: No AI coding tools detected at default locations.
If your tools are installed at custom paths, configure them:
  syllago config paths --provider claude-code --path /custom/path
```

**False positive handling:** Offer `syllago config paths` command to set custom locations per provider. This leverages the existing `PathResolver` infrastructure. Provider detection checks custom paths first.

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Onboarding approach | Context-aware welcome card | Detects user's situation and guides accordingly |
| Built-in content | Quickstart skill + starter loadout | Gives explorers something concrete without bloating defaults |
| Welcome card persistence | Disappears when library has items | Not annoying — shows up only when needed |
| No-providers UX | TUI banner + CLI warning | Informational, not blocking. Offers config escape hatch |
| False positive handling | `config paths` command | Leverages existing PathResolver, gives users control |

---

## Out of Scope

- Full first-run wizard (detect tools → suggest registries → walk through setup) — deferred
- Example content from multiple providers — starter loadout covers this
- Auto-detection of non-standard provider installations (registry-based heuristics)
