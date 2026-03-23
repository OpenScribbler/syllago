package converter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&CommandsConverter{})
}

// CommandMeta is the canonical command metadata (YAML frontmatter fields).
// Claude Code is the superset, so canonical uses Claude Code field names.
type CommandMeta struct {
	Name                   string   `yaml:"name,omitempty"`
	Description            string   `yaml:"description,omitempty"`
	AllowedTools           []string `yaml:"allowed-tools,omitempty"`
	Context                string   `yaml:"context,omitempty"` // "fork" | ""
	Agent                  string   `yaml:"agent,omitempty"`   // e.g. "Explore"
	Model                  string   `yaml:"model,omitempty"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty"`
	UserInvocable          *bool    `yaml:"user-invocable,omitempty"`
	ArgumentHint           string   `yaml:"argument-hint,omitempty"`
	Effort                 string   `yaml:"effort,omitempty"` // "low", "medium", "high", "max"
}

// codexCommandMeta represents Codex command frontmatter fields.
type codexCommandMeta struct {
	Description  string `yaml:"description,omitempty"`
	ArgumentHint string `yaml:"argument-hint,omitempty"`
}

// opencodeCommandMeta represents OpenCode command frontmatter fields.
type opencodeCommandMeta struct {
	Description string `yaml:"description,omitempty"`
	Agent       string `yaml:"agent,omitempty"`   // maps from canonical Agent field
	Model       string `yaml:"model,omitempty"`   // maps from canonical Model field
	Subtask     bool   `yaml:"subtask,omitempty"` // maps from canonical Context=="fork"
}

// vscodeCopilotCommandMeta represents VS Code Copilot .prompt.md frontmatter fields.
type vscodeCopilotCommandMeta struct {
	Name         string   `yaml:"name,omitempty"`
	Description  string   `yaml:"description,omitempty"`
	Agent        string   `yaml:"agent,omitempty"` // ask, agent, plan
	Model        string   `yaml:"model,omitempty"`
	Tools        []string `yaml:"tools,omitempty"`
	ArgumentHint string   `yaml:"argument-hint,omitempty"`
}

// geminiCommand represents a Gemini CLI command TOML structure.
type geminiCommand struct {
	Name        string `toml:"name,omitempty"`
	Description string `toml:"description,omitempty"`
	Prompt      string `toml:"prompt"`
}

type CommandsConverter struct{}

func (c *CommandsConverter) ContentType() catalog.ContentType {
	return catalog.Commands
}

func (c *CommandsConverter) Canonicalize(content []byte, sourceProvider string) (*Result, error) {
	switch sourceProvider {
	case "gemini-cli":
		return canonicalizeGeminiCommand(content)
	case "codex":
		return canonicalizeCodexCommand(content)
	case "opencode":
		// OpenCode commands use the same format as Claude Code
		return canonicalizeClaudeCommand(content)
	case "vscode-copilot":
		return canonicalizeVSCodeCopilotCommand(content)
	default:
		// Claude Code, Copilot CLI — already YAML frontmatter + markdown
		return canonicalizeClaudeCommand(content)
	}
}

func (c *CommandsConverter) Render(content []byte, target provider.Provider) (*Result, error) {
	meta, body, err := parseCommandCanonical(content)
	if err != nil {
		return nil, fmt.Errorf("parsing canonical command: %w", err)
	}

	switch target.Slug {
	case "gemini-cli":
		return renderGeminiCommand(meta, body)
	case "codex":
		return renderCodexCommand(meta, body)
	case "cursor":
		return renderCursorCommand(meta, body)
	case "opencode":
		return renderOpenCodeCommand(meta, body)
	case "vscode-copilot":
		return renderVSCodeCopilotCommand(meta, body)
	default:
		// Claude Code, Copilot CLI — YAML frontmatter + markdown
		return renderClaudeCommand(meta, body)
	}
}

// --- VS Code Copilot ---

