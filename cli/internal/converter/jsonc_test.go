package converter

import (
	"testing"
)

func TestStripJSONCLineComment(t *testing.T) {
	input := []byte(`{
	"key": "value" // this is a comment
}`)
	result := string(StripJSONCComments(input))
	assertContains(t, result, `"key": "value"`)
	assertNotContains(t, result, "// this is a comment")
}

func TestStripJSONCBlockComment(t *testing.T) {
	input := []byte(`{
	/* block comment */
	"key": "value"
}`)
	result := string(StripJSONCComments(input))
	assertContains(t, result, `"key": "value"`)
	assertNotContains(t, result, "block comment")
}

func TestStripJSONCMixedComments(t *testing.T) {
	input := []byte(`{
	/* header comment */
	"first": 1, // line comment
	"second": 2
}`)
	result := string(StripJSONCComments(input))
	assertContains(t, result, `"first": 1`)
	assertContains(t, result, `"second": 2`)
	assertNotContains(t, result, "header comment")
	assertNotContains(t, result, "line comment")
}

func TestStripJSONCPreservesURLsInStrings(t *testing.T) {
	input := []byte(`{
	"url": "https://example.com/path"
}`)
	result := string(StripJSONCComments(input))
	assertContains(t, result, `"https://example.com/path"`)
}

func TestStripJSONCEmptyInput(t *testing.T) {
	result := StripJSONCComments([]byte{})
	if len(result) != 0 {
		t.Errorf("expected empty output for empty input, got %q", result)
	}
}

func TestParseJSONC(t *testing.T) {
	input := []byte(`{
	// name of the thing
	"name": "test", /* inline block */
	"value": 42
}`)
	var out struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := ParseJSONC(input, &out); err != nil {
		t.Fatalf("ParseJSONC: %v", err)
	}
	assertEqual(t, "test", out.Name)
	if out.Value != 42 {
		t.Errorf("expected value 42, got %d", out.Value)
	}
}
