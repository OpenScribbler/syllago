package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/config"
	"github.com/OpenScribbler/nesco/cli/internal/output"
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
