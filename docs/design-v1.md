# nesco — AI Context File Generator & Sync Tool

**Design Document — February 2026**

Product: **Nesco** | CLI command: **nesco** | Language: Go
Status: Design complete, ready for implementation

---

## What nesco Does

nesco scans a codebase, detects its conventions, and generates context files for AI coding tools — CLAUDE.md, AGENTS.md, Cursor rules, Copilot instructions, and GEMINI.md. It also converts between formats and detects when context files have drifted out of sync with the codebase.

No LLM required. Purely mechanical detection. Single static binary. Works offline, in CI, and as an MCP server that LLM agents can call directly.

nesco is an independent tool. Nesco is branding heritage, not a dependency.

---

## Design Principles

**Dual-audience from day one.** Every command works for both humans at a terminal and LLM agents calling via bash or MCP. Humans get readable output by default; agents get `--json` structured data or MCP tool calls. This isn't an afterthought — it's the primary architectural constraint.

**Detect what's detectable, leave space for what isn't.** The research shows ~60% of useful codebase context is mechanically extractable (tech stack, build commands, dependencies, directory structure). The other ~40% requires human knowledge (architecture rationale, domain terminology, gotchas). nesco handles the 60% and generates clear placeholders for the 40%, with boundary markers so regeneration never overwrites human-authored content.

**Formats are an API contract.** JSON output schemas are versioned. Additive changes only within a major version. Breaking changes require a major version bump. This is critical because LLM agents parse the output programmatically — a changed field name breaks their workflows silently.

---

## Core Data Flow

nesco has two entry points that converge on a single intermediate representation:

**Scan path:** `nesco scan` analyzes a codebase and produces a `ContextDocument` — an ordered list of context sections, each tagged with a confidence tier:

- ✅ `auto` — Detected from file parsing, 95%+ reliable, safe to regenerate
- ⚠️ `heuristic` — Detected from patterns, 65-85% reliable, needs human verification
- 🔴 `human` — Cannot be auto-detected, placeholder for human input

Detectors are independent functions that each examine one aspect of the codebase and contribute sections to the document.

**Convert path:** `nesco convert --from cursor` parses an existing context file into the same `ContextDocument` representation. Converting *from* any supported format is a parser, converting *to* any format is an emitter. Parsers and emitters are independent — adding a new platform means writing one parser and one emitter.

**Emit:** Both paths feed into the same emitter layer. `nesco scan --format claude` runs detectors then emits CLAUDE.md. `nesco scan --all` emits every format. `nesco convert --from cursor --to claude` parses Cursor rules then emits CLAUDE.md. In JSON mode, the raw `ContextDocument` is returned instead of a rendered file.

**Drift:** `nesco drift` re-runs the scan detectors, compares against a stored baseline (`.nesco/baseline.json`), and reports what changed. Only ✅ sections are compared — ⚠️ and 🔴 sections are human-managed and excluded from drift detection.

**State storage:** `.nesco/` directory in project root holds baseline snapshots and config. Git-tracked by default so drift baselines are shared across contributors.

```
Codebase ──→ [Detectors] ──→ ContextDocument ──→ [Emitters] ──→ CLAUDE.md
                                    ↑                            AGENTS.md
Existing file ──→ [Parsers] ────────┘                            .cursor/rules/*.mdc
                                                                 copilot-instructions.md
                                                                 GEMINI.md
                                    ↓
                              .nesco/baseline.json ←──→ [Drift Engine]
```

---

## Format Auto-Detection

nesco doesn't dump five context files into a repo that only uses Claude Code. Before emitting, it checks which AI tools are actually in use:

- **CLAUDE.md** — `.claude/` directory exists, or `CLAUDE.md` already present
- **AGENTS.md** — `AGENTS.md` already present, or `.github/copilot-instructions.md` exists
- **Cursor** — `.cursor/` directory, `.cursorignore`, or `.cursorrules` present
- **Copilot** — `.github/copilot-instructions.md` exists, or `.github` directory with Copilot config
- **GEMINI.md** — `GEMINI.md` already present, or `.gemini/` directory

**First run:** `nesco scan` → detects which tools are present → prompts for confirmation ("I found Cursor and Claude Code. Generate context for these? Add others?") → user confirms or adjusts → emits only selected formats → saves selection to `.nesco/config.json`.

**Subsequent runs:** Reads config, emits only configured formats. No prompt.