// canonicalizeVSCodeCopilotCommand converts a VS Code Copilot .prompt.md file to canonical format.
// VS Code uses "tools" instead of "allowed-tools", "agent" for execution mode (ask/agent/plan),
// and ${input:varName} for arguments instead of $ARGUMENTS.
func canonicalizeVSCodeCopilotCommand(content []byte) (*Result, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		// No frontmatter — plain prompt
		canonical, err := buildCommandCanonical(CommandMeta{}, strings.TrimSpace(string(normalized)))
		if err != nil {
			return nil, err
		}
		return &Result{Content: canonical, Filename: "command.md"}, nil
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		canonical, err := buildCommandCanonical(CommandMeta{}, strings.TrimSpace(string(normalized)))
		if err != nil {
			return nil, err
		}
		return &Result{Content: canonical, Filename: "command.md"}, nil
	}

	yamlBytes := rest[:closingIdx]
	var vc vscodeCopilotCommandMeta
	if err := yaml.Unmarshal(yamlBytes, &vc); err != nil {
		return nil, fmt.Errorf("parsing VS Code Copilot frontmatter: %w", err)
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))

	// Map to canonical
	meta := CommandMeta{
		Name:         vc.Name,
		Description:  vc.Description,
		Model:        vc.Model,
		ArgumentHint: vc.ArgumentHint,
	}

	// VS Code "agent" field maps to canonical Agent (execution mode):
	// "ask" = read-only/chat, "agent" = full agent mode, "plan" = plan mode
	if vc.Agent != "" {
		meta.Agent = vc.Agent
	}

	// Map VS Code tools to canonical AllowedTools (best effort).
	// These are VS Code-specific tool IDs (e.g. "search/codebase", "myMcpServer/*")
	// but we preserve them as-is since there's no universal tool ID scheme.
	if len(vc.Tools) > 0 {
		meta.AllowedTools = vc.Tools
	}

	// Convert ${input:varName} to $ARGUMENTS in body.
	// This is lossy: named variables become a single positional arg.
	body = replaceVSCodeInputVars(body)

	canonical, err := buildCommandCanonical(meta, body)
	if err != nil {
		return nil, err
	}

	var warnings []string
	if strings.Contains(string(content), "${input:") {
		warnings = append(warnings, "VS Code ${input:varName} variables converted to $ARGUMENTS (named → positional, lossy)")
	}

	return &Result{Content: canonical, Filename: "command.md", Warnings: warnings}, nil
}

// replaceVSCodeInputVars replaces ${input:varName} and ${input:varName:placeholder}
// patterns with $ARGUMENTS. This is lossy: named variables become a single positional arg.
func replaceVSCodeInputVars(body string) string {
	result := body
	for strings.Contains(result, "${input:") {
		start := strings.Index(result, "${input:")
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		result = result[:start] + "$ARGUMENTS" + result[start+end+1:]
	}
	return result
}

// renderVSCodeCopilotCommand renders a canonical command to VS Code Copilot's .prompt.md format.
// VS Code Copilot commands use YAML frontmatter with tools/agent/model fields and
// ${input:args} for argument placeholders.
func renderVSCodeCopilotCommand(meta CommandMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	vc := vscodeCopilotCommandMeta{
		Name:         meta.Name,
		Description:  meta.Description,
		Model:        meta.Model,
		ArgumentHint: meta.ArgumentHint,
	}

	// Map canonical Agent to VS Code agent field
	if meta.Agent != "" {
		vc.Agent = meta.Agent
	}

	// Map canonical AllowedTools to VS Code tools
	if len(meta.AllowedTools) > 0 {
		vc.Tools = meta.AllowedTools
	}

	// Build prose notes for unsupported fields
	var notes []string
	if meta.Context == "fork" {
		notes = append(notes, "Run in an isolated context. Do not modify the main conversation.")
	}
	if meta.Effort != "" {
		notes = append(notes, fmt.Sprintf("Effort level: %s.", meta.Effort))
	}
	if meta.DisableModelInvocation {
		notes = append(notes, "Only invoke when the user explicitly requests it.")
	}

	result := cleanBody
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		result = AppendNotes(cleanBody, notesBlock)
	}

	// Convert $ARGUMENTS to ${input:args} for VS Code
	result = strings.ReplaceAll(result, "$ARGUMENTS", "${input:args}")

	fm, err := renderFrontmatter(vc)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(result)
	buf.WriteString("\n")

	name := "command"
	if meta.Name != "" {
		name = slugify(meta.Name)
	}
	return &Result{Content: buf.Bytes(), Filename: name + ".prompt.md"}, nil
}

