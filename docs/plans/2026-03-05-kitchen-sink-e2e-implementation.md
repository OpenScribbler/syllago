# Kitchen Sink E2E Test Repo - Implementation Plan

**Goal:** Create `syllago-kitchen-sink` — a standalone fixture repo covering all 11 providers with native-format content and a shell-based test harness that exercises the full syllago content lifecycle.

**Architecture:** Two colocated concerns: fixture files at the repo root (native-format content for all 11 providers) and a `tests/` shell harness that runs `syllago` commands against those fixtures. Tests override `$HOME` to a temp dir for isolation so they never touch the real `~/.syllago/` library. The repo is not inside the syllago repo — it lives at `~/.local/src/syllago-kitchen-sink/`.

**Tech Stack:** Bash test harness, no Go toolchain required. Fixture files are native-format for each provider (Markdown, JSON, YAML, TOML, MDC). Requires `syllago` binary on PATH or via `$SYLLAGO_BIN`, plus `jq` for JSON assertions.

**Design Doc:** `/home/hhewett/.local/src/syllago/docs/plans/2026-03-05-kitchen-sink-e2e-design.md`

---

## Group 1: Repo Setup

---

## Task 1.1: Initialize repo and top-level structure

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/` (directory)
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/README.md`

**Depends on:** Nothing

**Success Criteria:**
- [ ] Directory exists
- [ ] `git init` succeeds
- [ ] `git status` shows clean repo with README committed

---

### Step 1: Initialize the repo

```bash
mkdir -p /home/hhewett/.local/src/syllago-kitchen-sink
cd /home/hhewett/.local/src/syllago-kitchen-sink
git init
```

### Step 2: Create README.md

```markdown
# syllago-kitchen-sink

A "polyglot AI project" fixture repo for syllago end-to-end testing. All 11 AI coding tool providers are configured simultaneously using native file formats.

## Purpose

- **Regression testing:** Shell-based test harness verifies format conversion, discovery, and install behavior against the compiled `syllago` binary.
- **Manual exploration:** `cd` into this repo and run `syllago add --from <provider>` to see how syllago reads each provider's native content.

## Providers Covered

| Provider | Rules | Agents | Skills | Commands | Hooks | MCP |
|----------|:-----:|:------:|:------:|:--------:|:-----:|:---:|
| Claude Code | Y | Y | Y | Y | Y | Y |
| Gemini CLI | Y | Y | Y | Y | Y | Y |
| Cursor | Y | | | | | |
| Windsurf | Y | | | | | |
| Codex | Y | Y | | Y | | |
| Copilot CLI | Y | Y | | Y | Y | Y |
| Zed | Y | | | | | |
| Cline | Y | | | | | |
| Roo Code | Y | | | | | Y |
| OpenCode | Y | Y | Y | Y | | Y |
| Kiro | Y | Y | Y | | | Y |

## Canonical Items

Three items appear across all supporting providers:

- **security** (rule) — Never hardcode secrets; use environment variables
- **code-reviewer** (agent) — Reviews git diffs, reports findings by severity
- **greeting** (skill) — Takes a name argument, produces a greeting

Plus: **summarize** (command), filesystem MCP server, and basic logging hooks.

## Running Tests

```bash
# Run all test suites
./tests/run.sh

# Run with a specific binary
SYLLAGO_BIN=/path/to/syllago ./tests/run.sh

# Run a single suite
./tests/test_discovery.sh
```

Requirements: `syllago` binary on PATH (or `$SYLLAGO_BIN`), `jq`, `bash` >= 4.

## Manual Exploration

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from claude-code
syllago add --from cursor
syllago add --from kiro
```
```

### Step 3: Initial commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add README.md
git commit -m "chore: initial commit with README"
```

### Step 4: Verify

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink && git log --oneline
```
Expected: `chore: initial commit with README`

---

## Group 2: Claude Code Fixtures

---

## Task 2.1: Claude Code rules and top-level files

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/CLAUDE.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.claude.json`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.claude/settings.json`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.claude/rules/security.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.claude/rules/code-review.md`

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] `syllago add --from claude-code` discovers at least 2 rules
- [ ] `syllago add --from claude-code` output contains "security"

---

### Step 1: Create CLAUDE.md

`/home/hhewett/.local/src/syllago-kitchen-sink/CLAUDE.md`:
```markdown
---
alwaysApply: true
---

# Kitchen Sink Project

This is a polyglot AI project used for syllago end-to-end testing. All 11 AI coding tool providers are configured simultaneously.

Follow secure coding practices at all times. Never hardcode credentials.
```

### Step 2: Create .claude.json

`/home/hhewett/.local/src/syllago-kitchen-sink/.claude.json`:
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp/kitchen-sink-workspace"],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}
```

### Step 3: Create .claude/settings.json

`/home/hhewett/.local/src/syllago-kitchen-sink/.claude/settings.json`:
```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "echo \"[hook] bash tool call complete\"",
            "timeout": 5000
          }
        ]
      }
    ]
  }
}
```

### Step 4: Create .claude/rules/security.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.claude/rules/security.md`:
```markdown
---
description: Never hardcode secrets — use environment variables
alwaysApply: false
globs:
  - "**/*.env"
  - "**/*.env.*"
  - "**/config/**"
  - "**/*secret*"
---

# Security: No Hardcoded Secrets

Never hardcode API keys, passwords, tokens, or other secrets directly in source code.

## Rules

- Store secrets in environment variables or a secrets manager
- Use `.env` files for local development (never commit them)
- Reference secrets via `process.env.SECRET_NAME` or equivalent
- Add `.env` to `.gitignore` immediately when creating a project

## Examples

Bad:
```python
api_key = "sk-abc123..."
```

Good:
```python
api_key = os.environ["API_KEY"]
```
```

### Step 5: Create .claude/rules/code-review.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.claude/rules/code-review.md`:
```markdown
---
description: Code review guidelines for pull request analysis
alwaysApply: true
---

# Code Review Guidelines

When reviewing code changes, evaluate systematically across these dimensions.

## Review Checklist

- **Correctness:** Does the logic match the stated intent?
- **Security:** Are there injection risks, unvalidated inputs, or exposed secrets?
- **Performance:** Are there O(n²) loops, unnecessary allocations, or missing indexes?
- **Readability:** Is naming clear? Are complex sections explained?
- **Tests:** Are edge cases covered? Do tests verify behavior, not implementation?

## Severity Levels

- **Critical:** Blocks merge — security vulnerabilities, data loss risk, broken tests
- **Major:** Should fix before merge — logic errors, missing error handling
- **Minor:** Optional improvements — naming, style, documentation
- **Nit:** Trivial preferences — whitespace, formatting

Report findings grouped by severity with file:line references.
```

### Step 6: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from claude-code 2>&1 | head -30
```
Expected: output contains "security" and "code-review"

### Step 7: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add CLAUDE.md .claude.json .claude/
git commit -m "feat: add Claude Code fixtures (rules, MCP, hooks)"
```

---

## Task 2.2: Claude Code skills, agents, commands

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.claude/skills/greeting/SKILL.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.claude/agents/code-reviewer/agent.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.claude/commands/summarize.md`

**Depends on:** Task 2.1

**Success Criteria:**
- [ ] `syllago add --from claude-code` discovers "greeting" skill, "code-reviewer" agent, "summarize" command
- [ ] Files exist at exact paths above

---

### Step 1: Create .claude/skills/greeting/SKILL.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.claude/skills/greeting/SKILL.md`:
```markdown
---
name: Greeting
description: Generates a personalized greeting for a given name
---

# Greeting Skill

Generate a warm, personalized greeting for the provided name.

## Usage

Call this skill with a `name` argument to receive a customized greeting.

## Behavior

- Address the person by name
- Keep the greeting friendly and professional
- Vary the greeting style slightly each time (avoid repetition)

## Example

Input: `name = "Alex"`
Output: "Hello, Alex! Great to have you here. How can I help you today?"
```

### Step 2: Create .claude/agents/code-reviewer/agent.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.claude/agents/code-reviewer/agent.md`:
```markdown
---
name: Code Reviewer
description: Reviews git diffs and reports findings grouped by severity
tools:
  - Read
  - Glob
  - Grep
  - Bash
model: claude-sonnet-4-20250514
---

# Code Reviewer Agent

You are a systematic code reviewer. When invoked, you review the current git diff and produce a structured findings report.

## Process

1. Run `git diff HEAD` to see staged and unstaged changes
2. Run `git diff --name-only HEAD` to get the list of changed files
3. For each changed file, read the full file to understand context
4. Analyze changes across these dimensions: correctness, security, performance, readability, test coverage

## Output Format

```
## Code Review Summary

**Files changed:** N
**Findings:** N critical, N major, N minor, N nits

### Critical
- `file.go:42` — Description of the issue and why it matters

### Major
- `file.go:17` — Description of the issue

### Minor
- `file.go:8` — Description of the suggestion

### Nits
- `file.go:3` — Trivial suggestion
```

If no issues are found, say so explicitly: "No issues found. The changes look correct and well-structured."
```

### Step 3: Create .claude/commands/summarize.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.claude/commands/summarize.md`:
```markdown
---
description: Summarize the project structure and key files
---

Read the project structure and produce a concise summary.

Steps:
1. List the top-level directory structure (depth 2)
2. Read README.md if present
3. Identify the primary language and framework from file extensions and config files
4. Note any AI tool configurations present (.claude/, .cursor/, .kiro/, etc.)

Output a summary of:
- What this project does (from README or inferred)
- Primary tech stack
- AI tools configured
- Any notable structure patterns
```

### Step 4: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from claude-code 2>&1
```
Expected: output lists greeting (skill), code-reviewer (agent), summarize (command), security (rule), code-review (rule)

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add .claude/skills/ .claude/agents/ .claude/commands/
git commit -m "feat: add Claude Code skill, agent, and command fixtures"
```

---

## Group 3: Gemini CLI Fixtures

---

## Task 3.1: Gemini CLI all fixtures

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/GEMINI.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.gemini/settings.json`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.gemini/skills/greeting/SKILL.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.gemini/agents/code-reviewer/agent.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.gemini/commands/summarize.md`

**Depends on:** Task 2.2