**Overrides:** `--format claude` targets a single format regardless of config. `--all` emits everything. `--yes` skips prompts for CI and LLM use. JSON/MCP mode never prompts — returns detected formats in the response and lets the caller decide.

---

## Detector Design

Detectors are independent functions that each examine one aspect of the codebase. They share no state with each other — each gets a read-only view of the filesystem and returns its findings.

**Detector interface:** Every detector takes a project root path, returns zero or more `ContextSection` structs. Each section has a category, content, and confidence tier. A detector that finds nothing returns empty — no error, no placeholder.

### MVP Detectors — Tier 1 (file parsing, 95% reliable)

**Tech stack** — Reads `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`, `Gemfile`, `pom.xml`, `build.gradle`, and similar manifests. Extracts language, framework, version. Falls back to file extension census if no manifest found.

**Build commands** — Extracts scripts from `package.json`, `Makefile`, `Taskfile`, `justfile`, `Cargo.toml`, `pyproject.toml`. Maps to "how to build/test/lint/run."

**Dependencies** — Reads lockfiles and manifests. Extracts top-level deps, not transitive. Groups by category where possible (framework, testing, linting, database).

**Directory structure** — Walks the tree (respecting `.gitignore`), identifies conventional patterns: `src/`, `lib/`, `test/`, `cmd/`, `internal/`, `api/`, `migrations/`. Generates a pruned structural overview.

**Project metadata** — Reads README.md for project description, LICENSE for license type, `.github/` for CI setup.

### MVP Detectors — Tier 2 (heuristic, 65-85% reliable)

**File conventions** — Samples filenames across directories to detect casing patterns (camelCase, kebab-case, PascalCase). Checks for index/barrel files, test file colocation vs separate directory.

**Code style** — Detects presence of `.eslintrc`, `.prettierrc`, `rustfmt.toml`, `.editorconfig`, `ruff.toml`. Reports the tools, not the rules — "uses ESLint + Prettier" not "tabs vs spaces."

### Not in MVP — Tier 3 (needs human or LLM)

Architecture patterns, domain terminology, module responsibilities, gotchas. These appear as 🔴 placeholder sections with prompts like "Describe the architecture pattern used in this project."

### Execution Model

All detectors run in parallel (goroutines), results collected and ordered by category. A detector that panics or times out (5 second limit per detector) is skipped with a warning, never blocks the others. Total scan time target: under 2 seconds for a typical repo.

---

## Emitter Layer

Emitters take a `ContextDocument` and produce a platform-specific output file. Each emitter is a pure function: `ContextDocument` in, formatted string out. No filesystem access, no side effects — the caller handles writing files.

### MVP Emitters

**CLAUDE.md** — Markdown with optional `@path` imports for large projects. If project has subdirectories with distinct concerns, generates child CLAUDE.md files and imports them from root. Respects 32KB practical limit by pruning lower-priority sections.

**AGENTS.md** — Pure markdown, no special syntax. Most permissive format — essentially everything in the `ContextDocument` gets included. 32KB soft limit.

**Cursor (.mdc)** — Generates `.cursor/rules/*.mdc` files. Each `ContextSection` category becomes a separate rule file with YAML frontmatter specifying `alwaysApply: true` or glob patterns for scoped rules.

**Copilot** — Generates `.github/copilot-instructions.md`. Single markdown file, straightforward mapping.

**GEMINI.md** — Markdown in project root. Similar to AGENTS.md in structure.

### Confidence Tier Rendering

Emitters include boundary markers so humans and nesco itself know what's auto-maintained vs hand-written:

```markdown
## Tech Stack
<!-- nesco:auto — safe to regenerate -->
- TypeScript 5.3 / Node.js 20
- Next.js 14 (App Router)

## Architecture
<!-- nesco:human — manually maintained, never overwritten -->
(Add your architecture description here)
```

Comment format is platform-appropriate — HTML comments for markdown formats, YAML comments for `.mdc` files.

### Regeneration Behavior

`nesco scan` on an existing project with previously generated files only overwrites `nesco:auto` sections. `nesco:human` sections are preserved verbatim. New auto-detected sections are appended. Running `nesco scan` repeatedly never loses human-authored content.

### Format Conversion

`nesco convert --from cursor --to claude` works by: parser reads Cursor rules → produces `ContextDocument` → CLAUDE.md emitter renders it. Conversion is inherently lossy for platform-specific features (Cursor glob patterns become section headers in CLAUDE.md, for example). nesco warns about what couldn't be mapped.

