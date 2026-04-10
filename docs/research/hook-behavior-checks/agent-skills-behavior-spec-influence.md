# Lessons from the Metadata Convention for the Hook Spec

**Date:** 2026-03-31
**Status:** Reference document — captures cross-pollination from the metadata convention review process
**Context:** The Agent Skills Metadata Convention (`docs/spec/skills/metadata_convention.md`) went through 2 review rounds (5-persona adversarial review + behavioral data integration). Several findings apply directly to the hook spec or should inform how hook behavioral data is compiled.

---

## 1. Canonical Agent Identifier Alignment (CRITICAL)

**The problem:** The hook spec (section 3.6) defines its own provider slugs. The metadata convention (Appendix C) independently defines canonical agent identifiers. They are close but not aligned:

| Agent | Hook Spec Slug | Metadata Convention ID | Mismatch? |
|-------|---------------|----------------------|-----------|
| Claude Code | `claude-code` | `claude-code` | OK |
| Gemini CLI | `gemini-cli` | `gemini-cli` | OK |
| Cursor | `cursor` | `cursor` | OK |
| Windsurf | `windsurf` | `windsurf` | OK |
| VS Code Copilot | `vs-code-copilot` | `github-copilot` | MISMATCH |
| GitHub Copilot CLI | `copilot-cli` | (not listed) | MISSING |
| Kiro | `kiro` | `kiro` | OK |
| OpenCode | `opencode` | `opencode` | OK |
| Codex CLI | (not listed) | `codex-cli` | MISSING |
| Cline | (not listed) | `cline` | MISSING |
| Roo Code | (not listed) | `roo-code` | MISSING |
| Amp | (not listed) | `amp` | MISSING |
| Junie CLI | (not listed) | `junie-cli` | MISSING |

**The fix:** Two specs from the same ecosystem using different names for the same agents is exactly the fragmentation problem both specs are trying to solve. The hook behavior research should:
1. Flag the `vs-code-copilot` / `github-copilot` divergence — these may be different products (VS Code extension vs IDE extension) or the same product at different stages. The Group 4 research already notes "VS Code Copilot hooks and GitHub Copilot Coding Agent hooks are DIFFERENT products." If they're truly different, both need identifiers. If they're the same, pick one.
2. Propose a unified identifier table that both specs can reference.
3. Add `copilot-cli` to the metadata convention's Appendix C if it's a real product with skill support, or drop it from the hook spec if it's not.

**Source:** Metadata convention review — all 5 personas flagged freeform agent names as an interop problem. The fix was canonical identifiers with regex constraints (`[a-z][a-z0-9-]*`).

---

## 2. Content Hash Algorithm Precision

**The problem:** The hook spec's security doc (section 3.1) says "SHOULD include SHA-256 hashes" with a JSON example but doesn't specify:
- How to handle line endings (CRLF vs LF)
- How to handle self-referential hashing (if the manifest contains its own hash)
- What encoding to use for file paths in the hash computation
- Whether hidden files are included or excluded

The metadata convention hit all of these as interop-breaking issues:
- **CRLF divergence:** `git core.autocrlf=true` (Windows default) converts LF→CRLF on checkout. Same repo, different hash. All 5 reviewers flagged this.
- **Self-referential blanking:** "Replace content_hash with empty string" was ambiguous — YAML-level or byte-level? Different YAML serializers produce different bytes. All 5 reviewers flagged this.
- **Path encoding:** UTF-8 wasn't specified, causing potential divergence on non-ASCII filenames.

**The fix applied in the metadata convention:**
1. Mandatory CRLF→LF normalization before hashing
2. Byte-level string substitution for self-referential hash blanking (NOT YAML re-serialization)
3. UTF-8 encoding explicitly required for path bytes
4. Hidden files/directories excluded
5. Lexicographic sort by UTF-8 byte value
6. Null-byte separator between path and content

**Recommendation for hooks:** The hook spec's `content_hashes` field should adopt the same algorithm or reference the metadata convention's algorithm. At minimum, specify line-ending normalization and path encoding. The hook format is JSON (not YAML frontmatter), so the self-referential problem may not apply — but if the manifest ever includes its own hash, the blanking procedure needs to be specified.

**Source:** Metadata convention review — consensus fix #1 (content_hash replacement) and #4 (CRLF normalization).

---

## 3. Self-Assertion Warnings on Trust-Adjacent Fields

