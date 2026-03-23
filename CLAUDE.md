# Syllago

A package manager for AI coding tool content. Import, export, and convert content (rules, skills, agents, prompts, hooks, commands, MCP configs) between providers. Hub-and-spoke conversion through syllago's own canonical format.

Content registries are community-driven — syllago provides the tooling but does not own or curate registry content.

## Build and Test

```bash
make setup          # Configure git hooks (run once after clone)
make build          # Build dev binary (cli/syllago → ~/.local/bin/syllago)
make test           # Run all tests
```

IMPORTANT: Always run `make build` after code changes before testing. The `syllago` command runs from the compiled binary, not source.

IMPORTANT: Always run `cd cli && make fmt` before committing Go changes. CI enforces gofmt and will fail on unformatted code. A pre-commit hook also blocks unformatted commits locally.

For TUI visual changes, regenerate golden baselines after tests pass:
```bash
cd cli && go test ./internal/tui/ -update-golden
```

## Project Status

Syllago is **pre-release and unpublished**. No users depend on current APIs, CLI commands, or file formats. This means:
- No backwards compatibility, migration paths, or deprecation periods needed
- Commands, terminology, and file layouts can change freely
- Optimize for getting it right, not for preserving what exists

## Key Conventions

- **Hooks and MCP configs** merge into provider settings files (JSON merge). All other content types use filesystem (files, dirs, symlinks).
- **Hook canonical format** is defined in `docs/spec/hooks-v1.md`. Canonical event names are provider-neutral snake_case (`before_tool_execute`, not `PreToolUse`). Canonical tool names are descriptive lowercase (`shell`, `file_read`, `file_edit`). Provider-native names (CC's `PreToolUse`, Gemini's `BeforeTool`, etc.) live in the `HookEvents`/`ToolNames` maps in `toolmap.go` — Claude Code is a regular provider entry, not the implicit key.
- Go conventions: see `cli/CLAUDE.md`
- TUI component patterns: see `cli/internal/tui/CLAUDE.md`

## Go Edit Patterns

When adding new functionality that requires both a new import AND new code using that import:

1. **Prefer a new file** over modifying an existing file when adding a cohesive set of functions (e.g., `privacy.go` for privacy gate logic). This avoids any risk of import/usage sequencing issues.
2. **If modifying an existing file**, add the import and the code that uses it in a **single Edit call** — never add an import in one edit and the function using it in a separate edit. Between edits, `goimports` or `gopls` may strip the "unused" import.
3. **After editing Go files**, verify with `grep` that your changes survived before proceeding. System reminders about files being "modified by a linter" mean the edit may not have applied as expected.
