package sandbox

import (
	"os"
	"strings"
)

// BwrapConfig holds all inputs needed to construct bubblewrap arguments.
type BwrapConfig struct {
	ProjectDir          string // bind-mounted RW
	HomeDir             string // used for --setenv HOME
	StagingDir          string // contains wrapper.sh, git wrapper, proxy socket
	SocketPath          string // UNIX proxy socket path (inside sandbox)
	GitWrapperPath      string // path to git wrapper script (host side, mounted RO)
	WrapperScript       string // path to wrapper.sh (host side, mounted RO)
	Profile             *MountProfile
	Snapshots           []ConfigSnapshot  // for mounting staged config copies
	EcosystemCacheRO    []string          // RO bind mounts for package caches
	AdditionalMountsRO  []string          // user-supplied --mount-ro paths
	EnvPairs            []string          // KEY=VALUE pairs from FilterEnv
	SandboxEnvOverrides map[string]string // set by sandbox (PATH, HTTP_PROXY, etc.)
}

// BuildArgs returns the full slice of arguments to pass to bwrap.
func BuildArgs(cfg BwrapConfig) []string {
	var args []string

	// Security/namespace flags
	args = append(args,
		"--new-session",
		"--die-with-parent",
		"--unshare-net",
		"--unshare-pid",
		"--unshare-ipc",
		"--cap-drop", "ALL",
	)

	// Minimal device filesystem
	args = append(args, "--dev", "/dev", "--proc", "/proc")

	// Read-only system mounts
	for _, ro := range []string{"/usr", "/lib", "/lib64"} {
		args = append(args, "--ro-bind-try", ro, ro)
	}

	// Symlinks for /bin and /sbin (most distros have these as symlinks anyway)
	args = append(args, "--symlink", "usr/bin", "/bin")
	args = append(args, "--symlink", "usr/sbin", "/sbin")

	// Essential /etc files for DNS and TLS
	for _, f := range []string{
		"/etc/resolv.conf",
		"/etc/hosts",
		"/etc/nsswitch.conf",
	} {
		args = append(args, "--ro-bind-try", f, f)
	}
	for _, d := range []string{"/etc/ssl", "/etc/ca-certificates"} {
		args = append(args, "--ro-bind-try", d, d)
	}

	// Private /tmp
	args = append(args, "--tmpfs", "/tmp")

	// Project directory: read-write
	args = append(args, "--bind", cfg.ProjectDir, cfg.ProjectDir)

	// Proxy socket: bind into sandbox
	args = append(args, "--bind", cfg.SocketPath, cfg.SocketPath)

	// Git wrapper: mounted RO at higher PATH priority
	args = append(args, "--ro-bind", cfg.GitWrapperPath, "/usr/local/bin/git")

	// Wrapper script: mounted RO
	args = append(args, "--ro-bind", cfg.WrapperScript, cfg.WrapperScript)

	// Provider binary
	if cfg.Profile != nil {
		for _, bp := range cfg.Profile.BinaryPaths {
			args = append(args, "--ro-bind-try", bp, bp)
		}
	}

	// Staged config copies (RW — the sandbox can modify them)
	for _, snap := range cfg.Snapshots {
		if _, err := os.Stat(snap.StagedPath); err != nil {
			continue
		}
		args = append(args, "--bind", snap.StagedPath, snap.OriginalPath)
	}

	// Project-local config dirs (RW from actual CWD)
	if cfg.Profile != nil {
		for _, pd := range cfg.Profile.ProjectConfigDirs {
			if _, err := os.Stat(pd); err == nil {
				args = append(args, "--bind", pd, pd)
			}
		}
	}

	// Ecosystem caches (RO)
	for _, cache := range cfg.EcosystemCacheRO {
		args = append(args, "--ro-bind", cache, cache)
	}

	// User-supplied extra RO mounts (--mount-ro flag)
	for _, m := range cfg.AdditionalMountsRO {
		args = append(args, "--ro-bind", m, m)
	}

	// Environment: forwarded vars from FilterEnv
	for _, pair := range cfg.EnvPairs {
		idx := strings.IndexByte(pair, '=')
		if idx >= 0 {
			args = append(args, "--setenv", pair[:idx], pair[idx+1:])
		}
	}

	// Sandbox-set overrides (always applied, override anything from FilterEnv)
	for k, v := range cfg.SandboxEnvOverrides {
		args = append(args, "--setenv", k, v)
	}

	// The wrapper script is the entry point
	args = append(args, "--", cfg.WrapperScript)

	return args
}
