# Syllago

A package manager for AI coding tool content. Import, export, and convert content (rules, skills, agents, prompts, hooks, commands, MCP configs) between providers. Hub-and-spoke conversion through syllago's own canonical format.

Content registries are community-driven — syllago provides the tooling but does not own or curate registry content.

## Build and Test

```bash
make setup          # Configure git hooks (run once after clone)
make build          # Compile binary to cli/syllago
cp cli/syllago ~/.local/bin/syllago   # Install to PATH (REQUIRED — see below)
make test           # Run all tests
```

IMPORTANT: After any Go code change, you MUST do BOTH steps before declaring work complete or asking the user to test:

1. **Build** — `make build` writes the compiled binary to `cli/syllago` only. The Makefile has no install target.
2. **Install to PATH** — `cp cli/syllago ~/.local/bin/syllago` (or whichever path `which syllago` resolves to). The user's `syllago` command runs the binary on PATH, NOT `cli/syllago`. Skipping this step means the user tests against a stale binary and your fix appears not to work — wasting a full debug cycle.

If you only ran `make build`, the work is NOT done. Both commands are mandatory; treat them as one indivisible step.

IMPORTANT: Always run `cd cli && make fmt` before committing Go changes. CI enforces gofmt and will fail on unformatted code. A pre-commit hook also blocks unformatted commits locally.

For TUI visual changes, regenerate golden baselines after tests pass:
```bash
cd cli && go test ./internal/tui/ -update-golden
```

## Key Conventions

- **Hooks and MCP configs** merge into provider settings files (JSON merge). All other content types use filesystem (files, dirs, symlinks).
- **Hook canonical format** is defined in `docs/spec/hooks.md`. Canonical event names are provider-neutral snake_case (`before_tool_execute`, not `PreToolUse`). Canonical tool names are descriptive lowercase (`shell`, `file_read`, `file_edit`). Provider-native names (CC's `PreToolUse`, Gemini's `BeforeTool`, etc.) live in the `HookEvents`/`ToolNames` maps in `toolmap.go` — Claude Code is a regular provider entry, not the implicit key.
- Go conventions: see `cli/CLAUDE.md`
- TUI component patterns: see `cli/internal/tui/CLAUDE.md`

## Architectural Decisions

Active ADRs are indexed in `docs/adr/INDEX.md`. Before modifying files listed in an ADR's scope, read the full ADR to understand the rationale. Hooks will remind you when you touch scoped files — `strict` ADRs block commits, `advisory` ADRs warn.

## Testing Requirements

Every code change must include tests. Coverage target is **80% minimum per package, 95%+ aspirational**.

- **New functions** must have corresponding test cases covering the happy path and at least one error path.
- **Bug fixes** must include a regression test that would have caught the bug.
- **New files** must have a corresponding `_test.go` file unless they contain only types/constants.
- **CLI commands** must have integration tests using cobra's `RunE` pattern (see `cli/CLAUDE.md` and `.claude/rules/cli-test-patterns.md`).
- **TUI components** must have golden file tests for visual output and unit tests for logic (see `.claude/rules/tui-test-patterns.md`).

Test patterns already established:
- `httptest.NewServer` for HTTP endpoints (see `updater/updater_test.go`)
- `t.TempDir()` + `git init` for git operations (see `promote/promote_test.go`)
- Table-driven tests with `t.Run()` subtests
- No mocking libraries — hand-craft stubs using interfaces and function overrides

Check coverage after changes: `cd cli && go test ./your/package/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total`

## Go Edit Patterns

When adding new functionality that requires both a new import AND new code using that import:

1. **Prefer a new file** over modifying an existing file when adding a cohesive set of functions (e.g., `privacy.go` for privacy gate logic). This avoids any risk of import/usage sequencing issues.
2. **If modifying an existing file**, add the import and the code that uses it in a **single Edit call** — never add an import in one edit and the function using it in a separate edit. Between edits, `goimports` or `gopls` may strip the "unused" import.
3. **After editing Go files**, verify with `grep` that your changes survived before proceeding. System reminders about files being "modified by a linter" mean the edit may not have applied as expected.
