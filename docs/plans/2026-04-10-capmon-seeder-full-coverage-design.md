# Capmon Seeder: Full Provider Coverage — Design Document

**Goal:** Build a repeatable, bead-driven workflow to inspect every provider's source documentation, establish canonical capability keys for the skills content type, and seed all 14 provider capability YAMLs with real data.

> **Scope note:** This design covers skills only. Hooks capability seeding is a separate mapping exercise against the existing HIF spec (`docs/spec/hooks/`) — not this pattern. MCP, rules, agents, and commands follow subsequent iterations once the skills pattern is proven.

**Decision Date:** 2026-04-10

---

## Problem Statement

The `syllago capmon` pipeline can fetch and extract data from all 14 active providers, but the seeder (`syllago capmon seed`) only produces meaningful output for 2 of them (crush and roo-code) because only one recognizer exists — the `Skill.*` Go struct pattern. Every other provider's capability YAML is an empty stub.

Closing this gap requires:
1. A repeatable inspection workflow that reads provider source docs and extracted cache
2. Structured "seeder specs" that human reviewers can validate before any recognizer code is written
3. Per-provider recognizer functions that translate extracted fields into canonical capability YAML entries
4. An ongoing `syllago capmon inspect` subcommand so the workflow scales to new providers

There's also a deeper strategic goal: the capability YAML entries for skills need to use **canonical keys** (like `frontmatter_name`, `file_location_global`) that can seed a skills interchange format specification — parallel to what HIF already provides for hooks.

---

## Proposed Solution

A three-phase workflow:

**Phase 1 — Inspect:** For each provider × content type, an inspection bead (a prompted LLM workflow, not a Go CLI command) reads the provider's format reference doc and extracted cache side-by-side, then produces a structured seeder spec YAML. Each spec proposes: what canonical capability keys apply, what provider-native field names map to them, and what's missing from the extraction.

**Phase 2 — Review:** Human reads the seeder specs, approves proposed mappings or adjusts them, and sets `human_action: approve | adjust | skip` on each spec.

**Phase 3 — Implement:** For each approved spec, a subagent writes a `recognizeXxxSkills()` Go function and corresponding tests. The function is wired into the dispatch table in `recognize.go`. `syllago capmon seed --provider=<slug>` then populates the capability YAML.

This design covers **skills first**. The same workflow (same `capmon inspect` subcommand, same spec format, same recognizer dispatch pattern) applies to hooks, MCP, and other content types in subsequent iterations.

---

## Architecture

### Inspection Bead (Not a Go CLI Command)

Inspection is an LLM reasoning step — reading semi-structured docs and proposing canonical mappings. This cannot be encoded as a deterministic Go function. There is no Task SDK in the binary.

Instead, inspection is a **documented bead workflow**. Each inspection bead:
1. Reads `docs/provider-formats/<slug>.md` — the human-authored format reference (ground truth)
2. For providers missing a format doc (currently crush, pi): reads source manifest + raw cache content instead
3. Reads all `.capmon-cache/<slug>/<source_id>/extracted.json` files for the target content type
4. Reads `.capmon-cache/<slug>/<source_id>/raw.bin` for source excerpts (included in spec for human review)
5. Writes/updates `docs/provider-formats/<slug>.md` if gaps are found (proposed additions go to `<slug>.md.proposed-additions` for human merge if a human-authored doc exists)
6. Writes `.develop/seeder-specs/<slug>-<content-type>.yaml`

The Go CLI surface area for inspection is `syllago capmon validate-spec`:

```
syllago capmon validate-spec --provider=<slug> [--content-type=skills]
```

`validate-spec` is a real, testable Go command that:
- Validates the seeder spec YAML against the spec schema (required fields, enum values)
- Checks that `human_action` is `approve` or `adjust` (not the empty string)
- Checks that `reviewed_at` is set
- Returns a non-zero exit code with an actionable error message if any check fails

The `seed` command also validates `human_action` before proceeding (see Seeder Command Updates).

### Seeder Spec Format

Location: `.develop/seeder-specs/<provider>-<content-type>.yaml`

