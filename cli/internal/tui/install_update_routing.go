package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// wizardScanResult is the D16 verification snapshot the wizard uses to decide
// which (if any) D17 modal to open before dispatching installResultMsg. Only
// the first matching RuleAppend record for the item's library ID is used;
// multi-target D17 is a later-phase concern.
type wizardScanResult struct {
	state      installcheck.State
	reason     installcheck.Reason
	targetFile string
	// recordedHash / newHash are shown by installUpdateModal so the user can
	// distinguish versions. Both are canonical "<algo>:<hex>" strings; the
	// modal's shortHashLabel trims them to display length.
	recordedHash string
	newHash      string
	// readErr is surfaced only for StateModified/ReasonUnreadable.
	readErr string
}

// installWizardScanFn is the test-overridable seam used by the install
// wizard to discover D17 state. Production impl loads installed.json +
// library and runs installcheck.Scan; tests override to inject synthetic
// (State, Reason) tuples without seeding a project root.
var installWizardScanFn = defaultInstallWizardScan

// defaultInstallWizardScan is the production scan: loads installed.json
// from projectRoot, loads the library rule from ~/.syllago/content/rules,
// runs installcheck.Scan, and returns the first RuleAppend record matching
// the item. Returns StateFresh when no record matches or loading fails —
// any scan failure is non-fatal (it just skips the D17 modal routing).
func defaultInstallWizardScan(projectRoot string, item catalog.ContentItem) wizardScanResult {
	if item.Type != catalog.Rules {
		return wizardScanResult{state: installcheck.StateFresh}
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil || inst == nil {
		return wizardScanResult{state: installcheck.StateFresh}
	}
	// Look up the library rule by the item's directory path. The item's
	// disk layout is <library>/rules/<slug>/ containing .syllago.yaml; we
	// load it to get LibraryID + History for the scan.
	loaded, err := rulestore.LoadRule(item.Path)
	if err != nil {
		return wizardScanResult{state: installcheck.StateFresh}
	}
	library := map[string]*rulestore.Loaded{loaded.Meta.ID: loaded}
	scan := installcheck.Scan(inst, library)
	// Return the first matching record (D17 in Phase 7 is single-target).
	for key, pts := range scan.PerRecord {
		if key.LibraryID != loaded.Meta.ID {
			continue
		}
		out := wizardScanResult{
			state:      pts.State,
			reason:     pts.Reason,
			targetFile: key.TargetFile,
		}
		// Populate display hashes when available. The recorded hash is the
		// installed record's VersionHash; the "new" hash is the current
		// head of the rule's history (rule.Meta.Head).
		for _, r := range inst.RuleAppends {
			if r.LibraryID == key.LibraryID && r.TargetFile == key.TargetFile {
				out.recordedHash = r.VersionHash
				break
			}
		}
		out.newHash = loaded.Meta.CurrentVersion
		// For Unreadable, supply a human-readable read error (the scan's
		// Warnings slice is too general — we want the exact os.Stat/Read
		// error for the target file).
		if pts.State == installcheck.StateModified && pts.Reason == installcheck.ReasonUnreadable {
			if _, serr := os.Stat(filepath.Clean(key.TargetFile)); serr != nil {
				out.readErr = serr.Error()
			}
		}
		return out
	}
	// No matching record = Fresh.
	return wizardScanResult{state: installcheck.StateFresh}
}

// maybeRouteThroughReinstallModal is called from the review-step Install
// confirmation (keyboard + mouse). Returns a cmd and a routed=true flag when
// the wizard transitioned to a D17 modal; the caller must NOT emit
// installResultMsg on routed paths. Fresh state returns routed=false so the
// caller falls through to the normal emit flow.
func (m *installWizardModel) maybeRouteThroughReinstallModal() (cmd tea.Cmd, routed bool) {
	r := installWizardScanFn(m.projectRoot, m.item)
	switch r.state {
	case installcheck.StateClean:
		m.updateModal.Open(r.targetFile, r.recordedHash, r.newHash)
		return nil, true
	case installcheck.StateModified:
		var reason modifiedReason
		switch r.reason {
		case installcheck.ReasonEdited:
			reason = modifiedReasonEdited
		case installcheck.ReasonMissing:
			reason = modifiedReasonMissing
		case installcheck.ReasonUnreadable:
			reason = modifiedReasonUnreadable
		default:
			reason = modifiedReasonEdited
		}
		m.modifiedModal.Open(r.targetFile, reason, r.readErr)
		return nil, true
	}
	return nil, false
}

// emitInstallResultWithAction is the single place where the review-step
// Install confirm produces installResultMsg. It stamps decisionAction on
// the message so the downstream executor can branch without re-scanning.
func (m *installWizardModel) emitInstallResultWithAction(action string) tea.Cmd {
	result := m.installResult()
	result.decisionAction = action
	return func() tea.Msg { return result }
}

// handleUpdateModalDecision processes a Case A decision. Replace -> emit
// installResultMsg with decisionAction="replace"; Skip -> close the wizard.
func (m *installWizardModel) handleUpdateModalDecision(action string) (*installWizardModel, tea.Cmd) {
	switch action {
	case "replace":
		m.confirmed = true
		return m, m.emitInstallResultWithAction("replace")
	default: // "skip"
		return m, func() tea.Msg { return installCloseMsg{} }
	}
}

// handleModifiedModalDecision processes a Case B decision. drop-record and
// append-fresh emit installResultMsg with the matching action; keep closes
// the wizard without mutation.
func (m *installWizardModel) handleModifiedModalDecision(action string) (*installWizardModel, tea.Cmd) {
	switch action {
	case "drop-record", "append-fresh":
		m.confirmed = true
		return m, m.emitInstallResultWithAction(action)
	default: // "keep"
		return m, func() tea.Msg { return installCloseMsg{} }
	}
}
