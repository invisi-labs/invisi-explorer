package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

type probeResolver func(context.Context, string) ([]net.IPAddr, error)

func validatedProbeIPs(ctx context.Context, host string, system, public probeResolver) ([]net.IPAddr, error) {
	ips, err := system(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("service DNS unavailable")
	}
	// Some local TUN proxies answer all public DNS with benchmarking-range aliases.
	// Resolve those aliases independently; do not grant private-network access.
	_, synthetic, _ := net.ParseCIDR("198.18.0.0/15")
	aliases := true
	for _, entry := range ips {
		if !synthetic.Contains(entry.IP) {
			aliases = false
		}
	}
	if aliases {
		ips, err = public(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("public service DNS unavailable")
		}
	}
	for _, entry := range ips {
		if !publicProbeIP(entry.IP) {
			return nil, errors.New("private service address rejected")
		}
	}
	return ips, nil
}

// This resolver has a fixed public TLS endpoint and validates all returned IPs
// before the caller dials them. It is used only for synthetic local DNS aliases.
func publicProbeDNS(ctx context.Context, host string) ([]net.IPAddr, error) {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	transport := &http.Transport{DisableKeepAlives: true, TLSHandshakeTimeout: 2 * time.Second, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, "1.1.1.1:443")
	}}
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("DNS redirect rejected") }}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://cloudflare-dns.com/dns-query?name="+url.QueryEscape(host)+"&type=A", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil || result.Status != 0 {
		return nil, errors.New("public service DNS rejected")
	}
	ips := []net.IPAddr{}
	for _, answer := range result.Answer {
		if answer.Type == 1 {
			if ip := net.ParseIP(answer.Data); ip != nil {
				ips = append(ips, net.IPAddr{IP: ip})
			}
		}
	}
	return ips, nil
}
