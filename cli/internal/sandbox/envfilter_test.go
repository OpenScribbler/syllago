package sandbox

import (
	"sort"
	"testing"
)

func TestFilterEnv_BaseAllowlist(t *testing.T) {
	env := []string{"HOME=/home/user", "USER=user", "TERM=xterm", "SECRET=bad"}
	pairs, report := FilterEnv(env, nil)

	// HOME, USER, TERM should be forwarded
	if len(pairs) != 3 {
		t.Errorf("expected 3 pairs, got %d: %v", len(pairs), pairs)
	}

	sort.Strings(report.Forwarded)
	expected := []string{"HOME", "TERM", "USER"}
	for i, e := range expected {
		if i >= len(report.Forwarded) || report.Forwarded[i] != e {
			t.Errorf("expected forwarded[%d]=%q, got %v", i, e, report.Forwarded)
			break
		}
	}

	if len(report.Stripped) != 1 || report.Stripped[0] != "SECRET" {
		t.Errorf("expected SECRET stripped, got: %v", report.Stripped)
	}
}

func TestFilterEnv_StripsSecrets(t *testing.T) {
	env := []string{
		"HOME=/home/user",
		"AWS_ACCESS_KEY_ID=AKIA...",
		"AWS_SECRET_ACCESS_KEY=secret",
		"SSH_AUTH_SOCK=/tmp/agent",
		"GITHUB_TOKEN=ghp_xxx",
	}
	_, report := FilterEnv(env, nil)

	if len(report.Stripped) != 4 {
		t.Errorf("expected 4 stripped vars, got %d: %v", len(report.Stripped), report.Stripped)
	}
}

func TestFilterEnv_ExtraAllowlist(t *testing.T) {
	env := []string{"HOME=/home/user", "DATABASE_URL=postgres://localhost/db"}
	pairs, report := FilterEnv(env, []string{"DATABASE_URL"})

	if len(pairs) != 2 {
		t.Errorf("expected 2 pairs, got %d", len(pairs))
	}
	if len(report.Forwarded) != 2 {
		t.Errorf("expected 2 forwarded, got %d", len(report.Forwarded))
	}
	if len(report.Stripped) != 0 {
		t.Errorf("expected 0 stripped, got %d: %v", len(report.Stripped), report.Stripped)
	}
}

func TestFilterEnv_EmptyEnviron(t *testing.T) {
	pairs, report := FilterEnv(nil, nil)
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs for nil environ, got %d", len(pairs))
	}
	if len(report.Forwarded) != 0 || len(report.Stripped) != 0 {
		t.Errorf("expected empty report, got forwarded=%d stripped=%d",
			len(report.Forwarded), len(report.Stripped))
	}
}

func TestIsDeniedEnvVar(t *testing.T) {
	t.Parallel()

	denied := []string{
		"LD_PRELOAD", "LD_LIBRARY_PATH", "LD_AUDIT",
		"DYLD_INSERT_LIBRARIES", "DYLD_LIBRARY_PATH",
		"PYTHONPATH", "PYTHONSTARTUP",
		"NODE_PATH", "NODE_OPTIONS",
		"PERL5LIB", "PERL5OPT",
		"RUBYLIB", "RUBYOPT",
		"BASH_ENV", "ENV",
	}
	for _, name := range denied {
		if reason := IsDeniedEnvVar(name); reason == "" {
			t.Errorf("expected %q to be denied, got empty reason", name)
		}
	}

	safe := []string{"HOME", "PATH", "EDITOR", "MY_APP_TOKEN", "DATABASE_URL"}
	for _, name := range safe {
		if reason := IsDeniedEnvVar(name); reason != "" {
			t.Errorf("expected %q to be allowed, got: %s", name, reason)
		}
	}
}

func TestFilterEnv_ReportAccuracy(t *testing.T) {
	env := []string{
		"HOME=/home/user",
		"USER=user",
		"AWS_KEY=secret",
		"RANDOM_VAR=value",
		"TERM=xterm",
	}
	_, report := FilterEnv(env, nil)

	total := len(report.Forwarded) + len(report.Stripped)
	if total != 5 {
		t.Errorf("expected forwarded+stripped=5, got %d (forwarded=%d, stripped=%d)",
			total, len(report.Forwarded), len(report.Stripped))
	}
}
