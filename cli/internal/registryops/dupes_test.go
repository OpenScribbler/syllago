package registryops

import (
	"reflect"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

func TestNormalizeRegistryIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: "Syllago_Community", want: "syllago-community"},
		{name: "syllago-community", want: "syllago-community"},
		{name: "owner/Syllago_Community", want: "owner/syllago-community"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeRegistryIdentity(tt.name); got != tt.want {
				t.Fatalf("NormalizeRegistryIdentity(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestNormalizeRegistryURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
	}{
		{
			name: "lowercase and strip git suffix",
			a:    "https://github.com/Org/Repo.git",
			b:    "https://github.com/org/repo",
		},
		{
			name: "strip trailing slash before git suffix",
			a:    "HTTPS://Github.com/Org/Repo.git/",
			b:    "https://github.com/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if gotA, gotB := NormalizeRegistryURL(tt.a), NormalizeRegistryURL(tt.b); gotA != gotB {
				t.Fatalf("NormalizeRegistryURL(%q) = %q, NormalizeRegistryURL(%q) = %q; want equal", tt.a, gotA, tt.b, gotB)
			}
		})
	}
}

func TestFindSimilarRegistries(t *testing.T) {
	t.Parallel()

	existing := []config.Registry{
		{Name: "syllago-community", URL: "https://github.com/OpenScribbler/syllago-community.git"},
		{Name: "same", URL: "https://github.com/example/same.git"},
		{Name: "other", URL: "https://github.com/example/other.git"},
		{Name: "empty-url", URL: ""},
	}

	tests := []struct {
		name string
		in   string
		url  string
		want []string
	}{
		{
			name: "near-dup name found",
			in:   "Syllago_Community",
			url:  "https://github.com/example/different.git",
			want: []string{"syllago-community"},
		},
		{
			name: "near-dup url found",
			in:   "different-name",
			url:  "https://github.com/openscribbler/syllago-community",
			want: []string{"syllago-community"},
		},
		{
			name: "exact name excluded",
			in:   "same",
			url:  "https://github.com/example/same",
			want: nil,
		},
		{
			name: "unrelated entries excluded",
			in:   "new-registry",
			url:  "https://github.com/example/new.git",
			want: nil,
		},
		{
			name: "empty url skips url comparison",
			in:   "new-registry",
			url:  "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FindSimilarRegistries(existing, tt.in, tt.url); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("FindSimilarRegistries(..., %q, %q) = %#v, want %#v", tt.in, tt.url, got, tt.want)
			}
		})
	}
}
