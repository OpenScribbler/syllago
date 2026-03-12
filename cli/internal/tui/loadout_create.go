package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	overlay "github.com/rmhubbert/bubbletea-overlay"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

type createLoadoutStep int

const (
	clStepProvider createLoadoutStep = iota // pick provider (skipped if pre-filled)
	clStepTypes                             // checkbox list of content types
	clStepItems                             // per-type item selection (Task 1.3)
	clStepName                              // name + description text inputs
	clStepDest                              // pick destination
	clStepReview                            // summary + confirm
)

// typeCheckEntry represents a content type checkbox in the type selection step.
type typeCheckEntry struct {
	ct      catalog.ContentType
	checked bool
	count   int // number of available items for this type
}

const (
	createLoadoutModalWidth  = 56
	createLoadoutModalHeight = 24
)

// loadoutItemEntry is one item in the checkbox picker.
type loadoutItemEntry struct {
	item     catalog.ContentItem
	selected bool
}

type createLoadoutModal struct {
	active bool
	step   createLoadoutStep

	// Context passed in at creation
	prefilledProvider string // non-empty = skip provider step
	scopeRegistry    string // non-empty = scope items to this registry

	// Step 1: provider picker
	providerList   []provider.Provider
	providerCursor int

	// Step 2: content type selection
	typeEntries []typeCheckEntry
	typeCursor  int

	// Step 3: per-type item selection
	entries        []loadoutItemEntry              // all items from catalog
	selectedTypes  []catalog.ContentType           // types chosen in step 2 (ordered)
	typeStepIndex  int                             // which type we're currently on
	typeItemMap    map[catalog.ContentType][]int   // type -> indices into entries (compatible)
	typeItemMapAll map[catalog.ContentType][]int   // type -> indices into entries (all, including incompatible)
	showAllCompat  bool                            // true = show incompatible items grayed out
	perTypeCursor  map[catalog.ContentType]int     // preserved cursor per type
	perTypeScroll  map[catalog.ContentType]int     // preserved scroll offset per type
	perTypeSearch  map[catalog.ContentType]string  // preserved search query per type
	searchInput    textinput.Model

	// Step 3: name/desc
	nameInput textinput.Model
	descInput textinput.Model
	nameFirst bool // true = nameInput focused

	// Step 4: destination
	destOptions  []string // "Project", "Library", optionally "Registry: <name>"
	destCursor   int
	destDisabled []bool   // parallel: true = option grayed out
	destHints    []string // parallel: explanation for disabled options

	// Step 6: review
	reviewCursor int // 0=Back, 1=Create

	confirmed    bool // true = user pressed Create on review step; false = Esc cancelled
	message      string
	messageIsErr bool
}

// newCreateLoadoutModal creates a new Create Loadout wizard.
func newCreateLoadoutModal(
	prefilledProvider string,
	scopeRegistry string,
	allProviders []provider.Provider,
	cat *catalog.Catalog,
) createLoadoutModal {
	si := textinput.New()
	si.Placeholder = "filter items..."
	si.CharLimit = 100

	ni := textinput.New()
	ni.Prompt = labelStyle.Render("Name: ")
	ni.Placeholder = "my-loadout"
	ni.CharLimit = 100
	ni.Focus()

	di := textinput.New()
	di.Prompt = labelStyle.Render("Description: ")
	di.Placeholder = "What this loadout does"
	di.CharLimit = 300

	m := createLoadoutModal{
		active:           true,
		prefilledProvider: prefilledProvider,
		scopeRegistry:    scopeRegistry,
		providerList:     allProviders,
		searchInput:      si,
		nameInput:        ni,
		descInput:        di,
		nameFirst:        true,
		perTypeCursor:    make(map[catalog.ContentType]int),
		perTypeScroll:    make(map[catalog.ContentType]int),
		perTypeSearch:    make(map[catalog.ContentType]string),
		typeItemMap:      make(map[catalog.ContentType][]int),
		typeItemMapAll:   make(map[catalog.ContentType][]int),
	}

	m.destOptions = []string{"Project (loadouts/ in repo)", "Library (~/.syllago/content/loadouts/)"}
	m.destDisabled = []bool{false, false}
	m.destHints = []string{"", ""}
	if scopeRegistry != "" {
		m.destOptions = append(m.destOptions, fmt.Sprintf("Registry: %s", scopeRegistry))
		m.destDisabled = append(m.destDisabled, false)
		m.destHints = append(m.destHints, "")
	}

	m.entries = buildLoadoutItemEntries(cat, scopeRegistry)

	if prefilledProvider != "" {
		m.buildTypeEntries()
		m.step = clStepTypes
	} else {
		m.step = clStepProvider
	}

	return m
}

