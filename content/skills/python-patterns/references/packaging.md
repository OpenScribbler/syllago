# Python Packaging

Modern Python packaging with pyproject.toml.

---

## Package Structure

```
my-package/
    src/
        my_package/
            __init__.py
            core.py
    tests/
        test_core.py
    pyproject.toml
    README.md
```

## pyproject.toml (PEP 621)

```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "my-package"
version = "0.1.0"
requires-python = ">=3.9"
dependencies = ["requests>=2.28.0", "pydantic>=2.0.0"]

[project.optional-dependencies]
dev = ["pytest>=7.0", "mypy>=1.0", "ruff>=0.1"]

[project.scripts]
my-cli = "my_package.cli:main"
```

## Tool Configuration

```toml
[tool.pytest.ini_options]
testpaths = ["tests"]
addopts = "-v --cov=my_package"

[tool.mypy]
python_version = "3.11"
strict = true

[tool.ruff]
target-version = "py311"
line-length = 88
select = ["E", "F", "I", "N", "W", "UP"]

[tool.coverage.run]
source = ["src/my_package"]
branch = true
```

## Build and Publish

```bash
pip install build twine
python -m build
twine check dist/*
twine upload dist/*
```

## Version Management

- **Git tags**: Use `hatch-vcs` with `[tool.hatch.version] source = "vcs"`.
- **Single source**: `__version__` in `__init__.py` with `[tool.hatch.version] path = "src/my_package/__init__.py"`.

## Development

```bash
pip install -e ".[dev]"      # Editable install with dev deps
pip install -e ".[dev,docs]" # All optional deps
```

## Namespace Packages
- Rule: No `__init__.py` at namespace level. Multiple packages share the same namespace.

## Anti-Patterns

| Anti-Pattern | Fix |
|--------------|-----|
| setup.py only | Migrate to pyproject.toml |
| requirements.txt for deps | Use pyproject.toml dependencies |
| No src/ layout | Use src/ layout to avoid import confusion |
| Hardcoded version everywhere | Single source of truth |
