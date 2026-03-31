# Hook Runtime Behavior: Claude Code, Copilot CLI, Gemini CLI, Kiro

Research date: 2026-03-31
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| Claude Code | https://code.claude.com/docs/en/hooks |
| Copilot CLI | https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks, https://docs.github.com/en/copilot/reference/hooks-configuration |
| Gemini CLI | Source: packages/core/src/hooks/hookRunner.ts, hookRegistry.ts, hookPlanner.ts, hookAggregator.ts, types.ts; docs/hooks/reference.md |
| Kiro | https://kiro.dev/docs/hooks/, https://kiro.dev/docs/cli/hooks/ |

---

## D1: Hook Discovery Mechanism

| Agent | Config File(s) | Scope Levels | Auto-Discovery |
|-------|---------------|-------------|----------------|
| Claude Code | `~/.claude/settings.json`, `.claude/settings.json`, `.claude/settings.local.json`, plugin `hooks/hooks.json`, skill/agent frontmatter | Managed Policy > User > Project > Local > Plugin > Skill/Agent | No |
| Copilot CLI | `.github/hooks/*.json` | Project only | No |
| Gemini CLI | `~/.gemini/settings.json`, `.gemini/settings.json`, system path, extensions | Runtime(0) > Project(1) > User(2) > System(3) > Extensions(4) | No |
| Kiro | IDE-managed agent config | Unknown (IDE-managed) | No |

### Notable Details

**Claude Code:** Managed policy settings (org-wide, admin-controlled) have highest priority. Plugin hooks are bundled with plugins. Skill/agent frontmatter hooks are active while that component is loaded.

**Copilot CLI:** Requires `version: 1` in config files. Project-scope only — no user or system scope.

**Gemini CLI:** Project hooks are blocked if the folder is not in the trusted folders list (security feature from `hookRegistry.ts`). Priority is numeric: lower number = higher priority.

---

## D2: Hook Execution Environment

### Environment Variables

| Agent | Injected Env Vars |
|-------|------------------|
| Claude Code | `CLAUDE_PROJECT_DIR`, `CLAUDE_PLUGIN_ROOT` (plugins), `CLAUDE_PLUGIN_DATA` (plugins), `CLAUDE_ENV_FILE` (SessionStart/CwdChanged/FileChanged only), `CLAUDE_CODE_REMOTE` (remote only) |
| Copilot CLI | User-specified via `env` field only; no documented auto-injected vars |
| Gemini CLI | `GEMINI_PROJECT_DIR`, `CLAUDE_PROJECT_DIR` (compat alias!), filtered parent env via sanitization allowlist/blocklist, user-specified per-hook `env` |
| Kiro | Not documented |

### stdin JSON (Common Fields)

| Agent | Common Fields |
|-------|--------------|
| Claude Code | `session_id`, `transcript_path`, `cwd`, `permission_mode`, `hook_event_name` + subagent fields (`agent_id`, `agent_type`) when applicable |
| Copilot CLI | `timestamp`, `cwd` + event-specific fields (`toolName`, `toolArgs`, `prompt`, etc.) |
| Gemini CLI | `session_id`, `transcript_path`, `cwd`, `hook_event_name`, `timestamp` + event-specific fields |
| Kiro | `hook_event_name`, `cwd` + event-specific tool fields |

### CWD and Shell

