# Extraction Depth Guide

This reference defines what "deep extraction" means when analyzing customer transcripts and documents. Load this before beginning extraction in Phase 3 of the AnalyzeSession workflow.

**The goal of extraction is to capture every specific technical detail the customer mentioned — not to summarize topics discussed.**

---

## Extraction Depth Ladder

Every extracted item falls at one of three depth levels. Your target is always **Operational**.

| Level | What it captures | Example |
|-------|-----------------|---------|
| **Topic** (too shallow) | That a subject was discussed | "CrowdStrike integration was discussed" |
| **Detail** (insufficient) | The general nature of the requirement | "Customer needs CrowdStrike integration for endpoint security monitoring" |
| **Operational** (target) | Specific technical constraints, behaviors, versions, protocols, failure modes | "Customer uses CrowdStrike Falcon sensor with kernel-level hooks; iptables-based redirect conflicts with Falcon's network interception — need userspace proxy or eBPF-based approach instead" |

### Self-Check

Before proceeding past extraction, review each item against this ladder. If any item reads like a Topic or Detail summary, go back to the source and extract the operational specifics.

---

## Good vs Bad Extraction Examples

### Example 1: Integration Constraint

**Bad (Topic level):**
> Title: "MCP gateway integration needed"
> Problem: Customer wants to connect their AI tools through Aembit's MCP gateway.

**Good (Operational level):**
> Title: "MCP gateway blocked — Pangea only supports stdio transport"
> Problem: Pangea's MCP server implementation only supports stdio transport, not remote/SSE. Aembit's MCP gateway requires SSE for remote connections. Customer cannot route Pangea tool calls through the gateway without a transport adapter or Pangea adding SSE support.
> Evidence: "Pangea only does stdio — there's no way to point it at a remote endpoint like your gateway" — J. Smith (Acme Engineering), Call 2024-12-15

### Example 2: Authentication Flow

**Bad (Topic level):**
> Title: "Token exchange simplification"
> Problem: Customer wants simpler authentication.

**Good (Operational level):**
> Title: "Two-step auth+credential API adds 200ms per request at scale"
> Problem: Current flow requires two sequential API calls — first to /auth for an access token, then to /credential with that token to get the target credential. At 500 req/sec across 12 microservices, the extra round-trip adds ~200ms latency and doubles the load on the auth service. Customer is asking for a single-step endpoint that returns the target credential directly.
> Evidence: "Every request hits auth then credential — at our scale that's an extra 200 milliseconds we can't afford" — R. Chen (Platform Lead), Call 2025-01-08

### Example 3: Environment Detail

**Bad (Topic level):**
> Environment: "Azure, Kubernetes"

**Good (Operational level):**
> Environment: "AKS 1.28 in East US 2, 3 clusters (dev/staging/prod), ~400 pods in prod. Running Istio 1.20 with strict mTLS — any sidecar injection must be compatible with Istio's iptables rules. Using Azure Key Vault with CSI driver for current secrets, Entra ID (formerly Azure AD) as IDP with workload identity federation enabled."

### Example 4: Success Criteria

**Bad (Topic level):**
> Success criteria: "Identity works correctly"

**Good (Operational level):**
> Success criteria: "JWT-SVID issued within 50ms containing custom claims: service account name, namespace, and cluster identifier. Token must be accepted by their existing OPA policy that checks the 'svc_account' claim against an allowlist. Must work with their init container pattern — credential available before the main container starts."

---

## Extraction Triggers

These patterns in transcripts signal extractable technical detail. When you encounter them, follow the thread to capture the full operational picture:

- **Version numbers and specific products** — "we're on AKS 1.28", "Istio 1.20", "Vault Enterprise 1.15"
- **Protocol and transport details** — "stdio only", "gRPC not REST", "mTLS required"
- **Error conditions and failure modes** — "it fails when...", "we get a 403 because...", "the token expires before..."
- **Quantitative constraints** — "500 requests per second", "200ms latency budget", "12 microservices", "400 pods"
- **Architecture decisions and tradeoffs** — "we chose X because Y", "we can't use Z because..."
- **"We tried X but..."** — signals a constraint or incompatibility worth capturing
- **"The problem is..."** — signals the specific pain point, not just the topic
- **Workaround details** — "right now we manually...", "we have a cron job that...", "our workaround is..."
- **Explicit exclusions** — "we don't want...", "that won't work because...", "not the proxy approach"
- **Timeline and milestone specifics** — "before the March release", "our SOC2 audit is in Q2"

---

## Multi-Quote Threading Rule

Technical topics in transcripts often span multiple speaker turns — a customer states a problem, an SE asks a clarifying question, the customer adds detail, another customer participant adds context. A single quote captures only one piece.

**Rule:** When a technical topic spans multiple turns, follow the entire thread and consolidate all specific details into a single extraction. Use the most detailed statement as the primary evidence quote, and include additional quotes that add distinct technical information.

**Example of threading:**

Turn 1 (Customer): "We need to get credentials into our Lambda functions"
Turn 3 (Customer): "They're Python 3.11, running in us-east-1"
Turn 5 (SE): "Are you using layers or container images?"
Turn 6 (Customer): "Container images — we build on the AWS base image and add our code"
Turn 8 (Customer): "The tricky part is we have 30-second cold starts and the credential needs to be ready before the handler runs"

**Bad extraction:** Captures only Turn 1 — "We need to get credentials into our Lambda functions"

**Good extraction:** Consolidates all turns — "Python 3.11 Lambda functions in us-east-1 using container images (AWS base image). 30-second cold starts — credential must be available before the handler executes, ruling out lazy initialization. Need init-phase credential fetch that completes within the cold start window."

---

## When to Stop Extracting

Stop only when you have captured:
1. Every specific technical constraint mentioned (versions, protocols, limits)
2. Every architecture detail relevant to the access pattern or feature request
3. Every failure mode or incompatibility described
4. Every quantitative detail (scale, latency, count)
5. The customer's own words for why this matters (business value quotes)

If you find yourself writing a sentence that could apply to any customer ("they want better security"), you haven't extracted deeply enough. Go back to the source.
