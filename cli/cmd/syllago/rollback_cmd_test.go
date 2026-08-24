package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestRollbackGitHappyPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	env := setupGitRollbackState(t, false)
	stdout, _ := output.SetForTest(t)
	resetRollbackFlags(t)

	if err := rollbackCmd.RunE(rollbackCmd, []string{"canary-skill"}); err != nil {
		t.Fatalf("rollback RunE: %v", err)
	}

	assertFileContains(t, filepath.Join(env.libraryPath, "SKILL.md"), "v1")
	meta, err := metadata.Load(env.libraryPath)
	if err != nil || meta == nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if meta.SourceSHA != env.shaA {
		t.Fatalf("metadata SourceSHA = %q, want %q", meta.SourceSHA, env.shaA)
	}
	store := mustLoadInstallRecordStore(t, env.configDir)
	rec := store.Find(env.coord)
	if rec == nil {
		t.Fatal("record missing after rollback")
	}
	if rec.SourceSHA != env.shaA {
		t.Fatalf("record SourceSHA = %q, want %q", rec.SourceSHA, env.shaA)
	}
	if rec.Previous != nil {
		t.Fatalf("record Previous = %#v, want nil", rec.Previous)
	}
	if !strings.Contains(stdout.String(), "Rolled back skills/canary-skill to "+shortSHA(env.shaA)) {
		t.Fatalf("stdout = %q, want rollback summary", stdout.String())
	}
}

func TestRollbackMOATCopyHappyPath(t *testing.T) {
	env := setupMOATRollbackState(t, "canary-skill", catalog.Skills)
	output.SetForTest(t)
	resetRollbackFlags(t)

	if err := rollbackCmd.RunE(rollbackCmd, []string{"canary-skill"}); err != nil {
		t.Fatalf("rollback RunE: %v", err)
	}

	assertFileContains(t, filepath.Join(env.libraryPath, "SKILL.md"), "v1")
	store := mustLoadInstallRecordStore(t, env.configDir)
	rec := store.Find(env.coord)
	if rec == nil {
		t.Fatal("record missing after rollback")
	}
	if rec.Previous != nil {
		t.Fatalf("record Previous = %#v, want nil", rec.Previous)
	}
}

func TestRollbackStructuredErrors(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		wantError string
	}{
		{
			name: "no record",
			setup: func(t *testing.T) string {
				globalDir, configDir := setupRollbackGlobals(t)
				writeLibrarySkill(t, globalDir, "canary-skill", "v2", &metadata.Meta{
					ID:             "skill-no-record",
					Name:           "canary-skill",
					Type:           string(catalog.Skills),
					SourceType:     "registry",
					SourceRegistry: "test-reg",
				})
				_ = configDir
				return "canary-skill"
			},
			wantError: "no install record",
		},
		{
			name: "previous nil",
			setup: func(t *testing.T) string {
				globalDir, configDir := setupRollbackGlobals(t)
				libraryPath := writeLibrarySkill(t, globalDir, "canary-skill", "v2", &metadata.Meta{
					ID:             "skill-no-prev",
					Name:           "canary-skill",
					Type:           string(catalog.Skills),
					SourceType:     "registry",
					SourceRegistry: "test-reg",
				})
				storePath := filepath.Join(configDir, "installs.json")
				coord := installstore.Coord{Registry: "test-reg", Type: string(catalog.Skills), Name: "canary-skill"}
				if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
					Provider:  "claude-code",
					Mechanism: installstore.MechanismSymlink,
					Path:      filepath.Join(t.TempDir(), "canary-skill"),
				}, installstore.InstallMeta{}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
					t.Fatalf("RecordInstallMeta: %v", err)
				}
				return "canary-skill"
			},
			wantError: "no rollback data for skills/canary-skill",
		},
		{
			name: "copy path missing",
			setup: func(t *testing.T) string {
				env := setupMOATRollbackState(t, "canary-skill", catalog.Skills)
				store := mustLoadInstallRecordStore(t, env.configDir)
				rec := store.Find(env.coord)
				rec.Previous.CopyPath = filepath.Join(t.TempDir(), "gone")
				if err := store.Save(); err != nil {
					t.Fatalf("Save store: %v", err)
				}
				return "canary-skill"
			},
			wantError: "saved previous copy is gone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output.SetForTest(t)
			resetRollbackFlags(t)
			arg := tt.setup(t)

			err := rollbackCmd.RunE(rollbackCmd, []string{arg})
			if err == nil {
				t.Fatal("rollback returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestRollbackItemAbsentAtOldSHA(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	env := setupGitRollbackStateMissingAtPreviousSHA(t)
	output.SetForTest(t)
	resetRollbackFlags(t)

	err := rollbackCmd.RunE(rollbackCmd, []string{"canary-skill"})
	if err == nil {
		t.Fatal("rollback returned nil error")
	}
	want := "item did not exist at " + shortSHA(env.shaA)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
	assertFileContains(t, filepath.Join(env.libraryPath, "SKILL.md"), "v2")
}

func TestRollbackDryRunLeavesLibraryAndRecordUntouched(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	env := setupGitRollbackState(t, false)
	storePath := filepath.Join(env.configDir, "installs.json")
	beforeStore, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("ReadFile store before: %v", err)
	}
	stdout, _ := output.SetForTest(t)
	resetRollbackFlags(t)
	rollbackCmd.Flags().Set("dry-run", "true")

	if err := rollbackCmd.RunE(rollbackCmd, []string{"canary-skill"}); err != nil {
		t.Fatalf("rollback dry-run: %v", err)
	}

	assertFileContains(t, filepath.Join(env.libraryPath, "SKILL.md"), "v2")
	afterStore, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("ReadFile store after: %v", err)
	}
	if string(afterStore) != string(beforeStore) {
		t.Fatalf("dry-run changed install store\nbefore:\n%s\nafter:\n%s", beforeStore, afterStore)
	}
	if !strings.Contains(stdout.String(), "would roll back skills/canary-skill to "+shortSHA(env.shaA)) {
		t.Fatalf("stdout = %q, want dry-run plan", stdout.String())
	}
}

