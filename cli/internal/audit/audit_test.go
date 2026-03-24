package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLogger_Log(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWriter(&buf)

	err := logger.Log(Event{
		EventType: EventHookInstall,
		HookName:  "safety-check",
		HookEvent: "before_tool_execute",
		Provider:  "claude-code",
		Source:    "export",
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var event Event
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if event.Version != 1 {
		t.Errorf("expected version 1, got %d", event.Version)
	}
	if event.EventType != EventHookInstall {
		t.Errorf("expected event_type %q, got %q", EventHookInstall, event.EventType)
	}
	if event.HookName != "safety-check" {
		t.Errorf("expected hook_name %q, got %q", "safety-check", event.HookName)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestLogger_MultipleEvents(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWriter(&buf)

	logger.Log(Event{EventType: EventHookInstall, HookName: "hook-1"})
	logger.Log(Event{EventType: EventHookScan, HookName: "hook-1", ScanResult: "pass"})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestNewLogger_CreatesFileAndDirectories(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/subdir/nested/audit.jsonl"

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	// Log an event through the file-backed logger.
	err = logger.Log(Event{
		EventType: EventHookInstall,
		HookName:  "test-hook",
		HookEvent: "before_tool_execute",
		Provider:  "claude-code",
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Close the logger.
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify the file exists and contains valid JSON.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	line := strings.TrimSpace(string(data))
	var event Event
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if event.HookName != "test-hook" {
		t.Errorf("expected hook_name %q, got %q", "test-hook", event.HookName)
	}
	if event.Version != 1 {
		t.Errorf("expected version 1, got %d", event.Version)
	}
}

func TestNewLogger_AppendMode(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/audit.jsonl"

	// Write first entry.
	logger1, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger (1st): %v", err)
	}
	logger1.Log(Event{EventType: EventHookInstall, HookName: "hook-a"})
	logger1.Close()

	// Open again and write second entry — should append, not overwrite.
	logger2, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger (2nd): %v", err)
	}
	logger2.Log(Event{EventType: EventHookScan, HookName: "hook-b"})
	logger2.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (append mode), got %d", len(lines))
	}
}

func TestClose_NilFile(t *testing.T) {
	// A logger created via NewLoggerWriter has no underlying file.
	var buf bytes.Buffer
	logger := NewLoggerWriter(&buf)
	if err := logger.Close(); err != nil {
		t.Errorf("Close on writer-based logger should return nil, got: %v", err)
	}
}

func TestDefaultLogPath(t *testing.T) {
	got := DefaultLogPath("/home/user/project")
	want := "/home/user/project/.syllago/audit.jsonl"
	if got != want {
		t.Errorf("DefaultLogPath: got %q, want %q", got, want)
	}
}
