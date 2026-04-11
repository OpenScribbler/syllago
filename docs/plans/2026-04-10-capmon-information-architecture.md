# capmon Information Architecture

**Status:** Design complete — panel-reviewed, ready for plan/implementation  
**Brainstorm date:** 2026-04-10  
**Supersedes:** The original capmon pipeline design and SeederSpec-first approach

---

## Problem Statement

The existing capmon pipeline has the inspection bead doing two jobs with incompatible output requirements:

1. **Pipeline job** — produce structured `proposed_mappings` the seeder can consume → capability YAML
2. **Documentation job** — capture provider-specific knowledge for humans → format docs

Both outputs were crammed into one artifact (SeederSpec), and the documentation job had no reliable delivery path. Provider-specific details (Amp's MCP bundling, Cline's three-tier loading, Claude Code's 250-char truncation) fell between the cracks. The `provider_extensions` field was considered and rejected: optional fields that produce no downstream effect have near-zero fill rate.

The deeper problem: the inspection bead was reading format docs as input and writing SeederSpecs as output, but nothing wrote back to the format docs when new things were discovered.

---

## Design Direction: Format Doc as Source of Truth

**Invert the model.** The format doc is the primary work product — an LLM populates it from source material, and the pipeline derives everything else from it deterministically.

```
Source material (source code, llms.txt, docs, changelogs)
        │
        ▼
Format doc agent (LLM)
  writes: docs/provider-formats/<slug>.yaml   ← single source of truth
        │
        ▼
capmon derive (Go, deterministic)
  reads: format doc YAML
  writes: seeder spec YAML (auto-generated)
        │
        ▼
capmon seed (existing)
  reads: seeder spec → writes capability YAML
```

Human review happens by editing the format doc directly. Merging a format doc change is the approval signal. No separate review gate artifact.

### Supersession

The 2026-04-08 capmon pipeline design is superseded by this document. The `human_action` gate on `SeederSpec` is deprecated. Implementations building against this design MUST NOT enforce `human_action` validation for auto-derived specs. The `human_action`, `reviewed_at`, and `ValidateSeederSpec` logic in `seederspec.go` is legacy and will be removed as part of implementing this design. The format-doc-merge gate (merging a PR that updates `docs/provider-formats/<slug>.yaml`) is the canonical approval signal.

---

## Source Priority Hierarchy

For each (provider, content_type), sources are probed in priority order:

| Priority | Source type | Details |
|----------|-------------|---------|
| 1 | **Source code** | Go structs, TypeScript interfaces, Rust types — highest confidence, explicit field definitions |
| 2 | **llms.txt / llms-full.txt** | Check `docs.provider.com/llms.txt` and `docs.provider.com/llms-full.txt` — curated LLM-friendly index of all docs |
| 3 | **.md URL variants** | Two patterns: append `.md` directly (`page.md`), and child path (`section/page-name.md`). Try both. |
| 4 | **Readability MCP** | HTML via Mozilla Readability — strips nav/ads, leaves prose. Fallback for SPAs and non-markdown sites. |
| 5 | **Changelog / release notes** | Lowest confidence. Used for delta detection and identifying what changed between versions. |

**Full content required.** No partial extraction. Previous iterations missed behavioral details, edge cases, and nuance by capturing only excerpts. The format doc agent reads the full page or full source file. Partial content is not acceptable.

### Source Code Diffing

For source code sources, use structural diffing rather than text diff:
- New/removed functions, methods, types, struct fields
- This is more meaningful than line-level text diff and avoids noise from reformatting

### Doc Page Diffing

For documentation pages (markdown, text, HTML), use unified text diff between old and new cached content. The diff is stored in the GitHub issue — even if the full page is large, the diff is usually small.

---

## Execution Model: CI Detection + Local Remediation

LLM API calls cost money. The pipeline is split to keep CI costs at zero:

- **CI (GitHub Actions)** — detection only. HTTP fetches + hash comparisons. No LLM calls. Essentially free.
- **Local loop** — remediation. Runs on Holden's machine. LLM calls happen here, triggered by issues created by CI.

### CI Job (Detection)

**Schedule:** Mon–Fri every 12 hours, Sat–Sun once per day.

```
capmon check --all
  for each provider × content_type × source URI:
    fetch full content
    compute SHA-256 hash
    compare against .capmon-cache/<slug>/meta.json cached hash
    if fetch_error (4xx, 5xx, timeout, DNS failure):
      DO NOT update cached hash
      DO NOT treat as "no change"
      create or update GitHub issue:
        - label: capmon-change, capmon-fetch-error, provider:<slug>, content-type:<type>
        - body includes: URI, error code, timestamp
    if unchanged:
      update fetched_at timestamp, continue
    if changed:
      store new content as raw.bin in cache
      compute diff (text diff for docs, structural diff for source code)
      create or update GitHub issue:
        - label: capmon-change, provider:<slug>, content-type:<type>
        - if issue already open for this provider+type: append change event
        - body includes: changed URIs, old/new hash, diff preview
```

**Issue deduplication:** Never create a second issue for the same provider+content_type while one is already open. Append new change events to the existing issue. Dedup lookup: `gh issue list --label=capmon-change --label=provider:<slug>` filtered by content type. Each issue body includes a hidden HTML comment `<!-- capmon-check: <slug>/<content_type> -->` as the stable machine-readable anchor for lookup — this survives issue title edits.

**Hash advancement:** The cached hash in `.capmon-cache/<slug>/meta.json` advances only when the format doc PR is merged — not when an issue is opened. This ensures CI that detects a change while an existing PR is still open will append to the open issue rather than treating the change as already handled.

### Local /loop (Remediation)

The local remediation loop is Holden's personal operation of the pipeline — it is not part of the syllago binary and is not available to other syllago users. It runs as the `syllago-capmon-process` Claude Code skill, stored in `$PAI_DIR/skills/syllago-capmon-process/` and symlinked to `~/.claude/skills/`. The detailed processing steps are in that skill's `workflows/process-issues.md`.

The skill references `docs/workflows/update-format-doc.md` and `docs/workflows/graduation-comparison.md` from this repo as the LLM agent instructions for Steps 2c and 2d.

The skill is invoked via a cron job using Claude Code's programmatic mode (`-p` flag), which runs the skill headlessly and exits when complete. The cron schedule is set to run ~15 minutes after each CI action run, so the loop only fires when there is likely new work to do.

```
# Example cron entry (adjust times to match your CI schedule + 15m offset)
# CI: Mon-Fri 06:00 and 18:00 UTC → loop: 06:15 and 18:15
15 6,18 * * 1-5 claude -p "/syllago-capmon-process"
15 6   * * 0,6  claude -p "/syllago-capmon-process"
```

```
# Original /loop reference (replaced by cron):
# /loop 30m syllago-capmon-process

for each open capmon-change issue:

  Step 1 — Fetch (if cache stale or missing)
    Re-fetch source URIs using priority order
    Store full content in .capmon-cache/<slug>/*/raw.bin
    (CI may have already populated this — skip if content_hash matches)

  Step 2 — Format doc update agent (LLM)
    reads: full raw.bin content for changed sources
    reads: existing docs/provider-formats/<slug>.yaml (baseline)
    writes: updated docs/provider-formats/<slug>.yaml
    captures: canonical mappings + provider extensions (named, with source refs)

  Step 3 — Graduation check (conditional)
    structural diff: old format doc vs new format doc
    if new canonical_mappings keys OR new provider_extensions IDs found:
      run graduation comparison agent (LLM):
        reads: ALL docs/provider-formats/*.yaml
        compares: provider_extensions across all providers
        identifies: semantic overlaps (e.g., "Amp's mcp_bundling ≈ Cline's tool_bundling")
      if candidates found:
        create separate GitHub issue:
          label: capmon-graduation
          body: overlapping concepts, which providers, suggested canonical key
    if only existing content was edited (text, confidence, paths): skip graduation entirely

  Step 4 — Derive + seed (Go, deterministic)
    capmon derive: format doc YAML → seeder spec YAML
      (deterministic: identical format doc inputs → identical seeder spec output;
       if two runs produce different output, the format doc has ambiguous content
       that must be corrected before seeding)
    capmon seed: seeder spec → capability YAML

  Step 5 — PR
    git commit: format doc + seeder spec + capability YAML
    gh pr create referencing the capmon-change issue
    comment on issue with PR link → close issue
```

---

## Format Doc Schema

**File:** `docs/provider-formats/<slug>.yaml`  
**Owner:** LLM-populated, human-editable  
**Machine-parseable:** yes — deterministic Go parser derives SeederSpec from it

```yaml
provider: amp
last_fetched_at: "2026-04-10T14:00:00Z"
last_changed_at: "2026-04-08T09:00:00Z"
generation_method: subagent   # subagent | human-edited

content_types:
  skills:
    status: supported   # supported | unsupported | experimental | deprecated
    sources:
      - uri: "https://ampcode.com/manual/agent-skills.md"
        type: documentation          # source_code | documentation | changelog
        fetch_method: md_url         # source_code | llms_txt | md_url | readability
        content_hash: "sha256:abc123..."
        fetched_at: "2026-04-10T14:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml frontmatter key: name (required)"
        confidence: confirmed        # confirmed | inferred | unknown
      description:
        supported: true
        mechanism: "yaml frontmatter key: description (optional)"
        confidence: confirmed
      project_scope:
        supported: true
        mechanism: ".agents/skills/<name>/SKILL.md"
        paths:
          - ".agents/skills/<name>/SKILL.md"
          - ".claude/skills/<name>/SKILL.md"    # compat
        confidence: confirmed
      global_scope:
        supported: true
        mechanism: "~/.config/agents/skills/<name>/SKILL.md"
        paths:
          - "~/.config/agents/skills/<name>/SKILL.md"
          - "~/.claude/skills/<name>/SKILL.md"  # compat
        confidence: confirmed
      canonical_filename:
        supported: true
        mechanism: "SKILL.md (fixed)"
        confidence: confirmed
    provider_extensions:
      - id: mcp_bundling
        name: "MCP server bundling"
        description: "Skills can include an mcp.json file to bundle an MCP server that launches with Amp but stays hidden until the skill is invoked."
        source_ref: "https://ampcode.com/manual/agent-skills.md"
        graduation_candidate: false
        graduation_notes: ""
      - id: toolbox_dir
        name: "Toolbox subdirectory"
        description: "Skills can include a toolbox/ directory of executables available within the skill's context."
        source_ref: "https://ampcode.com/manual/agent-skills.md"
        graduation_candidate: false
        graduation_notes: ""
    loading_model: "Lazy — skill instructions loaded on demand when description matches user request"
    notes: ""
```

### Confidence Value Definitions

The `confidence` field on each `canonical_mappings` entry uses a controlled vocabulary with defined predicates. The LLM agent MUST assign values based on these definitions, not subjective judgment:

| Value | Definition |
|-------|------------|
| `confirmed` | Supported by an explicit field definition in source code (struct field, interface property, type annotation) OR by an unambiguous statement in official documentation that directly names the field and its behavior |
| `inferred` | Derived from examples, behavioral observation, or documentation that implies but does not explicitly state the field (e.g., the field appears in an example but is never formally described) |
| `unknown` | The mapping is believed likely but no source material clearly confirms or denies it — typically used for fields seen in a single example with no further context |

When the source material is ambiguous, prefer `inferred` over `confirmed`. `confirmed` must always be traceable to a specific source passage.

### Canonical Key Vocabulary

Canonical keys are defined in `docs/spec/canonical-keys.yaml`. The `canonical_mappings` block in each format doc MUST only use keys from that list. `capmon derive` exits non-zero if it encounters an unknown key — unknown keys are never silently passed through. If a capability has no canonical key, it belongs in `provider_extensions`, not `canonical_mappings`.

New canonical keys are added only via the graduation process (see below).

The file is structured per content type:

```yaml
content_types:
  skills:
    display_name:
      description: "Human-readable skill name. Maps from frontmatter 'name' key."
      type: string
    description:
      description: "What the skill does. Used by the provider for auto-invocation routing."
      type: string
    project_scope:
      description: "Installation path for project-scoped skills."
      type: string
    global_scope:
      description: "Installation path for user-global skills."
      type: string
    shared_scope:
      description: "Installation path for org-wide or enterprise-distributed skills."
      type: string
    canonical_filename:
      description: "Fixed filename required inside the skill directory (e.g. SKILL.md)."
      type: string
    custom_filename:
      description: "Variable directory name pattern where the directory name is the skill identifier."
      type: string
    disable_model_invocation:
      description: "When true, prevents the provider from auto-invoking the skill."
      type: bool
    user_invocable:
      description: "When false, hides the skill from the user-facing slash menu."
      type: bool
    license:
      description: "License declaration for the skill."
      type: string
    compatibility:
      description: "Tool compatibility constraints declared in frontmatter."
      type: string
    metadata_map:
      description: "Generic metadata container for provider-specific key-value pairs."
      type: object
    version:
      description: "Skill version string declared in frontmatter."
      type: string
```

The initial skills vocabulary is derived from the existing `canonicalKeyFromYAMLKey()` function in `cli/internal/capmon/recognize.go` and the 13 seeder specs in `.develop/seeder-specs/`. Other content types (rules, hooks, MCP, agents, commands) will have their vocabulary sections added as each type is worked on — the file grows through use, not by declaration in advance.

### Provider Extensions

Provider extensions capture capabilities that are real and documented but have no canonical key yet. They are:

- Named (stable `id` field — used for structural diff to detect new additions)
- Sourced (link back to where they were found)
- Flagged for graduation candidacy when appropriate

When a concept appears in two or more provider extensions across different providers, it becomes a graduation candidate.

---

## Graduation Process

Graduation means **adding a new canonical key to `docs/spec/canonical-keys.yaml`**. This happens when two or more providers implement the same concept in their `provider_extensions` under different names. Once a canonical key exists, providers use it in `canonical_mappings` instead of extensions.

**Trigger:** A new `provider_extensions` entry is added to a format doc. Pure edits to existing content do not trigger graduation. Adding a new extension is the signal that something potentially cross-provider has been discovered.

**Detection:** Graduation comparison agent reads all format docs and identifies semantic overlaps across `provider_extensions`. This is LLM judgment — "Amp's `mcp_bundling` and Cline's `tool_bundling` describe the same concept."

**Output:** Separate GitHub issue labeled `capmon-graduation`:
- Which concept(s) overlap
- Which providers have it, under what names
- Suggested canonical key name and definition
- Relevant source refs from each provider

**Resolution:** Human reviews. If promoting:
1. Add new key to `docs/spec/canonical-keys.yaml` with description and type
2. Update relevant `docs/provider-formats/*.yaml` — move the extension to `canonical_mappings` using the new key, remove from `provider_extensions`
3. Seeder specs auto-rederive on next `capmon derive` run

Graduation PRs are always human-initiated. No automated graduation. A concept that only one provider has stays in `provider_extensions` indefinitely — extensions are not second-class citizens, they are the correct home for provider-specific behavior that has no cross-provider equivalent.

---

## New Provider Onboarding

Adding a new provider to syllago has a distinct bootstrap path. Change detection assumes a cached baseline exists — a new provider has none.

### Human setup (one-time, manual)

Two files required before the pipeline can run:

1. **Add provider to `cli/providers.json`** — the syllago provider registry. This is the authoritative list of known providers.

2. **Create `docs/provider-sources/<slug>.yaml`** — the source manifest. Tells capmon where to look and what to expect:

```yaml
provider: newprovider
content_types:
  - skills
  - rules
sources:
  skills:
    - uri: "https://docs.newprovider.com/llms.txt"
      type: documentation
      notes: "llms.txt available — use as source index"
    - uri: "https://github.com/newprovider/newprovider/blob/main/internal/skills/skills.go"
      type: source_code
      notes: "Go struct definitions"
  rules:
    - uri: "https://docs.newprovider.com/configuration"
      type: documentation
      notes: "SPA — use Readability fallback"
```

### Automated first-run

CI detects a new provider on its next scheduled run: provider present in `providers.json` but no entry in `.capmon-cache/<slug>/meta.json`. This is treated as "all sources changed" — the full pipeline runs.

**No special `capmon-new-provider` label needed.** CI creates a standard `capmon-change` issue. The absence of a cached baseline is handled transparently: the format doc agent receives no existing format doc to compare against and produces one from scratch.

### First-run differences from normal updates

| Aspect | Normal update | New provider |
|--------|--------------|--------------|
| Baseline format doc | Exists — agent diffs against it | None — agent writes from scratch |
| Graduation check | Runs if new capabilities added | Always runs — all capabilities are new |
| PR content | Updated format doc | New format doc + seeder spec + capability YAML |
| Review priority | Low (incremental change) | Higher — first complete picture of this provider |

Because a new provider's graduation check always runs (every capability is new), it's especially likely to surface candidates — a new provider frequently implements concepts that existing providers already have.

### Source manifest validation

Before any pipeline run (onboard or otherwise), the source manifest must pass validation. The pipeline refuses to proceed if validation fails.

**Required:** every content type listed in the manifest must have at least one source URI.

```
capmon validate-sources --provider=newprovider

  ✓ skills: 2 source URIs
  ✓ rules: 1 source URI
  ✗ hooks: no source URIs — add at least one before continuing

Exit code 1 if any content type has zero sources.
```

This runs automatically as the first step of `capmon onboard` and `capmon check`. A provider with a missing source URI is an incomplete configuration — the pipeline won't silently produce an empty or fabricated format doc section.

**Scope:** Presence check only — every declared content type must have at least one source URI. URI liveness and content validity are checked at fetch time, not at validation time. `validate-sources` exiting 0 means the manifest is structurally complete, not that the sources are reachable or correct.

The validation is also useful as a standalone check when adding a new provider manually, before the first CI run picks it up.

### `capmon onboard` command (optional, for immediate bootstrap)

Rather than waiting for the next CI schedule, `capmon onboard --provider=<slug>` triggers an immediate first run:

```
capmon onboard --provider=newprovider
  1. Runs capmon validate-sources — exits if any content type has no source URIs
  2. Fetches all sources (same priority order as normal pipeline)
  3. Creates capmon-change issue
```

To process the issue immediately rather than waiting for the next cron run:

```bash
capmon onboard --provider=newprovider
claude -p "/syllago-capmon-process"
```

This is useful when manually adding a provider and wanting to see the format doc generated now rather than waiting for the next scheduled run.

---

## Workflow Doc Specifications

The two LLM workflows below are defined here as specs. During implementation, extract each section verbatim to its target file.

---

### `docs/workflows/update-format-doc.md` (target file)

```markdown
# Format Doc Update

**Invoked by:** capmon-process (Step 2 of the local remediation loop)

**Purpose:** Given new or changed source content for a provider, update the
provider's format doc YAML to reflect what the sources actually say.

## Inputs

- PROVIDER_SLUG — the provider identifier (e.g., amp, claude-code)
- FORMAT_DOC — path to existing docs/provider-formats/<slug>.yaml (absent for new providers)
- CHANGED_SOURCES — one or more raw.bin files under .capmon-cache/<slug>/, each
  containing the full fetched content for one source URI
- CANONICAL_KEYS — docs/spec/canonical-keys.yaml, the authoritative vocabulary

## Your job

Read each changed source in full. Do not summarize or excerpt — the format doc
must capture the full picture from the source material. Compare against the
existing format doc. Update it to reflect what you learned.

For each content type the provider supports:

**1. Map known capabilities to canonical keys.**
Use only keys defined in docs/spec/canonical-keys.yaml under the matching
content type. If the source material confirms a capability that matches a
canonical key, record it in canonical_mappings with mechanism and confidence.

**2. Capture unknown capabilities in provider_extensions.**
If a provider supports something real and documented that has no canonical key,
add it to provider_extensions. Give it:
- A stable id (snake_case, unique within this provider+content_type)
- A clear name
- A description of what it does and why it matters
- A source_ref pointing to the specific page or file where you found it
- graduation_candidate: false (default — set true only if you have positive
  evidence another provider already has the same concept)

**3. Assign confidence using the defined predicates.**
- confirmed: Stated explicitly in source code (struct field, type annotation)
  OR by an unambiguous official documentation statement that directly names and
  describes the field or behavior
- inferred: Appears in examples or is implied by documentation that does not
  formally define it
- unknown: You believe the capability exists but no source material clearly
  confirms or denies it

When in doubt, prefer inferred over confirmed. confirmed must be traceable to
a specific passage you can cite.

**4. Preserve existing content unless contradicted.**
Do not remove or downgrade existing canonical_mappings or provider_extensions
entries unless new source material explicitly contradicts them. If ambiguous,
keep the entry and lower confidence if appropriate.

**5. Capture behavioral nuance in prose fields.**
The loading_model and notes fields are for prose detail: loading semantics,
scope inheritance rules, truncation behavior, edge cases. This is where
provider-specific context lives when it does not map to a structured field.

## Output

A valid YAML file at docs/provider-formats/<slug>.yaml conforming to the format
doc schema. Update last_fetched_at and content_hash on each changed source
entry. Set generation_method to subagent.

## Do not

- Invent canonical keys. If no canonical key exists for a capability, use
  provider_extensions. Never add to canonical_mappings a key that is not in
  docs/spec/canonical-keys.yaml.
- Set graduation_candidate: true without evidence that another provider has the
  same concept.
- Summarize source content. Full detail is required.
- Modify any file other than docs/provider-formats/<slug>.yaml.
```

---

### `docs/workflows/graduation-comparison.md` (target file)

```markdown
# Graduation Comparison

**Invoked by:** capmon-process (Step 3 of the local remediation loop, conditional)

**Purpose:** Given that a new provider_extensions entry was just added to a
format doc, check whether any other provider already has a semantically
equivalent extension. If two or more providers have the same concept under
different names, that concept is a graduation candidate.

## Inputs

- CHANGED_PROVIDER — slug of the provider whose format doc was just updated
- NEW_EXTENSIONS — the list of new provider_extensions entries added in this run
- All docs/provider-formats/*.yaml files

## Your job

For each extension in NEW_EXTENSIONS:

1. Read its id, name, and description.
2. Read the provider_extensions sections of all other providers' format docs.
3. Determine: does any other provider have an extension describing the same
   underlying concept? Same concept means the same provider behavior or
   capability, even if named completely differently.

   Example of a match: "Amp bundles an MCP server with a skill" and "Cline
   packages tools inside a skill directory" both describe a mechanism for
   co-locating server-side tooling with skill content. Different names, same
   concept.

   Example of a non-match: one provider has a caching behavior and another
   has a lazy-loading behavior. Superficially related but solving different
   problems — not a graduation candidate.

4. If you find a match across two or more providers: record the details.

## Output

For each graduation candidate found, produce one section in this format:

---
## Graduation Candidate: <suggested_canonical_key>

**Concept:** One sentence describing the capability.

**Providers:**
- `<slug>`: extension `<id>` — "<name>" — <source_ref>
- `<slug>`: extension `<id>` — "<name>" — <source_ref>

**Suggested canonical key:** `<snake_case_key>`
**Suggested definition:** One sentence suitable for canonical-keys.yaml.
**Suggested type:** string | bool | object

**Notes:** Any ambiguity, differences in how providers implement this, or
open questions the human reviewer should consider.
---

This output becomes the body of a capmon-graduation GitHub issue.

If no matches are found, produce no output. No issue is created.

## Do not

- Flag tenuous connections. Only flag clear semantic equivalents where two
  providers are clearly solving the same problem with different naming.
- Suggest graduation for concepts only one provider has.
- Modify any file. Your output is a report only.
- Create graduation candidates across different content types (a skills
  extension and a hooks extension cannot graduate to the same key).
```

---

## What Changes From Today

| Aspect | Before | After |
|--------|--------|-------|
| Source of truth | SeederSpec (pipeline artifact) | Format doc YAML (human+LLM artifact) |
| Format doc updates | Optional, no enforcement | Primary output — LLM populates directly |
| SeederSpec | Human-reviewed, LLM-generated | Auto-derived from format doc (no human review) |
| Content captured | Partial excerpts (extractor-limited) | Full page / full source file |
| Human gate | `human_action: approve` on SeederSpec | Merge the format doc PR |
| Change detection | Manual / on-demand | CI scheduled, hash-based, free |
| Provider extensions | Fell between the cracks | First-class field in format doc schema |
| Graduation | Ad hoc, no mechanism | Explicit detection → graduation issue → human PR |

---

## Files Affected / New

### New files

| Path | Purpose |
|------|---------|
| `docs/spec/canonical-keys.yaml` | Authoritative list of canonical key names with definitions |
| `docs/provider-formats/<slug>.yaml` | New YAML format for all provider format docs (replaces .md files) |
| `cli/internal/capmon/formatdoc.go` | Go types for format doc YAML schema |
| `cli/internal/capmon/derive.go` | Deterministic parser: format doc → SeederSpec |
| `cli/cmd/syllago/capmon_check_cmd.go` | `capmon check` command (hash comparison, issue creation) |
| `cli/cmd/syllago/capmon_derive_cmd.go` | `capmon derive` command |
| `cli/cmd/syllago/capmon_onboard_cmd.go` | `capmon onboard` command (immediate first-run for new providers) |
| `cli/cmd/syllago/capmon_validate_sources_cmd.go` | `capmon validate-sources` command (pre-flight check: every content type has ≥1 source URI) |
| `docs/workflows/update-format-doc.md` | Workflow doc for the format doc update agent |
| `docs/workflows/graduation-comparison.md` | Workflow doc for the graduation comparison agent |
| `.github/workflows/capmon-check.yml` | CI job for scheduled change detection |

### Modified files

| Path | Change |
|------|--------|
| `cli/internal/capmon/seederspec.go` | SeederSpec becomes a derived type, not a reviewed artifact |
| `docs/workflows/inspect-provider-skills.md` | Replaced by `update-format-doc.md` |
| `docs/provider-formats/*.md` | Migrated to `*.yaml` format over time |

### Deprecated

| Path | Reason |
|------|--------|
| `.develop/seeder-specs/` | SeederSpecs are now auto-derived, not manually reviewed artifacts |
| `docs/provider-formats/*.md.proposed-additions` | No longer needed — format doc agent writes directly |

---

## Open Questions (Low Stakes — Implementation Choices)

1. **Migration path for existing `.md` format docs** — migrate all 14 at once during implementation, or incrementally as each provider is touched? Incrementally is lower risk.

2. **Cache storage for CI** — CI fetches full content but the canonical cache lives locally. Does CI commit the updated hashes to a `capmon-cache` branch, or does it store only hashes (not full content) and the local loop re-fetches? Lean: CI stores only hashes in the issue; local loop re-fetches full content when it processes the issue.

3. **Loop polling target** — poll `gh issue list --label=capmon-change` directly, or use a dedicated `capmon pending` command that wraps it? Either works.