```yaml
provider: claude-code
content_type: skills
format: markdown
format_doc_provenance: human    # human | subagent (subagent-generated docs produce inferred mappings)
extraction_gaps:
  - "Primary selector not set — fallback mode includes nav/example items"
  - "Table at h2='Frontmatter' not targeted; fields buried in list items"
source_excerpt: |
  ## Skills Frontmatter
  | Field | Required | Description |
  |-------|----------|-------------|
  | `name` | Yes | Display name for the skill |
  | `description` | Yes | When Claude should invoke this skill |
  | `disable-model-invocation` | No | Prevent LLM calls |
  | `user-invocable` | No | Whether the user can trigger it |
proposed_mappings:
  - canonical_key: display_name
    supported: true
    mechanism: "yaml frontmatter key: name (required)"
    source_field: "Skill.Name"    # which extracted field triggered this mapping
    source_value: "name"          # the value observed
    confidence: confirmed         # confirmed | inferred | unknown
    notes: ""
  - canonical_key: description
    supported: true
    mechanism: "yaml frontmatter key: description (required)"
    source_field: "Skill.Description"
    source_value: "description"
    confidence: confirmed
    notes: ""
  - canonical_key: project_scope
    supported: true
    mechanism: ".claude/skills/<name>/SKILL.md"
    source_field: "file_location"
    source_value: ".claude/skills/"
    confidence: confirmed
    notes: ""
human_action: ""    # approve | adjust | skip (set by reviewer — REQUIRED before seeding)
reviewed_at: ""     # ISO 8601 timestamp (set by reviewer — REQUIRED before seeding)
notes: ""           # reviewer notes
```

**`proposed_mappings` structure:** One entry per canonical key (not one per sub-property). The `source_field` and `source_value` fields link each mapping back to the extracted cache, so the Phase 3 implementer can write a recognizer without re-inspecting the cache. The `confidence` field (not `extraction_quality`) expresses per-mapping evidence quality.

**`human_action` enforcement:** The `seed` command and `validate-spec` command both refuse to proceed if `human_action` is empty or if `reviewed_at` is unset. Subagents must never pre-set `human_action` — it must be empty in every spec they write.

### Canonical Skills Capability Keys

> **Status: Provisional.** This table is defined here as a working draft. It must NOT be committed to `docs/spec/skills/capabilities.md` until the inspection phase is complete for all 14 providers and key names have stabilized across the full provider landscape. The success criterion for spec commitment is: 10+ provider inspections reviewed + no pending key renames + the spec file includes inference rules and degradation defaults (see "When the key table graduates to docs/spec/" below).

Canonical keys name **concepts** at a provider-neutral level of abstraction — not schema fields. A key must be stable across providers that use YAML frontmatter, JSON config, TOML, or any other format. The naming convention is `snake_case` and semantic-behavioral (what the skill system *can do* or *support*), not schema-structural (what fields exist in YAML).

| Key | Replaces (old name) | Concept |
|-----|---------------------|---------|
| `display_name` | `frontmatter_name` | Skill declares a human-readable display name |
| `description` | `frontmatter_description` | Skill declares a description (used for invocation routing) |
| `license` | `frontmatter_license` | Skill declares a license field |
| `compatibility` | `frontmatter_compatibility` | Skill declares platform/version compatibility constraints |
| `metadata_map` | `frontmatter_metadata` | Skill declares an arbitrary key-value metadata block |
| `disable_model_invocation` | `frontmatter_disable_model_invocation` | Provider allows skill to suppress LLM calls |
| `user_invocable` | `frontmatter_user_invocable` | Skill can be triggered directly by the user (not just the model) |
| `version` | `frontmatter_version` | Skill declares a version string |
| `project_scope` | `file_location_project` | Provider supports project-local skill storage |
| `global_scope` | `file_location_global` | Provider supports user-global skill storage |
| `shared_scope` | `file_location_shared` | Provider supports org/shared skill storage |
| `canonical_filename` | `file_naming_skill_md` | Provider uses a fixed canonical filename (e.g., SKILL.md, skill.yaml) |
| `custom_filename` | `file_naming_custom` | Provider uses a provider-specific or free-form filename scheme |

**Naming rationale:** `display_name`, `description`, `version`, `license`, `compatibility` are the underlying concepts; they are mechanism-agnostic. `project_scope`, `global_scope`, `shared_scope` describe storage scope concepts — the mechanism column captures the actual path. `disable_model_invocation`, `user_invocable` are behavioral capabilities. `canonical_filename` and `custom_filename` are tentative — inspection may reveal they belong as mechanism annotations under scope keys rather than top-level capability entries.

New keys are added when the inspection phase discovers provider-exclusive fields. Provider-exclusive capabilities go in `provider_exclusive`, not the canonical map. New canonical key proposals must be reviewed by the human before being added to this table — propose in the seeder spec `notes` field.

