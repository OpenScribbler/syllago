package canonical

import (
	"bytes"
	"testing"
)

func TestNormalize_Empty(t *testing.T) {
	got := Normalize(nil)
	want := []byte{'\n'}
	if !bytes.Equal(got, want) {
		t.Fatalf("Normalize(nil) = %q, want %q", got, want)
	}
}
