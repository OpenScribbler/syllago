# Rules Splitter & Monolithic-File Install — Design Decisions

**Status:** Working document — decisions captured during design dialogue, not yet implemented
**Date:** 2026-04-23
**Owner:** Holden Hewett
**Scope:** V1 ship-blocking feature — must be working before public release

---

## What we're building

A capability for syllago to:

1. Discover monolithic rule files (`CLAUDE.md`, `GEMINI.md`, `AGENTS.md`, `.cursorrules`, `.windsurfrules`, `.clinerules`, etc.) across project trees and home directories.
2. Split them into atomic rules using configurable heuristics, OR import them as a single rule, the user's choice.
3. Track provenance (project vs global, source path) as metadata.
4. Install rules as either individual files (existing model) or appended into a provider's monolithic file (new model).
5. Track every historical canonical version of each rule so that exact-match uninstall and update flows work without syllago "owning" content in the user's monolithic files.

The feature exists primarily to enable cross-provider portability of rules: take one section out of a monolithic `AGENTS.md` and install it into Claude Code as an individual `.claude/rules/*.md`, or vice versa.

---

## Decisions

### D1. Rules carry provenance metadata, not scope constraints

**Decision:** Library items get `sourceProvider`, `sourceScope` (`project` | `global`), and `sourcePath` metadata fields. These are informational only — they do not constrain where the rule can be installed.

**Why:** Users may legitimately want to install a project-discovered rule globally, or vice versa. Tracking *where the rule came from* is useful for the UI ("this came from `<repo>/CLAUDE.md`") without forcing constraints that hurt power users.

**Considered:**
- *Scope-binding*: enforce that project-scoped rules can only install at project scope. Rejected — too rigid, hides flexibility.
- *No tracking*: lose origin information. Rejected — user can't differentiate identically-named rules from different sources in the library or TUI.

**Implications:**
- Library schema gains three new fields.
- TUI Library and Content tabs surface scope as a column or badge.
- Discovery results in the Add wizard show full path so users can differentiate.

---

### D2. Discovery walks each directory for monolithic rule files

**Decision:** Discovery walks the project tree (stopping at `.git` boundaries) for the known monolithic filenames per provider, plus the user's home directory. Each match becomes a separate discovery candidate.

**Why:** Tools like Claude Code consume per-directory `CLAUDE.md` files (a project may legitimately have one at root, one in `cli/`, one in `cli/internal/tui/`, etc.). Restricting discovery to root-only would miss real-world content.

**Cost reality:** Searching for ~3-5 specific filenames per provider with `.git` boundary stops is cheap. Not a meaningful performance concern.

**Considered:**
- *Root-only*: simpler but misses per-directory files entirely.
- *Recursive without filename filter*: would read every file in the tree. Wasteful.

**Implications:**
- Discovery list can be long in monorepos. Prefix each item with full relative path for differentiation.
- The walker needs to know each provider's filenames — table lives in `cli/internal/provider/<provider>.go`.

---

### D3. Splitter heuristics: H2 default, H3/H4 optional, literal custom marker

**Decision:** Splitter offers three modes:

1. **By H2** (default) — split at every `##` heading.
2. **By H3 or H4** (opt-in) — for unusually deep files; H4 is the floor.
3. **By literal custom marker** — user types a literal string (no regex); splitter splits at every line that exactly matches.

`---` (horizontal rule) is **not** offered as a built-in option. Real-world files don't use it as a section divider, and false-positives from markdown tables and YAML fences make it dangerous as a default.

**Why:** Research across CLAUDE.md, AGENTS.md, GEMINI.md showed H2 is the universal section unit in the wild. H3 over-fragments in most cases but is needed for the rare deeply-nested file. Literal markers cover the edge case without regex complexity.

**Considered:**
- *Regex markers*: powerful but a foot-gun. Rejected for V1.
- *`---` as default*: zero of 23 sampled real-world files used standalone `---` as a section divider. Rejected.
- *AI-driven split as default*: conflates the deterministic core feature with an optional skill-based path.

**Implications:**
- Splitter contract: `Split(content, heuristic) -> []SplitCandidate{Name, Description, Body, OriginalRange}`.
- Same downstream pipeline regardless of which heuristic produced the candidates (and regardless of whether they came from the LLM-skill path described in D9).

---

### D4. Splitter behaviors derived from research

**Decision:** Splitter implements these behaviors:

- **Skip-splitting detection**: if the source file is fewer than 30 lines OR has fewer than 3 H2 headings, default to "import as single rule" with a one-line note. User can override.
- **Header promotion**: each split's H2 becomes an H1 in the resulting atomic rule, so the rule stands alone.
- **Numbered-prefix stripping**: `## 1. Coding Style` → slug `coding-style`, not `1-coding-style`. Original heading text is preserved in description.
- **Preamble handling**: text before the first split point is, by default, prepended to the body of the first split rule. User can override to "make preamble its own rule."
- **`@import` line preservation**: leave verbatim in the body of whichever split it falls into. Resolution is the consuming tool's job.

**Why:** Each behavior solves a real-world case observed in the research:
- Many AGENTS.md files are 1-line stubs that delegate to CLAUDE.md.
- The Cline ecosystem heavily uses `## 1. Foo` numbered patterns.
- Real CLAUDE.md / AGENTS.md / GEMINI.md files almost always have a 1-4 line preamble before the first H2.

**Implications:**
- The Add wizard's Review step needs an "as single rule vs split" choice when the heuristic returns the skip-split signal.
- The wizard's per-rule rename UI must show the slugified default and let the user override.
- **CLI flag for skip-split override:** `--split=single` imports the source file as a single rule, bypassing the splitter; this is the non-interactive equivalent of the wizard's "import as single rule" choice. When skip-split detection fires (file < 30 lines or fewer than 3 H2 headings) and the user did not pass `--split=single` (or any other explicit `--split=` value), the CLI errors out with the recommendation to either re-run with `--split=single` or pass an explicit heuristic to override the safety check. The implicit default ("use detected heuristic") is the wizard's behavior, not the CLI's, because non-interactive callers should be unambiguous about intent.

---

### D5. Append-to-end is the only monolithic-install method

**Decision:** When installing a rule into a provider's monolithic file (e.g., `~/CLAUDE.md`), syllago appends to the end of the file. No insert-after-marker, no in-place merging.

**Why:** Append is safe, predictable, and never corrupts user-authored content above. Insert-after-marker requires markers in the user's file and is fragile if the user edits the marker line.

**Considered:**
- *Insert-after-marker*: more flexible but requires user cooperation and degrades silently when markers move.
- *Beginning-of-file*: places syllago content above the user's preamble — confusing.
- *Smart placement* (e.g., insert before "## Conventions"): too provider-specific and fragile.

**Implications:**
- Install wizard offers two methods for monolithic-file providers: "as individual rule file" (existing) or "append to <monolithic file>" (new).
- The user retains full ownership of the monolithic file's structure; syllago's contribution is always at the end.

---

### D6. No syllago ownership of monolithic-file content

**Decision:** Once syllago appends a rule to a monolithic file, syllago does not "own" that content. There are no markers, no boundary metadata, no in-file annotations claiming ownership. The text becomes the user's text.

**Why:** Real users (including Holden) have other tools that mutate their `CLAUDE.md` and similar files. Syllago can't reliably maintain ownership claims when other tools are also writing. Better to be a one-shot importer than to make ownership claims that break.

**Considered:**
- *HTML comment markers* (`<!-- syllago:rule:foo --> ... <!-- syllago:end -->`): visible to users at least, but breaks if user edits inside markers or other tools strip comments.
- *Side-file ownership manifest* (record byte ranges or content hashes externally): brittle if user edits the file at all.
- *Both*: doubles the failure modes.

**Implications:**
- Uninstall via exact-match (D7) is the only mechanism for removing syllago-installed content from monolithic files.
- The Library "Installed" column reflects what's actually in the file, not what syllago thinks should be there (D8).

---

### D7. Exact-match uninstall against full version history

**Decision:** Uninstall searches the target monolithic file for the exact text of any historical version of the rule. If found, that block is removed. If not found (because the user edited it), the rule is treated as the user's content and left alone.

> The concrete byte pattern that uninstall searches for, the bytes that install writes, and the failure-mode policy when the search returns zero or multiple matches are specified in **D20** (Append byte contract). D7 establishes the principle; D20 specifies the implementation.

**Why:** Exact-match is the only safe semantic when syllago doesn't own the content. It draws a clean line: anything matching what syllago wrote is fair game; anything else is the user's.

**Considered:**
- *Fuzzy match* (e.g., 95% similarity): risks removing user-edited content the user wanted to keep. Rejected.
- *Match by section heading only*: too aggressive — user might have rewritten the body but kept the heading.
- *Refuse to uninstall from monolithic files*: makes the install method useless for iterative work.

**Implications:**
- Requires complete historical-version storage (D8).
- "Couldn't find this rule in the file — it may have been edited" is a normal, expected outcome with a clear user message.
- **Missing target file (ENOENT) on `syllago uninstall`:** the explicit uninstall command succeeds silently with the message "Note: target file already absent — install record dropped." No file mutation, no error. The user's intent ("make this not installed") is satisfied because there's nothing to clean up.
- **Unreadable target file on `syllago uninstall`:** the explicit uninstall command errors out with the OS error and leaves the install record intact. We refuse to mutate (or pretend to mutate) a file we can't read.
- **Scope of the above:** these two implications govern only the explicit `syllago uninstall` command. The verification scan (D16) and re-install path (D17) handle the same I/O errors differently — scan folds both into the `Modified` state for display, and re-install routes through the `Modified` flow with explicit recovery options. Three operations, three semantics on the same error: uninstall errors / drops, scan reports, re-install offers a choice. See D16 and D17 for the full state-machine behavior.

---

### D8. Per-rule canonical + history + original-source storage

**Decision:** Each library rule directory stores:

- The current canonical body
- All prior canonical versions (full text, not just hashes — exact-match needs the bytes)
- The original-format source file when source format ≠ canonical (for lossless same-provider install)
- Metadata indexing all of the above

The exact directory layout was an open question at the time of D8; **resolved by D11** (per-rule directory with `rule.md` + `.syllago.yaml` + `.source/` + `.history/`). D8 establishes *what* needs to be stored; D11 specifies the layout.

**Why:** Exact-match uninstall (D7) requires the actual byte content of every version syllago has ever written. Lossless same-provider install requires the original source bytes. Both are storage problems with the same shape: keep raw content alongside canonical content, indexed by metadata.

**Considered:**
- *Hash-only history*: cheaper but breaks exact-match uninstall.
- *Bounded history* (last N versions): may miss old installs the user is trying to remove.
- *Centralized history file* (one global YAML for all rules): conflates per-rule state with per-machine state, and history doesn't travel with the rule when shared/published.

**Implications:**
- Library directory size grows over time. Rule bodies are small (typically <5KB) so this is not a real concern in V1.
- All concrete storage questions (directory layout, file naming, version indexing) are answered downstream: layout and hash-keyed filenames in **D11**, the `versions:` list and `current_version` pointer in **D13**. D8 is the *what to store* decision; D11 and D13 are the *how* decisions.

