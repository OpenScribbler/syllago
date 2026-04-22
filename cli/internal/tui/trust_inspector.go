package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// trustInspectorScope distinguishes what the Trust Inspector is describing.
// The same modal serves both scopes so the user sees a consistent shape
// regardless of whether they opened it from a library row or a registry card.
type trustInspectorScope int

const (
	trustInspectorItem     trustInspectorScope = iota // single catalog item
	trustInspectorRegistry                            // whole registry
)

// RegistryTrustSummary carries the registry-scoped payload. Populated by the
// registries-tab wiring (bead syllago-jc3s7) from moat enrichment aggregates.
// Held as primitives so this struct stays in the tui package without pulling
// in moat — the existing moat → catalog → tui dependency direction is
// preserved.
type RegistryTrustSummary struct {
	Name          string
	Tier          catalog.TrustTier // registry-level signing tier
	Issuer        string
	Subject       string
	Operator      string
	ManifestURI   string
	FetchedAt     string // ISO-ish timestamp for display
	Staleness     string // "Fresh" / "Stale" / "Missing"
	TotalItems    int
	VerifiedItems int
	RevokedItems  int
	PrivateItems  int
}

// registryTrustSummaryFrom narrows a catalog.RegistryTrust (the producer
// aggregate populated by moat.EnrichFromMOATManifests) into the primitives
// this package displays. Keeps the tui package insulated from catalog's
// time.Time field — the inspector only needs a formatted timestamp string.
// Returns a zero-valued summary for a nil pointer so callers can skip the
// nil check at the call site.
func registryTrustSummaryFrom(rt *catalog.RegistryTrust) RegistryTrustSummary {
	if rt == nil {
		return RegistryTrustSummary{}
	}
	var fetched string
	if !rt.FetchedAt.IsZero() {
		fetched = rt.FetchedAt.UTC().Format("2006-01-02 15:04 UTC")
	}
	return RegistryTrustSummary{
		Name:          rt.Name,
		Tier:          rt.Tier,
		Issuer:        rt.Issuer,
		Subject:       rt.Subject,
		Operator:      rt.Operator,
		ManifestURI:   rt.ManifestURI,
		FetchedAt:     fetched,
		Staleness:     rt.Staleness,
		TotalItems:    rt.TotalItems,
		VerifiedItems: rt.VerifiedItems,
		RevokedItems:  rt.RevokedItems,
		PrivateItems:  rt.PrivateItems,
	}
}

// trustField is one row in the inspector's detail list.
//   - danger flips the value color to dangerColor (used for revocation rows).
//   - wrap tells the renderer to wrap the value across multiple lines (via
//     lipgloss Width()) instead of truncating it (MaxWidth). Reserved for
//     prose rows like "Detail" where the whole point is the explanation,
//     and the "Reason" row where an attacker-length string must not push
//     the layout around.
type trustField struct {
	label  string
	value  string
	danger bool
	wrap   bool
}

// trustInspectorModel is the reusable Trust Inspector overlay. Consumed by
// the library tab (bead syllago-6qspt) and the registries tab
// (bead syllago-jc3s7). Both surfaces open the same modal — the user gets a
// consistent view of the trust story regardless of entry point.
type trustInspectorModel struct {
	active bool
	scope  trustInspectorScope
	title  string // header identifies subject, e.g., "Trust: revoked-skill"
	fields []trustField
	width  int
	height int
}

func newTrustInspectorModel() trustInspectorModel {
	return trustInspectorModel{}
}

// OpenForItem activates the inspector for a library item. The title always
// identifies the subject so the user can never lose context about which item
// they clicked.
func (m *trustInspectorModel) OpenForItem(item catalog.ContentItem) {
	m.active = true
	m.scope = trustInspectorItem

	name := item.DisplayName
	if name == "" {
		name = item.Name
	}
	m.title = "Trust: " + name
	m.fields = buildItemTrustFields(item)
}

// OpenForRegistry activates the inspector for a whole registry. Fills in
// aggregate trust data (tier, signing profile, operator, item breakdown,
// staleness).
func (m *trustInspectorModel) OpenForRegistry(s RegistryTrustSummary) {
	m.active = true
	m.scope = trustInspectorRegistry
	m.title = "Registry Trust: " + s.Name
	m.fields = buildRegistryTrustFields(s)
}

// Close deactivates the modal and clears state.
func (m *trustInspectorModel) Close() {
	m.active = false
	m.scope = 0
	m.title = ""
	m.fields = nil
}

