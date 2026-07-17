package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/acif"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func unsetACIFInstallEntryPointsEnv(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv(acif.InstallEntryPointsPathEnv)
	if err := os.Unsetenv(acif.InstallEntryPointsPathEnv); err != nil {
		t.Fatalf("unset %s: %v", acif.InstallEntryPointsPathEnv, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(acif.InstallEntryPointsPathEnv, old)
			return
		}
		_ = os.Unsetenv(acif.InstallEntryPointsPathEnv)
	})
}

func TestACIFContentType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ct   catalog.ContentType
		want string
	}{
		{catalog.Skills, "skill"},
		{catalog.Agents, "agent"},
		{catalog.MCP, "mcp_config"},
		{catalog.Rules, "rule"},
		{catalog.Hooks, "hook"},
		{catalog.Commands, "command"},
		{catalog.Loadouts, ""},
		{catalog.SearchResults, ""},
		{catalog.Library, ""},
	}
	for _, tc := range tests {
		t.Run(string(tc.ct), func(t *testing.T) {
			t.Parallel()
			if got := acifContentType(tc.ct); got != tc.want {
				t.Errorf("acifContentType(%s) = %q, want %q", tc.ct, got, tc.want)
			}
		})
	}
}

func TestMatrixInstallDirVendoredRows(t *testing.T) {
	unsetACIFInstallEntryPointsEnv(t)

	tests := []struct {
		name         string
		providerSlug string
		ct           catalog.ContentType
		want         string
		wantOK       bool
	}{
		{
			name:         "directory_of_files trims content directory",
			providerSlug: "claude-code",
			ct:           catalog.Skills,
			want:         "/home/u/.claude/skills",
			wantOK:       true,
		},
		{
			name:         "single_file trims content file",
			providerSlug: "claude-code",
			ct:           catalog.Agents,
			want:         "/home/u/.claude/agents",
			wantOK:       true,
		},
		{
			name:         "merged layout returns JSON merge sentinel",
			providerSlug: "claude-code",
			ct:           catalog.Hooks,
			want:         provider.JSONMergeSentinel,
			wantOK:       true,
		},
		{
			name:         "no user row falls back to legacy caller",
			providerSlug: "claude-code",
			ct:           catalog.MCP,
			wantOK:       false,
		},
		{
			name:         "virtual type has no matrix mapping",
			providerSlug: "claude-code",
			ct:           catalog.Loadouts,
			wantOK:       false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := matrixInstallDir(tc.providerSlug, tc.ct, "/home/u")
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("matrixInstallDir = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMatrixInstallDirTemplateValidation(t *testing.T) {
	matrixPath := filepath.Join(t.TempDir(), "install-entry-points.yaml")
	if err := os.WriteFile(matrixPath, []byte(`
install_entry_points:
  valid-dir:
    skill:
      - {scope: user, path_template: "~/.valid/skills/<content-name>/", layout: directory_of_files, status: current}
  valid-file:
    skill:
      - {scope: user, path_template: "~/.valid/rules/<content-name>.md", layout: single_file, status: current}
  invalid-nonfinal:
    skill:
      - {scope: user, path_template: "~/.bad/<content-name>/rule.md", layout: single_file, status: current}
  invalid-missing:
    skill:
      - {scope: user, path_template: "~/.bad/static.md", layout: single_file, status: current}
  valid-merged:
    hook:
      - {scope: user, path_template: "~/.valid/settings.json", layout: merged_into_shared_file, status: current}
  gated-merged:
    skill:
      - {scope: user, path_template: "~/.valid/settings.md", layout: merged_into_shared_file, status: current}
`), 0644); err != nil {
		t.Fatalf("write matrix fixture: %v", err)
	}
	t.Setenv(acif.InstallEntryPointsPathEnv, matrixPath)

	tests := []struct {
		providerSlug string
		want         string
		wantOK       bool
	}{
		{"valid-dir", "/home/u/.valid/skills", true},
		{"valid-file", "/home/u/.valid/rules", true},
		{"invalid-nonfinal", "", false},
		{"invalid-missing", "", false},
		{"gated-merged", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.providerSlug, func(t *testing.T) {
			got, ok := matrixInstallDir(tc.providerSlug, catalog.Skills, "/home/u")
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("matrixInstallDir = %q, want %q", got, tc.want)
			}
		})
	}

	got, ok := matrixInstallDir("valid-merged", catalog.Hooks, "/home/u")
	if !ok || got != provider.JSONMergeSentinel {
		t.Errorf("matrixInstallDir(valid-merged hooks) = %q, %v; want JSON merge sentinel, true", got, ok)
	}
}

func TestResolverDefaultUsesMatrixBeforeLegacy(t *testing.T) {
	unsetACIFInstallEntryPointsEnv(t)

	prov := stubProvider("claude-code")
	r := NewResolver(&Config{}, "")

	got := r.InstallDir(prov, catalog.Skills, "/home/u")
	want := "/home/u/.claude/skills"
	if got != want {
		t.Errorf("InstallDir default = %q, want matrix path %q", got, want)
	}
}

func TestResolverOverridesBypassMatrix(t *testing.T) {
	unsetACIFInstallEntryPointsEnv(t)

	prov := stubProvider("claude-code")
	tests := []struct {
		name string
		cfg  *Config
		cli  string
		want string
	}{
		{
			name: "per type",
			cfg: &Config{ProviderPaths: map[string]ProviderPathConfig{
				"claude-code": {Paths: map[string]string{"skills": "/custom/skills"}},
			}},
			want: "/custom/skills",
		},
		{
			name: "cli base dir",
			cfg:  &Config{},
			cli:  "/cli/base",
			want: "/cli/base/.provider/skills",
		},
		{
			name: "config base dir",
			cfg: &Config{ProviderPaths: map[string]ProviderPathConfig{
				"claude-code": {BaseDir: "/config/base"},
			}},
			want: "/config/base/.provider/skills",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := NewResolver(tc.cfg, tc.cli)
			got := r.InstallDir(prov, catalog.Skills, "/home/u")
			if got != tc.want {
				t.Errorf("InstallDir = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolverDefaultPreservesProjectScopeSentinel(t *testing.T) {
	unsetACIFInstallEntryPointsEnv(t)

	prov := stubProvider("cline")
	prov.InstallDir = func(_ string, ct catalog.ContentType) string {
		if ct == catalog.Rules {
			return provider.ProjectScopeSentinel
		}
		return ""
	}

	r := NewResolver(&Config{}, "")
	got := r.InstallDir(prov, catalog.Rules, "/home/u")
	if got != provider.ProjectScopeSentinel {
		t.Errorf("InstallDir = %q, want project scope sentinel", got)
	}
}
