package converter

// Pluggable hook scanner infrastructure.
//
// The existing ScanHookSecurity / ScanScript functions are internal helpers.
// This file exposes them through a stable HookScanner interface so enterprises
// can chain external tools (Semgrep, ShellCheck, custom AST scanners) into
// the install pipeline.
//
// Design is documented in docs/plans/implementation/pluggable-scanner.md.
// Key properties:
//   - External scanners are subprocesses (universal, language-agnostic), not
//     Go plugins. Fragile plugin packages are avoided.
//   - Scanner errors are recorded in ScanResult.Errors, not returned as hard
//     errors. One broken external scanner must not discard findings from the
//     builtin scanner.
//   - Findings are tagged with the scanner that produced them so downstream
//     UI can attribute results accurately.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ScanFinding represents a single security concern found by a scanner.
// This is the public output type for the pluggable scanner interface.
// It replaces SecurityWarning as the unified result type, adding File, Line,
// and Scanner fields for actionable output.
type ScanFinding struct {
	Severity    string `json:"severity"`    // "high", "medium", "low", "info"
	File        string `json:"file"`        // path to the file containing the finding (relative to hook dir)
	Line        int    `json:"line"`        // line number (0 if not applicable, e.g. manifest-level findings)
	Description string `json:"description"` // human-readable explanation of the concern
	Scanner     string `json:"scanner"`     // name of the scanner that produced this finding (e.g. "builtin", "semgrep")
}

// ScanResult is the aggregated output from one or more scanners.
type ScanResult struct {
	Findings []ScanFinding `json:"findings"`
	Errors   []string      `json:"errors"` // non-fatal scanner errors (e.g. "semgrep not found in PATH")
}

// HookScanner analyzes a hook directory and returns security findings.
//
// hookDir is the absolute path to the hook directory containing hook.json
// and any referenced script files. The scanner must not modify the directory.
type HookScanner interface {
	Name() string
	Scan(hookDir string) (ScanResult, error)
}

// BuiltinScanner wraps the existing regex-based ScanHookSecurity function
// and extends it with script file scanning. It implements HookScanner.
type BuiltinScanner struct{}

// Name returns the scanner identifier.
func (s *BuiltinScanner) Name() string { return "builtin" }

// Scan reads hook.json from the directory, scans it with the existing regex
// patterns, and also scans any recognized script files (.sh, .bash, .zsh,
// .py, .js, .mjs, .cjs, .ts, .rb, .ps1) in the directory. Each finding is
// tagged with its source file and, for scripts, the line number.
func (s *BuiltinScanner) Scan(hookDir string) (ScanResult, error) {
	var result ScanResult

	hookPath := filepath.Join(hookDir, "hook.json")
	if data, err := os.ReadFile(hookPath); err == nil {
		for _, w := range ScanHookSecurity(data) {
			result.Findings = append(result.Findings, ScanFinding{
				Severity:    w.Severity,
				File:        "hook.json",
				Line:        0,
				Description: w.Description,
				Scanner:     s.Name(),
			})
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		// Missing hook.json is expected for some directory layouts; surface
		// anything else (permission errors, IO failures) as a scanner error
		// rather than a hard failure so other scanners can still run.
		result.Errors = append(result.Errors, fmt.Sprintf("read hook.json: %v", err))
	}

	entries, err := os.ReadDir(hookDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("read hook dir: %v", err))
		return result, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		lang := DetectLanguage(entry.Name())
		if lang == LangUnknown {
			continue
		}
		path := filepath.Join(hookDir, entry.Name())
		findings, scanErr := scanScriptFileWithLines(path, entry.Name(), lang, s.Name())
		if scanErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", entry.Name(), scanErr))
			continue
		}
		result.Findings = append(result.Findings, findings...)
	}

	return result, nil
}

