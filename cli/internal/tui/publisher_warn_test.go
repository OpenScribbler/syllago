package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// TestIsPublisherRevoked covers the gate predicate: publisher-source revocation
// triggers the modal; registry-source revocation does not (registry blocks are
// enforced in the installer, not via user acknowledgement).
func TestIsPublisherRevoked(t *testing.T) {
	tests := []struct {
		name string
		item catalog.ContentItem
		want bool
	}{
		{
			name: "not revoked returns false",
			item: catalog.ContentItem{Name: "foo"},
			want: false,
		},
		{
			name: "publisher-source revocation returns true",
			item: catalog.ContentItem{Name: "foo", Revoked: true, RevocationSource: "publisher"},
			want: true,
		},
		{
			name: "registry-source revocation returns false (installer hard-blocks)",
			item: catalog.ContentItem{Name: "foo", Revoked: true, RevocationSource: "registry"},
			want: false,
		},
		{
			name: "case-insensitive: PUBLISHER",
			item: catalog.ContentItem{Name: "foo", Revoked: true, RevocationSource: "PUBLISHER"},
			want: true,
		},
		{
			name: "revoked without source falls through",
			item: catalog.ContentItem{Name: "foo", Revoked: true},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPublisherRevoked(tt.item); got != tt.want {
				t.Errorf("isPublisherRevoked() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
				Name:                 "foo",
				Revoked:              true,
				RevocationSource:     "publisher",
				RevocationReason:     "key compromise",
				Revoker:              "ops@example.com",
				RevocationDetailsURL: "https://example.com/revocation/123",
			},
			rev: nil,
			wants: []string{
				"publisher has revoked",
				"Reason: key compromise",
				"Revoked by: ops@example.com",
				"Details: https://example.com/revocation/123",
				"The registry has not blocked this hash",
			},
		},
		{
			name: "gate record overrides fallback reason+details",
			item: catalog.ContentItem{
				Name:                 "foo",
				Revoked:              true,
				RevocationReason:     "stale-reason",
				Revoker:              "ops@example.com",
				RevocationDetailsURL: "https://stale.example.com/",
			},
			rev: &moat.RevocationRecord{
				Reason:     "live-reason",
				DetailsURL: "https://live.example.com/",
			},
			wants: []string{
				"Reason: live-reason",
				"Details: https://live.example.com/",
				"Revoked by: ops@example.com",
			},
			notWant: []string{"stale-reason", "stale.example.com"},
		},
		{
			name: "only reason",
			item: catalog.ContentItem{
				Name:             "foo",
				Revoked:          true,
				RevocationSource: "publisher",
				RevocationReason: "deprecated",
			},
			rev:     nil,
			wants:   []string{"Reason: deprecated"},
			notWant: []string{"Revoked by:", "Details:"},
		},
		{
			name: "no optional fields",
			item: catalog.ContentItem{
				Name:             "foo",
				Revoked:          true,
				RevocationSource: "publisher",
			},
			rev:     nil,
			wants:   []string{"publisher has revoked"},
			notWant: []string{"Reason:", "Revoked by:", "Details:"},
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

// TestHandleInstallResult_NonRevokedProceedsDirectly verifies the existing
// non-revoked path is unchanged: no stash, no modal, immediate install cmd.
func TestHandleInstallResult_NonRevokedProceedsDirectly(t *testing.T) {
	app := testApp(t)
	prov := provider.Provider{Name: "Claude Code", Slug: "claude-code"}

	msg := installResultMsg{
		item:        catalog.ContentItem{Name: "foo", Type: catalog.Skills, Path: "/tmp/fake"},
		provider:    prov,
		location:    "global",
		method:      installer.MethodSymlink,
		projectRoot: "",
	}

	m, cmd := app.Update(msg)
	a := m.(App)

	if a.pendingInstall != nil {
		t.Error("expected pendingInstall=nil for non-revoked item")
	}
	if a.confirm.active {
		t.Error("expected confirm modal NOT to be open for non-revoked item")
	}
	if cmd == nil {
		t.Error("expected install cmd to be dispatched immediately")
	}
}

// TestHandleInstallResult_RevokedOpensConfirm verifies a publisher-revoked
// item stashes the install and opens the confirm modal instead of installing
// directly.
func TestHandleInstallResult_RevokedOpensConfirm(t *testing.T) {
	app := testApp(t)
	prov := provider.Provider{Name: "Claude Code", Slug: "claude-code"}

	item := catalog.ContentItem{
		Name:             "dangerous",
		Type:             catalog.Skills,
		Path:             "/tmp/fake",
		Revoked:          true,
		RevocationSource: "publisher",
		RevocationReason: "publisher revoked",
	}
	msg := installResultMsg{
		item: item, provider: prov, location: "global", method: installer.MethodSymlink,
	}

	m, cmd := app.Update(msg)
	a := m.(App)

	if cmd != nil {
		t.Error("expected NO install cmd (install is stashed pending confirmation)")
	}
	if a.pendingInstall == nil {
		t.Fatal("expected pendingInstall to be stashed")
	}
	if a.pendingInstall.item.Name != "dangerous" {
		t.Errorf("expected stashed item name 'dangerous', got %q", a.pendingInstall.item.Name)
	}
	if !a.confirm.active {
		t.Error("expected confirm modal to be open")
	}
	if !a.confirm.danger {
		t.Error("expected confirm modal in danger mode (red border)")
	}
}

// TestHandleConfirmResult_DispatchesStashedInstall verifies that confirming
// the modal dispatches the stashed install, and cancelling clears the stash
// and pushes a toast.
func TestHandleConfirmResult_DispatchesStashedInstall(t *testing.T) {
	app := testApp(t)
	prov := provider.Provider{Name: "Claude Code", Slug: "claude-code"}

	item := catalog.ContentItem{
		Name: "dangerous", Type: catalog.Skills, Path: "/tmp/fake",
		Revoked: true, RevocationSource: "publisher",
	}
	m, _ := app.Update(installResultMsg{
		item: item, provider: prov, location: "global", method: installer.MethodSymlink,
	})
	app = m.(App)
	if app.pendingInstall == nil {
		t.Fatal("setup failed: expected pendingInstall to be stashed")
	}

	m, cmd := app.Update(confirmResultMsg{confirmed: true, item: item})
	a := m.(App)
	if cmd == nil {
		t.Error("expected install cmd to fire on confirm")
	}
	if a.pendingInstall != nil {
		t.Error("expected pendingInstall to be cleared after confirm")
	}
}

func TestHandleConfirmResult_CancelClearsStashAndToasts(t *testing.T) {
	app := testApp(t)
	prov := provider.Provider{Name: "Claude Code", Slug: "claude-code"}

	item := catalog.ContentItem{
		Name: "dangerous", Type: catalog.Skills, Path: "/tmp/fake",
		Revoked: true, RevocationSource: "publisher",
	}
	m, _ := app.Update(installResultMsg{
		item: item, provider: prov, location: "global", method: installer.MethodSymlink,
	})
	app = m.(App)

	m, cmd := app.Update(confirmResultMsg{confirmed: false, item: item})
	a := m.(App)
	if cmd == nil {
		t.Error("expected toast cmd on cancel")
	}
	if a.pendingInstall != nil {
		t.Error("expected pendingInstall to be cleared after cancel")
	}
	if !a.toast.visible {
		t.Error("expected toast to be visible after cancel")
	}
}

// TestHandleInstallAllResult_RevokedOpensConfirm covers the install-to-all
// path: the same modal gates a multi-provider batch.
func TestHandleInstallAllResult_RevokedOpensConfirm(t *testing.T) {
	app := testApp(t)
	provs := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code"},
		{Name: "Gemini CLI", Slug: "gemini-cli"},
	}
	item := catalog.ContentItem{
		Name: "dangerous", Type: catalog.Skills, Path: "/tmp/fake",
		Revoked: true, RevocationSource: "publisher",
	}

	m, cmd := app.Update(installAllResultMsg{item: item, providers: provs})
	a := m.(App)

	if cmd != nil {
		t.Error("expected no install cmd (stashed pending)")
	}
	if a.pendingInstallAll == nil {
		t.Fatal("expected pendingInstallAll stashed")
	}
	if a.pendingInstall != nil {
		t.Error("expected pendingInstall nil when pendingInstallAll set")
	}
	if !a.confirm.active {
		t.Error("expected confirm modal open")
	}
}

// TestHandleConfirmResult_StashedAllDispatches verifies install-all confirm
// dispatches the batch cmd.
func TestHandleConfirmResult_StashedAllDispatches(t *testing.T) {
	app := testApp(t)
	provs := []provider.Provider{{Name: "Claude Code", Slug: "claude-code"}}
	item := catalog.ContentItem{
		Name: "dangerous", Type: catalog.Skills, Path: "/tmp/fake",
		Revoked: true, RevocationSource: "publisher",
	}

	m, _ := app.Update(installAllResultMsg{item: item, providers: provs})
	app = m.(App)

	m, cmd := app.Update(confirmResultMsg{confirmed: true, item: item})
	a := m.(App)
	if cmd == nil {
		t.Error("expected batch install cmd on confirm")
	}
	if a.pendingInstallAll != nil {
		t.Error("expected pendingInstallAll cleared")
	}
}
