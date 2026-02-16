package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

var (
	JSON      bool      // set from --json flag
	Writer    io.Writer = os.Stdout
	ErrWriter io.Writer = os.Stderr
)

func Print(v any) {
	if JSON {
		data, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(Writer, string(data))
	} else {
		fmt.Fprintln(Writer, v)
	}
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
