# Hooks Spec Modularization Design

**Date:** 2026-04-08
**Status:** Design Complete (Panel Reviewed)
**Scope:** `docs/spec/hooks/`

## Problem

The hooks interchange spec (`hooks.md`) is a 900-line monolithic file covering format definitions, registry tables, capability models, conversion pipelines, and conformance levels. This creates several problems:

- **High contribution barrier.** Someone adding a provider event mapping must read and understand the entire spec to find the right table.
- **Registry updates touch the core spec.** Adding a new tool name or event requires modifying the same file that defines the format — conflating data changes with structural changes.
- **No quick-start path.** An implementer building a basic converter has no way to know which 30% of the spec they actually need.
- **No reference implementations.** The spec defines the format but provides no runnable code showing how to implement it. Pseudocode is insufficient — only real, tested code proves the spec is implementable.

This spec will also serve as the template for future interchange specs (skills, MCP configs, rules, etc.), so getting the structure right here pays forward.

## Solution

Break the monolith into modular documents following the W3C pattern. Add a directory-level README as the index page. Add reference implementations in Python and TypeScript that consume the existing test vectors as a conformance suite.

## Design Decisions

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D1 | File structure | W3C modular pattern | Independent versioning per registry, lower contribution barrier |
| D2 | Core spec file | `hooks.md` (slimmed, renumbered §1-9) | Preserves existing links, intuitive URL |
| D3 | Directory index | New `README.md` | GitHub auto-renders on directory browse |
| D4 | Section numbering | Renumber clean (§1-9) | Self-contained reading experience; Versioning stays as own section |
| D5 | Cross-references | Relative markdown links + inline context | Best UX, works in GitHub and locally |
| D6 | Ref impl languages | Python + TypeScript | Go is covered by the reference implementation's source code |
| D7 | Ref impl scope | Core conformance kit | Parse manifest, exit codes, matchers, two provider adapters |
| D8 | Test vector integration | Ref impls consume shared `test-vectors/` | Single source of truth — standard pattern (JSON Schema, TOML) |
| D9 | Provider adapters | Claude Code + Gemini CLI | Claude Code for max capability coverage; Gemini CLI as a simpler, more pedagogical adapter for newcomers |
| D10 | Spec neutrality | No mention of syllago or author's other projects | The spec names providers (Claude Code, Gemini CLI, etc.) because that's what it's for. It does not reference syllago or any specific implementation that uses the spec. |

## Target File Structure

```
docs/spec/hooks/
├── README.md                   # NEW — directory index / landing page
├── hooks.md                    # Core spec (slimmed from ~900 to ~400 lines, renumbered §1-9)
├── events.md                   # NEW — Event Registry (extracted from current §7)
├── tools.md                    # NEW — Tool Vocabulary (extracted from current §10)
├── capabilities.md             # NEW — Capability Registry + Degradation (extracted from current §9 + §11)
├── blocking-matrix.md          # NEW — Blocking Behavior Matrix (extracted from current §8)
├── provider-strengths.md       # NEW — Companion doc, non-normative (extracted from Appendix A)
├── glossary.md                 # Existing (no changes)
├── policy-interface.md         # Existing (no changes)
├── security-considerations.md  # Existing (no changes)
├── CHANGELOG.md                # Existing (update with modularization entry)
├── CONTRIBUTING.md             # Existing (update with cross-file dependency table)
├── LICENSE                     # Existing (no changes)
├── schema/                     # Existing (no changes)
│   └── hook.schema.json
├── test-vectors/               # Existing structure, NEW README
│   ├── README.md               # NEW — format contract, pairing rules, index table
│   ├── canonical/
│   ├── claude-code/
│   ├── cursor/
│   ├── gemini-cli/
│   ├── invalid/
│   └── windsurf/
└── examples/                   # NEW — reference implementations
    ├── README.md               # How to run, how to add a language
    ├── python/
    │   ├── pyproject.toml
    │   ├── hooks_interchange/
    │   │   ├── __init__.py
    │   │   ├── manifest.py     # Parse + validate canonical manifests
    │   │   ├── exit_codes.py   # Exit code / decision resolution (§5 truth table)
    │   │   ├── matchers.py     # Matcher parsing + tool vocabulary lookup
    │   │   ├── claude_code.py  # Claude Code adapter (decode/encode)
    │   │   └── gemini_cli.py   # Gemini CLI adapter (decode/encode)
    │   └── tests/
    │       └── test_conformance.py  # Loads ../../test-vectors/, validates
    └── typescript/
        ├── package.json
        ├── tsconfig.json
        ├── src/
        │   ├── manifest.ts
        │   ├── exitCodes.ts
        │   ├── matchers.ts
        │   ├── claudeCode.ts
        │   └── geminiCli.ts
        └── tests/
            └── conformance.test.ts  # Loads ../../test-vectors/, validates
```

