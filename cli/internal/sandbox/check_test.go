package sandbox

import (
	"strings"
	"testing"
)

func TestCheck_UnknownProvider(t *testing.T) {
	r := Check("bad-provider", "/home/user", "/tmp/project")
	if r.ProviderOK {
		t.Error("expected ProviderOK=false for unknown provider")
	}
	foundErr := false
	for _, e := range r.Errors {
		if strings.Contains(e, "provider") {
			foundErr = true
		}
	}
	if !foundErr {
		t.Error("expected error mentioning 'provider' for unknown slug")
	}
}

func TestFormatCheckResult_AllOK(t *testing.T) {
	r := CheckResult{
		BwrapOK:      true,
		BwrapVersion: "bubblewrap 0.9.0",
		SocatOK:      true,
		ProviderOK:   true,
		ProviderPath: "/usr/bin/claude",
	}
	out := FormatCheckResult(r, "claude-code")
	if !strings.Contains(out, "Status: Ready for sandboxing") {
		t.Errorf("expected 'Ready for sandboxing', got:\n%s", out)
	}
	if !strings.Contains(out, "0.9.0") {
		t.Error("expected bwrap version in output")
	}
	if !strings.Contains(out, "claude-code") {
		t.Error("expected provider name in output")
	}
}

func TestFormatCheckResult_WithErrors(t *testing.T) {
	r := CheckResult{
		BwrapOK: false,
		SocatOK: true,
		Errors:  []string{"bwrap not found — install bubblewrap >= 0.4.0"},
	}
	out := FormatCheckResult(r, "")
	if !strings.Contains(out, "Status: Not ready") {
		t.Errorf("expected 'Not ready', got:\n%s", out)
	}
	if !strings.Contains(out, "bwrap not found") {
		t.Error("expected error text in output")
	}
}

func TestFormatCheckResult_NoProvider(t *testing.T) {
	r := CheckResult{
		BwrapOK:      true,
		BwrapVersion: "bubblewrap 0.9.0",
		SocatOK:      true,
	}
	out := FormatCheckResult(r, "")
	// Should not contain any provider line
	if strings.Contains(out, "claude-code") {
		t.Error("expected no provider line when providerSlug is empty")
	}
	if !strings.Contains(out, "Ready for sandboxing") {
		t.Error("expected ready status")
	}
}

func TestFormatCheckResult_MissingBwrapFormat(t *testing.T) {
	r := CheckResult{
		BwrapOK: false,
		SocatOK: true,
		Errors:  []string{"bwrap not found — install bubblewrap >= 0.4.0"},
	}
	out := FormatCheckResult(r, "")
	if !strings.Contains(out, "MISSING") {
		t.Error("expected MISSING label for bwrap")
	}
	if !strings.Contains(out, "OK") {
		t.Error("expected OK label for socat")
	}
}
