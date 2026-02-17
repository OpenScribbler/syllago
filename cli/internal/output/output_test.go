package output

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	Writer = &buf
	JSON = true
	defer func() { JSON = false; Writer = os.Stdout }()

	Print(map[string]string{"key": "value"})
	if !strings.Contains(buf.String(), `"key": "value"`) {
		t.Errorf("JSON output missing expected content: %s", buf.String())
	}
}

func TestPrintHuman(t *testing.T) {
	var buf bytes.Buffer
	Writer = &buf
	JSON = false
	defer func() { Writer = os.Stdout }()

	Print("hello world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("human output missing expected content: %s", buf.String())
	}
}

func TestPrintErrorJSON(t *testing.T) {
	var buf bytes.Buffer
	ErrWriter = &buf
	JSON = true
	defer func() { JSON = false; ErrWriter = os.Stderr }()

	PrintError(1, "something broke", "try again")
	out := buf.String()
	if !strings.Contains(out, `"code": 1`) {
		t.Errorf("JSON error missing code: %s", out)
	}
	if !strings.Contains(out, `"message": "something broke"`) {
		t.Errorf("JSON error missing message: %s", out)
	}
	if !strings.Contains(out, `"suggestion": "try again"`) {
		t.Errorf("JSON error missing suggestion: %s", out)
	}
}

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

	// Nil input should return nil
	if SilentError(nil) != nil {
		t.Error("SilentError(nil) should return nil")
	}
	if IsSilentError(nil) {
		t.Error("IsSilentError(nil) should return false")
	}
}

func TestPrintErrorHuman(t *testing.T) {
	var buf bytes.Buffer
	ErrWriter = &buf
	JSON = false
	defer func() { ErrWriter = os.Stderr }()

	PrintError(1, "something broke", "try again")
	out := buf.String()
	if !strings.Contains(out, "Error: something broke") {
		t.Errorf("human error missing message: %s", out)
	}
	if !strings.Contains(out, "Suggestion: try again") {
		t.Errorf("human error missing suggestion: %s", out)
	}
}
