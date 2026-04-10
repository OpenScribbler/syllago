# Production Agentic Systems

Guide for deploying agentic systems to production (Phases 3-4).

---

## Table of Contents

1. [Overview](#overview)
2. [Production Readiness](#production-readiness)
3. [Phase 3: Advanced Agentic](#phase-3-advanced-agentic)
4. [Phase 4: Production Agentic](#phase-4-production-agentic)
5. [Common Production Issues](#common-production-issues)
6. [Migration Guide](#migration-guide)

---

## Overview

### What This Covers

**Phase 3 (Advanced)**: Multi-agent orchestration, worktrees, closed-loop feedback
**Phase 4 (Production)**: Security, observability, cost management, incident response

### Prerequisites

Before production:
- ✓ Completed Phase 1 (MVA) - Basic commands and ADWs
- ✓ Completed Phase 2 (Intermediate) - Documentation and specs
- ✓ Have working test suites
- ✓ Team familiar with agentic patterns

### Timeline

**Phase 3**: 3-5 days
**Phase 4**: 1-2 weeks

---

## Production Readiness

### Checklist

**Architecture**:
- [ ] Agentic layer separated from application
- [ ] ai_docs/ comprehensive and current
- [ ] specs/ directory for planning
- [ ] Test coverage > 70%

**Security**:
- [ ] API keys in environment variables (not code)
- [ ] Hooks validate dangerous operations
- [ ] Permissions scoped appropriately
- [ ] Secrets management implemented

**Observability**:
- [ ] Centralized logging configured
- [ ] Tool usage tracked
- [ ] Error monitoring active
- [ ] Context usage measured

**Operations**:
- [ ] CI/CD integration working
- [ ] Rollback procedures documented
- [ ] Incident response runbook exists
- [ ] Team training completed

---

## Phase 3: Advanced Agentic

**Goal**: Multi-agent orchestration with parallel execution and context optimization.

### Git Worktrees Setup

**Purpose**: Enable parallel agent execution without file conflicts.

**Setup**:

```bash
# 1. Create main project structure
cd /path/to/main/repo

# 2. Create worktrees for parallel work
git worktree add ../wt-feature-a -b feature-a
git worktree add ../wt-feature-b -b feature-b
git worktree add ../wt-feature-c -b feature-c

# 3. Configure sparse checkout (reduce context)
cd ../wt-feature-a
git sparse-checkout init --cone
git sparse-checkout set apps/backend/

cd ../wt-feature-b
git sparse-checkout init --cone
git sparse-checkout set apps/frontend/

cd ../wt-feature-c
git sparse-checkout init --cone
git sparse-checkout set tests/
```

**Create worktree helper script**:

```bash
# scripts/create_worktree.sh
#!/bin/bash
set -e

FEATURE_NAME=$1
WORKTREE_DIR="../wt-${FEATURE_NAME}"
SPARSE_PATHS=${2:-"apps/"}

if [ -z "$FEATURE_NAME" ]; then
    echo "Usage: ./create_worktree.sh <feature-name> [sparse-paths]"
    exit 1
fi

# Create worktree
git worktree add "$WORKTREE_DIR" -b "$FEATURE_NAME"

# Configure sparse checkout
cd "$WORKTREE_DIR"
git sparse-checkout init --cone
git sparse-checkout set $SPARSE_PATHS

echo "Worktree created: $WORKTREE_DIR"
echo "Branch: $FEATURE_NAME"
echo "Sparse paths: $SPARSE_PATHS"
```

**Usage**:

```bash
# Create worktree for backend work
./scripts/create_worktree.sh auth-feature "apps/backend/"

# Run agent in worktree
cd ../wt-auth-feature
./adws/implement_feature.py

# Merge when complete
cd /path/to/main/repo
git merge auth-feature
git worktree remove ../wt-auth-feature
```

---

### Multi-Agent Orchestration

**Pattern**: Coordinate specialized agents for parallel execution.

**Create orchestrator ADW**:

```python
# adws/orchestrate_parallel.py
#!/usr/bin/env -S uv run --script
"""Multi-agent orchestration with parallel execution."""
# /// script
# dependencies = [
#   "pydantic>=2.0.0",
#   "rich>=13.0.0",
# ]
# ///

from adw_modules.agent import AgentTemplateRequest, execute_template, generate_short_id
from rich.console import Console
from rich.progress import Progress
import concurrent.futures
from typing import List
from pydantic import BaseModel


class ParallelTask(BaseModel):
    """Definition for parallel agent task."""
    agent_name: str
    command: str
    args: List[str]
    worktree_dir: str


def execute_parallel_agent(task: ParallelTask, adw_id: str) -> tuple:
    """Execute single agent in parallel."""
    request = AgentTemplateRequest(
        agent_name=task.agent_name,
        slash_command=task.command,
        args=task.args,
        adw_id=adw_id,
        model="sonnet",
        working_dir=task.worktree_dir
    )

    response = execute_template(request)
    return task.agent_name, response


def main():
    """Orchestrate multiple agents in parallel."""
    console = Console()
    adw_id = generate_short_id()

    console.print(f"[cyan]Orchestrating parallel agents[/cyan]")
    console.print(f"[yellow]Workflow ID:[/yellow] {adw_id}\n")

    # Define parallel tasks
    tasks = [
        ParallelTask(
            agent_name="backend-agent",
            command="/implement",
            args=["specs/backend.md"],
            worktree_dir="../wt-feature-backend"
        ),
        ParallelTask(
            agent_name="frontend-agent",
            command="/implement",
            args=["specs/frontend.md"],
            worktree_dir="../wt-feature-frontend"
        ),
        ParallelTask(
            agent_name="test-agent",
            command="/test",
            args=["specs/test-plan.md"],
            worktree_dir="../wt-feature-tests"
        ),
    ]

    # Execute in parallel
    with Progress() as progress:
        task_progress = progress.add_task(
            "[cyan]Executing agents...",
            total=len(tasks)
        )

        with concurrent.futures.ThreadPoolExecutor(max_workers=len(tasks)) as executor:
            futures = [
                executor.submit(execute_parallel_agent, task, adw_id)
                for task in tasks
            ]

            results = {}
            for future in concurrent.futures.as_completed(futures):
                agent_name, response = future.result()
                results[agent_name] = response
                progress.update(task_progress, advance=1)
                console.print(f"[green]✓[/green] {agent_name} completed")

    # Report results
    console.print("\n[bold]Results:[/bold]")
    for agent_name, response in results.items():
        status = "[green]Success[/green]" if response.success else "[red]Failed[/red]"
        console.print(f"  {agent_name}: {status}")

    # Check if all succeeded
    all_success = all(r.success for r in results.values())
    if all_success:
        console.print("\n[green]All agents completed successfully![/green]")
    else:
        console.print("\n[red]Some agents failed. Review logs.[/red]")


if __name__ == "__main__":
    main()
```

---

### Context Optimization

**Measure context usage**:

```bash
# Track context in every ADW
claude --context-bundle output.jsonl -p "..."

# Analyze bundle
cat output.jsonl | jq '.total_tokens'
```

**Create context tracking utility**:

```python
# adws/adw_modules/context_tracker.py
import json
from pathlib import Path
from typing import Dict, Any

def analyze_context_bundle(bundle_path: str) -> Dict[str, Any]:
    """Analyze context bundle and provide recommendations."""
    with open(bundle_path) as f:
        data = json.load(f)

    total_tokens = data.get("total_tokens", 0)
    files = data.get("files", [])

    # Sort files by token usage
    sorted_files = sorted(
        files,
        key=lambda x: x.get("tokens", 0),
        reverse=True
    )

    recommendations = []

    if total_tokens > 150000:
        recommendations.append("CRITICAL: Context exceeds 150K tokens - consider resetting")

    if total_tokens > 100000:
        recommendations.append("WARNING: Consider context reduction or delegation")

    # Identify large files
    large_files = [f for f in sorted_files if f.get("tokens", 0) > 10000]
    if large_files:
        recommendations.append(f"Large files detected: {len(large_files)} files > 10K tokens")

    return {
        "total_tokens": total_tokens,
        "file_count": len(files),
        "largest_files": sorted_files[:5],
        "recommendations": recommendations
    }
```

---

### Observability Setup

**Install PostToolUse hook**:

```bash
# .claude/hooks/post_tool_use.py
#!/usr/bin/env python3
"""Log all tool usage for observability."""
import sys
import json
import os
from datetime import datetime
from pathlib import Path

# Read hook input
tool_name = os.environ.get("CLAUDE_TOOL_NAME", "unknown")
tool_input = json.loads(os.environ.get("CLAUDE_TOOL_INPUT", "{}"))
tool_result = json.loads(os.environ.get("CLAUDE_TOOL_RESULT", "{}"))

# Create logs directory
log_dir = Path("logs")
log_dir.mkdir(exist_ok=True)

# Log entry
log_entry = {
    "timestamp": datetime.utcnow().isoformat(),
    "tool": tool_name,
    "input": tool_input,
    "result": tool_result,
    "session_id": os.environ.get("CLAUDE_SESSION_ID")
}

# Append to JSONL log
log_file = log_dir / "tool_usage.jsonl"
with open(log_file, "a") as f:
    f.write(json.dumps(log_entry) + "\n")

sys.exit(0)
```

**Make executable**:

```bash
chmod +x .claude/hooks/post_tool_use.py
```

---

## Phase 4: Production Agentic

**Goal**: Production-hardened with security, monitoring, and team processes.

### Security Hardening

**1. Create PreToolUse validation hook**:

```bash
# .claude/hooks/pre_tool_use.py
#!/usr/bin/env python3
"""Validate and block dangerous operations."""
import sys
import json
import os
import re

tool_name = os.environ.get("CLAUDE_TOOL_NAME", "")
tool_input = json.loads(os.environ.get("CLAUDE_TOOL_INPUT", "{}"))

# Define security policies
ALLOWED_RM_DIRECTORIES = ['trees/', 'temp/', 'build/']
BLOCKED_COMMANDS = ['sudo', 'rm -rf /', 'chmod 777']
SENSITIVE_FILES = ['.env', 'credentials.json', 'secrets.yaml']


def is_dangerous_rm_command(command: str) -> bool:
    """Check for dangerous rm commands."""
    patterns = [r'\brm\s+.*-[a-z]*r[a-z]*f']

    if any(re.search(p, command) for p in patterns):
        # Check if in allowed directory
        for allowed_dir in ALLOWED_RM_DIRECTORIES:
            if allowed_dir in command:
                return False
        return True
    return False


def contains_blocked_command(command: str) -> bool:
    """Check for explicitly blocked commands."""
    return any(blocked in command for blocked in BLOCKED_COMMANDS)


def accesses_sensitive_file(tool_input: dict) -> bool:
    """Check if accessing sensitive files."""
    file_path = tool_input.get('file_path', '')
    content = tool_input.get('content', '')

    for sensitive in SENSITIVE_FILES:
        if sensitive in file_path or sensitive in content:
            return True
    return False


# Validation logic
if tool_name == 'Bash':
    command = tool_input.get('command', '')

    if is_dangerous_rm_command(command):
        print("BLOCKED: Dangerous rm command outside allowed directories")
        sys.exit(2)  # Block execution

    if contains_blocked_command(command):
        print(f"BLOCKED: Command contains blocked operation")
        sys.exit(2)

elif tool_name in ['Write', 'Edit']:
    if accesses_sensitive_file(tool_input):
        print("BLOCKED: Attempting to modify sensitive file")
        sys.exit(2)

# Allow execution
sys.exit(0)
```

**2. Secrets management**:

```bash
# .env (never commit)
ANTHROPIC_API_KEY=your_key
DATABASE_URL=postgresql://...
API_TOKEN=secret_token

# Load in ADWs
from dotenv import load_dotenv
load_dotenv()

api_key = os.getenv("ANTHROPIC_API_KEY")
```

**3. Permission scoping**:

```yaml
# .claude/config.yaml
permissions:
  tools:
    - Bash
    - Read
    - Write
    - Edit
  blocked_paths:
    - /etc/
    - /var/
    - ~/.ssh/
  allowed_commands:
    - git
    - pytest
    - npm
    - uv
```

---

### Cost Tracking

**Create cost monitoring**:

```python
# adws/adw_modules/cost_tracker.py
import json
from pathlib import Path
from datetime import datetime
from typing import Dict

# Token costs (as of 2025)
COST_PER_1M_TOKENS = {
    "sonnet": {"input": 3.00, "output": 15.00},
    "opus": {"input": 15.00, "output": 75.00},
    "o1": {"input": 15.00, "output": 60.00}
}


class CostTracker:
    """Track API costs across workflows."""

    def __init__(self, log_dir: str = "logs"):
        self.log_dir = Path(log_dir)
        self.log_dir.mkdir(exist_ok=True)
        self.cost_log = self.log_dir / "costs.jsonl"

    def log_usage(
        self,
        adw_id: str,
        model: str,
        input_tokens: int,
        output_tokens: int
    ):
        """Log token usage and cost."""
        costs = COST_PER_1M_TOKENS.get(model, COST_PER_1M_TOKENS["sonnet"])

        input_cost = (input_tokens / 1_000_000) * costs["input"]
        output_cost = (output_tokens / 1_000_000) * costs["output"]
        total_cost = input_cost + output_cost

        entry = {
            "timestamp": datetime.utcnow().isoformat(),
            "adw_id": adw_id,
            "model": model,
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "input_cost": input_cost,
            "output_cost": output_cost,
            "total_cost": total_cost
        }

        with open(self.cost_log, "a") as f:
            f.write(json.dumps(entry) + "\n")

    def get_daily_costs(self) -> Dict[str, float]:
        """Get costs for today."""
        today = datetime.utcnow().date()
        daily_cost = 0.0

        if not self.cost_log.exists():
            return {"date": str(today), "cost": 0.0}

        with open(self.cost_log) as f:
            for line in f:
                entry = json.loads(line)
                entry_date = datetime.fromisoformat(entry["timestamp"]).date()
                if entry_date == today:
                    daily_cost += entry["total_cost"]

        return {"date": str(today), "cost": daily_cost}


# Usage in ADW
tracker = CostTracker()
tracker.log_usage(
    adw_id="abc123",
    model="sonnet",
    input_tokens=50000,
    output_tokens=10000
)

print(f"Today's costs: ${tracker.get_daily_costs()['cost']:.2f}")
```

---

### Monitoring Setup

**Create monitoring dashboard script**:

```python
# scripts/monitoring_dashboard.py
#!/usr/bin/env python3
"""Real-time monitoring dashboard for agentic workflows."""
import json
from pathlib import Path
from collections import defaultdict
from rich.console import Console
from rich.table import Table
from rich.live import Live
import time


def analyze_logs():
    """Analyze tool usage and costs."""
    tool_log = Path("logs/tool_usage.jsonl")
    cost_log = Path("logs/costs.jsonl")

    # Tool usage stats
    tool_counts = defaultdict(int)
    if tool_log.exists():
        with open(tool_log) as f:
            for line in f:
                entry = json.loads(line)
                tool_counts[entry["tool"]] += 1

    # Cost stats
    total_cost = 0.0
    if cost_log.exists():
        with open(cost_log) as f:
            for line in f:
                entry = json.loads(line)
                total_cost += entry["total_cost"]

    return tool_counts, total_cost


def create_dashboard():
    """Create monitoring dashboard."""
    console = Console()

    while True:
        tool_counts, total_cost = analyze_logs()

        # Tool usage table
        tool_table = Table(title="Tool Usage")
        tool_table.add_column("Tool", style="cyan")
        tool_table.add_column("Count", justify="right")

        for tool, count in sorted(tool_counts.items(), key=lambda x: -x[1]):
            tool_table.add_row(tool, str(count))

        # Cost summary
        cost_table = Table(title="Cost Summary")
        cost_table.add_column("Metric", style="cyan")
        cost_table.add_column("Value", justify="right")
        cost_table.add_row("Total Cost", f"${total_cost:.2f}")

        console.clear()
        console.print(tool_table)
        console.print()
        console.print(cost_table)

        time.sleep(5)  # Refresh every 5 seconds


if __name__ == "__main__":
    create_dashboard()
```

---

### Incident Response

**Create runbook**:

```markdown
# Incident Response Runbook

## Agent Failure

**Symptoms**: Agent not responding, timing out, or producing errors

**Steps**:
1. Check logs: `tail -f logs/tool_usage.jsonl`
2. Verify API key: `echo $ANTHROPIC_API_KEY`
3. Check context size: Review recent context bundles
4. Apply Three-Legged Stool:
   - Context (80%): Missing files? Wrong directory?
   - Prompt (15%): Clear instructions?
   - Model (5%): Appropriate model?
5. If context pollution: Reset and prime
6. If persistent: Switch to manual mode

## Cost Spike

**Symptoms**: Daily costs exceed budget

**Steps**:
1. Check cost logs: `python scripts/analyze_costs.py`
2. Identify expensive workflows
3. Review context bundles for bloat
4. Implement context reduction
5. Consider model downgrade (Opus → Sonnet)
6. Add cost limits to ADWs

## Security Breach

**Symptoms**: Unauthorized operations, leaked credentials

**Steps**:
1. IMMEDIATELY rotate API keys
2. Review pre_tool_use.py logs
3. Check for blocked operations
4. Audit recent commits
5. Review hook configurations
6. Strengthen validation rules
7. Notify team

## Production Deployment Failure

**Symptoms**: Tests pass locally, fail in production

**Steps**:
1. Check environment variables
2. Verify dependencies (uv sync)
3. Review deployment logs
4. Compare environments (dev vs prod)
5. Rollback to last working version
6. Fix in isolated worktree
7. Re-deploy with verification
```

---

### Team Onboarding

**Create onboarding guide**:

```markdown
# Team Onboarding: Agentic Systems

## Week 1: Fundamentals

**Day 1-2**: Setup
- Install tools (Claude Code, UV, Git)
- Configure API keys
- Clone project repository
- Run first slash command

**Day 3-4**: Core Patterns
- Complete MVA tutorial
- Build first ADW
- Practice iterative prompting
- Understand verification loops

**Day 5**: Team Workflows
- Learn project structure
- Review ai_docs/
- Practice with shared commands
- Understand hooks and security

## Week 2: Advanced

**Day 1-2**: Multi-Agent
- Create git worktrees
- Run parallel agents
- Practice coordination

**Day 3-4**: Production
- Deploy monitoring
- Practice incident response
- Review security policies

**Day 5**: Project Work
- Implement real feature
- Team code review
- Document learnings

## Resources

- getting-started.md: MVA setup
- patterns.md: Common patterns
- decisions.md: Decision trees
- production.md: This document
```

---

## Common Production Issues

### Issue Resolution Table

| Issue | Symptom | Root Cause | Fix |
|-------|---------|------------|-----|
| **Context Overload** | Agent slow/failing | > 150K tokens | Reset + delegate |
| **Cost Spike** | High bills | Wrong model | Use Sonnet, not Opus |
| **Agent Conflicts** | File corruption | Parallel writes | Use worktrees |
| **Test Failures** | Local pass, prod fail | Environment diff | Audit env vars |
| **Hook Not Working** | Operations not blocked | Permissions | chmod +x hooks/ |
| **Missing Context** | Agent can't find files | Not loaded | Add to ai_docs/ |
| **API Rate Limit** | 429 errors | Too many requests | Add delays, batching |
| **Leaked Secrets** | Keys in git | Committed .env | Rotate keys, fix .gitignore |

---

## Migration Guide

### From Phase 2 to Phase 3

**Add**:
```bash
# Git worktrees
./scripts/create_worktree.sh

# Multi-agent coordination
adws/orchestrate_parallel.py

# Context tracking
--context-bundle flags
```

**Update**:
- ai_docs/ with architecture
- Specs for all major features
- Test coverage to 70%+

**Verify**:
- [ ] Worktrees work
- [ ] Parallel agents execute
- [ ] Context measured

---

### From Phase 3 to Phase 4

**Add**:
```bash
# Security
.claude/hooks/pre_tool_use.py
.claude/hooks/post_tool_use.py

# Monitoring
scripts/monitoring_dashboard.py
logs/ directory

# Cost tracking
adws/adw_modules/cost_tracker.py

# Documentation
runbooks/incident_response.md
docs/team_onboarding.md
```

**Update**:
- All secrets to .env
- CI/CD with agent workflows
- Team training materials

**Verify**:
- [ ] Hooks block dangerous ops
- [ ] All tools logged
- [ ] Costs tracked
- [ ] Monitoring active
- [ ] Runbook tested
- [ ] Team trained

---

**Source**: framework-production-playbook.md (Phases 3-4), framework-master.md (Sections 10-13)
**Last Updated**: 2025-10-31
**Lines**: ~600
