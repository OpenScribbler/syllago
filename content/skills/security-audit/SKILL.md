---
name: security-audit
description: Deep security audit skill with threat modeling, comprehensive scanning, and audit report generation. Use for security-focused reviews before production deployment, after security incidents, or for compliance audits.
---

# Security Audit Skill

Structured approach to deep security analysis: threat modeling, vulnerability scanning, and audit report generation.

## When to Use

- Pre-production security reviews
- Post-incident security audits
- Compliance assessments (SOC2, PCI-DSS, HIPAA)
- Third-party code/dependency audits
- Infrastructure security hardening

## Audit Workflow

### Phase 1: Threat Modeling

**1. Identify Assets** -- Data (PII, credentials, financial, health), critical operations (auth, payments), integrations (APIs, databases, queues).

**2. Map Trust Boundaries** -- Internet (untrusted) -> DMZ/Edge (semi-trusted) -> Internal (trusted). Identify where data crosses boundaries.

**3. Enumerate Attack Vectors**

| Entry Point | Attack Type | Risk |
|-------------|-------------|------|
| User input | Injection (SQL, XSS, Command) | HIGH |
| File upload | Malicious files, path traversal | HIGH |
| Authentication | Credential stuffing, brute force | HIGH |
| API endpoints | IDOR, rate limiting bypass | MEDIUM |
| Dependencies | Known CVEs, supply chain | MEDIUM |
| Configuration | Secrets exposure, misconfig | HIGH |

### Phase 2: Comprehensive Scanning

Run ALL security scans (set severity to LOW for audits):

```bash
# Environment tuning for audits
export SEC_SCAN_SEVERITY=LOW
export SEC_SCAN_VERBOSE=1
export SEC_SCAN_MAX_ISSUES=100

# Full scan suite
sec-scan all ./...

# Individual scans for detailed output:
sec-scan govulncheck ./...    # Go dependency vulnerabilities
sec-scan gosec ./...          # Go security linter (SAST)
sec-scan staticcheck ./...    # Static analysis
sec-scan semgrep .            # Cross-language SAST
sec-scan trivy-fs .           # Filesystem/dependency scan
sec-scan trivy-secret .       # CRITICAL: Always run secret detection
sec-scan trivy-config ./k8s/  # K8s/Docker misconfigurations
sec-scan trivy-image <image>  # Container image vulnerabilities
```

> **Windows**: Use `sec-scan.ps1` instead of `sec-scan`. Environment variables use `$env:VAR="value"` syntax.

### Phase 3: Manual Code Review

Scanners miss business logic flaws. Load [references/owasp-checklist.md](references/owasp-checklist.md) for the full manual review checklist covering authentication, authorization, data handling, and more.

### Phase 4: Infrastructure Security

Load domain skills for infrastructure review:
- `skills/kubernetes-patterns/SKILL.md` -- K8s security
- `skills/terraform-patterns/SKILL.md` -- IaC security

## Workflows

| Trigger | Workflow |
|---------|----------|
| "security audit", "pre-production security review" | [Workflows/SecurityAudit.md](Workflows/SecurityAudit.md) |

## References

Load on-demand based on task:

| Task | Reference |
|------|-----------|
| OWASP Top 10 checklist, manual review items | [references/owasp-checklist.md](references/owasp-checklist.md) |
| Audit report template (copy-paste) | [references/report-template.md](references/report-template.md) |
| Security testing patterns (what to test) | [references/testing.md](references/testing.md) |
