package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
