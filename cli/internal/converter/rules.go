package converter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/parse"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&RulesConverter{})
}

// RuleMeta is the canonical rule metadata (YAML frontmatter fields).
type RuleMeta struct {
	Description string   `yaml:"description,omitempty"`
	AlwaysApply bool     `yaml:"alwaysApply"`
	Globs       []string `yaml:"globs,omitempty"`
}

// RulesConverter handles conversion of Rules content between providers.
type RulesConverter struct{}

func (c *RulesConverter) ContentType() catalog.ContentType {
	return catalog.Rules
}

// Canonicalize converts provider-specific rule content to canonical format
// (YAML frontmatter with description/alwaysApply/globs + markdown body).
func (c *RulesConverter) Canonicalize(content []byte, sourceProvider string) (*Result, error) {
	switch sourceProvider {
	case "cursor":
		return canonicalizeCursorRule(content)
	case "windsurf":
		return canonicalizeWindsurfRule(content)
	case "cline":
		return canonicalizeClineRule(content)
	case "opencode":
		return canonicalizeMarkdownRule(content)
	case "kiro":
		return canonicalizeMarkdownRule(content)
	default:
		return canonicalizeMarkdownRule(content)
	}
}

// Render converts canonical rule content to a target provider's format.
func (c *RulesConverter) Render(content []byte, target provider.Provider) (*Result, error) {
	meta, body, err := parseCanonical(content)
	if err != nil {
		return nil, fmt.Errorf("parsing canonical rule: %w", err)
	}

	switch target.Slug {
	case "cursor":
		return renderCursorRule(meta, body)
	case "windsurf":
		return renderWindsurfRule(meta, body)
	case "claude-code", "codex", "gemini-cli", "copilot-cli":
		return renderSingleFileRule(meta, body)
	case "zed":
		return renderZedRule(meta, body)
	case "cline":
		return renderClineRule(meta, body)
	case "roo-code":
		return renderRooCodeRule(meta, body)
	case "opencode":
		return renderOpenCodeRule(meta, body)
	case "kiro":
		return renderKiroRule(meta, body)
	default:
		return renderMarkdownRule(meta, body)
	}
}

// --- Canonical parser ---

// parseCanonical extracts RuleMeta and body from canonical format (YAML frontmatter + markdown).
func parseCanonical(content []byte) (RuleMeta, string, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		// No frontmatter — treat as alwaysApply plain markdown
		return RuleMeta{AlwaysApply: true}, strings.TrimSpace(string(normalized)), nil
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		return RuleMeta{AlwaysApply: true}, strings.TrimSpace(string(normalized)), nil
	}

	yamlBytes := rest[:closingIdx]
	var meta RuleMeta
	if err := yaml.Unmarshal(yamlBytes, &meta); err != nil {
		return RuleMeta{}, "", err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))
	return meta, body, nil
}

// renderFrontmatter marshals any struct as YAML frontmatter.
func renderFrontmatter(v any) ([]byte, error) {
	yamlBytes, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	return buf.Bytes(), nil
}

// --- Canonicalize parsers (provider → canonical) ---

func canonicalizeCursorRule(content []byte) (*Result, error) {
	fm, body, err := parse.ParseMDCFrontmatter(content)
	if err != nil {
		// No frontmatter — treat as always-apply plain markdown
		meta := RuleMeta{AlwaysApply: true}
		canonical, err := buildCanonical(meta, strings.TrimSpace(string(content)))
		if err != nil {
			return nil, err
		}
		return &Result{Content: canonical, Filename: "rule.md"}, nil
	}

	meta := RuleMeta{
		Description: fm.Description,
		AlwaysApply: fm.AlwaysApply,
		Globs:       fm.Globs,
	}

	canonical, err := buildCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "rule.md"}, nil
}

// windsurfFrontmatter represents Windsurf's YAML frontmatter fields.
type windsurfFrontmatter struct {
	Trigger     string `yaml:"trigger"`
	Description string `yaml:"description,omitempty"`
	Globs       string `yaml:"globs,omitempty"`
}

func canonicalizeWindsurfRule(content []byte) (*Result, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		meta := RuleMeta{AlwaysApply: true}
		canonical, err := buildCanonical(meta, strings.TrimSpace(string(normalized)))
		if err != nil {
			return nil, err
		}
		return &Result{Content: canonical, Filename: "rule.md"}, nil
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		meta := RuleMeta{AlwaysApply: true}
		canonical, err := buildCanonical(meta, strings.TrimSpace(string(normalized)))
		if err != nil {
			return nil, err
		}
		return &Result{Content: canonical, Filename: "rule.md"}, nil
	}

	yamlBytes := rest[:closingIdx]
	var wfm windsurfFrontmatter
	if err := yaml.Unmarshal(yamlBytes, &wfm); err != nil {
		return nil, err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))

	meta := RuleMeta{Description: wfm.Description}
	switch wfm.Trigger {
	case "always_on":
		meta.AlwaysApply = true
	case "glob":
		meta.AlwaysApply = false
		if wfm.Globs != "" {
			meta.Globs = splitGlobs(wfm.Globs)
		}
	case "model_decision":
		meta.AlwaysApply = false
		// description carries the activation hint
	case "manual":
		meta.AlwaysApply = false
	default:
		// Unknown trigger — default to model_decision
		meta.AlwaysApply = false
	}

	canonical, err := buildCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "rule.md"}, nil
}

