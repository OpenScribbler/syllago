# Hook Interchange Format Specification

The Hook Interchange Format (HIF) Specification defines a provider-neutral canonical format for AI coding tool hooks — the scripts and commands that execute before and after tool calls, file edits, shell commands, and other coding assistant actions. The specification covers the canonical data model, event naming conventions, tool vocabulary, conversion pipeline rules, conformance levels, and an enterprise policy interface. Its goal is to make hooks portable: a hook written for one provider can be converted to work correctly on another without loss of behavior or security intent.

## Documents

| Document | Description |
|----------|-------------|
| [hooks.md](hooks.md) | Core specification: canonical format, exit codes, output schema, matchers, conversion pipeline, conformance levels |
| [events.md](events.md) | Event Registry: canonical event names and provider-native mappings |
| [tools.md](tools.md) | Tool Vocabulary: canonical tool names abstracting provider-specific naming |
| [capabilities.md](capabilities.md) | Capability Registry: optional features, support matrices, degradation strategies |
| [blocking-matrix.md](blocking-matrix.md) | Blocking Behavior Matrix: expected behavior per event-provider combination |
| [provider-strengths.md](provider-strengths.md) | Provider Strengths: what each provider does best (non-normative) |
| [glossary.md](glossary.md) | Terminology definitions |
| [security-considerations.md](security-considerations.md) | Security threat model and mitigations |
| [policy-interface.md](policy-interface.md) | Enterprise policy enforcement interface contract |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute to this specification |
| [CHANGELOG.md](CHANGELOG.md) | Version history |
| [test-vectors/](test-vectors/) | Conformance test vectors with format documentation |

## Getting Started

**Building a converter?**
Start with [hooks.md](hooks.md) for the canonical data model, exit code semantics, and conversion pipeline rules. Then read [events.md](events.md) and [tools.md](tools.md) for the provider-native name mappings your converter will need.

**Adding a new provider?**
Start with [events.md](events.md) to define the event mappings for your provider's hook lifecycle, then [tools.md](tools.md) to register the tool names your provider uses. Review [capabilities.md](capabilities.md) to document which optional features your provider supports.

**Looking for reference code?**
See [examples/](examples/) for Python and TypeScript implementations demonstrating canonical format parsing, event mapping, and round-trip conversion.

## Version and Status

**Version:** 0.1.0 — Initial Development

**License:** [CC-BY-4.0](LICENSE)

This specification is in early development. Interfaces and formats may change before a stable 1.0 release. See [CHANGELOG.md](CHANGELOG.md) for version history and [CONTRIBUTING.md](CONTRIBUTING.md) to participate in the specification process.
