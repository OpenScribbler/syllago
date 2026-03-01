//go:build !windows

package main

import (
	"os"
	"syscall"
)

// execSelf replaces the current process with a new invocation of itself.
// On Unix, this uses syscall.Exec for efficient process replacement.
func execSelf(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(exe, args, os.Environ())
}
