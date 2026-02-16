# Nesco — Implementation Reference (Carried from v1 Design Doc)

**February 2026**

This document captures implementation patterns and decisions from the original nesco design doc (v1) that remain valid under the v2 vision. The v2 vision doc is the source of truth for *what* nesco does. This document preserves *how* certain pieces were designed at the implementation level, for reference during the brainstorm-to-implementation process in Claude Code.

Not everything here will survive contact with the real implementation — it's a reference, not a spec.

---

## Carries Over Cleanly

These patterns were designed for v1's narrower scope but apply directly to v2's expanded scope without modification.

### Dual-Audience Interface Design

Every command works for both humans at a terminal and LLM agents calling via bash or MCP.

**JSON output mode** — Every command that produces output has a `--json` flag. The JSON schema is treated as an API contract — additive changes only within a major version. Breaking changes require a major version bump.

**MCP server mode** — `nesco mcp` starts a stdio MCP server. Thin wrapper over the same Go functions the CLI uses. Agents discover tools dynamically via input schemas.

**LLM-optimized help** — One-line description per command (for LLM tool selection), structured parameter documentation, concrete examples, documented exit codes. `nesco info` outputs a machine-readable capability manifest.

**Semantic exit codes:**

- 0 — Success
- 1 — User-correctable error (bad flags, missing config)
- 2 — Scan issue (no detectable project, ambiguous results)
- 3 — Drift detected (only in `--ci` mode)
- 4+ — Internal errors

Error output in JSON mode includes `code`, `message`, and `suggestion` fields.

### Format Auto-Detection

Don't dump context files into repos that don't use those tools. Before emitting, check which AI tools are in use via filesystem fingerprints:

- **Claude Code** — `.claude/` directory or `CLAUDE.md` present
- **Cursor** — `.cursor/` directory, `.cursorignore`, or `.cursorrules`
- **Copilot** — `.github/copilot-instructions.md` or Copilot config
- **Gemini CLI** — `GEMINI.md` or `.gemini/` directory
- **Windsurf** — (fingerprints TBD during implementation)
- **Codex** — (fingerprints TBD during implementation)

**First run:** Detect → prompt for confirmation → save selection to `.nesco/config.json`.
**Subsequent runs:** Read config, emit only configured formats. No prompt.
**Overrides:** `--format` targets single format. `--all` emits everything. `--yes` skips prompts. JSON/MCP mode never prompts.

### Detector Interface Pattern

Detectors are independent functions. Each gets a read-only view of the filesystem, returns zero or more sections. No shared state between detectors.

```go
type Detector interface {
    Name() string
    Detect(root string) ([]model.ContextSection, error)
}
```

**Execution:** All detectors run in parallel (goroutines). A detector that panics or times out (5 second default per detector) is skipped with a warning, never blocks others. Target: under 2 seconds total for a typical repo.

### Emitter as Pure Function

Emitters take a ContextDocument, produce a formatted string. No filesystem access, no side effects — the caller handles writing files.

```go
type Emitter interface {
    Name() string
    Format() string
    Emit(doc model.ContextDocument) (string, error)
}
```

### Parser Interface

For import/conversion, parsers read platform-specific formats into the canonical ContextDocument.

```go
type Parser interface {
    Name() string
    Format() string
    Parse(content string) (model.ContextDocument, error)
}
```

### Boundary Markers

Comment markers in emitted files distinguish auto-maintained from human-authored content:

```markdown
## Tech Stack
<!-- nesco:auto — safe to regenerate -->
- TypeScript 5.3 / Node.js 20

## Architecture
<!-- nesco:human — manually maintained, never overwritten -->
(Developer-authored content here)
```

Comment format is platform-appropriate — HTML comments for markdown, YAML comments for `.mdc`.

Regeneration only overwrites `nesco:auto` sections. `nesco:human` sections are preserved verbatim. New auto-detected sections are appended.

### Global Flags and Environment Variables

**Flags:** `--json`, `--no-color`, `--quiet`, `--verbose`, `--version`, `--help`

**Environment:**
- `NESCO_NO_PROMPT=1` — Never prompt, use defaults or fail
- `NESCO_CONFIG_DIR` — Override `.nesco/` location
- `NO_COLOR=1` — Standard no-color convention

### Distribution

- `go install github.com/holden/nesco@latest`
- GitHub Releases with prebuilt binaries via GoReleaser (Linux, macOS, Windows)
- Homebrew tap: `brew install holden/tap/nesco`
- MCP server registration in community directories

### Build Pipeline

- `make build` — local binary
- `make test` — unit tests (detectors tested against fixture directories)
- `make lint` — golangci-lint
- `make release` — GoReleaser cross-platform
- CI: GitHub Actions — test + lint on PR, release on tag

### Minimal Dependencies

- `cobra` — CLI framework
- `mcp-go` or hand-rolled — MCP server protocol (stdio JSON-RPC)
- Standard library for everything else

No database, no HTTP server, no external services. Single static binary.

---

## Partially Carries Over

These patterns are valid but designed for v1's narrower scope. They need expanding for v2.

