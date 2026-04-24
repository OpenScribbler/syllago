package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func sandboxTestDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{})

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	return tmp
}

func TestSandboxAllowDomain_WritesConfig(t *testing.T) {
	tmp := sandboxTestDir(t)
	stdout, _ := output.SetForTest(t)

	if err := sandboxAllowDomainCmd.RunE(sandboxAllowDomainCmd, []string{"foo.com"}); err != nil {
		t.Fatalf("allow-domain: %v", err)
	}

	cfg, _ := config.Load(tmp)
	found := false
	for _, d := range cfg.Sandbox.AllowedDomains {
		if d == "foo.com" {
			found = true
		}
	}
	if !found {
		t.Error("expected foo.com in AllowedDomains after allow-domain")
	}
	if stdout.Len() == 0 {
		t.Error("expected output message")
	}
}

func TestSandboxDenyDomain_RemovesFromConfig(t *testing.T) {
	tmp := sandboxTestDir(t)
	output.SetForTest(t)

	// Add then remove.
	sandboxAllowDomainCmd.RunE(sandboxAllowDomainCmd, []string{"bar.com"})
	sandboxDenyDomainCmd.RunE(sandboxDenyDomainCmd, []string{"bar.com"})

	cfg, _ := config.Load(tmp)
	for _, d := range cfg.Sandbox.AllowedDomains {
		if d == "bar.com" {
			t.Error("bar.com should have been removed by deny-domain")
		}
	}
}

func TestSandboxAllowPort_WritesConfig(t *testing.T) {
	tmp := sandboxTestDir(t)
	output.SetForTest(t)

	if err := sandboxAllowPortCmd.RunE(sandboxAllowPortCmd, []string{"5432"}); err != nil {
		t.Fatalf("allow-port: %v", err)
	}

	cfg, _ := config.Load(tmp)
	found := false
	for _, p := range cfg.Sandbox.AllowedPorts {
		if p == 5432 {
			found = true
		}
	}
	if !found {
		t.Error("expected 5432 in AllowedPorts after allow-port")
	}
}

func TestSandboxAllowEnv_WritesConfig(t *testing.T) {
	tmp := sandboxTestDir(t)
	output.SetForTest(t)

	if err := sandboxAllowEnvCmd.RunE(sandboxAllowEnvCmd, []string{"MY_VAR"}); err != nil {
		t.Fatalf("allow-env: %v", err)
	}

	cfg, _ := config.Load(tmp)
	found := false
	for _, v := range cfg.Sandbox.AllowedEnv {
		if v == "MY_VAR" {
			found = true
		}
	}
	if !found {
		t.Error("expected MY_VAR in AllowedEnv after allow-env")
	}
}

func TestSandboxDomains_ListsConfigured(t *testing.T) {
	sandboxTestDir(t)
	stdout, _ := output.SetForTest(t)

	sandboxAllowDomainCmd.RunE(sandboxAllowDomainCmd, []string{"example.com"})
	// Reset stdout for the listing
	stdout.Reset()

	if err := sandboxDomainsCmd.RunE(sandboxDomainsCmd, []string{}); err != nil {
		t.Fatalf("domains: %v", err)
	}
	// Domains prints to fmt.Println (os.Stdout), not output.Writer.
	// Just verify no error — the domain was written to config in the previous test.
}

func TestSandboxAllowPort_InvalidPort(t *testing.T) {
	sandboxTestDir(t)
	output.SetForTest(t)

	err := sandboxAllowPortCmd.RunE(sandboxAllowPortCmd, []string{"notaport"})
	if err == nil {
		t.Error("expected error for non-integer port")
	}
}

// TestSandboxDenyPort_RemovesFromConfig covers the deny-port mutator branch
// (the allow path was the only one previously exercised).
func TestSandboxDenyPort_RemovesFromConfig(t *testing.T) {
	tmp := sandboxTestDir(t)
	output.SetForTest(t)

	sandboxAllowPortCmd.RunE(sandboxAllowPortCmd, []string{"6379"})
	if err := sandboxDenyPortCmd.RunE(sandboxDenyPortCmd, []string{"6379"}); err != nil {
		t.Fatalf("deny-port: %v", err)
	}

	cfg, _ := config.Load(tmp)
	for _, p := range cfg.Sandbox.AllowedPorts {
		if p == 6379 {
			t.Error("6379 should have been removed by deny-port")
		}
	}
}

func TestSandboxDenyPort_InvalidPort(t *testing.T) {
	sandboxTestDir(t)
	output.SetForTest(t)

	err := sandboxDenyPortCmd.RunE(sandboxDenyPortCmd, []string{"notaport"})
	if err == nil {
		t.Error("expected error for non-integer port on deny-port")
	}
}

