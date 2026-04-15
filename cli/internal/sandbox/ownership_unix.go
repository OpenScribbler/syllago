//go:build !windows

package sandbox

import (
	"os"
	"syscall"
)

func isOwnedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}
	return stat.Uid == uint32(os.Getuid())
}
