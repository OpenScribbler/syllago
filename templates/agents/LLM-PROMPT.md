You are helping me create content for syllago, a shared library of AI coding tool configurations. Syllago manages skills, agents, prompts, MCP servers, apps, rules, hooks, and commands that get installed into tools like Claude Code, Cursor, and Gemini CLI.

I'm creating a new agent called "{{NAME}}". An agent is a specialized AI persona with specific instructions, behavior guidelines, and capabilities. Agents live in their own directory with an AGENT.md file.

Required structure:
  local/agents/{{NAME}}/
  ├── AGENT.md          # Required: frontmatter (name, description) + agent instructions
  └── .syllago.yaml      # Auto-generated, don't edit

AGENT.md format:
---
name: Agent Name
description: One-line description of what this agent does
---

# Agent Name

Agent-specific instructions, persona definition, and behavior guidelines.

Conventions:
- name: Human-readable, describes the role
- description: What the agent specializes in
- Content: Clear instructions for how the AI should behave when this agent is active

Help me write the AGENT.md content for "{{NAME}}". Ask me what role or specialization this agent should have, then create the content.
