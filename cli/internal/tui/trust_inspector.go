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
	RecalledItems int
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
		RecalledItems: rt.RecalledItems,
		PrivateItems:  rt.PrivateItems,
	}
}

// trustField is one row in the inspector's detail list. `danger` flips the
// value color to dangerColor (used for recall rows).
type trustField struct {
	label  string
	value  string
	danger bool
}

// trustInspectorModel is the reusable Trust Inspector overlay. Consumed by
// the library tab (bead syllago-6qspt) and the registries tab
// (bead syllago-jc3s7). Both surfaces open the same modal — the user gets a
// consistent view of the trust story regardless of entry point.
type trustInspectorModel struct {
	active bool
	scope  trustInspectorScope
	title  string // header identifies subject, e.g., "Trust: recalled-skill"
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
		valueText := valueStyle.MaxWidth(valueWidth).Render(value)
		lines = append(lines, pad+label+" "+valueText)
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
// inspector. Tier / Detail / Visibility are always present to keep the
// modal a stable shape across items; recall fields only appear when
// Recalled is true.
func buildItemTrustFields(item catalog.ContentItem) []trustField {
	tier := item.TrustTier.String()
	if tier == "" {
		tier = "Unknown"
	}
	desc := catalog.TrustDescription(item.TrustTier, false, "")
	if desc == "" {
		desc = "No trust claim made"
	}

	fields := []trustField{
		{label: "Tier", value: tier},
		{label: "Detail", value: desc},
		{label: "Visibility", value: visibilityLabel(item.PrivateRepo)},
	}

	if item.Recalled {
		fields = append(fields, trustField{label: "Status", value: "Recalled", danger: true})
		if item.RecallSource != "" {
			fields = append(fields, trustField{label: "Source", value: item.RecallSource})
		}
		if item.RecallReason != "" {
			fields = append(fields, trustField{label: "Reason", value: item.RecallReason, danger: true})
		}
		if item.RecallIssuer != "" {
			fields = append(fields, trustField{label: "Issuer", value: item.RecallIssuer})
		}
		if item.RecallDetailsURL != "" {
			fields = append(fields, trustField{label: "Details", value: item.RecallDetailsURL})
		}
	}

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
	return fmt.Sprintf("%d total · %d verified · %d recalled · %d private",
		s.TotalItems, s.VerifiedItems, s.RecalledItems, s.PrivateItems)
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
