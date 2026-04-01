package analyzer

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TopLevelDetector detects provider-agnostic content in top-level directories.
type TopLevelDetector struct{}

func (d *TopLevelDetector) ProviderSlug() string { return "top-level" }

func (d *TopLevelDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: "agents/*.md", ContentType: catalog.Agents, Confidence: 0.85},
		{Glob: "agents/*/*.md", ContentType: catalog.Agents, Confidence: 0.80},
		{Glob: "commands/*.md", ContentType: catalog.Commands, Confidence: 0.85},
		{Glob: "commands/*/*.md", ContentType: catalog.Commands, Confidence: 0.80},
		{Glob: "rules/*.md", ContentType: catalog.Rules, Confidence: 0.80},
		{Glob: "rules/*.mdc", ContentType: catalog.Rules, Confidence: 0.80},
		{Glob: "hooks/*.py", ContentType: catalog.Hooks, InternalLabel: "hook-script", Confidence: 0.60},
		{Glob: "hooks/*.js", ContentType: catalog.Hooks, InternalLabel: "hook-script", Confidence: 0.60},
		{Glob: "hooks/*.ts", ContentType: catalog.Hooks, InternalLabel: "hook-script", Confidence: 0.60},
		{Glob: "hooks/*.sh", ContentType: catalog.Hooks, InternalLabel: "hook-script", Confidence: 0.60},
		{Glob: "hooks/hooks.json", ContentType: catalog.Hooks, InternalLabel: "hook-wiring", Confidence: 0.85},
		{Glob: "hook-scripts/*/*.js", ContentType: catalog.Hooks, InternalLabel: "hook-script", Confidence: 0.70},
		// B2 RESOLVED — prompts/*.md maps to catalog.Rules (not a separate Prompts type).
		{Glob: "prompts/*.md", ContentType: catalog.Rules, Confidence: 0.75},
	}
}

func (d *TopLevelDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	// Hook-wiring files are consumed during hook correlation, not classified as items.
	normalized := filepath.ToSlash(path)
	if normalized == "hooks/hooks.json" {
		return nil, nil
	}

	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	// Find matching pattern to get type, confidence, and label.
	ct, confidence, label := d.matchPattern(path)
	if ct == "" {
		return nil, nil
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	item := &DetectedItem{
		Name:          name,
		Type:          ct,
		Provider:      "top-level",
		Path:          path,
		ContentHash:   hashBytes(data),
		Confidence:    confidence,
		InternalLabel: label,
	}

	// Extract frontmatter for agents.
	if ct == catalog.Agents {
		if fm := parseFrontmatterBasic(data); fm != nil {
			item.DisplayName = fm.name
			item.Description = fm.description
		}
	}

	return []*DetectedItem{item}, nil
}

// matchPattern returns the content type, confidence, and internal label for a path.
func (d *TopLevelDetector) matchPattern(path string) (catalog.ContentType, float64, string) {
	normalized := filepath.ToSlash(path)
	for _, pat := range d.Patterns() {
		ok, _ := filepath.Match(pat.Glob, normalized)
		if ok {
			return pat.ContentType, pat.Confidence, pat.InternalLabel
		}
	}
	return "", 0, ""
}
