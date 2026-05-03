# Syllago — Ubiquitous Language

_Canonical domain vocabulary for this repo. When a term has a bold canonical name, use it verbatim in code, docs, tests, commit messages, and PR descriptions. If you see an "alias to avoid," do not use it._

## Content Model

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Content Item** | A single, named, addressable piece of AI tool content stored as a directory or file within the library or a registry. | item, artifact, content |
| **Content Type** | A categorical classification of a content item: one of `skills`, `agents`, `mcp`, `rules`, `hooks`, `commands`, or `loadouts`. | type, category, kind |
| **Canonical Format** | Syllago's own provider-neutral intermediate representation of a content item, stored in the library; the form from which content is translated to a provider's native format on Install. | intermediate format, neutral format, syllago format |
| **Canonical Type** | A content type (`skills`, `agents`, `mcp`) whose canonical format is provider-neutral and can be installed to any provider that supports it. | universal type, provider-agnostic type, cross-provider type |
| **Provider-Specific Type** | A content type (`rules`, `hooks`, `commands`) keyed to a provider slug with format differences per provider. | native type |
| **DisplayName** | Human-readable label for a content item, sourced from frontmatter (`name:`) or `.syllago.yaml`; falls back to the directory name. | title, label |
| **Description** | One-sentence summary of a content item, extracted from frontmatter or the first non-heading line of the primary content file. | summary, blurb |

## Storage Layers

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Library** | The global, provider-neutral content store at `~/.syllago/content/` where all added content lives in canonical format, ready to install to any provider. | local content, user content, syllago content dir |
| **Project Content** | Content items scanned from `.syllago/`-adjacent shared directories within a specific project repo. | shared content, repo content |
| **Registry** | A git repository that distributes syllago content; cloned locally to `~/.syllago/registries/<name>/` and scanned as a catalog source. | remote registry, content registry |
| **Registry Clone** | The local git clone of a registry at `~/.syllago/registries/<name>/` used for catalog scanning and MOAT verification. | cached registry |
| **Catalog** | The merged, deduplicated, in-memory index of content items assembled from the library, project content, and all configured registries for a given scan. | content index, item list |
| **Source** | A string tag on a content item indicating its origin: `"library"`, `"project"`, `"global"`, or the registry name. | origin, provenance |

## Content Lifecycle (Verbs)

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Add** | Copy a content item from a provider or registry into the library, converting to canonical format. | import, copy-in, ingest |
| **Install** | Write a library item to a provider's config directory in native format, via symlink or copy. | export, deploy, push, publish |
| **Discover** | Scan a provider's config paths or a registry to enumerate available content without writing anything. | scan, probe |
| **Refresh** | Pull updated commits from all configured registries and apply content changes to the local library. | sync, update, pull |
| **Promote** | Copy a library item to the project's shared directory, commit to a branch, and open a pull request. | share, contribute |
| **Uninstall** | Remove a content item's symlink or JSON merge entry from a provider's config without touching the library copy. | remove, delete, detach |
| **Convert** | Render a content item from one provider's format to another without writing to the library or any provider config. | transform, translate |
| **Canonicalize** | Translate a provider-native content file into syllago's **Canonical Format** for library storage. | normalize, standardize |

## Provider Model

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Provider** | A supported AI coding tool (Claude Code, Cursor, Gemini CLI, etc.) with a stable slug, detected install paths, and format-specific install behavior. | agent, tool, client, harness |
| **Provider Slug** | The stable, lowercase, hyphenated identifier for a provider (e.g., `claude-code`, `gemini-cli`) used in directory names, CLI flags, and config keys. | provider name, provider ID |
| **Install Directory** | The filesystem path within a provider's config area where syllago places installed content items of a given type. | target directory |
| **JSON Merge** | The install mechanism for hooks and MCP configs: appends entries into a provider's JSON settings file rather than placing a file on the filesystem. | settings merge, config merge |

## Collections

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Loadout** | A named bundle of content item references (any mix of types) expressed in a `loadout.yaml` file, applied to a provider as a unit. | preset, bundle, collection |
| **Loadout Manifest** | The parsed representation of `loadout.yaml`, containing the loadout name, target provider(s), and typed lists of item references. | loadout config, loadout file |
| **ItemRef** | A reference to a content item within a loadout manifest, consisting of a name and an optional UUID. | loadout item, reference |
| **Snapshot** | A point-in-time backup of provider config files taken before a loadout is applied, enabling rollback if the apply fails. | backup, checkpoint |

## Registry & Trust

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Registry Manifest** | The `registry.yaml` at a registry root declaring its name, items, visibility, and optional MOAT signing metadata. | index, catalog file |
| **MOAT** | Model for Origin, Attestation, and Trust — syllago's supply-chain verification system using Sigstore/Rekor signatures to establish content provenance. | trust system, signing |
| **Trust Tier** | The normative classification of a content item's attestation state: Unknown, Unsigned, Signed, or Dual-Attested. | trust level, trust grade |
| **Trust Badge** | User-facing three-state collapse of trust tier: Verified, Revoked, or none. | trust indicator, trust label |
| **Dual-Attested** | The highest trust tier: both publisher (Sigstore keyless) and registry operator independently signed and recorded in Rekor. | fully signed, double-signed |
| **Signing Profile** | The issuer+subject tuple pinned at first-contact (TOFU) for a MOAT registry, compared on every sync to detect identity changes. | trust identity, pinned identity |
| **Privacy Gate** | A content-movement check preventing items sourced from private registries from being promoted or pushed to public destinations. | visibility gate |

