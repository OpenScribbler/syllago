package converter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"gopkg.in/yaml.v3"
)

// hookConfigHints maps provider slugs to the file path where hook configuration
// should be added. Only includes providers that support hooks.
var hookConfigHints = map[string]string{
	"gemini-cli":  ".gemini/settings.json hooks section",
	"cursor":      ".cursor/settings.json hooks section",
	"windsurf":    ".windsurf/hooks.json",
	"copilot-cli": ".github/hooks/ directory",
	"kiro":        ".kiro/ hooks agent file",
	"codex":       ".codex/hooks.json",
}

// hookScopingNotes describes scoping limitations per provider.
// Skill-scoped hooks are a Claude Code feature — other providers run hooks globally.
var hookScopingNotes = map[string]string{
	"gemini-cli":  "Gemini hooks are global (skill scoping will be lost)",
	"cursor":      "Cursor hooks are global (skill scoping will be lost)",
	"windsurf":    "Windsurf hooks are global (skill scoping will be lost)",
	"copilot-cli": "Copilot hooks are global (skill scoping will be lost)",
	"kiro":        "Kiro hooks are global (skill scoping will be lost)",
	"codex":       "Codex hooks are global (skill scoping will be lost)",
}

// formatSkillHookWarnings extracts hook details from SkillMeta.Hooks and generates
// actionable warnings for providers that support hooks. Returns nil if hooks is nil
// or cannot be interpreted.
func formatSkillHookWarnings(skillName string, hooks any, targetSlug string) []string {
	if hooks == nil {
		return nil
	}

	configHint, hasHooks := hookConfigHints[targetSlug]
	if !hasHooks {
		return nil // hookless provider — caller handles prose embedding
	}

	// SkillMeta.Hooks comes from YAML unmarshal into any.
	// Expected shape: map[string]interface{} with event names as keys,
	// each mapping to a list of hook entries (maps with command/matcher/timeout).
	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		// Fallback: hooks is present but not a map (unusual). Generate a generic warning.
		return []string{
			fmt.Sprintf("skill %q has hooks that require separate configuration in %s", skillName, configHint),
		}
	}

	label := skillName
	if label == "" {
		label = "(unnamed skill)"
	}

	var warnings []string
	warnings = append(warnings, fmt.Sprintf("skill %q has hooks requiring separate configuration:", label))

	for event, matchersRaw := range hooksMap {
		matchers, ok := matchersRaw.([]interface{})
		if !ok {
			warnings = append(warnings, fmt.Sprintf("  Event: %s (could not parse hook entries)", event))
			continue
		}

		for _, entryRaw := range matchers {
			entry, ok := entryRaw.(map[string]interface{})
			if !ok {
				continue
			}

			command, _ := entry["command"].(string)
			matcher, _ := entry["matcher"].(string)

			detail := fmt.Sprintf("  Event: %s", event)
			if matcher != "" {
				detail += fmt.Sprintf(", Matcher: %s", matcher)
			}
			if command != "" {
				detail += fmt.Sprintf(", Command: %s", command)
			}
			warnings = append(warnings, detail)
		}
	}

	warnings = append(warnings, fmt.Sprintf("  -> Add to %s", configHint))
	if note, ok := hookScopingNotes[targetSlug]; ok {
		warnings = append(warnings, fmt.Sprintf("  -> %s", note))
	}

	return warnings
}

func init() {
	Register(&SkillsConverter{})
}

// flexStringList is a []string that also accepts a single YAML scalar string.
// When unmarshaled from a scalar, the string is split on commas (if present)
// or whitespace, producing individual tool names.
type flexStringList []string

func (f *flexStringList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		// Standard YAML list: ["Read", "Grep", "Glob"]
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*f = list
		return nil
	case yaml.ScalarNode:
		// Single scalar string — could be comma-separated, space-delimited, or single tool
		*f = splitToolString(value.Value)
		return nil
	default:
		return fmt.Errorf("expected string or list, got YAML kind %d", value.Kind)
	}
}

// splitToolString splits a string on commas (if present) or whitespace.
func splitToolString(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		var result []string
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				result = append(result, t)
			}
		}
		return result
	}
	return strings.Fields(s)
}

