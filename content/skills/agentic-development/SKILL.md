---
name: agentic-development
description: Comprehensive patterns for AI agent and skill development. Covers agent design, evaluation, skill extraction, autonomous workflows, debugging, and production deployment. Use when building new agents, improving existing ones, creating agentic workflows, or diagnosing agent failures.
---

# Agentic Development

Patterns and practices for building effective AI agents, skills, and autonomous workflows.

## Quick Evaluation Checklist (6 Dimensions)

| Dimension | Check | Red Flag |
|-----------|-------|----------|
| **Clarity** | Are instructions unambiguous? | "Be helpful", "Use best judgment" |
| **Specialization** | Single focused purpose? | Kitchen-sink agent, generic "assistant" |
| **Workflow Quality** | Plan-first? Verification loops? | Jumps to implementation, no testing |
| **Context Awareness** | Knows when to delegate/reduce? | No file loading guidance, ignores limits |
| **Safety** | Boundaries + confidence levels? | No "What I DON'T Do", no evidence rules |
| **Efficiency** | <200 lines? Knowledge in skills? | Embedded tables, raw verbose commands |

## 10 Universal Principles

1. **Incremental Everything** - Small changes = verifiable progress. 5-10 lines at a time.
2. **Verification at Every Level** - Run tests, check outputs, validate assumptions.
3. **Natural Language as Interface** - Prompts over code for AI orchestration.
4. **Structure Emerges** - Start flat, organize when patterns appear.
5. **Measurement-Driven** - Track tokens, costs, success rates.
6. **One-Shot as Goal** - Optimize for first-try success.
7. **Agent Perspective** - Design for AI consumption, not human readability.
8. **Feedback Loops Everywhere** - Hooks, validation, metrics.
9. **Specialization > Generalization** - One agent, one purpose.
10. **Prompts Are Primitive** - Commands = operations, workflows = composition.

## Decision Trees

### Should This Be a Skill?

```
Content in agent prompt
        |
        v
Is it domain knowledge (checklists, patterns, commands)?
        |-- Yes --> EXTRACT TO SKILL
        |-- No
            v
Could multiple agents use this content?
        |-- Yes --> EXTRACT TO SKILL
        |-- No
            v
Is it >50 lines of static content?
        |-- Yes --> CONSIDER SKILL
        |-- No --> KEEP IN AGENT
```

### Component Selection

```
I need to...
    |
    +- Execute a simple task Claude knows --> SLASH COMMAND
    +- Give Claude specialized knowledge --> SKILL
    +- Run complex multi-step workflow --> ADW (Python)
    +- Offload context-heavy work --> SUB-AGENT (Task tool)
    +- Integrate external APIs --> MCP SERVER
    +- Observe/modify Claude's behavior --> HOOKS
```

### Architecture Maturity

| Phase | Goal | Key Additions |
|-------|------|---------------|
| **MVA** | First working automation | Command + basic ADW |
| **Intermediate** | Reliable workflows | ai_docs/, specs/, validation |
| **Advanced** | Parallel execution | Worktrees, multi-agent, context management |
| **Production** | Observable & secure | Hooks, monitoring, cost tracking |

## Debugging Quick Reference (Three-Legged Stool)

```
Agent Failed
    |
    +- 80% CONTEXT issues
    |   +- Missing files? --> Add to context
    |   +- Wrong directory? --> Check paths
    |   +- Too much context? --> Use sub-agents
    |
    +- 15% PROMPT issues
    |   +- Unclear intent? --> Add examples
    |   +- Conflicting? --> Reconcile
    |
    +- 5% MODEL issues
        +- Wrong model? --> Switch (Sonnet for coding, Opus for reasoning)
```

## Workflows

Interactive processes for common tasks:

| Trigger | Workflow | Purpose |
|---------|----------|---------|
| "create agent", "new agent" | [CreateAgent.md](Workflows/CreateAgent.md) | Design and create new agent |
| "evaluate agent", "review agent" | [EvaluateAgent.md](Workflows/EvaluateAgent.md) | Assess agent quality |
| "create skill", "new skill" | [CreateSkill.md](Workflows/CreateSkill.md) | Design skill from scratch |
| "evaluate skill", "review skill" | [EvaluateSkill.md](Workflows/EvaluateSkill.md) | Assess skill quality |
| "extract skill", "agent too big" | [ExtractSkill.md](Workflows/ExtractSkill.md) | Extract content to skill |

## References

Load on-demand based on task:

| Task | Reference |
|------|-----------|
| Building/improving agents | [patterns.md](references/patterns.md) |
| Reviewing agent quality | [evaluation-rubric.md](references/evaluation-rubric.md) |
| Looking for problems | [anti-patterns.md](references/anti-patterns.md) |
| Extracting skills | [skill-extraction.md](references/skill-extraction.md) |
| Optimizing token usage | [token-optimization.md](references/token-optimization.md) |
| Understanding components | [components.md](references/components.md) |
| Debugging agent failures | [debugging.md](references/debugging.md) |
| Production deployment | [production.md](references/production.md) |
| Structure templates | [templates.md](references/templates.md) |
| Condensing language-pattern skills for token efficiency | [skill-optimization-template.md](references/skill-optimization-template.md) |

## Quick Fixes Table

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| Agent did nothing | Unclear task | Make prompt actionable |
| Wrong changes | Missing context | Add relevant files/docs |
| Import errors | Missing file in context | Add imported file |
| Slow responses | Model too strong | Use Sonnet instead of Opus |
| High costs | Context bloat | Reduce files, use sub-agents |
| Inconsistent output | No confidence levels | Add confidence indicator |
| Scope creep | Missing boundaries | Add "What I DON'T Do" |
