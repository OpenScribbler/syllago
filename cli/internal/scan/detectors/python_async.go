package detectors

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// PythonAsync detects sync blocking calls inside async def functions.
// Common mistake in async Python projects: calling requests.get(),
// time.sleep(), etc. inside an async function blocks the event loop.
// The correct approach is to use async alternatives (httpx, asyncio.sleep,
// aiofiles, aiosqlite).
//
// How it works: a line-by-line state machine tracks whether we're inside
// an async def block (by indentation level). When inside one, it checks
// for known blocking call patterns.
//
// Trade-offs: this is a simple text scan, not an AST parse. It can produce
// false positives (e.g., blocking calls in comments or strings). A proper
// AST approach would be more accurate but significantly more complex and
// would require invoking a Python parser. The simple approach catches the
// vast majority of real cases.
type PythonAsync struct{}

func (d PythonAsync) Name() string { return "python-async" }

// syncBlockingCalls are function call patterns that block the event loop
// when used inside async functions.
var syncBlockingCalls = []string{
	"requests.get(",
	"requests.post(",
	"requests.put(",
	"requests.delete(",
	"requests.patch(",
	"requests.head(",
	"requests.options(",
	"time.sleep(",
	"sqlite3.connect(",
	"open(",
}

func (d PythonAsync) Detect(root string) ([]model.Section, error) {
	if !isPythonProject(root) {
		return nil, nil
	}

	type finding struct {
		file string
		line int
		call string
	}
	var findings []finding

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if shouldSkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(info.Name()) != ".py" {
			return nil
		}

		hits := scanAsyncForBlocking(path)
		for _, h := range hits {
			rel, _ := filepath.Rel(root, path)
			findings = append(findings, finding{file: rel, line: h.line, call: h.call})
		}
		return nil
	})

	if len(findings) == 0 {
		return nil, nil
	}

	// Build a readable body listing the offending locations.
	var lines []string
	for _, f := range findings {
		lines = append(lines, fmt.Sprintf("  %s:%d — %s", f.file, f.line, f.call))
	}
	body := fmt.Sprintf("Sync blocking calls found inside async functions (blocks the event loop):\n%s",
		strings.Join(lines, "\n"))

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Sync Calls in Async Functions",
		Body:     body,
		Source:   d.Name(),
	}}, nil
}

type blockingHit struct {
	line int
	call string
}

// scanAsyncForBlocking scans a Python file for blocking calls inside
// async def functions. Uses a simple state machine: when we see "async def",
// we record the indentation level. Subsequent lines with deeper indentation
// are considered part of that function. When indentation returns to the
// function's level (or less), we're outside the async function.
func scanAsyncForBlocking(path string) []blockingHit {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var hits []blockingHit
	inAsync := false
	asyncIndent := 0
	lineNum := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines and comments — they don't affect state.
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := countLeadingSpaces(line)

		// Check if this line starts a new async def.
		if strings.HasPrefix(trimmed, "async def ") {
			inAsync = true
			asyncIndent = indent
			continue
		}

		// Check if this line starts any new top-level definition that ends
		// the async block (def, class, or another construct at the same level).
		if inAsync && indent <= asyncIndent && !strings.HasPrefix(trimmed, "@") {
			inAsync = false
		}

		if !inAsync {
			continue
		}

		// Inside an async function — check for blocking calls.
		for _, call := range syncBlockingCalls {
			if strings.Contains(trimmed, call) {
				// Special case: "open(" should not flag "aiofiles.open("
				if call == "open(" && strings.Contains(trimmed, "aiofiles.open(") {
					continue
				}
				hits = append(hits, blockingHit{line: lineNum, call: strings.TrimSuffix(call, "(")})
				break // one hit per line
			}
		}
	}
	return hits
}

// countLeadingSpaces returns the number of leading space characters.
// Tabs are counted as 4 spaces (Python default).
func countLeadingSpaces(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 4
		} else {
			break
		}
	}
	return count
}

// isPythonProject returns true if the root contains pyproject.toml,
// setup.py, or requirements.txt.
func isPythonProject(root string) bool {
	for _, marker := range []string{"pyproject.toml", "setup.py", "requirements.txt"} {
		if _, err := os.Stat(filepath.Join(root, marker)); err == nil {
			return true
		}
	}
	return false
}

// shouldSkipDir returns true for directories that should be skipped
// during Python project walking.
func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "__pycache__", ".venv", "venv", "node_modules", "vendor":
		return true
	}
	return false
}
