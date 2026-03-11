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
	clStepItems                             // checkbox list of items
	clStepName                              // name + description text inputs
	clStepDest                              // pick destination
)

const (
	createLoadoutModalWidth  = 64
	createLoadoutModalHeight = 24
)

// loadoutItemEntry is one item in the checkbox picker.
type loadoutItemEntry struct {
	item     catalog.ContentItem
	selected bool
}

type createLoadoutModal struct {
	active     bool
	step       createLoadoutStep
	totalSteps int

	// Context passed in at creation
	prefilledProvider string // non-empty = skip provider step
	scopeRegistry    string // non-empty = scope items to this registry

	// Step 1: provider picker
	providerList   []provider.Provider
	providerCursor int

	// Step 2: item checkbox list
	entries     []loadoutItemEntry
	itemCursor  int
	searchInput textinput.Model

	// Step 3: name/desc
	nameInput textinput.Model
	descInput textinput.Model
	nameFirst bool // true = nameInput focused

	// Step 4: destination
	destOptions  []string // "Project", "Library", optionally "Registry: <name>"
	destCursor   int
	destDisabled []bool   // parallel: true = option grayed out
	destHints    []string // parallel: explanation for disabled options

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
		m.step = clStepItems
		m.totalSteps = 3
	} else {
		m.step = clStepProvider
		m.totalSteps = 4
	}

	return m
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
	if m.prefilledProvider != "" {
		switch m.step {
		case clStepItems:
			return 1
		case clStepName:
			return 2
		case clStepDest:
			return 3
		}
	}
	return int(m.step) + 1
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
				m.step = clStepItems
			}

		case clStepItems:
			if m.searchInput.Focused() {
				if msg.Type == tea.KeyEsc {
					m.searchInput.Blur()
					m.searchInput.SetValue("")
					return m, nil
				}
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
			switch {
			case msg.Type == tea.KeyEsc:
				if m.prefilledProvider != "" && m.totalSteps == 3 {
					m.active = false
				} else {
					m.step = clStepProvider
				}
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.itemCursor > 0 {
					m.itemCursor--
				}
			case key.Matches(msg, keys.Down):
				filtered := m.filteredEntries()
				if m.itemCursor < len(filtered)-1 {
					m.itemCursor++
				}
			case key.Matches(msg, keys.Space):
				filtered := m.filteredEntries()
				if m.itemCursor < len(filtered) {
					targetItem := filtered[m.itemCursor].item
					for i, e := range m.entries {
						if e.item.Path == targetItem.Path {
							m.entries[i].selected = !m.entries[i].selected
							break
						}
					}
					m.updateDestConstraints()
				}
			case msg.String() == "/":
				m.searchInput.Focus()
			case msg.Type == tea.KeyEnter:
				m.step = clStepName
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
				m.active = false
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
	stepLabel := fmt.Sprintf("(%d of %d)", m.currentStepNum(), m.totalSteps)
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
			body += fmt.Sprintf("  %s%s%s\n", prefix, style.Render(prov.Name), detected)
		}

	case clStepItems:
		filtered := m.filteredEntries()
		searchLine := m.searchInput.View()
		body = searchLine + "\n\n"
		if len(filtered) == 0 {
			body += helpStyle.Render("  No items found.")
		}
		innerH := createLoadoutModalHeight - 8
		shown := filtered
		if len(shown) > innerH {
			start := m.itemCursor - innerH/2
			if start < 0 {
				start = 0
			}
			if start+innerH > len(shown) {
				start = len(shown) - innerH
			}
			end := start + innerH
			if end > len(shown) {
				end = len(shown)
			}
			shown = shown[start:end]
		}
		for i, e := range shown {
			checkBox := "[ ]"
			if e.selected {
				checkBox = "[x]"
			}
			prefix := "  "
			style := itemStyle
			absIdx := i
			if len(shown) < len(filtered) {
				absIdx = m.itemCursor - innerH/2 + i
				if absIdx < 0 {
					absIdx = 0
				}
			}
			if absIdx == m.itemCursor {
				prefix = "> "
				style = selectedItemStyle
			}
			source := ""
			if e.item.Registry != "" {
				source = helpStyle.Render(" (" + e.item.Registry + ")")
			} else if e.item.Library {
				source = helpStyle.Render(" (library)")
			}
			typeLabel := helpStyle.Render("[" + string(e.item.Type) + "]")
			body += fmt.Sprintf("  %s%s %s %s%s\n",
				prefix,
				helpStyle.Render(checkBox),
				typeLabel,
				style.Render(e.item.Name),
				source,
			)
		}
		body += "\n" + helpStyle.Render("space select • / filter • enter next")

	case clStepName:
		body = labelStyle.Render("Name your loadout:") + "\n\n"
		body += m.nameInput.View() + "\n"
		body += m.descInput.View() + "\n"
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
			body += fmt.Sprintf("  %s%s\n", prefix, style.Render(opt))
			if m.destDisabled[i] && m.destHints[i] != "" {
				body += "      " + helpStyle.Render(m.destHints[i]) + "\n"
			}
		}
		body += "\n" + helpStyle.Render("enter confirm • esc back")
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

func (m createLoadoutModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
}
