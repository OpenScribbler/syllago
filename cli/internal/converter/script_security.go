package converter

// Script-level security scanning.
//
// The hook scanner in hook_security.go catches dangerous patterns in inline
// commands embedded in a hook's `command` field. But hooks often shell out to
// scripts (`bash ./install.sh`, `python3 ./check.py`, `pwsh ./ps.ps1`) whose
// body lives in a separate file. Those bodies need the same scrutiny.
//
// This file adds:
//   - Language detection from file extension.
//   - Per-language danger patterns for shell, python, js/ts, ruby, powershell.
//   - ScanScript / ScanScriptFile entry points that map a file's content to
//     warnings using the appropriate pattern set.
//   - ScanHookFull that composes the hook JSON scan with script-file scanning
//     and a recursive sweep of any provider_data fields.
//
// Pattern design: each language gets its own ordered list, but we also run
// the shared "shell-agnostic" set (curl/wget/ssh/etc. — anything that survives
// across runtimes via shelling out). The goal is catching obvious exfil and
// destruction, not building a static analyzer. We deliberately use literal
// regex matching rather than parsing because (a) these languages share too
// many dialects to parse soundly in Go, and (b) regex false positives are
// preferable to false negatives for a warning surface the user sees before
// install.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Script language identifiers. These are the string keys used by
// languagePatterns and the return value of detectLanguage.
const (
	LangShell      = "shell"
	LangPython     = "python"
	LangJavaScript = "javascript"
	LangRuby       = "ruby"
	LangPowerShell = "powershell"
	LangUnknown    = ""
)

// extToLanguage maps script file extensions to their language identifier.
// Absent entries return LangUnknown — the scanner falls back to shell
// patterns (which catch most universal exfil commands).
var extToLanguage = map[string]string{
	".sh":   LangShell,
	".bash": LangShell,
	".zsh":  LangShell,
	".py":   LangPython,
	".js":   LangJavaScript,
	".mjs":  LangJavaScript,
	".cjs":  LangJavaScript,
	".ts":   LangJavaScript,
	".rb":   LangRuby,
	".ps1":  LangPowerShell,
}

// languagePatterns maps a language identifier to its specific danger-pattern
// set. Each set is additive to shellPatterns — the scanner runs shell
// patterns on every file (shell exfil commands work across runtimes by
// shelling out) then the language-specific overlay.
var languagePatterns = map[string][]dangerPattern{
	LangShell:      shellPatterns,
	LangPython:     pythonPatterns,
	LangJavaScript: jsPatterns,
	LangRuby:       rubyPatterns,
	LangPowerShell: powershellPatterns,
}

// shellPatterns: dangerous commands reachable via any shell-out, regardless
// of the host language. Anything a script can invoke via subprocess is fair
// game here — dupe of hook_security.go's universal set plus a few additions.
var shellPatterns = []dangerPattern{
	{regexp.MustCompile(`\bcurl\b`), "high", "network request (curl)"},
	{regexp.MustCompile(`\bwget\b`), "high", "network request (wget)"},
	{regexp.MustCompile(`\bnc\b`), "high", "network tool (nc)"},
	{regexp.MustCompile(`\bnetcat\b`), "high", "network tool (netcat)"},
	{regexp.MustCompile(`\bncat\b`), "high", "network tool (ncat)"},
	{regexp.MustCompile(`\bssh\b`), "high", "remote access (ssh)"},
	{regexp.MustCompile(`\bscp\b`), "high", "remote file copy (scp)"},
	{regexp.MustCompile(`\brsync\b`), "medium", "remote sync (rsync)"},
	{regexp.MustCompile(`\brm\s+(-[a-zA-Z]*r|-[a-zA-Z]*f)`), "high", "recursive/forced file deletion"},
	{regexp.MustCompile(`\bshred\b`), "high", "secure file deletion (shred)"},
	{regexp.MustCompile(`\bmkfs\b`), "high", "filesystem format (mkfs)"},
	{regexp.MustCompile(`\bdd\s+.*of=/dev/`), "high", "raw device write (dd)"},
	{regexp.MustCompile(`\bchmod\b`), "medium", "permission change (chmod)"},
	{regexp.MustCompile(`\bchown\b`), "medium", "ownership change (chown)"},
	{regexp.MustCompile(`\bcat\b.*\.ssh`), "medium", "reads SSH credentials"},
	{regexp.MustCompile(`\bcat\b.*\.env\b`), "medium", "reads environment file"},
	{regexp.MustCompile(`\benv\b.*\|\s*grep\b`), "medium", "searches environment variables"},
	{regexp.MustCompile(`(/etc/|/usr/)`), "low", "writes to system path"},
	// eval/exec in shell — arbitrary command execution with user-controlled input.
	{regexp.MustCompile(`\beval\s+`), "high", "dynamic command execution (eval)"},
}

