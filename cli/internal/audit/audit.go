// Package audit provides structured JSON audit logging for content lifecycle events.
//
// Log format is JSON Lines (one JSON object per line), compatible with jq, Splunk,
// Datadog, ELK, and grep.
package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventType identifies the category of audit event.
type EventType string

// EventContentInstall records a content item being installed to a provider.
const EventContentInstall EventType = "content.install"

// Event is a single audit log entry.
type Event struct {
	Timestamp time.Time `json:"ts"`
	Version   int       `json:"version"`
	EventType EventType `json:"event_type"`

	// Content fields (for content.* events)
	ItemName string `json:"item_name,omitempty"`
	ItemType string `json:"item_type,omitempty"`
	Target   string `json:"target,omitempty"` // provider slug or registry name
}

// Logger writes audit events to a JSON Lines file.
type Logger struct {
	mu   sync.Mutex
	w    io.Writer
	file *os.File
}

// NewLogger creates a logger that writes to the given file path.
// Creates the file and parent directories if they don't exist.
// The file is opened in append mode.
func NewLogger(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating audit log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening audit log: %w", err)
	}
	return &Logger{w: f, file: f}, nil
}

// NewLoggerWriter creates a logger that writes to any io.Writer (useful for testing).
func NewLoggerWriter(w io.Writer) *Logger {
	return &Logger{w: w}
}

// Log writes an audit event as a JSON line.
func (l *Logger) Log(e Event) error {
	e.Version = 1
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshaling audit event: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, err = fmt.Fprintf(l.w, "%s\n", data)
	return err
}

// Close closes the underlying file if the logger owns it.
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// LogContent is a convenience method for logging content lifecycle events.
func (l *Logger) LogContent(eventType EventType, itemName, itemType, target string) error {
	return l.Log(Event{
		EventType: eventType,
		ItemName:  itemName,
		ItemType:  itemType,
		Target:    target,
	})
}

// DefaultLogPath returns the default audit log path within a project.
func DefaultLogPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".syllago", "audit.jsonl")
}
