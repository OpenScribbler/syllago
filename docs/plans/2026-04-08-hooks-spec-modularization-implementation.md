# Hooks Spec Modularization: Implementation Plan

**Date:** 2026-04-08
**Design Doc:** [2026-04-08-hooks-spec-modularization-design.md](./2026-04-08-hooks-spec-modularization-design.md)
**Status:** Ready to Execute

## Goal

Break the monolithic `hooks.md` (~900 lines) into a modular W3C-style document set.
Add a directory README, a test-vectors README, and reference implementations in Python and
TypeScript that consume the shared test vectors as a conformance suite.

## Architecture

```
docs/spec/hooks/
├── README.md                          ← New: directory index / landing page
├── hooks.md                           ← Modified: slimmed to ~400 lines, §1-9 renumbered
├── events.md                          ← New: extracted from current §7
├── tools.md                           ← New: extracted from current §10
├── capabilities.md                    ← New: extracted from current §9 + §11
├── blocking-matrix.md                 ← New: extracted from current §8
├── provider-strengths.md              ← New: extracted from Appendix A
├── glossary.md                        ← Unchanged
├── policy-interface.md                ← Unchanged
├── security-considerations.md         ← Unchanged
├── CHANGELOG.md                       ← Modified: add modularization entry
├── CONTRIBUTING.md                    ← Modified: add cross-file dependency table
├── LICENSE                            ← Unchanged
├── schema/hook.schema.json            ← Unchanged
├── test-vectors/
│   ├── README.md                      ← New: format contract and index
│   └── (existing directories)        ← Unchanged
└── examples/
    ├── README.md                      ← New: how to run, how to add a language
    ├── python/
    │   ├── pyproject.toml
    │   ├── hooks_interchange/
    │   │   ├── __init__.py
    │   │   ├── manifest.py
    │   │   ├── exit_codes.py
    │   │   ├── matchers.py
    │   │   ├── claude_code.py
    │   │   └── gemini_cli.py
    │   └── tests/test_conformance.py
    └── typescript/
        ├── package.json
        ├── tsconfig.json
        ├── src/
        │   ├── manifest.ts
        │   ├── exitCodes.ts
        │   ├── matchers.ts
        │   ├── claudeCode.ts
        │   └── geminiCli.ts
        └── tests/conformance.test.ts
```

## Tech Stack

- **Documentation:** Markdown, GitHub-rendered
- **Python ref impl:** Python 3.11+, PyYAML, pytest. No frameworks — stdlib + PyYAML only.
- **TypeScript ref impl:** TypeScript 5.x, Node.js 20+, Vitest. No frameworks — js-yaml for YAML parsing.
- **Test vectors:** Existing JSON files in `docs/spec/hooks/test-vectors/` — consumed as shared fixture data by both ref impls.

---

## Phase 1: Extract Registries to Separate Files

Tasks in this phase create the five new extracted documents. All content is copied from the
current `hooks.md` — the source is not modified yet. Extract tasks can run in parallel.

**Spec Neutrality (D10):** All authored text (document headers, introductory blurbs, cross-reference
sentences) MUST NOT reference syllago or any other specific implementation that uses the spec.
Provider names (Claude Code, Gemini CLI, Cursor, etc.) are fine — that is what the spec is for.
Only the spec itself and its community registries should be mentioned. Verify this during authoring;
no syllago references should appear in any new or modified file in `docs/spec/hooks/`.

---

### Task 1.1 — Create events.md

**Files:**
- `docs/spec/hooks/events.md` (create)

**Depends on:** nothing

**Content:**

Create a new file with the following structure:

```
# Event Registry

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines the canonical event names and their provider-native mappings
> referenced by the core spec's matcher and conversion pipeline sections.

**Registry Version:** 2026.04
**Last Modified:** 2026-04-08
**Status:** Initial Development
```

Then copy sections verbatim from `hooks.md`:

1. **§1 — Core Events** — Copy entire §7.1 table (6 events) with its introductory paragraph.
2. **§2 — Extended Events** — Copy entire §7.2 table (11 events).
3. **§3 — Provider-Exclusive Events** — Copy entire §7.3 table (5 events).
4. **§4 — Event Name Mapping** — Copy the full §7.4 table (21-row provider mapping matrix).
   Preserve the "Split-event providers" paragraph immediately following the table.

Use own heading numbering (§1-4) — do not carry over §7.x prefixes.

**Success Criteria:**
- `test -f docs/spec/hooks/events.md` → pass — File created
- `grep -c 'before_compact' docs/spec/hooks/events.md` → exactly 1 — `before_compact` present exactly once (verifies §2 Extended Events is complete and not duplicated)
- `grep -c 'config_change' docs/spec/hooks/events.md` → exactly 1 — Provider-exclusive events present (verifies §3 is complete)
- `grep -q 'Registry Version' docs/spec/hooks/events.md` → pass — Registry header present
- `grep -c '^## ' docs/spec/hooks/events.md` → 4 — Four top-level sections (§1 Core Events, §2 Extended Events, §3 Provider-Exclusive Events, §4 Event Name Mapping)

---

### Task 1.2 — Create tools.md

**Files:**
- `docs/spec/hooks/tools.md` (create)

**Depends on:** nothing

**Content:**

Create a new file with header:

```
# Tool Vocabulary

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines the canonical tool names that bare string matchers resolve against,
> and the provider-native names they map to during encode and decode.

**Registry Version:** 2026.04
**Last Modified:** 2026-04-08
**Status:** Initial Development
```

Then copy:

1. **§1 — Canonical Tool Names** — Copy the entire §10.1 table (9-row vocabulary table with
   introductory paragraph). The table columns are: Canonical Name, Description, then one column
   per provider.
2. **§2 — MCP Tool Names** — Copy §10.2 text verbatim.
   Update the cross-reference: `Section 6.3` → `[MCP Object matchers in the core spec](hooks.md#6-matcher-types)`.

Use own heading numbering (§1-2).

**Success Criteria:**
- `test -f docs/spec/hooks/tools.md` → pass — File created
- `grep '^| shell' docs/spec/hooks/tools.md` → pass — Verifies §1 table header row is present with correct canonical name column
- `grep -q 'Registry Version' docs/spec/hooks/tools.md` → pass — Registry header present
- `grep -c '^| ' docs/spec/hooks/tools.md` → 11 — Table has correct row count (header + separator + 9 tools; §10.2 has no table rows)

---

### Task 1.3 — Create capabilities.md

**Files:**
- `docs/spec/hooks/capabilities.md` (create)

**Depends on:** nothing

**Content:**

Create a new file with header:

```
# Capability Registry

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines optional semantic features beyond the core hook model, with
> provider support matrices, inference rules, and default degradation strategies.

**Registry Version:** 2026.04
**Last Modified:** 2026-04-08
**Status:** Initial Development
```

Then copy and merge content from two current sections:

**Part A — from §9 (Capability Registry):**

1. Copy the introductory paragraph from §9 verbatim.
2. **§1 — `structured_output`** — Copy §9.1 in full. This section already contains the
   advanced output fields table (suppress_output, system_message) — keep them here where they
   belong (this is the spec-craft fix that removes §5.4 from hooks.md).
3. **§2 — `input_rewrite`** — Copy §9.2 in full, including the advanced output field table
   for `updated_input`.
4. **§3 — `llm_evaluated`** — Copy §9.3 in full.
5. **§4 — `http_handler`** — Copy §9.4 in full.
6. **§5 — `async_execution`** — Copy §9.5 in full.
7. **§6 — `platform_commands`** — Copy §9.6 in full.
8. **§7 — `custom_env`** — Copy §9.7 in full.
9. **§8 — `configurable_cwd`** — Copy §9.8 in full.

**Part B — from §11 (Degradation Strategies), merged as §9:**

10. **§9 — Degradation Strategies** — Copy §11 in full (all three subsections: Strategy Values
    table, Author-Specified example, Safe Defaults table). This merges degradation into the
    capability doc since the two are tightly coupled.

Use own heading numbering (§1-9 for capabilities + degradation).

Update internal cross-references within the copied text:
- `Section 11` references → `§9 of this document`
- `Section 9` references → this document

**Success Criteria:**
- `test -f docs/spec/hooks/capabilities.md` → pass — File created
- `grep -q 'structured_output' docs/spec/hooks/capabilities.md` → pass — Capabilities present
- `grep -q 'updated_input' docs/spec/hooks/capabilities.md` → pass — input_rewrite output field present
- `grep -q 'suppress_output' docs/spec/hooks/capabilities.md` → pass — structured_output fields present
- `grep -q 'Degradation Strategies' docs/spec/hooks/capabilities.md` → pass — Degradation section merged
- `grep -q 'Registry Version' docs/spec/hooks/capabilities.md` → pass — Registry header present
- `grep -c '^## §' docs/spec/hooks/capabilities.md` → 9 — Exactly 8 capability sections + 1 degradation section (verifies all sections are present)

---

### Task 1.4 — Create blocking-matrix.md

**Files:**
- `docs/spec/hooks/blocking-matrix.md` (create)

**Depends on:** nothing

**Content:**

Create a new file with header:

```
# Blocking Behavior Matrix

> Part of the [Hook Interchange Format Specification](hooks.md).
> This document defines the blocking behavior per event/provider combination and the
> encoding rules adapters must follow when blocking intent cannot be honored.

**Last Modified:** 2026-04-08
**Status:** Initial Development
```

