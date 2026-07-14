package acif

import (
	"strings"
)

const argumentsToken = "$ARGUMENTS"

func CanonicalizeCommand(block map[string]any) (*ItemResult, error) {
	block = unwrapKindBlock(block, "command")
	out := make(map[string]any, len(block))
	for k, v := range block {
		out[k] = cloneJSONValue(v)
	}
	verdict := applyRequiresVerdict(out)
	return &ItemResult{Canonical: out, Verdict: verdict}, nil
}

func RewriteCommandPlaceholders(body string) (string, []Diagnostic) {
	var out strings.Builder
	out.Grow(len(body))
	var diagnostics []Diagnostic
	for i := 0; i < len(body); {
		switch {
		case strings.HasPrefix(body[i:], "{{args}}"):
			out.WriteString(argumentsToken)
			i += len("{{args}}")
		case strings.HasPrefix(body[i:], "${input:"):
			end := strings.IndexByte(body[i:], '}')
			if end < 0 {
				out.WriteByte(body[i])
				i++
				continue
			}
			token := body[i : i+end+1]
			if validInputPlaceholder(token) {
				out.WriteString(argumentsToken)
				diagnostics = append(diagnostics, Diagnostic{ID: DiagCommandPlaceholderNamedArgCollapsed})
			} else {
				out.WriteString(token)
			}
			i += end + 1
		default:
			out.WriteByte(body[i])
			i++
		}
	}
	return out.String(), diagnostics
}

func validInputPlaceholder(token string) bool {
	if !strings.HasPrefix(token, "${input:") || !strings.HasSuffix(token, "}") {
		return false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(token, "${input:"), "}")
	name, _, _ := strings.Cut(inner, ":")
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isInputVarNameByte(name[i]) {
			return false
		}
	}
	return true
}

func isInputVarNameByte(b byte) bool {
	return isASCIIAlpha(b) || ('0' <= b && b <= '9') || b == '_' || b == '-'
}

func CommandAdvisoryProjection(item map[string]any) (map[string]any, error) {
	block, _ := unwrapItemBlock(item, "command")
	result, err := CanonicalizeCommand(block)
	if err != nil {
		return nil, err
	}
	body, _ := result.Canonical["body"].(string)
	present := commandTokenPresent(body)
	token := map[string]any{"present": present}
	if present {
		token["method"] = "substring-canonical-v1"
	}
	return map[string]any{
		"argument_substitution_token": token,
	}, nil
}

func commandTokenPresent(body string) bool {
	for i := 0; i < len(body); {
		idx := strings.Index(body[i:], argumentsToken)
		if idx < 0 {
			return false
		}
		pos := i + idx
		next := pos + len(argumentsToken)
		if next == len(body) || !isArgumentContinuation(body[next]) {
			return true
		}
		i = next
	}
	return false
}

func RenderCommand(item map[string]any, target string) (*RenderResult, error) {
	block, _ := unwrapItemBlock(item, "command")
	result, err := CanonicalizeCommand(block)
	if err != nil {
		return nil, err
	}
	body, _ := result.Canonical["body"].(string)
	switch target {
	case "gemini-form":
		return &RenderResult{Output: translateArgumentsToken(body, "{{args}}")}, nil
	case "input-form":
		return &RenderResult{Output: translateArgumentsToken(body, "${input:args}")}, nil
	case "no-row-target":
		rendered := translateArgumentsToken(body, argumentsToken)
		var diagnostics []Diagnostic
		if commandTokenPresent(body) {
			diagnostics = append(diagnostics, Diagnostic{ID: DiagCommandPlaceholderUntranslated})
		}
		return &RenderResult{Output: rendered, Diagnostics: diagnostics}, nil
	default:
		return &RenderResult{Unsupported: true}, nil
	}
}

func translateArgumentsToken(body, replacement string) string {
	var out strings.Builder
	out.Grow(len(body))
	for i := 0; i < len(body); {
		if strings.HasPrefix(body[i:], argumentsToken) {
			next := i + len(argumentsToken)
			if next == len(body) || !isArgumentContinuation(body[next]) {
				if next < len(body) && body[next] == '[' {
					out.WriteString(argumentsToken)
				} else {
					out.WriteString(replacement)
				}
				i = next
				continue
			}
		}
		out.WriteByte(body[i])
		i++
	}
	return out.String()
}

func isArgumentContinuation(b byte) bool {
	return isASCIIAlpha(b) || ('0' <= b && b <= '9') || b == '_'
}
