package reconcile

import (
	"regexp"
	"strings"
)

// MarkerFormat determines which comment syntax to use for boundary markers.
type MarkerFormat string

const (
	FormatHTML MarkerFormat = "html" // <!-- nesco:auto:name -->
	FormatYAML MarkerFormat = "yaml" // # nesco:auto:name
)

// Section represents a parsed section from an existing file.
type Section struct {
	Name    string // e.g., "auto:tech-stack" or "human:architecture"
	Content string // everything between opening and closing markers
	IsAuto  bool
	IsHuman bool
}

// Unmarked holds content that appears outside any boundary markers.
type Unmarked struct {
	Content string
	Index   int // position in the file (for ordering)
}

// ParseResult holds all parsed sections and unmarked content from a file.
type ParseResult struct {
	Sections []Section
	Unmarked []Unmarked
}

var (
	htmlOpenRe  = regexp.MustCompile(`<!--\s*nesco:(auto|human):(\S+)\s*-->`)
	htmlCloseRe = regexp.MustCompile(`<!--\s*/nesco:(auto|human):(\S+)\s*-->`)
	yamlOpenRe  = regexp.MustCompile(`#\s*nesco:(auto|human):(\S+)`)
	yamlCloseRe = regexp.MustCompile(`#\s*/nesco:(auto|human):(\S+)`)
)

// Parse extracts boundary-marked sections from file content.
func Parse(content string, format MarkerFormat) ParseResult {
	var openRe, closeRe *regexp.Regexp
	if format == FormatYAML {
		openRe, closeRe = yamlOpenRe, yamlCloseRe
	} else {
		openRe, closeRe = htmlOpenRe, htmlCloseRe
	}

	result := ParseResult{}
	lines := strings.Split(content, "\n")
	var currentSection *Section
	var currentContent []string
	var unmarkedContent []string
	sectionIdx := 0

	for _, line := range lines {
		if match := openRe.FindStringSubmatch(line); match != nil {
			if len(unmarkedContent) > 0 {
				result.Unmarked = append(result.Unmarked, Unmarked{
					Content: strings.Join(unmarkedContent, "\n"),
					Index:   sectionIdx,
				})
				unmarkedContent = nil
				sectionIdx++
			}

			origin := match[1]
			name := match[2]
			currentSection = &Section{
				Name:    origin + ":" + name,
				IsAuto:  origin == "auto",
				IsHuman: origin == "human",
			}
			currentContent = nil
			continue
		}

		if match := closeRe.FindStringSubmatch(line); match != nil && currentSection != nil {
			currentSection.Content = strings.Join(currentContent, "\n")
			result.Sections = append(result.Sections, *currentSection)
			currentSection = nil
			currentContent = nil
			sectionIdx++
			continue
		}

		if currentSection != nil {
			currentContent = append(currentContent, line)
		} else {
			unmarkedContent = append(unmarkedContent, line)
		}
	}

	if len(unmarkedContent) > 0 {
		result.Unmarked = append(result.Unmarked, Unmarked{
			Content: strings.Join(unmarkedContent, "\n"),
			Index:   sectionIdx,
		})
	}

	return result
}
