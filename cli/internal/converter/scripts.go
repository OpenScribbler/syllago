package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BundledScript records the original path of a script that was bundled into
// a hook's library directory during add-time.
type BundledScript struct {
	OriginalPath string // absolute path at add time (e.g. ~/.claude/hooks/lint.sh)
	Filename     string // filename in library dir (e.g. lint.sh)
}

// knownInterpreters are shell/runtime prefixes that precede a script path.
var knownInterpreters = map[string]bool{
	"bash": true, "sh": true, "zsh": true, "dash": true,
	"node": true, "python": true, "python3": true, "bun": true,
	"ruby": true, "perl": true,
}

// knownSubcommands are runtime subcommands that appear between the interpreter
// and the script path (e.g. "bun run script.ts", "node exec script.js").
var knownSubcommands = map[string]bool{
	"run": true, "exec": true,
}

// ExtractScriptRef parses a hook command string and returns the script file
// path if it references an external file. Returns empty string for inline
// commands (echo, cat, etc.) or interpreter -c "..." patterns.
//
// Recognized patterns:
//   - Direct:   ~/.claude/hooks/lint.sh --arg
//   - Direct:   /home/user/.claude/hooks/lint.sh
//   - Direct:   ./scripts/check.sh
//   - Interp:   bash ~/.claude/hooks/lint.sh --arg
//   - Interp:   node ./checker.js
//   - Inline:   bash -c "echo hello"   → returns ""
//   - Inline:   echo "hello world"      → returns ""
func ExtractScriptRef(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}

	idx := 0

	// If first token is a known interpreter, skip past it, any flags, and subcommands
	base := filepath.Base(fields[0])
	if knownInterpreters[base] {
		idx = 1
		for idx < len(fields) && strings.HasPrefix(fields[idx], "-") {
			// -c means inline script, not a file reference
			if fields[idx] == "-c" || strings.HasPrefix(fields[idx], "-c") {
				return ""
			}
			idx++
		}
		// Skip known subcommands (e.g. "bun run", "node exec")
		if idx < len(fields) && knownSubcommands[fields[idx]] {
			idx++
		}
	}

	if idx >= len(fields) {
		return ""
	}

	token := fields[idx]

	// Strip a single pair of matching surrounding quotes. Hook commands
	// commonly quote the script path to protect expanded env vars with
	// spaces, e.g. node "$CLAUDE_PROJECT_DIR/hooks/foo.js".
	if len(token) >= 2 {
		if (token[0] == '"' && token[len(token)-1] == '"') ||
			(token[0] == '\'' && token[len(token)-1] == '\'') {
			token = token[1 : len(token)-1]
		}
	}

	// Check for path-like patterns
	if strings.HasPrefix(token, "~/") ||
		strings.HasPrefix(token, "/") ||
		strings.HasPrefix(token, "./") ||
		strings.HasPrefix(token, "../") ||
		strings.HasPrefix(token, "$") {
		return token
	}

	return ""
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expanding ~: %w", err)
	}
	return filepath.Join(home, path[2:]), nil
}

// resolveScriptPath resolves a script reference to an absolute path.
// For relative paths (./foo), sourceDir is used as the base.
// For tilde paths (~/foo), the home directory is expanded.
// For absolute paths (/foo), returned as-is.
func resolveScriptPath(ref string, sourceDir string) (string, error) {
	if strings.HasPrefix(ref, "~/") {
		return expandTilde(ref)
	}
	if strings.HasPrefix(ref, "/") {
		return ref, nil
	}
	// Relative path — resolve against sourceDir
	return filepath.Clean(filepath.Join(sourceDir, ref)), nil
}

// BundleHookScripts scans a HookData's commands for script references,
// copies them into destDir (the hook's library directory), and rewrites
// the commands to use relative ./filename paths. Returns the modified
// hooks and a list of bundled scripts.
//
// sourceDir is the directory containing the provider's settings.json,
// used to resolve relative script paths.
func BundleHookScripts(hook *HookData, sourceDir, destDir string) ([]BundledScript, error) {
	var bundled []BundledScript

	for i := range hook.Hooks {
		ref := ExtractScriptRef(hook.Hooks[i].Command)
		if ref == "" {
			continue
		}

		absPath, err := resolveScriptPath(ref, sourceDir)
		if err != nil {
			return nil, fmt.Errorf("resolving script path %q: %w", ref, err)
		}

		// Check if the script actually exists — skip silently if not
		if _, err := os.Stat(absPath); err != nil {
			continue
		}

		filename := filepath.Base(absPath)

		// Copy script to destDir
		scriptData, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("reading script %s: %w", absPath, err)
		}
		destPath := filepath.Join(destDir, filename)
		if err := os.WriteFile(destPath, scriptData, 0700); err != nil {
			return nil, fmt.Errorf("copying script to %s: %w", destPath, err)
		}

		// Rewrite command to use relative path
		newRef := "./" + filename
		hook.Hooks[i].Command = strings.Replace(hook.Hooks[i].Command, ref, newRef, 1)

		bundled = append(bundled, BundledScript{
			OriginalPath: absPath,
			Filename:     filename,
		})
	}

	return bundled, nil
}