// buildTypeEntries populates typeEntries for the selected provider.
// Only includes content types that have available items AND are supported by the provider.
// Loadouts are excluded (you don't nest loadouts).
func (m *createLoadoutModal) buildTypeEntries() {
	provSlug := m.prefilledProvider
	var prov *provider.Provider
	for i := range m.providerList {
		if m.providerList[i].Slug == provSlug {
			prov = &m.providerList[i]
			break
		}
	}

	// Count items per type from catalog entries
	typeCounts := make(map[catalog.ContentType]int)
	for _, e := range m.entries {
		typeCounts[e.item.Type]++
	}

	m.typeEntries = nil
	for _, ct := range catalog.AllContentTypes() {
		if ct == catalog.Loadouts {
			continue // don't nest loadouts
		}
		count := typeCounts[ct]
		if count == 0 {
			continue // skip types with no available items
		}
		if prov != nil && prov.SupportsType != nil && !prov.SupportsType(ct) {
			continue // skip types the provider doesn't support
		}
		m.typeEntries = append(m.typeEntries, typeCheckEntry{
			ct:      ct,
			checked: true, // default all checked
			count:   count,
		})
	}
	m.typeCursor = 0
}

// buildTypeItemMaps pre-filters entries by type and provider compatibility.
// Called when transitioning from types step to items step.
func (m *createLoadoutModal) buildTypeItemMaps() {
	m.selectedTypes = nil
	for _, te := range m.typeEntries {
		if te.checked {
			m.selectedTypes = append(m.selectedTypes, te.ct)
		}
	}

	provSlug := m.prefilledProvider
	var prov *provider.Provider
	for i := range m.providerList {
		if m.providerList[i].Slug == provSlug {
			prov = &m.providerList[i]
			break
		}
	}

	m.typeItemMap = make(map[catalog.ContentType][]int)
	m.typeItemMapAll = make(map[catalog.ContentType][]int)
	for i, e := range m.entries {
		ct := e.item.Type
		m.typeItemMapAll[ct] = append(m.typeItemMapAll[ct], i)
		// Compatible = provider supports this type
		if prov == nil || prov.SupportsType == nil || prov.SupportsType(ct) {
			m.typeItemMap[ct] = append(m.typeItemMap[ct], i)
		}
	}
	m.typeStepIndex = 0
}

// currentTypeItems returns the indices into m.entries for the current type step.
func (m createLoadoutModal) currentTypeItems() []int {
	if m.typeStepIndex >= len(m.selectedTypes) {
		return nil
	}
	ct := m.selectedTypes[m.typeStepIndex]
	if m.showAllCompat {
		return m.typeItemMapAll[ct]
	}
	return m.typeItemMap[ct]
}

// currentType returns the content type for the current type step.
func (m createLoadoutModal) currentType() catalog.ContentType {
	if m.typeStepIndex >= len(m.selectedTypes) {
		return ""
	}
	return m.selectedTypes[m.typeStepIndex]
}

// currentTypeSelectedCount returns how many items are selected for the current type.
func (m createLoadoutModal) currentTypeSelectedCount() int {
	count := 0
	ct := m.currentType()
	for _, e := range m.entries {
		if e.selected && e.item.Type == ct {
			count++
		}
	}
	return count
}

