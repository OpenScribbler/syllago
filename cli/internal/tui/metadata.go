package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// metadataModel renders the metadata bar showing selected item info.
// Three rows, full width, always visible above the content split.
type metadataModel struct {
	item  *catalog.ContentItem // nil = no selection (show summary)
	ct    catalog.ContentType
	items []catalog.ContentItem // all items of current type (for summary)
	width int
}

// View renders the metadata bar. When item is nil, shows a type summary.
func (m metadataModel) View() string {
	if m.item == nil {
		return m.viewSummary()
	}
	return m.viewItem()
}

// viewSummary renders the summary when no item is selected.
func (m metadataModel) viewSummary() string {
	count := len(m.items)
	row1 := metaNameStyle.Render(fmt.Sprintf("%s (%d items)", displayTypeName(m.ct), count))
	row1 = m.padRow(row1, m.renderSummaryActions())

	// Count sources
	var fromReg, fromLib int
	for _, it := range m.items {
		if it.Registry != "" {
			fromReg++
		} else if it.Library {
			fromLib++
		}
	}
	parts := []string{fmt.Sprintf("Type: %s", displayTypeName(m.ct))}
	if fromReg > 0 {
		parts = append(parts, fmt.Sprintf("%d from registries", fromReg))
	}
	if fromLib > 0 {
		parts = append(parts, fmt.Sprintf("%d from library", fromLib))
	}
	row2 := helpStyle.Render(strings.Join(parts, " * "))

	row3 := helpStyle.Render(typeDescription(m.ct))

	inner := row1 + "\n" + row2 + "\n" + row3
	return metadataStyle.Width(m.width).Render(inner)
}

// viewItem renders the metadata bar for a specific selected item.
func (m metadataModel) viewItem() string {
	it := m.item

	// Row 1: Name + action buttons
	row1 := metaNameStyle.Render(it.Name)
	row1 = m.padRow(row1, m.renderItemActions())

	// Row 2: Type + Source + type-specific fields
	parts := []string{displayTypeName(it.Type)}
	if it.Source != "" {
		parts = append(parts, it.Source)
	}
	parts = append(parts, m.typeSpecificFields()...)
	row2 := helpStyle.Render(strings.Join(parts, " * "))

	// Row 3: Description or type-specific summary
	row3 := m.row3Content()

	inner := row1 + "\n" + row2 + "\n" + row3
	return metadataStyle.Width(m.width).Render(inner)
}

// padRow right-aligns actions on the same line as the left content.
func (m metadataModel) padRow(left, right string) string {
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	innerWidth := m.width - 4 // border + padding
	gap := innerWidth - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// renderItemActions returns action buttons for a selected item.
func (m metadataModel) renderItemActions() string {
	actions := []string{
		metaActionStyle.Render("[i] Install"),
		metaActionStyle.Render("[u] Uninstall"),
		metaActionStyle.Render("[c] Copy"),
	}
	// Drop actions if too narrow
	innerWidth := m.width - 4
	result := strings.Join(actions, "  ")
	if lipgloss.Width(result) > innerWidth/2 {
		// Show fewer actions
		actions = actions[:2]
		result = strings.Join(actions, "  ")
	}
	return result
}

// renderSummaryActions returns action buttons for the summary view.
func (m metadataModel) renderSummaryActions() string {
	return metaActionStyle.Render("[+ Add]") + "  " + metaActionStyle.Render("[* New]")
}

// typeSpecificFields returns extra row 2 fields based on content type.
func (m metadataModel) typeSpecificFields() []string {
	it := m.item
	if it == nil {
		return nil
	}
	switch it.Type {
	case catalog.Skills:
		return []string{fmt.Sprintf("Files: %d", len(it.Files))}
	case catalog.Agents:
		return []string{"Permission: default"}
	case catalog.MCP:
		transport := "stdio"
		return []string{fmt.Sprintf("Transport: %s", transport)}
	case catalog.Rules:
		return []string{"Scope: project"}
	case catalog.Hooks:
		return []string{"Events: custom"}
	case catalog.Commands:
		return []string{"Effort: standard"}
	}
	return nil
}

// row3Content returns the third row content based on type.
func (m metadataModel) row3Content() string {
	it := m.item
	if it == nil {
		return ""
	}
	desc := it.Description
	if desc == "" {
		desc = "No description"
	}
	maxW := m.width - 6
	if maxW > 0 && len(desc) > maxW {
		desc = desc[:maxW-3] + "..."
	}
	return helpStyle.Render(desc)
}

// displayTypeName returns a human-readable type name.
func displayTypeName(ct catalog.ContentType) string {
	switch ct {
	case catalog.Skills:
		return "Skills"
	case catalog.Agents:
		return "Agents"
	case catalog.MCP:
		return "MCP Configs"
	case catalog.Rules:
		return "Rules"
	case catalog.Hooks:
		return "Hooks"
	case catalog.Commands:
		return "Commands"
	case catalog.Loadouts:
		return "Loadouts"
	case catalog.Library:
		return "Library"
	default:
		return string(ct)
	}
}

// typeDescription returns a short description for a content type.
func typeDescription(ct catalog.ContentType) string {
	switch ct {
	case catalog.Skills:
		return "Reusable skill definitions that extend AI tool capabilities"
	case catalog.Agents:
		return "Autonomous agent configurations for specialized tasks"
	case catalog.MCP:
		return "Model Context Protocol server configurations"
	case catalog.Rules:
		return "Project rules and coding standards for AI tools"
	case catalog.Hooks:
		return "Lifecycle hooks for AI tool events"
	case catalog.Commands:
		return "Custom slash commands for AI tools"
	case catalog.Loadouts:
		return "Curated bundles of content for a specific provider"
	default:
		return ""
	}
}
