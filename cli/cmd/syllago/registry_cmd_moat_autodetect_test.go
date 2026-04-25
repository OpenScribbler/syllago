package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
)

// stubClone swaps the orchestrator's clone seam (registryops.CloneFn) with a
// stub that creates a fake clone dir at registry.CloneDir(name) containing a
// registry.yaml with the given content. Restored on t.Cleanup.
func stubClone(t *testing.T, yamlContent string) {
	t.Helper()
	orig := registryops.CloneFn
	registryops.CloneFn = func(url, name, ref string) error {
		cloneDir, err := registry.CloneDir(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(cloneDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte(yamlContent), 0644)
	}
	t.Cleanup(func() { registryops.CloneFn = orig })
}

func TestRegistryAutoMOAT_AllowlistURL_SetsManifestURI(t *testing.T) {
	// No t.Parallel — swaps package-level cloneFn and registry.OverrideProbeForTest.
	// registry add for the meta-registry URL must auto-set type=moat + ManifestURI
	// from the bundled allowlist — no --moat flag required.
	root := withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })

	// Fake clone: minimal registry.yaml with no manifest_uri — allowlist provides it.
	stubClone(t, "name: syllago-meta-registry\nversion: \"1.0\"\n")

	err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/OpenScribbler/syllago-meta-registry"})
	if err != nil {
		t.Fatalf("registry add failed: %v", err)
	}

	_ = root
	got, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(got.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(got.Registries))
	}
	r := got.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected type=moat, got %q", r.Type)
	}
	if r.ManifestURI == "" {
		t.Error("expected non-empty ManifestURI from allowlist auto-detection")
	}
	wantPrefix := "https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/"
	if !strings.HasPrefix(r.ManifestURI, wantPrefix) {
		t.Errorf("ManifestURI %q does not start with %q", r.ManifestURI, wantPrefix)
	}
}

func TestRegistryAutoMOAT_RegistryYAML_SetsManifestURI(t *testing.T) {
	// No t.Parallel — swaps package-level cloneFn and registry.OverrideProbeForTest.
	// registry add for a non-allowlisted URL that declares manifest_uri in registry.yaml
	// must auto-set type=moat + ManifestURI from that self-declaration.
	root := withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })

	const testURL = "https://github.com/example/non-allowlisted-registry"
	const wantManifestURI = "https://raw.githubusercontent.com/example/non-allowlisted-registry/moat-registry/registry.json"

	stubClone(t, "name: non-allowlisted-registry\nversion: \"1.0\"\nmanifest_uri: "+wantManifestURI+"\n")

	err := registryAddCmd.RunE(registryAddCmd, []string{testURL})
	if err != nil {
		t.Fatalf("registry add failed: %v", err)
	}

	_ = root
	got, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(got.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(got.Registries))
	}
	r := got.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected type=moat, got %q", r.Type)
	}
	if r.ManifestURI != wantManifestURI {
		t.Errorf("expected ManifestURI %q, got %q", wantManifestURI, r.ManifestURI)
	}
}

func TestRegistryList_TrustColumn(t *testing.T) {
	// registry list must show the TRUST column header and "moat" for a synced MOAT registry.
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{
				Name:          "example/moat-reg",
				URL:           "https://github.com/example/moat-reg",
				Type:          config.RegistryTypeMOAT,
				ManifestURI:   "https://raw.githubusercontent.com/example/moat-reg/moat-registry/registry.json",
				LastFetchedAt: &now,
			},
			{
				Name: "example/plain-reg",
				URL:  "https://github.com/example/plain-reg",
			},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	stdout, _ := output.SetForTest(t)

	registryListCmd.SilenceUsage = true
	registryListCmd.SilenceErrors = true

	if err := registryListCmd.RunE(registryListCmd, nil); err != nil {
		t.Fatalf("registryListCmd.RunE: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "TRUST") {
		t.Errorf("expected TRUST column header in output, got:\n%s", got)
	}
	if !strings.Contains(got, "moat") {
		t.Errorf("expected 'moat' in TRUST column for MOAT registry, got:\n%s", got)
	}
}
