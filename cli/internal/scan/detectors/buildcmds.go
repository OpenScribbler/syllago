package detectors

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// BuildCommands detects build/task commands from Makefiles and package.json scripts.
type BuildCommands struct{}

func (d BuildCommands) Name() string { return "build-commands" }

func (d BuildCommands) Detect(root string) ([]model.Section, error) {
	s := &model.BuildCommandSection{
		Origin: model.OriginAuto,
		Title:  "Build Commands",
	}

	if cmds := parseNPMScripts(root); len(cmds) > 0 {
		s.Commands = append(s.Commands, cmds...)
	}
	if cmds := parseMakefileTargets(root); len(cmds) > 0 {
		s.Commands = append(s.Commands, cmds...)
	}

	if len(s.Commands) == 0 {
		return nil, nil
	}
	return []model.Section{*s}, nil
}

func parseNPMScripts(root string) []model.BuildCommand {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	var cmds []model.BuildCommand
	for name, cmd := range pkg.Scripts {
		cmds = append(cmds, model.BuildCommand{
			Name:    name,
			Command: cmd,
			Source:  "package.json",
		})
	}
	return cmds
}

func parseMakefileTargets(root string) []model.BuildCommand {
	f, err := os.Open(filepath.Join(root, "Makefile"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var cmds []model.BuildCommand
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && line[0] != '\t' && line[0] != '#' && line[0] != '.' {
			if idx := strings.Index(line, ":"); idx > 0 {
				target := strings.TrimSpace(line[:idx])
				if !strings.ContainsAny(target, " \t=") {
					cmds = append(cmds, model.BuildCommand{
						Name:    target,
						Command: "make " + target,
						Source:  "Makefile",
					})
				}
			}
		}
	}
	return cmds
}
