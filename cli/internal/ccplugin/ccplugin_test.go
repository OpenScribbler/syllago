package ccplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadMissingState(t *testing.T) {
	t.Parallel()

	t.Run("no claude dir", func(t *testing.T) {
		t.Parallel()

		got, err := Load(t.TempDir())
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("Load() returned %d plugins, want 0: %#v", len(got), got)
		}
	})

	t.Run("ledger present settings absent", func(t *testing.T) {
		t.Parallel()

		home := t.TempDir()
		installPath := makePluginInstall(t, home, "demo", nil)
		writeLedger(t, home, map[string][]ledgerFixtureEntry{
			"demo@local": {{
				Scope:       "user",
				InstallPath: installPath,
				Version:     "1.2.3",
			}},
		})

		got, err := Load(home)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("Load() returned %d plugins, want 1: %#v", len(got), got)
		}
		if got[0].Enabled {
			t.Fatalf("Enabled = true, want false when settings.json is absent")
		}
	})
}

func TestLoadMultipleScopeEntries(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	installPath := makePluginInstall(t, home, "demo", nil)
	writeLedger(t, home, map[string][]ledgerFixtureEntry{
		"demo@local": {
			{Scope: "project", InstallPath: installPath, Version: "a"},
			{Scope: "user", InstallPath: installPath, Version: "b"},
		},
	})

	got, err := Load(home)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Load() returned %d plugins, want 2: %#v", len(got), got)
	}
	if got[0].Scope != "project" || got[1].Scope != "user" {
		t.Fatalf("Scopes = %q, %q; want project, user", got[0].Scope, got[1].Scope)
	}
}

func TestLoadEnabledPlugins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings map[string]bool
		want     map[string]bool
	}{
		{
			name:     "true false and missing",
			settings: map[string]bool{"on@local": true, "off@local": false},
			want:     map[string]bool{"on@local": true, "off@local": false, "missing@local": false},
		},
		{
			name:     "missing enabledPlugins key",
			settings: nil,
			want:     map[string]bool{"on@local": false, "off@local": false, "missing@local": false},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			plugins := map[string][]ledgerFixtureEntry{}
			for id := range tc.want {
				plugins[id] = []ledgerFixtureEntry{{
					Scope:       "user",
					InstallPath: makePluginInstall(t, home, strings.TrimSuffix(id, "@local"), nil),
				}}
			}
			writeLedger(t, home, plugins)
			if tc.settings == nil {
				writeRawSettings(t, home, `{"other": true}`)
			} else {
				writeSettings(t, home, tc.settings)
			}

			got, err := Load(home)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			gotEnabled := map[string]bool{}
			for _, plugin := range got {
				gotEnabled[plugin.ID] = plugin.Enabled
			}
			if !reflect.DeepEqual(gotEnabled, tc.want) {
				t.Fatalf("Enabled map = %#v, want %#v", gotEnabled, tc.want)
			}
		})
	}
}

func TestLoadManifestSkillForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		manifest        map[string]any
		corruptManifest bool
		makeSkills      func(t *testing.T, installPath string) map[string]string
		wantNames       []string
	}{
		{
			name:     "absent key uses convention",
			manifest: nil,
			makeSkills: func(t *testing.T, installPath string) map[string]string {
				alpha := writeSkill(t, filepath.Join(installPath, "skills", "alpha"))
				beta := writeSkill(t, filepath.Join(installPath, "skills", "beta"))
				return map[string]string{"alpha": alpha, "beta": beta}
			},
			wantNames: []string{"alpha", "beta"},
		},
		{
			name:     "string directory",
			manifest: map[string]any{"skills": "./tool-skills/"},
			makeSkills: func(t *testing.T, installPath string) map[string]string {
				review := writeSkill(t, filepath.Join(installPath, "tool-skills", "review"))
				return map[string]string{"review": review}
			},
			wantNames: []string{"review"},
		},
		{
			name: "array of nested skill directories",
			manifest: map[string]any{
				"skills": []string{
					"./skills/engineering/tdd",
					"./skills/writing/release-notes",
					"./skills/no-skill-md",
				},
			},
			makeSkills: func(t *testing.T, installPath string) map[string]string {
				tdd := writeSkill(t, filepath.Join(installPath, "skills", "engineering", "tdd"))
				releaseNotes := writeSkill(t, filepath.Join(installPath, "skills", "writing", "release-notes"))
				if err := os.MkdirAll(filepath.Join(installPath, "skills", "no-skill-md"), 0o755); err != nil {
					t.Fatalf("mkdir no-skill-md: %v", err)
				}
				return map[string]string{"tdd": tdd, "release-notes": releaseNotes}
			},
			wantNames: []string{"release-notes", "tdd"},
		},
		{
			name:     "string directory escaping install path is refused",
			manifest: map[string]any{"skills": "../escape-string/"},
			makeSkills: func(t *testing.T, installPath string) map[string]string {
				writeSkill(t, filepath.Join(installPath, "..", "escape-string", "stolen"))
				return map[string]string{}
			},
			wantNames: []string{},
		},
		{
			name:     "array entry escaping install path is refused",
			manifest: map[string]any{"skills": []string{"../escape-array/stolen"}},
			makeSkills: func(t *testing.T, installPath string) map[string]string {
				writeSkill(t, filepath.Join(installPath, "..", "escape-array", "stolen"))
				return map[string]string{}
			},
			wantNames: []string{},
		},
		{
			name:            "unparsable manifest falls back to convention",
			corruptManifest: true,
			makeSkills: func(t *testing.T, installPath string) map[string]string {
				fallback := writeSkill(t, filepath.Join(installPath, "skills", "fallback"))
				return map[string]string{"fallback": fallback}
			},
			wantNames: []string{"fallback"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			installPath := makePluginInstall(t, home, tc.name, tc.manifest)
			if tc.corruptManifest {
				writeRawFile(t, filepath.Join(installPath, ".claude-plugin", "plugin.json"), `{"skills":`)
			}
			wantPaths := tc.makeSkills(t, installPath)
			writeLedger(t, home, map[string][]ledgerFixtureEntry{
				"demo@local": {{Scope: "user", InstallPath: installPath}},
			})

			got, err := Load(home)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("Load() returned %d plugins, want 1: %#v", len(got), got)
			}

			gotNames := make([]string, 0, len(got[0].Skills))
			for _, skill := range got[0].Skills {
				gotNames = append(gotNames, skill.Name)
				if wantPath := wantPaths[skill.Name]; skill.Path != wantPath {
					t.Fatalf("Skill %q path = %q, want %q", skill.Name, skill.Path, wantPath)
				}
			}
			if !reflect.DeepEqual(gotNames, tc.wantNames) {
				t.Fatalf("Skill names = %#v, want %#v", gotNames, tc.wantNames)
			}
		})
	}
}

func TestLoadMissingInstallPath(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	missingPath := filepath.Join(home, "cache", "missing")
	writeLedger(t, home, map[string][]ledgerFixtureEntry{
		"missing@local": {{Scope: "user", InstallPath: missingPath}},
	})

	got, err := Load(home)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Load() returned %d plugins, want 1: %#v", len(got), got)
	}
	if len(got[0].Skills) != 0 {
		t.Fatalf("Skills = %#v, want none", got[0].Skills)
	}
}

func TestLoadVersionAndTimestamps(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	installPath := makePluginInstall(t, home, "demo", nil)
	writeLedger(t, home, map[string][]ledgerFixtureEntry{
		"absent@local":  {{Scope: "user", InstallPath: installPath}},
		"unknown@local": {{Scope: "user", InstallPath: installPath, Version: "unknown", InstalledAt: "not-a-time"}},
		"sha@local":     {{Scope: "user", InstallPath: installPath, Version: "a1b2c3d4", InstalledAt: "2026-08-22T12:34:56Z"}},
	})

	got, err := Load(home)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	versions := map[string]string{}
	installed := map[string]time.Time{}
	for _, plugin := range got {
		versions[plugin.ID] = plugin.Version
		installed[plugin.ID] = plugin.InstalledAt
	}
	wantVersions := map[string]string{
		"absent@local":  "",
		"unknown@local": "unknown",
		"sha@local":     "a1b2c3d4",
	}
	if !reflect.DeepEqual(versions, wantVersions) {
		t.Fatalf("Versions = %#v, want %#v", versions, wantVersions)
	}
	if !installed["unknown@local"].IsZero() {
		t.Fatalf("unparsable InstalledAt = %v, want zero", installed["unknown@local"])
	}
	wantTime := time.Date(2026, 8, 22, 12, 34, 56, 0, time.UTC)
	if !installed["sha@local"].Equal(wantTime) {
		t.Fatalf("InstalledAt = %v, want %v", installed["sha@local"], wantTime)
	}
}

