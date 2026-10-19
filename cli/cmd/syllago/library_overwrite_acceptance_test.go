package main

// Acceptance tests: every CLI write over an existing Library item respects
// pins and rotates the install record of the item it replaces.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// --- shared helpers ---

// accSnapshot returns every file under dir, dotfiles included, by relative
// path.
func accSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	if len(files) == 0 {
		t.Fatalf("snapshot %s: no files; the Library item was not set up", dir)
	}
	return files
}

// accSeedRecord gives the Library item at libPath an install record at
// source commit sha-old, pinned when pinned is set, and returns its coord
// and content hash.
func accSeedRecord(t *testing.T, configDir string, ct catalog.ContentType, name, libPath string, pinned bool) (installstore.Coord, string) {
	t.Helper()
	storePath := filepath.Join(configDir, "installs.json")
	coord := installstore.Coord{Type: string(ct), Name: name}
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	if err := installstore.RecordInstallMeta(storePath, coord, libPath, installstore.PlacementInput{
		Provider: "claude-code", Mechanism: installstore.MechanismSymlink, Path: filepath.Join(t.TempDir(), name),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, now); err != nil {
		t.Fatalf("RecordInstallMeta %s: %v", name, err)
	}
	if pinned {
		if err := installstore.SetPinned(storePath, coord, true, now); err != nil {
			t.Fatalf("SetPinned %s: %v", name, err)
		}
	}
	hash, err := installstore.HashContent(libPath)
	if err != nil {
		t.Fatalf("HashContent %s: %v", libPath, err)
	}
	return coord, hash
}

func accRecord(t *testing.T, configDir string, coord installstore.Coord) *installstore.Record {
	t.Helper()
	store, err := installstore.Load(filepath.Join(configDir, "installs.json"))
	if err != nil {
		t.Fatalf("load installs.json: %v", err)
	}
	return store.Find(coord)
}

// accAssertHeld checks a pinned item kept its files and its record.
func accAssertHeld(t *testing.T, configDir string, coord installstore.Coord, libPath string, before map[string]string, oldHash string) {
	t.Helper()
	after := accSnapshot(t, libPath)
	for rel, data := range before {
		if after[rel] != data {
			t.Errorf("pinned %s: %s changed:\n before %q\n after  %q", coord.Name, rel, data, after[rel])
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			t.Errorf("pinned %s: new file %s written", coord.Name, rel)
		}
	}
	rec := accRecord(t, configDir, coord)
	if rec == nil {
		t.Fatalf("pinned %s: install record gone", coord.Name)
	}
	if !rec.Pinned || rec.Previous != nil || rec.ContentHash != oldHash || rec.SourceSHA != "sha-old" {
		t.Errorf("pinned %s: record = %+v, want it pinned and unchanged at %s / sha-old", coord.Name, rec, oldHash)
	}
}

// accAssertRotated checks a replaced item's record moved its old version to
// Previous and moved off the old content hash.
func accAssertRotated(t *testing.T, configDir string, coord installstore.Coord, libPath, oldHash string) {
	t.Helper()
	newHash, err := installstore.HashContent(libPath)
	if err != nil {
		t.Fatalf("HashContent %s: %v", libPath, err)
	}
	if newHash == oldHash {
		t.Fatalf("%s: Library content unchanged, want it replaced", coord.Name)
	}
	rec := accRecord(t, configDir, coord)
	if rec == nil {
		t.Fatalf("%s: install record gone", coord.Name)
	}
	if rec.Previous == nil {
		t.Fatalf("%s: record has no Previous; the replaced version was not kept", coord.Name)
	}
	if rec.Previous.ContentHash != oldHash || rec.Previous.SourceSHA != "sha-old" {
		t.Errorf("%s: Previous = %+v, want content hash %s at sha-old", coord.Name, rec.Previous, oldHash)
	}
	if rec.ContentHash == oldHash {
		t.Errorf("%s: record content hash still %s, want it moved to the new content", coord.Name, oldHash)
	}
}

func accAssertConflict(t *testing.T, err error) {
	t.Helper()
	var se output.StructuredError
	if !errors.As(err, &se) || se.Code != output.ErrInstallConflict {
		t.Fatalf("err = %v, want %s", err, output.ErrInstallConflict)
	}
}

func accAssertReported(t *testing.T, stderr, name string) {
	t.Helper()
	if !strings.Contains(stderr, name) || !strings.Contains(strings.ToLower(stderr), "pinned") {
		t.Errorf("stderr = %q, want pinned %s reported", stderr, name)
	}
}

func accResetArrayFlag(name string) {
	f := addCmd.Flags().Lookup(name)
	if f == nil {
		return
	}
	type sliceValue interface{ Replace([]string) error }
	if sv, ok := f.Value.(sliceValue); ok {
		_ = sv.Replace([]string{})
	}
	f.Changed = false
}

// accRunAdd runs `syllago add <args>` from projectRoot with the given flags
// and returns its stderr and error.
func accRunAdd(t *testing.T, projectRoot string, args []string, flags map[string]string) (string, error) {
	t.Helper()
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	defer func() { findProjectRoot = origRoot }()
	_, stderr := output.SetForTest(t)
	reset := func() {
		accResetArrayFlag("from")
		accResetArrayFlag("exclude")
		for name, def := range map[string]string{"all": "false", "dry-run": "false", "force": "false", "scope": "all", "split": "", "name": "", "install": "false", "to": "", "frozen": "false", "base-dir": ""} {
			addCmd.Flags().Set(name, def)
			addCmd.Flags().Lookup(name).Changed = false
		}
	}
	reset()
	defer reset()
	for name, value := range flags {
		if err := addCmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s=%s: %v", name, value, err)
		}
	}
	err := addCmd.RunE(addCmd, args)
	return stderr.String(), err
}

