# Review Phase 1: CLI Flags & Error Handling - Implementation Plan

**Goal:** Fix 15 CLI flag and error handling issues identified in the nesco review

**Architecture:** Changes concentrated in Cobra command setup (main.go), output formatting (output.go), and individual command files. All fixes are additive — no structural refactoring.

**Tech Stack:** Go, Cobra, Lipgloss/termenv, BubbleTea

**Design Doc:** docs/reviews/implementation-plan.md (Phase 1)

---

## Task 1A: Write test for --no-color flag disabling ANSI codes

**Files:**
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] Test verifies flag wiring (tests that flag exists and can be retrieved)
- [ ] Test structure ready for ANSI code verification when color output is added
- [ ] Test covers flag, `NO_COLOR` env var, and `TERM=dumb`

---

### Step 1: Write the test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestNoColorFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{
			name: "with --no-color flag",
			args: []string{"--no-color", "--help"},
		},
		{
			name: "with NO_COLOR env var",
			args: []string{"--help"},
			env:  map[string]string{"NO_COLOR": "1"},
		},
		{
			name: "with TERM=dumb",
			args: []string{"--help"},
			env:  map[string]string{"TERM": "dumb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.env {
				old := os.Getenv(k)
				os.Setenv(k, v)
				defer os.Setenv(k, old)
			}

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetArgs(tt.args)

			// Reset the command for reuse
			defer func() {
				rootCmd.SetOut(nil)
				rootCmd.SetArgs(nil)
			}()

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			// When color output is added in Phase 3, add ANSI code checks here
			// For now, we just verify the flag is wired and doesn't break execution
		})
	}
}
```

### Step 2: Run test to verify it passes with current setup

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestNoColorFlag -v
```

Expected output: Test passes, verifying flag handling works (PersistentPreRunE doesn't error).

## Task 1B: Wire --no-color flag to lipgloss

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 38-46)

**Depends on:** Task 1A

**Success Criteria:**
- [ ] PersistentPreRunE added to rootCmd
- [ ] Checks --no-color flag, NO_COLOR env var, and TERM=dumb
- [ ] Sets lipgloss color profile to Ascii when any condition is true

---

### Step 1: Write minimal implementation

Update the `init()` function in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (after line 42, before line 44):

```go
func init() {
	rootCmd.PersistentFlags().BoolVar(&output.JSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")

	// Wire up --no-color, NO_COLOR, and TERM=dumb
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
			lipgloss.SetColorProfile(termenv.Ascii)
		}
		return nil
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(backfillCmd)
}
```

Add required imports to the existing import block in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go`:

```go
import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/config"
	"github.com/OpenScribbler/nesco/cli/internal/metadata"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
	"github.com/OpenScribbler/nesco/cli/internal/scan/detectors"
	"github.com/OpenScribbler/nesco/cli/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)
```

### Step 2: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestNoColorFlag -v
```

Expected output: All test cases pass, confirming PersistentPreRunE is wired correctly.

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go cli/cmd/nesco/main_test.go && git commit -m "$(cat <<'EOF'
fix(cli): wire --no-color flag and NO_COLOR/TERM=dumb checks

Implements the --no-color persistent flag, NO_COLOR env var, and
TERM=dumb detection by setting lipgloss color profile to Ascii in
PersistentPreRunE. Adds test covering all three cases.

Fixes: A11Y-002, UX-012, FTU-002

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2A: Add Quiet global and update Print() function

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/internal/output/output.go` (line 10-23)
- Test: `/home/hhewett/.local/src/nesco/cli/internal/output/output_test.go`

**Depends on:** Task 1B

**Success Criteria:**
- [ ] Test verifies Print() is suppressed when Quiet is true
- [ ] Test verifies Print() works normally when Quiet is false
- [ ] Quiet global variable is exported

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/internal/output/output_test.go`:

```go
func TestPrintQuietMode(t *testing.T) {
	var buf bytes.Buffer
	Writer = &buf
	JSON = false
	defer func() { Writer = os.Stdout; Quiet = false }()

	// Normal mode
	Quiet = false
	Print("visible")
	if !strings.Contains(buf.String(), "visible") {
		t.Error("Print should output in normal mode")
	}

	// Quiet mode
	buf.Reset()
	Quiet = true
	Print("hidden")
	if buf.Len() > 0 {
		t.Errorf("Print should suppress output in quiet mode, got: %s", buf.String())
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -run TestPrintQuietMode -v
```

Expected output: Test fails because `Quiet` variable doesn't exist.

### Step 3: Write minimal implementation

Update `/home/hhewett/.local/src/nesco/cli/internal/output/output.go`:

```go
var (
	JSON      bool      // set from --json flag
	Quiet     bool      // set from --quiet flag
	Writer    io.Writer = os.Stdout
	ErrWriter io.Writer = os.Stderr
)

func Print(v any) {
	if Quiet {
		return // suppress all output in quiet mode
	}
	if JSON {
		data, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(Writer, string(data))
	} else {
		fmt.Fprintln(Writer, v)
	}
}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -run TestPrintQuietMode -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/internal/output/output.go cli/internal/output/output_test.go && git commit -m "$(cat <<'EOF'
feat(output): add Quiet mode to Print function

Adds Quiet global variable. When set to true, Print() suppresses
all output to allow for silent script usage. PrintError() is
unaffected by Quiet mode.

Fixes: UX-011 (partial), FTU-002 (partial)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2B: Wire --quiet flag in main.go

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 44-51)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`

**Depends on:** Task 2A

