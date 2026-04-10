"""
jira_helpers.py — Jira API helpers for POC document workflows.

Used by assembler.py for --from-jira (export) and file attachment.
All other Jira operations use the Atlassian MCP server.

Environment variables:
    JIRA_BASE_URL    — e.g., https://aembit.atlassian.net
    JIRA_EMAIL       — API user email
    JIRA_API_TOKEN   — Atlassian API token
"""

import os
import re
import sys
import time
from pathlib import Path

import requests
import yaml


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

# Standard POC guide sections (always the same)
POC_GUIDE_SECTIONS = [
    "poc_guide/executive_summary",
    "poc_guide/contacts",
    "poc_guide/business_goals",
    "poc_guide/success_criteria",
    "poc_guide/timeline",
]

# Section header pattern: Markdown (##)
SECTION_RE = re.compile(r"^##\s+(.+)$", re.MULTILINE)


# ---------------------------------------------------------------------------
# Auth Helpers
# ---------------------------------------------------------------------------

def _get_env(name: str) -> str:
    """Get required environment variable or exit with error."""
    value = os.environ.get(name)
    if not value:
        print(f"Error: {name} environment variable is not set", file=sys.stderr)
        sys.exit(1)
    return value


def _get_base_url() -> str:
    """Return the Jira REST API base URL.

    Scoped tokens require the cloud gateway: api.atlassian.com/ex/jira/<cloud_id>
    Classic tokens use the instance directly: <instance>.atlassian.net

    Set JIRA_CLOUD_ID to use scoped tokens via the gateway.
    Discover your cloud ID at: https://<instance>.atlassian.net/_edge/tenant_info
    """
    cloud_id = os.environ.get("JIRA_CLOUD_ID")
    if cloud_id:
        return f"https://api.atlassian.com/ex/jira/{cloud_id}"
    return _get_env("JIRA_BASE_URL").rstrip("/")


def _apply_auth(kwargs: dict) -> dict:
    """Apply Basic auth (email:token) to a requests kwargs dict.

    Both classic and scoped API tokens use Basic auth.
    """
    email = _get_env("JIRA_EMAIL")
    token = _get_env("JIRA_API_TOKEN")
    kwargs["auth"] = (email, token)
    return kwargs


# ---------------------------------------------------------------------------
# Jira API Helpers
# ---------------------------------------------------------------------------

def _jira_request(method: str, path: str, raise_on_error: bool = True, **kwargs) -> requests.Response:
    """Make an authenticated Jira REST API v2 request (plain text descriptions)."""
    base_url = _get_base_url()

    url = f"{base_url}/rest/api/2{path}"

    kwargs = _apply_auth(kwargs)
    resp = requests.request(
        method,
        url,
        timeout=30,
        **kwargs,
    )

    if not resp.ok:
        print(
            f"Error: Jira API {method} {path} returned {resp.status_code}",
            file=sys.stderr,
        )
        try:
            error_body = resp.json()
            errors = error_body.get("errors", {})
            error_messages = error_body.get("errorMessages", [])
            if errors:
                for field, msg in errors.items():
                    print(f"  {field}: {msg}", file=sys.stderr)
            for msg in error_messages:
                print(f"  {msg}", file=sys.stderr)
        except Exception:
            print(f"  {resp.text[:500]}", file=sys.stderr)
        if raise_on_error:
            sys.exit(1)

    return resp


# ---------------------------------------------------------------------------
# Data Formatting Helpers
# ---------------------------------------------------------------------------

def _slugify(name: str) -> str:
    """Convert customer name to slug: lowercase, spaces to hyphens, alphanumeric only."""
    slug = name.lower().strip()
    slug = re.sub(r"[^a-z0-9\s-]", "", slug)
    slug = re.sub(r"[\s]+", "-", slug)
    slug = re.sub(r"-+", "-", slug)
    return slug.strip("-")


def _title_case_slug(slug: str) -> str:
    """Convert slug back to title case for filenames: state-farm -> State_Farm."""
    return "_".join(word.capitalize() for word in slug.split("-"))


# ---------------------------------------------------------------------------
# Description Parsing
# ---------------------------------------------------------------------------

