package analyzer

import (
	"strings"
	"unicode"
)

// Field length limits for DetectedItem string fields.
const (
	maxNameLen        = 80
	maxDescriptionLen = 200
	maxPathLen        = 256
	maxShortFieldLen  = 64
)

// SanitizeItem strips C0/C1 control characters from all string fields of a
// DetectedItem and truncates fields that exceed display length limits.
// Must be called after Classify, before dedup or audit writes.
func SanitizeItem(item *DetectedItem) {
	item.Name = sanitizeField(item.Name, maxNameLen)
	item.DisplayName = sanitizeField(item.DisplayName, maxNameLen)
	item.Description = sanitizeField(item.Description, maxDescriptionLen)
	item.Path = sanitizeField(item.Path, maxPathLen)
	item.ConfigSource = sanitizeField(item.ConfigSource, maxPathLen)
	item.HookEvent = sanitizeField(item.HookEvent, maxShortFieldLen)
	item.Provider = sanitizeField(item.Provider, maxShortFieldLen)
	item.InternalLabel = sanitizeField(item.InternalLabel, maxShortFieldLen)
	// ContentHash: strip non-hex characters, keep max 64 chars.
	item.ContentHash = sanitizeHex(item.ContentHash)
	item.Scripts = sanitizeSlice(item.Scripts, maxPathLen)
	item.References = sanitizeSlice(item.References, maxPathLen)
	item.Providers = sanitizeSlice(item.Providers, maxPathLen)
}

// sanitizeField strips C0/C1 control chars (except \t and \n) and truncates to maxLen runes.
func sanitizeField(s string, maxLen int) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if isAllowedRune(r) {
			b.WriteRune(r)
		}
	}
	clean := b.String()
	runes := []rune(clean)
	if len(runes) > maxLen {
		return string(runes[:maxLen-1]) + "…"
	}
	return clean
}

// sanitizeSlice applies sanitizeField to each element.
func sanitizeSlice(ss []string, maxLen int) []string {
	for i, s := range ss {
		ss[i] = sanitizeField(s, maxLen)
	}
	return ss
}

// sanitizeHex strips non-hex characters and truncates to 64 chars.
func sanitizeHex(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) > 64 {
		return result[:64]
	}
	return result
}

// isAllowedRune returns true for runes that are safe for display and structured output.
// Allows printable characters, tab, and newline. Strips C0 (0x00–0x1F except \t,\n)
// and C1 (0x80–0x9F) control characters.
func isAllowedRune(r rune) bool {
	if r == '\t' || r == '\n' {
		return true
	}
	if r < 0x20 { // C0 control chars
		return false
	}
	if r >= 0x80 && r <= 0x9F { // C1 control chars
		return false
	}
	return unicode.IsPrint(r) || r == ' '
}
