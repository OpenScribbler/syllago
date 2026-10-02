package installer

import "strings"

// NoticeKind classifies a Notice so each front end can render it its own way.
type NoticeKind string

const (
	// NoticeNote reports a change of plan the install made on its own, such
	// as copying instead of symlinking onto a Windows mount.
	NoticeNote NoticeKind = "note"
	// NoticeConversionWarning reports content the render for the target
	// provider dropped or approximated.
	NoticeConversionWarning NoticeKind = "conversion_warning"
	// NoticeScannerFinding reports one hook scanner finding; Severity holds
	// the scanner's severity.
	NoticeScannerFinding NoticeKind = "scanner_finding"
	// NoticeScannerError reports a scanner in the chain that failed to run.
	NoticeScannerError NoticeKind = "scanner_error"
	// NoticeScriptSecurity reports that a hook's bundled scripts were copied
	// to a stable location and will run as commands.
	NoticeScriptSecurity NoticeKind = "script_security"
)

// Notice is something an install has to tell the user. The installer never
// prints; it returns notices on the Placement, including alongside an error,
// so a blocked install can still say why.
type Notice struct {
	Kind     NoticeKind
	Severity string // scanner findings only: "high", "medium", "low", ...
	Message  string // may span lines (script security)
}

// String renders the notice on one line, for front ends that show a short
// list rather than the CLI's install output.
func (n Notice) String() string {
	msg := strings.ReplaceAll(n.Message, "\n", " ")
	switch n.Kind {
	case NoticeNote:
		return "note: " + msg
	case NoticeConversionWarning:
		return "warning: " + msg
	case NoticeScannerFinding:
		return strings.ToUpper(n.Severity) + " " + msg
	case NoticeScannerError:
		return "scanner error: " + msg
	case NoticeScriptSecurity:
		return "security warning: " + msg
	}
	return msg
}

// ScanOptions configures the hook scanner chain for one install. The zero
// value runs only the builtin scanner and blocks on high-severity findings.
type ScanOptions struct {
	Scanners []string // external scanner binaries (--hook-scanner)
	Force    bool     // install despite high-severity findings (--force)
}
