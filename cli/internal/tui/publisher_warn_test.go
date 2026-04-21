package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// TestIsPublisherRevoked covers the gate predicate: publisher-source recall
// triggers the modal; registry-source recall does not (registry blocks are
// enforced in the installer, not via user acknowledgement).
func TestIsPublisherRevoked(t *testing.T) {
	tests := []struct {
		name string
		item catalog.ContentItem
		want bool
	}{
		{
			name: "not recalled returns false",
			item: catalog.ContentItem{Name: "foo"},
			want: false,
		},
		{
			name: "publisher-source recall returns true",
			item: catalog.ContentItem{Name: "foo", Recalled: true, RecallSource: "publisher"},
			want: true,
		},
		{
			name: "registry-source recall returns false (installer hard-blocks)",
			item: catalog.ContentItem{Name: "foo", Recalled: true, RecallSource: "registry"},
			want: false,
		},
		{
			name: "case-insensitive: PUBLISHER",
			item: catalog.ContentItem{Name: "foo", Recalled: true, RecallSource: "PUBLISHER"},
			want: true,
		},
		{
			name: "recalled without source falls through",
			item: catalog.ContentItem{Name: "foo", Recalled: true},
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
// field when present and collapses cleanly when they aren't.
func TestPublisherWarnBody(t *testing.T) {
	tests := []struct {
		name    string
		item    catalog.ContentItem
		wants   []string
		notWant []string
	}{
		{
			name: "all fields present",
			item: catalog.ContentItem{
				Name:             "foo",
				Recalled:         true,
				RecallSource:     "publisher",
				RecallReason:     "key compromise",
				RecallIssuer:     "ops@example.com",
				RecallDetailsURL: "https://example.com/recall/123",
			},
			wants: []string{
				"publisher has revoked",
				"Reason: key compromise",
				"Issued by: ops@example.com",
				"Details: https://example.com/recall/123",
				"The registry has not blocked this hash",
			},
		},
		{
			name: "only reason",
			item: catalog.ContentItem{
				Name:         "foo",
				Recalled:     true,
				RecallSource: "publisher",
				RecallReason: "deprecated",
			},
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
			wants:   []string{"publisher has revoked"},
			notWant: []string{"Reason:", "Issued by:", "Details:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := publisherWarnBody(tt.item)
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

// TestHandleInstallResult_NonRecalledProceedsDirectly verifies the existing
// non-recalled path is unchanged: no stash, no modal, immediate install cmd.
func TestHandleInstallResult_NonRecalledProceedsDirectly(t *testing.T) {
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
		t.Error("expected pendingInstall=nil for non-recalled item")
	}
	if a.confirm.active {
		t.Error("expected confirm modal NOT to be open for non-recalled item")
	}
	if cmd == nil {
		t.Error("expected install cmd to be dispatched immediately")
	}
}

// TestHandleInstallResult_RecalledOpensConfirm verifies a publisher-recalled
// item stashes the install and opens the confirm modal instead of installing
// directly.
func TestHandleInstallResult_RecalledOpensConfirm(t *testing.T) {
	app := testApp(t)
	prov := provider.Provider{Name: "Claude Code", Slug: "claude-code"}

	item := catalog.ContentItem{
		Name:         "dangerous",
		Type:         catalog.Skills,
		Path:         "/tmp/fake",
		Recalled:     true,
		RecallSource: "publisher",
		RecallReason: "publisher revoked",
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

	// Set up a stashed install via the install path.
	item := catalog.ContentItem{
		Name: "dangerous", Type: catalog.Skills, Path: "/tmp/fake",
		Recalled: true, RecallSource: "publisher",
	}
	m, _ := app.Update(installResultMsg{
		item: item, provider: prov, location: "global", method: installer.MethodSymlink,
	})
	app = m.(App)
	if app.pendingInstall == nil {
		t.Fatal("setup failed: expected pendingInstall to be stashed")
	}

	// Confirm path: dispatches install cmd, clears stash.
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
		Recalled: true, RecallSource: "publisher",
	}
	m, _ := app.Update(installResultMsg{
		item: item, provider: prov, location: "global", method: installer.MethodSymlink,
	})
	app = m.(App)

	// Cancel path: stash cleared, toast pushed, no install cmd.
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

// TestHandleInstallAllResult_RecalledOpensConfirm covers the install-to-all
// path: the same modal gates a multi-provider batch.
func TestHandleInstallAllResult_RecalledOpensConfirm(t *testing.T) {
	app := testApp(t)
	provs := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code"},
		{Name: "Gemini CLI", Slug: "gemini-cli"},
	}
	item := catalog.ContentItem{
		Name: "dangerous", Type: catalog.Skills, Path: "/tmp/fake",
		Recalled: true, RecallSource: "publisher",
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
		Recalled: true, RecallSource: "publisher",
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
