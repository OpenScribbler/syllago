# Content-Based Classification Gap Analysis

**Date:** 2026-04-01
**Context:** Discovered during real-world testing of the content discovery redesign (CDR)

---

## The Problem

The analyzer pipeline is entirely pattern-driven:

```
Walk (finds files) → MatchPatterns (filters to known globs) → Classify (reads content)
```

Step 2 is the gate. If a file doesn't match a hardcoded provider glob pattern, it never reaches Classify — regardless of how obviously AI content it is. This means repos with non-standard structures get **zero detection**, even when the content is clearly agent/skill/rule content.

## Real-World Evidence

Tested against repos in `~/.local/src/`:

| Repo | Result | Why |
|------|--------|-----|
| **ai-tools** | 39 items | Standard syllago layout — works perfectly |
| **syllago-kitchen-sink** | 12 items | Multi-provider — all 7 providers detected, dedup working |
| **phyllotaxis** | 6 items | CC rules, skills, hooks — all found |
| **PAI** | **0 items** | `Packs/pai-*-skill/` structure — no matching patterns |
| **BMAD-METHOD** | **0 items** | `src/bmm/agents/*.agent.yaml` — no matching patterns |

PAI has ~20 skills and BMAD has ~9 agents. Both return 0.

## Why a Custom Directory Flag Wouldn't Help

Even if someone told the wizard "look in `Packs/`", the MatchPatterns step would still filter everything out. The patterns are hardcoded to specific structures like `skills/*/SKILL.md` or `.claude/agents/*.md`. A file at `Packs/pai-redteam-skill/index.ts` matches none of them.

## The Architectural Gap

The 11 detectors answer: **"Is this file a known provider's content?"**

What's missing is a detector that answers: **"Is this file AI content, regardless of provider?"**

This requires content-based classification — reading the file and making a judgment based on:
- File extension (`.md`, `.yaml`, `.toml`, `.json`)
- Frontmatter fields (`name:`, `description:`, `trigger:`, `model:`)
- Directory name keywords (`agent`, `skill`, `hook`, `rule`, `command`, `prompt`)
- File name patterns (`SKILL.md`, `AGENT.md`, `*.agent.yaml`, `*.agent.md`)
- Content structure (instruction-like prose, configuration objects)

## Community Research Reference

See `docs/research/2026-03-31-community-repo-content-patterns.md` for analysis of ~44 repos. Key findings relevant to this gap:

**Structures our detectors cover (vast majority of repos):**
- All standard provider paths (`.claude/`, `.cursor/`, `.github/`, etc.)
- Top-level canonical directories (`skills/`, `agents/`, `rules/`, etc.)

**Structures we miss:**
1. **Categorized subdirectories** — `agents/<category>/<agent>.md` (deeper than `agents/*/*.md`)
2. **Completely custom layouts** — `Packs/pai-*-skill/`, `src/bmm/agents/`
3. **Meta-tools** — `ai-rules/`, `.ruler/`, `prompts/*/aiprompt.json` (each with own config format)
4. **Example directories** — `examples/agents/`, `examples/skills/`
5. **Extended plugin paths** — `plugins/*/hooks/*.py` (only `plugins/*/agents/*.md` is covered)

## Design Questions for Next Session

1. **Where does the content-based classifier fit in the pipeline?** Does it run as another detector (11th spoke) or as a fallback after all pattern-based detectors return empty?

2. **What confidence level?** Pattern-based detection gets 0.80-0.95. Content-based should be lower (0.40-0.60?) since it's guessing. But too low means items get dropped by the skip threshold (0.50).

3. **How to avoid false positives?** A README.md in `agents/` isn't an agent. A `.yaml` file in a `config/` dir isn't a skill. The heuristic needs negative signals too.

4. **Should the wizard allow user-directed discovery?** "Scan this directory as skills" — bypassing pattern matching entirely and just classifying everything in a specified path as a specified type. This is simpler than heuristics and gives the user full control.

5. **Quick wins vs full solution:** Adding `agents/<category>/*.md`, `examples/agents/*.md`, and `*.agent.yaml` patterns to existing detectors would catch several repos without architectural changes. Is that enough for v1?

## Quick Win Candidates (No Architecture Change)

These are pattern additions to existing detectors that would catch more content:

| Pattern | Detector | Would catch |
|---------|----------|-------------|
| `agents/*/*/*.md` | TopLevel | categorized agent directories (repos 3, 6, 10) |
| `examples/agents/*.md` | TopLevel | example directories (repo 9) |
| `examples/skills/*/SKILL.md` | TopLevel | example directories (repo 9) |
| `examples/commands/*.md` | TopLevel | example directories (repo 9) |

These would NOT catch PAI or BMAD since their structures are truly unique.