Note: no Registry Version on this document — per design decision D4, behavior matrices are
living documents updated in place, not date-stamped registries.

Then copy:

1. **§1 — Behavior Vocabulary** — Copy §8.1 table verbatim (prevent, retry, observe).
2. **§2 — Matrix** — Copy §8.2 matrix table verbatim plus both paragraphs following it
   (the "observe" encoding warning, and the blocking-on-observe MUST warn sentence).
3. **§3 — Timing Shift Warning** — Copy §8.3 verbatim.

Use own heading numbering (§1-3).

**Success Criteria:**
- `test -f docs/spec/hooks/blocking-matrix.md` → pass — File created
- `grep -c '^\| \*\*prevent\*\*' docs/spec/hooks/blocking-matrix.md` → 1 — Exactly one behavior vocabulary row for "prevent" (verifies §1 table is complete and not duplicated; §8.1 uses bold markdown for behavior names)
- `grep -q 'before_tool_execute' docs/spec/hooks/blocking-matrix.md` → pass — Matrix rows present
- `grep -q 'Timing Shift' docs/spec/hooks/blocking-matrix.md` → pass — Timing shift section present

---

### Task 1.5 — Create provider-strengths.md

**Files:**
- `docs/spec/hooks/provider-strengths.md` (create)

**Depends on:** nothing

**Content:**

Create a new file with header:

```
# Provider Strengths

> Non-normative companion to the [Hook Interchange Format Specification](hooks.md).
> This document highlights what each provider does best, independent of how many canonical
> features it supports.

**Last Modified:** 2026-04-08
**Status:** Non-normative
```

Then copy verbatim from Appendix A of `hooks.md`:

- All eight provider sections: Claude Code, Gemini CLI, Kiro, Cursor, Windsurf, OpenCode,
  VS Code Copilot, Copilot CLI. Each is a short paragraph (3-5 sentences).

