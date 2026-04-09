# Use Case Template Guide

This guide defines what a complete, well-documented use case ticket looks like. Use it when:
- **Creating** use case stories from customer content (via `/analyze-customer-input` or `/poc-scoper`)
- **Reviewing** existing use case tickets for completeness and quality
- **Assessing** whether the SE, engineering, and customer all have a shared understanding of what's being proven

A use case story should give anyone picking up the ticket a clear picture of: what connects to what, how it works today, how it should work with Aembit, and what success looks like.

---

## 1. Overview

### What is the customer trying to connect, and why?
Describe the access pattern in one paragraph. What workload talks to what resource, and what problem does this solve? Include enough context that someone unfamiliar with the customer can understand the use case without reading the epic.

---

## 2. Source Workload

### What is the workload that needs a credential?
- **Type** — Application, microservice, agent, pipeline, script, Lambda function, etc.
- **Runs on** — Where does it execute? (AKS, EKS, EC2, Lambda, on-prem VM, etc.)
- **Language/runtime** — What language or framework? (Java, Python, Node.js, Go, etc.) This affects SDK/integration approach.
- **Architecture details** — Is it a single service or part of a multi-tier architecture? How many instances/replicas? What scale?

---

## 3. Target Resource

### What does the source workload need to talk to?
- **Type** — API, database, SaaS service, peer microservice, registry, mainframe, etc.
- **Protocol** — REST, gRPC, mTLS, database protocol, etc.
- **Details** — What makes this target specific? Is it internal or external? Does it require specific credential formats?

---

## 4. Current Authentication

### How does the customer authenticate this access pattern today?
Describe the current state — this is what Aembit replaces or improves. Include:
- What credential type is used today (OIDC tokens, static API keys, certificates, service account tokens, etc.)
- How credentials are managed (hardcoded, secrets manager, manual rotation, etc.)
- What's broken or risky about the current approach
- Any authorization layers involved (OPA, API gateways, RBAC, etc.)

---

## 5. Confirmed Delivery Model

### How will Aembit deliver credentials for this use case?
This section should be confirmed with the customer, not assumed. Include:
- **Primary method** — Proxy, CLI sidecar, REST API, K8s operator, etc.
- **Secondary/fallback method** — If applicable
- **Explicitly NOT using** — Methods ruled out and why (e.g., "NOT proxy — IP tables conflict with Istio")
- **Caching/storage** — Where does the credential live? (emptyDir tmpfs, K8s Secret, in-memory in app code, etc.)

If the delivery model hasn't been confirmed yet, state that explicitly: "Delivery model TBD — to be determined during POC."

---

## 6. Business Value

### Why does this use case matter to the customer?
Use the customer's own words wherever possible. Include verbatim quotes that articulate:
- What pain point this solves
- What risk it mitigates
- What capability it enables

Do not paraphrase when a direct quote is available.

---

## 7. Environment

### What is the customer's technical environment for this use case?
- **Cloud** — AWS, Azure, GCP, on-prem, hybrid
- **Kubernetes** — Managed (AKS, EKS, GKE) or self-managed? Version? Any managed add-ons (Istio, CSI driver)?
- **IDP** — Okta, Entra, KeyCloak, custom OIDC, etc.
- **Service mesh** — Istio, Linkerd, App Mesh, Consul, none?
- **Secrets/CA** — HCP Vault, AWS Secrets Manager, cert-manager, custom CA, etc.
- **Other** — Relevant frameworks, on-prem systems, regions, compliance constraints
- **Scale** — Number of clusters, workloads, pods, requests/sec — whatever is relevant

---

## 8. Success Criteria

### What does "success" look like for this use case?
List concrete, testable criteria the customer would use to evaluate whether the use case works. Good criteria are:
- **Specific** — "JWT-SVID issued with custom claims" not "identity works"
- **Testable** — Can be verified in a demo or POC
- **Customer-stated** — From the customer's own requirements, not assumed