#### When the key table graduates to `docs/spec/skills/capabilities.md`

The spec file at `docs/spec/skills/capabilities.md`, when it exists, must include:
- **Inference rule per key**: the observable property (in extracted cache or format doc) that causes a recognizer to emit `supported: true`
- **Default degradation strategy per key**: what adapters must do when converting a skill to a provider that doesn't support this capability
- **File header**: declares normative status, scope, and version

Until these are present, the file is a glossary, not a normative spec.

### Recognizer Architecture

Each provider × content type gets a dedicated recognizer function. Shared helpers emerge naturally when two providers truly use the same pattern (confirmed by inspection, not assumed).

File layout:
```
cli/internal/capmon/
  recognize.go                    # registry + shared helpers (existing, updated)
  recognize_claude_code.go        # recognizeClaudeCodeSkills() + init() registration
  recognize_cline.go              # recognizeClineSkills() + init() registration
  recognize_kiro.go               # recognizeKiroSkills() + init() registration
  recognize_crush.go              # recognizeCrushSkills() + init() registration
  recognize_roo_code.go           # recognizeRooCodeSkills() + init() registration
  ...one file per provider...
```

**Registration pattern:** Per-provider files self-register via `init()`, replacing the hardcoded `switch` dispatch. This mirrors the extractor package pattern already in the codebase:

```go
// recognize.go — registry
var recognizerRegistry = map[string]func(map[string]FieldValue) map[string]string{}

func RegisterRecognizer(provider string, fn func(map[string]FieldValue) map[string]string) {
    recognizerRegistry[provider] = fn
}

func RecognizeContentTypeDotPaths(provider string, fields map[string]FieldValue) map[string]string {
    fn, ok := recognizerRegistry[provider]
    if !ok {
        // Warn: no recognizer registered for this provider
        return make(map[string]string)
    }
    result := make(map[string]string)
    mergeInto(result, fn(fields))
    return result
}
```

```go
// recognize_claude_code.go — example provider file
func init() {
    RegisterRecognizer("claude-code", recognizeClaudeCodeSkills)
}

func recognizeClaudeCodeSkills(fields map[string]FieldValue) map[string]string {
    // ...
}
```

A test in `recognize_test.go` asserts that all known provider slugs (from `docs/provider-sources/`) have a registered recognizer. This gives the registration pattern teeth — a missing wire is a test failure, not a silent empty-output bug.

**Capability YAML schema:** Each capability entry in the provider capability YAML uses a four-field schema:

```yaml
skills:
  capabilities:
    display_name:
      supported: true
      mechanism: "yaml frontmatter key: name (required)"
      confidence: confirmed   # confirmed | inferred | unknown
      notes: ""
```

- `confidence: confirmed` — mapping was derived from a human-authored format doc
- `confidence: inferred` — mapping was derived from cache extraction or subagent reasoning
- `confidence: unknown` — not yet assessed

The recognizer must explicitly write `confidence` into each capability entry — it is not optional.

**Dot-path translation:** The seeder spec no longer contains dot-paths. The recognizer owns the translation from canonical key to dot-paths:

```go
func capabilityDotPaths(key, mechanism, confidence string) map[string]string {
    prefix := "skills.capabilities." + key
    return map[string]string{
        prefix + ".supported":   "true",
        prefix + ".mechanism":   mechanism,
        prefix + ".confidence":  confidence,
    }
}
```

The existing `recognizeSkillsGoStruct` shared helper REMAINS in `recognize.go` as a utility function that individual recognizer functions may call, but it is no longer called directly from dispatch. Crush and roo-code inspections confirm whether they share it — confirmed by inspection, not assumed.

### Seeder Command Updates

`LoadAndRecognizeCache` gains a `provider` parameter, passed through to `RecognizeContentTypeDotPaths`. The seed command already has `--provider` and passes it through.

The `seed` command validates the seeder spec before processing:
1. Reads `.develop/seeder-specs/<slug>-skills.yaml` if present
2. Returns an error if `human_action` is empty string or any value other than `approve`, `adjust`, or `skip`
3. Returns an error if `reviewed_at` is empty
4. Logs a warning (not an error) if `confidence` is `inferred` on any mapping — proceeds, but surfaces the uncertainty

### New Provider Onboarding Workflow