No section numbering needed — use the provider name as the heading (### level) for each.
Add a brief one-paragraph introduction before the provider sections:

```markdown
Each provider brings unique capabilities to the hook ecosystem. This companion document
describes what makes each provider's hook system distinctive. Normative provider support
data lives in the [Event Registry](events.md), [Capability Registry](capabilities.md),
and [Blocking Behavior Matrix](blocking-matrix.md).
```

**Success Criteria:**
- `test -f docs/spec/hooks/provider-strengths.md` → pass — File created
- `grep -q 'Non-normative' docs/spec/hooks/provider-strengths.md` → pass — Non-normative marker present
- `grep -c '^### ' docs/spec/hooks/provider-strengths.md` → 8 — All eight providers present (verifies all 8 paragraphs were copied: Claude Code, Gemini CLI, Kiro, Cursor, Windsurf, OpenCode, VS Code Copilot, Copilot CLI)
- `wc -l docs/spec/hooks/provider-strengths.md` → line count between 50-100 — File is the right size for 8 provider paragraphs (Appendix A is ~60 lines including header and intro)

---

## Phase 2: Slim hooks.md

This phase makes a single coordinated edit to `hooks.md`. All changes happen in one task to
avoid partial states where the file has broken cross-references.

---

### Task 2.1 — Slim and renumber hooks.md

**Files:**
- `docs/spec/hooks/hooks.md` (modify)

**Depends on:** Tasks 1.1, 1.2, 1.3, 1.4, 1.5 (all extracted docs must exist)

**What to do:**

This task has eight distinct sub-operations. Perform them in order on a single editing pass.

**Sub-operation A: Update Table of Contents**

Replace the current 14-section + 2-appendix ToC with:

```markdown
1. [Introduction](#1-introduction)
2. [Terminology](#2-terminology)
3. [Canonical Format](#3-canonical-format)
4. [Exit Code Contract](#4-exit-code-contract)
5. [Canonical Output Schema](#5-canonical-output-schema)
6. [Matcher Types](#6-matcher-types)
7. [Conversion Pipeline](#7-conversion-pipeline)
8. [Conformance Levels](#8-conformance-levels)
9. [Versioning](#9-versioning)
- [Appendix A: JSON Schema Reference](#appendix-a-json-schema-reference)
```

**Sub-operation B: Update §1.1 (What This Spec Defines)**

Add two bullets to the "What This Spec Defines" list that were implicit before:
```markdown
- A [registry of canonical event names](events.md) with provider mappings
- A [capability model](capabilities.md) with provider support matrices and degradation strategies
- A [tool vocabulary](tools.md) mapping canonical tool names to provider-native names
- A [blocking behavior matrix](blocking-matrix.md) per event/provider combination
```

And add the following introductory cross-reference blurb at the end of §1:
```markdown
### 1.3 Document Set

This specification is organized as a set of focused documents. Readers who need only the
format definition can read §1-6 of this document. Implementers building a converter need
the [Event Registry](events.md) and [Tool Vocabulary](tools.md). The
[Blocking Behavior Matrix](blocking-matrix.md) and [Capability Registry](capabilities.md)
are used during the validation stage of the conversion pipeline.

See the [directory README](README.md) for the full document index.
```

**Sub-operation C: Fix §3.4 (Hook Definition — capabilities field)**

Locate the `capabilities` field row in the §3.4 table. The current description reads:

> Informational list of capability identifier strings. Implementations SHOULD infer
> capabilities from manifest fields and handler output patterns rather than relying on this
> field. See Section 9 for the inference rules.

Replace with (spec-craft fix: strengthen to MUST NOT per accepted finding #9):

> Informational list of capability identifier strings. Implementations MUST NOT make
> conformance or behavioral decisions based on this field. Capability inference rules are
> defined in the [Capability Registry](capabilities.md).

**Sub-operation D: Fix §3.5 (Handler — timeout_action field)**

The `timeout_action` field row currently cross-references §3.5 itself (describing behavior
inline). After this pass, timeout resolution is fully canonical — keep the field description
but remove the reference to "§4" at the end of the `timeout_action` row since §4 no longer
repeats this behavior (it will have a forward reference instead).

The description stays as-is since it is accurate and self-contained in §3.5.

**Sub-operation E: Fix §4 (Exit Code Contract)**

The last paragraph of §4 currently reads:

> When a hook exceeds its timeout, the `timeout_action` field on the handler definition
> determines the behavior (see Section 3.5). The default is `"warn"` (treat as exit code 1,
> action proceeds).

This is the consolidation fix. Replace the paragraph with a forward reference only:

> When a hook exceeds its timeout, behavior is determined by the `timeout_action` field on
> the handler definition (§3.5). The `blocking: false` downgrade applies before
> `timeout_action` evaluation — when `blocking` is `false`, timeout always degrades to a
> warning regardless of `timeout_action`.

This adds the missing downgrade note that was only in §3.5 before.

**Sub-operation F: Remove §5.4 (Phantom Fields)**

Delete the entire §5.4 section ("Advanced Output Fields") from `hooks.md`. The three fields
(`updated_input`, `suppress_output`, `system_message`) are now fully documented in
`capabilities.md` under their respective capability sections. Removing §5.4 eliminates the
editorial dependency where the core spec referenced fields owned by the extracted doc.

**Exact deletion target:** Locate the section that starts with the heading:

```
### 5.4 Advanced Output Fields
```

...and ends with the last sentence of that section: `"Implementations MUST ignore these fields
if the corresponding capability is not supported."` followed by the bullet list of the three
fields (`updated_input`, `suppress_output`, `system_message`).

Delete from the `### 5.4 Advanced Output Fields` heading line through the end of the bullet
list, including the blank line after the last bullet. Do NOT delete the `---` horizontal rule
that follows §5 and precedes §6.

Verify: `grep -q '5.4' docs/spec/hooks/hooks.md` → fail (0 matches)

**Sub-operation G: Replace §7-§14 with extracted-doc cross-references**

Remove sections §7, §8, §9, §10, §11 entirely. Replace with cross-reference stubs:

After §6 (Matcher Types), insert:

```markdown
---

## Registries

The following documents define the registries and matrices referenced by this specification.
Consult them when implementing adapters or verifying provider support.

| Document | Contents |
|----------|----------|
| [events.md](events.md) | Canonical event names, provider-native mappings, event name mapping table |
| [tools.md](tools.md) | Canonical tool names and provider-native equivalents |
| [capabilities.md](capabilities.md) | Optional capability features, support matrices, and degradation strategies |
| [blocking-matrix.md](blocking-matrix.md) | Blocking behavior per event and provider |
| [provider-strengths.md](provider-strengths.md) | Non-normative provider highlights |
```

Then add the conversion pipeline as §7 (renumbered from §12):

```markdown
## 7. Conversion Pipeline
```

Followed by the verbatim content of current §12 (all four subsections: Decode, Validate,
Encode, Verify). After pasting §12 content, do a **global find-replace across the entire
`hooks.md` file** (not just the §7 section) for every remaining old section reference. Replace
every occurrence of each pattern, regardless of where it appears in the file:
- Every occurrence of `Section 7.4` → `[Event Registry](events.md)`
- Every occurrence of `Section 10` → `[Tool Vocabulary](tools.md)`
- Every occurrence of `Section 9` → `[Capability Registry](capabilities.md)`
- Every occurrence of `Section 11.3` → `[Capability Registry](capabilities.md)` ← **do this BEFORE replacing Section 11**
- Every occurrence of `Section 11` (remaining, after 11.3 is already replaced) → `[Capability Registry](capabilities.md)`
- Every occurrence of `Section 8` → `[Blocking Behavior Matrix](blocking-matrix.md)`

**Important:** Replace `Section 11.3` before `Section 11`. If you replace `Section 11` first, `Section 11.3` becomes an orphaned `.3` that no longer matches the pattern.

Before replacing, audit with: `grep 'Section [0-9]' docs/spec/hooks/hooks.md` to see all
remaining references and confirm none are left after the replacements.

**Sub-operation H: Inline normative minimums in Conformance section (§8)**

After the extracted-doc references and conversion pipeline, add §8 (renumbered from §13).
Copy §13.1, 13.2, 13.3 verbatim (including their intro text and bullet lists). The inline
normative checklists are inserted into §8.1 (Core) **after** the existing bullet list as a new
subsection. Do NOT insert them between the bullets or replace any existing text.

**Exact §8.1 (Core) structure after insertion:**

```
### 8.1 Core

[existing intro sentence: "A Core-conformant implementation MUST:"]

[existing bullet list — copy verbatim from §13.1, all 7 bullets]

#### Core Conformance Checklist

[three requirement blocks below]
```

After the existing §13.1 bullet list, add a new `#### Core Conformance Checklist` heading,
then add:

```markdown
**Core event requirements.** A Core-conformant implementation MUST support these six events
by name: `before_tool_execute`, `after_tool_execute`, `session_start`, `session_end`,
`before_prompt`, `agent_stop`. Full event names and provider mappings are in
[events.md](events.md).

**Core tool vocabulary requirements.** A Core-conformant implementation MUST support bare
string matchers for these canonical tool names: `shell`, `file_read`, `file_write`,
`file_edit`, `search`, `find`, `web_search`, `web_fetch`, `agent`. Full vocabulary with
provider mappings is in [tools.md](tools.md).

**Core capability requirements.** Core conformance does not require capability support.
Extended conformance requires at minimum: `structured_output`, `input_rewrite`,
`platform_commands`. Full capability definitions are in [capabilities.md](capabilities.md).
```

Renumber §13.1 Core → §8.1 Core, §13.2 Extended → §8.2 Extended, §13.3 Full → §8.3 Full.

Then add §9 (Versioning, renumbered from §14) — copy §14 verbatim, updating:
- `Section 7` → `[Event Registry](events.md)`
- `Section 9` → `[Capability Registry](capabilities.md)`
- `Section 10` → `[Tool Vocabulary](tools.md)`
- `Section 8` → `[Blocking Behavior Matrix](blocking-matrix.md)`

Rename "Appendix B" → "Appendix A" (Appendix A was moved to provider-strengths.md).

**Success Criteria:**
- `wc -l docs/spec/hooks/hooks.md` → line count between 380-450 — File is meaningfully slimmed
- `grep -c '^## [0-9]' docs/spec/hooks/hooks.md` → 9 — Nine numbered sections (§1 Introduction through §9 Versioning)
- `grep -c '^## Registries' docs/spec/hooks/hooks.md` → 1 — Registries stub section present
- `grep -c '^## Appendix' docs/spec/hooks/hooks.md` → 1 — Appendix A (JSON Schema Reference) present
- `grep -q 'Section 7' docs/spec/hooks/hooks.md` → fail (0 matches) — Old section refs removed
- `grep -q 'Section 8' docs/spec/hooks/hooks.md` → fail — Old section refs removed
- `grep -q 'Section 9' docs/spec/hooks/hooks.md` → fail — Old section refs removed
- `grep -q 'Section 10' docs/spec/hooks/hooks.md` → fail — Old section refs removed
- `grep -q 'Section 11' docs/spec/hooks/hooks.md` → fail — Old section refs removed
- `grep -q 'Section 12' docs/spec/hooks/hooks.md` → fail — Old section refs removed
- `grep -q 'Section 13' docs/spec/hooks/hooks.md` → fail — Old section refs removed
- `grep -q 'events.md' docs/spec/hooks/hooks.md` → pass — Cross-references to events.md present
- `grep -q 'MUST NOT make conformance' docs/spec/hooks/hooks.md` → pass — §3.4 capabilities fix applied
- `grep -q '5.4' docs/spec/hooks/hooks.md` → fail — §5.4 removed
- `grep -q 'before_tool_execute.*after_tool_execute.*session_start' docs/spec/hooks/hooks.md` → pass — Inline normative minimums in §8

---

## Phase 3: Add README Files

---

### Task 3.1 — Create test-vectors/README.md

**Files:**
- `docs/spec/hooks/test-vectors/README.md` (create)

**Depends on:** nothing (independent of Phase 1/2)

**Content:**

The test-vectors README is the format contract for contributors adding test vectors. It has
four sections:

**§1 — Purpose**

Brief paragraph: test vectors are the shared ground truth consumed by reference implementations,
JSON Schema validators, and provider adapter tests. They are the authoritative statement of
what correct conversion looks like.

**§2 — Directory Structure**

```markdown
## Directory Structure

| Directory | Contents |
|-----------|----------|
| `canonical/` | Canonical format manifests (spec-compliant JSON). These are the inputs to encode operations and the expected outputs of decode operations. |
| `claude-code/` | Expected Claude Code native output for each canonical vector |
| `gemini-cli/` | Expected Gemini CLI native output for each canonical vector |
| `cursor/` | Expected Cursor native output for each canonical vector |
| `windsurf/` | Expected Windsurf native output for each canonical vector |
| `invalid/` | Malformed canonical inputs that MUST be rejected by conforming parsers |
```

**§3 — File Pairing Rules**

```markdown
## Pairing Rules

Each file in `canonical/` SHOULD have a corresponding file in each provider directory with
the same name. Example: `canonical/simple-blocking.json` pairs with
`claude-code/simple-blocking.json`, `gemini-cli/simple-blocking.json`, etc.

Files in `invalid/` have no provider pairs — they document expected rejection behavior only.

When a new test vector exercises a feature not supported by a provider, the provider file
is omitted (not an error). When a provider file is intentionally absent, document the
reason in the canonical file's `_comment` field.
```

**§4 — Non-normative Fields**

Document `_comment` and `_warnings` conventions (copy text from CONTRIBUTING.md §"Test Vector
Format" → Conventions subsection).

**§5 — Index Table**

An index of all current test vectors with a one-line description:

| File | Description |
|------|-------------|
| `canonical/simple-blocking.json` | Minimal blocking hook — core layer only |
| `canonical/full-featured.json` | All core fields: platform, cwd, env, degradation, provider_data, MCP matcher |
| `canonical/multi-event.json` | Multiple hooks across events |
| `canonical/degradation-input-rewrite.json` | Safety-critical input_rewrite degradation to block |
| `claude-code/roundtrip-source.json` | Claude Code native format for round-trip test |
| `claude-code/roundtrip-canonical.json` | Expected canonical output of decoding roundtrip-source |
| `invalid/missing-spec.json` | Must be rejected: missing `spec` field |
| `invalid/missing-hooks.json` | Must be rejected: missing `hooks` field |
| `invalid/empty-hooks-array.json` | Must be rejected: empty `hooks` array |
| `invalid/missing-event.json` | Must be rejected: hook missing required `event` |
| `invalid/missing-handler.json` | Must be rejected: hook missing required `handler` |
| `invalid/invalid-degradation-strategy.json` | Must be rejected: unknown degradation strategy |

**§6 — Validation Commands**

Copy the validation commands from CONTRIBUTING.md (ajv-cli and check-jsonschema).

**Note:** The paths in CONTRIBUTING.md's validation commands are incorrect — they use
`docs/spec/schema/hook.schema.json` and `docs/spec/test-vectors/canonical/*.json`. Correct
these to `docs/spec/hooks/schema/hook.schema.json` and `docs/spec/hooks/test-vectors/canonical/*.json`
when copying into this README.

**Success Criteria:**
- `test -f docs/spec/hooks/test-vectors/README.md` → pass — File created
- `grep -q '_comment' docs/spec/hooks/test-vectors/README.md` → pass — Convention documented
- `grep -q '_warnings' docs/spec/hooks/test-vectors/README.md` → pass — Convention documented
- `grep -q 'simple-blocking' docs/spec/hooks/test-vectors/README.md` → pass — Index table present

---

### Task 3.2 — Create docs/spec/hooks/README.md

**Files:**
- `docs/spec/hooks/README.md` (create)

**Depends on:** Tasks 1.1-1.5, 2.1 (all files must exist to be accurately listed)

**Content:**

**Abstract (1 paragraph):**

```markdown
The Hook Interchange Format is a provider-neutral specification for AI coding tool hooks.
It enables hook authors to write lifecycle hooks once and deploy them across multiple AI
coding tool providers through a canonical representation, capability model, and conversion
pipeline. This directory contains the full specification, registries, and reference
implementations.
```

**Quick Links Table:**

| Document | Description |
|----------|-------------|
| [hooks.md](hooks.md) | Core spec: canonical format, exit codes, matchers, conversion pipeline, conformance |
| [events.md](events.md) | Event Registry: canonical event names and provider-native mappings |
| [tools.md](tools.md) | Tool Vocabulary: canonical tool names and provider-native equivalents |
| [capabilities.md](capabilities.md) | Capability Registry: optional features, support matrices, degradation strategies |
| [blocking-matrix.md](blocking-matrix.md) | Blocking behavior per event and provider |
| [provider-strengths.md](provider-strengths.md) | Non-normative: what each provider does best |
| [glossary.md](glossary.md) | Term definitions |
| [security-considerations.md](security-considerations.md) | Security analysis and threat model |
| [policy-interface.md](policy-interface.md) | Enterprise policy enforcement interface |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute: registry updates, new providers, test vectors |
| [CHANGELOG.md](CHANGELOG.md) | Version history |
| [test-vectors/README.md](test-vectors/README.md) | Test vector format contract and index |
| [examples/](examples/) | Reference implementations in Python and TypeScript |

**Getting Started Section:**

Three paths:
1. **Reading the spec** → start with hooks.md §1-6
2. **Building a converter** → hooks.md §7 (pipeline), then events.md and tools.md
3. **Implementing from scratch** → read examples/, run the conformance tests, extend

**Version/Status:**

```markdown
**Specification Version:** 0.1.0 (initial development — anything may change)
**License:** [CC-BY-4.0](LICENSE)
```

**Success Criteria:**
- `test -f docs/spec/hooks/README.md` → pass — File created
- `grep -q 'hooks.md' docs/spec/hooks/README.md` → pass — Core spec linked
- `grep -q 'events.md' docs/spec/hooks/README.md` → pass — Events registry linked
- `grep -q 'Getting Started' docs/spec/hooks/README.md` → pass — Getting started section present
- `grep -c '\.md' docs/spec/hooks/README.md` → 12 — All 12 required documents linked (hooks.md, events.md, tools.md, capabilities.md, blocking-matrix.md, provider-strengths.md, glossary.md, security-considerations.md, policy-interface.md, CONTRIBUTING.md, CHANGELOG.md, test-vectors/README.md)
- `grep -qi 'syllago' docs/spec/hooks/README.md` → fail (0 matches) — No syllago references (D10 spec neutrality)

---

## Phase 4: Python Reference Implementation

Tasks in this phase create the Python package. Tasks 4.1-4.5 (module files) can run in
parallel immediately — no dependency on Phase 3. Task 4.6 (conformance tests) depends on
all module files (4.1-4.5).

---

### Task 4.1 — Python: manifest.py

**Files:**
- `docs/spec/hooks/examples/python/hooks_interchange/manifest.py` (create)
- `docs/spec/hooks/examples/python/hooks_interchange/__init__.py` (create)
- `docs/spec/hooks/examples/python/pyproject.toml` (create)

**Depends on:** nothing (create directory structure implicitly)

**pyproject.toml content:**

```toml
[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.backends.legacy:build"

[project]
name = "hooks-interchange"
version = "0.1.0"
description = "Hook Interchange Format reference implementation (Core conformance)"
requires-python = ">=3.11"
dependencies = ["pyyaml>=6.0"]

[project.optional-dependencies]
dev = ["pytest>=8.0", "pytest-json-report"]

[tool.pytest.ini_options]
testpaths = ["tests"]
```

**__init__.py content:** Export the three public entry points:

```python
from .manifest import parse_manifest, ValidationError
from .exit_codes import resolve_decision
from .matchers import resolve_matcher
```

**manifest.py — what to implement:**

```python
from __future__ import annotations
import json
from pathlib import Path
from typing import Any

import yaml


class ValidationError(Exception):
    """Raised when a manifest fails spec-required validation."""


def parse_manifest(source: str | bytes | dict) -> dict:
    """Parse a canonical hook manifest from JSON string, YAML string, or dict.

    Implements §3.1 (JSON/YAML), §3.2 (forward compatibility — unknown fields ignored),
    §3.3 (required fields: spec, hooks; non-empty hooks array).

    Returns the manifest as a Python dict.
    Raises ValidationError for any spec violation.
    """
```

Logic:
- If `source` is `str`, try `json.loads` first. On JSON parse error, try `yaml.safe_load`.
  If both fail, raise `ValidationError("Could not parse manifest as JSON or YAML")`.
- If `source` is `bytes`, decode as UTF-8 then apply the string logic.
- If `source` is `dict`, use as-is.
- Validate presence of `spec` field → raise `ValidationError("Missing required field: spec")`
- Validate `spec` value matches pattern `hooks/\d+\.\d+` → raise `ValidationError(f"Invalid spec value: {spec!r}")`
- Validate presence of `hooks` field → raise `ValidationError("Missing required field: hooks")`
- Validate `hooks` is a non-empty list → raise `ValidationError("hooks array must not be empty")`
- For each hook in `hooks`:
  - Validate presence of `event` field → raise `ValidationError(f"Hook {i}: missing required field: event")`
  - Validate presence of `handler` field → raise `ValidationError(f"Hook {i}: missing required field: handler")`
  - Validate `handler.type` is present → raise `ValidationError(f"Hook {i}: handler missing required field: type")`
- Do NOT validate unknown fields (§3.2 forward compatibility).
- Return the manifest dict unchanged (validation only, no transformation).

Also implement:

```python
def load_manifest(path: Path) -> dict:
    """Load and parse a manifest from a file path. Detects JSON vs YAML by extension."""
```

**Success Criteria:**
- `test -f docs/spec/hooks/examples/python/hooks_interchange/manifest.py` → pass — File created
- `test -f docs/spec/hooks/examples/python/pyproject.toml` → pass — Package config created
- `cd docs/spec/hooks/examples/python && python -c "from hooks_interchange import parse_manifest"` → pass — Module importable

**Note:** The `__init__.py` imports `resolve_decision` (from 4.2) and `resolve_matcher` (from 4.3).
The file-existence criteria can run in parallel with 4.2/4.3, but the importability test
MUST wait until 4.2 and 4.3 are also complete.

---

### Task 4.2 — Python: exit_codes.py

**Files:**
- `docs/spec/hooks/examples/python/hooks_interchange/exit_codes.py` (create)

**Depends on:** nothing

**exit_codes.py — what to implement:**

```python
from __future__ import annotations
from enum import Enum
from typing import Optional


class Decision(str, Enum):
    ALLOW = "allow"
    DENY = "deny"
    ASK = "ask"


class Result(str, Enum):
    ALLOW = "allow"
    BLOCK = "block"
    WARN_ALLOW = "warn_allow"   # Non-blocking warning, action proceeds
    ASK_USER = "ask_user"       # Interactive confirmation requested


def resolve_decision(
    blocking: bool,
    exit_code: int,
    decision: Optional[str],
    interactive: bool = True,
) -> Result:
    """Implement the §5.3 truth table.

    Args:
        blocking: The hook's blocking field.
        exit_code: The process exit code.
        decision: The JSON decision field value, or None if absent.
        interactive: Whether a user is available to respond to ASK decisions.
                     When False (e.g., CI/CD), ASK is treated as DENY (§5.2).

    Returns a Result enum value.
    """
```

**Implementation order (critical — do not reverse):**

1. **Non-blocking downgrade first:** if `blocking` is `False` and `exit_code == 2`, set `exit_code = 1` before any table lookup.
2. **Truth table lookup:** apply the 12-row table using the (possibly downgraded) `(blocking, exit_code, decision)` combination.
3. **Return the Result.**

The downgrade MUST happen before the table lookup. After downgrading, a `(False, 2, any)` input will never reach the table — it becomes `(False, 1, any)` and returns `WARN_ALLOW` via the last row. The table row `False | 2 | any | WARN_ALLOW` documents the *intent*, not a literal match — the downgrade step ensures it never needs a dedicated branch.

Implement the exact 12-row truth table from §5.3:

| blocking | exit_code | decision | Result |
|----------|-----------|----------|--------|
| True | 0 | "allow" | ALLOW |
| True | 0 | "deny" | BLOCK |
| True | 0 | "ask" | ASK_USER (or BLOCK if not interactive) |
| True | 0 | None | ALLOW |
| True | 2 | "allow" | BLOCK (exit code 2 overrides) |
| True | 2 | any/None | BLOCK |
| True | 1/other | any | WARN_ALLOW |
| False | 0 | "allow" | ALLOW |
| False | 0 | "deny" | BLOCK |
| False | 0 | None | ALLOW |
| False | 2 | any | WARN_ALLOW (downgraded to exit 1 before table lookup) |
| False | 1/other | any | WARN_ALLOW |

**Success Criteria:**
- `test -f docs/spec/hooks/examples/python/hooks_interchange/exit_codes.py` → pass — File created
- `cd docs/spec/hooks/examples/python && python -c "from hooks_interchange.exit_codes import resolve_decision, Result; assert resolve_decision(True, 2, 'allow') == Result.BLOCK"` → pass — §5.3 exit code 2 overrides allow

---

### Task 4.3 — Python: matchers.py

**Files:**
- `docs/spec/hooks/examples/python/hooks_interchange/matchers.py` (create)

**Depends on:** nothing

**matchers.py — what to implement:**

```python
from __future__ import annotations
import re
import warnings
from typing import Any

# Tool vocabulary: canonical name -> {provider_slug: native_name}
# SOURCE: docs/spec/hooks/tools.md §1 table (or docs/spec/hooks/hooks.md §10.1 if tools.md
# does not yet exist — the content is identical; tools.md is an extraction of §10.1).
# Populate ALL 9 names from that table: shell, file_read, file_write, file_edit, search,
# find, web_search, web_fetch, agent. Each entry maps provider slugs to native names.
# A "--" in the table means the provider has no equivalent; omit that provider's key.
# The "shell" entry is provided as a reference; read §10.1 for the remaining 8.
TOOL_VOCABULARY: dict[str, dict[str, str]] = {
    "shell": {
        "claude-code": "Bash",
        "gemini-cli": "run_shell_command",
        "cursor": "run_terminal_cmd",
        "copilot-cli": "bash",
        "kiro": "execute_bash",
        "opencode": "bash",
    },
    "file_read": { ... },   # read tools.md §1 — all 9 names must be populated
    "file_write": { ... },
    "file_edit": { ... },
    "search": { ... },
    "find": { ... },
    "web_search": { ... },
    "web_fetch": { ... },
    "agent": { ... },
}


def resolve_matcher_to_provider(
    matcher: str | dict | list | None,
    provider: str,
) -> str | dict | list | None:
    """Resolve a canonical matcher to a provider-native form.

    Implements §6.1-6.5.
    - Bare string: look up in TOOL_VOCABULARY; if not found, pass through as literal with warning
    - Pattern object: pass through unchanged (provider handles RE2 regex)
    - MCP object: encode to provider-specific combined format
    - Array: recursively resolve each element
    - None (omitted): return None (wildcard)
    """


def decode_matcher_from_provider(
    native_matcher: str | None,
    provider: str,
) -> str | dict | None:
    """Decode a provider-native matcher to canonical form.

    Implements decode direction of §6:
    - Reverse-look up native name in TOOL_VOCABULARY to find canonical name
    - Parse MCP combined strings (mcp__server__tool, mcp_server_tool, etc.) to structured dict
    - If native name not in vocabulary, return as literal bare string with warning
    """


MCP_COMBINED_FORMATS: dict[str, str] = {
    "claude-code": "mcp__{server}__{tool}",
    "kiro": "mcp__{server}__{tool}",
    "gemini-cli": "mcp_{server}_{tool}",
    "copilot-cli": "{server}/{tool}",
    "cursor": "{server}__{tool}",
    "windsurf": "{server}__{tool}",
}
```

**Success Criteria:**
- `test -f docs/spec/hooks/examples/python/hooks_interchange/matchers.py` → pass — File created
- `cd docs/spec/hooks/examples/python && python -c "from hooks_interchange.matchers import resolve_matcher_to_provider; assert resolve_matcher_to_provider('shell', 'gemini-cli') == 'run_shell_command'"` → pass — Tool vocabulary lookup works

---

### Task 4.4 — Python: claude_code.py

**Files:**
- `docs/spec/hooks/examples/python/hooks_interchange/claude_code.py` (create)

**Depends on:** Tasks 4.3 (matchers.py needed)

**claude_code.py — what to implement:**

```python
from __future__ import annotations
from typing import Any
from .matchers import resolve_matcher_to_provider, decode_matcher_from_provider

PROVIDER = "claude-code"

# Claude Code event name mapping (canonical -> native)
EVENT_MAP: dict[str, str] = {
    "before_tool_execute": "PreToolUse",
    "after_tool_execute": "PostToolUse",
    "session_start": "SessionStart",
    "session_end": "SessionEnd",
    "before_prompt": "UserPromptSubmit",
    "agent_stop": "Stop",
    "before_compact": "PreCompact",
    "notification": "Notification",
    "error_occurred": "StopFailure",
    "tool_use_failure": "PostToolUseFailure",
    "file_changed": "FileChanged",
    "subagent_start": "SubagentStart",
    "subagent_stop": "SubagentStop",
    "permission_request": "PermissionRequest",
    "config_change": "ConfigChange",
}

EVENT_MAP_REVERSE: dict[str, str] = {v: k for k, v in EVENT_MAP.items()}
```

**Field Mapping (Canonical → Claude Code):**

Derived from test vectors `canonical/simple-blocking.json`, `canonical/full-featured.json`,
and their corresponding `claude-code/` outputs.

| Canonical Field | Claude Code Field | Transformation |
|-----------------|-------------------|----------------|
| `hook.event` | top-level key in `hooks` dict | Translated via `EVENT_MAP` |
| `hook.matcher` | `hook_entry.matcher` | Translated via `resolve_matcher_to_provider`; bare string → native name (e.g., `"shell"` → `"Bash"`); MCP object → `mcp__server__tool`; absent → `""` (empty string) |
| `hook.handler.command` | `hook.command` | Copy as-is |
| `hook.handler.timeout` | `hook.timeout` | Copy as-is (seconds — CC uses seconds) |
| `hook.handler.type` | `hook.type` | Copy as-is (`"command"`) |
| `hook.blocking` | (not emitted) | CC hooks run regardless of blocking; field is not present in native output |
| `hook.handler.platform` | (dropped with warning) | CC does not support per-OS overrides (`platform_commands` capability, degrade: warn) |
| `hook.handler.cwd` | (dropped with warning) | CC does not support configurable cwd (`configurable_cwd` capability, degrade: warn) |
| `hook.handler.env` | (dropped with warning) | CC does not support custom env (`custom_env` capability, degrade: warn) |
| `hook.handler.degradation` | (not emitted) | Degradation is a canonical-layer concept; not represented in native format |
| `hook.provider_data["claude-code"]` | merged into hook entry | Shallow merge into the hook dict |
| `hook.provider_data[other]` | (dropped silently) | Only target-provider data is rendered |

**Output structure:** All hooks for the same native event are grouped under that event key. Each
group is a list of `{"matcher": ..., "hooks": [hook_entry, ...]}` objects. Hooks for the same
event with different matchers become separate objects in the list (see `full-featured.json`:
two `PreToolUse` entries, one for `"Bash"` and one for `"mcp__github__create_issue"`).

```python
def encode(manifest: dict) -> dict:
    """Encode a canonical manifest to Claude Code native format.

    Claude Code format: {"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [...]}]}}
    Each canonical hook becomes an entry in the appropriate event bucket.

    Rules:
    - Translate event names using EVENT_MAP. If unmapped, emit warning and skip.
    - Translate matchers using resolve_matcher_to_provider(matcher, PROVIDER).
    - Timeout stays in seconds (Claude Code uses seconds).
    - provider_data["claude-code"], if present, is merged into the hook entry.
    - Non-blocking hooks: hooks in Claude Code run regardless of blocking flag.
    - Returns: {"hooks": {NativeEvent: [{"matcher": ..., "hooks": [{...}]}]}}
    """


def decode(native: dict) -> dict:
    """Decode Claude Code native format to canonical manifest.

    Handles both event-keyed structure: {"hooks": {"PreToolUse": [...]}}
    Input to each decoded hook: {"type": "command", "command": ..., "timeout": ...}

    Rules:
    - Translate native event names to canonical using EVENT_MAP_REVERSE.
    - Translate matchers using decode_matcher_from_provider(matcher, PROVIDER).
    - Preserve unknown event names in provider_data["claude-code"].
    - Returns a canonical manifest dict with spec: "hooks/0.1".
    """
```

**Success Criteria:**
- `test -f docs/spec/hooks/examples/python/hooks_interchange/claude_code.py` → pass — File created
- `cd docs/spec/hooks/examples/python && python -c "from hooks_interchange.claude_code import encode, decode; print('ok')"` → pass — Module importable

---

### Task 4.5 — Python: gemini_cli.py

**Files:**
- `docs/spec/hooks/examples/python/hooks_interchange/gemini_cli.py` (create)

**Depends on:** Tasks 4.3 (matchers.py needed)

**gemini_cli.py — what to implement:**

```python
from __future__ import annotations
from .matchers import resolve_matcher_to_provider, decode_matcher_from_provider

PROVIDER = "gemini-cli"

# Gemini CLI event name mapping (canonical -> native)
EVENT_MAP: dict[str, str] = {
    "before_tool_execute": "BeforeTool",
    "after_tool_execute": "AfterTool",
    "session_start": "SessionStart",
    "session_end": "SessionEnd",
    "before_prompt": "BeforeAgent",
    "agent_stop": "AfterAgent",
    "before_compact": "PreCompress",
    "notification": "Notification",
    "before_model": "BeforeModel",
    "after_model": "AfterModel",
    "before_tool_selection": "BeforeToolSelection",
}

EVENT_MAP_REVERSE: dict[str, str] = {v: k for k, v in EVENT_MAP.items()}

TIMEOUT_SECONDS_TO_MS = 1000


def encode(manifest: dict) -> dict:
    """Encode a canonical manifest to Gemini CLI native format.

    Gemini CLI format: {"hooks": [{"trigger": "BeforeTool", "toolMatcher": ..., "command": ..., "timeoutMs": ...}]}
    
    Rules:
    - Translate event names using EVENT_MAP.
    - Translate matchers using resolve_matcher_to_provider(matcher, PROVIDER).
    - Timeout: convert seconds to milliseconds (multiply by 1000). Field name: timeoutMs.
    - provider_data["gemini-cli"], if present, is merged into the hook entry.
    - Returns: {"hooks": [{...}]}
    """


def decode(native: dict) -> dict:
    """Decode Gemini CLI native format to canonical manifest.

    Gemini CLI format: {"hooks": [{"trigger": "BeforeTool", "toolMatcher": ..., "command": ..., "timeoutMs": ...}]}

    Rules:
    - Translate trigger field to canonical event name.
    - Translate toolMatcher to canonical matcher.
    - Convert timeoutMs to seconds.
    - Returns a canonical manifest dict.
    """
```

**Success Criteria:**
- `test -f docs/spec/hooks/examples/python/hooks_interchange/gemini_cli.py` → pass — File created
- `cd docs/spec/hooks/examples/python && python -c "from hooks_interchange.gemini_cli import encode; print('ok')"` → pass — Module importable

---

### Task 4.6 — Python: conformance tests

**Files:**
- `docs/spec/hooks/examples/python/tests/__init__.py` (create, empty)
- `docs/spec/hooks/examples/python/tests/test_conformance.py` (create)

**Depends on:** Tasks 4.1, 4.2, 4.3, 4.4, 4.5 (all modules must exist)

**test_conformance.py — what to implement:**

The test file loads test vectors from `../../test-vectors/` (relative to the python directory).
Use `pathlib.Path(__file__).parent.parent.parent.parent / "test-vectors"` to locate the directory.

Path breakdown: `test_conformance.py` is at `examples/python/tests/`, so `.parent` = `tests/`,
`.parent.parent` = `python/`, `.parent.parent.parent` = `examples/`, `.parent.parent.parent.parent` = `hooks/`.
`test-vectors/` lives alongside `examples/` under `hooks/` — four `.parent` calls are required.

Structure: five test classes.

**Class 1: TestManifestParsing**

Parametrize over all files in `test-vectors/canonical/`:
```python
@pytest.mark.parametrize("vector_path", list((VECTORS / "canonical").glob("*.json")))
def test_canonical_parses_without_error(vector_path):
    manifest = parse_manifest(vector_path.read_text())
    assert manifest["spec"].startswith("hooks/")
    assert len(manifest["hooks"]) > 0
```

**Class 2: TestInvalidManifests**

Parametrize over all files in `test-vectors/invalid/`:
```python
@pytest.mark.parametrize("vector_path", list((VECTORS / "invalid").glob("*.json")))
def test_invalid_manifest_raises(vector_path):
    with pytest.raises(ValidationError):
        parse_manifest(vector_path.read_text())
```

**Class 3: TestExitCodes**

Unit tests for `resolve_decision` covering each row of the §5.3 truth table:
```python
def test_blocking_exit2_overrides_allow():
    assert resolve_decision(True, 2, "allow") == Result.BLOCK

def test_non_blocking_exit2_downgrades():
    assert resolve_decision(False, 2, None) == Result.WARN_ALLOW

# ... one test per truth table row
```

**Class 4: TestClaudeCodeVectors**

For each canonical vector that has a corresponding claude-code vector:
```python
@pytest.mark.parametrize("name", ["simple-blocking", "full-featured", "multi-event"])
def test_encode_matches_expected(name):
    canonical = parse_manifest((VECTORS / "canonical" / f"{name}.json").read_text())
    expected = json.loads((VECTORS / "claude-code" / f"{name}.json").read_text())
    actual = encode(canonical)
    assert_structurally_equivalent(actual, expected)
```

Also test decode for the roundtrip vector:
```python
def test_decode_roundtrip_source():
    native = json.loads((VECTORS / "claude-code" / "roundtrip-source.json").read_text())
    expected_canonical = json.loads((VECTORS / "claude-code" / "roundtrip-canonical.json").read_text())
    actual = decode(native)
    assert_structurally_equivalent(actual, expected_canonical)
```

**Class 5: TestGeminiCliVectors**

Same pattern as TestClaudeCodeVectors but for gemini-cli vectors
(`simple-blocking`, `full-featured`, `multi-event`, `degradation-input-rewrite`).

**Helper: assert_structurally_equivalent**

Per the glossary definition: same number of hooks, same canonical event names, same handler
types, same command strings, same matcher presence. Ignore field ordering and `_comment`.

```python
def assert_structurally_equivalent(actual: dict, expected: dict):
    """Structural equivalence check per spec glossary."""
    # Strip _comment from both
    # Compare hook count, events, handler types, command strings
```

**Success Criteria:**
- `test -f docs/spec/hooks/examples/python/tests/test_conformance.py` → pass — Test file created
- `cd docs/spec/hooks/examples/python && pip install -e ".[dev]" -q && python -m pytest tests/ -v` → pass — All conformance tests pass
- `cd docs/spec/hooks/examples/python && python -m pytest tests/ -v 2>&1 | grep -c 'PASSED'` → 20 or more — 20+ tests across all five classes

**Minimum coverage per test class:**
- `TestManifestParsing`: >= 5 tests — one per canonical vector file
- `TestInvalidManifests`: >= 6 tests — one per invalid vector file
- `TestExitCodes`: >= 12 tests — one per truth table row
- `TestClaudeCodeVectors`: >= 5 tests — encode for 3 vectors (simple-blocking, full-featured, multi-event) + decode for 1 roundtrip pair
- `TestGeminiCliVectors`: >= 4 tests — encode for 4 vectors (simple-blocking, full-featured, multi-event, degradation-input-rewrite)

All five classes must have passing tests; 20 passing tests from one class alone does not satisfy this criterion.

---

## Phase 5: TypeScript Reference Implementation

Tasks 5.1-5.5 (module files) can run in parallel. Task 5.6 (conformance tests) depends on all.

---

### Task 5.1 — TypeScript: project setup + manifest.ts

**Files:**
- `docs/spec/hooks/examples/typescript/package.json` (create)
- `docs/spec/hooks/examples/typescript/tsconfig.json` (create)
- `docs/spec/hooks/examples/typescript/src/manifest.ts` (create)

**Depends on:** nothing

**package.json content:**

```json
{
  "name": "hooks-interchange",
  "version": "0.1.0",
  "description": "Hook Interchange Format reference implementation (Core conformance)",
  "type": "module",
  "scripts": {
    "test": "vitest run",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "js-yaml": "^4.1.0"
  },
  "devDependencies": {
    "@types/js-yaml": "^4.0.9",
    "@types/node": "^20.0.0",
    "typescript": "^5.4.0",
    "vitest": "^1.6.0"
  }
}
```

**tsconfig.json content:**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "outDir": "dist",
    "rootDir": "src",
    "declaration": true
  },
  "include": ["src/**/*.ts", "tests/**/*.ts"]
}
```

**manifest.ts — what to implement:**

```typescript
import * as yaml from "js-yaml";

export class ValidationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ValidationError";
  }
}

