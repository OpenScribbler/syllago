package converter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&AgentsConverter{})
}

// AgentMeta is the canonical agent metadata (YAML frontmatter, superset of all providers).
type AgentMeta struct {
	Name            string   `yaml:"name,omitempty"`
	Description     string   `yaml:"description,omitempty"`
	Tools           []string `yaml:"tools,omitempty"`
	DisallowedTools []string `yaml:"disallowedTools,omitempty"`
	Model           string   `yaml:"model,omitempty"`
	MaxTurns        int      `yaml:"maxTurns,omitempty"`
	PermissionMode  string   `yaml:"permissionMode,omitempty"`
	Skills          []string `yaml:"skills,omitempty"`
	MCPServers      []string `yaml:"mcpServers,omitempty"`
	Memory          string   `yaml:"memory,omitempty"`
	Background      bool     `yaml:"background,omitempty"`
	Isolation       string   `yaml:"isolation,omitempty"`
	// Gemini-specific (stored in canonical for lossless round-trips)
	Temperature float64 `yaml:"temperature,omitempty"`
	TimeoutMins int     `yaml:"timeout_mins,omitempty"`
	Kind        string  `yaml:"kind,omitempty"`
}

// geminiAgentMeta is the subset of fields Gemini CLI supports in frontmatter.
type geminiAgentMeta struct {
	Name        string   `yaml:"name,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
	Model       string   `yaml:"model,omitempty"`
	MaxTurns    int      `yaml:"max_turns,omitempty"`
	Temperature float64  `yaml:"temperature,omitempty"`
	TimeoutMins int      `yaml:"timeout_mins,omitempty"`
	Kind        string   `yaml:"kind,omitempty"`
}

// copilotAgentMeta is the subset of fields Copilot CLI supports.
type copilotAgentMeta struct {
	Name        string   `yaml:"name,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
}

type AgentsConverter struct{}

func (c *AgentsConverter) ContentType() catalog.ContentType {
	return catalog.Agents
}

func (c *AgentsConverter) Canonicalize(content []byte, sourceProvider string) (*Result, error) {
	meta, body, err := parseAgentCanonical(content)
	if err != nil {
		return nil, err
	}

	// Normalize Gemini field names to canonical
	if sourceProvider == "gemini-cli" {
		// max_turns is handled by the YAML tag; temperature/timeout_mins are already in canonical
	}

	// Translate tool names to canonical (Claude Code names)
	if sourceProvider != "claude-code" && sourceProvider != "" {
		for i, tool := range meta.Tools {
			meta.Tools[i] = ReverseTranslateTool(tool, sourceProvider)
		}
		for i, tool := range meta.DisallowedTools {
			meta.DisallowedTools[i] = ReverseTranslateTool(tool, sourceProvider)
		}
	}

	canonical, err := buildAgentCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "agent.md"}, nil
}

func (c *AgentsConverter) Render(content []byte, target provider.Provider) (*Result, error) {
	meta, body, err := parseAgentCanonical(content)
	if err != nil {
		return nil, fmt.Errorf("parsing canonical agent: %w", err)
	}

	switch target.Slug {
	case "gemini-cli":
		return renderGeminiAgent(meta, body)
	case "copilot-cli":
		return renderCopilotAgent(meta, body)
	default:
		// Claude Code — full frontmatter preserved
		return renderClaudeAgent(meta, body)
	}
}

// --- Canonical parser ---

func parseAgentCanonical(content []byte) (AgentMeta, string, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		return AgentMeta{}, strings.TrimSpace(string(normalized)), nil
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		return AgentMeta{}, strings.TrimSpace(string(normalized)), nil
	}

	yamlBytes := rest[:closingIdx]
	var meta AgentMeta
	if err := yaml.Unmarshal(yamlBytes, &meta); err != nil {
		return AgentMeta{}, "", err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))
	return meta, body, nil
}

// --- Renderers ---

