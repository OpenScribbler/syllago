package gitutil

import (
	"os"
	"os/exec"
	"strings"
)

// Username returns the git user.name config value, falling back to $USER.
func Username() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			return name
		}
	}
	return os.Getenv("USER")
}
