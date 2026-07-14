package acif

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func FetchSourceURI(seed, trustCAPath string, resolve map[string]string) (string, error) {
	recorded, err := NormalizeSourceURI(seed)
	if err != nil {
		return "", err
	}
	current, err := url.Parse(seed)
	if err != nil {
		return "", &RejectError{ID: ErrSourceURIMalformed, Detail: err.Error()}
	}

	transport, err := fetchTransport(trustCAPath, resolve)
	if err != nil {
		return "", err
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	frozen := false
	visited := map[string]bool{recorded: true}
	for hop := 0; ; hop++ {
		resp, err := client.Get(current.String())
		if err != nil {
			return "", err
		}

		status := resp.StatusCode
		if !isRedirectStatus(status) {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			break
		}

		location := resp.Header.Get("Location")
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if location == "" {
			return "", fmt.Errorf("redirect missing Location")
		}
		ref, err := url.Parse(location)
		if err != nil {
			return "", err
		}
		next := current.ResolveReference(ref)
		if strings.ToLower(next.Scheme) != "https" {
			return "", &RejectError{ID: ErrSourceURIRedirectDowngrade}
		}

		if isPermanentRedirect(status) && !frozen {
			normalized, err := NormalizeSourceURI(next.String())
			if err != nil {
				return "", err
			}
			recorded = normalized
		} else if isTemporaryRedirect(status) {
			frozen = true
		}

		if hop+1 > 10 {
			return "", &RejectError{ID: ErrSourceURIRedirectLimit}
		}
		visitKey, err := redirectVisitKey(next)
		if err != nil {
			return "", err
		}
		if visited[visitKey] {
			return "", &RejectError{ID: ErrSourceURIRedirectLimit}
		}
		visited[visitKey] = true
		current = next
	}

	return recorded, nil
}

func fetchTransport(trustCAPath string, resolve map[string]string) (*http.Transport, error) {
	tlsConfig := &tls.Config{}
	if trustCAPath != "" {
		data, err := os.ReadFile(trustCAPath)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("trust_ca contains no PEM certificates")
		}
		tlsConfig.RootCAs = pool
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Transport{
		TLSClientConfig: tlsConfig,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if mapped, ok := resolve[host]; ok && mapped != "" {
				address = mapped
			}
			return dialer.DialContext(ctx, network, address)
		},
	}, nil
}

func isRedirectStatus(status int) bool {
	return isPermanentRedirect(status) || isTemporaryRedirect(status)
}

func isPermanentRedirect(status int) bool {
	return status == http.StatusMovedPermanently || status == http.StatusPermanentRedirect
}

func isTemporaryRedirect(status int) bool {
	return status == http.StatusFound || status == http.StatusSeeOther || status == http.StatusTemporaryRedirect
}

func redirectVisitKey(u *url.URL) (string, error) {
	copyURL := *u
	copyURL.RawQuery = ""
	copyURL.ForceQuery = false
	copyURL.Fragment = ""
	return NormalizeSourceURI(copyURL.String())
}
