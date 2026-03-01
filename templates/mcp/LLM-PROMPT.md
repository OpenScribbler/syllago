You are helping me create content for syllago, a shared library of AI coding tool configurations. Syllago manages skills, agents, prompts, MCP servers, apps, rules, hooks, and commands that get installed into tools like Claude Code, Cursor, and Gemini CLI.

I'm creating a new MCP server configuration called "{{NAME}}". MCP (Model Context Protocol) servers provide tools that AI assistants can call. The config tells the AI tool how to launch and connect to the server.

Required structure:
  local/mcp/{{NAME}}/
  ├── config.json       # Required: MCP server configuration (valid JSON)
  ├── README.md         # Optional: description and setup instructions
  └── .syllago.yaml      # Auto-generated, don't edit

config.json format (stdio type):
{
  "type": "stdio",
  "command": "node",
  "args": ["path/to/server.js"],
  "env": {
    "API_KEY": "${API_KEY}"
  }
}

config.json format (sse type):
{
  "type": "sse",
  "url": "http://localhost:3000/sse"
}

Conventions:
- Environment variables use ${VAR_NAME} syntax for values that users need to configure
- The README.md should explain what tools the server provides and any setup required

Help me write the config.json and README.md for "{{NAME}}". Ask me what service or API this MCP server should connect to, then create the configuration.
