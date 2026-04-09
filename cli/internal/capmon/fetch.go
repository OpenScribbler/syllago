package capmon

import (
	"fmt"
	"net"
	"net/url"
)

// ValidateSourceURL enforces the SSRF allowlist: HTTPS only, no raw IPs,
// no hostnames that resolve to reserved/private address space.
// Must be called for every source URL at pipeline startup — NOT cached.
func ValidateSourceURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only https scheme allowed, got %q", u.Scheme)
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil {
		return fmt.Errorf("raw IP literal not allowed: %q", host)
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", host, err)
	}
	for _, ipStr := range ips {
		parsed := net.ParseIP(ipStr)
		if isReservedIP(parsed) {
			return fmt.Errorf("hostname %q resolves to reserved IP %q", host, ipStr)
		}
	}
	return nil
}

func isReservedIP(ip net.IP) bool {
	reserved := []string{
		"127.0.0.0/8",    // loopback
		"169.254.0.0/16", // link-local / AWS IMDS
		"100.64.0.0/10",  // CGNAT / Alibaba IMDS
		"10.0.0.0/8",     // private
		"172.16.0.0/12",  // private
		"192.168.0.0/16", // private
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
	}
	for _, cidr := range reserved {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
