package converter

import (
	"encoding/json"
	"regexp"
	"strings"
)

// SecurityWarning represents a potential security concern found in a hook command.
type SecurityWarning struct {
	Severity    string // "high", "medium", "low"
	HookName    string // event name (e.g. "PreToolUse")
	Description string // human-readable explanation
	Command     string // the flagged command snippet
}

// dangerPattern pairs a compiled regex with its severity and description.
type dangerPattern struct {
	pattern     *regexp.Regexp
	severity    string
	description string
}

// Patterns are compiled once at init time. The regex approach is intentionally
// simple — we're doing string-level scanning, not shell parsing. This catches
// obvious dangerous commands but won't detect obfuscated variants (e.g. base64-
// encoded payloads). That's an acceptable trade-off: the goal is surfacing
// warnings for common risky patterns, not building a shell sandbox.
var dangerPatterns = []dangerPattern{
	// HIGH: Network exfiltration / remote access
	{regexp.MustCompile(`\bcurl\b`), "high", "network request (curl)"},
	{regexp.MustCompile(`\bwget\b`), "high", "network request (wget)"},
	{regexp.MustCompile(`\bnc\b`), "high", "network tool (nc)"},
	{regexp.MustCompile(`\bnetcat\b`), "high", "network tool (netcat)"},
	{regexp.MustCompile(`\bncat\b`), "high", "network tool (ncat)"},
	{regexp.MustCompile(`\bssh\b`), "high", "remote access (ssh)"},
	{regexp.MustCompile(`\bscp\b`), "high", "remote file copy (scp)"},

	// HIGH: Destructive commands
	{regexp.MustCompile(`\brm\s+(-[a-zA-Z]*r|-[a-zA-Z]*f)`), "high", "recursive/forced file deletion"},
	{regexp.MustCompile(`\bshred\b`), "high", "secure file deletion (shred)"},
	{regexp.MustCompile(`\bmkfs\b`), "high", "filesystem format (mkfs)"},

	// MEDIUM: Permission changes
	{regexp.MustCompile(`\bchmod\b`), "medium", "permission change (chmod)"},
	{regexp.MustCompile(`\bchown\b`), "medium", "ownership change (chown)"},

	// MEDIUM: Broad matchers — checked against matcher field, not command.
	// This is handled separately in scanMatcher().

	// MEDIUM: Secret/credential access
	{regexp.MustCompile(`\bcat\b.*\.ssh`), "medium", "reads SSH credentials"},
	{regexp.MustCompile(`\bcat\b.*\.env\b`), "medium", "reads environment file"},
	{regexp.MustCompile(`\benv\b.*\|\s*grep\b`), "medium", "searches environment variables"},

	// LOW: Writes to system paths
	{regexp.MustCompile(`(/etc/|/usr/)`), "low", "writes to system path"},
}

// ScanHookSecurity parses canonical hook JSON (flat or nested format) and
// returns warnings for commands that match known dangerous patterns.
//
// The scanner checks:
//  1. Command fields in each hook entry against dangerPatterns
//  2. URL fields in HTTP hooks (flagged as network access)
//  3. Matcher fields for overly broad patterns like ".*"
func ScanHookSecurity(content []byte) []SecurityWarning {
	var warnings []SecurityWarning

	// Try both formats — collect all hook data into a unified list
	var hookGroups []HookData

	if DetectHookFormat(content) == "flat" {
		hd, err := ParseFlat(content)
		if err != nil {
			return nil
		}
		hookGroups = append(hookGroups, hd)
	} else {
		items, err := ParseNested(content)
		if err != nil {
			return nil
		}
		hookGroups = items
	}

	for _, group := range hookGroups {
		// Check matcher for broad patterns
		warnings = append(warnings, scanMatcher(group.Event, group.Matcher)...)

		for _, hook := range group.Hooks {
			warnings = append(warnings, scanHookEntry(group.Event, hook)...)
		}
	}

	return warnings
}

// scanHookEntry checks a single hook entry's command (and URL) against patterns.
func scanHookEntry(event string, hook HookEntry) []SecurityWarning {
	var warnings []SecurityWarning

	// Check command field
	cmd := hook.Command
	if cmd != "" {
		for _, dp := range dangerPatterns {
			if dp.pattern.MatchString(cmd) {
				warnings = append(warnings, SecurityWarning{
					Severity:    dp.severity,
					HookName:    event,
					Description: dp.description,
					Command:     cmd,
				})
			}
		}
	}

	// HTTP hooks with a URL field are network access by definition
	if hook.URL != "" {
		warnings = append(warnings, SecurityWarning{
			Severity:    "high",
			HookName:    event,
			Description: "HTTP hook sends data to external endpoint",
			Command:     hook.URL,
		})
	}

	return warnings
}

// scanMatcher flags overly broad matchers that match all tools.
func scanMatcher(event string, matcher string) []SecurityWarning {
	if matcher == "" {
		return nil
	}

	// ".*" matches everything — the hook fires for every tool call.
	// This is risky because the hook author may not have intended to
	// intercept all tools, or a user importing this hook may not realize
	// the scope.
	if strings.TrimSpace(matcher) == ".*" {
		return []SecurityWarning{{
			Severity:    "medium",
			HookName:    event,
			Description: "matcher matches all tools (\".*\")",
			Command:     matcher,
		}}
	}

	return nil
}

// ScanHookSecurityFromRaw is a convenience wrapper that accepts raw JSON bytes
// that might not be valid hook JSON. Returns nil on parse failure (no warnings
// is the safe default for unparseable content — the parser will catch errors
// separately).
func ScanHookSecurityFromRaw(content []byte) []SecurityWarning {
	// Quick validity check — must be JSON object
	var raw json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil
	}
	return ScanHookSecurity(content)
}
