# Hook Interchange Format Specification v1.0.0-draft -- Open Source Maintainer Review

**Reviewer persona:** Senior Open Source Maintainer with multiple popular developer tools (5,000+ GitHub stars). Maintained ESLint plugins, Prettier configs, Homebrew formulas, and language server protocols. Focused on community adoption, contributor experience, backwards compatibility, and the "pit of success."

---

## 1. Executive Summary

This is an unusually well-structured draft specification for a cross-provider hook interchange format. The separation of concerns (canonical format, capability model, degradation strategies, conformance levels) shows real systems thinking. However, there are several underspecified areas that would lead two independent implementors to produce incompatible adapters, and the spec conflates "living reference document" with "normative specification" in ways that will create versioning headaches as the ecosystem grows.

## 2. Strengths

**Hub-and-spoke architecture is the right call.** Rather than defining N*(N-1) provider-to-provider translations, the spec defines N decode + N encode adapters through a canonical hub. This is the same pattern that made Pandoc successful for document conversion and LLVM successful for compilers. It scales linearly with new providers.

**The degradation model is thoughtful and safety-aware.** The `input_rewrite` capability defaulting to `block` (Section 9.2, line 514) rather than `warn` demonstrates real security thinking. The reasoning -- "silent drop of input sanitization creates a false sense of security" -- is exactly right. Most specs punt on this; this one gets it correct on the first try.

**Forward compatibility is baked in.** Section 3.2 (line 82): "Implementations MUST ignore unknown fields at any level." This is the single most important sentence for ecosystem longevity and it appears early. Combined with `additionalProperties: true` throughout the JSON schema, this puts the spec in the "pit of success" for forward compatibility.

**The exit code contract is simple and unambiguous.** Section 4 defines exactly four behaviors for four exit code classes. No overloading, no provider-specific interpretations. The interaction matrix with `decision` field in Section 5.2 is explicit about precedence. This is the kind of clarity that prevents implementation divergence.

**The test vectors include warnings metadata.** The `_warnings` arrays in provider output files (e.g., `claude-code/full-featured.json` lines 4-8) document what information was lost during conversion. This is unusual and valuable -- it makes degradation behavior testable, not just the structural output.

**CC-BY-4.0 licensing is correct for a spec.** Not MIT/Apache (which are for code), not GPL (which would deter enterprise adoption). CC-BY-4.0 is exactly what you want for a specification document that you want implementors to build against freely.

## 3. Adoption Barriers

**No reference implementation or compliance test suite.** The CONTRIBUTING.md (line 65) says "at least one adapter implementation must demonstrate the change" but there is no reference adapter shipped with the spec. Compare this to LSP, which shipped with a VS Code reference implementation, or JSON Schema, which maintains a validator suite. Without a reference implementation, every implementor is guessing independently. The test vectors are a start, but they are hand-authored expected outputs, not executable tests.

**Only three provider output directories in test vectors.** The spec defines eight providers but only ships vectors for `claude-code`, `gemini-cli`, and `cursor`. Windsurf, VS Code Copilot, Copilot CLI, Kiro, and OpenCode have no test vectors at all. An implementor building a Windsurf adapter has zero ground truth to test against. The CONTRIBUTING guide says new providers need test vectors (line 91), but the spec itself does not meet its own bar for five of eight providers.

**The spec is authored by and ships inside syllago.** The `$id` in the JSON schema points to `https://syllago.dev/spec/hooks/1.0/hook.schema.json` (schema line 3). For community adoption, having the spec live inside one tool's repository creates a perception problem. Other potential implementors (a Cursor plugin author, a Gemini CLI contributor) may see this as "syllago's format" rather than "the community format." LSP solved this by being a standalone spec from day one. EditorConfig solved it by having a dedicated `.editorconfig` organization. Consider whether the spec should live in its own repository with its own domain.

**No machine-readable registry.** The Event Registry (Section 7.4), Tool Vocabulary (Section 10.1), Capability Registry (Section 9), and Blocking Behavior Matrix (Section 8.2) are all markdown tables. An implementor must parse prose to build their adapter. Compare to LSP, which ships `protocol.md` alongside machine-readable TypeScript definitions. These tables should be available as JSON/YAML data files that adapters can consume programmatically.

## 4. Spec Ambiguities

### 4a. Cursor's native format is underspecified

Look at the Cursor test vectors: `simple-blocking.json` (line 4) uses `"hooks": {"beforeShellExecution": {"command": ...}}` (a single object), while `multi-event.json` (line 14) uses `"afterFileEdit": [{"command": ...}, {"command": ...}]` (an array). Is the Cursor native format an object when there is one hook and an array when there are multiple? Or is this inconsistency in the test vectors? Two implementors would handle this differently. The spec needs to define the expected native output schema for each provider, or explicitly declare that it does not normatively define provider-native formats.

### 4b. What counts as "structurally equivalent" for round-trip verification?

