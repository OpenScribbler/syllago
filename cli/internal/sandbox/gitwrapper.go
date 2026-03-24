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

# --- (C) Block dangerous env vars that could escape the sandbox ---
unset GIT_SSH_COMMAND GIT_SSH GIT_EXEC_PATH GIT_PROXY_COMMAND GIT_ASKPASS GIT_TERMINAL_PROMPT
# GIT_PROTOCOL_HANDLER_* is a family; unset any that are set.
for __syllago_var in $(env | grep '^GIT_PROTOCOL_HANDLER_' | cut -d= -f1); do
  unset "$__syllago_var"
done
unset __syllago_var

# --- (B) Disable aliases and external object replacements ---
export GIT_CONFIG_NOSYSTEM=1
unset GIT_REPLACE_REF_BASE

# --- (A) Find the actual subcommand by skipping flags ---
# Git flags before the subcommand: -c key=val, -C path, --git-dir=...,
# --work-tree=..., --namespace=..., --bare, --no-replace-objects, etc.
SUBCMD=""
__syllago_skip_next=0
for __syllago_arg in "$@"; do
  if [ "$__syllago_skip_next" = 1 ]; then
    __syllago_skip_next=0
    continue
  fi
  case "$__syllago_arg" in
    # Flags that consume the next argument
    -c|-C|--git-dir|--work-tree|--namespace|--super-prefix)
      __syllago_skip_next=1
      continue
      ;;
    # Flags with = syntax or single-char flags
    --git-dir=*|--work-tree=*|--namespace=*|--super-prefix=*|--bare|--no-replace-objects|--literal-pathspecs|--glob-pathspecs|--noglob-pathspecs|--no-optional-locks|--exec-path|--exec-path=*|--paginate|--no-pager|-p|--html-path|--man-path|--info-path|--version)
      continue
      ;;
    # Anything starting with - is an unknown flag; skip it
    -*)
      continue
      ;;
    # First non-flag argument is the subcommand
    *)
      SUBCMD="$__syllago_arg"
      break
      ;;
  esac
done
unset __syllago_skip_next __syllago_arg

case "$SUBCMD" in
%s    config)
      # Block global config writes.
      for arg in "$@"; do
        case "$arg" in --global|--system) echo "[sandbox] git config --global/--system is blocked." >&2; exit 1 ;; esac
      done
      exec %s --no-replace-objects "$@"
      ;;
    *)
      exec %s --no-replace-objects "$@"
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