// isItemCompatible checks if an entry index is compatible with the selected provider.
func (m createLoadoutModal) isItemCompatible(idx int) bool {
	ct := m.entries[idx].item.Type
	indices := m.typeItemMap[ct]
	for _, i := range indices {
		if i == idx {
			return true
		}
	}
	return false
}

// filteredTypeItems applies the search query to the current type's items.
func (m createLoadoutModal) filteredTypeItems() []int {
	items := m.currentTypeItems()
	ct := m.currentType()
	query := strings.ToLower(m.perTypeSearch[ct])
	if query == "" {
		return items
	}
	var out []int
	for _, idx := range items {
		e := m.entries[idx]
		if strings.Contains(strings.ToLower(e.item.Name), query) ||
			strings.Contains(strings.ToLower(e.item.Description), query) {
			out = append(out, idx)
		}
	}
	return out
}

func buildLoadoutItemEntries(cat *catalog.Catalog, scopeRegistry string) []loadoutItemEntry {
	var entries []loadoutItemEntry
	for _, item := range cat.Items {
		if scopeRegistry != "" && item.Registry != scopeRegistry {
			continue
		}
		entries = append(entries, loadoutItemEntry{item: item, selected: false})
	}
	return entries
}

func (m createLoadoutModal) currentStepNum() int {
	fixedBefore := 2 // provider + types (or just types if pre-filled)
	if m.prefilledProvider != "" {
		fixedBefore = 1 // just types
	}
	switch m.step {
	case clStepProvider:
		return 1
	case clStepTypes:
		if m.prefilledProvider != "" {
			return 1
		}
		return 2
	case clStepItems:
		return fixedBefore + m.typeStepIndex + 1
	case clStepName:
		return fixedBefore + len(m.selectedTypes) + 1
	case clStepDest:
		return fixedBefore + len(m.selectedTypes) + 2
	case clStepReview:
		return fixedBefore + len(m.selectedTypes) + 3
	}
	return 1
}

func (m createLoadoutModal) dynamicTotalSteps() int {
	fixedBefore := 2 // provider + types
	if m.prefilledProvider != "" {
		fixedBefore = 1 // just types
	}
	nTypes := len(m.selectedTypes)
	if nTypes == 0 {
		// Before types are selected, estimate from typeEntries
		nTypes = len(m.typeEntries)
	}
	return fixedBefore + nTypes + 3 // + name + dest + review
}

