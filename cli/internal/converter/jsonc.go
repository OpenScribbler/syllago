package converter

import (
	"bytes"
	"encoding/json"
)

// StripJSONCComments removes // line comments and /* */ block comments from src,
// while leaving content inside JSON string values untouched.
func StripJSONCComments(src []byte) []byte {
	var out bytes.Buffer
	i := 0
	inString := false

	for i < len(src) {
		c := src[i]

		if inString {
			out.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				// Escaped character — write the next byte verbatim and skip it.
				i++
				out.WriteByte(src[i])
			} else if c == '"' {
				inString = false
			}
			i++
			continue
		}

		// Outside a string: check for comment starts.
		if c == '/' && i+1 < len(src) {
			next := src[i+1]
			if next == '/' {
				// Line comment: skip until newline (but keep the newline).
				i += 2
				for i < len(src) && src[i] != '\n' {
					i++
				}
				continue
			}
			if next == '*' {
				// Block comment: skip until closing */.
				i += 2
				for i < len(src)-1 {
					if src[i] == '*' && src[i+1] == '/' {
						i += 2
						break
					}
					i++
				}
				continue
			}
		}

		if c == '"' {
			inString = true
		}
		out.WriteByte(c)
		i++
	}

	return out.Bytes()
}

// ParseJSONC strips JSONC comments from src then unmarshals the result into v.
func ParseJSONC(src []byte, v any) error {
	return json.Unmarshal(StripJSONCComments(src), v)
}
