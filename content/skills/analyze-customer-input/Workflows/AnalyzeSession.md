# AnalyzeSession Workflow

> **Trigger:** `/analyze-customer-input`, "analyze this transcript", "extract from", "what did the customer ask for", "analyze calls for"

## Purpose

Extract and classify items from customer-provided content into **Use Cases**, **Feature Requests**, and **Observations**. Present a structured review to the analyst, resolve ambiguities, run dedup checks, and write approved items to Jira. No Jira writes occur without explicit analyst approval.

---

## Phase 1: Setup

Ask as a single opening message:

> "Let's analyze this customer content.
> - What's the Jira label for this customer? (e.g., `customer-acme`)
> - Please paste or attach the content you'd like me to analyze — transcript, doc, diagram, or other."

If the content was already provided with the trigger, only ask for the Jira label:

> "Before I analyze, what's the Jira label for this customer?"

Capture: `customer_label`, `source_content`, `source_type` (transcript / document / other).

**Do not proceed until both are provided.**

### Multi-Source Analysis

When analyzing multiple sources (e.g., several Gong calls, a call plus a document), process all sources before classifying. This enables:

- **Cross-source consolidation** — the same use case or FR may be discussed across multiple calls as it evolves. Consolidate into a single item with evidence from all sources rather than creating duplicates.
- **Cross-source dedup** — if the same ask appears in call 1 and call 3, create one item with evidence quotes from both, not two items.
- **Evolution tracking** — note when an item evolved across sources (e.g., "discussed conceptually in Call 1, concrete requirements in Call 3"). Use the most detailed/recent version as the primary description, with earlier mentions as supporting evidence.
- **Temporal relevance** — when sources span months, items from early calls may no longer be relevant. For each item, check whether it was confirmed or revisited in the most recent calls. Classify items as:
  - **CURRENT** — discussed or confirmed in the most recent call(s)
  - **STALE** — only appeared in older calls and was never revisited
  Present stale items separately in the review so the analyst can decide whether to include them. Do not create Jira tickets for stale items unless the analyst explicitly overrides.

Build affiliation maps (Phase 2) for each source separately, then merge. If a speaker appears across multiple sources, use the most specific affiliation/role information available.

### Gong Call Fetching

If the source is Gong calls, load `references/gong-fetching.md` and follow the fetching protocol before proceeding to Phase 1b. Do not begin extraction until all calls have been fetched and all pages of search results have been consumed.

---

## Phase 1b: Load Existing Context

If a Jira epic key was provided (in the trigger or by the analyst), read the epic and ALL child tickets. First read the epic with `getJiraIssue` (pass `responseContentFormat: "markdown"`). Then discover child tickets with `searchJiraIssuesUsingJql` using `parent = [epic-key]`. Read each child ticket with `getJiraIssue`. Do not skip any child tickets.

If no epic key was provided, ask the analyst:

> "Do you have an existing Jira epic for this customer? Providing one enables me to compare new extractions against existing tickets to find enrichment opportunities."

If the analyst provides a key, fetch as above. If they say no, note that enrichment comparison (Phase 4c) will be skipped and semantic dedup (Phase 6, Step 1) will rely on keyword search only.

### Ticket Manifest

After reading existing tickets, hold a ticket manifest in memory. This manifest will be persisted along with extraction results in Phase 4b. For each child ticket, record:
- **Ticket key** (e.g., SOL-103)
- **Summary** (the ticket title)
- **Issue type** (Story, Feature Request, etc.)
- **One-sentence description** of what the ticket covers

This manifest is the authoritative list for semantic dedup in Phase 6 and enrichment comparison in Phase 4c. If the agent restarts, the manifest in the persisted file avoids re-fetching all tickets.

---

## Phase 2: Identify Source Type and Participants

### If source is a Gong transcript (has a `participants` table)

Build an affiliation map from the `participants` table:

- Collect all `name` values where `affiliation` is `"External"` → extraction targets
- Collect all `name` values where `affiliation` is `"Internal"` → context only, no extraction
- Collect any speaker names in the transcript that have no matching participant entry → flag as Unknown

**Name collision check:** If two or more non-Internal participants share the same display name, note it for the review output. The analyst will need to interpret affected utterances manually.

**Retain full transcript for context.** Do not strip Internal utterances — they provide the conversational context needed to understand what External speakers were responding to.

### If source is a document or non-transcript artifact

All content is in scope for extraction. Set `speaker: "Document"` for all extracted items.

---

## Phase 3: Extract and Classify

**Before extracting, load `references/extraction-depth.md`.** Your extraction target is the **Operational** level — every item must capture specific technical constraints, versions, protocols, failure modes, and quantitative details, not topic-level summaries.

