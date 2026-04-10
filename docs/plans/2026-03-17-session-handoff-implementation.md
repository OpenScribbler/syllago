# Session Handoff System — Implementation Plan

**Design doc:** `docs/plans/2026-03-17-session-handoff-design.md`
**Date:** 2026-03-17
**Estimate:** ~45 minutes total across 10 tasks

---

## Overview

This plan implements a four-layer session handoff system: a manual `/handoff` skill, a SessionStart hook that reads the handoff, a PreCompact reminder, and a Stop hook Haiku fallback. Tasks are ordered by dependency — hooks depend on the schema being correct; the Stop hook modification depends on understanding the Haiku extraction pattern.

---

## Task 1 — Create `/handoff` skill: SKILL.md

**Time:** 3 min
**Dependencies:** None
**File:** `~/.claude/skills/handoff/SKILL.md`

Create the skill directory and SKILL.md. This is the prompt Claude reads when you invoke `/handoff`. It instructs Claude to gather state, synthesize it into the schema, and write `.handoff.json`.

```bash
mkdir -p ~/.claude/skills/handoff
```

**File content** (`~/.claude/skills/handoff/SKILL.md`):

```markdown
---
name: handoff
description: Write session handoff state to .handoff.json. USE WHEN ending a session OR before a long break OR when asked to save progress. Captures git state, beads state, and active work into a structured handoff file for the next session.
---

# Handoff

Capture current session state into `.handoff.json` so the next session starts informed.

## When to Use

- At session end (part of close protocol)
- After PreCompact reminder
- Manually when you want to checkpoint progress
- After any significant milestone

## Execution

Follow the workflow in `workflows/write-handoff.md`.
```

**Verification:** `ls ~/.claude/skills/handoff/SKILL.md` returns the file.

---

## Task 2 — Create `/handoff` skill: write-handoff workflow

**Time:** 5 min
**Dependencies:** Task 1
**File:** `~/.claude/skills/handoff/workflows/write-handoff.md`

```bash
mkdir -p ~/.claude/skills/handoff/workflows
```

**File content** (`~/.claude/skills/handoff/workflows/write-handoff.md`):

```markdown
# Write Handoff Workflow

> **Trigger:** `/handoff`, PreCompact reminder, session close protocol

## Purpose

Capture current session state into `.handoff.json` at the project root. This file is read by the SessionStart hook at the start of the next session.

## Steps

### 1. Gather git state

Run these commands and capture their output:

```bash
git branch --show-current
git status --short | wc -l
git log -1 --oneline
git diff --name-only HEAD
```

### 2. Gather beads state

```bash
bd list --status=in_progress
bd list --status=open --ready
```

### 3. Read existing .handoff.json (if present)

Read `.handoff.json` from the project root. If it exists, extract `current` into `previousSessions` (capped at 2 entries, trimmed schema). If it doesn't exist, start with an empty `previousSessions` array.

### 4. Synthesize and write

Construct the JSON following the schema below. Use real data gathered above — do not invent or estimate values.

**Schema:**

```json
{
  "$schema": "handoff-v1",
  "current": {
    "project": "<basename of cwd>",
    "updated": "<ISO 8601 with timezone offset>",
    "sessionId": "<CLAUDE_SESSION_ID env var if available, else null>",
    "summary": "<one-line description of what this session worked on>",

    "activeWork": {
      "description": "<what's being actively worked on>",
      "feature": "<feature name matching .develop/ state if applicable, else null>",
      "stage": "<brainstorm|plan|validate|execute|complete>",
      "planRef": "<relative path to implementation plan, or null>",
      "designRef": "<relative path to design doc, or null>",
      "progress": "<free-form: what's done, what's next, where we are>",
      "blockers": [],
      "beads": {
        "inProgress": ["<bead IDs from bd list --status=in_progress>"],
        "completedThisSession": ["<bead IDs closed this session, if known>"],
        "readyNext": ["<bead IDs from bd list --status=open --ready>"]
      }
    },

    "git": {
      "branch": "<branch name>",
      "uncommittedFiles": <count>,
      "lastCommit": "<short hash + message>",
      "modifiedFiles": ["<key modified files, not exhaustive — pick the most relevant>"]
    },

    "recentDecisions": [
      "<key architectural or design decision made this session>",
      "<another decision if applicable>"
    ],

    "nextSteps": [
      "<ordered — the first thing to do next session>",
      "<second thing>",
      "<third thing if applicable>"
    ],

    "context": {
      "notes": "<anything important that doesn't fit above — caveats, gotchas, context needed to resume>"
    }
  },

  "previousSessions": [
    {
      "summary": "<what that session worked on>",
      "updated": "<ISO 8601>",
      "completedWork": ["<what got done>"],
      "nextSteps": ["<what was planned next>"]
    }
  ]
}
```

**previousSessions rules:**
- Take the `current` block from the existing `.handoff.json` and append it as a trimmed entry (keep only: `summary`, `updated`, `completedWork` derived from progress description, `nextSteps`).
- Keep a maximum of 2 entries. Drop the oldest if already at 2.
- If no existing `.handoff.json`, `previousSessions` is `[]`.

### 5. Write the file

Write the JSON to `.handoff.json` in the project root (the directory containing `.git/`). Use 2-space indentation.

### 6. Run bd sync

```bash
bd sync
```

This ensures bead state is persisted to git alongside the handoff.

### 7. Confirm

Output a brief confirmation: "Handoff written to `.handoff.json`. Beads synced."
```

