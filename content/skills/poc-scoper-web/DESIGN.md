# POC Web Skills — Design & Implementation Plan

## Overview

Two new Claude web skills for POC documentation, replacing the CLI-only workflow with a web-native approach that uses GitHub MCP for content, Google Drive for storage, and Playwright for PDF rendering.

## Architecture

```
Claude Web Project (with project instructions)
  ├── poc-scoper-web        → Interviews SE → recipe YAML → saves to Drive
  ├── poc-documentation-web → Fetches content → renders PDF → uploads to Drive
  ├── analyze-customer-input → Extracts UCs/FRs from calls → writes to Jira (unchanged)
  └── log-product-feedback   → Logs single FR to Jira (unchanged)

External systems:
  GitHub MCP    → content modules, CSS, assets (read-only)
  Google Drive  → recipes, PDFs, customer logos (read/write)
  Jira MCP      → customer intelligence: UCs, FRs, observations (read/write)
  Gong MCP      → call transcripts (read-only, used by analyze-customer-input)
```

### System Boundaries

| System | Responsibility |
|--------|---------------|
| **Jira** | Customer intelligence only — use cases, FRs, observations (written by analyze-customer-input and log-product-feedback). No recipes or guides. |
| **Google Drive** | POC documentation — recipes, generated PDFs, customer logos. Folder: `Customer Resources/<customer>/poc/` |
| **GitHub (private repo)** | Content module library, CSS, static assets (Aembit logos). Source of truth for reusable doc components. |
| **Claude web sandbox** | PDF rendering via Playwright. Pure computation, no I/O — receives pre-fetched content, returns PDF bytes. |

### Skills That Don't Change

These skills work in Claude web as-is (prompt-only, Jira MCP for I/O):

- **analyze-customer-input** — Reads transcripts/docs, classifies into UCs/FRs/Observations, writes to Jira. No filesystem or Drive dependency. Feeds use case information into poc-scoper-web conversationally.
- **log-product-feedback** — Logs single FRs to Jira conversationally. Fully independent.

### Drive Folder Structure

```
Customer Resources/<customer>/poc/
  <customer>_poc_guide.yaml              ← recipe (from poc-scoper-web)
  <customer>_impl_guide.yaml             ← recipe (from poc-scoper-web)
  <customer>_POC_Guide.pdf               ← generated (from poc-documentation-web)
  <customer>_Implementation_Guide.pdf    ← generated (from poc-documentation-web)
```

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Recipe storage | Google Drive, not Jira | Recipes are build artifacts that belong alongside PDFs, not in ticket descriptions |
| Recipe format | YAML (unchanged) | Structured enough for assembler, readable enough for SEs, clean serialization for Drive |
| Code sharing | Copy pure functions into assembler_web.py | Web sandbox is isolated, can't import from repo at runtime |
| Markdown-to-HTML | Python `markdown` library | Replaces pandoc, no system deps, available in Claude web |
| PDF rendering | Playwright Chromium | Available in Claude web, good CSS support |
| Cover page headers | Two-pass PDF (cover without, body with) merged via pypdf | Preserves current visual behavior, pypdf confirmed available |
| Asset handling | Base64 data URIs | No filesystem in sandbox |
| Module discovery | GitHub MCP `list_directory` on `content/` at session start | One call gives full manifest for validation |
| Jira role | None in this workflow | analyze-customer-input handles Jira; POC docs are Drive-only |

## Skill 1: poc-scoper-web

### Purpose

Conversational skill that interviews the SE, maps use cases to Aembit components, and produces validated recipe YAML files saved to Drive.

### What Changes from CLI poc-scoper

| Aspect | CLI | Web |
|--------|-----|-----|
| Module validation | `Path.exists()` on local filesystem | GitHub MCP `get_file_contents` / directory listing |
| Logo validation | Local file path check | Drive search in `Customer Resources/<customer>/` |
| Recipe output | YAML files written to local filesystem | YAML saved to customer's Drive `poc/` folder |
| Jira integration | Creates epic + stories via MCP | Not needed — Jira tickets come from analyze-customer-input |
| Component library | Read from local `references/components.md` | Fetched from GitHub MCP at session start |

### Workflow

1. **Session start**: Fetch content module directory tree from GitHub MCP (one call). Cache as available module manifest.
2. **Interview phases**: Same as CLI — collect customer info, use cases, component mapping. Unchanged.
3. **Module validation**: Check each component path against cached manifest. Flag missing modules.
4. **Logo check**: Search Drive for image files in `Customer Resources/<customer>/`. If found, note the file ID. If not, inform SE and omit from recipe.
5. **Recipe output**: Generate POC guide and impl guide YAML. Save to `Customer Resources/<customer>/poc/` in Drive.
6. **Handoff**: Offer to immediately generate PDFs via poc-documentation-web, or let SE trigger later.

### AddUseCase Workflow

Fetch existing recipe YAML from Drive → add new use case via conversation → validate new modules → save updated recipe back to Drive.

## Skill 2: poc-documentation-web

### Purpose

