# Changelog

All notable changes to the Hook Interchange Format Specification.

## [1.0.0-draft] - 2026-03-27

### Clarified
- Exit code / decision / blocking interaction order with truth table (Section 5.3)
- Behavior when `decision` field is absent (defaults to `"allow"`)
- Behavior when stdout is not valid JSON (treated as exit code 1)
- `timeout: 0` means no timeout (implementation default applies)
- Empty `hooks` array MUST be rejected as validation error
- Blocking on observe-only events: not an error, SHOULD warn
- `structured_output` inference rule (no longer circular)
- Patch version sentence in Section 14.1
- Pattern matchers matched at runtime by target provider
- MCP format table consolidated to Section 6.3 (removed duplication from Section 10.2)
- Blocking-on-observe warning upgraded from SHOULD to MUST

### Added
- `capabilities` field added to hook definition table (Section 3.4)
- Note on future promotion of `timeout_behavior` to canonical field
- Glossary entries: "hook directory", "project root", "structurally equivalent"
- Security considerations: prompt injection via hook output (Section 4.1)
- Security considerations: transitive prompt injection via LLM hooks (Section 4.2)
- Security considerations: async execution observability (Section 4.3)
- Security considerations: policy file integrity (Section 4.4)
- CONTRIBUTING.md: `_comment` and `_warnings` test vector conventions
- CONTRIBUTING.md: schema validation commands
- JSON Schema: `examples` arrays for `event` and handler `type` fields

### Fixed
- Phantom `signatures` and `author` field references in security-considerations.md marked as "(Future)"
- Section numbering in security-considerations.md after adding Section 4
