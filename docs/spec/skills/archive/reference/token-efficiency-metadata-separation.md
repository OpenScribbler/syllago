# Token Efficiency: Metadata Separation Decision

**Date:** 2026-03-31
**Decision:** Option B — Reference field (`metadata_file`) with external sidecar
**Consensus:** 4-1 (Solo Author, Registry Operator, Spec Purist, Community Member for; Agent Developer against)

---

## The Problem

The Agent Skills spec intentionally uses only two frontmatter fields — `name` and `description` — to be token-efficient. When an AI agent loads a skill into context, minimal frontmatter means minimal wasted tokens.

The metadata convention added ~30 lines of YAML frontmatter (provenance, attestation, triggers, expectations, tags, status, durability, supported_agents). On agents that don't strip frontmatter before context injection (confirmed: Codex CLI, GitHub Copilot), ALL of this metadata is injected into the model's context on every skill activation. This directly contradicts the original design philosophy.

## Options Evaluated

### Option A: Sidecar File

Metadata lives in a separate file (e.g., `SKILL.meta.yaml`) alongside SKILL.md. The SKILL.md keeps only `name` + `description`. Tooling reads the sidecar; the model never sees it.

**Strengths:** Physical separation is the only guarantee metadata never reaches the model. Cleanest separation of concerns. Unix philosophy — one file, one purpose.

**Weaknesses:** Two files per skill creates maintenance burden. Authors forget to create the sidecar. Skills copied without their sidecar lose all identity/metadata. No link from SKILL.md to sidecar means tooling must rely on naming conventions.

### Option B: Reference Field (SELECTED)

Add one optional field to frontmatter (`metadata_file: "./SKILL.meta.yaml"`) that points to the full metadata. Still just 3 lines in frontmatter. Tooling dereferences the pointer.

**Strengths:** Explicit link from skill to metadata — no guessing, no convention-based discovery. One extra line instead of 30. Degrades gracefully (absent field = no metadata). Minimal violation of original spec philosophy.

**Weaknesses:** Still adds one field to frontmatter. Sets a precedent for "just one more field." Metadata file can drift out of sync with the SKILL.md.

### Option C: Normative Strip Requirement

Keep all metadata in frontmatter but require agents to strip everything except `name`, `description`, and trigger-relevant fields before context injection.

**Strengths:** One-file authoring. No new file formats.

**Weaknesses:** Unenforceable — agents that don't strip today (Codex CLI, GitHub Copilot) have no incentive to change. Pushes the fix to consumers instead of fixing the source. Relies on every agent implementing the same stripping logic correctly.

### Option D: Two-Tier Frontmatter

Define a "model-visible" section and a "tooling-only" section (e.g., nested under `_tooling:`) within the same frontmatter block. Agents strip the tooling-only section.

**Strengths:** One file. Clear conceptual separation. Authors write everything in one place.

**Weaknesses:** The `_tooling:` boundary isn't a clean parsing boundary — agents doing regex or line-by-line extraction can't reliably detect where the tooling section ends without YAML-aware parsing. Agents that dump raw frontmatter into context still see everything.

## Debate Progression

| Persona | Round 1 | Round 3 | Why they shifted |
|---------|---------|---------|-----------------|
| Solo Author | D (Two-tier) | **B (Reference)** | Convinced by Community Member that `_tooling:` parsing boundary isn't clean for non-YAML-aware agents |
| Agent Developer | A (Sidecar) | **A (Sidecar)** | Held firm — physical separation is the only guarantee. Acceptable minority position. |
| Registry Operator | D (Two-tier) | **B (Reference)** | Moved by Spec Purist's argument that explicit pointer prevents detached-metadata guessing |
| Spec Purist | A (Sidecar) | **B (Reference)** | Conceded one field is an acceptable minimal violation of the original two-field design |
| Community Member | A (Sidecar) | **B (Reference)** | Moved by the explicit-link argument: reference field prevents convention-based discovery ambiguity |

## Key Arguments That Changed Minds

1. **Parsing complexity kills Option D.** Some agents do regex/line-by-line frontmatter extraction without a YAML parser. The `_tooling:` nested key requires YAML indentation awareness to determine where the section ends. Physical separation (A) or a pointer (B) avoids this entirely.

