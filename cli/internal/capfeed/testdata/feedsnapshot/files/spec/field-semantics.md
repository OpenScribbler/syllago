# Field Semantics

A reference for the published capmon capability tree. The load-bearing
consumer rules — nil-vs-false, the `confidence` enum and its sweep
downgrade, and enum openness — are normative in the **consumer contract**
(`docs/design/publish-layer.md`, "Consumer contract"). This page carries
the remainder not discharged to ACIF via a node's `spec_ref`.

Every ACIF-owned key inlines its authority at the node: the `key` object's
`spec_ref` points at the owning ACIF spec section. Where a rule below is
already owned by ACIF for a specific key, that key's `spec_ref` governs and
this page only summarizes the cross-cutting mechanics.

## Canonical-key grammar

A canonical key's `key_path` is its dot-joined position in the registry,
`<content_type>.<key>` (e.g. `agents.invocation_patterns`). Every segment
matches:

```
^[a-z_]+(\.[a-z_]+)*$
```

Lowercase ASCII letters and underscores per segment, segments joined by a
literal dot. The grammar **binds canonical keys only**. It does not bind:

- **Vocabulary-member names** — sub-values of an object-typed canonical key
  (e.g. `at_mention` under `invocation_patterns`). These are keys of a
  parent node's `capabilities` map and carry no `key`/`key_path`.
- **`provider_exclusive` node names** — provider-specific capabilities that
  are definitionally not canonical keys. Their original source casing is
  preserved (e.g. a camelCase `baseDir`); the grammar does not apply and
  they never carry `key`/`key_path`.

## The key-presence discriminator

`key` present ⇔ the node is a registered canonical key. This is normative.

- A node **with** `key` (and `key_path`) is a canonical key; its
  `description`, `type`, and `spec_ref` come from the registry.
- A node **without** `key` is a vocabulary member; its meaning lives in the
  nearest ancestor canonical key's `description` and that key's ACIF
  `spec_ref`.

`key.type` describes the concept as registered; it does **not** predict the
node's JSON shape. Consumers MUST inspect nodes structurally (see the
consumer contract).

## Parent-`supported` auto-flip

When a canonical key gains a sub-capability, the pipeline sets the parent
node's `supported` to `true` automatically (`seed.go:91`). Adding a
sub-capability is therefore additive: a reader that only inspects the parent
sees the parent concept as supported without any separate edit. A branch
node thus carries the same disposition fields as a leaf, and a leaf gaining
children never invalidates an existing read.

## `mechanism` provenance

`mechanism` is maintainer-authored prose derived from provider documentation
and source — a human explanation of *how* a provider realizes a capability.
It is not machine-generated and not a stable identifier: consumers MUST treat
it as descriptive text, never parse it, and never key off its exact wording.

## The `conversion` vocabulary

`conversion` describes how portable a provider-native feature is when moved
across providers. It appears in capmon's internal provider-format proposed
mappings (`docs/provider-formats/`), **not** in any published document — it
is pipeline state, listed here only so the ecosystem vocabulary is defined in
one place. ACIF does not currently own it. The observed set:

| Value | Meaning |
|---|---|
| `preserved` | Carries over unchanged to the target provider. |
| `translated` | Expressible on the target, but via a different mechanism. |
| `embedded` | Achievable only by inlining into other content, not as a first-class feature. |
| `not-portable` | No target equivalent; provider-exclusive. |
| `dropped` | Silently lost in conversion. |

Per the consumer contract, every enum-valued field is **open**: an
unrecognized `conversion` value MUST be handled as "unknown/other", never as
an error.

## nil vs. false

`supported` absent means **unknown** — never treat it as `false`.
`"supported": false` is an affirmative "not supported". This distinction is
normative and detailed in the consumer contract's disposition table; it is
the single most important rule for reading the tree correctly.
