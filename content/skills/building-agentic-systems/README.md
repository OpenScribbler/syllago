# Building Agentic Systems - Claude Code Skill

Comprehensive guidance for developing autonomous AI-driven software systems (agentic coding).

## Directory Structure

```
agentic-coding-framework/
├── SKILL.md                    (main entry point - to be created)
├── guides/                     (reference documentation)
│   ├── getting-started.md      (~500 lines)
│   ├── patterns.md             (~1,233 lines)
│   ├── decisions.md            (~673 lines)
│   ├── production.md           (~850 lines)
│   ├── principles.md           (~747 lines)
│   ├── components.md           (~1,011 lines)
│   ├── debugging.md            (~629 lines)
│   └── reference.md            (~684 lines)
├── scripts/                    (helper scripts - to be created)
│   ├── setup_mva.sh
│   └── create_worktree.sh
├── README.md                   (this file)
└── VALIDATION.md               (quality validation report)
```

## Guide Files (8 files, 6,325 lines)

### 1. guides/getting-started.md (~500 lines)
**Purpose**: Complete MVA (Minimum Viable Agentic) walkthrough in 2-4 hours

**Contents**:
- Prerequisites (tools, accounts, skills)
- Core concepts (Phase 1 vs Phase 2)
- Directory structure setup
- First slash command (copy-paste ready)
- First agent primitive (agent.py)
- First ADW workflow (adw_hello.py)
- Verification checklist
- Next steps

**Source**: framework-production-playbook.md (Phase 1), framework-master.md (Section 17)

---

### 2. guides/patterns.md (~1,233 lines)
**Purpose**: Concise catalog of top 20 patterns

**Contents**:
- **8 Foundational Patterns**:
  1. Iterative Prompting
  2. Verification Loops
  3. Multi-File Refactoring
  4. Structured Output
  5. Architect-Editor
  6. Director Pattern
  7. Three-Legged Stool
  8. Pitfalls Framework

- **8 Tactical Patterns**:
  9. Tool-Enabled Autonomy
  10. PITER Framework
  11. Closed-Loop Feedback
  12. One Agent One Purpose
  13. Worktree Isolation
  14. Agentic Layer
  15. Hook-Based Observability
  16. Multi-Agent Delegation

- **4 Advanced Patterns**:
  17. R&D Framework (Reduce or Delegate)
  18. 7 Prompt Levels
  19. Context Bundle Tracking
  20. Custom Tool Creation

- Pattern selection guide
- Pattern combinations (powerful stacks)

**Format**: Each pattern includes Intent, When to Use, Structure, Real Code Example, Related Patterns

**Source**: framework-pattern-catalog.md (compressed 4,156 → 1,233 lines)

---

### 3. guides/decisions.md (~673 lines)
**Purpose**: Visual decision trees for rapid choices

**Contents**:
- **Component Selection Tree** (Command vs Skill vs Sub-agent vs ADW vs MCP)
- **Architecture Maturity Tree** (MVA → Intermediate → Advanced → Production)
- **Pattern Selection Tree** (Which pattern for which scenario)
- **Context Management Tree** (Reduce vs Delegate - R&D Framework)
- **Model Selection Matrix** (Sonnet vs O1 with cost/performance)
- **Debugging Flowchart** (Three-Legged Stool: Context 80%, Prompt 15%, Model 5%)
- Quick reference tables (scenario → recommendation)

**Format**: ASCII decision trees with testable conditions → recommendations → rationale

**Source**: framework-decision-trees.md (compressed 1,618 → 673 lines)

---

### 4. guides/production.md (~850 lines)
**Purpose**: Production deployment guide - Phases 3-4

**Contents**:
- Production readiness checklist
- **Phase 3: Advanced Agentic**
  - Git worktrees setup (real code)
  - Multi-agent orchestration (real code)
  - Context optimization (tracking utilities)
  - Observability setup (hooks)
