package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

const libraryGateRegistry = "moat-reg"

// setupLibraryCopy builds a Library holding my-skill as a copy staged from
// a MOAT registry at copyHash, a cached manifest for that registry listing
// a newer version with revocations, and a provider to install to. It
// returns the provider's install base.
func setupLibraryCopy(t *testing.T, copyHash string, revocations []moat.Revocation) string {
	t.Helper()
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	if err := metadata.Save(filepath.Join(globalDir, "skills", "my-skill"), &metadata.Meta{
		Name:           "my-skill",
		SourceRegistry: libraryGateRegistry,
		SourceHash:     copyHash,
	}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })
	const uri = "https://registry.example.com/manifest.json"
	if err := config.SaveGlobal(&config.Config{Registries: []config.Registry{{
		Name:        libraryGateRegistry,
		Type:        config.RegistryTypeMOAT,
		ManifestURI: uri,
	}}}); err != nil {
		t.Fatalf("save global config: %v", err)
	}
	entry := signedManifestEntry("my-skill", "sha256:"+strings.Repeat("f", 64))
	entry.DisplayName, entry.SourceURI, entry.AttestedAt = "my-skill", "fixture-source", registryStatusTestNow()
	manifest := registryStatusManifest(t, "2026-05-01T00:00:00Z", []moat.ContentEntry{entry})
	manifest.ManifestURI = uri
	manifest.Revocations = append(manifest.Revocations, revocations...)
	path, err := moat.ManifestCachePath(config.GlobalDirOverride, libraryGateRegistry)
	if err != nil {
		t.Fatalf("ManifestCachePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir manifest cache: %v", err)
	}
	if err := os.WriteFile(path, registryStatusManifestBytes(t, manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	projectRoot := t.TempDir()
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	installBase := t.TempDir()
	addTestProviderOpts(t, "gate-prov", "Gate Provider", installBase, true)
	return installBase
}

func installedSkill(installBase string) bool {
	_, err := os.Lstat(filepath.Join(installBase, "skills", "my-skill"))
	return err == nil
}

// A Library copy of a version its registry has revoked is refused by both
// install paths, though the registry now lists a newer version.
func TestInstall_LibraryCopyOfRevokedVersionRefused(t *testing.T) {
	copyHash := "sha256:" + strings.Repeat("a", 64)
	for _, flag := range []string{"to", "to-all"} {
		t.Run(flag, func(t *testing.T) {
			installBase := setupLibraryCopy(t, copyHash, []moat.Revocation{{
				ContentHash: copyHash,
				Reason:      moat.RevocationReasonMalicious,
				Source:      moat.RevocationSourceRegistry,
				DetailsURL:  "https://example.com/rev",
			}})
			_, _ = output.SetForTest(t)
			if flag == "to" {
				installCmd.Flags().Set("to", "gate-prov")
				defer installCmd.Flags().Set("to", "")
			} else {
				installCmd.Flags().Set("to-all", "true")
				defer installCmd.Flags().Set("to-all", "false")
			}

			err := installCmd.RunE(installCmd, []string{"my-skill"})
			assertStructuredCode(t, err, output.ErrMoatRevocationBlock)
			if installedSkill(installBase) {
				t.Error("the revoked copy was installed")
			}
		})
	}
}

// The JSON result lists a refused copy as skipped, and the command still
// fails.
func TestInstall_LibraryCopyRefusalInJSON(t *testing.T) {
	copyHash := "sha256:" + strings.Repeat("a", 64)
	setupLibraryCopy(t, copyHash, []moat.Revocation{{
		ContentHash: copyHash,
		Reason:      moat.RevocationReasonMalicious,
		Source:      moat.RevocationSourceRegistry,
		DetailsURL:  "https://example.com/rev",
	}})
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	installCmd.Flags().Set("to", "gate-prov")
	defer installCmd.Flags().Set("to", "")

	err := installCmd.RunE(installCmd, []string{"my-skill"})
	assertStructuredCode(t, err, output.ErrMoatRevocationBlock)
	var result installResult
	if jerr := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); jerr != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", jerr, stdout.String())
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Name != "my-skill" {
		t.Errorf("Skipped = %+v, want my-skill", result.Skipped)
	}
}

// A headless install of a recalled copy exits with the publisher
// revocation code before anything installs.
func TestInstall_LibraryCopyPublisherWarnHeadlessExits(t *testing.T) {
	copyHash := "sha256:" + strings.Repeat("a", 64)
	installBase := setupLibraryCopy(t, copyHash, []moat.Revocation{{
		ContentHash: copyHash,
		Reason:      moat.RevocationReasonDeprecated,
		Source:      moat.RevocationSourcePublisher,
		DetailsURL:  "https://example.com/rev",
	}})
	capturedExit := withInstallGateStubs(t, false, false)
	_, _ = output.SetForTest(t)
	installCmd.Flags().Set("to", "gate-prov")
	defer installCmd.Flags().Set("to", "")

	if err := installCmd.RunE(installCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("headless publisher warn should exit through moatSyncExit; got %v", err)
	}
	if *capturedExit != moat.ExitMoatPublisherRevocation {
		t.Errorf("exit code = %d, want %d", *capturedExit, moat.ExitMoatPublisherRevocation)
	}
	if installedSkill(installBase) {
		t.Error("the recalled copy was installed")
	}
}

// An interactive yes installs a recalled copy; a no leaves it out without
// failing the command.
func TestInstall_LibraryCopyPublisherWarnInteractive(t *testing.T) {
	copyHash := "sha256:" + strings.Repeat("a", 64)
	for _, yes := range []bool{true, false} {
		t.Run(map[bool]string{true: "yes", false: "no"}[yes], func(t *testing.T) {
			installBase := setupLibraryCopy(t, copyHash, []moat.Revocation{{
				ContentHash: copyHash,
				Reason:      moat.RevocationReasonDeprecated,
				Source:      moat.RevocationSourcePublisher,
				DetailsURL:  "https://example.com/rev",
			}})
			withInstallGateStubs(t, true, yes)
			_, _ = output.SetForTest(t)
			installCmd.Flags().Set("to", "gate-prov")
			defer installCmd.Flags().Set("to", "")

			if err := installCmd.RunE(installCmd, []string{"my-skill"}); err != nil {
				t.Fatalf("install: %v", err)
			}
			if installedSkill(installBase) != yes {
				t.Errorf("installed = %v, want %v", installedSkill(installBase), yes)
			}
		})
	}
}

// A copy of a version no revocation names installs as before.
func TestInstall_LibraryCopyUnrevokedInstalls(t *testing.T) {
	installBase := setupLibraryCopy(t, "sha256:"+strings.Repeat("a", 64), nil)
	_, _ = output.SetForTest(t)
	installCmd.Flags().Set("to", "gate-prov")
	defer installCmd.Flags().Set("to", "")

	if err := installCmd.RunE(installCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if !installedSkill(installBase) {
		t.Error("the copy was not installed")
	}
}
