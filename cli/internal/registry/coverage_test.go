package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- probeGitHub / probeGitLab / probeBitbucket via httpClient redirect ---
//
// The existing tests use helper functions that replicate probe logic without
// calling the real functions. These tests override httpClient's Transport so
// the real probe functions' HTTP requests go to a local httptest server.

type redirectTransport struct {
	target string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect the request to our test server
	targetHost := strings.TrimPrefix(t.target, "http://")
	req.URL.Scheme = "http"
	req.URL.Host = targetHost
	return http.DefaultTransport.RoundTrip(req)
}

func withRedirectClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := httpClient
	httpClient = &http.Client{Transport: &redirectTransport{target: srv.URL}}
	t.Cleanup(func() { httpClient = orig })
}

func TestProbeGitHub_Real_Public(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"private": false})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitHub("test-owner", "test-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("probeGitHub() = %q, want %q", vis, VisibilityPublic)
	}
}

func TestProbeGitHub_Real_Private(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitHub("test-owner", "private-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("probeGitHub() = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestProbeGitHub_Real_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitHub("test-owner", "forbidden-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("probeGitHub() = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestProbeGitHub_Real_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitHub("test-owner", "error-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityUnknown {
		t.Errorf("probeGitHub() = %q, want %q", vis, VisibilityUnknown)
	}
}

func TestProbeGitHub_Real_PrivateTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"private": true})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitHub("test-owner", "private-true")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("probeGitHub() = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestProbeGitLab_Real_Public(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"visibility": "public"})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitLab("test-owner", "test-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("probeGitLab() = %q, want %q", vis, VisibilityPublic)
	}
}

func TestProbeGitLab_Real_Private(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitLab("test-owner", "private-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("probeGitLab() = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestProbeGitLab_Real_Internal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"visibility": "internal"})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitLab("test-owner", "internal-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("probeGitLab() = %q, want %q (internal → private)", vis, VisibilityPrivate)
	}
}

func TestProbeGitLab_Real_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeGitLab("test-owner", "error-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityUnknown {
		t.Errorf("probeGitLab() = %q, want %q", vis, VisibilityUnknown)
	}
}

func TestProbeBitbucket_Real_Public(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"is_private": false})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeBitbucket("test-owner", "test-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("probeBitbucket() = %q, want %q", vis, VisibilityPublic)
	}
}

func TestProbeBitbucket_Real_Private(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"is_private": true})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeBitbucket("test-owner", "private-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("probeBitbucket() = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestProbeBitbucket_Real_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeBitbucket("test-owner", "forbidden-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("probeBitbucket() = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestProbeBitbucket_Real_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	vis, err := probeBitbucket("test-owner", "error-repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityUnknown {
		t.Errorf("probeBitbucket() = %q, want %q", vis, VisibilityUnknown)
	}
}

// --- ProbeVisibility routing through real probe functions ---

func TestProbeVisibility_GitHub_Route(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"private": false})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	// Make sure OverrideProbeForTest is nil so real routing happens
	orig := OverrideProbeForTest
	OverrideProbeForTest = nil
	t.Cleanup(func() { OverrideProbeForTest = orig })

	vis, err := ProbeVisibility("https://github.com/acme/tools")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("ProbeVisibility() = %q, want %q", vis, VisibilityPublic)
	}
}

func TestProbeVisibility_GitLab_Route(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"visibility": "public"})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	orig := OverrideProbeForTest
	OverrideProbeForTest = nil
	t.Cleanup(func() { OverrideProbeForTest = orig })

	vis, err := ProbeVisibility("https://gitlab.com/org/project")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("ProbeVisibility() = %q, want %q", vis, VisibilityPublic)
	}
}

func TestProbeVisibility_Bitbucket_Route(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"is_private": false})
	}))
	defer srv.Close()
	withRedirectClient(t, srv)

	orig := OverrideProbeForTest
	OverrideProbeForTest = nil
	t.Cleanup(func() { OverrideProbeForTest = orig })

	vis, err := ProbeVisibility("https://bitbucket.org/team/repo")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("ProbeVisibility() = %q, want %q", vis, VisibilityPublic)
	}
}

// --- CacheDir ---

func TestCacheDir_Default(t *testing.T) {
	orig := CacheDirOverride
	CacheDirOverride = ""
	t.Cleanup(func() { CacheDirOverride = orig })

	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, filepath.Join(".syllago", "registries")) {
		t.Errorf("CacheDir() = %q, want suffix .syllago/registries", dir)
	}
}

// --- SyncAll ---