## Metadata & Scan

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Item Metadata** | The `.syllago.yaml` file co-located with a content item directory, storing UUID, display name, description, source provenance, and tags. | syllago.yaml, dot-meta |
| **Source Provider** | The provider slug recorded in `.syllago.yaml` from which a content item was originally added. | origin provider, from provider |
| **Content Hash** | SHA-256 digest of a content item's primary file at add time, stored in `.syllago.yaml` and used to detect upstream changes on re-add. | file hash, source hash |
| **Precedence** | The priority rule applied when multiple catalog sources contain an item with the same name and type: library (0) > project (1) > registry (2) > built-in (3). | priority, rank |

## Provider Documentation

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Capability Document** | A per-provider, per-content-type YAML in `docs/provider-capabilities/` produced by capmon, recording proposed canonical key mappings with confidence levels. The machine-generated draft reviewed before graduating to a Provider Format Document. | capability YAML, cap doc |
| **Provider Format Document** | A per-provider YAML in `docs/provider-formats/` containing finalized, reviewed canonical key mappings for all supported content types; the authoritative source syllago uses when converting between canonical and native format. | format doc, provider schema |
| **Provider Source Manifest** | A per-provider YAML in `docs/provider-sources/` declaring documentation URLs, fetch tier, change detection method, and last-verified baseline; the input provmon reads for URL health and version drift checks. | source manifest, provider manifest |
| **Provider Reference Docs** | Human-readable Markdown files under `docs/providers/<slug>/` documenting a provider's content structure (file formats, schemas, loading behavior) for human consumption; not machine-parsed. | provider docs, provider notes |
| **Canonical Key** | A provider-neutral field name (e.g., `display_name`, `description`) that maps to different native field names per provider; the bridge between canonical format and native format during Install and Canonicalize. | field name, capability key |

## Monitoring Pipelines

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Capmon** | The four-stage capability monitoring pipeline (fetch → extract → diff → review) that scrapes provider documentation, extracts structured capability fields, detects drift from a stored baseline, and opens GitHub issues or PRs when capabilities change. | capability monitor, cap-mon |
| **Provmon** | The provider source monitoring pipeline that reads Provider Source Manifests, checks URL health via HTTP HEAD requests, and detects version drift via the GitHub Releases API or content hashing. | provider monitor, prov-mon |
| **Heal Pipeline** | Capmon's auto-repair subsystem that attempts to find valid replacement URLs when a source returns invalid or missing content, filing a GitHub issue if healing fails. | healer, self-heal |
| **Version Drift** | A provmon finding where a provider's latest released version differs from the baseline recorded in the Provider Source Manifest; triggers an update recommendation. | drift, version mismatch |

## Relationships

- A **Catalog** is assembled from one **Library**, zero or more **Project Content** sources, and zero or more **Registry Clones**.
- A **Content Item** belongs to exactly one **Content Type** and carries exactly one **Source** tag.
- A **Loadout** contains one or more **ItemRefs**; each **ItemRef** resolves to a **Content Item** in the **Catalog**.
- A **Registry Clone** is the local materialization of a **Registry**; it has exactly one **Registry Manifest**.
- A **Provider Format Document** is the reviewed and promoted form of one or more **Capability Documents** for that provider.
- A **Provider Source Manifest** is the input to **Provmon**; a **Capability Document** is the output of **Capmon**.

## Flagged Ambiguities

- **"install"** is used in two distinct senses: writing a library item to a provider config directory (**Install**, the content-lifecycle operation) vs. setting up the syllago CLI binary itself. Use **Install** for the former; use "set up" or "get started" in docs when referring to the CLI binary.

- **"shared"** overlaps with three concepts: the `--from shared` CLI flag (project content dirs), `ContentItem.Source == "global"` (library), and `Catalog.ByTypeShared()` (items that are not Library and not Registry). Use **Project Content** for project repo shared dirs, **Library** for `~/.syllago/content/`, and avoid "shared" in code comments where it would be ambiguous.

- **"source"** is overloaded in `metadata.Meta`: `Source` (origin label), `SourceProvider` (provider slug), `SourceType` (mechanism), `SourceRegistry` (registry name). Qualify which dimension you mean: **Source Type**, **Source Provider**, **Source Registry**.

- **"registry"** appears as both a Cobra subcommand group (`syllago registry sync`) and a content-source concept (`ContentItem.Registry`). `catalog.RegistrySource` (scan input) and `config.Registry` (persisted config entry) are different structs representing the same concept at different pipeline stages; do not conflate them.
