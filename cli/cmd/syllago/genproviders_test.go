package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// repoProviderFormatsDir resolves the absolute path to the repo's real
// docs/provider-formats directory, independent of the test working directory.
// Used to point var-overrides like providerFormatsDirForDocsURL at on-disk
// YAMLs when exercising genprovidersCmd end-to-end.
func repoProviderFormatsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// thisFile = <repo>/cli/cmd/syllago/genproviders_test.go
	// target  = <repo>/docs/provider-formats
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "docs", "provider-formats")
}

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
// Reading happens in a goroutine to avoid pipe buffer deadlock on large outputs.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	fn()
	w.Close()
	<-done
	r.Close()

	return buf.Bytes()
}

func TestGenproviders(t *testing.T) {
	origDir := providerFormatsDirForDocsURL
	providerFormatsDirForDocsURL = repoProviderFormatsDir(t)
	t.Cleanup(func() { providerFormatsDirForDocsURL = origDir })

	raw := captureStdout(t, func() {
		if err := genprovidersCmd.RunE(genprovidersCmd, nil); err != nil {
			t.Fatalf("_genproviders failed: %v", err)
		}
	})

	var manifest ProviderManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("output is not valid JSON: %v\nfirst 200 bytes: %s", err, string(raw[:min(200, len(raw))]))
	}

	// Must have version and content types.
	if manifest.Version != "1" {
		t.Errorf("version = %q, want %q", manifest.Version, "1")
	}
	if len(manifest.ContentTypes) == 0 {
		t.Error("contentTypes is empty")
	}

	// Must have all providers.
	if len(manifest.Providers) != len(provider.AllProviders) {
		t.Errorf("providers count = %d, want %d", len(manifest.Providers), len(provider.AllProviders))
	}

	// Build a lookup for validation.
	provBySlug := make(map[string]ProviderCapEntry)
	for _, p := range manifest.Providers {
		provBySlug[p.Slug] = p
	}

	// Validate each provider matches the Go definition.
	for _, goProv := range provider.AllProviders {
		t.Run(goProv.Slug, func(t *testing.T) {
			jsonProv, ok := provBySlug[goProv.Slug]
			if !ok {
				t.Fatalf("provider %q missing from JSON output", goProv.Slug)
			}

			if jsonProv.Name != goProv.Name {
				t.Errorf("name = %q, want %q", jsonProv.Name, goProv.Name)
			}
			if jsonProv.ConfigDir != goProv.ConfigDir {
				t.Errorf("configDir = %q, want %q", jsonProv.ConfigDir, goProv.ConfigDir)
			}

			// Verify content type support matches.
			for _, ct := range catalog.AllContentTypes() {
				ctName := string(ct)
				goSupports := goProv.SupportsType != nil && goProv.SupportsType(ct)
				jsonCap, exists := jsonProv.Content[ctName]

				if !exists {
					t.Errorf("content type %q missing from JSON", ctName)
					continue
				}

				if jsonCap.Supported != goSupports {
					t.Errorf("content[%q].supported = %v, Go says %v", ctName, jsonCap.Supported, goSupports)
				}

				if !goSupports {
					continue
				}

				// Verify file format matches.
				if goProv.FileFormat != nil {
					goFormat := string(goProv.FileFormat(ct))
					if jsonCap.FileFormat != goFormat {
						t.Errorf("content[%q].fileFormat = %q, Go says %q", ctName, jsonCap.FileFormat, goFormat)
					}
				}

				// Verify install method.
				if goProv.InstallDir != nil {
					dir := goProv.InstallDir("{home}", ct)
					switch dir {
					case provider.JSONMergeSentinel:
						if jsonCap.InstallMethod != "json-merge" {
							t.Errorf("content[%q].installMethod = %q, want %q", ctName, jsonCap.InstallMethod, "json-merge")
						}
					case provider.ProjectScopeSentinel:
						if jsonCap.InstallMethod != "project-scope" {
							t.Errorf("content[%q].installMethod = %q, want %q", ctName, jsonCap.InstallMethod, "project-scope")
						}
					case "":
						// No install dir — skip.
					default:
						if jsonCap.InstallMethod != "filesystem" {
							t.Errorf("content[%q].installMethod = %q, want %q", ctName, jsonCap.InstallMethod, "filesystem")
						}
						if jsonCap.InstallPath != dir {
							t.Errorf("content[%q].installPath = %q, want %q", ctName, jsonCap.InstallPath, dir)
						}
					}
				}

				// Verify symlink support.
				if goProv.SymlinkSupport != nil {
					goSymlink := goProv.SymlinkSupport[ct]
					if jsonCap.SymlinkSupport != goSymlink {
						t.Errorf("content[%q].symlinkSupport = %v, Go says %v", ctName, jsonCap.SymlinkSupport, goSymlink)
					}
				}

				// Verify discovery paths count.
				if goProv.DiscoveryPaths != nil {
					goPaths := goProv.DiscoveryPaths("{project}", ct)
					if len(goPaths) != len(jsonCap.DiscoveryPaths) {
						t.Errorf("content[%q].discoveryPaths count = %d, Go has %d", ctName, len(jsonCap.DiscoveryPaths), len(goPaths))
					}
				}

				// Verify enrichment: hooks should have types and config location.
				// Events may be empty for providers without event mappings (e.g. Windsurf, Codex).
				if ct == catalog.Hooks {
					if len(jsonCap.HookTypes) == 0 {
						t.Errorf("content[hooks].hookTypes is empty for %s", goProv.Slug)
					}
					if jsonCap.ConfigLocation == "" {
						t.Errorf("content[hooks].configLocation is empty for %s", goProv.Slug)
					}
					// Verify each hook event has canonical and native name.
					for _, ev := range jsonCap.HookEvents {
						if ev.Canonical == "" {
							t.Error("hookEvent has empty canonical name")
						}
						if ev.NativeName == "" {
							t.Error("hookEvent has empty nativeName")
						}
					}
				}

				// Verify enrichment: MCP should have transports.
				if ct == catalog.MCP {
					if len(jsonCap.MCPTransports) == 0 {
						t.Errorf("content[mcp].mcpTransports is empty for %s", goProv.Slug)
					}
					if jsonCap.ConfigLocation == "" {
						t.Errorf("content[mcp].configLocation is empty for %s", goProv.Slug)
					}
				}
			}
		})
	}
}