Read the full source content. For each External (or non-Internal) speaker utterance, identify items and classify them using the decision tree from SKILL.md.

### Classification pass

For each item identified, apply in order:

1. **Can I name a source workload AND a target resource?** → **Use Case**
2. **Is the customer asking for a product capability that doesn't exist today?** → **Feature Request**
3. **Neither?** → **Observation**

Then apply the boundary rules:
- If it describes HOW a use case works and the capability exists → fold into that Use Case as an environment detail
- If it describes a missing capability needed across use cases → Feature Request
- If it's part of a use case's environment BUT the integration/capability doesn't exist today → include as a UC environment detail AND create a separate FR
- Procurement, timelines, internal politics → Observation
- Item raised AND resolved during the engagement → flag as resolved, no ticket. "Resolved" means a concrete question or concern was answered to the customer's satisfaction during the calls — not general topics that were simply discussed. When classifying an item as resolved, you must include both the original ask (verbatim quote) AND the resolution evidence (the specific statement showing it was addressed). If you cannot find an explicit resolution statement in the source, classify the item normally — do not infer resolution from topic change or silence.

### Extraction rules (apply to all categories)

1. Every item must have a **verbatim quote** from the source as evidence. No quote = do not create the item.
2. `title` must be 80 characters or fewer, written as a clear summary.
3. `speaker` is the display name of the person who said it, plus affiliation or title if available.
4. `source` is the callId (for Gong transcripts) or filename/description (for documents).
5. Internal-only utterances must not produce extracted items.
6. For each item, follow multi-turn discussion threads to their conclusion. A single quote is the minimum evidence requirement — but the extraction must capture ALL specific technical details mentioned across the discussion thread, not just the first quote. See the Multi-Quote Threading Rule in `references/extraction-depth.md`.
7. Observations that capture technical environment details, constraints, or architecture specifics must also meet the Operational depth bar from `references/extraction-depth.md`. An Observation like "Customer uses Azure" is Topic-level and must be deepened before it can be folded into a ticket.

### Use Case fields

For each Use Case, load `references/uc-template-guide.md` and populate every field you can from the evidence. Leave fields empty when the source doesn't contain the information — these are gaps the SE can fill during scoping.

### Feature Request fields

For each FR, load `references/fr-template-guide.md` and populate every field you can from the evidence. `poc_blocking` is `true` only when the customer explicitly says the absence of this feature blocks an active POC. Do not infer from urgency alone.

### Observation fields

For each Observation, capture: **title** (short summary), **description** (what was observed), and **evidence** (verbatim quotes with speaker and source).

---

## Phase 4: Ambiguity Resolution

After the initial classification pass, collect any items where:
- The classification is uncertain (could be UC or FR, could be FR or Observation)
- Evidence supports multiple interpretations
- An item partially matches multiple categories

Present all ambiguous items together with your reasoning:

```
## Items Needing Classification

The following items could fit multiple categories. Please confirm or reclassify:

1. **[title]**
   Evidence: "[quote]"
   My classification: Feature Request
   Reasoning: Customer asks for OPA policy support, which doesn't exist today
   Alternative: Could be an Observation if this is aspirational rather than a concrete ask
   → Keep as FR / Reclassify as [category] / Drop?

2. **[title]**
   ...
```

If there are no ambiguous items, skip this phase and say so briefly.

**Do not proceed to the review until all ambiguities are resolved.**

---

## Phase 4b: Persist Extraction

After classification and ambiguity resolution, write the full extraction to a temp file so it survives agent restarts and avoids re-fetching source content:

```bash
/tmp/<customer>-analysis-<timestamp>.md
```

Include all use cases, FRs, observations, resolved items, and ambiguous items with their full detail and evidence quotes. This file is the authoritative record of the extraction — any subsequent Jira writes should read from it rather than re-analyzing source content.

---

## Phase 4c: Enrichment Comparison (when existing tickets are available)

This phase activates ONLY when you have both transcript extractions AND existing Jira tickets for this customer (e.g., you read an epic and its children as part of the task). Skip this phase if there are no existing tickets to compare against.

For EVERY existing ticket you read, compare it against your transcript extraction:

1. List all specific technical details from the transcripts that are relevant to this ticket's scope
2. Check which of those details are already captured in the ticket
3. Flag any details that are in the transcripts but NOT in the ticket

Classify each existing ticket into one of three categories:

