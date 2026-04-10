# Contributing to the Hook Interchange Format Specification

Thank you for your interest in improving the Hook Interchange Format Specification. This document explains how to propose changes, add provider mappings, and extend registries.

---

## Types of Contributions

### Registry Updates (additive, no spec version bump)

- Adding a new provider's event name mappings to the Event Name Mapping table
- Adding a new tool to the Tool Vocabulary
- Adding a new MCP tool format to the MCP encoding table
- Updating the Blocking Behavior Matrix when a provider changes behavior
- Adding a new capability to the Capability Registry
- Updating provider support matrices

### Specification Changes (may require version bump)

- Adding new optional fields to the canonical format (minor version bump)
- Changing the semantics of existing fields (major version bump)
- Removing or renaming fields (major version bump)
- Changes to the exit code contract (major version bump)
- Changes to conformance level requirements (major version bump)

### Non-Normative Changes (no version bump)

- Fixing typos or clarifying wording
- Adding examples
- Updating the glossary
- Improving security considerations

---

## Process

### 1. Propose

Open a GitHub issue or Discussion describing:
- **What** you want to change
- **Why** the change is needed
- **Which providers** are affected (if applicable)
- **Evidence** from provider documentation (links, screenshots, or excerpts)

For registry updates, include the specific rows or entries to add/modify.

### 2. Discuss

Community feedback is welcomed, especially from:
- Provider implementors who can confirm native format details
- Hook authors who can validate the authoring experience
- Enterprise users who can assess policy implications

Discussion should aim for rough consensus. Contentious changes may require more evidence or a longer discussion period.

### 3. Implement

Submit a pull request containing:
- Changes to the relevant specification or registry documents
- Updated test vectors if the change affects conversion behavior
- A reference implementation demonstrating that the change works in at least one adapter

### 4. Validate

Before merge, at least one adapter implementation must demonstrate the change:
- For new provider mappings: a decode/encode round-trip test
- For new capabilities: an inference rule and degradation test
- For format changes: updated JSON Schema and test vector validation

---

## Adding a New Provider

To add support for a new AI coding tool provider:

1. **Choose a slug.** Provider slugs are lowercase, hyphenated identifiers (e.g., `my-tool`). Slugs MUST be unique and SHOULD match the tool's common name.

2. **Document event mappings.** For each canonical event the provider supports, provide:
   - The provider's native event name
   - Whether the event supports blocking
   - The blocking behavior (prevent, retry, or observe)

3. **Document tool mappings.** For each canonical tool the provider supports, provide:
   - The provider's native tool name
   - The MCP tool name format (if applicable)

4. **Document capabilities.** For each capability in the registry, indicate:
   - Whether the provider supports it
   - The provider's mechanism (field names, output format)

5. **Provide test vectors.** Create at least one canonical-to-provider and one provider-to-canonical test vector pair.

6. **Submit the PR** with all mappings, test vectors, and a brief description of the provider's hook system.

---

## Cross-File Dependencies

The specification is split across several documents. When making a change, consult the table below to ensure all affected files are updated together. Omitting a file from an otherwise complete change will leave the spec in an inconsistent state.

| Change Type | Files to Update |
|---|---|
| New event (with blocking behavior) | `events.md`, `blocking-matrix.md`, `CHANGELOG.md` |
| New event (observational only) | `events.md`, `CHANGELOG.md` |
| New provider | `events.md`, `blocking-matrix.md`, `capabilities.md`, `tools.md`, `CHANGELOG.md` |
| New capability | `capabilities.md`, `CHANGELOG.md` |
| New tool name | `tools.md`, `CHANGELOG.md` |
| Core format change | `hooks.md` (version bump), `schema/hook.schema.json`, `CHANGELOG.md` |

---

## Extending the Capability Registry

To propose a new capability:

1. **Describe the semantic intent.** A capability describes what a hook can do, not how a specific provider implements it. Ask: "Can I describe this intent without naming a provider?"

2. **Document support.** At least one provider must implement the capability. Ideally, show that two or more providers have analogous functionality.

3. **Define the inference rule.** How does tooling detect this capability from manifest fields or handler output? Capabilities should be inferable, not manually declared.

4. **Define the default degradation strategy.** Choose `block`, `warn`, or `exclude` with a rationale.

5. **Provide examples.** Show a canonical hook using the capability and the expected encoding for at least two providers.

---

## Test Vector Format

Test vectors live in `test-vectors/` with the following structure:

```
test-vectors/
  canonical/          # Canonical hook JSON files (source of truth)
  claude-code/        # Expected output for Claude Code adapter
  gemini-cli/         # Expected output for Gemini CLI adapter
  cursor/             # Expected output for Cursor adapter
  ...
```

Each canonical file has a corresponding file in each provider directory showing the expected encode output. File names should match across directories.

### Conventions

Test vectors use the following non-normative fields (prefixed with `_` to distinguish from canonical fields):

- **`_comment`**: A human-readable string explaining what the test vector exercises. Present in both canonical and provider files. Implementations MUST ignore this field (per the forward-compatibility rule in `hooks.md` Section 3.2).
- **`_warnings`**: An array of strings in provider output files documenting information lost or approximated during conversion. Used to verify that adapters produce the expected degradation warnings.

These conventions are informational. They are not part of the canonical format and MUST NOT appear in production hook manifests.

### Validating Test Vectors

To validate test vectors against the JSON Schema:

```bash
# Using ajv-cli (Node.js)
npx ajv validate -s docs/spec/schema/hook.schema.json -d "docs/spec/test-vectors/canonical/*.json"

# Using check-jsonschema (Python)
check-jsonschema --schemafile docs/spec/schema/hook.schema.json docs/spec/test-vectors/canonical/*.json
```

Provider output files are NOT validated against the canonical schema (they use provider-native formats).

---

## Style Guide

- Use RFC 2119 keywords (MUST, SHOULD, MAY) for normative statements
- Use `snake_case` for canonical field names, event names, and tool names
- Use present tense for normative text
- Include concrete examples for any new concept
- Keep the spec document focused on what, not why (rationale belongs in the design document)

---

## License

By contributing, you agree that your contributions will be licensed under [CC-BY-4.0](LICENSE).
