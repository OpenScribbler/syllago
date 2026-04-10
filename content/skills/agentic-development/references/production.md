# Production Agentic Systems

Guide for deploying agentic systems to production.

---

## Architecture Maturity Phases

| Phase | Goal | Key Additions |
|-------|------|---------------|
| **MVA** | First working automation | Command + basic ADW |
| **Intermediate** | Reliable workflows | ai_docs/, specs/, validation |
| **Advanced** | Parallel execution | Worktrees, multi-agent |
| **Production** | Observable & secure | Hooks, monitoring, cost tracking |

## Production Readiness Checklist

### Architecture
- [ ] Agentic layer separated from application
- [ ] ai_docs/ comprehensive and current
- [ ] specs/ directory for planning
- [ ] Test coverage >70%

### Security
- [ ] API keys in environment variables (not code)
- [ ] Hooks validate dangerous operations
- [ ] Permissions scoped appropriately
- [ ] Secrets management implemented

### Observability
- [ ] Centralized logging configured
- [ ] Tool usage tracked
- [ ] Error monitoring active
- [ ] Context usage measured

### Operations
- [ ] CI/CD integration working
- [ ] Rollback procedures documented
- [ ] Incident response runbook exists

---

## Security Hardening

### PreToolUse Validation Hook

```python
# .claude/hooks/pre_tool_use.py
import sys, json, os, re

tool_name = os.environ.get("CLAUDE_TOOL_NAME", "")
tool_input = json.loads(os.environ.get("CLAUDE_TOOL_INPUT", "{}"))

BLOCKED_PATTERNS = [r'\brm\s+.*-[a-z]*r[a-z]*f', r'\bsudo\b', r'chmod\s+777']

if tool_name == 'Bash':
    command = tool_input.get('command', '')
    for pattern in BLOCKED_PATTERNS:
        if re.search(pattern, command):
            print(f"BLOCKED: Matched dangerous pattern")
            sys.exit(2)
sys.exit(0)
```

### Secrets Management
- Rule: Store secrets in `.env` (never commit). Load with `dotenv`. Access via `os.getenv()`.

### Permission Scoping
- Rule: Scope tools to minimum needed. Block sensitive paths (`/etc/`, `~/.ssh/`). Allowlist commands.

---

## Cost Tracking

- Rule: Track token usage per ADW/session. Log to `costs.jsonl` with timestamp, model, token counts, total cost.
- Implement `CostTracker` class wrapping model pricing: Sonnet ($3/$15 per 1M), Opus ($15/$75), Haiku ($0.80/$4).

## Monitoring

### PostToolUse Logging Hook
- Rule: Log every tool invocation to `tool_usage.jsonl` with timestamp, tool name, input, session ID.

```python
# .claude/hooks/post_tool_use.py
log_entry = {
    "timestamp": datetime.utcnow().isoformat(),
    "tool": os.environ.get("CLAUDE_TOOL_NAME"),
    "input": json.loads(os.environ.get("CLAUDE_TOOL_INPUT", "{}")),
    "session_id": os.environ.get("CLAUDE_SESSION_ID")
}
# Append to logs/tool_usage.jsonl
```

---

## Incident Response

### Agent Failure
1. Check logs → 2. Verify API key → 3. Check context size → 4. Apply Three-Legged Stool (see [debugging.md](debugging.md)) → 5. If persistent: reset and prime

### Cost Spike
1. Check cost logs → 2. Identify expensive workflows → 3. Review context for bloat → 4. Consider model downgrade (Opus → Sonnet) → 5. Add cost limits

### Security Breach
1. IMMEDIATELY rotate API keys → 2. Review hook logs → 3. Audit recent commits → 4. Strengthen validation → 5. Notify team

---

## Git Worktrees for Parallel Agents

```bash
git worktree add ../wt-feature-a -b feature-a
cd ../wt-feature-a && git sparse-checkout set apps/backend/
# Run agents in parallel worktrees, merge when complete
```

## Common Production Issues

| Issue | Root Cause | Fix |
|-------|------------|-----|
| Agent slow/failing | Context >150K tokens | Reset + delegate |
| High bills | Wrong model (Opus for simple tasks) | Use Sonnet |
| File corruption | Parallel writes to same files | Use worktrees |
| Hook not working | Permissions | `chmod +x hooks/` |
| 429 rate limits | Too many requests | Add delays |
| Leaked secrets | Committed .env | Rotate keys immediately |
