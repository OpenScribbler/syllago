package catalog

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
)

// NativeItem represents a single discovered native content item.
type NativeItem struct {
	Name        string // item name (from dir/file name)
	DisplayName string // human-readable from frontmatter
	Description string // from frontmatter or content
	Path        string // relative to repo root
	HookEvent   string // for hooks: which event
	HookIndex   int    // for hooks: index within event array
}

// NativeProviderContent holds found content for one provider.
type NativeProviderContent struct {
	ProviderSlug string
	ProviderName string
	// Discovered items grouped by type label (e.g. "rules", "skills").
	Items map[string][]NativeItem
}

// NativeScanResult holds provider-native content found in a directory.
type NativeScanResult struct {
	Providers []NativeProviderContent
	// HasSyllagoStructure is true if the directory looks like a syllago registry
	// (has registry.yaml or has standard content type directories).
	HasSyllagoStructure bool
}

// ScanNativeContent scans dir for provider-native AI tool content.
// Only scans for providers that syllago supports. Returns findings grouped
// by provider. Reads file contents only for embedded JSON (hooks/MCP) and
// for markdown frontmatter (skills/agents).
func ScanNativeContent(dir string) NativeScanResult {
	var result NativeScanResult

	// Check for registry.yaml first.
	if _, err := os.Stat(filepath.Join(dir, "registry.yaml")); err == nil {
		result.HasSyllagoStructure = true
		return result
	}

	// Check for syllago content dirs.
	for _, ct := range AllContentTypes() {
		if _, err := os.Stat(filepath.Join(dir, string(ct))); err == nil {
			result.HasSyllagoStructure = true
			return result
		}
	}

	// Scan for provider-native patterns.
	patterns := providerNativePatterns()
	seen := make(map[string]*NativeProviderContent)

	ensureProvider := func(slug, name string) *NativeProviderContent {
		if _, ok := seen[slug]; !ok {
			seen[slug] = &NativeProviderContent{
				ProviderSlug: slug,
				ProviderName: name,
				Items:        make(map[string][]NativeItem),
			}
		}
		return seen[slug]
	}

	for _, p := range patterns {
		fullPath := filepath.Join(dir, p.path)

		if p.embedded {
			// Embedded JSON: parse content to extract items.
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			pc := ensureProvider(p.providerSlug, p.providerName)
			if p.typeLabel == "hooks" {
				items := extractEmbeddedHooks(data, p.path)
				if len(items) > 0 {
					pc.Items[p.typeLabel] = append(pc.Items[p.typeLabel], items...)
				}
			} else if p.typeLabel == "mcp" {
				items := extractEmbeddedMCP(data, p.path)
				if len(items) > 0 {
					pc.Items[p.typeLabel] = append(pc.Items[p.typeLabel], items...)
				}
			}
			continue
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		pc := ensureProvider(p.providerSlug, p.providerName)

		if info.IsDir() {
			entries, err := os.ReadDir(fullPath)
			if err != nil {
				continue
			}
			for _, e := range entries {
				entryPath := filepath.Join(p.path, e.Name())
				name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				item := NativeItem{
					Name: name,
					Path: entryPath,
				}

				// For skills and agents, try to read frontmatter.
				if p.typeLabel == "skills" || p.typeLabel == "agents" {
					item = enrichFromFrontmatter(item, filepath.Join(dir, entryPath))
				}

				if !e.IsDir() {
					pc.Items[p.typeLabel] = append(pc.Items[p.typeLabel], item)
				} else {
					// Directory entry (e.g. .cursor/rules subdir): use dir name.
					item.Name = e.Name()
					item.Path = entryPath
					// Look for a primary file inside the subdir (SKILL.md, AGENT.md).
					if p.typeLabel == "skills" {
						item = enrichFromSubdirFile(item, filepath.Join(dir, entryPath), "SKILL.md")
					} else if p.typeLabel == "agents" {
						item = enrichFromSubdirFile(item, filepath.Join(dir, entryPath), "AGENT.md")
					}
					pc.Items[p.typeLabel] = append(pc.Items[p.typeLabel], item)
				}
			}
		} else {
			// Single file. Use the base name without extension, but fall back
			// to the full base name for dotfiles (e.g. ".cursorrules" where
			// filepath.Ext returns the entire name, leaving an empty stem).
			base := filepath.Base(p.path)
			ext := filepath.Ext(p.path)
			stem := strings.TrimSuffix(base, ext)
			if stem == "" {
				stem = base
			}
			item := NativeItem{
				Name: stem,
				Path: p.path,
			}
			pc.Items[p.typeLabel] = append(pc.Items[p.typeLabel], item)
		}
	}

	for _, pc := range seen {
		if len(pc.Items) > 0 {
			result.Providers = append(result.Providers, *pc)
		}
	}
	return result
}

// enrichFromFrontmatter tries to read frontmatter from a file and populate
// DisplayName and Description on the item.
func enrichFromFrontmatter(item NativeItem, absPath string) NativeItem {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return item
	}
	fm, fmErr := ParseFrontmatter(data)
	if fmErr != nil {
		return item
	}
	if fm.Name != "" {
		item.DisplayName = fm.Name
	}
	if fm.Description != "" {
		item.Description = fm.Description
	}
	return item
}

