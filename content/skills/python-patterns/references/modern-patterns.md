# Modern Python Patterns (3.10+)

Production-ready patterns for modern Python features.

---

## Feature Version Reference

| Feature | Version | Use Case |
|---------|---------|----------|
| Pattern matching (`match/case`) | 3.10 | Complex branching on data structure |
| `ParamSpec` | 3.10 | Type-safe decorator wrappers |
| `dataclass(slots=True, kw_only=True)` | 3.10 | Memory-efficient dataclasses |
| Union syntax `X | Y` | 3.10 | Cleaner type hints |
| `TypeGuard` | 3.10 | Custom type narrowing |
| `Self` type | 3.11 | Fluent/builder return types |
| `StrEnum` | 3.11 | String-valued enumerations |
| `asyncio.TaskGroup` | 3.11 | Structured concurrency |
| `except*` | 3.11 | ExceptionGroup handling |
| `type` statement | 3.12 | Type alias declarations |
| Type param syntax `class Foo[T]:` | 3.12 | Generic class syntax |
| `itertools.batched` | 3.12 | Chunk processing |
| `TypeIs` | 3.13 | Improved type narrowing (both branches) |

---

## Structural Pattern Matching

### When to Use
- Parsing structured data (JSON, ASTs, command tuples), handling multiple event types, replacing isinstance chains.
- NOT for: simple value equality (use if/elif), single conditions, hot loops (overhead).

### Pattern Types
- **Literal**: `case 200:` - match exact values
- **Capture**: `case (x, y):` - bind matched values
- **Sequence**: `case [first, *rest]:` - match lists/tuples with structure
- **Mapping**: `case {"type": "click", "x": x}:` - match dicts by key
- **Class**: `case Circle(radius=r) if r > 0:` - match class instances with guards
- **OR**: `case "quit" | "exit" | "q":` - match alternatives

### Pitfalls
- Bare names are **capture patterns**, not constants. Use dotted names (`Status.OK`) or literals for constants.
- Names bound in patterns leak into enclosing scope.
- All alternatives in `|` must bind the same set of names.

---

## Dataclasses

### Key Parameters
- `frozen=True`: Immutable, hashable instances.
- `slots=True` (3.10+): Memory-efficient (`__slots__` instead of `__dict__`).
- `kw_only=True` (3.10+): All fields keyword-only.
- Use `field(default_factory=list)` for mutable defaults (never `tags: list = []`).

### Helpers
- `replace(obj, field=new_value)`: New instance with changes.
- `asdict()`: Recursive dict conversion (expensive for large nested structures).
- `fields()`: Tuple of Field objects.
- `KW_ONLY` sentinel: Mark boundary between positional and keyword-only fields.

### attrs (Third-Party)
- Rule: Use attrs when you need validators, converters, or advanced features beyond stdlib dataclasses. Use `@define(frozen=True)` with `field(validator=...)`.

### Pitfalls
- Mutable defaults: `x: list = []` raises `ValueError` in 3.11+. Always use `field(default_factory=...)`.
- Inheritance ordering: Base field with default + subclass field without default = `TypeError`.
- `asdict()` does deep copy via recursion -- expensive for large structures.

---

## Protocols and Structural Subtyping (PEP 544)

- Rule: Define interfaces without requiring inheritance. Any class with matching methods satisfies the protocol implicitly.
- Use `@runtime_checkable` to enable `isinstance()` checks (shallow, only checks attribute existence, not signatures).
- Generic protocols: `class Repository(Protocol[T]):` for typed abstractions.

### Protocol vs ABC

| Feature | Protocol | ABC |
|---------|----------|-----|
| Inheritance required | No | Yes |
| Retroactive conformance | Yes | No |
| Method bodies | `...` only | Can provide defaults |

---

## Advanced Type Hints

### TypeVar and Generics
- Old: `T = TypeVar("T")` + `def first(items: list[T]) -> T:`
- 3.12+: `def first[T](items: list[T]) -> T:` (preferred)

### ParamSpec (3.10+)
- Rule: Use `P = ParamSpec("P")` for type-safe decorator wrappers that preserve wrapped function signatures.

### TypeGuard vs TypeIs
- `TypeGuard` (3.10+): Narrows type on True branch only.
- `TypeIs` (3.13+): Narrows on both True and False branches. Must be subtype of input.

### Self Type (3.11+)
- Rule: Use `-> Self` for fluent/builder methods and `@classmethod` constructors.

### Union Syntax (3.10+)
- Rule: Use `int | str` and `str | None` instead of `Union[int, str]` and `Optional[str]`.

---

## Walrus Operator (:=)

