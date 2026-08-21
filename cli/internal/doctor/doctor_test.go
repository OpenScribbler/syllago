package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func doctorLinkProvider(slug string, dirs map[catalog.ContentType]string) provider.Provider {
	return provider.Provider{
		Name: slug,
		Slug: slug,
		InstallDir: func(home string, ct catalog.ContentType) string {
			return dirs[ct]
		},
	}
}

func TestCheckProviderLinksAt_AllHealthy(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, ".syllago")
	source := filepath.Join(root, "content", "skills", "healthy")
	installDir := filepath.Join(tmp, ".claude", "skills")
	linkPath := filepath.Join(installDir, "healthy")
	writeDoctorFile(t, source, "skill")
	symlinkDoctor(t, source, linkPath)

	result := checkProviderLinksAt(tmp, []provider.Provider{
		doctorLinkProvider("claude-code", map[catalog.ContentType]string{catalog.Skills: installDir}),
	}, []string{root})

	if result.Name != "provider-links" {
		t.Fatalf("Name = %q, want provider-links", result.Name)
	}
	if result.Status != CheckOK {
		t.Fatalf("Status = %s, want %s: %#v", result.Status, CheckOK, result)
	}
	if result.Message != "Provider links: 1 healthy" {
		t.Fatalf("Message = %q, want healthy count", result.Message)
	}
}

func TestCheckProviderLinksAt_ReportsBroken(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, ".syllago")
	installDir := filepath.Join(tmp, ".claude", "skills")
	linkPath := filepath.Join(installDir, "broken")
	target := filepath.Join(root, "content", "skills", "broken")
	symlinkDoctor(t, target, linkPath)

	result := checkProviderLinksAt(tmp, []provider.Provider{
		doctorLinkProvider("claude-code", map[catalog.ContentType]string{catalog.Skills: installDir}),
	}, []string{root})

	if result.Status != CheckErr {
		t.Fatalf("Status = %s, want %s: %#v", result.Status, CheckErr, result)
	}
	if result.Message != "Provider links: 1 broken of 1 total" {
		t.Fatalf("Message = %q, want broken count", result.Message)
	}
	if len(result.Details) != 2 {
		t.Fatalf("Details = %#v, want broken line plus fix hint", result.Details)
	}
	if !strings.Contains(result.Details[0], "broken: "+linkPath+" -> "+target) {
		t.Fatalf("Details[0] = %q, want broken detail", result.Details[0])
	}
	if result.Details[1] != "Run 'syllago doctor --fix' to repair" {
		t.Fatalf("Details[1] = %q, want fix hint", result.Details[1])
	}
}

func writeDoctorFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func symlinkDoctor(t *testing.T, target, linkPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
}