| Agent | Hook Process CWD | Shell |
|-------|-----------------|-------|
| Claude Code | Current agent CWD at fire time | `bash` (default), configurable via `shell` field to `powershell` |
| Copilot CLI | Configurable via `cwd` (default: repo root) | `bash` (Unix) / `powershell` (Windows) |
| Gemini CLI | `input.cwd` (agent's current dir), set explicitly in spawn() | `bash -c` (Unix), `powershell -NoProfile -Command` (Windows), `shell: false` in spawn |
| Kiro | Unknown | Unknown |

### Notable Details

**Gemini CLI environment sanitization:** Uses allowlist/blocklist system. Always-allowed: `PATH`, `HOME`, `SHELL`, `USER`, `TERM`, `LANG`, `TMPDIR`, etc. Never-allowed (stripped): `CLIENT_ID`, `DB_URI`, `AWS_DEFAULT_REGION`, `AZURE_CLIENT_ID`, and similar credential-adjacent vars.

**Gemini CLI CLAUDE_PROJECT_DIR alias:** Source explicitly sets `CLAUDE_PROJECT_DIR: input.cwd` alongside `GEMINI_PROJECT_DIR: input.cwd` for compatibility with hooks written for Claude Code.

**Copilot CLI toolArgs:** Passed as JSON string (not object) — requires parsing in hook scripts.

---

## D3: Hook Ordering

| Agent | Default Order | Configurable? | Output Chaining? |
|-------|-------------|---------------|------------------|
| Claude Code | **Parallel** (all fire simultaneously) | `async: true` for background; no sequential option | No (parallel = independent) |
| Copilot CLI | Sequential, array order | No — always sequential | Unknown |
| Gemini CLI | **Parallel** by default; sequential if any definition sets `sequential: true` | Yes, per hook definition | Yes, in sequential mode — `applyHookOutputToInput()` threads `additionalContext`, `llm_request`, `tool_input` between hooks |
| Kiro | Unknown | Unknown | Unknown |

### Notable Details

**Claude Code parallel execution:** Identical handlers are deduplicated by command string (command hooks) or URL (HTTP hooks). No priority field or ordering guarantee.

**Gemini CLI sequential mode:** From `hookPlanner.ts`: if ANY matching hook definition has `sequential: true`, ALL hooks for that event run sequentially. Earlier hook output can modify later hook input for specific fields.

**Copilot CLI:** "Multiple hooks of the same type execute sequentially in defined configuration order." Array index = sequence.

---

## D4: Concurrent Hook Execution

| Agent | Same-Event Concurrency | Different-Event Concurrency | Blocking Model |
|-------|----------------------|---------------------------|----------------|
| Claude Code | Parallel (default), optionally background async | Sequential by event lifecycle | Blocking events must complete before guarded action |
| Copilot CLI | Sequential only | Unknown (likely sequential given sync model) | All hooks synchronous and blocking |
| Gemini CLI | Parallel (default) or sequential (`sequential: true`) | Sequential by event lifecycle | Blocking events must complete before guarded action |
| Kiro | Unknown | Unknown | Unknown |

---

## D5: Error Propagation

| Agent | Exit 0 | Exit 2 | Other Non-Zero | Crash/Spawn Failure | Subsequent Hooks After Crash |
|-------|--------|--------|---------------|--------------------|-----------------------------|
| Claude Code | Success, parse JSON | Block (event-dependent) | Non-blocking, stderr in verbose | Non-blocking, stderr in verbose | Unaffected (parallel) |
| Copilot CLI | Success | Unknown | Unknown | Unknown | Unknown |
| Gemini CLI | Success, parse JSON/text | Block (any ≥2 blocks) | Exit 1 = warning allow; ≥2 = deny | Non-fatal (`debugLogger.warn`), continues | Parallel: unaffected. Sequential: input passes through unchanged |
| Kiro | Success, stdout captured | Block tool (PreToolUse only) | Warning, show stderr | Unknown | Unknown |

### Notable Details

**Gemini CLI exit code semantics:** ANY exit code ≥2 blocks (not just exit code 2 specifically). Exit 1 is explicitly a non-blocking warning that converts to `{ decision: "allow", systemMessage: "Warning: <text>" }`.

**Gemini CLI plain text output handling:** If stdout is not valid JSON, `convertPlainTextToHookOutput()` wraps it as `{ decision: "allow", systemMessage: text }` for exit 0 or `{ decision: "deny", reason: text }` for exit ≥2.

**Claude Code HTTP hooks:** Non-2xx status or connection failure = non-blocking error, execution continues.

---

## D6: Timeout Implementation

| Agent | Default Timeout | Config Field | Signal Sequence | Grace Period |
|-------|----------------|-------------|----------------|-------------|
| Claude Code | **600s** (command), 30s (HTTP/prompt), 60s (agent) | `timeout` (seconds) | Unknown (not documented) | Unknown |
| Copilot CLI | 30s | `timeoutSec` (seconds) | Unknown | Unknown |
| Gemini CLI | 60s | `timeout` (**milliseconds**) | SIGTERM → SIGKILL after 5s; Windows: `taskkill /f /t` | 5 seconds (hardcoded) |
| Kiro | 30s | `timeout_ms` (milliseconds) | Unknown | Unknown |

### Notable Details

**Claude Code 600s default for command hooks** is a major outlier — 10-20x the 30-60s defaults of other agents. This likely reflects its use in long-running CI/policy-check workflows.

**Gemini CLI timeout units are MILLISECONDS**, not seconds. This is a conversion concern for the spec (which uses seconds as canonical).

**Gemini CLI kill sequence from source:**
1. `child.kill('SIGTERM')` — sent immediately at timeout
2. After 5 seconds: `child.kill('SIGKILL')` — forced kill
3. Windows: `taskkill /pid <pid> /f /t` at timeout, then again after 5s
4. Result: `{ success: false, error: "Hook timed out after Nms" }` — non-fatal

**Kiro uses `timeout_ms`** (milliseconds) — matching Gemini CLI's unit choice, not the spec's seconds.
