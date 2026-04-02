# Agent Skills Metadata Convention

**Version:** 0.2.0 (Draft)
**Status:** Draft — companion convention to the Agent Skills Specification

---

## Abstract

This document defines a vocabulary of structured metadata fields for
AI coding agent skills. These fields — provenance, triggers,
expectations, agent compatibility, attestation, and trust — live in a
sidecar file alongside the skill's SKILL.md. The convention extends
the Agent Skills Specification without modifying it, adding one
optional frontmatter field that points to the sidecar. A SKILL.md
using this convention remains fully valid under the Agent Skills spec.

---

## 1. Introduction

AI coding agent skills need metadata beyond what the Agent Skills
Specification defines. Distribution tools need provenance and
integrity hashes. Registries need version numbers and attestation
records. Agents need structured triggers for deterministic activation.

The Agent Skills spec deliberately keeps SKILL.md frontmatter minimal
to avoid inflating model context. This convention preserves that
design by placing structured metadata in a sidecar file that the
model never sees.

### 1.1. Scope

This convention defines:

-   A sidecar file format (`SKILL.meta.yaml`) for structured metadata
-   A single frontmatter field (`metadata_file`) linking SKILL.md to
    the sidecar
-   Nine categories of convention fields: provenance, expectations,
    triggers, tags, status, durability, supported agents, attestation,
    and trust states

This convention does not define:

-   The SKILL.md format itself (defined by the Agent Skills spec)
-   How agents load or execute skills
-   Registry APIs or discovery protocols

### 1.2. Audience

This document is intended for:

-   **Skill authors** who want to annotate their skills with
    structured metadata
-   **Distribution tool developers** who build package managers,
    registries, or import/export tools for skills
-   **Agent developers** who want to consume structured triggers,
    expectations, or provenance data

---

## 2. Relationship to the Agent Skills Specification

This document is a **companion convention** to the Agent Skills
Specification [AGENTSKILLS]. It does not replace, fork, or override
that specification.

### 2.1. Dependency Direction

This convention depends on the Agent Skills spec. The Agent Skills
spec does not depend on or reference this convention. Changes to the
Agent Skills spec may require corresponding changes to this
convention.

### 2.2. Version Compatibility

This convention is designed for the Agent Skills Specification as
published at agentskills.io as of March 2026. Compatibility with
future major revisions of the Agent Skills spec is not guaranteed.

### 2.3. Conformance Relationship

A SKILL.md using this convention MUST also be valid under the Agent
Skills spec. This convention is additive — it does not restrict or
subset the parent spec.

### 2.4. What This Convention Adds

The Agent Skills spec defines the SKILL.md frontmatter format:
`name`, `description`, `license`, `compatibility`, `metadata` (a
string-to-string map), and `allowed-tools`.

This convention adds:

-   One optional top-level frontmatter field: `metadata_file`
-   A sidecar file (`SKILL.meta.yaml`) containing structured metadata
    that exceeds the string-to-string constraint of the parent spec's
    `metadata` field

### 2.5. Why a Sidecar File

The Agent Skills spec's `metadata` field is defined as a map from
string keys to string values. This convention requires structured data
(nested objects, arrays) that exceeds that definition.

Placing convention fields directly in SKILL.md frontmatter would add
approximately 30 lines of YAML to every skill. Multiple agents
(including Codex CLI and GitHub Copilot) do not strip frontmatter
before injecting skill content into the model's context window. On
those agents, 30 extra lines would be injected on every skill
activation — contradicting the Agent Skills spec's deliberately
minimal frontmatter design.

The sidecar pattern resolves both problems: the SKILL.md frontmatter
stays minimal (3–4 lines), and structured data lives in a file that
tooling reads but the model never sees.

---

## 3. Terminology

The following terms have specific meanings in this convention:

-   **agent** — A software product that uses large language models to
    help users write, review, or maintain code. An agent is what a user
    installs, launches, and interacts with. Examples: Claude Code,
    Cursor, GitHub Copilot, Gemini CLI.

-   **model provider** — A company or service providing the underlying
    language model that powers an agent. Examples: Anthropic (Claude),
    OpenAI (GPT), Google (Gemini). One agent may support multiple model
    providers.

-   **form factor** — How an agent is delivered to users. Values: IDE,
    CLI, extension, web, autonomous, hybrid.

-   **skill** — A SKILL.md file conforming to the Agent Skills
    Specification, optionally accompanied by supporting files in a
    skill directory.

-   **sidecar file** — The SKILL.meta.yaml file containing this
    convention's structured metadata, referenced by the SKILL.md's
    `metadata_file` field.

-   **registry** — A service or repository that indexes, stores, and
    distributes skills.

-   **distribution tool** — Software that imports, exports, installs,
    or manages skills on behalf of users. Examples: package managers,
    CLI tools, IDE integrations.

---

## 4. Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
"SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and
"OPTIONAL" in this document are to be interpreted as described in
BCP 14 [RFC2119] [RFC8174] when, and only when, they appear in all
capitals, as shown here.

---

## 5. Conformance

This convention defines three conformance classes:

**Conforming skill author.** A skill author conforms to this
convention if the SKILL.md frontmatter contains a valid
`metadata_file` field and the referenced sidecar file satisfies all
MUST requirements in Sections 6 through 14.

**Conforming registry.** A registry conforms to this convention if it
satisfies all MUST requirements in Sections 6, 7 (provenance
validation), 14 (attestation storage), and 15 (trust state
reporting).

