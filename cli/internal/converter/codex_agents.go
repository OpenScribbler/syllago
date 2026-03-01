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

// canonicalizeCodexAgents parses Codex TOML multi-agent config into canonical agents.
// Returns the first agent as the primary result, additional agents in ExtraFiles.
func canonicalizeCodexAgents(content []byte) (*Result, error) {
	var cfg codexConfig
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing Codex TOML: %w", err)
	}

	if len(cfg.Agents) == 0 {
		return nil, fmt.Errorf("no [agents.*] sections found in Codex TOML")
	}

	var results []struct {
		name    string
		content []byte
	}

	for name, agent := range cfg.Agents {
		meta := AgentMeta{
			Name:  name,
			Model: agent.Model,
		}

		// Reverse-translate Codex tool names to canonical.
		// Codex shares tool vocabulary with Copilot CLI.
		if len(agent.Tools) > 0 {
			canonical := make([]string, len(agent.Tools))
			for i, t := range agent.Tools {
				canonical[i] = ReverseTranslateTool(t, "copilot-cli")
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

	// First agent is the primary result
	primary := results[0]
	result := &Result{
		Content:  primary.content,
		Filename: primary.name + ".md",
	}

	// Additional agents go into ExtraFiles
	if len(results) > 1 {
		result.ExtraFiles = make(map[string][]byte)
		for _, r := range results[1:] {
			result.ExtraFiles[r.name+".md"] = r.content
		}
	}

	return result, nil
}

// renderCodexAgents renders a canonical agent to Codex TOML format.
func renderCodexAgents(meta AgentMeta, body string) (*Result, error) {
	var warnings []string

	// Warn about unsupported fields
	if meta.MaxTurns > 0 {
		warnings = append(warnings, "field 'maxTurns' not supported by Codex (dropped)")
	}
	if meta.PermissionMode != "" {
		warnings = append(warnings, "field 'permissionMode' not supported by Codex (dropped)")
	}
	if len(meta.Skills) > 0 {
		warnings = append(warnings, "field 'skills' not supported by Codex (dropped)")
	}
	if len(meta.MCPServers) > 0 {
		warnings = append(warnings, "field 'mcpServers' not supported by Codex (dropped)")
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

	// Translate tools to Codex names (same as Copilot CLI)
	var codexTools []string
	if len(meta.Tools) > 0 {
		codexTools = TranslateTools(meta.Tools, "copilot-cli")
	}

	// Strip any previous conversion notes from the body
	cleanBody := StripConversionNotes(body)

	// Build TOML
	slug := slugify(meta.Name)
	if slug == "" {
		slug = "agent"
	}

	var buf bytes.Buffer
	buf.WriteString("[features]\nmulti_agent = true\n\n")
	buf.WriteString(fmt.Sprintf("[agents.%s]\n", slug))

	if meta.Model != "" {
		buf.WriteString(fmt.Sprintf("model = %q\n", meta.Model))
	}
	if cleanBody != "" {
		buf.WriteString(fmt.Sprintf("prompt = %q\n", cleanBody))
	}
	if len(codexTools) > 0 {
		buf.WriteString("tools = [")
		for i, t := range codexTools {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(fmt.Sprintf("%q", t))
		}
		buf.WriteString("]\n")
	}

	return &Result{
		Content:  buf.Bytes(),
		Filename: "config.toml",
		Warnings: warnings,
	}, nil
}
