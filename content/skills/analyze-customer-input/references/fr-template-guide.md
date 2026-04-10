# Feature Request Template Guide

This guide defines what a complete, engineering-ready Feature Request looks like. Use it when:
- **Creating** FRs (via `/log-product-feedback` or `/analyze-customer-input`)
- **Reviewing** existing FRs for completeness and quality
- **Assessing** whether engineering can scope and vet an FR without asking follow-up questions

The questions below drive extraction from customer content and evaluation of existing tickets. A well-written FR should answer most of these questions. Empty fields are acceptable when the information genuinely isn't available — but the agent should flag them as gaps, not silently skip them.

---

## 1. The Problem

### What is the customer unable to do today?
What's happening (or not happening)? Describe the gap, pain point, or limitation the customer is experiencing. Be specific about the current state — not just "they can't do X" but why and what the consequences are.

Preserve **specific operational symptoms** from transcripts and customer materials: exact event types, field names, error conditions, API behaviors, and failure modes the customer described. "Cannot correlate audit events" is too generic — "mcp.response events don't include the user identity, forcing best-effort correlation by contextId" is actionable. Engineers need the specific technical details to scope the work.

### What is the customer asking for?
Capture the request in the customer's own words/framing — even if it's solution-oriented. We'll separate problem from solution during review. Include enough technical detail that an engineer can understand the scope: what system, what protocol, what integration point.

### What is NOT in scope for this request?
If known, note any boundaries or things the customer is NOT asking for. This prevents engineering from over-scoping. Good "not in scope" entries answer: "What might someone assume is included but isn't?"

---

## 2. Business Context & Urgency

Help Product and Engineering understand why this matters now and what's at stake.

### Why does this matter to the customer?
How does this connect to their business goals, workflows, or outcomes? What breaks or degrades if this isn't addressed?

### What's the urgency?
Is there a deadline, contract milestone, renewal date, or event driving the timeline? Be specific: "needed by Q2 2026" is better than "urgent."

### Business impact if not addressed
Mark all that apply and add detail where possible:

- **New deal risk** — details: Could this block the deal from closing?
- **Churn risk** — details: Could this cause the customer to leave?
- **Blocked expansion** — details: Does this prevent growing the account?
- **Degraded customer experience** — details: What's the user-facing impact?
- **Operational inefficiency** — details: What manual work or workarounds result?
- **Other** — details

### Revenue context (if known)
ARR, deal size, expansion opportunity, or strategic account designation.

### Are other customers asking for the same or similar thing?
Select all other customers from the Customer list or mention if this has come up in multiple conversations.

---

## 3. Current Workaround

### Is there an existing workaround?
- **Yes** — describe below
- **No** — customer is fully blocked
- **Partial** — describe below

### Workaround details
What is the customer doing today to get by? How sustainable is it? What are the limitations or risks of the workaround?

### Estimated workaround lifespan
How long can the customer reasonably operate this way? (weeks / months / indefinite)

---

## 4. Supporting Materials

Attach or link anything that adds context:
- Customer email / message thread
- Call recording or transcript
- Screenshots or mockups from customer
- Architecture diagrams

All evidence should include verbatim customer quotes where possible. Paraphrased evidence should be clearly marked as such.

---

## Markdown Convention

When writing an FR description (for `createJiraIssue` or `editJiraIssue` via MCP), generate Markdown following this structure. Pass `contentFormat: "markdown"` on the MCP call.

```markdown
## 1. The Problem

**What is the customer unable to do today?**
{specific gap with operational symptoms}

**What is the customer asking for?**
{request in customer's framing with technical detail}

**What is NOT in scope?**
{explicit exclusions - what might someone assume is included but isn't?}

## 2. Business Context & Urgency

**Why does this matter?**
{connection to business goals}

**Urgency:**
{specific deadline or milestone}

**Business impact if not addressed:**
- [x] New deal risk - {detail}
- [ ] Churn risk
- [x] Blocked expansion - {detail}
- [ ] Degraded experience
- [ ] Operational inefficiency
- [ ] Other

**Revenue context:** {ARR, deal size, etc.}

**Other customers asking:** {names or "none known"}

## 3. Current Workaround

**Status:** {Yes / No / Partial}
**Details:** {what they do today}
**Lifespan:** {weeks / months / indefinite}

## 4. Supporting Materials

> "{verbatim quote}" -- {Speaker} ({Affiliation}), {Source}
```

Use `- [x]` for impact items that apply and `- [ ]` for items that don't. Include detail after the checked items.

---

## Engineering Readiness Checklist

When reviewing an FR for engineering readiness, check whether an engineer could answer these questions from the ticket alone:

| Question | Where it should be answered |
|----------|---------------------------|
| What exactly should we build? | `asking_for` — specific enough to scope |
| What should we NOT build? | `not_in_scope` — prevents over-scoping |
| What is the customer's technical environment? | `asking_for` + evidence — stack, versions, constraints |
| How urgent is this? | `urgency` — specific timeline or milestone |
| What happens if we don't build it? | `impact` — deal/churn/expansion risk |
| What's the customer doing today? | `workaround` — current state and sustainability |
| Is there real customer evidence? | `supporting_materials` — verbatim quotes, not paraphrases |

---

## Enrichment Checklist

When transcript or call evidence is available, check whether the ticket captures the full depth of what was discussed. This goes beyond the Engineering Readiness Checklist — a ticket can pass readiness (engineer could scope it) but still be missing specific details from the transcripts that would improve engineering's understanding.

| Question | Source |
|----------|--------|
| Are ALL specific technical constraints captured? | Transcript evidence |
| Are exact system names, versions, protocols included? | Transcript evidence |
| Are workaround details specific (not just "Partial")? | Transcript evidence |
| Are verbatim customer quotes included for key points? | Transcript evidence |
| Are integration constraints and incompatibilities documented? | Transcript evidence |
| Is `not_in_scope` populated with explicit exclusions from the discussion? | Transcript evidence |
| Does the ticket reflect the most recent discussion of this feature? | Latest call |
