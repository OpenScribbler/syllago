package analyzer

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// classifySimple is the shared Classify logic for single-file detectors.
// It reads the file, checks for non-empty content, and returns a DetectedItem.
func classifySimple(path, repoRoot, provider string, ct catalog.ContentType, confidence float64) ([]*DetectedItem, error) {
	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	name := filepath.Base(path)
	if strings.Contains(path, "/") {
		// Use the filename without extension for directory-based items.
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return []*DetectedItem{{
		Name:        name,
		Type:        ct,
		Provider:    provider,
		Path:        path,
		ContentHash: hashBytes(data),
		Confidence:  confidence,
	}}, nil
}

// --- Windsurf ---

// WindsurfDetector detects Windsurf content.
type WindsurfDetector struct{}

func (d *WindsurfDetector) ProviderSlug() string { return "windsurf" }

func (d *WindsurfDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: ".windsurfrules", ContentType: catalog.Rules, Confidence: 0.95},
	}
}

func (d *WindsurfDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	return classifySimple(path, repoRoot, "windsurf", catalog.Rules, 0.95)
}

// --- Cline ---

// ClineDetector detects Cline content.
type ClineDetector struct{}

func (d *ClineDetector) ProviderSlug() string { return "cline" }

func (d *ClineDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: ".clinerules", ContentType: catalog.Rules, Confidence: 0.95},
		{Glob: ".clinerules/*.md", ContentType: catalog.Rules, Confidence: 0.90},
	}
}

func (d *ClineDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	confidence := 0.95
	if strings.Contains(path, "/") {
		confidence = 0.90
	}
	return classifySimple(path, repoRoot, "cline", catalog.Rules, confidence)
}

// --- Roo Code ---

// RooCodeDetector detects Roo Code content.
type RooCodeDetector struct{}

func (d *RooCodeDetector) ProviderSlug() string { return "roo-code" }

func (d *RooCodeDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: ".roo/rules/*.md", ContentType: catalog.Rules, Confidence: 0.90},
		{Glob: ".roomodes", ContentType: catalog.Rules, Confidence: 0.85},
	}
}

func (d *RooCodeDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	confidence := 0.90
	if filepath.Base(path) == ".roomodes" {
		confidence = 0.85
	}
	return classifySimple(path, repoRoot, "roo-code", catalog.Rules, confidence)
}

// --- Codex ---

// CodexDetector detects OpenAI Codex content.
type CodexDetector struct{}

func (d *CodexDetector) ProviderSlug() string { return "codex" }

func (d *CodexDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: "AGENTS.md", ContentType: catalog.Rules, Confidence: 0.85},
		{Glob: ".codex/agents/*.toml", ContentType: catalog.Agents, Confidence: 0.85},
	}
}

func (d *CodexDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	if strings.HasSuffix(path, ".toml") {
		return classifySimple(path, repoRoot, "codex", catalog.Agents, 0.85)
	}
	// AGENTS.md: default to Rules
	return classifySimple(path, repoRoot, "codex", catalog.Rules, 0.85)
}

// --- Gemini ---

// GeminiDetector detects Gemini CLI content.
type GeminiDetector struct{}

func (d *GeminiDetector) ProviderSlug() string { return "gemini-cli" }

func (d *GeminiDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: "GEMINI.md", ContentType: catalog.Rules, Confidence: 0.85},
		{Glob: ".gemini/skills/*/SKILL.md", ContentType: catalog.Skills, Confidence: 0.85},
	}
}

func (d *GeminiDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	if strings.HasSuffix(path, "SKILL.md") {
		absPath := filepath.Join(repoRoot, path)
		data, err := readFileLimited(absPath, limitMarkdown)
		if err != nil || len(bytes.TrimSpace(data)) == 0 {
			return nil, nil
		}

		// Extract name from directory: .gemini/skills/<name>/SKILL.md
		parts := strings.Split(filepath.ToSlash(path), "/")
		name := "unknown"
		if len(parts) >= 4 {
			name = parts[2]
		}

		item := &DetectedItem{
			Name:        name,
			Type:        catalog.Skills,
			Provider:    "gemini-cli",
			Path:        filepath.Dir(path),
			ContentHash: hashBytes(data),
			Confidence:  0.85,
		}
		if fm := parseFrontmatterBasic(data); fm != nil {
			item.DisplayName = fm.name
			item.Description = fm.description
		}
		return []*DetectedItem{item}, nil
	}

	// GEMINI.md → Rules
	return classifySimple(path, repoRoot, "gemini-cli", catalog.Rules, 0.85)
}
