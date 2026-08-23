package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestRegistryStatusMOATTextOutput(t *testing.T) {
	cases := []struct {
		name       string
		result     moat.SyncResult
		seedCache  *moat.Manifest
		wantOutput string
	}{
		{
			name: "up-to-date",
			result: moat.SyncResult{
				NotModified: true,
				ETag:        `"old"`,
				FetchedAt:   registryStatusTestNow(),
				Staleness:   moat.StalenessFresh,
			},
			wantOutput: "moat-reg (moat): up to date\n",
		},
		{
			name: "changes",
			result: registryStatusSyncResult(t, registryStatusManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
				registryStatusEntry("added", "skill", "3"),
				registryStatusEntry("kept", "skill", "4"),
			})),
			seedCache: registryStatusManifest(t, "2026-04-20T00:00:00Z", []moat.ContentEntry{
				registryStatusEntry("kept", "skill", "1"),
				registryStatusEntry("removed", "skill", "2"),
			}),
			wantOutput: "moat-reg (moat): 3 upstream change(s)\n" +
				"  + skill/added\n" +
				"  ~ skill/kept\n" +
				"  - skill/removed\n",
		},
		{
			name: "not-synced",
			result: func() moat.SyncResult {
				res := registryStatusSyncResult(t, registryStatusManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
					registryStatusEntry("first", "skill", "1"),
				}))
				res.IsTOFU = true
				return res
			}(),
			wantOutput: "moat-reg (moat): not synced yet — run 'syllago registry sync moat-reg'\n",
		},
		{
			name: "profile-changed",
			result: func() moat.SyncResult {
				res := registryStatusSyncResult(t, registryStatusManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
					registryStatusEntry("changed", "skill", "1"),
				}))
				res.ProfileChanged = true
				return res
			}(),
			wantOutput: "moat-reg (moat): signing profile changed upstream — run 'syllago registry sync moat-reg'\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, cacheDir := setupRegistryStatusMOATTest(t, tc.seedCache)
			oldCacheBytes := readCachedManifestIfPresent(t, cacheDir, "moat-reg")
			withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
				return tc.result, nil
			})
			stdout, _ := output.SetForTest(t)

			registryStatusCmd.SilenceUsage = true
			registryStatusCmd.SilenceErrors = true
			if err := registryStatusCmd.RunE(registryStatusCmd, nil); err != nil {
				t.Fatalf("registry status: %v", err)
			}
			if got := stdout.String(); got != tc.wantOutput {
				t.Fatalf("stdout = %q; want %q", got, tc.wantOutput)
			}
			assertRegistryStatusReadOnly(t, root, cacheDir, "moat-reg", oldCacheBytes)
		})
	}
}

func TestRegistryStatusGitShowsUpstreamChangesWithoutConsumingSync(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	bare, work, branch := createBareGitRegistryForSyncTest(t)
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "git-status", URL: bare},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	stdout, _ := output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) {
		return registry.VisibilityPublic, nil
	})

	registrySyncCmd.SilenceUsage = true
	registrySyncCmd.SilenceErrors = true
	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"git-status"}); err != nil {
		t.Fatalf("initial registry sync: %v", err)
	}

	updateBareGitRegistryForStatusDiffTest(t, work, bare, branch)

	stdout.Reset()
	registryStatusCmd.SilenceUsage = true
	registryStatusCmd.SilenceErrors = true
	if err := registryStatusCmd.RunE(registryStatusCmd, []string{"git-status"}); err != nil {
		t.Fatalf("registry status: %v", err)
	}
	wantStatus := "git-status (git): 2 upstream change(s)\n" +
		"  ~ rules/updated-rule\n" +
		"  + skills/new-thing\n"
	if got := stdout.String(); got != wantStatus {
		t.Fatalf("status stdout = %q; want %q", got, wantStatus)
	}

	stdout.Reset()
	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"git-status"}); err != nil {
		t.Fatalf("registry sync after status: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "Synced: git-status\nChanges since last sync:\n") {
		t.Fatalf("sync after status stdout = %q; want sync to still pull and print diff", got)
	}

	stdout.Reset()
	if err := registryStatusCmd.RunE(registryStatusCmd, []string{"git-status"}); err != nil {
		t.Fatalf("registry status after sync: %v", err)
	}
	if got := stdout.String(); got != "git-status (git): up to date\n" {
		t.Fatalf("status after sync stdout = %q; want up to date", got)
	}
}

