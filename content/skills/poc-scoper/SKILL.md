---
name: poc-scoper
description: |
  Conversational skill for Aembit Solutions Engineers scoping a customer POC. Interviews the SE about the upcoming engagement and outputs two complete YAML recipe files: a POC Guide (business doc) and an Implementation Guide (technical doc). Also supports adding use cases to an existing POC.

  Trigger: /poc-scoper, "scope a POC", "new customer POC", "prep POC for <customer>", "add use case to existing POC"

  <example>
  Context: SE is preparing for an upcoming POC kickoff
  user: "/poc-scoper"
  assistant: "Let's scope your POC. What's the customer name and industry?"
  <commentary>
  Begins the ScopeSession workflow, collecting context conversationally in phases.
  </commentary>
  </example>

  <example>
  Context: SE wants to add a use case to an existing POC
  user: "add a Snowflake use case to this POC"
  assistant: "I see the existing use cases. Tell me about the new one - what's the source workload and where does it run?"
  <commentary>
  Begins the AddUseCase workflow, reading existing YAML and appending new use cases.
  </commentary>
  </example>
---

# POC Scoper

Conversational scoping skill for Aembit Solutions Engineers. Interviews the SE, maps use cases to the component library, and outputs ready-to-use YAML recipe files.

## Quick Reference

| Output File | Purpose | Location |
|-------------|---------|----------|
| `<customer>_poc_guide.yaml` | Business doc recipe | SE's current working directory / `<slug>/` |
| `<customer>_impl_guide.yaml` | Technical doc recipe | SE's current working directory / `<slug>/` |

**Note:** Recipes are transitory work product — written to the SE's cwd, not stored in any repo or skill.

**Unfilled vars:** Leave as `{{VAR_NAME}}` tokens — the assembler renders them as bold placeholders.

## Workflow

| Trigger | Workflow | Purpose |
|---------|----------|---------|
| New POC scoping | [ScopeSession.md](Workflows/ScopeSession.md) | Interview SE, map components, write YAML files |
| Add use case to existing POC | [AddUseCase.md](Workflows/AddUseCase.md) | Read existing YAML, add use cases, update files |

## References

| Task | Load |
|------|------|
| Component library, policy chain rules, YAML schema | [components.md](references/components.md) |

## Writing Conventions

- **No em dashes (`—`)** - use a hyphen (`-`) or reword the sentence instead
- **Active voice** - "Navigate to Client Workloads" not "Client Workloads should be navigated to"
- **Second person / imperative for steps** - "Navigate to..." not "The user navigates to..."
- **Accuracy over completeness** - never invent details; leave unknown values as `{{VAR_NAME}}` placeholders rather than filling them with plausible-sounding content

## What This Skill Does NOT Do

- Does not run the assembler or generate PDFs - use the `poc-documentation` skill for that
- Does not invent business value language - always ask the SE or leave as `{{VAR_NAME}}`
- Does not fabricate customer contacts or dates - unfilled fields become `{{VAR_NAME}}` tokens
