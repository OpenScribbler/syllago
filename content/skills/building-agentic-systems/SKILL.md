---
name: building-agentic-systems
description: Guides development of autonomous AI-driven software systems (agentic coding) using patterns from 20+ production implementations. Provides architecture guidance (MVA to Production), pattern selection (34 patterns), component design (Commands, Workflows, Agents, Hooks, Tools), debugging strategies (Three-Legged Stool), and production deployment. Use when building AI-assisted or agentic development workflows, creating autonomous agents, implementing ADWs (Agentic Developer Workflows), designing multi-agent systems, setting up git worktrees for parallel execution, implementing closed-loop feedback systems, debugging agent failures, scaling from prototype to production, or any mention of agentic coding, autonomous workflows, agent orchestration, PITER framework, Director pattern, Architect-Editor, context engineering, prompt engineering, leverage points, or Claude Code automation.
---

# Building Agentic Systems

## Table of Contents
1. [Overview](#overview)
2. [Quick Decision Trees](#quick-decision-trees)
3. [Getting Started](#getting-started)
4. [The 10 Universal Principles](#the-10-universal-principles)
5. [When You're Working On...](#when-youre-working-on)
6. [Common Scenarios](#common-scenarios)

---

## Overview

**Agentic coding** = AI agents autonomously executing complex software development tasks with minimal human intervention.

### Two Phases of Evolution

**Phase 1: AI-Assisted Development**
- Human writes prompts, AI generates code
- Human reviews, refines, integrates
- Linear workflow: prompt → code → review → iterate

**Phase 2: Agentic Development** (This Framework)
- AI agent autonomously completes entire tasks
- Closed-loop feedback and verification
- Autonomous workflow: task → agent → verification → done

### When to Use This Skill

Use this skill when you need to:
- **Start**: Set up agentic development from scratch (MVA in 2-4 hours)
- **Choose**: Select the right pattern, component, or architecture approach
- **Build**: Design commands, workflows, agents, hooks, or tools
- **Debug**: Systematically troubleshoot agent failures
- **Scale**: Move from prototype to production-ready systems
- **Deploy**: Harden security, track costs, monitor performance

### Skill Navigation

This skill uses **progressive disclosure**:
- **This file (SKILL.md)**: Critical decisions and high-level guidance
- **guides/**: Deep dives on specific topics (8 comprehensive guides)
- **scripts/**: Helper scripts for deterministic operations

---

## Quick Decision Trees

### 1. Component Selection: "Which type should I build?"

```
START: I need to...
│
├─ Execute a simple task Claude already knows how to do
│  └─ Use a SLASH COMMAND (.claude/commands/task-name.md)
│     When: Task is <50 lines, no code execution needed
│
├─ Give Claude specialized domain knowledge
│  └─ Use a SKILL (.claude/skills/skill-name/)
│     When: Reusable capability, needs reference docs
│
├─ Run a complex multi-step Python workflow
│  └─ Use an ADW (Agentic Developer Workflow)
│     When: Needs orchestration, state management, tool integration
│
├─ Offload context-heavy work to another agent
│  └─ Use a SUB-AGENT (Task tool invocation)
│     When: Task requires >10K tokens context, or parallel execution
│
├─ Integrate external services/APIs
│  └─ Use MCP (Model Context Protocol)
│     When: Need persistent connections, streaming, or complex APIs
│
└─ Observe or modify Claude's behavior
   └─ Use HOOKS (.claude/hooks/)
      When: Need to intercept tool calls, add logging, or enforce policies
```

**Quick Rule**: Command < Skill < ADW < Sub-agent < MCP < Hook
(Complexity and power increase left to right)

**See [decisions guide](guides/decisions.md) for 20+ detailed decision trees**

---

### 2. Architecture Maturity: "What phase am I in?"

```
START: Current capabilities?
│
├─ No agentic setup yet
│  └─ PHASE 1: MVA (Minimum Viable Agentic) - 2-4 hours
│     Goal: First slash command + first ADW
│     See: guides/getting-started.md
│
├─ Have commands and basic ADWs
│  └─ PHASE 2: Intermediate - 1-2 weeks
│     Goal: ai_docs/, specs/, validation hooks, multi-phase workflows
│     See: guides/production.md#phase-2
│
├─ Need parallel agents or advanced orchestration
│  └─ PHASE 3: Advanced - 2-4 weeks
│     Goal: Git worktrees, multi-agent systems, context optimization
│     See: guides/production.md#phase-3
│
└─ Ready for production deployment
   └─ PHASE 4: Production - 4-8 weeks
      Goal: Security, monitoring, cost tracking, team adoption
      See: guides/production.md#phase-4
```

---

### 3. Debugging: "My agent failed. Where do I look?"

**The Three-Legged Stool** (80-15-5 Rule)

```
Agent Failed
│
├─ 80% CONTEXT issues
│  ├─ Missing files? → Add to ai_docs/ or explicit instructions
│  ├─ Wrong directory? → Check paths, use absolute paths
│  ├─ Too much context? → Use sub-agents (R&D Framework)
│  └─ Outdated context? → Refresh specs, update docs
│
├─ 15% PROMPT issues
│  ├─ Unclear intent? → Be more specific, add examples
│  ├─ Conflicting instructions? → Reconcile contradictions
│  └─ Missing constraints? → Add explicit boundaries
│
└─ 5% MODEL issues
   ├─ Wrong model? → Use Sonnet for coding, O1 for reasoning
   └─ Model limitations? → Break task into smaller steps
```

**Diagnostic Checklist**:
1. Check context (ai_docs/, specs/, file access)
2. Check prompt clarity (specific? examples?)
3. Check model selection (right tool for job?)
4. Check verification (is failure real or perceived?)

**See [debugging guide](guides/debugging.md) for systematic troubleshooting**

---

### 4. Pattern Selection: "Which pattern should I use?"

**Most Common Patterns** (by frequency of use):

```
Scenario → Pattern
│
├─ Agent needs to verify its work
│  └─ Verification Loops (run tests after code changes)
│
├─ Task is too complex for one shot
│  └─ Iterative Prompting (break into 5-10 line increments)
│
├─ Need to change multiple files safely
│  └─ Multi-File Refactoring (plan → execute → verify)
│
├─ Agent needs structured data output
│  └─ Structured Output (JSON, YAML, specific format)
│
├─ Need architectural planning before coding
│  └─ Architect-Editor (plan → implement)
│
├─ Need to orchestrate multiple agents
│  └─ Director Pattern (coordinator + specialist agents)
│
├─ Context is too large for single agent
│  └─ R&D Framework (Reduce or Delegate)
│
└─ Debugging agent failures
   └─ Three-Legged Stool (Context 80%, Prompt 15%, Model 5%)
```

**See [patterns guide](guides/patterns.md) for all 34 patterns**

---

### 5. Model Selection: "Sonnet or O1?"

```
Task Type → Model
│
├─ Code generation, file editing, tool use
│  └─ Sonnet 4.5 ($3/$15 per 1M tokens)
│     Fast, reliable, great for iterative coding
│
├─ Complex reasoning, mathematical problems, research
│  └─ O1 ($15/$60 per 1M tokens)
│     Deeper reasoning, but slower and more expensive
│
└─ Simple tasks, high volume, cost-sensitive
   └─ Haiku ($0.80/$4 per 1M tokens)
      Fast and cheap, good for basic operations
```

**Default Choice**: Start with Sonnet. Only use O1 if task requires deep reasoning.

---

## Getting Started

### MVA (Minimum Viable Agentic) in 2-4 Hours

**Goal**: Create your first autonomous workflow

**Setup**:
```bash
# 1. Directory structure
mkdir -p .claude/{commands,workflows,hooks,skills}
mkdir -p .claude/ai_docs

# 2. Create first command (.claude/commands/analyze.md)
echo "Analyze the codebase and provide insights on architecture" > .claude/commands/analyze.md

# 3. Test it
# In Claude Code: /analyze
```

**First ADW (Agentic Developer Workflow)**:
```python
# .claude/workflows/adw_hello.py
from anthropic import Anthropic

def execute_task(task_description: str):
    """Execute a task using Claude as an agent."""
    client = Anthropic()

    response = client.messages.create(
        model="claude-sonnet-4-5-20250929",
        max_tokens=4096,
        messages=[{
            "role": "user",
            "content": task_description
        }]
    )

    return response.content[0].text

if __name__ == "__main__":
    result = execute_task("Create a hello world Python script")
    print(result)
```

**Verification**:
- [ ] Can execute `/` commands in Claude Code
- [ ] Can run Python ADW from command line
- [ ] ADW completes task autonomously
- [ ] Results are correct

**Next Steps**:
- Add ai_docs/ for domain knowledge
- Create specs/ for task specifications
- Add validation hooks
- Build multi-phase workflows

**See [getting started guide](guides/getting-started.md) for complete MVA walkthrough**

---

## The 10 Universal Principles

### 1. Incremental Everything
**Why**: Large changes = high failure risk. Small steps = verifiable progress.
**How**: 5-10 lines at a time, test after each change.

### 2. Verification at Every Level
**Why**: Agents make mistakes. Verification catches them early.
**How**: Run tests, check outputs, validate assumptions.

### 3. Natural Language as Interface
**Why**: Writing code to automate AI is ironic. Use prompts.
**How**: Slash commands, instructions in markdown, conversational specs.

### 4. Structure Emerges
**Why**: Premature structure = overhead. Let patterns emerge organically.
**How**: Start flat (.claude/workflows/), organize when patterns appear.

### 5. Measurement-Driven
**Why**: Can't improve what you don't measure.
**How**: Token usage, cost per task, success rates, cycle time.

### 6. One-Shot as Goal
**Why**: Every refinement cycle = time + cost. Optimize for first-try success.
**How**: Better context, clearer prompts, verification loops.

### 7. Agent Perspective
**Why**: Design for AI consumption, not human readability.
**How**: Structured docs, clear file boundaries, explicit dependencies.

### 8. Feedback Loops Everywhere
**Why**: Agents improve through feedback. No feedback = no learning.
**How**: Hooks, validation, metrics, error messages.

### 9. Specialization > Generalization
**Why**: Specialist agents outperform generalist agents.
**How**: One agent, one purpose. Director pattern for orchestration.

### 10. Prompts Are Primitive
**Why**: Prompts are the lowest-level building block.
**How**: Slash commands = primitive operations, ADWs = composition.

**See [principles guide](guides/principles.md) for deep dive on frameworks**

---

## When You're Working On...

**Choose the right guide for your current task:**

### Starting a New Project
→ [Getting Started Guide](guides/getting-started.md)
- Complete MVA walkthrough (2-4 hours)
- Directory structure setup
- First command, agent, and ADW
- Verification checklist

### Choosing a Pattern
→ [Patterns Guide](guides/patterns.md) + [Decisions Guide](guides/decisions.md)
- 34 patterns with real code examples
- Pattern selection decision trees
- Pattern combinations and stacks
- When to use each pattern

### Understanding Principles
→ [Principles Guide](guides/principles.md)
- 10 Universal Principles (detailed WHY)
- Core Four framework (Context + Model + Prompt + Tools)
- 12 Leverage Points
- 7 Prompt Levels
- R&D Framework (Reduce or Delegate)
- PITER Framework

### Building Components
→ [Components Guide](guides/components.md)
- Commands (slash commands)
- Workflows/ADWs (orchestration)
- Agents (specialization)
- Hooks (observability)
- Tools/MCP (custom tools)
- Knowledge Bases (ai_docs/)
- Tasks (automation)
- Specs (specification-driven)

### Debugging Failures
→ [Debugging Guide](guides/debugging.md)
- Three-Legged Stool diagnostic framework
- Common failure modes by symptom
- Debug checklist
- Context/Prompt/Model issue deep-dives
- Quick fixes table

### Production Deployment
→ [Production Guide](guides/production.md)
- Production readiness checklist
- Phase 3: Advanced (worktrees, multi-agent)
- Phase 4: Production (security, monitoring, cost)
- Incident response
- Team adoption

### Quick Lookup
→ [Reference Guide](guides/reference.md)
- Command cheat sheet
- Workflow templates
- Directory structure
- Glossary (30 essential terms)
- Common patterns lookup

---

## Common Scenarios

| Scenario | Pattern | See Guide |
|----------|---------|-----------|
| "Create a command that..." | Slash Command | [components → commands](guides/components.md#commands) |
| "Build a workflow to..." | ADW | [components → workflows](guides/components.md#workflows) |
| "Set up parallel agents..." | Git Worktrees | [patterns → worktree](guides/patterns.md#worktree-isolation) |
| "My agent keeps failing..." | Three-Legged Stool | [debugging → stool](guides/debugging.md#three-legged-stool) |
| "Deploy to production..." | Phase 4 | [production → phase-4](guides/production.md#phase-4) |
| "Choose a pattern..." | Decision Trees | [decisions](guides/decisions.md) |
| "Optimize context..." | R&D Framework | [principles → r-d](guides/principles.md#rd-framework) |

---

## Framework Architecture

**Maturity**: MVA (2-4h) → Intermediate (1-2w) → Advanced (2-4w) → Production (4-8w)

**Costs**: $0.10-1 (MVA) → $1-10 (Intermediate) → $10-50 (Advanced) → $50-200 (Production)

**Metrics**: >80% first-shot success, <100K tokens per task, minutes (not hours) cycle time

---

## Learning Path

| Phase | Duration | Focus | Key Guide |
|-------|----------|-------|-----------|
| **Beginner** | Week 1-2 | MVA setup, 10 principles, simple commands | [getting-started](guides/getting-started.md) |
| **Intermediate** | Week 3-6 | Patterns, ADWs, ai_docs/, specs/ | [patterns](guides/patterns.md) |
| **Advanced** | Week 7-12 | Worktrees, multi-agent, context optimization | [production](guides/production.md) |
| **Production** | Month 4-6 | Security, monitoring, cost tracking, teams | [production](guides/production.md) |

---

## Key Frameworks Reference

**Core Four**: Context (what agent knows) + Model (which AI) + Prompt (what you ask) + Tools (what agent can do)

**12 Leverage Points** (Top 5): Model selection (10x) > Context quality (8x) > Prompt clarity (5x) > Tool access (5x) > Verification (4x)

**R&D Framework**: Reduce (<30K tokens) vs Delegate (>30K tokens, distinct phases)

**See [principles guide](guides/principles.md) for complete frameworks**

---

## Quick Reference Links

| Guide | Purpose |
|-------|---------|
| [Getting Started](guides/getting-started.md) | MVA walkthrough |
| [Patterns](guides/patterns.md) | 34 patterns catalog |
| [Decisions](guides/decisions.md) | 20+ decision trees |
| [Production](guides/production.md) | Phases 3-4 deployment |
| [Principles](guides/principles.md) | Universal principles + frameworks |
| [Components](guides/components.md) | Building blocks |
| [Debugging](guides/debugging.md) | Troubleshooting |
| [Reference](guides/reference.md) | Cheat sheets + glossary |

**Scripts**: [setup_mva.sh](scripts/setup_mva.sh) | [create_worktree.sh](scripts/create_worktree.sh)

---

**Start** → [Getting Started](guides/getting-started.md) | **Choose** → [Decisions](guides/decisions.md) | **Understand** → [Principles](guides/principles.md)