// pythonPatterns: patterns specific to Python source. Focus is on code that
// doesn't shell out — native Python exfil (urllib, requests, socket) and
// subprocess calls that wouldn't be caught by string-matching the shell name.
var pythonPatterns = []dangerPattern{
	// Network — match on the *function call* forms, not bare identifiers,
	// so an unrelated variable named `requests` doesn't trip.
	{regexp.MustCompile(`\burllib\.request\.`), "high", "python network call (urllib.request)"},
	{regexp.MustCompile(`\burllib\.urlopen\b`), "high", "python network call (urllib.urlopen)"},
	{regexp.MustCompile(`\brequests\.(get|post|put|delete|patch|head|request)\b`), "high", "python network call (requests)"},
	{regexp.MustCompile(`\bhttp\.client\.(HTTPConnection|HTTPSConnection)\b`), "high", "python network call (http.client)"},
	{regexp.MustCompile(`\bsocket\.(socket|create_connection)\b`), "high", "python raw socket"},
	// Subprocess / exec paths.
	{regexp.MustCompile(`\bsubprocess\.(run|call|check_call|check_output|Popen)\b`), "high", "python subprocess execution"},
	{regexp.MustCompile(`\bos\.(system|popen|execv|execvp|execve)\b`), "high", "python os command execution"},
	// Dynamic code.
	{regexp.MustCompile(`\b(eval|exec)\s*\(`), "high", "python dynamic code execution"},
	{regexp.MustCompile(`\b__import__\s*\(`), "medium", "python dynamic import"},
}

// jsPatterns: JavaScript / TypeScript. Node APIs for network and subprocess
// plus fetch-family globals that can run in any modern runtime.
var jsPatterns = []dangerPattern{
	{regexp.MustCompile(`\bchild_process\b`), "high", "node subprocess module (child_process)"},
	{regexp.MustCompile(`\brequire\s*\(\s*['"]child_process['"]\s*\)`), "high", "node subprocess require"},
	{regexp.MustCompile(`\bfrom\s+['"]child_process['"]`), "high", "node subprocess import"},
	{regexp.MustCompile(`\bnet\.(Socket|createConnection|createServer)\b`), "high", "node net module (raw sockets)"},
	{regexp.MustCompile(`\bhttps?\.(request|get)\b`), "medium", "node http(s) request"},
	{regexp.MustCompile(`\bfetch\s*\(`), "high", "fetch() network call"},
	{regexp.MustCompile(`\bnew\s+XMLHttpRequest\b`), "medium", "XMLHttpRequest network call"},
	// Dynamic evaluation and Function constructor (indirect eval).
	{regexp.MustCompile(`\beval\s*\(`), "high", "javascript dynamic code execution (eval)"},
	{regexp.MustCompile(`\bnew\s+Function\s*\(`), "high", "javascript dynamic code execution (Function constructor)"},
	// fs.unlink / rm / rmdir — destructive fs ops.
	{regexp.MustCompile(`\bfs\.(rm|rmSync|unlink|unlinkSync|rmdir|rmdirSync)\b`), "medium", "node filesystem deletion"},
}

// rubyPatterns: Ruby. Net::HTTP and Kernel#system / backticks / exec.
var rubyPatterns = []dangerPattern{
	{regexp.MustCompile(`\bNet::HTTP\b`), "high", "ruby network call (Net::HTTP)"},
	{regexp.MustCompile(`\bopen-uri\b|\brequire\s+['"]open-uri['"]`), "medium", "ruby network helper (open-uri)"},
	{regexp.MustCompile(`\bsocket\.(TCPSocket|UDPSocket)\b|\bTCPSocket\.new\b`), "high", "ruby raw socket"},
	{regexp.MustCompile(`\bKernel\.(system|exec|spawn)\b`), "high", "ruby command execution"},
	{regexp.MustCompile(`(^|\s)system\s*\(`), "high", "ruby system() call"},
	{regexp.MustCompile("`[^`]*`"), "medium", "ruby backtick command execution"},
	{regexp.MustCompile(`\b(eval|instance_eval|class_eval|module_eval)\s*\(`), "high", "ruby dynamic code execution"},
	{regexp.MustCompile(`\bFile\.(delete|unlink)\b|\bFileUtils\.(rm|rm_rf|rm_r)\b`), "medium", "ruby filesystem deletion"},
}

