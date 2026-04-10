# Hook Interchange Format Specification v1.0.0-draft -- AI Tool Builder Adoption Assessment

**Reviewer persona:** Lead Engineer building a new AI coding assistant (competitor to Cursor, Windsurf, Cline). Team of 8 engineers evaluating whether to adopt this hook specification as native hook format or build their own. Focused on implementation complexity, runtime performance, developer experience, and product roadmap constraints.

---

## 1. Build vs Adopt Decision

**Recommendation: Adopt as interchange, do not adopt as native format.**

This spec is well-designed as a *conversion hub* -- which is exactly what it claims to be. It is explicitly not trying to be a runtime format. If we adopted it as our native format, we would be constrained by the lowest-common-denominator of 8 providers. Instead, we should:

- Design our own native hook system optimized for our architecture and user needs.
- Implement a syllago adapter (decode + encode) so our hooks are portable to/from other tools.
- Register ourselves as a provider in the registry.

The spec makes this easy because it's designed as hub-and-spoke -- every provider keeps their native format and just implements an adapter. The cost of that adapter is low (Section 12 is a clear 4-stage pipeline), and the benefit is instant interop with the Claude Code, Gemini CLI, Cursor, and Windsurf ecosystems.

The CC-BY-4.0 license is as permissive as it gets for a spec. No patent traps, no implementation requirements beyond what we choose.

## 2. Implementation Complexity

**Core conformance: ~1-2 engineer-weeks. Extended: ~3-4. Full: ~6-8.**

Breaking it down:

- **Core** (Section 13.1) requires parsing the manifest, supporting 6 events, exit code contract, bare string matchers, and one provider mapping. This is straightforward JSON parsing + a lookup table. Most of the work is the exit code handling and process lifecycle management.

- **Extended** (Section 13.2) adds regex/MCP/array matchers, structured output parsing, capability inference, and degradation strategies. The capability inference is clean -- it's just checking for presence of specific fields, not complex analysis. Degradation is a lookup table.

- **Full** (Section 13.3) adds round-trip fidelity and verification. The verification stage (re-decode after encode) is clever but doubles the adapter code that needs to be correct.

The **split-event provider** handling (Section 7.4, for Cursor/Windsurf) is the single most complex part of the conversion pipeline. A single canonical `before_tool_execute` with an array matcher like `["shell", "file_write"]` can fan out into multiple provider-native events. The Cursor multi-event test vector shows this clearly -- the array gets split into `beforeShellExecution` + `afterFileEdit`, with a warning that `file_write` maps to an after-event because Cursor has no before-event for file operations. This is inherent complexity from the provider landscape, not spec bloat.

The JSON Schema (`hook.schema.json`) is well-structured with `additionalProperties: true` everywhere, which is good for forward compatibility but means validation alone does not catch semantic errors.

## 3. Architecture Fit

**Good fit for the event-driven lifecycle model that all AI coding tools share.**

The spec correctly identifies the fundamental lifecycle moments: before/after tool execution, session start/end, before user input reaches the agent, and agent termination. These map directly to the hook points we would build anyway.

**Concerns:**

- **The "command" handler type assumes shell execution.** Our tool might run in environments (cloud IDE, browser) where spawning a shell process is expensive or impossible. The spec acknowledges alternative handler types (`http`, `prompt`, `agent`) as capabilities, but `command` is the only first-class type. If we are building a cloud-native tool, we would want `http` or `wasm` as first-class.

- **No in-process handler type.** OpenCode uses TypeScript plugins that run in-process. The spec treats this as out-of-scope (adapters generate bridge code). For us, an in-process extension model might be preferable to shell spawning for performance reasons, and we would need to handle that conversion ourselves.

- **The matcher model assumes tool names are known strings.** This works well for the current generation of tools (Bash, Read, Write, etc.) but may not map cleanly if our tool uses dynamic tool registration, agent-defined tools, or tools with parameterized names.

## 4. Event Model Review

**Core events are solid. The extended/exclusive events reveal the real fragmentation.**

The 6 core events (`before_tool_execute`, `after_tool_execute`, `session_start`, `session_end`, `before_prompt`, `agent_stop`) represent genuine consensus across providers. These are the right primitives.