export interface HookManifest {
  spec: string;
  hooks: HookDefinition[];
  [key: string]: unknown; // forward compatibility: unknown fields allowed
}

export interface HookDefinition {
  event: string;
  handler: HandlerDefinition;
  name?: string;
  matcher?: string | object | unknown[];
  blocking?: boolean;
  degradation?: Record<string, string>;
  provider_data?: Record<string, unknown>;
  capabilities?: string[];
  [key: string]: unknown;
}

export interface HandlerDefinition {
  type: string;
  command?: string;
  platform?: Record<string, string>;
  cwd?: string;
  env?: Record<string, string>;
  timeout?: number;
  timeout_action?: "warn" | "block";
  async?: boolean;
  status_message?: string;
  [key: string]: unknown;
}

/**
 * Parse a canonical hook manifest from a JSON string, YAML string, or plain object.
 * Implements §3.1 (JSON/YAML), §3.2 (forward compat), §3.3 (required fields).
 * Throws ValidationError for spec violations.
 */
export function parseManifest(source: string | object): HookManifest {
  // ...
}
```

Logic mirrors the Python version exactly. Try JSON parse first, then YAML on failure. Validate
`spec`, `hooks`, non-empty array, each hook has `event` and `handler`, each handler has `type`.

**Success Criteria:**
- `test -f docs/spec/hooks/examples/typescript/package.json` → pass — Package config created
- `test -f docs/spec/hooks/examples/typescript/src/manifest.ts` → pass — Module created
- `cd docs/spec/hooks/examples/typescript && npm install && npx tsc --noEmit` → pass — Type-checks clean

---

### Task 5.2 — TypeScript: exitCodes.ts

**Files:**
- `docs/spec/hooks/examples/typescript/src/exitCodes.ts` (create)

**Depends on:** nothing

**exitCodes.ts — what to implement:**

```typescript
export type Decision = "allow" | "deny" | "ask";
export type Result = "allow" | "block" | "warn_allow" | "ask_user";

