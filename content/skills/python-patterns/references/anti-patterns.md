# Python Anti-Patterns

Design-level code smells and common mistakes for **reviewing** Python code. For language-level features and modern syntax, see [modern-patterns.md](modern-patterns.md).

---

## Common Anti-Patterns

### Mutable Default Arguments
**Severity**: high

- Rule: Default mutable arguments (`[]`, `{}`, `set()`) are shared across all calls. Use `None` + conditional initialization.
- Gotcha: Applies to all mutable defaults including custom objects and dataclass fields (use `field(default_factory=...)`).

### Bare Except Clauses
**Severity**: high

- Rule: Never use bare `except:` or silently swallow `except Exception: pass`. Catch specific exceptions and handle them (log, re-raise, or return a default).
- Gotcha: Bare `except:` catches `SystemExit`, `KeyboardInterrupt`, and `GeneratorExit`.

### Using `type()` Instead of `isinstance()`
**Severity**: medium

- Rule: Use `isinstance(value, int)` not `type(value) == int`. The latter ignores inheritance (`bool` is `int` subclass).
- Fix: `isinstance()` also supports tuples: `isinstance(value, (int, float))`.

### Wildcard Imports
**Severity**: medium

- Rule: Never use `from module import *`. It pollutes namespace, breaks IDE tooling, and silently shadows builtins. Import specific names.

### Not Using Context Managers
**Severity**: high

- Rule: Always use `with` for resources (files, connections, locks, sockets). Without it, resources leak on exceptions.

### `==` to Compare with `None`/`True`/`False`
**Severity**: low

- Rule: Use `is None`/`is not None` (singleton identity check). Use truthiness directly for booleans: `if flag:` not `if flag == True:`.

### Using `len()` for Empty Checks
**Severity**: low

- Rule: Use truthiness: `if not my_list:` not `if len(my_list) == 0:`. Empty containers are falsy.

### Returning Inconsistent Types
**Severity**: medium

- Rule: Don't return `False` or `-1` for "not found". Return `Optional[T]` with `None`, or raise a specific exception.

### Unnecessary List Comprehension
**Severity**: low

- Rule: Use generator expressions with `sum()`, `any()`, `all()`, `min()`, `max()`, `"".join()`. Use dict/set comprehensions directly.
- Fix: `sum(x*x for x in data)` not `sum([x*x for x in data])`.

### Catching and Re-raising Without Context
**Severity**: medium

- Rule: Use `raise NewError(...) from original` to preserve exception chain (PEP 3134). Don't catch-and-reraise without adding logging or context.

---

## Code Smells

### God Class / God Module
**Severity**: high

- Rule: Classes with 5+ unrelated responsibilities violate SRP. Split into focused classes that each own one concern.

### Long Parameter Lists
**Severity**: medium

- Rule: Functions with 5+ parameters are error-prone. Group related params into dataclasses or typed dicts.

### Deep Nesting
**Severity**: medium

- Rule: Max nesting depth of 3. Use guard clauses (early returns/raises) and extract validation into helpers.

### Feature Envy
**Severity**: medium

- Rule: When a method accesses another object's internals extensively, move the behavior to the data-owning class.

### Boolean Parameter Anti-Pattern
**Severity**: medium

- Rule: Multiple boolean params create unreadable call sites. Use enums, separate methods, or keyword-only args.

### Reinventing Standard Library
**Severity**: medium

- Rule: Use `Counter` for counting, `defaultdict(list)` for grouping, `chain.from_iterable` for flattening. Don't rewrite stdlib.

---

## Performance Anti-Patterns

### String Concatenation in Loops
**Severity**: medium

- Rule: `+=` on strings in loops is O(n^2). Use `"".join()` or `io.StringIO`.

### Not Using Generators for Large Data
**Severity**: medium

- Rule: Use `yield` for large datasets. Generators process one item at a time with O(1) memory.

### Repeated Attribute Lookups in Loops
**Severity**: low

- Rule: Cache method references in local variables for tight loops over millions of items. List comprehensions are also faster.

### Unnecessary Copies and Conversions
**Severity**: low

- Rule: Don't `list(dict.keys())` to iterate (just iterate dict), don't convert set to list before `in` (destroys O(1) lookup).

### N+1 Query Pattern
**Severity**: high

- Rule: Never query inside a loop. Use JOINs, eager loading (`joinedload`, `selectinload`), or batch fetching with `IN`.

### Global Mutable Cache
**Severity**: medium

- Rule: Naive dict caches grow without bound. Use `lru_cache(maxsize=N)` or `cachetools.TTLCache`.

### Inefficient Membership Testing
**Severity**: medium

- Rule: `in` on list is O(n); on set/frozenset is O(1). Use `frozenset` for constant lookup tables.

### Object Creation in Tight Loops
**Severity**: low

- Rule: Pre-compile regexes with `re.compile()`. Hoist expensive object creation out of hot loops.



