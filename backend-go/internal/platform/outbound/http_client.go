package outbound

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// NormalizePublicHTTPSBaseURL normalizes an outbound provider URL and rejects
// URL forms that can hide credentials, extra destinations, or local networks.
func NormalizePublicHTTPSBaseURL(value string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	if trimmed == "" || len([]rune(trimmed)) > 500 {
		return "", errors.New("长度必须在 1 到 500 之间")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("必须是有效 URL")
	}
	if parsed.User != nil {
		return "", errors.New("不允许包含用户名或密码")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("不允许包含 query 或 fragment")
	}
	if parsed.Scheme != "https" {
		return "", errors.New("仅支持 https")
	}
	if IsBlockedPublicProviderHost(parsed.Hostname()) {
		return "", errors.New("不允许指向本机、内网或保留地址")
	}
	return trimmed, nil
}

// NewPublicHTTPSClient creates an HTTP client for provider calls that does not
// follow redirects or use environment proxies, and validates resolved IPs before dialing.
func NewPublicHTTPSClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: timeout}
	resolver := net.DefaultResolver
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		target, err := resolvePublicDialTarget(ctx, resolver, network, address)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, target)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// IsBlockedPublicProviderHost reports whether host is local, private, or reserved.
func IsBlockedPublicProviderHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return true
	}
	switch host {
	case "localhost", "localhost.localdomain":
		return true
	}
	if strings.HasSuffix(host, ".localhost") {
		return true
	}
	if strings.Contains(host, "%") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return !IsPublicProviderAddr(addr)
}

// IsPublicProviderAddr reports whether addr is a public Internet address suitable for provider calls.
func IsPublicProviderAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, prefix := range blockedProviderAddrPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

func resolvePublicDialTarget(ctx context.Context, resolver *net.Resolver, network string, address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if IsBlockedPublicProviderHost(host) {
		return "", fmt.Errorf("blocked outbound host %q", host)
	}
	addrs, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("no addresses found for %q", host)
	}
	for _, addr := range addrs {
		if !IsPublicProviderAddr(addr) {
			return "", fmt.Errorf("blocked outbound address %s for %q", addr, host)
		}
	}
	addr, ok := pickDialAddr(network, addrs)
	if !ok {
		return "", fmt.Errorf("no %s address found for %q", network, host)
	}
	return net.JoinHostPort(addr.String(), port), nil
}

func pickDialAddr(network string, addrs []netip.Addr) (netip.Addr, bool) {
	for _, addr := range addrs {
		addr = addr.Unmap()
		switch network {
		case "tcp4":
			if addr.Is4() {
				return addr, true
			}
		case "tcp6":
			if addr.Is6() {
				return addr, true
			}
		default:
			return addr, true
		}
	}
	return netip.Addr{}, false
}

var blockedProviderAddrPrefixes = []netip.Prefix{
	mustPrefix("0.0.0.0/8"),
	mustPrefix("100.64.0.0/10"),
	mustPrefix("192.0.0.0/24"),
	mustPrefix("192.0.2.0/24"),
	mustPrefix("192.88.99.0/24"),
	mustPrefix("198.18.0.0/15"),
	mustPrefix("198.51.100.0/24"),
	mustPrefix("203.0.113.0/24"),
	mustPrefix("240.0.0.0/4"),
	mustPrefix("255.255.255.255/32"),
	mustPrefix("64:ff9b::/96"),
	mustPrefix("64:ff9b:1::/48"),
	mustPrefix("2001:db8::/32"),
	mustPrefix("2002::/16"),
}

func mustPrefix(value string) netip.Prefix {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		panic(err)
	}
	return prefix
}
