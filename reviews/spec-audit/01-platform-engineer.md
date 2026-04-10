# Hook Interchange Format Specification v1.0.0-draft -- Platform Engineer Review

**Reviewer persona:** Senior Platform Engineer at a large enterprise (10,000+ engineers) managing developer tooling infrastructure, internal developer platforms, and CI/CD pipelines. 15 years experience at Google, Stripe, and Netflix.

---

## 1. Executive Summary

This is a thoughtfully designed interchange specification that correctly identifies a real problem -- hook portability across AI coding tools -- and proposes a hub-and-spoke canonical format rather than trying to unify provider implementations. The spec is well-structured, mostly unambiguous, and demonstrates deep knowledge of the provider landscape. However, it has significant gaps around runtime behavior, enterprise operability, and several security blind spots that would need to be addressed before I would approve this for adoption at scale.

## 2. Strengths

**Hub-and-spoke architecture is the right call.** Section 1 (lines 43-44) explicitly states the canonical format "does not replace any provider's native format" -- this is the correct design for an interchange format. Trying to unify 8+ providers into a single runtime format would be a multi-year failure mode. The decode-validate-encode-verify pipeline (Section 12) is clean and composable.

**Degradation strategy system is well-designed.** The capability-aware degradation model (Section 11) with safe defaults is the standout feature. The `input_rewrite` defaulting to `block` (Section 9.2, line 514) is a security-first decision that shows mature thinking. Quoting: "This prevents a false sense of security." That single sentence demonstrates the spec authors understand the threat model.

**Split-event provider handling is explicit.** The spec calls out Cursor and Windsurf as split-event providers (Section 7.4, line 426) and provides explicit merge/split rules. This is the kind of detail that gets hand-waved in most interchange specs and then causes bugs for years.

**Test vectors are structured correctly.** The canonical-to-provider test vector structure (canonical/ -> claude-code/, gemini-cli/, cursor/) with matched filenames is the right approach. The `_comment` and `_warnings` fields in test vectors (e.g., `claude-code/full-featured.json` lines 3-8) are excellent -- they document the conversion rationale inline.

**Forward compatibility is baked in.** Section 3.2 (line 82): "Implementations MUST ignore unknown fields at any level." Combined with `additionalProperties: true` throughout the JSON Schema, this is correctly designed for evolution.

**Clean separation of concerns.** The policy interface (policy-interface.md) is deliberately separated from the format spec. The spec explicitly lists what it does NOT define (Section 1.2, lines 59-65). This shows discipline -- the temptation to scope-creep a v1 spec is real and they resisted it.

## 3. Gaps and Concerns

**No hook identity model.** Hooks have no `id`, `name`, or `version` field in the canonical format (Section 3.4, lines 96-104). This makes it impossible to:
- Reference a specific hook in policy rules ("deny hook X")
- Track hook versions across updates
- Build a revocation list
- Correlate audit log entries across systems
- Deduplicate hooks when merging configurations from multiple sources

At enterprise scale with potentially hundreds of hooks across dozens of teams, this is a critical gap. The policy interface (policy-interface.md Section 5, line 137) lists "Revocation of previously-allowed hooks" as future work, but you cannot revoke what you cannot identify.

**No hook ordering or priority model.** When multiple hooks bind to the same event, what order do they execute? The spec is silent. Section 3.4 defines a `hooks` array, which implies array order, but this is never stated normatively. At scale, hook ordering conflicts between teams are a significant operational burden. Windsurf's three-tier system (system > user > workspace) is mentioned in Appendix A (line 832) but not abstracted into the canonical format.

**No error aggregation or reporting contract.** Section 12.2 (line 725) says "Validation MUST produce a list of warnings" but defines no structure for these warnings. The `_warnings` array in test vectors is informational only. Without a standardized warning/error format, every implementation will produce different diagnostic output, making tooling integration painful.

**`before_prompt` blocking semantics are inconsistent.** The Blocking Behavior Matrix (Section 8.2) shows `before_prompt` as `prevent` for claude-code and gemini-cli but `observe` for cursor and windsurf. The spec says adapters "SHOULD emit a warning" (line 455) when blocking intent cannot be honored, but this is a SHOULD, not a MUST. A hook author writes a PII redaction hook on `before_prompt` with `blocking: true`, converts to Cursor, and their PII protection silently degrades to observational. This needs to be a MUST-warn, or better, should trigger the degradation strategy system.

**Timeout behavior is underspecified.** Section 3.5 (line 117) says implementations "SHOULD apply a reasonable default (30 seconds is RECOMMENDED)." But Section 4 (line 255) says timeout behavior for blocking hooks MAY be `block` if the author specified it in `provider_data`. This means timeout behavior is controlled by an opaque, provider-specific field rather than a canonical field. Timeout-on-block-hook is a critical safety decision that belongs in the canonical format, not in `provider_data`.

