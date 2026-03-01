# Contribution Model Implementation Plan

## Overview

Implement four components of the contribution model:
1. **GitHub Action safety net** — auto-close external PRs with a friendly redirect
2. **GitHub Issue Form** — structured YAML issue template as the lowest-friction fallback
3. **Agent skill** — `contribute` skill that agents can load and run programmatically
4. **`syllago contribute` CLI command** — interactive Go command with TUI flow

---

## 1. GitHub Action Safety Net

**File:** `.github/workflows/close-external-prs.yml`

- Triggers on `pull_request` events (opened)
- Checks if PR author is NOT in an allowlist (maintainers: `holdenhewett`, `OpenScribblerOwner`)
- Also allows PRs from bots/actions (for Dependabot, Claude Code Action, etc.)
- Auto-comments with a friendly message explaining the contribution model:
  - Points to `syllago contribute` CLI command
  - Points to the GitHub Issue Form as fallback
  - Explains the "ideas not code" philosophy briefly
  - Keeps tone warm, not dismissive
- Closes the PR
- **No AI processing of PR content** — purely mechanical

---

## 2. GitHub Issue Form

**File:** `.github/ISSUE_TEMPLATE/contribution.yml`

Structured YAML issue form with these fields:

1. **Contribution Type** — dropdown: Bug Report, Feature Idea, Improvement, Content Request
2. **Problem or Opportunity** — required textarea: "What's the problem or opportunity?"
3. **Who's Affected** — required text: "Who does this affect and how?"
4. **Suggested Approach** — optional textarea with helper text: "You don't need a solution — just the problem is valuable"
5. **Scope** — optional checkboxes: CLI, TUI, Skills, Agents, Prompts, Rules, Hooks, Commands, MCP, Documentation, Other
6. **Additional Context** — optional textarea for links, screenshots, related issues

Also create `.github/ISSUE_TEMPLATE/config.yml` to add a link to the `syllago contribute` CLI command guidance (and disable blank issues, directing people to the form).

---

## 3. Agent Skill — `contribute`

**Directory:** `skills/contribute/`

Files:
- `SKILL.md` — frontmatter + instructions for agents to run the contribution flow
- `.syllago.yaml` — metadata

The skill will:
- Instruct the agent to collect the same structured information as the CLI/issue form
- Provide the question flow as a structured prompt
- Define the output format (GitHub issue body in markdown)
- Include instructions to use `gh issue create` if available, or output the formatted body
- Reference the OpenScribbler/syllago repo specifically

This is the primary path for AI agents contributing to the project.

---

## 4. `syllago contribute` CLI Command

**File:** `cli/cmd/syllago/contribute.go`

A new cobra command `syllago contribute` that:

### Interactive Flow (when stdin is a TTY)
Uses simple stdin prompts (consistent with existing `init` command patterns — no bubbletea TUI for this flow):

1. **Select contribution type** — numbered menu: Bug Report, Feature Idea, Improvement, Content Request
2. **Problem/opportunity** — multi-line text input (end with blank line)
3. **Who's affected** — single line text
4. **Suggested approach** — optional, can skip with Enter
5. **Scope** — numbered multi-select from: CLI, TUI, Skills, Agents, Prompts, Rules, Hooks, Commands, MCP, Docs
6. **Additional context** — optional
7. **Review** — show the formatted output, confirm submission

### Non-Interactive Mode (--json input or piped stdin)
Accept JSON on stdin with the same fields for programmatic use.

### Output
- Try `gh issue create` first (check if `gh` is available and authenticated)
- Fall back to printing the formatted issue body to stdout
- Support `--json` flag to output structured JSON
- Support `--dry-run` flag to just print without submitting

### Flags
- `--type` — pre-select contribution type
- `--dry-run` — generate output without submitting
- `--json` — JSON output mode
- `--repo` — target repo (defaults to OpenScribbler/syllago)

---

## Implementation Order

1. GitHub Action (simplest, immediate value)
2. GitHub Issue Form (pairs with the action)
3. Agent skill (markdown, no Go code)
4. CLI command (Go implementation, most complex)

---

## Files Changed/Created

| File | Action |
|------|--------|
| `.github/workflows/close-external-prs.yml` | Create |
| `.github/ISSUE_TEMPLATE/contribution.yml` | Create |
| `.github/ISSUE_TEMPLATE/config.yml` | Create |
| `skills/contribute/SKILL.md` | Create |
| `skills/contribute/.syllago.yaml` | Create |
| `cli/cmd/syllago/contribute.go` | Create |
| `cli/cmd/syllago/contribute_test.go` | Create |

No existing files are modified.