**Conforming agent.** An agent conforms to this convention if it
satisfies all MUST requirements in Section 6 (sidecar discovery and
unknown-key handling). Support for individual convention fields
(triggers, expectations, supported agents, etc.) is OPTIONAL — agents
MAY implement any subset.

### 5.1. Stability Levels

Each field in this convention has an assigned stability level:

| Level | Meaning |
|-------|---------|
| **Stable** | Community consensus, multiple implementations. Semantics will not change in minor versions of this convention. |
| **Draft** | Being experimented with. May change or be removed based on implementation experience. |

Until this convention reaches version 1.0.0, all fields remain
subject to change regardless of their stability designation. Stability
levels indicate intended maturity and forward-compatibility intent,
not guarantees.

**Promotion criteria.** A field advances from Draft to Stable when it
has been implemented by at least two independent agents or tools, no
breaking changes have been required for at least two convention
releases, and a community review period of at least 30 days has been
completed.

**Backward-compatibility.** After this convention reaches 1.0.0:
Stable fields will not have their semantics changed in minor versions;
removal of a Stable field will require a major version bump with a
12-month deprecation notice. Draft fields may change in minor
versions; removal requires release notes but no deprecation period.

---

## 6. Sidecar File Format

The metadata sidecar is a YAML file containing all convention fields.
The RECOMMENDED filename is `SKILL.meta.yaml`, placed alongside the
SKILL.md in the skill directory:

```
skill-name/
├── SKILL.md            # name, description, metadata_file pointer
├── SKILL.meta.yaml     # all convention metadata
├── scripts/            # optional
└── references/         # optional
```

### 6.1. The `metadata_file` Field

This convention introduces `metadata_file` as a top-level SKILL.md
frontmatter field, alongside the Agent Skills spec's existing fields
(`name`, `description`, `license`, `compatibility`, `metadata`,
`allowed-tools`).

This is the convention's single addition to the SKILL.md frontmatter
schema. It is proposed for inclusion in the Agent Skills spec as a
top-level field. Until adopted, agents SHOULD ignore unknown top-level
fields without error, per standard forward-compatibility practice.

`metadata_file` and convention-defined keys inside `metadata:` are
mutually exclusive. A SKILL.md MUST NOT contain both `metadata_file`
and convention-defined keys inside `metadata:`.

### 6.2. Schema

The sidecar file is flat YAML — no wrapping `metadata:` key. The
entire file IS metadata. Top-level keys are `metadata_spec`,
`provenance`, `triggers`, `expectations`, `tags`, `status`,
`durability`, and `supported_agents`.

**Required fields:** `metadata_spec` (identifies the convention
version).

**Optional fields:** All other convention fields defined in this
document (Sections 7–15).

Agents and tools implementing this convention MUST ignore unknown keys
in the sidecar file without error. This ensures forward compatibility
when newer convention versions add fields.

### 6.3. Discovery Rules

1.  If `metadata_file` is present in the SKILL.md frontmatter,
    tooling MUST resolve it as a relative path from the SKILL.md
    file's directory.

2.  If `metadata_file` is absent, tooling MUST NOT search for sidecar
    files by naming convention. Absent field means no convention
    metadata exists.

3.  If `metadata_file` is present but the referenced file does not
    exist, tooling SHOULD warn the author (broken reference). The
    SKILL.md remains valid and functional — the skill has no
    convention metadata.

### 6.4. Content Hash Interaction

When computing `content_hash` (Section 7.5), the sidecar file is
included in the hash computation — it is a regular file in the skill
directory. The `content_hash` blanking step applies only to the file
containing the hash value itself (typically the sidecar file).

### 6.5. Example

**SKILL.md** (3 lines of frontmatter — the model sees only this):

``` yaml
---
name: code-review
description: Reviews code for quality, security, and style issues.
license: MIT
metadata_file: ./SKILL.meta.yaml
---

## Instructions

Review the code for...
```

**SKILL.meta.yaml** (tooling reads this — the model never sees it):

``` yaml
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.2"
provenance:
  version: "1.2.0"
  source_repo: "https://github.com/acme/skills"
  source_repo_subdirectory: "/skills/code-review"
  authors:
    - "Alice Smith <alice@example.com>"
triggers:
  mode: auto
  keywords:
    - "code review"
    - "review this PR"
```

> **Note (informative):** Convention fields live in the sidecar file,
> which the model never sees. The SKILL.md frontmatter contains only
> `name`, `description`, `license`, and optionally `metadata_file` —
> at most 4 lines. Agents that do not strip frontmatter (such as Codex
> CLI and GitHub Copilot) see at most one extra line (`metadata_file`)
> instead of 30+ lines of convention metadata.

---

## 7. Provenance

*Stability: Stable*

Provenance fields describe where the skill came from, who created it,
and how to verify its integrity.

### 7.1. provenance.version

The version of this specific skill. Value is a Semantic Versioning
[SEMVER] string. MUST NOT start with `v`.

Registries MAY reject skill submissions that omit `version`, as
versioning is essential for update detection, deduplication, and
dependency resolution.

**Immutability:** Registries MUST reject submissions where `version`
matches an existing entry but `content_hash` differs. Published
versions are immutable — to publish different content, bump the
version.

``` yaml
provenance:
  version: "4.2.0"
```

### 7.2. provenance.source\_repo

