---
name: Kitchen Sink Prompt
description: Example prompt that populates every available frontmatter field for testing and reference
providers:
  - claude-code
  - gemini-cli
  - copilot-cli
---

# Kitchen Sink Prompt

This is an example prompt template demonstrating every available frontmatter field.

## Context

You are reviewing {{language}} code in the {{framework}} framework.

## Task

Analyze the following code and provide:

1. **Summary** - What the code does in one sentence
2. **Quality assessment** - Rate from 1-5 with justification
3. **Improvements** - Specific, actionable suggestions

## Code

```{{language}}
{{code}}
```

## Constraints

- Keep the response under 500 words
- Focus on the most impactful improvements first
- Use the project's existing conventions

## Fields Demonstrated

- **name**: Display name for the prompt
- **description**: Human-readable summary
- **providers**: List of provider slugs this prompt supports