// SetSize records the outer terminal dimensions so the modal can size itself
// against the available area.
func (m *trustInspectorModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update handles input while the modal is active.
func (m trustInspectorModel) Update(msg tea.Msg) (trustInspectorModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter {
			m.Close()
		}
	case tea.MouseMsg:
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		if zone.Get("trust-inspector-close").InBounds(msg) {
			m.Close()
			return m, nil
		}
		// Click outside the modal body dismisses per tui-modals.md click-away rule.
		if !zone.Get("trust-inspector-modal").InBounds(msg) {
			m.Close()
		}
	}
	return m, nil
}

// View renders the modal overlay content. Centering and composition happen
// in the App's overlayModal helper.
func (m trustInspectorModel) View() string {
	if !m.active {
		return ""
	}

	modalW := min(64, m.width-10)
	if modalW < 40 {
		modalW = 40
	}
	contentW := modalW - borderSize
	usableW := contentW - 2 // left + right padding
	pad := " "

	var lines []string

	// Title — bold, always identifies the subject.
	titleText := lipgloss.NewStyle().Bold(true).Foreground(primaryText).MaxWidth(usableW).Render(m.title)
	lines = append(lines, pad+titleText, "")

	// Field rows, two-column (label: value). Width-capped per line so long
	// URLs / subjects truncate instead of wrapping.
	labelW := maxLabelWidth(m.fields)
	for _, f := range m.fields {
		label := lipgloss.NewStyle().
			Foreground(mutedColor).
			Width(labelW).
			Render(f.label + ":")

		valueStyle := lipgloss.NewStyle().Foreground(primaryText)
		if f.danger {
			valueStyle = valueStyle.Foreground(dangerColor).Bold(true)
		}
		value := f.value
		if value == "" {
			value = "—"
		}
		valueWidth := usableW - labelW - 1
		if valueWidth < 1 {
			valueWidth = 1
		}
		var valueText string
		if f.wrap {
			// Width() wraps at the boundary instead of truncating — used for
			// multi-line prose rows (Detail, Reason) so users can actually
			// read the full content.
			valueText = valueStyle.Width(valueWidth).Render(value)
		} else {
			valueText = valueStyle.MaxWidth(valueWidth).Render(value)
		}
		// JoinHorizontal with vertical Top alignment pads the label column
		// down to the value's height, so wrapped continuation lines sit
		// under the value column (not under the label). String concat
		// would only prefix the first line with label+space.
		row := lipgloss.JoinHorizontal(lipgloss.Top, label, " ", valueText)
		// PaddingLeft applies to every line in the block, giving the modal
		// its single-space left gutter even when the row wraps.
		row = lipgloss.NewStyle().PaddingLeft(1).Render(row)
		lines = append(lines, row)
	}

	// Close button pinned to the right.
	lines = append(lines, "")
	closeBtn := m.renderCloseButton()
	btnW := lipgloss.Width(closeBtn)
	btnPad := max(0, usableW-btnW)
	lines = append(lines, pad+strings.Repeat(" ", btnPad)+closeBtn)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Width(contentW).
		MaxWidth(modalW).
		Render(content)

	return zone.Mark("trust-inspector-modal", box)
}

func (m trustInspectorModel) renderCloseButton() string {
	style := lipgloss.NewStyle().
		Padding(0, 2).
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}).
		Background(accentColor)
	return zone.Mark("trust-inspector-close", style.Render("Close"))
}

