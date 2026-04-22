# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds


## bv + br (Graph-Aware Triage)

**Scope:** `bv` handles triage/planning. `br` handles creating, updating, closing issues.
**CRITICAL:** Use `--robot-*` flags only — bare `bv` launches an interactive TUI that blocks your session.

### Entry point
```bash
bv --robot-triage                    # start here — ranked work + dependency graph
bv --robot-triage --format toon      # token-efficient version
bv --robot-next                      # single top pick only
```

### Workflow
1. `bv --robot-triage` — find highest-impact work
2. `br update <id> --status=in_progress` — claim it
3. implement
4. `br close <id>`
5. `br sync --flush-only` + `git push`

*Full flag reference: `bv --help`, `br --help`*
