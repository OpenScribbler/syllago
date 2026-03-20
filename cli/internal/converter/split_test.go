package converter

import "testing"

func TestSplitSettingsHooks_ClaudeCode(t *testing.T) {
	input := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo check","statusMessage":"Checking..."}]}],"PostToolUse":[{"matcher":"Write|Edit","hooks":[{"type":"command","command":"echo lint"}]}]}}`
	items, err := SplitSettingsHooks([]byte(input), "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	events := map[string]bool{}
	for _, item := range items {
		events[item.Event] = true
	}
	if !events["PreToolUse"] {
		t.Error("expected PreToolUse event")
	}
	if !events["PostToolUse"] {
		t.Error("expected PostToolUse event")
	}
}

func TestSplitSettingsHooks_GeminiCLI(t *testing.T) {
	input := `{"hooks":{"BeforeTool":[{"matcher":"run_shell_command","hooks":[{"type":"command","command":"echo check"}]}]}}`
	items, err := SplitSettingsHooks([]byte(input), "gemini-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Event != "PreToolUse" {
		t.Errorf("expected event PreToolUse, got %q", items[0].Event)
	}
	if items[0].Matcher != "Bash" {
		t.Errorf("expected matcher Bash, got %q", items[0].Matcher)
	}
}

func TestSplitSettingsHooks_CopilotCLI(t *testing.T) {
	input := `{"hooks":{"preToolUse":[{"bash":"echo check","timeoutSec":5,"comment":"Safety"}]}}`
	items, err := SplitSettingsHooks([]byte(input), "copilot-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Event != "PreToolUse" {
		t.Errorf("expected event PreToolUse, got %q", items[0].Event)
	}
	if len(items[0].Hooks) != 1 {
		t.Fatalf("expected 1 hook entry, got %d", len(items[0].Hooks))
	}
	he := items[0].Hooks[0]
	if he.Type != "command" {
		t.Errorf("expected type command, got %q", he.Type)
	}
	if he.Command != "echo check" {
		t.Errorf("expected command 'echo check', got %q", he.Command)
	}
	if he.Timeout != 5000 {
		t.Errorf("expected timeout 5000ms, got %d", he.Timeout)
	}
	if he.StatusMessage != "Safety" {
		t.Errorf("expected statusMessage 'Safety', got %q", he.StatusMessage)
	}
}

func TestDeriveHookName_StatusMessage(t *testing.T) {
	hook := HookData{
		Event:   "PreToolUse",
		Matcher: "Bash",
		Hooks:   []HookEntry{{StatusMessage: "Validating shell command..."}},
	}
	got := DeriveHookName(hook)
	want := "validating-shell-command"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDeriveHookName_MatcherEvent(t *testing.T) {
	hook := HookData{
		Event:   "PreToolUse",
		Matcher: "Bash",
		Hooks:   []HookEntry{{Type: "command", Command: "go vet"}},
	}
	got := DeriveHookName(hook)
	want := "pretooluse-bash"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDeriveHookName_EventCommand(t *testing.T) {
	hook := HookData{
		Event: "SessionStart",
		Hooks: []HookEntry{{Command: "echo starting session"}},
	}
	got := DeriveHookName(hook)
	want := "sessionstart-echo"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Validating shell command...", "validating-shell-command"},
		{"Hello World!!!", "hello-world"},
		{"--leading-trailing--", "leading-trailing"},
		{"multiple   spaces", "multiple-spaces"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := slugify(tc.input)
			if got != tc.want {
				t.Errorf("slugify(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
