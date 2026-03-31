# Agentic Coding Decision Trees

Visual decision guides for rapid choices in common scenarios.

---

## Table of Contents

1. [How to Use](#how-to-use)
2. [Component Selection](#component-selection)
3. [Architecture Maturity](#architecture-maturity)
4. [Pattern Selection](#pattern-selection)
5. [Context Management](#context-management)
6. [Model Selection](#model-selection)
7. [Debugging Flowchart](#debugging-flowchart)
8. [Quick Reference](#quick-reference)

---

## How to Use

**Purpose**: Instant guidance without reading comprehensive docs.

**Format**: Testable conditions → Recommendations → Rationale

**Reading**:
- **Bold**: Decision points
- *Italic*: Rationale
- `Code`: Components/files
- →: Recommendations

**Coverage**: 80% of common scenarios. For edge cases, consult full framework.

---

## Component Selection

### Command vs Skill vs Sub-agent vs ADW vs MCP

**Question**: Which component for my capability?

```
Need reusable capability?
  │
  ├─ External API/service integration?
  │  ├─ YES → MCP Server
  │  │         Why: Designed for external integrations
  │  │         Example: Jira API, Weather API, Database
  │  │         Warning: All tools load on startup (context cost)
  │  │
  │  └─ NO → Continue
  │
  ├─ Need parallel execution?
  │  ├─ YES → Sub-agent (ALWAYS)
  │  │         Why: Only sub-agents support parallelism
  │  │         Example: Create 3 worktrees simultaneously
  │  │         Pattern: Task tool spawns N sub-agents
  │  │
  │  └─ NO → Continue
  │
  ├─ Agent should trigger automatically?
  │  ├─ YES → Multiple operations?
  │  │  ├─ YES → Skill
  │  │  │         Why: Modular structure for workflows
  │  │  │         Example: Git worktree manager
  │  │  │         Location: skills/<name>/skill.md
  │  │  │
  │  │  └─ NO → Slash Command
  │  │            Why: Single operation, no overhead
  │  │            Example: Generate commit message
  │  │            Location: .claude/commands/<name>.md
  │  │
  │  └─ NO (Manual trigger) → Slash Command
  │            Why: Explicit user control
  │            Example: /quick-plan, /build
  │
  └─ Unattended execution needed?
     └─ YES → ADW (Python script)
               Why: Runs without human in loop
               Example: PITER workflow
               Location: adws/<name>.py
```

**Key Insight**: PRIMARY distinction is WHO triggers (agent vs user), not WHAT they do.

**Decision Examples**:

1. "Extract PDFs automatically" → **Skill** (agent-triggered, multi-step)
2. "Connect to Jira" → **MCP Server** (external service)
3. "Create 5 worktrees in parallel" → **Sub-agents** (parallel keyword)
4. "Generate commit message" → **Slash Command** (manual, single-step)
5. "Security audit" → **Sub-agent** (context isolation)

---

### MCP Server vs Skill

**Question**: Internal workflow - MCP or Skill?

```
Internal workflow decision:
  │
  ├─ Truly external system?
  │  ├─ YES → MCP Server
  │  │         Examples: APIs, databases, file systems
  │  │
  │  └─ NO → Continue
  │
  ├─ Need to expose tools to Claude?
  │  ├─ YES → Tools used EVERY session?
  │  │  ├─ YES → MCP Server
  │  │  │         Why: Worth upfront context load
  │  │  │         Example: Project-specific navigator
  │  │  │
  │  │  └─ NO → Skill (progressive disclosure)
  │  │            Why: Load only when needed
  │  │            Example: Code analysis workflow
  │  │
  │  └─ NO (orchestration) → Skill
  │            Why: Skills orchestrate existing tools
  │            Example: Worktree manager calling git
```

**Context Impact**:

| Approach | Load Time | Use When |
|----------|-----------|----------|
| MCP Server | Immediate (all tools) | Every session |
| Skill | Progressive (on-demand) | Occasional |
| Command | None until invoked | Manual ops |

---

### When to Create ADW

**Question**: Build ADW or manual workflow?

```
Task automation needs?
  │
  ├─ Runs completely unattended (AFK)?
  │  ├─ YES → Build ADW
  │  │         Why: "Away From Keyboard" automation
  │  │         Example: PITER (Plan→Implement→Test→Execute→Report)
  │  │         Pattern: Python script with Anthropic SDK
  │  │
  │  └─ NO → Continue
  │
  ├─ Multiple AI calls with deterministic logic?
  │  ├─ YES → Build ADW
  │  │         Why: Mix agentic (AI) + deterministic (Python)
  │  │         Example: Director (generate→test→evaluate→loop)
  │  │         Structure: adws/ directory
  │  │
  │  └─ NO → Continue
  │
  ├─ Integrate with external systems (CI/CD, webhooks, cron)?
  │  ├─ YES → Build ADW
  │  │         Why: Python scripts integrate easily
  │  │         Example: GitHub webhook → ADW processes issue
  │  │         Trigger: Cron, webhook, manual
  │  │
  │  └─ NO → Continue
  │
  ├─ Performed frequently (daily/weekly)?
  │  ├─ YES → Build ADW or Skill
  │  │         ADW for: Unattended, external triggers
  │  │         Skill for: Agent-initiated, progressive disclosure
  │  │
  │  └─ NO → Manual workflow (slash commands)
  │            Why: One-off tasks don't justify overhead
  │            Pattern: Chain commands interactively
```

**When NOT to Build ADW**:
- One-time exploration
- Requires human judgment
- Simple single-step
- Changes frequently

---

## Architecture Maturity

### MVA vs Intermediate vs Advanced vs Production

**Question**: What architecture level do I need?

```
Project maturity?
  │
  ├─ Just validating concept?
  │  ├─ YES → MVA (Minimum Viable Architecture)
  │  │         Structure: Single file or simple directory
  │  │         Testing: Manual verification
  │  │         Docs: Inline comments
  │  │         Example: Proof-of-concept
  │  │
  │  └─ NO → Continue
  │
  ├─ Building for team or learning?
  │  ├─ YES → Intermediate Architecture
  │  │         Structure: Modular with separation
  │  │         Testing: Basic automated tests
  │  │         Docs: README + ai_docs/
  │  │         Components:
  │  │         - .claude/commands/
  │  │         - Modular code
  │  │         - Basic test suite
  │  │
  │  └─ NO → Continue
  │
  ├─ Production deployment planned?
  │  ├─ YES → Critical system?
  │  │  ├─ YES → Production Architecture
  │  │  │         Structure: Full observability + security
  │  │  │         Testing: Comprehensive suite
  │  │  │         Docs: Complete + runbooks
  │  │  │         Components:
  │  │  │         - .claude/hooks/
  │  │  │         - Comprehensive ai_docs/
  │  │  │         - CI/CD integration
  │  │  │         - Monitoring
  │  │  │
  │  │  └─ NO → Advanced Architecture
  │  │            Structure: Agentic layer + core separation
  │  │            Testing: Good coverage
  │  │            Docs: ai_docs/ + specs/
  │  │            Components:
  │  │            - adws/ workflows
  │  │            - .claude/agents/ specialists
  │  │            - specs/ planning
  │  │
  │  └─ NO → Intermediate
```

**Levels**:

| Level | Structure | Testing | Docs | Use Case |
|-------|-----------|---------|------|----------|
| MVA | Single/simple | Manual | Inline | Proof of concept |
| Intermediate | Modular | Basic auto | README + ai_docs/ | Learning, internal |
| Advanced | Agentic layer | Good coverage | ai_docs/ + specs/ | Serious projects |
| Production | Full observability | Comprehensive | Complete + runbooks | Critical systems |

**Key**: Don't over-architect early, but plan upgrade path.

---

### Monorepo vs Multi-Agent Worktrees

**Question**: Single repo or worktrees?

```
Parallel development needs?
  │
  ├─ Multiple agents simultaneously?
  │  ├─ YES → Git Worktrees
  │  │         Why: Isolated directories prevent conflicts
  │  │         Pattern: Main repo + N worktrees
  │  │         Example: Agent A (feature-x) + Agent B (feature-y)
  │  │         Setup:
  │  │         git worktree add ../wt-feature-A -b feature-A
  │  │         git sparse-checkout set apps/  # Reduce context
  │  │
  │  └─ NO → Continue
  │
  ├─ Need context isolation between features?
  │  ├─ YES → Git Worktrees
  │  │         Why: Independent context per worktree
  │  │         Example: Experimental refactor isolated
  │  │
  │  └─ NO → Continue
  │
  ├─ Sequential development (one at a time)?
  │  ├─ YES → Monorepo
  │  │         Why: Simpler, no worktree overhead
  │  │         Pattern: Single directory, feature branches
  │  │
  │  └─ NO → Evaluate complexity
```

**Benefits of Worktrees**:
- True parallelism (no file conflicts)
- Context isolation
- Easy cleanup
- Shared Git history

**When NOT to Use**:
- Solo sequential work
- Small projects
- Team unfamiliar with worktrees

---

## Pattern Selection

### Which Workflow Pattern

**Question**: How to orchestrate multiple steps?

```
Workflow complexity?
  │
  ├─ All steps in strict sequence?
  │  ├─ YES → Sequential Workflow
  │  │         Why: Step N+1 depends on Step N
  │  │         Example: Plan → Build → Test → Deploy
  │  │         Implementation: Numbered workflow steps
  │  │
  │  └─ NO → Continue
  │
  ├─ Steps run simultaneously (independent)?
  │  ├─ YES → Parallel Workflow
  │  │         Why: No dependencies, concurrent
  │  │         Example: Create 5 worktrees, scrape 10 URLs
  │  │         Implementation: Task tool with sub-agents
  │  │
  │  └─ NO → Continue
  │
  ├─ Path depends on runtime conditions?
  │  ├─ YES → Conditional Workflow
  │  │         Why: Branching logic
  │  │         Example: If tests pass → deploy, else → report
  │  │         Implementation: Level 3 prompt (If/Otherwise)
  │  │
  │  └─ NO → Continue
  │
  ├─ Repeat operation for multiple items?
  │  ├─ YES → Loop Workflow
  │  │         Why: Same operation, different inputs
  │  │         Example: Process each image, edit each file
  │  │         Implementation: <loop_prompt> tags or "For each"
  │  │
  │  └─ NO → Simple sequential
```

---

### Architect-Editor vs Direct Execution

**Question**: Separate planning from execution?

```
Context and complexity?
  │
  ├─ Task requires > 100K tokens context?
  │  ├─ YES → Architect-Editor
  │  │         Why: Context reduction via delegation
  │  │         Architect: High context, create spec
  │  │         Editor: Low context, execute spec
  │  │         Example: Large codebase refactoring
  │  │
  │  └─ NO → Continue
  │
  ├─ Significant architectural decisions?
  │  ├─ YES → Architect-Editor
  │  │         Why: Separate reasoning from implementation
  │  │         Architect: Reasoning model (O1, Opus)
  │  │         Editor: Fast model (Sonnet)
  │  │         Example: API design, database schema
  │  │
  │  └─ NO → Continue
  │
  ├─ Multiple agents implementing parts?
  │  ├─ YES → Architect-Editor
  │  │         Why: Single spec ensures consistency
  │  │         Architect: Unified plan
  │  │         Editors: Multiple parallel agents
  │  │         Example: Full-stack (backend + frontend)
  │  │
  │  └─ NO → Direct Execution
  │            Why: Single agent handles end-to-end
  │            Example: Bug fix, small feature
```

**When NOT to Use**:
- Simple, well-understood tasks
- Small context (< 50K tokens)
- Single-file changes
- Quick fixes

---

## Context Management

### Reduce or Delegate (R&D Framework)

**Question**: How to handle large context?

```
Context size?
  │
  ├─ Context < 50K tokens?
  │  ├─ YES → Use Full Context
  │  │         Why: Fits comfortably
  │  │         Pattern: Load all relevant files
  │  │
  │  └─ NO → Continue
  │
  ├─ Can context be strategically reduced?
  │  ├─ YES → REDUCE
  │  │         Techniques:
  │  │         - Context priming (load only needed)
  │  │         - Sparse checkout (worktrees)
  │  │         - Focused file selection
  │  │         - Strategic summarization
  │  │         - Output styles (reduce response)
  │  │         Example: Large codebase, focus on module
  │  │
  │  └─ NO (complex, not just large) → DELEGATE
  │            Techniques:
  │            - Architect-Editor pattern
  │            - Specialized sub-agents
  │            - Multi-agent coordination
  │            - Context bundling
  │            Example: Full-stack feature
  │                     Architect creates spec
  │                     Backend agent (low context)
  │                     Frontend agent (low context)
```

**Decision Matrix**:

| Scenario | Strategy | Technique |
|----------|----------|-----------|
| Large codebase, focused task | REDUCE | Context priming |
| Large codebase, full refactor | DELEGATE | Architect-Editor |
| Multiple independent features | DELEGATE | Sub-agents |
| Complex analysis | REDUCE | Summarization |
| Full-stack feature | DELEGATE | Specialized agents |

---

### When to Reset Context

**Question**: Start fresh agent session?

```
Context quality?
  │
  ├─ Context > 150K tokens?
  │  ├─ YES → Reset + Prime
  │  │         Why: Nearing limit
  │  │         Pattern: /reset → /prime → continue
  │  │
  │  └─ NO → Continue
  │
  ├─ Working on completely different area?
  │  ├─ YES → Reset + Prime
  │  │         Why: Previous context irrelevant
  │  │         Example: Switch frontend to backend
  │  │
  │  └─ NO → Continue
  │
  ├─ Agent confused or giving wrong answers?
  │  ├─ YES → Reset + Prime
  │  │         Why: Context pollution from errors
  │  │         Pattern: Reset → Prime clean slate
  │  │
  │  └─ NO → Continue
  │
  ├─ Starting new work session (after break)?
  │  ├─ YES → Prime (don't necessarily reset)
  │  │         Why: Refresh understanding
  │  │         Pattern: /prime to reload context
  │  │
  │  └─ NO → Continue with existing
```

---

## Model Selection

### Sonnet vs O1 (Fast vs Reasoning)

**Question**: Which model for this task?

```
Task characteristics?
  │
  ├─ Requires deep reasoning or complex planning?
  │  ├─ YES → O1 / Opus (Reasoning)
  │  │         Why: Extended thinking for complexity
  │  │         Use cases:
  │  │         - Architectural design
  │  │         - Complex debugging
  │  │         - Algorithm optimization
  │  │         - Security analysis
  │  │         - Strategic planning
  │  │
  │  └─ NO → Continue
  │
  ├─ Well-defined task with clear path?
  │  ├─ YES → Sonnet (Fast)
  │  │         Why: Fast, cost-effective
  │  │         Use cases:
  │  │         - Implementation from spec
  │  │         - Test writing
  │  │         - Refactoring
  │  │         - Documentation
  │  │         - CRUD operations
  │  │
  │  └─ NO → Continue
  │
  ├─ Iterative with frequent back-and-forth?
  │  ├─ YES → Sonnet
  │  │         Why: Faster iterations, lower cost
  │  │         Example: UI adjustments, incremental fixes
  │  │
  │  └─ NO → Consider task-specific needs
  │
  ├─ Cost-sensitive or high-volume?
  │  ├─ YES → Sonnet
  │  │         Why: More economical
  │  │         Example: Batch processing, maintenance
  │  │
  │  └─ NO → O1/Opus for quality
```

**Selection Matrix**:

| Task | Model | Rationale |
|------|-------|-----------|
| Architectural design | O1/Opus | Deep reasoning |
| Implementation from spec | Sonnet | Well-defined, fast |
| Complex debugging | O1/Opus | System behavior reasoning |
| Test writing | Sonnet | Straightforward |
| Security audit | O1/Opus | Critical, thorough |
| Refactoring | Sonnet | Mechanical transformation |
| API design | O1/Opus | Strategic decisions |
| Documentation | Sonnet | Standard writing |

**Cost-Performance**:
- Sonnet: ~1/10th cost of Opus, 95% capability for standard tasks
- O1/Opus: 10x cost, significantly better for reasoning

---

## Debugging Flowchart

### Three-Legged Stool (Context → Prompt → Model)

**Question**: AI coding failed - where's the problem?

```
Systematic debugging (check in order):
  │
  ├─ Step 1: CONTEXT (80% of issues)
  │  │
  │  ├─ All necessary files loaded?
  │  │  ├─ NO → Load missing files
  │  │  │        Fix: Use Read/Glob
  │  │  │
  │  │  └─ YES → Continue
  │  │
  │  ├─ In correct directory?
  │  │  ├─ NO → Navigate to correct location
  │  │  │        Fix: Bash cd
  │  │  │
  │  │  └─ YES → Continue
  │  │
  │  ├─ Has relevant documentation?
  │  │  ├─ NO → Add to ai_docs/
  │  │  │        Fix: Create or prime ai_docs/
  │  │  │
  │  │  └─ YES → Continue
  │  │
  │  └─ Context polluted with errors?
  │         ├─ YES → Reset + Prime
  │         │        Fix: /reset → /prime
  │         │
  │         └─ NO → Check Prompt
  │
  ├─ Step 2: PROMPT (15% of issues)
  │  │
  │  ├─ Clear and specific?
  │  │  ├─ NO → Refine prompt
  │  │  │        Fix: Add details, examples
  │  │  │
  │  │  └─ YES → Continue
  │  │
  │  ├─ Conflicting instructions?
  │  │  ├─ YES → Resolve conflicts
  │  │  │        Fix: Remove contradictions
  │  │  │
  │  │  └─ NO → Continue
  │  │
  │  ├─ Asking for impossible task?
  │  │  ├─ YES → Adjust expectations
  │  │  │        Fix: Break into smaller steps
  │  │  │
  │  │  └─ NO → Continue
  │  │
  │  └─ Output format specified?
  │         ├─ NO → Add Report section
  │         │        Fix: Specify expected output
  │         │
  │         └─ YES → Check Model
  │
  └─ Step 3: MODEL (5% of issues)
     │
     ├─ Task beyond model capability?
     │  ├─ YES → Upgrade model
     │  │        Fix: Sonnet → Opus/O1
     │  │
     │  └─ NO → Continue
     │
     ├─ Model hallucinating?
     │  ├─ YES → Provide more context/constraints
     │  │        Fix: Add examples, stricter instructions
     │  │
     │  └─ NO → Deeper investigation
     │
     └─ Model appropriate for complexity?
            ├─ NO → Adjust model selection
            │        Fix: Use model decision tree
            │
            └─ YES → Problem elsewhere (tooling, env)
```

**Priority Order**: Context (80%) → Prompt (15%) → Model (5%)

---

## Quick Reference

### Master Decision Matrix

| Scenario | Recommended | Key Factors |
|----------|-------------|-------------|
| External API | MCP Server | External service, tool exposure |
| Parallel tasks | Sub-agents | Keyword "parallel", independent |
| Multi-step automation | Skill | Agent-triggered, multiple ops |
| One-off manual task | Slash Command | Manual trigger, single op |
| Unattended automation | ADW | AFK execution, external triggers |
| Block dangerous ops | PreToolUse Hook | Security, validation |
| Log all tool usage | PostToolUse Hook | Observability, audit |
| Large context (> 100K) | Architect-Editor | Context reduction via delegation |
| Well-defined feature | Spec-based | Clear requirements, one-shot |
| Exploratory task | Iterative Prompting | Uncertain requirements |
| Deep reasoning | O1/Opus Model | Complex planning, architecture |
| Standard implementation | Sonnet Model | Well-defined, fast execution |
| Parallel features | Git Worktrees | Multiple agents, context isolation |
| AI not finding files | Context Issue (80%) | Missing files, wrong directory |

### Context Management Guide

| Size | Action | Technique |
|------|--------|-----------|
| < 50K tokens | Full context | No optimization |
| 50-100K | REDUCE | Priming, focused files |
| 100-150K | REDUCE OR DELEGATE | Architect-Editor OR Sub-agents |
| > 150K | RESET + DELEGATE | Reset context, use delegation |

### Debugging Quick Reference

**80-15-5 Rule**: Context (80%) → Prompt (15%) → Model (5%)

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| Can't find files | Context (missing) | Read to load files |
| Wrong directory | Context (navigation) | cd /correct/path |
| Generic responses | Prompt (too vague) | Add examples |
| Repeated errors | Context (pollution) | /reset → /prime |
| Wrong style | Context (no examples) | Add style guide to ai_docs/ |
| Hallucinations | Model (limitations) | Add tests, verification |

---

**Source**: framework-decision-trees.md (compressed 1,618 → 400 lines)
**Last Updated**: 2025-10-31
**Lines**: ~400
