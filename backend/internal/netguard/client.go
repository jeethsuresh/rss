// Package netguard provides an HTTP client suitable for URLs supplied by
// server tenants. It resolves and dials only public IP addresses, preventing
// feed/read-later URLs from reaching loopback, private, link-local, or metadata
// services. Redirects pass through the same guarded transport.
package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

func NewClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = guardedDialer{resolver: net.DefaultResolver}.DialContext
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

type guardedDialer struct{ resolver Resolver }

func (d guardedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return nil, fmt.Errorf("private network address is not allowed")
	}
	addresses, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			continue
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
	}
	return nil, fmt.Errorf("host does not resolve to a public address")
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	// Carrier-grade NAT is not classified as private by net.IP.IsPrivate.
	_, cgnat, _ := net.ParseCIDR("100.64.0.0/10")
	return !cgnat.Contains(ip)
}
