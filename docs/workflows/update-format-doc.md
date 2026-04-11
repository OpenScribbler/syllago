# Format Doc Update

**Invoked by:** capmon-process (Step 2 of the local remediation loop)

**Purpose:** Given new or changed source content for a provider, update the
provider's format doc YAML to reflect what the sources actually say.

## Inputs

- PROVIDER_SLUG — the provider identifier (e.g., amp, claude-code)
- FORMAT_DOC — path to existing docs/provider-formats/<slug>.yaml (absent for new providers)
- CHANGED_SOURCES — one or more raw.bin files under .capmon-cache/<slug>/, each
  containing the full fetched content for one source URI
- CANONICAL_KEYS — docs/spec/canonical-keys.yaml, the authoritative vocabulary

## Your job

Read each changed source in full. Do not summarize or excerpt — the format doc
must capture the full picture from the source material. Compare against the
existing format doc. Update it to reflect what you learned.

For each content type the provider supports:

**1. Map known capabilities to canonical keys.**
Use only keys defined in docs/spec/canonical-keys.yaml under the matching
content type. If the source material confirms a capability that matches a
canonical key, record it in canonical_mappings with mechanism and confidence.

The `supported` field is always a boolean. For capabilities that are absent
from or not documented in the source material, use `supported: false` with
`confidence: unknown`. Never write `supported: unknown` — the schema will
reject it.

Write `mechanism` fields narrowly. A mechanism describes only how THIS specific
canonical key is implemented — not adjacent features, not creation flows, not
invocation behaviors that belong to a different key. If information is relevant
to more than one key or to a provider_extension, put it in the most specific
place and keep the other fields clean. For example: if a provider supports both
user-triggered invocation (@mention) and UI-based skill creation, the
`user_invocable` mechanism covers only the invocation syntax, not the UI
creation path (which goes in a provider_extension).

**2. Capture unknown capabilities in provider_extensions.**
If a provider supports something real and documented that has no canonical key,
add it to provider_extensions. Give it:
- A stable id (snake_case, unique within this provider+content_type)
- A clear name
- A description of what it does and why it matters
- A source_ref pointing to the specific page or file where you found it
- graduation_candidate: false (default — set true only if you have positive
  evidence another provider already has the same concept)

**3. Assign confidence using the defined predicates.**
- confirmed: Stated explicitly in source code (struct field, type annotation)
  OR by an unambiguous official documentation statement that directly names and
  describes the field or behavior
- inferred: Appears in examples or is implied by documentation that does not
  formally define it
- unknown: You believe the capability exists but no source material clearly
  confirms or denies it

When in doubt, prefer inferred over confirmed. confirmed must be traceable to
a specific passage you can cite.

**4. Preserve existing content unless contradicted.**
Do not remove or downgrade existing canonical_mappings or provider_extensions
entries unless new source material explicitly contradicts them. If ambiguous,
keep the entry and lower confidence if appropriate.

**5. Capture behavioral nuance in prose fields.**
The loading_model and notes fields are for prose detail: loading semantics,
scope inheritance rules, truncation behavior, edge cases. This is where
provider-specific context lives when it does not map to a structured field.

## Output

A valid YAML file at docs/provider-formats/<slug>.yaml conforming to the format
doc schema. Update last_fetched_at and content_hash on each changed source
entry. Set generation_method to subagent.

## Do not

- Invent canonical keys. If no canonical key exists for a capability, use
  provider_extensions. Never add to canonical_mappings a key that is not in
  docs/spec/canonical-keys.yaml.
- Set graduation_candidate: true without evidence that another provider has the
  same concept.
- Summarize source content. Full detail is required.
- Bleed adjacent concepts into a mechanism field. Each mechanism describes one
  thing. If a feature spans multiple canonical keys or belongs in a
  provider_extension, split it — do not concatenate it into a single mechanism.
- Write `supported: unknown` — `supported` is always a bool. Use `supported: false` +
  `confidence: unknown` when a capability is absent from source material.
- Modify any file other than docs/provider-formats/<slug>.yaml.
