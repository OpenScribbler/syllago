---
id: "0020"
title: Canonical Hook Encoder for Install, Convert, and Export
status: accepted
date: 2026-07-15
enforcement: advisory
files: ["cli/internal/converter/*", "cli/internal/installer/hooks.go", "cli/internal/loadout/apply.go"]
tags: [hooks, conversion, encoder, architecture]
---

# ADR 0020: Canonical Hook Encoder for Install, Convert, and Export

## Status

Accepted

## Context

`syllago install` and `loadout apply` write **dead config** for most hook
providers (bead `syllago-igkju`, surfaced by the PR #512 codex review). They
merge a Claude-Code-shape matcher group into `<ConfigDir>/settings.json` for
every provider except claude-code and crush, ignoring each provider's declared
`ConfigLocations[catalog.Hooks]` and its native hook format. The write succeeds
and is reported as success; the provider never reads it. PR #512 fixed the
*event* dimension (reject events a provider can't read) and the crush case; the
*file/format* dimension remains for ~10 providers.

The root cause is that `internal/converter` contains **two competing
per-provider hook encoders**, and the install path uses neither:

1. **`HooksConverter.Render` → `converter.Result{Content, Filename, ExtraFiles}`**
   (`converter.go`, `hooks.go`). This is what `syllago convert` and `export`
   actually use. It is an ad-hoc `switch target.Slug` that specializes
   copilot / crush / kiro / cursor / windsurf and sends CC + gemini (and, by
   fallthrough, pi and factory-droid) to a generic CC-shape "standard" renderer
   — which is likely wrong for pi and factory-droid. It has no symmetric
   per-provider decode; canonicalization is a separate code path.

2. **`HookAdapter` (`Encode` / `Decode` / `Capabilities` / `Verify`) →
   `EncodedResult{Content, Filename, Scripts}`** (`adapter*.go`, 10 providers).
   A clean, symmetric interface with round-trip verification. It is dormant in
   production — only `Capabilities()` is live (via
   `hookhelpers.TranslateHandlerType`); `Encode`/`Decode`/`Verify` are exercised
   only by adapter unit tests.

3. **Install/apply use neither** — a naive `sjson` append into
   `hooks.<event>` plus SHA-hash tracking of the appended matcher group
   (`installer/hooks.go`, `loadout/apply.go`).

Additional constraints discovered during investigation:

- **Three storage models**, from `ConfigLocations[catalog.Hooks]`: shared JSON
  under a `hooks` key that must be merged, never overwritten (claude-code,
  crush, cursor, gemini, factory-droid; amp uses an `amp.hooks` array);
  dedicated hooks file (windsurf, codex); directory of per-hook files (copilot,
  kiro, pi, cline).
- Both encoders emit a **whole hooks-only file** (`{"hooks": {...}}`). Writing
  that directly over a shared config would clobber sibling keys
  (`permissions`, `env`, MCP servers, model settings). Install must extract the
  hooks portion and merge it, preserving siblings — mirroring the MCP install
  pattern (`installer.ExtractServerEntries` + per-key `sjson.SetRawBytes`).
- Correct **uninstall/status** requires a per-provider **decode** to find and
  remove one specific hook. The current SHA-of-appended-group match breaks the
  moment the hooks subtree is re-serialized by an encoder.
- Three hook providers have **no adapter** (amp, cline, codex).
- Whatever wins, **install, convert, and export must all agree** — a single
  canonical encoder, not two.

## Decision

Adopt the **`HookAdapter` model** (`Encode` / `Decode` / `Capabilities` /
`Verify`) as the single canonical per-provider hook encoder for install, apply,
convert, and export.

Rationale over the `Render`/`Result` path:

- It is the only path with a **symmetric per-provider `Decode`**, which
  identity-based uninstall/status require (decode the provider's file → find the
  hook by name+event+command → remove → re-encode).
- It has broader, more correct per-provider coverage (e.g. pi TS-template
  hooks, factory-droid) where `Render` falls back to a wrong CC-shape.
- `Verify` gives install a built-in encode→decode fidelity check.
- The interface is the cleaner long-term abstraction; `Render`'s `switch` is
  legacy ad-hoc code.

Implementation is phased, each phase its own PR:

- **Phase 1 (resolves `syllago-igkju`)** — route `installHook` and
  `applyHook` through `converter.AdapterFor(slug)`. Add a per-storage-model
  write layer that takes the adapter's encoded hooks and:
  - *shared-JSON providers*: merges the `hooks` value into the real file at
    `ConfigLocations[catalog.Hooks]`, preserving all non-hook keys;
  - *dedicated-file providers*: writes the file;
  - *directory providers*: writes the per-hook file(s) into the directory.

  Reject providers with no adapter (amp, cline, codex) and hookless providers
  with an honest error — never write dead config, never report a no-op as
  success. Move uninstall/status to **identity-based** matching via `Decode`,
  not `GroupHash`. Retire the `prov.Slug == "crush"` hardcode introduced in
  PR #512 (crush becomes just another adapter-routed provider).

- **Phase 2** — migrate `convert` and `export` off `HooksConverter.Render`
  onto the adapters, so all four commands share one encoder. Validate that
  adapter output matches or improves on current convert output before cutting
  over (guard the behavior change with golden tests).

- **Phase 3** — remove the now-redundant `Render` hook path (`render*Hooks`,
  the `HooksConverter.Render` switch, and `converter.Result`'s hook usage).

**Never** write an adapter's whole-file output directly over a shared config —
always merge the hooks portion.

**Alternative considered — `Render` as canonical (reuse for install):** smaller
initial blast radius and install would match convert immediately, but it
perpetuates an ad-hoc switch with known coverage gaps (pi, factory-droid), has
no clean per-provider decode for uninstall/status, and still needs the same
per-storage-model merge layer. Rejected because the decode requirement and the
coverage gaps make the adapter model the better foundation.

## Consequences

**What becomes easier:**
- One canonical encoder: install, apply, convert, and export agree by
  construction.
- Identity-based uninstall/status via `Decode` — no fragile raw-bytes hashing.
- Honest rejection of unserializable providers instead of silent dead config —
  extends PR #512's principle to the file/format dimension.
- Per-provider correctness (pi, factory-droid, etc.) instead of a CC-shape
  fallback.

**What becomes harder:**
- Larger migration than a one-line route: `convert`/`export` move off `Render`
  (Phase 2), a behavior change that must be validated against current output.
- The per-storage-model merge layer carries **data-loss risk** on shared
  configs (clobbering `permissions`/`env`/MCP) if implemented wrong — requires
  per-provider, per-storage-model golden tests.
- `Verify`, currently test-only, becomes production-load-bearing.

**What's deferred:**
- Adapters for amp, cline, codex (rejected with an honest error until written).
- Any provider whose native hook schema is not yet modeled by an adapter.
- Removal of the `Render` hook path (Phase 3).

**Interaction with `syllago-kh64o`:** that bead assumes the *adapters* are the
dead code to retire. This decision inverts that — the adapters become canonical
and `Render`'s `render*Hooks` path is the code retired (Phase 3). If this ADR is
accepted, `syllago-kh64o` must be reframed (retire `Render` hooks, keep
adapters) or closed in favor of Phase 3.
