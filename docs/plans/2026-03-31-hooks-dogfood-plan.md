# Hooks Benchmark Dogfood Plan

**Created:** 2026-03-31
**Benchmark hooks:** `content/hooks/benchmark/`
**Checklist:** `docs/checks/hooks-behavior-checklist.md`
**Results:** `docs/checks/results/<agent-slug>.md`

## Purpose

This plan walks through installing and running the hooks behavior benchmark suite across AI coding agents using syllago itself. It validates two things simultaneously:

1. **Agent behavior** — does the agent fire the expected events and honor blocking signals?
2. **Syllago converter** — does `syllago install` produce hook configs that work on each agent?

## Setup

Before any testing, set the benchmark log location:

```bash
export SYLLAGO_BENCHMARK_LOG=/tmp/syllago-benchmark.log
touch "$SYLLAGO_BENCHMARK_LOG"
```

To watch results live:

```bash
tail -f /tmp/syllago-benchmark.log
```

## Test Execution Per Agent

For each agent:

**Step 1: Install benchmark hooks via syllago**

```bash
syllago install hb-01-before-tool-execute --provider <agent-slug>
syllago install hb-02-after-tool-execute --provider <agent-slug>
syllago install hb-03-session-start --provider <agent-slug>
# ... for each applicable check
```

Or, once registry support lands:
```bash
syllago install content/hooks/benchmark --provider <agent-slug> --all
```

**Step 2: Verify hook file placement**

Inspect the agent's config file to confirm:
- Hook entries are present at the correct path
- Event names are converted to the agent's native names
- Shell command is pointing to the installed script
- No YAML/JSON syntax errors

**Step 3: Trigger events**

| Event | How to trigger |
|-------|---------------|
| before_tool_execute | Ask agent to run a shell command or read a file |
| after_tool_execute | Same — fires after step above |
| session_start | Open a new session / restart the agent |
| session_end | Close the session |
| before_prompt | Submit any chat message |
| agent_stop | Ask agent a question, wait for it to finish |

**Step 4: Check log**

```bash
cat /tmp/syllago-benchmark.log
```

Expected format:
```
PASS|HB-01|before_tool_execute|2026-03-31T15:30:00Z
PASS|HB-02|after_tool_execute|2026-03-31T15:30:01Z
PASS|HB-03|session_start|2026-03-31T15:30:02Z
```

**Step 5: Record results**

Copy `docs/checks/results/_template.md` to `docs/checks/results/<agent-slug>.md` and fill in each check's status.

---

## Phase 1: Free Agents

Target agents with free access. Run all applicable checks from HB-01–28.

**Note on ordering:** The design doc lists gptme and Pi as Phase 1 (free) and Amazon Q / Augment as Phase 3 (stretch). This plan inverts that: Amazon Q and Augment are mainstream CLI agents with straightforward shell hook models — they're better Phase 1 targets. gptme and Pi use language-native in-process hooks (Python and TypeScript respectively) — benchmark shell scripts won't run on them, so they're stretch. Kiro is split by mode: kiro-cli (Phase 1) vs kiro-ide (Phase 3, requires IDE environment).

### Gemini CLI

**Prerequisites:** Gemini CLI installed, `gemini` in PATH

```bash
# Install hooks
syllago install hb-01-before-tool-execute --provider gemini-cli
syllago install hb-02-after-tool-execute --provider gemini-cli
syllago install hb-03-session-start --provider gemini-cli
syllago install hb-04-session-end --provider gemini-cli
syllago install hb-05-before-prompt --provider gemini-cli
syllago install hb-06-agent-stop --provider gemini-cli
syllago install hb-11-structured-output --provider gemini-cli
syllago install hb-12-input-rewrite --provider gemini-cli
syllago install hb-16-discovery --provider gemini-cli
syllago install hb-19-env-vars --provider gemini-cli
syllago install hb-20-stdin --provider gemini-cli

# Verify placement
cat ~/.gemini/settings.json | jq '.hooks'

# Run session
gemini
# Trigger: open session (HB-03), submit prompt (HB-05), run shell tool (HB-01, HB-02), exit (HB-04, HB-06)

# Check results
cat /tmp/syllago-benchmark.log
```