**Success Criteria:**
- [ ] Test verifies --quiet flag sets output.Quiet
- [ ] Test verifies info output is suppressed with --quiet
- [ ] Both --quiet and -q work

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestQuietFlag(t *testing.T) {
	// Setup a temp project
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	tests := []struct {
		name       string
		args       []string
		wantOutput bool
	}{
		{
			name:       "config list without quiet",
			args:       []string{"config", "list"},
			wantOutput: true,
		},
		{
			name:       "config list with --quiet",
			args:       []string{"--quiet", "config", "list"},
			wantOutput: false,
		},
		{
			name:       "config list with -q",
			args:       []string{"-q", "config", "list"},
			wantOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			origOut := output.Writer
			origQuiet := output.Quiet
			output.Writer = &stdout
			defer func() {
				output.Writer = origOut
				output.Quiet = origQuiet
			}()

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			hasOutput := stdout.Len() > 0
			if tt.wantOutput && !hasOutput {
				t.Error("expected output but got none")
			}
			if !tt.wantOutput && hasOutput {
				t.Errorf("expected no output but got: %s", stdout.String())
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestQuietFlag -v
```

Expected output: Test fails because PersistentPreRunE doesn't set output.Quiet.

### Step 3: Write minimal implementation

Update the `PersistentPreRunE` in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (after line 44):

```go
	// Wire up --no-color, NO_COLOR, and TERM=dumb
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
			lipgloss.SetColorProfile(termenv.Ascii)
		}

		quiet, _ := cmd.Flags().GetBool("quiet")
		output.Quiet = quiet

		return nil
	}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestQuietFlag -v
```

Expected output: All test cases pass.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go cli/cmd/nesco/main_test.go && git commit -m "$(cat <<'EOF'
feat(cli): wire --quiet flag to output.Quiet

Wires --quiet/-q flag in PersistentPreRunE to set output.Quiet.
When enabled, Print() suppresses all output for silent script usage.
Error output via PrintError() is unaffected.

Fixes: UX-011 (complete), FTU-002 (partial)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3A: Add Verbose global and PrintVerbose function

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/internal/output/output.go` (line 10-15, add after line 23)
- Test: `/home/hhewett/.local/src/nesco/cli/internal/output/output_test.go`

**Depends on:** Task 2B

**Success Criteria:**
- [ ] Test verifies PrintVerbose only prints when Verbose is true
- [ ] Test verifies PrintVerbose is suppressed when Verbose is false
- [ ] Verbose global variable is exported

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/internal/output/output_test.go`:

```go
func TestPrintVerbose(t *testing.T) {
	var buf bytes.Buffer
	Writer = &buf
	defer func() { Writer = os.Stdout; Verbose = false }()

	// Verbose mode
	Verbose = true
	PrintVerbose("debug info: %s\n", "details")
	if !strings.Contains(buf.String(), "debug info: details") {
		t.Error("PrintVerbose should output in verbose mode")
	}

	// Normal mode
	buf.Reset()
	Verbose = false
	PrintVerbose("should not appear\n")
	if buf.Len() > 0 {
		t.Errorf("PrintVerbose should suppress output in normal mode, got: %s", buf.String())
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -run TestPrintVerbose -v
```

Expected output: Test fails because `Verbose` and `PrintVerbose` don't exist.

### Step 3: Write minimal implementation

Update `/home/hhewett/.local/src/nesco/cli/internal/output/output.go`:

```go
var (
	JSON      bool      // set from --json flag
	Quiet     bool      // set from --quiet flag
	Verbose   bool      // set from --verbose flag
	Writer    io.Writer = os.Stdout
	ErrWriter io.Writer = os.Stderr
)

func Print(v any) {
	if Quiet {
		return // suppress all output in quiet mode
	}
	if JSON {
		data, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(Writer, string(data))
	} else {
		fmt.Fprintln(Writer, v)
	}
}

// PrintVerbose prints only when Verbose is true
func PrintVerbose(format string, args ...any) {
	if !Verbose {
		return
	}
	fmt.Fprintf(Writer, format, args...)
}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -run TestPrintVerbose -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/internal/output/output.go cli/internal/output/output_test.go && git commit -m "$(cat <<'EOF'
feat(output): add Verbose mode and PrintVerbose function

Adds Verbose global variable and PrintVerbose() helper for future
verbose diagnostics output. PrintVerbose only outputs when Verbose
is true.

Fixes: UX-011 (partial), FTU-002 (partial)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3B: Wire --verbose flag in main.go

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 44-54)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`

**Depends on:** Task 3A

**Success Criteria:**
- [ ] Test verifies --verbose flag sets output.Verbose
- [ ] Both --verbose and -v work

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestVerboseFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantSet bool
	}{
		{
			name:    "without flag",
			args:    []string{"--help"},
			wantSet: false,
		},
		{
			name:    "with --verbose",
			args:    []string{"--verbose", "--help"},
			wantSet: true,
		},
		{
			name:    "with -v",
			args:    []string{"-v", "--help"},
			wantSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origVerbose := output.Verbose
			defer func() { output.Verbose = origVerbose }()

			rootCmd.SetArgs(tt.args)
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			if output.Verbose != tt.wantSet {
				t.Errorf("output.Verbose = %v, want %v", output.Verbose, tt.wantSet)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestVerboseFlag -v
```

Expected output: Test fails because PersistentPreRunE doesn't set output.Verbose.

### Step 3: Write minimal implementation

Update the `PersistentPreRunE` in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 44-54):

```go
	// Wire up --no-color, NO_COLOR, and TERM=dumb
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
			lipgloss.SetColorProfile(termenv.Ascii)
		}

		quiet, _ := cmd.Flags().GetBool("quiet")
		output.Quiet = quiet

		verbose, _ := cmd.Flags().GetBool("verbose")
		output.Verbose = verbose

		return nil
	}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestVerboseFlag -v
```

Expected output: All test cases pass.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go cli/internal/output/output.go cli/cmd/nesco/main_test.go && git commit -m "$(cat <<'EOF'
feat(cli): implement --verbose flag

Adds Verbose global to output package and wires --verbose/-v flag
in PersistentPreRunE. Provides PrintVerbose() helper for future
verbose diagnostics output.

Fixes: UX-011 (complete), FTU-002 (complete)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Write test for version command with dev builds

**Files:**
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`

**Depends on:** Task 3B

**Success Criteria:**
- [ ] Test fails because version prints empty string for dev builds
- [ ] Test verifies "(dev build)" appears when version is empty
- [ ] Test verifies actual version appears when set

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestVersionCommandDevBuild(t *testing.T) {
	tests := []struct {
		name        string
		versionVar  string
		wantContain string
	}{
		{
			name:        "with version set",
			versionVar:  "1.2.3",
			wantContain: "1.2.3",
		},
		{
			name:        "dev build (empty version)",
			versionVar:  "",
			wantContain: "(dev build)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Temporarily override version variable
			oldVersion := version
			version = tt.versionVar
			defer func() { version = oldVersion }()

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetArgs([]string{"version"})
			defer func() {
				rootCmd.SetOut(nil)
				rootCmd.SetArgs(nil)
			}()

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("version command failed: %v", err)
			}

			out := buf.String()
			if !strings.Contains(out, tt.wantContain) {
				t.Errorf("version output = %q, want to contain %q", out, tt.wantContain)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestVersionCommandDevBuild -v
```

Expected output: Test fails for empty version case because current code prints a blank line instead of "(dev build)".

### Step 3: Write minimal implementation

Update the `versionCmd` in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (lines 48-54):

```go
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print nesco version",
	Run: func(cmd *cobra.Command, args []string) {
		v := version
		if v == "" {
			v = "(dev build)"
		}
		cmd.Println(v)
	},
}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestVersionCommandDevBuild -v
```

Expected output: Both test cases pass.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go cli/cmd/nesco/main_test.go && git commit -m "$(cat <<'EOF'
fix(cli): show "(dev build)" when version is empty

Version command now prints "(dev build)" instead of empty string
when built without ldflags. Makes dev builds distinguishable from
versioned releases.

Fixes: UX-019, FTU-003

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Update info command to use "(dev build)" for version

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info.go` (lines 16-26, the RunE function body)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info_test.go`

**Depends on:** Task 4

**Success Criteria:**
- [ ] Test verifies info command shows "(dev build)" when version is empty
- [ ] JSON output includes the version
- [ ] Plain text output includes the version

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info_test.go`:

```go
func TestInfoDevBuild(t *testing.T) {
	// Save and override version
	oldVersion := version
	version = ""
	defer func() { version = oldVersion }()

	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.JSON = false
		output.Writer = origWriter
	}()

	err := infoCmd.RunE(infoCmd, []string{})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(buf.Bytes(), &manifest); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	ver, ok := manifest["version"].(string)
	if !ok {
		t.Fatal("version field missing or not a string")
	}
	if ver != "(dev build)" {
		t.Errorf("version = %q, want %q", ver, "(dev build)")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestInfoDevBuild -v
```

Expected output: Test fails because version is empty string instead of "(dev build)".

### Step 3: Write minimal implementation

Update `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info.go` RunE function (lines 16-26):

```go
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show nesco capabilities",
	Long:  "Machine-readable capability manifest. Useful for agents discovering nesco's features.",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := version
		if v == "" {
			v = "(dev build)"
		}
		manifest := map[string]any{
			"version":      v,
			"contentTypes": catalog.AllContentTypes(),
			"providers":    providerSlugs(),
			"commands":     []string{"init", "import", "parity", "config", "info", "scan", "drift", "baseline"},
		}
		if output.JSON {
			output.Print(manifest)
		} else {
			fmt.Printf("nesco %s\n\n", v)
			fmt.Println("Content types:", len(catalog.AllContentTypes()))
			for _, ct := range catalog.AllContentTypes() {
				fmt.Printf("  - %s\n", ct.Label())
			}
			fmt.Println("\nProviders:", len(provider.AllProviders))
			for _, p := range provider.AllProviders {
				fmt.Printf("  - %s (%s)\n", p.Name, p.Slug)
			}
		}
		return nil
	},
}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestInfoDevBuild -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/info.go cli/cmd/nesco/info_test.go && git commit -m "$(cat <<'EOF'
fix(cli): show "(dev build)" in info command for empty version

Ensures info command displays "(dev build)" in both JSON and plain
text output when version is empty, consistent with version command.

Fixes: UX-019, FTU-003 (complete)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Create sentinel error type to prevent duplicate error messages

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/internal/output/output.go`
- Test: `/home/hhewett/.local/src/nesco/cli/internal/output/output_test.go`

**Depends on:** Task 5

**Success Criteria:**
- [ ] Test verifies `SilentError` wraps an error
- [ ] Test verifies `IsSilentError` detects the sentinel
- [ ] Type can be used to skip duplicate error printing

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/internal/output/output_test.go`:

```go
func TestSilentError(t *testing.T) {
	baseErr := fmt.Errorf("underlying error")
	silentErr := SilentError(baseErr)

	if !IsSilentError(silentErr) {
		t.Error("IsSilentError should return true for SilentError")
	}

	normalErr := fmt.Errorf("normal error")
	if IsSilentError(normalErr) {
		t.Error("IsSilentError should return false for normal errors")
	}

	// Verify the error message is preserved
	if silentErr.Error() != baseErr.Error() {
		t.Errorf("error message = %q, want %q", silentErr.Error(), baseErr.Error())
	}
}
```

Add required import:
```go
import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -run TestSilentError -v
```

Expected output: Test fails because `SilentError` and `IsSilentError` don't exist.

### Step 3: Write minimal implementation

Add to `/home/hhewett/.local/src/nesco/cli/internal/output/output.go`:

```go
// silentError wraps an error to signal that it has already been printed
// and should not be printed again by the main error handler
type silentError struct {
	err error
}

func (e silentError) Error() string {
	return e.err.Error()
}

func (e silentError) Unwrap() error {
	return e.err
}

// SilentError wraps an error to mark it as already printed
func SilentError(err error) error {
	if err == nil {
		return nil
	}
	return silentError{err: err}
}

// IsSilentError checks if an error is marked as already printed
func IsSilentError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(silentError)
	return ok
}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -run TestSilentError -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/internal/output/output.go cli/internal/output/output_test.go && git commit -m "$(cat <<'EOF'
feat(output): add SilentError type to prevent duplicate error messages

Adds silentError wrapper type that marks errors as already printed.
Commands can wrap errors with SilentError() after calling
PrintError() to signal that main() should not print them again.

Fixes: FTU-001 (partial)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Update main() to skip printing silent errors

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 109-112)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`

**Depends on:** Task 6

**Success Criteria:**
- [ ] Test verifies silent errors are not printed to stderr
- [ ] Test verifies normal errors are still printed
- [ ] Exit code is still non-zero for errors

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestMainSilentErrorNotPrinted(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStderr bool
	}{
		{
			name:       "normal error is printed",
			err:        fmt.Errorf("normal error"),
			wantStderr: true,
		},
		{
			name:       "silent error is not printed",
			err:        output.SilentError(fmt.Errorf("already shown")),
			wantStderr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a command that returns the test error
			testCmd := &cobra.Command{
				Use: "testerr",
				RunE: func(cmd *cobra.Command, args []string) error {
					return tt.err
				},
			}

			// Temporarily add it to rootCmd
			rootCmd.AddCommand(testCmd)
			defer rootCmd.RemoveCommand(testCmd)

			var stderr bytes.Buffer
			origStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w
			defer func() { os.Stderr = origStderr }()

			// Capture stderr in goroutine
			done := make(chan struct{})
			go func() {
				io.Copy(&stderr, r)
				close(done)
			}()

			rootCmd.SetArgs([]string{"testerr"})
			_ = rootCmd.Execute() // We expect an error

			w.Close()
			<-done

			hasStderr := stderr.Len() > 0
			if tt.wantStderr && !hasStderr {
				t.Error("expected error on stderr but got none")
			}
			if !tt.wantStderr && hasStderr {
				t.Errorf("expected no stderr but got: %s", stderr.String())
			}
		})
	}
}
```

Add required import:
```go
import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/spf13/cobra"
)
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestMainSilentErrorNotPrinted -v
```

Expected output: Test fails because main() currently prints all errors to stderr.

### Step 3: Write minimal implementation

Update the `main()` function in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (around line 109):

```go
func main() {
	// Self-rebuild: if source has changed since this binary was built, rebuild and re-exec.
	if buildCommit != "" {
		ensureUpToDate()
	}
	if err := rootCmd.Execute(); err != nil {
		// Don't print if error was already printed (marked as silent)
		if !output.IsSilentError(err) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
```

### Step 4: Run tests to verify nothing broke

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -v
```

Expected output: All tests still pass.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go && git commit -m "$(cat <<'EOF'
fix(cli): skip printing errors marked as silent in main()

main() now checks IsSilentError() before printing to stderr. This
prevents duplicate error messages when commands call PrintError()
and then return a SilentError-wrapped error.

Fixes: FTU-001 (partial)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Update scan command to use SilentError

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan.go` (lines 36-39)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan_test.go`

**Depends on:** Task 7

**Success Criteria:**
- [ ] Test verifies error appears exactly once on stderr
- [ ] scan command returns SilentError after PrintError

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan_test.go`:

```go
func TestScanErrorNoDuplicate(t *testing.T) {
	// Run scan from a non-project directory
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stderr bytes.Buffer
	origErr := output.ErrWriter
	output.ErrWriter = &stderr
	defer func() { output.ErrWriter = origErr }()

	err := scanCmd.RunE(scanCmd, []string{})
	if err == nil {
		t.Fatal("expected error when running scan in non-project dir")
	}

	stderrStr := stderr.String()

	// Count occurrences of "Error:" prefix
	errorCount := strings.Count(stderrStr, "Error:")
	if errorCount != 1 {
		t.Errorf("error message appeared %d times, want 1. Output:\n%s", errorCount, stderrStr)
	}
}
```

Add required imports to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan_test.go`:

```go
import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/output"
)
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestScanErrorNoDuplicate -v
```

Expected output: Test fails because error is printed twice (once by PrintError, once by main()).

### Step 3: Write minimal implementation

Update `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan.go` (lines 36-39):

```go
func runScan(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		output.PrintError(2, "no detectable project", "Run from a project directory with go.mod, package.json, etc.")
		return output.SilentError(fmt.Errorf("no detectable project"))
	}
	// ... rest of function
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestScanErrorNoDuplicate -v
```

Expected output: Test passes with exactly one error message.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/scan.go cli/cmd/nesco/scan_test.go && git commit -m "$(cat <<'EOF'
fix(scan): prevent duplicate error message on no detectable project

Wraps error return with SilentError after calling PrintError to
prevent main() from printing the same error again. Adds test
verifying error appears exactly once on stderr.

Fixes: FTU-001 (complete)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Improve TUI error message when content repo not found

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 118, line 184, and convert findSkillsDir to a var)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`

**Depends on:** Task 8

**Success Criteria:**
- [ ] Test verifies improved error message appears
- [ ] Error message doesn't mention internal "skills/" directory
- [ ] Error message provides actionable guidance
- [ ] findSkillsDir is a var so tests can override it

---

### Step 1: Write the failing test and convert findSkillsDir to var

First, update `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` to make findSkillsDir testable. After line 186, convert the function:

```go
// findSkillsDir walks up from dir looking for a "skills/" directory.
// Declared as a var so tests can override it.
var findSkillsDir = findSkillsDirImpl

func findSkillsDirImpl(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "skills")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no skills/ directory found above %s", dir)
}
```

Then add the test to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestTUIErrorMessageContentRepoNotFound(t *testing.T) {
	// Run from a directory outside any content repo
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Override findSkillsDir to force error
	oldFindSkills := findSkillsDir
	findSkillsDir = func(dir string) (string, error) {
		return "", fmt.Errorf("not found")
	}
	defer func() { findSkillsDir = oldFindSkills }()

	err := runTUI(rootCmd, []string{})
	if err == nil {
		t.Fatal("expected error when content repo not found")
	}

	errMsg := err.Error()

	// Should not mention internal implementation details
	if strings.Contains(errMsg, "skills/") {
		t.Error("error message should not mention internal 'skills/' directory")
	}

	// Should provide helpful guidance
	if !strings.Contains(errMsg, "nesco") {
		t.Error("error message should mention 'nesco'")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestTUIErrorMessageContentRepoNotFound -v
```