---

### D9. LLM-driven split as a parallel path via syllago-meta-registry skill

**Decision:** Beyond the deterministic splitter, syllago supports an LLM-driven split via a skill distributed from `syllago-meta-registry`. The skill reads the source file, asks the user's LLM to propose a split, returns the structured result, and feeds it into the same Add wizard Review step that the deterministic splitter uses.

The skill is opt-in: users install it from the registry like any other content. If the skill is detected in the user's library, the Add wizard offers it as an alternative to the H2/marker split. If not detected, the wizard surfaces a one-line discoverability hint ("install `split-rules-llm` from syllago-meta-registry to enable AI splitting") with no nag.

**Why:** Some users will prefer determinism (H2/marker); others will prefer "let an LLM figure it out." The two paths share the same downstream pipeline (Review step → write candidates as library entries), so the only seam is in *how candidates are produced*.

**Considered:**
- *Built-in CLI subcommand calling an LLM*: couples syllago to a specific LLM API and reaches every user. Rejected — not syllago's job to ship an LLM dependency.
- *No LLM option*: leaves users with messy or unstructured rule files in a tough spot. Rejected — the use case is real.

**Implications:**
- The splitter contract must accommodate a `[]SplitCandidate` produced by anything (deterministic function or skill-driven flow).
- Discovery of the skill requires syllago to know how to detect a known skill in the user's library — likely by canonical name match.
- **Non-interactive CLI behavior:** the deterministic splitter runs by default. The LLM-skill discoverability hint is suppressed in non-interactive mode (no nag in scripts). Users opt into the LLM path by passing `--split=llm` explicitly; if the skill is not installed, the command errors out with a one-line install pointer ("install `split-rules-llm` from syllago-meta-registry: `syllago add split-rules-llm`").
- **V1 ship requirement:** the `split-rules-llm` skill is part of V1, not a follow-up. Both pieces must be done before V1 is callable — (1) the skill itself, authored under syllago-meta-registry, including its SKILL.md and the prompt/parsing logic that returns a structured `[]SplitCandidate`, and (2) the meta-registry publish step that makes `syllago add split-rules-llm` actually resolve. Without both, `--split=llm` lands in the binary as a dead flag, which is an unusable developer experience for the path syllago is explicitly recommending. The skill build is engineering scope captured in Phase 10 of the V1 plan.

---

### D10. Per-provider monolithic-install hints in the install wizard

**Decision:** The install wizard, when offering monolithic-file install for providers with strong conventions, surfaces a one-line hint about the provider's preferred pattern. Examples:

- **Codex**: "Codex prefers per-directory `AGENTS.md` files; consider installing per directory rather than as a single root file."
- **`.windsurfrules`**: "Windsurf has a 6KB limit on this file; the file rules format (`.windsurf/rules/`) is recommended for non-trivial content."

The hints are informational, not blocking. Users can still install however they want.

**Why:** Cross-provider portability is the whole point of the splitter. Refusing to split a Codex `AGENTS.md` would defeat that. But silently letting users install in ways that don't match a provider's expectations leaves them confused.

**Considered:**
- *Refuse* split for providers with hierarchical preferences. Rejected — defeats portability.
- *Silent default* with no hint. Rejected — users won't know they're going against the grain.

**Implications:**
- A small per-provider hints table lives in the provider package or splitter package.
- TUI displays hints as a non-blocking note in the install wizard's Review step.
- **Non-interactive CLI:** hints print to stderr as `NOTE:` lines before the install proceeds — they never block. Example: `NOTE: Windsurf has a 6KB limit on this file; the file rules format (.windsurf/rules/) is recommended for non-trivial content.` Users can suppress with `--quiet` if they want a clean stderr stream.

---

### D11. Library rule storage layout (resolves Q1)

**Decision:** Each library rule is a per-rule directory mirroring the hook pattern, with a fresh `.history/` subdirectory for prior canonical versions:

```
content/rules/<source-provider>/<name>/
  rule.md                    # current canonical (mirror of latest .history entry)
  .syllago.yaml              # metadata + chronological versions index
  .source/<original-file>    # original-format source bytes (when source format ≠ canonical)
  .history/
    sha256-<64-hex>.md       # one file per unique canonical version, including current
    sha256-<64-hex>.md
    ...
```

**Hash format (single canonical chain across all layers):**

| Layer | Form | Example |
|---|---|---|
| YAML `versions[].hash` | `<algo>:<hex>` (full hash) | `sha256:7a3f2c0e8d1b4f56...` (64 hex chars) |
| YAML `current_version` | `<algo>:<hex>` | `sha256:7a3f2c0e8d1b4f56...` |
| JSON `installed.json` `VersionHash` | `<algo>:<hex>` | `sha256:7a3f2c0e8d1b4f56...` |
| In-memory map key (`history[hash]`) | `<algo>:<hex>` | (same string, used directly as map key) |
| Filesystem filename | `<algo>-<hex>.md` | `.history/sha256-7a3f2c0e8d1b4f56...md` |
| Display (UI, log lines) | `<algo>:<12-hex>` | `sha256:7a3f2c0e8d1b` |

Two helper functions own the only conversions:
- `hashToFilename(hash string) string` — `:` → `-`, append `.md`. Input: `sha256:abc...`. Output: `sha256-abc....md`.
- `filenameToHash(name string) (string, error)` — strip `.md`, `-` → `:`. Input: `sha256-abc....md`. Output: `sha256:abc...`.

D16 / D17 / D14 all consume the canonical `<algo>:<hex>` form directly — no format juggling at call sites. The filesystem mapping is hidden behind those two helpers.

**Why each piece:**
- *Multi-file directory* (over single embedded YAML): keeps `.md` as `.md` so editors syntax-highlight, humans can `cat` any file directly, and binary/non-text source files embed cleanly. Stays consistent with the hook layout.
- *Hash-keyed history filenames*: identical content collapses to one file naturally (revert-to-prior reuses the same entry). Hashes are addressable; chronology lives in metadata.
- *Full sha256 (no truncation in storage)*: zero collision risk by construction. The "almost certainly unique" property of a truncated prefix is not the same as "guaranteed unique" — and the failure mode (one history entry silently overwriting another) is exactly the kind of subtle correctness bug that's expensive to debug. Display layer is the only place that truncates (12 hex chars after the algo prefix).
- *`<algo>:<hex>` (OCI-style) in serialized data*: self-describing (a future reader knows exactly what algorithm produced the digest), forward-compatible without schema migration if we ever add sha512/blake3, matches D13's existing convention. Filesystem uses `-` instead of `:` because Windows reserves `:` and library content travels via git to any platform.
- *Current version mirrored in `.history/`*: scan code has one code path — load every `.history/*.md` and try each against the target file. No special-casing of `rule.md`.
- *`rule.md` at the directory root*: human-friendly entry point; tools and IDEs see "the rule" without parsing metadata.

**Considered:**
- *Single embedded YAML* (`<name>.yaml` with bodies inline): atomic-update simplicity, but markdown-in-YAML loses editor highlighting, blocks binary source, and breaks parity with every other file-based content type.
- *Hybrid* (`rule.md` + history bodies inline in `.syllago.yaml`): splits the difference, but mixes per-rule state across two file types and bloats the YAML over time.
- *Timestamp-keyed history*: chronology obvious from `ls`, but no dedup on revert. Hybrid `<timestamp>-<hash>.md` adds info per filename but still no dedup.
- *Prior-only `.history/`* (current only at `rule.md`): less storage, but two scan code paths.

**Implications:**
- `.syllago.yaml` versions index needs `hash` + `written_at` per entry. Formal schema is **D13** (was Q2).
- Library directory size grows with version count, but rule bodies are small (<5KB typical) — not a concern for V1.
- Catalog scan over the library is unchanged — same per-directory walk pattern as today.
- **Load-time invariant (consistency check):** every `versions[].hash` in `.syllago.yaml` MUST have a corresponding `.history/<algo>-<hex>.md` file, and every `.history/*.md` file MUST have a matching `versions[].hash` entry. Either form of orphan is a load error with a fix-up suggestion ("missing history file" → "rebuild from another machine's library or remove the orphan entry"; "orphan history file" → "add a `versions:` entry or delete the file"). Cheap to check at scan time; eliminates a class of silent-corruption bugs.
- **Hash-format invariant:** the canonical `<algo>:<hex>` form is the only string syllago code passes around. Anywhere code touches the filesystem layer, it goes through `hashToFilename` / `filenameToHash`. No ad-hoc string formatting allowed. Linted by a small unit test that grep-fails the build if either pattern leaks: (a) `sha256-` outside `hashToFilename` (filename construction), or (b) `"sha256:" + ` string-concat outside the loader's format-validation entry point (canonical-hash construction). Both directions are equally easy to get wrong; the lint covers both.
- **In-memory representation:** `library[id].history` is `map[string][]byte` — the key is the canonical `<algo>:<hex>` string (same form as serialized data), the value is the normalized canonical body bytes loaded from `.history/<algo>-<hex>.md`. Population happens at catalog load time; the filesystem-to-canonical key conversion runs through `filenameToHash` once per file at load, never at lookup. Lookup at scan time (D16) is therefore a direct hash-string map access, not a filename construction.
- **Helper signature asymmetry:** `hashToFilename(hash string) string` has no error return because it operates on canonical hashes that have already passed validation (loader format check + caller-side type guarantees from the in-memory hash strings). `filenameToHash(name string) (string, error)` returns an error because filesystem entries can be malformed (externally created files, partial writes, manual mucking, FS corruption) and the caller — the loader — needs to react with a specific load error rather than crashing.

---

### D12. Canonical byte normalization for hash + scan

**Decision:** All bytes are normalized to a canonical form before being hashed for storage in `.history/` AND before being substring-searched against any monolithic target file at scan/uninstall time. Same normalization applied at both ends, so `bytes.Contains(normalize(target), normalize(version))` is the operative match.

**Normalization rules (applied):**
- CRLF → LF (cross-platform parity)
- Strip leading UTF-8 BOM (some editors add it silently)
- Exactly one trailing newline (POSIX expectation)

**Explicitly NOT normalized (would change semantics):**
- Trailing whitespace on a line (two trailing spaces = `<br>` in Markdown)
- Indentation (tabs vs spaces — semantic in code fences; user choice elsewhere)
- Unicode normalization (NFC/NFD) — visible-identical strings can differ; we don't "fix" user content
- Casing of headings, link refs, anything user-authored

**Why:** Without normalization, exact-match uninstall (D7) silently fails for any user whose monolithic file uses different line endings, has a BOM, or differs by a trailing newline. These differences are non-visual but byte-significant — exactly the trap that makes "hash never matches" feel mysterious.

**Considered:**
- *No normalization*: simpler but breaks uninstall on Windows / mixed-platform teams.
- *Aggressive normalization* (strip trailing whitespace, NFC): would silently mangle `<br>` line breaks and rewrite user-authored Unicode. Rejected.
- *Normalize at write only*: leaves the asymmetry where target file isn't normalized at scan, so match still fails.

