package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
)

// printInstallNotices writes an install's notices in the form the installer
// printed them before it returned them as data, so CLI output is unchanged.
func printInstallNotices(w io.Writer, notices []installer.Notice) {
	for _, n := range notices {
		switch n.Kind {
		case installer.NoticeNote:
			fmt.Fprintf(w, "note: %s\n", n.Message)
		case installer.NoticeConversionWarning:
			fmt.Fprintf(w, "warning: %s\n", n.Message)
		case installer.NoticeScannerFinding:
			fmt.Fprintf(w, "  %s %s\n", strings.ToUpper(n.Severity), n.Message)
		case installer.NoticeScannerError:
			fmt.Fprintf(w, "  scanner error: %s\n", n.Message)
		case installer.NoticeScriptSecurity:
			fmt.Fprintf(w, "\n  SECURITY WARNING\n")
			for _, line := range strings.Split(n.Message, "\n") {
				fmt.Fprintf(w, "  %s\n", line)
			}
			fmt.Fprintln(w)
		default:
			fmt.Fprintf(w, "%s\n", n)
		}
	}
}