---

## Drift Engine

The drift engine answers: "has your codebase changed in ways that make your context files stale?"

### Baseline Snapshot

When `nesco scan` generates files, it writes `.nesco/baseline.json` — a serialized `ContextDocument` with only ✅ auto-detected sections. The baseline is minimal: tech stack versions, dependency lists, build commands, directory structure hash, detected conventions.

```json
{
  "version": 1,
  "generated_at": "2026-02-15T10:30:00Z",
  "nesco_version": "0.1.0",
  "sections": {
    "tech-stack": { "tier": "auto", "hash": "a1b2c3", "data": {} },
    "build-commands": { "tier": "auto", "hash": "d4e5f6", "data": {} },
    "dependencies": { "tier": "auto", "hash": "g7h8i9", "data": {} },
    "structure": { "tier": "auto", "hash": "j0k1l2", "data": {} }
  }
}
```

### Drift Detection Flow

`nesco drift` re-runs all detectors, produces a fresh `ContextDocument`, diffs each ✅ section against the baseline by hash:

```
$ nesco drift

  Tech Stack
  ✓ No changes

  Dependencies
  ⚠ 3 additions: zod, drizzle-orm, hono
  ⚠ 1 removal: express

  Build Commands
  ⚠ "test" script changed: "jest" → "vitest run"

  Directory Structure
  ⚠ New directories: src/api/, src/middleware/

Run `nesco scan` to update context files.
```

JSON mode returns each section with `status: "unchanged" | "modified" | "new" | "removed"` and specific changes.

### What Drift Does NOT Do

It doesn't auto-update files. It doesn't touch ⚠️ heuristic or 🔴 human sections. It doesn't judge whether a change matters. It reports; the human or LLM agent decides.

### CI Integration

`nesco drift --ci` exits with code 0 if no drift, code 1 if drift detected. Same pattern as linting or formatting checks. Add to CI to fail when context files are stale.

---

## Dual-Audience Interface Design

LLM agents interact with CLIs fundamentally differently than humans. They chain commands, parse structured output, retry based on exit codes, and run parallel operations. nesco is designed for both audiences from day one.

### JSON Output Mode

Every command that produces output has a `--json` flag. `nesco scan --json` returns the canonical `ContextDocument` directly. `nesco drift --json` returns a structured diff. The JSON schema is treated as an API contract — additive changes only, breaking changes require major version bumps.

### MCP Server Mode

`nesco mcp` starts a stdio MCP server exposing tools: `scan`, `drift`, `convert`, `info`. An agent in Claude Code or Cursor discovers nesco's tools dynamically, sees input schemas, and calls them without constructing shell commands. The MCP server is a thin wrapper over the same Go functions the CLI uses.

### LLM-Optimized Help

Each command includes a one-line description (for LLM tool selection), structured parameter documentation, concrete examples, and documented exit codes. `nesco info` outputs a machine-readable capability manifest.

### Semantic Exit Codes + Structured Errors

- 0 — Success
- 1 — User-correctable error (bad flags, missing config)
- 2 — Scan issue (no detectable project, ambiguous results)
- 3 — Drift detected (only in `--ci` mode)
- 4+ — Internal errors

Error output in JSON mode includes `code`, `message`, and `suggestion` fields so an LLM can self-correct without human intervention.

---

## CLI Command Surface

### Core Commands

**`nesco scan`** — Detect codebase conventions, generate context files.
Flags: `--format <name>`, `--all`, `--json`, `--dry-run`, `--yes`

**`nesco drift`** — Compare current codebase state against baseline.
Flags: `--json`, `--ci`

**`nesco convert --from <format> --to <format>`** — Parse existing context file, emit in different format.
Flags: `--json`, `--output <path>`

### Configuration Commands

**`nesco config formats`** — View or edit active formats. `add`, `remove`, `list` subcommands.

**`nesco config init`** — Interactive first-run setup without scanning.

### Introspection Commands

**`nesco info`** — Machine-readable capability manifest. Detectors available, formats supported, current config, baseline status.

**`nesco info formats`** — List supported output formats.

**`nesco info detectors`** — List available detectors.

### Global Flags

`--json`, `--no-color`, `--quiet`, `--verbose`, `--version`, `--help`

### Environment Variables

- `NESCO_NO_PROMPT=1` — Never prompt, use defaults or fail
- `NESCO_CONFIG_DIR` — Override `.nesco/` location
- `NO_COLOR=1` — Standard no-color convention

