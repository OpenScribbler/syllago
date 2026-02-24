package converter

import (
	"testing"
)

func TestConversionMarker(t *testing.T) {
	got := ConversionMarker("claude-code")
	assertEqual(t, `<!-- nesco:converted from="claude-code" -->`, got)
}

func TestBuildConversionNotes(t *testing.T) {
	notes := BuildConversionNotes("claude-code", []string{
		"**Tool restriction:** Use only read_file and grep_search tools.",
		"Run in an isolated context.",
	})
	assertContains(t, notes, "---\n")
	assertContains(t, notes, `<!-- nesco:converted from="claude-code" -->`)
	assertContains(t, notes, "**Tool restriction:** Use only read_file and grep_search tools.")
	assertContains(t, notes, "Run in an isolated context.")
}

func TestBuildConversionNotesEmpty(t *testing.T) {
	notes := BuildConversionNotes("claude-code", nil)
	assertEqual(t, "", notes)
}

func TestAppendNotes(t *testing.T) {
	body := "Do the thing.\nMore instructions."
	notes := BuildConversionNotes("claude-code", []string{"Use temperature: 0.7"})
	result := AppendNotes(body, notes)

	assertContains(t, result, "Do the thing.")
	assertContains(t, result, "More instructions.")
	assertContains(t, result, "---\n")
	assertContains(t, result, "Use temperature: 0.7")
}

func TestAppendNotesEmpty(t *testing.T) {
	body := "Do the thing."
	result := AppendNotes(body, "")
	assertEqual(t, "Do the thing.", result)
}

func TestStripConversionNotes(t *testing.T) {
	input := `Do the thing.
More instructions.

---
<!-- nesco:converted from="claude-code" -->
**Tool restriction:** Use only read_file and grep_search tools.
Run in an isolated context.
`
	got := StripConversionNotes(input)
	assertEqual(t, "Do the thing.\nMore instructions.", got)
}

func TestStripConversionNotesNoBlock(t *testing.T) {
	input := "Just a normal body.\nNo conversion notes."
	got := StripConversionNotes(input)
	assertEqual(t, input, got)
}

func TestRoundTripBuildAndStrip(t *testing.T) {
	original := "Original body content.\nWith multiple lines."
	notes := BuildConversionNotes("gemini-cli", []string{"Some embedded note."})
	combined := AppendNotes(original, notes)

	assertContains(t, combined, "Some embedded note.")

	stripped := StripConversionNotes(combined)
	assertEqual(t, original, stripped)
}