**Verification:** `ls ~/.claude/skills/handoff/workflows/write-handoff.md` returns the file.

---

## Task 3 — Create `session-handoff-read.ts` hook

**Time:** 8 min
**Dependencies:** None (reads a file that may not yet exist — graceful)
**File:** `~/.config/pai/hooks/session-handoff-read.ts`

This is the SessionStart hook that reads `.handoff.json` from the project root and injects it as a context block into Claude's session.

**Key design decisions:**
- Resolves project root by walking up from `cwd` looking for `.git/` or `.handoff.json`. Falls back to `cwd` itself.
- Outputs via `console.log` (stdout) — same pattern as `load-core-context.ts`. SessionStart hooks that write to stdout inject into Claude's context.
- Skips if file doesn't exist (first session), is malformed (logs warning, continues), or project root can't be determined.
- Skips for subagent sessions (same guard as `load-core-context.ts`).

**File content** (`~/.config/pai/hooks/session-handoff-read.ts`):

```typescript
#!/usr/bin/env bun
// ~/.config/pai/hooks/session-handoff-read.ts
// SessionStart hook: Read .handoff.json and inject as session context

import { existsSync, readFileSync } from 'fs';
import { join, dirname } from 'path';

interface SessionStartPayload {
  session_id: string;
  cwd?: string;
  [key: string]: any;
}

interface PreviousSession {
  summary: string;
  updated: string;
  completedWork: string[];
  nextSteps: string[];
}

interface HandoffV1 {
  $schema: string;
  current: {
    project: string;
    updated: string;
    sessionId: string | null;
    summary: string;
    activeWork: {
      description: string;
      feature: string | null;
      stage: string;
      planRef: string | null;
      designRef: string | null;
      progress: string;
      blockers: string[];
      beads?: {
        inProgress: string[];
        completedThisSession: string[];
        readyNext: string[];
      } | null;
    };
    git: {
      branch: string;
      uncommittedFiles: number;
      lastCommit: string;
      modifiedFiles: string[];
    };
    recentDecisions: string[];
    nextSteps: string[];
    context: {
      notes: string;
    };
  };
  previousSessions: PreviousSession[];
}

function isSubagentSession(): boolean {
  return process.env.CLAUDE_CODE_AGENT !== undefined ||
         process.env.SUBAGENT === 'true';
}

/**
 * Walk up from startDir looking for a .git directory or .handoff.json file.
 * Returns the directory where one is found, or null if we reach the filesystem root.
 */
function findProjectRoot(startDir: string): string | null {
  let dir = startDir;
  const root = '/';

  while (dir !== root) {
    if (existsSync(join(dir, '.git')) || existsSync(join(dir, '.handoff.json'))) {
      return dir;
    }
    const parent = dirname(dir);
    if (parent === dir) break; // reached root
    dir = parent;
  }

  return null;
}

function formatHandoff(h: HandoffV1): string {
  const c = h.current;
  const aw = c.activeWork;
  const lines: string[] = [];

  lines.push(`=== SESSION HANDOFF (from previous session) ===`);
  lines.push(`Project: ${c.project}`);
  lines.push(`Last updated: ${c.updated}`);
  lines.push(`Summary: ${c.summary}`);
  lines.push(``);

  lines.push(`ACTIVE WORK`);
  lines.push(`  ${aw.description}`);
  if (aw.feature) lines.push(`  Feature: ${aw.feature} (stage: ${aw.stage})`);
  if (aw.planRef) lines.push(`  Plan: ${aw.planRef}`);
  if (aw.designRef) lines.push(`  Design: ${aw.designRef}`);
  lines.push(`  Progress: ${aw.progress}`);
  if (aw.blockers.length > 0) {
    lines.push(`  Blockers:`);
    aw.blockers.forEach(b => lines.push(`    - ${b}`));
  }

  if (aw.beads) {
    lines.push(``);
    lines.push(`BEADS`);
    if (aw.beads.inProgress.length > 0) {
      lines.push(`  In progress: ${aw.beads.inProgress.join(', ')}`);
    }
    if (aw.beads.completedThisSession.length > 0) {
      lines.push(`  Completed last session: ${aw.beads.completedThisSession.join(', ')}`);
    }
    if (aw.beads.readyNext.length > 0) {
      lines.push(`  Ready next: ${aw.beads.readyNext.join(', ')}`);
    }
  }

  lines.push(``);
  lines.push(`GIT STATE`);
  lines.push(`  Branch: ${c.git.branch}`);
  lines.push(`  Uncommitted files: ${c.git.uncommittedFiles}`);
  lines.push(`  Last commit: ${c.git.lastCommit}`);
  if (c.git.modifiedFiles.length > 0) {
    lines.push(`  Key modified files: ${c.git.modifiedFiles.join(', ')}`);
  }

  if (c.recentDecisions.length > 0) {
    lines.push(``);
    lines.push(`RECENT DECISIONS`);
    c.recentDecisions.forEach(d => lines.push(`  - ${d}`));
  }

  if (c.nextSteps.length > 0) {
    lines.push(``);
    lines.push(`NEXT STEPS`);
    c.nextSteps.forEach((s, i) => lines.push(`  ${i + 1}. ${s}`));
  }

  if (c.context?.notes) {
    lines.push(``);
    lines.push(`NOTES`);
    lines.push(`  ${c.context.notes}`);
  }

  if (h.previousSessions.length > 0) {
    lines.push(``);
    lines.push(`PREVIOUS SESSIONS`);
    h.previousSessions.forEach(ps => {
      lines.push(`  [${ps.updated}] ${ps.summary}`);
      if (ps.nextSteps.length > 0) {
        lines.push(`    Planned next: ${ps.nextSteps.join(' / ')}`);
      }
    });
  }

  lines.push(`=== END SESSION HANDOFF ===`);

  return lines.join('\n');
}

async function main() {
  try {
    if (isSubagentSession()) {
      process.exit(0);
    }

    const stdinData = await Bun.stdin.text();
    if (!stdinData.trim()) {
      process.exit(0);
    }

    const payload: SessionStartPayload = JSON.parse(stdinData);
    const cwd = payload.cwd || process.cwd();

    const projectRoot = findProjectRoot(cwd);
    if (!projectRoot) {
      // No project root found — skip silently
      process.exit(0);
    }

    const handoffPath = join(projectRoot, '.handoff.json');
    if (!existsSync(handoffPath)) {
      // No handoff file — first session or fresh clone
      process.exit(0);
    }

    let handoff: HandoffV1;
    try {
      const raw = readFileSync(handoffPath, 'utf-8');
      handoff = JSON.parse(raw);
    } catch (err) {
      console.error(`[PAI] Warning: .handoff.json exists but could not be parsed: ${err}`);
      process.exit(0);
    }

    // Validate minimum required fields
    if (!handoff.current || !handoff.current.summary) {
      console.error('[PAI] Warning: .handoff.json missing required fields, skipping injection');
      process.exit(0);
    }

    const formatted = formatHandoff(handoff);

    console.log(`<system-reminder>
