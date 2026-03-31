# Hooks Empirical Testing Framework - Design Document

**Goal:** Create a diagnostic checklist, executable benchmark hooks, and dogfood plan that empirically validate hooks behavior across 12+ agents and simultaneously test syllago's converter.

**Decision Date:** 2026-03-31

---

## Problem Statement

The hooks spec v0.1.0 has 27 confirmed errors discovered through source code research. But source code research only tells us what the code *says* it does — empirical testing tells us what it *actually* does. We need:

1. A community-facing reference showing what hook behaviors work on which agents
2. Executable tests that can be re-run as agents update their hook implementations
3. A way to validate syllago's converter produces hooks that actually fire on target agents

## Proposed Solution

Three artifacts that work together:

### Artifact 1: Hooks Behavior Checklist

A diagnostic checklist modeled on agentskillimplementation.com/checks/ with 28 checks across 9 categories. Each check has an ID, category, description, rationale, and per-agent expected results.

**Location:** `docs/checks/hooks-behavior-checklist.md`

**Format per check:**
```markdown
### HB-01: before_tool_execute event binding
**Category:** Event Binding
**What it checks:** The agent fires a native event when a tool is about to execute, and syllago's canonical `before_tool_execute` maps to the correct native name.
**Why it matters:** This is the most-used hook event. If the mapping is wrong, blocking hooks silently bind to nothing.
**Automatable:** Yes

| Agent | Expected | Native Event |
|-------|----------|-------------|
| claude-code | PASS | PreToolUse |
| gemini-cli | PASS | BeforeTool |
| cursor | PASS | preToolUse / beforeShellExecution / ... |
| windsurf | PASS | pre_read_code / pre_write_code / ... |
```

**Categories:**

| # | Category | Checks | Source |
|---|----------|--------|--------|
| 1 | Event Binding | 6 | Research Categories A1-A8 |
| 2 | Blocking Behavior | 4 | Research Categories B1-B4 |
| 3 | Capability Support | 5 | Research Categories C1-C5 |
| 4 | Runtime Discovery | 2 | Research Category D1 |
| 5 | Runtime Execution | 4 | Research Categories D2-D6 |
| 6 | Security Posture | 3 | Research Categories E1-E2 |
| 7 | Input Rewrite Safety | 1 | Critical finding (Gemini/Cursor) |
| 8 | Prompt Blocking | 2 | Critical VS Code Copilot finding |
| 9 | Non-Spec Agent Coverage | 1 | Research non-spec agents |

**Automability breakdown:** 22 fully automatable, 4 semi-automatable (need human trigger), 1 slow (timeout testing), 1 manual (security posture assessment).

### Artifact 2: Benchmark Test Hooks

Executable hooks in syllago canonical format — one per automatable check. Each hook is a minimal shell script that logs a PASS line when the target event fires.

**Location:** `content/hooks/benchmark/`

**Structure per hook:**
```
content/hooks/benchmark/hb-01-before-tool-execute/
  .syllago.yaml     # Content metadata
  hb-01.sh          # Test script
```

**`.syllago.yaml` format:**
```yaml
type: hook
name: hb-01-before-tool-execute
description: "Benchmark: validates before_tool_execute event fires"
hook:
  event: before_tool_execute
  blocking: false
  command: ./hb-01.sh
```

**Hook script pattern:**
```bash
#!/bin/bash
# HB-01: before_tool_execute event binding
echo "PASS|HB-01|before_tool_execute|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$SYLLAGO_BENCHMARK_LOG"
```

**Log format:** Pipe-delimited, one line per check result:
```
PASS|HB-01|before_tool_execute|2026-03-31T15:30:00Z
PASS|HB-02|after_tool_execute|2026-03-31T15:30:01Z
FAIL|HB-03|session_start|2026-03-31T15:30:02Z
```

**Environment:** Hooks expect `$SYLLAGO_BENCHMARK_LOG` to be set before test execution. The dogfood plan handles this setup.

### Artifact 3: Syllago Dogfood Plan

Execution playbook for installing and running benchmark hooks across agents using syllago itself.

**Location:** `docs/plans/2026-03-31-hooks-dogfood-plan.md`

**Phases:**

| Phase | Agents | Cost |
|-------|--------|------|
| 1 (Free tier) | Gemini CLI, Cursor, Windsurf, Kiro, Cline, OpenCode, gptme, Pi | Free |
| 2 (Paid tier) | Claude Code, GitHub Copilot | $10-20/mo each |
| 3 (Stretch) | Amazon Q, Augment | Free (if available) |

**Test execution per agent:**
1. `syllago install benchmark-hooks --provider <agent>` — validates converter
2. Verify hook file placement (correct path, correct format for the agent)
3. Trigger each bound event (open session, run tool, submit prompt, etc.)
4. Check `$SYLLAGO_BENCHMARK_LOG` for results
5. Record in `results/<agent-slug>.md`

**Result collection format:**
```markdown
# <agent> Benchmark Results
Agent: <agent-slug>
Date: <ISO date>
Syllago version: <version>

| Check | Status | Notes |
|-------|--------|-------|
| HB-01 | PASS | |
| HB-02 | PASS | |
| HB-03 | SKIP | Agent doesn't support session_end |
```

**What the dogfood validates for syllago:**
- Converter correctly maps canonical events to native events
- Install places hooks in the right location for each agent
- The `.syllago.yaml` content model is sufficient to describe hooks
- Round-trip fidelity: canonical -> native -> hook actually fires

## Architecture

```
docs/checks/hooks-behavior-checklist.md  (reference doc)
    |
    | derives from
    v
docs/research/hook-behavior-checks/      (existing research, 14 files)
    |
    | informs
    v
content/hooks/benchmark/                 (executable test hooks)
    hb-01-before-tool-execute/
    hb-02-after-tool-execute/
    ...
    |
    | installed by syllago to
    v
~/.config/<agent>/hooks/                 (agent-native hook format)
    |
    | produces
    v
/tmp/syllago-benchmark.log              (shared log file)
    |
    | collected into
    v
docs/checks/results/<agent-slug>.md     (per-agent results)
```

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Checklist format | Dachary's ID+category+description | Proven pattern from skill checks; community already familiar |
| Check count | 28 across 9 categories | Comprehensive but manageable; matches research coverage |
| Hook script language | Bash (sh-compatible) | Universal across agents; no runtime deps |
| Logging | Shared log file via env var | Simple, grep-friendly, one file to check per agent |
| Test phases | Free tier first | Validate the framework cheaply before investing in paid agents |
| Content location | `content/hooks/benchmark/` | Dogfoods syllago's content model; installable via registry |

## Success Criteria

1. All 28 checks documented with per-agent expected results
2. 22+ benchmark hooks written in syllago canonical format
3. Benchmark hooks installable on at least 4 agents via `syllago install`
4. At least one full test run completed on a free-tier agent
5. Results collected in structured markdown format

## Open Questions

1. Should benchmark hooks also test `syllago uninstall` cleanup? (Deferred — can add later)
2. Should results be machine-parseable (JSON) in addition to markdown? (Deferred — markdown first)
3. How to handle agents that require IDE context (Kiro IDE, VS Code Copilot)? (Manual testing with result recording)

---

## Next Steps

Ready for implementation planning with `/develop` Plan stage.