**No batch/bulk conversion semantics.** The conversion pipeline (Section 12) describes single-manifest conversion. In practice, enterprise deployments will have hundreds of hook manifests that need to be converted together. There is no discussion of:
- Deduplication (same hook from multiple sources)
- Conflict resolution (two hooks on same event with contradictory blocking)
- Atomic conversion (all-or-nothing vs. best-effort for a batch)

**Registry versioning is vague.** Section 14.2 (line 802) says registries use `YYYY.MM` format but does not say where this version is declared, how implementations discover it, or what happens when an implementation encounters an event name not in its registry version. The forward-compatibility rule (ignore unknown fields) does not clearly apply to unknown event names in matchers.

## 4. Security Review

**The threat model is incomplete.** Security-considerations.md Section 1.1 lists four threat actors but misses two critical ones for enterprise:

1. **Insider threat / compromised developer machine.** A developer with legitimate repository access installs a hook that exfiltrates code to an external endpoint. The hook manifest looks normal; the script payload is the attack. The spec's content hashing (Section 3.1) only verifies integrity, not intent.

2. **Hook chaining / escalation.** A non-blocking observational hook on `after_tool_execute` can read tool output (which may contain secrets from shell commands). Combined with an `http_handler` on another event, this creates a two-stage exfiltration path that no single hook review would catch. The spec has no concept of inter-hook data flow analysis.

**`provider_data` is an injection vector.** Security-considerations.md Section 2.2 (lines 36-41) correctly identifies this but the mitigation is weak -- "Adapters MUST NOT blindly copy" and "SHOULD validate." At enterprise scale, every adapter is a separate codebase maintained by different teams. A SHOULD on validation means some adapters will skip it. This should be a MUST with a defined validation interface.

**No content security policy for HTTP handlers.** Section 2.4 of security-considerations.md recommends displaying URLs and restricting to HTTPS, but:
- No allowlist/denylist mechanism for HTTP endpoints
- No domain pinning or certificate pinning option
- No rate limiting guidance for HTTP hooks (DoS against external services)
- No guidance on what request body data is sent (the spec never defines the HTTP handler's request contract)

The `http_handler` capability (Section 9.4) says adapters "MAY approximate HTTP handlers by generating a shell script that invokes `curl`." This generated curl script is itself an attack surface -- if the URL contains shell metacharacters, command injection is possible. The security doc mentions this for bridge plugins (Section 2.6) but not for curl generation.

**SHA-256 hashes without a trust anchor are security theater.** Security-considerations.md Section 3.1 defines per-file hashes but does not specify who generates them, where they are stored, or how they are verified. If the hash file ships alongside the hook (which is implied), an attacker who can modify the script can also modify the hash. Without a separate trust anchor (signed manifest, registry-provided hashes, Sigstore attestation), the hashes only catch accidental corruption, not malicious tampering.

**LLM-evaluated hooks are a prompt injection surface.** Section 2.5 (line 72) mentions prompt injection but only as a concern for hook authors. It does not address the scenario where a hook's `type: "prompt"` handler processes the output of a tool that was itself influenced by an attacker (e.g., a cloned repository with a malicious README that gets read by `file_read`, then passed to the hook's prompt). This is a transitive prompt injection vector.

## 5. Integration Concerns

**CI/CD pipeline integration is not addressed.** Enterprise hooks often need to run in CI/CD contexts (GitHub Actions, GitLab CI, Jenkins) where there is no interactive AI coding session. The spec assumes an interactive agent context (e.g., `decision: "ask"` in Section 5.1 defers to the user). There is no guidance on:
- How hooks behave in non-interactive/headless environments
- Whether `ask` should default to `deny` or `allow` in CI
- How timeout values should differ between interactive and CI contexts

**No integration with existing policy-as-code systems.** The policy interface (policy-interface.md) defines a bespoke policy model. Enterprise platform teams already use OPA/Rego, Cedar, or Sentinel for policy enforcement. The policy interface should either (a) define a generic policy evaluation hook that delegates to external policy engines, or (b) provide mappings to at least one established policy-as-code system.

**Observability gap.** The spec mentions audit logging (security-considerations.md Section 4.4) but defines no structured format. At 10,000+ engineers, I need:
- OpenTelemetry-compatible trace/span IDs for hook executions
- Structured log format (not just "SHOULD log")
- Metrics emission (hook latency, block rate, error rate per event)
- Integration with SIEM systems for security monitoring

Without these, adopting this at scale means every team builds their own observability layer around hooks, which is exactly the toil a spec should prevent.

**No guidance on hook testing.** Hook authors need to test their hooks against the canonical format before publishing. The spec defines test vectors for adapter validation but provides no guidance on hook-level testing:
- How to simulate events locally
- How to test exit code contracts
- How to validate structured output against the schema
- How to test degradation behavior before conversion