// powershellPatterns: PowerShell. Heavy network cmdlets + invocation sinks.
var powershellPatterns = []dangerPattern{
	{regexp.MustCompile(`(?i)\bInvoke-WebRequest\b`), "high", "powershell network call (Invoke-WebRequest)"},
	{regexp.MustCompile(`(?i)\bInvoke-RestMethod\b`), "high", "powershell network call (Invoke-RestMethod)"},
	{regexp.MustCompile(`(?i)\bStart-BitsTransfer\b`), "high", "powershell BITS transfer"},
	{regexp.MustCompile(`(?i)\bNew-Object\s+Net\.WebClient\b`), "high", "powershell WebClient network call"},
	{regexp.MustCompile(`(?i)\bDownloadString\b|\bDownloadFile\b|\bDownloadData\b`), "high", "powershell download method"},
	{regexp.MustCompile(`(?i)\bInvoke-Expression\b`), "high", "powershell dynamic code execution (iex)"},
	{regexp.MustCompile(`(?i)\bIEX\b`), "high", "powershell iex alias"},
	// Destructive filesystem.
	{regexp.MustCompile(`(?i)\bRemove-Item\b.*-Recurse\b`), "high", "powershell recursive deletion"},
	{regexp.MustCompile(`(?i)\bFormat-Volume\b`), "high", "powershell volume format"},
	// Execution policy tamper — classic LOLBin prelude.
	{regexp.MustCompile(`(?i)\b-ExecutionPolicy\s+Bypass\b`), "high", "powershell execution policy bypass"},
}

// DetectLanguage returns the language identifier for a script path by looking
// at its extension (case-insensitive). Returns LangUnknown when the extension
// is absent or not recognized — callers should treat that as "use shell
// patterns only" rather than skipping the scan entirely.
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := extToLanguage[ext]; ok {
		return lang
	}
	return LangUnknown
}

// ScanScript runs the language's pattern set plus the shared shell set over
// content. The hook name parameter is a label propagated into each warning so
// downstream UI can attribute the match to a specific hook.
//
// An empty language string is treated as LangUnknown and only the shell set
// runs — this is the safe default for unrecognized files.
func ScanScript(content []byte, language, hookName string) []SecurityWarning {
	if len(content) == 0 {
		return nil
	}
	s := string(content)

	var warnings []SecurityWarning
	// Run shell patterns universally — any script can shell out.
	warnings = append(warnings, matchPatterns(s, shellPatterns, hookName)...)

	// Layer language-specific patterns if the language is recognized and has
	// its own set distinct from shellPatterns.
	if language != LangUnknown && language != LangShell {
		if specific, ok := languagePatterns[language]; ok {
			warnings = append(warnings, matchPatterns(s, specific, hookName)...)
		}
	}
	return warnings
}

// ScanScriptFile is a filesystem convenience wrapper: reads path, detects
// language, scans, returns warnings. Returns the read error unwrapped so the
// caller can distinguish missing-file from scanner output.
func ScanScriptFile(path string) ([]SecurityWarning, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read script %q: %w", path, err)
	}
	return ScanScript(data, DetectLanguage(path), filepath.Base(path)), nil
}

// matchPatterns applies a pattern set to content and collects warnings. Split
// out so ScanScript can combine the shell set with a language set without
// duplicating the inner loop.
func matchPatterns(content string, patterns []dangerPattern, hookName string) []SecurityWarning {
	var warnings []SecurityWarning
	for _, dp := range patterns {
		if loc := dp.pattern.FindStringIndex(content); loc != nil {
			// Extract the matched region so the user sees *what* triggered
			// the warning, not the entire script.
			snippet := content[loc[0]:loc[1]]
			warnings = append(warnings, SecurityWarning{
				Severity:    dp.severity,
				HookName:    hookName,
				Description: dp.description,
				Command:     snippet,
			})
		}
	}
	return warnings
}

