package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// --- Fixtures ---

func verifiedItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:      "verified-skill",
		Type:      catalog.Skills,
		TrustTier: catalog.TrustTierDualAttested,
	}
}

func signedItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:      "signed-skill",
		Type:      catalog.Skills,
		TrustTier: catalog.TrustTierSigned,
	}
}

func unsignedItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:      "unsigned-skill",
		Type:      catalog.Skills,
		TrustTier: catalog.TrustTierUnsigned,
	}
}

func recalledItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:             "recalled-skill",
		Type:             catalog.Skills,
		TrustTier:        catalog.TrustTierSigned,
		Recalled:         true,
		RecallSource:     "publisher",
		RecallReason:     "key compromise",
		RecallIssuer:     "ops@example.com",
		RecallDetailsURL: "https://example.com/recall/123",
	}
}

func privateItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:        "private-skill",
		Type:        catalog.Skills,
		TrustTier:   catalog.TrustTierSigned,
		PrivateRepo: true,
	}
}

func registrySummary() RegistryTrustSummary {
	return RegistryTrustSummary{
		Name:          "moat-registry",
		Tier:          catalog.TrustTierDualAttested,
		Issuer:        "https://accounts.example.com",
		Subject:       "registry@example.com",
		Operator:      "Example Corp",
		ManifestURI:   "https://example.com/moat.json",
		FetchedAt:     "2026-04-21T10:00:00Z",
		Staleness:     "Fresh",
		TotalItems:    10,
		VerifiedItems: 7,
		RecalledItems: 1,
		PrivateItems:  2,
	}
}

func sizedInspector() trustInspectorModel {
	m := newTrustInspectorModel()
	m.SetSize(80, 30)
	return m
}

// --- OpenForItem / Close ---

func TestTrustInspector_OpenForItem_Activates(t *testing.T) {
	m := sizedInspector()
	if m.active {
		t.Fatal("inspector should start inactive")
	}
	m.OpenForItem(verifiedItem())
	if !m.active {
		t.Fatal("OpenForItem should activate")
	}
	if m.scope != trustInspectorItem {
		t.Errorf("expected item scope, got %d", m.scope)
	}
}

func TestTrustInspector_OpenForItem_TitleIdentifiesSubject(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	if m.title != "Trust: verified-skill" {
		t.Errorf("expected title to identify item, got %q", m.title)
	}
}

func TestTrustInspector_OpenForItem_PrefersDisplayName(t *testing.T) {
	m := sizedInspector()
	item := verifiedItem()
	item.DisplayName = "Verified Skill"
	m.OpenForItem(item)
	if m.title != "Trust: Verified Skill" {
		t.Errorf("expected DisplayName to be used, got %q", m.title)
	}
}

func TestTrustInspector_OpenForItem_FallsBackToName(t *testing.T) {
	m := sizedInspector()
	item := verifiedItem()
	item.DisplayName = "" // explicit fallback
	m.OpenForItem(item)
	if m.title != "Trust: verified-skill" {
		t.Errorf("expected Name fallback, got %q", m.title)
	}
}

func TestTrustInspector_Close_ClearsState(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	m.Close()
	if m.active {
		t.Fatal("inspector should be inactive after Close")
	}
	if m.title != "" || len(m.fields) != 0 {
		t.Errorf("expected cleared state, got title=%q fields=%d", m.title, len(m.fields))
	}
}

// --- Item field builders ---

func findField(fields []trustField, label string) *trustField {
	for i := range fields {
		if fields[i].label == label {
			return &fields[i]
		}
	}
	return nil
}

func TestTrustInspector_ItemFields_AlwaysHasCoreRows(t *testing.T) {
	// Tier / Detail / Visibility must be present for every tier so the
	// modal layout stays stable across items.
	cases := []struct {
		name string
		item catalog.ContentItem
	}{
		{"verified", verifiedItem()},
		{"signed", signedItem()},
		{"unsigned", unsignedItem()},
		{"private", privateItem()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fields := buildItemTrustFields(c.item)
			if findField(fields, "Tier") == nil {
				t.Error("expected Tier row")
			}
			if findField(fields, "Detail") == nil {
				t.Error("expected Detail row")
			}
			if findField(fields, "Visibility") == nil {
				t.Error("expected Visibility row")
			}
		})
	}
}

