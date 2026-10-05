package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// localSettingsFolder writes a folder holding a Claude Code settings hook
// with its script, a .mcp.json with two servers, and a rule.
func localSettingsFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTUITestFile(t, filepath.Join(dir, ".claude", "settings.json"), []byte(`{"hooks": {"PostToolUse": [{"matcher": "Write", "hooks": [{"type": "command", "command": "./scripts/lint.sh"}]}]}}`))
	writeTUITestFile(t, filepath.Join(dir, ".claude", "scripts", "lint.sh"), []byte("#!/bin/sh\nexit 0\n"))
	writeTUITestFile(t, filepath.Join(dir, ".mcp.json"), []byte(`{"mcpServers": {"db": {"command": "db-server"}, "web": {"command": "web-server"}}}`))
	writeTUITestFile(t, filepath.Join(dir, ".claude", "rules", "style.md"), []byte("# Style\nUse tabs.\n"))
	return dir
}

func itemsOfType(items []addDiscoveryItem, ct catalog.ContentType) []addDiscoveryItem {
	var out []addDiscoveryItem
	for _, it := range items {
		if it.itemType == ct {
			out = append(out, it)
		}
	}
	return out
}

// Regression: a folder's settings hooks and .mcp.json servers were listed
// twice, once from the provider pattern scan and once from the analyzer,
// and the analyzer's copy of a server added the whole .mcp.json.
func TestDiscoverFromLocalPath_SettingsEntriesListedOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := localSettingsFolder(t)
	items, confirm, unread, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Hooks, catalog.MCP}, t.TempDir())
	if err != nil || len(unread) > 0 {
		t.Fatalf("discoverFromLocalPath: err %v unread %v", err, unread)
	}
	for _, c := range confirm {
		t.Errorf("confirm item %s %s, want every entry listed once as a settings item", c.itemType, c.displayName)
	}
	if hooks := itemsOfType(items, catalog.Hooks); len(hooks) != 1 || hooks[0].settings == nil {
		t.Errorf("hooks = %d, want one settings item", len(hooks))
	}
	servers := itemsOfType(items, catalog.MCP)
	if len(servers) != 2 {
		t.Fatalf("servers = %d, want two", len(servers))
	}
	for _, s := range servers {
		if s.settings == nil {
			t.Errorf("server %s is not a settings item", s.name)
		}
	}
}

// Regression: a server added from a folder's .mcp.json copied the whole
// file, so each Library item held every server in it.
func TestAddSingleItem_LocalServersAddedAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := localSettingsFolder(t)
	library := t.TempDir()
	items, _, _, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.MCP}, library)
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}
	for _, s := range itemsOfType(items, catalog.MCP) {
		if r := addSingleItem(s, library, dir, "", "", "", ""); r.status != "added" {
			t.Fatalf("%s: status %q err %v", s.name, r.status, r.err)
		}
	}
	for name, other := range map[string]string{"db": "web-server", "web": "db-server"} {
		data, err := os.ReadFile(filepath.Join(library, string(catalog.MCP), "claude-code", name, "config.json"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(string(data), name+"-server") || strings.Contains(string(data), other) {
			t.Errorf("%s/config.json = %s, want its own server alone", name, data)
		}
	}
}

// Regression: a rule found in a folder's provider layout was added with no
// provider, so it landed at rules/<name> where the Library does not list
// provider rules.
func TestAddSingleItem_LocalRuleLandsUnderItsProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := localSettingsFolder(t)
	library := t.TempDir()
	items, confirm, _, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, library)
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}
	m := testOpenAddWizard(t)
	m.handleDiscoveryDone(addDiscoveryDoneMsg{seq: m.seq, items: items, confirmItems: confirm})
	m.mergeConfirmIntoDiscovery()
	rules := itemsOfType(m.discoveredItems, catalog.Rules)
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want one", len(rules))
	}
	if r := addSingleItem(rules[0], library, dir, "", "", "", ""); r.status != "added" {
		t.Fatalf("status %q err %v", r.status, r.err)
	}
	if _, err := os.Stat(filepath.Join(library, string(catalog.Rules), "claude-code", "style")); err != nil {
		t.Errorf("rule not under rules/claude-code: %v", err)
	}

	// Found again, the rule is known to be in the Library.
	items, _, _, err = discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, library)
	if err != nil {
		t.Fatalf("second discovery: %v", err)
	}
	if rules := itemsOfType(items, catalog.Rules); len(rules) != 1 || rules[0].status != add.StatusInLibrary {
		t.Errorf("rediscovered rule status = %v, want in library", rules[0].status)
	}

	// Changed since, it is outdated.
	writeTUITestFile(t, filepath.Join(dir, ".claude", "rules", "style.md"), []byte("# Style\nUse spaces.\n"))
	items, _, _, err = discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, library)
	if err != nil {
		t.Fatalf("third discovery: %v", err)
	}
	if rules := itemsOfType(items, catalog.Rules); len(rules) != 1 || rules[0].status != add.StatusOutdated {
		t.Errorf("changed rule status = %v, want outdated", rules[0].status)
	}
}

