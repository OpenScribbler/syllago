package provider

import (
	"strings"
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

// funcWriter has a non-comparable dynamic type, so it cannot key the
// warning-dedupe map.
type funcWriter func([]byte)

func (f funcWriter) Write(p []byte) (int, error) { f(p); return len(p), nil }

func TestCanonicalSlugNonComparableWriter(t *testing.T) {
	output.SetForTest(t)
	var got []string
	output.ErrWriter = funcWriter(func(p []byte) { got = append(got, string(p)) })
	CanonicalSlug("windsurf")
	CanonicalSlug("windsurf")
	if len(got) != 2 || !strings.Contains(got[0], "renamed to") {
		t.Errorf("warnings = %q, want the warning on each call", got)
	}
}
