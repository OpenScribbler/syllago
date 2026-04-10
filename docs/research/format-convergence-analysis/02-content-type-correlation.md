# Section 2: Content Type Correlation Analysis

**Source data:** `docs/research/provider-agent-naming.md` — Content Type Support Matrix  
**Population:** 59 agents across 6 categories (IDE, CLI, Extension, Web, Autonomous, Orchestrators)  
**Excluded:** Syllago's 12 already-supported agents, infrastructure tools, distribution tools, code review bots, frameworks/protocols  
**Scoring:** ✅ = 2, ⚠️ = 1, ❌ = 0. "Adoption" includes both full and partial (score ≥ 1) unless otherwise noted.

---

## 2.1 Overall Adoption Rates

| Content Type | ✅ Full | ⚠️ Partial | Total (any) | Adoption Rate |
|--------------|---------|------------|-------------|---------------|
| **Rules**    | 30      | 16         | 46          | 78%           |
| **MCP**      | 33      | 8          | 41          | 69%           |
| **Agents**   | 26      | 11         | 37          | 63%           |
| **Commands** | 24      | 4          | 28          | 47%           |
| **Skills**   | 18      | 4          | 22          | 37%           |
| **Hooks**    | 12      | 3          | 15          | 25%           |

The ordering is consistent with the source document's summary. Rules and MCP are near-universal; Hooks are rare at one-quarter adoption. The gap between MCP (69%) and Agents (63%) is narrow. The gap between Commands (47%) and Skills (37%) is also narrow, suggesting they are often bundled together.

Four agents support no content types at all: Plandex, Supermaven, CodeGeeX, and AutoCodeRover. These are either completion-only tools or fully autonomous pipelines with no user-facing extensibility. They are excluded from tier analysis below.

---

## 2.2 Co-occurrence Matrix

This matrix shows conditional adoption: "of agents that support type A, what percentage also support type B?"

| Given ↓ also has → | Rules | Skills | Hooks | MCP  | Commands | Agents |
|--------------------|-------|--------|-------|------|----------|--------|
| **Rules** (n=46)   | —     | 46%    | 33%   | 80%  | 57%      | 67%    |
| **Skills** (n=22)  | 95%   | —      | 50%   | 82%  | 77%      | 73%    |
| **Hooks** (n=15)   | 100%  | 73%    | —     | 87%  | 73%      | 87%    |
| **MCP** (n=41)     | 90%   | 44%    | 32%   | —    | 61%      | 71%    |
| **Commands** (n=28)| 93%   | 61%    | 39%   | 89%  | —        | 68%    |
| **Agents** (n=37)  | 84%   | 43%    | 35%   | 78%  | 51%      | —      |

Raw co-occurrence counts (out of 59 total agents):

| | Rules | Skills | Hooks | MCP | Commands | Agents |
|---|---|---|---|---|---|---|
| **Rules** | — | 21 | 15 | 37 | 26 | 31 |
| **Skills** | 21 | — | 11 | 18 | 17 | 16 |
| **Hooks** | 15 | 11 | — | 13 | 11 | 13 |
| **MCP** | 37 | 18 | 13 | — | 25 | 29 |
| **Commands** | 26 | 17 | 11 | 25 | — | 19 |
| **Agents** | 31 | 16 | 13 | 29 | 19 | — |

**Key observations from the matrix:**

- **Hooks is the strongest predictor of everything else.** Every agent with Hooks also has Rules (100%), 87% have MCP, 87% have Agents, 73% have Commands, and 73% have Skills. Hooks appear only in the most feature-complete agents. Supporting Hooks is a strong signal of full ecosystem buildout.

- **Skills is a near-certain predictor of Rules** (95%). Only one agent (Herm) supports Skills without Rules — and Herm's "skills" are partial (⚠️). In practice, Skills without Rules does not appear in the wild.

- **Rules is a weak predictor of Skills** (46%). Having Rules is necessary but far from sufficient for Skills. This reinforces Rules as a universal baseline that nearly everyone adds first.

- **MCP + Commands are tightly coupled.** Of agents with Commands, 89% also have MCP. Of agents with MCP, 61% have Commands. The pairing appears frequently together in the 3–4 content type range.

- **Agents (sub-agent spawning) is the most evenly distributed middle type.** It appears with 67% of Rules agents, 73% of Skills agents, and 87% of Hooks agents. It is common across tiers rather than gated to the highest.

---

## 2.3 Score Distribution

Content type scores (count of content types supported at any level) distribute as follows:

| Score | Count | % of 59 | Interpretation |
|-------|-------|---------|----------------|
| 0     | 4     | 7%      | No extensibility |
| 1     | 7     | 12%     | Minimal footprint |
| 2     | 11    | 19%     | Baseline pair |
| 3     | 13    | 22%     | Common middle range |
| 4     | 7     | 12%     | Advanced |
| 5     | 9     | 15%     | Near-complete |
| 6     | 8     | 14%     | Full ecosystem |