### MCP Surface

`nesco mcp` starts a stdio MCP server exposing:

- `scan` — `{format?: string, all?: boolean}` → generated file contents or ContextDocument
- `drift` — `{}` → structured drift report
- `convert` — `{from: string, to: string, content?: string}` → converted file
- `info` — `{topic?: "formats" | "detectors" | "config"}` → capability manifest

---

## Project Structure

```
nesco/
├── cmd/
│   └── nesco/
│       └── main.go              # CLI entrypoint, cobra commands
├── internal/
│   ├── model/
│   │   └── document.go          # ContextDocument, ContextSection types
│   ├── scan/
│   │   ├── scanner.go           # Orchestrates detectors, parallel execution
│   │   ├── detectors/
│   │   │   ├── techstack.go
│   │   │   ├── build.go
│   │   │   ├── dependencies.go
│   │   │   ├── structure.go
│   │   │   ├── metadata.go
│   │   │   └── conventions.go
│   │   └── detector.go          # Detector interface
│   ├── emit/
│   │   ├── emitter.go           # Emitter interface
│   │   ├── claude.go
│   │   ├── agents.go
│   │   ├── cursor.go
│   │   ├── copilot.go
│   │   └── gemini.go
│   ├── parse/
│   │   ├── parser.go            # Parser interface
│   │   ├── claude.go
│   │   ├── agents.go
│   │   ├── cursor.go
│   │   ├── copilot.go
│   │   └── gemini.go
│   ├── drift/
│   │   ├── baseline.go          # Read/write .nesco/baseline.json
│   │   └── diff.go              # Compare ContextDocuments
│   ├── detect/
│   │   └── formats.go           # Auto-detect which AI tools are in use
│   ├── config/
│   │   └── config.go            # .nesco/config.json read/write
│   ├── mcp/
│   │   └── server.go            # MCP stdio server
│   └── output/
│       └── output.go            # JSON vs human-readable switching
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Key Interfaces

```go
type Detector interface {
    Name() string
    Detect(root string) ([]model.ContextSection, error)
}

type Emitter interface {
    Name() string
    Format() string
    Emit(doc model.ContextDocument) (string, error)
}

type Parser interface {
    Name() string
    Format() string
    Parse(content string) (model.ContextDocument, error)
}
```

### Dependencies — Minimal

- `cobra` — CLI framework
- `mcp-go` or hand-rolled — MCP server protocol (stdio JSON-RPC)
- Standard library for everything else

No database, no HTTP server, no external services. Single static binary.

### Distribution

- `go install github.com/holden/nesco@latest`
- GitHub Releases with prebuilt binaries via GoReleaser (Linux, macOS, Windows)
- Homebrew tap: `brew install holden/tap/nesco`
- MCP server registration in community directories

### Build

- `make build` — local binary
- `make test` — unit tests (detectors tested against fixture directories)
- `make lint` — golangci-lint
- `make release` — GoReleaser cross-platform
- CI: GitHub Actions — test + lint on PR, release on tag

---

## Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Repository | Fresh repo | nesco is independent, not an extension |
| Primary use case | Scan + convert via shared emitter layer | Same ContextDocument model serves both paths |
| Maintenance scope | Generation + drift detection | Auto-update too risky for MVP |
| Output formats | CLAUDE.md, AGENTS.md, Cursor, Copilot, GEMINI.md | Covers major AI coding tools |
| LLM requirement | Purely mechanical | Ship fast, deterministic, zero cost, add LLM enrichment later |
| Nesco relationship | Independent tool, branding only | No ecosystem dependency |
| CLI name | `nesco` | 5 chars, derived from nesco, no meaningful collisions in Go/CLI space |
| Format auto-detection | Detect + confirm on first run, save to config | Don't clog repos with unused files |

---

## Future Considerations (Not MVP)

- **LLM enrichment** — Optional `--enrich` flag that runs an LLM pass to fill in architecture descriptions and module explanations. Same boundary markers distinguish LLM-generated from mechanical content.
- **Watch mode** — `nesco watch` monitors filesystem changes and warns about drift in real time.
- **IDE plugins** — VS Code extension that shows drift status and offers one-click regeneration.
- **Custom detectors** — User-defined detectors via plugin system or config-driven rules.
- **Workflow-to-workflow** — nesco as a component in larger Nesco-generated workflows.
