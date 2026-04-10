# Hook Behavior Research Plan

**Date:** 2026-03-31
**Goal:** Validate and extend `docs/spec/hooks/hooks.md` (v0.1.0) with empirical runtime behavior data from 8+ agents
**Method:** Same as skill behavior checks — source code inspection + official docs, validated by subagents until clean

---

## Why This Research Differs from Skills

The skills research started with an external checks framework (agentskillimplementation.com) and discovered behaviors from scratch. The hooks spec already has extensive claims — event name mappings for 8 providers, blocking behavior matrices, capability support tables, and degradation strategies. The research here is primarily **validation**: are the spec's claims correct? Where they're correct, what implementation details are missing? Where they're wrong, what's the actual behavior?

---

## Research Questions (22 checks across 5 categories)

### Category A: Event Name Mapping Validation (spec Section 7.4)

The spec maps 20 canonical events to 8 provider-native names. These mappings are the foundation of the conversion pipeline. If any are wrong, hooks silently bind to the wrong lifecycle moment.

**A1. before_tool_execute mapping accuracy**
For each agent: what is the actual native event name for "before a tool runs"? Does it match the spec's mapping table?
- Spec claims: CC=`PreToolUse`, Gemini=`BeforeTool`, Cursor=`beforeShellExecution`/`beforeMCPExecution`/`beforeReadFile`, Windsurf=`pre_read_code`/`pre_write_code`/`pre_run_command`/`pre_mcp_tool_use`, etc.

**A2. after_tool_execute mapping accuracy**
Same as A1 for the after-tool event. Special attention to Cursor (`afterFileEdit` — is this the only after-tool event?).

**A3. session_start / session_end mapping accuracy**
For each agent: are these events real? What triggers them? Do agents that show `--` truly not support them?

**A4. before_prompt mapping accuracy**
Spec claims CC=`UserPromptSubmit`, Gemini=`BeforeAgent`, Cursor=`beforeSubmitPrompt`. Validate.

**A5. agent_stop mapping accuracy**
Spec claims CC=`Stop`, Gemini=`AfterAgent`, Cursor=`stop`. Validate the retry behavior column.

**A6. Extended event existence**
Validate that `before_model`, `after_model`, `before_tool_selection` exist in Gemini CLI as claimed. Validate that `subagent_start`/`subagent_stop` exist in Claude Code and VS Code Copilot.

**A7. Provider-exclusive event existence**
Validate Kiro's `File Create`, `File Delete`, `Pre Task Execution`, `Post Task Execution`. Validate CC's `ConfigChange`.

**A8. Split-event behavior (Cursor, Windsurf)**
The spec claims Cursor and Windsurf split `before_tool_execute` into category-specific events. Validate: are these truly separate events, or aliases? How does the agent decide which event to fire?

### Category B: Blocking Behavior Validation (spec Section 8)

**B1. Exit code 2 semantics**
For each agent: does exit code 2 actually block? The format convergence analysis found Windsurf uses JSON responses, not exit codes — does the spec account for this?

**B2. Blocking truth table accuracy**
Validate the spec's combined truth table (Section 5.3). Especially: does `decision: "deny"` with exit code 0 actually block on each agent? Does `decision: "ask"` work?

**B3. Non-blocking downgrade**
Does each agent correctly ignore exit code 2 when `blocking: false`? Or do some agents block regardless?

**B4. Timing shift accuracy**
The spec warns that Cursor maps some before-events to after-events (Section 8.3). Validate: which specific matchers cause this inversion?

### Category C: Capability Support Validation (spec Section 9)

**C1. structured_output support matrix**
Validate the 8-cell support table. Especially: does Windsurf truly not support structured output? Does Cursor use `permission`/`userMessage`/`agentMessage` fields?

**C2. input_rewrite support matrix**
Validate that only CC, VS Code Copilot, and OpenCode support `updated_input`/`updatedInput`. This is the safety-critical capability — getting it wrong means hooks that think they're sanitizing input are actually doing nothing.

**C3. llm_evaluated support**
Validate that CC's `prompt` and `agent` handler types work as described. Validate Kiro's "Ask Kiro" mechanism.

**C4. http_handler support**
Validate CC's `type: "http"` with `url`, `headers`, `allowedEnvVars`. Is this the only agent with HTTP handler support?

**C5. async_execution support**
Which agents support fire-and-forget hooks? Is the spec's support matrix correct?

### Category D: Runtime Behavior (not yet in spec)

**D1. Hook discovery mechanism**
How does each agent find hooks? Config file scan? Directory scan? Extension settings? The spec defines the canonical format but not the discovery path.

