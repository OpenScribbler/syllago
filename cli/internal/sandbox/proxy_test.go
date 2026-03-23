package sandbox

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
)

func TestProxy_AllowedDomain(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "proxy.sock")
	p := NewProxy(sock, []string{"api.anthropic.com"}, nil)
	if err := p.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Shutdown()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// CONNECT to an allowed domain (will fail to actually connect but should get 502, not 403)
	fmt.Fprintf(conn, "CONNECT api.anthropic.com:443 HTTP/1.1\r\nHost: api.anthropic.com:443\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	// 200 if reachable, 502 if can't connect — but NOT 403
	if resp.StatusCode == 403 {
		t.Error("expected allowed domain to not be blocked, got 403")
	}
}

func TestProxy_BlockedDomain(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "proxy.sock")
	p := NewProxy(sock, []string{"api.anthropic.com"}, nil)
	if err := p.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Shutdown()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT evil.com:443 HTTP/1.1\r\nHost: evil.com:443\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Errorf("expected 403 for blocked domain, got %d", resp.StatusCode)
	}

	blocked := p.BlockedDomains()
	if len(blocked) != 1 || blocked[0] != "evil.com" {
		t.Errorf("expected [evil.com] in blocked log, got %v", blocked)
	}
}

func TestProxy_WildcardAllowlist(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "proxy.sock")
	p := NewProxy(sock, []string{"*.npmjs.org"}, nil)
	if err := p.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Shutdown()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// registry.npmjs.org should match *.npmjs.org
	fmt.Fprintf(conn, "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode == 403 {
		t.Error("expected wildcard *.npmjs.org to allow registry.npmjs.org")
	}
}

func TestProxy_NonConnectMethod(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "proxy.sock")
	p := NewProxy(sock, []string{"api.anthropic.com"}, nil)
	if err := p.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Shutdown()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: api.anthropic.com\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != 405 {
		t.Errorf("expected 405 for GET method, got %d", resp.StatusCode)
	}
}

func TestProxy_Shutdown(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "proxy.sock")
	p := NewProxy(sock, []string{"api.anthropic.com"}, nil)
	if err := p.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	p.Shutdown()

	// After shutdown, connecting should fail
	_, err := net.Dial("unix", sock)
	if err == nil {
		t.Error("expected connection refused after shutdown")
	}

	// Socket file may or may not exist after shutdown — either way, the listener is closed.
}
