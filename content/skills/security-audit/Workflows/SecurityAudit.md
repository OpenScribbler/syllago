# SecurityAudit Workflow

> **Trigger:** "security audit", "audit security", "pre-production security review", "security assessment", "vulnerability assessment"

## Purpose

Conduct a structured, interactive security audit including threat modeling, comprehensive scanning, manual review, and report generation.

## Prerequisites

- Access to codebase to audit
- Security scanning tools available (sec-scan wrapper)
- Write access to generate audit report

## Interactive Flow

### Phase 1: Scope and Threat Modeling

Use `AskUserQuestion` to collect inputs **one at a time**:

| Order | Input | Type | Default |
|-------|-------|------|---------|
| 1 | System name | text | (required) |
| 2 | Audit scope | choice | "full" |
| 3 | Compliance context | multi-select | [] |
| 4 | Known assets | text | (optional) |

**Question Examples:**

Q1: "What system or service are we auditing?" -- Include version if relevant (e.g., "user-service v2.3")

Q2: "What scope should this audit cover?" -- Options: Full audit (all phases) / Targeted (specific areas only). If Targeted, ask follow-up about focus areas.

Q3: "Is this audit for any compliance frameworks?" -- Options: SOC2, PCI-DSS, HIPAA, CMMC, None. Multiple selections allowed.

Q4: "What are the critical assets?" -- Examples: PII, payment data, OAuth tokens, external payment API. Skip if unknown.

**After collecting inputs, perform threat modeling per SKILL.md Phase 1** (identify assets, map trust boundaries, enumerate attack vectors).

**Present threat model for confirmation:**

```markdown
## Threat Model Summary

**System**: [name]
**Scope**: [full/targeted]
**Compliance**: [frameworks or "General"]

### Identified Assets
| Asset Type | Examples Found | Risk Level |
|------------|----------------|------------|
| Credentials | [list] | HIGH |
| PII | [list] | HIGH |
| Business Data | [list] | MEDIUM |

### Attack Vectors to Investigate
| Entry Point | Attack Type | Priority |
|-------------|-------------|----------|
| [point 1] | [type] | HIGH |

---
Confirm threat model before proceeding to scanning? [Confirm / Modify / Add vectors]
```

### Phase 2: Comprehensive Scanning

Run security scans per SKILL.md Phase 2 commands:

```bash
export SEC_SCAN_SEVERITY=LOW
export SEC_SCAN_VERBOSE=1
export SEC_SCAN_MAX_ISSUES=100

sec-scan all ./...
```

> **Windows**: Use `sec-scan.ps1` with `$env:VAR="value"` syntax.

If full scan fails, run individual scans as listed in SKILL.md Phase 2.

**Error Handling:**

| Error | Action |
|-------|--------|
| Tool not installed | Note in report, skip scan, continue with others |
| Scan timeout | Note partial results, recommend manual follow-up |
| No findings | Proceed -- absence of findings is valuable data |

**Present scan summary:**

```markdown
## Scan Results

| Tool | Status | Findings |
|------|--------|----------|
| govulncheck | [OK/Failed/Skipped] | X CRIT, X HIGH, X MED |
| gosec | [OK/Failed/Skipped] | X CRIT, X HIGH, X MED |
| trivy-secret | [OK/Failed/Skipped] | X findings |

### Critical/High Findings Preview
1. [CVE-XXXX: Brief description]
2. [gosec G101: Hardcoded credential]

---
Proceed to manual review? [Yes / Re-run specific scan / View details]
```

### Phase 3: Manual Review

Load [references/owasp-checklist.md](../references/owasp-checklist.md) for the full OWASP Top 10 checklist. Walk through each section with the user.

Key areas to review (scanners miss these):
- Business logic flaws (authorization bypasses, workflow manipulation)
- IDOR vulnerabilities (test with multiple user contexts)
- Error handling information leakage
- Session lifecycle issues

**For each finding, document:**

```markdown
### Finding: [Area] - [Issue]

**Severity**: [CRITICAL/HIGH/MEDIUM/LOW]
**Confidence**: [HIGH/MEDIUM/LOW]
**Location**: `path/to/file:line`

**Evidence**: [Code snippet or observation]
**Risk**: [What could go wrong]
**Recommendation**: [Specific fix]
```

### Phase 4: Report Generation

Load [references/report-template.md](../references/report-template.md) and compile all findings.

**Present draft for approval before generating full report:**

```markdown
### Executive Summary
[2-3 sentences summarizing posture]

**Overall Risk Rating**: [CRITICAL/HIGH/MEDIUM/LOW]

| Severity | Count |
|----------|-------|
| CRITICAL | X |
| HIGH | X |
| MEDIUM | X |
| LOW | X |

### Top Findings
1. **[CRIT-001]** [Title] - [Brief impact]
2. **[HIGH-001]** [Title] - [Brief impact]

---
Review draft before finalizing? [Approve / Modify / Add findings]
```

After approval, generate full report using report-template.md structure. Output to `docs/security/audit-[date].md` or user-specified path.

### Phase 5: Completion

```
Security audit complete!

System: [name] | Date: [date] | Report: [path]

Summary: X critical, X high, X medium, X low | Overall: [rating]

Top 3 priorities:
  1. [Action 1]
  2. [Action 2]
  3. [Action 3]

Next steps:
  1. Share report with stakeholders
  2. Create tickets for remediation
  3. Schedule follow-up audit after fixes
```

## Error Handling

| Error | Action |
|-------|--------|
| No codebase found | Ask for correct path |
| Scan tools unavailable | Document which tools missing, proceed with available |
| Cannot write report | Ask for alternative output location |
| User cancels mid-audit | Offer to save partial findings |
| No vulnerabilities found | Generate clean report with positive observations |

## Output Artifacts

| Artifact | Format | Location |
|----------|--------|----------|
| Audit report | Markdown | `docs/security/audit-[date].md` or specified path |
| Threat model | In report | Section within report |
| Scan logs | Reference | Appendix or separate file if verbose |
