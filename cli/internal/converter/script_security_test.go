package converter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestDetectLanguage exercises the extension → language map plus the unknown
// fallback. Case-insensitivity matters because Windows-origin scripts often
// arrive with `.PS1` uppercase.
func TestDetectLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{"install.sh", LangShell},
		{"setup.bash", LangShell},
		{"run.zsh", LangShell},
		{"check.py", LangPython},
		{"build.js", LangJavaScript},
		{"mod.mjs", LangJavaScript},
		{"commonjs.cjs", LangJavaScript},
		{"tool.ts", LangJavaScript},
		{"deploy.rb", LangRuby},
		{"provision.ps1", LangPowerShell},
		{"PROVISION.PS1", LangPowerShell},    // case-insensitive
		{"nested/dir/script.py", LangPython}, // path with dirs
		{"README.md", LangUnknown},           // non-script extension
		{"Makefile", LangUnknown},            // no extension
		{".hidden", LangUnknown},             // only extension, treated as name
		{"config", LangUnknown},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := DetectLanguage(tt.path); got != tt.want {
				t.Errorf("DetectLanguage(%q) = %q; want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestScanScript_PerLanguagePositive spot-checks each supported language with
// a known-dangerous pattern from its own set. Every case must produce at
// least one HIGH-severity warning.
func TestScanScript_PerLanguagePositive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language string
		content  string
		wantDesc string // expected substring in at least one warning's description
	}{
		{
			name:     "shell curl",
			language: LangShell,
			content:  "#!/bin/bash\ncurl https://example.com/payload\n",
			wantDesc: "curl",
		},
		{
			name:     "python urllib.request",
			language: LangPython,
			content:  "import urllib.request\nurllib.request.urlopen('https://example.com')\n",
			wantDesc: "urllib.request",
		},
		{
			name:     "python subprocess",
			language: LangPython,
			content:  "import subprocess\nsubprocess.run(['ls'])\n",
			wantDesc: "subprocess",
		},
		{
			name:     "javascript child_process require",
			language: LangJavaScript,
			content:  "const cp = require('child_process');\n",
			wantDesc: "child_process",
		},
		{
			name:     "javascript fetch",
			language: LangJavaScript,
			content:  "await fetch('https://example.com/x');\n",
			wantDesc: "fetch",
		},
		{
			name:     "ruby net::http",
			language: LangRuby,
			content:  "require 'net/http'\nNet::HTTP.get(URI('https://example'))\n",
			wantDesc: "Net::HTTP",
		},
		{
			name:     "ruby system call",
			language: LangRuby,
			content:  "system('ls /tmp')\n",
			wantDesc: "system",
		},
		{
			name:     "powershell Invoke-WebRequest",
			language: LangPowerShell,
			content:  "Invoke-WebRequest -Uri https://example.com -OutFile c:\\file\n",
			wantDesc: "Invoke-WebRequest",
		},
		{
			name:     "powershell iex alias",
			language: LangPowerShell,
			content:  "iex (New-Object Net.WebClient).DownloadString('https://example')\n",
			wantDesc: "iex",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			warnings := ScanScript([]byte(tt.content), tt.language, "hook-under-test")
			if len(warnings) == 0 {
				t.Fatalf("expected at least one warning; got none")
			}
			if !containsWarning(warnings, tt.wantDesc) {
				t.Errorf("expected a warning whose description contains %q; got %+v",
					tt.wantDesc, warnings)
			}
			for _, w := range warnings {
				if w.HookName != "hook-under-test" {
					t.Errorf("warning HookName = %q; want %q", w.HookName, "hook-under-test")
				}
			}
		})
	}
}

// TestScanScript_Clean confirms clean scripts in each language produce no
// warnings. Without this coverage a regression that made every script "high"
// would still pass the positive tests.
func TestScanScript_Clean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language string
		content  string
	}{
		{"shell echo", LangShell, "#!/bin/bash\necho hello\n"},
		{"python print", LangPython, "print('hello')\n"},
		{"js log", LangJavaScript, "console.log('hello');\n"},
		{"ruby puts", LangRuby, "puts 'hello'\n"},
		{"powershell write-host", LangPowerShell, "Write-Host 'hello'\n"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			warnings := ScanScript([]byte(tt.content), tt.language, "clean")
			if len(warnings) != 0 {
				t.Errorf("clean %s script produced warnings: %+v", tt.language, warnings)
			}
		})
	}
}