func (m createLoadoutModal) Update(msg tea.Msg) (createLoadoutModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.message = ""

		switch m.step {
		case clStepProvider:
			switch {
			case msg.Type == tea.KeyEsc:
				m.active = false
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.providerCursor > 0 {
					m.providerCursor--
				}
			case key.Matches(msg, keys.Down):
				if m.providerCursor < len(m.providerList)-1 {
					m.providerCursor++
				}
			case msg.Type == tea.KeyEnter:
				m.prefilledProvider = m.providerList[m.providerCursor].Slug
				m.buildTypeEntries()
				m.step = clStepTypes
			}

		case clStepTypes:
			switch {
			case msg.Type == tea.KeyEsc:
				if m.prefilledProvider != "" {
					// Pre-filled provider — Esc dismisses
					m.active = false
				} else {
					m.step = clStepProvider
				}
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.typeCursor > 0 {
					m.typeCursor--
				}
			case key.Matches(msg, keys.Down):
				if m.typeCursor < len(m.typeEntries)-1 {
					m.typeCursor++
				}
			case key.Matches(msg, keys.Space):
				if m.typeCursor < len(m.typeEntries) {
					m.typeEntries[m.typeCursor].checked = !m.typeEntries[m.typeCursor].checked
				}
			case key.Matches(msg, keys.ToggleAll):
				// Toggle all: if any are unchecked, check all; otherwise uncheck all
				allChecked := true
				for _, te := range m.typeEntries {
					if !te.checked {
						allChecked = false
						break
					}
				}
				for i := range m.typeEntries {
					m.typeEntries[i].checked = !allChecked
				}
			case msg.Type == tea.KeyEnter:
				// Validate: at least one type selected
				anySelected := false
				for _, te := range m.typeEntries {
					if te.checked {
						anySelected = true
						break
					}
				}
				if !anySelected {
					m.message = "Select at least one content type"
					m.messageIsErr = true
					return m, nil
				}
				m.buildTypeItemMaps()
				m.step = clStepItems
			}

		case clStepItems:
			ct := m.currentType()
			if m.searchInput.Focused() {
				if msg.Type == tea.KeyEsc {
					m.searchInput.Blur()
					m.searchInput.SetValue("")
					m.perTypeSearch[ct] = ""
					return m, nil
				}
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.perTypeSearch[ct] = m.searchInput.Value()
				return m, cmd
			}
			filtered := m.filteredTypeItems()
			cursor := m.perTypeCursor[ct]
			switch {
			case msg.Type == tea.KeyEsc:
				// Save state before leaving
				m.perTypeCursor[ct] = cursor
				if m.typeStepIndex == 0 {
					m.step = clStepTypes
				} else {
					m.typeStepIndex--
					// Restore search for previous type
					prevCt := m.currentType()
					m.searchInput.SetValue(m.perTypeSearch[prevCt])
				}
				return m, nil
			case key.Matches(msg, keys.Up):
				if cursor > 0 {
					m.perTypeCursor[ct] = cursor - 1
				}
			case key.Matches(msg, keys.Down):
				if cursor < len(filtered)-1 {
					m.perTypeCursor[ct] = cursor + 1
				}
			case key.Matches(msg, keys.Space):
				if cursor < len(filtered) {
					entryIdx := filtered[cursor]
					if m.isItemCompatible(entryIdx) {
						m.entries[entryIdx].selected = !m.entries[entryIdx].selected
						m.updateDestConstraints()
					}
				}
			case key.Matches(msg, keys.ToggleAll):
				// Toggle all compatible items for this type
				compatible := m.typeItemMap[ct]
				allSelected := true
				for _, idx := range compatible {
					if !m.entries[idx].selected {
						allSelected = false
						break
					}
				}
				for _, idx := range compatible {
					m.entries[idx].selected = !allSelected
				}
				m.updateDestConstraints()
			case key.Matches(msg, keys.ToggleCompat):
				m.showAllCompat = !m.showAllCompat
				// Clamp cursor after filter change
				newFiltered := m.filteredTypeItems()
				if cursor >= len(newFiltered) && len(newFiltered) > 0 {
					m.perTypeCursor[ct] = len(newFiltered) - 1
				}
			case msg.String() == "/":
				m.searchInput.SetValue(m.perTypeSearch[ct])
				m.searchInput.Focus()
			case msg.Type == tea.KeyEnter:
				m.perTypeCursor[ct] = cursor
				if m.typeStepIndex < len(m.selectedTypes)-1 {
					m.typeStepIndex++
					// Restore search for next type
					nextCt := m.currentType()
					m.searchInput.SetValue(m.perTypeSearch[nextCt])
					m.searchInput.Blur()
				} else {
					m.step = clStepName
				}
			}

		case clStepName:
			switch {
			case msg.Type == tea.KeyEsc:
				m.step = clStepItems
				return m, nil
			case msg.Type == tea.KeyTab:
				if m.nameFirst {
					m.nameFirst = false
					m.nameInput.Blur()
					m.descInput.Focus()
				} else {
					m.nameFirst = true
					m.descInput.Blur()
					m.nameInput.Focus()
				}
				return m, nil
			case msg.Type == tea.KeyEnter:
				if strings.TrimSpace(m.nameInput.Value()) == "" {
					m.message = "Name is required"
					m.messageIsErr = true
					return m, nil
				}
				m.step = clStepDest
				return m, nil
			}
			var cmd tea.Cmd
			if m.nameFirst {
				m.nameInput, cmd = m.nameInput.Update(msg)
			} else {
				m.descInput, cmd = m.descInput.Update(msg)
			}
			return m, cmd

		case clStepDest:
			switch {
			case msg.Type == tea.KeyEsc:
				m.step = clStepName
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.destCursor > 0 {
					m.destCursor--
					for m.destCursor > 0 && m.destDisabled[m.destCursor] {
						m.destCursor--
					}
				}
			case key.Matches(msg, keys.Down):
				if m.destCursor < len(m.destOptions)-1 {
					m.destCursor++
					for m.destCursor < len(m.destOptions)-1 && m.destDisabled[m.destCursor] {
						m.destCursor++
					}
				}
			case msg.Type == tea.KeyEnter:
				if m.destDisabled[m.destCursor] {
					return m, nil
				}
				m.reviewCursor = 1 // default to Create button
				m.step = clStepReview
				return m, nil
			}

		case clStepReview:
			switch {
			case msg.Type == tea.KeyEsc:
				m.step = clStepDest
				return m, nil
			case key.Matches(msg, keys.Left):
				if m.reviewCursor > 0 {
					m.reviewCursor--
				}
			case key.Matches(msg, keys.Right):
				if m.reviewCursor < 1 {
					m.reviewCursor++
				}
			case msg.Type == tea.KeyEnter:
				if m.reviewCursor == 0 {
					// Back
					m.step = clStepDest
				} else {
					// Create
					m.confirmed = true
					m.active = false
				}
				return m, nil
			}
		}
	}
	return m, nil
}

