"""
Aembit POC Document Assembler

Usage (CLI):
    python3 assembler.py <recipe.yaml> [--output <path>]

Usage (Python):
    from assembler import assemble
    assemble("customers/ctc/ctc_impl_guide.yaml")
"""

import sys
import subprocess
import yaml
import re
from pathlib import Path

TEMPLATES_DIR = Path(__file__).parent
CONTENT_DIR = TEMPLATES_DIR / "content"
STYLES_DIR = TEMPLATES_DIR / "styles"
ASSETS_DIR = TEMPLATES_DIR / "assets"


def validate_recipe(recipe_path: str | Path) -> bool:
    """
    Validate a recipe file: check it loads and all referenced modules exist.
    Returns True if valid, False otherwise.
    """
    recipe_path = Path(recipe_path)
    issues = []

    try:
        recipe = yaml.safe_load(recipe_path.read_text())
    except Exception as e:
        print(f"ERROR: Cannot load recipe: {e}", file=sys.stderr)
        sys.exit(1)

    # Check sections list
    for section in recipe.get("sections", []):
        md_file = CONTENT_DIR / f"{section}.md"
        if not md_file.exists():
            issues.append(f"Missing section: {md_file}")

    # Check infrastructure list
    for section in recipe.get("infrastructure", []):
        md_file = CONTENT_DIR / f"{section}.md"
        if not md_file.exists():
            issues.append(f"Missing infrastructure module: {md_file}")

    # Check use case component paths
    for uc in recipe.get("use_cases", []):
        uc_name = uc.get("name", "unnamed")
        for field in ["client_identity", "server_integration", "client_deployment"]:
            path = uc.get(field, "")
            if path:
                md_file = CONTENT_DIR / f"{path}.md"
                if not md_file.exists():
                    issues.append(f"Missing {field} in '{uc_name}': {md_file}")
        for ac_path in uc.get("access_conditions", []):
            md_file = CONTENT_DIR / f"{ac_path}.md"
            if not md_file.exists():
                issues.append(f"Missing access_condition in '{uc_name}': {md_file}")

    if issues:
        for issue in issues:
            print(f"  {issue}", file=sys.stderr)
        print(f"Recipe has {len(issues)} issue(s)", file=sys.stderr)
        sys.exit(1)
    else:
        print("Recipe valid")
        return True


