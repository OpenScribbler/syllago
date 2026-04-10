# Getting Started with Agentic Coding

Complete MVA (Minimum Viable Agentic) setup in 2-4 hours.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Core Concepts](#core-concepts)
3. [Directory Structure](#directory-structure)
4. [First Slash Command](#first-slash-command)
5. [First Agent Primitive](#first-agent-primitive)
6. [First ADW Workflow](#first-adw-workflow)
7. [Verification](#verification)
8. [Next Steps](#next-steps)

---

## Prerequisites

### Required Tools

Install these before starting:

1. **Claude Code CLI**
   ```bash
   brew install claude
   claude --version  # Verify installation
   ```

2. **UV (Python Package Manager)**
   ```bash
   curl -LsSf https://astral.sh/uv/install.sh | sh
   uv --version
   ```

3. **Git**
   ```bash
   git --version  # Usually pre-installed
   ```

4. **Python 3.11+**
   ```bash
   python3 --version
   ```

### Required Accounts

**Anthropic API Key**:
- Sign up: https://console.anthropic.com
- Create key: Settings → API Keys
- Save in `.env`: `ANTHROPIC_API_KEY=your_key_here`
- Never commit this to git

### Initial Setup

```bash
mkdir my-agentic-project
cd my-agentic-project
git init
echo "ANTHROPIC_API_KEY=your_key_here" > .env
echo ".env" >> .gitignore
git add .gitignore
git commit -m "Initial commit"
```

---

## Core Concepts

### What is Agentic Coding?

**Phase 1 (AI-Assisted)**: Human drives, AI assists with suggestions
**Phase 2 (Agentic)**: AI drives with tools, human reviews results

**The differentiator**: Tools. When AI can execute bash, read/write files, run tests, and commit to git, it becomes autonomous.

### The Four Pillars

1. **Context** - What agent can see (files, docs, history)
2. **Model** - Intelligence (reasoning vs speed)
3. **Prompt** - Instructions (the fundamental unit)
4. **Tools** - Actions (THE Phase 2 differentiator)

### Key Philosophy

- **Incremental Everything**: Small, verified steps
- **Verification at Every Level**: Test after each change
- **Natural Language as Interface**: Express WHAT, not HOW
- **One-Shot as Goal**: Reduce iteration through preparation

---

## Directory Structure

Create this structure:

```bash
mkdir -p .claude/commands
mkdir -p adws/adw_modules
mkdir -p apps/my_app
```

**Result**:
```
my-agentic-project/
├── .claude/
│   └── commands/          # Slash commands (reusable prompts)
├── adws/
│   ├── adw_modules/       # Agent primitives
│   └── adw_*.py           # Workflows
├── apps/
│   └── my_app/            # Your application
└── pyproject.toml         # Dependencies
```

**The Agentic Layer**: `.claude/` and `adws/` orchestrate development of `apps/`

---

## First Slash Command

### What is a Slash Command?

A reusable prompt template stored in `.claude/commands/`. Invoked via `claude -p "/command_name"`.

### Create /build Command

```bash
cat > .claude/commands/build.md << 'EOF'
# Build Command

Execute tasks based on natural language instructions.

## Variables
task_description: $1

## Instructions

Execute the task: `{task_description}`

1. Identify relevant files
2. Make minimal, focused changes
3. Verify changes work

## Report

After completion, report:
- Files modified
- Summary of changes
- Any issues encountered
EOF
```

### Test the Command

```bash
claude -p "/build Add version 0.1.0 to apps/my_app/README.md"
```

Claude should modify the file and report results.

---

## First Agent Primitive

### What is This?

`agent.py` provides functions for invoking Claude Code programmatically from Python scripts.

### Create pyproject.toml

```bash
cat > pyproject.toml << 'EOF'
[project]
name = "my-agentic-project"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = [
    "pydantic>=2.0.0",
    "rich>=13.0.0",
]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"
EOF
```

### Initialize Environment

```bash
uv sync  # Creates .venv/ automatically
```

### Create agent.py

```bash
cat > adws/adw_modules/agent.py << 'EOF'
#!/usr/bin/env python3
"""Agent execution primitives for programmatic Claude Code invocation."""
import subprocess
import json
from pathlib import Path
from typing import Optional, List
from pydantic import BaseModel


class AgentTemplateRequest(BaseModel):
    """Request to execute a slash command."""
    agent_name: str
    slash_command: str
    args: List[str]
    adw_id: str
    model: str = "sonnet"
    working_dir: Optional[str] = None


class AgentPromptResponse(BaseModel):
    """Response from agent execution."""
    output: str
    success: bool
    session_id: Optional[str] = None


def execute_template(request: AgentTemplateRequest) -> AgentPromptResponse:
    """Execute a slash command via Claude Code."""
    prompt = f"{request.slash_command} {' '.join(request.args)}"

    output_dir = Path("agents") / request.adw_id / request.agent_name
    output_dir.mkdir(parents=True, exist_ok=True)

    cmd = [
        "claude",
        "-p", prompt,
        "--model", request.model,
        "--dangerously-skip-permissions"
    ]

    cwd = request.working_dir if request.working_dir else None

    try:
        result = subprocess.run(
            cmd,
            cwd=cwd,
            capture_output=True,
            text=True,
            timeout=300
        )

        output_file = output_dir / "raw_output.txt"
        output_file.write_text(result.stdout)

        return AgentPromptResponse(
            output=result.stdout,
            success=result.returncode == 0
        )
    except subprocess.TimeoutExpired:
        return AgentPromptResponse(
            output="Command timed out",
            success=False
        )
    except Exception as e:
        return AgentPromptResponse(
            output=f"Error: {str(e)}",
            success=False
        )


def generate_short_id() -> str:
    """Generate 8-character tracking ID."""
    import uuid
    return uuid.uuid4().hex[:8]
EOF
```

### Verify

```bash
uv run python -c "from adws.adw_modules.agent import execute_template; print('OK')"
```

---

## First ADW Workflow

### What is an ADW?

An Agentic Developer Workflow - Python script that orchestrates agent execution programmatically.

### Create adw_hello.py

```bash
cat > adws/adw_hello.py << 'EOF'
#!/usr/bin/env -S uv run --script
"""ADW: Hello World - Basic agent invocation."""
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
    """Execute a simple build task via agent."""
    console = Console()

    console.print(Panel.fit(
        "[bold cyan]ADW: Hello World[/bold cyan]\n"
        "Demonstrating basic agent invocation",
        border_style="cyan"
    ))

    # Generate tracking ID
    adw_id = generate_short_id()
    console.print(f"\n[yellow]Workflow ID:[/yellow] {adw_id}")

    # Build request
    request = AgentTemplateRequest(
        agent_name="builder",
        slash_command="/build",
        args=["Add 'Hello from ADW!' to apps/my_app/README.md"],
        adw_id=adw_id,
        model="sonnet"
    )

    # Execute agent
    console.print("\n[cyan]Executing agent...[/cyan]")
    response = execute_template(request)

    # Report results
    if response.success:
        console.print("\n[green]✓ Success![/green]")
        console.print("\n[bold]Agent Output:[/bold]")
        console.print(response.output)
    else:
        console.print("\n[red]✗ Failed[/red]")
        console.print(f"\n[bold]Error:[/bold]\n{response.output}")


if __name__ == "__main__":
    main()
EOF
```

### Run the Workflow

```bash
chmod +x adws/adw_hello.py
./adws/adw_hello.py
```

**Expected Output**:
```
┌─────────────────────────────────────┐
│ ADW: Hello World                    │
│ Demonstrating basic agent invocation│
└─────────────────────────────────────┘

Workflow ID: a3f9c8e1

Executing agent...

✓ Success!

Agent Output:
[Claude's response showing file modification]
```

### Verify Changes

```bash
cat apps/my_app/README.md
# Should contain "Hello from ADW!"
```

---

## Verification

### Checklist

- [ ] Directory structure created (`.claude/`, `adws/`, `apps/`)
- [ ] `pyproject.toml` exists and `uv sync` succeeded
- [ ] `.env` file with API key (not committed to git)
- [ ] `/build` command exists at `.claude/commands/build.md`
- [ ] Manual command works: `claude -p "/build <task>"`
- [ ] `agent.py` exists and imports successfully
- [ ] `adw_hello.py` executes without errors
- [ ] Agent modified `apps/my_app/README.md`
- [ ] Output saved to `agents/<id>/builder/raw_output.txt`

### Common Issues

**"claude: command not found"**
- Solution: Check PATH, restart terminal, reinstall CLI

**"ANTHROPIC_API_KEY not found"**
- Solution: Verify `.env` file exists with valid key

**"Module not found" errors**
- Solution: Run `uv sync` from project root

**Agent times out**
- Solution: Check internet connection, API key validity

---

## Next Steps

### You've Built

- ✓ MVA directory structure
- ✓ First slash command (`/build`)
- ✓ Agent execution primitive (`agent.py`)
- ✓ First ADW workflow (`adw_hello.py`)

### Immediate Next Steps

1. **Create more commands**:
   ```bash
   # .claude/commands/test.md
   # .claude/commands/refactor.md
   ```

2. **Add ai_docs/** for agent knowledge:
   ```bash
   mkdir ai_docs
   echo "# Project Overview" > ai_docs/README.md
   ```

3. **Build spec-based workflow**:
   ```bash
   mkdir specs
   # Create specs/feature.md
   # Reference from ADW
   ```

### Intermediate Phase

When ready to scale:
- **Multi-file refactoring** with structured ADWs
- **Comprehensive ai_docs/** for context
- **Pydantic models** for structured output
- **Test-driven workflows** with verification loops

See **patterns.md** for common patterns and **production.md** for deployment.

### Learning Path

**Next 2-4 hours**: Create 3-5 slash commands for your domain
**Next 1-2 days**: Build ADW with multiple agent steps
**Next week**: Add ai_docs/, implement verification loops
**Next month**: Multi-agent orchestration with worktrees

---

## Quick Reference

### Run Commands

```bash
# Manual command
claude -p "/build Add feature X"

# ADW workflow
./adws/adw_hello.py

# With UV explicitly
uv run adws/adw_hello.py
```

### Key Files

- **Commands**: `.claude/commands/*.md`
- **Primitives**: `adws/adw_modules/*.py`
- **Workflows**: `adws/adw_*.py`
- **Docs**: `ai_docs/*.md`
- **Specs**: `specs/*.md`

### Essential Patterns

1. **Iterative Prompting**: One feature per prompt
2. **Verification Loops**: Test after each change
3. **Slash Commands**: Reusable prompt templates
4. **ADW Pattern**: Track ID → Request → Execute → Report

---

**Source**: framework-production-playbook.md (Phase 1), framework-master.md (Section 17)
**Last Updated**: 2025-10-31
**Lines**: ~300