func TestRollbackPinnedRecordSucceeds(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	env := setupGitRollbackState(t, true)
	output.SetForTest(t)
	resetRollbackFlags(t)

	if err := rollbackCmd.RunE(rollbackCmd, []string{"canary-skill"}); err != nil {
		t.Fatalf("rollback pinned record: %v", err)
	}
	assertFileContains(t, filepath.Join(env.libraryPath, "SKILL.md"), "v1")
	store := mustLoadInstallRecordStore(t, env.configDir)
	rec := store.Find(env.coord)
	if rec == nil {
		t.Fatal("record missing after rollback")
	}
	if !rec.Pinned {
		t.Fatal("record Pinned = false, want true")
	}
}

func TestRollbackReappliesHookMergePlacement(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	projectRoot := t.TempDir()
	withFakeRepoRoot(t, projectRoot)
	env := setupMOATHookRollbackState(t, homeDir, projectRoot)
	stdout, _ := output.SetForTest(t)
	resetRollbackFlags(t)

	if err := rollbackCmd.RunE(rollbackCmd, []string{"rollback-hook"}); err != nil {
		t.Fatalf("rollback hook: %v", err)
	}

	assertFileContains(t, filepath.Join(env.libraryPath, "hook.json"), "echo v1")
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	assertFileContains(t, settingsPath, "echo v1")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile settings: %v", err)
	}
	if strings.Contains(string(data), "echo v2") {
		t.Fatalf("settings still contain v2 hook: %s", data)
	}
	if !strings.Contains(stdout.String(), "Re-applied hook_merge placement for claude-code") {
		t.Fatalf("stdout = %q, want reapply line", stdout.String())
	}
}

