# Python Design Patterns & Architecture

Pythonic design patterns and production code organization.

---

## Pythonic Alternatives to GoF

| GoF Pattern | Pythonic Alternative |
|-------------|---------------------|
| Strategy | First-class functions or callable Protocol |
| Command | Functions + closures |
| Iterator | `__iter__`/`__next__` or generators |
| Template Method | Functions with callbacks or ABCs |
| Singleton | Module-level instance (avoid `__new__` pattern) |
| Proxy | `__getattr__` delegation |
| Adapter | Duck typing or Protocols |

## Factory Pattern

### Simple Factory (Function-Based)
- Rule: Use a dict mapping names to classes. Return instance from a factory function.

```python
def create_serializer(format: str) -> Serializer:
    serializers = {"json": JSONSerializer, "xml": XMLSerializer, "yaml": YAMLSerializer}
    cls = serializers.get(format)
    if cls is None:
        raise ValueError(f"Unknown format: {format!r}. Available: {list(serializers)}")
    return cls()
```

### Registry-Based Factory
- Rule: Use a decorator-based registry with `Registry[T]` generic class for self-registering plugins.
- Pattern: `registry = Registry()` + `@registry.register("name")` decorator + `registry.create("name")`.

### Abstract Factory
- Rule: Use when you need families of related objects. Define abstract factory with ABC, implement per family. For single product types, use simple factory.

## Strategy Pattern

- Rule: In Python, strategy is "pass a function": `PricingStrategy = Callable[[float], float]`.
- For strategies needing state, use Protocol with a single method. Avoid if only 2 options (use if/else).

## Observer Pattern

- Rule: Use lightweight `EventEmitter` class with `defaultdict(list)` for listeners. `on()` registers, `emit()` notifies.
- For async, consider `blinker` library or asyncio events.

## Decorator Pattern (Structural)

- Rule: Python function decorators use `@functools.wraps(func)`. GoF structural decorators wrap an interface implementation with another implementing the same interface for composable behavior.

## Builder Pattern

- Rule: Often unnecessary in Python due to keyword args and dataclasses. Use only when construction needs step-by-step validation. Use `Self` return type (3.11+) for fluent chaining.

## Repository Pattern

- Rule: Define CRUD Protocol in domain package. Implement `InMemoryRepo` for testing, `SQLAlchemyRepo` for production. Business logic depends on the abstract Protocol.

```python
class UserRepository(Protocol):
    def get(self, user_id: str) -> User | None: ...
    def save(self, user: User) -> None: ...
    def delete(self, user_id: str) -> None: ...
```

---

## Dependency Injection

### Constructor Injection (Preferred)
- Rule: Accept Protocol dependencies in constructor, wire in `main()` or composition root.
- Gotcha: Most Python projects never need a DI framework. Explicit wiring suffices.

### Protocols for DI (PEP 544)
- Rule: Use `Protocol` for structural typing. No inheritance required -- just implement the methods.
- Use `@runtime_checkable` sparingly (shallow check, slower than normal `isinstance`).

### FastAPI Depends
- Rule: Use `Annotated[T, Depends(func)]` type aliases for clean signatures. Yield dependencies for resource cleanup (try/finally). Override with `app.dependency_overrides` for testing.

### DI Approach Selection

| Approach | Best For | Complexity |
|----------|----------|------------|
| Constructor injection | Small-medium apps | Low |
| Protocol + functions | Medium apps | Low |
| FastAPI Depends | FastAPI applications | Medium |
| dependency-injector | Large enterprise apps | High |

---

## SOLID Principles

### SRP (Single Responsibility)
- Rule: Each class/module has one reason to change. Split God classes into focused classes.

### OCP (Open/Closed)
- Rule: Use Protocols and lists of handlers. New behavior = new class implementing Protocol, not modifying existing code.

### LSP (Liskov Substitution)
- Rule: Subtypes must be substitutable for base types. Avoid inheritance that changes method semantics (Square/Rectangle problem). Prefer composition + Protocol.

### ISP (Interface Segregation)
- Rule: Use multiple small Protocols. Components declare only what they need. Compose with `Readable & Listable`.

### DIP (Dependency Inversion)
- Rule: Depend on Protocols (abstractions), not concrete classes. High-level modules define the Protocol; low-level modules implement it.

---

## Code Organization

### Project Structure
- Rule: Use `src/` layout with `core/` (domain logic), `api/` (routes), `infrastructure/` (external integrations).
- Layer deps: `api/ -> core/`, `api/ -> infrastructure/`, `infrastructure/ -> core/`. Core has no deps on api/infrastructure.

