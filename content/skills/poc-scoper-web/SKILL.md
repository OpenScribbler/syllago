# POC Scoper (Web)

Conversational scoping skill for Aembit Solutions Engineers. Interviews the SE, maps use cases to the component library, and outputs ready-to-use YAML recipe files saved to Google Drive.

## Quick Reference

| Output File | Purpose | Location |
|-------------|---------|----------|
| `<customer_slug>_poc_guide.yaml` | Business doc recipe | Google Drive: `Customer Resources/<customer>/poc/` |
| `<customer_slug>_impl_guide.yaml` | Technical doc recipe | Google Drive: `Customer Resources/<customer>/poc/` |

**Unfilled vars:** Leave as `{{VAR_NAME}}` tokens - the assembler renders them as bold placeholders.

## Session Start

At the beginning of every session, fetch two things from GitHub MCP before starting any workflow:

1. **Component library** - fetch `skills/poc-scoper/references/components.md` from the `ai-tools` repo via GitHub MCP (`get_file_contents`). This is the single source of truth for component paths, policy chain rules, and YAML schemas. Do not use a cached or local copy.

2. **Content module manifest** - call `list_directory` on `skills/poc-documentation/content/` in the `ai-tools` repo via GitHub MCP. Cache this directory listing as the available module manifest. Use it to validate component paths before writing recipes.

Do not proceed with the interview until both fetches succeed. If either fails, report the error and stop.

## Workflow

| Trigger | Workflow | Purpose |
|---------|----------|---------|
| New POC scoping | [ScopeSession.md](Workflows/ScopeSession.md) | Interview SE, map components, write YAML to Drive |
| Add use case to existing POC | [AddUseCase.md](Workflows/AddUseCase.md) | Fetch recipe from Drive, add use cases, save back to Drive |

## Writing Conventions

- **No em dashes** - use a hyphen (`-`) or reword the sentence instead
- **Active voice** - "Navigate to Client Workloads" not "Client Workloads should be navigated to"
- **Second person / imperative for steps** - "Navigate to..." not "The user navigates to..."
- **Accuracy over completeness** - never invent details; leave unknown values as `{{VAR_NAME}}` placeholders rather than filling them with plausible-sounding content

## What This Skill Does NOT Do

- Does not generate PDFs - use the `poc-documentation-web` skill for that
- Does not create Jira tickets - use `analyze-customer-input` for extracting use cases and FRs to Jira
- Does not invent business value language - always ask the SE or leave as `{{VAR_NAME}}`
- Does not fabricate customer contacts or dates - unfilled fields become `{{VAR_NAME}}` tokens
- Does not read from or write to the local filesystem - all storage is Google Drive
