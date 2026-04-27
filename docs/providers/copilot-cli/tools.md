# GitHub Copilot CLI Built-in Tools Reference

> Reference for all tools available to the GitHub Copilot CLI terminal agent.
> Last updated: 2026-03-21. Based on Copilot CLI GA (Feb 2026).

## Sources

All claims are tagged with attribution:

- `[Official]` -- from GitHub docs or official repo
- `[Community]` -- from community discussions, issues, or third-party references
- `[Inferred]` -- derived from observed behavior or cross-referencing multiple sources

Primary sources:
- [About GitHub Copilot CLI](https://docs.github.com/copilot/concepts/agents/about-copilot-cli) `[Official]`
- [Configure Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/set-up-copilot-cli/configure-copilot-cli) `[Official]`
- [Comparing CLI customization features](https://docs.github.com/en/copilot/concepts/agents/copilot-cli/comparing-cli-features) `[Official]`
- [Copilot CLI changelog](https://github.blog/changelog/2026-01-14-github-copilot-cli-enhanced-agents-context-management-and-new-ways-to-install/) `[Official]`
- [Copilot CLI GA announcement](https://github.blog/changelog/2026-02-25-github-copilot-cli-is-now-generally-available/) `[Official]`
- [Issue #1482: tool name mapping](https://github.com/github/copilot-cli/issues/1482) `[Community]`
- [Issue #407: /tools slash command request](https://github.com/github/copilot-cli/issues/407) `[Community]`
- [ggprompts reference guide](https://ggprompts.github.io/htmlstyleguides/techguides/copilot-cli.html) `[Community]`
- [htekdev reference](https://htekdev.github.io/copilot-cli-reference/) `[Community]`

---

## Important Caveat

GitHub has not published a comprehensive built-in tools reference for Copilot CLI.
Unlike Claude Code (which documents all 28+ tools), Copilot CLI's tool inventory is
pieced together from configuration docs, community issues, and observed behavior.
A `/tools` slash command has been [requested](https://github.com/github/copilot-cli/issues/407)
but not yet shipped. The list below is the best reconstruction available as of March 2026.

---

## Quick Reference Table

| Tool | Permission Category | Purpose | Claude Code Equivalent |
|------|:-------------------:|---------|----------------------|
| `bash` | `shell` | Execute shell commands | `Bash` |
| `view` | `read` | Read file contents with line numbers | `Read` |
| `edit` | `write` | String replacements in existing files | `Edit` |
| `create` | `write` | Create new files | `Write` |
| `glob` | `read` | Pattern-based file discovery | `Glob` |
| `grep` | `read` | Search file contents (ripgrep) | `Grep` |
| `task` | (none) | Delegate work to a subagent | `Agent` |
| `skill` | (none) | Invoke a skill file | (no equivalent) |
| `web_fetch` | `url` | Fetch URL content as markdown | `WebFetch` |
| `web_search` | `url` | Search the web | `WebSearch` |
| `write_bash` | `shell` | Send input to running bash session | (no equivalent) |
| `read_bash` | `shell` | Read output from async bash command | (no equivalent) |
| `stop_bash` | `shell` | Stop a running bash session | (no equivalent) |

---

## Tool Details

### bash

Execute shell commands synchronously, asynchronously, or in detached mode. `[Community]`

- **Permission pattern:** `shell`, `shell(COMMAND)`, `shell(COMMAND:*)`
- **Key parameters:** command to execute, mode (sync/async/detached)
- **Notes:** Path permissions also apply to shell commands. The `:*` suffix matches the command stem followed by a space, preventing partial matches (e.g., `shell(git:*)` matches `git push` but not `gitea`). `[Official]`
- **Claude Code equivalent:** `Bash`

### view

View files and directories with line numbers. `[Community]`

- **Permission pattern:** Falls under `read` permission category
- **Key parameters:** file path, optional line range
- **Notes:** Appears in `events.jsonl` as tool name `view`. Users initially confused it with a shell subcommand; it is a distinct built-in tool. `[Community]`
- **Claude Code equivalent:** `Read`

### edit

Make string replacements in existing files. `[Community]`

- **Permission pattern:** Falls under `write` permission category
- **Key parameters:** file path, old string, new string
- **Notes:** Also referenced as `apply_patch` in some contexts. The syllago toolmap uses `apply_patch` as the canonical name; `edit` appears in the comparing-features doc. These may be the same tool under different names, or `apply_patch` may be the internal wire name while `edit` is the user-facing alias. `[Inferred]`
- **Claude Code equivalent:** `Edit`

### create

Create new files with specified content. `[Community]`

- **Permission pattern:** Falls under `write` permission category
- **Key parameters:** file path, content
- **Notes:** Distinct from `edit` -- used for files that don't exist yet. `[Community]`
- **Claude Code equivalent:** `Write`

### glob

Pattern-based file discovery. `[Community]`

- **Permission pattern:** Falls under `read` permission category
- **Key parameters:** glob pattern (e.g., `**/*.ts`)
- **Notes:** Appears in `events.jsonl` as tool name `glob`. Not a shell subcommand. `[Community]`
- **Claude Code equivalent:** `Glob`

### grep

Search file contents using ripgrep-style matching. `[Community]`

- **Permission pattern:** Falls under `read` permission category
- **Key parameters:** search pattern, optional path filter
- **Notes:** Also referenced as `rg` in some contexts (the syllago toolmap uses `rg`). Appears in `events.jsonl` as `grep`. The underlying implementation likely uses ripgrep. `[Community]`, `[Inferred]`
- **Claude Code equivalent:** `Grep`

### task

Delegate work to a subagent. `[Official]`

- **Permission pattern:** Not gated by shell/write/read permissions
- **Key parameters:** prompt/instruction for the subagent
- **Notes:** Copilot creates specialized subagents (explore, task, code-review, plan) and can run multiple in parallel. The `task` tool is how delegation happens programmatically. `[Official]`
- **Claude Code equivalent:** `Agent`

### skill

Invoke a markdown-based skill file. `[Official]`

- **Permission pattern:** Not gated by shell/write/read permissions
- **Key parameters:** skill name/path
- **Notes:** Skills are `.skill.md` files that load automatically when relevant. They work across Copilot CLI, Copilot coding agent, and VS Code. `[Official]`
- **Claude Code equivalent:** No direct equivalent. Claude Code uses `Skill` but it works differently (internal prompt enhancement, not a callable tool in the same sense).

### web_fetch

Fetch content from a URL and return it as markdown. `[Official]`

- **Permission pattern:** `url`, `url(DOMAIN)`, `url(FULL_URL)`
- **Key parameters:** URL to fetch
- **Notes:** URL access is controlled via `allowed_urls` and `denied_urls` in `~/.copilot/config`. Added in the Jan 2026 update. `[Official]`
- **Claude Code equivalent:** `WebFetch`

### web_search

Search the web for information. `[Inferred]`

- **Permission pattern:** Falls under `url` permission category
- **Key parameters:** search query
- **Notes:** Referenced alongside `web_fetch` as a built-in web tool. Less documented than `web_fetch`. `[Inferred]`
- **Claude Code equivalent:** `WebSearch`

### write_bash

Send input to a running interactive bash session. `[Community]`

- **Permission pattern:** Falls under `shell` permission category
- **Key parameters:** session ID, input text
- **Notes:** Part of the async bash session management trio (`write_bash`, `read_bash`, `stop_bash`). Enables interactive terminal workflows. `[Community]`
- **Claude Code equivalent:** No direct equivalent (Claude Code's Bash is synchronous only)

### read_bash

Read output from an async or detached bash command. `[Community]`

- **Permission pattern:** Falls under `shell` permission category
- **Key parameters:** session ID
- **Notes:** Pairs with `write_bash` for interactive session management. `[Community]`
- **Claude Code equivalent:** No direct equivalent

### stop_bash

Stop a running bash session. `[Community]`

- **Permission pattern:** Falls under `shell` permission category
- **Key parameters:** session ID
- **Notes:** Terminates an async/detached bash session. `[Community]`
- **Claude Code equivalent:** No direct equivalent

---

## Permission System

Copilot CLI uses a layered permission model with four tool categories: `[Official]`

| Category | Controls | Pattern syntax |
|----------|----------|---------------|
| `shell` | bash, write_bash, read_bash, stop_bash | `shell`, `shell(git:*)`, `shell(npm run test:*)` |
| `write` | edit, create | `write` |
| `read` | view, glob, grep | (not separately configurable as of GA) |
| `url` | web_fetch, web_search | `url`, `url(github.com)` |

**Flags:**
- `--allow-tool='PATTERN'` -- allow a specific tool/pattern
- `--deny-tool='PATTERN'` -- deny a specific tool/pattern (always wins)
- `--allow-all-tools` -- auto-approve all tool usage
- `--allow-all` / `--yolo` -- equivalent to `--allow-all-tools --allow-all-paths --allow-all-urls`
- `--available-tools` -- restrict to only named tools

Deny rules always take precedence over allow rules.

---

## Built-in Agents (Not Tools)

These are specialized agents, not tools -- but they use tools internally: `[Official]`

| Agent | Purpose |
|-------|---------|
| `explore` | Fast codebase analysis |
| `task` | Build and test execution |
| `code-review` | Change review |
| `plan` | Implementation planning |
| `research` | Web research |

Multiple agents can run in parallel.

---

## Unconfirmed / Absent Tools

Tools that exist in other providers but have no confirmed Copilot CLI equivalent:

| Other Provider Tool | Status in Copilot CLI |
|----|------|
| `think` (Claude Code) | Not confirmed. Copilot has Ctrl+T to toggle reasoning visibility, but no named tool. |
| `TodoRead`/`TodoWrite` (Claude Code) | Not confirmed. |
| `NotebookEdit` (Claude Code) | Not confirmed. |
| `EnterWorktree`/`ExitWorktree` (Claude Code) | Not confirmed. |
| `LSP` tools | Copilot CLI supports LSP for code intelligence, but it's unclear if this surfaces as a callable tool. `[Inferred]` |

---

## Syllago Toolmap Alignment

The syllago toolmap (`cli/internal/converter/toolmap.go`) currently uses these Copilot CLI tool names:

| Syllago canonical | Copilot CLI name in toolmap |
|---|---|
| Read | `view` |
| Write/Edit | `apply_patch` |
| Bash | `shell` |
| Glob | `glob` |
| Grep | `rg` |
| Agent | `task` |

**Potential discrepancies to investigate:**
- `apply_patch` vs `edit` vs `create` -- the toolmap maps Write/Edit to `apply_patch`, but official docs reference `edit` and `create` as separate tools. `apply_patch` may be the internal/wire name.
- `rg` vs `grep` -- the toolmap uses `rg` but the tool appears in events as `grep`. May be aliases.
