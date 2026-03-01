package loadout

import (
	"path/filepath"
	"testing"
)

func TestResolveHookCommand_RelativePath(t *testing.T) {
	t.Parallel()
	result := ResolveHookCommand("/repo/content/hooks/claude-code/my-hook", "./script.sh")
	want := filepath.Join("/repo/content/hooks/claude-code/my-hook", "script.sh")
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestResolveHookCommand_ParentRelative(t *testing.T) {
	t.Parallel()
	result := ResolveHookCommand("/repo/content/hooks/claude-code/my-hook", "../shared/script.sh")
	want := filepath.Join("/repo/content/hooks/claude-code/my-hook", "../shared/script.sh")
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestResolveHookCommand_AbsolutePath(t *testing.T) {
	t.Parallel()
	result := ResolveHookCommand("/repo/content/hooks/claude-code/my-hook", "/usr/bin/lint")
	if result != "/usr/bin/lint" {
		t.Errorf("got %q, want /usr/bin/lint", result)
	}
}

func TestResolveHookCommand_InlineCommand(t *testing.T) {
	t.Parallel()
	result := ResolveHookCommand("/some/dir", "echo hello")
	if result != "echo hello" {
		t.Errorf("got %q, want 'echo hello'", result)
	}
}