// Regression: a folder's settings file that could not be parsed was
// dropped without a word.
func TestDiscoverFromLocalPath_ReportsUnreadSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	// The trailing comma leaves the hooks readable to the pattern scan and
	// the file unreadable as JSON.
	writeTUITestFile(t, filepath.Join(dir, ".claude", "settings.json"), []byte(`{"hooks": {"PostToolUse": [{"hooks": [{"type": "command", "command": "x"}]}]}, }`))
	_, _, unread, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Hooks}, t.TempDir())
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}
	if len(unread) != 1 || !strings.Contains(unread[0].Error(), "settings.json") {
		t.Errorf("unread = %v, want the broken settings file", unread)
	}
}

// A project-scope settings item from a folder records that folder as its
// project, and one from a clone, which is deleted after the add, none.
func TestAddItemCmd_LocalSettingsRecordTheFolder(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source addSource
		want   func(dir string) string
	}{
		{"local", addSourceLocal, filepath.Base},
		{"git", addSourceGit, func(string) string { return "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dir := localSettingsFolder(t)
			library := t.TempDir()
			items, _, _, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.MCP}, library)
			if err != nil {
				t.Fatalf("discoverFromLocalPath: %v", err)
			}
			m := testOpenAddWizard(t)
			m.source = tc.source
			m.pathInput = dir
			m.contentRoot, m.projectRoot = library, t.TempDir()
			m.discoveredItems = items
			m.actionableCount = len(items)
			m.discoveryList = m.buildDiscoveryList()
			msg := m.addItemCmd(0)().(addExecItemDoneMsg)
			if msg.result.status != "added" {
				t.Fatalf("status %q err %v", msg.result.status, msg.result.err)
			}
			meta, err := metadata.Load(filepath.Join(library, string(catalog.MCP), "claude-code", "db"))
			if err != nil || meta == nil {
				t.Fatalf("metadata.Load: %v", err)
			}
			if want := tc.want(dir); meta.SourceProject != want {
				t.Errorf("SourceProject = %q, want %q", meta.SourceProject, want)
			}
		})
	}
}

// A settings file the shared reader finds no entries in, such as Cursor's
// hooks.json, is left to the analyzer's items rather than dropped.
func TestDiscoverSettingsFromFolder_FileWithoutEntriesStaysUncovered(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	writeTUITestFile(t, filepath.Join(dir, ".cursor", "hooks.json"), []byte(`{"version": 1, "hooks": {"afterFileEdit": [{"command": "./fmt.sh"}]}}`))
	confirm := []*analyzer.DetectedItem{{Name: "afterFileEdit", Type: catalog.Hooks, Provider: "cursor", Path: ".cursor/hooks.json"}}
	items, covered, _ := discoverSettingsFromFolder(dir, catalog.NativeScanResult{}, confirm, map[catalog.ContentType]bool{catalog.Hooks: true}, t.TempDir())
	if len(items) != 0 || covered["hooks/.cursor/hooks.json"] {
		t.Errorf("items %d covered %v, want the file left to the analyzer", len(items), covered)
	}
}

