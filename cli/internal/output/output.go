package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
