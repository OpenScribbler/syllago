# ADR Index

Architectural decisions for syllago. Before modifying files in a listed scope, read the full ADR.

| ADR | Title | Status | Enforcement | Scope | Summary |
|-----|-------|--------|-------------|-------|---------|
| [0001](0001-hook-degradation-enforcement.md) | Hook Degradation Enforcement | accepted | strict | `cli/internal/converter/*` | block/warn/exclude strategies must be enforced during conversion, not silently dropped |
| [0002](0002-analyzer-hub-and-spoke-architecture.md) | Analyzer Hub-and-Spoke Architecture | accepted | strict | `cli/internal/analyzer/*`, `cli/internal/catalog/scanner.go` | new analyzer package with pluggable detectors; scanner stays as manifest reader |
| [0003](0003-manifest-first-scanner-path.md) | Manifest-First Scanner Path | accepted | strict | `cli/internal/catalog/scanner.go`, `cli/internal/analyzer/manifest.go` | manifest is authoritative; nil vs empty items distinguishes legacy from intentional |
| [0004](0004-executable-content-always-confirms.md) | Executable Content Always Confirms | accepted | strict | `cli/internal/analyzer/analyzer.go` | hooks and MCP always route to Confirm regardless of confidence score |
| [0005](0005-provider-priority-tiebreaking.md) | Provider Priority Tiebreaking | accepted | advisory | `cli/internal/analyzer/dedup.go` | syllago (0) > named providers (1) > top-level (2) when confidence ties |
| [0006](0006-empty-manifest-is-authoritative.md) | Empty Manifest Is Authoritative | accepted | strict | `cli/internal/catalog/scanner.go` | items: [] means zero items, not "scan directories" |
| [0007](0007-moat-g3-slice-1-scope.md) | MOAT G-3 Slice-1 Scope and Trusted Root Strategy | accepted | strict | `cli/internal/moat/*`, `cli/internal/config/config.go`, `cli/cmd/syllago/moat_cmd.go` | primitive + forward-compat schema; bundled trusted root with 90/180/365 staleness; GitHub OIDC numeric-ID binding (OIDs .15/.17) |
