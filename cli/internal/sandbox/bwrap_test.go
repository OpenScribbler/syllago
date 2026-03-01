package sandbox

import (
	"testing"
)

func testConfig() BwrapConfig {
	return BwrapConfig{
		ProjectDir:     "/home/user/project",
		HomeDir:        "/home/user",
		StagingDir:     "/tmp/syllago-sandbox-abc123",
		SocketPath:     "/tmp/syllago-sandbox-abc123/proxy.sock",
		GitWrapperPath: "/tmp/syllago-sandbox-abc123/git",
		WrapperScript:  "/tmp/syllago-sandbox-abc123/wrapper.sh",
		Profile: &MountProfile{
			BinaryPaths: []string{"/usr/bin/claude"},
		},
		EnvPairs: []string{"HOME=/home/user", "TERM=xterm-256color"},
		SandboxEnvOverrides: map[string]string{
			"HTTP_PROXY": "socks5://localhost:1080",
		},
	}
}

// argsContain checks if the args slice contains the given value.
func argsContain(args []string, val string) bool {
	for _, a := range args {
		if a == val {
			return true
		}
	}
	return false
}

// argsContainPair checks if args[i]==a and args[i+1]==b for some i.
func argsContainPair(args []string, a, b string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == a && args[i+1] == b {
			return true
		}
	}
	return false
}

// argsContainTriple checks if args[i]==a, args[i+1]==b, args[i+2]==c.
func argsContainTriple(args []string, a, b, c string) bool {
	for i := 0; i < len(args)-2; i++ {
		if args[i] == a && args[i+1] == b && args[i+2] == c {
			return true
		}
	}
	return false
}

func TestBuildArgs_ContainsUnshareNet(t *testing.T) {
	args := BuildArgs(testConfig())
	if !argsContain(args, "--unshare-net") {
		t.Error("expected --unshare-net in args")
	}
}

func TestBuildArgs_ContainsDieWithParent(t *testing.T) {
	args := BuildArgs(testConfig())
	if !argsContain(args, "--die-with-parent") {
		t.Error("expected --die-with-parent in args")
	}
}

func TestBuildArgs_ContainsCapDropAll(t *testing.T) {
	args := BuildArgs(testConfig())
	if !argsContainPair(args, "--cap-drop", "ALL") {
		t.Error("expected --cap-drop ALL in args")
	}
}

func TestBuildArgs_ProjectDirBind(t *testing.T) {
	cfg := testConfig()
	args := BuildArgs(cfg)
	if !argsContainTriple(args, "--bind", cfg.ProjectDir, cfg.ProjectDir) {
		t.Error("expected --bind for project dir")
	}
}

func TestBuildArgs_GitWrapperMount(t *testing.T) {
	cfg := testConfig()
	args := BuildArgs(cfg)
	if !argsContainTriple(args, "--ro-bind", cfg.GitWrapperPath, "/usr/local/bin/git") {
		t.Error("expected --ro-bind for git wrapper at /usr/local/bin/git")
	}
}

func TestBuildArgs_ProxySocketBound(t *testing.T) {
	cfg := testConfig()
	args := BuildArgs(cfg)
	if !argsContainTriple(args, "--bind", cfg.SocketPath, cfg.SocketPath) {
		t.Error("expected --bind for proxy socket")
	}
}

func TestBuildArgs_EnvVarSet(t *testing.T) {
	args := BuildArgs(testConfig())
	if !argsContainTriple(args, "--setenv", "HOME", "/home/user") {
		t.Error("expected --setenv HOME /home/user")
	}
}

func TestBuildArgs_SandboxOverridesEnv(t *testing.T) {
	args := BuildArgs(testConfig())
	if !argsContainTriple(args, "--setenv", "HTTP_PROXY", "socks5://localhost:1080") {
		t.Error("expected --setenv HTTP_PROXY override")
	}
}