A URL indicating where the canonical version of this skill can be
found. MUST be an HTTPS URL. The URL is forge-agnostic — GitHub,
GitLab, Codeberg, self-hosted Gitea, or any git hosting service.

REQUIRED for content submitted to registries. Registries MUST reject
submissions without `source_repo` and MUST verify the URL resolves at
submission time. OPTIONAL for local-only content.

``` yaml
provenance:
  source_repo: "https://github.com/my_username/cool_skill_repo"
```

> **Note (informative):** This field is a self-asserted claim by the
> skill author unless independently verified through attestation (see
> Section 14). Distribution tools and registries SHOULD NOT treat it
> as verified provenance without attestation or equivalent
> verification.

### 7.3. provenance.source\_commit

The full git commit SHA at which the content was published. MUST be
exactly 40 lowercase hexadecimal characters.

`source_commit` provides an immutable reference point. Unlike branch
names or tags, commit SHAs cannot be rewritten after the fact.

`source_commit` SHOULD be captured automatically by tooling, not
manually entered by authors. Registries SHOULD capture the commit SHA
at submission time. Distribution tools SHOULD capture the current HEAD
SHA during publishing workflows.

``` yaml
provenance:
  source_commit: "abc123def456789012345678901234567890abcd"
```

### 7.4. provenance.source\_repo\_subdirectory

Path within the repository when the repo contains more than one skill.
Relative to the repository root. MUST start with `/`. When there is
only one skill, it is assumed to be `/`.

``` yaml
provenance:
  source_repo_subdirectory: "/skills/tool_x/integration"
```

### 7.5. provenance.content\_hash

A cryptographic hash of the skill's content files, using the format
`algorithm:hex_value`. The algorithm MUST be `sha256`. The hex value
MUST be 64 lowercase hexadecimal characters.

Example: `sha256:a1b2c3d4e5f67890...` (64 hex characters total)

The hash enables integrity verification and drift detection. To
compute the hash deterministically:

1.  Enumerate all files in the skill directory recursively. Exclude
    any file or directory whose name starts with `.` (hidden files
    and hidden directories, including all their contents — e.g.,
    `.git/`, `.DS_Store`). The file containing the YAML frontmatter
    (typically `SKILL.md`) IS included after the blanking step below.

2.  **Blank the `content_hash` value.** In the file containing the
    hash (typically the sidecar file), perform a byte-level
    substitution: find the first occurrence of the pattern
    `content_hash: "sha256:<64 hex chars>"` and replace the
    71-character value string (`"sha256:<64 hex chars>"`) with `""`
    (two ASCII double-quote characters with nothing between them).
    The resulting line MUST be `content_hash: ""`. This is a raw byte
    operation — do NOT re-serialize YAML, as serializers produce
    inconsistent whitespace and quoting. If `content_hash` is absent
    or already `""`, no substitution is needed.

3.  **Normalize line endings.** For each file, replace all `\r\n`
    (CRLF) sequences with `\n` (LF) before hashing. This ensures
    identical hashes regardless of platform checkout settings (e.g.,
    git `core.autocrlf`). Bare `\r` characters are left unchanged.

4.  Construct relative paths from the skill directory root using
    forward slash (`/`) as separator on all platforms. Paths MUST NOT
    start with `/` or `./`. Example: `scripts/extract.py`, `SKILL.md`.
    Relative paths MUST be encoded as UTF-8 bytes.

5.  Sort paths lexicographically by UTF-8 byte value.

6.  For each file, compute SHA-256 of:
    `relative_path_bytes + '\0' + normalized_file_bytes`.

7.  Concatenate all per-file hashes (raw 32-byte digests, not hex).

8.  Compute a final SHA-256 over the concatenated result.

Empty directories and symlinks are excluded. Only regular files are
hashed.

### 7.6. provenance.authors

A list of strings identifying the skill's creators. Uses the git
author format: `Name <email>`. The email portion is optional.

``` yaml
provenance:
  authors:
    - "Mary Smith <mary@example.com>"
    - "Andrea Barley"
```

### 7.7. provenance.publisher

The entity that distributed this skill to the consumer. Distinct from
`authors` — the publisher may not have written the skill. This field
is typically set by a distribution tool or registry, not by the skill
author. It is a plain string identifier.

> **Note (informative):** Like `source_repo`, this is a self-asserted
> claim. Any skill can claim any publisher. Distribution tools SHOULD
> NOT make authorization or trust decisions based solely on this field
> without attestation (Section 14) or equivalent verification.

### 7.8. provenance.license\_spdx\_id

An SPDX license identifier [SPDX]. MUST be a valid SPDX expression.
Examples: `MIT`, `Apache-2.0`, `GPL-3.0-only`.

If the skill has no license, use `UNLICENSED`. If the license is
unknown, use `NOASSERTION`.

This complements the Agent Skills spec's existing `license` field
(which is freeform) with a machine-parseable identifier. When both are
present, `license_spdx_id` is the authoritative value for machine
processing. Authors SHOULD keep both fields consistent.

### 7.9. provenance.license\_url

The URL to the full license text including author and copyright
attribution.

When a skill is extracted from a multi-skill repository, the
`license_url` SHOULD point to the license that applies to this
specific skill, which may differ from the repository's root license.

### 7.10. provenance.derived\_from

An array of objects indicating that this skill was derived from one or
more other skills. Each entry contains the upstream coordinates and
the type of derivation. Most derivations have a single parent (array
of one). Merges have multiple parents.

