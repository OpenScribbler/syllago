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
		Name:             "verified-skill",
		Type:             catalog.Skills,
		Registry:         "moat-registry",
		TrustTier:        catalog.TrustTierDualAttested,
		PublisherSubject: "https://github.com/openscribbler/verified-skill",
		PublisherIssuer:  "https://token.actions.githubusercontent.com",
		RegistrySubject:  "https://github.com/openscribbler/moat-registry",
		RegistryIssuer:   "https://token.actions.githubusercontent.com",
		RegistryOperator: "OpenScribbler",
	}
}

func signedItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:             "signed-skill",
		Type:             catalog.Skills,
		Registry:         "moat-registry",
		TrustTier:        catalog.TrustTierSigned,
		RegistrySubject:  "https://github.com/openscribbler/moat-registry",
		RegistryIssuer:   "https://token.actions.githubusercontent.com",
		RegistryOperator: "OpenScribbler",
	}
}

func unsignedItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:      "unsigned-skill",
		Type:      catalog.Skills,
		Registry:  "moat-registry",
		TrustTier: catalog.TrustTierUnsigned,
	}
}

func revokedItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:                 "revoked-skill",
		Type:                 catalog.Skills,
		Registry:             "moat-registry",
		TrustTier:            catalog.TrustTierSigned,
		Revoked:              true,
		RevocationSource:     "publisher",
		RevocationReason:     "key compromise",
		Revoker:              "ops@example.com",
		RevocationDetailsURL: "https://example.com/revocation/123",
		RegistrySubject:      "https://github.com/openscribbler/moat-registry",
		RegistryIssuer:       "https://token.actions.githubusercontent.com",
		RegistryOperator:     "OpenScribbler",
	}
}

func privateItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:             "private-skill",
		Type:             catalog.Skills,
		Registry:         "moat-registry",
		TrustTier:        catalog.TrustTierSigned,
		PrivateRepo:      true,
		RegistrySubject:  "https://github.com/openscribbler/moat-registry",
		RegistryIssuer:   "https://token.actions.githubusercontent.com",
		RegistryOperator: "OpenScribbler",
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
		RevokedItems:  1,
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

