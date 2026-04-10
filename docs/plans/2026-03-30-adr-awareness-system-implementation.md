# ADR Awareness System — Implementation Plan

**Design doc:** `docs/plans/2026-03-30-adr-awareness-system-design.md`
**Date:** 2026-03-30
**Executor:** This plan is self-contained. Read it top to bottom. No external context is required.

---

## Overview

Build a three-component ADR awareness system:
1. Global ADR Skill (`~/.config/pai/skills/adr/`) — SKILL.md + 3 workflows + template
2. Global Awareness Hook (`~/.config/pai/hooks/adr-awareness.sh`) — PostToolUse on Edit|Write
3. Global Commit Gate Hook (`~/.config/pai/hooks/adr-commit-gate.sh`) — PreToolUse on Bash

Plus syllago project setup:
4. Add YAML frontmatter to `docs/adr/0001-hook-degradation-enforcement.md`
5. Create `docs/adr/INDEX.md`
6. Add section to `CLAUDE.md`
7. Register hooks in `~/.claude/settings.json`
8. Create symlink `~/.claude/skills/adr -> ~/.config/pai/skills/adr/`

---

## Critical Context for the Executor

### How hooks receive input

Claude Code passes a JSON object via stdin to every hook. The shape differs by hook type:

**PreToolUse and PostToolUse:**
```json
{
  "tool_name": "Edit",
  "tool_input": {
    "file_path": "/absolute/path/to/file.go",
    "old_string": "...",
    "new_string": "..."
  }
}
```

For Write: `tool_input.file_path` and `tool_input.content`.
For Bash: `tool_input.command` contains the full command string.

**Important:** The existing `tui-convention-check.sh` hook uses `data.get('input', {})` to access tool input. But the actual Claude Code hook JSON schema uses `tool_input`, not `input`. The release-guard.sh hook uses `jq -r '.tool_input.command // empty'`. The awareness and commit gate hooks must use `tool_input` (not `input`) to be consistent with the actual schema. Use the release-guard.sh pattern as the reference.

However, looking at tui-convention-check.sh more carefully — it uses `data.get('input', {})` with python3. Both patterns exist in the codebase. The safer approach: try `tool_input` first, fall back to `input`. The python3 snippet should do:
```python
inp = data.get('tool_input', data.get('input', {}))
```
This handles both schemas without fragility.

### Exit codes

- Exit 0: allow (with optional stdout message shown to agent)
- Exit 1: error/violation (for PreToolUse, shows message but does NOT block)
- Exit 2: block (PreToolUse only — Claude Code refuses to execute the tool)

The commit gate uses exit 2 for strict enforcement.

### Pattern matching

The INDEX.md scope column uses glob patterns like `` `cli/internal/converter/*` ``. The hooks do **bash glob-style** matching, not regex. Use bash's `[[ "$file" == $pattern ]]` which supports `*` but NOT `**`. For patterns ending in `/*`, match files one level deep. This is sufficient for the current ADR-0001 scope.

### Settings.json hook registration

The PostToolUse for Edit and Write need separate matcher entries OR a combined `"Edit|Write"` matcher. Check what Claude Code supports. Looking at the existing settings.json, matchers are single tool names (e.g., `"Write"`, `"Edit"`, `"Bash"`). Create separate entries for Edit and Write, each pointing to the same `adr-awareness.sh` script.

---

## Task Ordering

```
Group A: Syllago Project Content
  Task 1 — Add frontmatter to ADR-0001
  Task 2 — Create docs/adr/INDEX.md
  Task 3 — Add section to CLAUDE.md

Group B: Hook Scripts
  Task 4 — Create adr-awareness.sh
  Task 5 — Create adr-commit-gate.sh

Group C: ADR Skill
  Task 6 — Create ~/.config/pai/skills/adr/ directory structure
  Task 7 — Write SKILL.md
  Task 8 — Write workflows/create.md
  Task 9 — Write workflows/consult.md
  Task 10 — Write workflows/review.md
  Task 11 — Write references/template.md

Group D: Global Registration
  Task 12 — Register hooks in ~/.claude/settings.json
  Task 13 — Create skill symlink
```

---

## Group A: Syllago Project Content

### Task 1 — Add YAML frontmatter to ADR-0001

**File:** `/home/hhewett/.local/src/syllago/docs/adr/0001-hook-degradation-enforcement.md`
**Action:** Edit — prepend YAML frontmatter block before the existing `# ADR 0001:` heading.

**Depends on:** Nothing — first task.

**Complete file content after edit:**

```markdown
---
id: "0001"
title: Hook Degradation Enforcement
status: accepted
date: 2026-03-28
enforcement: strict
files: ["cli/internal/converter/*"]
tags: [hooks, conversion, safety, degradation]
---

# ADR 0001: Hook Degradation Enforcement

## Status

Accepted

## Context
...
```

The existing content of the file starting from `# ADR 0001:` is unchanged. Only the frontmatter block is prepended.

**Exact edit operation:** Insert at the very beginning of the file (before line 1):

```
---
id: "0001"
title: Hook Degradation Enforcement
status: accepted
date: 2026-03-28
enforcement: strict
files: ["cli/internal/converter/*"]
tags: [hooks, conversion, safety, degradation]
---

```

**Success Criteria:**
- `head -10 /home/hhewett/.local/src/syllago/docs/adr/0001-hook-degradation-enforcement.md` → first line is `---`, line 9 is `---` — frontmatter block present
- `grep "enforcement: strict" /home/hhewett/.local/src/syllago/docs/adr/0001-hook-degradation-enforcement.md` → matches — enforcement field readable by hooks

---

### Task 2 — Create docs/adr/INDEX.md

