package capmon_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_html"
)

func TestHTMLExtractor_BasicSelection(t *testing.T) {
	raw := []byte(`<html><body>
		<main>
			<h2 id="events">Events</h2>
			<table>
				<tr><th>Event Name</th><th>Description</th></tr>
				<tr><td>PreToolUse</td><td>Fires before tool</td></tr>
				<tr><td>PostToolUse</td><td>Fires after tool</td></tr>
			</table>
		</main>
	</body></html>`)

	cfg := capmon.SelectorConfig{
		Primary:          "main table tr td:first-child",
		ExpectedContains: "Event Name",
		MinResults:       1,
	}

	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("Extract html: %v", err)
	}
	if len(result.Fields) == 0 {
		t.Error("no fields extracted")
	}
}

func TestHTMLExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`<html><body><table><tr><td>Unrelated</td></tr></table></body></html>`)
	cfg := capmon.SelectorConfig{
		Primary:          "table tr td",
		ExpectedContains: "Event Name", // not present
	}
	_, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err == nil {
		t.Error("expected error when anchor is missing")
	}
	if !strings.Contains(err.Error(), "anchor_missing") {
		t.Errorf("error %q should mention anchor_missing", err.Error())
	}
}

func TestHTMLExtractor_BelowMinResults(t *testing.T) {
	raw := []byte(`<html><body><table>
		<tr><td>Event Name</td></tr>
		<tr><td>OnlyOne</td></tr>
	</table></body></html>`)
	cfg := capmon.SelectorConfig{
		Primary:          "table tr td",
		ExpectedContains: "Event Name",
		MinResults:       10, // way more than we have
	}
	result, err := capmon.Extract(context.Background(), "html", raw, cfg)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !result.Partial {
		t.Error("result should be marked Partial when below min_results")
	}
}
