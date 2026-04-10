# Hook System Security Audit

Research date: 2026-03-31
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| Claude Code | https://code.claude.com/docs/en/hooks |
| Gemini CLI | Source: trustedHooks.ts, trustedFolders.ts, hookRunner.ts, environmentSanitization.ts; docs/cli/sandbox.md |
| Cursor | cursor.com/docs/hooks |
| Windsurf | docs.windsurf.com/windsurf/cascade/hooks |
| VS Code Copilot | code.visualstudio.com/docs/copilot/customization/hooks |
| Copilot CLI | docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks |
| Kiro | kiro.dev/docs/hooks/ |
| OpenCode | opencode.ai/docs/plugins/, github.com/sst/opencode |

---

## E1: Hook Provenance Controls

| Agent | Integrity Verification | First-Run Approval | Trust Tracking | Modification Detection | Untrusted Repo Protection |
|-------|----------------------|-------------------|---------------|----------------------|--------------------------|
| Claude Code | None | None | None (source labels only) | None | None — project hooks auto-execute |
| Gemini CLI | Yes — `name:command` string key concatenation (not cryptographic hash) via TrustedHooksManager | Yes — when folderTrust enabled (**opt-in, off by default**) | Yes — `~/.gemini/trusted_hooks.json` per-project | Yes — command string change invalidates trust (but NOT script content changes) | Yes — project hooks blocked in untrusted folders (when enabled) |
| Cursor | None | Workspace trust (VS Code model) — coarse, all-or-nothing | Persistent workspace trust | None | Partial — workspace trust required but users nudged to grant |
| Windsurf | None | None | None | None | None — explicit warning: "you are entirely responsible" |
| VS Code Copilot | None | None | None | None | None — advisory only |
| Copilot CLI | None | None | None | None | Ephemeral runner mitigates blast radius |
| Kiro | None | None | None | None | Unknown — no security docs |
| OpenCode | None | N/A (no hook-scripts-from-repos model) | Tool permission rules (ask/allow/deny) | N/A | Structurally safer — no project hook injection |

### Key Finding: Gemini CLI Is the Only Agent with Real Provenance Controls

Gemini CLI's `TrustedHooksManager` provides:
- Unique key per hook based on `name:command`
- Command changes invalidate trust
- Trust persisted per-project in `~/.gemini/trusted_hooks.json`
- Project hooks blocked in untrusted folders via `hookRunner.ts`

**Critical caveat:** This is disabled by default (`security.folderTrust.enabled: false`). Most users get no protection.

---

## E2: Hook Sandboxing

| Agent | OS-Level Sandbox | Network Access | Filesystem Access | Process Spawning | Env Var Protection |
|-------|-----------------|---------------|-------------------|-----------------|-------------------|
| Claude Code | None | Unrestricted (command hooks) | Unrestricted | Unrestricted | HTTP hooks: `allowedEnvVars` allowlist. Command hooks: full `process.env` |
| Gemini CLI | Yes — 5 methods: macOS Seatbelt, Docker/Podman, Windows Sandbox, gVisor, LXC/LXD | Unrestricted (hooks) | Constrained when sandbox active | Via spawn(), sandbox-dependent | `sanitizeEnvironment()` with deny-list (disabled locally by default, auto-enabled in CI) |
| Cursor | None (hook scripts not sandboxed; agent has network restrictions) | Hook scripts: unrestricted. Agent: restricted to GitHub, search providers | Unrestricted | Unrestricted | Full process env |
| Windsurf | None | Unrestricted | Unrestricted | Unrestricted | Full process env |
| VS Code Copilot | None | Unrestricted | Unrestricted | Unrestricted | Full process env |
| Copilot CLI | Implicit — ephemeral Actions runner | Unrestricted within runner | Ephemeral workspace | Unrestricted within runner | Actions secrets if configured |
| Kiro | None documented | Unknown | Unknown | Unknown | Unknown |
| OpenCode | None (in-process) | Controlled via permission rules | Controlled via permission rules | `bash` permission required | No isolation (same process) |

### Gemini CLI Environment Variable Protection

`NEVER_ALLOWED_ENVIRONMENT_VARIABLES` deny-list includes: `CLIENT_ID`, `DB_URI`, `CONNECTION_STRING`, `AWS_DEFAULT_REGION`, `AZURE_CLIENT_ID`, and similar credential-adjacent vars.

Always-allowed: `PATH`, `HOME`, `SHELL`, `USER`, `TERM`, `LANG`, `TMPDIR`.

---

## E3: Supply Chain Risks

### Auto-Execute Project Hooks Without User Consent

| Agent | Auto-Executes from Cloned Repo | Warning? | Mitigation |
|-------|-------------------------------|---------|------------|
| Claude Code | Yes (`.claude/settings.json`) | No | Enterprise `allowManagedHooksOnly` |
| Gemini CLI | Yes (unless folderTrust enabled) | Only with opt-in folderTrust | Folder trust + modification detection |
| Cursor | Yes (in trusted workspaces) | First-time workspace trust | Workspace trust gate (but users nudged to grant) |
| Windsurf | Yes (`.windsurf/hooks.json`) | No | None |
| VS Code Copilot | Yes (`.github/hooks/*.json`) | No | None |
| Copilot CLI | Yes (`.github/hooks/`) | No | Ephemeral runner |
| Kiro | Unknown | Unknown | None documented |
| OpenCode | N/A | N/A | No hook-script injection model |

