"""
Aembit POC Document Assembler — Web/Sandbox Version

This module assembles documents from pre-fetched content for use in
Claude's web Python sandbox. All content (markdown modules, CSS, assets)
arrives via function parameters — there is no filesystem I/O anywhere.

Returns HTML string by default, or PDF bytes when render_pdf=True.
PDF rendering uses Playwright (Chromium) with a two-pass approach:
cover page (no header/footer) + body (with header/footer), merged via pypdf.

Usage (Python):
    from assembler_web import assemble_web

    # HTML only
    html = assemble_web(recipe=..., content_modules=..., css_text=..., assets=..., customer_logo=None)

    # PDF rendering
    pdf_bytes = assemble_web(recipe=..., content_modules=..., css_text=..., assets=..., customer_logo=None, render_pdf=True)
"""

import io
import re
import markdown


# ------------------------------------------------------------------ #
# Pure utility functions (copied verbatim from assembler.py)          #
# ------------------------------------------------------------------ #


def substitute_vars(content: str, vars: dict) -> str:
    """
    Substitute {{VAR_NAME}} tokens that are in the vars dict.
    Tokens NOT in the vars dict are left as-is (they are POC-time placeholders).
    """
    for key, value in vars.items():
        content = content.replace(f"{{{{{key}}}}}", str(value))
    return content


def extract_section(md_content: str, heading: str) -> str:
    """
    Extract the body of a ## heading section from markdown content.

    Returns everything between the target ## heading and the next ## heading
    (or end of file), excluding the heading line itself.
    """
    lines = md_content.split("\n")
    in_section = False
    collected = []

    for line in lines:
        if line.startswith("## ") and line[3:].strip() == heading:
            in_section = True
            continue
        if in_section:
            if line.startswith("## "):
                break
            collected.append(line)

    return "\n".join(collected)


def count_numbered_steps(text: str) -> int:
    """Count ordered list items (lines starting with a digit and period) in extracted markdown."""
    return sum(1 for line in text.split("\n") if re.match(r"^\d+\.", line.strip()))


def make_final_policy_step(n: int, label: str | None = None, has_access_conditions: bool = False) -> str:
    """Generate the boilerplate 'create access policy' step at position n."""
    ac_suffix = "\n   - Attach the **Access Condition(s)** configured above" if has_access_conditions else ""
    if label:
        return (
            f"{n}. Navigate to **Access Policies** and create **{label}**:\n"
            f"   - Client Workload (step 1) \u2192 Trust Provider (step 2) "
            f"\u2192 Credential Provider (step 3) \u2192 Server Workload (step 4)"
            f"{ac_suffix}"
        )
    return (
        f"{n}. Navigate to **Access Policies** and create a new Access Policy connecting:\n"
        f"   - Client Workload \u2192 Trust Provider \u2192 Credential Provider \u2192 Server Workload"
        f"{ac_suffix}"
    )


TITLE_OVERRIDES = {
    "crowdstrike": "CrowdStrike",
    "oidc": "OIDC",
    "saml": "SAML",
    "iam": "IAM",
    "oauth": "OAuth",
    "vmware": "VMware",
}


def infer_title(section_path: str) -> str:
    """Infer a human-readable title from a section path like 'infrastructure/agent_controller'."""
    name = section_path.split("/")[-1]
    words = name.split("_")
    titled = [TITLE_OVERRIDES.get(word.lower(), word.title()) for word in words]
    return " ".join(titled)


def is_placeholder_path(path: str) -> bool:
    """Check if a module path is a placeholder (e.g. '{{CUSTOM}}' or contains '{{')."""
    return "{{" in path


def post_process_html(html: str) -> str:
    """Post-process HTML: style placeholders and differentiate blockquote types."""
    # Wrap backtick-rendered {{PLACEHOLDER}} tokens in .placeholder spans
    html = re.sub(
        r'<code>(\{\{[A-Z_0-9]+\}\})</code>',
        r'<span class="placeholder">\1</span>',
        html,
    )
    # Style "Start Here" blockquotes with a distinct class
    html = re.sub(
        r'<blockquote>\s*<p><strong>Start Here',
        '<blockquote class="callout-start"><p><strong>Start Here',
        html,
    )
    # Style "Note" blockquotes with a distinct class
    html = re.sub(
        r'<blockquote>\s*<p><strong>Note',
        '<blockquote class="callout-note"><p><strong>Note',
        html,
    )
    return html