The distribution is roughly bimodal: there is a cluster at scores 2–3 (41% of agents) and another cluster at scores 5–6 (29% of agents), with a trough at score 4 (12%). This suggests agents tend to either stop at a minimal footprint or build out fairly completely — there is no strong middle tier.

---

## 2.4 Natural Adoption Tiers

The tier progression is not perfectly linear, but there is a clear dominant path. Starting from the 55 non-zero agents:

**Rules and MCP form the base pair.** Among all 55 non-zero agents:
- 84% have Rules
- 75% have MCP
- 67% have both Rules + MCP (37 agents)

These two types appear together far more than either appears alone. Rules-only (no MCP): 9 agents. MCP-only (no Rules): 4 agents. Both together: 37 agents. Rules and MCP are close to a mutual baseline — the pairing is stable enough to treat as a single Tier 0 entry point.

**From Rules + MCP, the next additions are Agents and Commands.** Among the 37 Rules+MCP agents:
- 73% add Agents
- 65% add Commands
- 49% add Skills
- 35% add Hooks

Agents and Commands are added at similar rates from the Rules+MCP base, and they tend to co-occur: of the 27 Rules+MCP+Agents agents, 67% also have Commands.

**From Rules + MCP + Agents + Commands, Skills and Hooks are added.** Among the 18 agents with all four of the above:
- 78% also have Skills
- 50% also have Hooks

This establishes a clear ordering. Skills and Hooks are late additions that appear primarily in agents that have already built out the rest.

---

## 2.5 Tier Definitions

Based on the data, four tiers emerge naturally:

### Tier 0 — Baseline (Rules + MCP)

Every agent with any meaningful content extensibility supports these two types. Among non-zero agents, 84% have Rules and 75% have MCP; the combination appears in 67% of all non-zero agents.

**Characteristic agents:** Continue, Tabnine, Frontman, Lovable, Replit, SWE-agent, Open Interpreter

**What this means for syllago:** Rules+MCP conversion is not optional. Any agent syllago adds must handle at least these two types.

### Tier 1 — Common (Rules + MCP + Agents + Commands)

Adding sub-agent spawning and slash/CLI commands is the most common growth path from the baseline. 73% of Rules+MCP agents add Agents; 65% add Commands; 67% of those with Agents also have Commands.

**Score 3–4 agents in this tier:** Crush (lacks Commands+Agents), Aider (lacks Skills+Hooks+Agents), Warp, Kilo Code, GitHub Copilot, Continue, Tabnine, DeepAgents CLI, v0, Antigravity, JetBrains AI, Mistral Vibe

**What this means for syllago:** Agents and Commands support is needed for the majority of agents worth converting to.

### Tier 2 — Advanced (adds Skills)

Skills adoption is at 37% overall but climbs to 78% among agents that already have all four Tier 1 types. Skills appear to be added after the foundational four are established, not alongside them.

**Score 5 agents in this tier:** Trae, Antigravity, Junie CLI, Mistral Vibe, Augment Code, Amazon Q Developer, Gemini Code Assist, JetBrains AI, v0

Note: Several of these are score-5 agents missing only Hooks, which is consistent with Hooks as the final addition.

### Tier 3 — Full Ecosystem (adds Hooks)

Hooks are the rarest content type at 25% overall but 50% among Tier 2+ agents. Every agent with Hooks also has Rules (100%), and 73% also have Skills. Hooks appear only after the full content stack is otherwise in place.

**Score 6 agents:** Cursor CLI, gptme, Qwen Code, oh-my-pi, Qodo, Factory Droid, Devin, OpenHands

The strictly all-✅ (no partials) subset is: Cursor CLI, gptme, Qwen Code, oh-my-pi, OpenHands (5 agents).

---

## 2.6 Full 6/6 Agents

Eight agents support all six content types at any level (full or partial). Five support all six at full (✅) level.

| Agent | Rules | Skills | Hooks | MCP | Commands | Agents | All-✅ | Form Factor | Open Source | Notes |
|-------|-------|--------|-------|-----|----------|--------|--------|-------------|-------------|-------|
| Cursor CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Yes | No | Launched Jan 2026 |
| gptme | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Yes | Yes | Python, research-origin |
| Qwen Code | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Yes | Yes | Claude Code fork (Alibaba) |
| oh-my-pi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Yes | Yes | Pi fork, reads 8+ formats |
| OpenHands | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Yes | Yes | Autonomous, micro-agents |
| Qodo | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ | No | No | Partial Hooks (webhooks) |
| Factory Droid | ✅ | ⚠️ | ✅ | ✅ | ✅ | ✅ | No | No | Partial Skills |
| Devin | ⚠️ | ✅ | ⚠️ | ✅ | ✅ | ⚠️ | No | No | Partial Rules/Hooks/Agents |

