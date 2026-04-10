---
name: python-patterns
description: Python development patterns and best practices. Use when building or fixing Python services, scripts, or applications.
---

# Python Development Patterns

This skill provides patterns for writing production-quality Python code.

## Quick Reference

| Category | Best Practice |
|----------|---------------|
| Types | Use type hints for function signatures |
| Paths | Use `pathlib.Path` over `os.path` |
| Errors | Use specific exception types |
| Async | Prefer `async/await` over threads for I/O |
| Config | Use environment variables or config files |
| Testing | Use pytest with fixtures |

## Commands

> **NEVER run raw `pytest`, `mypy`, `ruff`, or `flake8` commands.**
> Always use the `py-dev` wrapper for token-efficient output.

```bash
# Testing
~/.claude/bin/py-dev test .              # Run all tests
~/.claude/bin/py-dev test-v .            # Tests with long tracebacks
~/.claude/bin/py-dev test-x .            # Stop on first failure
~/.claude/bin/py-dev test-k "pattern" .  # Match test names
~/.claude/bin/py-dev test-m "not slow" . # Filter by marker

# Quality checks
~/.claude/bin/py-dev lint .              # Lint (ruff or flake8)
~/.claude/bin/py-dev type src/           # Type check (mypy)
~/.claude/bin/py-dev fmt .               # Format check (dry run)
~/.claude/bin/py-dev fmt-fix .           # Apply formatting

# Combined
~/.claude/bin/py-dev check .             # Lint + type + test
~/.claude/bin/py-dev cover .             # Tests with coverage
```

```bash
# Package management (not wrapped - run directly)
pip install -r requirements.txt     # Install deps
pip freeze > requirements.txt       # Freeze deps
python -m pip install -e .          # Install in dev mode
```

## Security Checklist

- [ ] Never log secrets or tokens
- [ ] Use parameterized queries for SQL
- [ ] Validate all user input
- [ ] Use `secrets` module for random tokens
- [ ] Set appropriate file permissions
- [ ] Use HTTPS for external APIs

## References

Load on-demand based on task:

| When to Use | Reference |
|-------------|-----------|
| Full py-dev wrapper commands, fallback, env vars | [py-dev-wrapper.md](references/py-dev-wrapper.md) |
| Code smells: mutable defaults, bare except, God class, boolean params, N+1 | [anti-patterns.md](references/anti-patterns.md) |
| Security: eval/exec, pickle, SQL injection, command injection, secrets | [security.md](references/security.md) |
| Writing or debugging tests, fixtures, mocking, parametrize, test smells | [testing.md](references/testing.md) |
| Structuring packages, Factory, Strategy, DI, SOLID, logging, errors | [design-patterns.md](references/design-patterns.md) |
| Using 3.10+ features: match/case, dataclasses, Protocols, type hints | [modern-patterns.md](references/modern-patterns.md) |
| Profiling, data structures, caching, threading, memory, NumPy/Pandas | [performance.md](references/performance.md) |
| Building FastAPI, Pydantic v2, SQLAlchemy 2.0, auth, middleware | [web-api-patterns.md](references/web-api-patterns.md) |
| Using asyncio, gather, TaskGroup, semaphores, producer-consumer | [async-patterns.md](references/async-patterns.md) |
| Packaging with pyproject.toml, uv, build, publish | [packaging.md](references/packaging.md) |
