# Py-Dev Wrapper Reference

Token-efficient wrapper for Python development commands (40-60% reduction).

---

## Prohibition

> **NEVER run raw `pytest`, `mypy`, `ruff`, or `flake8` commands.**
> Always use the `py-dev` wrapper.

## Before Running Any Python Command

1. Is this `pytest`, `mypy`, `ruff check`, `ruff format`, or `flake8`?
2. Do NOT run directly.
3. Use `py-dev <command>` (located at `~/.claude/bin/py-dev` or in PATH).

## Available Commands

```bash
py-dev test .              # Test with pass/fail summary
py-dev test-v .            # Tests with long tracebacks
py-dev test-x .            # Stop on first failure
py-dev test-k "pattern" .  # Match test names
py-dev test-m "not slow" . # Filter by marker
py-dev type src/           # Type check (mypy, errors only)
py-dev lint .              # Lint (ruff, fallback: flake8)
py-dev fmt .               # Format check (dry run)
py-dev fmt-fix .           # Apply formatting
py-dev cover .             # Tests with coverage summary
py-dev check .             # Full: lint + type + test
```

## Environment Variables

- `PY_DEV_VERBOSE=1` - Show more output
- `PY_DEV_RAW=1` - Unfiltered output
- `PY_DEV_MAX_LINES=N` - Override line limit (default: 50)

## Fallback Conditions

Only use raw commands when ALL true:
1. Wrapper not at `~/.claude/bin/py-dev`
2. Not in PATH (`which py-dev` fails)
3. User explicitly approves

Fallback: `python -m pytest -q --tb=short 2>&1 | head -50`

### Tool-Specific Fallbacks
- mypy not installed: `py-dev type` prints warning, returns 0
- ruff not installed: `py-dev lint` falls back to `flake8`
- Neither ruff nor flake8: `py-dev lint` prints warning, returns 0