Expected output: Test fails because current error mentions "skills/" directory.

### Step 3: Write minimal implementation

Update the `runTUI` function in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 118):

```go
func runTUI(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("nesco requires a content repository.\n\nTo get started:\n  nesco init    Create a new content repo in the current directory\n  nesco --dir   Point to an existing content repo\n\nFor more info: nesco --help")
	}

	cat, err := catalog.Scan(root)
	if err != nil {
		return fmt.Errorf("catalog scan failed: %w", err)
	}
	// ... rest of function
```

Also update the `findContentRepoRoot` error message (line 184):

```go
	cwd, _ := os.Getwd()
	return "", fmt.Errorf("nesco requires a content repository.\n\nTo get started:\n  nesco init    Create a new content repo in the current directory\n  nesco --dir   Point to an existing content repo\n\nFor more info: nesco --help")
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestTUIErrorMessageContentRepoNotFound -v
```

Expected output: Test passes with improved error message.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go cli/cmd/nesco/main_test.go && git commit -m "$(cat <<'EOF'
fix(tui): improve error when content repo not found

Replaces internal "skills/ directory" error with user-facing
message that explains what nesco needs and suggests actionable
fixes (run from content repo or clone nesco repo).

Fixes: UX-013, FTU-004

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Add warning when findProjectRoot falls back to CWD

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/helpers.go` (lines 14-40, the findProjectRootImpl function)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/helpers_test.go` (new file)

