package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrapperScript_ContainsSocat(t *testing.T) {
	script := WrapperScript("/tmp/proxy.sock", "/usr/bin/claude", nil)
	if !strings.Contains(script, "socat TCP-LISTEN:3128") {
		t.Error("expected socat TCP-LISTEN:3128 in wrapper script")
	}
	if !strings.Contains(script, "UNIX-CONNECT:/tmp/proxy.sock") {
		t.Error("expected UNIX-CONNECT:/tmp/proxy.sock in wrapper script")
	}
}

func TestWrapperScript_ContainsExec(t *testing.T) {
	script := WrapperScript("/tmp/proxy.sock", "/usr/bin/claude", nil)
	if !strings.Contains(script, "exec '/usr/bin/claude'") {
		t.Error("expected exec '/usr/bin/claude' in wrapper script")
	}
}

func TestWrapperScript_ShebangFirstLine(t *testing.T) {
	script := WrapperScript("/tmp/proxy.sock", "/usr/bin/claude", nil)
	if !strings.HasPrefix(script, "#!/bin/sh\n") {
		t.Error("expected #!/bin/sh as first line")
	}
}

func TestWrapperScript_ProviderArgs(t *testing.T) {
	script := WrapperScript("/tmp/proxy.sock", "/usr/bin/claude", []string{"--help", "--verbose"})
	if !strings.Contains(script, "'--help'") {
		t.Error("expected '--help' in wrapper script")
	}
	if !strings.Contains(script, "'--verbose'") {
		t.Error("expected '--verbose' in wrapper script")
	}
}

func TestWriteWrapperScript_FileIsExecutable(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteWrapperScript(dir, "/tmp/proxy.sock", "/usr/bin/claude", nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("expected executable permissions, got %v", info.Mode().Perm())
	}
	if filepath.Base(path) != "wrapper.sh" {
		t.Errorf("expected wrapper.sh, got %s", filepath.Base(path))
	}
}

func TestShellescape_SingleQuote(t *testing.T) {
	result := shellescape("it's a test")
	expected := "'it'\"'\"'s a test'"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
