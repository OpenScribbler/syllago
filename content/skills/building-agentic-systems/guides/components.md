# Agentic System Components

Component design guidance for building agentic systems.

## Table of Contents

1. [Commands (Slash Commands)](#commands-slash-commands)
2. [Workflows / ADWs](#workflows--adws)
3. [Agents](#agents)
4. [Hooks](#hooks)
5. [Tools / MCP](#tools--mcp)
6. [Knowledge Bases (ai_docs/)](#knowledge-bases-ai_docs)
7. [Tasks](#tasks)
8. [Specs](#specs)

---

## Commands (Slash Commands)

**What**: Manual prompt templates stored in `.claude/commands/` that agents execute via `/command-name` syntax.

**When to Use**:
- ✅ Manual workflows (human decides when to trigger)
- ✅ Reusable patterns (same process used 3+ times)
- ✅ Team collaboration (share expertise across developers)
- ❌ Agent-triggered tasks (use Skills instead)
- ❌ One-off operations (just use conversational mode)

**Structure Template**:
```markdown
# Command Title

## Purpose
Brief description of what this command does

## Variables
$1 - First argument
$2 - Second argument

## Instructions
1. READ the specified files
2. PERFORM the analysis/work
3. OUTPUT/MODIFY according to requirements

## Report
What to communicate back to the user
```

**Organization**:
```
.claude/commands/
├── features/
│   ├── plan-feature.md
│   └── implement-feature.md
├── bugs/
│   └── fix-bug.md
├── chores/
│   └── refactor.md
├── planning/
│   ├── plan.md
│   └── quick-plan.md
└── workflows/
    └── plan-build-test.md
```

**Real Example** (from TAC-3):
```markdown
# /chore

## Purpose
Generate comprehensive plan for technical chore tasks

## Variables
- description: $1

## Instructions
1. READ ai_docs/ for project context
2. CREATE specs/chore-{adw-id}-{description-slug}.md following template
3. INCLUDE:
   - Current state analysis
   - Proposed changes
   - Step-by-step plan
   - Testing approach
   - Rollback strategy

## Report
- Path to created spec file
- Summary of planned changes
- Estimated complexity
```

**Key Patterns**:

1. **Meta-Prompts** - Generate plans from descriptions:
```markdown
# /plan command
Input: One-line feature description
Output: Comprehensive spec in specs/ directory
Example: /plan "Add user authentication"
```

2. **Higher-Order Prompts** - Accept prompts as input:
```markdown
# /implement command
Input: Path to spec file
Output: Code implementing the spec
Example: /implement specs/feat-auth.md
```

3. **Validation Commands** - Verify work quality:
```markdown
# /test command
RUN pytest
RUN linters
REPORT results
```

**Source**: TAC-3 (Template Engineering), Prompt Engineering

---

## Workflows / ADWs

**What**: Agentic Developer Workflows - Python scripts that orchestrate agent execution programmatically.

**When to Use**:
- Needs to run completely unattended (AFK)
- Involves multiple AI calls with deterministic logic between
- Needs to integrate with external systems (CI/CD, webhooks, cron)
- Performed frequently (daily/weekly)

**ADW Pattern**:
```python
#!/usr/bin/env -S uv run --script
"""
ADW: Workflow Name - Brief description

This workflow demonstrates core pattern:
1. Generate tracking ID
2. Build request
3. Execute agent
4. Report results
"""
# /// script
# dependencies = [
#   "pydantic>=2.0.0",
#   "rich>=13.0.0",
# ]
# ///

from adw_modules.agent import AgentTemplateRequest, execute_template, generate_short_id
from rich.console import Console
from rich.panel import Panel


def main():
    """Execute workflow."""
    console = Console()

    # Generate unique ID for tracking
    adw_id = generate_short_id()

    console.print(Panel(
        f"[bold green]Starting Workflow[/bold green]\n"
        f"ADW ID: {adw_id}",
        title="Workflow"
    ))

    # Create agent request
    request = AgentTemplateRequest(
        agent_name="worker",
        slash_command="/build",
        args=["Task description"],
        adw_id=adw_id,
        model="sonnet"
    )

    # Execute agent
    response = execute_template(request)

    # Report results
    if response.success:
        console.print(Panel(
            f"[bold green]✓ Success[/bold green]\n\n{response.output}",
            title=f"Results ({adw_id})"
        ))
    else:
        console.print(Panel(
            f"[bold red]✗ Failed[/bold red]\n\n{response.output}",
            title=f"Error ({adw_id})"
        ))


if __name__ == "__main__":
    main()
```

**PITER Workflow Example**:
```python
# adws/adw_plan_implement.py

def piter_workflow(issue_number):
    """
    PITER: Plan → Implement → Test → Execute → Report
    """
    adw_id = generate_short_id()

    # PLAN
    plan_request = AgentTemplateRequest(
        agent_name="planner",
        slash_command="/plan",
        args=[adw_id, f"Issue #{issue_number}"],
        adw_id=adw_id
    )
    plan_response = execute_template(plan_request)
    spec_path = extract_spec_path(plan_response.output)

    # IMPLEMENT
    implement_request = AgentTemplateRequest(
        agent_name="implementor",
        slash_command="/implement",
        args=[spec_path],
        adw_id=adw_id
    )
    implement_response = execute_template(implement_request)

    # TEST
    test_results = run_tests()

    # EXECUTE
    if test_results.passed:
        git.commit()
        git.push()

    # REPORT
    return create_pr(adw_id, issue_number)
```

**Agent Execution Primitive**:
```python
# adws/adw_modules/agent.py

from pydantic import BaseModel
from typing import Optional, List
import subprocess

class AgentTemplateRequest(BaseModel):
    agent_name: str
    slash_command: str
    args: List[str]
    adw_id: str
    model: str = "sonnet"
    working_dir: Optional[str] = None

class AgentPromptResponse(BaseModel):
    output: str
    success: bool
    session_id: Optional[str] = None

def execute_template(request: AgentTemplateRequest) -> AgentPromptResponse:
    """Execute a slash command template via Claude Code."""
    prompt = f"{request.slash_command} {' '.join(request.args)}"

    cmd = [
        "claude",
        "-p", prompt,
        "--model", request.model,
        "--dangerously-skip-permissions"
    ]

    result = subprocess.run(
        cmd,
        cwd=request.working_dir,
        capture_output=True,
        text=True,
        timeout=300
    )

    return AgentPromptResponse(
        output=result.stdout,
        success=(result.returncode == 0)
    )
```

**Source**: TAC-4 (PITER Framework), TAC-8 (Primitives)

---

## Agents

**What**: Specialized agents with focused purposes and minimal context.

**One Agent One Purpose**: Each agent should have a single, clearly defined responsibility.

**When to Use**:
- Need parallel execution
- Task requires context isolation
- Specialized expertise needed
- Context > 100K tokens (use Architect-Editor)

**Custom Agent with SDK**:
```markdown
# .claude/agents/crypto-analyzer.md
---
name: crypto-analyzer
description: Cryptocurrency analysis specialist
tools: WebFetch, Read, Write
model: opus
---

You are a crypto specialist. Analyze projects for:
- Tokenomics
- Smart contract security
- Market trends

## Context
Read ai_docs/crypto-patterns.md for analysis frameworks.

## Output Format
Use CryptoAnalysis Pydantic model for structured results.
```

**Architect-Editor Example**:
```python
# Architect (High Context, Reasoning)
architect = Agent(
    model="o1-preview",
    context=["specs/", "ai_docs/architecture.md", "src/**/*.py"],
    purpose="Design architecture and create detailed plan"
)

# Editor (Low Context, Fast)
editor = Agent(
    model="claude-sonnet-4",
    context=["specific_file.py"],  # Only file being edited
    purpose="Implement specific changes to one file"
)

# Usage
plan = architect.create_plan(requirements)
for task in plan.tasks:
    editor.execute(task)
```

**Sub-Agent Pattern**:
```markdown
# Parallel Execution with Task Tool

## Workflow
1. Parse list of 10 URLs
2. For each URL, use Task tool:
   <task_loop_prompt>
   Use @agent-docs-scraper - pass url
   </task_loop_prompt>
3. All 10 agents run simultaneously
```

**Specialized Agent Example**:
```
Frontend Agent:
- Context: UI components, styles, API client (8K tokens)
- Purpose: Implement UI changes
- Tools: Read, Write, Edit
- Model: Sonnet

Backend Agent:
- Context: Database models, API routes, business logic (12K tokens)
- Purpose: Implement backend changes
- Tools: Read, Write, Edit, Bash(pytest)
- Model: Sonnet

Result: 60% context reduction, 3x faster, 40% fewer errors
```

**Source**: TAC-6 (One Agent One Purpose), Custom Agents, PAICC-5 (Architect-Editor)

---

## Hooks

**What**: Event-driven scripts that run before or after tool execution.

**Hook Types**:
- **PreToolUse**: Block dangerous operations
- **PostToolUse**: Log tool results
- **SessionStart/SessionEnd**: Track agent lifecycle
- **Stop**: Collect transcripts
- **SubagentStop**: Monitor sub-agents

**When to Use**:
- Need to BLOCK dangerous operations (PreToolUse)
- Need to log/audit all tool usage (PostToolUse)
- Need to track agent lifecycle (SessionStart/End)
- Need cross-cutting concern (applies to all operations)

**Security Hook Example**:
```python
# .claude/hooks/pre_tool_use.py
#!/usr/bin/env python3
"""
Pre-tool-use hook: Validate before execution.
Exit code 2 = block, 0 = allow
"""
import sys
import json

def main():
    hook_input = json.loads(sys.stdin.read())
    tool_name = hook_input.get("tool_name")
    params = hook_input.get("params", {})

    # Block dangerous bash commands
    if tool_name == "Bash":
        command = params.get("command", "")
        dangerous_patterns = ["rm -rf /", "sudo rm", "> /dev/sda"]

        for pattern in dangerous_patterns:
            if pattern in command:
                sys.exit(2)  # Block execution

    sys.exit(0)  # Allow execution

if __name__ == "__main__":
    main()
```

**Observability Hook Example**:
```python
# .claude/hooks/post_tool_use.py
#!/usr/bin/env python3
"""
Post-tool-use hook: Log after execution.
"""
import sys
import json
from datetime import datetime

def main():
    hook_input = json.loads(sys.stdin.read())
    tool_name = hook_input.get("tool_name")
    success = hook_input.get("success", False)

    # Log to file
    with open(".claude/logs/tool_usage.log", "a") as f:
        timestamp = datetime.now().isoformat()
        status = "SUCCESS" if success else "FAILED"
        f.write(f"{timestamp} | {tool_name} | {status}\n")

    sys.exit(0)

if __name__ == "__main__":
    main()
```

**Hook Configuration**:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "python3 .claude/hooks/pre_tool_use.py"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "python3 .claude/hooks/post_tool_use.py"
          }
        ]
      }
    ]
  }
}
```

**Key Principle**: Hooks are for infrastructure concerns (security, logging, observability), NOT business logic.

**Source**: Claude Skills, TAC-8 (Hook-based observability)

---

## Tools / MCP

**What**: Custom tools and MCP servers that extend agent capabilities.

**When to Use**:
- External API/service integration (MCP Server)
- Need to expose tools to Claude
- Tools used in EVERY session (MCP)
- Occasional use (Skill with progressive disclosure)

**@tool Decorator Example**:
```python
# custom_tools.py

from anthropic import tool

@tool
def calculate_mortgage(
    principal: float,
    annual_rate: float,
    years: int
) -> dict:
    """
    Calculate monthly mortgage payment.

    Args:
        principal: Loan amount in dollars
        annual_rate: Annual interest rate as percentage (e.g., 4.5)
        years: Loan term in years
    """
    monthly_rate = (annual_rate / 100) / 12
    num_payments = years * 12

    if monthly_rate == 0:
        monthly_payment = principal / num_payments
    else:
        monthly_payment = principal * (
            monthly_rate * (1 + monthly_rate) ** num_payments
        ) / ((1 + monthly_rate) ** num_payments - 1)

    return {
        "monthly_payment": round(monthly_payment, 2),
        "total_paid": round(monthly_payment * num_payments, 2),
        "total_interest": round((monthly_payment * num_payments) - principal, 2)
    }
```

**MCP Server Creation**:
```python
# mcp_server.py
from mcp.server import Server
from mcp.types import Tool, TextContent

server = Server("my-custom-server")

@server.list_tools()
async def list_tools() -> list[Tool]:
    return [
        Tool(
            name="fetch_user_data",
            description="Fetch user data from API",
            inputSchema={
                "type": "object",
                "properties": {
                    "user_id": {"type": "string"}
                },
                "required": ["user_id"]
            }
        )
    ]

@server.call_tool()
async def call_tool(name: str, arguments: dict) -> list[TextContent]:
    if name == "fetch_user_data":
        user_id = arguments["user_id"]
        # Fetch from API
        data = fetch_from_api(user_id)
        return [TextContent(type="text", text=json.dumps(data))]
```

**MCP vs Skill Decision**:
```
For internal workflow:
  │
  ├─→ Truly external system? → MCP Server
  │   Examples: APIs, databases, file systems
  │
  ├─→ Need to expose tools to Claude?
  │   ├─→ Tools used in EVERY session? → MCP Server
  │   └─→ Occasional use? → Skill with progressive disclosure
  │
  └─→ Orchestration, not tools → Skill
      Example: Worktree manager calling git commands
```

**Context Impact**:
- **MCP Server**: Immediate, all tools loaded on startup
- **Skill**: Progressive, metadata → instructions → resources
- **Command**: None until invoked

**Source**: Claude Skills, MCP Documentation

---

## Knowledge Bases (ai_docs/)

**What**: Documentation written specifically for agents to understand your codebase.

**When to Use**:
- Context > 50K tokens
- Agent makes incorrect assumptions
- Team onboarding new agents
- Codebases with non-obvious patterns

**Structure**:
```
ai_docs/
├── README.md              # Project overview
├── architecture.md        # System design
├── api-guidelines.md      # API conventions
├── database.md            # Schema and queries
├── patterns/
│   ├── authentication.md  # Auth patterns
│   └── error-handling.md  # Error patterns
└── workflows/
    └── deployment.md      # Deploy process
```

**Example** (ai_docs/architecture.md):
```markdown
# Project Architecture

## Overview
This is a full-stack application with clear separation of concerns.

## Directory Structure
```
apps/
├── backend/           # FastAPI server
│   ├── api/          # API endpoints
│   ├── models/       # Database models
│   ├── services/     # Business logic
│   └── tests/        # Backend tests
└── frontend/         # React application
    ├── components/   # React components
    ├── hooks/        # Custom hooks
    └── services/     # API client
```

## Database Layer
We use PostgreSQL via SQLAlchemy ORM.
- Models: apps/backend/models/
- Migrations: alembic/versions/
- Connection: apps/backend/database.py

To add a new model:
1. Create file in apps/backend/models/
2. Import in apps/backend/models/__init__.py
3. Run: alembic revision --autogenerate
4. Run: alembic upgrade head

## API Conventions
All endpoints follow REST conventions:
- GET /api/users - List users
- GET /api/users/{id} - Get user
- POST /api/users - Create user
- PUT /api/users/{id} - Update user
- DELETE /api/users/{id} - Delete user

## Error Handling
Use HTTPException with standard status codes:
```python
from fastapi import HTTPException

if not user:
    raise HTTPException(status_code=404, detail="User not found")
```

## Testing Strategy
- Unit tests: Test individual functions
- Integration tests: Test API endpoints
- E2E tests: Test complete user flows

Run tests: `pytest tests/`
```

**What to Document**:
- Project structure and purpose of each directory
- Where to find things (models, tests, configs)
- Coding standards and patterns
- How to add new features
- Common pitfalls and solutions

**Source**: TAC-2 (Documentation leverage point), Production Playbook

---

## Tasks

**What**: Automation patterns for recurring operations.

**GitHub Issue Automation**:
```python
# adws/adw_issue_automation.py

def handle_github_issue(issue_number: int):
    """
    Automate issue processing:
    1. Issue created (trigger)
    2. Classify issue type (/bug, /feature, /chore)
    3. Create plan
    4. Implement
    5. Test
    6. Create PR
    """
    adw_id = generate_short_id()

    # Fetch issue
    issue = github.get_issue(issue_number)

    # Create feature branch
    branch = f"adw-{adw_id}-issue-{issue_number}"
    git.checkout(branch, create=True)

    # Classify
    classification = agent.execute("/classify", issue.body)

    # Plan
    spec_path = agent.execute(classification.template, adw_id, issue.body)
    git.commit("Add plan")

    # Implement
    agent.execute("/implement", spec_path)
    git.commit("Implement feature")

    # Test
    results = pytest.run()
    if not results.passed:
        agent.execute("/debug", results.failures)
        git.commit("Fix issues")

    # Create PR
    pr = github.create_pr(
        title=f"[ADW-{adw_id}] {issue.title}",
        body=f"Resolves #{issue_number}\n\nADW ID: {adw_id}",
        branch=branch
    )

    github.comment_on_issue(issue_number, f"PR created: {pr.url}")
```

**Cron Trigger Example**:
```python
# adws/cron_process_tasks.py

import time

def poll_for_issues():
    """Poll GitHub every 20 seconds for new issues."""
    processed = set()

    while True:
        issues = github.list_open_issues()

        for issue in issues:
            if issue.number not in processed:
                # Launch ADW in background
                subprocess.Popen([
                    "uv", "run",
                    "adws/adw_issue_automation.py",
                    str(issue.number)
                ])
                processed.add(issue.number)

        time.sleep(20)
```

**Webhook Trigger Example**:
```python
# adws/webhook_server.py

from fastapi import FastAPI, Request

app = FastAPI()

@app.post("/webhook/issues")
async def handle_issue_webhook(request: Request):
    """Instant trigger on issue creation."""
    payload = await request.json()

    if payload["action"] == "opened":
        issue_number = payload["issue"]["number"]

        subprocess.Popen([
            "uv", "run",
            "adws/adw_issue_automation.py",
            str(issue_number)
        ])

        return {"status": "triggered"}
```

**Source**: TAC-4 (PITER), TAC-5 (Closed-loop feedback)

---

## Specs

**What**: Comprehensive technical specifications that guide implementation.

**When to Use**:
- Complex features requiring upfront planning
- Multiple people/agents working on same feature
- One-shot execution goal (reduce iterations)
- When comprehensive planning reduces rework

**Spec Template**:
```markdown
# Specification: {Feature Name}

**Created**: {Date}
**ADW ID**: {adw_id}
**Status**: Draft

---

## High-Level Objective

What are we building and why?

**User Story**: As a {user}, I want to {action} so that {benefit}

**Success Criteria**: How do we know this is complete?

---

## Mid-Level Objectives

Break down the high-level goal:

1. **Objective 1**: {Description}
   - What: {Specific action}
   - Why: {Rationale}
   - Validation: {How to verify}

2. **Objective 2**: {Description}
   - What: {Specific action}
   - Why: {Rationale}
   - Validation: {How to verify}

---

## Implementation Notes

### Technical Approach
- Architecture decisions
- Technology choices
- Design patterns to use

### Dependencies
- External libraries needed
- Services required
- Existing modules to leverage

### Files to Modify/Create
- `path/to/file1.py` - Purpose
- `path/to/file2.py` - Purpose

---

## Testing Strategy

### Unit Tests
- Test case 1: {Description}
- Test case 2: {Description}

### Integration Tests
- Test case 1: {Description}

### Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2
- [ ] All tests pass

---

## Low-Level Tasks

Ordered from start to finish. Each task is atomic.

### Task 1: {Description}
```aider
PROMPT: {Exact prompt for agent}
FILES: {Files to modify}
EXPECTED OUTCOME: {What should change}
```

**Validation**: {How to verify}

### Task 2: {Description}
```aider
PROMPT: {Exact prompt for agent}
FILES: {Files to modify}
EXPECTED OUTCOME: {What should change}
```

**Validation**: {How to verify}

---

## Rollback Plan

If implementation fails:
1. Revert commits: `git revert {commit_hash}`
2. Restore files: `git checkout HEAD -- {files}`
3. Run tests to verify stability
```

**Architect-Editor with Specs**:
```bash
# Architect creates comprehensive spec
aider --o1-preview --architect spec.md

# Editor implements from spec
aider --editor-model claude-3-5-sonnet --yes spec.md
```

**Source**: PAICC-5 (Spec-based coding), TAC-2 (Specs leverage point)

---

## Component Integration

Components work together in patterns:

**Pattern 1: Command → Spec → Implementation**
```
User: /plan "Add authentication"
  ↓
Command generates spec
  ↓
specs/feat-auth.md created
  ↓
User: /implement specs/feat-auth.md
  ↓
Command executes spec
  ↓
Application code updated
```

**Pattern 2: ADW → Multi-Agent → Worktrees**
```
Python: adw_multi_agent.py
  ↓
Creates worktrees for each task
  ↓
Spawns parallel agents in trees/
  ↓
Each agent executes independently
  ↓
Results aggregated and reported
```

**Pattern 3: Hook → Validation → Action**
```
Agent attempts tool use
  ↓
PreToolUse hook triggered
  ↓
Security validation
  ↓
If approved: continue
If denied: block and log
  ↓
PostToolUse hook logs action
```

**Pattern 4: Director → Test → Iterate**
```
Director config defines task
  ↓
Generate code (fast model)
  ↓
Run tests (automated)
  ↓
Evaluate results (reasoning model)
  ↓
If pass: done
If fail: feedback → loop
```

---

## Summary

Essential components for agentic systems:

**Commands**: Reusable prompt templates for manual workflows
**Workflows/ADWs**: Python scripts for autonomous execution
**Agents**: Specialized workers with focused purposes
**Hooks**: Event-driven validation and logging
**Tools/MCP**: Custom capabilities and integrations
**Knowledge Bases**: Agent-specific documentation
**Tasks**: Automation for recurring operations
**Specs**: Comprehensive planning documents

**Component Selection**:
```
External Integration → MCP Server
Parallel Execution → Sub-agents
Agent-triggered → Skill
Manual-triggered → Command
Observability → Hooks
Unattended Workflows → ADW
Orchestration → Delegate Prompt
```

**Sources**:
- TAC-3 (Commands), TAC-4 (ADWs), TAC-6 (Agents), TAC-8 (Hooks)
- Claude Skills (Skills, MCP, Task tool)
- PAICC-5 (Specs, Architect-Editor)
- Framework Master (Components Deep Dive section 5)