/**
 * Implement the §5.3 truth table.
 */
export function resolveDecision(
  blocking: boolean,
  exitCode: number,
  decision: Decision | null,
  interactive: boolean = true
): Result {
  // Step 1: non-blocking downgrade
  // Step 2: truth table lookup
}
```

Same logic as Python version — 12-row truth table, non-blocking downgrade first.

**Success Criteria:**
- `test -f docs/spec/hooks/examples/typescript/src/exitCodes.ts` → pass — File created

---

### Task 5.3 — TypeScript: matchers.ts

**Files:**
- `docs/spec/hooks/examples/typescript/src/matchers.ts` (create)

**Depends on:** nothing

**matchers.ts — what to implement:**

```typescript
// Tool vocabulary: canonical name -> provider slug -> native name
// SOURCE: docs/spec/hooks/tools.md §1 table (or docs/spec/hooks/hooks.md §10.1 if tools.md
// does not yet exist — the content is identical; tools.md is an extraction of §10.1).
// Populate ALL 9 names from that table: shell, file_read, file_write, file_edit, search,
// find, web_search, web_fetch, agent. Each entry maps provider slugs to native names.
// A "--" in the table means the provider has no equivalent; omit that provider's key.
export const TOOL_VOCABULARY: Record<string, Record<string, string>> = {
  shell: {
    "claude-code": "Bash",
    "gemini-cli": "run_shell_command",
    // ... all providers from tools.md §1
  },
  // file_read, file_write, file_edit, search, find, web_search, web_fetch, agent
  // — all 9 canonical tool names must be present
};

