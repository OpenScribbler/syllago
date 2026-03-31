# Hook Interchange Format Specification v1.0.0-draft -- Technical Writer Editorial Review

**Reviewer persona:** Senior Technical Writer with 12 years of experience at Stripe, Twilio, and Vercel. Specializes in API references, specification documents, and developer guides considered industry benchmarks. Focused on clarity, consistency, information architecture, progressive disclosure, and audience awareness.

---

## 1. Overall Assessment

**Grade: B+**

This is a well-structured, technically sound specification that demonstrates genuine domain expertise. The author clearly understands both the problem space (cross-provider hook portability) and the discipline of specification writing (RFC 2119 keywords, explicit conformance levels, separation of normative and informative content). The hub-and-spoke conversion model is cleanly articulated, the capability/degradation system is thoughtfully designed, and the test vectors are genuinely useful.

What keeps this from an A: a handful of ambiguities that would block an independent implementor, inconsistencies between the spec and its companion documents, places where the spec assumes context the reader does not have, and some structural choices that scatter related information across sections when colocation would serve better. The spec reads like a strong v0.9 that needs one more editorial pass before it can be implemented without asking the authors clarifying questions.

## 2. Audience Analysis

**Primary audience:** Developers building adapters (encoders/decoders) for specific AI coding tool providers. These are the people who will implement the conversion pipeline described in Section 12.

**Secondary audience:** Hook authors who want to write portable hooks, and enterprise architects evaluating whether the format meets their policy requirements.

**Is the spec written for them?** Mostly yes, with gaps:

- **Adapter implementors** get strong guidance on event mapping, tool mapping, and degradation. However, they get almost no guidance on the actual native format structures they need to produce. The test vectors partially fill this gap, but the spec never formally describes what a Claude Code hook config looks like, what a Gemini CLI hook config looks like, etc. An implementor needs to cross-reference external provider documentation. This is probably intentional (the spec is about the interchange format, not native formats), but it should be stated explicitly.

- **Hook authors** are somewhat underserved. The spec talks about manifests, capabilities, and degradation strategies, but never walks through the authoring experience: "Here is how you write a hook, what you put in the manifest, and what happens when your hook is converted to a provider that lacks a feature you depend on." The examples in Section 3.7 and 3.8 help, but a narrative walkthrough is absent.

- **Enterprise architects** are well-served by the policy-interface.md document, though it is explicitly an interface stub, not a full specification.

## 3. Information Architecture

### Strengths

- The table of contents provides excellent navigability.
- The "What This Spec Defines" / "What This Spec Does Not Define" section (1.1/1.2) sets boundaries clearly.
- The progression from format (Section 3) to semantics (Sections 4-6) to registries (Sections 7-10) to conversion (Sections 11-12) to conformance (Section 13) is logical.
- Splitting the glossary, policy interface, and security considerations into separate documents is the right call.

### Weaknesses

- **The output schema (Section 5) comes before the matcher types (Section 6).** A reader encountering hooks for the first time would benefit from seeing "what events do hooks bind to" and "what tools do they filter" before "what does the hook's stdout look like." The current order (format -> exit codes -> output -> matchers -> events) front-loads implementation details before establishing the conceptual model. A more natural order: format -> events -> matchers -> exit codes -> output -> capabilities -> degradation -> conversion -> conformance.

- **Capability-defined output fields are split from the output schema.** Section 5.3 says "the following fields are capability-specific" and lists `updated_input`, `suppress_output`, and `system_message`, but their actual definitions live in Section 9. A reader looking at hook output will need to jump to Section 9.1 and 9.2 to understand what these fields contain. Consider at minimum adding a forward-reference table with types.

- **The Tool Vocabulary (Section 10) and MCP encoding table (Section 6.3) are duplicated.** The MCP format table appears identically in both Section 6.3 and Section 10.2. Duplication creates a maintenance risk -- if one is updated and the other is not, implementors get conflicting information.

## 4. Clarity and Precision

### Ambiguities that would block an implementor

**4.1 "Relative to the hook directory" vs "relative to the project root"**