**D2. Hook execution environment**
What env vars does each agent pass to hooks? What's in stdin? Is the CWD the project root or the hook directory? What's the shell used?

**D3. Hook ordering**
When multiple hooks bind to the same event, what's the execution order? The spec says "order they appear in the array" — do agents honor this?

**D4. Concurrent hook execution**
Can hooks for the same event run in parallel, or are they sequential? What about hooks for different events?

**D5. Error propagation**
When a hook crashes (segfault, OOM, permission denied), how does each agent handle it? Is it always treated as exit code 1?

**D6. Timeout implementation**
How do agents handle hook timeouts? Kill signal? Grace period? The spec recommends 30s default — what do agents actually use?

### Category E: Security and Trust (cross-ref with security-considerations.md)

**E1. Hook provenance controls**
Does any agent verify hook integrity (hashes, signatures) before execution? The spec's security doc identifies this as a gap.

**E2. Hook sandboxing**
Are hooks executed in any kind of sandbox? Network access? File system access? Process spawning?

---

## Agent Coverage

Research each check for these agents (matching the spec's 8 provider slugs):

| Agent | Slug | Source Available? |
|-------|------|-------------------|
| Claude Code | `claude-code` | Docs: code.claude.com/docs/en/hooks |
| Gemini CLI | `gemini-cli` | Source: google-gemini/gemini-cli |
| Cursor | `cursor` | Docs: cursor.com blog posts, community |
| Windsurf | `windsurf` | Docs: docs.windsurf.com/windsurf/cascade/hooks |
| VS Code Copilot | `vs-code-copilot` | Source: microsoft/vscode-copilot-chat (partial) |
| GitHub Copilot CLI | `copilot-cli` | Docs: docs.github.com/copilot |
| Kiro | `kiro` | Docs: kiro.dev/docs |
| OpenCode | `opencode` | Source: sst/opencode |

**Additional agents to consider** (not in spec but have hooks):
- Amazon Q Developer (hooks with matchers)
- Cline (directory-based hooks)
- Roo Code (if hooks added)
- Augment Code (.augment/hooks/)
- Factory Droid (~/.factory/hooks/)
- Pi (TypeScript extensions via jiti)
- gptme (plugin hooks)

---

## Execution Plan

### Phase 1: Spec Validation (Categories A-C)
Dispatch 4 parallel subagents, each covering 2 agents from the spec's 8:
- Group 1: Claude Code + Codex/Copilot CLI (Anthropic + OpenAI ecosystem)
- Group 2: Gemini CLI + Kiro (Google ecosystem)
- Group 3: Cursor + Windsurf (VS Code fork ecosystem)
- Group 4: VS Code Copilot + OpenCode (extension + OSS)

Each agent validates the spec's event mappings, blocking behavior, and capability matrices against source code or official docs. Output: per-group markdown with CONFIRMED/CORRECTED/NOT_FOUND for each spec claim.

### Phase 2: Runtime Behavior Discovery (Category D)
Dispatch 4 parallel subagents with the same grouping, answering D1-D6 for each agent. This is new research — the spec doesn't cover these topics.

### Phase 3: Security Audit (Category E)
Single focused subagent cross-referencing security-considerations.md against actual agent implementations.

### Phase 4: Non-Spec Agents
Dispatch subagents for Amazon Q, Cline, Augment, Factory Droid, Pi, gptme to discover hook behaviors for agents not yet in the spec.

### Phase 5: Validation
Per the skills research protocol: validate all claims with sonnet subagents, fix corrections, re-validate until CLEAN.

### Phase 6: Compilation
Compile into:
- `00-hook-behavior-matrix.md` — Master matrix (analogous to skill behavior matrix)
- `spec-validation-report.md` — Corrections and confirmations for hooks.md
- `reference/behavior-data-spec-influences.md` — How findings inform spec v0.2

---

## Expected Outcomes

1. **Spec corrections** — Event mappings or capability claims that are wrong
2. **Spec additions** — Runtime behaviors (D1-D6) that should be normative or informative in the spec
3. **New provider data** — Hook behaviors for agents not yet in the spec (Amazon Q, Cline, etc.)
4. **Security findings** — Gap between spec security model and actual agent implementations
5. **Converter test cases** — Real-world edge cases that should become test vectors

---

## Relationship to Existing Research

- `docs/research/format-convergence-analysis/06-emerging-patterns.md` Section 6 has per-agent hook implementation details for 11 agents
- `docs/research/format-convergence-analysis/01-format-convergence.md` Section 3 has hook format tables
- The format convergence data can serve as a starting point — but it was format-focused, not runtime-focused