func canonicalizeMarkdownRule(content []byte) (*Result, error) {
	// Check if it already has canonical frontmatter
	meta, body, err := parseCanonical(content)
	if err != nil {
		meta = RuleMeta{AlwaysApply: true}
		body = strings.TrimSpace(string(content))
	}
	// If parsed but has no explicit fields, default to alwaysApply
	if !meta.AlwaysApply && meta.Description == "" && len(meta.Globs) == 0 {
		meta.AlwaysApply = true
	}

	canonical, err := buildCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "rule.md"}, nil
}

// clineFrontmatter represents Cline's YAML frontmatter fields.
type clineFrontmatter struct {
	Paths []string `yaml:"paths,omitempty"`
}

func canonicalizeClineRule(content []byte) (*Result, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		meta := RuleMeta{AlwaysApply: true}
		canonical, err := buildCanonical(meta, strings.TrimSpace(string(normalized)))
		if err != nil {
			return nil, err
		}
		return &Result{Content: canonical, Filename: "rule.md"}, nil
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		meta := RuleMeta{AlwaysApply: true}
		canonical, err := buildCanonical(meta, strings.TrimSpace(string(normalized)))
		if err != nil {
			return nil, err
		}
		return &Result{Content: canonical, Filename: "rule.md"}, nil
	}

	yamlBytes := rest[:closingIdx]
	var cfm clineFrontmatter
	if err := yaml.Unmarshal(yamlBytes, &cfm); err != nil {
		return nil, err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))

	meta := RuleMeta{}
	if len(cfm.Paths) > 0 {
		meta.Globs = cfm.Paths
	} else {
		meta.AlwaysApply = true
	}

	canonical, err := buildCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "rule.md"}, nil
}

// --- Renderers (canonical → provider) ---

func renderCursorRule(meta RuleMeta, body string) (*Result, error) {
	// Cursor uses the same fields as canonical (alwaysApply, globs, description)
	fm, err := renderFrontmatter(meta)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(body)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: "rule.mdc"}, nil
}

// windsurfOutput represents the Windsurf frontmatter for rendering.
type windsurfOutput struct {
	Trigger     string `yaml:"trigger"`
	Description string `yaml:"description,omitempty"`
	Globs       string `yaml:"globs,omitempty"`
}

func renderWindsurfRule(meta RuleMeta, body string) (*Result, error) {
	wf := windsurfOutput{Description: meta.Description}

	switch {
	case meta.AlwaysApply:
		wf.Trigger = "always_on"
	case len(meta.Globs) > 0:
		wf.Trigger = "glob"
		wf.Globs = strings.Join(meta.Globs, ", ")
	case meta.Description != "":
		wf.Trigger = "model_decision"
	default:
		wf.Trigger = "manual"
	}

	fm, err := renderFrontmatter(wf)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(body)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: "rule.md"}, nil
}

// renderSingleFileRule renders for providers that use a flat markdown file
// (Claude Code, Codex, Gemini CLI). Non-alwaysApply rules get scope embedded as prose.
func renderSingleFileRule(meta RuleMeta, body string) (*Result, error) {
	if meta.AlwaysApply {
		// Always-active rules get body only — no frontmatter
		content := []byte(body + "\n")
		return &Result{Content: content, Filename: "rule.md"}, nil
	}

	// Embed activation scope as prose
	var notes []string
	switch {
	case len(meta.Globs) > 0:
		notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
	case meta.Description != "":
		notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
	default:
		notes = append(notes, "**Scope:** Apply only when explicitly asked.")
	}

	notesBlock := BuildConversionNotes("syllago", notes)
	result := AppendNotes(body, notesBlock)
	return &Result{Content: []byte(result + "\n"), Filename: "rule.md"}, nil
}

func renderMarkdownRule(meta RuleMeta, body string) (*Result, error) {
	// Generic markdown fallback: canonical format as-is
	canonical, err := buildCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "rule.md"}, nil
}

