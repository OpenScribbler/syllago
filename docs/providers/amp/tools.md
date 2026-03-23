<!-- provider-audit-meta
provider: amp
provider_version: "unknown (auth-gated)"
report_format: 1
researched: 2026-03-23
researcher: claude-opus-4.6
changelog_checked: https://ampcode.com/blog
-->

# Amp — Built-in Tools

Amp is a CLI-based AI coding agent built by **Sourcegraph** (not Anthropic). It uses Claude and other models (GPT-5 variants, Gemini) in a multi-model routing architecture.

## Core Tools

| Tool | Purpose | Cross-provider equivalent |
|------|---------|--------------------------|
| Read | Read file contents | Claude Code: Read, Cursor: file read |
| Edit File | Modify files with old_str/new_str replacements | Claude Code: Edit |
| Create File | Generate new files | Claude Code: Write |
| Grep | Search file contents with regex | Claude Code: Grep |
| Glob | Pattern-based file matching | Claude Code: Glob |
| Finder | Locate files in repositories | Claude Code: Glob (similar) |
| Bash | Execute terminal commands | Claude Code: Bash |
| Format File | Code formatting | No direct equivalent |
| Undo Edit | Revert previous edits | No direct equivalent |
| Web Search | Internet search | Claude Code: WebSearch |
| Read Web Page | Fetch and process web content | Claude Code: WebFetch |
| Todo Write | Task management | Claude Code: TodoWrite |

[Inferred] Tool names and descriptions from public ampcode.com content and community sources.

## Specialized Tools

| Tool | Purpose | Description |
|------|---------|-------------|
| Oracle | Architectural review | Second-opinion model for complex reasoning [Community] |
| Librarian | External repo exploration | Searches GitHub/Bitbucket repositories [Community] |
| Code Review | Automated code review | Checks for bugs, security, performance; customizable checks in `.agents/checks/` [Community] |
| Diagnostics | LSP-based code analysis | Language Server Protocol integration [Community] |
| Mermaid | Diagram generation | Creates architectural diagrams [Community] |
| Painter | Image generation | Uses Gemini 3 Pro for image work [Community] |
| Look At | Media/PDF analysis | Analyzes PDFs and images [Community] |
| Read Thread | Cross-thread context | Load data from other Amp threads [Community] |
| Course Correct | Parallel correction agent | Background agent for course corrections [Community] |

## Agent Modes

| Mode | Purpose | Model |
|------|---------|-------|
| Smart | Standard agentic development | Leading models (Claude Opus 4.6, GPT-5 variants) |
| Deep | Extended reasoning | GPT-5.3-Codex (specialized deep-mode) |
| Rush | Faster, cheaper operations | Lighter-weight models |

[Community] From public ampcode.com content.

## Unique Capabilities

- Multi-model routing (uses different models for different tasks) [Community]
- Parallel subagent execution [Community]
- Sourcegraph code search integration [Inferred]
- Thread-based conversation architecture with cross-thread references [Community]
- Custom checks system (`.agents/checks/` with YAML frontmatter) [Community]

## Documentation Limitations

Most Amp documentation is behind authentication at ampcode.com. The findings above are from publicly accessible pages and community sources. Tool parameter details and complete specifications require authenticated access.