// --- Canonical parser ---

func parseCommandCanonical(content []byte) (CommandMeta, string, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		return CommandMeta{}, strings.TrimSpace(string(normalized)), nil
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		return CommandMeta{}, strings.TrimSpace(string(normalized)), nil
	}

	yamlBytes := rest[:closingIdx]
	var meta CommandMeta
	if err := yaml.Unmarshal(yamlBytes, &meta); err != nil {
		return CommandMeta{}, "", err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))
	return meta, body, nil
}

// --- Canonicalizers ---

func canonicalizeClaudeCommand(content []byte) (*Result, error) {
	meta, body, err := parseCommandCanonical(content)
	if err != nil {
		return nil, err
	}
	canonical, err := buildCommandCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "command.md"}, nil
}

func canonicalizeGeminiCommand(content []byte) (*Result, error) {
	var gc geminiCommand
	if err := toml.Unmarshal(content, &gc); err != nil {
		return nil, fmt.Errorf("parsing Gemini TOML command: %w", err)
	}

	meta := CommandMeta{
		Name:        gc.Name,
		Description: gc.Description,
	}
	body := strings.TrimSpace(gc.Prompt)
	body = strings.ReplaceAll(body, "{{args}}", "$ARGUMENTS")

	canonical, err := buildCommandCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "command.md"}, nil
}

func canonicalizeCodexCommand(content []byte) (*Result, error) {
	// Codex commands are plain markdown — wrap with minimal frontmatter
	body := strings.TrimSpace(string(content))
	meta := CommandMeta{}
	canonical, err := buildCommandCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "command.md"}, nil
}

// --- Renderers ---

func renderGeminiCommand(meta CommandMeta, body string) (*Result, error) {
	// Build behavioral embedding notes for Claude-specific fields
	var notes []string
	if len(meta.AllowedTools) > 0 {
		translated := TranslateTools(meta.AllowedTools, "gemini-cli")
		notes = append(notes, fmt.Sprintf("**Tool restriction:** Use only %s tools.", strings.Join(translated, ", ")))
	}
	if meta.Context == "fork" {
		notes = append(notes, "Run in an isolated context. Do not modify the main conversation.")
	}
	if meta.Agent != "" {
		notes = append(notes, fmt.Sprintf("Use a %s-focused approach.", strings.ToLower(meta.Agent)))
	}
	if meta.DisableModelInvocation {
		notes = append(notes, "Only invoke when the user explicitly requests it.")
	}
	if meta.Model != "" {
		notes = append(notes, fmt.Sprintf("Designed for model: %s.", meta.Model))
	}
	if meta.Effort != "" {
		notes = append(notes, fmt.Sprintf("Effort level: %s.", meta.Effort))
	}

	prompt := body
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		prompt = AppendNotes(body, notesBlock)
	}

	// Replace Claude argument placeholder with Gemini's
	prompt = strings.ReplaceAll(prompt, "$ARGUMENTS", "{{args}}")

	gc := geminiCommand{
		Name:        meta.Name,
		Description: meta.Description,
		Prompt:      prompt,
	}

	out, err := toml.Marshal(gc)
	if err != nil {
		return nil, fmt.Errorf("marshaling Gemini TOML: %w", err)
	}

	var warnings []string
	if containsGeminiDirectives(body) {
		// Gemini → Gemini: no warning needed. But this shouldn't happen in render path.
	}

	return &Result{Content: out, Filename: "command.toml", Warnings: warnings}, nil
}

func renderCodexCommand(meta CommandMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	cm := codexCommandMeta{
		Description:  meta.Description,
		ArgumentHint: meta.ArgumentHint,
	}

	// Build behavioral notes only for fields NOT supported in Codex frontmatter.
	// Description and ArgumentHint are now in frontmatter — no notes needed for those.
	var notes []string
	if len(meta.AllowedTools) > 0 {
		notes = append(notes, fmt.Sprintf("**Tool restriction:** Use only %s tools.", strings.Join(meta.AllowedTools, ", ")))
	}
	if meta.Context == "fork" {
		notes = append(notes, "Run in an isolated context. Do not modify the main conversation.")
	}
	if meta.Agent != "" {
		notes = append(notes, fmt.Sprintf("Use a %s-focused approach.", strings.ToLower(meta.Agent)))
	}
	if meta.Model != "" {
		notes = append(notes, fmt.Sprintf("Designed for model: %s.", meta.Model))
	}
	if meta.Effort != "" {
		notes = append(notes, fmt.Sprintf("Effort level: %s.", meta.Effort))
	}

	result := cleanBody
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		result = AppendNotes(cleanBody, notesBlock)
	}

	fm, err := renderFrontmatter(cm)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(result)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: "command.md"}, nil
}