**What is well-covered:**
- Tool lifecycle (before/after) with matcher-based filtering -- this is the most common hook use case by far.
- Session lifecycle -- critical for init/cleanup.
- Prompt interception -- important for PII redaction, policy enforcement.

**What is missing:**
- **No `before_response` or `after_response` event.** There is no hook point between the LLM generating a response and it being shown to the user. Gemini has `after_model` but that is provider-exclusive. This is a significant gap for content filtering, brand compliance, and output redaction use cases.
- **No `before_file_read` as a core event.** Cursor has `beforeReadFile` but it is buried in the split-event mapping. Data classification (preventing the agent from reading `.env` files, secrets, etc.) is a real enterprise need that deserves first-class treatment.
- **No streaming hook.** All hooks are fire-once at lifecycle boundaries. There is no way to hook into streaming token output, which matters for real-time content filtering.
- **No `context_window_change` event.** When the context window is truncated, compacted, or summarized, users may want to inject or preserve context. `before_compact` exists but is extended, not core.

**What seems unnecessary for v1:**
- `notification` as an event feels premature. It is vaguely defined ("non-blocking system notification") and only supported by 2 providers.
- `config_change` as a provider-exclusive event for Claude Code -- this is an internal implementation detail, not a lifecycle moment.

## 5. Performance Concerns

**Shell execution overhead is the elephant in the room.**

Every hook invocation spawns a process: fork, exec, shell parse, script load, execute, exit. On a modern machine, this is 5-50ms per hook. For a single blocking `before_tool_execute` hook, that is tolerable. But consider the compound case:

- User has 5 hooks on `before_tool_execute` (security check, audit log, input sanitizer, PII scanner, policy gate).
- Agent is performing a multi-step task that involves 20 tool calls.
- That is 100 process spawns, adding 0.5-5 seconds of pure overhead to the task.

**Specific concerns:**

1. **Blocking hooks on the critical path.** A blocking `before_tool_execute` hook with a 10-second timeout means every tool call could take 10 seconds longer if the hook is slow. The spec recommends 30-second default timeouts (Section 3.5) -- that is extremely generous for something on the hot path.

2. **No batching.** If 3 hooks match the same event+tool, they run sequentially. There is no facility for parallel execution of independent non-blocking hooks.

3. **No caching or memoization.** A hook that checks the same policy on every `before_tool_execute` re-runs the full script every time. The spec has no concept of hook result caching.

4. **JSON parsing on every invocation.** The output schema (Section 5) requires parsing JSON from stdout on every hook exit code 0. For high-frequency events, this adds up.

**What I would want for our tool:**
- In-process hooks (WASM, JS/TS plugin) as a first-class option, not just shell commands.
- Parallel execution of non-blocking hooks.
- A warm process model (long-running hook daemon that receives events over stdin/stdout or a socket) to avoid fork/exec overhead.
- Hook result caching with invalidation.

The spec does not prevent any of these as provider-specific optimizations, but it also does not define interfaces for them.

## 6. Developer Experience

**The authoring experience is clean. The debugging experience is undefined.**

**Positives:**
- The canonical format is readable. A hook author can understand `{"event": "before_tool_execute", "matcher": "shell", "handler": {"type": "command", "command": "./check.sh"}, "blocking": true}` immediately.
- Exit codes 0/1/2 are simple and memorable. The contract is easy to implement in any language.
- The tool vocabulary (`shell`, `file_read`, `file_write`, etc.) uses intuitive names.
- Matcher syntax has a nice progression: bare string (simple), pattern object (medium), MCP object (specific), array (compound).