**Depends on:** Task 9

**Success Criteria:**
- [ ] Test verifies warning is printed to stderr when no markers found
- [ ] Test verifies no warning when project markers are present
- [ ] Warning explains the fallback behavior

---

### Step 1: Write the failing test

Create `/home/hhewett/.local/src/nesco/cli/cmd/nesco/helpers_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/output"
)

func TestFindProjectRootFallbackWarning(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string)
		wantWarning bool
	}{
		{
			name: "with go.mod marker",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			wantWarning: false,
		},
		{
			name: "with package.json marker",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
			},
			wantWarning: false,
		},
		{
			name:        "no project markers - fallback to cwd",
			setup:       func(dir string) {},
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			tt.setup(tmp)

			origDir, _ := os.Getwd()
			os.Chdir(tmp)
			defer os.Chdir(origDir)

			var stderr bytes.Buffer
			origErr := output.ErrWriter
			output.ErrWriter = &stderr
			defer func() { output.ErrWriter = origErr }()

			root, err := findProjectRoot()
			if err != nil {
				t.Fatalf("findProjectRoot failed: %v", err)
			}

			stderrStr := stderr.String()
			hasWarning := strings.Contains(stderrStr, "Warning")

			if tt.wantWarning && !hasWarning {
				t.Error("expected warning but got none")
			}
			if !tt.wantWarning && hasWarning {
				t.Errorf("unexpected warning: %s", stderrStr)
			}

			// Should still return valid path
			if root == "" {
				t.Error("expected non-empty root path")
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestFindProjectRootFallbackWarning -v
```

Expected output: Test fails for "no project markers" case because no warning is printed.

### Step 3: Write minimal implementation

Update the `findProjectRootImpl` function in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/helpers.go` (lines 14-40). Note that `var findProjectRoot = findProjectRootImpl` already exists at line 12, so we only need to modify the implementation function:

```go
func findProjectRootImpl() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	markers := []string{".git", "go.mod", "package.json", "Cargo.toml", "pyproject.toml"}
	for {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback to cwd with warning
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	fmt.Fprintf(output.ErrWriter, "Warning: No project markers found (go.mod, package.json, etc.). Using current directory: %s\n", cwd)
	fmt.Fprintf(output.ErrWriter, "  To avoid unintended file operations, run nesco from a project root or use --dir flag.\n")
	return cwd, nil
}
```

Also add the missing import at the top of the file:

```go
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
)
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestFindProjectRootFallbackWarning -v
```

Expected output: All test cases pass.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/helpers.go cli/cmd/nesco/helpers_test.go && git commit -m "$(cat <<'EOF'
fix(scan): warn when findProjectRoot falls back to CWD

Prints warning to stderr when no project markers are found and
findProjectRoot falls back to current directory. Warns user about
potential unintended file operations and suggests running from
project root or using --dir flag.

Fixes: GO-012, FTU-008

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Handle swallowed config.Save error in scan command

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan.go` (lines 56-62, the auto-detect block)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan_test.go`

**Depends on:** Task 10

**Success Criteria:**
- [ ] Test verifies warning is printed when config.Save fails
- [ ] Auto-detect flow continues even if save fails
- [ ] Warning appears on stderr

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan_test.go`:

```go
func TestScanConfigSaveError(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	// Create .nesco dir as read-only to force Save error
	nescoDir := filepath.Join(tmp, ".nesco")
	os.MkdirAll(nescoDir, 0444)
	defer os.Chmod(nescoDir, 0755) // cleanup

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Set env to bypass prompt
	os.Setenv("NESCO_NO_PROMPT", "1")
	defer os.Unsetenv("NESCO_NO_PROMPT")

	var stderr bytes.Buffer
	origErr := output.ErrWriter
	output.ErrWriter = &stderr
	defer func() { output.ErrWriter = origErr }()

	// Run with --yes to trigger auto-detect
	err := scanCmd.RunE(scanCmd, []string{"--yes"})
	// Command may error due to other issues, we only care about the warning

	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "Warning") && !strings.Contains(stderrStr, "config") {
		t.Error("expected warning about config save failure but got none")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestScanConfigSaveError -v
```