func (m createLoadoutModal) filteredEntries() []loadoutItemEntry {
	query := strings.ToLower(m.searchInput.Value())
	if query == "" {
		return m.entries
	}
	var out []loadoutItemEntry
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.item.Name), query) ||
			strings.Contains(strings.ToLower(e.item.Description), query) {
			out = append(out, e)
		}
	}
	return out
}

func (m createLoadoutModal) selectedItems() []loadoutItemEntry {
	var out []loadoutItemEntry
	for _, e := range m.entries {
		if e.selected {
			out = append(out, e)
		}
	}
	return out
}

func (m *createLoadoutModal) updateDestConstraints() {
	if len(m.destOptions) < 3 {
		return
	}
	selected := m.selectedItems()
	registries := make(map[string]bool)
	providers := make(map[string]bool)
	for _, e := range selected {
		if e.item.Registry != "" {
			registries[e.item.Registry] = true
		}
		if e.item.Provider != "" {
			providers[e.item.Provider] = true
		}
	}
	regIdx := 2
	if len(registries) > 1 {
		m.destDisabled[regIdx] = true
		m.destHints[regIdx] = "Items span multiple registries"
	} else if len(providers) > 1 {
		m.destDisabled[regIdx] = true
		m.destHints[regIdx] = "Items target multiple providers"
	} else {
		m.destDisabled[regIdx] = false
		m.destHints[regIdx] = ""
	}
	if m.destDisabled[regIdx] && m.destCursor == regIdx {
		m.destCursor--
	}
}