export const MCP_COMBINED_FORMATS: Record<string, string> = {
  "claude-code": "mcp__{server}__{tool}",
  "kiro": "mcp__{server}__{tool}",
  "gemini-cli": "mcp_{server}_{tool}",
  "copilot-cli": "{server}/{tool}",
  cursor: "{server}__{tool}",
  windsurf: "{server}__{tool}",
};

export interface McpMatcher {
  mcp: { server: string; tool?: string };
}

/**
 * Resolve a canonical matcher to provider-native form. Implements §6.1-6.5.
 */
export function resolveMatcherToProvider(
  matcher: string | object | unknown[] | null | undefined,
  provider: string
): string | object | unknown[] | null | undefined {
  // ...
}

/**
 * Decode a provider-native matcher to canonical form.
 */
export function decodeMatcherFromProvider(
  nativeMatcher: string | null | undefined,
  provider: string
): string | McpMatcher | null | undefined {
  // ...
}
```

**Success Criteria:**
- `test -f docs/spec/hooks/examples/typescript/src/matchers.ts` → pass — File created

---

### Task 5.4 — TypeScript: claudeCode.ts

**Files:**
- `docs/spec/hooks/examples/typescript/src/claudeCode.ts` (create)

**Depends on:** Task 5.3 (matchers.ts)

**claudeCode.ts — what to implement:**

```typescript
import { HookManifest } from "./manifest.js";
import { resolveMatcherToProvider, decodeMatcherFromProvider } from "./matchers.js";

