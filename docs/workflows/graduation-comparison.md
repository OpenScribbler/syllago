# Graduation Comparison

**Invoked by:** capmon-process (Step 3 of the local remediation loop, conditional)

**Purpose:** Given that a new provider_extensions entry was just added to a
format doc, check whether any other provider already has a semantically
equivalent extension. If two or more providers have the same concept under
different names, that concept is a graduation candidate.

## Inputs

- CHANGED_PROVIDER — slug of the provider whose format doc was just updated
- NEW_EXTENSIONS — the list of new provider_extensions entries added in this run
- All docs/provider-formats/*.yaml files

## Your job

Read ALL provider_extensions entries across ALL format docs, regardless of the
graduation_candidate flag value. graduation_candidate: false means "not yet
evaluated" — it is not a gate. Do not skip any entry based on this flag.

For each extension in NEW_EXTENSIONS:

1. Read its id, name, and description.
2. Read the provider_extensions sections of all other providers' format docs.
3. Determine: does any other provider have an extension describing the same
   underlying concept? Same concept means the same provider behavior or
   capability, even if named completely differently.

   Example of a match: "Amp bundles an MCP server with a skill" and "Cline
   packages tools inside a skill directory" both describe a mechanism for
   co-locating server-side tooling with skill content. Different names, same
   concept.

   Example of a non-match: one provider has a caching behavior and another
   has a lazy-loading behavior. Superficially related but solving different
   problems — not a graduation candidate.

4. If you find a match across two or more providers: check whether a closed
   capmon-graduation issue already covers this concept pairing before recording
   it. If a closed issue exists, do NOT re-flag — note the prior issue instead.

5. If a match is new (no prior closed issue): record the details.

## Output

For each graduation candidate found, produce one section in this format:

---
## Graduation Candidate: <suggested_canonical_key>

**Concept:** One sentence describing the capability.

**Providers:**
- `<slug>`: extension `<id>` — "<name>" — <source_ref>
- `<slug>`: extension `<id>` — "<name>" — <source_ref>

**Suggested canonical key:** `<snake_case_key>`
**Suggested definition:** One sentence suitable for canonical-keys.yaml.
**Suggested type:** string | bool | object

**Notes:** Any ambiguity, differences in how providers implement this, or
open questions the human reviewer should consider.
---

This output becomes the body of a capmon-graduation GitHub issue.

If no matches are found, produce no output. No issue is created.

## Do not

- Flag tenuous connections. Only flag clear semantic equivalents where two
  providers are clearly solving the same problem with different naming.
- Suggest graduation for concepts only one provider has.
- Modify any file. Your output is a report only.
- Create graduation candidates across different content types (a skills
  extension and a hooks extension cannot graduate to the same key).