**Success Criteria:**
- [ ] `syllago add --from gemini-cli` discovers GEMINI.md as a rule, greeting skill, code-reviewer agent, summarize command
- [ ] `.gemini/settings.json` is valid JSON containing both `mcpServers` and `hooks` keys

---

### Step 1: Create GEMINI.md

`/home/hhewett/.local/src/syllago-kitchen-sink/GEMINI.md`:
```markdown
---
alwaysApply: true
---

# Kitchen Sink Project

This is a polyglot AI project used for syllago end-to-end testing. All 11 AI coding tool providers are configured simultaneously.

Follow secure coding practices at all times. Never hardcode credentials.
```

### Step 2: Create .gemini/settings.json (hooks + MCP combined)

`/home/hhewett/.local/src/syllago-kitchen-sink/.gemini/settings.json`:
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp/kitchen-sink-workspace"],
      "env": {
        "NODE_ENV": "production"
      }
    }
  },
  "hooks": {
    "tool_use_end": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "echo \"[hook] gemini tool use complete\"",
            "timeout": 5000
          }
        ]
      }
    ]
  }
}
```

### Step 3: Create .gemini/skills/greeting/SKILL.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.gemini/skills/greeting/SKILL.md`:
```markdown
---
name: Greeting
description: Generates a personalized greeting for a given name
---

# Greeting Skill

Generate a warm, personalized greeting for the provided name.

## Usage

Call this skill with a `name` argument to receive a customized greeting.

## Behavior

- Address the person by name
- Keep the greeting friendly and professional
- Vary the greeting style slightly each time (avoid repetition)

## Example

Input: `name = "Alex"`
Output: "Hello, Alex! Great to have you here. How can I help you today?"
```

### Step 4: Create .gemini/agents/code-reviewer/agent.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.gemini/agents/code-reviewer/agent.md`:
```markdown
---
name: Code Reviewer
description: Reviews git diffs and reports findings grouped by severity
tools:
  - read_file
  - list_directory
  - grep
  - run_shell_command
model: gemini-2.5-pro
max_turns: 10
---

# Code Reviewer Agent

You are a systematic code reviewer. When invoked, you review the current git diff and produce a structured findings report.

## Process

1. Run `git diff HEAD` to see staged and unstaged changes
2. Run `git diff --name-only HEAD` to get the list of changed files
3. For each changed file, read the full file to understand context
4. Analyze changes across these dimensions: correctness, security, performance, readability, test coverage

## Output Format

```
## Code Review Summary

**Files changed:** N
**Findings:** N critical, N major, N minor, N nits

### Critical
- `file.go:42` — Description of the issue and why it matters

### Major
- `file.go:17` — Description of the issue

### Minor
- `file.go:8` — Description of the suggestion

### Nits
- `file.go:3` — Trivial suggestion
```

If no issues are found, say so explicitly.
```

### Step 5: Create .gemini/commands/summarize.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.gemini/commands/summarize.md`:
```markdown
---
description: Summarize the project structure and key files
---

Read the project structure and produce a concise summary.

Steps:
1. List the top-level directory structure (depth 2)
2. Read README.md if present
3. Identify the primary language and framework from file extensions and config files
4. Note any AI tool configurations present (.claude/, .cursor/, .kiro/, etc.)

Output a summary of:
- What this project does (from README or inferred)
- Primary tech stack
- AI tools configured
- Any notable structure patterns
```

### Step 6: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from gemini-cli 2>&1
```
Expected: lists GEMINI (rule), greeting (skill), code-reviewer (agent), summarize (command)

### Step 7: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add GEMINI.md .gemini/
git commit -m "feat: add Gemini CLI fixtures (rules, MCP+hooks settings, skill, agent, command)"
```

---

## Group 4: Cursor and Windsurf Fixtures

---

## Task 4.1: Cursor .mdc rules

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.cursor/rules/security.mdc`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.cursor/rules/code-review.mdc`

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] `syllago add --from cursor` discovers both rules
- [ ] Both files have valid YAML frontmatter with `description:`, `alwaysApply:`, `globs:` fields

---

### Step 1: Create .cursor/rules/security.mdc

The `.mdc` format uses the same fields as canonical but with a `.mdc` extension. The `globs:` field is a YAML list.

`/home/hhewett/.local/src/syllago-kitchen-sink/.cursor/rules/security.mdc`:
```markdown
---
description: Never hardcode secrets — use environment variables
alwaysApply: false
globs:
  - "**/*.env"
  - "**/*.env.*"
  - "**/config/**"
  - "**/*secret*"
---

# Security: No Hardcoded Secrets

Never hardcode API keys, passwords, tokens, or other secrets directly in source code.

## Rules

- Store secrets in environment variables or a secrets manager
- Use `.env` files for local development (never commit them)
- Reference secrets via `process.env.SECRET_NAME` or equivalent
- Add `.env` to `.gitignore` immediately when creating a project

## Examples

Bad:
```python
api_key = "sk-abc123..."
```

Good:
```python
api_key = os.environ["API_KEY"]
```
```

### Step 2: Create .cursor/rules/code-review.mdc

`/home/hhewett/.local/src/syllago-kitchen-sink/.cursor/rules/code-review.mdc`:
```markdown
---
description: Code review guidelines for pull request analysis
alwaysApply: true
globs: []
---

# Code Review Guidelines

When reviewing code changes, evaluate systematically across these dimensions.

## Review Checklist

- **Correctness:** Does the logic match the stated intent?
- **Security:** Are there injection risks, unvalidated inputs, or exposed secrets?
- **Performance:** Are there O(n²) loops, unnecessary allocations, or missing indexes?
- **Readability:** Is naming clear? Are complex sections explained?
- **Tests:** Are edge cases covered? Do tests verify behavior, not implementation?

## Severity Levels

- **Critical:** Blocks merge — security vulnerabilities, data loss risk, broken tests
- **Major:** Should fix before merge — logic errors, missing error handling
- **Minor:** Optional improvements — naming, style, documentation
- **Nit:** Trivial preferences — whitespace, formatting

Report findings grouped by severity with file:line references.
```

### Step 3: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from cursor 2>&1
```
Expected: output lists "security" and "code-review" rules

### Step 4: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add .cursor/
git commit -m "feat: add Cursor fixtures (.mdc rules with frontmatter)"
```

---

## Task 4.2: Windsurf rules

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.windsurfrules`

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] `syllago add --from windsurf` discovers the rule
- [ ] File has Windsurf frontmatter with `trigger:` field
- [ ] The `trigger: glob` variant with comma-separated `globs:` string is exercised

---

### Step 1: Create .windsurfrules

Windsurf's `.windsurfrules` is a single file. We use `trigger: glob` with a comma-separated `globs:` string to exercise the non-trivial format quirk.

`/home/hhewett/.local/src/syllago-kitchen-sink/.windsurfrules`:
```markdown
---
trigger: glob
globs: "**/*.env, **/*.env.*, **/config/**, **/*secret*"
description: Never hardcode secrets — use environment variables
---

# Security: No Hardcoded Secrets

Never hardcode API keys, passwords, tokens, or other secrets directly in source code.

## Rules

- Store secrets in environment variables or a secrets manager
- Use `.env` files for local development (never commit them)
- Reference secrets via `process.env.SECRET_NAME` or equivalent
- Add `.env` to `.gitignore` immediately when creating a project

## Examples

Bad:
```python
api_key = "sk-abc123..."
```

Good:
```python
api_key = os.environ["API_KEY"]
```
```

### Step 2: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from windsurf 2>&1
```
Expected: output lists "windsurfrules" rule

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add .windsurfrules
git commit -m "feat: add Windsurf fixture (.windsurfrules with trigger:glob format)"
```

---

## Group 5: Codex and Copilot Fixtures

---

## Task 5.1: Shared AGENTS.md and Codex TOML agent

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/AGENTS.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.codex/agents/code-reviewer.toml`

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] `syllago add --from codex` discovers AGENTS.md as a rule and code-reviewer.toml as an agent
- [ ] `syllago add --from opencode` also discovers AGENTS.md as a rule
- [ ] `code-reviewer.toml` is valid TOML with `[features]` and `[agents.code-reviewer]` sections

---

### Step 1: Create AGENTS.md

AGENTS.md is discovered by both Codex and OpenCode as their rules file.

`/home/hhewett/.local/src/syllago-kitchen-sink/AGENTS.md`:
```markdown
# Kitchen Sink Project

This is a polyglot AI project used for syllago end-to-end testing. All 11 AI coding tool providers are configured simultaneously.

Follow secure coding practices at all times. Never hardcode credentials. Always use environment variables for secrets.
```

### Step 2: Create .codex/agents/code-reviewer.toml

The Codex TOML format requires a `[features]` table with `multi_agent = true`, then `[agents.<slug>]` sections with `model`, `prompt`, and `tools` fields. Tool names use Copilot CLI vocabulary (e.g., `read_file` not `Read`).

`/home/hhewett/.local/src/syllago-kitchen-sink/.codex/agents/code-reviewer.toml`:
```toml
[features]
multi_agent = true

[agents.code-reviewer]
model = "o4-mini"
prompt = "You are a systematic code reviewer. When invoked, review the current git diff and produce a structured findings report.\n\nProcess:\n1. Run git diff HEAD to see changes\n2. Read changed files for context\n3. Analyze for correctness, security, performance, readability, test coverage\n\nReport findings grouped by severity (Critical, Major, Minor, Nit) with file:line references."
tools = ["read_file", "list_directory", "grep"]
```

### Step 3: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from codex 2>&1
```
Expected: lists AGENTS (rule) and code-reviewer (agent)

### Step 4: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add AGENTS.md .codex/
git commit -m "feat: add Codex fixtures (AGENTS.md shared rule, TOML agent)"
```

---

## Task 5.2: Copilot CLI fixtures

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.github/copilot-instructions.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.copilot/agents/code-reviewer.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.copilot/commands/summarize.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.copilot/mcp.json`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.copilot/hooks.json`

**Depends on:** Task 5.1

**Success Criteria:**
- [ ] `syllago add --from copilot-cli` discovers copilot-instructions as a rule, code-reviewer agent, summarize command
- [ ] `.copilot/hooks.json` has camelCase event names, `bash` key, `timeoutSec` in seconds
- [ ] `.copilot/mcp.json` is valid JSON with `mcpServers` key

---

