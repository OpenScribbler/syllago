package provider

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestClineSupportsTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Skills, catalog.Hooks, catalog.MCP, catalog.Commands} {
		if !Cline.SupportsType(ct) {
			t.Errorf("Cline.SupportsType(%s) = false, want true", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Agents} {
		if Cline.SupportsType(ct) {
			t.Errorf("Cline.SupportsType(%s) = true, want false", ct)
		}
	}
}

func TestClineSkillsSupport(t *testing.T) {
	t.Parallel()

	home := "/home/testuser"
	installDir := Cline.InstallDir(home, catalog.Skills)
	wantInstall := "/home/testuser/.cline/skills"
	if installDir != wantInstall {
		t.Errorf("Cline.InstallDir(Skills) = %q, want %q", installDir, wantInstall)
	}

	paths := Cline.DiscoveryPaths("/tmp/project", catalog.Skills)
	if len(paths) == 0 {
		t.Fatal("expected at least one discovery path for Skills")
	}

	var foundCline, foundClinerules, foundClaude bool
	for _, p := range paths {
		switch {
		case strings.HasSuffix(p, ".cline/skills") && strings.Contains(p, "/tmp/project"):
			foundCline = true
		case strings.HasSuffix(p, ".clinerules/skills") && strings.Contains(p, "/tmp/project"):
			foundClinerules = true
		case strings.HasSuffix(p, ".claude/skills") && strings.Contains(p, "/tmp/project"):
			foundClaude = true
		}
	}
	if !foundCline {
		t.Errorf("expected project discovery path ending in .cline/skills, got: %v", paths)
	}
	if !foundClinerules {
		t.Errorf("expected project discovery path ending in .clinerules/skills, got: %v", paths)
	}
	if !foundClaude {
		t.Errorf("expected project discovery path ending in .claude/skills (interop), got: %v", paths)
	}

	if got := Cline.FileFormat(catalog.Skills); got != FormatMarkdown {
		t.Errorf("Cline.FileFormat(Skills) = %q, want %q", got, FormatMarkdown)
	}

	if !Cline.SymlinkSupport[catalog.Skills] {
		t.Errorf("Cline.SymlinkSupport[Skills] = false, want true")
	}
}

func TestClineCommandsSupport(t *testing.T) {
	t.Parallel()

	home := "/home/testuser"
	installDir := Cline.InstallDir(home, catalog.Commands)
	wantInstall := "/home/testuser/Documents/Cline/Workflows"
	if installDir != wantInstall {
		t.Errorf("Cline.InstallDir(Commands) = %q, want %q", installDir, wantInstall)
	}

	paths := Cline.DiscoveryPaths("/tmp/project", catalog.Commands)
	if len(paths) == 0 {
		t.Fatal("expected at least one discovery path for Commands")
	}
	foundProject := false
	for _, p := range paths {
		if strings.HasSuffix(p, ".clinerules/workflows") && strings.Contains(p, "/tmp/project") {
			foundProject = true
		}
	}
	if !foundProject {
		t.Errorf("expected project discovery path ending in .clinerules/workflows, got: %v", paths)
	}

	if got := Cline.FileFormat(catalog.Commands); got != FormatMarkdown {
		t.Errorf("Cline.FileFormat(Commands) = %q, want %q", got, FormatMarkdown)
	}

	if !Cline.SymlinkSupport[catalog.Commands] {
		t.Errorf("Cline.SymlinkSupport[Commands] = false, want true")
	}
}

func TestClineDiscoveryPaths(t *testing.T) {
	t.Parallel()
	paths := Cline.DiscoveryPaths("/tmp/project", catalog.Rules)
	if len(paths) == 0 {
		t.Fatal("expected at least one discovery path for Rules, got none")
	}
	found := false
	for _, p := range paths {
		if strings.HasSuffix(p, ".clinerules") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a discovery path ending in .clinerules, got: %v", paths)
	}
}

func TestClineEmitPath(t *testing.T) {
	t.Parallel()
	path := Cline.EmitPath("/tmp/project")
	if path == "" {
		t.Fatal("expected non-empty emit path")
	}
	if !strings.HasSuffix(path, ".clinerules") {
		t.Errorf("expected emit path ending in .clinerules, got %q", path)
	}
}

func TestClineFileFormat(t *testing.T) {
	t.Parallel()
	if got := Cline.FileFormat(catalog.Rules); got != FormatMarkdown {
		t.Errorf("Cline.FileFormat(Rules) = %q, want %q", got, FormatMarkdown)
	}
	if got := Cline.FileFormat(catalog.MCP); got != FormatJSON {
		t.Errorf("Cline.FileFormat(MCP) = %q, want %q", got, FormatJSON)
	}
}

func TestClineMCPSettingsPath(t *testing.T) {
	t.Parallel()
	path := ClineMCPSettingsPath()

	// The function returns a non-empty path on all platforms (it calls os.UserHomeDir
	// which succeeds in test environments).
	if path == "" {
		t.Fatal("ClineMCPSettingsPath returned empty string")
	}

	// Verify the path ends with the expected suffix regardless of platform.
	wantSuffix := "cline_mcp_settings.json"
	if !strings.HasSuffix(path, wantSuffix) {
		t.Errorf("ClineMCPSettingsPath: got %q, want suffix %q", path, wantSuffix)
	}

	// Verify the path contains the VS Code globalStorage segment.
	if !strings.Contains(path, "globalStorage") {
		t.Errorf("ClineMCPSettingsPath: expected path to contain 'globalStorage', got %q", path)
	}

	// Platform-specific path segment check.
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(path, "Application Support") {
			t.Errorf("on darwin, expected 'Application Support' in path, got %q", path)
		}
	case "linux":
		if !strings.Contains(path, ".config") {
			t.Errorf("on linux, expected '.config' in path, got %q", path)
		}
	}
}