// accEnv isolates HOME, the Library, and the install records.
func accEnv(t *testing.T) (projectRoot, globalDir, configDir string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	projectRoot, globalDir = t.TempDir(), t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
	configDir = withInstallRecordConfigDir(t)
	return projectRoot, globalDir, configDir
}

func accWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- MCP and hooks: add hooks|mcp --from <provider> --force ---

func accMCPSettings(version string) string {
	return `{"mcpServers": {
  "heldsrv": {"command": "held-server", "args": ["` + version + `"]},
  "freshsrv": {"command": "fresh-server", "args": ["` + version + `"]}
}}`
}

// accSeedMCP adds both MCP servers at v1 and points the settings at v2.
func accSeedMCP(t *testing.T) (projectRoot, globalDir, configDir string) {
	t.Helper()
	projectRoot, globalDir, configDir = accEnv(t)
	settings := filepath.Join(projectRoot, ".claude", "settings.json")
	accWrite(t, settings, accMCPSettings("v1"))
	if _, err := accRunAdd(t, projectRoot, []string{"mcp"}, map[string]string{"from": "claude-code", "scope": "project"}); err != nil {
		t.Fatalf("seed add mcp: %v", err)
	}
	accWrite(t, settings, accMCPSettings("v2"))
	return projectRoot, globalDir, configDir
}

func accMCPPath(globalDir, name string) string {
	return filepath.Join(globalDir, string(catalog.MCP), "claude-code", name)
}

func TestAcceptance_AddMCPForce_PinnedHeldOthersReplaced(t *testing.T) {
	projectRoot, globalDir, configDir := accSeedMCP(t)
	heldPath, freshPath := accMCPPath(globalDir, "heldsrv"), accMCPPath(globalDir, "freshsrv")
	heldCoord, heldHash := accSeedRecord(t, configDir, catalog.MCP, "heldsrv", heldPath, true)
	freshCoord, freshHash := accSeedRecord(t, configDir, catalog.MCP, "freshsrv", freshPath, false)
	before := accSnapshot(t, heldPath)

	stderr, err := accRunAdd(t, projectRoot, []string{"mcp"}, map[string]string{"from": "claude-code", "scope": "project", "force": "true"})
	if err != nil {
		t.Fatalf("add mcp --force with one item unpinned: %v", err)
	}
	accAssertHeld(t, configDir, heldCoord, heldPath, before, heldHash)
	accAssertReported(t, stderr, "heldsrv")
	accAssertRotated(t, configDir, freshCoord, freshPath, freshHash)
}

func TestAcceptance_AddMCPForce_AllPinnedIsConflict(t *testing.T) {
	projectRoot, globalDir, configDir := accSeedMCP(t)
	heldPath := accMCPPath(globalDir, "heldsrv")
	heldCoord, heldHash := accSeedRecord(t, configDir, catalog.MCP, "heldsrv", heldPath, true)
	before := accSnapshot(t, heldPath)

	stderr, err := accRunAdd(t, projectRoot, []string{"mcp"}, map[string]string{"from": "claude-code", "scope": "project", "force": "true", "exclude": "freshsrv"})
	accAssertConflict(t, err)
	accAssertHeld(t, configDir, heldCoord, heldPath, before, heldHash)
	accAssertReported(t, stderr, "heldsrv")
}