# ------------------------------------------------------------------ #
# Markdown to HTML conversion                                         #
# ------------------------------------------------------------------ #


def md_to_html(md_text: str) -> str:
    """Convert markdown to HTML body fragment using the Python markdown library."""
    return markdown.markdown(
        md_text,
        extensions=["tables", "fenced_code", "codehilite", "attr_list"],
        output_format="html5",
    )


# ------------------------------------------------------------------ #
# Use case rendering                                                   #
# ------------------------------------------------------------------ #


def render_use_case(
    uc: dict, vars: dict, module_number: int, content_modules: dict[str, str],
    warnings: list[str] | None = None,
) -> str:
    """Render a use case YAML entry into a Markdown string.

    All content module reads use the content_modules dict instead of filesystem.
    Raises KeyError if a required module is missing from the dict.
    """
    if warnings is None:
        warnings = []
    name = uc.get("name", "Unnamed Use Case")
    overview = uc.get("overview", "")
    business_value = uc.get("business_value", "")
    business_value_hint = uc.get("business_value_hint", "")

    client_identity_path = uc.get("client_identity", "")
    server_integration_path = uc.get("server_integration", "")
    client_deployment_path = uc.get("client_deployment", "")
    access_condition_paths = uc.get("access_conditions", [])

    # Skip placeholder paths (e.g. {{CUSTOM}}) — warn and treat as empty
    for field, path in [("client_identity", client_identity_path),
                        ("server_integration", server_integration_path),
                        ("client_deployment", client_deployment_path)]:
        if path and is_placeholder_path(path):
            warnings.append(f"Use case '{name}': {field} path '{path}' is a placeholder — skipping module")
    if is_placeholder_path(client_identity_path):
        client_identity_path = ""
    if is_placeholder_path(server_integration_path):
        server_integration_path = ""
    if is_placeholder_path(client_deployment_path):
        client_deployment_path = ""
    access_condition_paths = [p for p in access_condition_paths if not is_placeholder_path(p)]

    verification = uc.get("verification", [])
    success_criteria = uc.get("success_criteria", [])
    troubleshooting = uc.get("troubleshooting", [])

    parts = []

    # H1 heading with page break and anchor ID
    anchor = f"module-{module_number}"
    parts.append(f'<div class="page-break"></div>\n\n<h1 id="{anchor}">Module {module_number}: {name}</h1>')

    # Overview
    parts.append(f"## Overview\n\n{overview}")

    # Business Value — resolve vars first so a resolved value is not wrapped in backticks
    business_value = substitute_vars(business_value, vars)
    bv_line = f"`{business_value}`" if business_value.startswith("{{") else business_value
    bv_hint = f"\n\n> *{business_value_hint}*" if business_value_hint else ""
    parts.append(f"## Business Value\n\n{bv_line}{bv_hint}")

    # Prerequisites — aggregate from component files
    prereq_sections = []
    for path in [client_identity_path, server_integration_path, client_deployment_path] + access_condition_paths:
        if path:
            if path not in content_modules:
                raise KeyError(f"Content module not found: '{path}' (required by use case '{name}')")
            content = content_modules[path]
            extracted = extract_section(content, "Prerequisites")
            if extracted and extracted.strip():
                prereq_sections.append(extracted.strip())

    prereq_body = "\n".join(prereq_sections) if prereq_sections else ""
    parts.append(f"## Prerequisites\n\n{prereq_body}")

    # Values You'll Need — aggregate from component files
    values_ref_sections = []
    for path in [client_identity_path, server_integration_path, client_deployment_path] + access_condition_paths:
        if path:
            if path not in content_modules:
                raise KeyError(f"Content module not found: '{path}' (required by use case '{name}')")
            content = content_modules[path]
            extracted = extract_section(content, "Values Reference")
            if extracted and extracted.strip():
                values_ref_sections.append(extracted.strip())

    if values_ref_sections:
        # Deduplicate rows across component tables by first column (var name)
        seen = set()
        deduped_rows = []
        for section in values_ref_sections:
            for line in section.split("\n"):
                # Match data rows: | `{{VAR}}` | ... |
                if line.startswith("|") and not re.match(r"\|\s*[-:]+\s*\|", line):
                    first_col = line.split("|")[1].strip()
                    if first_col in ("Value", ""):
                        deduped_rows.append(line)  # header row always included
                    elif first_col not in seen:
                        seen.add(first_col)
                        deduped_rows.append(line)
        header = "| Value | Where to Find It |\n|-------|-----------------|"
        # Filter out extra header/separator rows, keep just one at the top
        data_rows = [r for r in deduped_rows if not re.match(r"\|\s*(Value|[-:]+)\s*\|", r)]
        values_ref_body = header + "\n" + "\n".join(data_rows)
        parts.append(f"## Values You'll Need\n\n{values_ref_body}")

    # Part 1: Service Configuration — from server_integration
    part1_body = ""
    if server_integration_path:
        if server_integration_path not in content_modules:
            raise KeyError(
                f"Content module not found: '{server_integration_path}' "
                f"(server_integration for use case '{name}')"
            )
        content = content_modules[server_integration_path]
        part1_body = extract_section(content, "Service Configuration").strip()
    parts.append(f"## Part 1: Service Configuration\n\n{part1_body}")

    # Deprecation warning for old recipes still using policy_pattern
    if uc.get("policy_pattern"):
        warnings.append(
            f"'policy_pattern' key in use case '{name}' is deprecated and ignored. "
            f"Use 'policy_chain' instead."
        )

    policy_chain = uc.get("policy_chain", "single")
    policy_chain_labels = uc.get("policy_chain_labels", [])

    # Part 2: Access Policy Configuration — assembled from component files
    part2_parts = []

    if policy_chain == "dual":
        # Policy 1: full content from client_identity's ## Aembit Configuration
        label1 = policy_chain_labels[0] if len(policy_chain_labels) > 0 else "Policy 1"
        ci_aembit = ""
        if client_identity_path:
            if client_identity_path not in content_modules:
                raise KeyError(
                    f"Content module not found: '{client_identity_path}' "
                    f"(client_identity for use case '{name}')"
                )
            ci_aembit = extract_section(content_modules[client_identity_path], "Aembit Configuration").strip()
        n1 = count_numbered_steps(ci_aembit) + 1
        final_step1 = make_final_policy_step(n1, label1)
        part2_parts.append(f"### {label1}\n\n{ci_aembit}\n\n{final_step1}")

        # Policy 2: full content from server_integration's ## Aembit Configuration
        label2 = policy_chain_labels[1] if len(policy_chain_labels) > 1 else "Policy 2"
        si_aembit = ""
        if server_integration_path:
            if server_integration_path not in content_modules:
                raise KeyError(
                    f"Content module not found: '{server_integration_path}' "
                    f"(server_integration for use case '{name}')"
                )
            si_aembit = extract_section(content_modules[server_integration_path], "Aembit Configuration").strip()

        # Extract access condition Aembit Configuration steps for Policy 2
        ac_aembit_parts = []
        for ac_path in access_condition_paths:
            if ac_path not in content_modules:
                raise KeyError(
                    f"Content module not found: '{ac_path}' "
                    f"(access_condition for use case '{name}')"
                )
            ac_text = extract_section(content_modules[ac_path], "Aembit Configuration").strip()
            if ac_text:
                ac_aembit_parts.append(ac_text)

        combined_p2 = "\n\n".join(filter(None, [si_aembit] + ac_aembit_parts))
        n2 = count_numbered_steps(combined_p2) + 1
        has_ac = bool(access_condition_paths)
        final_step2 = make_final_policy_step(n2, label2, has_access_conditions=has_ac)
        part2_parts.append(f"### {label2}\n\n{combined_p2}\n\n{final_step2}")

    else:  # single
        ci_aembit = ""
        if client_identity_path:
            if client_identity_path not in content_modules:
                raise KeyError(
                    f"Content module not found: '{client_identity_path}' "
                    f"(client_identity for use case '{name}')"
                )
            ci_aembit = extract_section(content_modules[client_identity_path], "Aembit Configuration").strip()

        si_aembit = ""
        if server_integration_path:
            if server_integration_path not in content_modules:
                raise KeyError(
                    f"Content module not found: '{server_integration_path}' "
                    f"(server_integration for use case '{name}')"
                )
            si_aembit = extract_section(content_modules[server_integration_path], "Aembit Configuration").strip()

        # Extract access condition Aembit Configuration steps
        ac_aembit_parts = []
        for ac_path in access_condition_paths:
            if ac_path not in content_modules:
                raise KeyError(
                    f"Content module not found: '{ac_path}' "
                    f"(access_condition for use case '{name}')"
                )
            ac_text = extract_section(content_modules[ac_path], "Aembit Configuration").strip()
            if ac_text:
                ac_aembit_parts.append(ac_text)

        combined_aembit = "\n\n".join(filter(None, [ci_aembit, si_aembit] + ac_aembit_parts))
        n = count_numbered_steps(combined_aembit) + 1
        has_ac = bool(access_condition_paths)
        final_step = make_final_policy_step(n, has_access_conditions=has_ac)
        part2_parts.append(f"{combined_aembit}\n\n{final_step}")

    part2_body = "\n\n".join(part2_parts)
    parts.append(f"## Part 2: Access Policy Configuration\n\n{part2_body}")

    # Part 3: Client Workload Deployment — from client_deployment
    part3_body = ""
    if client_deployment_path:
        if client_deployment_path not in content_modules:
            raise KeyError(
                f"Content module not found: '{client_deployment_path}' "
                f"(client_deployment for use case '{name}')"
            )
        content = content_modules[client_deployment_path]
        part3_body = extract_section(content, "Deployment").strip()
    parts.append(f"## Part 3: Client Workload Deployment\n\n{part3_body}")

    # Verification
    if verification:
        items = "\n".join(f"- {item}" for item in verification)
        parts.append(f"## Verification\n\n{items}")

    # Success Criteria
    if success_criteria:
        items = "\n".join(f"<li>{item}</li>" for item in success_criteria)
        parts.append(f'## Success Criteria\n\n<ul class="checklist">\n{items}\n</ul>')

    # Troubleshooting
    if troubleshooting:
        items = "\n".join(f"- {item}" for item in troubleshooting)
        parts.append(f"## Troubleshooting\n\n{items}")

    # Substitute vars into the whole use case block
    combined = "\n\n".join(parts)
    combined = substitute_vars(combined, vars)
    return combined


