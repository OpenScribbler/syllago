---
name: javascript-patterns
description: JavaScript and Node.js development patterns. Use when building or fixing JavaScript/TypeScript services, applications, or scripts.
---

# JavaScript and Node.js Development Patterns

Patterns for writing production-quality JavaScript and TypeScript code.

## Commands

```bash
# Package management (pnpm preferred, npm also supported)
pnpm install && pnpm run build && pnpm test
npm ci && npm run build && npm test

# TypeScript
npx tsc --noEmit                   # Type check only

# Linting and formatting
npx eslint . && npx prettier --write .
```

**Token optimization**: Pipe through `| head -50` for test/tsc output.

## Security Checklist

- [ ] Never log secrets or tokens
- [ ] Validate all user input (use zod or similar)
- [ ] Use parameterized queries (never string concatenation)
- [ ] Set appropriate CORS headers
- [ ] Use `crypto.randomUUID()` for random IDs
- [ ] Sanitize user input before rendering (XSS prevention)
- [ ] Use HTTPS for all external APIs

## References

Load on-demand based on task:

| Task | Reference |
|------|-----------|
| Error handling, async, retry, config patterns | [references/patterns.md](references/patterns.md) |
| Jest, Vitest, mocking, React Testing Library | [references/testing.md](references/testing.md) |
| Vite, esbuild, webpack bundler config | [references/bundling.md](references/bundling.md) |
| Express middleware, routing, auth, validation | [references/express-patterns.md](references/express-patterns.md) |
| JS/TS footguns (this, coercion, async, scope) | [references/gotchas.md](references/gotchas.md) |
| Common mistakes and fixes (code review) | [references/anti-patterns.md](references/anti-patterns.md) |

## Related Skills

| Context | Skill |
|---------|-------|
| Test design and strategy | `skills/testing-patterns/SKILL.md` |
| Code review checklists | `skills/code-review-standards/SKILL.md` |
