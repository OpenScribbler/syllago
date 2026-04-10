---
name: poc-module-writer
description: |
  Specialized skill for writing, reviewing, and editing Aembit POC component modules. Understands the assembler pipeline, module type rules, and the balance principle for POC documentation.

  Trigger: /poc-module-writer, "write a module for", "review this module", "edit this module"

  <example>
  Context: SE needs a new integration module
  user: "/poc-module-writer write a new module for GitHub Actions OIDC as a client identity"
  assistant: "Let me ask a few questions before writing..."
  <commentary>
  Gathers required information, checks aembit-knowledge, stops on unknowns before writing.
  </commentary>
  </example>

  <example>
  Context: SE wants an existing module reviewed
  user: "/poc-module-writer review content/client_deployment/ec2_proxy.md"
  assistant: "Reviewing ec2_proxy.md against assembly rules and balance principles..."
  <commentary>
  Reads the file, evaluates against all rules, returns grouped findings.
  </commentary>
  </example>
---

# POC Module Writer

Specialized skill for writing, reviewing, and editing Aembit POC component modules. All work is grounded in how the assembler pipeline renders modules, the design philosophy for POC documentation, and the target audience.

## Modes

On invocation, determine the mode from context:

| Mode | Trigger | Output |
|------|---------|--------|
| **Write** | "write a module for X" | New `.md` file written to disk at the correct path |
| **Review** | "review [file or module]" | Findings grouped by issue type, no changes made |
| **Edit** | "edit [file] to [change]" | Updated `.md` file written to disk |

For **Write** mode: ask all clarifying questions before writing. Do not guess at unknown values, UI paths, or integration-specific details — stop and ask. Write directly to `~/.claude/skills/poc-documentation/content/<type>/<name>.md` and update the component registry (see Registry Update below).

For **Review** mode: read the file, evaluate against all rules below, return findings. Do not make changes.

For **Edit** mode: read the file first, make only the requested change, write back to disk.

---

## Assembly Model — What Gets Rendered

The assembler (`~/.claude/skills/poc-documentation/assembler.py`) extracts specific sections from each module by exact `## ` heading match. Understanding which sections are rendered is critical to writing correct modules.

### Sections extracted by module type

| Module Type | Sections extracted | Sections ignored |
|-------------|-------------------|-----------------|
| `client_identity` | `## Prerequisites` (aggregated), `## Values Reference` (aggregated), `## Aembit Configuration` (rendered as Part 2) | Everything else, including `## Verification` and `## Troubleshooting` |
| `server_integration` | `## Prerequisites` (aggregated), `## Values Reference` (aggregated), `## Service Configuration` (rendered as Part 1), `## Aembit Configuration` (rendered as Part 2) | `## Verification` and `## Troubleshooting` are never rendered in the PDF |
| `client_deployment` | `## Prerequisites` (aggregated), `## Values Reference` (aggregated), `## Deployment` (rendered as Part 3) | `## Verification` and `## Troubleshooting` are never rendered in the PDF |
| `infrastructure` | Entire file rendered as-is, including V+T | Nothing ignored |
| `access_conditions` | Entire file rendered as-is (included via `infrastructure:` key in recipe) | Nothing ignored |

### What the assembler auto-generates

The assembler auto-generates the final Access Policy step at the end of Part 2 via `make_final_policy_step()`. **Never include a "Navigate to Access Policies" step in `client_identity` or `server_integration` modules.** It will produce a duplicate step in the PDF.

The assembler also auto-generates Part 1, Part 2, and Part 3 headings. Module files do not need to include these headings.

### Verification and Troubleshooting in server_integration and client_deployment modules

Even though V+T sections are not rendered in the assembled PDF, **they must still be written** in `server_integration` and `client_deployment` modules. The SE uses these sections as reference when populating the YAML recipe's `verification:` and `troubleshooting:` fields. Write them as well-crafted, SE-usable reference content.

### Section heading rules