func TestGenproviders_ClaudeCodeHookDetails(t *testing.T) {
	manifest := loadTestManifest(t)
	cc := findProvider(t, manifest, "claude-code")

	hooks := cc.Content["hooks"]

	// Claude Code should support all 4 hook types.
	if len(hooks.HookTypes) != 4 {
		t.Errorf("claude-code hookTypes count = %d, want 4", len(hooks.HookTypes))
	}
	wantTypes := map[string]bool{"command": true, "http": true, "prompt": true, "agent": true}
	for _, ht := range hooks.HookTypes {
		if !wantTypes[ht] {
			t.Errorf("unexpected hookType %q", ht)
		}
	}

	// Should have a significant number of events (CC has the most).
	if len(hooks.HookEvents) < 10 {
		t.Errorf("claude-code hookEvents count = %d, want >= 10", len(hooks.HookEvents))
	}

	// Verify a specific event exists with correct category.
	found := false
	for _, ev := range hooks.HookEvents {
		if ev.Canonical == "before_tool_execute" && ev.NativeName == "PreToolUse" {
			found = true
			if ev.Category != "tool" {
				t.Errorf("before_tool_execute category = %q, want %q", ev.Category, "tool")
			}
			break
		}
	}
	if !found {
		t.Error("claude-code missing before_tool_execute/PreToolUse hook event")
	}

	// Verify all events have a category assigned.
	for _, ev := range hooks.HookEvents {
		if ev.Category == "" {
			t.Errorf("hook event %q has no category", ev.Canonical)
		}
	}

	// Verify emitPath is populated.
	if cc.EmitPath == "" {
		t.Error("claude-code emitPath is empty")
	}

	// Verify MCP transports.
	mcp := cc.Content["mcp"]
	if len(mcp.MCPTransports) != 3 {
		t.Errorf("claude-code mcpTransports count = %d, want 3", len(mcp.MCPTransports))
	}
	if mcp.ConfigLocation != ".mcp.json" {
		t.Errorf("claude-code mcp.configLocation = %q, want %q", mcp.ConfigLocation, ".mcp.json")
	}

	// Verify rules frontmatter.
	rules := cc.Content["rules"]
	if len(rules.FrontmatterFields) == 0 {
		t.Error("claude-code rules.frontmatterFields is empty")
	}
	// CC rules should include paths (description is not emitted in CC rule files).
	fmSet := toSet(rules.FrontmatterFields)
	if !fmSet["paths"] {
		t.Error("claude-code rules.frontmatterFields missing 'paths'")
	}
	if fmSet["description"] {
		t.Error("claude-code rules.frontmatterFields should not include 'description' (not emitted by CC)")
	}
}

