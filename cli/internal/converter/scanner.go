package converter

// ScanFinding represents a single security concern found by a scanner.
// This is the public output type for the pluggable scanner interface.
// It replaces SecurityWarning as the unified result type, adding File, Line,
// and Scanner fields for actionable output.
type ScanFinding struct {
	Severity    string // "high", "medium", "low", "info"
	File        string // path to the file containing the finding (relative to hook dir)
	Line        int    // line number (0 if not applicable, e.g. manifest-level findings)
	Description string // human-readable explanation of the concern
	Scanner     string // name of the scanner that produced this finding (e.g. "builtin", "semgrep")
}

// ScanResult is the aggregated output from one or more scanners.
type ScanResult struct {
	Findings []ScanFinding
	Errors   []string // non-fatal scanner errors (e.g. "semgrep not found in PATH")
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
// patterns, and also scans any .sh/.ps1/.py/.bash script files found in the
// directory. Returns findings with file paths and line numbers.
func (s *BuiltinScanner) Scan(hookDir string) (ScanResult, error) {
	// TODO: Implement directory-based scanning.
	// Phase 1: Read hook.json, call ScanHookSecurity, convert SecurityWarning → ScanFinding
	// Phase 2: Walk directory for script files, scan each line against dangerPatterns
	return ScanResult{}, nil
}

// ExternalScanner runs an external executable (e.g. Semgrep, ShellCheck) against
// a hook directory. The executable must accept a directory path as its first argument
// and output JSON findings on stdout.
type ExternalScanner struct {
	// Path is the absolute path to the scanner executable.
	Path string
}

// Name returns the scanner executable base name.
func (s *ExternalScanner) Name() string { return s.Path }

// Scan runs the external scanner and parses its output.
func (s *ExternalScanner) Scan(hookDir string) (ScanResult, error) {
	// TODO: Implement subprocess execution + JSON output parsing.
	return ScanResult{}, nil
}
