package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractScriptRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
		want    string
	}{
		// Inline commands — no script
		{"empty", "", ""},
		{"echo", "echo hello", ""},
		{"env var", "MY_VAR=1 echo test", ""},
		{"pipe", "cat file.txt | grep foo", ""},

		// Direct absolute/tilde paths
		{"tilde path", "~/.claude/hooks/lint.sh", "~/.claude/hooks/lint.sh"},
		{"absolute path", "/home/user/.claude/hooks/lint.sh", "/home/user/.claude/hooks/lint.sh"},
		{"relative dot", "./scripts/lint.sh", "./scripts/lint.sh"},
		{"relative dotdot", "../scripts/check.sh", "../scripts/check.sh"},

		// With arguments
		{"tilde with args", "~/.claude/hooks/lint.sh --strict", "~/.claude/hooks/lint.sh"},
		{"absolute with args", "/opt/hooks/check.sh arg1 arg2", "/opt/hooks/check.sh"},

		// Interpreter prefix
		{"bash tilde", "bash ~/.claude/hooks/lint.sh", "~/.claude/hooks/lint.sh"},
		{"sh absolute", "sh /opt/hooks/check.sh", "/opt/hooks/check.sh"},
		{"node relative", "node ./checker.js", "./checker.js"},
		{"python tilde", "python ~/scripts/analyze.py", "~/scripts/analyze.py"},
		{"bash with args", "bash ~/.claude/hooks/lint.sh --mode strict", "~/.claude/hooks/lint.sh"},

		// Interpreter + subcommand (bun run, node exec)
		{"bun run absolute", "bun run /home/user/scripts/test.ts", "/home/user/scripts/test.ts"},
		{"bun run tilde", "bun run ~/scripts/test.ts", "~/scripts/test.ts"},
		{"bun run relative", "bun run ./test.ts", "./test.ts"},
		{"bun run with args", "bun run /scripts/test.ts --flag", "/scripts/test.ts"},
		{"bun run envvar", "bun run $PAI_DIR/hooks/foo.ts", ""},
		{"bun direct", "bun /home/user/scripts/test.ts", "/home/user/scripts/test.ts"},

		// Interpreter -c (inline) — no script
		{"bash -c", `bash -c "echo hello"`, ""},
		{"sh -c", `sh -c 'ls -la'`, ""},

		// Interpreter with flags before path
		{"bash flag then path", "bash -e ~/.claude/hooks/lint.sh", "~/.claude/hooks/lint.sh"},

		// No path-like token after interpreter
		{"bash alone", "bash", ""},
		{"node flags only", "node --version", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractScriptRef(tc.command)
			if got != tc.want {
				t.Errorf("ExtractScriptRef(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}

func TestBundleHookScripts_InlineCommandUnchanged(t *testing.T) {
	t.Parallel()
	destDir := t.TempDir()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{
			{Type: "command", Command: "echo lint"},
		},
	}

	bundled, err := BundleHookScripts(&hook, t.TempDir(), destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundled) != 0 {
		t.Errorf("expected no bundled scripts, got %d", len(bundled))
	}
	if hook.Hooks[0].Command != "echo lint" {
		t.Errorf("command was modified: %q", hook.Hooks[0].Command)
	}
}

func TestBundleHookScripts_AbsolutePath(t *testing.T) {
	t.Parallel()

	// Create a source script
	sourceDir := t.TempDir()
	scriptContent := "#!/bin/bash\necho lint"
	scriptPath := filepath.Join(sourceDir, "lint.sh")
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	destDir := t.TempDir()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{
			{Type: "command", Command: scriptPath + " --strict"},
		},
	}

	bundled, err := BundleHookScripts(&hook, sourceDir, destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundled) != 1 {
		t.Fatalf("expected 1 bundled script, got %d", len(bundled))
	}
	if bundled[0].OriginalPath != scriptPath {
		t.Errorf("original path = %q, want %q", bundled[0].OriginalPath, scriptPath)
	}
	if bundled[0].Filename != "lint.sh" {
		t.Errorf("filename = %q, want lint.sh", bundled[0].Filename)
	}

	// Command should be rewritten to relative
	if !strings.HasPrefix(hook.Hooks[0].Command, "./lint.sh") {
		t.Errorf("command not rewritten: %q", hook.Hooks[0].Command)
	}
	if !strings.HasSuffix(hook.Hooks[0].Command, " --strict") {
		t.Errorf("args lost: %q", hook.Hooks[0].Command)
	}

	// Script should exist in destDir
	copied, err := os.ReadFile(filepath.Join(destDir, "lint.sh"))
	if err != nil {
		t.Fatalf("script not copied: %v", err)
	}
	if string(copied) != scriptContent {
		t.Errorf("script content = %q, want %q", string(copied), scriptContent)
	}
}

func TestBundleHookScripts_RelativePath(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	scriptContent := "#!/bin/bash\ncheck"
	os.MkdirAll(filepath.Join(sourceDir, "scripts"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "scripts", "check.sh"), []byte(scriptContent), 0755)

	destDir := t.TempDir()
	hook := HookData{
		Event: "after_tool_execute",
		Hooks: []HookEntry{
			{Type: "command", Command: "./scripts/check.sh"},
		},
	}

	bundled, err := BundleHookScripts(&hook, sourceDir, destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundled) != 1 {
		t.Fatalf("expected 1 bundled, got %d", len(bundled))
	}
	if bundled[0].Filename != "check.sh" {
		t.Errorf("filename = %q, want check.sh", bundled[0].Filename)
	}

	// Command rewritten to flat relative
	if hook.Hooks[0].Command != "./check.sh" {
		t.Errorf("command = %q, want ./check.sh", hook.Hooks[0].Command)
	}
}

func TestBundleHookScripts_InterpreterPrefix(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	os.WriteFile(filepath.Join(sourceDir, "lint.sh"), []byte("#!/bin/bash"), 0755)

	destDir := t.TempDir()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{
			{Type: "command", Command: "bash " + filepath.Join(sourceDir, "lint.sh") + " --mode strict"},
		},
	}

	bundled, err := BundleHookScripts(&hook, sourceDir, destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundled) != 1 {
		t.Fatalf("expected 1 bundled, got %d", len(bundled))
	}

	// Should rewrite only the path, preserving "bash" and args
	if hook.Hooks[0].Command != "bash ./lint.sh --mode strict" {
		t.Errorf("command = %q, want 'bash ./lint.sh --mode strict'", hook.Hooks[0].Command)
	}
}

func TestBundleHookScripts_MissingScriptSkipped(t *testing.T) {
	t.Parallel()
	destDir := t.TempDir()
	hook := HookData{
		Event: "session_start",
		Hooks: []HookEntry{
			{Type: "command", Command: "/nonexistent/script.sh"},
		},
	}

	bundled, err := BundleHookScripts(&hook, t.TempDir(), destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundled) != 0 {
		t.Errorf("expected no bundled scripts for missing file, got %d", len(bundled))
	}
	// Command should be unchanged
	if hook.Hooks[0].Command != "/nonexistent/script.sh" {
		t.Errorf("command was modified: %q", hook.Hooks[0].Command)
	}
}

func TestBundleHookScripts_MultipleHooks(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	os.WriteFile(filepath.Join(sourceDir, "pre.sh"), []byte("pre"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "post.sh"), []byte("post"), 0755)

	destDir := t.TempDir()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{
			{Type: "command", Command: filepath.Join(sourceDir, "pre.sh")},
			{Type: "command", Command: "echo inline"},
			{Type: "command", Command: filepath.Join(sourceDir, "post.sh")},
		},
	}

	bundled, err := BundleHookScripts(&hook, sourceDir, destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundled) != 2 {
		t.Fatalf("expected 2 bundled scripts, got %d", len(bundled))
	}

	// First and third rewritten, second unchanged
	if hook.Hooks[0].Command != "./pre.sh" {
		t.Errorf("hooks[0] = %q, want ./pre.sh", hook.Hooks[0].Command)
	}
	if hook.Hooks[1].Command != "echo inline" {
		t.Errorf("hooks[1] = %q, want 'echo inline'", hook.Hooks[1].Command)
	}
	if hook.Hooks[2].Command != "./post.sh" {
		t.Errorf("hooks[2] = %q, want ./post.sh", hook.Hooks[2].Command)
	}
}

func TestBundleHookScripts_HTTPHookSkipped(t *testing.T) {
	t.Parallel()
	destDir := t.TempDir()
	hook := HookData{
		Event: "session_start",
		Hooks: []HookEntry{
			{Type: "http", URL: "https://example.com/webhook"},
		},
	}

	bundled, err := BundleHookScripts(&hook, t.TempDir(), destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundled) != 0 {
		t.Errorf("expected no bundled scripts for HTTP hook, got %d", len(bundled))
	}
}

func TestExpandTilde(t *testing.T) {
	t.Parallel()
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}
	for _, tc := range tests {
		got, err := expandTilde(tc.input)
		if err != nil {
			t.Errorf("expandTilde(%q): %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("expandTilde(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