For future providers, the sequence is:
```
1. docs/provider-sources/<slug>.yaml      ← write manually (done once)
2. docs/provider-formats/<slug>.md        ← write manually or verify if auto-generated
3. syllago capmon run --stage=fetch-extract --provider=<slug>
4. [Run inspection bead for <slug> × skills]
   → reads: docs/provider-formats/<slug>.md
   → reads: .capmon-cache/<slug>/*/extracted.json
   → writes: .develop/seeder-specs/<slug>-skills.yaml
5. Human reviews .develop/seeder-specs/<slug>-skills.yaml
   → sets human_action: approve | adjust | skip
   → sets reviewed_at: <ISO timestamp>
   → optionally runs: syllago capmon validate-spec --provider=<slug>
6. [Implement recognizer bead for <slug> using approved spec as source of truth]
7. syllago capmon seed --provider=<slug>
```

Steps 3-7 are repeatable when the provider updates its docs. Steps 1-2 are done once per provider.

**Re-inspection trigger:** Re-inspection should run when a provider's documentation URL changes (detectable via hash diff in `syllago capmon run`) or when a capability review flags missing entries. The pipeline itself (Step 3) is automated; the inspection bead (Step 4) and human review (Step 5) require manual initiation.

---

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Recognizer scope | Per-provider × content type | Bespoke formatting per provider; code sharing emerges from inspection, not assumption |
| Spec review | Human-in-the-loop before code | Wrong mappings baked into recognizers are hard to find; specs are cheap to review |
| Canonical key definition | Provisional in design doc, not yet in docs/spec/ | Keys must be validated across all 14 providers before committing to a spec path; two data points (crush, roo-code) is insufficient |
| Canonical key naming | Concept-level (display_name) not schema-structural (frontmatter_name) | Provider-neutral names survive non-YAML providers; structural names become a migration when committed to 14 YAMLs |
| `inspect` implementation | Bead workflow, not Go CLI | Inspection requires LLM reasoning over semi-structured text; no Task SDK exists in the binary; `validate-spec` fills the Go CLI need |
| Format docs | Human-authored docs get proposed additions as sidecars; subagent-generated docs (crush, pi) are labeled `method: subagent` | Prevents subagent from silently corrupting human-verified docs |
| Shared GoStruct helper | Retained as utility, not dispatch | crush and roo-code inspection confirms whether they share it; dispatch is per-provider |
| Recognizer registration | init()-based registry, not hardcoded switch | Mirrors extractor pattern; compile-time registration gap caught by a test asserting all known slugs are registered |
| Capability confidence | Per-entry `confidence` field in capability YAML | Distinguishes confirmed (from format doc) from inferred (from cache); downstream spec authors need this signal |
| `human_action` validation | Enforced in seed + validate-spec commands | An unenforced gate is not a gate; empty string must return an error |

---

## Data Flow

```
[Inspection bead — claude-code × skills]
  │
  ├─ reads: docs/provider-formats/claude-code.md  (ground truth)
  ├─ reads: .capmon-cache/claude-code/skills.*/extracted.json
  ├─ reads: .capmon-cache/claude-code/skills.*/raw.bin  (for excerpt)
  │
  ├─ writes: docs/provider-formats/claude-code.md.proposed-additions  (if gaps found)
  └─ writes: .develop/seeder-specs/claude-code-skills.yaml
             (human_action: "", reviewed_at: "" — must be empty)

[Human reviews spec, sets human_action: approve, reviewed_at: <timestamp>]
[Optionally: syllago capmon validate-spec --provider=claude-code]

syllago capmon seed --provider=claude-code
  │
  ├─ validates: .develop/seeder-specs/claude-code-skills.yaml
  │             human_action must be approve|adjust (not empty)
  │             reviewed_at must be set
  ├─ reads: .capmon-cache/claude-code/**/extracted.json
  ├─ calls: RecognizeContentTypeDotPaths("claude-code", allFields)
  │          → recognizerRegistry["claude-code"](fields)
  │            → recognizeClaudeCodeSkills(fields)
  │              → skills.capabilities.display_name.supported = "true"
  │              → skills.capabilities.display_name.mechanism = "yaml frontmatter key: name (required)"
  │              → skills.capabilities.display_name.confidence = "confirmed"
  │              → ...
  └─ writes: docs/provider-capabilities/claude-code.yaml
```

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Provider has no format doc | Inspection bead reads source manifest + raw cache; writes new format doc labeled `method: subagent`; all mappings get `confidence: inferred` |
| Extraction quality is poor (0 fields) | Spec notes extraction gaps; `proposed_mappings` uses format doc only; entries get `confidence: inferred` |
| Human sets `human_action: skip` | Provider gets `skills.supported: false` or is omitted; no recognizer written |
| `human_action` is empty string | `seed` and `validate-spec` return error: "seeder spec for <slug> has not been reviewed; set human_action before seeding" |
| Recognizer produces no dot-paths | Seed command logs a warning; capability YAML gets `skills.supported: true` only |
| No recognizer registered for provider | `RecognizeContentTypeDotPaths` logs warning: "no recognizer registered for provider <slug>"; returns empty map (not panic) |
| New canonical key discovered | Key is proposed in seeder spec `notes` field; human approves before adding to this table; key table in `docs/spec/skills/` is not updated without normative content |

