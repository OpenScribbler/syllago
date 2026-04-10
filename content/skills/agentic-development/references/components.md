# Agentic System Components

Component design guidance for building agentic systems.

---

## Component Selection

```
I need to...
    +- Execute a simple task Claude knows         → SLASH COMMAND
    +- Give Claude specialized domain knowledge   → SKILL
    +- Run complex multi-step workflow             → ADW (Python)
    +- Offload context-heavy work                  → SUB-AGENT (Task tool)
    +- Integrate external APIs                     → MCP SERVER
    +- Observe/modify Claude's behavior            → HOOKS
```

**Complexity order**: Command < Skill < ADW < Sub-agent < MCP < Hook

---

## Commands (Slash Commands)

- **What**: Prompt templates in `.claude/commands/` executed via `/command-name`.
- **When**: Manual workflows, reusable patterns (3+ uses), team collaboration.
- **Not when**: Agent-triggered tasks (use Skills) or one-off operations.
- **Key patterns**: Meta-prompts (description → spec), Higher-order prompts (spec → implementation).

## Skills

- **What**: Reusable domain knowledge loaded by agents on demand.
- **Structure**: `skills/name/SKILL.md` (entry point, <100 lines) + `references/` (deep content) + optional `Workflows/`.
- **When**: Content reusable by multiple agents, domain knowledge (not behavior), >50 lines of static content.

## ADWs (Agentic Developer Workflows)

- **What**: Python scripts that orchestrate agent execution.
- **When**: Fully unattended execution, multiple AI calls with deterministic logic between, external system integration, frequent tasks.

```python
#!/usr/bin/env -S uv run --script
from adw_modules.agent import execute_template, generate_short_id

def main():
    adw_id = generate_short_id()
    request = AgentTemplateRequest(
        agent_name="worker", slash_command="/build",
        args=["Task description"], adw_id=adw_id
    )
    response = execute_template(request)
```

## Agents

- **What**: Specialized agents with focused purposes and minimal context.
- **Key principle**: One Agent One Purpose.
- **When**: Parallel execution, context isolation, specialized expertise, context >100K tokens.

```yaml
---
name: agent-name
description: When to use this agent
tools: Read, Write, Edit, Bash
model: sonnet
---

You are a [role] specialist.

## Focus
- [Primary responsibility]

## Ignore
- [Out of scope]
```

## Hooks

- **What**: Event-driven scripts that run before/after tool execution.
- **Types**: PreToolUse (block dangerous ops), PostToolUse (log results), SessionStart/End (lifecycle).
- **When**: Security validation, audit logging, cross-cutting concerns.
- **Principle**: Hooks are for infrastructure, NOT business logic.

```python
# .claude/hooks/pre_tool_use.py
tool_name = os.environ.get("CLAUDE_TOOL_NAME", "")
tool_input = json.loads(os.environ.get("CLAUDE_TOOL_INPUT", "{}"))
if tool_name == "Bash":
    command = tool_input.get("command", "")
    if "rm -rf /" in command:
        print("BLOCKED: Dangerous command")
        sys.exit(2)  # Block execution
sys.exit(0)  # Allow
```

## MCP Servers

- **What**: Custom tools and MCP servers extending agent capabilities.
- **When**: External API/service integration, tools used in every session.
- **Decision**: Truly external system → MCP. Occasional use → Skill with progressive disclosure. Orchestration → Skill.

## Knowledge Bases (ai_docs/)

- **What**: Documentation for agents to understand your codebase.
- **When**: Context >50K tokens, agent makes incorrect assumptions, non-obvious codebase patterns.
- **Structure**: `ai_docs/README.md`, `architecture.md`, `api-guidelines.md`, `patterns/`.
- **Document**: Project structure, where to find things, coding standards, how to add features, common pitfalls.

## Specs

- **What**: Technical specifications guiding implementation.
- **When**: Complex features requiring upfront planning, multiple agents on same feature, one-shot execution goal.
- **Sections**: High-Level Objective → Mid-Level Objectives → Implementation Notes → Testing Strategy → Low-Level Tasks (with exact prompts).

---

## Integration Patterns

1. **Command → Spec → Implementation**: `/plan "Add auth"` → `specs/feat-auth.md` → `/implement specs/feat-auth.md`
2. **ADW → Multi-Agent → Worktrees**: Python ADW → creates worktrees → spawns parallel agents → aggregates results.
3. **Hook → Validation → Action**: Agent attempts tool use → PreToolUse validates → continue or block+log.

## Summary

| Component | When to Use |
|-----------|-------------|
| **Command** | Manual-triggered, reusable prompts |
| **Skill** | Domain knowledge, agent-triggered |
| **ADW** | Unattended autonomous workflows |
| **Agent** | Specialized parallel execution |
| **Hook** | Observability, security |
| **MCP** | External integrations |
| **ai_docs** | Codebase context |
| **Spec** | Complex feature planning |
