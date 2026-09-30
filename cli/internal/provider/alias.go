package provider

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// slugAliases maps retired provider slugs to their current slugs.
var slugAliases = map[string]string{"windsurf": "devin"}

// ResolveSlugAlias maps a retired provider slug to its current slug. It
// returns slug unchanged, with aliased false, when slug is not retired.
func ResolveSlugAlias(slug string) (canonical string, aliased bool) {
	if c, ok := slugAliases[slug]; ok {
		return c, true
	}
	return slug, false
}

// CanonicalSlug resolves a user- or file-supplied slug through
// ResolveSlugAlias and prints a deprecation warning to stderr when the slug
// is retired.
func CanonicalSlug(slug string) string {
	canonical, aliased := ResolveSlugAlias(slug)
	if aliased {
		fmt.Fprintf(output.ErrWriter, "warning: provider %q was renamed to %q; use %q instead\n", slug, canonical, canonical)
	}
	return canonical
}
