# Installing Benchmark Skills via Syllago

Step-by-step guide for using syllago to install the 17 agentskillimplementation.com benchmark skills to all supported AI coding agents.

## Prerequisites

- syllago installed: `syllago --version` shows a version
- At least one target agent installed (Claude Code, Gemini CLI, Cursor, Amp, Windsurf, Roo Code, OpenCode, or Codex)
- For BYOK agents (Roo Code, OpenCode): a Google AI Studio API key from [aistudio.google.com](https://aistudio.google.com/)

## Step 1: Add the Benchmark Registry

Register the benchmark repo as a syllago content source:

```bash
syllago registry add agent-ecosystem/agent-skill-implementation
```

Verify it was added:

```bash
syllago registry list
```

## Step 2: Import Benchmark Skills to Your Library

Import all 17 benchmark skills to your local syllago library:

```bash
syllago add skills --all --from agent-skill-implementation
```

Verify the import:

```bash
syllago list --type skills
```

You should see 17 skills, including `probe-loading`, `probe-linked-resources`, and others.

## Step 3: Install to All Agents

Install the skills to every detected AI coding agent in one command:

```bash
syllago install --type skills --to-all
```

Syllago will:
1. Detect which agents are installed on your system
2. Skip agents that don't support skills (Cline) or use project scope (Kiro)
3. Install to each detected agent, reporting success/skip/failure per agent

## Step 4: Verify Per-Agent Installation

Check that skills appear in each agent's skill directory:

| Agent | Verify |
|-------|--------|
| Claude Code | `ls ~/.claude/skills/` |
| Gemini CLI | `ls ~/.gemini/skills/` |
| Cursor | `ls ~/.cursor/skills/` |
| Amp | `ls ~/.config/agents/skills/` |
| Windsurf | `ls ~/.codeium/windsurf/skills/` |
| Roo Code | `ls ~/.roo/skills/` |
| OpenCode | `ls ~/.config/opencode/skills/` |
| Codex | `ls ~/.agents/skills/` |

## Step 5: Canary Check

Open a fresh session in each agent (no existing context) and ask:

> Do you know CARDINAL-ZEBRA-7742?

A "yes" response confirms the agent loaded the skill from its skills directory.

## Step 6: Run the 28 Behavioral Checks

With skills installed, run the empirical benchmark. See the [benchmark plan](./benchmark-plan.md) for the full check matrix.

**Model selection:** Most checks (22 of 28) test platform behavior — any model works. The 6 model-sensitive checks (`cross-skill-invocation`, `invocation-depth-limit`, `invocation-language-sensitivity`, `circular-invocation-handling`, `informal-dependency-resolution`, `missing-dependency-behavior`) may differ between models. Run cheapest-available first, then re-run with a capable model for the delta.

## Agent Coverage Notes

**Excluded from syllago install:**
- **Cline**: syllago's Cline provider does not support the Skills content type
- **Kiro**: skills are project-scoped (`.kiro/steering/`), not user-scoped
- **Junie CLI**: no syllago provider
- **GitHub Copilot (VS Code extension)**: syllago supports `copilot-cli` (the CLI), not the VS Code extension

These agents require manual skill installation.

## Troubleshooting

**Skills not detected after install:**
- Verify the agent reads from the skills directory at startup (not session-wide context)
- Some agents require a session restart to pick up new skills

**"no providers detected" from --to-all:**
- Run `syllago install --to <slug>` for a specific agent to confirm the path configuration
- Use `syllago config paths --provider <slug> --path /your/path` if installed at a non-default location
