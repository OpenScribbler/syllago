# Quality Report: Cross-Provider Converter Implementation Plan

**Plan file:** `/home/hhewett/.local/src/nesco/docs/plans/2026-02-20-cross-provider-converter-implementation.md`
**Reviewed:** 2026-02-20

---

## Pre-Review Finding: Implementation Already Complete

Before assessing the plan's task quality, a significant fact must be noted: **the implementation described by this plan already exists in the codebase and all tests pass.**

Verified state:
- `make vet` — passes with no errors
- `make test` — all packages pass, including `cli/internal/converter`

Existing files that implement the plan:
- `/home/hhewett/.local/src/nesco/cli/internal/converter/converter.go` — fully implemented
- `/home/hhewett/.local/src/nesco/cli/internal/converter/rules.go` — fully implemented
- `/home/hhewett/.local/src/nesco/cli/internal/converter/rules_test.go` — full test matrix passing
- `/home/hhewett/.local/src/nesco/cli/internal/metadata/metadata.go` — `SourceProvider` and `SourceFormat` fields already present (lines 39-40)
- `/home/hhewett/.local/src/nesco/cli/cmd/nesco/add.go` — `canonicalizeContent()` already integrated
- `/home/hhewett/.local/src/nesco/cli/internal/installer/installer.go` — `installWithRender()` and `installFromSource()` already integrated
- `/home/hhewett/.local/src/nesco/cli/cmd/nesco/export.go` — `exportWithConverter()` already integrated
- `/home/hhewett/.local/src/nesco/cli/internal/parse/cursor.go` — `ParseMDCFrontmatter` already exported

This means the plan is a **retrospective document** — it describes completed work. The quality assessment below evaluates the plan's usefulness as documentation and as a template for future implementation plans.

---

## Quality Assessment

### 1. Granularity (Task Duration 2-5 Minutes Each)

**FAIL — Tasks are too coarse.**

The plan defines 10 tasks, but several are multi-hour efforts disguised as single steps:

- **Task 4 (Canonicalize parsers):** Implementing three separate parsers (`canonicalizeCursorRule`, `canonicalizeWindsurfRule`, `canonicalizeMarkdownRule`), each with different parsing logic, glob handling, and edge cases, is at minimum 30-60 minutes of work. It should be split into at minimum three tasks.
- **Task 5 (Renderers):** Four separate renderers with non-trivial logic (trigger-enum mapping, nil-Content signaling, glob serialization). Another 30-60 minutes. Should be split.
- **Task 6 (Write converter tests):** The test file has 12 distinct test functions covering 7 different scenarios. Writing and verifying all of them at once is not 2-5 minutes.
- **Task 8 (Integrate into installer):** Requires understanding the installer's dispatch flow, writing two new functions (`installWithRender`, `installFromSource`), and not breaking any existing tests. More than 5 minutes.
- **Task 9 (Integrate into export):** The actual `exportWithConverter` function has 50+ lines with three distinct code paths. Not a 5-minute task.

A correct granularity breakdown for Task 4-5 alone would be:
- 4a: Implement `canonicalizeCursorRule` (uses existing `ParseMDCFrontmatter`)
- 4b: Implement `canonicalizeWindsurfRule` (new frontmatter struct, trigger enum parsing)
- 4c: Implement `canonicalizeMarkdownRule` (plain markdown default case)
- 5a: Implement `renderCursorRule` and `renderWindsurfRule`
- 5b: Implement `renderSingleFileRule` (nil-Content pattern, warning generation)
- 5c: Implement `renderMarkdownRule`, `buildCanonical`, `renderFrontmatter` helpers

### 2. Specificity (No TBDs, Vague Descriptions, or Placeholders)

**PASS with minor gaps.**

The plan is largely specific. Architecture descriptions, conversion mapping tables, and code snippets are concrete. No "TBD" or placeholder text appears.

Minor gaps:
- Task 1 says "Add `SourceProvider` and `SourceFormat` fields" but does not include the exact struct field definitions with YAML tags. The "Files to Modify" section above the tasks does include them, but this creates a split-attention problem during execution. The field definitions should appear inline in the task.
- Tasks 7-9 reference "around line 115" and "around line 157" as insertion points in `installer.go` and `export.go`. These are inherently fragile — line numbers shift with any edit. The plan should use function names or structural landmarks ("after the JSON merge dispatch block in `Install()`") rather than line numbers.
- The `canonicalizeContent()` helper in `add.go` and `findContentFile()` helper are not described in the plan at all. The plan says to "Add" a block of code inline, but the actual implementation extracted those into named helpers. Someone executing this plan would not know to create those helpers.

### 3. Dependencies (Implicit Dependencies Explicitly Stated)

**PASS with one gap.**

Most dependencies are correctly captured. The sequence (Task 1 → 2 → 3 → 4 → 5 → 6, with 7/8/9 depending on 4+5) is sound.

One implicit dependency missing: Task 6 (write tests) depends on `provider.Windsurf`, `provider.Cursor`, `provider.ClaudeCode`, and `provider.Codex` being exported package-level variables. The test file imports `github.com/OpenScribbler/nesco/cli/internal/provider` and uses these directly. The plan does not mention verifying that these named provider variables exist, or note that the test depends on them. A developer executing the plan should be told: "Verify that `provider.Windsurf`, `provider.Cursor`, `provider.ClaudeCode`, and `provider.Codex` are exported variables in the provider package before writing tests."