// add --all --force reaches the same MCP writer.
func TestAcceptance_AddAllForce_PinnedMCPHeld(t *testing.T) {
	projectRoot, globalDir, configDir := accSeedMCP(t)
	heldPath := accMCPPath(globalDir, "heldsrv")
	heldCoord, heldHash := accSeedRecord(t, configDir, catalog.MCP, "heldsrv", heldPath, true)
	before := accSnapshot(t, heldPath)

	stderr, _ := accRunAdd(t, projectRoot, nil, map[string]string{"from": "claude-code", "scope": "project", "force": "true", "all": "true"})
	accAssertHeld(t, configDir, heldCoord, heldPath, before, heldHash)
	accAssertReported(t, stderr, "heldsrv")
}

func accHookSettings(command string) string {
	return `{"hooks": {"PreToolUse": [
  {"matcher": "Bash", "hooks": [{"type": "command", "command": "` + command + `", "statusMessage": "heldhook"}]}
]}}`
}

func TestAcceptance_AddHooksForce_AllPinnedIsConflict(t *testing.T) {
	projectRoot, globalDir, configDir := accEnv(t)
	settings := filepath.Join(projectRoot, ".claude", "settings.json")
	accWrite(t, settings, accHookSettings("echo v1"))
	if _, err := accRunAdd(t, projectRoot, []string{"hooks"}, map[string]string{"from": "claude-code", "scope": "project"}); err != nil {
		t.Fatalf("seed add hooks: %v", err)
	}
	hookDir := filepath.Join(globalDir, string(catalog.Hooks), "claude-code")
	entries, err := os.ReadDir(hookDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("seed add hooks: want one hook in %s, got %v (%v)", hookDir, entries, err)
	}
	hookName := entries[0].Name()
	heldPath := filepath.Join(hookDir, hookName)
	heldCoord, heldHash := accSeedRecord(t, configDir, catalog.Hooks, hookName, heldPath, true)
	before := accSnapshot(t, heldPath)
	accWrite(t, settings, accHookSettings("echo v2"))

	stderr, err := accRunAdd(t, projectRoot, []string{"hooks"}, map[string]string{"from": "claude-code", "scope": "project", "force": "true"})
	accAssertConflict(t, err)
	accAssertHeld(t, configDir, heldCoord, heldPath, before, heldHash)
	accAssertReported(t, stderr, hookName)
}

// --- monolithic rule add: add --from <file> --split=h2 ---

