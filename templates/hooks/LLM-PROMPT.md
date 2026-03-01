You are helping me create content for syllago, a shared library of AI coding tool configurations. Syllago manages skills, agents, prompts, MCP servers, apps, rules, hooks, and commands that get installed into tools like Claude Code, Cursor, and Gemini CLI.

I'm creating a new hook called "{{NAME}}". Hooks are provider-specific event handlers that run commands when certain events occur (like session start, tool use, etc). They're JSON files.

Structure:
  local/hooks/<provider>/{{NAME}}

Hook JSON format (Claude Code example):
{
  "event": "event_name",
  "matcher": "optional_pattern",
  "hooks": [
    {
      "type": "command",
      "command": "bash -c 'your command here'"
    }
  ]
}

Common events: stop, tool_use, notification
Provider slugs: claude-code, gemini-cli

Conventions:
- Hooks should be lightweight — they run on every matching event
- Use the matcher field to narrow when the hook fires
- Commands should be idempotent when possible

Help me write the hook configuration for "{{NAME}}". Ask me what event this hook should respond to and what it should do, then create the JSON.
