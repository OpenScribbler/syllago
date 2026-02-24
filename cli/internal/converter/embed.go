package converter

import (
	"fmt"
	"regexp"
	"strings"
)

// conversionNotePattern matches the nesco conversion notes block at the bottom of a body.
// Matches: ---\n<!-- nesco:converted ... -->\n...content...
var conversionNotePattern = regexp.MustCompile(`(?s)\n---\n<!-- nesco:converted[^>]*-->\n.*$`)

// ConversionMarker returns the HTML comment marker for converted content.
func ConversionMarker(sourceProvider string) string {
	return fmt.Sprintf("<!-- nesco:converted from=%q -->", sourceProvider)
}

// BuildConversionNotes assembles a conversion notes block from individual note lines.
// Returns empty string if no notes are provided.
func BuildConversionNotes(sourceProvider string, notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(ConversionMarker(sourceProvider))
	b.WriteString("\n")
	for _, note := range notes {
		b.WriteString(note)
		b.WriteString("\n")
	}
	return b.String()
}

// AppendNotes appends a conversion notes block to a body string.
// If notes is empty, returns body unchanged.
func AppendNotes(body, notes string) string {
	if notes == "" {
		return body
	}
	return strings.TrimRight(body, "\n") + "\n\n" + notes
}

// StripConversionNotes removes any nesco conversion notes block from a body.
func StripConversionNotes(body string) string {
	return strings.TrimSpace(conversionNotePattern.ReplaceAllString(body, ""))
}
