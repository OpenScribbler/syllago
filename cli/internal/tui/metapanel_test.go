package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestMetaBarLinesFor verifies the dynamic line-count contract for the
// metadata panel. Non-MOAT items stay at 3 lines (protecting the hundreds
// of existing golden snapshots); adding a trust surface bumps to 4; a
// revocation banner bumps to 5.
func TestMetaBarLinesFor(t *testing.T) {
	tests := []struct {
		name string
		item *catalog.ContentItem
		want int
	}{
		{
			name: "nil item returns baseline",
			item: nil,
			want: metaBarLinesBase,
		},
		{
			name: "non-MOAT item (Unknown, not recalled, not private)",
			item: &catalog.ContentItem{
				Name:      "foo",
				TrustTier: catalog.TrustTierUnknown,
			},
			want: metaBarLinesBase,
		},
		{
			name: "Signed MOAT item adds chip line",
			item: &catalog.ContentItem{
				Name:      "foo",
				TrustTier: catalog.TrustTierSigned,
			},
			want: metaBarLinesBase + 1,
		},
		{
			name: "Unsigned MOAT item still adds chip line",
			item: &catalog.ContentItem{
				Name:      "foo",
				TrustTier: catalog.TrustTierUnsigned,
			},
			want: metaBarLinesBase + 1,
		},
		{
			name: "DualAttested MOAT item adds chip line",
			item: &catalog.ContentItem{
				Name:      "foo",
				TrustTier: catalog.TrustTierDualAttested,
			},
			want: metaBarLinesBase + 1,
		},
		{
			name: "Private-repo non-MOAT adds chip line for visibility",
			item: &catalog.ContentItem{
				Name:        "foo",
				TrustTier:   catalog.TrustTierUnknown,
				PrivateRepo: true,
			},
			want: metaBarLinesBase + 1,
		},
		{
			name: "Recalled item adds both chip line and banner",
			item: &catalog.ContentItem{
				Name:         "foo",
				TrustTier:    catalog.TrustTierDualAttested,
				Recalled:     true,
				RecallReason: "publisher revoked",
			},
			want: metaBarLinesBase + 2,
		},
		{
			name: "Recalled without tier still adds chip + banner",
			item: &catalog.ContentItem{
				Name:     "foo",
				Recalled: true,
			},
			want: metaBarLinesBase + 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := metaBarLinesFor(tt.item); got != tt.want {
				t.Errorf("metaBarLinesFor() = %d, want %d", got, tt.want)
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
		{Name: "foo", Recalled: true, RecallReason: "r", RecallIssuer: "i", RecallSource: "publisher", RecallDetailsURL: "https://x/y"},
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

// TestRenderMetaPanel_TrustLine verifies Line 4 content for each tier+recall
// combination. The text must match catalog.TrustDescription so the UI and
// other surfaces (install wizard, JSON output) stay consistent.
func TestRenderMetaPanel_TrustLine(t *testing.T) {
	tests := []struct {
		name    string
		item    catalog.ContentItem
		wants   []string // substrings that must appear
		notWant []string // substrings that must NOT appear
	}{
		{
			name: "DualAttested",
			item: catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierDualAttested},
			wants: []string{
				"Trust:",
				"Verified (dual-attested by publisher and registry)",
			},
			notWant: []string{"RECALLED"},
		},
		{
			name:  "Signed",
			item:  catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierSigned},
			wants: []string{"Trust:", "Verified (registry-attested)"},
		},
		{
			name:  "Unsigned",
			item:  catalog.ContentItem{Name: "foo", TrustTier: catalog.TrustTierUnsigned},
			wants: []string{"Trust:", "Unsigned (registry declares no attestation)"},
		},
		{
			name: "Recalled with reason",
			item: catalog.ContentItem{
				Name: "foo", TrustTier: catalog.TrustTierDualAttested,
				Recalled: true, RecallReason: "publisher revoked 2026-04-18",
			},
			wants: []string{
				"Trust:",
				"Recalled \u2014 publisher revoked 2026-04-18",
				"RECALLED",
			},
			notWant: []string{"Verified"},
		},
		{
			name: "Private-repo visibility chip",
			item: catalog.ContentItem{
				Name: "foo", TrustTier: catalog.TrustTierSigned, PrivateRepo: true,
			},
			wants: []string{"Visibility: Private"},
		},
		{
			name:    "Unknown tier suppresses line 4 entirely",
			item:    catalog.ContentItem{Name: "foo"},
			notWant: []string{"Trust:", "RECALLED", "Visibility:"},
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

// TestRenderMetaPanel_RevocationBanner verifies the Line 5 banner format
// preserves all four source/reason/issuer/details fields when present.
func TestRenderMetaPanel_RevocationBanner(t *testing.T) {
	item := catalog.ContentItem{
		Name:             "foo",
		TrustTier:        catalog.TrustTierSigned,
		Recalled:         true,
		RecallSource:     "publisher",
		RecallReason:     "key compromise",
		RecallIssuer:     "registry-admin@example.com",
		RecallDetailsURL: "https://example.com/recall/123",
	}
	out := renderMetaPanel(&item, metaPanelData{installed: "--"}, 200)

	for _, want := range []string{
		"RECALLED",
		"(publisher)",
		"key compromise",
		"Issued by registry-admin@example.com",
		"https://example.com/recall/123",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("revocation banner missing %q:\n%s", want, out)
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
		{"Recalled", catalog.ContentItem{Recalled: true}, "R  "},
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