Each skill points only to its immediate parents — follow the links to
trace the full chain (each upstream has its own `derived_from` field).

Each entry in the array contains:

-   `source_repo` — REQUIRED. Upstream repository URL.
-   `source_repo_subdirectory` — Path within the upstream repo.
-   `version` — Upstream version at the time of derivation.
-   `content_hash` — Upstream content hash at the time of derivation.
-   `relation` — REQUIRED. The type of derivation. Known values:
    `fork`, `convert`, `extract`, `merge`. Agents MUST accept unknown
    relation values without error (the vocabulary is extensible).

| Relation  | Meaning |
|-----------|---------|
| `fork`    | Copied and potentially modified. |
| `convert` | Transformed to another agent's format. |
| `extract` | Pulled from a larger bundle. |
| `merge`   | Combined from multiple sources. |

> **Note (informative):** All fields within `derived_from` entries are
> self-asserted by the skill author. A skill can claim derivation from
> any upstream without proof. Enterprise tooling SHOULD treat
> `derived_from` as informational metadata, not a verified supply
> chain attestation, until attestation (Section 14) or signing (future
> version) provides independent linkage.

### 7.11. provenance.script\_hashes

*Stability: Draft*

A map of relative file paths to cryptographic hashes for executable
files within the skill directory. Each key is a relative path (same
format as `content_hash` file enumeration) and each value uses the
`algorithm:hex_value` format. The algorithm MUST be `sha256`.

Script hashes enable distribution tools to verify script integrity on
install, agents with trust gating to compare hashes before execution,
and drift detection when scripts change after publication.

``` yaml
provenance:
  script_hashes:
    "scripts/deploy.sh": "sha256:def456..."
    "scripts/validate.py": "sha256:789abc..."
```

### 7.12. Full Provenance Example

``` yaml
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.2"
provenance:
  version: "4.2.0"
  source_repo: "https://github.com/my_username/cool_skill_repo"
  source_repo_subdirectory: "/skills/tool_x/integration"
  content_hash: "sha256:a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890"
  authors:
    - "Mary Smith <mary@example.com>"
  publisher: "acme-skills-registry"
  license_spdx_id: "MIT"
  license_url: "https://github.com/my_username/cool_skill_repo/blob/main/LICENSE"
  script_hashes:
    "scripts/setup.sh": "sha256:a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890"
  derived_from:
    - source_repo: "https://github.com/original_author/skills"
      source_repo_subdirectory: "/skills/tool_x/integration"
      version: "2.0.0"
      content_hash: "sha256:f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5"
      relation: "fork"
```

---

## 8. metadata\_spec

*Stability: Stable*

A URI identifying the version of this metadata convention. This value
is an opaque identifier, not a locator — agents MUST NOT attempt to
dereference it. Agents MUST use exact string comparison to match the
`metadata_spec` value against known convention versions.

Agents that recognize the URI MAY parse the structured metadata fields
defined in this document. Agents that do not recognize it MUST ignore
all convention fields without error. Agents MUST NOT interpret
convention-defined fields (provenance, triggers, expectations,
supported\_agents) unless the `metadata_spec` key is present and
matches a recognized value.

``` yaml
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.2"
```

---

## 9. Expectations

*Stability: Draft*

Runtime dependencies that MUST be present for the skill to function.
This complements the Agent Skills spec's freeform `compatibility`
field with machine-parseable dependency declarations.

Version constraints use semver range syntax:

| Operator | Meaning | Example |
|----------|---------|---------|
| `>=`     | Greater than or equal | `>=1.2.0` |
| `<=`     | Less than or equal | `<=2.0.0` |
| `>`      | Greater than | `>1.0.0` |
| `<`      | Less than | `<3.0.0` |
| `==`     | Exactly equal | `==1.2.3` |
| `!=`     | Not equal | `!=1.0.0` |
| `~>`     | Compatible (pessimistic) | `~>3.4` |

**`~>` (pessimistic constraint) algorithm:** Given `~>X.Y.Z`, the
constraint expands to `>=X.Y.Z and <X.(Y+1).0`. Given `~>X.Y` (two
components), it expands to `>=X.Y.0 and <(X+1).0.0`. The operand MUST
have exactly 2 or 3 numeric components separated by `.` —
single-component (`~>1`) and 4+-component (`~>1.2.3.4`) values are
invalid. This operator follows RubyGems semantics.

| Input | Expands to |
|-------|-----------|
| `~>3.4` | `>=3.4.0 and <4.0.0` |
| `~>1.2.3` | `>=1.2.3 and <1.3.0` |
| `~>0.9` | `>=0.9.0 and <1.0.0` |

Multiple constraints may be joined with `and`: `>=1.2.0 and <2.0.0`.

### 9.1. expectations.software

An array of objects. CLI tools or applications expected to be present.
Multiple entries use AND logic (all are required).

Each entry MUST include `name` (the canonical upstream binary name).
Each entry MAY include `version` (a version constraint string). If
`version` is absent, any version is acceptable.

``` yaml
expectations:
  software:
    - name: "bat"
      version: ">=0.22"
    - name: "ripgrep"
      version: ">=14.1.0"
```

### 9.2. expectations.services

An array of objects. Remote services the user needs an account on.

Each entry MUST include `name`. Each entry MAY include `url` (the
service homepage).

``` yaml
expectations:
  services:
    - name: "Notion"
      url: "https://notion.so"
```

### 9.3. expectations.runtimes

