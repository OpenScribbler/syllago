---
name: poc-documentation-web
description: Generates customer POC PDFs from recipe YAML using GitHub MCP for content modules and Google Drive for storage. Trigger on "generate docs for", "build the PDF", "generate the implementation guide".
---

# POC Documentation — Web

Generates customer POC PDFs from recipe YAML using GitHub MCP for content modules and Google Drive for recipe/PDF storage. This is a mechanical, non-conversational skill — follow the steps exactly. Do NOT write custom scripts — use the bundled `run_assembly.py` and `assembler_web.py` as-is.

## Quick Reference

| Resource | Location |
|----------|----------|
| Recipe source | Google Drive or pasted in conversation |
| GitHub repo | `aemccormick/ai-tools` (use this for all GitHub MCP calls) |
| Content modules | GitHub MCP: `skills/poc-documentation/content/` |
| CSS | GitHub MCP: `skills/poc-documentation/styles/aembit.css` |
| Assets | Bundled in this skill: `assets/aembit-logo-white.png.b64`, `assets/aembit-icon-small.png.b64` |
| Assembler code | Bundled in this skill: `assembler_web.py`, `run_assembly.py` |
| Customer logo | Google Drive (if recipe has `CUSTOMER_LOGO` var) |
| PDF output | Google Drive |

## Workspace Layout

All files go into a workspace directory before running the assembler. The runner script reads from this fixed structure:

```
/home/claude/workspace/
    recipe.yaml
    content/
        shared/cover.md
        shared/introduction.md
        infrastructure/agent_controller.md
        client_identity/ec2_iam_role.md
        ...
    css/
        aembit.css
    assets/
        aembit-logo-white.png.b64
        aembit-icon-small.png.b64
    customer_logo.b64              (optional)
```

## Step 1: Get Recipe

**Option A — From Drive:**
Search Google Drive for the recipe YAML. Read the file content.

**Option B — From conversation:**
The SE pastes the recipe YAML directly.

Parse the YAML to identify which content modules to fetch.

## Step 2: Identify Required Files

From the parsed recipe, collect all content module paths needed:

**Always needed:**
- `shared/cover`
- `shared/introduction` (unless recipe has `introduction: false`)

**From `sections` list (flat recipe):**
Each entry is a content module path. Skip `shared/` entries (already covered).

**From `infrastructure` list:**
Each entry is a content module path.

**From `use_cases` list:**
For each use case: `client_identity`, `server_integration`, `client_deployment`, each `access_conditions` entry.

## Step 3: Set Up Workspace

In the Python sandbox, create the workspace directory structure and populate it:

### 3a: Copy bundled files
Copy `assembler_web.py` and `run_assembly.py` from this skill into `/home/claude/workspace/`.
Copy `assets/aembit-logo-white.png.b64` and `assets/aembit-icon-small.png.b64` into `/home/claude/workspace/assets/`.

### 3b: Write recipe
Write the recipe YAML to `/home/claude/workspace/recipe.yaml`.

### 3c: Fetch and write content modules from GitHub MCP
For each content module path identified in Step 2, fetch from GitHub MCP:
- `get_file_contents` on repo `aemccormick/ai-tools`, path `skills/poc-documentation/content/{module_path}.md`
- Write the content to `/home/claude/workspace/content/{module_path}.md`

Create subdirectories as needed (e.g., `content/shared/`, `content/infrastructure/`, `content/client_identity/`).

### 3d: Fetch and write CSS from GitHub MCP
- `get_file_contents` on repo `aemccormick/ai-tools`, path `skills/poc-documentation/styles/aembit.css`
- Write to `/home/claude/workspace/css/aembit.css`

### 3e: Customer logo (optional)
If the recipe has `CUSTOMER_LOGO` in vars, fetch the logo from Drive, base64-encode it, and write to `/home/claude/workspace/customer_logo.b64`.

## Step 4: Run Assembly

In the Python sandbox, run:

```bash
cd /home/claude/workspace
python run_assembly.py --pdf
```

The default is `--pdf`. If Playwright is not available and PDF rendering fails, fall back to `--html` and inform the SE.

Do NOT modify `run_assembly.py` or `assembler_web.py`. If something fails, report the error and ask the user to update the skill scripts.

## Step 5: Deliver Output

- If rendering locally: offer the generated file for download from `/home/claude/workspace/output.html` or `output.pdf`.
- If Drive is available: upload to the customer's Drive folder.
- Report the output filename from the recipe's `output` field.

## Error Handling

| Error | Action |
|-------|--------|
| Recipe not found in Drive | Ask SE to run poc-scoper-web first, or paste the recipe YAML directly |
| Content module not found in GitHub | Report the module path and which recipe component references it. Do not proceed. |
| Customer logo not found in Drive | Inform SE, proceed without logo (skip customer_logo.b64) |
| run_assembly.py fails | Report the full error output. Do NOT attempt to fix or rewrite the scripts. Ask the user to update the skill. |
| Playwright not available | Use `--html` flag instead of `--pdf`. Inform SE that PDF rendering is unavailable. |

## What This Skill Does NOT Do

- Does not create or edit recipe YAML — use poc-scoper-web for that
- Does not create Jira tickets — use analyze-customer-input for that
- Does not modify content modules, CSS, or assembler code
- Does not write custom assembly scripts — uses only the bundled `run_assembly.py`