def assemble(recipe_path: str | Path, output_path: str | Path = None, quiet: bool = False, strict: bool = False) -> Path:
    """
    Assemble a PDF document from a yaml recipe.

    Recipe format (flat sections):
        output: relative/path/to/output.pdf
        vars:
            CUSTOMER_NAME: CTC
            ...
        sections:
            - shared/cover
            - shared/introduction

    Recipe format (structured use cases):
        output: relative/path/to/output.pdf
        vars:
            CUSTOMER_NAME: CTC
            ...
        infrastructure:
            - infrastructure/agent_controller
        use_cases:
            - name: "Use Case Name"
              overview: "..."
              business_value: "{{VAR}}"
              business_value_hint: "e.g. ..."
              client_identity: client_identity/ec2_iam_role
              server_integration: server_integration/box_api_oauth
              client_deployment: client_deployment/ec2_proxy
              policy_pattern: policy_patterns/standard
              verification: [...]
              success_criteria: [...]
              troubleshooting: [...]

    Returns the path to the generated PDF.
    """
    recipe_path = Path(recipe_path)
    recipe = yaml.safe_load(recipe_path.read_text())

    vars = recipe.get("vars", {})

    # Detect recipe type: impl guides don't use business_goals/success_criteria/exec_summary
    is_impl_guide = "use_cases" in recipe or "infrastructure" in recipe

    # Render top-level list fields into vars before substitution.

    # business_goals: list → numbered bold markdown → vars["BUSINESS_GOALS_RENDERED"]
    goals = recipe.get("business_goals") or []
    if not goals and not is_impl_guide and not quiet:
        print(
            "WARNING: 'business_goals' list is empty — the Business Goals section will be blank.",
            file=sys.stderr,
        )
    vars["BUSINESS_GOALS_RENDERED"] = "\n".join(
        f"{i + 1}. **{goal}**" for i, goal in enumerate(goals)
    )

    # success_criteria: list of dicts → markdown table → vars["SUCCESS_CRITERIA_TABLE"]
    rows = recipe.get("success_criteria") or []
    if not rows and not is_impl_guide and not quiet:
        print(
            "WARNING: 'success_criteria' list is empty — the Success Criteria section will be blank.",
            file=sys.stderr,
        )
    header = "| No | Test Case | Success Criterion | Mandatory |"
    separator = "|----|-----------|-------------------|-----------|"
    data_rows = [
        f"| {row.get('no', '')} | {row.get('test_case', '')} "
        f"| {row.get('criterion', '')} "
        f"| {'Yes' if row.get('mandatory', True) else 'No'} |"
        for row in rows
    ]
    vars["SUCCESS_CRITERIA_TABLE"] = "\n".join([header, separator] + data_rows)

    # exec_summary_use_cases: list → markdown bullet list → vars["EXEC_SUMMARY_USE_CASES"]
    items = recipe.get("exec_summary_use_cases") or []
    if not items and not is_impl_guide and not quiet:
        print(
            "WARNING: 'exec_summary_use_cases' list is empty — the use cases list in the Executive Summary will be blank.",
            file=sys.stderr,
        )
    vars["EXEC_SUMMARY_USE_CASES"] = "\n".join(f"- {item}" for item in items)

    # contacts: list of dicts → markdown table rows → vars["CUSTOMER_CONTACTS_TABLE"]
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

    if output_path is None:
        output_path = recipe_path.parent / recipe.get("output", "output.pdf")
    output_path = Path(output_path)

    md_parts = []

    # Render cover
    cover_file = CONTENT_DIR / "shared/cover.md"
    if cover_file.exists():
        content = cover_file.read_text()
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
        intro_file = CONTENT_DIR / "shared/introduction.md"
        if intro_file.exists():
            content = intro_file.read_text()
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
            md_file = CONTENT_DIR / f"{section}.md"
            if not md_file.exists():
                raise FileNotFoundError(f"Section not found: {md_file}")
            content = md_file.read_text()
            content = substitute_vars(content, vars)
            md_parts.append(content)

    # Handle structured use case mode
    if "infrastructure" in recipe or "use_cases" in recipe:
        module_number = 1

        # Auto-inject infrastructure dependencies based on use case modules.
        # If a module requires a prerequisite infrastructure step, add it if not already present.
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
                    print(
                        f"INFO: Auto-injected '{dep}' (required by '{module_path}')",
                        file=sys.stderr,
                    )
            for ac_path in uc.get("access_conditions", []):
                dep = MODULE_INFRA_DEPS.get(ac_path)
                if dep and dep not in existing_infra:
                    recipe.setdefault("infrastructure", []).append(dep)
                    existing_infra.add(dep)
                    print(f"INFO: Auto-injected '{dep}' (required by '{ac_path}')", file=sys.stderr)

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
            md_file = CONTENT_DIR / f"{section}.md"
            if not md_file.exists():
                raise FileNotFoundError(f"Infrastructure section not found: {md_file}")
            infra_md = md_file.read_text()
            infra_md = substitute_vars(infra_md, vars)

            # Wrap in H1 module heading with page break and anchor ID
            title = infer_title(section)
            anchor = f"module-{module_number}"
            wrapper = f'<div class="page-break"></div>\n\n<h1 id="{anchor}">Module {module_number}: {title}</h1>\n\n{infra_md}'
            md_parts.append(wrapper)
            module_number += 1

        # Use case sections
        for uc in recipe.get("use_cases", []):
            uc_md = render_use_case(uc, vars, module_number)
            md_parts.append(uc_md)
            module_number += 1

    combined_md = "\n\n".join(md_parts)

    # Warn on unresolved {{VAR_NAME}} tokens before generating the PDF
    unresolved = sorted(set(re.findall(r'\{\{[A-Z_0-9]+\}\}', combined_md)))
    if unresolved:
        if len(unresolved) <= 5:
            print(f"WARNING: {len(unresolved)} unresolved placeholder(s): {', '.join(unresolved)}", file=sys.stderr)
        else:
            print(f"WARNING: {len(unresolved)} unresolved placeholder(s):", file=sys.stderr)
            for token in unresolved:
                print(f"  {token}", file=sys.stderr)
        if strict:
            print("ERROR: --strict mode: aborting without generating PDF", file=sys.stderr)
            sys.exit(1)

    # Convert markdown to HTML via pandoc
    html_body = md_to_html(combined_md)

    # Post-process: wrap backtick-rendered {{PLACEHOLDER}} tokens in .placeholder spans
    html_body = post_process_html(html_body)

    # Wrap in full HTML document
    css_path = STYLES_DIR / "aembit.css"
    assets_abs = ASSETS_DIR.resolve()
    html = wrap_html(html_body, css_path, assets_abs, vars)

    # Render to PDF via WeasyPrint
    html_to_pdf(html, output_path, base_url=str(TEMPLATES_DIR))

    print(f"Generated: {output_path}")
    return output_path


