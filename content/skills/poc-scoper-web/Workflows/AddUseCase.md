# AddUseCase Workflow

> **Trigger:** "add a use case to existing POC", "add use case to <customer>'s POC"

## Purpose

Add one or more use cases to an existing POC. Fetches the current recipe YAML from Google Drive, runs the component mapping interview, updates both recipes, and saves them back to Drive.

## Before Starting

1. Fetch `skills/poc-scoper/references/components.md` from the `ai-tools` repo via GitHub MCP (`get_file_contents`). You need the component library and YAML schemas throughout this workflow.

2. Fetch the content module manifest by calling `list_directory` on `skills/poc-documentation/content/` in the `ai-tools` repo via GitHub MCP. Cache this listing for module validation.

Do not proceed until both fetches succeed.

---

## Phase 1: Display Current Use Cases

Fetch the existing recipe YAML from Google Drive:

1. Search Drive for `<customer_slug>_impl_guide.yaml` in `Customer Resources/<customer>/poc/`
2. Read the file contents and parse the YAML
3. Also fetch `<customer_slug>_poc_guide.yaml` for updating later

Present the current state:

```
Current POC for [CUSTOMER_NAME]:

Existing use cases:
  1. [use_case_name]: [client_identity] -> [server_integration]
  2. ...

Ready to add a new use case.
```

If the recipe is not found in Drive, report:

> "I couldn't find recipe files for <customer> in Drive (`Customer Resources/<customer>/poc/`). Run the poc-scoper-web skill first to create the initial recipes, or provide the YAML directly."

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

## Phase 3: Update Recipes and Save

### Module Validation

Before updating, verify each new component path exists in the cached content module manifest. Warn for any missing paths (same as ScopeSession Phase 5).

### Custom Component Warning

If any new use case has `{{CUSTOM}}` or `{{CUSTOM_DEPLOYMENT}}` component paths, warn:

> "Use case '<name>' uses custom component paths that don't have content modules yet. The poc-documentation-web skill will fail when generating the Implementation Guide until those modules are authored."

### Summarize Before Saving

Present a summary before writing:

```
Adding [N] new use case(s) to [CUSTOMER_NAME] POC:

New use cases:
  [N+1]. [use_case_name]: [client_identity] -> [server_integration]

Update the recipes in Drive?
```

### Save Updated Recipes

If confirmed:

1. Add new use case entries to the `use_cases` list in the impl guide recipe. Include all fields: `name`, `overview`, `business_value`, `client_identity`, `server_integration`, `client_deployment`, `policy_chain`, `verification`, `success_criteria`, `troubleshooting`. Add `business_value_hint` only when `business_value` is a placeholder.

2. Add any new per-use-case vars (e.g., `USE_CASE_<DESCRIPTOR>_VALUE`) to the impl guide recipe's `vars` section.

3. Add the new use case names to `exec_summary_use_cases` in the POC guide recipe.

4. Save both updated YAML files back to Drive at their original locations:
   - `Customer Resources/<customer>/poc/<customer_slug>_impl_guide.yaml`
   - `Customer Resources/<customer>/poc/<customer_slug>_poc_guide.yaml`

### Report Completion

```
Updated [CUSTOMER_NAME] POC recipes in Drive:

Added [N] new use case(s):
  [N+1]. [use_case_name]: [client_identity] -> [server_integration]

Updated files:
  Customer Resources/<customer>/poc/<customer_slug>_impl_guide.yaml
  Customer Resources/<customer>/poc/<customer_slug>_poc_guide.yaml

To regenerate the PDFs, use the poc-documentation-web skill:
  "Generate docs for <customer>"
```

---

## Error Handling

| Situation | Action |
|-----------|--------|
| Recipe not found in Drive | Report error, suggest running poc-scoper-web first |
| Duplicate use case name | Warn: "A use case named '[name]' already exists. Use a different name or skip." |
| Custom component path | Warn about PDF generation failure; write YAML with placeholder |
| SE cancels before saving | Discard changes; Drive files unchanged |
| GitHub MCP fetch fails | Report error, do not proceed until component library is loaded |
| Content module path not in manifest | Warn SE that PDF generation will fail for that component; update YAML anyway |