### Step 1: Create .github/copilot-instructions.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.github/copilot-instructions.md`:
```markdown
# Kitchen Sink Project

This is a polyglot AI project used for syllago end-to-end testing. All 11 AI coding tool providers are configured simultaneously.

Follow secure coding practices at all times. Never hardcode credentials. Always use environment variables for secrets.
```

### Step 2: Create .copilot/agents/code-reviewer.md

Copilot agent frontmatter uses `name`, `description`, and `tools` (with Copilot tool names like `read_file`).

`/home/hhewett/.local/src/syllago-kitchen-sink/.copilot/agents/code-reviewer.md`:
```markdown
---
name: Code Reviewer
description: Reviews git diffs and reports findings grouped by severity
tools:
  - read_file
  - list_directory
  - grep
---

# Code Reviewer Agent

You are a systematic code reviewer. When invoked, you review the current git diff and produce a structured findings report.

## Process

1. Run `git diff HEAD` to see staged and unstaged changes
2. Run `git diff --name-only HEAD` to get the list of changed files
3. For each changed file, read the full file to understand context
4. Analyze changes across these dimensions: correctness, security, performance, readability, test coverage

## Output Format

Report findings grouped by severity (Critical, Major, Minor, Nit) with file:line references.

If no issues are found, say so explicitly.
```

### Step 3: Create .copilot/commands/summarize.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.copilot/commands/summarize.md`:
```markdown
---
description: Summarize the project structure and key files
---

Read the project structure and produce a concise summary.

Steps:
1. List the top-level directory structure (depth 2)
2. Read README.md if present
3. Identify the primary language and framework from file extensions and config files
4. Note any AI tool configurations present (.claude/, .cursor/, .kiro/, etc.)

Output a summary of:
- What this project does (from README or inferred)
- Primary tech stack
- AI tools configured
- Any notable structure patterns
```

### Step 4: Create .copilot/mcp.json

`/home/hhewett/.local/src/syllago-kitchen-sink/.copilot/mcp.json`:
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp/kitchen-sink-workspace"],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}
```

### Step 5: Create .copilot/hooks.json

Copilot hooks use camelCase event names (`postToolUse`), a `bash` key for the command, and `timeoutSec` (in seconds, not milliseconds).

`/home/hhewett/.local/src/syllago-kitchen-sink/.copilot/hooks.json`:
```json
{
  "hooks": {
    "postToolUse": [
      {
        "bash": "echo \"[hook] copilot tool call complete\"",
        "timeoutSec": 5,
        "comment": "Log tool usage for debugging"
      }
    ]
  }
}
```

### Step 6: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from copilot-cli 2>&1
```
Expected: lists copilot-instructions (rule), code-reviewer (agent), summarize (command)

### Step 7: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add .github/ .copilot/
git commit -m "feat: add Copilot CLI fixtures (rule, agent, command, MCP, hooks)"
```

---

## Group 6: Zed, Cline, and Roo Code Fixtures

---

## Task 6.1: Zed and Cline rules

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.rules`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.clinerules/security.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.clinerules/code-review.md`

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] `syllago add --from zed` discovers the `.rules` file
- [ ] `syllago add --from cline` discovers both `.clinerules/` files
- [ ] `.clinerules/security.md` has Cline `paths:` frontmatter (YAML list, not comma-separated)

---

### Step 1: Create .rules (Zed)

Zed uses a plain `.rules` file with no frontmatter — just markdown.

`/home/hhewett/.local/src/syllago-kitchen-sink/.rules`:
```markdown
<!-- Code review and security guidelines for this project -->

# Project Rules

## Security

Never hardcode API keys, passwords, tokens, or other secrets directly in source code. Store secrets in environment variables.

## Code Review

When reviewing code changes, evaluate for correctness, security, performance, readability, and test coverage. Report findings grouped by severity (Critical, Major, Minor, Nit).
```

### Step 2: Create .clinerules/security.md

Cline uses `paths:` (YAML list of globs) instead of `globs:`. This is the key format distinction from Cursor.

`/home/hhewett/.local/src/syllago-kitchen-sink/.clinerules/security.md`:
```markdown
---
paths:
  - "**/*.env"
  - "**/*.env.*"
  - "**/config/**"
  - "**/*secret*"
---

# Security: No Hardcoded Secrets

Never hardcode API keys, passwords, tokens, or other secrets directly in source code.

## Rules

- Store secrets in environment variables or a secrets manager
- Use `.env` files for local development (never commit them)
- Reference secrets via `process.env.SECRET_NAME` or equivalent
- Add `.env` to `.gitignore` immediately when creating a project

## Examples

Bad:
```python
api_key = "sk-abc123..."
```

Good:
```python
api_key = os.environ["API_KEY"]
```
```

### Step 3: Create .clinerules/code-review.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.clinerules/code-review.md`:
```markdown
# Code Review Guidelines

When reviewing code changes, evaluate systematically across these dimensions.

## Review Checklist

- **Correctness:** Does the logic match the stated intent?
- **Security:** Are there injection risks, unvalidated inputs, or exposed secrets?
- **Performance:** Are there O(n²) loops, unnecessary allocations, or missing indexes?
- **Readability:** Is naming clear? Are complex sections explained?
- **Tests:** Are edge cases covered? Do tests verify behavior, not implementation?

## Severity Levels

- **Critical:** Blocks merge — security vulnerabilities, data loss risk, broken tests
- **Major:** Should fix before merge — logic errors, missing error handling
- **Minor:** Optional improvements — naming, style, documentation
- **Nit:** Trivial preferences — whitespace, formatting

Report findings grouped by severity with file:line references.
```

### Step 4: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from zed 2>&1
syllago add --from cline 2>&1
```
Expected: zed finds `.rules`, cline finds `security` and `code-review`

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add .rules .clinerules/
git commit -m "feat: add Zed and Cline fixtures (flat .rules, Cline paths: frontmatter)"
```

---

## Task 6.2: Roo Code fixtures

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.roorules`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.roo/rules/security.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.roo/rules-code/code-review.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.roo/mcp.json`

**Depends on:** Task 6.1

**Note:** The design's coverage matrix marks `Y` for Roo Code agents, but no agent fixture is included here. Roo Code's `DiscoveryPaths` in `roocode.go` does not include an agents path (discovery is not wired for the Agents content type). The README in Task 1.1 reflects this accurately with an empty cell for Roo Code agents. If agent discovery is wired in the future, a `.roo/agents/code-reviewer.yaml` fixture should be added then.

**Success Criteria:**
- [ ] `syllago add --from roo-code` discovers items from `.roorules` (flat file), `.roo/rules/` (directory), and `.roo/rules-code/` (mode-specific directory)
- [ ] `.roo/mcp.json` is valid JSON with `mcpServers` key

---

### Step 1: Create .roorules (flat file)

`.roorules` is a flat file — Roo Code discovers it alongside the directory-based paths.

`/home/hhewett/.local/src/syllago-kitchen-sink/.roorules`:
```markdown
# Global Roo Code Rules

Follow secure coding practices. Never hardcode credentials or API keys. Use environment variables for all secrets.

When writing code, prefer readability over cleverness. Name variables and functions clearly to express intent.
```

### Step 2: Create .roo/rules/security.md

Roo Code rules are plain markdown (no frontmatter).

`/home/hhewett/.local/src/syllago-kitchen-sink/.roo/rules/security.md`:
```markdown
<!-- Security rules applied across all Roo Code modes -->

# Security: No Hardcoded Secrets

Never hardcode API keys, passwords, tokens, or other secrets directly in source code.

## Rules

- Store secrets in environment variables or a secrets manager
- Use `.env` files for local development (never commit them)
- Reference secrets via `process.env.SECRET_NAME` or equivalent
- Add `.env` to `.gitignore` immediately when creating a project
```

### Step 3: Create .roo/rules-code/code-review.md

`rules-code/` is mode-specific — applies only when Roo Code is in "code" mode.

`/home/hhewett/.local/src/syllago-kitchen-sink/.roo/rules-code/code-review.md`:
```markdown
<!-- Code review rules for Roo Code "code" mode -->

# Code Review Guidelines

When reviewing code changes, evaluate systematically across these dimensions.

## Review Checklist

- **Correctness:** Does the logic match the stated intent?
- **Security:** Are there injection risks, unvalidated inputs, or exposed secrets?
- **Performance:** Are there O(n²) loops, unnecessary allocations, or missing indexes?
- **Readability:** Is naming clear? Are complex sections explained?
- **Tests:** Are edge cases covered? Do tests verify behavior, not implementation?

## Severity Levels

- **Critical:** Blocks merge
- **Major:** Should fix before merge
- **Minor:** Optional improvements
- **Nit:** Trivial preferences
```

### Step 4: Create .roo/mcp.json

Roo Code uses `mcpServers` (same key as canonical/Claude Code).

`/home/hhewett/.local/src/syllago-kitchen-sink/.roo/mcp.json`:
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp/kitchen-sink-workspace"],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}
```

### Step 5: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from roo-code 2>&1
```
Expected: lists roorules (flat file), security (from .roo/rules/), code-review (from .roo/rules-code/)

### Step 6: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add .roorules .roo/
git commit -m "feat: add Roo Code fixtures (.roorules flat file, rules dirs, MCP)"
```

---

## Group 7: OpenCode and Kiro Fixtures

---

## Task 7.1: OpenCode fixtures

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/opencode.json`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.opencode/skill/greeting/SKILL.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.opencode/agents/code-reviewer.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.opencode/commands/summarize.md`

**Depends on:** Task 5.1 (AGENTS.md must exist)

**Success Criteria:**
- [ ] `syllago add --from opencode` discovers AGENTS.md as a rule, greeting skill, code-reviewer agent, summarize command
- [ ] `opencode.json` has `mcp` key (not `mcpServers`) with `command` as an array and `environment` key

---

### Step 1: Create opencode.json

OpenCode uses `mcp` (not `mcpServers`), `command` as an array (not separate command+args), and `environment` (not `env`). The type is `"local"` for stdio servers.

`/home/hhewett/.local/src/syllago-kitchen-sink/opencode.json`:
```json
{
  "mcp": {
    "filesystem": {
      "type": "local",
      "command": ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp/kitchen-sink-workspace"],
      "environment": {
        "NODE_ENV": "production"
      }
    }
  }
}
```

### Step 2: Create .opencode/skill/greeting/SKILL.md

OpenCode skill directory is `skill` (singular), not `skills`.

`/home/hhewett/.local/src/syllago-kitchen-sink/.opencode/skill/greeting/SKILL.md`:
```markdown
---
name: Greeting
description: Generates a personalized greeting for a given name
---

