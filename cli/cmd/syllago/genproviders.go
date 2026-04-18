package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ProviderManifest is the top-level JSON structure output by _genproviders.
type ProviderManifest struct {
	Version        string             `json:"version"`
	GeneratedAt    string             `json:"generatedAt"`
	SyllagoVersion string             `json:"syllagoVersion"`
	Providers      []ProviderCapEntry `json:"providers"`
	ContentTypes   []string           `json:"contentTypes"`
}

// ProviderCapEntry represents a single provider's full capability data.
type ProviderCapEntry struct {
	Name      string                       `json:"name"`
	Slug      string                       `json:"slug"`
	ConfigDir string                       `json:"configDir"`
	EmitPath  string                       `json:"emitPath,omitempty"` // e.g. "{project}/CLAUDE.md"
	DocsURL   string                       `json:"docsURL,omitempty"`  // provider's official documentation root (from format-doc docs_url)
	Category  string                       `json:"category,omitempty"` // "cli" | "ide-extension" | "standalone-app" | "web-based" (from format-doc category)
	Content   map[string]ContentCapability `json:"content"`
}

// ContentCapability describes a provider's capability for one content type.
type ContentCapability struct {
	Supported      bool     `json:"supported"`
	FileFormat     string   `json:"fileFormat,omitempty"`
	InstallMethod  string   `json:"installMethod,omitempty"` // filesystem | json-merge | project-scope
	InstallPath    string   `json:"installPath,omitempty"`   // template with {home}
	SymlinkSupport bool     `json:"symlinkSupport"`
	DiscoveryPaths []string `json:"discoveryPaths,omitempty"` // templates with {project}, {home}

	// Hooks enrichment (only populated for hooks content type)
	HookEvents     []HookEventInfo `json:"hookEvents,omitempty"`
	HookTypes      []string        `json:"hookTypes,omitempty"`      // e.g. ["command", "http", "prompt", "agent"]
	ConfigLocation string          `json:"configLocation,omitempty"` // where hooks/MCP are configured

	// MCP enrichment (only populated for mcp content type)
	MCPTransports []string `json:"mcpTransports,omitempty"` // e.g. ["stdio", "sse", "streamable-http"]

	// Frontmatter fields (for markdown-based content types)
	FrontmatterFields []string `json:"frontmatterFields,omitempty"` // e.g. ["name", "description", "alwaysApply"]
}

// HookEventInfo describes a single hook event supported by a provider.
type HookEventInfo struct {
	Canonical  string `json:"canonical"`          // e.g. "before_tool_execute"
	NativeName string `json:"nativeName"`         // e.g. "PreToolUse"
	Category   string `json:"category,omitempty"` // e.g. "tool", "lifecycle", "model"
}

var genprovidersCmd = &cobra.Command{
	Use:    "_genproviders",
	Short:  "Generate providers.json manifest",
	Hidden: true,
	RunE:   runGenproviders,
}

func init() {
	rootCmd.AddCommand(genprovidersCmd)
}

// providerFormatsDirForDocsURL is the directory containing
// docs/provider-formats/*.yaml files from which we cross-reference authored
// static metadata (docs_url, category). Overridable in tests.
var providerFormatsDirForDocsURL = filepath.Join("..", "docs", "provider-formats")

// providerStaticMetaYAML captures only the fields we need to pluck out of a
// format-doc YAML without pulling in the full capmon schema.
type providerStaticMetaYAML struct {
	Provider string `yaml:"provider"`
	DocsURL  string `yaml:"docs_url"`
	Category string `yaml:"category"`
}

// providerStaticMeta carries per-provider static fields sourced from format-doc YAMLs.
type providerStaticMeta struct {
	DocsURL  string
	Category string
}

// loadProviderStaticMeta reads every <slug>.yaml under dir and returns a map
// from provider slug → its format-doc-authored static metadata. Slug is derived
// from the filename stem so a mismatched provider: field inside the file does
// not shadow it.
func loadProviderStaticMeta(dir string) (map[string]providerStaticMeta, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("readdir %q: %w", dir, err)
	}
	out := make(map[string]providerStaticMeta)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		slug := strings.TrimSuffix(name, ".yaml")
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", name, err)
		}
		var doc providerStaticMetaYAML
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("parsing %q: %w", name, err)
		}
		out[slug] = providerStaticMeta{DocsURL: doc.DocsURL, Category: doc.Category}
	}
	return out, nil
}