// enrichFromSubdirFile looks for a specific file inside a subdirectory and
// reads its frontmatter to enrich the item.
func enrichFromSubdirFile(item NativeItem, absDir, filename string) NativeItem {
	return enrichFromFrontmatter(item, filepath.Join(absDir, filename))
}

// extractEmbeddedHooks parses a settings JSON file for Claude Code-style
// hooks and returns one NativeItem per event+index.
//
// Expected format: {"hooks": {"PostToolUse": [{"matcher":"...", ...}]}}
func extractEmbeddedHooks(data []byte, relPath string) []NativeItem {
	var items []NativeItem
	hooksObj := gjson.GetBytes(data, "hooks")
	if !hooksObj.Exists() || !hooksObj.IsObject() {
		return nil
	}
	hooksObj.ForEach(func(eventKey, eventVal gjson.Result) bool {
		if !eventVal.IsArray() {
			return true
		}
		idx := 0
		eventVal.ForEach(func(_, hookVal gjson.Result) bool {
			// Use matcher as name hint if available.
			matcher := hookVal.Get("matcher").String()
			name := eventKey.String()
			if matcher != "" {
				name = eventKey.String() + ":" + matcher
			}
			items = append(items, NativeItem{
				Name:      name,
				Path:      relPath,
				HookEvent: eventKey.String(),
				HookIndex: idx,
			})
			idx++
			return true
		})
		return true
	})
	return items
}

// extractEmbeddedMCP parses a JSON file for MCP server definitions and
// returns one NativeItem per server name.
//
// Most providers use "mcpServers"; Zed uses "context_servers". Both keys
// are checked so a single scan handles all providers.
func extractEmbeddedMCP(data []byte, relPath string) []NativeItem {
	var items []NativeItem
	for _, key := range []string{"mcpServers", "context_servers"} {
		serversObj := gjson.GetBytes(data, key)
		if !serversObj.Exists() || !serversObj.IsObject() {
			continue
		}
		serversObj.ForEach(func(nameKey, _ gjson.Result) bool {
			items = append(items, NativeItem{
				Name: nameKey.String(),
				Path: relPath,
			})
			return true
		})
	}
	return items
}

// nativePattern describes one provider-native path to check.
type nativePattern struct {
	providerSlug string
	providerName string
	path         string // relative to repo root
	typeLabel    string // e.g. "rules", "skills"
	embedded     bool   // true if content is embedded in a JSON file (hooks, mcp)
}

// providerNativePatterns returns all known provider-native content paths.
func providerNativePatterns() []nativePattern {
	return []nativePattern{
		// Claude Code
		{providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/commands", typeLabel: "commands"},
		{providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/skills", typeLabel: "skills"},
		{providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/agents", typeLabel: "agents"},
		{providerSlug: "claude-code", providerName: "Claude Code", path: "CLAUDE.md", typeLabel: "rules"},
		// Project-scoped hooks (embedded in settings JSON)
		{providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/settings.json", typeLabel: "hooks", embedded: true},
		{providerSlug: "copilot-cli", providerName: "Copilot CLI", path: ".copilot/hooks.json", typeLabel: "hooks", embedded: true},
		// Gemini CLI
		{providerSlug: "gemini-cli", providerName: "Gemini CLI", path: ".gemini", typeLabel: "config"},
		// Cursor
		{providerSlug: "cursor", providerName: "Cursor", path: ".cursorrules", typeLabel: "rules"},
		{providerSlug: "cursor", providerName: "Cursor", path: ".cursor/rules", typeLabel: "rules"},
		// Windsurf
		{providerSlug: "windsurf", providerName: "Windsurf", path: ".windsurfrules", typeLabel: "rules"},
		// Codex
		{providerSlug: "codex", providerName: "Codex", path: ".codex", typeLabel: "config"},
		// Copilot CLI
		{providerSlug: "copilot-cli", providerName: "Copilot CLI", path: ".github/copilot-instructions.md", typeLabel: "rules"},
		// Project-scoped MCP (embedded in JSON)
		{providerSlug: "copilot-cli", providerName: "Copilot CLI", path: ".copilot/mcp.json", typeLabel: "mcp", embedded: true},
		{providerSlug: "cline", providerName: "Cline", path: ".vscode/mcp.json", typeLabel: "mcp", embedded: true},
		{providerSlug: "roo-code", providerName: "Roo Code", path: ".roo/mcp.json", typeLabel: "mcp", embedded: true},
		{providerSlug: "kiro", providerName: "Kiro", path: ".kiro/settings/mcp.json", typeLabel: "mcp", embedded: true},
		{providerSlug: "opencode", providerName: "OpenCode", path: "opencode.json", typeLabel: "mcp", embedded: true},
		// Zed
		{providerSlug: "zed", providerName: "Zed", path: ".zed/settings.json", typeLabel: "mcp", embedded: true},
		{providerSlug: "zed", providerName: "Zed", path: ".zed", typeLabel: "settings"},
		// Cline
		{providerSlug: "cline", providerName: "Cline", path: ".clinerules", typeLabel: "rules"},
		// Roo Code
		{providerSlug: "roo-code", providerName: "Roo Code", path: ".roo", typeLabel: "config"},
		// OpenCode
		{providerSlug: "opencode", providerName: "OpenCode", path: ".opencode", typeLabel: "config"},
		// Kiro
		{providerSlug: "kiro", providerName: "Kiro", path: ".kiro", typeLabel: "config"},
	}
}
