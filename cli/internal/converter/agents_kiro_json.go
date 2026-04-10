package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// kiroJSONAgent is Kiro CLI's JSON agent format (.kiro/agents/*.json).
// Richer than the IDE markdown format — supports embedded MCP servers,
// per-tool settings, hooks with matchers, and resource references.
type kiroJSONAgent struct {
	Name             string                       `json:"name,omitempty"`
	Description      string                       `json:"description,omitempty"`
	Prompt           string                       `json:"prompt,omitempty"` // system prompt or file:// URI
	Model            string                       `json:"model,omitempty"`
	Tools            []string                     `json:"tools,omitempty"`
	AllowedTools     []string                     `json:"allowedTools,omitempty"`
	ToolAliases      map[string]string            `json:"toolAliases,omitempty"`
	ToolsSettings    map[string]any               `json:"toolsSettings,omitempty"`
	MCPServers       map[string]kiroJSONMCPServer `json:"mcpServers,omitempty"`
	Resources        json.RawMessage              `json:"resources,omitempty"` // string[] or object[]
	Hooks            map[string][]kiroJSONHook    `json:"hooks,omitempty"`
	KeyboardShortcut string                       `json:"keyboardShortcut,omitempty"`
	WelcomeMessage   string                       `json:"welcomeMessage,omitempty"`
	IncludeMCPJSON   bool                         `json:"includeMcpJson,omitempty"`
}

// kiroJSONMCPServer is an embedded MCP server definition in Kiro CLI agents.
type kiroJSONMCPServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
}

// kiroJSONHook is a lifecycle hook entry in Kiro CLI agents.
type kiroJSONHook struct {
	Command         string `json:"command"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds,omitempty"`
	MaxOutputSize   int    `json:"max_output_size,omitempty"`
	Matcher         string `json:"matcher,omitempty"`
}

// canonicalizeKiroAgentJSON parses a Kiro CLI JSON agent into canonical format.
func canonicalizeKiroAgentJSON(content []byte) (*Result, error) {
	var ka kiroJSONAgent
	if err := json.Unmarshal(content, &ka); err != nil {
		return nil, fmt.Errorf("parsing Kiro CLI JSON agent: %w", err)
	}

	// Translate Kiro tool names to canonical
	var tools []string
	for _, t := range ka.Tools {
		tools = append(tools, ReverseTranslateTool(t, "kiro"))
	}
	for _, t := range ka.AllowedTools {
		base := t
		if idx := strings.Index(t, "/"); idx != -1 {
			base = t[:idx]
		}
		canonical := ReverseTranslateTool(base, "kiro")
		if !containsString(tools, canonical) {
			tools = append(tools, canonical)
		}
	}

	// Extract MCP server names from embedded definitions
	var mcpServers []string
	for name := range ka.MCPServers {
		mcpServers = append(mcpServers, name)
	}

	// Resolve prompt: file:// URIs can't be resolved here, preserve as-is
	body := ka.Prompt
	if strings.HasPrefix(body, "file://") {
		// Can't resolve file URIs during conversion — embed as a note
		body = fmt.Sprintf("<!-- Prompt loaded from: %s -->\n\n(Prompt content from external file not available during conversion.)", body)
	}

	meta := AgentMeta{
		Name:        ka.Name,
		Description: ka.Description,
		Tools:       tools,
		Model:       ka.Model,
		MCPServers:  mcpServers,
	}

	canonical, err := buildAgentCanonical(meta, body)
	if err != nil {
		return nil, err
	}

	var warnings []string
	if len(ka.ToolAliases) > 0 {
		warnings = append(warnings, "toolAliases dropped (no canonical equivalent)")
	}
	if len(ka.ToolsSettings) > 0 {
		warnings = append(warnings, "toolsSettings dropped (no canonical equivalent)")
	}
	if len(ka.Resources) > 0 {
		warnings = append(warnings, "resources dropped (no canonical equivalent)")
	}
	if len(ka.Hooks) > 0 {
		warnings = append(warnings, fmt.Sprintf("hooks dropped (%d event(s); Kiro CLI hook format not portable)", len(ka.Hooks)))
	}
	if ka.KeyboardShortcut != "" {
		warnings = append(warnings, "keyboardShortcut dropped (no canonical equivalent)")
	}
	if ka.WelcomeMessage != "" {
		warnings = append(warnings, "welcomeMessage dropped (no canonical equivalent)")
	}

	return &Result{Content: canonical, Filename: "agent.md", Warnings: warnings}, nil
}

// renderKiroAgentJSON renders a canonical agent to Kiro CLI JSON format.
func renderKiroAgentJSON(meta AgentMeta, body string) (*Result, error) {
	var warnings []string
	cleanBody := StripConversionNotes(body)

	kiroTools := TranslateTools(meta.Tools, "kiro")

	ka := kiroJSONAgent{
		Name:        meta.Name,
		Description: meta.Description,
		Prompt:      cleanBody,
		Model:       meta.Model,
		Tools:       kiroTools,
	}

	// Map canonical MCPServers (string names) to empty embedded objects
	if len(meta.MCPServers) > 0 {
		ka.MCPServers = make(map[string]kiroJSONMCPServer, len(meta.MCPServers))
		for _, name := range meta.MCPServers {
			ka.MCPServers[name] = kiroJSONMCPServer{}
		}
	}

	if meta.MaxTurns > 0 {
		warnings = append(warnings, fmt.Sprintf("maxTurns (%d) not supported by Kiro CLI (dropped)", meta.MaxTurns))
	}
	if meta.PermissionMode != "" {
		warnings = append(warnings, fmt.Sprintf("permissionMode (%q) not supported by Kiro CLI (dropped)", meta.PermissionMode))
	}
	if len(meta.DisallowedTools) > 0 {
		warnings = append(warnings, "disallowedTools not supported by Kiro CLI; consider using toolsSettings")
	}
	if len(meta.Skills) > 0 {
		warnings = append(warnings, "skills not supported by Kiro CLI (dropped)")
	}
	if meta.Memory != "" {
		warnings = append(warnings, "memory not supported by Kiro CLI (dropped)")
	}
	if meta.Background {
		warnings = append(warnings, "background not supported by Kiro CLI (dropped)")
	}
	if meta.Isolation != "" {
		warnings = append(warnings, "isolation not supported by Kiro CLI (dropped)")
	}
	if meta.Effort != "" {
		warnings = append(warnings, "effort not supported by Kiro CLI (dropped)")
	}
	if meta.Hooks != nil {
		warnings = append(warnings, "hooks not supported in canonical→Kiro CLI direction (Kiro CLI hooks have a different schema)")
	}
	if meta.Color != "" {
		warnings = append(warnings, "color not supported by Kiro CLI (dropped)")
	}

	out, err := json.MarshalIndent(ka, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding Kiro CLI JSON agent: %w", err)
	}

	name := "agent"
	if meta.Name != "" {
		name = slugify(meta.Name)
	}
	return &Result{Content: out, Filename: name + ".json", Warnings: warnings}, nil
}