Generate PDFs from recipes using content modules from GitHub and assets from Drive. Pure rendering — no scoping or intelligence.

### Orchestration Flow

Claude (the LLM) orchestrates all I/O because the Python sandbox cannot call MCP:

```
1. Claude reads recipe YAML (from Drive or conversation)
2. Claude parses recipe, identifies needed files:
   - Content modules: shared/cover, shared/introduction, + recipe-specific modules
   - CSS: styles/aembit.css
   - Assets: assets/aembit-logo-white.png, assets/aembit-icon-small.png
   - Customer logo (from Drive, if recipe has CUSTOMER_LOGO)
3. Claude fetches all files via GitHub MCP (content, CSS, assets)
4. Claude fetches customer logo via Drive connector (if needed)
5. Claude passes everything to Python sandbox as a single call:
   {
     recipe: <dict>,
     content_modules: {"shared/cover": "<md>", "client_identity/ec2_iam_role": "<md>", ...},
     css: "<css text>",
     assets: {"aembit-logo-white.png": "<base64>", "aembit-icon-small.png": "<base64>"},
     customer_logo: "<base64>" or null
   }
6. Python sandbox runs assembler_web.py → returns PDF bytes
7. Claude uploads PDF to Customer Resources/<customer>/poc/ in Drive
8. Claude reports Drive link to SE
```

### assembler_web.py Design

**Entry point:**
```python
def assemble_web(
    recipe: dict,
    content_modules: dict[str, str],
    css_text: str,
    assets: dict[str, str],        # filename -> base64
    customer_logo: str | None,     # base64 or None
) -> bytes:
    """Assemble a PDF from pre-fetched content. Returns PDF bytes."""
```

**Functions copied from assembler.py (pure, no I/O):**
- `substitute_vars()`
- `extract_section()`
- `count_numbered_steps()`
- `make_final_policy_step()`
- `infer_title()` + `TITLE_OVERRIDES`
- `post_process_html()`

**Functions rewritten:**
- `assemble()` → `assemble_web()` — reads from content dict instead of filesystem
- `render_use_case()` — same logic, reads from content dict
- `md_to_html()` — Python `markdown` library instead of pandoc
- `wrap_html()` — embeds assets as base64 data URIs, strips @page CSS
- `html_to_pdf()` — Playwright instead of weasyprint

**New functions:**
- `build_header_template()` — Playwright header HTML (icon + confidential text)
- `build_footer_template()` — Playwright footer HTML (page numbers)
- `build_css()` — strips @page blocks, converts asset URLs to base64
- `merge_pdfs()` — combines cover (no header) + body (with header) via pypdf

### CSS Changes for Playwright

1. Remove all `@page { ... }` blocks (margin boxes not supported by Chromium)
2. Remove `@page:first { ... }` block
3. Headers/footers move to Playwright `headerTemplate`/`footerTemplate`
4. `page-break-*` properties work as-is in Chromium
5. Google Font `@import` works (sandbox has network access)
6. Asset `url()` references become base64 data URIs

### Header/Footer Templates

**Header** (replaces `@top-left` icon + `@top-right` confidential text):
```html
<div style="width:100%; display:flex; justify-content:space-between;
            align-items:center; padding:0 20mm;
            font-family:'Be Vietnam Pro',Arial,sans-serif; font-size:7.5pt;">
  <img src="data:image/png;base64,{icon_b64}" style="height:16px;">
  <span style="font-weight:500; color:#29204C; letter-spacing:0.5pt;">
    {customer_name} Confidential
  </span>
</div>
```

**Footer** (replaces `@bottom-center` page counter):
```html
<div style="width:100%; text-align:center;
            font-family:'Be Vietnam Pro',Arial,sans-serif; font-size:7.5pt; color:#666;">
  {customer_name} {doc_title} — Page <span class="pageNumber"></span>
  of <span class="totalPages"></span>
</div>
```

### Cover Page Header Suppression

Two-pass rendering + merge via pypdf:
1. Render cover page HTML with `display_header_footer=False`
2. Render body HTML with `display_header_footer=True` + templates
3. Merge PDFs with pypdf

## Implementation Sequence

### Phase 1: assembler_web.py core
- Copy pure utility functions from assembler.py
- Implement in-memory content loading (dict-based)
- Implement `md_to_html()` with Python `markdown` library
- Implement CSS processing (strip @page, convert asset URLs)
- **Test**: Pass sample recipe + content, verify HTML output

### Phase 2: Playwright PDF rendering
- Implement `html_to_pdf()` with Playwright sync API
- Implement header/footer templates
- Implement two-pass cover + body rendering
- Implement PDF merge via pypdf
- **Test**: Render HTML from Phase 1 to PDF, verify visually

### Phase 3: poc-scoper-web SKILL.md
- Write SKILL.md with GitHub MCP validation instructions
- Write ScopeSession workflow adapted for Drive storage
- Write AddUseCase workflow for Drive-based recipes
- Include component library reference
- **Can parallel with Phases 1-2**