Section 12.4 (line 745) says "event count, handler types, command strings, matcher presence" but this is an informative list, not a normative definition. Does field ordering matter? Does whitespace matter? What about `_comment` fields that appear in test vectors but are not in the schema? If I decode and re-encode a Claude Code hook, and the output is semantically identical but has different key ordering, is that a conformance failure? Full conformance requires round-trip fidelity (Section 13.3, line 777) but "structurally equivalent" is not defined in the glossary.

### 4c. The `_comment` field in test vectors has no schema backing

Every test vector includes `_comment` (e.g., canonical/simple-blocking.json line 2). This is a de facto convention but is not mentioned in the spec, the schema, or the CONTRIBUTING guide. Are implementations expected to preserve it? Ignore it? The schema allows it via `additionalProperties: true`, but implementors will ask.

### 4d. Matcher encoding for split-event providers with mixed matchers

The multi-event Cursor test vector (lines 4-5) shows an array matcher `["shell", "file_write"]` being split into two separate Cursor events. But what happens with `["shell", {"mcp": {"server": "github"}}]`? Shell maps to `beforeShellExecution`, MCP maps to `beforeMCPExecution`. The spec (Section 7.4, line 426) says "adapters MUST inspect the `matcher` field to select the correct native event" but does not specify whether a single canonical hook becomes multiple native hooks when the array matcher spans multiple event categories. The Cursor multi-event test vector implies yes, but this should be normative text, not left to test vector inference.

### 4e. Regex flavor for pattern matchers

Section 6.2 (line 304) says "The regex flavor SHOULD be RE2." This is a SHOULD, not a MUST. Two implementations -- one using RE2, one using PCRE -- could produce different matching results for the same pattern. For example, `(?<=foo)bar` is valid PCRE but invalid RE2. A hook author writing a pattern matcher cannot know which regex flavor will be used. This should either be MUST RE2 (and implementations that cannot support RE2 must document deviations) or the spec should define a restricted subset.

### 4f. Empty `hooks` array ambiguity

The schema (line 17) specifies `"minItems": 1` for the hooks array, and the spec (Section 3.3, line 92) says "Non-empty array." But what should an implementation do when it receives an empty array? Reject with an error? Treat as a no-op? The schema validation would catch this, but not all implementations will validate against the schema. The spec should state normative behavior for this case.

### 4g. Timeout of 0

The schema allows `"minimum": 0` for timeout (line 172). Does timeout 0 mean "no timeout" or "immediately time out"? The spec does not define this edge case. Most shell timeout implementations treat 0 as "no timeout" but this is not universal.

## 5. Versioning & Evolution

**The three-artifact versioning model is sound but has a gap.** The spec version (semver in `spec` field), registry versions (date-stamped), and support matrices (unversioned, living) are a good decomposition. However:

- There is no mechanism for a manifest to declare which registry version it was authored against. If the Tool Vocabulary adds a new canonical name `container` in registry version `2026.06`, and a manifest uses `"matcher": "container"`, an implementation running registry version `2026.03` will encounter an unknown tool name. The spec says nothing about this case. Should the implementation reject it? Pass it through as a literal string? The forward-compatibility rule (Section 3.2) says "ignore unknown fields" but a bare string matcher is a value, not a field.

- The `spec` field uses `"hooks/<major>.<minor>"` which omits the patch version (Section 14.1, line 796). This is good -- patch versions should not be in manifests. But the spec document header says `Version: 1.0.0-draft` (line 3). The `-draft` suffix has no handling in the spec field pattern. What does a manifest set for `spec` during the draft period? `"hooks/1.0"` implies finality. Consider `"hooks/1.0-draft"` and updating the schema pattern to allow it.

**No deprecation mechanism.** If a canonical event name needs to change (e.g., `agent_stop` is renamed to `agent_idle` in a future version), there is no mechanism for soft deprecation. Compare to LSP, which has a `deprecated` field on capabilities, or OpenAPI, which has `deprecated: true` on operations. A minor version bump that adds a replacement name should be able to mark the old name as deprecated without breaking existing manifests.

## 6. Contributor Experience

**The CONTRIBUTING.md is well-organized but incomplete.** It covers registry updates, spec changes, and new providers clearly. However:

- There is no setup guide. How does a contributor run the test vectors? Is there a validation script? The CONTRIBUTING guide says "Updated test vectors if the change affects conversion behavior" (line 60) but does not explain how to create or validate them.
- There is no mention of how to run schema validation. The schema exists but there is no `make validate` or equivalent.
- The "Adding a New Provider" section (line 74) is good but could use a worked example showing the full PR for a hypothetical provider.

**Test vector coverage is thin.** Three canonical manifests is not enough. Missing test cases:

- A hook with `type: "http"` (tests http_handler capability)
- A hook with `type: "prompt"` or `type: "agent"` (tests llm_evaluated capability)
- A hook with `async: true` (tests async_execution capability)
- A hook targeting a provider-exclusive event (e.g., `before_model` for Gemini CLI)
- A hook with `decision: "ask"` output (tests the fallback to deny)
- A hook with conflicting exit code and JSON decision (tests Section 5.2 precedence rules)
- Negative test vectors: invalid manifests that MUST be rejected
- Round-trip test vectors: canonical -> provider -> canonical, verifying structural equivalence