def _parse_description(description: str) -> dict:
    """Parse the structured description back into named sections.

    Splits on ## headers (Markdown).
    Returns a dict mapping section names (uppercase) to their text content.
    """
    sections = {}

    parts = SECTION_RE.split(description)

    # parts alternates: [preamble, name1, content1, name2, content2, ...]
    i = 1  # Skip preamble (anything before first header)
    while i < len(parts) - 1:
        name = parts[i].strip()
        content = parts[i + 1].strip()
        sections[name.upper()] = content
        i += 2

    return sections


def _parse_markdown_table(text: str) -> list:
    """Parse a Markdown table into a list of dicts.

    | Header1 | Header2 |  -> headers
    |---------|---------|  -> separator (skipped)
    | val1    | val2    |  -> row
    """
    lines = [l.strip() for l in text.strip().splitlines() if l.strip()]
    if not lines:
        return []

    # Find header row: starts with | and not a separator
    headers = []
    data_start = 0
    for i, line in enumerate(lines):
        if line.startswith("|"):
            cells = [c.strip() for c in line.split("|")]
            cells = [c for c in cells if c]  # remove empties from leading/trailing |
            if cells and not re.match(r"^[-:]+$", cells[0]):
                headers = cells
                data_start = i + 1
                break

    if not headers:
        return []

    rows = []
    for line in lines[data_start:]:
        if not line.startswith("|"):
            continue
        # Skip separator rows (|---|---|)
        if re.match(r"^\|[\s\-:|]+\|$", line):
            continue
        cells = [c.strip() for c in line.split("|")]
        cells = [c for c in cells if c]
        if len(cells) >= len(headers):
            rows.append(dict(zip(headers, cells)))

    return rows


def _parse_table(text: str) -> list:
    """Parse a Markdown table."""
    return _parse_markdown_table(text)


def _parse_customer_section(text: str) -> dict:
    """Parse the Customer section. Handles Markdown table and bold-key list formats."""
    # Try Markdown table format first (| Field | Value |)
    rows = _parse_table(text)
    if rows:
        result = {}
        for row in rows:
            field = row.get("Field", "")
            value = row.get("Value", "")
            if field:
                result[field] = value
        return result

    # Try Markdown bold-key list format: - **Field**: value
    result = {}
    for line in text.strip().splitlines():
        line = line.strip()
        if not line:
            continue
        # Markdown bold key: - **Name**: State Farm
        bold_match = re.match(r"^-\s+\*\*(.+?)\*\*:\s*(.+)$", line)
        if bold_match:
            result[bold_match.group(1).strip()] = bold_match.group(2).strip()
            continue
        # Plain key: value format
        if ":" in line:
            key, _, value = line.partition(":")
            result[key.strip()] = value.strip()
    return result


def _parse_customer_field(value: str) -> tuple:
    """Parse 'Name (email)' format into (name, email). Returns (value, '') if no parens."""
    match = re.match(r"^(.+?)\s*\(([^)]+)\)\s*$", value)
    if match:
        return match.group(1).strip(), match.group(2).strip()
    return value.strip(), ""


def _parse_contacts(text: str) -> list:
    """Parse contacts from Markdown table format."""
    rows = _parse_table(text)
    return [{"name": r.get("Name", ""), "role": r.get("Role", ""), "email": r.get("Email", "")} for r in rows]


def _parse_success_criteria(text: str) -> list:
    """Parse success criteria from Markdown table format."""
    rows = _parse_table(text)
    criteria = []
    for r in rows:
        mandatory_val = r.get("Required", r.get("Mandatory", "")).lower()
        criteria.append({
            "no": int(r.get("No", r.get("#", 0))),
            "test_case": r.get("Test Case", ""),
            "criterion": r.get("Criterion", r.get("Success Criterion", "")),
            "mandatory": mandatory_val in ("mandatory", "yes"),
        })
    return criteria


