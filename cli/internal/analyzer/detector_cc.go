package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/tidwall/gjson"
)

// ClaudeCodeDetector detects Claude Code content.
type ClaudeCodeDetector struct{}

func (d *ClaudeCodeDetector) ProviderSlug() string { return "claude-code" }

func (d *ClaudeCodeDetector) Patterns() []DetectionPattern {
	return []DetectionPattern{
		{Glob: ".claude/agents/*.md", ContentType: catalog.Agents, Confidence: 0.90},
		{Glob: ".claude/skills/*/SKILL.md", ContentType: catalog.Skills, Confidence: 0.90},
		{Glob: ".claude/commands/*.md", ContentType: catalog.Commands, Confidence: 0.90},
		{Glob: ".claude/rules/*.md", ContentType: catalog.Rules, Confidence: 0.90},
		{Glob: ".claude/hooks/*", ContentType: catalog.Hooks, InternalLabel: "hook-script", Confidence: 0.70},
		{Glob: ".claude/settings.json", ContentType: catalog.Hooks, InternalLabel: "hook-wiring", Confidence: 0.90},
		{Glob: ".claude/output-styles/*.md", ContentType: catalog.Rules, Confidence: 0.85},
		{Glob: ".mcp.json", ContentType: catalog.MCP, Confidence: 0.90},
		{Glob: "CLAUDE.md", ContentType: catalog.Rules, Confidence: 0.80},
	}
}

func (d *ClaudeCodeDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
	normalized := filepath.ToSlash(path)

	if normalized == ".claude/settings.json" {
		return classifyCCSettings(path, repoRoot)
	}
	if strings.HasPrefix(normalized, ".claude/hooks/") {
		return classifyCCHookScript(path, repoRoot)
	}
	if normalized == ".mcp.json" {
		return classifyCCMCP(path, repoRoot)
	}
	return classifyCCContent(path, repoRoot)
}

// classifyCCSettings parses .claude/settings.json and extracts hook entries.
func classifyCCSettings(path, repoRoot string) ([]*DetectedItem, error) {
	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitJSON)
	if err != nil {
		return nil, nil
	}

	// Validate it's at least valid-ish JSON.
	if !json.Valid(data) {
		return nil, nil
	}

	hooks := gjson.GetBytes(data, "hooks")
	if !hooks.Exists() || !hooks.IsObject() {
		return nil, nil
	}

	var items []*DetectedItem
	hooks.ForEach(func(eventKey, eventArray gjson.Result) bool {
		event := eventKey.String()
		canonEvent := converter.ReverseTranslateHookEvent(event, "claude-code")

		if !eventArray.IsArray() {
			return true
		}

		for i, hookEntry := range eventArray.Array() {
			command := hookEntry.Get("command").String()
			scriptPath := resolveHookScript(command, repoRoot)

			name := fmt.Sprintf("%s:%d", canonEvent, i)
			if scriptPath != "" {
				name = strings.TrimSuffix(filepath.Base(scriptPath), filepath.Ext(scriptPath))
			}

			item := &DetectedItem{
				Name:         name,
				Type:         catalog.Hooks,
				Provider:     "claude-code",
				Path:         path,
				Confidence:   0.90,
				HookEvent:    canonEvent,
				HookIndex:    i,
				ConfigSource: ".claude/settings.json",
			}

			if scriptPath != "" {
				item.Scripts = []string{scriptPath}
				item.Path = scriptPath
			}

			items = append(items, item)
		}
		return true
	})

	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

// resolveHookScript extracts a script path from a hook command string.
// Returns the relative path if the file exists, empty string for inline commands.
func resolveHookScript(command, repoRoot string) string {
	scriptExts := map[string]bool{
		".sh": true, ".py": true, ".js": true, ".ts": true, ".rb": true, ".bash": true,
	}

	tokens := strings.Fields(command)
	for _, token := range tokens {
		if strings.Contains(token, "/") || strings.Contains(token, "\\") {
			// Token looks like a path.
			abs := filepath.Join(repoRoot, token)
			if _, err := os.Stat(abs); err == nil {
				return token
			}
		}
		ext := filepath.Ext(token)
		if scriptExts[ext] {
			abs := filepath.Join(repoRoot, token)
			if _, err := os.Stat(abs); err == nil {
				return token
			}
		}
	}
	return ""
}

// classifyCCHookScript classifies a standalone hook script file.
func classifyCCHookScript(path, repoRoot string) ([]*DetectedItem, error) {
	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return []*DetectedItem{{
		Name:          name,
		Type:          catalog.Hooks,
		InternalLabel: "hook-script",
		Provider:      "claude-code",
		Path:          path,
		ContentHash:   hashBytes(data),
		Confidence:    0.70,
	}}, nil
}

// classifyCCMCP parses .mcp.json and returns one item per server entry.
func classifyCCMCP(path, repoRoot string) ([]*DetectedItem, error) {
	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitJSON)
	if err != nil {
		return nil, nil
	}

	servers := gjson.GetBytes(data, "mcpServers")
	if !servers.Exists() || !servers.IsObject() {
		return nil, nil
	}

	var items []*DetectedItem
	servers.ForEach(func(key, value gjson.Result) bool {
		item := &DetectedItem{
			Name:        key.String(),
			Type:        catalog.MCP,
			Provider:    "claude-code",
			Path:        path,
			Confidence:  0.90,
			Description: catalog.MCPServerDescription(value),
		}
		items = append(items, item)
		return true
	})

	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

// classifyCCContent handles standard CC content files (agents, skills, etc.).
func classifyCCContent(path, repoRoot string) ([]*DetectedItem, error) {
	absPath := filepath.Join(repoRoot, path)
	data, err := readFileLimited(absPath, limitMarkdown)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	normalized := filepath.ToSlash(path)
	ct, confidence, label := ccMatchPattern(normalized)
	if ct == "" {
		return nil, nil
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Skills use directory name.
	if ct == catalog.Skills {
		parts := strings.Split(normalized, "/")
		if len(parts) >= 4 {
			name = parts[2] // .claude/skills/<name>/SKILL.md
		}
	}

	item := &DetectedItem{
		Name:          name,
		Type:          ct,
		InternalLabel: label,
		Provider:      "claude-code",
		Path:          path,
		ContentHash:   hashBytes(data),
		Confidence:    confidence,
	}

	if ct == catalog.Agents || ct == catalog.Skills {
		if fm := parseFrontmatterBasic(data); fm != nil {
			item.DisplayName = fm.name
			item.Description = fm.description
		}
	}

	return []*DetectedItem{item}, nil
}

// ccMatchPattern matches a path against CC detector patterns.
func ccMatchPattern(normalized string) (catalog.ContentType, float64, string) {
	d := &ClaudeCodeDetector{}
	for _, pat := range d.Patterns() {
		// Skip settings.json and .mcp.json — handled by dedicated functions.
		if pat.InternalLabel == "hook-wiring" || pat.ContentType == catalog.MCP {
			continue
		}
		ok, _ := filepath.Match(pat.Glob, normalized)
		if ok {
			return pat.ContentType, pat.Confidence, pat.InternalLabel
		}
	}
	return "", 0, ""
}
