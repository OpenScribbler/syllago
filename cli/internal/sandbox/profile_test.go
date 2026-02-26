package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileFor_UnknownProvider(t *testing.T) {
	_, err := ProfileFor("bad-provider", "/home/user", "/tmp/project")
	if err == nil {
		t.Error("expected error for unknown provider, got nil")
	}
}

func TestProfileFor_Windsurf(t *testing.T) {
	_, err := ProfileFor("windsurf", "/home/user", "/tmp/project")
	if err == nil {
		t.Error("expected error for windsurf (not supported in v1)")
	}
}

func TestEcosystemDomains_GoMod(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	domains := EcosystemDomains(dir)
	found := make(map[string]bool)
	for _, d := range domains {
		found[d] = true
	}
	if !found["proxy.golang.org"] {
		t.Error("expected proxy.golang.org for go.mod project")
	}
	if !found["sum.golang.org"] {
		t.Error("expected sum.golang.org for go.mod project")
	}
}

func TestEcosystemDomains_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)

	domains := EcosystemDomains(dir)
	found := make(map[string]bool)
	for _, d := range domains {
		found[d] = true
	}
	if !found["*.npmjs.org"] {
		t.Error("expected *.npmjs.org for package.json project")
	}
}

func TestEcosystemDomains_MultipleMarkers(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644)

	domains := EcosystemDomains(dir)
	found := make(map[string]bool)
	for _, d := range domains {
		found[d] = true
	}
	if !found["proxy.golang.org"] {
		t.Error("expected Go domains")
	}
	if !found["crates.io"] {
		t.Error("expected Rust domains")
	}

	// No duplicates
	seen := make(map[string]bool)
	for _, d := range domains {
		if seen[d] {
			t.Errorf("duplicate domain: %s", d)
		}
		seen[d] = true
	}
}

func TestEcosystemDomains_NoMarkers(t *testing.T) {
	dir := t.TempDir()
	domains := EcosystemDomains(dir)
	if len(domains) != 0 {
		t.Errorf("expected empty domains for dir with no markers, got %v", domains)
	}
}

func TestEcosystemCacheMounts_OnlyExisting(t *testing.T) {
	dir := t.TempDir()
	homeDir := t.TempDir()

	// Create a go.mod in the project
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	// Don't create any cache dirs — should return empty
	mounts := EcosystemCacheMounts(dir, homeDir)
	if len(mounts) != 0 {
		t.Errorf("expected no mounts (no cache dirs exist), got %v", mounts)
	}

	// Create one cache dir — should appear
	goCache := filepath.Join(homeDir, ".cache", "go-build")
	os.MkdirAll(goCache, 0755)
	mounts = EcosystemCacheMounts(dir, homeDir)
	if len(mounts) != 1 || mounts[0] != goCache {
		t.Errorf("expected [%s], got %v", goCache, mounts)
	}
}
