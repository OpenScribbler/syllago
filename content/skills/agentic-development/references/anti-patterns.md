# Agent & Skill Anti-Patterns

Common mistakes that reduce agent effectiveness, increase hallucination risk, or waste tokens. For the positive patterns, see [patterns.md](patterns.md).

---

## Agent Anti-Patterns

### Bloated Agent (Embedded Knowledge)
- Smell: Large reference tables, checklists, or code examples (50+ lines) embedded in agent prompt.
- Fix: Extract to a skill. Agent prompt says `Load skills/x/SKILL.md`.
- Impact: Wastes tokens on every invocation, can't be reused or updated independently.

### Missing Boundaries
- Smell: Agent only describes what it does, never what it doesn't.
- Fix: Add "What I DON'T Do" section listing out-of-scope actions (don't deploy, don't make changes without approval, etc.).

### Vague Instructions
- Smell: "Be thorough", "use best judgment", "when appropriate".
- Fix: Replace with specific behavior: "Analyze all functions handling user input", "Format as: Issue | File:Line | Severity | Fix".

### Confidence Without Evidence
- Smell: Reports findings as facts without indicating certainty level.
- Fix: Require HIGH/MEDIUM/LOW confidence with evidence. See [patterns.md](patterns.md) Confidence Indicators.

### Token-Wasteful Commands
- Smell: Raw CLI commands without output filtering (`go test -v ./...`, `grep -r pattern .`).
- Fix: Use wrapper scripts (`go-dev test`) or tool parameters (`Grep head_limit=20`).

### No Fallback Guidance
- Smell: Assumes tools are always available. Agent fails if tool not installed.
- Fix: Add fallback path: "Tool not found? Fall back to manual pattern searching with Grep."

### Implement Without Planning
- Smell: Workflow jumps to "Read code → Fix issues → Run tests" with no planning phase.
- Fix: Add Analysis → Plan (with user approval) → Implement phases. See [patterns.md](patterns.md) Plan-First Workflow.

### Generic Persona
- Smell: "You are a helpful assistant that can review code."
- Fix: Specific expertise: "You are a Senior Go/Kubernetes Engineer specializing in secure distributed systems."

### Kitchen Sink Agent
- Smell: Agent does review + implementation + architecture + docs + deployment.
- Fix: One agent, one purpose. Create specialized agents. See [patterns.md](patterns.md) One Agent One Purpose.

### Context Blindness
- Smell: No awareness of context limits. No strategy for large codebases.
- Fix: Add context thresholds: <50K = full context, 50-100K = reduce, >100K = delegate. See [patterns.md](patterns.md) R&D Framework.

### No Verification Loop
- Smell: Makes changes without testing. "Understand → Change → Report completion."
- Fix: Require verification after every change. Test immediately, undo on failure.

## Skill Anti-Patterns

### Monolithic Skill
- Smell: Single 500+ line SKILL.md with everything inline.
- Fix: Entry point with quick reference + separate reference files loaded on-demand.

### No Quick Reference
- Smell: SKILL.md just says "see references" with no summary.
- Fix: Include quick lookup table in SKILL.md that satisfies 80% of needs.

### Commands in Agent, Not Skill
- Smell: Wrapper script documentation embedded in agent prompt, duplicated across agents.
- Fix: Commands in skill, agent just says "Load skill X for commands."

### Missing Load Guidance
- Smell: References listed without when-to-load context.
- Fix: Routing table with trigger keywords: `| Writing tests | testing.md |`.

### Abstract Rules Without Examples
- Smell: "Always handle errors properly." No concrete guidance.
- Fix: Include at least one concrete BAD/GOOD pair per rule.

## Token Waste Anti-Patterns

### Read Everything First
- Smell: Read 10+ files before analyzing. Wastes tokens on irrelevant content.
- Fix: `Grep files_with_matches` first, then read only matches.

### Full File Reads
- Smell: `Read: large_file.go` (2000 lines) when you need one function.
- Fix: `Grep -A 50` to find section, or `Read offset=450, limit=100`.

### Verbose Command Output
- Smell: `go test -v`, `docker build .`, `kubectl get pods -o yaml`.
- Fix: Use filtered wrappers (`go-dev test`), summary formats (`-o wide`), or pipe to `tail`.
