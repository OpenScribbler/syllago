---
name: code-review-standards
description: Code review standards and checklists. Use when reviewing code for correctness, security, performance, and maintainability. Activate for "review code", "security audit", "code quality check", or pre-production reviews.
---

# Code Review Standards

Structured checklists and patterns for comprehensive code review.

## Review Priority Order

1. **Correctness** - Bugs, logic errors, edge cases
2. **Security** - Vulnerability check (basic for general review, deep for security review)
3. **Performance** - Bottlenecks and inefficiencies
4. **Maintainability** - Code quality and readability
5. **Architecture** - Design patterns and structure

## Quick Checklists

| Security | Check For |
|----------|-----------|
| Injection | SQL, command, LDAP, XSS in user inputs |
| Auth | Token handling, session management, password storage |
| Data | Secrets in code, PII in logs, insecure storage |
| Crypto | Weak algorithms, hardcoded keys, improper randomness |
| Access | Missing authorization checks, privilege escalation |

| Performance | Signs |
|-------------|-------|
| N+1 queries | Loops with DB calls, missing eager loading |
| Memory leaks | Unclosed resources, growing collections |
| Blocking I/O | Sync calls in async context, missing timeouts |
| Inefficient algos | Nested loops on large data, string concat in loops |

## Output Format

```markdown
### Executive Summary
2-3 sentence health assessment.

### Critical Issues (must fix)
| File:Line | Issue | Impact | Fix |

### High Priority (should fix)
[Same format]

### Positive Observations
What's done well - maintain these patterns.
```

## Security Scanning

Use `sec-scan` wrapper (Mac/Linux) or `sec-scan.ps1` (Windows). Start with `sec-scan summary ./...` for quick counts, then run targeted scans based on results.

## References

Load on-demand based on code being reviewed:

| When to Use | Reference |
|-------------|-----------|
| Go review: false positives, verification steps, non-issues vs real issues, decision trees | [go-review.md](references/go-review.md) |
| Security review: injection, auth, secrets, crypto, TOCTOU, randomness | [security.md](references/security.md) |
| K8s/Helm review: RBAC, pod security, resources, probes, network policies, images | Load [kubernetes-patterns](../kubernetes-patterns/SKILL.md), then `references/anti-patterns.md` |

## Language-Specific Patterns

For detailed language patterns, load the respective skill:

| Language | Skill |
|----------|-------|
| Go | [go-patterns](../go-patterns/SKILL.md) |
| Python | [python-patterns](../python-patterns/SKILL.md) |
| JavaScript/TypeScript | [javascript-patterns](../javascript-patterns/SKILL.md) |
| Rust | [rust-patterns](../rust-patterns/SKILL.md) |