**Patterns among 6/6 agents:**

- **CLI form factor is dominant.** Four of the five all-✅ agents are CLIs (Cursor CLI, gptme, Qwen Code, oh-my-pi). OpenHands is autonomous. No IDE or extension agent achieves all-✅.

- **Open source is the norm.** Four of five all-✅ agents are open source (gptme, Qwen Code, oh-my-pi, OpenHands). Only Cursor CLI is proprietary. This aligns with the observation that open-source agents are more willing to implement the full content stack rather than selectively exposing features.

- **Derivative agents punch above their weight.** Qwen Code is explicitly described as a Claude Code-like ecosystem (a fork), and oh-my-pi is a Pi fork. Both inherit architecture decisions that support full extensibility. Fork-based agents start ahead of scratch-built agents on content type coverage.

- **Age is not the determining factor.** Cursor CLI launched in January 2026 (very new) and is already all-✅. gptme is research-origin but fully featured. The determining factor appears to be architectural intent, not maturity.

---

## 2.7 Surprising Gaps

### Hooks without Skills (4 agents)

The co-occurrence matrix shows 73% of Hooks agents also have Skills — but 4 agents have Hooks without Skills:

| Agent | Form Factor | Why |
|-------|-------------|-----|
| GitHub Copilot | Extension | Hooks in JetBrains preview only (⚠️); Skills would require open ecosystem |
| Amazon Q Developer | Extension | Enterprise focus; strong lifecycle hooks but no user-defined skills |
| Open SWE | Autonomous | Middleware hooks for CI pipelines; not a user-skills platform |
| Composio Agent Orchestrator | Orchestrator | Reaction hooks for CI/review triggers; dispatches to other agents |

The pattern: Hooks-without-Skills appears when hooks serve infrastructure/automation purposes (CI triggers, event reactions) rather than user customization. These are not "coding session hooks" in the Claude Code sense; they are CI/CD pipeline hooks. This is a meaningful distinction for syllago's conversion logic — hooks from these agents likely target different event types than hooks from full-ecosystem CLIs.

### Agents (sub-agent spawning) without Rules (6 agents)

Six agents support sub-agent spawning but lack any rules/instructions layer:

| Agent | Form Factor | Why |
|-------|-------------|-----|
| JetBrains Air | IDE | Orchestrator-only; delegates all content to managed agents |
| Melty | IDE | Chat-first; pivoted to Conductor orchestrator |
| Aide | IDE | Discontinued |
| CodeGPT | Extension | Platform-synced agents; no local rules system |
| Claude Squad | Orchestrator | Session manager; inherits content from managed agents |
| IttyBitty | Orchestrator | Manages Claude Code instances; content lives in the managed agent |

The pattern: Agents-without-Rules almost exclusively appears in orchestrators and session managers. These tools delegate content management to the agents they manage rather than carrying their own rules. They are meta-agents rather than content-bearing agents. Syllago likely should not treat "Agents" support in orchestrators as equivalent to "Agents" support in Claude Code or OpenHands.

### Skills without Rules (1 agent)

Only Herm (container-based CLI) has Skills (⚠️, partial) without Rules. This is an exception that proves the rule — Herm's "skills" are partial and the agent has minimal customization overall. In practice, every agent with meaningful Skills support also has Rules.

### Rules without MCP (9 agents)

Nine agents have Rules but not MCP. These fall into three clusters:
- **Research/academic tools** (SWE-agent, Open Interpreter, Live-SWE-agent): built for benchmarks, not extensibility
- **Privacy-first local tools** (Pi, Manus, agenticSeek): deliberately avoid external integrations
- **Orchestrators** (OpenAI Symphony, Composio, AgentPipe): delegate MCP to managed agents

This is less surprising than it appears — most reflect principled architectural decisions, not missing features.

---

## 2.8 Summary: What the Data Tells Syllago

| Finding | Implication |
|---------|-------------|
| Rules + MCP are the universal base | Every new agent integration must prioritize these two types |
| Hooks appear only in fully-built agents (25%) | Hooks converter quality is high-value but narrow audience |
| Skills + Hooks are tightly coupled (73% of Hooks agents have Skills) | Converting a hooks-heavy agent means converting its skills too |
| 8 agents support all 6 content types | Priority targets for full-stack conversion work |
| Orchestrators support "Agents" but not Rules/Skills | "Agents" means different things — orchestration vs. content management |
| CLI form factor achieves the highest content type coverage | CLI agents are syllago's natural primary audience |
| Open-source agents have higher full-stack coverage | OSS agents are better conversion targets; their formats are inspectable |