# ------------------------------------------------------------------ #
# CSS processing                                                       #
# ------------------------------------------------------------------ #


def build_css(css_text: str, assets: dict[str, str]) -> str:
    """Process CSS for web/Playwright rendering.

    1. Strip @page { ... } block (WeasyPrint margin boxes, not supported by Chromium)
    2. Strip @page:first { ... } block
    3. Replace url() references to asset files with base64 data URIs
    Returns cleaned CSS string.
    """
    # Strip @page { ... } block (including nested margin box content)
    css_text = re.sub(r'@page\s*\{[^}]*(?:\{[^}]*\}[^}]*)*\}', '', css_text)

    # Strip @page:first { ... } block
    css_text = re.sub(r'@page\s*:first\s*\{[^}]*(?:\{[^}]*\}[^}]*)*\}', '', css_text)

    # Replace url() references to asset files with base64 data URIs
    def replace_url(match):
        url_path = match.group(1).strip('\'"')
        # Extract just the filename from paths like "assets/aembit-icon-small.png"
        filename = url_path.split("/")[-1]
        if filename in assets:
            # Infer MIME type from extension
            ext = filename.rsplit(".", 1)[-1].lower()
            mime_types = {"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg", "svg": "image/svg+xml"}
            mime = mime_types.get(ext, "application/octet-stream")
            return f'url("data:{mime};base64,{assets[filename]}")'
        return match.group(0)

    css_text = re.sub(r'url\(([^)]+)\)', replace_url, css_text)

    # Override cover page styles for Chromium's print model.
    # With zero margins on the cover PDF, the negative-margin trick from
    # WeasyPrint is unnecessary and breaks layout. Replace with zero margins
    # and internal padding. Also fix h1 negative margins for body pages.
    css_text += """

/* Chromium print overrides */
.cover-band {
  margin: 0 !important;
  padding: 60px 40px 44px 40px !important;
}
.cover-page {
  page-break-after: always !important;
  margin: 0 !important;
  padding: 0 !important;
}
.cover-meta {
  padding: 0 40px !important;
}
.cover-customer-logo {
  padding: 0 40px !important;
}
h1 {
  margin-left: 0 !important;
  margin-right: 0 !important;
  padding-left: 20px !important;
  padding-right: 20px !important;
}
"""

    return css_text


