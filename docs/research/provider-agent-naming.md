# AI Coding Tool Terminology: Naming Research

**Date:** 2026-03-30
**Context:** Research conducted to resolve terminology fragmentation across the Agent Ecosystem community spec, syllago, agentskills.io, and the broader industry. The spec needs a shared vocabulary before defining fields like `supported_agents` or `model_provider`.

---

## The Problem

Three different terms are used for the same concept (Claude Code, Cursor, Copilot, etc.):

- **"Providers"** — syllago's internal term
- **"Supported agent platforms"** — Agent Ecosystem community spec
- **"Harnesses"** / **"clients"** — agentskills.io spec internals

Additionally, syllago's "provider" conflates two distinct concepts: the tool (Claude Code) and the LLM vendor (Anthropic). The spec needs distinct terms for distinct concepts.

---

## Proposed Terminology

| Concept                                  | Proposed Term         | Definition                                                                           | Replaces                                      |
|------------------------------------------|-----------------------|--------------------------------------------------------------------------------------|-----------------------------------------------|
| The product (Claude Code, Cursor, etc.)  | **agent**             | A software product that uses LLMs to help users write, review, or maintain code      | syllago's "provider", spec's "agent platform" |
| The runtime that executes skills         | **harness**           | The non-model software that manages context, tools, file access, and skill loading   | Not previously named                          |
| The LLM vendor (Anthropic, OpenAI, etc.) | **model provider**    | The company or service providing the underlying language model                       | syllago's implicit conflation with "provider" |
| A category/family of agents              | **form factor**       | How the agent is delivered: IDE, CLI, extension, web, autonomous                     | Not previously categorized                    |
| Tools that move content between agents   | **distribution tool** | Software that syncs, converts, or packages content across agents (syllago, rulesync) | Not previously categorized                    |

---

## What Each Tool Calls Itself

| Self-Description Term                  | Tools Using It                                                                                                                                      | Trend                       |
|----------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------|
| **"coding agent"**                     | Claude Code, Codex CLI, Amp, Cline, Roo Code, Kilo Code, Crush, Factory Droid, Zencoder, Junie, OpenCode, OpenHands, Goose, Pi, Mistral Vibe, gptme | Growing (post-2025 default) |
| **"coding assistant"**                 | Copilot, Tabnine, Gemini Code Assist, Augment, Supermaven, CodeGeeX, Windsurf, Amazon Q, Baidu Comate                                               | Receding (pre-2025 default) |
| **"AI code editor"**                   | Cursor, Trae, PearAI, Void, Melty                                                                                                                   | Stable (IDE-specific)       |
| **"agentic IDE"**                      | Windsurf, Kiro, Antigravity, JetBrains Air                                                                                                          | Emerging (IDE + agent)      |
| **"autonomous agent" / "AI engineer"** | Devin, Jules, Manus, Open SWE                                                                                                                       | Niche (fully autonomous)    |
| **"pair programmer"**                  | Copilot (legacy), Aider                                                                                                                             | Legacy                      |

The directional shift is from "assistant" to "agent." Tools launched after 2025 almost universally use "agent." Amp explicitly declared the "assistant" framing dead. Even the "assistant" camp (Copilot, Windsurf) is adding agentic features and shifting terminology.

---

## Why "Agent" Wins

1. **Already dominant** — 16+ tools explicitly self-describe as "agent"; it's the directional term
2. **Matches the ecosystem name** — "Agent Ecosystem" community
3. **Covers all form factors** — IDEs, CLIs, extensions, autonomous systems
4. **How practitioners talk** — people say "coding agents" not "coding tools" or "coding assistants"
5. **Provider-neutral** — describes capability, not vendor
6. **Concise** — `supported_agents` reads better than `supported_agent_platforms`

### Why Not the Alternatives

| Term            | Problem                                                                |
|-----------------|------------------------------------------------------------------------|
| **"provider"**  | Means LLM vendor everywhere else (Anthropic, OpenAI, Google)           |
| **"platform"**  | Overloaded (GitHub is a platform, AWS is a platform)                   |
| **"client"**    | Overloaded (MCP client, HTTP client, customer)                         |
| **"tool"**      | Too generic, also means "a tool the agent can call" (shell, file_read) |
| **"assistant"** | Receding term, implies less autonomy than these tools have             |
| **"harness"**   | Correct for the runtime layer, but too jargony for the product itself  |

---

## Full Landscape: 70+ Agents (98 Tools Total)

### IDE-Based Agents

| Agent         | Form Factor | Harness Type               | Model Provider(s)                      | Skill Support                    | Self-Description                  |
|---------------|-------------|----------------------------|----------------------------------------|----------------------------------|-----------------------------------|
| Cursor        | IDE         | Native (VS Code fork)      | Anthropic, OpenAI, Google, custom      | Full (rules, .cursor/)           | "AI code editor and coding agent" |
| Windsurf      | IDE         | Native (VS Code fork)      | Anthropic, OpenAI, Google, proprietary | Full (rules, .windsurf/)         | "Agentic IDE"                     |
| Kiro          | IDE         | Native (VS Code fork)      | Amazon Bedrock (Anthropic, Amazon)     | Full (agent hooks, specs)        | "Agentic IDE"                     |
| JetBrains Air | IDE         | Native (Fleet codebase)    | Agent-agnostic via ACP                 | Full (task-based, multi-agent)   | "Agentic development environment" |
| Trae          | IDE         | Native (VS Code fork)      | Anthropic, OpenAI, ByteDance (Doubao)  | Limited                          | "The Real AI Engineer"            |
| Antigravity   | IDE         | Native (VS Code fork)      | Google, Anthropic, OpenAI              | Full (AgentKit skills, commands) | "Agent-first agentic platform"    |
| Zed           | IDE         | Native (Rust, not VS Code) | Anthropic, OpenAI, Google, Ollama      | Full (ACP, MCP)                  | "AI editor"                       |
| PearAI        | IDE         | Native (VS Code fork)      | Multi (PearAI Router)                  | Full (via Continue/Aider)        | "AI Code Editor"                  |
| Void          | IDE         | Native (VS Code fork)      | Any (including local)                  | Full (customizable)              | "Open Source Cursor Alternative"  |
| Melty         | IDE         | Native (VS Code fork)      | Various                                | Unknown                          | "AI code editor"                  |
| Aide          | IDE         | Native (VS Code fork)      | OpenAI, Anthropic, open-source         | Full (multi-backend)             | "AI-native IDE" (discontinued)    |

