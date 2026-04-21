package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// metaPanelData holds pre-computed display strings for the metadata panel.
type metaPanelData struct {
	installed  string // "CC,GC,Cu" or "--"
	typeDetail string // type-specific detail (hooks/MCP/loadouts) or ""
	canInstall bool   // true when item is in library and has at least one uninstalled provider
}

// computeMetaPanelData computes installed status and type-specific detail for an item.
func computeMetaPanelData(item catalog.ContentItem, providers []provider.Provider, repoRoot string) metaPanelData {
	var abbrevs []string
	for _, prov := range providers {
		if installer.CheckStatus(item, prov, repoRoot) == installer.StatusInstalled {
			abbrevs = append(abbrevs, providerAbbrev(prov.Slug))
		}
	}
	installed := "--"
	if len(abbrevs) > 0 {
		installed = strings.Join(abbrevs, ",")
	}

	// Any local item (library or content root) can be installed — not registry-only items.
	canInstall := false
	if item.Library || item.Registry == "" {
		for _, prov := range providers {
			if prov.Detected && installer.CheckStatus(item, prov, repoRoot) != installer.StatusInstalled {
				canInstall = true
				break
			}
		}
	}

	return metaPanelData{
		installed:  installed,
		typeDetail: computeTypeDetail(item),
		canInstall: canInstall,
	}
}

// metaBarLinesFor reports how many lines renderMetaPanel will emit for the
// given item. Returns metaBarLinesBase (3) for non-MOAT items so the
// hundreds of existing golden snapshots stay unchanged. MOAT items (any
// TrustTier, PrivateRepo, or Recalled) gain exactly one Line 4 that
// always carries both Trust and Visibility fields — recalled items now
// collapse their reason/issuer/URL details into the Trust Inspector
// rather than a multi-line banner, eliminating overflow jank at narrow
// widths.
//
// Nil-item case returns metaBarLinesBase — the "no selection" placeholder
// renders blank lines matching the baseline so height math is stable.
func metaBarLinesFor(item *catalog.ContentItem) int {
	if item == nil {
		return metaBarLinesBase
	}
	if item.TrustTier != catalog.TrustTierUnknown || item.PrivateRepo || item.Recalled {
		return metaBarLinesBase + 1
	}
	return metaBarLinesBase
}