**The problem:** The metadata convention originally had `source_repo`, `publisher`, and `derived_from` without any trust boundary warnings. Three reviewers (security, spec pedant, registry maintainer) independently flagged that these fields *look* like verified provenance but are actually self-asserted claims anyone can fake.

**The hook spec equivalent:** The hook spec's security doc is ahead here — section 3.3 says "author metadata is self-reported and MUST NOT be treated as verified identity." But the main hook spec itself has fields that could be mistaken for trust signals:
- `name` — a hook claiming to be "security-audit" could be malicious
- `provider_data` — opaque data keyed by provider slug, with no integrity guarantee
- Any future `author` or `source` fields

**Recommendation:** Apply self-assertion warnings to any hook spec field that could be mistaken for verified trust. The security doc covers this conceptually, but normative warnings in the main spec (next to the field definitions) are more visible to implementers who may not read the security doc.

**Source:** Metadata convention review — strong-signal fix #7 (3/5 agreement).

---

## 4. Behavioral Security Tier Data for Hooks

**The problem:** The metadata convention found that quantifying security tiers by agent count was extremely persuasive in reviews and community discussions. "7 of 12 agents auto-load skills without consent" is more actionable than "some agents auto-load."

**The hook spec's security doc** correctly identifies hook RCE as a threat (section 1) and mentions the Feb 2026 CVE, but doesn't quantify the current landscape:
- How many agents auto-load hooks from cloned repos?
- How many require explicit approval before running hooks?
- How many sandbox hook execution?
- How many verify hook integrity before execution?

**Recommendation:** The hook behavior research (Categories D and E) should compile a security tier table similar to what the metadata convention produced:

| Security Tier | Definition | Agents | Count |
|---|---|---|---|
| **Gated** | Approval required before hook executes | ? | ? |
| **Permissioned** | Tool access controlled, no per-hook gate | ? | ? |
| **Sandboxed** | Hook runs with reduced privileges | ? | ? |
| **Open** | Auto-load, no approval, full privileges | ? | ? |

This data would strengthen the hook spec's security doc considerably and provide the same "concrete numbers instead of hand-waving" quality that made the metadata convention's behavioral data effective.

**Source:** Metadata convention `behavior-data-spec-influences.md` section 3 (Script Security).

---

## 5. Portable Glob/Matcher Syntax

**The problem:** The metadata convention originally referenced micromatch (a JS library) for `file_patterns` glob syntax. Three reviewers flagged that a JS library is not a specification — Go, Rust, and Python agents can't claim conformance against a library whose edge cases differ from every other glob engine.

**The hook spec equivalent:** Section 6 (Matcher Types) defines tool matchers. If any matcher uses glob-like patterns (e.g., for file path matching in `before_tool_execute` matchers), the same portability concern applies.

**The fix applied in the metadata convention:**
- Defined a portable glob subset inline: `*`, `**`, `?`, `[...]`
- Explicitly excluded brace expansion (`{a,b}`) and extglobs (`+(pattern)`) as non-portable
- Dropped all library references
- Added: "Agents MAY support them as extensions but MUST NOT require them"

**Recommendation:** Review the hook spec's matcher syntax for any glob-like patterns and apply the same treatment — define the portable subset, exclude non-portable extensions.

**Source:** Metadata convention review — consensus fix #3 (3/5 agreement).

---

## 6. `mode: always` / Persistent Hooks as Injection Vectors

**The problem:** The metadata convention found that `mode: always` skills (always loaded into context) combined with 7 agents that auto-load without consent create a context injection vector. A malicious `SKILL.md` in a cloned repo could inject arbitrary instructions into every conversation.

**The hook equivalent is worse.** Hooks execute code, not just inject context. A hook bound to `session_start` with no matcher runs on every session. If the agent auto-discovers hooks from the project directory (check D1 in the research plan), a malicious `hook.json` in a cloned repo could execute arbitrary code on first open.

**The hook spec's security doc** covers this well (section 1 threat model, section 2.1 "scripts are the attack surface"), but the behavioral data from the hook research should quantify:
- How many agents auto-discover hooks from project directories?
- How many require explicit hook registration?
- How many prompt the user before executing a newly discovered hook?

This maps directly to research question D1 (Hook discovery mechanism) and E1 (Hook provenance controls).

**Source:** Metadata convention review — notable single-persona finding (security persona) on `mode: always`.

---

## 7. Durability / Compaction Protection for Hook-Injected Context

**The problem:** The metadata convention introduced `durability` (persistent/ephemeral) because skill instructions may not survive context compaction. Only 1 of 12 agents confirmed compaction protection for skills.

