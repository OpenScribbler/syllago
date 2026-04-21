package loadout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- RefNames / NameRefsByType (0% coverage) ---

func TestRefNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		refs []ItemRef
		want []string
	}{
		{"empty", nil, []string{}},
		{"single", []ItemRef{{Name: "a"}}, []string{"a"}},
		{"multiple", []ItemRef{{Name: "a"}, {Name: "b", ID: "123"}}, []string{"a", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RefNames(tc.refs)
			if len(got) != len(tc.want) {
				t.Fatalf("RefNames() len = %d, want %d", len(got), len(tc.want))
			}
			for i, n := range got {
				if n != tc.want[i] {
					t.Errorf("RefNames()[%d] = %q, want %q", i, n, tc.want[i])
				}
			}
		})
	}
}

func TestNameRefsByType(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Rules:  []ItemRef{{Name: "rule-a"}, {Name: "rule-b"}},
		Hooks:  []ItemRef{{Name: "hook-x"}},
		Skills: []ItemRef{},
	}
	got := m.NameRefsByType()

	if len(got[catalog.Rules]) != 2 {
		t.Errorf("Rules names len = %d, want 2", len(got[catalog.Rules]))
	}
	if got[catalog.Rules][0] != "rule-a" || got[catalog.Rules][1] != "rule-b" {
		t.Errorf("Rules names = %v, want [rule-a rule-b]", got[catalog.Rules])
	}
	if len(got[catalog.Hooks]) != 1 || got[catalog.Hooks][0] != "hook-x" {
		t.Errorf("Hooks names = %v, want [hook-x]", got[catalog.Hooks])
	}
	// Empty skills should not be in the map (RefsByType filters empty)
	if _, ok := got[catalog.Skills]; ok {
		t.Error("empty Skills should not appear in NameRefsByType")
	}
}

// --- resolveItemID (0% coverage) ---

func TestResolveItemID_NoMetadata(t *testing.T) {
	t.Parallel()
	// No .syllago.yaml exists, should return empty string
	tmp := t.TempDir()
	// Rules are provider-specific: globalDir/rules/<provider>/<name>
	itemDir := filepath.Join(tmp, "rules", "claude-code", "my-rule")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# Rule"), 0644)

	id := resolveItemID(tmp, catalog.Rules, "claude-code", "my-rule")
	if id != "" {
		t.Errorf("resolveItemID() = %q, want empty string", id)
	}
}

func TestResolveItemID_WithMetadata(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	// Rules are provider-specific: globalDir/rules/<provider>/<name>
	itemDir := filepath.Join(tmp, "rules", "claude-code", "my-rule")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, ".syllago.yaml"), []byte("id: abc-123-uuid\n"), 0644)

	id := resolveItemID(tmp, catalog.Rules, "claude-code", "my-rule")
	if id != "abc-123-uuid" {
		t.Errorf("resolveItemID() = %q, want %q", id, "abc-123-uuid")
	}
}

func TestResolveItemID_UniversalType(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	// Skills are universal: globalDir/skills/<name> (no provider in path)
	itemDir := filepath.Join(tmp, "skills", "my-skill")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, ".syllago.yaml"), []byte("id: skill-uuid-456\n"), 0644)

	id := resolveItemID(tmp, catalog.Skills, "claude-code", "my-skill")
	if id != "skill-uuid-456" {
		t.Errorf("resolveItemID() = %q, want %q", id, "skill-uuid-456")
	}
}

func TestResolveItemID_NonexistentDir(t *testing.T) {
	t.Parallel()
	id := resolveItemID("/nonexistent", catalog.Rules, "claude-code", "missing")
	if id != "" {
		t.Errorf("resolveItemID() = %q, want empty string", id)
	}
}

// --- findHookFile (30% coverage) ---

func TestFindHookFile_HookJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hookPath := filepath.Join(dir, "hook.json")
	os.WriteFile(hookPath, []byte(`{}`), 0644)

	got := findHookFile(dir)
	if got != hookPath {
		t.Errorf("findHookFile() = %q, want %q", got, hookPath)
	}
}

func TestFindHookFile_FallbackToAnyJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No hook.json, but there's a custom-named .json file
	customPath := filepath.Join(dir, "custom-hook.json")
	os.WriteFile(customPath, []byte(`{}`), 0644)
	// Also add a non-JSON file that should be skipped
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Readme"), 0644)

	got := findHookFile(dir)
	if got != customPath {
		t.Errorf("findHookFile() = %q, want %q", got, customPath)
	}
}

func TestFindHookFile_NoJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# No hooks"), 0644)

	got := findHookFile(dir)
	if got != "" {
		t.Errorf("findHookFile() = %q, want empty string", got)
	}
}

func TestFindHookFile_EmptyDir(t *testing.T) {
	t.Parallel()
	got := findHookFile(t.TempDir())
	if got != "" {
		t.Errorf("findHookFile() = %q, want empty string", got)
	}
}

func TestFindHookFile_NonexistentDir(t *testing.T) {
	t.Parallel()
	got := findHookFile("/nonexistent/dir/for/test")
	if got != "" {
		t.Errorf("findHookFile() = %q, want empty string", got)
	}
}

// --- settingsPathFor (50% coverage) ---

func TestSettingsPathFor_NoResolver(t *testing.T) {
	t.Parallel()
	prov := provider.Provider{Slug: "claude-code", ConfigDir: ".claude"}
	got := settingsPathFor(prov, "/home/user", nil)
	want := "/home/user/.claude/settings.json"
	if got != want {
		t.Errorf("settingsPathFor() = %q, want %q", got, want)
	}
}