- Headings must match exactly: `## Prerequisites`, `## Values Reference`, `## Service Configuration`, `## Aembit Configuration`, `## Deployment`, `## Verification`, `## Troubleshooting`
- Case-sensitive. A heading like `## Aembit Setup` will not be extracted.
- Subsections (`###`) are fine within a section and will be included in the extract.
- A standalone `##` heading with any other name (e.g., `## How Authentication Works`) will be silently ignored by the assembler.

---

## Module Type Rules

### client_identity modules
- **Required sections:** `## Prerequisites`, `## Values Reference`, `## Aembit Configuration`
- **Omit:** `## Service Configuration`, `## Deployment`, `## Verification`, `## Troubleshooting` — these are never extracted and will not appear in the PDF
- **No Access Policy step** — the assembler generates it
- Focus: configuring the Client Workload and Trust Provider in Aembit only. This is intentionally minimal.

### server_integration modules
- **Required sections:** `## Prerequisites`, `## Values Reference`, `## Service Configuration`, `## Aembit Configuration`, `## Verification`, `## Troubleshooting`
- **No Access Policy step** in `## Aembit Configuration` — the assembler generates it
- `## Service Configuration` = setting up the target service (Box, Snowflake, Salesforce, etc.)
- `## Aembit Configuration` = setting up the Credential Provider and Server Workload in Aembit

### client_deployment modules
- **Required sections:** `## Prerequisites`, `## Values Reference`, `## Deployment`, `## Verification`, `## Troubleshooting`
- `## Deployment` = step-by-step install/configure instructions
- Verification and Troubleshooting are SE reference material

### infrastructure modules
- **Required sections:** `## Prerequisites`, `## Values Reference` (if applicable), `## Aembit Configuration` (or equivalent setup section), `## Verification`, `## Troubleshooting`
- Entire file renders — every section appears in the PDF
- Frame as one-time setup, not per-use-case steps

### access_conditions modules
- **Required sections:** `## Prerequisites`, `## Values Reference` (if applicable), `## Service Configuration` (external service setup), `## Aembit Configuration` (Access Condition + attach to policy), `## Verification`, `## Troubleshooting`
- Entire file renders — every section appears in the PDF

---

## The Balance Principle

Every sentence in a module should pass this test: **would a mid-level engineer get stuck without it?**

If yes → include it.
If no → cut it.

### Happy path only

- Document one path: the most common, straightforward scenario
- Edge cases and advanced configurations are handled verbally by the SE
- Do not add "if X then Y, if Z then W" branches — pick one and document it
- Do not add caveats like "for production, consider..." — this is a POC

### Right level of detail for the audience

The audience is engineers (junior to senior) following the doc independently, with the SE available but not always present. Calibrate to the junior end for step navigation, but do not over-explain concepts that any engineer will recognize.

- **Include:** UI navigation paths (exact menu names), specific field names, required vs. optional fields, command syntax with correct flags, where to find values the engineer needs
- **Omit:** Architecture explanations before field-entry steps, rationale for design decisions, comparison tables where one option is clearly the right one, product marketing language

### What "too dense" looks like

- A four-sentence explanation before a list of fields to fill in
- A parenthetical that starts with "Note: for production..." or "If your environment requires..."
- Repeating the same guidance in both a numbered step and a troubleshooting entry
- Scope options with three alternatives and a governance recommendation

### What "too sparse" looks like

- A step that says "run the command" without showing the command
- A field entry that says "enter the appropriate value" without saying where to find it
- A verification section that says "confirm it works" without saying what success looks like
- A step that navigates through a multi-level UI without naming each level

---

## Writing Conventions

### Placeholders
- Customer-specific values: `{{PLACEHOLDER_NAME}}` in ALL_CAPS_SNAKE_CASE
- Every `{{PLACEHOLDER}}` used in the body must appear in `## Values Reference` with a "Where to Find It" path
- Dynamic expressions (Aembit runtime values): `${oidc.identityToken.decode.payload.<claim>}`
- Leave unknown values as `{{PLACEHOLDER}}` — never fill in plausible-sounding values

### Values Reference table format
```markdown
## Values Reference

| Value | Where to Find It |
|-------|-----------------|
| `{{PLACEHOLDER_NAME}}` | Exact navigation path to find this value |
```