**Negatives:**
- **No hook naming.** Hooks have no `name` or `id` field. When a user has 10 hooks, debugging which one fired, which one blocked, which one timed out is going to produce error messages like "Hook 3 on PreToolUse returned exit code 2." Adding a `name` field would be trivial and dramatically improve DX.
- **No dry-run or test mode.** The spec defines no mechanism for a hook author to test their hook without actually triggering the real event. Test vectors exist for adapter implementors but not for hook authors.
- **Degradation warnings go... where?** The spec says adapters "emit warnings" (Section 11) but does not define where warnings go. Stderr? A warnings array in the output? A separate warnings file? This is left to implementations, which means every tool will handle it differently.
- **The `decision: "ask"` fallback is harsh.** If the provider does not support interactive confirmation, `ask` becomes `deny` (Section 5.2). That means a hook author who writes `decision: "ask"` gets silent blocking on most providers. This should be documented more prominently or the default fallback should be configurable.

## 7. Product Roadmap Constraints

**Adopting as interchange: minimal constraints. Adopting as native: significant constraints.**

If we implement an adapter only (recommended approach):

- We are free to design our native hook system however we want.
- We add/remove native events without asking permission from the spec.
- We can implement advanced features (WASM handlers, streaming hooks, hook chaining) natively and mark them as provider-exclusive.
- The only constraint is maintaining the adapter, which is a lookup-table exercise.

If we adopted this as our native format:

- We would be locked into the `command` handler type as the primary execution model.
- Our event names would need to be canonical snake_case, which might not match our internal architecture.
- Adding a new capability requires either going through the spec contribution process or using `provider_data` as an escape hatch.
- The `provider_data` escape hatch is explicitly opaque and not validated by the spec, which means our provider-specific features would live in an untyped blob.

**The spec does not constrain our roadmap if we treat it as what it is: an interchange format.** The risk is in over-adopting it.

## 8. Test Vector Analysis

**The test vectors are good but insufficient in quantity and provider coverage.**

**What is covered well:**
- The three canonical files (simple-blocking, full-featured, multi-event) cover the most important conversion paths: bare string matcher, MCP matcher, array matcher, wildcard, platform overrides, provider_data, and multiple event types.
- The `_comment` and `_warnings` fields in test vectors are excellent -- they explain the conversion rationale, which doubles as documentation.
- The Gemini CLI vectors correctly show timeout conversion (seconds to milliseconds).
- The Cursor vectors correctly show split-event handling and the lossy `file_write` -> `afterFileEdit` mapping.

**Accuracy concerns:**

1. **Claude Code `blocking` field is lost.** In the Claude Code test vectors, the `blocking: true` from the canonical format does not appear in the output. Claude Code hooks are blocking by default when they return exit code 2, so this is technically correct -- the blocking semantic is preserved through the exit code contract rather than a field. But the test vector does not make this explicit.

2. **Cursor `afterFileEdit` for `file_write` before-event.** The multi-event Cursor vector maps a `before_tool_execute` + `file_write` to `afterFileEdit`, with a warning that it is the "closest match." This is an honest lossy conversion, but it means the hook fires *after* the file is written, not before. For a security hook that blocks dangerous writes, this is a total semantic inversion. The warning is there, but the test vector treats this as acceptable when it should arguably trigger a `block` degradation.

3. **Gemini CLI multi-event splits array matchers.** The canonical `["shell", "file_write"]` becomes two separate Gemini hooks with duplicate commands. This is correct but means the hook script runs twice for a single canonical intent if both tools are invoked. No warning is emitted about this duplication.

**What is missing:**
- No test vectors for Windsurf, VS Code Copilot, Copilot CLI, Kiro, or OpenCode. Only 3 of 8 providers have vectors.
- No test vector for `input_rewrite` degradation (the safety-critical capability).
- No test vector for `llm_evaluated` or `http_handler` conversion.
- No test vector for round-trip: canonical -> provider -> canonical and verifying equivalence.
- No test vector for error cases (invalid event name, unsupported event on target, timeout edge cases).

## 9. Missing Use Cases

Real-world hook scenarios that this spec does not cover:

1. **Hook ordering and priority.** When multiple hooks match the same event, the spec does not define execution order. In practice, users care deeply about this -- a "log everything" hook should run after a "block dangerous operations" hook, not before. Windsurf has a three-tier priority system (system > user > workspace) that solves this; the spec ignores it.

2. **Hook chaining and data passing.** Hook A's output cannot feed into Hook B's input. Each hook runs independently against the original event data. This prevents pipeline patterns like: sanitize -> validate -> log.

