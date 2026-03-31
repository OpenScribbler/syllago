# How Hook Behavior Research Influences Spec v0.2

Research date: 2026-03-31

---

## 1. Immediate Corrections for v0.1.1 Patch

These are factual errors that should be fixed immediately without architectural changes.

### 1.1 Event Mapping Table (Section 7.4)
- Fix CC `error_occurred` → `ErrorOccurred` (add `stop_failure` → `StopFailure` as separate entry)
- Add Copilot CLI `agentStop` for `agent_stop`
- Add Cursor `sessionStart`/`sessionEnd`
- Add Cursor `afterShellExecution`, `afterMCPExecution`, `postToolUse`
- Remove Cursor `beforeAgentResponse` and `beforeToolSelection` mappings
- Fix all Kiro event names from display names to camelCase identifiers
- Fix Kiro `session_end` → remove mapping (no session-end event exists)
- Add note about Kiro IDE vs CLI differences

### 1.2 Blocking Behavior Matrix (Section 8.2)
- VS Code Copilot `before_prompt`: change `prevent` → `observe`
- Cursor `before_prompt`: change `observe` → `prevent`
- Windsurf `before_prompt`: change `observe` → `prevent`
- Kiro IDE `before_prompt`: change `observe` → `prevent` (add note: CLI remains observe)
- VS Code Copilot `after_tool_execute`: add note that `decision: "block"` is supported
- Copilot CLI `agent_stop`: change `--` → `observe`
- Rename `retry` to `continue` for CC, Gemini CLI, VS Code Copilot

### 1.3 Capability Matrices (Section 9)
- Add Gemini CLI to `input_rewrite` support matrix via `hookSpecificOutput.tool_input`
- Add Cursor to `input_rewrite` support matrix via `preToolUse.updated_input`
- Add Copilot CLI to `custom_env` support matrix
- Add missing Gemini CLI output fields to `structured_output`
- Correct OpenCode `async_execution` to supported (named hooks are async)

---

## 2. Architectural Changes for v0.2

### 2.1 Split Kiro into Two Provider Entries

Kiro's IDE and CLI systems have different event sets, different event names (though both camelCase), and different blocking semantics. The spec should either:
- **(Recommended)** Add a `kiro-cli` slug alongside `kiro` (IDE), with separate mappings
- Or add a "system" qualifier column to the mapping table

### 2.2 Add `stop_failure` as Distinct Canonical Event

The codebase already treats `error_occurred` and `stop_failure` as separate canonical events. The spec should formalize this distinction.

### 2.3 Rename `retry` to `continue` in Blocking Vocabulary

The `retry` behavior label is misleading. The actual mechanism is "prevent termination / force continuation." Add `continue` to the behavior vocabulary (Section 8.1):

| Behavior | Meaning |
|----------|---------|
| **continue** | The agent is prevented from stopping and forced to continue its loop. The blocking reason is injected as context for the next turn. This is NOT an automatic retry of the same operation. Consumes additional compute/premium requests. |

### 2.4 Add Runtime Behavior Section (New Section 15)

The spec currently covers format and semantics but not runtime behavior. Research discovered significant divergence in:

**Concurrency model:**
- Claude Code and Gemini CLI: hooks run in parallel by default
- Everyone else: sequential
- Impact: parallel hooks cannot chain output; sequential hooks may be able to

**Timeout defaults:**
- Range: 30s (Copilot, Kiro, VS Code) to 600s (CC command hooks)
- Units: seconds (most) vs milliseconds (Gemini CLI, Kiro)
- No timeout at all: OpenCode (in-process plugins)

**Error handling:**
- Fail-open is universal default (only Cursor has explicit `failClosed` option)