// renderZedRule renders for Zed, which uses a plain .rules file with no frontmatter.
// Zed does not support conditional activation (globs or alwaysApply toggles), so
// those fields are warned about and dropped.
func renderZedRule(meta RuleMeta, body string) (*Result, error) {
	var warnings []string
	if len(meta.Globs) > 0 {
		warnings = append(warnings, fmt.Sprintf("Zed does not support glob-scoped rules; globs (%s) will be ignored", strings.Join(meta.Globs, ", ")))
	}
	if !meta.AlwaysApply {
		warnings = append(warnings, "Zed does not support conditional activation; rule will be applied unconditionally")
	}

	var buf bytes.Buffer
	if meta.Description != "" {
		buf.WriteString("<!-- ")
		buf.WriteString(meta.Description)
		buf.WriteString(" -->\n\n")
	}
	buf.WriteString(body)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: ".rules", Warnings: warnings}, nil
}

// renderClineRule renders for Cline, which uses markdown files with optional YAML
// frontmatter. The `paths:` field activates the rule conditionally on glob patterns.
func renderClineRule(meta RuleMeta, body string) (*Result, error) {
	filename := "rule.md"
	if meta.Description != "" {
		filename = slugify(meta.Description) + ".md"
	}

	var buf bytes.Buffer

	if len(meta.Globs) > 0 {
		cfm := clineFrontmatter{Paths: meta.Globs}
		fm, err := renderFrontmatter(cfm)
		if err != nil {
			return nil, err
		}
		buf.Write(fm)
		buf.WriteString("\n")
	}

	if meta.Description != "" {
		buf.WriteString("<!-- ")
		buf.WriteString(meta.Description)
		buf.WriteString(" -->\n\n")
	}

	buf.WriteString(body)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: filename}, nil
}

// renderRooCodeRule renders for Roo Code, which uses plain markdown files placed in
// .roo/rules/ (all modes) or .roo/rules-{mode}/ (mode-specific). No frontmatter is
// used. Mode selection is a TUI concern; the converter always targets the default path.
func renderRooCodeRule(meta RuleMeta, body string) (*Result, error) {
	filename := "rule.md"
	if meta.Description != "" {
		filename = slugify(meta.Description) + ".md"
	}

	var warnings []string
	if len(meta.Globs) > 0 {
		warnings = append(warnings, fmt.Sprintf("Roo Code uses mode-based scoping, not glob scoping; globs (%s) will be ignored", strings.Join(meta.Globs, ", ")))
	}

	var buf bytes.Buffer
	if meta.Description != "" {
		buf.WriteString("<!-- ")
		buf.WriteString(meta.Description)
		buf.WriteString(" -->\n\n")
	}
	buf.WriteString(body)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: filename, Warnings: warnings}, nil
}

// renderKiroRule renders a rule as a plain markdown steering file for Kiro.
// Kiro steering files (.kiro/steering/) are plain markdown — no frontmatter.
func renderKiroRule(meta RuleMeta, body string) (*Result, error) {
	var notes []string

	if !meta.AlwaysApply {
		switch {
		case len(meta.Globs) > 0:
			notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
		case meta.Description != "":
			notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
		default:
			notes = append(notes, "**Scope:** Apply only when explicitly asked.")
		}
	}

	content := body
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("syllago", notes)
		content = AppendNotes(body, notesBlock)
	}

	filename := "rule.md"
	if meta.Description != "" {
		filename = slugify(meta.Description) + ".md"
	}
	return &Result{Content: []byte(content + "\n"), Filename: filename}, nil
}

// renderOpenCodeRule renders a rule as plain markdown for OpenCode's AGENTS.md.
// OpenCode does not support frontmatter in AGENTS.md — it is plain markdown.
// Scope information from alwaysApply/globs is embedded as prose if needed.
func renderOpenCodeRule(meta RuleMeta, body string) (*Result, error) {
	if meta.AlwaysApply {
		return &Result{Content: []byte(body + "\n"), Filename: "AGENTS.md"}, nil
	}

	// Embed scope as prose
	var notes []string
	switch {
	case len(meta.Globs) > 0:
		notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
	case meta.Description != "":
		notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
	default:
		notes = append(notes, "**Scope:** Apply only when explicitly asked.")
	}

	notesBlock := BuildConversionNotes("syllago", notes)
	result := AppendNotes(body, notesBlock)
	return &Result{Content: []byte(result + "\n"), Filename: "AGENTS.md"}, nil
}

// slugify converts a description string into a safe filename slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "rule"
	}
	return result
}

// --- Helpers ---

// buildCanonical assembles canonical format from RuleMeta and body.
func buildCanonical(meta RuleMeta, body string) ([]byte, error) {
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

// splitGlobs splits a comma-or-space-separated glob string into a slice.
func splitGlobs(s string) []string {
	var globs []string
	for _, g := range strings.Split(s, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			globs = append(globs, g)
		}
	}
	return globs
}
