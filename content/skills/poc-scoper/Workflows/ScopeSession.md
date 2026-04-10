# ScopeSession Workflow

> **Trigger:** `/poc-scoper`, "scope a POC", "new customer POC", "prep POC for <customer>"

## Purpose

Conversationally interview a Solutions Engineer about an upcoming customer POC, map workloads to the component library, and write two YAML recipe files to the customer directory.

## Before Starting

Load `references/components.md` — you need the component library and YAML schemas throughout this workflow.

---

## Phase 1: Customer Context

Ask as a single opening message (not one-at-a-time — these are naturally grouped):

> "Let's scope your POC. To start:
> - What's the customer name and industry?
> - Roughly how large is the company?
> - Is this a direct evaluation or going through a partner or reseller?
> - Do you have a customer logo file to include on the cover page? (optional - provide a file path, or skip)"

Capture: `customer_name`, `customer_slug` (derived), `industry`, `company_size`, `direct_or_partner`, `customer_logo` (optional, file path or empty).

---

## Phase 2: Team and Dates

Ask as a single message:

> "Who's on the Aembit side?
> - Your name and email (SA)?
> - AE name and email?
> - When is the POC kickoff date?"

Capture: `sa_name`, `sa_email`, `ae_name`, `ae_email`, `poc_start_date`.

---

## Phase 3: Use Case Discovery

This phase is iterative. Use it to discover each use case, one at a time.

### Opening Question

> "Now let's map out the use cases. Tell me about the first workload Aembit would protect:
> - What is the source application or script? Where does it run (EC2, Lambda, K8s, Claude, etc.)?
> - What is the target system or API it needs to reach?
> - How does it authenticate today — stored credentials, secrets manager, hardcoded tokens?"

### For Each Use Case

After getting the answer, apply the **component mapping decision tree**:

**client_identity selection:**
```
Source workload is...
  Claude Desktop or Web Enterprise + Okta? → claude_okta_oidc
  EC2 instance with IAM role?             → ec2_iam_role
  AWS Lambda function?                    → lambda_iam_role
  AWS ECS task (Fargate or EC2)?         → ecs_task_role
  Kubernetes pod (any cluster type)?      → k8s_oidc_service_account
  GitHub Actions workflow?                → github_actions_oidc
  GitLab CI/CD pipeline (Cloud only)?    → gitlab_cicd_oidc
  Other?                                  → ask SE for closest match or leave note
```

**server_integration selection:**
```
Target system is...
  Any MCP Server (AI agent access)?       → mcp_server
  Box API (programmatic/script access)?   → box_api_oauth
  Snowflake?                              → snowflake_jwt
  PostgreSQL?                             → postgresql_password
  Any AWS service (S3, SQS, DynamoDB...)?→ aws_sts_federation
  Salesforce API?                         → salesforce_oauth_3lo
  Azure Entra ID (via JWT-SVID WIF)?     → entra_id_jwt_svid
  HashiCorp Vault?                        → hashicorp_vault
  Aembit API (automation/CI-CD)?          → aembit_api
  Internal API validating JWT-SVIDs?      → jwt_svid_bearer
  Other?                                  → note as {{CUSTOM}} and flag for SE
```

**client_deployment selection:**
```
client_identity is...
  ec2_iam_role?            → ec2_proxy
  lambda_iam_role?         → lambda_extension
  ecs_task_role?           → ecs_sidecar
  claude_okta_oidc?        → mcp_client
  github_actions_oidc?     → github_actions
  gitlab_cicd_oidc?        → gitlab_cicd
  k8s_oidc_service_account → ask: which cluster type?
    Standard K8s (AKS/GKE/EKS non-Fargate/on-prem)?  → k8s_helm
    OpenShift?                                         → openshift_helm
    EKS Fargate?                                       → eks_fargate_helm
    K8s with Istio or conflicting service mesh?        → agent_cli_sidecar
  Other (custom)?          → set as {{CUSTOM_DEPLOYMENT}} and flag for SE
                              (no standard content module — assembler will error until one is authored)
```

**policy_chain selection:**
```
client_identity = claude_okta_oidc AND server_integration = mcp_server?
  → dual, labels: "Policy 1: Claude → MCP Gateway", "Policy 2: {{MCP_USER_EMAIL}} → {{MCP_SERVER_NAME}}"
  Otherwise → single (no labels)
```

### Ask for Business Value and Success Criteria

After mapping the components, ask as a single grouped message:

> "Two quick questions about this use case:
> - In the customer's own words — what's the business value? What problem does it solve for them?
> - Does the customer have specific success criteria or acceptance tests for this use case — particular behaviors or outcomes they need to see pass?"

**Business value:** If the SE can't articulate it, set `business_value: "{{USE_CASE_<DESCRIPTOR>_VALUE}}"` and populate `business_value_hint` using the "Business Value Hint" column from the component library table for the matched `client_identity` and `server_integration` components.

**Success criteria:** If the customer has formal criteria, transcribe them verbatim — use this as the primary source for `SUCCESS_CRITERIA_TABLE`. If not, note it and do not synthesize generic Aembit capability language; the table will be derived from other sources in Phase 4 or left as a placeholder.

### Ask for More Use Cases

> "Is there another use case to include, or is that all for this POC?"

Repeat Phase 3 for each use case. Most POCs have 1-3 use cases.

---

## Phase 4: Business Goals, Contacts, and Timeline

Ask as a single grouped message:

> "A few more details for the business doc:
> - What are the top 1-3 business goals the customer wants to achieve with this POC? (Use their words if you have them.)
> - Who are the customer contacts for the POC? For each, I need name, role, and email. (At least one required.)
> - Do you have a target POC closeout date?"

Capture: `business_goals[]`, `contacts[]` (each with name/role/email), `timeline_closeout_date`.

Leave any unknown fields as `{{VAR_NAME}}` tokens.

### After capturing business goals — populate derived vars

Use the business goals and use cases to write two additional vars for the POC guide:

- **`EXEC_SUMMARY_INTRO`** — 2-3 sentences describing what the customer is evaluating and why. Draw from the business goals and the customer's industry/context. Do not fabricate; if you don't have enough, leave as `{{EXEC_SUMMARY_INTRO}}`.
- **`EXEC_SUMMARY_USE_CASES`** — a markdown bullet list, one line per use case name (e.g. `- Use Case Name`). Derived directly from the use cases discovered in Phase 3.
- **`SUCCESS_CRITERIA_TABLE`** — a markdown table with columns `No | Test Case | Success Criterion | Mandatory`. Derived from the customer's actual requirements (requirements doc, transcript, or stated success criteria). Do not use generic Aembit capability language.

---

## Phase 5: Confirm and Write Files

### Validate Before Summarizing

Before presenting the summary, check all required fields. For any field that would write a `{{VAR_NAME}}` token into the YAML (i.e., the value is unknown), ask the SE for it now rather than leaving it as a placeholder.

**Required fields that must not be placeholders in the final YAML:**

| Field | Required for |
|-------|-------------|
| `contacts` (at least one entry with name/role/email) | POC Guide |
| `BUSINESS_GOAL_1`, `BUSINESS_GOAL_2` (at minimum) | POC Guide |
| `EXEC_SUMMARY_INTRO` | POC Guide |
| `SA_NAME`, `SA_EMAIL`, `AE_NAME`, `AE_EMAIL` | Both |
| `POC_START_DATE` | Both |
| Use case `business_value` (all use cases) | Impl Guide |

If any required field is missing, ask for it directly before proceeding:

> "Before I write the files, I'm missing a few things:
> - [list each unknown required field with a brief description]
> Can you fill these in, or should I leave them as placeholders and you'll edit the YAML manually?"

If the SE explicitly says to leave as placeholders, proceed — but call out each placeholder field in the completion report.

### Summarize Before Writing

Present a brief summary:

```
Here's what I have:

Customer: <customer_name> (<industry>, <size>)
Customer logo: <path or "none">
SA: <sa_name> | AE: <ae_name>
Kickoff: <poc_start_date>

Use cases:
  1. <use_case_name>: <client_identity> → <server_integration> (<policy_chain>)
  2. ...

Business goals: <count> captured
Customer contact: <name> (<role>)

Output directory: <cwd>/<customer_slug>/

Ready to write the YAML files?
```

If the SE confirms (or says yes/proceed/looks good), continue. If they want to correct anything, update and re-summarize.

### Write Output Files

**Output directory:** `<current working directory>/<customer_slug>/`

Create the directory if it does not exist. Customer recipe files are transitory work product — they go in the SE's current working directory, not in any skill or repo.

**Custom component warning:** Before writing, check whether any use case has `{{CUSTOM}}` or `{{CUSTOM_DEPLOYMENT}}` component paths. If so, tell the SE:

> "⚠ Use case '<name>' uses custom component paths that don't have content modules yet (`<path>`). The poc-documentation assembler will fail with a FileNotFoundError when generating the Implementation Guide until those modules are authored. The YAML will still be written — flag this as a follow-up item."

Write two files using the schemas from `references/components.md`:

1. `<customer_slug>_poc_guide.yaml` — populate all known vars, use `{{VAR_NAME}}` for unknowns
2. `<customer_slug>_impl_guide.yaml` — one `use_cases` entry per discovered use case

**Naming convention for per-use-case value vars:** Use short descriptive names, not positional numbers. Examples:
- `USE_CASE_MCP_VALUE` for a Claude → Box MCP use case
- `USE_CASE_EC2_BOX_VALUE` for an EC2 → Box API use case
- `USE_CASE_LAMBDA_SF_VALUE` for a Lambda → Snowflake use case

**Verification, success_criteria, and troubleshooting:** Populate these based on the matched components. Each item should be specific and testable - name the exact UI location, command, or observable output. Avoid generic statements like "confirm the policy works"; instead write "Navigate to Activity → confirm a log entry shows the workload authenticated successfully."

**Long commands in verification steps:** When a verification step includes a command (curl, wget, etc.) longer than ~80 characters, use YAML block scalar (`|`) syntax with the command in a fenced code block and `\` line continuations. Example:
```yaml
verification:
  - |
    From the VM, run:

    ```bash
    curl --location --request POST \
      'https://example.com/token' \
      --header 'Content-Type: application/x-www-form-urlencoded' \
      --data-urlencode 'client_id={{CLIENT_ID}}' | jq
    ```
```

---

## Phase 6: Report Completion

After writing the files:

```
POC scoping complete. Two recipe files written:

  <cwd>/<customer_slug>/<customer_slug>_poc_guide.yaml
  <cwd>/<customer_slug>/<customer_slug>_impl_guide.yaml

Unfilled placeholders ({{VAR_NAME}} tokens):
  <list any vars left as placeholders, or "none">

To generate the PDFs, run:
  /poc-documentation <customer_slug>/

Or generate individually:
  /poc-documentation <customer_slug>/<customer_slug>_poc_guide.yaml
  /poc-documentation <customer_slug>/<customer_slug>_impl_guide.yaml
```

### Optional: Create Jira Ticket

After reporting completion, offer to create a Jira ticket:

> "Would you like me to create a Jira ticket for this POC? This will:
> - Create a POC Epic with all the business and technical data
> - Create a Story for each use case (with rich Markdown descriptions)
> - Attach the customer logo if provided
>
> Create Jira ticket? (yes/no)"

If yes, create the epic and stories via MCP:

1. Generate the epic description in Markdown following the convention (Customer, Executive Summary, Contacts, Business Goals, Success Criteria, Technical Recipe). Include the Technical Recipe section with the YAML recipe content in a fenced code block (required for `poc-doc generate --from-jira`).

```
createJiraIssue:
  cloudId: aembit.atlassian.net
  projectKey: SOL
  issueType: Epic
  summary: "POC: <customer_name>"
  description: [Markdown epic description]
  contentFormat: markdown
```

2. For each use case, generate a rich Markdown description following the UC template guide convention (Overview, Source Workload, Target Resource, Current Auth, Delivery Model, Environment, Business Value, Success Criteria). Create via MCP:

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

3. Attach the customer logo if provided: `~/.claude/bin/poc-doc attach <epic-key> <logo_path>`

Report the result:

```
Jira ticket created: <PROJ-XX>
  Stories: <count> use case(s)

To generate PDFs from Jira later:
  poc-doc generate --from-jira <PROJ-XX>
```

---

## Error Handling

| Situation | Action |
|-----------|--------|
| SE describes a workload with no matching component | Map to closest match, note it explicitly, flag as needing review |
| Workload maps to `{{CUSTOM}}` or `{{CUSTOM_DEPLOYMENT}}` | Write the YAML with placeholder paths; warn the SE that the assembler will FileNotFoundError until content modules are authored |
| Customer name contains special characters | Strip to alphanumeric + hyphens for slug |
| Output directory already has YAML files | Ask: "Files already exist for <customer>. Overwrite?" |
| SE unsure of business value | Set `{{USE_CASE_<DESCRIPTOR>_VALUE}}`, populate `business_value_hint` from the component library hint column |
| SE provides partial contact info | Fill known fields, use `{{VAR_NAME}}` for missing fields |
