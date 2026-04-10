# LogFeedback Workflow

> **Trigger:** `/log-product-feedback`, "log this FR", "create a Jira ticket for this customer feedback", "the customer mentioned..."

## Purpose

Collect a feature request from the SE and write it to Jira. Walks through the structured FR template, creates a `Feature Request` issue with `feature-request` label via MCP `createJiraIssue`.

The SE can say "skip" or "leave empty" at any point to move past a section.

---

## Phase 1: Quick Context

Ask for the essentials in a single opening message. If the SE has already provided some in their trigger, skip those.

> "Let's log this feature request. To start:
> - What's the customer name or Jira label?
> - What did they ask for? (A short title works.)
> - Do you have a direct quote or evidence?"

Capture: `customer_label`, `title`, initial `supporting_materials` (evidence quote).

If the SE has no verbatim quote, ask once:

> "A direct quote makes the ticket traceable. Do you have one, or should I note it as a paraphrase?"

If confirmed no quote, proceed and note `[Based on SE summary — no verbatim quote available]` in supporting_materials.

---

## Phase 2: The Problem (Template Section 1)

Load `skills/analyze-customer-input/references/fr-template-guide.md` Section 1 for the full question set. Ask:

> "A few questions about the problem:
> - What is the customer unable to do today? What's happening or not happening?
> - What exactly are they asking for (in their words)? Include enough technical detail that an engineer can understand the scope.
> - Anything explicitly NOT in scope? What might someone assume is included but isn't?"

Capture: `problem.unable_to_do`, `problem.asking_for`, `problem.not_in_scope`.

### Depth nudge

If the SE's `unable_to_do` or `asking_for` response is at the topic level (e.g., "they need CrowdStrike integration" with no technical specifics), ask one follow-up:

> "Can you be more specific — what constraint, failure mode, or technical detail did the customer mention? For example: a specific version, protocol, error condition, or incompatibility."

If the SE says they don't have more detail, accept their answer and proceed.

The SE can say "skip" to leave any field empty.

---

## Phase 3: Business Context (Template Section 2)

> "Now the business context:
> - Why does this matter to the customer — how does it connect to their goals?
> - Is there urgency — a deadline, milestone, or event driving the timeline?
> - What's the business impact if this isn't addressed? Check all that apply:
>   - New deal risk
>   - Churn risk
>   - Blocked expansion
>   - Degraded customer experience
>   - Operational inefficiency
>   - Other
> - Any revenue context — ARR, deal size, expansion opportunity?
> - Are other customers asking for the same thing?
> - Is this POC-blocking — does the customer say the absence of this feature blocks an active POC?"

Capture: `business_context.*` fields and `poc_blocking` (true/false). For impact items, capture details for any the SE selects. `poc_blocking` is true only when the customer explicitly says the feature's absence blocks a POC — do not infer from urgency alone.

---

## Phase 4: Workaround (Template Section 3)

> "Last section — workarounds:
> - Is there an existing workaround? (Yes / No / Partial)
> - If yes or partial — what are they doing today?
> - How long can they operate this way?"

Capture: `workaround.*` fields.

---

## Phase 5: Epic Selection

> "Which epic should this FR go under? Please provide the epic key (e.g., SOL-XX). This can be a POC Epic or a Customer Epic."

If the SE does not have the key, offer to search via MCP:

```
searchJiraIssuesUsingJql: project = SOL AND issuetype = Epic AND text ~ "[customer_name]"
```

Present the results and let the SE choose. Do not auto-select.

Capture: `epic_key`.

---

## Phase 6: Draft Review

Present the full draft before any Jira action:

```
Here's the draft FR:

Title: [title]
Epic: [epic_key]
POC-blocking: [Yes/No]

1. The Problem
   Unable to do: [content or "empty"]
   Asking for: [content or "empty"]
   Not in scope: [content or "empty"]

2. Business Context
   Why it matters: [content or "empty"]
   Urgency: [content or "empty"]
   Impact: [checked items with details]
   Revenue: [content or "empty"]
   Other customers: [content or "empty"]

3. Workaround
   Status: [Yes/No/Partial or "empty"]
   Details: [content or "empty"]
   Lifespan: [content or "empty"]

4. Supporting Materials
   [evidence/quotes or "empty"]

Look right? I'll run a dedup check before creating it.
```

### Engineering Readiness Check

After showing the draft, run the Engineering Readiness Checklist from `skills/analyze-customer-input/references/fr-template-guide.md` against the draft. Flag any questions an engineer could NOT answer from the ticket alone:

```
Readiness gaps (optional — won't block creation):
- [gap]: e.g., "No specific technical environment mentioned — engineer can't assess scope"
- [gap]: e.g., "No workaround details — unclear if customer is fully blocked"

Want to fill any of these in, or create the ticket as-is?
```

If the draft has no gaps, skip this section. These gaps are informational — do not block ticket creation over them. If the SE wants to fill gaps, update the draft and re-show. Only proceed on confirmation.

---

## Phase 7: Dedup Check

Search for existing Jira issues using multiple keyword variations - not just the title. Two tickets can describe the same product ask with completely different wording.

Use `searchJiraIssuesUsingJql` via MCP:

```
JQL: project = SOL AND text ~ "[title keywords]" AND labels = "[customer_label]"
JQL: project = SOL AND text ~ "[alternate keywords]" AND labels = "[customer_label]"
```

For example, for "CSI driver for credential delivery", also search "ephemeral secrets", "pod storage", etc.

- If matches found:

```
Dedup check — possible match found:
  [JIRA-123] [existing title] — [status]
  Same ask? [Yes/No — brief reasoning]

Options: (a) Link to this existing issue, (b) Update it instead, (c) Create a new issue anyway
```

The SE makes the final call. Do not proceed without a decision.

If no matches, say so briefly and proceed.

---

## Phase 8: Jira Write

Generate a Markdown description following the FR template guide convention, then create via MCP:

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

---

## Phase 9: Report Completion

```
Done. Jira ticket created:

  [JIRA-123] [title]
  Label: [customer_label]
  Link: [URL]
```

If the ticket was created without a verbatim customer quote, append:

> "Tip: If you find the exact customer quote later, add it to the Supporting Materials section of [JIRA-123]. Verbatim evidence strengthens engineering prioritization."

If the write fails, report the error and ask whether to retry or handle manually.

---

## Error Handling

| Situation | Action |
|-----------|--------|
| No customer label | Ask before proceeding — required for dedup |
| No title provided | Ask — required field |
| SE says "skip" on a section | Leave those fields empty in the draft |
| SE says this isn't really an FR | Clarify what they're looking for — feedback items are not written to Jira |
| MCP search fails | Report error; ask whether to proceed without dedup or retry |
| Jira write fails | Report error; do not silently skip |