func renderGeminiAgent(meta AgentMeta, body string) (*Result, error) {
	// Build behavioral embedding notes for Claude-specific fields
	var notes []string
	if meta.PermissionMode != "" {
		switch meta.PermissionMode {
		case "plan":
			notes = append(notes, "Operate in read-only exploration mode.")
		case "acceptEdits":
			notes = append(notes, "Auto-approve file edits.")
		default:
			notes = append(notes, fmt.Sprintf("Permission mode: %s.", meta.PermissionMode))
		}
	}
	if len(meta.Skills) > 0 {
		notes = append(notes, fmt.Sprintf("Preload these skills: %s.", strings.Join(meta.Skills, ", ")))
	}
	if len(meta.MCPServers) > 0 {
		notes = append(notes, fmt.Sprintf("Expected MCP servers: %s.", strings.Join(meta.MCPServers, ", ")))
	}
	if meta.Memory != "" {
		notes = append(notes, fmt.Sprintf("Use persistent memory scope: %s.", meta.Memory))
	}
	if meta.Background {
		notes = append(notes, "Run as a background task.")
	}
	if meta.Isolation == "worktree" {
		notes = append(notes, "Work in a separate git worktree.")
	}
	if len(meta.DisallowedTools) > 0 {
		translated := TranslateTools(meta.DisallowedTools, "gemini-cli")
		notes = append(notes, fmt.Sprintf("Do not use these tools: %s.", strings.Join(translated, ", ")))
	}

	outBody := body
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		outBody = AppendNotes(body, notesBlock)
	}

	// Build Gemini frontmatter with translated tool names
	gm := geminiAgentMeta{
		Name:        meta.Name,
		Description: meta.Description,
		Tools:       TranslateTools(meta.Tools, "gemini-cli"),
		Model:       meta.Model,
		MaxTurns:    meta.MaxTurns,
		Temperature: meta.Temperature,
		TimeoutMins: meta.TimeoutMins,
		Kind:        meta.Kind,
	}
	fm, err := renderFrontmatter(gm)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(outBody)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: "agent.md"}, nil
}

func renderCopilotAgent(meta AgentMeta, body string) (*Result, error) {
	var notes []string
	if meta.PermissionMode != "" {
		switch meta.PermissionMode {
		case "plan":
			notes = append(notes, "Operate in read-only exploration mode.")
		case "acceptEdits":
			notes = append(notes, "Auto-approve file edits.")
		default:
			notes = append(notes, fmt.Sprintf("Permission mode: %s.", meta.PermissionMode))
		}
	}
	if meta.Model != "" {
		notes = append(notes, fmt.Sprintf("Designed for model: %s.", meta.Model))
	}
	if meta.MaxTurns > 0 {
		notes = append(notes, fmt.Sprintf("Limit to %d turns.", meta.MaxTurns))
	}
	if len(meta.DisallowedTools) > 0 {
		translated := TranslateTools(meta.DisallowedTools, "copilot-cli")
		notes = append(notes, fmt.Sprintf("Do not use these tools: %s.", strings.Join(translated, ", ")))
	}

	outBody := body
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		outBody = AppendNotes(body, notesBlock)
	}

	cm := copilotAgentMeta{
		Name:        meta.Name,
		Description: meta.Description,
		Tools:       TranslateTools(meta.Tools, "copilot-cli"),
	}
	fm, err := renderFrontmatter(cm)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(outBody)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: "agent.md"}, nil
}

func renderClaudeAgent(meta AgentMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	// Embed Gemini-specific fields as conversion notes
	var notes []string
	if meta.Temperature > 0 {
		notes = append(notes, fmt.Sprintf("Use temperature: %.1f for response variability.", meta.Temperature))
	}
	if meta.TimeoutMins > 0 {
		notes = append(notes, fmt.Sprintf("Limit execution to %d minutes.", meta.TimeoutMins))
	}
	if meta.Kind == "remote" {
		notes = append(notes, "Note: This was a remote agent type in the source provider.")
	}

	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("gemini-cli", notes)
		cleanBody = AppendNotes(cleanBody, notesBlock)
	}

	fm, err := renderFrontmatter(meta)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(cleanBody)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: "agent.md"}, nil
}

// --- Helpers ---

func buildAgentCanonical(meta AgentMeta, body string) ([]byte, error) {
	fm, err := renderFrontmatter(meta)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(body)
	buf.WriteString("\n")
	return buf.Bytes(), nil
}
