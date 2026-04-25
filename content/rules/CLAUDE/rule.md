# OpenWolf

@.wolf/OPENWOLF.md

This project uses OpenWolf for context management. Read and follow .wolf/OPENWOLF.md every session. Check .wolf/cerebrum.md before generating code. Check .wolf/anatomy.md before reading files.


# CLAUDE.md

Project instructions for Claude Code.

## Agent Delegation (Mandatory)

**Git operations** → Use `git-workflow` agent (commits, branches, PRs, reverts, stash)
- Exception: read-only commands (`git status`, `git diff`, `git log`) are fine directly

**Research tasks** → Use `research-agent` via Task tool before WebSearch/WebFetch
- Trigger: "find out about", "what's new in", release info, documentation lookup
- See `.claude/skills/research-delegation/` for full anti-rationalization checklist

## Issue Tracking

Uses **beads** (`bd`) for issue tracking. See `AGENTS.md` for workflow.

## PR Scope Discipline

Documentation PRs must contain only content changes and their direct dependencies. Infrastructure improvements noticed during docs work get captured and handled separately.

### The Dependency Test

For every non-content file in a docs PR, ask: **"Can this docs PR merge and function correctly without this change?"**

- **YES** → Defer it. Create a bead, handle in a separate PR.
- **NO** → Include it. It's a content dependency.

### Change Categories

| Category | Example | Action |
|----------|---------|--------|
| Content | New/updated `.mdx` pages, images, definitions, sharedTopics | Include in docs PR |
| Content dependency | New component imported by new page, sidebar entry for new page, schema change for new collection | Include — passes dependency test |
| Trivial blocker | Vale false positive blocking CI on your page | Prerequisite PR — merge first, then rebase docs PR |
| Unrelated improvement | Component refactor, CI/CD update, Vale rule addition, AI tooling | `bd create` → separate PR |

### Capture Flow

When you notice an improvement during docs work:
1. Run `bd create --title="..." --type=task --priority=3` to capture it
2. Continue with the docs work — don't context-switch
3. Create a Jira ticket only when work on the improvement actually begins

### Vale Hybrid Strategy

- **Trivial fixes** (false positive on your page, missing word in accept list): Create a prerequisite PR, merge it, rebase your docs branch
- **Complex work** (new rule, major vocabulary update): Create a bead, note the CI noise in the docs PR description, handle separately

### When Uncertain

If a file's classification is ambiguous — especially components that could be either a new dependency or an existing modification — **always ask the user**. The cost of a question is seconds; the cost of a wrong guess is a tangled PR.

## Core Principles

**Simplicity**: Only implement what's requested. No extra features, abstractions, or premature error handling.

**Incremental output**: Work in sequence (do step → write step → next), not gather-then-dump.

**Questions**: Always use AskUserQuestion tool, never plain text with numbered options.

## Mutual Accountability

We're collaborators working toward accuracy, not validation.

**Claude must:**
- Challenge assumptions directly ("That doesn't match my understanding")
- Correct with evidence - use WebFetch to verify before correcting
- Calibrate confidence - say "I think" when uncertain, not false certainty
- Treat "are you sure?" as a verification request - immediately fetch proof, don't defend first
- Disagree directly - "I disagree" or "That's not correct", not hedged uselessness

**User will:**
- Question confident output (Claude's tone ≠ correctness)
- Provide evidence when asked
- Correct Claude directly

**The goal:** Catch errors before they matter. Confidence is not evidence.

## MDX Component Imports

- **Use absolute paths** for component imports in MDX files: `/src/components/MyComponent.astro`
- Avoid relative paths like `../../components/` - Vite's virtual module resolution in `src/content/` can fail even when the path is valid on disk
- This applies to all MDX files in `src/content/docs/`
