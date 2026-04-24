package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
	"github.com/OpenScribbler/syllago/cli/internal/splitter"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// parseSplitOption converts the --split flag value into a splitter.Options.
// Recognized forms: "", "h2", "h3", "h4", "marker:<literal>", "single", "llm".
// An empty value defaults to H2. "llm" is handled by the caller (D9: requires
// the split-rules-llm skill).
func parseSplitOption(val string) (splitter.Options, bool, error) {
	val = strings.TrimSpace(val)
	isLLM := false
	switch {
	case val == "" || val == "h2":
		return splitter.Options{Heuristic: splitter.HeuristicH2}, isLLM, nil
	case val == "h3":
		return splitter.Options{Heuristic: splitter.HeuristicH3}, isLLM, nil
	case val == "h4":
		return splitter.Options{Heuristic: splitter.HeuristicH4}, isLLM, nil
	case val == "single":
		return splitter.Options{Heuristic: splitter.HeuristicSingle}, isLLM, nil
	case val == "llm":
		isLLM = true
		return splitter.Options{}, isLLM, nil
	case strings.HasPrefix(val, "marker:"):
		lit := strings.TrimPrefix(val, "marker:")
		if lit == "" {
			return splitter.Options{}, false, fmt.Errorf("--split=marker: requires a literal after the colon")
		}
		return splitter.Options{Heuristic: splitter.HeuristicMarker, MarkerLiteral: lit}, false, nil
	}
	return splitter.Options{}, false, fmt.Errorf("unrecognized --split value %q (want h2|h3|h4|marker:<literal>|single|llm)", val)
}

// runAddFromMonolithicFiles handles "syllago add --from <path> [--from <path2>]
// --split=<mode>" — the non-interactive batched import for monolithic rule
// files. D3 picks the heuristic, D4 applies skip-split gates, D9 gates the LLM
// path behind the split-rules-llm skill, D18 requires --split to be specified.
func runAddFromMonolithicFiles(cmd *cobra.Command, projectRoot string, paths []string) error {
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}

	splitVal, _ := cmd.Flags().GetString("split")
	opts, isLLM, err := parseSplitOption(splitVal)
	if err != nil {
		return output.NewStructuredError(output.ErrInputInvalid, err.Error(), "")
	}
	if isLLM {
		return output.NewStructuredError(
			output.ErrItemNotFound,
			"--split=llm requires the split-rules-llm skill: syllago add split-rules-llm",
			"Install split-rules-llm from the syllago-meta-registry",
		)
	}

	type candidateForFile struct {
		sourceFile string
		slug       string
		source     []byte
		cands      []splitter.SplitCandidate
	}

	var skipped []string
	var allCandidates []candidateForFile

	for _, path := range paths {
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return output.NewStructuredErrorDetail(output.ErrSystemIO, "reading "+path, "Check file exists and is readable", rerr.Error())
		}
		canon := canonical.Normalize(raw)
		fileSlug := provider.SlugForMonolithicFilename(filepath.Base(path))
		if fileSlug == "" {
			return output.NewStructuredError(
				output.ErrInputInvalid,
				fmt.Sprintf("unrecognized monolithic filename %q", filepath.Base(path)),
				"Supported: CLAUDE.md, AGENTS.md, GEMINI.md, .cursorrules, .clinerules, .windsurfrules",
			)
		}
		cands, skip := splitter.Split(canon, opts)
		if skip != nil {
			skipped = append(skipped, path)
			continue
		}
		allCandidates = append(allCandidates, candidateForFile{
			sourceFile: path,
			slug:       fileSlug,
			source:     canon,
			cands:      cands,
		})
	}

	// D4: on skip-split, require --split=single explicitly.
	if len(skipped) > 0 && opts.Heuristic != splitter.HeuristicSingle {
		return output.NewStructuredError(
			output.ErrInputInvalid,
			fmt.Sprintf("skip-split triggered for %d file(s); pass --split=single to import whole-file", len(skipped)),
			"Files: "+strings.Join(skipped, ", ")+"\nExample: syllago add --from "+skipped[0]+" --split=single",
		)
	}

	// Write each candidate to the library under globalDir/rules/<slug>/<name>.
	totalWritten := 0
	for _, f := range allCandidates {
		for _, c := range f.cands {
			slug := c.Name
			if slug == "" {
				// Whole-file import (single mode) — derive slug from filename stem.
				stem := strings.TrimSuffix(filepath.Base(f.sourceFile), filepath.Ext(filepath.Base(f.sourceFile)))
				slug = strings.ToLower(strings.TrimPrefix(stem, "."))
				if slug == "" {
					slug = "monolithic"
				}
			}
			meta := metadata.RuleMetadata{
				ID:          metadata.NewID(),
				Name:        slug,
				Type:        "rule",
				Description: c.Description,
			}
			canonBody := canonical.Normalize([]byte(c.Body))
			sourceFilename := filepath.Base(f.sourceFile)
			rulesRoot := filepath.Join(globalDir, string(catalog.Rules))
			if werr := rulestore.WriteRuleWithSource(rulesRoot, f.slug, slug, meta, canonBody, sourceFilename, f.source); werr != nil {
				return output.NewStructuredErrorDetail(output.ErrSystemIO, "writing rule "+slug, "", werr.Error())
			}
			totalWritten++
		}
	}

	if !output.Quiet {
		fmt.Fprintf(output.Writer, "Added %d rule(s) from %d monolithic file(s).\n", totalWritten, len(paths))
	}
	telemetry.Enrich("content_type", "rules")
	telemetry.Enrich("content_count", totalWritten)
	telemetry.Enrich("mode", "monolithic")
	// D18 per-run telemetry: how many files were considered, how many the user
	// kept, which heuristic fired, and the scope label for the source files.
	telemetry.Enrich("discovery_candidate_count", len(paths))
	telemetry.Enrich("selected_count", len(allCandidates))
	telemetry.Enrich("split_method", splitMethodTag(opts))
	telemetry.Enrich("scope", scopeTagFromPaths(paths, projectRoot))
	return nil
}

// splitMethodTag maps a splitter.Options back to the telemetry string used in
// docs/telemetry.json. Keeping this narrow (one of h2/h3/h4/marker/single)
// avoids leaking marker-literal content into telemetry.
func splitMethodTag(opts splitter.Options) string {
	switch opts.Heuristic {
	case splitter.HeuristicH2:
		return "h2"
	case splitter.HeuristicH3:
		return "h3"
	case splitter.HeuristicH4:
		return "h4"
	case splitter.HeuristicMarker:
		return "marker"
	case splitter.HeuristicSingle:
		return "single"
	}
	return "h2"
}

// scopeTagFromPaths derives a best-effort scope label from the paths being
// imported. If every path sits under projectRoot, "project"; if every path
// sits under the home dir, "global"; otherwise "mixed". No filenames or paths
// are emitted — only the categorical label.
func scopeTagFromPaths(paths []string, projectRoot string) string {
	home, _ := os.UserHomeDir()
	anyProject, anyGlobal, anyOther := false, false, false
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			anyOther = true
			continue
		}
		switch {
		case projectRoot != "" && strings.HasPrefix(abs, filepath.Clean(projectRoot)+string(filepath.Separator)):
			anyProject = true
		case home != "" && strings.HasPrefix(abs, filepath.Clean(home)+string(filepath.Separator)):
			anyGlobal = true
		default:
			anyOther = true
		}
	}
	if anyProject && !anyGlobal && !anyOther {
		return "project"
	}
	if anyGlobal && !anyProject && !anyOther {
		return "global"
	}
	return "mixed"
}