// Regression: a provider rule found by the folder's layout was looked up
// at rules/<name>, where the Library never holds it, so it read as new
// after it was added.
func TestDiscoverFromLocalPath_LayoutRuleInLibraryOnceAdded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	writeTUITestFile(t, filepath.Join(dir, ".cursor", "rules", "style.mdc"), []byte("---\ndescription: Style\n---\nUse tabs.\n"))
	library := t.TempDir()
	items, _, _, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, library)
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}
	rules := itemsOfType(items, catalog.Rules)
	if len(rules) != 1 || rules[0].detectionSource == "content-signal" {
		t.Fatalf("rules = %d, want one found by the layout", len(rules))
	}
	if r := addSingleItem(rules[0], library, dir, "", "", "", ""); r.status != "added" {
		t.Fatalf("status %q err %v", r.status, r.err)
	}
	items, _, _, err = discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, library)
	if err != nil {
		t.Fatalf("second discovery: %v", err)
	}
	if rules := itemsOfType(items, catalog.Rules); len(rules) != 1 || rules[0].status != add.StatusInLibrary {
		t.Errorf("rediscovered rule status = %v, want in library", rules[0].status)
	}

	// Regression: changed since, it still read as in the Library, so it
	// left the list of items to add.
	writeTUITestFile(t, filepath.Join(dir, ".cursor", "rules", "style.mdc"), []byte("---\ndescription: Style\n---\nUse spaces.\n"))
	items, _, _, err = discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, library)
	if err != nil {
		t.Fatalf("third discovery: %v", err)
	}
	if rules := itemsOfType(items, catalog.Rules); len(rules) != 1 || rules[0].status != add.StatusOutdated {
		t.Errorf("changed rule status = %v, want outdated", rules[0].status)
	}
}

// Regression: a folder's hook copied any script its command named, so a
// command reaching outside the folder copied the user's own file into the
// Library.
func TestAddSingleItem_LocalHookBringsOnlyItsFolderScripts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeTUITestFile(t, filepath.Join(home, "secret.sh"), []byte("#!/bin/sh\necho secret\n"))
	outside := t.TempDir()
	writeTUITestFile(t, filepath.Join(outside, "far.sh"), []byte("#!/bin/sh\necho far\n"))
	dir := t.TempDir()
	writeTUITestFile(t, filepath.Join(dir, ".claude", "scripts", "lint.sh"), []byte("#!/bin/sh\nexit 0\n"))
	if err := os.Symlink(filepath.Join(outside, "far.sh"), filepath.Join(dir, ".claude", "scripts", "link.sh")); err != nil {
		t.Fatal(err)
	}
	writeTUITestFile(t, filepath.Join(dir, ".claude", "settings.json"), []byte(`{"hooks": {"PostToolUse": [
		{"matcher": "Write", "hooks": [{"type": "command", "command": "./scripts/lint.sh"}]},
		{"matcher": "Edit", "hooks": [{"type": "command", "command": "bash ~/secret.sh"}]},
		{"matcher": "Read", "hooks": [{"type": "command", "command": "bash `+filepath.Join(outside, "far.sh")+`"}]},
		{"matcher": "Glob", "hooks": [{"type": "command", "command": "./scripts/link.sh"}]}
	]}}`))
	library := t.TempDir()
	items, _, _, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Hooks}, library)
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}
	hooks := itemsOfType(items, catalog.Hooks)
	if len(hooks) != 4 {
		t.Fatalf("hooks = %d, want four", len(hooks))
	}
	var copied []string
	for _, h := range hooks {
		if r := addSingleItem(h, library, dir, "", "", "", ""); r.status != "added" {
			t.Fatalf("%s: status %q err %v", h.name, r.status, r.err)
		}
		files, _ := filepath.Glob(filepath.Join(h.settings.Dest, "*.sh"))
		for _, f := range files {
			copied = append(copied, filepath.Base(f))
		}
	}
	if len(copied) != 1 || copied[0] != "lint.sh" {
		t.Errorf("scripts copied = %v, want lint.sh alone", copied)
	}
}
