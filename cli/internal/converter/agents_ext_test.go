package converter

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// TestAgentFileExtMatchesRender keeps AgentFileExt, which the installer uses
// to name installed agents, in step with the file Render produces.
func TestAgentFileExtMatchesRender(t *testing.T) {
	t.Parallel()
	canonical := []byte("---\nname: reviewer\ndescription: Reviews code\n---\n\nReview.\n")
	for _, prov := range provider.AllProviders {
		if prov.SupportsType == nil || !prov.SupportsType(catalog.Agents) {
			continue
		}
		res, err := (&AgentsConverter{}).Render(canonical, prov)
		if err != nil || res.Content == nil {
			continue
		}
		if ext := AgentFileExt(prov.Slug); !strings.HasSuffix(res.Filename, ext) {
			t.Errorf("%s: Render filename %q does not end with AgentFileExt %q", prov.Slug, res.Filename, ext)
		}
	}
}