**Implications:**
- A small `canonical.Normalize([]byte) []byte` helper lives in the splitter or rules package; called at write time and scan time.
- Test fixtures must cover: LF/CRLF parity, with/without final newline, with/without BOM, two-trailing-spaces line preserved, CRLF target containing LF-normalized rule (uninstall must find and remove it without disturbing surrounding CRLF content).
- Performance: normalization is a single linear pass over typically <50KB; cost is negligible. Hashing happens only at write time (filename addressing); scan time is normalize-once + substring-search-many, no hashing of target.

---

### D13. `.syllago.yaml` schema for rules (resolves Q2)

**Decision:** Library rules use the following `.syllago.yaml` schema. Source-related fields nest under a `source:` block; versions are a chronological list with an explicit `current_version` pointer.

```yaml
format_version: 1
id: 01HRX5K9ABCDEF                      # UUID, matches hooks
name: Coding Style                       # display name (user-editable)
description: Indent rules and naming     # short description (user-editable)
type: rule
added_at: 2026-04-23T14:30:00Z
added_by: holden                         # username or empty

source:
  provider: claude-code                  # source provider slug
  scope: project                         # project | global
  path: cli/CLAUDE.md                    # within-scope path; ~ prefix for home-relative
  format: claude-code                    # source format identifier
  filename: CLAUDE.md                    # original filename, mirrored at .source/<filename>
  hash: sha256:abc1230000000000000000000000000000000000000000000000000000000000  # full sha256 of original source bytes (immutable). truncated above for readability — real values are always 64 hex chars per D11
  split_method: h2                       # h2 | h3 | h4 | marker | single | llm
  split_from_section: "## Coding Style"  # heading this slice came from (omit for whole-file imports)
  captured_at: 2026-04-23T14:30:00Z      # when source bytes were captured to .source/

versions:
  - hash: sha256:7a3f2c0e8d1b4f56...     # always full 64-hex per D11 (shown abbreviated in this example)
    written_at: 2026-04-23T14:30:00Z
  - hash: sha256:9b1e442b0d6c8e91...
    written_at: 2026-04-25T09:12:00Z
  - hash: sha256:c0a8d1f47e2a3b85...
    written_at: 2026-05-02T16:00:00Z

current_version: sha256:c0a8d1f47e2a3b85...   # MUST be the full 64-hex form of an entry in versions[]
```

**Hash format:** All hash strings in `.syllago.yaml` use the canonical `<algo>:<hex>` form with the **full** 64-character hex digest (sha256 in V1). The example shows abbreviated values for readability only — the loader rejects any hash field that is not exactly `<algo>:<64-hex-chars>`. The full chain across all storage layers is documented in D11.

**Why nested `source:`:** Rules need ~9 source-related fields (provider, scope, path, format, filename, hash, split_method, split_from_section, captured_at). Flat `source_*` prefixes work for hooks (3 fields) but get noisy at this count. Nesting makes the per-rule provenance block self-contained and easier to read.

**Why explicit `current_version`:** Robust against accidental list reordering (manual edits, merges, future sort operations). Makes intent visible at the top level without scanning to the tail of the list. One extra invariant to enforce (`current_version` must exist in `versions[]`), but the cost is trivial and the safety win is real.

**Why we kept `versions:` ordered:** Chronology is real metadata. Order matters for the TUI's "version history" display and for any future "show me what changed between version A and B" feature. The list is implicitly append-only at write time; `current_version` lets us be explicit about which entry is the active one.

**Considered:**
- *Flat `source_*` matching hooks*: parity with hooks, but ~9 prefixed keys is hard to scan.
- *Hybrid (nest rules now, migrate hooks later)*: clean end-state but not worth the migration cost in V1; hooks at 3 source fields don't need it. Revisit later if a third content type with rich source metadata appears.
- *Implicit "last entry is current"*: one less field but fragile if list ever reordered.
- *Per-entry `current: true` flag*: distributes the invariant across entries; risk of zero or multiple `current: true` if YAML hand-edited.

**Implications:**
- Rules and hooks YAML schemas diverge intentionally. Both are valid. A future ADR may unify them, but not as a V1 blocker.
- The YAML loader (`internal/metadata` package) needs a `RuleMetadata` struct distinct from the existing `HookMetadata` — keep type-specific structs rather than a generic shape.
- The `current_version` invariant is enforced by the loader at parse time: missing or dangling pointer is a load error.
- `split_from_section` is an empty string for whole-file imports (where `split_method: single`).
- `captured_at` ≠ `added_at`: the latter is when the library entry was created, the former is when source bytes were captured. They equal each other on first add. If source is re-captured later (e.g., user re-adds the same rule from an updated source file), `captured_at` advances to the new capture time and the new source bytes overwrite `.source/<filename>`; `added_at` stays at the original creation time. The previous source bytes are not retained — `.source/` mirrors only the current capture, paralleling how `current_version` mirrors only the current canonical body. Users who need to compare old vs new source rely on git or backup, not syllago.

---

### D14. Install record-keeping for monolithic appends (resolves Q3)

**Decision:** A new entry type `InstalledRuleAppend` is added to the existing `Installed` struct in `cli/internal/installer/installed.go`. Records continue to live in the project-scoped `<projectRoot>/.syllago/installed.json` file, matching how hooks, MCP, and symlinks are tracked today.

```go
// InstalledRuleAppend records a rule appended to a provider's monolithic file.
type InstalledRuleAppend struct {
    Name        string    `json:"name"`            // display name (for UI/debug)
    LibraryID   string    `json:"libraryId"`       // UUID from .syllago.yaml
    Provider    string    `json:"provider"`        // claude-code, codex, gemini-cli
    TargetFile  string    `json:"targetFile"`      // abs path of monolithic file written to
    VersionHash string    `json:"versionHash"`     // canonical "<algo>:<hex>" form per D11 — always full 64-hex (e.g., "sha256:7a3f2c0e8d1b4f56789ab012cd34ef56789012345678abcdef0123456789abcd"); never truncated in storage
    Source      string    `json:"source"`          // "manual" or "loadout:<name>"
    Scope       string    `json:"scope,omitempty"` // "global" | "project"
    InstalledAt time.Time `json:"installedAt"`
}

type Installed struct {
    Hooks       []InstalledHook       `json:"hooks,omitempty"`
    MCP         []InstalledMCP        `json:"mcp,omitempty"`
    Symlinks    []InstalledSymlink    `json:"symlinks,omitempty"`
    RuleAppends []InstalledRuleAppend `json:"ruleAppends,omitempty"` // new
}
```

**Granularity:** One entry per `(LibraryID, TargetFile)` pair. **This is a hard invariant: no `RuleAppends[]` slice ever contains two entries with the same `(LibraryID, TargetFile)`.** Re-installing the same rule into the same target file does not silently overwrite — the install path always routes through D17's verification + decision step first (the TUI surfaces it via the `installUpdateModal` / `installModifiedModal` modals from D17 Cases A and B; the CLI surfaces it as the `--on-clean` / `--on-modified` flags; non-interactive runs without an applicable flag error out). `VersionHash` is updated only on a Replace decision (Clean state) or an Append-fresh decision (Modified state); Skip leaves `VersionHash` unchanged, and Drop-record removes the entry entirely. The library's `.history/` is the historical record; `installed.json` is current state.

The `--on-clean=append-duplicate` path was considered and explicitly cut from V1 (see D17 Case A "Why no append-duplicate option") — that was the only path that would have violated the uniqueness invariant. With it gone, `FindRuleAppend(libraryID, targetFile)` returns at most one match, and every `(LibraryID, TargetFile)` pair maps to at most one canonical-byte block in the target file.

**`Scope` field semantics:** Set at install time by the writer based on `TargetFile`'s location: `"global"` if `TargetFile` is under the user's home directory (e.g., `~/CLAUDE.md`), `"project"` if it is under the current project root (e.g., `<projectRoot>/CLAUDE.md`). Anything else (rare — explicit absolute path outside both) records as `"global"` with the absolute path preserved in `TargetFile`. The field is informational: consumed by the metadata panel's "Installed at: <scope>" line and by telemetry (`scope` enrichment). It does not gate verification or uninstall — those operate on `TargetFile` directly. The same `LibraryID` may have one record with `Scope: project` and one with `Scope: global` if the user installed it into both files.

**Why inherit the existing scope model (and not redesign install record storage now):**
- The current `installed.json` design records every install in `<projectRoot>/.syllago/installed.json`, including installs whose effect lands at global scope (e.g., a global hook in `~/.claude/settings.json` recorded under whichever project ran the install). RuleAppends adopting this same model means hooks, MCP, symlinks, and rule appends all behave the same way.
- Redesigning the scope model for rule appends alone would split install state across two truth systems within a single `installed.json` file. Redesigning it for all four entry types is a larger change with its own migration of existing user state — appropriate for a focused follow-up, not bundled into the rules splitter feature.
- The decision to inherit is a real design choice, not a deferral of investigation: we're committing to one consistent install record model in V1, and a separate ADR (D15) will design the broader scope-aware storage redesign.

**Considered:**
- *Split global vs project files*: would fix the quirk for rule appends but introduce inconsistency with the other three install record types. Two state files to keep in sync.
- *Fix the quirk everywhere now*: cleanest end state, but largest blast radius for V1 — every install/uninstall code path and a migration of existing user state.

**Implications:**
- The `Installed` struct grows one field; existing JSON files stay readable (new field is `omitempty`).
- A pair of helper methods on the existing `Installed` struct, matching the `FindHook` / `RemoveHook` pattern from `cli/internal/installer/installed.go`:
  ```go
  func (inst *Installed) FindRuleAppend(libraryID, targetFile string) int  // -1 if not found
  func (inst *Installed) RemoveRuleAppend(idx int)                          // splice out at index
  ```
- Library "Installed?" lookup is a two-step process per D16: `RuleAppends[]` is the source of truth for *which* (LibraryID, TargetFile, VersionHash) records to verify; the file scan in D16 determines *whether* the recorded `VersionHash` bytes are still present in `TargetFile` (state `Clean` vs `Modified`). Records that resolve to `Modified` show "Not Installed" in the column. Individual-file installs continue to be tracked via the existing `Symlinks[]` reverse-resolution by Target path.
- `LibraryID` is stable across rule edits because it lives in `.syllago.yaml`'s `id` field. Renames don't break the install record.

---

### D15. Scope-aware install record storage — separate follow-up

**Decision:** Designing scope-aware install record storage (so global-effect installs are tracked in a global file and project-effect installs in a project file) is its own work item. It applies to all four install record types (hooks, MCP, symlinks, rule appends) and is sized as a follow-up ADR, not part of the rules splitter scope.

**Why a separate work item:**
- The current model (everything in `<projectRoot>/.syllago/installed.json`, with per-entry `Scope`) covers correctness for installs and uninstalls run from the same project. Most real workflows do this naturally.
- The cases where it falls down (install from project A, uninstall from project B) require a larger redesign that touches every install/uninstall path across four record types and migrates existing user state. Bundling it into the rules splitter would expand the splitter's blast radius and delay V1.
- Splitting it out lets the splitter ship with a clean, consistent install record model in V1 and lets the storage redesign get the focused attention it deserves.