### Phase 4: poc-documentation-web SKILL.md
- Write SKILL.md with orchestration instructions
- Define the file-fetching checklist (which files for each recipe type)
- Define Drive upload instructions
- **Depends on Phase 2** (need assembler_web interface finalized)

### Phase 5: Integration testing
- End-to-end: poc-scoper-web → recipe in Drive → poc-documentation-web → PDF in Drive
- Test with: one POC guide, one impl guide with 2+ use cases
- Compare visual output to CLI-generated PDFs

### Phase 6: Migration path
- Document how to cut over from CLI skills to web skills
- Identify any CLI-only features not yet ported
- Plan deprecation of old skills

## Claude Web Project Instructions

The Claude web Project is the orchestration layer — the web equivalent of the poc-manager agent in Claude Code. Project instructions tell Claude which skill to use, how the skills connect, and what conventions to follow.

### Project Setup

The Project requires these connections:
- **GitHub MCP server** — connected to the private `ai-tools` repo (read access)
- **Google Drive connector** — enabled (read/write)
- **Jira MCP server** — connected to Atlassian (for analyze-customer-input and log-product-feedback)
- **Gong MCP server** — connected (for analyze-customer-input transcript fetching)
- **Code execution** — enabled (for PDF generation via Playwright)

### Project Instructions Content

The project instructions should cover:

#### 1. Skill Routing

| SE Intent | Skill | Action |
|-----------|-------|--------|
| "Scope a POC for X" / "new POC" | poc-scoper-web | Conversational interview → recipe YAML → save to Drive |
| "Generate docs for X" / "build the PDF" | poc-documentation-web | Fetch recipe from Drive → fetch modules from GitHub → render PDF → upload to Drive |
| "Analyze this transcript" / "extract from calls" | analyze-customer-input | Extract UCs/FRs/Observations → write to Jira |
| "Log this FR" / "customer mentioned X" | log-product-feedback | Collect FR details → dedup → write to Jira |
| "Add a use case to X's POC" | poc-scoper-web (AddUseCase) | Fetch recipe from Drive → add use case → save back |

#### 2. Workflow Sequencing

Typical POC lifecycle in Claude web:

```
1. analyze-customer-input → extract UCs and FRs from Gong calls → Jira tickets
2. poc-scoper-web → interview SE using extracted UCs → recipe YAML → Drive
3. poc-documentation-web → generate PDFs from recipe → Drive
4. (Optional) SE shares Drive link with customer
```

Steps 1-3 can happen in the same conversation or across sessions. The recipe in Drive is the handoff artifact between scoper and documentation.

#### 3. Conventions

- **Drive folder**: `Customer Resources/<customer>/poc/` for all POC artifacts
- **GitHub repo**: `<org>/ai-tools` — content modules at `skills/poc-documentation/content/`
- **Recipe files**: `<customer_slug>_poc_guide.yaml` and `<customer_slug>_impl_guide.yaml`
- **PDF files**: `<Customer>_POC_Guide.pdf` and `<Customer>_Implementation_Guide.pdf`
- **Customer slug**: lowercase, hyphens for spaces (e.g., `state-farm`)

#### 4. Cross-Skill Data Flow

```
analyze-customer-input
  └── writes UCs/FRs to Jira (SOL project)
  └── SE reviews and confirms

poc-scoper-web
  └── reads confirmed UCs from conversation context (SE describes them)
  └── validates component paths against GitHub content library
  └── validates customer logo in Drive
  └── writes recipe YAML to Drive

poc-documentation-web
  └── reads recipe YAML from Drive
  └── fetches content modules from GitHub
  └── fetches assets from GitHub
  └── fetches customer logo from Drive
  └── renders PDF in sandbox
  └── uploads PDF to Drive
```

#### 5. Error Conventions

- **Missing content module**: Report the module path and which recipe component references it. Do not proceed to PDF generation. Suggest the SE check if the module name is correct or if a new module needs to be created.
- **Missing customer logo**: Inform the SE, proceed without logo (recipe omits `CUSTOMER_LOGO`).
- **Recipe not found in Drive**: Ask the SE to run poc-scoper-web first, or provide the recipe YAML directly.
- **PDF rendering failure**: Report the error from the sandbox. Offer to retry or debug.

### Implementation Note

The project instructions are a markdown document, not code. They will be written as a single file that gets pasted into the Claude web Project's custom instructions field. This is Phase 3.5 in the implementation sequence — after the skills are written but before integration testing.

---

## Known Differences from CLI Version

| Feature | CLI | Web |
|---------|-----|-----|
| Orchestration | poc-manager agent | Claude web Project instructions |
| Font rendering | Cairo/Pango (weasyprint) | Chromium Skia (Playwright) — generally better |
| `counter(pages)` | CSS counter | `<span class="totalPages">` in Playwright template — works |
| Cover page header suppression | `@page:first` CSS | Two-pass render + pypdf merge |
| Content loading | Local filesystem | GitHub MCP → conversation → Python sandbox |
| Output delivery | Local PDF file | Drive upload |
| Jira role | Stores recipes + tickets | Customer intelligence only (UCs/FRs/Observations) |
| Recipe storage | Jira epic description | Google Drive |
