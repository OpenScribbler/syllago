package converter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// writeHookDir builds a minimal hook directory for BuiltinScanner testing.
// hookJSON is written to hook.json; scripts is a map of filename → body.
func writeHookDir(t *testing.T, hookJSON string, scripts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if hookJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644); err != nil {
			t.Fatalf("write hook.json: %v", err)
		}
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// TestBuiltinScanner_ManifestFindings verifies the builtin scanner surfaces
// findings from a legacy nested hook.json. File is "hook.json", Line is 0
// (we don't track JSON line numbers), Scanner tagged "builtin".
func TestBuiltinScanner_ManifestFindings(t *testing.T) {
	t.Parallel()

	dir := writeHookDir(t,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"curl bad","type":"command"}]}]}}`,
		nil)

	s := &BuiltinScanner{}
	res, err := s.Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Findings) == 0 {
		t.Fatalf("expected manifest findings, got none")
	}

	var found bool
	for _, f := range res.Findings {
		if f.File == "hook.json" && f.Scanner == "builtin" && contains(f.Description, "curl") {
			found = true
			if f.Line != 0 {
				t.Errorf("manifest finding should have Line=0, got %d", f.Line)
			}
			if f.Severity != "high" {
				t.Errorf("expected high severity for curl, got %q", f.Severity)
			}
		}
	}
	if !found {
		t.Errorf("no curl finding tagged as builtin/hook.json; got %+v", res.Findings)
	}
}

// TestBuiltinScanner_ScriptFindings verifies script files in the hook dir
// are scanned with file+line info. A wget in check.sh must surface with
// File="check.sh", Scanner="builtin", and the correct 1-indexed line.
func TestBuiltinScanner_ScriptFindings(t *testing.T) {
	t.Parallel()

	dir := writeHookDir(t, "",
		map[string]string{
			"check.sh": "#!/bin/bash\necho start\nwget https://example.com/x\necho done\n",
		})

	s := &BuiltinScanner{}
	res, err := s.Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var hit *ScanFinding
	for i, f := range res.Findings {
		if f.File == "check.sh" && contains(f.Description, "wget") {
			hit = &res.Findings[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("missing wget finding in check.sh; got %+v", res.Findings)
	}
	if hit.Line != 3 {
		t.Errorf("expected wget at line 3, got %d", hit.Line)
	}
	if hit.Scanner != "builtin" {
		t.Errorf("scanner tag = %q, want builtin", hit.Scanner)
	}
}

// TestBuiltinScanner_CleanHook — zero-finding case for coverage parity with
// the positive tests. An empty hook dir should return an empty result and
// no error.
func TestBuiltinScanner_CleanHook(t *testing.T) {
	t.Parallel()

	dir := writeHookDir(t,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"echo hi","type":"command"}]}]}}`,
		map[string]string{"clean.sh": "#!/bin/bash\necho hi\n"})

	s := &BuiltinScanner{}
	res, err := s.Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("clean hook produced findings: %+v", res.Findings)
	}
	if len(res.Errors) != 0 {
		t.Errorf("clean hook produced errors: %+v", res.Errors)
	}
}

// TestBuiltinScanner_MultipleScripts — ensure both .sh and .py files are
// scanned with their language-specific patterns. Each finding tagged with
// its own file.
func TestBuiltinScanner_MultipleScripts(t *testing.T) {
	t.Parallel()

	dir := writeHookDir(t, "",
		map[string]string{
			"a.sh": "#!/bin/bash\ncurl https://example\n",
			"b.py": "import subprocess\nsubprocess.run(['ls'])\n",
		})

	s := &BuiltinScanner{}
	res, err := s.Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	sawSh, sawPy := false, false
	for _, f := range res.Findings {
		if f.File == "a.sh" && contains(f.Description, "curl") {
			sawSh = true
		}
		if f.File == "b.py" && contains(f.Description, "subprocess") {
			sawPy = true
		}
	}
	if !sawSh {
		t.Error("missing .sh finding")
	}
	if !sawPy {
		t.Error("missing .py finding")
	}
}