// TestGenproviders_NonCCProviderEnrichment tests enrichment for providers that
// are NOT Claude Code — covers the sparse/partial data cases.
func TestGenproviders_NonCCProviderEnrichment(t *testing.T) {
	manifest := loadTestManifest(t)

	t.Run("gemini-cli", func(t *testing.T) {
		prov := findProvider(t, manifest, "gemini-cli")

		hooks := prov.Content["hooks"]
		// Gemini should have events (it's in HookEvents map).
		if len(hooks.HookEvents) < 3 {
			t.Errorf("gemini-cli hookEvents count = %d, want >= 3", len(hooks.HookEvents))
		}
		// Gemini only supports command hooks.
		if len(hooks.HookTypes) != 1 || hooks.HookTypes[0] != "command" {
			t.Errorf("gemini-cli hookTypes = %v, want [command]", hooks.HookTypes)
		}

		// Verify a Gemini-specific event.
		foundBeforeModel := false
		for _, ev := range hooks.HookEvents {
			if ev.Canonical == "before_model" && ev.NativeName == "BeforeModel" {
				foundBeforeModel = true
				break
			}
		}
		if !foundBeforeModel {
			t.Error("gemini-cli missing before_model/BeforeModel event")
		}

		// MCP
		mcp := prov.Content["mcp"]
		if len(mcp.MCPTransports) == 0 {
			t.Error("gemini-cli mcpTransports is empty")
		}
		if mcp.ConfigLocation == "" {
			t.Error("gemini-cli mcp.configLocation is empty")
		}

		// EmitPath
		if prov.EmitPath == "" {
			t.Error("gemini-cli emitPath is empty")
		}
	})

	t.Run("zed-hookless", func(t *testing.T) {
		prov := findProvider(t, manifest, "zed")

		// Zed does NOT support hooks — hooks.supported should be false.
		hooks := prov.Content["hooks"]
		if hooks.Supported {
			t.Error("zed hooks.supported = true, want false")
		}
		// Enrichment fields should be empty for unsupported types.
		if len(hooks.HookEvents) > 0 {
			t.Errorf("zed hookEvents should be empty, got %d", len(hooks.HookEvents))
		}
		if len(hooks.HookTypes) > 0 {
			t.Errorf("zed hookTypes should be empty, got %v", hooks.HookTypes)
		}

		// Zed supports MCP — should have transports.
		mcp := prov.Content["mcp"]
		if !mcp.Supported {
			t.Error("zed mcp.supported = false, want true")
		}
		if len(mcp.MCPTransports) == 0 {
			t.Error("zed mcpTransports is empty")
		}
	})

	t.Run("windsurf-hooks-no-events", func(t *testing.T) {
		prov := findProvider(t, manifest, "windsurf")

		// Windsurf supports hooks but has no event mappings in HookEvents.
		hooks := prov.Content["hooks"]
		if !hooks.Supported {
			t.Error("windsurf hooks.supported = false, want true")
		}
		// Should still have hookTypes and configLocation even without events.
		if len(hooks.HookTypes) == 0 {
			t.Error("windsurf hookTypes is empty")
		}
		if hooks.ConfigLocation == "" {
			t.Error("windsurf hooks.configLocation is empty")
		}
	})

	t.Run("cursor-frontmatter", func(t *testing.T) {
		prov := findProvider(t, manifest, "cursor")

		// Cursor rules use MDC format with frontmatter.
		rules := prov.Content["rules"]
		if rules.FileFormat != "mdc" {
			t.Errorf("cursor rules.fileFormat = %q, want %q", rules.FileFormat, "mdc")
		}
		if len(rules.FrontmatterFields) == 0 {
			t.Error("cursor rules.frontmatterFields is empty")
		}
		fmSet := toSet(rules.FrontmatterFields)
		if !fmSet["globs"] {
			t.Error("cursor rules.frontmatterFields missing 'globs'")
		}
	})

	t.Run("opencode-no-hooks", func(t *testing.T) {
		prov := findProvider(t, manifest, "opencode")

		// OpenCode does not support hooks.
		hooks := prov.Content["hooks"]
		if hooks.Supported {
			t.Error("opencode hooks.supported = true, want false")
		}

		// OpenCode supports MCP.
		mcp := prov.Content["mcp"]
		if !mcp.Supported {
			t.Error("opencode mcp.supported = false, want true")
		}
		if len(mcp.MCPTransports) == 0 {
			t.Error("opencode mcpTransports is empty")
		}
	})
}