func (m createLoadoutModal) View() string {
	stepLabel := fmt.Sprintf("(%d of %d)", m.currentStepNum(), m.dynamicTotalSteps())
	title := titleStyle.Render("Create Loadout") + "  " + helpStyle.Render(stepLabel)
	var body string

	switch m.step {
	case clStepProvider:
		body = labelStyle.Render("Pick a provider:") + "\n\n"
		for i, prov := range m.providerList {
			prefix := "  "
			style := itemStyle
			if i == m.providerCursor {
				prefix = "> "
				style = selectedItemStyle
			}
			detected := ""
			if prov.Detected {
				detected = " " + installedStyle.Render("(detected)")
			}
			row := fmt.Sprintf("  %s%s%s", prefix, style.Render(prov.Name), detected)
			body += zone.Mark(fmt.Sprintf("modal-opt-%d", i), row) + "\n"
		}

	case clStepTypes:
		body = labelStyle.Render("Uncheck any content types you want to skip.") + "\n\n"
		for i, te := range m.typeEntries {
			checkBox := "[x]"
			if !te.checked {
				checkBox = "[ ]"
			}
			prefix := "  "
			style := itemStyle
			if i == m.typeCursor {
				prefix = "> "
				style = selectedItemStyle
			}
			countLabel := helpStyle.Render(fmt.Sprintf("(%d)", te.count))
			row := fmt.Sprintf("  %s%s %s %s",
				prefix,
				helpStyle.Render(checkBox),
				style.Render(te.ct.Label()),
				countLabel,
			)
			body += zone.Mark(fmt.Sprintf("modal-opt-%d", i), row) + "\n"
		}
		body += "\n" + helpStyle.Render("space toggle \u2022 a toggle all \u2022 enter next")
		if m.message != "" && m.messageIsErr {
			body += "\n" + errorMsgStyle.Render(m.message)
		}

	case clStepItems:
		ct := m.currentType()
		selCount := m.currentTypeSelectedCount()
		body = labelStyle.Render(fmt.Sprintf("Select %s (%d selected)", ct.Label(), selCount)) + "\n"
		if m.searchInput.Focused() {
			body += zone.Mark("modal-field-search", m.searchInput.View()) + "\n"
		} else {
			body += "\n"
		}

		filtered := m.filteredTypeItems()
		cursor := m.perTypeCursor[ct]
		if len(filtered) == 0 {
			body += helpStyle.Render(fmt.Sprintf("  No %s available for %s", ct.Label(), m.prefilledProvider))
		} else {
			innerH := createLoadoutModalHeight - 9
			start := 0
			if len(filtered) > innerH {
				start = cursor - innerH/2
				if start < 0 {
					start = 0
				}
				if start+innerH > len(filtered) {
					start = len(filtered) - innerH
				}
			}
			end := start + innerH
			if end > len(filtered) {
				end = len(filtered)
			}
			for vi, fi := range filtered[start:end] {
				e := m.entries[fi]
				absIdx := start + vi
				compatible := m.isItemCompatible(fi)

				checkBox := "[ ]"
				if e.selected {
					checkBox = "[x]"
				}
				prefix := "  "
				style := itemStyle
				if absIdx == cursor {
					prefix = "> "
					style = selectedItemStyle
				}

				source := ""
				if e.item.Registry != "" {
					source = " (" + e.item.Registry + ")"
				} else if e.item.Library {
					source = " (library)"
				}

				if !compatible {
					// Incompatible: muted + strikethrough + suffix
					row := fmt.Sprintf("  %s%s %s%s (incompatible)",
						prefix,
						helpStyle.Render(checkBox),
						helpStyle.Render(strikethrough(e.item.Name)),
						helpStyle.Render(source),
					)
					body += zone.Mark(fmt.Sprintf("modal-opt-%d", absIdx), row) + "\n"
				} else {
					row := fmt.Sprintf("  %s%s %s%s",
						prefix,
						helpStyle.Render(checkBox),
						style.Render(e.item.Name),
						helpStyle.Render(source),
					)
					body += zone.Mark(fmt.Sprintf("modal-opt-%d", absIdx), row) + "\n"
				}
			}
		}
		filterMode := "compatible only"
		if m.showAllCompat {
			filterMode = "showing all"
		}
		body += "\n" + helpStyle.Render(fmt.Sprintf("space select \u2022 a all \u2022 t filter (%s) \u2022 / search \u2022 enter next", filterMode))

	case clStepName:
		body = labelStyle.Render("Name your loadout:") + "\n\n"
		body += zone.Mark("modal-field-name", m.nameInput.View()) + "\n"
		body += zone.Mark("modal-field-desc", m.descInput.View()) + "\n"
		body += "\n" + helpStyle.Render("tab switch field • enter next")
		if m.message != "" && m.messageIsErr {
			body += "\n" + errorMsgStyle.Render(m.message)
		}

	case clStepDest:
		body = labelStyle.Render("Choose destination:") + "\n\n"
		for i, opt := range m.destOptions {
			prefix := "  "
			style := itemStyle
			if m.destDisabled[i] {
				style = helpStyle
			} else if i == m.destCursor {
				prefix = "> "
				style = selectedItemStyle
			}
			row := fmt.Sprintf("  %s%s", prefix, style.Render(opt))
			if m.destDisabled[i] && m.destHints[i] != "" {
				row += "\n      " + helpStyle.Render(m.destHints[i])
			}
			body += zone.Mark(fmt.Sprintf("modal-opt-%d", i), row) + "\n"
		}
		body += "\n" + helpStyle.Render("enter next • esc back")

	case clStepReview:
		body = m.reviewBody()
	}

	content := title + "\n\n" + body
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(createLoadoutModalWidth).
		Height(createLoadoutModalHeight).
		Render(content)
}

