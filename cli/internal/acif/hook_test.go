package acif

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	tvHookG = "9c8ab2d7f2465728140264d725011daad97054aceaaedfa9dd0a03d68d06b629"

	tvPlatformQBase = "194225ce5a7ec641253e47b6abd7f07b22fdf8ac4845cba159bef8ffe7f8783f"
	tvPlatformQFlip = "f5c964cdeddb304e801431f21c2c6c29f0e1e51cf99c63af78d2be58057c7dfc"

	tvPlatformQ2 = "bdee51e032051d9fc88d39f9196e47b4742d51fe4673cb020ba82c47903bee3a"

	tvPlatformI = "11f1e91480d2fdfd247311cc52240bf0fb2293febeccaeeadafacd74707cb32d"
)

func hookBlock(t *testing.T, raw string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var block map[string]any
	if err := dec.Decode(&block); err != nil {
		t.Fatalf("decode hook block: %v", err)
	}
	return block
}

func hookCanonicalBytes(t *testing.T, block map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal canonical block: %v", err)
	}
	canonical, err := CanonicalJSON(raw)
	if err != nil {
		t.Fatalf("canonical JSON: %v", err)
	}
	return string(canonical)
}

func hookHash(t *testing.T, block map[string]any, bodyRoot string) string {
	t.Helper()
	hash, computed, err := HookBodyHash(block, bodyRoot)
	if err != nil {
		t.Fatalf("HookBodyHash() error: %v", err)
	}
	if !computed {
		t.Fatalf("HookBodyHash() computed = false, want true")
	}
	return hash
}

func expectHookReject(t *testing.T, err error, id string) {
	t.Helper()
	var reject *RejectError
	if !errors.As(err, &reject) {
		t.Fatalf("error = %T %v, want *RejectError %s", err, err, id)
	}
	if reject.ID != id {
		t.Fatalf("RejectError.ID = %q, want %q", reject.ID, id)
	}
}

func writeHookFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