func runGenproviders(_ *cobra.Command, _ []string) error {
	staticMeta, err := loadProviderStaticMeta(providerFormatsDirForDocsURL)
	if err != nil {
		return fmt.Errorf("loading provider static metadata: %w", err)
	}

	var entries []ProviderCapEntry
	for _, prov := range provider.AllProviders {
		entries = append(entries, buildProviderEntry(prov, staticMeta[prov.Slug]))
	}

	v := version
	if v == "" {
		v = "dev"
	}

	var ctNames []string
	for _, ct := range catalog.AllContentTypes() {
		ctNames = append(ctNames, string(ct))
	}

	manifest := ProviderManifest{
		Version:        "1",
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		SyllagoVersion: v,
		Providers:      entries,
		ContentTypes:   ctNames,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

func buildProviderEntry(prov provider.Provider, meta providerStaticMeta) ProviderCapEntry {
	content := make(map[string]ContentCapability)

	for _, ct := range catalog.AllContentTypes() {
		content[string(ct)] = buildContentCap(prov, ct)
	}

	var emitPath string
	if prov.EmitPath != nil {
		emitPath = prov.EmitPath("{project}")
	}

	return ProviderCapEntry{
		Name:      prov.Name,
		Slug:      prov.Slug,
		ConfigDir: prov.ConfigDir,
		EmitPath:  emitPath,
		DocsURL:   meta.DocsURL,
		Category:  meta.Category,
		Content:   content,
	}
}

func buildContentCap(prov provider.Provider, ct catalog.ContentType) ContentCapability {
	supported := prov.SupportsType != nil && prov.SupportsType(ct)
	if !supported {
		return ContentCapability{Supported: false}
	}

	cap := ContentCapability{
		Supported: true,
	}

	// File format.
	if prov.FileFormat != nil {
		cap.FileFormat = string(prov.FileFormat(ct))
	}

	// Install method and path.
	if prov.InstallDir != nil {
		dir := prov.InstallDir("{home}", ct)
		switch dir {
		case provider.JSONMergeSentinel:
			cap.InstallMethod = "json-merge"
		case provider.ProjectScopeSentinel:
			cap.InstallMethod = "project-scope"
		case "":
			// Not supported via install (shouldn't happen if SupportsType is true).
			cap.InstallMethod = ""
		default:
			cap.InstallMethod = "filesystem"
			cap.InstallPath = dir
		}
	}

	// Symlink support.
	if prov.SymlinkSupport != nil {
		cap.SymlinkSupport = prov.SymlinkSupport[ct]
	}

	// Discovery paths.
	// Some providers (Cline, Roo Code) call os.UserHomeDir() internally,
	// producing machine-specific absolute paths. Sanitize those to {home}.
	if prov.DiscoveryPaths != nil {
		paths := prov.DiscoveryPaths("{project}", ct)
		if len(paths) > 0 {
			cap.DiscoveryPaths = sanitizeHomePaths(paths)
		}
	}

	// --- Enrichment: hooks ---
	if ct == catalog.Hooks {
		cap.HookEvents = buildHookEvents(prov.Slug)
		if len(prov.HookTypes) > 0 {
			cap.HookTypes = prov.HookTypes
		}
		if loc := prov.ConfigLocations[catalog.Hooks]; loc != "" {
			cap.ConfigLocation = loc
		}
	}

	// --- Enrichment: MCP ---
	if ct == catalog.MCP {
		if len(prov.MCPTransports) > 0 {
			cap.MCPTransports = prov.MCPTransports
		}
		if loc := prov.ConfigLocations[catalog.MCP]; loc != "" {
			cap.ConfigLocation = loc
		}
	}

	// --- Enrichment: frontmatter ---
	if fm := converter.FrontmatterFieldsFor(ct, prov.Slug); len(fm) > 0 {
		cap.FrontmatterFields = fm
	}

	return cap
}

// --- Path sanitization ---

// sanitizeHomePaths replaces absolute home directory prefixes with {home}.
// Some provider DiscoveryPaths implementations call os.UserHomeDir() directly,
// which bakes machine-specific paths into the generated manifest. This function
// normalizes those back to the {home} placeholder.
func sanitizeHomePaths(paths []string) []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return paths
	}
	// Ensure trailing separator for clean replacement.
	homePrefix := home + string(os.PathSeparator)

	out := make([]string, len(paths))
	for i, p := range paths {
		if strings.HasPrefix(p, homePrefix) {
			out[i] = "{home}/" + strings.TrimPrefix(p, homePrefix)
		} else if p == home {
			out[i] = "{home}"
		} else {
			out[i] = p
		}
	}
	return out
}

// --- Hook enrichment helpers ---

// hookEventCategory classifies canonical hook event names into categories for docs display.
var hookEventCategory = map[string]string{
	"before_tool_execute":   "tool",
	"after_tool_execute":    "tool",
	"after_tool_failure":    "tool",
	"before_prompt":         "lifecycle",
	"agent_stop":            "lifecycle",
	"session_start":         "lifecycle",
	"session_end":           "lifecycle",
	"subagent_start":        "lifecycle",
	"subagent_stop":         "lifecycle",
	"error_occurred":        "lifecycle",
	"before_compact":        "context",
	"after_compact":         "context",
	"instructions_loaded":   "context",
	"notification":          "output",
	"permission_request":    "security",
	"config_change":         "config",
	"worktree_create":       "workspace",
	"worktree_remove":       "workspace",
	"elicitation":           "interaction",
	"elicitation_result":    "interaction",
	"teammate_idle":         "collaboration",
	"task_completed":        "lifecycle",
	"stop_failure":          "lifecycle",
	"before_model":          "model",
	"after_model":           "model",
	"before_tool_selection": "model",
	"task_resume":           "lifecycle",
	"task_cancel":           "lifecycle",
	// Events added for multi-provider coverage (kiro, cursor, windsurf, pi, opencode)
	"tool_use_failure":  "tool",
	"file_changed":      "workspace",
	"file_created":      "workspace",
	"file_deleted":      "workspace",
	"before_task":       "lifecycle",
	"after_task":        "lifecycle",
	"transcript_export": "output",
	"turn_start":        "model",
	"turn_end":          "model",
	"model_select":      "model",
	"user_bash":         "tool",
	"context_update":    "context",
	"message_start":     "model",
	"message_end":       "model",
}

// buildHookEvents derives the list of hook events a provider supports
// from the converter.HookEvents map.
func buildHookEvents(slug string) []HookEventInfo {
	var events []HookEventInfo
	for canonical, provMap := range converter.HookEvents {
		if native, ok := provMap[slug]; ok {
			events = append(events, HookEventInfo{
				Canonical:  canonical,
				NativeName: native,
				Category:   hookEventCategory[canonical],
			})
		}
	}
	// Sort by canonical name for deterministic output.
	sort.Slice(events, func(i, j int) bool {
		return events[i].Canonical < events[j].Canonical
	})
	return events
}
