package extract_go

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
	capmon.RegisterExtractor("go", &goExtractor{})
}

type goExtractor struct{}

func (e *goExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
		return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in source", cfg.ExpectedContains)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "source.go", raw, 0)
	if err != nil {
		return nil, fmt.Errorf("parse Go source: %w", err)
	}

	fields := make(map[string]capmon.FieldValue)
	var landmarks []string

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			switch d.Tok {
			case token.CONST:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if !name.IsExported() {
							continue
						}
						// For string literal consts, extract the value
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								val := strings.Trim(lit.Value, `"`)
								sanitized := capmon.SanitizeExtractedString(val)
								fields[name.Name] = capmon.FieldValue{
									Value:     sanitized,
									ValueHash: capmon.SHA256Hex([]byte(sanitized)),
								}
								continue
							}
						}
						// For iota/other consts, use the identifier name as value
						sanitized := capmon.SanitizeExtractedString(name.Name)
						fields[name.Name] = capmon.FieldValue{
							Value:     sanitized,
							ValueHash: capmon.SHA256Hex([]byte(sanitized)),
						}
					}
				}
			case token.TYPE:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || !ts.Name.IsExported() {
						continue
					}
					landmarks = append(landmarks, ts.Name.Name)
				}
			}
		}
	}

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "go",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}
