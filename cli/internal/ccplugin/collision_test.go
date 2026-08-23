package ccplugin

import (
	"reflect"
	"testing"
)

func TestSkillCollisions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		plugins           []Plugin
		librarySkillNames []string
		want              []Collision
	}{
		{
			name: "case-insensitive match enabled only sorted and deduped",
			plugins: []Plugin{
				{
					ID:      "second@market",
					Enabled: true,
					Skills: []Skill{
						{Name: "Deploy", Path: "/plugins/second/Deploy"},
						{Name: "Build", Path: "/plugins/second/Build"},
					},
				},
				{
					ID:      "disabled@market",
					Enabled: false,
					Skills: []Skill{
						{Name: "Deploy", Path: "/plugins/disabled/Deploy"},
					},
				},
				{
					ID:      "first@market",
					Enabled: true,
					Skills: []Skill{
						{Name: "deploy", Path: "/plugins/first/deploy"},
					},
				},
			},
			librarySkillNames: []string{"DEPLOY", "deploy", "other", "BUILD"},
			want: []Collision{
				{SkillName: "build", PluginID: "second@market", SkillPath: "/plugins/second/Build"},
				{SkillName: "deploy", PluginID: "first@market", SkillPath: "/plugins/first/deploy"},
				{SkillName: "deploy", PluginID: "second@market", SkillPath: "/plugins/second/Deploy"},
			},
		},
		{
			name: "no match",
			plugins: []Plugin{
				{
					ID:      "demo@market",
					Enabled: true,
					Skills:  []Skill{{Name: "lint", Path: "/plugins/demo/lint"}},
				},
			},
			librarySkillNames: []string{"test"},
			want:              nil,
		},
		{
			name: "disabled plugin excluded",
			plugins: []Plugin{
				{
					ID:      "demo@market",
					Enabled: false,
					Skills:  []Skill{{Name: "test", Path: "/plugins/demo/test"}},
				},
			},
			librarySkillNames: []string{"test"},
			want:              nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SkillCollisions(tc.plugins, tc.librarySkillNames)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SkillCollisions() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
