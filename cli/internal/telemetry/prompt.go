package telemetry

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrNoTTY is returned by PromptCLIConsent when the input or output stream is
// not interactive. Callers must treat this as "no consent recorded yet" and
// leave telemetry disabled. The user can opt in later via the TUI or by
// running `syllago telemetry on` from an interactive shell.
var ErrNoTTY = errors.New("telemetry: cannot prompt for consent on a non-interactive stream")

// PromptCLIConsent renders the disclosure block to out, reads a Yes/No
// answer from in, and returns the user's choice. It does NOT persist the
// choice — call RecordConsent(choice) afterward.
//
// Defaults to "No" on empty input, malformed input, or EOF: if the user is
// not paying attention, we err on the side of not collecting.
//
// The caller is responsible for deciding when prompting is appropriate
// (interactive stdin, not in --json/--quiet mode, not running a telemetry
// subcommand). See cli/cmd/syllago/main.go.
func PromptCLIConsent(out io.Writer, in io.Reader) (bool, error) {
	if out == nil || in == nil {
		return false, ErrNoTTY
	}
	fmt.Fprint(out, RenderDisclosure())
	fmt.Fprint(out, "\nEnable anonymous usage data? [y/N]: ")

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("reading consent answer: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	switch answer {
	case "y", "yes":
		fmt.Fprintln(out, "\nThank you. Telemetry is now enabled. Run `syllago telemetry off` any time to disable.")
		return true, nil
	default:
		fmt.Fprintln(out, "\nGot it — telemetry stays off. You can opt in later with `syllago telemetry on`.")
		return false, nil
	}
}

// RenderDisclosure returns the plain-text consent block printed by
// PromptCLIConsent. Exposed as a separate function so the same content can be
// reproduced by `syllago telemetry status` or any other CLI surface that
// wants to remind the user what is and is not collected.
func RenderDisclosure() string {
	var b strings.Builder
	const bar = "──────────────────────────────────────────────────────────────────────"
	b.WriteString("\n")
	b.WriteString(bar + "\n")
	b.WriteString("  syllago needs your help\n")
	b.WriteString(bar + "\n\n")
	b.WriteString("  ")
	b.WriteString(wrap(MaintainerAppeal, 66, "  "))
	b.WriteString("\n\n")

	b.WriteString("  What gets collected (only if you opt in):\n")
	for _, item := range CollectedItems() {
		b.WriteString("    • ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	b.WriteString("\n  Never collected:\n")
	for _, item := range NeverItems() {
		b.WriteString("    • ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	b.WriteString("\n  Read the docs: ")
	b.WriteString(DocsURL)
	b.WriteString("\n  Read the code: ")
	b.WriteString(CodeURL)
	b.WriteString("\n\n  Change this any time:\n")
	b.WriteString("    syllago telemetry on    — enable\n")
	b.WriteString("    syllago telemetry off   — disable\n")
	b.WriteString("    syllago telemetry reset — rotate anonymous ID\n")
	b.WriteString(bar + "\n")
	return b.String()
}

// wrap word-wraps s to width columns, prefixing every continuation line with
// indent. Used so the maintainer appeal reads as a paragraph regardless of
// the user's terminal width.
func wrap(s string, width int, indent string) string {
	if width <= 0 {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	line := words[0]
	for _, w := range words[1:] {
		if len(line)+1+len(w) > width {
			b.WriteString(line)
			b.WriteString("\n")
			b.WriteString(indent)
			line = w
			continue
		}
		line += " " + w
	}
	b.WriteString(line)
	return b.String()
}
