package parity

import (
	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/parse"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
)

// Coverage represents what a single provider has configured.
type Coverage struct {
	Provider string                      `json:"provider"`
	Types    map[catalog.ContentType]int `json:"types"`
}

// Gap represents a content type present in one provider but missing in another.
type Gap struct {
	ContentType catalog.ContentType `json:"contentType"`
	HasIt       []string            `json:"hasIt"`
	MissingIt   []string            `json:"missingIt"`
}

// Report is the complete parity analysis output.
type Report struct {
	Coverages []Coverage `json:"coverages"`
	Gaps      []Gap      `json:"gaps"`
	Summary   string     `json:"summary"`
}

// Analyze runs discovery for all detected providers and compares coverage.
func Analyze(providers []provider.Provider, projectRoot string) Report {
	var coverages []Coverage

	for _, prov := range providers {
		report := parse.Discover(prov, projectRoot)
		coverages = append(coverages, Coverage{
			Provider: prov.Slug,
			Types:    report.Counts,
		})
	}

	gaps := findGaps(coverages, providers)

	return Report{
		Coverages: coverages,
		Gaps:      gaps,
		Summary:   summarize(gaps),
	}
}

func findGaps(coverages []Coverage, providers []provider.Provider) []Gap {
	allFound := make(map[catalog.ContentType]bool)
	for _, c := range coverages {
		for ct, count := range c.Types {
			if count > 0 {
				allFound[ct] = true
			}
		}
	}

	var gaps []Gap
	for ct := range allFound {
		var hasIt, missingIt []string
		for _, c := range coverages {
			prov := findProvider(c.Provider, providers)
			if prov == nil {
				continue
			}
			// Guard against nil SupportsType
			if prov.SupportsType == nil {
				continue
			}
			if prov.SupportsType(ct) {
				if c.Types[ct] > 0 {
					hasIt = append(hasIt, c.Provider)
				} else {
					missingIt = append(missingIt, c.Provider)
				}
			}
		}
		if len(missingIt) > 0 && len(hasIt) > 0 {
			gaps = append(gaps, Gap{
				ContentType: ct,
				HasIt:       hasIt,
				MissingIt:   missingIt,
			})
		}
	}

	return gaps
}

func findProvider(slug string, providers []provider.Provider) *provider.Provider {
	for i := range providers {
		if providers[i].Slug == slug {
			return &providers[i]
		}
	}
	return nil
}

func summarize(gaps []Gap) string {
	if len(gaps) == 0 {
		return "All providers are in sync."
	}
	s := ""
	for _, g := range gaps {
		s += g.ContentType.Label() + ": present in "
		for i, p := range g.HasIt {
			if i > 0 {
				s += ", "
			}
			s += p
		}
		s += " but missing in "
		for i, p := range g.MissingIt {
			if i > 0 {
				s += ", "
			}
			s += p
		}
		s += ". "
	}
	return s
}
