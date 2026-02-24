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
	Register(&SkillsConverter{})
}

// SkillMeta is the canonical skill metadata (YAML frontmatter fields).
// Claude Code is the superset.
type SkillMeta struct {
	Name                   string   `yaml:"name,omitempty"`
	Description            string   `yaml:"description,omitempty"`
	AllowedTools           []string `yaml:"allowed-tools,omitempty"`
	DisallowedTools        []string `yaml:"disallowed-tools,omitempty"`
	Context                string   `yaml:"context,omitempty"`
	Agent                  string   `yaml:"agent,omitempty"`
	Model                  string   `yaml:"model,omitempty"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty"`
	UserInvocable          *bool    `yaml:"user-invocable,omitempty"`
	ArgumentHint           string   `yaml:"argument-hint,omitempty"`
}

// geminiSkillMeta is the subset of fields Gemini CLI supports.
type geminiSkillMeta struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type SkillsConverter struct{}

func (c *SkillsConverter) ContentType() catalog.ContentType {
	return catalog.Skills
}

func (c *SkillsConverter) Canonicalize(content []byte, sourceProvider string) (*Result, error) {
	// Both providers use YAML frontmatter + markdown
	meta, body, err := parseSkillCanonical(content)
	if err != nil {
		return nil, err
	}
	canonical, err := buildSkillCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "SKILL.md"}, nil
}

func (c *SkillsConverter) Render(content []byte, target provider.Provider) (*Result, error) {
	meta, body, err := parseSkillCanonical(content)
	if err != nil {
		return nil, fmt.Errorf("parsing canonical skill: %w", err)
	}

	switch target.Slug {
	case "gemini-cli":
		return renderGeminiSkill(meta, body)
	default:
		// Claude Code, Copilot CLI — full frontmatter preserved
		return renderClaudeSkill(meta, body)
	}
}

// --- Canonical parser ---

func parseSkillCanonical(content []byte) (SkillMeta, string, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		return SkillMeta{}, strings.TrimSpace(string(normalized)), nil
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		return SkillMeta{}, strings.TrimSpace(string(normalized)), nil
	}

	yamlBytes := rest[:closingIdx]
	var meta SkillMeta
	if err := yaml.Unmarshal(yamlBytes, &meta); err != nil {
		return SkillMeta{}, "", err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))
	return meta, body, nil
}

// --- Renderers ---

func renderGeminiSkill(meta SkillMeta, body string) (*Result, error) {
	// Build behavioral embedding notes for Claude-specific fields
	var notes []string
	if len(meta.AllowedTools) > 0 {
		translated := TranslateTools(meta.AllowedTools, "gemini-cli")
		notes = append(notes, fmt.Sprintf("**Tool restriction:** Use only %s tools.", strings.Join(translated, ", ")))
	}
	if len(meta.DisallowedTools) > 0 {
		translated := TranslateTools(meta.DisallowedTools, "gemini-cli")
		notes = append(notes, fmt.Sprintf("**Do not use:** %s tools.", strings.Join(translated, ", ")))
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
	if meta.DisableModelInvocation {
		notes = append(notes, "Only invoke when the user explicitly requests it.")
	}
	if meta.UserInvocable != nil && *meta.UserInvocable {
		notes = append(notes, "Intended to appear in the command menu.")
	}
	if meta.ArgumentHint != "" {
		notes = append(notes, fmt.Sprintf("Usage: %s", meta.ArgumentHint))
	}

	outBody := body
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		outBody = AppendNotes(body, notesBlock)
	}

	// Emit Gemini-compatible frontmatter (name + description only)
	gm := geminiSkillMeta{
		Name:        meta.Name,
		Description: meta.Description,
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

	return &Result{Content: buf.Bytes(), Filename: "SKILL.md"}, nil
}

func renderClaudeSkill(meta SkillMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	fm, err := renderFrontmatter(meta)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(cleanBody)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: "SKILL.md"}, nil
}

// --- Helpers ---

func buildSkillCanonical(meta SkillMeta, body string) ([]byte, error) {
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
