package sandbox

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Proxy is an HTTP CONNECT proxy with a domain allowlist.
// It listens on a UNIX socket and tunnels allowed connections.
type Proxy struct {
	socketPath     string
	allowedDomains map[string]bool // host → allowed (no port)
	allowedPorts   map[int]bool    // localhost ports allowed
	blockedLog     []string        // blocked domain names (for session summary)
	mu             sync.Mutex
	listener       net.Listener
}

// NewProxy creates a Proxy. socketPath must not exist yet.
func NewProxy(socketPath string, allowedDomains []string, allowedPorts []int) *Proxy {
	dm := make(map[string]bool)
	for _, d := range allowedDomains {
		dm[d] = true
	}
	pm := make(map[int]bool)
	for _, p := range allowedPorts {
		pm[p] = true
	}
	return &Proxy{
		socketPath:     socketPath,
		allowedDomains: dm,
		allowedPorts:   pm,
	}
}

// Start begins accepting connections. Returns an error if the socket cannot be created.
// Runs the accept loop in a new goroutine; returns immediately.
func (p *Proxy) Start() error {
	ln, err := net.Listen("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("proxy listen: %w", err)
	}
	p.listener = ln
	go p.accept(ln)
	return nil
}

// Shutdown closes the listener, causing the accept loop to exit.
func (p *Proxy) Shutdown() {
	if p.listener != nil {
		_ = p.listener.Close()
	}
}

// BlockedDomains returns the list of domains that were blocked during the session.
func (p *Proxy) BlockedDomains() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.blockedLog))
	copy(out, p.blockedLog)
	return out
}

func (p *Proxy) accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed
		}
		go p.handleConn(conn)
	}
}

func (p *Proxy) handleConn(client net.Conn) {
	defer func() { _ = client.Close() }()
	br := bufio.NewReader(client)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		fmt.Fprintf(client, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
		return
	}

	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
	}

	if !p.isAllowed(host) {
		p.mu.Lock()
		p.blockedLog = append(p.blockedLog, host)
		p.mu.Unlock()
		log.Printf("[sandbox] Blocked connection to %s (not in allowlist)", host)
		fmt.Fprintf(client, "HTTP/1.1 403 Forbidden\r\n\r\n")
		return
	}

	// Port filtering: if allowedPorts is configured and the target is localhost,
	// verify the port is in the allowlist. This prevents sandboxed processes
	// from connecting to arbitrary local services (databases, dev servers, etc.).
	if !p.isPortAllowed(host, req.Host) {
		p.mu.Lock()
		p.blockedLog = append(p.blockedLog, req.Host)
		p.mu.Unlock()
		log.Printf("[sandbox] Blocked connection to %s (port not in allowlist)", req.Host)
		fmt.Fprintf(client, "HTTP/1.1 403 Forbidden\r\n\r\n")
		return
	}

	upstream, err := net.Dial("tcp", req.Host)
	if err != nil {
		fmt.Fprintf(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer func() { _ = upstream.Close() }()

	fmt.Fprintf(client, "HTTP/1.1 200 Connection Established\r\n\r\n")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(upstream, br) }()
	go func() { defer wg.Done(); _, _ = io.Copy(client, upstream) }()
	wg.Wait()
}

// isPortAllowed checks if a localhost connection is allowed based on the port allowlist.
// Non-localhost targets are always allowed (domain check handles those).
// If allowedPorts is empty, all ports are allowed (no port filtering configured).
func (p *Proxy) isPortAllowed(host, hostPort string) bool {
	if len(p.allowedPorts) == 0 {
		return true // no port filtering configured
	}
	host = strings.ToLower(host)
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return true // port filtering only applies to localhost
	}
	_, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return false // can't parse port — deny
	}
	port := 0
	for _, c := range portStr {
		if c < '0' || c > '9' {
			return false
		}
		port = port*10 + int(c-'0')
	}
	return p.allowedPorts[port]
}

// isAllowed returns true if the host is on the allowlist.
// Handles wildcard prefixes like "*.npmjs.org".
func (p *Proxy) isAllowed(host string) bool {
	host = strings.ToLower(host)
	if p.allowedDomains[host] {
		return true
	}
	// Wildcard match: *.foo.com matches bar.foo.com
	parts := strings.SplitN(host, ".", 2)
	if len(parts) == 2 {
		wildcard := "*." + parts[1]
		if p.allowedDomains[wildcard] {
			return true
		}
	}
	return false
}