// scanScriptFileWithLines reads a script file and runs the universal shell
// patterns plus any language-specific overlay against each line. Returns
// findings tagged with file name and 1-indexed line number.
func scanScriptFileWithLines(path, fileName, language, scanner string) ([]ScanFinding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", fileName, err)
	}

	var findings []ScanFinding
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 10*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()

		for _, dp := range shellPatterns {
			if dp.pattern.MatchString(line) {
				findings = append(findings, ScanFinding{
					Severity:    dp.severity,
					File:        fileName,
					Line:        lineNo,
					Description: dp.description,
					Scanner:     scanner,
				})
			}
		}

		if language != LangUnknown && language != LangShell {
			if specific, ok := languagePatterns[language]; ok {
				for _, dp := range specific {
					if dp.pattern.MatchString(line) {
						findings = append(findings, ScanFinding{
							Severity:    dp.severity,
							File:        fileName,
							Line:        lineNo,
							Description: dp.description,
							Scanner:     scanner,
						})
					}
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return findings, fmt.Errorf("scan %s: %w", fileName, err)
	}
	return findings, nil
}

// ExternalScanner runs an external executable (e.g. Semgrep, ShellCheck) against
// a hook directory. The executable must accept a directory path as its first
// argument and output ScanResult JSON on stdout.
//
// Protocol:
//
//	Exit 0 — scan completed. Stdout is ScanResult JSON.
//	Exit 1 — scan completed with findings. Stdout is ScanResult JSON.
//	Exit 2+ — scanner error. Stderr captured into ScanResult.Errors.
//
// Timeout: Timeout (default 30s) after which the subprocess is killed.
type ExternalScanner struct {
	Path    string
	Timeout time.Duration
}

// Name returns the scanner executable base name — used to tag findings.
func (s *ExternalScanner) Name() string {
	return filepath.Base(s.Path)
}

// Scan executes the scanner subprocess, enforces a timeout, and parses its
// JSON output. Any error (missing binary, exit code ≥2, invalid JSON,
// timeout) is recorded in ScanResult.Errors rather than returned — the
// chain runner decides policy.
func (s *ExternalScanner) Scan(hookDir string) (ScanResult, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	var result ScanResult

	// Pre-check: if the path doesn't exist, `cmd.Run()` returns a wrapped
	// PathError whose exact type depends on Go version. Stat-first gives us
	// a deterministic "missing binary" branch across platforms.
	if _, err := os.Stat(s.Path); err != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("scanner %s: %v", s.Name(), err))
		return result, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.Path, hookDir)
	// WaitDelay bounds how long cmd.Wait blocks on pipe closure after the
	// context kills the process. Without it, a grandchild (e.g. sleep) that
	// inherits the stdout pipe can keep Wait blocked even after the main
	// process has been killed.
	cmd.WaitDelay = 2 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Errors = append(result.Errors,
			fmt.Sprintf("scanner %s: timeout after %s", s.Name(), timeout))
		return result, nil
	}

	switch {
	case runErr == nil:
		// Exit 0 — fall through to stdout parsing.
	case exitCode == 1:
		// Protocol-valid "findings present" exit — fall through.
	case exitCode >= 2:
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("exit %d", exitCode)
		}
		result.Errors = append(result.Errors,
			fmt.Sprintf("scanner %s: %s", s.Name(), msg))
		return result, nil
	default:
		// Process failed to start, killed by signal, or any other non-
		// exit-code condition. Record and bail.
		result.Errors = append(result.Errors,
			fmt.Sprintf("scanner %s: %v", s.Name(), runErr))
		return result, nil
	}

	out := bytes.TrimSpace(stdout.Bytes())
	if len(out) == 0 {
		return result, nil
	}
	var parsed ScanResult
	if err := json.Unmarshal(out, &parsed); err != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("scanner %s: invalid JSON output: %v", s.Name(), err))
		return result, nil
	}
	for i := range parsed.Findings {
		parsed.Findings[i].Scanner = s.Name()
	}
	result.Findings = append(result.Findings, parsed.Findings...)
	result.Errors = append(result.Errors, parsed.Errors...)
	return result, nil
}

// chainedScanner runs a sequence of scanners, merging their results. Fail-open
// per scanner: a hard error from one scanner is recorded in Errors and the
// chain continues. This protects findings from earlier scanners against
// later failures.
type chainedScanner struct {
	scanners []HookScanner
}

// Name identifies the chain as a whole.
func (c *chainedScanner) Name() string { return "chain" }

// Scan runs each scanner in order and merges the results.
func (c *chainedScanner) Scan(hookDir string) (ScanResult, error) {
	var merged ScanResult
	for _, s := range c.scanners {
		res, err := s.Scan(hookDir)
		if err != nil {
			merged.Errors = append(merged.Errors,
				fmt.Sprintf("scanner %s: %v", s.Name(), err))
		}
		merged.Findings = append(merged.Findings, res.Findings...)
		merged.Errors = append(merged.Errors, res.Errors...)
	}
	return merged, nil
}

// ChainScanners returns a HookScanner that runs all given scanners
// sequentially and merges their findings. If no scanners are supplied,
// returns a chain that reports empty results.
func ChainScanners(scanners ...HookScanner) HookScanner {
	return &chainedScanner{scanners: scanners}
}

// RunScanChain is the install-flow entry point. It constructs a scanner chain
// consisting of the builtin scanner followed by any external scanners passed
// by path, runs them against hookDir, and returns the merged result.
//
// Builtin always runs first — it is fast and has no external dependencies,
// so downstream policy can count on at least having builtin findings even
// if every external scanner breaks.
func RunScanChain(hookDir string, externalScannerPaths []string) (ScanResult, error) {
	scanners := []HookScanner{&BuiltinScanner{}}
	for _, path := range externalScannerPaths {
		if path == "" {
			continue
		}
		scanners = append(scanners, &ExternalScanner{Path: path})
	}
	return ChainScanners(scanners...).Scan(hookDir)
}

// HighestSeverity returns the most severe severity level in a finding list
// according to the order high > medium > low > info. Returns "" for empty.
func HighestSeverity(findings []ScanFinding) string {
	rank := map[string]int{"high": 4, "medium": 3, "low": 2, "info": 1}
	best := ""
	bestRank := 0
	for _, f := range findings {
		r := rank[strings.ToLower(f.Severity)]
		if r > bestRank {
			best = strings.ToLower(f.Severity)
			bestRank = r
		}
	}
	return best
}
