package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Exit codes for consistent scripting behavior.
const (
	ExitSuccess = 0 // Success
	ExitError   = 1 // General error
	ExitUsage   = 2 // Usage error (invalid arguments, missing config, etc.)
	ExitDrift   = 3 // Drift detected (drift command only)
)

var (
	JSON      bool      // set from --json flag
	Quiet     bool      // set from --quiet flag
	Verbose   bool      // set from --verbose flag
	Writer    io.Writer = os.Stdout
	ErrWriter io.Writer = os.Stderr
)

// SetForTest saves current global state and returns test-safe writers.
// It registers a cleanup function via t.Cleanup to restore all globals
// after the test completes (even if the test panics).
//
// Returns (stdout, stderr) buffers that are also set as Writer/ErrWriter.
func SetForTest(t interface{ Cleanup(func()) }) (stdout, stderr *bytes.Buffer) {
	savedJSON := JSON
	savedQuiet := Quiet
	savedVerbose := Verbose
	savedWriter := Writer
	savedErrWriter := ErrWriter

	t.Cleanup(func() {
		JSON = savedJSON
		Quiet = savedQuiet
		Verbose = savedVerbose
		Writer = savedWriter
		ErrWriter = savedErrWriter
	})

	JSON = false
	Quiet = false
	Verbose = false
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	Writer = stdout
	ErrWriter = stderr

	return stdout, stderr
}

func Print(v any) {
	if Quiet {
		return
	}
	if JSON {
		data, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(Writer, string(data))
	} else {
		fmt.Fprintln(Writer, v)
	}
}

// PrintVerbose prints only when Verbose is true.
func PrintVerbose(format string, args ...any) {
	if !Verbose {
		return
	}
	fmt.Fprintf(Writer, format, args...)
}

// silentError wraps an error to signal that it has already been printed
// and should not be printed again by the main error handler.
type silentError struct {
	err error
}

func (e silentError) Error() string { return e.err.Error() }
func (e silentError) Unwrap() error { return e.err }

// SilentError wraps an error to mark it as already printed.
func SilentError(err error) error {
	if err == nil {
		return nil
	}
	return silentError{err: err}
}

// IsSilentError checks if an error is marked as already printed.
func IsSilentError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(silentError)
	return ok
}

type ErrorResponse struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

func PrintError(code int, message, suggestion string) {
	if JSON {
		data, _ := json.MarshalIndent(ErrorResponse{
			Code: code, Message: message, Suggestion: suggestion,
		}, "", "  ")
		fmt.Fprintln(ErrWriter, string(data))
	} else {
		fmt.Fprintf(ErrWriter, "Error: %s\n", message)
		if suggestion != "" {
			fmt.Fprintf(ErrWriter, "  Suggestion: %s\n", suggestion)
		}
	}
}

// StructuredError is a machine-readable error with a namespaced code.
type StructuredError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	DocsURL    string `json:"docs_url,omitempty"`
	Details    string `json:"details,omitempty"`
}

// Error implements the error interface so StructuredError works with errors.As().
func (e StructuredError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// PrintStructuredError prints a StructuredError to ErrWriter.
// In JSON mode, prints structured JSON. In plain mode, prints human-readable format.
func PrintStructuredError(e StructuredError) {
	if JSON {
		data, _ := json.MarshalIndent(e, "", "  ")
		fmt.Fprintln(ErrWriter, string(data))
		return
	}
	fmt.Fprintf(ErrWriter, "Error [%s]: %s\n", e.Code, e.Message)
	if e.Suggestion != "" {
		fmt.Fprintf(ErrWriter, "  Suggestion: %s\n", e.Suggestion)
	}
	if e.DocsURL != "" {
		fmt.Fprintf(ErrWriter, "  Docs: %s\n", e.DocsURL)
	}
	if e.Details != "" {
		// Indent each detail line for readability.
		for _, line := range strings.Split(e.Details, "\n") {
			fmt.Fprintf(ErrWriter, "  %s\n", line)
		}
	}
	if e.Code != "" {
		fmt.Fprintf(ErrWriter, "  Run 'syllago explain %s' for details\n", e.Code)
	}
}