## Section Mapping

How current `hooks.md` sections map to the new structure:

| Current Section | Current Title | New Location | New Section # |
|----------------|---------------|--------------|---------------|
| §1 | Introduction | `hooks.md` §1 | §1 |
| §2 | Terminology | `hooks.md` §2 | §2 |
| §3 | Canonical Format | `hooks.md` §3 | §3 |
| §4 | Exit Code Contract | `hooks.md` §4 | §4 |
| §5 | Canonical Output Schema | `hooks.md` §5 | §5 |
| §6 | Matcher Types | `hooks.md` §6 | §6 |
| §7 | Event Registry | `events.md` | Own numbering |
| §8 | Blocking Behavior Matrix | `blocking-matrix.md` | Own numbering |
| §9 | Capability Registry | `capabilities.md` | Own numbering |
| §10 | Tool Vocabulary | `tools.md` | Own numbering |
| §11 | Degradation Strategies | `capabilities.md` | Merged with §9 |
| §12 | Conversion Pipeline | `hooks.md` §7 | §7 |
| §13 | Conformance Levels | `hooks.md` §8 | §8 |
| §14 | Versioning | `hooks.md` §9 | §9 |
| Appendix A | Provider Strengths | `provider-strengths.md` | Non-normative companion |
| Appendix B | JSON Schema Reference | `hooks.md` Appendix A | Stays in core |

## Cross-Reference Pattern

Each extracted document is referenced from the core spec with a relative link and one line of context explaining why the reader would follow it:

```markdown
Canonical event names and their provider-native mappings are defined in the
[Event Registry](events.md). Consult that document when implementing the
decode and encode stages of the conversion pipeline.
```

Each extracted document includes a brief header linking back to the core spec:

```markdown
> This document is part of the [Hook Interchange Format Specification](hooks.md).
> It defines the event registry referenced by the core spec's matcher and
> conversion pipeline sections.
```

## Reference Implementation Scope (Core Conformance Kit)

Each language implementation covers the Core conformance level (current §13.1):

1. **Manifest parsing** — Parse JSON and YAML canonical manifests, validate required fields, ignore unknown fields (forward compatibility)
2. **Exit code resolution** — Implement the §5.3 truth table: blocking × exit code × decision → result
3. **Matcher parsing** — Parse all matcher types (bare string, pattern, MCP, array, omitted). Resolve bare strings against tool vocabulary.
4. **Claude Code adapter** — Decode Claude Code native format → canonical. Encode canonical → Claude Code native format. Covers the most capability code paths.
5. **Gemini CLI adapter** — Decode Gemini CLI native format → canonical. Encode canonical → Gemini CLI native format. Simpler adapter, better for learning the pipeline.
6. **Conformance tests** — Load test vectors from `../../test-vectors/`, validate parsing, round-trip encoding/decoding

### What the ref impls do NOT cover

- Extended or Full conformance levels
- Capability inference or degradation strategies
- Additional provider adapters beyond Claude Code and Gemini CLI
- HTTP handler approximation (curl shim generation)
- LLM-evaluated hook handling

These are explicitly out of scope for v1. The ref impls demonstrate the core pipeline — someone can extend them for Extended/Full conformance.

## README.md Content

The directory-level `README.md` serves as the landing page when browsing `docs/spec/hooks/` on GitHub. It includes:

1. One-paragraph abstract (what this spec is)
2. Quick links table: each document with a one-line description
3. "Getting Started" section pointing to the core spec + examples
4. Version/status badge

## Extracted Document Format

Each extracted document follows a consistent structure:

```markdown
# [Title]

> Part of the [Hook Interchange Format Specification](hooks.md).

**Registry Version:** YYYY.MM (for registries)
**Last Modified:** YYYY-MM-DD
**Status:** [matches core spec status]

## [Content sections with own numbering]
```

Extracted documents do NOT have their own changelog sections. All changes are recorded in the single root `CHANGELOG.md` with section prefixes:

```
## [events] 2026.04 — Added `before_tool_selection` extended event
## [capabilities] 2026.04 — Added `http_handler` curl-shim approximation note
## [core] 0.2.0 — Modularized spec into separate documents
```

## Migration Notes

- `hooks.md` is modified in-place (slimmed, renumbered). No file rename needed.
- All new files are additions, not renames — git history for `hooks.md` is preserved.
- Internal references within the repo (CLAUDE.md, converter code comments) need updating if they cite specific section numbers.
- The CHANGELOG.md gets an entry documenting the modularization.

## Template Value

This modular structure serves as the template for future content type specs:

