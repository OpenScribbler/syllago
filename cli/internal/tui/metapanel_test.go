package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestMetaBarLinesFor verifies the fixed line-count contract for the
// metadata panel. The panel always emits metaBarLinesBase (4) lines
// regardless of content type or trust state: trust+visibility chips now
// share Line 4 with the action buttons (short labels, fixed columns), so
// a revoked or dual-attested item renders the same height as a plain
// skill. Line 3 holds the type-specific handler when present (hooks) and
// is blank otherwise.
func TestMetaBarLinesFor(t *testing.T) {
	tests := []struct {
		name string
		item *catalog.ContentItem
	}{
		{"nil item", nil},
		{"non-MOAT Unknown", &catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierUnknown}},
		{"Signed MOAT", &catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierSigned}},
		{"Unsigned MOAT", &catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierUnsigned}},
		{"DualAttested MOAT", &catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierDualAttested}},
		{"Private-repo non-MOAT", &catalog.ContentItem{Name: "foo", PrivateRepo: true}},
		{"Revoked with reason", &catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierDualAttested, Revoked: true, RevocationReason: "publisher revoked"}},
		{"Revoked without tier", &catalog.ContentItem{Name: "foo", Revoked: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := metaBarLinesFor(tt.item); got != metaBarLinesBase {
				t.Errorf("metaBarLinesFor() = %d, want %d", got, metaBarLinesBase)
			}
		})
	}
}

// TestRenderMetaPanel_HeightContract is the load-bearing invariant callers
// rely on: the rendered panel's lipgloss.Height must equal metaBarLinesFor.
// If this drifts, every parent view's pane math breaks (rows clip or leave
// blank gaps).
func TestRenderMetaPanel_HeightContract(t *testing.T) {
	cases := []*catalog.ContentItem{
		nil,
		{Name: "foo", TrustTier: catalog.TrustTierUnknown},
		{Name: "foo", TrustTier: catalog.TrustTierSigned},
		{Name: "foo", TrustTier: catalog.TrustTierDualAttested, PrivateRepo: true},
		{Name: "foo", Revoked: true, RevocationReason: "r", Revoker: "i", RevocationSource: "publisher", RevocationDetailsURL: "https://x/y"},
	}
	for _, item := range cases {
		var data metaPanelData
		if item != nil {
			data = metaPanelData{installed: "--", typeDetail: ""}
		}
		out := renderMetaPanel(item, data, 120)
		want := metaBarLinesFor(item)
		got := lipgloss.Height(out)
		if got != want {
			t.Errorf("item=%+v: Height=%d, want %d\nrendered:\n%s", item, got, want, out)
		}
	}
}

