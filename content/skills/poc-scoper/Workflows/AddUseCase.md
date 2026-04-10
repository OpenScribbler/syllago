# AddUseCase Workflow

> **Trigger:** "add a use case to existing POC", "add use case to SOL-XX"

## Purpose

Add one or more use cases to an existing POC. Reads the current epic and children via MCP, runs the component mapping interview, creates new UC stories via MCP with rich Markdown descriptions, and optionally updates local YAML recipes for PDF assembly.

## Before Starting

Load `references/components.md` - you need the component library and YAML schemas throughout this workflow.

---

## Phase 1: Display Current Use Cases

Read the epic and its children via MCP:

1. `getJiraIssue` with `responseContentFormat: "markdown"` to get the epic description
2. `searchJiraIssuesUsingJql` with `parent = <epic-key> AND labels = "use-case"` to get existing UC stories

Present the current state:

```
Current POC for [CUSTOMER_NAME]:

Existing use cases:
  1. [ISSUE-KEY] [use_case_name] — [status]
  2. ...

Ready to add a new use case.
```

---

## Phase 2: Use Case Discovery

Run the same use case discovery loop as ScopeSession Phase 3:

> "Tell me about the new use case:
> - What is the source application or script? Where does it run?
> - What is the target system or API it needs to reach?
> - How does it authenticate today?"

Apply the **component mapping decision tree** from ScopeSession Phase 3 (same rules for client_identity, server_integration, client_deployment, policy_chain).

Ask for business value and success criteria:

> "Two quick questions about this use case:
> - In the customer's own words - what's the business value?
> - Does the customer have specific success criteria or acceptance tests?"

Ask if there are more use cases to add:

> "Any more use cases to add, or is that all?"

Repeat for each additional use case.

---

## Phase 3: Create UC Stories via MCP

Present a summary before creating:

```
Adding [N] new use case(s) to [CUSTOMER_NAME] POC:

New use cases:
  [N+1]. [use_case_name]: [client_identity] -> [server_integration]

Create these stories under <epic-key>?
```

If confirmed, for each new use case:

1. Generate a rich Markdown description following the UC template guide convention (Overview, Source Workload, Target Resource, Current Auth, Delivery Model, Environment, Business Value, Success Criteria).

2. Create via MCP:

```
createJiraIssue:
  cloudId: aembit.atlassian.net
  projectKey: SOL
  issueType: Story
  summary: <use_case_name>
  description: [Markdown UC description]
  contentFormat: markdown
  parent: <epic-key>
  labels: ["use-case"]
```

3. Update the epic description's Technical Recipe section to include the new use case (read current description via `getJiraIssue`, add the new use case to the YAML code block, write back via `editJiraIssue`).

### Custom Component Warning

If any new use case has `{{CUSTOM}}` or `{{CUSTOM_DEPLOYMENT}}` component paths, warn:

> "Use case '[name]' uses custom component paths that don't have content modules yet. The assembler will fail until those modules are authored."

---

## Phase 4: Update Local YAML (if applicable)

If local YAML recipe files exist for this POC, also update them for PDF assembly:

1. Add new entries to `use_cases` in the impl guide recipe
2. Add any new per-use-case vars (e.g., `USE_CASE_<DESCRIPTOR>_VALUE`)
3. Update `exec_summary_use_cases` in the POC guide to include new use case names

If no local YAML files exist, skip this step - the agent can regenerate them from Jira via `poc-doc generate --from-jira` when needed.

Report completion:

```
Added [N] new use case(s) to <epic-key>:
  [ISSUE-KEY] [use_case_name] (created)

Epic Technical Recipe updated.
```

---

## Error Handling

| Situation | Action |
|-----------|--------|
| Epic not found | Error: "Cannot read epic <KEY>. Verify the key is correct." |
| Duplicate use case name | Warn: "A use case named '[name]' already exists. Use a different name or skip." |
| Custom component path | Warn about assembler failure; write YAML with placeholder |
| SE cancels before writing | Discard changes; YAML files unchanged |