func TestTrustInspector_ItemFields_RevokedAddsDangerStatus(t *testing.T) {
	fields := buildItemTrustFields(revokedItem())
	status := findField(fields, "Status")
	if status == nil {
		t.Fatal("expected Status row for revoked item")
	}
	if status.value != "Revoked" {
		t.Errorf("expected Status=Revoked, got %q", status.value)
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
	// Revocation-scoped revoker identity was renamed from "Issuer" to
	// "Revoked by" to disambiguate from the signing cert issuer that now
	// appears in the always-present attestation chain rows (Reg. issuer,
	// Pub. issuer).
	if findField(fields, "Revoked by") == nil {
		t.Error("expected Revoked by row")
	}
	// Revocation details URL row was renamed from "Details" to "More info" —
	// "Detail" is now reserved for the tier-explanation prose row shown
	// on every item.
	if findField(fields, "More info") == nil {
		t.Error("expected More info row")
	}
	// Every revoked item includes the static docs link so users have a
	// path to understand what revocation means outside of this one artifact.
	if learn := findField(fields, "Learn more"); learn == nil {
		t.Error("expected Learn more row with docs link")
	} else if learn.value != "https://syllago.dev/moat/revocations" {
		t.Errorf("expected Learn more docs URL, got %q", learn.value)
	}
}

// TestTrustInspector_ItemFields_AttestationChainRows verifies every item
// surfaces the full signing chain: Registry name, Reg. issuer (OIDC),
// Reg. subject (workload identity), and Publisher rows. The attestation
// chain rows are always present — users should see the same shape on a
// Verified artifact as on a Revoked one so they can compare directly.
func TestTrustInspector_ItemFields_AttestationChainRows(t *testing.T) {
	fields := buildItemTrustFields(verifiedItem())

	if reg := findField(fields, "Registry"); reg == nil || reg.value != "moat-registry" {
		t.Errorf("expected Registry=moat-registry, got %+v", reg)
	}
	if ri := findField(fields, "Reg. issuer"); ri == nil || ri.value == "" {
		t.Errorf("expected Reg. issuer populated, got %+v", ri)
	}
	if rs := findField(fields, "Reg. subject"); rs == nil || rs.value == "" {
		t.Errorf("expected Reg. subject populated, got %+v", rs)
	}
	if ro := findField(fields, "Reg. operator"); ro == nil || ro.value != "OpenScribbler" {
		t.Errorf("expected Reg. operator=OpenScribbler, got %+v", ro)
	}
	if pub := findField(fields, "Publisher"); pub == nil || pub.value == "" {
		t.Errorf("expected Publisher populated for dual-attested, got %+v", pub)
	}
	if pi := findField(fields, "Pub. issuer"); pi == nil || pi.value == "" {
		t.Errorf("expected Pub. issuer populated for dual-attested, got %+v", pi)
	}
}

// TestTrustInspector_ItemFields_SignedItemPublisherFallback verifies that
// a Signed-tier item (registry attests, item carries no signing_profile)
// renders "Not attested individually" as the Publisher value — not an
// empty string or dash. This is the explicit, truthful fallback the user
// asked for: distinguish "deliberately absent" from "data lost."
func TestTrustInspector_ItemFields_SignedItemPublisherFallback(t *testing.T) {
	fields := buildItemTrustFields(signedItem())
	pub := findField(fields, "Publisher")
	if pub == nil {
		t.Fatal("expected Publisher row")
	}
	if pub.value != "Not attested individually" {
		t.Errorf("expected Signed-tier Publisher fallback, got %q", pub.value)
	}
}

// TestTrustInspector_ItemFields_UnknownTierStillRendersChain verifies the
// attestation chain rows are present even for Unknown-tier items so the
// modal has a stable shape — users always see the same rows regardless of
// which item they opened. Values fall back to empty and the renderer
// displays "—" in the View layer.
func TestTrustInspector_ItemFields_UnknownTierStillRendersChain(t *testing.T) {
	unknown := catalog.ContentItem{Name: "vanilla", Type: catalog.Skills}
	fields := buildItemTrustFields(unknown)
	for _, label := range []string{"Tier", "Status", "Detail", "Visibility", "Registry", "Reg. issuer", "Reg. subject", "Publisher", "Pub. issuer"} {
		if findField(fields, label) == nil {
			t.Errorf("expected %s row even for Unknown-tier item", label)
		}
	}
}

func TestTrustInspector_ItemFields_NonRevokedHasStatusRow(t *testing.T) {
	// The Status row is now always present — users should be able to see
	// the collapsed user-facing state (Verified / Not attested / …) on
	// every artifact, not only when it's been revoked. Revocation-only rows
	// (Reason, Revoked by, More info) stay gated on item.Revoked.
	fields := buildItemTrustFields(verifiedItem())
	status := findField(fields, "Status")
	if status == nil {
		t.Fatal("expected Status row even for non-revoked item")
	}
	if status.value != "Verified" {
		t.Errorf("expected Status=Verified for dual-attested item, got %q", status.value)
	}
	if status.danger {
		t.Error("non-revoked Status row should not be flagged danger")
	}
	if findField(fields, "Reason") != nil {
		t.Error("non-revoked should not have Reason row")
	}
	if findField(fields, "Revoked by") != nil {
		t.Error("non-revoked should not have Revoked by row")
	}
	if findField(fields, "Learn more") != nil {
		t.Error("non-revoked should not have Learn more row")
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
	want := "10 total · 7 verified · 1 revoked · 2 private"
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
	// The Detail row renders the long-form prose from
	// catalog.TrustDetailExplanation. For Dual-Attested the explanation
	// opens with "Dual-attested means..." — checking for the phrase
	// "two independent parties" pins the assertion to actual explanation
	// content without coupling to exact punctuation or wrapping.
	if !strings.Contains(out, "two independent parties") {
		t.Errorf("view should include TrustDetailExplanation prose, got:\n%s", out)
	}
}

func TestTrustInspector_ViewShowsRevocationBanner(t *testing.T) {
	m := sizedInspector()
	m.OpenForItem(revokedItem())
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "Revoked") {
		t.Error("revoked view should show Revoked status")
	}
	if !strings.Contains(out, "key compromise") {
		t.Error("revoked view should include the reason")
	}
	if !strings.Contains(out, "ops@example.com") {
		t.Error("revoked view should include the issuer")
	}
	if !strings.Contains(out, "example.com/revocation/123") {
		t.Error("revoked view should include details URL")
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
