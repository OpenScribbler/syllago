# JavaScript/TypeScript Gotchas

Common footguns that bite even experienced developers. Use as a review checklist.

## Type Coercion

- `==` performs type coercion; always use `===` and `!==`. Only exception: `== null` checks both `null` and `undefined`.
- `typeof null === 'object'` — use `value === null` for null checks, not `typeof`.
- `+[]` is `0`, `+{}` is `NaN`, `[] + {}` is `"[object Object]"`. Avoid implicit coercion with `+` operator on non-numbers.

## Numbers

- `0.1 + 0.2 !== 0.3` — IEEE 754 floating point. For currency, use integers (cents) or a decimal library.
- `parseInt('08')` works in modern engines but always pass radix: `parseInt(str, 10)`.
- `NaN !== NaN` — use `Number.isNaN(x)`, not `isNaN(x)` (global `isNaN` coerces argument).

## `this` Binding

- Arrow functions inherit `this` from enclosing scope; regular functions get `this` from call site.
- Gotcha: Class method passed as callback loses `this`. Fix: use arrow function in class field or `.bind(this)` in constructor.
- Gotcha: `this` in `forEach`/`map` callback is `undefined` in strict mode. Use arrow functions.

## Closures and Scope

- `var` in loops shares a single binding across iterations. Use `let` (block-scoped) for loop variables.
- Forgetting `let`/`const` creates implicit globals (strict mode throws `ReferenceError` instead — always use `"use strict"` or ESM).
- Closure over mutable variable captures the variable, not the value. In async loops, `let` fixes this; `var` does not.

## Arrays and Objects

- `for...in` iterates all enumerable properties (including prototype), keys are strings. Use `for...of` for arrays, `Object.keys()`/`Object.entries()` for objects.
- `Array.sort()` mutates in place AND converts elements to strings by default. Always pass a comparator: `.sort((a, b) => a - b)`.
- Spread `{...obj}` and `Object.assign` are shallow copies. Nested objects are still references.
- `delete arr[i]` leaves a hole (`undefined`), doesn't update `.length`. Use `.splice(i, 1)`.

## Async

- `JSON.parse()` throws on invalid input — always wrap in try/catch.
- `forEach` does not await async callbacks. Use `for...of` with `await` for sequential async, or `Promise.all(arr.map(...))` for parallel.
- Unhandled promise rejections crash Node.js (v15+). Always `.catch()` or use try/catch in async functions.

## AbortController + Fetch Timeout

```typescript
async function fetchWithTimeout(url: string, ms: number) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), ms);
  try {
    return await fetch(url, { signal: controller.signal });
  } finally {
    clearTimeout(timeout);
  }
}
```
- Gotcha: Must clear timeout in `finally` to prevent memory leaks on success.
- Gotcha: Aborted fetch throws `AbortError` — catch and handle separately from network errors.

## TypeScript-Specific

- `any` silences all type checking and propagates. Use `unknown` for truly unknown types, then narrow with type guards.
- Non-null assertion `!` hides bugs. Prefer optional chaining `?.` and nullish coalescing `??`.
- Enum values are numbers by default and allow reverse lookup. Use `const enum` or string literal unions for type safety.
