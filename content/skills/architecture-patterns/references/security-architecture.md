# Security Architecture Patterns

Patterns for designing security into architecture. For compliance framework mapping (CMMC, NIST, OWASP, MITRE), see [security-frameworks.md](security-frameworks.md). For security auditing and threat modeling, collaborate with **senior-security-engineer**.

## Authentication Pattern Selection

| Scenario | Pattern | Notes |
|----------|---------|-------|
| User-facing web/mobile apps | OIDC (via OAuth2) | Delegated identity, supports MFA, SSO |
| Service-to-service (internal) | mTLS | Strongest machine identity, no shared secrets |
| Service-to-service (cross-org) | OAuth2 client credentials | When mTLS infra is impractical across orgs |
| Legacy/SOAP integration | SAML 2.0 | Enterprise SSO federation |
| Low-sensitivity, high-volume | API keys + rate limiting | Identification only, not authentication |
| Admin/CLI tools | OIDC device flow | No browser on the device |

- Rule: mTLS for service-to-service, OIDC for user-facing, API keys only for low-sensitivity with mandatory rotation.
- Gotcha: API keys are **not authentication** -- they identify the caller, not prove identity. Never use API keys as the sole control for sensitive operations.
- Gotcha: OAuth2 implicit flow is deprecated. Use authorization code flow with PKCE for all public clients (SPAs, mobile).

## Authorization Patterns

| Model | When to Use | Limitation |
|-------|-------------|------------|
| **RBAC** (Role-Based) | <20 roles, clear role hierarchy | Role explosion when permissions become fine-grained |
| **ABAC** (Attribute-Based) | Complex policies, multiple dimensions (time, location, classification) | Policy authoring/debugging complexity |
| **ReBAC** (Relationship-Based) | Relationship-driven access (user owns doc, org member) | Requires relationship graph (e.g., Zanzibar/SpiceDB) |

- Rule: Start with RBAC. Move to ABAC when role explosion occurs (>20 roles). Use ReBAC when access depends on resource relationships.
- Gotcha: RBAC + resource ownership checks covers 80% of cases. Don't jump to ABAC/ReBAC prematurely.

### Policy Decision Point (PDP) Deployment

| Approach | Pros | Cons |
|----------|------|------|
| PDP as a service (OPA, Cedar) | Centralized policy, audit trail, hot-reload | Network latency per decision, availability dependency |
| Embedded library | Zero network latency, no external dependency | Policy updates require redeploy, harder to audit |
| Sidecar (OPA + Envoy) | No app changes, centralized policy | Sidecar resource overhead, debugging indirection |

- Rule: Use PDP-as-service or sidecar for microservices (centralized policy management). Use embedded library for monoliths or latency-critical paths.

## Secrets Management Strategy

| Solution | Best For |
|----------|----------|
| HashiCorp Vault | Multi-cloud, dynamic secrets, PKI, encryption-as-a-service |
| AWS Secrets Manager | AWS-native workloads, RDS credential rotation |
| Azure Key Vault | Azure-native workloads, managed HSM |
| External Secrets Operator | K8s workloads syncing from any cloud secrets manager |

- Rule: Never store secrets in code, config files, or environment variables at rest. Use a secrets manager with automatic rotation.
- Rule: For K8s workloads, use External Secrets Operator to sync from cloud secrets manager into K8s secrets. See `skills/kubernetes-patterns/references/external-secrets.md`.
- Rule: For workload-to-workload access, prefer workload identity federation over shared secrets. See `skills/aembit-knowledge/SKILL.md` for the Aembit pattern.
- Gotcha: Environment variables are readable by any process in the container and appear in debug dumps, crash reports, and `docker inspect`. They are not a safe secrets transport mechanism.

## Zero Trust Architecture

- Rule: Never trust network location. Verify every request: identity + device posture + context. Authenticate at the application layer, not the network perimeter.
- Components: Identity-aware proxy, microsegmentation, continuous verification, least-privilege access, encrypted communications everywhere.
- Pattern: BeyondCorp model -- replace VPN with identity-aware proxy (e.g., IAP, Cloudflare Access, Tailscale). Every request is authenticated and authorized regardless of network origin.
- Gotcha: Zero trust is a model, not a product. Buying a "zero trust" appliance without redesigning authentication and authorization flows achieves nothing.

## Data Protection Patterns

### Classification Levels

| Level | Examples | Required Controls |
|-------|----------|-------------------|
| Public | Marketing content, docs | Integrity checks |
| Internal | Internal wikis, non-sensitive configs | Encrypt in transit (TLS) |
| Confidential | PII, financial data, credentials | Encrypt at rest + in transit, access logging, field-level encryption |
| Restricted | PHI, payment card data, CUI | All above + dedicated KMS, data residency, audit trail, DLP |

- Rule: Encrypt at rest (AES-256) and in transit (TLS 1.3). For Confidential and above, add field-level encryption for sensitive columns/fields.
- Pattern: Envelope encryption for large datasets -- encrypt data with a Data Encryption Key (DEK), encrypt the DEK with a Key Encryption Key (KEK) stored in KMS. Enables key rotation without re-encrypting all data.
- Gotcha: Never roll your own crypto. Use cloud KMS or established libraries (libsodium, AWS Encryption SDK).
- Gotcha: TLS termination at the load balancer means traffic is unencrypted between LB and app. For Confidential+ data, use end-to-end TLS or mTLS between all hops.

## Security Architecture Anti-Patterns

| Anti-Pattern | Why It Fails | Alternative |
|--------------|-------------|-------------|
| Perimeter-only security | One breach = full access | Defense in depth, zero trust |
| Shared service accounts across environments | Blast radius spans dev/staging/prod | Per-environment service identity |
| Secrets in env vars without a manager | Readable by any process, appears in logs | Secrets manager with injection at runtime |
| Symmetric keys for service-to-service auth | Key distribution problem, no non-repudiation | mTLS or workload identity federation |
| Security as a bolt-on phase | Expensive rework, gaps in design | Threat model during design, security ADRs |
| Overly permissive CORS (`*`) | Enables cross-origin attacks | Explicit origin allowlist |
