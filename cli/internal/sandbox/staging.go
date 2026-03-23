package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StagingDir manages the per-session temporary directory.
type StagingDir struct {
	ID   string // random hex ID
	Path string // absolute path: /tmp/syllago-sandbox-<id>
}

// NewStagingDir creates a new staging directory with a random ID.
// Uses XDG_RUNTIME_DIR if available (user-owned tmpfs, avoids conflicts
// with bwrap's --tmpfs /tmp), falls back to /tmp.
func NewStagingDir() (*StagingDir, error) {
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generating staging ID: %w", err)
	}
	id := hex.EncodeToString(idBytes)

	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = "/tmp"
	}
	path := filepath.Join(base, "syllago-sandbox-"+id)
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, fmt.Errorf("creating staging dir: %w", err)
	}
	return &StagingDir{ID: id, Path: path}, nil
}

// SocketPath returns the path for the proxy UNIX socket.
func (s *StagingDir) SocketPath() string {
	return filepath.Join(s.Path, "proxy.sock")
}

// GitconfigPath returns the path for the sandbox-local gitconfig.
func (s *StagingDir) GitconfigPath() string {
	return filepath.Join(s.Path, "gitconfig")
}

// WriteGitconfig writes a minimal gitconfig (user.name, user.email only).
func (s *StagingDir) WriteGitconfig(name, email string) error {
	content := fmt.Sprintf("[user]\n\tname = %s\n\temail = %s\n", name, email)
	return os.WriteFile(s.GitconfigPath(), []byte(content), 0600)
}

// Cleanup removes the staging directory and all its contents.
func (s *StagingDir) Cleanup() error {
	return os.RemoveAll(s.Path)
}

// CleanStale removes any stale syllago-sandbox-* directories from previous
// crashed sessions. Checks both XDG_RUNTIME_DIR and /tmp.
// Called at the start of each new session.
func CleanStale() {
	dirs := []string{"/tmp"}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		dirs = append(dirs, xdg)
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "syllago-sandbox-") {
				fullPath := filepath.Join(dir, e.Name())
				info, err := os.Lstat(fullPath)
				if err != nil || !info.IsDir() {
					continue
				}
				_ = os.RemoveAll(fullPath)
			}
		}
	}
}