// TestBuiltinScanner_BinaryFilesSkipped — files without a recognized script
// extension are not scanned. Ensures adding a README.md or a compiled binary
// to a hook dir doesn't cause the scanner to panic or emit false findings.
func TestBuiltinScanner_BinaryFilesSkipped(t *testing.T) {
	t.Parallel()

	dir := writeHookDir(t, "",
		map[string]string{
			"README.md": "run `curl https://example` manually\n", // would trip regex if scanned
			"binary":    string([]byte{0x7f, 'E', 'L', 'F', 0x02}),
		})

	s := &BuiltinScanner{}
	res, err := s.Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("expected no findings for non-script files, got %+v", res.Findings)
	}
}

// TestBuiltinScanner_NestedDirsIgnored — subdirectories in the hook dir are
// not walked. Supporting nested scripts is out of scope (the hook layout is
// flat), and recursing risks surprising users who stash unrelated files.
func TestBuiltinScanner_NestedDirsIgnored(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "evil.sh"), []byte("curl https://evil\n"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := (&BuiltinScanner{}).Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("expected no findings for nested dirs; got %+v", res.Findings)
	}
}

// --- ExternalScanner --------------------------------------------------------

// writeExternalScanner creates an executable shell script at dir/name that
// produces the given stdout and exits with the given code. Skips the test
// on non-Unix platforms where bash isn't guaranteed.
func writeExternalScanner(t *testing.T, name, body string, exitCode int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("external scanner tests require a POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	script := fmt.Sprintf("#!/bin/sh\ncat <<'EOF'\n%s\nEOF\nexit %d\n", body, exitCode)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write scanner: %v", err)
	}
	return path
}

// TestExternalScanner_ValidOutput — scanner prints valid ScanResult JSON,
// exits 0. Parsed findings are returned with Scanner field rewritten to the
// scanner's basename so downstream aggregation can attribute.
func TestExternalScanner_ValidOutput(t *testing.T) {
	t.Parallel()

	out := `{"findings":[{"severity":"high","file":"x.sh","line":4,"description":"sample finding"}],"errors":[]}`
	path := writeExternalScanner(t, "good", out, 0)

	s := &ExternalScanner{Path: path}
	res, err := s.Scan(t.TempDir())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(res.Findings))
	}
	if res.Findings[0].Scanner != "good" {
		t.Errorf("scanner tag = %q; want basename %q", res.Findings[0].Scanner, "good")
	}
	if res.Findings[0].Severity != "high" {
		t.Errorf("severity = %q; want high", res.Findings[0].Severity)
	}
}

// TestExternalScanner_EmptyFindings — exit 0, empty JSON, no findings and
// no errors. Verifies the happy-clean path.
func TestExternalScanner_EmptyFindings(t *testing.T) {
	t.Parallel()

	path := writeExternalScanner(t, "clean", `{"findings":[],"errors":[]}`, 0)
	res, err := (&ExternalScanner{Path: path}).Scan(t.TempDir())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Findings) != 0 || len(res.Errors) != 0 {
		t.Errorf("expected empty result; got %+v", res)
	}
}

// TestExternalScanner_ExitCode1 — exit 1 is a valid "findings present"
// signal per the protocol. Treated identically to exit 0 + findings.
func TestExternalScanner_ExitCode1(t *testing.T) {
	t.Parallel()

	out := `{"findings":[{"severity":"medium","file":"x","line":1,"description":"d"}],"errors":[]}`
	path := writeExternalScanner(t, "findings", out, 1)

	res, err := (&ExternalScanner{Path: path}).Scan(t.TempDir())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding on exit 1, got %d", len(res.Findings))
	}
}

// TestExternalScanner_ExitCode2 — exit ≥2 is a scanner error. Stderr is
// captured into ScanResult.Errors; no findings are returned (stdout is
// ignored because exit code signals a malfunction).
func TestExternalScanner_ExitCode2(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "broken")
	script := "#!/bin/sh\necho 'boom' 1>&2\nexit 2\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	res, err := (&ExternalScanner{Path: path}).Scan(dir)
	if err != nil {
		t.Fatalf("Scan should not return hard error; got %v", err)
	}
	if len(res.Errors) == 0 {
		t.Errorf("expected error recorded for exit 2; got %+v", res)
	}
	if len(res.Findings) != 0 {
		t.Errorf("expected no findings on scanner error; got %+v", res.Findings)
	}
}

