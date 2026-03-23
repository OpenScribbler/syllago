package audit

import (
	"bytes"
	"encoding/json"
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
