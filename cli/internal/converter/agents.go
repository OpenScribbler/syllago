package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
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
	if sourceProvider == "kiro" {
		return canonicalizeKiroAgent(content)
	}
	if sourceProvider == "codex" {
		return canonicalizeCodexAgents(content)
	}

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
	case "roo-code":
		return renderRooCodeAgent(meta, body)
	case "opencode":
		return renderOpenCodeAgent(meta, body)
	case "kiro":
		return renderKiroAgent(meta, body)
	case "codex":
		return renderCodexAgents(meta, body)
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

func renderRooCodeAgent(meta AgentMeta, body string) (*Result, error) {
	// Slugify the name: lowercase and replace spaces/underscores with hyphens.
	slug := strings.ToLower(meta.Name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Map canonical tool names to Roo Code tool groups.
	groupSet := map[string]struct{}{}
	for _, tool := range meta.Tools {
		switch tool {
		case "Read", "Glob", "Grep":
			groupSet["read"] = struct{}{}
		case "Write", "Edit":
			groupSet["edit"] = struct{}{}
		case "Bash":
			groupSet["command"] = struct{}{}
		case "WebSearch", "WebFetch":
			groupSet["browser"] = struct{}{}
		}
	}
	var groups []string
	// Emit in a stable order.
	for _, g := range []string{"read", "edit", "command", "browser"} {
		if _, ok := groupSet[g]; ok {
			groups = append(groups, g)
		}
	}

	mode := rooCodeMode{
		Slug:           slug,
		Name:           meta.Name,
		RoleDefinition: body,
		WhenToUse:      meta.Description,
		Groups:         groups,
	}

	out, err := yaml.Marshal(mode)
	if err != nil {
		return nil, fmt.Errorf("marshalling roo-code mode: %w", err)
	}

	// Warn about fields with no Roo Code equivalent.
	var warnings []string
	if meta.MaxTurns > 0 {
		warnings = append(warnings, "maxTurns has no Roo Code equivalent; dropped")
	}
	if meta.PermissionMode != "" {
		warnings = append(warnings, "permissionMode has no Roo Code equivalent; dropped")
	}
	if meta.Model != "" {
		warnings = append(warnings, "model has no Roo Code equivalent; dropped")
	}
	if meta.Memory != "" {
		warnings = append(warnings, "memory has no Roo Code equivalent; dropped")
	}
	if meta.Background {
		warnings = append(warnings, "background has no Roo Code equivalent; dropped")
	}
	if meta.Isolation != "" {
		warnings = append(warnings, "isolation has no Roo Code equivalent; dropped")
	}
	if len(meta.Skills) > 0 {
		warnings = append(warnings, "skills has no Roo Code equivalent; dropped")
	}
	if len(meta.MCPServers) > 0 {
		warnings = append(warnings, "mcpServers has no Roo Code equivalent; dropped")
	}
	if len(meta.DisallowedTools) > 0 {
		warnings = append(warnings, "disallowedTools has no Roo Code equivalent; dropped")
	}

	return &Result{
		Content:  out,
		Filename: slug + ".yaml",
		Warnings: warnings,
	}, nil
}

// rooCodeMode is the schema for a Roo Code custom mode YAML file.
type rooCodeMode struct {
	Slug               string   `yaml:"slug"`
	Name               string   `yaml:"name"`
	RoleDefinition     string   `yaml:"roleDefinition"`
	WhenToUse          string   `yaml:"whenToUse,omitempty"`
	CustomInstructions string   `yaml:"customInstructions,omitempty"`
	Groups             []string `yaml:"groups,omitempty"`
}

// kiroAgentConfig is Kiro's on-disk agent format.
type kiroAgentConfig struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Prompt       string   `json:"prompt"`
	Model        string   `json:"model,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	AllowedTools []string `json:"allowedTools,omitempty"`
}

func canonicalizeKiroAgent(content []byte) (*Result, error) {
	var ka kiroAgentConfig
	if err := json.Unmarshal(content, &ka); err != nil {
		return nil, fmt.Errorf("parsing Kiro agent JSON: %w", err)
	}

	// Translate Kiro tool names to canonical
	var tools []string
	for _, t := range ka.Tools {
		tools = append(tools, ReverseTranslateTool(t, "kiro"))
	}
	for _, t := range ka.AllowedTools {
		// allowedTools may include granular forms like "@git/git_status"
		base := t
		if idx := strings.Index(t, "/"); idx != -1 {
			base = t[:idx]
		}
		canonical := ReverseTranslateTool(base, "kiro")
		if !containsString(tools, canonical) {
			tools = append(tools, canonical)
		}
	}

	meta := AgentMeta{
		Name:        ka.Name,
		Description: ka.Description,
		Tools:       tools,
		Model:       ka.Model,
	}

	// Prompt body: if it's a file:// reference, store as a note
	body := ""
	if strings.HasPrefix(ka.Prompt, "file://") {
		body = fmt.Sprintf("<!-- kiro:prompt-file=%q -->\n\n(Prompt body loaded from %s)", ka.Prompt, ka.Prompt)
	} else {
		body = ka.Prompt
	}

	canonical, err := buildAgentCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "agent.md"}, nil
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func renderKiroAgent(meta AgentMeta, body string) (*Result, error) {
	var warnings []string
	cleanBody := StripConversionNotes(body)

	agentName := meta.Name
	if agentName == "" {
		agentName = "agent"
	}

	// Translate tools to Kiro names
	kiroTools := TranslateTools(meta.Tools, "kiro")

	// Inline the prompt body directly in the JSON. Kiro supports both inline
	// prompts and file:// references; inline produces a self-contained file
	// that works for both convert (sharing) and install.
	ka := kiroAgentConfig{
		Name:        meta.Name,
		Description: meta.Description,
		Prompt:      cleanBody,
		Tools:       kiroTools,
		Model:       meta.Model,
	}

	if meta.MaxTurns > 0 {
		warnings = append(warnings, fmt.Sprintf("maxTurns (%d) not supported by Kiro (dropped)", meta.MaxTurns))
	}
	if meta.PermissionMode != "" {
		warnings = append(warnings, fmt.Sprintf("permissionMode (%q) not supported by Kiro (dropped)", meta.PermissionMode))
	}
	if len(meta.DisallowedTools) > 0 {
		warnings = append(warnings, "disallowedTools not supported by Kiro; consider using tool groups instead")
	}

	agentJSON, err := json.MarshalIndent(ka, "", "  ")
	if err != nil {
		return nil, err
	}

	return &Result{
		Content:  append(agentJSON, '\n'),
		Filename: slugify(agentName) + ".json",
		Warnings: warnings,
	}, nil
}

// renderOpenCodeAgent renders a canonical agent to OpenCode's markdown format.
// OpenCode agents are markdown files with YAML frontmatter in .opencode/agents/.
// The format is nearly identical to Claude Code's sub-agents.
func renderOpenCodeAgent(meta AgentMeta, body string) (*Result, error) {
	var warnings []string
	cleanBody := StripConversionNotes(body)

	// OpenCode does not support permissionMode
	if meta.PermissionMode != "" {
		warnings = append(warnings, fmt.Sprintf("permissionMode (%q) not supported by OpenCode (dropped)", meta.PermissionMode))
	}

	canonical, err := buildAgentCanonical(AgentMeta{
		Name:        meta.Name,
		Description: meta.Description,
		Tools:       meta.Tools,
		Model:       meta.Model,
		MaxTurns:    meta.MaxTurns,
	}, cleanBody)
	if err != nil {
		return nil, err
	}

	name := "agent"
	if meta.Name != "" {
		name = slugify(meta.Name)
	}
	return &Result{Content: canonical, Filename: name + ".md", Warnings: warnings}, nil
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
