package capmon_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestValidateContentResponse_Valid(t *testing.T) {
	t.Parallel()
	body := []byte(strings.Repeat("a", 1000))
	err := capmon.ValidateContentResponse(body, "text/html; charset=utf-8", "https://docs.example.com/page", "https://docs.example.com/page")
	if err != nil {
		t.Errorf("expected valid response to pass, got: %v", err)
	}
}

func TestValidateContentResponse_TooSmall(t *testing.T) {
	t.Parallel()
	body := []byte(strings.Repeat("a", 400))
	err := capmon.ValidateContentResponse(body, "text/html", "https://docs.example.com/page", "https://docs.example.com/page")
	if err == nil {
		t.Fatal("expected error for body under 512 bytes")
	}
	var invalid *capmon.ErrContentInvalid
	if !errors.As(err, &invalid) {
		t.Errorf("expected ErrContentInvalid, got %T: %v", err, err)
	}
}

func TestValidateContentResponse_BinaryContentType(t *testing.T) {
	t.Parallel()
	cases := []string{
		"image/png",
		"image/jpeg",
		"video/mp4",
		"audio/mpeg",
		"application/octet-stream",
		"application/zip",
		"application/pdf",
	}
	body := []byte(strings.Repeat("b", 2000))
	for _, ct := range cases {
		ct := ct
		t.Run(ct, func(t *testing.T) {
			t.Parallel()
			err := capmon.ValidateContentResponse(body, ct, "https://docs.example.com/page", "https://docs.example.com/page")
			if err == nil {
				t.Fatalf("expected error for binary content-type %q", ct)
			}
			var invalid *capmon.ErrContentInvalid
			if !errors.As(err, &invalid) {
				t.Errorf("expected ErrContentInvalid, got %T: %v", err, err)
			}
		})
	}
}

func TestValidateContentResponse_DomainSameETLD(t *testing.T) {
	t.Parallel()
	// Redirect between subdomains of the same eTLD+1 domain should pass.
	body := []byte(strings.Repeat("x", 1000))
	err := capmon.ValidateContentResponse(body, "text/html", "https://docs.example.com/page", "https://login.example.com/page")
	if err != nil {
		t.Errorf("redirect within same eTLD+1 should pass, got: %v", err)
	}
}

func TestValidateContentResponse_DomainMismatch(t *testing.T) {
	t.Parallel()
	body := []byte(strings.Repeat("x", 1000))
	err := capmon.ValidateContentResponse(body, "text/html", "https://docs.example.com/page", "https://otherdomain.com/login")
	if err == nil {
		t.Fatal("expected error for redirect to different eTLD+1 domain")
	}
	var invalid *capmon.ErrContentInvalid
	if !errors.As(err, &invalid) {
		t.Errorf("expected ErrContentInvalid, got %T: %v", err, err)
	}
}

func TestValidateContentResponse_InvalidURL(t *testing.T) {
	t.Parallel()
	body := []byte(strings.Repeat("x", 1000))
	// An unparseable URL should return an error.
	err := capmon.ValidateContentResponse(body, "text/html", "://bad-url", "https://example.com")
	if err == nil {
		t.Fatal("expected error for unparseable original URL")
	}
}

func TestValidateContentResponse_TextPlainValid(t *testing.T) {
	t.Parallel()
	body := []byte(strings.Repeat("y", 600))
	err := capmon.ValidateContentResponse(body, "text/plain", "https://raw.github.com/org/repo/main/SKILL.md", "https://raw.github.com/org/repo/main/SKILL.md")
	if err != nil {
		t.Errorf("text/plain with adequate body should pass, got: %v", err)
	}
}

func TestValidateContentResponse_NonHTMLSmallPasses(t *testing.T) {
	t.Parallel()
	// Real codex source files are TypeScript enums and short markdown stubs
	// served from raw.githubusercontent.com (Content-Type: text/plain).
	// A 200-byte text/plain body must pass — the 512-byte threshold was an
	// HTML-stub heuristic and shouldn't false-positive on legitimate small
	// source files. See bead syllago-soagt.
	cases := []struct {
		name        string
		contentType string
	}{
		{"text/plain", "text/plain; charset=utf-8"},
		{"application/json", "application/json"},
		{"text/markdown", "text/markdown"},
	}
	body := []byte(strings.Repeat("a", 200))
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := capmon.ValidateContentResponse(body, tc.contentType, "https://raw.githubusercontent.com/openai/codex/main/HookScope.ts", "https://raw.githubusercontent.com/openai/codex/main/HookScope.ts")
			if err != nil {
				t.Errorf("expected non-HTML 200-byte body to pass, got: %v", err)
			}
		})
	}
}