**Expected checks to PASS:** HB-01, HB-02, HB-03, HB-04, HB-05, HB-06, HB-07, HB-11, HB-12 (uses `hookSpecificOutput.tool_input`)
**Expected checks to SKIP:** HB-22, HB-24 (not automatable)

**Format Note:** After installation, inspect `~/.gemini/settings.json`. Verify hook uses `hookSpecificOutput.tool_input` (not generic `updatedInput`). The benchmark hook logs in generic format for cross-agent testing, but the Gemini CLI converter should produce the agent-specific field name. If the converter emits `updatedInput`, that's a converter bug — Gemini CLI won't recognize it.

### Windsurf

**Prerequisites:** Windsurf installed

Note: Windsurf has no session_start/session_end events. Skip HB-03, HB-04.
Note: Windsurf uses exit codes only — no JSON structured output. Expect HB-08, HB-11 to FAIL.
Note: Windsurf `before_prompt` CAN block (exit 2). Expect HB-09 to PASS.

```bash
syllago install hb-01-before-tool-execute --provider windsurf
syllago install hb-02-after-tool-execute --provider windsurf
syllago install hb-05-before-prompt --provider windsurf
syllago install hb-06-agent-stop --provider windsurf
syllago install hb-16-discovery --provider windsurf
syllago install hb-19-env-vars --provider windsurf

cat ~/.codeium/windsurf/hooks.json

# Trigger events in Windsurf
# Check results
cat /tmp/syllago-benchmark.log
```

### Cursor

**Prerequisites:** Cursor installed

Note: Cursor has sessionStart/sessionEnd. Expect HB-03, HB-04 to PASS.
Note: Cursor supports input_rewrite via `preToolUse.updated_input`. Expect HB-25 to PASS (no downgrade).

```bash
syllago install hb-01-before-tool-execute --provider cursor
syllago install hb-03-session-start --provider cursor
syllago install hb-04-session-end --provider cursor
syllago install hb-12-input-rewrite --provider cursor
syllago install hb-25-input-rewrite-no-downgrade --provider cursor
syllago install hb-16-discovery --provider cursor
syllago install hb-19-env-vars --provider cursor
syllago install hb-20-stdin --provider cursor

cat ~/.cursor/hooks.json

# Trigger events in Cursor
cat /tmp/syllago-benchmark.log
```

### OpenCode

**Prerequisites:** OpenCode CLI installed

Note: OpenCode is in-process TypeScript. Script-based hooks use a different mechanism (shell.env, tool plugins). Most checks will SKIP.

```bash
# OpenCode uses opencode.json plugins dir — verify converter output
syllago install hb-01-before-tool-execute --provider opencode
syllago install hb-16-discovery --provider opencode

cat opencode.json

# Trigger tool use
opencode
cat /tmp/syllago-benchmark.log
```

### Amazon Q Developer

**Prerequisites:** AWS CLI installed, `q` CLI available, `~/.aws/amazonq/agents/` accessible

```bash
syllago install hb-01-before-tool-execute --provider amazon-q
syllago install hb-05-before-prompt --provider amazon-q
syllago install hb-03-session-start --provider amazon-q
syllago install hb-16-discovery --provider amazon-q

cat ~/.aws/amazonq/agents/benchmark.json

q chat
# Use /agent benchmark to activate
# Trigger events
cat /tmp/syllago-benchmark.log
```

### Cline

**Prerequisites:** Cline installed in VS Code

Note: Cline uses directory-based auto-discovery. Syllago install should create script files in the hooks directory, not JSON config.

```bash
syllago install hb-01-before-tool-execute --provider cline
# Verify: ls ~/Documents/Cline/Hooks/
# Trigger tool use in Cline
cat /tmp/syllago-benchmark.log
```

### Augment Code

**Prerequisites:** Augment Code CLI installed

```bash
syllago install hb-01-before-tool-execute --provider augment-code
syllago install hb-03-session-start --provider augment-code
syllago install hb-04-session-end --provider augment-code
syllago install hb-05-before-prompt --provider augment-code
syllago install hb-06-agent-stop --provider augment-code
syllago install hb-16-discovery --provider augment-code
syllago install hb-19-env-vars --provider augment-code

cat ~/.augment/settings.json | jq '.hooks'
# Trigger events
cat /tmp/syllago-benchmark.log
```