**Recommendations for spec:**
- RECOMMEND 30s default timeout (most common; CC's 600s is an outlier for CI use cases)
- RECOMMEND sequential execution as default (easier to reason about; parallel as opt-in)
- RECOMMEND fail-open default with `fail_closed: true` option for security-critical hooks
- DEFINE timeout unit as seconds (canonical) with conversion note for ms-based agents

### 2.5 Add Security Considerations Section (New Section 16)

Research found 8 critical security gaps across the ecosystem. Spec should include:

**MUST requirements:**
1. Conforming implementations MUST NOT auto-execute project-scoped hooks without user awareness (warning or approval)
2. Conforming implementations MUST document what environment variables hook processes receive
3. Blocking hooks SHOULD default to fail-closed behavior

**SHOULD requirements:**
1. Implementations SHOULD implement content-based integrity verification (SHA-256 of hook script files)
2. Implementations SHOULD provide an environment variable allowlist mechanism (hooks receive only declared-needed vars)
3. Implementations SHOULD provide a hook audit log (append-only: name, event, timestamp, exit code)

**Informative guidance:**
- Supply chain risk: project hooks in cloned repos are an attack vector
- PostToolUse context injection is a prompt injection vector
- Hooks have full process environment access by default on most agents

---

## 3. New Capabilities from Non-Spec Agents

These capabilities appear in non-spec agents and should be considered for the spec's capability registry.

### 3.1 Output Caching (Amazon Q)

`cache_ttl_seconds` on hook output avoids re-running expensive hooks per prompt. The spec has no model for this. Proposed capability:

```
### 9.X `output_caching`
Hook output is cached and reused for subsequent events within a TTL window.
**Inference rule:** Tooling infers this when `handler.cache_ttl` is present.
**Default degradation:** `warn` — hooks execute every time; cache is ignored.
```

### 3.2 Context Injection Field (Cline, Augment)

Hooks that inject text into the LLM context without blocking. The spec's `context` field in Section 5.1 partially covers this, but Cline's `contextModification` and Augment's `SessionStart` → context injection are richer patterns. The spec should clarify that `context` in the output schema is the canonical mechanism for this.

### 3.3 PostToolUse Blocking (Cline)

Cline allows PostToolUse to cancel further task processing (though the tool already ran). This is architecturally distinct from the spec's `observe` classification. The spec should acknowledge this as a possible `after_tool_execute` behavior variant:

| Behavior | Meaning |
|----------|---------|
| **observe+cancel** | The hook observes the completed action and can cancel subsequent processing in the current task chain. The completed action is not undone. |

### 3.4 System-Level Immutable Hooks (Augment, Windsurf)

Enterprise deployment where IT/security can enforce hooks that users cannot override. The spec should add a `scope` field to the canonical format:

```json
{
  "scope": "system",
  "immutable": true
}
```

### 3.5 Privacy Opt-In Data Model (Augment)

Hooks receive minimal data by default; sensitive data requires explicit opt-in flags. The spec should RECOMMEND data minimization as a security best practice.

---

## 4. Converter Impact

### 4.1 Existing Converter Bugs (from toolmap.go)

The research confirms these toolmap.go entries are wrong and would produce incorrect conversions:
- Kiro file events: `"File Save"` should be `"fileEdited"`, etc.
- CC error_occurred: currently maps to `"ErrorOccurred"` in toolmap but this may not be a real CC event

### 4.2 New Conversion Paths Enabled

With corrected mappings, these conversions become possible:
- **Gemini CLI ↔ Cursor input rewrite:** Both support it — conversion is now lossless for this capability
- **Copilot CLI custom_env:** Can now round-trip env vars with VS Code Copilot
- **Cursor session events:** Can now convert session lifecycle hooks to/from Cursor

### 4.3 Conversion Correctness Rules

**Rule 1 — Cursor input_rewrite + category-specific events:**
`updated_input` only works on the generic `preToolUse`, NOT on category-specific events (`beforeShellExecution`, `beforeMCPExecution`). If a source hook uses input rewriting AND targets a specific tool category, the converter MUST either:
- Map to generic `preToolUse` (losing the category-specific matcher precision), OR
- Apply `degradation: block` to prevent silent input rewrite drops

This is a correctness bug waiting to happen, not just a warning.

**Rule 2 — Kiro conversion target:**
Conversions MUST target the Kiro CLI format (JSON config with camelCase event identifiers). Kiro IDE uses `.kiro.hook` files (YAML frontmatter + markdown) which are a different format and NOT a converter output target. The Kiro IDE column in the behavior matrix is for documentation only — it informs the mapping table but is not a supported conversion destination.

**Rule 3 — OpenCode converter scope:**
OpenCode uses an in-process TypeScript plugin model (Bun `import()`). Converting a canonical shell hook to OpenCode requires TypeScript source generation, not config transformation. This is **out of scope** for the current converter. The OpenCode column in the behavior matrix documents behavior for spec completeness but does not imply syllago can produce working OpenCode plugins from canonical hooks.

### 4.4 Conversion Warnings

- OpenCode `permission.ask`: hook exists in types but is never triggered (bug #7006) — converter should warn and note the hook will be non-functional
- `ErrorOccurred` (CC): mapping exists in toolmap.go but event may be undocumented in CC — mark as unconfirmed until verified

---

## 5. Panel Review Feedback (Applied)

Multi-persona panel (5 personas: Spec Author, Security Reviewer, Converter Implementor, Hook Author, Enterprise Administrator) reviewed the deliverables and produced 10 unified action items. The following high-priority items were applied to this document:

### Applied (P1)

**U1. VS Code Copilot `before_prompt` observe-only gap:**
Hook authors deploying prompt-guard hooks that target VS Code Copilot will find them silently ignored. The spec SHOULD include an explicit callout: "VS Code Copilot does not support blocking on `before_prompt`. Hooks using `before_prompt` for security enforcement SHOULD include `degradation: { before_prompt: block }` for VS Code Copilot, which will cause the hook to be excluded entirely on that agent rather than deployed as a silent no-op."

**U2. Kiro IDE vs CLI converter target resolved:**
See Rule 2 in Section 4.3 above. Kiro IDE is documentation-only, not a converter output target.

### Applied (P2)

**U3. Severity ratings for non-existent Cursor events upgraded:**
`beforeAgentResponse` and `beforeToolSelection` upgraded from Medium to Critical/High in spec-validation-report.md. A converter producing these event names generates hooks that silently never fire.

**U5. Copilot CLI sandbox entry corrected:**
The security posture entry for Copilot CLI should read "None (ephemeral runner provides run isolation between runs, not in-run sandboxing)" — the runner is not a real sandbox.

**U6. Parallel vs sequential hook authoring guidance:**
The spec SHOULD include a hook authoring rule: "Hooks MUST NOT depend on output from sibling hooks firing on the same event unless the target agent is known to use sequential execution. Claude Code and Gemini CLI (default) execute hooks in parallel — sibling hook output chaining will silently fail."

**U7. OpenCode converter scope clarified:**
See Rule 3 in Section 4.3 above. Shell-to-OpenCode conversion is out of scope.

### Deferred (P3)

**U8.** Security MUST requirements and immutable hook model cross-reference — deferred to spec v0.2 security section drafting.
**U9.** Cursor updated_input incompatibility — addressed as Rule 1 in Section 4.3.
**U10.** Enterprise deployment gap for CC/Cursor/VS Code Copilot — acknowledged. These agents have no system-level hook enforcement mechanism. Enterprise administrators must rely on repo access controls as the sole defense layer.