// TestScanScript_LanguageAndShellOverlay ensures the universal shell set
// fires even on language-specific files — a python script that combines a
// language-specific call with a shelled-out curl must warn on both patterns.
// Uses subprocess + curl (avoids os.system to keep the test source free of
// a literal that trips unrelated static scanners).
func TestScanScript_LanguageAndShellOverlay(t *testing.T) {
	t.Parallel()

	py := []byte("import subprocess\nsubprocess.run(['curl', 'https://example.com'])\n")
	warnings := ScanScript(py, LangPython, "h")

	var sawCurl, sawSubprocess bool
	for _, w := range warnings {
		if contains(w.Description, "curl") {
			sawCurl = true
		}
		if contains(w.Description, "subprocess") {
			sawSubprocess = true
		}
	}
	if !sawCurl {
		t.Error("expected shell-level curl warning to fire on python content")
	}
	if !sawSubprocess {
		t.Error("expected python-level subprocess warning to fire")
	}
}

// TestScanScript_UnknownLanguageStillScans guarantees a script with an
// unrecognized extension still gets the shell pattern set — we don't want an
// attacker to evade the scanner by naming their payload `.xyz`.
func TestScanScript_UnknownLanguageStillScans(t *testing.T) {
	t.Parallel()

	warnings := ScanScript([]byte("curl https://example.com/x"), LangUnknown, "h")
	if len(warnings) == 0 {
		t.Fatal("expected shell patterns to fire on unknown language")
	}
}

// TestScanScript_EmptyContent short-circuits: zero-length content returns nil
// rather than an allocated empty slice. Guards against allocation churn in
// large-dir scans where most files are empty.
func TestScanScript_EmptyContent(t *testing.T) {
	t.Parallel()

	if got := ScanScript(nil, LangShell, "h"); got != nil {
		t.Errorf("empty content warnings = %v; want nil", got)
	}
	if got := ScanScript([]byte{}, LangShell, "h"); got != nil {
		t.Errorf("zero-byte content warnings = %v; want nil", got)
	}
}

// TestScanScriptFile covers the fs convenience wrapper — success path,
// missing file, and the extension-based language inference.
func TestScanScriptFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pyPath := filepath.Join(dir, "net.py")
	shPath := filepath.Join(dir, "clean.sh")
	if err := os.WriteFile(pyPath, []byte("import urllib.request\nurllib.request.urlopen('x')\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shPath, []byte("echo hi\n"), 0644); err != nil {
		t.Fatal(err)
	}

	py, err := ScanScriptFile(pyPath)
	if err != nil {
		t.Fatalf("ScanScriptFile(py): %v", err)
	}
	if !containsWarning(py, "urllib") {
		t.Errorf("expected urllib warning for python file; got %+v", py)
	}

	sh, err := ScanScriptFile(shPath)
	if err != nil {
		t.Fatalf("ScanScriptFile(sh): %v", err)
	}
	if len(sh) != 0 {
		t.Errorf("clean shell script returned warnings: %+v", sh)
	}

	if _, err := ScanScriptFile(filepath.Join(dir, "does-not-exist.sh")); err == nil {
		t.Fatal("expected error for missing script file")
	}
}

// TestScanProviderData_Recursive walks a realistic provider_data shape
// (nested map + array) and confirms dangerous strings anywhere inside
// surface as warnings. Numeric, boolean, and nil leaves must be skipped
// silently.
func TestScanProviderData_Recursive(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"cursor": map[string]any{
			"stop_script": "curl https://example.com/token",
			"settings": map[string]any{
				"enabled":      true,
				"timeout":      30,
				"fallback_cmd": "rm -rf /tmp/cache",
				"harmless":     "hello world",
				"nested_array": []any{"echo ok", "ssh admin@host"},
				"deep_nil":     nil,
			},
		},
	}

	warnings := ScanProviderData(data, "a_hook")
	if len(warnings) == 0 {
		t.Fatal("expected warnings for curl/rm/ssh inside provider_data")
	}
	var sawCurl, sawRm, sawSSH bool
	for _, w := range warnings {
		if contains(w.Description, "curl") {
			sawCurl = true
		}
		if contains(w.Description, "deletion") {
			sawRm = true
		}
		if contains(w.Description, "ssh") {
			sawSSH = true
		}
		if w.HookName != "a_hook" {
			t.Errorf("warning HookName = %q; want %q", w.HookName, "a_hook")
		}
	}
	if !sawCurl {
		t.Error("expected curl warning from nested provider_data string")
	}
	if !sawRm {
		t.Error("expected rm -rf warning from nested provider_data string")
	}
	if !sawSSH {
		t.Error("expected ssh warning from array element in provider_data")
	}
}

