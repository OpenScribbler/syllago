package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHelperAppDataDir(t *testing.T) {
	got := appDataDir("/home/u", "Cursor")
	switch runtime.GOOS {
	case "linux":
		want := filepath.Join("/home/u", ".config", "Cursor")
		if got != want {
			t.Errorf("appDataDir on linux = %q, want %q", got, want)
		}
	case "darwin":
		want := filepath.Join("/home/u", "Library", "Application Support", "Cursor")
		if got != want {
			t.Errorf("appDataDir on darwin = %q, want %q", got, want)
		}
	default:
		if got != "" {
			t.Errorf("appDataDir on %s = %q, want empty (deferred)", runtime.GOOS, got)
		}
	}
}

func TestHelperBinaryOnPath(t *testing.T) {
	// /bin/sh is required to exist on any POSIX-y system.
	if !binaryOnPath("sh") {
		t.Error("binaryOnPath(\"sh\") = false, want true")
	}
	if binaryOnPath("syllago-nonexistent-binary-xyz123") {
		t.Error("binaryOnPath of nonexistent binary returned true")
	}
}

func TestHelperBinaryOnPath_EmptyPATH(t *testing.T) {
	t.Setenv("PATH", "/syllago-nonexistent-dir-xyz")
	if binaryOnPath("sh") {
		t.Error("binaryOnPath(\"sh\") with empty PATH returned true")
	}
}

func TestHelperFileExists(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.txt")
	if fileExists(missing) {
		t.Error("fileExists(missing) = true, want false")
	}

	f := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(f) {
		t.Error("fileExists(real) = false, want true")
	}

	// A directory must NOT count as a file.
	if fileExists(dir) {
		t.Error("fileExists(dir) = true, want false")
	}
}

func TestHelperDirExists(t *testing.T) {
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Error("dirExists(tempdir) = false, want true")
	}

	missing := filepath.Join(dir, "missing")
	if dirExists(missing) {
		t.Error("dirExists(missing) = true, want false")
	}

	f := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if dirExists(f) {
		t.Error("dirExists(file) = true, want false")
	}
}

func TestHelperVSCodeExtensionInstalled(t *testing.T) {
	home := t.TempDir()

	// No .vscode/extensions dir → false (must not panic).
	if vscodeExtensionInstalled(home, "test.ext") {
		t.Error("vscodeExtensionInstalled with no .vscode dir returned true")
	}

	extDir := filepath.Join(home, ".vscode", "extensions")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Empty extensions dir → false.
	if vscodeExtensionInstalled(home, "test.ext") {
		t.Error("vscodeExtensionInstalled with empty extensions dir returned true")
	}

	// Different extension → false.
	if err := os.MkdirAll(filepath.Join(extDir, "other.ext-1.0.0"), 0755); err != nil {
		t.Fatal(err)
	}
	if vscodeExtensionInstalled(home, "test.ext") {
		t.Error("vscodeExtensionInstalled with non-matching extension returned true")
	}

	// Matching extension dir → true.
	if err := os.MkdirAll(filepath.Join(extDir, "test.ext-1.2.3"), 0755); err != nil {
		t.Fatal(err)
	}
	if !vscodeExtensionInstalled(home, "test.ext") {
		t.Error("vscodeExtensionInstalled with matching extension returned false")
	}
}

func TestHelperGhExtensionInstalled_NoBinary(t *testing.T) {
	origLook := ghLookPath
	t.Cleanup(func() { ghLookPath = origLook })

	ghLookPath = func(name string) (string, error) {
		return "", os.ErrNotExist
	}
	if ghExtensionInstalled("gh-copilot") {
		t.Error("ghExtensionInstalled returned true when gh not on PATH")
	}
}

func TestHelperGhExtensionInstalled_NotInstalled(t *testing.T) {
	origLook := ghLookPath
	origRun := ghRunCommand
	t.Cleanup(func() {
		ghLookPath = origLook
		ghRunCommand = origRun
	})

	ghLookPath = func(name string) (string, error) {
		return "/usr/bin/gh", nil
	}
	ghRunCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("github/some-other-extension\n"), nil
	}
	if ghExtensionInstalled("gh-copilot") {
		t.Error("ghExtensionInstalled returned true when output does not contain name")
	}
}

func TestHelperGhExtensionInstalled_Installed(t *testing.T) {
	origLook := ghLookPath
	origRun := ghRunCommand
	t.Cleanup(func() {
		ghLookPath = origLook
		ghRunCommand = origRun
	})

	ghLookPath = func(name string) (string, error) {
		return "/usr/bin/gh", nil
	}
	ghRunCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("github/gh-copilot\nother/extension\n"), nil
	}
	if !ghExtensionInstalled("gh-copilot") {
		t.Error("ghExtensionInstalled returned false when output contains name")
	}
}

func TestHelperGhExtensionInstalled_CommandError(t *testing.T) {
	origLook := ghLookPath
	origRun := ghRunCommand
	t.Cleanup(func() {
		ghLookPath = origLook
		ghRunCommand = origRun
	})

	ghLookPath = func(name string) (string, error) {
		return "/usr/bin/gh", nil
	}
	ghRunCommand = func(name string, args ...string) ([]byte, error) {
		return nil, os.ErrPermission
	}
	if ghExtensionInstalled("gh-copilot") {
		t.Error("ghExtensionInstalled returned true on command error")
	}
}