func TestLoadCorruptJSON(t *testing.T) {
	t.Parallel()

	t.Run("ledger", func(t *testing.T) {
		t.Parallel()

		home := t.TempDir()
		writeRawLedger(t, home, `{"plugins":`)
		_, err := Load(home)
		if err == nil {
			t.Fatalf("Load() error = nil, want error")
		}
	})

	t.Run("settings", func(t *testing.T) {
		t.Parallel()

		home := t.TempDir()
		installPath := makePluginInstall(t, home, "demo", nil)
		writeLedger(t, home, map[string][]ledgerFixtureEntry{
			"demo@local": {{Scope: "user", InstallPath: installPath}},
		})
		writeRawSettings(t, home, `{"enabledPlugins":`)

		_, err := Load(home)
		if err == nil {
			t.Fatalf("Load() error = nil, want error")
		}
	})
}

func TestLoadDeterministicSort(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	installPath := makePluginInstall(t, home, "demo", nil)
	writeLedger(t, home, map[string][]ledgerFixtureEntry{
		"zeta@market-b":  {{Scope: "user", InstallPath: installPath}},
		"alpha@market-a": {{Scope: "workspace", InstallPath: installPath}, {Scope: "user", InstallPath: installPath}},
		"beta@market-a":  {{Scope: "user", InstallPath: installPath}},
	})

	got, err := Load(home)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	gotOrder := make([]string, 0, len(got))
	for _, plugin := range got {
		gotOrder = append(gotOrder, plugin.ID+"/"+plugin.Scope)
	}
	wantOrder := []string{
		"alpha@market-a/user",
		"alpha@market-a/workspace",
		"beta@market-a/user",
		"zeta@market-b/user",
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("Order = %#v, want %#v", gotOrder, wantOrder)
	}
}

type ledgerFixtureEntry struct {
	Scope       string `json:"scope,omitempty"`
	InstallPath string `json:"installPath,omitempty"`
	Version     string `json:"version,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
}

func makePluginInstall(t *testing.T, home, name string, manifest map[string]any) string {
	t.Helper()

	installPath := filepath.Join(home, "plugin-cache", name)
	if err := os.MkdirAll(filepath.Join(installPath, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir plugin install: %v", err)
	}
	if manifest == nil {
		manifest = map[string]any{"name": name}
	} else if _, ok := manifest["name"]; !ok {
		manifest["name"] = name
	}
	if err := writeJSON(filepath.Join(installPath, ".claude-plugin", "plugin.json"), manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return installPath
}

func writeSkill(t *testing.T, dir string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("Use this skill.\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return dir
}

func writeLedger(t *testing.T, home string, plugins map[string][]ledgerFixtureEntry) {
	t.Helper()

	body := map[string]any{
		"version": 2,
		"plugins": plugins,
	}
	if err := writeJSON(filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), body); err != nil {
		t.Fatalf("write ledger: %v", err)
	}
}

func writeRawLedger(t *testing.T, home, body string) {
	t.Helper()
	writeRawFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), body)
}

func writeSettings(t *testing.T, home string, enabled map[string]bool) {
	t.Helper()

	body := map[string]any{"enabledPlugins": enabled}
	if err := writeJSON(filepath.Join(home, ".claude", "settings.json"), body); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func writeRawSettings(t *testing.T, home, body string) {
	t.Helper()
	writeRawFile(t, filepath.Join(home, ".claude", "settings.json"), body)
}

func writeRawFile(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
