package analyzer

import (
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// SyllagoDetector detects content organized in syllago's canonical layout.
type SyllagoDetector struct{}

func (d *SyllagoDetector) ProviderSlug() string { return "syllago" }

func (d *SyllagoDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: "skills/*/SKILL.md", ContentType: catalog.Skills, Confidence: 0.95},
		{Glob: "agents/*/AGENT.md", ContentType: catalog.Agents, Confidence: 0.95},
		{Glob: "hooks/*/*/hook.json", ContentType: catalog.Hooks, Confidence: 0.95},
		{Glob: "mcp/*/config.json", ContentType: catalog.MCP, Confidence: 0.95},
		{Glob: "rules/*/*/rule.md", ContentType: catalog.Rules, Confidence: 0.95},
		{Glob: "commands/*/*/command.md", ContentType: catalog.Commands, Confidence: 0.95},
		{Glob: "loadouts/*/loadout.yaml", ContentType: catalog.Loadouts, Confidence: 0.95},
	}
}

func (d *SyllagoDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil {
		return nil, nil // skip unreadable or oversized files
	}

	name := nameFromPath(path)
	if name == "" {
		return nil, nil
	}

	ct := ctFromSyllagoPath(path)
	if ct == "" {
		return nil, nil
	}

	item := &DetectedItem{
		Name:        name,
		Type:        ct,
		Provider:    "syllago",
		Path:        filepath.Dir(path),
		ContentHash: hashBytes(data),
		Confidence:  0.95,
	}

	// Extract display name and description from frontmatter for skills/agents.
	switch ct {
	case catalog.Skills, catalog.Agents:
		if fm := parseFrontmatterBasic(data); fm != nil {
			item.DisplayName = fm.name
			item.Description = fm.description
		}
	}

	return []*DetectedItem{item}, nil
}

// nameFromPath extracts the item name from a syllago canonical path.
// "skills/my-skill/SKILL.md" → "my-skill"
// "hooks/claude-code/lint/hook.json" → "lint"
func nameFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	switch len(parts) {
	case 3: // skills/name/SKILL.md, agents/name/AGENT.md, etc.
		return parts[1]
	case 4: // hooks/provider/name/hook.json, rules/provider/name/rule.md
		return parts[2]
	}
	return ""
}

// ctFromSyllagoPath maps the first path segment to a ContentType.
func ctFromSyllagoPath(path string) catalog.ContentType {
	first := strings.Split(filepath.ToSlash(path), "/")[0]
	switch first {
	case "skills":
		return catalog.Skills
	case "agents":
		return catalog.Agents
	case "hooks":
		return catalog.Hooks
	case "mcp":
		return catalog.MCP
	case "rules":
		return catalog.Rules
	case "commands":
		return catalog.Commands
	case "loadouts":
		return catalog.Loadouts
	}
	return ""
}
