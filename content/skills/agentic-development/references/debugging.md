# Debugging Agentic Systems

Systematic troubleshooting using the Three-Legged Stool framework.

---

## The Three-Legged Stool

**The Rule**: 95% of failures are Context or Prompt. Don't blame the model first.

```
Check in order:  CONTEXT (80%)  →  PROMPT (15%)  →  MODEL (5%)
```

## Check Context (80% of failures)

- Are all necessary files in agent context? (most common: missing imports/dependencies)
- Is agent in correct working directory?
- Is context too large (causing truncation or confusion)?
- Is relevant documentation/ai_docs included?

**Fix**: Add missing files, verify directory, reduce context if bloated.

## Check Prompt (15% of failures)

- Is intent clear and unambiguous? (Bad: "Add error handling". Good: "Add try/except around file I/O in main.py, catch FileNotFoundError.")
- Are there conflicting instructions?
- Are requirements complete? (endpoints, validation rules, return types, etc.)
- Is output format specified?

**Fix**: Be specific, add examples, remove contradictions.

## Check Model (5% of failures)

- Simple task → Haiku. Complex task → Sonnet. Reasoning task → Opus.
- If >100K tokens, consider context reduction before upgrading model.

---

## Common Failure Modes

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| Agent did nothing | Unclear task or missing permissions | Make prompt actionable, verify tool access |
| Wrong changes | Missing context or ambiguous prompt | Add relevant files, include examples |
| Import/module errors | Missing file in context | Add imported file to context |
| Wrong file locations | Wrong working directory | Verify with `pwd`, correct path |
| Outdated API usage | Missing documentation | Add API docs to ai_docs/ |
| Partial implementation | Incomplete requirements | List all requirements explicitly |
| Hallucinated code | Missing context | Add relevant source files |
| Breaking changes | Ambiguous prompt | Add constraint: "WITHOUT changing signature" |
| Slow/expensive | Context bloat | Reduce files, use sub-agents |
| Inconsistent output | No confidence levels | Add confidence indicator |

## Debug Checklist

1. **Context verification**: List files in context. Verify working directory. Check paths exist. View context size.
2. **Prompt analysis**: Specific? Complete requirements? Concrete examples? No contradictions?
3. **Model check**: Appropriate for task complexity? Within token limits?
4. **Logs**: Check agent session logs, hook logs, raw output.

## Prevention Best Practices

1. Start simple -- minimal prompt, add complexity incrementally
2. Use ai_docs/ to document architecture, patterns, conventions
3. Verify after each change -- test immediately
4. Document solutions when you find them -- update ai_docs/
5. Track token usage to catch context bloat early
