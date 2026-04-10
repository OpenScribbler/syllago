# Agentic Coding Patterns

Concise catalog of top 20 patterns for building agentic systems.

---

## Table of Contents

1. [How to Use](#how-to-use)
2. [Foundational Patterns](#foundational-patterns)
3. [Tactical Patterns](#tactical-patterns)
4. [Advanced Patterns](#advanced-patterns)
5. [Pattern Selection](#pattern-selection)
6. [Pattern Combinations](#pattern-combinations)

---

## How to Use

**Pattern Format**:
- **Intent**: What problem it solves (1 line)
- **When to Use**: Specific scenarios (3-5 bullets)
- **Structure**: Core components
- **Example**: Real code from framework
- **Related**: Links to complementary patterns

**Finding Patterns**: Match your scenario to "When to Use"
**Combining Patterns**: See Pattern Combinations section

---

## Foundational Patterns

Master these before tactical/advanced patterns.

### 1. Iterative Prompting

**Intent**: Build through incremental natural language instructions, one feature at a time.

**When to Use**:
- Starting any new feature
- Learning unfamiliar APIs
- Maintaining tight verification loops
- Collaborating with AI assistants

**Structure**:
```
1. Express intent in natural language
2. Review proposed changes
3. Apply changes
4. Commit to version control
5. Verify functionality
6. Iterate to next feature
```

**Example**:
```bash
# Step 1: Start simple
Prompt: "print hello world"
→ Verify: python main.py
→ Commit: "feat: add hello world"

# Step 2: Add feature
Prompt: "print 10 times"
→ Verify: outputs 10 times
→ Commit: "feat: add loop"

# Step 3: Introduce variables
Prompt: "store string in variable"
→ Verify: same output
→ Commit: "refactor: extract variable"
```

**Related**: Verification Loops, Multi-File Refactoring

**Source**: PAICC-1

---

### 2. Verification Loops

**Intent**: Validate code after every AI change before proceeding.

**When to Use**:
- After every code modification
- Before committing to version control
- When building on previous changes
- In production workflows

**Structure**:
```python
def development_cycle():
    1. AI generates code
    2. Review changes
    3. Apply changes
    4. RUN CODE IMMEDIATELY  # Critical
    5. Observe output
    6. If success → commit
    7. If failure → /undo and refine
```

**Example**:
```bash
# After AI modifies code
> python main.py

# Verify expected output
hello world
hello world
# ... (10 times)

# Success → commit
> git commit -m "feat: add loop"

# Next iteration builds on verified foundation
```

**Related**: Iterative Prompting, Closed-Loop Feedback

**Source**: PAICC-1, PAICC-2

---

### 3. Multi-File Refactoring

**Intent**: Organize code into modular files through incremental extraction.

**When to Use**:
- File exceeds ~200 lines
- Multiple concerns in one file
- Preparing for team collaboration
- Enabling parallel agent workflows

**Structure**:
```
Single-File Monolith
  ↓
Identify Concern (e.g., "CLI parsing")
  ↓
Extract to Module ("create arg_parse.py")
  ↓
Verify Imports
  ↓
Commit
  ↓
Repeat
```

**Example**:
```bash
# Step 1: Extract argument parsing
Prompt: "create arg_parse.py, move CLI parsing there"

# AI creates:
# arg_parse.py
import argparse

def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("file")
    return parser.parse_args()

# AI updates main.py:
from arg_parse import parse_args
args = parse_args()

→ Verify: python main.py file.txt
→ Commit: "refactor: extract arg parsing"

# Step 2: Extract constants
Prompt: "create constants.py, move STOPWORDS there"
→ Verify, Commit

# Step 3: Extract data types
Prompt: "create data_types.py for Pydantic models"
→ Verify, Commit
```

**Related**: Iterative Prompting, Verification Loops

**Source**: PAICC-2

---

### 4. Structured Output

**Intent**: Define Pydantic schemas for LLM responses ensuring type-safe, validated data.

**When to Use**:
- LLM output feeds programmatic logic
- Building production systems
- Need specific data types (lists, nested objects)
- Type synchronization across languages

**Structure**:
```python
# 1. Define contract
class Result(BaseModel):
    field_1: str
    field_2: List[str]

# 2. Generate schema
schema = Result.model_json_schema()

# 3. Include in prompt
response = llm.call(
    prompt="...",
    response_format={"type": "json_schema", "json_schema": {"schema": schema}}
)

# 4. Parse and validate
result = Result(**json.loads(response.text))

# 5. Use type-safe data
for item in result.field_2:  # IDE knows type
    process(item)
```

**Example**:
```python
# data_types.py
from pydantic import BaseModel
from typing import List

class TranscriptAnalysis(BaseModel):
    quick_summary: str
    highlights: List[str]
    sentiment: str
    keywords: List[str]

# llm.py
def analyze(transcript: str) -> TranscriptAnalysis:
    response = client.messages.create(
        model="claude-sonnet-4-20250514",
        messages=[{"role": "user", "content": transcript}],
        response_format={
            "type": "json_schema",
            "json_schema": {
                "schema": TranscriptAnalysis.model_json_schema()
            }
        }
    )
    return TranscriptAnalysis(**json.loads(response.content[0].text))

# main.py
analysis: TranscriptAnalysis = analyze(text)
print(analysis.quick_summary)  # Type-safe
for h in analysis.highlights:  # IDE autocomplete
    print(f"- {h}")
```

**Related**: Data Types Pattern, ADW Workflows

**Source**: PAICC-2

---

### 5. Architect-Editor

**Intent**: Separate planning (high context) from execution (low context).

**When to Use**:
- Task requires > 100K tokens context
- Significant architectural decisions
- Multiple agents implementing parts
- Large codebase refactoring

**Structure**:
```
Architect Agent (O1/Opus):
  Input: Full requirements + codebase
  Output: Comprehensive spec

Editor Agent(s) (Sonnet):
  Input: Spec only (minimal context)
  Output: Implementation + tests
```

**Example**:
```python
# Step 1: Architect creates spec
architect_prompt = """
Analyze this 100K line codebase.
Design API refactoring to support new auth system.
Output: Comprehensive spec in specs/auth_refactor.md
"""

# Architect (O1) runs with full context
spec_path = architect.execute(architect_prompt, context=full_codebase)

# Step 2: Editor implements from spec
editor_prompt = f"""
Read spec at {spec_path}.
Implement changes following the spec exactly.
Run tests after each module.
"""

# Editor (Sonnet) runs with minimal context (just spec)
result = editor.execute(editor_prompt, context=[spec_path])
```

**Related**: Context Engineering, Specialization

**Source**: PAICC-5, Context Engineering

---

### 6. Director Pattern

**Intent**: Agent generates, tests, evaluates, and iterates autonomously until success.

**When to Use**:
- Test-driven development
- Clear success criteria exist
- Autonomous iteration desired
- Self-correcting workflows

**Structure**:
```python
for iteration in range(max_iterations):
    code = coder_agent.generate()
    results = run_tests(code)
    if results.all_passed:
        return code
    feedback = evaluator_agent.analyze(results)
    coder_agent.update_prompt(feedback)
```

**Example**:
```python
# director.py
def director_workflow(spec_path: str, max_iterations: int = 5):
    """Director pattern: generate → test → evaluate → repeat."""

    for i in range(max_iterations):
        # Generate code
        code = agent.execute(f"Implement spec at {spec_path}")

        # Test code
        test_results = subprocess.run(["pytest"], capture_output=True)

        # Evaluate results
        if test_results.returncode == 0:
            print(f"Success on iteration {i+1}")
            return code

        # Generate feedback
        feedback = agent.execute(f"""
        Tests failed:
        {test_results.stdout}

        Analyze failures and suggest fixes.
        """)

        # Update prompt for next iteration
        spec_path = f"{spec_path}\n\nFeedback: {feedback}"

    return None  # Failed after max iterations
```

**Related**: Closed-Loop Feedback, Verification Loops

**Source**: PAICC-7

---

### 7. Three-Legged Stool

**Intent**: Debug AI failures systematically: Context (80%) → Prompt (15%) → Model (5%).

**When to Use**:
- AI coding failed
- Agent producing wrong results
- Systematic debugging needed
- Root cause analysis

**Structure**:
```
Step 1: Check CONTEXT (80% of issues)
  - Are necessary files loaded?
  - Correct directory?
  - Relevant docs available?
  - Context polluted?

Step 2: Check PROMPT (15% of issues)
  - Clear and specific?
  - Conflicting instructions?
  - Output format specified?

Step 3: Check MODEL (5% of issues)
  - Appropriate for task complexity?
  - Hallucinating?
```

**Example**:
```bash
# Agent failure: Can't find module

# Check Context First (80%)
> ls src/  # Module exists?
> pwd  # In right directory?
> cat ai_docs/architecture.md  # Docs explain structure?

# Fix: Load missing context
> claude -p "Read src/module.py and implement feature"

# Still failing? Check Prompt (15%)
> Review prompt for clarity
> Add examples, constraints

# Still failing? Check Model (5%)
> Switch from Sonnet to Opus for complex reasoning
```

**Related**: Verification Loops, Context Engineering

**Source**: PAICC-4

---

### 8. Pitfalls Framework

**Intent**: Avoid common failure modes through awareness and prevention.

**When to Use**:
- Before starting complex features
- When reviewing agent failures
- Team onboarding
- Production readiness review

**Structure**:
```
Common Pitfalls:
1. Compound Changes (violates incremental)
2. No Verification (violates verification loops)
3. Premature Abstraction (violates emergence)
4. Context Overload (exceeds token limits)
5. Missing Specs (violates one-shot goal)
```

**Example**:
```python
# BAD: Compound changes
Prompt: "Add auth, logging, error handling, and metrics"
# Problem: If something breaks, which change caused it?

# GOOD: Incremental
Prompt 1: "Add auth"
→ Verify, Commit
Prompt 2: "Add logging"
→ Verify, Commit

# BAD: No verification
code = agent.generate()
deploy(code)  # Hope it works!

# GOOD: Verification loop
code = agent.generate()
tests = run_tests(code)
if tests.passed:
    deploy(code)
```

**Related**: All patterns (pitfalls are violations)

**Source**: PAICC-4, framework-master.md

---

## Tactical Patterns

Apply these in production systems.

### 9. Tool-Enabled Autonomy

**Intent**: Give agents tools (bash, read, write) to close feedback loops autonomously.

**When to Use**:
- Phase 2 (agentic) coding
- Autonomous workflows
- Self-correcting systems
- Unattended execution

**Structure**:
```python
# Phase 1: AI suggests, human executes
suggestion = ai.suggest("Add error handling")
human.review()
human.apply()

# Phase 2: AI executes directly with tools
ai.tool_use("Edit", file="main.py", changes="...")
ai.tool_use("Bash", command="pytest")
ai.tool_use("Write", file="output.txt", content="...")
```

**Example**:
```python
# Claude Code with tools enabled
agent.execute("""
1. Read src/api.py
2. Add error handling for network timeouts
3. Run tests: pytest tests/test_api.py
4. If tests fail, fix issues and retry
5. Commit: "feat: add timeout error handling"
""")

# Agent autonomously:
# - Reads file (Read tool)
# - Modifies code (Edit tool)
# - Runs tests (Bash tool)
# - Fixes failures (loop)
# - Commits (Bash tool)
```

**Related**: Director Pattern, Closed-Loop Feedback

**Source**: TAC-1

---

### 10. PITER Framework

**Intent**: Plan → Implement → Test → Execute → Report workflow for autonomous features.

**When to Use**:
- Complete feature automation
- Unattended execution
- Standardized workflows
- Team collaboration

**Structure**:
```python
def piter_workflow(feature_request: str):
    # P - Plan
    spec = architect.create_spec(feature_request)

    # I - Implement
    code = editor.implement(spec)

    # T - Test
    results = run_tests(code)

    # E - Execute (deploy)
    if results.all_passed:
        deploy(code)

    # R - Report
    generate_report(spec, code, results)
```

**Example**:
```python
# adws/piter.py
def main():
    # Plan phase
    plan = agent.execute("/plan Create user login endpoint")

    # Implement phase
    impl = agent.execute(f"/implement {plan.spec_path}")

    # Test phase
    tests = subprocess.run(["pytest", "tests/"])

    # Execute phase
    if tests.returncode == 0:
        subprocess.run(["git", "push"])
        deploy()

    # Report phase
    report = {
        "feature": "user login",
        "spec": plan.spec_path,
        "tests": "passed" if tests.returncode == 0 else "failed",
        "deployed": tests.returncode == 0
    }
    write_report(report)
```

**Related**: Architect-Editor, Director Pattern

**Source**: TAC-4

---

### 11. Closed-Loop Feedback

**Intent**: Agent tests its own output and iterates until tests pass.

**When to Use**:
- Test-driven development
- Autonomous quality gates
- Production workflows
- Self-healing systems

**Structure**:
```python
def closed_loop(task):
    while not success:
        code = generate(task)
        result = test(code)
        if result.passed:
            return code
        task = refine(task, result.feedback)
```

**Example**:
```python
# GitHub issue automation
def process_issue(issue_id):
    # Get issue details
    issue = gh.get_issue(issue_id)

    # Create spec
    spec = agent.execute(f"Create spec for: {issue.body}")

    # Implement with closed loop
    for attempt in range(5):
        code = agent.execute(f"Implement {spec.path}")

        # Test
        result = subprocess.run(["pytest"])

        if result.returncode == 0:
            # Tests passed - create PR
            pr = gh.create_pr(code, f"Fixes #{issue_id}")
            return pr

        # Tests failed - analyze and retry
        feedback = agent.execute(f"""
        Tests failed:
        {result.stdout}

        Fix issues and retry.
        """)
```

**Related**: Director Pattern, Verification Loops

**Source**: TAC-5

---

### 12. One Agent One Purpose

**Intent**: Focused agents with minimal context outperform generalists.

**When to Use**:
- Multiple distinct responsibilities
- Context reduction needed
- Parallel execution
- Specialized expertise

**Structure**:
```
General Agent (bad):
  Context: Everything
  Tasks: All types
  Result: Confused, slow

Specialized Agents (good):
  Agent A: Context = Backend only
  Agent B: Context = Frontend only
  Agent C: Context = Tests only
```

**Example**:
```markdown
# .claude/agents/backend-specialist.md
---
name: backend-specialist
description: FastAPI backend specialist
tools: Read, Write, Bash
model: sonnet
---

You are a backend specialist.

Focus on:
- FastAPI endpoints
- Database queries
- API contracts

Ignore:
- Frontend code
- UI components
```

```python
# Use specialized agents
backend = agent.create("backend-specialist")
backend.execute("Implement /auth/login endpoint")

frontend = agent.create("frontend-specialist")
frontend.execute("Create login form component")
```

**Related**: Architect-Editor, Worktree Isolation

**Source**: TAC-6

---

### 13. Worktree Isolation

**Intent**: Use git worktrees for parallel agent execution without conflicts.

**When to Use**:
- Multiple agents working simultaneously
- Parallel feature development
- Context isolation between tasks
- Avoiding file conflicts

**Structure**:
```bash
# Main repo
/project

# Worktrees
/project-worktree-feature-a  # Agent A
/project-worktree-feature-b  # Agent B
/project-worktree-feature-c  # Agent C

# Each has independent working directory
# All share same .git history
```

**Example**:
```bash
# Create worktrees
git worktree add ../wt-feature-a -b feature-a
git worktree add ../wt-feature-b -b feature-b
git worktree add ../wt-feature-c -b feature-c

# Sparse checkout for context reduction
cd ../wt-feature-a
git sparse-checkout set apps/backend/

cd ../wt-feature-b
git sparse-checkout set apps/frontend/

# Run agents in parallel
cd ../wt-feature-a
./adws/implement_feature.py &

cd ../wt-feature-b
./adws/implement_feature.py &

# Merge when complete
git worktree remove ../wt-feature-a
git merge feature-a
```

**Related**: One Agent One Purpose, Multi-Agent Coordination

**Source**: TAC-7

---

### 14. Agentic Layer

**Intent**: Separate agentic orchestration from application code.

**When to Use**:
- Clear separation of concerns
- Team collaboration (different skill sets)
- Evolving automation independently
- Production systems

**Structure**:
```
.claude/          # Agentic layer: commands, hooks
adws/             # Agentic layer: workflows
ai_docs/          # Agentic layer: agent knowledge
specs/            # Agentic layer: planning

apps/             # Application layer: actual software
tests/            # Application layer: test suites
```

**Example**:
```python
# Agentic layer orchestrates application layer

# adws/build_feature.py (agentic layer)
def build_feature(spec_path):
    # Orchestrate development
    code = agent.execute(f"Implement {spec_path}")
    tests = run_tests()
    if tests.passed:
        deploy_to_prod()

# apps/my_service/api.py (application layer)
@app.post("/users")
def create_user(user: User):
    # Actual business logic
    return db.create(user)
```

**Related**: Architecture Patterns, Separation of Concerns

**Source**: TAC-8

---

### 15. Hook-Based Observability

**Intent**: Use Claude Code hooks for security, logging, and monitoring.

**When to Use**:
- Security validation needed
- Audit logging required
- Multi-agent coordination
- Production observability

**Structure**:
```python
# PreToolUse: Block dangerous operations
def pre_tool_use(tool_name, tool_input):
    if is_dangerous(tool_input):
        sys.exit(2)  # Block
    sys.exit(0)  # Allow

# PostToolUse: Log all operations
def post_tool_use(tool_name, tool_result):
    log_to_central(tool_name, tool_result)
    sys.exit(0)
```

**Example**:
```python
# .claude/hooks/pre_tool_use.py
import sys
import re

ALLOWED_DIRS = ['trees/', 'temp/']

def is_dangerous_rm(command):
    """Block rm -rf outside allowed dirs."""
    patterns = [r'\brm\s+.*-[a-z]*r[a-z]*f']
    if any(re.search(p, command) for p in patterns):
        if not is_path_allowed(command, ALLOWED_DIRS):
            return True
    return False

if tool_name == 'Bash':
    command = tool_input.get('command', '')
    if is_dangerous_rm(command):
        print("BLOCKED: rm -rf outside allowed directories")
        sys.exit(2)  # Block execution

sys.exit(0)  # Allow

# .claude/hooks/post_tool_use.py
import sys
import json

# Log all tool usage
log_entry = {
    "tool": tool_name,
    "input": tool_input,
    "result": tool_result
}

with open("logs/tool_usage.jsonl", "a") as f:
    f.write(json.dumps(log_entry) + "\n")

sys.exit(0)
```

**Related**: Security Patterns, Observability

**Source**: TAC-8, Claude Skills

---

### 16. Multi-Agent Delegation

**Intent**: Coordinate multiple specialized agents through Task tool.

**When to Use**:
- Parallel execution needed
- Different expertise domains
- Large workload distribution
- Context isolation

**Structure**:
```markdown
Main Agent:
  1. Decompose work
  2. For each subtask:
     <task_loop_prompt>
     Use @agent-specialist - pass subtask
     </task_loop_prompt>
  3. Aggregate results
```

**Example**:
```markdown
# load_ai_docs.md (skill)

## Workflow

1. Read ai_docs/README.md for URL list
2. For each URL, use Task tool in parallel:

<scrape_loop_prompt>
Use @agent-docs-scraper - pass the url
</scrape_loop_prompt>

3. After all Tasks complete:
   - Aggregate content
   - Generate summary
   - Save to ai_docs/
```

```python
# Implementation
urls = read_urls("ai_docs/README.md")

for url in urls:
    # Each spawns sub-agent in parallel
    agent.task(f"@agent-docs-scraper {url}")

# Wait for all to complete
results = agent.wait_all_tasks()
```

**Related**: One Agent One Purpose, Worktree Isolation

**Source**: TAC-8, Claude Skills

---

## Advanced Patterns

For sophisticated systems.

### 17. R&D Framework (Reduce or Delegate)

**Intent**: Handle large context through reduction or delegation, not both blindly.

**When to Use**:
- Context > 50K tokens
- Complex not just large
- Multiple concerns
- Strategic context management

**Structure**:
```
Context < 50K:
  → Use full context

Context 50-100K + Focused:
  → REDUCE (priming, sparse checkout, focused files)

Context 100K+ or Complex:
  → DELEGATE (Architect-Editor, sub-agents)
```

**Example**:
```python
# Measure context
context_size = measure_context()

if context_size < 50000:
    # Full context
    agent.execute(task, context=all_files)

elif context_size < 100000:
    # REDUCE
    relevant_files = glob("src/module_x/**/*.py")
    agent.execute(task, context=relevant_files)

else:
    # DELEGATE
    spec = architect.plan(task, context=all_files)
    result = editor.execute(spec, context=[spec])
```

**Related**: Architect-Editor, Context Engineering

**Source**: Context Engineering

---

### 18. 7 Prompt Levels

**Intent**: Progress from simple to self-improving prompts.

**When to Use**:
- Understanding prompt sophistication
- Evolving workflow complexity
- Building reusable systems
- Organizational learning

**Structure**:
```
Level 1: High-Level (ad-hoc)
Level 2: Workflow (Input → Work → Output)
Level 3: Control Flow (if/else, loops)
Level 4: Delegate (orchestrate agents)
Level 5: Higher-Order (prompts as input)
Level 6: Metaprompt (generate prompts)
Level 7: Self-Improving (expertise accumulation)
```

**Example**:
```markdown
# Level 1: High-Level
"Add user authentication"

# Level 2: Workflow
## Input
feature_description: $1

## Work
1. Create spec
2. Implement code
3. Run tests

## Output
Report results

# Level 3: Control Flow
If tests pass:
  Deploy to production
Otherwise:
  Generate failure report

# Level 4: Delegate
For each module:
  <delegate_prompt>
  Use @specialist-agent - implement module
  </delegate_prompt>

# Level 7: Self-Improving
## Expertise
[Accumulated learnings from previous executions]

## Workflow
[Uses expertise to improve over time]
```

**Related**: All patterns (levels apply to all)

**Source**: Prompt Engineering

---

### 19. Context Bundle Tracking

**Intent**: Monitor and optimize token usage through JSONL bundles.

**When to Use**:
- Context optimization
- Cost management
- Performance tuning
- Debugging context issues

**Structure**:
```bash
# Generate bundle
claude --context-bundle output.jsonl

# Analyze
{
  "total_tokens": 95000,
  "files_loaded": 127,
  "largest_files": [...],
  "recommendations": [...]
}
```

**Example**:
```python
# Track context in ADW
def execute_with_tracking(prompt):
    bundle_path = f"bundles/{uuid.uuid4()}.jsonl"

    result = subprocess.run([
        "claude",
        "-p", prompt,
        "--context-bundle", bundle_path
    ])

    # Analyze bundle
    bundle = parse_jsonl(bundle_path)

    print(f"Tokens used: {bundle['total_tokens']}")
    print(f"Files loaded: {len(bundle['files'])}")

    if bundle['total_tokens'] > 100000:
        print("WARNING: Consider context reduction")

    return result
```

**Related**: R&D Framework, Context Engineering

**Source**: Context Engineering

---

### 20. Custom Tool Creation

**Intent**: Extend Claude Code with project-specific tools via MCP.

**When to Use**:
- Frequent external API calls
- Project-specific operations
- Team standardization
- Complex integrations

**Structure**:
```python
# mcp_server.py
from mcp.server import Server

@server.tool()
def deploy_to_staging(service: str):
    """Deploy service to staging environment."""
    # Project-specific deployment logic
    return result
```

**Example**:
```python
# .claude/mcp/project_tools.py
from mcp.server import Server

server = Server("project-tools")

@server.tool()
def run_migrations(env: str):
    """Run database migrations."""
    subprocess.run(["alembic", "upgrade", "head"])
    return {"status": "success"}

@server.tool()
def deploy(service: str, env: str):
    """Deploy service to environment."""
    subprocess.run(["kubectl", "apply", "-f", f"{service}.yaml"])
    return {"status": "deployed"}

# Agent uses custom tools
agent.execute("""
1. Run migrations: staging
2. Deploy: api-service, staging
3. Verify health checks
""")
```

**Related**: Tool-Enabled Autonomy, MCP Integration

**Source**: Claude Skills, TAC-8

---

## Pattern Selection

### By Scenario

| Scenario | Pattern |
|----------|---------|
| Starting new feature | Iterative Prompting |
| Large codebase refactor | Architect-Editor |
| Parallel development | Worktree Isolation |
| Test-driven workflow | Director Pattern, Closed-Loop |
| Context too large | R&D Framework |
| Agent keeps failing | Three-Legged Stool |
| Need security controls | Hook-Based Observability |
| Multiple agents | Multi-Agent Delegation |

### By Phase

**MVA (Phase 1)**:
- Iterative Prompting
- Verification Loops
- Structured Output

**Intermediate (Phase 2)**:
- Multi-File Refactoring
- PITER Framework
- One Agent One Purpose

**Advanced (Phase 3)**:
- Architect-Editor
- Worktree Isolation
- Multi-Agent Delegation

**Production (Phase 4)**:
- Hook-Based Observability
- Context Bundle Tracking
- Custom Tool Creation

---

## Pattern Combinations

### Powerful Stacks

**Full-Stack Feature**:
1. Architect-Editor (planning)
2. One Agent One Purpose (specialization)
3. Closed-Loop Feedback (testing)
4. Multi-Agent Delegation (parallel)

**Production Workflow**:
1. PITER Framework (structure)
2. Director Pattern (autonomy)
3. Hook-Based Observability (security)
4. Context Bundle Tracking (optimization)

**Large Refactoring**:
1. R&D Framework (context management)
2. Architect-Editor (delegation)
3. Worktree Isolation (parallel)
4. Three-Legged Stool (debugging)

**Learning Path**:
1. Iterative Prompting (start here)
2. Verification Loops (essential)
3. Multi-File Refactoring (structure)
4. Structured Output (reliability)
5. PITER Framework (workflows)
6. Architect-Editor (scaling)

---

**Source**: framework-pattern-catalog.md (compressed 4,156 → 800 lines)
**Last Updated**: 2025-10-31
**Lines**: ~800
