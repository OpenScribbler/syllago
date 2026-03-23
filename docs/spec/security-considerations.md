# Security Considerations

This document describes the threat model, attack surface, and security recommendations for implementations of the [Hook Interchange Format Specification](hooks-v1.md).

---

## 1. Threat Model

Hooks execute arbitrary code on a developer's machine with the privileges of the AI coding tool. The primary threat is **remote code execution (RCE) via malicious hooks** distributed through registries, repositories, or shared configurations.

### 1.1 Threat Actors

| Actor | Capability | Goal |
|-------|-----------|------|
| Malicious hook author | Publishes hooks to public registries | Execute arbitrary code on victim machines |
| Compromised registry | Serves modified hook packages | Supply chain attack against downstream users |
| Repository contributor | Commits hooks to shared repositories | Lateral movement within a team or organization |
| Man-in-the-middle | Intercepts hook distribution | Tamper with hook scripts during transit |

### 1.2 Historical Context

In February 2026, hooks in AI coding tools were identified as an RCE vector (CVE details vary by provider). This demonstrated that hook distribution without provenance controls is an active, exploited attack surface.

---

## 2. Attack Surface

### 2.1 Scripts Are the Attack Surface, Not Manifests

The canonical hook manifest (`hook.json`) is a JSON document that declares event bindings, matchers, and handler configuration. **The manifest itself is not dangerous.** The danger lies in the scripts and commands that the manifest references.

A manifest that declares `"command": "./check.sh"` is metadata. The file `check.sh` is the executable payload. Security analysis MUST focus on script content, not manifest structure.

### 2.2 Provider Data Validation

The `provider_data` field is opaque to the canonical format. Each provider adapter MUST validate its own section during encode. Specifically:

- Adapters MUST NOT blindly copy `provider_data` values into provider-native fields that have security implications (e.g., permission overrides, URL endpoints, file paths outside the project directory).
- Adapters SHOULD validate that `provider_data` values conform to the expected schema for the target provider.
- Adapters MUST NOT execute or evaluate values from `provider_data` without validation.

A malicious manifest could include `provider_data` designed to exploit a specific adapter's parsing logic. Adapters MUST treat `provider_data` as untrusted input.

### 2.3 Input Rewrite as a Privileged Capability

The `input_rewrite` capability allows a hook to modify tool arguments before execution. This is a **safety-critical** operation:

- A hook that sanitizes shell commands (removing `rm -rf /`, for example) provides security value **only if** the rewrite is actually applied.
- If the target provider does not support input rewriting and the adapter silently drops the rewrite, the tool executes with the original (unsanitized) arguments. The user believes they are protected when they are not.

For this reason, the specification mandates `block` as the default degradation strategy for `input_rewrite`. Implementations MUST NOT silently degrade input rewriting to a no-op.

### 2.4 HTTP Handlers

Hooks with `type: "http"` send request data to external endpoints. This creates additional attack vectors:

- **Data exfiltration:** A malicious hook can POST session data, file contents, or conversation transcripts to an attacker-controlled server.
- **SSRF (Server-Side Request Forgery):** If the hook runs in an environment with access to internal networks, the URL endpoint could target internal services.
- **Credential leakage:** HTTP handlers that interpolate environment variables into headers (e.g., `Authorization: Bearer $TOKEN`) may leak secrets if the URL is attacker-controlled.

Implementations SHOULD:
- Display the target URL to the user before executing HTTP hooks.
- Restrict HTTP handler URLs to HTTPS.
- Warn when environment variables are interpolated into HTTP headers.

### 2.5 LLM-Evaluated Hooks

Hooks with `type: "prompt"` or `type: "agent"` delegate logic to an LLM. These hooks:

- Consume API credits or tokens, creating a denial-of-wallet attack vector.
- May produce non-deterministic results, making security auditing difficult.
- Can be influenced by prompt injection if the hook's prompt incorporates untrusted input (e.g., file contents, tool arguments).

Implementations SHOULD clearly indicate when a hook uses LLM evaluation and the associated cost implications.

### 2.6 Generated Bridge Plugins

When converting hooks to providers that use a programmatic plugin model (e.g., OpenCode), the adapter generates code (typically TypeScript). Generated code inherits the trust level of the source hook but adds a new attack surface:

- **Code injection:** If the source hook's command string or arguments are interpolated into generated code without escaping, an attacker can inject arbitrary code.
- **Dependency confusion:** Generated plugins that import packages could be targeted by dependency confusion attacks.

Adapters that generate code MUST:
- Escape all interpolated values.
- Not import external dependencies unless explicitly required.
- Clearly mark generated code as machine-generated.

---

## 3. Content Integrity

### 3.1 Per-File Hashes

Hook distribution packages SHOULD include SHA-256 hashes for every file in the hook directory. This enables integrity verification before execution:

```json
{
  "content_hashes": {
    "check.sh": "sha256:a1b2c3d4e5f6...",
    "check.ps1": "sha256:f6e5d4c3b2a1...",
    "lib/helpers.sh": "sha256:1a2b3c4d5e6f..."
  }
}
```

Implementations SHOULD verify file hashes before hook execution when hashes are available.

### 3.2 Signatures

The specification defines an optional `signatures` field for cryptographic signatures. The specification does not mandate a specific signing mechanism. Implementations MAY support:

- Sigstore/cosign for keyless signing with identity verification
- GPG/PGP signatures for traditional key-based signing
- Other mechanisms appropriate to their ecosystem

### 3.3 Author Metadata

The specification defines optional author identity fields:

```json
{
  "author": {
    "name": "Jane Smith",
    "email": "jane@example.com",
    "url": "https://github.com/janesmith"
  }
}
```

Author metadata is self-reported and MUST NOT be treated as verified identity without independent verification (e.g., Sigstore identity binding, GPG key verification).

---

## 4. Recommendations for Implementations

### 4.1 Installation Prompts

Implementations SHOULD prompt the user before installing hooks, displaying:
- The hook's event bindings and blocking behavior
- The commands or scripts that will be executed
- Any capabilities that grant elevated privileges (especially `input_rewrite`)
- The source of the hook (registry, repository, local file)

### 4.2 Script Scanning

Implementations SHOULD provide a mechanism for integrating external security scanning tools. A pluggable scanner interface allows organizations to apply their own SAST/DAST tools to hook scripts before installation.

The specification does not mandate specific scanning tools or rules. Pattern-based scanning of manifest `command` fields catches trivial threats; scanning actual script file contents is necessary for meaningful security analysis.

### 4.3 Execution Sandboxing

Implementations SHOULD consider executing hooks with reduced privileges where the operating system supports it. Hooks generally need read access to the project directory and network access for HTTP handlers, but rarely need write access outside the project or access to sensitive system paths.

### 4.4 Audit Logging

Implementations SHOULD log hook executions with sufficient detail for security auditing:
- Timestamp
- Hook identity (name, source)
- Event that triggered the hook
- Exit code
- Whether the hook blocked an action

### 4.5 Timeout Enforcement

Implementations MUST enforce timeouts on hook execution. A hook that hangs indefinitely blocks the AI coding tool's operation. The recommended default timeout is 30 seconds. Implementations SHOULD terminate hook processes that exceed their timeout and treat the result as a non-blocking error (exit code 1).