2. **Convention-based discovery is fragile.** Option A without a reference field means tooling must guess: "is there a SKILL.meta.yaml next to this SKILL.md?" The Community Member insisted: if the field is absent, tooling MUST NOT search for sidecars. Explicit or nothing. Option B provides the explicit link.

3. **One field is not thirty.** The Spec Purist initially wanted zero additional frontmatter fields. But `metadata_file` is a pointer, not a payload. One line that says "metadata is over there" is categorically different from 30 lines of metadata in context. This distinction convinced the Spec Purist to accept the compromise.

4. **Failure modes favor Option B.** Missing sidecar with Option A = skill works but has no metadata and nobody knows. Missing sidecar with Option B = skill works, tooling sees a broken reference, can warn the author. The explicit link creates accountability.

5. **Physical separation isn't guaranteed anyway.** The Agent Developer's argument for Option A (only physical separation guarantees metadata never reaches the model) is technically correct but practically irrelevant for agents that already strip frontmatter. For agents that don't strip, one extra line (the reference field) is negligible compared to 30 lines of metadata.

## Conditions for Option B

The panel set five conditions that must be met:

1. **`metadata_file` is strictly optional.** A SKILL.md without it is fully valid under both the Agent Skills spec and this convention. No degraded status, no warnings.

2. **No convention-based discovery.** If the field is absent, tooling MUST NOT search for metadata files by naming convention. Absent field = no metadata exists. Period.

3. **Sidecar schema fully specified.** The metadata file format must be completely defined in the convention spec — not "YAML with some fields" but a complete schema with required/optional fields, types, and validation rules.

4. **This is the ONE allowed frontmatter extension.** The spec must explicitly state that `metadata_file` is the convention's single addition to frontmatter. This prevents scope creep ("just one more field" repeated indefinitely).

5. **Zero impact on stripping agents.** Agents that already strip frontmatter (Claude Code, Gemini CLI, Cline, Roo Code, OpenCode) continue working with zero changes. Agents that don't strip see at most one extra line instead of 30.

## Dissenting Position

The Agent Developer maintained that only physical separation (Option A) truly guarantees metadata never reaches the model. This is a valid technical argument: Option B still puts one field in frontmatter, and agents that concatenate raw frontmatter into the prompt will include it. However, the panel judged that one line of reference metadata is an acceptable trade-off versus the discovery and consistency benefits the reference field provides.

## Impact on the Spec

This decision affects the spec architecture significantly:

- **SKILL.md frontmatter** returns to near-original simplicity: `name`, `description`, `license` (from Agent Skills spec), and optionally `metadata_file` (from this convention)
- **All convention metadata** (provenance, attestation, triggers, expectations, tags, status, durability, supported_agents) moves to the sidecar file
- **The convention field `metadata_spec`** (the version identifier) moves to the sidecar file — it's tooling metadata, not model-relevant
- **Agents need no changes** — they read SKILL.md as before, seeing 3-4 lines of frontmatter instead of 30+
- **Tooling reads both files** — SKILL.md for the skill content, sidecar for metadata

## Resolved Follow-Up Decisions

**Sidecar format:** Flat YAML (no `metadata:` wrapper). The entire file IS metadata — wrapping it in `metadata:` is redundant. Top-level keys are `metadata_spec`, `provenance`, `triggers`, etc.

**Recommended filename:** `SKILL.meta.yaml`. Mirrors the SKILL.md naming, clearly associated, visible in directory listings. The `metadata_file` field accepts any relative path, but the spec RECOMMENDS this default.

## Pending: Spec Changes

The following changes need to be applied to `metadata_convention.md` in the next session:

1. Add `metadata_file` as the ONE new optional frontmatter field
2. Define the sidecar file format (flat YAML schema with all current convention fields)
3. Move `metadata_spec` and all convention fields from frontmatter to sidecar
4. Update all YAML examples (minimal, full, provenance) to show two-file pattern
5. Update "Relationship to Agent Skills Spec" section — one field, not a nested schema
6. Update Appendix A and B — convention fields consumed from sidecar, not frontmatter
7. Update discussion doc to reflect the architectural change
