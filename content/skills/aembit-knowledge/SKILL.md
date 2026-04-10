---
name: aembit-knowledge
description: Domain knowledge for Aembit Workload IAM platform - concepts, APIs, and integration patterns. Use when working with Aembit SDK, Terraform provider, or workload identity federation.
---

# Aembit Knowledge

Domain knowledge for Aembit Workload IAM platform.

## CRITICAL: No Fabrication Rule

Aembit is a specialized product NOT fully in Claude's training data. DO NOT hallucinate capabilities.

- If not in loaded references: "I don't see this in my Aembit references. Let me check live docs."
- If partially documented: "The docs mention X but don't cover [detail]. Can you clarify?"
- If uncertain: check live docs via WebFetch before answering

## Quick Reference

| Concept | Description |
|---------|-------------|
| **Workload IAM** | Identity and access management for non-human identities (workloads, not users) |
| **Client Workload** | Application/service initiating an access request |
| **Server Workload** | Target service/API/database being accessed |
| **Access Policy** | Links client + server + trust + credential + conditions |
| **Trust Provider** | Verifies workload identity via runtime attestation |
| **Credential Provider** | Obtains credentials after access is authorized |
| **Access Condition** | Dynamic constraint (time, geo, security posture) |
| **Aembit Cloud** | SaaS control plane — policy evaluation, identity verification, credential brokering |
| **Aembit Edge** | Data plane — Agent Controller + Agent Proxy + Agent Injector |

**Trust Provider Types:** AWS, Azure, GCP, Kubernetes, GitHub Actions, GitLab CI, Kerberos (via Agent Controller)

**Credential Provider Types:** AembitAccessToken, ApiKey, AwsSecretsManager, AwsStsFederation, AzureEntraFederation, AzureKeyVault, GcpWorkloadIdentityFederation, GitLabManagedAccount, JWTToken, OAuth2AuthorizationCode, OAuth2ClientCredentials, UsernamePassword, VaultClientToken

**Access Condition Types:** Time-based, Geographic (GeoIP), Security Posture (Wiz, CrowdStrike)

## References

Load on-demand based on task:

| Task | Load |
|------|------|
| Understanding Aembit architecture and concepts | `references/concepts.md` |
| Cloud API development (endpoints + schemas) | `references/cloud-api.md` |
| Edge API development (endpoints + schemas) | `references/edge-api.md` |
| Real-world integration patterns and gotchas | `references/real-world-patterns.md` |
| Aembit Terraform provider | `skills/terraform-patterns/references/providers/aembit.md` |

**Loading order:**
1. Start with `concepts.md` for context if unfamiliar with Aembit
2. Load specific API reference based on task
3. Check `real-world-patterns.md` for gotchas

## Live Documentation

When references don't cover the question, fetch live docs:

| Content | URL |
|---------|-----|
| Full concepts (LLM-optimized) | `https://docs.aembit.io/llms-small.txt` |
| Cloud API endpoints | `https://docs.aembit.io/_llms-txt/api-cloud-endpoints.txt` |
| Cloud API schemas | `https://docs.aembit.io/_llms-txt/api-cloud-schemas.txt` |
| Edge API endpoints | `https://docs.aembit.io/_llms-txt/api-edge-endpoints.txt` |
| Edge API schemas | `https://docs.aembit.io/_llms-txt/api-edge-schemas.txt` |

## Incremental Training

Add to `references/real-world-patterns.md` when:
- User shares a working integration pattern
- You discover a gotcha or edge case
- A workaround or best practice emerges

Ask first: "This is useful knowledge. Should I add it to the Aembit patterns reference?"