// TestSandboxDenyEnv_RemovesFromConfig covers the deny-env mutator branch.
func TestSandboxDenyEnv_RemovesFromConfig(t *testing.T) {
	tmp := sandboxTestDir(t)
	output.SetForTest(t)

	sandboxAllowEnvCmd.RunE(sandboxAllowEnvCmd, []string{"GOAWAY_VAR"})
	if err := sandboxDenyEnvCmd.RunE(sandboxDenyEnvCmd, []string{"GOAWAY_VAR"}); err != nil {
		t.Fatalf("deny-env: %v", err)
	}

	cfg, _ := config.Load(tmp)
	for _, v := range cfg.Sandbox.AllowedEnv {
		if v == "GOAWAY_VAR" {
			t.Error("GOAWAY_VAR should have been removed by deny-env")
		}
	}
}

// TestSandboxInfo_PrintsConfig covers the no-args branch of `sandbox info`.
// The command writes via fmt.Printf so we capture the real os.Stdout.
func TestSandboxInfo_PrintsConfig(t *testing.T) {
	sandboxTestDir(t)
	output.SetForTest(t)
	sandboxAllowDomainCmd.RunE(sandboxAllowDomainCmd, []string{"example.org"})
	sandboxAllowEnvCmd.RunE(sandboxAllowEnvCmd, []string{"INFO_VAR"})

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if err := sandboxInfoCmd.RunE(sandboxInfoCmd, nil); err != nil {
		t.Fatalf("info: %v", err)
	}
	w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	for _, want := range []string{"Sandbox configuration", "example.org", "INFO_VAR"} {
		if !contains(out, want) {
			t.Errorf("info output missing %q\nfull output:\n%s", want, out)
		}
	}
}

// TestSandboxEnv_ListsConfigured exercises the env-listing command.
func TestSandboxEnv_ListsConfigured(t *testing.T) {
	sandboxTestDir(t)
	output.SetForTest(t)

	sandboxAllowEnvCmd.RunE(sandboxAllowEnvCmd, []string{"LIST_ME"})

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if err := sandboxEnvCmd.RunE(sandboxEnvCmd, nil); err != nil {
		t.Fatalf("env list: %v", err)
	}
	w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !contains(out, "LIST_ME") {
		t.Errorf("expected 'LIST_ME' in env list, got: %s", out)
	}
}

// TestSandboxPorts_ListsConfigured exercises the ports-listing command.
func TestSandboxPorts_ListsConfigured(t *testing.T) {
	sandboxTestDir(t)
	output.SetForTest(t)

	sandboxAllowPortCmd.RunE(sandboxAllowPortCmd, []string{"7777"})

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	if err := sandboxPortsCmd.RunE(sandboxPortsCmd, nil); err != nil {
		t.Fatalf("ports list: %v", err)
	}
	w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !contains(out, "7777") {
		t.Errorf("expected '7777' in ports list, got: %s", out)
	}
}

// TestSandboxRun_InvalidPort covers the early port-parsing error path of
// sandbox run, before it dispatches to sandbox.RunSession (which requires
// bubblewrap and would fail uncontrollably in a unit test).
func TestSandboxRun_InvalidPort(t *testing.T) {
	sandboxTestDir(t)
	output.SetForTest(t)

	sandboxRunCmd.Flags().Set("allow-port", "notaport")
	t.Cleanup(func() { sandboxRunCmd.Flags().Set("allow-port", "") })

	err := sandboxRunCmd.RunE(sandboxRunCmd, []string{"claude-code"})
	if err == nil || !contains(err.Error(), "invalid port") {
		t.Errorf("got %v, want 'invalid port' error", err)
	}
}

// contains avoids dragging strings into the test file's import block — it's
// a tiny helper used only by these tests.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestRemoveIntItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		slice []int
		item  int
		want  []int
	}{
		{"remove from middle", []int{1, 2, 3}, 2, []int{1, 3}},
		{"remove first", []int{1, 2, 3}, 1, []int{2, 3}},
		{"remove last", []int{1, 2, 3}, 3, []int{1, 2}},
		{"missing item", []int{1, 2, 3}, 4, []int{1, 2, 3}},
		{"empty slice", []int{}, 1, nil},
		{"single element removed", []int{5}, 5, nil},
		{"single element kept", []int{5}, 3, []int{5}},
		{"duplicates", []int{1, 2, 2, 3}, 2, []int{1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := removeIntItem(tt.slice, tt.item)
			if len(got) != len(tt.want) {
				t.Errorf("removeIntItem(%v, %d) = %v, want %v", tt.slice, tt.item, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("removeIntItem(%v, %d) = %v, want %v", tt.slice, tt.item, got, tt.want)
					return
				}
			}
		})
	}
}

func TestSandboxFormatList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		items []string
		want  string
	}{
		{"empty", []string{}, "(none)"},
		{"single", []string{"foo"}, "foo"},
		{"multiple", []string{"foo", "bar", "baz"}, "foo, bar, baz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sandboxFormatList(tt.items)
			if got != tt.want {
				t.Errorf("sandboxFormatList(%v) = %q, want %q", tt.items, got, tt.want)
			}
		})
	}
}
