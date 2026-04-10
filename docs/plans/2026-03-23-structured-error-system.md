# Structured Error System — Implementation Plan

**Date:** 2026-03-23
**Status:** Ready for implementation
**Feature:** structured-errors

## Context

Syllago has 46 error codes defined in `cli/internal/output/errors.go` across 14 categories, but only 6 of ~196 error call sites actually use structured errors. The rest use plain `fmt.Errorf`. Error documentation exists on the docs site (syllago-docs repo) but is maintained separately, creating drift between code and docs.

This plan builds a Rust-inspired error system where:

- Error docs live in THIS repo (source of truth), embedded into the binary via Go's `//go:embed`
- `syllago explain CODE` prints error details offline from the compiled binary
- Every user-facing error gets a code, message, suggestion, and docs link
- A test enforces 1:1 parity between error code constants and doc files
- The docs site will consume these files as a content collection (separate PR to syllago-docs)

## Key Design Decisions

1. **Pattern B as default.** Return `StructuredError` directly from commands — `printExecuteError` in `main.go:203` already handles it via `errors.As`. No need for `PrintStructuredError + SilentError` boilerplate at each call site. Pattern A (print at source + SilentError) only for cases needing extra pre-error output.

2. **All errors get codes.** Including `%w` wraps — the wrapped error goes in the `Details` field via `NewStructuredErrorDetail`. ~196 call sites total across ~20 files.

3. **Internal packages keep Go errors.** Only `promote/privacy.go` (2 privacy gate errors) returns `StructuredError` directly because those messages are user-facing and terminal. Other promote/loadout/add errors stay as `fmt.Errorf` and get converted to structured errors at the command layer boundary.

4. **Docs format matches existing docs site.** Full prose with 4 sections: What This Means / Common Causes / How to Fix / Example Output.

5. **Docs embedded via `//go:embed`.** Located at `cli/internal/errordocs/docs/*.md`, compiled into the binary at build time.

## Conversion Patterns

```go
// Terminal error (no wrapped cause):
return output.NewStructuredError(output.ErrProviderNotFound,
    "unknown provider: "+slug,
    "Available: "+strings.Join(slugs, ", "))

// Wrapped error (has underlying cause):
return output.NewStructuredErrorDetail(output.ErrConfigNotFound,
    "loading config",
    "Check .syllago/config.json exists and is valid JSON",
    err.Error())
```

## Error Code Taxonomy (46 codes, 14 categories)

Defined in `cli/internal/output/errors.go`:

| Category | Codes | Description |
|----------|-------|-------------|
| CATALOG | 001-002 | Repo/library discovery and scanning |
| REGISTRY | 001-008 | Remote content registry operations |
| PROVIDER | 001-002 | AI coding tool provider detection/lookup |
| INSTALL | 001-005 | Installing/uninstalling content to providers |
| CONVERT | 001-003 | Content format conversion between providers |
| IMPORT | 001-002 | Importing content from external sources |
| EXPORT | 001-002 | Exporting content to provider directories |
| CONFIG | 001-004 | Configuration loading, saving, validation |
| LOADOUT | 001-005 | Loadout creation, application, removal |
| PRIVACY | 001-003 | Registry privacy gates blocking content flow |
| PROMOTE | 001-003 | Sharing/publishing content via git operations |
| ITEM | 001-003 | Content item lookup and resolution |
| INPUT | 001-004 | CLI flag and argument validation |
| INIT | 001 | Project initialization |
| SYSTEM | 001-002 | Environment and filesystem issues |

## Files Summary

| Action | Count | Path Pattern |
|--------|-------|-------------|
| Create | 1 | `cli/internal/errordocs/errordocs.go` |
| Create | 1 | `cli/internal/errordocs/errordocs_test.go` |
| Create | 46 | `cli/internal/errordocs/docs/*.md` |
| Create | 1 | `cli/cmd/syllago/explain_cmd.go` |
| Modify | 2 | `cli/internal/output/errors.go`, `output.go` |
| Modify | 1 | `cli/internal/output/errors_test.go` |
| Modify | ~20 | `cli/cmd/syllago/*.go` (wiring) |
| Modify | 1 | `cli/internal/promote/privacy.go` |

## Verification Criteria

- `make test` passes (all packages)
- `make build` succeeds
- Parity test confirms 46 codes ↔ 46 doc files
- `syllago explain CATALOG_001` prints full documentation
- `syllago explain --list` shows all 46 codes
- Every user-facing error shows `[CODE]` prefix and explain hint
- No plain `fmt.Errorf` remains in command-layer files for user-facing errors
