package extract_html_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_html"
)

func TestHTMLExtractor_PrimarySelector(t *testing.T) {
	raw := []byte(`<html><body>
<h1>Title</h1>
<table>
  <tr><td>Alpha</td></tr>
  <tr><td>Beta</td></tr>
  <tr><td>Gamma</td></tr>
</table>
<ul><li>Ignored</li></ul>
</body></html>`)
	cfg := capmon.SelectorConfig{Primary: "table td"}

	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract html: %v", err)
	}

	if len(result.Fields) != 3 {
		t.Errorf("expected 3 fields from scoped selector, got %d", len(result.Fields))
	}
	wantValues := []string{"Alpha", "Beta", "Gamma"}
	for _, want := range wantValues {
		found := false
		for _, fv := range result.Fields {
			if fv.Value == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected scoped value %q in fields", want)
		}
	}
	// "Ignored" is in <li>, but Primary scoped to "table td" should exclude it.
	for _, fv := range result.Fields {
		if fv.Value == "Ignored" {
			t.Error("scoped selector must not extract elements outside the selector")
		}
	}
}

func TestHTMLExtractor_FallbackMode(t *testing.T) {
	raw := []byte(`<html><body>
<table>
  <tr><th>Header</th></tr>
  <tr><td>Cell1</td><td>Cell2</td></tr>
</table>
<ul><li>List1</li><li>List2</li></ul>
</body></html>`)
	cfg := capmon.SelectorConfig{} // no Primary → fallback

	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract html: %v", err)
	}

	wantValues := []string{"Header", "Cell1", "Cell2", "List1", "List2"}
	for _, want := range wantValues {
		found := false
		for _, fv := range result.Fields {
			if fv.Value == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected fallback value %q in fields", want)
		}
	}
}

func TestHTMLExtractor_AnchorMatch(t *testing.T) {
	raw := []byte(`<html><body><p>The Skill schema describes name and description fields.</p></body></html>`)
	cfg := capmon.SelectorConfig{ExpectedContains: "Skill schema"}

	_, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Errorf("expected anchor match to succeed, got: %v", err)
	}
}

func TestHTMLExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`<html><body><p>Unrelated content.</p></body></html>`)
	cfg := capmon.SelectorConfig{ExpectedContains: "Skill schema"}

	_, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err == nil {
		t.Fatal("expected error when expected_contains anchor not found")
	}
	if !strings.Contains(err.Error(), "anchor_missing") {
		t.Errorf("expected error to mention anchor_missing, got: %v", err)
	}
}

func TestHTMLExtractor_MinResultsPartial(t *testing.T) {
	raw := []byte(`<html><body><table><tr><td>Only</td></tr></table></body></html>`)
	cfg := capmon.SelectorConfig{MinResults: 5}

	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract html: %v", err)
	}
	if !result.Partial {
		t.Errorf("expected Partial=true when len(fields)=%d < MinResults=%d",
			len(result.Fields), cfg.MinResults)
	}
}

func TestHTMLExtractor_MinResultsMet(t *testing.T) {
	raw := []byte(`<html><body><ul><li>A</li><li>B</li><li>C</li></ul></body></html>`)
	cfg := capmon.SelectorConfig{MinResults: 2}

	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract html: %v", err)
	}
	if result.Partial {
		t.Errorf("expected Partial=false when len(fields)=%d >= MinResults=%d",
			len(result.Fields), cfg.MinResults)
	}
}

func TestHTMLExtractor_Landmarks(t *testing.T) {
	raw := []byte(`<html><body>
<h1>Top</h1>
<h2>Section A</h2>
<h3>Subsection</h3>
<h4>Detail</h4>
<h5>NotALandmark</h5>
<p>body</p>
</body></html>`)
	cfg := capmon.SelectorConfig{}

	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract html: %v", err)
	}

	wantLandmarks := []string{"Top", "Section A", "Subsection", "Detail"}
	for _, want := range wantLandmarks {
		found := false
		for _, got := range result.Landmarks {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected landmark %q in result", want)
		}
	}
	for _, got := range result.Landmarks {
		if got == "NotALandmark" {
			t.Error("h5 should not be collected as a landmark (only h1-h4)")
		}
	}
}

func TestHTMLExtractor_FormatAndVersion(t *testing.T) {
	raw := []byte(`<html><body><p>x</p></body></html>`)
	cfg := capmon.SelectorConfig{}

	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract html: %v", err)
	}
	if result.Format != "html" {
		t.Errorf("Format: got %q, want %q", result.Format, "html")
	}
	if result.ExtractorVersion != "1" {
		t.Errorf("ExtractorVersion: got %q, want %q", result.ExtractorVersion, "1")
	}
}

func TestHTMLExtractor_FieldHashesPopulated(t *testing.T) {
	raw := []byte(`<html><body><ul><li>Alpha</li></ul></body></html>`)
	cfg := capmon.SelectorConfig{}

	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract html: %v", err)
	}
	if len(result.Fields) == 0 {
		t.Fatal("expected at least one field")
	}
	for key, fv := range result.Fields {
		if fv.ValueHash == "" {
			t.Errorf("field %q: ValueHash must be populated", key)
		}
		if fv.Value == "" {
			t.Errorf("field %q: Value must be populated", key)
		}
	}
}
