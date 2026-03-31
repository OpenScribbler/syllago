# Provenance, Versioning, and Trigger Mechanisms for AI Content Specs

**Date:** 2026-03-30
**Context:** Research conducted to inform extensions to the Skill Frontmatter Specification being developed with the Agent Ecosystem community.
**Scope:** 20+ specifications, package managers, and content systems analyzed for provenance, versioning, derivation tracking, and trigger mechanisms.

---

## Motivation

A developer in the Agent Ecosystem community raised a practical problem: when you find someone's skill repository and cherry-pick a single skill, you lose the connection to the original. There's no way to:

1. **Track upstream updates** without manually revisiting the source
2. **Know if you've diverged** from the original after local edits
3. **Revert to a previous version** if an update breaks your workflow
4. **Identify the original author** when content passes through multiple hands
5. **Compare skills** by their activation conditions rather than reading full descriptions

These problems are not unique to skills. They apply to all shareable AI content (hooks, rules, agents, commands, MCP configs). This research explores how other ecosystems have solved -- or failed to solve -- these challenges.

---

## Part 1: Provenance and Derivation Tracking

### The Landscape

We analyzed how 15 systems handle "where did this come from?" and "is this derived from something else?"

| System | Fork/Derivation Tracking | Mechanism |
|--------|--------------------------|-----------|
| **SPDX** | `DESCENDANT_OF`, `VARIANT_OF`, `COPY_OF` relationships | Graph-based, pairwise relationships between elements |
| **CycloneDX** | `pedigree.ancestors` with full component objects, commits, patches | Richest model -- captures what changed and why |
| **Hugging Face** | `base_model` + `base_model_relation` (finetune/quantized/adapter/merge) | Single-level typed derivation, bidirectional with `new_version` |
| **SAP FMS** | `FORK.yaml` with upstream coordinates + sync status enum | Only system that tracks relationship trajectory |
| **npm** | None | Convention only (rename package, update `repository.url`) |
| **Cargo** | None | Convention only (publish under new name) |
| **Helm** | None | Annotations escape hatch (e.g., Rancher's `+upX.Y.Z` suffix) |
| **Git** | Commit parent DAG (implicit) | No repo-level fork metadata; GitHub adds `parent`/`source` as platform layer |
| **Nix** | Overlays (implicit in code) | Derivation DAG tracks build inputs, not content lineage |
| **OCI** | None (lower-level primitive) | Content-addressable identity via digests, but no lineage |
| **SLSA** | None (tracks build process) | Answers "was this built correctly?" not "where did this come from?" |
| **LangChain Hub** | None | `<handle>/<name>` namespace provides attribution, not derivation |
| **MLflow** | Run lineage (platform-only) | Lineage lost on export -- platform-dependent |
| **GPT Store** | None | Opaque, no export, no derivation metadata |

### Key Finding: Fork Lineage Is the Biggest Unsolved Problem

**No major package manager has structured fork/derivation tracking.** npm, Cargo, and Helm -- the three most widely used package managers -- all rely on naming conventions when someone forks a package. The link to the original is maintained only through git history and human memory.

Only three systems have *structured* derivation tracking:
- **SPDX** provides relationship vocabulary (DESCENDANT_OF, VARIANT_OF) but is focused on SBOM documents, not package metadata
- **CycloneDX** provides the richest pedigree model (ancestors, commits, patches) but is also SBOM-focused
- **Hugging Face** solves it for models with `base_model` + relationship type, and it's the closest domain analog to our use case

### Patterns Worth Adopting

#### 1. Nix's Two-Field Fetcher Model

Nix fetchers record `(source_locator, content_hash)` for every dependency:
- The **locator** (URL, repo, path, ref) tells you where to re-fetch
- The **hash** (SHA-256 of content) tells you what you got
- If the URL changes but the hash matches, it's the same content
- If the URL stays but the hash changes, it's different content

This cleanly separates "where from" (informational, for humans and re-fetching) from "what is it" (canonical, for identity and verification).

#### 2. Hugging Face's Typed Derivation

HF's `base_model_relation` field recognizes that "derived from" isn't enough -- the *type* of derivation matters:
- `finetune` = trained further on the base
- `quantized` = compressed version of the base
- `adapter` = lightweight modification (LoRA)
- `merge` = combined from multiple models

For AI content, equivalent relationships would be:
- `fork` = copied and potentially modified
- `convert` = transformed to another provider's format
- `extract` = pulled from a larger bundle
- `merge` = combined from multiple sources

#### 3. SPDX's Originator vs. Supplier

SPDX distinguishes `PackageOriginator` (who created it) from `PackageSupplier` (who distributed it to you). npm learned the same lesson: `author` (self-declared creator) is separate from `maintainers` (people with publish access).

When Bob publishes Alice's skill in his registry, users should know both who wrote it and who they got it from.

#### 4. SAP FMS's Sync Status

SAP's Fork Metadata Standard is the only system that tracks the *relationship trajectory* -- not just "where from" but "am I still following?" Their enum:
- `actively-synchronized` -- fork tracks upstream
- `one-time-fork` -- grabbed once, now independent
- `abandoned` -- was tracking, stopped

Note: This is a *consumer-side* concern, not intrinsic to the content. It belongs in local install state, not in the portable spec.

#### 5. Single-Level Derivation with Chain Traversal

HF uses single-level `base_model` -- each item points only to its immediate parent. If you want the full chain, follow the links. SPDX uses the same approach with pairwise relationships.

Full chain embedding (storing the entire derivation history in each artifact) was considered and rejected because:
- Chains go stale if any intermediate ancestor updates its metadata
- Storage grows linearly with chain depth
- The same information is recoverable by traversal

### Recommended Spec Addition: `derived_from` Block

```yaml
provenance:
  # ... existing fields ...
  content_hash: "sha256:abc123..."
  publisher: "registry-name-or-person"
  derived_from:
    source_repo: "https://github.com/original-author/skills"
    source_repo_subdirectory: "/skills/code-review"
    version: "1.0.0"
    content_hash: "sha256:def456..."
    relation: "fork"  # fork | convert | extract | merge
```

---

## Part 2: Content Integrity

### The Pattern: Content-Addressable Identity

Every system we studied that handles integrity uses cryptographic digests:

| System | Digest Use |
|--------|-----------|
| **OCI** | Manifest digest (SHA-256) is the canonical identifier; tags are mutable convenience |
| **SLSA** | Subject digests in attestation statements; matching is purely by digest |
| **Nix** | Store paths computed from derivation inputs or output content |
| **npm** | Tarball SHA-512 in provenance statements |
| **Cargo** | Checksum in registry index |
| **Helm** | SHA-256 in `.prov` files |

### Key Insight: Dual Identity

OCI established the definitive pattern: **mutable names + immutable digests**. A tag (`v1.0`) is a human-friendly pointer that can be updated. A digest (`sha256:abc123`) is the truth that never changes.

Model providers use the same pattern: `claude-sonnet-4-5` (alias, auto-upgrades) vs `claude-sonnet-4-5-20250929` (snapshot, immutable).

### What's Missing in the Current Spec

The existing Skill Frontmatter Spec has `version` (semver) but **no content hash**. This means:
- You trust `source_repo` implicitly -- if the repo is force-pushed, you won't know
- You can't verify "what I have matches what the author published"
- Drift detection (did I modify this locally?) is impossible without a baseline hash

### Recommended Spec Addition: `content_hash`

A `content_hash` field in `provenance` using the format `algorithm:hex_value` (e.g., `sha256:abc123...`). The hash should cover the skill's content files, computed deterministically (sorted file list, normalized line endings).

---

## Part 3: Trigger Mechanisms

### The Problem

A 650-trial study found that passive skill descriptions ("Docker expert for containerization") achieve only ~77% activation, while directive descriptions ("ALWAYS invoke this skill when...") hit 100%. An entire cottage industry of workaround hooks exists to compensate for unreliable activation.

The root cause: asking the LLM to parse trigger intent from natural language description is probabilistic, not deterministic. The description competes with conversation history, project context, and other skills for the model's attention.

### How Other Systems Handle Triggers

| System | Trigger Mechanism | Deterministic? |
|--------|-------------------|----------------|
| **Claude Code** | `description` field only | No -- LLM judgment |
| **Cursor** | 4 modes: always / intelligent / file globs / manual | Partially -- globs are deterministic |
| **GitHub Copilot** | `applyTo` globs + agent routing + manual `/` | Partially |
| **Gemini CLI** | Manual `/command` only | Yes -- but no auto-activation |
| **VS Code extensions** | `activationEvents` (onLanguage, onCommand, workspaceContains) | Yes -- fully deterministic |
| **GitHub Actions** | `on:` block (push, PR, schedule, paths) | Yes -- fully deterministic |
| **Dialogflow** | Training phrases + events + contexts | ML-classified from examples |

### Key Finding: Every Established System Separates "What" from "When"

- VS Code: `description` (what) vs `activationEvents` (when)
- GitHub Actions: `name` (what) vs `on:` (when)
- Cursor: rule content (what) vs activation mode/globs (when)
- Dialogflow: fulfillment (what) vs intents/training phrases (when)

Claude Code's SKILL.md and the agentskills.io spec are outliers in combining both into a single `description` field.

### The Case for Structured Triggers in the Spec

1. **Reliability** -- Deterministic triggers (file patterns, workspace detection) work 100% of the time. Description-based triggers work 50-77% of the time.

2. **Cross-provider portability** -- Each provider maps structured triggers to their native mechanism:
   - `file_patterns` maps to Claude Code `paths`, Cursor `globs`, Copilot `applyTo`
   - `commands` maps to Claude slash commands, Gemini `/command`, Copilot `/prompt`
   - `mode: always` maps to Cursor `alwaysApply`, Copilot auto-attached instructions

3. **Machine comparability** -- When browsing or searching skills, structured triggers let tools compare activation conditions programmatically. "This skill triggers on test files" vs "this skill triggers on PR events" is instantly clearer than parsing two paragraphs of description.

4. **Author intent** -- A `mode` field explicitly communicates whether the author intended auto-activation, manual-only, or always-on. Without this, consumers must guess from the description.

### Recommended Spec Addition: `triggers` Section

```yaml
triggers:
  mode: auto                    # auto | manual | always
  file_patterns:                # Activate when working with matching files
    - "**/*.test.ts"
    - "**/*.spec.ts"
  workspace_contains:           # Activate when project contains these files
    - "jest.config.*"
  commands:                     # Explicit slash command names
    - "/test-review"
  keywords:                     # Structured hints for LLM routing
    - "review tests"
    - "check test coverage"
```

All fields optional but strongly encouraged. Skills without triggers fall back to description-only activation (preserving backward compatibility).

---

## Part 4: What Belongs in the Spec vs. What's Tool-Specific

A clear principle emerged from the research: **if it describes what the content IS, it belongs in the spec. If it describes a consumer's RELATIONSHIP to the content, it's tool-specific.**

### In the Spec (Portable, Travels with Content)

| Feature | Rationale |
|---------|-----------|
| Provenance (author, source, derivation) | Intrinsic to the content |
| Content integrity (hash) | Intrinsic, enables verification anywhere |
| Triggers (activation conditions) | Author's intent for how the skill should be used |
| Expectations (runtime dependencies) | Already in the spec -- software, services, OS requirements |
| Version (semver) | Already in the spec |
| License | Already in the spec |

### Tool-Specific (Local State, Not in Spec)

| Feature | Rationale |
|---------|-----------|
| Sync strategy (track/pin/detach) | Consumer's choice about upstream tracking |
| Last-checked timestamp | Operational state |
| Installed version history | Local install log |
| Drift detection (local vs. imported hash) | Computed from spec fields |
| Trust tiers | Security model varies by tool |
| Registry-specific metadata (downloads, stars) | Platform data, not content property |

This mirrors the split every system makes:
- **Git:** commit objects (intrinsic) vs refs/remotes (relationship)
- **npm:** package.json (intrinsic) vs registry metadata (platform)
- **Nix:** derivation (intrinsic) vs store path provenance (local)

---

## Sources

### Package Managers
- npm: [package.json docs](https://docs.npmjs.com/cli/v11/configuring-npm/package-json/), [provenance statements](https://docs.npmjs.com/generating-provenance-statements/)
- Cargo: [manifest format](https://doc.rust-lang.org/cargo/reference/manifest.html), [publishing](https://doc.rust-lang.org/cargo/reference/publishing.html)
- Helm: [charts docs](https://helm.sh/docs/topics/charts/), [provenance](https://helm.sh/docs/topics/provenance/)

### Container & Supply Chain
- OCI: [image manifest spec](https://github.com/opencontainers/image-spec/blob/main/manifest.md), [referrers API](https://oras.land/docs/concepts/reftypes/)
- SLSA: [v1.0 spec](https://slsa.dev/spec/v1.0/), [provenance predicate](https://slsa.dev/provenance/v1)
- in-toto: [attestation framework](https://github.com/in-toto/attestation)

### SBOM Standards
- SPDX: [package information](https://spdx.github.io/spdx-spec/v2.3/package-information/), [relationship types](https://spdx.github.io/spdx-spec/v3.0.1/model/Core/Vocabularies/RelationshipType/)
- CycloneDX: [pedigree](https://cyclonedx.org/use-cases/pedigree/), [SBOM guide](https://cyclonedx.org/guides/sbom/pedigree/)
- Package URL: [purl spec](https://github.com/package-url/purl-spec)

### Version Control & Content-Addressed Systems
- Git: [replace docs](https://git-scm.com/docs/git-replace), [notes](https://tylercipriani.com/blog/2022/11/19/git-notes-gits-coolest-most-unloved-feature/)
- Nix: [derivations](https://nix.dev/manual/nix/2.34/store/derivation/), [CA paths RFC](https://github.com/NixOS/rfcs/blob/master/rfcs/0062-content-addressed-paths.md), [store path provenance PR](https://github.com/NixOS/nix/pull/11749)
- Homebrew: [formula cookbook](https://docs.brew.sh/Formula-Cookbook)
- SAP Fork Metadata Standard: [FORK.yaml](https://github.com/SAP/fork-metadata-standard)

### AI/ML Ecosystem
- Hugging Face: [model cards](https://huggingface.co/docs/hub/en/model-cards)
- MLflow: [model registry](https://mlflow.org/docs/latest/ml/model-registry/)
- W&B: [artifact lineage](https://docs.wandb.ai/guides/artifacts/explore-and-traverse-an-artifact-graph/)
- LangChain Hub: [prompt management](https://docs.langchain.com/langsmith/manage-prompts)

### Trigger Mechanisms
- VS Code: [activation events](https://code.visualstudio.com/api/references/activation-events)
- Claude Code: [skills docs](https://code.claude.com/docs/en/skills)
- Cursor: [rules docs](https://cursor.com/docs/rules)
- Copilot: [custom instructions](https://docs.github.com/copilot/customizing-copilot/adding-custom-instructions-for-github-copilot)
- Agent Skills spec: [agentskills.io](https://agentskills.io/specification)
- Skill activation study: [650 Trials (Medium)](https://medium.com/@ivan.seleznov1/why-claude-code-skills-dont-activate-and-how-to-fix-it-86f679409af1)