def render_use_case(uc: dict, vars: dict, module_number: int) -> str:
    """Render a use case YAML entry into a Markdown string."""
    name = uc.get("name", "Unnamed Use Case")
    overview = uc.get("overview", "")
    business_value = uc.get("business_value", "")
    business_value_hint = uc.get("business_value_hint", "")

    client_identity_path = uc.get("client_identity", "")
    server_integration_path = uc.get("server_integration", "")
    client_deployment_path = uc.get("client_deployment", "")
    access_condition_paths = uc.get("access_conditions", [])

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
            md_file = CONTENT_DIR / f"{path}.md"
            if md_file.exists():
                content = md_file.read_text()
                extracted = extract_section(content, "Prerequisites")
                if extracted and extracted.strip():
                    prereq_sections.append(extracted.strip())

    prereq_body = "\n".join(prereq_sections) if prereq_sections else ""
    parts.append(f"## Prerequisites\n\n{prereq_body}")

    # Values You'll Need — aggregate from component files
    values_ref_sections = []
    for path in [client_identity_path, server_integration_path, client_deployment_path] + access_condition_paths:
        if path:
            md_file = CONTENT_DIR / f"{path}.md"
            if md_file.exists():
                content = md_file.read_text()
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
        md_file = CONTENT_DIR / f"{server_integration_path}.md"
        if md_file.exists():
            content = md_file.read_text()
            part1_body = extract_section(content, "Service Configuration").strip()
    parts.append(f"## Part 1: Service Configuration\n\n{part1_body}")

    # Deprecation warning for old recipes still using policy_pattern
    if uc.get("policy_pattern"):
        print(
            f"WARNING: 'policy_pattern' key in use case '{name}' is deprecated and ignored. "
            f"Use 'policy_chain' instead.",
            file=sys.stderr,
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
            md_file = CONTENT_DIR / f"{client_identity_path}.md"
            if md_file.exists():
                ci_aembit = extract_section(md_file.read_text(), "Aembit Configuration").strip()
        n1 = count_numbered_steps(ci_aembit) + 1
        final_step1 = make_final_policy_step(n1, label1)
        part2_parts.append(f"### {label1}\n\n{ci_aembit}\n\n{final_step1}")

        # Policy 2: full content from server_integration's ## Aembit Configuration
        label2 = policy_chain_labels[1] if len(policy_chain_labels) > 1 else "Policy 2"
        si_aembit = ""
        if server_integration_path:
            md_file = CONTENT_DIR / f"{server_integration_path}.md"
            if md_file.exists():
                si_aembit = extract_section(md_file.read_text(), "Aembit Configuration").strip()

        # Extract access condition Aembit Configuration steps for Policy 2
        ac_aembit_parts = []
        for ac_path in access_condition_paths:
            md_file = CONTENT_DIR / f"{ac_path}.md"
            if md_file.exists():
                ac_text = extract_section(md_file.read_text(), "Aembit Configuration").strip()
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
            md_file = CONTENT_DIR / f"{client_identity_path}.md"
            if md_file.exists():
                ci_aembit = extract_section(md_file.read_text(), "Aembit Configuration").strip()

        si_aembit = ""
        if server_integration_path:
            md_file = CONTENT_DIR / f"{server_integration_path}.md"
            if md_file.exists():
                si_aembit = extract_section(md_file.read_text(), "Aembit Configuration").strip()

        # Extract access condition Aembit Configuration steps
        ac_aembit_parts = []
        for ac_path in access_condition_paths:
            md_file = CONTENT_DIR / f"{ac_path}.md"
            if md_file.exists():
                ac_text = extract_section(md_file.read_text(), "Aembit Configuration").strip()
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
        md_file = CONTENT_DIR / f"{client_deployment_path}.md"
        if md_file.exists():
            content = md_file.read_text()
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
            f"   - Client Workload (step 1) → Trust Provider (step 2) "
            f"→ Credential Provider (step 3) → Server Workload (step 4)"
            f"{ac_suffix}"
        )
    return (
        f"{n}. Navigate to **Access Policies** and create a new Access Policy connecting:\n"
        f"   - Client Workload → Trust Provider → Credential Provider → Server Workload"
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


def substitute_vars(content: str, vars: dict) -> str:
    """
    Substitute {{VAR_NAME}} tokens that are in the vars dict.
    Tokens NOT in the vars dict are left as-is (they are POC-time placeholders).
    """
    for key, value in vars.items():
        content = content.replace(f"{{{{{key}}}}}", str(value))
    return content


def md_to_html(markdown: str) -> str:
    """Convert markdown to HTML body fragment using pandoc."""
    result = subprocess.run(
        ["pandoc", "--from=markdown", "--to=html5", "--highlight-style=pygments"],
        input=markdown,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout


def wrap_html(body: str, css_path: Path, assets_dir: Path, vars: dict) -> str:
    """Wrap HTML body in a full document with inlined CSS."""
    title = vars.get("CUSTOMER_NAME", "Aembit POC Guide")
    doc_title = vars.get("DOCUMENT_TITLE", "POC Guide")
    customer_name = vars.get("CUSTOMER_NAME", "")

    # Read CSS and inject runtime values
    css_content = css_path.read_text()

    # Resolve icon path dynamically (fixes portability)
    icon_path = (assets_dir / "aembit-icon-small.png").resolve()
    css_content = re.sub(
        r'content:\s*url\([^)]+aembit-icon-small\.png[^)]*\)',
        f'content: url("{icon_path}")',
        css_content,
    )

    # Inject customer name into confidential header
    confidential_text = f"Aembit & {customer_name} Confidential" if customer_name else "Aembit Confidential"
    css_content = css_content.replace(
        'content: "Aembit Confidential"',
        f'content: "{confidential_text}"',
    )

    # Inject document title into footer
    footer_text = f"{customer_name} {doc_title} — " if customer_name else ""
    css_content = css_content.replace(
        'content: "Page " counter(page) " of " counter(pages)',
        f'content: "{footer_text}Page " counter(page) " of " counter(pages)',
    )

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{title} — Aembit POC Guide</title>
  <style>{css_content}</style>
</head>
<body>
{body}
</body>
</html>"""


def html_to_pdf(html: str, output_path: Path, base_url: str = None) -> None:
    """Render HTML to PDF using WeasyPrint."""
    from weasyprint import HTML
    output_path.parent.mkdir(parents=True, exist_ok=True)
    HTML(string=html, base_url=base_url).write_pdf(str(output_path))


def resolve_recipes(path: str) -> list[Path]:
    """Resolve a path to a list of YAML recipe files."""
    p = Path(path)
    if p.is_dir():
        recipes = sorted(p.glob("*.yaml"))
        if not recipes:
            print(f"Error: No *.yaml files found in {path}", file=sys.stderr)
            sys.exit(1)
        return recipes
    elif p.is_file():
        return [p]
    else:
        print(f"Error: Path not found: {path}", file=sys.stderr)
        sys.exit(1)


def cmd_generate(args) -> None:
    """Generate PDFs from recipe(s), optionally from a Jira ticket."""
    import time

    tmpdir = None
    path = args.path

    # If --from-jira, export recipes to a temp directory
    if args.from_jira:
        import tempfile
        import jira_helpers

        tmpdir = tempfile.mkdtemp()
        print(f"Exporting recipes from {args.from_jira}...")
        try:
            jira_helpers.cmd_export(args.from_jira, tmpdir)
        except SystemExit:
            import shutil
            shutil.rmtree(tmpdir, ignore_errors=True)
            sys.exit(1)
        path = tmpdir

    if not path:
        print("Usage: poc-doc generate <path> [--open] [--quiet] [--strict]", file=sys.stderr)
        print("       poc-doc generate --from-jira <KEY> [--open] [--quiet] [--strict]", file=sys.stderr)
        sys.exit(1)

    recipes = resolve_recipes(path)
    generated = []
    failed = 0

    for recipe_path in recipes:
        print(f"Generating from {recipe_path.name}...")
        try:
            pdf_path = assemble(recipe_path, quiet=args.quiet, strict=args.strict)
            generated.append(pdf_path)
            print(f"  -> {pdf_path}")
        except (FileNotFoundError, SystemExit) as e:
            print(f"Failed: {recipe_path}", file=sys.stderr)
            if str(e):
                print(f"  {e}", file=sys.stderr)
            failed += 1

    print()
    if failed == 0:
        print(f"Generated {len(generated)} of {len(recipes)} PDF(s)")
    else:
        print(f"Generated {len(generated)} of {len(recipes)} PDF(s) ({failed} failed)", file=sys.stderr)

    # Attach PDFs back to Jira if --from-jira was used
    if args.from_jira and generated:
        import jira_helpers

        print(f"Attaching PDFs to {args.from_jira}...")
        for i, pdf in enumerate(generated):
            if i > 0:
                time.sleep(3)
            jira_helpers.cmd_attach(args.from_jira, str(pdf))

    # Open if requested
    if args.open and generated:
        import subprocess as sp
        for pdf in generated:
            sp.run(["open", str(pdf)])

    # Clean up temp directory
    if tmpdir:
        import shutil
        shutil.rmtree(tmpdir, ignore_errors=True)

    if failed > 0:
        sys.exit(1)


def cmd_attach(args) -> None:
    """Attach a file to a Jira issue."""
    import jira_helpers
    jira_helpers.cmd_attach(args.issue_key, args.file)


def cmd_validate(args) -> None:
    """Validate recipe(s) without generating PDFs."""
    recipes = resolve_recipes(args.path)
    total = len(recipes)
    valid = 0

    for recipe_path in recipes:
        print(f"Validating {recipe_path.name}...")
        try:
            validate_recipe(recipe_path)
            valid += 1
        except SystemExit:
            pass

    print()
    if valid == total:
        print(f"All {total} recipe(s) valid")
    else:
        print(f"{valid} of {total} recipe(s) valid", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(
        prog="poc-doc",
        description="POC document assembly tool",
    )
    sub = parser.add_subparsers(dest="command")

    gen = sub.add_parser("generate", help="Generate PDFs from recipe(s)")
    gen.add_argument("path", nargs="?", default=None, help="Recipe file or directory")
    gen.add_argument("--from-jira", metavar="KEY", help="Export recipes from Jira ticket, generate, attach back")
    gen.add_argument("--open", action="store_true", help="Open generated PDFs (macOS)")
    gen.add_argument("--quiet", action="store_true", help="Suppress non-critical warnings")
    gen.add_argument("--strict", action="store_true", help="Exit 1 if unresolved placeholders remain")

    val = sub.add_parser("validate", help="Validate recipe(s) without generating")
    val.add_argument("path", help="Recipe file or directory")

    att = sub.add_parser("attach", help="Attach file to Jira issue")
    att.add_argument("issue_key", help="Jira issue key (e.g., SOL-92)")
    att.add_argument("file", help="File to attach")

    args = parser.parse_args()
    if args.command == "generate":
        cmd_generate(args)
    elif args.command == "validate":
        cmd_validate(args)
    elif args.command == "attach":
        cmd_attach(args)
    else:
        parser.print_help()
        sys.exit(1)
