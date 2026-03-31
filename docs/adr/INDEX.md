# ADR Index

Architectural decisions for syllago. Before modifying files in a listed scope, read the full ADR.

| ADR | Title | Status | Enforcement | Scope | Summary |
|-----|-------|--------|-------------|-------|---------|
| [0001](0001-hook-degradation-enforcement.md) | Hook Degradation Enforcement | accepted | strict | `cli/internal/converter/*` | block/warn/exclude strategies must be enforced during conversion, not silently dropped |
