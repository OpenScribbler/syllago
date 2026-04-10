# Agentic Development Patterns

Proven patterns for building effective agents, skills, and autonomous workflows.

---

## Agent Design Patterns

### Focused Persona with Clear Boundaries
- Rule: Define WHO (specific expertise), WHAT (focused purpose), and WHAT NOT (explicit boundaries via "What I DON'T Do" section).
- Why: Clear boundaries prevent scope creep. Generic "helpful assistant" personas dilute effectiveness.

### Confidence Indicators
- Rule: Require agents to express HIGH/MEDIUM/LOW certainty on all findings.
- Action rules: HIGH = implement after approval. MEDIUM = confirm first. LOW = discuss only, never implement.
- Why: Prevents confident-but-wrong outputs.

### Skill Loading Protocol
- Rule: Agents reference skills for domain knowledge rather than embedding it. Keeps agent prompt focused on behavior, enables knowledge reuse, allows independent updates.

```markdown
## Skills to Load
**Primary**: Load `skills/go-patterns/SKILL.md` for Go commands.
**On-demand**: Load `references/resilience.md` when adding retry logic.
```

### Structured Output Formats
- Rule: Define specific output formats (tables, templates) for consistency. Consistent outputs are easier to parse and act on.

## Workflow Patterns

### Iterative Prompting
- Rule: Build through incremental natural language instructions. One feature at a time: express intent → review → apply → commit → verify → iterate.

### Verification Loops
- Rule: Validate code after every AI change before proceeding. Every compound error grows from an unverified change.
- Process: AI generates → review → apply → RUN IMMEDIATELY → if success: commit, if failure: undo and refine.

### Plan-First Workflow
- Rule: For agents that make changes, require planning before implementation.
- Phases: Analysis (load skills, read files, evaluate) → Plan (propose with confidence levels, get approval) → Implement (after approval, verify with before/after).

### Investigation Before Action
- Rule: Read existing code before changes. Confirm bugs are real before fixing. Understand root cause first. 80% of agent failures trace to skipping investigation.

## Orchestration Patterns

### Architect-Editor
- When: Task requires >100K tokens context, significant architectural decisions, or large refactoring.
- Structure: Architect (Opus) gets full context → produces spec. Editor(s) (Sonnet) get spec only → implement.

### Director Pattern
- When: Test-driven development with clear success criteria.
- Structure: Generate code → run tests → if pass: done. If fail: evaluator analyzes → coder updates → loop.

### PITER Framework
- Phases: Plan → Implement → Test → Execute (deploy) → Report.
- Use for autonomous feature implementation.

### Closed-Loop Feedback
- Rule: Agent tests its own output and iterates until tests pass. `generate → test → if pass: return, if fail: refine → loop`.

## Multi-Agent Patterns

### One Agent One Purpose
- Rule: Specialized agents with minimal context outperform generalists. One agent = one domain. Focused context = better results.

### Worktree Isolation
- When: Parallel agent execution without file conflicts.
- Structure: `git worktree add ../wt-feature-a -b feature-a`. Each worktree = independent working directory, shared git history. Use sparse checkout for context reduction.

### Multi-Agent Delegation
- Rule: Decompose work → delegate subtasks via Task tool to specialists → aggregate results.

## Context Patterns

### R&D Framework (Reduce or Delegate)
- <50K tokens: Use full context.
- 50-100K tokens + focused: REDUCE (search before read, sparse checkout, focused files).
- 100K+ tokens or complex: DELEGATE (Architect-Editor, sub-agents).

### Three-Legged Stool
- Rule: Debug AI failures systematically. Check Context (80%) → Prompt (15%) → Model (5%).
- See [debugging.md](debugging.md) for detailed troubleshooting.

## Skill Patterns

### Entry Point with Quick Reference
- Rule: SKILL.md provides fast lookup (<100 lines); references provide depth. Agent can often get what it needs without loading large references.

### Progressive Detail Loading
- Rule: Search before read. `Grep files_with_matches` → `Grep content with -A` → `Read with offset/limit`.

### Agent-Skill Separation

| Belongs in Agent | Belongs in Skill |
|------------------|------------------|
| Persona, workflow, boundaries | Domain knowledge, commands |
| Confidence rules, when to load | Checklists, patterns, examples |

## Pattern Selection

| Scenario | Pattern |
|----------|---------|
| Starting new feature | Iterative Prompting |
| Large codebase refactor | Architect-Editor |
| Parallel development | Worktree Isolation |
| Test-driven workflow | Director, Closed-Loop |
| Context too large | R&D Framework |
| Agent keeps failing | Three-Legged Stool |
| Multiple agents | Multi-Agent Delegation |
