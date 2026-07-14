package acif

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeSourceURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "scheme host and percent hex case", in: "HTTPS://GitHub.IO/A%3ab", want: "https://github.io/A%3Ab"},
		{name: "unreserved percent decode", in: "https://example.com/a%2Db%7Ec", want: "https://example.com/a-b~c"},
		{name: "reserved percent preserved", in: "https://example.com/a%2Fb", want: "https://example.com/a%2Fb"},
		{name: "decoded dot segments collapse", in: "https://example.com/x/%2E%2E/y", want: "https://example.com/y"},
		{name: "literal dot segments collapse", in: "https://example.com/a/./b/../c", want: "https://example.com/a/c"},
		{name: "default port stripped", in: "https://example.com:443/x", want: "https://example.com/x"},
		{name: "non default port retained", in: "https://example.com:8443/x", want: "https://example.com:8443/x"},
		{name: "empty path becomes slash", in: "https://example.com", want: "https://example.com/"},
		{name: "fragment dropped", in: "https://example.com/x#readme", want: "https://example.com/x"},
		{name: "bare query marker dropped", in: "https://example.com/x?", want: "https://example.com/x"},
		{name: "query rejected", in: "https://example.com/x?token=secret", wantErr: ErrSourceURIQueryPresent},
		{name: "http rejected by scheme gate", in: "http://example.com/x", wantErr: ErrSourceURISchemeForbidden},
		{name: "scp style malformed", in: "git@github.com:o/r.git", wantErr: ErrSourceURIMalformed},
		{name: "javascript rejected by scheme gate", in: "javascript:alert(1)", wantErr: ErrSourceURISchemeForbidden},
		{name: "data rejected by scheme gate", in: "data:text/plain,hi", wantErr: ErrSourceURISchemeForbidden},
		{name: "file rejected by scheme gate", in: "file:///etc/passwd", wantErr: ErrSourceURISchemeForbidden},
		{name: "userinfo rejected", in: "https://user:pw@example.com/x", wantErr: ErrSourceURIUserinfoPresent},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeSourceURI(tc.in)
			if tc.wantErr != "" {
				assertRejectID(t, err, tc.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("NormalizeSourceURI() error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeSourceURI() = %q, want %q", got, tc.want)
			}
			again, err := NormalizeSourceURI(got)
			if err != nil {
				t.Fatalf("NormalizeSourceURI(normalized) error: %v", err)
			}
			if again != got {
				t.Fatalf("NormalizeSourceURI() not idempotent: %q then %q", got, again)
			}
		})
	}
}

func TestNormalizeSourceURIIdempotencyLiterals(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		"HTTPS://GitHub.IO:443/A%3ab/./x/../y#frag",
		"https://example.com/a%2Db",
	} {
		got, err := NormalizeSourceURI(in)
		if err != nil {
			t.Fatalf("NormalizeSourceURI(%q) error: %v", in, err)
		}
		again, err := NormalizeSourceURI(got)
		if err != nil {
			t.Fatalf("NormalizeSourceURI(%q normalized) error: %v", in, err)
		}
		if again != got {
			t.Fatalf("NormalizeSourceURI(%q) not idempotent: %q then %q", in, got, again)
		}
	}
}

func TestDeriveURLName(t *testing.T) {
	t.Parallel()

	got, err := DeriveURLName("https://example.com/skills/my-skill.md", "single-file", "")
	if err != nil {
		t.Fatalf("DeriveURLName(single-file) error: %v", err)
	}
	if got.URLDerivedName != "my-skill" || len(got.Diagnostics) != 0 {
		t.Fatalf("DeriveURLName(single-file) = %#v", got)
	}

	conflict, err := DeriveURLName("https://example.com/skills/my-skill.md", "single-file", "declared")
	if err != nil {
		t.Fatalf("DeriveURLName(conflict) error: %v", err)
	}
	wantDiag := []Diagnostic{{
		ID: ErrSourceURIFilenameConflict,
		Params: map[string]any{
			"url_derived_name": "my-skill",
			"declared_name":    "declared",
		},
	}}
	if !reflect.DeepEqual(conflict.Diagnostics, wantDiag) {
		t.Fatalf("conflict diagnostics = %#v, want %#v", conflict.Diagnostics, wantDiag)
	}

	multi, err := DeriveURLName("https://example.com/skills/", "multi-file", "")
	if err != nil {
		t.Fatalf("DeriveURLName(multi-file) error: %v", err)
	}
	if !multi.Conformant || multi.URLDerivedName != "none" {
		t.Fatalf("DeriveURLName(multi-file) = %#v", multi)
	}

	_, err = DeriveURLName("https://example.com/skills/", "single-file", "")
	assertRejectID(t, err, ErrSourceURIDirectFileTrailingSlash)
}

func assertRejectID(t *testing.T, err error, want string) {
	t.Helper()
	var reject *RejectError
	if !errors.As(err, &reject) {
		t.Fatalf("error = %v, want RejectError %s", err, want)
	}
	if reject.ID != want {
		t.Fatalf("reject ID = %q, want %q", reject.ID, want)
	}
}