// renderCursorCommand renders a canonical command to Cursor's plain markdown format.
// Cursor commands are plain markdown files — no frontmatter, no TOML.
// Unsupported fields are embedded as behavioral prose notes.
func renderCursorCommand(meta CommandMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	// Build behavioral notes for fields Cursor doesn't support
	var notes []string
	if len(meta.AllowedTools) > 0 {
		translated := TranslateTools(meta.AllowedTools, "cursor")
		notes = append(notes, fmt.Sprintf("**Tool restriction:** Use only %s tools.", strings.Join(translated, ", ")))
	}
	if meta.Context == "fork" {
		notes = append(notes, "Run in an isolated context. Do not modify the main conversation.")
	}
	if meta.Agent != "" {
		notes = append(notes, fmt.Sprintf("Use a %s-focused approach.", strings.ToLower(meta.Agent)))
	}
	if meta.Model != "" {
		notes = append(notes, fmt.Sprintf("Designed for model: %s.", meta.Model))
	}
	if meta.Effort != "" {
		notes = append(notes, fmt.Sprintf("Effort level: %s.", meta.Effort))
	}

	result := cleanBody
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		result = AppendNotes(cleanBody, notesBlock)
	}

	// Convert argument placeholder: $ARGUMENTS → $1 (Cursor's shell-style arg)
	result = strings.ReplaceAll(result, "$ARGUMENTS", "$1")

	return &Result{Content: []byte(result + "\n"), Filename: "command.md"}, nil
}

func renderClaudeCommand(meta CommandMeta, body string) (*Result, error) {
	// Strip any conversion notes that may have been in the canonical body
	cleanBody := StripConversionNotes(body)

	// Check for Gemini template directives and add informational note
	var warnings []string
	if containsGeminiDirectives(cleanBody) {
		warnings = append(warnings, "Command contains Gemini CLI template directives (!{...} or @{...}) that are not natively supported by this provider.")
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

	return &Result{Content: buf.Bytes(), Filename: "command.md", Warnings: warnings}, nil
}

// renderOpenCodeCommand renders a canonical command to OpenCode's markdown format.
// OpenCode commands are markdown files in .opencode/commands/ with optional frontmatter.
// OpenCode natively supports description, agent, model, and subtask fields.
// Other Claude-specific fields are embedded as behavioral notes in the body.
func renderOpenCodeCommand(meta CommandMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	om := opencodeCommandMeta{
		Description: meta.Description,
		Agent:       meta.Agent,
		Model:       meta.Model,
		Subtask:     meta.Context == "fork",
	}

	name := "command"
	if meta.Name != "" {
		name = slugify(meta.Name)
	}

	// Build behavioral notes only for fields NOT supported in OpenCode frontmatter.
	// Agent, Model, and Context→Subtask are now in frontmatter — no notes needed for those.
	var notes []string
	if len(meta.AllowedTools) > 0 {
		notes = append(notes, fmt.Sprintf("**Tool restriction:** Use only %s tools.", strings.Join(meta.AllowedTools, ", ")))
	}
	if meta.Effort != "" {
		notes = append(notes, fmt.Sprintf("Effort level: %s.", meta.Effort))
	}

	result := cleanBody
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		result = AppendNotes(cleanBody, notesBlock)
	}

	fm, err := renderFrontmatter(om)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(result)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: name + ".md"}, nil
}

// --- Helpers ---

func buildCommandCanonical(meta CommandMeta, body string) ([]byte, error) {
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

func containsGeminiDirectives(body string) bool {
	return strings.Contains(body, "!{") || strings.Contains(body, "@{")
}