def _parse_business_goals(text: str) -> list:
    """Parse business goals from Markdown list or plain text."""
    goals = []
    for line in text.strip().splitlines():
        line = line.strip()
        if not line or line.startswith("_"):
            continue
        # Strip Markdown bullet (- ), numbered list (1. ), or asterisk bullet (* )
        if line.startswith("- "):
            line = line[2:]
        elif re.match(r"^\d+\.\s+", line):
            line = re.sub(r"^\d+\.\s+", "", line)
        # Strip optional bold wrapper from Markdown (e.g., **goal text**)
        line = re.sub(r"^\*\*(.+)\*\*$", r"\1", line)
        goals.append(line)
    return goals


def _extract_code_block(text: str) -> str:
    """Extract content from a Jira {code} block or Markdown fenced code block."""
    # Markdown fenced code block: ```yaml ... ``` or ````yaml ... ````
    match = re.search(r"(`{3,})(?:yaml)?\s*\n(.*?)\1", text, re.DOTALL)
    if match:
        return match.group(2).strip()
    return text.strip()


# ---------------------------------------------------------------------------
# Attachment Helpers
# ---------------------------------------------------------------------------

def _download_attachment(attachment: dict, dest: Path) -> None:
    """Download a Jira attachment to a local file."""
    url = attachment.get("content", "")
    if not url:
        return

    kwargs = _apply_auth({})
    resp = requests.get(url, timeout=30, **kwargs)
    if resp.ok:
        with open(dest, "wb") as f:
            f.write(resp.content)
    else:
        print(
            f"  Warning: Could not download attachment {attachment.get('filename', '')}: {resp.status_code}",
            file=sys.stderr,
        )


def _attach_file(issue_key: str, file_path: Path, retries: int = 3) -> None:
    """Attach a file to a Jira issue, with retry on connection errors."""
    base_url = _get_base_url()
    url = f"{base_url}/rest/api/2/issue/{issue_key}/attachments"

    for attempt in range(1, retries + 1):
        try:
            kwargs = _apply_auth({"headers": {"X-Atlassian-Token": "no-check"}})
            with open(file_path, "rb") as f:
                resp = requests.post(
                    url,
                    files={"file": (file_path.name, f)},
                    timeout=120,
                    **kwargs,
                )

            if not resp.ok:
                print(
                    f"  Warning: Failed to attach {file_path.name}: {resp.status_code}",
                    file=sys.stderr,
                )
            return
        except (requests.ConnectionError, requests.Timeout) as e:
            if attempt < retries:
                wait = attempt * 3
                print(f"  Retry {attempt}/{retries} for {file_path.name} (waiting {wait}s)...", file=sys.stderr)
                time.sleep(wait)
            else:
                print(f"  Warning: Failed to attach {file_path.name} after {retries} attempts: {e}", file=sys.stderr)


# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------

