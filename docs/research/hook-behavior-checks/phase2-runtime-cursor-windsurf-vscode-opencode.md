# Hook Runtime Behavior: Cursor, Windsurf, VS Code Copilot, OpenCode

Research date: 2026-03-31
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| Cursor | cursor.com/docs/hooks, cursor.com/docs/reference/third-party-hooks, blog.gitbutler.com/cursor-hooks-deep-dive |
| Windsurf | docs.windsurf.com/windsurf/cascade/hooks |
| VS Code Copilot | code.visualstudio.com/docs/copilot/customization/hooks, github.com/microsoft/vscode-docs |
| OpenCode | opencode.ai/docs/plugins/, github.com/sst/opencode — packages/plugin/src/index.ts, packages/opencode/src/shell/shell.ts |

---

## D1: Hook Discovery Mechanism

| Agent | Config File(s) | Scope Levels | Auto-Discovery |
|-------|---------------|-------------|----------------|
| Cursor | `hooks.json` at 4 locations | Enterprise > Team > Project (`.cursor/hooks.json`) > User (`~/.cursor/hooks.json`) | No — explicit registration. Also reads CC hooks via "Third-party hooks" compat mode (must be enabled). |
| Windsurf | `hooks.json` at 3 locations | System > User (`~/.codeium/windsurf/hooks.json`) > Workspace (`.windsurf/hooks.json`) | No — explicit registration |
| VS Code Copilot | `.github/hooks/*.json`, `~/.copilot/hooks` | Workspace > User | Partial — `.github/hooks/*.json` glob. Customizable via `chat.hookFilesLocations` setting. |
| OpenCode | `opencode.json` `"plugin"` array + `.opencode/plugins/` dir | Internal > Global config > Project config > Global file dir > Project file dir | Yes — files in plugins dir auto-load |

### Notable Details

**Cursor third-party hook compatibility:** Cursor can read Claude Code hooks from `.claude/settings.json` and `.claude/settings.local.json` when "Third-party hooks" is enabled in Cursor Settings. These are merged as lowest-priority tier.

**Windsurf system-level paths:**
- macOS: `/Library/Application Support/Windsurf/hooks.json`
- Linux/WSL: `/etc/windsurf/hooks.json`
- Windows: `C:\ProgramData\Windsurf\hooks.json`
- JetBrains plugin: `~/.codeium/hooks.json` (different from IDE path)

**OpenCode plugin loading:** npm plugins installed automatically via Bun to `~/.cache/opencode/node_modules/`. Local file plugins loaded via `import()`.

---

## D2: Hook Execution Environment

### Environment Variables

| Agent | Injected Env Vars |
|-------|------------------|
| Cursor | `CURSOR_PROJECT_DIR`, `CURSOR_VERSION`, `CURSOR_USER_EMAIL` (if logged in), `CURSOR_TRANSCRIPT_PATH` (if transcripts enabled), `CURSOR_CODE_REMOTE` (remote only), `CLAUDE_PROJECT_DIR` (CC compat) |
| Windsurf | `ROOT_WORKSPACE_PATH` (only in `post_setup_worktree` hooks). Others: unknown. |
| VS Code Copilot | Inherited process env + per-hook `env` property. Auto-injected vars: unknown. |
| OpenCode | N/A (in-process). Plugin receives `PluginInput` object: `client`, `project`, `directory`, `worktree`, `serverUrl`, `$` (Bun shell) |

### stdin / Input Format

| Agent | Format | Common Fields |
|-------|--------|--------------|
| Cursor | JSON on stdin | `conversation_id`, `generation_id`, `prompt`, `attachments`, `workspace_roots` + event-specific |
| Windsurf | JSON on stdin | `agent_action_name`, `trajectory_id`, `execution_id`, `timestamp`, `tool_info` |
| VS Code Copilot | JSON on stdin | `timestamp`, `cwd`, `sessionId`, `hookEventName`, `transcript_path` |
| OpenCode | Function arguments (TypeScript) | `input: InputType, output: OutputType` — varies per hook |

### CWD and Shell

| Agent | Hook Process CWD | Shell |
|-------|-----------------|-------|
| Cursor | Per-tier: project hooks = project root, user hooks = `~/.cursor/`, enterprise = enterprise dir | Unknown ("spawned processes", shell not specified) |
| Windsurf | Workspace root (configurable via `working_directory`). **`~` expansion NOT supported.** | Any executable (Python, Bash, Node, etc.) |
| VS Code Copilot | Repo root (configurable via `cwd` relative to repo root) | System default shell, same permissions as VS Code process |
| OpenCode | N/A (in-process) | N/A. For shell commands: `$SHELL` → fallback: macOS=zsh, Linux=bash/sh, Windows=pwsh. Fish/nu blacklisted. |