// buildItemTrustFields assembles the ordered row list for an item-scoped
// inspector. The row set is the same shape for every item — Verified and
// Revoked artifacts show the same attestation chain so users can compare
// directly and never wonder "where's the signing info?" The goal is full
// transparency: what was attested, by whom, via which transparency log.
//
// Revocation rows cluster immediately after Status so the "who revoked it
// and why" story reads top-to-bottom without jumping around. The
// attestation chain lives below the prose detail because it is background
// context once an item is revoked, not the headline.
//
// Row order, top to bottom:
//  1. Tier         — normative classification (Dual-Attested / Signed / …)
//  2. Status       — collapsed user-facing state (Verified / Revoked / …)
//
// Revocation rows (appended right after Status when Revoked) — they sit
// together so the revocation story is unbroken:
//   - Revoked by   — who performed the revocation (disambiguated from the
//     OIDC "Reg. issuer" field lower down)
//   - Reason       — sanitized publisher/registry-supplied text (wrapped
//     so a long reason cannot push the layout around)
//   - Source       — "registry" / "publisher"
//   - More info    — sanitized URL from the revocation record
//   - Learn more   — static link to syllago docs so users have a path to
//     understand what revocation means generally
//
// Continuing the attestation chain (always present, below the revocation
// cluster so Verified and Revoked items share the same shape below the
// fold):
//  3. Detail       — multi-line prose explaining the tier / revocation
//  4. Visibility   — Public / Private
//  5. Registry     — registry name (always present for MOAT items)
//  6. Reg. issuer  — OIDC issuer that signed the registry-level claim
//  7. Reg. subject — workload identity in that cert (e.g., GitHub Actions ref)
//  8. Reg. operator — operator name, if the manifest sets one (optional)
//  9. Publisher    — per-item signing subject, or "Not attested individually"
//     when the registry attested but the item did not carry
//     its own signing_profile (Signed tier).
//
// 10. Pub. issuer  — per-item OIDC issuer, "—" when no per-item profile
func buildItemTrustFields(item catalog.ContentItem) []trustField {
	tier := item.TrustTier.String()
	if tier == "" {
		tier = "Unknown"
	}

	status := "No trust claim"
	if item.Revoked {
		status = "Revoked"
	} else {
		switch catalog.UserFacingBadge(item.TrustTier, false) {
		case catalog.TrustBadgeVerified:
			status = "Verified"
		default:
			if item.TrustTier == catalog.TrustTierUnsigned {
				status = "Not attested"
			}
		}
	}

	detail := catalog.TrustDetailExplanation(item.TrustTier, item.Revoked)
	if detail == "" {
		detail = "This content was not sourced from a MOAT registry, so no " +
			"attestation claim has been made either way."
	}

	// Publisher display. Signed tier = registry attested the content but
	// the individual entry carried no signing_profile, so publisher info
	// is absent by design — surface that explicitly instead of rendering
	// an empty dash, which would read as "we lost the data."
	publisherSubject := item.PublisherSubject
	if publisherSubject == "" && item.TrustTier == catalog.TrustTierSigned {
		publisherSubject = "Not attested individually"
	}
	publisherIssuer := item.PublisherIssuer

	fields := []trustField{
		{label: "Tier", value: tier},
		{label: "Status", value: status, danger: item.Revoked},
	}

	// Revocation cluster — appended immediately after Status so the
	// revocation story is one contiguous block.
	if item.Revoked {
		if item.Revoker != "" {
			fields = append(fields, trustField{label: "Revoked by", value: item.Revoker})
		}
		if item.RevocationReason != "" {
			// wrap:true keeps a long / attacker-controlled reason from
			// pushing the layout around — the row grows vertically,
			// not horizontally.
			fields = append(fields, trustField{label: "Reason", value: item.RevocationReason, danger: true, wrap: true})
		}
		if item.RevocationSource != "" {
			fields = append(fields, trustField{label: "Source", value: item.RevocationSource})
		}
		if item.RevocationDetailsURL != "" {
			fields = append(fields, trustField{label: "More info", value: item.RevocationDetailsURL})
		}
		fields = append(fields, trustField{
			label: "Learn more",
			value: "https://syllago.dev/moat/revocations",
		})
	}

	fields = append(fields,
		trustField{label: "Detail", value: detail, wrap: true},
		trustField{label: "Visibility", value: visibilityLabel(item.PrivateRepo)},
		trustField{label: "Registry", value: item.Registry},
		trustField{label: "Reg. issuer", value: item.RegistryIssuer},
		trustField{label: "Reg. subject", value: item.RegistrySubject},
	)
	if item.RegistryOperator != "" {
		fields = append(fields, trustField{label: "Reg. operator", value: item.RegistryOperator})
	}
	fields = append(fields,
		trustField{label: "Publisher", value: publisherSubject},
		trustField{label: "Pub. issuer", value: publisherIssuer},
	)

	return fields
}

// buildRegistryTrustFields assembles the registry-scoped row list.
func buildRegistryTrustFields(s RegistryTrustSummary) []trustField {
	tier := s.Tier.String()
	if tier == "" {
		tier = "Unknown"
	}
	staleness := s.Staleness
	if staleness == "" {
		staleness = "Unknown"
	}

	return []trustField{
		{label: "Tier", value: tier},
		{label: "Issuer", value: s.Issuer},
		{label: "Subject", value: s.Subject},
		{label: "Operator", value: s.Operator},
		{label: "Manifest", value: s.ManifestURI},
		{label: "Fetched", value: s.FetchedAt},
		{label: "Status", value: staleness},
		{label: "Items", value: formatRegistryItemCounts(s)},
	}
}

func formatRegistryItemCounts(s RegistryTrustSummary) string {
	return fmt.Sprintf("%d total · %d verified · %d revoked · %d private",
		s.TotalItems, s.VerifiedItems, s.RevokedItems, s.PrivateItems)
}

func visibilityLabel(privateRepo bool) string {
	if privateRepo {
		return "Private"
	}
	return "Public"
}

func maxLabelWidth(fields []trustField) int {
	w := 0
	for _, f := range fields {
		if l := len(f.label) + 1; l > w { // +1 for the colon
			w = l
		}
	}
	return w
}
