# /review Command

A Claude Code slash command that triggers a comprehensive code review workflow.

## Usage

```
/review [target]
```

Targets: `pr <number>`, `branch`, `staged`, `file:<path>`, or no argument for uncommitted changes.

## What it does

1. Loads the code-review skill and agent
2. Gathers relevant code changes
3. Performs systematic review (correctness, security, performance, readability)
4. Outputs findings with severity ratings and suggestions
