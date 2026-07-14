package acif

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestMCPCanonicalizationPinnedHashes(t *testing.T) {
	t.Parallel()

	stdioDefault, err := CanonicalizeMCP(map[string]any{
		"servers": map[string]any{
			"demo": map[string]any{"command": "npx", "args": []any{"-y", "@demo/mcp-server"}},
		},
	})
	if err != nil {
		t.Fatalf("CanonicalizeMCP(stdio default): %v", err)
	}
	if stdioDefault.BodyHash != tvMCPStdio {
		t.Fatalf("stdio body_hash = %s, want %s", stdioDefault.BodyHash, tvMCPStdio)
	}
	if got := stdioDefault.Canonical["mcp"].(map[string]any)["servers"].(map[string]any)["demo"].(map[string]any)["type"]; got != "stdio" {
		t.Fatalf("stdio type = %#v", got)
	}

	stdioExplicit, err := CanonicalizeMCP(map[string]any{
		"servers": map[string]any{
			"demo": map[string]any{"type": "stdio", "command": "npx", "args": []any{"-y", "@demo/mcp-server"}},
		},
	})
	if err != nil {
		t.Fatalf("CanonicalizeMCP(stdio explicit): %v", err)
	}
	if stdioExplicit.BodyHash != stdioDefault.BodyHash || stdioExplicit.CanonicalBytes != stdioDefault.CanonicalBytes {
		t.Fatalf("explicit/default mismatch hash=%s/%s bytes=%s/%s", stdioExplicit.BodyHash, stdioDefault.BodyHash, stdioExplicit.CanonicalBytes, stdioDefault.CanonicalBytes)
	}

	http, err := CanonicalizeMCP(map[string]any{
		"servers": map[string]any{
			"demo": map[string]any{"url": "https://mcp.example.com/sse-endpoint"},
		},
	})
	if err != nil {
		t.Fatalf("CanonicalizeMCP(http): %v", err)
	}
	if http.BodyHash != tvMCPHTTP {
		t.Fatalf("http body_hash = %s, want %s", http.BodyHash, tvMCPHTTP)
	}
}

func TestMCPRejectsAndDiagnostics(t *testing.T) {
	t.Parallel()

	rejects := []struct {
		name string
		in   map[string]any
		id   string
	}{
		{"servers absent", map[string]any{}, ErrMCPServersMissing},
		{"servers empty", map[string]any{"servers": map[string]any{}}, ErrMCPServersMissing},
		{"invalid type", map[string]any{"servers": map[string]any{"demo": map[string]any{"type": "STDIO", "command": "npx"}}}, ErrMCPTransportTypeInvalid},
		{"ambiguous default", map[string]any{"servers": map[string]any{"demo": map[string]any{"command": "npx", "url": "https://example.test"}}}, ErrMCPTransportDefaultAmbiguous},
		{"undetermined default", map[string]any{"servers": map[string]any{"demo": map[string]any{"args": []any{"x"}}}}, ErrMCPTransportDefaultUndetermined},
	}
	for _, tc := range rejects {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := CanonicalizeMCP(tc.in)
			assertReject(t, err, tc.id)
		})
	}

	got, err := CanonicalizeMCP(map[string]any{"servers": map[string]any{"1demo": map[string]any{"command": "npx"}}})
	if err != nil {
		t.Fatalf("CanonicalizeMCP(unconventional): %v", err)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].ID != DiagMCPServerNameUnconventional {
		t.Fatalf("diagnostics = %#v", got.Diagnostics)
	}
}

func TestMCPDerivedCapabilitiesAndDeterminism(t *testing.T) {
	t.Parallel()

	rich := map[string]any{"mcp": map[string]any{"servers": map[string]any{
		"b": map[string]any{
			"command":       "npx",
			"args":          []any{"${PACKAGE}", "--flag"},
			"env":           map[string]any{"TOKEN": "${TOKEN}"},
			"includeTools":  []any{"z", "a", "a"},
			"excludeTools":  []any{"x"},
			"autoApprove":   []any{"tool"},
			"oauth":         map[string]any{"client_id": "id"},
			"disabledTools": []any{"d"},
		},
	}}}
	caps, err := DerivedCapabilitiesForItem(rich)
	if err != nil {
		t.Fatalf("DerivedCapabilitiesForItem(mcp): %v", err)
	}
	want := map[string]bool{
		"transport_types":   true,
		"oauth_support":     true,
		"env_var_expansion": true,
		"tool_filtering":    true,
		"auto_approve":      true,
	}
	if !reflect.DeepEqual(caps, want) {
		t.Fatalf("mcp caps = %#v, want %#v", caps, want)
	}

	a, err := CanonicalizeMCP(map[string]any{"servers": map[string]any{
		"z": map[string]any{"command": "npx", "includeTools": []any{"b", "a", "a"}},
		"a": map[string]any{"url": "https://example.test"},
	}})
	if err != nil {
		t.Fatalf("CanonicalizeMCP(a): %v", err)
	}
	b, err := CanonicalizeMCP(map[string]any{"servers": map[string]any{
		"a": map[string]any{"url": "https://example.test"},
		"z": map[string]any{"includeTools": []any{"a", "b"}, "command": "npx"},
	}})
	if err != nil {
		t.Fatalf("CanonicalizeMCP(b): %v", err)
	}
	if a.CanonicalBytes != b.CanonicalBytes || a.BodyHash != b.BodyHash {
		t.Fatalf("canonicalization not deterministic:\n%s\n%s\n%s\n%s", a.CanonicalBytes, b.CanonicalBytes, a.BodyHash, b.BodyHash)
	}
	if !strings.Contains(a.CanonicalBytes, `"includeTools":["a","b"]`) {
		t.Fatalf("filter list was not sorted/deduped: %s", a.CanonicalBytes)
	}
}

func TestMCPStripAndRestoreRoundTrip(t *testing.T) {
	t.Parallel()

	item, err := CanonicalizeMCP(map[string]any{"servers": map[string]any{"demo": map[string]any{"command": "npx"}}})
	if err != nil {
		t.Fatalf("CanonicalizeMCP(): %v", err)
	}
	rendered, err := RenderMCP(item.Canonical, "no-type-format")
	if err != nil {
		t.Fatalf("RenderMCP(no-type-format): %v", err)
	}
	if strings.Contains(rendered.Output, `"type"`) {
		t.Fatalf("rendered output retained type: %s", rendered.Output)
	}
	var bare map[string]any
	if err := json.Unmarshal([]byte(rendered.Output), &bare); err != nil {
		t.Fatalf("render output is not JSON: %v", err)
	}
	roundtrip, err := CanonicalizeMCPProviderConfig(map[string]any{
		"provider": "no-type-format",
		"content":  rendered.Output,
	})
	if err != nil {
		t.Fatalf("CanonicalizeMCPProviderConfig(): %v", err)
	}
	if roundtrip.CanonicalBytes != item.CanonicalBytes || roundtrip.BodyHash != item.BodyHash {
		t.Fatalf("roundtrip mismatch:\n%s\n%s\n%s\n%s", roundtrip.CanonicalBytes, item.CanonicalBytes, roundtrip.BodyHash, item.BodyHash)
	}
}
