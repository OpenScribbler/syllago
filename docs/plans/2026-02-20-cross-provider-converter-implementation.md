# Cross-Provider Content Conversion Pipeline — Implementation Plan

## Context

Syllago stores AI coding tool content in a repo and installs to provider-specific locations. Currently, installation just copies or symlinks files — no format transformation happens. When a Claude Code rule needs to go to Cursor (.mdc format) or a Claude agent needs to go to Codex (flat AGENTS.md), the content is copied as-is, producing broken or incompatible output.

This plan adds a **converter package** with a hub-and-spoke architecture: content is normalized to a canonical format on import (with the original preserved in `.source/`), and rendered to provider-specific formats on export.

Background research: `docs/cross-provider-conversion-reference.md`

## Architecture: Source-Preserving Canonical Model

```
External source (Cursor .mdc)
    ↓ syllago add --type rules --provider cursor
    ↓   1. Copy original → .source/rule.mdc (verbatim preservation)
    ↓   2. Parse source → RuleMeta → canonical rule.md (with frontmatter)
    ↓   3. Record source_provider in .syllago.yaml
rules/cursor/my-rule/
    rule.md              ← canonical format (YAML frontmatter + markdown body)
    .source/rule.mdc     ← original file (preserved verbatim)
    .syllago.yaml          ← metadata (source_provider: cursor, source_format: mdc)

    ↓ syllago export --to windsurf
    ↓   Reads canonical rule.md → renders with Windsurf trigger: field
~/.codeium/windsurf/rules/my-rule/rule.md

    ↓ syllago export --to cursor
    ↓   Detects target == source_provider → copies .source/rule.mdc verbatim (lossless!)
~/.cursor/rules/my-rule/rule.mdc
```

### Why This Approach
- **No data loss ever** — original preserved in `.source/`
- **One parse format for export** — all renderers read from canonical, never from arbitrary provider formats
- **Lossless roundtrip to source provider** — use `.source/` original directly
- **Adding new providers** — only need a renderer, not a parser (parsers only needed for import)
- **Auditability** — diff canonical vs original to see what was normalized

### Canonical Rule Format

`rule.md` with YAML frontmatter containing the universal rule activation fields:

```yaml
---
description: "Rule description"
alwaysApply: true
globs:
  - "*.ts"
  - "*.tsx"
---

# Rule Title

Rule content in markdown...
```

This is the superset of all provider rule formats. The frontmatter fields map to every provider's activation semantics (see Rosetta Stone in reference doc).

## Scope

**Phase 1 (this plan)**: Rules converter + infrastructure. Rules are the most complex content type and the most valuable — every provider has a different format.

**Future phases** (separate plans): Skills, commands, agents, MCP env vars, hooks.

## Files to Create

### `cli/internal/converter/converter.go` — Core types + registry

```go
package converter

// Result holds converted content and data loss warnings.
type Result struct {
    Content  []byte   // Transformed bytes (nil = skip this item)
    Filename string   // Output filename (e.g. "rule.mdc" for Cursor)
    Warnings []string // Human-readable data loss messages
}

// Converter transforms content for a target provider.
type Converter interface {
    // Canonicalize converts provider-specific content to canonical format.
    // Used during import/add.
    Canonicalize(content []byte, sourceProvider string) (*Result, error)

    // Render converts canonical content to a target provider's format.
    // Used during export/install.
    Render(content []byte, target provider.Provider) (*Result, error)

    // ContentType returns which content type this converter handles.
    ContentType() catalog.ContentType
}

// Registry functions:
func For(ct catalog.ContentType) Converter        // lookup by content type
func HasSourceFile(item catalog.ContentItem) bool  // check if .source/ exists
func SourceFilePath(item catalog.ContentItem) string // path to .source/ original
```

**Key difference from previous plan**: Two methods instead of one. `Canonicalize()` runs on import, `Render()` runs on export. The previous plan had a single `Convert()` that tried to do both.

### `cli/internal/converter/rules.go` — Rules conversion

**Canonical intermediate** (same as before, used internally):

```go
type RuleMeta struct {
    Description string   `yaml:"description,omitempty"`
    AlwaysApply bool     `yaml:"alwaysApply"`
    Globs       []string `yaml:"globs,omitempty"`
}
```

**Canonicalize parsers** (source provider → canonical):
- `canonicalizeCursorRule(content)` — parse .mdc frontmatter, normalize to canonical YAML
- `canonicalizeWindsurfRule(content)` — parse trigger enum, convert to alwaysApply/globs
- `canonicalizeMarkdownRule(content)` — plain markdown, defaults to alwaysApply: true

**Renderers** (canonical → target provider):
- `renderCursorRule(meta, body)` — YAML frontmatter with alwaysApply/globs/description, `.mdc` filename
- `renderWindsurfRule(meta, body)` — YAML frontmatter with trigger enum, `.md` filename
- `renderSingleFileRule(meta, body, slug)` — body only if alwaysApply; nil Content + warning if not
- `renderMarkdownRule(meta, body)` — plain markdown fallback

