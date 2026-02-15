You are helping me create content for romanesco, a shared library of AI coding tool configurations. Romanesco manages skills, agents, prompts, MCP servers, apps, rules, hooks, and commands that get installed into tools like Claude Code, Cursor, and Gemini CLI.

I'm creating a new skill called "{{NAME}}". A skill is a domain knowledge package that gets loaded into an AI assistant's context. Skills live in their own directory and must have a SKILL.md file with YAML frontmatter.

Required structure:
  my-tools/skills/{{NAME}}/
  ├── SKILL.md          # Required: frontmatter (name, description) + skill content
  └── .romanesco.yaml      # Auto-generated, don't edit

SKILL.md format:
---
name: Human Readable Name
description: One-line description of what this skill provides
---

# Skill Title

Content that will be loaded into the AI assistant's context.
Skills typically include domain knowledge, workflows, conventions, and reference material.

Conventions:
- name: Human-readable, title case
- description: Concise, starts with verb or noun (e.g., "Comprehensive Atlassian workspace management")
- Content: Structured with headings, focused on what the AI needs to know

Help me write the SKILL.md content for "{{NAME}}". Ask me what domain or capability this skill should cover, then create the content.