def cmd_export(issue_key: str, output_dir: str) -> None:
    """Export a Jira ticket to POC recipe YAML files."""
    out_path = Path(output_dir)
    out_path.mkdir(parents=True, exist_ok=True)

    # Fetch the ticket
    resp = _jira_request("GET", f"/issue/{issue_key}")
    issue = resp.json()
    fields = issue["fields"]

    # Parse structured description
    description = fields.get("description", "") or ""
    if not description:
        print(f"Error: Ticket {issue_key} has no description", file=sys.stderr)
        sys.exit(1)

    sections = _parse_description(description)

    # Parse CUSTOMER section
    customer_data = _parse_customer_section(sections.get("CUSTOMER", ""))
    customer_name = customer_data.get("Name", "Unknown")
    ae_name, ae_email = _parse_customer_field(customer_data.get("AE", ""))
    sa_name, sa_email = _parse_customer_field(customer_data.get("SA", ""))
    poc_start = customer_data.get("POC Start", "{{POC_START_DATE}}")
    closeout = customer_data.get("Closeout", "{{TIMELINE_CLOSEOUT_DATE}}")

    # Parse other sections
    exec_summary = sections.get("EXECUTIVE SUMMARY", "")
    contacts = _parse_contacts(sections.get("CONTACTS", ""))
    business_goals = _parse_business_goals(sections.get("BUSINESS GOALS", ""))
    success_criteria = _parse_success_criteria(sections.get("SUCCESS CRITERIA", ""))

    # Parse technical recipe YAML (extract from {code} block if present)
    tech_recipe_raw = _extract_code_block(sections.get("TECHNICAL RECIPE", ""))
    tech_recipe = {}
    if tech_recipe_raw:
        try:
            tech_recipe = yaml.safe_load(tech_recipe_raw) or {}
        except yaml.YAMLError as e:
            print(
                f"Error: Invalid YAML in TECHNICAL RECIPE section:\n  {e}",
                file=sys.stderr,
            )
            sys.exit(1)

    # Also use due date from standard field as fallback for closeout
    due_date = fields.get("duedate", "") or ""
    if due_date and closeout == "{{TIMELINE_CLOSEOUT_DATE}}":
        closeout = due_date

    # Check for customer logo in attachments
    logo_path = ""
    attachments = fields.get("attachment", [])
    logo_extensions = {".png", ".jpg", ".jpeg", ".svg"}
    for att in attachments:
        name = att.get("filename", "")
        if any(name.lower().endswith(ext) for ext in logo_extensions):
            logo_local = out_path / name
            _download_attachment(att, logo_local)
            logo_path = str(logo_local)
            print(f"  Downloaded logo: {name}")
            break

    # Derive values
    slug = _slugify(customer_name)
    title_slug = _title_case_slug(slug)

    use_cases = tech_recipe.get("use_cases", [])
    exec_summary_use_cases = [uc.get("name", "") for uc in use_cases]

    # Build POC guide recipe
    poc_vars = {
        "DOCUMENT_TITLE": "Proof of Concept Guide",
        "CUSTOMER_NAME": customer_name,
        "SA_NAME": sa_name,
        "SA_EMAIL": sa_email,
        "AE_NAME": ae_name,
        "AE_EMAIL": ae_email,
        "POC_START_DATE": poc_start,
        "TIMELINE_CLOSEOUT_DATE": closeout,
        "EXEC_SUMMARY_INTRO": exec_summary,
    }
    if logo_path:
        poc_vars["CUSTOMER_LOGO"] = logo_path

    poc_recipe = {
        "output": f"{title_slug}_POC_Guide.pdf",
        "introduction": False,
        "vars": poc_vars,
        "contacts": contacts,
        "exec_summary_use_cases": exec_summary_use_cases,
        "business_goals": business_goals,
        "success_criteria": success_criteria,
        "sections": POC_GUIDE_SECTIONS,
    }

    # Build impl guide recipe
    impl_vars = {
        "DOCUMENT_TITLE": "Implementation Guide",
        "CUSTOMER_NAME": customer_name,
        "SA_NAME": sa_name,
        "SA_EMAIL": sa_email,
        "AE_NAME": ae_name,
        "AE_EMAIL": ae_email,
        "POC_START_DATE": poc_start,
    }
    if logo_path:
        impl_vars["CUSTOMER_LOGO"] = logo_path

    # Merge impl-specific vars from technical recipe
    tech_vars = tech_recipe.get("vars", {})
    impl_vars.update(tech_vars)

    impl_recipe = {
        "output": f"{title_slug}_Implementation_Guide.pdf",
        "vars": impl_vars,
    }

    if tech_recipe.get("infrastructure"):
        impl_recipe["infrastructure"] = tech_recipe["infrastructure"]

    if use_cases:
        impl_recipe["use_cases"] = use_cases

    # Write YAML files
    poc_path = out_path / f"{slug}_poc_guide.yaml"
    impl_path = out_path / f"{slug}_impl_guide.yaml"

    with open(poc_path, "w") as f:
        yaml.dump(poc_recipe, f, default_flow_style=False, sort_keys=False, width=120)

    with open(impl_path, "w") as f:
        yaml.dump(impl_recipe, f, default_flow_style=False, sort_keys=False, width=120)

    print(f"Exported: {poc_path}")
    print(f"Exported: {impl_path}")


def cmd_attach(issue_key: str, file_path: str) -> None:
    """Attach a file to a Jira issue."""
    fp = Path(file_path)
    if not fp.is_file():
        print(f"Error: File not found: {file_path}", file=sys.stderr)
        sys.exit(1)

    _attach_file(issue_key, fp)
    print(f"Attached: {fp.name} -> {issue_key}")


