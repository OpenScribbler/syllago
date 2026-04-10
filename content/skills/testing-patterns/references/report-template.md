# Test Suite Analysis Report Template

Use this template when presenting test analysis findings.

```markdown
## Test Suite Analysis

**Scope**: [Files/modules analyzed]
**Framework**: [Testing framework in use]
**Coverage**: [If available]

### Quality Assessment

| Dimension | Rating | Notes |
|-----------|--------|-------|
| Behavior focus | Good/Fair/Poor | [Details] |
| Determinism | Good/Fair/Poor | [Details] |
| Clarity | Good/Fair/Poor | [Details] |
| Edge cases | Good/Fair/Poor | [Details] |
| Maintainability | Good/Fair/Poor | [Details] |

### Issues Found

#### HIGH Priority
| File:Line | Issue | Impact | Fix |
|-----------|-------|--------|-----|

#### MEDIUM Priority
[Same format]

### Coverage Gaps
[Missing test scenarios]

### Positive Observations
[What's done well]

### Recommendations
[Prioritized improvements]
```

## Rating Guidelines

| Rating | Criteria |
|--------|----------|
| **Good** | Follows best practices, no significant issues |
| **Fair** | Some issues but generally acceptable |
| **Poor** | Significant problems requiring attention |

## Dimension Definitions

- **Behavior focus**: Tests verify what the system does, not implementation details
- **Determinism**: Tests produce consistent results across runs
- **Clarity**: Test names and structure clearly communicate intent
- **Edge cases**: Boundary conditions and error paths are covered
- **Maintainability**: Tests are easy to understand and modify