func TestTrustInspector_ItemFields_TierText(t *testing.T) {
	cases := []struct {
		name string
		item catalog.ContentItem
		want string
	}{
		{"dual-attested", verifiedItem(), "Dual-Attested"},
		{"signed", signedItem(), "Signed"},
		{"unsigned", unsignedItem(), "Unsigned"},
		{"unknown", catalog.ContentItem{Name: "n"}, "Unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fields := buildItemTrustFields(c.item)
			tier := findField(fields, "Tier")
			if tier == nil || tier.value != c.want {
				got := ""
				if tier != nil {
					got = tier.value
				}
				t.Errorf("want Tier=%q, got %q", c.want, got)
			}
		})
	}
}

func TestTrustInspector_ItemFields_VisibilityLabel(t *testing.T) {
	pub := buildItemTrustFields(verifiedItem())
	if v := findField(pub, "Visibility"); v == nil || v.value != "Public" {
		t.Errorf("expected Visibility=Public for non-private, got %v", v)
	}
	priv := buildItemTrustFields(privateItem())
	if v := findField(priv, "Visibility"); v == nil || v.value != "Private" {
		t.Errorf("expected Visibility=Private for PrivateRepo, got %v", v)
	}
}

func TestTrustInspector_ItemFields_RecalledAddsDangerStatus(t *testing.T) {
	fields := buildItemTrustFields(recalledItem())
	status := findField(fields, "Status")
	if status == nil {
		t.Fatal("expected Status row for recalled item")
	}
	if status.value != "Recalled" {
		t.Errorf("expected Status=Recalled, got %q", status.value)
	}
	if !status.danger {
		t.Error("expected Status row to be flagged danger")
	}
	reason := findField(fields, "Reason")
	if reason == nil || !reason.danger {
		t.Error("expected Reason row flagged danger")
	}
	if findField(fields, "Source") == nil {
		t.Error("expected Source row")
	}
	if findField(fields, "Issuer") == nil {
		t.Error("expected Issuer row")
	}
	if findField(fields, "Details") == nil {
		t.Error("expected Details row")
	}
}

func TestTrustInspector_ItemFields_NonRecalledHasNoRecallRows(t *testing.T) {
	fields := buildItemTrustFields(verifiedItem())
	if findField(fields, "Status") != nil {
		t.Error("non-recalled should not have Status row")
	}
	if findField(fields, "Reason") != nil {
		t.Error("non-recalled should not have Reason row")
	}
}

// --- Registry field builders ---

func TestTrustInspector_OpenForRegistry_Title(t *testing.T) {
	m := sizedInspector()
	m.OpenForRegistry(registrySummary())
	if m.title != "Registry Trust: moat-registry" {
		t.Errorf("expected registry title, got %q", m.title)
	}
	if m.scope != trustInspectorRegistry {
		t.Errorf("expected registry scope, got %d", m.scope)
	}
}

func TestTrustInspector_RegistryFields_AllCoreRows(t *testing.T) {
	fields := buildRegistryTrustFields(registrySummary())
	expected := []string{"Tier", "Issuer", "Subject", "Operator", "Manifest", "Fetched", "Status", "Items"}
	for _, label := range expected {
		if findField(fields, label) == nil {
			t.Errorf("missing registry field %q", label)
		}
	}
}

func TestTrustInspector_RegistryFields_ItemCountsFormatted(t *testing.T) {
	got := formatRegistryItemCounts(registrySummary())
	want := "10 total · 7 verified · 1 recalled · 2 private"
	if got != want {
		t.Errorf("item counts: want %q, got %q", want, got)
	}
}

// --- Key handling ---

func TestTrustInspector_EscCloses(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.active {
		t.Error("Esc should close the inspector")
	}
}

func TestTrustInspector_EnterCloses(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.active {
		t.Error("Enter should close the inspector")
	}
}

