package upstream

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
)

func publicProbeIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "198.18.0.0/15", "192.0.0.0/24", "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return false
		}
	}
	return true
}
func ValidateBroadcasterEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("registered service requires public HTTPS")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !publicProbeIP(ip) {
		return errors.New("private service endpoint rejected")
	}
	return nil
}

// Discovered registry endpoints are untrusted. Resolve and validate at dial time,
// pin the dial to a validated address, reject redirects and bypass proxy env vars.
func BroadcasterClient() *http.Client {
	transport := &http.Transport{TLSHandshakeTimeout: 2 * time.Second, ResponseHeaderTimeout: 3 * time.Second, MaxIdleConns: 16, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := validatedProbeIPs(ctx, host, net.DefaultResolver.LookupIPAddr, publicProbeDNS)
		if err != nil {
			return nil, err
		}

		dialer := net.Dialer{Timeout: 2 * time.Second}
		for _, entry := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(entry.IP.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, errors.New("service connection unavailable")
	}}
	return &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("service redirect rejected") }}
}