### CLI / Terminal Agents

| Agent            | Form Factor                  | Harness Type                 | Model Provider(s)                                                         | Skill Support                              | Self-Description                           |
|------------------|------------------------------|------------------------------|---------------------------------------------------------------------------|--------------------------------------------|--------------------------------------------|
| Claude Code      | CLI                          | Native (Node.js)             | Anthropic                                                                 | Full (CLAUDE.md, skills, hooks, MCP)       | "Agentic coding tool"                      |
| Codex CLI        | CLI                          | Native (Rust)                | OpenAI                                                                    | Full (CODEX.md, skills, hooks, MCP)        | "Lightweight coding agent"                 |
| Gemini CLI       | CLI                          | Native (Node.js)             | Google                                                                    | Full (GEMINI.md, commands)                 | "AI agent"                                 |
| Cursor CLI       | CLI                          | Native (Cursor backend)      | OpenAI, Anthropic (via Cursor subscription)                               | Full (inherits Cursor rules)               | "Cursor Agent in your terminal"            |
| Amp              | Hybrid (CLI + IDE ext)       | Native                       | Anthropic, OpenAI                                                         | Full (tools, walkthrough skill)            | "Frontier coding agent"                    |
| Aider            | CLI                          | Native (Python)              | Any (BYOK)                                                                | Partial (.aider.conf.yml, conventions)     | "AI pair programming"                      |
| OpenCode         | Hybrid (CLI + Desktop + ext) | Native (Go, Bubble Tea)      | 75+ models (BYOK)                                                         | Full (opencode.json, .opencode/)           | "Open source AI coding agent"              |
| Goose            | Hybrid (CLI + Desktop)       | Native (Rust)                | Any with tool calling (BYOK)                                              | Full (GOOSE.md, MCP)                       | "Open source extensible AI agent"          |
| Crush            | CLI                          | Native (Go)                  | Anthropic, OpenAI, Google, Groq, OpenRouter, Azure, Bedrock, HF, 10+ more | Full (Agent Skills, MCP, LSP, .crush.json) | "Glamorous agentic coding"                 |
| Warp             | CLI / ADE                    | Native (Rust)                | OpenAI, Anthropic, Google                                                 | Partial (WARP.md)                          | "Agentic Development Environment"          |
| Junie CLI        | Hybrid (CLI + IDE plugin)    | Native                       | Any (BYOK)                                                                | Partial (conventions, BYOK)                | "LLM-agnostic coding agent"                |
| Mistral Vibe     | CLI                          | Native (Python)              | Mistral                                                                   | Limited (minimal CLI agent)                | "Minimal CLI coding agent"                 |
| gptme            | CLI                          | Native (Python)              | Anthropic, OpenAI, Google, xAI, DeepSeek, local models                    | Full (lessons, skills, prompt templates)   | "Your agent in your terminal"              |
| Open Interpreter | CLI                          | Native (Python)              | Multi-provider, local models                                              | Partial (via prompting)                    | "Terminal tool for code execution"         |
| Plandex          | CLI                          | Native                       | Multi-provider                                                            | Partial (structured step plans)            | "Plan-first CLI agent"                     |
| DeepAgents CLI   | CLI                          | LangGraph                    | Any (BYOK)                                                                | Partial (memory, planning)                 | "Pre-built coding agent"                   |
| Qwen Code        | Hybrid (CLI + IDE ext)       | Native (based on Gemini CLI) | Alibaba (Qwen), any API                                                   | Full (skills, subagents)                   | "Open-source AI agent"                     |
| Pi               | Hybrid (CLI + Slack + Web)   | Native (TypeScript)          | OpenAI, Anthropic, Google (BYOK)                                          | Partial (AGENTS.md, agent runtime)         | "Interactive coding agent CLI"             |
| oh-my-pi         | CLI                          | Native (TypeScript)          | Multiple                                                                  | Partial (tool harness, subagents)          | "AI coding agent with hash-anchored edits" |
| Herm             | CLI                          | Container-based (Go)         | Multiple                                                                  | Partial (container isolation)              | "Terminal-native agent in containers"      |

### IDE Extension Agents