**File:** `/home/hhewett/.local/src/syllago/docs/adr/INDEX.md`
**Action:** Write new file.

**Depends on:** Task 1 (frontmatter defines the data for the first row).

**Complete file content:**

```markdown
# ADR Index

Architectural decisions for this project. Before modifying files in a listed scope, read the full ADR.

| ADR | Title | Status | Enforcement | Scope | Summary |
|-----|-------|--------|-------------|-------|---------|
| [0001](0001-hook-degradation-enforcement.md) | Hook Degradation Enforcement | accepted | strict | `cli/internal/converter/*` | block/warn/exclude strategies must be enforced during conversion, not silently dropped |
```

**Row format rules (for the executor creating future ADR rows):**
- ADR column: `[NNNN](NNNN-slug.md)` — relative link, no path prefix
- Scope column: pattern in backticks — this is what hooks parse
- Superseded ADRs: ~~strikethrough~~ the title cell with `~~title~~`
- Status values: `proposed`, `accepted`, `superseded`
- Enforcement values: `strict`, `advisory`

**Success Criteria:**
- `cat /home/hhewett/.local/src/syllago/docs/adr/INDEX.md` → shows the table with one data row — file exists and is readable
- `grep 'cli/internal/converter/\*' /home/hhewett/.local/src/syllago/docs/adr/INDEX.md` → matches — scope pattern present in backtick format for hook parsing

---

### Task 3 — Add Architectural Decisions section to CLAUDE.md

**File:** `/home/hhewett/.local/src/syllago/CLAUDE.md`
**Action:** Edit — insert new section between `## Key Conventions` and `## Testing Requirements`.

**Depends on:** Task 2 (INDEX.md must exist before CLAUDE.md references it).

**Exact insertion:** Add the following block after the `## Key Conventions` section ends (after the line `- TUI component patterns: see \`cli/internal/tui/CLAUDE.md\``) and before the line `## Testing Requirements`:

```markdown

## Architectural Decisions

Active ADRs are indexed in `docs/adr/INDEX.md`. Before modifying files listed in an ADR's scope, read the full ADR to understand the rationale. Hooks will remind you when you touch scoped files — `strict` ADRs block commits, `advisory` ADRs warn.
```

**Success Criteria:**
- `grep -A3 "## Architectural Decisions" /home/hhewett/.local/src/syllago/CLAUDE.md` → shows the three-sentence paragraph — section present in file
- `grep -n "INDEX.md" /home/hhewett/.local/src/syllago/CLAUDE.md` → returns a line number — INDEX.md is referenced

---

## Group B: Hook Scripts

### Task 4 — Create adr-awareness.sh

**File:** `/home/hhewett/.config/pai/hooks/adr-awareness.sh`
**Action:** Write new file. Make executable after writing.

**Depends on:** Task 2 (INDEX.md format must be understood; no runtime dependency for testing).

**What this hook does:**
1. Read JSON from stdin
2. Extract `file_path` from `tool_input` (or `input` fallback)
3. Find git repo root with `git rev-parse --show-toplevel`
4. Check if `$REPO_ROOT/docs/adr/INDEX.md` exists — exit 0 silently if not
5. Normalize file_path to repo-relative path
6. Scan INDEX.md, skipping header rows. For each data row:
   - Check status column is `accepted`
   - Extract scope pattern from backtick-wrapped value in Scope column
   - Test if normalized file_path matches pattern using bash glob
7. On first match: print advisory message to stdout and exit 0
8. No match: exit 0 silently

**INDEX.md row parsing logic:**
The table has this structure:
```
| [0001](0001-hook-degradation-enforcement.md) | Hook Degradation Enforcement | accepted | strict | `cli/internal/converter/*` | summary |
```
Column positions (1-indexed pipe splits):
- Column 1: ADR link → extract `[NNNN](slug.md)` to get number and slug
- Column 3: Status
- Column 4: Enforcement
- Column 5: Scope — value is wrapped in backticks

To extract ADR number from column 1: match `\[([0-9]+)\]` → group 1.
To extract slug from column 1: match `\(([^)]+)\)` → group 1.
To extract scope: strip surrounding backticks from the trimmed column value.

**Bash glob matching:**
The scope pattern `cli/internal/converter/*` matches files like `cli/internal/converter/adapter.go`. Use:
```bash
[[ "$REL_FILE" == $PATTERN ]]
```
Note: do NOT quote `$PATTERN` when using it in `[[...==...]]` — unquoted right side enables glob matching.

**Complete file content:**

```bash
#!/usr/bin/env bash
# ADR Awareness Hook
# PostToolUse hook for Edit|Write operations.
# When a file is edited that falls under an ADR scope, prints an advisory.
# Auto-detects ADR-enabled projects via docs/adr/INDEX.md — exits 0 silently
# for projects without that file.

set -euo pipefail

# Read JSON input from stdin
INPUT=$(cat)

# Extract file_path from tool input (handles both tool_input and input schemas)
FILE_PATH=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    inp = data.get('tool_input', data.get('input', {}))
    print(inp.get('file_path', ''))
except Exception:
    print('')
" 2>/dev/null || echo "")

# If no file path extracted, nothing to check
if [[ -z "$FILE_PATH" ]]; then
    exit 0
fi

# Find git repo root — exit silently if not in a repo
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo "")
if [[ -z "$REPO_ROOT" ]]; then
    exit 0
fi

# Check if this project uses ADRs — exit silently if not
INDEX_FILE="$REPO_ROOT/docs/adr/INDEX.md"
if [[ ! -f "$INDEX_FILE" ]]; then
    exit 0
fi

# Normalize file path to repo-relative
# Strip repo root prefix if file_path is absolute
if [[ "$FILE_PATH" == "$REPO_ROOT/"* ]]; then
    REL_FILE="${FILE_PATH#$REPO_ROOT/}"
else
    REL_FILE="$FILE_PATH"
fi

# Scan INDEX.md for matching ADRs
# Lines that are table data rows start with "| [" (ADR link format)
while IFS='|' read -ra COLS; do
    # Raw column values (still have leading/trailing spaces)
    RAW_ADR="${COLS[1]:-}"
    RAW_STATUS="${COLS[3]:-}"
    RAW_SCOPE="${COLS[5]:-}"

    # Skip non-data rows (headers, separators, empty lines)
    if [[ ! "$RAW_ADR" =~ \[0-9 ]]; then
        continue
    fi

    # Trim whitespace from columns
    ADR_COL=$(echo "$RAW_ADR" | xargs)
    STATUS=$(echo "$RAW_STATUS" | xargs)
    SCOPE_RAW=$(echo "$RAW_SCOPE" | xargs)

    # Only process accepted ADRs
    if [[ "$STATUS" != "accepted" ]]; then
        continue
    fi

    # Extract ADR number from [NNNN](slug.md) format
    ADR_NUM=$(echo "$ADR_COL" | python3 -c "
import re, sys
m = re.search(r'\[([0-9]+)\]', sys.stdin.read())
print(m.group(1) if m else '')
" 2>/dev/null || echo "")

    # Extract slug (filename) from [NNNN](slug.md) format
    ADR_SLUG=$(echo "$ADR_COL" | python3 -c "
import re, sys
m = re.search(r'\(([^)]+)\)', sys.stdin.read())
print(m.group(1) if m else '')
" 2>/dev/null || echo "")

    # Extract scope pattern — strip backticks
    PATTERN="${SCOPE_RAW#\`}"
    PATTERN="${PATTERN%\`}"

    if [[ -z "$PATTERN" || -z "$ADR_NUM" ]]; then
        continue
    fi

    # Match file against scope pattern (bash glob, unquoted pattern for glob expansion)
    if [[ "$REL_FILE" == $PATTERN ]]; then
        echo ""
        echo "Note: This file is covered by ADR-${ADR_NUM}. Read docs/adr/${ADR_SLUG} for the rationale behind decisions affecting this code."
        echo ""
        exit 0
    fi
done < "$INDEX_FILE"

exit 0
```

**After writing:** run `chmod +x /home/hhewett/.config/pai/hooks/adr-awareness.sh`

**Manual test:** Simulate the hook being called for a converter file:
```bash
echo '{"tool_name":"Edit","tool_input":{"file_path":"/home/hhewett/.local/src/syllago/cli/internal/converter/adapter.go","old_string":"x","new_string":"y"}}' \
  | /home/hhewett/.config/pai/hooks/adr-awareness.sh
```
Expected output contains `Note: This file is covered by ADR-0001`.

**Negative test** (non-scoped file):
```bash
echo '{"tool_name":"Edit","tool_input":{"file_path":"/home/hhewett/.local/src/syllago/cli/internal/tui/app.go","old_string":"x","new_string":"y"}}' \
  | /home/hhewett/.config/pai/hooks/adr-awareness.sh
```
Expected: empty output, exit 0.

**Success Criteria:**
- `echo '{"tool_name":"Edit","tool_input":{"file_path":"/home/hhewett/.local/src/syllago/cli/internal/converter/adapter.go","old_string":"x","new_string":"y"}}' | /home/hhewett/.config/pai/hooks/adr-awareness.sh` → output contains `ADR-0001` — behavioral: hook fires for scoped converter files
- `echo '{"tool_name":"Edit","tool_input":{"file_path":"/home/hhewett/.local/src/syllago/cli/internal/tui/app.go","old_string":"x","new_string":"y"}}' | /home/hhewett/.config/pai/hooks/adr-awareness.sh; echo "exit:$?"` → output is `exit:0` with no ADR note — behavioral: non-scoped files produce no output
- `echo '{"tool_name":"Edit","tool_input":{"file_path":"/tmp/file.go"}}' | /home/hhewett/.config/pai/hooks/adr-awareness.sh; echo "exit:$?"` → `exit:0` with no output — non-repo files silently pass
- `ls -la /home/hhewett/.config/pai/hooks/adr-awareness.sh` → shows `-rwxr-xr-x` permissions — executable

---

### Task 5 — Create adr-commit-gate.sh

**File:** `/home/hhewett/.config/pai/hooks/adr-commit-gate.sh`
**Action:** Write new file. Make executable after writing.

**Depends on:** Task 2 (INDEX.md exists for testing).

**What this hook does:**
1. Read JSON from stdin
2. Extract `command` from `tool_input` (or `input` fallback)
3. Check if command contains `git commit` — exit 0 if not
4. Find git repo root — exit 0 if not in a repo
5. Check if `$REPO_ROOT/docs/adr/INDEX.md` exists — exit 0 if not
6. Run `git diff --cached --name-only` to get staged files
7. For each staged file, check against ADR scopes in INDEX.md
8. Collect matches grouped by enforcement level
9. If any `strict` matches: print blocking message to stderr, exit 2
10. If only `advisory` matches: print warning to stdout, exit 0
11. No matches: exit 0 silently

**Why stderr for blocking messages:** Claude Code displays stderr output from hooks that exit 2. Using stderr for the block message and stdout for the warning message ensures correct routing.

**Complete file content:**

```bash
#!/usr/bin/env bash
# ADR Commit Gate Hook
# PreToolUse hook for Bash commands.
# Intercepts git commit commands and checks staged files against ADR scopes.
# strict ADRs: exit 2 (blocks commit) with message
# advisory ADRs: exit 0 with warning message
# Auto-detects ADR-enabled projects — exits 0 silently without docs/adr/INDEX.md.

set -euo pipefail

# Read JSON input from stdin
INPUT=$(cat)

# Extract command from tool input (handles both tool_input and input schemas)
COMMAND=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    inp = data.get('tool_input', data.get('input', {}))
    print(inp.get('command', ''))
except Exception:
    print('')
" 2>/dev/null || echo "")

# Only intercept git commit commands
if [[ ! "$COMMAND" =~ git[[:space:]]+commit ]]; then
    exit 0
fi

# Find git repo root — exit silently if not in a repo
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo "")
if [[ -z "$REPO_ROOT" ]]; then
    exit 0
fi

# Check if this project uses ADRs — exit silently if not
INDEX_FILE="$REPO_ROOT/docs/adr/INDEX.md"
if [[ ! -f "$INDEX_FILE" ]]; then
    exit 0
fi

# Get staged files
STAGED=$(git diff --cached --name-only 2>/dev/null || echo "")
if [[ -z "$STAGED" ]]; then
    exit 0
fi

# Parse INDEX.md and collect matching ADRs per enforcement level
STRICT_MATCHES=""
ADVISORY_MATCHES=""

while IFS='|' read -ra COLS; do
    RAW_ADR="${COLS[1]:-}"
    RAW_STATUS="${COLS[3]:-}"
    RAW_ENFORCEMENT="${COLS[4]:-}"
    RAW_SCOPE="${COLS[5]:-}"

    # Skip non-data rows
    if [[ ! "$RAW_ADR" =~ \[0-9 ]]; then
        continue
    fi

    ADR_COL=$(echo "$RAW_ADR" | xargs)
    STATUS=$(echo "$RAW_STATUS" | xargs)
    ENFORCEMENT=$(echo "$RAW_ENFORCEMENT" | xargs)
    SCOPE_RAW=$(echo "$RAW_SCOPE" | xargs)

    # Only process accepted ADRs
    if [[ "$STATUS" != "accepted" ]]; then
        continue
    fi

    # Extract ADR number and slug
    ADR_NUM=$(echo "$ADR_COL" | python3 -c "
import re, sys
m = re.search(r'\[([0-9]+)\]', sys.stdin.read())
print(m.group(1) if m else '')
" 2>/dev/null || echo "")

    ADR_SLUG=$(echo "$ADR_COL" | python3 -c "
import re, sys
m = re.search(r'\(([^)]+)\)', sys.stdin.read())
print(m.group(1) if m else '')
" 2>/dev/null || echo "")

    # Extract scope pattern — strip backticks
    PATTERN="${SCOPE_RAW#\`}"
    PATTERN="${PATTERN%\`}"

    if [[ -z "$PATTERN" || -z "$ADR_NUM" ]]; then
        continue
    fi

    # Check each staged file against this ADR's scope
    MATCHED=0
    while IFS= read -r STAGED_FILE; do
        if [[ "$STAGED_FILE" == $PATTERN ]]; then
            MATCHED=1
            break
        fi
    done <<< "$STAGED"

    if [[ "$MATCHED" -eq 0 ]]; then
        continue
    fi

    # Accumulate matches by enforcement level
    ENTRY="  ADR-${ADR_NUM}: docs/adr/${ADR_SLUG}"
    if [[ "$ENFORCEMENT" == "strict" ]]; then
        STRICT_MATCHES="${STRICT_MATCHES}${ENTRY}\n"
    else
        ADVISORY_MATCHES="${ADVISORY_MATCHES}${ENTRY}\n"
    fi

done < "$INDEX_FILE"

# Strict matches: block the commit
if [[ -n "$STRICT_MATCHES" ]]; then
    cat >&2 << BLOCK
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ADR COMMIT GATE — Strict enforcement

Your staged files fall under one or more strictly-enforced ADRs:

$(echo -e "$STRICT_MATCHES")
Review these ADRs and verify your changes align with the recorded decisions.
If they do, re-run the commit command to proceed.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
BLOCK
    exit 2
fi

# Advisory matches: warn but allow
if [[ -n "$ADVISORY_MATCHES" ]]; then
    cat << WARN

ADR Advisory: Your staged files fall under advisory ADRs:

$(echo -e "$ADVISORY_MATCHES")
No action required, but review these ADRs if you're making architectural changes.

WARN
fi

exit 0
```

**After writing:** run `chmod +x /home/hhewett/.config/pai/hooks/adr-commit-gate.sh`

**Manual test — strict block:**
```bash
cd /home/hhewett/.local/src/syllago && \
git add cli/internal/converter/adapter.go 2>/dev/null || true && \
echo '{"tool_name":"Bash","tool_input":{"command":"git commit -m test"}}' \
  | /home/hhewett/.config/pai/hooks/adr-commit-gate.sh; echo "exit:$?"
```
Expected: exit code 2, stderr contains `ADR COMMIT GATE`.

**Manual test — non-commit passthrough:**
```bash
echo '{"tool_name":"Bash","tool_input":{"command":"make build"}}' \
  | /home/hhewett/.config/pai/hooks/adr-commit-gate.sh; echo "exit:$?"
```
Expected: `exit:0`, no output.

**Success Criteria:**
- Running the strict block test above → `exit:2` and stderr contains `ADR COMMIT GATE — Strict enforcement` — behavioral: strict ADR blocks git commit
- `echo '{"tool_name":"Bash","tool_input":{"command":"make build"}}' | /home/hhewett/.config/pai/hooks/adr-commit-gate.sh; echo "exit:$?"` → `exit:0` with no output — non-commit commands pass through silently
- `ls -la /home/hhewett/.config/pai/hooks/adr-commit-gate.sh` → shows `-rwxr-xr-x` — executable

---

## Group C: ADR Skill

### Task 6 — Create skill directory structure

**Action:** Create the following directories (no files yet):
```
mkdir -p /home/hhewett/.config/pai/skills/adr/workflows
mkdir -p /home/hhewett/.config/pai/skills/adr/references
```

**Depends on:** Nothing.

**Success Criteria:**
- `ls /home/hhewett/.config/pai/skills/adr/` → shows `workflows/` and `references/` directories — structure created
- `ls /home/hhewett/.config/pai/skills/` → shows `adr` in the listing — skill is alongside other global skills

---

### Task 7 — Write SKILL.md

**File:** `/home/hhewett/.config/pai/skills/adr/SKILL.md`
**Action:** Write new file.

**Depends on:** Task 6.

**Complete file content:**

```markdown
---
name: adr
description: Create, consult, and review Architecture Decision Records. USE WHEN creating a new ADR OR consulting existing ADRs for current work context OR reviewing ADRs for staleness. Manages docs/adr/ directory with INDEX.md and YAML-frontmattered ADR files.
---

# ADR Skill

Manage Architecture Decision Records (ADRs) — lightweight documents capturing important decisions, their context, and consequences.

## Workflow Routing

| Workflow | Trigger | File |
|----------|---------|------|
| **Create** | `/adr create "Decision Title"` | `workflows/create.md` |
| **Consult** | `/adr` or `/adr consult` | `workflows/consult.md` |
| **Review** | `/adr review` | `workflows/review.md` |

## Conventions

- ADRs live in `docs/adr/` within the current git repo
- Every ADR has YAML frontmatter (see `references/template.md`)
- `INDEX.md` is the single-file summary — hooks parse it directly
- Scope patterns in INDEX.md Scope column are wrapped in backticks
- Enforcement: `strict` = hooks block commits, `advisory` = hooks warn only
- Status lifecycle: `proposed` → `accepted` → `superseded`
- Superseded ADRs: ~~strikethrough~~ title in INDEX.md, update frontmatter status

## Examples

```
/adr create "Database Connection Pooling"
→ Scaffolds docs/adr/0002-database-connection-pooling.md
→ Asks for scope, tags, enforcement
→ Updates INDEX.md with new row

/adr
→ Reads INDEX.md and current work context
→ Surfaces relevant ADRs for files being discussed

/adr review
→ Scans all ADRs for staleness signals
→ Reports dead scopes, stale proposals, superseded references
```
```

**Success Criteria:**
- `cat /home/hhewett/.config/pai/skills/adr/SKILL.md | head -5` → first line is `---`, contains `name: adr` — frontmatter present
- `grep "Workflow Routing" /home/hhewett/.config/pai/skills/adr/SKILL.md` → matches — routing table present

---

### Task 8 — Write workflows/create.md

**File:** `/home/hhewett/.config/pai/skills/adr/workflows/create.md`
**Action:** Write new file.

**Depends on:** Task 6.

**Complete file content:**

```markdown
# Create Workflow

> **Trigger:** `/adr create "Decision Title"`

## Purpose

Scaffold a new ADR file with frontmatter, add it to INDEX.md, and guide the agent to fill in the decision body.

## Steps

### 1. Locate docs/adr/

Find the ADR directory in the current git repo:
```bash
git rev-parse --show-toplevel
```
Append `/docs/adr/`. If the directory doesn't exist, ask the user: "docs/adr/ doesn't exist yet. Should I create it?"

### 2. Determine next ADR number

Read `docs/adr/INDEX.md`. Find the highest ADR number in the table. Increment by 1. Zero-pad to 4 digits (e.g., `0002`).

If INDEX.md doesn't exist, this is the first ADR. Number = `0001`.

### 3. Generate slug

Slugify the title:
- Lowercase
- Replace spaces and special characters with hyphens
- Remove consecutive hyphens
- Example: "Database Connection Pooling" → `database-connection-pooling`

Filename: `NNNN-slug.md`

### 4. Gather required fields

Ask the user (one question at a time):

**Question 1 — Scope:**
"Which files does this decision govern? Enter glob patterns like `cli/internal/foo/*` or `src/api/**`. Separate multiple patterns with commas."

**Question 2 — Enforcement:**
"Should violations block commits (`strict`) or just warn (`advisory`)? Choose `strict` for safety-critical decisions, `advisory` for guidelines."
Default: `advisory`

**Question 3 — Tags:**
"Add 2-4 tags describing the topic area (e.g., `hooks, conversion, safety`). These help the consult workflow find relevant ADRs."

### 5. Write the ADR file

Write `docs/adr/NNNN-slug.md` using the template from `references/template.md`. Pre-fill:
- `id`: NNNN
- `title`: exact title from trigger
- `date`: today's date (YYYY-MM-DD)
- `status`: `proposed`
- `enforcement`: from user answer
- `files`: array from user scope answer, each pattern quoted
- `tags`: array from user tags answer

Leave the body sections (Context, Decision, Consequences) with placeholder text so the agent knows to fill them in.

### 6. Update INDEX.md

Add a row to the table in `docs/adr/INDEX.md`:

```
| [NNNN](NNNN-slug.md) | Title Here | proposed | advisory | `scope/pattern/*` | one-line summary |
```

For the summary: use a brief description of the decision (10-15 words). If there are multiple scope patterns, show the first in the table and add a note in the ADR body.

If INDEX.md doesn't exist yet, create it first using this template:
```markdown
# ADR Index

Architectural decisions for this project. Before modifying files in a listed scope, read the full ADR.

| ADR | Title | Status | Enforcement | Scope | Summary |
|-----|-------|--------|-------------|-------|---------|
```

### 7. Prompt for body content

Tell the agent:

"ADR-NNNN has been scaffolded at `docs/adr/NNNN-slug.md`. Please fill in the three body sections:

- **Context** — What situation led to this decision? What constraints exist?
- **Decision** — What did you decide to do? Be specific.
- **Consequences** — What becomes easier? What becomes harder? What's deferred?

When the body is complete, update `status` from `proposed` to `accepted` in both the frontmatter and the INDEX.md row."
```

**Success Criteria:**
- `cat /home/hhewett/.config/pai/skills/adr/workflows/create.md | grep "## Steps"` → matches — step structure present
- `grep "INDEX.md" /home/hhewett/.config/pai/skills/adr/workflows/create.md` → returns multiple lines — INDEX.md update instructions present

---

### Task 9 — Write workflows/consult.md

**File:** `/home/hhewett/.config/pai/skills/adr/workflows/consult.md`
**Action:** Write new file.

**Depends on:** Task 6.

**Complete file content:**

```markdown
# Consult Workflow

> **Trigger:** `/adr` or `/adr consult`

## Purpose

Surface ADRs relevant to the current work context so the agent is aware of architectural constraints before proceeding.

## Steps

### 1. Read INDEX.md

Find the git repo root and read `docs/adr/INDEX.md`. If it doesn't exist, respond: "No ADR index found in this project. Use `/adr create` to start documenting decisions."

### 2. Identify current work context

Determine what files or topics are currently in scope:
- What files have been recently edited or are under discussion?
- What topic is the current conversation about?
- Are there specific packages, components, or patterns involved?

### 3. Find relevant ADRs

For each accepted ADR in the index:

**Scope match:** Does any recently-edited or discussed file match the ADR's scope pattern?

**Tag match:** Do the ADR's tags overlap with the current topic? Extract tags from the frontmatter of candidate ADRs.

Collect all matching ADRs. Read the full content of each matching ADR file.

### 4. Summarize findings

Present findings clearly:

```
ADR-0001 (strict): Hook Degradation Enforcement
Scope: cli/internal/converter/*

Key constraint: block/warn/exclude strategies must be enforced during conversion.
The degradation field on CanonicalHook is a safety contract — do not drop hooks
that declare degradation: block.

Full ADR: docs/adr/0001-hook-degradation-enforcement.md
```

If no relevant ADRs found: "No ADRs match the current work context. Proceed freely — or consider whether a new ADR is warranted."

If multiple ADRs match: list all of them, starting with `strict` enforcement ones.

### 5. Recommend action

After the summary, suggest: "Review the full ADR before modifying these files. Hooks will remind you again when you save changes."
```

**Success Criteria:**
- `cat /home/hhewett/.config/pai/skills/adr/workflows/consult.md | grep "## Steps"` → matches — step structure present
- `grep "Scope match" /home/hhewett/.config/pai/skills/adr/workflows/consult.md` → matches — matching logic documented

---

### Task 10 — Write workflows/review.md

**File:** `/home/hhewett/.config/pai/skills/adr/workflows/review.md`
**Action:** Write new file.

**Depends on:** Task 6.

**Complete file content:**

```markdown
# Review Workflow

> **Trigger:** `/adr review`

## Purpose

Audit all ADRs for staleness signals and surface actionable findings.

## Steps

### 1. Scan ADR files

List all `*.md` files in `docs/adr/` except `INDEX.md`. Read each file's YAML frontmatter (id, status, date, files, tags).

### 2. Check for staleness signals

Run each check for every ADR:

#### Check A: Stale proposals

ADR has `status: proposed` AND date is more than 6 months ago.

Signal: Decision was proposed but never formally accepted. Either accept it, reject it, or delete it.

#### Check B: Dead scopes

For each pattern in the ADR's `files` array, check whether any files matching that pattern exist in the repo:
```bash
ls $REPO_ROOT/PATTERN 2>/dev/null || echo "no matches"
```
If no files match any scope pattern: scope is dead.

Signal: The files this ADR governs no longer exist. The ADR may be obsolete — consider superseding it or updating the scope.

#### Check C: Superseded ADRs referenced from CLAUDE.md

ADR has `status: superseded` but is still referenced in `CLAUDE.md` or `docs/` by its filename.
```bash
grep -r "NNNN-slug" $REPO_ROOT/CLAUDE.md $REPO_ROOT/docs/ 2>/dev/null
```

Signal: Superseded ADRs should not be referenced as authoritative. Update CLAUDE.md to reference the successor ADR.

#### Check D: INDEX.md sync

For each ADR file found, verify a matching row exists in INDEX.md with the same status. If the frontmatter says `accepted` but INDEX.md says `proposed` (or vice versa), they're out of sync.

### 3. Report findings

Group findings by severity:

**Action required:**
- Dead scopes on accepted ADRs (enforcement logic may be broken)
- INDEX.md sync mismatches (hooks read INDEX.md, not frontmatter)

**Worth reviewing:**
- Stale proposals (> 6 months as proposed)
- Superseded ADRs referenced in docs

**Format each finding:**
```
[ACTION REQUIRED] ADR-0002: Dead scope
  Scope pattern `src/old-module/*` matches no existing files.
  Options: update scope to new path, or supersede this ADR.

[REVIEW] ADR-0003: Stale proposal
  Status has been "proposed" since 2025-09-15 (6+ months).
  Accept, reject, or delete.
```

If no issues found: "All ADRs look healthy. Nothing to action."
```

**Success Criteria:**
- `cat /home/hhewett/.config/pai/skills/adr/workflows/review.md | grep "Check A"` → matches — staleness checks documented
- `grep "INDEX.md sync" /home/hhewett/.config/pai/skills/adr/workflows/review.md` → matches — sync check present

---

### Task 11 — Write references/template.md

**File:** `/home/hhewett/.config/pai/skills/adr/references/template.md`
**Action:** Write new file.

**Depends on:** Task 6.

**Complete file content:**

```markdown
# ADR Template

Use this template when creating new ADRs. The frontmatter drives automation; the body drives readability.

---

## Template

\`\`\`markdown
---
id: "NNNN"
title: Short Decision Title
status: proposed
date: YYYY-MM-DD
enforcement: advisory
files: ["scope/pattern/*"]
tags: [tag1, tag2, tag3]
---

# ADR NNNN: Short Decision Title

## Status

Proposed

## Context

What situation led to this decision? Describe:
- The problem or constraint
- Why a decision was needed
- What alternatives were considered
- Any external pressures (spec requirements, user feedback, performance limits)

## Decision

What did you decide to do? Be specific and direct. Describe:
- The approach chosen
- How it will be implemented
- Any interface or API changes

## Consequences

**What becomes easier:**
- List positive outcomes

**What becomes harder:**
- List trade-offs and costs

**What's deferred:**
- List out-of-scope follow-on work that may be needed later
\`\`\`

---

## Frontmatter Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Zero-padded 4-digit number (`"0001"`) |
| `title` | string | Human-readable decision name |
| `status` | string | `proposed` \| `accepted` \| `superseded` |
| `date` | string | ISO date when decision was made (`YYYY-MM-DD`) |
| `enforcement` | string | `strict` (blocks commits) or `advisory` (warns) |
| `files` | array | Glob patterns for scoped files, each quoted |
| `tags` | array | Topic keywords for consult workflow matching |

## Status Lifecycle

```
proposed → accepted → superseded
```

- Start every new ADR as `proposed`
- Change to `accepted` once the decision is final and the work is done
- Change to `superseded` if a later ADR replaces this one — link to the successor in the body

## INDEX.md Row Format

When adding to INDEX.md, the row format is:

```
| [NNNN](NNNN-slug-title.md) | Title | accepted | strict | `scope/pattern/*` | 10-15 word summary |
```

Superseded ADRs use strikethrough on the title: `~~Title~~`
```

**Success Criteria:**
- `cat /home/hhewett/.config/pai/skills/adr/references/template.md | grep "enforcement"` → matches multiple lines — enforcement field documented
- `grep "Status Lifecycle" /home/hhewett/.config/pai/skills/adr/references/template.md` → matches — lifecycle documented

---

## Group D: Global Registration

### Task 12 — Register hooks in ~/.claude/settings.json

**File:** `/home/hhewett/.claude/settings.json`
**Action:** Edit — add two PostToolUse matchers and one PreToolUse matcher.

**Depends on:** Tasks 4 and 5 (hook scripts must exist before registration).

**What to add:**

In the `"PostToolUse"` array, add two new entries (one for `"Edit"`, one for `"Write"`). These should be inserted **before** the existing `"*"` wildcard matcher to ensure they run with clear intent ordering:

```json
{
  "matcher": "Edit",
  "hooks": [
    {
      "type": "command",
      "command": "/home/hhewett/.config/pai/hooks/adr-awareness.sh"
    }
  ]
},
{
  "matcher": "Write",
  "hooks": [
    {
      "type": "command",
      "command": "/home/hhewett/.config/pai/hooks/adr-awareness.sh"
    }
  ]
}
```

In the `"PreToolUse"` array, add a new entry for `"Bash"`. The existing Bash PreToolUse entry already has three hooks. Add a fourth hook to that same entry by appending to its `"hooks"` array:

```json
{
  "type": "command",
  "command": "/home/hhewett/.config/pai/hooks/adr-commit-gate.sh"
}
```

**Exact edit for PostToolUse:** The current PostToolUse array starts with:
```json
"PostToolUse": [
  {
    "matcher": "Write",
    "hooks": [
```

Insert the two new Edit and Write matchers before the existing `Write` matcher. The result should be:
```json
"PostToolUse": [
  {
    "matcher": "Edit",
    "hooks": [
      {
        "type": "command",
        "command": "/home/hhewett/.config/pai/hooks/adr-awareness.sh"
      }
    ]
  },
  {
    "matcher": "Write",
    "hooks": [
      {
        "type": "command",
        "command": "/home/hhewett/.config/pai/hooks/adr-awareness.sh"
      }
    ]
  },
  {
    "matcher": "Write",
    "hooks": [
      {
        "type": "command",
        "command": "bun run /home/hhewett/.config/pai/hooks/post-brainstorm.ts"
      },
      ...existing Write hooks...
```

**Exact edit for PreToolUse Bash:** Find the existing Bash PreToolUse entry:
```json
{
  "matcher": "Bash",
  "hooks": [
    {
      "type": "command",
      "command": "bun run $PAI_DIR/hooks/sandbox-bypass-gate.ts"
    },
    {
      "type": "command",
      "command": "bun run $PAI_DIR/hooks/security-validator.ts"
    },
    {
      "type": "command",
      "command": "$PAI_DIR/hooks/release-guard.sh"
    }
  ]
}
```

Add the commit gate as a fourth hook:
```json
{
  "matcher": "Bash",
  "hooks": [
    {
      "type": "command",
      "command": "bun run $PAI_DIR/hooks/sandbox-bypass-gate.ts"
    },
    {
      "type": "command",
      "command": "bun run $PAI_DIR/hooks/security-validator.ts"
    },
    {
      "type": "command",
      "command": "$PAI_DIR/hooks/release-guard.sh"
    },
    {
      "type": "command",
      "command": "/home/hhewett/.config/pai/hooks/adr-commit-gate.sh"
    }
  ]
}
```

**After editing:** Validate JSON syntax:
```bash
python3 -m json.tool /home/hhewett/.claude/settings.json > /dev/null && echo "valid JSON"
```

**Success Criteria:**
- `python3 -m json.tool /home/hhewett/.claude/settings.json > /dev/null && echo "valid"` → `valid` — settings.json is valid JSON after edits
- `grep "adr-awareness.sh" /home/hhewett/.claude/settings.json | wc -l` → `2` — awareness hook registered for both Edit and Write
- `grep "adr-commit-gate.sh" /home/hhewett/.claude/settings.json` → matches one line — commit gate registered in PreToolUse Bash

---

### Task 13 — Create skill symlink

**Action:** Create symlink from `~/.claude/skills/adr` to `~/.config/pai/skills/adr/`.

**Depends on:** Tasks 6–11 (skill directory and all files must exist first).

**Command:**
```bash
ln -s /home/hhewett/.config/pai/skills/adr /home/hhewett/.claude/skills/adr
```

If `~/.claude/skills/` doesn't exist:
```bash
mkdir -p /home/hhewett/.claude/skills
ln -s /home/hhewett/.config/pai/skills/adr /home/hhewett/.claude/skills/adr
```

**Success Criteria:**
- `ls -la /home/hhewett/.claude/skills/adr` → shows `-> /home/hhewett/.config/pai/skills/adr` — symlink points to PAI skill
- `cat /home/hhewett/.claude/skills/adr/SKILL.md | head -3` → shows frontmatter starting with `---` — symlink is readable and resolves correctly

---

## End-to-End Validation

After all 13 tasks complete, run these integration checks:

### Integration Check 1 — Awareness hook fires for ADR-0001 scope

```bash
echo '{"tool_name":"Edit","tool_input":{"file_path":"/home/hhewett/.local/src/syllago/cli/internal/converter/adapter.go","old_string":"x","new_string":"y"}}' \
  | /home/hhewett/.config/pai/hooks/adr-awareness.sh
```
Expected: Output contains `Note: This file is covered by ADR-0001. Read docs/adr/0001-hook-degradation-enforcement.md`

### Integration Check 2 — Awareness hook silent for non-scoped file

```bash
echo '{"tool_name":"Edit","tool_input":{"file_path":"/home/hhewett/.local/src/syllago/cli/internal/tui/app.go","old_string":"x","new_string":"y"}}' \
  | /home/hhewett/.config/pai/hooks/adr-awareness.sh
```
Expected: no output, exit 0

### Integration Check 3 — Commit gate blocks for staged converter file

```bash
cd /home/hhewett/.local/src/syllago && \
touch /tmp/adr-test-file.go && \
git add /tmp/adr-test-file.go 2>/dev/null || true && \
echo '{"tool_name":"Bash","tool_input":{"command":"git commit -m test"}}' \
  | /home/hhewett/.config/pai/hooks/adr-commit-gate.sh; echo "exit:$?"
```
Note: this test may show `exit:0` if no converter files are staged. Stage an actual converter file to test blocking behavior.

### Integration Check 4 — Non-ADR project sees zero overhead

```bash
cd /tmp && mkdir adr-test-noadr && cd adr-test-noadr && git init && \
echo '{"tool_name":"Edit","tool_input":{"file_path":"/tmp/adr-test-noadr/anything.go"}}' \
  | /home/hhewett/.config/pai/hooks/adr-awareness.sh; echo "exit:$?" && \
echo '{"tool_name":"Bash","tool_input":{"command":"git commit -m test"}}' \
  | /home/hhewett/.config/pai/hooks/adr-commit-gate.sh; echo "exit:$?" && \
cd /tmp && rm -rf adr-test-noadr
```
Expected: both hooks output nothing, both exit 0

### Integration Check 5 — Skill symlink resolves

```bash
ls /home/hhewett/.claude/skills/adr/SKILL.md
```
Expected: file exists (no "No such file" error)

### Integration Check 6 — settings.json valid JSON

```bash
python3 -m json.tool /home/hhewett/.claude/settings.json > /dev/null && echo "valid"
```
Expected: `valid`

---

## Summary of Files Created/Modified

| Action | Path |
|--------|------|
| Modified | `/home/hhewett/.local/src/syllago/docs/adr/0001-hook-degradation-enforcement.md` |
| Created  | `/home/hhewett/.local/src/syllago/docs/adr/INDEX.md` |
| Modified | `/home/hhewett/.local/src/syllago/CLAUDE.md` |
| Created  | `/home/hhewett/.config/pai/hooks/adr-awareness.sh` |
| Created  | `/home/hhewett/.config/pai/hooks/adr-commit-gate.sh` |
| Created  | `/home/hhewett/.config/pai/skills/adr/SKILL.md` |
| Created  | `/home/hhewett/.config/pai/skills/adr/workflows/create.md` |
| Created  | `/home/hhewett/.config/pai/skills/adr/workflows/consult.md` |
| Created  | `/home/hhewett/.config/pai/skills/adr/workflows/review.md` |
| Created  | `/home/hhewett/.config/pai/skills/adr/references/template.md` |
| Modified | `/home/hhewett/.claude/settings.json` |
| Created  | `/home/hhewett/.claude/skills/adr` (symlink) |
