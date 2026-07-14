package acif

import (
	"encoding/json"
	"testing"
)

func TestMetadataHashVectors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		section   string
		canonical string
		hash      string
	}{
		{
			name:      "TV-9 command",
			section:   `{"kind":"command","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Review PR","version":"1.2.0"}`,
			canonical: `{"display_name":"Review PR","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","kind":"command","version":"1.2.0"}`,
			hash:      "ceb0cf9212c530e85444020aeb3cbae8865fdc16d91ee63fe6f5cb374d67b5c6",
		},
		{
			name:      "TV-2 skill",
			section:   `{"kind":"skill","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Demo Skill"}`,
			canonical: `{"display_name":"Demo Skill","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","kind":"skill"}`,
			hash:      "b68bf2e4cbd6b9123d23de684498157adeaf4915e65435a95a556ac27ec50316",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hash, canonical, err := MetadataHash(json.RawMessage(tc.section))
			if err != nil {
				t.Fatalf("MetadataHash() error: %v", err)
			}
			if string(canonical) != tc.canonical {
				t.Fatalf("canonical = %s, want %s", canonical, tc.canonical)
			}
			if hash != tc.hash {
				t.Fatalf("hash = %s, want %s", hash, tc.hash)
			}
		})
	}
}

func TestCanonicalJSONUsesRawBytes(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage([]byte("{\n  \"b\":2,\n  \"a\":1\n}"))
	got, err := CanonicalJSON(raw)
	if err != nil {
		t.Fatalf("CanonicalJSON() error: %v", err)
	}
	if want := `{"a":1,"b":2}`; string(got) != want {
		t.Fatalf("CanonicalJSON() = %s, want %s", got, want)
	}
}
