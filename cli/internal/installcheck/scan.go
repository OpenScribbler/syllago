package installcheck

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// readFile is the filesystem-read hook indirected for test control of the
// mtime cache (see cache_test.go). Production path delegates to os.ReadFile.
var readFile = os.ReadFile

// Scan runs D16's verification algorithm against the provided install
// record and library. Library indexes rules by LibraryID (the .syllago.yaml
// id) and provides history as map[hash][]byte (canonical bytes per D11/D12).
func Scan(inst *installer.Installed, library map[string]*rulestore.Loaded) *VerificationResult {
	out := &VerificationResult{
		PerRecord: map[RecordKey]PerTargetState{},
		MatchSet:  map[string][]string{},
	}
	if inst == nil {
		return out
	}
	// Group records by unique TargetFile (D16 pseudocode).
	byTarget := map[string][]installer.InstalledRuleAppend{}
	for _, r := range inst.RuleAppends {
		byTarget[r.TargetFile] = append(byTarget[r.TargetFile], r)
	}
	for targetFile, records := range byTarget {
		stat, statErr := os.Stat(targetFile)
		if errors.Is(statErr, fs.ErrNotExist) {
			for _, r := range records {
				out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonMissing}
			}
			InvalidateCache(targetFile)
			continue
		}
		if statErr != nil {
			for _, r := range records {
				out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonUnreadable}
			}
			out.Warnings = append(out.Warnings, fmt.Sprintf("verify %s: %s", targetFile, statErr))
			InvalidateCache(targetFile)
			continue
		}
		// mtime cache — on hit, reuse previously-computed per-target state.
		if cached, ok := cacheGet(targetFile, stat.ModTime().UnixNano(), stat.Size()); ok {
			for _, r := range records {
				pts, found := cached[r.LibraryID]
				if !found {
					// Not in cache (new record added since); compute now by falling through.
					pts = PerTargetState{StateModified, ReasonEdited}
				}
				out.PerRecord[RecordKey{r.LibraryID, targetFile}] = pts
				if pts.State == StateClean {
					out.MatchSet[r.LibraryID] = append(out.MatchSet[r.LibraryID], targetFile)
				}
			}
			// Skip rereads if every record was in the cached set.
			allCached := true
			for _, r := range records {
				if _, found := cached[r.LibraryID]; !found {
					allCached = false
					break
				}
			}
			if allCached {
				continue
			}
		}
		raw, readErr := readFile(targetFile)
		if readErr != nil {
			for _, r := range records {
				out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonUnreadable}
			}
			out.Warnings = append(out.Warnings, fmt.Sprintf("verify %s: %s", targetFile, readErr))
			InvalidateCache(targetFile)
			continue
		}
		normalizedTarget := canonical.Normalize(raw)
		perLib := map[string]PerTargetState{}
		for _, r := range records {
			rule, ok := library[r.LibraryID]
			if !ok {
				pts := PerTargetState{StateModified, ReasonEdited}
				out.PerRecord[RecordKey{r.LibraryID, targetFile}] = pts
				perLib[r.LibraryID] = pts
				out.Warnings = append(out.Warnings, fmt.Sprintf("verify %s: no library rule for %s", targetFile, r.LibraryID))
				continue
			}
			body := rule.History[r.VersionHash]
			if body == nil {
				pts := PerTargetState{StateModified, ReasonEdited}
				out.PerRecord[RecordKey{r.LibraryID, targetFile}] = pts
				perLib[r.LibraryID] = pts
				out.Warnings = append(out.Warnings, fmt.Sprintf("verify %s: orphan record for %s (no history file for %s)", targetFile, r.LibraryID, r.VersionHash))
				continue
			}
			pattern := append([]byte{'\n'}, canonical.Normalize(body)...)
			var pts PerTargetState
			if bytes.Contains(normalizedTarget, pattern) {
				pts = PerTargetState{StateClean, ReasonNone}
				out.MatchSet[r.LibraryID] = append(out.MatchSet[r.LibraryID], targetFile)
			} else {
				pts = PerTargetState{StateModified, ReasonEdited}
			}
			out.PerRecord[RecordKey{r.LibraryID, targetFile}] = pts
			perLib[r.LibraryID] = pts
		}
		cachePut(targetFile, stat.ModTime().UnixNano(), stat.Size(), perLib)
	}
	return out
}
