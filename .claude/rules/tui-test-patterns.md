---
paths:
  - "cli/internal/tui_v1/**"
---

# TUI Test Conventions

## Golden File Tests

Visual output tests use golden files in `testdata/`:
- Component tests: `component-*.golden`
- Full app tests: `fullapp-*.golden`
- Size variants: `-60x20`, `-120x40`, `-160x50` (responsive testing)
- Overflow tests: `fullapp-*-overflow*.golden` (large dataset)
- Empty tests: `fullapp-*-empty*.golden` (empty catalog)

**After any visual change, regenerate goldens:**
```bash
go test ./cli/internal/tui_v1/ -update-golden
# Then: git diff cli/internal/tui_v1/testdata/ to verify changes are intentional
```

**Path normalization:** Tests replace temp directory paths with `<TESTDIR>` and strip trailing whitespace for deterministic output.

## Test Helpers (testhelpers_test.go)

- `testApp(t)` / `testAppSize(t, w, h)` — standard catalog (8 items)
- `testAppLarge(t)` / `testAppLargeSize(t, w, h)` — large catalog (85+ items) for overflow testing
- `testAppEmpty(t)` / `testAppEmptySize(t, w, h)` — empty catalog for empty-state testing
- `requireGolden(t, name, got)` — compare output to golden file
- `pressN(app, key, n)` — send a key n times
- `navigateToDetail(t, ct)` — navigate to first item detail for content type

## Boundary-Condition Testing

When adding or modifying visual components, consider:
- **Overflow:** What happens with 50+ items? Use `testAppLarge(t)`.
- **Empty state:** What happens with 0 items? Use `testAppEmpty(t)`.
- **Small terminal:** What happens at 60x20? Use `testAppLargeSize(t, 60, 20)`.
- **Long text:** What happens with 200+ character descriptions or names?
- **Scroll bounds:** Does cursor clamp correctly at start and end?

## Test Structure

- Table-driven tests with `t.Run()` subtests
- Test both keyboard and mouse interactions for interactive components
- Modal tests: open, navigate, confirm/cancel, state after close
- Always test Esc dismissal and click-away behavior for modals
- Toast assertions use `app.toast.text` (not component `message` fields, which are cleared by promotion)
