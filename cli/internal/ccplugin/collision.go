package ccplugin

import (
	"sort"
	"strings"
)

// Collision reports a plugin skill whose name matches a syllago library
// skill. Comparison is case-insensitive (the catalog's precedence dedupe
// lowercases names -- match that convention).
type Collision struct {
	SkillName string
	PluginID  string
	SkillPath string
}

// SkillCollisions reports enabled plugin skills whose names match library skills.
func SkillCollisions(plugins []Plugin, librarySkillNames []string) []Collision {
	libraryNames := make(map[string]struct{}, len(librarySkillNames))
	for _, name := range librarySkillNames {
		libraryNames[strings.ToLower(name)] = struct{}{}
	}

	var collisions []Collision
	seen := make(map[string]struct{})
	for _, plugin := range plugins {
		if !plugin.Enabled {
			continue
		}
		for _, skill := range plugin.Skills {
			skillName := strings.ToLower(skill.Name)
			if _, ok := libraryNames[skillName]; !ok {
				continue
			}

			key := skillName + "\x00" + plugin.ID + "\x00" + skill.Path
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			collisions = append(collisions, Collision{
				SkillName: skillName,
				PluginID:  plugin.ID,
				SkillPath: skill.Path,
			})
		}
	}

	sort.Slice(collisions, func(i, j int) bool {
		if collisions[i].SkillName != collisions[j].SkillName {
			return collisions[i].SkillName < collisions[j].SkillName
		}
		if collisions[i].PluginID != collisions[j].PluginID {
			return collisions[i].PluginID < collisions[j].PluginID
		}
		return collisions[i].SkillPath < collisions[j].SkillPath
	})

	return collisions
}
