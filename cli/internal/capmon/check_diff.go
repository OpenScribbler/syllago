package capmon

import (
	"bytes"
	"fmt"
	"strings"
)

// maxInputLinesForDiff caps how many lines we feed into the O(n*m) LCS
// algorithm, preventing excessive memory use on large source files.
const maxInputLinesForDiff = 2000

// GenerateUnifiedDiff returns a human-readable diff between oldContent and
// newContent labelled with path and a sourceType (used to select the truncation
// threshold). Returns an empty string if the contents are identical.
//
// Truncation thresholds:
//   - sourceType == "source_code": 500 output lines
//   - all other types: 200 output lines
//
// When truncated, a final indicator line is appended:
//
//	[truncated after N lines (~X bytes shown) — full diff at .capmon-cache/<slug>/]
func GenerateUnifiedDiff(oldContent, newContent []byte, path, sourceType string) (string, error) {
	if bytes.Equal(oldContent, newContent) {
		return "", nil
	}

	oldLines := splitContentLines(oldContent)
	newLines := splitContentLines(newContent)

	// Cap inputs to limit LCS memory.
	if len(oldLines) > maxInputLinesForDiff {
		oldLines = oldLines[:maxInputLinesForDiff]
	}
	if len(newLines) > maxInputLinesForDiff {
		newLines = newLines[:maxInputLinesForDiff]
	}

	edits := lcsEdits(oldLines, newLines)

	maxLines := 200
	if sourceType == "source_code" {
		maxLines = 500
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- a/%s\n", path))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", path))
	lineCount := 2

	truncated := false
	for _, e := range edits {
		if lineCount >= maxLines {
			truncated = true
			break
		}
		switch e.kind {
		case editDelete:
			sb.WriteString("-")
			sb.WriteString(e.line)
			sb.WriteByte('\n')
		case editInsert:
			sb.WriteString("+")
			sb.WriteString(e.line)
			sb.WriteByte('\n')
		case editEqual:
			sb.WriteString(" ")
			sb.WriteString(e.line)
			sb.WriteByte('\n')
		}
		lineCount++
	}

	if truncated {
		sb.WriteString(fmt.Sprintf("[truncated after %d lines (~%d bytes shown) — full diff at .capmon-cache/<slug>/]\n",
			maxLines, sb.Len()))
	}

	return sb.String(), nil
}

// splitContentLines splits content into lines, stripping the trailing newline
// of each line. A trailing empty slice element from a final newline is dropped.
func splitContentLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	s := string(content)
	lines := strings.Split(s, "\n")
	// Trim trailing empty element from a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// editKind classifies a line in the LCS edit script.
type editKind int

const (
	editEqual  editKind = iota
	editDelete          // present in old, absent in new
	editInsert          // absent in old, present in new
)

type lineEdit struct {
	kind editKind
	line string
}

// lcsEdits computes an edit script between oldLines and newLines using the
// classic O(n*m) LCS dynamic-programming algorithm. The returned slice
// contains one entry per line in the merged output, tagged as equal/delete/insert.
func lcsEdits(oldLines, newLines []string) []lineEdit {
	m, n := len(oldLines), len(newLines)
	if m == 0 && n == 0 {
		return nil
	}

	// Build LCS table.
	// dp[i][j] = length of LCS of oldLines[:i] and newLines[:j].
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce the edit script.
	edits := make([]lineEdit, 0, m+n)
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldLines[i-1] == newLines[j-1]:
			edits = append(edits, lineEdit{editEqual, oldLines[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			edits = append(edits, lineEdit{editInsert, newLines[j-1]})
			j--
		default:
			edits = append(edits, lineEdit{editDelete, oldLines[i-1]})
			i--
		}
	}

	// Reverse: backtracking produces the script in reverse order.
	for a, b := 0, len(edits)-1; a < b; a, b = a+1, b-1 {
		edits[a], edits[b] = edits[b], edits[a]
	}
	return edits
}
