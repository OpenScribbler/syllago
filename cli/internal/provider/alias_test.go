package provider

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestResolveSlugAlias(t *testing.T) {
	t.Parallel()
	if got, aliased := ResolveSlugAlias("windsurf"); got != "devin" || !aliased {
		t.Errorf("ResolveSlugAlias(windsurf) = (%q, %v), want (devin, true)", got, aliased)
	}
	if got, aliased := ResolveSlugAlias("cursor"); got != "cursor" || aliased {
		t.Errorf("ResolveSlugAlias(cursor) = (%q, %v), want (cursor, false)", got, aliased)
	}
}

func TestCanonicalSlugWarnsOnAlias(t *testing.T) {
	_, stderr := output.SetForTest(t)
	if got := CanonicalSlug("windsurf"); got != "devin" {
		t.Errorf("CanonicalSlug(windsurf) = %q, want devin", got)
	}
	want := "warning: provider \"windsurf\" was renamed to \"devin\"; use \"devin\" instead\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}

	stderr.Reset()
	if got := CanonicalSlug("windsurf"); got != "devin" || stderr.Len() != 0 {
		t.Errorf("second CanonicalSlug(windsurf) = %q with stderr %q, want devin and no repeat warning", got, stderr.String())
	}
	if got := CanonicalSlug("devin"); got != "devin" || stderr.Len() != 0 {
		t.Errorf("CanonicalSlug(devin) = %q with stderr %q, want devin and no warning", got, stderr.String())
	}
}
