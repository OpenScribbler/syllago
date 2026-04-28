package telemetry

// Disclosure URLs surfaced in the consent UI. Centralized here so the CLI
// prompt and TUI modal cannot drift apart.
const (
	DocsURL = "https://syllago.dev/telemetry"
	CodeURL = "https://github.com/OpenScribbler/syllago/tree/main/cli/internal/telemetry"
)

// MaintainerAppeal is the human paragraph the consent UI leads with. It is
// the only place in the codebase where syllago asks for something from the
// user — keep it honest and brief.
const MaintainerAppeal = "syllago is built and maintained by one person, in their free time. " +
	"Anonymous usage data is the only signal I have for what to fix and what " +
	"to build next. If you opt in, I am so grateful. It directly shapes " +
	"the project."

// CollectedItems returns the list of things telemetry sends, when enabled.
// One bullet per item, plain text — both renderers wrap them in their own
// styling. Order matches the disclosure flow: command context first, then
// system metadata, then the rotatable identifier.
func CollectedItems() []string {
	return []string{
		"Command name (install, convert, list, …)",
		"Provider slug (claude-code, cursor, …)",
		"Content type (rules, hooks, skills, …)",
		"Counts (number of items in a command)",
		"Boolean flags (--dry-run, success/failure)",
		"syllago version",
		"Operating system + CPU architecture (e.g. linux/amd64)",
		"A random anonymous ID, rotatable via `syllago telemetry reset`",
	}
}

// NeverItems returns the categories of data that are never collected. This
// list intentionally repeats what the docs say — the consent UI must stand
// on its own without requiring the user to follow a link.
func NeverItems() []string {
	return []string{
		"File contents, file paths, content names",
		"Usernames, hostnames, IP addresses, emails",
		"Registry URLs, hook commands, MCP configs",
		"Keystrokes or TUI navigation",
	}
}