**Behavior in V1:**
- A user install run from `~/projectA` records in `~/projectA/.syllago/installed.json`, regardless of whether the install effect is global or project-scoped. Hooks, MCP, symlinks, and rule appends all behave this way.
- TUI Library "Installed?" column truth in V1 is bounded by which project the user has open in the TUI. The follow-up redesign will make global-effect installs visible across projects.

**Implications:**
- Open a separate bead for the scope-aware storage redesign. Do not block V1 on it.
- Document the V1 behavior in user-facing docs so users know to install/uninstall from the same project context.

---

### D16. Library "Installed" status check for monolithic appends (resolves Q4)

**Decision:** The Library column re-verifies install status on every catalog scan. `installed.json.RuleAppends[]` is the source of truth for *which* rules to verify against *which* target files; the file scan determines *whether* the bytes of the recorded version are still present. A rule's column is "Installed" only if both (a) a RuleAppend record exists for it and (b) the bytes of the version recorded in that RuleAppend (`r.VersionHash`) are present in `r.TargetFile`. Strictly the recorded version, not "any version we ever wrote" — that asymmetry would let column status drift away from the install record (see "Why strict match" below). The column is binary (Installed / Not Installed); a richer "modified" sub-state surfaces in the metadata panel and in action messages.

**Input → output contract:**