// ScanProviderData walks a provider_data map recursively and runs all
// dangerPatterns (universal shell set) against every string it encounters.
// Provider_data is opaque by contract — it can hold arbitrary provider-
// specific fields — so the safest treatment is to assume any string might
// become a command somewhere downstream.
//
// Returned warnings carry hookName as their HookName so callers can route
// them back to the originating hook entry. The recursion does not attempt
// to interpret structure (arrays vs maps vs scalars) — it just flattens to
// strings.
func ScanProviderData(data map[string]any, hookName string) []SecurityWarning {
	if len(data) == 0 {
		return nil
	}
	var warnings []SecurityWarning
	walkJSONForPatterns(data, hookName, &warnings)
	return warnings
}

// walkJSONForPatterns traverses any JSON-shaped Go value (map, slice, scalar)
// and appends warnings for every string leaf that matches shellPatterns. We
// only run the universal set here because provider_data is language-agnostic
// — callers who know a specific field is code (e.g. cursor.stop_script)
// should invoke ScanScript directly with the appropriate language.
func walkJSONForPatterns(v any, hookName string, out *[]SecurityWarning) {
	switch t := v.(type) {
	case map[string]any:
		for _, child := range t {
			walkJSONForPatterns(child, hookName, out)
		}
	case []any:
		for _, child := range t {
			walkJSONForPatterns(child, hookName, out)
		}
	case string:
		*out = append(*out, matchPatterns(t, shellPatterns, hookName)...)
	}
	// Numbers, bools, nils — nothing to scan.
}

// ScanHookFull is the aggregate entry point that combines hook-level scanning
// with external script files and provider_data.
//
//   - hookJSON: bytes of either the legacy hook format (flat/nested as handled
//     by ScanHookSecurity) OR the CanonicalHooks format (with
//     per-hook Handler fields and ProviderData). Both are attempted;
//     whichever parses contributes warnings.
//   - scripts:  map of script path → file contents, provided by the caller
//     (the hook JSON alone doesn't disclose which scripts it
//     references — resolving commands like `bash ./setup.sh` is a
//     caller responsibility)
//
// Warnings are returned in a stable order: hook JSON first, then scripts in
// the iteration order of the supplied map (callers needing determinism
// should provide an ordered map via sorted keys externally). Duplicates are
// NOT deduplicated — callers see one warning per matched pattern, including
// cases where the same pattern triggers in a script and in an embedded
// command. This matches ScanHookSecurity's existing behavior.
func ScanHookFull(hookJSON []byte, scripts map[string][]byte) []SecurityWarning {
	var warnings []SecurityWarning

	// Legacy flat/nested path — unchanged from pre-existing ScanHookSecurity.
	warnings = append(warnings, ScanHookSecurity(hookJSON)...)

	// Canonical path — covers CanonicalHooks shape (Handler.Command,
	// Handler.URL) plus provider_data. Distinct from the legacy path because
	// the schema differs, and a consumer might hand us either representation.
	warnings = append(warnings, scanCanonicalHooks(hookJSON)...)

	for path, body := range scripts {
		warnings = append(warnings, ScanScript(body, DetectLanguage(path), filepath.Base(path))...)
	}
	return warnings
}

// scanCanonicalHooks parses the CanonicalHooks format and scans each hook's
// handler command, handler URL, and provider_data. Silently returns nil on
// parse failure — an unrelated format (legacy HookData) will have been
// picked up by ScanHookSecurity and any true structural error surfaces
// through the parser at install time.
func scanCanonicalHooks(hookJSON []byte) []SecurityWarning {
	var doc CanonicalHooks
	if err := json.Unmarshal(hookJSON, &doc); err != nil {
		return nil
	}
	var warnings []SecurityWarning
	for _, h := range doc.Hooks {
		label := h.Name
		if label == "" {
			label = h.Event
		}
		// Handler.Command — run the universal shell pattern set. We don't
		// know the language of an inline handler command, so shell patterns
		// are the safe lowest common denominator.
		if h.Handler.Command != "" {
			warnings = append(warnings, matchPatterns(h.Handler.Command, shellPatterns, label)...)
		}
		// Handler.URL — HTTP hooks always indicate network egress.
		if h.Handler.URL != "" {
			warnings = append(warnings, SecurityWarning{
				Severity:    "high",
				HookName:    label,
				Description: "HTTP hook sends data to external endpoint",
				Command:     h.Handler.URL,
			})
		}
		if len(h.ProviderData) > 0 {
			warnings = append(warnings, ScanProviderData(h.ProviderData, label)...)
		}
	}
	return warnings
}
