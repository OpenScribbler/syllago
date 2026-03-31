# Session Handoff System - Design Document

**Goal:** Automatically persist structured project state between Claude Code sessions so each new session picks up where the last left off.

**Decision Date:** 2026-03-17

---

## Problem Statement

Every Claude Code session starts with a blank context. CLAUDE.md, auto-memory, and beads provide topical knowledge and task tracking, but none capture the *task-progress state* — what you were actively working on, what decisions were made, what's next. This forces re-explanation at the start of every session and risks re-debating settled decisions.

The Anthropic engineering blog ("Effective Harnesses for Long-Running Agents") demonstrated that a simple progress file — updated each session, read at the start of the next — provides surprisingly effective continuity. The pattern is: "like shift workers leaving clear handoff notes and a tidy workstation so the next person can pick up where they left off."

## Proposed Solution

A project-scoped JSON handoff document (`.handoff.json`) written by Claude at session end and loaded at session start. Claude is the primary author (highest quality, full conversation context), with a Haiku-based fallback for sessions that end abruptly. Four trigger layers ensure reliability: PreCompact hook, session close protocol, manual `/handoff` command, and Stop hook fallback.

## Architecture

### Scope: Project-Scoped Only

One handoff file per project, living at the project root (`.handoff.json`, gitignored).

- **Global context** is already covered by `MEMORY.md` + core context loading
- **Feature-scoped context** is already covered by `.develop/` state files + beads
- **Project-scoped task-progress** is the gap this system fills

### Data Flow

```
Session N:
  ┌─ SessionStart ─── reads .handoff.json ─── injects into context
  │
  │  ... work happens ...
  │
  ├─ PreCompact ────── outputs reminder to write/update handoff
  │                    (Claude writes .handoff.json)
  │
  ├─ /handoff ──────── manual trigger (same write logic)
  │
  ├─ Close protocol ── Claude writes .handoff.json
  │                    bd sync (syncs beads)
  │                    git commit + push
  │
  └─ Stop hook ─────── IF no fresh .handoff.json:
                         Haiku extracts from transcript
                         Writes .handoff.json
                       bd sync (final safety net)
```

### Write Flow (Claude-Authored)

When Claude writes the handoff (via `/handoff`, PreCompact reminder, or close protocol):

1. Run `git branch --show-current`, `git status --short | wc -l`, `git log -1 --oneline`
2. Run `bd list --status=in_progress` for active beads
3. Synthesize current session state into the schema
4. Push `current` into `previousSessions` (trim to 2 entries max)
5. Write JSON to `.handoff.json` in project root
6. Run `bd sync`

### Read Flow (SessionStart Hook)