// reviewBody builds the review step content with grouped summary and buttons.
func (m createLoadoutModal) reviewBody() string {
	innerWidth := createLoadoutModalWidth - 6 // borders + padding

	// Provider display name
	provName := m.prefilledProvider
	for _, p := range m.providerList {
		if p.Slug == m.prefilledProvider {
			provName = p.Name
			break
		}
	}

	// Destination label
	destLabel := "Project"
	if m.destCursor < len(m.destOptions) {
		destLabel = m.destOptions[m.destCursor]
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Name:"), m.nameInput.Value()))
	if desc := strings.TrimSpace(m.descInput.Value()); desc != "" {
		b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Desc:"), desc))
	}
	b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Provider:"), provName))
	b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Dest:"), destLabel))

	// Group selected items by type
	grouped := make(map[catalog.ContentType][]string)
	hasExecutable := false
	for _, e := range m.entries {
		if !e.selected {
			continue
		}
		name := e.item.Name
		if e.item.Registry != "" {
			name += " (" + e.item.Registry + ")"
		}
		grouped[e.item.Type] = append(grouped[e.item.Type], name)
		if e.item.Type == catalog.Hooks || e.item.Type == catalog.MCP {
			hasExecutable = true
		}
	}

	if len(grouped) == 0 {
		b.WriteString("\n" + warningStyle.Render("  No items selected"))
	} else {
		b.WriteString("\n  " + labelStyle.Render("Contents:") + "\n")
		maxNamesWidth := innerWidth - 8 // indent + type label overhead
		for _, ct := range catalog.AllContentTypes() {
			names := grouped[ct]
			if len(names) == 0 {
				continue
			}
			line := strings.Join(names, ", ")
			if len(line) > maxNamesWidth && len(names) > 3 {
				line = strings.Join(names[:3], ", ") + fmt.Sprintf(" + %d more", len(names)-3)
			}
			b.WriteString(fmt.Sprintf("    %s (%d): %s\n",
				ct.Label(), len(names), helpStyle.Render(line)))
		}
	}

	// Security warning for hooks/MCP
	if hasExecutable {
		b.WriteString("\n" + warningStyle.Render("  !! Security Notice !!") + "\n")
		b.WriteString(helpStyle.Render("  This loadout includes executable content.") + "\n")
		b.WriteString(helpStyle.Render("  Review commands before installing."))
	}

	body := b.String()

	// Buttons pinned to bottom
	contentLines := strings.Count(body, "\n") + 1
	innerH := createLoadoutModalHeight - 6 // title+spacing+border+padding
	spacer := innerH - contentLines - 1
	if spacer < 0 {
		spacer = 0
	}
	body += strings.Repeat("\n", spacer)
	body += renderButtons("Back", "Create", m.reviewCursor, innerWidth)

	return body
}

// strikethrough applies Unicode combining strikethrough to each character.
func strikethrough(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(r)
		b.WriteRune('\u0336') // combining long stroke overlay
	}
	return b.String()
}

func (m createLoadoutModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
}