## 6. Spec Quality

**Writing quality is high.** RFC 2119 keywords are used correctly and consistently. Tables are well-formatted. Cross-references between sections are complete. The glossary accurately reflects the spec content.

**The JSON Schema is permissive but correct.** `additionalProperties: true` everywhere is necessary for forward compatibility but means the schema will validate garbage with extra fields. This is a deliberate tradeoff that is well-documented (Section 3.2). The schema correctly uses `oneOf` for the matcher union type (schema lines 71-93).

**One ambiguity in the exit code contract.** Section 4 (line 250) says exit code 2 on a non-blocking hook "MUST be treated as exit code 1." But Section 5.2 (line 276) says "Exit code 2 takes precedence over `decision: "allow"` in the JSON output." These interact: if a non-blocking hook returns exit code 2 with `decision: "allow"`, does the exit code 2 get downgraded to 1 first (non-blocking rule), making it a warning? Or does the exit code 2 precedence rule apply first? The spec should clarify the evaluation order.

**`_comment` in test vectors is non-standard.** JSON does not support comments, so the test vectors use `_comment` fields (e.g., `canonical/simple-blocking.json` line 2). This works because of the `additionalProperties: true` in the schema, but it means test vectors pass schema validation while containing non-canonical fields. The CONTRIBUTING.md should note this convention explicitly.

**Missing: event name enum in the schema.** The JSON Schema (line 31) defines `event` as `"type": "string"` with no enum constraint. This means the schema accepts any string as an event name, including typos. A non-normative `enum` or `examples` array would help tooling provide autocomplete and catch errors early without making the schema overly rigid.

**Test vector coverage is thin.** Three test vectors for the entire spec is insufficient. Missing vectors:
- Array matcher with mixed types (bare string + MCP object)
- `degradation` field triggering `block` or `exclude` during conversion
- Provider-exclusive events (round-trip test)
- HTTP handler type
- LLM-evaluated handler type
- Async execution
- Exit code / decision interaction edge cases

## 7. Recommendations (Prioritized)

### P0 -- Must fix before enterprise adoption

1. **Add a hook identity field** (`id` or `name` + `version`). Without this, policy enforcement, audit logging, revocation, and deduplication are all impossible at scale.

2. **Promote `provider_data` validation from SHOULD to MUST** in the security considerations. Define a minimum validation interface that all adapters must implement.

3. **Define structured warning/error format** for the validation stage. At minimum: severity, capability or event affected, source hook index, human-readable message. Without this, tooling integration requires parsing prose.

4. **Clarify exit code evaluation order** when blocking/non-blocking interacts with decision field precedence. Add a truth table.

### P1 -- Should fix before v1.0 final

5. **Add hook execution ordering semantics.** At minimum, define that hooks execute in array order and that implementations MUST NOT reorder. Ideally, define a priority model.

6. **Add timeout behavior to the canonical format** as a first-class field (e.g., `timeout_action: "warn" | "block"`), not buried in `provider_data`.

7. **Define headless/CI behavior** for interactive features like `decision: "ask"`. State normatively that `ask` MUST be treated as `deny` in non-interactive environments.

8. **Expand test vectors** to at least 10-12 cases covering all matcher types, all degradation strategies, provider-exclusive events, and non-command handler types.

9. **Strengthen blocking-on-observe from SHOULD-warn to MUST-warn** (Section 8, line 455). Better yet, make it trigger the degradation strategy system so authors can specify `block` or `exclude` when their blocking intent cannot be honored.

### P2 -- Should address before broad adoption

10. **Define a structured audit log format.** Even if optional, providing a schema for hook execution logs (timestamp, hook ID, event, exit code, latency, blocked?) enables consistent observability across implementations.

11. **Add guidance on CI/CD and non-interactive environments** as a new section or appendix.

12. **Address inter-hook data flow** in the security considerations. A hook reading tool output on `after_tool_execute` and an HTTP hook on another event create a two-stage exfiltration path. At minimum, call this out as a known risk.

13. **Define the HTTP handler request contract.** Section 9.4 says `type: "http"` exists but never specifies what fields are available (url, method, headers, body template). The curl approximation strategy cannot work without this.

14. **Add `examples` arrays to the JSON Schema** for `event` and `handler.type` fields. These are non-normative but dramatically improve tooling support.

---

**Bottom line:** This spec is 75% of the way to enterprise-ready. The format design, degradation model, and provider mapping work are strong. The gaps are mostly around operational concerns (identity, ordering, observability, CI integration) and hardening the security posture from advisory to mandatory. The P0 items are blockers for any enterprise deployment I would sign off on; the P1 items would need to be on the roadmap with committed dates.
