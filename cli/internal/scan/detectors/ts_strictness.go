package detectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// TSStrictness reports the effective TypeScript strictness level by parsing
// tsconfig.json compilerOptions. This is factual context (CatConventions),
// not a surprise — it tells contributors what type-checking rules apply.
//
// The "strict" flag is an umbrella that enables several sub-flags. When strict
// is true, individual flags can still override (e.g., strict + noImplicitAny:false
// means everything is strict except implicit any). When strict is false or absent,
// only individually enabled flags apply.
type TSStrictness struct{}

func (d TSStrictness) Name() string { return "ts-strictness" }

// strictSubFlags are the flags controlled by the "strict" umbrella.
var strictSubFlags = []string{
	"noImplicitAny",
	"strictNullChecks",
	"strictFunctionTypes",
	"strictBindCallApply",
	"strictPropertyInitialization",
	"noImplicitThis",
	"alwaysStrict",
	"useUnknownInCatchVariables",
}

func (d TSStrictness) Detect(root string) ([]model.Section, error) {
	tsconfigPath := filepath.Join(root, "tsconfig.json")
	if _, err := os.Stat(tsconfigPath); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	data, err := os.ReadFile(tsconfigPath)
	if err != nil {
		return nil, nil
	}

	var tsconfig struct {
		CompilerOptions map[string]interface{} `json:"compilerOptions"`
	}
	if err := json.Unmarshal(data, &tsconfig); err != nil {
		return nil, nil
	}

	if tsconfig.CompilerOptions == nil {
		return nil, nil
	}

	opts := tsconfig.CompilerOptions

	// Check the "strict" umbrella flag
	strictEnabled := boolOpt(opts, "strict")

	// Determine effective state of each sub-flag
	var enabled, disabled []string
	for _, flag := range strictSubFlags {
		if val, exists := opts[flag]; exists {
			if asBool(val) {
				enabled = append(enabled, flag)
			} else {
				disabled = append(disabled, flag)
			}
		} else if strictEnabled {
			enabled = append(enabled, flag)
		}
	}

	// Build description
	var body string
	switch {
	case strictEnabled && len(disabled) == 0:
		body = "TypeScript strict mode is fully enabled — all strict sub-flags are active."
	case strictEnabled && len(disabled) > 0:
		sort.Strings(disabled)
		body = fmt.Sprintf("TypeScript strict mode is enabled but these flags are explicitly disabled: %s.", strings.Join(disabled, ", "))
	case !strictEnabled && len(enabled) > 0:
		sort.Strings(enabled)
		body = fmt.Sprintf("TypeScript strict mode is not enabled. These strict flags are individually set: %s.", strings.Join(enabled, ", "))
	default:
		body = "TypeScript strict mode is not enabled and no individual strict flags are set."
	}

	return []model.Section{model.TextSection{
		Category: model.CatConventions,
		Origin:   model.OriginAuto,
		Title:    "TypeScript Strictness",
		Body:     body,
		Source:   d.Name(),
	}}, nil
}

// boolOpt reads a boolean from a JSON object map, defaulting to false.
func boolOpt(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	return asBool(v)
}

// asBool converts an interface{} to bool, handling JSON number/bool types.
func asBool(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	default:
		return false
	}
}
