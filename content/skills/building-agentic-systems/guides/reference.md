# Quick Reference

Quick reference and glossary for rapid lookup.

## Table of Contents

1. [Command Cheat Sheet](#command-cheat-sheet)
2. [Workflow / ADW Template](#workflow--adw-template)
3. [Directory Structure Reference](#directory-structure-reference)
4. [Glossary](#glossary)
5. [File Naming Conventions](#file-naming-conventions)
6. [Common Patterns Quick Lookup](#common-patterns-quick-lookup)

---

## Command Cheat Sheet

### Essential Command Structure

```markdown
# Command Title

## Purpose
One-line description

## Variables
variable_name: $1
another_var: $2

## Instructions
1. READ relevant files
2. PERFORM the work
3. CREATE/MODIFY outputs

## Report
What to communicate back
```

### Essential Sections

**Minimal Command**:
- Title
- Instructions
- Report

**Standard Command**:
- Title
- Purpose
- Variables (if accepts args)
- Instructions
- Report

**Advanced Command**:
- Title
- Purpose
- Variables
- Context (what to read)
- Instructions
- Format (output structure)
- Report

### Variable Syntax

```markdown
## Variables
feature_name: $1          # First argument
spec_file: $2             # Second argument
all_args: $@              # All arguments as array

## Usage
/command arg1 arg2
→ $1 = "arg1"
→ $2 = "arg2"
→ $@ = ["arg1", "arg2"]
```

---

## Workflow / ADW Template

### Minimal ADW

```python
#!/usr/bin/env -S uv run --script
"""ADW: Name - Description"""
# /// script
# dependencies = ["pydantic>=2.0.0", "rich>=13.0.0"]
# ///

from adw_modules.agent import AgentTemplateRequest, execute_template, generate_short_id
from rich.console import Console

def main():
    console = Console()
    adw_id = generate_short_id()

    request = AgentTemplateRequest(
        agent_name="worker",
        slash_command="/build",
        args=["Task description"],
        adw_id=adw_id
    )

    response = execute_template(request)
    console.print(f"Success: {response.success}")

if __name__ == "__main__":
    main()
```

### Coder.create() Pattern

```python
from aider.coders import Coder

# Create agent
coder = Coder.create(
    fnames=["src/main.py"],           # Files to edit
    read_only_fnames=["README.md"],   # Read-only context
    main_model="claude-sonnet-4",     # Model to use
    edit_format="diff"                # Edit format
)

# Execute prompt
coder.run("Add error handling to main function")
```

### Essential Imports

```python
# Core ADW
from adw_modules.agent import (
    AgentTemplateRequest,
    execute_template,
    generate_short_id
)

# Rich output
from rich.console import Console
from rich.panel import Panel

# Pydantic
from pydantic import BaseModel
from typing import List, Optional, Literal

# System
import subprocess
import json
from pathlib import Path
```

---

## Directory Structure Reference

### Complete Agentic Project

```
project_root/
├── .claude/                      # AGENTIC LAYER
│   ├── commands/                 # Slash commands (manual)
│   │   ├── features/
│   │   │   ├── plan-feature.md
│   │   │   └── implement-feature.md
│   │   ├── bugs/fix-bug.md
│   │   └── chores/refactor.md
│   ├── agents/                   # Custom agent configs
│   │   └── specialist.md
│   ├── hooks/                    # Event-driven scripts
│   │   ├── pre_tool_use.py
│   │   └── post_tool_use.py
│   ├── settings.json             # Permissions, hooks config
│   └── logs/                     # Hook logs
│
├── adws/                         # Agentic Developer Workflows
│   ├── adw_modules/              # Shared utilities
│   │   ├── agent.py              # Execution primitives
│   │   ├── git_ops.py            # Git worktree management
│   │   └── status_tracker.py    # Multi-agent tracking
│   ├── adw_hello.py              # Simple workflow
│   └── adw_plan_build.py         # PITER workflow
│
├── specs/                        # Implementation plans
│   ├── spec_template.md          # Template
│   └── plan-{adw-id}-{name}.md  # Generated specs
│
├── ai_docs/                      # Agent knowledge base
│   ├── architecture.md           # System design
│   ├── api-guidelines.md         # API patterns
│   └── patterns/                 # Common patterns
│
├── agents/                       # Execution logs
│   └── {adw-id}/                 # Per-workflow logs
│       └── {agent-name}/
│           └── raw_output.txt
│
├── apps/                         # APPLICATION LAYER
│   └── my_app/                   # Your actual software
│       ├── main.py
│       ├── core/
│       └── tests/
│
├── tests/                        # Test suite
│   ├── test_example.py
│   └── conftest.py
│
├── pyproject.toml                # Dependencies
├── .env                          # Secrets (gitignored)
└── .gitignore
```

### What Goes Where

| Purpose | Location | Example |
|---------|----------|---------|
| Manual prompts | `.claude/commands/` | `features/plan.md` |
| Autonomous workflows | `adws/` | `adw_plan_build.py` |
| Agent utilities | `adws/adw_modules/` | `agent.py` |
| Specs | `specs/` | `plan-a1b2c3-auth.md` |
| Agent docs | `ai_docs/` | `architecture.md` |
| Your app | `apps/` | `my_app/main.py` |
| Tests | `tests/` | `test_feature.py` |
| Logs | `agents/` | `{id}/agent/raw_output.txt` |

---

## Glossary

### ADW (Agentic Developer Workflow)
Python script that orchestrates agent execution programmatically. Runs autonomously without human intervention.

**Source**: TAC-4

---

### AFK (Away From Keyboard)
Development pattern where agents work autonomously while developer is unavailable. Enabled by PITER framework.

**Source**: TAC-4

---

### Architect-Editor Pattern
Design pattern using reasoning model for planning (high context) and fast model for execution (low context).

**Source**: PAICC-5, Context Engineering

---

### Context Bundle
Snapshot of all context items (files, docs) and their token usage for a specific agent session. Used for measurement and optimization.

**Source**: Context Engineering

---

### Context Priming
Technique of using slash commands to load targeted context instead of giant files. Reduces context bloat.

**Source**: Context Engineering

---

### Core Four
The four always-present pillars: Context, Model, Prompt, Tools. All agent interactions involve all four.

**Source**: TAC-1

---

### Director Pattern
Autonomous iteration pattern where agent generates code, tests run, evaluator analyzes, and loop continues until success.

**Source**: PAICC-7

---

### In-Loop vs Out-Loop
In-Loop: Interactive, human in the conversation. Out-Loop: Autonomous, agent works independently.

**Source**: TAC-4

---

### KPIs (Key Performance Indicators)
Metrics for measuring agentic success: Attempts (iterations), Streak (consecutive successes), Size (complexity), Presence (human intervention).

**Source**: TAC-2

---

### Leverage Point
Strategic intervention point that multiplies agent capability. 12 total: 4 in-agent (Core Four) + 8 through-agent (docs, types, tests, etc.).

**Source**: TAC-2

---

### MCP (Model Context Protocol)
Protocol for creating servers that expose tools and resources to Claude. Alternative to Skills for external integrations.

**Source**: Claude Skills

---

### One-Shot Success
Agent executes task correctly on first attempt without iteration. Achieved by stacking leverage points.

**Source**: TAC-2

---

### PITER Framework
Plan → Implement → Test → Execute → Report. Framework for autonomous AFK development.

**Source**: TAC-4

---

### Primary Agent
Independent agent launched via Task tool for parallel execution. Unlike sub-agents, doesn't share context with parent.

**Source**: Claude Skills

---

### R&D Framework
Context management: Reduce (minimize tokens) or Delegate (offload to sub-agents). Only two strategies.

**Source**: Context Engineering

---

### Slash Command
Reusable prompt template stored in `.claude/commands/` and invoked via `/command-name` syntax.

**Source**: TAC-3

---

### Spec (Specification)
Comprehensive technical document detailing feature requirements, approach, tasks, and acceptance criteria. Lives in `specs/`.

**Source**: PAICC-5

---

### Sub-Agent
Agent spawned by parent agent to handle subtask. Shares conversation history but gets fresh context.

**Source**: Claude Skills

---

### Template Metaprompt
Prompt that generates other prompts dynamically. Level 6 of 7 Prompt Levels.

**Source**: Prompt Engineering

---

### Three-Legged Stool
Debugging framework: Check Context (80%) → Prompt (15%) → Model (5%) in order.

**Source**: PAICC-4

---

### 12 Leverage Points
Framework of strategic intervention points: 4 in-agent (Core Four) + 8 through-agent (documentation, types, tests, planning, workflows, specs, projects, KPIs).

**Source**: TAC-2

---

### Verification Loop
Pattern of testing and validating after every change before proceeding. Non-negotiable for agent success.

**Source**: PAICC-1

---

### Worktree
Git feature enabling multiple working directories from same repository. Enables parallel agent execution without conflicts.

**Source**: TAC-7

---

### 7 Prompt Levels
Progression: High-Level → Workflow → Control Flow → Delegate → Higher-Order → Template Metaprompt → Self-Improving.

**Source**: Prompt Engineering

---

## File Naming Conventions

### Commands
```
.claude/commands/
├── {category}/
│   └── {action}-{target}.md

Examples:
├── features/
│   ├── plan-feature.md
│   └── implement-feature.md
├── bugs/
│   └── fix-bug.md
└── chores/
    └── refactor-code.md
```

### ADWs
```
adws/
└── adw_{action}_{target}.py

Examples:
├── adw_hello.py
├── adw_plan_build.py
├── adw_multi_agent.py
└── adw_github_automation.py
```

### Hooks
```
.claude/hooks/
└── {event}_{when}.py

Examples:
├── pre_tool_use.py
├── post_tool_use.py
├── session_start.py
└── session_end.py
```

### Specs
```
specs/
└── {type}-{adw-id}-{description}.md

Examples:
├── plan-a1b2c3-authentication.md
├── feat-x7y8z9-user-profile.md
└── bug-p4q5r6-login-fix.md
```

### Agent Logs
```
agents/
└── {adw-id}/
    └── {agent-name}/
        └── raw_output.txt

Example:
└── a1b2c3d4/
    ├── planner/
    │   └── raw_output.txt
    └── implementor/
        └── raw_output.txt
```

---

## Common Patterns Quick Lookup

| Pattern | Use Case | File Reference |
|---------|----------|----------------|
| **Iterative Prompting** | Build features incrementally | principles.md |
| **Verification Loops** | Test after every change | principles.md |
| **Structured Output** | Type-safe LLM responses | principles.md |
| **Context Enrichment** | Pass computed data to LLM | principles.md |
| **Architect-Editor** | Separate planning from execution | principles.md, components.md |
| **Director Pattern** | Autonomous self-evaluation | principles.md |
| **Three-Legged Stool** | Debug agent failures | debugging.md |
| **Tool Autonomy** | Enable agent actions | components.md |
| **12 Leverage Points** | Stack for one-shot success | principles.md |
| **PITER** | AFK development workflow | principles.md, components.md |
| **One Agent One Purpose** | Specialized agents | components.md |
| **Sub-Agent** | Delegate subtasks | components.md |
| **Hooks** | Validate/log tool usage | components.md |
| **ai_docs/** | Agent knowledge base | components.md |
| **Specs** | Comprehensive planning | components.md |

---

## Command Quick Reference

### Essential Commands

```bash
# List files in context
/context

# Prime context from files
/prime

# Plan feature
/plan "Feature description"

# Implement from spec
/implement specs/plan-{id}.md

# Test module
/test module_name

# Build simple task
/build "Task description"
```

### Claude Code CLI

```bash
# Basic invocation
claude -p "Your prompt here"

# With specific model
claude -p "Prompt" --model sonnet

# Programmatic (from Python)
subprocess.run(["claude", "-p", prompt])
```

### Aider CLI

```bash
# Basic usage
aider main.py utils.py

# With architect mode
aider --o1-preview --architect spec.md

# With specific model
aider --sonnet main.py

# Read-only context
aider --read docs.md main.py
```

---

## Permission Patterns

### Minimal (Read-only)
```json
{"permissions": {"allow": ["Read"]}}
```

### Standard Development
```json
{
  "permissions": {
    "allow": [
      "Read",
      "Write",
      "Edit",
      "Bash(git checkout:*)",
      "Bash(git add:*)",
      "Bash(git commit:*)",
      "Bash(uv run:*)",
      "WebSearch"
    ]
  }
}
```

### Advanced (with testing)
```json
{
  "permissions": {
    "allow": [
      "Read", "Write", "Edit",
      "Bash(git:*)",
      "Bash(pytest:*)",
      "Bash(uv:*)",
      "WebSearch"
    ]
  }
}
```

---

## Model Selection Guide

| Task Type | Model | Why |
|-----------|-------|-----|
| Simple edits | Haiku | Fast, cheap |
| Standard development | Sonnet | Balance of quality/speed |
| Complex features | O1 | Deep reasoning |
| Architecture design | O1-preview | Maximum reasoning |
| Planning (Architect) | O1 | High context, reasoning |
| Execution (Editor) | Sonnet | Low context, fast |
| Evaluation | O1 / GPT-4o | Reasoning for analysis |
| Code generation | Sonnet / Haiku | Speed for execution |

---

## Context Optimization

### Token Budgets

- **Haiku**: ~200K tokens
- **Sonnet**: ~200K tokens
- **O1**: ~200K tokens (input)

### Strategies

**Reduce**:
- Remove unnecessary files
- Use read-only for docs
- Compact output formats
- Context priming

**Delegate**:
- Sub-agents for subtasks
- Architect-Editor split
- One Agent One Purpose
- Primary agents (parallel)

---

## Testing Patterns

### Basic Test
```python
def test_function():
    # Arrange
    expected = "value"

    # Act
    actual = function()

    # Assert
    assert actual == expected
```

### Parametrized Test
```python
@pytest.mark.parametrize("input,expected", [
    (1, 2),
    (2, 4),
])
def test_multiply(input, expected):
    assert input * 2 == expected
```

### With Fixture
```python
@pytest.fixture
def sample_data():
    return {"key": "value"}

def test_with_fixture(sample_data):
    assert sample_data["key"] == "value"
```

---

## Summary

This reference provides quick lookup for:

**Commands**: Structure, variables, sections
**Workflows**: ADW template, Coder.create pattern
**Directory Structure**: What goes where
**Glossary**: 30 essential terms
**File Naming**: Conventions for all file types
**Common Patterns**: Quick lookup table

**Remember**:
- Commands = Manual, in `.claude/commands/`
- ADWs = Autonomous, in `adws/`
- Specs = Planning, in `specs/`
- ai_docs = Agent knowledge, in `ai_docs/`
- Logs = Execution traces, in `agents/`

**Sources**:
- Framework Master sections 14, 19 (Quick Reference, Glossary)
- Production Playbook Appendix (Quick Reference)
- Pattern Catalog (all patterns)
- All TAC and PAICC lessons
