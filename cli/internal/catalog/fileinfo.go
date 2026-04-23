package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
)

// PrimaryFileName returns the best default file to display for a content item.
// Skills: SKILL.md; Hooks: first .json/.yaml/.yml; MCP: first .json;
// Agents/Rules: first .md; Commands: first file; Loadouts: loadout.yaml or loadout.yml.
// Returns empty string if no match found.
func PrimaryFileName(files []string, ct ContentType) string {
	for _, f := range files {
		// Use only the base name for matching (files may be relative paths).
		name := strings.ToLower(filepath.Base(f))
		switch ct {
		case Skills:
			if name == "skill.md" {
				return f
			}
		case Agents:
			if strings.HasSuffix(name, ".md") {
				return f
			}
		case Rules:
			if strings.HasSuffix(name, ".md") {
				return f
			}
		case Hooks:
			if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				return f
			}
		case MCP:
			if strings.HasSuffix(name, ".json") {
				return f
			}
		case Commands:
			// First file is the primary entry point.
			return f
		case Loadouts:
			if name == "loadout.yaml" || name == "loadout.yml" {
				return f
			}
		}
	}
	return ""
}

// ReadFileContent reads the file at filepath.Join(itemPath, relPath), capping output
// at maxLines lines. If the file exceeds maxLines, the returned string is truncated and
// a "(N more lines)" suffix is appended.
func ReadFileContent(itemPath, relPath string, maxLines int) (string, error) {
	absPath := filepath.Clean(filepath.Join(itemPath, relPath))
	cleanBase := filepath.Clean(itemPath) + string(filepath.Separator)
	if !strings.HasPrefix(absPath, cleanBase) && absPath != filepath.Clean(itemPath) {
		return "", fmt.Errorf("path traversal: %s escapes %s", relPath, itemPath)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", relPath, err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		extra := len(lines) - maxLines
		content = strings.Join(lines[:maxLines], "\n")
		content += fmt.Sprintf("\n\n(%d more lines)", extra)
	}

	return content, nil
}

// RemoveLibraryItem removes a content item's directory from disk.
// Both TUI and CLI commands should use this rather than os.RemoveAll directly.
func RemoveLibraryItem(itemPath string) error {
	return os.RemoveAll(itemPath)
}

// HookSummary returns a one-line summary of a hook item's configuration
// (event, matcher, handler type). Reads the canonical hooks/0.1 Manifest
// shape: top-level hooks[0].{event, matcher, handler.type}. Returns empty
// string on read errors.
func HookSummary(item ContentItem) string {
	hookPath := filepath.Join(item.Path, "hook.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return ""
	}
	event := gjson.GetBytes(data, "hooks.0.event").String()
	matcher := gjson.GetBytes(data, "hooks.0.matcher").String()
	hookType := gjson.GetBytes(data, "hooks.0.handler.type").String()
	if hookType == "" {
		hookType = "command"
	}

	var parts []string
	if event != "" {
		parts = append(parts, "Event: "+event)
	}
	if matcher != "" {
		parts = append(parts, "Matcher: "+matcher)
	}
	parts = append(parts, "Handler: "+hookType)
	return strings.Join(parts, " · ")
}

// MCPSummary returns a one-line summary of an MCP item's configuration
// (server key, command). Returns empty string on read errors.
func MCPSummary(item ContentItem) string {
	configPath := filepath.Join(item.Path, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	servers := gjson.GetBytes(data, "mcpServers")
	if !servers.Exists() || !servers.IsObject() {
		return ""
	}

	var parts []string
	key := item.ServerKey
	if key == "" {
		servers.ForEach(func(k, _ gjson.Result) bool {
			key = k.String()
			return false
		})
	}
	if key != "" {
		parts = append(parts, "Server: "+key)
	}
	cmd := gjson.GetBytes(data, "mcpServers."+key+".command").String()
	args := gjson.GetBytes(data, "mcpServers."+key+".args")
	if cmd != "" {
		cmdStr := cmd
		if args.Exists() && args.IsArray() {
			for _, a := range args.Array() {
				cmdStr += " " + a.String()
			}
		}
		parts = append(parts, "Command: "+cmdStr)
	}
	return strings.Join(parts, " · ")
}