// SkillMeta is the canonical skill metadata (YAML frontmatter fields).
// Claude Code is the superset.
type SkillMeta struct {
	Name                   string         `yaml:"name,omitempty"`
	Description            string         `yaml:"description,omitempty"`
	AllowedTools           flexStringList `yaml:"allowed-tools,omitempty"`
	DisallowedTools        flexStringList `yaml:"disallowed-tools,omitempty"`
	Context                string         `yaml:"context,omitempty"`
	Agent                  string         `yaml:"agent,omitempty"`
	Model                  string         `yaml:"model,omitempty"`
	Effort                 string         `yaml:"effort,omitempty"`
	DisableModelInvocation bool           `yaml:"disable-model-invocation,omitempty"`
	UserInvocable          *bool          `yaml:"user-invocable,omitempty"`
	ArgumentHint           string         `yaml:"argument-hint,omitempty"`
	Hooks                  any            `yaml:"hooks,omitempty"`
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
	switch sourceProvider {
	case "kiro", "opencode":
		return canonicalizeSkillFromMarkdown(content)
	default:
		// Claude Code, Gemini CLI, Copilot CLI, Cursor — YAML frontmatter + markdown
		// Cursor SKILL.md uses the same frontmatter format as Claude Code (subset of fields),
		// so it parses identically through the canonical path.
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
}

func (c *SkillsConverter) Render(content []byte, target provider.Provider) (*Result, error) {
	meta, body, err := parseSkillCanonical(content)
	if err != nil {
		return nil, fmt.Errorf("parsing canonical skill: %w", err)
	}

	switch target.Slug {
	case "gemini-cli":
		return renderGeminiSkill(meta, body)
	case "opencode":
		return renderOpenCodeSkill(meta, body)
	case "kiro":
		return renderKiroSkill(meta, body)
	case "cursor":
		return renderCursorSkill(meta, body)
	case "windsurf":
		return renderWindsurfSkill(meta, body)
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
	if meta.Effort != "" {
		notes = append(notes, fmt.Sprintf("Effort level: %s.", meta.Effort))
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
	// Hooks: generate actionable warnings instead of prose (Gemini supports hooks)
	hookWarnings := formatSkillHookWarnings(meta.Name, meta.Hooks, "gemini-cli")

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

	return &Result{Content: buf.Bytes(), Filename: "SKILL.md", Warnings: hookWarnings}, nil
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

// cursorSkillMeta is the subset of fields Cursor supports in SKILL.md frontmatter.
// Cursor supports: name, description, license, compatibility, metadata, disable-model-invocation.
// It does NOT support: allowed-tools, context, agent, model, effort, hooks, user-invocable, argument-hint.
type cursorSkillMeta struct {
	Name                   string `yaml:"name,omitempty"`
	Description            string `yaml:"description,omitempty"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation,omitempty"`
}

// renderCursorSkill renders a canonical skill to Cursor's SKILL.md format.
// Cursor uses the same SKILL.md shape as Claude Code but supports fewer frontmatter fields.
// Unsupported fields (allowed-tools, context, agent, model, etc.) are embedded as prose notes.
func renderCursorSkill(meta SkillMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	// Build behavioral embedding notes for fields Cursor doesn't support
	var notes []string
	if len(meta.AllowedTools) > 0 {
		translated := TranslateTools(meta.AllowedTools, "cursor")
		notes = append(notes, fmt.Sprintf("**Tool restriction:** Use only %s tools.", strings.Join(translated, ", ")))
	}
	if len(meta.DisallowedTools) > 0 {
		translated := TranslateTools(meta.DisallowedTools, "cursor")
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
	if meta.Effort != "" {
		notes = append(notes, fmt.Sprintf("Effort level: %s.", meta.Effort))
	}
	if meta.UserInvocable != nil && *meta.UserInvocable {
		notes = append(notes, "Intended to appear in the command menu.")
	}
	if meta.ArgumentHint != "" {
		notes = append(notes, fmt.Sprintf("Usage: %s", meta.ArgumentHint))
	}
	// Hooks: generate actionable warnings instead of prose (Cursor supports hooks)
	hookWarnings := formatSkillHookWarnings(meta.Name, meta.Hooks, "cursor")

	outBody := cleanBody
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		outBody = AppendNotes(outBody, notesBlock)
	}

	cm := cursorSkillMeta{
		Name:                   meta.Name,
		Description:            meta.Description,
		DisableModelInvocation: meta.DisableModelInvocation,
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

	return &Result{Content: buf.Bytes(), Filename: "SKILL.md", Warnings: hookWarnings}, nil
}

// windsurfSkillMeta is the subset of fields Windsurf supports in SKILL.md frontmatter.
// Windsurf only supports name and description — same as Gemini CLI.
type windsurfSkillMeta struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// renderWindsurfSkill renders a canonical skill to Windsurf's SKILL.md format.
// Windsurf uses the Agent Skills standard (SKILL.md with YAML frontmatter) but only
// supports name and description fields. Unsupported fields are embedded as prose notes.
func renderWindsurfSkill(meta SkillMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	// Build behavioral embedding notes for fields Windsurf doesn't support
	var notes []string
	if len(meta.AllowedTools) > 0 {
		translated := TranslateTools(meta.AllowedTools, "windsurf")
		notes = append(notes, fmt.Sprintf("**Tool restriction:** Use only %s tools.", strings.Join(translated, ", ")))
	}
	if len(meta.DisallowedTools) > 0 {
		translated := TranslateTools(meta.DisallowedTools, "windsurf")
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
	if meta.Effort != "" {
		notes = append(notes, fmt.Sprintf("Effort level: %s.", meta.Effort))
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
	// Hooks: generate actionable warnings instead of prose (Windsurf supports hooks)
	hookWarnings := formatSkillHookWarnings(meta.Name, meta.Hooks, "windsurf")

	outBody := cleanBody
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		outBody = AppendNotes(outBody, notesBlock)
	}

	wm := windsurfSkillMeta{
		Name:        meta.Name,
		Description: meta.Description,
	}
	fm, err := renderFrontmatter(wm)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(outBody)
	buf.WriteString("\n")

	return &Result{Content: buf.Bytes(), Filename: "SKILL.md", Warnings: hookWarnings}, nil
}

// canonicalizeSkillFromMarkdown wraps plain markdown content in minimal canonical skill format.
// Used for providers whose skills are plain markdown without frontmatter (Kiro, OpenCode).
func canonicalizeSkillFromMarkdown(content []byte) (*Result, error) {
	body := strings.TrimSpace(string(content))
	meta := SkillMeta{}
	canonical, err := buildSkillCanonical(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: canonical, Filename: "SKILL.md"}, nil
}

// buildSkillProseNotes generates behavioral embedding notes for skill metadata
// that can't be represented as structured fields. Uses canonical tool names
// (no provider-specific translation) since the target is plain markdown.
func buildSkillProseNotes(meta SkillMeta) []string {
	var notes []string
	if len(meta.AllowedTools) > 0 {
		notes = append(notes, fmt.Sprintf("**Tool restriction:** Use only %s tools.", strings.Join(meta.AllowedTools, ", ")))
	}
	if len(meta.DisallowedTools) > 0 {
		notes = append(notes, fmt.Sprintf("**Do not use:** %s tools.", strings.Join(meta.DisallowedTools, ", ")))
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
	if meta.DisableModelInvocation {
		notes = append(notes, "Only invoke when the user explicitly requests it.")
	}
	if meta.UserInvocable != nil && *meta.UserInvocable {
		notes = append(notes, "Intended to appear in the command menu.")
	}
	if meta.ArgumentHint != "" {
		notes = append(notes, fmt.Sprintf("Usage: %s", meta.ArgumentHint))
	}
	if meta.Hooks != nil {
		notes = append(notes, "**Hooks:** This skill defines lifecycle hooks that execute shell commands. Hooks require a provider with skill-scoped hook support (currently only Claude Code).")
	}
	return notes
}

// renderPlainMarkdownSkill renders a canonical skill to a plain markdown file
// with no frontmatter. Used by Kiro and OpenCode where skill files are simple markdown.
// Metadata that can't be represented structurally is embedded as prose.
func renderPlainMarkdownSkill(meta SkillMeta, body string) (*Result, error) {
	cleanBody := StripConversionNotes(body)

	var header strings.Builder
	if meta.Name != "" {
		header.WriteString("# ")
		header.WriteString(meta.Name)
		header.WriteString("\n\n")
	}
	if meta.Description != "" {
		header.WriteString(meta.Description)
		header.WriteString("\n\n")
	}

	outBody := header.String() + cleanBody

	// Embed behavioral metadata as prose rather than dropping it
	notes := buildSkillProseNotes(meta)
	if len(notes) > 0 {
		notesBlock := BuildConversionNotes("claude-code", notes)
		outBody = AppendNotes(outBody, notesBlock)
	}

	name := "skill"
	if meta.Name != "" {
		name = slugify(meta.Name)
	}

	return &Result{
		Content:  []byte(outBody + "\n"),
		Filename: name + ".md",
	}, nil
}

// renderKiroSkill renders a canonical skill to a Kiro steering file (plain markdown).
// Kiro supports hooks, so skill hooks generate actionable warnings instead of only prose.
func renderKiroSkill(meta SkillMeta, body string) (*Result, error) {
	result, err := renderPlainMarkdownSkill(meta, body)
	if err != nil {
		return nil, err
	}
	result.Warnings = formatSkillHookWarnings(meta.Name, meta.Hooks, "kiro")
	return result, nil
}

// renderOpenCodeSkill renders a canonical skill to OpenCode's plain markdown format.
func renderOpenCodeSkill(meta SkillMeta, body string) (*Result, error) {
	return renderPlainMarkdownSkill(meta, body)
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
