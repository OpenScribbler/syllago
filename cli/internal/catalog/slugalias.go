package catalog

// providerSlugAliases maps retired provider slugs to their current slugs.
// It lives in catalog rather than provider because provider imports catalog,
// and the scanner must resolve slugs it reads from directory names,
// registry manifests, and metadata.
var providerSlugAliases = map[string]string{"windsurf": "devin"}

// ResolveProviderSlug maps a retired provider slug to its current slug. It
// returns slug unchanged, with aliased false, when slug is not retired.
func ResolveProviderSlug(slug string) (canonical string, aliased bool) {
	if c, ok := providerSlugAliases[slug]; ok {
		return c, true
	}
	return slug, false
}

// RetiredProviderSlugs returns the retired slugs that resolve to slug, for
// callers that must find content still stored under an old provider
// directory.
func RetiredProviderSlugs(slug string) []string {
	var out []string
	for old, current := range providerSlugAliases {
		if current == slug {
			out = append(out, old)
		}
	}
	return out
}