| Component | Hooks | Skills | MCP | Rules |
|-----------|-------|--------|-----|-------|
| Core spec (format + pipeline) | ✓ | ✓ | ✓ | ✓ |
| Event/lifecycle registry | ✓ | Maybe | No | No |
| Tool/name vocabulary | ✓ | ✓ | ✓ | ✓ |
| Capability + degradation | ✓ | Likely | Maybe | No |
| Blocking matrix | ✓ | No | No | No |
| Provider strengths | ✓ | ✓ | ✓ | ✓ |
| Test vectors | ✓ | ✓ | ✓ | ✓ |
| Reference implementations | ✓ | ✓ | ✓ | ✓ |

Hooks is the most complex interchange format because hooks are behavior (execution, blocking, timing), not just data. Skills and rules are primarily filesystem content — their specs will be shorter. MCP configs are JSON merge — also simpler.

## Spec-Craft Fixes (During Modularization)

These existing spec issues should be fixed during the modularization pass:

### Remove §5.4 (Phantom Fields)

Current §5.4 lists "advanced output fields" (`updated_input`, `suppress_output`, `system_message`) but says they're defined in the Capability Registry. After extraction, this creates an editorial dependency where the core spec references fields owned by `capabilities.md`. Fix: remove §5.4 entirely. Let `capabilities.md` define and document its own output fields.

### Consolidate Timeout Behavior

§3.5 and §4 both describe timeout behavior but contradict: the `blocking: false` override for `timeout_action` only appears in §3.5, not §4. An implementer reading §4 alone would miss the downgrade. Fix: consolidate all timeout behavior into one location (§3.5, with a forward reference from §4).

### Strengthen `capabilities` Field Language

§3.4 says implementations "SHOULD infer capabilities from manifest fields... rather than relying on this field." This is too soft for a field that is explicitly informational. Fix: strengthen to "implementations MUST NOT make conformance or behavioral decisions based on the `capabilities` field."

### Inline Normative Minimums in Conformance Section

After extraction, the conformance section (§8 in new numbering) must not degrade to a list of links. Fix: inline the specific normative requirements — the six core events by name, the core tool vocabulary entries, and the capability identifiers required at each level. The extracted documents hold the full tables; the conformance section holds the compliance checklist.

## CONTRIBUTING.md Updates

Add a cross-file dependency table so contributors know which files to update for each change type:

| Change Type | Files to Update |
|-------------|----------------|
| New event (with blocking behavior) | `events.md`, `blocking-matrix.md`, `CHANGELOG.md` |
| New event (observational only) | `events.md`, `CHANGELOG.md` |
| New provider | `events.md`, `blocking-matrix.md`, `capabilities.md`, `tools.md`, `CHANGELOG.md` |
| New capability | `capabilities.md`, `CHANGELOG.md` |
| New tool name | `tools.md`, `CHANGELOG.md` |
| Core format change | `hooks.md` (version bump), `schema/hook.schema.json`, `CHANGELOG.md` |

## Panel Review Record

This design was reviewed by a four-agent panel on 2026-04-08:

| Reviewer | Focus | Key Findings |
|----------|-------|-------------|
| Skeptic | Gaps, overreach, contradictions | Test vector format undocumented; section numbering inconsistency; dual changelog drift risk |
| Early Adopter | Onboarding, first-hour experience | Claude Code adapter too complex for learning; need test vector index; missing decode-direction vectors |
| Spec Author | Normative boundaries, versioning, spec-craft | Conformance section needs inline requirements; §5.4/§3.5-§4 spec-craft fixes; CONTRIBUTING.md dependency table |
| Enterprise Adopter | Policy enforcement, compliance, scale | Format spec is solid; governance gaps (audit logging, mandatory hooks, fleet distribution) deferred to policy spec |

### Accepted Changes

| # | Finding | Resolution |
|---|---------|------------|
| 1 | Inline normative minimums in conformance | Accept — core events, tool names, capability IDs listed in §8 |
| 2 | Single CHANGELOG + per-doc datestamps | Accept — replaces dual-changelog proposal |
| 3 | Versioning as own section (§9) | Accept — core spec is §1-9 |
| 4 | Add test-vectors/README.md | Accept — format contract, pairing rules, index |
| 5 | Cross-file dependency table in CONTRIBUTING.md | Accept |
| 6 | Clarify D10 scope | Accept — no syllago references; provider names are fine |
| 7a | Remove §5.4 phantom fields | Accept — capabilities.md owns its output fields |
| 7b | Consolidate timeout behavior | Accept — one location, forward reference |
| 8 | Exit code 137 handling | Defer — speculative, no provider implements this |
| 9 | Strengthen `capabilities` field to MUST NOT | Accept |
| 10 | Enterprise governance gaps | Defer — policy spec scope |