**Mapping logic** (from `docs/cross-provider-conversion-reference.md`):
```
Canonical → Cursor:      alwaysApply/globs/description pass through (near-identity)
Canonical → Windsurf:    alwaysApply:true → trigger:always_on
                         globs present    → trigger:glob
                         description only → trigger:model_decision
                         bare false       → trigger:manual
Canonical → Claude/Codex/Gemini: only alwaysApply:true rules, body only (lossy)
Canonical → Cline/Cody:  description only (lossy)

Cursor → Canonical:      alwaysApply/globs/description normalized (near-identity)
Windsurf → Canonical:    trigger:always_on → alwaysApply:true
                         trigger:glob → alwaysApply:false + globs
                         trigger:model_decision → alwaysApply:false + description
                         trigger:manual → alwaysApply:false
```

### `cli/internal/converter/rules_test.go` — Comprehensive tests

Test matrix:
- Cursor → canonical → Windsurf (all 4 trigger types)
- Windsurf → canonical → Cursor (reverse)
- Cursor → canonical → Claude (alwaysApply:true included, false excluded with warning)
- Round-trip: Cursor → canonical → Cursor (lossless via .source/)
- Round-trip: canonical → Windsurf → canonical (lossless)
- Missing frontmatter → defaults to alwaysApply:true
- Warning generation for each data loss scenario

## Files to Modify

### `cli/internal/metadata/metadata.go` — Add source tracking fields

Add to `Meta` struct:

```go
SourceProvider string `yaml:"source_provider,omitempty"` // provider slug the content was imported from
SourceFormat   string `yaml:"source_format,omitempty"`   // file extension (e.g., "mdc", "md")
```

### `cli/cmd/syllago/add.go` — Canonicalize on add + preserve source

After copying content into the repo (current behavior), add:

1. If a converter exists for the content type AND a source provider is specified:
   - Copy the original content file to `.source/` within the item directory
   - Run `Canonicalize()` on the content file to produce canonical format
   - Write canonical output over the content file
   - Set `source_provider` and `source_format` in `.syllago.yaml`

2. If no converter exists or no source provider specified:
   - Existing behavior (copy as-is, no canonicalization)

### `cli/internal/installer/installer.go` — Add render step to Install()

Insert render check before the symlink/copy step (around line 115):

```go
// After JSON merge dispatch, before resolveTarget:
conv := converter.For(item.Type)
if conv != nil && item.Provider != prov.Slug {
    // Source provider differs from target — render from canonical
    return installWithRender(item, prov, conv, repoRoot)
}
// Same provider + has .source/ — use original for lossless install
if conv != nil && converter.HasSourceFile(item) && item.Provider == prov.Slug {
    return installFromSource(item, prov, repoRoot)
}
```

`installWithRender()`:
1. Read canonical content file
2. Call `conv.Render(content, prov)`
3. Handle nil Content (skip) and warnings (print to stderr)
4. Write rendered bytes to target path (always copy, never symlink)

`installFromSource()`:
1. Copy `.source/` original directly to target (lossless for source provider)

### `cli/cmd/syllago/export.go` — Add render to export loop

Insert render check before `installer.CopyContent()` call (around line 157):

```go
conv := converter.For(item.Type)
if conv != nil && item.Provider != "" && item.Provider != toSlug {
    // Cross-provider export: render from canonical
    contentFile := converter.ResolveContentFile(item)
    result, err := conv.Render(readFile(contentFile), *prov)
    // ... handle result, write to dest
    continue
}
if conv != nil && converter.HasSourceFile(item) && item.Provider == toSlug {
    // Same provider: use .source/ original
    // ... copy .source/ file to dest
    continue
}
// Fallback: existing copy behavior
```

Add `Converted bool` and `Warnings []string` fields to `exportedItem` struct.

### `cli/internal/parse/cursor.go` — Export frontmatter parser

Rename `parseMDCFrontmatter` → `ParseMDCFrontmatter` (exported). Update internal call site in `parseMDC()`. The converter reuses this to parse Cursor .mdc during canonicalization.

## Implementation Sequence

### Task 1: Add source tracking to metadata
Add `SourceProvider` and `SourceFormat` fields to `Meta` struct in `cli/internal/metadata/metadata.go`.

**Files:** `cli/internal/metadata/metadata.go`
**Test:** `make vet` passes
**Dependencies:** None

### Task 2: Create converter package core types
Create `cli/internal/converter/converter.go` with `Result` type, `Converter` interface, registry (`For()`, `Register()`), and source file helpers (`HasSourceFile()`, `SourceFilePath()`, `ResolveContentFile()`).