The current vectors only test `type: "command"` handlers, which means the entire capability system for http/prompt/agent handlers has zero test coverage.

## 7. Comparison to Similar Specs

**vs. LSP (Language Server Protocol):** LSP succeeded because it shipped with a reference implementation (VS Code) and a compliance test suite from day one. The hooks spec has the same quality of prose specification but lacks the executable artifacts. LSP also defined its transport layer (JSON-RPC over stdio); the hooks spec wisely avoids defining transport, which is appropriate since hooks are not a protocol but a data format.

**vs. EditorConfig:** EditorConfig is simpler (key-value properties) but achieved massive adoption by being tool-agnostic from the start. The hooks spec has the same aspiration but is more complex by necessity (the problem space demands it). EditorConfig's test suite (`editorconfig/editorconfig-core-test`) was critical for adoption -- every editor plugin could verify compliance against a shared suite. The hooks spec needs an equivalent.

**vs. .tool-versions (asdf/mise):** The `.tool-versions` format is a cautionary tale in underspecification. Multiple implementations (asdf, mise, rtx) each interpret edge cases differently. The hooks spec is far more rigorous than `.tool-versions` ever was, which is encouraging. The explicit conformance levels (Section 13) are a particularly good mechanism that `.tool-versions` lacked entirely.

**vs. OpenAPI/Swagger:** OpenAPI succeeded partly because of the JSON Schema and validator ecosystem. The hooks spec ships a JSON Schema, which is good. But OpenAPI also shipped code generators and a linter (spectral). The hooks spec would benefit from a standalone validator that goes beyond JSON Schema (checking event names against the registry, validating matcher types against the tool vocabulary, etc.).

## 8. Recommendations (Prioritized)

### P0 -- Ship before announcing

1. **Add a `hooks/1.0-draft` spec identifier.** The current `"hooks/1.0"` implies a finalized spec. Until the status changes from Draft, manifests should use a draft identifier so that implementations can distinguish draft-era manifests from final ones. Update the schema pattern to `^hooks/\\d+\\.\\d+(-draft)?$`.

2. **Define "structurally equivalent" in the glossary.** At minimum: same event count, same events, same handler types, same command strings, same matcher semantics. Explicitly exclude: field ordering, whitespace, comments.

3. **Specify behavior for unknown bare string matchers.** When a tool vocabulary lookup fails (the bare string does not match any canonical tool name), MUST the implementation reject it, pass it through as a literal, or warn? This is a forward-compatibility question that will bite every implementor.

### P1 -- Ship within first month

4. **Publish machine-readable registries.** Create `registry/events.json`, `registry/tools.json`, `registry/capabilities.json` alongside the spec. Adapters should be able to import these directly rather than parsing markdown tables.

5. **Expand test vectors to cover capabilities.** Add at least: one http_handler vector, one async vector, one provider-exclusive event vector, one array-matcher-on-split-event vector, and a set of negative/invalid test vectors.

6. **Add a round-trip test vector.** For at least one provider, include canonical -> provider -> canonical and show that the result is "structurally equivalent."

7. **Strengthen regex requirement from SHOULD to MUST.** Either mandate RE2 or define a normative subset. A SHOULD on pattern matching semantics is a WILL-diverge in practice.

### P2 -- Ship before stable release

8. **Ship a standalone schema validator.** Beyond JSON Schema validation, check event names, tool names, degradation keys, and capability identifiers against the registries. This could be a small Go or Node CLI.

9. **Consider hosting the spec independently.** Even if syllago is the first implementor, a spec hosted at `github.com/hooks-spec/hooks-v1` (or similar) has better optics for community adoption than one nested inside a specific tool's repo.

10. **Add a deprecation mechanism.** At minimum, allow registry entries to carry a `deprecated` flag and a `replacement` pointer. This will be needed the first time any provider renames an event.

11. **Define the `_comment` convention.** Either make it a recognized informational field (mentioned in Section 3 alongside `provider_data`) or remove it from the test vectors. Its current status as an informal convention will confuse contributors.

12. **Add a "hook identity" field.** The spec has no way to name or identify a hook within a manifest. When a manifest has six hooks and the adapter emits warnings, it refers to "Hook 1" and "Hook 3" by position index (see test vector warnings). A `name` or `id` field on hook definitions would make warnings, logging, and policy references much more useful. This can be an optional field added in a minor version bump.

---

**Bottom line:** This is a high-quality draft that demonstrates genuine understanding of the cross-tool interoperability problem space. The degradation model and conformance levels are particularly well-designed. The primary gap is the absence of executable test infrastructure -- the spec reads well but an implementor cannot yet mechanically verify compliance. Address the P0 items before any public announcement, the P1 items before inviting third-party implementors, and the P2 items before removing the "draft" status.