# ------------------------------------------------------------------ #
# HTML document wrapping                                               #
# ------------------------------------------------------------------ #


def wrap_html(
    body: str,
    css_text: str,
    assets: dict[str, str],
    vars: dict,
    customer_logo: str | None,
) -> str:
    """Wrap HTML body in a full document with inlined CSS and base64 assets."""
    title = vars.get("CUSTOMER_NAME", "Aembit POC Guide")

    # Build cleaned CSS (strip @page blocks, convert asset URLs to data URIs)
    processed_css = build_css(css_text, assets)

    # Convert cover page Aembit logo to base64 data URI
    if "aembit-logo-white.png" in assets:
        body = body.replace(
            'src="assets/aembit-logo-white.png"',
            f'src="data:image/png;base64,{assets["aembit-logo-white.png"]}"',
        )

    # Handle customer logo
    if customer_logo:
        # Replace customer logo placeholder with base64 img tag
        body = re.sub(
            r'<div class="cover-customer-logo">.*?</div>',
            f'<div class="cover-customer-logo">'
            f'<img src="data:image/png;base64,{customer_logo}" style="max-height:60px;">'
            f'</div>',
            body,
            flags=re.DOTALL,
        )
    else:
        # Strip the customer logo div entirely
        body = re.sub(
            r'<div class="cover-customer-logo">.*?</div>',
            '',
            body,
            flags=re.DOTALL,
        )

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{title} — Aembit POC Guide</title>
  <style>{processed_css}</style>
