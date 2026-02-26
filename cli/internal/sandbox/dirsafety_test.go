package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDir_BlocksSensitivePaths(t *testing.T) {
	home, _ := os.UserHomeDir()

	cases := []string{"/", "/tmp", "/etc", "/var", "/opt", home}
	for _, dir := range cases {
		err := ValidateDir(dir, false)
		if err == nil {
			t.Errorf("expected error for sensitive path %q, got nil", dir)
		}
		if _, ok := err.(*DirSafetyError); !ok {
			t.Errorf("expected DirSafetyError for %q, got %T", dir, err)
		}
	}
}

func TestValidateDir_DepthCheck(t *testing.T) {
	home, _ := os.UserHomeDir()

	// 1 level deep — should fail (no marker, but depth check comes first if it passes blocklist)
	shallow := filepath.Join(home, "projects")
	// Create it temporarily
	os.MkdirAll(shallow, 0755)
	defer os.RemoveAll(shallow)

	err := ValidateDir(shallow, false)
	if err == nil {
		t.Error("expected error for shallow directory (1 level), got nil")
	}

	// 2 levels deep with marker — should pass
	deep := filepath.Join(home, "test-sandbox-validate", "myapp")
	os.MkdirAll(deep, 0755)
	defer os.RemoveAll(filepath.Join(home, "test-sandbox-validate"))

	// Add a project marker
	os.WriteFile(filepath.Join(deep, "go.mod"), []byte("module test"), 0644)

	err = ValidateDir(deep, false)
	if err != nil {
		t.Errorf("expected nil for 2-level deep dir with marker, got: %v", err)
	}
}

func TestValidateDir_RequiresMarker(t *testing.T) {
	home, _ := os.UserHomeDir()
	noMarker := filepath.Join(home, "test-sandbox-validate2", "nomarker")
	os.MkdirAll(noMarker, 0755)
	defer os.RemoveAll(filepath.Join(home, "test-sandbox-validate2"))

	err := ValidateDir(noMarker, false)
	if err == nil {
		t.Error("expected error for directory without project marker, got nil")
	}

	// Add go.mod → should now pass
	os.WriteFile(filepath.Join(noMarker, "go.mod"), []byte("module test"), 0644)
	err = ValidateDir(noMarker, false)
	if err != nil {
		t.Errorf("expected nil after adding go.mod marker, got: %v", err)
	}
}

func TestValidateDir_SymlinkResolution(t *testing.T) {
	home, _ := os.UserHomeDir()
	linkDir := filepath.Join(home, "test-sandbox-validate3", "symproj")
	os.MkdirAll(filepath.Dir(linkDir), 0755)
	defer os.RemoveAll(filepath.Join(home, "test-sandbox-validate3"))

	// Create symlink to / — should fail after resolution
	err := os.Symlink("/", linkDir)
	if err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	err = ValidateDir(linkDir, false)
	if err == nil {
		t.Error("expected error for symlink pointing to /, got nil")
	}
}

func TestValidateDir_ForceDir(t *testing.T) {
	// Even "/" should pass with forceDir
	err := ValidateDir("/", true)
	if err != nil {
		t.Errorf("expected nil with forceDir=true, got: %v", err)
	}
}
