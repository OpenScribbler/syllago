# Standards Precedents for Cross-Platform Hook Specification

Research into existing standards and specifications analogous to syllago's cross-platform hook system. Focus on design patterns, extensibility models, capability negotiation, and lessons learned.

---

## 1. Git Hooks

The most successful hook system in developer tooling. Git hooks are scripts that run automatically when particular events occur in a repository.

### Design Model

- **Convention-based discovery**: Hooks are executable files in `$GIT_DIR/hooks/` named after the event (e.g., `pre-commit`, `post-merge`). No registration, no manifest — the filename IS the contract.
- **Language-agnostic execution**: Any executable works. Shell, Python, Ruby, compiled binary — Git doesn't care. It just needs the executable bit set.
- **Input contract**: Each hook type has a defined contract for how it receives data — some via command-line arguments, some via stdin, some via environment variables. The contract varies per hook type (no universal pattern).
- **Exit code semantics**: Non-zero exit = abort the operation (for pre-hooks). Post-hooks are fire-and-forget. This binary pass/fail model is simple and effective.
- **Bypass mechanism**: `--no-verify` skips client-side hooks. This escape hatch is important — hooks that can't be bypassed create frustration.

### Extensibility

Git hooks have essentially zero extensibility. The set of hook points is hardcoded in Git's source. Adding a new hook type requires a Git release. There's no mechanism for custom hook types, no namespacing, no metadata beyond the filename.

### What Works

- **Radical simplicity**: The filename-is-the-contract pattern means zero configuration overhead. Drop a file, it runs.
- **Predictable execution model**: Pre-hooks gate operations, post-hooks notify. Everyone understands this.
- **No runtime dependencies**: Hooks don't need a framework, SDK, or registration system.

### What Doesn't Work

- **Distribution is unsolved**: `.git/hooks/` isn't version-controlled. Every team reinvents hook distribution — symlinks, copy scripts, frameworks (Husky, pre-commit, Lefthook). This is Git hooks' single biggest failure.
- **No composition**: Only one script per hook point. Multiple tools wanting `pre-commit` must be orchestrated externally.
- **No capability declaration**: Hooks can't declare what they need or what they provide. There's no metadata about a hook beyond its existence.
- **Environment coupling**: Hooks depend on the local environment (installed tools, PATH, OS). A hook that works on macOS may fail on Linux.

### Lessons for Syllago

- The simplicity of "filename = hook type, exit code = result" is worth preserving.
- Distribution was the unsolved problem that spawned an entire ecosystem of tools. Syllago is inherently solving this.
- The lack of composition (one script per hook point) drove the need for frameworks. Our spec should handle multiple hooks per event natively.
- Capability declaration (what a hook needs to run) is critical and completely absent from Git hooks.

---

## 2. OpenAPI / Swagger

A successful cross-tool specification for describing REST APIs. Relevant for its extensibility model and adoption trajectory.

### Extensibility: The `x-` Prefix

- **Any property starting with `x-` is a vendor extension.** Tools MUST ignore extensions they don't understand. This is the single most important design decision for cross-tool compatibility.
- **Convention over enforcement**: Vendors typically further prefix (e.g., `x-speakeasy-`, `x-amazon-`), but this is convention, not required.
- **Extensions can appear almost anywhere** in the document. The value can be any JSON type.
- **Reserved prefixes**: `x-oai-` and `x-oas-` are reserved for the OpenAPI Initiative itself (added in 3.1.0). This creates a two-tier system: official extensions and vendor extensions.
- **Extensions can graduate**: Widely-used extensions can be proposed for official adoption. This creates a pipeline from experimentation to standardization.

### Versioning

- Uses `major.minor.patch` but notably **does not follow SemVer** (as of 3.1.x). The `major.minor` portion designates the feature set; patches address spec document errors.
- The `openapi` field in every document declares which version it was written for. This is analogous to JSON Schema's `$schema`.

### What Made It Succeed

