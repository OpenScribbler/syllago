# Agent Skills Metadata Convention

**Version:** 0.1.0 (Draft)
**Status:** Companion convention to the [Agent Skills Specification](https://agentskills.io/specification)
**Repository:** To be hosted at the Agent Ecosystem GitHub organization

The key words MUST, MUST NOT, SHOULD, SHOULD NOT, and MAY in this
document are to be interpreted as described in
[RFC 2119](https://www.ietf.org/rfc/rfc2119.txt).

---

## Relationship to the Agent Skills Specification

This document is a **companion convention** to the Agent Skills
specification at agentskills.io. It does not replace, fork, or
override that specification. It defines structured metadata fields
that live in a **sidecar file** (`SKILL.meta.yaml`) alongside the
SKILL.md, referenced by a single optional `metadata_file` field in
the SKILL.md frontmatter.

The Agent Skills spec defines the **mechanism** — a YAML frontmatter
format with `name`, `description`, `license`, `compatibility`,
`metadata`, and `allowed-tools`. This convention defines a
**vocabulary** of structured metadata (provenance, triggers,
expectations, agent compatibility) and places it in an external
sidecar file to preserve the spec's token-efficient design.

### Why a sidecar file, not `metadata:`

The Agent Skills spec's `metadata` field is defined as "a map from
string keys to string values." This convention requires structured
data (nested objects, arrays) that exceeds that definition. Earlier
drafts placed convention fields inside `metadata:` as nested YAML,
but this caused two problems:

1.  **Token cost.** Agents that do not strip frontmatter before
    context injection (confirmed: Codex CLI, GitHub Copilot) would
    inject ~30 lines of metadata YAML into the model's context on
    every skill activation. The Agent Skills spec's original design
    — just `name` and `description` — is deliberately minimal to
    avoid this. Adding 30 lines of tooling metadata directly
    contradicts that design philosophy.

2.  **Spec boundary.** Nested YAML objects inside a string→string
    map exceed the base spec's stated definition of `metadata`.

Moving convention fields to a sidecar file resolves both: the
SKILL.md frontmatter stays minimal (3–4 lines), and structured
data lives in a file the model never sees. See the
[Token Efficiency Decision](reference/token-efficiency-metadata-separation.md)
for the full design rationale including a 5-persona panel review.

### The `metadata_file` field

This convention introduces `metadata_file` as a **top-level
frontmatter field** — alongside `name`, `description`, `license`,
`compatibility`, `metadata`, and `allowed-tools`.

This is the convention's **single addition** to the SKILL.md
frontmatter schema. It is proposed for inclusion in the Agent Skills
spec as a top-level field. Until adopted, agents SHOULD ignore
unknown top-level fields without error (standard forward-compatibility
practice, per JSON Schema, OpenAPI, and similar specifications).

`metadata_file` and convention fields inside `metadata:` are
**mutually exclusive**. A SKILL.md MUST NOT contain both
`metadata_file` and convention-defined keys inside `metadata:`.

A SKILL.md file using this convention remains fully valid under the
Agent Skills spec — agents that do not recognize `metadata_file`
will ignore it, and the skill's `name`, `description`, and body
content are unaffected.

### Compatibility

Agent Skills required fields (`name`, `description`) remain REQUIRED
and are not affected by this convention. The `license`,
`compatibility`, and `allowed-tools` fields remain as defined by the
Agent Skills spec.

This convention's fields are additive. A minimal SKILL.md needs only
`name` and `description`. Everything in this convention is optional.
A SKILL.md without `metadata_file` is fully valid — no degraded
status, no warnings.

### Example: Minimal + Convention

**SKILL.md** (3 lines of frontmatter — the model sees only this):

``` yaml
---
name: code-review
description: Reviews code for quality, security, and style issues. Use when reviewing PRs or checking code quality.
license: MIT
metadata_file: ./SKILL.meta.yaml
---

## Instructions

Review the code for...
```

**SKILL.meta.yaml** (tooling reads this — the model never sees it):

``` yaml
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.1"
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

---

## Sidecar File Format

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

### Schema

The sidecar file is **flat YAML** — no wrapping `metadata:` key.
The entire file IS metadata; wrapping it would be redundant. Top-level
keys are `metadata_spec`, `provenance`, `triggers`, `expectations`,
`tags`, `status`, `durability`, and `supported_agents`.

**Required fields:** `metadata_spec` (identifies the convention version).

**Optional fields:** All other convention fields defined in this
document (sections 1–9 below).

Agents implementing this convention MUST ignore unknown keys in the
sidecar file without error. This ensures forward compatibility when
newer convention versions add fields.

### Discovery rules

1.  If `metadata_file` is present in the SKILL.md frontmatter,
    tooling MUST resolve it as a relative path from the SKILL.md
    file's directory.

2.  If `metadata_file` is absent, tooling MUST NOT search for
    sidecar files by naming convention. Absent field = no convention
    metadata exists. Period.

3.  If `metadata_file` is present but the referenced file does not
    exist, tooling SHOULD warn the author (broken reference). The
    SKILL.md remains valid and functional — the skill just has no
    convention metadata.

### Content hash interaction

When computing `content_hash` (section 1), the sidecar file
(`SKILL.meta.yaml`) IS included in the hash computation — it is a
regular file in the skill directory. The `content_hash` blanking
step applies only to the file containing the hash value itself
(typically the sidecar file in this two-file pattern, not the
SKILL.md).

---

## Stability Levels

Each field in this convention has an assigned stability level:

| Level | Meaning | Guarantees |
|-------|---------|------------|
| **Stable** | Community consensus, multiple implementations. | Semantics will not change. Field may be proposed for inclusion in the Agent Skills core spec. |
| **Draft** | Being experimented with. | May change or be removed based on implementation experience. Adopters should expect possible breaking changes. |

Fields that reach Stable status with demonstrated adoption across
multiple agents and registries are candidates for proposal into the
Agent Skills core spec as top-level frontmatter fields.

---

## Terminology

These terms are used throughout this convention:

-   **agent** — A software product that uses large language models to
    help users write, review, or maintain code. An agent is what you
    install, launch, and interact with. Ex. Claude Code, Cursor, GitHub
    Copilot, Gemini CLI.

-   **model provider** — A company or service providing the underlying
    language model that powers an agent. Ex. Anthropic (Claude), OpenAI
    (GPT), Google (Gemini). One agent may support multiple model
    providers.

-   **form factor** — How an agent is delivered to users. Values: IDE,
    CLI, extension, web, autonomous, hybrid.

---

## Convention Fields

All fields below are placed in the **metadata sidecar file**
(`SKILL.meta.yaml`), not in the SKILL.md frontmatter. The SKILL.md
contains only `metadata_file` pointing to the sidecar. See
[Sidecar File Format](#sidecar-file-format) for the file schema
and discovery rules.

The `metadata_spec` key identifies which version of this convention
the sidecar follows. Agents implementing this convention MUST ignore
unknown keys within convention sub-maps without error. This ensures
forward compatibility when newer convention versions add fields.

### metadata_spec

**Stability:** Stable

A URI identifying the version of this metadata convention. This value
is an opaque identifier, not a locator — agents MUST NOT attempt to
dereference it. Agents MUST use exact string comparison to match the
`metadata_spec` value against known convention versions.

Agents that recognize the URI MAY parse the structured metadata
fields defined below. Agents that do not recognize it MUST ignore
all convention fields without error. Agents MUST NOT interpret
convention-defined fields (provenance, triggers, expectations,
supported\_agents) unless the `metadata_spec` key is present and matches
a recognized value.

``` yaml
# In SKILL.meta.yaml (top-level key, not nested under metadata:)
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.1"
```

### 1. Provenance

*Stability: Stable*

Provenance fields describe where the skill came from, who created it,
and how to verify its integrity.

#### provenance.version

The version of this specific skill. Value is a
[Semver](https://semver.org/) string. MUST NOT start with `v`.

Registries MAY reject skill submissions that omit `version`, as
versioning is essential for update detection, deduplication, and
dependency resolution in registry contexts.

**Immutability:** Registries MUST reject submissions where `version`
matches an existing entry but `content_hash` differs. Published
versions are immutable — to publish different content, bump the
version.

``` yaml
provenance:
  version: "4.2.0"
```

#### provenance.source\_repo

A URL indicating where the canonical version of this skill can be
found. MUST be an HTTPS URL. The URL is forge-agnostic — GitHub,
GitLab, Codeberg, self-hosted Gitea, or any git hosting service.

REQUIRED for content submitted to registries. Registries MUST
reject submissions without `source_repo` and MUST verify the URL
resolves at submission time. OPTIONAL for local-only content.

This field is a **self-asserted claim** by the skill author unless
independently verified through attestation (see section 8).
Distribution tools and registries SHOULD NOT treat it as verified
provenance without attestation or equivalent verification.

``` yaml
provenance:
  source_repo: "https://github.com/my_username/cool_skill_repo"
```

#### provenance.source\_commit

*Strongly recommended.* The full git commit SHA at which the content
was published. MUST be exactly 40 lowercase hexadecimal characters.

`source_commit` provides an immutable reference point. Unlike branch
names or tags, commit SHAs cannot be rewritten after the fact. When
both `source_commit` and `content_hash` are present, verification
tools can deterministically check whether the content at a specific
repo at a specific commit produces a specific hash.

`source_commit` SHOULD be captured automatically by tooling, not
manually entered by authors. Registries SHOULD capture the commit
SHA at submission time. Distribution tools SHOULD capture the
current HEAD SHA during publishing workflows.

``` yaml
provenance:
  source_commit: "abc123def456789012345678901234567890abcd"
```

#### provenance.source\_repo\_subdirectory

Only necessary when there is more than one skill within the repo. Path
within the repository, relative to the repository root. MUST start with
`/`. When there is only one skill it is assumed to be `/`.

Given the following repository structure, the "integration" skill would
have a `source_repo_subdirectory` value of `/skills/tool_x/integration`:

    Repository
    └── skills
        └── tool_x
            ├── integration
            └── search

#### provenance.content\_hash

*Strongly recommended.* A cryptographic hash of the skill's content
files, using the format `algorithm:hex_value`. The algorithm MUST be
`sha256`. The hex value MUST be 64 lowercase hexadecimal characters.

Ex. `sha256:a1b2c3d4e5f67890...` (64 chars total)

The hash enables integrity verification and drift detection. To compute
the hash deterministically:

1.  Enumerate all files in the skill directory recursively. Exclude
    any file or directory whose name starts with `.` (hidden files
    and hidden directories, including all their contents — e.g.,
    `.git/`, `.DS_Store`). The file containing the YAML frontmatter
    (typically `SKILL.md`) IS included after the blanking step
    described below.
2.  **Blank the `content_hash` value.** In the frontmatter file
    (typically `SKILL.md`), perform a byte-level substitution: find
    the first occurrence of the pattern
    `content_hash: "sha256:<64 hex chars>"` and replace the 71-character
    value string (`"sha256:<64 hex chars>"`) with `""` (two ASCII
    double-quote characters with nothing between them). The resulting
    line MUST be `content_hash: ""`. This is a raw byte operation on
    the file content — do NOT re-serialize YAML, as serializers
    produce inconsistent whitespace and quoting. If `content_hash` is
    absent or already `""`, no substitution is needed.
3.  **Normalize line endings.** For each file, replace all `\r\n`
    (CRLF) sequences with `\n` (LF) before hashing. This ensures
    identical hashes regardless of platform checkout settings (e.g.,
    `git core.autocrlf`). Bare `\r` characters are left unchanged.
4.  Construct relative paths from the skill directory root using
    forward slash (`/`) as separator on all platforms. Paths MUST
    NOT start with `/` or `./`. Example: `scripts/extract.py`,
    `SKILL.md`. Relative paths MUST be encoded as UTF-8 bytes.
5.  Sort paths lexicographically by UTF-8 byte value.
6.  For each file, compute SHA-256 of:
    `relative_path_bytes + '\0' + normalized_file_bytes`.
7.  Concatenate all per-file hashes (raw 32-byte digests, not hex).
8.  Compute a final SHA-256 over the concatenated result.

Empty directories and symlinks are excluded. Only regular files are
hashed.

#### provenance.authors

A list of strings identifying the skill's creators. Uses the git author
format: `Name <email>`. The email portion is optional.

``` yaml
provenance:
  authors:
    - "Mary Smith <mary@example.com>"
    - "Andrea Barley"
```

#### provenance.publisher

The entity that distributed this skill to the consumer. Distinct from
`authors` — the publisher may not have written the skill.

This field is typically set by a distribution tool or registry, not by
the skill author. It is a plain string identifier. Like `source_repo`,
this is a **self-asserted claim** — any skill can claim any publisher.
Distribution tools SHOULD NOT make authorization or trust decisions
based solely on this field without attestation (section 8) or
equivalent independent verification.

#### provenance.license\_spdx\_id

*Strongly recommended.* An [SPDX license identifier](https://spdx.org/licenses/).
MUST be a valid SPDX expression. Ex. `MIT`, `Apache-2.0`, `GPL-3.0-only`.

If the skill has no license, use `UNLICENSED`. If the license is
unknown, use `NOASSERTION`.

This complements the Agent Skills spec's existing `license` field
(which is freeform) with a machine-parseable identifier. When both
are present, `license_spdx_id` is the authoritative value for machine
processing. Authors SHOULD keep both fields consistent.

#### provenance.license\_url

The URL to the full license text including author and copyright
attribution.

When a skill is extracted from a multi-skill repository, the
`license_url` SHOULD point to the license that applies to *this
specific skill*, which may differ from the repository's root license.

#### provenance.derived\_from

An array of objects indicating that this skill was derived from one
or more other skills. Each entry contains the upstream coordinates
and the type of derivation. Most derivations have a single parent
(array of one). Merges have multiple parents.

Each skill points only to its immediate parents — if you need the
full chain, follow the links (each upstream has its own
`derived_from` field).

All fields within `derived_from` entries are **self-asserted** by the
skill author. A skill can claim derivation from any upstream without
proof. Enterprise tooling SHOULD treat `derived_from` as informational
metadata, not a verified supply chain attestation, until attestation
(section 8) or signing (future version) provides independent linkage.

Each entry in the array contains:

-   `source_repo` — REQUIRED. Upstream repository URL.
-   `source_repo_subdirectory` — Path within the upstream repo.
-   `version` — Upstream version at the time of derivation.
-   `content_hash` — Upstream content hash at the time of derivation.
-   `relation` — REQUIRED. The type of derivation.
    Known values: `fork`, `convert`, `extract`, `merge`. Agents
    MUST accept unknown relation values without error (the vocabulary
    is extensible).

| Relation  | Meaning |
|-----------|---------|
| `fork`    | Copied and potentially modified. |
| `convert` | Transformed to another agent's format. |
| `extract` | Pulled from a larger bundle. |
| `merge`   | Combined from multiple sources. |

#### provenance.script\_hashes

*Stability: Draft*

A map of relative file paths to cryptographic hashes for executable
files within the skill directory. Each key is a relative path (same
format as `content_hash` file enumeration) and each value uses the
`algorithm:hex_value` format. The algorithm MUST be `sha256`.

Script hashes enable distribution tools to verify script integrity on
install, agents with trust gating to compare hashes before execution,
and drift detection when scripts change after publication.

This field is especially important given the current security landscape:
of 12 surveyed agents, only 3 gate skill loading behind user approval
(Gemini CLI, Roo Code, OpenCode), while 7 auto-load skills from cloned
repositories with no consent (Codex CLI, Cursor, Windsurf, Copilot,
Cline, Amp, Junie). On those 7 agents, a malicious skill with an
executable script could be activated without user awareness.

`script_hashes` does not solve the gating problem — agents must
implement approval mechanisms — but it provides the building blocks for
a trust chain. Without hashes, there is nothing to verify even if an
agent adds gating later.

``` yaml
provenance:
  script_hashes:
    "scripts/deploy.sh": "sha256:def456..."
    "scripts/validate.py": "sha256:789abc..."
```

#### Full provenance example

``` yaml
# In SKILL.meta.yaml
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.1"
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

**Note on token efficiency:** Convention fields live in the sidecar
file, which the model never sees. The SKILL.md frontmatter contains
only `name`, `description`, `license`, and optionally `metadata_file`
— at most 4 lines. This preserves the Agent Skills spec's
token-efficient design. Agents that do not strip frontmatter
(confirmed: Codex CLI, GitHub Copilot) see at most one extra line
(`metadata_file`) instead of 30+ lines of convention metadata.
Skill authors SHOULD NOT store secrets in the sidecar file, as it
may be distributed alongside the skill.

### 2. Expectations

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
| `~>`     | Compatible (pessimistic). See algorithm below. | `~>3.4` |

**`~>` (pessimistic constraint) algorithm:** Given `~>X.Y.Z`,
the constraint expands to `>=X.Y.Z and <X.(Y+1).0`. Given `~>X.Y`
(two components), it expands to `>=X.Y.0 and <(X+1).0.0`. The
operand MUST have exactly 2 or 3 numeric components separated by
`.` — single-component (`~>1`) and 4+-component (`~>1.2.3.4`)
values are invalid. This operator follows RubyGems semantics: increment
the second-to-last specified component and drop trailing components.

| Input | Expands to |
|-------|-----------|
| `~>3.4` | `>=3.4.0 and <4.0.0` |
| `~>1.2.3` | `>=1.2.3 and <1.3.0` |
| `~>0.9` | `>=0.9.0 and <1.0.0` |

Multiple constraints may be joined with `and`: `>=1.2.0 and <2.0.0`.

#### expectations.software

An array of objects. CLI tools or applications expected to be present.
Multiple entries use AND logic (all are required).

Each entry MUST include `name` (the canonical upstream binary name —
the name you type to invoke it). Each entry MAY include `version`
(a version constraint string). If `version` is absent, any version
is acceptable.

``` yaml
expectations:
  software:
    - name: "bat"
      version: ">=0.22"
    - name: "ripgrep"
      version: ">=14.1.0"
```

#### expectations.services

An array of objects. Remote services the user needs an account on.

Each entry MUST include `name`. Each entry MAY include `url` (the
service homepage).

``` yaml
expectations:
  services:
    - name: "Notion"
      url: "https://notion.so"
```

#### expectations.runtimes

An array of objects. Programming language runtimes required.

Each entry MUST include `name` (the canonical runtime command name,
e.g., `node`, `python`, `ruby`). Each entry MAY include `version`.
If `version` is absent, any version is acceptable.

``` yaml
expectations:
  runtimes:
    - name: "ruby"
      version: "~>3.4"
    - name: "node"
      version: ">=18"
```

#### expectations.operating\_systems

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

### 3. Triggers

*Stability: Draft*

Structured activation conditions. Triggers tell an agent *when* to
activate the skill, separately from `description` which explains
*what* the skill does.

Studies have shown that description-only activation achieves 50-77%
reliability. Deterministic triggers (file\_patterns,
workspace\_contains, commands) are evaluated mechanically by the
agent's runtime with 100% reliability. Probabilistic triggers
(keywords) provide structured hints for better LLM routing.

Multiple triggers within a skill use OR logic — any matching trigger
activates the skill.

#### triggers.mode

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

**Worked example:** A skill has `mode: always` and
`activation_file_globs: ["**/*.rs"]`. The skill activates on every
project, including those with no Rust files. The globs are not
evaluated — `always` takes precedence unconditionally.

All trigger patterns use standard glob syntax (not regex, not
gitignore syntax). See pattern operators below.

#### triggers.activation\_file\_globs

An array of glob patterns. The skill SHOULD activate when the user
is working with files matching any pattern. Paths are relative to
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
trigger patterns. Agents MAY support them as extensions but MUST
NOT require them.

``` yaml
triggers:
  activation_file_globs:
    - "**/*.test.ts"
    - "**/*.spec.ts"
```

#### triggers.activation\_workspace\_globs

An array of glob patterns. The skill SHOULD activate when the project
contains files matching any pattern. Evaluated once at project load,
not on every file change.

``` yaml
triggers:
  activation_workspace_globs:
    - "docker-compose.yml"
    - "Dockerfile"
```

#### triggers.commands

An array of command name strings that explicitly invoke this skill.
The format is agent-specific — some agents use `/command`, others use
`@command` or other syntax. The value SHOULD be the bare command name
without prefix (e.g., `test-review` not `/test-review`), and agents
SHOULD apply their own prefix convention.

When multiple skills register the same command name, agents are
responsible for disambiguation (e.g., prompting the user, using
skill name as a namespace). Registries SHOULD flag command name
collisions across indexed skills.

``` yaml
triggers:
  commands:
    - "test-review"
    - "check-coverage"
```

#### triggers.activation

*Stability: Draft*

A hint for how agents should handle re-activation of this skill within
a session. If absent, agents SHOULD treat the skill as `single`.

| Value        | Meaning |
|--------------|---------|
| `single`     | This skill should activate at most once per session. Agents SHOULD deduplicate. Default. |
| `repeatable` | This skill may be meaningfully re-activated (e.g., a skill that produces different output based on conversation state). |

Skill deduplication varies dramatically across agents: GitHub Copilot
has robust URI + content dedup, Codex CLI deduplicates per-turn only,
Cline and Roo Code use prompt-level instructions (fragile), and
OpenCode, Gemini CLI, and others have no dedup — each re-activation
injects the full skill content again, wasting tokens.

`activation` gives agents a lightweight signal they can implement with
whatever mechanism they have — Copilot's `hasSeen` set, a prompt
instruction, or a simple "already activated" flag.

``` yaml
triggers:
  mode: auto
  activation: single
```

#### triggers.keywords

An array of phrases. These provide structured hints to the agent's
LLM for routing. Whether matching is exact, substring, or semantic
is agent-specific. The convention provides the vocabulary; the agent
decides how to use it.

``` yaml
triggers:
  keywords:
    - "review tests"
    - "check test coverage"
    - "test quality"
```

### 4. Tags

*Stability: Draft*

An array of freeform strings for categorization and discovery.
Registries and search tools MAY use tags for faceted search and
filtering. There is no controlled vocabulary — authors choose tags
that describe the skill's domain and purpose.

Tags SHOULD be lowercase. Avoid redundancy with the skill's `name`
or `description`.

``` yaml
# In SKILL.meta.yaml
tags:
  - "testing"
  - "code-quality"
  - "typescript"
```

### 5. Status

*Stability: Draft*

The lifecycle status of the skill. If absent, the skill is presumed
to be `active`.

| Value        | Meaning |
|--------------|---------|
| `active`     | The skill is maintained and recommended for use. Default. |
| `deprecated` | The skill is no longer recommended. It still works but a better alternative may exist. |
| `archived`   | The skill is no longer maintained. It may not work with current agents. |

``` yaml
# In SKILL.meta.yaml
status: "deprecated"
```

### 6. Durability

*Stability: Draft*

The author's intended behavior for context compaction. If absent, the
skill is presumed to be `persistent`.

| Value        | Meaning |
|--------------|---------|
| `persistent` | These instructions must remain in context for the full session. Agents with compaction SHOULD protect this skill's content from pruning. Default. |
| `ephemeral`  | These instructions are useful for the current task only and can be safely summarized or dropped during compaction. |

This field addresses a significant portability risk: context compaction
protection varies widely across agents. Of 12 surveyed agents, only
Claude Code (for CLAUDE.md-embedded content) confirms compaction
protection. OpenCode confirmed no protection — its compaction prompt
is generic with no skill awareness. The remaining 10 agents are
undocumented, meaning skill authors cannot know whether their
instructions will survive long conversations.

`durability` does not guarantee protection — agents must implement it.
But it signals author intent to agents that support compaction
protection, gives distribution tools a basis for warnings ("This skill
is marked `persistent` but Agent X has no compaction protection"), and
creates a spec-level hook that agents can adopt incrementally.

``` yaml
# In SKILL.meta.yaml
durability: persistent
```

### 7. Supported Agents

*Stability: Draft*

An array of agents this skill is known to work with. If absent, the
skill is presumed to be generic and should work with any agent.

Note: this complements the Agent Skills spec's freeform
`compatibility` field with structured agent declarations.

Each entry MUST include a `name`. Agent names MUST use the canonical
identifier from Appendix C (e.g., `claude-code`, `cursor`,
`gemini-cli`). Agents not yet listed in Appendix C SHOULD use a
lowercase hyphenated form of the product name. New identifiers will
be added in convention patches.

Each entry MAY include an `integrations` array listing platform
integrations required for the skill. Each integration has an
`identifier` (the agent's canonical name for the integration) and a
`required` boolean.

``` yaml
# In SKILL.meta.yaml
supported_agents:
  - name: "claude-code"
    integrations:
      - identifier: "google-drive"
        required: true
      - identifier: "google-sheets"
        required: false
```

#### Behavioral assumptions checklist

Listing an agent as "supported" means the skill's instructions,
loading assumptions, and runtime behavior have been tested on that
agent. Before adding an agent to `supported_agents`, authors SHOULD
verify the following (non-normative):

1.  **Frontmatter handling.** Your skill does not depend on frontmatter
    being stripped or included. Codex CLI and GitHub Copilot include
    raw frontmatter in model context; Claude Code, Gemini CLI, Cline,
    Roo Code, and OpenCode strip it.

2.  **Supporting file loading.** Your skill does not assume supporting
    files are auto-loaded into context. Only Windsurf may load all
    skill directory files. All other agents load only the skill body.

3.  **Directory enumeration.** Your skill does not assume the model
    knows about files in the skill directory. Only Gemini CLI and
    OpenCode enumerate the skill directory on activation.

4.  **Nested discovery.** If your skill is nested in a subdirectory,
    verify the agent discovers it. Claude Code, Codex CLI, and
    OpenCode scan nested directories; 6 agents only scan flat paths.

5.  **Path resolution.** Your scripts use environment variables or
    relative-to-skill-dir paths, not hardcoded paths. Most agents
    resolve relative to the skill directory, but Kiro may use the
    workspace root.

6.  **Trust gating.** If your skill contains executable scripts, note
    that 7 of 12 agents auto-load skills without user approval.
    Security-sensitive skills may want to declare a minimum trust
    requirement in their instructions.

### 8. Attestation

*Stability: Draft*

All provenance fields in this convention (section 1) are self-asserted
— any skill can claim any `source_repo`, any author, any hash. Of 12
surveyed AI agents, 7 auto-load skills without user approval, and
skills can contain executable scripts. Attestation provides a mechanism
for independent verification of provenance claims, so that consumers
can distinguish verified origins from unverified ones.

Attestation is the mechanism by which a third party — typically a
registry — independently verifies provenance claims. Without
attestation, provenance fields (`source_repo`, `content_hash`,
`source_commit`) are self-asserted and unverifiable.

**Attestation means authenticity, not safety.** A `source_verified`
attestation proves the content bytes match a specific commit in a
specific repository. It does NOT prove the content is safe, that
scripts have been reviewed, or that the content is free of prompt
injection.

#### Attestation records

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

**Optional fields:** `attester.display_name`, `review_policy` (URL
to the attester's published review policy).

The `claims` array is extensible. Consumers MUST ignore unknown
claim types without error.

#### Claim types

v0.1 defines one claim type:

**`source_verified`** — The attester fetched content from
`source_repo` at `source_commit` and confirmed the computed hash
matches `content_hash`. Asserts only that the bytes at the declared
source match the bytes being distributed.

Renewed attestations MUST include `last_reviewed` (ISO 8601
timestamp) and `review_type` (`automated_hash_check`,
`manual_audit`, `dependency_scan`, or other values). `review_type`
is self-reported and informational — consumers MUST accept unknown
values without error.

Future claim types (`script_reviewed`, `conversion_verified`,
`publisher_vouched`, `content_safety_reviewed`) will be defined in
subsequent convention versions. The `claims` array accommodates
them without schema changes.

#### Self-attestation disclosure

Self-attestation (where the attester is also the content author or
publisher) is disclosed, not prohibited. Asymmetric enforcement:

-   If `attester.id` and `subject.source_repo` share the same
    hostname AND first path segment (user/org), `self_attestation`
    MUST be `true` regardless of declared value.
-   An attester MAY freely declare `self_attestation: true`.
-   An attester MUST NOT declare `self_attestation: false` when
    the domain comparison indicates self-attestation.

Acceptable error direction: false positive (independent attester
flagged as self) is acceptable. False negative (self-attester
escaping to `false`) is a spec violation.

This hostname comparison is a heuristic disclosure mechanism, not
a security boundary. It catches the common case (same GitHub org)
but cannot detect all self-attestation scenarios (e.g., an attester
using a different org on the same forge). Future versions may use
cryptographic identity for stronger binding.

#### Verification contract

1.  Verification inputs are `source_repo` + `source_commit`.
2.  Implementations MUST resolve to the exact commit SHA. Branch
    names and tags MUST NOT be accepted as substitutes.
3.  Unresolvable SHA (repo deleted, force-pushed, made private)
    MUST be treated as verification failure, not a warning.

The algorithm MUST be implementable using only standard `git` CLI
operations. Forge-specific API calls are a tooling optimization,
not a requirement.

#### Expiry

`expires_at` is REQUIRED. After expiry, the attestation provides
no trust signal. Recommended default: 12 months. Allowed range:
6–24 months.

When content changes, `content_hash` changes, and all attestations
for the old hash are automatically invalid. Re-attestation is
required for the new content.

#### Attestation location

Registries that publish attestations MUST store them at
`.attestations/{content_hash}.json` in the registry repository,
where the colon in the hash prefix is replaced with a hyphen.
Example: content hash `sha256:a1b2c3d4...` → filename
`sha256-a1b2c3d4....json`.

Content authors MAY include attestation breadcrumbs in their source
repository at `.syllago/attestations/{content_hash}.json`. Source-
side records are informational audit trails, not trust anchors.

When content moves between registries, the receiving registry
SHOULD verify independently and issue its own attestation.

#### Display requirements

v0.1 attestation records are unsigned. Tooling MUST show the
attester identity alongside any attestation status. Tooling MUST
NOT display a bare "verified" or "trusted" badge without attester
context. Tooling SHOULD indicate that v0.1 attestations are
unsigned.

### 9. Trust States

*Stability: Draft*

Registries MAY assign trust states to indexed content. These are
registry-level metadata — a registry flags content; content does
not flag itself.

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

## Full Example

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
metadata_spec: "https://github.com/agent-ecosystem/metadata-convention/v0.1"
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

---

## Appendix A: Trigger Field Mapping to Native Agent Mechanisms (Informative)

Every trigger field in this convention maps to native mechanisms that
agents already implement — mostly for rules, not yet for skills. This
table demonstrates that structured triggers are implementable without
inventing new runtime capabilities.

Convention trigger fields are read from the **sidecar file**
(`SKILL.meta.yaml`), not from the SKILL.md frontmatter. Agents that
implement this convention resolve `metadata_file` from the SKILL.md
frontmatter, then read trigger fields from the sidecar.

| Convention Field | Claude Code | Cursor | Kiro | Copilot | Gemini CLI |
|---|---|---|---|---|---|
| `mode: always` | Default behavior | `alwaysApply: true` | `inclusion: always` | Auto-attached instructions | N/D |
| `mode: manual` | `/skill-name` invocation | Manual rule type | `inclusion: manual` + `#name` | N/D | N/D |
| `activation_file_globs` | `paths:` (rules frontmatter) | `globs:` (rules frontmatter) | `fileMatch` + glob | `applyTo:` (instructions frontmatter) | N/D |
| `activation_workspace_globs` | No native equivalent | No native equivalent | No native equivalent | No native equivalent | No native equivalent |
| `commands` | Slash command registration | N/D | N/D | N/D | `/command` registration |
| `keywords` | `description` field (probabilistic) | `description` rule type | `description` for `auto` mode | Reminder prompt routing | N/D |

**Notable findings:**

-   `activation_workspace_globs` has **no native equivalent in any agent**. It
    is borrowed from VS Code's `activationEvents` and represents a
    genuinely new capability. Agents will need to build this, not just
    wire existing mechanisms.

-   `mode` is the **highest-value field**. Every agent already has an
    implicit mode (Claude Code defaults to auto, Cursor has 4 explicit
    modes, Kiro has 4 inclusion modes). Making mode explicit gives
    distribution tools a portable way to express author intent.

-   Trigger fields provide **deterministic activation on agents that
    implement them** and **structured hints on agents that don't**.
    On agents with `read_file` architectures (e.g., GitHub Copilot),
    the model sees triggers as YAML text and must interpret them.

**Data source:** Behavior matrix Q1 — discovery depth and activation
mechanisms across 12 agents.

---

## Appendix B: Loading Architectures and Convention Field Consumption (Informative)

Agents consume SKILL.md files through three distinct loading
architectures. Understanding which architecture an agent uses helps
convention implementers predict how sidecar fields will be processed.

With the sidecar pattern, convention field consumption is a two-step
process: (1) the agent reads `metadata_file` from the SKILL.md
frontmatter, (2) the agent reads convention fields from the sidecar.
Agents that do not implement this convention skip both steps — the
SKILL.md frontmatter contains at most one unrecognized field
(`metadata_file`), not 30+ lines of convention metadata.

| Architecture | How It Works | Agents | How Sidecar Fields Are Consumed |
|---|---|---|---|
| **Dedicated skill tool** | A specialized tool handler loads and injects skill content on activation | Codex CLI, Gemini CLI, Cline, Roo Code, OpenCode | Tool handler resolves `metadata_file`, reads sidecar at activation time; can influence tool output format |
| **Standard read\_file** | The model reads the skill file through the same mechanism as any file | GitHub Copilot | Agent cannot programmatically read the sidecar. The model sees only `metadata_file: ./SKILL.meta.yaml` as a single line in frontmatter |
| **Implicit injection** | The agent runtime reads and injects skill content without explicit model action | Claude Code, Cursor, Windsurf, Kiro, Amp, Junie | Runtime resolves `metadata_file`, reads sidecar; can influence injection decisions (e.g., mode determines whether to inject) |

**Implications for convention fields:**

-   `triggers.mode` and `triggers.activation_file_globs` are only actionable on
    agents with dedicated skill tools or implicit injection. On
    `read_file` agents (Copilot), the agent cannot read the sidecar
    programmatically — the model sees only the `metadata_file` pointer,
    not the trigger fields themselves.

-   `durability` is only actionable on agents with implicit injection
    that also implement compaction protection. On other architectures,
    it serves as documentation of author intent.

-   `script_hashes` are consumed by distribution tools (like syllago)
    regardless of loading architecture. They operate at install time,
    not runtime.

**Data source:** Behavior matrix Q1, Q2, Q8 — loading mechanisms,
tool architecture, and activation paths across 12 agents.

---

## Appendix C: Canonical Agent Identifiers (Informative)

The following identifiers are recognized in v0.1 of this convention.
Use these exact strings in `supported_agents.name` entries. The list
is sorted alphabetically. New identifiers will be added in convention
patches as agents adopt the Agent Skills specification.

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