func TestValidateContentResponse_EmptyAlwaysFails(t *testing.T) {
	t.Parallel()
	// 0-byte responses indicate truncation, vanished resources, or the
	// explorer.toml-style empty-source case. Must always be rejected
	// regardless of content-type so real upstream emptiness is not masked.
	cases := []string{
		"text/html",
		"text/plain",
		"application/json",
		"text/markdown",
	}
	for _, ct := range cases {
		ct := ct
		t.Run(ct, func(t *testing.T) {
			t.Parallel()
			err := capmon.ValidateContentResponse([]byte{}, ct, "https://docs.example.com/page", "https://docs.example.com/page")
			if err == nil {
				t.Fatalf("expected empty body to fail for content-type %q", ct)
			}
			var invalid *capmon.ErrContentInvalid
			if !errors.As(err, &invalid) {
				t.Errorf("expected ErrContentInvalid, got %T: %v", err, err)
			}
			if invalid.Kind != capmon.InvalidBodyTooSmall {
				t.Errorf("Kind = %q, want %q", invalid.Kind, capmon.InvalidBodyTooSmall)
			}
		})
	}
}

func TestValidateContentResponse_HTMLSmallStillFails(t *testing.T) {
	t.Parallel()
	// HTML stubs (login walls, redirect pages, lean 404s) under 512 bytes
	// remain rejected — the original threat model is unchanged.
	body := []byte(strings.Repeat("a", 200))
	cases := []string{
		"text/html",
		"text/html; charset=utf-8",
		"application/xhtml+xml",
	}
	for _, ct := range cases {
		ct := ct
		t.Run(ct, func(t *testing.T) {
			t.Parallel()
			err := capmon.ValidateContentResponse(body, ct, "https://docs.example.com/page", "https://docs.example.com/page")
			if err == nil {
				t.Fatalf("expected HTML body under 512 bytes to fail for content-type %q", ct)
			}
			var invalid *capmon.ErrContentInvalid
			if !errors.As(err, &invalid) {
				t.Errorf("expected ErrContentInvalid, got %T: %v", err, err)
			}
			if invalid.Kind != capmon.InvalidBodyTooSmall {
				t.Errorf("Kind = %q, want %q", invalid.Kind, capmon.InvalidBodyTooSmall)
			}
		})
	}
}

func TestErrContentInvalid_KindSet(t *testing.T) {
	t.Parallel()
	// Each of ValidateContentResponse's three rejection paths must populate
	// ErrContentInvalid.Kind with the matching constant so heal callers can
	// map invalidity to a CandidateOutcomeKind without parsing Reason.
	body512 := []byte(strings.Repeat("a", 600))
	cases := []struct {
		name        string
		body        []byte
		contentType string
		original    string
		final       string
		wantKind    capmon.InvalidKind
	}{
		{
			name:        "binary content",
			body:        body512,
			contentType: "image/png",
			original:    "https://docs.example.com/page",
			final:       "https://docs.example.com/page",
			wantKind:    capmon.InvalidBinaryContent,
		},
		{
			name:        "body too small",
			body:        []byte("short"),
			contentType: "text/html",
			original:    "https://docs.example.com/page",
			final:       "https://docs.example.com/page",
			wantKind:    capmon.InvalidBodyTooSmall,
		},
		{
			name:        "domain mismatch",
			body:        body512,
			contentType: "text/html",
			original:    "https://docs.example.com/page",
			final:       "https://otherdomain.com/login",
			wantKind:    capmon.InvalidDomainMismatch,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := capmon.ValidateContentResponse(tc.body, tc.contentType, tc.original, tc.final)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var invalid *capmon.ErrContentInvalid
			if !errors.As(err, &invalid) {
				t.Fatalf("expected *ErrContentInvalid, got %T: %v", err, err)
			}
			if invalid.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", invalid.Kind, tc.wantKind)
			}
		})
	}
}