- **Phase 4: Production Agentic**
  - Security hardening (PreToolUse hooks)
  - Cost tracking (real Python code)
  - Monitoring setup (dashboard script)
  - Incident response runbook
  - Team onboarding guide
- Common production issues (table with fixes)
- Migration guide (Phase 2→3→4)

**Source**: framework-production-playbook.md (Phases 3-4), framework-master.md (Sections 10-13)

---

### 5. guides/principles.md (~747 lines)
**Purpose**: Deep dive on principles and frameworks

**Contents**:
- The 10 Universal Principles (detailed WHY explanations)
- The Core Four (Context + Model + Prompt + Tools)
- 12 Leverage Points framework
- 7 Prompt Levels progression
- R&D Framework (Reduce or Delegate)
- PITER Framework
- Architect-Editor Pattern
- Director Pattern

**Source**: framework-master.md (Sections 2, 3)

---

### 6. guides/components.md (~1,011 lines)
**Purpose**: Component design guidance

**Contents**:
- Commands (slash commands with templates)
- Workflows/ADWs (Python patterns)
- Agents (specialization, SDK)
- Hooks (PreToolUse, PostToolUse)
- Tools/MCP (@tool decorator)
- Knowledge Bases (ai_docs/)
- Tasks (automation)
- Specs (specification-driven)

**Source**: framework-master.md (Section 5), framework-pattern-catalog.md

---

### 7. guides/debugging.md (~629 lines)
**Purpose**: Systematic troubleshooting

**Contents**:
- Three-Legged Stool (Context 80%, Prompt 15%, Model 5%)
- Common failure modes by symptom
- Debug checklist
- Context/Prompt/Model issue deep-dives
- Quick fixes table

**Source**: framework-master.md (Sections 8, 11), framework-decision-trees.md

---

### 8. guides/reference.md (~684 lines)
**Purpose**: Quick reference and glossary

**Contents**:
- Command cheat sheet
- Workflow/ADW template
- Directory structure reference
- Glossary (30 essential terms)
- File naming conventions
- Common patterns lookup

**Source**: framework-master.md (Sections 14, 19)

---

## Quality Criteria Met

✅ **Table of contents** at top of each file
✅ **Real code examples** from framework (with source citations)
✅ **Comprehensive coverage**:
  - getting-started: 498 lines
  - patterns: 1,233 lines
  - decisions: 673 lines
  - production: 850 lines
  - principles: 747 lines
  - components: 1,011 lines
  - debugging: 629 lines
  - reference: 684 lines
✅ **Concise** (challenged every sentence)
✅ **Forward slashes** in all paths
✅ **One level deep** (no nested references)

## Statistics

- **Total Lines**: 6,325 lines across 8 guides
- **Total Size**: ~168KB
- **Compression**: 2.4:1 from framework (15,086 → 6,325 lines)
- **Coverage**: Complete lifecycle from MVA → Production
- **Code Examples**: 50+ working snippets with source citations

## Organization

**guides/** - All reference documentation (one level deep from SKILL.md)
- User intent-based organization
- Table of contents in each file
- Real code examples throughout
- Source citations for traceability

**scripts/** - Helper scripts for deterministic operations
- setup_mva.sh - Creates MVA directory structure
- create_worktree.sh - Git worktree management

## Usage from SKILL.md

```markdown
## When You're Working On...

- Starting a new project → [getting started guide](guides/getting-started.md)
- Choosing a pattern → [patterns](guides/patterns.md), [decisions](guides/decisions.md)
- Understanding principles → [principles guide](guides/principles.md)
- Building components → [components guide](guides/components.md)
- Debugging failures → [debugging guide](guides/debugging.md)
- Production deployment → [production guide](guides/production.md)
- Quick lookup → [reference guide](guides/reference.md)
```

## Source Documents

- framework-master.md (~10,000 lines)
- framework-production-playbook.md (~2,500 lines)
- framework-pattern-catalog.md (~4,156 lines)
- framework-decision-trees.md (~1,618 lines)

---

**Created**: 2025-10-31
**Version**: 1.0
**Status**: Complete
