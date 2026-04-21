package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errFakeDiskFull is a sentinel returned by failingWriter to simulate a
// write-side I/O failure (disk full, closed pipe, broken network writer).
// It lets TestLogger_Log_PropagatesWriterError assert that Log surfaces the
// exact underlying cause rather than silently swallowing it.
var errFakeDiskFull = errors.New("fake: no space left on device")

// failingWriter is an io.Writer that always fails. Used to exercise the
// error-propagation path inside Logger.Log without having to actually
// exhaust disk space or simulate platform-specific I/O failures.
type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errFakeDiskFull
}

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

func TestLogContent(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWriter(&buf)

	err := logger.LogContent(EventContentInstall, "my-skill", "skills", "claude-code")
	if err != nil {
		t.Fatalf("LogContent: %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var event Event
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if event.EventType != EventContentInstall {
		t.Errorf("expected event_type %q, got %q", EventContentInstall, event.EventType)
	}
	if event.ItemName != "my-skill" {
		t.Errorf("expected item_name %q, got %q", "my-skill", event.ItemName)
	}
	if event.ItemType != "skills" {
		t.Errorf("expected item_type %q, got %q", "skills", event.ItemType)
	}
	if event.Target != "claude-code" {
		t.Errorf("expected target %q, got %q", "claude-code", event.Target)
	}
}

func TestContentEventTypes(t *testing.T) {
	// Verify all content event types are distinct and non-empty
	types := []EventType{EventContentAdd, EventContentInstall, EventContentRemove, EventContentShare}
	seen := make(map[EventType]bool)
	for _, et := range types {
		if et == "" {
			t.Error("empty event type")
		}
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
	}
}

func TestDefaultLogPath(t *testing.T) {
	got := DefaultLogPath("/home/user/project")
	want := "/home/user/project/.syllago/audit.jsonl"
	if got != want {
		t.Errorf("DefaultLogPath: got %q, want %q", got, want)
	}
}

// TestNewLogger_ParentPathIsFile ensures NewLogger surfaces a directory-
// creation failure when the parent of the requested log path is already a
// regular file. Without this coverage, a misconfigured audit-log path could
// silently fail at runtime and the operator would only notice when events
// stopped appearing. Uses the "file-as-dir" trick so it works regardless of
// test privilege level (chmod-based tests no-op when running as root).
func TestNewLogger_ParentPathIsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a regular file where NewLogger expects a directory.
	parentAsFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(parentAsFile, []byte("x"), 0644); err != nil {
		t.Fatalf("setup: writing parent-as-file: %v", err)
	}
	logPath := filepath.Join(parentAsFile, "audit.jsonl")

	logger, err := NewLogger(logPath)
	if err == nil {
		_ = logger.Close()
		t.Fatal("regression: NewLogger must fail when the parent path is a regular file — returning nil error here means we silently swallow a fatal misconfiguration")
	}
	if !strings.Contains(err.Error(), "creating audit log directory") {
		t.Errorf("regression: directory-creation failures must be wrapped with 'creating audit log directory' so operators can tell the two NewLogger error paths apart; got: %v", err)
	}
}

// TestNewLogger_PathIsDirectory ensures NewLogger surfaces a file-open
// failure distinctly from a directory-creation failure. The failure modes
// come from different syscalls and operators need to tell them apart when
// diagnosing broken audit pipelines. Uses a pre-existing directory at the
// log-file path so the MkdirAll call succeeds (no-op on existing dir) and
// only the OpenFile call fails.
func TestNewLogger_PathIsDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	if err := os.MkdirAll(logPath, 0755); err != nil {
		t.Fatalf("setup: creating dir at log path: %v", err)
	}

	logger, err := NewLogger(logPath)
	if err == nil {
		_ = logger.Close()
		t.Fatal("regression: NewLogger must fail when the log path is a directory — opening a directory for write should not silently succeed")
	}
	if !strings.Contains(err.Error(), "opening audit log") {
		t.Errorf("regression: file-open failures must be wrapped with 'opening audit log' so they are distinguishable from directory-creation failures; got: %v", err)
	}
}

// TestLogger_LogAfterClose_ReturnsErrorDoesNotPanic guards the lifecycle
// contract: Log() on a Logger whose file has been closed must return an
// error, not panic. Hook handlers and background goroutines frequently
// outlive the Logger, so a panic here would surface as an opaque runtime
// crash in production rather than a recoverable error. The defer/recover
// turns a silent regression (panic) into a visible one (test fatal).
func TestLogger_LogAfterClose_ReturnsErrorDoesNotPanic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("setup: NewLogger: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("setup: Close: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("regression: Log() after Close() must not panic — hook handlers and async goroutines can race Close() and a panic here would crash the whole process; got panic: %v", r)
		}
	}()

	err = logger.Log(Event{EventType: EventHookInstall, HookName: "post-close-hook"})
	if err == nil {
		t.Error("regression: Log() after Close() must return a non-nil error — returning nil here means the caller assumes the event was persisted when it was not, breaking the audit trail")
	}
}

// TestLogger_Log_PropagatesWriterError stands in for the disk-full case:
// any io.Writer that fails mid-log must propagate its error up to the
// caller so hook flows can decide whether to abort. A silent swallow here
// would cause audit events to vanish without the caller knowing. Using a
// deterministic failingWriter keeps the test portable — simulating real
// disk exhaustion is not practical inside `go test`.
func TestLogger_Log_PropagatesWriterError(t *testing.T) {
	t.Parallel()

	logger := NewLoggerWriter(failingWriter{})
	err := logger.Log(Event{EventType: EventHookInstall, HookName: "hook-x"})
	if err == nil {
		t.Fatal("regression: Log() must return the underlying writer error — silently swallowing it would cause audit events to disappear without the caller knowing")
	}
	if !errors.Is(err, errFakeDiskFull) {
		t.Errorf("regression: Log() must wrap or return the underlying writer error unchanged (errors.Is must match) — otherwise callers cannot programmatically distinguish disk-full from malformed-event; got: %v", err)
	}
}

func TestLogger_SignalTrace(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := NewLoggerWriter(&buf)
	err := l.Log(Event{
		EventType:               EventContentSignalClassify,
		ItemName:                "redteam",
		ItemType:                "skills",
		ContentSignalFile:       "Packs/redteam/SKILL.md",
		ContentSignalConfidence: 0.65,
		ContentSignalBucket:     "confirm",
		ContentSignalSource:     "content-signal",
		ContentSignalStaticSignals: []SignalTrace{
			{Signal: "filename_SKILL.md", Weight: 0.25},
			{Signal: "directory_keyword_pack", Weight: 0.10},
		},
	})
	if err != nil {
		t.Fatalf("Log error: %v", err)
	}
	line := buf.String()
	if !strings.Contains(line, `"event_type":"content-signal.classify"`) {
		t.Errorf("missing event_type in: %s", line)
	}
	if !strings.Contains(line, `"filename_SKILL.md"`) {
		t.Errorf("missing signal name in: %s", line)
	}
	if !strings.Contains(line, `"confidence":0.65`) {
		t.Errorf("missing confidence in: %s", line)
	}
}
