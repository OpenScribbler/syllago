# Security Audit Report Template

Use this template when generating security audit reports. Copy and fill in the sections.

```markdown
# Security Audit Report

**System**: [Name/Version]
**Audit Date**: [Date]
**Auditor**: [Agent/Human]
**Scope**: [What was reviewed]

## Executive Summary

[2-3 sentences: Overall security posture, critical findings count, key recommendations]

**Risk Rating**: [CRITICAL / HIGH / MEDIUM / LOW]

| Severity | Count |
|----------|-------|
| CRITICAL | X |
| HIGH | X |
| MEDIUM | X |
| LOW | X |

## Threat Model Summary

**Assets**: [List critical assets]
**Trust Boundaries**: [Brief description]
**Key Attack Vectors**: [Top 3-5]

## Findings

### [CRITICAL-001] Title

**Severity**: CRITICAL | **Confidence**: HIGH
**Location**: `file:line`
**CVSS Estimate**: 9.5 (Attack: Network, Complexity: Low, Auth: None)

**Description**:
[What is the vulnerability]

**Evidence**:
```
[Code snippet or scan output]
```

**Risk**:
[What could an attacker do]

**Remediation**:
[Specific fix with code example]

**References**:
- [CWE/CVE if applicable]
- [OWASP reference]

---

### [HIGH-001] Title
[Same format]

---

## Scan Results Summary

| Tool | Findings | Notes |
|------|----------|-------|
| govulncheck | X HIGH, X MED | [Key findings] |
| gosec | X HIGH, X MED | [Key findings] |
| trivy-secret | X findings | [Key findings] |
| trivy-config | X findings | [Key findings] |

## Compliance Notes

[If applicable: SOC2, PCI-DSS, HIPAA relevant observations]

## Positive Observations

[What's done well - patterns to maintain]

## Recommendations Summary

| Priority | Recommendation | Effort |
|----------|----------------|--------|
| 1 | [Fix X] | [Hours/Days] |
| 2 | [Implement Y] | [Hours/Days] |

## Appendix

### A. Full Scan Output
[Reference to attached scan results]

### B. Files Reviewed
[List of files manually reviewed]
```
