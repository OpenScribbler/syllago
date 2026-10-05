package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

const providerSettingsHooks = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo guard"}]}
    ]
  }
}`

const providerMCPServers = `{
  "mcpServers": {
    "db": {"command": "db-server", "env": {"TOKEN": "secret-db"}},
    "web": {"url": "https://example.test/mcp", "headers": {"Authorization": "Bearer secret-web"}}
  }
}`

// providerSettingsEnv isolates HOME and the install records, and writes a
// Claude Code project holding one hook and two MCP servers. It returns the
// project root, an empty Library, and the install records file.
func providerSettingsEnv(t *testing.T) (projectRoot, library, records string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	records = filepath.Join(withTUIInstallRecordConfigDir(t), "installs.json")
	projectRoot, library = t.TempDir(), t.TempDir()
	writeTUITestFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), []byte(providerSettingsHooks))
	writeTUITestFile(t, filepath.Join(projectRoot, ".mcp.json"), []byte(providerMCPServers))
	return projectRoot, library, records
}

// mergedProviderItems runs provider discovery through the wizard's triage
// merge, as the Review step sees the items, and returns them by name.
func mergedProviderItems(t *testing.T, projectRoot, library string) map[string]addDiscoveryItem {
	t.Helper()
	items, unread, err := discoverFromProvider(provider.ClaudeCode, projectRoot, nil, library, []catalog.ContentType{catalog.Hooks, catalog.MCP})
	if err != nil || len(unread) > 0 {
		t.Fatalf("discoverFromProvider: err=%v unread=%v", err, unread)
	}
	m := openAddWizard([]provider.Provider{provider.ClaudeCode}, nil, nil, projectRoot, library, "")
	m.handleDiscoveryDone(addDiscoveryDoneMsg{seq: m.seq, items: items})
	m.mergeConfirmIntoDiscovery()
	byName := map[string]addDiscoveryItem{}
	for _, it := range m.discoveredItems {
		byName[it.underlying.Name] = it
	}
	return byName
}

func riskLabels(risks []catalog.RiskIndicator) []string {
	var labels []string
	for _, r := range risks {
		labels = append(labels, r.Label)
	}
	return labels
}

func TestDiscoverFromProvider_ReadsSettingsEntries(t *testing.T) {
	projectRoot, library, _ := providerSettingsEnv(t)

	items, unread, err := discoverFromProvider(provider.ClaudeCode, projectRoot, nil, library, []catalog.ContentType{catalog.Hooks, catalog.MCP})
	if err != nil || len(unread) > 0 {
		t.Fatalf("discoverFromProvider: err=%v unread=%v", err, unread)
	}
	want := map[string]string{
		"db":  "Runs process,Environment variables",
		"web": "Network access",
	}
	got := map[string]string{}
	for _, it := range items {
		if it.settings == nil {
			t.Errorf("%s: no settings entry, so the add would copy %s whole", it.name, it.path)
			continue
		}
		if it.scope != "project" {
			t.Errorf("%s: scope = %q, want project", it.name, it.scope)
		}
		if !strings.HasSuffix(it.relativePath, "["+it.name+"]") {
			t.Errorf("%s: relativePath = %q, want the entry name appended", it.name, it.relativePath)
		}
		got[it.name] = strings.Join(riskLabels(it.risks), ",")
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want a hook and 2 servers", len(items))
	}
	for name, risks := range want {
		if got[name] != risks {
			t.Errorf("%s risks = %q, want %q", name, got[name], risks)
		}
	}
}

// Regression: an MCP server added from a provider's settings copied the
// whole settings file, so every other server and its secrets landed in the
// Library beside it.
func TestAddSingleItem_ProviderMCPAddsOnlyThatServer(t *testing.T) {
	projectRoot, library, _ := providerSettingsEnv(t)
	db := mergedProviderItems(t, projectRoot, library)["db"]

	result := addSingleItem(db, nil, library, projectRoot, "", "", "claude-code", "")
	if result.status != "added" {
		t.Fatalf("status = %q err=%v, want added", result.status, result.err)
	}
	dest := filepath.Join(library, string(catalog.MCP), "claude-code", "db")
	data, err := os.ReadFile(filepath.Join(dest, "config.json"))
	if err != nil {
		t.Fatalf("reading config.json: %v", err)
	}
	if !strings.Contains(string(data), "secret-db") || strings.Contains(string(data), "secret-web") {
		t.Errorf("config.json = %s, want the db server alone", data)
	}
}

// Regression: the triage merge dropped a settings hook's scope, so a hook
// added from the wizard recorded no source scope and a second add could not
// tell a project hook from a global one.
func TestAddSingleItem_ProviderHookKeepsItsScope(t *testing.T) {
	projectRoot, library, _ := providerSettingsEnv(t)
	var hook addDiscoveryItem
	for _, it := range mergedProviderItems(t, projectRoot, library) {
		if it.itemType == catalog.Hooks {
			hook = it
		}
	}
	if hook.underlying == nil {
		t.Fatal("no hook discovered")
	}

	result := addSingleItem(hook, nil, library, projectRoot, "", "", "claude-code", "")
	if result.status != "added" {
		t.Fatalf("status = %q err=%v, want added", result.status, result.err)
	}
	meta, err := metadata.Load(filepath.Join(library, string(catalog.Hooks), "claude-code", hook.underlying.Name))
	if err != nil || meta == nil {
		t.Fatalf("metadata.Load: meta=%v err=%v", meta, err)
	}
	if meta.SourceScope != "project" || meta.SourceName != hook.underlying.Name || meta.SourceProject != filepath.Base(projectRoot) {
		t.Errorf("meta scope=%q name=%q project=%q, want project, %q, %q",
			meta.SourceScope, meta.SourceName, meta.SourceProject, hook.underlying.Name, filepath.Base(projectRoot))
	}

	again := mergedProviderItems(t, projectRoot, library)[hook.underlying.Name]
	if again.status != add.StatusInLibrary {
		t.Errorf("second discovery status = %v, want the hook found in the Library", again.status)
	}
}

func TestAddSingleItem_ProviderSettingsLeavesPinnedItem(t *testing.T) {
	projectRoot, library, records := providerSettingsEnv(t)
	dest := filepath.Join(library, string(catalog.MCP), "claude-code", "db")
	writeTUITestFile(t, filepath.Join(dest, "config.json"), []byte(`{"mcpServers": {"db": {"command": "old"}}}`))
	if err := metadata.Save(dest, &metadata.Meta{Name: "db", SourceType: "registry", SourceRegistry: "acme/tools", SourceScope: "project", SourceName: "db"}); err != nil {
		t.Fatalf("metadata.Save: %v", err)
	}
	coord := installstore.Coord{Registry: "acme/tools", Type: string(catalog.MCP), Name: "db"}
	if err := installstore.RecordInstallMeta(records, coord, dest, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismMCPMerge,
		Path:      filepath.Join(t.TempDir(), ".mcp.json"),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	if err := installstore.SetPinned(records, coord, true, time.Date(2026, 10, 4, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	db := mergedProviderItems(t, projectRoot, library)["db"]
	db.overwrite = true

	result := addSingleItem(db, nil, library, projectRoot, "", "", "claude-code", "")
	if result.status != "pinned" {
		t.Fatalf("status = %q err=%v, want pinned", result.status, result.err)
	}
	data, _ := os.ReadFile(filepath.Join(dest, "config.json"))
	if !strings.Contains(string(data), `"old"`) {
		t.Errorf("config.json = %s, want the pinned copy left as it was", data)
	}
}

// Regression: a server placed at a -N directory was checked for a pin under
// that directory's name, while its install record is keyed on the server's
// name, so a forced add replaced a pinned server.
func TestAddSingleItem_ProviderSettingsLeavesPinnedSuffixedServer(t *testing.T) {
	projectRoot, library, records := providerSettingsEnv(t)
	global := filepath.Join(library, string(catalog.MCP), "claude-code", "db")
	writeTUITestFile(t, filepath.Join(global, "config.json"), []byte(`{"mcpServers": {"db": {"command": "global"}}}`))
	if err := metadata.Save(global, &metadata.Meta{Name: "db", SourceScope: "global", SourceName: "db"}); err != nil {
		t.Fatalf("metadata.Save: %v", err)
	}
	dest := filepath.Join(library, string(catalog.MCP), "claude-code", "db-2")
	writeTUITestFile(t, filepath.Join(dest, "config.json"), []byte(`{"mcpServers": {"db": {"command": "old"}}}`))
	if err := metadata.Save(dest, &metadata.Meta{Name: "db", SourceType: "registry", SourceRegistry: "acme/tools", SourceScope: "project", SourceName: "db"}); err != nil {
		t.Fatalf("metadata.Save: %v", err)
	}
	coord := installstore.Coord{Registry: "acme/tools", Type: string(catalog.MCP), Name: "db"}
	if err := installstore.RecordInstallMeta(records, coord, dest, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismMCPMerge,
		Path:      filepath.Join(t.TempDir(), ".mcp.json"),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	if err := installstore.SetPinned(records, coord, true, time.Date(2026, 10, 4, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	db := mergedProviderItems(t, projectRoot, library)["db"]
	if db.settings == nil || db.settings.Dest != dest {
		t.Fatalf("db placed at %v, want %s", db.settings, dest)
	}
	db.overwrite = true

	result := addSingleItem(db, nil, library, projectRoot, "", "", "claude-code", "")
	if result.status != "pinned" {
		t.Fatalf("status = %q err=%v, want pinned", result.status, result.err)
	}
	data, _ := os.ReadFile(filepath.Join(dest, "config.json"))
	if !strings.Contains(string(data), `"old"`) {
		t.Errorf("config.json = %s, want the pinned copy left as it was", data)
	}
}

func TestApp_AddDiscoveryWarnsOfUnreadSettings(t *testing.T) {
	app := testAppWithItems(t)
	m, _ := app.Update(keyRune('a'))
	a := m.(App)
	if a.addWizard == nil {
		t.Fatal("expected addWizard not nil")
	}
	a.toast.queue = nil
	unread := []error{errors.New("parsing settings.json: invalid JSON")}

	// A discovery the wizard has moved past is not reported.
	m, _ = a.Update(addDiscoveryDoneMsg{seq: a.addWizard.seq - 1, warnings: unread})
	a = m.(App)
	if len(a.toast.queue) != 0 {
		t.Fatalf("stale discovery queued toasts %+v", a.toast.queue)
	}

	m, _ = a.Update(addDiscoveryDoneMsg{seq: a.addWizard.seq, warnings: unread})
	a = m.(App)
	if len(a.toast.queue) != 1 || a.toast.queue[0].level != toastWarning {
		t.Fatalf("toasts = %+v, want one warning", a.toast.queue)
	}
	if got := a.toast.queue[0].details; len(got) != 1 || !strings.Contains(got[0], "settings.json") {
		t.Errorf("details = %v, want the unread file named", got)
	}
}

// Regression: the wizard adds one item per command, and each settings item
// was placed alone, so a second same-scope server named db landed on the
// first one's directory and was skipped as already added.
func TestAddItemCmd_PlacesSettingsItemsAfterEarlierOnes(t *testing.T) {
	projectRoot, library, _ := providerSettingsEnv(t)
	writeTUITestFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), []byte(`{"mcpServers": {"db": {"command": "from-settings"}}}`))
	writeTUITestFile(t, filepath.Join(projectRoot, ".mcp.json"), []byte(`{"mcpServers": {"db": {"command": "from-mcp-json"}}}`))
	items, unread := discoverSettingsFromProvider(provider.ClaudeCode, projectRoot, "", library, catalog.MCP)
	if len(items) != 2 || len(unread) > 0 {
		t.Fatalf("discovered %d items, unread %v; want two db servers", len(items), unread)
	}

	m := testOpenAddWizard(t)
	m.source = addSourceProvider
	m.providers = []provider.Provider{provider.ClaudeCode}
	m.providerCursor = 0
	m.contentRoot, m.projectRoot = library, projectRoot
	m.discoveredItems = items
	m.actionableCount = len(items)
	m.discoveryList = m.buildDiscoveryList()
	for i := range items {
		msg := m.addItemCmd(i)().(addExecItemDoneMsg)
		if msg.result.status != "added" {
			t.Fatalf("item %d: status %q err %v, want added", i, msg.result.status, msg.result.err)
		}
	}
	for _, dir := range []string{"db", "db-2"} {
		if _, err := os.Stat(filepath.Join(library, string(catalog.MCP), "claude-code", dir, "config.json")); err != nil {
			t.Errorf("%s: %v", dir, err)
		}
	}
}

// Regression: a broken settings file that holds both hooks and servers was
// reported once per content type, so one file read as two.
func TestDiscoverFromProvider_ReportsABrokenFileOnce(t *testing.T) {
	projectRoot, library, _ := providerSettingsEnv(t)
	writeTUITestFile(t, filepath.Join(projectRoot, ".claude", "settings.json"), []byte(`{"mcpServers": {"db": {"command": "db-server"}}, "hooks": `))
	_, unread, err := discoverFromProvider(provider.ClaudeCode, projectRoot, nil, library, []catalog.ContentType{catalog.Hooks, catalog.MCP})
	if err != nil {
		t.Fatalf("discoverFromProvider: %v", err)
	}
	if len(unread) != 1 {
		t.Errorf("unread = %v, want the broken file once", unread)
	}
}
