# Python Testing Patterns

Python-specific testing patterns using pytest and related tools.

---

## Fixtures

### Basics
- Rule: Use `@pytest.fixture` for test setup. Compose fixtures (fixture depends on fixture). Use `yield` for setup/teardown.

### Scopes
- `scope="function"` (default): New for each test.
- `scope="class"`: Shared across class.
- `scope="module"`: Shared across module.
- `scope="session"`: Shared across entire session.

### Parameterized Fixtures
- Rule: `@pytest.fixture(params=["sqlite", "postgres"])` runs every dependent test once per param.

### Autouse Fixtures
- Rule: `@pytest.fixture(autouse=True)` runs before every test without explicit request. Use for state reset, DB transaction rollback.

---

## Mocking

### unittest.mock Essentials
- `Mock()`: Create mock with `.return_value` and `.side_effect`.
- `@patch("myapp.services.module.requests.get")`: Patch at **import location**, not definition.
- `patch.object(Class, "method")`: Patch specific attribute.
- `MagicMock()`: Auto-creates nested attributes and methods.

### pytest-mock (Preferred)
- Rule: Use `mocker` fixture for cleaner syntax. `mocker.patch()`, `mocker.spy()` for real implementation + tracking.

### Patching Rules
- Patch where imported, not where defined.
- Use context manager or decorator style.
- `side_effect=[val1, val2, Exception()]` for successive returns.

---

## Parameterized Tests

```python
@pytest.mark.parametrize("input,expected", [
    pytest.param("valid@email.com", True, id="valid"),
    pytest.param("invalid", False, id="no_at_sign"),
])
def test_validate_email(input, expected):
    assert validate_email(input) == expected
```

- Multiple `@pytest.mark.parametrize` decorators create cartesian product of test cases.

---

## Async Testing

- Rule: Use `pytest-asyncio` plugin. Mark with `@pytest.mark.asyncio`. Async fixtures work with `async def`.

---

## Testing Exceptions

```python
with pytest.raises(ValueError, match=r".*must be positive.*"):
    set_count(-1)
```

- Access exception object via `exc_info.value` for attribute assertions.

---

## Markers

| Marker | Purpose |
|--------|---------|
| `@pytest.mark.slow` | Skip with `-m "not slow"` |
| `@pytest.mark.integration` | Run with `-m integration` |
| `@pytest.mark.skip(reason=...)` | Always skip |
| `@pytest.mark.skipif(condition)` | Conditional skip |
| `@pytest.mark.xfail(reason=...)` | Expected failure |

- Register custom markers in `conftest.py` via `pytest_configure`.

---

## conftest.py Shared Fixtures

- Rule: Place shared fixtures (app, client, db) in `tests/conftest.py`. pytest auto-discovers.

---

## HTTP API Testing

### FastAPI
- Rule: Use `TestClient(app)` for sync tests. Use `httpx.AsyncClient(transport=ASGITransport(app=app))` for async.
- Override dependencies with `app.dependency_overrides[get_db] = override_fn`.

### Flask
- Rule: Use `app.test_client()` within context.

---

## Test Organization

```
tests/
  conftest.py        # Shared fixtures
  unit/              # Fast, isolated
  integration/       # External dependencies
  fixtures/          # Test data files
```

### pytest Configuration (pyproject.toml)
- `testpaths = ["tests"]`, `addopts = "-v --tb=short"`, custom markers.

### Coverage
- `fail_under = 80` in coverage config. Exclude `pragma: no cover`, `if TYPE_CHECKING:`, `raise NotImplementedError`.

---

## Testing Anti-Patterns

### Testing Implementation Details
**Severity**: high

- Rule: Test behavior and outcomes, not internal method calls. Tests coupled to implementation break on every refactor.

### Over-Mocking
**Severity**: high

- Rule: Use fakes (in-memory implementations) for collaborators. Only mock true external boundaries (HTTP APIs, third-party services).

### Flaky Tests
**Severity**: high

- Rule: Mock time instead of `time.sleep()`. Use fixtures for isolation instead of shared state. Use response mocking for network calls.

### Testing Private Methods
**Severity**: medium

- Rule: Test through the public interface. If a private method needs direct testing, extract it into its own class/module.

### No Assertion Context
**Severity**: low

- Rule: Add descriptive messages to assertions with actual values: `assert len(users) == 3, f"Expected 3, got {len(users)}"`.

### Shared Test State
**Severity**: high

- Rule: Each test sets up its own state. Use `autouse` fixtures for common setup/teardown. Use transaction rollback for DB isolation.

### Happy-Path-Only Tests
**Severity**: medium

- Rule: Test edge cases: empty inputs, None, zero, negative, max size, special chars, error conditions. Use `pytest.mark.parametrize`.
