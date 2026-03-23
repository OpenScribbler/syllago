package output

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestDocsURL_Format(t *testing.T) {
	got := docsURL("CATALOG_001")
	want := "https://openscribbler.github.io/syllago-docs/errors/catalog-001/"
	if got != want {
		t.Errorf("docsURL(%q) = %q, want %q", "CATALOG_001", got, want)
	}
}

func TestDocsURL_OtherCategories(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"REGISTRY_001", "https://openscribbler.github.io/syllago-docs/errors/registry-001/"},
		{"INSTALL_003", "https://openscribbler.github.io/syllago-docs/errors/install-003/"},
		{"CONVERT_002", "https://openscribbler.github.io/syllago-docs/errors/convert-002/"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := docsURL(tt.code)
			if got != tt.want {
				t.Errorf("docsURL(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestNewStructuredError_PopulatesDocsURL(t *testing.T) {
	se := NewStructuredError(ErrCatalogNotFound, "catalog not found", "run syllago init")
	if se.DocsURL == "" {
		t.Error("DocsURL should be populated automatically")
	}
	want := "https://openscribbler.github.io/syllago-docs/errors/catalog-001/"
	if se.DocsURL != want {
		t.Errorf("DocsURL = %q, want %q", se.DocsURL, want)
	}
}

func TestNewStructuredError_Fields(t *testing.T) {
	se := NewStructuredError(ErrProviderNotFound, "unknown provider: foo", "try bar instead")
	if se.Code != ErrProviderNotFound {
		t.Errorf("Code = %q, want %q", se.Code, ErrProviderNotFound)
	}
	if se.Message != "unknown provider: foo" {
		t.Errorf("Message = %q", se.Message)
	}
	if se.Suggestion != "try bar instead" {
		t.Errorf("Suggestion = %q", se.Suggestion)
	}
	if se.Details != "" {
		t.Errorf("Details should be empty, got %q", se.Details)
	}
}

func TestNewStructuredErrorDetail_Fields(t *testing.T) {
	se := NewStructuredErrorDetail(ErrCatalogScanFailed, "scan failed", "check permissions", "stat /some/path: permission denied")
	if se.Code != ErrCatalogScanFailed {
		t.Errorf("Code = %q, want %q", se.Code, ErrCatalogScanFailed)
	}
	if se.Details != "stat /some/path: permission denied" {
		t.Errorf("Details = %q", se.Details)
	}
	if se.DocsURL == "" {
		t.Error("DocsURL should be populated automatically")
	}
}

func TestStructuredError_ErrorInterface(t *testing.T) {
	se := NewStructuredError(ErrProviderNotFound, "unknown provider: foo", "")
	got := se.Error()
	if !strings.Contains(got, "PROVIDER_001") {
		t.Errorf("Error() = %q, want code PROVIDER_001 in output", got)
	}
	if !strings.Contains(got, "unknown provider: foo") {
		t.Errorf("Error() = %q, want message in output", got)
	}
}

func TestPrintStructuredError_PlainText(t *testing.T) {
	_, stderr := SetForTest(t)

	se := NewStructuredError(ErrCatalogNotFound, "catalog not found", "run syllago init")
	PrintStructuredError(se)

	out := stderr.String()
	if !strings.Contains(out, "CATALOG_001") {
		t.Errorf("plain text output missing code, got:\n%s", out)
	}
	if !strings.Contains(out, "catalog not found") {
		t.Errorf("plain text output missing message, got:\n%s", out)
	}
	if !strings.Contains(out, "run syllago init") {
		t.Errorf("plain text output missing suggestion, got:\n%s", out)
	}
	if !strings.Contains(out, "https://openscribbler.github.io/syllago-docs/errors/catalog-001/") {
		t.Errorf("plain text output missing docs URL, got:\n%s", out)
	}
}

func TestPrintStructuredError_JSON(t *testing.T) {
	_, stderr := SetForTest(t)
	JSON = true

	se := NewStructuredError(ErrRegistryClone, "clone failed", "check network")
	PrintStructuredError(se)

	out := stderr.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if result["code"] != ErrRegistryClone {
		t.Errorf("JSON code = %v, want %q", result["code"], ErrRegistryClone)
	}
	if result["message"] != "clone failed" {
		t.Errorf("JSON message = %v", result["message"])
	}
	if result["suggestion"] != "check network" {
		t.Errorf("JSON suggestion = %v", result["suggestion"])
	}
	if result["docs_url"] == "" || result["docs_url"] == nil {
		t.Error("JSON docs_url should be present")
	}
}

func TestPrintStructuredError_NoSuggestion(t *testing.T) {
	_, stderr := SetForTest(t)

	se := NewStructuredError(ErrExportNotSupported, "export not supported", "")
	PrintStructuredError(se)

	out := stderr.String()
	if strings.Contains(out, "Suggestion:") {
		t.Errorf("output should omit Suggestion line when empty, got:\n%s", out)
	}
	if !strings.Contains(out, "EXPORT_001") {
		t.Errorf("output missing code, got:\n%s", out)
	}
}

func TestPrintStructuredError_NoDocsURL(t *testing.T) {
	_, stderr := SetForTest(t)

	// Construct a StructuredError with no DocsURL directly (bypassing constructor)
	se := StructuredError{
		Code:    "CUSTOM_001",
		Message: "custom error",
	}
	PrintStructuredError(se)

	out := stderr.String()
	if strings.Contains(out, "Docs:") {
		t.Errorf("output should omit Docs line when DocsURL is empty, got:\n%s", out)
	}
}

func TestAllErrorCodes_UniqueValues(t *testing.T) {
	codes := allErrorCodes()
	seen := make(map[string]string)
	for _, name := range codes {
		val := errorCodeValue(name)
		if prev, exists := seen[val]; exists {
			t.Errorf("duplicate error code value %q: used by both %q and %q", val, prev, name)
		}
		seen[val] = name
	}
}

func TestAllErrorCodes_Format(t *testing.T) {
	pattern := regexp.MustCompile(`^[A-Z]+_\d{3}$`)
	for _, val := range allErrorCodes() {
		if !pattern.MatchString(val) {
			t.Errorf("error code %q does not match CATEGORY_NNN pattern", val)
		}
	}
}