### Notable Details

**Cursor session-scoped env vars:** `sessionStart` hooks can inject environment variables that propagate to ALL subsequent hooks in the session. Useful for auth tokens or session IDs.

**Windsurf `working_directory` gotcha:** Home directory `~` shorthand is explicitly unsupported. Must use absolute paths or workspace-relative paths.

**Windsurf `show_output`:** Boolean per-hook config. Controls whether stdout/stderr shown in Cascade UI. Does NOT apply to `pre_user_prompt` or `post_cascade_response*` hooks.

---

## D3: Hook Ordering

| Agent | Default Order | Configurable? | Output Chaining? |
|-------|-------------|---------------|------------------|
| Cursor | Array order within each tier; tiers: Enterprise > Team > Project > User > CC-compat | No priority field | Unknown |
| Windsurf | System → User → Workspace; array order within tier | No | Unknown |
| VS Code Copilot | Workspace before User (workspace takes precedence). Intra-tier: unknown. | No | No (each hook returns independent JSON) |
| OpenCode | Plugin registration order (load order): internal → global config → project config → file-based | No | **Yes** — shared `output` object mutated in-place. Hook N sees mutations from hooks 1..N-1 |

### Notable Details

**OpenCode output chaining confirmed from source:**
```typescript
for (const hook of state.hooks) {
  const fn = hook[name] as any
  if (!fn) continue
  yield* Effect.promise(async () => fn(input, output))
}
return output
```
The shared `output` object is passed by reference to all hooks sequentially. This is the only agent where hook chaining is architecturally guaranteed.

---

## D4: Concurrent Hook Execution

| Agent | Same-Event Concurrency | Different-Event Concurrency |
|-------|----------------------|---------------------------|
| Cursor | Unknown (docs say all hooks execute, don't specify parallel/sequential) | Unknown |
| Windsurf | Implied sequential (tier ordering implies sequential) | Unknown |
| VS Code Copilot | Sequential (explicitly stated) | Unknown |
| OpenCode | Sequential (confirmed from `for` loop in source; comment: "Keep plugin execution sequential") | Bus event subscriptions via `Stream.runForEach` — also sequential |

---

## D5: Error Propagation

| Agent | Exit 0 | Exit 2 | Other Non-Zero | Crash Behavior |
|-------|--------|--------|---------------|----------------|
| Cursor | Success, use JSON | Block (= `permission: "deny"`) | Fail-open (proceed, log) | Fail-open default; `failClosed: true` option inverts |
| Windsurf | Success, proceed | Block (pre-hooks only), stderr in UI | Log, proceed | Unknown |
| VS Code Copilot | Parse JSON | Block, show to model | Non-blocking warning | Unknown |
| OpenCode | N/A (in-process) | N/A | N/A | Load errors: skip + publish error. Trigger errors: exception propagates (no catch in `trigger()`) |

### Notable Details

**Cursor `failClosed`:** Configuration option that inverts default fail-open behavior. When set to `true`, hook failures (timeouts, crashes) cause the action to be BLOCKED rather than proceeding. This is unique to Cursor.

**OpenCode trigger errors:** There is NO try/catch around per-hook calls in `trigger()`. If a hook throws, it propagates up. Config hooks and event hooks (bus subscriptions) ARE caught with `Effect.ignore`.

---

## D6: Timeout Implementation

| Agent | Default Timeout | Config Field | Signal on Timeout | Behavior |
|-------|----------------|-------------|-------------------|----------|
| Cursor | Unknown ("platform default") | `timeout` (seconds) | Unknown | Fail-open; `failClosed: true` inverts |
| Windsurf | Unknown (not documented) | Unknown | Unknown | Unknown |
| VS Code Copilot | **30 seconds** | `timeout` (seconds) | Unknown | Hook considered failed |
| OpenCode | **No timeout** for plugin hooks | N/A | N/A | Blocking/infinite hook blocks the entire Effect fiber |

### Notable Details

**OpenCode has NO timeout mechanism for plugin hooks.** Since plugins run in-process as async functions, a blocking hook will block indefinitely. The shell kill sequence (SIGTERM → 200ms → SIGKILL) only applies to PTY shell processes, not plugin hooks.

**OpenCode shell kill sequence (for PTY, not hooks):**
1. `process.kill(-pid, "SIGTERM")` — entire process group
2. Wait 200ms
3. `process.kill(-pid, "SIGKILL")`
4. Windows: `taskkill /pid <pid> /f /t`

**Windsurf runtime behavior is the least documented** of all 8 agents — no timeout docs, incomplete env var list, no concurrency model, no crash handling docs.