func TestSyncAll_WithClonedRepos(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Setup: create a bare repo and clone it
	tmp := t.TempDir()
	orig := CacheDirOverride
	CacheDirOverride = filepath.Join(tmp, "cache")
	t.Cleanup(func() { CacheDirOverride = orig })

	bareDir := filepath.Join(tmp, "bare.git")
	workDir := filepath.Join(tmp, "work")

	// Create bare repo
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	os.MkdirAll(workDir, 0755)
	os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# Test"), 0644)

	run("init", workDir)
	run("-C", workDir, "config", "user.email", "test@test.com")
	run("-C", workDir, "config", "user.name", "Test")
	run("-C", workDir, "add", "-A")
	run("-C", workDir, "commit", "-m", "init")
	run("clone", "--bare", workDir, bareDir)

	// Clone into cache
	if err := Clone(bareDir, "test-reg", ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// SyncAll should succeed
	results := SyncAll([]string{"test-reg"})
	if len(results) != 1 {
		t.Fatalf("SyncAll returned %d results, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("SyncAll[0].Err = %v, want nil", results[0].Err)
	}
	if results[0].Name != "test-reg" {
		t.Errorf("SyncAll[0].Name = %q, want %q", results[0].Name, "test-reg")
	}
}

func TestSyncAll_Empty(t *testing.T) {
	results := SyncAll(nil)
	if len(results) != 0 {
		t.Errorf("SyncAll(nil) returned %d results, want 0", len(results))
	}
}

// --- ExtractHooksToDir with script copying ---

func TestExtractHooksToDir_WithScript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create an in-repo script that the hook references
	scriptDir := filepath.Join(dir, "scripts")
	os.MkdirAll(scriptDir, 0755)
	scriptPath := filepath.Join(scriptDir, "lint.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho lint"), 0755)

	targetDir := filepath.Join(dir, "output")

	hooks := []UserScopedHook{
		{
			Name:         "posttooluse-lint",
			Event:        "PostToolUse",
			Definition:   json.RawMessage(`{"matcher": "Edit", "hooks": [{"type": "command", "command": "./scripts/lint.sh"}]}`),
			Command:      "./scripts/lint.sh",
			ScriptPath:   scriptPath,
			ScriptInRepo: true,
		},
	}

	if err := ExtractHooksToDir(hooks, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify hook.json was created
	hookPath := filepath.Join(targetDir, "posttooluse-lint", "hook.json")
	if _, err := os.Stat(hookPath); err != nil {
		t.Fatalf("hook.json not created: %v", err)
	}

	// Verify script was copied
	copiedScript := filepath.Join(targetDir, "posttooluse-lint", "lint.sh")
	data, err := os.ReadFile(copiedScript)
	if err != nil {
		t.Fatalf("script not copied: %v", err)
	}
	if !strings.Contains(string(data), "echo lint") {
		t.Errorf("copied script content = %q, want to contain 'echo lint'", string(data))
	}
}

func TestExtractHooksToDir_MultipleHooks(t *testing.T) {
	t.Parallel()
	targetDir := filepath.Join(t.TempDir(), "output")

	hooks := []UserScopedHook{
		{
			Name:       "posttooluse-0",
			Event:      "PostToolUse",
			Definition: json.RawMessage(`{"hooks": [{"type": "command", "command": "echo a"}]}`),
		},
		{
			Name:       "pretooluse-bash",
			Event:      "PreToolUse",
			Definition: json.RawMessage(`{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo b"}]}`),
		},
	}

	if err := ExtractHooksToDir(hooks, targetDir); err != nil {
		t.Fatal(err)
	}

	// Both hook directories should exist
	for _, name := range []string{"posttooluse-0", "pretooluse-bash"} {
		hookPath := filepath.Join(targetDir, name, "hook.json")
		if _, err := os.Stat(hookPath); err != nil {
			t.Errorf("hook.json not created for %s: %v", name, err)
		}
	}
}

func TestExtractHooksToDir_Empty(t *testing.T) {
	t.Parallel()
	if err := ExtractHooksToDir(nil, t.TempDir()); err != nil {
		t.Errorf("ExtractHooksToDir(nil) = %v, want nil", err)
	}
}

// --- Remove ---

func TestRemove_NonexistentRegistry(t *testing.T) {
	tmp := t.TempDir()
	orig := CacheDirOverride
	CacheDirOverride = tmp
	t.Cleanup(func() { CacheDirOverride = orig })

	// Remove on a non-existent registry should succeed (RemoveAll is tolerant)
	if err := Remove("nonexistent"); err != nil {
		t.Errorf("Remove(nonexistent) = %v, want nil", err)
	}
}

func TestRemove_ExistingRegistry(t *testing.T) {
	tmp := t.TempDir()
	orig := CacheDirOverride
	CacheDirOverride = tmp
	t.Cleanup(func() { CacheDirOverride = orig })

	// Create a fake registry dir
	regDir := filepath.Join(tmp, "my-reg")
	os.MkdirAll(regDir, 0755)
	os.WriteFile(filepath.Join(regDir, "README.md"), []byte("test"), 0644)

	if err := Remove("my-reg"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(regDir); !os.IsNotExist(err) {
		t.Error("registry dir should be removed")
	}
}

// --- normalizeVisibility edge cases ---

func TestNormalizeVisibility(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"public", "public"},
		{"private", "private"},
		{"unknown", "unknown"},
		{"", "unknown"},
		{"garbage", "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeVisibility(tc.input)
			if got != tc.want {
				t.Errorf("normalizeVisibility(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// --- StricterOf with unrecognized values ---

func TestStricterOf_Unrecognized(t *testing.T) {
	t.Parallel()
	got := stricterOf("public", "garbage")
	if got != "unknown" {
		t.Errorf("stricterOf(public, garbage) = %q, want unknown", got)
	}
}
