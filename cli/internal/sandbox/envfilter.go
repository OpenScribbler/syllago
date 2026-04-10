package sandbox

import (
	"fmt"
	"os"
	"strings"
)

// EnvReport describes which variables were forwarded vs stripped.
type EnvReport struct {
	Forwarded []string // variable names that will be passed into the sandbox
	Stripped  []string // variable names that were present but removed
}

// baseAllowlist is always forwarded regardless of provider or user config.
var baseAllowlist = []string{
	"HOME", "USER", "SHELL", "TERM", "LANG", "LC_ALL", "LC_CTYPE",
	"XDG_RUNTIME_DIR", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME",
	"COLORTERM", "TERM_PROGRAM", "FORCE_COLOR", "NO_COLOR",
	"EDITOR", "VISUAL",
	"TZ",
}

// FilterEnv returns (allowedPairs, report).
// allowedPairs is a slice of "KEY=VALUE" strings for the sandbox environment.
// extra is a list of additional variable names to allow (provider-specific + user config).
func FilterEnv(environ []string, extra []string) ([]string, EnvReport) {
	allowed := make(map[string]bool)
	for _, k := range baseAllowlist {
		allowed[k] = true
	}
	for _, k := range extra {
		allowed[k] = true
	}

	var pairs []string
	var report EnvReport

	present := make(map[string]string)
	for _, e := range environ {
		idx := strings.IndexByte(e, '=')
		if idx < 0 {
			continue
		}
		k, v := e[:idx], e[idx+1:]
		present[k] = v
	}

	for k, v := range present {
		if allowed[k] {
			pairs = append(pairs, k+"="+v)
			report.Forwarded = append(report.Forwarded, k)
		} else {
			report.Stripped = append(report.Stripped, k)
		}
	}

	return pairs, report
}

// FilterCurrentEnv is a convenience wrapper that filters os.Environ().
func FilterCurrentEnv(extra []string) ([]string, EnvReport) {
	return FilterEnv(os.Environ(), extra)
}

// deniedEnvVars are environment variable names that must never be injected
// into the sandbox, even from provider profiles. These enable arbitrary code
// loading or shell startup injection that could escape sandbox protections.
var deniedEnvVars = map[string]bool{
	// Dynamic linker injection (Linux)
	"LD_PRELOAD":      true,
	"LD_LIBRARY_PATH": true,
	"LD_AUDIT":        true,
	// Dynamic linker injection (macOS)
	"DYLD_INSERT_LIBRARIES": true,
	"DYLD_LIBRARY_PATH":     true,
	// Language runtime injection
	"PYTHONPATH":    true,
	"PYTHONSTARTUP": true,
	"NODE_PATH":     true,
	"NODE_OPTIONS":  true,
	"PERL5LIB":      true,
	"PERL5OPT":      true,
	"RUBYLIB":       true,
	"RUBYOPT":       true,
	// Shell startup injection
	"BASH_ENV": true,
	"ENV":      true,
}

// IsDeniedEnvVar reports whether name is a denied environment variable that
// must not be injected into the sandbox. If denied, it returns a human-readable
// reason string; otherwise it returns an empty string.
func IsDeniedEnvVar(name string) string {
	if deniedEnvVars[name] {
		return fmt.Sprintf("env var %q is blocked from sandbox injection (code loading/injection risk)", name)
	}
	return ""
}
