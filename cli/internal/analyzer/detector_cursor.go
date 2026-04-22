package analyzer

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// CursorDetector detects Cursor content.
type CursorDetector struct{}

func (d *CursorDetector) ProviderSlug() string { return "cursor" }

func (d *CursorDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: ".cursorrules", ContentType: catalog.Rules, Confidence: 0.95},
		{Glob: ".cursor/rules/*.mdc", ContentType: catalog.Rules, Confidence: 0.90},
		{Glob: ".cursor/rules/*.md", ContentType: catalog.Rules, Confidence: 0.85},
		{Glob: ".cursor/agents/*.md", ContentType: catalog.Agents, Confidence: 0.90},
		{Glob: ".cursor/skills/*/SKILL.md", ContentType: catalog.Skills, Confidence: 0.90},
		{Glob: ".cursor/hooks.json", ContentType: catalog.Hooks, InternalLabel: "hook-wiring", Confidence: 0.90},
		{Glob: ".cursor/hooks/*", ContentType: catalog.Hooks, InternalLabel: "hook-script", Confidence: 0.70},
	}
}

func (d *CursorDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	normalized := filepath.ToSlash(path)

	// Hook-wiring files are consumed during hook correlation, not classified as items.
	if normalized == ".cursor/hooks.json" {
		return nil, nil
	}

	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	ct, confidence, label := d.matchPattern(path)
	if ct == "" {
		return nil, nil
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Skills use directory name, not filename.
	if ct == catalog.Skills {
		parts := strings.Split(normalized, "/")
		if len(parts) >= 4 {
			name = parts[2] // .cursor/skills/<name>/SKILL.md
		}
	}

	// .cursorrules uses the filename as-is.
	if normalized == ".cursorrules" {
		name = ".cursorrules"
	}

	item := &DetectedItem{
		Name:          name,
		Type:          ct,
		Provider:      "cursor",
		Path:          path,
		ContentHash:   hashBytes(data),
		Confidence:    confidence,
		InternalLabel: label,
	}

	// Extract frontmatter for agents and skills.
	if ct == catalog.Agents || ct == catalog.Skills {
		if fm := parseFrontmatterBasic(data); fm != nil {
			item.DisplayName = fm.name
			item.Description = fm.description
		}
	}

	return []*DetectedItem{item}, nil
}

func (d *CursorDetector) matchPattern(path string) (catalog.ContentType, float64, string) {
	normalized := filepath.ToSlash(path)
	for _, pat := range d.Patterns() {
		ok, _ := filepath.Match(pat.Glob, normalized)
		if ok {
			return pat.ContentType, pat.Confidence, pat.InternalLabel
		}
	}
	return "", 0, ""
}
