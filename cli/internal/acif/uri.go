package acif

import (
	"net"
	"net/url"
	"strings"
)

type URLNameResult struct {
	Conformant     bool         `json:"conformant,omitempty"`
	URLDerivedName string       `json:"url_derived_name"`
	Diagnostics    []Diagnostic `json:"diagnostics,omitempty"`
}

func NormalizeSourceURI(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", &RejectError{ID: ErrSourceURIMalformed, Detail: err.Error()}
	}
	if u.Scheme == "" {
		return "", &RejectError{ID: ErrSourceURIMalformed}
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" {
		return "", &RejectError{ID: ErrSourceURISchemeForbidden}
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", &RejectError{ID: ErrSourceURIMalformed}
	}
	if u.User != nil {
		return "", &RejectError{ID: ErrSourceURIUserinfoPresent}
	}

	escapedPath := u.EscapedPath()
	normalizedPath := removeDotSegments(normalizeEscapedPath(escapedPath))
	if normalizedPath == "" {
		normalizedPath = "/"
	}

	port := u.Port()
	if port == "443" {
		port = ""
	}

	if u.RawQuery != "" {
		return "", &RejectError{ID: ErrSourceURIQueryPresent}
	}

	return "https://" + normalizedAuthority(strings.ToLower(u.Hostname()), port) + normalizedPath, nil
}

func DeriveURLName(uri, bodyClassification, frontmatterName string) (URLNameResult, error) {
	return deriveURLName(uri, bodyClassification, nilIfEmpty(frontmatterName))
}

func deriveURLName(uri, bodyClassification string, frontmatterName *string) (URLNameResult, error) {
	if bodyClassification == "multi-file" {
		return URLNameResult{Conformant: true, URLDerivedName: "none"}, nil
	}

	normalized, err := NormalizeSourceURI(uri)
	if err != nil {
		return URLNameResult{}, err
	}
	pathStart := strings.Index(normalized[len("https://"):], "/")
	if pathStart < 0 {
		return URLNameResult{}, &RejectError{ID: ErrSourceURIMalformed}
	}
	escapedPath := normalized[len("https://")+pathStart:]
	if strings.HasSuffix(escapedPath, "/") {
		return URLNameResult{}, &RejectError{ID: ErrSourceURIDirectFileTrailingSlash}
	}

	segment := escapedPath[strings.LastIndex(escapedPath, "/")+1:]
	if dot := strings.LastIndex(segment, "."); dot >= 0 {
		segment = segment[:dot]
	}

	result := URLNameResult{URLDerivedName: segment}
	if frontmatterName != nil && *frontmatterName != segment {
		result.Diagnostics = []Diagnostic{{
			ID: ErrSourceURIFilenameConflict,
			Params: map[string]any{
				"url_derived_name": segment,
				"declared_name":    *frontmatterName,
			},
		}}
	}
	return result, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func normalizedAuthority(host, port string) string {
	if port != "" {
		return net.JoinHostPort(host, port)
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func normalizeEscapedPath(path string) string {
	var out strings.Builder
	out.Grow(len(path))
	for i := 0; i < len(path); {
		if path[i] == '%' && i+2 < len(path) && isHex(path[i+1]) && isHex(path[i+2]) {
			hi := upperHex(path[i+1])
			lo := upperHex(path[i+2])
			decoded := fromHex(hi)<<4 | fromHex(lo)
			if isUnreserved(decoded) {
				out.WriteByte(decoded)
			} else {
				out.WriteByte('%')
				out.WriteByte(hi)
				out.WriteByte(lo)
			}
			i += 3
			continue
		}
		out.WriteByte(path[i])
		i++
	}
	return out.String()
}

func removeDotSegments(input string) string {
	var output strings.Builder
	for input != "" {
		switch {
		case strings.HasPrefix(input, "../"):
			input = input[3:]
		case strings.HasPrefix(input, "./"):
			input = input[2:]
		case strings.HasPrefix(input, "/./"):
			input = "/" + input[3:]
		case input == "/.":
			input = "/"
		case strings.HasPrefix(input, "/../"):
			input = "/" + input[4:]
			removeLastOutputSegment(&output)
		case input == "/..":
			input = "/"
			removeLastOutputSegment(&output)
		case input == "." || input == "..":
			input = ""
		default:
			segment, rest := firstPathSegment(input)
			output.WriteString(segment)
			input = rest
		}
	}
	return output.String()
}

func firstPathSegment(input string) (segment, rest string) {
	if strings.HasPrefix(input, "/") {
		next := strings.Index(input[1:], "/")
		if next < 0 {
			return input, ""
		}
		idx := next + 1
		return input[:idx], input[idx:]
	}
	next := strings.Index(input, "/")
	if next < 0 {
		return input, ""
	}
	return input[:next], input[next:]
}

func removeLastOutputSegment(output *strings.Builder) {
	current := output.String()
	if current == "" {
		return
	}
	idx := strings.LastIndex(current, "/")
	switch {
	case idx < 0:
		current = ""
	case idx == 0:
		current = ""
	default:
		current = current[:idx]
	}
	output.Reset()
	output.WriteString(current)
}

func isUnreserved(b byte) bool {
	return isASCIIAlpha(b) || ('0' <= b && b <= '9') || b == '.' || b == '_' || b == '~' || b == '-'
}

func isHex(b byte) bool {
	return ('0' <= b && b <= '9') || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F')
}

func upperHex(b byte) byte {
	if 'a' <= b && b <= 'f' {
		return b - ('a' - 'A')
	}
	return b
}

func fromHex(b byte) byte {
	switch {
	case '0' <= b && b <= '9':
		return b - '0'
	case 'A' <= b && b <= 'F':
		return b - 'A' + 10
	default:
		return 0
	}
}