export const PROVIDER = "claude-code";

// Same EVENT_MAP as Python version (15 events)
export const EVENT_MAP: Record<string, string> = { ... };
export const EVENT_MAP_REVERSE: Record<string, string> = { ... };

/**
 * Field mapping (Canonical → Claude Code) — mirrors Python Task 4.4 exactly:
 *
 * | Canonical Field              | Claude Code Field       | Transformation                        |
 * |------------------------------|-------------------------|---------------------------------------|
 * | hook.event                   | top-level key in hooks  | Translated via EVENT_MAP              |
 * | hook.matcher                 | hook_entry.matcher      | resolve_matcher_to_provider; absent → "" |
 * | hook.handler.command         | hook.command            | Copy as-is                            |
 * | hook.handler.timeout         | hook.timeout            | Copy as-is (seconds)                  |
 * | hook.handler.type            | hook.type               | Copy as-is                            |
 * | hook.blocking                | (not emitted)           | CC does not use blocking field        |
 * | hook.handler.platform        | (dropped with warning)  | platform_commands not supported       |
 * | hook.handler.cwd             | (dropped with warning)  | configurable_cwd not supported        |
 * | hook.handler.env             | (dropped with warning)  | custom_env not supported              |
 * | hook.provider_data["claude-code"] | merged into hook  | Shallow merge                         |
 * | hook.provider_data[other]    | (dropped silently)      | Non-target provider ignored           |
 */

/** Encode canonical manifest to Claude Code native format. */
export function encode(manifest: HookManifest): Record<string, unknown> {
  // Returns {"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [...]}]}}
}

/** Decode Claude Code native format to canonical manifest. */
export function decode(native: Record<string, unknown>): HookManifest {
  // Handles {"hooks": {"PreToolUse": [...]}}
}
```

**Success Criteria:**
- `test -f docs/spec/hooks/examples/typescript/src/claudeCode.ts` → pass — File created

---

### Task 5.5 — TypeScript: geminiCli.ts

**Files:**
- `docs/spec/hooks/examples/typescript/src/geminiCli.ts` (create)

**Depends on:** Task 5.3 (matchers.ts)

**geminiCli.ts — what to implement:**

```typescript
import { HookManifest } from "./manifest.js";
import { resolveMatcherToProvider, decodeMatcherFromProvider } from "./matchers.js";

export const PROVIDER = "gemini-cli";

// Same EVENT_MAP as Python version (11 events)
export const EVENT_MAP: Record<string, string> = { ... };
export const EVENT_MAP_REVERSE: Record<string, string> = { ... };

export const TIMEOUT_SECONDS_TO_MS = 1000;

/** Encode canonical manifest to Gemini CLI native format. */
export function encode(manifest: HookManifest): Record<string, unknown> {
  // Returns {"hooks": [{"trigger": "BeforeTool", "toolMatcher": ..., "command": ..., "timeoutMs": ...}]}
}