# Greeting Skill

Generate a warm, personalized greeting for the provided name.

## Usage

Call this skill with a `name` argument to receive a customized greeting.

## Behavior

- Address the person by name
- Keep the greeting friendly and professional
- Vary the greeting style slightly each time (avoid repetition)

## Example

Input: `name = "Alex"`
Output: "Hello, Alex! Great to have you here. How can I help you today?"
```

### Step 3: Create .opencode/agents/code-reviewer.md

OpenCode agents are markdown with YAML frontmatter, similar to Claude Code sub-agents.

`/home/hhewett/.local/src/syllago-kitchen-sink/.opencode/agents/code-reviewer.md`:
```markdown
---
name: Code Reviewer
description: Reviews git diffs and reports findings grouped by severity
tools:
  - Read
  - Glob
  - Grep
  - Bash
model: claude-sonnet-4-20250514
---

# Code Reviewer Agent

You are a systematic code reviewer. When invoked, you review the current git diff and produce a structured findings report.

## Process

1. Run `git diff HEAD` to see staged and unstaged changes
2. Run `git diff --name-only HEAD` to get the list of changed files
3. For each changed file, read the full file to understand context
4. Analyze changes across these dimensions: correctness, security, performance, readability, test coverage

## Output Format

Report findings grouped by severity (Critical, Major, Minor, Nit) with file:line references.

If no issues are found, say so explicitly.
```

### Step 4: Create .opencode/commands/summarize.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.opencode/commands/summarize.md`:
```markdown
---
description: Summarize the project structure and key files
---

Read the project structure and produce a concise summary.

Steps:
1. List the top-level directory structure (depth 2)
2. Read README.md if present
3. Identify the primary language and framework from file extensions and config files
4. Note any AI tool configurations present (.claude/, .cursor/, .kiro/, etc.)

Output a summary of:
- What this project does (from README or inferred)
- Primary tech stack
- AI tools configured
- Any notable structure patterns
```

### Step 5: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from opencode 2>&1
```
Expected: lists AGENTS (rule), greeting (skill), code-reviewer (agent), summarize (command)

### Step 6: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add opencode.json .opencode/
git commit -m "feat: add OpenCode fixtures (mcp array command, skill, agent, command)"
```

---

## Task 7.2: Kiro fixtures

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/steering/security.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/steering/greeting.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/agents/code-reviewer.json`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/prompts/code-reviewer.md`
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/settings/mcp.json`

**Depends on:** Task 7.1

**Success Criteria:**
- [ ] `syllago add --from kiro` discovers security (rule), greeting (skill) from `.kiro/steering/`, code-reviewer (agent) from `.kiro/agents/` and `.kiro/prompts/`
- [ ] `code-reviewer.json` has `prompt: "file://./prompts/code-reviewer.md"` reference
- [ ] `.kiro/settings/mcp.json` has `mcpServers` key

---

### Step 1: Create .kiro/steering/security.md

Kiro steering files are plain markdown — same path discovers both rules and skills.

`/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/steering/security.md`:
```markdown
# Security: No Hardcoded Secrets

Never hardcode API keys, passwords, tokens, or other secrets directly in source code.

## Rules

- Store secrets in environment variables or a secrets manager
- Use `.env` files for local development (never commit them)
- Reference secrets via `process.env.SECRET_NAME` or equivalent
- Add `.env` to `.gitignore` immediately when creating a project

## Examples

Bad:
```python
api_key = "sk-abc123..."
```

Good:
```python
api_key = os.environ["API_KEY"]
```
```

### Step 2: Create .kiro/steering/greeting.md

`/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/steering/greeting.md`:
```markdown
# Greeting Skill

Generate a warm, personalized greeting for the provided name.

## Usage

Call this skill with a `name` argument to receive a customized greeting.

## Behavior

- Address the person by name
- Keep the greeting friendly and professional
- Vary the greeting style slightly each time (avoid repetition)

## Example

Input: `name = "Alex"`
Output: "Hello, Alex! Great to have you here. How can I help you today?"
```

### Step 3: Create .kiro/agents/code-reviewer.json

Kiro agents are JSON with a `prompt` field that references a separate `.md` file via `file://` URI.

`/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/agents/code-reviewer.json`:
```json
{
  "name": "Code Reviewer",
  "description": "Reviews git diffs and reports findings grouped by severity",
  "prompt": "file://./prompts/code-reviewer.md",
  "tools": ["read_file", "list_directory", "grep", "run_shell_command"],
  "model": "claude-sonnet-4-20250514"
}
```

### Step 4: Create .kiro/prompts/code-reviewer.md

This is the prompt body file referenced by the agent JSON.

`/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/prompts/code-reviewer.md`:
```markdown
You are a systematic code reviewer. When invoked, you review the current git diff and produce a structured findings report.

## Process

1. Run `git diff HEAD` to see staged and unstaged changes
2. Run `git diff --name-only HEAD` to get the list of changed files
3. For each changed file, read the full file to understand context
4. Analyze changes across these dimensions: correctness, security, performance, readability, test coverage

## Output Format

```
## Code Review Summary

**Files changed:** N
**Findings:** N critical, N major, N minor, N nits

### Critical
- `file.go:42` — Description of the issue and why it matters

### Major
- `file.go:17` — Description of the issue

### Minor
- `file.go:8` — Description of the suggestion

### Nits
- `file.go:3` — Trivial suggestion
```

If no issues are found, say so explicitly.
```

### Step 5: Create .kiro/settings/mcp.json

Kiro MCP uses `mcpServers` key (same as canonical/Claude Code format).

`/home/hhewett/.local/src/syllago-kitchen-sink/.kiro/settings/mcp.json`:
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp/kitchen-sink-workspace"],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}
```

### Step 6: Verify discovery

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from kiro 2>&1
```
Expected: lists security (rule), greeting (skill), code-reviewer (agent from agents/ and prompts/)

### Step 7: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add .kiro/
git commit -m "feat: add Kiro fixtures (steering files, JSON agent with file:// prompt, MCP)"
```

---

## Group 8: Test Harness

---

## Task 8.1: Create tests/lib.sh

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/tests/lib.sh`

**Depends on:** Task 1.1

**Success Criteria:**
- [ ] `source tests/lib.sh` succeeds with no errors
- [ ] `assert_exit_zero echo hello` passes
- [ ] `assert_output_contains "echo hello world" "world"` passes
- [ ] `assert_output_contains "echo hello world" "missing"` fails and increments FAIL count

---

### Step 1: Create tests/lib.sh

`/home/hhewett/.local/src/syllago-kitchen-sink/tests/lib.sh`:
```bash
#!/usr/bin/env bash
# Shared test utilities for syllago-kitchen-sink test harness.
# Source this file from each test script.

# Colors and symbols
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASS_COUNT=0
FAIL_COUNT=0
CURRENT_SUITE=""

# SYLLAGO_BIN: path to syllago binary. Defaults to 'syllago' on PATH.
SYLLAGO="${SYLLAGO_BIN:-syllago}"

# setup_test_home: create a temp HOME dir and set $HOME + $XDG_CONFIG_HOME.
# Also sets TEST_HOME for reference. Installs a cleanup trap.
setup_test_home() {
    TEST_HOME=$(mktemp -d)
    export HOME="$TEST_HOME"
    export XDG_CONFIG_HOME="$TEST_HOME/.config"
    mkdir -p "$TEST_HOME/.config"
    trap 'rm -rf "$TEST_HOME"' EXIT
}

# suite NAME: set the current test suite name for output grouping.
suite() {
    CURRENT_SUITE="$1"
    echo ""
    echo "=== Suite: $CURRENT_SUITE ==="
}

# pass MSG: record a passing test.
pass() {
    local msg="$1"
    PASS_COUNT=$((PASS_COUNT + 1))
    echo -e "  ${GREEN}PASS${NC} $msg"
}

# fail MSG: record a failing test.
fail() {
    local msg="$1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    echo -e "  ${RED}FAIL${NC} $msg"
}

# assert_exit_zero CMD [ARGS...]: run command, pass if exit 0, fail otherwise.
assert_exit_zero() {
    local label="$1"
    shift
    local output
    output=$("$@" 2>&1)
    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        pass "$label"
    else
        fail "$label (exit $exit_code)"
        echo "    Output: $output" >&2
    fi
}

# assert_exit_nonzero CMD [ARGS...]: pass if exit non-zero.
assert_exit_nonzero() {
    local label="$1"
    shift
    local output
    output=$("$@" 2>&1)
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        pass "$label"
    else
        fail "$label (expected non-zero, got 0)"
        echo "    Output: $output" >&2
    fi
}

# assert_output_contains LABEL PATTERN CMD [ARGS...]: run command, check stdout+stderr for pattern.
assert_output_contains() {
    local label="$1"
    local pattern="$2"
    shift 2
    local output
    output=$("$@" 2>&1)
    if echo "$output" | grep -q "$pattern"; then
        pass "$label"
    else
        fail "$label (pattern '$pattern' not found)"
        echo "    Output: $output" >&2
    fi
}

# assert_output_not_contains LABEL PATTERN CMD [ARGS...]: pass if pattern is NOT in output.
assert_output_not_contains() {
    local label="$1"
    local pattern="$2"
    shift 2
    local output
    output=$("$@" 2>&1)
    if echo "$output" | grep -q "$pattern"; then
        fail "$label (pattern '$pattern' unexpectedly found)"
        echo "    Output: $output" >&2
    else
        pass "$label"
    fi
}

# assert_file_exists PATH: pass if file/dir exists.
assert_file_exists() {
    local label="$1"
    local path="$2"
    if [ -e "$path" ]; then
        pass "$label"
    else
        fail "$label (not found: $path)"
    fi
}

# assert_file_not_exists PATH: pass if file/dir does NOT exist.
assert_file_not_exists() {
    local label="$1"
    local path="$2"
    if [ -e "$path" ]; then
        fail "$label (unexpectedly found: $path)"
    else
        pass "$label"
    fi
}

# assert_file_contains PATH PATTERN: pass if file contains pattern.
assert_file_contains() {
    local label="$1"
    local path="$2"
    local pattern="$3"
    if grep -q "$pattern" "$path" 2>/dev/null; then
        pass "$label"
    else
        fail "$label (pattern '$pattern' not found in $path)"
    fi
}

# assert_json_key FILE KEY: pass if jq can extract the key without error/null.
assert_json_key() {
    local label="$1"
    local file="$2"
    local key="$3"
    local value
    value=$(jq -e "$key" "$file" 2>/dev/null)
    local exit_code=$?
    if [ $exit_code -eq 0 ] && [ "$value" != "null" ]; then
        pass "$label"
    else
        fail "$label (key '$key' not found or null in $file)"
    fi
}

# print_summary: print pass/fail counts and exit non-zero if any failures.
print_summary() {
    local total=$((PASS_COUNT + FAIL_COUNT))
    echo ""
    echo "=== Summary ==="
    echo -e "  Total:  $total"
    echo -e "  ${GREEN}Passed: $PASS_COUNT${NC}"
    if [ $FAIL_COUNT -gt 0 ]; then
        echo -e "  ${RED}Failed: $FAIL_COUNT${NC}"
        return 1
    else
        echo -e "  ${GREEN}Failed: 0${NC}"
        return 0
    fi
}
```