- **NEEDS UPDATE** — ticket is missing substantive technical details from transcripts that would change engineering's understanding of the requirement (specific constraints, failure modes, environment details, integration specifics)
- **ENRICHMENT AVAILABLE** — ticket passes readiness but transcripts contain additional context that would improve it (additional verbatim quotes, environment specifics, workaround details, scale numbers)
- **COMPLETE** — ticket already contains everything relevant from the transcripts. You must explicitly list which transcript details you compared against to reach this conclusion.

**Do NOT mark any ticket as COMPLETE without showing your comparison.** "Looks good" or "passes readiness" is not a valid assessment. State what transcript evidence you checked and why it's already covered.

**Assess every ticket, but only produce update files when there is substantive new information.** For tickets where the transcripts contain no relevant new details, note "No enrichment available - [reason]" in the analysis rather than producing an empty update file.

Use the Enrichment Checklists in `references/fr-template-guide.md` (for FRs) and `references/uc-template-guide.md` (for use cases) to guide the comparison.

Include the enrichment comparison results in the persisted analysis file and in the Draft Review (Phase 5).

### Overlap with new extractions

If an extracted item covers the same scope as an existing ticket flagged as NEEDS UPDATE or ENRICHMENT AVAILABLE in this phase, do NOT create a new ticket for that item in Phase 7. Instead, merge the extracted details into the Phase 4c update for the existing ticket. Only items with no existing ticket match should proceed to Phase 6 dedup as new items.

---

## Phase 5: Draft Review

Present the full classified extraction before taking any Jira action. Group by category (Use Cases, Feature Requests, Observations, Resolved Items). Include counts per category.

For each item, present: **title**, **classification**, **key evidence quote**, **speaker (affiliation)**, and any flags (POC-blocking, stale, ambiguous speaker, name collision).

If enrichment comparison was performed (Phase 4c), include the per-ticket assessment (NEEDS UPDATE / ENRICHMENT AVAILABLE / COMPLETE).

If any extracted items were merged into existing ticket updates per the Phase 4c overlap rule (instead of being proposed as new tickets), include a **Merged Items** section:

```
## Merged Items

The following extracted items overlap with existing tickets and will be folded
into their updates rather than created as new tickets:

1. **[extracted item title]** → merged into [SOL-XXX] — [one-line rationale]
2. **[extracted item title]** → merged into [SOL-YYY] — [one-line rationale]

For each: approve merge / split out as new ticket / override
```

The analyst must explicitly approve each merge. If they split an item out, it proceeds to Phase 6 dedup as a new item.

After presenting, ask the analyst to approve, edit, reclassify, or reject items.

If zero items are extracted across all categories, say so plainly and ask the analyst if they want to review the content together or try a different source.

### POC-Blocking Gate

If any item (UC or FR) has `poc_blocking: true`:

> "The following items are marked POC-blocking and require explicit acknowledgment:
>
> - Item [N]: [title]
>
> Please confirm each one individually before I proceed."

Bulk approval does not satisfy this gate.

---

## Phase 6: Dedup Check

Dedup has two steps: semantic comparison (primary) and keyword search (safety net). Both must pass before creating a new ticket.

### Step 1: Semantic comparison against known tickets

Using the ticket manifest from Phase 1b, compare each proposed new item against every listed ticket. If no ticket manifest exists (Phase 1b was skipped), skip this step and rely on keyword search only. Ask: **"Is this the same product ask, even if worded differently?"**

Two tickets are duplicates if they request the same product capability, even if:
- Titles use different words ("single-step token exchange" vs "collapse auth and credential API calls")
- One is more specific than the other
- They cite different evidence for the same underlying need

For each proposed new item, show the comparison:

```
Dedup (semantic) for "[new item title]":
  Compared against: SOL-103, SOL-104, SOL-105, SOL-106, SOL-108, SOL-109
  Result: NOT a duplicate — none of these cover [specific reason]
```

Or:

```
Dedup (semantic) for "[new item title]":
  Compared against: SOL-103, SOL-104, SOL-105, SOL-106, SOL-108, SOL-109
  DUPLICATE of SOL-106 — both request reducing the two-step auth+credential API flow
  Recommendation: Update SOL-106 with new evidence instead of creating a new ticket
```

If a duplicate is found, propose updating the existing ticket with new evidence rather than creating a new one. The analyst decides.

### Step 2: Keyword search (safety net)

For items that pass semantic dedup, also search Jira for tickets you may not have read. Use `searchJiraIssuesUsingJql` via MCP:

```
JQL: project = SOL AND text ~ "[keywords]" AND labels = "[customer_label]"
```

Use multiple keyword variations - not just the title. For example, for a "single-step credential fetch" FR, also search for "two-step", "auth latency", "API calls", etc.

- If matches found, present them before writing:

```
Dedup (keyword) for "[title]":
- Possible match: [JIRA-123] [existing title] — [status]

Options: (a) Link to existing issue, (b) Update existing issue, (c) Create new issue
```