func TestRollbackJSONOutput(t *testing.T) {
	env := setupMOATRollbackState(t, "canary-skill", catalog.Skills)
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	resetRollbackFlags(t)

	if err := rollbackCmd.RunE(rollbackCmd, []string{"canary-skill"}); err != nil {
		t.Fatalf("rollback RunE: %v", err)
	}

	var got struct {
		Name                string `json:"name"`
		Type                string `json:"type"`
		Registry            string `json:"registry"`
		RestoredFromCopy    bool   `json:"restored_from_copy"`
		PlacementsReapplied int    `json:"placements_reapplied"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, stdout.String())
	}
	if got.Name != "canary-skill" || got.Type != string(catalog.Skills) || got.Registry != env.coord.Registry || !got.RestoredFromCopy {
		t.Fatalf("unexpected JSON output: %+v", got)
	}
}

type rollbackGitEnv struct {
	globalDir   string
	configDir   string
	regName     string
	shaA        string
	shaB        string
	libraryPath string
	coord       installstore.Coord
}

type rollbackMOATEnv struct {
	globalDir   string
	configDir   string
	libraryPath string
	coord       installstore.Coord
}

func setupRollbackGlobals(t *testing.T) (globalDir, configDir string) {
	t.Helper()
	globalDir = t.TempDir()
	withGlobalLibrary(t, globalDir)
	configDir = withInstallRecordConfigDir(t)
	withFakeRepoRoot(t, t.TempDir())
	return globalDir, configDir
}

func setupRollbackCache(t *testing.T) string {
	t.Helper()
	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })
	return cacheDir
}

func setupGitRollbackState(t *testing.T, pinned bool) rollbackGitEnv {
	t.Helper()
	globalDir, configDir := setupRollbackGlobals(t)
	cacheDir := setupRollbackCache(t)
	const regName = "test-org/rollback-reg"
	cloneDir := filepath.Join(cacheDir, filepath.FromSlash(regName))
	if err := os.MkdirAll(cloneDir, 0755); err != nil {
		t.Fatalf("MkdirAll clone: %v", err)
	}
	gitRunForRegistryAddTest(t, cloneDir, "init")
	gitRunForRegistryAddTest(t, cloneDir, "config", "user.email", "test@example.com")
	gitRunForRegistryAddTest(t, cloneDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte("name: rollback-reg\n"), 0644); err != nil {
		t.Fatalf("WriteFile registry.yaml: %v", err)
	}
	writeGitRegistrySkill(t, cloneDir, "v1")
	gitRunForRegistryAddTest(t, cloneDir, "add", "-A")
	gitRunForRegistryAddTest(t, cloneDir, "commit", "-m", "v1")
	shaA := gitOutputForRegistryAddTest(t, cloneDir, "rev-parse", "HEAD")
	addRegistrySkillToLibrary(t, regName, cloneDir, globalDir, shaA)

	libraryPath := filepath.Join(globalDir, "skills", "canary-skill")
	coord := installstore.Coord{Registry: regName, Type: string(catalog.Skills), Name: "canary-skill"}
	storePath := filepath.Join(configDir, "installs.json")
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismSymlink,
		Path:      filepath.Join(t.TempDir(), "canary-skill"),
	}, installstore.InstallMeta{SourceSHA: shaA}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}

	writeGitRegistrySkill(t, cloneDir, "v2")
	gitRunForRegistryAddTest(t, cloneDir, "add", "-A")
	gitRunForRegistryAddTest(t, cloneDir, "commit", "-m", "v2")
	shaB := gitOutputForRegistryAddTest(t, cloneDir, "rev-parse", "HEAD")
	addRegistrySkillToLibrary(t, regName, cloneDir, globalDir, shaB)
	if err := installstore.RecordUpdate(storePath, coord, libraryPath, shaB, "", time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordUpdate: %v", err)
	}
	if pinned {
		if err := installstore.SetPinned(storePath, coord, true, time.Date(2026, 8, 24, 14, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("SetPinned: %v", err)
		}
	}
	return rollbackGitEnv{
		globalDir:   globalDir,
		configDir:   configDir,
		regName:     regName,
		shaA:        shaA,
		shaB:        shaB,
		libraryPath: libraryPath,
		coord:       coord,
	}
}

func setupGitRollbackStateMissingAtPreviousSHA(t *testing.T) rollbackGitEnv {
	t.Helper()
	globalDir, configDir := setupRollbackGlobals(t)
	cacheDir := setupRollbackCache(t)
	const regName = "test-org/rollback-missing"
	cloneDir := filepath.Join(cacheDir, filepath.FromSlash(regName))
	if err := os.MkdirAll(cloneDir, 0755); err != nil {
		t.Fatalf("MkdirAll clone: %v", err)
	}
	gitRunForRegistryAddTest(t, cloneDir, "init")
	gitRunForRegistryAddTest(t, cloneDir, "config", "user.email", "test@example.com")
	gitRunForRegistryAddTest(t, cloneDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte("name: rollback-missing\n"), 0644); err != nil {
		t.Fatalf("WriteFile registry.yaml: %v", err)
	}
	gitRunForRegistryAddTest(t, cloneDir, "add", "-A")
	gitRunForRegistryAddTest(t, cloneDir, "commit", "-m", "empty")
	shaA := gitOutputForRegistryAddTest(t, cloneDir, "rev-parse", "HEAD")

	writeGitRegistrySkill(t, cloneDir, "v2")
	gitRunForRegistryAddTest(t, cloneDir, "add", "-A")
	gitRunForRegistryAddTest(t, cloneDir, "commit", "-m", "add skill")
	shaB := gitOutputForRegistryAddTest(t, cloneDir, "rev-parse", "HEAD")
	addRegistrySkillToLibrary(t, regName, cloneDir, globalDir, shaB)

	libraryPath := filepath.Join(globalDir, "skills", "canary-skill")
	coord := installstore.Coord{Registry: regName, Type: string(catalog.Skills), Name: "canary-skill"}
	storePath := filepath.Join(configDir, "installs.json")
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismSymlink,
		Path:      filepath.Join(t.TempDir(), "canary-skill"),
	}, installstore.InstallMeta{SourceSHA: shaA}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	if err := installstore.RecordUpdate(storePath, coord, libraryPath, shaB, "", time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordUpdate: %v", err)
	}
	return rollbackGitEnv{
		globalDir:   globalDir,
		configDir:   configDir,
		regName:     regName,
		shaA:        shaA,
		shaB:        shaB,
		libraryPath: libraryPath,
		coord:       coord,
	}
}

func setupMOATRollbackState(t *testing.T, name string, ct catalog.ContentType) rollbackMOATEnv {
	t.Helper()
	globalDir, configDir := setupRollbackGlobals(t)
	libraryPath := writeLibrarySkill(t, globalDir, name, "v1", &metadata.Meta{
		ID:             "skill-" + name,
		Name:           name,
		Type:           string(ct),
		SourceType:     "registry",
		SourceRegistry: "moat-reg",
	})
	prevCopy := filepath.Join(t.TempDir(), "prev-copy")
	copyTreeForRollbackTest(t, libraryPath, prevCopy)
	_ = writeLibrarySkill(t, globalDir, name, "v2", &metadata.Meta{
		ID:             "skill-" + name,
		Name:           name,
		Type:           string(ct),
		SourceType:     "registry",
		SourceRegistry: "moat-reg",
	})

	coord := installstore.Coord{Registry: "moat-reg", Type: string(ct), Name: name}
	storePath := filepath.Join(configDir, "installs.json")
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismSymlink,
		Path:      filepath.Join(t.TempDir(), name),
	}, installstore.InstallMeta{}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	if err := installstore.RecordUpdate(storePath, coord, libraryPath, "", prevCopy, time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordUpdate: %v", err)
	}
	return rollbackMOATEnv{globalDir: globalDir, configDir: configDir, libraryPath: libraryPath, coord: coord}
}

func setupMOATHookRollbackState(t *testing.T, homeDir, projectRoot string) rollbackMOATEnv {
	t.Helper()
	globalDir, configDir := setupRollbackGlobals(t)
	withFakeRepoRoot(t, projectRoot)
	hookPath := writeLibraryHook(t, globalDir, "rollback-hook", "echo v1")
	prevCopy := filepath.Join(t.TempDir(), "prev-hook")
	copyTreeForRollbackTest(t, hookPath, prevCopy)
	hookPath = writeLibraryHook(t, globalDir, "rollback-hook", "echo v2")

	coord := installstore.Coord{Registry: "moat-reg", Type: string(catalog.Hooks), Name: "rollback-hook"}
	storePath := filepath.Join(configDir, "installs.json")
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	if err := installstore.RecordInstallMeta(storePath, coord, hookPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismHookMerge,
		Path:      settingsPath,
		Keys:      []string{"hooks.PreToolUse"},
	}, installstore.InstallMeta{}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	if err := installstore.RecordUpdate(storePath, coord, hookPath, "", prevCopy, time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RecordUpdate: %v", err)
	}

	item := scanRollbackTestItem(t, "rollback-hook", string(catalog.Hooks))
	if _, err := installer.Install(*item, provider.ClaudeCode, projectRoot, installer.MethodSymlink, ""); err != nil {
		t.Fatalf("install v2 hook: %v", err)
	}
	assertFileContains(t, settingsPath, "echo v2")
	return rollbackMOATEnv{globalDir: globalDir, configDir: configDir, libraryPath: hookPath, coord: coord}
}

func addRegistrySkillToLibrary(t *testing.T, regName, cloneDir, globalDir, sourceSHA string) {
	t.Helper()
	items, err := add.DiscoverFromRegistry(regName, cloneDir, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromRegistry: %v", err)
	}
	var match *add.DiscoveryItem
	for i := range items {
		if items[i].Type == catalog.Skills && items[i].Name == "canary-skill" {
			match = &items[i]
			break
		}
	}
	if match == nil {
		t.Fatalf("canary-skill not discovered in %s", cloneDir)
	}
	results := add.AddItems([]add.DiscoveryItem{*match}, add.AddOptions{
		Force:            true,
		SourceRegistry:   regName,
		SourceSHA:        sourceSHA,
		SourceVisibility: "public",
	}, globalDir, nil, version)
	if len(results) != 1 || results[0].Status == add.AddStatusError {
		t.Fatalf("AddItems results = %#v", results)
	}
}

func writeGitRegistrySkill(t *testing.T, cloneDir, versionLabel string) {
	t.Helper()
	skillDir := filepath.Join(cloneDir, "skills", "canary-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Canary skill\n"+versionLabel+"\n"), 0644); err != nil {
		t.Fatalf("WriteFile skill: %v", err)
	}
}

func writeLibrarySkill(t *testing.T, globalDir, name, versionLabel string, meta *metadata.Meta) string {
	t.Helper()
	skillDir := filepath.Join(globalDir, "skills", name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Canary skill\n"+versionLabel+"\n"), 0644); err != nil {
		t.Fatalf("WriteFile skill: %v", err)
	}
	if meta != nil {
		if err := metadata.Save(skillDir, meta); err != nil {
			t.Fatalf("metadata.Save: %v", err)
		}
	}
	return skillDir
}

func writeLibraryHook(t *testing.T, globalDir, name, command string) string {
	t.Helper()
	hookDir := filepath.Join(globalDir, "hooks", "claude-code", name)
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatalf("MkdirAll hook dir: %v", err)
	}
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"name":"` + name + `","event":"PreToolUse","matcher":"Bash","handler":{"type":"command","command":"` + command + `"}}]}`
	if err := os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatalf("WriteFile hook: %v", err)
	}
	if err := metadata.Save(hookDir, &metadata.Meta{
		ID:             "hook-" + name,
		Name:           name,
		Type:           string(catalog.Hooks),
		SourceProvider: "claude-code",
		SourceType:     "registry",
		SourceRegistry: "moat-reg",
	}); err != nil {
		t.Fatalf("metadata.Save hook: %v", err)
	}
	return hookDir
}

func scanRollbackTestItem(t *testing.T, name, typeFilter string) *catalog.ContentItem {
	t.Helper()
	emptyRoot := t.TempDir()
	cat, err := catalog.ScanWithGlobalAndRegistries(emptyRoot, emptyRoot, nil)
	if err != nil {
		t.Fatalf("ScanWithGlobalAndRegistries: %v", err)
	}
	item, err := findLibraryItem(cat, name, typeFilter)
	if err != nil {
		t.Fatalf("findLibraryItem: %v", err)
	}
	return item
}

func resetRollbackFlags(t *testing.T) {
	t.Helper()
	rollbackCmd.Flags().Set("type", "")
	rollbackCmd.Flags().Set("dry-run", "false")
	t.Cleanup(func() {
		rollbackCmd.Flags().Set("type", "")
		rollbackCmd.Flags().Set("dry-run", "false")
	})
}

func copyTreeForRollbackTest(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0755)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	}); err != nil {
		t.Fatalf("copyTree %s -> %s: %v", src, dst, err)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want substring %q", path, data, want)
	}
}