- **Tooling ecosystem**: Swagger Codegen, Swagger UI, and later a massive ecosystem of generators, validators, and documentation tools. The spec succeeded because tools adopted it.
- **Pragmatic extensibility**: `x-` extensions let vendors add what they needed without waiting for the spec to catch up. This prevented the "standards committee bottleneck."
- **Single source of truth**: One document describes the entire API. No fragmentation across multiple files/formats.

### What Didn't Work

- **Extension sprawl**: Without a registry or governance, extensions proliferate. The Mermade project found hundreds of unique `x-` prefixes in the wild, many semantically overlapping.
- **Version migration pain**: Moving from Swagger 2.0 to OpenAPI 3.0 was a significant breaking change. Many organizations stayed on 2.0 for years.

### Lessons for Syllago

- **The `x-` pattern is proven.** Provider-specific properties should use a namespaced prefix that tools MUST ignore if unrecognized. This is directly applicable to hook definitions.
- **Extension graduation** (experiment -> propose -> standardize) is a good lifecycle model.
- **Tooling drives adoption**, not the spec itself. Syllago already has the tooling; the spec should be designed to make tooling easy to build.
- **Reserved prefixes for the spec maintainer** (like `x-oai-`) create a clean governance boundary.

---

## 3. JSON Schema

The foundational validation language. Relevant for its versioning model, extensibility approach, and the `additionalProperties` design decision.

### Versioning via `$schema`

- Every schema declares its dialect via the `$schema` keyword: `"$schema": "https://json-schema.org/draft/2020-12/schema"`.
- This is a **self-describing document** pattern — the document itself tells you how to interpret it.
- Each version is called a "dialect." Different dialects can coexist; tooling reads `$schema` to know which rules apply.
- `$schema` applies to the entire document and does NOT propagate to `$ref` targets — each referenced schema declares its own dialect.

### Extensibility

- **`additionalProperties`**: Controls whether unknown properties are allowed. Default is `true` (allow anything), which enables forward compatibility — old validators don't reject new fields.
- **`additionalProperties: false`** is the strict mode. Good for security (prevents mass assignment) but hostile to extensibility — schemas can't be extended via composition.
- **`unevaluatedProperties`** (added in 2019-09): Solves the composition problem. Unlike `additionalProperties`, it can "see through" `$ref` and `allOf`, enabling inheritance-like patterns without rejecting parent properties.
- **Custom keywords via vocabularies**: You can extend JSON Schema with custom keywords by defining a custom meta-schema. However, tooling support is inconsistent.

### The Subtractive Model

JSON Schema is fundamentally subtractive — more constraints means fewer valid documents. This is the opposite of how most people think about data modeling (additive). This impedance mismatch causes confusion and bugs, especially with inheritance patterns.

### Schema Versioning (SchemaVer)

Snowplow proposed SchemaVer as an alternative to SemVer for data schemas:
- **MODEL**: Breaking change, incompatible with all historical data.
- **REVISION**: May be incompatible with some historical data.
- **ADDITION**: Compatible with all historical data.

This framing is more useful than SemVer for schemas because the concern is data compatibility, not API compatibility.

### Lessons for Syllago

- **Self-describing documents** (`$schema` equivalent) should be required. Every hook definition should declare what schema version it conforms to.
- **Default to permissive** (allow unknown properties) for forward compatibility. A hook definition with extra fields an older tool doesn't understand should still be valid.
- **SchemaVer thinking applies**: Our spec versions should be categorized by whether they break existing hook definitions (MODEL), might break some (REVISION), or are purely additive (ADDITION).
- **The `additionalProperties` trap**: If we make our schema too strict, composition and extension become impossible. But if we make it too loose, validation provides no value. The right answer is probably strict on known fields, permissive on unknown fields (the `x-` pattern from OpenAPI).

---

## 4. MCP (Model Context Protocol)

Anthropic's protocol for tool integration. Directly relevant as a contemporary protocol in the AI tooling space.

### Capability Negotiation

- **Initialization handshake**: Client sends `initialize` with its capabilities and supported protocol version. Server responds with its capabilities and confirmed version.
- **Explicit declaration**: Both sides declare what they support. Features are gated behind capability flags — you only use what both sides agreed to.
- **Graceful degradation**: If version negotiation fails, the client can gracefully terminate. The protocol doesn't just fail silently.