| Agent                | Form Factor              | Harness Type                         | Model Provider(s)                     | Skill Support                                        | Self-Description                               |
|----------------------|--------------------------|--------------------------------------|---------------------------------------|------------------------------------------------------|------------------------------------------------|
| GitHub Copilot       | Extension                | VS Code / JetBrains / Neovim         | OpenAI, Anthropic, Google             | Full (.github/copilot-instructions, agents, prompts) | "AI coding assistant"                          |
| Cline                | Extension + CLI          | VS Code                              | Any (BYOK)                            | Full (.clinerules, custom instructions)              | "Agentic coding"                               |
| Roo Code             | Extension                | VS Code                              | Any (BYOK)                            | Full (Custom Modes, instructions)                    | "AI coding agent"                              |
| Kilo Code            | Extension                | VS Code + JetBrains + CLI            | 500+ models (BYOK)                    | Full (Orchestrator modes)                            | "Open-Source AI Coding Agent"                  |
| Continue             | Extension + CLI          | VS Code + JetBrains                  | Any (including local)                 | Full (markdown checks, prompts)                      | "Open-source AI code agent"                    |
| Tabnine              | Extension                | VS Code + JetBrains + Eclipse        | Proprietary + Anthropic, OpenAI, Meta | Limited (enterprise context engine)                  | "AI Coding Agents"                             |
| Supermaven           | Extension                | VS Code + JetBrains + Neovim         | OpenAI, Anthropic                     | None (completion-focused)                            | "Fast code completion"                         |
| Augment Code         | Extension + Desktop      | VS Code + JetBrains                  | Multi-model (Anthropic default)       | Full (Context Engine, Jira/Confluence)               | "The Software Agent Company"                   |
| Amazon Q Developer   | Extension + CLI          | VS Code + JetBrains + VS             | Amazon Bedrock                        | Limited (C#/C++ customization, MCP)                  | "AI-powered assistant"                         |
| Gemini Code Assist   | Extension                | VS Code + JetBrains + Android Studio | Google                                | Partial (via Gemini CLI's GEMINI.md)                 | "AI coding assistant"                          |
| JetBrains AI / Junie | Extension + CLI          | JetBrains IDEs                       | Multi-model (BYOK in CLI)             | Partial (IDE inspections + BYOK)                     | "AI coding agent"                              |
| Qodo                 | Extension + GitHub       | VS Code + JetBrains                  | Multi-model                           | Partial (code plans, Jira criteria)                  | "Agentic code integrity platform"              |
| Traycer              | Extension                | VS Code                              | Multi-model                           | Partial (spec-first plans)                           | "AI assistant that plans, implements, reviews" |
| CodeGeeX             | Extension                | VS Code + JetBrains                  | Zhipu AI (GLM-4)                      | Limited                                              | "AI Programming Assistant"                     |
| CodeGPT              | Extension                | IDE                                  | Multiple                              | Partial                                              | "AI code assistant"                            |
| Double.bot           | Extension                | VS Code                              | Unknown                               | Unknown                                              | "Fast VS Code extension"                       |
| Zencoder             | Extension + Web          | VS Code + JetBrains                  | Anthropic (primary), multi-model      | Full (Zen Agents, MCP)                               | "AI Coding Agent Platform"                     |
| Factory Droid        | Hybrid (CLI + multi-ext) | VS Code + JetBrains + Zed            | Any (BYOK)                            | Full (enterprise guardrails)                         | "Enterprise-grade AI coding agent"             |

### Web / Browser-Based Agents

| Agent     | Form Factor      | Harness Type         | Model Provider(s)        | Skill Support               | Self-Description                  |
|-----------|------------------|----------------------|--------------------------|-----------------------------|-----------------------------------|
| Replit    | Web              | Browser IDE          | Proprietary + frontier   | Limited (project AI config) | "Build software faster"           |
| Stagewise | Web (browser)    | Electron-based       | Any (BYOK)               | Unknown                     | "Developer browser with agent"    |
| Frontman  | Web (middleware) | Framework middleware | Anthropic, OpenAI (BYOK) | Partial (framework-aware)   | "Open-source AI agent in browser" |
| Bolt.new  | Web              | Browser              | Multiple                 | None                        | "Rapid prototyping"               |
| Lovable   | Web              | Browser              | Multiple                 | None                        | "AI app builder"                  |
| v0        | Web              | Browser (Vercel)     | Multiple                 | None                        | "AI-powered UI generation"        |

### Autonomous / Async Agents

| Agent          | Form Factor      | Harness Type      | Model Provider(s)       | Skill Support            | Self-Description                   |
|----------------|------------------|-------------------|-------------------------|--------------------------|------------------------------------|
| Devin          | Autonomous       | Cloud sandbox     | Proprietary (Cognition) | Task-based only          | "AI software engineer"             |
| Jules          | Autonomous       | Cloud VM (Google) | Google (Gemini)         | Task-based only          | "Autonomous Coding Agent"          |
| Manus          | Autonomous       | Multi-component   | Multiple                | Task-based only          | "Hands On AI"                      |
| Open SWE       | Autonomous       | LangGraph sandbox | Any (BYOK)              | Full (agent definitions) | "Open-source async coding agent"   |
| OpenHands      | Autonomous + CLI | SDK + GUI         | Any (BYOK)              | Full (agent definitions) | "Platform for Cloud Coding Agents" |
| agenticSeek    | Autonomous       | Local (Python)    | Local models only       | None (self-contained)    | "Fully local Manus alternative"    |
| SWE-agent      | Autonomous       | CLI (research)    | Any                     | Configurable             | "Research-grade autonomous agent"  |
| AutoCodeRover  | Autonomous       | CLI (research)    | Multiple                | None                     | "Autonomous software engineering"  |
| Live-SWE-agent | Autonomous       | CLI (research)    | Anthropic, Google       | None                     | "Live AI Software Agent"           |

### Code Review Specialists

| Agent      | Form Factor        | Harness Type | Model Provider(s) | Skill Support | Self-Description                               |
|------------|--------------------|--------------|-------------------|---------------|------------------------------------------------|
| CodeRabbit | GitHub integration | Web hook     | Multiple          | None          | "AI code review"                               |
| Greptile   | GitHub integration | Web hook     | Multiple          | None          | "AI code review"                               |
| Graphite   | GitHub integration | Web hook     | Multiple          | None          | "Stacked PRs + AI review" (acquired by Cursor) |
| Ellipsis   | GitHub integration | Web hook     | Multiple          | None          | "AI that reviews AND fixes bugs"               |
| Panto AI   | GitHub integration | Web hook     | Multiple          | None          | "Repository-aware code review"                 |
| Korbit     | GitHub integration | Web hook     | Multiple          | None          | "AI mentor for code review"                    |
| BugBot     | GitHub integration | Web hook     | Cursor models     | None          | "Pre-merge safety net" (by Cursor)             |
| CodeAnt AI | GitHub integration | Web hook     | Multiple          | None          | "Security-first code review"                   |

### Agent Orchestrators (New Category)

| Tool                        | Form Factor | Purpose                                                           | Self-Description                         |
|-----------------------------|-------------|-------------------------------------------------------------------|------------------------------------------|
| OpenAI Symphony             | CLI / API   | Project-level orchestration above Codex; isolated autonomous runs | "Turns project work into implementation" |
| Claude Squad                | CLI / TUI   | Manages multiple terminal agents via tmux + git worktrees         | "Manage multiple AI terminal agents"     |
| Composio Agent Orchestrator | CLI         | AI-powered task decomposition + parallel agent dispatch           | "Agentic orchestrator for coding agents" |
| Superset                    | Desktop     | IDE wrapper for running multiple agents in parallel               | "Code editor for the AI agents era"      |
| amux                        | TUI         | Terminal multiplexer for parallel agent execution                 | "Terminal UI for multiple coding agents" |
| AgentPipe                   | CLI / TUI   | Multi-agent conversations in shared "rooms"                       | "Orchestrates multi-agent conversations" |
| Conductor                   | Desktop     | Visual dashboard with git worktree isolation (by Melty Labs)      | "Multi-agent orchestration"              |
| Vibe Kanban                 | CLI + Web   | Kanban board for agent task management                            | "Kanban board for agent tasks"           |
| IttyBitty                   | CLI         | Lightweight Claude Code instance manager via tmux                 | "Manage multiple Claude Code instances"  |

### Chinese / Asian Market Agents

| Agent         | Form Factor     | Harness Type                | Model Provider(s) | Skill Support                | Self-Description                    |
|---------------|-----------------|-----------------------------|-------------------|------------------------------|-------------------------------------|
| Baidu Comate  | Extension + CLI | VS Code + JetBrains + Xcode | Baidu (ERNIE)     | Full (Zulu autonomous agent) | "Intelligent programming workbench" |
| Tongyi Lingma | Extension       | IDE                         | Alibaba (Tongyi)  | Partial (agent-mode tasks)   | "Agent-mode coding assistant"       |

### Agent Infrastructure (Not Agents Themselves)

| Tool      | Form Factor | Purpose                                                                       |
|-----------|-------------|-------------------------------------------------------------------------------|
| Serena    | MCP server  | LSP-based semantic code intelligence for any MCP-compatible agent (22K stars) |
| Morph     | API / MCP   | Acceleration layer: Fast Apply (code merging), WarpGrep (semantic search)     |
| Engram    | MCP server  | Persistent memory system for agents via SQLite + FTS5 (Go)                    |
| Domscribe | MCP server  | Frontend DOM-to-source mapping for browser-aware agents                       |

### Agent-Optimized Terminals (New Category)

| Tool | Form Factor | Purpose                                                            |
|------|-------------|--------------------------------------------------------------------|
| cmux | Desktop     | Ghostty-based macOS terminal built for agent workflows (11K stars) |
| Kaku | Desktop     | Rust-based terminal optimized for AI coding (3.7K stars)           |

### Distribution Tools (Not Agents)

| Tool             | Form Factor   | Purpose                                                                    |
|------------------|---------------|----------------------------------------------------------------------------|
| Syllago          | CLI + TUI     | Content package manager — imports, exports, converts content across agents |
| Rulesync         | CLI (Node.js) | Syncs rules/commands/MCP across 25 agents from a single source directory   |
| Agent Rules Sync | VS Code ext   | Syncs AGENTS.md / CLAUDE.md / .cursor between formats                      |
| AI Rules Sync    | VS Code ext   | Syncs rules between AI IDEs                                                |

### Frameworks and Protocols (Not Agents)

| Tool                  | Category  | Purpose                                                                |
|-----------------------|-----------|------------------------------------------------------------------------|
| LangChain / LangGraph | Framework | Agent orchestration framework (DeepAgents CLI, Open SWE built on this) |
| Charm                 | Framework | Terminal UI frameworks (Bubble Tea, Lip Gloss) + Crush coding agent    |
| CrewAI                | Framework | Multi-agent orchestration framework                                    |
| AutoGen               | Framework | Multi-agent conversation framework                                     |
| MCP                   | Protocol  | Open protocol for connecting agents to tools and data sources          |

---

## Content Type Support Matrix

Syllago manages six content types. This matrix shows which agents support each type natively, excluding the 12 agents syllago already supports and the categories outside scope (infrastructure, terminals, frameworks, distribution, review, Chinese market).

Legend: ✅ = supported, ⚠️ = partial/limited, ❌ = not supported

### IDE Agents

| Agent         | Rules | Skills | Hooks | MCP | Commands | Agents | Notes                                                                                              |
|---------------|-------|--------|-------|-----|----------|--------|----------------------------------------------------------------------------------------------------|
| JetBrains Air | ❌    | ❌    | ❌    | ✅  | ❌       | ✅    | Orchestrator only — delegates to managed agents. MCP via `.air/mcp.json`. ACP for external agents. |
| Trae          | ✅    | ✅    | ❌    | ✅  | ⚠️       | ✅    | `.trae/rules/`, Agent Skills, `.mcp.json`, custom agents with prompts/tools. No hooks.             |
| Antigravity   | ✅    | ✅    | ❌    | ✅  | ✅       | ✅    | `GEMINI.md`/`AGENTS.md`, AgentKit 2.0 (40+ skills), MCP Store, Workflows as `/commands`. No hooks. |
| PearAI        | ❌    | ❌    | ❌    | ⚠️  | ⚠️       | ❌    | VS Code + Continue fork. Inherits some MCP/commands. Minimal own customization.                    |
| Void          | ❌    | ❌    | ❌    | ✅  | ❌       | ❌    | MCP only. Rules requested (GitHub #643) but never built. Project paused.                           |
| Melty         | ❌    | ❌    | ❌    | ❌  | ❌       | ⚠️    | Chat-first, no extensibility. Team pivoted to Conductor.                                           |
| Aide          | ❌    | ❌    | ❌    | ❌  | ❌       | ⚠️    | Discontinued Feb 2025. GitHub repo archived.                                                       |

### CLI Agents

| Agent            | Rules | Skills | Hooks | MCP | Commands | Agents | Notes                                                                                                                   |
|------------------|-------|--------|-------|-----|----------|--------|-------------------------------------------------------------------------------------------------------------------------|
| Cursor CLI       | ✅    | ✅    | ✅    | ✅  | ✅       | ✅    | Full 6/6. `.cursor/rules/`, SKILL.md, hooks since v1.7, `mcp.json`, slash commands, subagents.                          |
| Crush            | ✅    | ✅    | ❌    | ✅  | ❌       | ❌    | `CRUSH.md`/`AGENTS.md`/`CLAUDE.md`, Agent Skills standard, Docker MCP. No hooks/commands/subagents.                     |
| Goose            | ✅    | ❌    | ❌    | ✅  | ⚠️       | ⚠️    | `AGENTS.md` + `goosehints`. MCP-first. Recipes as workflow definitions. ACP server mode.                                |
| Aider            | ✅    | ❌    | ❌    | ⚠️  | ✅       | ❌    | `CONVENTIONS.md`/`AGENTS.md`. Built-in `/` commands. Can be exposed AS an MCP server. No hooks/skills.                  |
| gptme            | ✅    | ✅    | ✅    | ✅  | ✅       | ✅    | Full 6/6. `gptme.toml`, Agent Skills, plugin hooks, dynamic MCP, 20+ commands, agent templates.                         |
| Warp             | ✅    | ❌    | ❌    | ✅  | ✅       | ✅    | `WARP.md`/`AGENTS.md`. Rich MCP GUI. Agent profiles + Oz cloud agents. No hooks (requested).                            |
| Junie CLI        | ✅    | ✅    | ❌    | ✅  | ✅       | ⚠️    | `.junie/AGENTS.md`, `.junie/skills/`, imports from other agents' directories. No hooks (requested).                     |
| Mistral Vibe     | ⚠️    | ✅    | ❌    | ✅  | ✅       | ✅    | Custom prompts in `~/.vibe/prompts/`. Agent Skills spec. TOML agent profiles. No hooks.                                 |
| Open Interpreter | ⚠️    | ❌    | ❌    | ❌  | ❌       | ❌    | System message customization via API. YAML profiles. No declarative content types.                                      |
| Plandex          | ❌    | ❌    | ❌    | ❌  | ❌       | ❌    | JSON model config only. MCP is an open GitHub issue (#241). No content types.                                           |
| DeepAgents CLI   | ✅    | ✅    | ❌    | ✅  | ❌       | ✅    | `AGENTS.md`, `~/.deepagents/skills/`, MCP via langchain-mcp-adapters, subagent spawning.                                |
| Qwen Code        | ✅    | ✅    | ✅    | ✅  | ✅       | ✅    | Full 6/6. Claude Code-like ecosystem: `.qwen/settings.json`, skills, hooks (experimental), MCP, slash commands, agents. |
| Pi               | ✅    | ✅    | ✅    | ❌  | ✅       | ❌    | `AGENTS.md`/`SYSTEM.md`, `/skill:name`, extension lifecycle hooks. No MCP by design (uses CLI tools).                   |
| oh-my-pi         | ✅    | ✅    | ✅    | ✅  | ✅       | ✅    | Full 6/6. Pi fork with batteries-included. Discovers content from 8+ agent formats natively. Full MCP + OAuth.          |
| Herm             | ❌    | ⚠️    | ❌    | ❌  | ❌       | ❌    | `.herm/skills/` only. Container-based execution. Minimal customization.                                                 |

### Extension Agents

| Agent                | Rules | Skills | Hooks | MCP | Commands | Agents | Notes                                                                                                                      |
|----------------------|-------|--------|-------|-----|----------|--------|----------------------------------------------------------------------------------------------------------------------------|
| GitHub Copilot       | ✅    | ❌    | ⚠️    | ✅  | ❌       | ✅     | `.github/copilot-instructions.md`, `AGENTS.md`. MCP via JSON config. Agent YAML profiles. Hooks in JetBrains preview only. |
| Kilo Code            | ✅    | ❌    | ❌    | ✅  | ✅       | ✅     | `AGENTS.md`, `.kilocode/mcp.json` + marketplace. Agent Manager. Orchestrator mode. No skills/hooks.                        |
| Continue             | ✅    | ❌    | ❌    | ✅  | ✅       | ❌     | `.continue/rules/`. MCP auto-imports from Claude/Cursor configs. Custom prompts as slash commands.                         |
| Tabnine              | ✅    | ❌    | ❌    | ✅  | ✅       | ❌     | `.tabnine/guidelines/`, `.tabnine/mcp_servers.json`, `.tabnine/agent/commands/`. No skills/hooks/agents.                   |
| Supermaven           | ❌    | ❌    | ❌    | ❌  | ❌       | ❌     | Completion engine only. No content types. Acquired by Cursor.                                                              |
| Augment Code         | ✅    | ✅    | ✅    | ✅  | ❌       | ✅     | `.augment/rules/`, `.augment/skills/`, `.augment/hooks/`. MCP + JSON import. Agent modes.                                  |
| Amazon Q Developer   | ✅    | ❌    | ✅    | ✅  | ✅       | ✅     | `.amazonq/rules/*.md`. Hooks with matchers (agentSpawn, preToolUse, etc.). MCP per-agent. Custom agent JSON.               |
| Gemini Code Assist   | ✅    | ⚠️    | ✅    | ✅  | ✅       | ❌     | Context files for rules. Gemini CLI hooks. MCP via settings. Skills at API level only, not user-defined.                   |
| JetBrains AI / Junie | ✅    | ✅    | ❌    | ✅  | ✅       | ✅     | `AGENTS.md`, `.junie/skills/`, MCP via `.junie/mcp/mcp.json`. Custom subagents. No hooks.                                  |
| Qodo                 | ✅    | ✅    | ⚠️    | ✅  | ✅       | ✅     | Agent TOML/YAML, `qodo-skills`, webhook hooks, `mcp.json`, CLI commands, agent marketplace.                                |
| Traycer              | ⚠️    | ❌    | ❌    | ⚠️  | ❌       | ⚠️     | Orchestration layer — delegates to Cursor/Claude Code. Thin own content type support.                                      |
| CodeGeeX             | ❌    | ❌    | ❌    | ❌  | ❌       | ❌     | Completion model only. No extensibility framework.                                                                         |
| CodeGPT              | ❌    | ❌    | ❌    | ✅  | ❌       | ⚠️     | MCP via `mcp.json`. Agent sync from platform. No other content types.                                                      |
| Zencoder             | ⚠️    | ❌    | ❌    | ✅  | ❌       | ✅     | MCP Library (100+ servers). Zen Agents Marketplace (20+ agents, open-source JSON).                                         |
| Factory Droid        | ✅    | ⚠️    | ✅    | ✅  | ✅       | ✅     | `AGENTS.md`, `~/.factory/` with mcp/droids/commands. Hooks with global toggle.                                             |

### Web / Browser Agents

| Agent     | Rules | Skills | Hooks | MCP | Commands | Agents | Notes                                                                                                                           |
|-----------|-------|--------|-------|-----|----------|--------|---------------------------------------------------------------------------------------------------------------------------------|
| Replit    | ⚠️    | ❌    | ❌    | ✅  | ⚠️       | ❌    | `replit.md`/`AGENTS.md`. MCP in marketplace. Slash commands for connector selection only.                                       |
| Stagewise | ⚠️    | ❌    | ❌    | ⚠️  | ❌       | ⚠️    | Reads `agents.md`/`claude.md`. Agent Interface for external agents. Not a standard MCP host.                                    |
| Frontman  | ⚠️    | ❌    | ❌    | ✅  | ❌       | ❌    | Reads `agents.md`/`claude.md`. Turns dev server into MCP server. Framework middleware.                                          |
| Bolt.new  | ⚠️    | ❌    | ❌    | ⚠️  | ❌       | ❌    | Reads `claude.md`. MCP in Bolt.diy fork only.                                                                                   |
| Lovable   | ⚠️    | ❌    | ❌    | ✅  | ❌       | ❌    | Custom instructions + `AGENTS.md`. MCP via Personal Connectors (Notion, Linear, etc.).                                          |
| v0        | ✅    | ✅    | ❌    | ✅  | ✅       | ✅    | `AGENTS.md`, Agent Skills via Skills.sh, Vercel MCP server, ContextKit commands, pre-installed agents. Most complete web agent. |

### Autonomous / Async Agents

| Agent          | Rules | Skills | Hooks | MCP | Commands | Agents | Notes                                                                                                                        |
|----------------|-------|--------|-------|-----|----------|--------|------------------------------------------------------------------------------------------------------------------------------|
| Devin          | ⚠️    | ✅     | ⚠️   | ✅  | ✅       | ⚠️     | Playbooks for rules/skills. MCP marketplace. Slash commands. Event-driven triggers via API. Child sessions as sub-agents.    |
| Jules          | ✅    | ❌     | ❌   | ⚠️  | ✅       | ❌     | `AGENTS.md`. MCP via third-party server only. Jules Tools CLI commands. Async cloud VM execution.                            |
| Manus          | ⚠️    | ✅     | ❌   | ❌  | ✅       | ❌     | Skills are first-class (Agent Skills standard). Rules embedded in skills. No MCP/hooks.                                      |
| Open SWE       | ✅    | ❌     | ✅   | ⚠️  | ❌       | ✅     | `AGENTS.md`. Middleware hooks (4 built-in). MCP via LangGraph tools. Sub-agent spawning via `task` tool.                     |
| OpenHands      | ✅    | ✅     | ✅   | ✅  | ✅       | ✅     | Full 6/6. Micro-agents (md + YAML frontmatter), plugin hooks, native MCP + OAuth, slash commands, `.openhands/microagents/`. |
| agenticSeek    | ⚠️    | ❌     | ❌   | ❌  | ❌       | ⚠️     | `config.ini` + prompt templates. Built-in agent specialization not user-definable. Fully local.                              |
| SWE-agent      | ✅    | ❌     | ❌   | ❌  | ❌       | ❌     | YAML config files for tools/prompts. Academic research tool. Not designed for extensibility.                                 |
| AutoCodeRover  | ❌    | ❌     | ❌   | ❌  | ❌       | ❌     | Zero user-facing configuration. Fully autonomous pipeline. Academic tool.                                                    |
| Live-SWE-agent | ⚠️    | ⚠️     | ❌   | ❌  | ❌       | ❌     | YAML config. Self-creates tools at runtime (emergent skills). Minimal by design.                                             |

### Agent Orchestrators

| Tool                        | Rules | Skills | Hooks | MCP | Commands | Agents | Notes                                                                                       |
|-----------------------------|-------|--------|-------|-----|----------|--------|---------------------------------------------------------------------------------------------|
| OpenAI Symphony             | ✅    | ❌     | ❌   | ❌  | ❌       | ⚠️    | `WORKFLOW.md` (YAML frontmatter + markdown). Codex agent dispatch. Linear integration only. |
| Claude Squad                | ❌    | ❌     | ❌   | ❌  | ❌       | ⚠️    | Pure session manager. Delegates all content to managed agents. Profiles in config.json.     |
| Composio Agent Orchestrator | ✅    | ❌     | ✅   | ❌  | ❌       | ✅    | `agent-orchestrator.yaml`. Reaction hooks for CI/review. 8 swappable plugin slots.          |
| Superset                    | ⚠️    | ❌     | ❌   | ✅  | ❌       | ✅    | `.superset/config.json`. MCP via `.mcp.json`. Agent-agnostic wrapper.                       |
| amux                        | ⚠️    | ❌     | ❌   | ⚠️  | ❌       | ✅    | Multiple variants. Config profiles. Some variants have MCP. All manage agent instances.     |
| AgentPipe                   | ⚠️    | ❌     | ❌   | ❌  | ❌       | ✅    | YAML config for multi-agent rooms. Round-robin/reactive/free-form modes.                    |
| IttyBitty                   | ❌    | ❌     | ❌   | ❌  | ✅       | ✅    | `ib new-agent`, `ib list`, `ib nuke`. Manager/Worker hierarchy. Claude Code orchestrator.   |

---

## Content Type Adoption Summary

Across all agents researched (excluding syllago's 12 existing + skipped categories):

| Content Type | ✅ Supported | ⚠️ Partial | ❌ None | Adoption Rate |
|--------------|--------------|-------------|---------|---------------|
| **Rules**    | 28           | 14          | 12      | 78%           |
| **MCP**      | 30           | 8           | 16      | 70%           |
| **Agents**   | 22           | 12          | 20      | 63%           |
| **Commands** | 21           | 3           | 30      | 44%           |
| **Skills**   | 16           | 4           | 34      | 37%           |
| **Hooks**    | 11           | 3           | 40      | 26%           |

**Key findings:**
- **Rules and MCP are near-universal** — the clear baseline content types every agent supports
- **Skills adoption is growing fast** — the Agent Skills standard (`SKILL.md` + frontmatter) is gaining traction across Cursor CLI, Crush, Junie, gptme, Mistral Vibe, Trae, Antigravity, v0, Manus, and OpenHands
- **Hooks are the rarest** — only 11 agents have lifecycle hooks, making this syllago's biggest conversion opportunity and the content type most in need of spec standardization
- **Full 6/6 support is rare** — only Cursor CLI, gptme, Qwen Code, oh-my-pi, and OpenHands support all six content types

---

## Skill Support Tiers

| Tier           | Description                                                | Count | Examples                                                                                                                                                                                                                                      |
|----------------|------------------------------------------------------------|-------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Full**       | Structured skills, rules, hooks, MCP, custom instructions  | ~25   | Claude Code, Codex CLI, Cursor, Cursor CLI, Copilot, Gemini CLI, Windsurf, Crush, gptme, Cline, Roo Code, Kilo Code, OpenCode, Amp, Goose, Kiro, Zed, Continue, Antigravity, Augment, Zencoder, Factory Droid, OpenHands, Open SWE, Qwen Code |
| **Partial**    | Rules/config files, limited customization surfaces         | ~18   | Aider, Pi, Mistral Vibe, Open Interpreter, Plandex, oh-my-pi, Herm, Tabnine, Amazon Q, Trae, Qodo, Traycer, Warp, Junie, Replit, CodeGeeX, CodeGPT, Gemini Code Assist, JetBrains AI, Baidu Comate, Tongyi Lingma                             |
| **Task-based** | No persistent customization; accepts per-task instructions | ~6    | Devin, Jules, Manus, SWE-agent, agenticSeek, JetBrains Air                                                                                                                                                                                    |
| **None**       | Completion-only or no customization surface                | ~12   | Supermaven, Bolt.new, Lovable, v0, AutoCodeRover, Live-SWE-agent, CodeRabbit, Greptile, Graphite, Ellipsis, BugBot, CodeAnt AI                                                                                                                |

The **Full** tier (~25 agents) is the primary audience for the skill frontmatter spec. The **Partial** tier may adopt parts of it. **Task-based** and **None** tiers are out of scope for persistent skill content.

---

## Rulesync's Supported Agents (25)

For reference, rulesync (dyoshikawa) explicitly supports these 25 targets:

1. AGENTS.md (agentsmd)
2. AgentSkills (agentsskills)
3. Claude Code (claudecode)
4. Codex CLI (codexcli)
5. Gemini CLI (geminicli)
6. Goose (goose)
7. GitHub Copilot (copilot)
8. GitHub Copilot CLI (copilotcli)
9. Cursor (cursor)
10. DeepAgents CLI (deepagents)
11. Factory Droid (factorydroid)
12. OpenCode (opencode)
13. Cline (cline)
14. Kilo Code (kilo)
15. Roo Code (roo)
16. Rovo Dev / Atlassian (rovodev)
17. Qwen Code (qwencode)
18. Kiro (kiro)
19. Google Antigravity (antigravity)
20. JetBrains Junie (junie)
21. Augment Code (augmentcode)
22. Windsurf (windsurf)
23. Warp (warp)
24. Replit (replit)
25. Zed (zed)

---

## Syllago's Currently Supported Agents (12)

1. Claude Code (claude-code)
2. Gemini CLI (gemini-cli)
3. Cursor (cursor)
4. Windsurf (windsurf)
5. Codex (codex)
6. Copilot CLI (copilot-cli)
7. Zed (zed)
8. Cline (cline)
9. Roo Code (roo-code)
10. OpenCode (opencode)
11. Kiro (kiro)
12. Amp (amp)

---

## Terminology Validation

### "Agent" for the product

Works for all 70+ tools in the main tables. Even edge cases fit:
- Supermaven (completion-only) is a minimal agent
- CodeRabbit/Greptile (review-only) are specialized agents
- Devin/Jules (autonomous) are autonomous agents

The term is broad enough to include the full spectrum while being specific enough to exclude frameworks (LangChain), protocols (MCP), distribution tools (syllago, rulesync), and infrastructure (Serena, Morph).

### "Harness" for the runtime

Works as a technical term within specs. Each agent has a harness:
- Cursor's harness is a VS Code fork runtime
- Claude Code's harness is a Node.js CLI runtime
- Devin's harness is a cloud sandbox runtime

The harness determines what content formats the agent can consume and what capabilities it supports (hooks, MCP, tools, etc.). agentskills.io already uses this term internally.

### "Model provider" for the LLM vendor

Cleanly separates from "agent":
- Anthropic is a model provider; Claude Code is an agent
- OpenAI is a model provider; Codex CLI is an agent
- Amp is an agent that uses Anthropic as its model provider

This eliminates syllago's current ambiguity where "provider" means both the tool and the LLM vendor.

### "Form factor" for delivery model

Six values cover the landscape:
- **IDE** — standalone editor (Cursor, Windsurf, Kiro, Zed, JetBrains Air)
- **CLI** — terminal-based (Claude Code, Gemini CLI, Aider, Crush, gptme)
- **Extension** — plugin for a host IDE (Copilot, Cline, Roo Code)
- **Web** — browser-based (Replit, Bolt.new, Stagewise, Frontman)
- **Autonomous** — async/cloud-based (Devin, Jules, Open SWE)
- **Hybrid** — multiple surfaces (Amp: CLI + extension; OpenCode: CLI + desktop + extension)

---

## Spec Impact

### Field naming changes

| Current (Agent Ecosystem spec) | Proposed                                 |
|--------------------------------|------------------------------------------|
| `supported_agent_platforms`    | `supported_agents`                       |
| (no field)                     | `model_providers` (if needed)            |
| (no concept)                   | `form_factor` as optional agent metadata |

### Glossary section for the spec

The spec should include a terminology glossary defining these terms so the entire community speaks the same language. Proposed definitions:

> **Agent**: A software product that uses large language models to help users write, review, or maintain code. Examples: Claude Code, Cursor, GitHub Copilot.
>
> **Harness**: The non-model runtime software within an agent that manages context windows, tool execution, file access, permission systems, and skill/rule loading. The harness determines what content formats an agent can consume and what capabilities it supports.
>
> **Model provider**: A company or service providing the underlying language model that powers an agent. Examples: Anthropic (Claude), OpenAI (GPT), Google (Gemini). One agent may support multiple model providers.
>
> **Form factor**: How an agent is delivered to users. Values: IDE (standalone editor), CLI (terminal), extension (plugin for a host IDE), web (browser-based), autonomous (async/cloud-based), hybrid (multiple surfaces).
>
> **Distribution tool**: Software that syncs, converts, or packages content across multiple agents. Examples: syllago, rulesync. Distribution tools are not agents — they do not use LLMs to write code.

---

## Sources

### Spec and Community Sources
- [agentskills.io specification](https://agentskills.io/specification) — uses "agent," "client," "harness," "agent product"
- [Agent Ecosystem community spec](docs/spec/skills/frontmatter_spec_proposal.md) — uses "supported agent platforms"
- [Rulesync GitHub](https://github.com/dyoshikawa/rulesync) — 25 supported targets

### Tool Documentation
- [Claude Code docs](https://code.claude.com/docs/en/skills) — "agentic coding tool"
- [Cursor docs](https://cursor.com/docs/rules) — "AI editor and coding agent"
- [Cursor CLI](https://cursor.com/cli) — "Cursor Agent in your terminal" (launched Jan 2026)
- [GitHub Copilot docs](https://docs.github.com/copilot) — "AI coding assistant"
- [Gemini CLI docs](https://geminicli.com/) — "AI agent"
- [Windsurf docs](https://windsurf.com/) — "agentic IDE"
- [Codex CLI GitHub](https://github.com/openai/codex) — "lightweight coding agent"
- [Kiro docs](https://kiro.dev/) — "agentic AI development"
- [JetBrains Air](https://air.dev) — "agentic development environment" (launched Mar 2026)
- [Amp docs](https://ampcode.com/) — "frontier coding agent"
- [OpenCode docs](https://opencode.ai/) — "open source AI coding agent"
- [Aider docs](https://aider.chat/) — "AI pair programming"
- [Goose GitHub](https://github.com/block/goose) — "open source extensible AI agent"
- [Pi monorepo](https://github.com/badlogic/pi-mono) — "interactive coding agent CLI"
- [Crush](https://github.com/charmbracelet/crush) — "glamorous agentic coding" (by Charm)
- [gptme](https://gptme.org/) — "your agent in your terminal"
- [Mistral Vibe](https://github.com/mistralai/mistral-vibe) — "minimal CLI coding agent"
- [Open Interpreter](https://github.com/OpenInterpreter/open-interpreter) — "terminal tool for code execution" (63K stars)
- [Plandex](https://plandex.ai/) — "plan-first CLI agent" (15K stars)

### New Entrants (Jan-Mar 2026)
- [Open SWE](https://github.com/langchain-ai/open-swe) — LangChain's async coding agent (launched Mar 2026)
- [OpenAI Symphony](https://github.com/openai/symphony) — project-level agent orchestration (14K stars)
- [Serena](https://github.com/oraios/serena) — LSP-based semantic code intelligence MCP server (22K stars)
- [agenticSeek](https://github.com/Fosowl/agenticSeek) — local-only autonomous agent (25K stars)
- [Claude Squad](https://github.com/smtg-ai/claude-squad) — multi-agent terminal manager
- [Composio Agent Orchestrator](https://github.com/ComposioHQ/agent-orchestrator) — parallel agent dispatch (5.6K stars)
- [Stagewise](https://stagewise.io/) — developer browser with built-in agent (YC S25)
- [Frontman](https://frontman.sh/) — open-source AI agent as framework middleware
- [Graphite](https://graphite.dev/) — stacked PRs + AI review (acquired by Cursor Dec 2025)
- [Baidu Comate](https://comate.baidu.com/) — Baidu's coding assistant + Zulu autonomous agent
- [cmux](https://github.com/manaflow-ai/cmux) — agent-optimized terminal (11K stars)

### Industry Analysis
- [Faros — Best AI Coding Agents 2026](https://www.faros.ai/blog/best-ai-coding-agents-2026)
- [Qodo — Top AI Coding Assistant Tools 2026](https://www.qodo.ai/blog/best-ai-coding-assistant-tools/)
- [Tembo — CLI Tools Comparison](https://www.tembo.io/blog/coding-cli-tools-comparison)
- [bradAGI/awesome-cli-coding-agents](https://github.com/bradAGI/awesome-cli-coding-agents)
- [sorrycc/awesome-code-agents](https://github.com/sorrycc/awesome-code-agents)