// TestGenproviders_EventSorting verifies hook events are sorted deterministically.
func TestGenproviders_EventSorting(t *testing.T) {
	manifest := loadTestManifest(t)
	cc := findProvider(t, manifest, "claude-code")
	events := cc.Content["hooks"].HookEvents

	for i := 1; i < len(events); i++ {
		if events[i].Canonical < events[i-1].Canonical {
			t.Errorf("hookEvents not sorted: %q comes after %q",
				events[i].Canonical, events[i-1].Canonical)
		}
	}
}

// TestGenproviders_AllMCPProvidersHaveTransports verifies every provider that
// supports MCP has at least one transport listed.
func TestGenproviders_AllMCPProvidersHaveTransports(t *testing.T) {
	manifest := loadTestManifest(t)

	for _, prov := range manifest.Providers {
		mcp := prov.Content["mcp"]
		if !mcp.Supported {
			continue
		}
		if len(mcp.MCPTransports) == 0 {
			t.Errorf("provider %q supports MCP but has no transports", prov.Slug)
		}
		if mcp.ConfigLocation == "" {
			t.Errorf("provider %q supports MCP but has no configLocation", prov.Slug)
		}
	}
}

// TestGenproviders_AllEmitPathsPopulated verifies emitPath is set for all providers.
func TestGenproviders_AllEmitPathsPopulated(t *testing.T) {
	manifest := loadTestManifest(t)

	for _, prov := range manifest.Providers {
		if prov.EmitPath == "" {
			t.Errorf("provider %q has empty emitPath", prov.Slug)
		}
	}
}

// TestGenproviders_AllDocsURLsPopulated verifies every provider entry carries a
// docsURL sourced from its format-doc YAML, and that the URL is http(s). This
// is the providers.json-side mirror of the capmon docs_url validator.
func TestGenproviders_AllDocsURLsPopulated(t *testing.T) {
	manifest := loadTestManifest(t)

	for _, prov := range manifest.Providers {
		if prov.DocsURL == "" {
			t.Errorf("provider %q has empty docsURL", prov.Slug)
			continue
		}
		if !strings.HasPrefix(prov.DocsURL, "http://") && !strings.HasPrefix(prov.DocsURL, "https://") {
			t.Errorf("provider %q docsURL = %q; must be http(s) URL", prov.Slug, prov.DocsURL)
		}
	}
}

// TestGenproviders_AllCategoriesPopulated verifies every provider entry carries
// a category sourced from its format-doc YAML, and that the value is in the
// allowed enum. This is the providers.json-side mirror of the capmon category
// validator.
func TestGenproviders_AllCategoriesPopulated(t *testing.T) {
	manifest := loadTestManifest(t)

	allowed := map[string]bool{
		"cli":            true,
		"ide-extension":  true,
		"standalone-app": true,
		"web-based":      true,
	}
	for _, prov := range manifest.Providers {
		if prov.Category == "" {
			t.Errorf("provider %q has empty category", prov.Slug)
			continue
		}
		if !allowed[prov.Category] {
			t.Errorf("provider %q category = %q; must be one of cli|ide-extension|standalone-app|web-based", prov.Slug, prov.Category)
		}
	}
}