// TestRenderMetaPanel_TrustLine verifies Line 4 content for each tier+revocation
// combination. The metapanel now surfaces only the short tier label (via
// TrustTier.ShortLabel) so the Visibility chip sits at a stable column
// regardless of tier. The long-form descriptor (TrustDescription) lives
// in the Trust Inspector modal, not the metapanel.
func TestRenderMetaPanel_TrustLine(t *testing.T) {
	tests := []struct {
		name    string
		item    catalog.ContentItem
		wants   []string // substrings that must appear
		notWant []string // substrings that must NOT appear
	}{
		{
			name:  "DualAttested shows short label",
			item:  catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierDualAttested},
			wants: []string{"Trust:", "Dual attested", "Visibility: Public"},
			// Long-form descriptor belongs in the Trust Inspector, not the
			// metapanel — keeping it here would re-introduce the column
			// floating the Visibility chip never sits in a stable place.
			notWant: []string{"dual-attested by publisher and registry", "[t] Inspect trust"},
		},
		{
			name:    "Signed",
			item:    catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierSigned},
			wants:   []string{"Trust:", "Signed", "Visibility: Public"},
			notWant: []string{"registry-attested", "[t] Inspect trust"},
		},
		{
			name:    "Unsigned",
			item:    catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierUnsigned},
			wants:   []string{"Trust:", "Unsigned", "Visibility: Public"},
			notWant: []string{"registry declares no attestation", "[t] Inspect trust"},
		},
		{
			name: "Revoked collapses reason into Trust Inspector",
			item: catalog.ContentItem{
				Name: "foo", TrustTier: catalog.TrustTierDualAttested,
				Revoked: true, RevocationReason: "publisher revoked 2026-04-18",
			},
			wants: []string{"Trust:", "Revoked", "Visibility: Public"},
			// Reason text must not leak into the metapanel — it is surfaced
			// by the Trust Inspector (opened via [t] or a click on the
			// Trust chip) so the column grid stays stable at any reason
			// length.
			notWant: []string{
				"publisher revoked 2026-04-18",
				"\u2014 publisher revoked",
				"[t] Inspect trust",
				"Dual attested",
				"Verified",
			},
		},
		{
			name:  "Private-repo visibility chip",
			item:  catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierSigned, PrivateRepo: true},
			wants: []string{"Visibility: Private"},
		},
		{
			name:  "Signed public item always renders Visibility: Public",
			item:  catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierSigned},
			wants: []string{"Trust:", "Visibility: Public"},
		},
		{
			name: "Unknown tier still renders Trust/Visibility chips",
			item: catalog.ContentItem{Name: "foo"},
			// Non-MOAT items now show "Unknown" / "Public" so the chip row
			// is a stable fixture across every catalog item. Full detail
			// lives in the Trust Inspector, opened via [t].
			wants: []string{"Trust:", "Unknown", "Visibility: Public", "[e] Edit"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderMetaPanel(&tt.item, metaPanelData{installed: "--"}, 120)
			for _, w := range tt.wants {
				if !strings.Contains(out, w) {
					t.Errorf("expected %q in output, got:\n%s", w, out)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(out, nw) {
					t.Errorf("did not expect %q in output, got:\n%s", nw, out)
				}
			}
		})
	}
}

// TestRenderMetaPanel_RevokedCollapsed verifies that revocation details (reason,
// source, issuer, details URL) are routed to the Trust Inspector modal
// instead of the metapanel. The metapanel shows only the "Revoked" short
// state so the column grid is stable regardless of reason length — the
// full breakdown is one key away via [t] or a click on the Trust chip.
func TestRenderMetaPanel_RevokedCollapsed(t *testing.T) {
	item := catalog.ContentItem{
		Name:                 "foo",
		TrustTier:            catalog.TrustTierSigned,
		Revoked:              true,
		RevocationSource:     "publisher",
		RevocationReason:     "key compromise",
		Revoker:              "registry-admin@example.com",
		RevocationDetailsURL: "https://example.com/revocation/123",
	}
	out := renderMetaPanel(&item, metaPanelData{installed: "--"}, 200)

	// Short revoked summary is present; Visibility remains on Line 4.
	for _, want := range []string{
		"Trust:",
		"Revoked",
		"Visibility: Public",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in collapsed output:\n%s", want, out)
		}
	}

	// All revocation detail text must be routed to the Trust Inspector; none
	// of it should appear in the metapanel body. This keeps narrow widths
	// stable and ensures the Visibility chip never shifts with reason
	// length.
	for _, reject := range []string{
		"key compromise",
		"\u2014 key compromise",
		"RECALLED",
		"(publisher)",
		"Issued by",
		"registry-admin@example.com",
		"https://example.com/revocation/123",
		"[t] Inspect trust",
	} {
		if strings.Contains(out, reject) {
			t.Errorf("banner text %q leaked into metapanel:\n%s", reject, out)
		}
	}
}

// TestTrustPrefix covers the 3-char row prefix used by the library table
// and the explorer items list.
func TestTrustPrefix(t *testing.T) {
	tests := []struct {
		name string
		item catalog.ContentItem
		want string
	}{
		{"non-MOAT", catalog.ContentItem{}, "   "},
		{"Verified", catalog.ContentItem{TrustTier: catalog.TrustTierDualAttested}, "\u2713  "},
		{"Revoked", catalog.ContentItem{Revoked: true}, "R  "},
		{"Private non-MOAT", catalog.ContentItem{PrivateRepo: true}, " P "},
		{"Verified + Private", catalog.ContentItem{TrustTier: catalog.TrustTierSigned, PrivateRepo: true}, "\u2713P "},
		{"Unsigned stays blank glyph", catalog.ContentItem{TrustTier: catalog.TrustTierUnsigned}, "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trustPrefix(tt.item); got != tt.want {
				t.Errorf("trustPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