### __init__.py and __all__
- Rule: Use `__all__` to define public API. Controls `from module import *`, type checker visibility, and doc generators.

### Circular Import Solutions (ranked)
1. Restructure to eliminate cycle (best)
2. Import at function level (acceptable)
3. `TYPE_CHECKING` guard (for type hints only)
4. Lazy imports with `importlib`

### Import Style
- Rule: Absolute imports for application code, relative within reusable libraries.

---

## Configuration Management

### pydantic-settings v2 (Recommended)
- Rule: Use `BaseSettings` with `SettingsConfigDict(env_file=".env", env_prefix="APP_")`. Supports validation, nesting, and typed fields including `SecretStr`.
- Cache with `@lru_cache` on settings factory function.

### Environment-Specific Config
- Rule: Use subclasses of `BaseSettings` per environment. Select via `os.getenv("ENVIRONMENT")` mapping.

---

## Logging

### Standard Library
- Rule: `logging.getLogger(__name__)` per module. Configure once at startup. Use `logger.exception()` for tracebacks.

### structlog (Recommended for Production)
- Rule: JSON output in production, colored console in dev. Use `contextvars` for request-scoped fields. Use `logger.bind()` for structured context.
- Always use `%`-style formatting or structured key-value pairs, never f-strings in log calls.

---

## Error Handling Architecture

### Exception Hierarchies
- Rule: Define `AppError` base with `message` and `code` fields. Subclass for `ValidationError`, `NotFoundError`, `AuthenticationError`, `ExternalServiceError`.

### Result Type Pattern
- Rule: Use `Ok[T] | Err[E]` for operations where failure is expected and not exceptional. Works well with `match/case` (3.10+).

### Error Boundaries
- Rule: Catch and convert exceptions at architectural boundaries (e.g., FastAPI exception handlers mapping domain errors to HTTP status codes).

### Retry Patterns
- Rule: Use `tenacity` library with `@retry(stop=stop_after_attempt(3), wait=wait_exponential(), retry=retry_if_exception_type(...))`.

---

## Resource Management

### Context Managers
- Rule: Use `@contextmanager` for simple cases (yield pattern). Use class-based (`__enter__`/`__exit__`) when you need reusability or complex state.
- Async: `@asynccontextmanager` for async resources.

### Connection Pooling
- Rule: Use `queue.Queue`-based pool with `@contextmanager`. Discard connections on error, return on success.

### Cleanup Patterns
- Rule: Use `ExitStack` to manage multiple context managers dynamically. Use `atexit` and `signal.signal(SIGTERM)` for graceful shutdown.

---

## Plugin/Extension Architectures

### Entry Points (Recommended)
- Rule: Use `pyproject.toml` `[project.entry-points]` + `importlib.metadata.entry_points()` for distributed plugin ecosystems.

### Auto-Registration via `__init_subclass__`
- Rule: Use `__init_subclass__(cls, *, name=None)` to auto-register plugin subclasses in a class-level registry dict.

### Hook-Based System
- Rule: Use `PluginManager` with `defaultdict(list)` of callbacks. `call_hook()` for fan-out, `call_hook_pipeline()` for sequential transformation.

---

## Functional Patterns

### Immutability
- Rule: Use `@dataclass(frozen=True)` or `NamedTuple`. Return new instances with `replace()` instead of mutating.

### Pure Functions
- Rule: Separate pure logic from side effects. Pure functions (same input -> same output, no side effects) are easier to test and cache.

### functools Essentials
- `@lru_cache(maxsize=N)`: Bounded memoization for expensive pure functions (hashable args only).
- `@cache`: Unbounded memoization (3.9+, faster but memory risk).
- `@cached_property`: Computed once per instance (incompatible with `__slots__`).
- `partial()`: Fix function arguments for callbacks.
- `@singledispatch`: Type-based function overloading.

For generator pipelines and itertools patterns, see [performance.md](performance.md).

---

## Pattern Selection Guide

| Pattern | Use When | Avoid When |
|---------|----------|------------|
| Factory | Multiple types share interface | Single implementation |
| Strategy | Swappable algorithms | Only 2 options |
| Observer | Decoupled event handling | Tight ordering required |
| Repository | Abstracting data access | Simple scripts |
| DI (manual) | Testable, flexible code | Trivial scripts |
| Result type | Expected failure paths | Truly exceptional errors |
| Context manager | Resource lifecycle | No cleanup needed |
| Protocols | Structural typing | Concrete types suffice |
| Generators | Large data, memory efficiency | Small collections |
| `lru_cache` | Pure, expensive, repeated calls | Side effects, mutable args |
| Entry points | Distributed plugins | Single-app plugins |
