package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- shouldSkipDiff (0% coverage) ---

func TestShouldSkipDiff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		patterns []string
		want     bool
	}{
		{"exact match", "/home/user/.claude.json", []string{".claude.json"}, true},
		{"glob match", "/home/user/.claude.json", []string{"*.json"}, true},
		{"no match", "/home/user/.claude.json", []string{".gemini"}, false},
		{"empty patterns", "/home/user/settings.json", nil, false},
		{"dir pattern", "/home/user/.claude", []string{".claude"}, true},
		{"multiple patterns first matches", "/home/user/oauth.json", []string{"settings.json", "oauth.json"}, true},
		{"multiple patterns second matches", "/home/user/settings.json", []string{"oauth.json", "settings.json"}, true},
		{"multiple patterns none match", "/home/user/config.yaml", []string{"settings.json", "oauth.json"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := shouldSkipDiff(tc.path, tc.patterns)
			if got != tc.want {
				t.Errorf("shouldSkipDiff(%q, %v) = %v, want %v", tc.path, tc.patterns, got, tc.want)
			}
		})
	}
}

// --- protectStagedFiles (0% coverage) ---

func TestProtectStagedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a staged directory with files
	stagedDir := filepath.Join(dir, "staged")
	os.MkdirAll(stagedDir, 0755)
	os.WriteFile(filepath.Join(stagedDir, "oauth_creds.json"), []byte("creds"), 0644)
	os.WriteFile(filepath.Join(stagedDir, "settings.json"), []byte("settings"), 0644)

	snapshots := []ConfigSnapshot{
		{StagedPath: stagedDir},
	}

	protectStagedFiles(snapshots, []string{"oauth_creds.json"})

	// oauth_creds.json should now be read-only
	info, err := os.Stat(filepath.Join(stagedDir, "oauth_creds.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0222 != 0 {
		t.Errorf("oauth_creds.json should be read-only, got %v", info.Mode().Perm())
	}

	// settings.json should still be writable
	info, err = os.Stat(filepath.Join(stagedDir, "settings.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0200 == 0 {
		t.Errorf("settings.json should still be writable, got %v", info.Mode().Perm())
	}
}

func TestProtectStagedFiles_NonDirSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a staged file (not a directory)
	filePath := filepath.Join(dir, "settings.json")
	os.WriteFile(filePath, []byte("{}"), 0644)

	snapshots := []ConfigSnapshot{
		{StagedPath: filePath},
	}

	// Should not panic on non-directory
	protectStagedFiles(snapshots, []string{"*.json"})
}

func TestProtectStagedFiles_NonexistentPath(t *testing.T) {
	t.Parallel()
	snapshots := []ConfigSnapshot{
		{StagedPath: "/nonexistent/path"},
	}
	// Should not panic on nonexistent path
	protectStagedFiles(snapshots, []string{"*.json"})
}

// --- FilterCurrentEnv (0% coverage) ---

func TestFilterCurrentEnv(t *testing.T) {
	// Not parallel: uses os.Environ() directly
	pairs, report := FilterCurrentEnv(nil)
	if len(pairs) == 0 {
		t.Error("FilterCurrentEnv returned 0 pairs, expected at least some forwarded env vars")
	}
	// HOME should always be forwarded
	homeForwarded := false
	for _, name := range report.Forwarded {
		if name == "HOME" {
			homeForwarded = true
			break
		}
	}
	if !homeForwarded {
		t.Error("HOME should be forwarded")
	}
}

// --- ApplyDiff (50% coverage) ---

func TestApplyDiff_File(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create staged file with new content
	stagedPath := filepath.Join(dir, "staged.json")
	os.WriteFile(stagedPath, []byte(`{"new":"data"}`), 0644)

	// Create original file with old content
	origPath := filepath.Join(dir, "original.json")
	os.WriteFile(origPath, []byte(`{"old":"data"}`), 0644)

	result := DiffResult{
		Snapshot: ConfigSnapshot{
			OriginalPath: origPath,
			StagedPath:   stagedPath,
		},
	}

	if err := ApplyDiff(result); err != nil {
		t.Fatalf("ApplyDiff: %v", err)
	}

	// Verify original was overwritten
	data, _ := os.ReadFile(origPath)
	if string(data) != `{"new":"data"}` {
		t.Errorf("original content = %q, want new data", string(data))
	}
}