### Formatting
- `**bold**` for UI element names, button labels, menu items, field names
- `` `code` `` for commands, values, file paths, env var names
- Code blocks for multi-line commands or config
- Long commands (curl, wget, install commands with multiple flags) must be formatted as multi-line with `\` continuations inside a fenced code block. Never write a command longer than ~80 characters on a single line - it renders poorly in the PDF.
- `> **Note:**` or `> **Start Here:**` blockquote for important callouts
- Numbered lists for sequential steps; bullet lists for non-sequential items
- No em dashes (`—`) — use a hyphen or reword

### Voice and style
- Active voice, imperative: "Navigate to **Client Workloads**" not "Client Workloads should be navigated to"
- Second person: "Click **Save**" not "The user clicks Save"
- Step verbs: "Navigate to", "Click", "Enter", "Copy", "Paste", "Enable", "Run"

### Troubleshooting format
```
- **<symptom as the engineer would experience it>:** <diagnosis and fix in one sentence>
```

### "Start Here" pattern (for two-pass flows)
When completing step A requires a value generated by step B (which comes later), use a blockquote at the top of the section:
```markdown
> **Start Here:** Before completing [Service] steps, open [Aembit dialog] first to retrieve [value]. Then return here to complete setup.
```

---

## Anti-Patterns — Never Do These

1. **Redundant Access Policy step** in `server_integration` or `client_identity` modules — the assembler generates it
2. **V+T sections in `client_identity` modules** — they are invisible in the assembled output
3. **Invisible section headings** — any `##` heading other than the recognized list will be silently ignored
4. **Multi-scenario branches** — "if your cluster uses X, do Y; if it uses Z, do W"
5. **Edge cases as inline guidance** — rare scenarios disguised as required steps
6. **Architecture explanations before field-entry steps** — the engineer needs to fill in a field, not understand the token re-issuance model
7. **Duplicate content** — same guidance in a numbered step and in troubleshooting
8. **Passive voice** — "the proxy should be configured" → "configure the proxy"
9. **Vague steps** — "deploy the updated function" without showing how
10. **Fabricated UI paths** — if unsure of the exact navigation, ask or use `[VERIFY]` marker

---

## Aembit Product Knowledge

When writing module content that requires knowledge of Aembit's product (credential provider types, trust provider types, UI field names, console navigation, API structure):

1. **First:** Load the `aembit-knowledge` skill for authoritative product details
2. **If not found there:** Fetch `https://docs.aembit.io/llms.txt` to identify the relevant docs pages, then fetch the specific page
3. **If still uncertain:** Stop and ask the user rather than guessing

Never fabricate Aembit UI field names, credential provider types, or navigation paths. These must be verified.

---

## File Paths and Naming

Module files live at:
```
~/.claude/skills/poc-documentation/content/<type>/<name>.md
```

Where `<type>` is one of: `client_identity`, `server_integration`, `client_deployment`, `infrastructure`, `access_conditions`

And `<name>` is `lowercase_snake_case` describing the integration (e.g., `github_actions_oidc`, `azure_blob_storage`).

---

## Registry Update (Write mode only)

After writing a new module, add it to the component registry at:
`~/.claude/skills/poc-scoper/references/components.md`

Add a row to the appropriate table section with:
- **Component Path**: `<type>/<name>`
- **Use When**: one concise sentence describing when an SE would select this component
- **Business Value Hint** (client_identity and server_integration only): one sentence framing the customer business value, starting with a verb (e.g., "Eliminating static credentials stored in...")

---

## Review Output Format

When in Review mode, group findings as:

```
### <module filename>

**Too sparse** (steps that would block a mid-level engineer):
- [step number or section]: specific issue

**Too dense** (content that adds noise without helping):
- [step number or section]: specific issue

**Assembly issues** (section headings, redundant steps, invisible content):
- specific issue

**Convention violations** (placeholder gaps, formatting, voice):
- specific issue

**No issues** (if clean)
```

At the end, call out the top 1-3 modules needing the most attention.