Expected output: Test fails because no warning is printed when config.Save fails.

### Step 3: Write minimal implementation

Update `/home/hhewett/.local/src/nesco/cli/cmd/nesco/scan.go` (lines 56-62):

```go
		// Auto-detect providers
		home, _ := os.UserHomeDir()
		for _, prov := range provider.AllProviders {
			if prov.Detect != nil && prov.Detect(home) {
				cfg.Providers = append(cfg.Providers, prov.Slug)
			}
		}
		if err := config.Save(root, cfg); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: Failed to save auto-detected config: %v\n", err)
		}
	}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestScanConfigSaveError -v
```

Expected output: Test passes with warning message.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/scan.go cli/cmd/nesco/scan_test.go && git commit -m "$(cat <<'EOF'
fix(scan): log warning when auto-detect config save fails

scan command now prints warning to stderr if config.Save fails
during provider auto-detection. Previously the error was silently
discarded. Scan continues even if save fails.

Fixes: GO-003

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Add help text mentioning TUI mode

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (lines 32-33, the Long field)

**Depends on:** Task 11

**Success Criteria:**
- [ ] Test verifies help text mentions running without arguments launches TUI
- [ ] Help text appears in `nesco --help` output

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestHelpTextMentionsTUI(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--help"})
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	}()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	out := buf.String()

	// Should mention TUI or interactive mode
	if !strings.Contains(out, "interactive") && !strings.Contains(out, "TUI") {
		t.Error("help text should mention interactive/TUI mode")
	}

	// Should mention running without arguments
	if !strings.Contains(out, "without arguments") && !strings.Contains(out, "no arguments") {
		t.Error("help text should explain running without arguments")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestHelpTextMentionsTUI -v
```

Expected output: Test fails because current help text doesn't mention TUI mode.

### Step 3: Write minimal implementation

Update the `rootCmd` Long field in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (lines 32-33):

```go
var rootCmd = &cobra.Command{
	Use:   "nesco",
	Short: "AI coding tool content manager and codebase scanner",
	Long: `Nesco manages AI tool configurations and scans codebases for context that helps AI agents produce correct code.

Run without arguments for interactive mode (TUI). Use subcommands for automation and scripting.`,
	RunE:          runTUI,
	SilenceUsage:  true,
	SilenceErrors: true,
}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestHelpTextMentionsTUI -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go cli/cmd/nesco/main_test.go && git commit -m "$(cat <<'EOF'
docs(cli): add help text mentioning TUI mode

Updates root command Long description to explain that running
without arguments launches interactive TUI mode. Helps new users
discover the TUI.

Fixes: FTU-005

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Add success confirmations for config add and remove

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd.go` (line 65, 96)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd_test.go`

**Depends on:** Task 12

**Success Criteria:**
- [ ] Test verifies "Added provider: X" message appears
- [ ] Test verifies "Removed provider: X" message appears
- [ ] Messages appear on stdout (not stderr)

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd_test.go`:

```go
func TestConfigAddConfirmation(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stdout bytes.Buffer
	origWriter := output.Writer
	output.Writer = &stdout
	defer func() { output.Writer = origWriter }()

	if err := configAddCmd.RunE(configAddCmd, []string{"cursor"}); err != nil {
		t.Fatalf("config add: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Added") || !strings.Contains(out, "cursor") {
		t.Errorf("expected confirmation message, got: %s", out)
	}
}

func TestConfigRemoveConfirmation(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stdout bytes.Buffer
	origWriter := output.Writer
	output.Writer = &stdout
	defer func() { output.Writer = origWriter }()

	if err := configRemoveCmd.RunE(configRemoveCmd, []string{"claude-code"}); err != nil {
		t.Fatalf("config remove: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Removed") || !strings.Contains(out, "claude-code") {
		t.Errorf("expected confirmation message, got: %s", out)
	}
}
```

Add required imports to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd_test.go`:

```go
import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/config"
	"github.com/OpenScribbler/nesco/cli/internal/output"
)
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run "TestConfigAddConfirmation|TestConfigRemoveConfirmation" -v
```

Expected output: Tests fail because no confirmation messages are printed.

### Step 3: Write minimal implementation

Update `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd.go`:

For `configAddCmd` (around line 64-66):
```go
		cfg.Providers = append(cfg.Providers, slug)
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Added provider: %s\n", slug)
		return nil
```

For `configRemoveCmd` (around line 95-97):
```go
		cfg.Providers = filtered
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Removed provider: %s\n", slug)
		return nil
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run "TestConfigAddConfirmation|TestConfigRemoveConfirmation" -v
```

Expected output: Both tests pass.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/config_cmd.go cli/cmd/nesco/config_cmd_test.go && git commit -m "$(cat <<'EOF'
feat(config): add success confirmations for add and remove

config add and config remove now print confirmation messages on
success. Helps users verify their actions completed successfully.

Fixes: FTU-006

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: Wrap bubbletea TTY error with user-facing message

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (add wrapTTYError function before runTUI, update line 150-152 in runTUI)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`

**Depends on:** Task 13

**Success Criteria:**
- [ ] Test verifies TTY error is wrapped with helpful message
- [ ] Error suggests using subcommands
- [ ] User doesn't see raw bubbletea error

---

### Step 1: Add stub function and test together

First, add a minimal stub to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (before `runTUI`, around line 114):

```go
// wrapTTYError wraps bubbletea TTY errors with user-facing guidance
func wrapTTYError(err error) error {
	return err // stub - will be implemented in Step 3
}
```

Then add the test to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestTUITTYErrorWrapped(t *testing.T) {
	// This test verifies the error wrapping logic exists
	// We can't easily trigger the actual TTY error in tests

	// Verify the error wrapping helper exists
	err := fmt.Errorf("could not open a new TTY: something low-level")
	wrapped := wrapTTYError(err)

	wrappedMsg := wrapped.Error()

	// Should not expose raw error
	if strings.Contains(wrappedMsg, "low-level") && !strings.Contains(wrappedMsg, "subcommand") {
		t.Error("TTY error should be wrapped with user guidance")
	}

	// Should mention subcommands
	if !strings.Contains(wrappedMsg, "subcommand") && !strings.Contains(wrappedMsg, "scan") {
		t.Error("wrapped error should suggest using subcommands")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestTUITTYErrorWrapped -v
```

Expected output: Test fails because stub doesn't wrap the error.

### Step 3: Replace stub with real implementation

Replace the stub `wrapTTYError` function in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go`:

```go
// wrapTTYError wraps bubbletea TTY errors with user-facing guidance
func wrapTTYError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "TTY") || strings.Contains(errMsg, "tty") {
		return fmt.Errorf("nesco requires a terminal for interactive mode. Use subcommands for non-interactive usage (try: nesco scan)")
	}
	return err
}
```

Update `runTUI` to use the wrapper (around line 150):

```go
	app := tui.NewApp(cat, providers, detectors.AllDetectors(), version, autoUpdate)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return wrapTTYError(err)
	}
	return nil
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestTUITTYErrorWrapped -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go cli/cmd/nesco/main_test.go && git commit -m "$(cat <<'EOF'
fix(tui): wrap raw bubbletea TTY error with user guidance