An array of objects. Programming language runtimes required.

Each entry MUST include `name` (the canonical runtime command name,
e.g., `node`, `python`, `ruby`). Each entry MAY include `version`. If
`version` is absent, any version is acceptable.

``` yaml
expectations:
  runtimes:
    - name: "ruby"
      version: "~>3.4"
    - name: "node"
      version: ">=18"
```

### 9.4. expectations.operating\_systems

An array of objects. Operating systems the skill is known to work on.
Multiple entries use OR logic (any one is sufficient). If absent, the
skill is presumed to work on any major OS.

Each entry MUST include `name`. Canonical values are `linux`, `macos`,
and `windows`. Each entry MAY include `version`.

``` yaml
expectations:
  operating_systems:
    - name: "macos"
      version: "~>15.7"
    - name: "linux"
```

---

## 10. Triggers

*Stability: Draft*

Structured activation conditions. Triggers tell an agent when to
activate the skill, separately from `description` which explains what
the skill does.

Multiple triggers within a skill use OR logic — any matching trigger
activates the skill.

> **Rationale (informative):** Description-only activation achieves
> 50–77% reliability based on analysis of agent behavior.
> Deterministic triggers (`activation_file_globs`,
> `activation_workspace_globs`, `commands`) are evaluated mechanically
> by the agent's runtime with higher reliability. Probabilistic
> triggers (`keywords`) provide structured hints for LLM routing.

### 10.1. triggers.mode

The author's intended activation behavior. If absent, agents MUST
treat the skill as if `mode: auto` was specified.

| Value    | Meaning |
|----------|---------|
| `auto`   | Agent decides based on triggers and/or description. Default. |
| `manual` | Only activated by explicit user invocation. |
| `always` | Always loaded into the agent's context. |

**Precedence:** Mode is evaluated first. If mode resolves to a
definitive state (`always` → active, `manual` → inactive unless
commanded), trigger evaluation is skipped entirely.

| Mode | `activation_file_globs` | `activation_workspace_globs` | `commands` | `keywords` |
|------|------------------------|------------------------------|-----------|------------|
| `always` | Ignored | Ignored | Ignored | Ignored |
| `auto` | Evaluated | Evaluated | Evaluated | Evaluated |
| `manual` | Ignored | Ignored | Evaluated | Ignored |

### 10.2. triggers.activation\_file\_globs

An array of glob patterns. The skill SHOULD activate when the user is
working with files matching any pattern. Paths are relative to
workspace root.

Patterns use a portable glob subset:

| Operator | Meaning | Example |
|----------|---------|---------|
| `*`      | Match any characters within a path segment | `*.ts` |
| `**`     | Match zero or more path segments (recursive) | `**/*.test.ts` |
| `?`      | Match exactly one character | `test?.ts` |
| `[...]`  | Match one character from the set | `[Mm]akefile` |

Brace expansion (`{a,b}`) and extglobs (`+(pattern)`) are NOT
portable across glob engines and MUST NOT be used in convention
trigger patterns. Agents MAY support them as extensions but MUST NOT
require them.

``` yaml
triggers:
  activation_file_globs:
    - "**/*.test.ts"
    - "**/*.spec.ts"
```

### 10.3. triggers.activation\_workspace\_globs

An array of glob patterns. The skill SHOULD activate when the project
contains files matching any pattern. Evaluated once at project load,
not on every file change.

``` yaml
triggers:
  activation_workspace_globs:
    - "docker-compose.yml"
    - "Dockerfile"
```

### 10.4. triggers.commands

An array of command name strings that explicitly invoke this skill.
The value SHOULD be the bare command name without prefix (e.g.,
`test-review` not `/test-review`), and agents SHOULD apply their own
prefix convention.

When multiple skills register the same command name, agents are
responsible for disambiguation (e.g., prompting the user, using skill
name as a namespace). Registries SHOULD flag command name collisions
across indexed skills.

``` yaml
triggers:
  commands:
    - "test-review"
    - "check-coverage"
```

### 10.5. triggers.activation

*Stability: Draft*

A hint for how agents should handle re-activation of this skill within
a session. If absent, agents SHOULD treat the skill as `single`.

| Value        | Meaning |
|--------------|---------|
| `single`     | Activate at most once per session. Agents SHOULD deduplicate. Default. |
| `repeatable` | May be meaningfully re-activated (e.g., a skill producing different output based on conversation state). |

``` yaml
triggers:
  mode: auto
  activation: single
```

### 10.6. triggers.keywords

An array of phrases providing structured hints to the agent's LLM for
routing. Whether matching is exact, substring, or semantic is
agent-specific. The convention provides the vocabulary; the agent
decides how to use it.

``` yaml
triggers:
  keywords:
    - "review tests"
    - "check test coverage"
    - "test quality"
```

---

## 11. Tags

*Stability: Draft*

An array of freeform strings for categorization and discovery.
Registries and search tools MAY use tags for faceted search and
filtering. There is no controlled vocabulary — authors choose tags
that describe the skill's domain and purpose.

Tags SHOULD be lowercase. Avoid redundancy with the skill's `name` or
`description`.

``` yaml
tags:
  - "testing"
  - "code-quality"
  - "typescript"
```

---

## 12. Status

*Stability: Draft*

The lifecycle status of the skill. If absent, the skill is presumed to
be `active`.

| Value        | Meaning |
|--------------|---------|
| `active`     | Maintained and recommended for use. Default. |
| `deprecated` | No longer recommended. Still works but a better alternative may exist. |
| `archived`   | No longer maintained. May not work with current agents. |

