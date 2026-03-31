# ADR 0001: Hook Degradation Enforcement

## Status

Accepted

## Context

When converting hooks between providers, some capabilities aren't supported by the target. For example, a `type: "prompt"` hook (LLM-evaluated) can't run natively on Gemini CLI. The hooks spec defines a `degradation` field on `CanonicalHook` that lets hook authors declare what should happen when a capability is missing:

- `"block"` — Conversion must fail. The hook is safety-critical and silent degradation is dangerous.
- `"warn"` — Include the hook in degraded form with a warning. Partial operation is acceptable.
- `"exclude"` — Drop the hook entirely with a warning. The missing capability *is* the hook's purpose.

The spec also defines **default degradation strategies** per capability when the author doesn't specify one:

| Capability | Default | Rationale |
|---|---|---|
| `llm_evaluated` | `exclude` | LLM evaluation is entire hook purpose |
| `http_handler` | `warn` | Can be approximated with generated curl script |
| `input_rewrite` | `block` | Silent drop of input sanitization creates false security |
| `async_execution` | `warn` | Synchronous execution is safe fallback |
| `platform_commands` | `warn` | Default command available |
| `custom_env` | `warn` | Missing env vars may cause errors but not safety issues |
| `configurable_cwd` | `warn` | Wrong directory detectable, not security risk |

During Tier 2 implementation, `ApplyDegradation` was initially deferred as "speculative infrastructure" because no hook content currently uses the `degradation` field. All unsupported handler types were dropped with a warning regardless of any degradation policy.

This created a problem: if a hook author sets `degradation: {"llm_evaluated": "block"}`, they're declaring "this hook is safety-critical — fail the conversion rather than silently dropping it." Our implementation silently dropped it anyway, violating the author's explicit safety contract.

The risk isn't theoretical — it's an architectural invariant. If the spec promises that `"block"` prevents degradation and our implementation ignores it, users who trust the field have a false sense of security. The fact that nobody uses it *yet* doesn't make the contract violation acceptable — it makes it a latent bug waiting for the first adopter.

## Decision

Enforce `degradation` during hook encoding. When an adapter encounters an unsupported capability:

1. Check the hook's `degradation` map for an author-specified strategy
2. If no author strategy, use the spec's default strategy for that capability
3. Apply the strategy:
   - `"block"` — Return an error. The hook is not included. The conversion surfaces the failure.
   - `"exclude"` — Drop the hook. Emit a warning with actionable suggestion.
   - `"warn"` — Drop the hook (for now). Emit a warning with actionable suggestion. When wrapper script generation is implemented, this becomes "include degraded form + warning" instead.

This requires changing `TranslateHandlerType` to accept the hook's `Degradation` map so it can check the author's policy before deciding to drop.

The `"warn"` strategy is partially implemented: we drop with a suggestion rather than including a degraded form. Full `"warn"` support (generating wrapper scripts or curl commands as degraded alternatives) is deferred until there's demand. The key safety property — `"block"` prevents silent degradation — is enforced now.

## Consequences

**What becomes easier:**
- Hook authors can trust the `degradation` field. Setting `"block"` on a security-critical hook guarantees the conversion will fail rather than silently dropping the safety check.
- The spec contract is honored from day one. Early adopters of the `degradation` field get correct behavior.
- `"exclude"` (the default for `llm_evaluated`) works identically to the current drop behavior — no user-visible change for the common case.

**What becomes harder:**
- Conversions that include hooks with `degradation: {"llm_evaluated": "block"}` will fail where they previously succeeded (by silently dropping). This is the intended behavior — the previous "success" was incorrect.
- The `TranslateHandlerType` signature changes from `(HookHandler, string)` to `(HookHandler, string, map[string]string)` — all 5 adapter call sites need updating.

**What's deferred:**
- Full `"warn"` support (degraded wrapper scripts). Currently `"warn"` behaves like `"exclude"` + suggestion. This is acceptable because the spec's defaults don't use `"warn"` for any capability where dropping is dangerous — `input_rewrite` defaults to `"block"`, and `llm_evaluated` defaults to `"exclude"`.
