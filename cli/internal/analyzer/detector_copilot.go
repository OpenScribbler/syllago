package analyzer

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// CopilotDetector detects VS Code Copilot content.
type CopilotDetector struct{}

func (d *CopilotDetector) ProviderSlug() string { return "vs-code-copilot" }

func (d *CopilotDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: ".github/copilot-instructions.md", ContentType: catalog.Rules, Confidence: 0.95},
		{Glob: ".github/instructions/*.instructions.md", ContentType: catalog.Rules, Confidence: 0.90},
		{Glob: ".github/agents/*.md", ContentType: catalog.Agents, Confidence: 0.90},
	}
}

func (d *CopilotDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	// Determine type and confidence based on path.
	var ct catalog.ContentType
	var confidence float64
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	normalized := filepath.ToSlash(path)
	switch {
	case strings.HasPrefix(normalized, ".github/agents/"):
		ct = catalog.Agents
		confidence = 0.90
	case strings.HasPrefix(normalized, ".github/instructions/"):
		ct = catalog.Rules
		confidence = 0.90
	default:
		ct = catalog.Rules
		confidence = 0.95
		name = "copilot-instructions"
	}

	item := &DetectedItem{
		Name:        name,
		Type:        ct,
		Provider:    "vs-code-copilot",
		Path:        path,
		ContentHash: hashBytes(data),
		Confidence:  confidence,
	}

	// Extract frontmatter for agent files.
	if ct == catalog.Agents {
		if fm := parseFrontmatterBasic(data); fm != nil {
			item.DisplayName = fm.name
			item.Description = fm.description
		}
	}

	return []*DetectedItem{item}, nil
}
