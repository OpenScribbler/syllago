You are helping me create content for nesco, a shared library of AI coding tool configurations. Nesco manages skills, agents, prompts, MCP servers, apps, rules, hooks, and commands that get installed into tools like Claude Code, Cursor, and Gemini CLI.

I'm creating a new app called "{{NAME}}". An app is a complete installable package with its own install.sh script. Apps can include MCP servers, hooks, configuration files, and any other components. They're the most flexible content type.

Required structure:
  local/apps/{{NAME}}/
  ├── README.md         # Required: frontmatter (name, description, providers) + documentation
  ├── install.sh        # Required: bash script for install/uninstall
  └── .nesco.yaml      # Auto-generated, don't edit

README.md format:
---
name: App Name
description: One-line description of what this app does
providers: [claude-code, gemini-cli]
---

# App Name

Documentation for the app.

install.sh conventions:
- Must handle --uninstall flag for removal
- Use set -euo pipefail
- Reference SCRIPT_DIR for relative paths
- Print clear status messages

Supported provider slugs: claude-code, gemini-cli, cursor, windsurf, codex

Help me write the README.md and install.sh for "{{NAME}}". Ask me what this app should do, then create the content.
