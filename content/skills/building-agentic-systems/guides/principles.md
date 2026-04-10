# Agentic Coding Principles and Frameworks

Deep dive on core principles, mental models, and frameworks for building agentic systems.

## Table of Contents

1. [The 10 Universal Principles](#the-10-universal-principles)
2. [The Core Four Framework](#the-core-four-framework)
3. [12 Leverage Points](#12-leverage-points)
4. [7 Prompt Levels](#7-prompt-levels)
5. [R&D Framework](#rd-framework-reduce-or-delegate)
6. [PITER Framework](#piter-framework)
7. [Architect-Editor Pattern](#architect-editor-pattern)
8. [Director Pattern](#director-pattern)

---

## The 10 Universal Principles

### 1. Incremental Everything

**Definition**: All work happens through small, verifiable increments. Never combine multiple changes.

**Why It Works**: Compound errors grow exponentially. If you change 5 things and something breaks, you must check 5 things and their interactions (25 combinations). One change = one thing to check.

**What Happens If Violated**: Debugging becomes detective work, you can't pinpoint failures, rollback means "undo everything".

**Code Example**:
```python
# GOOD: One feature per prompt
# Prompt 1: "Add error handling for network requests"
# Prompt 2: "Add retry logic with exponential backoff"
# Prompt 3: "Add logging for retry attempts"

# BAD: Everything at once
# "Add error handling, retry logic, logging, and metrics tracking"
```

### 2. Verification at Every Level

**Definition**: Test and verify after every change before proceeding. Feedback loops are non-negotiable.

**Why It Works**: Early detection is exponentially cheaper. Agents learn from feedback - without loops they can't self-correct.

**What Happens If Violated**: Errors compound silently, agents build on broken foundations, root cause analysis becomes archaeological.

**Code Example**:
```python
# GOOD: Director pattern with verification loop
def director_loop(max_iterations=5):
    for i in range(max_iterations):
        code = coder_agent.generate()
        results = run_tests(code)
        if results.all_passed:
            return code
        feedback = evaluator_agent.analyze(results)
        coder_agent.update_prompt(feedback)
    return None

# BAD: No verification
code = agent.generate()
deploy(code)  # Hope it works!
```

### 3. Natural Language as Interface

**Definition**: Express intent in plain language - the WHAT and WHY. Let agents determine HOW.

**Why It Works**: Natural language is more maintainable than code. "Add error handling for database timeouts" is clearer than 50 lines of try/catch/retry code. Prompts are the fundamental unit of knowledge work.

**What Happens If Violated**: Over-specification constrains agents unnecessarily, prompts become brittle pseudo-code.

**Code Example**:
```markdown
# GOOD: Natural language spec
Create a user authentication endpoint that:
- Accepts email and password via POST
- Returns JWT token on success
- Returns 401 on invalid credentials
- Logs all authentication attempts

# BAD: Over-specified pseudo-code
Create function authenticate_user(email: str, password: str):
  user = db.query(User).filter(email=email).first()
  if not user: return 401
  if not bcrypt.verify(password, user.hashed_password): return 401
  ...
```

### 4. Structure Emerges from Iteration

**Definition**: Don't design architecture upfront. Build working code first, then refactor to reveal patterns.

**Why It Works**: Premature abstraction creates wrong abstractions. Building first reveals actual needs. Refactoring is cheap with AI.

**What Happens If Violated**: Over-engineered solutions for problems you don't have, abstractions that don't fit actual usage.

**Code Example**:
```python
# GOOD: Emergent architecture (PAICC-2 pattern)
# Step 1: Single-file working script (main.py)
# Step 2: Extract arg parsing → arg_parse.py
# Step 3: Extract constants → constants.py
# Structure emerges from refactoring working code

# BAD: Upfront design
# Create full directory structure before writing code:
# src/domain/entities/repositories/services/infrastructure/application/
# ...then discover you don't need half of it
```

### 5. Measurement-Driven Decisions

**Definition**: Measure everything. Use data to guide optimization, not intuition.

**Why It Works**: Intuition fails at scale. Optimization requires baselines. Metrics resolve debates.

**What Happens If Violated**: Blind optimization, can't prove improvements, cost spirals unnoticed.

**Code Example**:
```python
# GOOD: Context bundle tracking (from Context Engineering)
{
  "timestamp": "2025-10-30T10:15:00Z",
  "context_items": [
    {"file": "main.py", "tokens": 523},
    {"file": "README.md", "tokens": 234},
    {"doc": "architecture.md", "tokens": 891}
  ],
  "total_tokens": 1648,
  "success": true
}

# Real Example from TAC-2:
# KPI: Iterations to completion
# Baseline = 3.2 iterations average
# After optimization = 1.4 iterations (56% reduction)
# Proof of improvement: Data, not opinions
```

### 6. One-Shot as Aspiration

**Definition**: Reduce iteration through upfront leverage stacking. Goal: agent executes correctly on first try.

**Why It Works**: Iteration has costs - time, API calls, context switching. Stack leverage points for one-shot success.

**What Happens If Violated**: Not a violation (iteration is valid), but missed opportunity for efficiency.

**Code Example**:
```markdown
# GOOD: Stacking leverage for one-shot success
1. Context: Comprehensive specs loaded
2. Model: O1 for planning, Sonnet for execution
3. Prompt: Detailed workflow with examples
4. Tools: All necessary tools granted
5. Documentation: ai_docs/ explain architecture
6. Types: Pydantic models define contracts
7. Tests: Fixtures provide validation
8. Planning: High-level spec broken into tasks
9. Specs: 750+ line requirement doc
10. ADWs: Automated orchestration
11. Trees: Clear project structure
12. KPIs: Success criteria defined upfront

Result: Agent completes on first run
```

### 7. Agent Perspective - Brilliant but Blind

**Definition**: Design for agents' strengths and limitations. They're capable within context window but can't see outside it.

**Why It Works**: Agents don't "just know" where files are. Optimize for what they CAN see.

**What Happens If Violated**: Agents fail on "obvious" tasks, wasted context, hallucinations filling gaps.

**Code Example**:
```markdown
# GOOD: Agent-specific documentation (ai_docs/architecture.md)
## Database Layer
Our app uses PostgreSQL via SQLAlchemy ORM.
- Models defined in: app/models/
- Migrations in: alembic/versions/
- Connection config: app/database.py

To add a new model:
1. Create file in app/models/
2. Import in app/models/__init__.py
3. Run: alembic revision --autogenerate
4. Run: alembic upgrade head

# BAD: Human-only documentation
# "The database stuff is in the usual place"
# Agent has no idea what "usual place" means
```

### 8. Feedback Loops Everywhere

**Definition**: Always add verification, testing, and validation loops. Agents without feedback guess once; with feedback they iterate toward correctness.

**Why It Works**: Value difference is exponential. No feedback = linear value (one shot). Feedback = exponential value (compounding improvement).

**What Happens If Violated**: Agents can't self-validate, no path to improvement, human becomes bottleneck.

**Code Example**:
```python
# GOOD: Director pattern with feedback loop
class Director:
    def develop(self, spec, max_iterations=10):
        for i in range(max_iterations):
            code = self.coder.generate(spec)
            test_results = self.run_tests(code)

            if test_results.all_passed:
                return code

            # Feedback loop: Evaluator analyzes failures
            feedback = self.evaluator.analyze(test_results)
            spec = self.enhance_spec(spec, feedback)

        return None  # Failed after max iterations

# Real Example from TAC-5:
# GitHub Issue Automation: Issue → Agent implements → Tests run
# → If fail, agent debugs → Repeat until pass → Create PR
# Result: 80% of issues resolved without human intervention
```

### 9. Specialization Beats Generalization

**Definition**: Focused agents with minimal, relevant context outperform generalists. Single-purpose beats multi-purpose.

**Why It Works**: Every additional responsibility adds noise. Noise drowns signal. Focused agents process faster and more accurately.

**What Happens If Violated**: Slow execution, lower accuracy, harder debugging, can't parallelize.

**Code Example**:
```python
# GOOD: Specialized agents
# Architect agent (O1, high context)
architect = Agent(
    model="o1-preview",
    context=["specs/", "ai_docs/architecture.md"],
    purpose="Design architecture and create implementation plan"
)

# Editor agent (Sonnet, low context)
editor = Agent(
    model="claude-sonnet-4",
    context=["single_file_to_edit.py"],
    purpose="Implement specific changes to one file"
)

# Real Example from TAC-6:
# Before: One agent for frontend + backend (40K token context)
# After: Frontend agent (8K tokens) + Backend agent (12K tokens)
# Results: 60% context reduction, 3x faster, 40% fewer errors
```

### 10. Prompts are Primitive

**Definition**: Prompts are the fundamental unit of knowledge work. They deserve versioning, documentation, testing, evolution.

**Why It Works**: In agentic systems, prompts ARE the program. They encode domain expertise, workflow logic, institutional knowledge.

**What Happens If Violated**: Knowledge loss, repeated work, no compounding, can't debug or improve.

**Code Example**:
```markdown
# GOOD: Versioned, documented prompt template
# .claude/commands/features/implement-feature.md

## Metadata
version: 2.1.0
author: team
updated: 2025-10-30

## Variables
feature_name: $1
spec_file: specs/feature-${feature_name}.md

## Instructions
1. READ the spec file: ${spec_file}
2. PLAN implementation phases
3. IMPLEMENT each phase incrementally
4. TEST after each phase
5. REPORT: Success criteria met (yes/no)

## Expertise (accumulated over 50 runs)
- Always check for existing similar implementations
- Frontend changes require updating TypeScript types
- Backend changes require database migration check

# BAD: Throwaway prompt
# "Add the feature from that spec we talked about"
# No versioning, no reusability, no improvement
```

---

## The Core Four Framework

The four always-present pillars of agentic development. Every agent interaction involves all four.

```
┌─────────────────────────────────────────┐
│           AGENT CAPABILITY              │
├─────────────────────────────────────────┤
│  1. CONTEXT     ←→  What can it see?    │
│  2. MODEL       ←→  How smart is it?    │
│  3. PROMPT      ←→  What to do?         │
│  4. TOOLS       ←→  What can it do?     │
└─────────────────────────────────────────┘
```

**Debugging Order** (when agents fail):
1. **Context first** (80% of failures) - Missing files? Wrong directory?
2. **Prompt second** (15% of failures) - Unclear intent? Conflicting instructions?
3. **Model third** (4% of failures) - Wrong model for task?
4. **Tools last** (1% of failures) - Tool access denied?

**Application Example**:
```markdown
Agent fails to implement authentication:

Check Context:
- ✓ Has access to user model file
- ✓ Has access to database config
- ✗ MISSING: Examples of existing auth patterns in codebase

Fix: Add ai_docs/authentication.md explaining existing patterns
Result: Agent succeeds on next attempt
```

**Source**: TAC-1 (Core Four framework)

---

## 12 Leverage Points

Systematic framework for maximizing agent autonomy and one-shot success.

### In-Agent Leverage (Core Four)

1. **Context** - What agent can see (files, docs, conversation)
2. **Model** - Intelligence capabilities (O1 for reasoning, Sonnet for speed)
3. **Prompt** - Instructions and format (workflow prompts, templates)
4. **Tools** - Available actions (Bash, Read, Write, Edit, custom MCP)

### Through-Agent Leverage

5. **Documentation** - ai_docs/ directory for agent knowledge
6. **Types** - Synchronized contracts (Pydantic ↔ TypeScript)
7. **Tests** - Self-validation with fixtures
8. **Planning** - Specs before execution
9. **Specs** - Comprehensive requirements documents
10. **ADWs** - Agentic Developer Workflows (automation scripts)
11. **Trees** - File/folder organization clarity
12. **KPIs** - Success metrics

**Leverage Stacking Example** (from TAC-2):
```markdown
Task: Build natural language SQL interface

Leverage Applied:
1. Context: Database schema, example queries, API structure
2. Model: O1 for architecture, Sonnet for implementation
3. Prompt: 750-line comprehensive spec
4. Tools: Full Bash, Read, Write, Edit access
5. Documentation: ai_docs/architecture.md, ai_docs/database.md
6. Types: Pydantic models for SQL queries and responses
7. Tests: 15 test fixtures covering all query types
8. Planning: High-level phases defined
9. Specs: Complete requirements with examples
10. ADWs: adw_plan_build.py orchestration
11. Trees: Clear separation of frontend/backend/shared
12. KPIs: All tests pass, API responds < 200ms

Result: One-shot success - agent builds entire app without iteration
```

**Source**: TAC-2 (12 Leverage Points)

---

## 7 Prompt Levels

Prompts evolve from simple to sophisticated. Each level adds capability.

### Level 1: High-Level Prompt
- Simple reusable instructions
- Title + Purpose + The Prompt
- Example: "Review code for security issues"

### Level 2: Workflow Prompt
- Input → Work → Output structure
- Most common pattern for slash commands
- Sections: Variables, Instructions, Report
- Example: `/plan` command that creates specs

### Level 3: Control Flow
- Adds conditions and loops
- Enables dynamic decision-making
- Uses IF/THEN, WHILE, FOR structures
- Example: Conditional error handling workflows

### Level 4: Delegate Prompt
- Orchestrates other agents
- Spawns sub-agents or primary agents
- Manages multi-agent workflows
- Example: `/background` command spawning parallel agent

### Level 5: Higher-Order Prompt
- Accepts prompts as input
- Meta-level abstraction
- Enables prompt composition
- Example: `/load_bundle` accepts bundle + prompt to execute

### Level 6: Template Metaprompt
- Generates new prompts dynamically
- Uses Template section
- Creates prompts on the fly
- Example: `/feature` command generating implementation prompts

### Level 7: Self-Improving Prompt
- Accumulates expertise over time
- Expertise section updated after each run
- Compound learning
- Example: Hook expert that learns from each project

**Progression Decision**:
```
What do you need?
  → Reusable instruction → Level 1
  → Multi-step workflow → Level 2
  → Sequential with decisions → Level 3
  → Multiple agents → Level 4
  → Prompt composition → Level 5
  → Generate prompts dynamically → Level 6
  → Learn and improve over time → Level 7
```

**Source**: Prompt Engineering (Agentic Horizon)

---

## R&D Framework (Reduce or Delegate)

There are only TWO ways to manage context effectively:

### 1. Reduce - Minimize tokens strategically
### 2. Delegate - Offload to sub-agents

```
Context Problem: Agent context window approaching limit

        ┌─────────────┐
        │   PROBLEM   │
        └──────┬──────┘
               │
        ┌──────┴──────┐
        │             │
   ┌────▼────┐   ┌────▼────┐
   │ REDUCE  │   │DELEGATE │
   └────┬────┘   └────┬────┘
        │             │
   ┌────▼────┐   ┌────▼────┐
   │ Remove  │   │ Sub-    │
   │ files   │   │ agents  │
   │ Compact │   │ Primary │
   │ output  │   │ agents  │
   │ Prime   │   │ Arch-   │
   │ context │   │ Editor  │
   └─────────┘   └─────────┘
```

**The 12 Techniques**:

**Beginner**:
1. Measure Context - Use /context command, track token usage
2. Avoid MCP Bloat - Don't enable MCP servers you don't need
3. Context Priming - Use slash commands vs giant files
4. Output Styles - Reduce output tokens with concise mode

**Intermediate**:
5. Sub-Agents - Spawn agents for isolated tasks
6. Architect-Editor - Separate planning (high context) from execution (low context)
7. Selective File Access - Only include files agents need
8. Context Bundles - Track exact usage with JSONL logs

**Advanced**:
9. Reset + Prime - Fresh agent with targeted context priming
10. One Agent One Purpose - Specialized agents with minimal context
11. Background Agents - Offload non-critical work to separate sessions
12. Primary Agent Delegation - Multiple independent agents for parallel work

**Decision Tree**:
```
Context window filling up?
  → Can you remove files/docs?
    → Yes: REDUCE (techniques 1-4)
    → No: Check if task is decomposable
      → Yes: DELEGATE (techniques 5-12)
      → No: You need a bigger model
```

**Source**: Context Engineering (Agentic Horizon)

---

## PITER Framework

Framework for "Away From Keyboard" (AFK) autonomous development:

**P**lan → **I**mplement → **T**est → **E**xecute → **R**eport

```
┌──────────┐
│   PLAN   │  Generate comprehensive spec
└────┬─────┘
     │
┌────▼────────┐
│ IMPLEMENT   │  Execute autonomously
└────┬────────┘
     │
┌────▼────┐
│  TEST   │  Run validation
└────┬────┘
     │
┌────▼────────┐
│  EXECUTE    │  Deploy/apply changes
└────┬────────┘
     │
┌────▼────────┐
│  REPORT     │  Communicate results
└─────────────┘
```

**When to Use PITER**:
- AFK coding (agent works while you're away)
- GitHub issue automation (issue → PR pipeline)
- Cron-triggered workflows (scheduled tasks)
- Any workflow requiring complete autonomy

**In-Loop vs Out-Loop**:

**In-Loop** (Interactive):
- High human presence
- Conversational iteration
- Real-time feedback
- Use when: Novel problems, subjective decisions, learning

**Out-Loop / AFK** (Autonomous):
- Low human presence
- Autonomous iteration
- Automated feedback (tests)
- Use when: Known patterns, objective criteria, scale

**PITER Implementation Example**:
```python
# ADW (Agentic Developer Workflow)
def piter_workflow(issue_number):
    # PLAN
    spec = plan_agent.create_spec(issue_number)

    # IMPLEMENT
    code = implementation_agent.execute(spec)

    # TEST
    results = run_tests()
    if not results.passed:
        code = debug_agent.fix(code, results)

    # EXECUTE
    git.commit(code)
    git.push()

    # REPORT
    pr = github.create_pr(code, spec)
    return pr.url
```

**Source**: TAC-4 (PITER Framework)

---

## Architect-Editor Pattern

Separate high-context planning from low-context execution using two specialized agents.

**Architect** (High Context, Reasoning Model):
- Uses O1 or similar reasoning model
- Has access to full context (specs, docs, architecture)
- Creates comprehensive implementation plans
- Outputs: Detailed specs with exact file operations

**Editor** (Low Context, Fast Model):
- Uses Sonnet or similar fast model
- Has access only to files being edited
- Executes specific changes from plan
- Outputs: Implemented code changes

**Why This Works**:
- **Context efficiency**: Editor doesn't need architectural context
- **Speed**: Fast model for execution (cheaper, faster)
- **Quality**: Reasoning model for complex planning
- **Specialization**: Each agent optimized for its task

```
High Context          Low Context
High Reasoning        High Speed
    ↓                     ↓
┌───────────┐        ┌─────────┐
│ ARCHITECT │──plan─→│ EDITOR  │
│  (O1)     │        │(Sonnet) │
└───────────┘        └─────────┘
     │                    │
  Design              Implement
  Explore             Execute
  Reason              Fast
```

**Application Example**:
```markdown
Task: Add user authentication system

Architect Phase:
- Model: o1-preview
- Context: Full codebase, architecture docs, existing auth examples
- Output: 500-line spec detailing:
  - Database schema changes
  - API endpoint implementations
  - Frontend form components
  - Test requirements
  - Security considerations

Editor Phase:
- Model: claude-sonnet-4
- Context: Only files being modified (one at a time)
- Input: Architect's spec for specific file
- Output: Implemented changes to that file

Result: High-quality design + fast execution
```

**Source**: PAICC-5 (Architect-Editor), Context Engineering

---

## Director Pattern

Agent evaluates its own work and iterates autonomously:

```
┌─────────────────────────────────┐
│   DIRECTOR (Orchestrator)       │
└────────┬────────────────────────┘
         │
    ┌────▼─────┐
    │  LOOP    │
    └────┬─────┘
         │
    ┌────▼────────────────────────┐
    │ 1. CODER generates code     │
    │ 2. TESTS run automatically  │
    │ 3. EVALUATOR analyzes       │
    │ 4. FEEDBACK generated       │
    │ 5. Repeat until success     │
    └─────────────────────────────┘
```

**Components**:
- **Coder** (Fast model): Generates code from spec
- **Test Runner** (Automated): Executes validation
- **Evaluator** (Reasoning model): Analyzes results
- **Director** (Orchestrator): Manages loop, updates prompts

**Why This Works**:
- **Closed-loop feedback**: Tests provide objective validation
- **Self-correction**: Evaluator generates improvement prompts
- **Autonomous iteration**: No human in the loop
- **Dual models**: Fast for generation, reasoning for evaluation

**Real Code Example**:
```python
class Director:
    def __init__(self):
        self.coder = Coder.create(main_model="claude-sonnet-4")
        self.evaluator = Coder.create(main_model="o1-preview")

    def develop(self, spec, max_iterations=10):
        prompt = spec
        for i in range(max_iterations):
            # Generate
            self.coder.run(prompt)

            # Test
            results = pytest.run()

            # Evaluate
            if results.all_passed:
                return "Success"

            # Feedback
            feedback = self.evaluator.run(f"""
                Tests failed: {results.failures}
                Analyze and suggest improvements.
            """)

            # Update prompt
            prompt = f"{spec}\n\nPrevious attempt feedback:\n{feedback}"

        return "Failed after max iterations"
```

**Source**: PAICC-7 (Director Pattern)

---

## Summary

These principles and frameworks form the foundation of effective agentic coding:

**Principles**: The 10 Universal rules guide every decision
**Core Four**: Context, Model, Prompt, Tools - always optimize these first
**12 Leverage Points**: Stack systematically for one-shot success
**7 Prompt Levels**: Progress from simple to self-improving
**R&D Framework**: Reduce or Delegate to manage context
**PITER**: Plan-Implement-Test-Execute-Report for autonomy
**Architect-Editor**: Separate planning from execution
**Director**: Self-evaluating autonomous loops

**Progression Path**:
```
Start: Iterative Prompting + Verification
  ↓
Add: Structured Output + Context Enrichment
  ↓
Scale: Architect-Editor for complex features
  ↓
Automate: Director Pattern for full autonomy
```

**Sources**:
- PAICC lessons 1-7 (framework-master.md sections 2-3)
- TAC lessons 1-8 (framework-pattern-catalog.md)
- Agentic Horizon (framework-decision-trees.md sections 5-6)
- Production Playbook (framework-production-playbook.md phases 1-4)