**Hooks that inject context have the same risk.** The hook spec's security doc (section 4.1) mentions hooks that return `context` or `system_message` fields. These inject text into the conversation or system prompt. If the agent compacts context mid-session, hook-injected text may be lost — and the hook won't re-fire to reinject it (the event already passed).

**Recommendation:** The hook behavior research should check (as part of D-category runtime questions):
- For hooks that inject `context` or `system_message` at `session_start`, is that content protected from compaction?
- If a `before_prompt` hook injects context, does it persist across turns or get re-injected each turn?

This isn't necessarily a hook spec field (hooks fire on events, not from static context), but it affects the correctness of hooks that inject context and should be documented as a known limitation.

**Source:** Metadata convention `behavior-data-spec-influences.md` section 4 (Durability).

---

## 8. Behavioral Assumptions Checklist Pattern

**The problem:** The metadata convention's `supported_agents` section was vague — "works with Agent X" didn't mean anything concrete. The fix was a 6-point behavioral checklist defining what "supported" actually means in terms of testable runtime assumptions.

**The hook spec could benefit from a similar pattern.** "This hook works on Claude Code and Gemini CLI" should mean something verifiable. A hook-specific checklist might include:

1. **Event binding.** The target agent supports the hook's canonical event. Check the event registry mapping table.
2. **Blocking behavior.** If the hook uses `blocking: true`, the target agent honors exit code 2 as a block signal (not all do — Windsurf uses JSON responses).
3. **Matcher syntax.** The target agent supports the matcher type used (string, glob, regex). Some agents only support string matchers.
4. **Structured output.** If the hook returns structured JSON output, the target agent parses it. Some agents only read exit codes.
5. **Input rewrite.** If the hook uses `input_rewrite`, the target agent supports `updated_input`. Only 3 agents do — on others, the rewrite is silently dropped, which is a safety-critical failure.
6. **Timeout handling.** The target agent enforces the hook's timeout and handles the `timeout_action` field.

This checklist is derivable from the spec's existing capability matrices, but presenting it as a practical "before you claim support" checklist (like the metadata convention does) makes it more actionable for hook authors.

**Source:** Metadata convention `supported_agents` behavioral assumptions checklist.

---

## 9. Version Constraint Syntax Alignment

**The problem:** The metadata convention defines a version constraint syntax for `expectations` (semver range operators: `>=`, `~>`, etc.). If the hook spec ever adds dependency declarations (e.g., "this hook requires Node >= 18"), it should use the same syntax.

**Current state:** The hook spec doesn't have dependency declarations. But if they're added later, aligning with the metadata convention's grammar avoids the ecosystem having two different constraint syntaxes.

**Recommendation:** Not actionable now, but note for future hook spec versions.

---

## 10. Review Process as a Pattern

**The metadata convention's 5-persona adversarial review found real issues.** The process:
1. Write the spec
2. Integrate behavioral data
3. Run 5 simulated reviewers in parallel (skill author, agent developer, registry maintainer, enterprise security, spec pedant)
4. Classify findings by consensus strength (5/5, 4/5, 3/5, single-persona)
5. Fix consensus items, add single-persona items as open questions

This caught issues that individual review would miss — the content_hash ambiguity was flagged by all 5 personas from different angles. The hook spec could benefit from the same treatment once the behavioral research is complete. The 5 personas would shift slightly:
- **Hook author** (writes hooks for policy enforcement)
- **Agent developer** (implements hook execution in their agent)
- **Enterprise security** (evaluates hook RCE risk for their org)
- **Spec pedant** (precision, testability, conformance)
- **Distribution tool developer** (converts hooks between agents — syllago's perspective)

---

## Summary: Action Items for Hook Research

| # | Action | Priority | When |
|---|--------|----------|------|
| 1 | Compile security tier table (gated/permissioned/sandboxed/open) from D1+E1+E2 data | High | Phase 2-3 |
| 2 | Flag identifier alignment mismatches in spec validation report | High | Phase 1 |
| 3 | Check if hook spec matchers use glob-like syntax needing portability treatment | Medium | Phase 1 |
| 4 | Check compaction protection for hook-injected context (D-category addition) | Medium | Phase 2 |
| 5 | Recommend hash algorithm alignment with metadata convention | Medium | Phase 6 |
| 6 | Draft behavioral assumptions checklist for hook `supported_agents` equivalent | Low | Phase 6 |
| 7 | Consider 5-persona review on hook spec after research completes | Low | Post-Phase 6 |
