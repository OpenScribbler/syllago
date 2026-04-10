package converter

import (
	"bytes"
	"fmt"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// codexAgentConfig represents a single agent in Codex's TOML config.
type codexAgentConfig struct {
	Model  string   `toml:"model,omitempty"`
	Prompt string   `toml:"prompt,omitempty"`
	Tools  []string `toml:"tools,omitempty"`
}

// codexConfig represents the full Codex config.toml structure.
type codexConfig struct {
	Features map[string]bool             `toml:"features,omitempty"`
	Agents   map[string]codexAgentConfig `toml:"agents,omitempty"`
}

// canonicalizeCodexAgents parses Codex agent TOML into canonical agents.
// Handles both formats:
//   - Multi-agent (AGENTS.toml): [features] + [agents.<name>] sections
//   - Single-agent (.codex/agents/*.toml): [agent] with developer_instructions
func canonicalizeCodexAgents(content []byte) (*Result, error) {
	// Try single-agent format first (more specific structure)
	var single codexSingleAgent
	if err := toml.Unmarshal(content, &single); err == nil && single.Agent.Name != "" {
		return canonicalizeSingleCodexAgent(single)
	}

	// Fall back to multi-agent format
	var cfg codexConfig
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing Codex TOML: %w", err)
	}

	if len(cfg.Agents) == 0 {
		return nil, fmt.Errorf("no [agent] or [agents.*] sections found in Codex TOML")
	}

	return canonicalizeMultiCodexAgents(cfg)
}

// canonicalizeSingleCodexAgent handles the [agent] with developer_instructions format.
func canonicalizeSingleCodexAgent(single codexSingleAgent) (*Result, error) {
	meta := AgentMeta{
		Name:        single.Agent.Name,
		Description: single.Agent.Description,
		Model:       single.Agent.Model,
	}

	if len(single.Agent.Tools) > 0 {
		canonical := make([]string, len(single.Agent.Tools))
		for i, t := range single.Agent.Tools {
			canonical[i] = ReverseTranslateTool(t, "codex")
		}
		meta.Tools = canonical
	}

	// Extract MCP server names from the map keys
	if len(single.Agent.MCPServers) > 0 {
		for name := range single.Agent.MCPServers {
			meta.MCPServers = append(meta.MCPServers, name)
		}
	}

	// Extract skill names from the config map keys
	if len(single.Agent.Skills.Config) > 0 {
		for name := range single.Agent.Skills.Config {
			meta.Skills = append(meta.Skills, name)
		}
	}

	body := strings.TrimSpace(single.Agent.DeveloperInstructions)

	data, err := buildAgentCanonical(meta, body)
	if err != nil {
		return nil, fmt.Errorf("building canonical for agent %q: %w", meta.Name, err)
	}

	return &Result{
		Content:  data,
		Filename: meta.Name + ".md",
	}, nil
}

// canonicalizeMultiCodexAgents handles the [features] + [agents.<name>] format.
func canonicalizeMultiCodexAgents(cfg codexConfig) (*Result, error) {
	var results []struct {
		name    string
		content []byte
	}

	for name, agent := range cfg.Agents {
		meta := AgentMeta{
			Name:  name,
			Model: agent.Model,
		}

		if len(agent.Tools) > 0 {
			canonical := make([]string, len(agent.Tools))
			for i, t := range agent.Tools {
				canonical[i] = ReverseTranslateTool(t, "codex")
			}
			meta.Tools = canonical
		}

		body := strings.TrimSpace(agent.Prompt)

		data, err := buildAgentCanonical(meta, body)
		if err != nil {
			return nil, fmt.Errorf("building canonical for agent %q: %w", name, err)
		}

		results = append(results, struct {
			name    string
			content []byte
		}{name: name, content: data})
	}

	primary := results[0]
	result := &Result{
		Content:  primary.content,
		Filename: primary.name + ".md",
	}

	if len(results) > 1 {
		result.ExtraFiles = make(map[string][]byte)
		for _, r := range results[1:] {
			result.ExtraFiles[r.name+".md"] = r.content
		}
	}

	return result, nil
}

