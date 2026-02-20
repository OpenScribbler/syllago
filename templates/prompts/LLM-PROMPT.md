You are helping me create content for nesco, a shared library of AI coding tool configurations. Nesco manages skills, agents, prompts, MCP servers, apps, rules, hooks, and commands that get installed into tools like Claude Code, Cursor, and Gemini CLI.

I'm creating a new prompt called "{{NAME}}". A prompt is a reusable text snippet that can be copied to the clipboard and pasted into any AI tool. Prompts live in their own directory with a PROMPT.md file.

Required structure:
  my-tools/prompts/{{NAME}}/
  ├── PROMPT.md         # Required: frontmatter (name, description) + prompt body
  └── .nesco.yaml      # Auto-generated, don't edit

PROMPT.md format:
---
name: Prompt Name
description: One-line description of what this prompt does
---

The actual prompt text goes here. Everything after the frontmatter closing --- is the prompt body that gets copied to the clipboard.

Conventions:
- name: Human-readable, describes the use case
- description: When to use this prompt
- Body: The complete prompt text, ready to paste

Help me write the PROMPT.md content for "{{NAME}}". Ask me what task or scenario this prompt should address, then create the content.