Include both mandatory and optional criteria if the customer distinguished between them.

---

## 9. POC Status (if applicable)

### Where does this use case stand in the POC?
- Not started / In progress / Complete
- What has been validated so far?
- What remains to be tested?
- Any blockers or open questions?

Update this section as the POC progresses.

---

## 10. Evidence

### Verbatim quotes supporting this use case
Every use case should have at least one verbatim customer quote. For each quote include:
- The exact words spoken or written
- Speaker name and affiliation
- Source (call date, document name, email)

Multiple quotes from different calls showing how the requirement evolved are ideal.

---

## Completeness Checklist

When reviewing a use case ticket, check whether these questions can be answered from the ticket alone:

| Question | Where it should be answered |
|----------|---------------------------|
| What connects to what? | Overview + Source/Target |
| How does it work today? | Current Authentication |
| How will Aembit deliver credentials? | Confirmed Delivery Model |
| Why does the customer care? | Business Value — with quotes |
| What's their tech stack? | Environment |
| How do we know it works? | Success Criteria — testable items |
| Is there real customer evidence? | Evidence — verbatim quotes |
| What's the current POC status? | POC Status |

---

## Markdown Convention

When writing a UC story description (for `createJiraIssue` or `editJiraIssue` via MCP), generate Markdown following this structure. Pass `contentFormat: "markdown"` on the MCP call. The story `summary` field carries the use case name.

```markdown
## Overview
{What the customer wants to connect and why}

## Source Workload
- **Type**: {application, agent, pipeline, script, etc.}
- **Runs on**: {EC2, AKS, Lambda, etc.}
- **Language/Runtime**: {Python, Java, Go, etc.}
- **Details**: {additional architecture context}

## Target Resource
- **Type**: {API, database, SaaS, registry, etc.}
- **Protocol**: {REST, gRPC, database protocol, etc.}
- **Details**: {specific target context}

## Current Authentication
{How they authenticate today and what's broken about it}

## Delivery Model
- **Primary**: {Proxy, CLI sidecar, REST API, etc.}
- **Ruled out**: {methods excluded and why}
- **Status**: {Confirmed / TBD}

## Environment
- **Cloud**: {AWS, Azure, GCP, hybrid}
- **Kubernetes**: {AKS 1.28, EKS, etc.}
- **IDP**: {Okta, Entra, etc.}
- **Service mesh**: {Istio, none, etc.}
- **Scale**: {clusters, pods, req/sec}

## Business Value
{In the customer's own words, with verbatim quotes}

## Success Criteria
- [ ] {specific testable criterion}
- [ ] {specific testable criterion}

## POC Status
- **Status**: {Not started / In progress / Complete}
- **Validated**: {what has been tested so far, if any}
- **Remaining**: {what still needs testing}
- **Blockers**: {open questions or blockers, if any}

## Evidence
> "{verbatim quote}" -- {Speaker} ({Affiliation}), {Source}
```

All sections are optional except Overview. Empty sections indicate gaps the SE can fill during scoping.

---

## Enrichment Checklist

When transcript or call evidence is available, check whether the ticket captures the full depth of what was discussed. This goes beyond the Completeness Checklist — a ticket can be "complete" (all sections present) but still missing specific details from the transcripts.

| Question | Source |
|----------|--------|
| Is the delivery model confirmed or explicitly TBD? | Transcript evidence |
| Are ruled-out approaches documented with reasons? | Transcript evidence |
| Are environment details specific (versions, regions, scale)? | Transcript evidence |
| Are success criteria testable and customer-stated? | Transcript evidence |
| Is current_auth specific about what's broken, not just what exists? | Transcript evidence |
| Are all relevant verbatim quotes included? | Transcript evidence |
| Are architecture specifics captured (replicas, tiers, protocols)? | Transcript evidence |
| Does the ticket reflect the most recent discussion of this use case? | Latest call |
