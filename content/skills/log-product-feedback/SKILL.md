---
name: log-product-feedback
description: |
  Lightweight skill for SEs and analysts to quickly log a single feature request or product observation to Jira from any source — a quote heard on a call, a note from a meeting, something from a customer email. Collects the minimum required information, runs a dedup check, shows a draft ticket, and writes to Jira on approval.

  Trigger: /log-product-feedback, "log this FR", "create a Jira ticket for this customer feedback", "the customer mentioned..."

  <example>
  Context: SE just got off a call and wants to log something quickly
  user: "/log-product-feedback"
  assistant: "Sure — a few quick questions: What's the customer name or Jira label? What did they ask for (a short title works)? And do you have a quote from them I can attach as evidence?"
  <commentary>
  Begins the LogFeedback workflow, collecting only what's needed for a valid FR ticket.
  </commentary>
  </example>

  <example>
  Context: SE provides the quote inline with the trigger
  user: "the customer mentioned they need SSO support before they can go to production"
  assistant: "Got it. What's the customer Jira label, and should I treat this as POC-blocking?"
  <commentary>
  When partial information is provided, ask only for what's missing — don't re-ask for what was already given.
  </commentary>
  </example>
---

# Log Product Feedback

Conversational skill for SEs and analysts to log feature requests to Jira. Walks through the FR template, creates a `Feature Request` issue type with `feature-request` label via MCP `createJiraIssue`.

Runs a dedup check and shows a draft for approval before writing.

**Agent mode:** When invoked by another agent that will handle Jira writes separately, output the draft FR as structured data and let the invoking agent control the write phase. If the invoking agent's instructions say to run the full workflow including Jira writes, follow those instructions instead.

**One ticket per run.** For bulk extraction from a full transcript or document, use `/analyze-customer-input` instead.

## When to Use This vs. `/analyze-customer-input`

| Situation | Use |
|-----------|-----|
| Single feature request from memory / email / meeting notes | `/log-product-feedback` |
| Full Gong transcript or customer document to analyze | `/analyze-customer-input` |
| Multiple FRs to extract in one pass | `/analyze-customer-input` |

## Workflow

| Trigger | Workflow | Purpose |
|---------|----------|---------|
| Any logging request | [LogFeedback.md](Workflows/LogFeedback.md) | Collect FR, dedup, write to Jira |

## FR Description Convention

The skill collects information conversationally and generates a Markdown description following the FR template convention. All FR tickets use the same template regardless of how they were created.

For the Markdown convention, field-level guidance, and the Engineering Readiness Checklist, load `skills/analyze-customer-input/references/fr-template-guide.md`.

## What This Skill Does NOT Do

- Does not write to Jira without showing a draft and getting SE approval
- Does not skip the dedup check
- Does not create a ticket without at least a title and customer label
- Does not handle bulk extraction — use `/analyze-customer-input` for that
