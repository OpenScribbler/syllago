# Token Optimization Guide

Patterns for minimizing token usage in agents without sacrificing effectiveness.

---

## When to Recommend a Wrapper Script

| Signal | Action |
|--------|--------|
| Tool output >100 lines typical | Wrapper needed |
| Agent pipes through grep/head 3+ times | Consolidate in wrapper |
| 3+ related commands with similar filtering | Unified wrapper |
| Verbose defaults (`go test -v`, `docker build`, `terraform plan`) | Wrapper with summary mode |

## Wrapper Script Specification

When creating a wrapper, define:
- **Name**: `<tool>-dev` or `<domain>-scan` (e.g., `go-dev`, `sec-scan`)
- **Commands table**: subcommand, input, output, filtering applied
- **Success output**: summary only. **Failure output**: errors + context.
- **Environment variables**: `WRAPPER_VERBOSE=1` for more, `WRAPPER_RAW=1` for unfiltered.
- **Error handling**: missing tool, command failure, timeout.

---

## File Reading Optimization

### Search Before Read
- Anti-pattern: Read 10+ files then analyze.
- Pattern: `Grep files_with_matches` → read only matching files.
- Savings: 80%+ on large codebases.

### Targeted Reading
- Anti-pattern: `Read large_file.go` (2000 lines).
- Pattern: `Grep pattern, -A=50` to find section, or `Read offset=450, limit=100`.
- Savings: 90%+ for large files.

### Progressive Detail
```
Level 1: Grep files_with_matches → list of relevant files
Level 2: Grep content, head_limit=20 → functions in those files
Level 3: Read offset/limit → specific function body
```

---

## Command Output Optimization

| Tool Type | Strategy |
|-----------|----------|
| Build tools | Errors only |
| Test runners | Summary on pass, details on fail |
| Linters | Limit by severity (`--severity HIGH,CRITICAL`) |
| Scanners | Count first (`sec-scan summary`), details if needed |
| Kubectl | `-o wide` not `-o yaml` |

### Output Limits

| Output Type | Limit |
|-------------|-------|
| Build errors | 30-50 lines |
| Test failures | 5-10 per failure |
| Lint issues | 20-30 issues |
| Search results | 15-20 matches |
| Log output | 50-100 lines |

---

## Skill Content Optimization

### SKILL.md Sizing
| Size | Assessment |
|------|------------|
| <50 lines | Excellent |
| 50-100 lines | Good |
| 100-150 lines | Extract to references |
| >150 lines | Must extract |

### Reference Loading Strategy
- Include file sizes in routing table so agents know cost of loading.
- Guide selective loading: "Quick check = SKILL.md only. Full audit = + security.md."

---

## Agent Prompt Optimization

### Size Guidelines

| Section | Target |
|---------|--------|
| Persona | 2-3 sentences |
| Skills to Load | 5-10 lines |
| Core Principles | 3-5 bullets |
| Workflow | 10-20 lines |
| Confidence Indicator | 10-15 lines |
| Boundaries | 5-10 lines |
| Context Awareness | 5-10 lines |
| **Total** | **<150 lines preferred, <200 acceptable** |

### Content Placement

| Content Type | Location |
|--------------|----------|
| Commands, checklists, patterns | Skill |
| 1-2 examples anchoring behavior | Agent |
| 3+ examples | Skill reference |
| Workflow, boundaries | Agent |

---

## Optimization Checklist

### Agent
- [ ] Persona is 2-3 sentences
- [ ] No embedded checklists >10 items
- [ ] No embedded command documentation
- [ ] Skills referenced, not duplicated

### Skill
- [ ] SKILL.md <100 lines
- [ ] Quick reference in entry point
- [ ] Deep content in references/
- [ ] Clear when-to-load guidance

### Commands
- [ ] Wrapper scripts for verbose tools
- [ ] Output limits specified
- [ ] Fallback for missing tools

### File Operations
- [ ] Search before read
- [ ] Limits on large files (>200 lines)
- [ ] Progressive detail loading