// TestGenproviders_ConfigLocationMatchesDiscoveryPaths ensures that the hardcoded
// configLocation values in genproviders.go are consistent with what the provider's
// DiscoveryPaths function actually returns. This catches copy-paste errors where
// config paths are guessed rather than derived from the provider definition.
func TestGenproviders_ConfigLocationMatchesDiscoveryPaths(t *testing.T) {
	manifest := loadTestManifest(t)

	for _, goProv := range provider.AllProviders {
		prov := findProvider(t, manifest, goProv.Slug)

		// Check hooks configLocation against DiscoveryPaths.
		hooksCap := prov.Content["hooks"]
		if hooksCap.Supported && hooksCap.ConfigLocation != "" && goProv.DiscoveryPaths != nil {
			discoveryPaths := goProv.DiscoveryPaths("{project}", catalog.Hooks)
			if len(discoveryPaths) > 0 {
				// The configLocation should be related to at least one discovery path.
				// Strip the {project}/ prefix for comparison.
				found := false
				for _, dp := range discoveryPaths {
					dpClean := strings.TrimPrefix(dp, "{project}/")
					// Exact match or the config location is a parent directory of a discovery path.
					if dpClean == hooksCap.ConfigLocation ||
						strings.HasPrefix(dpClean, hooksCap.ConfigLocation) ||
						strings.HasPrefix(hooksCap.ConfigLocation, dpClean) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("provider %q: hooks.configLocation %q does not match any DiscoveryPaths %v",
						goProv.Slug, hooksCap.ConfigLocation, discoveryPaths)
				}
			}
		}

		// Check MCP configLocation against DiscoveryPaths.
		mcpCap := prov.Content["mcp"]
		if mcpCap.Supported && mcpCap.ConfigLocation != "" && goProv.DiscoveryPaths != nil {
			discoveryPaths := goProv.DiscoveryPaths("{project}", catalog.MCP)
			if len(discoveryPaths) > 0 {
				found := false
				for _, dp := range discoveryPaths {
					dpClean := strings.TrimPrefix(dp, "{project}/")
					// Some providers (Cline) use absolute system paths for MCP config.
					// In that case, check if the config location filename appears in the path.
					if dpClean == mcpCap.ConfigLocation ||
						strings.HasPrefix(dpClean, mcpCap.ConfigLocation) ||
						strings.HasPrefix(mcpCap.ConfigLocation, dpClean) ||
						strings.HasSuffix(dp, mcpCap.ConfigLocation) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("provider %q: mcp.configLocation %q does not match any DiscoveryPaths %v",
						goProv.Slug, mcpCap.ConfigLocation, discoveryPaths)
				}
			}
		}
	}
}

// TestGenproviders_AllHookEventsHaveCategories ensures every hook event across
// all providers has a category assigned.
func TestGenproviders_AllHookEventsHaveCategories(t *testing.T) {
	manifest := loadTestManifest(t)

	for _, prov := range manifest.Providers {
		hooksCap := prov.Content["hooks"]
		for _, ev := range hooksCap.HookEvents {
			if ev.Category == "" {
				t.Errorf("provider %q: hook event %q has no category", prov.Slug, ev.Canonical)
			}
		}
	}
}

