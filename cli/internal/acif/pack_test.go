package acif

import (
	"testing"

	"github.com/google/uuid"
)

func TestDerivePackIDTV3(t *testing.T) {
	t.Parallel()
	namespace := uuid.MustParse("93516344-00e5-419b-a230-6e8b1d02f87d")
	got := DerivePackID(namespace, "https://github.com/obra/superpowers", "superpowers")
	if want := "d932cd6d-1c14-527d-b2e7-185c717b7a0d"; got.String() != want {
		t.Fatalf("DerivePackID() = %s, want %s", got.String(), want)
	}
}

func TestResolvePack(t *testing.T) {
	t.Parallel()

	const (
		declared = "11111111-1111-4111-8111-111111111111"
		inferred = "22222222-2222-4222-8222-222222222222"
		unknown  = "99999999-9999-4999-8999-999999999999"
	)
	cases := []struct {
		name       string
		declared   string
		inferred   string
		known      []string
		resolution string
		memberOf   string
		install    string
	}{
		{
			name:       "TV-4 declared wins when both known",
			declared:   declared,
			inferred:   inferred,
			known:      []string{declared, inferred},
			resolution: "declared",
			memberOf:   declared,
			install:    "proceed",
		},
		{
			name:       "TV-5 unknown declared refuses",
			declared:   unknown,
			known:      nil,
			resolution: "unresolved",
			memberOf:   "",
			install:    "refuse-unless-operator-opt-in",
		},
		{
			name:       "inferred only resolves without consulting known",
			inferred:   inferred,
			known:      nil,
			resolution: "inferred",
			memberOf:   inferred,
			install:    "proceed",
		},
		{
			name:       "neither",
			resolution: "none",
			install:    "proceed",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ResolvePack(tc.declared, tc.inferred, tc.known)
			if got.Resolution != tc.resolution || got.MemberOf != tc.memberOf || got.Install != tc.install {
				t.Fatalf("ResolvePack() = %+v, want resolution=%q member_of=%q install=%q",
					got, tc.resolution, tc.memberOf, tc.install)
			}
		})
	}
}