/** Decode Gemini CLI native format to canonical manifest. */
export function decode(native: Record<string, unknown>): HookManifest {
  // Converts timeoutMs -> timeout (divide by 1000)
}
```

**Success Criteria:**
- `test -f docs/spec/hooks/examples/typescript/src/geminiCli.ts` → pass — File created

---

### Task 5.6 — TypeScript: conformance tests

**Files:**
- `docs/spec/hooks/examples/typescript/tests/conformance.test.ts` (create)

**Depends on:** Tasks 5.1, 5.2, 5.3, 5.4, 5.5

**conformance.test.ts — what to implement:**

Use Vitest. Load test vectors using Node.js `fs` and `path` — test vectors are at
`../../test-vectors/` relative to the `tests/` directory (i.e., `../../../test-vectors`
relative to the `typescript/` root).

```typescript
import { describe, it, expect } from "vitest";
import { readFileSync, readdirSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";
import { parseManifest, ValidationError } from "../src/manifest.js";
import { resolveDecision } from "../src/exitCodes.js";
import { encode as ccEncode, decode as ccDecode } from "../src/claudeCode.js";
import { encode as gcEncode } from "../src/geminiCli.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const VECTORS = resolve(__dirname, "../../../test-vectors");
```

Structure: four describe blocks, mirroring the Python test classes.

**describe("manifest parsing")** — tests that canonical vectors parse and invalid vectors throw:
- `it("parses simple-blocking canonical")` — loads and parses
- `it("parses full-featured canonical")`
- Parametrize (manually, since vitest uses `it.each`) over all invalid vectors → each must throw `ValidationError`

**describe("exit codes")** — tests the §5.3 truth table directly:
- `it("blocking=true exit=2 overrides allow decision")`
- `it("blocking=false exit=2 downgrades to warn_allow")`
- `it("blocking=false deny decision blocks")`
- `it("ask in non-interactive is deny")`
- Plus one test per distinct path in the truth table

**describe("claude code adapter")** — encode and decode:
- `it("encodes simple-blocking correctly")` — compare against test vector
- `it("encodes full-featured correctly")`
- `it("encodes multi-event correctly")`
- `it("decodes roundtrip-source to expected canonical")`

**describe("gemini cli adapter")** — encode:
- `it("encodes simple-blocking correctly")`
- `it("encodes full-featured correctly")`
- `it("encodes multi-event correctly")`

Helper `assertStructurallyEquivalent(actual, expected)` — same logic as Python version.

**Success Criteria:**
- `test -f docs/spec/hooks/examples/typescript/tests/conformance.test.ts` → pass — Test file created
- `cd docs/spec/hooks/examples/typescript && npm install && npx vitest run` → pass — All tests pass
- `cd docs/spec/hooks/examples/typescript && npx vitest run 2>&1 | grep -c 'pass'` → 1 or more — Tests passing

**Minimum coverage per describe block:**
- `describe("manifest parsing")`: >= 5 tests — canonical vectors + invalid rejection
- `describe("exit codes")`: >= 12 tests — one per truth table path (including non-blocking downgrade)
- `describe("claude code adapter")`: >= 4 tests — encode for 3 vectors + decode for roundtrip
- `describe("gemini cli adapter")`: >= 3 tests — encode for simple-blocking, full-featured, multi-event

All four describe blocks must have passing tests; a high count in one block alone does not satisfy this criterion.

---

## Phase 6: Update Housekeeping Files

---

### Task 6.1 — Create examples/README.md

**Files:**
- `docs/spec/hooks/examples/README.md` (create)

**Depends on:** Tasks 4.6 AND 5.6 — both Phase 4 (Python) and Phase 5 (TypeScript) must be
complete so the README can accurately describe both implementations and link to their test
commands. Cannot start until both 4.6 and 5.6 complete.

**Content:**

**§1 — Overview**

Reference implementations for the Hook Interchange Format Specification (Core conformance
level). Each language implementation is standalone — same capabilities, same test vectors,
independently runnable.

**§2 — Running the Python Implementation**

```bash
cd docs/spec/hooks/examples/python
pip install -e ".[dev]"
python -m pytest tests/ -v
```

Module usage:
```python
from hooks_interchange import parse_manifest
from hooks_interchange.claude_code import encode

manifest = parse_manifest(open("my-hooks.json").read())
claude_hooks = encode(manifest)
```

**§3 — Running the TypeScript Implementation**

```bash
cd docs/spec/hooks/examples/typescript
npm install
npx vitest run
```

Module usage (ESM):
```typescript
import { parseManifest } from "./src/manifest.js";
import { encode } from "./src/claudeCode.js";

const manifest = parseManifest(readFileSync("my-hooks.json", "utf8"));
const claudeHooks = encode(manifest);
```

**§4 — Test Vectors**

Both implementations consume the same shared test vectors in `../test-vectors/`. A test
vector index is in [test-vectors/README.md](../test-vectors/README.md).

**§5 — Adding a New Language**

Brief instructions: create `examples/<language>/`, implement the same five modules
(manifest parser, exit codes, matchers, two provider adapters), consume `../test-vectors/`
as the conformance suite.

**§6 — Conformance Level**

These reference implementations target **Core conformance** as defined in hooks.md §8.1.
They do not implement Extended or Full conformance levels. Anyone extending them for
Extended/Full conformance should consult the [Capability Registry](../capabilities.md).

**Success Criteria:**
- `test -f docs/spec/hooks/examples/README.md` → pass — File created
- `grep -q 'pip install' docs/spec/hooks/examples/README.md` → pass — Python instructions present
- `grep -q 'vitest' docs/spec/hooks/examples/README.md` → pass — TypeScript instructions present

---

### Task 6.2 — Update CONTRIBUTING.md

**Files:**
- `docs/spec/hooks/CONTRIBUTING.md` (modify)

**Depends on:** Tasks 2.1 AND 4.6 (new file structure must be finalized and test-vector
conventions must be settled before updating CONTRIBUTING.md's test-vector references). Does
NOT need to wait for Phase 5 — can start as soon as Phase 4 (4.6) completes.

**What to add:**

Insert a new section "Cross-File Dependencies" after the "Types of Contributions" section
and before "Process":

```markdown
## Cross-File Dependencies

When making changes, consult this table to ensure all relevant files are updated:

| Change Type | Files to Update |
|-------------|----------------|
| New event (with blocking behavior) | `events.md`, `blocking-matrix.md`, `CHANGELOG.md` |
| New event (observational only) | `events.md`, `CHANGELOG.md` |
| New provider | `events.md`, `blocking-matrix.md`, `capabilities.md`, `tools.md`, `CHANGELOG.md` |
| New capability | `capabilities.md`, `CHANGELOG.md` |
| New tool name | `tools.md`, `CHANGELOG.md` |
| Core format change | `hooks.md` (version bump), `schema/hook.schema.json`, `CHANGELOG.md` |
| Provider support matrix update | `capabilities.md` or `blocking-matrix.md` (as applicable), `CHANGELOG.md` |
```

Also update the "Test Vector Format" section to reference the new `test-vectors/README.md`
instead of duplicating the conventions:

Replace the "Conventions" subsection with:

```markdown
### Conventions

See [test-vectors/README.md](test-vectors/README.md) for full conventions including
`_comment`, `_warnings`, pairing rules, and validation commands.
```

**Success Criteria:**
- `grep -q 'Cross-File Dependencies' docs/spec/hooks/CONTRIBUTING.md` → pass — New section present
- `grep -q 'New provider' docs/spec/hooks/CONTRIBUTING.md` → pass — Dependency table present
- `grep -q 'test-vectors/README.md' docs/spec/hooks/CONTRIBUTING.md` → pass — Reference to test-vectors README

---

### Task 6.3 — Update CHANGELOG.md

**Files:**
- `docs/spec/hooks/CHANGELOG.md` (modify)

**Depends on:** Tasks 2.1, 4.6, AND 5.6 — the changelog entry must describe all deliverables
accurately, including the Python and TypeScript reference implementations. Cannot start until
spec changes (2.1) and both ref impls (4.6, 5.6) are complete.

**Parallelization note for Phase 6:**
- After 4.6 completes: start 6.2 (CONTRIBUTING.md) immediately — no need to wait for 5.6
- After 5.6 completes: start 6.1 (examples/README.md) and 6.3 (CHANGELOG.md) in parallel

**What to add:**

Insert a new entry at the top of the file (after the header), before the `[0.1.0]` entry:

```markdown
## [core] 0.2.0 — 2026-04-08

### Breaking (from 0.1.0)
- `hooks.md` renumbered: §7-§14 replaced with §7-§9 (Conversion Pipeline, Conformance, Versioning)
- §5.4 (Advanced Output Fields) removed from core spec — output fields now owned by capabilities.md
- `capabilities` field strengthened from SHOULD NOT rely on to MUST NOT make conformance decisions

### Added
- `events.md` — Event Registry extracted from §7
- `tools.md` — Tool Vocabulary extracted from §10
- `capabilities.md` — Capability Registry + Degradation Strategies extracted from §9 + §11
- `blocking-matrix.md` — Blocking Behavior Matrix extracted from §8
- `provider-strengths.md` — Non-normative provider highlights extracted from Appendix A
- `README.md` — Directory index / landing page
- `test-vectors/README.md` — Format contract, pairing rules, vector index
- `examples/python/` — Python reference implementation (Core conformance)
- `examples/typescript/` — TypeScript reference implementation (Core conformance)
- Inline normative minimums in Conformance section (§8): core events by name, core tool names, capability IDs per level
- Cross-file dependency table in CONTRIBUTING.md

### Fixed
- Timeout behavior consolidation: §3.5 and §4 no longer contradict; blocking=false downgrade documented in §4
- §5.4 phantom field editorial dependency resolved by moving output fields to capabilities.md

### Changed
- CONTRIBUTING.md: Test vector conventions now reference test-vectors/README.md instead of duplicating
```

**Success Criteria:**
- `grep -q '0.2.0' docs/spec/hooks/CHANGELOG.md` → pass — New version entry present
- `grep -q 'events.md' docs/spec/hooks/CHANGELOG.md` → pass — New files documented
- `grep -q 'Breaking' docs/spec/hooks/CHANGELOG.md` → pass — Breaking changes documented

---

## End-to-End Verification

Run these after all phases complete to confirm the full result.

### Documentation completeness

```bash
# All new files exist
test -f docs/spec/hooks/README.md
test -f docs/spec/hooks/events.md
test -f docs/spec/hooks/tools.md
test -f docs/spec/hooks/capabilities.md
test -f docs/spec/hooks/blocking-matrix.md
test -f docs/spec/hooks/provider-strengths.md
test -f docs/spec/hooks/test-vectors/README.md
test -f docs/spec/hooks/examples/README.md

# Core spec slimmed
wc -l docs/spec/hooks/hooks.md   # expect 380-450 lines

# No dangling old section numbers in hooks.md
grep -c 'Section [7-9]\|Section 1[0-4]' docs/spec/hooks/hooks.md  # expect 0
```

### Python ref impl

```bash
cd docs/spec/hooks/examples/python
pip install -e ".[dev]" -q
python -m pytest tests/ -v
```

All tests pass.

### TypeScript ref impl

```bash
cd docs/spec/hooks/examples/typescript
npm install
npx vitest run
```

All tests pass.

### Cross-reference integrity

```bash
# All links in README.md resolve to actual files
for f in events.md tools.md capabilities.md blocking-matrix.md provider-strengths.md hooks.md glossary.md security-considerations.md policy-interface.md CONTRIBUTING.md CHANGELOG.md; do
  test -f docs/spec/hooks/$f || echo "MISSING: $f"
done

# events.md links back to hooks.md
grep -q 'hooks.md' docs/spec/hooks/events.md

# capabilities.md links back to hooks.md
grep -q 'hooks.md' docs/spec/hooks/capabilities.md
```

### Spec neutrality (D10)

```bash
# No syllago references in any spec file (provider names are fine, syllago is not)
for f in README.md hooks.md events.md tools.md capabilities.md blocking-matrix.md provider-strengths.md test-vectors/README.md examples/README.md; do
  grep -qi 'syllago' docs/spec/hooks/$f && echo "NEUTRALITY VIOLATION: $f"
done
# Expect: no output
```

---

## Task Dependency Graph

```
Phase 1 (parallel):
  1.1 events.md ─────────┐
  1.2 tools.md ──────────┤
  1.3 capabilities.md ───┤──→ 2.1 slim hooks.md ──→ 3.2 README.md
  1.4 blocking-matrix.md ┤
  1.5 provider-strengths ┘

Phase 3 (independent):
  3.1 test-vectors/README.md (no deps)

Phase 4 (Python, fully parallel — no Phase 3 dependency):
  4.1 manifest.py ───────┐
  4.2 exit_codes.py ─────┤
  4.3 matchers.py ───────┤──→ 4.4 claude_code.py ──┐
                              4.5 gemini_cli.py ────┤──→ 4.6 test_conformance.py ──→ 6.2 CONTRIBUTING.md

Phase 5 (TypeScript, parallel with Phase 4):
  5.1 manifest.ts ───────┐
  5.2 exitCodes.ts ──────┤
  5.3 matchers.ts ───────┤──→ 5.4 claudeCode.ts ──┐
                              5.5 geminiCli.ts ────┤──→ 5.6 conformance.test.ts

Phase 6 (after 4.6 and 5.6):
  4.6 + 5.6 ──→ 6.1 examples/README.md
  4.6 + 5.6 ──→ 6.3 CHANGELOG.md
  2.1 + 4.6  ──→ 6.2 CONTRIBUTING.md  (does not need 5.6)
```

Total tasks: 21
Estimated time: 2-3 hours for a focused executor working through phases sequentially,
or ~1 hour with Phase 1 and Phase 4/5 parallelized.
