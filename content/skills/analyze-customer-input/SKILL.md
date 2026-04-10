---
name: analyze-customer-input
description: |
  Analyzes customer-provided content — Gong call transcripts, requirements docs, architecture diagrams, or other materials — and classifies extracted items into Use Cases, Feature Requests, and Observations. All classifications are grounded in verbatim evidence. Presents a structured review to the analyst with ambiguity resolution before any Jira writes occur.

  Trigger: /analyze-customer-input, "analyze this transcript", "extract from", "what did the customer ask for", "analyze calls for"

  <example>
  Context: Analyst has a Gong transcript to analyze
  user: "/analyze-customer-input"
  assistant: "Let's analyze this customer content. What's the Jira label for this customer? And please paste or attach the content you'd like me to analyze."
  <commentary>
  Begins the AnalyzeSession workflow, asking for the customer label before touching any content.
  </commentary>
  </example>

  <example>
  Context: POC Manager agent delegates transcript analysis
  user: "Analyze all Gong calls with customer X and extract use cases and FRs"
  assistant: "I'll search for calls, fetch transcripts, classify everything, and present a consolidated review."
  <commentary>
  When invoked by poc-manager, produces classified extraction for the agent to act on.
  </commentary>
  </example>
---

# Analyze Customer Input

Extraction and classification skill for analysts and SEs. Reads customer content, classifies items into **Use Cases**, **Feature Requests**, and **Observations**, grounds every classification in verbatim evidence, resolves ambiguities with the analyst, and writes approved items to Jira.

## Classification Framework

Every extracted item must be assigned exactly one category. Use the decision tree below.

### Decision Tree

```
For each customer statement backed by evidence:

1. Can I name a SOURCE WORKLOAD and a TARGET RESOURCE the customer wants to connect?
   → YES → Use Case

2. Is the customer asking for a PRODUCT CAPABILITY that does not exist today?
   → YES → Feature Request

3. Neither of the above?
   → Observation
```

### Definitions

| Category | Definition | Jira artifact |
|----------|-----------|---------------|
| **Use Case** | A workload-to-workload access pattern the customer wants to prove. Has a source workload, a target resource, and a "from → to" flow. | Story under epic |
| **Feature Request** | A product capability that does not exist today that the customer needs built. | Feature Request issue under epic |
| **Observation** | Context about the customer's environment, timeline, procurement process, internal dynamics, references to Aembit competitors, or preferences. Useful for understanding but not actionable as a UC or FR. | Only written to Jira in descriptions as context for UC or FR tickets when relevant — informational only |

### Boundary Rules

These rules resolve the most common gray areas:

| Situation | Classification | Rationale |
|-----------|---------------|-----------|
| Describes HOW a use case works, and the capability already exists | Part of the Use Case (environment detail) | Not a missing capability |
| Describes a missing capability needed by one or more use cases | Feature Request | The gap is in the product, not the access pattern |
| Part of a use case's environment BUT the integration/capability doesn't exist today | **Both**: include as UC environment detail AND create a separate FR | The UC needs it to work; the product needs to build it |
| Customer mentions a specific integration they need (e.g., "we need Okta support") | Depends: if it's the IDP for a use case → UC detail; if it's a new integration type Aembit doesn't support → FR (or both, per rule above) | Ask if ambiguous |
| Procurement timelines, internal politics, budget cycles | Observation | Not actionable as UC or FR |
| Customer's environment details not tied to a specific access pattern | Observation | Context, not a use case |
| Customer describes a workaround they use today | Part of the relevant UC or FR (workaround field) | Evidence of the problem, not a standalone item |
| Item was raised AND resolved during the engagement | Flag as resolved — do not create a ticket | Already addressed |

### When Ambiguous — ASK

If an item could reasonably fall into more than one category, **do not guess**. Present the item with the competing classifications and your reasoning, and ask the analyst to decide. Batch all ambiguous items into a single question after the initial classification pass.

**Agent mode:** When invoked by another agent that will handle Jira writes separately, output the classified extraction as structured data and let the invoking agent control the write phase. If the invoking agent's instructions say to run the full workflow including Jira writes, follow those instructions instead.

**Multi-source analysis is supported.** Multiple calls, documents, or mixed source types can be analyzed in a single run. Items are consolidated across sources with evidence from all relevant sources. See the workflow for dedup and consolidation rules.

## Workflow

| Trigger | Workflow | Purpose |
|---------|----------|---------|
| Any analysis request | [AnalyzeSession.md](Workflows/AnalyzeSession.md) | Extract, classify, review, dedup, write to Jira |

## Use Case Schema

Use cases extracted from customer content. Captures enough detail for internal understanding of the customer's environment and intent.

For the Markdown convention, field-level guidance, and the Completeness Checklist, load `references/uc-template-guide.md`.

## FR Description Convention

Every extracted Feature Request is created via MCP `createJiraIssue` with a Markdown description. FRs without a verbatim evidence quote are not valid and must be discarded.

For the Markdown convention, field-level guidance, and the Engineering Readiness Checklist, load `references/fr-template-guide.md`.

## POC-Blocking Gate

Any item (UC or FR) with `poc_blocking: true` requires explicit analyst acknowledgment before the full batch can be approved. These items cannot be bulk-approved alongside other items — the analyst must explicitly confirm each one.

## What This Skill Does NOT Do

- Does not write to Jira without explicit analyst approval
- Does not classify ambiguous items without asking — when in doubt, ask
- Does not create items without a traceable verbatim quote
- Does not assume a customer Jira label — always asks the analyst to confirm it
- Does not skip the dedup check before writing to Jira
- Does not do Aembit component mapping — that is poc-scoper's job
- Does not create Jira tickets for Observations — they are informational only
