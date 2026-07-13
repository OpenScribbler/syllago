package capfeed

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadMarker_MissingAndTolerant(t *testing.T) {
	t.Run("missing file is a zero marker, not an error (first run)", func(t *testing.T) {
		m, err := ReadMarker(filepath.Join(t.TempDir(), "provenance.json"))
		if err != nil {
			t.Fatalf("ReadMarker on a missing file: %v; want nil error", err)
		}
		if m.DataRevision != "" {
			t.Errorf("zero marker DataRevision = %q; want empty", m.DataRevision)
		}
	})

	t.Run("extra JSON keys are ignored", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "provenance.json")
		body := `{"data_revision": "rev-9", "generated_at": "2026-07-12T20:41:41Z", "future_key": {"x": 1}}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := ReadMarker(path)
		if err != nil {
			t.Fatalf("ReadMarker: %v", err)
		}
		if m.DataRevision != "rev-9" {
			t.Errorf("DataRevision = %q; want rev-9", m.DataRevision)
		}
		want := time.Date(2026, 7, 12, 20, 41, 41, 0, time.UTC)
		if !m.GeneratedAt.Equal(want) {
			t.Errorf("GeneratedAt = %v; want %v", m.GeneratedAt, want)
		}
	})

	t.Run("malformed JSON is an error, not a silent zero marker", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "provenance.json")
		if err := os.WriteFile(path, []byte(`{"data_revision": `), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadMarker(path); err == nil {
			t.Fatal("ReadMarker on corrupt JSON returned nil error; a corrupt marker must surface, not silently re-pull")
		}
	})
}
