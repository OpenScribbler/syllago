package acif

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFetchSourceURIComposition(t *testing.T) {
	t.Parallel()

	type route struct {
		status int
		loc    string
	}
	tests := []struct {
		name    string
		routes  map[string]route
		want    string
		wantErr string
	}{
		{
			name: "301 advances recorded URI",
			routes: map[string]route{
				"/start":  {status: http.StatusMovedPermanently, loc: "/target"},
				"/target": {status: http.StatusOK},
			},
			want: "https://127.0.0.1/target",
		},
		{
			name: "302 freezes before signed query target",
			routes: map[string]route{
				"/start":  {status: http.StatusFound, loc: "/signed?token=secret"},
				"/signed": {status: http.StatusOK},
			},
			want: "https://127.0.0.1/start",
		},
		{
			name: "303 freezes recorded URI",
			routes: map[string]route{
				"/start":  {status: http.StatusSeeOther, loc: "/target"},
				"/target": {status: http.StatusOK},
			},
			want: "https://127.0.0.1/start",
		},
		{
			name: "301 then 302 keeps first permanent target",
			routes: map[string]route{
				"/start": {status: http.StatusMovedPermanently, loc: "/p1"},
				"/p1":    {status: http.StatusFound, loc: "/tmp"},
				"/tmp":   {status: http.StatusOK},
			},
			want: "https://127.0.0.1/p1",
		},
		{
			name: "301 then 308 advances to final permanent target",
			routes: map[string]route{
				"/start": {status: http.StatusMovedPermanently, loc: "/p1"},
				"/p1":    {status: http.StatusPermanentRedirect, loc: "/p2"},
				"/p2":    {status: http.StatusOK},
			},
			want: "https://127.0.0.1/p2",
		},
		{
			name: "301 then 302 then 301 keeps first permanent target",
			routes: map[string]route{
				"/start": {status: http.StatusMovedPermanently, loc: "/p1"},
				"/p1":    {status: http.StatusFound, loc: "/tmp"},
				"/tmp":   {status: http.StatusMovedPermanently, loc: "/p2"},
				"/p2":    {status: http.StatusOK},
			},
			want: "https://127.0.0.1/p1",
		},
		{
			name: "302 then 301 keeps seed",
			routes: map[string]route{
				"/start": {status: http.StatusFound, loc: "/tmp"},
				"/tmp":   {status: http.StatusMovedPermanently, loc: "/p1"},
				"/p1":    {status: http.StatusOK},
			},
			want: "https://127.0.0.1/start",
		},
		{
			name: "permanent downgrade rejects",
			routes: map[string]route{
				"/start": {status: http.StatusMovedPermanently, loc: "http://127.0.0.1/insecure"},
			},
			wantErr: ErrSourceURIRedirectDowngrade,
		},
		{
			name: "temporary then permanent downgrade rejects",
			routes: map[string]route{
				"/start": {status: http.StatusFound, loc: "/tmp"},
				"/tmp":   {status: http.StatusMovedPermanently, loc: "http://127.0.0.1/insecure"},
			},
			wantErr: ErrSourceURIRedirectDowngrade,
		},
		{
			name: "loop rejects",
			routes: map[string]route{
				"/start": {status: http.StatusMovedPermanently, loc: "/a"},
				"/a":     {status: http.StatusMovedPermanently, loc: "/start"},
			},
			wantErr: ErrSourceURIRedirectLimit,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server, trustCA := newFetchTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
				step, ok := tc.routes[r.URL.Path]
				if !ok {
					http.NotFound(w, r)
					return
				}
				if step.loc != "" {
					w.Header().Set("Location", step.loc)
				}
				if step.status == 0 {
					step.status = http.StatusOK
				}
				w.WriteHeader(step.status)
				_, _ = w.Write([]byte("ok"))
			})
			defer server.Close()

			got, err := FetchSourceURI("https://127.0.0.1/start", trustCA, map[string]string{
				"127.0.0.1": server.Listener.Addr().String(),
			})
			if tc.wantErr != "" {
				assertRejectID(t, err, tc.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("FetchSourceURI() error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("FetchSourceURI() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFetchSourceURIRedirectHopLimit(t *testing.T) {
	t.Parallel()

	server, trustCA := newFetchTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		var n int
		_, _ = fmt.Sscanf(r.URL.Path, "/hop/%d", &n)
		w.Header().Set("Location", fmt.Sprintf("/hop/%d", n+1))
		w.WriteHeader(http.StatusMovedPermanently)
	})
	defer server.Close()

	_, err := FetchSourceURI("https://127.0.0.1/hop/0", trustCA, map[string]string{
		"127.0.0.1": server.Listener.Addr().String(),
	})
	assertRejectID(t, err, ErrSourceURIRedirectLimit)
}

func newFetchTLSServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox blocks local sockets: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = ln
	server.StartTLS()

	cert := server.Certificate()
	if cert == nil {
		t.Fatal("test TLS server has no certificate")
	}
	if _, err := x509.ParseCertificate(cert.Raw); err != nil {
		t.Fatalf("test TLS certificate parse: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if pemBytes == nil {
		t.Fatal("encoding test TLS certificate PEM")
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pemBytes, 0o644); err != nil {
		t.Fatalf("write trust CA: %v", err)
	}
	return server, path
}
