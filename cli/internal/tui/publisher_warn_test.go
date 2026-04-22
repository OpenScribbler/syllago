package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// TestPublisherWarnBody verifies the modal body preserves every revocation
// field when present and collapses cleanly when they aren't. Covers both
// sources of data: the live RevocationRecord (preferred) and the
// ContentItem fallback fields populated at enrich time.
func TestPublisherWarnBody(t *testing.T) {
	tests := []struct {
		name    string
		item    catalog.ContentItem
		rev     *moat.RevocationRecord
		wants   []string
		notWant []string
	}{
		{
			name: "all fields from ContentItem fallback",
			item: catalog.ContentItem{
				Name:             "foo",
				Recalled:         true,
				RecallSource:     "publisher",
				RecallReason:     "key compromise",
				RecallIssuer:     "ops@example.com",
				RecallDetailsURL: "https://example.com/recall/123",
			},
			rev: nil,
			wants: []string{
				"publisher has revoked",
				"Reason: key compromise",
				"Issued by: ops@example.com",
				"Details: https://example.com/recall/123",
				"The registry has not blocked this hash",
			},
		},
		{
			name: "gate record overrides fallback reason+details",
			item: catalog.ContentItem{
				Name:             "foo",
				Recalled:         true,
				RecallReason:     "stale-reason",
				RecallIssuer:     "ops@example.com",
				RecallDetailsURL: "https://stale.example.com/",
			},
			rev: &moat.RevocationRecord{
				Reason:     "live-reason",
				DetailsURL: "https://live.example.com/",
			},
			wants: []string{
				"Reason: live-reason",
				"Details: https://live.example.com/",
				"Issued by: ops@example.com",
			},
			notWant: []string{"stale-reason", "stale.example.com"},
		},
		{
			name: "only reason",
			item: catalog.ContentItem{
				Name:         "foo",
				Recalled:     true,
				RecallSource: "publisher",
				RecallReason: "deprecated",
			},
			rev:     nil,
			wants:   []string{"Reason: deprecated"},
			notWant: []string{"Issued by:", "Details:"},
		},
		{
			name: "no optional fields",
			item: catalog.ContentItem{
				Name:         "foo",
				Recalled:     true,
				RecallSource: "publisher",
			},
			rev:     nil,
			wants:   []string{"publisher has revoked"},
			notWant: []string{"Reason:", "Issued by:", "Details:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := publisherWarnBody(tt.item, tt.rev)
			for _, w := range tt.wants {
				if !strings.Contains(body, w) {
					t.Errorf("body missing %q:\n%s", w, body)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(body, nw) {
					t.Errorf("body should not contain %q:\n%s", nw, body)
				}
			}
		})
	}
}

// TestPublisherWarnTitle prefers DisplayName but falls back to Name.
func TestPublisherWarnTitle(t *testing.T) {
	if got := publisherWarnTitle(catalog.ContentItem{Name: "foo"}); !strings.Contains(got, "\"foo\"") {
		t.Errorf("expected title to quote Name; got %q", got)
	}
	if got := publisherWarnTitle(catalog.ContentItem{Name: "foo", DisplayName: "Foo Pretty"}); !strings.Contains(got, "\"Foo Pretty\"") {
		t.Errorf("expected title to prefer DisplayName; got %q", got)
	}
}

// TestPrivatePromptTitle prefers DisplayName and distinguishes from publisher.
func TestPrivatePromptTitle(t *testing.T) {
	if got := privatePromptTitle(catalog.ContentItem{Name: "foo"}); !strings.Contains(got, "private") {
		t.Errorf("expected private-prompt title to mention 'private'; got %q", got)
	}
	if got := privatePromptTitle(catalog.ContentItem{Name: "foo", DisplayName: "Foo Pretty"}); !strings.Contains(got, "\"Foo Pretty\"") {
		t.Errorf("expected private-prompt title to prefer DisplayName; got %q", got)
	}
}

// TestPrivatePromptBody covers the private-source body text. Does not make
// claims about optional fields (the manifest carries none for private
// items beyond the flag itself).
func TestPrivatePromptBody(t *testing.T) {
	body := privatePromptBody(catalog.ContentItem{Name: "foo"})
	for _, w := range []string{"private source", "credentials"} {
		if !strings.Contains(body, w) {
			t.Errorf("body missing %q:\n%s", w, body)
		}
	}
}