### Versioning

- Currently uses **date-based versions** (e.g., `2025-11-25`). This is simple but provides no information about change severity.
- **Active proposal (SEP-1400)** to switch to SemVer, which would communicate whether changes are breaking. The proposal also floats **per-capability versioning** — tracking implementation compliance at the individual feature level. This is sophisticated but complex.

### Extension Model

- Extensions use **vendor-prefixed identifiers**: `{vendor-prefix}/{extension-name}` (e.g., `io.modelcontextprotocol/oauth-client-credentials`).
- Extensions are **strictly additive**: An implementation that doesn't recognize an extension skips it during negotiation. The core protocol keeps working.
- **Disabled by default**: Extensions MUST require explicit opt-in. This prevents accidental dependency on non-standard features.
- **Lifecycle**: Propose (SEP) -> Review -> Implement (reference implementation required) -> Publish -> Adopt.

### Lessons for Syllago

- **The `{vendor}/{name}` extension identifier pattern** is cleaner than `x-` prefixes. It provides both namespacing and attribution in one identifier.
- **Extensions disabled by default** is a strong pattern. Provider-specific hook features should not activate unless explicitly requested.
- **Requiring a reference implementation** before publishing an extension prevents paper specs that nobody implements.
- **Date-based versioning** is simple but uninformative. The industry is moving toward SemVer for good reason.
- **Per-capability versioning** is interesting but may be over-engineering for our needs. Worth monitoring how MCP's proposal plays out.

---

## 5. LSP (Language Server Protocol)

Microsoft's cross-editor protocol. The closest analogue to our problem: different editors (providers) support different subsets of language features (capabilities).

### Capability Negotiation

- **Initialize handshake**: Client and server exchange capabilities during `initialize`. The server says "I can do hover, completion, go-to-definition" and the client says "I support about-to-save notifications, dynamic registration."
- **Feature gating**: Capabilities are grouped by feature. A server that doesn't support `textDocument/definition` simply doesn't declare it, and the client never asks for it.
- **Forward compatibility rule**: Clients MUST ignore server capabilities they don't understand. The `initialize` request does not fail on unknown capabilities. This is critical for evolution.
- **Dynamic registration**: Capabilities can be registered/unregistered at runtime, not just at initialization. This enables lazy loading but adds complexity.

### Versioning

- Spec versions (e.g., 3.17, 3.18) group features. Individual features use `@since` annotations to track when they were introduced.
- **Capability flags are the real versioning mechanism**, not the spec version. Two LSP 3.17 implementations might support completely different feature sets. The spec version is more like a "feature catalog version."

### What Made It Succeed

- **Solved a real M-times-N problem**: Before LSP, every editor needed custom code for every language. LSP reduced this to M+N. This value proposition was so strong that adoption was almost inevitable.
- **Machine-readable spec**: The formal model generates ~90% of type definitions, eliminating serialization bugs across implementations.
- **Extreme backward compatibility**: Only one breaking change across the protocol's entire history. Existing implementations almost never break.
- **Presentation-focused**: LSP describes what to show, not semantic structure. This reduced language-specific assumptions and made the protocol more universal.

### What Doesn't Work

- **No standardized extension mechanism**: Custom methods are possible but require special code in every client. This negates the M+N benefit for anything beyond the core spec.
- **Lowest common denominator effect**: Without open governance for enumerations (symbol types, completion kinds), the spec becomes the ceiling, not the floor. Experimentation happens outside the spec and can't be shared.
- **Inconsistent state synchronization**: Different features use different update patterns (push vs. pull, full vs. incremental, with/without invalidation). This emerged organically as features were added and was never unified.
- **Governance gap**: LSP is effectively a Microsoft project. Features arrive as "fait accompli" rather than through open community process. This frustrates the broader community.

### Lessons for Syllago

