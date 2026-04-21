package converter

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// frontmatterRegistry stores pre-extracted YAML/TOML field name lists, indexed
// by (content type, provider slug). Reflection happens once at registration time;
// all subsequent lookups are plain map reads with zero reflection.
//
// Assumption: all registered structs are flat (no embedded struct fields).
// If nested/embedded structs are added in the future, RegisterFrontmatter must
// be updated to recurse into them.
//
// frontmatterRegistryMu guards the map. Production callers Register from
// init() (sequential) and Lookup from request handlers, but parallel tests
// mix both on the same goroutines — the mutex keeps the race detector happy
// without forcing tests to serialize.
var (
	frontmatterRegistryMu sync.RWMutex
	frontmatterRegistry   = map[catalog.ContentType]map[string][]string{}
)

// RegisterFrontmatter reflects on example to extract YAML/TOML tag names and
// stores the resulting []string in the registry. Must be called from init()
// functions in converter files.
//
// Tag resolution order: yaml tag first, then toml tag. Fields tagged "-" or
// with empty tag names are skipped. The omitempty suffix (e.g. "name,omitempty")
// is stripped — only the field name is stored.
//
// example may be a struct value or a pointer to a struct. Passing any other type
// panics at init time to fail fast.
func RegisterFrontmatter(ct catalog.ContentType, slug string, example interface{}) {
	t := reflect.TypeOf(example)
	if t == nil {
		panic(fmt.Sprintf("converter.RegisterFrontmatter(%s, %q): nil example", ct, slug))
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("converter.RegisterFrontmatter(%s, %q): example must be a struct, got %s", ct, slug, t.Kind()))
	}

	var fields []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// Try yaml tag first, fall back to toml tag.
		tag := f.Tag.Get("yaml")
		if tag == "" {
			tag = f.Tag.Get("toml")
		}
		if tag == "" {
			continue
		}

		// Split on comma to handle "name,omitempty" → "name".
		name := tag
		if idx := len(tag); idx > 0 {
			for j := 0; j < len(tag); j++ {
				if tag[j] == ',' {
					name = tag[:j]
					break
				}
			}
		}

		// Skip explicitly excluded fields.
		if name == "" || name == "-" {
			continue
		}

		fields = append(fields, name)
	}

	frontmatterRegistryMu.Lock()
	defer frontmatterRegistryMu.Unlock()
	if frontmatterRegistry[ct] == nil {
		frontmatterRegistry[ct] = map[string][]string{}
	}
	frontmatterRegistry[ct][slug] = fields
}

// FrontmatterFieldsFor returns the pre-extracted field name list for the given
// (content type, provider slug) pair. Returns nil if no registration exists —
// callers should treat nil as "no frontmatter data" rather than an error.
func FrontmatterFieldsFor(ct catalog.ContentType, slug string) []string {
	frontmatterRegistryMu.RLock()
	defer frontmatterRegistryMu.RUnlock()
	ctMap, ok := frontmatterRegistry[ct]
	if !ok {
		return nil
	}
	return ctMap[slug]
}
