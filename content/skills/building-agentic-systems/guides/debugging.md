# Debugging Agentic Systems

Systematic troubleshooting for agent failures.

## Table of Contents

1. [The Three-Legged Stool](#the-three-legged-stool)
2. [Common Failure Modes](#common-failure-modes)
3. [Debug Checklist](#debug-checklist)
4. [Context Issues](#context-issues)
5. [Prompt Issues](#prompt-issues)
6. [Model Issues](#model-issues)
7. [Quick Fixes Table](#quick-fixes-table)

---

## The Three-Legged Stool

The diagnostic framework for agent failures. Check in order:

```
        AI FAILURE
             │
    ┌────────┼────────┐
    │        │        │
┌───▼──┐ ┌──▼───┐ ┌──▼────┐
│CONTEXT│PROMPT │ MODEL  │
│ 80%  ││ 15%  ││  5%   │
└──────┘ └──────┘ └───────┘
```

**The Rule**: 95% of failures are Context or Prompt. Don't blame the model first.

### 1. Check Context (80% of failures)

**What to Check**:
- Are all necessary files added to agent context?
- Is agent in correct working directory?
- Are dependencies/imports visible?
- Is relevant documentation included?
- Is context too large (causing truncation)?

**Example**:
```bash
# Agent creates import errors

# ❌ WRONG: "The AI is broken"

# ✅ CORRECT: Check context
$ aider --list-models  # Check which files in context
Added files:
  - src/main.py
Missing files:
  - src/utils.py (trying to import from)

# FIX: Add missing file
$ aider src/main.py src/utils.py
```

### 2. Check Prompt (15% of failures)

**What to Check**:
- Is intent clear and unambiguous?
- Are there conflicting instructions?
- Is specification complete?
- Are examples provided where needed?
- Is output format specified?

**Example**:
```markdown
# ❌ VAGUE PROMPT
"Add error handling"

# ✅ SPECIFIC PROMPT
"Add try/except blocks around file I/O operations in main.py.
Catch FileNotFoundError and print user-friendly message.
Catch other exceptions and log to errors.log file.
Include function names in error messages."
```

### 3. Check Model (5% of failures)

**What to Check**:
- Is model appropriate for task complexity?
- Does task require reasoning vs speed?
- Are model limitations relevant?
- Is context within token limits?

**Example**:
```bash
# Task: Complex algorithm requiring reasoning
# Current: claude-3-5-haiku (fast, cheap)

# FIX: Use reasoning model
$ aider --opus-or-o1-preview algorithm.py
```

---

## Common Failure Modes

### Symptom: Agent Did Nothing

**Check**:
1. **Permissions** - Does agent have required tool access?
2. **Prompt clarity** - Is the task actionable?
3. **Context** - Can agent see what needs changing?

**Fix**:
```json
// Check .claude/settings.json
{
  "permissions": {
    "allow": [
      "Read",
      "Write",
      "Edit",
      "Bash(git:*)"
    ]
  }
}
```

### Symptom: Agent Made Wrong Changes

**Check**:
1. **Context** - Missing relevant files or docs?
2. **Prompt** - Ambiguous or conflicting instructions?
3. **Examples** - Did you provide correct examples?

**Fix**:
```markdown
# Add specific examples to prompt
"Implement authentication like we did in user_service.py.
Use the same JWT pattern and error handling."
```

### Symptom: Agent Failed with Error

**Check**:
1. **Context** - Does agent have all imports?
2. **Environment** - Are dependencies installed?
3. **Permissions** - Can agent execute commands?

**Fix**:
```bash
# Verify environment
$ uv sync  # Install dependencies
$ uv run python -c "import module"  # Test imports
```

### Symptom: Agent Ran But No Output

**Check**:
1. **Output location** - Where is output supposed to go?
2. **Prompt** - Did you specify output format?
3. **Logs** - Check agent session logs

**Fix**:
```markdown
# Specify output clearly
"After analysis, REPORT your findings here in markdown format.
Include:
1. Summary
2. Key findings
3. Recommendations"
```

---

## Debug Checklist

Use this step-by-step when agent fails:

### Step 1: Context Verification

```bash
# 1.1 List files in context
$ aider --list-models

# 1.2 Verify working directory
$ pwd
$ ls -la

# 1.3 Check file paths
$ ls -la path/to/expected/file.py

# 1.4 View current context size
$ /context  # In Claude session
```

### Step 2: Prompt Analysis

```markdown
# 2.1 Review your prompt
- Is it specific?
- Does it have concrete examples?
- Are requirements complete?

# 2.2 Check for conflicts
- Multiple contradictory instructions?
- Impossible constraints?

# 2.3 Test with minimal prompt
Try simplest possible version first
```

### Step 3: Model Check

```bash
# 3.1 Is model appropriate?
Simple task → Haiku
Complex task → Sonnet
Reasoning task → O1

# 3.2 Check token usage
$ /context  # See token count
If > 100K, consider context reduction

# 3.3 Try different model
$ aider --model claude-sonnet-4
```

### Step 4: Logs Investigation

```bash
# 4.1 Check agent logs
$ ls agents/{adw-id}/

# 4.2 Review raw output
$ cat agents/{adw-id}/agent-name/raw_output.txt

# 4.3 Check hook logs
$ cat .claude/logs/pre_tool_use.log
$ cat .claude/logs/post_tool_use.log
```

---

## Context Issues

**Issue**: Missing files in context

**Symptoms**:
- Import errors
- "File not found" errors
- Agent creates duplicate code

**Fix**:
```bash
# Add missing files explicitly
$ aider src/main.py src/utils.py src/config.py

# For multi-file projects, add systematically
$ aider src/**/*.py
```

---

**Issue**: Wrong working directory

**Symptoms**:
- Path errors
- Files created in wrong location
- Relative imports fail

**Fix**:
```bash
# Verify location
$ pwd
/Users/dev/project

# Navigate to correct directory
$ cd /correct/path
$ aider main.py
```

---

**Issue**: Missing documentation

**Symptoms**:
- Agent uses outdated patterns
- Agent makes incorrect assumptions
- Inconsistent with codebase style

**Fix**:
```bash
# Add ai_docs
$ mkdir -p ai_docs
$ cat > ai_docs/architecture.md << EOF
# Architecture Guide

## Directory Structure
...

## Coding Patterns
...
EOF

# Reference in prompt
"Read ai_docs/architecture.md for project context"
```

---

**Issue**: Context too large

**Symptoms**:
- Slow responses
- Truncated output
- High costs
- Context window errors

**Fix**:
```bash
# Strategy 1: Reduce files
$ aider specific_file.py  # Only what's needed

# Strategy 2: Use sub-agents
# Delegate to specialized agents with minimal context

# Strategy 3: Context priming
# Use /prime command to load only essentials
```

---

## Prompt Issues

**Issue**: Vague intent

**Symptoms**:
- Agent implements wrong solution
- Multiple iterations needed
- Partial implementation

**Fix**:
```markdown
# ❌ BAD
"Add database support"

# ✅ GOOD
"Add PostgreSQL database support using SQLAlchemy.
- Use asyncpg driver
- Create connection pool (max 10 connections)
- Add health check endpoint
- Store connection string in .env"
```

---

**Issue**: Conflicting instructions

**Symptoms**:
- Agent makes inconsistent changes
- Some requirements ignored
- Unpredictable behavior

**Fix**:
```markdown
# ❌ BAD
"Make it fast but also add detailed logging to everything"

# ✅ GOOD
"Optimize critical path (payment processing).
Add detailed logging only to error cases.
Keep INFO logging for audit trail."
```

---

**Issue**: Missing requirements

**Symptoms**:
- Agent asks clarifying questions
- Incomplete implementation
- Missing edge cases

**Fix**:
```markdown
# ✅ COMPREHENSIVE PROMPT
"Add user authentication endpoint:
- POST /api/auth/login
- Accept: email (string), password (string)
- Validate: email format, password length >= 8
- Return: JWT token (expires 24h) on success
- Return: 401 on invalid credentials
- Return: 422 on validation errors
- Log: all authentication attempts to auth.log
- Rate limit: 5 attempts per minute per IP"
```

---

## Model Issues

**Issue**: Model too weak for task

**Symptoms**:
- Poor quality code
- Logical errors
- Doesn't understand complex requirements

**Fix**:
```bash
# Current: Haiku (fast/cheap)
# Task requires: Reasoning

# Switch to stronger model
$ aider --opus-or-o1-preview main.py
```

---

**Issue**: Model too strong for task

**Symptoms**:
- Slow responses
- High costs
- Over-engineered solutions

**Fix**:
```bash
# Current: O1 (reasoning)
# Task: Simple file rename

# Switch to fast model
$ aider --sonnet-or-haiku files.py
```

---

**Issue**: Context exceeds token limit

**Symptoms**:
- "Token limit exceeded" errors
- Truncated responses
- Missing information in output

**Fix**:
```bash
# Strategy 1: Use sparse context
$ aider --read ai_docs/ src/main.py  # Read-only docs

# Strategy 2: Architect-Editor pattern
# High context for planning, low context for execution

# Strategy 3: Sub-agents
# Parallel execution with isolated contexts
```

---

## Quick Fixes Table

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| ModuleNotFoundError | Missing file in context | Add imported file: `aider main.py utils.py` |
| Wrong file location | Wrong working directory | `cd /correct/path` |
| Outdated API usage | Missing documentation | Add API docs to ai_docs/ |
| Breaking changes | Ambiguous prompt | Add constraint: "WITHOUT changing signature" |
| Partial implementation | Missing requirements | List all requirements explicitly |
| Poor code quality | Model too weak | Use `--opus-or-o1-preview` |
| Slow responses | Model too strong | Use `--sonnet-or-haiku` |
| High costs | Context bloat | Reduce files, use sub-agents |
| Import errors | Dependencies not installed | `uv sync` |
| Permission denied | Tool not allowed | Add to .claude/settings.json |
| Agent did nothing | Unclear task | Make prompt actionable |
| Hallucinated code | Missing context | Add relevant files/docs |
| Tests fail | No examples | Provide test fixtures |
| Inconsistent style | No style guide | Add ai_docs/style.md |
| Wrong architecture | No design doc | Add ai_docs/architecture.md |

---

## Debugging Workflow

Step-by-step process:

```
1. Agent fails
   ↓
2. Don't panic or blame model
   ↓
3. Check CONTEXT first (80% chance)
   - Files in context?
   - Correct directory?
   - Dependencies visible?
   ↓
   Context OK?
   ↓
4. Check PROMPT second (15% chance)
   - Clear intent?
   - Complete requirements?
   - No conflicts?
   ↓
   Prompt OK?
   ↓
5. Check MODEL last (5% chance)
   - Appropriate for task?
   - Token limits OK?
   - Right capabilities?
   ↓
6. Check LOGS
   - agents/{adw-id}/
   - .claude/logs/
   ↓
7. Fix identified issue
   ↓
8. Test again
   ↓
9. Document solution in ai_docs/
```

---

## Advanced Debugging

### Director Pattern Debugging

When Director fails after max iterations:

```python
# Check:
1. Are tests actually validating correctly?
2. Is evaluator model strong enough?
3. Are error messages actionable?
4. Is coder model capable enough?

# Fix:
- Improve test specificity
- Use stronger evaluator (o1-preview)
- Add detailed error logging
- Switch to better coder model
```

### Multi-Agent Debugging

When parallel agents fail:

```bash
# Check each agent individually
$ ls agents/{adw-id}/

# Review each agent's logs
$ cat agents/{adw-id}/agent-1/raw_output.txt
$ cat agents/{adw-id}/agent-2/raw_output.txt

# Check for conflicts
- Are agents modifying same files?
- Are worktrees properly isolated?
- Are branches correctly named?
```

### Hook Debugging

When hooks block or fail:

```bash
# Check hook logs
$ cat .claude/logs/pre_tool_use.log

# Test hook manually
$ echo '{"tool_name":"Bash","params":{"command":"git status"}}' | \
  python3 .claude/hooks/pre_tool_use.py

# Verify exit codes
$ echo $?  # 0=allow, 2=block
```

---

## Prevention

Best practices to avoid failures:

**1. Start Simple**
- Begin with minimal prompt
- Add complexity incrementally
- Verify each step

**2. Use ai_docs/**
- Document architecture
- Explain patterns
- Provide examples

**3. Test Continuously**
- Verify after each change
- Don't compound errors
- Keep tests comprehensive

**4. Measure Everything**
- Track token usage
- Log all actions
- Monitor success rates

**5. Learn from Failures**
- Document solutions
- Update ai_docs/
- Refine prompts

---

## Summary

**The Three-Legged Stool**: Check Context (80%) → Prompt (15%) → Model (5%)

**Most Common Failures**:
1. Missing files in context
2. Wrong working directory
3. Vague prompts
4. Missing requirements
5. Model too weak for task

**Quick Wins**:
- Add ai_docs/ with architecture and patterns
- Be specific in prompts (include examples)
- Start with minimal context, add as needed
- Use appropriate model for task complexity
- Check logs when troubleshooting

**Remember**: 95% of failures are fixable by improving context or prompt. Don't rush to blame the model.

**Sources**:
- PAICC-4 (Three-Legged Stool pattern)
- Framework Master sections 8, 11 (Anti-Patterns, Debugging)
- Framework Decision Trees (Debugging section)
- Pattern Catalog (Three-Legged Stool pattern)
