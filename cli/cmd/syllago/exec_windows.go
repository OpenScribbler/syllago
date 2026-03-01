//go:build windows

package main

import (
	"os"
	"os/exec"
)

// execSelf on Windows uses exec.Command instead of syscall.Exec.
// Windows doesn't support replacing the current process.
func execSelf(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
