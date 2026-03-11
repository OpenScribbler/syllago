---
paths:
  - "cli/internal/tui/*_test.go"
---

# TUI Boundary-Condition Testing

When adding or modifying a TUI component, include tests for boundary conditions. These catch the most common visual bugs (scroll panics, truncation issues, empty-state rendering).

## Checklist

For any new page model or significant visual change, test:

1. **Overflow (50+ items):** Use `testAppLarge(t)` — verifies scroll indicators appear, cursor clamps correctly, no panics
2. **Empty state (0 items):** Use `testAppEmpty(t)` — verifies graceful empty message, no index-out-of-range
3. **Long text:** Items with 200+ char names/descriptions — verifies truncation works
4. **Minimum terminal:** Use `testAppLargeSize(t, 60, 20)` — verifies nothing breaks at tiny dimensions with lots of data

## Test Helpers

```go
testAppLarge(t)                    // 85+ items across Skills/Agents/MCP at 80x30
testAppLargeSize(t, 60, 20)       // same catalog at minimum terminal size
testAppEmpty(t)                    // empty catalog at 80x30
testAppEmptySize(t, 60, 20)       // empty catalog at minimum size
```

## Scroll Behavior Tests

For any component with `scrollOffset` or `cursor`:
- Pressing Down past the last item should clamp (not panic)
- Pressing Up at cursor=0 should stay at 0
- Switching tabs or navigating away should reset scroll to 0
- Home/End should jump to first/last item
