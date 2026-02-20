# Nesco Design Decisions — Phase 1 + 2 Brainstorm

**Date:** 2026-02-15
**Scope:** Phase 1 (Provider Parity) + Phase 2 (Scan + Surprise Detection)
**Method:** One-by-one brainstorm, decisions captured as made

---

## Table of Contents

1. [Tool Identity: Content Manager + Scan/Detect](#1-tool-identity-content-manager--scandetect)
2. [Data Model: Two Separate Models](#2-data-model-two-separate-models)
3. [Provider Abstraction: Expand Existing Interface](#3-provider-abstraction-expand-existing-interface)
4. [ContextDocument Shape: Hybrid Typed + Text](#4-contextdocument-shape-hybrid-typed--text)
5. [Content-Type Mapping: Emitters Decide](#5-content-type-mapping-emitters-decide)
6. [.nesco/ Directory: Scan State, Git-Tracked](#6-nesco-directory-scan-state-git-tracked)
7. [Reconciler: Standalone Step After Emitter](#7-reconciler-standalone-step-after-emitter)
8. [Conflict Handling: Warn But Don't Touch](#8-conflict-handling-warn-but-dont-touch)
9. [Detector Architecture: Uniform Interface](#9-detector-architecture-uniform-interface)
10. [Parity Analysis: Separate Command + Scan --full](#10-parity-analysis-separate-command--scan---full)
11. [Surprise Detectors: Full Catalog (24 Detectors)](#11-surprise-detectors-full-catalog-24-detectors)
12. [CLI Command Surface: Phase 1 + 2](#12-cli-command-surface-phase-1--2)
13. [Project Structure: Flat internal/](#13-project-structure-flat-internal)
14. [Testing: Fixture Directories](#14-testing-fixture-directories)
15. [Boundary Markers: Section-Level](#15-boundary-markers-section-level)
16. [Import: Read-Only Into Memory](#16-import-read-only-into-memory)

---

## Decisions

### 1. Tool Identity: Content Manager + Scan/Detect

**Question:** How should the existing content manager TUI (v0.2.0) and the new scan/detect/drift capabilities relate?

**Options considered:**
- **Single unified CLI** — One binary, two concerns. TUI stays as default, scan/drift/etc. as subcommands.
- **Separate binaries** — Split into nesco-tui and nesco. Shared Go module, different build targets.
- **Content manager becomes subcommand** — `nesco` becomes the scan tool, TUI moves to `nesco content`.
- **Content manager deprecated** — TUI fades, scan/detect becomes the whole product.

**Decision:** Single unified CLI.

**Rationale:** One binary keeps distribution simple (single `go install`, single Homebrew formula). The TUI and scan capabilities are complementary — content repo provides the skills/rules that scan results inform. `nesco` (no args) launches the TUI. `nesco scan`, `nesco drift`, `nesco import`, etc. are subcommands. Both concerns share the same provider abstraction and content model underneath.

**Implications:**
- Existing `cmd/nesco/main.go` gains new cobra subcommands alongside the TUI launcher
- Provider detection, content types, and the canonical model are shared infrastructure
- The TUI can surface scan results and drift status in the future (natural integration point)

---

### 2. Data Model: Two Separate Models

**Question:** Should the content repo model (Items with metadata) and the scan/emit model (ContextDocument with sections) be the same data structure?

**Options considered:**
- **Two separate models** — Item (content repo) and ContextDocument (scan/emit) are distinct types sharing the provider/emitter layer.
- **Unified model** — One model for both. More complex, risks forcing two concerns into one shape.
- **ContextDocument wraps Items** — Items become a section type within ContextDocument. Everything goes through one pipeline.

**Decision:** Two separate models.

**Rationale:** They represent fundamentally different things with different lifecycles:
- `Item` = an installable content package (skill, rule, hook) with metadata, versioning, install/uninstall semantics. Managed by the content repo TUI.
- `ContextDocument` = generated context about a codebase (tech stack, conventions, surprises). Produced by scan, consumed by emitters, tracked by drift.

Both need provider knowledge (where files go, what formats look like), so they share the provider abstraction layer. But they don't share data shape.

**Implications:**
- `internal/model/` holds ContextDocument, ContextSection types (new)
- `internal/catalog/` continues holding Item, ContentType types (existing)
- `internal/provider/` is shared infrastructure — both models use it to know where to write
- Emitters for scan output are separate from the install logic for content items
- Clear boundary: catalog/installer handles Items, scan/emit handles ContextDocuments

---

### 3. Provider Abstraction: Expand Existing Interface

**Question:** How should we expand the provider abstraction to handle Phase 1's needs (discovery maps, format knowledge, capability matrix, emit targets)?

**Options considered:**
- **Expand existing Provider interface** — Add methods: DiscoveryPaths, FileFormat, Supports, EmitPath. Each provider file grows richer.
- **Provider + ProviderProfile** — Separate install-time from scan-time knowledge into two types.
- **Declarative provider configs** — Define providers as YAML/JSON rather than Go code.

**Decision:** Expand the existing Provider interface.

**Rationale:** With 6 providers, the "add a new provider" event is rare enough that recompiling is fine. Co-locating all provider knowledge in one Go file per provider keeps things navigable, type-checked, and testable. Declarative configs add indirection without real benefit at this scale.

**New methods on Provider (sketch):**
```go
// Existing
Detect(homeDir string) bool
InstallDir(homeDir string, contentType ContentType) string

// New for Phase 1
Supports(contentType ContentType) bool                    // capability matrix
DiscoveryPaths(homeDir string, contentType ContentType) []string  // where to find existing content
FileFormat(contentType ContentType) Format                // .mdc, .md, .json, etc.
EmitPath(projectRoot string, contentType ContentType) string     // where to write generated context
```

**Implications:**
- Each provider Go file (`claude.go`, `cursor.go`, etc.) becomes the single source of truth for that provider
- Existing installer code continues working — new methods are additive
- Import discovery and scan emission both call into the same provider for path knowledge
- The `Supports()` method replaces the current implicit "return empty string for unsupported" pattern

---

### 4. ContextDocument Shape: Hybrid Typed + Text

**Question:** How structured should ContextDocument sections be?

**Options considered:**
- **Fully typed sections** — Every section has a Go struct with fields. Maximum emitter intelligence, more upfront code, forces structure on narrative content.
- **Flat text sections** — Category + markdown. Simple but emitters can't format intelligently.
- **Hybrid** — Typed structs for core deterministic sections, freeform text for heuristic/surprise/curated.

**Decision:** Hybrid — typed for Tier 1 deterministic, text for the rest.

**Rationale:** The data is genuinely two shapes:
- **Structured:** Tech stack versions, dependency lists, build commands — these are parsed facts with fields. Typed structs let emitters format them per-provider and let drift diff at the field level.
- **Narrative:** Surprise detections ("two testing frameworks coexist"), curated interview answers, heuristic observations — these are inherently freeform text. Forcing them into structs produces `SurpriseSection{Type: "...", Description: "..."}` which is just text with extra steps.

**Typed sections (Phase 2 detectors):**
- `TechStackSection` — Language, Version, Framework, FrameworkVersion
- `DependencySection` — grouped deps with Name, Version, Category
- `BuildCommandSection` — Command name, script, source file
- `DirectoryStructureSection` — tree representation with conventional pattern tags
- `ProjectMetadataSection` — description, license, CI setup

**Text sections:**
- Heuristic detections (file conventions, code style presence)
- Surprise detections (inconsistencies, competing conventions)
- Curated content (interview answers, architecture rationale)

**Implications:**
- Emitters have two code paths: typed formatting and text pass-through
- Drift engine can do field-level diffing on typed sections, hash-based on text sections
- `internal/model/` has both typed section structs and a generic `TextSection` type
- ContextDocument holds `[]Section` where Section is an interface satisfied by both

---

### 5. Content-Type Mapping: Emitters Decide

**Question:** Should sections know which provider content type they map to (rules, hooks, skills)?

**Options considered:**
- **Emitters decide** — Sections carry semantic categories. Each emitter maps categories → provider content types.
- **Sections declare target** — Each section includes a content type hint.
- **Mapping config per provider** — Separate mapping table, neither section nor emitter hardcodes it.

**Decision:** Emitters decide.

**Rationale:** A "tech stack" section is a semantic category — it's not inherently a "rule" or a "context file." The Claude emitter writes it into CLAUDE.md (a rule file). The Cursor emitter might split it into a separate `.mdc` file with `alwaysApply: true`. Having the emitter own this mapping keeps sections provider-agnostic and lets each emitter optimize for its platform.

**Implications:**
- Sections carry a `Category` field (enum: tech-stack, dependencies, build-commands, conventions, surprises, etc.)
- Each emitter has its own mapping logic: category → file placement, formatting, frontmatter
- Adding a new provider means writing one emitter that defines its own mapping
- No coupling between detectors and providers — detectors don't need to know about Cursor's `.mdc` format

---

### 6. .nesco/ Directory: Scan State, Git-Tracked

**Question:** What lives in `.nesco/` and should it be git-tracked?

**Options considered (scope):**
- **Scan state only** — config, baseline, interview answers. No content repo state.
- **Everything** — scan state + content install tracking + provider config.
- **Minimal** — config.json only. Baselines computed on demand.

**Options considered (tracking):**
- **Git-tracked** — Team shares config and baseline. Consistent drift detection.
- **Gitignored** — Local state per developer.
- **Selective** — Config tracked, baseline gitignored.

**Decision:** Scan state only, git-tracked by default.

**Contents of `.nesco/`:**
```
.nesco/
├── config.json      # Provider selection, preferences, enabled detectors
├── baseline.json    # Last scan snapshot (section hashes + typed data)
└── interview.json   # Curated answers from guided interview (Phase 3)
```

**Rationale:**
- `.nesco/` is scan infrastructure — it exists to support `nesco scan`, `nesco drift`, and the onboarding workflow. The content repo (TUI) doesn't need it.
- Git-tracking means the team shares a common baseline. When someone runs `nesco drift`, they're comparing against the same known-good state. This makes CI drift checks meaningful — everyone agrees on what "current" looks like.
- `interview.json` is git-tracked because curated context IS the team's shared knowledge. It's the high-value output of the onboarding process.

**Created by:** `nesco scan` (first run) or `nesco init` (explicit setup without scanning).

**Implications:**
- `.nesco/` does not exist until someone runs scan or init — zero footprint until opted in
- Content manager TUI works without `.nesco/` existing at all
- `NESCO_CONFIG_DIR` env var overrides location for special setups
- Baseline updates are explicit (via `nesco scan` or `nesco baseline`), never silent

---

### 7. Reconciler: Standalone Step After Emitter

**Question:** Where does the reconciler live in the pipeline?

**Options considered:**
- **Standalone step after emitter** — Emitter produces complete auto-section string. Reconciler reads existing file, merges, writes.
- **Inside the emitter** — Each emitter handles its own merging. Duplicates reconciliation logic.
- **At the command level** — `scan` command orchestrates reading, emitting, stitching.

**Decision:** Standalone step after emitter.

**Pipeline (clarified):**
```
Detectors → ContextDocument → Emitter (per provider) → formatted string → Reconciler → disk
                                   ↑ pure function          ↑ reads existing file
                                   no disk I/O              handles boundary markers
```

1. Detectors produce a ContextDocument (all detected sections)
2. Per configured provider, the emitter renders auto-maintained sections into that provider's format (pure function — no disk I/O)
3. Reconciler reads the existing file on disk, finds `nesco:auto`/`nesco:human` boundary markers, replaces auto sections with fresh emitter output, preserves human sections verbatim, writes merged result

**Rationale:** The emitter produces all auto sections every time (not just changed ones), but this is cheap — it's string formatting. The reconciler is the only component that touches disk. This keeps the emitter testable as a pure function and centralizes all file-merging logic in one place rather than duplicating it across emitters.

**Implications:**
- `internal/reconcile/` is a new package
- Emitters remain pure: `ContextDocument → string`. Testable without filesystem.
- Reconciler is format-aware only in terms of boundary marker syntax (HTML comments for markdown, YAML comments for `.mdc`)
- Reconciler coordinates baseline writes — `.nesco/baseline.json` updates only after successful file emission

---

### 8. Conflict Handling: Warn But Don't Touch

**Question:** How should nesco handle when a curated section contradicts scan results?

**Options considered:**
- **Warn but don't touch** — Surface discrepancy in output. Never modify curated sections.
- **Append a note** — Add comment inside curated section. Modifies human sections (feels wrong).
- **Separate conflicts file** — Write to `.nesco/conflicts.json`.

**Decision:** Warn but don't touch.

**Rationale:** Curated sections are sacrosanct — they represent human knowledge captured through the interview process. Modifying them, even with a helpful note, violates the trust model ("nesco will never overwrite my content"). Surfacing the discrepancy in scan output (and in `nesco drift` reports) gives the human or skill enough information to act.

**Example output:**
```
⚠ Conflict: curated "Architecture" section mentions React 18,
  but scan detected React 19 in package.json.
  Run `nesco interview --update` to review curated content.
```

**Implications:**
- Reconciler's only job with human sections is `preserve verbatim`
- Conflict detection compares curated text against typed section data (fuzzy matching — version strings, framework names)
- Conflicts appear in `nesco scan` output, `nesco drift` output, and JSON mode
- No `.nesco/conflicts.json` — conflicts are transient, computed at scan/drift time

---

### 9. Detector Architecture: Uniform Interface

**Question:** Should surprise detectors use the same interface as fact detectors, or be a separate system?

**Options considered:**
- **Same interface, different category** — Surprise detectors implement `Detector`. Return `TextSection` with category `surprise`. All detectors run in parallel as peers.
- **Two-pass system** — Facts first, then surprise detectors receive the ContextDocument + filesystem. More powerful, more complex.
- **Post-processing** — Single surprise analyzer examines the full ContextDocument. Centralizes surprise logic but creates a monolith.

**Decision:** Same interface, different category.

**Rationale:** Surprise detectors can discover facts independently — a "competing test frameworks" detector walks the filesystem and finds both Jest and Vitest configs directly. It doesn't need the tech stack detector's output. The uniform interface keeps the scanner simple (run all detectors in parallel, collect results) and makes adding new detectors trivial.

If a future surprise detector genuinely needs fact detector output, we can evolve the interface then (add optional `ContextDocument` parameter). Don't build the two-pass pipeline until there's a concrete need.

**Detector interface (confirmed):**
```go
type Detector interface {
    Name() string
    Detect(root string) ([]Section, error)
}
```

**Execution model:**
- All detectors (fact + surprise) run in parallel via goroutines
- 5-second timeout per detector (configurable)
- A detector that panics or times out is skipped with a warning
- Target: under 2 seconds total for typical repo
- Results collected, ordered by category, assembled into ContextDocument

**Implications:**
- `internal/scan/detectors/` holds both fact and surprise detectors
- Scanner doesn't distinguish between types — it runs everything and collects results
- New detectors are added by implementing the interface and registering with the scanner
- Surprise detectors return `TextSection` (category: surprise) with a descriptive finding

---

### 10. Parity Analysis: Separate Command + Scan --full

**Question:** Should agent config parity analysis be a detector within scan, a separate command, or a flag?

**Options considered:**
- **Separate command (`nesco parity`)** — Clean separation, independently useful.
- **Detector within scan** — One command, one output, but mixes concerns.
- **`nesco scan --full`** — Default scan is codebase-only, full adds parity.

**Decision:** Separate command AND composable via `--full` flag.

- `nesco parity` — Standalone command comparing AI tool configs across providers. Reports gaps, suggests sync opportunities.
- `nesco scan` — Codebase analysis only (facts + surprises). Output includes a hint: "Run `nesco parity` to check AI tool config gaps."
- `nesco scan --full` — Runs both codebase scan AND parity analysis. Combined output.

**Rationale:** Parity analysis is a different concern from codebase scanning — it compares AI tool configurations, not codebase patterns. Making it independently invocable means you can check parity without scanning (useful when you just added a new tool and want to know what's missing). The `--full` flag composes both for the common "give me everything" use case.

**Implications:**
- `internal/parity/` is a new package — reads all detected providers, compares content type coverage
- `nesco parity` calls provider detection + discovery, reports per-provider per-content-type matrix
- `nesco scan --full` calls scanner then parity analyzer, combines output
- JSON mode: parity results are a separate top-level key in the output
- The onboarding skill (Phase 3) will call `nesco scan --full` to get the complete picture

---

### 11. Surprise Detectors: Full Catalog (24 Detectors)

**Question:** Which surprise detectors should ship in Phase 2?

**Research conducted:** Four research agents studied AI agent failure patterns:
- General survey: 25 categories across 12 areas, sourced from CodeRabbit, IEEE Spectrum, Stack Overflow, Columbia DAPLab, Augment Code, JetBrains, and others
- Go-specific: 18 categories (internal packages, nil interfaces, CGO, module system, goroutine patterns)
- Python-specific: 20 categories (version constraints, framework conventions, async/sync, packaging, type hints)
- Rust-specific: 17 categories (editions, feature flags, async runtimes, unsafe policy, serde, MSRV)

**Decision:** Ship all 24 detectors — 12 cross-language + 12 language-specific.

**Rationale:** Each detector maps to a documented AI failure pattern backed by research. Language-specific detectors activate only when the relevant language is detected (go.mod, pyproject.toml, Cargo.toml, package.json), so they add no overhead to projects that don't use that language. The detector interface (Decision #9) makes each one independently implementable and testable.

---

#### Cross-Language Detectors (12) — Active for all projects

| # | Detector | What it finds | Impact |
|---|----------|--------------|--------|
| 1 | **Competing Frameworks** | Two tools of the same category coexist (Jest+Vitest, pytest+unittest, testify+stdlib, gin+echo, etc.) | HIGH |
| 2 | **Module System Conflict** | ESM/CJS mixing in JS, relative/absolute import inconsistency in Python, edition-dependent syntax in Rust | VERY HIGH |
| 3 | **Migration-in-Progress** | Old + new patterns coexisting (.js+.ts, class+hooks, Pages+App Router, Python 2 compat) | HIGH |
| 4 | **Custom Wrapper Bypass Risk** | Internal wrappers around common libraries (HTTP, DB, logging) that AI will ignore | HIGH |
| 5 | **Lock File Conflict** | Multiple package manager artifacts coexist (package-lock.json + yarn.lock, Pipfile + poetry.lock) | MEDIUM |
| 6 | **Test Convention Mismatch** | Tests in wrong location, wrong naming (.test vs .spec), wrong framework style | HIGH |
| 7 | **Deprecated Pattern Prevalence** | @deprecated markers, legacy/ directories, TODO:migrate comments — patterns AI should NOT follow | MEDIUM-HIGH |
| 8 | **Path Alias Gap** | Aliases configured (tsconfig paths, vite resolve) but inconsistently used | HIGH |
| 9 | **Major Version Pattern Mismatch** | Framework version X installed but code patterns match version Y (Next.js 14 but Pages Router code) | VERY HIGH |
| 10 | **Version Constraint Violation** | Language version in config vs syntax features in code (go.mod version vs generics, requires-python vs match/case) | HIGH |
| 11 | **Environment Variable Convention** | Framework-specific prefixes not followed (NEXT_PUBLIC_, VITE_), vars in code not in .env.example | HIGH |
| 12 | **Formatter/Linter Config Extraction** | Parse enforced rules from .prettierrc, .editorconfig, ruff.toml, .golangci.yml, clippy.toml | MEDIUM |

**Detection approach per cross-language detector:**

1. **Competing Frameworks** — Maintain a category map: `{testing: [jest, vitest, mocha, pytest, unittest, testify], styling: [tailwind, styled-components, css-modules], orm: [prisma, typeorm, drizzle, gorm, sqlx], ...}`. For each category, check if 2+ are present in dependency files.

2. **Module System Conflict** — JS: check `"type"` in package.json vs `require()`/`import` usage. Python: check for `from __future__ import absolute_import` alongside modern imports. Rust: check edition in Cargo.toml vs syntax patterns.

3. **Migration-in-Progress** — Detect dual patterns: `.js`+`.ts` file ratio in src/, class+function components, `pages/`+`app/` in Next.js, `require()`+`import` mixing. Report the ratio and which pattern appears newer.

4. **Custom Wrapper Bypass Risk** — Find files in util/lib/common directories that import a popular library and re-export configured version. Check if rest of codebase imports the wrapper or the raw library. Flag when both patterns exist.

5. **Lock File Conflict** — Check for coexistence: `package-lock.json`+`yarn.lock`, `package-lock.json`+`pnpm-lock.yaml`, `Pipfile.lock`+`poetry.lock`, `Pipfile`+`uv.lock`. Also check for `.npmrc`/`.yarnrc` mismatches.

6. **Test Convention Mismatch** — Glob for test files. Determine naming (.test vs .spec), location (co-located vs top-level `tests/`), and framework imports. Check test runner config for `testMatch`/`include` patterns. Flag inconsistencies.

7. **Deprecated Pattern Prevalence** — Grep for `@deprecated`, `// DEPRECATED`, `# Deprecated`, `TODO: migrate`, `TODO: remove`. Find directories named legacy/, old/, deprecated/. Report count and locations.

8. **Path Alias Gap** — Parse tsconfig.json `paths`, vite.config resolve aliases, webpack resolve aliases. Scan imports in source — calculate ratio of alias vs relative imports. Flag if aliases defined but rarely used (or vice versa).

9. **Major Version Pattern Mismatch** — Parse major version of key frameworks from dependency files. Check for version-specific filesystem patterns: `app/` vs `pages/` (Next.js), `tailwind.config.js` format (v3 vs v4), React class vs function component ratio.

10. **Version Constraint Violation** — Parse declared language version (go.mod `go` directive, pyproject.toml `requires-python`, Cargo.toml `edition`, tsconfig `target`, .nvmrc). Scan for syntax features requiring higher versions. Cross-reference against feature availability tables.

11. **Environment Variable Convention** — Parse `.env.example`/`.env.template` for documented vars. Detect framework-specific prefixes (`NEXT_PUBLIC_`, `VITE_`, `REACT_APP_`). Scan source for `process.env.*`/`import.meta.env.*` usage. Flag undocumented vars and wrong prefixes.

12. **Formatter/Linter Config Extraction** — Parse config files: `.prettierrc` (quote style, semicolons, indent), `.editorconfig` (indent size, charset), `ruff.toml`/`pyproject.toml [tool.ruff]` (selected rules), `.golangci.yml` (enabled linters), `clippy.toml` (configured lints). Extract as structured facts.

---

#### Go-Specific Detectors (4) — Active when go.mod detected

| # | Detector | What it finds | Impact |
|---|----------|--------------|--------|
| 13 | **Internal Package Visibility** | `internal/` packages imported from outside allowed scope | CRITICAL |
| 14 | **Nil Interface Comparison** | Functions returning interface types (especially error) that return typed nil values | HIGH |
| 15 | **CGO Detection** | `import "C"` or `//go:build cgo` in a project that may run with CGO_ENABLED=0 | HIGH |
| 16 | **Replace Directives** | `replace` directives in go.mod pointing to local paths or different versions | MEDIUM-HIGH |

**Detection approach:**

13. **Internal Package Visibility** — Find directories named `internal/`. Trace import statements across all .go files. Flag any import of an `internal` package from outside its allowed parent tree.

14. **Nil Interface Comparison** — Find functions with return type `error` (or other interfaces) that return a concrete nil pointer (e.g., `return (*MyError)(nil)`). Find `== nil` comparisons on interface-typed variables that could hold typed nils.

15. **CGO Detection** — Search for `import "C"` or `//go:build cgo` in .go files. Check CI config and Dockerfile for `CGO_ENABLED` settings. Flag if CGO code exists but no evidence the build environment supports it.

16. **Replace Directives** — Parse go.mod `replace` statements. Flag filesystem-path replacements (common during local dev, breaks CI). Flag version replacements that may indicate pinning workarounds.

---

#### Python-Specific Detectors (3) — Active when pyproject.toml/setup.py/requirements.txt detected

| # | Detector | What it finds | Impact |
|---|----------|--------------|--------|
| 17 | **Async/Sync Context Mismatch** | Sync blocking calls inside async handlers (Django/FastAPI) | HIGH |
| 18 | **Package Layout Detection** | src/ layout vs flat layout affects import paths and test configuration | MEDIUM |
| 19 | **Namespace Package Confusion** | Directories with .py files but no `__init__.py` (PEP 420 namespace packages) | MEDIUM |

**Detection approach:**

17. **Async/Sync Context Mismatch** — Find `async def` route handlers. Scan bodies for sync blocking calls: `time.sleep()`, `requests.get()`, synchronous DB queries. Check for `asyncio.to_thread()` or `loop.run_in_executor()` wrappers. Flag unwrapped sync calls in async contexts.

18. **Package Layout Detection** — Check for `src/` directory containing package code. Look at pyproject.toml for `packages = [{from = "src"}]`. Determine layout and report implications for imports and testing.

19. **Namespace Package Confusion** — Find directories containing `.py` files but missing `__init__.py`. Check if this is intentional (namespace packages) or accidental. Cross-reference with Ruff INP001 rule if configured.

---

#### Rust-Specific Detectors (3) — Active when Cargo.toml detected

| # | Detector | What it finds | Impact |
|---|----------|--------------|--------|
| 20 | **Feature Flag Mapping** | Conditional compilation paths that only exist with specific feature flags | HIGH |
| 21 | **Unsafe Code Policy** | `#![forbid(unsafe_code)]` or `#![deny(unsafe_code)]` in lib.rs/main.rs | HIGH |
| 22 | **Async Runtime Choice** | Tokio vs smol vs async-std — incompatible runtime traits and macros | HIGH |

**Detection approach:**

20. **Feature Flag Mapping** — Parse `[features]` and `default = [...]` from Cargo.toml. Scan source for `#[cfg(feature = "...")]` attributes. Build a map of what code is available under which feature combinations. Report non-default features that gate significant functionality.

21. **Unsafe Code Policy** — Search lib.rs and main.rs for `#![forbid(unsafe_code)]` or `#![deny(unsafe_code)]`. Check for `#![deny(unsafe_op_in_unsafe_fn)]`. Report the project's unsafe stance so AI knows whether unsafe is acceptable.

22. **Async Runtime Choice** — Check dependencies for `tokio`, `async-std`, `smol`. Detect entry point macros: `#[tokio::main]`, `#[async_std::main]`, `smol::block_on`. Report which runtime is canonical. Note: async-std is discontinued (March 2025) in favor of smol.

---

#### JS/TS-Specific Detectors (2) — Active when package.json/tsconfig.json detected

| # | Detector | What it finds | Impact |
|---|----------|--------------|--------|
| 23 | **TypeScript Strictness Level** | `strict: true` and individual strict flags in tsconfig.json | HIGH |
| 24 | **Monorepo Workspace Structure** | Workspace configuration and cross-package dependency mapping | HIGH |

**Detection approach:**

23. **TypeScript Strictness Level** — Parse tsconfig.json for `strict`, `noImplicitAny`, `strictNullChecks`, `strictFunctionTypes`, etc. Report the effective strictness level. If strict mode is on, AI must generate fully typed code.

24. **Monorepo Workspace Structure** — Check for workspace markers: `pnpm-workspace.yaml`, `turbo.json`, `nx.json`, `lerna.json`, `package.json` with `workspaces`. Map workspace packages and their interdependencies. Report shared configuration packages.

---

### 12. CLI Command Surface: Phase 1 + 2

**Question:** What commands ship, with what flags and behavior?

**Decision: First run is interactive.** `nesco scan` on a project without `.nesco/config.json` detects providers, shows findings, asks for confirmation. `--yes` skips prompts for CI/automation. `NESCO_NO_PROMPT=1` also skips prompts.

**Decision: Import ships in Phase 1.** Reading existing provider configs is essential for parity analysis. Phase 1 adds parsers; Phase 4 adds cross-format conversion.

**Phase 1 Commands:**

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `nesco` (no args) | Launch TUI (existing) | — |
| `nesco init` | Interactive setup — detect providers, create `.nesco/config.json` | `--yes`, `--json` |
| `nesco import --from <provider>` | Read existing AI tool configs into canonical model | `--type <content-type>`, `--preview`, `--json` |
| `nesco parity` | Compare AI tool configs across providers, report gaps | `--json`, `--format <provider>` |
| `nesco config` | View/edit provider selection and preferences | `list`, `add`, `remove` subcommands |
| `nesco info` | Machine-readable capability manifest | `formats`, `detectors`, `providers` subcommands |

**Phase 2 Commands (added):**

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `nesco scan` | Scan codebase, generate context files | `--format <provider>`, `--all`, `--full`, `--dry-run`, `--json`, `--yes` |
| `nesco drift` | Compare current state against baseline | `--ci`, `--json` |
| `nesco baseline` | Accept current state as baseline without regenerating files | `--from-import`, `--json` |

**Global Flags (all commands):** `--json`, `--no-color`, `--quiet`, `--verbose`, `--version`, `--help`

**Semantic Exit Codes:**
- 0 — Success
- 1 — User-correctable error (bad flags, missing config)
- 2 — Scan issue (no detectable project, ambiguous results)
- 3 — Drift detected (only in `--ci` mode)
- 4+ — Internal errors

**`nesco scan --full` behavior:**
1. Run codebase scan (all detectors)
2. Run parity analysis
3. Emit combined report (human-readable or JSON)
4. Update baseline

**`nesco import --preview` behavior:**
- Shows what would be imported without writing anything
- Reports: "Found 12 Cursor rules, 3 hooks, 1 MCP config. No skills or agents detected. 2 files couldn't be classified."

---

### 13. Project Structure: Flat internal/

**Question:** How should new packages be organized alongside existing ones?

**Options considered:**
- **Flat internal/** — All packages at the same level. Simple, no nesting.
- **Grouped by concern** — internal/content/ (TUI) vs internal/context/ (scan/emit). Adds nesting.
- **Separate cmd/ entries** — Each subcommand gets its own main.go.

**Decision:** Flat internal/.

**Updated project structure:**
```
cli/
├── cmd/nesco/
│   └── main.go              # CLI entrypoint — TUI + cobra subcommands
├── internal/
│   ├── catalog/             # [EXISTING] Content repo scanning/indexing
│   ├── metadata/            # [EXISTING] .nesco.yaml handling
│   ├── provider/            # [EXISTING, EXPANDED] Provider detection + Phase 1 methods
│   ├── installer/           # [EXISTING] Content install/uninstall
│   ├── promote/             # [EXISTING] Local → shared promotion
│   ├── tui/                 # [EXISTING] Bubble Tea UI
│   ├── model/               # [NEW] ContextDocument, Section types
│   ├── scan/                # [NEW] Scanner orchestrator + detectors/
│   │   ├── scanner.go       #   Parallel detector execution
│   │   └── detectors/       #   Individual detector implementations
│   ├── emit/                # [NEW] Emitters (pure functions, one per provider)
│   ├── parse/               # [NEW] Parsers (read provider formats → canonical model)
│   ├── reconcile/           # [NEW] Merge emitter output with existing files
│   ├── drift/               # [NEW] Baseline management + diffing
│   ├── parity/              # [NEW] Agent config parity analysis
│   ├── config/              # [NEW] .nesco/config.json read/write
│   └── output/              # [NEW] JSON vs human-readable output switching
```

**Rationale:** Go's flat package convention works well here — each package has a clear single responsibility. No nesting means every import path is `internal/<package>`. The `scan/detectors/` sub-package is the one exception — detectors are numerous enough to warrant their own directory under scan/.

**Implications:**
- `cmd/nesco/main.go` registers cobra subcommands that call into these packages
- Shared dependencies flow through `provider/` and `model/`
- Each new package is independently testable
- Existing packages are untouched except `provider/` which gets new methods (Decision #3)

---

### 14. Testing: Fixture Directories

**Question:** How should detectors (and parsers, emitters) be tested?

**Options considered:**
- **Fixture directories** — Checked-in testdata/ with purpose-built mini-projects. Deterministic, fast, readable.
- **Generated fixtures** — Tests create temp directories programmatically. Flexible but harder to read.
- **Real repo snapshots** — Clone real repos. Realistic but slow, brittle, large.

**Decision:** Fixture directories.

**Test structure:**
```
cli/internal/scan/detectors/testdata/
├── techstack/
│   ├── node-project/         # package.json, tsconfig.json
│   ├── go-project/           # go.mod
│   ├── python-project/       # pyproject.toml
│   └── multi-language/       # go.mod + package.json
├── build-commands/
│   ├── makefile-project/     # Makefile with standard targets
│   └── npm-scripts/          # package.json with scripts
├── surprises/
│   ├── competing-frameworks/ # jest.config + vitest.config
│   ├── mixed-naming/         # camelCase + kebab-case files
│   └── ...
```

**Testing approach per component:**
- **Detectors:** Run against fixture dirs, assert returned sections match expected output (golden file or struct comparison)
- **Emitters:** Pure function tests — pass ContextDocument, assert formatted string matches golden file
- **Parsers:** Pass provider-specific file content, assert canonical model matches expected
- **Reconciler:** Pass emitter output + existing file content, assert merged result preserves human sections
- **Drift:** Compare two ContextDocuments, assert diff report matches expected

**Implications:**
- `testdata/` directories are checked into the repo alongside test files
- Golden file pattern where useful: `testdata/techstack/node-project.golden.json`
- Emitter tests are filesystem-free (pure function → string comparison)
- Reconciler tests need both "existing file" and "emitter output" as inputs

---

### 15. Boundary Markers: Section-Level

**Question:** Should boundary markers wrap individual sections or divide the file into zones?

**Options considered:**
- **Section-level markers** — Each section tagged: `<!-- nesco:auto:tech-stack -->`. Reconciler identifies/replaces individual sections by name.
- **Zone-level markers** — `<!-- nesco:auto-start -->` ... `<!-- nesco:auto-end -->`. Simpler but rigid ordering.
- **No markers in output** — Mapping in `.nesco/manifest.json`. Clean files but fragile.

**Decision:** Section-level markers.

**Marker format (markdown files):**
```markdown
<!-- nesco:auto:tech-stack -->
## Tech Stack
- TypeScript 5.3 / Node.js 20
- Next.js 14 (App Router)
<!-- /nesco:auto:tech-stack -->

<!-- nesco:auto:dependencies -->
## Dependencies
...
<!-- /nesco:auto:dependencies -->

<!-- nesco:human:architecture -->
## Architecture
(Developer-authored content here)
<!-- /nesco:human:architecture -->
```

**Marker format (Cursor .mdc files):**
```yaml
# nesco:auto:tech-stack
---
description: Tech stack context
alwaysApply: true
---
...
# /nesco:auto:tech-stack
```

**Reconciler rules:**
1. Parse existing file into named sections using markers
2. For each `nesco:auto:*` section: replace content with emitter output
3. For each `nesco:human:*` section: preserve verbatim
4. Append new auto sections that didn't exist in previous version
5. Sections without markers are treated as human-authored (safe default)

**Implications:**
- Opening AND closing markers needed (so reconciler knows where sections end)
- Section names must be stable across scans (use category enum, not freeform)
- Sections can be interleaved — auto and human sections in any order
- The drift engine benefits from section-level granularity (surgical updates, clean git diffs)
- Unmarked content in existing files is preserved (graceful handling of pre-nesco files)

---

### 16. Import: Read-Only Into Memory

**Question:** When importing provider content, where does the parsed data go?

**Options considered:**
- **Canonical model in memory** — Import parses → ContextDocument in memory. Emit, analyze, or inspect from there. Nothing persisted unless explicitly requested.
- **Write to .nesco/imported/** — Persistent storage of parsed content.
- **Write to content repo** — Convert into nesco repo format.

**Decision:** Canonical model in memory.

**Import flow:**
```
nesco import --from cursor
  1. Provider.DiscoveryPaths(cursor, *) → find all Cursor content files
  2. Classify each file by content type (rule, hook, MCP config, etc.)
  3. Parse each file → ContextDocument sections
  4. Report: "Found 12 rules, 3 hooks, 1 MCP config. 2 unclassified."
  5. ContextDocument is in memory — pipe to emitter, parity, or stdout
```

**`nesco import --from cursor --preview`** — Steps 1-4 only, no parsing. Just discovery and classification report.

**`nesco import --from cursor --json`** — Full parse, output canonical ContextDocument as JSON.

**Rationale:** Import is a read operation — it answers "what does this provider have?" It shouldn't create state or files as a side effect. The user decides what to do with the result: pipe to parity analysis, emit to another format (Phase 4), or just inspect. Keeping import stateless simplifies the mental model.

**Implications:**
- `internal/parse/` has one parser per provider — reads provider-specific format → ContextDocument
- Parsers use Provider.DiscoveryPaths() to find content (from Decision #3)
- No persistent storage from import — ContextDocument lives in memory for the duration of the command
- Parity analysis (`nesco parity`) calls import internally for each detected provider