``` yaml
status: "deprecated"
```

---

## 13. Durability

*Stability: Draft*

The author's intended behavior for context compaction. If absent, the
skill is presumed to be `persistent`.

| Value        | Meaning |
|--------------|---------|
| `persistent` | Instructions must remain in context for the full session. Agents with compaction SHOULD protect this skill's content from pruning. Default. |
| `ephemeral`  | Instructions are useful for the current task only and can be safely summarized or dropped during compaction. |

``` yaml
durability: persistent
```

> **Rationale (informative):** Context compaction protection varies
> across agents. Some agents protect skill content during compaction;
> others use generic compaction with no skill awareness. `durability`
> signals author intent to agents that support compaction protection
> and gives distribution tools a basis for compatibility warnings.

---

## 14. Supported Agents

*Stability: Draft*

An array of agents this skill is known to work with. If absent, the
skill is presumed to be generic and should work with any agent.

This complements the Agent Skills spec's freeform `compatibility`
field with structured agent declarations.

Each entry MUST include a `name`. Agent names MUST use the canonical
identifier from Appendix C (e.g., `claude-code`, `cursor`,
`gemini-cli`). Agents not yet listed in Appendix C SHOULD use a
lowercase hyphenated form of the product name. New identifiers will be
added in convention patches.

Each entry MAY include an `integrations` array listing platform
integrations required for the skill. Each integration has an
`identifier` (the agent's canonical name for the integration) and a
`required` boolean.

``` yaml
supported_agents:
  - name: "claude-code"
    integrations:
      - identifier: "google-drive"
        required: true
      - identifier: "google-sheets"
        required: false
```

### 14.1. Behavioral Assumptions Checklist (Informative)

Listing an agent as "supported" means the skill's instructions,
loading assumptions, and runtime behavior have been tested on that
agent. Before adding an agent to `supported_agents`, authors should
verify the following:

1.  **Frontmatter handling.** The skill does not depend on frontmatter
    being stripped or included. Some agents include raw frontmatter in
    model context; others strip it.

2.  **Supporting file loading.** The skill does not assume supporting
    files are auto-loaded into context. Most agents load only the
    skill body.

3.  **Directory enumeration.** The skill does not assume the model
    knows about files in the skill directory. Only some agents
    enumerate the skill directory on activation.

4.  **Nested discovery.** If the skill is nested in a subdirectory,
    the agent discovers it. Some agents scan nested directories;
    others only scan flat paths.

5.  **Path resolution.** Scripts use environment variables or
    relative-to-skill-dir paths, not hardcoded paths.

6.  **Trust gating.** If the skill contains executable scripts,
    note that multiple agents auto-load skills without user approval.

---

## 15. Attestation

*Stability: Draft*

Attestation is the mechanism by which a third party — typically a
registry — independently verifies provenance claims. Without
attestation, provenance fields (`source_repo`, `content_hash`,
`source_commit`) are self-asserted and unverifiable.

**Attestation means authenticity, not safety.** A `source_verified`
attestation proves the content bytes match a specific commit in a
specific repository. It does NOT prove the content is safe, that
scripts have been reviewed, or that the content is free of prompt
injection.

### 15.1. Attestation Records

An attestation record is a JSON object published by an attester:

``` json
{
  "schema": "https://agentskills.io/attestation/v0.1",
  "subject": {
    "content_hash": "sha256:a1b2c3d4...",
    "source_repo": "https://github.com/alice/skills",
    "source_commit": "abc123def456..."
  },
  "attester": {
    "id": "https://registry.example.com",
    "display_name": "Community Skills Registry"
  },
  "claims": [
    { "type": "source_verified" }
  ],
  "self_attestation": false,
  "issued_at": "2026-03-31T12:00:00Z",
  "expires_at": "2027-03-31T12:00:00Z"
}
```

**Required fields:** `schema`, `subject` (with `content_hash`,
`source_repo`, `source_commit`), `attester.id`, `claims`,
`self_attestation`, `issued_at`, `expires_at`.

**Optional fields:** `attester.display_name`, `review_policy` (URL to
the attester's published review policy).

The `claims` array is extensible. Consumers MUST ignore unknown claim
types without error.

### 15.2. Claim Types

v0.2 defines one claim type:

**`source_verified`** — The attester fetched content from
`source_repo` at `source_commit` and confirmed the computed hash
matches `content_hash`. Asserts only that the bytes at the declared
source match the bytes being distributed.

Renewed attestations MUST include `last_reviewed` (ISO 8601 timestamp)
and `review_type` (`automated_hash_check`, `manual_audit`,
`dependency_scan`, or other values). `review_type` is self-reported
and informational — consumers MUST accept unknown values without
error.

### 15.3. Self-Attestation Disclosure

Self-attestation (where the attester is also the content author or
publisher) is disclosed, not prohibited.

-   If `attester.id` and `subject.source_repo` share the same hostname
    AND first path segment (user/org), `self_attestation` MUST be
    `true` regardless of declared value.
-   An attester MAY freely declare `self_attestation: true`.
-   An attester MUST NOT declare `self_attestation: false` when the
    domain comparison indicates self-attestation.

> **Note (informative):** This hostname comparison is a heuristic
> disclosure mechanism, not a security boundary. It catches the common
> case (same GitHub org) but cannot detect all self-attestation
> scenarios (e.g., an attester using a different org on the same
> forge). Future versions may use cryptographic identity for stronger
> binding.

### 15.4. Verification Contract

1.  Verification inputs are `source_repo` + `source_commit`.
2.  Implementations MUST resolve to the exact commit SHA. Branch names
    and tags MUST NOT be accepted as substitutes.
3.  Unresolvable SHA (repo deleted, force-pushed, made private) MUST
    be treated as verification failure, not a warning.

The algorithm MUST be implementable using only standard `git` CLI
operations.

### 15.5. Expiry

`expires_at` is REQUIRED. After expiry, the attestation provides no
trust signal. Recommended default: 12 months. Allowed range: 6–24
months.

When content changes, `content_hash` changes, and all attestations for
the old hash are automatically invalid. Re-attestation is required for
the new content.

### 15.6. Attestation Location

Registries that publish attestations MUST store them at
`.attestations/{content_hash}.json` in the registry repository, where
the colon in the hash prefix is replaced with a hyphen. Example:
content hash `sha256:a1b2c3d4...` → filename
`sha256-a1b2c3d4....json`.

Content authors MAY include attestation breadcrumbs in their source
repository at `.syllago/attestations/{content_hash}.json`. Source-side
records are informational audit trails, not trust anchors.

### 15.7. Display Requirements

v0.2 attestation records are unsigned. Tooling MUST show the attester
identity alongside any attestation status. Tooling MUST NOT display a
bare "verified" or "trusted" badge without attester context. Tooling
SHOULD indicate that v0.2 attestations are unsigned.

---

## 16. Trust States

*Stability: Draft*

Registries MAY assign trust states to indexed content. These are
registry-level metadata — a registry flags content; content does not
flag itself.

| State | Meaning |
|-------|---------|
| `active` | No issues known. May or may not have attestations. |
| `quarantined` | Flagged for review. Not yet confirmed safe or malicious. |
| `denounced` | Confirmed problematic. Removed from active index. |

Tooling SHOULD NOT silently install `denounced` content.

Registries MAY publish a `maintenance_status` for themselves
(`active`, `maintenance-mode`, `archived`) to help consumers
contextualize expired attestations.

---

## 17. Security Considerations

### 17.1. Self-Asserted Provenance

All provenance fields in this convention (Section 7) are
self-asserted. Any skill can claim any `source_repo`, any author, any
`content_hash`. Consumers MUST NOT make trust decisions based solely
on self-asserted provenance without independent verification through
attestation (Section 15) or equivalent mechanisms.

### 17.2. Executable Script Risks

Skills may contain executable scripts within their directory. The
`script_hashes` field (Section 7.11) enables integrity verification
but does not guarantee safety. A verified hash proves the bytes match
— not that the script is safe to execute.

Multiple agents auto-load skills from cloned repositories without user
consent. On those agents, a malicious skill with an executable script
could be activated without user awareness. Skill authors SHOULD
document trust requirements in the skill body. Distribution tools
SHOULD warn users before installing skills that contain executable
files.

### 17.3. Attestation Limitations

Attestation (Section 15) proves authenticity, not safety. A
`source_verified` attestation confirms that the content bytes match a
specific commit in a specific repository. It does NOT prove:

-   The content is free of prompt injection
-   Scripts have been reviewed for malicious behavior
-   The skill will not exfiltrate data
-   The content is appropriate for the user's environment

v0.2 attestation records are unsigned. Tooling MUST clearly
communicate the attester identity and the unsigned nature of v0.2
records.

### 17.4. Sidecar File Sensitivity

Skill authors SHOULD NOT store secrets in the sidecar file, as it may
be distributed alongside the skill through registries and distribution
tools.

### 17.5. Self-Attestation Gaming

The self-attestation disclosure mechanism (Section 15.3) uses hostname
comparison, which is a heuristic. An attester using a different
organization on the same forge can escape detection. This mechanism
catches common cases but is not a security boundary.

---

## 18. References

### 18.1. Normative References

\[AGENTSKILLS\] "Agent Skills Specification",
\<https://agentskills.io/specification\>.

\[RFC2119\] Bradner, S., "Key words for use in RFCs to Indicate
Requirement Levels", BCP 14, RFC 2119, DOI 10.17487/RFC2119,
March 1997, \<https://www.rfc-editor.org/info/rfc2119\>.

\[RFC8174\] Leiba, B., "Ambiguity of Uppercase vs Lowercase in
RFC 2119 Key Words", BCP 14, RFC 8174, DOI 10.17487/RFC8174,
May 2017, \<https://www.rfc-editor.org/info/rfc8174\>.

\[SEMVER\] Preston-Werner, T., "Semantic Versioning 2.0.0",
\<https://semver.org/spec/v2.0.0.html\>.

### 18.2. Informative References

\[SPDX\] "SPDX License List", \<https://spdx.org/licenses/\>.

---

## Appendix A: Trigger Field Mapping to Native Agent Mechanisms (Informative)

Every trigger field in this convention maps to native mechanisms that
agents already implement — mostly for rules, not yet for skills. This
table demonstrates that structured triggers are implementable without
inventing new runtime capabilities.

Convention trigger fields are read from the sidecar file, not from the
SKILL.md frontmatter. Agents that implement this convention resolve
`metadata_file` from the SKILL.md frontmatter, then read trigger
fields from the sidecar.

| Convention Field | Claude Code | Cursor | Kiro | Copilot | Gemini CLI |
|---|---|---|---|---|---|
| `mode: always` | Default behavior | `alwaysApply: true` | `inclusion: always` | Auto-attached instructions | N/D |
| `mode: manual` | `/skill-name` invocation | Manual rule type | `inclusion: manual` + `#name` | N/D | N/D |
| `activation_file_globs` | `paths:` (rules frontmatter) | `globs:` (rules frontmatter) | `fileMatch` + glob | `applyTo:` (instructions frontmatter) | N/D |
| `activation_workspace_globs` | No native equivalent | No native equivalent | No native equivalent | No native equivalent | No native equivalent |
| `commands` | Slash command registration | N/D | N/D | N/D | `/command` registration |
| `keywords` | `description` field (probabilistic) | `description` rule type | `description` for `auto` mode | Reminder prompt routing | N/D |

> **Notable findings:**
>
> `activation_workspace_globs` has no native equivalent in any
> surveyed agent. It is borrowed from VS Code's `activationEvents` and
> represents a genuinely new capability.
>
> `mode` is the highest-value field. Every surveyed agent has an
> implicit mode. Making mode explicit gives distribution tools a
> portable way to express author intent.
>
> Trigger fields provide deterministic activation on agents that
> implement them and structured hints on agents that don't.

---

## Appendix B: Loading Architectures and Convention Field Consumption (Informative)

Agents consume SKILL.md files through three distinct loading
architectures. Understanding which architecture an agent uses helps
convention implementers predict how sidecar fields will be processed.

| Architecture | How It Works | Agents | Sidecar Consumption |
|---|---|---|---|
| **Dedicated skill tool** | A specialized tool handler loads and injects skill content on activation | Codex CLI, Gemini CLI, Cline, Roo Code, OpenCode | Tool handler resolves `metadata_file`, reads sidecar at activation time |
| **Standard read\_file** | The model reads the skill file through the same mechanism as any file | GitHub Copilot | Agent cannot programmatically read the sidecar; the model sees only `metadata_file: ./SKILL.meta.yaml` as text |
| **Implicit injection** | The agent runtime reads and injects skill content without explicit model action | Claude Code, Cursor, Windsurf, Kiro, Amp, Junie | Runtime resolves `metadata_file`, reads sidecar; can influence injection decisions |

> **Implications:**
>
> `triggers.mode` and `triggers.activation_file_globs` are only
> actionable on agents with dedicated skill tools or implicit
> injection. On read\_file agents, the agent cannot read the sidecar
> programmatically.
>
> `durability` is only actionable on agents with implicit injection
> that also implement compaction protection.
>
> `script_hashes` are consumed by distribution tools regardless of
> loading architecture — they operate at install time, not runtime.

---

## Appendix C: Canonical Agent Identifiers (Informative)

The following identifiers are recognized in v0.2 of this convention.
Use these exact strings in `supported_agents.name` entries. New
identifiers will be added in convention patches as agents adopt the
Agent Skills specification.

| Identifier | Product Name | Form Factor |
|---|---|---|
| `amp` | Amp | CLI |
| `claude-code` | Claude Code | CLI |
| `cline` | Cline | IDE extension |
| `codex-cli` | Codex CLI | CLI |
| `cursor` | Cursor | IDE |
| `gemini-cli` | Gemini CLI | CLI |
| `github-copilot` | GitHub Copilot | IDE extension |
| `junie-cli` | JetBrains Junie CLI | CLI |
| `kiro` | Kiro | IDE |
| `opencode` | OpenCode | CLI |
| `roo-code` | Roo Code | IDE extension |
| `windsurf` | Windsurf | IDE |

Agents not listed here SHOULD use a lowercase hyphenated form of the
product name (e.g., `my-agent`). Identifier strings MUST match the
regex `[a-z][a-z0-9-]*` (lowercase ASCII, hyphens, no leading
digits).

---

## Appendix D: Full Example (Informative)

**SKILL.md:**

``` yaml
---
name: code-review
description: Reviews code for quality, security, and style. Use when reviewing PRs, checking code quality, or preparing code for merge.
license: MIT
metadata_file: ./SKILL.meta.yaml
---

## Instructions

When reviewing code, follow these steps:
...
```

**SKILL.meta.yaml:**

``` yaml
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.2"
provenance:
  version: "2.1.0"
  source_repo: "https://github.com/acme-org/agent-skills"
  source_commit: "abc123def456789012345678901234567890abcd"
  source_repo_subdirectory: "/skills/code-review"
  content_hash: "sha256:a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890"
  authors:
    - "Alice Smith <alice@example.com>"
    - "Bob Jones <bob@acme.org>"
  publisher: "acme-skills-registry"
  license_spdx_id: "MIT"
  license_url: "https://github.com/acme-org/agent-skills/blob/main/LICENSE"
tags:
  - "code-quality"
  - "review"
  - "typescript"
status: "active"
durability: persistent
expectations:
  software:
    - name: "git"
      version: ">=2.30"
  runtimes:
    - name: "node"
      version: ">=18"
triggers:
  mode: auto
  activation: single
  activation_file_globs:
    - "**/*.ts"
    - "**/*.js"
  activation_workspace_globs:
    - ".github/pull_request_template.md"
  commands:
    - "review"
  keywords:
    - "code review"
    - "review this PR"
    - "check my code"
supported_agents:
  - name: "claude-code"
  - name: "cursor"
  - name: "gemini-cli"
```