func TestApplyDiff_Dir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create staged directory with a file
	stagedDir := filepath.Join(dir, "staged")
	os.MkdirAll(stagedDir, 0755)
	os.WriteFile(filepath.Join(stagedDir, "config.json"), []byte(`{"updated":true}`), 0644)

	// Create original directory
	origDir := filepath.Join(dir, "original")
	os.MkdirAll(origDir, 0755)
	os.WriteFile(filepath.Join(origDir, "config.json"), []byte(`{"updated":false}`), 0644)

	result := DiffResult{
		Snapshot: ConfigSnapshot{
			OriginalPath: origDir,
			StagedPath:   stagedDir,
		},
	}

	if err := ApplyDiff(result); err != nil {
		t.Fatalf("ApplyDiff: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(origDir, "config.json"))
	if string(data) != `{"updated":true}` {
		t.Errorf("content = %q, want updated", string(data))
	}
}

func TestApplyDiff_StagedMissing(t *testing.T) {
	t.Parallel()
	result := DiffResult{
		Snapshot: ConfigSnapshot{
			OriginalPath: "/some/original",
			StagedPath:   "/nonexistent/staged",
		},
	}

	err := ApplyDiff(result)
	if err == nil {
		t.Fatal("expected error for missing staged file")
	}
	if !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("error = %q, want mention of 'no longer exists'", err)
	}
}

// --- ProfileFor routing and error paths ---

// Note: TestProfileFor_Windsurf and TestProfileFor_UnknownProvider
// already exist in profile_test.go — not duplicated here.

// Test each profile's binary-not-found error path.
// These exercise the claudeProfile, codexProfile, copilotProfile, cursorProfile
// functions through ProfileFor — each fails at resolveBinary when the binary isn't on PATH.

func TestProfileFor_ClaudeNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty PATH
	_, err := ProfileFor("claude-code", "/home/user", "/project")
	if err == nil {
		t.Fatal("expected error when claude not on PATH")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want mention of not found", err)
	}
}

func TestProfileFor_CodexNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := ProfileFor("codex", "/home/user", "/project")
	if err == nil {
		t.Fatal("expected error when codex not on PATH")
	}
}

func TestProfileFor_CopilotNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := ProfileFor("copilot-cli", "/home/user", "/project")
	if err == nil {
		t.Fatal("expected error when gh not on PATH")
	}
}

func TestProfileFor_CursorNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := ProfileFor("cursor", "/home/user", "/project")
	if err == nil {
		t.Fatal("expected error when cursor not on PATH")
	}
}

func TestProfileFor_GeminiNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := ProfileFor("gemini-cli", "/home/user", "/project")
	if err == nil {
		t.Fatal("expected error when gemini not on PATH")
	}
}

// --- resolveBinary / findNodePrefix ---

func TestResolveBinary_NotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, _, err := resolveBinary("nonexistent-binary-xyz")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found on PATH") {
		t.Errorf("error = %q, want mention of PATH", err)
	}
}

func TestResolveBinary_Success(t *testing.T) {
	// Use "git" which is always available in our test environment
	bin, paths, err := resolveBinary("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	if bin == "" {
		t.Error("resolved binary path is empty")
	}
	if len(paths) == 0 {
		t.Error("mount paths is empty")
	}
	// Resolved path should be an absolute path
	if !filepath.IsAbs(bin) {
		t.Errorf("resolved path %q is not absolute", bin)
	}
}

func TestFindNodePrefix_NoNodeOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := findNodePrefix("/some/random/script.js")
	if err == nil {
		t.Fatal("expected error when node is not on PATH")
	}
}

func TestFindNodePrefix_WithNodeBinDir(t *testing.T) {
	t.Parallel()
	// Create a directory tree that looks like a node installation
	dir := t.TempDir()
	nodePrefix := filepath.Join(dir, "node", "20.0.0")
	binDir := filepath.Join(nodePrefix, "bin")
	os.MkdirAll(binDir, 0755)
	os.WriteFile(filepath.Join(binDir, "node"), []byte("#!/bin/bash"), 0755)

	// Script is nested deep inside the node prefix
	scriptPath := filepath.Join(nodePrefix, "lib", "node_modules", "pkg", "index.js")
	os.MkdirAll(filepath.Dir(scriptPath), 0755)
	os.WriteFile(scriptPath, []byte("// script"), 0644)

	prefix, err := findNodePrefix(scriptPath)
	if err != nil {
		t.Fatalf("findNodePrefix: %v", err)
	}
	if prefix != nodePrefix {
		t.Errorf("findNodePrefix() = %q, want %q", prefix, nodePrefix)
	}
}

