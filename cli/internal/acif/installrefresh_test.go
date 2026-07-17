package acif

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type rewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = rt.target.Scheme
	clone.URL.Host = rt.target.Host
	clone.Host = rt.target.Host
	return rt.base.RoundTrip(clone)
}

func installRefreshClient(t *testing.T, target string) *http.Client {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return &http.Client{
		Timeout: 2 * time.Second,
		Transport: rewriteTransport{
			target: u,
			base:   http.DefaultTransport,
		},
	}
}

func newInstallRefreshServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local httptest listener unavailable: %v", err)
	}
	ts := httptest.NewUnstartedServer(handler)
	ts.Listener = ln
	ts.Start()
	return ts
}

func resetInstallMatrixStateForTest(t *testing.T) {
	t.Helper()
	installMatrixMu.Lock()
	installMatrixLoaded = false
	installMatrixVal = nil
	installMatrixErr = nil
	installMatrixMu.Unlock()
}

func unsetInstallEntryPointsPathEnv(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv(InstallEntryPointsPathEnv)
	if err := os.Unsetenv(InstallEntryPointsPathEnv); err != nil {
		t.Fatalf("unset %s: %v", InstallEntryPointsPathEnv, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(InstallEntryPointsPathEnv, old)
			return
		}
		_ = os.Unsetenv(InstallEntryPointsPathEnv)
	})
}

func vendoredRows(t *testing.T) []InstallEntry {
	t.Helper()
	unsetInstallEntryPointsPathEnv(t)
	resetInstallMatrixStateForTest(t)
	rows, err := InstallEntryRows("claude-code", "skill")
	if err != nil {
		t.Fatalf("InstallEntryRows: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected vendored claude-code skill rows")
	}
	return rows
}

func TestRefreshInstallEntryPoints_Success(t *testing.T) {
	unsetInstallEntryPointsPathEnv(t)
	resetInstallMatrixStateForTest(t)

	ts := newInstallRefreshServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
install_entry_points:
  test-provider:
    skill:
      - {scope: user, path_template: "~/.test/skills/<content-name>/", layout: directory_of_files, status: current}
`))
	}))
	defer ts.Close()

	if err := RefreshInstallEntryPoints(installRefreshClient(t, ts.URL)); err != nil {
		t.Fatalf("RefreshInstallEntryPoints: %v", err)
	}

	rows, err := InstallEntryRows("test-provider", "skill")
	if err != nil {
		t.Fatalf("InstallEntryRows: %v", err)
	}
	wantRows := []InstallEntry{{
		Scope:        "user",
		PathTemplate: "~/.test/skills/<content-name>/",
		Layout:       "directory_of_files",
		Status:       "current",
	}}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Errorf("rows = %+v, want %+v", rows, wantRows)
	}

	targets, _, err := ResolveInstallTargets(InstallResolveInput{
		Provider: "test-provider", ContentType: "skill", ContentName: "alpha",
		HomeDir: "/home/u", ProjectRoot: "/repo", Scope: "user",
	})
	if err != nil {
		t.Fatalf("ResolveInstallTargets: %v", err)
	}
	wantTargets := []InstallTarget{{
		Scope: "user", Path: "/home/u/.test/skills/alpha/",
		Layout: "directory_of_files", Status: "current", WriteTarget: true,
	}}
	if !reflect.DeepEqual(targets, wantTargets) {
		t.Errorf("targets = %+v, want %+v", targets, wantTargets)
	}
}

func TestRefreshInstallEntryPoints_TransportCoverage(t *testing.T) {
	validBody := `
install_entry_points:
  transport-provider:
    skill:
      - {scope: user, path_template: "~/.transport/skills/<content-name>/", layout: directory_of_files, status: current}
`

	tests := []struct {
		name      string
		status    int
		body      string
		fetchErr  error
		wantError bool
	}{
		{name: "valid", status: http.StatusOK, body: validBody},
		{name: "garbage", status: http.StatusOK, body: "install_entry_points: [", wantError: true},
		{name: "non_200", status: http.StatusTeapot, body: "nope", wantError: true},
		{name: "connection_refused", fetchErr: errors.New("connection refused"), wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetInstallEntryPointsPathEnv(t)
			resetInstallMatrixStateForTest(t)

			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != installEntryPointsURL {
					t.Fatalf("request URL = %q, want %q", req.URL.String(), installEntryPointsURL)
				}
				if tc.fetchErr != nil {
					return nil, tc.fetchErr
				}
				return &http.Response{
					StatusCode: tc.status,
					Status:     http.StatusText(tc.status),
					Body:       io.NopCloser(strings.NewReader(tc.body)),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			})}

			err := RefreshInstallEntryPoints(client)
			if tc.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				if rows, rowsErr := InstallEntryRows("transport-provider", "skill"); rowsErr != nil || len(rows) != 0 {
					t.Fatalf("failed refresh should not expose transport rows: rows=%+v err=%v", rows, rowsErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("RefreshInstallEntryPoints: %v", err)
			}
			rows, rowsErr := InstallEntryRows("transport-provider", "skill")
			if rowsErr != nil {
				t.Fatalf("InstallEntryRows: %v", rowsErr)
			}
			if len(rows) != 1 || rows[0].PathTemplate != "~/.transport/skills/<content-name>/" {
				t.Fatalf("rows = %+v, want refreshed transport row", rows)
			}
		})
	}
}

func TestRefreshInstallEntryPoints_GarbageKeepsVendoredState(t *testing.T) {
	before := vendoredRows(t)

	ts := newInstallRefreshServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("install_entry_points: ["))
	}))
	defer ts.Close()

	if err := RefreshInstallEntryPoints(installRefreshClient(t, ts.URL)); err == nil {
		t.Fatal("expected parse error")
	}
	after, err := InstallEntryRows("claude-code", "skill")
	if err != nil {
		t.Fatalf("InstallEntryRows after failed refresh: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("vendored rows changed after failed refresh: got %+v, want %+v", after, before)
	}
}

func TestRefreshInstallEntryPoints_Non200KeepsVendoredState(t *testing.T) {
	before := vendoredRows(t)

	ts := newInstallRefreshServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusTeapot)
	}))
	defer ts.Close()

	if err := RefreshInstallEntryPoints(installRefreshClient(t, ts.URL)); err == nil {
		t.Fatal("expected status error")
	}
	after, err := InstallEntryRows("claude-code", "skill")
	if err != nil {
		t.Fatalf("InstallEntryRows after failed refresh: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("vendored rows changed after non-200 refresh: got %+v, want %+v", after, before)
	}
}

func TestRefreshInstallEntryPoints_ConnectionRefusedKeepsVendoredState(t *testing.T) {
	before := vendoredRows(t)

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	target := "http://" + ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	if err := RefreshInstallEntryPoints(installRefreshClient(t, target)); err == nil {
		t.Fatal("expected connection error")
	}
	after, err := InstallEntryRows("claude-code", "skill")
	if err != nil {
		t.Fatalf("InstallEntryRows after failed refresh: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("vendored rows changed after connection error: got %+v, want %+v", after, before)
	}
}