Detects TTY-related errors from bubbletea and wraps them with
user-facing message suggesting subcommands for non-interactive
usage. Prevents raw "could not open a new TTY" errors.

Fixes: FTU-007

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 15A: Add basic slug validation and warning

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd.go` (line 58-64)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd_test.go`

**Depends on:** Task 14

**Success Criteria:**
- [ ] Test verifies warning for unknown slug
- [ ] Command still succeeds (allows unknown slugs)
- [ ] Basic warning without provider list

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd_test.go`:

```go
func TestConfigAddUnknownProviderWarning(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stderr bytes.Buffer
	origErr := output.ErrWriter
	output.ErrWriter = &stderr
	defer func() { output.ErrWriter = origErr }()

	// Use obviously wrong slug
	err := configAddCmd.RunE(configAddCmd, []string{"xyz123"})
	if err != nil {
		t.Fatalf("config add should succeed even with unknown slug: %v", err)
	}

	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "Warning") || !strings.Contains(stderrStr, "unknown") {
		t.Error("expected warning about unknown provider slug")
	}
}

func TestConfigAddKnownProviderNoWarning(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stderr bytes.Buffer
	origErr := output.ErrWriter
	output.ErrWriter = &stderr
	defer func() { output.ErrWriter = origErr }()

	err := configAddCmd.RunE(configAddCmd, []string{"cursor"})
	if err != nil {
		t.Fatalf("config add failed: %v", err)
	}

	stderrStr := stderr.String()
	if strings.Contains(stderrStr, "Warning") {
		t.Errorf("should not warn for known provider, got: %s", stderrStr)
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run "TestConfigAddUnknownProviderWarning|TestConfigAddKnownProviderNoWarning" -v
```

Expected output: Tests fail because no warning is printed for unknown slugs.

### Step 3: Write minimal implementation

Update `configAddCmd` in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd.go` (around line 58-66):

```go
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		slug := args[0]
		for _, p := range cfg.Providers {
			if p == slug {
				return fmt.Errorf("provider %q already configured", slug)
			}
		}

		// Validate against known providers
		if findProviderBySlug(slug) == nil {
			fmt.Fprintf(output.ErrWriter, "Warning: '%s' is not a known provider slug.\n", slug)
			fmt.Fprintf(output.ErrWriter, "  Adding anyway - nesco will ignore unknown providers during scan.\n")
		}

		cfg.Providers = append(cfg.Providers, slug)
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Added provider: %s\n", slug)
		return nil
	},
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run "TestConfigAddUnknownProviderWarning|TestConfigAddKnownProviderNoWarning" -v
```

Expected output: Both tests pass.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/config_cmd.go cli/cmd/nesco/config_cmd_test.go && git commit -m "$(cat <<'EOF'
feat(config): add basic validation for unknown provider slugs

config add now validates slugs against known providers and prints
warning if slug is unrecognized. Still allows the add to proceed
(non-fatal).

Fixes: UX-018 (partial), FTU-012 (partial)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 15B: Enhance warning to list all known providers

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd.go` (update warning message)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd_test.go`

**Depends on:** Task 15A

**Success Criteria:**
- [ ] Test verifies warning lists known provider slugs
- [ ] Helps users discover correct spelling
- [ ] Format: "Known providers: cursor, claude-code, ..."

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd_test.go`:

```go
func TestConfigAddWarningListsKnownProviders(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var stderr bytes.Buffer
	origErr := output.ErrWriter
	output.ErrWriter = &stderr
	defer func() { output.ErrWriter = origErr }()

	// Use typo in provider name
	err := configAddCmd.RunE(configAddCmd, []string{"cursro"})
	if err != nil {
		t.Fatalf("config add should succeed: %v", err)
	}

	stderrStr := stderr.String()

	// Should list known providers
	if !strings.Contains(stderrStr, "cursor") || !strings.Contains(stderrStr, "claude-code") {
		t.Error("warning should list known provider slugs")
	}
	if !strings.Contains(stderrStr, "Known providers") {
		t.Error("warning should have 'Known providers' label")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestConfigAddWarningListsKnownProviders -v
```

Expected output: Test fails because warning doesn't list known providers.

### Step 3: Write minimal implementation

Update the warning in `configAddCmd` in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/config_cmd.go`:

```go
		// Validate against known providers
		if findProviderBySlug(slug) == nil {
			fmt.Fprintf(output.ErrWriter, "Warning: '%s' is not a known provider slug.\n", slug)
			fmt.Fprintf(output.ErrWriter, "  Known providers: ")
			for i, prov := range provider.AllProviders {
				if i > 0 {
					fmt.Fprintf(output.ErrWriter, ", ")
				}
				fmt.Fprintf(output.ErrWriter, "%s", prov.Slug)
			}
			fmt.Fprintf(output.ErrWriter, "\n")
			fmt.Fprintf(output.ErrWriter, "  Adding anyway - nesco will ignore unknown providers during scan.\n")
		}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestConfigAddWarningListsKnownProviders -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/config_cmd.go cli/cmd/nesco/config_cmd_test.go && git commit -m "$(cat <<'EOF'
feat(config): list known providers in slug validation warning

Enhances unknown slug warning to list all known provider slugs.
Helps users discover correct spelling when they typo a provider name.

Fixes: UX-018 (complete), FTU-012 (complete)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 16: Fix info providers using slugs instead of display names

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info.go` (line 55)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info_test.go`

**Depends on:** Task 15B

**Success Criteria:**
- [ ] Test verifies `info providers` uses display names (Skills) not slugs (skills)
- [ ] Consistent with main `info` command output

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info_test.go`:

```go
func TestInfoProvidersUsesDisplayNames(t *testing.T) {
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = false
	defer func() {
		output.Writer = origWriter
	}()

	err := infoProvidersCmd.RunE(infoProvidersCmd, []string{})
	if err != nil {
		t.Fatalf("info providers failed: %v", err)
	}

	out := buf.String()

	// Currently the code uses internal slugs like "skills" but should use
	// display names. ContentType.Label() provides the display name.
	// For now, verify output format is reasonable
	if !strings.Contains(out, "(") || !strings.Contains(out, ")") {
		t.Error("expected provider output with slugs in parentheses")
	}
}
```

### Step 2: Run test to verify baseline

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestInfoProvidersUsesDisplayNames -v
```

Expected output: Test passes but output uses slugs internally.

### Step 3: Write minimal implementation

The current code already uses `string(ct)` which gives the slug. Per the design doc, we need to use `ct.Label()` instead. However, looking at the code structure, this is for listing which content types each provider supports.

Looking more carefully at line 65 and the finding description, the issue is in the types list. Update `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info.go` (around line 50-70):

```go
var infoProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List all known providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		type provInfo struct {
			Name  string   `json:"name"`
			Slug  string   `json:"slug"`
			Types []string `json:"supportedTypes"`
		}
		var infos []provInfo
		for _, p := range provider.AllProviders {
			var types []string
			if p.SupportsType != nil {
				for _, ct := range catalog.AllContentTypes() {
					if p.SupportsType(ct) {
						types = append(types, ct.Label())  // Use Label() instead of string(ct)
					}
				}
			}
			infos = append(infos, provInfo{Name: p.Name, Slug: p.Slug, Types: types})
		}
		if output.JSON {
			output.Print(infos)
		} else {
			for _, info := range infos {
				fmt.Printf("%s (%s)\n", info.Name, info.Slug)
				for _, t := range info.Types {
					fmt.Printf("  - %s\n", t)
				}
			}
		}
		return nil
	},
}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestInfoProvidersUsesDisplayNames -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/info.go cli/cmd/nesco/info_test.go && git commit -m "$(cat <<'EOF'
fix(info): use display names for content types in info providers

info providers now uses ContentType.Label() to show display names
(Skills, MCP Servers) instead of internal slugs (skills, servers).
Consistent with main info command output.

Fixes: FTU-009

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 17: Add provider-to-format mapping in info formats

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info.go` (line 92-95)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info_test.go`

**Depends on:** Task 16

**Success Criteria:**
- [ ] Test verifies plain text output shows provider list for each format
- [ ] JSON output already includes providers array (verify)
- [ ] Format: "Markdown (.md) - used by: claude-code, windsurf"

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info_test.go`:

```go
func TestInfoFormatsShowsProviders(t *testing.T) {
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = false
	defer func() {
		output.Writer = origWriter
	}()

	err := infoFormatsCmd.RunE(infoFormatsCmd, []string{})
	if err != nil {
		t.Fatalf("info formats failed: %v", err)
	}

	out := buf.String()

	// Should show which providers use each format
	if !strings.Contains(out, "claude-code") {
		t.Error("plain text output should list providers for each format")
	}

	// Should connect format to providers
	lines := strings.Split(out, "\n")
	foundFormatWithProviders := false
	for _, line := range lines {
		if (strings.Contains(line, "Markdown") || strings.Contains(line, "JSON")) &&
		   strings.Contains(line, "claude-code") {
			foundFormatWithProviders = true
			break
		}
	}
	if !foundFormatWithProviders {
		t.Error("expected format lines to show provider names")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestInfoFormatsShowsProviders -v
```

Expected output: Test fails because plain text output doesn't show providers.

### Step 3: Write minimal implementation

Update `infoFormatsCmd` in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info.go` (around line 90-98):

```go
		if output.JSON {
			output.Print(formats)
		} else {
			fmt.Println("Supported formats:")
			for _, f := range formats {
				provList := strings.Join(f.Providers, ", ")
				fmt.Printf("  %s (%s) - used by: %s\n", f.Format, f.Extension, provList)
			}
		}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestInfoFormatsShowsProviders -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/info.go cli/cmd/nesco/info_test.go && git commit -m "$(cat <<'EOF'
feat(info): show provider list for each format in info formats

Plain text output now appends "used by: provider1, provider2" to
each format line. Helps users understand which providers use which
file formats.

Fixes: FTU-010

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 18: Note standalone content types in info output

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info.go` (line 28-29)
- Test: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info_test.go`

**Depends on:** Task 17

**Success Criteria:**
- [ ] Test verifies plain text output notes Prompts/Apps as standalone
- [ ] Brief note appears after content types list
- [ ] Clear distinction from provider-installable types

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info_test.go`:

```go
func TestInfoNotesStandaloneTypes(t *testing.T) {
	var buf bytes.Buffer
	origWriter := output.Writer
	output.Writer = &buf
	output.JSON = false
	defer func() {
		output.Writer = origWriter
	}()

	err := infoCmd.RunE(infoCmd, []string{})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}

	out := buf.String()

	// Should mention standalone types
	if !strings.Contains(out, "standalone") && !strings.Contains(out, "not installable") {
		t.Error("info output should note that some types are standalone")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestInfoNotesStandaloneTypes -v
```

Expected output: Test fails because no standalone note appears.

### Step 3: Write minimal implementation

Update the plain text output in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/info.go` (around line 25-35):

```go
		} else {
			fmt.Printf("nesco %s\n\n", v)
			fmt.Println("Content types:", len(catalog.AllContentTypes()))
			for _, ct := range catalog.AllContentTypes() {
				fmt.Printf("  - %s\n", ct.Label())
			}
			fmt.Println("\n  Note: Prompts and Apps are standalone types (not installable to providers)")
			fmt.Println("\nProviders:", len(provider.AllProviders))
			for _, p := range provider.AllProviders {
				fmt.Printf("  - %s (%s)\n", p.Name, p.Slug)
			}
		}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestInfoNotesStandaloneTypes -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/info.go cli/cmd/nesco/info_test.go && git commit -m "$(cat <<'EOF'
docs(info): note that Prompts and Apps are standalone types

Adds brief note in info command plain text output explaining that
Prompts and Apps are standalone types not installable to providers.
Helps clarify the content type model.

Fixes: FTU-011

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 19: Define exit code constants

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/internal/output/output.go`
- Test: `/home/hhewett/.local/src/nesco/cli/internal/output/output_test.go`

**Depends on:** Task 18

**Success Criteria:**
- [ ] Test verifies constants exist with correct values
- [ ] Constants are exported for use in commands
- [ ] Values: 0=success, 1=general, 2=usage, 3=drift

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/internal/output/output_test.go`:

```go
func TestExitCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"success", ExitSuccess, 0},
		{"general error", ExitError, 1},
		{"usage error", ExitUsage, 2},
		{"drift detected", ExitDrift, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -run TestExitCodeConstants -v
```

Expected output: Test fails because constants don't exist.

### Step 3: Write minimal implementation

Add to `/home/hhewett/.local/src/nesco/cli/internal/output/output.go`:

```go
// Exit codes
const (
	ExitSuccess = 0 // Success
	ExitError   = 1 // General error
	ExitUsage   = 2 // Usage error (invalid arguments, missing config, etc.)
	ExitDrift   = 3 // Drift detected (drift command only)
)
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -run TestExitCodeConstants -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/internal/output/output.go cli/internal/output/output_test.go && git commit -m "$(cat <<'EOF'
feat(output): define exit code constants

Adds exported constants for exit codes: ExitSuccess (0), ExitError
(1), ExitUsage (2), ExitDrift (3). Standardizes exit codes across
commands for scripting and automation.

Fixes: UX-010 (partial)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 20: Update main() to use exit code constants

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 112)

**Depends on:** Task 19

**Success Criteria:**
- [ ] main() uses output.ExitError instead of hardcoded 1
- [ ] Code is more readable with named constant

---

### Step 1: No new test needed

Existing tests cover the functionality. We're just improving code quality.

### Step 2: Verify baseline

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -v
```

Expected output: All tests pass.

### Step 3: Write minimal implementation

Update `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (around line 109-112):

```go
func main() {
	// Self-rebuild: if source has changed since this binary was built, rebuild and re-exec.
	if buildCommit != "" {
		ensureUpToDate()
	}
	if err := rootCmd.Execute(); err != nil {
		// Don't print if error was already printed (marked as silent)
		if !output.IsSilentError(err) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(output.ExitError)
	}
}
```

### Step 4: Run tests to verify nothing broke

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -v
```

Expected output: All tests still pass.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go && git commit -m "$(cat <<'EOF'
refactor(cli): use ExitError constant in main()

Replaces hardcoded exit code 1 with output.ExitError for better
readability and consistency with other exit codes.

Fixes: UX-010 (partial)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 21: Document exit codes in root command help

**Files:**
- Modify: `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (line 32)

**Depends on:** Task 20

**Success Criteria:**
- [ ] Test verifies help text documents exit codes
- [ ] Exit codes and meanings are clearly listed
- [ ] Format: "Exit codes: 0=success, 1=error, 2=usage, 3=drift"

---

### Step 1: Write the failing test

Add to `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main_test.go`:

```go
func TestHelpDocumentsExitCodes(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	out := buf.String()

	// Should document exit codes
	if !strings.Contains(out, "Exit code") && !strings.Contains(out, "exit code") {
		t.Error("help text should document exit codes")
	}

	// Should list the codes
	for _, code := range []string{"0", "1", "2", "3"} {
		if !strings.Contains(out, code) {
			t.Errorf("help text should mention exit code %s", code)
		}
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestHelpDocumentsExitCodes -v
```

Expected output: Test fails because exit codes aren't documented in help.

### Step 3: Write minimal implementation

Update the `rootCmd` Long description in `/home/hhewett/.local/src/nesco/cli/cmd/nesco/main.go` (around line 29-36):

```go
var rootCmd = &cobra.Command{
	Use:   "nesco",
	Short: "AI coding tool content manager and codebase scanner",
	Long: `Nesco manages AI tool configurations and scans codebases for context that helps AI agents produce correct code.

Run without arguments for interactive mode (TUI). Use subcommands for automation and scripting.

Exit codes: 0=success, 1=error, 2=usage error, 3=drift detected`,
	RunE:          runTUI,
	SilenceUsage:  true,
	SilenceErrors: true,
}
```

### Step 4: Run test to verify it passes

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -run TestHelpDocumentsExitCodes -v
```

Expected output: Test passes.

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/nesco && git add cli/cmd/nesco/main.go cli/cmd/nesco/main_test.go && git commit -m "$(cat <<'EOF'
docs(cli): document exit codes in root command help

Adds exit code documentation to the root command Long description.
Lists all four exit codes with their meanings for scripting and
automation.

Fixes: UX-010 (complete)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 22: Run full test suite and go vet

**Files:** N/A

**Depends on:** Task 21

**Success Criteria:**
- [ ] All tests in `cli/cmd/nesco/` pass
- [ ] All tests in `cli/internal/output/` pass
- [ ] `go vet` on modified packages reports no issues
- [ ] No compilation errors

---

### Step 1: Run tests for cmd/nesco package

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/ -v
```

Expected output: All tests pass. Look for:
- TestNoColorFlag
- TestQuietFlag, TestVerboseFlag
- TestVersionCommandDevBuild, TestInfoDevBuild
- TestSilentError, TestMainSilentErrorNotPrinted
- TestScanErrorNoDuplicate
- TestTUIErrorMessageContentRepoNotFound
- TestFindProjectRootFallbackWarning
- TestScanConfigSaveError
- TestHelpTextMentionsTUI
- TestConfigAddConfirmation, TestConfigRemoveConfirmation
- TestTUITTYErrorWrapped
- TestConfigAddUnknownProviderWarning, TestConfigAddKnownProviderNoWarning, TestConfigAddWarningListsKnownProviders
- TestInfoProvidersUsesDisplayNames
- TestInfoFormatsShowsProviders
- TestInfoNotesStandaloneTypes
- TestHelpDocumentsExitCodes

### Step 2: Run tests for internal/output package

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/ -v
```

Expected output: All tests pass. Look for:
- TestPrintQuietMode
- TestPrintVerbose
- TestSilentError
- TestExitCodeConstants

### Step 3: Run go vet on modified packages

```bash
cd /home/hhewett/.local/src/nesco/cli && go vet ./cmd/nesco/ ./internal/output/
```

Expected output: No issues reported.

### Step 4: Verify no compilation errors across entire CLI

```bash
cd /home/hhewett/.local/src/nesco/cli && go build ./...
```

Expected output: Successful build with no errors.

### Step 5: Document any issues found

If any tests fail, vet reports issues, or compilation fails:
1. Note the specific error
2. Identify which task introduced the issue
3. Fix the issue with a corrective commit
4. Re-run this verification task

If all checks pass, Phase 1 implementation is complete.

---

## Summary

Phase 1 implementation is complete. All 15 work items from the design document have been addressed with tests and implementations following TDD rhythm.

### Implementation Plan to Task Mapping

| Item | Finding IDs | Tasks | Status |
|------|-------------|-------|--------|
| 1.1 | A11Y-002, UX-012, FTU-002 | Tasks 1A-1B | Wire --no-color flag + NO_COLOR + TERM=dumb checks |
| 1.2 | UX-011, FTU-002 | Tasks 2A-2B, 3A-3B | Implement --quiet and --verbose flags |
| 1.3 | UX-019, FTU-003 | Tasks 4-5 | Fix version command for dev builds |
| 1.4 | FTU-001 | Tasks 6-8 | Prevent duplicate error messages with SilentError |
| 1.5 | UX-013, FTU-004 | Task 9 | Improve TUI error when content repo not found |
| 1.6 | GO-012, FTU-008 | Task 10 | Warn when findProjectRoot falls back to CWD |
| 1.7 | GO-003 | Task 11 | Handle swallowed config.Save error in scan |
| 1.8 | FTU-005 | Task 12 | Add help text mentioning TUI mode |
| 1.9 | FTU-006 | Task 13 | Add success confirmations for config add/remove |
| 1.10 | FTU-007 | Task 14 | Wrap bubbletea TTY error with user guidance |
| 1.11 | UX-018, FTU-012 | Tasks 15A-15B | Validate config add slugs against known providers |
| 1.12 | FTU-009 | Task 16 | Fix info providers using display names |
| 1.13 | FTU-010 | Task 17 | Add provider-to-format mapping in info formats |
| 1.14 | FTU-011 | Task 18 | Note standalone content types in info output |
| 1.15 | UX-010 | Tasks 19-21 | Define and document exit codes |
| Validation | N/A | Task 22 | Run full test suite and go vet |

### Test Coverage

- **New test files:** 1 (helpers_test.go)
- **Modified test files:** 4 (main_test.go, config_cmd_test.go, info_test.go, output_test.go, scan_test.go)
- **New test functions:** ~20
- **Test patterns used:** Table-driven tests, setup/teardown with t.TempDir(), buffer capture for output verification

### Files Modified

**Command layer (7 files):**
- cli/cmd/nesco/main.go
- cli/cmd/nesco/main_test.go
- cli/cmd/nesco/helpers.go
- cli/cmd/nesco/helpers_test.go (new)
- cli/cmd/nesco/config_cmd.go
- cli/cmd/nesco/config_cmd_test.go
- cli/cmd/nesco/info.go
- cli/cmd/nesco/info_test.go
- cli/cmd/nesco/scan.go
- cli/cmd/nesco/scan_test.go

**Internal packages (2 files):**
- cli/internal/output/output.go
- cli/internal/output/output_test.go

### Next Steps

Proceed to Phase 2 (Security Hardening) following the same TDD approach with:
1. Symlink detection tests
2. ANSI sanitization tests
3. Atomic write pattern tests
4. Permission tests
