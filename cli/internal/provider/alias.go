package provider

import (
	"fmt"
	"io"
	"sync"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// ResolveSlugAlias maps a retired provider slug to its current slug. It
// returns slug unchanged, with aliased false, when slug is not retired.
func ResolveSlugAlias(slug string) (canonical string, aliased bool) {
	return catalog.ResolveProviderSlug(slug)
}

// aliasWarnings records which retired slugs have already been warned about
// on which writer, so one command that meets a retired slug in several flags
// or config fields warns once. Keying on the writer lets each test that
// installs its own stderr see the warning afresh.
var (
	aliasWarnMu   sync.Mutex
	aliasWarnings = map[aliasWarnKey]bool{}
)

type aliasWarnKey struct {
	w    io.Writer
	slug string
}

// CanonicalSlug resolves a user- or file-supplied slug through
// ResolveSlugAlias and prints a deprecation warning to stderr the first time
// a given retired slug appears.
func CanonicalSlug(slug string) string {
	canonical, aliased := ResolveSlugAlias(slug)
	if !aliased {
		return canonical
	}
	aliasWarnMu.Lock()
	defer aliasWarnMu.Unlock()
	key := aliasWarnKey{output.ErrWriter, slug}
	if !aliasWarnings[key] {
		aliasWarnings[key] = true
		fmt.Fprintf(output.ErrWriter, "warning: provider %q was renamed to %q; use %q instead\n", slug, canonical, canonical)
	}
	return canonical
}