${formatted}
</system-reminder>`);

  } catch (error) {
    // Never crash — session start must be silent on failure
    console.error('[PAI] session-handoff-read error:', error);
  }

  process.exit(0);
}

main();
```

**Verification:** `bun run ~/.config/pai/hooks/session-handoff-read.ts <<< '{"session_id":"test","cwd":"/home/hhewett/.local/src/syllago"}'` — if `.handoff.json` doesn't exist yet, exits cleanly with no output. If it does exist, outputs a formatted context block.

---

## Task 4 — Register `session-handoff-read.ts` in settings.json

**Time:** 3 min
**Dependencies:** Task 3
**File:** `~/.claude/settings.json`

Add the new hook to the `SessionStart` array. It must run after `load-core-context.ts` (so core context loads first) and before `bd prime` (ordering doesn't matter for correctness, but convention is: system hooks before workflow hooks).

In `~/.claude/settings.json`, find the `SessionStart` section. The first hook block with `"matcher": "*"` currently has 4 hooks. Add the handoff reader as the third entry (after `load-core-context.ts`, before the learnings injector):

```json
{
  "type": "command",
  "command": "bun run $PAI_DIR/hooks/session-handoff-read.ts"
}
```

**Full updated SessionStart block after edit:**

```json
"SessionStart": [
  {
    "matcher": "*",
    "hooks": [
      {
        "type": "command",
        "command": "bun run $PAI_DIR/hooks/initialize-session.ts"
      },
      {
        "type": "command",
        "command": "bun run $PAI_DIR/hooks/load-core-context.ts"
      },
      {
        "type": "command",
        "command": "bun run $PAI_DIR/hooks/session-handoff-read.ts"
      },
      {
        "type": "command",
        "command": "$HOME/.config/learnings/inject-learning-context.sh"
      },
      {
        "type": "command",
        "command": "bun run $PAI_DIR/hooks/capture-all-events.ts --event-type SessionStart"
      }
    ]
  },
  {
    "matcher": "",
    "hooks": [
      {
        "type": "command",
        "command": "bd prime"
      }
    ]
  }
]
```

**Verification:** `cat ~/.claude/settings.json | python3 -c "import json,sys; d=json.load(sys.stdin); hooks=[h['command'] for block in d['hooks']['SessionStart'] for h in block['hooks']]; print('\n'.join(hooks))"` — should list `session-handoff-read.ts` in the output.

---

## Task 5 — Modify `develop-precompact.ts` to add handoff reminder

**Time:** 4 min
**Dependencies:** None (independent modification)
**File:** `~/.config/pai/hooks/develop-precompact.ts`

Add a handoff reminder block to the PreCompact output. This reminder should always appear, regardless of whether a develop workflow is active — because compaction can happen any time, and we want Claude to write the handoff before context is lost.

The reminder is appended after the existing develop workflow section (or output alone if no workflows are active).

**Current structure:** The script outputs a `<system-reminder>` block only when `workflows.length > 0`, and `process.exit(0)` early when no workflows are active.

**Change:** Remove the early exit, always output a `<system-reminder>`. If workflows are active, include them. Always include the handoff reminder at the end.

**IMPORTANT — file structure note:** `develop-precompact.ts` uses a **top-level `try/catch` block**, NOT an `async function main()`. The replacement code below is compatible with this structure (no async/await). After replacing the try block, ensure the `process.exit(0)` at the end of the file (after the catch) is preserved.

Replace the `try` block (lines 63–88) with:

```typescript
try {
  const workflows = listActiveWorkflows();

  const parts: string[] = [];

  if (workflows.length > 0) {
    const sections = workflows.map(formatWorkflow);
    const stateFiles = workflows.map(w => {
      const sanitized = w.feature.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-');
      return `.develop/${sanitized}.json`;
    });

    parts.push(`DEVELOP WORKFLOW ACTIVE:

${sections.join('\n\n')}

TO RESUME: Read ${stateFiles.join(', ')}, check nextAction field, execute it.
If nextAction is null, run /develop --resume to re-derive the next step.`);
  }

  parts.push(`HANDOFF REMINDER:
Before this compaction completes your context, write a session handoff:
  /handoff
This captures your current work state to .handoff.json so the next session starts informed.
If you cannot run /handoff right now, write .handoff.json manually using the schema in ~/.claude/skills/handoff/workflows/write-handoff.md.`);

  console.log(`<system-reminder>
${parts.join('\n\n')}
</system-reminder>`);

} catch (error) {
  // Never crash - PreCompact hooks must be silent on failure
  console.error('Develop PreCompact hook error:', error);
}
```

**Verification:** `bun run ~/.config/pai/hooks/develop-precompact.ts` with no active workflows should output a `<system-reminder>` containing "HANDOFF REMINDER". With an active workflow, it should output both sections.

---

## Task 6 — Modify `stop-hook.ts`: add Haiku fallback

**Time:** 10 min
**Dependencies:** Task 3 (same `.handoff.json` schema), understanding of transcript format
**File:** `~/.config/pai/hooks/stop-hook.ts`

Add a Haiku-based handoff fallback that fires when `.handoff.json` is missing or stale (older than 4 hours). This is a safety net for sessions that end abruptly without running `/handoff`.

**Key design decisions:**
- Use `ANTHROPIC_API_KEY` env var (same as every other Anthropic API caller).
- Call `claude-haiku-4-5` (current Haiku model). Use `max_tokens: 2000` — the JSON schema is compact.
- Read the last 20 assistant+user exchanges from the transcript (not the full thing — Haiku's context is small and we only need recent work).
- Write to project root resolved from `payload.cwd` (from the Stop payload, which includes cwd).
- Run `bd sync` after writing — regardless of whether Haiku or Claude authored the handoff.

**Stop payload type update** — add `cwd` field:

```typescript
interface StopPayload {
  stop_hook_active: boolean;
  transcript_path?: string;
  response?: string;
  session_id?: string;
  cwd?: string;
}
```

**New helper: `isHandoffFresh`**

```typescript
/**
 * Returns true if .handoff.json exists and was written within the last 4 hours.
 */
function isHandoffFresh(projectRoot: string): boolean {
  const handoffPath = join(projectRoot, '.handoff.json');
  if (!existsSync(handoffPath)) return false;

  try {
    const stat = Bun.file(handoffPath);
    // stat.lastModified is ms since epoch
    const ageMs = Date.now() - (stat.lastModified ?? 0);
    return ageMs < 4 * 60 * 60 * 1000; // 4 hours
  } catch {
    return false;
  }
}
```

**Import update required** — `stop-hook.ts` currently only imports `join` from `path`. Update line 6 to add `dirname`:

```typescript
// Change from:
import { join } from 'path';
// To:
import { join, dirname } from 'path';
```

**New helper: `findProjectRootFromCwd`**

```typescript
function findProjectRootFromCwd(startDir: string): string | null {
  let dir = startDir;
  const root = '/';

  while (dir !== root) {
    if (existsSync(join(dir, '.git'))) {
      return dir;
    }
    const parent = dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }

  return null;
}
```

**New helper: `extractRecentExchanges`**

```typescript
/**
 * Extract the last N user+assistant text exchanges from a transcript JSONL.
 * Returns a formatted string suitable for Haiku's extraction prompt.
 */
function extractRecentExchanges(transcriptPath: string, maxExchanges: number = 20): string {
  try {
    const content = readFileSync(transcriptPath, 'utf-8');
    const lines = content.trim().split('\n').filter(l => l.trim());

    const exchanges: Array<{ role: string; text: string }> = [];

    for (const line of lines) {
      try {
        const entry = JSON.parse(line);
        if (entry.type === 'user' && entry.message?.content) {
          const content = Array.isArray(entry.message.content)
            ? entry.message.content
            : [entry.message.content];
          // Skip pure tool_result messages
          const isToolResult = Array.isArray(entry.message.content) &&
            entry.message.content.every((b: any) => b.type === 'tool_result');
          if (!isToolResult) {
            const text = content
              .map((c: any) => typeof c === 'string' ? c : c?.text || '')
              .join('\n')
              .trim();
            if (text) exchanges.push({ role: 'user', text });
          }
        } else if (entry.type === 'assistant' && entry.message?.content) {
          const content = Array.isArray(entry.message.content)
            ? entry.message.content
            : [entry.message.content];
          const text = content
            .map((c: any) => typeof c === 'string' ? c : c?.text || '')
            .join('\n')
            .trim();
          if (text) exchanges.push({ role: 'assistant', text });
        }
      } catch {
        // skip malformed lines
      }
    }

    // Take last maxExchanges, format as conversation
    const recent = exchanges.slice(-maxExchanges);
    return recent
      .map(e => `[${e.role.toUpperCase()}]\n${e.text.slice(0, 1000)}`)
      .join('\n\n---\n\n');
  } catch {
    return '';
  }
}
```

**New function: `writeHandoffViaHaiku`**

```typescript
/**
 * Extract handoff state from transcript using Haiku and write to .handoff.json.
 * This is a fallback — quality is lower than Claude-authored handoffs.
 */
async function writeHandoffViaHaiku(
  transcriptPath: string,
  projectRoot: string,
  sessionId: string | undefined
): Promise<void> {
  const apiKey = process.env.ANTHROPIC_API_KEY;
  if (!apiKey) {
    console.error('[PAI] ANTHROPIC_API_KEY not set — skipping Haiku handoff fallback');
    return;
  }

  const conversation = extractRecentExchanges(transcriptPath, 20);
  if (!conversation) {
    console.error('[PAI] Could not extract conversation from transcript — skipping Haiku handoff');
    return;
  }

  const projectName = projectRoot.split('/').pop() || 'unknown';
  const now = new Date().toISOString();

  const prompt = `You are extracting session state from a conversation transcript to write a handoff file.

The conversation is from a Claude Code session working on project: ${projectName}

Conversation (most recent exchanges):
${conversation}

Extract the session state and output ONLY valid JSON matching this exact schema. No prose, no markdown, just the JSON object:

{
  "$schema": "handoff-v1",
  "current": {
    "project": "${projectName}",
    "updated": "${now}",
    "sessionId": ${sessionId ? `"${sessionId}"` : 'null'},
    "summary": "<one line: what was worked on this session>",
    "activeWork": {
      "description": "<what was being actively worked on>",
      "feature": "<feature name or null>",
      "stage": "<brainstorm|plan|validate|execute|complete>",
      "planRef": "<relative path to plan doc or null>",
      "designRef": "<relative path to design doc or null>",
      "progress": "<what got done and what's next>",
      "blockers": [],
      "beads": null
    },
    "git": {
      "branch": "unknown",
      "uncommittedFiles": 0,
      "lastCommit": "unknown",
      "modifiedFiles": []
    },
    "recentDecisions": ["<key decisions made>"],
    "nextSteps": ["<ordered: what to do next session>"],
    "context": {
      "notes": "<anything important for resuming work>"
    }
  },
  "previousSessions": []
}

Fill in all fields from the conversation. Output JSON only.`;

  try {
    const response = await fetch('https://api.anthropic.com/v1/messages', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-api-key': apiKey,
        'anthropic-version': '2023-06-01',
      },
      body: JSON.stringify({
        model: 'claude-haiku-4-5',
        max_tokens: 2000,
        messages: [{ role: 'user', content: prompt }],
      }),
    });

    if (!response.ok) {
      console.error(`[PAI] Haiku API error: ${response.status} ${response.statusText}`);
      return;
    }

    const data = await response.json() as any;
    const text = data?.content?.[0]?.text;
    if (!text) {
      console.error('[PAI] Haiku returned empty content');
      return;
    }

    // Validate it's parseable JSON before writing
    let parsed: any;
    try {
      parsed = JSON.parse(text.trim());
    } catch {
      // Try to extract JSON from the response in case of accidental wrapping
      const jsonMatch = text.match(/\{[\s\S]+\}/);
      if (!jsonMatch) {
        console.error('[PAI] Haiku response was not valid JSON');
        return;
      }
      parsed = JSON.parse(jsonMatch[0]);
    }

    const handoffPath = join(projectRoot, '.handoff.json');
    writeFileSync(handoffPath, JSON.stringify(parsed, null, 2));
    console.log(`[PAI] Haiku fallback: wrote .handoff.json to ${handoffPath}`);

  } catch (err) {
    console.error('[PAI] Haiku handoff extraction failed:', err);
    // Silent failure — acceptable degradation per design doc
  }
}
```

**Addition to `main()`**

**IMPORTANT — early exit guard:** `main()` currently exits early at line ~490 if no `response` is extractable from the transcript. The handoff code below must be inserted BEFORE that early exit, or restructured so it runs independently of response extraction. The clearest fix: add the handoff block immediately after parsing the payload (before `if (!response)`), so sessions with no text response still get a handoff file.

After the existing session indexing block (after the `backupDb` call), add:

```typescript
// --- Handoff fallback ---
// If .handoff.json is missing or stale, extract from transcript via Haiku
if (payload.transcript_path && payload.cwd) {
  try {
    const projectRoot = findProjectRootFromCwd(payload.cwd);
    if (projectRoot && !isHandoffFresh(projectRoot)) {
      await writeHandoffViaHaiku(payload.transcript_path, projectRoot, payload.session_id);
    }
  } catch (err) {
    console.error('[PAI] Handoff fallback error:', err);
    // Never fail the stop hook
  }
}

// --- bd sync (final safety net) ---
try {
  const bdSync = Bun.spawn(['bd', 'sync'], {
    stdout: 'pipe',
    stderr: 'pipe',
    cwd: payload.cwd,
  });
  await bdSync.exited;
} catch {
  // Silent failure — bd may not be available in all projects
}
```

**Verification:**
1. `bun run ~/.config/pai/hooks/stop-hook.ts <<< '{"stop_hook_active":false}'` — exits cleanly (no cwd, no transcript).
2. After a test session, verify `.handoff.json` is created when it didn't exist before.

---

## Task 7 — Update session close protocol in beads context

**Time:** 5 min
**Dependencies:** None
**File:** `~/.beads/PRIME.md` (must be created first — see below)

**IMPORTANT:** The SESSION CLOSE PROTOCOL text is **compiled into the `bd` binary**, not stored in any editable markdown file. The find/grep discovery commands in the original plan will return no results. The correct approach:

**Step 1: Extract the default content to PRIME.md:**

```bash
bd prime --export > ~/.beads/PRIME.md
```

This creates a custom override file that `bd prime` reads instead of the built-in content.

**Step 2: Verify it works:**

```bash
bd prime | grep -A 12 "SESSION CLOSE"
```

**Step 3:** Now edit `~/.beads/PRIME.md` to add step 3.5 (between `bd sync` and `git commit`):

Once located, add step 3.5 (between `bd sync` and `git commit`):

**Updated close protocol checklist:**

```markdown
[ ] 1. git status              (check what changed)
[ ] 2. git add <files>         (stage code changes)
[ ] 3. bd sync                 (commit beads changes)
[ ] 3.5. /handoff              (write .handoff.json for next session)
[ ] 4. git commit -m "..."     (commit code)
[ ] 5. bd sync                 (commit any new beads changes)
[ ] 6. git push                (push to remote)
```

**Note:** `/handoff` runs `bd sync` internally (step 6 of the workflow). This means after `/handoff`, the second `bd sync` in step 5 may be a no-op — that's fine. Belt-and-suspenders.

**Verification:** `bd prime 2>&1 | grep -A 12 "SESSION CLOSE"` — should show the updated checklist with step 3.5.

---

## Task 8 — Update `.gitignore` files

**Time:** 2 min
**Dependencies:** None
**Files:**
- `/home/hhewett/.local/src/syllago/.gitignore` (project-specific, checked in)
- `~/.config/git/ignore` or `~/.gitignore_global` (global gitignore, affects all projects)

**For syllago `.gitignore`** — add to the "Development workflow state" section:

```
# Session handoff (ephemeral, not for sharing)
.handoff.json
```

**For global gitignore** — add `.handoff.json` so it's ignored in all projects without requiring per-project configuration. Find the global ignore file:

```bash
git config --global core.excludesfile
# Typical result: ~/.config/git/ignore  or  ~/.gitignore_global
```

Add to that file:

```
# Session handoff (project-scoped, ephemeral)
.handoff.json
```

**Why both?** The syllago `.gitignore` handles the immediate case. The global gitignore ensures every other project also ignores `.handoff.json` from day one, without needing to remember to add it.

**Verification:** From the syllago repo: `echo '{}' > .handoff.json && git status` — `.handoff.json` should not appear in untracked files. Clean up: `rm .handoff.json`.

---

## Task 9 — Smoke test: full write/read cycle

**Time:** 5 min
**Dependencies:** Tasks 1, 2, 3, 4 complete

Test the full cycle manually without starting a new Claude session.

**Step 1: Write a test handoff**

From the syllago project root:

```bash
cat > /home/hhewett/.local/src/syllago/.handoff.json << 'EOF'
{
  "$schema": "handoff-v1",
  "current": {
    "project": "syllago",
    "updated": "2026-03-17T15:30:00-08:00",
    "sessionId": "test-session-001",
    "summary": "Testing session handoff system",
    "activeWork": {
      "description": "Implementing session handoff system from design doc",
      "feature": "session-handoff",
      "stage": "execute",
      "planRef": "docs/plans/2026-03-17-session-handoff-implementation.md",
      "designRef": "docs/plans/2026-03-17-session-handoff-design.md",
      "progress": "Tasks 1-8 complete. Running smoke tests.",
      "blockers": [],
      "beads": {
        "inProgress": [],
        "completedThisSession": [],
        "readyNext": []
      }
    },
    "git": {
      "branch": "main",
      "uncommittedFiles": 5,
      "lastCommit": "94db53f feat(tui): delete old modal",
      "modifiedFiles": [
        "~/.config/pai/hooks/session-handoff-read.ts",
        "~/.config/pai/hooks/stop-hook.ts"
      ]
    },
    "recentDecisions": [
      "Haiku fallback threshold is 4 hours",
      "Hook outputs to stdout (not stderr) for context injection"
    ],
    "nextSteps": [
      "Verify full cycle with a real session",
      "Confirm Stop hook Haiku fallback fires correctly"
    ],
    "context": {
      "notes": "This is a test handoff for smoke testing."
    }
  },
  "previousSessions": []
}
EOF
```

**Step 2: Run the SessionStart hook manually**

```bash
echo '{"session_id":"test","cwd":"/home/hhewett/.local/src/syllago"}' | \
  bun run /home/hhewett/.config/pai/hooks/session-handoff-read.ts
```

Expected output: A formatted `<system-reminder>` block containing "SESSION HANDOFF", project name, summary, next steps, etc.

**Step 3: Test with non-project directory (no .git)**

```bash
echo '{"session_id":"test","cwd":"/tmp"}' | \
  bun run /home/hhewett/.config/pai/hooks/session-handoff-read.ts
```

Expected output: Nothing (no project root found).

**Step 4: Test with malformed JSON**

```bash
echo 'not json' > /tmp/test-handoff.json
# Temporarily override by testing the parser branch directly:
echo '{"session_id":"test","cwd":"/tmp"}' | \
  bun run /home/hhewett/.config/pai/hooks/session-handoff-read.ts
```

Expected output: Nothing (no `.handoff.json` in `/tmp` after step 3 cleanup).

**Step 5: Clean up test file**

```bash
rm /home/hhewett/.local/src/syllago/.handoff.json
```

**Verification:** All steps produce expected output with no TypeScript errors or uncaught exceptions.

---

## Task 10 — Smoke test: PreCompact hook output

**Time:** 2 min
**Dependencies:** Task 5 complete

```bash
bun run /home/hhewett/.config/pai/hooks/develop-precompact.ts
```

**Expected output (no active workflows):**

```
<system-reminder>
HANDOFF REMINDER:
Before this compaction completes your context, write a session handoff:
  /handoff
...
</system-reminder>
```

**Expected output (with active workflow):**

```
<system-reminder>
DEVELOP WORKFLOW ACTIVE:

  Feature: some-feature
  Stage: ...
  ...

HANDOFF REMINDER:
...
</system-reminder>
```

**Verification:** Output contains "HANDOFF REMINDER" in both cases. No TypeScript errors.

---

## Dependency Graph

```
Task 1 (SKILL.md) ──────────────────────────────────────────┐
Task 2 (workflow) ─────────────── depends on Task 1 ────────┤
Task 3 (read hook) ─────────────────────────────────────────┤
Task 4 (register hook) ────────── depends on Task 3 ────────┤
Task 5 (precompact mod) ────────────────────────────────────┤──► Task 9 (smoke test cycle)
Task 6 (stop-hook mod) ─────────────────────────────────────┤       depends on 1,2,3,4
Task 7 (close protocol) ────────────────────────────────────┤
Task 8 (.gitignore) ────────────────────────────────────────┘
                                                              └──► Task 10 (smoke test precompact)
                                                                       depends on 5
```

Tasks 1, 3, 5, 6, 7, 8 have no dependencies on each other and can be worked in any order. Task 2 depends on Task 1. Task 4 depends on Task 3. Tasks 9 and 10 are verification and should run last.

---

## Files Created/Modified

| File | Action |
|------|--------|
| `~/.claude/skills/handoff/SKILL.md` | Create |
| `~/.claude/skills/handoff/workflows/write-handoff.md` | Create |
| `~/.config/pai/hooks/session-handoff-read.ts` | Create |
| `~/.claude/settings.json` | Modify (add hook to SessionStart) |
| `~/.config/pai/hooks/develop-precompact.ts` | Modify (always output, add handoff reminder) |
| `~/.config/pai/hooks/stop-hook.ts` | Modify (Haiku fallback + bd sync) |
| `<beads context source file>` | Modify (add /handoff step to close protocol) |
| `/home/hhewett/.local/src/syllago/.gitignore` | Modify (add .handoff.json) |
| `<global gitignore>` | Modify (add .handoff.json) |

---

## Success Criteria

Matches the design doc:

- [ ] New session in syllago repo outputs "Session Handoff" context block
- [ ] `/handoff` writes valid JSON to `.handoff.json` with correct schema
- [ ] PreCompact hook always outputs handoff reminder (with or without develop workflow)
- [ ] Stop hook writes fallback `.handoff.json` via Haiku when none exists
- [ ] `bd sync` runs on every handoff write (both Claude-authored and Haiku fallback)
- [ ] `previousSessions` array maintains exactly 2 entries (rolling)
- [ ] All failure modes degrade gracefully — no crashes, no blocking
- [ ] `.handoff.json` does not appear in `git status` (gitignored)