The analyst makes the final call per item. Do not proceed without a decision on each match.

If no matches are found, say so briefly and proceed.

---

## Phase 7: Jira Write

Ask the analyst which epic to target (if not already provided):

> "Which epic should these items go under? Please provide the epic key (e.g., SOL-XX)."

If the analyst does not have the key, offer to search via MCP:

```
searchJiraIssuesUsingJql: project = SOL AND issuetype = Epic AND text ~ "[customer_name]"
```

Present the results and let the analyst choose. Do not auto-select.

### Creating new tickets

For new FRs, generate a Markdown description following the FR template guide convention, then create via MCP:

```
createJiraIssue:
  cloudId: aembit.atlassian.net
  projectKey: SOL
  issueType: Feature Request
  summary: [FR title]
  description: [Markdown following FR convention]
  contentFormat: markdown
  parent: [epic-key]
  labels: ["feature-request", "[customer-label]"]
```

If `poc_blocking` is true, also add the `poc-blocking` label to the labels array.

For new UC stories, generate a Markdown description following the UC template guide convention:

```
createJiraIssue:
  cloudId: aembit.atlassian.net
  projectKey: SOL
  issueType: Story
  summary: [UC name]
  description: [Markdown following UC convention]
  contentFormat: markdown
  parent: [epic-key]
  labels: ["use-case", "[customer-label]"]
```

### Updating existing tickets (from enrichment comparison)

When Phase 4c classified existing tickets, apply the analyst-approved results here:

- **NEEDS UPDATE** — read the current description, merge the missing substantive details into the appropriate template sections, then update via MCP. These are mandatory updates when approved by the analyst.
- **ENRICHMENT AVAILABLE** — same read-modify-write flow, but only update if the analyst approved the enrichment in Phase 5. These are optional improvements.
- **COMPLETE** — no action needed. Do not touch these tickets.

For each update, read the current description, modify the Markdown in-memory, then update:

```
getJiraIssue: issueKey, responseContentFormat: "markdown"
  -> modify Markdown
editJiraIssue: issueKey, description: [updated Markdown], contentFormat: "markdown"
```

For FRs, follow the FR template guide convention. For UC stories, follow the UC template guide convention.

### Observations

Observations are not written to Jira as standalone tickets. However, when creating or updating UC or FR tickets above, cross-reference the extracted Observations. If an Observation contains environment details, timeline context, or technical constraints relevant to a specific UC or FR, fold that context into the ticket's description — in the Environment, Current Authentication, or Supporting Materials section as appropriate.

Before writing tickets, present a brief fold plan to the analyst showing which Observations will be included in which tickets and where:

```
Observation folds:
- Observation 3 (AKS 1.28 environment details) → UC-1 Environment section
- Observation 5 (SOC2 audit timeline) → FR-2 Business Context section
- Observations 1, 2, 4 — no fold (not relevant to any specific ticket)

Approve these folds, or adjust?
```

The analyst can approve or reject individual folds. Do not force-fit unrelated observations. Only attach context that would help engineering or the SE understand the ticket better.

---

## Phase 8: Report Completion

After all writes are complete:

```
Analysis complete — [Source Title / callId]

Use Cases:
  [JIRA-123] [name] (created)

Feature Requests:
  [JIRA-456] [title] (created)
  [JIRA-789] [title] (updated)

Observations (informational — not in Jira):
  [title]
  [title]

Items rejected by analyst:
  [title] — rejected

Customer label: [customer_label]
```

If any Jira write fails, report the error and the item that failed. Do not silently skip failed writes.

---

## Error Handling

| Situation | Action |
|-----------|--------|
| No customer Jira label provided | Do not begin extraction — ask for it |
| Item has no traceable verbatim quote | Discard it; do not include in review output |
| Classification is ambiguous | Present in Phase 4 for analyst resolution — do not guess |
| POC-blocking item not explicitly acknowledged | Block approval of the full batch until acknowledged |
| MCP search returns no results | Note "no matches found" and proceed |
| MCP search fails | Report the error; ask analyst whether to proceed without dedup or retry |
| MCP create/update fails | Report the specific item and error; do not skip silently |
| Unknown speakers present | Include their utterances in extraction; flag in Ambiguous Speakers section and assume they are non-Internal participants whose quotes are relevant |
| Name collision among non-Internal participants | Add Name Collision Warning; do not disambiguate automatically |
| Source is ambiguous type | Ask analyst: "Is this a Gong transcript with a participants table, or another type of document?" |
| Item raised and resolved during engagement | Flag as resolved in the review; do not create a ticket |
