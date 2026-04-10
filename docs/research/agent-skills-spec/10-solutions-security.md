# Solutions: Security

## Spec-Level Proposals

### Capabilities/Permissions Field (agentskills#181)

**Proposer:** orlyjamie

```yaml
---
name: git-autopush
capabilities:
  - shell
  - network
---
```

Four base categories: `shell`, `filesystem`, `network`, `browser`. Unknown values trigger warnings but don't block. Optional modifier proposed by yarmoluk: `- network: optional`.

### Required Permissions (agentskills#251)

**Proposer:** luoxiner. Finer-grained than #181:
- `command`: executables like `curl`
- `tool`: dependencies like `ffmpeg`
- `filesystem`: specific paths like `/tmp/skill-data/`
- `api/network`: access requirements
- Distinguishes required vs optional

### Signature Block RFC (agentskills#252)

**Proposer:** fanqi1909

```yaml
signature:
  algorithm: ed25519-sha256
  signer: skills.sh
  kid: key-rotation-id
  content_hash: sha256-of-body
  signed_at: 2026-03-15T00:00:00Z
  sig: base64-encoded-signature
```

Federated trust: signers host public keys at `https://{signer}/.well-known/skills-pubkey` (JWKS format).

**Maintainer pushback:** Belongs at distribution layer, not SKILL.md. Concerns about context bloat and incomplete coverage (signs body only, not assets/scripts).

### Skill Provenance (agentskills#198)

**Proposer:** snapsynapse. Proposes `MANIFEST.yaml`:
- File inventory with roles and integrity hashes
- Author-side provenance (bundle identity)
- Consumer-side provenance (registry/install tracking)
- Optional `source_repo`, `source_commit`, `fork_date`

### Credentials Field (agentskills#173)

**Proposer:** iwasrobbed

```yaml
credentials:
  - name: OPENAI_API_KEY
    description: OpenAI API key for image generation
  - name: SLACK_WEBHOOK_URL
    description: Slack webhook for posting
    required: false
```

Declarative, implementation-agnostic. Runtimes decide resolution strategy. `required` defaults to `true`.

### Security Harness (agentskills#157)

**Proposer:** yu-iskw. Multi-layer defense:
- Security intent declarations
- Policy hooks for allow/deny pre-execution
- Standardized audit event schemas
- Two options: (A) structured frontmatter, or (B) separate `policy/` directory with OPA/CEL

## Tools and Frameworks

### OWASP Agentic Skills Top 10

| ID | Risk | Key Mitigations |
|----|------|-----------------|
| AST01 | Malicious Skills | Merkle root signing, automated scanning, cryptographic publisher identity |
| AST02 | Supply Chain Compromise | Transparency logs, provenance tracking, explicit consent |
| AST03 | Over-Privileged Skills | Least-privilege manifests, domain allowlists |
| AST04 | Insecure Metadata | Static analysis, typosquatting detection, publisher verification |
| AST05 | Unsafe Deserialization | Safe parsers, schema validation, sandboxed deserialization |
| AST06 | Weak Isolation | Containerized execution default, explicit host-mode opt-in |
| AST07 | Update Drift | Pin to immutable hashes, hash verification chain |
| AST08 | Poor Scanning | Behavioral + pattern scanning |
| AST09 | No Governance | Skill inventories, approval workflows, audit logging |
| AST10 | Cross-Platform Reuse | Universal format standard, cross-registry threat intel |

### Snyk agent-scan

CLI scanner for agent configs (MCP + skills). Three phases: Discovery → Connection → Validation.

Detects: prompt injection, tool poisoning, tool shadowing, toxic data flows, malware, credential flaws, hardcoded secrets.

Usage: `uvx snyk-agent-scan@latest` with `SNYK_TOKEN`.

### Grith.ai (Zero-Trust Runtime)

Wraps CLI tools with `grith exec`. Intercepts every syscall at OS level. 17 independent filters across three phases:
- Phase 1 (<1ms): 6 static checks
- Phase 2 (~3ms): 5 pattern matching
- Phase 3 (~5ms): 6 context analysis

Each syscall gets composite score → auto-allow (80-90%), quarantine (5-15%), auto-deny (1-5%).

**Key design:** "Actions are evaluated independently of what the model thinks is safe" — removes LLM from security boundary.

### SafeDep (vet tool)

CLI with `--agent-skill` mode. Behavioral malware detection, vulnerability assessment contextualized to actual code usage, CEL policy enforcement, multi-ecosystem dependency scanning.

### Cloudflare Discovery RFC Security Provisions

- SHA-256 content digests mandatory for all artifacts
- Archive safety: path traversal prevention, symlink/hardlink restrictions, decompression bomb limits
- "Clients SHALL NOT execute scripts by default" (normative)
- Gaps: no cryptographic signing, no key infrastructure, no publisher identity

### Embrace The Red: Unicode Tag Attack + Defenses

**Attack:** Unicode Tag codepoints (U+E0000-U+E007F) invisible in editors/GitHub but interpreted by LLMs as instructions.

**Defenses:**
1. ASCII Smuggler tool (decodes hidden tags)
2. Aid scanner (flags consecutive Unicode Tag runs)
3. Registry-level scanning
4. Claude Code has some built-in detection

## Summary: Security Control Categories

| Category | Proposals/Tools |
|----------|-----------------|
| Capability declarations | #181 capabilities, #251 required_permissions |
| Signing & provenance | #252 signature block, #198 MANIFEST.yaml, Cloudflare digests |
| Credential management | #173 credentials field |
| Policy enforcement | #157 Security Harness (OPA/CEL), SafeDep CEL |
| Runtime sandboxing | Grith.ai syscall interception, OWASP AST06 |
| Static scanning | Snyk agent-scan, SafeDep vet, OWASP AST08 |
| Hidden content detection | ASCII Smuggler, Aid scanner |
| Governance | OWASP AST09, vskill tiered verification |

**The spec itself contains zero normative security language.** Cloudflare RFC is the closest to having enforceable provisions.