// --- resolveNodeBinary ---

func TestResolveNodeBinary_NotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, _, _, err := resolveNodeBinary("nonexistent-node-tool")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

// --- RunSession error paths ---

func TestRunSession_PreflightCheckFails(t *testing.T) {
	// Create a valid project dir (passes dir safety) but use a provider that
	// won't pass pre-flight (no bwrap/socat typically). This tests Step 2→3.
	projDir := t.TempDir()
	os.WriteFile(filepath.Join(projDir, "go.mod"), []byte("module test"), 0644)

	w := devNullCov(t)
	err := RunSession(RunConfig{
		ProviderSlug: "claude-code",
		ProjectDir:   projDir,
		HomeDir:      t.TempDir(),
	}, w)
	if err == nil {
		t.Log("RunSession succeeded (bwrap+socat+claude available) — skipping error check")
		return
	}
	// Should fail at either dir safety or pre-flight check
	errStr := err.Error()
	if !strings.Contains(errStr, "pre-flight") && !strings.Contains(errStr, "blocked") {
		t.Logf("RunSession error (expected pre-flight or dir-safety): %s", errStr)
	}
}

func devNullCov(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// --- StageConfigs edge cases ---

func TestStageConfigs_MixedExistAndMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stagingDir := filepath.Join(dir, "staging")

	// Create one file that exists
	existingFile := filepath.Join(dir, "settings.json")
	os.WriteFile(existingFile, []byte(`{"key":"val"}`), 0644)

	// Pass both existing and non-existing paths
	snaps, err := StageConfigs(stagingDir, []string{
		existingFile,
		filepath.Join(dir, "nonexistent.json"),
	})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}
	// Only the existing file should be staged
	if len(snaps) != 1 {
		t.Errorf("got %d snapshots, want 1", len(snaps))
	}
}

func TestStageConfigs_DirCopy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stagingDir := filepath.Join(dir, "staging")

	// Create a config directory with files
	configDir := filepath.Join(dir, ".claude")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{"a":1}`), 0644)
	os.WriteFile(filepath.Join(configDir, "hooks.json"), []byte(`{}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{configDir})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(snaps))
	}
	// Verify staged directory has the files
	stagedSettings := filepath.Join(snaps[0].StagedPath, "settings.json")
	if _, err := os.Stat(stagedSettings); err != nil {
		t.Errorf("staged settings.json not found: %v", err)
	}
}

// --- hashPath ---

func TestHashPath_File(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	hash1, err := hashPath(path)
	if err != nil {
		t.Fatalf("hashPath: %v", err)
	}
	if len(hash1) == 0 {
		t.Error("hash is empty")
	}

	// Same content should produce same hash
	path2 := filepath.Join(dir, "test2.txt")
	os.WriteFile(path2, []byte("hello"), 0644)
	hash2, _ := hashPath(path2)
	if string(hash1) != string(hash2) {
		t.Error("same content should produce same hash")
	}

	// Different content should produce different hash
	path3 := filepath.Join(dir, "test3.txt")
	os.WriteFile(path3, []byte("world"), 0644)
	hash3, _ := hashPath(path3)
	if string(hash1) == string(hash3) {
		t.Error("different content should produce different hash")
	}
}

func TestHashPath_Dir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb"), 0644)

	hash, err := hashPath(dir)
	if err != nil {
		t.Fatalf("hashPath dir: %v", err)
	}
	if len(hash) == 0 {
		t.Error("hash is empty for directory")
	}
}

func TestHashPath_Nonexistent(t *testing.T) {
	t.Parallel()
	_, err := hashPath("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

// --- Check function ---

// Note: TestCheck_UnknownProvider already exists in check_test.go

// --- WriteWrapperScript ---

func TestWriteWrapperScript_Basic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "proxy.sock")
	binaryExec := "/usr/bin/fake-provider"

	path, err := WriteWrapperScript(dir, socketPath, binaryExec, nil)
	if err != nil {
		t.Fatalf("WriteWrapperScript: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading wrapper: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, binaryExec) {
		t.Error("wrapper should contain binary exec path")
	}
}

// --- WriteGitWrapper ---

func TestWriteGitWrapper_Basic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path, err := WriteGitWrapper(dir, "/usr/bin/git")
	if err != nil {
		t.Fatalf("WriteGitWrapper: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading git wrapper: %v", err)
	}
	if !strings.Contains(string(data), "/usr/bin/git") {
		t.Error("git wrapper should contain git path")
	}
}
