---
name: go-patterns
description: Go development patterns and best practices. Use when building or fixing Go services, implementing resilience patterns, or writing production-ready Go code. For Kubernetes deployment patterns, see kubernetes-patterns.
---

# Go Development Patterns

This skill provides patterns for writing production-ready Go code.

## Quick Reference

| Category | Best Practice |
|----------|---------------|
| Errors | Always handle errors explicitly |
| Context | Pass context as first parameter |
| Concurrency | Use channels and sync primitives correctly |
| Testing | Table-driven tests with clear names |
| Logging | Structured logging with levels |
| Config | Environment variables or config files |

## Go Commands (Token-Optimized)

> **PROHIBITION: NEVER run raw `go test`, `go build`, or `go vet` commands.**
> Always use the `go-dev` (Mac/Linux) or `go-dev.ps1` (Windows) wrapper (80-85% token reduction).

```bash
go-dev check ./...   # Build + vet + test (quick verification)
go-dev test ./...    # Tests with filtered output
go-dev build ./...   # Build with errors only
go-dev race ./...    # Race detection - issues only
go-dev bench ./...   # Run benchmarks (results only)
go-dev escape ./...  # Escape analysis (heap escapes only)
```

Wrapper location:
- Mac/Linux: `~/.claude/bin/go-dev` or in PATH
- Windows: `~/.claude/bin/go-dev.ps1` or in PATH

On Windows, use `go-dev.ps1` instead of `go-dev` in all commands. See [go-dev-wrapper.md](references/go-dev-wrapper.md) for full command reference, chaining examples, and fallback conditions.

## File Reading Strategy

1. **Find first**: `Grep output_mode: "files_with_matches"`
2. **Read targeted**: Use `limit: 100` for files >200 lines
3. **Trace paths**: Follow specific function calls, don't read entire files

## References

Load on-demand based on task:

| When to Use | Reference |
|-------------|-----------|
| Full go-dev wrapper commands, chaining, fallback | [go-dev-wrapper.md](references/go-dev-wrapper.md) |
| Language footguns: nil, slices, maps, defer, mutex | [gotchas.md](references/gotchas.md) |
| Code smells: error handling, goroutine leaks, interfaces | [anti-patterns.md](references/anti-patterns.md) |
| Writing or debugging tests, mocking, benchmarks | [testing.md](references/testing.md) |
| Adding retries, circuit breakers, timeouts, health checks | [resilience.md](references/resilience.md) |
| Using Go 1.21+ features (slog, iterators, cmp, maps) | [modern-go.md](references/modern-go.md) |
| Structuring packages, interfaces, options, DI, middleware | [design-patterns.md](references/design-patterns.md) |
| Optimizing allocations, profiling, concurrency tuning | [performance.md](references/performance.md) |

## Related Skills

- **Kubernetes deployment**: Load [kubernetes-patterns](../kubernetes-patterns/SKILL.md) for K8s manifests and deployment
- **Terraform/IaC**: Load [terraform-patterns](../terraform-patterns/SKILL.md) for infrastructure code
- **Code review**: Load [code-review-standards](../code-review-standards/SKILL.md) when reviewing Go code
- **Testing patterns**: Load [testing-patterns](../testing-patterns/SKILL.md) for universal test design principles