- **Capability flags, not version numbers, should determine what's available.** A provider's capabilities matter more than what spec version it claims to support.
- **The forward-compatibility rule is essential**: Implementations MUST ignore what they don't understand. Never fail on unknown fields.
- **Open governance matters.** LSP's single-maintainer model is its biggest political weakness. If syllago's hook spec is meant to be adopted by multiple providers, the governance model matters as much as the technical design.
- **Machine-readable spec** generation prevents implementation drift. If we publish a JSON Schema for hook definitions, tools can validate without reimplementing parsing.
- **Avoid the inconsistent-patterns trap**: Design one way to handle hook execution, results, and state. Don't let different hook types evolve different patterns organically.

---

## 6. OCI (Open Container Initiative)

How a proprietary format (Docker) became an industry standard. Relevant to our potential journey from syllago's internal format to a broader spec.

### Standardization Story

- **Docker dominated** (2013-2015): Docker's image format and runtime became the de facto standard through adoption, not through any standards body.
- **Competing formats emerged**: rkt (CoreOS), lmctfy (Google), appc. Fragmentation threatened the ecosystem.
- **Docker donated its formats** to a neutral body (Linux Foundation) in June 2015. This was proactive — they donated while dominant, not after losing market share.
- **Three specs emerged**: Runtime (how to run containers), Image (how to package them), Distribution (how to share them). Each was split into a focused spec rather than one monolithic document.

### Design Decisions

- **Based on existing implementations**: The specs formalized what Docker already did, not a theoretical ideal. This meant implementations already existed on day one.
- **Focused scope**: Each spec does one thing. The runtime spec doesn't care about distribution; the image spec doesn't care about execution. This separation enabled independent evolution.
- **Extensibility through annotations**: OCI specs support annotations (key-value metadata) for custom data. Similar to OpenAPI's `x-` pattern.
- **Neutral governance**: The Linux Foundation hosts OCI. No single vendor controls the spec.

### What Made It Succeed

- **Incumbent donation**: Docker donating its format gave the spec instant legitimacy and existing implementations.
- **Industry alignment**: Major cloud vendors (AWS, Google, Azure) all committed to OCI support, making it the only viable path.
- **Distribution spec generality**: The distribution spec was designed generically enough to handle ANY content type, not just containers. This foresight enabled tools like Helm charts and WASM modules to reuse OCI registries.

### What Didn't Work

- **Slow pace**: OCI 1.0 took two years (2015-2017). The ecosystem moved faster than the standard.
- **Docker compatibility mode**: For years, OCI images were essentially Docker images with different metadata. The spec struggled to diverge from Docker's decisions even when better options existed.

### Lessons for Syllago

