package extract_go

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
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
						// For literal consts, extract the actual value
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok {
								var val string
								switch lit.Kind {
								case token.STRING:
									val = strings.Trim(lit.Value, `"`)
								case token.INT, token.FLOAT:
									val = lit.Value
								}
								if val != "" {
									sanitized := capmon.SanitizeExtractedString(val)
									fields[name.Name] = capmon.FieldValue{
										Value:     sanitized,
										ValueHash: capmon.SHA256Hex([]byte(sanitized)),
									}
									continue
								}
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

					// Extract struct fields with yaml tags
					st, ok := ts.Type.(*ast.StructType)
					if !ok || st.Fields == nil {
						continue
					}
					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							continue // embedded field
						}
						fieldIdent := field.Names[0]
						if !fieldIdent.IsExported() {
							continue
						}
						yamlKey := strings.ToLower(fieldIdent.Name)
						if field.Tag != nil {
							tagStr := strings.Trim(field.Tag.Value, "`")
							tag := reflect.StructTag(tagStr)
							yv := tag.Get("yaml")
							if yv == "-" {
								continue
							}
							if yv != "" {
								yamlKey = strings.SplitN(yv, ",", 2)[0]
							}
						}
						key := ts.Name.Name + "." + fieldIdent.Name
						sanitized := capmon.SanitizeExtractedString(yamlKey)
						fields[key] = capmon.FieldValue{
							Value:     sanitized,
							ValueHash: capmon.SHA256Hex([]byte(sanitized)),
						}
					}
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