### 4. TDD Structure (Test → Fail → Implement → Pass → Commit)

**FAIL — TDD rhythm is reversed in key tasks.**

The plan places tests in Task 6, after implementation in Tasks 4 and 5. This is implement-then-test, not TDD.

Tasks 4 and 5 both say `make vet passes` as their verification — they have no failing test to drive the implementation. This means there is no red-green cycle for the core conversion logic.

Correct TDD sequence would be:
- Write one failing test for `canonicalizeCursorRule` → implement → pass → commit
- Write one failing test for `canonicalizeWindsurfRule` → implement → pass → commit
- etc.

Alternatively, if the intent is to write all tests first (Tasks 4/5 deferred until Task 6), the test task should come *before* the implementation tasks, and implementation tasks should cite "makes Task 6 tests pass" as their verification criterion.

### 5. Complete Code (Actual Snippets, Not Placeholders)

**PASS with one significant gap.**

The architecture section and "Files to Create" section contain accurate, compilable code snippets for the `Result` type, `Converter` interface, and `RuleMeta` struct. The conversion mapping table is precise and complete.

Significant gap: The "Files to Modify" sections for `add.go`, `installer.go`, and `export.go` show partial pseudo-code with narrative placeholders:

```go
// installWithRender():
// 1. Read canonical content file
// 2. Call conv.Render(content, prov)
// 3. Handle nil Content (skip) and warnings (print to stderr)
// 4. Write rendered bytes to target path (always copy, never symlink)
```

This is a bulleted description, not actual code. A plan of this quality standard should show the actual function signature and key implementation steps in Go. The `exportWithConverter` snippet in the export section is similarly a skeleton — it shows the outer if/continue structure but omits the actual file-writing logic.

The `canonicalizeContent()` helper and `findContentFile()` helper that appear in the actual `add.go` implementation are absent from the plan entirely. If someone followed the plan literally, they would write a different (less clean) implementation.

### 6. Exact Paths (Full File Paths for All Files Mentioned)

**FAIL — Paths are relative throughout.**

Every file path in the plan is relative (e.g., `cli/internal/converter/converter.go`, `cli/cmd/nesco/add.go`). The project instructions explicitly require absolute paths.

Complete list of relative paths that should be absolute:
- `cli/internal/converter/converter.go` → `/home/hhewett/.local/src/nesco/cli/internal/converter/converter.go`
- `cli/internal/converter/rules.go` → `/home/hhewett/.local/src/nesco/cli/internal/converter/rules.go`
- `cli/internal/converter/rules_test.go` → `/home/hhewett/.local/src/nesco/cli/internal/converter/rules_test.go`
- `cli/internal/metadata/metadata.go` → `/home/hhewett/.local/src/nesco/cli/internal/metadata/metadata.go`
- `cli/cmd/nesco/add.go` → `/home/hhewett/.local/src/nesco/cli/cmd/nesco/add.go`
- `cli/internal/installer/installer.go` → `/home/hhewett/.local/src/nesco/cli/internal/installer/installer.go`
- `cli/cmd/nesco/export.go` → `/home/hhewett/.local/src/nesco/cli/cmd/nesco/export.go`
- `cli/internal/parse/cursor.go` → `/home/hhewett/.local/src/nesco/cli/internal/parse/cursor.go`
- `docs/cross-provider-conversion-reference.md` → `/home/hhewett/.local/src/nesco/docs/cross-provider-conversion-reference.md`

The verification script also uses relative paths (`rules/cursor/test-ts-rule/rule.md`) that would fail unless the script is run from the repo root — the repo root should be explicit.

---

## Summary of Issues

| Check | Result | Issue |
|-------|--------|-------|
| Granularity (2-5 min tasks) | FAIL | Tasks 4, 5, 6, 8, 9 each represent 30-120 minutes of work |
| Specificity (no vague descriptions) | PASS* | Line-number references are fragile; `canonicalizeContent` helper undocumented |
| Dependencies (all explicit) | PASS* | Missing note that tests depend on exported provider variables |
| TDD structure (test → fail → implement) | FAIL | Tests come after implementation; no red phase defined |
| Complete code (actual snippets) | PASS* | Modifier functions described with bullet lists instead of code |
| Exact paths (absolute) | FAIL | All file paths are relative |

*PASS with minor gaps noted above.

---

## Issues to Fix

1. **Split Tasks 4 and 5** into 6 sub-tasks, one per function/renderer, each 2-5 minutes.
2. **Split Task 6** into one test-writing sub-task per conversion path (Cursor→Windsurf, Windsurf→Cursor, Cursor→Claude, round-trips, edge cases).
3. **Reorder Tasks 4-6** to follow TDD: write one test, implement to pass, commit, repeat.
4. **Replace all relative paths** with absolute paths rooted at `/home/hhewett/.local/src/nesco/`.
5. **Replace line-number references** ("around line 115", "around line 157") with structural landmarks (function names, block descriptions).
6. **Add `canonicalizeContent()` and `findContentFile()` helpers** to the `add.go` task description, including their signatures.
7. **Add a dependency note** in Task 6: verify `provider.Windsurf`, `provider.Cursor`, `provider.ClaudeCode`, and `provider.Codex` are exported variables before writing tests.
8. **Replace bullet-list pseudo-code** in Tasks 7-9 with actual Go function signatures and key implementation lines.