- **Formalize what exists, don't design in a vacuum.** The most successful specs codify existing practice. Our hook spec should describe what providers already do, with a clean canonical form for interchange.
- **Split by concern**: Separate specs for definition (what a hook is), execution (how it runs), and distribution (how it's shared) would be cleaner than one monolithic spec.
- **Donate early**: If we want broader adoption, defining the spec while we're the primary implementor gives us the most influence. Waiting until competitors exist means negotiating.
- **Generality pays off**: OCI's distribution spec being content-type-agnostic was a major win. Our spec should be flexible enough to handle hook types we haven't imagined yet.

---

## 7. AWS Cedar Policy Language

Used by sondera-ai for hook evaluation. A purpose-built authorization language with formal verification.

### Policy Model (PARC)

- Every policy evaluates: **Principal** (who) performs **Action** (what) on **Resource** (which) in **Context** (conditions).
- Policies have an **effect**: either `permit` or `forbid`. There's no "maybe."
- **Forbid overrides permit**: If ANY forbid policy matches, the answer is DENY regardless of permits. This is a conservative, safe-by-default model.
- **Default deny**: If no policy matches at all, the answer is DENY. You must explicitly permit access.

### Design Properties

- **No side effects**: Policy evaluation never changes state. This makes evaluation safe and cacheable.
- **Order-independent**: The result is the same regardless of which order policies are evaluated. No precedence rules, no ordering bugs.
- **Formally verified**: Cedar's authorization engine is proven correct via automated reasoning (SMT solvers). This is unique among policy languages.
- **Schema-validated but schema-independent at runtime**: Policies are validated against a schema at creation time, but the schema is not used during evaluation. This separates authoring-time safety from runtime performance.

### Capability Model

- **RBAC via entity hierarchies**: Principals belong to groups; groups have permissions. Standard role-based access.
- **ABAC via conditions**: Policies can reference arbitrary attributes of principals, resources, and context. This enables fine-grained access based on any property.
- **ReBAC via relationships**: Policies can navigate entity relationships (e.g., "permit if the user is a member of the resource's parent organization").

### Lessons for Syllago

- **The PARC model maps cleanly to hooks**: Principal = provider/user, Action = hook event, Resource = content item, Context = execution environment. Worth considering as the mental model for hook capability policies.
- **Forbid-overrides-permit** is a strong safety pattern. If a provider explicitly blocks a hook behavior, that should override any permit from the hook definition.
- **Default deny** is the right posture for security-sensitive operations. Hooks should do nothing unless explicitly permitted.
- **Order-independence** prevents subtle bugs. Hook evaluation should not depend on definition order.
- **Authoring-time validation vs. runtime execution** is a useful separation. Validate hook definitions when they're created; execute them without re-validating.

---

## 8. Standard Webhooks

An attempt to standardize webhook formats across providers. The closest analogue to standardizing a "fire-and-forget" event interface across different implementations.

### Design Model

- **Minimal required contract**: Three required headers (`webhook-id`, `webhook-signature`, `webhook-timestamp`) plus a JSON payload with `type`, `timestamp`, and `data` fields. That's the entire required surface.
- **Event type convention**: Hierarchical, dot-delimited identifiers (e.g., `user.created`, `invoice.payment_failed`). Character set restricted to `[a-zA-Z0-9_.]`.
- **Signature scheme**: Content signed as `{msg_id}.{timestamp}.{body}` using HMAC-SHA256. Signature format includes a version prefix (`v1,{signature}`) to allow algorithm evolution.
- **Idempotency built in**: `webhook-id` serves as an idempotency key. Receivers can deduplicate.
- **Replay protection**: Timestamp must be within an acceptable tolerance window.

### Extensibility

- **Forward and backward compatible by design**: New metadata can be added as top-level payload properties without breaking existing consumers.
- **Signature versioning**: The `v1,` prefix in signatures allows new signature schemes to be introduced without breaking existing verification. Multiple signatures can coexist (space-delimited) for zero-downtime key rotation.
- **Thin vs. full payloads**: The spec allows both, letting implementations optimize for their use case without the spec prescribing one model.

### What Made It Work

- **Extremely small surface area**: The spec is short enough to implement in an afternoon. Low adoption cost drives adoption.
- **Reference libraries in 8+ languages**: SDKs exist for Node, Python, Go, Ruby, Java, Kotlin, PHP, and Rust. This removes the "implement it yourself" barrier.
- **Focused on interoperability, not features**: The spec standardizes the contract (how webhooks look) not the content (what events mean). This avoids the domain-specific trap.
- **JWT analogy**: They explicitly positioned themselves as "JWT for webhooks" — a small, composable standard that larger systems build on.

### What's Still Unresolved

- **Adoption is voluntary**: Unlike OAuth or JWT which have strong network effects, webhooks don't require interoperability. Each provider can (and many do) ignore the standard.
- **No registry of event types**: The spec defines the format for event types but not a registry of standard types. Each provider defines its own.
- **Limited to HTTP POST**: The spec assumes webhooks are HTTP requests. This excludes other transport mechanisms.

### Lessons for Syllago

- **Small surface area drives adoption.** The required contract should be as minimal as possible. Everything else should be optional/extensible.
- **Version prefixes on values** (like `v1,{signature}`) allow evolution without breaking existing parsers. We could version hook definition formats similarly.
- **Reference libraries are as important as the spec.** Publishing an SDK for each provider to validate/convert hook definitions would accelerate adoption more than a perfect spec document.
- **Standardize the contract, not the content.** Define how hook definitions look, not what every hook does. Provider-specific behaviors belong in the extensibility layer.
- **Idempotency and replay protection** are patterns worth borrowing for hook execution tracking.

---

## Cross-Cutting Patterns

Patterns that appear across multiple successful standards:

### 1. Self-Describing Documents
Every successful spec requires documents to declare their version/dialect: OpenAPI's `openapi` field, JSON Schema's `$schema`, MCP's version negotiation. **Our hook definitions should include a schema version declaration.**

### 2. Ignore What You Don't Understand
OpenAPI, LSP, MCP, and Standard Webhooks all require implementations to ignore unknown fields/capabilities rather than failing. **This is the single most important forward-compatibility rule.**

### 3. Namespaced Extensions
OpenAPI uses `x-{vendor}-`, MCP uses `{vendor}/{name}`, OCI uses annotations. All provide a way for providers to add custom data without polluting the core spec. **Provider-specific hook properties should use a consistent namespacing scheme.**

### 4. Capability Flags Over Version Numbers
LSP and MCP both demonstrate that knowing "what can this implementation do?" matters more than "what version is it?" **Our provider capability matrix should be flag-based, not version-based.**

### 5. Strict Core, Permissive Extensions
The core spec should be tightly defined (required fields, known semantics), while the extension layer should be maximally permissive. JSON Schema's `additionalProperties` lesson: too strict prevents evolution, too loose prevents validation.

### 6. Formalize Existing Practice
OCI succeeded by standardizing what Docker already did. Git hooks succeeded because they formalized a simple execution model. **Our spec should describe what providers already do, with a clean interchange format, not invent an ideal that nothing implements.**

### 7. Small Surface Area
Standard Webhooks and Git hooks both demonstrate that minimal required contracts drive adoption. Complex specs (LSP has hundreds of methods) create implementation burden. **The required portion of our hook spec should fit on one page.**

### 8. Reference Implementations Matter
Standard Webhooks ships SDKs in 8 languages. MCP requires a reference implementation before publishing an extension. **Syllago itself IS the reference implementation — this is an advantage.**

---

## Anti-Patterns to Avoid

| Anti-Pattern | Source | Lesson |
|---|---|---|
| No distribution mechanism | Git hooks | Distribution must be solved from day one, not left to the ecosystem |
| Single-vendor governance | LSP | Multi-stakeholder input prevents "fait accompli" designs |
| Inconsistent internal patterns | LSP state sync | Design one pattern for hook execution/results and use it everywhere |
| Monolithic spec | Pre-split OCI | Separate concerns (definition, execution, distribution) into focused specs |
| Date-based versioning | MCP current | SemVer communicates change severity; dates don't |
| No composition model | Git hooks | Multiple hooks per event must be first-class |
| Extension sprawl without registry | OpenAPI `x-` | Namespacing alone isn't enough; a registry of known extensions prevents semantic duplication |
| Strict schemas blocking evolution | JSON Schema `additionalProperties: false` | Default permissive on unknown fields |

---

## Recommended Spec Design for Syllago Hooks

Based on the cross-cutting analysis:

1. **Self-describing**: Every hook definition includes `$schema` or equivalent version declaration.
2. **Forward-compatible**: Unknown fields are preserved, not rejected. The `ignore what you don't understand` rule.
3. **Namespaced extensions**: Provider-specific fields use `{provider}/` prefix (MCP style) or `x-{provider}-` prefix (OpenAPI style). The MCP style is more modern and explicit.
4. **Capability-based**: Providers declare supported hook features via capability flags, not version numbers.
5. **Default deny**: Hooks require explicit permission for security-sensitive operations (Cedar pattern).
6. **Order-independent**: Hook evaluation produces the same result regardless of definition order (Cedar pattern).
7. **Minimal required surface**: Core spec is small. Most features are optional/extensible.
8. **Machine-readable**: Publish a JSON Schema for hook definitions so tools can validate without custom parsers.
9. **Lossy conversion acknowledged**: Like LSP's presentation-focused design, accept that converting between providers is inherently lossy. Define what's preserved (core fields) and what's best-effort (extensions).
10. **Separate concerns**: Definition (what a hook is), validation (is it well-formed), execution (how it runs), and distribution (how it's shared) should be independent layers.
