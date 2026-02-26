package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitWrapperScript_ContainsBlockedCommands(t *testing.T) {
	script := GitWrapperScript("/usr/bin/git")
	for _, cmd := range []string{"push)", "fetch)", "clone)", "pull)", "remote)", "ls-remote)"} {
		if !strings.Contains(script, cmd) {
			t.Errorf("expected blocked command %q in git wrapper", cmd)
		}
	}
}

func TestGitWrapperScript_AllowsCommit(t *testing.T) {
	script := GitWrapperScript("/usr/bin/git")
	// "commit" should NOT be in the blocked list — it should fall through to the default exec
	if strings.Contains(script, `    commit)`) {
		t.Error("commit should not be blocked")
	}
	// Default case should exec real git
	if !strings.Contains(script, "exec '/usr/bin/git' \"$@\"") {
		t.Error("expected default case to exec real git")
	}
}

func TestGitWrapperScript_BlocksGlobalConfig(t *testing.T) {
	script := GitWrapperScript("/usr/bin/git")
	if !strings.Contains(script, "--global|--system") {
		t.Error("expected --global|--system check in config branch")
	}
}

func TestGitWrapperScript_ShebangFirstLine(t *testing.T) {
	script := GitWrapperScript("/usr/bin/git")
	if !strings.HasPrefix(script, "#!/bin/sh\n") {
		t.Error("expected #!/bin/sh as first line")
	}
}

func TestWriteGitWrapper_FileIsExecutable(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteGitWrapper(dir, "/usr/bin/git")
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
	if filepath.Base(path) != "git" {
		t.Errorf("expected 'git', got %s", filepath.Base(path))
	}
}