**Files:** `cli/internal/converter/converter.go`
**Test:** `make vet` passes
**Dependencies:** Task 1

### Task 3: Export cursor parser
Rename `parseMDCFrontmatter` → `ParseMDCFrontmatter` in `cli/internal/parse/cursor.go`. Update the internal call site in `parseMDC()`.

**Files:** `cli/internal/parse/cursor.go`
**Test:** `make test` passes (existing tests)
**Dependencies:** None

### Task 4: Implement rules converter — canonicalize parsers
Create `cli/internal/converter/rules.go` with `RulesConverter` struct implementing `Canonicalize()`. Includes parsers for Cursor, Windsurf, and plain markdown sources.

**Files:** `cli/internal/converter/rules.go`
**Test:** `make vet` passes
**Dependencies:** Tasks 2, 3

### Task 5: Implement rules converter — renderers
Add `Render()` method to `RulesConverter` with renderers for Cursor (.mdc), Windsurf (.md), single-file (Claude/Codex/Gemini), and plain markdown fallback.

**Files:** `cli/internal/converter/rules.go`
**Test:** `make vet` passes
**Dependencies:** Task 4

### Task 6: Write converter tests
Create `cli/internal/converter/rules_test.go` with full test matrix covering all conversion paths, round-trips, edge cases, and warning generation.

**Files:** `cli/internal/converter/rules_test.go`
**Test:** `make test` passes — all converter tests green
**Dependencies:** Task 5

### Task 7: Integrate converter into add command
Modify `cli/cmd/syllago/add.go` to canonicalize on add when a converter exists and source provider is specified. Preserve original in `.source/`, update metadata.

**Files:** `cli/cmd/syllago/add.go`
**Test:** `make test && make vet`
**Dependencies:** Tasks 4, 5

### Task 8: Integrate converter into installer
Modify `cli/internal/installer/installer.go` to add `installWithRender()` and `installFromSource()` paths in the `Install()` function.

**Files:** `cli/internal/installer/installer.go`
**Test:** `make test && make vet`
**Dependencies:** Tasks 4, 5

### Task 9: Integrate converter into export command
Modify `cli/cmd/syllago/export.go` to render through converter on cross-provider export and use `.source/` for same-provider export.

**Files:** `cli/cmd/syllago/export.go`
**Test:** `make test && make vet`
**Dependencies:** Tasks 4, 5

### Task 10: Full build and test verification
Run `make test && make vet` to verify everything compiles and tests pass.

**Dependencies:** All previous tasks

## Verification

```bash
# Unit tests
make test

# Manual flow test:
# 1. Create a test Cursor rule
mkdir -p /tmp/test-cursor-rule
cat > /tmp/test-cursor-rule/rule.mdc << 'EOF'
---
description: "Test rule for TypeScript files"
globs: ["*.ts", "*.tsx"]
alwaysApply: false
---

# TypeScript Conventions

Use strict mode. Prefer const over let.
EOF

# 2. Add to syllago
syllago add /tmp/test-cursor-rule --type rules --provider cursor --name test-ts-rule

# 3. Verify canonical + source preservation
cat rules/cursor/test-ts-rule/rule.md        # Should have canonical frontmatter
cat rules/cursor/test-ts-rule/.source/rule.mdc  # Should be verbatim original
cat rules/cursor/test-ts-rule/.syllago.yaml    # Should have source_provider: cursor

# 4. Export to Windsurf
syllago export --to windsurf --type rules --name test-ts-rule
# Verify: output has trigger: glob, .md extension

# 5. Export to Claude
syllago export --to claude-code --type rules --name test-ts-rule
# Verify: SKIPPED with warning (alwaysApply: false, can't express in Claude)

# 6. Export back to Cursor
syllago export --to cursor --type rules --name test-ts-rule
# Verify: uses .source/rule.mdc verbatim (lossless roundtrip)
```

## Key Design Decisions

1. **Source preservation via `.source/`** — original files are never lost. Enables lossless roundtrip to source provider.

2. **Two-method interface** — `Canonicalize()` for import, `Render()` for export. Cleaner than a single `Convert()` that has to handle both directions.

3. **Canonical format uses YAML frontmatter** — same structure as Cursor .mdc (most expressive directory-per-rule format) but with `.md` extension.

4. **Export to source provider uses original** — `installFromSource()` copies `.source/` directly, avoiding any conversion artifacts.

5. **nil Content signals "skip"** — non-alwaysApply rules targeting single-file formats return nil with a warning rather than producing broken output.

6. **Warnings are structured** — each render produces specific messages about data loss. CLI prints to stderr; JSON mode includes them in the response.

7. **Reuse existing parser** — `parse.ParseMDCFrontmatter` is exported rather than duplicated.

8. **Phase 1 is rules only** — most complex, most value. Future phases add skills (trivial), commands, agents, MCP, hooks.
