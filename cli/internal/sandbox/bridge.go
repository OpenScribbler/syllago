package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WrapperScript generates the content of the in-sandbox wrapper shell script.
// The script:
//  1. Starts socat bridging the UNIX socket to TCP localhost:3128.
//  2. Execs the provider binary with its arguments.
//
// socketPath is the UNIX socket path as seen inside the sandbox.
// providerBin is the absolute path of the provider binary inside the sandbox.
// providerArgs are additional arguments to pass to the provider.
func WrapperScript(socketPath, providerBin string, providerArgs []string) string {
	args := ""
	for _, a := range providerArgs {
		args += " " + shellescape(a)
	}
	return fmt.Sprintf(`#!/bin/sh
socat TCP-LISTEN:3128,fork,reuseaddr UNIX-CONNECT:%s &
exec %s%s
`, socketPath, shellescape(providerBin), args)
}

// WriteWrapperScript writes the wrapper script to stagingDir/wrapper.sh
// and makes it executable.
func WriteWrapperScript(stagingDir, socketPath, providerBin string, providerArgs []string) (string, error) {
	content := WrapperScript(socketPath, providerBin, providerArgs)
	path := filepath.Join(stagingDir, "wrapper.sh")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return "", fmt.Errorf("writing wrapper script: %w", err)
	}
	return path, nil
}

// shellescape wraps a string in single quotes for safe shell interpolation.
// Single quotes within the string are handled via the '"'"' idiom.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