// accMonolithic is a CLAUDE.md that the H2 splitter cuts into heldrule,
// freshrule, and otherrule.
func accMonolithic(version string) string {
	var b strings.Builder
	b.WriteString("# Project rules\n\n")
	for _, name := range []string{"heldrule", "freshrule", "otherrule"} {
		b.WriteString("## " + name + "\n\n")
		for i := 0; i < 9; i++ {
			b.WriteString("- " + name + " guidance " + version + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func accRulePath(globalDir, name string) string {
	return filepath.Join(globalDir, string(catalog.Rules), "claude-code", name)
}

// accSeedMonolithic adds the three rules at v1 and points the file at v2.
func accSeedMonolithic(t *testing.T) (projectRoot, globalDir, configDir, source string) {
	t.Helper()
	projectRoot, globalDir, configDir = accEnv(t)
	source = filepath.Join(t.TempDir(), "CLAUDE.md")
	accWrite(t, source, accMonolithic("v1"))
	if _, err := accRunAdd(t, projectRoot, nil, map[string]string{"from": source, "split": "h2"}); err != nil {
		t.Fatalf("seed monolithic add: %v", err)
	}
	for _, name := range []string{"heldrule", "freshrule", "otherrule"} {
		if _, err := os.Stat(accRulePath(globalDir, name)); err != nil {
			t.Fatalf("seed monolithic add did not write %s: %v", name, err)
		}
	}
	accWrite(t, source, accMonolithic("v2"))
	return projectRoot, globalDir, configDir, source
}

func TestAcceptance_AddMonolithic_PinnedHeldOthersReplaced(t *testing.T) {
	projectRoot, globalDir, configDir, source := accSeedMonolithic(t)
	heldPath, freshPath := accRulePath(globalDir, "heldrule"), accRulePath(globalDir, "freshrule")
	heldCoord, heldHash := accSeedRecord(t, configDir, catalog.Rules, "heldrule", heldPath, true)
	freshCoord, freshHash := accSeedRecord(t, configDir, catalog.Rules, "freshrule", freshPath, false)
	before := accSnapshot(t, heldPath)

	stderr, err := accRunAdd(t, projectRoot, nil, map[string]string{"from": source, "split": "h2"})
	if err != nil {
		t.Fatalf("monolithic add with some rules unpinned: %v", err)
	}
	accAssertHeld(t, configDir, heldCoord, heldPath, before, heldHash)
	accAssertReported(t, stderr, "heldrule")
	accAssertRotated(t, configDir, freshCoord, freshPath, freshHash)
	if got, _ := os.ReadFile(filepath.Join(accRulePath(globalDir, "otherrule"), "rule.md")); !strings.Contains(string(got), "v2") {
		t.Errorf("unpinned otherrule rule.md = %q, want the v2 body", got)
	}
}

func TestAcceptance_AddMonolithic_AllPinnedIsConflict(t *testing.T) {
	projectRoot, globalDir, configDir, source := accSeedMonolithic(t)
	type held struct {
		coord  installstore.Coord
		hash   string
		path   string
		before map[string]string
	}
	var all []held
	for _, name := range []string{"heldrule", "freshrule", "otherrule"} {
		path := accRulePath(globalDir, name)
		coord, hash := accSeedRecord(t, configDir, catalog.Rules, name, path, true)
		all = append(all, held{coord, hash, path, accSnapshot(t, path)})
	}

	stderr, err := accRunAdd(t, projectRoot, nil, map[string]string{"from": source, "split": "h2"})
	accAssertConflict(t, err)
	for _, h := range all {
		accAssertHeld(t, configDir, h.coord, h.path, h.before, h.hash)
		accAssertReported(t, stderr, h.coord.Name)
	}
}

// --- shared add: add [name] --from shared --force ---

func accSharedSkill(t *testing.T, projectRoot, name, version string) {
	t.Helper()
	accWrite(t, filepath.Join(projectRoot, string(catalog.Skills), name, "SKILL.md"),
		"---\nname: "+name+"\ndescription: "+name+" skill\n---\n\n# "+name+"\n\nBody "+version+".\n")
}

// accSeedShared adds two project skills to the Library at v1 and moves the
// project copies to v2.
func accSeedShared(t *testing.T) (projectRoot, globalDir, configDir string) {
	t.Helper()
	projectRoot, globalDir, configDir = accEnv(t)
	for _, name := range []string{"heldskill", "freshskill"} {
		accSharedSkill(t, projectRoot, name, "v1")
	}
	if _, err := accRunAdd(t, projectRoot, nil, map[string]string{"from": "shared", "all": "true"}); err != nil {
		t.Fatalf("seed shared add: %v", err)
	}
	for _, name := range []string{"heldskill", "freshskill"} {
		if _, err := os.Stat(filepath.Join(globalDir, string(catalog.Skills), name, "SKILL.md")); err != nil {
			t.Fatalf("seed shared add did not write %s: %v", name, err)
		}
		accSharedSkill(t, projectRoot, name, "v2")
	}
	return projectRoot, globalDir, configDir
}

func TestAcceptance_AddSharedForce_PinnedHeldOthersReplaced(t *testing.T) {
	projectRoot, globalDir, configDir := accSeedShared(t)
	heldPath := filepath.Join(globalDir, string(catalog.Skills), "heldskill")
	freshPath := filepath.Join(globalDir, string(catalog.Skills), "freshskill")
	heldCoord, heldHash := accSeedRecord(t, configDir, catalog.Skills, "heldskill", heldPath, true)
	freshCoord, freshHash := accSeedRecord(t, configDir, catalog.Skills, "freshskill", freshPath, false)
	before := accSnapshot(t, heldPath)

	stderr, err := accRunAdd(t, projectRoot, nil, map[string]string{"from": "shared", "all": "true", "force": "true"})
	if err != nil {
		t.Fatalf("shared add --all --force with one item unpinned: %v", err)
	}
	accAssertHeld(t, configDir, heldCoord, heldPath, before, heldHash)
	accAssertReported(t, stderr, "heldskill")
	accAssertRotated(t, configDir, freshCoord, freshPath, freshHash)
}

func TestAcceptance_AddSharedForce_AllPinnedIsConflict(t *testing.T) {
	projectRoot, globalDir, configDir := accSeedShared(t)
	heldPath := filepath.Join(globalDir, string(catalog.Skills), "heldskill")
	heldCoord, heldHash := accSeedRecord(t, configDir, catalog.Skills, "heldskill", heldPath, true)
	before := accSnapshot(t, heldPath)

	stderr, err := accRunAdd(t, projectRoot, []string{"heldskill"}, map[string]string{"from": "shared", "force": "true"})
	accAssertConflict(t, err)
	accAssertHeld(t, configDir, heldCoord, heldPath, before, heldHash)
	accAssertReported(t, stderr, "heldskill")
}
