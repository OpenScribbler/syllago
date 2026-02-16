package output

import (
	"bytes"
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