Section 3.5, `command` field: "Shell command or script path, relative to the hook directory."
Section 3.5, `cwd` field: "Working directory for the hook process, relative to the project root."

These use different base paths without defining either term. What is "the hook directory"? Is it the directory containing the manifest? The directory containing the script? What is "the project root"? The git root? The directory containing `.claude/` or `.gemini/`? An implementor cannot resolve these paths without guessing.

**Suggested fix:** Define "hook directory" and "project root" in the glossary, or replace with unambiguous language: "relative to the directory containing the hook manifest file" and "relative to the repository root (the directory containing `.git/`)."

**4.2 What happens when `decision` is absent?**

Section 5.1 defines `decision` as OPTIONAL with values `"allow"`, `"deny"`, or `"ask"`. Section 5.2 describes interactions between exit codes and `decision`. But what happens when a hook exits with code 0, produces valid JSON, and the `decision` field is absent? Is it treated as `"allow"`? The spec never says.

**Suggested fix:** Add: "When the `decision` field is absent and the exit code is 0, the implementation MUST treat the result as `decision: \"allow\"`."

**4.3 Pattern matcher: matched against what?**

Section 6.2: "An object with a `pattern` key specifies a regular expression matched against the provider-native tool name."

This is confusing in the context of a canonical format. If I am writing a canonical manifest with `"matcher": {"pattern": "file_(read|write|edit)"}`, am I matching against canonical tool names or provider-native tool names? The sentence says "provider-native," but the manifest is provider-neutral. When does the matching actually happen -- at decode time, at encode time, or at runtime?

**Suggested fix:** Clarify: "The pattern is applied at runtime by the target provider against that provider's native tool names. During encode, the adapter preserves the pattern as-is unless it can be translated to an equivalent provider-native construct. Hook authors writing pattern matchers SHOULD use canonical tool names; adapters MAY translate patterns to match provider-native names during encode."

**4.4 Blocking behavior for `after_tool_execute` with exit code 2**

Section 8.2 shows `after_tool_execute` as `observe` across all providers. Section 4 says exit code 2 is only meaningful when `blocking` is `true`. But the spec never explicitly says what happens if an `after_tool_execute` hook has `blocking: true` and exits with code 2. Is `blocking: true` invalid on `after_tool_execute`? Is it silently ignored? Is it a validation error?

**Suggested fix:** Add normative text: "Implementations SHOULD warn when `blocking: true` is set on events where the blocking behavior is `observe` for all providers."

### Minor ambiguities

**4.5** Section 3.4: "The `capabilities` field MAY appear on a hook definition." This field is not listed in the table immediately above it. Either add it to the table or move this paragraph before the table with a note that it is informational.

**4.6** Section 9, capability inference: "Tooling infers this capability when the hook is known to produce JSON stdout." How is tooling "known" to produce JSON? By static analysis of the script? By the hook author declaring it? This inference rule is circular -- tooling can only know the hook produces structured output by running it or by the author declaring the capability. The other inference rules (checking manifest fields) are clean. This one is not.

**4.7** Section 14.1: "The `spec` field in manifests includes only major and minor versions. Implementations MUST accept any patch version for a given major.minor." But the `spec` field format is `"hooks/1.0"` which has no patch component. You cannot "accept any patch version" of something that has no patch version. The sentence seems to be about the specification version (1.0.0-draft), not the manifest field value.

## 5. Consistency

### Terminology

- The term "hook" is used consistently throughout.
- "Provider" and "adapter" are used correctly and consistently.
- "Canonical" is well-defined and consistently applied.

### Formatting

- Tables are consistently formatted with pipes and headers.
- JSON examples use consistent 2-space indentation.
- RFC 2119 keywords are consistently capitalized.

### Inconsistencies found

**5.1 `osx` vs `macos`:** The platform field uses `"osx"` (Section 3.5, JSON Schema), but the glossary does not define this value, and the Copilot CLI maps platform commands to `"bash"/"powershell"` (not `"windows"/"linux"/"osx"`). The schema description calls it "macOS-specific" but the key is `osx`. Pick one name and use it everywhere. Modern Apple developer documentation uses "macOS," and `GOOS` in Go uses `darwin`. Using `osx` (a brand name retired in 2016) is surprising.

