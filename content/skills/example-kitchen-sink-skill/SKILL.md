---
name: Kitchen Sink Skill
description: Example skill that populates every available frontmatter field for testing and reference
allowed-tools:
  - Read
  - Glob
  - Grep
disallowed-tools:
  - Bash
  - Write
context: fork
agent: Explore
model: claude-sonnet-4-20250514
disable-model-invocation: true
user-invocable: true
argument-hint: <file-path> [--verbose]
---

# Kitchen Sink Skill

This is an example skill that demonstrates every available frontmatter field in the canonical skill format. It exists purely for testing and reference purposes.

## What This Skill Does

Nothing practical. It serves as a living reference for the complete set of metadata fields a skill can declare.

## Fields Demonstrated

- **name**: Display name shown in menus and listings
- **description**: Human-readable summary of the skill's purpose
- **allowed-tools**: Whitelist of tools this skill may use (Read, Glob, Grep)
- **disallowed-tools**: Tools explicitly forbidden (Bash, Write)
- **context**: Execution context ("fork" means isolated from main conversation)
- **agent**: Which agent personality to use (Explore)
- **model**: Preferred model for this skill
- **disable-model-invocation**: Prevents the model from invoking this skill automatically
- **user-invocable**: Whether users can trigger this skill from the command menu
- **argument-hint**: Usage hint shown when the skill appears in menus
