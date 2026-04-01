# Spec Gaps & Competing Approaches

## Technical Gaps in the Current Spec

### 1. No Versioning or Dependency System

- No semver enforcement, no lock files, no dependency declarations
- `metadata.version` is optional freeform string — purely informational
- Open issues: agentskills#110 (version validation), #46 (version locking), #100 (skill-to-skill dependencies)
- Consensus forming: versioning belongs in distribution/manifest layer, not SKILL.md

### 2. No Distribution or Registry Standard

- Spec defines what a skill *is* but nothing about distribution, installation, updates, or remote discovery
- Open: agentskills#255 (`.well-known` URI), #81 (npm distribution RFC), #42 (remote skills via URL)
- Absence led to third-party registries (ClawHub, skills.sh, SkillsMP) → security crisis

### 3. No Security Model

- Zero normative security language in the spec
- `allowed-tools` marked "Experimental"
- No: code signing, provenance verification, sandboxing spec, trust boundaries, permissions model
- Consequences: 36.82% of registry skills had security flaws (Snyk ToxicSkills)

### 4. Activation Reliability

- Relies entirely on model judgment reading name/description pairs
- No: trigger patterns, priority ordering, forced activation, feedback on rejected skills
- Vercel evals: 53-79% activation vs AGENTS.md 100%
- Open RFC: agentskills#57

### 5. Underspecified Script Execution

- "Supported languages depend on the agent implementation"
- No specification for: environment variables, working directory, stdout/stderr contract, success/failure signaling, resource limits
- Google ADK explicitly notes script execution "not yet supported"

### 6. No Structured Input/Output Contract

- No mechanism for declaring inputs, outputs, parameter types, schemas
- LlamaIndex: "being defined with natural language leaves skills open for misinterpretations and hallucinations"

### 7. No Nested or Composed Skills

- agentskills#137: spec silent on whether nested skills are permitted
- No way to compose small skills into larger workflows
- Community proposals for "Skill Composition" exist but unresolved

### 8. Discovery Path Fragmentation

- Spec doesn't mandate discovery paths — only suggests patterns
- Testing confirmed skills in `.claude/skills/` not discovered by Cursor
- No unified resolution algorithm across implementations

## MCP Comparison

| Dimension | Agent Skills | MCP |
|-----------|-------------|-----|
| **Format** | Markdown + YAML frontmatter | JSON-RPC 2.0 + JSON Schema |
| **Execution** | LLM interprets natural language | Deterministic API calls |
| **Auth** | None | OAuth, bearer tokens, API keys |
| **Discovery** | Filesystem scan at startup | `initialize` → capability negotiation |
| **State** | Stateless files | Stateful connections with lifecycle |
| **Real-time** | No (static files) | Yes (notifications, SSE) |
| **Typing** | None | JSON Schema for all inputs/outputs |
| **Infrastructure** | Zero (markdown files) | Server process + transport |
| **Portability** | 30+ tools, no runtime dependency | Requires transport support |

### What MCP Does Better
- Deterministic execution with typed inputs/outputs
- First-class authentication (OAuth, bearer tokens)
- Dynamic capability updates via notifications
- Structured schemas with JSON Schema validation
- Client-server capability negotiation

### What Skills Do Better
- Zero infrastructure — directory with markdown
- Human readable — anyone can audit
- Context efficient — ~50 tokens at startup vs MCP's upfront schema cost
- Portable — works across 30+ tools without runtime
- Workflow encoding — multi-step procedures, decision trees, domain knowledge

### Community Consensus
"Use MCP to connect to your customer database. Use a Skill to teach Claude how to generate a monthly report from that data." (Mintlify) The community has largely settled on using both.

## Simpler Alternatives

### .cursorrules / Cursor Rules

Four activation modes skills lack:
- **alwaysApply**: injected into every prompt (100% reliable)
- **globs**: applied when specific file patterns match (deterministic)
- **description-based**: agent decides (same as skills)
- **manual @-mention**: user-triggered

Also supports **Team Rules** with organizational governance and precedence ordering (Team > Project > User).

### AGENTS.md / CLAUDE.md

Simplest possible format: markdown file in project root. No frontmatter, no discovery, no activation logic.

Why developers prefer them:
- 100% reliable (always loaded)
- Zero indirection
- Easy to audit
- Cross-tool support

Counter-argument is scale: 20+ instruction sets would consume enormous context on every interaction.

### Plain Docs / README

Some argue "just write instructions in English in any old format" is sufficient. Counter: Skills provide progressive disclosure that plain docs don't.

## Enterprise Gaps

### Authentication and Authorization
- No signing, no permission scoping, no organizational access controls, no revocation
- Led to Snyk ToxicSkills findings

### Governance
- No organizational approval workflows
- No mandatory skill policies
- No audit trails
- No role-based access
- Cursor's Team Rules is closest existing implementation (vendor-specific)

### Versioning
- `metadata.version` optional, freeform, purely informational
- No semver parsing, compatibility checking, upgrade paths
- `skills-ref` validation tool doesn't check version values

### Discovery
- Remote discovery entirely unspecified
- `.well-known` proposal (agentskills#255) open
- Currently: manually browse GitHub or third-party registries

## Emerging Patterns

### A2A (Agent-to-Agent Protocol)
Different layer — agents discover and collaborate as peers. Agent Cards provide richer capability declaration than SKILL.md. Complementary: skills teach *how*, A2A provides the protocol.

### Google ADK
Full-platform approach: orchestration, state management, evaluation, deployment, explicit skills support. Consumes SKILL.md but adds runtime infrastructure spec doesn't provide.

### The Layering Pattern (Emerging Best Practice)
- **AGENTS.md** — always-on project context (100% reliable)
- **Skills** — on-demand procedural knowledge (progressive disclosure)
- **MCP** — external tool connectivity (authenticated, structured)

Critical insight: skills work best when always-on context explicitly tells the agent to use them.

### Community-Proposed Extensions
- `includes` frontmatter field for shared files
- Signature blocks for cryptographic provenance
- Tool dependencies declaration
- Skill parameterization (environment requirements)
- AgentFile format (declarative agent composition)
- Capabilities field (security and transparency)

### Supply Chain Security Response
- OWASP Agentic Skills Top 10
- Snyk ToxicSkills research + agent-scan tool
- Cisco AI Defense skill scanner
- Embrace The Red Unicode attack research
- Linux Foundation AAIF governance body