- **Input:** `installed.json.RuleAppends[]` (the set of (LibraryID, TargetFile, VersionHash) records).
- **Output:** `verification_state: (LibraryID, TargetFile) → (State, Reason)` where `State ∈ {Fresh, Clean, Modified}` and `Reason ∈ {none, edited, missing, unreadable}`. Plus an aggregate `match_set: LibraryID → []TargetFile` listing the targets that resolved to `Clean` (the column-summary projection).
- **States (CLI surface — these are first-class names every command path uses):**
  - **`Fresh`**: no RuleAppend record exists for `(LibraryID, TargetFile)`. Install proceeds without prompt. `Reason: none`.
  - **`Clean`**: record exists AND the recorded `VersionHash` bytes are present in `TargetFile`. Re-install routes to D17 Case A (Update flow). `Reason: none`.
  - **`Modified`**: record exists BUT the recorded `VersionHash` bytes are NOT present in `TargetFile`. Re-install routes to D17 Case B (Modified flow). `Reason` carries which kind of divergence: `edited` (file present, bytes don't match), `missing` (ENOENT — file deleted/renamed/branch-switched), `unreadable` (EACCES/EIO/etc.). Verification does not look at any other history version — see "Why strict match" below.
- The `Reason` field is presentation-only: the TUI modal and CLI prompt vary copy based on it, but the recovery options (`--on-modified` flag values) are identical across all three reasons. See D17 Case B for the variants.
- Output contains only hashes, no byte offsets — Replace re-runs the byte search at execute time per D20.

**Why strict match (recorded `VersionHash` only, not "any history version"):**
- The RuleAppend record is the install-time truth claim. Verification's job is to confirm or refute that claim, not to discover whatever-old-version-might-still-be-around.
- Eliminates cross-LibraryID identical-content collisions: if rule X v1 and rule Y v1 share bytes, only the rule whose record specifies that exact `VersionHash` matches against the bytes. The other rule's record points to a different hash; lookup is unique.
- Keeps record and reality in sync by construction. The lenient alternative ("any history version matches") lets the column report Installed against history v1 while the record still says v2, and a subsequent Replace then fails over to the Modified path because v2's bytes aren't actually there. The column would have been lying.
- The user-revert-to-older-version case (rare in practice) correctly falls into Modified, where D17 Case B's flow gives clean recovery: Drop the stale record, Append-fresh, or Skip. No silent acceptance of a state that diverges from what syllago wrote.

**Library column with multiple TargetFiles:** A single library rule may have RuleAppend records for multiple TargetFiles (e.g., `~/CLAUDE.md` and `<projectRoot>/CLAUDE.md`). The Library column is binary across the union: it shows Installed if at least one TargetFile in `match_set[LibraryID]` matched, Not Installed if zero matched. The metadata panel breaks the result down per target — listing each `(TargetFile, status)` pair — so the user can see which copies are clean, which are modified, and which are missing entirely. Uninstall and Replace actions in the modal pick the affected TargetFile explicitly; the column is the summary, the panel is the detail.

**Verification algorithm (per scan):**

```
# In-memory state used by the scan:
#   library[libraryID].history is map[string][]byte where key is canonical "<algo>:<hex>"
#       (same form as r.VersionHash) and value is the normalized canonical body bytes.
#       Population happens at catalog load time; lookup at scan time is a direct map access,
#       not a filesystem read or filename construction.
#   mtime_cache and per_target_state both carry (State, Reason) tuples per the D16 output contract.

mtime_cache:        targetFile        -> (mtime, size, per_target_state: {LibraryID: (State, Reason)})
verification_state: (LibraryID, targetFile) -> (State, Reason)   # one tuple per record
match_set:          LibraryID         -> []targetFile            # column-summary projection (Clean only)
warnings:           []string                                     # surfaced via stderr (CLI) / toast (TUI)

for each unique targetFile referenced by installed.json.RuleAppends:
    stat, statErr := os.Stat(targetFile)
    if statErr is fs.ErrNotExist:
        # ENOENT — file is gone. Mark every record for this target as Modified/missing.
        per_target_state = {r.LibraryID: (Modified, missing) for each RuleAppend r where r.TargetFile == targetFile}
        invalidate mtime_cache[targetFile]
        # No warning — ENOENT is a normal, expected state (user deleted, branch switch, etc.)
    elif statErr != nil:
        # Other I/O error (EACCES, EIO, broken symlink, etc.). State is unknown;
        # fold to Modified/unreadable so the column has a value, warn so the user can act.
        per_target_state = {r.LibraryID: (Modified, unreadable) for each RuleAppend r where r.TargetFile == targetFile}
        invalidate mtime_cache[targetFile]
        warnings.append(fmt.Sprintf("verify %s: %s", targetFile, statErr))
    elif mtime_cache[targetFile] is fresh (mtime+size match stat):
        per_target_state = mtime_cache[targetFile].per_target_state
    else:
        raw, readErr := os.ReadFile(targetFile)
        if readErr != nil:
            # Stat succeeded but read failed — race or transient I/O issue.
            per_target_state = {r.LibraryID: (Modified, unreadable) for each RuleAppend r where r.TargetFile == targetFile}
            invalidate mtime_cache[targetFile]
            warnings.append(fmt.Sprintf("verify %s: %s", targetFile, readErr))
        else:
            normalizedTarget = canonical.Normalize(raw)
            per_target_state = {}
            for each RuleAppend r where r.TargetFile == targetFile:
                recorded_body = library[r.LibraryID].history[r.VersionHash]   # direct hash-string lookup
                if recorded_body == nil:
                    # Defensive: orphan record (history file missing despite D11's load-time
                    # invariant). Should never happen in normal operation; treat as edited
                    # and warn so the user can investigate.
                    per_target_state[r.LibraryID] = (Modified, edited)
                    warnings.append(fmt.Sprintf("verify %s: orphan record for %s (no history file for %s)",
                        targetFile, r.LibraryID, r.VersionHash))
                    continue
                pattern = "\n" + canonical.Normalize(recorded_body)           # D20 byte contract: leading-\n invariant
                if bytes.Contains(normalizedTarget, pattern):
                    per_target_state[r.LibraryID] = (Clean, none)
                else:
                    per_target_state[r.LibraryID] = (Modified, edited)
            mtime_cache[targetFile] = (stat.mtime, stat.size, per_target_state)

    for libraryID, (state, reason) in per_target_state:
        verification_state[(libraryID, targetFile)] = (state, reason)
        if state == Clean:
            match_set[libraryID].append(targetFile)

for each library rule:
    column = match_set has rule.id ? Installed : NotInstalled   # any-target wins;
                                                                # rules with no RuleAppend records
                                                                # have State=Fresh, Reason=none implicitly
                                                                # (the loop above never visits them)

# Warnings are flushed to stderr (CLI) prefixed with "verify <path>: <error>" and
# surfaced as a toast (TUI) at scan completion.
```

A library rule with no RuleAppend records at all has state `Fresh` for every potential `TargetFile` (the loop above never visits it; the install command path treats absence-of-record as Fresh).

**Why fold ENOENT and read errors into `Modified` (rather than introducing new states):**
- The recovery options for "file is missing," "file is unreadable," and "file was edited" are the same set: Drop the stale record, Append-fresh (which D20's create-on-empty/missing path handles), or Keep. Same `--on-modified` flag values, same TUI modal options.
- The only legitimate UX difference is the modal's first line ("the file no longer exists" vs "no longer contains the recorded version" vs "couldn't read the file"). That's a presentation-layer concern, not a state-machine one. The TUI varies modal copy based on a `verification_state.reason` field carried alongside the state; the CLI prints the same variations in its prompt/error text.
- Holding the state count at three keeps the install command path's branch logic small and the test matrix manageable.

**Why no auto-drop on ENOENT:**
- A user who ran `git checkout` on a branch without the file would lose every install record silently. Equally, an accidental `rm` would erase the user's record of what was installed before they noticed. Both are non-recoverable without backup.
- Persisting the record and surfacing it through the normal Modified flow gives the user explicit agency: Drop, Append-fresh, or Keep. Matches D17's "explicit decision for every state-changing action" principle.
- Stale records accumulating is a real but bounded concern. The Library "Installed" column showing Not Installed (because state is Modified) is the discoverability path: user notices, opens the rule, picks Drop in the modal.

**Why re-verify on every scan:**
- Matches the existing pattern for filesystem-installed content (rules-as-files, skills, agents): truth lives on disk, not in a record file. This keeps the install-status mental model consistent across content types.
- Catches the cases install records can't catch: user manually deleted the syllago-installed text, another tool overwrote the target file, user edited the appended block.
- Cost is genuinely small: with strict match the inner per-target work is one substring search per RuleAppend record (no history iteration). For realistic libraries (~300 RuleAppend records across ~3 unique target files of ~30KB each), well under 100ms cold; ~0ms warm thanks to mtime caching.

**Why mtime cache (not always re-read):**
- Repeat scans (R-key refresh) hit the cache and skip both file I/O and substring search.
- Same memoization shape used at `cli/internal/moat/enrich_verify_cache.go`. Cache key is (path, mtime, size); persistent across rescans within a TUI session, dropped on TUI exit.
- Process-local only — no on-disk cache file (matches the MOAT precedent of rejecting persistent cache for local state).

**Why binary column with rich metadata panel:**
- Three-state columns add a state ("modified") that has no obvious action: the user can't install or uninstall a "modified" rule in the usual sense — they can only edit the file or remove the install record.
- The information about modification is preserved in the verification result and surfaces where the user can act on it: in the metadata panel when they select the rule, and in action messages ("Uninstall will be a no-op — file no longer contains the recorded version").
- Binary column keeps the visual scan of the Library list clean.

**Considered:**
- *Trust install record only*: fastest, but column lies whenever the user edits the target file. Uninstall would silently fail or no-op without warning.
- *Trust record for column, verify on demand*: split-truth model where the column may be wrong while feeling fast. Discoverability of the wrong-ness is bad — users only learn during uninstall.
- *Three-state column*: more information per row, but the third state has no clear user action.

**Implications:**
- A new `install_check` (or similar) module owns the mtime cache and the per-target verification loop. Lives near the installer or catalog package.
- The cache is a `sync.Map` (or equivalent) keyed by absolute target file path. Invalidated on every install/uninstall that touches that target.
- Verification uses the canonical normalization helper from D12 — both target file bytes and history version bytes go through `Normalize()` before substring search.
- The metadata panel needs a new "Installed at" / "Status" subsection for rules with a RuleAppend record. The "modified" status string lives there.
- A future enhancement (post-V1) can add a tertiary column state if usage shows users want it. The data is already collected by the verification loop.

---

### D17. Update flow UX for monolithic-rule re-install (resolves Q5)

**Decision:** Re-installs of a rule that has an existing RuleAppend record always require an explicit decision — never silently mutate a target file. The decision logic lives at the CLI / library layer (the install command path) and consumes D16's `verification_state`. The TUI surfaces it as a modal; the CLI surfaces it as a flag (or errors when non-interactive without a flag). Two distinct decision shapes depending on which state D16 returned (Clean vs Modified).

**Working assumption (foundational):** Every action that would change a target file goes through an explicit decision. Target files (CLAUDE.md, AGENTS.md, etc.) are personally meaningful to users, and silent mutation is the worst-case failure mode for this feature. "Explicit decision" means: TUI shows a modal with a default action, CLI requires a flag (or non-interactive errors out). The default-action and flag values map 1:1 across the two surfaces.

#### Case A: Update flow — D16 state `Clean` (RuleAppend record exists AND the recorded version's bytes are present in the target file)

TUI modal:

```
This rule is already installed at:
  <TargetFile> (version <recorded-short-hash>)
Current library version is <new-short-hash>.

> Replace with current version            [Enter]
  Skip (leave file unchanged)             [Esc]
```

CLI equivalents (both map to the same two actions; see "Implications" for the full flag table):
- `--on-clean=replace` (TUI default: Replace)
- `--on-clean=skip`

**Default: Replace.** The user invoked install — their intent is to have the latest version installed.

**Why no append-duplicate option:** An earlier draft included `--on-clean=append-duplicate` as a power-user escape (append a second copy of the same rule body to the file). Cut from V1 because it is the only path that violates D14's `(LibraryID, TargetFile)` uniqueness invariant, creates a deferred specification gap (record shape under multiple installs of the same content), and produces a silent-failure mode on subsequent uninstall (`bytes.Contains` returns ≥2 matches, falls to D17 Modified flow with no clear recovery action). No real user has asked for duplicate installs of the same rule into the same file; if the use case emerges, it can be re-introduced with full record-shape semantics specified up front.

**What Replace does:** Per the byte contract in D20:
1. Locate the recorded version's exact canonical bytes in the target file (look up `r.VersionHash` → `.history/` + `hashToFilename(r.VersionHash)` per D11 → read → search for the byte sequence `\n + canonical bytes`).
2. Splice in `\n<new canonical content>` at the same offset. The leading `\n` is part of the matched range removed in step 1 (per D20's leading-`\n` invariant), so writing `\n<new>` preserves the separator without introducing or losing one.
3. Update the `RuleAppend` record's `VersionHash` to the new version.

(D20's contract makes Replace an in-place splice, not a remove-then-append-at-end. The new content lands where the old content was, with the same surrounding whitespace.)

**Why Replace as default (not Skip):**
- Skip-as-default makes update a 2-step process (open modal, press [r]) for the most common case; users will quickly find that frustrating.
- Replace-as-default still requires explicit confirmation (Enter); zero file mutation is silent.

#### Case B: Modified flow — D16 state `Modified` (RuleAppend record exists BUT the recorded version's bytes are NOT present in the target file)

Reachable via four real-world paths, all surfaced through the same state and recovery flow:
1. **Edited:** user (or another tool) edited the previously-appended block in place.
2. **Rolled back:** user manually rolled the file back to a different (older or unrelated) state.
3. **Missing (ENOENT):** target file no longer exists (deleted, renamed, branch-switched).
4. **Unreadable:** stat or read failed with EACCES, EIO, or other I/O error.

The state is the same; the modal/CLI prompt copy varies on a `reason` field that D16 carries alongside the state (`edited` | `missing` | `unreadable` — paths 1 and 2 both surface as `edited`).

The same three options are offered for all three reasons; the **default action depends on the reason** so that the conservative path is taken when state is unknown:

| Reason | Modal default (`[Enter]`) | Why |
|---|---|---|
| `edited` | Drop stale record | File contents are observed to diverge from the record — record is provably stale. |
| `missing` | Drop stale record | File is observed to not exist — record is provably stale. |
| `unreadable` | Keep (leave record + file unchanged) | We don't know what's in the file. Don't make a state-changing decision when state is unknown; user investigates and re-invokes. |

TUI modal copy (varies by reason; default action moves with the reason):

```
[reason: edited]
This rule was installed at:
  <TargetFile>
...but the file no longer contains the version we recorded.
Either the appended block was edited, or the file was rolled back.

> Drop stale install record (no file change)  [Enter]
  Append current version as a fresh copy      [a]
  Skip (leave record + file unchanged)        [Esc]

[reason: missing]
This rule was installed at:
  <TargetFile>
...but that file no longer exists. It may have been deleted, renamed, or
the project may have been moved.

> Drop stale install record (no file change)  [Enter]
  Append current version as a fresh copy      [a]   # creates the file per D20
  Skip (leave record + file unchanged)        [Esc]

[reason: unreadable]
This rule was installed at:
  <TargetFile>
...but we couldn't read the file: <error>.
Check permissions and whether the path is still accessible, then re-run.

> Keep (leave record + file unchanged)        [Enter]
  Drop stale install record (no file change)  [d]
  Append current version as a fresh copy      [a]
```

CLI equivalents (one flag, three values, same semantics across all reasons; only the *interactive default* varies by reason):
- `--on-modified=drop-record` (interactive default for `edited` and `missing`)
- `--on-modified=keep` (interactive default for `unreadable`)
- `--on-modified=append-fresh` (creates the file per D20 if it's missing)

Non-interactive CLI prints the reason-appropriate text before erroring on missing flag, so scripted callers can branch on the message if they want. The non-interactive error message is the same for all three reasons (`"rule install record is stale; specify --on-modified=drop-record|append-fresh|keep"`) — non-interactive callers must be explicit, regardless of which reason fired.

**Why the split default (Drop for `edited`/`missing`, Keep for `unreadable`):**
- For `edited` and `missing`, syllago has *observed* that the file's reality diverges from the record. The record is provably stale; dropping it makes Library state truthful again per D16's invariant ("the column is the truth").
- For `unreadable`, syllago has *not* observed the file's contents — only an I/O error. Dropping the record asserts a state we couldn't verify. Defaulting to Keep preserves the record until the user fixes the underlying issue (permissions, broken symlink, mounted-filesystem hiccup) and re-invokes; on the next scan, the same record will resolve to either `Clean` or `Modified/edited` and the user gets a real signal.
- Drop and Append-fresh remain available as explicit choices for `unreadable` — users who know the file is genuinely gone can still take the destructive action; we just don't put it under `[Enter]`.

**What "Drop stale install record" does (when chosen):**
1. No file mutation.
2. Drop the matching `RuleAppend` entry from `installed.json`.
3. Library "Installed" column for this rule + this target now shows Not Installed (matches reality per D16 for `edited`/`missing`; for `unreadable`, this is an explicit user-asserted state since syllago couldn't verify).

**Why Drop is the right default for `edited`:**
- The user's edited content is theirs to manage. We don't know if their edit was minor (whitespace fix) or substantive (rewrote half the rule), so we can't safely deduplicate.
- Dropping the stale record makes the Library state truthful again.
- "Append fresh" is offered as an explicit option for users who want a clean re-install regardless.
- "Refuse + require manual cleanup" was rejected: it adds steps without protecting anything the default doesn't already protect.

**Considered (Case A — Update flow):**
- *Default Skip*: safest by file-mutation count, but slowest path for the common "I want to update" use case.
- *Default Append*: never-removes guarantee, but accumulates duplicates.

**Considered (Case B — Modified flow):**
- *Default Append fresh + warn*: treats modification as "install lost, reinstall from scratch." Risks duplicating content the user kept.
- *Refuse + require manual cleanup*: most conservative, most steps. Doesn't earn the friction.

**Implications:**
- The decision logic lives in the install command path (used by both CLI and TUI). It runs D16's verification, reads the resulting `verification_state` for `(LibraryID, TargetFile)`, then either proceeds (Fresh), routes to Case A (Clean), or routes to Case B (Modified).
- TUI surface: two modals — `installUpdateModal` (Case A) and `installModifiedModal` (Case B), following existing modal patterns (`internal/tui/modal.go`). Each modal's options correspond 1:1 to the CLI flag values for that state.
- CLI flag table (interactive defaults match the TUI modal defaults; non-interactive without an applicable flag errors out):

  | D16 state | Reason | Flag | Values | Interactive default | Non-interactive without flag |
  |-----------|--------|------|--------|---------------------|------------------------------|
  | `Fresh`   | `none` | none | n/a    | proceed             | proceed                      |
  | `Clean`   | `none` | `--on-clean=...` | `replace` \| `skip` | Replace | error: "rule already installed at clean state; specify --on-clean=replace\|skip" |
  | `Modified`| `edited` | `--on-modified=...` | `drop-record` \| `append-fresh` \| `keep` | Drop record | error: "rule install record is stale; specify --on-modified=drop-record\|append-fresh\|keep" |
  | `Modified`| `missing` | `--on-modified=...` | (same three values) | Drop record | (same error) |
  | `Modified`| `unreadable` | `--on-modified=...` | (same three values) | **Keep** | (same error) |

  Same flag, same value set, same non-interactive error — only the *interactive default* shifts to `Keep` when reason is `unreadable`. The conservative default reflects that syllago has not observed the file's contents and should not assert a state-changing decision.

  *(Optional ergonomic shortcuts the implementation may expose alongside the canonical flags: `--update` ≡ `--on-clean=replace`, `--skip-if-installed` ≡ `--on-clean=skip`. These are nice-to-haves, not part of the contract — the canonical CLI surface is `--on-<state>=<action>`. Neither alias exists in syllago today; they would be introduced as new ergonomic shortcuts if implementation chooses, not preserved from any prior surface.)*
- The "remove block in-place" operation in Case A's Replace path follows the byte contract in D20: re-run the canonical-byte search at execute time using the recorded `r.VersionHash` to look up the history file via `hashToFilename(r.VersionHash)` per D11, then splice. Re-search is O(file_size) which is microseconds at our scale; no offset/length sidecar is tracked.
- Telemetry (per `.claude/rules/telemetry-enrichment.md`): enrich install events with `verification_state` ("fresh" | "clean" | "modified") and `decision_action` ("proceed" | "replace" | "skip" | "drop_record" | "append_fresh" | "keep"). Add both to `EventCatalog()`.

---

### D18. Discovery list display + multi-select for batched import (resolves Q6)

**Decision:** The Add wizard's discovery step shows monolithic-file candidates as a **flat list sorted by path**, with each row tagged `[project]` or `[global]` and an inline `✓ in library` indicator when the file's bytes match an existing library rule's `.source/`. The list supports **multi-select** via spacebar, with Enter confirming the selected set.

**Discovery list visual contract:**

```
Add Rules — Discovery (12 found, 3 selected)
┌──────────────────────────────────────────────────────
  ./CLAUDE.md                  42L  7H2  [project]
  ./apps/api/CLAUDE.md         58L  4H2  [project]   ◉
  ./apps/web/CLAUDE.md        120L  9H2  [project]   ◉
  ./packages/shared/CLAUDE.md  30L  3H2  [project]
  ./services/auth/AGENTS.md   118L 11H3  [project]
> ./services/billing/CLAUDE.md 88L  6H2  [project] ✓ in library
  ~/CLAUDE.md                 200L 12H2  [global]    ◉
  ~/AGENTS.md                  15L  0H2  [global]
└──────────────────────────────────────────────────────
j/k navigate · space toggle · / filter · enter confirm · esc back
```

Per-row data: relative path (with `~/` prefix for global), filename, line count, heading count (matching the chosen heuristic when set; H2 default), scope tag, in-library indicator, selection marker.

**Why flat sorted (not tree, not two-section):**
- Hierarchy is implicit in the path string already; a separate tree adds rendering and expand/collapse state without revealing more information than the path does.
- The two-section split (Project / Global) is a degenerate version of grouping that adds visual weight without solving the monorepo-with-many-subdirs case.
- Existing TUI list patterns (sortable columns, type-to-filter, j/k navigation) carry over with no new conventions.

**Why multi-select (not single-select):**
- Real monorepo workflow: user wants to import all per-package CLAUDE.md files in one go. Single-select would force 5+ wizard passes.
- Cost of the choice is concentrated in one place: the Review step has to handle N source files. Once that's built, every other downstream piece (storage, install) is per-rule and unchanged.
- The selection model uses spacebar (toggle) + Enter (confirm) — the same convention used elsewhere in the existing TUI for any list that supports multi-pick.

**Considered:**
- *Grouped tree by directory*: better for very large monorepos (>30 candidates) but over-engineered for the typical 1–5 case. Tree expand/collapse is wizard state we don't need.
- *Two-section Project/Global split*: doesn't solve the monorepo-many-subdirs case and adds vertical space.
- *Single-select*: simpler downstream Review step but punishes the bulk-import workflow we explicitly want to support per D2.

**Implications for the Review step (downstream of D18):**

Multi-select changes the Review step's shape. It must now display candidates from N source files. Options for that display, to settle in implementation:
- Tabbed-by-source: top tab strip, one tab per source file, candidates from that file in the active tab.
- Grouped-flat: one scrollable list with `── apps/web/CLAUDE.md ──` separator headers between groups.

Both are viable; tabbed is better for big batches, grouped-flat is cleaner for small batches. Implementation can pick whichever fits the existing wizard's modal/list scaffolding more naturally — call out as a sub-decision in the wizard PR description rather than gating the design doc on it.

**Heuristic-step contract under multi-select:**
- The heuristic (H2 / H3 / H4 / marker / single, per D3) is chosen once for the whole batch.
- The skip-split detection from D4 still applies per file: any file under 30 lines or with fewer than 3 H2 headings auto-switches to "import as single rule" regardless of the chosen heuristic, and the Review step labels those files visibly so the user can override per-file if they want.

**CLI form for batched import (non-interactive equivalent of multi-select):**

```
syllago add --from <file1> [--from <file2> ...] --split=<h2|h3|h4|marker:<literal>|single|llm>
```

- The `--from` flag is repeatable; each value is a path to a monolithic source file (relative or absolute). Order does not matter.
- Discovery is *not* invoked in this form — the user has already chosen the source files explicitly. The walker is reserved for the interactive (TUI) path where discovery is the value-add.
- The heuristic chosen via `--split` applies to the whole batch (matching the wizard's batch-wide choice). Per-file heuristic overrides are not supported in V1; if the user needs different heuristics per file, they invoke the command once per file.
- Skip-split detection (D4) applies per file. If it fires for any file in the batch and the user did not pass `--split=single`, the CLI errors with a list of the affected files and a suggestion to either re-run with `--split=single` or split each file individually.
- The Review step has no non-interactive equivalent — non-interactive batch import accepts the splitter output as-is. Renaming and per-rule edits require the wizard.

**Other implications:**
- Discovery walk per D2 already returns a flat list of candidates; no walker changes needed.
- "✓ in library" check is a hash comparison against each library rule's `source.hash` from D13. O(N candidates × M library rules); both small.
- The Add wizard step machine (per `.claude/rules/tui-wizard-patterns.md`) gains a multi-select state on the Discovery step. The `validateStep()` method must enforce: "Heuristic step entered with at least one selected candidate."
- Mouse parity (per `.claude/rules/tui-wizard-patterns.md` and `feedback_mouse_parity.md`): each row needs a `zone.Mark` for click-to-toggle; the spacebar toggle behavior must have a click equivalent.
- Telemetry: enrich the add event with `discovery_candidate_count` (int) and `selected_count` (int). Add both to `EventCatalog()`.

---

### D19. Splitter test fixtures: synthesized-only, structurally modeled (resolves Q7)

**Decision:** All splitter test fixtures are synthesized (hand-authored, no verbatim third-party content). Each fixture is **structurally modeled** after a specific real-world reference from the inventory — the structure (heading depth, line count band, frontmatter usage, `---` placement, numbered-prefix pattern, etc.) mimics the reference, but the content is entirely fabricated.

**Fixture directory layout:**

```
cli/internal/converter/testdata/splitter/
  # Behavior-specific synthesized fixtures (named for what they exercise)
  h2-clean.md                 # 3 H2s, no preamble, no nesting
  h2-with-preamble.md         # 2-line preamble before first H2
  h2-numbered-prefix.md       # "## 1. Foo" / "## 2. Bar" patterns
  h2-emoji-prefix.md          # "## 🚀 Foo" patterns (slug normalization)
  h3-deep.md                  # forces H3 splitting (3 H2 / 11 H3 shape)
  h4-rare.md                  # H4 splitting case
  marker-literal.md           # custom literal-marker split fixture
  too-small.md                # < 30 lines (skip-split trigger)
  no-h2.md                    # 0 H2s (skip-split trigger)
  delegating-stub.md          # 1-line file pointing to another rules file
  table-heavy.md              # markdown tables to stress preamble + section logic
  decorative-hr.md            # standalone --- as decoration (verifies we don't split on it)
  must-should-may.md          # mandate-language fixture (preserve casing)
  trailing-whitespace.md      # two-trailing-spaces line preserved through normalization
  crlf-line-endings.md        # CRLF normalization fixture
  bom-prefix.md               # UTF-8 BOM stripping fixture
  no-trailing-newline.md      # missing final newline normalization fixture
  import-line.md              # @import line preservation fixture

  # Format-specific fixtures (replicate the wire format quirks)
  cursorrules-flat-numbered.md
  cursorrules-points-elsewhere.md
  clinerules-numbered-h2.md
  windsurfrules-pointer.md
  windsurfrules-numbered-rules.md

  REFERENCES.md               # inventory pointer: which real file inspired each fixture
```

**REFERENCES.md contents:** Two-column table mapping each fixture filename to the real-world reference it was structurally modeled after (URL + brief shape note). Purely informational — no licensed content is committed, so no formal attribution is required, but the reference trail helps a future developer understand *why* a given fixture exists and verify against the inspiration.

```markdown
# Splitter fixture references

These fixtures are synthesized (content fabricated). Their *structural shape*
is modeled after the real-world references below for coverage traceability.
No third-party content is committed.

## CLAUDE.md / AGENTS.md / GEMINI.md shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| h2-clean.md | github.com/saaspegasus/pegasus-docs | ~45L, 7 H2s, no preamble |
| h3-deep.md | github.com/payloadcms/payload | ~330L, 9 H2 + H3 nesting, trailing --- |
| table-heavy.md | github.com/DataDog/lading | tables-in-content stress |
| ... | ... | ... |
```

**Why synthesized-only (and not the verbatim hybrid I initially recommended):**
- No licensing burden — every byte is fabricated.
- No attribution maintenance — the REFERENCES.md is informational, not legally required.
- No stale-fixture risk from upstream deletions, repo moves, or unannounced re-licensing.
- Smallest repo growth.
- The splitter cares about *structure*, not *content* — fabricated content with the same structural shape exercises the same code paths as the verbatim original.
- The reference trail in REFERENCES.md preserves the "why does this fixture exist" knowledge without committing third-party bytes.

**Considered:**
- *Hybrid (synthesized + small verbatim set)*: still incurred license check + attribution maintenance for the verbatim portion. The splitter doesn't need real bytes to test correctly, so hybrid is hybrid for no actual technical benefit.
- *Verbatim full inventory*: maximum real-world coverage; maximum maintenance and license burden; ~1–2 MB repo growth.

**Implications:**
- Fixtures live at `cli/internal/converter/testdata/splitter/`. Tests load them via standard Go `testdata/` relative paths.
- Each fixture's first line may be a `<!-- modeled after: <ref> -->` HTML comment so that opening any fixture file shows its inspiration without needing to cross-reference REFERENCES.md. Splitter must treat HTML comments as content (which it already does — they pass through normalization unchanged and the splitter only acts on heading lines).
- Authoring each fixture is a small task: open the real reference, observe its shape, write a fixture with the same structure but fabricated text. Captured as part of the splitter implementation work in V1 phase 2.
- The doc's existing "Test Fixtures" inventory section (further below) becomes the authoring guide: each entry's "modeled after" attribute drives one synthesized fixture.

---

### D20. Append byte contract — no markers, canonical-byte block matching

**Decision:** Monolithic-file install, uninstall, and replace operate on canonical bytes only. Syllago writes no markers, comments, or boundary metadata into the user's monolithic file. Block location is performed by searching the target file for the exact canonical bytes of a known history version. Imperfect matches do not trigger recovery logic — they fall through to D17's Modified path.

This decision was reached after live research into prior art (Ansible `blockinfile`, SaltStack `file.blockreplace`, Debian `ucf`, remark/unified mdast, Augeas). The marker pattern dominates that prior art, but for our specific domain (small markdown files read by an LLM on every turn) the trade-offs invert. D6 already established the principle (no in-file ownership claims); D20 specifies the concrete byte contract that implements that principle.

**Append contract:**

1. If the target file does not exist or is empty, create it containing `\n<canonical content>`.
2. Otherwise, ensure the target file ends with `\n` (add one if missing), then append `\n<canonical content>`.
3. `<canonical content>` is the bytes of the matching history version (loaded via `.history/` + `hashToFilename(version_hash)` per D11), already normalized per D12 (single trailing `\n` guaranteed).

Net effect on a non-empty file: one blank line of visual separation between prior content and the appended rule, with the rule ending on its own newline.

**Why the always-leading `\n` (including on empty files):** The byte pattern that uninstall and Replace search for is exactly `\n<canonical content>`. Making the leading `\n` invariant — present on every appended block regardless of file state — means the search pattern is single-shape across all cases. The trade-off is one leading blank line on a freshly-created file. We accepted this because (a) the alternative (no leading `\n` on empty files, leading `\n` everywhere else) splits the search into two byte patterns and doubles the cases the test suite must cover, and (b) freshly-created monolithic files are the rare path — the common case is appending to an existing user-authored file where the leading blank line is the intended separator.

**Uninstall / Replace search:**

1. Read the target file. Look up the version's canonical bytes from the on-disk version directory (`<library-id>/.history/` + `hashToFilename(version_hash)` per D11).
2. Search the target file for the exact byte sequence `\n<canonical content>` (the same byte pattern the Append contract writes — leading `\n` included).
3. **Found exactly once:** that byte range is the block, leading `\n` and all. For uninstall, remove the entire matched range; the byte that previously preceded the leading `\n` is now adjacent to the byte that previously followed the rule's trailing `\n`. For Replace, splice in `\n<new canonical content>` at the same offset (no separator manipulation needed — the surrounding context already has correct whitespace).
4. **Found zero times:** fall to D17's Modified flow (D16 state `Modified`) — surface the explicit decision step (TUI modal / CLI `--on-modified`), do not guess.
5. **Found multiple times:** fall to D17's Modified flow — surface the explicit decision step, do not guess which copy to remove.

**Mid-file uninstall correctness (worked example):** Suppose the user's preamble is `P\n` and the user has installed three rules sequentially with bodies `r1`, `r2`, `r3` (each canonical body ending in `\n` per D12). The file on disk is `P\n\nr1\n\nr2\n\nr3\n`. Each `\n\n` is one blank line of visual separation; the leading `\n` of every appended block is the separator we wrote, the trailing `\n` is the canonical content's own.

Uninstalling `r2` searches for the pattern `\nr2\n`. It matches once, between `r1\n` and `\nr3\n`. Removing the matched range leaves `P\n\nr1\n\nr3\n` — exactly the byte sequence we'd have produced had `r2` never been installed. Uninstalling `r1` from the original three-rule file yields `P\n\nr2\n\nr3\n`. Uninstalling `r3` yields `P\n\nr1\n\nr2\n`. Uninstalling all three in any order yields `P\n`. The leading-`\n` invariant in the search pattern is what makes this work without per-position special cases.

**Multiple-match scenario:** Syllago's own install paths cannot produce multiple matches — D14's `(LibraryID, TargetFile)` uniqueness invariant ensures each rule has at most one canonical-byte block per target file, and the `--on-clean=append-duplicate` escape that would have violated it was cut from V1 (see D17 Case A "Why no append-duplicate option"). The multiple-match path remains in the contract above (rule 5) only as a defense against external mutation: a user (or another tool) hand-pasting a copy of a rule body elsewhere in the file. When `bytes.Count` returns ≥2 for the search pattern, syllago falls to D17's Modified flow rather than guessing which copy to remove. This is a defensive branch, not a supported install state.

The "fail to Modified path on imperfect match" rule is the design pattern from SaltStack's `file.blockreplace` ("fails hard if marker_end missing") expressed in marker-free terms. The principle is the same: when the file state diverges from what we wrote, defer to the user rather than risking destructive recovery.

**Why no markers (and why prior-art consensus does not apply here):**

- *LLM context tax:* HTML-comment markers like `<!-- syllago:<library-id> begin v<hash> -->` cost ~25-30 tokens per rule. A typical CLAUDE.md with 10 rules would carry 250-300 tokens of syllago metadata that the LLM reads on every turn, every project, forever. For a 2KB CLAUDE.md (~500 tokens), that is 50%+ overhead. This is a tax we cannot impose on every LLM invocation across a user's lifetime.
- *LLM semantic interpretation:* HTML comments are not invisible to the model — only to the rendered-markdown viewer. Models read marker text and may treat marked content as "managed, do not reason about" (the opposite of what we want for rules), echo markers when refactoring, or get confused about scope. This risk is well-precedented in practice with other tool-specific in-file metadata.
- *Co-modification by other tools:* Prettier, markdownlint, other AI coding tools, and manual edits routinely mutate CLAUDE.md and similar files. Markers introduce additional fragility — any tool that strips comments, reflows whitespace around markers, or "cleans up" perceived cruft becomes a failure mode for syllago's invariants. Marker-free bytes have no syllago-specific surface area for other tools to mangle.
- *Prior-art alignment for our specific domain:* Ansible/Salt/Puppet manage config files read by daemons, with thousands of similar single-line entries where markers are strictly necessary. Our domain is small markdown files with multi-line semantic blocks read by LLMs. The constraints that produced the marker consensus do not hold for us.

**Why the no-marker objections from earlier deliberation do not hold up:**

- *"We need markers for inside-block edit detection"*: D17's Modified path treats "user edited inside" and "block missing" identically (same modal, same Replace/Skip choice). The distinction is irrelevant to user-facing behavior.
- *"We need markers for performance"*: target files are KB-sized. O(file_size) byte search per rule is microseconds. No real performance argument exists at our scale.
- *"Replace must be O(1) given byte offsets"*: drop the O(1) claim. O(file_size) Replace is microseconds at our scale; the "byte offset/length" sidecar concept was over-engineered.

**Why we do NOT add an opt-in marker mode:**

- Adds complexity for a hypothetical user preference. If demand emerges later, add it then.
- Splits the install code path between two byte contracts; doubles the failure modes to test.

**Considered:**

- *HTML-comment marker pair (`<!-- syllago:<id> begin v<hash> -->` ... `<!-- syllago:<id> end -->`)*: industry-standard pattern from Ansible/Salt/Puppet. Rejected for the LLM-context, semantic-interpretation, and co-modification reasons above.
- *Sidecar byte-offset/length tracking in `installed.json`*: would make Replace genuinely O(1), but offsets are invalidated by any external edit before our block. Non-starter for files users and other tools modify.
- *Heading-based delimitation (find rule's H1, find next H1)*: semantically natural in markdown, but brittle if the user renames the heading and impossible if the rule has no top-level heading.
- *Markdown AST round-trip (mdast/Augeas-style)*: technically purest, but Go has no battle-tested round-trip markdown library that preserves formatting. Multi-month implementation cost for marginal benefit over canonical-byte search.

**Implications:**

- The "remove block in-place" operation in D17's Replace path uses search-and-splice on the target file bytes. No external state needed; no offset tracking in `installed.json`.
- D16's verification output contract is `verification_state: (LibraryID, TargetFile) → {Fresh, Clean, Modified}` plus the column-summary `match_set: LibraryID → []TargetFile`. No byte ranges, no offsets. Replace re-runs the byte search at execute time using the recorded `r.VersionHash` to look up the canonical content from `.history/`.
- Single canonical normalization helper (D12) is called at write time (when constructing the bytes to append), at scan time (D16's verification), and at uninstall/replace time (locating the block).
- Test fixtures must cover: install into empty file, install into file ending without `\n`, install into file ending with `\n`, install into file ending with `\n\n`, replace at end-of-file vs middle-of-file, uninstall when version bytes appear elsewhere in the file (zero-or-multiple-match path), uninstall when target file has been edited (Modified path), Replace when surrounding lines have been edited.

---

### D21. End-to-end roundtrip test for normalization-chain consistency

**Decision:** A single end-to-end roundtrip integration test exercises every byte path between the splitter, library storage, install, scan, and uninstall — using the normalization-sensitive fixtures from D19. The test is a hard ship gate for V1: all 10 cells of the matrix below must pass before V1 is callable.

**Why this test exists:** D6 + D20's no-markers approach is correct, but it concentrates all uninstall correctness into normalization consistency across five independently-implemented paths:

1. D12 canonical normalization (called at write, scan, and search time)
2. D11 `.history/<algo>-<hex>.md` write
3. D11 catalog load (reads `.history/*.md` into the in-memory `library[id].history` map)
4. D16 scan (reads target file, normalizes, runs `bytes.Contains` against history bytes)
5. D20 append/uninstall (writes/searches `\n<canonical content>` against target file)

If any single path normalizes differently from the others — a BOM strip applied at write time but not at search time, a trailing-newline rule applied on load but not on write — uninstall silently fails. Not a crash, not an exception: state goes to `Modified, edited` and the user sees a recovery modal for a file they did not edit. This is exactly the failure mode marker-based designs avoid by not relying on byte equivalence; we accepted the trade-off in D6/D20 because the alternative imposes an LLM context tax we will not pay. D21 is the test contract that makes the trade-off survivable. Without it, per-path tests can all pass while the composition fails silently — and the composition is what users hit in production.

**Matrix (10 cells):**

Each row asserts: install succeeds → scan returns `(Clean, none)` → uninstall restores target file to byte-equal pre-install state.

| Fixture (from D19) | Target file pre-state |
|---|---|
| `h2-clean.md` (LF baseline) | empty |
| `h2-clean.md` (LF baseline) | non-empty (with preamble) |
| `crlf-line-endings.md` | empty |
| `crlf-line-endings.md` | non-empty |
| `bom-prefix.md` | empty |
| `bom-prefix.md` | non-empty |
| `no-trailing-newline.md` | empty |
| `no-trailing-newline.md` | non-empty |
| `trailing-whitespace.md` | empty |
| `trailing-whitespace.md` | non-empty |

The five fixtures cover the dimensions D12 commits to handling: line endings (LF baseline + CRLF variant), BOM prefix, missing final newline, and significant trailing whitespace. The two pre-states cover D20's empty-file leading-`\n` invariant separately from the mid-file blank-line-separator case.

**What is intentionally NOT in the matrix:**

- *Install location variants* (project CLAUDE.md vs global vs AGENTS.md vs `.cursorrules`): install location only changes path resolution, not the byte path. Once the byte path is proven consistent against one location, the others retest the same five normalization layers. Per-location smoke tests live in install-method tests, not this matrix.
- *Provider-specific quirks* (D10 hints, Codex per-directory pattern): tested in their own integration tests; D21's scope is the byte-roundtrip property only.

**Roundtrip chain each cell exercises:**

```
1. Load fixture from testdata/
2. Run splitter (D3/D4) → []SplitCandidate
3. Normalize each candidate body (D12)
4. Write to library: rule.md + .syllago.yaml + .history/<algo>-<hex>.md (D11)
5. Catalog load: read .history/*.md into in-memory library[id].history map (D11)
6. Append-install into target file (D5 + D20 byte contract)
7. Scan target file (D16) → assert verification_state == (Clean, none)
8. Uninstall (D7) → assert target file bytes == pre-install snapshot, byte-for-byte
```

Each step touches a different package. The test is unique in covering all five normalization layers in composition.

**Catalog-load normalization invariant (resolved by construction):** Step 5 reads bytes from `.history/<algo>-<hex>.md`. Those bytes were written by step 4 already in canonical form (per D11's "files on disk = exact canonical bytes" invariant). The roundtrip will fail if the catalog-load path applies any further transformation — which is the only behavior that would let it fail at this step. The invariant therefore stands as: **history files written by step 4 are read back byte-equal by step 5 with zero further normalization applied.** This was an open ambiguity in D11 (raised in the Karpathy review as "does catalog load re-normalize on load?"); D21's matrix resolves it without a separate decision.

**Acceptance:**

- Test lives at `cli/internal/installer/roundtrip_test.go`. Installer is the natural home — it is the most-downstream package in the chain and already imports splitter, catalog, and scan.
- Test is **table-driven**, one row per matrix cell (~10–15 lines per cell, ~150 lines total including harness).
- All 10 cells must pass for V1 ship. Not advisory, not "we'll fix what it finds." If a cell fails, the underlying normalization-path mismatch must be fixed before V1 is callable.
- Phase 6 of the V1 plan gains D21 as an acceptance criterion: "Phase 6 ships when D21's 10-cell matrix passes." Phase 1 must author the 5 fixtures (already in D19); the test itself lands in Phase 6 alongside the verification scan it exercises.

**Considered:**

- *Per-path tests only (status quo)*: cheap, but as Karpathy notes, individual path tests can all pass while the composition fails silently. The cost of the composition test (~150 lines) is trivial relative to the worst-case failure mode (every user who hits a drift sees an unexplained Modified prompt).
- *Wider matrix (5 fixtures × 5 install locations = 25 cells)*: rejected as redundant — install location changes path resolution, not the byte path. One smoke test per location lives elsewhere.
- *Smaller matrix (4 cells: LF/CRLF × empty/non-empty)*: rejected as missing real-world normalization cases (BOM, no-trailing-newline, trailing-whitespace) that D12 commits to handling and that appear in the wild.
- *Make D21 advisory rather than a ship gate*: rejected. The whole point of the test is that the failure mode is silent in production — if it's advisory, an implementer under pressure will skip it, and we ship the bug. Hard gate or no test.

**Implications:**

- The 5 fixtures must exist before D21's test can run — they are listed in D19 and authored as part of Phase 1 (foundation).
- If catalog-load ever does start applying normalization on load (e.g., a future migration that changes the canonical form), D21 forces an explicit migration decision: either re-canonicalize every existing `.history/*.md` file at migration time, or accept that historical entries are now Modified by the column. Both are explicit decisions; neither is silent drift.
- Failure of any cell in CI blocks merge to main on any change to `cli/internal/converter/canonical/`, `cli/internal/installer/`, `cli/internal/catalog/`, or splitter code. This is the natural consequence of the test existing — not an additional rule.

---

## Open questions

These are not blockers for the doc, but need answers before implementation lands.

1. ~~Library directory layout for rules with history.~~ **Resolved by D11.**

2. ~~`.syllago.yaml` schema additions.~~ **Resolved by D13.**

3. ~~Install method record-keeping.~~ **Resolved by D14.** Scope-aware storage redesign deferred to a separate ADR (**D15**).

4. ~~Library "Installed" column truthfulness.~~ **Resolved by D16.**

5. ~~Update flow UX.~~ **Resolved by D17.**

6. ~~Per-directory discovery display.~~ **Resolved by D18.**

7. ~~Splitter test fixtures.~~ **Resolved by D19.**

**Additional decisions captured during the same review (no Q-number in the original list):**

- **Append byte contract** — concrete bytes that install writes and that uninstall/replace search for, and the failure-mode policy when the search returns zero or multiple matches. **Resolved by D20.**
- **End-to-end roundtrip test for normalization-chain consistency** — single integration test exercising splitter → library write → catalog load → install → scan → uninstall against normalization-sensitive fixtures, hard ship gate for V1. **Resolved by D21.**

---

## Research findings (summary)

Full research delivered in conversation 2026-04-23. Key facts:

- **YAML frontmatter on root monolithic rule files is essentially nonexistent in the wild.** 0 of 6 CLAUDE.md, 0 of 7 AGENTS.md, 0 of 10 GEMINI.md sampled. Frontmatter is reserved for rules-directory files.
- **H2 is universally the section unit.** Every sampled real-world file would split sensibly at H2.
- **Standalone `---` as section divider is essentially nonexistent.** Used decoratively or in tables, not as breaks.
- **AGENTS.md = CLAUDE.md = GEMINI.md structurally.** Often symlinked or delegating. Same splitter logic for all three.
- **`.cursorrules` is the highest-volume V1 use case.** ~9,120 files in GitHub search; 56% over 5KB. Migration to `.cursor/rules/*.mdc` is a real user need.
- **`.windsurfrules` is small by enforcement** (6KB cap); typically not worth splitting.
- **`.clinerules` files commonly use numbered-H2 patterns** (`## 1. Coding Style`); split very cleanly.
- **Codex uses hierarchical AGENTS.md by design** (88 files in openai/codex itself); splitting is allowed but per-directory install is the native pattern.

---

## V1 phase plan

Rough order. Some phases parallelize.

1. **Foundation**: provenance metadata fields (D1, D13) + per-directory discovery walk (D2) + canonical normalization helper (D12) + synthesized fixture *source files* per D19 (the input `.md` files only — expected splitter outputs land in Phase 2 once the splitter contract is concrete enough to assert against).
2. **Splitter core**: deterministic split with H2/H3/H4 + literal marker (D3, D4) returning `[]SplitCandidate`. Phase 1's fixtures get their expected-output halves authored here.
3. **Library storage with history**: per-rule directory layout with `rule.md` + `.syllago.yaml` + `.source/` + `.history/` (D11), schema per D13, write/read using D12 normalization. `hashToFilename` / `filenameToHash` helpers + load-time orphan invariant + hash-format lint land here.
4. **Add wizard adaptation**: Discovery step with multi-select (D18), Heuristic step (D3, D4), Review step rendering N candidates from M source files (D18), per-file skip-split detection (D4). Wizard is the primary install UX path, but Phases 5–7 are unit-testable via the install-library layer without the wizard wired up — they need not block on this phase.
5. **Append-to-monolithic install method**: second install path (D5), no in-file ownership (D6), append byte contract per D20, records via new `InstalledRuleAppend` entry type (D14), per-provider hints (D10) including the `NOTE:` stderr surface for non-interactive CLI.
6. **Exact-match uninstall** (D7): D16 verification implementation with mtime cache, normalization per D12, byte-search-and-remove per D20 (zero/multiple-match cases fall to D17 Modified path). Delivers the `verification_state` (State + Reason) function that Phases 7 and 9 consume. Can land in parallel with Phase 4. **Acceptance gate: D21's 10-cell roundtrip matrix must pass before this phase is considered done.**
7. **Update flow** (D17): explicit-decision routing per D16 state, Replace-in-place per D20 byte contract as default for `Clean`, Drop stale record as default for `Modified` (subject to the Drop-for-`unreadable` decision still pending). CLI flags `--on-clean` and `--on-modified`; TUI `installUpdateModal` / `installModifiedModal` as the interactive surface. Depends on Phase 6's `verification_state` function.
8. **TUI install picker** (D5, D10): user chooses between individual-file install (existing path) and monolithic append (D14 path), with the per-provider hints surfaced as non-blocking notes.
9. **Library "Installed" status surface** (D16 UI): wire Phase 6's `verification_state` into the TUI library column with binary states, plus the Modified sub-state and Reason in the metadata panel.
10. **LLM-skill in syllago-meta-registry** (D9): hard V1 dependency, not a follow-up. Two deliverables both required before V1 is callable: (a) author the `split-rules-llm` skill — SKILL.md + prompt + structured-output parsing that returns a `[]SplitCandidate` consumable by Phase 4's Review step; (b) publish to syllago-meta-registry so `syllago add split-rules-llm` resolves. Parallelizable with Phases 2–9 (different code surface), but ship gate for V1 includes both pieces — without them, `--split=llm` is a dead flag in the binary.

Out of V1 scope, separate ADRs:
- Scope-aware install record storage redesign (**D15**) — global vs project file split across all four record types.

---

## Fixture authoring guide

Per D19, all fixtures are **synthesized** (content fabricated) but **structurally modeled** after the real-world references below. Use each row as the spec for a synthesized fixture: same heading depth, line count band, structural quirks — fabricated content. URLs pinned to specific commits in research output for verification.

**CLAUDE.md / AGENTS.md / GEMINI.md shaped (modeled after):**
- saaspegasus/pegasus-docs CLAUDE.md (~45 lines, 7 H2s, no preamble) — clean small case
- p33m5t3r/vibecoding/conway CLAUDE.md (72 lines, 8 H2s, has standalone `---`) — `---` decorative usage (verifies we don't split on it)
- steadycursor/steadystart CLAUDE.md (~142 lines, 9 H2s) — medium with preamble
- payloadcms/payload CLAUDE.md (~330 lines, 9 H2s with H3 nesting, trailing `---`) — large with deep hierarchy
- grahama1970/claude-code-mcp-enhanced CLAUDE.md (~380 lines, emoji-prefixed headings) — slug normalization edge case
- kubernetes/kops AGENTS.md (118 lines, 3 H2 / 11 H3) — deep H3 nesting
- DataDog/lading AGENTS.md (184 lines, table-heavy with ADR cross-refs) — tables-in-content stress test
- matthiasn/lotti AGENTS.md (159 lines, 11 H2 / 4 H3) — canonical "Repository Guidelines" template
- pingcap/tidb AGENTS.md (208 lines, MUST/SHOULD/MAY language) — mandate-language casing preservation
- victrme/Bonjourr AGENTS.md (8 lines, no H2) — "do not split this" skip-split case
- google-gemini/gemini-cli GEMINI.md (~130 lines, 6 H2s) — canonical Google example
- fynnfluegge/rocketnotes GEMINI.md (~110 lines, H1+H2+H3 monorepo)
- panoptes/POCS GEMINI.md (58 lines, pure H1+H2, terse mandates)
- pathintegral-institute/mcpm.sh GEMINI.md (1-line file delegating to CLAUDE.md) — degenerate case

**`.cursorrules` shaped (modeled after):**
- Mirix-AI/MIRIX (well-structured, splits cleanly)
- uhop/stream-json (medium, mixed structure, points to AGENTS.md)
- level09/enferno (numbered flat list, anti-fixture for "don't split")

**`.clinerules` shaped (modeled after):**
- nammayatri/nammayatri (textbook splittable, 105 lines, numbered `## N. Topic` H2s)
- sammcj/gollama (medium, H1-sectioned)
- cline/prompts (canonical Cline-recommended structure)

**`.windsurfrules` shaped (modeled after):**
- SAP/fundamental-ngx (1-line pointer — nonsensical to split)
- level09/enferno (17 numbered rules)

REFERENCES.md inside the fixture directory captures the same mapping in a more compact table so a developer opening the fixture directory can see the inspiration without scrolling this design doc.

---

## Conversation history pointer

Design decisions captured here originated in conversation on 2026-04-23 between Holden and Claude (Opus 4.7). Research was delegated to four parallel sub-agents (CLAUDE.md, AGENTS.md, GEMINI.md, legacy single-file formats). Full research outputs are in the conversation transcript; key findings inlined above.

This document supersedes any in-conversation working notes once it is current. Update this doc when decisions change rather than restating in conversation.