func TestHookPinnedBodyHashes(t *testing.T) {
	t.Run("TV-HOOK-g inline only", func(t *testing.T) {
		result, err := CanonicalizeHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"type":"command","scripts":[{"type":"inline","content":"#!/bin/sh\r\nexit 0\r\n"}]}]
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeHook() error: %v", err)
		}
		if result.Verdict != nil {
			t.Fatalf("Verdict = %#v, want nil", result.Verdict)
		}
		if got := hookHash(t, result.Canonical, ""); got != tvHookG {
			t.Fatalf("body_hash = %s, want %s", got, tvHookG)
		}
		wantBytes := `{"blocking":false,"event":"before_tool_execute","handlers":[{"async":false,"scripts":[{"content":"#!/bin/sh\nexit 0\n","type":"inline"}],"type":"command"}]}`
		if got := hookCanonicalBytes(t, result.Canonical); got != wantBytes {
			t.Fatalf("canonical_bytes = %s, want %s", got, wantBytes)
		}
	})

	t.Run("TV-PLATFORM-q file os flip changes hash", func(t *testing.T) {
		dir := t.TempDir()
		writeHookFiles(t, dir, map[string]string{"hooks/run.sh": "#!/bin/sh\necho ok\n"})

		base, err := CanonicalizeHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"type":"command","scripts":[{"type":"file","path":"hooks/run.sh","os":["linux"]}]}]
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeHook(base) error: %v", err)
		}
		if got := hookHash(t, base.Canonical, dir); got != tvPlatformQBase {
			t.Fatalf("base body_hash = %s, want %s", got, tvPlatformQBase)
		}

		flip, err := CanonicalizeHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"type":"command","scripts":[{"type":"file","path":"hooks/run.sh","os":["windows"]}]}]
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeHook(flip) error: %v", err)
		}
		if got := hookHash(t, flip.Canonical, dir); got != tvPlatformQFlip {
			t.Fatalf("flip body_hash = %s, want %s", got, tvPlatformQFlip)
		}
	})

	t.Run("TV-PLATFORM-q2 deterministic inline and file mix", func(t *testing.T) {
		dir := t.TempDir()
		writeHookFiles(t, dir, map[string]string{"hooks/win.cmd": "@echo off\r\necho hi\r\n"})

		a, err := CanonicalizeHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"type":"command","scripts":[
				{"type":"inline","content":"#!/bin/sh\necho hi\n","os":["darwin","linux"]},
				{"type":"file","path":"hooks/win.cmd","os":["windows"]}
			]}]
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeHook(a) error: %v", err)
		}
		b, err := CanonicalizeHook(hookBlock(t, `{
			"handlers":[{"scripts":[
				{"os":["windows"],"path":"hooks/win.cmd","type":"file"},
				{"os":["linux","darwin"],"content":"#!/bin/sh\r\necho hi\r\n","type":"inline"}
			]}],
			"event":"before_tool_execute"
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeHook(b) error: %v", err)
		}

		if gotA, gotB := hookCanonicalBytes(t, a.Canonical), hookCanonicalBytes(t, b.Canonical); gotA != gotB {
			t.Fatalf("canonical_bytes differ:\nA %s\nB %s", gotA, gotB)
		}
		if got := hookHash(t, a.Canonical, dir); got != tvPlatformQ2 {
			t.Fatalf("body_hash(a) = %s, want %s", got, tvPlatformQ2)
		}
		if got := hookHash(t, b.Canonical, dir); got != tvPlatformQ2 {
			t.Fatalf("body_hash(b) = %s, want %s", got, tvPlatformQ2)
		}
	})

	t.Run("TV-PLATFORM-i per-os-key-map", func(t *testing.T) {
		dir := t.TempDir()
		writeHookFiles(t, dir, map[string]string{
			"hooks/base.sh": "#!/bin/sh\necho base\n",
			"hooks/win.cmd": "@echo off\r\necho win\r\n",
			"hooks/lin.sh":  "#!/bin/sh\necho lin\n",
			"hooks/mac.sh":  "#!/bin/sh\necho mac\n",
		})
		result, err := CanonicalizeProviderConfig(hookBlock(t, `{
			"provider":"per-os-key-map",
			"path":"unused.json",
			"content":{"command":"hooks/base.sh","windows":"hooks/win.cmd","linux":"hooks/lin.sh","osx":"hooks/mac.sh"}
		}`), HookOpts{BodyRoot: dir})
		if err != nil {
			t.Fatalf("CanonicalizeProviderConfig() error: %v", err)
		}
		if result.Provenance != "declared" {
			t.Fatalf("Provenance = %q, want declared", result.Provenance)
		}
		if got := hookHash(t, result.Canonical, dir); got != tvPlatformI {
			t.Fatalf("body_hash = %s, want %s", got, tvPlatformI)
		}
		wantScripts := []map[string]any{
			{"type": "file", "path": "hooks/mac.sh", "os": []any{"darwin"}},
			{"type": "file", "path": "hooks/lin.sh", "os": []any{"linux"}},
			{"type": "file", "path": "hooks/win.cmd", "os": []any{"windows"}},
			{"type": "file", "path": "hooks/base.sh"},
		}
		gotScripts := result.Canonical["handlers"].([]any)[0].(map[string]any)["scripts"]
		if !reflect.DeepEqual(gotScripts, anySliceFromMaps(wantScripts)) {
			t.Fatalf("scripts = %#v, want %#v", gotScripts, wantScripts)
		}
	})
}

func TestHookRejectsAndVerdicts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		id   string
	}{
		{"handlers absent", `{"event":"before_tool_execute"}`, ErrHookHandlersMissing},
		{"handlers empty", `{"event":"before_tool_execute","handlers":[]}`, ErrHookHandlersMissing},
		{"handlers wrong type", `{"event":"before_tool_execute","handlers":{}}`, ErrHookHandlersMissing},
		{"bad handler type", `{"event":"before_tool_execute","handlers":[{"type":"webhook2"}]}`, ErrHookHandlerTypeUnrecognized},
		{"os empty", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"hooks/run.sh","os":[]}]}]}`, ErrHookScriptOSEmpty},
		{"arch empty", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"hooks/run.sh","arch":[]}]}]}`, ErrHookScriptArchEmpty},
		{"os freebsd", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"hooks/run.sh","os":["linux","freebsd"]}]}]}`, ErrHookScriptOSInvalid},
		{"os case", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"hooks/run.sh","os":["Linux"]}]}]}`, ErrHookScriptOSInvalid},
		{"two defaults", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"hooks/a.sh"},{"type":"file","path":"hooks/b.sh"}]}]}`, ErrHookScriptDefaultAmbiguous},
		{"absolute path", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"/etc/hooks/run.sh"}]}]}`, ErrHookScriptPathInvalid},
		{"parent path", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"../outside/run.sh"}]}]}`, ErrHookScriptPathInvalid},
		{"backslash path", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"a\\b"}]}]}`, ErrHookScriptPathInvalid},
		{"windows drive path", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"C:x"}]}]}`, ErrHookScriptPathInvalid},
		{"dot segment path", `{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"a/./b"}]}]}`, ErrHookScriptPathInvalid},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := CanonicalizeHook(hookBlock(t, tt.raw), HookOpts{})
			expectHookReject(t, err, tt.id)
		})
	}

	t.Run("platform ambiguity diagnostic uses input order", func(t *testing.T) {
		_, err := CanonicalizeHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[
				{"type":"file","path":"hooks/unix.sh","os":["darwin","linux"]},
				{"type":"file","path":"hooks/linux.sh","os":["linux"]}
			]}]
		}`), HookOpts{})
		var reject *RejectError
		if !errors.As(err, &reject) {
			t.Fatalf("error = %T %v, want RejectError", err, err)
		}
		if reject.ID != ErrHookScriptPlatformAmbiguous {
			t.Fatalf("RejectError.ID = %q, want %q", reject.ID, ErrHookScriptPlatformAmbiguous)
		}
		if len(reject.Diagnostics) != 1 {
			t.Fatalf("diagnostics = %#v, want one", reject.Diagnostics)
		}
		want := Diagnostic{ID: ErrHookScriptPlatformAmbiguous, Params: map[string]any{"os": "linux", "entries": []any{0, 1}}}
		if !reflect.DeepEqual(reject.Diagnostics[0], want) {
			t.Fatalf("diagnostic = %#v, want %#v", reject.Diagnostics[0], want)
		}
	})

	t.Run("missing referenced file", func(t *testing.T) {
		result, err := CanonicalizeHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[{"type":"file","path":"hooks/missing.sh"}]}]
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeHook() error: %v", err)
		}
		_, _, err = HookBodyHash(result.Canonical, t.TempDir())
		expectHookReject(t, err, ErrHookScriptFileMissing)
	})

	t.Run("requires empty omitted and non-empty verdict", func(t *testing.T) {
		empty, err := CanonicalizeHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"requires":{},
			"handlers":[{"scripts":[{"type":"inline","content":"x"}]}]
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeHook(empty requires) error: %v", err)
		}
		if _, ok := empty.Canonical["requires"]; ok {
			t.Fatalf("requires present in canonical: %#v", empty.Canonical)
		}
		nonEmpty, err := CanonicalizeHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"requires":{"handler_types":["command"]},
			"handlers":[{"scripts":[{"type":"inline","content":"x"}]}]
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeHook(non-empty requires) error: %v", err)
		}
		if nonEmpty.Verdict == nil || nonEmpty.Verdict.Reason != ReasonRequiresOrphanKey {
			t.Fatalf("Verdict = %#v, want %s", nonEmpty.Verdict, ReasonRequiresOrphanKey)
		}
	})
}

func TestHookHandlerDefaultsAndEventTranslation(t *testing.T) {
	explicit, err := CanonicalizeHook(hookBlock(t, `{
		"event":"before_tool_execute",
		"handlers":[{"type":"command","scripts":[{"type":"inline","content":"#!/bin/sh\nexit 0\n"}]}]
	}`), HookOpts{})
	if err != nil {
		t.Fatalf("explicit CanonicalizeHook() error: %v", err)
	}
	implicit, err := CanonicalizeHook(hookBlock(t, `{
		"event":"before_tool_execute",
		"handlers":[{"scripts":[{"type":"inline","content":"#!/bin/sh\nexit 0\n"}]}]
	}`), HookOpts{})
	if err != nil {
		t.Fatalf("implicit CanonicalizeHook() error: %v", err)
	}
	if got, want := hookHash(t, implicit.Canonical, ""), hookHash(t, explicit.Canonical, ""); got != want {
		t.Fatalf("implicit hash = %s, explicit hash = %s", got, want)
	}
	handler := implicit.Canonical["handlers"].([]any)[0].(map[string]any)
	if handler["type"] != "command" || handler["async"] != false {
		t.Fatalf("handler defaults = %#v", handler)
	}

	for _, tc := range []struct {
		provider string
		event    string
	}{
		{"claude-code", "PreToolUse"},
		{"gemini-cli", "BeforeTool"},
		{"opencode", "tool.execute.before"},
		{"unknown-provider", "PreToolUse"},
		{"", "before_tool_execute"},
	} {
		tc := tc
		t.Run(tc.provider+"/"+tc.event, func(t *testing.T) {
			result, err := CanonicalizeHook(hookBlock(t, `{
				"event":`+mustJSONString(t, tc.event)+`,
				"handlers":[{"scripts":[{"type":"inline","content":"#!/bin/sh\nexit 0\n"}]}]
			}`), HookOpts{Provider: tc.provider})
			if err != nil {
				t.Fatalf("CanonicalizeHook() error: %v", err)
			}
			if result.Canonical["event"] != "before_tool_execute" {
				t.Fatalf("event = %#v, want before_tool_execute", result.Canonical["event"])
			}
			if got := hookHash(t, result.Canonical, ""); got != tvHookG {
				t.Fatalf("body_hash = %s, want %s", got, tvHookG)
			}
		})
	}

	_, err = CanonicalizeHook(hookBlock(t, `{
		"event":"onToolStart",
		"handlers":[{"scripts":[{"type":"inline","content":"x"}]}]
	}`), HookOpts{})
	expectHookReject(t, err, ErrHookEventUnrecognized)
}

func TestHookProjection(t *testing.T) {
	t.Run("script selection with none diagnostic", func(t *testing.T) {
		block := hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[
				{"type":"file","path":"hooks/unix.sh","os":["darwin","linux"]}
			]}]
		}`)
		selection, diags, err := ScriptSelection(block, []string{"darwin", "linux", "windows"})
		if err != nil {
			t.Fatalf("ScriptSelection() error: %v", err)
		}
		want := map[string]string{"darwin": "hooks/unix.sh", "linux": "hooks/unix.sh", "windows": "none"}
		if !reflect.DeepEqual(selection, want) {
			t.Fatalf("selection = %#v, want %#v", selection, want)
		}
		if len(diags) != 1 || diags[0].ID != DiagHookScriptNoPlatformMatch {
			t.Fatalf("diagnostics = %#v, want no-platform-match", diags)
		}
	})

	t.Run("derived capabilities", func(t *testing.T) {
		caps, err := DerivedCapabilities(hookBlock(t, `{
			"event":"before_tool_execute",
			"matcher":"shell",
			"handlers":[{"async":true,"scripts":[{"type":"inline","content":"x"}]}]
		}`))
		if err != nil {
			t.Fatalf("DerivedCapabilities() error: %v", err)
		}
		want := map[string]bool{"handler_types": true, "matcher_patterns": true, "async_execution": true}
		if !reflect.DeepEqual(caps, want) {
			t.Fatalf("caps = %#v, want %#v", caps, want)
		}
	})

	t.Run("os coverage divergence cases", func(t *testing.T) {
		portable, err := OSCoverage(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[{"type":"file","path":"hooks/run.sh","os":["darwin","linux","windows"]}]}]
		}`))
		if err != nil {
			t.Fatalf("OSCoverage(portable) error: %v", err)
		}
		if portable["os_divergent"] != false {
			t.Fatalf("portable os_divergent = %#v, want false", portable["os_divergent"])
		}

		perOS, err := OSCoverage(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[
				{"type":"file","path":"hooks/mac.sh","os":["darwin"]},
				{"type":"file","path":"hooks/lin.sh","os":["linux"]},
				{"type":"file","path":"hooks/win.cmd","os":["windows"]}
			]}]
		}`))
		if err != nil {
			t.Fatalf("OSCoverage(perOS) error: %v", err)
		}
		if perOS["os_divergent"] != true {
			t.Fatalf("perOS os_divergent = %#v, want true", perOS["os_divergent"])
		}

		decoy, err := OSCoverage(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[
				{"type":"file","path":"hooks/unix.sh"},
				{"type":"file","path":"hooks/win.cmd","os":["windows"]}
			]}]
		}`))
		if err != nil {
			t.Fatalf("OSCoverage(decoy) error: %v", err)
		}
		if decoy["os_divergent"] != true || decoy["unconstrained"] != true {
			t.Fatalf("decoy coverage = %#v, want divergent unconstrained", decoy)
		}
	})
}

func TestHookMechanisms(t *testing.T) {
	t.Run("dual shell fields", func(t *testing.T) {
		result, err := CanonicalizeProviderConfig(hookBlock(t, `{
			"provider":"dual-shell-fields",
			"path":"unused.json",
			"content":{"bash":"hooks/unix.sh","powershell":"hooks/win.ps1"}
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeProviderConfig() error: %v", err)
		}
		if result.Provenance != "inferred-from-convention" {
			t.Fatalf("Provenance = %q", result.Provenance)
		}
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].ID != DiagHookPlatformShellOSProxy {
			t.Fatalf("diagnostics = %#v, want shell os proxy", result.Diagnostics)
		}
	})

	t.Run("filename extension convention table", func(t *testing.T) {
		cases := []struct {
			file       string
			wantOS     []any
			wantDiag   string
			provenance string
		}{
			{"hooks/run.ps1", []any{"windows"}, DiagHookPlatformFilenameInferred, "inferred-from-convention"},
			{"hooks/run.cmd", []any{"windows"}, DiagHookPlatformFilenameInferred, "inferred-from-convention"},
			{"hooks/run.bat", []any{"windows"}, DiagHookPlatformFilenameInferred, "inferred-from-convention"},
			{"hooks/run.sh", []any{"darwin", "linux"}, DiagHookPlatformFilenameInferred, "inferred-from-convention"},
			{"hooks/run", []any{"darwin", "linux"}, DiagHookPlatformFilenameInferred, "inferred-from-convention"},
			{"hooks/run.py", nil, DiagHookPlatformFilenameUninferable, "declared"},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.file, func(t *testing.T) {
				result, err := CanonicalizeProviderConfig(hookBlock(t, `{
					"provider":"filename-extension-convention",
					"path":"unused.json",
					"content":{"file":`+mustJSONString(t, tc.file)+`}
				}`), HookOpts{})
				if err != nil {
					t.Fatalf("CanonicalizeProviderConfig() error: %v", err)
				}
				script := result.Canonical["handlers"].([]any)[0].(map[string]any)["scripts"].([]any)[0].(map[string]any)
				if tc.wantOS == nil {
					if _, ok := script["os"]; ok {
						t.Fatalf("os present = %#v, want absent", script["os"])
					}
				} else if !reflect.DeepEqual(script["os"], tc.wantOS) {
					t.Fatalf("os = %#v, want %#v", script["os"], tc.wantOS)
				}
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].ID != tc.wantDiag {
					t.Fatalf("diagnostics = %#v, want %s", result.Diagnostics, tc.wantDiag)
				}
				if result.Provenance != tc.provenance {
					t.Fatalf("provenance = %q, want %q", result.Provenance, tc.provenance)
				}
			})
		}
	})

	t.Run("interpreter flag passthrough", func(t *testing.T) {
		result, err := CanonicalizeProviderConfig(hookBlock(t, `{
			"provider":"per-os-key-map",
			"path":"unused.json",
			"content":{"command":"hooks/run","shell":"pwsh -NoProfile -Command \"Write-Host hi\""}
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeProviderConfig() error: %v", err)
		}
		script := result.Canonical["handlers"].([]any)[0].(map[string]any)["scripts"].([]any)[0].(map[string]any)
		if script["shell"] != `pwsh -NoProfile -Command "Write-Host hi"` {
			t.Fatalf("shell passthrough = %#v", script["shell"])
		}
		if _, ok := script["os"]; ok {
			t.Fatalf("os synthesized unexpectedly: %#v", script)
		}
	})

	t.Run("unknown table key without command rejects", func(t *testing.T) {
		_, err := CanonicalizeProviderConfig(hookBlock(t, `{
			"provider":"per-os-key-map",
			"path":"unused.json",
			"content":{"shell":"bash"}
		}`), HookOpts{})
		expectHookReject(t, err, ErrHookPlatformUnmappable)
	})

	t.Run("identity merge", func(t *testing.T) {
		result, err := CanonicalizeProviderConfig(hookBlock(t, `{
			"provider":"per-os-key-map",
			"path":"unused.json",
			"content":{"command":"base.sh","linux":"unix.sh","osx":"unix.sh","windows":"win.cmd"}
		}`), HookOpts{})
		if err != nil {
			t.Fatalf("CanonicalizeProviderConfig() error: %v", err)
		}
		scripts := result.Canonical["handlers"].([]any)[0].(map[string]any)["scripts"].([]any)
		if len(scripts) != 3 {
			t.Fatalf("scripts length = %d, want 3: %#v", len(scripts), scripts)
		}
		for _, s := range scripts {
			script := s.(map[string]any)
			if script["path"] == "unix.sh" && !reflect.DeepEqual(script["os"], []any{"darwin", "linux"}) {
				t.Fatalf("unix.sh os = %#v, want darwin/linux", script["os"])
			}
		}
	})
}

func TestHookRender(t *testing.T) {
	t.Run("default degraded render drops overrides", func(t *testing.T) {
		out, err := RenderHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[
				{"type":"file","path":"hooks/unix.sh","os":["darwin","linux"]},
				{"type":"file","path":"hooks/base.sh"}
			]}]
		}`), "no-mechanism-provider", nil)
		if err != nil {
			t.Fatalf("RenderHook() error: %v", err)
		}
		if !strings.Contains(out.Output, `"command":"hooks/base.sh"`) {
			t.Fatalf("output = %s, want base command", out.Output)
		}
		if strings.Contains(out.Output, "hooks/unix.sh") {
			t.Fatalf("output contains dropped override path: %s", out.Output)
		}
		if len(out.Diagnostics) != 1 || out.Diagnostics[0].ID != DiagHookPlatformOverrideDropped {
			t.Fatalf("diagnostics = %#v, want override dropped", out.Diagnostics)
		}
	})

	t.Run("target os selection required without default", func(t *testing.T) {
		block := hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[{"type":"file","path":"hooks/unix.sh","os":["darwin","linux"]}]}]
		}`)
		out, err := RenderHook(block, "no-mechanism-provider", map[string]any{"target_os": "linux"})
		if err != nil {
			t.Fatalf("RenderHook(linux) error: %v", err)
		}
		if !strings.Contains(out.Output, `"command":"hooks/unix.sh"`) {
			t.Fatalf("output = %s, want unix command", out.Output)
		}
		_, err = RenderHook(block, "no-mechanism-provider", nil)
		expectHookReject(t, err, ErrHookNoDefaultForDegradedRender)
		_, err = RenderHook(block, "no-mechanism-provider", map[string]any{"target_os": "windows"})
		expectHookReject(t, err, ErrHookNoDefaultForDegradedRender)
	})

	t.Run("render back preserves shell string field", func(t *testing.T) {
		out, err := RenderHook(hookBlock(t, `{
			"event":"before_tool_execute",
			"handlers":[{"scripts":[{"type":"file","path":"hooks/run","shell":"pwsh -NoProfile -Command \"Write-Host hi\""}]}]
		}`), "no-mechanism-provider", nil)
		if err != nil {
			t.Fatalf("RenderHook() error: %v", err)
		}
		var rendered map[string]any
		if err := json.Unmarshal([]byte(out.Output), &rendered); err != nil {
			t.Fatalf("rendered output not JSON: %v", err)
		}
		if rendered["shell"] != `pwsh -NoProfile -Command "Write-Host hi"` {
			t.Fatalf("shell = %#v", rendered["shell"])
		}
	})
}

func TestHookInstallAndRequires(t *testing.T) {
	t.Run("evaluate install", func(t *testing.T) {
		item := hookBlock(t, `{
			"event":"before_tool_execute",
			"blocking":true,
			"handlers":[{"scripts":[{"type":"file","path":"hooks/unix.sh","os":["darwin","linux"]}]}]
		}`)
		refuse, err := EvaluateInstall(item, "windows")
		if err != nil {
			t.Fatalf("EvaluateInstall(windows) error: %v", err)
		}
		if refuse["install"] != "refuse-unless-operator-opt-in" {
			t.Fatalf("windows install = %#v", refuse)
		}
		proceed, err := EvaluateInstall(item, "linux")
		if err != nil {
			t.Fatalf("EvaluateInstall(linux) error: %v", err)
		}
		if proceed["install"] != "proceed" {
			t.Fatalf("linux install = %#v", proceed)
		}
	})

	t.Run("evaluate requires", func(t *testing.T) {
		unknown := EvaluateRequires(map[string]any{"handler_types": []any{"command"}}, []string{"matcher_patterns"})
		if unknown["evaluation"] != "unknown" || unknown["install"] != "refuse-unless-operator-opt-in" {
			t.Fatalf("unknown requires = %#v", unknown)
		}
		satisfied := EvaluateRequires(map[string]any{"handler_types": []any{"command"}}, []string{"handler_types"})
		if satisfied["evaluation"] != "satisfied" || satisfied["install"] != "proceed" {
			t.Fatalf("satisfied requires = %#v", satisfied)
		}
	})
}

func anySliceFromMaps(in []map[string]any) []any {
	out := make([]any, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}

func mustJSONString(t *testing.T, s string) string {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal string: %v", err)
	}
	return string(data)
}