// TestClineMCPSettingsPathFor_AllPlatforms exercises every GOOS branch of
// the path builder. The outer ClineMCPSettingsPath runs against whatever
// runtime.GOOS the test process happens to have, so the other branches
// would be permanently dark without this.
func TestClineMCPSettingsPathFor_AllPlatforms(t *testing.T) {
	t.Parallel()

	const extSegment = "saoudrizwan.claude-dev"
	const leaf = "cline_mcp_settings.json"

	cases := []struct {
		name        string
		goos        string
		home        string
		appdata     string
		wantContain []string
	}{
		{
			name:        "darwin uses Library/Application Support",
			goos:        "darwin",
			home:        "/Users/alice",
			appdata:     "",
			wantContain: []string{"/Users/alice", "Library/Application Support", "globalStorage", extSegment, leaf},
		},
		{
			name:        "windows with APPDATA set uses APPDATA directly",
			goos:        "windows",
			home:        `C:\Users\alice`,
			appdata:     `C:\Users\alice\AppData\Roaming`,
			wantContain: []string{`C:\Users\alice\AppData\Roaming`, "globalStorage", extSegment, leaf},
		},
		{
			name:        "windows with APPDATA unset falls back to home/AppData/Roaming",
			goos:        "windows",
			home:        `C:\Users\bob`,
			appdata:     "",
			wantContain: []string{filepath.Join(`C:\Users\bob`, "AppData", "Roaming"), "globalStorage", extSegment, leaf},
		},
		{
			name:        "linux uses .config",
			goos:        "linux",
			home:        "/home/carol",
			appdata:     "",
			wantContain: []string{"/home/carol/.config", "globalStorage", extSegment, leaf},
		},
		{
			name:        "unrecognised GOOS falls through default (.config)",
			goos:        "freebsd",
			home:        "/home/dave",
			appdata:     "",
			wantContain: []string{"/home/dave/.config", "globalStorage", extSegment, leaf},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := clineMCPSettingsPathFor(tc.goos, tc.home, tc.appdata)
			if got == "" {
				t.Fatalf("clineMCPSettingsPathFor(%q, %q, %q) = empty", tc.goos, tc.home, tc.appdata)
			}
			for _, want := range tc.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("path %q missing expected segment %q", got, want)
				}
			}
			// Windows branch must never route through the Linux .config path
			// (regression guard against the branches accidentally merging).
			if tc.goos == "windows" && strings.Contains(got, ".config") {
				t.Errorf("windows path should not contain '.config', got %q", got)
			}
			// Darwin branch must never route through the Linux .config path.
			if tc.goos == "darwin" && strings.Contains(got, ".config") {
				t.Errorf("darwin path should not contain '.config', got %q", got)
			}
		})
	}
}