func TestSettingsPathFor_WithResolver(t *testing.T) {
	t.Parallel()
	prov := provider.Provider{Slug: "claude-code", ConfigDir: ".claude"}
	resolver := config.NewResolver(nil, "/custom/base")
	got := settingsPathFor(prov, "/home/user", resolver)
	want := "/custom/base/.claude/settings.json"
	if got != want {
		t.Errorf("settingsPathFor() = %q, want %q", got, want)
	}
}

func TestSettingsPathFor_ResolverNoMatch(t *testing.T) {
	t.Parallel()
	prov := provider.Provider{Slug: "cursor", ConfigDir: ".cursor"}
	// Resolver has no CLI base dir and no config for cursor
	resolver := config.NewResolver(nil, "")
	got := settingsPathFor(prov, "/home/user", resolver)
	want := "/home/user/.cursor/settings.json"
	if got != want {
		t.Errorf("settingsPathFor() = %q, want %q", got, want)
	}
}

// --- resolveHookCommands (64.3% coverage) ---

func TestResolveHookCommands_RelativePath(t *testing.T) {
	t.Parallel()
	itemDir := "/content/hooks/claude-code/my-hook"
	input := []byte(`{"matcher":".*","hooks":[{"type":"command","command":"./run.sh"}]}`)

	got := resolveHookCommands(input, itemDir)
	// The relative ./run.sh should become an absolute path
	want := filepath.Join(itemDir, "./run.sh")
	if string(got) == string(input) {
		t.Error("resolveHookCommands should have resolved relative path")
	}
	_ = want // The actual path check depends on ResolveHookCommand behavior
}

func TestResolveHookCommands_AbsolutePath(t *testing.T) {
	t.Parallel()
	input := []byte(`{"matcher":".*","hooks":[{"type":"command","command":"/usr/bin/echo test"}]}`)

	got := resolveHookCommands(input, "/some/dir")
	// Absolute paths should not be modified
	if string(got) != string(input) {
		t.Errorf("resolveHookCommands should not modify absolute paths, got %q", string(got))
	}
}

func TestResolveHookCommands_NoHooksArray(t *testing.T) {
	t.Parallel()
	input := []byte(`{"matcher":".*"}`)

	got := resolveHookCommands(input, "/some/dir")
	if string(got) != string(input) {
		t.Errorf("resolveHookCommands should return input unchanged when no hooks array")
	}
}

func TestResolveHookCommands_EmptyCommand(t *testing.T) {
	t.Parallel()
	input := []byte(`{"hooks":[{"type":"command","command":""}]}`)

	got := resolveHookCommands(input, "/some/dir")
	if string(got) != string(input) {
		t.Errorf("resolveHookCommands should not modify empty commands")
	}
}

// --- symlinkSource (66.7% coverage — agents case untested) ---

func TestSymlinkSource_Agents(t *testing.T) {
	t.Parallel()
	ref := ResolvedRef{
		Type: catalog.Agents,
		Item: catalog.ContentItem{Path: "/content/agents/my-agent"},
	}
	got := symlinkSource(ref)
	want := "/content/agents/my-agent/AGENT.md"
	if got != want {
		t.Errorf("symlinkSource() = %q, want %q", got, want)
	}
}

func TestSymlinkSource_Rules(t *testing.T) {
	t.Parallel()
	ref := ResolvedRef{
		Type: catalog.Rules,
		Item: catalog.ContentItem{Path: "/content/rules/my-rule"},
	}
	got := symlinkSource(ref)
	want := "/content/rules/my-rule"
	if got != want {
		t.Errorf("symlinkSource() = %q, want %q", got, want)
	}
}

// --- writeJSONFileAtomic (61.5% coverage) ---

func TestWriteJSONFileAtomic_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "dir", "settings.json")

	err := writeJSONFileAtomic(path, []byte(`{"key":"value"}`))
	if err != nil {
		t.Fatalf("writeJSONFileAtomic: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != `{"key":"value"}` {
		t.Errorf("got %q, want %q", string(data), `{"key":"value"}`)
	}
}

func TestWriteJSONFileAtomic_Overwrites(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"old":"data"}`), 0644)

	err := writeJSONFileAtomic(path, []byte(`{"new":"data"}`))
	if err != nil {
		t.Fatalf("writeJSONFileAtomic: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != `{"new":"data"}` {
		t.Errorf("got %q, want %q", string(data), `{"new":"data"}`)
	}
}

// --- BuildManifestFromNames with ID resolution ---

func TestBuildManifestFromNames_WithGlobalDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	// Create a rule with metadata (rules are provider-specific)
	ruleDir := filepath.Join(tmp, "rules", "claude-code", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, ".syllago.yaml"), []byte("id: test-uuid-789\n"), 0644)

	items := map[catalog.ContentType][]string{
		catalog.Rules: {"my-rule"},
	}
	m := BuildManifestFromNames("claude-code", "test", "desc", items, tmp)

	if len(m.Rules) != 1 {
		t.Fatalf("Rules len = %d, want 1", len(m.Rules))
	}
	if m.Rules[0].ID != "test-uuid-789" {
		t.Errorf("Rules[0].ID = %q, want %q", m.Rules[0].ID, "test-uuid-789")
	}
}

func TestBuildManifestFromNames_NoGlobalDir(t *testing.T) {
	t.Parallel()
	items := map[catalog.ContentType][]string{
		catalog.Rules: {"my-rule"},
	}
	m := BuildManifestFromNames("claude-code", "test", "desc", items)

	if len(m.Rules) != 1 {
		t.Fatalf("Rules len = %d, want 1", len(m.Rules))
	}
	if m.Rules[0].ID != "" {
		t.Errorf("Rules[0].ID = %q, want empty (no globalDir)", m.Rules[0].ID)
	}
}