3. **Conditional hooks.** No mechanism for hooks that only fire based on file path patterns, project type, git branch, or environment. The `matcher` field only matches tool names, not contextual conditions. A common need: "only run this security hook on the `main` branch" or "only audit file writes in `src/`."

4. **Hook disabling per-project.** The policy interface (policy-interface.md) defines a `disable_all_hooks` kill switch, but there is no mechanism for a project to say "I want hooks X and Y but not Z." Since hooks have no names (see DX section), there is no way to reference them individually.

5. **Hook versioning and updates.** The spec versions the format but not individual hooks. There is no mechanism for a hook to declare its own version, check for updates, or declare compatibility with specific provider versions.

6. **Streaming/incremental hooks.** No support for hooks that process streaming output token-by-token. All hooks are batch: wait for the full event, run the script, parse the output.

7. **Multi-tool transactions.** No way to define a hook that fires only when a specific *sequence* of tools is invoked (e.g., "alert me when the agent reads a .env file and then makes a network request").

8. **Resource cleanup guarantees.** `session_end` is documented as "best-effort delivery" (Section 7.1) -- the process may exit before the hook completes. For hooks that need guaranteed cleanup (releasing locks, closing connections), this is insufficient.

## 10. Recommendations -- What Would Need to Change for Adoption

For us to adopt this spec as our interchange format (and contribute an adapter):

### Must-have changes (blocking)

1. **Add a `name` field to hook definitions.** This is trivial (optional string, no breaking change) and critical for debugging, logging, policy targeting, and user experience. Without it, every implementation will invent their own naming scheme and interop suffers.

2. **Add test vectors for at least 5 providers.** 3 of 8 is not sufficient for an interchange spec. We need to see Windsurf (split-event + enterprise features), Copilot CLI (bash/powershell split), and at least one more to trust the conversion model.

3. **Define where warnings go.** The spec says "emit a warning" dozens of times without specifying the output channel. Define a `_warnings` array in the encoded output (as the test vectors already use informally) or a separate warnings report format.

### Should-have changes (strongly desired)

4. **Add hook execution ordering.** At minimum, define that hooks fire in array order within the manifest. Ideally, support a `priority` field (integer, lower = earlier) for cross-manifest ordering.

5. **Add a `before_file_read` core event.** Data access control is a first-class enterprise concern. Cursor already has `beforeReadFile`. Promoting this to core (even if most providers map it to `before_tool_execute` + `matcher: "file_read"`) signals the right intent.

6. **Define a warm-process handler type.** A hook that starts once and receives events over stdin (JSON lines) would eliminate the fork/exec overhead for high-frequency events. This does not need to be required -- just having the type defined in the spec prevents 8 providers from inventing 8 incompatible long-running hook protocols.

7. **Add a test vector for `input_rewrite` degradation.** This is the spec's most safety-critical feature and it has zero test coverage.

### Nice-to-have changes (would improve our confidence)

8. Define the environment variables that providers SHOULD pass to hook processes (tool name, tool args as JSON, event name, session ID). Currently the spec says hooks "receive" tool names and arguments but does not define the data contract for how.

9. Formalize the `_warnings` / `_comment` fields that test vectors already use as part of the adapter output schema.

10. Add a conformance test suite (executable, not just JSON fixtures) that adapter implementors can run against their code.

## Summary

This is a well-crafted interchange specification that correctly identifies the problem space and makes sensible design decisions. The hub-and-spoke model is the right architecture for a fragmented provider ecosystem. The degradation strategy system (especially the `block` default for `input_rewrite`) shows real security thinking. The capability inference model avoids the brittleness of manual declarations.

The spec is weakest in: developer experience details (no hook naming, no debug story), performance considerations (shell-only handler model), test coverage (3/8 providers), and advanced use cases (no ordering, no chaining, no conditional execution).

**Our team's decision: adopt as interchange format, implement an adapter, contribute our provider mapping back to the spec. Estimated cost: 2-3 engineer-weeks for an Extended-conformant adapter. Do not adopt as our native format -- design our own hook system optimized for our architecture, with this spec as the portability layer.**