**5.2 Blocking field in test vectors:** The Claude Code test vectors do not include any `blocking` field in the output. The canonical simple-blocking.json has `"blocking": true`, but the Claude Code output has no trace of this. Either Claude Code infers blocking from the event type, or the test vector is incomplete. The spec should clarify how blocking intent is encoded for providers that do not have an explicit blocking field.

**5.3 Warnings format:** Test vectors include a `_warnings` array, but this format is never defined in the spec or CONTRIBUTING.md. Is `_warnings` a normative part of the test vector format? Is the underscore prefix a convention for non-normative fields? This should be documented.

**5.4 `_comment` field convention:** Every test vector uses `_comment` but this is never mentioned in the spec. Section 3.2 says implementations MUST ignore unknown fields, which covers it technically, but a test vector format specification should document its own conventions.

## 6. Completeness

### What is missing

**6.1 No input schema.** The spec defines what hooks output (Section 5) but says almost nothing about what hooks receive as input. Section 7.1 mentions "the hook receives the tool name and input arguments" and "the hook receives the tool name, input, and result," but never defines the input JSON schema. An implementor cannot write a hook without knowing the shape of stdin or environment variables that carry event data. This is arguably the single largest gap in the spec.

**Suggested fix:** Add a Section 4.5 or similar: "Hook Input Contract." Define the environment variables or stdin JSON that implementations MUST provide to hooks at each event. At minimum, define the fields for `before_tool_execute` (tool name, tool input) and `after_tool_execute` (tool name, tool input, tool result).

**6.2 No error handling for malformed output.** Section 5 defines the output schema but never says what happens if stdout is not valid JSON. Is it treated as exit code 1? Is it ignored? Is the raw string used as a reason?

**Suggested fix:** Add: "When a hook exits with code 0 and stdout is not valid JSON, implementations MUST treat the result as exit code 1 (hook error) and SHOULD log stderr and stdout for debugging."

**6.3 No signal handling / process termination.** Section 4 mentions timeout behavior but does not specify how the process should be terminated. SIGTERM? SIGKILL? Grace period? On Windows?

**6.4 No guidance on concurrent hook execution.** If multiple hooks bind to the same event, do they run sequentially? In parallel? In manifest order? Can a blocking hook prevent subsequent hooks from running?

**6.5 Missing provider test vectors.** The test-vectors directory contains only claude-code, gemini-cli, and cursor. Windsurf, VS Code Copilot, Copilot CLI, Kiro, and OpenCode are missing. The CONTRIBUTING.md implies each canonical file should have a corresponding file in "each provider directory." This is the spec's own test coverage gap.

**6.6 No changelog.** Section 14.3 references a changelog for support matrices, but no changelog exists anywhere in the spec.

## 7. Examples and Test Vectors

### Strengths

- The `_comment` field in each test vector is excellent -- it explains not just what the test does but what specific conversion behaviors it exercises.
- The `_warnings` arrays in provider outputs are brilliant for implementor validation.
- The three canonical vectors (simple, multi-event, full-featured) form a good progression from minimal to complex.
- The Cursor multi-event test vector is particularly valuable because it documents the lossy conversion for a split-event provider with honest warnings about semantic approximation.

### Weaknesses

**7.1 No negative test vectors.** All three canonical files are valid manifests. There are no test vectors for:
- Invalid manifests (missing required fields)
- Unknown event names
- Conflicting exit code + decision combinations
- Hooks with unsupported events on the target provider (the `_warnings` cover this informally but there is no dedicated test)

**7.2 No round-trip test vectors.** The CONTRIBUTING.md says "For new provider mappings: a decode/encode round-trip test." But the existing test vectors only test encode (canonical -> provider). There are no provider -> canonical vectors to test decode.

**7.3 No capability-specific test vectors.** The full-featured vector exercises `platform_commands`, `custom_env`, and `configurable_cwd`, but there is no test vector for `input_rewrite` (the most safety-critical capability), `llm_evaluated`, `http_handler`, or `async_execution`. The degradation behavior for `input_rewrite: "block"` is the spec's most important safety feature and it has no test vector.