---

## Success Criteria

**Skills seeder (this project):**
- [ ] Inspection bead prompt exists and is documented in `docs/workflows/inspect-provider-skills.md`
- [ ] `syllago capmon validate-spec --provider=<slug>` exists, validates seeder spec schema, and enforces `human_action` + `reviewed_at`
- [ ] 14 seeder specs exist in `.develop/seeder-specs/*-skills.yaml`, all with `human_action: approve` and `reviewed_at` set
- [ ] 14 provider-specific recognizer functions exist, registered via `init()`, with tests passing
- [ ] A test in `recognize_test.go` asserts all known provider slugs are registered in `recognizerRegistry`
- [ ] `syllago capmon seed --provider=<slug>` produces a non-empty `content_types.skills` section with `confidence` fields for all 14 providers
- [ ] crush.md and pi.md exist in `docs/provider-formats/` (labeled `method: subagent` if generated)
- [ ] Canonical skills capability key table is stable in this design doc across all 14 inspection reviews

**Deferred (not blocking skills seeder):**
- [ ] Canonical skills capability key table committed to `docs/spec/skills/capabilities.md` — requires: 10+ inspections reviewed, no pending key renames, inference rules and degradation defaults present per key

**Provider onboarding:**
- [ ] Inspection bead and `validate-spec` work for a net-new provider (tested with a scratch provider)
- [ ] New provider onboarding checklist is documented in `docs/adding-a-provider.md`

---

## Scope Boundary

This design covers **skills only**.

- **Hooks:** The HIF spec at `docs/spec/hooks/` already defines canonical names, inference rules, and degradation strategies. The hooks seeder is a *mapping exercise against an existing spec* — a different workflow from the skills canonical model definition exercise documented here. Do not apply this design pattern to hooks.
- **MCP, rules, agents, commands:** Subsequent iterations. Each will determine whether they get canonical model definition (like skills) or spec mapping (like hooks) based on whether an interchange format already exists.

The claim that this design "establishes the pattern for all content types" is false. It establishes the pattern for content types without a pre-existing interchange format spec.

---

## Open Questions

- `canonical_filename` and `custom_filename` — do these belong as top-level capability keys or as mechanism annotations under scope keys (`project_scope`, `global_scope`)? Defer to inspection phase — if no provider uniquely declares file naming as a separate capability from scope, merge them.
- When a provider's inspection bead produces `confidence: inferred` on all mappings (crush, pi — no human format doc), should the recognizer emit capability entries at all, or wait for human verification? Current decision: emit with `confidence: inferred`, warn on seed. Revisit if this creates noise in the capability YAML.
- Should `reviewed_at` be a required field on ALL seeder specs, or only on approved/adjusted ones? (Lean toward: required on any spec that will be seeded; skip specs may omit it.)

---

## Panel Review Summary

This design was reviewed by a 5-panelist async panel (remy, ia, techwriter, engineer, standards) over 3 rounds. All 8 panel proposals were endorsed 5/5. Key changes from the original design:

1. **Inspect → bead-only workflow** (removed Go CLI command; added `validate-spec` CLI)
2. **Key renaming** (concept-level names replacing schema-structural names)
3. **Dot-paths removed from seeder spec** (recognizer owns schema-to-path translation)
4. **proposed_mappings restructured** (one entry per key; added `source_field`, `source_value`, `confidence`)
5. **`confidence` field added** to capability YAML schema (confirmed/inferred/unknown)
6. **init()-based registration** replacing hardcoded dispatch switch
7. **`human_action` enforcement** in seed and validate-spec commands
8. **Canonical key table deferred** from `docs/spec/skills/` until 14 inspections reviewed

## Next Steps

Ready for implementation planning with the `Plan` skill.
