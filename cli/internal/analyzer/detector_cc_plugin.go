package analyzer

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// ClaudeCodePluginDetector detects Claude Code Plugin content.
type ClaudeCodePluginDetector struct{}

func (d *ClaudeCodePluginDetector) ProviderSlug() string { return "claude-code-plugin" }

func (d *ClaudeCodePluginDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: ".claude-plugin/plugin.json", ContentType: catalog.Skills, InternalLabel: "plugin-manifest", Confidence: 0.90},
		{Glob: "plugins/*/agents/*.md", ContentType: catalog.Agents, Confidence: 0.90},
		{Glob: "plugins/*/skills/*/SKILL.md", ContentType: catalog.Skills, Confidence: 0.90},
		{Glob: "plugins/*/hooks/hooks.json", ContentType: catalog.Hooks, InternalLabel: "hook-wiring", Confidence: 0.90},
		{Glob: "plugins/*/commands/*.md", ContentType: catalog.Commands, Confidence: 0.90},
	}
}

func (d *ClaudeCodePluginDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	normalized := filepath.ToSlash(path)

	// plugin.json is a preprocessing manifest, not a directly classifiable item.
	if strings.HasSuffix(normalized, "plugin.json") {
		return nil, nil
	}
	// Hook-wiring files are consumed during hook correlation.
	if strings.HasSuffix(normalized, "hooks/hooks.json") {
		return nil, nil
	}

	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	ct, confidence, _ := d.matchPattern(normalized)
	if ct == "" {
		return nil, nil
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Skills use directory name.
	if ct == catalog.Skills {
		parts := strings.Split(normalized, "/")
		if len(parts) >= 5 {
			name = parts[3] // plugins/<plugin>/skills/<name>/SKILL.md
		}
	}

	item := &DetectedItem{
		Name:        name,
		Type:        ct,
		Provider:    "claude-code-plugin",
		Path:        path,
		ContentHash: hashBytes(data),
		Confidence:  confidence,
	}

	if ct == catalog.Agents || ct == catalog.Skills {
		if fm := parseFrontmatterBasic(data); fm != nil {
			item.DisplayName = fm.name
			item.Description = fm.description
		}
	}

	return []*DetectedItem{item}, nil
}

func (d *ClaudeCodePluginDetector) matchPattern(normalized string) (catalog.ContentType, float64, string) {
	for _, pat := range d.Patterns() {
		if pat.InternalLabel == "plugin-manifest" || pat.InternalLabel == "hook-wiring" {
			continue
		}
		ok, _ := filepath.Match(pat.Glob, normalized)
		if ok {
			return pat.ContentType, pat.Confidence, pat.InternalLabel
		}
	}
	return "", 0, ""
}
