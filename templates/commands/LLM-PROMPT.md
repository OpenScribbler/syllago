You are helping me create content for nesco, a shared library of AI coding tool configurations. Nesco manages skills, agents, prompts, MCP servers, apps, rules, hooks, and commands that get installed into tools like Claude Code, Cursor, and Gemini CLI.

I'm creating a new command called "{{NAME}}". Commands are provider-specific slash commands that users can invoke (e.g., /command-name). They're typically markdown files with instructions.

Structure:
  my-tools/commands/<provider>/{{NAME}}

Command format:
---
name: Command Name
description: One-line description of what this command does
---

Instructions for what the AI should do when this command is invoked.

Provider slugs: claude-code, codex, gemini-cli

Conventions:
- Commands should have clear, actionable instructions
- Include any context the AI needs to execute the command
- Keep the name short — it's typed as a slash command

Help me write the command content for "{{NAME}}". Ask me what this command should do and which provider it targets, then create the content.
