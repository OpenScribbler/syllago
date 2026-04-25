package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// ruleTargetStatus is a single row for the D16 per-target breakdown in the
// rule metapanel: which target file an InstalledRuleAppend record points at
// plus its looked-up PerTargetState. Callers build the slice from
// installer.Installed.RuleAppends filtered by library ID.
type ruleTargetStatus struct {
	TargetFile string
	State      installcheck.PerTargetState
}

// metaPanelData holds pre-computed display strings for the metadata panel.
type metaPanelData struct {
	installed   string // "CC,GC,Cu" or "--"
	typeDetail  string // type-specific detail (hooks/MCP/loadouts) or ""
	canInstall  bool   // true when item is in library and has at least one uninstalled provider
	ruleRecords []ruleTargetStatus
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
	// MOAT-materialized items are intentionally excluded for now: the TUI install
	// pipeline (installer.Install) reads from item.Path, but materialized items
	// have empty Path because the content blob is only fetched at install time
	// from the manifest's SourceURI. The CLI install path knows how to do that
	// fetch; the TUI does not yet. Until that is plumbed, hiding the button is
	// the honest signal — the toast in handleInstall directs the user to the CLI.
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

// metaBarLinesFor reports the BASE number of lines renderMetaPanel will emit
// for the given item (metaBarLinesBase = 4 across every item). MOAT state
// no longer adds a row; trust+visibility chips share Line 4 with the action
// buttons so the handler slot on Line 3 stays at a predictable position
// whether the content type supplies a handler row or not.
//
// The parameter is kept even though it is ignored so existing call sites
// (gallery, explorer, library views) don't need to change signature and
// future variants (e.g. a nil-placeholder with shorter height) can plug
// in without churn.
//
// D16 per-target rule breakdown lines are EXTRA lines beyond this base —
// callers that need the total height (library + explorer when displaying a
// rule item) must add metaPanelExtraLines(data) to this value.
func metaBarLinesFor(item *catalog.ContentItem) int {
	_ = item
	return metaBarLinesBase
}

// metaPanelExtraLines reports how many additional lines the metapanel emits
// for data-driven extensions — currently just the D16 rule per-target
// breakdown. Returns 0 when data has no rule records attached.
//
// Layout: one header line ("Installed at:") plus one line per record.
func metaPanelExtraLines(data metaPanelData) int {
	if len(data.ruleRecords) == 0 {
		return 0
	}
	return 1 + len(data.ruleRecords)
}

// trustValueMaxW is the reserved width for the Trust chip value on Line 4.
// It equals len("Dual attested") — the widest short tier label — so the
// Visibility chip always starts at the same column regardless of which
// tier or state the item is in. Trust values shorter than this are padded
// on the right; longer values (shouldn't occur with ShortLabel, but the
// "Revoked" family could grow if we ever add a suffix here) are
// truncated to keep the column grid stable.
const trustValueMaxW = 13

// renderMetaPanel renders the metadata panel as a fixed 4-line block. Every
// item emits the same number of lines so callers' height math is stable:
//
//	Line 1: name, type, files, origin, installed
//	Line 2: scope, registry, path
//	Line 3: type-specific handler (hooks show Event/Tool/Handler; skills &
//	        commands emit a blank line in this slot)
//	Line 4: trust + visibility (fixed columns) + action buttons (right-
//	        aligned). Non-MOAT items omit the Trust/Visibility chips but
//	        the buttons still sit on Line 4 at the same position.
//
// Extended revocation details (reason, revoker, details URL) live in the
// Trust Inspector modal — not the metapanel — so the column width stays
// stable at any reason length.
func renderMetaPanel(item *catalog.ContentItem, data metaPanelData, width int) string {
	pad := func(s string) string {
		s = lipgloss.NewStyle().MaxWidth(width).Render(s)
		if g := width - lipgloss.Width(s); g > 0 {
			s += strings.Repeat(" ", g)
		}
		return s
	}

	if item == nil {
		blank := pad("")
		return blank + "\n" + blank + "\n" + blank + "\n" + blank
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

	// --- Line 3: type-specific handler detail (blank when absent) ---
	// Skills, rules, commands, etc. have no handler row. Their Line 3 is
	// a single leading space so the height stays 4 lines and buttons on
	// Line 4 below never migrate upward.
	line3 := " "
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
		// Clamp handler detail to the row width so buttons below aren't
		// pushed around by an unusually long handler signature.
		if lipgloss.Width(styled) > width {
			styled = lipgloss.NewStyle().MaxWidth(width).Render(styled)
		}
		line3 = styled
	}

	// --- Line 4: trust + visibility (fixed columns) + buttons (right) ---
	// Buttons sit at the right edge so their position is stable across
	// content types. Trust+Visibility (if any) occupy fixed left columns
	// so the Visibility chip never floats with tier-label length.
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

	// Left portion: Trust + Visibility chips. Always render — for non-MOAT
	// items we show "Unknown"/"Public" so the row is stable across item
	// switches and users learn where to look regardless of tier state. Full
	// detail still lives in the Trust Inspector, opened via [t].
	var trustValStyle lipgloss.Style
	badge := catalog.UserFacingBadge(item.TrustTier, item.Revoked)
	switch badge {
	case catalog.TrustBadgeRevoked:
		trustValStyle = trustRevokedStyle
	case catalog.TrustBadgeVerified:
		trustValStyle = trustVerifiedStyle
	default:
		trustValStyle = mutedStyle
	}

	trustValue := item.TrustTier.ShortLabel()
	if trustValue == "" {
		trustValue = "Unknown"
	}
	if item.Revoked {
		trustValue = "Revoked"
	}
	// Pin column by padding to the reserved width. truncate() guards
	// against accidental overrun if a future label exceeds the max.
	trustValue = truncate(trustValue, trustValueMaxW)
	trustValuePadded := padRight(trustValue, trustValueMaxW)
	trustField := boldStyle.Render("Trust: ") + trustValStyle.Render(trustValuePadded)
	leftPortion := " " + zone.Mark("meta-trust", trustField)

	visibility := "Public"
	visibilityStyle := mutedStyle
	if item.PrivateRepo {
		visibility = "Private"
		visibilityStyle = privateIndicatorStyle
	}
	leftPortion += gap + boldStyle.Render("Visibility: ") + visibilityStyle.Render(visibility)

	// Gap between left portion and buttons; always at least one space.
	leftW := lipgloss.Width(leftPortion)
	btnGap := max(1, width-leftW-btnRowW)
	// If the chips don't fit with the buttons at this width, truncate the
	// chip cluster from the right so buttons are never clipped.
	if leftW+btnRowW+1 > width {
		maxLeftW := max(0, width-btnRowW-1)
		leftPortion = lipgloss.NewStyle().MaxWidth(maxLeftW).Render(leftPortion)
		btnGap = max(1, width-lipgloss.Width(leftPortion)-btnRowW)
	}
	line4 := leftPortion + strings.Repeat(" ", btnGap) + btnRow

	out := pad(line1) + "\n" + pad(line2) + "\n" + pad(line3) + "\n" + pad(line4)

	// D16 per-target rule breakdown (optional). Rendered as a header line
	// followed by one line per InstalledRuleAppend record. Each status line
	// lists the target file (truncated by the pad() width clamp) and its
	// PerTargetState rendered via ruleStatusString. Order mirrors the
	// caller-supplied slice — typically the order from installer.Installed.
	for _, extra := range renderRuleRecords(data.ruleRecords, width) {
		out += "\n" + pad(extra)
	}
	return out
}

// renderRuleRecords builds the "Installed at:" section lines for a rule's
// D16 per-target breakdown. Returns an empty slice when no records are
// provided. Status coloring: Clean → successColor; Modified → dangerColor.
// Target files are rendered as basenames so the section stays readable at
// any width; the full path lives in installed.json (and the Trust Inspector
// surfaces similar detail elsewhere). The "Installed at:" header stays
// muted-bold so it visually groups with the other field labels.
func renderRuleRecords(records []ruleTargetStatus, width int) []string {
	if len(records) == 0 {
		return nil
	}
	_ = width // clamping happens in the parent pad() helper
	lines := make([]string, 0, 1+len(records))
	lines = append(lines, " "+boldStyle.Render("Installed at:"))
	for _, r := range records {
		status := ruleStatusString(r.State)
		style := mutedStyle
		switch r.State.State {
		case installcheck.StateClean:
			style = trustVerifiedStyle
		case installcheck.StateModified:
			style = trustRevokedStyle
		}
		name := filepath.Base(r.TargetFile)
		lines = append(lines, "   "+mutedStyle.Render(name)+"  "+style.Render(status))
	}
	return lines
}

// ruleStatusString converts a D16 PerTargetState to the human-readable
// string used in the metapanel per-target breakdown. These strings are the
// contract surface consumed by both the UI and the acceptance test in
// metapanel_rule_test.go — do not rename without updating both.
func ruleStatusString(pts installcheck.PerTargetState) string {
	if pts.State == installcheck.StateClean {
		return "Clean"
	}
	switch pts.Reason {
	case installcheck.ReasonEdited:
		return "Modified · edited"
	case installcheck.ReasonMissing:
		return "Modified · missing"
	case installcheck.ReasonUnreadable:
		return "Modified · unreadable"
	default:
		return "Modified"
	}
}