// renderMetaPanel renders the variable-height metadata content for a content
// item. Emits 3 lines for non-MOAT items, 4 lines for MOAT items with a
// trust surface, and 5 lines when the item is Recalled (adds a revocation
// banner). Height contract: lipgloss.Height(result) always equals
// metaBarLinesFor(item) — callers rely on this for layout math.
func renderMetaPanel(item *catalog.ContentItem, data metaPanelData, width int) string {
	if item == nil {
		blank := strings.Repeat(" ", width)
		return blank + "\n" + blank + "\n" + blank
	}

	// pad clamps a line to width. Shared by all line builders below so the
	// returned string's Height matches metaBarLinesFor(item) exactly.
	pad := func(s string) string {
		s = lipgloss.NewStyle().MaxWidth(width).Render(s)
		if g := width - lipgloss.Width(s); g > 0 {
			s += strings.Repeat(" ", g)
		}
		return s
	}

	chip := func(key, val string, w int) string {
		valW := w - len(key) - 2
		val = padRight(truncate(sanitizeLine(val), valW), valW)
		return boldStyle.Render(key+": ") + mutedStyle.Render(val)
	}

	gap := "  "

	tryAdd := func(line, c string) string {
		candidate := line + gap + c
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
		return line
	}

	// --- Line 1: name (40), type, files, origin, installed ---
	nameMaxW := 40
	displayName := truncate(sanitizeLine(itemDisplayName(*item)), nameMaxW)
	line1 := " " + boldStyle.Render(padRight(displayName, nameMaxW))
	line1 += "  " + chip("Type", typeLabel(item.Type), 14)
	line1 = tryAdd(line1, chip("Files", itoa(len(item.Files)), 9))

	origin := "syllago"
	if item.Meta != nil && item.Meta.SourceProvider != "" {
		origin = item.Meta.SourceProvider
	} else if item.Provider != "" {
		origin = item.Provider
	}
	line1 = tryAdd(line1, chip("Origin", origin, 19))

	line1 = tryAdd(line1, boldStyle.Render("Installed: ")+mutedStyle.Render(data.installed))

	// --- Line 2: scope, registry, path ---
	scope := "project"
	if item.Meta != nil && item.Meta.SourceScope != "" {
		scope = item.Meta.SourceScope
	} else if item.Library {
		scope = "global"
	}
	line2 := " " + chip("Scope", scope, 15)

	regName := "not in a registry"
	if item.Registry != "" {
		regName = item.Registry
	} else if item.Meta != nil && item.Meta.SourceRegistry != "" {
		regName = item.Meta.SourceRegistry
	}
	line2 += gap + chip("Registry", regName, 30)

	if item.Path != "" {
		path := item.Path
		if home, err := homeDir(); err == nil && strings.HasPrefix(path, home) {
			path = "~" + path[len(home):]
		}
		pathW := max(20, width-lipgloss.Width(line2)-10)
		line2 = tryAdd(line2, boldStyle.Render("Path: ")+mutedStyle.Render(truncateMiddle(path, pathW)))
	}

	// --- Line 3: type-specific detail + rename button ---
	line3 := ""
	if data.typeDetail != "" {
		segments := strings.Split(sanitizeLine(data.typeDetail), " · ")
		styled := " "
		for i, seg := range segments {
			if i > 0 {
				styled += gap
			}
			if idx := strings.Index(seg, ": "); idx >= 0 {
				styled += boldStyle.Render(seg[:idx+2]) + mutedStyle.Render(seg[idx+2:])
			} else {
				styled += mutedStyle.Render(seg)
			}
		}
		line3 = styled
	}

	// Per-item action buttons ordered: [i] Install, [x] Uninstall, [d] Remove, [e] Edit
	// Only show buttons that are actionable for this item.
	var btns []string
	if data.canInstall {
		btns = append(btns, zone.Mark("meta-install", activeButtonStyle.Render("[i] Install")))
	}
	if data.installed != "--" {
		btns = append(btns, zone.Mark("meta-uninstall", activeButtonStyle.Render("[x] Uninstall")))
	}
	if item.Library || item.Registry == "" {
		btns = append(btns, zone.Mark("meta-remove", activeButtonStyle.Render("[d] Remove")))
	}
	btns = append(btns, zone.Mark("meta-edit", activeButtonStyle.Render("[e] Edit")))
	btnRow := strings.Join(btns, " ")
	btnRowW := lipgloss.Width(btnRow)

	// Truncate type detail to ensure buttons always fit — buttons must never be clipped.
	maxDetailW := max(0, width-btnRowW-2) // -2 for minimum gap
	line3W := lipgloss.Width(line3)
	if line3W > maxDetailW {
		line3 = lipgloss.NewStyle().MaxWidth(maxDetailW).Render(line3)
		line3W = lipgloss.Width(line3)
	}
	btnGap := max(1, width-line3W-btnRowW)
	line3 += strings.Repeat(" ", btnGap) + btnRow

	out := pad(line1) + "\n" + pad(line2) + "\n" + pad(line3)

	// --- Line 4: trust + visibility chips (MOAT items only) ---
	// Rendered when the item has any trust surface: a known TrustTier, a
	// Recalled flag, or PrivateRepo. Zero-value items (git registries,
	// local content) skip this line per AD-7 "absence is not a negative
	// signal" — their layout stays at 3 lines.
	//
	// Both Trust and Visibility fields always render when shown, so the
	// row layout is stable whether an item is verified, unsigned, or
	// recalled. Recalled items collapse reason/issuer/URL details into
	// the Trust Inspector (opened via the clickable Trust field or [t])
	// rather than emitting a second multi-line banner.
	showChips := item.TrustTier != catalog.TrustTierUnknown || item.PrivateRepo || item.Recalled
	if showChips {
		// Trust value style: recalled→danger, verified→success, else muted.
		var trustValStyle lipgloss.Style
		badge := catalog.UserFacingBadge(item.TrustTier, item.Recalled)
		switch badge {
		case catalog.TrustBadgeRecalled:
			trustValStyle = trustRecalledStyle
		case catalog.TrustBadgeVerified:
			trustValStyle = trustVerifiedStyle
		default:
			trustValStyle = mutedStyle
		}

		var trustValue string
		if item.Recalled {
			// Collapse recalled details to a single short summary. Reason
			// may be empty when a feed omits it; we still surface the
			// "Recalled" state so users see the warning.
			trustValue = "Recalled"
			if item.RecallReason != "" {
				trustValue += " \u2014 " + sanitizeLine(item.RecallReason)
			}
		} else {
			trustValue = sanitizeLine(catalog.TrustDescription(item.TrustTier, false, ""))
		}

		trustField := boldStyle.Render("Trust: ") + trustValStyle.Render(trustValue)
		line4 := " " + zone.Mark("meta-trust", trustField)

		// Visibility always renders when the chip row is shown so layout
		// stays stable across public/private items in a mixed list.
		visibility := "Public"
		visibilityStyle := mutedStyle
		if item.PrivateRepo {
			visibility = "Private"
			visibilityStyle = privateIndicatorStyle
		}
		line4 += gap + boldStyle.Render("Visibility: ") + visibilityStyle.Render(visibility)

		if item.Recalled {
			// Give users an explicit affordance to see the full recall
			// breakdown (source, issuer, details URL) in the inspector.
			line4 += gap + mutedStyle.Render("[t] Inspect trust")
		}

		out += "\n" + pad(line4)
	}

	return out
}
