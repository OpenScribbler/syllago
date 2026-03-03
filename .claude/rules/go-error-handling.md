---
paths:
  - "cli/**/*.go"
---

# Go Conventions (CLI)

## Error Handling

- Return errors up the call stack — never panic for recoverable errors
- Wrap with context: `fmt.Errorf("doing X: %w", err)`
- TUI components store errors as `message string` + `messageIsErr bool` for user display
- Non-TUI code (commands, installer, catalog) returns `error` to the caller

## Naming

- Follow standard Go conventions: MixedCaps for exported, mixedCaps for unexported
- Enum types use typed constants with iota
- Message types end in `Msg` suffix (e.g., `appInstallDoneMsg`, `openModalMsg`)
- Model types end in `Model` suffix (e.g., `detailModel`, `sidebarModel`)

## Testing

- Table-driven tests with `t.Run()` subtests
- Test file names match source: `foo.go` → `foo_test.go`
- Use `t.Helper()` in test utility functions
- No test dependencies on network or filesystem state (use temp dirs)