</head>
<body>
{body}
</body>
</html>"""


# ------------------------------------------------------------------ #
# Playwright PDF rendering                                             #
# ------------------------------------------------------------------ #


def build_header_template(assets: dict[str, str], customer_name: str) -> str:
    """Build Playwright headerTemplate HTML.

    Replicates the WeasyPrint @top-left (icon) and @top-right (confidential text).
    """
    icon_b64 = assets.get("aembit-icon-small.png", "")
    icon_src = f"data:image/png;base64,{icon_b64}" if icon_b64 else ""
    confidential = f"Aembit &amp; {customer_name} Confidential" if customer_name else "Aembit Confidential"
    return (
        '<div style="width:100%; display:flex; justify-content:space-between;'
        ' align-items:center; padding:0 20mm;'
        " font-family:'Be Vietnam Pro',Arial,sans-serif; font-size:7.5pt;\">"
        f'<img src="{icon_src}" style="height:16px;">'
        f'<span style="font-weight:500; color:#29204C; letter-spacing:0.5pt;">'
        f'{confidential}</span>'
        '</div>'
    )


def build_footer_template(customer_name: str, doc_title: str) -> str:
    """Build Playwright footerTemplate HTML.

    Replicates the WeasyPrint @bottom-center page counter.
    """
    prefix = f"{customer_name} {doc_title} — " if customer_name else ""
    return (
        '<div style="width:100%; text-align:center;'
        " font-family:'Be Vietnam Pro',Arial,sans-serif; font-size:7.5pt; color:#666;\">"
        f'{prefix}Page <span class="pageNumber"></span>'
        ' of <span class="totalPages"></span>'
        '</div>'
    )


def split_cover_body(html: str) -> tuple[str, str]:
    """Split a full HTML document into cover-only and body-only documents.

    The cover is everything inside <div class="cover-page">...</div>.
    The body is everything else. Both get the same <head> wrapper.
    """
    # Extract head (everything up to and including </head>) and body content
    head_match = re.search(r'(<!DOCTYPE html>.*?</head>)', html, re.DOTALL)
    body_match = re.search(r'<body>\s*(.*?)\s*</body>', html, re.DOTALL)
    if not head_match or not body_match:
        raise ValueError("Cannot parse HTML document structure for cover/body split")

    head = head_match.group(1)
    body_content = body_match.group(1)

    # Split on the cover-page div — must handle nested divs correctly
    start_tag = '<div class="cover-page">'
    start_idx = body_content.find(start_tag)
    if start_idx == -1:
        # No cover page found — return empty cover, full body
        return "", html

    # Walk forward counting div nesting to find the matching closing </div>
    depth = 0
    i = start_idx
    while i < len(body_content):
        if body_content[i:].startswith('<div'):
            depth += 1
            i += 4
        elif body_content[i:].startswith('</div>'):
            depth -= 1
            if depth == 0:
                end_idx = i + len('</div>')
                break
            i += 6
        else:
            i += 1
    else:
        # Malformed HTML — couldn't find closing div
        return "", html

    cover_html_body = body_content[start_idx:end_idx]
    body_html_body = body_content[end_idx:]

    cover_doc = f"{head}\n<body>\n{cover_html_body}\n</body>\n</html>"
    body_doc = f"{head}\n<body>\n{body_html_body}\n</body>\n</html>"
    return cover_doc, body_doc


def html_to_pdf(
    html: str,
    display_header_footer: bool = False,
    header_template: str = "",
    footer_template: str = "",
    margin: dict | None = None,
) -> bytes:
    """Render HTML to PDF using Playwright's Chromium.

    Args:
        html: Complete HTML document string.
        display_header_footer: Whether to show header/footer.
        header_template: Playwright headerTemplate HTML.
        footer_template: Playwright footerTemplate HTML.
        margin: Page margins dict. Defaults to body margins (22mm/20mm).

    Returns:
        PDF bytes.
    """
    if margin is None:
        margin = {"top": "22mm", "right": "20mm", "bottom": "20mm", "left": "20mm"}

    from playwright.sync_api import sync_playwright

    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        page.set_content(html, wait_until="networkidle")
        pdf_bytes = page.pdf(
            format="Letter",
            margin=margin,
            display_header_footer=display_header_footer,
            header_template=header_template if display_header_footer else "",
            footer_template=footer_template if display_header_footer else "",
            print_background=True,
        )
        browser.close()
    return pdf_bytes


def merge_pdfs(cover_pdf: bytes, body_pdf: bytes) -> bytes:
    """Merge cover PDF (no header/footer) with body PDF (with header/footer).

    Returns combined PDF bytes.
    """
    from pypdf import PdfReader, PdfWriter

    writer = PdfWriter()

    if cover_pdf:
        cover_reader = PdfReader(io.BytesIO(cover_pdf))
        for page in cover_reader.pages:
            writer.add_page(page)

    body_reader = PdfReader(io.BytesIO(body_pdf))
    for page in body_reader.pages:
        writer.add_page(page)

    output = io.BytesIO()
    writer.write(output)
    return output.getvalue()


# ------------------------------------------------------------------ #
# Main orchestrator                                                    #
# ------------------------------------------------------------------ #


def assemble_web(
    recipe: dict,
    content_modules: dict[str, str],
    css_text: str,
    assets: dict[str, str],
    customer_logo: str | None,
    render_pdf: bool = False,
) -> str | bytes:
    """Assemble a document from pre-fetched content.

    Args:
        recipe: Parsed recipe dict (already deserialized from YAML).
        content_modules: Map of module path -> markdown content (e.g. "shared/cover" -> "# Cover\n...").
        css_text: Raw CSS text from aembit.css.
        assets: Map of asset filename -> base64 encoded content.
        customer_logo: Base64 encoded customer logo image, or None.
        render_pdf: If True, render to PDF via Playwright and return bytes.
                    If False, return complete HTML string.

    Returns:
        HTML string (render_pdf=False) or PDF bytes (render_pdf=True).
    """
    vars = recipe.get("vars", {})
    warnings = []  # Collected for HTML comment output

    # Detect recipe type: impl guides don't use business_goals/success_criteria/exec_summary
    is_impl_guide = "use_cases" in recipe or "infrastructure" in recipe

    # Render top-level list fields into vars before substitution.

    # business_goals: list -> numbered bold markdown -> vars["BUSINESS_GOALS_RENDERED"]
    goals = recipe.get("business_goals") or []
    if not goals and not is_impl_guide:
        warnings.append("'business_goals' list is empty — the Business Goals section will be blank")
    vars["BUSINESS_GOALS_RENDERED"] = "\n".join(
        f"{i + 1}. **{goal}**" for i, goal in enumerate(goals)
    )

    # success_criteria: list of dicts -> markdown table -> vars["SUCCESS_CRITERIA_TABLE"]
    rows = recipe.get("success_criteria") or []
    if not rows and not is_impl_guide:
        warnings.append("'success_criteria' list is empty — the Success Criteria section will be blank")
    header = "| No | Test Case | Success Criterion | Mandatory |"
    separator = "|----|-----------|-------------------|-----------|"
    data_rows = [
        f"| {row.get('no', '')} | {row.get('test_case', '')} "
        f"| {row.get('criterion', '')} "
        f"| {'Yes' if row.get('mandatory', True) else 'No'} |"
        for row in rows
    ]
    vars["SUCCESS_CRITERIA_TABLE"] = "\n".join([header, separator] + data_rows)

    # exec_summary_use_cases: list -> markdown bullet list -> vars["EXEC_SUMMARY_USE_CASES"]
    items = recipe.get("exec_summary_use_cases") or []
    if not items and not is_impl_guide:
        warnings.append("'exec_summary_use_cases' list is empty — the Executive Summary use cases will be blank")
    vars["EXEC_SUMMARY_USE_CASES"] = "\n".join(f"- {item}" for item in items)

    # contacts: list of dicts -> markdown table rows -> vars["CUSTOMER_CONTACTS_TABLE"]
    contacts = recipe.get("contacts") or []
    if contacts:
        contact_rows = [
            f"| {c.get('name', '')} | {c.get('role', '')} | {c.get('email', '')} |"
            for c in contacts
        ]
        vars["CUSTOMER_CONTACTS_TABLE"] = "\n".join(contact_rows)
    else:
        # Fall back to legacy CONTACT_1_* vars if no contacts list
        legacy_name = vars.get("CONTACT_1_NAME", "")
        legacy_role = vars.get("CONTACT_1_ROLE", "")
        legacy_email = vars.get("CONTACT_1_EMAIL", "")
        if legacy_name:
            vars["CUSTOMER_CONTACTS_TABLE"] = f"| {legacy_name} | {legacy_role} | {legacy_email} |"
        else:
            vars["CUSTOMER_CONTACTS_TABLE"] = "| {{CONTACT_NAME}} | {{CONTACT_ROLE}} | {{CONTACT_EMAIL}} |"

    md_parts = []

    # Render cover
    if "shared/cover" in content_modules:
        content = content_modules["shared/cover"]
        content = substitute_vars(content, vars)
        # Strip customer logo block if CUSTOMER_LOGO was not provided
        if "CUSTOMER_LOGO" not in vars:
            content = re.sub(
                r'<div class="cover-customer-logo">.*?</div>',
                "",
                content,
                flags=re.DOTALL,
            )
        md_parts.append(content)

    # Render introduction (default: true, can be disabled with introduction: false)
    if recipe.get("introduction", True):
        if "shared/introduction" in content_modules:
            content = content_modules["shared/introduction"]
            content = substitute_vars(content, vars)
            # For impl guides, append a "What's Covered" section listing all modules
            if "infrastructure" in recipe or "use_cases" in recipe:
                listing_num = 1
                listing_items = []
                for section in recipe.get("infrastructure", []):
                    title = infer_title(section)
                    listing_items.append(f"{listing_num}. **{title}** - Infrastructure setup")
                    listing_num += 1
                for uc in recipe.get("use_cases", []):
                    name = uc.get("name", "Unnamed Use Case")
                    listing_items.append(f"{listing_num}. **{name}** - Use case implementation")
                    listing_num += 1
                if listing_items:
                    content += "\n\n## What's Covered\n\n" + "\n".join(listing_items) + "\n"
            md_parts.append(content)

    # Handle flat sections list (legacy / simple mode)
    if "sections" in recipe:
        sections = recipe.get("sections", [])
        # Skip shared/ entries already rendered above
        for section in sections:
            if section.startswith("shared/"):
                continue
            if section not in content_modules:
                raise KeyError(f"Section not found in content_modules: '{section}'")
            content = content_modules[section]
            content = substitute_vars(content, vars)
            md_parts.append(content)

    # Handle structured use case mode
    if "infrastructure" in recipe or "use_cases" in recipe:
        module_number = 1

        # Auto-inject infrastructure dependencies based on use case modules.
        MODULE_INFRA_DEPS = {
            "client_identity/claude_okta_oidc": "infrastructure/okta_oidc",
            "access_conditions/crowdstrike_posture": "infrastructure/crowdstrike_integration",
        }
        existing_infra = set(recipe.get("infrastructure", []))
        for uc in recipe.get("use_cases", []):
            for field in ["client_identity", "server_integration", "client_deployment"]:
                module_path = uc.get(field, "")
                dep = MODULE_INFRA_DEPS.get(module_path)
                if dep and dep not in existing_infra:
                    recipe.setdefault("infrastructure", []).append(dep)
                    existing_infra.add(dep)
                    warnings.append(f"Auto-injected '{dep}' (required by '{module_path}')")
            for ac_path in uc.get("access_conditions", []):
                dep = MODULE_INFRA_DEPS.get(ac_path)
                if dep and dep not in existing_infra:
                    recipe.setdefault("infrastructure", []).append(dep)
                    existing_infra.add(dep)
                    warnings.append(f"Auto-injected '{dep}' (required by '{ac_path}')")

        # Build Table of Contents from module titles
        toc_entries = []
        toc_number = 1
        for section in recipe.get("infrastructure", []):
            title = infer_title(section)
            toc_entries.append((toc_number, f"Module {toc_number}: {title}", "infrastructure"))
            toc_number += 1
        for uc in recipe.get("use_cases", []):
            name = uc.get("name", "Unnamed Use Case")
            toc_entries.append((toc_number, f"Module {toc_number}: {name}", "use_case"))
            toc_number += 1

        if toc_entries:
            toc_md = '<div class="page-break"></div>\n\n'
            toc_md += '<h1>Table of Contents</h1>\n\n'
            toc_md += '<div class="toc">\n'
            for num, entry, entry_type in toc_entries:
                label = "Infrastructure" if entry_type == "infrastructure" else "Use Case"
                anchor = f"module-{num}"
                toc_md += f'<a href="#{anchor}" class="toc-entry"><span class="toc-label">{label}</span> <span class="toc-title">{entry}</span></a>\n'
            toc_md += '</div>\n'
            md_parts.append(toc_md)

        # Infrastructure sections
        for section in recipe.get("infrastructure", []):
            if section not in content_modules:
                raise KeyError(f"Infrastructure module not found in content_modules: '{section}'")
            infra_md = content_modules[section]
            infra_md = substitute_vars(infra_md, vars)

            # Wrap in H1 module heading with page break and anchor ID
            title = infer_title(section)
            anchor = f"module-{module_number}"
            wrapper = f'<div class="page-break"></div>\n\n<h1 id="{anchor}">Module {module_number}: {title}</h1>\n\n{infra_md}'
            md_parts.append(wrapper)
            module_number += 1

        # Use case sections
        for uc in recipe.get("use_cases", []):
            uc_md = render_use_case(uc, vars, module_number, content_modules, warnings)
            md_parts.append(uc_md)
            module_number += 1

    combined_md = "\n\n".join(md_parts)

    # Collect unresolved {{VAR_NAME}} tokens for debugging (included as HTML comment)
    unresolved = sorted(set(re.findall(r'\{\{[A-Z_0-9]+\}\}', combined_md)))

    # Convert markdown to HTML
    html_body = md_to_html(combined_md)

    # Post-process: wrap backtick-rendered {{PLACEHOLDER}} tokens in .placeholder spans
    html_body = post_process_html(html_body)

    # Wrap in full HTML document
    html = wrap_html(html_body, css_text, assets, vars, customer_logo)

    # Prepend debug comments (warnings + unresolved placeholders)
    comments = []
    if warnings:
        comment_lines = "\n  ".join(warnings)
        comments.append(f"<!-- WARNINGS ({len(warnings)}):\n  {comment_lines}\n-->")
    if unresolved:
        comments.append(f"<!-- UNRESOLVED PLACEHOLDERS ({len(unresolved)}): {', '.join(unresolved)} -->")
    if comments:
        html = "\n".join(comments) + "\n" + html

    if not render_pdf:
        return html

    # --- PDF rendering path ---
    customer_name = vars.get("CUSTOMER_NAME", "")
    doc_title = vars.get("DOCUMENT_TITLE", "POC Guide")

    header_template = build_header_template(assets, customer_name)
    footer_template = build_footer_template(customer_name, doc_title)

    # Split into cover (no header/footer) and body (with header/footer)
    cover_html, body_html = split_cover_body(html)

    cover_pdf = b""
    if cover_html:
        cover_pdf = html_to_pdf(
            cover_html,
            display_header_footer=False,
            margin={"top": "0mm", "right": "0mm", "bottom": "0mm", "left": "0mm"},
        )

    body_pdf = html_to_pdf(
        body_html,
        display_header_footer=True,
        header_template=header_template,
        footer_template=footer_template,
    )

    return merge_pdfs(cover_pdf, body_pdf)
