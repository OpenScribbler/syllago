package sandbox

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckResult holds the outcome of a pre-flight check.
type CheckResult struct {
	BwrapOK      bool
	BwrapVersion string
	SocatOK      bool
	ProviderOK   bool
	ProviderPath string
	Errors       []string
}

// Check performs the pre-flight check for the system (no provider) or for a specific provider.
// providerSlug may be empty to skip provider-specific checks.
func Check(providerSlug, homeDir, projectDir string) CheckResult {
	var r CheckResult

	// Check bwrap
	out, err := exec.Command("bwrap", "--version").Output()
	if err != nil {
		r.Errors = append(r.Errors, "bwrap not found — install bubblewrap >= 0.4.0")
	} else {
		r.BwrapOK = true
		r.BwrapVersion = strings.TrimSpace(string(out))
	}

	// Check socat
	if _, err := exec.LookPath("socat"); err != nil {
		r.Errors = append(r.Errors, "socat not found — install socat >= 1.7.0")
	} else {
		r.SocatOK = true
	}

	// Provider-specific check
	if providerSlug != "" {
		profile, err := ProfileFor(providerSlug, homeDir, projectDir)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("provider: %s", err))
		} else {
			r.ProviderOK = true
			r.ProviderPath = profile.BinaryExec
		}
	}

	return r
}

// FormatCheckResult formats a CheckResult for human display.
func FormatCheckResult(r CheckResult, providerSlug string) string {
	var sb strings.Builder

	status := func(ok bool) string {
		if ok {
			return "OK"
		}
		return "MISSING"
	}

	fmt.Fprintf(&sb, "  bwrap:  %s", status(r.BwrapOK))
	if r.BwrapVersion != "" {
		fmt.Fprintf(&sb, " (%s)", r.BwrapVersion)
	}
	sb.WriteByte('\n')

	fmt.Fprintf(&sb, "  socat:  %s\n", status(r.SocatOK))

	if providerSlug != "" {
		fmt.Fprintf(&sb, "  %s: %s", providerSlug, status(r.ProviderOK))
		if r.ProviderPath != "" {
			fmt.Fprintf(&sb, " (%s)", r.ProviderPath)
		}
		sb.WriteByte('\n')
	}

	for _, e := range r.Errors {
		fmt.Fprintf(&sb, "  ERROR: %s\n", e)
	}

	if len(r.Errors) == 0 {
		sb.WriteString("  Status: Ready for sandboxing\n")
	} else {
		sb.WriteString("  Status: Not ready\n")
	}

	return sb.String()
}