// codexSingleAgent is the single-agent TOML format used for .codex/agents/<name>.toml files.
type codexSingleAgent struct {
	Agent codexSingleAgentBody `toml:"agent"`
}

type codexSingleAgentBody struct {
	Name                  string            `toml:"name"`
	Description           string            `toml:"description,omitempty"`
	Model                 string            `toml:"model,omitempty"`
	ModelReasoningEffort  string            `toml:"model_reasoning_effort,omitempty"`
	Tools                 []string          `toml:"tools,omitempty"`
	DeveloperInstructions string            `toml:"developer_instructions,omitempty"`
	SandboxMode           string            `toml:"sandbox_mode,omitempty"`
	NicknameCandidates    []string          `toml:"nickname_candidates,omitempty"`
	MCPServers            map[string]any    `toml:"mcp_servers,omitempty"`
	Skills                codexSkillsConfig `toml:"skills,omitempty"`
}

// codexSkillsConfig represents the [agent.skills] section with a config sub-table.
type codexSkillsConfig struct {
	Config map[string]any `toml:"config,omitempty"`
}

// renderCodexAgents renders a canonical agent to Codex single-agent TOML format.
// Outputs the per-file format with [agent] and developer_instructions used by
// .codex/agents/<name>.toml, not the multi-agent format ([agents.<name>]) used by AGENTS.toml.
func renderCodexAgents(meta AgentMeta, body string) (*Result, error) {
	var warnings []string

	// Warn about unsupported fields
	if meta.MaxTurns > 0 {
		warnings = append(warnings, "field 'maxTurns' not supported by Codex (dropped)")
	}
	if meta.PermissionMode != "" {
		warnings = append(warnings, "field 'permissionMode' not supported by Codex (dropped)")
	}
	if meta.Memory != "" {
		warnings = append(warnings, "field 'memory' not supported by Codex (dropped)")
	}
	if meta.Background {
		warnings = append(warnings, "field 'background' not supported by Codex (dropped)")
	}
	if meta.Isolation != "" {
		warnings = append(warnings, "field 'isolation' not supported by Codex (dropped)")
	}
	if len(meta.DisallowedTools) > 0 {
		warnings = append(warnings, "field 'disallowedTools' not supported by Codex (dropped)")
	}

	// Translate tools to Codex names
	var codexTools []string
	if len(meta.Tools) > 0 {
		codexTools = TranslateTools(meta.Tools, "codex")
	}

	cleanBody := StripConversionNotes(body)

	slug := slugify(meta.Name)
	if slug == "" {
		slug = "agent"
	}

	// Build MCP servers map from canonical names
	var mcpServers map[string]any
	if len(meta.MCPServers) > 0 {
		mcpServers = make(map[string]any, len(meta.MCPServers))
		for _, name := range meta.MCPServers {
			mcpServers[name] = map[string]any{}
		}
	}

	// Build skills config from canonical names
	var skills codexSkillsConfig
	if len(meta.Skills) > 0 {
		skills.Config = make(map[string]any, len(meta.Skills))
		for _, name := range meta.Skills {
			skills.Config[name] = map[string]any{}
		}
	}

	cfg := codexSingleAgent{
		Agent: codexSingleAgentBody{
			Name:                  meta.Name,
			Description:           meta.Description,
			Model:                 meta.Model,
			Tools:                 codexTools,
			DeveloperInstructions: cleanBody,
			MCPServers:            mcpServers,
			Skills:                skills,
		},
	}

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(true)
	if err := enc.Encode(cfg); err != nil {
		return nil, fmt.Errorf("encoding Codex agent TOML: %w", err)
	}

	return &Result{
		Content:  buf.Bytes(),
		Filename: slug + ".toml",
		Warnings: warnings,
	}, nil
}