### Core Data Flow

The scan→ContextDocument→emitter pipeline is still the backbone. What v2 adds:

- **Third entry point** — The author path (write in nesco's format, emit everywhere) alongside scan and import
- **Skill-driven interview layer** — Sits on top of the scan path, interprets surprises, asks targeted questions
- **Runtime context serving** — MCP server does more than expose CLI commands; it answers "what should I know about this directory?"

The v1 data flow diagram needs expanding to show these additional paths.

### MVP Detector List

These v1 detectors map to v2's "deterministic context" category:

**Tier 1 (file parsing, 95%+ reliable):**
- Tech stack — package.json, go.mod, Cargo.toml, pyproject.toml, etc.
- Build commands — package.json scripts, Makefile, Taskfile, justfile
- Dependencies — Lockfiles and manifests, top-level only, grouped by category
- Directory structure — Tree walk respecting .gitignore, conventional patterns
- Project metadata — README, LICENSE, CI setup

**Tier 2 (heuristic, 65-85% reliable):**
- File conventions — Casing patterns, index/barrel files, test colocation
- Code style — Presence of .eslintrc, .prettierrc, .editorconfig, etc. (Note: v2's design review recommended extracting actual rules from .editorconfig as Tier 1, not just reporting tool presence)

What v2 adds: **Surprise detectors** — a parallel concern. These find inconsistencies, competing conventions, and assumption-breakers rather than documenting facts. The surprise detection taxonomy is a research follow-up in v2.

### Drift Engine

Hash-comparison against baseline for deterministic facts still applies. What v2 adds:

- **Surprise-aware drift** — When new inconsistencies appear (not just stale facts), surface them
- **Multiple trigger points** — On demand, CI check, pre-commit hook, MCP server flagging
- **PR commenting** — CI integration that comments on PRs introducing new surprises

### CLI Command Surface

Commands that carry over: `scan`, `drift`, `config`, `info`, `mcp`.

What v2 changes/adds:
- `convert` may become `import` (import pulls in, author+emit pushes out)
- MCP server tools expand beyond scan/drift/convert/info to include runtime context queries
- No explicit command surface for the skills layer (skills are driven by the user's agent, not CLI commands)

### Project Structure Skeleton

The v1 `internal/` layout is a reasonable starting point:

```
internal/
├── model/       # ContextDocument, ContextSection types
├── scan/        # Scanner + detectors
├── emit/        # Emitters (pure functions)
├── parse/       # Parsers (for import)
├── drift/       # Baseline + diff
├── detect/      # Format auto-detection
├── config/      # .nesco/config.json
├── mcp/         # MCP stdio server
└── output/      # JSON vs human-readable switching
```

Needs additions for v2: surprise detection, content management (8 content types), reconciler (who writes files and protects boundaries), and possibly skill coordination.

---

## Superseded

These v1 decisions have been replaced by v2.

| v1 Decision | Replaced By |
|---|---|
| Three-tier confidence (auto/heuristic/human) | Two-category split: deterministic (auto-maintained) vs curated (protected) |
| "AI Context File Generator & Sync Tool" framing | "Agent onboarding assistant" — scan-and-generate is one piece, not the whole product |
| 5 output formats only | 8 content types across 6 providers |
| "No LLM required" as blanket statement | "Dumb tool + smart skill" — CLI is no-LLM, skills layer deliberately uses user's LLM |
| LLM enrichment as future consideration | Replaced by skill-driven guided interview |
| Custom detectors as future plugin system | Addressed by contribution model (structured artifacts, not code) |

---

## Emitter-Specific Notes

These v1 implementation notes about specific output formats are still useful reference:

**CLAUDE.md** — Markdown with optional `@path` imports for large projects. Generates child CLAUDE.md files for subdirectories with distinct concerns. Respects 32KB practical limit by pruning lower-priority sections.

**AGENTS.md** — Pure markdown, most permissive format. Everything in ContextDocument gets included. 32KB soft limit.

**Cursor (.mdc)** — `.cursor/rules/*.mdc` files. Each section category becomes a separate rule file with YAML frontmatter specifying `alwaysApply: true` or glob patterns for scoped rules.

**Copilot** — `.github/copilot-instructions.md`. Single markdown file.

**GEMINI.md** — Markdown in project root. Similar to AGENTS.md structure.

**Conversion is inherently lossy** for platform-specific features. Cursor glob patterns become section headers in CLAUDE.md. Nesco warns about what couldn't be mapped.

---

## Drift Engine Details

**Baseline snapshot:** `.nesco/baseline.json` — serialized ContextDocument with only auto-detected sections. Contains section hashes for efficient comparison.

**Drift detection flow:** Re-run detectors → produce fresh ContextDocument → diff each section against baseline by hash → report unchanged/modified/new/removed.

**What drift does NOT do:** Auto-update files. Touch curated sections. Judge whether a change matters. It reports; the human or agent decides.

**CI integration:** `nesco drift --ci` exits code 3 if drift detected (not code 1 — that's user error). Same pattern as linting/formatting checks.
