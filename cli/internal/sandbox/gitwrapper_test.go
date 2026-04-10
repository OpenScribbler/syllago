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
	if !strings.Contains(script, "exec '/usr/bin/git' --no-replace-objects \"$@\"") {
		t.Error("expected default case to exec real git with --no-replace-objects")
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

func TestGitWrapperScript_SubcommandParsing(t *testing.T) {
	script := GitWrapperScript("/usr/bin/git")
	// The wrapper should scan past flags to find the real subcommand.
	// Verify it contains the arg-scanning loop.
	if !strings.Contains(script, "__syllago_skip_next") {
		t.Error("expected subcommand-scanning loop in wrapper")
	}
	// Should not use $1 directly as SUBCMD
	if strings.Contains(script, `SUBCMD="${1:-}"`) {
		t.Error("wrapper should not use $1 directly as SUBCMD — must scan past flags")
	}
}

func TestGitWrapperScript_BlocksDangerousEnvVars(t *testing.T) {
	script := GitWrapperScript("/usr/bin/git")
	for _, env := range []string{
		"GIT_SSH_COMMAND", "GIT_SSH", "GIT_EXEC_PATH",
		"GIT_PROXY_COMMAND", "GIT_ASKPASS", "GIT_TERMINAL_PROMPT",
	} {
		if !strings.Contains(script, env) {
			t.Errorf("expected wrapper to unset %s", env)
		}
	}
	if !strings.Contains(script, "GIT_PROTOCOL_HANDLER_") {
		t.Error("expected wrapper to unset GIT_PROTOCOL_HANDLER_* family")
	}
}

func TestGitWrapperScript_DisablesAliasesAndReplaceObjects(t *testing.T) {
	script := GitWrapperScript("/usr/bin/git")
	if !strings.Contains(script, "GIT_CONFIG_NOSYSTEM=1") {
		t.Error("expected GIT_CONFIG_NOSYSTEM=1 to disable system config/aliases")
	}
	if !strings.Contains(script, "unset GIT_REPLACE_REF_BASE") {
		t.Error("expected unset GIT_REPLACE_REF_BASE")
	}
	if !strings.Contains(script, "--no-replace-objects") {
		t.Error("expected --no-replace-objects in exec calls")
	}
}

func TestGitWrapperScript_FlagBypassBlocked(t *testing.T) {
	// Verify the wrapper handles -c and -C flags that precede the subcommand.
	// The scanning loop must recognize these as flag-value pairs to skip.
	script := GitWrapperScript("/usr/bin/git")
	// -c and -C should be listed as flags that consume the next argument
	if !strings.Contains(script, "-c|-C|") {
		t.Error("expected -c and -C in skip-next-arg flag list")
	}
	// --git-dir should be handled both with = and as separate arg
	if !strings.Contains(script, "--git-dir=*") {
		t.Error("expected --git-dir=* pattern")
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
