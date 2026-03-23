package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
)

// blockedGitSubcommands is the list of git subcommands that are always blocked.
var blockedGitSubcommands = []string{
	"push", "fetch", "pull", "clone",
	"remote", "ls-remote", "submodule",
}

// GitWrapperScript returns the content of the git wrapper shell script.
// realGit is the path to the real git binary (e.g. /usr/bin/git).
func GitWrapperScript(realGit string) string {
	blocked := ""
	for _, cmd := range blockedGitSubcommands {
		blocked += fmt.Sprintf(`    %s)
      echo "[sandbox] git %s is blocked in the sandbox." >&2
      exit 1
      ;;
`, cmd, cmd)
	}

	return fmt.Sprintf(`#!/bin/sh
# Syllago sandbox git wrapper — blocks network operations.
SUBCMD="${1:-}"
case "$SUBCMD" in
%s    config)
      # Block global config writes.
      for arg in "$@"; do
        case "$arg" in --global|--system) echo "[sandbox] git config --global/--system is blocked." >&2; exit 1 ;; esac
      done
      exec %s "$@"
      ;;
    *)
      exec %s "$@"
      ;;
esac
`, blocked, shellescape(realGit), shellescape(realGit))
}

// WriteGitWrapper writes the git wrapper script to stagingDir/bin/git and makes it
// executable. The bin/ subdirectory is mounted into the sandbox and prepended to PATH
// so the wrapper shadows the real git binary.
// Returns the path to the written script.
func WriteGitWrapper(stagingDir, realGit string) (string, error) {
	content := GitWrapperScript(realGit)
	binDir := filepath.Join(stagingDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("creating bin dir: %w", err)
	}
	path := filepath.Join(binDir, "git")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return "", fmt.Errorf("writing git wrapper: %w", err)
	}
	return path, nil
}

// GitWrapperBinDir returns the bin/ directory containing the git wrapper,
// suitable for mounting into the sandbox and adding to PATH.
func GitWrapperBinDir(gitWrapperPath string) string {
	return filepath.Dir(gitWrapperPath)
}