**7.4 The Cursor multi-event test vector has a semantic problem.** The `_warnings` say: "'file_write' maps to afterFileEdit (Cursor has no beforeFileEdit event; afterFileEdit is the closest match but fires after, not before)." This means a blocking pre-execution safety hook is being converted to a post-execution observational hook. The warning acknowledges the problem but the conversion still happens. Should this trigger the degradation strategy instead? The spec does not address this class of semantic mismatch (timing shift, not capability loss).

## 8. Cross-Document Coherence

**The four documents tell a coherent story overall.** The main spec defines the format, the glossary defines terms, the security doc analyzes threats, and the policy doc defines the enterprise surface area. Each document correctly references the others.

**Specific coherence issues:**

**8.1** The security document (Section 3.2) references a `signatures` field: "The specification defines an optional `signatures` field for cryptographic signatures." But the main spec never defines this field. It is not in the canonical format (Section 3), not in the JSON Schema, and not in the glossary. Either the security doc is forward-referencing a planned feature, or the main spec is missing a field definition.

**8.2** The security document (Section 3.3) references `author` metadata fields. Same problem -- these are not defined anywhere in the main spec or schema.

**8.3** The policy interface references "managed channels" and "managed sources" but never defines what makes a source "managed." It says "the definition of 'managed' is implementation-specific," which is fine for an interface contract, but could lead to incompatible policy implementations.

**8.4** The glossary defines "round-trip" but the term does not appear in the main spec's normative text. The conformance level (Section 13.3) uses "round-tripping" but the glossary entry says "round-trip" (noun vs. gerund). Minor, but a terminology-obsessed reader will notice.

## 9. Comparison to Exemplary Specs

**vs. RFC style (e.g., RFC 7231 HTTP/1.1):**
The spec correctly uses RFC 2119 keywords and the "Status of This Document" convention. It lacks IANA-style registry management procedures -- who approves new events or capabilities? The CONTRIBUTING.md partially covers this, but an RFC would have a formal registry policy.

**vs. OpenAPI Specification:**
OpenAPI excels at providing a machine-readable schema alongside human-readable prose, with the schema being authoritative. This spec has a JSON Schema (Appendix B) but the prose explicitly says "normative text takes precedence over the JSON Schema in case of conflict." This is pragmatic but means the schema is technically informational, which reduces its value for automated tooling. OpenAPI also provides comprehensive examples for every construct; this spec has examples for the top-level structure but not for individual capabilities.

**vs. JSON Schema Specification:**
JSON Schema is famously self-hosted (the spec is itself a JSON Schema). This spec does not attempt that, which is fine. But JSON Schema provides formal definitions using keywords like "MUST produce," "MUST accept," and "the result of validation is." This spec sometimes slips into descriptive rather than prescriptive language (e.g., "The regex flavor SHOULD be RE2" -- is it RE2 or not?).

**vs. LSP (Language Server Protocol):**
LSP excels at defining request/response pairs with precise TypeScript interfaces. This spec would benefit from a similar approach for the hook input contract (what data does the hook process receive?) and output contract (already partially defined). LSP also provides a clear versioning story for capabilities; this spec's capability versioning is handled by date-stamped registries, which is workable but less precise.

**Overall positioning:** This spec is better organized than most open-source specifications and compares favorably to mid-tier IETF drafts. It is not yet at the level of OpenAPI or LSP in terms of precision, but the gap is closeable with the edits outlined here.

## 10. Line-Level Edits

**10.1** Section 1, paragraph 2:

> "Every provider's hook system is different. Event names, JSON schemas, output contracts, matcher syntax, timeout units, and configuration file layouts vary across providers."

This is a strong opening, but the second sentence is a list dump. Rewrite to show rather than tell:

> "Every provider's hook system is different. Claude Code calls it `PreToolUse`; Gemini CLI calls it `BeforeTool`; Cursor splits it into `beforeShellExecution` and `beforeMCPExecution`. One provider measures timeouts in seconds, another in milliseconds. A hook written for one tool cannot run on another without manual adaptation."

**10.2** Section 3.1:

> "conforming implementations MUST accept both JSON and YAML representations and MUST produce identical canonical structures from either."

The phrase "identical canonical structures" is ambiguous. Do you mean byte-identical JSON output? Semantically equivalent structures? What about YAML-specific features (anchors, tags, multi-line strings)? Clarify:

> "conforming implementations MUST accept both JSON and YAML representations and MUST produce semantically equivalent canonical structures from either. YAML-specific features (anchors, aliases, custom tags) that have no JSON equivalent MUST NOT be used in canonical manifests."

**10.3** Section 3.4, `provider_data` description:

> "Opaque provider-specific data, keyed by provider slug."

The word "keyed" assumes familiarity with JSON object terminology. More precise:

> "Opaque provider-specific data. Keys are canonical provider slugs (Section 3.6); values are JSON objects whose schemas are defined by the respective providers."

**10.4** Section 4, exit code 0 description:

> "Empty stdout is treated as `{}`."

This statement has no RFC 2119 keyword. Is it normative? It should be:

> "Empty stdout MUST be treated as equivalent to `{}`."

**10.5** Section 9.3:

> "LLM evaluation IS the hook"

The capitalized "IS" for emphasis is informal and inconsistent with the rest of the document's tone. Rewrite:

> "LLM evaluation constitutes the hook's entire functionality -- there is no deterministic fallback."

**10.6** Section 7.4, the `--` convention:

> "A `--` indicates the provider does not support that event."

This convention is introduced here but used earlier in the Tool Vocabulary (Section 10.1). Define it once in a conventions section or at first use.

## 11. Recommendations (Prioritized)

### P0 -- Must fix before any implementation work

1. **Define the hook input contract.** Add a section specifying what data hooks receive (stdin JSON schema and/or environment variables) per event type. Without this, the spec is incomplete -- it defines outputs but not inputs.

2. **Resolve the `signatures` and `author` phantom fields.** Either add these to the main spec and schema, or remove the references from security-considerations.md. The current state implies features that do not exist.

3. **Clarify path resolution.** Define "hook directory" and "project root" unambiguously, either in the glossary or inline in Section 3.5.

4. **Define behavior for absent `decision` field** when exit code is 0 and JSON output is present.

5. **Define behavior for malformed stdout** (not valid JSON) when exit code is 0.

### P1 -- Should fix before draft finalization

6. **Add hook execution ordering semantics.** Specify whether multiple hooks on the same event run sequentially or in parallel, and whether manifest order is significant.

7. **Add negative test vectors** for invalid manifests and conflicting exit code/decision combinations.

8. **Add round-trip (decode) test vectors** -- at least one provider-native -> canonical vector per provider.

9. **Add an `input_rewrite` degradation test vector.** This is the spec's most critical safety feature and it has zero test coverage.

10. **Remove the MCP format table duplication** between Section 6.3 and Section 10.2. Keep it in one place and cross-reference.

### P2 -- Should fix before 1.0 status

11. **Reorder sections** to present the conceptual model (events, matchers) before implementation details (exit codes, output schema).

12. **Add per-capability examples** showing a canonical hook that uses each capability and the expected encode output for at least two providers.

13. **Expand test vectors** to cover all eight providers, not just three.

14. **Add a "Hook Authoring Guide"** as a non-normative companion document (or appendix) that walks through writing a portable hook from scratch.

15. **Modernize `osx` to `macos` or `darwin`** in the platform field, or at minimum document why the outdated name was chosen.

16. **Define the `_comment` and `_warnings` conventions** used in test vectors, either in CONTRIBUTING.md or in a test-vector README.

17. **Address the timing-shift degradation gap** exposed by the Cursor multi-event test vector (a before-event hook being mapped to an after-event is a semantic loss more severe than a capability gap).

18. **Clarify the `structured_output` inference rule** -- "known to produce JSON stdout" is circular without defining how tooling acquires this knowledge.

---

This spec is genuinely good work and clearly benefits from the author understanding both the domain deeply and the conventions of specification writing. The gaps identified above are the kind that surface when moving from "spec that the author can implement" to "spec that a stranger can implement" -- which is exactly the right time to catch them.
