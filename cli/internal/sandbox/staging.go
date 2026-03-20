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
func NewStagingDir() (*StagingDir, error) {
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generating staging ID: %w", err)
	}
	id := hex.EncodeToString(idBytes)
	path := filepath.Join("/tmp", "syllago-sandbox-"+id)
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

// CleanStale removes any stale /tmp/syllago-sandbox-* directories from previous
// crashed sessions. Called at the start of each new session.
func CleanStale() {
	entries, err := os.ReadDir("/tmp")
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "syllago-sandbox-") {
			fullPath := filepath.Join("/tmp", e.Name())
			// Verify it's a real directory (not a symlink) before removing
			info, err := os.Lstat(fullPath)
			if err != nil || !info.IsDir() {
				continue
			}
			_ = os.RemoveAll(fullPath)
		}
	}
}