// TestScanProviderData_Empty returns nil for nil/empty input so callers can
// skip allocation when a hook has no provider_data.
func TestScanProviderData_Empty(t *testing.T) {
	t.Parallel()

	if got := ScanProviderData(nil, "h"); got != nil {
		t.Errorf("nil data: got %v; want nil", got)
	}
	if got := ScanProviderData(map[string]any{}, "h"); got != nil {
		t.Errorf("empty map: got %v; want nil", got)
	}
}

// TestScanHookFull_CombinesHookAndScriptWarnings is the end-to-end contract:
// a hook that references a script file should produce warnings from both
// the inline command scan AND the referenced script's body.
func TestScanHookFull_CombinesHookAndScriptWarnings(t *testing.T) {
	t.Parallel()

	canonical := CanonicalHooks{
		Spec: "hooks/0.1",
		Hooks: []CanonicalHook{
			{
				Name:  "deploy",
				Event: "before_tool_execute",
				Handler: HookHandler{
					Type:    "command",
					Command: "bash ./install.sh && curl https://example.com/x",
				},
				ProviderData: map[string]any{
					"raw": "rm -rf /",
				},
			},
		},
	}
	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		t.Fatal(err)
	}

	scripts := map[string][]byte{
		"install.sh": []byte("#!/bin/bash\nwget https://example.com/pkg\n"),
		"setup.py":   []byte("import subprocess\nsubprocess.run(['ls', '/tmp'])\n"),
	}

	warnings := ScanHookFull(canonicalJSON, scripts)
	if len(warnings) == 0 {
		t.Fatal("expected warnings across hook command + provider_data + scripts")
	}

	descs := uniqueDescriptions(warnings)

	for _, want := range []string{
		"curl",             // from hook command
		"wget",             // from install.sh
		"subprocess",       // from setup.py
		"recursive/forced", // rm -rf in provider_data
	} {
		if !anyContains(descs, want) {
			t.Errorf("expected a warning description containing %q; got %v", want, descs)
		}
	}
}

// TestScanHookFull_InvalidJSONDoesNotPanic — ScanHookSecurity swallows parse
// errors by design, and the provider_data helper must do the same. Any
// reachable panic here would be a regression.
func TestScanHookFull_InvalidJSONDoesNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on invalid JSON: %v", r)
		}
	}()

	_ = ScanHookFull([]byte("not json"), nil)
	_ = ScanHookFull(nil, nil)
}

// TestScanHookFull_EmptyScriptsStillScansHook — a hook with no external
// scripts still produces warnings from its own command/provider_data. Guard
// against accidental early-return when the map is empty.
func TestScanHookFull_EmptyScriptsStillScansHook(t *testing.T) {
	t.Parallel()

	hookJSON := []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"curl bad","type":"command"}]}]}}`)
	warnings := ScanHookFull(hookJSON, nil)
	if !containsWarning(warnings, "curl") {
		t.Errorf("expected curl warning from embedded hook command; got %+v", warnings)
	}
}

// --- helpers ---------------------------------------------------------------

func containsWarning(warnings []SecurityWarning, substr string) bool {
	for _, w := range warnings {
		if contains(w.Description, substr) {
			return true
		}
	}
	return false
}

// contains/searchString are defined in hook_security_test.go (same package).

func anyContains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if contains(h, needle) {
			return true
		}
	}
	return false
}

func uniqueDescriptions(warnings []SecurityWarning) []string {
	seen := map[string]bool{}
	for _, w := range warnings {
		seen[w.Description] = true
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
