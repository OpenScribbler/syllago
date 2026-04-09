# OWASP Top 10 Deep Checklist

Comprehensive security checklist based on OWASP Top 10. Load during thorough security audits and manual code review phases.

## A01: Broken Access Control

- [ ] Deny by default access policy
- [ ] Record ownership verified on access
- [ ] JWT tokens validated (signature, expiry, audience)
- [ ] CORS configuration restrictive
- [ ] Directory listing disabled
- [ ] Rate limiting on sensitive endpoints

## A02: Cryptographic Failures

- [ ] No deprecated algorithms (MD5, SHA1, DES, RC4)
- [ ] Proper key management (rotation, secure storage)
- [ ] TLS 1.2+ enforced
- [ ] Certificates valid and not self-signed (prod)
- [ ] Password hashing with bcrypt/argon2 (cost >= 12)

## A03: Injection

- [ ] Parameterized queries everywhere
- [ ] ORM without raw queries on user input
- [ ] No eval/exec with user data
- [ ] Command execution uses arrays, not shell
- [ ] LDAP queries parameterized

## A04: Insecure Design

- [ ] Threat model documented
- [ ] Security requirements defined
- [ ] Secure defaults (opt-in to risky features)
- [ ] Fail securely (deny on error)

## A05: Security Misconfiguration

- [ ] Unnecessary features disabled
- [ ] Default credentials changed
- [ ] Error messages generic
- [ ] Security headers present (CSP, HSTS, X-Frame-Options)
- [ ] Cloud permissions least privilege

## A06: Vulnerable Components

- [ ] Dependency scanning in CI/CD
- [ ] Known CVE remediation process
- [ ] Component inventory maintained
- [ ] Update policy defined

## A07: Auth Failures

- [ ] Strong password requirements
- [ ] Credential stuffing protection
- [ ] Session ID regeneration on auth
- [ ] Secure session storage

## A08: Data Integrity Failures

- [ ] CI/CD pipeline secured
- [ ] Dependency integrity verified (checksums, signing)
- [ ] Deserialization input validated
- [ ] Critical updates signed

## A09: Logging and Monitoring

- [ ] Auth events logged
- [ ] Access control failures logged
- [ ] Server-side validation failures logged
- [ ] No sensitive data in logs
- [ ] Alerting configured

## A10: SSRF

- [ ] URL validation on user-provided URLs
- [ ] Allowlist for outbound connections
- [ ] No raw URL fetch from user input
- [ ] Cloud metadata endpoint blocked

## Severity Classification

| Severity | CVSS | Criteria | SLA |
|----------|------|----------|-----|
| CRITICAL | 9.0-10.0 | RCE, auth bypass, data breach likely | Block deployment |
| HIGH | 7.0-8.9 | Significant vuln, exploitation likely | Fix before merge |
| MEDIUM | 4.0-6.9 | Moderate impact, mitigations exist | Fix within sprint |
| LOW | 0.1-3.9 | Minor issue, hardening recommendation | Track in backlog |