**This is the highest-severity gap across the ecosystem.** 6 of 8 agents auto-execute project hooks (CC, Gemini CLI with default settings, Cursor after workspace trust granted, Windsurf, VS Code Copilot, Copilot CLI). Only OpenCode (no hook injection model) and Kiro (unknown) are exempt. Gemini CLI and Cursor have partial mitigations (folder trust opt-in and workspace trust gate respectively) but both default to permissive behavior.

---

## E4: Privilege Escalation

### Secret Access via Environment Variables

All agents except Gemini CLI (with redaction enabled) and OpenCode (permission-gated) pass the full process environment to hook scripts. A malicious hook can trivially read API keys, cloud credentials, and session tokens.

### PostToolUse → Prompt Injection Vector

Every agent allows post-execution hooks to inject content into the model context. A malicious hook can write content that influences subsequent model behavior (tool selection, code generation, permission decisions). **No agent documents this as a threat or implements mitigations.**

### Data Exfiltration

All agents with unrestricted network access in command hooks (Claude Code, Gemini CLI without redaction, Windsurf, VS Code Copilot, Cursor, Kiro) are vulnerable. A hook can `curl` any endpoint with:
- Conversation context from stdin JSON
- API keys from environment variables
- Source code content passed in tool input/output fields

---

## E5: Trust Models

| Agent | Trust Model | Comparable to Pi's Lifecycle? |
|-------|------------|------------------------------|
| Claude Code | None — source labels informational only | No |
| Gemini CLI | `unreviewed → trusted` with modification detection | Closest equivalent, but opt-in |
| Cursor | Workspace trust (coarse, all-or-nothing) | No |
| Windsurf | None — hierarchy for precedence only | No |
| VS Code Copilot | None | No |
| Copilot CLI | Repository access = trust | No |
| Kiro | Unknown | No |
| OpenCode | Tool permission rules (not hook-specific) | Partially — ask/allow/deny is similar |

### No Agent Implements

- Hook code review or static analysis before execution
- Cryptographic signing or verification of hook scripts
- A "killed/revoked" trust state that actively blocks previously trusted hooks
- Tamper-evident audit logs of hook execution

---

## Security Posture Ranking

1. **Gemini CLI** — Only real provenance model (modification detection), multi-method sandbox (gVisor/Seatbelt/Docker), env var redaction deny-list, folder trust system. Weakness: opt-in, disabled by default.

2. **GitHub Copilot Coding Agent** — Ephemeral Actions runners provide run-to-run isolation (no persistent state between runs). Note: this is NOT in-run sandboxing — within a single run, hooks have full process access including Actions secrets. Weakness: no provenance controls; secrets exfiltrable within a run.

3. **Cursor** — Workspace trust provides a coarse gate. `failClosed` option for security-critical hooks. Agent network restrictions (bypassed by hook scripts). Weakness: all-or-nothing trust, friction pushes toward blanket grant.

4. **Claude Code** — Enterprise `allowManagedHooksOnly`. HTTP hook `allowedEnvVars`. Source labels for audit visibility. Weakness: no approval gate, no modification detection, no sandbox.

5. **VS Code Copilot** — PreToolUse deny/ask/allow with "most restrictive wins". Weakness: no sandboxing, no provenance controls.

6. **Windsurf** — Configuration hierarchy enables org policy. Weakness: most explicit about lack of mitigations ("you are entirely responsible").

7. **OpenCode** — Structurally safer (no hook-scripts-from-repos model). Tool permissions with ask/allow/deny. Weakness: no OS-level sandbox, in-process plugins have full access.

8. **Kiro** — No documented security controls of any kind.

---

## Critical Gaps for Spec

1. **No agent implements hook signing or integrity verification.** Content hashing (SHA-256 of script files) is not used by any agent. Only Gemini CLI checks command-string identity.

2. **Supply chain attack via committed project hooks is universally under-protected.** 5/8 agents auto-execute hooks from cloned repos without consent.

3. **No agent sandboxes hooks by default.** Gemini CLI has sandbox support but it's separate from the trust model.

4. **PostToolUse prompt injection is an undocumented threat vector.** Every agent allows post-execution hooks to inject content influencing subsequent model behavior.

5. **Environment variable exfiltration is trivially available.** Only Gemini CLI has a structured deny-list (disabled locally by default).

6. **Failure mode defaults are dangerous.** Fail-open is default everywhere. Only Cursor has an explicit `failClosed` option.

7. **No audit trail.** No tamper-evident log of hook execution across any agent.

---

## Spec Recommendations

1. **Mandatory trust lifecycle for project-scoped hooks:** `unreviewed → approved → trusted`. Cloned-repo hooks must not execute until user approves. On by default, not opt-in.

2. **Content hash verification:** SHA-256 of hook script files stored at approval time, verified at execution time. Detect both command-string and script-content modifications.

3. **Risk-tier classification:** Read-only hooks (logging, context injection) vs write hooks (blocking, input rewrite) need different approval and sandbox requirements.

4. **Environment variable isolation as required control:** Hooks receive only declared-needed vars (`allowedEnvVars` pattern from CC HTTP hooks, applied to command hooks). Full `process.env` passthrough should not be default.

5. **Fail-closed default for blocking hooks:** Hooks implementing allow/deny should fail-closed on hook failure. Fail-open as explicit opt-in.

6. **Document PostToolUse prompt injection threat.** Hook-injected context should be marked/sandboxed in conversation so model can distinguish it.

7. **Discoverable hook audit log:** Append-only log of hook name, event, timestamp, exit code, input hash. Enables post-incident analysis.

8. **Sandbox-level recommendations by hook type:** Minimum: separate process with env-var allowlist. Recommended: filesystem write restrictions via OS-native mechanisms.