// TestGenproviders_AgentsCommandsFrontmatter verifies frontmatter fields for
// agents and commands content types match the actual Go struct definitions.
func TestGenproviders_AgentsCommandsFrontmatter(t *testing.T) {
	manifest := loadTestManifest(t)

	t.Run("claude-code-skills", func(t *testing.T) {
		cc := findProvider(t, manifest, "claude-code")
		skills := cc.Content["skills"]
		if len(skills.FrontmatterFields) < 10 {
			t.Errorf("claude-code skills.frontmatterFields count = %d, want >= 10 (SkillMeta has 15 fields)", len(skills.FrontmatterFields))
		}
		fmSet := toSet(skills.FrontmatterFields)
		for _, want := range []string{"allowed-tools", "context", "agent", "model", "effort", "hooks"} {
			if !fmSet[want] {
				t.Errorf("claude-code skills.frontmatterFields missing %q", want)
			}
		}
	})

	t.Run("claude-code-agents", func(t *testing.T) {
		cc := findProvider(t, manifest, "claude-code")
		agents := cc.Content["agents"]
		if len(agents.FrontmatterFields) < 10 {
			t.Errorf("claude-code agents.frontmatterFields count = %d, want >= 10 (AgentMeta has ~15 fields)", len(agents.FrontmatterFields))
		}
		fmSet := toSet(agents.FrontmatterFields)
		for _, want := range []string{"tools", "model", "maxTurns", "permissionMode", "skills", "mcpServers", "effort", "hooks", "color"} {
			if !fmSet[want] {
				t.Errorf("claude-code agents.frontmatterFields missing %q", want)
			}
		}
	})

	t.Run("claude-code-commands", func(t *testing.T) {
		cc := findProvider(t, manifest, "claude-code")
		commands := cc.Content["commands"]
		if len(commands.FrontmatterFields) < 8 {
			t.Errorf("claude-code commands.frontmatterFields count = %d, want >= 8 (CommandMeta has 10 fields)", len(commands.FrontmatterFields))
		}
		fmSet := toSet(commands.FrontmatterFields)
		for _, want := range []string{"allowed-tools", "context", "agent", "model", "argument-hint", "effort"} {
			if !fmSet[want] {
				t.Errorf("claude-code commands.frontmatterFields missing %q", want)
			}
		}
	})

	t.Run("cursor-agents", func(t *testing.T) {
		prov := findProvider(t, manifest, "cursor")
		agents := prov.Content["agents"]
		fmSet := toSet(agents.FrontmatterFields)
		for _, want := range []string{"readonly", "is_background"} {
			if !fmSet[want] {
				t.Errorf("cursor agents.frontmatterFields missing %q", want)
			}
		}
	})

	t.Run("kiro-agents", func(t *testing.T) {
		prov := findProvider(t, manifest, "kiro")
		agents := prov.Content["agents"]
		fmSet := toSet(agents.FrontmatterFields)
		for _, want := range []string{"model", "tools", "mcpServers"} {
			if !fmSet[want] {
				t.Errorf("kiro agents.frontmatterFields missing %q", want)
			}
		}
	})

	t.Run("roo-code-agents", func(t *testing.T) {
		prov := findProvider(t, manifest, "roo-code")
		agents := prov.Content["agents"]
		fmSet := toSet(agents.FrontmatterFields)
		for _, want := range []string{"slug", "roleDefinition", "whenToUse", "groups"} {
			if !fmSet[want] {
				t.Errorf("roo-code agents.frontmatterFields missing %q", want)
			}
		}
	})

	t.Run("codex-agents-no-yaml-frontmatter", func(t *testing.T) {
		prov := findProvider(t, manifest, "codex")
		agents := prov.Content["agents"]
		// Codex agents use TOML, not YAML frontmatter
		if len(agents.FrontmatterFields) > 0 {
			t.Errorf("codex agents.frontmatterFields = %v, want empty (Codex uses TOML)", agents.FrontmatterFields)
		}
	})

	t.Run("copilot-cli-skills", func(t *testing.T) {
		prov := findProvider(t, manifest, "copilot-cli")
		skills := prov.Content["skills"]
		fmSet := toSet(skills.FrontmatterFields)
		for _, want := range []string{"license", "argument-hint"} {
			if !fmSet[want] {
				t.Errorf("copilot-cli skills.frontmatterFields missing %q", want)
			}
		}
	})

	t.Run("kiro-skills", func(t *testing.T) {
		prov := findProvider(t, manifest, "kiro")
		skills := prov.Content["skills"]
		fmSet := toSet(skills.FrontmatterFields)
		for _, want := range []string{"license", "compatibility", "metadata"} {
			if !fmSet[want] {
				t.Errorf("kiro skills.frontmatterFields missing %q", want)
			}
		}
	})
}

// TestGenproviders_NoMachineSpecificPaths ensures no generated discovery paths
// contain the current user's home directory.
func TestGenproviders_NoMachineSpecificPaths(t *testing.T) {
	manifest := loadTestManifest(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	for _, prov := range manifest.Providers {
		for ctName, cap := range prov.Content {
			for _, p := range cap.DiscoveryPaths {
				if strings.Contains(p, home) {
					t.Errorf("provider %q content[%q] has machine-specific path: %q (contains %q)",
						prov.Slug, ctName, p, home)
				}
			}
		}
	}
}

// TestGenproviders_HookEventCategoryCompleteness verifies that every canonical
// hook event name present in converter.HookEvents has an entry in the
// hookEventCategory map. This catches new hook events being added to the
// converter without a matching docs category.
func TestGenproviders_HookEventCategoryCompleteness(t *testing.T) {
	for canonical := range converter.HookEvents {
		if _, ok := hookEventCategory[canonical]; !ok {
			t.Errorf("hook event %q exists in converter.HookEvents but has no entry in hookEventCategory", canonical)
		}
	}
}

// --- Helpers ---

func loadTestManifest(t *testing.T) ProviderManifest {
	t.Helper()
	origDir := providerFormatsDirForDocsURL
	providerFormatsDirForDocsURL = repoProviderFormatsDir(t)
	t.Cleanup(func() { providerFormatsDirForDocsURL = origDir })

	raw := captureStdout(t, func() {
		if err := genprovidersCmd.RunE(genprovidersCmd, nil); err != nil {
			t.Fatalf("_genproviders failed: %v", err)
		}
	})
	var manifest ProviderManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	return manifest
}

func findProvider(t *testing.T, manifest ProviderManifest, slug string) ProviderCapEntry {
	t.Helper()
	for _, p := range manifest.Providers {
		if p.Slug == slug {
			return p
		}
	}
	t.Fatalf("provider %q not found in manifest", slug)
	return ProviderCapEntry{} // unreachable
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}