// TestExternalScanner_Timeout — scanner sleeps longer than Timeout. Must
// record a timeout error and return without hanging.
func TestExternalScanner_Timeout(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "slow")
	script := "#!/bin/sh\nsleep 5\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	s := &ExternalScanner{Path: path, Timeout: 100 * time.Millisecond}
	start := time.Now()
	res, err := s.Scan(dir)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Scan should not hard-error on timeout; got %v", err)
	}
	// 100ms timeout + up to 2s WaitDelay for pipe closure ≈ 2.1s worst case.
	// Allow a small margin for slow CI schedulers.
	if elapsed > 4*time.Second {
		t.Errorf("timeout did not kill subprocess in reasonable time (%s)", elapsed)
	}
	if len(res.Errors) == 0 {
		t.Errorf("expected timeout error; got %+v", res)
	}
}

// TestExternalScanner_InvalidJSON — scanner writes non-JSON to stdout. The
// parser records an error and returns an empty result rather than panicking.
func TestExternalScanner_InvalidJSON(t *testing.T) {
	t.Parallel()

	path := writeExternalScanner(t, "garbled", "not valid json", 0)
	res, err := (&ExternalScanner{Path: path}).Scan(t.TempDir())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Errors) == 0 {
		t.Errorf("expected invalid-JSON error; got %+v", res)
	}
}

// TestExternalScanner_NotFound — scanner path points to a nonexistent file.
// Record as a non-fatal error so the chain can continue.
func TestExternalScanner_NotFound(t *testing.T) {
	t.Parallel()

	s := &ExternalScanner{Path: filepath.Join(t.TempDir(), "missing")}
	res, err := s.Scan(t.TempDir())
	if err != nil {
		t.Fatalf("Scan should not hard-error on missing binary; got %v", err)
	}
	if len(res.Errors) == 0 {
		t.Errorf("expected missing-binary error; got %+v", res)
	}
}

// --- Chain ------------------------------------------------------------------

// fakeScanner is a HookScanner test double so chain tests don't need real
// subprocesses.
type fakeScanner struct {
	name     string
	findings []ScanFinding
	errs     []string
	err      error
}

func (f *fakeScanner) Name() string { return f.name }
func (f *fakeScanner) Scan(dir string) (ScanResult, error) {
	return ScanResult{Findings: f.findings, Errors: f.errs}, f.err
}

// TestChainScanners_MergesFindings — two scanners with disjoint findings.
// Both appear in the merged result with their Scanner fields preserved.
func TestChainScanners_MergesFindings(t *testing.T) {
	t.Parallel()

	a := &fakeScanner{name: "a", findings: []ScanFinding{{Description: "from-a", Scanner: "a"}}}
	b := &fakeScanner{name: "b", findings: []ScanFinding{{Description: "from-b", Scanner: "b"}}}

	res, _ := ChainScanners(a, b).Scan("/tmp")
	if len(res.Findings) != 2 {
		t.Fatalf("expected 2 findings; got %d", len(res.Findings))
	}
	if res.Findings[0].Scanner != "a" || res.Findings[1].Scanner != "b" {
		t.Errorf("scanners not preserved in order; got %+v", res.Findings)
	}
}

// TestChainScanners_MergesErrors — one scanner returns a hard error; the
// chain continues and records the error, preserving findings from the good
// scanner.
func TestChainScanners_MergesErrors(t *testing.T) {
	t.Parallel()

	good := &fakeScanner{name: "good", findings: []ScanFinding{{Description: "keep me"}}}
	bad := &fakeScanner{name: "bad", err: fmt.Errorf("boom")}

	res, _ := ChainScanners(good, bad).Scan("/tmp")
	if len(res.Findings) != 1 {
		t.Errorf("expected good finding preserved; got %d findings", len(res.Findings))
	}
	if len(res.Errors) == 0 {
		t.Errorf("expected error recorded for bad scanner; got %+v", res)
	}
}

// TestChainScanners_Empty — no scanners, empty result, no error.
func TestChainScanners_Empty(t *testing.T) {
	t.Parallel()

	res, err := ChainScanners().Scan("/tmp")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Findings) != 0 || len(res.Errors) != 0 {
		t.Errorf("empty chain produced output: %+v", res)
	}
}

