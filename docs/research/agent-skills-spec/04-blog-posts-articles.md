# Blog Posts & Technical Articles

## Critical Analysis

### Simon Willison (simonwillison.net, Dec 2025)

Called the spec "a deliciously tiny specification" but also "quite heavily under-specified." Flagged:
- `metadata` field is vague — only recommends "reasonably unique" key names
- `allowed-skills` marked "Experimental. Support for this field may vary."
- Predicted it might "end up in the AAIF" (Agentic AI Foundation)
- Suggested skills might be "a bigger deal than MCP" due to token efficiency

Source: https://simonwillison.net/2025/Dec/19/agent-skills/

### A B Vijay Kumar (Medium, Mar 2026)

Deep dive identifying three core constraints:
1. "Skills lack memory, cannot directly call APIs, and have context window limits"
2. Not suitable for "highly dynamic logic with complex branching or real-time feedback"
3. "Skill effectiveness varies across different models, impacting cross-platform compatibility"

On undertriggering: "simple queries like 'read this PDF' may not trigger the pdf skill."
On versioning: "Skills don't have a built-in versioning mechanism."
On composability: "They can't explicitly talk to each other."

Source: https://abvijaykumar.medium.com/deep-dive-skill-md-part-1-2-09fc9a536996

### inference.sh (Feb 2026)

Documented platform fragmentation: "Claude Code looks in ~/.claude/skills by default. Codex CLI uses ~/.codex/skills with an enable flag. Cursor reads from project-level directories." Conflict resolution is "agent-dependent rather than standardized."

Source: https://inference.sh/blog/skills/agent-skills-overview

### Nils Friedrichs (friedrichs-it.de, 2026)

Most technically detailed security comparison. Key findings:
- Agent Skills: "in-process execution." MCP: "process isolation" with separate runtime
- Spec provides **no credential guidance**
- "There's no way to grant a skill limited permissions"
- OpenClaw "persists secrets in a shared local directory"

Source: https://www.friedrichs-it.de/blog/agent-skills-vs-model-context-protocol/

## Experience Reports

### Microsoft .NET Team (devblogs.microsoft.com)

Structured measurement approach: "For each skill merged, we run a lightweight validator to score it" against no-skill baseline. Acknowledged "more is always better" doesn't apply — newer models may render previously essential context unnecessary. Discovery remains a pain point: "discovery of even marketplaces can be a challenge."

Source: https://devblogs.microsoft.com/dotnet/extend-your-coding-agent-with-dotnet-skills/

### Will Larson (lethain.com)

Initial skeptic → convert. Two problems forced reconsideration: managing reusable snippets across workflows and preventing irrelevant context. Verdict: "For something that I was initially deeply skeptical about, I now wish I had implemented skills much earlier."

Concrete gaps identified:
- No support for loading sub-skill files (progressive disclosure within a skill)
- Lack of sandboxed Python execution

Built own implementation to remain "largely platform agnostic."

Source: https://lethain.com/agents-skills/

### Tristan Handy (dbt Labs, Substack)

Deployed migrate-to-fusion skill on real dbt Core 1.10 project: completed entire migration "with zero help from me; Fusion compiled and ran flawlessly." Frames skills as encoding "hundreds, maybe thousands of hours of collective human experience" in 12kb of markdown. Notes ecosystem is thin — only 8 dbt-related skills exist.

Source: https://roundup.getdbt.com/p/agent-skills-disseminating-expertise

### Bibek Poudel (Medium, Feb 2026)

Core debugging insight: "skills failing to trigger due to poor descriptions rather than flawed instructions." Emphasized "it is almost never the instructions" causing failures. Constraints that create friction: descriptions limited to 1024 characters, XML angle bracket restrictions, naming conventions. Skills "do not guarantee execution."

Source: https://bibek-poudel.medium.com/the-skill-md-pattern-how-to-write-ai-agent-skills-that-actually-work-72a3169dd7ee

### Arcade.dev

Token economics favor skills: "One GitHub MCP server can expose 90+ tools, consuming over 50,000 tokens of JSON schemas." Anthropic reduced 150K-token workflow to ~2K with skills. But "roughly 70% of AI projects fail to reach production, and authorization complexity is a primary culprit."

Source: https://www.arcade.dev/blog/what-are-agent-skills-and-tools/

## Comparison Articles

### Goose/Block Team (Dec 2025)

Clear framing: "MCP is where capability lives. Skills encode how work should be done." Skills without MCP = "well written instructions with no execution capability." MCP without Skills = "raw power with no guidance."

Source: https://block.github.io/goose/blog/2025/12/22/agent-skills-vs-mcp/

### Layered.dev

Praised progressive disclosure: "Skills got it right from the start. MCP hosts are catching up." Skills mandate lazy context loading at the protocol level; MCP has no such requirement.

Source: https://layered.dev/mcp-vs-agent-skills/

### LlamaIndex

"Being defined with natural language leaves skills open for misinterpretations and hallucinations by the LLM." Positioned as complementary to MCP tools.

Source: https://www.llamaindex.ai/blog/skills-vs-mcp-tools-for-agents-when-to-use-what

## Enterprise/Security Articles

### Snyk ToxicSkills (Feb 2026)

3,984 skills scanned: 13.4% critical issues, 36.82% flaws of any severity, 76 confirmed malicious payloads. "100% of confirmed malicious skills contain malicious code patterns, while 91% simultaneously employ prompt injection techniques."

Source: https://snyk.io/blog/toxicskills-malicious-ai-agent-skills-clawhub/

### Grith.ai/Koi Security

341 malicious out of 2,857 (12%). Documented six-step install-to-exploit chain. "Defensive leverage is highest at execution time."

Source: https://grith.ai/blog/agent-skills-supply-chain

### Red Hat (Mar 2026)

Five threat categories: filesystem exploitation, malicious scripts, YAML parser vulnerabilities, prompt injection, credential exposure. "No industry-standard fix" for prompt injection. Recommends container isolation, signed skills, guardrails.

Source: https://developers.redhat.com/articles/2026/03/10/agent-skills-explore-security-threats-and-controls

### JFrog

"Who actually wrote this skill? What version is this? Are there malicious prompts hidden inside?" Need for centralized versioning, ownership attribution, cryptographic provenance attestation.

Source: https://jfrog.com/blog/agent-skills-new-ai-packages/

### Pluto Security

Second-order supply chain risk: "malicious skills remain discoverable even after takedown" through downstream aggregators. SkillsMP indexes 145K+ skills. Advocates capability-based permission manifests, mandatory scanning, runtime sandboxing.

Source: https://blog.pluto.security/p/clawing-out-the-skills-marketplace

### Subramanya N

Enterprise gap: "shadow AI" without standardized skill registries. Need "policy engines to control which agents can use which skills in specific contexts."

Source: https://subramanya.ai/2025/12/18/agent-skills-the-missing-piece-of-the-enterprise-ai-puzzle/

### OWASP Agentic Skills Top 10

Dedicated threat model created in response to the security crisis.

Source: https://owasp.org/www-project-agentic-skills-top-10/