### Kiro CLI

**Prerequisites:** Kiro CLI installed

```bash
syllago install hb-01-before-tool-execute --provider kiro-cli
syllago install hb-05-before-prompt --provider kiro-cli
syllago install hb-16-discovery --provider kiro-cli

ls .kiro/hooks/
# Trigger events
cat /tmp/syllago-benchmark.log
```

---

## Phase 2: Paid Agents

Run after Phase 1 validation confirms the framework is working. All applicable checks HB-01–28.

### Claude Code

**Prerequisites:** Claude Code subscription active

```bash
syllago install hb-01-before-tool-execute --provider claude-code
syllago install hb-02-after-tool-execute --provider claude-code
syllago install hb-03-session-start --provider claude-code
syllago install hb-04-session-end --provider claude-code
syllago install hb-05-before-prompt --provider claude-code
syllago install hb-06-agent-stop --provider claude-code
syllago install hb-07-block-exit2 --provider claude-code
syllago install hb-08-block-json --provider claude-code
syllago install hb-10-stop-continue --provider claude-code
syllago install hb-11-structured-output --provider claude-code
syllago install hb-12-input-rewrite --provider claude-code
syllago install hb-16-discovery --provider claude-code
syllago install hb-19-env-vars --provider claude-code
syllago install hb-20-stdin --provider claude-code
syllago install hb-21-error-handling --provider claude-code

cat ~/.claude/settings.json | jq '.hooks'

claude
# Trigger all event types
cat /tmp/syllago-benchmark.log
```

**Expected checks to PASS:** HB-01–12, HB-16, HB-18, HB-19, HB-20, HB-21

### VS Code Copilot

**Prerequisites:** VS Code Copilot subscription

Note: Critical check — HB-26 (before_prompt is observe-only). Expect this to demonstrate that exit code 2 does NOT block prompts.

```bash
syllago install hb-01-before-tool-execute --provider vscode-copilot
syllago install hb-26-vscode-copilot-prompt-observe --provider vscode-copilot
syllago install hb-16-discovery --provider vscode-copilot

ls .github/hooks/
# Open VS Code with Copilot
# Submit a prompt — observe whether it is processed despite HB-26 hook exiting 2
cat /tmp/syllago-benchmark.log
```

**Expected outcome for HB-26:** Log shows `PASS|HB-26|prompt_block_test|...` (hook fired), but your message IS processed (proving observe-only).

### Copilot CLI

**Prerequisites:** GitHub Copilot CLI installed (`gh extension install github/gh-copilot` or standalone), GitHub Copilot subscription

Note: Copilot CLI uses project-only hooks (no user-level config). Config path is `.github/hooks/`.
Note: Many checks are UNVERIFIED — this is the highest-priority empirical target.

```bash
syllago install hb-01-before-tool-execute --provider copilot-cli
syllago install hb-02-after-tool-execute --provider copilot-cli
syllago install hb-03-session-start --provider copilot-cli
syllago install hb-05-before-prompt --provider copilot-cli
syllago install hb-06-agent-stop --provider copilot-cli
syllago install hb-07-block-exit2 --provider copilot-cli
syllago install hb-16-discovery --provider copilot-cli
syllago install hb-19-env-vars --provider copilot-cli
syllago install hb-20-stdin --provider copilot-cli

ls .github/hooks/

# Trigger tool use via copilot CLI
# Check results
cat /tmp/syllago-benchmark.log
```

**Expected checks to PASS:** HB-01, HB-02, HB-03, HB-05, HB-06, HB-07, HB-16
**High-value UNVERIFIED targets:** HB-08 (JSON blocking), HB-09 (prompt blocking), HB-11 (structured output), HB-15 (custom env)

---

## Phase 3: Stretch

Run if agents are available. Record results even if only partial checks are possible.

### Kiro IDE

**Prerequisites:** Kiro IDE installed (requires IDE environment — cannot run headless)

Focus on: HB-01 (tool events), HB-16 (discovery), HB-22 (security posture manual assessment)

### gptme

**Prerequisites:** gptme installed