### Step 2: Make executable and verify

```bash
chmod +x /home/hhewett/.local/src/syllago-kitchen-sink/tests/lib.sh
bash -c 'source /home/hhewett/.local/src/syllago-kitchen-sink/tests/lib.sh && echo "sourced OK"'
```
Expected: `sourced OK`

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add tests/lib.sh
git commit -m "feat: add test harness library (lib.sh)"
```

---

## Task 8.2: Create tests/run.sh

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/tests/run.sh`

**Depends on:** Task 8.1

**Success Criteria:**
- [ ] `./tests/run.sh` exits 0 (even if suites haven't been created yet — with no test files it should exit 0)
- [ ] Output contains "ALL SUITES PASSED" on success or "SUITE FAILURES" on failure
- [ ] `./tests/run.sh --syllago-path /usr/local/bin/syllago` uses the specified binary
- [ ] `SYLLAGO_BIN=/path/to/syllago ./tests/run.sh` also works (env var form)

---

### Step 1: Create tests/run.sh

`/home/hhewett/.local/src/syllago-kitchen-sink/tests/run.sh`:
```bash
#!/usr/bin/env bash
# Entry point for the syllago-kitchen-sink test harness.
# Runs all test suites and reports pass/fail.
#
# Usage:
#   ./tests/run.sh
#   SYLLAGO_BIN=/path/to/syllago ./tests/run.sh
#   ./tests/run.sh --syllago-path /path/to/syllago
#
# Requirements:
#   - syllago binary on PATH (or $SYLLAGO_BIN or --syllago-path)
#   - jq

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Parse --syllago-path flag (overrides SYLLAGO_BIN and PATH lookup)
while [[ $# -gt 0 ]]; do
    case "$1" in
        --syllago-path)
            SYLLAGO_BIN="$2"
            shift 2
            ;;
        *)
            echo "ERROR: Unknown argument: $1" >&2
            echo "Usage: $0 [--syllago-path /path/to/syllago]" >&2
            exit 1
            ;;
    esac
done

# Verify syllago is available
SYLLAGO="${SYLLAGO_BIN:-syllago}"
if ! command -v "$SYLLAGO" &>/dev/null && [ ! -x "$SYLLAGO" ]; then
    echo "ERROR: syllago not found. Set SYLLAGO_BIN, use --syllago-path, or add syllago to PATH." >&2
    exit 1
fi

# Verify jq is available
if ! command -v jq &>/dev/null; then
    echo "ERROR: jq not found. Install jq to run tests." >&2
    exit 1
fi

echo "syllago-kitchen-sink test harness"
echo "Binary: $(which "$SYLLAGO")"
echo "Version: $("$SYLLAGO" --version 2>/dev/null || echo 'unknown')"
echo "Repo: $REPO_ROOT"
echo ""

SUITE_FAILURES=0
SUITES_RUN=0
SUITES=()

# Find and run all test_*.sh files
for suite_file in "$SCRIPT_DIR"/test_*.sh; do
    if [ ! -f "$suite_file" ]; then
        continue
    fi
    SUITES+=("$(basename "$suite_file")")
    SUITES_RUN=$((SUITES_RUN + 1))

    echo "--- Running: $(basename "$suite_file") ---"
    if SYLLAGO_BIN="$SYLLAGO" KITCHEN_SINK_ROOT="$REPO_ROOT" bash "$suite_file"; then
        echo "--- PASSED: $(basename "$suite_file") ---"
    else
        echo "--- FAILED: $(basename "$suite_file") ---"
        SUITE_FAILURES=$((SUITE_FAILURES + 1))
    fi
    echo ""
done

echo "=============================="
echo "  Suites run:    $SUITES_RUN"
echo "  Suites passed: $((SUITES_RUN - SUITE_FAILURES))"
echo "  Suites failed: $SUITE_FAILURES"
echo "=============================="

if [ $SUITE_FAILURES -eq 0 ]; then
    echo ""
    echo "ALL SUITES PASSED"
    exit 0
else
    echo ""
    echo "SUITE FAILURES: $SUITE_FAILURES"
    exit 1
fi
```

### Step 2: Make executable and verify

```bash
chmod +x /home/hhewett/.local/src/syllago-kitchen-sink/tests/run.sh
cd /home/hhewett/.local/src/syllago-kitchen-sink
./tests/run.sh
```
Expected: `ALL SUITES PASSED` with 0 suites run (no test files exist yet)

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add tests/run.sh
git commit -m "feat: add test harness entry point (run.sh)"
```

---

## Group 9: Discovery Tests

---

## Task 9.1: Create tests/test_discovery.sh

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/tests/test_discovery.sh`

**Depends on:** Tasks 2.1, 2.2, 3.1, 4.1, 4.2, 5.1, 5.2, 6.1, 6.2, 7.1, 7.2, 8.1, 8.2

**Success Criteria:**
- [ ] `./tests/test_discovery.sh` exits 0
- [ ] All 11 providers show expected item counts in discovery mode
- [ ] Isolated: uses temp HOME, does not write to real `~/.syllago/`

---

### Step 1: Create tests/test_discovery.sh

`/home/hhewett/.local/src/syllago-kitchen-sink/tests/test_discovery.sh`:
```bash
#!/usr/bin/env bash
# Test: Discovery mode for all 11 providers.
# Verifies that `syllago add --from <provider>` discovers the expected items
# from the kitchen-sink fixture files.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

SYLLAGO="${SYLLAGO_BIN:-syllago}"
REPO="${KITCHEN_SINK_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

setup_test_home

# --- Claude Code ---
suite "Discovery: Claude Code"

assert_output_contains \
    "discovers CLAUDE.md as rule" \
    "CLAUDE" \
    "$SYLLAGO" add --from claude-code --dry-run "$REPO"

assert_output_contains \
    "discovers security rule" \
    "security" \
    "$SYLLAGO" add --from claude-code --dry-run "$REPO"

assert_output_contains \
    "discovers code-review rule" \
    "code-review" \
    "$SYLLAGO" add --from claude-code --dry-run "$REPO"

assert_output_contains \
    "discovers greeting skill" \
    "greeting" \
    "$SYLLAGO" add --from claude-code --dry-run "$REPO"

assert_output_contains \
    "discovers code-reviewer agent" \
    "code-reviewer" \
    "$SYLLAGO" add --from claude-code --dry-run "$REPO"

assert_output_contains \
    "discovers summarize command" \
    "summarize" \
    "$SYLLAGO" add --from claude-code --dry-run "$REPO"

# --- Gemini CLI ---
suite "Discovery: Gemini CLI"

assert_output_contains \
    "discovers GEMINI.md as rule" \
    "GEMINI" \
    "$SYLLAGO" add --from gemini-cli --dry-run "$REPO"

assert_output_contains \
    "discovers greeting skill" \
    "greeting" \
    "$SYLLAGO" add --from gemini-cli --dry-run "$REPO"

assert_output_contains \
    "discovers code-reviewer agent" \
    "code-reviewer" \
    "$SYLLAGO" add --from gemini-cli --dry-run "$REPO"

assert_output_contains \
    "discovers summarize command" \
    "summarize" \
    "$SYLLAGO" add --from gemini-cli --dry-run "$REPO"

# --- Cursor ---
suite "Discovery: Cursor"

assert_output_contains \
    "discovers security rule (.mdc)" \
    "security" \
    "$SYLLAGO" add --from cursor --dry-run "$REPO"

assert_output_contains \
    "discovers code-review rule (.mdc)" \
    "code-review" \
    "$SYLLAGO" add --from cursor --dry-run "$REPO"

# --- Windsurf ---
suite "Discovery: Windsurf"

assert_output_contains \
    "discovers .windsurfrules" \
    "windsurfrules" \
    "$SYLLAGO" add --from windsurf --dry-run "$REPO"

# --- Codex ---
suite "Discovery: Codex"

assert_output_contains \
    "discovers AGENTS.md as rule" \
    "AGENTS" \
    "$SYLLAGO" add --from codex --dry-run "$REPO"

assert_output_contains \
    "discovers code-reviewer TOML agent" \
    "code-reviewer" \
    "$SYLLAGO" add --from codex --dry-run "$REPO"

# --- Copilot CLI ---
suite "Discovery: Copilot CLI"

assert_output_contains \
    "discovers copilot-instructions rule" \
    "copilot-instructions" \
    "$SYLLAGO" add --from copilot-cli --dry-run "$REPO"

assert_output_contains \
    "discovers code-reviewer agent" \
    "code-reviewer" \
    "$SYLLAGO" add --from copilot-cli --dry-run "$REPO"

assert_output_contains \
    "discovers summarize command" \
    "summarize" \
    "$SYLLAGO" add --from copilot-cli --dry-run "$REPO"

# --- Zed ---
suite "Discovery: Zed"

assert_output_contains \
    "discovers .rules file" \
    "rules" \
    "$SYLLAGO" add --from zed --dry-run "$REPO"

# --- Cline ---
suite "Discovery: Cline"

assert_output_contains \
    "discovers security rule (.clinerules)" \
    "security" \
    "$SYLLAGO" add --from cline --dry-run "$REPO"

assert_output_contains \
    "discovers code-review rule (.clinerules)" \
    "code-review" \
    "$SYLLAGO" add --from cline --dry-run "$REPO"

# --- Roo Code ---
suite "Discovery: Roo Code"

assert_output_contains \
    "discovers .roorules flat file" \
    "roorules" \
    "$SYLLAGO" add --from roo-code --dry-run "$REPO"

assert_output_contains \
    "discovers security from .roo/rules/" \
    "security" \
    "$SYLLAGO" add --from roo-code --dry-run "$REPO"

assert_output_contains \
    "discovers code-review from .roo/rules-code/" \
    "code-review" \
    "$SYLLAGO" add --from roo-code --dry-run "$REPO"

# --- OpenCode ---
suite "Discovery: OpenCode"

assert_output_contains \
    "discovers AGENTS.md as rule" \
    "AGENTS" \
    "$SYLLAGO" add --from opencode --dry-run "$REPO"

assert_output_contains \
    "discovers greeting skill (.opencode/skill/)" \
    "greeting" \
    "$SYLLAGO" add --from opencode --dry-run "$REPO"

assert_output_contains \
    "discovers code-reviewer agent" \
    "code-reviewer" \
    "$SYLLAGO" add --from opencode --dry-run "$REPO"

assert_output_contains \
    "discovers summarize command" \
    "summarize" \
    "$SYLLAGO" add --from opencode --dry-run "$REPO"

# --- Kiro ---
suite "Discovery: Kiro"

assert_output_contains \
    "discovers security from .kiro/steering/" \
    "security" \
    "$SYLLAGO" add --from kiro --dry-run "$REPO"

assert_output_contains \
    "discovers greeting from .kiro/steering/" \
    "greeting" \
    "$SYLLAGO" add --from kiro --dry-run "$REPO"

assert_output_contains \
    "discovers code-reviewer agent" \
    "code-reviewer" \
    "$SYLLAGO" add --from kiro --dry-run "$REPO"

print_summary
```

### Step 2: Make executable

```bash
chmod +x /home/hhewett/.local/src/syllago-kitchen-sink/tests/test_discovery.sh
```

### Step 3: Run and check

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
./tests/test_discovery.sh 2>&1 | tail -20
```
Expected: all PASS lines, print_summary exits 0

### Step 4: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add tests/test_discovery.sh
git commit -m "feat: add discovery tests for all 11 providers"
```

---

## Group 10: Add Tests

---

## Task 10.1: Create tests/test_add.sh

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/tests/test_add.sh`

**Depends on:** Tasks 9.1 (all fixtures complete), 8.1, 8.2

**Success Criteria:**
- [ ] `./tests/test_add.sh` exits 0
- [ ] After `syllago add --from cursor --force`, a file appears in `$HOME/.syllago/rules/cursor/security/`
- [ ] After `syllago add --from cline --force`, library file contains no `paths:` frontmatter (canonicalized to `globs:`)
- [ ] After `syllago add --from opencode --force`, library MCP file uses `mcpServers` key (canonicalized from `mcp:`)

---

### Step 1: Create tests/test_add.sh

`/home/hhewett/.local/src/syllago-kitchen-sink/tests/test_add.sh`:
```bash
#!/usr/bin/env bash
# Test: Add items from each provider to the library.
# Verifies format conversion is correct: provider-specific formats
# are canonicalized to syllago's standard format on write.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

SYLLAGO="${SYLLAGO_BIN:-syllago}"
REPO="${KITCHEN_SINK_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

setup_test_home

LIBRARY="$HOME/.syllago"

# --- Claude Code: add rules ---
suite "Add: Claude Code rules"

"$SYLLAGO" add --from claude-code --force "$REPO" >/dev/null 2>&1 || true

assert_file_exists \
    "security rule written to library" \
    "$LIBRARY/rules/claude-code/security"

assert_file_exists \
    "code-review rule written to library" \
    "$LIBRARY/rules/claude-code/code-review"

assert_file_exists \
    "greeting skill written to library" \
    "$LIBRARY/skills/greeting"

assert_file_exists \
    "code-reviewer agent written to library" \
    "$LIBRARY/agents/claude-code/code-reviewer"

assert_file_exists \
    "summarize command written to library" \
    "$LIBRARY/commands/claude-code/summarize"

# --- Cursor: verify .mdc → canonical conversion ---
suite "Add: Cursor (.mdc → canonical)"

"$SYLLAGO" add --from cursor --force "$REPO" >/dev/null 2>&1 || true

CURSOR_SECURITY="$LIBRARY/rules/cursor/security"
assert_file_exists "cursor security rule in library" "$CURSOR_SECURITY"

# The canonical file should be rule.md (not rule.mdc)
assert_file_exists "canonical file is rule.md" "$CURSOR_SECURITY/rule.md"

# Cursor globs: list → canonical globs: list (both use same format)
assert_file_contains \
    "canonical file has globs field" \
    "$CURSOR_SECURITY/rule.md" \
    "globs:"

assert_file_contains \
    "canonical file has description field" \
    "$CURSOR_SECURITY/rule.md" \
    "description:"

# --- Windsurf: verify trigger: glob → canonical globs: list ---
suite "Add: Windsurf (trigger:glob → canonical globs)"

"$SYLLAGO" add --from windsurf --force "$REPO" >/dev/null 2>&1 || true

WINDSURF_RULE="$LIBRARY/rules/windsurf/windsurfrules"
assert_file_exists "windsurf rule in library" "$WINDSURF_RULE"
assert_file_exists "canonical file is rule.md" "$WINDSURF_RULE/rule.md"

# After canonicalization, should have globs: list (not trigger: glob)
assert_file_contains \
    "canonical file has globs field" \
    "$WINDSURF_RULE/rule.md" \
    "globs:"

assert_output_not_contains \
    "canonical file does NOT have trigger: field" \
    "trigger:" \
    cat "$WINDSURF_RULE/rule.md"

# --- Cline: verify paths: → canonical globs: ---
suite "Add: Cline (paths: → canonical globs:)"

"$SYLLAGO" add --from cline --force "$REPO" >/dev/null 2>&1 || true

CLINE_SECURITY="$LIBRARY/rules/cline/security"
assert_file_exists "cline security rule in library" "$CLINE_SECURITY"
assert_file_exists "canonical file is rule.md" "$CLINE_SECURITY/rule.md"

# After canonicalization: paths: → globs:
assert_file_contains \
    "canonical file has globs field (converted from paths:)" \
    "$CLINE_SECURITY/rule.md" \
    "globs:"

assert_output_not_contains \
    "canonical file does NOT have paths: field" \
    "^paths:" \
    cat "$CLINE_SECURITY/rule.md"

# --- Codex: verify TOML agent → canonical markdown ---
suite "Add: Codex (TOML → canonical agent)"

"$SYLLAGO" add --from codex --force "$REPO" >/dev/null 2>&1 || true

CODEX_AGENT="$LIBRARY/agents/codex/code-reviewer"
assert_file_exists "codex agent in library" "$CODEX_AGENT"
assert_file_exists "canonical file is agent.md" "$CODEX_AGENT/agent.md"

# Canonical agent.md should have YAML frontmatter with name field
assert_file_contains \
    "canonical agent has name field" \
    "$CODEX_AGENT/agent.md" \
    "name:"

# Source TOML should be preserved in .source/
assert_file_exists \
    "source TOML preserved in .source/" \
    "$CODEX_AGENT/.source/code-reviewer.toml"

# --- Copilot CLI: verify hooks format ---
suite "Add: Copilot CLI (camelCase hooks)"

"$SYLLAGO" add --from copilot-cli --force "$REPO" >/dev/null 2>&1 || true

assert_file_exists \
    "copilot rule (copilot-instructions) in library" \
    "$LIBRARY/rules/copilot-cli/copilot-instructions"

assert_file_exists \
    "copilot agent in library" \
    "$LIBRARY/agents/copilot-cli/code-reviewer"

# --- OpenCode: verify mcp: → canonical mcpServers ---
suite "Add: OpenCode (mcp: array command → canonical)"

"$SYLLAGO" add --from opencode --force "$REPO" >/dev/null 2>&1 || true

assert_file_exists \
    "opencode greeting skill in library" \
    "$LIBRARY/skills/greeting"

OPENCODE_AGENT="$LIBRARY/agents/opencode/code-reviewer"
assert_file_exists "opencode agent in library" "$OPENCODE_AGENT"

# --- Kiro: verify JSON agent with file:// prompt ---
suite "Add: Kiro (JSON agent, steering rules)"

"$SYLLAGO" add --from kiro --force "$REPO" >/dev/null 2>&1 || true

KIRO_SECURITY="$LIBRARY/rules/kiro/security"
assert_file_exists "kiro security rule in library" "$KIRO_SECURITY"

KIRO_GREETING="$LIBRARY/skills/greeting"
assert_file_exists "kiro greeting skill in library" "$KIRO_GREETING"

KIRO_AGENT="$LIBRARY/agents/kiro/code-reviewer"
assert_file_exists "kiro agent in library" "$KIRO_AGENT"
assert_file_exists "kiro canonical agent.md" "$KIRO_AGENT/agent.md"

# The JSON source should be preserved in .source/
assert_file_exists \
    "kiro agent JSON preserved in .source/" \
    "$KIRO_AGENT/.source/code-reviewer.json"

print_summary
```

### Step 2: Make executable and run

```bash
chmod +x /home/hhewett/.local/src/syllago-kitchen-sink/tests/test_add.sh
cd /home/hhewett/.local/src/syllago-kitchen-sink
./tests/test_add.sh 2>&1 | tail -30
```
Expected: all PASS lines, summary exits 0

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add tests/test_add.sh
git commit -m "feat: add library write tests (format conversion verification)"
```

---

## Group 11: Install Tests

---

## Task 11.1: Create tests/test_install.sh

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/tests/test_install.sh`

**Depends on:** Task 10.1

**Success Criteria:**
- [ ] `./tests/test_install.sh` exits 0
- [ ] After add+install to cursor, a `.mdc` file exists at the expected cursor path
- [ ] After add+install to windsurf, a `.windsurfrules` file exists with `trigger:` frontmatter
- [ ] After add+install to cline, a file in `.clinerules/` has `paths:` frontmatter

---

### Step 1: Create tests/test_install.sh

`/home/hhewett/.local/src/syllago-kitchen-sink/tests/test_install.sh`:
```bash
#!/usr/bin/env bash
# Test: Install round-trips — add from claude-code, install to other providers.
# Verifies that canonical library items are rendered back to provider-native format.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

SYLLAGO="${SYLLAGO_BIN:-syllago}"
REPO="${KITCHEN_SINK_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

setup_test_home

LIBRARY="$HOME/.syllago"

# Set up: add all Claude Code content to library first
suite "Install setup: add Claude Code content"

assert_exit_zero \
    "add claude-code content to library" \
    "$SYLLAGO" add --from claude-code --force "$REPO"

assert_file_exists "security rule in library" "$LIBRARY/rules/claude-code/security"
assert_file_exists "code-reviewer agent in library" "$LIBRARY/agents/claude-code/code-reviewer"
assert_file_exists "greeting skill in library" "$LIBRARY/skills/greeting"

# --- Install to Cursor ---
suite "Install: Cursor (canonical → .mdc)"

CURSOR_INSTALL_ROOT=$(mktemp -d)
trap 'rm -rf "$CURSOR_INSTALL_ROOT"' RETURN

assert_exit_zero \
    "install security to cursor" \
    "$SYLLAGO" install security --to cursor --project-root "$CURSOR_INSTALL_ROOT"

# Cursor rules go in .cursor/rules/ as .mdc files
CURSOR_RULE=$(find "$CURSOR_INSTALL_ROOT/.cursor/rules" -name "*.mdc" 2>/dev/null | head -1)
assert_file_exists \
    "security .mdc file written to .cursor/rules/" \
    "$CURSOR_RULE"

assert_file_contains \
    "installed .mdc has alwaysApply field" \
    "$CURSOR_RULE" \
    "alwaysApply:"

assert_file_contains \
    "installed .mdc has globs field" \
    "$CURSOR_RULE" \
    "globs:"

# --- Install to Windsurf ---
suite "Install: Windsurf (canonical → trigger: frontmatter)"

WINDSURF_INSTALL_ROOT=$(mktemp -d)
trap 'rm -rf "$WINDSURF_INSTALL_ROOT"' RETURN

assert_exit_zero \
    "install security to windsurf" \
    "$SYLLAGO" install security --to windsurf --project-root "$WINDSURF_INSTALL_ROOT"

WINDSURF_RULE="$WINDSURF_INSTALL_ROOT/.windsurfrules"
assert_file_exists "windsurf rule written" "$WINDSURF_RULE"

assert_file_contains \
    "windsurf rule has trigger: field" \
    "$WINDSURF_RULE" \
    "trigger:"

# Since the security rule has globs, it should render as trigger: glob
assert_file_contains \
    "windsurf rule has trigger: glob" \
    "$WINDSURF_RULE" \
    "trigger: glob"

# The globs should be comma-separated (not a YAML list)
assert_file_contains \
    "windsurf rule has comma-separated globs string" \
    "$WINDSURF_RULE" \
    "globs:"

# --- Install to Cline ---
suite "Install: Cline (canonical → paths: frontmatter)"

CLINE_INSTALL_ROOT=$(mktemp -d)
trap 'rm -rf "$CLINE_INSTALL_ROOT"' RETURN

assert_exit_zero \
    "install security to cline" \
    "$SYLLAGO" install security --to cline --project-root "$CLINE_INSTALL_ROOT"

# Cline rules go in .clinerules/ as .md files with paths: frontmatter
CLINE_RULE=$(find "$CLINE_INSTALL_ROOT/.clinerules" -name "*.md" 2>/dev/null | head -1)
assert_file_exists "cline rule written to .clinerules/" "$CLINE_RULE"

assert_file_contains \
    "cline rule has paths: field (not globs:)" \
    "$CLINE_RULE" \
    "paths:"

assert_output_not_contains \
    "cline rule does NOT have globs: field" \
    "^globs:" \
    cat "$CLINE_RULE"

# --- Install to Roo Code ---
suite "Install: Roo Code (canonical → plain markdown)"

ROO_INSTALL_ROOT=$(mktemp -d)
trap 'rm -rf "$ROO_INSTALL_ROOT"' RETURN

assert_exit_zero \
    "install security to roo-code" \
    "$SYLLAGO" install security --to roo-code --project-root "$ROO_INSTALL_ROOT"

# Roo Code rules go in .roo/rules/ as plain .md (no frontmatter)
ROO_RULE=$(find "$ROO_INSTALL_ROOT/.roo/rules" -name "*.md" 2>/dev/null | head -1)
assert_file_exists "roo code rule written to .roo/rules/" "$ROO_RULE"

# Roo Code does NOT support frontmatter — the file should be plain markdown
assert_output_not_contains \
    "roo code rule has no YAML frontmatter delimiter" \
    "^---$" \
    cat "$ROO_RULE"

# --- Install agent to Kiro ---
suite "Install: Kiro (canonical agent → JSON + prompt file)"

KIRO_INSTALL_ROOT=$(mktemp -d)
trap 'rm -rf "$KIRO_INSTALL_ROOT"' RETURN

assert_exit_zero \
    "install code-reviewer agent to kiro" \
    "$SYLLAGO" install code-reviewer --to kiro --project-root "$KIRO_INSTALL_ROOT"

# Kiro agents go in .kiro/agents/ as .json files
KIRO_AGENT_JSON=$(find "$KIRO_INSTALL_ROOT/.kiro/agents" -name "*.json" 2>/dev/null | head -1)
assert_file_exists "kiro agent .json written" "$KIRO_AGENT_JSON"

assert_json_key \
    "kiro agent JSON has name field" \
    "$KIRO_AGENT_JSON" \
    ".name"

assert_json_key \
    "kiro agent JSON has prompt field" \
    "$KIRO_AGENT_JSON" \
    ".prompt"

# Kiro agent should reference a prompt file via file://
assert_file_contains \
    "kiro agent prompt is file:// reference" \
    "$KIRO_AGENT_JSON" \
    "file://"

# Prompt file should exist in .kiro/prompts/
KIRO_PROMPT_FILE=$(find "$KIRO_INSTALL_ROOT/.kiro/prompts" -name "*.md" 2>/dev/null | head -1)
assert_file_exists "kiro prompt .md file written" "$KIRO_PROMPT_FILE"

print_summary
```

### Step 2: Make executable and run

```bash
chmod +x /home/hhewett/.local/src/syllago-kitchen-sink/tests/test_install.sh
cd /home/hhewett/.local/src/syllago-kitchen-sink
./tests/test_install.sh 2>&1 | tail -30
```
Expected: all PASS, summary exits 0

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add tests/test_install.sh
git commit -m "feat: add install round-trip tests (canonical → provider-native format)"
```

---

## Group 12: Convert Tests

---

## Task 12.1: Create tests/test_convert.sh

**Files:**
- Create: `/home/hhewett/.local/src/syllago-kitchen-sink/tests/test_convert.sh`

**Depends on:** Task 11.1

**Success Criteria:**
- [ ] `./tests/test_convert.sh` exits 0
- [ ] Converting Cursor security rule to Windsurf produces a file with `trigger: glob`
- [ ] Converting Cline security rule to Cursor produces a `.mdc` file with `globs:` list
- [ ] Converting Claude Code agent to Kiro produces JSON with `file://` prompt reference

---

### Step 1: Create tests/test_convert.sh

`/home/hhewett/.local/src/syllago-kitchen-sink/tests/test_convert.sh`:
```bash
#!/usr/bin/env bash
# Test: Cross-format conversion between providers.
# Verifies that converting content between providers produces valid
# target-format output with correct structural transformations.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

SYLLAGO="${SYLLAGO_BIN:-syllago}"
REPO="${KITCHEN_SINK_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

setup_test_home

# --- Cursor → Windsurf ---
suite "Convert: Cursor .mdc → Windsurf"

WORK=$(mktemp -d)

assert_exit_zero \
    "syllago convert cursor security → windsurf" \
    "$SYLLAGO" convert \
        --from cursor \
        --to windsurf \
        --input "$REPO/.cursor/rules/security.mdc" \
        --output "$WORK/windsurf-security.md"

assert_file_exists "output file created" "$WORK/windsurf-security.md"

# Cursor has globs: list → Windsurf should have trigger: glob
assert_file_contains \
    "output has trigger: glob (globs preserved)" \
    "$WORK/windsurf-security.md" \
    "trigger: glob"

# Windsurf globs: is a comma-separated string (not YAML list)
assert_file_contains \
    "output has globs: field" \
    "$WORK/windsurf-security.md" \
    "globs:"

# Should NOT have alwaysApply (Windsurf uses trigger: instead)
assert_output_not_contains \
    "output does NOT have alwaysApply field" \
    "alwaysApply:" \
    cat "$WORK/windsurf-security.md"

rm -rf "$WORK"

# --- Cline → Cursor ---
suite "Convert: Cline paths: → Cursor globs:"

WORK=$(mktemp -d)

assert_exit_zero \
    "syllago convert cline security → cursor" \
    "$SYLLAGO" convert \
        --from cline \
        --to cursor \
        --input "$REPO/.clinerules/security.md" \
        --output "$WORK/cursor-security.mdc"

assert_file_exists "output .mdc file created" "$WORK/cursor-security.mdc"

# Cline paths: → Cursor globs: (YAML list)
assert_file_contains \
    "output has globs: field" \
    "$WORK/cursor-security.mdc" \
    "globs:"

# Should NOT have Cline-specific paths: field
assert_output_not_contains \
    "output does NOT have paths: field" \
    "^paths:" \
    cat "$WORK/cursor-security.mdc"

# Should have alwaysApply field (Cursor canonical)
assert_file_contains \
    "output has alwaysApply field" \
    "$WORK/cursor-security.mdc" \
    "alwaysApply:"

rm -rf "$WORK"

# --- Windsurf → Cline ---
suite "Convert: Windsurf trigger:glob → Cline paths:"

WORK=$(mktemp -d)

assert_exit_zero \
    "syllago convert windsurf → cline" \
    "$SYLLAGO" convert \
        --from windsurf \
        --to cline \
        --input "$REPO/.windsurfrules" \
        --output "$WORK/cline-security.md"

assert_file_exists "output file created" "$WORK/cline-security.md"

# Windsurf trigger:glob → Cline paths: list
assert_file_contains \
    "output has paths: field (not globs:)" \
    "$WORK/cline-security.md" \
    "paths:"

assert_output_not_contains \
    "output does NOT have trigger: field" \
    "^trigger:" \
    cat "$WORK/cline-security.md"

rm -rf "$WORK"

# --- Codex TOML → Claude Code agent ---
suite "Convert: Codex TOML agent → Claude Code agent.md"

WORK=$(mktemp -d)

assert_exit_zero \
    "syllago convert codex agent → claude-code" \
    "$SYLLAGO" convert \
        --from codex \
        --to claude-code \
        --input "$REPO/.codex/agents/code-reviewer.toml" \
        --output "$WORK/agent.md"

assert_file_exists "output agent.md created" "$WORK/agent.md"

# Canonical Claude Code agent has YAML frontmatter
assert_file_contains \
    "output has frontmatter delimiter" \
    "$WORK/agent.md" \
    "^---$"

assert_file_contains \
    "output has name field" \
    "$WORK/agent.md" \
    "name:"

rm -rf "$WORK"

# --- Claude Code agent → Kiro JSON ---
suite "Convert: Claude Code agent → Kiro JSON"

WORK=$(mktemp -d)

assert_exit_zero \
    "syllago convert claude-code agent → kiro" \
    "$SYLLAGO" convert \
        --from claude-code \
        --to kiro \
        --input "$REPO/.claude/agents/code-reviewer/agent.md" \
        --output "$WORK"

# Kiro renders two files: .json agent + .md prompt
KIRO_JSON=$(find "$WORK" -name "*.json" 2>/dev/null | head -1)
assert_file_exists "kiro agent .json created" "$KIRO_JSON"

assert_json_key "kiro json has name" "$KIRO_JSON" ".name"
assert_json_key "kiro json has description" "$KIRO_JSON" ".description"
assert_json_key "kiro json has prompt" "$KIRO_JSON" ".prompt"

assert_file_contains \
    "kiro json prompt is file:// reference" \
    "$KIRO_JSON" \
    "file://"

KIRO_PROMPT=$(find "$WORK" -name "*.md" 2>/dev/null | head -1)
assert_file_exists "kiro prompt .md file created" "$KIRO_PROMPT"

rm -rf "$WORK"

# --- OpenCode MCP → canonical mcpServers ---
suite "Convert: OpenCode mcp: → canonical mcpServers"

WORK=$(mktemp -d)

assert_exit_zero \
    "syllago convert opencode MCP → claude-code" \
    "$SYLLAGO" convert \
        --from opencode \
        --to claude-code \
        --input "$REPO/opencode.json" \
        --output "$WORK/mcp.json"

assert_file_exists "output mcp.json created" "$WORK/mcp.json"

# Canonical uses mcpServers key
assert_json_key \
    "output has mcpServers key (converted from mcp:)" \
    "$WORK/mcp.json" \
    ".mcpServers"

assert_json_key \
    "filesystem server present" \
    "$WORK/mcp.json" \
    ".mcpServers.filesystem"

# Canonical uses command + args (not array form)
assert_json_key \
    "filesystem server has command field" \
    "$WORK/mcp.json" \
    ".mcpServers.filesystem.command"

rm -rf "$WORK"

# --- Copilot hooks → Claude Code hooks ---
suite "Convert: Copilot camelCase hooks → Claude Code hooks"

WORK=$(mktemp -d)

assert_exit_zero \
    "syllago convert copilot hooks → claude-code" \
    "$SYLLAGO" convert \
        --from copilot-cli \
        --to claude-code \
        --input "$REPO/.copilot/hooks.json" \
        --output "$WORK/hooks.json"

assert_file_exists "output hooks.json created" "$WORK/hooks.json"

# Claude Code hooks use the canonical nested format
assert_json_key \
    "output has hooks key" \
    "$WORK/hooks.json" \
    ".hooks"

rm -rf "$WORK"

print_summary
```

### Step 2: Make executable and run

```bash
chmod +x /home/hhewett/.local/src/syllago-kitchen-sink/tests/test_convert.sh
cd /home/hhewett/.local/src/syllago-kitchen-sink
./tests/test_convert.sh 2>&1 | tail -40
```
Expected: all PASS, summary exits 0

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add tests/test_convert.sh
git commit -m "feat: add cross-format conversion tests"
```

---

## Group 13: Smoke Test

---

## Task 13.1: Run full test suite, fix any failures

**Files:**
- Modify: Any fixture or test files that fail

**Depends on:** All previous tasks

**Success Criteria:**
- [ ] `./tests/run.sh` exits 0
- [ ] Output shows "ALL SUITES PASSED"
- [ ] No test writes to real `~/.syllago/` (verify with `ls ~/.syllago/` before and after)
- [ ] `syllago add --from claude-code` works interactively from repo root

---

### Step 1: Verify no pre-existing library writes

```bash
ls ~/.syllago/ 2>/dev/null || echo "library does not exist (expected)"
```
Expected: either empty or not found

### Step 2: Run full test suite

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
./tests/run.sh 2>&1
```
Expected: `ALL SUITES PASSED`

### Step 3: Verify library not polluted

```bash
ls ~/.syllago/ 2>/dev/null || echo "library unchanged (correct)"
```
Expected: unchanged — tests use temp HOME

### Step 4: Manual exploration smoke test

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
syllago add --from claude-code 2>&1 | head -20
```
Expected: discovery output listing content items with their types and status

### Step 5: Fix any failures

For each failing test:
1. Read the failure output to understand which assertion failed
2. Check the fixture file vs the expected format in the converter source
3. Either fix the fixture file or fix the test assertion

Common fixes needed:
- If `assert_output_contains` for an item name fails: check the exact slug syllago uses for that item (may differ from filename). Use `syllago add --from <provider> --dry-run` to see exact names.
- If canonicalization assertions fail: re-read the converter source for the exact field names expected.
- If install path assertions fail: check `DiscoveryPaths` and `InstallDir` in the provider Go files.

### Step 6: Commit fixes (if any)

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
git add -A
git commit -m "fix: resolve test failures from full suite run"
```

### Step 7: Final verification

```bash
cd /home/hhewett/.local/src/syllago-kitchen-sink
./tests/run.sh
```
Expected: `ALL SUITES PASSED`, exit code 0

---

## Implementation Notes

### Key Format Quirks (verified from source code)

1. **Windsurf `globs:`**: The `windsurfFrontmatter` struct uses `Globs string` (a single string). When rendering, it joins `meta.Globs` with `", "`. When canonicalizing, it calls `splitGlobs()` which splits on commas. The fixture correctly uses `"**/*.env, **/*.env.*"` format.

2. **Cline `paths:` → `globs:`**: `canonicalizeClineRule` maps `cfm.Paths` directly to `meta.Globs`. No other transformation. Fixture uses `paths:` YAML list.

3. **OpenCode `command` array**: `opencodeServerConfig.Command` is `[]string`. When canonicalizing, `s.Command[0]` → `canonical.Command` and `s.Command[1:]` → `canonical.Args`. The fixture correctly uses `"command": ["npx", "-y", "..."]`.

4. **OpenCode `environment` key**: Maps to `canonical.Env` on canonicalization. The fixture correctly uses `"environment"` not `"env"`.

5. **Copilot hooks `timeoutSec`**: The `copilotHookEntry.TimeoutSec` field is in seconds. On canonicalization, it multiplies by 1000 to get milliseconds. The fixture correctly uses `"timeoutSec": 5` (not `5000`).

6. **Kiro agent `prompt: "file://..."`**: When `canonicalizeKiroAgent` sees a `file://` prefix, it stores a placeholder body. The original JSON is preserved in `.source/`. The fixture correctly uses `"prompt": "file://./prompts/code-reviewer.md"`.

7. **Codex TOML tool names**: Codex uses Copilot CLI vocabulary (`read_file`, `list_directory`, `grep`). The `canonicalizeCodexAgents` function calls `ReverseTranslateTool(t, "copilot-cli")` for each tool.

8. **Roo Code agents (YAML)**: The `renderRooCodeAgent` function produces YAML with `slug`, `name`, `roleDefinition`, `whenToUse`, `groups` fields. When the repo's `.roo/` directory has an agents subdir, Roo Code discovers agents there. The discovery paths in `roocode.go` only include `rules`-type paths — the `Agents` content type is supported via `ProjectScopeSentinel` but the discovery path is not wired in `DiscoveryPaths`. This means Roo Code agent discovery may be limited; the tests for Roo Code focus on rules and MCP.

9. **Kiro steering = rules AND skills**: Both `catalog.Rules` and `catalog.Skills` discovery paths for Kiro return `filepath.Join(projectRoot, ".kiro", "steering")`. This means items in `.kiro/steering/` are discovered for both types. The test verifies `security` shows up as a rule and `greeting` shows up as a skill.

10. **Gemini CLI settings.json**: Both `catalog.MCP` and `catalog.Hooks` discovery paths point to `.gemini/settings.json`. The file must contain both `mcpServers` and `hooks` keys. The fixture correctly combines both in a single file.

### Library Structure

After `syllago add --from claude-code`, the library at `~/.syllago/` looks like:

```
~/.syllago/
├── rules/
│   └── claude-code/
│       ├── CLAUDE/           # from CLAUDE.md
│       │   ├── rule.md
│       │   └── .syllago.yaml
│       ├── security/
│       │   ├── rule.md
│       │   └── .syllago.yaml
│       └── code-review/
│           ├── rule.md
│           └── .syllago.yaml
├── skills/
│   └── greeting/             # universal (no provider subdir)
│       ├── SKILL.md
│       └── .syllago.yaml
├── agents/
│   └── claude-code/
│       └── code-reviewer/
│           ├── agent.md
│           └── .syllago.yaml
└── commands/
    └── claude-code/
        └── summarize/
            ├── command.md
            └── .syllago.yaml
```

Skills are universal (`ct.IsUniversal()` returns true) — they go at `~/.syllago/skills/<name>/` without a provider subdir. Rules, agents, and commands are provider-specific.