// TestChainScanners_OrderPreserved — findings from the first scanner come
// before findings from the second. Lets downstream UI rely on deterministic
// ordering for display.
func TestChainScanners_OrderPreserved(t *testing.T) {
	t.Parallel()

	a := &fakeScanner{name: "a", findings: []ScanFinding{{Description: "1"}, {Description: "2"}}}
	b := &fakeScanner{name: "b", findings: []ScanFinding{{Description: "3"}}}

	res, _ := ChainScanners(a, b).Scan("/tmp")
	got := []string{res.Findings[0].Description, res.Findings[1].Description, res.Findings[2].Description}
	want := []string{"1", "2", "3"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

// --- RunScanChain integration ----------------------------------------------

// TestRunScanChain_BuiltinOnly — no external scanners, builtin runs against
// the hook dir. Returns findings merged as if the builtin were invoked
// directly.
func TestRunScanChain_BuiltinOnly(t *testing.T) {
	t.Parallel()

	dir := writeHookDir(t,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"curl bad","type":"command"}]}]}}`,
		nil)

	res, err := RunScanChain(dir, nil)
	if err != nil {
		t.Fatalf("RunScanChain: %v", err)
	}
	if len(res.Findings) == 0 {
		t.Fatal("expected builtin findings")
	}
}

// TestRunScanChain_WithExternal — builtin + a real external script. The
// merged result contains findings from both, each tagged with its own
// scanner name.
func TestRunScanChain_WithExternal(t *testing.T) {
	t.Parallel()

	dir := writeHookDir(t,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"curl bad","type":"command"}]}]}}`,
		nil)

	out := `{"findings":[{"severity":"medium","file":"z","line":1,"description":"external"}],"errors":[]}`
	extPath := writeExternalScanner(t, "ext", out, 0)

	res, err := RunScanChain(dir, []string{extPath})
	if err != nil {
		t.Fatalf("RunScanChain: %v", err)
	}

	sawBuiltin, sawExternal := false, false
	for _, f := range res.Findings {
		if f.Scanner == "builtin" {
			sawBuiltin = true
		}
		if f.Scanner == "ext" {
			sawExternal = true
		}
	}
	if !sawBuiltin {
		t.Error("no builtin findings in merged result")
	}
	if !sawExternal {
		t.Error("no external findings in merged result")
	}
}

// TestRunScanChain_BadExternalPath — external scanner doesn't exist; the
// builtin scanner still runs and its findings are preserved. The missing
// scanner produces an entry in Errors.
func TestRunScanChain_BadExternalPath(t *testing.T) {
	t.Parallel()

	dir := writeHookDir(t,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"curl bad","type":"command"}]}]}}`,
		nil)

	res, err := RunScanChain(dir, []string{"/nonexistent/scanner"})
	if err != nil {
		t.Fatalf("RunScanChain: %v", err)
	}
	if len(res.Findings) == 0 {
		t.Error("expected builtin findings preserved despite missing external scanner")
	}
	if len(res.Errors) == 0 {
		t.Error("expected error recorded for missing external scanner")
	}
}

// TestHighestSeverity — convenience helper used by the installer to decide
// blocking policy. Ranks high > medium > low > info; empty input returns
// empty string.
func TestHighestSeverity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		findings []ScanFinding
		want     string
	}{
		{"empty", nil, ""},
		{"single low", []ScanFinding{{Severity: "low"}}, "low"},
		{"mixed", []ScanFinding{{Severity: "low"}, {Severity: "high"}, {Severity: "medium"}}, "high"},
		{"case-insensitive", []ScanFinding{{Severity: "HIGH"}}, "high"},
		{"unknown-ignored", []ScanFinding{{Severity: "bogus"}, {Severity: "low"}}, "low"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := HighestSeverity(tc.findings); got != tc.want {
				t.Errorf("got %q; want %q", got, tc.want)
			}
		})
	}
}

// TestScanFinding_JSONRoundTrip — the external scanner protocol round-trips
// ScanFinding JSON in and out. Guards against accidental breaking changes
// to field tags.
func TestScanFinding_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	in := ScanFinding{Severity: "high", File: "x.sh", Line: 3, Description: "d", Scanner: "s"}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out ScanFinding
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("round-trip: got %+v; want %+v", out, in)
	}
}