Note: gptme uses Python in-process hooks, not shell scripts. The benchmark hooks are bash scripts — they won't work as gptme plugins. Record SKIP for most checks. Manually document the event model.

### Pi Agent

**Prerequisites:** Pi Agent installed

Note: Pi uses TypeScript in-process hooks. Same situation as gptme. Record SKIP for automated checks. Manually document capabilities for HB-28.

---

## Manual Checks (HB-13, HB-17, HB-22–24, HB-27–28)

These checks cannot be tested with automated benchmark hooks. Verify them manually and record results directly in the agent's result file.

### HB-13: LLM-Evaluated Hooks
For agents supporting LLM hook types (claude-code `prompt`/`agent` types, kiro-ide "Ask Kiro"):
1. Configure an LLM-evaluated hook in the agent's settings
2. Trigger the bound event and observe whether the LLM handler runs
3. Record: PASS (LLM hook executes as expected) / SKIP (agent doesn't support this)

### HB-17: Config Precedence (Multi-Level Merge)
For agents with multiple config levels (claude-code has user + project, gemini-cli has 4 levels, cursor has 4 tiers):
1. Install a hook at the user/global level
2. Install the same hook at the project level with different behavior (e.g., different log message)
3. Verify: Which takes precedence? Both run? Does behavior match documentation?
4. Record: PASS (behavior matches documented precedence) / FAIL (unexpected merge behavior)

Best tested on Claude Code first (user: `~/.claude/settings.json`, project: `.claude/settings.json`).

### HB-22: Supply Chain Protection
1. Clone or open a repo that contains a hook config file (`.claude/settings.json`, `.gemini/settings.json`, etc.)
2. Observe: Does the agent prompt for approval before executing project-scoped hooks?
3. Record: PASS (approval required) / PARTIAL (warning shown but auto-executes) / FAIL (silent auto-execute)

### HB-23: Environment Variable Protection
1. Review agent documentation for env var filtering mechanisms
2. Check: Does agent restrict which env vars hook processes receive (allowlist, denylist, sanitization)?
3. Record: PASS (env vars restricted) / PARTIAL (some restriction) / FAIL (full parent env passed through)

### HB-24: Hook Sandboxing
1. Check: Does agent provide any hook execution sandbox (gVisor, Docker, namespace, capabilities)?
2. If yes: test network access and file system access from within a hook
3. Record: PASS (sandbox enforced) / PARTIAL (some isolation, e.g., network only) / FAIL (no sandbox)

### HB-27: Prompt Blocking Alternatives
For agents where `before_prompt` is observe-only (VS Code Copilot confirmed):
1. Document what alternative blocking mechanisms exist (server-side filtering, admin policies, tool-level blocking)
2. Record: the specific alternatives available, not pass/fail

### HB-28: Non-Spec Agent Baseline
For Amazon Q, Cline, Augment, gptme, Pi:
1. Verify syllago converter can handle the agent (or document that it can't yet)
2. Test basic hook installation and event firing
3. Record: agent supports hooks (Yes/No), converter status (Works/Partial/Not yet implemented)

---

## Result Collection

After each agent run:

1. Clear the log: `> /tmp/syllago-benchmark.log`
2. Run the agent session (trigger all applicable events)
3. Copy `docs/checks/results/_template.md` to `docs/checks/results/<agent-slug>.md`
4. Fill in each check row with status (PASS/FAIL/SKIP/PARTIAL/UNVERIFIED/MANUAL)
5. Paste contents of `/tmp/syllago-benchmark.log` into the "Raw Log" code block in the result file
6. Add notes for any converter issues, unexpected behavior, or format mismatches
7. Commit only `.md` files (no separate `.log` files — log content lives inside the result markdown)

## What a Complete Run Produces

- One `docs/checks/results/<agent-slug>.md` per tested agent (log embedded in markdown)
- Empirical confirmation (or correction) of expected results in the checklist
- Bug reports for syllago converter if installed hooks don't fire as expected
- Updated `docs/checks/hooks-behavior-checklist.md` with confirmed results replacing UNVERIFIED
- Manual check results for HB-13, HB-17, HB-22–24, HB-27–28 recorded in each agent's result file