- Rule: Assignment expression for avoiding duplicate computation. Good for `while (line := f.readline()):`, `if (n := len(x)) > 10:`, regex matching, comprehension filters.
- Gotcha: Low precedence -- use parentheses. Name leaks into enclosing scope in comprehensions.
- Avoid: Complex chained walrus expressions that reduce readability.

---

## Context Managers

### @contextmanager / @asynccontextmanager
- Rule: Use for simple resource management. Must yield exactly once.

### ExitStack / AsyncExitStack
- Rule: Manage dynamic number of context managers. Register cleanup callbacks. LIFO execution order.
- `stack.pop_all()`: Prevent cleanup on success path.

### Utility Context Managers
- `suppress(ErrorType)`: Ignore specific exceptions.
- `redirect_stdout(buffer)`: Capture output.
- `nullcontext(value)`: Conditional context manager.
- `chdir(path)` (3.11+): Temporary directory change.

### ContextDecorator
- Rule: Inherit from `ContextDecorator` to make a context manager usable as both `with` statement and `@decorator`.

### Pitfalls
- Returning `True` from `__exit__` silences exceptions -- only when intentional.
- ExitStack callbacks run LIFO.

---

## Decorators

### Essentials
- Always use `@functools.wraps(func)` to preserve `__name__`, `__doc__`, `__module__`.
- Decorator factory: outer function returns decorator function. `@retry(max_attempts=3)`.
- Stacking order: bottom-up. `@a @b def f` means `a(b(f))`.

### Key functools Decorators

| Decorator | Purpose |
|-----------|---------|
| `@lru_cache(maxsize=N)` | Bounded memoization with LRU eviction |
| `@cache` (3.9+) | Unbounded memoization (faster, memory risk) |
| `@cached_property` (3.8+) | Computed once per instance |
| `@singledispatch` | Type-based function overloading |
| `@singledispatchmethod` | Type-based method overloading |
| `@total_ordering` | Auto-generate comparison ops from `__eq__` + `__lt__` |

### Pitfalls
- Missing `@wraps`: breaks introspection tools.
- `@retry` vs `@retry()`: forgetting `()` passes decorated function as first arg.
- `@cached_property` requires `__dict__`, incompatible with `__slots__`.
- `@lru_cache` on methods: caches `self` reference, prevents GC. Use `@staticmethod` + cache or external cache.

---

## Descriptors

### Protocol
- `__get__`, `__set__`, `__delete__`, `__set_name__`
- Data descriptor (`__get__` + `__set__`): overrides instance `__dict__`.
- Non-data descriptor (`__get__` only): overridden by instance `__dict__`.

### Validation Descriptor
- Rule: Use `__set_name__` + private attribute storage for reusable field validation (e.g., `PositiveNumber`, `NonEmptyString`).

### Attribute Lookup Order
1. Data descriptors from class MRO
2. Instance `__dict__`
3. Non-data descriptors / class variables from MRO
4. `__getattr__()` if defined

---

## Metaclasses and __init_subclass__

### __init_subclass__ (Preferred)
- Rule: Use for subclass registration, validation, and field enforcement. Always call `super().__init_subclass__(**kwargs)`.

### When to Use Metaclasses
- Only when you need: custom class namespace, control `__new__`, modify MRO, or framework-level class construction.
- 95% of metaclass use cases are better served by `__init_subclass__`, class decorators, or descriptors.

---

## Enum Patterns

### Types

| Type | Use Case | Members Behave As |
|------|----------|-------------------|
| `Enum` | General symbolic constants | Opaque identifiers |
| `StrEnum` (3.11+) | String constants for APIs | `str` (no `.value` needed) |
| `IntEnum` | Integer constants needing arithmetic | `int` (direct comparison) |
| `Flag` | Combinable bit flags | Bitwise-composable |

### Pitfalls
- `StrEnum auto()` generates **lowercase** member name: `RED = auto()` -> `"red"`.
- `IntEnum` equality: `Priority.HIGH == 3` is `True` -- can mask bugs.
- Pattern matching: `case CIRCLE:` captures any value. Must use `case Shape.CIRCLE:` for enum matching.
- `Flag(0)` is **falsy**.

---

## Pitfalls

### Type Hints That Lie
**Severity**: medium

- Rule: Match type hints to actual behavior. Return `Optional[T]` if you can return `None`. Run `mypy` to catch mismatches.

### F-String in Logging
**Severity**: medium

- Rule: Use `%`-style formatting for logging: `logger.info("user %s", user_id)`. F-strings eagerly evaluate even when log level is disabled.
- Gotcha: Never use f-strings for SQL queries, shell commands, or HTML.

### Mutable Dataclass Pitfalls
**Severity**: medium

- Rule: Use `frozen=True` for immutable hashable dataclasses. Use `field(default_factory=list)` for mutable defaults (never `tags: list = []`).