func TestTrustInspector_InactiveIgnoresInput(t *testing.T) {
	m := sizedInspector()
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("inactive inspector should not produce commands")
	}
	if m.active {
		t.Error("inactive inspector should remain inactive")
	}
}

// --- Mouse handling ---

func TestTrustInspector_ClickCloseButton(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	zone.Scan(m.View())

	z := zone.Get("trust-inspector-close")
	if z.IsZero() {
		t.Skip("zone trust-inspector-close not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.active {
		t.Error("click on Close should dismiss inspector")
	}
}

func TestTrustInspector_ClickInsideModalStaysOpen(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	zone.Scan(m.View())

	z := zone.Get("trust-inspector-modal")
	if z.IsZero() {
		t.Skip("zone trust-inspector-modal not registered")
	}
	// Click in the middle of the modal body (not on Close button)
	midX := z.StartX + 2
	midY := z.StartY + 2
	m, _ = m.Update(mouseClick(midX, midY))
	if !m.active {
		t.Error("click inside modal (not on Close) should keep it open")
	}
}

func TestTrustInspector_ClickAwayDismisses(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	zone.Scan(m.View())

	z := zone.Get("trust-inspector-modal")
	if z.IsZero() {
		t.Skip("zone trust-inspector-modal not registered")
	}
	// Click far outside the modal zone
	m, _ = m.Update(mouseClick(z.EndX+10, z.EndY+10))
	if m.active {
		t.Error("click outside modal should dismiss per click-away rule")
	}
}

func TestTrustInspector_NonLeftClickIgnored(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	zone.Scan(m.View())

	// Right-click on Close button — must not dismiss
	z := zone.Get("trust-inspector-close")
	if z.IsZero() {
		t.Skip("zone trust-inspector-close not registered")
	}
	msg := tea.MouseMsg{X: z.StartX, Y: z.StartY, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
	m, _ = m.Update(msg)
	if !m.active {
		t.Error("right-click should be ignored, inspector should stay open")
	}
}

// --- View rendering ---

func TestTrustInspector_ViewContainsSubjectName(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "verified-skill") {
		t.Error("view must identify the subject by name")
	}
	if !strings.Contains(out, "Trust:") {
		t.Error("view should include the Trust: title prefix")
	}
}

func TestTrustInspector_ViewShowsTierAndDetail(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "Dual-Attested") {
		t.Error("view should show full tier label")
	}
	if !strings.Contains(out, "dual-attested") {
		t.Error("view should include TrustDescription text")
	}
}

func TestTrustInspector_ViewShowsRecallBanner(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(recalledItem())
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "Recalled") {
		t.Error("recalled view should show Recalled status")
	}
	if !strings.Contains(out, "key compromise") {
		t.Error("recalled view should include the reason")
	}
	if !strings.Contains(out, "ops@example.com") {
		t.Error("recalled view should include the issuer")
	}
	if !strings.Contains(out, "example.com/recall/123") {
		t.Error("recalled view should include details URL")
	}
}

func TestTrustInspector_ViewShowsPrivateVisibility(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(privateItem())
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "Private") {
		t.Error("private item view should show Private visibility")
	}
}

func TestTrustInspector_ViewShowsRegistrySubject(t *testing.T) {
	m := sizedInspector()
	m.OpenForRegistry(registrySummary())
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "moat-registry") {
		t.Error("registry view should identify the subject")
	}
	if !strings.Contains(out, "Registry Trust:") {
		t.Error("registry view should include the Registry Trust: title prefix")
	}
	if !strings.Contains(out, "Example Corp") {
		t.Error("registry view should include operator")
	}
	if !strings.Contains(out, "10 total") {
		t.Error("registry view should include item counts")
	}
}

func TestTrustInspector_ViewInactiveReturnsEmpty(t *testing.T) {
	m := sizedInspector()
	if m.View() != "" {
		t.Error("inactive inspector should render empty")
	}
}

func TestTrustInspector_ViewHasCloseButton(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(verifiedItem())
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "Close") {
		t.Error("view should render a Close button")
	}
}