1. Resolve project root from cwd (look for `.git/` or `.handoff.json`)
2. If `.handoff.json` exists, read and parse it
3. Format as human-readable context block
4. Output to stderr (injected into Claude's context)
5. If file doesn't exist, output nothing (first session or fresh clone)

### Haiku Fallback Flow (Stop Hook)

If Stop hook fires and `.handoff.json` is missing or stale (updated > 4 hours ago):

1. Read transcript JSONL (last ~20 exchanges)
2. Call Haiku with structured extraction prompt
3. Write result to `.handoff.json`
4. Run `bd sync`

This is a safety net, not the primary path. Quality will be lower than Claude-authored handoffs.

## Schema (v1)

```json
{
  "$schema": "handoff-v1",
  "current": {
    "project": "syllago",
    "updated": "2026-03-17T15:30:00-08:00",
    "sessionId": "2c80ed9e-2d91-4885-99bc-48ac62fe0994",
    "summary": "Adding wizard step machine enforcement",

    "activeWork": {
      "description": "Implementing wizard enforcement system from design doc",
      "feature": "wizard-enforcement",
      "stage": "execute",
      "planRef": "docs/plans/2026-03-17-wizard-enforcement-implementation.md",
      "designRef": "docs/prompts/wizard-enforcement-design.md",
      "progress": "Tasks 1-4 complete. Task 5 next (forward-path tests).",
      "blockers": [],
      "beads": {
        "inProgress": ["beads-abc123"],
        "completedThisSession": ["beads-def456", "beads-ghi789"],
        "readyNext": ["beads-jkl012"]
      }
    },

    "git": {
      "branch": "main",
      "uncommittedFiles": 12,
      "lastCommit": "94db53f feat(tui): delete old modal",
      "modifiedFiles": [
        "cli/internal/tui/import.go",
        "cli/internal/tui/import_test.go"
      ]
    },

    "recentDecisions": [
      "validateStep() checks entry-prerequisites only",
      "PostToolUse for feedback, pre-commit for hard gate"
    ],

    "nextSteps": [
      "Create wizard_invariant_test.go with forward-path tests",
      "Create PostToolUse hook script"
    ],

    "context": {
      "notes": "Import wizard has 15 steps. 6 importable types."
    }
  },

  "previousSessions": [
    {
      "summary": "Fixed import wizard bugs from audit",
      "updated": "2026-03-16T14:00:00-08:00",
      "completedWork": ["3 bug fixes in import.go", "8 new tests"],
      "nextSteps": ["Design wizard enforcement system"]
    },
    {
      "summary": "TUI modal redesign",
      "updated": "2026-03-12T23:55:00-08:00",
      "completedWork": ["Delete old modal", "Wizard mouse support"],
      "nextSteps": ["Audit add flows"]
    }
  ]
}
```

### Schema Field Reference

| Field | Type | Purpose |
|-------|------|---------|
| `$schema` | string | Version identifier for forward compatibility |
| `current.project` | string | Project name (from cwd) |
| `current.updated` | ISO 8601 | When handoff was last written |
| `current.sessionId` | string | Claude Code session ID |
| `current.summary` | string | One-line description of current work |
| `current.activeWork.description` | string | What's being worked on |
| `current.activeWork.feature` | string | Feature name (matches .develop/ and beads) |
| `current.activeWork.stage` | string | brainstorm/plan/validate/execute/complete |
| `current.activeWork.planRef` | string? | Path to implementation plan |
| `current.activeWork.designRef` | string? | Path to design document |
| `current.activeWork.progress` | string | Free-form progress description |
| `current.activeWork.blockers` | string[] | Current blockers (empty if none) |
| `current.activeWork.beads` | object? | Bead state (null if not using beads) |
| `current.activeWork.beads.inProgress` | string[] | Bead IDs actively being worked |
| `current.activeWork.beads.completedThisSession` | string[] | Beads closed this session |
| `current.activeWork.beads.readyNext` | string[] | Next beads to pick up |
| `current.git.branch` | string | Current git branch |
| `current.git.uncommittedFiles` | number | Count of uncommitted changes |
| `current.git.lastCommit` | string | Short hash + message of last commit |
| `current.git.modifiedFiles` | string[] | Key files modified (not exhaustive) |
| `current.recentDecisions` | string[] | Key decisions made this session |
| `current.nextSteps` | string[] | Ordered list of what to do next |
| `current.context.notes` | string | Free-form context that doesn't fit elsewhere |
| `previousSessions` | array | Last 2 sessions (trimmed schema) |
| `previousSessions[].summary` | string | What that session worked on |
| `previousSessions[].updated` | ISO 8601 | When that session ended |
| `previousSessions[].completedWork` | string[] | What got done |
| `previousSessions[].nextSteps` | string[] | What was planned next |

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Scope | Project-scoped only | Global covered by MEMORY.md, feature covered by .develop/ |
| Format | JSON | Machine-readable, consistent with existing .develop/ state |
| Location | `.handoff.json` in project root | Simple path resolution, gitignored |
| Primary author | Claude (in-conversation) | Highest quality — full context, can capture reasoning |
| Fallback author | Haiku (from transcript) | Safety net for crashes/abrupt endings |
| History depth | 3 sessions (current + 2 previous) | Gives trajectory without bloat |
| Triggers | PreCompact + close protocol + /handoff + Stop fallback | Four layers of reliability |
| Beads integration | Include in activeWork + bd sync on write | Ensures bead state is persisted alongside handoff |

## Components to Build

| Component | Type | New/Modified |
|-----------|------|-------------|
| `/handoff` skill | Skill (SKILL.md + prompt) | New |
| `session-handoff-read.ts` | SessionStart hook | New |
| `develop-precompact.ts` | PreCompact hook | Modified (add handoff reminder) |
| `stop-hook.ts` | Stop hook | Modified (add Haiku fallback + bd sync) |
| Session close protocol | Beads context / CLAUDE.md | Modified (add handoff step) |
| `.gitignore` | Config | Modified (add .handoff.json) |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `.handoff.json` doesn't exist | SessionStart outputs nothing. Normal first-session behavior. |
| `.handoff.json` is malformed | SessionStart logs warning, skips injection. Don't crash. |
| Claude forgets to write handoff | Stop hook Haiku fallback creates one from transcript. |
| Session crashes mid-write | Partial JSON is malformed → falls back to Haiku extraction. |
| Haiku API call fails | Log error, skip. Next session starts without handoff. Acceptable degradation. |
| Project root can't be determined | Skip handoff entirely. Log warning. |

## Success Criteria

- [ ] New session starts with "Session Handoff" context block showing previous work
- [ ] `/handoff` command writes valid JSON to `.handoff.json`
- [ ] PreCompact hook reminds Claude to update handoff
- [ ] Stop hook creates fallback handoff via Haiku when none exists
- [ ] `bd sync` runs on every handoff write
- [ ] Previous sessions array maintains exactly 2 entries (rolling)
- [ ] System degrades gracefully (no crashes, no blocking) on any failure

---

## Next Steps

Ready for implementation planning with `Plan` skill.
