package converter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/provider"
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
	Context                string   `yaml:"context,omitempty"`        // "fork" | ""
	Agent                  string   `yaml:"agent,omitempty"`          // e.g. "Explore"
	Model                  string   `yaml:"model,omitempty"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty"`
	UserInvocable          *bool    `yaml:"user-invocable,omitempty"`
	ArgumentHint           string   `yaml:"argument-hint,omitempty"`
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
	default:
		// Claude Code, Copilot CLI — YAML frontmatter + markdown
		return renderClaudeCommand(meta, body)
	}
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

	result := body
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		result = AppendNotes(body, notesBlock)
	}

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