func TestRegistryStatusJSON(t *testing.T) {
	setupRegistryStatusMOATTest(t, registryStatusManifest(t, "2026-04-20T00:00:00Z", []moat.ContentEntry{
		registryStatusEntry("kept", "skill", "1"),
		registryStatusEntry("removed", "skill", "2"),
	}))
	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return registryStatusSyncResult(t, registryStatusManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
			registryStatusEntry("added", "skill", "3"),
			registryStatusEntry("kept", "skill", "4"),
		})), nil
	})
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	registryStatusCmd.SilenceUsage = true
	registryStatusCmd.SilenceErrors = true
	if err := registryStatusCmd.RunE(registryStatusCmd, nil); err != nil {
		t.Fatalf("registry status: %v", err)
	}

	var rows []struct {
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		State    string `json:"state"`
		Added    int    `json:"added"`
		Modified int    `json:"modified"`
		Removed  int    `json:"removed"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal status JSON: %v\n%s", err, stdout.String())
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows; want 1", len(rows))
	}
	got := rows[0]
	if got.Name != "moat-reg" || got.Kind != "moat" || got.State != "changes" || got.Added != 1 || got.Modified != 1 || got.Removed != 1 {
		t.Fatalf("JSON row = %+v; want moat-reg changes with 1/1/1 counts", got)
	}
}

func TestRegistryStatusUnknownExplicitName(t *testing.T) {
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "exists", URL: "https://example.com/exists.git"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)

	registryStatusCmd.SilenceUsage = true
	registryStatusCmd.SilenceErrors = true
	err := registryStatusCmd.RunE(registryStatusCmd, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "registry \"missing\" not found in config") {
		t.Fatalf("got %v; want missing registry error", err)
	}
}

func setupRegistryStatusMOATTest(t *testing.T, cached *moat.Manifest) (root, cacheDir string) {
	t.Helper()
	profile := incomingProfile()
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{
				Name:           "moat-reg",
				URL:            "https://registry.example.com/manifest.json",
				Type:           config.RegistryTypeMOAT,
				ManifestURI:    "https://registry.example.com/manifest.json",
				SigningProfile: &profile,
				ManifestETag:   `"old"`,
			},
		},
	}
	root = withRegistryProjectAndCache(t, nil, cfg)
	var err error
	cacheDir, err = config.GlobalDirPath()
	if err != nil {
		t.Fatalf("GlobalDirPath: %v", err)
	}
	if cached != nil {
		if err := moat.WriteManifestCache(cacheDir, "moat-reg", registryStatusManifestBytes(t, cached), []byte(`{"bundle":true}`)); err != nil {
			t.Fatalf("WriteManifestCache: %v", err)
		}
	}
	return root, cacheDir
}

func assertRegistryStatusReadOnly(t *testing.T, root, cacheDir, name string, oldCacheBytes []byte) {
	t.Helper()
	reloaded, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if reloaded.Registries[0].ManifestETag != `"old"` {
		t.Fatalf("ManifestETag = %q; want unchanged", reloaded.Registries[0].ManifestETag)
	}
	if _, err := os.Stat(moat.LockfilePath(root)); !os.IsNotExist(err) {
		t.Fatalf("lockfile exists after registry status: %v", err)
	}
	gotCacheBytes := readCachedManifestIfPresent(t, cacheDir, name)
	if string(gotCacheBytes) != string(oldCacheBytes) {
		t.Fatalf("cached manifest changed:\n got %s\nwant %s", gotCacheBytes, oldCacheBytes)
	}
}

func readCachedManifestIfPresent(t *testing.T, cacheDir, name string) []byte {
	t.Helper()
	path, err := moat.ManifestCachePath(cacheDir, name)
	if err != nil {
		t.Fatalf("ManifestCachePath: %v", err)
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("ReadFile cached manifest: %v", err)
	}
	return data
}

func registryStatusSyncResult(t *testing.T, manifest *moat.Manifest) moat.SyncResult {
	t.Helper()
	return moat.SyncResult{
		ManifestURL:     "https://registry.example.com/manifest.json",
		BundleURL:       "https://registry.example.com/manifest.json.sigstore",
		ETag:            `"fresh"`,
		Manifest:        manifest,
		ManifestBytes:   registryStatusManifestBytes(t, manifest),
		BundleBytes:     []byte(`{"bundle":true}`),
		IncomingProfile: incomingProfile(),
		FetchedAt:       registryStatusTestNow(),
		Staleness:       moat.StalenessFresh,
	}
}

func registryStatusManifest(t *testing.T, updatedAt string, entries []moat.ContentEntry) *moat.Manifest {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		t.Fatalf("parse updated_at: %v", err)
	}
	return &moat.Manifest{
		SchemaVersion:          moat.ManifestSchemaVersion,
		ManifestURI:            "https://registry.example.com/manifest.json",
		Name:                   "Status Registry",
		Operator:               "Status Operator",
		UpdatedAt:              ts,
		RegistrySigningProfile: moat.SigningProfile{Issuer: incomingProfile().Issuer, Subject: incomingProfile().Subject},
		Content:                entries,
		Revocations:            []moat.Revocation{},
	}
}

func registryStatusEntry(name, typ, digit string) moat.ContentEntry {
	return moat.ContentEntry{
		Name:        name,
		DisplayName: name,
		Type:        typ,
		ContentHash: "sha256:" + strings.Repeat(digit, 64),
		SourceURI:   "fixture-source",
		AttestedAt:  registryStatusTestNow(),
	}
}

func registryStatusManifestBytes(t *testing.T, manifest *moat.Manifest) []byte {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return data
}

func registryStatusTestNow() time.Time {
	return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
}

func updateBareGitRegistryForStatusDiffTest(t *testing.T, work, bare, branch string) {
	t.Helper()
	writeRepoFileForSyncTest(t, work, "rules/claude-code/updated-rule.md", "# Updated Rule\n\nv2\n")
	writeRepoFileForSyncTest(t, work, "skills/new-thing/SKILL.md", "# New Thing\n")
	gitRunForSyncTest(t, work, "add", "-A")
	gitRunForSyncTest(t, work, "commit", "-m", "update content for status")
	gitRunForSyncTest(t, work, "push", bare, "HEAD:refs/heads/"+branch)
}
