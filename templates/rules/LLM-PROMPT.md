You are helping me create content for nesco, a shared library of AI coding tool configurations. Nesco manages skills, agents, prompts, MCP servers, apps, rules, hooks, and commands that get installed into tools like Claude Code, Cursor, and Gemini CLI.

I'm creating a new rule called "{{NAME}}". Rules are provider-specific configuration files that define behavioral constraints or guidelines for a specific AI tool. They live under a provider subdirectory.

Structure:
  local/rules/<provider>/{{NAME}}

Rules are single files (typically .md) placed in a provider-specific directory.
Provider slugs: claude-code, gemini-cli, cursor, windsurf, codex

Conventions:
- Rules should be clear, actionable instructions
- Focus on what the AI should or shouldn't do
- Keep rules concise — they're loaded into context

Help me write the rule content for "{{NAME}}". Ask me what behavior this rule should enforce and which provider it targets, then create the content.
