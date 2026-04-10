# OpenAPI Specification Patterns

## Design-First vs Code-First

| Approach | Use When | Trade-offs |
|----------|----------|------------|
| Design-first | Public APIs, multi-team, contract-driven | Better API design, parallel frontend/backend; requires spec tooling |
| Code-first | Internal APIs, rapid iteration, single team | Faster initial development; spec drifts from implementation |

- **Default to design-first for public APIs** -- the spec IS the contract
- Code-first is acceptable for internal services where the team owns both producer and consumer
- Never mix approaches in the same API -- pick one and enforce it

## Spec Quality Rules

### Structure

- **Use `$ref` for schemas**: Define once in `components/schemas/`, reference everywhere. Never inline complex schemas in endpoint definitions.
- **Mark required fields**: Every request schema must explicitly list `required` fields. Omitting `required` makes all fields optional by default.
- **Use `operationId`**: Every endpoint needs a unique `operationId` -- code generators use it for method names.
- **Consistent naming**: Schema names in `PascalCase`, properties in `snake_case` (match JSON convention).

### Documentation

- **Examples on every endpoint**: Include `example` or `examples` on request body AND response. Reviewers and code generators depend on them.
- **Document all error responses**: List every status code the endpoint can return (400, 401, 403, 404, 422, 429, 500). Don't just document the happy path.
- **Description on every parameter**: Even "obvious" params like `id` benefit from constraints (format, min/max, pattern).
- **Tags for grouping**: Group endpoints by resource (`Users`, `Orders`) using tags. One primary tag per endpoint.

### Versioning in Specs

- Separate spec files per major version: `openapi-v1.yaml`, `openapi-v2.yaml`
- Mark deprecated endpoints: `deprecated: true` with description noting the replacement
- Include `x-sunset` extension for sunset date if using deprecation lifecycle

## Linting with Spectral

Run Spectral in CI to enforce spec quality. Catches common mistakes before review.

### Recommended Setup

```yaml
# .spectral.yaml
extends: ["spectral:oas", "spectral:asyncapi"]
rules:
  operation-operationId: error
  operation-description: warn
  oas3-valid-schema-example: error
```

- `spectral:oas` is the built-in OpenAPI ruleset -- covers most quality checks
- Add custom rules for project conventions (naming patterns, required headers, etc.)
- Run in CI: `spectral lint openapi.yaml --fail-severity=error`
- Block merges on Spectral errors; warnings are advisory

## Contract Testing

Validate that your implementation matches the spec.

| Tool | Approach | Best For |
|------|----------|----------|
| Schemathesis | Property-based (generates random valid requests) | Finding edge cases, fuzzing |
| Dredd | Deterministic (runs spec examples) | Regression testing, CI gates |
| Prism | Mock server from spec | Frontend development, parallel teams |

- **Schemathesis** for public APIs: generates thousands of valid requests from the spec, catches unexpected 500s and schema violations
- **Dredd** for CI gates: runs the examples from your spec against the live API
- Cross-reference: see `skills/testing-patterns/references/integration-testing.md` for Pact consumer-driven contract testing

### CI Integration

1. Lint spec with Spectral (fast, runs on spec file only)
2. Generate mock server with Prism for frontend CI
3. Run Dredd against staging after deploy
4. Run Schemathesis nightly (slower, more thorough)

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| Inline schemas everywhere | Duplication, inconsistency across endpoints | Use `$ref` to `components/schemas/` |
| No examples | Code generators produce useless stubs | Add `example` on every endpoint |
| Spec generated but never validated | Spec drifts from implementation | Contract test in CI (Dredd/Schemathesis) |
| Single spec file for all versions | Breaking changes break all consumers | Separate files per major version |
| Missing error responses | Clients don't handle errors correctly | Document all possible status codes |
| `additionalProperties: true` default | Accepts any garbage without validation | Set `additionalProperties: false` on request schemas |
