---
name: kitchen-sink
description: Example command that populates every available frontmatter field for testing and reference
allowed-tools:
  - Read
  - Glob
  - Grep
  - Bash
context: fork
agent: Explore
model: claude-sonnet-4-20250514
disable-model-invocation: true
user-invocable: true
argument-hint: <target> [--format json|text]
---

# /kitchen-sink Command

This is an example slash command demonstrating every available metadata field in the canonical command format.

## Usage

```
/kitchen-sink $ARGUMENTS
```

## What It Does

1. Reads the target specified in the arguments
2. Analyzes it using the Explore agent personality
3. Outputs a summary in the requested format

## Examples

```
/kitchen-sink src/main.go
```

Analyzes the specified file.

```
/kitchen-sink . --format json
```

Analyzes the current directory and outputs JSON.

## Fields Demonstrated

- **name**: Slash command name (used as /kitchen-sink)
- **description**: Human-readable summary shown in command menus
- **allowed-tools**: Tools this command may use (Read, Glob, Grep, Bash)
- **context**: Execution context ("fork" means isolated conversation)
- **agent**: Agent personality to use (Explore)
- **model**: Preferred model for execution
- **disable-model-invocation**: Prevents automatic invocation by the model
- **user-invocable**: Whether the command appears in the slash command menu
- **argument-hint**: Usage hint shown in the menu
