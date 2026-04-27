# Codex CLI: Hooks & Lifecycle

## Status

Codex CLI's hooks system is **experimental** as of v0.114.0 (2026-03-11). It must be
enabled via a feature flag and has limited event coverage compared to Claude Code's
17 lifecycle points.

**Feature flag to enable:** `[Official]`
```
codex -c features.codex_hooks=true
```

Or in `~/.codex/config.toml`:
```toml
[features]
codex_hooks = true
```

Source: [Discussion #2150](https://github.com/openai/codex/discussions/2150),
[PR #13276](https://github.com/openai/codex/pull/13276)

---

## Supported Hook Events

| Event | Added In | Description |
|-------|----------|-------------|
| `SessionStart` | v0.114.0 (2026-03-11) | Fires when a session begins. Output is appended as additional context to the model. |
| `Stop` | v0.114.0 (2026-03-11) | Fires when a turn ends. Can block responses with a required `reason`. |
| `UserPromptSubmit` | v0.116.0 (2026-03-19) | Fires before a prompt is executed or enters history. Can block or augment prompts. |

`[Official]` Sources:
- [Changelog](https://developers.openai.com/codex/changelog)
- [Release v0.114.0](https://github.com/openai/codex/releases/tag/rust-v0.114.0)

---

## Configuration Format

Hooks are configured in a `hooks.json` file located in:
- `.codex/hooks.json` (project-scoped)
- `~/.codex/hooks.json` (user-scoped)

`[Official]` Based on PR #13276 and Discussion #2150.

### Schema

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo 'session started' >> /tmp/codex.log",
            "statusMessage": "Initializing session...",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo 'turn ended' >> /tmp/codex.log",
            "statusMessage": "Wrapping up turn...",
            "timeout": 10
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "my-prompt-filter",
            "statusMessage": "Checking prompt...",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

### Hook Object Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Hook type. Currently only `"command"` is supported. |
| `command` | string | Yes | Shell command to execute. |
| `statusMessage` | string | No | User-facing message displayed during execution. |
| `timeout` | number | No | Maximum execution time in seconds. |

`[Official]` Source: [PR #13276](https://github.com/openai/codex/pull/13276)

---

## Execution Model

- **Blocking:** Hook execution is synchronous and blocks turn progression while running. `[Official]`
- **Parallel within group:** Multiple matching hooks run in parallel; results are aggregated into a `HookRunSummary`. `[Official]`
- **Ephemeral:** Hook messages are not persisted to transcript. Context changes (e.g., SessionStart output) get appended to the user's prompt. `[Official]`

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Malformed output |

### Event-Specific Behavior

**SessionStart:**
- Output plain text as additional context, OR return JSON with structured fields.
- Malformed JSON is treated as plain text (graceful fallback).
- Supports a `suppress_output` field (deferred, not yet implemented).

**Stop:**
- Must include a non-empty `reason` when blocking a response.
- Cannot silently ignore JSON parsing failures.

**UserPromptSubmit:**
- Can block prompts (prevent execution) or augment them (modify before execution).
- Fires before the prompt enters history.

`[Official]` Source: [PR #13276](https://github.com/openai/codex/pull/13276)

---

## Protocol Integration (App Server)

Hooks are exposed as operational metadata rather than transcript items:
- **Live notifications:** `hook/started`, `hook/completed`
- **Persisted data:** `Turn.hookRuns` contains replayed hook results

`[Official]` Source: [PR #13276](https://github.com/openai/codex/pull/13276)

---

## Notifications (Separate from Hooks)

Codex has a simpler notification mechanism in `config.toml` that triggers an external
program on `agent-turn-complete`. This is not the hooks system but provides a
lightweight callback:

```toml
notify = ["bash", "-lc", "afplay /System/Library/Sounds/Blow.aiff"]
```

`[Official]` Source: [Advanced Configuration](https://developers.openai.com/codex/config-advanced)

---

## What Codex Does NOT Have (vs. Claude Code)

Claude Code provides 17 hook lifecycle points with fine-grained control. Codex currently
has 3 experimental events. Notable gaps:

| Claude Code Hook | Codex Equivalent |
|------------------|------------------|
| `PreToolUse` / `PostToolUse` | Not supported |
| `Notification` | `notify` config (simpler) |
| `Stop` | `Stop` (similar) |
| `SubagentStop` | Not supported |

`[Inferred]` Based on comparison of documented features.

---

## Community Alternatives

### codex-hooks (Third Party)

[codex-hooks](https://github.com/hatayama/codex-hooks) bridges the gap by providing:
- Events: `TaskStarted`, `TaskComplete`, `TurnAborted`
- Loads hooks from `~/.codex/hooks.json` (falls back to `~/.claude/settings.json`)
- Executes commands via `/bin/sh -lc`
- Passes JSON on stdin: `hook_event_name`, `transcript_path`, `cwd`, `session_id`, `raw_event`

`[Community]` Source: [github.com/hatayama/codex-hooks](https://github.com/hatayama/codex-hooks)

---

## Syllago Implications

- Codex hooks use JSON (`hooks.json`), not TOML — unlike the rest of Codex config.
- The schema structure nests differently from Claude Code's hooks in `settings.json`.
- Only 3 events exist; mapping from Claude Code's 17 events will be lossy.
- Feature is experimental and may change significantly.
